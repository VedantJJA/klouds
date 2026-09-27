// Package webhook provides Git push-to-deploy webhook receivers for GitHub and GitLab.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/builder"
	"github.com/vedant/klouds/internal/db"
)

// Handler processes incoming Git webhooks.
type Handler struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	engine   *builder.Engine
	deployer *builder.Deployer
}

// NewHandler creates a new webhook handler.
func NewHandler(pool *pgxpool.Pool, engine *builder.Engine, deployer *builder.Deployer) *Handler {
	return &Handler{
		pool:     pool,
		queries:  db.New(pool),
		engine:   engine,
		deployer: deployer,
	}
}

// GitHubPushEvent represents the GitHub push webhook payload structure.
type GitHubPushEvent struct {
	Ref        string `json:"ref"` // e.g. "refs/heads/main"
	After      string `json:"after"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"head_commit"`
	Repository struct {
		Name     string `json:"name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
}

// HandleGitHub handles POST /api/webhooks/github/{serviceID}
func (h *Handler) HandleGitHub(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if serviceID == "" {
		http.Error(w, `{"error":"serviceID is required"}`, http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to read payload"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify event header
	event := r.Header.Get("X-GitHub-Event")
	if event == "ping" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"pong"}`))
		return
	}
	if event != "push" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored","reason":"not a push event"}`))
		return
	}

	svc, err := h.queries.GetServiceByID(r.Context(), serviceID)
	if err != nil {
		http.Error(w, `{"error":"service not found"}`, http.StatusNotFound)
		return
	}

	if !svc.AutoDeploy {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored","reason":"auto-deploy disabled"}`))
		return
	}

	// Verify HMAC signature if signature header is provided
	sig := r.Header.Get("X-Hub-Signature-256")
	if sig != "" {
		if !verifyGitHubSignature(body, svc.ID, sig) {
			http.Error(w, `{"error":"invalid webhook signature"}`, http.StatusUnauthorized)
			return
		}
	}

	var payload GitHubPushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, `{"error":"invalid json payload"}`, http.StatusBadRequest)
		return
	}

	// Extract branch name from ref (e.g., "refs/heads/main" -> "main")
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	expectedBranch := "main"
	if svc.Branch != nil && *svc.Branch != "" {
		expectedBranch = *svc.Branch
	}

	if branch != expectedBranch {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"ignored","reason":"branch mismatch (%s != %s)"}`, branch, expectedBranch)))
		return
	}

	// Create deployment record
	commitSHA := payload.HeadCommit.ID
	commitMsg := payload.HeadCommit.Message
	trigger := "webhook"
	imageTag := fmt.Sprintf("klouds/%s:%s", svc.Slug, commitSHA[:min(8, len(commitSHA))])

	dep, err := h.queries.CreateDeployment(r.Context(), db.CreateDeploymentParams{
		ServiceID: svc.ID,
		UserID:    svc.UserID,
		Status:    "building",
		CommitSHA: &commitSHA,
		CommitMsg: &commitMsg,
		ImageTag:  imageTag,
		Trigger:   trigger,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create deployment record")
		http.Error(w, `{"error":"failed to create deployment"}`, http.StatusInternalServerError)
		return
	}

	// Launch build and deployment asynchronously in background
	go func() {
		ctx := r.Context()
		repoURL := ""
		if svc.RepoURL != nil {
			repoURL = *svc.RepoURL
		}

		buildRes, err := h.engine.Build(ctx, builder.BuildOptions{
			DeploymentID: dep.ID,
			ServiceID:    svc.ID,
			ServiceSlug:  svc.Slug,
			RepoURL:      repoURL,
			Branch:       branch,
			CommitSHA:    commitSHA,
			BuildMethod:  svc.BuildMethod,
			EnvVars:      nil,
		})

		if err != nil {
			log.Error().Err(err).Str("deployment", dep.ID).Msg("Build step failed")
			_, _ = h.queries.UpdateDeploymentFinished(ctx, dep.ID, "failed", 0)
			return
		}

		// Perform zero-downtime blue-green deployment
		deployErr := h.deployer.Deploy(ctx, builder.DeployRequest{
			ServiceID:    svc.ID,
			DeploymentID: dep.ID,
			ImageTag:     buildRes.ImageTag,
			EnvVars:      nil,
		})

		if deployErr != nil {
			log.Error().Err(deployErr).Str("deployment", dep.ID).Msg("Deployment rollout failed")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"queued","deployment_id":"%s"}`, dep.ID)))
}

func verifyGitHubSignature(payload []byte, secret string, signatureHeader string) bool {
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}
	expectedMAC := strings.TrimPrefix(signatureHeader, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actualMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(actualMAC), []byte(expectedMAC))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

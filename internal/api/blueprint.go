package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vedant/klouds/internal/blueprint"
	"github.com/vedant/klouds/internal/builder"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
	"github.com/vedant/klouds/internal/secrets"
)

// BlueprintHandler handles auto-detection and blueprint application for projects.
type BlueprintHandler struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	dataDir    string
	reconciler *blueprint.Reconciler
}

// NewBlueprintHandler creates a new blueprint handler instance.
func NewBlueprintHandler(
	pool *pgxpool.Pool,
	dataDir string,
	encryptor *secrets.Encryptor,
	containers *container.Manager,
	engine *builder.Engine,
	deployer *builder.Deployer,
	domain string,
) *BlueprintHandler {
	return &BlueprintHandler{
		pool:       pool,
		queries:    db.New(pool),
		dataDir:    dataDir,
		reconciler: blueprint.NewReconciler(pool, encryptor, containers, engine, deployer, domain),
	}
}

// DetectRequest contains repository info to scan for services.
type DetectRequest struct {
	RepoURL string `json:"repo_url"`
	Branch  string `json:"branch"`
}

// ApplyBlueprintRequest contains repository or YAML configuration to apply.
type ApplyBlueprintRequest struct {
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	YAMLContent string `json:"yaml_content"`
}

// Detect handles POST /api/projects/{projectID}/blueprint/detect
func (h *BlueprintHandler) Detect(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	// Verify project ownership
	project, err := h.queries.GetProjectByID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if role != "admin" && project.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req DetectRequest
	if err := decodeJSON(r, &req); err != nil || req.RepoURL == "" {
		writeError(w, http.StatusBadRequest, "repo_url is required")
		return
	}

	branch := strings.TrimSpace(req.Branch)

	res, err := blueprint.DetectFromRepo(r.Context(), req.RepoURL, branch, h.dataDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// Preview handles POST /api/projects/{projectID}/blueprint/preview
func (h *BlueprintHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req ApplyBlueprintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var bp *blueprint.Blueprint
	detectedBranch := ""
	if req.YAMLContent != "" {
		parsed, err := blueprint.ParseBlueprint([]byte(req.YAMLContent))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		bp = parsed
	} else if req.RepoURL != "" {
		branch := req.Branch
		res, err := blueprint.DetectFromRepo(r.Context(), req.RepoURL, branch, h.dataDir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		bp = res.Blueprint
		detectedBranch = res.DetectedBranch
	} else {
		writeError(w, http.StatusBadRequest, "either yaml_content or repo_url must be provided")
		return
	}

	if err := bp.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":         bp.Version,
		"services":        bp.Services,
		"databases":       bp.Databases,
		"detected_branch": detectedBranch,
	})
}

// Apply handles POST /api/projects/{projectID}/blueprint/apply
func (h *BlueprintHandler) Apply(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	// Verify project ownership
	project, err := h.queries.GetProjectByID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if role != "admin" && project.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req ApplyBlueprintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var bp *blueprint.Blueprint
	branch := strings.TrimSpace(req.Branch)

	if req.YAMLContent != "" {
		parsed, err := blueprint.ParseBlueprint([]byte(req.YAMLContent))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		bp = parsed
	} else if req.RepoURL != "" {
		res, err := blueprint.DetectFromRepo(r.Context(), req.RepoURL, branch, h.dataDir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		bp = res.Blueprint
		if (branch == "" || branch == "main") && res.DetectedBranch != "" {
			branch = res.DetectedBranch
		}
	} else {
		writeError(w, http.StatusBadRequest, "either yaml_content or repo_url must be provided")
		return
	}

	result, err := h.reconciler.Reconcile(r.Context(), projectID, userID, req.RepoURL, branch, bp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "reconciled",
		"result": result,
	})
}

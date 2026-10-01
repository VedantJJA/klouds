package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/caddy"
	"github.com/vedant/klouds/internal/cleaner"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
)

// ProjectHandler handles project CRUD operations.
type ProjectHandler struct {
	queries    *db.Queries
	pool       *pgxpool.Pool
	containers *container.Manager
	caddy      *caddy.Manager
	cleaner    *cleaner.Cleaner
}

// NewProjectHandler creates a new project handler.
func NewProjectHandler(pool *pgxpool.Pool, containers *container.Manager, caddyMgr *caddy.Manager, cln *cleaner.Cleaner) *ProjectHandler {
	return &ProjectHandler{
		queries:    db.New(pool),
		pool:       pool,
		containers: containers,
		caddy:      caddyMgr,
		cleaner:    cln,
	}
}

// CreateProjectRequest is the JSON body for creating a project.
type CreateProjectRequest struct {
	Name string `json:"name"`
}

// Create handles POST /api/projects
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())

	// Verify user is active
	if err := h.checkUserActive(r.Context(), userID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	var req CreateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	slug := slugify(req.Name)

	project, err := h.queries.CreateProject(r.Context(), db.CreateProjectParams{
		UserID: userID,
		Name:   req.Name,
		Slug:   slug,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "a project with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

// List handles GET /api/projects
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	var projects []db.Project
	var err error

	if role == "admin" {
		projects, err = h.queries.ListAllProjects(r.Context())
	} else {
		projects, err = h.queries.ListProjectsByUser(r.Context(), userID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

// Get handles GET /api/projects/{projectID}
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	project, err := h.queries.GetProjectByID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Authorization: user can only see their own projects (admin sees all)
	if role != "admin" && project.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// Delete handles DELETE /api/projects/{projectID}
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	project, err := h.queries.GetProjectByID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if role != "admin" && project.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// 1. Fetch all services belonging to this project
	services, err := h.queries.ListServicesByProject(r.Context(), projectID)
	if err != nil {
		log.Error().Err(err).Str("project_id", projectID).Msg("failed to list services for project deletion")
	}

	// 2. Fetch all databases belonging to this project
	databases, err := h.queries.ListDatabasesByProject(r.Context(), projectID)
	if err != nil {
		log.Error().Err(err).Str("project_id", projectID).Msg("failed to list databases for project deletion")
	}

	// 3. Stop and remove all service containers, remove Caddy routes
	for _, svc := range services {
		if svc.ContainerID != nil && *svc.ContainerID != "" && h.containers != nil {
			_ = h.containers.StopContainer(r.Context(), *svc.ContainerID)
			_ = h.containers.RemoveContainer(r.Context(), *svc.ContainerID)
		}
		if svc.Subdomain != "" && h.caddy != nil {
			_ = h.caddy.RemoveRoute(svc.Subdomain)
		}
	}

	// 4. Stop and remove all database containers
	for _, dbRecord := range databases {
		if dbRecord.ContainerID != nil && *dbRecord.ContainerID != "" && h.containers != nil {
			_ = h.containers.StopContainer(r.Context(), *dbRecord.ContainerID)
			_ = h.containers.RemoveContainer(r.Context(), *dbRecord.ContainerID)
		}
		if h.containers != nil {
			dbName := fmt.Sprintf("klouds-db-%s", dbRecord.InternalHost)
			_ = h.containers.StopContainer(r.Context(), dbName)
			_ = h.containers.RemoveContainer(r.Context(), dbName)
		}
	}

	// 5. Delete project from DB (CASCADE deletes services and databases rows in PostgreSQL)
	if err := h.queries.DeleteProject(r.Context(), projectID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	// 6. Prune any remaining unreferenced containers and Caddy routes in background
	if h.cleaner != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			_, _ = h.cleaner.PruneAll(ctx, 0)
		}()
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "project deleted"})
}

// checkUserActive verifies the user has an active account.
func (h *ProjectHandler) checkUserActive(ctx context.Context, userID string) error {
	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Status != "active" {
		return &StatusError{Message: "account is not active"}
	}
	return nil
}

// StatusError is a simple error with a message.
type StatusError struct {
	Message string
}

func (e *StatusError) Error() string {
	return e.Message
}

// --- slug helpers ---

var slugRegex = regexp.MustCompile(`[^a-z0-9-]`)

// slugify converts a name to a URL-safe slug.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRegex.ReplaceAllString(s, "")
	// Remove consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "project"
	}
	return s
}

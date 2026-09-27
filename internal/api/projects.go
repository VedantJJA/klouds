package api

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vedant/klouds/internal/db"
)

// ProjectHandler handles project CRUD operations.
type ProjectHandler struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

// NewProjectHandler creates a new project handler.
func NewProjectHandler(pool *pgxpool.Pool) *ProjectHandler {
	return &ProjectHandler{
		queries: db.New(pool),
		pool:    pool,
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

	// CASCADE will delete services and databases under this project
	if err := h.queries.DeleteProject(r.Context(), projectID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
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

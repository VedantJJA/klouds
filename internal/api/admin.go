package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
)

// AdminHandler handles admin-only operations.
type AdminHandler struct {
	queries    *db.Queries
	pool       *pgxpool.Pool
	containers *container.Manager
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(pool *pgxpool.Pool, containers *container.Manager) *AdminHandler {
	return &AdminHandler{
		queries:    db.New(pool),
		pool:       pool,
		containers: containers,
	}
}

// ListUsers handles GET /api/admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	dtos := make([]UserDTO, len(users))
	for i, u := range users {
		dtos[i] = toUserDTO(u)
	}

	writeJSON(w, http.StatusOK, dtos)
}

// ListPendingUsers handles GET /api/admin/users/pending
func (h *AdminHandler) ListPendingUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListPendingUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pending users")
		return
	}

	dtos := make([]UserDTO, len(users))
	for i, u := range users {
		dtos[i] = toUserDTO(u)
	}

	writeJSON(w, http.StatusOK, dtos)
}

// ApproveUserRequest is the request body for approving/rejecting a user.
type ApproveUserRequest struct {
	Status string `json:"status"` // "active" or "suspended"
}

// UpdateUserStatus handles PATCH /api/admin/users/{userID}/status
func (h *AdminHandler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	var req ApproveUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status != "active" && req.Status != "suspended" && req.Status != "pending" {
		writeError(w, http.StatusBadRequest, "status must be 'active', 'suspended', or 'pending'")
		return
	}

	user, err := h.queries.UpdateUserStatus(r.Context(), userID, req.Status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user status")
		return
	}

	// If user is being suspended, stop their containers
	if req.Status == "suspended" {
		go h.stopUserContainers(userID)
	}

	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// DeleteUser handles DELETE /api/admin/users/{userID}
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	// Don't allow deleting self
	if getUserID(r.Context()) == userID {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	// Stop user's containers first
	go h.stopUserContainers(userID)

	if err := h.queries.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

// GetUserQuota handles GET /api/admin/users/{userID}/quota
func (h *AdminHandler) GetUserQuota(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	quota, err := h.queries.GetUserQuota(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quota not found")
		return
	}

	writeJSON(w, http.StatusOK, quota)
}

// UpdateQuotaRequest is the request body for updating user quotas.
type UpdateQuotaRequest struct {
	MaxServices        int32 `json:"max_services"`
	MaxDatabases       int32 `json:"max_databases"`
	CpuPerContainer    int32 `json:"cpu_per_container"`
	MemoryPerContainer int64 `json:"memory_per_container"`
	TotalMemory        int64 `json:"total_memory"`
	DiskPerService     int64 `json:"disk_per_service"`
	DiskPerDatabase    int64 `json:"disk_per_database"`
	BuildTimeoutSecs   int32 `json:"build_timeout_secs"`
	MaxDeploysPerDay   int32 `json:"max_deploys_per_day"`
	MaxCustomDomains   int32 `json:"max_custom_domains"`
}

// UpdateUserQuota handles PUT /api/admin/users/{userID}/quota
func (h *AdminHandler) UpdateUserQuota(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	var req UpdateQuotaRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	quota, err := h.queries.UpdateUserQuota(r.Context(), db.CreateUserQuotaParams{
		UserID:             userID,
		MaxServices:        req.MaxServices,
		MaxDatabases:       req.MaxDatabases,
		CpuPerContainer:    req.CpuPerContainer,
		MemoryPerContainer: req.MemoryPerContainer,
		TotalMemory:        req.TotalMemory,
		DiskPerService:     req.DiskPerService,
		DiskPerDatabase:    req.DiskPerDatabase,
		BuildTimeoutSecs:   req.BuildTimeoutSecs,
		MaxDeploysPerDay:   req.MaxDeploysPerDay,
		MaxCustomDomains:   req.MaxCustomDomains,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update quota")
		return
	}

	writeJSON(w, http.StatusOK, quota)
}

// ListAllServices handles GET /api/admin/services — lists services across all users.
func (h *AdminHandler) ListAllServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.queries.ListAllServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services")
		return
	}
	writeJSON(w, http.StatusOK, services)
}

// ListAllDeployments handles GET /api/admin/deployments
func (h *AdminHandler) ListAllDeployments(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.queries.ListAllDeployments(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deployments")
		return
	}
	writeJSON(w, http.StatusOK, deployments)
}

// stopUserContainers stops all containers owned by a user (runs in background).
func (h *AdminHandler) stopUserContainers(userID string) {
	ctx := context.Background()

	services, err := h.queries.ListServicesByUser(ctx, userID)
	if err != nil {
		return
	}

	for _, svc := range services {
		if svc.ContainerID != nil && *svc.ContainerID != "" {
			_ = h.containers.StopContainer(ctx, *svc.ContainerID)
		}
	}

	databases, err := h.queries.ListDatabasesByUser(ctx, userID)
	if err != nil {
		return
	}

	for _, dbInst := range databases {
		if dbInst.ContainerID != nil && *dbInst.ContainerID != "" {
			_ = h.containers.StopContainer(ctx, *dbInst.ContainerID)
		}
	}
}

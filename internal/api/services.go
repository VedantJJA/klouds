package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/builder"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
)

// ServiceHandler handles service CRUD and lifecycle operations.
type ServiceHandler struct {
	queries    *db.Queries
	pool       *pgxpool.Pool
	containers *container.Manager
	engine     *builder.Engine
	deployer   *builder.Deployer
	domain     string // Base domain for subdomain routing
}

// NewServiceHandler creates a new service handler.
func NewServiceHandler(pool *pgxpool.Pool, containers *container.Manager, engine *builder.Engine, deployer *builder.Deployer, domain string) *ServiceHandler {
	return &ServiceHandler{
		queries:    db.New(pool),
		pool:       pool,
		containers: containers,
		engine:     engine,
		deployer:   deployer,
		domain:     domain,
	}
}

// CreateServiceRequest is the JSON body for creating a service.
type CreateServiceRequest struct {
	ProjectID       string  `json:"project_id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	BuildMethod     string  `json:"build_method"`
	RepoURL         *string `json:"repo_url"`
	Branch          *string `json:"branch"`
	RootDir         string  `json:"root_dir"`
	DockerfilePath  *string `json:"dockerfile_path"`
	BuildCommand    *string `json:"build_command"`
	StartCommand    *string `json:"start_command"`
	Port            int32   `json:"port"`
	HealthCheckPath *string `json:"health_check_path"`
	AutoDeploy      bool    `json:"auto_deploy"`
}

// Create handles POST /api/services
func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	var req CreateServiceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "name and project_id are required")
		return
	}

	// Verify project ownership
	project, err := h.queries.GetProjectByID(r.Context(), req.ProjectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if role != "admin" && project.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Check quota
	if role != "admin" {
		quota, err := h.queries.GetUserQuota(r.Context(), userID)
		if err == nil && quota.MaxServices > 0 {
			count, _ := h.queries.CountServicesByUser(r.Context(), userID)
			if count >= int64(quota.MaxServices) {
				writeError(w, http.StatusForbidden, fmt.Sprintf("service limit reached (%d/%d)", count, quota.MaxServices))
				return
			}
		}
	}

	// Defaults
	if req.Type == "" {
		req.Type = "web"
	}
	if req.BuildMethod == "" {
		req.BuildMethod = "nixpacks"
	}
	if req.Port == 0 {
		req.Port = 3000
	}

	slug := slugify(req.Name)
	subdomain := slug // slug.yourdomain.com

	// Get resource limits from quota
	var cpuLimit int32 = 500
	var memLimit int64 = 256 * 1024 * 1024
	if role != "admin" {
		quota, err := h.queries.GetUserQuota(r.Context(), userID)
		if err == nil {
			cpuLimit = quota.CpuPerContainer
			memLimit = quota.MemoryPerContainer
		}
	} else {
		cpuLimit = 2000
		memLimit = 2 * 1024 * 1024 * 1024
	}

	rootDir := req.RootDir
	if rootDir == "" {
		rootDir = "."
	}

	svc, err := h.queries.CreateService(r.Context(), db.CreateServiceParams{
		ProjectID:       req.ProjectID,
		UserID:          userID,
		Name:            req.Name,
		Slug:            slug,
		Type:            req.Type,
		BuildMethod:     req.BuildMethod,
		RepoURL:         req.RepoURL,
		Branch:          req.Branch,
		RootDirectory:   rootDir,
		DockerfilePath:  req.DockerfilePath,
		BuildCommand:    req.BuildCommand,
		StartCommand:    req.StartCommand,
		Port:            req.Port,
		HealthCheckPath: req.HealthCheckPath,
		AutoDeploy:      req.AutoDeploy,
		Subdomain:       subdomain,
		CpuLimit:        cpuLimit,
		MemoryLimit:     memLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create service")
		writeError(w, http.StatusInternalServerError, "failed to create service")
		return
	}

	writeJSON(w, http.StatusCreated, svc)
}

// List handles GET /api/services?project_id=xxx
func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())
	projectID := r.URL.Query().Get("project_id")

	var services []db.Service
	var err error

	if projectID != "" {
		// Verify project access
		project, perr := h.queries.GetProjectByID(r.Context(), projectID)
		if perr != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		if role != "admin" && project.UserID != userID {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		services, err = h.queries.ListServicesByProject(r.Context(), projectID)
	} else if role == "admin" {
		services, err = h.queries.ListAllServices(r.Context())
	} else {
		services, err = h.queries.ListServicesByUser(r.Context(), userID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services")
		return
	}

	writeJSON(w, http.StatusOK, services)
}

// Get handles GET /api/services/{serviceID}
func (h *ServiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	svc, err := h.queries.GetServiceByID(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	if role != "admin" && svc.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

// Stop handles POST /api/services/{serviceID}/stop
func (h *ServiceHandler) Stop(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	svc, err := h.queries.GetServiceByID(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	if role != "admin" && svc.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if svc.ContainerID != nil && *svc.ContainerID != "" {
		if err := h.containers.StopContainer(r.Context(), *svc.ContainerID); err != nil {
			log.Error().Err(err).Str("container", *svc.ContainerID).Msg("failed to stop container")
		}
	}

	svc, err = h.queries.UpdateServiceStatus(r.Context(), serviceID, "stopped")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update service status")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

// Restart handles POST /api/services/{serviceID}/restart
func (h *ServiceHandler) Restart(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	svc, err := h.queries.GetServiceByID(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	if role != "admin" && svc.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Stop existing container
	if svc.ContainerID != nil && *svc.ContainerID != "" {
		_ = h.containers.StopContainer(r.Context(), *svc.ContainerID)
		_ = h.containers.RemoveContainer(r.Context(), *svc.ContainerID)
	}

	// Re-create from last image
	if svc.ImageTag == nil || *svc.ImageTag == "" {
		writeError(w, http.StatusBadRequest, "service has never been deployed")
		return
	}

	envVars := map[string]string{
		"PORT": fmt.Sprintf("%d", svc.Port),
	}

	containerName := fmt.Sprintf("klouds-svc-%s", svc.Slug)
	containerID, err := h.containers.CreateServiceContainer(r.Context(), container.ServiceConfig{
		Name:         containerName,
		Image:        *svc.ImageTag,
		Port:         int(svc.Port),
		EnvVars:      envVars,
		CPULimit:     int64(svc.CpuLimit),
		MemoryLimit:  svc.MemoryLimit,
		NetworkAlias: svc.Slug,
		Labels: map[string]string{
			"klouds.service":  svc.ID,
			"klouds.user":     svc.UserID,
			"klouds.project":  svc.ProjectID,
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to restart container")
		writeError(w, http.StatusInternalServerError, "failed to restart service")
		return
	}

	svc, err = h.queries.UpdateServiceContainer(r.Context(), serviceID, &containerID, svc.ImageTag, "running")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update service")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

// Delete handles DELETE /api/services/{serviceID}
func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	svc, err := h.queries.GetServiceByID(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	if role != "admin" && svc.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Stop and remove container
	if svc.ContainerID != nil && *svc.ContainerID != "" {
		_ = h.containers.StopContainer(r.Context(), *svc.ContainerID)
		_ = h.containers.RemoveContainer(r.Context(), *svc.ContainerID)
	}

	if err := h.queries.DeleteService(r.Context(), serviceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete service")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "service deleted"})
}

// Deployments handles GET /api/services/{serviceID}/deployments
func (h *ServiceHandler) Deployments(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	svc, err := h.queries.GetServiceByID(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	if role != "admin" && svc.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	deployments, err := h.queries.ListDeploymentsByService(r.Context(), serviceID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deployments")
		return
	}

	writeJSON(w, http.StatusOK, deployments)
}

// Deploy handles POST /api/services/{serviceID}/deploy
func (h *ServiceHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	svc, err := h.queries.GetServiceByID(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	if role != "admin" && svc.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	trigger := "manual"
	commitMsg := "Manual deployment triggered from control panel"
	imageTag := fmt.Sprintf("klouds/%s:manual", svc.Slug)

	dep, err := h.queries.CreateDeployment(r.Context(), db.CreateDeploymentParams{
		ServiceID: svc.ID,
		UserID:    svc.UserID,
		Status:    "building",
		CommitMsg: &commitMsg,
		ImageTag:  imageTag,
		Trigger:   trigger,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment record")
		return
	}

	if h.engine != nil && h.deployer != nil {
		go func() {
			ctx := context.Background()
			repoURL := ""
			if svc.RepoURL != nil {
				repoURL = *svc.RepoURL
			}
			branch := "main"
			if svc.Branch != nil && *svc.Branch != "" {
				branch = *svc.Branch
			}

			buildRes, err := h.engine.Build(ctx, builder.BuildOptions{
				DeploymentID: dep.ID,
				ServiceID:    svc.ID,
				ServiceSlug:  svc.Slug,
				RepoURL:      repoURL,
				Branch:       branch,
				BuildMethod:  svc.BuildMethod,
			})
			if err != nil {
				log.Error().Err(err).Str("deployment", dep.ID).Msg("Build failed")
				_, _ = h.queries.UpdateDeploymentFinished(ctx, dep.ID, "failed", 0)
				return
			}

			_ = h.deployer.Deploy(ctx, builder.DeployRequest{
				ServiceID:    svc.ID,
				DeploymentID: dep.ID,
				ImageTag:     buildRes.ImageTag,
			})
		}()
	}

	writeJSON(w, http.StatusAccepted, dep)
}

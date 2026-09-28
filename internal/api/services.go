package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/builder"
	"github.com/vedant/klouds/internal/caddy"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
	"github.com/vedant/klouds/internal/secrets"
)

// ServiceHandler handles service CRUD and lifecycle operations.
type ServiceHandler struct {
	queries    *db.Queries
	pool       *pgxpool.Pool
	containers *container.Manager
	engine     *builder.Engine
	deployer   *builder.Deployer
	encryptor  *secrets.Encryptor
	caddy      *caddy.Manager
	domain     string // Base domain for subdomain routing
}

// NewServiceHandler creates a new service handler.
func NewServiceHandler(pool *pgxpool.Pool, containers *container.Manager, engine *builder.Engine, deployer *builder.Deployer, encryptor *secrets.Encryptor, caddyMgr *caddy.Manager, domain string) *ServiceHandler {
	return &ServiceHandler{
		queries:    db.New(pool),
		pool:       pool,
		containers: containers,
		engine:     engine,
		deployer:   deployer,
		encryptor:  encryptor,
		caddy:      caddyMgr,
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
	RuntimeVersion  string  `json:"runtime_version"`
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

	port := req.Port
	if port <= 0 {
		port = 3000
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
		Port:            port,
		HealthCheckPath: req.HealthCheckPath,
		AutoDeploy:      req.AutoDeploy,
		Subdomain:       subdomain,
		RuntimeVersion:  req.RuntimeVersion,
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

	for i := range services {
		if services[i].Port <= 0 {
			services[i].Port = 3000
		}
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

	if svc.Port <= 0 {
		svc.Port = 3000
	}

	writeJSON(w, http.StatusOK, svc)
}

// UpdateServiceRequest is the JSON payload for updating service settings.
type UpdateServiceRequest struct {
	Name            *string           `json:"name"`
	Type            *string           `json:"type"`
	BuildMethod     *string           `json:"build_method"`
	RepoURL         *string           `json:"repo_url"`
	Branch          *string           `json:"branch"`
	RootDirectory   *string           `json:"root_directory"`
	DockerfilePath  *string           `json:"dockerfile_path"`
	BuildCommand    *string           `json:"build_command"`
	StartCommand    *string           `json:"start_command"`
	Port            *int32            `json:"port"`
	HealthCheckPath *string           `json:"health_check_path"`
	AutoDeploy      *bool             `json:"auto_deploy"`
	RuntimeVersion  *string           `json:"runtime_version"`
	EnvVars         map[string]string `json:"env_vars"`
}

// Update handles PUT /api/services/{serviceID} and PATCH /api/services/{serviceID}
func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateServiceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := svc.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}

	svcType := svc.Type
	if req.Type != nil && strings.TrimSpace(*req.Type) != "" {
		svcType = strings.TrimSpace(*req.Type)
	}

	buildMethod := svc.BuildMethod
	if req.BuildMethod != nil && strings.TrimSpace(*req.BuildMethod) != "" {
		buildMethod = strings.TrimSpace(*req.BuildMethod)
	}

	repoURL := svc.RepoURL
	if req.RepoURL != nil {
		trimmed := strings.TrimSpace(*req.RepoURL)
		repoURL = &trimmed
	}

	branch := svc.Branch
	if req.Branch != nil {
		trimmed := strings.TrimSpace(*req.Branch)
		branch = &trimmed
	}

	rootDir := svc.RootDirectory
	if req.RootDirectory != nil && strings.TrimSpace(*req.RootDirectory) != "" {
		rootDir = strings.TrimSpace(*req.RootDirectory)
	}

	dockerfilePath := svc.DockerfilePath
	if req.DockerfilePath != nil {
		trimmed := strings.TrimSpace(*req.DockerfilePath)
		dockerfilePath = &trimmed
	}

	buildCmd := svc.BuildCommand
	if req.BuildCommand != nil {
		trimmed := strings.TrimSpace(*req.BuildCommand)
		buildCmd = &trimmed
	}

	startCmd := svc.StartCommand
	if req.StartCommand != nil {
		trimmed := strings.TrimSpace(*req.StartCommand)
		startCmd = &trimmed
	}

	port := svc.Port
	if req.Port != nil {
		port = *req.Port
	}
	if port <= 0 {
		port = 3000
	}

	healthCheckPath := svc.HealthCheckPath
	if req.HealthCheckPath != nil {
		trimmed := strings.TrimSpace(*req.HealthCheckPath)
		healthCheckPath = &trimmed
	}

	autoDeploy := svc.AutoDeploy
	if req.AutoDeploy != nil {
		autoDeploy = *req.AutoDeploy
	}

	runtimeVersion := svc.RuntimeVersion
	if req.RuntimeVersion != nil {
		runtimeVersion = strings.TrimSpace(*req.RuntimeVersion)
	}

	updatedSvc, err := h.queries.UpdateServiceSpec(r.Context(), db.UpdateServiceSpecParams{
		ID:              svc.ID,
		Name:            name,
		Type:            svcType,
		BuildMethod:     buildMethod,
		RepoURL:         repoURL,
		Branch:          branch,
		RootDirectory:   rootDir,
		DockerfilePath:  dockerfilePath,
		BuildCommand:    buildCmd,
		StartCommand:    startCmd,
		Port:            port,
		HealthCheckPath: healthCheckPath,
		AutoDeploy:      autoDeploy,
		RuntimeVersion:  runtimeVersion,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update service spec: %v", err))
		return
	}

	// Update env vars if provided
	if req.EnvVars != nil {
		for k, v := range req.EnvVars {
			trimmedKey := strings.TrimSpace(k)
			if trimmedKey == "" {
				continue
			}
			valEnc := v
			if h.encryptor != nil {
				if encrypted, encErr := h.encryptor.Encrypt(v); encErr == nil {
					valEnc = encrypted
				}
			}
			_, _ = h.queries.CreateEnvVar(r.Context(), db.CreateEnvVarParams{
				ServiceID:      svc.ID,
				Key:            trimmedKey,
				ValueEncrypted: valEnc,
				IsBuildTime:    true,
				IsLinked:       false,
			})
		}
	}

	writeJSON(w, http.StatusOK, updatedSvc)
}

// GetEnv handles GET /api/services/{serviceID}/env
func (h *ServiceHandler) GetEnv(w http.ResponseWriter, r *http.Request) {
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

	vars, err := h.queries.ListEnvVarsByService(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list env vars")
		return
	}

	result := make([]map[string]interface{}, 0, len(vars))
	for _, v := range vars {
		val := v.ValueEncrypted
		if h.encryptor != nil {
			if dec, err := h.encryptor.Decrypt(v.ValueEncrypted); err == nil {
				val = dec
			}
		}
		result = append(result, map[string]interface{}{
			"id":            v.ID,
			"key":           v.Key,
			"value":         val,
			"is_build_time": v.IsBuildTime,
			"is_linked":     v.IsLinked,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// SetEnv handles PUT /api/services/{serviceID}/env
func (h *ServiceHandler) SetEnv(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		EnvVars map[string]string `json:"env_vars"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Delete all existing and replace
	existing, _ := h.queries.ListEnvVarsByService(r.Context(), serviceID)
	for _, ev := range existing {
		if _, keep := req.EnvVars[ev.Key]; !keep {
			_ = h.queries.DeleteEnvVar(r.Context(), ev.ID)
		}
	}

	for k, v := range req.EnvVars {
		trimmedKey := strings.TrimSpace(k)
		if trimmedKey == "" {
			continue
		}
		valEnc := v
		if h.encryptor != nil {
			if encrypted, encErr := h.encryptor.Encrypt(v); encErr == nil {
				valEnc = encrypted
			}
		}
		_, _ = h.queries.CreateEnvVar(r.Context(), db.CreateEnvVarParams{
			ServiceID:      serviceID,
			Key:            trimmedKey,
			ValueEncrypted: valEnc,
			IsBuildTime:    true,
			IsLinked:       false,
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "environment variables updated"})
}

// RouteRuleItem represents a redirect or rewrite rule for API requests.
type RouteRuleItem struct {
	ID     string `json:"id,omitempty"`
	Type   string `json:"type"` // "redirect" or "rewrite"
	Source string `json:"source"`
	Target string `json:"target"`
	Status int32  `json:"status,omitempty"` // 301, 302
}

// SetRoutesRequest represents the payload to update service route rules.
type SetRoutesRequest struct {
	Routes []RouteRuleItem `json:"routes"`
}

// GetRoutes handles GET /api/services/{serviceID}/routes
func (h *ServiceHandler) GetRoutes(w http.ResponseWriter, r *http.Request) {
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

	rules, err := h.queries.ListRouteRulesByService(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list route rules")
		return
	}

	writeJSON(w, http.StatusOK, rules)
}

// SetRoutes handles PUT /api/services/{serviceID}/routes
func (h *ServiceHandler) SetRoutes(w http.ResponseWriter, r *http.Request) {
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

	var req SetRoutesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate rules
	for _, item := range req.Routes {
		t := strings.ToLower(strings.TrimSpace(item.Type))
		if t != "redirect" && t != "rewrite" {
			writeError(w, http.StatusBadRequest, "route rule type must be 'redirect' or 'rewrite'")
			return
		}
		if strings.TrimSpace(item.Source) == "" || strings.TrimSpace(item.Target) == "" {
			writeError(w, http.StatusBadRequest, "route rule source and target are required")
			return
		}
	}

	// Delete existing rules for this service
	if err := h.queries.DeleteRouteRulesByService(r.Context(), serviceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear existing route rules")
		return
	}

	createdRules := make([]db.RouteRule, 0, len(req.Routes))
	orderedRules := make([]caddy.OrderedRule, 0, len(req.Routes))

	for _, item := range req.Routes {
		ruleType := strings.ToLower(strings.TrimSpace(item.Type))
		src := strings.TrimSpace(item.Source)
		dst := strings.TrimSpace(item.Target)
		var statusPtr *int32
		statusCode := 0
		if ruleType == "redirect" {
			st := item.Status
			if st != 301 && st != 302 {
				st = 301
			}
			statusPtr = &st
			statusCode = int(st)
		}

		orderedRules = append(orderedRules, caddy.OrderedRule{
			Type:       ruleType,
			Source:     src,
			Target:     dst,
			StatusCode: statusCode,
		})

		rr, err := h.queries.CreateRouteRule(r.Context(), db.CreateRouteRuleParams{
			ServiceID: serviceID,
			Type:      ruleType,
			Source:    src,
			Target:    dst,
			Status:    statusPtr,
		})
		if err != nil {
			log.Error().Err(err).Msg("failed to insert route rule")
			continue
		}
		createdRules = append(createdRules, rr)
	}

	// If service has a running container and active subdomain, live-update Caddy
	if h.caddy != nil && svc.Subdomain != "" && svc.ContainerID != nil && *svc.ContainerID != "" {
		port := int(svc.Port)
		if port <= 0 {
			port = 3000
		}
		containerTarget := svc.Slug
		cRoute := caddy.Route{
			Subdomain:   svc.Subdomain,
			BackendHost: containerTarget,
			BackendPort: port,
			Rules:       orderedRules,
		}
		if err := h.caddy.AddRoute(cRoute); err != nil {
			log.Error().Err(err).Msg("failed to live update Caddy route rules")
		} else {
			log.Info().Str("service", svc.Name).Int("rules", len(createdRules)).Msg("Caddy route rules updated live")
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "route rules updated successfully",
		"routes":  createdRules,
	})
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

	port := int(svc.Port)
	if port <= 0 {
		port = 3000
	}

	envVars := map[string]string{
		"PORT": fmt.Sprintf("%d", port),
	}
	if envs, err := h.queries.ListEnvVarsByService(r.Context(), svc.ID); err == nil {
		for _, ev := range envs {
			val := ev.ValueEncrypted
			if h.encryptor != nil {
				if decrypted, decErr := h.encryptor.Decrypt(ev.ValueEncrypted); decErr == nil {
					val = decrypted
				}
			}
			envVars[ev.Key] = val
		}
	}

	containerName := fmt.Sprintf("klouds-svc-%s", svc.Slug)
	containerID, err := h.containers.CreateServiceContainer(r.Context(), container.ServiceConfig{
		Name:         containerName,
		Image:        *svc.ImageTag,
		Port:         port,
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
			branch := ""
			if svc.Branch != nil && *svc.Branch != "" {
				branch = *svc.Branch
			}

			// Load and decrypt environment variables
			envVarsMap := make(map[string]string)
			if envs, err := h.queries.ListEnvVarsByService(ctx, svc.ID); err == nil {
				for _, ev := range envs {
					val := ev.ValueEncrypted
					if h.encryptor != nil {
						if decrypted, decErr := h.encryptor.Decrypt(ev.ValueEncrypted); decErr == nil {
							val = decrypted
						}
					}
					envVarsMap[ev.Key] = val
				}
			}

			buildRes, err := h.engine.Build(ctx, builder.BuildOptions{
				DeploymentID:   dep.ID,
				ServiceID:      svc.ID,
				ServiceSlug:    svc.Slug,
				RepoURL:        repoURL,
				Branch:         branch,
				BuildMethod:    svc.BuildMethod,
				RootDir:        svc.RootDirectory,
				DockerfilePath: deref(svc.DockerfilePath),
				BuildCommand:   deref(svc.BuildCommand),
				StartCommand:   deref(svc.StartCommand),
				RuntimeVersion: svc.RuntimeVersion,
				EnvVars:        envVarsMap,
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
				EnvVars:      envVarsMap,
			})
		}()
	}

	writeJSON(w, http.StatusAccepted, dep)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// BatchCreateServiceItem defines an individual service in a multi-service batch creation request.
type BatchCreateServiceItem struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	BuildMethod     string            `json:"build_method"`
	RepoURL         *string           `json:"repo_url"`
	Branch          *string           `json:"branch"`
	RootDir         string            `json:"root_dir"`
	DockerfilePath  *string           `json:"dockerfile_path"`
	BuildCommand    *string           `json:"build_command"`
	StartCommand    *string           `json:"start_command"`
	Port            int32             `json:"port"`
	HealthCheckPath *string           `json:"health_check_path"`
	AutoDeploy      bool              `json:"auto_deploy"`
	RuntimeVersion  string            `json:"runtime_version"`
	Subdomain       string            `json:"subdomain,omitempty"`
	EnvVars         map[string]string `json:"env_vars,omitempty"`
	RouteRules      []RouteRuleItem   `json:"route_rules,omitempty"`
}

// BatchCreateRequest is the request body for POST /api/services/batch
type BatchCreateRequest struct {
	ProjectID string                   `json:"project_id"`
	Services  []BatchCreateServiceItem `json:"services"`
	Deploy    bool                     `json:"deploy"`
}

// BatchCreateResult represents the response of creating multiple services together.
type BatchCreateResult struct {
	Services    []db.Service `json:"services"`
	Deployments []string     `json:"deployments"`
}

// BatchCreate handles POST /api/services/batch
func (h *ServiceHandler) BatchCreate(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	var req BatchCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ProjectID == "" || len(req.Services) == 0 {
		writeError(w, http.StatusBadRequest, "project_id and at least one service are required")
		return
	}

	// Verify project access
	project, err := h.queries.GetProjectByID(r.Context(), req.ProjectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if role != "admin" && project.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Check quotas
	if role != "admin" {
		quota, err := h.queries.GetUserQuota(r.Context(), userID)
		if err == nil && quota.MaxServices > 0 {
			count, _ := h.queries.CountServicesByUser(r.Context(), userID)
			if count+int64(len(req.Services)) > int64(quota.MaxServices) {
				writeError(w, http.StatusForbidden, fmt.Sprintf("creating %d services exceeds your quota limit (%d/%d)", len(req.Services), count, quota.MaxServices))
				return
			}
		}
	}

	createdServices := make([]db.Service, 0, len(req.Services))
	deployIDs := make([]string, 0)

	for _, item := range req.Services {
		svcName := strings.TrimSpace(item.Name)
		if svcName == "" {
			svcName = "service-" + randomHex(3)
		}

		slug := slugify(svcName)
		if _, err := h.queries.GetServiceBySlug(r.Context(), slug); err == nil {
			slug = fmt.Sprintf("%s-%s", slug, randomHex(3))
		}

		subdomain := item.Subdomain
		if subdomain == "" {
			subdomain = slug
		}

		svcType := item.Type
		if svcType == "" {
			svcType = "web"
		}
		buildMethod := item.BuildMethod
		if buildMethod == "" {
			buildMethod = "nixpacks"
		}
		port := item.Port
		if port <= 0 {
			if svcType == "frontend" || svcType == "static" {
				port = 80
			} else {
				port = 3000
			}
		}

		rootDir := item.RootDir
		if rootDir == "" {
			rootDir = "."
		}

		cpuLimit := int32(500)
		memLimit := int64(268435456)
		if quota, qerr := h.queries.GetUserQuota(r.Context(), userID); qerr == nil {
			cpuLimit = quota.CpuPerContainer
			memLimit = quota.MemoryPerContainer
		}

		svc, err := h.queries.CreateService(r.Context(), db.CreateServiceParams{
			ProjectID:       req.ProjectID,
			UserID:          userID,
			Name:            svcName,
			Slug:            slug,
			Type:            svcType,
			BuildMethod:     buildMethod,
			RepoURL:         item.RepoURL,
			Branch:          item.Branch,
			RootDirectory:   rootDir,
			DockerfilePath:  item.DockerfilePath,
			BuildCommand:    item.BuildCommand,
			StartCommand:    item.StartCommand,
			Port:            port,
			HealthCheckPath: item.HealthCheckPath,
			AutoDeploy:      item.AutoDeploy,
			Subdomain:       subdomain,
			RuntimeVersion:  item.RuntimeVersion,
			CpuLimit:        cpuLimit,
			MemoryLimit:     memLimit,
		})
		if err != nil {
			log.Error().Err(err).Str("service", svcName).Msg("failed to create batch service")
			continue
		}

		// Save Environment Variables
		if len(item.EnvVars) > 0 {
			for k, v := range item.EnvVars {
				trimmedKey := strings.TrimSpace(k)
				if trimmedKey == "" {
					continue
				}
				valEnc := v
				if h.encryptor != nil {
					if enc, err := h.encryptor.Encrypt(v); err == nil {
						valEnc = enc
					}
				}
				_, _ = h.queries.CreateEnvVar(r.Context(), db.CreateEnvVarParams{
					ServiceID:      svc.ID,
					Key:            trimmedKey,
					ValueEncrypted: valEnc,
					IsBuildTime:    true,
					IsLinked:       false,
				})
			}
		}

		// Save Route Rules (Redirects / Rewrites)
		if len(item.RouteRules) > 0 {
			_ = h.queries.DeleteRouteRulesByService(r.Context(), svc.ID)
			for _, rrule := range item.RouteRules {
				if rrule.Source == "" || rrule.Target == "" {
					continue
				}
				st := rrule.Status
				if st == 0 {
					if rrule.Type == "rewrite" {
						st = 200
					} else {
						st = 301
					}
				}
				_, _ = h.queries.CreateRouteRule(r.Context(), db.CreateRouteRuleParams{
					ServiceID: svc.ID,
					Type:      rrule.Type,
					Source:    rrule.Source,
					Target:    rrule.Target,
					Status:    &st,
				})
			}
		}

		createdServices = append(createdServices, svc)

		// Trigger Deploy if requested
		if req.Deploy && (item.RepoURL != nil && *item.RepoURL != "" || item.BuildMethod == "image") {
			dep, err := h.queries.CreateDeployment(r.Context(), db.CreateDeploymentParams{
				ServiceID: svc.ID,
				UserID:    userID,
				ImageTag:  fmt.Sprintf("klouds/%s:%s", svc.Slug, "initial"),
				Trigger:   "manual",
			})
			if err == nil {
				deployIDs = append(deployIDs, dep.ID)
				_, _ = h.queries.UpdateServiceStatus(r.Context(), svc.ID, "building")

				go func(targetSvc db.Service, targetDep db.Deployment, itemEnvVars map[string]string) {
					ctx := context.Background()
					repoURL := deref(targetSvc.RepoURL)
					branch := deref(targetSvc.Branch)
					if branch == "" {
						branch = "main"
					}

					buildRes, bErr := h.engine.Build(ctx, builder.BuildOptions{
						DeploymentID:   targetDep.ID,
						ServiceID:      targetSvc.ID,
						ServiceSlug:    targetSvc.Slug,
						RepoURL:        repoURL,
						Branch:         branch,
						BuildMethod:    targetSvc.BuildMethod,
						RootDir:        targetSvc.RootDirectory,
						DockerfilePath: deref(targetSvc.DockerfilePath),
						BuildCommand:   deref(targetSvc.BuildCommand),
						StartCommand:   deref(targetSvc.StartCommand),
						RuntimeVersion: targetSvc.RuntimeVersion,
						EnvVars:        itemEnvVars,
					})
					if bErr != nil {
						log.Error().Err(bErr).Str("service", targetSvc.Name).Msg("Batch build failed")
						_, _ = h.queries.UpdateDeploymentFinished(ctx, targetDep.ID, "failed", 0)
						return
					}

					_ = h.deployer.Deploy(ctx, builder.DeployRequest{
						ServiceID:    targetSvc.ID,
						DeploymentID: targetDep.ID,
						ImageTag:     buildRes.ImageTag,
						EnvVars:      itemEnvVars,
					})
				}(svc, dep, item.EnvVars)
			}
		}
	}

	writeJSON(w, http.StatusCreated, BatchCreateResult{
		Services:    createdServices,
		Deployments: deployIDs,
	})
}


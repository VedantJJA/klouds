package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/cleaner"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
	"github.com/vedant/klouds/internal/minecraft"
	"github.com/vedant/klouds/internal/secrets"
)

// MinecraftHandler handles Minecraft game server operations.
type MinecraftHandler struct {
	queries    *db.Queries
	pool       *pgxpool.Pool
	containers *container.Manager
	encryptor  *secrets.Encryptor
	domain     string
	dataDir    string
	cleaner    *cleaner.Cleaner
}

// NewMinecraftHandler creates a new Minecraft handler.
func NewMinecraftHandler(pool *pgxpool.Pool, containers *container.Manager, enc *secrets.Encryptor, domain, dataDir string, cln *cleaner.Cleaner) *MinecraftHandler {
	return &MinecraftHandler{
		queries:    db.New(pool),
		pool:       pool,
		containers: containers,
		encryptor:  enc,
		domain:     domain,
		dataDir:    dataDir,
		cleaner:    cln,
	}
}

// CreateMinecraftServerRequest is the request body for provisioning a Minecraft server.
type CreateMinecraftServerRequest struct {
	ProjectID        string `json:"project_id"`
	Name             string `json:"name"`
	ServerType       string `json:"server_type"` // PAPER, VANILLA, FABRIC, FORGE, PURPUR, SPIGOT, BEDROCK
	Version          string `json:"version"`     // 1.21.4, LATEST, etc.
	ServerPort       int32  `json:"server_port"` // Optional, 0 for auto-assign
	MOTD             string `json:"motd"`
	Difficulty       string `json:"difficulty"`   // peaceful, easy, normal, hard
	GameMode         string `json:"game_mode"`    // survival, creative, adventure, spectator
	MaxPlayers       int32  `json:"max_players"`
	PVP              *bool  `json:"pvp"`
	OnlineMode       *bool  `json:"online_mode"`
	WhitelistEnabled *bool  `json:"whitelist_enabled"`
	MemoryLimit      int64  `json:"memory_limit"` // In bytes (e.g. 2147483648 for 2GB)
	CpuLimit         int32  `json:"cpu_limit"`    // In millicores
	Ops              string `json:"ops"`
}

// UpdateMinecraftServerRequest is the request body for modifying configuration.
type UpdateMinecraftServerRequest struct {
	Name             string `json:"name"`
	MOTD             string `json:"motd"`
	Difficulty       string `json:"difficulty"`
	GameMode         string `json:"game_mode"`
	MaxPlayers       int32  `json:"max_players"`
	PVP              bool   `json:"pvp"`
	OnlineMode       bool   `json:"online_mode"`
	WhitelistEnabled bool   `json:"whitelist_enabled"`
	ServerType       string `json:"server_type"`
	Version          string `json:"version"`
	CpuLimit         int32  `json:"cpu_limit"`
	MemoryLimit      int64  `json:"memory_limit"`
	Ops              string `json:"ops"`
}

// SendCommandRequest is the request body for running a console command.
type SendCommandRequest struct {
	Command string `json:"command"`
}

// UpdatePropertiesRequest is the request body for updating server.properties.
type UpdatePropertiesRequest struct {
	Content string `json:"content"`
}

// Create handles POST /api/minecraft
func (h *MinecraftHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	var req CreateMinecraftServerRequest
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
			count, _ := h.queries.CountMinecraftServersByUser(r.Context(), userID)
			maxAllowed := int64(quota.MaxServices)
			if quota.MaxMinecraftServers > 0 {
				maxAllowed = int64(quota.MaxMinecraftServers)
			}
			if count >= maxAllowed {
				writeError(w, http.StatusForbidden, fmt.Sprintf("minecraft server limit reached (%d/%d)", count, maxAllowed))
				return
			}
		}
	}

	// Defaults & normalization
	serverType := strings.ToUpper(strings.TrimSpace(req.ServerType))
	switch serverType {
	case "PAPER", "PURPUR", "SPIGOT", "VANILLA", "FABRIC", "FORGE", "BEDROCK":
		// valid
	default:
		serverType = "PAPER"
	}

	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = "1.21.4"
	}

	difficulty := strings.ToLower(strings.TrimSpace(req.Difficulty))
	switch difficulty {
	case "peaceful", "easy", "normal", "hard":
	default:
		difficulty = "normal"
	}

	gameMode := strings.ToLower(strings.TrimSpace(req.GameMode))
	switch gameMode {
	case "survival", "creative", "adventure", "spectator":
	default:
		gameMode = "survival"
	}

	maxPlayers := req.MaxPlayers
	if maxPlayers <= 0 {
		maxPlayers = 20
	}

	pvp := true
	if req.PVP != nil {
		pvp = *req.PVP
	}

	onlineMode := true
	if req.OnlineMode != nil {
		onlineMode = *req.OnlineMode
	}

	whitelist := false
	if req.WhitelistEnabled != nil {
		whitelist = *req.WhitelistEnabled
	}

	motd := strings.TrimSpace(req.MOTD)
	if motd == "" {
		motd = fmt.Sprintf("Welcome to %s - Powered by Klouds", req.Name)
	}

	cpuLimit := req.CpuLimit
	if cpuLimit <= 0 {
		cpuLimit = 1000 // 1 CPU
	}

	memoryLimit := req.MemoryLimit
	if memoryLimit <= 0 {
		memoryLimit = 2 * 1024 * 1024 * 1024 // 2GB default
	}

	// Port allocation
	allocatedPorts, _ := h.queries.GetAllocatedMinecraftPorts(r.Context())
	allocatedSet := make(map[int32]bool)
	for _, p := range allocatedPorts {
		allocatedSet[p] = true
	}

	var assignedPort int32
	if req.ServerPort > 0 {
		if req.ServerPort < 1024 || req.ServerPort > 65535 {
			writeError(w, http.StatusBadRequest, "server_port must be between 1024 and 65535")
			return
		}
		if allocatedSet[req.ServerPort] {
			writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already in use by another Minecraft server", req.ServerPort))
			return
		}
		assignedPort = req.ServerPort
	} else {
		// Auto-assign first free port >= 25565
		p := int32(25565)
		for allocatedSet[p] {
			p++
		}
		assignedPort = p
	}

	rconPort := int32(25575)
	rconPassword := generatePassword(16)
	idSuffix := generateRandomID(6)
	slug := fmt.Sprintf("%s-%s", slugify(req.Name), idSuffix)

	// Create persistent data directory on host
	serverDataDir := filepath.Join(h.dataDir, "minecraft", slug)
	if err := os.MkdirAll(serverDataDir, 0755); err != nil {
		log.Warn().Err(err).Str("dir", serverDataDir).Msg("Failed to create minecraft server data dir")
	}

	// Insert database record
	srv, err := h.queries.CreateMinecraftServer(r.Context(), db.CreateMinecraftServerParams{
		ProjectID:        req.ProjectID,
		UserID:           userID,
		Name:             req.Name,
		Slug:             slug,
		ServerType:       serverType,
		Version:          version,
		Status:           "creating",
		ContainerID:      nil,
		ServerPort:       assignedPort,
		RconPort:         rconPort,
		RconPassword:     rconPassword,
		MOTD:             motd,
		Difficulty:       difficulty,
		GameMode:         gameMode,
		MaxPlayers:       maxPlayers,
		PVP:              pvp,
		OnlineMode:       onlineMode,
		WhitelistEnabled: whitelist,
		CpuLimit:         cpuLimit,
		MemoryLimit:      memoryLimit,
		DiskLimit:        10 * 1024 * 1024 * 1024,
		Ops:              req.Ops,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create minecraft server record")
		writeError(w, http.StatusInternalServerError, "failed to create server record")
		return
	}

	// Spin up Docker container asynchronously / synchronously
	if h.containers != nil {
		go func(srvRecord db.MinecraftServer, dataDir string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			containerName := fmt.Sprintf("klouds-mc-%s", srvRecord.Slug)
			cid, err := h.containers.CreateMinecraftContainer(ctx, container.MinecraftConfig{
				Name:         containerName,
				ServerType:   srvRecord.ServerType,
				Version:      srvRecord.Version,
				ServerPort:   int(srvRecord.ServerPort),
				RconPort:     int(srvRecord.RconPort),
				RconPassword: srvRecord.RconPassword,
				MOTD:         srvRecord.MOTD,
				Difficulty:   srvRecord.Difficulty,
				GameMode:     srvRecord.GameMode,
				MaxPlayers:   int(srvRecord.MaxPlayers),
				PVP:          srvRecord.PVP,
				OnlineMode:   srvRecord.OnlineMode,
				Whitelist:    srvRecord.WhitelistEnabled,
				Ops:          srvRecord.Ops,
				CPULimit:     int64(srvRecord.CpuLimit),
				MemoryLimit:  srvRecord.MemoryLimit,
				DataVolume:   dataDir,
				NetworkAlias: srvRecord.Slug,
				Labels: map[string]string{
					"klouds.minecraft.id": srvRecord.ID,
				},
			})
			if err != nil {
				log.Error().Err(err).Str("id", srvRecord.ID).Msg("Failed to start minecraft container")
				_, _ = h.queries.UpdateMinecraftServerStatus(ctx, srvRecord.ID, "failed")
				return
			}

			_, _ = h.queries.UpdateMinecraftServerContainer(ctx, srvRecord.ID, &cid, "running")
		}(srv, serverDataDir)
	}

	writeJSON(w, http.StatusCreated, srv)
}

// List handles GET /api/minecraft
func (h *MinecraftHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())
	projectID := r.URL.Query().Get("project_id")

	var servers []db.MinecraftServer
	var err error

	if projectID != "" {
		servers, err = h.queries.ListMinecraftServersByProject(r.Context(), projectID)
	} else if role == "admin" {
		servers, err = h.queries.ListAllMinecraftServers(r.Context())
	} else {
		servers, err = h.queries.ListMinecraftServersByUser(r.Context(), userID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query servers")
		return
	}

	// Sync live container status
	if h.containers != nil {
		for i, s := range servers {
			if s.ContainerID != nil && *s.ContainerID != "" {
				running := h.containers.IsRunning(r.Context(), *s.ContainerID)
				if running && s.Status != "running" {
					servers[i].Status = "running"
					_, _ = h.queries.UpdateMinecraftServerStatus(r.Context(), s.ID, "running")
				} else if !running && s.Status == "running" {
					servers[i].Status = "stopped"
					_, _ = h.queries.UpdateMinecraftServerStatus(r.Context(), s.ID, "stopped")
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, servers)
}

// Get handles GET /api/minecraft/{id}
func (h *MinecraftHandler) Get(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	if h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" {
		running := h.containers.IsRunning(r.Context(), *srv.ContainerID)
		if running && srv.Status != "running" {
			srv.Status = "running"
			_, _ = h.queries.UpdateMinecraftServerStatus(r.Context(), srv.ID, "running")
		} else if !running && srv.Status == "running" {
			srv.Status = "stopped"
			_, _ = h.queries.UpdateMinecraftServerStatus(r.Context(), srv.ID, "stopped")
		}
	}

	writeJSON(w, http.StatusOK, srv)
}

// Update handles PUT /api/minecraft/{id}
func (h *MinecraftHandler) Update(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	var req UpdateMinecraftServerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = srv.Name
	}

	motd := strings.TrimSpace(req.MOTD)
	if motd == "" {
		motd = srv.MOTD
	}

	diff := strings.ToLower(strings.TrimSpace(req.Difficulty))
	if diff == "" {
		diff = srv.Difficulty
	}

	mode := strings.ToLower(strings.TrimSpace(req.GameMode))
	if mode == "" {
		mode = srv.GameMode
	}

	maxPlayers := req.MaxPlayers
	if maxPlayers <= 0 {
		maxPlayers = srv.MaxPlayers
	}

	stype := strings.ToUpper(strings.TrimSpace(req.ServerType))
	if stype == "" {
		stype = srv.ServerType
	}

	ver := strings.TrimSpace(req.Version)
	if ver == "" {
		ver = srv.Version
	}

	cpuLimit := req.CpuLimit
	if cpuLimit <= 0 {
		cpuLimit = srv.CpuLimit
	}

	memLimit := req.MemoryLimit
	if memLimit <= 0 {
		memLimit = srv.MemoryLimit
	}

	updated, err := h.queries.UpdateMinecraftServerConfig(r.Context(), db.UpdateMinecraftServerConfigParams{
		ID:               srv.ID,
		Name:             name,
		MOTD:             motd,
		Difficulty:       diff,
		GameMode:         mode,
		MaxPlayers:       maxPlayers,
		PVP:              req.PVP,
		OnlineMode:       req.OnlineMode,
		WhitelistEnabled: req.WhitelistEnabled,
		ServerType:       stype,
		Version:          ver,
		CpuLimit:         cpuLimit,
		MemoryLimit:      memLimit,
		Ops:              req.Ops,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to update minecraft server config")
		writeError(w, http.StatusInternalServerError, "failed to update configuration")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// Start handles POST /api/minecraft/{id}/start
func (h *MinecraftHandler) Start(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	if h.containers == nil {
		writeError(w, http.StatusServiceUnavailable, "Docker container service unavailable")
		return
	}

	serverDataDir := filepath.Join(h.dataDir, "minecraft", srv.Slug)
	_ = os.MkdirAll(serverDataDir, 0755)

	// If container exists, check if running or start/recreate
	if srv.ContainerID != nil && *srv.ContainerID != "" {
		if h.containers.IsRunning(r.Context(), *srv.ContainerID) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "already running"})
			return
		}
		// Try to start stopped container
		err := h.startExistingOrRecreate(r.Context(), srv, serverDataDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start container: %v", err))
			return
		}
	} else {
		containerName := fmt.Sprintf("klouds-mc-%s", srv.Slug)
		cid, err := h.containers.CreateMinecraftContainer(r.Context(), container.MinecraftConfig{
			Name:         containerName,
			ServerType:   srv.ServerType,
			Version:      srv.Version,
			ServerPort:   int(srv.ServerPort),
			RconPort:     int(srv.RconPort),
			RconPassword: srv.RconPassword,
			MOTD:         srv.MOTD,
			Difficulty:   srv.Difficulty,
			GameMode:     srv.GameMode,
			MaxPlayers:   int(srv.MaxPlayers),
			PVP:          srv.PVP,
			OnlineMode:   srv.OnlineMode,
			Whitelist:    srv.WhitelistEnabled,
			Ops:          srv.Ops,
			CPULimit:     int64(srv.CpuLimit),
			MemoryLimit:  srv.MemoryLimit,
			DataVolume:   serverDataDir,
			NetworkAlias: srv.Slug,
			Labels: map[string]string{
				"klouds.minecraft.id": srv.ID,
			},
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create container: %v", err))
			return
		}
		srv, _ = h.queries.UpdateMinecraftServerContainer(r.Context(), srv.ID, &cid, "running")
	}

	_, _ = h.queries.UpdateMinecraftServerStatus(r.Context(), srv.ID, "running")
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

// Stop handles POST /api/minecraft/{id}/stop
func (h *MinecraftHandler) Stop(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	if h.containers == nil || srv.ContainerID == nil || *srv.ContainerID == "" {
		_, _ = h.queries.UpdateMinecraftServerStatus(r.Context(), srv.ID, "stopped")
		writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
		return
	}

	// Try graceful stop via RCON "stop" command first
	if srv.RconPassword != "" {
		_, _ = h.executeConsoleCommand(r.Context(), srv, "stop")
	}

	// Stop Docker container
	_ = h.containers.StopContainer(r.Context(), *srv.ContainerID)
	_, _ = h.queries.UpdateMinecraftServerStatus(r.Context(), srv.ID, "stopped")

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// Restart handles POST /api/minecraft/{id}/restart
func (h *MinecraftHandler) Restart(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	if h.containers == nil {
		writeError(w, http.StatusServiceUnavailable, "Docker unavailable")
		return
	}

	serverDataDir := filepath.Join(h.dataDir, "minecraft", srv.Slug)

	// Stop container if running
	if srv.ContainerID != nil && *srv.ContainerID != "" {
		_, _ = h.executeConsoleCommand(r.Context(), srv, "stop")
		_ = h.containers.StopContainer(r.Context(), *srv.ContainerID)
		_ = h.containers.RemoveContainer(r.Context(), *srv.ContainerID)
	}

	// Create and start fresh container with current configuration
	containerName := fmt.Sprintf("klouds-mc-%s", srv.Slug)
	cid, err := h.containers.CreateMinecraftContainer(r.Context(), container.MinecraftConfig{
		Name:         containerName,
		ServerType:   srv.ServerType,
		Version:      srv.Version,
		ServerPort:   int(srv.ServerPort),
		RconPort:     int(srv.RconPort),
		RconPassword: srv.RconPassword,
		MOTD:         srv.MOTD,
		Difficulty:   srv.Difficulty,
		GameMode:     srv.GameMode,
		MaxPlayers:   int(srv.MaxPlayers),
		PVP:          srv.PVP,
		OnlineMode:   srv.OnlineMode,
		Whitelist:    srv.WhitelistEnabled,
		Ops:          srv.Ops,
		CPULimit:     int64(srv.CpuLimit),
		MemoryLimit:  srv.MemoryLimit,
		DataVolume:   serverDataDir,
		NetworkAlias: srv.Slug,
		Labels: map[string]string{
			"klouds.minecraft.id": srv.ID,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to restart container: %v", err))
		return
	}

	_, _ = h.queries.UpdateMinecraftServerContainer(r.Context(), srv.ID, &cid, "running")
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

// Delete handles DELETE /api/minecraft/{id}
func (h *MinecraftHandler) Delete(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	if h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" {
		_ = h.containers.StopContainer(r.Context(), *srv.ContainerID)
		_ = h.containers.RemoveContainer(r.Context(), *srv.ContainerID)
	}

	// Clean up data folder on disk
	serverDataDir := filepath.Join(h.dataDir, "minecraft", srv.Slug)
	_ = os.RemoveAll(serverDataDir)

	if err := h.queries.DeleteMinecraftServer(r.Context(), srv.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete server record")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Logs handles GET /api/minecraft/{id}/logs
func (h *MinecraftHandler) Logs(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	if h.containers == nil || srv.ContainerID == nil || *srv.ContainerID == "" {
		writeJSON(w, http.StatusOK, map[string]string{"logs": "Server container is not running."})
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "250"
	}

	logs, err := h.containers.GetContainerLogsString(r.Context(), *srv.ContainerID, tail)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"logs": fmt.Sprintf("Error retrieving logs: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

// Command handles POST /api/minecraft/{id}/command
func (h *MinecraftHandler) Command(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	var req SendCommandRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Command) == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	output, err := h.executeConsoleCommand(r.Context(), srv, req.Command)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"command":  req.Command,
			"response": fmt.Sprintf("Command executed (no response or error: %v)", err),
			"error":    err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"command":  req.Command,
		"response": output,
	})
}

// Properties handles GET /api/minecraft/{id}/properties
func (h *MinecraftHandler) Properties(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	propsFile := filepath.Join(h.dataDir, "minecraft", srv.Slug, "server.properties")
	content, err := os.ReadFile(propsFile)
	if err != nil {
		// Generate standard server.properties if not yet created on disk
		content = []byte(fmt.Sprintf(`# Minecraft server properties - Klouds Managed
server-port=%d
motd=%s
difficulty=%s
gamemode=%s
max-players=%d
pvp=%t
online-mode=%t
white-list=%t
enable-rcon=true
rcon.port=%d
view-distance=10
simulation-distance=10
spawn-protection=16
`, srv.ServerPort, srv.MOTD, srv.Difficulty, srv.GameMode, srv.MaxPlayers, srv.PVP, srv.OnlineMode, srv.WhitelistEnabled, srv.RconPort))
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"content": string(content),
	})
}

// UpdateProperties handles PUT /api/minecraft/{id}/properties
func (h *MinecraftHandler) UpdateProperties(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	var req UpdatePropertiesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	serverDir := filepath.Join(h.dataDir, "minecraft", srv.Slug)
	_ = os.MkdirAll(serverDir, 0755)
	propsFile := filepath.Join(serverDir, "server.properties")

	if err := os.WriteFile(propsFile, []byte(req.Content), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save properties file")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// Players handles GET /api/minecraft/{id}/players
func (h *MinecraftHandler) Players(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		return
	}

	opsList := []string{}
	if srv.Ops != "" {
		for _, o := range strings.Split(srv.Ops, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				opsList = append(opsList, trimmed)
			}
		}
	}

	resp := map[string]interface{}{
		"server_id":         srv.ID,
		"max_players":       srv.MaxPlayers,
		"current_players":   0,
		"online_players":    []string{},
		"ops":               opsList,
		"whitelist_enabled": srv.WhitelistEnabled,
	}

	if h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" && h.containers.IsRunning(r.Context(), *srv.ContainerID) {
		output, err := h.executeConsoleCommand(r.Context(), srv, "list")
		if err == nil && output != "" {
			info := minecraft.ParsePlayerList(output)
			resp["current_players"] = info.Current
			if info.Max > 0 {
				resp["max_players"] = info.Max
			}
			resp["online_players"] = info.OnlinePlayers
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Internal Helpers ---

func (h *MinecraftHandler) getAndVerifyServer(r *http.Request, id string) (db.MinecraftServer, error) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	srv, err := h.queries.GetMinecraftServerByID(r.Context(), id)
	if err != nil {
		return srv, err
	}
	if role != "admin" && srv.UserID != userID {
		return srv, fmt.Errorf("access denied")
	}
	return srv, nil
}

func (h *MinecraftHandler) startExistingOrRecreate(ctx context.Context, srv db.MinecraftServer, dataDir string) error {
	containerName := fmt.Sprintf("klouds-mc-%s", srv.Slug)
	_ = h.containers.RemoveContainer(ctx, *srv.ContainerID)

	cid, err := h.containers.CreateMinecraftContainer(ctx, container.MinecraftConfig{
		Name:         containerName,
		ServerType:   srv.ServerType,
		Version:      srv.Version,
		ServerPort:   int(srv.ServerPort),
		RconPort:     int(srv.RconPort),
		RconPassword: srv.RconPassword,
		MOTD:         srv.MOTD,
		Difficulty:   srv.Difficulty,
		GameMode:     srv.GameMode,
		MaxPlayers:   int(srv.MaxPlayers),
		PVP:          srv.PVP,
		OnlineMode:   srv.OnlineMode,
		Whitelist:    srv.WhitelistEnabled,
		Ops:          srv.Ops,
		CPULimit:     int64(srv.CpuLimit),
		MemoryLimit:  srv.MemoryLimit,
		DataVolume:   dataDir,
		NetworkAlias: srv.Slug,
		Labels: map[string]string{
			"klouds.minecraft.id": srv.ID,
		},
	})
	if err != nil {
		return err
	}

	_, _ = h.queries.UpdateMinecraftServerContainer(ctx, srv.ID, &cid, "running")
	return nil
}

func (h *MinecraftHandler) executeConsoleCommand(ctx context.Context, srv db.MinecraftServer, command string) (string, error) {
	if h.containers == nil || srv.ContainerID == nil || *srv.ContainerID == "" {
		return "", fmt.Errorf("server container is not active")
	}

	// 1. Try pure Go RCON client first
	ip, err := h.containers.GetContainerIP(ctx, *srv.ContainerID)
	if err == nil && ip != "" && srv.RconPassword != "" {
		addr := fmt.Sprintf("%s:%d", ip, srv.RconPort)
		res, rconErr := minecraft.SendRconCommand(addr, srv.RconPassword, command, 3*time.Second)
		if rconErr == nil {
			return res, nil
		}
	}

	// 2. Fallback to docker exec: rcon-cli <command>
	cmdSlice := []string{"rcon-cli", command}
	return h.containers.ExecCommand(ctx, *srv.ContainerID, cmdSlice)
}

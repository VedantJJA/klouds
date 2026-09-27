package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
	"github.com/vedant/klouds/internal/secrets"
)

// DatabaseHandler handles managed database operations.
type DatabaseHandler struct {
	queries    *db.Queries
	pool       *pgxpool.Pool
	containers *container.Manager
	encryptor  *secrets.Encryptor
	domain     string
	dataDir    string
}

// NewDatabaseHandler creates a new database handler.
func NewDatabaseHandler(pool *pgxpool.Pool, containers *container.Manager, enc *secrets.Encryptor, domain, dataDir string) *DatabaseHandler {
	return &DatabaseHandler{
		queries:    db.New(pool),
		pool:       pool,
		containers: containers,
		encryptor:  enc,
		domain:     domain,
		dataDir:    dataDir,
	}
}

// CreateDatabaseRequest is the JSON body for creating a managed database.
type CreateDatabaseRequest struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Engine    string `json:"engine"`  // postgresql, mysql, redis, mongodb
	Version   string `json:"version"` // e.g. "16", "8.0", "7"
}

// Create handles POST /api/databases
func (h *DatabaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	var req CreateDatabaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.ProjectID == "" || req.Engine == "" {
		writeError(w, http.StatusBadRequest, "name, project_id, and engine are required")
		return
	}

	// Validate engine
	validEngines := map[string]bool{"postgresql": true, "mysql": true, "redis": true, "mongodb": true}
	if !validEngines[req.Engine] {
		writeError(w, http.StatusBadRequest, "engine must be postgresql, mysql, redis, or mongodb")
		return
	}

	// Default versions
	if req.Version == "" {
		switch req.Engine {
		case "postgresql":
			req.Version = "16"
		case "mysql":
			req.Version = "8.0"
		case "redis":
			req.Version = "7"
		case "mongodb":
			req.Version = "7"
		}
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
		if err == nil && quota.MaxDatabases > 0 {
			count, _ := h.queries.CountDatabasesByUser(r.Context(), userID)
			if count >= int64(quota.MaxDatabases) {
				writeError(w, http.StatusForbidden, fmt.Sprintf("database limit reached (%d/%d)", count, quota.MaxDatabases))
				return
			}
		}
	}

	slug := slugify(req.Name)
	dbName := fmt.Sprintf("klouds_%s", slug)
	username := fmt.Sprintf("klouds_%s", slug)
	password := generatePassword(24)

	// Encrypt password for storage
	encPassword, err := h.encryptor.Encrypt(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt password")
		return
	}

	// Determine internal port
	portMap := map[string]int32{"postgresql": 5432, "mysql": 3306, "redis": 6379, "mongodb": 27017}
	internalPort := portMap[req.Engine]

	// Internal hostname on Docker network
	internalHost := fmt.Sprintf("klouds-db-%s", slug)

	// Get resource limits
	var cpuLimit int32 = 500
	var memLimit int64 = 256 * 1024 * 1024
	var diskLimit int64 = 2 * 1024 * 1024 * 1024
	if role != "admin" {
		quota, err := h.queries.GetUserQuota(r.Context(), userID)
		if err == nil {
			cpuLimit = quota.CpuPerContainer
			memLimit = quota.MemoryPerContainer
			diskLimit = quota.DiskPerDatabase
		}
	} else {
		cpuLimit = 2000
		memLimit = 2 * 1024 * 1024 * 1024
		diskLimit = 20 * 1024 * 1024 * 1024
	}

	// Save to DB first
	dbRecord, err := h.queries.CreateDatabase(r.Context(), db.CreateDatabaseParams{
		ProjectID:         req.ProjectID,
		UserID:            userID,
		Name:              req.Name,
		Slug:              slug,
		Engine:            req.Engine,
		Version:           req.Version,
		InternalHost:      internalHost,
		InternalPort:      internalPort,
		DatabaseName:      dbName,
		Username:          username,
		PasswordEncrypted: encPassword,
		CpuLimit:          cpuLimit,
		MemoryLimit:       memLimit,
		DiskLimit:         diskLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create database record")
		writeError(w, http.StatusInternalServerError, "failed to create database")
		return
	}

	// Create container in background
	go h.provisionDatabase(dbRecord, password)

	writeJSON(w, http.StatusCreated, dbRecord)
}

// List handles GET /api/databases?project_id=xxx
func (h *DatabaseHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())
	projectID := r.URL.Query().Get("project_id")

	var databases []db.Database
	var err error

	if projectID != "" {
		project, perr := h.queries.GetProjectByID(r.Context(), projectID)
		if perr != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		if role != "admin" && project.UserID != userID {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		databases, err = h.queries.ListDatabasesByProject(r.Context(), projectID)
	} else if role == "admin" {
		databases, err = h.queries.ListAllDatabases(r.Context())
	} else {
		databases, err = h.queries.ListDatabasesByUser(r.Context(), userID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list databases")
		return
	}

	writeJSON(w, http.StatusOK, databases)
}

// Get handles GET /api/databases/{databaseID}
func (h *DatabaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	dbRecord, err := h.queries.GetDatabaseByID(r.Context(), databaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	if role != "admin" && dbRecord.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	writeJSON(w, http.StatusOK, dbRecord)
}

// ConnectionInfo handles GET /api/databases/{databaseID}/connection
// Returns the internal connection string (and external if enabled).
func (h *DatabaseHandler) ConnectionInfo(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	dbRecord, err := h.queries.GetDatabaseByID(r.Context(), databaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	if role != "admin" && dbRecord.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Decrypt password
	password, err := h.encryptor.Decrypt(dbRecord.PasswordEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt credentials")
		return
	}

	info := map[string]interface{}{
		"host":     dbRecord.InternalHost,
		"port":     dbRecord.InternalPort,
		"database": dbRecord.DatabaseName,
		"username": dbRecord.Username,
		"password": password,
	}

	// Build connection string based on engine
	switch dbRecord.Engine {
	case "postgresql":
		info["connection_string"] = fmt.Sprintf(
			"postgresql://%s:%s@%s:%d/%s",
			dbRecord.Username, password, dbRecord.InternalHost, dbRecord.InternalPort, dbRecord.DatabaseName,
		)
	case "mysql":
		info["connection_string"] = fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s",
			dbRecord.Username, password, dbRecord.InternalHost, dbRecord.InternalPort, dbRecord.DatabaseName,
		)
	case "redis":
		info["connection_string"] = fmt.Sprintf(
			"redis://:%s@%s:%d",
			password, dbRecord.InternalHost, dbRecord.InternalPort,
		)
	case "mongodb":
		info["connection_string"] = fmt.Sprintf(
			"mongodb://%s:%s@%s:%d/%s",
			dbRecord.Username, password, dbRecord.InternalHost, dbRecord.InternalPort, dbRecord.DatabaseName,
		)
	}

	if dbRecord.ExternalAccess && dbRecord.ExternalSubdomain != nil {
		info["external_host"] = fmt.Sprintf("%s.%s", *dbRecord.ExternalSubdomain, h.domain)
		info["external_port"] = 443
	}

	writeJSON(w, http.StatusOK, info)
}

// Delete handles DELETE /api/databases/{databaseID}
func (h *DatabaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	databaseID := chi.URLParam(r, "databaseID")
	userID := getUserID(r.Context())
	role := getUserRole(r.Context())

	dbRecord, err := h.queries.GetDatabaseByID(r.Context(), databaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}

	if role != "admin" && dbRecord.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Stop and remove container
	if dbRecord.ContainerID != nil && *dbRecord.ContainerID != "" {
		_ = h.containers.StopContainer(r.Context(), *dbRecord.ContainerID)
		_ = h.containers.RemoveContainer(r.Context(), *dbRecord.ContainerID)
	}

	if err := h.queries.DeleteDatabase(r.Context(), databaseID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete database")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "database deleted"})
}

// provisionDatabase creates the Docker container for a database (runs in background).
func (h *DatabaseHandler) provisionDatabase(dbRecord db.Database, password string) {
	ctx := context.Background()

	containerName := fmt.Sprintf("klouds-db-%s", dbRecord.Slug)
	dataVolume := fmt.Sprintf("%s/databases/%s", h.dataDir, dbRecord.Slug)

	containerID, err := h.containers.CreateDatabaseContainer(ctx, container.DatabaseConfig{
		Name:         containerName,
		Engine:       dbRecord.Engine,
		Version:      dbRecord.Version,
		DatabaseName: dbRecord.DatabaseName,
		Username:     dbRecord.Username,
		Password:     password,
		Port:         int(dbRecord.InternalPort),
		CPULimit:     int64(dbRecord.CpuLimit),
		MemoryLimit:  dbRecord.MemoryLimit,
		DiskLimit:    dbRecord.DiskLimit,
		DataVolume:   dataVolume,
		NetworkAlias: dbRecord.InternalHost,
	})
	if err != nil {
		log.Error().Err(err).Str("database", dbRecord.Name).Msg("failed to provision database")
		_, _ = h.queries.UpdateDatabaseStatus(ctx, dbRecord.ID, "failed")
		return
	}

	_, err = h.queries.UpdateDatabaseContainer(ctx, dbRecord.ID, &containerID, "running")
	if err != nil {
		log.Error().Err(err).Msg("failed to update database container info")
	}

	log.Info().
		Str("database", dbRecord.Name).
		Str("engine", dbRecord.Engine).
		Str("container", containerID[:12]).
		Msg("Database provisioned successfully")
}

// generatePassword creates a random hex password of the given byte length.
func generatePassword(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

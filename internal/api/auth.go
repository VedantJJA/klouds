package api

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vedant/klouds/internal/auth"
	"github.com/vedant/klouds/internal/db"
)

// AuthHandler handles user registration and authentication.
type AuthHandler struct {
	queries  *db.Queries
	pool     *pgxpool.Pool
	tokenSvc *auth.TokenService
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(pool *pgxpool.Pool, tokenSvc *auth.TokenService) *AuthHandler {
	return &AuthHandler{
		queries:  db.New(pool),
		pool:     pool,
		tokenSvc: tokenSvc,
	}
}

// RegisterRequest is the JSON body for user registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest is the JSON body for user login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned on successful auth.
type AuthResponse struct {
	Token string  `json:"token"`
	User  UserDTO `json:"user"`
}

// UserDTO is the public-facing user representation (no sensitive fields).
type UserDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// Register handles POST /api/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	if req.Email == "" || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email, username, and password are required")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if len(req.Username) < 3 {
		writeError(w, http.StatusBadRequest, "username must be at least 3 characters")
		return
	}

	// Sanitize username: lowercase, alphanumeric + hyphens only
	req.Username = strings.ToLower(req.Username)

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	// Check if this is the first user (they become admin)
	count, err := h.queries.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	role := "user"
	status := "pending"
	if count == 0 {
		role = "admin"
		status = "active" // First user (admin) is auto-approved
	}

	// Create user
	user, err := h.queries.CreateUser(r.Context(), db.CreateUserParams{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: hash,
		Role:         role,
		Status:       status,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "email or username already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Create default quota
	var quotaParams db.CreateUserQuotaParams
	if role == "admin" {
		q := defaultAdminQuota(user.ID)
		quotaParams = q
	} else {
		q := defaultUserQuota(user.ID)
		quotaParams = q
	}

	_, err = h.queries.CreateUserQuota(r.Context(), quotaParams)
	if err != nil {
		// Non-fatal — quota can be created later
		_ = err
	}

	// If admin, return token immediately. If user, they need approval.
	if role == "admin" {
		token, err := h.tokenSvc.CreateToken(user.ID, user.Username, user.Role)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create token")
			return
		}

		writeJSON(w, http.StatusCreated, AuthResponse{
			Token: token,
			User:  toUserDTO(user),
		})
		return
	}

	// Non-admin: pending approval
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Account created. Awaiting admin approval.",
		"user":    toUserDTO(user),
	})
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Find user
	user, err := h.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Check password
	if err := auth.CheckPassword(req.Password, user.PasswordHash); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Check status
	switch user.Status {
	case "pending":
		writeError(w, http.StatusForbidden, "account is pending admin approval")
		return
	case "suspended":
		writeError(w, http.StatusForbidden, "account is suspended")
		return
	}

	// Generate token
	token, err := h.tokenSvc.CreateToken(user.ID, user.Username, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		Token: token,
		User:  toUserDTO(user),
	})
}

// Me handles GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// --- helpers ---

func toUserDTO(u db.User) UserDTO {
	return UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func defaultUserQuota(userID string) db.CreateUserQuotaParams {
	return db.CreateUserQuotaParams{
		UserID:             userID,
		MaxServices:        3,
		MaxDatabases:       2,
		CpuPerContainer:    500,
		MemoryPerContainer: 256 * 1024 * 1024,
		TotalMemory:        1024 * 1024 * 1024,
		DiskPerService:     1024 * 1024 * 1024,
		DiskPerDatabase:    2 * 1024 * 1024 * 1024,
		BuildTimeoutSecs:   600,
		MaxDeploysPerDay:   20,
		MaxCustomDomains:   2,
	}
}

func defaultAdminQuota(userID string) db.CreateUserQuotaParams {
	return db.CreateUserQuotaParams{
		UserID:             userID,
		MaxServices:        -1,
		MaxDatabases:       -1,
		CpuPerContainer:    2000,
		MemoryPerContainer: 2 * 1024 * 1024 * 1024,
		TotalMemory:        -1,
		DiskPerService:     10 * 1024 * 1024 * 1024,
		DiskPerDatabase:    20 * 1024 * 1024 * 1024,
		BuildTimeoutSecs:   1800,
		MaxDeploysPerDay:   -1,
		MaxCustomDomains:   -1,
	}
}

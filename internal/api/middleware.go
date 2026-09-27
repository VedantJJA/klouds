// Package api provides the HTTP API for the Klouds platform.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/auth"
)

// contextKey is a private type for context keys to prevent collisions.
type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUsername  contextKey = "username"
	ctxUserRole contextKey = "user_role"
)

// AuthMiddleware validates the auth token and injects user info into context.
func AuthMiddleware(tokenSvc *auth.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				writeError(w, http.StatusUnauthorized, "missing auth token")
				return
			}

			claims, err := tokenSvc.ValidateToken(tokenStr)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Inject into context
			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxUsername, claims.Username)
			ctx = context.WithValue(ctx, ctxUserRole, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnly middleware restricts access to admin users.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := getUserRole(r.Context())
		if role != "admin" {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ActiveOnly middleware restricts access to active (approved) users.
// This is checked at the handler level since status comes from DB.
// Here we just check the token role isn't empty.

// RequestLogger logs each HTTP request.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote", r.RemoteAddr).
			Msg("request")

		next.ServeHTTP(w, r)
	})
}

// --- Context helpers ---

func getUserID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

func getUserRole(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserRole).(string); ok {
		return v
	}
	return ""
}

// --- HTTP helpers ---

func extractToken(r *http.Request) string {
	// Check Authorization header first
	bearer := r.Header.Get("Authorization")
	if strings.HasPrefix(bearer, "Bearer ") {
		return strings.TrimPrefix(bearer, "Bearer ")
	}

	// Check cookie as fallback
	cookie, err := r.Cookie("klouds_token")
	if err == nil {
		return cookie.Value
	}

	return ""
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("failed to write JSON response")
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// decodeJSON decodes a JSON request body into the given struct.
func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

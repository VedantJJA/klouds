package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vedant/klouds/internal/auth"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/secrets"
)

// RouterConfig holds dependencies for the router.
type RouterConfig struct {
	Pool       *pgxpool.Pool
	TokenSvc   *auth.TokenService
	Containers *container.Manager
	Encryptor  *secrets.Encryptor
	Domain     string
	DataDir    string
}

// NewRouter creates the main API router with all routes.
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(RequestLogger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Tighten in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check (unauthenticated)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "klouds"})
	})

	// Initialize handlers
	authHandler := NewAuthHandler(cfg.Pool, cfg.TokenSvc)
	projectHandler := NewProjectHandler(cfg.Pool)
	serviceHandler := NewServiceHandler(cfg.Pool, cfg.Containers, cfg.Domain)
	dbHandler := NewDatabaseHandler(cfg.Pool, cfg.Containers, cfg.Encryptor, cfg.Domain, cfg.DataDir)
	adminHandler := NewAdminHandler(cfg.Pool, cfg.Containers)
	metricsHandler := NewMetricsHandler()

	r.Route("/api", func(r chi.Router) {
		// --- Public routes (no auth) ---
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
		})

		// --- Authenticated routes ---
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(cfg.TokenSvc))

			// Auth
			r.Get("/auth/me", authHandler.Me)

			// Projects
			r.Post("/projects", projectHandler.Create)
			r.Get("/projects", projectHandler.List)
			r.Get("/projects/{projectID}", projectHandler.Get)
			r.Delete("/projects/{projectID}", projectHandler.Delete)

			// Services
			r.Post("/services", serviceHandler.Create)
			r.Get("/services", serviceHandler.List)
			r.Get("/services/{serviceID}", serviceHandler.Get)
			r.Post("/services/{serviceID}/stop", serviceHandler.Stop)
			r.Post("/services/{serviceID}/restart", serviceHandler.Restart)
			r.Delete("/services/{serviceID}", serviceHandler.Delete)
			r.Get("/services/{serviceID}/deployments", serviceHandler.Deployments)

			// Databases
			r.Post("/databases", dbHandler.Create)
			r.Get("/databases", dbHandler.List)
			r.Get("/databases/{databaseID}", dbHandler.Get)
			r.Get("/databases/{databaseID}/connection", dbHandler.ConnectionInfo)
			r.Delete("/databases/{databaseID}", dbHandler.Delete)

			// --- Admin routes ---
			r.Route("/admin", func(r chi.Router) {
				r.Use(AdminOnly)

				// User management
				r.Get("/users", adminHandler.ListUsers)
				r.Get("/users/pending", adminHandler.ListPendingUsers)
				r.Patch("/users/{userID}/status", adminHandler.UpdateUserStatus)
				r.Delete("/users/{userID}", adminHandler.DeleteUser)
				r.Get("/users/{userID}/quota", adminHandler.GetUserQuota)
				r.Put("/users/{userID}/quota", adminHandler.UpdateUserQuota)

				// Global views
				r.Get("/services", adminHandler.ListAllServices)
				r.Get("/deployments", adminHandler.ListAllDeployments)

				// System metrics
				r.Get("/metrics/system", metricsHandler.GetSystemMetrics)
			})
		})
	})

	return r
}

// Klouds API Server - the main entrypoint for the Klouds PaaS platform.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/vedant/klouds/internal/api"
	"github.com/vedant/klouds/internal/auth"
	"github.com/vedant/klouds/internal/builder"
	"github.com/vedant/klouds/internal/caddy"
	"github.com/vedant/klouds/internal/cleaner"
	"github.com/vedant/klouds/internal/config"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
	"github.com/vedant/klouds/internal/dbproxy"
	"github.com/vedant/klouds/internal/secrets"
)

func main() {
	// Configure zerolog for structured logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("Klouds - Starting API Server")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Warn().Err(err).Str("dir", cfg.DataDir).Msg("Failed to create data directory")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL
	log.Info().Str("db", maskDSN(cfg.DatabaseURL)).Msg("Connecting to database")
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()
	log.Info().Msg("Database connected successfully")

	// Run migrations
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Warn().Err(err).Msg("Migration warning (may be harmless if already up to date)")
	}

	// Initialize services
	tokenSvc := auth.NewTokenService(cfg.SecretKey, cfg.TokenDuration)

	encryptor, err := secrets.NewEncryptor(cfg.SecretKey)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize encryptor")
	}

	containerMgr, err := container.NewManager(cfg.DockerNetwork)
	if err != nil {
		log.Warn().Err(err).Msg("Docker not available (container features disabled)")
		containerMgr = nil
	} else {
		// Ensure the internal Docker network exists
		if err := containerMgr.EnsureNetwork(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to create Docker network")
		}
		log.Info().Msg("Docker connected successfully")
	}

	// Initialize Caddy proxy manager
	caddyMgr := caddy.NewManager(cfg.CaddyAdminAPI, cfg.Domain)

	// Initialize Cleaner for unreferenced containers and routes GC
	cleanerMgr := cleaner.New(pool, containerMgr, caddyMgr)
	cleanerMgr.StartBackground(ctx, 10*time.Minute)

	// Initialize Nixpacks build engine and blue-green deployer
	buildEngine := builder.NewEngine(pool, cfg.DataDir)
	deployer := builder.NewDeployer(pool, containerMgr, caddyMgr, cfg.Domain)

	// Initialize single-port external database TCP router (PostgreSQL 5432, Redis 6379)
	databaseProxy := dbproxy.NewDatabaseProxy(pool, cfg.Domain, 5432, 6379, containerMgr)
	if err := databaseProxy.Start(); err != nil {
		log.Warn().Err(err).Msg("Database proxy initialization warning")
	}
	defer databaseProxy.Stop()

	// Build router
	router := api.NewRouter(api.RouterConfig{
		Pool:       pool,
		TokenSvc:   tokenSvc,
		Containers: containerMgr,
		Encryptor:  encryptor,
		Caddy:      caddyMgr,
		Engine:     buildEngine,
		Deployer:   deployer,
		Cleaner:    cleanerMgr,
		Domain:     cfg.Domain,
		DataDir:    cfg.DataDir,
		SecretKey:  cfg.SecretKey,
		BaseURL:    cfg.BaseURL,
	})

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Info().Msg("Shutting down server...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal().Err(err).Msg("Server forced to shutdown")
		}
		cancel()
	}()

	// Start server
	log.Info().
		Str("addr", cfg.Addr()).
		Str("domain", cfg.Domain).
		Msg("Klouds API Server is running")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server error")
	}

	log.Info().Msg("Server stopped")
}

// runMigrations applies database migrations.
func runMigrations(databaseURL string) error {
	// Use golang-migrate
	// Import with blank identifier to register the drivers
	m, err := newMigrate(databaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err.Error() != "no change" {
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Info().Msg("Database migrations applied successfully")
	return nil
}

// maskDSN hides the password in a DSN for logging.
func maskDSN(dsn string) string {
	// Simple masking - find password between : and @ after ://
	start := 0
	for i := 0; i < len(dsn)-3; i++ {
		if dsn[i] == ':' && dsn[i+1] == '/' && dsn[i+2] == '/' {
			start = i + 3
			break
		}
	}

	colonIdx := -1
	atIdx := -1
	for i := start; i < len(dsn); i++ {
		if dsn[i] == ':' && colonIdx == -1 {
			colonIdx = i
		}
		if dsn[i] == '@' {
			atIdx = i
			break
		}
	}

	if colonIdx > 0 && atIdx > 0 && atIdx > colonIdx {
		return dsn[:colonIdx+1] + "****" + dsn[atIdx:]
	}
	return dsn
}

// Package config manages application configuration via environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	// Server
	Host     string
	Port     int
	BaseURL  string
	Domain   string // e.g. "yourdomain.com"

	// Database
	DatabaseURL string

	// Auth
	SecretKey     string // For PASETO token signing & secrets encryption
	TokenDuration time.Duration

	// Docker
	DockerHost     string
	DockerNetwork  string

	// Caddy
	CaddyAdminAPI string // Caddy admin API endpoint

	// Build
	BuildTimeout time.Duration
	DataDir      string // Directory for persistent data (builds, backups, etc.)
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		Host:          getEnv("KLOUDS_HOST", "0.0.0.0"),
		Port:          getEnvInt("KLOUDS_PORT", 8080),
		BaseURL:       getEnv("KLOUDS_BASE_URL", "http://localhost:8080"),
		Domain:        getEnv("KLOUDS_DOMAIN", "localhost"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://klouds:klouds@localhost:5432/klouds?sslmode=disable"),
		SecretKey:     getEnv("KLOUDS_SECRET_KEY", ""),
		TokenDuration: getEnvDuration("KLOUDS_TOKEN_DURATION", 24*time.Hour),
		DockerHost:    getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		DockerNetwork: getEnv("KLOUDS_DOCKER_NETWORK", "klouds-internal"),
		CaddyAdminAPI: getEnv("KLOUDS_CADDY_ADMIN", "http://localhost:2019"),
		BuildTimeout:  getEnvDuration("KLOUDS_BUILD_TIMEOUT", 10*time.Minute),
		DataDir:       getEnv("KLOUDS_DATA_DIR", "./data"),
	}

	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("KLOUDS_SECRET_KEY is required (must be a 32+ char random string)")
	}

	if len(cfg.SecretKey) < 32 {
		return nil, fmt.Errorf("KLOUDS_SECRET_KEY must be at least 32 characters")
	}

	return cfg, nil
}

// LoadDev returns a config suitable for local development.
func LoadDev() *Config {
	return &Config{
		Host:          "0.0.0.0",
		Port:          8080,
		BaseURL:       "http://localhost:8080",
		Domain:        "localhost",
		DatabaseURL:   "postgres://klouds:klouds@localhost:5432/klouds?sslmode=disable",
		SecretKey:     "dev-secret-key-change-in-production!!",
		TokenDuration: 24 * time.Hour,
		DockerHost:    "unix:///var/run/docker.sock",
		DockerNetwork: "klouds-internal",
		CaddyAdminAPI: "http://localhost:2019",
		BuildTimeout:  10 * time.Minute,
		DataDir:       "./data",
	}
}

// Addr returns the server listen address.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// loadDotEnv reads .env from the current directory if it exists.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
}

// Package db provides type-safe database access for the Klouds platform.
// This is structured in the style of sqlc-generated code but written manually
// to avoid requiring the sqlc CLI during development. Once sqlc is installed,
// run `sqlc generate` to regenerate from the SQL queries.
package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the interface for pgx pool or transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Queries provides all database operations.
type Queries struct {
	db DBTX
}

// New creates a new Queries instance.
func New(db DBTX) *Queries {
	return &Queries{db: db}
}

// WithTx creates a new Queries instance within a transaction.
func (q *Queries) WithTx(tx pgx.Tx) *Queries {
	return &Queries{db: tx}
}

// Connect creates a pgxpool connection to the database.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	// Memory-optimized pool settings
	config.MaxConns = 10
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// ============================================================
// Model types (matching the DB schema)
// ============================================================

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserQuota struct {
	ID                 string `json:"id"`
	UserID             string `json:"user_id"`
	MaxServices        int32  `json:"max_services"`
	MaxDatabases       int32  `json:"max_databases"`
	CpuPerContainer    int32  `json:"cpu_per_container"`
	MemoryPerContainer int64  `json:"memory_per_container"`
	TotalMemory        int64  `json:"total_memory"`
	DiskPerService     int64  `json:"disk_per_service"`
	DiskPerDatabase    int64  `json:"disk_per_database"`
	BuildTimeoutSecs   int32  `json:"build_timeout_secs"`
	MaxDeploysPerDay   int32  `json:"max_deploys_per_day"`
	MaxCustomDomains   int32  `json:"max_custom_domains"`
}

type Project struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Service struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	BuildMethod     string    `json:"build_method"`
	RepoURL         *string   `json:"repo_url"`
	Branch          *string   `json:"branch"`
	RootDirectory   string    `json:"root_directory"`
	DockerfilePath  *string   `json:"dockerfile_path"`
	BuildCommand    *string   `json:"build_command"`
	StartCommand    *string   `json:"start_command"`
	Port            int32     `json:"port"`
	HealthCheckPath *string   `json:"health_check_path"`
	AutoDeploy      bool      `json:"auto_deploy"`
	ContainerID     *string   `json:"container_id"`
	ImageTag        *string   `json:"image_tag"`
	Subdomain       string    `json:"subdomain"`
	CpuLimit        int32     `json:"cpu_limit"`
	MemoryLimit     int64     `json:"memory_limit"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Database struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	UserID            string    `json:"user_id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	Engine            string    `json:"engine"`
	Version           string    `json:"version"`
	Status            string    `json:"status"`
	ContainerID       *string   `json:"container_id"`
	InternalHost      string    `json:"internal_host"`
	InternalPort      int32     `json:"internal_port"`
	ExternalAccess    bool      `json:"external_access"`
	ExternalSubdomain *string   `json:"external_subdomain"`
	DatabaseName      string    `json:"database_name"`
	Username          string    `json:"username"`
	PasswordEncrypted string    `json:"-"`
	CpuLimit          int32     `json:"cpu_limit"`
	MemoryLimit       int64     `json:"memory_limit"`
	DiskLimit         int64     `json:"disk_limit"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Deployment struct {
	ID          string     `json:"id"`
	ServiceID   string     `json:"service_id"`
	UserID      string     `json:"user_id"`
	Status      string     `json:"status"`
	CommitSHA   *string    `json:"commit_sha"`
	CommitMsg   *string    `json:"commit_msg"`
	ImageTag    string     `json:"image_tag"`
	Trigger     string     `json:"trigger"`
	BuildLog    *string    `json:"build_log"`
	DurationSec *int32     `json:"duration_sec"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

type EnvVar struct {
	ID             string    `json:"id"`
	ServiceID      string    `json:"service_id"`
	Key            string    `json:"key"`
	ValueEncrypted string    `json:"-"`
	IsBuildTime    bool      `json:"is_build_time"`
	IsLinked       bool      `json:"is_linked"`
	CreatedAt      time.Time `json:"created_at"`
}

type RouteRule struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Type      string    `json:"type"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Status    *int32    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

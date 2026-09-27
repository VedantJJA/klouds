// Package models defines the core domain types shared across Klouds.
package models

import (
	"time"
)

// UserRole represents the role of a user in the system.
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// UserStatus represents the current state of a user account.
type UserStatus string

const (
	StatusPending   UserStatus = "pending"
	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
)

// User represents a registered user in the system.
type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"` // Never serialized
	Role         UserRole   `json:"role"`
	Status       UserStatus `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// UserQuota defines resource limits for a user.
type UserQuota struct {
	ID                 string `json:"id"`
	UserID             string `json:"user_id"`
	MaxServices        int    `json:"max_services"`
	MaxDatabases       int    `json:"max_databases"`
	CPUPerContainer    int    `json:"cpu_per_container"`    // millicores (e.g. 500 = 0.5 vCPU)
	MemoryPerContainer int64  `json:"memory_per_container"` // bytes
	TotalMemory        int64  `json:"total_memory"`         // bytes
	DiskPerService     int64  `json:"disk_per_service"`     // bytes
	DiskPerDatabase    int64  `json:"disk_per_database"`    // bytes
	BuildTimeoutSecs   int    `json:"build_timeout_secs"`
	MaxDeploysPerDay   int    `json:"max_deploys_per_day"`
	MaxCustomDomains   int    `json:"max_custom_domains"`
}

// DefaultUserQuota returns the default quota for a regular user.
func DefaultUserQuota(userID string) UserQuota {
	return UserQuota{
		UserID:             userID,
		MaxServices:        3,
		MaxDatabases:       2,
		CPUPerContainer:    500,                    // 0.5 vCPU
		MemoryPerContainer: 256 * 1024 * 1024,      // 256 MB
		TotalMemory:        1024 * 1024 * 1024,      // 1 GB
		DiskPerService:     1024 * 1024 * 1024,      // 1 GB
		DiskPerDatabase:    2 * 1024 * 1024 * 1024,  // 2 GB
		BuildTimeoutSecs:   600,                     // 10 min
		MaxDeploysPerDay:   20,
		MaxCustomDomains:   2,
	}
}

// DefaultAdminQuota returns the default quota for the admin.
func DefaultAdminQuota(userID string) UserQuota {
	return UserQuota{
		UserID:             userID,
		MaxServices:        -1,                      // -1 = unlimited
		MaxDatabases:       -1,
		CPUPerContainer:    2000,                     // 2 vCPU
		MemoryPerContainer: 2 * 1024 * 1024 * 1024,  // 2 GB
		TotalMemory:        -1,                       // unlimited
		DiskPerService:     10 * 1024 * 1024 * 1024,  // 10 GB
		DiskPerDatabase:    20 * 1024 * 1024 * 1024,  // 20 GB
		BuildTimeoutSecs:   1800,                      // 30 min
		MaxDeploysPerDay:   -1,
		MaxCustomDomains:   -1,
	}
}

// ServiceType represents the type of a hosted service.
type ServiceType string

const (
	ServiceTypeWeb      ServiceType = "web"
	ServiceTypeWorker   ServiceType = "worker"
	ServiceTypeCron     ServiceType = "cron"
	ServiceTypeStatic   ServiceType = "static"
	ServiceTypePrivate  ServiceType = "private"
)

// ServiceStatus represents the lifecycle state of a service.
type ServiceStatus string

const (
	ServiceStatusCreated   ServiceStatus = "created"
	ServiceStatusBuilding  ServiceStatus = "building"
	ServiceStatusRunning   ServiceStatus = "running"
	ServiceStatusStopped   ServiceStatus = "stopped"
	ServiceStatusFailed    ServiceStatus = "failed"
	ServiceStatusDeploying ServiceStatus = "deploying"
)

// BuildMethod indicates how the service is built.
type BuildMethod string

const (
	BuildMethodNixpacks   BuildMethod = "nixpacks"
	BuildMethodDockerfile BuildMethod = "dockerfile"
)

// Project is a logical grouping of services and databases.
type Project struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Service represents a deployed application or worker.
type Service struct {
	ID              string        `json:"id"`
	ProjectID       string        `json:"project_id"`
	UserID          string        `json:"user_id"`
	Name            string        `json:"name"`
	Slug            string        `json:"slug"`
	Type            ServiceType   `json:"type"`
	Status          ServiceStatus `json:"status"`
	BuildMethod     BuildMethod   `json:"build_method"`
	RepoURL         string        `json:"repo_url,omitempty"`
	Branch          string        `json:"branch,omitempty"`
	DockerfilePath  string        `json:"dockerfile_path,omitempty"`
	BuildCommand    string        `json:"build_command,omitempty"`
	StartCommand    string        `json:"start_command,omitempty"`
	Port            int           `json:"port"`
	HealthCheckPath string        `json:"health_check_path,omitempty"`
	AutoDeploy      bool          `json:"auto_deploy"`
	ContainerID     string        `json:"container_id,omitempty"`
	ImageTag        string        `json:"image_tag,omitempty"`
	Subdomain       string        `json:"subdomain"` // <slug>.yourdomain.com
	CPULimit        int           `json:"cpu_limit"`  // millicores
	MemoryLimit     int64         `json:"memory_limit"` // bytes
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// DatabaseEngine represents a supported database engine.
type DatabaseEngine string

const (
	DatabaseEnginePostgres DatabaseEngine = "postgresql"
	DatabaseEngineMySQL    DatabaseEngine = "mysql"
	DatabaseEngineRedis    DatabaseEngine = "redis"
	DatabaseEngineMongo    DatabaseEngine = "mongodb"
)

// DatabaseStatus represents the lifecycle state of a managed database.
type DatabaseStatus string

const (
	DatabaseStatusCreating DatabaseStatus = "creating"
	DatabaseStatusRunning  DatabaseStatus = "running"
	DatabaseStatusStopped  DatabaseStatus = "stopped"
	DatabaseStatusFailed   DatabaseStatus = "failed"
)

// Database represents a managed database instance.
type Database struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"project_id"`
	UserID           string         `json:"user_id"`
	Name             string         `json:"name"`
	Slug             string         `json:"slug"`
	Engine           DatabaseEngine `json:"engine"`
	Version          string         `json:"version"`
	Status           DatabaseStatus `json:"status"`
	ContainerID      string         `json:"container_id,omitempty"`
	InternalHost     string         `json:"internal_host"` // Docker network hostname
	InternalPort     int            `json:"internal_port"`
	ExternalAccess   bool           `json:"external_access"`
	ExternalSubdomain string        `json:"external_subdomain,omitempty"`
	DatabaseName     string         `json:"database_name"`
	Username         string         `json:"username"`
	PasswordEncrypted string        `json:"-"` // AES-256-GCM encrypted
	CPULimit         int            `json:"cpu_limit"`    // millicores
	MemoryLimit      int64          `json:"memory_limit"` // bytes
	DiskLimit        int64          `json:"disk_limit"`   // bytes
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// DeployStatus represents the outcome of a deployment.
type DeployStatus string

const (
	DeployStatusPending  DeployStatus = "pending"
	DeployStatusBuilding DeployStatus = "building"
	DeployStatusLive     DeployStatus = "live"
	DeployStatusFailed   DeployStatus = "failed"
	DeployStatusRolledBack DeployStatus = "rolled_back"
)

// Deployment represents a single deployment of a service.
type Deployment struct {
	ID          string       `json:"id"`
	ServiceID   string       `json:"service_id"`
	UserID      string       `json:"user_id"`
	Status      DeployStatus `json:"status"`
	CommitSHA   string       `json:"commit_sha,omitempty"`
	CommitMsg   string       `json:"commit_msg,omitempty"`
	ImageTag    string       `json:"image_tag"`
	Trigger     string       `json:"trigger"` // "webhook", "manual", "rollback", "blueprint"
	BuildLog    string       `json:"build_log,omitempty"`
	DurationSec int          `json:"duration_sec"`
	CreatedAt   time.Time    `json:"created_at"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
}

// EnvVar represents an environment variable for a service.
type EnvVar struct {
	ID             string `json:"id"`
	ServiceID      string `json:"service_id"`
	Key            string `json:"key"`
	ValueEncrypted string `json:"-"`    // Stored encrypted
	Value          string `json:"value"` // Decrypted, only in memory
	IsBuildTime    bool   `json:"is_build_time"`
	IsLinked       bool   `json:"is_linked"` // Auto-generated from DB link
	CreatedAt      time.Time `json:"created_at"`
}

// RouteRule represents a redirect or rewrite rule for a service.
type RouteRule struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	Type      string `json:"type"` // "redirect" or "rewrite"
	Source    string `json:"source"`
	Target    string `json:"target"`
	Status    int    `json:"status,omitempty"` // 301, 302 for redirects
	CreatedAt time.Time `json:"created_at"`
}

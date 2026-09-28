// Package container manages Docker container lifecycle for user services and databases.
package container

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	mobycontainer "github.com/moby/moby/api/types/container"
	mobynetwork "github.com/moby/moby/api/types/network"
	mobyclient "github.com/moby/moby/client"
	"github.com/rs/zerolog/log"
)

// Manager handles Docker container operations.
type Manager struct {
	client      *mobyclient.Client
	networkName string
}

// NewManager creates a new container manager.
func NewManager(networkName string) (*Manager, error) {
	cli, err := mobyclient.NewClientWithOpts(mobyclient.FromEnv, mobyclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &Manager{
		client:      cli,
		networkName: networkName,
	}, nil
}

// EnsureNetwork creates the internal Docker bridge network if it doesn't exist.
func (m *Manager) EnsureNetwork(ctx context.Context) error {
	result, err := m.client.NetworkList(ctx, mobyclient.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}

	for _, n := range result.Items {
		if n.Name == m.networkName {
			log.Info().Str("network", m.networkName).Msg("Docker network already exists")
			return nil
		}
	}

	_, err = m.client.NetworkCreate(ctx, m.networkName, mobyclient.NetworkCreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			"managed-by": "klouds",
		},
	})
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	log.Info().Str("network", m.networkName).Msg("Created Docker network")
	return nil
}

// ServiceConfig holds configuration for creating a service container.
type ServiceConfig struct {
	Name         string            // Container name (e.g., "klouds-svc-myapp")
	Image        string            // Docker image tag
	Port         int               // Internal port the app listens on
	EnvVars      map[string]string // Environment variables
	CPULimit     int64             // CPU limit in millicores
	MemoryLimit  int64             // Memory limit in bytes
	Labels       map[string]string // Container labels
	Cmd          []string          // Override command
	NetworkAlias string            // Hostname on the internal network
}

// CreateServiceContainer creates and starts a container for a user service.
func (m *Manager) CreateServiceContainer(ctx context.Context, cfg ServiceConfig) (string, error) {
	portNum := cfg.Port
	if portNum <= 0 {
		portNum = 3000
	}

	if cfg.EnvVars == nil {
		cfg.EnvVars = make(map[string]string)
	}
	if _, ok := cfg.EnvVars["PORT"]; !ok {
		cfg.EnvVars["PORT"] = fmt.Sprintf("%d", portNum)
	}

	// Build env vars list
	env := make([]string, 0, len(cfg.EnvVars))
	for k, v := range cfg.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add standard labels
	if cfg.Labels == nil {
		cfg.Labels = make(map[string]string)
	}
	cfg.Labels["managed-by"] = "klouds"

	exposedPorts := make(mobynetwork.PortSet)
	port := mobynetwork.MustParsePort(fmt.Sprintf("%d/tcp", portNum))
	exposedPorts[port] = struct{}{}

	containerConfig := &mobycontainer.Config{
		Image:        cfg.Image,
		Env:          env,
		Labels:       cfg.Labels,
		ExposedPorts: exposedPorts,
	}

	if len(cfg.Cmd) > 0 {
		containerConfig.Cmd = cfg.Cmd
	}

	// Resource limits
	hostConfig := &mobycontainer.HostConfig{
		Resources: mobycontainer.Resources{
			NanoCPUs: cfg.CPULimit * 1_000_000, // millicores to nanocpus
			Memory:   cfg.MemoryLimit,
		},
		RestartPolicy: mobycontainer.RestartPolicy{
			Name: mobycontainer.RestartPolicyUnlessStopped,
		},
		SecurityOpt: []string{"no-new-privileges"},
	}

	// Network config - attach to klouds-internal
	networkConfig := &mobynetwork.NetworkingConfig{
		EndpointsConfig: map[string]*mobynetwork.EndpointSettings{
			m.networkName: {
				Aliases: []string{cfg.NetworkAlias},
			},
		},
	}

	resp, err := m.client.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config:           containerConfig,
		HostConfig:       hostConfig,
		NetworkingConfig: networkConfig,
		Name:             cfg.Name,
	})
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if _, err := m.client.ContainerStart(ctx, resp.ID, mobyclient.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("start container: %w", err)
	}

	log.Info().
		Str("container_id", resp.ID[:12]).
		Str("name", cfg.Name).
		Msg("Container started")

	return resp.ID, nil
}

// DatabaseConfig holds configuration for creating a managed database container.
type DatabaseConfig struct {
	Name         string
	Engine       string // postgresql, mysql, redis, mongodb
	Version      string
	DatabaseName string
	Username     string
	Password     string
	Port         int
	CPULimit     int64
	MemoryLimit  int64
	DiskLimit    int64
	DataVolume   string // Host path for data persistence
	NetworkAlias string
}

// CreateDatabaseContainer creates and starts a database container.
func (m *Manager) CreateDatabaseContainer(ctx context.Context, cfg DatabaseConfig) (string, error) {
	image, env, cmd, dbPort := m.databaseImageConfig(cfg)

	// Pull image first
	if err := m.pullImage(ctx, image); err != nil {
		return "", fmt.Errorf("pull image %s: %w", image, err)
	}

	labels := map[string]string{
		"managed-by":    "klouds",
		"klouds.type":   "database",
		"klouds.engine": cfg.Engine,
	}

	exposedPorts := make(mobynetwork.PortSet)
	port := mobynetwork.MustParsePort(fmt.Sprintf("%d/tcp", dbPort))
	exposedPorts[port] = struct{}{}

	containerConfig := &mobycontainer.Config{
		Image:        image,
		Env:          env,
		Cmd:          cmd,
		Labels:       labels,
		ExposedPorts: exposedPorts,
	}

	hostConfig := &mobycontainer.HostConfig{
		Resources: mobycontainer.Resources{
			NanoCPUs: cfg.CPULimit * 1_000_000,
			Memory:   cfg.MemoryLimit,
		},
		RestartPolicy: mobycontainer.RestartPolicy{
			Name: mobycontainer.RestartPolicyUnlessStopped,
		},
		SecurityOpt: []string{"no-new-privileges"},
	}

	// Mount a volume for data persistence
	if cfg.DataVolume != "" {
		hostConfig.Binds = []string{
			fmt.Sprintf("%s:/data:rw", cfg.DataVolume),
		}
	}

	networkConfig := &mobynetwork.NetworkingConfig{
		EndpointsConfig: map[string]*mobynetwork.EndpointSettings{
			m.networkName: {
				Aliases: []string{cfg.NetworkAlias},
			},
		},
	}

	resp, err := m.client.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config:           containerConfig,
		HostConfig:       hostConfig,
		NetworkingConfig: networkConfig,
		Name:             cfg.Name,
	})
	if err != nil {
		return "", fmt.Errorf("create database container: %w", err)
	}

	if _, err := m.client.ContainerStart(ctx, resp.ID, mobyclient.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("start database container: %w", err)
	}

	log.Info().
		Str("container_id", resp.ID[:12]).
		Str("engine", cfg.Engine).
		Str("name", cfg.Name).
		Msg("Database container started")

	return resp.ID, nil
}

// StopContainer stops a running container gracefully.
func (m *Manager) StopContainer(ctx context.Context, containerID string) error {
	timeout := 30 // seconds
	_, err := m.client.ContainerStop(ctx, containerID, mobyclient.ContainerStopOptions{
		Timeout: &timeout,
	})
	return err
}

// RemoveContainer removes a stopped container.
func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	_, err := m.client.ContainerRemove(ctx, containerID, mobyclient.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: false, // Keep data volumes
	})
	return err
}

// ContainerLogs returns a reader for a container's logs.
func (m *Manager) ContainerLogs(ctx context.Context, containerID string, tail string) (io.ReadCloser, error) {
	return m.client.ContainerLogs(ctx, containerID, mobyclient.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Follow:     false,
		Timestamps: true,
	})
}

// GetContainerLogsString returns recent container logs as a string.
func (m *Manager) GetContainerLogsString(ctx context.Context, containerID string, tail string) (string, error) {
	rc, err := m.ContainerLogs(ctx, containerID, tail)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, rc)
	return buf.String(), nil
}

// ContainerLogsFollow returns a streaming reader for live logs.
func (m *Manager) ContainerLogsFollow(ctx context.Context, containerID string) (io.ReadCloser, error) {
	return m.client.ContainerLogs(ctx, containerID, mobyclient.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
		Timestamps: true,
	})
}

// HealthCheck checks if a container is running and has been up for a bit.
func (m *Manager) HealthCheck(ctx context.Context, containerID string, port int, path string) bool {
	result, err := m.client.ContainerInspect(ctx, containerID, mobyclient.ContainerInspectOptions{})
	if err != nil {
		return false
	}

	if !result.Container.State.Running {
		return false
	}

	startedAt, err := time.Parse(time.RFC3339Nano, result.Container.State.StartedAt)
	if err != nil {
		return false
	}

	if time.Since(startedAt) < 2*time.Second {
		return false
	}

	return true
}

// IsRunning checks if a container is currently running.
func (m *Manager) IsRunning(ctx context.Context, containerID string) bool {
	result, err := m.client.ContainerInspect(ctx, containerID, mobyclient.ContainerInspectOptions{})
	if err != nil {
		return false
	}
	return result.Container.State.Running
}

// pullImage pulls a Docker image if not already present.
func (m *Manager) pullImage(ctx context.Context, image string) error {
	reader, err := m.client.ImagePull(ctx, image, mobyclient.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// databaseImageConfig returns the Docker image, environment variables, command, and default port
// for a given database engine.
func (m *Manager) databaseImageConfig(cfg DatabaseConfig) (image string, env []string, cmd []string, port int) {
	switch strings.ToLower(cfg.Engine) {
	case "postgresql", "postgres", "pgsql":
		v := cfg.Version
		if v == "" {
			v = "16"
		}
		image = fmt.Sprintf("postgres:%s", v)
		env = []string{
			fmt.Sprintf("POSTGRES_DB=%s", cfg.DatabaseName),
			fmt.Sprintf("POSTGRES_USER=%s", cfg.Username),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", cfg.Password),
			"PGDATA=/data/pgdata",
		}
		port = 5432
	case "mysql":
		v := cfg.Version
		if v == "" {
			v = "8.0"
		}
		image = fmt.Sprintf("mysql:%s", v)
		env = []string{
			fmt.Sprintf("MYSQL_DATABASE=%s", cfg.DatabaseName),
			fmt.Sprintf("MYSQL_USER=%s", cfg.Username),
			fmt.Sprintf("MYSQL_PASSWORD=%s", cfg.Password),
			fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", cfg.Password),
		}
		port = 3306
	case "redis", "cache", "redis-cache", "valkey":
		v := cfg.Version
		if v == "" || v == "16" || v == "8.0" || v == "latest" {
			v = "7-alpine"
		} else if !strings.Contains(v, "alpine") && !strings.Contains(v, ".") {
			v = fmt.Sprintf("%s-alpine", v)
		}
		image = fmt.Sprintf("redis:%s", v)
		env = []string{
			fmt.Sprintf("REDIS_PASSWORD=%s", cfg.Password),
		}
		if cfg.Password != "" {
			cmd = []string{"redis-server", "--requirepass", cfg.Password, "--appendonly", "yes"}
		} else {
			cmd = []string{"redis-server", "--appendonly", "yes"}
		}
		port = 6379
	case "mongodb", "mongo":
		v := cfg.Version
		if v == "" || v == "16" || v == "8.0" {
			v = "7"
		}
		image = fmt.Sprintf("mongo:%s", v)
		env = []string{
			fmt.Sprintf("MONGO_INITDB_DATABASE=%s", cfg.DatabaseName),
			fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", cfg.Username),
			fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", cfg.Password),
		}
		port = 27017
	default:
		image = fmt.Sprintf("postgres:%s", cfg.Version)
		port = 5432
	}
	return
}

// GetContainerIP returns the container's IP address on the internal network.
func (m *Manager) GetContainerIP(ctx context.Context, containerID string) (string, error) {
	result, err := m.client.ContainerInspect(ctx, containerID, mobyclient.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	if result.Container.NetworkSettings != nil && result.Container.NetworkSettings.Networks != nil {
		if nw, ok := result.Container.NetworkSettings.Networks[m.networkName]; ok && nw != nil && nw.IPAddress.IsValid() {
			return nw.IPAddress.String(), nil
		}
		for _, nw := range result.Container.NetworkSettings.Networks {
			if nw != nil && nw.IPAddress.IsValid() {
				return nw.IPAddress.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no IP address found for container %s", containerID)
}

// Close releases the Docker client.
func (m *Manager) Close() error {
	return m.client.Close()
}

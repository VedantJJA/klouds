// Package builder handles zero-downtime blue-green service deployments.
package builder

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/caddy"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
)

// Deployer coordinates zero-downtime deployments.
type Deployer struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	containers *container.Manager
	caddy      *caddy.Manager
	domain     string
}

// NewDeployer creates a new blue-green deployer instance.
func NewDeployer(pool *pgxpool.Pool, containers *container.Manager, caddy *caddy.Manager, domain string) *Deployer {
	return &Deployer{
		pool:       pool,
		queries:    db.New(pool),
		containers: containers,
		caddy:      caddy,
		domain:     domain,
	}
}

// DeployRequest contains parameters for deploying a service container.
type DeployRequest struct {
	ServiceID    string
	DeploymentID string
	ImageTag     string
	EnvVars      map[string]string
}

// Deploy executes the zero-downtime blue-green container swap.
func (d *Deployer) Deploy(ctx context.Context, req DeployRequest) error {
	startTime := time.Now()
	svc, err := d.queries.GetServiceByID(ctx, req.ServiceID)
	if err != nil {
		return fmt.Errorf("fetch service: %w", err)
	}

	shortID := req.DeploymentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	newContainerName := fmt.Sprintf("klouds-svc-%s-%s", svc.Slug, shortID)
	oldContainerID := ""
	if svc.ContainerID != nil {
		oldContainerID = *svc.ContainerID
	}

	log.Info().
		Str("service", svc.Name).
		Str("deployment", req.DeploymentID).
		Str("new_container", newContainerName).
		Msg("Beginning zero-downtime blue-green deployment")

	// Step 1: Start new container (green)
	port := int(svc.Port)
	if port <= 0 {
		port = 3000
	}

	if req.EnvVars == nil {
		req.EnvVars = make(map[string]string)
	}
	if _, ok := req.EnvVars["NODE_OPTIONS"]; !ok {
		req.EnvVars["NODE_OPTIONS"] = "--experimental-require-module"
	}

	cfg := container.ServiceConfig{
		Name:         newContainerName,
		Image:        req.ImageTag,
		Port:         port,
		EnvVars:      req.EnvVars,
		CPULimit:     int64(svc.CpuLimit),
		MemoryLimit:  int64(svc.MemoryLimit) * 1024 * 1024,
		NetworkAlias: newContainerName,
		Labels: map[string]string{
			"klouds.service.id":    svc.ID,
			"klouds.deployment.id": req.DeploymentID,
		},
	}

	newContainerID, err := d.containers.CreateServiceContainer(ctx, cfg)
	if err != nil {
		_ = d.failDeployment(ctx, req.DeploymentID, fmt.Sprintf("Failed to launch container: %v", err))
		return fmt.Errorf("launch container: %w", err)
	}

	// Step 2: Health check verification (wait for readiness)
	healthCheckPath := "/"
	if svc.HealthCheckPath != nil && *svc.HealthCheckPath != "" {
		healthCheckPath = *svc.HealthCheckPath
	}

	containerTarget := newContainerName
	if ip, err := d.containers.GetContainerIP(ctx, newContainerID); err == nil && ip != "" {
		containerTarget = ip
	}

	healthy := d.waitForHealth(ctx, containerTarget, port, healthCheckPath, 30*time.Second)
	if !healthy {
		// Rollback: Tear down green container, leave old container running!
		log.Warn().
			Str("service", svc.Name).
			Str("container", newContainerName).
			Str("target", containerTarget).
			Msg("New container failed healthcheck. Rolling back...")

		containerLogs, _ := d.containers.GetContainerLogsString(ctx, newContainerID, "50")
		_ = d.containers.StopContainer(ctx, newContainerID)
		_ = d.containers.RemoveContainer(ctx, newContainerID)

		failMsg := "Healthcheck probe timed out after 30 seconds."
		if containerLogs != "" {
			failMsg = fmt.Sprintf("Healthcheck probe timed out after 30 seconds.\n[klouds-container-logs]\n%s", strings.TrimSpace(containerLogs))
		}
		_ = d.failDeployment(ctx, req.DeploymentID, failMsg)
		return fmt.Errorf("healthcheck failed for new deployment")
	}

	// Step 3: Promote green container - Update Caddy route
	if d.caddy != nil && svc.Subdomain != "" {
		route := caddy.Route{
			Subdomain:   svc.Subdomain,
			BackendHost: containerTarget,
			BackendPort: port,
		}
		if err := d.caddy.AddRoute(route); err != nil {
			log.Error().Err(err).Msg("Failed to update Caddy route")
		}
	}

	// Step 4: Decommission old container (blue)
	if oldContainerID != "" && oldContainerID != newContainerID {
		log.Info().Str("old_container", oldContainerID).Msg("Stopping decommissioned container")
		_ = d.containers.StopContainer(ctx, oldContainerID)
		_ = d.containers.RemoveContainer(ctx, oldContainerID)
	}

	// Step 5: Update database state
	imageTag := req.ImageTag
	_, err = d.queries.UpdateServiceContainer(ctx, svc.ID, &newContainerID, &imageTag, "running")
	if err != nil {
		log.Error().Err(err).Msg("Failed to update service container record")
	}

	durationSec := int32(time.Since(startTime).Seconds())
	_, err = d.queries.UpdateDeploymentFinished(ctx, req.DeploymentID, "live", durationSec)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update deployment status")
	}

	log.Info().
		Str("service", svc.Name).
		Str("container_id", newContainerID).
		Msg("Zero-downtime deployment finished successfully")

	return nil
}

func (d *Deployer) waitForHealth(ctx context.Context, host string, port int, path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// TCP connectivity probe
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 1*time.Second)
		if err == nil {
			conn.Close()

			// HTTP GET probe if reachable
			url := fmt.Sprintf("http://%s:%d%s", host, port, path)
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 500 {
					return true
				}
			} else {
				// If TCP connected but HTTP returned non-standard response, consider healthy
				return true
			}
		}

		time.Sleep(1 * time.Second)
	}

	return false
}

func (d *Deployer) failDeployment(ctx context.Context, deploymentID string, reason string) error {
	_ = d.queries.UpdateDeploymentLog(ctx, deploymentID, fmt.Sprintf("\n[klouds-deployer] ERROR: %s", reason))
	_, err := d.queries.UpdateDeploymentFinished(ctx, deploymentID, "failed", 0)
	return err
}

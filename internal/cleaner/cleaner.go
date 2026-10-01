// Package cleaner provides garbage collection for unreferenced Docker containers and Caddy reverse proxy routes.
package cleaner

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/caddy"
	"github.com/vedant/klouds/internal/container"
)

// PruneResult summarizes the containers and routes pruned during a cleanup run.
type PruneResult struct {
	PrunedContainers []string `json:"pruned_containers"`
	PrunedRoutes     []string `json:"pruned_routes"`
}

// Cleaner coordinates orphan container and routing cleanup across PostgreSQL, Docker, and Caddy.
type Cleaner struct {
	pool       *pgxpool.Pool
	containers *container.Manager
	caddy      *caddy.Manager
}

// New creates a new Cleaner instance.
func New(pool *pgxpool.Pool, containers *container.Manager, caddyMgr *caddy.Manager) *Cleaner {
	return &Cleaner{
		pool:       pool,
		containers: containers,
		caddy:      caddyMgr,
	}
}

// PruneAll identifies and purges all Docker containers and Caddy routes that are no longer
// referenced by active services or databases in PostgreSQL.
// minAge specifies a minimum container age for in-flight deployment protection (e.g. 0 to force immediately).
func (c *Cleaner) PruneAll(ctx context.Context, minAge time.Duration) (PruneResult, error) {
	var result PruneResult

	activeContainerIDs := make(map[string]bool)
	activeServiceIDs := make(map[string]bool)
	activeDatabaseIDs := make(map[string]bool)
	activeDBHosts := make(map[string]bool)
	var activeSubdomains []string

	// 1. Fetch active references from PostgreSQL
	if c.pool != nil {
		// Active container IDs from services and databases
		rows, err := c.pool.Query(ctx, `
			SELECT container_id FROM services WHERE container_id IS NOT NULL AND container_id != ''
			UNION
			SELECT container_id FROM databases WHERE container_id IS NOT NULL AND container_id != ''
		`)
		if err == nil {
			for rows.Next() {
				var cid string
				if err := rows.Scan(&cid); err == nil {
					cid = strings.TrimSpace(cid)
					if cid != "" {
						activeContainerIDs[cid] = true
						if len(cid) >= 12 {
							activeContainerIDs[cid[:12]] = true
						}
					}
				}
			}
			rows.Close()
		} else {
			log.Warn().Err(err).Msg("Failed to query active container IDs for orphan prune")
		}

		// Active service IDs and subdomains
		sRows, err := c.pool.Query(ctx, `SELECT id, subdomain FROM services`)
		if err == nil {
			for sRows.Next() {
				var sid string
				var sub *string
				if err := sRows.Scan(&sid, &sub); err == nil {
					if sid != "" {
						activeServiceIDs[sid] = true
					}
					if sub != nil && *sub != "" {
						activeSubdomains = append(activeSubdomains, *sub)
					}
				}
			}
			sRows.Close()
		} else {
			log.Warn().Err(err).Msg("Failed to query active service IDs for orphan prune")
		}

		// Active database IDs & internal hosts
		dbRows, err := c.pool.Query(ctx, `SELECT id, internal_host FROM databases`)
		if err == nil {
			for dbRows.Next() {
				var dbID, host string
				if err := dbRows.Scan(&dbID, &host); err == nil {
					if dbID != "" {
						activeDatabaseIDs[dbID] = true
					}
					if host != "" {
						activeDBHosts[host] = true
					}
				}
			}
			dbRows.Close()
		} else {
			log.Warn().Err(err).Msg("Failed to query active database IDs for orphan prune")
		}
	}

	// 2. Prune unreferenced containers from Docker
	if c.containers != nil {
		criteria := container.OrphanCriteria{
			ActiveContainerIDs: activeContainerIDs,
			ActiveServiceIDs:   activeServiceIDs,
			ActiveDatabaseIDs:  activeDatabaseIDs,
			ActiveDBHosts:      activeDBHosts,
		}
		pruned, err := c.containers.PruneOrphanContainers(ctx, criteria, minAge)
		if err != nil {
			log.Error().Err(err).Msg("Failed to prune orphan containers")
		} else {
			result.PrunedContainers = pruned
		}
	}

	// 3. Prune unreferenced routes from Caddy
	if c.caddy != nil {
		prunedRoutes, err := c.caddy.PruneOrphanRoutes(activeSubdomains)
		if err != nil {
			log.Error().Err(err).Msg("Failed to prune orphan Caddy routes")
		} else {
			result.PrunedRoutes = prunedRoutes
		}
	}

	return result, nil
}

// StartBackground starts a periodic background worker that cleans up unreferenced containers and routes.
func (c *Cleaner) StartBackground(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}

	go func() {
		// Run initial cleanup after 5s of startup to catch any containers left from before reboot
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			log.Info().Msg("Running initial orphan container and route prune pass...")
			res, err := c.PruneAll(context.Background(), 2*time.Minute)
			if err != nil {
				log.Error().Err(err).Msg("Initial orphan prune failed")
			} else if len(res.PrunedContainers) > 0 || len(res.PrunedRoutes) > 0 {
				log.Info().
					Int("containers", len(res.PrunedContainers)).
					Int("routes", len(res.PrunedRoutes)).
					Msg("Initial orphan prune completed")
			}
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res, err := c.PruneAll(context.Background(), 5*time.Minute)
				if err != nil {
					log.Error().Err(err).Msg("Background orphan prune failed")
				} else if len(res.PrunedContainers) > 0 || len(res.PrunedRoutes) > 0 {
					log.Info().
						Int("containers", len(res.PrunedContainers)).
						Int("routes", len(res.PrunedRoutes)).
						Msg("Background orphan prune completed")
				}
			}
		}
	}()
}

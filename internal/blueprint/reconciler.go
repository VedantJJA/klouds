package blueprint

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/builder"
	"github.com/vedant/klouds/internal/container"
	"github.com/vedant/klouds/internal/db"
	"github.com/vedant/klouds/internal/secrets"
)

// Reconciler applies a blueprint to a project, creating and deploying services.
type Reconciler struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	encryptor  *secrets.Encryptor
	containers *container.Manager
	engine     *builder.Engine
	deployer   *builder.Deployer
}

// NewReconciler creates a new blueprint reconciler.
func NewReconciler(
	pool *pgxpool.Pool,
	encryptor *secrets.Encryptor,
	containers *container.Manager,
	engine *builder.Engine,
	deployer *builder.Deployer,
) *Reconciler {
	return &Reconciler{
		pool:       pool,
		queries:    db.New(pool),
		encryptor:  encryptor,
		containers: containers,
		engine:     engine,
		deployer:   deployer,
	}
}

// ReconcileResult contains the outcome of applying a blueprint.
type ReconcileResult struct {
	ServicesCreated []string `json:"services_created"`
	ServicesUpdated []string `json:"services_updated"`
	Databases       []string `json:"databases"`
	Deployments     []string `json:"deployments"`
}

// Reconcile applies the blueprint specification to a target project.
func (r *Reconciler) Reconcile(
	ctx context.Context,
	projectID, userID, repoURL, branch string,
	bp *Blueprint,
) (*ReconcileResult, error) {
	if err := bp.Validate(); err != nil {
		return nil, fmt.Errorf("invalid blueprint: %w", err)
	}

	result := &ReconcileResult{
		ServicesCreated: make([]string, 0),
		ServicesUpdated: make([]string, 0),
		Databases:       make([]string, 0),
		Deployments:     make([]string, 0),
	}

	// 1. Reconcile Databases
	dbLookup := make(map[string]db.Database)
	existingDBs, _ := r.queries.ListDatabasesByProject(ctx, projectID)
	for _, edb := range existingDBs {
		dbLookup[edb.Name] = edb
		dbLookup[edb.Slug] = edb
	}

	for _, dbDef := range bp.Databases {
		if _, exists := dbLookup[dbDef.Name]; exists {
			result.Databases = append(result.Databases, dbDef.Name)
			continue
		}

		slug := slugify(dbDef.Name)
		portMap := map[string]int32{"postgresql": 5432, "mysql": 3306, "redis": 6379, "mongodb": 27017}
		internalPort := portMap[dbDef.Engine]
		if internalPort == 0 {
			internalPort = 5432
		}
		internalHost := fmt.Sprintf("klouds-db-%s", slug)
		databaseName := fmt.Sprintf("klouds_%s", slug)
		username := fmt.Sprintf("klouds_%s", slug)
		password := generateRandomPassword(24)

		encPassword, err := r.encryptor.Encrypt(password)
		if err != nil {
			return nil, fmt.Errorf("encrypt db password: %w", err)
		}

		createdDB, err := r.queries.CreateDatabase(ctx, db.CreateDatabaseParams{
			ProjectID:         projectID,
			UserID:            userID,
			Name:              dbDef.Name,
			Slug:              slug,
			Engine:            dbDef.Engine,
			Version:           dbDef.Version,
			InternalHost:      internalHost,
			InternalPort:      internalPort,
			DatabaseName:      databaseName,
			Username:          username,
			PasswordEncrypted: encPassword,
			CpuLimit:          500,
			MemoryLimit:       256 * 1024 * 1024,
			DiskLimit:         2 * 1024 * 1024 * 1024,
		})
		if err != nil {
			return nil, fmt.Errorf("create database %s: %w", dbDef.Name, err)
		}

		dbLookup[dbDef.Name] = createdDB
		result.Databases = append(result.Databases, dbDef.Name)
	}

	// 2. Reconcile Services
	existingSvcs, _ := r.queries.ListServicesByProject(ctx, projectID)
	svcLookup := make(map[string]db.Service)
	for _, es := range existingSvcs {
		svcLookup[es.Slug] = es
		svcLookup[es.Name] = es
	}

	for _, svcDef := range bp.Services {
		slug := slugify(svcDef.Name)
		autoDeploy := true
		if svcDef.AutoDeploy != nil {
			autoDeploy = *svcDef.AutoDeploy
		}

		rootDir := svcDef.RootDir
		if rootDir == "" {
			rootDir = "."
		}

		dockerfilePath := svcDef.DockerfilePath
		if dockerfilePath == "" {
			dockerfilePath = "Dockerfile"
		}

		healthCheckPath := svcDef.HealthCheckPath
		if healthCheckPath == "" {
			healthCheckPath = "/"
		}

		var targetSvc db.Service
		var isNew bool

		if existing, ok := svcLookup[slug]; ok {
			targetSvc = existing
			result.ServicesUpdated = append(result.ServicesUpdated, svcDef.Name)
		} else {
			// Create new service
			subdomain := slug
			newSvc, err := r.queries.CreateService(ctx, db.CreateServiceParams{
				ProjectID:       projectID,
				UserID:          userID,
				Name:            svcDef.Name,
				Slug:            slug,
				Type:            svcDef.Type,
				BuildMethod:     svcDef.BuildMethod,
				RepoURL:         &repoURL,
				Branch:          &branch,
				RootDirectory:   rootDir,
				DockerfilePath:  &dockerfilePath,
				BuildCommand:    &svcDef.BuildCommand,
				StartCommand:    &svcDef.StartCommand,
				Port:            svcDef.Port,
				HealthCheckPath: &healthCheckPath,
				AutoDeploy:      autoDeploy,
				Subdomain:       subdomain,
				CpuLimit:        500,
				MemoryLimit:     256 * 1024 * 1024,
			})
			if err != nil {
				return nil, fmt.Errorf("create service %s: %w", svcDef.Name, err)
			}
			targetSvc = newSvc
			isNew = true
			result.ServicesCreated = append(result.ServicesCreated, svcDef.Name)
		}

		// Save/Sync Environment Variables
		resolvedEnv := make(map[string]string)
		for _, ev := range svcDef.EnvVars {
			val := ev.Value
			if ev.FromDatabase != nil {
				if d, ok := dbLookup[ev.FromDatabase.Name]; ok {
					switch ev.FromDatabase.Property {
					case "host":
						val = d.InternalHost
					case "port":
						val = fmt.Sprintf("%d", d.InternalPort)
					case "user":
						val = d.Username
					case "database":
						val = d.DatabaseName
					case "connectionString", "url":
						rawPass, _ := r.encryptor.Decrypt(d.PasswordEncrypted)
						switch d.Engine {
						case "postgresql":
							val = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", d.Username, rawPass, d.InternalHost, d.InternalPort, d.DatabaseName)
						case "mysql":
							val = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", d.Username, rawPass, d.InternalHost, d.InternalPort, d.DatabaseName)
						case "redis":
							val = fmt.Sprintf("redis://default:%s@%s:%d", rawPass, d.InternalHost, d.InternalPort)
						case "mongodb":
							val = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", d.Username, rawPass, d.InternalHost, d.InternalPort, d.DatabaseName)
						}
					}
				}
			}

			if ev.Key != "" {
				resolvedEnv[ev.Key] = val
				encVal, err := r.encryptor.Encrypt(val)
				if err == nil {
					_, _ = r.queries.CreateEnvVar(ctx, db.CreateEnvVarParams{
						ServiceID:      targetSvc.ID,
						Key:            ev.Key,
						ValueEncrypted: encVal,
						IsBuildTime:    false,
						IsLinked:       ev.FromDatabase != nil,
					})
				}
			}
		}

		// Save Route Rules (redirects/rewrites)
		for _, rule := range svcDef.Routes {
			stat := rule.Status
			if stat == 0 {
				stat = 301
			}
			_, _ = r.queries.CreateRouteRule(ctx, db.CreateRouteRuleParams{
				ServiceID: targetSvc.ID,
				Type:      rule.Type,
				Source:    rule.Source,
				Target:    rule.Target,
				Status:    &stat,
			})
		}

		// Trigger deployment
		if repoURL != "" && (isNew || autoDeploy) {
			commitSHA := "HEAD"
			commitMsg := "Blueprint deployment"
			imageTag := fmt.Sprintf("klouds/%s:bp-%s", targetSvc.Slug, targetSvc.ID[:min(8, len(targetSvc.ID))])

			dep, err := r.queries.CreateDeployment(ctx, db.CreateDeploymentParams{
				ServiceID: targetSvc.ID,
				UserID:    userID,
				Status:    "building",
				CommitSHA: &commitSHA,
				CommitMsg: &commitMsg,
				ImageTag:  imageTag,
				Trigger:   "blueprint",
			})
			if err == nil {
				result.Deployments = append(result.Deployments, dep.ID)

				// Async Build and Deploy
				go func(s db.Service, dID, tag string, envs map[string]string) {
					bgCtx := context.Background()
					bRes, bErr := r.engine.Build(bgCtx, builder.BuildOptions{
						DeploymentID:   dID,
						ServiceID:      s.ID,
						ServiceSlug:    s.Slug,
						RepoURL:        repoURL,
						Branch:         branch,
						CommitSHA:      "",
						BuildMethod:    s.BuildMethod,
						RootDir:        s.RootDirectory,
						DockerfilePath: *s.DockerfilePath,
						EnvVars:        envs,
					})
					if bErr != nil {
						log.Error().Err(bErr).Str("service", s.Name).Msg("Blueprint build failed")
						_, _ = r.queries.UpdateDeploymentFinished(bgCtx, dID, "failed", 0)
						return
					}

					depErr := r.deployer.Deploy(bgCtx, builder.DeployRequest{
						ServiceID:    s.ID,
						DeploymentID: dID,
						ImageTag:     bRes.ImageTag,
						EnvVars:      envs,
					})
					if depErr != nil {
						log.Error().Err(depErr).Str("service", s.Name).Msg("Blueprint rollout failed")
					}
				}(targetSvc, dep.ID, imageTag, resolvedEnv)
			}
		}
	}

	return result, nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else if r == '-' || r == '_' || r == ' ' {
			sb.WriteRune('-')
		}
	}
	res := strings.Trim(sb.String(), "-")
	if res == "" {
		res = "svc"
	}
	return res
}

func generateRandomPassword(length int) string {
	b := make([]byte, length/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

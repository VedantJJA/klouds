// Package blueprint provides Infrastructure-as-Code parsing, auto-detection,
// and reconciliation for multi-service repositories.
package blueprint

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Blueprint represents a declarative stack configuration (klouds.yaml or render.yaml).
type Blueprint struct {
	Version   string               `yaml:"version" json:"version"`
	Services  []ServiceDefinition  `yaml:"services" json:"services"`
	Databases []DatabaseDefinition `yaml:"databases,omitempty" json:"databases,omitempty"`
}

// ServiceDefinition declares a containerized or static application service.
type ServiceDefinition struct {
	Name            string             `yaml:"name" json:"name"`
	Type            string             `yaml:"type" json:"type"` // web, worker, cron, static, private
	Env             string             `yaml:"env" json:"env"`   // node, go, python, rust, docker, static
	BuildMethod     string             `yaml:"buildMethod,omitempty" json:"build_method,omitempty"`
	RootDir         string             `yaml:"rootDir,omitempty" json:"root_dir,omitempty"`
	BuildCommand    string             `yaml:"buildCommand,omitempty" json:"build_command,omitempty"`
	StartCommand    string             `yaml:"startCommand,omitempty" json:"start_command,omitempty"`
	DockerfilePath  string             `yaml:"dockerfilePath,omitempty" json:"dockerfile_path,omitempty"`
	Port            int32              `yaml:"port,omitempty" json:"port,omitempty"`
	HealthCheckPath string             `yaml:"healthCheckPath,omitempty" json:"health_check_path,omitempty"`
	AutoDeploy      *bool              `yaml:"autoDeploy,omitempty" json:"auto_deploy,omitempty"`
	Schedule        string             `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	EnvVars         []EnvVarDefinition `yaml:"envVars,omitempty" json:"env_vars,omitempty"`
	Routes          []RouteDefinition  `yaml:"routes,omitempty" json:"routes,omitempty"`
}

// DatabaseDefinition declares a managed stateful database service.
type DatabaseDefinition struct {
	Name         string `yaml:"name" json:"name"`
	Engine       string `yaml:"engine" json:"engine"` // postgresql, mysql, redis, mongodb
	Version      string `yaml:"version,omitempty" json:"version,omitempty"`
	DatabaseName string `yaml:"databaseName,omitempty" json:"database_name,omitempty"`
	User         string `yaml:"user,omitempty" json:"user,omitempty"`
}

// EnvVarDefinition defines environment variables with support for database references.
type EnvVarDefinition struct {
	Key          string                 `yaml:"key" json:"key"`
	Value        string                 `yaml:"value,omitempty" json:"value,omitempty"`
	FromDatabase *DatabaseRefDefinition `yaml:"fromDatabase,omitempty" json:"from_database,omitempty"`
}

// DatabaseRefDefinition references attributes of a managed database.
type DatabaseRefDefinition struct {
	Name     string `yaml:"name" json:"name"`
	Property string `yaml:"property" json:"property"` // connectionString, host, port, user, password, database
}

// RouteDefinition defines redirect or rewrite rules.
type RouteDefinition struct {
	Type   string `yaml:"type" json:"type"` // redirect, rewrite
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
	Status int32  `yaml:"status,omitempty" json:"status,omitempty"`
}

// ParseBlueprint parses YAML content and normalizes Render or Klouds blueprint formats.
func ParseBlueprint(data []byte) (*Blueprint, error) {
	var raw struct {
		Version   string                   `yaml:"version"`
		Services  []map[string]interface{} `yaml:"services"`
		Databases []map[string]interface{} `yaml:"databases"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	bp := &Blueprint{
		Version:   raw.Version,
		Services:  make([]ServiceDefinition, 0, len(raw.Services)),
		Databases: make([]DatabaseDefinition, 0, len(raw.Databases)),
	}

	if bp.Version == "" {
		bp.Version = "1"
	}

	// Parse and normalize services
	for _, rawSvc := range raw.Services {
		svc := ServiceDefinition{}

		if name, ok := rawSvc["name"].(string); ok {
			svc.Name = name
		}

		// Normalize type (Render uses "pserv" for private service)
		if typ, ok := rawSvc["type"].(string); ok {
			switch strings.ToLower(typ) {
			case "pserv", "private":
				svc.Type = "private"
			case "worker":
				svc.Type = "worker"
			case "cron":
				svc.Type = "cron"
			case "static":
				svc.Type = "static"
			default:
				svc.Type = "web"
			}
		} else {
			svc.Type = "web"
		}

		// Runtime / Env
		if env, ok := rawSvc["env"].(string); ok {
			svc.Env = env
		} else if rt, ok := rawSvc["runtime"].(string); ok {
			svc.Env = rt
		}

		// Root Directory (Monorepo support)
		if rootDir, ok := rawSvc["rootDir"].(string); ok {
			svc.RootDir = rootDir
		} else if baseDir, ok := rawSvc["baseDir"].(string); ok {
			svc.RootDir = baseDir
		} else if dir, ok := rawSvc["directory"].(string); ok {
			svc.RootDir = dir
		} else if dir, ok := rawSvc["dir"].(string); ok {
			svc.RootDir = dir
		}

		if svc.RootDir == "" {
			svc.RootDir = "."
		}

		// Nested build block support
		if buildObj, ok := rawSvc["build"].(map[string]interface{}); ok {
			if bc, ok := buildObj["command"].(string); ok {
				svc.BuildCommand = bc
			}
			if eng, ok := buildObj["engine"].(string); ok {
				svc.Env = eng
			}
			if df, ok := buildObj["dockerfilePath"].(string); ok {
				svc.DockerfilePath = df
			}
		}

		// Nested deploy block support
		if deployObj, ok := rawSvc["deploy"].(map[string]interface{}); ok {
			if sc, ok := deployObj["command"].(string); ok {
				svc.StartCommand = sc
			}
			if p, ok := deployObj["port"].(int); ok {
				svc.Port = int32(p)
			} else if pFloat, ok := deployObj["port"].(float64); ok {
				svc.Port = int32(pFloat)
			} else if pStr, ok := deployObj["port"].(string); ok {
				var pInt int
				if _, err := fmt.Sscanf(pStr, "%d", &pInt); err == nil {
					svc.Port = int32(pInt)
				}
			}
		}

		// Top-level build and start commands
		if bc, ok := rawSvc["buildCommand"].(string); ok {
			svc.BuildCommand = bc
		}
		if sc, ok := rawSvc["startCommand"].(string); ok {
			svc.StartCommand = sc
		}
		if df, ok := rawSvc["dockerfilePath"].(string); ok {
			svc.DockerfilePath = df
		}

		// Top-level port
		if p, ok := rawSvc["port"].(int); ok {
			svc.Port = int32(p)
		} else if pFloat, ok := rawSvc["port"].(float64); ok {
			svc.Port = int32(pFloat)
		} else if pStr, ok := rawSvc["port"].(string); ok {
			var pInt int
			if _, err := fmt.Sscanf(pStr, "%d", &pInt); err == nil {
				svc.Port = int32(pInt)
			}
		}

		// Health Check
		if hc, ok := rawSvc["healthCheckPath"].(string); ok {
			svc.HealthCheckPath = hc
		}

		// Auto deploy
		if ad, ok := rawSvc["autoDeploy"].(bool); ok {
			svc.AutoDeploy = &ad
		} else {
			t := true
			svc.AutoDeploy = &t
		}

		// Build method
		if svc.Env == "docker" || svc.DockerfilePath != "" {
			svc.BuildMethod = "dockerfile"
		} else {
			svc.BuildMethod = "nixpacks"
		}

		// Env Vars: support both list of {key, value} and map {KEY: VAL} in envVars and env
		parseEnvMap := func(envMap map[string]interface{}) {
			for k, rawVal := range envMap {
				ev := EnvVarDefinition{Key: k}
				switch v := rawVal.(type) {
				case string:
					ev.Value = v
				case int:
					ev.Value = fmt.Sprintf("%d", v)
				case float64:
					ev.Value = fmt.Sprintf("%v", v)
				case bool:
					ev.Value = fmt.Sprintf("%v", v)
				case map[string]interface{}:
					if gen, ok := v["generate_value"].(bool); ok && gen {
						ev.Value = generateRandomSecret(32)
					} else if fromDb, ok := v["fromDatabase"].(map[string]interface{}); ok {
						ref := &DatabaseRefDefinition{}
						if dbName, ok := fromDb["name"].(string); ok {
							ref.Name = dbName
						}
						if prop, ok := fromDb["property"].(string); ok {
							ref.Property = prop
						}
						ev.FromDatabase = ref
					}
				}
				if ev.Key != "" {
					svc.EnvVars = append(svc.EnvVars, ev)
				}
			}
		}

		if envVarsList, ok := rawSvc["envVars"].([]interface{}); ok {
			for _, item := range envVarsList {
				if evMap, ok := item.(map[string]interface{}); ok {
					ev := EnvVarDefinition{}
					if k, ok := evMap["key"].(string); ok {
						ev.Key = k
					}
					if v, ok := evMap["value"].(string); ok {
						ev.Value = v
					}
					if fromDb, ok := evMap["fromDatabase"].(map[string]interface{}); ok {
						ref := &DatabaseRefDefinition{}
						if dbName, ok := fromDb["name"].(string); ok {
							ref.Name = dbName
						}
						if prop, ok := fromDb["property"].(string); ok {
							ref.Property = prop
						}
						ev.FromDatabase = ref
					}
					if ev.Key != "" {
						svc.EnvVars = append(svc.EnvVars, ev)
					}
				}
			}
		} else if envVarsMap, ok := rawSvc["envVars"].(map[string]interface{}); ok {
			parseEnvMap(envVarsMap)
		}

		if envList, ok := rawSvc["env"].([]interface{}); ok {
			for _, item := range envList {
				if evMap, ok := item.(map[string]interface{}); ok {
					ev := EnvVarDefinition{}
					if k, ok := evMap["key"].(string); ok {
						ev.Key = k
					}
					if v, ok := evMap["value"].(string); ok {
						ev.Value = v
					}
					if ev.Key != "" {
						svc.EnvVars = append(svc.EnvVars, ev)
					}
				}
			}
		} else if envMap, ok := rawSvc["env"].(map[string]interface{}); ok {
			parseEnvMap(envMap)
		}

		// If Port not set, check if PORT is present in EnvVars
		if svc.Port == 0 {
			for _, ev := range svc.EnvVars {
				if strings.ToUpper(ev.Key) == "PORT" && ev.Value != "" {
					var p int
					if _, err := fmt.Sscanf(ev.Value, "%d", &p); err == nil && p > 0 {
						svc.Port = int32(p)
						break
					}
				}
			}
		}

		// Fallback default ports: 3000 for web/static
		if svc.Port == 0 {
			if svc.Type == "static" {
				svc.Port = 3000
			} else {
				svc.Port = 3000
			}
		}

		// Routes (Redirects / Rewrites)
		if routes, ok := rawSvc["routes"].([]interface{}); ok {
			for _, item := range routes {
				if rMap, ok := item.(map[string]interface{}); ok {
					r := RouteDefinition{}
					if typ, ok := rMap["type"].(string); ok {
						r.Type = typ
					}
					if src, ok := rMap["source"].(string); ok {
						r.Source = src
					}
					if tgt, ok := rMap["target"].(string); ok {
						r.Target = tgt
					} else if dst, ok := rMap["destination"].(string); ok {
						r.Target = dst
					}
					if stat, ok := rMap["status"].(int); ok {
						r.Status = int32(stat)
					} else {
						r.Status = 301
					}
					if r.Source != "" && r.Target != "" {
						svc.Routes = append(svc.Routes, r)
					}
				}
			}
		}

		if svc.Name != "" {
			bp.Services = append(bp.Services, svc)
		}
	}

	// Parse databases
	for _, rawDb := range raw.Databases {
		db := DatabaseDefinition{}
		if name, ok := rawDb["name"].(string); ok {
			db.Name = name
		}
		if eng, ok := rawDb["engine"].(string); ok {
			e := strings.ToLower(strings.TrimSpace(eng))
			switch e {
			case "postgres", "postgresql", "pgsql":
				db.Engine = "postgresql"
			case "mysql":
				db.Engine = "mysql"
			case "redis":
				db.Engine = "redis"
			case "mongo", "mongodb":
				db.Engine = "mongodb"
			default:
				db.Engine = e
			}
		} else {
			db.Engine = "postgresql"
		}
		if ver, ok := rawDb["version"].(string); ok {
			db.Version = ver
		} else {
			db.Version = "16"
		}
		if dbName, ok := rawDb["databaseName"].(string); ok {
			db.DatabaseName = dbName
		}
		if user, ok := rawDb["user"].(string); ok {
			db.User = user
		}

		if db.Name != "" {
			bp.Databases = append(bp.Databases, db)
		}
	}

	return bp, nil
}

// Validate checks for required fields and unique names across services and databases.
func (b *Blueprint) Validate() error {
	if len(b.Services) == 0 && len(b.Databases) == 0 {
		return fmt.Errorf("blueprint must declare at least one service or database")
	}

	names := make(map[string]bool)
	for _, s := range b.Services {
		if s.Name == "" {
			return fmt.Errorf("service missing name")
		}
		lower := strings.ToLower(s.Name)
		if names[lower] {
			return fmt.Errorf("duplicate service name '%s'", s.Name)
		}
		names[lower] = true
	}

	for _, d := range b.Databases {
		if d.Name == "" {
			return fmt.Errorf("database missing name")
		}
		lower := strings.ToLower(d.Name)
		if names[lower] {
			return fmt.Errorf("database name '%s' conflicts with another service or database", d.Name)
		}
		names[lower] = true
	}

	return nil
}

func generateRandomSecret(length int) string {
	b := make([]byte, length/2+1)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sec_%d", time.Now().UnixNano())
	}
	res := hex.EncodeToString(b)
	if len(res) > length {
		return res[:length]
	}
	return res
}

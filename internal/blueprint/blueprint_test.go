package blueprint

import (
	"testing"
)

func TestParseBlueprintKloudsYAML(t *testing.T) {
	yamlData := `
version: "1"
services:
  - name: api-server
    type: web
    env: go
    rootDir: backend
    buildCommand: go build -o server .
    startCommand: ./server
    port: 8080
    healthCheckPath: /health
    autoDeploy: true
    routes:
      - type: redirect
        source: /old-path
        target: /new-path
        status: 301
    envVars:
      - key: PORT
        value: "8080"
      - key: DATABASE_URL
        fromDatabase:
          name: app-db
          property: connectionString

  - name: frontend-client
    type: web
    env: node
    rootDir: frontend
    buildCommand: npm run build
    startCommand: npm start
    port: 3000
    autoDeploy: true

databases:
  - name: app-db
    engine: postgresql
    version: "16"
`

	bp, err := ParseBlueprint([]byte(yamlData))
	if err != nil {
		t.Fatalf("unexpected error parsing blueprint: %v", err)
	}

	if len(bp.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(bp.Services))
	}

	if len(bp.Databases) != 1 {
		t.Fatalf("expected 1 database, got %d", len(bp.Databases))
	}

	// Verify api-server
	api := bp.Services[0]
	if api.Name != "api-server" {
		t.Errorf("expected api-server, got %s", api.Name)
	}
	if api.RootDir != "backend" {
		t.Errorf("expected rootDir 'backend', got %s", api.RootDir)
	}
	if api.Port != 8080 {
		t.Errorf("expected port 8080, got %d", api.Port)
	}
	if len(api.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(api.Routes))
	}
	if len(api.EnvVars) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(api.EnvVars))
	}

	// Verify frontend-client
	fe := bp.Services[1]
	if fe.Name != "frontend-client" {
		t.Errorf("expected frontend-client, got %s", fe.Name)
	}
	if fe.RootDir != "frontend" {
		t.Errorf("expected rootDir 'frontend', got %s", fe.RootDir)
	}
}

func TestParseBlueprintRenderYAMLCompatibility(t *testing.T) {
	renderYAML := `
services:
  - type: web
    name: webapp
    runtime: node
    rootDir: apps/web
    buildCommand: yarn build
    startCommand: yarn start
    envVars:
      - key: NODE_ENV
        value: production
  - type: pserv
    name: worker-service
    runtime: python
    baseDir: services/worker
    startCommand: python main.py
`

	bp, err := ParseBlueprint([]byte(renderYAML))
	if err != nil {
		t.Fatalf("unexpected error parsing render blueprint: %v", err)
	}

	if len(bp.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(bp.Services))
	}

	// First service (web)
	s1 := bp.Services[0]
	if s1.Name != "webapp" || s1.Type != "web" || s1.RootDir != "apps/web" {
		t.Errorf("unexpected service 1: %+v", s1)
	}

	// Second service (pserv -> private)
	s2 := bp.Services[1]
	if s2.Name != "worker-service" || s2.Type != "private" || s2.RootDir != "services/worker" {
		t.Errorf("unexpected service 2 (pserv -> private mapping): %+v", s2)
	}
}

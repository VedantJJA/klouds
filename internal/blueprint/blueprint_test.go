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

func TestDetectFromRepoLive(t *testing.T) {
	ctx := t.Context()
	res, err := DetectFromRepo(ctx, "https://github.com/VedantJJA/DevPtestDynrepo", "main", "./data")
	if err != nil {
		t.Fatalf("unexpected error scanning repo: %v", err)
	}

	if res.Source != "blueprint" {
		t.Errorf("expected source 'blueprint', got '%s'", res.Source)
	}

	if len(res.Blueprint.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(res.Blueprint.Services))
	}

	if len(res.Blueprint.Databases) != 1 {
		t.Errorf("expected 1 database, got %d", len(res.Blueprint.Databases))
	}
}

func TestParseBlueprintNestedMaps(t *testing.T) {
	yamlData := `
version: "1.0"
services:
  - name: vtopcc-backend
    type: web
    directory: "backend"
    build:
      engine: "node"
      command: "npm install && npm run build"
    deploy:
      port: 5000
      command: "npm run start"
    env:
      NODE_ENV: "production"
      PORT: "5000"
      JWT_SECRET:
        generate_value: true

  - name: vtopcc
    type: static
    directory: "frontend"
    build:
      command: "npm install && npm run build"
      output_dir: "dist"
    env:
      VITE_API_URL: "${services.vtopcc-backend.url}"
    routes:
      - type: rewrite
        source: "/api/*"
        destination: "${services.vtopcc-backend.url}/api/*"
      - type: rewrite
        source: "/*"
        destination: "/index.html"
`

	bp, err := ParseBlueprint([]byte(yamlData))
	if err != nil {
		t.Fatalf("unexpected error parsing blueprint: %v", err)
	}

	if len(bp.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(bp.Services))
	}

	b := bp.Services[0]
	if b.Name != "vtopcc-backend" || b.RootDir != "backend" || b.Port != 5000 {
		t.Errorf("unexpected backend service: %+v", b)
	}
	if b.BuildCommand != "npm install && npm run build" || b.StartCommand != "npm run start" {
		t.Errorf("unexpected backend commands: bc=%q sc=%q", b.BuildCommand, b.StartCommand)
	}

	f := bp.Services[1]
	if f.Name != "vtopcc" || f.RootDir != "frontend" || f.Type != "static" || f.Port != 80 {
		t.Errorf("unexpected frontend service: %+v", f)
	}
	if len(f.Routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(f.Routes))
	}
}

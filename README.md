# Klouds

A lightweight, self-hosted PaaS (Platform as a Service) built in Go, combining the best of Dokploy and Render. Optimized for ARM64 and constrained environments.

## Features

- **Git Push-to-Deploy** - Connect a GitHub repo, deploy on push
- **Nixpacks Auto-Build** - Auto-detect language, build without Dockerfile
- **Managed Databases** - PostgreSQL, MySQL, Redis, MongoDB
- **Wildcard Subdomain Routing** - `<slug>.yourdomain.com` for each service
- **Automated SSL/TLS** - On-demand Let's Encrypt / ZeroSSL certificate management
- **Redirects & Rewrites** - First-class routing rules via Caddy
- **Admin Approval System** - Users register, admin approves
- **Resource Quotas** - Per-user CPU/memory/disk limits
- **VM Metrics Dashboard** - Live CPU, RAM, disk monitoring
- **Blueprint IaC** - `klouds.yaml` to declare your entire stack
- **Single Port Architecture** - Only port 443 exposed
- **ARM64 Native** - Built for Oracle Ampere A1

## Quick Start

### Prerequisites

- Go 1.23+
- Docker Engine
- PostgreSQL 16
- Caddy v2 (optional for dev)

### Development

```bash
# 1. Start PostgreSQL
docker compose -f deploy/docker-compose.dev.yml up -d postgres

# 2. Copy and edit environment config
cp .env.example .env
# Edit .env with your values

# 3. Run the API server
go run ./cmd/klouds-server/

# Or with Task runner:
task dev
```

### Build

```bash
# Build for current platform
go build -ldflags="-s -w" -o bin/klouds-server ./cmd/klouds-server/

# Cross-compile for ARM64 (Oracle Ampere A1)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/klouds-server-arm64 ./cmd/klouds-server/
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/auth/register` | Register a new user |
| `POST` | `/api/auth/login` | Login |
| `GET` | `/api/auth/me` | Get current user |
| `POST` | `/api/projects` | Create a project |
| `GET` | `/api/projects` | List projects |
| `POST` | `/api/services` | Create a service |
| `GET` | `/api/services` | List services |
| `POST` | `/api/services/:id/stop` | Stop a service |
| `POST` | `/api/services/:id/restart` | Restart a service |
| `POST` | `/api/databases` | Create a managed database |
| `GET` | `/api/databases/:id/connection` | Get connection info |
| `GET` | `/api/admin/users/pending` | List pending approvals |
| `PATCH` | `/api/admin/users/:id/status` | Approve/suspend user |
| `GET` | `/api/admin/metrics/system` | System metrics |

## Architecture

- **API Server**: Go (chi router, pgx, zerolog) - ~13 MB binary, ~20 MB RAM
- **Reverse Proxy**: Caddy v2 - auto HTTPS, redirects, rewrites
- **Database**: PostgreSQL (system) + managed user databases
- **Containers**: Docker Engine with resource limits
- **Auth**: HMAC-SHA256 signed tokens, bcrypt passwords
- **Secrets**: AES-256-GCM encrypted at rest

## License

MIT

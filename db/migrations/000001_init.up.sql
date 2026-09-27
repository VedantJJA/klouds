-- 000001_init.up.sql
-- Initial schema for Klouds platform database

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE users (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    email       TEXT NOT NULL UNIQUE,
    username    TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'suspended')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);

-- ============================================================
-- USER QUOTAS
-- ============================================================
CREATE TABLE user_quotas (
    id                   TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id              TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    max_services         INT NOT NULL DEFAULT 3,
    max_databases        INT NOT NULL DEFAULT 2,
    cpu_per_container    INT NOT NULL DEFAULT 500,          -- millicores
    memory_per_container BIGINT NOT NULL DEFAULT 268435456, -- 256 MB
    total_memory         BIGINT NOT NULL DEFAULT 1073741824, -- 1 GB
    disk_per_service     BIGINT NOT NULL DEFAULT 1073741824, -- 1 GB
    disk_per_database    BIGINT NOT NULL DEFAULT 2147483648, -- 2 GB
    build_timeout_secs   INT NOT NULL DEFAULT 600,          -- 10 min
    max_deploys_per_day  INT NOT NULL DEFAULT 20,
    max_custom_domains   INT NOT NULL DEFAULT 2
);

-- ============================================================
-- PROJECTS
-- ============================================================
CREATE TABLE projects (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_user ON projects(user_id);
CREATE INDEX idx_projects_slug ON projects(slug);

-- ============================================================
-- SERVICES
-- ============================================================
CREATE TABLE services (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id       TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    slug             TEXT NOT NULL UNIQUE,
    type             TEXT NOT NULL DEFAULT 'web' CHECK (type IN ('web', 'worker', 'cron', 'static', 'private')),
    status           TEXT NOT NULL DEFAULT 'created' CHECK (status IN ('created', 'building', 'running', 'stopped', 'failed', 'deploying')),
    build_method     TEXT NOT NULL DEFAULT 'nixpacks' CHECK (build_method IN ('nixpacks', 'dockerfile')),
    repo_url         TEXT,
    branch           TEXT DEFAULT 'main',
    dockerfile_path  TEXT DEFAULT 'Dockerfile',
    build_command    TEXT,
    start_command    TEXT,
    port             INT DEFAULT 3000,
    health_check_path TEXT DEFAULT '/health',
    auto_deploy      BOOLEAN NOT NULL DEFAULT true,
    container_id     TEXT,
    image_tag        TEXT,
    subdomain        TEXT NOT NULL UNIQUE,
    cpu_limit        INT NOT NULL DEFAULT 500,
    memory_limit     BIGINT NOT NULL DEFAULT 268435456,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_services_project ON services(project_id);
CREATE INDEX idx_services_user ON services(user_id);
CREATE INDEX idx_services_slug ON services(slug);
CREATE INDEX idx_services_subdomain ON services(subdomain);

-- ============================================================
-- DATABASES (Managed)
-- ============================================================
CREATE TABLE databases (
    id                 TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id            TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    slug               TEXT NOT NULL UNIQUE,
    engine             TEXT NOT NULL CHECK (engine IN ('postgresql', 'mysql', 'redis', 'mongodb')),
    version            TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'creating' CHECK (status IN ('creating', 'running', 'stopped', 'failed')),
    container_id       TEXT,
    internal_host      TEXT NOT NULL,
    internal_port      INT NOT NULL,
    external_access    BOOLEAN NOT NULL DEFAULT false,
    external_subdomain TEXT,
    database_name      TEXT NOT NULL,
    username           TEXT NOT NULL,
    password_encrypted TEXT NOT NULL,
    cpu_limit          INT NOT NULL DEFAULT 500,
    memory_limit       BIGINT NOT NULL DEFAULT 268435456,
    disk_limit         BIGINT NOT NULL DEFAULT 2147483648,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_databases_project ON databases(project_id);
CREATE INDEX idx_databases_user ON databases(user_id);
CREATE INDEX idx_databases_slug ON databases(slug);

-- ============================================================
-- DEPLOYMENTS
-- ============================================================
CREATE TABLE deployments (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    service_id   TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'building', 'live', 'failed', 'rolled_back')),
    commit_sha   TEXT,
    commit_msg   TEXT,
    image_tag    TEXT NOT NULL,
    trigger      TEXT NOT NULL DEFAULT 'manual' CHECK (trigger IN ('webhook', 'manual', 'rollback', 'blueprint')),
    build_log    TEXT,
    duration_sec INT DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at  TIMESTAMPTZ
);

CREATE INDEX idx_deployments_service ON deployments(service_id);
CREATE INDEX idx_deployments_user ON deployments(user_id);

-- ============================================================
-- ENVIRONMENT VARIABLES
-- ============================================================
CREATE TABLE env_vars (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    service_id      TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    value_encrypted TEXT NOT NULL,
    is_build_time   BOOLEAN NOT NULL DEFAULT false,
    is_linked       BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(service_id, key)
);

CREATE INDEX idx_env_vars_service ON env_vars(service_id);

-- ============================================================
-- ROUTE RULES (Redirects / Rewrites)
-- ============================================================
CREATE TABLE route_rules (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('redirect', 'rewrite')),
    source     TEXT NOT NULL,
    target     TEXT NOT NULL,
    status     INT DEFAULT 301,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_route_rules_service ON route_rules(service_id);

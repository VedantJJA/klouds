-- 000004_add_minecraft_servers.up.sql
-- Minecraft Server Hosting schema

CREATE TABLE IF NOT EXISTS minecraft_servers (
    id                TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    slug              TEXT NOT NULL UNIQUE,
    server_type       TEXT NOT NULL DEFAULT 'PAPER' CHECK (server_type IN ('PAPER', 'VANILLA', 'FABRIC', 'FORGE', 'PURPUR', 'SPIGOT', 'BEDROCK')),
    version           TEXT NOT NULL DEFAULT '1.21.4',
    status            TEXT NOT NULL DEFAULT 'stopped' CHECK (status IN ('creating', 'running', 'stopped', 'failed')),
    container_id      TEXT,
    server_port       INT NOT NULL UNIQUE,
    rcon_port         INT NOT NULL DEFAULT 25575,
    rcon_password     TEXT NOT NULL DEFAULT '',
    motd              TEXT NOT NULL DEFAULT 'A Minecraft Server hosted on Klouds',
    difficulty        TEXT NOT NULL DEFAULT 'normal' CHECK (difficulty IN ('peaceful', 'easy', 'normal', 'hard')),
    game_mode         TEXT NOT NULL DEFAULT 'survival' CHECK (game_mode IN ('survival', 'creative', 'adventure', 'spectator')),
    max_players       INT NOT NULL DEFAULT 20,
    pvp               BOOLEAN NOT NULL DEFAULT true,
    online_mode       BOOLEAN NOT NULL DEFAULT true,
    whitelist_enabled BOOLEAN NOT NULL DEFAULT false,
    cpu_limit         INT NOT NULL DEFAULT 1000,
    memory_limit      BIGINT NOT NULL DEFAULT 2147483648,
    disk_limit        BIGINT NOT NULL DEFAULT 10737418240,
    ops               TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mc_project ON minecraft_servers(project_id);
CREATE INDEX IF NOT EXISTS idx_mc_user ON minecraft_servers(user_id);
CREATE INDEX IF NOT EXISTS idx_mc_slug ON minecraft_servers(slug);
CREATE INDEX IF NOT EXISTS idx_mc_port ON minecraft_servers(server_port);

ALTER TABLE user_quotas ADD COLUMN IF NOT EXISTS max_minecraft_servers INT NOT NULL DEFAULT 3;

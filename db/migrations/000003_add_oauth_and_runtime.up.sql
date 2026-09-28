-- 000003_add_oauth_and_runtime.up.sql

-- Add runtime_version to services
ALTER TABLE services ADD COLUMN IF NOT EXISTS runtime_version TEXT NOT NULL DEFAULT '';

-- Admin-configured OAuth providers (GitHub, GitLab, Bitbucket)
CREATE TABLE IF NOT EXISTS oauth_provider_configs (
    provider                TEXT PRIMARY KEY CHECK (provider IN ('github', 'gitlab', 'bitbucket')),
    client_id               TEXT NOT NULL DEFAULT '',
    client_secret_encrypted TEXT NOT NULL DEFAULT '',
    auth_url                TEXT,
    token_url               TEXT,
    api_url                 TEXT,
    enabled                 BOOLEAN NOT NULL DEFAULT false,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- User-linked OAuth accounts
CREATE TABLE IF NOT EXISTS user_oauth_accounts (
    id                      TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider                TEXT NOT NULL CHECK (provider IN ('github', 'gitlab', 'bitbucket')),
    provider_user_id        TEXT NOT NULL,
    provider_username       TEXT NOT NULL DEFAULT '',
    provider_email          TEXT NOT NULL DEFAULT '',
    avatar_url              TEXT NOT NULL DEFAULT '',
    access_token_encrypted  TEXT NOT NULL,
    refresh_token_encrypted TEXT,
    token_expires_at        TIMESTAMPTZ,
    scopes                  TEXT[] NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_user_id),
    UNIQUE(user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_user_oauth_user ON user_oauth_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_user_oauth_provider ON user_oauth_accounts(provider);

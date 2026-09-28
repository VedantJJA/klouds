-- 000003_add_oauth_and_runtime.down.sql

DROP TABLE IF EXISTS user_oauth_accounts;
DROP TABLE IF EXISTS oauth_provider_configs;
ALTER TABLE services DROP COLUMN IF EXISTS runtime_version;

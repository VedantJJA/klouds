-- 000004_add_minecraft_servers.down.sql

DROP TABLE IF EXISTS minecraft_servers;
ALTER TABLE user_quotas DROP COLUMN IF EXISTS max_minecraft_servers;

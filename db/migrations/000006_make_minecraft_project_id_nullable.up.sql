-- 000006_make_minecraft_project_id_nullable.up.sql
-- Decouple minecraft_servers from projects so Minecraft can run independently.

ALTER TABLE minecraft_servers ALTER COLUMN project_id DROP NOT NULL;

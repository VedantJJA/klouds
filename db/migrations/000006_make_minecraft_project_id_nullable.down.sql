-- 000006_make_minecraft_project_id_nullable.down.sql
-- Re-add NOT NULL constraint to project_id in minecraft_servers.

ALTER TABLE minecraft_servers ALTER COLUMN project_id SET NOT NULL;

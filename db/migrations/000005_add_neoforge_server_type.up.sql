-- 000005_add_neoforge_server_type.up.sql
ALTER TABLE minecraft_servers DROP CONSTRAINT IF EXISTS minecraft_servers_server_type_check;
ALTER TABLE minecraft_servers ADD CONSTRAINT minecraft_servers_server_type_check CHECK (server_type IN ('PAPER', 'VANILLA', 'FABRIC', 'FORGE', 'NEOFORGE', 'PURPUR', 'SPIGOT', 'BEDROCK'));

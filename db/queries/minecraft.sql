-- name: CreateMinecraftServer :one
INSERT INTO minecraft_servers (
    project_id, user_id, name, slug, server_type, version,
    status, container_id, server_port, rcon_port, rcon_password,
    motd, difficulty, game_mode, max_players, pvp, online_mode,
    whitelist_enabled, cpu_limit, memory_limit, disk_limit, ops
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16, $17,
    $18, $19, $20, $21, $22
)
RETURNING *;

-- name: GetMinecraftServerByID :one
SELECT * FROM minecraft_servers WHERE id = $1;

-- name: GetMinecraftServerBySlug :one
SELECT * FROM minecraft_servers WHERE slug = $1;

-- name: ListMinecraftServersByProject :many
SELECT * FROM minecraft_servers WHERE project_id = $1 ORDER BY created_at DESC;

-- name: ListMinecraftServersByUser :many
SELECT * FROM minecraft_servers WHERE user_id = $1 ORDER BY created_at DESC;

-- name: ListAllMinecraftServers :many
SELECT * FROM minecraft_servers ORDER BY created_at DESC;

-- name: UpdateMinecraftServerStatus :one
UPDATE minecraft_servers SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateMinecraftServerContainer :one
UPDATE minecraft_servers SET container_id = $2, status = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateMinecraftServerConfig :one
UPDATE minecraft_servers SET
    name = $2,
    motd = $3,
    difficulty = $4,
    game_mode = $5,
    max_players = $6,
    pvp = $7,
    online_mode = $8,
    whitelist_enabled = $9,
    server_type = $10,
    version = $11,
    cpu_limit = $12,
    memory_limit = $13,
    ops = $14,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteMinecraftServer :exec
DELETE FROM minecraft_servers WHERE id = $1;

-- name: CountMinecraftServersByUser :one
SELECT COUNT(*) FROM minecraft_servers WHERE user_id = $1;

-- name: GetAllocatedMinecraftPorts :many
SELECT server_port FROM minecraft_servers ORDER BY server_port ASC;

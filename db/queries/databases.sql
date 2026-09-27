-- name: CreateDatabase :one
INSERT INTO databases (
    project_id, user_id, name, slug, engine, version,
    internal_host, internal_port, database_name, username,
    password_encrypted, cpu_limit, memory_limit, disk_limit
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetDatabaseByID :one
SELECT * FROM databases WHERE id = $1;

-- name: GetDatabaseBySlug :one
SELECT * FROM databases WHERE slug = $1;

-- name: ListDatabasesByProject :many
SELECT * FROM databases WHERE project_id = $1 ORDER BY created_at DESC;

-- name: ListDatabasesByUser :many
SELECT * FROM databases WHERE user_id = $1 ORDER BY created_at DESC;

-- name: ListAllDatabases :many
SELECT * FROM databases ORDER BY created_at DESC;

-- name: UpdateDatabaseStatus :one
UPDATE databases SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateDatabaseContainer :one
UPDATE databases SET container_id = $2, status = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateDatabaseExternalAccess :one
UPDATE databases SET external_access = $2, external_subdomain = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteDatabase :exec
DELETE FROM databases WHERE id = $1;

-- name: CountDatabasesByUser :one
SELECT COUNT(*) FROM databases WHERE user_id = $1;

-- name: CreateService :one
INSERT INTO services (
    project_id, user_id, name, slug, type, build_method,
    repo_url, branch, dockerfile_path, build_command, start_command,
    port, health_check_path, auto_deploy, subdomain, cpu_limit, memory_limit
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: GetServiceByID :one
SELECT * FROM services WHERE id = $1;

-- name: GetServiceBySlug :one
SELECT * FROM services WHERE slug = $1;

-- name: GetServiceBySubdomain :one
SELECT * FROM services WHERE subdomain = $1;

-- name: ListServicesByProject :many
SELECT * FROM services WHERE project_id = $1 ORDER BY created_at DESC;

-- name: ListServicesByUser :many
SELECT * FROM services WHERE user_id = $1 ORDER BY created_at DESC;

-- name: ListAllServices :many
SELECT * FROM services ORDER BY created_at DESC;

-- name: ListRunningServices :many
SELECT * FROM services WHERE status = 'running' ORDER BY created_at DESC;

-- name: UpdateServiceStatus :one
UPDATE services SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateServiceContainer :one
UPDATE services SET container_id = $2, image_tag = $3, status = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateService :one
UPDATE services SET
    name = $2, repo_url = $3, branch = $4, build_method = $5,
    dockerfile_path = $6, build_command = $7, start_command = $8,
    port = $9, health_check_path = $10, auto_deploy = $11,
    cpu_limit = $12, memory_limit = $13, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteService :exec
DELETE FROM services WHERE id = $1;

-- name: CountServicesByUser :one
SELECT COUNT(*) FROM services WHERE user_id = $1;

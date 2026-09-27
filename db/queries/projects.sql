-- name: CreateProject :one
INSERT INTO projects (user_id, name, slug)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProjectByID :one
SELECT * FROM projects WHERE id = $1;

-- name: GetProjectBySlug :one
SELECT * FROM projects WHERE slug = $1;

-- name: ListProjectsByUser :many
SELECT * FROM projects WHERE user_id = $1 ORDER BY created_at DESC;

-- name: ListAllProjects :many
SELECT * FROM projects ORDER BY created_at DESC;

-- name: UpdateProject :one
UPDATE projects SET name = $2, slug = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: CountProjectsByUser :one
SELECT COUNT(*) FROM projects WHERE user_id = $1;

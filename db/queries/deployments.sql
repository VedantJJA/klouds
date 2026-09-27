-- name: CreateDeployment :one
INSERT INTO deployments (service_id, user_id, status, commit_sha, commit_msg, image_tag, trigger)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetDeploymentByID :one
SELECT * FROM deployments WHERE id = $1;

-- name: ListDeploymentsByService :many
SELECT * FROM deployments WHERE service_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListDeploymentsByUser :many
SELECT * FROM deployments WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListAllDeployments :many
SELECT * FROM deployments ORDER BY created_at DESC LIMIT $1;

-- name: UpdateDeploymentStatus :one
UPDATE deployments SET status = $2, finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateDeploymentLog :one
UPDATE deployments SET build_log = $2
WHERE id = $1
RETURNING *;

-- name: UpdateDeploymentFinished :one
UPDATE deployments SET status = $2, duration_sec = $3, finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CountDeploymentsToday :one
SELECT COUNT(*) FROM deployments
WHERE user_id = $1 AND created_at >= CURRENT_DATE;

-- name: GetLatestDeployment :one
SELECT * FROM deployments WHERE service_id = $1 AND status = 'live'
ORDER BY created_at DESC LIMIT 1;

-- name: CreateUserQuota :one
INSERT INTO user_quotas (
    user_id, max_services, max_databases, cpu_per_container,
    memory_per_container, total_memory, disk_per_service,
    disk_per_database, build_timeout_secs, max_deploys_per_day, max_custom_domains
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetUserQuota :one
SELECT * FROM user_quotas WHERE user_id = $1;

-- name: UpdateUserQuota :one
UPDATE user_quotas SET
    max_services = $2,
    max_databases = $3,
    cpu_per_container = $4,
    memory_per_container = $5,
    total_memory = $6,
    disk_per_service = $7,
    disk_per_database = $8,
    build_timeout_secs = $9,
    max_deploys_per_day = $10,
    max_custom_domains = $11
WHERE user_id = $1
RETURNING *;

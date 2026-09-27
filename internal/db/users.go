package db

import (
	"context"
)

// ============================================================
// User Queries
// ============================================================

type CreateUserParams struct {
	Email        string
	Username     string
	PasswordHash string
	Role         string
	Status       string
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO users (email, username, password_hash, role, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, username, password_hash, role, status, created_at, updated_at`,
		arg.Email, arg.Username, arg.PasswordHash, arg.Role, arg.Status,
	)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (q *Queries) GetUserByID(ctx context.Context, id string) (User, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, email, username, password_hash, role, status, created_at, updated_at
		 FROM users WHERE id = $1`, id)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, email, username, password_hash, role, status, created_at, updated_at
		 FROM users WHERE email = $1`, email)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (q *Queries) GetUserByUsername(ctx context.Context, username string) (User, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, email, username, password_hash, role, status, created_at, updated_at
		 FROM users WHERE username = $1`, username)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (q *Queries) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, email, username, password_hash, role, status, created_at, updated_at
		 FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, rows.Err()
}

func (q *Queries) ListPendingUsers(ctx context.Context) ([]User, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, email, username, password_hash, role, status, created_at, updated_at
		 FROM users WHERE status = 'pending' ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, rows.Err()
}

func (q *Queries) UpdateUserStatus(ctx context.Context, id string, status string) (User, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE users SET status = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, email, username, password_hash, role, status, created_at, updated_at`,
		id, status)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (q *Queries) DeleteUser(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (q *Queries) CountUsers(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// ============================================================
// Quota Queries
// ============================================================

type CreateUserQuotaParams struct {
	UserID             string
	MaxServices        int32
	MaxDatabases       int32
	CpuPerContainer    int32
	MemoryPerContainer int64
	TotalMemory        int64
	DiskPerService     int64
	DiskPerDatabase    int64
	BuildTimeoutSecs   int32
	MaxDeploysPerDay   int32
	MaxCustomDomains   int32
}

func (q *Queries) CreateUserQuota(ctx context.Context, arg CreateUserQuotaParams) (UserQuota, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO user_quotas (
			user_id, max_services, max_databases, cpu_per_container,
			memory_per_container, total_memory, disk_per_service,
			disk_per_database, build_timeout_secs, max_deploys_per_day, max_custom_domains
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, user_id, max_services, max_databases, cpu_per_container,
			memory_per_container, total_memory, disk_per_service,
			disk_per_database, build_timeout_secs, max_deploys_per_day, max_custom_domains`,
		arg.UserID, arg.MaxServices, arg.MaxDatabases, arg.CpuPerContainer,
		arg.MemoryPerContainer, arg.TotalMemory, arg.DiskPerService,
		arg.DiskPerDatabase, arg.BuildTimeoutSecs, arg.MaxDeploysPerDay, arg.MaxCustomDomains,
	)
	var uq UserQuota
	err := row.Scan(&uq.ID, &uq.UserID, &uq.MaxServices, &uq.MaxDatabases, &uq.CpuPerContainer,
		&uq.MemoryPerContainer, &uq.TotalMemory, &uq.DiskPerService,
		&uq.DiskPerDatabase, &uq.BuildTimeoutSecs, &uq.MaxDeploysPerDay, &uq.MaxCustomDomains)
	return uq, err
}

func (q *Queries) GetUserQuota(ctx context.Context, userID string) (UserQuota, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, user_id, max_services, max_databases, cpu_per_container,
			memory_per_container, total_memory, disk_per_service,
			disk_per_database, build_timeout_secs, max_deploys_per_day, max_custom_domains
		FROM user_quotas WHERE user_id = $1`, userID)
	var uq UserQuota
	err := row.Scan(&uq.ID, &uq.UserID, &uq.MaxServices, &uq.MaxDatabases, &uq.CpuPerContainer,
		&uq.MemoryPerContainer, &uq.TotalMemory, &uq.DiskPerService,
		&uq.DiskPerDatabase, &uq.BuildTimeoutSecs, &uq.MaxDeploysPerDay, &uq.MaxCustomDomains)
	return uq, err
}

func (q *Queries) UpdateUserQuota(ctx context.Context, arg CreateUserQuotaParams) (UserQuota, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE user_quotas SET
			max_services = $2, max_databases = $3, cpu_per_container = $4,
			memory_per_container = $5, total_memory = $6, disk_per_service = $7,
			disk_per_database = $8, build_timeout_secs = $9, max_deploys_per_day = $10,
			max_custom_domains = $11
		WHERE user_id = $1
		RETURNING id, user_id, max_services, max_databases, cpu_per_container,
			memory_per_container, total_memory, disk_per_service,
			disk_per_database, build_timeout_secs, max_deploys_per_day, max_custom_domains`,
		arg.UserID, arg.MaxServices, arg.MaxDatabases, arg.CpuPerContainer,
		arg.MemoryPerContainer, arg.TotalMemory, arg.DiskPerService,
		arg.DiskPerDatabase, arg.BuildTimeoutSecs, arg.MaxDeploysPerDay, arg.MaxCustomDomains,
	)
	var uq UserQuota
	err := row.Scan(&uq.ID, &uq.UserID, &uq.MaxServices, &uq.MaxDatabases, &uq.CpuPerContainer,
		&uq.MemoryPerContainer, &uq.TotalMemory, &uq.DiskPerService,
		&uq.DiskPerDatabase, &uq.BuildTimeoutSecs, &uq.MaxDeploysPerDay, &uq.MaxCustomDomains)
	return uq, err
}

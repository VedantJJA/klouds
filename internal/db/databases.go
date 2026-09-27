package db

import (
	"context"
)

// ============================================================
// Database (Managed DB) Queries
// ============================================================

type CreateDatabaseParams struct {
	ProjectID         string
	UserID            string
	Name              string
	Slug              string
	Engine            string
	Version           string
	InternalHost      string
	InternalPort      int32
	DatabaseName      string
	Username          string
	PasswordEncrypted string
	CpuLimit          int32
	MemoryLimit       int64
	DiskLimit         int64
}

func (q *Queries) CreateDatabase(ctx context.Context, arg CreateDatabaseParams) (Database, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO databases (
			project_id, user_id, name, slug, engine, version,
			internal_host, internal_port, database_name, username,
			password_encrypted, cpu_limit, memory_limit, disk_limit
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, project_id, user_id, name, slug, engine, version, status,
			container_id, internal_host, internal_port, external_access, external_subdomain,
			database_name, username, password_encrypted, cpu_limit, memory_limit, disk_limit,
			created_at, updated_at`,
		arg.ProjectID, arg.UserID, arg.Name, arg.Slug, arg.Engine, arg.Version,
		arg.InternalHost, arg.InternalPort, arg.DatabaseName, arg.Username,
		arg.PasswordEncrypted, arg.CpuLimit, arg.MemoryLimit, arg.DiskLimit,
	)
	return scanDatabase(row)
}

func (q *Queries) GetDatabaseByID(ctx context.Context, id string) (Database, error) {
	row := q.db.QueryRow(ctx, selectDatabaseSQL+` WHERE id = $1`, id)
	return scanDatabase(row)
}

func (q *Queries) GetDatabaseBySlug(ctx context.Context, slug string) (Database, error) {
	row := q.db.QueryRow(ctx, selectDatabaseSQL+` WHERE slug = $1`, slug)
	return scanDatabase(row)
}

func (q *Queries) ListDatabasesByProject(ctx context.Context, projectID string) ([]Database, error) {
	rows, err := q.db.Query(ctx, selectDatabaseSQL+` WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDatabases(rows)
}

func (q *Queries) ListDatabasesByUser(ctx context.Context, userID string) ([]Database, error) {
	rows, err := q.db.Query(ctx, selectDatabaseSQL+` WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDatabases(rows)
}

func (q *Queries) ListAllDatabases(ctx context.Context) ([]Database, error) {
	rows, err := q.db.Query(ctx, selectDatabaseSQL+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDatabases(rows)
}

func (q *Queries) UpdateDatabaseStatus(ctx context.Context, id string, status string) (Database, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE databases SET status = $2, updated_at = NOW() WHERE id = $1
		 RETURNING id, project_id, user_id, name, slug, engine, version, status,
			container_id, internal_host, internal_port, external_access, external_subdomain,
			database_name, username, password_encrypted, cpu_limit, memory_limit, disk_limit,
			created_at, updated_at`,
		id, status)
	return scanDatabase(row)
}

func (q *Queries) UpdateDatabaseContainer(ctx context.Context, id string, containerID *string, status string) (Database, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE databases SET container_id = $2, status = $3, updated_at = NOW() WHERE id = $1
		 RETURNING id, project_id, user_id, name, slug, engine, version, status,
			container_id, internal_host, internal_port, external_access, external_subdomain,
			database_name, username, password_encrypted, cpu_limit, memory_limit, disk_limit,
			created_at, updated_at`,
		id, containerID, status)
	return scanDatabase(row)
}

func (q *Queries) DeleteDatabase(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM databases WHERE id = $1`, id)
	return err
}

func (q *Queries) CountDatabasesByUser(ctx context.Context, userID string) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM databases WHERE user_id = $1`, userID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// ============================================================
// Deployment Queries
// ============================================================

type CreateDeploymentParams struct {
	ServiceID string
	UserID    string
	Status    string
	CommitSHA *string
	CommitMsg *string
	ImageTag  string
	Trigger   string
}

func (q *Queries) CreateDeployment(ctx context.Context, arg CreateDeploymentParams) (Deployment, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO deployments (service_id, user_id, status, commit_sha, commit_msg, image_tag, trigger)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, service_id, user_id, status, commit_sha, commit_msg, image_tag, trigger,
			build_log, duration_sec, created_at, finished_at`,
		arg.ServiceID, arg.UserID, arg.Status, arg.CommitSHA, arg.CommitMsg, arg.ImageTag, arg.Trigger,
	)
	return scanDeployment(row)
}

func (q *Queries) GetDeploymentByID(ctx context.Context, id string) (Deployment, error) {
	row := q.db.QueryRow(ctx, selectDeploymentSQL+` WHERE id = $1`, id)
	return scanDeployment(row)
}

func (q *Queries) ListDeploymentsByService(ctx context.Context, serviceID string, limit int32) ([]Deployment, error) {
	rows, err := q.db.Query(ctx,
		selectDeploymentSQL+` WHERE service_id = $1 ORDER BY created_at DESC LIMIT $2`,
		serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeployments(rows)
}

func (q *Queries) ListAllDeployments(ctx context.Context, limit int32) ([]Deployment, error) {
	rows, err := q.db.Query(ctx,
		selectDeploymentSQL+` ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeployments(rows)
}

func (q *Queries) UpdateDeploymentFinished(ctx context.Context, id string, status string, durationSec int32) (Deployment, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE deployments SET status = $2, duration_sec = $3, finished_at = NOW()
		 WHERE id = $1
		 RETURNING id, service_id, user_id, status, commit_sha, commit_msg, image_tag, trigger,
			build_log, duration_sec, created_at, finished_at`,
		id, status, durationSec)
	return scanDeployment(row)
}

func (q *Queries) UpdateDeploymentLog(ctx context.Context, id string, buildLog string) error {
	_, err := q.db.Exec(ctx, `UPDATE deployments SET build_log = $2 WHERE id = $1`, id, buildLog)
	return err
}

func (q *Queries) CountDeploymentsToday(ctx context.Context, userID string) (int64, error) {
	row := q.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM deployments WHERE user_id = $1 AND created_at >= CURRENT_DATE`,
		userID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// --- constants and scan helpers ---

const selectDatabaseSQL = `SELECT id, project_id, user_id, name, slug, engine, version, status,
	container_id, internal_host, internal_port, external_access, external_subdomain,
	database_name, username, password_encrypted, cpu_limit, memory_limit, disk_limit,
	created_at, updated_at
	FROM databases`

const selectDeploymentSQL = `SELECT id, service_id, user_id, status, commit_sha, commit_msg, image_tag, trigger,
	build_log, duration_sec, created_at, finished_at
	FROM deployments`

func scanDatabase(row interface{ Scan(dest ...interface{}) error }) (Database, error) {
	var d Database
	err := row.Scan(
		&d.ID, &d.ProjectID, &d.UserID, &d.Name, &d.Slug, &d.Engine, &d.Version, &d.Status,
		&d.ContainerID, &d.InternalHost, &d.InternalPort, &d.ExternalAccess, &d.ExternalSubdomain,
		&d.DatabaseName, &d.Username, &d.PasswordEncrypted, &d.CpuLimit, &d.MemoryLimit, &d.DiskLimit,
		&d.CreatedAt, &d.UpdatedAt,
	)
	return d, err
}

func scanDatabases(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]Database, error) {
	var dbs []Database
	for rows.Next() {
		d, err := scanDatabase(rows)
		if err != nil {
			return nil, err
		}
		dbs = append(dbs, d)
	}
	if dbs == nil {
		dbs = []Database{}
	}
	return dbs, rows.Err()
}

func scanDeployment(row interface{ Scan(dest ...interface{}) error }) (Deployment, error) {
	var d Deployment
	err := row.Scan(
		&d.ID, &d.ServiceID, &d.UserID, &d.Status, &d.CommitSHA, &d.CommitMsg,
		&d.ImageTag, &d.Trigger, &d.BuildLog, &d.DurationSec, &d.CreatedAt, &d.FinishedAt,
	)
	if err == nil {
		d.BuildLogs = d.BuildLog
	}
	return d, err
}

func scanDeployments(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]Deployment, error) {
	var deps []Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	if deps == nil {
		deps = []Deployment{}
	}
	return deps, rows.Err()
}

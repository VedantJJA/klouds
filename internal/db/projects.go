package db

import (
	"context"
)

// ============================================================
// Project Queries
// ============================================================

type CreateProjectParams struct {
	UserID string
	Name   string
	Slug   string
}

func (q *Queries) CreateProject(ctx context.Context, arg CreateProjectParams) (Project, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO projects (user_id, name, slug)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, name, slug, created_at, updated_at`,
		arg.UserID, arg.Name, arg.Slug,
	)
	var p Project
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (q *Queries) GetProjectByID(ctx context.Context, id string) (Project, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, user_id, name, slug, created_at, updated_at
		 FROM projects WHERE id = $1`, id)
	var p Project
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (q *Queries) GetProjectBySlug(ctx context.Context, slug string) (Project, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, user_id, name, slug, created_at, updated_at
		 FROM projects WHERE slug = $1`, slug)
	var p Project
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (q *Queries) ListProjectsByUser(ctx context.Context, userID string) ([]Project, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, user_id, name, slug, created_at, updated_at
		 FROM projects WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []Project{}
	}
	return projects, rows.Err()
}

func (q *Queries) ListAllProjects(ctx context.Context) ([]Project, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, user_id, name, slug, created_at, updated_at
		 FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Slug, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []Project{}
	}
	return projects, rows.Err()
}

func (q *Queries) DeleteProject(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

func (q *Queries) CountProjectsByUser(ctx context.Context, userID string) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM projects WHERE user_id = $1`, userID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// ============================================================
// Service Queries
// ============================================================

type CreateServiceParams struct {
	ProjectID       string
	UserID          string
	Name            string
	Slug            string
	Type            string
	BuildMethod     string
	RepoURL         *string
	Branch          *string
	RootDirectory   string
	DockerfilePath  *string
	BuildCommand    *string
	StartCommand    *string
	Port            int32
	HealthCheckPath *string
	AutoDeploy      bool
	Subdomain       string
	CpuLimit        int32
	MemoryLimit     int64
}

func (q *Queries) CreateService(ctx context.Context, arg CreateServiceParams) (Service, error) {
	rootDir := arg.RootDirectory
	if rootDir == "" {
		rootDir = "."
	}
	row := q.db.QueryRow(ctx,
		`INSERT INTO services (
			project_id, user_id, name, slug, type, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, subdomain, cpu_limit, memory_limit
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at`,
		arg.ProjectID, arg.UserID, arg.Name, arg.Slug, arg.Type, arg.BuildMethod,
		arg.RepoURL, arg.Branch, rootDir, arg.DockerfilePath, arg.BuildCommand, arg.StartCommand,
		arg.Port, arg.HealthCheckPath, arg.AutoDeploy, arg.Subdomain, arg.CpuLimit, arg.MemoryLimit,
	)
	return scanService(row)
}

func (q *Queries) GetServiceByID(ctx context.Context, id string) (Service, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at
		 FROM services WHERE id = $1`, id)
	return scanService(row)
}

func (q *Queries) GetServiceBySlug(ctx context.Context, slug string) (Service, error) {
	row := q.db.QueryRow(ctx,
		`SELECT id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at
		 FROM services WHERE slug = $1`, slug)
	return scanService(row)
}

func (q *Queries) ListServicesByProject(ctx context.Context, projectID string) ([]Service, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at
		 FROM services WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

func (q *Queries) ListServicesByUser(ctx context.Context, userID string) ([]Service, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at
		 FROM services WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

func (q *Queries) ListServicesByRepoURL(ctx context.Context, repoURL string) ([]Service, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at
		 FROM services WHERE repo_url = $1 ORDER BY created_at DESC`, repoURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

func (q *Queries) ListAllServices(ctx context.Context) ([]Service, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at
		 FROM services ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

func (q *Queries) UpdateServiceStatus(ctx context.Context, id string, status string) (Service, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE services SET status = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at`,
		id, status)
	return scanService(row)
}

func (q *Queries) UpdateServiceContainer(ctx context.Context, id string, containerID *string, imageTag *string, status string) (Service, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE services SET container_id = $2, image_tag = $3, status = $4, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, project_id, user_id, name, slug, type, status, build_method,
			repo_url, branch, root_directory, dockerfile_path, build_command, start_command,
			port, health_check_path, auto_deploy, container_id, image_tag,
			subdomain, cpu_limit, memory_limit, created_at, updated_at`,
		id, containerID, imageTag, status)
	return scanService(row)
}

func (q *Queries) DeleteService(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	return err
}

func (q *Queries) CountServicesByUser(ctx context.Context, userID string) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM services WHERE user_id = $1`, userID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// --- scan helpers ---

func scanService(row interface{ Scan(dest ...interface{}) error }) (Service, error) {
	var s Service
	err := row.Scan(
		&s.ID, &s.ProjectID, &s.UserID, &s.Name, &s.Slug, &s.Type, &s.Status, &s.BuildMethod,
		&s.RepoURL, &s.Branch, &s.RootDirectory, &s.DockerfilePath, &s.BuildCommand, &s.StartCommand,
		&s.Port, &s.HealthCheckPath, &s.AutoDeploy, &s.ContainerID, &s.ImageTag,
		&s.Subdomain, &s.CpuLimit, &s.MemoryLimit, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

func scanServices(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]Service, error) {
	var services []Service
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	if services == nil {
		services = []Service{}
	}
	return services, rows.Err()
}


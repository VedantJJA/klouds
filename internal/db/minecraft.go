package db

import (
	"context"
)

// ============================================================
// Minecraft Server Queries
// ============================================================

type CreateMinecraftServerParams struct {
	ProjectID        string
	UserID           string
	Name             string
	Slug             string
	ServerType       string
	Version          string
	Status           string
	ContainerID      *string
	ServerPort       int32
	RconPort         int32
	RconPassword     string
	MOTD             string
	Difficulty       string
	GameMode         string
	MaxPlayers       int32
	PVP              bool
	OnlineMode       bool
	WhitelistEnabled bool
	CpuLimit         int32
	MemoryLimit      int64
	DiskLimit        int64
	Ops              string
}

type UpdateMinecraftServerConfigParams struct {
	ID               string
	Name             string
	MOTD             string
	Difficulty       string
	GameMode         string
	MaxPlayers       int32
	PVP              bool
	OnlineMode       bool
	WhitelistEnabled bool
	ServerType       string
	Version          string
	CpuLimit         int32
	MemoryLimit      int64
	Ops              string
}

const selectMinecraftServerSQL = `SELECT id, COALESCE(project_id, ''), user_id, name, slug, server_type, version,
	status, container_id, server_port, rcon_port, rcon_password, motd, difficulty, game_mode,
	max_players, pvp, online_mode, whitelist_enabled, cpu_limit, memory_limit, disk_limit, ops,
	created_at, updated_at
	FROM minecraft_servers`

func scanMinecraftServer(row interface{ Scan(dest ...interface{}) error }) (MinecraftServer, error) {
	var s MinecraftServer
	err := row.Scan(
		&s.ID, &s.ProjectID, &s.UserID, &s.Name, &s.Slug, &s.ServerType, &s.Version,
		&s.Status, &s.ContainerID, &s.ServerPort, &s.RconPort, &s.RconPassword, &s.MOTD, &s.Difficulty, &s.GameMode,
		&s.MaxPlayers, &s.PVP, &s.OnlineMode, &s.WhitelistEnabled, &s.CpuLimit, &s.MemoryLimit, &s.DiskLimit, &s.Ops,
		&s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

func scanMinecraftServers(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]MinecraftServer, error) {
	var list []MinecraftServer
	for rows.Next() {
		s, err := scanMinecraftServer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = []MinecraftServer{}
	}
	return list, rows.Err()
}

func (q *Queries) CreateMinecraftServer(ctx context.Context, arg CreateMinecraftServerParams) (MinecraftServer, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO minecraft_servers (
			project_id, user_id, name, slug, server_type, version,
			status, container_id, server_port, rcon_port, rcon_password,
			motd, difficulty, game_mode, max_players, pvp, online_mode,
			whitelist_enabled, cpu_limit, memory_limit, disk_limit, ops
		) VALUES (
			NULLIF($1, ''), $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22
		)
		RETURNING id, COALESCE(project_id, ''), user_id, name, slug, server_type, version,
			status, container_id, server_port, rcon_port, rcon_password, motd, difficulty, game_mode,
			max_players, pvp, online_mode, whitelist_enabled, cpu_limit, memory_limit, disk_limit, ops,
			created_at, updated_at`,
		arg.ProjectID, arg.UserID, arg.Name, arg.Slug, arg.ServerType, arg.Version,
		arg.Status, arg.ContainerID, arg.ServerPort, arg.RconPort, arg.RconPassword,
		arg.MOTD, arg.Difficulty, arg.GameMode, arg.MaxPlayers, arg.PVP, arg.OnlineMode,
		arg.WhitelistEnabled, arg.CpuLimit, arg.MemoryLimit, arg.DiskLimit, arg.Ops,
	)
	return scanMinecraftServer(row)
}

func (q *Queries) GetMinecraftServerByID(ctx context.Context, id string) (MinecraftServer, error) {
	row := q.db.QueryRow(ctx, selectMinecraftServerSQL+` WHERE id = $1`, id)
	return scanMinecraftServer(row)
}

func (q *Queries) GetMinecraftServerBySlug(ctx context.Context, slug string) (MinecraftServer, error) {
	row := q.db.QueryRow(ctx, selectMinecraftServerSQL+` WHERE slug = $1`, slug)
	return scanMinecraftServer(row)
}

func (q *Queries) ListMinecraftServersByProject(ctx context.Context, projectID string) ([]MinecraftServer, error) {
	rows, err := q.db.Query(ctx, selectMinecraftServerSQL+` WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMinecraftServers(rows)
}

func (q *Queries) ListMinecraftServersByUser(ctx context.Context, userID string) ([]MinecraftServer, error) {
	rows, err := q.db.Query(ctx, selectMinecraftServerSQL+` WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMinecraftServers(rows)
}

func (q *Queries) ListAllMinecraftServers(ctx context.Context) ([]MinecraftServer, error) {
	rows, err := q.db.Query(ctx, selectMinecraftServerSQL+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMinecraftServers(rows)
}

func (q *Queries) UpdateMinecraftServerStatus(ctx context.Context, id string, status string) (MinecraftServer, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE minecraft_servers SET status = $2, updated_at = NOW() WHERE id = $1
		 RETURNING id, COALESCE(project_id, ''), user_id, name, slug, server_type, version,
			status, container_id, server_port, rcon_port, rcon_password, motd, difficulty, game_mode,
			max_players, pvp, online_mode, whitelist_enabled, cpu_limit, memory_limit, disk_limit, ops,
			created_at, updated_at`,
		id, status)
	return scanMinecraftServer(row)
}

func (q *Queries) UpdateMinecraftServerContainer(ctx context.Context, id string, containerID *string, status string) (MinecraftServer, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE minecraft_servers SET container_id = $2, status = $3, updated_at = NOW() WHERE id = $1
		 RETURNING id, COALESCE(project_id, ''), user_id, name, slug, server_type, version,
			status, container_id, server_port, rcon_port, rcon_password, motd, difficulty, game_mode,
			max_players, pvp, online_mode, whitelist_enabled, cpu_limit, memory_limit, disk_limit, ops,
			created_at, updated_at`,
		id, containerID, status)
	return scanMinecraftServer(row)
}

func (q *Queries) UpdateMinecraftServerConfig(ctx context.Context, arg UpdateMinecraftServerConfigParams) (MinecraftServer, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE minecraft_servers SET
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
		RETURNING id, COALESCE(project_id, ''), user_id, name, slug, server_type, version,
			status, container_id, server_port, rcon_port, rcon_password, motd, difficulty, game_mode,
			max_players, pvp, online_mode, whitelist_enabled, cpu_limit, memory_limit, disk_limit, ops,
			created_at, updated_at`,
		arg.ID, arg.Name, arg.MOTD, arg.Difficulty, arg.GameMode, arg.MaxPlayers,
		arg.PVP, arg.OnlineMode, arg.WhitelistEnabled, arg.ServerType, arg.Version,
		arg.CpuLimit, arg.MemoryLimit, arg.Ops,
	)
	return scanMinecraftServer(row)
}

func (q *Queries) DeleteMinecraftServer(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM minecraft_servers WHERE id = $1`, id)
	return err
}

func (q *Queries) CountMinecraftServersByUser(ctx context.Context, userID string) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM minecraft_servers WHERE user_id = $1`, userID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

func (q *Queries) GetAllocatedMinecraftPorts(ctx context.Context) ([]int32, error) {
	rows, err := q.db.Query(ctx, `SELECT server_port FROM minecraft_servers ORDER BY server_port ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ports []int32
	for rows.Next() {
		var p int32
		if err := rows.Scan(&p); err == nil {
			ports = append(ports, p)
		}
	}
	return ports, rows.Err()
}

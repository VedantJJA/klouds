package db

import (
	"context"
)

// ============================================================
// EnvVar Queries
// ============================================================

type CreateEnvVarParams struct {
	ServiceID      string
	Key            string
	ValueEncrypted string
	IsBuildTime    bool
	IsLinked       bool
}

func (q *Queries) CreateEnvVar(ctx context.Context, arg CreateEnvVarParams) (EnvVar, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO env_vars (service_id, key, value_encrypted, is_build_time, is_linked)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (service_id, key) DO UPDATE
		 SET value_encrypted = EXCLUDED.value_encrypted,
		     is_build_time = EXCLUDED.is_build_time,
		     is_linked = EXCLUDED.is_linked
		 RETURNING id, service_id, key, value_encrypted, is_build_time, is_linked, created_at`,
		arg.ServiceID, arg.Key, arg.ValueEncrypted, arg.IsBuildTime, arg.IsLinked,
	)
	var ev EnvVar
	err := row.Scan(&ev.ID, &ev.ServiceID, &ev.Key, &ev.ValueEncrypted, &ev.IsBuildTime, &ev.IsLinked, &ev.CreatedAt)
	return ev, err
}

func (q *Queries) ListEnvVarsByService(ctx context.Context, serviceID string) ([]EnvVar, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, service_id, key, value_encrypted, is_build_time, is_linked, created_at
		 FROM env_vars WHERE service_id = $1 ORDER BY key ASC`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []EnvVar
	for rows.Next() {
		var ev EnvVar
		if err := rows.Scan(&ev.ID, &ev.ServiceID, &ev.Key, &ev.ValueEncrypted, &ev.IsBuildTime, &ev.IsLinked, &ev.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, ev)
	}
	if list == nil {
		list = []EnvVar{}
	}
	return list, rows.Err()
}

func (q *Queries) DeleteEnvVar(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM env_vars WHERE id = $1`, id)
	return err
}

// ============================================================
// RouteRule Queries
// ============================================================

type CreateRouteRuleParams struct {
	ServiceID string
	Type      string
	Source    string
	Target    string
	Status    *int32
}

func (q *Queries) CreateRouteRule(ctx context.Context, arg CreateRouteRuleParams) (RouteRule, error) {
	status := int32(301)
	if arg.Status != nil && *arg.Status != 0 {
		status = *arg.Status
	}
	row := q.db.QueryRow(ctx,
		`INSERT INTO route_rules (service_id, type, source, target, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, service_id, type, source, target, status, created_at`,
		arg.ServiceID, arg.Type, arg.Source, arg.Target, status,
	)
	var rr RouteRule
	err := row.Scan(&rr.ID, &rr.ServiceID, &rr.Type, &rr.Source, &rr.Target, &rr.Status, &rr.CreatedAt)
	return rr, err
}

func (q *Queries) ListRouteRulesByService(ctx context.Context, serviceID string) ([]RouteRule, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, service_id, type, source, target, status, created_at
		 FROM route_rules WHERE service_id = $1 ORDER BY created_at ASC`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RouteRule
	for rows.Next() {
		var rr RouteRule
		if err := rows.Scan(&rr.ID, &rr.ServiceID, &rr.Type, &rr.Source, &rr.Target, &rr.Status, &rr.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, rr)
	}
	if list == nil {
		list = []RouteRule{}
	}
	return list, rows.Err()
}

func (q *Queries) DeleteRouteRule(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM route_rules WHERE id = $1`, id)
	return err
}

func (q *Queries) DeleteRouteRulesByService(ctx context.Context, serviceID string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM route_rules WHERE service_id = $1`, serviceID)
	return err
}

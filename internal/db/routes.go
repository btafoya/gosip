package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/btafoya/gosip/internal/models"
)

var ErrRouteNotFound = errors.New("route not found")

// RouteRepository handles database operations for call routing rules
type RouteRepository struct {
	db *sql.DB
}

// NewRouteRepository creates a new RouteRepository
func NewRouteRepository(db *sql.DB) *RouteRepository {
	return &RouteRepository{db: db}
}

// Create inserts a new route
func (r *RouteRepository) Create(ctx context.Context, route *models.Route) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO routes (did_id, priority, name, condition_type, condition_data, action_type, action_data, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, route.DIDID, route.Priority, route.Name, route.ConditionType, route.ConditionData, route.ActionType, route.ActionData, route.Enabled)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	route.ID = id
	return nil
}

// GetByID retrieves a route by ID
func (r *RouteRepository) GetByID(ctx context.Context, id int64) (*models.Route, error) {
	route := &models.Route{}
	var didID sql.NullInt64
	var conditionData, actionData []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, did_id, priority, name, condition_type, condition_data, action_type, action_data, enabled
		FROM routes WHERE id = ?
	`, id).Scan(&route.ID, &didID, &route.Priority, &route.Name, &route.ConditionType, &conditionData, &route.ActionType, &actionData, &route.Enabled)
	if err == sql.ErrNoRows {
		return nil, ErrRouteNotFound
	}
	if err != nil {
		return nil, err
	}
	if didID.Valid {
		route.DIDID = &didID.Int64
	}
	route.ConditionData = conditionData
	route.ActionData = actionData
	return route, nil
}

// Update updates an existing route
func (r *RouteRepository) Update(ctx context.Context, route *models.Route) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE routes SET did_id = ?, priority = ?, name = ?, condition_type = ?,
		condition_data = ?, action_type = ?, action_data = ?, enabled = ?
		WHERE id = ?
	`, route.DIDID, route.Priority, route.Name, route.ConditionType, route.ConditionData, route.ActionType, route.ActionData, route.Enabled, route.ID)
	return err
}

// Delete removes a route
func (r *RouteRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM routes WHERE id = ?`, id)
	return err
}

// GetByDID returns all routes for a specific DID, ordered by priority
func (r *RouteRepository) GetByDID(ctx context.Context, didID int64) ([]*models.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, did_id, priority, name, condition_type, condition_data, action_type, action_data, enabled
		FROM routes WHERE did_id = ? ORDER BY priority ASC
	`, didID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*models.Route
	for rows.Next() {
		route := &models.Route{}
		var nullDIDID sql.NullInt64
		var conditionData, actionData []byte
		if err := rows.Scan(&route.ID, &nullDIDID, &route.Priority, &route.Name, &route.ConditionType, &conditionData, &route.ActionType, &actionData, &route.Enabled); err != nil {
			return nil, err
		}
		if nullDIDID.Valid {
			route.DIDID = &nullDIDID.Int64
		}
		route.ConditionData = conditionData
		route.ActionData = actionData
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

// GetEnabledByDID returns all enabled routes for a specific DID, ordered by priority
func (r *RouteRepository) GetEnabledByDID(ctx context.Context, didID int64) ([]*models.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, did_id, priority, name, condition_type, condition_data, action_type, action_data, enabled
		FROM routes WHERE did_id = ? AND enabled = 1 ORDER BY priority ASC
	`, didID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*models.Route
	for rows.Next() {
		route := &models.Route{}
		var nullDIDID sql.NullInt64
		var conditionData, actionData []byte
		if err := rows.Scan(&route.ID, &nullDIDID, &route.Priority, &route.Name, &route.ConditionType, &conditionData, &route.ActionType, &actionData, &route.Enabled); err != nil {
			return nil, err
		}
		if nullDIDID.Valid {
			route.DIDID = &nullDIDID.Int64
		}
		route.ConditionData = conditionData
		route.ActionData = actionData
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

// List returns all routes
func (r *RouteRepository) List(ctx context.Context) ([]*models.Route, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, did_id, priority, name, condition_type, condition_data, action_type, action_data, enabled
		FROM routes ORDER BY did_id, priority ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*models.Route
	for rows.Next() {
		route := &models.Route{}
		var nullDIDID sql.NullInt64
		var conditionData, actionData []byte
		if err := rows.Scan(&route.ID, &nullDIDID, &route.Priority, &route.Name, &route.ConditionType, &conditionData, &route.ActionType, &actionData, &route.Enabled); err != nil {
			return nil, err
		}
		if nullDIDID.Valid {
			route.DIDID = &nullDIDID.Int64
		}
		route.ConditionData = conditionData
		route.ActionData = actionData
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

// UpdatePriorities updates the priority of multiple routes in a single transaction
func (r *RouteRepository) UpdatePriorities(ctx context.Context, priorities map[int64]int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `UPDATE routes SET priority = ? WHERE id = ?`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for id, priority := range priorities {
		if _, err := stmt.ExecContext(ctx, priority, id); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

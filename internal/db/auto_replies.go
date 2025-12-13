package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/btafoya/gosip/internal/models"
)

var ErrAutoReplyNotFound = errors.New("auto reply not found")

// AutoReplyRepository handles database operations for auto-reply rules
type AutoReplyRepository struct {
	db *sql.DB
}

// NewAutoReplyRepository creates a new AutoReplyRepository
func NewAutoReplyRepository(db *sql.DB) *AutoReplyRepository {
	return &AutoReplyRepository{db: db}
}

// Create inserts a new auto-reply rule
func (r *AutoReplyRepository) Create(ctx context.Context, ar *models.AutoReply) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO auto_replies (did_id, trigger_type, trigger_data, reply_text, enabled)
		VALUES (?, ?, ?, ?, ?)
	`, ar.DIDID, ar.TriggerType, ar.TriggerData, ar.ReplyText, ar.Enabled)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	ar.ID = id
	return nil
}

// GetByID retrieves an auto-reply rule by ID
func (r *AutoReplyRepository) GetByID(ctx context.Context, id int64) (*models.AutoReply, error) {
	ar := &models.AutoReply{}
	var didID sql.NullInt64
	var triggerData []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, did_id, trigger_type, trigger_data, reply_text, enabled
		FROM auto_replies WHERE id = ?
	`, id).Scan(&ar.ID, &didID, &ar.TriggerType, &triggerData, &ar.ReplyText, &ar.Enabled)
	if err == sql.ErrNoRows {
		return nil, ErrAutoReplyNotFound
	}
	if err != nil {
		return nil, err
	}
	if didID.Valid {
		ar.DIDID = &didID.Int64
	}
	ar.TriggerData = triggerData
	return ar, nil
}

// Update updates an existing auto-reply rule
func (r *AutoReplyRepository) Update(ctx context.Context, ar *models.AutoReply) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE auto_replies SET did_id = ?, trigger_type = ?, trigger_data = ?,
		reply_text = ?, enabled = ?
		WHERE id = ?
	`, ar.DIDID, ar.TriggerType, ar.TriggerData, ar.ReplyText, ar.Enabled, ar.ID)
	return err
}

// Delete removes an auto-reply rule
func (r *AutoReplyRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auto_replies WHERE id = ?`, id)
	return err
}

// List returns all auto-reply rules
func (r *AutoReplyRepository) List(ctx context.Context) ([]*models.AutoReply, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, did_id, trigger_type, trigger_data, reply_text, enabled
		FROM auto_replies ORDER BY did_id, trigger_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ars []*models.AutoReply
	for rows.Next() {
		ar := &models.AutoReply{}
		var didID sql.NullInt64
		var triggerData []byte
		if err := rows.Scan(&ar.ID, &didID, &ar.TriggerType, &triggerData, &ar.ReplyText, &ar.Enabled); err != nil {
			return nil, err
		}
		if didID.Valid {
			ar.DIDID = &didID.Int64
		}
		ar.TriggerData = triggerData
		ars = append(ars, ar)
	}
	return ars, rows.Err()
}

// ListByDID returns all auto-reply rules for a specific DID
func (r *AutoReplyRepository) ListByDID(ctx context.Context, didID int64) ([]*models.AutoReply, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, did_id, trigger_type, trigger_data, reply_text, enabled
		FROM auto_replies WHERE did_id = ? ORDER BY trigger_type
	`, didID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ars []*models.AutoReply
	for rows.Next() {
		ar := &models.AutoReply{}
		var nullDIDID sql.NullInt64
		var triggerData []byte
		if err := rows.Scan(&ar.ID, &nullDIDID, &ar.TriggerType, &triggerData, &ar.ReplyText, &ar.Enabled); err != nil {
			return nil, err
		}
		if nullDIDID.Valid {
			ar.DIDID = &nullDIDID.Int64
		}
		ar.TriggerData = triggerData
		ars = append(ars, ar)
	}
	return ars, rows.Err()
}

// ListEnabled returns all enabled auto-reply rules
func (r *AutoReplyRepository) ListEnabled(ctx context.Context) ([]*models.AutoReply, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, did_id, trigger_type, trigger_data, reply_text, enabled
		FROM auto_replies WHERE enabled = 1 ORDER BY did_id, trigger_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ars []*models.AutoReply
	for rows.Next() {
		ar := &models.AutoReply{}
		var didID sql.NullInt64
		var triggerData []byte
		if err := rows.Scan(&ar.ID, &didID, &ar.TriggerType, &triggerData, &ar.ReplyText, &ar.Enabled); err != nil {
			return nil, err
		}
		if didID.Valid {
			ar.DIDID = &didID.Int64
		}
		ar.TriggerData = triggerData
		ars = append(ars, ar)
	}
	return ars, rows.Err()
}

// ListEnabledByDID returns enabled auto-reply rules for a specific DID
func (r *AutoReplyRepository) ListEnabledByDID(ctx context.Context, didID int64) ([]*models.AutoReply, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, did_id, trigger_type, trigger_data, reply_text, enabled
		FROM auto_replies WHERE did_id = ? AND enabled = 1 ORDER BY trigger_type
	`, didID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ars []*models.AutoReply
	for rows.Next() {
		ar := &models.AutoReply{}
		var nullDIDID sql.NullInt64
		var triggerData []byte
		if err := rows.Scan(&ar.ID, &nullDIDID, &ar.TriggerType, &triggerData, &ar.ReplyText, &ar.Enabled); err != nil {
			return nil, err
		}
		if nullDIDID.Valid {
			ar.DIDID = &nullDIDID.Int64
		}
		ar.TriggerData = triggerData
		ars = append(ars, ar)
	}
	return ars, rows.Err()
}

// GetByTriggerType returns an enabled auto-reply rule by DID and trigger type
func (r *AutoReplyRepository) GetByTriggerType(ctx context.Context, didID int64, triggerType string) (*models.AutoReply, error) {
	ar := &models.AutoReply{}
	var nullDIDID sql.NullInt64
	var triggerData []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, did_id, trigger_type, trigger_data, reply_text, enabled
		FROM auto_replies WHERE did_id = ? AND trigger_type = ? AND enabled = 1
	`, didID, triggerType).Scan(&ar.ID, &nullDIDID, &ar.TriggerType, &triggerData, &ar.ReplyText, &ar.Enabled)
	if err == sql.ErrNoRows {
		return nil, ErrAutoReplyNotFound
	}
	if err != nil {
		return nil, err
	}
	if nullDIDID.Valid {
		ar.DIDID = &nullDIDID.Int64
	}
	ar.TriggerData = triggerData
	return ar, nil
}

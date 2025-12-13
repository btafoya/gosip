package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var ErrVoicemailNotFound = errors.New("voicemail not found")

// VoicemailRepository handles database operations for voicemails
type VoicemailRepository struct {
	db *sql.DB
}

// NewVoicemailRepository creates a new VoicemailRepository
func NewVoicemailRepository(db *sql.DB) *VoicemailRepository {
	return &VoicemailRepository{db: db}
}

// Create inserts a new voicemail
func (r *VoicemailRepository) Create(ctx context.Context, vm *models.Voicemail) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO voicemails (cdr_id, user_id, from_number, audio_url, transcript, duration, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, vm.CDRID, vm.UserID, vm.FromNumber, vm.AudioURL, vm.Transcript, vm.Duration, vm.IsRead, time.Now())
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	vm.ID = id
	return nil
}

// GetByID retrieves a voicemail by ID
func (r *VoicemailRepository) GetByID(ctx context.Context, id int64) (*models.Voicemail, error) {
	vm := &models.Voicemail{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, cdr_id, user_id, from_number, audio_url, transcript, duration, is_read, created_at
		FROM voicemails WHERE id = ?
	`, id).Scan(&vm.ID, &vm.CDRID, &vm.UserID, &vm.FromNumber, &vm.AudioURL, &vm.Transcript, &vm.Duration, &vm.IsRead, &vm.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrVoicemailNotFound
	}
	if err != nil {
		return nil, err
	}
	return vm, nil
}

// Update updates an existing voicemail
func (r *VoicemailRepository) Update(ctx context.Context, vm *models.Voicemail) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE voicemails SET cdr_id = ?, user_id = ?, from_number = ?, audio_url = ?,
		transcript = ?, duration = ?, is_read = ?
		WHERE id = ?
	`, vm.CDRID, vm.UserID, vm.FromNumber, vm.AudioURL, vm.Transcript, vm.Duration, vm.IsRead, vm.ID)
	return err
}

// Delete removes a voicemail
func (r *VoicemailRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM voicemails WHERE id = ?`, id)
	return err
}

// MarkAsRead marks a voicemail as read
func (r *VoicemailRepository) MarkAsRead(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE voicemails SET is_read = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrVoicemailNotFound
	}
	return nil
}

// MarkAsUnread marks a voicemail as unread
func (r *VoicemailRepository) MarkAsUnread(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE voicemails SET is_read = 0 WHERE id = ?`, id)
	return err
}

// List returns voicemails with pagination
func (r *VoicemailRepository) List(ctx context.Context, limit, offset int) ([]*models.Voicemail, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, cdr_id, user_id, from_number, audio_url, transcript, duration, is_read, created_at
		FROM voicemails ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vms []*models.Voicemail
	for rows.Next() {
		vm := &models.Voicemail{}
		if err := rows.Scan(&vm.ID, &vm.CDRID, &vm.UserID, &vm.FromNumber, &vm.AudioURL, &vm.Transcript, &vm.Duration, &vm.IsRead, &vm.CreatedAt); err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

// ListByUser returns voicemails for a specific user
func (r *VoicemailRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*models.Voicemail, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, cdr_id, user_id, from_number, audio_url, transcript, duration, is_read, created_at
		FROM voicemails WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vms []*models.Voicemail
	for rows.Next() {
		vm := &models.Voicemail{}
		if err := rows.Scan(&vm.ID, &vm.CDRID, &vm.UserID, &vm.FromNumber, &vm.AudioURL, &vm.Transcript, &vm.Duration, &vm.IsRead, &vm.CreatedAt); err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

// ListUnread returns unread voicemails
func (r *VoicemailRepository) ListUnread(ctx context.Context, userID *int64) ([]*models.Voicemail, error) {
	query := `
		SELECT id, cdr_id, user_id, from_number, audio_url, transcript, duration, is_read, created_at
		FROM voicemails WHERE is_read = 0
	`
	args := []interface{}{}

	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vms []*models.Voicemail
	for rows.Next() {
		vm := &models.Voicemail{}
		if err := rows.Scan(&vm.ID, &vm.CDRID, &vm.UserID, &vm.FromNumber, &vm.AudioURL, &vm.Transcript, &vm.Duration, &vm.IsRead, &vm.CreatedAt); err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

// CountUnread returns the count of unread voicemails
func (r *VoicemailRepository) CountUnread(ctx context.Context, userID *int64) (int, error) {
	query := `SELECT COUNT(*) FROM voicemails WHERE is_read = 0`
	args := []interface{}{}

	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// Count returns the total number of voicemails
func (r *VoicemailRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM voicemails`).Scan(&count)
	return count, err
}

// CountByUser returns the count of voicemails for a specific user
func (r *VoicemailRepository) CountByUser(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM voicemails WHERE user_id = ?`, userID).Scan(&count)
	return count, err
}

// UpdateTranscript updates just the transcript field
func (r *VoicemailRepository) UpdateTranscript(ctx context.Context, id int64, transcript string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE voicemails SET transcript = ? WHERE id = ?`, transcript, id)
	return err
}

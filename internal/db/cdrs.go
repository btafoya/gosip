package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var ErrCDRNotFound = errors.New("CDR not found")

// CDRRepository handles database operations for Call Detail Records
type CDRRepository struct {
	db *sql.DB
}

// NewCDRRepository creates a new CDRRepository
func NewCDRRepository(db *sql.DB) *CDRRepository {
	return &CDRRepository{db: db}
}

// Create inserts a new CDR
func (r *CDRRepository) Create(ctx context.Context, cdr *models.CDR) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO cdrs (call_sid, direction, from_number, to_number, did_id, device_id, started_at, answered_at, ended_at, duration, disposition, recording_url, spam_score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, cdr.CallSID, cdr.Direction, cdr.FromNumber, cdr.ToNumber, cdr.DIDID, cdr.DeviceID, cdr.StartedAt, cdr.AnsweredAt, cdr.EndedAt, cdr.Duration, cdr.Disposition, cdr.RecordingURL, cdr.SpamScore)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	cdr.ID = id
	return nil
}

// GetByID retrieves a CDR by ID
func (r *CDRRepository) GetByID(ctx context.Context, id int64) (*models.CDR, error) {
	cdr := &models.CDR{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, call_sid, direction, from_number, to_number, did_id, device_id, started_at, answered_at, ended_at, duration, disposition, recording_url, spam_score
		FROM cdrs WHERE id = ?
	`, id).Scan(&cdr.ID, &cdr.CallSID, &cdr.Direction, &cdr.FromNumber, &cdr.ToNumber, &cdr.DIDID, &cdr.DeviceID, &cdr.StartedAt, &cdr.AnsweredAt, &cdr.EndedAt, &cdr.Duration, &cdr.Disposition, &cdr.RecordingURL, &cdr.SpamScore)
	if err == sql.ErrNoRows {
		return nil, ErrCDRNotFound
	}
	if err != nil {
		return nil, err
	}
	return cdr, nil
}

// GetByCallSID retrieves a CDR by Twilio Call SID
func (r *CDRRepository) GetByCallSID(ctx context.Context, callSID string) (*models.CDR, error) {
	cdr := &models.CDR{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, call_sid, direction, from_number, to_number, did_id, device_id, started_at, answered_at, ended_at, duration, disposition, recording_url, spam_score
		FROM cdrs WHERE call_sid = ?
	`, callSID).Scan(&cdr.ID, &cdr.CallSID, &cdr.Direction, &cdr.FromNumber, &cdr.ToNumber, &cdr.DIDID, &cdr.DeviceID, &cdr.StartedAt, &cdr.AnsweredAt, &cdr.EndedAt, &cdr.Duration, &cdr.Disposition, &cdr.RecordingURL, &cdr.SpamScore)
	if err == sql.ErrNoRows {
		return nil, ErrCDRNotFound
	}
	if err != nil {
		return nil, err
	}
	return cdr, nil
}

// Update updates an existing CDR
func (r *CDRRepository) Update(ctx context.Context, cdr *models.CDR) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE cdrs SET call_sid = ?, direction = ?, from_number = ?, to_number = ?,
		did_id = ?, device_id = ?, started_at = ?, answered_at = ?, ended_at = ?,
		duration = ?, disposition = ?, recording_url = ?, spam_score = ?
		WHERE id = ?
	`, cdr.CallSID, cdr.Direction, cdr.FromNumber, cdr.ToNumber, cdr.DIDID, cdr.DeviceID, cdr.StartedAt, cdr.AnsweredAt, cdr.EndedAt, cdr.Duration, cdr.Disposition, cdr.RecordingURL, cdr.SpamScore, cdr.ID)
	return err
}

// Delete removes a CDR
func (r *CDRRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM cdrs WHERE id = ?`, id)
	return err
}

// CDRFilter holds filter options for listing CDRs
type CDRFilter struct {
	Direction   string
	Disposition string
	DIDID       *int64
	DeviceID    *int64
	StartDate   *time.Time
	EndDate     *time.Time
	Limit       int
	Offset      int
}

// List returns CDRs with optional filtering and pagination
func (r *CDRRepository) List(ctx context.Context, filter CDRFilter) ([]*models.CDR, error) {
	query := `
		SELECT id, call_sid, direction, from_number, to_number, did_id, device_id, started_at, answered_at, ended_at, duration, disposition, recording_url, spam_score
		FROM cdrs WHERE 1=1
	`
	args := []interface{}{}

	if filter.Direction != "" {
		query += " AND direction = ?"
		args = append(args, filter.Direction)
	}
	if filter.Disposition != "" {
		query += " AND disposition = ?"
		args = append(args, filter.Disposition)
	}
	if filter.DIDID != nil {
		query += " AND did_id = ?"
		args = append(args, *filter.DIDID)
	}
	if filter.DeviceID != nil {
		query += " AND device_id = ?"
		args = append(args, *filter.DeviceID)
	}
	if filter.StartDate != nil {
		query += " AND started_at >= ?"
		args = append(args, *filter.StartDate)
	}
	if filter.EndDate != nil {
		query += " AND started_at <= ?"
		args = append(args, *filter.EndDate)
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cdrs []*models.CDR
	for rows.Next() {
		cdr := &models.CDR{}
		if err := rows.Scan(&cdr.ID, &cdr.CallSID, &cdr.Direction, &cdr.FromNumber, &cdr.ToNumber, &cdr.DIDID, &cdr.DeviceID, &cdr.StartedAt, &cdr.AnsweredAt, &cdr.EndedAt, &cdr.Duration, &cdr.Disposition, &cdr.RecordingURL, &cdr.SpamScore); err != nil {
			return nil, err
		}
		cdrs = append(cdrs, cdr)
	}
	return cdrs, rows.Err()
}

// Count returns the total count with optional filtering
func (r *CDRRepository) Count(ctx context.Context, filter CDRFilter) (int, error) {
	query := `SELECT COUNT(*) FROM cdrs WHERE 1=1`
	args := []interface{}{}

	if filter.Direction != "" {
		query += " AND direction = ?"
		args = append(args, filter.Direction)
	}
	if filter.Disposition != "" {
		query += " AND disposition = ?"
		args = append(args, filter.Disposition)
	}
	if filter.DIDID != nil {
		query += " AND did_id = ?"
		args = append(args, *filter.DIDID)
	}
	if filter.DeviceID != nil {
		query += " AND device_id = ?"
		args = append(args, *filter.DeviceID)
	}
	if filter.StartDate != nil {
		query += " AND started_at >= ?"
		args = append(args, *filter.StartDate)
	}
	if filter.EndDate != nil {
		query += " AND started_at <= ?"
		args = append(args, *filter.EndDate)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// GetRecent returns the most recent CDRs
func (r *CDRRepository) GetRecent(ctx context.Context, limit int) ([]*models.CDR, error) {
	return r.List(ctx, CDRFilter{Limit: limit})
}

// GetStatsByDisposition returns counts grouped by disposition
func (r *CDRRepository) GetStatsByDisposition(ctx context.Context, startDate, endDate time.Time) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT disposition, COUNT(*) as count
		FROM cdrs WHERE started_at >= ? AND started_at <= ?
		GROUP BY disposition
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var disposition string
		var count int
		if err := rows.Scan(&disposition, &count); err != nil {
			return nil, err
		}
		stats[disposition] = count
	}
	return stats, rows.Err()
}

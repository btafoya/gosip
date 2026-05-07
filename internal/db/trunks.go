package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/btafoya/gosip/internal/models"
)

var (
	ErrTrunkNotFound      = errors.New("trunk not found")
	ErrTrunkAlreadyExists = errors.New("trunk already exists")
)

// TrunkRepository handles database operations for SIP trunks
type TrunkRepository struct {
	db *sql.DB
}

// NewTrunkRepository creates a new TrunkRepository
func NewTrunkRepository(db *sql.DB) *TrunkRepository {
	return &TrunkRepository{db: db}
}

// Create inserts a new trunk
func (r *TrunkRepository) Create(ctx context.Context, trunk *models.Trunk) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO trunks (twilio_sid, friendly_name, domain_name, secure, transfer_mode, cnam_lookup_enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`, trunk.TwilioSID, trunk.FriendlyName, trunk.DomainName, trunk.Secure, trunk.TransferMode, trunk.CnamLookupEnabled)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	trunk.ID = id
	return nil
}

// GetByID retrieves a trunk by ID
func (r *TrunkRepository) GetByID(ctx context.Context, id int64) (*models.Trunk, error) {
	trunk := &models.Trunk{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, twilio_sid, friendly_name, domain_name, secure, transfer_mode, cnam_lookup_enabled, created_at, updated_at
		FROM trunks WHERE id = ?
	`, id).Scan(
		&trunk.ID, &trunk.TwilioSID, &trunk.FriendlyName, &trunk.DomainName,
		&trunk.Secure, &trunk.TransferMode, &trunk.CnamLookupEnabled,
		&trunk.CreatedAt, &trunk.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTrunkNotFound
	}
	if err != nil {
		return nil, err
	}
	return trunk, nil
}

// GetByTwilioSID retrieves a trunk by Twilio SID
func (r *TrunkRepository) GetByTwilioSID(ctx context.Context, twilioSID string) (*models.Trunk, error) {
	trunk := &models.Trunk{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, twilio_sid, friendly_name, domain_name, secure, transfer_mode, cnam_lookup_enabled, created_at, updated_at
		FROM trunks WHERE twilio_sid = ?
	`, twilioSID).Scan(
		&trunk.ID, &trunk.TwilioSID, &trunk.FriendlyName, &trunk.DomainName,
		&trunk.Secure, &trunk.TransferMode, &trunk.CnamLookupEnabled,
		&trunk.CreatedAt, &trunk.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTrunkNotFound
	}
	if err != nil {
		return nil, err
	}
	return trunk, nil
}

// Update updates an existing trunk
func (r *TrunkRepository) Update(ctx context.Context, trunk *models.Trunk) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE trunks SET twilio_sid = ?, friendly_name = ?, domain_name = ?, secure = ?, transfer_mode = ?, cnam_lookup_enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, trunk.TwilioSID, trunk.FriendlyName, trunk.DomainName, trunk.Secure, trunk.TransferMode, trunk.CnamLookupEnabled, trunk.ID)
	return err
}

// Delete removes a trunk
func (r *TrunkRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM trunks WHERE id = ?`, id)
	return err
}

// List returns all trunks
func (r *TrunkRepository) List(ctx context.Context) ([]*models.Trunk, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, twilio_sid, friendly_name, domain_name, secure, transfer_mode, cnam_lookup_enabled, created_at, updated_at
		FROM trunks ORDER BY friendly_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trunks []*models.Trunk
	for rows.Next() {
		trunk := &models.Trunk{}
		if err := rows.Scan(
			&trunk.ID, &trunk.TwilioSID, &trunk.FriendlyName, &trunk.DomainName,
			&trunk.Secure, &trunk.TransferMode, &trunk.CnamLookupEnabled,
			&trunk.CreatedAt, &trunk.UpdatedAt,
		); err != nil {
			return nil, err
		}
		trunks = append(trunks, trunk)
	}
	return trunks, rows.Err()
}

// ListByDID returns the trunk assigned to a DID
func (r *TrunkRepository) ListByDID(ctx context.Context, didID int64) (*models.Trunk, error) {
	trunk := &models.Trunk{}
	err := r.db.QueryRowContext(ctx, `
		SELECT t.id, t.twilio_sid, t.friendly_name, t.domain_name, t.secure, t.transfer_mode, t.cnam_lookup_enabled, t.created_at, t.updated_at
		FROM trunks t
		JOIN dids d ON d.trunk_id = t.id
		WHERE d.id = ?
	`, didID).Scan(
		&trunk.ID, &trunk.TwilioSID, &trunk.FriendlyName, &trunk.DomainName,
		&trunk.Secure, &trunk.TransferMode, &trunk.CnamLookupEnabled,
		&trunk.CreatedAt, &trunk.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTrunkNotFound
	}
	if err != nil {
		return nil, err
	}
	return trunk, nil
}

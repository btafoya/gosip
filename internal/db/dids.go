package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/btafoya/gosip/internal/models"
)

var (
	ErrDIDNotFound      = errors.New("DID not found")
	ErrDIDAlreadyExists = errors.New("DID already exists")
)

// DIDRepository handles database operations for phone numbers (DIDs)
type DIDRepository struct {
	db *sql.DB
}

// NewDIDRepository creates a new DIDRepository
func NewDIDRepository(db *sql.DB) *DIDRepository {
	return &DIDRepository{db: db}
}

// Create inserts a new DID
func (r *DIDRepository) Create(ctx context.Context, did *models.DID) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO dids (number, twilio_sid, name, sms_enabled, voice_enabled)
		VALUES (?, ?, ?, ?, ?)
	`, did.Number, did.TwilioSID, did.Name, did.SMSEnabled, did.VoiceEnabled)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	did.ID = id
	return nil
}

// GetByID retrieves a DID by ID
func (r *DIDRepository) GetByID(ctx context.Context, id int64) (*models.DID, error) {
	did := &models.DID{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, number, twilio_sid, name, sms_enabled, voice_enabled
		FROM dids WHERE id = ?
	`, id).Scan(&did.ID, &did.Number, &did.TwilioSID, &did.Name, &did.SMSEnabled, &did.VoiceEnabled)
	if err == sql.ErrNoRows {
		return nil, ErrDIDNotFound
	}
	if err != nil {
		return nil, err
	}
	return did, nil
}

// GetByNumber retrieves a DID by phone number
func (r *DIDRepository) GetByNumber(ctx context.Context, number string) (*models.DID, error) {
	did := &models.DID{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, number, twilio_sid, name, sms_enabled, voice_enabled
		FROM dids WHERE number = ?
	`, number).Scan(&did.ID, &did.Number, &did.TwilioSID, &did.Name, &did.SMSEnabled, &did.VoiceEnabled)
	if err == sql.ErrNoRows {
		return nil, ErrDIDNotFound
	}
	if err != nil {
		return nil, err
	}
	return did, nil
}

// Update updates an existing DID
func (r *DIDRepository) Update(ctx context.Context, did *models.DID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE dids SET number = ?, twilio_sid = ?, name = ?, sms_enabled = ?, voice_enabled = ?
		WHERE id = ?
	`, did.Number, did.TwilioSID, did.Name, did.SMSEnabled, did.VoiceEnabled, did.ID)
	return err
}

// Delete removes a DID
func (r *DIDRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM dids WHERE id = ?`, id)
	return err
}

// List returns all DIDs
func (r *DIDRepository) List(ctx context.Context) ([]*models.DID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, number, twilio_sid, name, sms_enabled, voice_enabled
		FROM dids ORDER BY number ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dids []*models.DID
	for rows.Next() {
		did := &models.DID{}
		if err := rows.Scan(&did.ID, &did.Number, &did.TwilioSID, &did.Name, &did.SMSEnabled, &did.VoiceEnabled); err != nil {
			return nil, err
		}
		dids = append(dids, did)
	}
	return dids, rows.Err()
}

// ListVoiceEnabled returns all DIDs with voice enabled
func (r *DIDRepository) ListVoiceEnabled(ctx context.Context) ([]*models.DID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, number, twilio_sid, name, sms_enabled, voice_enabled
		FROM dids WHERE voice_enabled = 1 ORDER BY number ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dids []*models.DID
	for rows.Next() {
		did := &models.DID{}
		if err := rows.Scan(&did.ID, &did.Number, &did.TwilioSID, &did.Name, &did.SMSEnabled, &did.VoiceEnabled); err != nil {
			return nil, err
		}
		dids = append(dids, did)
	}
	return dids, rows.Err()
}

// ListSMSEnabled returns all DIDs with SMS enabled
func (r *DIDRepository) ListSMSEnabled(ctx context.Context) ([]*models.DID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, number, twilio_sid, name, sms_enabled, voice_enabled
		FROM dids WHERE sms_enabled = 1 ORDER BY number ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dids []*models.DID
	for rows.Next() {
		did := &models.DID{}
		if err := rows.Scan(&did.ID, &did.Number, &did.TwilioSID, &did.Name, &did.SMSEnabled, &did.VoiceEnabled); err != nil {
			return nil, err
		}
		dids = append(dids, did)
	}
	return dids, rows.Err()
}

// Count returns the total number of DIDs
func (r *DIDRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dids`).Scan(&count)
	return count, err
}

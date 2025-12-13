package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var ErrRegistrationNotFound = errors.New("registration not found")

// RegistrationRepository handles database operations for SIP registrations
type RegistrationRepository struct {
	db *sql.DB
}

// NewRegistrationRepository creates a new RegistrationRepository
func NewRegistrationRepository(db *sql.DB) *RegistrationRepository {
	return &RegistrationRepository{db: db}
}

// Create inserts a new registration
func (r *RegistrationRepository) Create(ctx context.Context, reg *models.Registration) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO registrations (device_id, contact, expires_at, user_agent, ip_address, transport, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, reg.DeviceID, reg.Contact, reg.ExpiresAt, reg.UserAgent, reg.IPAddress, reg.Transport, time.Now())
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	reg.ID = id
	return nil
}

// GetByID retrieves a registration by ID
func (r *RegistrationRepository) GetByID(ctx context.Context, id int64) (*models.Registration, error) {
	reg := &models.Registration{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, contact, expires_at, user_agent, ip_address, transport, last_seen
		FROM registrations WHERE id = ?
	`, id).Scan(&reg.ID, &reg.DeviceID, &reg.Contact, &reg.ExpiresAt, &reg.UserAgent, &reg.IPAddress, &reg.Transport, &reg.LastSeen)
	if err == sql.ErrNoRows {
		return nil, ErrRegistrationNotFound
	}
	if err != nil {
		return nil, err
	}
	return reg, nil
}

// GetByDeviceID retrieves the active registration for a device
func (r *RegistrationRepository) GetByDeviceID(ctx context.Context, deviceID int64) (*models.Registration, error) {
	reg := &models.Registration{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, contact, expires_at, user_agent, ip_address, transport, last_seen
		FROM registrations WHERE device_id = ? AND expires_at > ? ORDER BY expires_at DESC LIMIT 1
	`, deviceID, time.Now()).Scan(&reg.ID, &reg.DeviceID, &reg.Contact, &reg.ExpiresAt, &reg.UserAgent, &reg.IPAddress, &reg.Transport, &reg.LastSeen)
	if err == sql.ErrNoRows {
		return nil, ErrRegistrationNotFound
	}
	if err != nil {
		return nil, err
	}
	return reg, nil
}

// Update updates an existing registration
func (r *RegistrationRepository) Update(ctx context.Context, reg *models.Registration) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE registrations SET contact = ?, expires_at = ?, user_agent = ?,
		ip_address = ?, transport = ?, last_seen = ?
		WHERE id = ?
	`, reg.Contact, reg.ExpiresAt, reg.UserAgent, reg.IPAddress, reg.Transport, time.Now(), reg.ID)
	return err
}

// Upsert creates or updates a registration for a device
func (r *RegistrationRepository) Upsert(ctx context.Context, reg *models.Registration) error {
	// Check for existing registration
	existing, err := r.GetByDeviceID(ctx, reg.DeviceID)
	if err == nil {
		// Update existing
		reg.ID = existing.ID
		return r.Update(ctx, reg)
	}
	if err != ErrRegistrationNotFound {
		return err
	}
	// Create new
	return r.Create(ctx, reg)
}

// Delete removes a registration
func (r *RegistrationRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM registrations WHERE id = ?`, id)
	return err
}

// DeleteByDeviceID removes all registrations for a device
func (r *RegistrationRepository) DeleteByDeviceID(ctx context.Context, deviceID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM registrations WHERE device_id = ?`, deviceID)
	return err
}

// DeleteExpired removes all expired registrations
func (r *RegistrationRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM registrations WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ListActive returns all active (non-expired) registrations
func (r *RegistrationRepository) ListActive(ctx context.Context) ([]*models.Registration, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, contact, expires_at, user_agent, ip_address, transport, last_seen
		FROM registrations WHERE expires_at > ? ORDER BY last_seen DESC
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var regs []*models.Registration
	for rows.Next() {
		reg := &models.Registration{}
		if err := rows.Scan(&reg.ID, &reg.DeviceID, &reg.Contact, &reg.ExpiresAt, &reg.UserAgent, &reg.IPAddress, &reg.Transport, &reg.LastSeen); err != nil {
			return nil, err
		}
		regs = append(regs, reg)
	}
	return regs, rows.Err()
}

// CountActive returns the count of active registrations
func (r *RegistrationRepository) CountActive(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE expires_at > ?`, time.Now()).Scan(&count)
	return count, err
}

// TouchLastSeen updates the last_seen timestamp
func (r *RegistrationRepository) TouchLastSeen(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE registrations SET last_seen = ? WHERE id = ?`, time.Now(), id)
	return err
}

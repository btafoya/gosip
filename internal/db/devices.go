package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var (
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceAlreadyExists = errors.New("device already exists")
)

// DeviceRepository handles database operations for SIP devices
type DeviceRepository struct {
	db *sql.DB
}

// NewDeviceRepository creates a new DeviceRepository
func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// Create inserts a new device
func (r *DeviceRepository) Create(ctx context.Context, device *models.Device) error {
	now := time.Now()
	device.CreatedAt = now
	if device.ProvisioningStatus == "" {
		device.ProvisioningStatus = "unknown"
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO devices (user_id, name, username, password_hash, device_type, recording_enabled, created_at,
			mac_address, vendor, model, firmware_version, provisioning_status, config_template)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, device.UserID, device.Name, device.Username, device.PasswordHash, device.DeviceType, device.RecordingEnabled, now,
		device.MACAddress, device.Vendor, device.Model, device.FirmwareVersion, device.ProvisioningStatus, device.ConfigTemplate)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	device.ID = id
	return nil
}

// GetByID retrieves a device by ID
func (r *DeviceRepository) GetByID(ctx context.Context, id int64) (*models.Device, error) {
	device := &models.Device{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
			mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
		FROM devices WHERE id = ?
	`, id).Scan(&device.ID, &device.UserID, &device.Name, &device.Username, &device.PasswordHash, &device.DeviceType, &device.RecordingEnabled, &device.CreatedAt,
		&device.MACAddress, &device.Vendor, &device.Model, &device.FirmwareVersion, &device.ProvisioningStatus, &device.LastConfigFetch, &device.LastRegistration, &device.ConfigTemplate)
	if err == sql.ErrNoRows {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	return device, nil
}

// GetByUsername retrieves a device by SIP username
func (r *DeviceRepository) GetByUsername(ctx context.Context, username string) (*models.Device, error) {
	device := &models.Device{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
			mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
		FROM devices WHERE username = ?
	`, username).Scan(&device.ID, &device.UserID, &device.Name, &device.Username, &device.PasswordHash, &device.DeviceType, &device.RecordingEnabled, &device.CreatedAt,
		&device.MACAddress, &device.Vendor, &device.Model, &device.FirmwareVersion, &device.ProvisioningStatus, &device.LastConfigFetch, &device.LastRegistration, &device.ConfigTemplate)
	if err == sql.ErrNoRows {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	return device, nil
}

// GetByMAC retrieves a device by MAC address
func (r *DeviceRepository) GetByMAC(ctx context.Context, mac string) (*models.Device, error) {
	device := &models.Device{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
			mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
		FROM devices WHERE mac_address = ?
	`, mac).Scan(&device.ID, &device.UserID, &device.Name, &device.Username, &device.PasswordHash, &device.DeviceType, &device.RecordingEnabled, &device.CreatedAt,
		&device.MACAddress, &device.Vendor, &device.Model, &device.FirmwareVersion, &device.ProvisioningStatus, &device.LastConfigFetch, &device.LastRegistration, &device.ConfigTemplate)
	if err == sql.ErrNoRows {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	return device, nil
}

// Update updates an existing device
func (r *DeviceRepository) Update(ctx context.Context, device *models.Device) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE devices SET user_id = ?, name = ?, username = ?, password_hash = ?,
		device_type = ?, recording_enabled = ?, mac_address = ?, vendor = ?, model = ?,
		firmware_version = ?, provisioning_status = ?, last_config_fetch = ?, last_registration = ?, config_template = ?
		WHERE id = ?
	`, device.UserID, device.Name, device.Username, device.PasswordHash, device.DeviceType, device.RecordingEnabled,
		device.MACAddress, device.Vendor, device.Model, device.FirmwareVersion, device.ProvisioningStatus,
		device.LastConfigFetch, device.LastRegistration, device.ConfigTemplate, device.ID)
	return err
}

// UpdateProvisioningStatus updates just the provisioning status of a device
func (r *DeviceRepository) UpdateProvisioningStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE devices SET provisioning_status = ? WHERE id = ?`, status, id)
	return err
}

// UpdateLastConfigFetch updates the last config fetch timestamp
func (r *DeviceRepository) UpdateLastConfigFetch(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE devices SET last_config_fetch = ? WHERE id = ?`, time.Now(), id)
	return err
}

// UpdateLastRegistration updates the last registration timestamp
func (r *DeviceRepository) UpdateLastRegistration(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE devices SET last_registration = ? WHERE id = ?`, time.Now(), id)
	return err
}

// Delete removes a device
func (r *DeviceRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, id)
	return err
}

// List returns all devices with pagination
func (r *DeviceRepository) List(ctx context.Context, limit, offset int) ([]*models.Device, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
			mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
		FROM devices ORDER BY name ASC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		device := &models.Device{}
		if err := rows.Scan(&device.ID, &device.UserID, &device.Name, &device.Username, &device.PasswordHash, &device.DeviceType, &device.RecordingEnabled, &device.CreatedAt,
			&device.MACAddress, &device.Vendor, &device.Model, &device.FirmwareVersion, &device.ProvisioningStatus, &device.LastConfigFetch, &device.LastRegistration, &device.ConfigTemplate); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

// ListByUser returns all devices for a specific user
func (r *DeviceRepository) ListByUser(ctx context.Context, userID int64) ([]*models.Device, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
			mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
		FROM devices WHERE user_id = ? ORDER BY name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		device := &models.Device{}
		if err := rows.Scan(&device.ID, &device.UserID, &device.Name, &device.Username, &device.PasswordHash, &device.DeviceType, &device.RecordingEnabled, &device.CreatedAt,
			&device.MACAddress, &device.Vendor, &device.Model, &device.FirmwareVersion, &device.ProvisioningStatus, &device.LastConfigFetch, &device.LastRegistration, &device.ConfigTemplate); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

// ListByProvisioningStatus returns devices with a specific provisioning status
func (r *DeviceRepository) ListByProvisioningStatus(ctx context.Context, status string) ([]*models.Device, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
			mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
		FROM devices WHERE provisioning_status = ? ORDER BY name ASC
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		device := &models.Device{}
		if err := rows.Scan(&device.ID, &device.UserID, &device.Name, &device.Username, &device.PasswordHash, &device.DeviceType, &device.RecordingEnabled, &device.CreatedAt,
			&device.MACAddress, &device.Vendor, &device.Model, &device.FirmwareVersion, &device.ProvisioningStatus, &device.LastConfigFetch, &device.LastRegistration, &device.ConfigTemplate); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

// Count returns the total number of devices
func (r *DeviceRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices`).Scan(&count)
	return count, err
}

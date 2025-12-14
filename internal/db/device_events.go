package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

// DeviceEventRepository handles database operations for device events
type DeviceEventRepository struct {
	db *sql.DB
}

// NewDeviceEventRepository creates a new DeviceEventRepository
func NewDeviceEventRepository(db *sql.DB) *DeviceEventRepository {
	return &DeviceEventRepository{db: db}
}

// Create inserts a new device event
func (r *DeviceEventRepository) Create(ctx context.Context, event *models.DeviceEvent) error {
	now := time.Now()
	event.CreatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO device_events (device_id, event_type, event_data, ip_address, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, event.DeviceID, event.EventType, event.EventData, event.IPAddress, event.UserAgent, now)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	event.ID = id
	return nil
}

// LogEvent is a helper to log an event with common data
func (r *DeviceEventRepository) LogEvent(ctx context.Context, deviceID int64, eventType string, data map[string]interface{}, ipAddress, userAgent string) error {
	var eventData json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		eventData = b
	}

	event := &models.DeviceEvent{
		DeviceID:  deviceID,
		EventType: eventType,
		EventData: eventData,
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
	}
	return r.Create(ctx, event)
}

// GetByID retrieves an event by ID
func (r *DeviceEventRepository) GetByID(ctx context.Context, id int64) (*models.DeviceEvent, error) {
	event := &models.DeviceEvent{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, event_type, event_data, ip_address, user_agent, created_at
		FROM device_events WHERE id = ?
	`, id).Scan(&event.ID, &event.DeviceID, &event.EventType, &event.EventData, &event.IPAddress, &event.UserAgent, &event.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}

// ListByDevice returns events for a device with pagination
func (r *DeviceEventRepository) ListByDevice(ctx context.Context, deviceID int64, limit, offset int) ([]*models.DeviceEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, event_type, event_data, ip_address, user_agent, created_at
		FROM device_events WHERE device_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, deviceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.DeviceEvent
	for rows.Next() {
		event := &models.DeviceEvent{}
		if err := rows.Scan(&event.ID, &event.DeviceID, &event.EventType, &event.EventData, &event.IPAddress, &event.UserAgent, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ListByDeviceAndType returns events for a device filtered by event type
func (r *DeviceEventRepository) ListByDeviceAndType(ctx context.Context, deviceID int64, eventType string, limit int) ([]*models.DeviceEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, event_type, event_data, ip_address, user_agent, created_at
		FROM device_events WHERE device_id = ? AND event_type = ? ORDER BY created_at DESC LIMIT ?
	`, deviceID, eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.DeviceEvent
	for rows.Next() {
		event := &models.DeviceEvent{}
		if err := rows.Scan(&event.ID, &event.DeviceID, &event.EventType, &event.EventData, &event.IPAddress, &event.UserAgent, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ListRecent returns recent events across all devices
func (r *DeviceEventRepository) ListRecent(ctx context.Context, limit int) ([]*models.DeviceEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, event_type, event_data, ip_address, user_agent, created_at
		FROM device_events ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.DeviceEvent
	for rows.Next() {
		event := &models.DeviceEvent{}
		if err := rows.Scan(&event.ID, &event.DeviceID, &event.EventType, &event.EventData, &event.IPAddress, &event.UserAgent, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ListByType returns recent events of a specific type
func (r *DeviceEventRepository) ListByType(ctx context.Context, eventType string, limit int) ([]*models.DeviceEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, event_type, event_data, ip_address, user_agent, created_at
		FROM device_events WHERE event_type = ? ORDER BY created_at DESC LIMIT ?
	`, eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.DeviceEvent
	for rows.Next() {
		event := &models.DeviceEvent{}
		if err := rows.Scan(&event.ID, &event.DeviceID, &event.EventType, &event.EventData, &event.IPAddress, &event.UserAgent, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// CountByDevice returns the total number of events for a device
func (r *DeviceEventRepository) CountByDevice(ctx context.Context, deviceID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_events WHERE device_id = ?`, deviceID).Scan(&count)
	return count, err
}

// GetLastEventOfType returns the most recent event of a specific type for a device
func (r *DeviceEventRepository) GetLastEventOfType(ctx context.Context, deviceID int64, eventType string) (*models.DeviceEvent, error) {
	event := &models.DeviceEvent{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, event_type, event_data, ip_address, user_agent, created_at
		FROM device_events WHERE device_id = ? AND event_type = ? ORDER BY created_at DESC LIMIT 1
	`, deviceID, eventType).Scan(&event.ID, &event.DeviceID, &event.EventType, &event.EventData, &event.IPAddress, &event.UserAgent, &event.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}

// Delete removes an event
func (r *DeviceEventRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM device_events WHERE id = ?`, id)
	return err
}

// DeleteByDevice removes all events for a device
func (r *DeviceEventRepository) DeleteByDevice(ctx context.Context, deviceID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM device_events WHERE device_id = ?`, deviceID)
	return err
}

// CleanupOld removes events older than a specified number of days
func (r *DeviceEventRepository) CleanupOld(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result, err := r.db.ExecContext(ctx, `DELETE FROM device_events WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

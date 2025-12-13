package sip

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
)

// RegistrationInfo provides registration status information
type RegistrationInfo struct {
	DeviceID   int64     `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Username   string    `json:"username"`
	Contact    string    `json:"contact"`
	IPAddress  string    `json:"ip_address"`
	Transport  string    `json:"transport"`
	UserAgent  string    `json:"user_agent"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastSeen   time.Time `json:"last_seen"`
	Online     bool      `json:"online"`
}

// Registrar manages SIP device registrations
type Registrar struct {
	db *db.DB

	// In-memory cache for fast lookups
	cache map[int64]*models.Registration
	mu    sync.RWMutex

	// Event callbacks
	onRegister   func(deviceID int64)
	onUnregister func(deviceID int64)
}

// NewRegistrar creates a new Registrar
func NewRegistrar(database *db.DB) *Registrar {
	return &Registrar{
		db:    database,
		cache: make(map[int64]*models.Registration),
	}
}

// Register creates or updates a device registration
func (r *Registrar) Register(ctx context.Context, reg *models.Registration) error {
	// Update in database
	if err := r.db.Registrations.Upsert(ctx, reg); err != nil {
		return err
	}

	// Update cache
	r.mu.Lock()
	r.cache[reg.DeviceID] = reg
	r.mu.Unlock()

	// Fire callback
	if r.onRegister != nil {
		go r.onRegister(reg.DeviceID)
	}

	slog.Debug("Device registered",
		"device_id", reg.DeviceID,
		"contact", reg.Contact,
		"expires", reg.ExpiresAt,
	)

	return nil
}

// Unregister removes a device registration
func (r *Registrar) Unregister(ctx context.Context, deviceID int64) error {
	// Remove from database
	if err := r.db.Registrations.DeleteByDeviceID(ctx, deviceID); err != nil {
		return err
	}

	// Remove from cache
	r.mu.Lock()
	delete(r.cache, deviceID)
	r.mu.Unlock()

	// Fire callback
	if r.onUnregister != nil {
		go r.onUnregister(deviceID)
	}

	slog.Debug("Device unregistered", "device_id", deviceID)

	return nil
}

// IsRegistered checks if a device is currently registered
func (r *Registrar) IsRegistered(ctx context.Context, deviceID int64) bool {
	// Check cache first
	r.mu.RLock()
	reg, exists := r.cache[deviceID]
	r.mu.RUnlock()

	if exists && time.Now().Before(reg.ExpiresAt) {
		return true
	}

	// Check database
	dbReg, err := r.db.Registrations.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return false
	}

	// Update cache
	r.mu.Lock()
	r.cache[deviceID] = dbReg
	r.mu.Unlock()

	return time.Now().Before(dbReg.ExpiresAt)
}

// GetRegistration retrieves the current registration for a device
func (r *Registrar) GetRegistration(ctx context.Context, deviceID int64) (*models.Registration, error) {
	// Check cache first
	r.mu.RLock()
	reg, exists := r.cache[deviceID]
	r.mu.RUnlock()

	if exists && time.Now().Before(reg.ExpiresAt) {
		return reg, nil
	}

	// Get from database
	dbReg, err := r.db.Registrations.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	// Update cache
	r.mu.Lock()
	r.cache[deviceID] = dbReg
	r.mu.Unlock()

	return dbReg, nil
}

// GetActiveRegistrations returns all active registrations with device info
func (r *Registrar) GetActiveRegistrations(ctx context.Context) ([]RegistrationInfo, error) {
	// Get all active registrations from database
	regs, err := r.db.Registrations.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	var result []RegistrationInfo
	now := time.Now()

	for _, reg := range regs {
		// Get device info
		device, err := r.db.Devices.GetByID(ctx, reg.DeviceID)
		if err != nil {
			slog.Warn("Failed to get device for registration", "device_id", reg.DeviceID, "error", err)
			continue
		}

		result = append(result, RegistrationInfo{
			DeviceID:   reg.DeviceID,
			DeviceName: device.Name,
			Username:   device.Username,
			Contact:    reg.Contact,
			IPAddress:  reg.IPAddress,
			Transport:  reg.Transport,
			UserAgent:  reg.UserAgent,
			ExpiresAt:  reg.ExpiresAt,
			LastSeen:   reg.LastSeen,
			Online:     now.Before(reg.ExpiresAt),
		})
	}

	return result, nil
}

// OnRegister sets a callback for when a device registers
func (r *Registrar) OnRegister(callback func(deviceID int64)) {
	r.onRegister = callback
}

// OnUnregister sets a callback for when a device unregisters
func (r *Registrar) OnUnregister(callback func(deviceID int64)) {
	r.onUnregister = callback
}

// Touch updates the last_seen timestamp for a registration
func (r *Registrar) Touch(ctx context.Context, deviceID int64) error {
	// Update cache
	r.mu.Lock()
	if reg, exists := r.cache[deviceID]; exists {
		reg.LastSeen = time.Now()
	}
	r.mu.Unlock()

	// Get registration and update in DB
	reg, err := r.db.Registrations.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	return r.db.Registrations.TouchLastSeen(ctx, reg.ID)
}

// RefreshCache reloads all active registrations into memory
func (r *Registrar) RefreshCache(ctx context.Context) error {
	regs, err := r.db.Registrations.ListActive(ctx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear and rebuild cache
	r.cache = make(map[int64]*models.Registration)
	for _, reg := range regs {
		r.cache[reg.DeviceID] = reg
	}

	slog.Debug("Refreshed registration cache", "count", len(regs))
	return nil
}

// CleanupExpired removes expired registrations from cache
func (r *Registrar) CleanupExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for deviceID, reg := range r.cache {
		if now.After(reg.ExpiresAt) {
			delete(r.cache, deviceID)
		}
	}
}

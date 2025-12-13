package sip

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

func TestNewRegistrar(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)

	if registrar == nil {
		t.Fatal("NewRegistrar should not return nil")
	}

	if registrar.db != database {
		t.Error("Registrar should have the provided database")
	}

	if registrar.cache == nil {
		t.Error("Registrar cache should be initialized")
	}

	if len(registrar.cache) != 0 {
		t.Error("Registrar cache should be empty initially")
	}
}

func TestRegistrar_Register(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	// Create a test device first
	device := createTestDevice(t, database, "alice", "passwordhash")

	// Create registration
	reg := &models.Registration{
		DeviceID:  device.ID,
		Contact:   "sip:alice@192.168.1.100:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UserAgent: "TestPhone/1.0",
		IPAddress: "192.168.1.100",
		Transport: "udp",
	}

	err := registrar.Register(ctx, reg)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify registration ID was set
	if reg.ID == 0 {
		t.Error("Registration ID should be set after Register")
	}

	// Verify it's in the cache
	registrar.mu.RLock()
	cachedReg, exists := registrar.cache[device.ID]
	registrar.mu.RUnlock()

	if !exists {
		t.Error("Registration should be in cache")
	}

	if cachedReg.Contact != reg.Contact {
		t.Errorf("Cached contact mismatch: got %s, want %s", cachedReg.Contact, reg.Contact)
	}
}

func TestRegistrar_Unregister(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	// Create and register a device
	device := createTestDevice(t, database, "alice", "passwordhash")
	reg := &models.Registration{
		DeviceID:  device.ID,
		Contact:   "sip:alice@192.168.1.100:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Transport: "udp",
	}

	if err := registrar.Register(ctx, reg); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify registered
	if !registrar.IsRegistered(ctx, device.ID) {
		t.Fatal("Device should be registered")
	}

	// Unregister
	err := registrar.Unregister(ctx, device.ID)
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	// Verify unregistered
	registrar.mu.RLock()
	_, exists := registrar.cache[device.ID]
	registrar.mu.RUnlock()

	if exists {
		t.Error("Registration should be removed from cache")
	}

	if registrar.IsRegistered(ctx, device.ID) {
		t.Error("Device should not be registered after unregister")
	}
}

func TestRegistrar_IsRegistered(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	device := createTestDevice(t, database, "alice", "passwordhash")

	t.Run("unregistered device", func(t *testing.T) {
		if registrar.IsRegistered(ctx, device.ID) {
			t.Error("New device should not be registered")
		}
	})

	t.Run("registered device", func(t *testing.T) {
		reg := &models.Registration{
			DeviceID:  device.ID,
			Contact:   "sip:alice@192.168.1.100:5060",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			Transport: "udp",
		}
		if err := registrar.Register(ctx, reg); err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		if !registrar.IsRegistered(ctx, device.ID) {
			t.Error("Registered device should show as registered")
		}
	})

	t.Run("expired registration", func(t *testing.T) {
		// Create a new device for this test case to avoid interference
		expiredDevice := createTestDevice(t, database, "expired_user", "passwordhash")

		// Create an expired registration directly in DB
		expiredReg := &models.Registration{
			DeviceID:  expiredDevice.ID,
			Contact:   "sip:expired@192.168.1.100:5060",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
			Transport: "udp",
		}
		if err := database.Registrations.Create(ctx, expiredReg); err != nil {
			t.Fatalf("Failed to create expired registration: %v", err)
		}

		// Should not be registered (expired)
		// Note: The DB query filters by expires_at > now, so expired won't be found
		if registrar.IsRegistered(ctx, expiredDevice.ID) {
			t.Error("Expired registration should not show as registered")
		}
	})
}

func TestRegistrar_GetRegistration(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	device := createTestDevice(t, database, "alice", "passwordhash")

	t.Run("non-existent device", func(t *testing.T) {
		_, err := registrar.GetRegistration(ctx, 9999)
		if err == nil {
			t.Error("Should return error for non-existent device")
		}
	})

	t.Run("registered device from cache", func(t *testing.T) {
		reg := &models.Registration{
			DeviceID:  device.ID,
			Contact:   "sip:alice@192.168.1.100:5060",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			UserAgent: "TestPhone/1.0",
			IPAddress: "192.168.1.100",
			Transport: "udp",
		}
		if err := registrar.Register(ctx, reg); err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		result, err := registrar.GetRegistration(ctx, device.ID)
		if err != nil {
			t.Fatalf("GetRegistration failed: %v", err)
		}

		if result.Contact != reg.Contact {
			t.Errorf("Contact mismatch: got %s, want %s", result.Contact, reg.Contact)
		}
		if result.UserAgent != reg.UserAgent {
			t.Errorf("UserAgent mismatch: got %s, want %s", result.UserAgent, reg.UserAgent)
		}
	})

	t.Run("registered device from DB", func(t *testing.T) {
		// Clear cache
		registrar.mu.Lock()
		delete(registrar.cache, device.ID)
		registrar.mu.Unlock()

		result, err := registrar.GetRegistration(ctx, device.ID)
		if err != nil {
			t.Fatalf("GetRegistration from DB failed: %v", err)
		}

		if result.DeviceID != device.ID {
			t.Errorf("DeviceID mismatch: got %d, want %d", result.DeviceID, device.ID)
		}

		// Should now be in cache
		registrar.mu.RLock()
		_, exists := registrar.cache[device.ID]
		registrar.mu.RUnlock()

		if !exists {
			t.Error("Registration should be cached after DB lookup")
		}
	})
}

func TestRegistrar_GetRegistrationCount(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	t.Run("empty registrar", func(t *testing.T) {
		count := registrar.GetRegistrationCount()
		if count != 0 {
			t.Errorf("Empty registrar should have 0 registrations, got %d", count)
		}
	})

	t.Run("with registrations", func(t *testing.T) {
		// Create and register multiple devices
		for i := 0; i < 3; i++ {
			device := createTestDevice(t, database, "user"+string(rune('a'+i)), "passwordhash")
			reg := &models.Registration{
				DeviceID:  device.ID,
				Contact:   "sip:user@192.168.1.100:5060",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				Transport: "udp",
			}
			if err := registrar.Register(ctx, reg); err != nil {
				t.Fatalf("Register failed: %v", err)
			}
		}

		count := registrar.GetRegistrationCount()
		if count != 3 {
			t.Errorf("Should have 3 registrations, got %d", count)
		}
	})

	t.Run("excludes expired", func(t *testing.T) {
		// Add an expired registration to cache
		registrar.mu.Lock()
		registrar.cache[999] = &models.Registration{
			DeviceID:  999,
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		}
		registrar.mu.Unlock()

		count := registrar.GetRegistrationCount()
		if count != 3 {
			t.Errorf("Should still have 3 active registrations, got %d (expired should not count)", count)
		}
	})
}

func TestRegistrar_CleanupExpired(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)

	// Add some registrations to cache
	registrar.mu.Lock()
	registrar.cache[1] = &models.Registration{
		DeviceID:  1,
		ExpiresAt: time.Now().Add(1 * time.Hour), // Active
	}
	registrar.cache[2] = &models.Registration{
		DeviceID:  2,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
	}
	registrar.cache[3] = &models.Registration{
		DeviceID:  3,
		ExpiresAt: time.Now().Add(-30 * time.Minute), // Expired
	}
	registrar.mu.Unlock()

	// Run cleanup
	registrar.CleanupExpired()

	// Verify only active registration remains
	registrar.mu.RLock()
	defer registrar.mu.RUnlock()

	if _, exists := registrar.cache[1]; !exists {
		t.Error("Active registration should remain")
	}
	if _, exists := registrar.cache[2]; exists {
		t.Error("Expired registration 2 should be removed")
	}
	if _, exists := registrar.cache[3]; exists {
		t.Error("Expired registration 3 should be removed")
	}
}

func TestRegistrar_RefreshCache(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	// Create devices and registrations in DB
	device1 := createTestDevice(t, database, "alice", "passwordhash")
	device2 := createTestDevice(t, database, "bob", "passwordhash")

	reg1 := &models.Registration{
		DeviceID:  device1.ID,
		Contact:   "sip:alice@192.168.1.100:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Transport: "udp",
	}
	reg2 := &models.Registration{
		DeviceID:  device2.ID,
		Contact:   "sip:bob@192.168.1.101:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Transport: "udp",
	}

	if err := database.Registrations.Create(ctx, reg1); err != nil {
		t.Fatalf("Failed to create reg1: %v", err)
	}
	if err := database.Registrations.Create(ctx, reg2); err != nil {
		t.Fatalf("Failed to create reg2: %v", err)
	}

	// Clear cache
	registrar.mu.Lock()
	registrar.cache = make(map[int64]*models.Registration)
	registrar.mu.Unlock()

	// Refresh cache
	if err := registrar.RefreshCache(ctx); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}

	// Verify cache has both registrations
	registrar.mu.RLock()
	defer registrar.mu.RUnlock()

	if len(registrar.cache) != 2 {
		t.Errorf("Cache should have 2 registrations, got %d", len(registrar.cache))
	}

	if _, exists := registrar.cache[device1.ID]; !exists {
		t.Error("Device 1 should be in cache")
	}
	if _, exists := registrar.cache[device2.ID]; !exists {
		t.Error("Device 2 should be in cache")
	}
}

func TestRegistrar_Callbacks(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	var registeredID, unregisteredID int64
	var wg sync.WaitGroup

	// Set callbacks
	wg.Add(2)
	registrar.OnRegister(func(deviceID int64) {
		registeredID = deviceID
		wg.Done()
	})
	registrar.OnUnregister(func(deviceID int64) {
		unregisteredID = deviceID
		wg.Done()
	})

	device := createTestDevice(t, database, "alice", "passwordhash")

	// Register should trigger callback
	reg := &models.Registration{
		DeviceID:  device.ID,
		Contact:   "sip:alice@192.168.1.100:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Transport: "udp",
	}
	if err := registrar.Register(ctx, reg); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Unregister should trigger callback
	if err := registrar.Unregister(ctx, device.ID); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	// Wait for callbacks with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Callbacks completed
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for callbacks")
	}

	if registeredID != device.ID {
		t.Errorf("OnRegister callback got wrong ID: %d, want %d", registeredID, device.ID)
	}
	if unregisteredID != device.ID {
		t.Errorf("OnUnregister callback got wrong ID: %d, want %d", unregisteredID, device.ID)
	}
}

func TestRegistrar_Touch(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	device := createTestDevice(t, database, "alice", "passwordhash")

	// Register device
	initialTime := time.Now().Add(-1 * time.Hour)
	reg := &models.Registration{
		DeviceID:  device.ID,
		Contact:   "sip:alice@192.168.1.100:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Transport: "udp",
		LastSeen:  initialTime,
	}
	if err := registrar.Register(ctx, reg); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Touch the registration
	if err := registrar.Touch(ctx, device.ID); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	// Verify last_seen was updated in cache
	registrar.mu.RLock()
	cachedReg := registrar.cache[device.ID]
	registrar.mu.RUnlock()

	if cachedReg.LastSeen.Before(initialTime) || cachedReg.LastSeen.Equal(initialTime) {
		t.Error("LastSeen should be updated after Touch")
	}
}

func TestRegistrar_GetActiveRegistrations(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	// Create devices with registrations
	device1 := createTestDevice(t, database, "alice", "passwordhash")
	device2 := createTestDevice(t, database, "bob", "passwordhash")

	reg1 := &models.Registration{
		DeviceID:  device1.ID,
		Contact:   "sip:alice@192.168.1.100:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UserAgent: "Phone/1.0",
		IPAddress: "192.168.1.100",
		Transport: "udp",
	}
	reg2 := &models.Registration{
		DeviceID:  device2.ID,
		Contact:   "sip:bob@192.168.1.101:5060",
		ExpiresAt: time.Now().Add(2 * time.Hour),
		UserAgent: "Phone/2.0",
		IPAddress: "192.168.1.101",
		Transport: "tcp",
	}

	if err := registrar.Register(ctx, reg1); err != nil {
		t.Fatalf("Register device1 failed: %v", err)
	}
	if err := registrar.Register(ctx, reg2); err != nil {
		t.Fatalf("Register device2 failed: %v", err)
	}

	// Get active registrations
	regs, err := registrar.GetActiveRegistrations(ctx)
	if err != nil {
		t.Fatalf("GetActiveRegistrations failed: %v", err)
	}

	if len(regs) != 2 {
		t.Errorf("Should have 2 active registrations, got %d", len(regs))
	}

	// Verify registration info includes device details
	for _, info := range regs {
		if info.DeviceName == "" {
			t.Error("DeviceName should be populated")
		}
		if info.Username == "" {
			t.Error("Username should be populated")
		}
		if !info.Online {
			t.Error("Active registrations should show as Online")
		}
	}
}

func TestRegistrar_ConcurrentAccess(t *testing.T) {
	database := setupTestDB(t)
	registrar := NewRegistrar(database)
	ctx := context.Background()

	// Create multiple devices
	var devices []*models.Device
	for i := 0; i < 5; i++ {
		device := createTestDevice(t, database, "user"+string(rune('a'+i)), "passwordhash")
		devices = append(devices, device)
	}

	// Concurrent registrations
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for _, device := range devices {
		wg.Add(1)
		go func(d *models.Device) {
			defer wg.Done()
			reg := &models.Registration{
				DeviceID:  d.ID,
				Contact:   "sip:user@192.168.1.100:5060",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				Transport: "udp",
			}
			if err := registrar.Register(ctx, reg); err != nil {
				errors <- err
			}
		}(device)
	}

	// Concurrent reads while registering
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			registrar.GetRegistrationCount()
			registrar.CleanupExpired()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Verify final state
	count := registrar.GetRegistrationCount()
	if count != 5 {
		t.Errorf("Should have 5 registrations after concurrent ops, got %d", count)
	}
}

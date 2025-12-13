package db

import (
	"context"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestDeviceRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	device := &models.Device{
		Name:             "Test Phone",
		Username:         "testphone",
		PasswordHash:     "hashed_password",
		DeviceType:       "grandstream",
		RecordingEnabled: true,
	}

	err := db.Devices.Create(ctx, device)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	if device.ID == 0 {
		t.Error("Expected device ID to be set after creation")
	}
}

func TestDeviceRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	device := &models.Device{
		Name:         "Test Phone",
		Username:     "testphone",
		PasswordHash: "hashed_password",
		DeviceType:   "softphone",
	}
	if err := db.Devices.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	retrieved, err := db.Devices.GetByID(ctx, device.ID)
	if err != nil {
		t.Fatalf("Failed to get device by ID: %v", err)
	}

	if retrieved.Name != device.Name {
		t.Errorf("Expected name %s, got %s", device.Name, retrieved.Name)
	}
	if retrieved.Username != device.Username {
		t.Errorf("Expected username %s, got %s", device.Username, retrieved.Username)
	}
	if retrieved.DeviceType != device.DeviceType {
		t.Errorf("Expected device type %s, got %s", device.DeviceType, retrieved.DeviceType)
	}
}

func TestDeviceRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Devices.GetByID(ctx, 9999)
	if err != ErrDeviceNotFound {
		t.Errorf("Expected ErrDeviceNotFound, got %v", err)
	}
}

func TestDeviceRepository_GetByUsername(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	device := &models.Device{
		Name:         "Find Me Phone",
		Username:     "findme",
		PasswordHash: "hashed_password",
		DeviceType:   "grandstream",
	}
	if err := db.Devices.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	retrieved, err := db.Devices.GetByUsername(ctx, "findme")
	if err != nil {
		t.Fatalf("Failed to get device by username: %v", err)
	}

	if retrieved.ID != device.ID {
		t.Errorf("Expected ID %d, got %d", device.ID, retrieved.ID)
	}
}

func TestDeviceRepository_GetByUsername_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Devices.GetByUsername(ctx, "nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("Expected ErrDeviceNotFound, got %v", err)
	}
}

func TestDeviceRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	device := &models.Device{
		Name:             "Original Name",
		Username:         "updateme",
		PasswordHash:     "hashed_password",
		DeviceType:       "softphone",
		RecordingEnabled: false,
	}
	if err := db.Devices.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	// Update the device
	device.Name = "Updated Name"
	device.RecordingEnabled = true
	if err := db.Devices.Update(ctx, device); err != nil {
		t.Fatalf("Failed to update device: %v", err)
	}

	// Verify update
	retrieved, err := db.Devices.GetByID(ctx, device.ID)
	if err != nil {
		t.Fatalf("Failed to get updated device: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", retrieved.Name)
	}
	if !retrieved.RecordingEnabled {
		t.Error("Expected RecordingEnabled to be true")
	}
}

func TestDeviceRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	device := &models.Device{
		Name:         "Delete Me",
		Username:     "deleteme",
		PasswordHash: "hashed_password",
		DeviceType:   "softphone",
	}
	if err := db.Devices.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	if err := db.Devices.Delete(ctx, device.ID); err != nil {
		t.Fatalf("Failed to delete device: %v", err)
	}

	_, err := db.Devices.GetByID(ctx, device.ID)
	if err != ErrDeviceNotFound {
		t.Errorf("Expected ErrDeviceNotFound after delete, got %v", err)
	}
}

func TestDeviceRepository_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create multiple devices
	for i := 0; i < 5; i++ {
		device := &models.Device{
			Name:         "Device " + string(rune('A'+i)),
			Username:     "device" + string(rune('0'+i)),
			PasswordHash: "hashed_password",
			DeviceType:   "softphone",
		}
		if err := db.Devices.Create(ctx, device); err != nil {
			t.Fatalf("Failed to create device %d: %v", i, err)
		}
	}

	// List with pagination
	devices, err := db.Devices.List(ctx, 3, 0)
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}

	if len(devices) != 3 {
		t.Errorf("Expected 3 devices, got %d", len(devices))
	}
}

func TestDeviceRepository_ListByUser(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create a user first
	user := &models.User{
		Email:        "deviceowner@example.com",
		PasswordHash: "hashed_password",
		Role:         "user",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create devices for the user
	for i := 0; i < 3; i++ {
		device := &models.Device{
			UserID:       &user.ID,
			Name:         "User Device " + string(rune('A'+i)),
			Username:     "userdevice" + string(rune('0'+i)),
			PasswordHash: "hashed_password",
			DeviceType:   "softphone",
		}
		if err := db.Devices.Create(ctx, device); err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}
	}

	// Create device without user
	device := &models.Device{
		Name:         "Orphan Device",
		Username:     "orphan",
		PasswordHash: "hashed_password",
		DeviceType:   "softphone",
	}
	if err := db.Devices.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create orphan device: %v", err)
	}

	// List devices by user
	devices, err := db.Devices.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to list devices by user: %v", err)
	}

	if len(devices) != 3 {
		t.Errorf("Expected 3 devices for user, got %d", len(devices))
	}
}

func TestDeviceRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially 0
	count, err := db.Devices.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count devices: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 devices, got %d", count)
	}

	// Create devices
	for i := 0; i < 4; i++ {
		device := &models.Device{
			Name:         "Count Device " + string(rune('A'+i)),
			Username:     "count" + string(rune('0'+i)),
			PasswordHash: "hashed_password",
			DeviceType:   "softphone",
		}
		if err := db.Devices.Create(ctx, device); err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}
	}

	count, err = db.Devices.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count devices: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 devices, got %d", count)
	}
}

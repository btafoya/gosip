package db

import (
	"context"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestDIDRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{
		Number:       "+15551234567",
		TwilioSID:    "PN123456789",
		Name:         "Main Line",
		SMSEnabled:   true,
		VoiceEnabled: true,
	}

	err := db.DIDs.Create(ctx, did)
	if err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	if did.ID == 0 {
		t.Error("Expected DID ID to be set after creation")
	}
}

func TestDIDRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{
		Number:       "+15551234567",
		TwilioSID:    "PN123456789",
		Name:         "Test Line",
		SMSEnabled:   true,
		VoiceEnabled: true,
	}
	if err := db.DIDs.Create(ctx, did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	retrieved, err := db.DIDs.GetByID(ctx, did.ID)
	if err != nil {
		t.Fatalf("Failed to get DID by ID: %v", err)
	}

	if retrieved.Number != did.Number {
		t.Errorf("Expected number %s, got %s", did.Number, retrieved.Number)
	}
	if retrieved.Name != did.Name {
		t.Errorf("Expected name %s, got %s", did.Name, retrieved.Name)
	}
}

func TestDIDRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.DIDs.GetByID(ctx, 9999)
	if err != ErrDIDNotFound {
		t.Errorf("Expected ErrDIDNotFound, got %v", err)
	}
}

func TestDIDRepository_GetByNumber(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{
		Number:       "+15559876543",
		TwilioSID:    "PN987654321",
		Name:         "Find Me",
		SMSEnabled:   false,
		VoiceEnabled: true,
	}
	if err := db.DIDs.Create(ctx, did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	retrieved, err := db.DIDs.GetByNumber(ctx, "+15559876543")
	if err != nil {
		t.Fatalf("Failed to get DID by number: %v", err)
	}

	if retrieved.ID != did.ID {
		t.Errorf("Expected ID %d, got %d", did.ID, retrieved.ID)
	}
}

func TestDIDRepository_GetByNumber_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.DIDs.GetByNumber(ctx, "+15550000000")
	if err != ErrDIDNotFound {
		t.Errorf("Expected ErrDIDNotFound, got %v", err)
	}
}

func TestDIDRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{
		Number:       "+15551111111",
		TwilioSID:    "PN111111111",
		Name:         "Original Name",
		SMSEnabled:   false,
		VoiceEnabled: true,
	}
	if err := db.DIDs.Create(ctx, did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	// Update the DID
	did.Name = "Updated Name"
	did.SMSEnabled = true
	if err := db.DIDs.Update(ctx, did); err != nil {
		t.Fatalf("Failed to update DID: %v", err)
	}

	// Verify update
	retrieved, err := db.DIDs.GetByID(ctx, did.ID)
	if err != nil {
		t.Fatalf("Failed to get updated DID: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", retrieved.Name)
	}
	if !retrieved.SMSEnabled {
		t.Error("Expected SMSEnabled to be true")
	}
}

func TestDIDRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{
		Number:       "+15552222222",
		TwilioSID:    "PN222222222",
		Name:         "Delete Me",
		SMSEnabled:   true,
		VoiceEnabled: true,
	}
	if err := db.DIDs.Create(ctx, did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	if err := db.DIDs.Delete(ctx, did.ID); err != nil {
		t.Fatalf("Failed to delete DID: %v", err)
	}

	_, err := db.DIDs.GetByID(ctx, did.ID)
	if err != ErrDIDNotFound {
		t.Errorf("Expected ErrDIDNotFound after delete, got %v", err)
	}
}

func TestDIDRepository_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create multiple DIDs
	numbers := []string{"+15551000001", "+15551000002", "+15551000003"}
	for i, num := range numbers {
		did := &models.DID{
			Number:       num,
			TwilioSID:    "PN" + num,
			Name:         "Line " + string(rune('A'+i)),
			SMSEnabled:   true,
			VoiceEnabled: true,
		}
		if err := db.DIDs.Create(ctx, did); err != nil {
			t.Fatalf("Failed to create DID: %v", err)
		}
	}

	dids, err := db.DIDs.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list DIDs: %v", err)
	}

	if len(dids) != 3 {
		t.Errorf("Expected 3 DIDs, got %d", len(dids))
	}
}

func TestDIDRepository_ListVoiceEnabled(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create DIDs with different voice settings
	dids := []struct {
		number       string
		voiceEnabled bool
	}{
		{"+15553000001", true},
		{"+15553000002", false},
		{"+15553000003", true},
		{"+15553000004", false},
	}

	for _, d := range dids {
		did := &models.DID{
			Number:       d.number,
			TwilioSID:    "PN" + d.number,
			Name:         "Test",
			VoiceEnabled: d.voiceEnabled,
		}
		if err := db.DIDs.Create(ctx, did); err != nil {
			t.Fatalf("Failed to create DID: %v", err)
		}
	}

	voiceDIDs, err := db.DIDs.ListVoiceEnabled(ctx)
	if err != nil {
		t.Fatalf("Failed to list voice-enabled DIDs: %v", err)
	}

	if len(voiceDIDs) != 2 {
		t.Errorf("Expected 2 voice-enabled DIDs, got %d", len(voiceDIDs))
	}
}

func TestDIDRepository_ListSMSEnabled(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create DIDs with different SMS settings
	dids := []struct {
		number     string
		smsEnabled bool
	}{
		{"+15554000001", true},
		{"+15554000002", true},
		{"+15554000003", false},
	}

	for _, d := range dids {
		did := &models.DID{
			Number:       d.number,
			TwilioSID:    "PN" + d.number,
			Name:         "Test",
			SMSEnabled:   d.smsEnabled,
			VoiceEnabled: true,
		}
		if err := db.DIDs.Create(ctx, did); err != nil {
			t.Fatalf("Failed to create DID: %v", err)
		}
	}

	smsDIDs, err := db.DIDs.ListSMSEnabled(ctx)
	if err != nil {
		t.Fatalf("Failed to list SMS-enabled DIDs: %v", err)
	}

	if len(smsDIDs) != 2 {
		t.Errorf("Expected 2 SMS-enabled DIDs, got %d", len(smsDIDs))
	}
}

func TestDIDRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially 0
	count, err := db.DIDs.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count DIDs: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 DIDs, got %d", count)
	}

	// Create DIDs
	for i := 0; i < 5; i++ {
		did := &models.DID{
			Number:       "+1555500000" + string(rune('0'+i)),
			TwilioSID:    "PN" + string(rune('0'+i)),
			Name:         "Count Test",
			VoiceEnabled: true,
		}
		if err := db.DIDs.Create(ctx, did); err != nil {
			t.Fatalf("Failed to create DID: %v", err)
		}
	}

	count, err = db.DIDs.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count DIDs: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 DIDs, got %d", count)
	}
}

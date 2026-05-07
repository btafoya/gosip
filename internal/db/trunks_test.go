package db

import (
	"context"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func createTestTrunk(t *testing.T, database *DB, twilioSID, friendlyName string) *models.Trunk {
	t.Helper()
	trunk := &models.Trunk{
		TwilioSID:    twilioSID,
		FriendlyName: friendlyName,
		DomainName:   "example.sip.twilio.com",
		Secure:       true,
		TransferMode: "enable-all",
	}
	if err := database.Trunks.Create(context.Background(), trunk); err != nil {
		t.Fatalf("Failed to create test trunk: %v", err)
	}
	return trunk
}

func TestTrunkRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	trunk := createTestTrunk(t, db, "TK123456", "Main Trunk")

	if trunk.ID == 0 {
		t.Error("Expected trunk ID to be set")
	}
	if trunk.TwilioSID != "TK123456" {
		t.Errorf("Expected TwilioSID TK123456, got %s", trunk.TwilioSID)
	}
}

func TestTrunkRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	created := createTestTrunk(t, db, "TK123456", "Main Trunk")

	trunk, err := db.Trunks.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Failed to get trunk: %v", err)
	}
	if trunk.TwilioSID != "TK123456" {
		t.Errorf("Expected TwilioSID TK123456, got %s", trunk.TwilioSID)
	}
}

func TestTrunkRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.Trunks.GetByID(context.Background(), 9999)
	if err != ErrTrunkNotFound {
		t.Errorf("Expected ErrTrunkNotFound, got %v", err)
	}
}

func TestTrunkRepository_GetByTwilioSID(t *testing.T) {
	db := setupTestDB(t)
	created := createTestTrunk(t, db, "TK789012", "Secondary")

	trunk, err := db.Trunks.GetByTwilioSID(context.Background(), "TK789012")
	if err != nil {
		t.Fatalf("Failed to get trunk: %v", err)
	}
	if trunk.ID != created.ID {
		t.Errorf("Expected ID %d, got %d", created.ID, trunk.ID)
	}
}

func TestTrunkRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	trunk := createTestTrunk(t, db, "TK555", "Old Name")

	trunk.FriendlyName = "New Name"
	trunk.Secure = false
	if err := db.Trunks.Update(context.Background(), trunk); err != nil {
		t.Fatalf("Failed to update trunk: %v", err)
	}

	updated, _ := db.Trunks.GetByID(context.Background(), trunk.ID)
	if updated.FriendlyName != "New Name" {
		t.Errorf("Expected FriendlyName 'New Name', got %s", updated.FriendlyName)
	}
	if updated.Secure {
		t.Error("Expected Secure to be false")
	}
}

func TestTrunkRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	trunk := createTestTrunk(t, db, "TK999", "To Delete")

	if err := db.Trunks.Delete(context.Background(), trunk.ID); err != nil {
		t.Fatalf("Failed to delete trunk: %v", err)
	}

	_, err := db.Trunks.GetByID(context.Background(), trunk.ID)
	if err != ErrTrunkNotFound {
		t.Errorf("Expected ErrTrunkNotFound after delete, got %v", err)
	}
}

func TestTrunkRepository_List(t *testing.T) {
	db := setupTestDB(t)
	createTestTrunk(t, db, "TK001", "Trunk A")
	createTestTrunk(t, db, "TK002", "Trunk B")

	trunks, err := db.Trunks.List(context.Background())
	if err != nil {
		t.Fatalf("Failed to list trunks: %v", err)
	}
	if len(trunks) != 2 {
		t.Errorf("Expected 2 trunks, got %d", len(trunks))
	}
}

func TestTrunkRepository_ListByDID(t *testing.T) {
	db := setupTestDB(t)
	trunk := createTestTrunk(t, db, "TK111", "Assigned Trunk")

	did := &models.DID{
		Number:       "+15551234567",
		TwilioSID:    "PN123",
		VoiceEnabled: true,
		TrunkID:      &trunk.ID,
	}
	if err := db.DIDs.Create(context.Background(), did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	found, err := db.Trunks.ListByDID(context.Background(), did.ID)
	if err != nil {
		t.Fatalf("Failed to find trunk by DID: %v", err)
	}
	if found.ID != trunk.ID {
		t.Errorf("Expected trunk ID %d, got %d", trunk.ID, found.ID)
	}
}

func TestTrunkRepository_Delete_CascadeDID(t *testing.T) {
	db := setupTestDB(t)
	trunk := createTestTrunk(t, db, "TK222", "Cascade Test")

	did := &models.DID{
		Number:       "+15551234567",
		TwilioSID:    "PN456",
		VoiceEnabled: true,
		TrunkID:      &trunk.ID,
	}
	if err := db.DIDs.Create(context.Background(), did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	if err := db.Trunks.Delete(context.Background(), trunk.ID); err != nil {
		t.Fatalf("Failed to delete trunk: %v", err)
	}

	updated, _ := db.DIDs.GetByID(context.Background(), did.ID)
	if updated.TrunkID != nil {
		t.Error("Expected DID trunk_id to be NULL after trunk deletion")
	}
}

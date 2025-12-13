package db

import (
	"context"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestBlocklistRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Spam caller",
	}

	err := db.Blocklist.Create(ctx, entry)
	if err != nil {
		t.Fatalf("Failed to create blocklist entry: %v", err)
	}

	if entry.ID == 0 {
		t.Error("Expected entry ID to be set after creation")
	}
}

func TestBlocklistRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Telemarketer",
	}
	if err := db.Blocklist.Create(ctx, entry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	retrieved, err := db.Blocklist.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Failed to get entry by ID: %v", err)
	}

	if retrieved.Pattern != entry.Pattern {
		t.Errorf("Expected pattern %s, got %s", entry.Pattern, retrieved.Pattern)
	}
	if retrieved.PatternType != entry.PatternType {
		t.Errorf("Expected pattern type %s, got %s", entry.PatternType, retrieved.PatternType)
	}
}

func TestBlocklistRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Blocklist.GetByID(ctx, 9999)
	if err != ErrBlocklistEntryNotFound {
		t.Errorf("Expected ErrBlocklistEntryNotFound, got %v", err)
	}
}

func TestBlocklistRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Original reason",
	}
	if err := db.Blocklist.Create(ctx, entry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	entry.Reason = "Updated reason"
	entry.PatternType = "prefix"
	if err := db.Blocklist.Update(ctx, entry); err != nil {
		t.Fatalf("Failed to update entry: %v", err)
	}

	retrieved, err := db.Blocklist.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Failed to get updated entry: %v", err)
	}

	if retrieved.Reason != "Updated reason" {
		t.Errorf("Expected reason 'Updated reason', got %s", retrieved.Reason)
	}
	if retrieved.PatternType != "prefix" {
		t.Errorf("Expected pattern type 'prefix', got %s", retrieved.PatternType)
	}
}

func TestBlocklistRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Delete me",
	}
	if err := db.Blocklist.Create(ctx, entry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	if err := db.Blocklist.Delete(ctx, entry.ID); err != nil {
		t.Fatalf("Failed to delete entry: %v", err)
	}

	_, err := db.Blocklist.GetByID(ctx, entry.ID)
	if err != ErrBlocklistEntryNotFound {
		t.Errorf("Expected ErrBlocklistEntryNotFound after delete, got %v", err)
	}
}

func TestBlocklistRepository_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entries := []struct {
		pattern     string
		patternType string
	}{
		{"+15551111111", "exact"},
		{"+1555", "prefix"},
		{"^\\+1800", "regex"},
	}

	for _, e := range entries {
		entry := &models.BlocklistEntry{
			Pattern:     e.pattern,
			PatternType: e.patternType,
			Reason:      "Test",
		}
		if err := db.Blocklist.Create(ctx, entry); err != nil {
			t.Fatalf("Failed to create entry: %v", err)
		}
	}

	list, err := db.Blocklist.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list entries: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(list))
	}
}

func TestBlocklistRepository_IsBlocked_Exact(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Exact block",
	}
	if err := db.Blocklist.Create(ctx, entry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	// Test exact match
	blocked, matchedEntry, err := db.Blocklist.IsBlocked(ctx, "+15551234567")
	if err != nil {
		t.Fatalf("Failed to check blocked: %v", err)
	}
	if !blocked {
		t.Error("Expected number to be blocked")
	}
	if matchedEntry == nil || matchedEntry.ID != entry.ID {
		t.Error("Expected matched entry to be returned")
	}

	// Test non-match
	blocked, _, err = db.Blocklist.IsBlocked(ctx, "+15559999999")
	if err != nil {
		t.Fatalf("Failed to check blocked: %v", err)
	}
	if blocked {
		t.Error("Expected number to not be blocked")
	}
}

func TestBlocklistRepository_IsBlocked_Prefix(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     "+1800",
		PatternType: "prefix",
		Reason:      "Block 800 numbers",
	}
	if err := db.Blocklist.Create(ctx, entry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	// Test prefix match
	blocked, _, err := db.Blocklist.IsBlocked(ctx, "+18001234567")
	if err != nil {
		t.Fatalf("Failed to check blocked: %v", err)
	}
	if !blocked {
		t.Error("Expected 800 number to be blocked")
	}

	// Test another 800 number
	blocked, _, err = db.Blocklist.IsBlocked(ctx, "+18009876543")
	if err != nil {
		t.Fatalf("Failed to check blocked: %v", err)
	}
	if !blocked {
		t.Error("Expected 800 number to be blocked")
	}

	// Test non-800 number
	blocked, _, err = db.Blocklist.IsBlocked(ctx, "+15551234567")
	if err != nil {
		t.Fatalf("Failed to check blocked: %v", err)
	}
	if blocked {
		t.Error("Expected non-800 number to not be blocked")
	}
}

func TestBlocklistRepository_IsBlocked_Regex(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     `^\+1(800|888|877)`,
		PatternType: "regex",
		Reason:      "Block toll-free numbers",
	}
	if err := db.Blocklist.Create(ctx, entry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	// Test matching patterns
	tollFreeNumbers := []string{"+18001234567", "+18881234567", "+18771234567"}
	for _, num := range tollFreeNumbers {
		blocked, _, err := db.Blocklist.IsBlocked(ctx, num)
		if err != nil {
			t.Fatalf("Failed to check blocked: %v", err)
		}
		if !blocked {
			t.Errorf("Expected %s to be blocked", num)
		}
	}

	// Test non-matching
	blocked, _, err := db.Blocklist.IsBlocked(ctx, "+15551234567")
	if err != nil {
		t.Fatalf("Failed to check blocked: %v", err)
	}
	if blocked {
		t.Error("Expected regular number to not be blocked")
	}
}

func TestBlocklistRepository_IsBlocked_NormalizesNumbers(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Test normalization",
	}
	if err := db.Blocklist.Create(ctx, entry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	// Test with various formats
	formats := []string{
		"+15551234567",
		"+1 555 123 4567",
		"+1-555-123-4567",
		"+1 (555) 123-4567",
	}

	for _, num := range formats {
		blocked, _, err := db.Blocklist.IsBlocked(ctx, num)
		if err != nil {
			t.Fatalf("Failed to check blocked for %s: %v", num, err)
		}
		if !blocked {
			t.Errorf("Expected %s to be blocked (normalized)", num)
		}
	}
}

func TestBlocklistRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially 0
	count, err := db.Blocklist.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 entries, got %d", count)
	}

	// Create entries
	for i := 0; i < 4; i++ {
		entry := &models.BlocklistEntry{
			Pattern:     "+1555000000" + string(rune('0'+i)),
			PatternType: "exact",
			Reason:      "Test",
		}
		if err := db.Blocklist.Create(ctx, entry); err != nil {
			t.Fatalf("Failed to create entry: %v", err)
		}
	}

	count, err = db.Blocklist.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 entries, got %d", count)
	}
}

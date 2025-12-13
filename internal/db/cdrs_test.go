package db

import (
	"context"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

func TestCDRRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	cdr := &models.CDR{
		CallSID:     "CA123456789",
		Direction:   "inbound",
		FromNumber:  "+15551234567",
		ToNumber:    "+15559876543",
		StartedAt:   time.Now(),
		Disposition: "answered",
		Duration:    120,
	}

	err := db.CDRs.Create(ctx, cdr)
	if err != nil {
		t.Fatalf("Failed to create CDR: %v", err)
	}

	if cdr.ID == 0 {
		t.Error("Expected CDR ID to be set after creation")
	}
}

func TestCDRRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	cdr := &models.CDR{
		CallSID:     "CA123456789",
		Direction:   "outbound",
		FromNumber:  "+15551234567",
		ToNumber:    "+15559876543",
		StartedAt:   time.Now(),
		Disposition: "answered",
		Duration:    60,
	}
	if err := db.CDRs.Create(ctx, cdr); err != nil {
		t.Fatalf("Failed to create CDR: %v", err)
	}

	retrieved, err := db.CDRs.GetByID(ctx, cdr.ID)
	if err != nil {
		t.Fatalf("Failed to get CDR by ID: %v", err)
	}

	if retrieved.CallSID != cdr.CallSID {
		t.Errorf("Expected CallSID %s, got %s", cdr.CallSID, retrieved.CallSID)
	}
	if retrieved.Direction != cdr.Direction {
		t.Errorf("Expected Direction %s, got %s", cdr.Direction, retrieved.Direction)
	}
}

func TestCDRRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.CDRs.GetByID(ctx, 9999)
	if err != ErrCDRNotFound {
		t.Errorf("Expected ErrCDRNotFound, got %v", err)
	}
}

func TestCDRRepository_GetByCallSID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	cdr := &models.CDR{
		CallSID:     "CA_UNIQUE_123",
		Direction:   "inbound",
		FromNumber:  "+15551234567",
		ToNumber:    "+15559876543",
		StartedAt:   time.Now(),
		Disposition: "missed",
	}
	if err := db.CDRs.Create(ctx, cdr); err != nil {
		t.Fatalf("Failed to create CDR: %v", err)
	}

	retrieved, err := db.CDRs.GetByCallSID(ctx, "CA_UNIQUE_123")
	if err != nil {
		t.Fatalf("Failed to get CDR by CallSID: %v", err)
	}

	if retrieved.ID != cdr.ID {
		t.Errorf("Expected ID %d, got %d", cdr.ID, retrieved.ID)
	}
}

func TestCDRRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	cdr := &models.CDR{
		CallSID:     "CA_UPDATE_123",
		Direction:   "inbound",
		FromNumber:  "+15551234567",
		ToNumber:    "+15559876543",
		StartedAt:   time.Now(),
		Disposition: "answered",
		Duration:    0,
	}
	if err := db.CDRs.Create(ctx, cdr); err != nil {
		t.Fatalf("Failed to create CDR: %v", err)
	}

	// Update with end time and duration
	now := time.Now()
	cdr.EndedAt = &now
	cdr.Duration = 180
	if err := db.CDRs.Update(ctx, cdr); err != nil {
		t.Fatalf("Failed to update CDR: %v", err)
	}

	retrieved, err := db.CDRs.GetByID(ctx, cdr.ID)
	if err != nil {
		t.Fatalf("Failed to get updated CDR: %v", err)
	}

	if retrieved.Duration != 180 {
		t.Errorf("Expected duration 180, got %d", retrieved.Duration)
	}
	if retrieved.EndedAt == nil {
		t.Error("Expected EndedAt to be set")
	}
}

func TestCDRRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	cdr := &models.CDR{
		CallSID:     "CA_DELETE_123",
		Direction:   "inbound",
		FromNumber:  "+15551234567",
		ToNumber:    "+15559876543",
		StartedAt:   time.Now(),
		Disposition: "blocked",
	}
	if err := db.CDRs.Create(ctx, cdr); err != nil {
		t.Fatalf("Failed to create CDR: %v", err)
	}

	if err := db.CDRs.Delete(ctx, cdr.ID); err != nil {
		t.Fatalf("Failed to delete CDR: %v", err)
	}

	_, err := db.CDRs.GetByID(ctx, cdr.ID)
	if err != ErrCDRNotFound {
		t.Errorf("Expected ErrCDRNotFound after delete, got %v", err)
	}
}

func TestCDRRepository_List_WithFilters(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test CDRs
	dispositions := []string{"answered", "missed", "voicemail", "blocked"}
	for i, disp := range dispositions {
		cdr := &models.CDR{
			CallSID:     "CA_LIST_" + string(rune('0'+i)),
			Direction:   "inbound",
			FromNumber:  "+1555000000" + string(rune('0'+i)),
			ToNumber:    "+15559876543",
			StartedAt:   time.Now().Add(time.Duration(-i) * time.Hour),
			Disposition: disp,
		}
		if err := db.CDRs.Create(ctx, cdr); err != nil {
			t.Fatalf("Failed to create CDR: %v", err)
		}
	}

	// Test listing all
	cdrs, err := db.CDRs.List(ctx, CDRFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to list CDRs: %v", err)
	}
	if len(cdrs) != 4 {
		t.Errorf("Expected 4 CDRs, got %d", len(cdrs))
	}

	// Test filtering by disposition
	cdrs, err = db.CDRs.List(ctx, CDRFilter{Disposition: "answered", Limit: 10})
	if err != nil {
		t.Fatalf("Failed to list CDRs: %v", err)
	}
	if len(cdrs) != 1 {
		t.Errorf("Expected 1 answered CDR, got %d", len(cdrs))
	}

	// Test filtering by direction
	cdrs, err = db.CDRs.List(ctx, CDRFilter{Direction: "inbound", Limit: 10})
	if err != nil {
		t.Fatalf("Failed to list CDRs: %v", err)
	}
	if len(cdrs) != 4 {
		t.Errorf("Expected 4 inbound CDRs, got %d", len(cdrs))
	}
}

func TestCDRRepository_List_DateRange(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	// Create CDRs at different times
	times := []time.Time{now, yesterday, twoDaysAgo}
	for i, startTime := range times {
		cdr := &models.CDR{
			CallSID:     "CA_DATE_" + string(rune('0'+i)),
			Direction:   "inbound",
			FromNumber:  "+15551234567",
			ToNumber:    "+15559876543",
			StartedAt:   startTime,
			Disposition: "answered",
		}
		if err := db.CDRs.Create(ctx, cdr); err != nil {
			t.Fatalf("Failed to create CDR: %v", err)
		}
	}

	// Filter by date range (yesterday to now)
	startDate := yesterday.Add(-1 * time.Hour)
	endDate := now.Add(1 * time.Hour)
	cdrs, err := db.CDRs.List(ctx, CDRFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Failed to list CDRs: %v", err)
	}
	if len(cdrs) != 2 {
		t.Errorf("Expected 2 CDRs in date range, got %d", len(cdrs))
	}
}

func TestCDRRepository_List_Pagination(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create 10 CDRs
	for i := 0; i < 10; i++ {
		cdr := &models.CDR{
			CallSID:     "CA_PAGE_" + string(rune('0'+i)),
			Direction:   "inbound",
			FromNumber:  "+15551234567",
			ToNumber:    "+15559876543",
			StartedAt:   time.Now().Add(time.Duration(-i) * time.Minute),
			Disposition: "answered",
		}
		if err := db.CDRs.Create(ctx, cdr); err != nil {
			t.Fatalf("Failed to create CDR: %v", err)
		}
	}

	// First page
	cdrs, err := db.CDRs.List(ctx, CDRFilter{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("Failed to list CDRs: %v", err)
	}
	if len(cdrs) != 3 {
		t.Errorf("Expected 3 CDRs on page 1, got %d", len(cdrs))
	}

	// Second page
	cdrs, err = db.CDRs.List(ctx, CDRFilter{Limit: 3, Offset: 3})
	if err != nil {
		t.Fatalf("Failed to list CDRs: %v", err)
	}
	if len(cdrs) != 3 {
		t.Errorf("Expected 3 CDRs on page 2, got %d", len(cdrs))
	}
}

func TestCDRRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create CDRs with different dispositions
	for i := 0; i < 5; i++ {
		cdr := &models.CDR{
			CallSID:     "CA_COUNT_" + string(rune('0'+i)),
			Direction:   "inbound",
			FromNumber:  "+15551234567",
			ToNumber:    "+15559876543",
			StartedAt:   time.Now(),
			Disposition: "answered",
		}
		if err := db.CDRs.Create(ctx, cdr); err != nil {
			t.Fatalf("Failed to create CDR: %v", err)
		}
	}

	count, err := db.CDRs.Count(ctx, CDRFilter{})
	if err != nil {
		t.Fatalf("Failed to count CDRs: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 CDRs, got %d", count)
	}

	// Count with filter
	count, err = db.CDRs.Count(ctx, CDRFilter{Disposition: "answered"})
	if err != nil {
		t.Fatalf("Failed to count filtered CDRs: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 answered CDRs, got %d", count)
	}
}

func TestCDRRepository_GetRecent(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create 10 CDRs
	for i := 0; i < 10; i++ {
		cdr := &models.CDR{
			CallSID:     "CA_RECENT_" + string(rune('0'+i)),
			Direction:   "inbound",
			FromNumber:  "+15551234567",
			ToNumber:    "+15559876543",
			StartedAt:   time.Now().Add(time.Duration(-i) * time.Minute),
			Disposition: "answered",
		}
		if err := db.CDRs.Create(ctx, cdr); err != nil {
			t.Fatalf("Failed to create CDR: %v", err)
		}
	}

	recent, err := db.CDRs.GetRecent(ctx, 5)
	if err != nil {
		t.Fatalf("Failed to get recent CDRs: %v", err)
	}
	if len(recent) != 5 {
		t.Errorf("Expected 5 recent CDRs, got %d", len(recent))
	}
}

func TestCDRRepository_GetStatsByDisposition(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	now := time.Now()
	startDate := now.Add(-24 * time.Hour)
	endDate := now.Add(1 * time.Hour)

	// Create CDRs with various dispositions
	dispositions := []string{"answered", "answered", "missed", "voicemail", "blocked", "answered"}
	for i, disp := range dispositions {
		cdr := &models.CDR{
			CallSID:     "CA_STATS_" + string(rune('0'+i)),
			Direction:   "inbound",
			FromNumber:  "+15551234567",
			ToNumber:    "+15559876543",
			StartedAt:   now.Add(time.Duration(-i) * time.Hour),
			Disposition: disp,
		}
		if err := db.CDRs.Create(ctx, cdr); err != nil {
			t.Fatalf("Failed to create CDR: %v", err)
		}
	}

	stats, err := db.CDRs.GetStatsByDisposition(ctx, startDate, endDate)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats["answered"] != 3 {
		t.Errorf("Expected 3 answered calls, got %d", stats["answered"])
	}
	if stats["missed"] != 1 {
		t.Errorf("Expected 1 missed call, got %d", stats["missed"])
	}
	if stats["voicemail"] != 1 {
		t.Errorf("Expected 1 voicemail, got %d", stats["voicemail"])
	}
	if stats["blocked"] != 1 {
		t.Errorf("Expected 1 blocked, got %d", stats["blocked"])
	}
}

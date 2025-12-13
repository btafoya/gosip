package db

import (
	"context"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestVoicemailRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	vm := &models.Voicemail{
		FromNumber: "+15551234567",
		AudioURL:   "https://example.com/audio.mp3",
		Transcript: "Hello, this is a test message.",
		Duration:   30,
		IsRead:     false,
	}

	err := db.Voicemails.Create(ctx, vm)
	if err != nil {
		t.Fatalf("Failed to create voicemail: %v", err)
	}

	if vm.ID == 0 {
		t.Error("Expected voicemail ID to be set after creation")
	}
}

func TestVoicemailRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	vm := &models.Voicemail{
		FromNumber: "+15551234567",
		AudioURL:   "https://example.com/audio.mp3",
		Transcript: "Test transcript",
		Duration:   45,
		IsRead:     false,
	}
	if err := db.Voicemails.Create(ctx, vm); err != nil {
		t.Fatalf("Failed to create voicemail: %v", err)
	}

	retrieved, err := db.Voicemails.GetByID(ctx, vm.ID)
	if err != nil {
		t.Fatalf("Failed to get voicemail by ID: %v", err)
	}

	if retrieved.FromNumber != vm.FromNumber {
		t.Errorf("Expected from number %s, got %s", vm.FromNumber, retrieved.FromNumber)
	}
	if retrieved.Transcript != vm.Transcript {
		t.Errorf("Expected transcript %s, got %s", vm.Transcript, retrieved.Transcript)
	}
}

func TestVoicemailRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Voicemails.GetByID(ctx, 9999)
	if err != ErrVoicemailNotFound {
		t.Errorf("Expected ErrVoicemailNotFound, got %v", err)
	}
}

func TestVoicemailRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	vm := &models.Voicemail{
		FromNumber: "+15551234567",
		AudioURL:   "https://example.com/audio.mp3",
		Transcript: "",
		Duration:   60,
		IsRead:     false,
	}
	if err := db.Voicemails.Create(ctx, vm); err != nil {
		t.Fatalf("Failed to create voicemail: %v", err)
	}

	// Update the voicemail
	vm.Transcript = "Updated transcript"
	vm.IsRead = true
	if err := db.Voicemails.Update(ctx, vm); err != nil {
		t.Fatalf("Failed to update voicemail: %v", err)
	}

	retrieved, err := db.Voicemails.GetByID(ctx, vm.ID)
	if err != nil {
		t.Fatalf("Failed to get updated voicemail: %v", err)
	}

	if retrieved.Transcript != "Updated transcript" {
		t.Errorf("Expected transcript 'Updated transcript', got %s", retrieved.Transcript)
	}
	if !retrieved.IsRead {
		t.Error("Expected IsRead to be true")
	}
}

func TestVoicemailRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	vm := &models.Voicemail{
		FromNumber: "+15551234567",
		AudioURL:   "https://example.com/audio.mp3",
		Duration:   30,
	}
	if err := db.Voicemails.Create(ctx, vm); err != nil {
		t.Fatalf("Failed to create voicemail: %v", err)
	}

	if err := db.Voicemails.Delete(ctx, vm.ID); err != nil {
		t.Fatalf("Failed to delete voicemail: %v", err)
	}

	_, err := db.Voicemails.GetByID(ctx, vm.ID)
	if err != ErrVoicemailNotFound {
		t.Errorf("Expected ErrVoicemailNotFound after delete, got %v", err)
	}
}

func TestVoicemailRepository_MarkAsRead(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	vm := &models.Voicemail{
		FromNumber: "+15551234567",
		AudioURL:   "https://example.com/audio.mp3",
		Duration:   30,
		IsRead:     false,
	}
	if err := db.Voicemails.Create(ctx, vm); err != nil {
		t.Fatalf("Failed to create voicemail: %v", err)
	}

	if err := db.Voicemails.MarkAsRead(ctx, vm.ID); err != nil {
		t.Fatalf("Failed to mark as read: %v", err)
	}

	retrieved, _ := db.Voicemails.GetByID(ctx, vm.ID)
	if !retrieved.IsRead {
		t.Error("Expected voicemail to be marked as read")
	}
}

func TestVoicemailRepository_MarkAsUnread(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	vm := &models.Voicemail{
		FromNumber: "+15551234567",
		AudioURL:   "https://example.com/audio.mp3",
		Duration:   30,
		IsRead:     true,
	}
	if err := db.Voicemails.Create(ctx, vm); err != nil {
		t.Fatalf("Failed to create voicemail: %v", err)
	}

	if err := db.Voicemails.MarkAsUnread(ctx, vm.ID); err != nil {
		t.Fatalf("Failed to mark as unread: %v", err)
	}

	retrieved, _ := db.Voicemails.GetByID(ctx, vm.ID)
	if retrieved.IsRead {
		t.Error("Expected voicemail to be marked as unread")
	}
}

func TestVoicemailRepository_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create multiple voicemails
	for i := 0; i < 5; i++ {
		vm := &models.Voicemail{
			FromNumber: "+1555000000" + string(rune('0'+i)),
			AudioURL:   "https://example.com/audio" + string(rune('0'+i)) + ".mp3",
			Duration:   30 + i*10,
		}
		if err := db.Voicemails.Create(ctx, vm); err != nil {
			t.Fatalf("Failed to create voicemail: %v", err)
		}
	}

	// List with pagination
	vms, err := db.Voicemails.List(ctx, 3, 0)
	if err != nil {
		t.Fatalf("Failed to list voicemails: %v", err)
	}

	if len(vms) != 3 {
		t.Errorf("Expected 3 voicemails, got %d", len(vms))
	}
}

func TestVoicemailRepository_ListByUser(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create user
	user := &models.User{
		Email:        "vmuser@example.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create voicemails for the user
	for i := 0; i < 3; i++ {
		vm := &models.Voicemail{
			UserID:     &user.ID,
			FromNumber: "+1555000000" + string(rune('0'+i)),
			AudioURL:   "https://example.com/audio.mp3",
			Duration:   30,
		}
		if err := db.Voicemails.Create(ctx, vm); err != nil {
			t.Fatalf("Failed to create voicemail: %v", err)
		}
	}

	// Create voicemail without user
	vm := &models.Voicemail{
		FromNumber: "+15559999999",
		AudioURL:   "https://example.com/audio.mp3",
		Duration:   30,
	}
	db.Voicemails.Create(ctx, vm)

	vms, err := db.Voicemails.ListByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list voicemails by user: %v", err)
	}

	if len(vms) != 3 {
		t.Errorf("Expected 3 voicemails for user, got %d", len(vms))
	}
}

func TestVoicemailRepository_ListUnread(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create read and unread voicemails
	for i := 0; i < 4; i++ {
		vm := &models.Voicemail{
			FromNumber: "+1555000000" + string(rune('0'+i)),
			AudioURL:   "https://example.com/audio.mp3",
			Duration:   30,
			IsRead:     i%2 == 0, // 0 and 2 are read
		}
		if err := db.Voicemails.Create(ctx, vm); err != nil {
			t.Fatalf("Failed to create voicemail: %v", err)
		}
	}

	unread, err := db.Voicemails.ListUnread(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list unread: %v", err)
	}

	if len(unread) != 2 {
		t.Errorf("Expected 2 unread voicemails, got %d", len(unread))
	}
}

func TestVoicemailRepository_CountUnread(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create read and unread voicemails
	for i := 0; i < 5; i++ {
		vm := &models.Voicemail{
			FromNumber: "+1555000000" + string(rune('0'+i)),
			AudioURL:   "https://example.com/audio.mp3",
			Duration:   30,
			IsRead:     i < 2, // 0 and 1 are read
		}
		if err := db.Voicemails.Create(ctx, vm); err != nil {
			t.Fatalf("Failed to create voicemail: %v", err)
		}
	}

	count, err := db.Voicemails.CountUnread(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to count unread: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 unread, got %d", count)
	}
}

func TestVoicemailRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		vm := &models.Voicemail{
			FromNumber: "+1555000000" + string(rune('0'+i)),
			AudioURL:   "https://example.com/audio.mp3",
			Duration:   30,
		}
		if err := db.Voicemails.Create(ctx, vm); err != nil {
			t.Fatalf("Failed to create voicemail: %v", err)
		}
	}

	count, err := db.Voicemails.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count != 4 {
		t.Errorf("Expected 4 voicemails, got %d", count)
	}
}

func TestVoicemailRepository_UpdateTranscript(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	vm := &models.Voicemail{
		FromNumber: "+15551234567",
		AudioURL:   "https://example.com/audio.mp3",
		Transcript: "",
		Duration:   30,
	}
	if err := db.Voicemails.Create(ctx, vm); err != nil {
		t.Fatalf("Failed to create voicemail: %v", err)
	}

	newTranscript := "This is the transcribed message content."
	if err := db.Voicemails.UpdateTranscript(ctx, vm.ID, newTranscript); err != nil {
		t.Fatalf("Failed to update transcript: %v", err)
	}

	retrieved, _ := db.Voicemails.GetByID(ctx, vm.ID)
	if retrieved.Transcript != newTranscript {
		t.Errorf("Expected transcript '%s', got '%s'", newTranscript, retrieved.Transcript)
	}
}

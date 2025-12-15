package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestMWINotifier_UpdateMWIForDID_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	notifier := NewMWINotifier(deps)

	// Should not error when SIP server is not available
	err := notifier.UpdateMWIForDID(context.Background(), 1)
	if err != nil {
		t.Errorf("Expected no error with nil SIP, got %v", err)
	}
}

func TestMWINotifier_UpdateMWIForVoicemail_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	notifier := NewMWINotifier(deps)

	// Should error when voicemail doesn't exist
	err := notifier.UpdateMWIForVoicemail(context.Background(), 99999)
	if err == nil {
		t.Error("Expected error for non-existent voicemail")
	}
}

func TestVoicemailHandler_MarkRead_TriggersMWI(t *testing.T) {
	setup := setupTestAPI(t)
	ctx := context.Background()

	// Create a user (which acts as DID owner)
	user := createTestUser(t, setup.DB, "mwitest@example.com", "password123", "user")

	// Create a voicemail associated with this user
	voicemail := createTestVoicemail(t, setup.DB, user.ID, "+15551234567")

	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewVoicemailHandler(deps)

	// Mark voicemail as read
	req := httptest.NewRequest(http.MethodPut, "/api/voicemails/"+strconv.FormatInt(voicemail.ID, 10)+"/read", nil)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(voicemail.ID, 10)})

	rr := httptest.NewRecorder()
	handler.MarkRead(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify voicemail is marked as read
	updated, err := setup.DB.Voicemails.GetByID(ctx, voicemail.ID)
	if err != nil {
		t.Fatalf("Failed to get voicemail: %v", err)
	}
	if !updated.IsRead {
		t.Error("Expected voicemail to be marked as read")
	}
}

func TestVoicemailHandler_Delete_TriggersMWI(t *testing.T) {
	setup := setupTestAPI(t)
	ctx := context.Background()

	// Create a user (which acts as DID owner)
	user := createTestUser(t, setup.DB, "mwideltest@example.com", "password123", "user")

	// Create a voicemail associated with this user
	voicemail := createTestVoicemail(t, setup.DB, user.ID, "+15559876543")

	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewVoicemailHandler(deps)

	// Delete voicemail
	req := httptest.NewRequest(http.MethodDelete, "/api/voicemails/"+strconv.FormatInt(voicemail.ID, 10), nil)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(voicemail.ID, 10)})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify voicemail is deleted
	_, err := setup.DB.Voicemails.GetByID(ctx, voicemail.ID)
	if err == nil {
		t.Error("Expected voicemail to be deleted")
	}
}

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVoicemailHandler_List(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test user and voicemails
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	createTestVoicemail(t, setup.DB, user.ID, "+15559876543")
	createTestVoicemail(t, setup.DB, user.ID, "+15558888888")

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 2 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 2 voicemails, got %d", total)
	}
}

func TestVoicemailHandler_List_FilterByDID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test users and voicemails
	user1 := createTestUser(t, setup.DB, "user1@example.com", "password", "user")
	user2 := createTestUser(t, setup.DB, "user2@example.com", "password", "user")
	createTestVoicemail(t, setup.DB, user1.ID, "+15559876543")
	createTestVoicemail(t, setup.DB, user2.ID, "+15558888888")

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails?did_id=1", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 voicemail for DID 1, got %d", total)
	}
}

func TestVoicemailHandler_List_FilterByUnread(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test user and voicemails
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	vm1 := createTestVoicemail(t, setup.DB, user.ID, "+15559876543")
	createTestVoicemail(t, setup.DB, user.ID, "+15558888888")

	// Mark one as read
	setup.DB.Voicemails.MarkAsRead(context.Background(), vm1.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails?unread=true", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 unread voicemail, got %d", total)
	}
}

func TestVoicemailHandler_List_Pagination(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test user and multiple voicemails
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	for i := 0; i < 5; i++ {
		createTestVoicemail(t, setup.DB, user.ID, "+15559876543")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails?limit=2&offset=0", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 5 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected total 5, got %d", total)
	}
	if resp.Pagination == nil || resp.Pagination.Limit != 2 {
		limit := 0
		if resp.Pagination != nil {
			limit = resp.Pagination.Limit
		}
		t.Errorf("Expected limit 2, got %d", limit)
	}
}

func TestVoicemailHandler_Get(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test user and voicemail
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	vm := createTestVoicemail(t, setup.DB, user.ID, "+15559876543")

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp VoicemailResponse
	decodeResponse(t, rr, &resp)

	if resp.CallerID != vm.FromNumber {
		t.Errorf("Expected caller_id %s, got %s", vm.FromNumber, resp.CallerID)
	}
	if resp.Duration != vm.Duration {
		t.Errorf("Expected duration %d, got %d", vm.Duration, resp.Duration)
	}
}

func TestVoicemailHandler_Get_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails/9999", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertErrorCode(t, rr, ErrCodeNotFound)
}

func TestVoicemailHandler_Get_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails/invalid", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestVoicemailHandler_MarkRead(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test user and voicemail
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	vm := createTestVoicemail(t, setup.DB, user.ID, "+15559876543")

	if vm.IsRead {
		t.Fatal("Expected voicemail to be unread initially")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/voicemails/1/read", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.MarkRead(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify marked as read
	updatedVM, _ := setup.DB.Voicemails.GetByID(context.Background(), vm.ID)
	if !updatedVM.IsRead {
		t.Error("Expected voicemail to be marked as read")
	}
}

func TestVoicemailHandler_MarkRead_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/voicemails/9999/read", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.MarkRead(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestVoicemailHandler_MarkRead_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/voicemails/invalid/read", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.MarkRead(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestVoicemailHandler_Delete(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test user and voicemail
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	createTestVoicemail(t, setup.DB, user.ID, "+15559876543")

	req := httptest.NewRequest(http.MethodDelete, "/api/voicemails/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify deleted
	_, err := setup.DB.Voicemails.GetByID(context.Background(), 1)
	if err == nil {
		t.Error("Expected voicemail to be deleted")
	}
}

func TestVoicemailHandler_Delete_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/voicemails/invalid", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestVoicemailHandler_GetUnreadCount(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewVoicemailHandler(deps)

	// Create test user and voicemails
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	vm1 := createTestVoicemail(t, setup.DB, user.ID, "+15559876543")
	createTestVoicemail(t, setup.DB, user.ID, "+15558888888")
	createTestVoicemail(t, setup.DB, user.ID, "+15557777777")

	// Mark one as read
	setup.DB.Voicemails.MarkAsRead(context.Background(), vm1.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/voicemails/unread-count", nil)
	rr := httptest.NewRecorder()
	handler.GetUnreadCount(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp map[string]int
	decodeResponse(t, rr, &resp)

	if resp["unread_count"] != 2 {
		t.Errorf("Expected unread count 2, got %d", resp["unread_count"])
	}
}

func TestVoicemailResponse_Format(t *testing.T) {
	setup := setupTestAPI(t)
	user := createTestUser(t, setup.DB, "test@example.com", "password", "user")
	vm := createTestVoicemail(t, setup.DB, user.ID, "+15559876543")

	resp := toVoicemailResponse(vm)

	if resp.ID != vm.ID {
		t.Errorf("Expected ID %d, got %d", vm.ID, resp.ID)
	}
	if resp.CallerID != vm.FromNumber {
		t.Errorf("Expected caller_id %s, got %s", vm.FromNumber, resp.CallerID)
	}
	if resp.Duration != vm.Duration {
		t.Errorf("Expected duration %d, got %d", vm.Duration, resp.Duration)
	}
	if resp.RecordingURL != vm.AudioURL {
		t.Errorf("Expected recording_url %s, got %s", vm.AudioURL, resp.RecordingURL)
	}
	if resp.TranscriptText != vm.Transcript {
		t.Errorf("Expected transcript_text %s, got %s", vm.Transcript, resp.TranscriptText)
	}
	if resp.IsRead != vm.IsRead {
		t.Errorf("Expected is_read %v, got %v", vm.IsRead, resp.IsRead)
	}
}

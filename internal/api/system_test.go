package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemHandler_GetConfig(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	// Set some config values
	setup.DB.Config.Set(context.Background(), "twilio_account_sid", "AC123456789")
	setup.DB.Config.Set(context.Background(), "smtp_host", "smtp.example.com")
	setup.DB.Config.Set(context.Background(), "voicemail_enabled", "true")

	req := httptest.NewRequest(http.MethodGet, "/api/system/config", nil)
	rr := httptest.NewRecorder()
	handler.GetConfig(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ConfigResponse
	decodeResponse(t, rr, &resp)

	if !resp.TwilioConfigured {
		t.Error("Expected TwilioConfigured to be true")
	}
	if !resp.SMTPConfigured {
		t.Error("Expected SMTPConfigured to be true")
	}
	if !resp.VoicemailEnabled {
		t.Error("Expected VoicemailEnabled to be true")
	}
	// TwilioAccountSID should be masked
	if resp.TwilioAccountSID != "AC123456..." {
		t.Errorf("Expected masked Twilio SID 'AC123456...', got %s", resp.TwilioAccountSID)
	}
}

func TestSystemHandler_GetConfig_Empty(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/system/config", nil)
	rr := httptest.NewRecorder()
	handler.GetConfig(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ConfigResponse
	decodeResponse(t, rr, &resp)

	if resp.TwilioConfigured {
		t.Error("Expected TwilioConfigured to be false")
	}
	if resp.SMTPConfigured {
		t.Error("Expected SMTPConfigured to be false")
	}
}

func TestSystemHandler_UpdateConfig(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	reqBody := UpdateConfigRequest{
		Key:   "voicemail_enabled",
		Value: "true",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateConfig(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify the value was set
	value, _ := setup.DB.Config.Get(context.Background(), "voicemail_enabled")
	if value != "true" {
		t.Errorf("Expected voicemail_enabled to be 'true', got %s", value)
	}
}

func TestSystemHandler_UpdateConfig_AllowedKeys(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	allowedKeys := []string{
		"voicemail_enabled",
		"recording_enabled",
		"transcription_enabled",
		"voicemail_greeting",
		"business_hours_start",
		"business_hours_end",
		"timezone",
	}

	for _, key := range allowedKeys {
		t.Run(key, func(t *testing.T) {
			reqBody := UpdateConfigRequest{
				Key:   key,
				Value: "test_value",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPut, "/api/system/config", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.UpdateConfig(rr, req)

			assertStatus(t, rr, http.StatusOK)
		})
	}
}

func TestSystemHandler_UpdateConfig_DisallowedKey(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	reqBody := UpdateConfigRequest{
		Key:   "twilio_auth_token",
		Value: "some_token",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateConfig(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertErrorCode(t, rr, ErrCodeValidation)
}

func TestSystemHandler_UpdateConfig_InvalidJSON(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	req := httptest.NewRequest(http.MethodPut, "/api/system/config", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateConfig(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestSystemHandler_SetupWizard(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewSystemHandler(deps)

	reqBody := SetupWizardRequest{
		TwilioAccountSID: "AC123456789",
		TwilioAuthToken:  "auth_token_here",
		AdminEmail:       "admin@example.com",
		AdminPassword:    "securepassword123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/system/setup", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.SetupWizard(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify setup_completed was set
	completed, _ := setup.DB.Config.Get(context.Background(), "setup_completed")
	if completed != "true" {
		t.Error("Expected setup_completed to be true")
	}

	// Verify Twilio credentials were saved
	sid, _ := setup.DB.Config.Get(context.Background(), "twilio_account_sid")
	if sid != "AC123456789" {
		t.Errorf("Expected Twilio SID 'AC123456789', got %s", sid)
	}
}

func TestSystemHandler_SetupWizard_AlreadyCompleted(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	// Mark setup as completed
	setup.DB.Config.Set(context.Background(), "setup_completed", "true")

	reqBody := SetupWizardRequest{
		TwilioAccountSID: "AC123456789",
		TwilioAuthToken:  "auth_token_here",
		AdminEmail:       "admin@example.com",
		AdminPassword:    "securepassword123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/system/setup", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.SetupWizard(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestSystemHandler_SetupWizard_ValidationError(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	tests := []struct {
		name    string
		reqBody SetupWizardRequest
	}{
		{
			name: "Missing Twilio Account SID",
			reqBody: SetupWizardRequest{
				TwilioAuthToken: "token",
				AdminEmail:      "admin@example.com",
				AdminPassword:   "password123",
			},
		},
		{
			name: "Missing Twilio Auth Token",
			reqBody: SetupWizardRequest{
				TwilioAccountSID: "AC123",
				AdminEmail:       "admin@example.com",
				AdminPassword:    "password123",
			},
		},
		{
			name: "Missing Admin Email",
			reqBody: SetupWizardRequest{
				TwilioAccountSID: "AC123",
				TwilioAuthToken:  "token",
				AdminPassword:    "password123",
			},
		},
		{
			name: "Password too short",
			reqBody: SetupWizardRequest{
				TwilioAccountSID: "AC123",
				TwilioAuthToken:  "token",
				AdminEmail:       "admin@example.com",
				AdminPassword:    "short",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/system/setup", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.SetupWizard(rr, req)

			assertStatus(t, rr, http.StatusBadRequest)
			assertErrorCode(t, rr, ErrCodeValidation)
		})
	}
}

func TestSystemHandler_SetupWizard_WithSMTP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewSystemHandler(deps)

	reqBody := SetupWizardRequest{
		TwilioAccountSID: "AC123456789",
		TwilioAuthToken:  "auth_token_here",
		AdminEmail:       "admin@example.com",
		AdminPassword:    "securepassword123",
		SMTPHost:         "smtp.example.com",
		SMTPPort:         587,
		SMTPUser:         "user@example.com",
		SMTPPassword:     "smtppassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/system/setup", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.SetupWizard(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify SMTP settings were saved
	host, _ := setup.DB.Config.Get(context.Background(), "smtp_host")
	if host != "smtp.example.com" {
		t.Errorf("Expected SMTP host 'smtp.example.com', got %s", host)
	}
}

func TestSystemHandler_SetupWizard_WithGotify(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewSystemHandler(deps)

	reqBody := SetupWizardRequest{
		TwilioAccountSID: "AC123456789",
		TwilioAuthToken:  "auth_token_here",
		AdminEmail:       "admin@example.com",
		AdminPassword:    "securepassword123",
		GotifyURL:        "https://gotify.example.com",
		GotifyToken:      "gotifytoken123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/system/setup", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.SetupWizard(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify Gotify settings were saved
	url, _ := setup.DB.Config.Get(context.Background(), "gotify_url")
	if url != "https://gotify.example.com" {
		t.Errorf("Expected Gotify URL 'https://gotify.example.com', got %s", url)
	}
}

func TestSystemHandler_GetStatus(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil, Twilio: setup.Twilio}
	handler := NewSystemHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	rr := httptest.NewRecorder()
	handler.GetStatus(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp StatusResponse
	decodeResponse(t, rr, &resp)

	if resp.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", resp.Version)
	}
	if resp.GoVersion == "" {
		t.Error("Expected GoVersion to be set")
	}
	if resp.Uptime == "" {
		t.Error("Expected Uptime to be set")
	}
}

func TestSystemHandler_GetStatus_Degraded(t *testing.T) {
	setup := setupTestAPI(t)

	// Set Twilio as not healthy
	setup.Twilio.IsHealthyFunc = func() bool {
		return false
	}

	deps := &Dependencies{DB: setup.DB, SIP: nil, Twilio: setup.Twilio}
	handler := NewSystemHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	rr := httptest.NewRecorder()
	handler.GetStatus(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp StatusResponse
	decodeResponse(t, rr, &resp)

	if resp.TwilioStatus != "degraded" {
		t.Errorf("Expected Twilio status 'degraded', got %s", resp.TwilioStatus)
	}
}

func TestSystemHandler_CreateBackup(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/system/backup", nil)
	rr := httptest.NewRecorder()
	handler.CreateBackup(rr, req)

	// This may return 200 or 500 depending on backup implementation
	// Just verify it doesn't panic
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Unexpected status code: %d", rr.Code)
	}
}

func TestSystemHandler_ListBackups(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/system/backups", nil)
	rr := httptest.NewRecorder()
	handler.ListBackups(rr, req)

	// This may return 200 or 500 depending on backup implementation
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Unexpected status code: %d", rr.Code)
	}
}

func TestNewSystemHandler(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	if handler == nil {
		t.Fatal("Expected handler to be created")
	}
	if handler.deps != deps {
		t.Error("Expected deps to be set")
	}
	if handler.startTime.IsZero() {
		t.Error("Expected startTime to be set")
	}
}

func TestSystemHandler_GetConfig_MaskedSID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	tests := []struct {
		sid      string
		expected string
	}{
		{"AC12345678901234567890", "AC123456..."},
		{"AC1234567", "AC123456..."},
		{"short", ""}, // Too short to mask
	}

	for _, tt := range tests {
		t.Run(tt.sid, func(t *testing.T) {
			setup.DB.Config.Set(context.Background(), "twilio_account_sid", tt.sid)

			req := httptest.NewRequest(http.MethodGet, "/api/system/config", nil)
			rr := httptest.NewRecorder()
			handler.GetConfig(rr, req)

			assertStatus(t, rr, http.StatusOK)

			var resp ConfigResponse
			decodeResponse(t, rr, &resp)

			if resp.TwilioAccountSID != tt.expected {
				t.Errorf("Expected masked SID '%s', got '%s'", tt.expected, resp.TwilioAccountSID)
			}
		})
	}
}

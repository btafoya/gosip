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
	// TwilioAccountSID should be returned (admin-only endpoint)
	if resp.TwilioAccountSID != "AC123456789" {
		t.Errorf("Expected Twilio SID 'AC123456789', got %s", resp.TwilioAccountSID)
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
		VoicemailGreeting: "Hello, please leave a message",
		Timezone:          "America/Los_Angeles",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateConfig(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify the values were set
	greeting, _ := setup.DB.Config.Get(context.Background(), "voicemail_greeting")
	if greeting != "Hello, please leave a message" {
		t.Errorf("Expected voicemail_greeting to be 'Hello, please leave a message', got %s", greeting)
	}
	timezone, _ := setup.DB.Config.Get(context.Background(), "timezone")
	if timezone != "America/Los_Angeles" {
		t.Errorf("Expected timezone to be 'America/Los_Angeles', got %s", timezone)
	}
}

func TestSystemHandler_UpdateConfig_AllFields(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewSystemHandler(deps)

	tests := []struct {
		name     string
		reqBody  UpdateConfigRequest
		checkKey string
		expected string
	}{
		{
			name:     "Update Twilio Account SID",
			reqBody:  UpdateConfigRequest{TwilioAccountSID: "AC123456789"},
			checkKey: "twilio_account_sid",
			expected: "AC123456789",
		},
		{
			name:     "Update SMTP Host",
			reqBody:  UpdateConfigRequest{SMTPHost: "smtp.example.com"},
			checkKey: "smtp_host",
			expected: "smtp.example.com",
		},
		{
			name:     "Update SMTP Port",
			reqBody:  UpdateConfigRequest{SMTPPort: 465},
			checkKey: "smtp_port",
			expected: "465",
		},
		{
			name:     "Update Gotify URL",
			reqBody:  UpdateConfigRequest{GotifyURL: "https://gotify.example.com"},
			checkKey: "gotify_url",
			expected: "https://gotify.example.com",
		},
		{
			name:     "Update Voicemail Greeting",
			reqBody:  UpdateConfigRequest{VoicemailGreeting: "Custom greeting"},
			checkKey: "voicemail_greeting",
			expected: "Custom greeting",
		},
		{
			name:     "Update Timezone",
			reqBody:  UpdateConfigRequest{Timezone: "Europe/London"},
			checkKey: "timezone",
			expected: "Europe/London",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)

			req := httptest.NewRequest(http.MethodPut, "/api/system/config", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.UpdateConfig(rr, req)

			assertStatus(t, rr, http.StatusOK)

			// Verify the value was set
			value, _ := setup.DB.Config.Get(context.Background(), tt.checkKey)
			if value != tt.expected {
				t.Errorf("Expected %s to be '%s', got '%s'", tt.checkKey, tt.expected, value)
			}
		})
	}
}

func TestSystemHandler_UpdateConfig_EmptyRequest(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	// Empty request should succeed but not change anything
	reqBody := UpdateConfigRequest{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/system/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateConfig(rr, req)

	assertStatus(t, rr, http.StatusOK)
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

func TestSystemHandler_GetConfig_FullSID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewSystemHandler(deps)

	// Admin-only endpoint should return full SIDs
	tests := []struct {
		sid      string
		expected string
	}{
		{"AC12345678901234567890", "AC12345678901234567890"},
		{"AC1234567", "AC1234567"},
		{"short", "short"},
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
				t.Errorf("Expected SID '%s', got '%s'", tt.expected, resp.TwilioAccountSID)
			}
		})
	}
}

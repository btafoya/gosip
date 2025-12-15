package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/pkg/sip"
)

// setupTLSTestAPI creates a test environment for TLS testing
func setupTLSTestAPI(t *testing.T) *db.DB {
	t.Helper()

	// Create in-memory database
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations to create tables
	if err := database.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

// createTLSTestDependencies creates Dependencies without SIP server (nil)
func createTLSTestDependencies(database *db.DB) *Dependencies {
	return &Dependencies{
		DB:  database,
		SIP: nil, // SIP server is nil in tests
	}
}

func TestTLSHandler_GetStatus_NoSIPServer(t *testing.T) {
	database := setupTLSTestAPI(t)
	ctx := context.Background()

	// Set initial config values
	database.Config.Set(ctx, db.ConfigKeyTLSEnabled, "true")
	database.Config.Set(ctx, db.ConfigKeyTLSCertMode, "acme")
	database.Config.Set(ctx, db.ConfigKeyACMEDomain, "sip.example.com")
	database.Config.Set(ctx, db.ConfigKeyACMEDomains, "sip2.example.com,sip3.example.com")
	database.Config.Set(ctx, db.ConfigKeyTLSPort, "5061")
	database.Config.Set(ctx, db.ConfigKeyTLSWSSPort, "5081")

	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tls/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response TLSStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response fields from database config
	if !response.Enabled {
		t.Error("Expected TLS to be enabled")
	}
	if response.CertMode != "acme" {
		t.Errorf("Expected cert mode 'acme', got '%s'", response.CertMode)
	}
	if response.Domain != "sip.example.com" {
		t.Errorf("Expected domain 'sip.example.com', got '%s'", response.Domain)
	}
}

func TestTLSHandler_GetStatus_Disabled(t *testing.T) {
	database := setupTLSTestAPI(t)
	ctx := context.Background()

	// TLS disabled
	database.Config.Set(ctx, db.ConfigKeyTLSEnabled, "false")

	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tls/status", nil)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response TLSStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Enabled {
		t.Error("Expected TLS to be disabled")
	}
}

func TestTLSHandler_UpdateConfig_Valid(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	updateReq := TLSConfigRequest{
		Enabled:    boolPtr(true),
		CertMode:   "manual",
		CertFile:   "/etc/ssl/cert.pem",
		KeyFile:    "/etc/ssl/key.pem",
		Port:       intPtr(5061),
		WSSPort:    intPtr(5081),
		MinVersion: "1.2",
		ClientAuth: "none",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tls/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTLSHandler_UpdateConfig_InvalidCertMode(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	updateReq := TLSConfigRequest{
		CertMode: "invalid_mode",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tls/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid cert mode, got %d", rec.Code)
	}
}

func TestTLSHandler_UpdateConfig_InvalidACMECA(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	updateReq := TLSConfigRequest{
		CertMode: "acme",
		ACMECA:   "invalid_ca",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tls/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid ACME CA, got %d", rec.Code)
	}
}

func TestTLSHandler_UpdateConfig_InvalidTLSVersion(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	updateReq := TLSConfigRequest{
		MinVersion: "1.0",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tls/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid TLS version, got %d", rec.Code)
	}
}

func TestTLSHandler_UpdateConfig_InvalidClientAuth(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	updateReq := TLSConfigRequest{
		ClientAuth: "invalid_auth",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tls/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid client auth, got %d", rec.Code)
	}
}

func TestTLSHandler_GetSRTPStatus(t *testing.T) {
	database := setupTLSTestAPI(t)
	ctx := context.Background()

	// Set SRTP config
	database.Config.Set(ctx, db.ConfigKeySRTPEnabled, "true")
	database.Config.Set(ctx, db.ConfigKeySRTPProfile, string(sip.SRTPProfileAES128CMHMACSHA180))

	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tls/srtp", nil)
	rec := httptest.NewRecorder()

	handler.GetSRTPStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response SRTPStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Enabled {
		t.Error("Expected SRTP to be enabled")
	}
	if response.Profile != string(sip.SRTPProfileAES128CMHMACSHA180) {
		t.Errorf("Expected profile AES_CM_128_HMAC_SHA1_80, got %s", response.Profile)
	}
}

func TestTLSHandler_UpdateSRTPConfig(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	updateReq := SRTPConfigRequest{
		Enabled: boolPtr(true),
		Profile: string(sip.SRTPProfileAEADAES128GCM),
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tls/srtp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateSRTPConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTLSHandler_UpdateSRTPConfig_InvalidProfile(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	updateReq := SRTPConfigRequest{
		Enabled: boolPtr(true),
		Profile: "INVALID_PROFILE",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tls/srtp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateSRTPConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid SRTP profile, got %d", rec.Code)
	}
}

func TestTLSHandler_ForceRenewal_NoSIPServer(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/tls/renew", nil)
	rec := httptest.NewRecorder()

	handler.ForceRenewal(rec, req)

	// Should fail because SIP server is not running
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when SIP server not running, got %d", rec.Code)
	}
}

func TestTLSHandler_ReloadCertificates_NoSIPServer(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/tls/reload", nil)
	rec := httptest.NewRecorder()

	handler.ReloadCertificates(rec, req)

	// Should fail because SIP server is not running
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when SIP server not running, got %d", rec.Code)
	}
}

func TestTLSHandler_GetCertificateInfo_NoSIPServer(t *testing.T) {
	database := setupTLSTestAPI(t)
	deps := createTLSTestDependencies(database)
	handler := NewTLSHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/tls/certificate", nil)
	rec := httptest.NewRecorder()

	handler.GetCertificateInfo(rec, req)

	// Should fail because SIP server is not running
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when SIP server not running, got %d", rec.Code)
	}
}

// Helper functions for creating pointers
func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

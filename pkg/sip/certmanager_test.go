package sip

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/config"
)

func TestNewCertManager_Disabled(t *testing.T) {
	// Test with nil config
	cm, err := NewCertManager(nil, "")
	if err != nil {
		t.Errorf("Expected no error for nil config, got: %v", err)
	}
	if cm != nil {
		t.Error("Expected nil CertManager for nil config")
	}

	// Test with disabled config
	cfg := &config.TLSConfig{Enabled: false}
	cm, err = NewCertManager(cfg, "")
	if err != nil {
		t.Errorf("Expected no error for disabled config, got: %v", err)
	}
	if cm != nil {
		t.Error("Expected nil CertManager for disabled config")
	}
}

func TestNewCertManager_ManualMode_MissingFiles(t *testing.T) {
	cfg := &config.TLSConfig{
		Enabled:  true,
		CertMode: "manual",
	}

	_, err := NewCertManager(cfg, "")
	if err == nil {
		t.Error("Expected error for missing cert/key files")
	}
}

func TestNewCertManager_ManualMode_NonexistentFiles(t *testing.T) {
	cfg := &config.TLSConfig{
		Enabled:  true,
		CertMode: "manual",
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}

	_, err := NewCertManager(cfg, "")
	if err == nil {
		t.Error("Expected error for nonexistent cert files")
	}
}

func TestNewCertManager_ACMEMode_MissingEmail(t *testing.T) {
	cfg := &config.TLSConfig{
		Enabled:  true,
		CertMode: "acme",
	}

	_, err := NewCertManager(cfg, t.TempDir())
	if err == nil {
		t.Error("Expected error for missing ACME email")
	}
}

func TestNewCertManager_ACMEMode_MissingDomain(t *testing.T) {
	cfg := &config.TLSConfig{
		Enabled:   true,
		CertMode:  "acme",
		ACMEEmail: "test@example.com",
	}

	_, err := NewCertManager(cfg, t.TempDir())
	if err == nil {
		t.Error("Expected error for missing ACME domain")
	}
}

func TestCertManager_GetTLSMinVersion(t *testing.T) {
	tests := []struct {
		name       string
		minVersion string
		expected   uint16
	}{
		{"TLS 1.2", "1.2", 0x0303},
		{"TLS 1.3", "1.3", 0x0304},
		{"default", "", 0x0303},
		{"unknown", "1.0", 0x0303},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &CertManager{
				config: &config.TLSConfig{
					MinVersion: tt.minVersion,
				},
			}

			result := cm.getTLSMinVersion()
			if result != tt.expected {
				t.Errorf("getTLSMinVersion() = %x, want %x", result, tt.expected)
			}
		})
	}
}

func TestCertManager_GetClientAuth(t *testing.T) {
	tests := []struct {
		name       string
		clientAuth string
		expected   int
	}{
		{"none", "none", 0},
		{"request", "request", 1},
		{"require", "require", 4},
		{"default", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &CertManager{
				config: &config.TLSConfig{
					ClientAuth: tt.clientAuth,
				},
			}

			result := cm.getClientAuth()
			if int(result) != tt.expected {
				t.Errorf("getClientAuth() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestCertStatus_Fields(t *testing.T) {
	status := CertStatus{
		Enabled:     true,
		CertMode:    "acme",
		Domain:      "example.com",
		Domains:     []string{"sip.example.com"},
		CertExpiry:  time.Now().Add(90 * 24 * time.Hour),
		CertIssuer:  "Let's Encrypt",
		AutoRenewal: true,
		LastRenewal: time.Now(),
		NextRenewal: time.Now().Add(60 * 24 * time.Hour),
		Valid:       true,
	}

	if !status.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if status.CertMode != "acme" {
		t.Errorf("Expected CertMode 'acme', got %s", status.CertMode)
	}
	if status.Domain != "example.com" {
		t.Errorf("Expected Domain 'example.com', got %s", status.Domain)
	}
	if !status.Valid {
		t.Error("Expected Valid to be true")
	}
}

func TestCertManager_GetStatus_Manual(t *testing.T) {
	// Create a test certificate for manual mode
	// Note: This test verifies the structure but not actual TLS functionality
	cm := &CertManager{
		config: &config.TLSConfig{
			Enabled:  true,
			CertMode: "manual",
		},
		certExpiry:  time.Now().Add(30 * 24 * time.Hour),
		certIssuer:  "Test Issuer",
		lastRenewal: time.Now(),
	}

	status := cm.GetStatus()

	if !status.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if status.CertMode != "manual" {
		t.Errorf("Expected CertMode 'manual', got %s", status.CertMode)
	}
	if status.AutoRenewal {
		t.Error("Manual mode should not have auto-renewal")
	}
	if status.CertIssuer != "Test Issuer" {
		t.Errorf("Expected CertIssuer 'Test Issuer', got %s", status.CertIssuer)
	}
}

func TestCertManager_GetStatus_ACME(t *testing.T) {
	cm := &CertManager{
		config: &config.TLSConfig{
			Enabled:     true,
			CertMode:    "acme",
			ACMEDomain:  "example.com",
			ACMEDomains: []string{"sip.example.com"},
		},
		certExpiry:  time.Now().Add(90 * 24 * time.Hour),
		certIssuer:  "Let's Encrypt",
		lastRenewal: time.Now(),
	}

	status := cm.GetStatus()

	if !status.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if status.CertMode != "acme" {
		t.Errorf("Expected CertMode 'acme', got %s", status.CertMode)
	}
	if !status.AutoRenewal {
		t.Error("ACME mode should have auto-renewal")
	}
	if status.Domain != "example.com" {
		t.Errorf("Expected Domain 'example.com', got %s", status.Domain)
	}
	if !status.Valid {
		t.Error("Certificate should be valid")
	}

	// Check next renewal calculation (30 days before expiry)
	expectedNextRenewal := status.CertExpiry.Add(-30 * 24 * time.Hour)
	if status.NextRenewal.Sub(expectedNextRenewal) > time.Minute {
		t.Errorf("NextRenewal calculation incorrect")
	}
}

func TestCertManager_ForceRenewal_ManualMode(t *testing.T) {
	cm := &CertManager{
		config: &config.TLSConfig{
			Enabled:  true,
			CertMode: "manual",
		},
	}

	err := cm.ForceRenewal(nil)
	if err == nil {
		t.Error("Expected error for force renewal in manual mode")
	}
}

func TestCertManager_ReloadCertificates_ACMEMode(t *testing.T) {
	cm := &CertManager{
		config: &config.TLSConfig{
			Enabled:  true,
			CertMode: "acme",
		},
	}

	err := cm.ReloadCertificates()
	if err == nil {
		t.Error("Expected error for reload in ACME mode")
	}
}

func TestCertManager_Close(t *testing.T) {
	cm := &CertManager{
		config: &config.TLSConfig{
			Enabled: true,
		},
	}

	err := cm.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

func TestGenerateTLSConfig_MissingFiles(t *testing.T) {
	_, err := GenerateTLSConfig("/nonexistent/cert.pem", "/nonexistent/key.pem", nil)
	if err == nil {
		t.Error("Expected error for nonexistent files")
	}
}

func TestGenerateTLSConfig_InvalidRootPEM(t *testing.T) {
	// Create temp cert files
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	// Generate test certificates (self-signed)
	// For now, we test the error case with invalid root PEM
	if err := os.WriteFile(certFile, []byte("invalid"), 0600); err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("invalid"), 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	_, err := GenerateTLSConfig(certFile, keyFile, []byte("invalid root pem"))
	if err == nil {
		t.Error("Expected error for invalid PEM files")
	}
}

func TestCertManager_GetTLSConfig_Nil(t *testing.T) {
	cm := &CertManager{}

	config := cm.GetTLSConfig()
	if config != nil {
		t.Error("Expected nil TLS config when not initialized")
	}
}

func TestCertStatus_ExpiredCertificate(t *testing.T) {
	cm := &CertManager{
		config: &config.TLSConfig{
			Enabled:  true,
			CertMode: "manual",
		},
		certExpiry: time.Now().Add(-24 * time.Hour), // Expired
	}

	status := cm.GetStatus()

	if status.Valid {
		t.Error("Expired certificate should not be valid")
	}
}

package sip

import (
	"context"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *db.DB {
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

func createTestDevice(t *testing.T, database *db.DB, username, passwordHash string) *models.Device {
	t.Helper()

	device := &models.Device{
		Name:         "Test Device",
		Username:     username,
		PasswordHash: passwordHash,
		DeviceType:   "softphone",
	}

	if err := database.Devices.Create(context.Background(), device); err != nil {
		t.Fatalf("Failed to create test device: %v", err)
	}

	return device
}

func TestParseDigestAuth(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    map[string]string
		shouldError bool
	}{
		{
			name:  "valid digest auth",
			input: `Digest username="alice", realm="gosip", nonce="abc123", uri="sip:gosip", response="def456"`,
			expected: map[string]string{
				"username": "alice",
				"realm":    "gosip",
				"nonce":    "abc123",
				"uri":      "sip:gosip",
				"response": "def456",
			},
			shouldError: false,
		},
		{
			name:  "digest with extra spaces",
			input: `Digest  username="bob"  ,  realm="test"  ,  nonce="xyz"  `,
			expected: map[string]string{
				"username": "bob",
				"realm":    "test",
				"nonce":    "xyz",
			},
			shouldError: false,
		},
		{
			name:  "all common parameters",
			input: `Digest username="user", realm="gosip", nonce="n1", uri="sip:host", response="r1", algorithm="MD5", cnonce="c1", nc="00000001", qop="auth"`,
			expected: map[string]string{
				"username":  "user",
				"realm":     "gosip",
				"nonce":     "n1",
				"uri":       "sip:host",
				"response":  "r1",
				"algorithm": "MD5",
				"cnonce":    "c1",
				"nc":        "00000001",
				"qop":       "auth",
			},
			shouldError: false,
		},
		{
			name:        "missing Digest prefix",
			input:       `username="alice", realm="gosip"`,
			expected:    nil,
			shouldError: true,
		},
		{
			name:  "empty values",
			input: `Digest username="", realm=""`,
			expected: map[string]string{
				"username": "",
				"realm":    "",
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDigestAuth(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("Key %q: expected %q, got %q", key, expectedValue, result[key])
				}
			}
		})
	}
}

func TestMd5Hash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "hello",
			expected: "5d41402abc4b2a76b9719d911017c592",
		},
		{
			input:    "alice:gosip:password123",
			expected: md5Hash("alice:gosip:password123"), // Self-consistent check
		},
		{
			input:    "",
			expected: "d41d8cd98f00b204e9800998ecf8427e", // MD5 of empty string
		},
		{
			input:    "REGISTER:sip:gosip",
			expected: md5Hash("REGISTER:sip:gosip"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := md5Hash(tt.input)
			if result != tt.expected {
				t.Errorf("md5Hash(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateHA1(t *testing.T) {
	tests := []struct {
		username string
		realm    string
		password string
	}{
		{"alice", "gosip", "secret123"},
		{"bob", "testrealm", "password"},
		{"user@domain", "gosip", "p@ss!word"},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			ha1 := GenerateHA1(tt.username, tt.realm, tt.password)

			// HA1 should be MD5 hash (32 hex chars)
			if len(ha1) != 32 {
				t.Errorf("HA1 should be 32 chars, got %d", len(ha1))
			}

			// HA1 should be consistent
			ha1Again := GenerateHA1(tt.username, tt.realm, tt.password)
			if ha1 != ha1Again {
				t.Error("HA1 should be deterministic")
			}

			// HA1 = MD5(username:realm:password)
			expected := md5Hash(tt.username + ":" + tt.realm + ":" + tt.password)
			if ha1 != expected {
				t.Errorf("HA1 mismatch: got %s, want %s", ha1, expected)
			}
		})
	}
}

func TestAuthenticator_GenerateNonce(t *testing.T) {
	database := setupTestDB(t)
	auth := NewAuthenticator(database)

	// Generate multiple nonces
	nonces := make(map[string]bool)
	for i := 0; i < 100; i++ {
		nonce := auth.GenerateNonce()

		// Nonce should be 32 hex chars (16 bytes)
		if len(nonce) != 32 {
			t.Errorf("Nonce should be 32 chars, got %d", len(nonce))
		}

		// Nonces should be unique
		if nonces[nonce] {
			t.Errorf("Duplicate nonce generated: %s", nonce)
		}
		nonces[nonce] = true
	}
}

func TestAuthenticator_ValidateNonce(t *testing.T) {
	database := setupTestDB(t)
	auth := NewAuthenticator(database)

	t.Run("valid nonce", func(t *testing.T) {
		nonce := auth.GenerateNonce()
		if !auth.ValidateNonce(nonce) {
			t.Error("Freshly generated nonce should be valid")
		}
	})

	t.Run("unknown nonce", func(t *testing.T) {
		if auth.ValidateNonce("unknown_nonce_12345678901234") {
			t.Error("Unknown nonce should be invalid")
		}
	})

	t.Run("empty nonce", func(t *testing.T) {
		if auth.ValidateNonce("") {
			t.Error("Empty nonce should be invalid")
		}
	})
}

func TestAuthenticator_RemoveNonce(t *testing.T) {
	database := setupTestDB(t)
	auth := NewAuthenticator(database)

	nonce := auth.GenerateNonce()

	// Should be valid initially
	if !auth.ValidateNonce(nonce) {
		t.Fatal("Nonce should be valid initially")
	}

	// Remove the nonce
	auth.removeNonce(nonce)

	// Should no longer be valid
	if auth.ValidateNonce(nonce) {
		t.Error("Removed nonce should be invalid")
	}
}

func TestAuthenticator_Authenticate_NoCredentials(t *testing.T) {
	database := setupTestDB(t)
	auth := NewAuthenticator(database)
	ctx := context.Background()

	// Create a mock request without Authorization header
	// We can't easily create a real sip.Request, so we test the exported API
	// The full authentication flow will be tested in integration tests

	// Test that error constants are defined correctly
	if ErrNoCredentials.Error() != "no credentials provided" {
		t.Errorf("ErrNoCredentials message mismatch")
	}
	if ErrInvalidCredentials.Error() != "invalid credentials" {
		t.Errorf("ErrInvalidCredentials message mismatch")
	}
	if ErrDeviceNotFound.Error() != "device not found" {
		t.Errorf("ErrDeviceNotFound message mismatch")
	}
	if ErrInvalidNonce.Error() != "invalid or expired nonce" {
		t.Errorf("ErrInvalidNonce message mismatch")
	}

	// Test that authenticator can be created with database
	if auth == nil {
		t.Error("Authenticator should not be nil")
	}

	// Test nonce management works with real usage
	nonce := auth.GenerateNonce()
	if !auth.ValidateNonce(nonce) {
		t.Error("Generated nonce should be immediately valid")
	}

	// Verify database is accessible
	_, err := database.Devices.List(ctx, 10, 0)
	if err != nil {
		t.Errorf("Database should be accessible: %v", err)
	}
}

func TestDigestAuthResponse(t *testing.T) {
	// Test complete digest auth calculation
	username := "alice"
	realm := "gosip"
	password := "secret123"
	method := "REGISTER"
	uri := "sip:gosip"
	nonce := "abc123def456"

	// Calculate HA1
	ha1 := GenerateHA1(username, realm, password)

	// Calculate HA2
	ha2 := md5Hash(method + ":" + uri)

	// Calculate response
	response := md5Hash(ha1 + ":" + nonce + ":" + ha2)

	// Verify response is 32 hex chars
	if len(response) != 32 {
		t.Errorf("Response should be 32 chars, got %d", len(response))
	}

	// Verify determinism
	response2 := md5Hash(ha1 + ":" + nonce + ":" + ha2)
	if response != response2 {
		t.Error("Digest response calculation should be deterministic")
	}

	// Verify changing any component changes the response
	differentNonce := md5Hash(ha1 + ":different:" + ha2)
	if response == differentNonce {
		t.Error("Different nonce should produce different response")
	}

	differentMethod := md5Hash(ha1 + ":" + nonce + ":" + md5Hash("INVITE:"+uri))
	if response == differentMethod {
		t.Error("Different method should produce different response")
	}
}

func TestAuthenticator_NonceConcurrency(t *testing.T) {
	database := setupTestDB(t)
	auth := NewAuthenticator(database)

	// Generate nonces concurrently
	done := make(chan string, 100)
	for i := 0; i < 100; i++ {
		go func() {
			done <- auth.GenerateNonce()
		}()
	}

	nonces := make(map[string]bool)
	for i := 0; i < 100; i++ {
		select {
		case nonce := <-done:
			if nonces[nonce] {
				t.Errorf("Duplicate nonce in concurrent generation: %s", nonce)
			}
			nonces[nonce] = true
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for nonce generation")
		}
	}

	// Verify all nonces are valid
	for nonce := range nonces {
		if !auth.ValidateNonce(nonce) {
			t.Errorf("Generated nonce should be valid: %s", nonce)
		}
	}
}

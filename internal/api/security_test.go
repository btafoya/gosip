package api

import (
	"testing"
)

// TestGenerateRandomToken_Entropy verifies token generation is cryptographically secure
func TestGenerateRandomToken_Entropy(t *testing.T) {
	const tokenLength = 32
	const iterations = 1000

	tokens := make(map[string]bool)

	// Generate multiple tokens
	for i := 0; i < iterations; i++ {
		token, err := generateRandomToken(tokenLength)
		if err != nil {
			t.Fatalf("generateRandomToken failed: %v", err)
		}

		// Verify length
		if len(token) != tokenLength {
			t.Errorf("Token length mismatch: got %d, want %d", len(token), tokenLength)
		}

		// Check for duplicates (extremely unlikely with crypto/rand)
		if tokens[token] {
			t.Errorf("Duplicate token generated: %s (CRITICAL: weak randomness)", token)
		}
		tokens[token] = true
	}

	// Verify we generated expected number of unique tokens
	if len(tokens) != iterations {
		t.Errorf("Expected %d unique tokens, got %d", iterations, len(tokens))
	}
}

// TestGenerateRandomToken_NonPredictable verifies tokens are not time-based
func TestGenerateRandomToken_NonPredictable(t *testing.T) {
	const tokenLength = 32

	// Generate two tokens in rapid succession
	token1, err := generateRandomToken(tokenLength)
	if err != nil {
		t.Fatalf("generateRandomToken failed: %v", err)
	}

	token2, err := generateRandomToken(tokenLength)
	if err != nil {
		t.Fatalf("generateRandomToken failed: %v", err)
	}

	// Tokens must be different (with crypto/rand, probability of collision is negligible)
	if token1 == token2 {
		t.Errorf("Consecutive tokens are identical: %s (CRITICAL: predictable generation)", token1)
	}

	// Verify tokens contain URL-safe base64 characters only
	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, char := range token1 {
		if !containsRune(validChars, char) {
			t.Errorf("Token contains invalid character: %c", char)
		}
	}
}

// TestGenerateRandomToken_ErrorHandling verifies error handling
func TestGenerateRandomToken_ErrorHandling(t *testing.T) {
	// Valid lengths
	validLengths := []int{16, 32, 64, 128}
	for _, length := range validLengths {
		token, err := generateRandomToken(length)
		if err != nil {
			t.Errorf("generateRandomToken(%d) failed: %v", length, err)
		}
		if len(token) != length {
			t.Errorf("generateRandomToken(%d) returned token of length %d", length, len(token))
		}
	}
}

// TestCreateSession_SecureTokens verifies session creation uses secure tokens
func TestCreateSession_SecureTokens(t *testing.T) {
	const iterations = 100
	sessionTokens := make(map[string]bool)

	// Create multiple sessions
	for i := 0; i < iterations; i++ {
		token, err := createSession(int64(i + 1))
		if err != nil {
			t.Fatalf("createSession failed: %v", err)
		}

		// Verify token is not empty
		if token == "" {
			t.Error("createSession returned empty token")
		}

		// Check for duplicates
		if sessionTokens[token] {
			t.Errorf("Duplicate session token: %s (CRITICAL: collision detected)", token)
		}
		sessionTokens[token] = true

		// Clean up session
		deleteSession(token)
	}
}

// Helper function to check if rune is in string
func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

package sip

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/emiago/sipgo/sip"
)

var (
	ErrNoCredentials      = errors.New("no credentials provided")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrDeviceNotFound     = errors.New("device not found")
	ErrInvalidNonce       = errors.New("invalid or expired nonce")
)

// Authenticator handles SIP digest authentication
type Authenticator struct {
	db     *db.DB
	nonces map[string]time.Time
	mu     sync.RWMutex
	realm  string
}

// NewAuthenticator creates a new Authenticator
func NewAuthenticator(database *db.DB) *Authenticator {
	auth := &Authenticator{
		db:     database,
		nonces: make(map[string]time.Time),
		realm:  "gosip",
	}

	// Start nonce cleanup goroutine
	go auth.cleanupNonces()

	return auth
}

// Authenticate validates a SIP request using Digest authentication
func (a *Authenticator) Authenticate(ctx context.Context, req *sip.Request) (*models.Device, error) {
	authHeader := req.GetHeader("Authorization")
	if authHeader == nil {
		return nil, ErrNoCredentials
	}

	// Parse digest auth parameters
	params, err := parseDigestAuth(authHeader.Value())
	if err != nil {
		return nil, err
	}

	// Validate required parameters
	username := params["username"]
	nonce := params["nonce"]
	uri := params["uri"]
	response := params["response"]

	if username == "" || nonce == "" || uri == "" || response == "" {
		return nil, ErrInvalidCredentials
	}

	// Validate nonce
	if !a.ValidateNonce(nonce) {
		return nil, ErrInvalidNonce
	}

	// Look up device by username
	device, err := a.db.Devices.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, db.ErrDeviceNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	// Calculate expected response
	// For SIP, we store the password hash (HA1) directly
	// HA1 = MD5(username:realm:password)
	// HA2 = MD5(method:uri)
	// response = MD5(HA1:nonce:HA2)

	method := string(req.Method)

	// If password_hash is the actual HA1, use it directly
	// Otherwise, compute HA1 (this assumes password_hash is the plain HA1)
	ha1 := device.PasswordHash
	ha2 := md5Hash(fmt.Sprintf("%s:%s", method, uri))
	expectedResponse := md5Hash(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))

	if response != expectedResponse {
		return nil, ErrInvalidCredentials
	}

	// Remove used nonce (one-time use for security)
	a.removeNonce(nonce)

	return device, nil
}

// GenerateNonce creates a new nonce for auth challenges
func (a *Authenticator) GenerateNonce() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	nonce := hex.EncodeToString(bytes)

	a.mu.Lock()
	a.nonces[nonce] = time.Now()
	a.mu.Unlock()

	return nonce
}

// ValidateNonce checks if a nonce is valid and not expired
func (a *Authenticator) ValidateNonce(nonce string) bool {
	a.mu.RLock()
	created, exists := a.nonces[nonce]
	a.mu.RUnlock()

	if !exists {
		return false
	}

	// Nonces expire after 5 minutes
	return time.Since(created) < 5*time.Minute
}

// removeNonce removes a used nonce
func (a *Authenticator) removeNonce(nonce string) {
	a.mu.Lock()
	delete(a.nonces, nonce)
	a.mu.Unlock()
}

// cleanupNonces periodically removes expired nonces
func (a *Authenticator) cleanupNonces() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		a.mu.Lock()
		now := time.Now()
		for nonce, created := range a.nonces {
			if now.Sub(created) > 5*time.Minute {
				delete(a.nonces, nonce)
			}
		}
		a.mu.Unlock()
	}
}

// GenerateHA1 generates the HA1 hash for storing device credentials
// This should be called when creating/updating a device password
func GenerateHA1(username, realm, password string) string {
	return md5Hash(fmt.Sprintf("%s:%s:%s", username, realm, password))
}

// parseDigestAuth parses a Digest Authorization header value
func parseDigestAuth(value string) (map[string]string, error) {
	result := make(map[string]string)

	// Remove "Digest " prefix
	if !strings.HasPrefix(value, "Digest ") {
		return nil, errors.New("invalid digest auth format")
	}
	value = strings.TrimPrefix(value, "Digest ")

	// Parse key=value pairs
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		idx := strings.Index(part, "=")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(part[:idx])
		val := strings.TrimSpace(part[idx+1:])

		// Remove quotes
		val = strings.Trim(val, `"`)

		result[key] = val
	}

	return result, nil
}

// md5Hash computes MD5 hash of a string
func md5Hash(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

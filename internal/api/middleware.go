package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
)

// Context keys
type contextKey string

const (
	contextKeyUser contextKey = "user"
)

// AuthMiddleware validates session tokens
func AuthMiddleware(deps *Dependencies) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session token from cookie or Authorization header
			var token string

			// Check cookie first
			if cookie, err := r.Cookie("session"); err == nil {
				token = cookie.Value
			}

			// Check Authorization header as fallback
			if token == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if token == "" {
				WriteError(w, http.StatusUnauthorized, ErrCodeAuthentication, "Authentication required", nil)
				return
			}

			// Validate session token
			user, err := validateSession(r.Context(), deps.DB, token)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, ErrCodeAuthentication, "Invalid or expired session", nil)
				return
			}

			// Add user to context
			ctx := context.WithValue(r.Context(), contextKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnlyMiddleware restricts access to admin users
func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			WriteError(w, http.StatusForbidden, ErrCodeAuthorization, "Admin access required", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SetupOnlyMiddleware allows access only when setup is not complete
func SetupOnlyMiddleware(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if database.Config.IsSetupComplete(r.Context()) {
				WriteError(w, http.StatusForbidden, ErrCodeAuthorization, "Setup already complete", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromContext retrieves the authenticated user from context
func GetUserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(contextKeyUser).(*models.User)
	if !ok {
		return nil
	}
	return user
}

// getUserIDFromContext retrieves the authenticated user's ID from context
func getUserIDFromContext(ctx context.Context) int64 {
	user := GetUserFromContext(ctx)
	if user == nil {
		return 0
	}
	return user.ID
}

// Session management with persistent database storage
// Includes in-memory cache for performance with database persistence

// SessionDuration defines how long sessions are valid
const SessionDuration = 24 * time.Hour

// sessionCache provides fast lookup for recently accessed sessions
// This is a performance optimization - the database is the source of truth
type sessionCache struct {
	mu       sync.RWMutex
	sessions map[string]*cachedSession
}

type cachedSession struct {
	UserID    int64
	ExpiresAt time.Time
	CachedAt  time.Time
}

var cache = &sessionCache{
	sessions: make(map[string]*cachedSession),
}

const cacheExpiry = 5 * time.Minute // Cache entries expire after 5 minutes

// createSession creates a new persistent session for a user
func createSession(userID int64) (string, error) {
	return createSessionWithRequest(context.Background(), nil, userID, "", "")
}

// createSessionWithRequest creates a new persistent session with request metadata
func createSessionWithRequest(ctx context.Context, database *db.DB, userID int64, userAgent, ipAddress string) (string, error) {
	// Generate cryptographically secure random token
	token, err := generateRandomToken(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	expiresAt := time.Now().Add(SessionDuration)

	// Store in database if available
	if database != nil && database.Sessions != nil {
		_, err = database.Sessions.Create(ctx, token, userID, expiresAt, userAgent, ipAddress)
		if err != nil {
			return "", fmt.Errorf("failed to persist session: %w", err)
		}
	}

	// Also cache locally for fast lookup
	cache.mu.Lock()
	cache.sessions[token] = &cachedSession{
		UserID:    userID,
		ExpiresAt: expiresAt,
		CachedAt:  time.Now(),
	}
	cache.mu.Unlock()

	return token, nil
}

// validateSession checks if a session token is valid
func validateSession(ctx context.Context, database *db.DB, token string) (*models.User, error) {
	// First check the cache for fast lookup
	cache.mu.RLock()
	cached, exists := cache.sessions[token]
	cache.mu.RUnlock()

	if exists {
		// Check if cache entry is still valid
		if time.Since(cached.CachedAt) < cacheExpiry && time.Now().Before(cached.ExpiresAt) {
			// Refresh session expiry (sliding window)
			newExpiry := time.Now().Add(SessionDuration)

			// Update database asynchronously
			if database != nil && database.Sessions != nil {
				go func() {
					_ = database.Sessions.UpdateActivity(context.Background(), token, newExpiry)
				}()
			}

			// Update cache
			cache.mu.Lock()
			cached.ExpiresAt = newExpiry
			cached.CachedAt = time.Now()
			cache.mu.Unlock()

			return database.Users.GetByID(ctx, cached.UserID)
		}

		// Cache entry is stale or expired, remove it
		cache.mu.Lock()
		delete(cache.sessions, token)
		cache.mu.Unlock()
	}

	// Cache miss or stale - check database
	if database != nil && database.Sessions != nil {
		session, err := database.Sessions.GetByToken(ctx, token)
		if err != nil {
			return nil, db.ErrUserNotFound
		}

		// Check if session is expired
		if time.Now().After(session.ExpiresAt) {
			// Clean up expired session
			_ = database.Sessions.Delete(ctx, token)
			return nil, db.ErrUserNotFound
		}

		// Refresh session expiry (sliding window)
		newExpiry := time.Now().Add(SessionDuration)
		_ = database.Sessions.UpdateActivity(ctx, token, newExpiry)

		// Update cache
		cache.mu.Lock()
		cache.sessions[token] = &cachedSession{
			UserID:    session.UserID,
			ExpiresAt: newExpiry,
			CachedAt:  time.Now(),
		}
		cache.mu.Unlock()

		return database.Users.GetByID(ctx, session.UserID)
	}

	return nil, db.ErrUserNotFound
}

// deleteSession removes a session from both cache and database
func deleteSession(token string) {
	deleteSessionWithDB(context.Background(), nil, token)
}

// deleteSessionWithDB removes a session from both cache and database
func deleteSessionWithDB(ctx context.Context, database *db.DB, token string) {
	// Remove from cache
	cache.mu.Lock()
	delete(cache.sessions, token)
	cache.mu.Unlock()

	// Remove from database
	if database != nil && database.Sessions != nil {
		_ = database.Sessions.Delete(ctx, token)
	}
}

// cleanupExpiredSessions removes expired sessions from cache
// This should be called periodically
func cleanupExpiredSessions() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	now := time.Now()
	for token, session := range cache.sessions {
		if now.After(session.ExpiresAt) || time.Since(session.CachedAt) > cacheExpiry {
			delete(cache.sessions, token)
		}
	}
}

// generateRandomToken creates a cryptographically secure random string token
// Uses crypto/rand for unpredictable token generation resistant to brute-force attacks
func generateRandomToken(length int) (string, error) {
	// Calculate the number of random bytes needed
	// We'll use base64 URL encoding which is more efficient than charset selection
	// Base64 encoding: 3 bytes → 4 characters, so we need (length * 3 / 4) bytes
	// Add extra to ensure we have enough after encoding
	numBytes := (length * 3 / 4) + 1

	randomBytes := make([]byte, numBytes)

	// Use crypto/rand for cryptographically secure random generation
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}

	// Encode to base64 URL-safe format (no padding)
	token := base64.RawURLEncoding.EncodeToString(randomBytes)

	// Truncate to exact requested length
	if len(token) > length {
		token = token[:length]
	}

	return token, nil
}

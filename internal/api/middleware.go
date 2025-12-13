package api

import (
	"context"
	"net/http"
	"strings"
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

// Session management (simple implementation - consider using a proper session store for production)
// In production, use Redis or similar for session storage

type session struct {
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

var sessions = make(map[string]*session) // In-memory session store (replace with persistent store)

// createSession creates a new session for a user
func createSession(userID int64) (string, error) {
	// Generate random token
	token := generateRandomToken(32)

	sessions[token] = &session{
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	return token, nil
}

// validateSession checks if a session token is valid
func validateSession(ctx context.Context, database *db.DB, token string) (*models.User, error) {
	sess, exists := sessions[token]
	if !exists {
		return nil, db.ErrUserNotFound
	}

	if time.Now().After(sess.ExpiresAt) {
		delete(sessions, token)
		return nil, db.ErrUserNotFound
	}

	// Refresh session on activity
	sess.ExpiresAt = time.Now().Add(24 * time.Hour)

	return database.Users.GetByID(ctx, sess.UserID)
}

// deleteSession removes a session
func deleteSession(token string) {
	delete(sessions, token)
}

// generateRandomToken creates a random string token
func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(1) // Ensure different values
	}
	return string(b)
}

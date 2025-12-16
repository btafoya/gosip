package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

func TestGetUserFromContext(t *testing.T) {
	tests := []struct {
		name     string
		user     *models.User
		wantNil  bool
		wantID   int64
	}{
		{
			name:    "no user in context",
			user:    nil,
			wantNil: true,
		},
		{
			name: "user in context",
			user: &models.User{
				ID:   123,
				Email: "testuser@example.com",
				Role: "user",
			},
			wantNil: false,
			wantID:  123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.user != nil {
				ctx = context.WithValue(ctx, contextKeyUser, tt.user)
			}

			got := GetUserFromContext(ctx)
			if tt.wantNil && got != nil {
				t.Errorf("GetUserFromContext() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("GetUserFromContext() = nil, want user")
			}
			if !tt.wantNil && got.ID != tt.wantID {
				t.Errorf("GetUserFromContext().ID = %v, want %v", got.ID, tt.wantID)
			}
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name   string
		user   *models.User
		wantID int64
	}{
		{
			name:   "no user in context",
			user:   nil,
			wantID: 0,
		},
		{
			name: "user in context",
			user: &models.User{
				ID:    456,
				Email: "admin@example.com",
				Role:  "admin",
			},
			wantID: 456,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.user != nil {
				ctx = context.WithValue(ctx, contextKeyUser, tt.user)
			}

			got := getUserIDFromContext(ctx)
			if got != tt.wantID {
				t.Errorf("getUserIDFromContext() = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestAdminOnlyMiddleware_WithContext(t *testing.T) {
	tests := []struct {
		name       string
		user       *models.User
		wantStatus int
	}{
		{
			name:       "no user - forbidden",
			user:       nil,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "non-admin user - forbidden",
			user: &models.User{
				ID:    1,
				Email: "user1@example.com",
				Role:  "user",
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "admin user - allowed",
			user: &models.User{
				ID:    2,
				Email: "admin@example.com",
				Role:  "admin",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler that the middleware wraps
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with AdminOnlyMiddleware
			handler := AdminOnlyMiddleware(nextHandler)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if tt.user != nil {
				ctx := context.WithValue(req.Context(), contextKeyUser, tt.user)
				req = req.WithContext(ctx)
			}

			// Record response
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("AdminOnlyMiddleware() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestSessionCacheOperations(t *testing.T) {
	// Test session cache initialization
	cache := &sessionCache{
		sessions: make(map[string]*cachedSession),
	}

	// Test empty cache lookup
	cache.mu.RLock()
	_, exists := cache.sessions["nonexistent"]
	cache.mu.RUnlock()
	if exists {
		t.Error("Expected empty cache to not have session")
	}

	// Test adding to cache
	testSession := &cachedSession{
		UserID:    123,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	cache.mu.Lock()
	cache.sessions["test-token"] = testSession
	cache.mu.Unlock()

	// Test retrieving from cache
	cache.mu.RLock()
	retrieved, exists := cache.sessions["test-token"]
	cache.mu.RUnlock()
	if !exists {
		t.Error("Expected session to exist in cache")
	}
	if retrieved.UserID != 123 {
		t.Errorf("Expected UserID 123, got %d", retrieved.UserID)
	}

	// Test deletion from cache
	cache.mu.Lock()
	delete(cache.sessions, "test-token")
	cache.mu.Unlock()

	cache.mu.RLock()
	_, exists = cache.sessions["test-token"]
	cache.mu.RUnlock()
	if exists {
		t.Error("Expected session to be deleted from cache")
	}
}

func TestSessionCacheConcurrency(t *testing.T) {
	cache := &sessionCache{
		sessions: make(map[string]*cachedSession),
	}

	// Test concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.mu.Lock()
			cache.sessions["token"] = &cachedSession{
				UserID:    int64(i),
				ExpiresAt: time.Now().Add(time.Hour),
			}
			cache.mu.Unlock()
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.mu.RLock()
			_ = cache.sessions["token"]
			cache.mu.RUnlock()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}

func TestContextKeyType(t *testing.T) {
	// Test that context key is a specific type (not string)
	var key contextKey = "test"
	if key != "test" {
		t.Errorf("Expected key to equal 'test', got %v", key)
	}

	// Verify the contextKeyUser constant
	if contextKeyUser != "user" {
		t.Errorf("Expected contextKeyUser to be 'user', got %v", contextKeyUser)
	}
}

func TestSessionDuration(t *testing.T) {
	expected := 24 * time.Hour
	if SessionDuration != expected {
		t.Errorf("SessionDuration = %v, want %v", SessionDuration, expected)
	}
}

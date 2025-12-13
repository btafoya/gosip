package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// createTestUserWithBcrypt creates a user with a proper bcrypt hash
func createTestUserWithBcrypt(t *testing.T, setup *testSetup, email, password, role string) *models.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}

	if err := setup.DB.Users.Create(context.Background(), user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

func TestAuthHandler_Login_Success(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	// Create test user
	createTestUserWithBcrypt(t, setup, "test@example.com", "password123", "user")

	// Make login request
	reqBody := LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp LoginResponse
	decodeResponse(t, rr, &resp)

	if resp.User == nil {
		t.Error("Expected user in response")
	}
	if resp.User.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", resp.User.Email)
	}
	if resp.Token == "" {
		t.Error("Expected token in response")
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	createTestUserWithBcrypt(t, setup, "test@example.com", "password123", "user")

	reqBody := LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	assertStatus(t, rr, http.StatusUnauthorized)
	assertErrorCode(t, rr, ErrCodeAuthentication)
}

func TestAuthHandler_Login_UserNotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	reqBody := LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	assertStatus(t, rr, http.StatusUnauthorized)
	assertErrorCode(t, rr, ErrCodeAuthentication)
}

func TestAuthHandler_Login_MissingFields(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	// Missing password
	reqBody := LoginRequest{
		Email: "test@example.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertErrorCode(t, rr, ErrCodeValidation)
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestAuthHandler_Logout(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "test-token"})

	rr := httptest.NewRecorder()
	handler.Logout(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Check cookie is cleared
	cookies := rr.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "session" {
			if cookie.MaxAge != -1 {
				t.Error("Expected session cookie to be deleted")
			}
		}
	}
}

func TestAuthHandler_GetCurrentUser(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	user := createTestUserWithBcrypt(t, setup, "test@example.com", "password123", "user")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	ctx := context.WithValue(req.Context(), contextKeyUser, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetCurrentUser(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp UserResponse
	decodeResponse(t, rr, &resp)

	if resp.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", resp.Email)
	}
}

func TestAuthHandler_GetCurrentUser_Unauthorized(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rr := httptest.NewRecorder()
	handler.GetCurrentUser(rr, req)

	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthHandler_ChangePassword(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	user := createTestUserWithBcrypt(t, setup, "test@example.com", "oldpassword", "user")

	reqBody := ChangePasswordRequest{
		CurrentPassword: "oldpassword",
		NewPassword:     "newpassword123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), contextKeyUser, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestAuthHandler_ChangePassword_WrongCurrentPassword(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	user := createTestUserWithBcrypt(t, setup, "test@example.com", "oldpassword", "user")

	reqBody := ChangePasswordRequest{
		CurrentPassword: "wrongpassword",
		NewPassword:     "newpassword123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), contextKeyUser, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestAuthHandler_ChangePassword_ShortPassword(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	user := createTestUserWithBcrypt(t, setup, "test@example.com", "oldpassword", "user")

	reqBody := ChangePasswordRequest{
		CurrentPassword: "oldpassword",
		NewPassword:     "short",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), contextKeyUser, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ChangePassword(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertErrorCode(t, rr, ErrCodeValidation)
}

func TestAuthHandler_ListUsers(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	// Create some test users
	createTestUserWithBcrypt(t, setup, "user1@example.com", "password", "user")
	createTestUserWithBcrypt(t, setup, "user2@example.com", "password", "user")
	adminUser := createTestUserWithBcrypt(t, setup, "admin@example.com", "password", "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	ctx := context.WithValue(req.Context(), contextKeyUser, adminUser)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListUsers(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination.Total != 3 {
		t.Errorf("Expected 3 users, got %d", resp.Pagination.Total)
	}
}

func TestAuthHandler_CreateUser(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	reqBody := CreateUserRequest{
		Email:    "newuser@example.com",
		Password: "password123",
		Role:     "user",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.CreateUser(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp UserResponse
	decodeResponse(t, rr, &resp)

	if resp.Email != "newuser@example.com" {
		t.Errorf("Expected email newuser@example.com, got %s", resp.Email)
	}
	if resp.Role != "user" {
		t.Errorf("Expected role user, got %s", resp.Role)
	}
}

func TestAuthHandler_CreateUser_ValidationError(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	// Missing email
	reqBody := CreateUserRequest{
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.CreateUser(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertErrorCode(t, rr, ErrCodeValidation)
}

func TestAuthHandler_GetUser(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	user := createTestUserWithBcrypt(t, setup, "test@example.com", "password", "user")

	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.GetUser(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp UserResponse
	decodeResponse(t, rr, &resp)

	if resp.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, resp.Email)
	}
}

func TestAuthHandler_GetUser_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/users/9999", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.GetUser(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertErrorCode(t, rr, ErrCodeNotFound)
}

func TestAuthHandler_UpdateUser(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	createTestUserWithBcrypt(t, setup, "test@example.com", "password", "user")

	reqBody := UpdateUserRequest{
		Email: "updated@example.com",
		Role:  "admin",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/users/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.UpdateUser(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp UserResponse
	decodeResponse(t, rr, &resp)

	if resp.Email != "updated@example.com" {
		t.Errorf("Expected email updated@example.com, got %s", resp.Email)
	}
	if resp.Role != "admin" {
		t.Errorf("Expected role admin, got %s", resp.Role)
	}
}

func TestAuthHandler_DeleteUser(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	adminUser := createTestUserWithBcrypt(t, setup, "admin@example.com", "password", "admin")
	createTestUserWithBcrypt(t, setup, "test@example.com", "password", "user")

	req := httptest.NewRequest(http.MethodDelete, "/api/users/2", nil)
	ctx := context.WithValue(req.Context(), contextKeyUser, adminUser)
	req = req.WithContext(ctx)
	req = withURLParams(req, map[string]string{"id": "2"})

	rr := httptest.NewRecorder()
	handler.DeleteUser(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestAuthHandler_DeleteUser_CannotDeleteSelf(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	adminUser := createTestUserWithBcrypt(t, setup, "admin@example.com", "password", "admin")

	req := httptest.NewRequest(http.MethodDelete, "/api/users/1", nil)
	ctx := context.WithValue(req.Context(), contextKeyUser, adminUser)
	req = req.WithContext(ctx)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.DeleteUser(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertErrorCode(t, rr, ErrCodeBadRequest)
}

func TestAuthMiddleware(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}

	// Create a user and session
	user := createTestUserWithBcrypt(t, setup, "test@example.com", "password", "user")

	// Create session manually for testing
	token, _ := createSession(user.ID)

	middleware := AuthMiddleware(deps)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := GetUserFromContext(r.Context())
		if u == nil {
			t.Error("Expected user in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}

	middleware := AuthMiddleware(deps)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assertStatus(t, rr, http.StatusUnauthorized)
}

func TestAdminOnlyMiddleware(t *testing.T) {
	adminUser := &models.User{ID: 1, Email: "admin@test.com", Role: "admin"}
	regularUser := &models.User{ID: 2, Email: "user@test.com", Role: "user"}

	tests := []struct {
		name           string
		user           *models.User
		expectedStatus int
	}{
		{"Admin user allowed", adminUser, http.StatusOK},
		{"Regular user denied", regularUser, http.StatusForbidden},
		{"No user denied", nil, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := AdminOnlyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.user != nil {
				ctx := context.WithValue(req.Context(), contextKeyUser, tt.user)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assertStatus(t, rr, tt.expectedStatus)
		})
	}
}

func TestRateLimiting(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewAuthHandler(deps)

	createTestUserWithBcrypt(t, setup, "test@example.com", "password123", "user")

	// Make multiple failed login attempts
	for i := 0; i < 6; i++ {
		reqBody := LoginRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.1:12345"

		rr := httptest.NewRecorder()
		handler.Login(rr, req)

		// After max attempts, should be rate limited
		if i >= 5 {
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf("Expected 429 after %d attempts, got %d", i+1, rr.Code)
			}
		}
	}
}

// Helper to add chi URL params
func withURLParamsForAuth(r *http.Request, params map[string]string) *http.Request {
	ctx := chi.NewRouteContext()
	for key, value := range params {
		ctx.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
}

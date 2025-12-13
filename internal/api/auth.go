package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication-related API endpoints
type AuthHandler struct {
	deps           *Dependencies
	loginAttempts  map[string][]time.Time
	attemptsMu     sync.RWMutex
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(deps *Dependencies) *AuthHandler {
	return &AuthHandler{
		deps:          deps,
		loginAttempts: make(map[string][]time.Time),
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User  *UserResponse `json:"user"`
	Token string        `json:"token"`
}

// UserResponse represents a user in API responses (without password hash)
type UserResponse struct {
	ID        int64      `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		WriteValidationError(w, "Email and password are required", []FieldError{
			{Field: "email", Message: "Email is required"},
			{Field: "password", Message: "Password is required"},
		})
		return
	}

	// Check rate limiting
	clientIP := r.RemoteAddr
	if allowed, lockoutRemaining := h.checkLoginAttempt(clientIP); !allowed {
		WriteError(w, http.StatusTooManyRequests, ErrCodeRateLimited,
			"Too many login attempts. Try again in "+lockoutRemaining.String(), nil)
		return
	}

	// Get user by email
	user, err := h.deps.DB.Users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if err == db.ErrUserNotFound {
			h.recordFailedAttempt(clientIP)
			WriteError(w, http.StatusUnauthorized, ErrCodeAuthentication, "Invalid email or password", nil)
			return
		}
		WriteInternalError(w)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.recordFailedAttempt(clientIP)
		WriteError(w, http.StatusUnauthorized, ErrCodeAuthentication, "Invalid email or password", nil)
		return
	}

	// Clear failed attempts on successful login
	h.clearFailedAttempts(clientIP)

	// Update last login
	h.deps.DB.Users.UpdateLastLogin(r.Context(), user.ID)

	// Create session
	token, err := createSession(user.ID)
	if err != nil {
		WriteInternalError(w)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(config.SessionDuration.Seconds()),
	})

	WriteJSON(w, http.StatusOK, LoginResponse{
		User:  toUserResponse(user),
		Token: token,
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get session from cookie
	cookie, err := r.Cookie("session")
	if err == nil {
		deleteSession(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

// GetCurrentUser returns the current authenticated user
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		WriteUnauthorizedError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toUserResponse(user))
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles password changes for the current user
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		WriteUnauthorizedError(w)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "Current password is incorrect", nil)
		return
	}

	// Validate new password
	if len(req.NewPassword) < 8 {
		WriteValidationError(w, "Password must be at least 8 characters", []FieldError{
			{Field: "new_password", Message: "Password must be at least 8 characters"},
		})
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		WriteInternalError(w)
		return
	}

	// Update password
	user.PasswordHash = string(hash)
	if err := h.deps.DB.Users.Update(r.Context(), user); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

// Admin user management endpoints

// ListUsers returns all users (admin only)
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = config.DefaultPageSize
	}
	if limit > config.MaxPageSize {
		limit = config.MaxPageSize
	}

	users, err := h.deps.DB.Users.List(r.Context(), limit, offset)
	if err != nil {
		WriteInternalError(w)
		return
	}

	total, _ := h.deps.DB.Users.Count(r.Context())

	// Convert to response format
	var response []*UserResponse
	for _, u := range users {
		response = append(response, toUserResponse(u))
	}

	WriteList(w, response, total, limit, offset)
}

// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// CreateUser creates a new user (admin only)
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	var errors []FieldError
	if req.Email == "" {
		errors = append(errors, FieldError{Field: "email", Message: "Email is required"})
	}
	if len(req.Password) < 8 {
		errors = append(errors, FieldError{Field: "password", Message: "Password must be at least 8 characters"})
	}
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}

	if len(errors) > 0 {
		WriteValidationError(w, "Validation failed", errors)
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		WriteInternalError(w)
		return
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         req.Role,
		CreatedAt:    time.Now(),
	}

	if err := h.deps.DB.Users.Create(r.Context(), user); err != nil {
		WriteError(w, http.StatusConflict, ErrCodeConflict, "User with this email already exists", nil)
		return
	}

	WriteJSON(w, http.StatusCreated, toUserResponse(user))
}

// GetUser returns a specific user (admin only)
func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid user ID", nil)
		return
	}

	user, err := h.deps.DB.Users.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrUserNotFound {
			WriteNotFoundError(w, "User")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toUserResponse(user))
}

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Role     string `json:"role,omitempty"`
}

// UpdateUser updates a user (admin only)
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid user ID", nil)
		return
	}

	user, err := h.deps.DB.Users.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrUserNotFound {
			WriteNotFoundError(w, "User")
			return
		}
		WriteInternalError(w)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			WriteInternalError(w)
			return
		}
		user.PasswordHash = string(hash)
	}
	if req.Role == "admin" || req.Role == "user" {
		user.Role = req.Role
	}

	if err := h.deps.DB.Users.Update(r.Context(), user); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toUserResponse(user))
}

// DeleteUser deletes a user (admin only)
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid user ID", nil)
		return
	}

	// Prevent deleting yourself
	currentUser := GetUserFromContext(r.Context())
	if currentUser.ID == id {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "Cannot delete your own account", nil)
		return
	}

	if err := h.deps.DB.Users.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

// Rate limiting helpers

func (h *AuthHandler) checkLoginAttempt(ip string) (bool, time.Duration) {
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	// Clean old attempts
	var recent []time.Time
	for _, attempt := range h.loginAttempts[ip] {
		if attempt.After(cutoff) {
			recent = append(recent, attempt)
		}
	}
	h.loginAttempts[ip] = recent

	// Check if locked out
	if len(recent) >= config.MaxFailedLoginAttempts {
		lockoutEnd := recent[0].Add(config.LoginLockoutDuration)
		if now.Before(lockoutEnd) {
			return false, lockoutEnd.Sub(now)
		}
		// Lockout expired, reset
		h.loginAttempts[ip] = nil
	}

	return true, 0
}

func (h *AuthHandler) recordFailedAttempt(ip string) {
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()
	h.loginAttempts[ip] = append(h.loginAttempts[ip], time.Now())
}

func (h *AuthHandler) clearFailedAttempts(ip string) {
	h.attemptsMu.Lock()
	defer h.attemptsMu.Unlock()
	delete(h.loginAttempts, ip)
}

func toUserResponse(user *models.User) *UserResponse {
	return &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		LastLogin: user.LastLogin,
	}
}

package db

import (
	"context"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	user := &models.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		Role:         "admin",
	}

	err := db.Users.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be set after creation")
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create a user first
	user := &models.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		Role:         "user",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Retrieve the user
	retrieved, err := db.Users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user by ID: %v", err)
	}

	if retrieved.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrieved.Email)
	}
	if retrieved.Role != user.Role {
		t.Errorf("Expected role %s, got %s", user.Role, retrieved.Role)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Users.GetByID(ctx, 9999)
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	user := &models.User{
		Email:        "findme@example.com",
		PasswordHash: "hashed_password",
		Role:         "admin",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	retrieved, err := db.Users.GetByEmail(ctx, "findme@example.com")
	if err != nil {
		t.Fatalf("Failed to get user by email: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("Expected ID %d, got %d", user.ID, retrieved.ID)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Users.GetByEmail(ctx, "nonexistent@example.com")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	user := &models.User{
		Email:        "update@example.com",
		PasswordHash: "hashed_password",
		Role:         "user",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Update the user
	user.Email = "updated@example.com"
	user.Role = "admin"
	if err := db.Users.Update(ctx, user); err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify update
	retrieved, err := db.Users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if retrieved.Email != "updated@example.com" {
		t.Errorf("Expected email updated@example.com, got %s", retrieved.Email)
	}
	if retrieved.Role != "admin" {
		t.Errorf("Expected role admin, got %s", retrieved.Role)
	}
}

func TestUserRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	user := &models.User{
		Email:        "delete@example.com",
		PasswordHash: "hashed_password",
		Role:         "user",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if err := db.Users.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	_, err := db.Users.GetByID(ctx, user.ID)
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound after delete, got %v", err)
	}
}

func TestUserRepository_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create multiple users
	for i := 0; i < 5; i++ {
		user := &models.User{
			Email:        "list" + string(rune('0'+i)) + "@example.com",
			PasswordHash: "hashed_password",
			Role:         "user",
		}
		if err := db.Users.Create(ctx, user); err != nil {
			t.Fatalf("Failed to create user %d: %v", i, err)
		}
	}

	// List with pagination
	users, err := db.Users.List(ctx, 3, 0)
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}

	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}

	// List second page
	users, err = db.Users.List(ctx, 3, 3)
	if err != nil {
		t.Fatalf("Failed to list users page 2: %v", err)
	}

	if len(users) != 2 {
		t.Errorf("Expected 2 users on page 2, got %d", len(users))
	}
}

func TestUserRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially should be 0
	count, err := db.Users.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 users, got %d", count)
	}

	// Create users
	for i := 0; i < 3; i++ {
		user := &models.User{
			Email:        "count" + string(rune('0'+i)) + "@example.com",
			PasswordHash: "hashed_password",
			Role:         "user",
		}
		if err := db.Users.Create(ctx, user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}

	count, err = db.Users.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 users, got %d", count)
	}
}

func TestUserRepository_HasAdmin(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially no admin
	hasAdmin, err := db.Users.HasAdmin(ctx)
	if err != nil {
		t.Fatalf("Failed to check HasAdmin: %v", err)
	}
	if hasAdmin {
		t.Error("Expected no admin initially")
	}

	// Create regular user
	user := &models.User{
		Email:        "regular@example.com",
		PasswordHash: "hashed_password",
		Role:         "user",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	hasAdmin, err = db.Users.HasAdmin(ctx)
	if err != nil {
		t.Fatalf("Failed to check HasAdmin: %v", err)
	}
	if hasAdmin {
		t.Error("Expected no admin with only regular user")
	}

	// Create admin user
	admin := &models.User{
		Email:        "admin@example.com",
		PasswordHash: "hashed_password",
		Role:         "admin",
	}
	if err := db.Users.Create(ctx, admin); err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	hasAdmin, err = db.Users.HasAdmin(ctx)
	if err != nil {
		t.Fatalf("Failed to check HasAdmin: %v", err)
	}
	if !hasAdmin {
		t.Error("Expected admin to exist")
	}
}

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	user := &models.User{
		Email:        "login@example.com",
		PasswordHash: "hashed_password",
		Role:         "user",
	}
	if err := db.Users.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Initially no last login
	retrieved, _ := db.Users.GetByID(ctx, user.ID)
	if retrieved.LastLogin != nil {
		t.Error("Expected nil LastLogin initially")
	}

	// Update last login
	if err := db.Users.UpdateLastLogin(ctx, user.ID); err != nil {
		t.Fatalf("Failed to update last login: %v", err)
	}

	retrieved, _ = db.Users.GetByID(ctx, user.ID)
	if retrieved.LastLogin == nil {
		t.Error("Expected LastLogin to be set")
	}
}

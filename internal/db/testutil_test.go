package db

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations to create tables
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

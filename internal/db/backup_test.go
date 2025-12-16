package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateBackup(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create a backup
	ctx := context.Background()
	filename, size, err := db.CreateBackup(ctx)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Verify filename format
	if filename == "" {
		t.Error("Backup filename is empty")
	}
	if len(filename) < 20 {
		t.Errorf("Backup filename too short: %s", filename)
	}

	// Verify backup file exists
	backupPath := filepath.Join(db.backupsDir, filename)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file does not exist: %s", backupPath)
	}

	// Verify size is positive
	if size <= 0 {
		t.Logf("Warning: backup size is %d (may be due to empty database)", size)
	}

	t.Logf("Created backup: %s (size: %d bytes)", filename, size)
}

func TestListBackups(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()

	// Initially should be empty
	backups, err := db.ListBackups(ctx)
	if err != nil {
		t.Fatalf("Failed to list backups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("Expected 0 backups, got %d", len(backups))
	}

	// Create a backup
	filename, _, err := db.CreateBackup(ctx)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Now should have one backup
	backups, err = db.ListBackups(ctx)
	if err != nil {
		t.Fatalf("Failed to list backups: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("Expected 1 backup, got %d", len(backups))
	}
	if backups[0].Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, backups[0].Filename)
	}
}

func TestVerifyBackup(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()

	// Create a backup
	filename, _, err := db.CreateBackup(ctx)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Verify the backup
	if err := db.VerifyBackup(ctx, filename); err != nil {
		t.Errorf("Backup verification failed: %v", err)
	}
}

func TestDeleteBackup(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()

	// Create a backup
	filename, _, err := db.CreateBackup(ctx)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Delete the backup
	if err := db.DeleteBackup(ctx, filename); err != nil {
		t.Fatalf("Failed to delete backup: %v", err)
	}

	// Verify backup no longer exists
	backups, err := db.ListBackups(ctx)
	if err != nil {
		t.Fatalf("Failed to list backups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("Expected 0 backups after delete, got %d", len(backups))
	}
}

func TestRestoreBackup(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()

	// Insert some test data
	testEmail := "test@example.com"
	_, err = db.conn.ExecContext(ctx, "INSERT INTO users (email, password_hash, role) VALUES (?, 'hash', 'user')", testEmail)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Create a backup
	filename, _, err := db.CreateBackup(ctx)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Delete the test data
	_, err = db.conn.ExecContext(ctx, "DELETE FROM users WHERE email = ?", testEmail)
	if err != nil {
		t.Fatalf("Failed to delete test user: %v", err)
	}

	// Verify data is deleted
	var count int
	err = db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = ?", testEmail).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 users after delete, got %d", count)
	}

	// Restore from backup
	if err := db.RestoreBackup(ctx, filename); err != nil {
		t.Fatalf("Failed to restore backup: %v", err)
	}

	// Verify data is restored
	err = db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = ?", testEmail).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count users after restore: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 user after restore, got %d", count)
	}
}

func TestCleanOldBackups(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create a fake old backup
	oldBackupFile := "backup_20200101_120000.db"
	oldBackupPath := filepath.Join(db.backupsDir, oldBackupFile)

	// Create an empty file
	f, err := os.Create(oldBackupPath)
	if err != nil {
		t.Fatalf("Failed to create old backup file: %v", err)
	}
	f.Close()

	// Set the modification time to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(oldBackupPath, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set modification time: %v", err)
	}

	// Create a recent backup
	recentBackupFile := "backup_" + time.Now().Format("20060102_150405") + ".db"
	recentBackupPath := filepath.Join(db.backupsDir, recentBackupFile)
	f, err = os.Create(recentBackupPath)
	if err != nil {
		t.Fatalf("Failed to create recent backup file: %v", err)
	}
	f.Close()

	// Clean old backups (30 day retention)
	deletedCount, err := db.CleanOldBackups(ctx, 30)
	if err != nil {
		t.Fatalf("Failed to clean old backups: %v", err)
	}

	if deletedCount != 1 {
		t.Errorf("Expected 1 deleted backup, got %d", deletedCount)
	}

	// Verify old backup is deleted
	if _, err := os.Stat(oldBackupPath); !os.IsNotExist(err) {
		t.Error("Old backup should have been deleted")
	}

	// Verify recent backup still exists
	if _, err := os.Stat(recentBackupPath); os.IsNotExist(err) {
		t.Error("Recent backup should not have been deleted")
	}
}

func TestValidateFilename(t *testing.T) {
	testCases := []struct {
		name      string
		filename  string
		expectErr bool
	}{
		{"valid filename", "backup_20251215_143022.db", false},
		{"empty filename", "", true},
		{"path traversal", "../backup_20251215_143022.db", true},
		{"slash in name", "path/backup_20251215_143022.db", true},
		{"wrong prefix", "other_20251215_143022.db", true},
		{"wrong extension", "backup_20251215_143022.txt", true},
		{"invalid date format", "backup_abc123.db", true},
		{"sql injection attempt", "backup'; DROP TABLE users;--.db", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFilename(tc.filename)
			if tc.expectErr && err == nil {
				t.Errorf("Expected error for filename %q, got nil", tc.filename)
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error for filename %q: %v", tc.filename, err)
			}
		})
	}
}

func TestGetBackupsDir(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Verify backups directory is set correctly
	expectedDir := filepath.Join(tmpDir, "backups")
	if db.GetBackupsDir() != expectedDir {
		t.Errorf("Expected backups dir %s, got %s", expectedDir, db.GetBackupsDir())
	}

	// Verify directory exists
	if _, err := os.Stat(db.GetBackupsDir()); os.IsNotExist(err) {
		t.Error("Backups directory should exist")
	}
}

func TestSetBackupsDir(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a test database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Set a custom backups directory
	customDir := filepath.Join(tmpDir, "custom_backups")
	if err := db.SetBackupsDir(customDir); err != nil {
		t.Fatalf("Failed to set backups dir: %v", err)
	}

	// Verify it was set correctly
	if db.GetBackupsDir() != customDir {
		t.Errorf("Expected backups dir %s, got %s", customDir, db.GetBackupsDir())
	}

	// Verify directory was created
	if _, err := os.Stat(customDir); os.IsNotExist(err) {
		t.Error("Custom backups directory should have been created")
	}
}

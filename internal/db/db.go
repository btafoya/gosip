// Package db provides database access and repository implementations
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the SQL database connection and provides repositories
type DB struct {
	conn   *sql.DB
	dbPath string // Path to the database file

	// Backup configuration
	backupsDir string

	// Repositories
	Users         *UserRepository
	Devices       *DeviceRepository
	Registrations *RegistrationRepository
	DIDs          *DIDRepository
	Routes        *RouteRepository
	Blocklist     *BlocklistRepository
	CDRs          *CDRRepository
	Voicemails    *VoicemailRepository
	Messages      *MessageRepository
	AutoReplies   *AutoReplyRepository
	Config        *ConfigRepository
	Sessions      *SessionRepository

	// Provisioning repositories
	ProvisioningTokens   *ProvisioningTokenRepository
	ProvisioningProfiles *ProvisioningProfileRepository
	DeviceEvents         *DeviceEventRepository
}

// New creates a new database connection and initializes repositories
func New(dbPath string) (*DB, error) {
	// Enable WAL mode and foreign keys via connection string
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000", dbPath)

	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings for SQLite
	conn.SetMaxOpenConns(1) // SQLite handles one writer at a time
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Calculate backups directory (sibling to database file)
	dataDir := filepath.Dir(dbPath)
	backupsDir := filepath.Join(dataDir, "backups")

	db := &DB{
		conn:       conn,
		dbPath:     dbPath,
		backupsDir: backupsDir,
	}

	// Ensure backups directory exists
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create backups directory: %w", err)
	}

	// Initialize repositories
	db.Users = NewUserRepository(conn)
	db.Devices = NewDeviceRepository(conn)
	db.Registrations = NewRegistrationRepository(conn)
	db.DIDs = NewDIDRepository(conn)
	db.Routes = NewRouteRepository(conn)
	db.Blocklist = NewBlocklistRepository(conn)
	db.CDRs = NewCDRRepository(conn)
	db.Voicemails = NewVoicemailRepository(conn)
	db.Messages = NewMessageRepository(conn)
	db.AutoReplies = NewAutoReplyRepository(conn)
	db.Config = NewConfigRepository(conn)
	db.Sessions = NewSessionRepository(conn)

	// Provisioning repositories
	db.ProvisioningTokens = NewProvisioningTokenRepository(conn)
	db.ProvisioningProfiles = NewProvisioningProfileRepository(conn)
	db.DeviceEvents = NewDeviceEventRepository(conn)

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Migrate runs all database migrations
func (db *DB) Migrate() error {
	// Create migrations table if not exists
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Filter and sort up migrations
	var migrations []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			migrations = append(migrations, entry.Name())
		}
	}

	// Apply each migration
	for _, filename := range migrations {
		// Extract version number from filename (e.g., "001_initial_schema.up.sql" -> 1)
		var version int
		fmt.Sscanf(filename, "%d_", &version)

		// Check if already applied
		var count int
		err := db.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if count > 0 {
			continue // Already applied
		}

		// Read migration file
		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		// Execute migration in transaction
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Split by semicolons and execute each statement
		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to execute migration %s: %w", filename, err)
			}
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}

		slog.Info("Applied migration", "version", version, "file", filename)
	}

	return nil
}

// Conn returns the underlying database connection for advanced operations
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// BackupInfo represents backup file information
type BackupInfo struct {
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// validateBackupPath validates and sanitizes a backup file path to prevent SQL injection
// and path traversal attacks
func validateBackupPath(backupPath string) error {
	// Normalize the path to remove any redundant separators or relative components
	cleanPath := filepath.Clean(backupPath)

	// Ensure the path is absolute
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("backup path must be absolute: %s", backupPath)
	}

	// Check for directory traversal attempts
	if strings.Contains(backupPath, "..") {
		return fmt.Errorf("backup path cannot contain directory traversal: %s", backupPath)
	}

	// Validate only safe characters (alphanumeric, underscores, hyphens, slashes, dots)
	// This prevents SQL injection through special characters
	safePathPattern := regexp.MustCompile(`^[a-zA-Z0-9_/.\-]+$`)
	if !safePathPattern.MatchString(cleanPath) {
		return fmt.Errorf("backup path contains invalid characters: %s", backupPath)
	}

	return nil
}

// validateFilename validates a backup filename to prevent path traversal
func validateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is required")
	}

	// Check for path separators
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("filename cannot contain path separators")
	}

	// Check for directory traversal
	if strings.Contains(filename, "..") {
		return fmt.Errorf("filename cannot contain directory traversal")
	}

	// Validate filename format (must be backup_*.db)
	if !strings.HasPrefix(filename, "backup_") || !strings.HasSuffix(filename, ".db") {
		return fmt.Errorf("invalid backup filename format")
	}

	// Validate only safe characters
	safePattern := regexp.MustCompile(`^backup_[0-9]{8}_[0-9]{6}\.db$`)
	if !safePattern.MatchString(filename) {
		return fmt.Errorf("invalid backup filename format")
	}

	return nil
}

// SetBackupsDir sets the backups directory path
func (db *DB) SetBackupsDir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("failed to create backups directory: %w", err)
	}

	db.backupsDir = absDir
	return nil
}

// GetBackupsDir returns the backups directory path
func (db *DB) GetBackupsDir() string {
	return db.backupsDir
}

// CreateBackup creates a backup of the database using SQLite VACUUM INTO
// Returns the filename, size in bytes, and any error
func (db *DB) CreateBackup(ctx context.Context) (string, int64, error) {
	// Generate backup filename with timestamp
	filename := fmt.Sprintf("backup_%s.db", time.Now().Format("20060102_150405"))

	// Build full backup path
	backupPath := filepath.Join(db.backupsDir, filename)

	// Get absolute path for the backup
	absBackupPath, err := filepath.Abs(backupPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Validate the backup path to prevent SQL injection and path traversal
	if err := validateBackupPath(absBackupPath); err != nil {
		return "", 0, fmt.Errorf("invalid backup path: %w", err)
	}

	slog.Info("Creating database backup", "filename", filename, "path", absBackupPath)

	// Use VACUUM INTO for a consistent backup (creates compacted copy)
	// This is safe for hot backups and doesn't lock the database for long
	query := "VACUUM INTO ?"
	_, err = db.conn.ExecContext(ctx, query, absBackupPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create backup: %w", err)
	}

	// Get the backup file size
	fileInfo, err := os.Stat(absBackupPath)
	if err != nil {
		slog.Warn("Failed to get backup file size", "error", err)
		// Return success with 0 size rather than failing
		return filename, 0, nil
	}

	slog.Info("Database backup created successfully",
		"filename", filename,
		"size", fileInfo.Size(),
	)

	return filename, fileInfo.Size(), nil
}

// ListBackups returns available backup files sorted by creation time (newest first)
func (db *DB) ListBackups(ctx context.Context) ([]BackupInfo, error) {
	entries, err := os.ReadDir(db.backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read backups directory: %w", err)
	}

	var backups []BackupInfo

	for _, entry := range entries {
		// Skip directories and non-backup files
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// Only include files matching backup pattern
		if !strings.HasPrefix(filename, "backup_") || !strings.HasSuffix(filename, ".db") {
			continue
		}

		// Get file info for size and modification time
		info, err := entry.Info()
		if err != nil {
			slog.Warn("Failed to get file info", "filename", filename, "error", err)
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:  filename,
			Size:      info.Size(),
			CreatedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	// Sort by created_at descending (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt > backups[j].CreatedAt
	})

	return backups, nil
}

// GetBackup returns information about a specific backup file
func (db *DB) GetBackup(ctx context.Context, filename string) (*BackupInfo, error) {
	if err := validateFilename(filename); err != nil {
		return nil, err
	}

	backupPath := filepath.Join(db.backupsDir, filename)

	info, err := os.Stat(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("backup not found: %s", filename)
		}
		return nil, fmt.Errorf("failed to get backup info: %w", err)
	}

	return &BackupInfo{
		Filename:  filename,
		Size:      info.Size(),
		CreatedAt: info.ModTime().Format(time.RFC3339),
	}, nil
}

// DeleteBackup deletes a backup file
func (db *DB) DeleteBackup(ctx context.Context, filename string) error {
	if err := validateFilename(filename); err != nil {
		return err
	}

	backupPath := filepath.Join(db.backupsDir, filename)

	if err := os.Remove(backupPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("backup not found: %s", filename)
		}
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	slog.Info("Backup deleted", "filename", filename)
	return nil
}

// VerifyBackup checks the integrity of a backup file
func (db *DB) VerifyBackup(ctx context.Context, filename string) error {
	if err := validateFilename(filename); err != nil {
		return err
	}

	backupPath := filepath.Join(db.backupsDir, filename)

	// Open the backup database
	backupConn, err := sql.Open("sqlite3", backupPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("failed to open backup: %w", err)
	}
	defer backupConn.Close()

	// Run integrity check
	var result string
	err = backupConn.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result)
	if err != nil {
		return fmt.Errorf("failed to check backup integrity: %w", err)
	}

	if result != "ok" {
		return fmt.Errorf("backup integrity check failed: %s", result)
	}

	slog.Info("Backup integrity verified", "filename", filename)
	return nil
}

// RestoreBackup restores the database from a backup file
// WARNING: This operation is destructive - the current database will be replaced
// The application should be restarted after restoration
func (db *DB) RestoreBackup(ctx context.Context, filename string) error {
	if err := validateFilename(filename); err != nil {
		return err
	}

	backupPath := filepath.Join(db.backupsDir, filename)

	// Check if backup file exists
	if _, err := os.Stat(backupPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("backup not found: %s", filename)
		}
		return fmt.Errorf("failed to access backup: %w", err)
	}

	// Verify backup integrity before restoring
	if err := db.VerifyBackup(ctx, filename); err != nil {
		return fmt.Errorf("backup verification failed: %w", err)
	}

	slog.Info("Starting database restore", "filename", filename, "target", db.dbPath)

	// Create a backup of the current database before restoring
	preRestoreBackup := fmt.Sprintf("pre_restore_%s.db", time.Now().Format("20060102_150405"))
	preRestorePath := filepath.Join(db.backupsDir, preRestoreBackup)

	// Copy current database to pre-restore backup
	if err := copyFile(db.dbPath, preRestorePath); err != nil {
		slog.Warn("Failed to create pre-restore backup", "error", err)
		// Continue with restore anyway - the backup file exists
	} else {
		slog.Info("Created pre-restore backup", "filename", preRestoreBackup)
	}

	// Close the current database connection
	if err := db.conn.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	// Copy backup file to database path
	if err := copyFile(backupPath, db.dbPath); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	// Reopen the database connection
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000", db.dbPath)
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("failed to reopen database: %w", err)
	}

	// Verify the restored database
	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx2); err != nil {
		conn.Close()
		return fmt.Errorf("failed to verify restored database: %w", err)
	}

	// Update the connection
	db.conn = conn

	// Reinitialize all repositories with new connection
	db.Users = NewUserRepository(conn)
	db.Devices = NewDeviceRepository(conn)
	db.Registrations = NewRegistrationRepository(conn)
	db.DIDs = NewDIDRepository(conn)
	db.Routes = NewRouteRepository(conn)
	db.Blocklist = NewBlocklistRepository(conn)
	db.CDRs = NewCDRRepository(conn)
	db.Voicemails = NewVoicemailRepository(conn)
	db.Messages = NewMessageRepository(conn)
	db.AutoReplies = NewAutoReplyRepository(conn)
	db.Config = NewConfigRepository(conn)
	db.Sessions = NewSessionRepository(conn)
	db.ProvisioningTokens = NewProvisioningTokenRepository(conn)
	db.ProvisioningProfiles = NewProvisioningProfileRepository(conn)
	db.DeviceEvents = NewDeviceEventRepository(conn)

	slog.Info("Database restored successfully", "filename", filename)
	return nil
}

// CleanOldBackups removes backup files older than the specified number of days
func (db *DB) CleanOldBackups(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays < 1 {
		return 0, fmt.Errorf("retention days must be at least 1")
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deletedCount := 0

	entries, err := os.ReadDir(db.backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read backups directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasPrefix(filename, "backup_") || !strings.HasSuffix(filename, ".db") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			backupPath := filepath.Join(db.backupsDir, filename)
			if err := os.Remove(backupPath); err != nil {
				slog.Warn("Failed to delete old backup", "filename", filename, "error", err)
				continue
			}
			deletedCount++
			slog.Info("Deleted old backup", "filename", filename, "age_days", int(time.Since(info.ModTime()).Hours()/24))
		}
	}

	return deletedCount, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

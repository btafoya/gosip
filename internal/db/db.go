// Package db provides database access and repository implementations
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the SQL database connection and provides repositories
type DB struct {
	conn *sql.DB

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

	db := &DB{conn: conn}

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

// CreateBackup creates a backup of the database
func (db *DB) CreateBackup(ctx context.Context) (string, int64, error) {
	// Generate backup filename
	filename := fmt.Sprintf("backup_%s.db", time.Now().Format("20060102_150405"))

	// Get absolute path for the backup
	backupPath, err := filepath.Abs(filename)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Validate the backup path to prevent SQL injection and path traversal
	if err := validateBackupPath(backupPath); err != nil {
		return "", 0, fmt.Errorf("invalid backup path: %w", err)
	}

	// Use VACUUM INTO with validated path
	// The path is now sanitized, but we still use proper SQL formatting
	query := "VACUUM INTO ?"
	_, err = db.conn.ExecContext(ctx, query, backupPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create backup: %w", err)
	}

	// Note: In production, you'd want to get the actual file size
	// For now, return 0 as placeholder
	return filename, 0, nil
}

// ListBackups returns available backup files
func (db *DB) ListBackups(ctx context.Context) ([]BackupInfo, error) {
	// In production, this would list files from the backups directory
	// For now, return empty list
	return []BackupInfo{}, nil
}

// RestoreBackup restores the database from a backup file
func (db *DB) RestoreBackup(ctx context.Context, filename string) error {
	// In production, this would restore from the backup file
	// This is a placeholder implementation
	if filename == "" {
		return fmt.Errorf("backup filename is required")
	}
	return nil
}

// Package db provides database access and repository implementations
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
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

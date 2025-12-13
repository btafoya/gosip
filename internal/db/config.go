package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var ErrConfigNotFound = errors.New("config key not found")

// ConfigRepository handles database operations for system configuration
type ConfigRepository struct {
	db *sql.DB
}

// NewConfigRepository creates a new ConfigRepository
func NewConfigRepository(db *sql.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// Get retrieves a config value by key
func (r *ConfigRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", ErrConfigNotFound
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// GetWithDefault retrieves a config value or returns the default if not found
func (r *ConfigRepository) GetWithDefault(ctx context.Context, key, defaultValue string) string {
	value, err := r.Get(ctx, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// Set creates or updates a config value
func (r *ConfigRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now())
	return err
}

// Delete removes a config entry
func (r *ConfigRepository) Delete(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM config WHERE key = ?`, key)
	return err
}

// GetAll retrieves all config entries
func (r *ConfigRepository) GetAll(ctx context.Context) ([]*models.SystemConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value, updated_at FROM config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.SystemConfig
	for rows.Next() {
		cfg := &models.SystemConfig{}
		if err := rows.Scan(&cfg.Key, &cfg.Value, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// GetByPrefix retrieves all config entries with a given key prefix
func (r *ConfigRepository) GetByPrefix(ctx context.Context, prefix string) ([]*models.SystemConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value, updated_at FROM config WHERE key LIKE ? ORDER BY key`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.SystemConfig
	for rows.Next() {
		cfg := &models.SystemConfig{}
		if err := rows.Scan(&cfg.Key, &cfg.Value, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// SetMultiple sets multiple config values in a transaction
func (r *ConfigRepository) SetMultiple(ctx context.Context, configs map[string]string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for key, value := range configs {
		if _, err := stmt.ExecContext(ctx, key, value, now); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// Predefined config keys
const (
	ConfigKeyTwilioAccountSID   = "twilio.account_sid"
	ConfigKeyTwilioAuthToken    = "twilio.auth_token"
	ConfigKeySMTPHost           = "smtp.host"
	ConfigKeySMTPPort           = "smtp.port"
	ConfigKeySMTPUser           = "smtp.user"
	ConfigKeySMTPPassword       = "smtp.password"
	ConfigKeySMTPFrom           = "smtp.from"
	ConfigKeyPostmarkAPIToken   = "postmark.api_token"
	ConfigKeyGotifyURL          = "gotify.url"
	ConfigKeyGotifyToken        = "gotify.token"
	ConfigKeySetupComplete      = "setup.complete"
	ConfigKeyDNDEnabled         = "dnd.enabled"
	ConfigKeyDNDStart           = "dnd.start_time"
	ConfigKeyDNDEnd             = "dnd.end_time"
	ConfigKeySpamFilterEnabled  = "spam_filter.enabled"
	ConfigKeySpamScoreThreshold = "spam_filter.threshold"
)

// IsSetupComplete checks if the initial setup has been completed
func (r *ConfigRepository) IsSetupComplete(ctx context.Context) bool {
	return r.GetWithDefault(ctx, ConfigKeySetupComplete, "false") == "true"
}

// MarkSetupComplete marks the initial setup as complete
func (r *ConfigRepository) MarkSetupComplete(ctx context.Context) error {
	return r.Set(ctx, ConfigKeySetupComplete, "true")
}

// IsDNDEnabled checks if Do Not Disturb mode is enabled
func (r *ConfigRepository) IsDNDEnabled(ctx context.Context) bool {
	return r.GetWithDefault(ctx, ConfigKeyDNDEnabled, "false") == "true"
}

// SetDNDEnabled sets the Do Not Disturb mode
func (r *ConfigRepository) SetDNDEnabled(ctx context.Context, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return r.Set(ctx, ConfigKeyDNDEnabled, value)
}

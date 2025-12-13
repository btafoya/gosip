// Package config provides runtime configuration management for GoSIP
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the runtime configuration for GoSIP
type Config struct {
	// Server settings
	SIPPort   int
	HTTPPort  int
	DataDir   string
	SIPDomain string // SIP domain for registrations (e.g., "sip.example.com")

	// Twilio credentials (loaded from database after setup)
	TwilioAccountSID string
	TwilioAuthToken  string

	// Email settings
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPTLS      bool

	// Postmarkapp (alternative to SMTP)
	PostmarkAPIToken string

	// Gotify push notifications
	GotifyURL   string
	GotifyToken string

	// Feature flags
	RecordingEnabled bool
	DebugMode        bool
}

// Load creates a Config from environment variables with defaults
func Load() *Config {
	cfg := &Config{
		SIPPort:   getEnvInt("GOSIP_SIP_PORT", DefaultSIPPort),
		HTTPPort:  getEnvInt("GOSIP_HTTP_PORT", DefaultHTTPPort),
		DataDir:   getEnv("GOSIP_DATA_DIR", DefaultDataDir),
		SIPDomain: getEnv("GOSIP_SIP_DOMAIN", "localhost"),

		// These are typically loaded from database after initial setup
		TwilioAccountSID: getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:  getEnv("TWILIO_AUTH_TOKEN", ""),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),
		SMTPTLS:      getEnvBool("SMTP_TLS", false),

		PostmarkAPIToken: getEnv("POSTMARK_API_TOKEN", ""),

		GotifyURL:   getEnv("GOTIFY_URL", ""),
		GotifyToken: getEnv("GOTIFY_TOKEN", ""),

		RecordingEnabled: getEnvBool("GOSIP_RECORDING_ENABLED", true),
		DebugMode:        getEnvBool("GOSIP_DEBUG", false),
	}

	return cfg
}

// DBPath returns the full path to the SQLite database file
func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, DefaultDBFile)
}

// RecordingsPath returns the path to the recordings directory
func (c *Config) RecordingsPath() string {
	return filepath.Join(c.DataDir, RecordingsDir)
}

// VoicemailsPath returns the path to the voicemails directory
func (c *Config) VoicemailsPath() string {
	return filepath.Join(c.DataDir, VoicemailsDir)
}

// BackupsPath returns the path to the backups directory
func (c *Config) BackupsPath() string {
	return filepath.Join(c.DataDir, BackupsDir)
}

// EnsureDirectories creates all required data directories
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.DataDir,
		c.RecordingsPath(),
		c.VoicemailsPath(),
		c.BackupsPath(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

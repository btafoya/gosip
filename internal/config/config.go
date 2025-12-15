// Package config provides runtime configuration management for GoSIP
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
	// Enabled enables TLS/SIPS support
	Enabled bool

	// Port for SIPS (default: 5061)
	Port int

	// WSSPort for WebSocket Secure (default: 5081)
	WSSPort int

	// CertMode: "manual" | "acme"
	CertMode string

	// Manual certificate paths (when CertMode = "manual")
	CertFile string
	KeyFile  string
	CAFile   string // Optional CA certificate for client verification

	// ACME/Let's Encrypt settings (when CertMode = "acme")
	ACMEEmail   string
	ACMEDomain  string   // Primary domain for certificate
	ACMEDomains []string // Additional SANs
	ACMECA      string   // "production" | "staging"

	// Cloudflare DNS challenge settings
	CloudflareAPIToken string

	// Client certificate verification
	ClientAuth string // "none" | "request" | "require"

	// Minimum TLS version: "1.2" | "1.3"
	MinVersion string
}

// SRTPConfig holds SRTP-specific configuration (optional)
type SRTPConfig struct {
	Enabled bool
	Profile string // "AES_CM_128_HMAC_SHA1_80" | "AEAD_AES_128_GCM"
}

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

	// TLS configuration
	TLS *TLSConfig

	// SRTP configuration (optional)
	SRTP *SRTPConfig
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

	// Load TLS configuration
	cfg.TLS = loadTLSConfig()

	// Load SRTP configuration
	cfg.SRTP = loadSRTPConfig()

	return cfg
}

// loadTLSConfig loads TLS configuration from environment variables
func loadTLSConfig() *TLSConfig {
	return &TLSConfig{
		Enabled:            getEnvBool("GOSIP_TLS_ENABLED", false),
		Port:               getEnvInt("GOSIP_TLS_PORT", DefaultTLSPort),
		WSSPort:            getEnvInt("GOSIP_TLS_WSS_PORT", DefaultWSSPort),
		CertMode:           getEnv("GOSIP_TLS_CERT_MODE", DefaultCertMode),
		CertFile:           getEnv("GOSIP_TLS_CERT_FILE", ""),
		KeyFile:            getEnv("GOSIP_TLS_KEY_FILE", ""),
		CAFile:             getEnv("GOSIP_TLS_CA_FILE", ""),
		ACMEEmail:          getEnv("GOSIP_ACME_EMAIL", ""),
		ACMEDomain:         getEnv("GOSIP_ACME_DOMAIN", ""),
		ACMECA:             getEnv("GOSIP_ACME_CA", DefaultACMECA),
		CloudflareAPIToken: getEnv("CLOUDFLARE_DNS_API_TOKEN", ""),
		ClientAuth:         getEnv("GOSIP_TLS_CLIENT_AUTH", "none"),
		MinVersion:         getEnv("GOSIP_TLS_MIN_VERSION", DefaultTLSMinVersion),
	}
}

// loadSRTPConfig loads SRTP configuration from environment variables
func loadSRTPConfig() *SRTPConfig {
	return &SRTPConfig{
		Enabled: getEnvBool("GOSIP_SRTP_ENABLED", false),
		Profile: getEnv("GOSIP_SRTP_PROFILE", DefaultSRTPProfile),
	}
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

// CertsPath returns the path to the certificates directory
func (c *Config) CertsPath() string {
	return filepath.Join(c.DataDir, CertsDir)
}

// EnsureDirectories creates all required data directories
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.DataDir,
		c.RecordingsPath(),
		c.VoicemailsPath(),
		c.BackupsPath(),
		c.CertsPath(),
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

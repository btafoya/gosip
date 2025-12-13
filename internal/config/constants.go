// Package config provides configuration constants and settings for GoSIP
package config

import "time"

// Performance SLAs - P0 requirements
const (
	SIPRegistrationTimeout = 500 * time.Millisecond // < 500ms
	CallSetupTimeout       = 2 * time.Second        // < 2 seconds
	APIGetTimeout          = 200 * time.Millisecond // < 200ms (95th percentile)
	APIPostTimeout         = 500 * time.Millisecond // < 500ms (95th percentile)
	MaxConcurrentCalls     = 5
	SystemStartupTimeout   = 30 * time.Second
)

// Security constants - P0 requirements
const (
	MaxFailedLoginAttempts   = 5
	LoginLockoutDuration     = 15 * time.Minute
	SessionDuration          = 24 * time.Hour
	SessionRefreshOnActivity = true
	SpamScoreThreshold       = 0.7 // Calls > 0.7 blocked
)

// Voicemail settings - P0 requirements
const (
	VoicemailRingTimeout    = 30 * time.Second
	VoicemailMaxLength      = 180 * time.Second // 3 minutes
	VoicemailMinLength      = 3 * time.Second   // Shorter discarded
	VoicemailSilenceTimeout = 10 * time.Second
)

// API pagination defaults
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Retry/Recovery settings - P0 requirements
const (
	TwilioMaxRetries   = 3
	TwilioRetryBackoff = true // Exponential backoff
	EmailMaxRetries    = 3
	EmailRetryWindow   = 1 * time.Hour
	GotifyMaxRetries   = 3
)

// SIP Server defaults
const (
	DefaultSIPPort      = 5060
	DefaultHTTPPort     = 8080
	DefaultUserAgent    = "GoSIP/1.0"
	RegistrationExpires = 3600 // seconds
)

// Database paths
const (
	DefaultDataDir    = "./data"
	DefaultDBFile     = "gosip.db"
	RecordingsDir     = "recordings"
	VoicemailsDir     = "voicemails"
	BackupsDir        = "backups"
)

// Package models defines the domain models for GoSIP
package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// User represents an admin or regular user account
type User struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never serialize password hash
	Role         string     `json:"role"` // "admin" or "user"
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
}

// Device represents a registered SIP device (phone, softphone, etc.)
type Device struct {
	ID                 int64      `json:"id"`
	UserID             *int64     `json:"user_id,omitempty"`
	Name               string     `json:"name"`
	Username           string     `json:"username"`
	PasswordHash       string     `json:"-"`
	DeviceType         string     `json:"device_type"` // "grandstream", "softphone", "webrtc"
	RecordingEnabled   bool       `json:"recording_enabled"`
	CreatedAt          time.Time  `json:"created_at"`
	// Provisioning fields
	MACAddress         *string    `json:"mac_address,omitempty"`
	Vendor             *string    `json:"vendor,omitempty"`
	Model              *string    `json:"model,omitempty"`
	FirmwareVersion    *string    `json:"firmware_version,omitempty"`
	ProvisioningStatus string     `json:"provisioning_status"` // "pending", "provisioned", "failed", "unknown"
	LastConfigFetch    *time.Time `json:"last_config_fetch,omitempty"`
	LastRegistration   *time.Time `json:"last_registration,omitempty"`
	ConfigTemplate     *string    `json:"config_template,omitempty"`
}

// Registration represents an active SIP registration
type Registration struct {
	ID        int64     `json:"id"`
	DeviceID  int64     `json:"device_id"`
	Contact   string    `json:"contact"`
	ExpiresAt time.Time `json:"expires_at"`
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	Transport string    `json:"transport"` // "udp", "tcp", "tls", "ws", "wss"
	LastSeen  time.Time `json:"last_seen"`
}

// DID represents a phone number (Direct Inward Dial)
type DID struct {
	ID           int64  `json:"id"`
	Number       string `json:"number"`
	TwilioSID    string `json:"twilio_sid,omitempty"`
	Name         string `json:"name,omitempty"`
	SMSEnabled   bool   `json:"sms_enabled"`
	VoiceEnabled bool   `json:"voice_enabled"`
}

// Route represents a call routing rule
type Route struct {
	ID            int64           `json:"id"`
	DIDID         *int64          `json:"did_id,omitempty"`
	Priority      int             `json:"priority"`
	Name          string          `json:"name"`
	ConditionType string          `json:"condition_type"` // "time", "callerid", "default"
	ConditionData json.RawMessage `json:"condition_data,omitempty"`
	ActionType    string          `json:"action_type"` // "ring", "forward", "voicemail", "reject"
	ActionData    json.RawMessage `json:"action_data,omitempty"`
	Enabled       bool            `json:"enabled"`
}

// TimeCondition represents time-based routing conditions
type TimeCondition struct {
	Days      []int  `json:"days"`       // 0=Sunday, 6=Saturday
	StartTime string `json:"start_time"` // "09:00"
	EndTime   string `json:"end_time"`   // "17:00"
	Timezone  string `json:"timezone"`   // "America/Los_Angeles"
}

// CallerIDCondition represents caller ID based routing conditions
type CallerIDCondition struct {
	Numbers []string `json:"numbers"` // List of numbers or patterns
	Type    string   `json:"type"`    // "vip", "block", "match"
}

// RingAction represents action data for ringing devices
type RingAction struct {
	DeviceIDs []int64 `json:"device_ids"`
	Timeout   int     `json:"timeout"` // seconds
	Fallback  string  `json:"fallback"` // "voicemail", "forward", "reject"
}

// ForwardAction represents action data for call forwarding
type ForwardAction struct {
	Number  string `json:"number"`
	Timeout int    `json:"timeout"`
}

// BlocklistEntry represents a blocked phone number or pattern
type BlocklistEntry struct {
	ID          int64     `json:"id"`
	Pattern     string    `json:"pattern"`
	PatternType string    `json:"pattern_type"` // "exact", "prefix", "regex"
	Reason      string    `json:"reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CDR represents a Call Detail Record
type CDR struct {
	ID           int64          `json:"id"`
	CallSID      string         `json:"call_sid,omitempty"`
	Direction    string         `json:"direction"` // "inbound", "outbound"
	FromNumber   string         `json:"from_number"`
	ToNumber     string         `json:"to_number"`
	DIDID        *int64         `json:"did_id,omitempty"`
	DeviceID     *int64         `json:"device_id,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
	AnsweredAt   *time.Time     `json:"answered_at,omitempty"`
	EndedAt      *time.Time     `json:"ended_at,omitempty"`
	Duration     int            `json:"duration"` // seconds
	Disposition  string         `json:"disposition"` // "answered", "voicemail", "missed", "blocked", "busy", "failed"
	RecordingURL sql.NullString `json:"recording_url,omitempty"`
	SpamScore    *float64       `json:"spam_score,omitempty"`
}

// Voicemail represents a voicemail message
type Voicemail struct {
	ID         int64     `json:"id"`
	CDRID      *int64    `json:"cdr_id,omitempty"`
	UserID     *int64    `json:"user_id,omitempty"`
	FromNumber string    `json:"from_number"`
	AudioURL   string    `json:"audio_url,omitempty"`
	Transcript string    `json:"transcript,omitempty"`
	Duration   int       `json:"duration"` // seconds
	IsRead     bool      `json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}

// Message represents an SMS/MMS message
type Message struct {
	ID          int64           `json:"id"`
	MessageSID  string          `json:"message_sid,omitempty"`
	Direction   string          `json:"direction"` // "inbound", "outbound"
	FromNumber  string          `json:"from_number"`
	ToNumber    string          `json:"to_number"`
	DIDID       *int64          `json:"did_id,omitempty"`
	Body        string          `json:"body,omitempty"`
	MediaURLs   json.RawMessage `json:"media_urls,omitempty"`
	Status      string          `json:"status,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	IsRead      bool            `json:"is_read"`
}

// AutoReply represents an automatic reply rule
type AutoReply struct {
	ID          int64           `json:"id"`
	DIDID       *int64          `json:"did_id,omitempty"`
	TriggerType string          `json:"trigger_type"` // "dnd", "after_hours", "keyword"
	TriggerData json.RawMessage `json:"trigger_data,omitempty"`
	ReplyText   string          `json:"reply_text"`
	Enabled     bool            `json:"enabled"`
}

// SystemConfig represents a key-value configuration entry
type SystemConfig struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProvisioningToken represents a tokened URL for device auto-provisioning
type ProvisioningToken struct {
	ID            int64      `json:"id"`
	Token         string     `json:"token"`
	DeviceID      int64      `json:"device_id"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	Revoked       bool       `json:"revoked"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	UsedCount     int        `json:"used_count"`
	MaxUses       int        `json:"max_uses"`
	IPRestriction *string    `json:"ip_restriction,omitempty"`
	CreatedBy     *int64     `json:"created_by,omitempty"`
}

// ProvisioningProfile represents a vendor/model configuration template
type ProvisioningProfile struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	Vendor         string          `json:"vendor"`
	Model          *string         `json:"model,omitempty"`
	Description    *string         `json:"description,omitempty"`
	ConfigTemplate string          `json:"config_template"`
	Variables      json.RawMessage `json:"variables,omitempty"`
	IsDefault      bool            `json:"is_default"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// DeviceEvent represents an operational event for a device
type DeviceEvent struct {
	ID        int64           `json:"id"`
	DeviceID  int64           `json:"device_id"`
	EventType string          `json:"event_type"` // "config_fetch", "registration", "provision_complete", etc.
	EventData json.RawMessage `json:"event_data,omitempty"`
	IPAddress *string         `json:"ip_address,omitempty"`
	UserAgent *string         `json:"user_agent,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// ProvisioningRequest represents a request to provision a device
type ProvisioningRequest struct {
	DeviceName   string `json:"device_name"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DeviceType   string `json:"device_type"`
	Vendor       string `json:"vendor"`
	Model        string `json:"model,omitempty"`
	MACAddress   string `json:"mac_address,omitempty"`
	ProfileID    *int64 `json:"profile_id,omitempty"`
	UserID       *int64 `json:"user_id,omitempty"`
	GenerateURL  bool   `json:"generate_url"`
	URLExpiresIn int    `json:"url_expires_in"` // seconds
}

// ProvisioningResponse represents the response from provisioning a device
type ProvisioningResponse struct {
	Device           *Device `json:"device"`
	ProvisioningURL  string  `json:"provisioning_url,omitempty"`
	Token            string  `json:"token,omitempty"`
	TokenExpiresAt   string  `json:"token_expires_at,omitempty"`
	SIPServer        string  `json:"sip_server"`
	SIPPort          int     `json:"sip_port"`
	ConfigInstructions string `json:"config_instructions,omitempty"`
}

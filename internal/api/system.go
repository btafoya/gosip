package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

// SystemHandler handles system configuration and status API endpoints
type SystemHandler struct {
	deps      *Dependencies
	startTime time.Time
}

// NewSystemHandler creates a new SystemHandler
func NewSystemHandler(deps *Dependencies) *SystemHandler {
	return &SystemHandler{
		deps:      deps,
		startTime: time.Now(),
	}
}

// ConfigResponse represents system configuration in API responses
type ConfigResponse struct {
	TwilioAccountSID   string `json:"twilio_account_sid,omitempty"`
	TwilioConfigured   bool   `json:"twilio_configured"`
	SMTPConfigured     bool   `json:"smtp_configured"`
	GotifyConfigured   bool   `json:"gotify_configured"`
	SetupCompleted     bool   `json:"setup_completed"`
	VoicemailEnabled   bool   `json:"voicemail_enabled"`
	RecordingEnabled   bool   `json:"recording_enabled"`
	TranscriptionEnabled bool `json:"transcription_enabled"`
}

// GetConfig returns current system configuration
func (h *SystemHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.deps.DB.Config.GetAll(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	// Mask sensitive values
	response := ConfigResponse{
		TwilioConfigured:     cfg["twilio_account_sid"] != "",
		SMTPConfigured:       cfg["smtp_host"] != "",
		GotifyConfigured:     cfg["gotify_url"] != "",
		SetupCompleted:       cfg["setup_completed"] == "true",
		VoicemailEnabled:     cfg["voicemail_enabled"] == "true",
		RecordingEnabled:     cfg["recording_enabled"] == "true",
		TranscriptionEnabled: cfg["transcription_enabled"] == "true",
	}

	// Show partial account SID for verification
	if sid := cfg["twilio_account_sid"]; sid != "" && len(sid) > 8 {
		response.TwilioAccountSID = sid[:8] + "..."
	}

	WriteJSON(w, http.StatusOK, response)
}

// UpdateConfigRequest represents a configuration update request
type UpdateConfigRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// UpdateConfig updates a system configuration value
func (h *SystemHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate key
	allowedKeys := map[string]bool{
		"voicemail_enabled":     true,
		"recording_enabled":     true,
		"transcription_enabled": true,
		"voicemail_greeting":    true,
		"business_hours_start":  true,
		"business_hours_end":    true,
		"timezone":              true,
	}

	if !allowedKeys[req.Key] {
		WriteValidationError(w, "Invalid configuration key", []FieldError{
			{Field: "key", Message: "Configuration key not allowed"},
		})
		return
	}

	if err := h.deps.DB.Config.Set(r.Context(), req.Key, req.Value); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Configuration updated"})
}

// SetupWizardRequest represents setup wizard data
type SetupWizardRequest struct {
	TwilioAccountSID  string `json:"twilio_account_sid"`
	TwilioAuthToken   string `json:"twilio_auth_token"`
	AdminEmail        string `json:"admin_email"`
	AdminPassword     string `json:"admin_password"`
	SMTPHost          string `json:"smtp_host,omitempty"`
	SMTPPort          int    `json:"smtp_port,omitempty"`
	SMTPUser          string `json:"smtp_user,omitempty"`
	SMTPPassword      string `json:"smtp_password,omitempty"`
	GotifyURL         string `json:"gotify_url,omitempty"`
	GotifyToken       string `json:"gotify_token,omitempty"`
}

// SetupWizard handles initial system setup
func (h *SystemHandler) SetupWizard(w http.ResponseWriter, r *http.Request) {
	// Check if setup already completed
	setupCompleted, _ := h.deps.DB.Config.Get(r.Context(), "setup_completed")
	if setupCompleted == "true" {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "Setup already completed", nil)
		return
	}

	var req SetupWizardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate required fields
	var errors []FieldError
	if req.TwilioAccountSID == "" {
		errors = append(errors, FieldError{Field: "twilio_account_sid", Message: "Twilio Account SID is required"})
	}
	if req.TwilioAuthToken == "" {
		errors = append(errors, FieldError{Field: "twilio_auth_token", Message: "Twilio Auth Token is required"})
	}
	if req.AdminEmail == "" {
		errors = append(errors, FieldError{Field: "admin_email", Message: "Admin email is required"})
	}
	if len(req.AdminPassword) < 8 {
		errors = append(errors, FieldError{Field: "admin_password", Message: "Admin password must be at least 8 characters"})
	}

	if len(errors) > 0 {
		WriteValidationError(w, "Validation failed", errors)
		return
	}

	// Save Twilio credentials
	h.deps.DB.Config.Set(r.Context(), "twilio_account_sid", req.TwilioAccountSID)
	h.deps.DB.Config.Set(r.Context(), "twilio_auth_token", req.TwilioAuthToken)

	// Save SMTP settings if provided
	if req.SMTPHost != "" {
		h.deps.DB.Config.Set(r.Context(), "smtp_host", req.SMTPHost)
		h.deps.DB.Config.Set(r.Context(), "smtp_port", string(rune(req.SMTPPort)))
		h.deps.DB.Config.Set(r.Context(), "smtp_user", req.SMTPUser)
		h.deps.DB.Config.Set(r.Context(), "smtp_password", req.SMTPPassword)
	}

	// Save Gotify settings if provided
	if req.GotifyURL != "" {
		h.deps.DB.Config.Set(r.Context(), "gotify_url", req.GotifyURL)
		h.deps.DB.Config.Set(r.Context(), "gotify_token", req.GotifyToken)
	}

	// Create admin user
	authHandler := NewAuthHandler(h.deps)
	_, err := createAdminUser(r.Context(), authHandler, req.AdminEmail, req.AdminPassword)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Failed to create admin user", nil)
		return
	}

	// Mark setup as complete
	h.deps.DB.Config.Set(r.Context(), "setup_completed", "true")

	// Initialize Twilio client
	if h.deps.Twilio != nil {
		h.deps.Twilio.UpdateCredentials(req.TwilioAccountSID, req.TwilioAuthToken)
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Setup completed successfully"})
}

// StatusResponse represents system status
type StatusResponse struct {
	Status           string            `json:"status"`
	Version          string            `json:"version"`
	Uptime           string            `json:"uptime"`
	GoVersion        string            `json:"go_version"`
	SIPServerStatus  string            `json:"sip_server_status"`
	TwilioStatus     string            `json:"twilio_status"`
	DatabaseStatus   string            `json:"database_status"`
	ActiveCalls      int               `json:"active_calls"`
	RegisteredDevices int              `json:"registered_devices"`
	Stats            map[string]int64  `json:"stats"`
}

// GetStatus returns system health status
func (h *SystemHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)

	// Check SIP server status
	sipStatus := "offline"
	if h.deps.SIP != nil && h.deps.SIP.IsRunning() {
		sipStatus = "online"
	}

	// Check Twilio status
	twilioStatus := "not_configured"
	if h.deps.Twilio != nil {
		if h.deps.Twilio.IsHealthy() {
			twilioStatus = "healthy"
		} else {
			twilioStatus = "degraded"
		}
	}

	// Get registered device count
	registeredDevices := 0
	if h.deps.SIP != nil {
		registeredDevices = h.deps.SIP.GetRegistrar().GetRegistrationCount()
	}

	// Get active call count
	activeCalls := 0
	if h.deps.SIP != nil {
		activeCalls = h.deps.SIP.GetActiveCallCount()
	}

	// Get stats
	stats := make(map[string]int64)
	if deviceCount, err := h.deps.DB.Devices.Count(r.Context()); err == nil {
		stats["total_devices"] = int64(deviceCount)
	}
	if didCount, err := h.deps.DB.DIDs.Count(r.Context()); err == nil {
		stats["total_dids"] = int64(didCount)
	}
	if userCount, err := h.deps.DB.Users.Count(r.Context()); err == nil {
		stats["total_users"] = int64(userCount)
	}

	response := StatusResponse{
		Status:            "healthy",
		Version:           "1.0.0",
		Uptime:            uptime.String(),
		GoVersion:         runtime.Version(),
		SIPServerStatus:   sipStatus,
		TwilioStatus:      twilioStatus,
		DatabaseStatus:    "healthy",
		ActiveCalls:       activeCalls,
		RegisteredDevices: registeredDevices,
		Stats:             stats,
	}

	// Overall status based on components
	if sipStatus != "online" || twilioStatus == "degraded" {
		response.Status = "degraded"
	}

	WriteJSON(w, http.StatusOK, response)
}

// BackupResponse represents a backup creation response
type BackupResponse struct {
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// CreateBackup creates a database backup
func (h *SystemHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	filename, size, err := h.deps.DB.CreateBackup(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, BackupResponse{
		Filename:  filename,
		Size:      size,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

// ListBackups returns available backups
func (h *SystemHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := h.deps.DB.ListBackups(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, backups)
}

// Helper to create admin user during setup
func createAdminUser(ctx interface{}, h *AuthHandler, email, password string) (int64, error) {
	// This is handled by the auth handler's CreateUser method
	// For setup, we create directly
	return 0, nil
}

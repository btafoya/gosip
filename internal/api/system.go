package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/btafoya/gosip/internal/models"
	"golang.org/x/crypto/bcrypt"
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
	TwilioAccountSID     string `json:"twilio_account_sid,omitempty"`
	TwilioConfigured     bool   `json:"twilio_configured"`
	SMTPHost             string `json:"smtp_host,omitempty"`
	SMTPPort             int    `json:"smtp_port,omitempty"`
	SMTPUser             string `json:"smtp_user,omitempty"`
	SMTPConfigured       bool   `json:"smtp_configured"`
	GotifyURL            string `json:"gotify_url,omitempty"`
	GotifyConfigured     bool   `json:"gotify_configured"`
	SetupCompleted       bool   `json:"setup_completed"`
	VoicemailEnabled     bool   `json:"voicemail_enabled"`
	VoicemailGreeting    string `json:"voicemail_greeting,omitempty"`
	RecordingEnabled     bool   `json:"recording_enabled"`
	TranscriptionEnabled bool   `json:"transcription_enabled"`
	Timezone             string `json:"timezone,omitempty"`
}

// GetConfig returns current system configuration
func (h *SystemHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	configs, err := h.deps.DB.Config.GetAll(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	// Convert slice to map for easier access
	cfg := make(map[string]string)
	for _, c := range configs {
		cfg[c.Key] = c.Value
	}

	// Parse SMTP port
	smtpPort := 587
	if portStr := cfg["smtp_port"]; portStr != "" {
		fmt.Sscanf(portStr, "%d", &smtpPort)
	}

	// Build response with actual values (this endpoint is admin-only)
	response := ConfigResponse{
		TwilioAccountSID:     cfg["twilio_account_sid"],
		TwilioConfigured:     cfg["twilio_account_sid"] != "",
		SMTPHost:             cfg["smtp_host"],
		SMTPPort:             smtpPort,
		SMTPUser:             cfg["smtp_user"],
		SMTPConfigured:       cfg["smtp_host"] != "",
		GotifyURL:            cfg["gotify_url"],
		GotifyConfigured:     cfg["gotify_url"] != "",
		SetupCompleted:       cfg["setup_completed"] == "true",
		VoicemailEnabled:     cfg["voicemail_enabled"] == "true",
		VoicemailGreeting:    cfg["voicemail_greeting"],
		RecordingEnabled:     cfg["recording_enabled"] == "true",
		TranscriptionEnabled: cfg["transcription_enabled"] == "true",
		Timezone:             cfg["timezone"],
	}

	// Default timezone if not set
	if response.Timezone == "" {
		response.Timezone = "America/New_York"
	}

	WriteJSON(w, http.StatusOK, response)
}

// UpdateConfigRequest represents a bulk configuration update request
type UpdateConfigRequest struct {
	TwilioAccountSID  string `json:"twilio_account_sid,omitempty"`
	TwilioAuthToken   string `json:"twilio_auth_token,omitempty"`
	SMTPHost          string `json:"smtp_host,omitempty"`
	SMTPPort          int    `json:"smtp_port,omitempty"`
	SMTPUser          string `json:"smtp_user,omitempty"`
	SMTPPassword      string `json:"smtp_password,omitempty"`
	GotifyURL         string `json:"gotify_url,omitempty"`
	GotifyToken       string `json:"gotify_token,omitempty"`
	VoicemailGreeting string `json:"voicemail_greeting,omitempty"`
	Timezone          string `json:"timezone,omitempty"`
}

// UpdateConfig updates system configuration values
func (h *SystemHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	ctx := r.Context()

	// Update Twilio settings (only if provided)
	if req.TwilioAccountSID != "" {
		h.deps.DB.Config.Set(ctx, "twilio_account_sid", req.TwilioAccountSID)
	}
	if req.TwilioAuthToken != "" {
		h.deps.DB.Config.Set(ctx, "twilio_auth_token", req.TwilioAuthToken)
		// Update Twilio client with new credentials
		if h.deps.Twilio != nil && req.TwilioAccountSID != "" {
			h.deps.Twilio.UpdateCredentials(req.TwilioAccountSID, req.TwilioAuthToken)
		}
	}

	// Update SMTP settings
	if req.SMTPHost != "" {
		h.deps.DB.Config.Set(ctx, "smtp_host", req.SMTPHost)
	}
	if req.SMTPPort > 0 {
		h.deps.DB.Config.Set(ctx, "smtp_port", fmt.Sprintf("%d", req.SMTPPort))
	}
	if req.SMTPUser != "" {
		h.deps.DB.Config.Set(ctx, "smtp_user", req.SMTPUser)
	}
	if req.SMTPPassword != "" {
		h.deps.DB.Config.Set(ctx, "smtp_password", req.SMTPPassword)
	}

	// Update Gotify settings
	if req.GotifyURL != "" {
		h.deps.DB.Config.Set(ctx, "gotify_url", req.GotifyURL)
	}
	if req.GotifyToken != "" {
		h.deps.DB.Config.Set(ctx, "gotify_token", req.GotifyToken)
	}

	// Update general settings
	if req.VoicemailGreeting != "" {
		h.deps.DB.Config.Set(ctx, "voicemail_greeting", req.VoicemailGreeting)
	}
	if req.Timezone != "" {
		h.deps.DB.Config.Set(ctx, "timezone", req.Timezone)
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

// GetBackup returns information about a specific backup
func (h *SystemHandler) GetBackup(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		WriteValidationError(w, "Filename is required", []FieldError{{Field: "filename", Message: "Filename is required"}})
		return
	}

	backup, err := h.deps.DB.GetBackup(r.Context(), filename)
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, err.Error(), nil)
		return
	}

	WriteJSON(w, http.StatusOK, backup)
}

// VerifyBackup checks the integrity of a backup file
func (h *SystemHandler) VerifyBackup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.Filename == "" {
		WriteValidationError(w, "Filename is required", []FieldError{{Field: "filename", Message: "Filename is required"}})
		return
	}

	if err := h.deps.DB.VerifyBackup(r.Context(), req.Filename); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error(), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"filename": req.Filename,
		"valid":    true,
		"message":  "Backup integrity verified successfully",
	})
}

// DeleteBackup removes a backup file
func (h *SystemHandler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.Filename == "" {
		WriteValidationError(w, "Filename is required", []FieldError{{Field: "filename", Message: "Filename is required"}})
		return
	}

	if err := h.deps.DB.DeleteBackup(r.Context(), req.Filename); err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, err.Error(), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Backup deleted successfully"})
}

// CleanOldBackups removes backup files older than the specified retention period
func (h *SystemHandler) CleanOldBackups(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Default to 30 days if not specified
	if req.RetentionDays <= 0 {
		req.RetentionDays = 30
	}

	deletedCount, err := h.deps.DB.CleanOldBackups(r.Context(), req.RetentionDays)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error(), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":        "Old backups cleaned successfully",
		"deleted_count":  deletedCount,
		"retention_days": req.RetentionDays,
	})
}

// GetSetupStatus returns whether setup is completed
func (h *SystemHandler) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	setupCompleted, _ := h.deps.DB.Config.Get(r.Context(), "setup_completed")
	WriteJSON(w, http.StatusOK, map[string]bool{
		"setup_completed": setupCompleted == "true",
	})
}

// CompleteSetup handles initial system setup (same as SetupWizard)
func (h *SystemHandler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	h.SetupWizard(w, r)
}

// RestoreBackup restores system from a backup
func (h *SystemHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.Filename == "" {
		WriteValidationError(w, "Filename is required", []FieldError{{Field: "filename", Message: "Filename is required"}})
		return
	}

	if err := h.deps.DB.RestoreBackup(r.Context(), req.Filename); err != nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Failed to restore backup", nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Backup restored successfully"})
}

// ToggleDND toggles Do Not Disturb mode
func (h *SystemHandler) ToggleDND(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	value := "false"
	if req.Enabled {
		value = "true"
	}

	if err := h.deps.DB.Config.Set(r.Context(), "dnd_enabled", value); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"dnd_enabled": req.Enabled})
}

// Helper to create admin user during setup
func createAdminUser(ctx context.Context, h *AuthHandler, email, password string) (int64, error) {
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		CreatedAt:    time.Now(),
	}

	if err := h.deps.DB.Users.Create(ctx, user); err != nil {
		return 0, err
	}

	return user.ID, nil
}

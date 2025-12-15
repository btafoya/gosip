package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
	qrcode "github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"golang.org/x/crypto/bcrypt"
)

// Helper functions for consistent API responses
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	WriteJSON(w, statusCode, data)
}

func respondError(w http.ResponseWriter, statusCode int, code, message string) {
	WriteError(w, statusCode, code, message, nil)
}

// ProvisioningHandler handles provisioning-related endpoints
type ProvisioningHandler struct {
	deps *Dependencies
}

// NewProvisioningHandler creates a new ProvisioningHandler
func NewProvisioningHandler(deps *Dependencies) *ProvisioningHandler {
	return &ProvisioningHandler{deps: deps}
}

// ProvisionDevice handles device provisioning request
func (h *ProvisioningHandler) ProvisionDevice(w http.ResponseWriter, r *http.Request) {
	var req models.ProvisioningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Validate required fields
	if req.DeviceName == "" || req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Device name, username, and password are required")
		return
	}

	// Check if username already exists
	existing, _ := h.deps.DB.Devices.GetByUsername(r.Context(), req.Username)
	if existing != nil {
		respondError(w, http.StatusConflict, "USERNAME_EXISTS", "A device with this username already exists")
		return
	}

	// Hash the password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "PASSWORD_ERROR", "Failed to process password")
		return
	}

	// Create the device
	device := &models.Device{
		Name:               req.DeviceName,
		Username:           req.Username,
		PasswordHash:       string(passwordHash),
		DeviceType:         req.DeviceType,
		UserID:             req.UserID,
		MACAddress:         nilIfEmpty(req.MACAddress),
		Vendor:             nilIfEmpty(req.Vendor),
		Model:              nilIfEmpty(req.Model),
		ProvisioningStatus: "pending",
	}

	if err := h.deps.DB.Devices.Create(r.Context(), device); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to create device")
		return
	}

	// Log the provisioning event
	h.deps.DB.DeviceEvents.LogEvent(r.Context(), device.ID, "provision_start", map[string]interface{}{
		"vendor": req.Vendor,
		"model":  req.Model,
	}, r.RemoteAddr, r.UserAgent())

	// Build response
	response := models.ProvisioningResponse{
		Device:    device,
		SIPServer: h.deps.Config.SIPDomain,
		SIPPort:   h.deps.Config.SIPPort,
	}

	// Generate provisioning URL if requested
	if req.GenerateURL {
		expiresIn := req.URLExpiresIn
		if expiresIn <= 0 {
			expiresIn = 3600 // Default 1 hour
		}

		token := &models.ProvisioningToken{
			DeviceID:  device.ID,
			ExpiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
			MaxUses:   5,
		}

		// Get user ID from context if available
		if userID := getUserIDFromContext(r.Context()); userID > 0 {
			token.CreatedBy = &userID
		}

		if err := h.deps.DB.ProvisioningTokens.Create(r.Context(), token); err != nil {
			// Don't fail the whole request, just log the error
			fmt.Printf("Failed to create provisioning token: %v\n", err)
		} else {
			response.Token = token.Token
			response.TokenExpiresAt = token.ExpiresAt.Format(time.RFC3339)
			response.ProvisioningURL = fmt.Sprintf("https://%s/provision/%s", h.deps.Config.SIPDomain, token.Token)
		}
	}

	// Generate config instructions based on vendor
	response.ConfigInstructions = h.generateConfigInstructions(req.Vendor, device, response.SIPServer, response.SIPPort)

	respondJSON(w, http.StatusCreated, response)
}

// GetDeviceConfig serves the provisioning configuration for a device via token
func (h *ProvisioningHandler) GetDeviceConfig(w http.ResponseWriter, r *http.Request) {
	tokenStr := chi.URLParam(r, "token")
	if tokenStr == "" {
		respondError(w, http.StatusBadRequest, "MISSING_TOKEN", "Provisioning token is required")
		return
	}

	// Validate and use the token
	token, err := h.deps.DB.ProvisioningTokens.ValidateAndUse(r.Context(), tokenStr, r.RemoteAddr)
	if err != nil {
		switch err {
		case db.ErrTokenNotFound:
			respondError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Invalid provisioning token")
		case db.ErrTokenExpired:
			respondError(w, http.StatusGone, "TOKEN_EXPIRED", "Provisioning token has expired")
		case db.ErrTokenRevoked:
			respondError(w, http.StatusForbidden, "TOKEN_REVOKED", "Provisioning token has been revoked")
		case db.ErrTokenMaxUses:
			respondError(w, http.StatusForbidden, "TOKEN_MAX_USES", "Provisioning token usage limit exceeded")
		default:
			respondError(w, http.StatusForbidden, "TOKEN_ERROR", err.Error())
		}
		return
	}

	// Get the device
	device, err := h.deps.DB.Devices.GetByID(r.Context(), token.DeviceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "Device not found")
		return
	}

	// Log the config fetch event
	h.deps.DB.DeviceEvents.LogEvent(r.Context(), device.ID, "config_fetch", map[string]interface{}{
		"token_id": token.ID,
	}, r.RemoteAddr, r.UserAgent())

	// Update device's last config fetch time
	h.deps.DB.Devices.UpdateLastConfigFetch(r.Context(), device.ID)

	// Get the provisioning profile
	var profile *models.ProvisioningProfile
	if device.Vendor != nil {
		if device.Model != nil {
			profile, _ = h.deps.DB.ProvisioningProfiles.GetByVendorModel(r.Context(), *device.Vendor, *device.Model)
		}
		if profile == nil {
			profile, _ = h.deps.DB.ProvisioningProfiles.GetDefaultForVendor(r.Context(), *device.Vendor)
		}
	}

	if profile == nil {
		respondError(w, http.StatusNotFound, "NO_PROFILE", "No provisioning profile found for this device")
		return
	}

	// Generate the config from template
	config, err := h.generateConfig(profile, device)
	if err != nil {
		h.deps.DB.DeviceEvents.LogEvent(r.Context(), device.ID, "config_fetch_failed", map[string]interface{}{
			"error": err.Error(),
		}, r.RemoteAddr, r.UserAgent())
		respondError(w, http.StatusInternalServerError, "CONFIG_ERROR", "Failed to generate configuration")
		return
	}

	// Update provisioning status
	h.deps.DB.Devices.UpdateProvisioningStatus(r.Context(), device.ID, "provisioned")
	h.deps.DB.DeviceEvents.LogEvent(r.Context(), device.ID, "provision_complete", nil, r.RemoteAddr, r.UserAgent())

	// Determine content type based on profile vendor
	contentType := "application/xml"
	if device.Vendor != nil {
		switch *device.Vendor {
		case "grandstream":
			contentType = "application/xml"
		case "linphone":
			// Linphone expects XML with specific content type
			contentType = "application/xml; charset=utf-8"
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(config))
}

// generateConfig generates a device configuration from a template
func (h *ProvisioningHandler) generateConfig(profile *models.ProvisioningProfile, device *models.Device) (string, error) {
	// Parse the template
	tmpl, err := template.New("config").Parse(profile.ConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare template variables
	vars := map[string]interface{}{
		"SIPServer":     h.deps.Config.SIPDomain,
		"SIPPort":       strconv.Itoa(h.deps.Config.SIPPort),
		"AuthID":        device.Username,
		"AuthPassword":  "", // We don't store plaintext passwords, device needs to be configured manually for password
		"DisplayName":   device.Name,
		"Username":      device.Username,
		"STUNServer":    "stun.l.google.com:19302",
		"NTPServer":     "pool.ntp.org",
		"Timezone":      "America/New_York",
		"AdminPassword": "", // Should be set by user
	}

	// Merge with profile variables if any
	if profile.Variables != nil {
		var profileVars map[string]interface{}
		if err := json.Unmarshal(profile.Variables, &profileVars); err == nil {
			for k, v := range profileVars {
				if _, exists := vars[k]; !exists || v != "" {
					vars[k] = v
				}
			}
		}
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// generateConfigInstructions generates setup instructions for a device
func (h *ProvisioningHandler) generateConfigInstructions(vendor string, device *models.Device, sipServer string, sipPort int) string {
	switch strings.ToLower(vendor) {
	case "grandstream":
		return fmt.Sprintf(`Grandstream Phone Configuration:
1. Access phone web interface (typically http://phone-ip-address)
2. Go to Accounts → Account 1
3. Configure these settings:
   - SIP Server: %s
   - SIP Port: %d
   - SIP User ID: %s
   - Authenticate ID: %s
   - Authenticate Password: [your configured password]
   - Name: %s
4. Save and reboot the phone

Or use the provisioning URL to auto-configure.`, sipServer, sipPort, device.Username, device.Username, device.Name)

	case "linphone":
		return fmt.Sprintf(`Linphone Configuration:

Option 1 - Remote Provisioning (Recommended):
1. Generate a provisioning URL from the Provisioning page
2. In Linphone, go to Settings → Advanced → Remote Provisioning
3. Enter the provisioning URL and tap "Fetch and Apply"
4. Or scan the QR code with the Linphone app

Option 2 - Manual Configuration:
1. Open Linphone and go to Settings → Account
2. Tap "Add account" or "Use SIP account"
3. Configure:
   - Username: %s
   - SIP Domain: %s
   - Password: [your configured password]
   - Transport: UDP (or TCP/TLS if available)
4. Advanced settings:
   - Display Name: %s
   - Proxy: sip:%s:%d
5. Save and register

Option 3 - QR Code:
1. Generate a provisioning URL with QR code
2. In Linphone, use the QR code scanner
3. Configuration will be applied automatically

Features supported:
- Voice calls (G.711, Opus)
- Message Waiting Indicator (MWI)
- Call hold/transfer
- NAT traversal (STUN/ICE)`, device.Username, sipServer, device.Name, sipServer, sipPort)

	case "softphone":
		return fmt.Sprintf(`Softphone Configuration:
1. Add a new SIP account
2. Configure:
   - Domain/Server: %s
   - Port: %d
   - Username: %s
   - Password: [your configured password]
   - Display Name: %s
3. Save and register`, sipServer, sipPort, device.Username, device.Name)

	default:
		return fmt.Sprintf(`SIP Device Configuration:
- SIP Server: %s
- SIP Port: %d
- Username: %s
- Display Name: %s
- Password: [your configured password]`, sipServer, sipPort, device.Username, device.Name)
	}
}

// ListProfiles lists all provisioning profiles
func (h *ProvisioningHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	vendor := r.URL.Query().Get("vendor")
	profiles, err := h.deps.DB.ProvisioningProfiles.List(r.Context(), vendor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch profiles")
		return
	}
	respondJSON(w, http.StatusOK, profiles)
}

// GetProfile gets a single provisioning profile
func (h *ProvisioningHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid profile ID")
		return
	}

	profile, err := h.deps.DB.ProvisioningProfiles.GetByID(r.Context(), id)
	if err == db.ErrProfileNotFound {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Profile not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch profile")
		return
	}

	respondJSON(w, http.StatusOK, profile)
}

// CreateProfile creates a new provisioning profile
func (h *ProvisioningHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var profile models.ProvisioningProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if profile.Name == "" || profile.Vendor == "" || profile.ConfigTemplate == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Name, vendor, and config_template are required")
		return
	}

	if err := h.deps.DB.ProvisioningProfiles.Create(r.Context(), &profile); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to create profile")
		return
	}

	respondJSON(w, http.StatusCreated, profile)
}

// UpdateProfile updates an existing provisioning profile
func (h *ProvisioningHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid profile ID")
		return
	}

	var profile models.ProvisioningProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	profile.ID = id
	if err := h.deps.DB.ProvisioningProfiles.Update(r.Context(), &profile); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to update profile")
		return
	}

	respondJSON(w, http.StatusOK, profile)
}

// DeleteProfile deletes a provisioning profile
func (h *ProvisioningHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid profile ID")
		return
	}

	if err := h.deps.DB.ProvisioningProfiles.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to delete profile")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListVendors lists all available vendors
func (h *ProvisioningHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
	vendors, err := h.deps.DB.ProvisioningProfiles.ListVendors(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch vendors")
		return
	}
	respondJSON(w, http.StatusOK, vendors)
}

// ListTokens lists provisioning tokens for a device
func (h *ProvisioningHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	deviceIDStr := r.URL.Query().Get("device_id")
	if deviceIDStr == "" {
		// List all active tokens
		tokens, err := h.deps.DB.ProvisioningTokens.ListActive(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch tokens")
			return
		}
		respondJSON(w, http.StatusOK, tokens)
		return
	}

	deviceID, err := strconv.ParseInt(deviceIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	tokens, err := h.deps.DB.ProvisioningTokens.ListByDevice(r.Context(), deviceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch tokens")
		return
	}
	respondJSON(w, http.StatusOK, tokens)
}

// CreateToken creates a new provisioning token for a device
func (h *ProvisioningHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID      int64  `json:"device_id"`
		ExpiresIn     int    `json:"expires_in"` // seconds
		MaxUses       int    `json:"max_uses"`
		IPRestriction string `json:"ip_restriction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.DeviceID == 0 {
		respondError(w, http.StatusBadRequest, "MISSING_DEVICE_ID", "Device ID is required")
		return
	}

	// Verify device exists
	_, err := h.deps.DB.Devices.GetByID(r.Context(), req.DeviceID)
	if err == db.ErrDeviceNotFound {
		respondError(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "Device not found")
		return
	}

	expiresIn := req.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // Default 1 hour
	}

	maxUses := req.MaxUses
	if maxUses <= 0 {
		maxUses = 5
	}

	token := &models.ProvisioningToken{
		DeviceID:      req.DeviceID,
		ExpiresAt:     time.Now().Add(time.Duration(expiresIn) * time.Second),
		MaxUses:       maxUses,
		IPRestriction: nilIfEmpty(req.IPRestriction),
	}

	if userID := getUserIDFromContext(r.Context()); userID > 0 {
		token.CreatedBy = &userID
	}

	if err := h.deps.DB.ProvisioningTokens.Create(r.Context(), token); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to create token")
		return
	}

	response := map[string]interface{}{
		"token":            token,
		"provisioning_url": fmt.Sprintf("https://%s/provision/%s", h.deps.Config.SIPDomain, token.Token),
	}

	respondJSON(w, http.StatusCreated, response)
}

// GetTokenQRCode generates a QR code for a provisioning token
func (h *ProvisioningHandler) GetTokenQRCode(w http.ResponseWriter, r *http.Request) {
	tokenStr := chi.URLParam(r, "token")
	if tokenStr == "" {
		respondError(w, http.StatusBadRequest, "MISSING_TOKEN", "Token is required")
		return
	}

	// Validate the token exists and is not expired
	token, err := h.deps.DB.ProvisioningTokens.GetByToken(r.Context(), tokenStr)
	if err != nil {
		respondError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Token not found")
		return
	}

	if token.ExpiresAt.Before(time.Now()) {
		respondError(w, http.StatusGone, "TOKEN_EXPIRED", "Token has expired")
		return
	}

	if token.RevokedAt != nil {
		respondError(w, http.StatusForbidden, "TOKEN_REVOKED", "Token has been revoked")
		return
	}

	// Build the provisioning URL
	provisioningURL := fmt.Sprintf("https://%s/provision/%s", h.deps.Config.SIPDomain, tokenStr)

	// Check format param - can be "png" (image) or "base64" (data URL)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "base64"
	}

	// Generate QR code
	qrData, contentType, err := generateQRCode(provisioningURL, format)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "QR_ERROR", "Failed to generate QR code")
		return
	}

	if format == "png" {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"provision-%s.png\"", tokenStr[:8]))
		w.WriteHeader(http.StatusOK)
		w.Write(qrData)
	} else {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"qr_code":          string(qrData),
			"provisioning_url": provisioningURL,
			"token":            tokenStr,
			"expires_at":       token.ExpiresAt.Format(time.RFC3339),
		})
	}
}

// nopCloser wraps an io.Writer with a no-op Close method
type nopCloser struct {
	*bytes.Buffer
}

func (nopCloser) Close() error { return nil }

// generateQRCode creates a QR code image for the given URL
func generateQRCode(url string, format string) ([]byte, string, error) {
	// Create QR code
	qrc, err := qrcode.New(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create QR code: %w", err)
	}

	// Create a buffer to write the image
	var buf bytes.Buffer

	// Create writer that writes to buffer (with Close implementation)
	writer := standard.NewWithWriter(nopCloser{&buf}, standard.WithQRWidth(10))

	// Save QR code to buffer
	if err := qrc.Save(writer); err != nil {
		return nil, "", fmt.Errorf("failed to save QR code: %w", err)
	}

	if format == "png" {
		return buf.Bytes(), "image/png", nil
	}

	// Return as base64 data URL
	base64Data := base64.StdEncoding.EncodeToString(buf.Bytes())
	dataURL := fmt.Sprintf("data:image/png;base64,%s", base64Data)
	return []byte(dataURL), "text/plain", nil
}

// RevokeToken revokes a provisioning token
func (h *ProvisioningHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid token ID")
		return
	}

	if err := h.deps.DB.ProvisioningTokens.Revoke(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to revoke token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// GetDeviceEvents gets events for a device
func (h *ProvisioningHandler) GetDeviceEvents(w http.ResponseWriter, r *http.Request) {
	deviceIDStr := chi.URLParam(r, "id")
	deviceID, err := strconv.ParseInt(deviceIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	events, err := h.deps.DB.DeviceEvents.ListByDevice(r.Context(), deviceID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch events")
		return
	}

	count, _ := h.deps.DB.DeviceEvents.CountByDevice(r.Context(), deviceID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  count,
		"limit":  limit,
		"offset": offset,
	})
}

// GetRecentEvents gets recent events across all devices
func (h *ProvisioningHandler) GetRecentEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	events, err := h.deps.DB.DeviceEvents.ListRecent(r.Context(), limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch events")
		return
	}

	respondJSON(w, http.StatusOK, events)
}

// Helper function to convert empty string to nil pointer
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

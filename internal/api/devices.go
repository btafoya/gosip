package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/btafoya/gosip/pkg/sip"
	"github.com/go-chi/chi/v5"
)

// DeviceHandler handles device-related API endpoints
type DeviceHandler struct {
	deps *Dependencies
}

// NewDeviceHandler creates a new DeviceHandler
func NewDeviceHandler(deps *Dependencies) *DeviceHandler {
	return &DeviceHandler{deps: deps}
}

// DeviceResponse represents a device in API responses
type DeviceResponse struct {
	ID               int64  `json:"id"`
	UserID           *int64 `json:"user_id,omitempty"`
	Name             string `json:"name"`
	Username         string `json:"username"`
	DeviceType       string `json:"device_type"`
	RecordingEnabled bool   `json:"recording_enabled"`
	CreatedAt        string `json:"created_at"`
	Online           bool   `json:"online"`
}

// List returns all devices
func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = config.DefaultPageSize
	}
	if limit > config.MaxPageSize {
		limit = config.MaxPageSize
	}

	devices, err := h.deps.DB.Devices.List(r.Context(), limit, offset)
	if err != nil {
		WriteInternalError(w)
		return
	}

	total, _ := h.deps.DB.Devices.Count(r.Context())

	// Get registration status for each device
	var response []*DeviceResponse
	for _, d := range devices {
		online := h.deps.SIP.GetRegistrar().IsRegistered(r.Context(), d.ID)
		response = append(response, toDeviceResponse(d, online))
	}

	WriteList(w, response, total, limit, offset)
}

// CreateDeviceRequest represents a device creation request
type CreateDeviceRequest struct {
	Name             string `json:"name"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	DeviceType       string `json:"device_type"`
	RecordingEnabled bool   `json:"recording_enabled"`
	UserID           *int64 `json:"user_id,omitempty"`
}

// Create creates a new device
func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	var errors []FieldError
	if req.Name == "" {
		errors = append(errors, FieldError{Field: "name", Message: "Name is required"})
	}
	if req.Username == "" {
		errors = append(errors, FieldError{Field: "username", Message: "Username is required"})
	}
	if req.Password == "" {
		errors = append(errors, FieldError{Field: "password", Message: "Password is required"})
	}
	if req.DeviceType == "" {
		req.DeviceType = "softphone"
	}
	if req.DeviceType != "grandstream" && req.DeviceType != "softphone" && req.DeviceType != "webrtc" {
		errors = append(errors, FieldError{Field: "device_type", Message: "Invalid device type"})
	}

	if len(errors) > 0 {
		WriteValidationError(w, "Validation failed", errors)
		return
	}

	// Generate HA1 hash for SIP authentication
	ha1 := sip.GenerateHA1(req.Username, "gosip", req.Password)

	device := &models.Device{
		Name:             req.Name,
		Username:         req.Username,
		PasswordHash:     ha1, // Store HA1 for SIP digest auth
		DeviceType:       req.DeviceType,
		RecordingEnabled: req.RecordingEnabled,
		UserID:           req.UserID,
	}

	if err := h.deps.DB.Devices.Create(r.Context(), device); err != nil {
		WriteError(w, http.StatusConflict, ErrCodeConflict, "Device with this username already exists", nil)
		return
	}

	WriteJSON(w, http.StatusCreated, toDeviceResponse(device, false))
}

// Get returns a specific device
func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid device ID", nil)
		return
	}

	device, err := h.deps.DB.Devices.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrDeviceNotFound {
			WriteNotFoundError(w, "Device")
			return
		}
		WriteInternalError(w)
		return
	}

	online := h.deps.SIP.GetRegistrar().IsRegistered(r.Context(), device.ID)
	WriteJSON(w, http.StatusOK, toDeviceResponse(device, online))
}

// UpdateDeviceRequest represents a device update request
type UpdateDeviceRequest struct {
	Name             string `json:"name,omitempty"`
	Password         string `json:"password,omitempty"`
	DeviceType       string `json:"device_type,omitempty"`
	RecordingEnabled *bool  `json:"recording_enabled,omitempty"`
	UserID           *int64 `json:"user_id,omitempty"`
}

// Update updates a device
func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid device ID", nil)
		return
	}

	device, err := h.deps.DB.Devices.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrDeviceNotFound {
			WriteNotFoundError(w, "Device")
			return
		}
		WriteInternalError(w)
		return
	}

	var req UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.Name != "" {
		device.Name = req.Name
	}
	if req.Password != "" {
		device.PasswordHash = sip.GenerateHA1(device.Username, "gosip", req.Password)
	}
	if req.DeviceType != "" {
		device.DeviceType = req.DeviceType
	}
	if req.RecordingEnabled != nil {
		device.RecordingEnabled = *req.RecordingEnabled
	}
	if req.UserID != nil {
		device.UserID = req.UserID
	}

	if err := h.deps.DB.Devices.Update(r.Context(), device); err != nil {
		WriteInternalError(w)
		return
	}

	online := h.deps.SIP.GetRegistrar().IsRegistered(r.Context(), device.ID)
	WriteJSON(w, http.StatusOK, toDeviceResponse(device, online))
}

// Delete removes a device
func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid device ID", nil)
		return
	}

	if err := h.deps.DB.Devices.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Device deleted successfully"})
}

// GetRegistrations returns all active SIP registrations
func (h *DeviceHandler) GetRegistrations(w http.ResponseWriter, r *http.Request) {
	registrations, err := h.deps.SIP.GetActiveRegistrations(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, registrations)
}

func toDeviceResponse(device *models.Device, online bool) *DeviceResponse {
	return &DeviceResponse{
		ID:               device.ID,
		UserID:           device.UserID,
		Name:             device.Name,
		Username:         device.Username,
		DeviceType:       device.DeviceType,
		RecordingEnabled: device.RecordingEnabled,
		CreatedAt:        device.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Online:           online,
	}
}

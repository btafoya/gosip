package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
)

// DIDHandler handles DID-related API endpoints
type DIDHandler struct {
	deps *Dependencies
}

// NewDIDHandler creates a new DIDHandler
func NewDIDHandler(deps *Dependencies) *DIDHandler {
	return &DIDHandler{deps: deps}
}

// DIDCapabilities represents the capabilities of a DID
type DIDCapabilities struct {
	Voice bool `json:"voice"`
	SMS   bool `json:"sms"`
	MMS   bool `json:"mms"`
}

// DIDResponse represents a DID in API responses
type DIDResponse struct {
	ID           int64           `json:"id"`
	PhoneNumber  string          `json:"phone_number"`
	FriendlyName string          `json:"friendly_name,omitempty"`
	TwilioSID    string          `json:"twilio_sid,omitempty"`
	Capabilities DIDCapabilities `json:"capabilities"`
}

// List returns all DIDs
func (h *DIDHandler) List(w http.ResponseWriter, r *http.Request) {
	dids, err := h.deps.DB.DIDs.List(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*DIDResponse
	for _, d := range dids {
		response = append(response, toDIDResponse(d))
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": response})
}

// CreateDIDRequest represents a DID creation request
type CreateDIDRequest struct {
	Number       string `json:"number"`
	TwilioSID    string `json:"twilio_sid,omitempty"`
	Name         string `json:"name,omitempty"`
	SMSEnabled   bool   `json:"sms_enabled"`
	VoiceEnabled bool   `json:"voice_enabled"`
}

// Create creates a new DID
func (h *DIDHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateDIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	if req.Number == "" {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "number", Message: "Phone number is required"},
		})
		return
	}

	did := &models.DID{
		Number:       req.Number,
		TwilioSID:    req.TwilioSID,
		Name:         req.Name,
		SMSEnabled:   req.SMSEnabled,
		VoiceEnabled: req.VoiceEnabled,
	}

	if err := h.deps.DB.DIDs.Create(r.Context(), did); err != nil {
		WriteError(w, http.StatusConflict, ErrCodeConflict, "DID with this number already exists", nil)
		return
	}

	WriteJSON(w, http.StatusCreated, toDIDResponse(did))
}

// Get returns a specific DID
func (h *DIDHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid DID ID", nil)
		return
	}

	did, err := h.deps.DB.DIDs.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrDIDNotFound {
			WriteNotFoundError(w, "DID")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toDIDResponse(did))
}

// UpdateDIDRequest represents a DID update request
type UpdateDIDRequest struct {
	PhoneNumber  string `json:"phone_number,omitempty"`
	TwilioSID    string `json:"twilio_sid,omitempty"`
	FriendlyName string `json:"friendly_name,omitempty"`
	SMSEnabled   *bool  `json:"sms_enabled,omitempty"`
	VoiceEnabled *bool  `json:"voice_enabled,omitempty"`
}

// Update updates a DID
func (h *DIDHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid DID ID", nil)
		return
	}

	did, err := h.deps.DB.DIDs.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrDIDNotFound {
			WriteNotFoundError(w, "DID")
			return
		}
		WriteInternalError(w)
		return
	}

	var req UpdateDIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.PhoneNumber != "" {
		did.Number = req.PhoneNumber
	}
	if req.TwilioSID != "" {
		did.TwilioSID = req.TwilioSID
	}
	if req.FriendlyName != "" {
		did.Name = req.FriendlyName
	}
	if req.SMSEnabled != nil {
		did.SMSEnabled = *req.SMSEnabled
	}
	if req.VoiceEnabled != nil {
		did.VoiceEnabled = *req.VoiceEnabled
	}

	if err := h.deps.DB.DIDs.Update(r.Context(), did); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toDIDResponse(did))
}

// Delete removes a DID
func (h *DIDHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid DID ID", nil)
		return
	}

	if err := h.deps.DB.DIDs.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "DID deleted successfully"})
}

func toDIDResponse(did *models.DID) *DIDResponse {
	return &DIDResponse{
		ID:           did.ID,
		PhoneNumber:  did.Number,
		FriendlyName: did.Name,
		TwilioSID:    did.TwilioSID,
		Capabilities: DIDCapabilities{
			Voice: did.VoiceEnabled,
			SMS:   did.SMSEnabled,
			MMS:   did.SMSEnabled, // MMS typically follows SMS capability
		},
	}
}

// SyncFromTwilio syncs DIDs from Twilio account
func (h *DIDHandler) SyncFromTwilio(w http.ResponseWriter, r *http.Request) {
	// Check if Twilio client is available
	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, "TWILIO_NOT_CONFIGURED", "Twilio is not configured", nil)
		return
	}

	// Get phone numbers from Twilio
	twilioNumbers, err := h.deps.Twilio.ListIncomingPhoneNumbers(r.Context())
	if err != nil {
		WriteError(w, http.StatusBadGateway, "TWILIO_ERROR", "Failed to fetch phone numbers from Twilio: "+err.Error(), nil)
		return
	}

	// Get existing DIDs
	existingDIDs, err := h.deps.DB.DIDs.List(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	// Create a map of existing DIDs by phone number
	existingMap := make(map[string]*models.DID)
	for _, did := range existingDIDs {
		existingMap[did.Number] = did
	}

	var synced []*DIDResponse
	var created, updated int

	for _, tn := range twilioNumbers {
		if existing, ok := existingMap[tn.PhoneNumber]; ok {
			// Update existing DID
			existing.TwilioSID = tn.SID
			existing.SMSEnabled = tn.SMSEnabled
			existing.VoiceEnabled = tn.VoiceEnabled
			if existing.Name == "" && tn.FriendlyName != "" {
				existing.Name = tn.FriendlyName
			}
			if err := h.deps.DB.DIDs.Update(r.Context(), existing); err != nil {
				continue
			}
			synced = append(synced, toDIDResponse(existing))
			updated++
		} else {
			// Create new DID
			did := &models.DID{
				Number:       tn.PhoneNumber,
				TwilioSID:    tn.SID,
				Name:         tn.FriendlyName,
				SMSEnabled:   tn.SMSEnabled,
				VoiceEnabled: tn.VoiceEnabled,
			}
			if err := h.deps.DB.DIDs.Create(r.Context(), did); err != nil {
				continue
			}
			synced = append(synced, toDIDResponse(did))
			created++
		}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "DIDs synced successfully",
		"created": created,
		"updated": updated,
		"total":   len(synced),
		"dids":    synced,
	})
}

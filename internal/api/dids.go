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

// DIDResponse represents a DID in API responses
type DIDResponse struct {
	ID           int64  `json:"id"`
	Number       string `json:"number"`
	TwilioSID    string `json:"twilio_sid,omitempty"`
	Name         string `json:"name,omitempty"`
	SMSEnabled   bool   `json:"sms_enabled"`
	VoiceEnabled bool   `json:"voice_enabled"`
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

	WriteJSON(w, http.StatusOK, response)
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
	Number       string `json:"number,omitempty"`
	TwilioSID    string `json:"twilio_sid,omitempty"`
	Name         string `json:"name,omitempty"`
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

	if req.Number != "" {
		did.Number = req.Number
	}
	if req.TwilioSID != "" {
		did.TwilioSID = req.TwilioSID
	}
	if req.Name != "" {
		did.Name = req.Name
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
		Number:       did.Number,
		TwilioSID:    did.TwilioSID,
		Name:         did.Name,
		SMSEnabled:   did.SMSEnabled,
		VoiceEnabled: did.VoiceEnabled,
	}
}

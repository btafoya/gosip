package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
)

// validTransferModes defines allowed SIP trunk transfer modes
var validTransferModes = map[string]bool{
	"disable-all": true,
	"enable-all":  true,
	"sip-only":    true,
}

// TrunkHandler handles SIP trunk API endpoints
type TrunkHandler struct {
	deps *Dependencies
}

// NewTrunkHandler creates a new TrunkHandler
func NewTrunkHandler(deps *Dependencies) *TrunkHandler {
	return &TrunkHandler{deps: deps}
}

// TrunkResponse represents a trunk in API responses
type TrunkResponse struct {
	ID                int64     `json:"id"`
	TwilioSID         string    `json:"twilio_sid"`
	FriendlyName      string    `json:"friendly_name,omitempty"`
	DomainName        string    `json:"domain_name,omitempty"`
	Secure            bool      `json:"secure"`
	TransferMode      string    `json:"transfer_mode"`
	CnamLookupEnabled bool      `json:"cnam_lookup_enabled"`
	CreatedAt         string    `json:"created_at,omitempty"`
	UpdatedAt         string    `json:"updated_at,omitempty"`
}

// CreateTrunkRequest represents a trunk creation request
type CreateTrunkRequest struct {
	FriendlyName      string `json:"friendly_name"`
	Secure            bool   `json:"secure"`
	TransferMode      string `json:"transfer_mode,omitempty"`
	CnamLookupEnabled bool   `json:"cnam_lookup_enabled,omitempty"`
}

// UpdateTrunkRequest represents a trunk update request
type UpdateTrunkRequest struct {
	FriendlyName      *string `json:"friendly_name,omitempty"`
	Secure            *bool   `json:"secure,omitempty"`
	TransferMode      *string `json:"transfer_mode,omitempty"`
	CnamLookupEnabled *bool   `json:"cnam_lookup_enabled,omitempty"`
}

// AssignDIDRequest represents a DID assignment request
type AssignDIDRequest struct {
	DIDID int64 `json:"did_id"`
}

func toTrunkResponse(trunk *models.Trunk) *TrunkResponse {
	resp := &TrunkResponse{
		ID:                trunk.ID,
		TwilioSID:         trunk.TwilioSID,
		FriendlyName:      trunk.FriendlyName,
		DomainName:        trunk.DomainName,
		Secure:            trunk.Secure,
		TransferMode:      trunk.TransferMode,
		CnamLookupEnabled: trunk.CnamLookupEnabled,
	}
	if !trunk.CreatedAt.IsZero() {
		resp.CreatedAt = trunk.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	if !trunk.UpdatedAt.IsZero() {
		resp.UpdatedAt = trunk.UpdatedAt.Format("2006-01-02T15:04:05Z")
	}
	return resp
}

// List returns all trunks
func (h *TrunkHandler) List(w http.ResponseWriter, r *http.Request) {
	trunks, err := h.deps.DB.Trunks.List(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*TrunkResponse
	for _, t := range trunks {
		response = append(response, toTrunkResponse(t))
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": response})
}

// Get returns a specific trunk
func (h *TrunkHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid trunk ID", nil)
		return
	}

	trunk, err := h.deps.DB.Trunks.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrTrunkNotFound {
			WriteNotFoundError(w, "Trunk")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toTrunkResponse(trunk))
}

// Create creates a new trunk in Twilio and locally
func (h *TrunkHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, "TWILIO_NOT_CONFIGURED", "Twilio is not configured", nil)
		return
	}

	var req CreateTrunkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.FriendlyName == "" {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "friendly_name", Message: "Friendly name is required"},
		})
		return
	}
	if req.TransferMode != "" && !validTransferModes[req.TransferMode] {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "transfer_mode", Message: "Invalid transfer mode"},
		})
		return
	}

	// Create in Twilio
	twilioTrunk, err := h.deps.Twilio.CreateSIPTrunk(r.Context(), req.FriendlyName, req.Secure)
	if err != nil {
		WriteError(w, http.StatusBadGateway, "TWILIO_ERROR", "Failed to create trunk in Twilio: "+err.Error(), nil)
		return
	}

	transferMode := req.TransferMode
	if transferMode == "" {
		transferMode = "disable-all"
	}

	trunk := &models.Trunk{
		TwilioSID:         twilioTrunk.SID,
		FriendlyName:      req.FriendlyName,
		DomainName:        twilioTrunk.DomainName,
		Secure:            req.Secure,
		TransferMode:      transferMode,
		CnamLookupEnabled: req.CnamLookupEnabled,
	}

	if err := h.deps.DB.Trunks.Create(r.Context(), trunk); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusCreated, toTrunkResponse(trunk))
}

// Update updates a trunk in Twilio and locally
func (h *TrunkHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, "TWILIO_NOT_CONFIGURED", "Twilio is not configured", nil)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid trunk ID", nil)
		return
	}

	trunk, err := h.deps.DB.Trunks.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrTrunkNotFound {
			WriteNotFoundError(w, "Trunk")
			return
		}
		WriteInternalError(w)
		return
	}

	var req UpdateTrunkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	secure := trunk.Secure
	friendlyName := trunk.FriendlyName
	if req.Secure != nil {
		secure = *req.Secure
	}
	if req.FriendlyName != nil && *req.FriendlyName != "" {
		friendlyName = *req.FriendlyName
	}
	if req.TransferMode != nil {
		if !validTransferModes[*req.TransferMode] {
			WriteValidationError(w, "Validation failed", []FieldError{
				{Field: "transfer_mode", Message: "Invalid transfer mode"},
			})
			return
		}
		trunk.TransferMode = *req.TransferMode
	}
	if req.CnamLookupEnabled != nil {
		trunk.CnamLookupEnabled = *req.CnamLookupEnabled
	}

	// Update in Twilio
	if err := h.deps.Twilio.UpdateSIPTrunk(r.Context(), trunk.TwilioSID, secure, friendlyName); err != nil {
		WriteError(w, http.StatusBadGateway, "TWILIO_ERROR", "Failed to update trunk in Twilio: "+err.Error(), nil)
		return
	}

	trunk.Secure = secure
	trunk.FriendlyName = friendlyName

	if err := h.deps.DB.Trunks.Update(r.Context(), trunk); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toTrunkResponse(trunk))
}

// Delete removes a trunk from Twilio and locally
func (h *TrunkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, "TWILIO_NOT_CONFIGURED", "Twilio is not configured", nil)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid trunk ID", nil)
		return
	}

	trunk, err := h.deps.DB.Trunks.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrTrunkNotFound {
			WriteNotFoundError(w, "Trunk")
			return
		}
		WriteInternalError(w)
		return
	}

	// Delete from Twilio
	if err := h.deps.Twilio.DeleteSIPTrunk(r.Context(), trunk.TwilioSID); err != nil {
		WriteError(w, http.StatusBadGateway, "TWILIO_ERROR", "Failed to delete trunk from Twilio: "+err.Error(), nil)
		return
	}

	if err := h.deps.DB.Trunks.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Trunk deleted successfully"})
}

// SyncFromTwilio syncs trunks from Twilio into local cache
func (h *TrunkHandler) SyncFromTwilio(w http.ResponseWriter, r *http.Request) {
	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, "TWILIO_NOT_CONFIGURED", "Twilio is not configured", nil)
		return
	}

	twilioTrunks, err := h.deps.Twilio.ListSIPTrunks(r.Context())
	if err != nil {
		WriteError(w, http.StatusBadGateway, "TWILIO_ERROR", "Failed to fetch trunks from Twilio: "+err.Error(), nil)
		return
	}

	existingTrunks, err := h.deps.DB.Trunks.List(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	existingMap := make(map[string]*models.Trunk)
	for _, t := range existingTrunks {
		existingMap[t.TwilioSID] = t
	}

	var synced []*TrunkResponse
	var created, updated int

	for _, tt := range twilioTrunks {
		if existing, ok := existingMap[tt.SID]; ok {
			existing.FriendlyName = tt.FriendlyName
			existing.DomainName = tt.DomainName
			existing.Secure = tt.Secure
			existing.TransferMode = tt.TransferMode
			existing.CnamLookupEnabled = tt.CnamLookupEnabled
			if err := h.deps.DB.Trunks.Update(r.Context(), existing); err != nil {
				slog.Error("failed to update trunk during sync", "error", err, "trunk_id", existing.ID)
				continue
			}
			synced = append(synced, toTrunkResponse(existing))
			updated++
		} else {
			trunk := &models.Trunk{
				TwilioSID:         tt.SID,
				FriendlyName:      tt.FriendlyName,
				DomainName:        tt.DomainName,
				Secure:            tt.Secure,
				TransferMode:      tt.TransferMode,
				CnamLookupEnabled: tt.CnamLookupEnabled,
			}
			if err := h.deps.DB.Trunks.Create(r.Context(), trunk); err != nil {
				slog.Error("failed to create trunk during sync", "error", err, "twilio_sid", tt.SID)
				continue
			}
			synced = append(synced, toTrunkResponse(trunk))
			created++
		}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Trunks synced successfully",
		"created": created,
		"updated": updated,
		"total":   len(synced),
		"trunks":  synced,
	})
}

// AssignDID assigns a DID to this trunk in Twilio and locally
func (h *TrunkHandler) AssignDID(w http.ResponseWriter, r *http.Request) {
	if h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, "TWILIO_NOT_CONFIGURED", "Twilio is not configured", nil)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid trunk ID", nil)
		return
	}

	trunk, err := h.deps.DB.Trunks.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrTrunkNotFound {
			WriteNotFoundError(w, "Trunk")
			return
		}
		WriteInternalError(w)
		return
	}

	var req AssignDIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.DIDID <= 0 {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "did_id", Message: "did_id is required"},
		})
		return
	}

	did, err := h.deps.DB.DIDs.GetByID(r.Context(), req.DIDID)
	if err != nil {
		if err == db.ErrDIDNotFound {
			WriteNotFoundError(w, "DID")
			return
		}
		WriteInternalError(w)
		return
	}

	if did.TwilioSID == "" {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "did_id", Message: "DID does not have a Twilio SID"},
		})
		return
	}

	if err := h.deps.Twilio.AssignPhoneNumberToTrunk(r.Context(), trunk.TwilioSID, did.TwilioSID); err != nil {
		WriteError(w, http.StatusBadGateway, "TWILIO_ERROR", "Failed to assign number to trunk: "+err.Error(), nil)
		return
	}

	did.TrunkID = &trunk.ID
	if err := h.deps.DB.DIDs.Update(r.Context(), did); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "DID assigned to trunk successfully"})
}

// UnassignDID removes a DID from this trunk locally
func (h *TrunkHandler) UnassignDID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid trunk ID", nil)
		return
	}

	_, err = h.deps.DB.Trunks.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrTrunkNotFound {
			WriteNotFoundError(w, "Trunk")
			return
		}
		WriteInternalError(w)
		return
	}

	var req AssignDIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.DIDID <= 0 {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "did_id", Message: "did_id is required"},
		})
		return
	}

	did, err := h.deps.DB.DIDs.GetByID(r.Context(), req.DIDID)
	if err != nil {
		if err == db.ErrDIDNotFound {
			WriteNotFoundError(w, "DID")
			return
		}
		WriteInternalError(w)
		return
	}

	if did.TrunkID == nil || *did.TrunkID != id {
		WriteError(w, http.StatusConflict, "CONFLICT", "DID is not assigned to this trunk", nil)
		return
	}

	did.TrunkID = nil
	if err := h.deps.DB.DIDs.Update(r.Context(), did); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "DID unassigned from trunk successfully"})
}

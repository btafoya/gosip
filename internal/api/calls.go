package api

import (
	"encoding/json"
	"net/http"

	"github.com/btafoya/gosip/pkg/sip"
	"github.com/go-chi/chi/v5"
)

// CallHandler handles active call control API endpoints
type CallHandler struct {
	deps *Dependencies
}

// NewCallHandler creates a new CallHandler
func NewCallHandler(deps *Dependencies) *CallHandler {
	return &CallHandler{deps: deps}
}

// ActiveCallResponse represents an active call in API responses
type ActiveCallResponse struct {
	CallID          string `json:"call_id"`
	Direction       string `json:"direction"`
	State           string `json:"state"`
	FromNumber      string `json:"from_number"`
	ToNumber        string `json:"to_number"`
	Duration        int    `json:"duration"`
	DeviceID        int64  `json:"device_id,omitempty"`
	LocalURI        string `json:"local_uri"`
	RemoteURI       string `json:"remote_uri"`
	TransferTarget  string `json:"transfer_target,omitempty"`
	ConsultCallID   string `json:"consult_call_id,omitempty"`
	TransferredFrom string `json:"transferred_from,omitempty"`
}

// ListActiveCalls returns all active calls
func (h *CallHandler) ListActiveCalls(w http.ResponseWriter, r *http.Request) {
	if h.deps.SIP == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"data":  []ActiveCallResponse{},
			"count": 0,
		})
		return
	}

	sessionMgr := h.deps.SIP.GetSessions()
	if sessionMgr == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"data":  []ActiveCallResponse{},
			"count": 0,
		})
		return
	}

	sessions := sessionMgr.GetAll()
	response := make([]ActiveCallResponse, 0, len(sessions))

	for _, s := range sessions {
		response = append(response, ActiveCallResponse{
			CallID:          s.CallID,
			Direction:       string(s.Direction),
			State:           string(s.GetState()),
			FromNumber:      s.FromNumber,
			ToNumber:        s.ToNumber,
			Duration:        s.Duration(),
			DeviceID:        s.DeviceID,
			LocalURI:        s.LocalURI,
			RemoteURI:       s.RemoteURI,
			TransferTarget:  s.TransferTarget,
			ConsultCallID:   s.ConsultCallID,
			TransferredFrom: s.TransferredFrom,
		})
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data":  response,
		"count": len(response),
	})
}

// GetCall returns a specific call by ID
func (h *CallHandler) GetCall(w http.ResponseWriter, r *http.Request) {
	callID := chi.URLParam(r, "callID")

	if h.deps.SIP == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	sessionMgr := h.deps.SIP.GetSessions()
	if sessionMgr == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	session := sessionMgr.Get(callID)
	if session == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": ActiveCallResponse{
			CallID:          session.CallID,
			Direction:       string(session.Direction),
			State:           string(session.GetState()),
			FromNumber:      session.FromNumber,
			ToNumber:        session.ToNumber,
			Duration:        session.Duration(),
			DeviceID:        session.DeviceID,
			LocalURI:        session.LocalURI,
			RemoteURI:       session.RemoteURI,
			TransferTarget:  session.TransferTarget,
			ConsultCallID:   session.ConsultCallID,
			TransferredFrom: session.TransferredFrom,
		},
	})
}

// HoldRequest represents a hold/resume request
type HoldRequest struct {
	Hold bool `json:"hold"`
}

// HoldCall places a call on hold or resumes it
func (h *CallHandler) HoldCall(w http.ResponseWriter, r *http.Request) {
	callID := chi.URLParam(r, "callID")

	var req HoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if h.deps.SIP == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	sessionMgr := h.deps.SIP.GetSessions()
	if sessionMgr == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	session := sessionMgr.Get(callID)
	if session == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	holdMgr := h.deps.SIP.GetHoldManager()
	if holdMgr == nil {
		WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Hold manager not available", nil)
		return
	}

	var err error
	if req.Hold {
		err = holdMgr.PutOnHold(r.Context(), session)
	} else {
		err = holdMgr.Resume(r.Context(), session)
	}

	if err != nil {
		WriteError(w, http.StatusBadRequest, "HOLD_FAILED", err.Error(), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"state":   string(session.GetState()),
	})
}

// TransferRequest represents a transfer request
type TransferRequest struct {
	Type      string `json:"type"` // "blind" or "attended"
	Target    string `json:"target"`
	ConsultID string `json:"consult_id,omitempty"` // For attended transfer
}

// TransferCall initiates a call transfer
func (h *CallHandler) TransferCall(w http.ResponseWriter, r *http.Request) {
	callID := chi.URLParam(r, "callID")

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	var errors []FieldError
	if req.Type != "blind" && req.Type != "attended" {
		errors = append(errors, FieldError{Field: "type", Message: "Type must be 'blind' or 'attended'"})
	}
	if req.Type == "blind" && req.Target == "" {
		errors = append(errors, FieldError{Field: "target", Message: "Target is required for blind transfer"})
	}
	if req.Type == "attended" && req.ConsultID == "" {
		errors = append(errors, FieldError{Field: "consult_id", Message: "Consult call ID is required for attended transfer"})
	}
	if len(errors) > 0 {
		WriteValidationError(w, "Validation failed", errors)
		return
	}

	if h.deps.SIP == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	sessionMgr := h.deps.SIP.GetSessions()
	if sessionMgr == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	session := sessionMgr.Get(callID)
	if session == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	transferMgr := h.deps.SIP.GetTransferManager()
	if transferMgr == nil {
		WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Transfer manager not available", nil)
		return
	}

	var err error
	if req.Type == "blind" {
		err = transferMgr.BlindTransfer(r.Context(), session, req.Target)
	} else {
		// Attended transfer
		consultSession := sessionMgr.Get(req.ConsultID)
		if consultSession == nil {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Consult call not found", nil)
			return
		}
		err = transferMgr.AttendedTransfer(r.Context(), session, consultSession)
	}

	if err != nil {
		WriteError(w, http.StatusBadRequest, "TRANSFER_FAILED", err.Error(), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"state":   string(session.GetState()),
	})
}

// CancelTransferCall cancels an in-progress transfer
func (h *CallHandler) CancelTransferCall(w http.ResponseWriter, r *http.Request) {
	callID := chi.URLParam(r, "callID")

	if h.deps.SIP == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	sessionMgr := h.deps.SIP.GetSessions()
	if sessionMgr == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	session := sessionMgr.Get(callID)
	if session == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	transferMgr := h.deps.SIP.GetTransferManager()
	if transferMgr == nil {
		WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Transfer manager not available", nil)
		return
	}

	if err := transferMgr.CancelTransfer(r.Context(), session); err != nil {
		WriteError(w, http.StatusBadRequest, "CANCEL_FAILED", err.Error(), nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"state":   string(session.GetState()),
	})
}

// HangupCall ends a call
func (h *CallHandler) HangupCall(w http.ResponseWriter, r *http.Request) {
	callID := chi.URLParam(r, "callID")

	if h.deps.SIP == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	sessionMgr := h.deps.SIP.GetSessions()
	if sessionMgr == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	session := sessionMgr.Get(callID)
	if session == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Call not found", nil)
		return
	}

	// Terminate the session (actual BYE handling is done by the SIP server)
	if err := session.SetState(sip.CallStateTerminated); err != nil {
		WriteError(w, http.StatusBadRequest, "HANGUP_FAILED", err.Error(), nil)
		return
	}

	// Stop MOH if active
	mohMgr := h.deps.SIP.GetMOHManager()
	if mohMgr != nil {
		mohMgr.Stop(callID)
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// MOHStatusResponse represents MOH status
type MOHStatusResponse struct {
	Enabled     bool   `json:"enabled"`
	AudioPath   string `json:"audio_path"`
	ActiveCount int    `json:"active_count"`
}

// GetMOHStatus returns Music on Hold status
func (h *CallHandler) GetMOHStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.SIP == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"data": MOHStatusResponse{
				Enabled:     false,
				AudioPath:   "",
				ActiveCount: 0,
			},
		})
		return
	}

	mohMgr := h.deps.SIP.GetMOHManager()
	if mohMgr == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"data": MOHStatusResponse{
				Enabled:     false,
				AudioPath:   "",
				ActiveCount: 0,
			},
		})
		return
	}

	status := mohMgr.GetStatus()
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": MOHStatusResponse{
			Enabled:     status.Enabled,
			AudioPath:   status.AudioPath,
			ActiveCount: status.ActiveCount,
		},
	})
}

// UpdateMOHRequest represents MOH configuration update
type UpdateMOHRequest struct {
	Enabled   *bool   `json:"enabled,omitempty"`
	AudioPath *string `json:"audio_path,omitempty"`
}

// UpdateMOH updates Music on Hold configuration
func (h *CallHandler) UpdateMOH(w http.ResponseWriter, r *http.Request) {
	var req UpdateMOHRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if h.deps.SIP == nil {
		WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "SIP server not available", nil)
		return
	}

	mohMgr := h.deps.SIP.GetMOHManager()
	if mohMgr == nil {
		WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "MOH manager not available", nil)
		return
	}

	if req.Enabled != nil {
		mohMgr.Enable(*req.Enabled)
	}
	if req.AudioPath != nil {
		mohMgr.SetAudioPath(*req.AudioPath)
	}

	status := mohMgr.GetStatus()
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": MOHStatusResponse{
			Enabled:     status.Enabled,
			AudioPath:   status.AudioPath,
			ActiveCount: status.ActiveCount,
		},
	})
}

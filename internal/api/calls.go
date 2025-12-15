package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/btafoya/gosip/internal/audio"
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

// MOHUploadResponse represents the response from uploading MOH audio
type MOHUploadResponse struct {
	Success   bool                       `json:"success"`
	Message   string                     `json:"message"`
	FilePath  string                     `json:"file_path,omitempty"`
	Duration  float64                    `json:"duration,omitempty"`
	Warnings  []string                   `json:"warnings,omitempty"`
	Error     *audio.WAVValidationError  `json:"error,omitempty"`
}

// UploadMOHAudio handles uploading a WAV file for Music on Hold
// POST /api/calls/moh/upload
func (h *CallHandler) UploadMOHAudio(w http.ResponseWriter, r *http.Request) {
	// Max upload size: 10MB (matching audio.MaxFileSize)
	const maxUploadSize = 10 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		if err.Error() == "http: request body too large" {
			WriteError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
				"File is too large. Maximum size is 10MB.", nil)
			return
		}
		WriteValidationError(w, "Failed to parse form data", nil)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("audio")
	if err != nil {
		WriteValidationError(w, "No audio file provided. Use form field 'audio'.", nil)
		return
	}
	defer file.Close()

	// Check file extension
	ext := filepath.Ext(header.Filename)
	if ext != ".wav" && ext != ".WAV" {
		WriteError(w, http.StatusBadRequest, "INVALID_FORMAT",
			"Only WAV files are supported. Please upload a .wav file.", nil)
		return
	}

	// Validate the WAV file
	validationResult := audio.ValidateWAV(file, header.Size)
	if !validationResult.Valid {
		WriteJSON(w, http.StatusBadRequest, MOHUploadResponse{
			Success: false,
			Message: "WAV file validation failed",
			Error:   validationResult.Error,
		})
		return
	}

	// Reset file reader for saving
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	} else {
		// If we can't seek, we need to re-read from form
		file.Close()
		file, _, err = r.FormFile("audio")
		if err != nil {
			WriteInternalError(w)
			return
		}
		defer file.Close()
	}

	// Create MOH directory if it doesn't exist
	mohDir := "/var/lib/gosip/moh"
	if err := os.MkdirAll(mohDir, 0755); err != nil {
		// Fallback to data directory
		mohDir = "data/moh"
		if err := os.MkdirAll(mohDir, 0755); err != nil {
			WriteError(w, http.StatusInternalServerError, "STORAGE_ERROR",
				"Failed to create MOH storage directory", nil)
			return
		}
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("moh_%s.wav", timestamp)
	filePath := filepath.Join(mohDir, filename)

	// Save the file
	dst, err := os.Create(filePath)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "STORAGE_ERROR",
			"Failed to create audio file", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath) // Clean up on error
		WriteError(w, http.StatusInternalServerError, "STORAGE_ERROR",
			"Failed to save audio file", nil)
		return
	}

	// Update MOH manager with new audio path
	if h.deps.SIP != nil {
		mohMgr := h.deps.SIP.GetMOHManager()
		if mohMgr != nil {
			mohMgr.SetAudioPath(filePath)
		}
	}

	// Store the path in config for persistence
	if h.deps.DB != nil {
		h.deps.DB.Config.Set(r.Context(), "moh_audio_path", filePath)
	}

	WriteJSON(w, http.StatusOK, MOHUploadResponse{
		Success:  true,
		Message:  "MOH audio uploaded successfully",
		FilePath: filePath,
		Duration: validationResult.Duration,
		Warnings: validationResult.Warnings,
	})
}

// ValidateMOHAudio validates a WAV file without saving it
// POST /api/calls/moh/validate
func (h *CallHandler) ValidateMOHAudio(w http.ResponseWriter, r *http.Request) {
	// Max upload size: 10MB
	const maxUploadSize = 10 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		if err.Error() == "http: request body too large" {
			WriteError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
				"File is too large. Maximum size is 10MB.", nil)
			return
		}
		WriteValidationError(w, "Failed to parse form data", nil)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("audio")
	if err != nil {
		WriteValidationError(w, "No audio file provided. Use form field 'audio'.", nil)
		return
	}
	defer file.Close()

	// Check file extension
	ext := filepath.Ext(header.Filename)
	if ext != ".wav" && ext != ".WAV" {
		WriteJSON(w, http.StatusOK, audio.WAVValidationResult{
			Valid: false,
			Error: &audio.WAVValidationError{
				Code:    audio.ErrCodeInvalidFormat,
				Message: "Only WAV files are supported",
				Details: fmt.Sprintf("Got %s file, expected .wav", ext),
			},
		})
		return
	}

	// Validate the WAV file
	result := audio.ValidateWAV(file, header.Size)
	WriteJSON(w, http.StatusOK, result)
}

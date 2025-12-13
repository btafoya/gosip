package api

import (
	"net/http"
	"strconv"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
)

// VoicemailHandler handles voicemail-related API endpoints
type VoicemailHandler struct {
	deps *Dependencies
}

// NewVoicemailHandler creates a new VoicemailHandler
func NewVoicemailHandler(deps *Dependencies) *VoicemailHandler {
	return &VoicemailHandler{deps: deps}
}

// VoicemailResponse represents a voicemail in API responses
type VoicemailResponse struct {
	ID              int64   `json:"id"`
	DIDID           int64   `json:"did_id"`
	CallerID        string  `json:"caller_id"`
	Duration        int     `json:"duration"`
	RecordingURL    string  `json:"recording_url,omitempty"`
	TranscriptText  string  `json:"transcript_text,omitempty"`
	IsRead          bool    `json:"is_read"`
	CreatedAt       string  `json:"created_at"`
	TwilioRecordingSID string `json:"twilio_recording_sid,omitempty"`
}

// List returns voicemails with filtering and pagination
func (h *VoicemailHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	didIDStr := r.URL.Query().Get("did_id")
	unreadOnly := r.URL.Query().Get("unread") == "true"

	if limit == 0 {
		limit = config.DefaultPageSize
	}
	if limit > config.MaxPageSize {
		limit = config.MaxPageSize
	}

	var voicemails []*models.Voicemail
	var err error
	var total int

	// Handle filtering
	if unreadOnly {
		var userID *int64
		if didIDStr != "" {
			uid, parseErr := strconv.ParseInt(didIDStr, 10, 64)
			if parseErr == nil {
				userID = &uid
			}
		}
		voicemails, err = h.deps.DB.Voicemails.ListUnread(r.Context(), userID)
		if err != nil {
			WriteInternalError(w)
			return
		}
		total = len(voicemails)
		// Apply manual pagination to unread list
		end := offset + limit
		if end > len(voicemails) {
			end = len(voicemails)
		}
		if offset < len(voicemails) {
			voicemails = voicemails[offset:end]
		} else {
			voicemails = nil
		}
	} else if didIDStr != "" {
		userID, parseErr := strconv.ParseInt(didIDStr, 10, 64)
		if parseErr == nil {
			voicemails, err = h.deps.DB.Voicemails.ListByUser(r.Context(), userID, limit, offset)
			if err == nil {
				total, _ = h.deps.DB.Voicemails.CountByUser(r.Context(), userID)
			}
		} else {
			voicemails, err = h.deps.DB.Voicemails.List(r.Context(), limit, offset)
			if err == nil {
				total, _ = h.deps.DB.Voicemails.Count(r.Context())
			}
		}
	} else {
		voicemails, err = h.deps.DB.Voicemails.List(r.Context(), limit, offset)
		if err == nil {
			total, _ = h.deps.DB.Voicemails.Count(r.Context())
		}
	}

	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*VoicemailResponse
	for _, v := range voicemails {
		response = append(response, toVoicemailResponse(v))
	}

	WriteList(w, response, total, limit, offset)
}

// Get returns a specific voicemail
func (h *VoicemailHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid voicemail ID", nil)
		return
	}

	voicemail, err := h.deps.DB.Voicemails.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrVoicemailNotFound {
			WriteNotFoundError(w, "Voicemail")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toVoicemailResponse(voicemail))
}

// MarkRead marks a voicemail as read
func (h *VoicemailHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid voicemail ID", nil)
		return
	}

	if err := h.deps.DB.Voicemails.MarkAsRead(r.Context(), id); err != nil {
		if err == db.ErrVoicemailNotFound {
			WriteNotFoundError(w, "Voicemail")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Voicemail marked as read"})
}

// Delete removes a voicemail
func (h *VoicemailHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid voicemail ID", nil)
		return
	}

	if err := h.deps.DB.Voicemails.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Voicemail deleted successfully"})
}

// GetUnreadCount returns the count of unread voicemails
func (h *VoicemailHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	count, err := h.deps.DB.Voicemails.CountUnread(r.Context(), nil)
	if err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}

// ListUnread returns only unread voicemails
func (h *VoicemailHandler) ListUnread(w http.ResponseWriter, r *http.Request) {
	voicemails, err := h.deps.DB.Voicemails.ListUnread(r.Context(), nil)
	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*VoicemailResponse
	for _, v := range voicemails {
		response = append(response, toVoicemailResponse(v))
	}

	WriteJSON(w, http.StatusOK, response)
}

// MarkAsRead marks a voicemail as read (alias for MarkRead)
func (h *VoicemailHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	h.MarkRead(w, r)
}

func toVoicemailResponse(v *models.Voicemail) *VoicemailResponse {
	var didID int64
	if v.UserID != nil {
		didID = *v.UserID
	}
	return &VoicemailResponse{
		ID:             v.ID,
		DIDID:          didID,
		CallerID:       v.FromNumber,
		Duration:       v.Duration,
		RecordingURL:   v.AudioURL,
		TranscriptText: v.Transcript,
		IsRead:         v.IsRead,
		CreatedAt:      v.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

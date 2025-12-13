package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
)

// MessageHandler handles SMS/MMS message API endpoints
type MessageHandler struct {
	deps *Dependencies
}

// NewMessageHandler creates a new MessageHandler
func NewMessageHandler(deps *Dependencies) *MessageHandler {
	return &MessageHandler{deps: deps}
}

// MessageResponse represents a message in API responses
type MessageResponse struct {
	ID           int64    `json:"id"`
	DIDID        int64    `json:"did_id"`
	Direction    string   `json:"direction"`
	RemoteNumber string   `json:"remote_number"`
	Body         string   `json:"body"`
	MediaURLs    []string `json:"media_urls,omitempty"`
	Status       string   `json:"status"`
	TwilioSID    string   `json:"twilio_sid,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

// List returns messages with filtering and pagination
func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	didIDStr := r.URL.Query().Get("did_id")
	direction := r.URL.Query().Get("direction")
	remoteNumber := r.URL.Query().Get("remote_number")

	if limit == 0 {
		limit = config.DefaultPageSize
	}
	if limit > config.MaxPageSize {
		limit = config.MaxPageSize
	}

	var messages []*models.Message
	var total int
	var err error

	// Apply filters in order of specificity
	switch {
	case didIDStr != "":
		didID, parseErr := strconv.ParseInt(didIDStr, 10, 64)
		if parseErr == nil {
			messages, err = h.deps.DB.Messages.ListByDID(r.Context(), didID, limit, offset)
			if err == nil {
				total, _ = h.deps.DB.Messages.CountByDID(r.Context(), didID)
			}
		} else {
			messages, err = h.deps.DB.Messages.List(r.Context(), limit, offset)
			if err == nil {
				total, _ = h.deps.DB.Messages.Count(r.Context())
			}
		}
	case direction != "":
		messages, err = h.deps.DB.Messages.ListByDirection(r.Context(), direction, limit, offset)
		if err == nil {
			total, _ = h.deps.DB.Messages.CountByDirection(r.Context(), direction)
		}
	case remoteNumber != "":
		messages, err = h.deps.DB.Messages.ListByRemoteNumber(r.Context(), remoteNumber, limit, offset)
		if err == nil {
			total, _ = h.deps.DB.Messages.CountByRemoteNumber(r.Context(), remoteNumber)
		}
	default:
		messages, err = h.deps.DB.Messages.List(r.Context(), limit, offset)
		if err == nil {
			total, _ = h.deps.DB.Messages.Count(r.Context())
		}
	}

	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*MessageResponse
	for _, m := range messages {
		response = append(response, toMessageResponse(m))
	}

	WriteList(w, response, total, limit, offset)
}

// SendMessageRequest represents a message send request
type SendMessageRequest struct {
	DIDID        int64    `json:"did_id"`
	ToNumber     string   `json:"to_number"`
	Body         string   `json:"body"`
	MediaURLs    []string `json:"media_urls,omitempty"`
}

// Send sends a new SMS/MMS message
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	var errors []FieldError
	if req.DIDID == 0 {
		errors = append(errors, FieldError{Field: "did_id", Message: "DID ID is required"})
	}
	if req.ToNumber == "" {
		errors = append(errors, FieldError{Field: "to_number", Message: "To number is required"})
	}
	if req.Body == "" && len(req.MediaURLs) == 0 {
		errors = append(errors, FieldError{Field: "body", Message: "Message body or media is required"})
	}

	if len(errors) > 0 {
		WriteValidationError(w, "Validation failed", errors)
		return
	}

	// Verify DID exists and is SMS-enabled
	did, err := h.deps.DB.DIDs.GetByID(r.Context(), req.DIDID)
	if err != nil {
		if err == db.ErrDIDNotFound {
			WriteNotFoundError(w, "DID")
			return
		}
		WriteInternalError(w)
		return
	}

	if !did.SMSEnabled {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "DID is not SMS-enabled", nil)
		return
	}

	// Convert media URLs to JSON
	var mediaURLsJSON []byte
	if len(req.MediaURLs) > 0 {
		mediaURLsJSON, _ = json.Marshal(req.MediaURLs)
	}

	// Create message record
	didID := req.DIDID
	message := &models.Message{
		DIDID:      &didID,
		Direction:  "outbound",
		FromNumber: did.Number,
		ToNumber:   req.ToNumber,
		Body:       req.Body,
		MediaURLs:  mediaURLsJSON,
		Status:     "queued",
		CreatedAt:  time.Now(),
	}

	if err := h.deps.DB.Messages.Create(r.Context(), message); err != nil {
		WriteInternalError(w)
		return
	}

	// Send via Twilio (async - queue for sending)
	go func() {
		if h.deps.Twilio != nil {
			twilioSID, sendErr := h.deps.Twilio.SendSMS(did.Number, req.ToNumber, req.Body, req.MediaURLs)
			if sendErr != nil {
				h.deps.DB.Messages.UpdateStatus(r.Context(), message.ID, "failed")
			} else {
				message.MessageSID = twilioSID
				message.Status = "sent"
				h.deps.DB.Messages.Update(r.Context(), message)
			}
		}
	}()

	WriteJSON(w, http.StatusAccepted, toMessageResponse(message))
}

// Get returns a specific message
func (h *MessageHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid message ID", nil)
		return
	}

	message, err := h.deps.DB.Messages.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrMessageNotFound {
			WriteNotFoundError(w, "Message")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toMessageResponse(message))
}

// Delete removes a message
func (h *MessageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid message ID", nil)
		return
	}

	if err := h.deps.DB.Messages.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Message deleted successfully"})
}

// GetConversation returns messages grouped by conversation (remote number)
func (h *MessageHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	remoteNumber := chi.URLParam(r, "number")
	if remoteNumber == "" {
		WriteValidationError(w, "Remote number is required", nil)
		return
	}

	didIDStr := r.URL.Query().Get("did_id")
	if didIDStr == "" {
		WriteValidationError(w, "did_id query parameter is required", nil)
		return
	}

	didID, err := strconv.ParseInt(didIDStr, 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid did_id", nil)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	messages, err := h.deps.DB.Messages.GetConversation(r.Context(), didID, remoteNumber, limit, offset)
	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*MessageResponse
	for _, m := range messages {
		response = append(response, toMessageResponse(m))
	}

	WriteJSON(w, http.StatusOK, response)
}

// GetConversations returns a list of conversation summaries
func (h *MessageHandler) GetConversations(w http.ResponseWriter, r *http.Request) {
	didIDStr := r.URL.Query().Get("did_id")
	if didIDStr == "" {
		WriteValidationError(w, "did_id query parameter is required", nil)
		return
	}

	didID, err := strconv.ParseInt(didIDStr, 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid did_id", nil)
		return
	}

	conversations, err := h.deps.DB.Messages.GetConversationSummaries(r.Context(), didID)
	if err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, conversations)
}

// MarkAsRead marks a message as read
func (h *MessageHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid message ID", nil)
		return
	}

	if err := h.deps.DB.Messages.MarkAsRead(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Message marked as read"})
}

// Auto-reply endpoints

// AutoReplyResponse represents an auto-reply rule in API responses
type AutoReplyResponse struct {
	ID          int64  `json:"id"`
	DIDID       *int64 `json:"did_id,omitempty"`
	TriggerType string `json:"trigger_type"`
	TriggerData string `json:"trigger_data,omitempty"`
	ReplyText   string `json:"reply_text"`
	Enabled     bool   `json:"enabled"`
}

// ListAutoReplies returns all auto-reply rules
func (h *MessageHandler) ListAutoReplies(w http.ResponseWriter, r *http.Request) {
	rules, err := h.deps.DB.AutoReplies.List(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*AutoReplyResponse
	for _, rule := range rules {
		response = append(response, toAutoReplyResponse(rule))
	}

	WriteJSON(w, http.StatusOK, response)
}

// CreateAutoReplyRequest represents an auto-reply creation request
type CreateAutoReplyRequest struct {
	DIDID       *int64 `json:"did_id,omitempty"`
	TriggerType string `json:"trigger_type"`
	TriggerData string `json:"trigger_data,omitempty"`
	ReplyText   string `json:"reply_text"`
	Enabled     bool   `json:"enabled"`
}

// CreateAutoReply creates a new auto-reply rule
func (h *MessageHandler) CreateAutoReply(w http.ResponseWriter, r *http.Request) {
	var req CreateAutoReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	var errors []FieldError
	if req.TriggerType == "" {
		errors = append(errors, FieldError{Field: "trigger_type", Message: "Trigger type is required"})
	}
	if req.TriggerType != "keyword" && req.TriggerType != "after_hours" && req.TriggerType != "always" {
		errors = append(errors, FieldError{Field: "trigger_type", Message: "Invalid trigger type"})
	}
	if req.ReplyText == "" {
		errors = append(errors, FieldError{Field: "reply_text", Message: "Reply text is required"})
	}

	if len(errors) > 0 {
		WriteValidationError(w, "Validation failed", errors)
		return
	}

	rule := &models.AutoReply{
		DIDID:       req.DIDID,
		TriggerType: req.TriggerType,
		TriggerData: json.RawMessage(req.TriggerData),
		ReplyText:   req.ReplyText,
		Enabled:     req.Enabled,
	}

	if err := h.deps.DB.AutoReplies.Create(r.Context(), rule); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusCreated, toAutoReplyResponse(rule))
}

// UpdateAutoReply updates an auto-reply rule
func (h *MessageHandler) UpdateAutoReply(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid auto-reply ID", nil)
		return
	}

	rule, err := h.deps.DB.AutoReplies.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrAutoReplyNotFound {
			WriteNotFoundError(w, "Auto-reply rule")
			return
		}
		WriteInternalError(w)
		return
	}

	var req CreateAutoReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.TriggerType != "" {
		rule.TriggerType = req.TriggerType
	}
	if req.TriggerData != "" {
		rule.TriggerData = json.RawMessage(req.TriggerData)
	}
	if req.ReplyText != "" {
		rule.ReplyText = req.ReplyText
	}
	rule.Enabled = req.Enabled
	rule.DIDID = req.DIDID

	if err := h.deps.DB.AutoReplies.Update(r.Context(), rule); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toAutoReplyResponse(rule))
}

// DeleteAutoReply removes an auto-reply rule
func (h *MessageHandler) DeleteAutoReply(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid auto-reply ID", nil)
		return
	}

	if err := h.deps.DB.AutoReplies.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Auto-reply rule deleted successfully"})
}

func toMessageResponse(m *models.Message) *MessageResponse {
	var mediaURLs []string
	if len(m.MediaURLs) > 0 {
		json.Unmarshal(m.MediaURLs, &mediaURLs)
	}

	var didID int64
	if m.DIDID != nil {
		didID = *m.DIDID
	}

	// Determine remote number based on direction
	remoteNumber := m.ToNumber
	if m.Direction == "inbound" {
		remoteNumber = m.FromNumber
	}

	return &MessageResponse{
		ID:           m.ID,
		DIDID:        didID,
		Direction:    m.Direction,
		RemoteNumber: remoteNumber,
		Body:         m.Body,
		MediaURLs:    mediaURLs,
		Status:       m.Status,
		TwilioSID:    m.MessageSID,
		CreatedAt:    m.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toAutoReplyResponse(rule *models.AutoReply) *AutoReplyResponse {
	triggerData := ""
	if len(rule.TriggerData) > 0 {
		triggerData = string(rule.TriggerData)
	}
	return &AutoReplyResponse{
		ID:          rule.ID,
		DIDID:       rule.DIDID,
		TriggerType: rule.TriggerType,
		TriggerData: triggerData,
		ReplyText:   rule.ReplyText,
		Enabled:     rule.Enabled,
	}
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestMessageHandler_List(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and messages
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Hello")
	createTestMessage(t, setup.DB, did.ID, "outbound", "+15559876543", "Hi there")

	req := httptest.NewRequest(http.MethodGet, "/api/messages", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 2 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 2 messages, got %d", total)
	}
}

func TestMessageHandler_List_FilterByDirection(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and messages
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Hello")
	createTestMessage(t, setup.DB, did.ID, "outbound", "+15559876543", "Hi there")

	req := httptest.NewRequest(http.MethodGet, "/api/messages?direction=inbound", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 inbound message, got %d", total)
	}
}

func TestMessageHandler_List_FilterByDID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DIDs and messages
	did1 := createTestDID(t, setup.DB, "+15551234567")
	did2 := createTestDID(t, setup.DB, "+15559999999")
	createTestMessage(t, setup.DB, did1.ID, "inbound", "+15559876543", "Hello 1")
	createTestMessage(t, setup.DB, did2.ID, "inbound", "+15559876543", "Hello 2")

	req := httptest.NewRequest(http.MethodGet, "/api/messages?did_id=1", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 message for DID 1, got %d", total)
	}
}

func TestMessageHandler_List_FilterByRemoteNumber(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and messages
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Hello")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15558888888", "Hi")

	req := httptest.NewRequest(http.MethodGet, "/api/messages?remote_number=%2B15559876543", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 message from +15559876543, got %d", total)
	}
}

func TestMessageHandler_Send(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID (SMS-enabled)
	did := createTestDID(t, setup.DB, "+15551234567")

	reqBody := SendMessageRequest{
		DIDID:    did.ID,
		ToNumber: "+15559876543",
		Body:     "Test message",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/send", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Send(rr, req)

	assertStatus(t, rr, http.StatusAccepted)

	var resp MessageResponse
	decodeResponse(t, rr, &resp)

	if resp.Body != "Test message" {
		t.Errorf("Expected body 'Test message', got %s", resp.Body)
	}
	if resp.Direction != "outbound" {
		t.Errorf("Expected direction 'outbound', got %s", resp.Direction)
	}
}

func TestMessageHandler_Send_ValidationError(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	tests := []struct {
		name    string
		reqBody SendMessageRequest
	}{
		{
			name: "Missing DID ID",
			reqBody: SendMessageRequest{
				ToNumber: "+15559876543",
				Body:     "Test",
			},
		},
		{
			name: "Missing to number",
			reqBody: SendMessageRequest{
				DIDID: 1,
				Body:  "Test",
			},
		},
		{
			name: "Missing body and media",
			reqBody: SendMessageRequest{
				DIDID:    1,
				ToNumber: "+15559876543",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/messages/send", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Send(rr, req)

			assertStatus(t, rr, http.StatusBadRequest)
			assertErrorCode(t, rr, ErrCodeValidation)
		})
	}
}

func TestMessageHandler_Send_DIDNotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	reqBody := SendMessageRequest{
		DIDID:    9999,
		ToNumber: "+15559876543",
		Body:     "Test message",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/send", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Send(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestMessageHandler_Send_DIDNotSMSEnabled(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create DID with SMS disabled
	did := &models.DID{
		Number:       "+15551234567",
		VoiceEnabled: true,
		SMSEnabled:   false,
	}
	setup.DB.DIDs.Create(context.Background(), did)

	reqBody := SendMessageRequest{
		DIDID:    did.ID,
		ToNumber: "+15559876543",
		Body:     "Test message",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/send", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Send(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestMessageHandler_Get(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and message
	did := createTestDID(t, setup.DB, "+15551234567")
	msg := createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Hello")

	req := httptest.NewRequest(http.MethodGet, "/api/messages/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp MessageResponse
	decodeResponse(t, rr, &resp)

	if resp.Body != msg.Body {
		t.Errorf("Expected body %s, got %s", msg.Body, resp.Body)
	}
}

func TestMessageHandler_Get_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/messages/9999", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestMessageHandler_Delete(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and message
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Hello")

	req := httptest.NewRequest(http.MethodDelete, "/api/messages/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify deleted
	_, err := setup.DB.Messages.GetByID(context.Background(), 1)
	if err == nil {
		t.Error("Expected message to be deleted")
	}
}

func TestMessageHandler_GetConversation(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and messages
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Hello")
	createTestMessage(t, setup.DB, did.ID, "outbound", "+15559876543", "Hi there")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15558888888", "Different person")

	req := httptest.NewRequest(http.MethodGet, "/api/messages/conversation/+15559876543?did_id=1", nil)
	req = withURLParams(req, map[string]string{"number": "+15559876543"})

	rr := httptest.NewRecorder()
	handler.GetConversation(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var wrapper struct {
		Data []*MessageResponse `json:"data"`
	}
	decodeResponse(t, rr, &wrapper)

	if len(wrapper.Data) != 2 {
		t.Errorf("Expected 2 messages in conversation, got %d", len(wrapper.Data))
	}
}

func TestMessageHandler_GetConversation_MissingNumber(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/messages/conversation/", nil)
	req = withURLParams(req, map[string]string{"number": ""})

	rr := httptest.NewRecorder()
	handler.GetConversation(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestMessageHandler_ListAutoReplies(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test auto-replies
	createTestAutoReply(t, setup.DB, "keyword", "Thanks for your message!")
	createTestAutoReply(t, setup.DB, "after_hours", "We're currently closed")

	req := httptest.NewRequest(http.MethodGet, "/api/messages/auto-replies", nil)
	rr := httptest.NewRecorder()
	handler.ListAutoReplies(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var wrapper struct {
		Data []*AutoReplyResponse `json:"data"`
	}
	decodeResponse(t, rr, &wrapper)

	if len(wrapper.Data) != 2 {
		t.Errorf("Expected 2 auto-replies, got %d", len(wrapper.Data))
	}
}

func TestMessageHandler_CreateAutoReply(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	reqBody := CreateAutoReplyRequest{
		TriggerType: "keyword",
		TriggerData: "HELP",
		ReplyText:   "How can we help you?",
		Enabled:     true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/auto-replies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.CreateAutoReply(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp AutoReplyResponse
	decodeResponse(t, rr, &resp)

	if resp.TriggerType != "keyword" {
		t.Errorf("Expected trigger type 'keyword', got %s", resp.TriggerType)
	}
	if resp.ReplyText != "How can we help you?" {
		t.Errorf("Expected reply text 'How can we help you?', got %s", resp.ReplyText)
	}
}

func TestMessageHandler_CreateAutoReply_ValidationError(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	tests := []struct {
		name    string
		reqBody CreateAutoReplyRequest
	}{
		{
			name: "Missing trigger type",
			reqBody: CreateAutoReplyRequest{
				ReplyText: "Hello",
			},
		},
		{
			name: "Invalid trigger type",
			reqBody: CreateAutoReplyRequest{
				TriggerType: "invalid",
				ReplyText:   "Hello",
			},
		},
		{
			name: "Missing reply text",
			reqBody: CreateAutoReplyRequest{
				TriggerType: "keyword",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/messages/auto-replies", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateAutoReply(rr, req)

			assertStatus(t, rr, http.StatusBadRequest)
			assertErrorCode(t, rr, ErrCodeValidation)
		})
	}
}

func TestMessageHandler_UpdateAutoReply(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test auto-reply
	createTestAutoReply(t, setup.DB, "keyword", "Original reply")

	reqBody := CreateAutoReplyRequest{
		TriggerType: "after_hours",
		ReplyText:   "Updated reply",
		Enabled:     false,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/messages/auto-replies/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.UpdateAutoReply(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp AutoReplyResponse
	decodeResponse(t, rr, &resp)

	if resp.TriggerType != "after_hours" {
		t.Errorf("Expected trigger type 'after_hours', got %s", resp.TriggerType)
	}
	if resp.ReplyText != "Updated reply" {
		t.Errorf("Expected reply text 'Updated reply', got %s", resp.ReplyText)
	}
	if resp.Enabled {
		t.Error("Expected enabled to be false")
	}
}

func TestMessageHandler_UpdateAutoReply_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	reqBody := CreateAutoReplyRequest{
		TriggerType: "keyword",
		ReplyText:   "Hello",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/messages/auto-replies/9999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.UpdateAutoReply(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestMessageHandler_DeleteAutoReply(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test auto-reply
	createTestAutoReply(t, setup.DB, "keyword", "Delete me")

	req := httptest.NewRequest(http.MethodDelete, "/api/messages/auto-replies/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.DeleteAutoReply(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestMessageHandler_ValidTriggerTypes(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	validTypes := []string{"keyword", "after_hours", "always"}

	for _, triggerType := range validTypes {
		t.Run(triggerType, func(t *testing.T) {
			reqBody := CreateAutoReplyRequest{
				TriggerType: triggerType,
				ReplyText:   "Test reply",
				Enabled:     true,
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/messages/auto-replies", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateAutoReply(rr, req)

			assertStatus(t, rr, http.StatusCreated)
		})
	}
}

// Tests for new message handlers

func TestMessageHandler_GetUnreadCount(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and messages
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create some messages - inbound messages start as unread
	msg1 := createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Unread 1")
	msg2 := createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Unread 2")
	msg3 := createTestMessage(t, setup.DB, did.ID, "outbound", "+15559876543", "Outbound")

	// Mark one as read
	setup.DB.Messages.MarkAsRead(context.Background(), msg1.ID)
	_ = msg2
	_ = msg3

	req := httptest.NewRequest(http.MethodGet, "/api/messages/unread/count", nil)
	rr := httptest.NewRecorder()
	handler.GetUnreadCount(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp struct {
		UnreadCount int `json:"unread_count"`
	}
	decodeResponse(t, rr, &resp)

	// msg2 and msg3 should be unread (msg1 was marked read)
	// CountUnread counts all messages where is_read = 0 regardless of direction
	if resp.UnreadCount != 2 {
		t.Errorf("Expected 2 unread messages, got %d", resp.UnreadCount)
	}
}

func TestMessageHandler_GetStats(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and messages
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Inbound 1")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Inbound 2")
	createTestMessage(t, setup.DB, did.ID, "outbound", "+15559876543", "Outbound 1")

	req := httptest.NewRequest(http.MethodGet, "/api/messages/stats", nil)
	rr := httptest.NewRecorder()
	handler.GetStats(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp map[string]interface{}
	decodeResponse(t, rr, &resp)

	// Check that stats are returned
	if resp["total"] == nil {
		t.Error("Expected 'total' in stats response")
	}
	if resp["inbound"] == nil {
		t.Error("Expected 'inbound' in stats response")
	}
	if resp["outbound"] == nil {
		t.Error("Expected 'outbound' in stats response")
	}

	// Verify counts
	if int(resp["total"].(float64)) != 3 {
		t.Errorf("Expected total 3, got %v", resp["total"])
	}
	if int(resp["inbound"].(float64)) != 2 {
		t.Errorf("Expected inbound 2, got %v", resp["inbound"])
	}
	if int(resp["outbound"].(float64)) != 1 {
		t.Errorf("Expected outbound 1, got %v", resp["outbound"])
	}
}

func TestMessageHandler_MarkConversationAsRead(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	// Create test DID and messages in a conversation
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Message 1")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15559876543", "Message 2")
	createTestMessage(t, setup.DB, did.ID, "inbound", "+15558888888", "Different convo")

	// Verify initial unread count
	unread, _ := setup.DB.Messages.CountUnread(context.Background())
	if unread != 3 {
		t.Errorf("Expected 3 unread messages initially, got %d", unread)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/messages/conversation/+15559876543/read?did_id=1", nil)
	req = withURLParams(req, map[string]string{"number": "+15559876543"})

	rr := httptest.NewRecorder()
	handler.MarkConversationAsRead(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify unread count decreased (only the different convo message should be unread)
	unread, _ = setup.DB.Messages.CountUnread(context.Background())
	if unread != 1 {
		t.Errorf("Expected 1 unread message after marking conversation read, got %d", unread)
	}
}

func TestMessageHandler_MarkConversationAsRead_MissingParams(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewMessageHandler(deps)

	tests := []struct {
		name   string
		url    string
		params map[string]string
	}{
		{
			name:   "Missing number",
			url:    "/api/messages/conversation//read?did_id=1",
			params: map[string]string{"number": ""},
		},
		{
			name:   "Missing DID ID",
			url:    "/api/messages/conversation/+15559876543/read",
			params: map[string]string{"number": "+15559876543"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, tt.url, nil)
			req = withURLParams(req, tt.params)

			rr := httptest.NewRecorder()
			handler.MarkConversationAsRead(rr, req)

			assertStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestMessageHandler_Resend(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create a failed outbound message
	msg := &models.Message{
		MessageSID: "SM123456789",
		DIDID:      &did.ID,
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Failed message",
		Status:     "failed",
	}
	setup.DB.Messages.Create(context.Background(), msg)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/1/resend", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Resend(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp MessageResponse
	decodeResponse(t, rr, &resp)

	// The response should be the updated message with new TwilioSID and status "sent"
	if resp.TwilioSID == "" {
		t.Error("Expected twilio_sid in response")
	}
	if resp.Status != "sent" {
		t.Errorf("Expected status 'sent', got '%s'", resp.Status)
	}
}

func TestMessageHandler_Resend_NotOutbound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create an inbound message (can't resend inbound)
	msg := &models.Message{
		MessageSID: "SM123456789",
		DIDID:      &did.ID,
		Direction:  "inbound",
		FromNumber: "+15559876543",
		ToNumber:   "+15551234567",
		Body:       "Inbound message",
		Status:     "received",
	}
	setup.DB.Messages.Create(context.Background(), msg)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/1/resend", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Resend(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestMessageHandler_Resend_NotFailedStatus(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create a delivered outbound message (can't resend delivered)
	msg := &models.Message{
		MessageSID: "SM123456789",
		DIDID:      &did.ID,
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Delivered message",
		Status:     "delivered",
	}
	setup.DB.Messages.Create(context.Background(), msg)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/1/resend", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Resend(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestMessageHandler_Resend_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/9999/resend", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Resend(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestMessageHandler_SyncFromTwilio(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create a message with a SID
	msg := &models.Message{
		MessageSID: "SM123456789",
		DIDID:      &did.ID,
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Test message",
		Status:     "queued",
	}
	setup.DB.Messages.Create(context.Background(), msg)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/1/sync", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.SyncFromTwilio(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestMessageHandler_SyncFromTwilio_NoSID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create a message without a SID
	msg := &models.Message{
		MessageSID: "",
		DIDID:      &did.ID,
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Test message",
		Status:     "pending",
	}
	setup.DB.Messages.Create(context.Background(), msg)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/1/sync", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.SyncFromTwilio(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestMessageHandler_Cancel(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create a queued message
	msg := &models.Message{
		MessageSID: "SM123456789",
		DIDID:      &did.ID,
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Queued message",
		Status:     "queued",
	}
	setup.DB.Messages.Create(context.Background(), msg)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/1/cancel", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Cancel(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestMessageHandler_Cancel_NotCancelable(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	// Create test DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create a delivered message (can't cancel delivered)
	msg := &models.Message{
		MessageSID: "SM123456789",
		DIDID:      &did.ID,
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Delivered message",
		Status:     "delivered",
	}
	setup.DB.Messages.Create(context.Background(), msg)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/1/cancel", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Cancel(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestMessageHandler_Cancel_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio}
	handler := NewMessageHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/messages/9999/cancel", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Cancel(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

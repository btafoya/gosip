package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestMessageRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	msg := &models.Message{
		MessageSID: "SM123456789",
		Direction:  "inbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Hello, this is a test message.",
		Status:     "received",
	}

	err := db.Messages.Create(ctx, msg)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	if msg.ID == 0 {
		t.Error("Expected message ID to be set after creation")
	}
}

func TestMessageRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	msg := &models.Message{
		MessageSID: "SM123456789",
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Test body",
		Status:     "sent",
	}
	if err := db.Messages.Create(ctx, msg); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	retrieved, err := db.Messages.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("Failed to get message by ID: %v", err)
	}

	if retrieved.Body != msg.Body {
		t.Errorf("Expected body %s, got %s", msg.Body, retrieved.Body)
	}
	if retrieved.Direction != msg.Direction {
		t.Errorf("Expected direction %s, got %s", msg.Direction, retrieved.Direction)
	}
}

func TestMessageRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Messages.GetByID(ctx, 9999)
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound, got %v", err)
	}
}

func TestMessageRepository_GetByMessageSID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	msg := &models.Message{
		MessageSID: "SM_UNIQUE_123",
		Direction:  "inbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Find me by SID",
		Status:     "received",
	}
	if err := db.Messages.Create(ctx, msg); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	retrieved, err := db.Messages.GetByMessageSID(ctx, "SM_UNIQUE_123")
	if err != nil {
		t.Fatalf("Failed to get message by SID: %v", err)
	}

	if retrieved.ID != msg.ID {
		t.Errorf("Expected ID %d, got %d", msg.ID, retrieved.ID)
	}
}

func TestMessageRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	msg := &models.Message{
		MessageSID: "SM_UPDATE_123",
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Original body",
		Status:     "queued",
	}
	if err := db.Messages.Create(ctx, msg); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	msg.Status = "delivered"
	msg.IsRead = true
	if err := db.Messages.Update(ctx, msg); err != nil {
		t.Fatalf("Failed to update message: %v", err)
	}

	retrieved, _ := db.Messages.GetByID(ctx, msg.ID)
	if retrieved.Status != "delivered" {
		t.Errorf("Expected status 'delivered', got %s", retrieved.Status)
	}
	if !retrieved.IsRead {
		t.Error("Expected IsRead to be true")
	}
}

func TestMessageRepository_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	msg := &models.Message{
		MessageSID: "SM_STATUS_123",
		Direction:  "outbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Test",
		Status:     "queued",
	}
	if err := db.Messages.Create(ctx, msg); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	if err := db.Messages.UpdateStatus(ctx, msg.ID, "sent"); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	retrieved, _ := db.Messages.GetByID(ctx, msg.ID)
	if retrieved.Status != "sent" {
		t.Errorf("Expected status 'sent', got %s", retrieved.Status)
	}
}

func TestMessageRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	msg := &models.Message{
		MessageSID: "SM_DELETE_123",
		Direction:  "inbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Delete me",
		Status:     "received",
	}
	if err := db.Messages.Create(ctx, msg); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	if err := db.Messages.Delete(ctx, msg.ID); err != nil {
		t.Fatalf("Failed to delete message: %v", err)
	}

	_, err := db.Messages.GetByID(ctx, msg.ID)
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound after delete, got %v", err)
	}
}

func TestMessageRepository_MarkAsRead(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	msg := &models.Message{
		MessageSID: "SM_READ_123",
		Direction:  "inbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Mark me read",
		Status:     "received",
		IsRead:     false,
	}
	if err := db.Messages.Create(ctx, msg); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	if err := db.Messages.MarkAsRead(ctx, msg.ID); err != nil {
		t.Fatalf("Failed to mark as read: %v", err)
	}

	retrieved, _ := db.Messages.GetByID(ctx, msg.ID)
	if !retrieved.IsRead {
		t.Error("Expected message to be marked as read")
	}
}

func TestMessageRepository_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		msg := &models.Message{
			MessageSID: "SM_LIST_" + string(rune('0'+i)),
			Direction:  "inbound",
			FromNumber: "+1555000000" + string(rune('0'+i)),
			ToNumber:   "+15559876543",
			Body:       "Test message " + string(rune('0'+i)),
			Status:     "received",
		}
		if err := db.Messages.Create(ctx, msg); err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	msgs, err := db.Messages.List(ctx, 3, 0)
	if err != nil {
		t.Fatalf("Failed to list messages: %v", err)
	}

	if len(msgs) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(msgs))
	}
}

func TestMessageRepository_ListByDID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create DID
	did := &models.DID{
		Number:     "+15551234567",
		SMSEnabled: true,
	}
	if err := db.DIDs.Create(ctx, did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	// Create messages for the DID
	for i := 0; i < 3; i++ {
		msg := &models.Message{
			MessageSID: "SM_DID_" + string(rune('0'+i)),
			Direction:  "inbound",
			FromNumber: "+1555000000" + string(rune('0'+i)),
			ToNumber:   "+15551234567",
			DIDID:      &did.ID,
			Body:       "Test",
			Status:     "received",
		}
		if err := db.Messages.Create(ctx, msg); err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	// Create message without DID
	msg := &models.Message{
		MessageSID: "SM_NO_DID",
		Direction:  "inbound",
		FromNumber: "+15559999999",
		ToNumber:   "+15558888888",
		Body:       "No DID",
		Status:     "received",
	}
	db.Messages.Create(ctx, msg)

	msgs, err := db.Messages.ListByDID(ctx, did.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list by DID: %v", err)
	}

	if len(msgs) != 3 {
		t.Errorf("Expected 3 messages for DID, got %d", len(msgs))
	}
}

func TestMessageRepository_GetConversation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", SMSEnabled: true}
	db.DIDs.Create(ctx, did)

	// Create conversation messages
	conversationNumber := "+15559876543"
	msgs := []struct {
		direction string
		from      string
		to        string
	}{
		{"inbound", conversationNumber, "+15551234567"},
		{"outbound", "+15551234567", conversationNumber},
		{"inbound", conversationNumber, "+15551234567"},
		{"inbound", "+15550000000", "+15551234567"}, // Different conversation
	}

	for i, m := range msgs {
		msg := &models.Message{
			MessageSID: "SM_CONV_" + string(rune('0'+i)),
			Direction:  m.direction,
			FromNumber: m.from,
			ToNumber:   m.to,
			DIDID:      &did.ID,
			Body:       "Message " + string(rune('0'+i)),
			Status:     "received",
		}
		db.Messages.Create(ctx, msg)
	}

	conv, err := db.Messages.GetConversation(ctx, did.ID, conversationNumber, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get conversation: %v", err)
	}

	if len(conv) != 3 {
		t.Errorf("Expected 3 messages in conversation, got %d", len(conv))
	}
}

func TestMessageRepository_ListUnread(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create read and unread messages
	for i := 0; i < 4; i++ {
		msg := &models.Message{
			MessageSID: "SM_UNREAD_" + string(rune('0'+i)),
			Direction:  "inbound",
			FromNumber: "+1555000000" + string(rune('0'+i)),
			ToNumber:   "+15559876543",
			Body:       "Test",
			Status:     "received",
			IsRead:     i%2 == 0, // 0 and 2 are read
		}
		if err := db.Messages.Create(ctx, msg); err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	unread, err := db.Messages.ListUnread(ctx)
	if err != nil {
		t.Fatalf("Failed to list unread: %v", err)
	}

	if len(unread) != 2 {
		t.Errorf("Expected 2 unread messages, got %d", len(unread))
	}
}

func TestMessageRepository_CountUnread(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		msg := &models.Message{
			MessageSID: "SM_COUNT_" + string(rune('0'+i)),
			Direction:  "inbound",
			FromNumber: "+15551234567",
			ToNumber:   "+15559876543",
			Body:       "Test",
			Status:     "received",
			IsRead:     i < 2, // 0 and 1 are read
		}
		db.Messages.Create(ctx, msg)
	}

	count, err := db.Messages.CountUnread(ctx)
	if err != nil {
		t.Fatalf("Failed to count unread: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 unread, got %d", count)
	}
}

func TestMessageRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 6; i++ {
		msg := &models.Message{
			MessageSID: "SM_TOTAL_" + string(rune('0'+i)),
			Direction:  "inbound",
			FromNumber: "+15551234567",
			ToNumber:   "+15559876543",
			Body:       "Test",
			Status:     "received",
		}
		db.Messages.Create(ctx, msg)
	}

	count, err := db.Messages.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count != 6 {
		t.Errorf("Expected 6 messages, got %d", count)
	}
}

func TestMessageRepository_WithMediaURLs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	mediaURLs, _ := json.Marshal([]string{
		"https://example.com/image1.jpg",
		"https://example.com/image2.jpg",
	})

	msg := &models.Message{
		MessageSID: "SM_MMS_123",
		Direction:  "inbound",
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Check these images!",
		MediaURLs:  mediaURLs,
		Status:     "received",
	}

	if err := db.Messages.Create(ctx, msg); err != nil {
		t.Fatalf("Failed to create MMS: %v", err)
	}

	retrieved, err := db.Messages.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("Failed to get MMS: %v", err)
	}

	var urls []string
	if err := json.Unmarshal(retrieved.MediaURLs, &urls); err != nil {
		t.Fatalf("Failed to unmarshal media URLs: %v", err)
	}

	if len(urls) != 2 {
		t.Errorf("Expected 2 media URLs, got %d", len(urls))
	}
}

func TestMessageRepository_GetConversationSummaries(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", SMSEnabled: true}
	db.DIDs.Create(ctx, did)

	// Create messages from different conversations
	conversations := []struct {
		direction string
		from      string
		to        string
		isRead    bool
	}{
		{"inbound", "+15559999999", "+15551234567", false},
		{"inbound", "+15559999999", "+15551234567", false},
		{"outbound", "+15551234567", "+15559999999", true},
		{"inbound", "+15558888888", "+15551234567", true},
		{"inbound", "+15558888888", "+15551234567", false},
	}

	for i, c := range conversations {
		msg := &models.Message{
			MessageSID: "SM_SUMM_" + string(rune('0'+i)),
			Direction:  c.direction,
			FromNumber: c.from,
			ToNumber:   c.to,
			DIDID:      &did.ID,
			Body:       "Test",
			Status:     "received",
			IsRead:     c.isRead,
		}
		db.Messages.Create(ctx, msg)
	}

	summaries, err := db.Messages.GetConversationSummaries(ctx, &did.ID)
	if err != nil {
		t.Fatalf("Failed to get summaries: %v", err)
	}

	if len(summaries) != 2 {
		t.Errorf("Expected 2 conversation summaries, got %d", len(summaries))
	}
}

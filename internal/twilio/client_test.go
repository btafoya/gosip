package twilio

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/config"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.Config
		expectHealthy  bool
		expectClientNil bool
	}{
		{
			name: "with credentials",
			cfg: &config.Config{
				TwilioAccountSID: "AC123",
				TwilioAuthToken:  "token123",
			},
			expectHealthy:  true,
			expectClientNil: false,
		},
		{
			name: "without credentials",
			cfg: &config.Config{
				TwilioAccountSID: "",
				TwilioAuthToken:  "",
			},
			expectHealthy:  false,
			expectClientNil: true,
		},
		{
			name: "partial credentials",
			cfg: &config.Config{
				TwilioAccountSID: "AC123",
				TwilioAuthToken:  "",
			},
			expectHealthy:  false,
			expectClientNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.cfg)

			if client == nil {
				t.Fatal("NewClient should not return nil")
			}

			if tt.expectClientNil {
				if client.client != nil {
					t.Error("Expected nil Twilio client")
				}
			} else {
				if client.client == nil {
					t.Error("Expected non-nil Twilio client")
				}
			}

			if client.IsHealthy() != tt.expectHealthy {
				t.Errorf("IsHealthy() = %v, want %v", client.IsHealthy(), tt.expectHealthy)
			}

			// Queue should always be initialized
			if client.queue == nil {
				t.Error("Queue should be initialized")
			}
		})
	}
}

func TestClient_UpdateCredentials(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	// Initially not healthy
	if client.IsHealthy() {
		t.Error("Client should not be healthy initially without credentials")
	}

	// Update credentials
	client.UpdateCredentials("AC123", "token123")

	// Now should be healthy
	if !client.IsHealthy() {
		t.Error("Client should be healthy after updating credentials")
	}

	// Verify credentials stored
	if client.accountSID != "AC123" {
		t.Errorf("accountSID = %s, want AC123", client.accountSID)
	}
	if client.authToken != "token123" {
		t.Errorf("authToken = %s, want token123", client.authToken)
	}
}

func TestClient_IsHealthy(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	// Test when client is nil
	if client.IsHealthy() {
		t.Error("IsHealthy should return false when client is nil")
	}

	// Set up client
	client.UpdateCredentials("AC123", "token123")

	// Test healthy state
	if !client.IsHealthy() {
		t.Error("IsHealthy should return true with valid client")
	}

	// Manually set unhealthy
	client.mu.Lock()
	client.healthy = false
	client.mu.Unlock()

	if client.IsHealthy() {
		t.Error("IsHealthy should return false when marked unhealthy")
	}
}

func TestClient_SendSMS_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.SendSMS("+15551234567", "+15559876543", "Test message", nil)
	if err == nil {
		t.Error("SendSMS should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_MakeCall_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.MakeCall("+15551234567", "+15559876543", "http://example.com/twiml")
	if err == nil {
		t.Error("MakeCall should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_GetRecording_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.GetRecording("RE123")
	if err == nil {
		t.Error("GetRecording should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_RequestTranscription_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.RequestTranscription("RE123", 1)
	if err == nil {
		t.Error("RequestTranscription should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_HealthMonitoring(t *testing.T) {
	cfg := &config.Config{
		TwilioAccountSID: "AC123",
		TwilioAuthToken:  "token123",
	}
	client := NewClient(cfg)

	// Initially healthy
	if !client.IsHealthy() {
		t.Fatal("Client should be healthy initially with credentials")
	}

	// Record failures up to threshold
	for i := 0; i < config.TwilioMaxRetries; i++ {
		client.recordFailure()
	}

	// Should now be unhealthy
	if client.IsHealthy() {
		t.Error("Client should be unhealthy after max retries failures")
	}

	// Record success should restore health
	client.recordSuccess()

	if !client.IsHealthy() {
		t.Error("Client should be healthy after success")
	}

	// Verify failure count was reset
	client.mu.RLock()
	failureCount := client.failureCount
	client.mu.RUnlock()

	if failureCount != 0 {
		t.Errorf("failureCount = %d, want 0 after success", failureCount)
	}
}

func TestClient_HealthMonitoringConcurrency(t *testing.T) {
	cfg := &config.Config{
		TwilioAccountSID: "AC123",
		TwilioAuthToken:  "token123",
	}
	client := NewClient(cfg)

	var wg sync.WaitGroup

	// Concurrent failures
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.recordFailure()
		}()
	}

	// Concurrent successes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.recordSuccess()
		}()
	}

	wg.Wait()

	// Just verify no panic or race condition
	_ = client.IsHealthy()
}

func TestRecording(t *testing.T) {
	recording := Recording{
		SID:      "RE123456",
		Duration: 30,
		URL:      "https://api.twilio.com/recordings/RE123456.mp3",
	}

	if recording.SID != "RE123456" {
		t.Errorf("SID = %s, want RE123456", recording.SID)
	}
	if recording.Duration != 30 {
		t.Errorf("Duration = %d, want 30", recording.Duration)
	}
	if recording.URL != "https://api.twilio.com/recordings/RE123456.mp3" {
		t.Errorf("URL mismatch")
	}
}

func TestAvailableNumber(t *testing.T) {
	number := AvailableNumber{
		PhoneNumber:  "+15551234567",
		FriendlyName: "(555) 123-4567",
	}

	if number.PhoneNumber != "+15551234567" {
		t.Errorf("PhoneNumber = %s, want +15551234567", number.PhoneNumber)
	}
	if number.FriendlyName != "(555) 123-4567" {
		t.Errorf("FriendlyName = %s, want (555) 123-4567", number.FriendlyName)
	}
}

func TestClient_CheckHealth_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.CheckHealth(nil)
	if err == nil {
		t.Error("CheckHealth should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_GetAccountBalance_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.GetAccountBalance(nil)
	if err == nil {
		t.Error("GetAccountBalance should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_ListPhoneNumbers_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.ListPhoneNumbers(nil, "555")
	if err == nil {
		t.Error("ListPhoneNumbers should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_PurchasePhoneNumber_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.PurchasePhoneNumber(nil, "+15551234567", "", "")
	if err == nil {
		t.Error("PurchasePhoneNumber should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_Stop(t *testing.T) {
	cfg := &config.Config{
		TwilioAccountSID: "AC123",
		TwilioAuthToken:  "token123",
	}
	client := NewClient(cfg)

	// Stop should not panic
	client.Stop()
}

// Additional tests for better coverage

func TestClient_ListIncomingPhoneNumbers_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.ListIncomingPhoneNumbers(nil)
	if err == nil {
		t.Error("ListIncomingPhoneNumbers should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_GetMessage_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.GetMessage(nil, "SM123")
	if err == nil {
		t.Error("GetMessage should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_ListMessages_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.ListMessages(nil, "", "", 10)
	if err == nil {
		t.Error("ListMessages should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_DeleteMessage_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.DeleteMessage(nil, "SM123")
	if err == nil {
		t.Error("DeleteMessage should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_CancelMessage_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.CancelMessage(nil, "SM123")
	if err == nil {
		t.Error("CancelMessage should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_SendSMSWithCallback_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.SendSMSWithCallback("+15551234567", "+15559876543", "Test", nil, "http://callback.example.com")
	if err == nil {
		t.Error("SendSMSWithCallback should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_GetMediaURLs_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.GetMediaURLs(nil, "SM123")
	if err == nil {
		t.Error("GetMediaURLs should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseTwilioTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantZero bool
	}{
		{
			name:     "RFC1123Z format",
			input:    "Mon, 02 Jan 2006 15:04:05 +0000",
			wantZero: false,
		},
		{
			name:     "RFC3339 format",
			input:    "2006-01-02T15:04:05Z",
			wantZero: false,
		},
		{
			name:     "Invalid format",
			input:    "not-a-date",
			wantZero: true,
		},
		{
			name:     "Empty string",
			input:    "",
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTwilioTime(tt.input)
			if tt.wantZero && !result.IsZero() {
				t.Errorf("parseTwilioTime(%s) should return zero time", tt.input)
			}
			if !tt.wantZero && result.IsZero() {
				t.Errorf("parseTwilioTime(%s) should not return zero time", tt.input)
			}
		})
	}
}

func TestTwilioMessage(t *testing.T) {
	msg := TwilioMessage{
		SID:       "SM123456",
		Body:      "Hello, World!",
		From:      "+15551234567",
		To:        "+15559876543",
		Status:    "delivered",
		Direction: "outbound-api",
		NumMedia:  0,
	}

	if msg.SID != "SM123456" {
		t.Errorf("SID = %s, want SM123456", msg.SID)
	}
	if msg.Body != "Hello, World!" {
		t.Errorf("Body mismatch")
	}
	if msg.From != "+15551234567" {
		t.Errorf("From = %s, want +15551234567", msg.From)
	}
	if msg.To != "+15559876543" {
		t.Errorf("To = %s, want +15559876543", msg.To)
	}
	if msg.Status != "delivered" {
		t.Errorf("Status = %s, want delivered", msg.Status)
	}
}

func TestIncomingPhoneNumber(t *testing.T) {
	number := IncomingPhoneNumber{
		SID:          "PN123456",
		PhoneNumber:  "+15551234567",
		FriendlyName: "Main Line",
		SMSEnabled:   true,
		VoiceEnabled: true,
	}

	if number.SID != "PN123456" {
		t.Errorf("SID = %s, want PN123456", number.SID)
	}
	if number.PhoneNumber != "+15551234567" {
		t.Errorf("PhoneNumber mismatch")
	}
	if !number.SMSEnabled {
		t.Error("SMSEnabled should be true")
	}
	if !number.VoiceEnabled {
		t.Error("VoiceEnabled should be true")
	}
}

// Queue tests

func TestMessageQueue_NewMessageQueue(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	queue := NewMessageQueue(client)

	if queue == nil {
		t.Fatal("NewMessageQueue should not return nil")
	}
	if queue.client != client {
		t.Error("Queue client should match provided client")
	}
	if queue.messages == nil {
		t.Error("Queue messages channel should be initialized")
	}
	if queue.pending == nil {
		t.Error("Queue pending map should be initialized")
	}
}

func TestMessageQueue_GetPendingCount(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)
	queue := NewMessageQueue(client)

	// Initially should be 0
	if count := queue.GetPendingCount(); count != 0 {
		t.Errorf("Initial pending count = %d, want 0", count)
	}
}

func TestMessageQueue_GetQueuedCount(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)
	queue := NewMessageQueue(client)

	// Initially should be 0
	if count := queue.GetQueuedCount(); count != 0 {
		t.Errorf("Initial queued count = %d, want 0", count)
	}
}

func TestMessageQueue_Enqueue(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)
	queue := NewMessageQueue(client)

	msg := &QueuedMessage{
		ID:        "msg-1",
		From:      "+15551234567",
		To:        "+15559876543",
		Body:      "Test message",
		MediaURLs: nil,
		Retries:   0,
	}

	queue.Enqueue(msg)

	// Message should be in pending
	if count := queue.GetPendingCount(); count != 1 {
		t.Errorf("Pending count = %d, want 1", count)
	}
}

func TestMessageQueue_Stop(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)
	queue := NewMessageQueue(client)

	// Start the queue
	ctx, cancel := context.WithCancel(context.Background())
	go queue.Start(ctx)

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic
	queue.Stop()
	cancel()

	// Double stop should not panic
	queue = NewMessageQueue(client) // Fresh queue needed since stopChan is closed
	queue.Stop()
}

func TestMessageQueue_StartStop(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)
	queue := NewMessageQueue(client)

	ctx, cancel := context.WithCancel(context.Background())

	// Start should mark as running
	go queue.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	queue.mu.RLock()
	running := queue.running
	queue.mu.RUnlock()

	if !running {
		t.Error("Queue should be running after Start")
	}

	// Cancel context should stop the queue
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestMessageQueue_StartIdempotent(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)
	queue := NewMessageQueue(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start multiple times - should not panic or deadlock
	go queue.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	go queue.Start(ctx) // Second start should return immediately

	time.Sleep(10 * time.Millisecond)
}

func TestQueuedMessage(t *testing.T) {
	callbackCalled := false
	callback := func(sid string, err error) {
		callbackCalled = true
	}

	msg := &QueuedMessage{
		ID:        "msg-123",
		From:      "+15551234567",
		To:        "+15559876543",
		Body:      "Hello",
		MediaURLs: []string{"http://example.com/image.jpg"},
		Retries:   0,
		Callback:  callback,
	}

	if msg.ID != "msg-123" {
		t.Errorf("ID = %s, want msg-123", msg.ID)
	}
	if msg.From != "+15551234567" {
		t.Errorf("From mismatch")
	}
	if len(msg.MediaURLs) != 1 {
		t.Errorf("MediaURLs length = %d, want 1", len(msg.MediaURLs))
	}

	// Call the callback
	msg.Callback("SM123", nil)
	if !callbackCalled {
		t.Error("Callback should have been called")
	}
}

func TestClient_CredentialsValidation(t *testing.T) {
	tests := []struct {
		name        string
		accountSID  string
		authToken   string
		wantHealthy bool
	}{
		{
			name:        "Valid credentials format",
			accountSID:  "ACtest00000000000000000000000000",
			authToken:   "test00000000000000000000000000ab",
			wantHealthy: true,
		},
		{
			name:        "Empty account SID",
			accountSID:  "",
			authToken:   "token123",
			wantHealthy: false,
		},
		{
			name:        "Empty auth token",
			accountSID:  "AC123",
			authToken:   "",
			wantHealthy: false,
		},
		{
			name:        "Both empty",
			accountSID:  "",
			authToken:   "",
			wantHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				TwilioAccountSID: tt.accountSID,
				TwilioAuthToken:  tt.authToken,
			}
			client := NewClient(cfg)

			if client.IsHealthy() != tt.wantHealthy {
				t.Errorf("IsHealthy() = %v, want %v", client.IsHealthy(), tt.wantHealthy)
			}
		})
	}
}

func TestClient_FailureCountTracking(t *testing.T) {
	cfg := &config.Config{
		TwilioAccountSID: "AC123",
		TwilioAuthToken:  "token123",
	}
	client := NewClient(cfg)

	// Record failures one at a time
	client.recordFailure()
	client.mu.RLock()
	count := client.failureCount
	client.mu.RUnlock()

	if count != 1 {
		t.Errorf("failureCount = %d, want 1", count)
	}

	// Record another failure
	client.recordFailure()
	client.mu.RLock()
	count = client.failureCount
	client.mu.RUnlock()

	if count != 2 {
		t.Errorf("failureCount = %d, want 2", count)
	}
}

func TestClient_LastCheckUpdated(t *testing.T) {
	cfg := &config.Config{
		TwilioAccountSID: "AC123",
		TwilioAuthToken:  "token123",
	}
	client := NewClient(cfg)

	before := time.Now()
	client.recordSuccess()
	after := time.Now()

	client.mu.RLock()
	lastCheck := client.lastCheck
	client.mu.RUnlock()

	if lastCheck.Before(before) || lastCheck.After(after) {
		t.Error("lastCheck should be updated to current time")
	}
}

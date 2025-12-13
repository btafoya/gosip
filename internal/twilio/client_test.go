package twilio

import (
	"sync"
	"testing"

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

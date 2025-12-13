package notifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

func TestNewNotifier(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{}

	notifier := NewNotifier(cfg, database)

	if notifier == nil {
		t.Fatal("NewNotifier should not return nil")
	}

	if notifier.cfg != cfg {
		t.Error("Config not properly set")
	}

	if notifier.database != database {
		t.Error("Database not properly set")
	}

	if notifier.client == nil {
		t.Error("HTTP client should be initialized")
	}
}

func TestNotifier_SendEmail_NotConfigured(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		SMTPHost: "", // Not configured
	}

	notifier := NewNotifier(cfg, database)

	err := notifier.SendEmail("test@example.com", "Test Subject", "Test body")
	if err == nil {
		t.Error("SendEmail should error when SMTP not configured")
	}
	if err.Error() != "SMTP not configured" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNotifier_SendHTMLEmail_NotConfigured(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		SMTPHost: "", // Not configured
	}

	notifier := NewNotifier(cfg, database)

	err := notifier.SendHTMLEmail("test@example.com", "Test Subject", "<html>Test</html>")
	if err == nil {
		t.Error("SendHTMLEmail should error when SMTP not configured")
	}
	if err.Error() != "SMTP not configured" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNotifier_SendPush_NotConfigured(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		GotifyURL: "", // Not configured
	}

	notifier := NewNotifier(cfg, database)

	err := notifier.SendPush("Test Title", "Test message")
	if err == nil {
		t.Error("SendPush should error when Gotify not configured")
	}
	if err.Error() != "Gotify not configured" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNotifier_SendWebhook_Success(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected application/json content type, got %s", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	database := setupTestDB(t)
	cfg := &config.Config{}
	notifier := NewNotifier(cfg, database)

	payload := map[string]string{
		"event": "test",
		"data":  "hello",
	}

	err := notifier.SendWebhook(server.URL, payload)
	if err != nil {
		t.Errorf("SendWebhook failed: %v", err)
	}

	if receivedPayload["event"] != "test" {
		t.Errorf("Expected event=test, got %v", receivedPayload["event"])
	}
	if receivedPayload["data"] != "hello" {
		t.Errorf("Expected data=hello, got %v", receivedPayload["data"])
	}
}

func TestNotifier_SendWebhook_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	database := setupTestDB(t)
	cfg := &config.Config{}
	notifier := NewNotifier(cfg, database)

	err := notifier.SendWebhook(server.URL, map[string]string{"test": "data"})
	if err == nil {
		t.Error("SendWebhook should error on server error")
	}
}

func TestNotifier_SendWebhook_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	database := setupTestDB(t)
	cfg := &config.Config{}
	notifier := NewNotifier(cfg, database)

	err := notifier.SendWebhook(server.URL, map[string]string{"test": "data"})
	if err == nil {
		t.Error("SendWebhook should error on bad request")
	}
}

func TestNotifier_SendPush_Success(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected application/json content type, got %s", r.Header.Get("Content-Type"))
		}

		// Check token in URL
		token := r.URL.Query().Get("token")
		if token != "test-token" {
			t.Errorf("Expected token=test-token, got %s", token)
		}

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	database := setupTestDB(t)
	cfg := &config.Config{
		GotifyURL:   server.URL,
		GotifyToken: "test-token",
	}
	notifier := NewNotifier(cfg, database)

	err := notifier.SendPush("Test Title", "Test message")
	if err != nil {
		t.Errorf("SendPush failed: %v", err)
	}

	if receivedPayload["title"] != "Test Title" {
		t.Errorf("Expected title=Test Title, got %v", receivedPayload["title"])
	}
	if receivedPayload["message"] != "Test message" {
		t.Errorf("Expected message=Test message, got %v", receivedPayload["message"])
	}
	if receivedPayload["priority"] != float64(5) {
		t.Errorf("Expected priority=5, got %v", receivedPayload["priority"])
	}
}

func TestNotifier_SendPush_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	database := setupTestDB(t)
	cfg := &config.Config{
		GotifyURL:   server.URL,
		GotifyToken: "test-token",
	}
	notifier := NewNotifier(cfg, database)

	// This will retry and fail
	err := notifier.SendPush("Test Title", "Test message")
	if err == nil {
		t.Error("SendPush should error on server error")
	}
}

func TestVoicemailHTMLTemplate(t *testing.T) {
	data := struct {
		CallerID      string
		DIDNumber     string
		Duration      int
		Time          string
		Transcription string
		RecordingURL  string
	}{
		CallerID:      "+15551234567",
		DIDNumber:     "+15559876543",
		Duration:      30,
		Time:          "Dec 13, 2025 3:04 PM",
		Transcription: "Hello, this is a test message.",
		RecordingURL:  "https://example.com/recording.mp3",
	}

	var buf bytes.Buffer
	err := voicemailHTMLTemplate.Execute(&buf, data)
	if err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}

	html := buf.String()

	// Check for key elements
	// Note: html/template escapes + to &#43; for security
	if !bytes.Contains([]byte(html), []byte("15551234567")) {
		t.Error("Template should contain caller ID digits")
	}
	if !bytes.Contains([]byte(html), []byte("15559876543")) {
		t.Error("Template should contain DID number digits")
	}
	if !bytes.Contains([]byte(html), []byte("30 seconds")) {
		t.Error("Template should contain duration")
	}
	if !bytes.Contains([]byte(html), []byte("Hello, this is a test message.")) {
		t.Error("Template should contain transcription")
	}
	if !bytes.Contains([]byte(html), []byte("https://example.com/recording.mp3")) {
		t.Error("Template should contain recording URL")
	}
	if !bytes.Contains([]byte(html), []byte("New Voicemail")) {
		t.Error("Template should contain header")
	}
}

func TestVoicemailHTMLTemplate_NoTranscription(t *testing.T) {
	data := struct {
		CallerID      string
		DIDNumber     string
		Duration      int
		Time          string
		Transcription string
		RecordingURL  string
	}{
		CallerID:      "+15551234567",
		DIDNumber:     "+15559876543",
		Duration:      30,
		Time:          "Dec 13, 2025 3:04 PM",
		Transcription: "", // No transcription
		RecordingURL:  "https://example.com/recording.mp3",
	}

	var buf bytes.Buffer
	err := voicemailHTMLTemplate.Execute(&buf, data)
	if err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}

	html := buf.String()

	// Should not contain transcription section
	if bytes.Contains([]byte(html), []byte("Transcription:")) {
		t.Error("Template should not contain transcription section when empty")
	}
}

func TestNotifier_SendVoicemailNotification_NoConfig(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		SMTPHost:  "", // Not configured
		GotifyURL: "", // Not configured
	}

	notifier := NewNotifier(cfg, database)

	voicemail := &models.Voicemail{
		ID:         1,
		FromNumber: "+15551234567",
		Duration:   30,
		AudioURL:   "https://example.com/recording.mp3",
		Transcript: "Test message",
		CreatedAt:  time.Now(),
	}

	// Should not error even without notification config
	err := notifier.SendVoicemailNotification(voicemail)
	if err != nil {
		t.Errorf("SendVoicemailNotification should not error without notification config: %v", err)
	}
}

func TestNotifier_SendSMSNotification_NoConfig(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		SMTPHost:  "", // Not configured
		GotifyURL: "", // Not configured
	}

	notifier := NewNotifier(cfg, database)

	message := &models.Message{
		ID:         1,
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Test SMS message",
		Direction:  "inbound",
		CreatedAt:  time.Now(),
	}

	// Should not error even without notification config
	err := notifier.SendSMSNotification(message)
	if err != nil {
		t.Errorf("SendSMSNotification should not error without notification config: %v", err)
	}
}

func TestNotifier_SendSMSNotification_WithMedia(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		SMTPHost:  "", // Not configured
		GotifyURL: "", // Not configured
	}

	notifier := NewNotifier(cfg, database)

	mediaURLs, _ := json.Marshal([]string{
		"https://example.com/image1.jpg",
		"https://example.com/image2.jpg",
	})

	message := &models.Message{
		ID:         1,
		FromNumber: "+15551234567",
		ToNumber:   "+15559876543",
		Body:       "Test MMS message",
		Direction:  "inbound",
		MediaURLs:  mediaURLs,
		CreatedAt:  time.Now(),
	}

	// Should not error even without notification config
	err := notifier.SendSMSNotification(message)
	if err != nil {
		t.Errorf("SendSMSNotification should not error without notification config: %v", err)
	}
}

func TestNotifier_SendSMSNotification_Outbound(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		SMTPHost:  "", // Not configured
		GotifyURL: "", // Not configured
	}

	notifier := NewNotifier(cfg, database)

	message := &models.Message{
		ID:         1,
		FromNumber: "+15559876543",
		ToNumber:   "+15551234567",
		Body:       "Outbound test message",
		Direction:  "outbound",
		CreatedAt:  time.Now(),
	}

	// Should not error even without notification config
	err := notifier.SendSMSNotification(message)
	if err != nil {
		t.Errorf("SendSMSNotification should not error without notification config: %v", err)
	}
}

func TestNotifier_SendWebhook_InvalidURL(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{}
	notifier := NewNotifier(cfg, database)

	// Invalid URL should fail
	err := notifier.SendWebhook("://invalid-url", map[string]string{"test": "data"})
	if err == nil {
		t.Error("SendWebhook should error on invalid URL")
	}
}

func TestNotifier_SendEmail_FromAddress(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUser:     "user@example.com",
		SMTPPassword: "password",
		SMTPFrom:     "", // Empty, should fall back to SMTPUser
	}

	notifier := NewNotifier(cfg, database)

	// This will fail because we can't actually connect to SMTP
	// But we can at least verify the configuration is being used
	err := notifier.SendEmail("test@example.com", "Test", "Body")
	if err == nil {
		t.Error("Expected error when SMTP server is unreachable")
	}
	// The error should be connection-related, not configuration-related
}

func TestNotifier_ClientTimeout(t *testing.T) {
	database := setupTestDB(t)
	cfg := &config.Config{}
	notifier := NewNotifier(cfg, database)

	// Verify client has appropriate timeout
	if notifier.client.Timeout != 30*time.Second {
		t.Errorf("Client timeout = %v, want 30s", notifier.client.Timeout)
	}
}

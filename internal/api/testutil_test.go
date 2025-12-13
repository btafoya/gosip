package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
	_ "github.com/mattn/go-sqlite3"
)

// MockTwilioClient is a mock implementation of TwilioClient for testing
type MockTwilioClient struct {
	SendSMSFunc             func(from, to, body string, mediaURLs []string) (string, error)
	UpdateCredentialsFunc   func(accountSID, authToken string)
	IsHealthyFunc           func() bool
	RequestTranscriptionFunc func(recordingSID string, voicemailID int64) error
}

func (m *MockTwilioClient) SendSMS(from, to, body string, mediaURLs []string) (string, error) {
	if m.SendSMSFunc != nil {
		return m.SendSMSFunc(from, to, body, mediaURLs)
	}
	return "SM123456789", nil
}

func (m *MockTwilioClient) UpdateCredentials(accountSID, authToken string) {
	if m.UpdateCredentialsFunc != nil {
		m.UpdateCredentialsFunc(accountSID, authToken)
	}
}

func (m *MockTwilioClient) IsHealthy() bool {
	if m.IsHealthyFunc != nil {
		return m.IsHealthyFunc()
	}
	return true
}

func (m *MockTwilioClient) RequestTranscription(recordingSID string, voicemailID int64) error {
	if m.RequestTranscriptionFunc != nil {
		return m.RequestTranscriptionFunc(recordingSID, voicemailID)
	}
	return nil
}

// MockNotifier is a mock implementation of Notifier for testing
type MockNotifier struct {
	SendVoicemailNotificationFunc func(voicemail *models.Voicemail) error
	SendSMSNotificationFunc       func(message *models.Message) error
	SendEmailFunc                 func(to, subject, body string) error
	SendPushFunc                  func(title, message string) error
}

func (m *MockNotifier) SendVoicemailNotification(voicemail *models.Voicemail) error {
	if m.SendVoicemailNotificationFunc != nil {
		return m.SendVoicemailNotificationFunc(voicemail)
	}
	return nil
}

func (m *MockNotifier) SendSMSNotification(message *models.Message) error {
	if m.SendSMSNotificationFunc != nil {
		return m.SendSMSNotificationFunc(message)
	}
	return nil
}

func (m *MockNotifier) SendEmail(to, subject, body string) error {
	if m.SendEmailFunc != nil {
		return m.SendEmailFunc(to, subject, body)
	}
	return nil
}

func (m *MockNotifier) SendPush(title, message string) error {
	if m.SendPushFunc != nil {
		return m.SendPushFunc(title, message)
	}
	return nil
}

// MockRegistrar is a mock SIP registrar for testing
type MockRegistrar struct {
	registrations map[int64]bool
}

func NewMockRegistrar() *MockRegistrar {
	return &MockRegistrar{
		registrations: make(map[int64]bool),
	}
}

func (m *MockRegistrar) IsRegistered(ctx context.Context, deviceID int64) bool {
	return m.registrations[deviceID]
}

func (m *MockRegistrar) SetRegistered(deviceID int64, registered bool) {
	m.registrations[deviceID] = registered
}

func (m *MockRegistrar) GetRegistrationCount() int {
	count := 0
	for _, registered := range m.registrations {
		if registered {
			count++
		}
	}
	return count
}

// MockSIPServer is a minimal mock of the SIP server for testing
type MockSIPServer struct {
	registrar   *MockRegistrar
	running     bool
	activeCalls int
}

func NewMockSIPServer() *MockSIPServer {
	return &MockSIPServer{
		registrar:   NewMockRegistrar(),
		running:     true,
		activeCalls: 0,
	}
}

func (m *MockSIPServer) GetRegistrar() *MockRegistrar {
	return m.registrar
}

func (m *MockSIPServer) GetActiveRegistrations(ctx context.Context) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (m *MockSIPServer) IsRunning() bool {
	return m.running
}

func (m *MockSIPServer) GetActiveCallCount() int {
	return m.activeCalls
}

func (m *MockSIPServer) SetRunning(running bool) {
	m.running = running
}

func (m *MockSIPServer) SetActiveCallCount(count int) {
	m.activeCalls = count
}

// testSetup contains all the test dependencies
type testSetup struct {
	DB       *db.DB
	Twilio   *MockTwilioClient
	Notifier *MockNotifier
	SIP      *MockSIPServer
	Deps     *testDependencies
}

// testDependencies wraps Dependencies for testing with our mocks
type testDependencies struct {
	DB       *db.DB
	SIP      *MockSIPServer
	Twilio   TwilioClient
	Notifier Notifier
}

// setupTestAPI creates a test environment with mocked dependencies
func setupTestAPI(t *testing.T) *testSetup {
	t.Helper()

	// Create in-memory database
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations to create tables
	if err := database.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	mockTwilio := &MockTwilioClient{}
	mockNotifier := &MockNotifier{}
	mockSIP := NewMockSIPServer()

	return &testSetup{
		DB:       database,
		Twilio:   mockTwilio,
		Notifier: mockNotifier,
		SIP:      mockSIP,
		Deps: &testDependencies{
			DB:       database,
			SIP:      mockSIP,
			Twilio:   mockTwilio,
			Notifier: mockNotifier,
		},
	}
}

// createTestUser creates a user for testing and returns it
func createTestUser(t *testing.T, database *db.DB, email, password, role string) *models.User {
	t.Helper()

	user := &models.User{
		Email:        email,
		PasswordHash: hashPassword(password),
		Role:         role,
	}

	if err := database.Users.Create(context.Background(), user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

// createTestDevice creates a device for testing
func createTestDevice(t *testing.T, database *db.DB, name, username string) *models.Device {
	t.Helper()

	device := &models.Device{
		Name:       name,
		Username:   username,
		DeviceType: "softphone",
	}

	if err := database.Devices.Create(context.Background(), device); err != nil {
		t.Fatalf("Failed to create test device: %v", err)
	}

	return device
}

// createTestDID creates a DID for testing
func createTestDID(t *testing.T, database *db.DB, number string) *models.DID {
	t.Helper()

	did := &models.DID{
		Number:       number,
		VoiceEnabled: true,
		SMSEnabled:   true,
	}

	if err := database.DIDs.Create(context.Background(), did); err != nil {
		t.Fatalf("Failed to create test DID: %v", err)
	}

	return did
}

// cdrCounter is used to generate unique CallSIDs for test CDRs
var cdrCounter int64

// createTestCDR creates a CDR for testing
func createTestCDR(t *testing.T, database *db.DB, didID int64, direction, fromNumber, toNumber string) *models.CDR {
	t.Helper()

	cdrCounter++
	didIDPtr := didID
	cdr := &models.CDR{
		CallSID:     fmt.Sprintf("CA%d%d", time.Now().UnixNano(), cdrCounter),
		DIDID:       &didIDPtr,
		Direction:   direction,
		FromNumber:  fromNumber,
		ToNumber:    toNumber,
		Duration:    60,
		Disposition: "answered",
	}

	if err := database.CDRs.Create(context.Background(), cdr); err != nil {
		t.Fatalf("Failed to create test CDR: %v", err)
	}

	return cdr
}

// createTestVoicemail creates a voicemail for testing
func createTestVoicemail(t *testing.T, database *db.DB, userID int64, fromNumber string) *models.Voicemail {
	t.Helper()

	voicemail := &models.Voicemail{
		UserID:     &userID,
		FromNumber: fromNumber,
		Duration:   30,
		AudioURL:   "https://example.com/recording.mp3",
		Transcript: "Test voicemail message",
		IsRead:     false,
	}

	if err := database.Voicemails.Create(context.Background(), voicemail); err != nil {
		t.Fatalf("Failed to create test voicemail: %v", err)
	}

	return voicemail
}

// messageCounter is used to generate unique MessageSIDs for test messages
var messageCounter int64

// createTestMessage creates a message for testing
func createTestMessage(t *testing.T, database *db.DB, didID int64, direction, remoteNumber, body string) *models.Message {
	t.Helper()

	messageCounter++
	didIDPtr := didID
	var fromNumber, toNumber string
	if direction == "inbound" {
		fromNumber = remoteNumber
		toNumber = "+15551234567" // Local DID placeholder
	} else {
		fromNumber = "+15551234567" // Local DID placeholder
		toNumber = remoteNumber
	}

	message := &models.Message{
		MessageSID: fmt.Sprintf("SM%d%d", time.Now().UnixNano(), messageCounter),
		DIDID:      &didIDPtr,
		Direction:  direction,
		FromNumber: fromNumber,
		ToNumber:   toNumber,
		Body:       body,
		Status:     "delivered",
	}

	if err := database.Messages.Create(context.Background(), message); err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}

	return message
}

// createTestAutoReply creates an auto-reply for testing
func createTestAutoReply(t *testing.T, database *db.DB, triggerType, replyText string) *models.AutoReply {
	t.Helper()

	autoReply := &models.AutoReply{
		TriggerType: triggerType,
		ReplyText:   replyText,
		Enabled:     true,
	}

	if err := database.AutoReplies.Create(context.Background(), autoReply); err != nil {
		t.Fatalf("Failed to create test auto-reply: %v", err)
	}

	return autoReply
}

// hashPassword creates a bcrypt hash for testing
func hashPassword(password string) string {
	// Using a simple bcrypt hash for testing
	// In production, this would use bcrypt.GenerateFromPassword
	return "$2a$10$test.hash.for." + password
}

// makeRequest is a helper to create and execute HTTP requests in tests
func makeRequest(t *testing.T, method, url string, body interface{}, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, url, reqBody)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	return rr
}

// makeAuthenticatedRequest creates a request with a user in context
func makeAuthenticatedRequest(t *testing.T, method, url string, body interface{}, handler http.Handler, user *models.User) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, url, reqBody)
	req.Header.Set("Content-Type", "application/json")

	// Add user to context
	ctx := context.WithValue(req.Context(), contextKeyUser, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	return rr
}

// withURLParams adds chi URL parameters to a request
func withURLParams(r *http.Request, params map[string]string) *http.Request {
	ctx := chi.NewRouteContext()
	for key, value := range params {
		ctx.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
}

// decodeResponse decodes a JSON response into the given interface
func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder, v interface{}) {
	t.Helper()

	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("Failed to decode response: %v (body: %s)", err, rr.Body.String())
	}
}

// assertStatus checks the HTTP status code
func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, expected int) {
	t.Helper()

	if rr.Code != expected {
		t.Errorf("Expected status %d, got %d. Body: %s", expected, rr.Code, rr.Body.String())
	}
}

// assertErrorCode checks the error code in an error response
func assertErrorCode(t *testing.T, rr *httptest.ResponseRecorder, expectedCode string) {
	t.Helper()

	var errResp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errResp.Error.Code != expectedCode {
		t.Errorf("Expected error code %s, got %s", expectedCode, errResp.Error.Code)
	}
}

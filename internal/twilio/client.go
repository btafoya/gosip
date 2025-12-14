package twilio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// Client wraps the Twilio API client with retry logic and health monitoring
type Client struct {
	client      *twilio.RestClient
	accountSID  string
	authToken   string
	mu          sync.RWMutex
	healthy     bool
	lastCheck   time.Time
	failureCount int
	queue       *MessageQueue
	cfg         *config.Config
}

// NewClient creates a new Twilio client
func NewClient(cfg *config.Config) *Client {
	c := &Client{
		cfg:     cfg,
		healthy: false,
	}

	if cfg.TwilioAccountSID != "" && cfg.TwilioAuthToken != "" {
		c.UpdateCredentials(cfg.TwilioAccountSID, cfg.TwilioAuthToken)
	}

	c.queue = NewMessageQueue(c)

	return c
}

// UpdateCredentials updates the Twilio credentials and reinitializes the client
func (c *Client) UpdateCredentials(accountSID, authToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accountSID = accountSID
	c.authToken = authToken
	c.client = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSID,
		Password: authToken,
	})
	c.healthy = true
	c.failureCount = 0
}

// IsHealthy returns the current health status of the Twilio connection
func (c *Client) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthy && c.client != nil
}

// SendSMS sends an SMS message with retry logic
func (c *Client) SendSMS(from, to, body string, mediaURLs []string) (string, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return "", fmt.Errorf("twilio client not initialized")
	}
	c.mu.RUnlock()

	var lastErr error
	for attempt := 0; attempt < config.TwilioMaxRetries; attempt++ {
		sid, err := c.sendSMSOnce(from, to, body, mediaURLs)
		if err == nil {
			c.recordSuccess()
			return sid, nil
		}
		lastErr = err
		c.recordFailure()

		// Exponential backoff
		backoff := time.Duration(1<<uint(attempt)) * time.Second
		time.Sleep(backoff)
	}

	return "", fmt.Errorf("failed after %d retries: %w", config.TwilioMaxRetries, lastErr)
}

func (c *Client) sendSMSOnce(from, to, body string, mediaURLs []string) (string, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.CreateMessageParams{}
	params.SetFrom(from)
	params.SetTo(to)
	params.SetBody(body)

	if len(mediaURLs) > 0 {
		params.SetMediaUrl(mediaURLs)
	}

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		return "", fmt.Errorf("twilio API error: %w", err)
	}

	if resp.Sid == nil {
		return "", fmt.Errorf("no SID returned from Twilio")
	}

	return *resp.Sid, nil
}

// MakeCall initiates an outbound call
func (c *Client) MakeCall(from, to, url string) (string, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return "", fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.CreateCallParams{}
	params.SetFrom(from)
	params.SetTo(to)
	params.SetUrl(url)

	resp, err := client.Api.CreateCall(params)
	if err != nil {
		c.recordFailure()
		return "", fmt.Errorf("twilio API error: %w", err)
	}

	c.recordSuccess()

	if resp.Sid == nil {
		return "", fmt.Errorf("no SID returned from Twilio")
	}

	return *resp.Sid, nil
}

// RequestTranscription requests transcription for a recording
func (c *Client) RequestTranscription(recordingSID string, voicemailID int64) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	c.mu.RUnlock()

	// Twilio automatically transcribes if transcribe=true in TwiML
	// This method can be used to fetch existing transcription or trigger re-transcription
	// For now, transcription is handled via webhook callback

	return nil
}

// GetRecording fetches a recording by SID
func (c *Client) GetRecording(recordingSID string) (*Recording, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	resp, err := client.Api.FetchRecording(recordingSID, nil)
	if err != nil {
		return nil, fmt.Errorf("twilio API error: %w", err)
	}

	recording := &Recording{
		SID:      *resp.Sid,
		Duration: 0,
	}

	if resp.Duration != nil {
		fmt.Sscanf(*resp.Duration, "%d", &recording.Duration)
	}

	return recording, nil
}

// Recording represents a Twilio recording
type Recording struct {
	SID      string
	Duration int
	URL      string
}

// GetAccountBalance returns the current account balance
func (c *Client) GetAccountBalance(ctx context.Context) (float64, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return 0, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	resp, err := client.Api.FetchBalance(nil)
	if err != nil {
		return 0, fmt.Errorf("twilio API error: %w", err)
	}

	if resp.Balance == nil {
		return 0, nil
	}

	var balance float64
	fmt.Sscanf(*resp.Balance, "%f", &balance)

	return balance, nil
}

// ListPhoneNumbers returns available phone numbers for a given area code
func (c *Client) ListPhoneNumbers(ctx context.Context, areaCode string) ([]AvailableNumber, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.ListAvailablePhoneNumberLocalParams{}
	// Convert area code string to int for the Twilio API
	var areaCodeInt int
	fmt.Sscanf(areaCode, "%d", &areaCodeInt)
	params.SetAreaCode(areaCodeInt)
	params.SetSmsEnabled(true)
	params.SetVoiceEnabled(true)

	resp, err := client.Api.ListAvailablePhoneNumberLocal("US", params)
	if err != nil {
		return nil, fmt.Errorf("twilio API error: %w", err)
	}

	var numbers []AvailableNumber
	for _, n := range resp {
		if n.PhoneNumber != nil && n.FriendlyName != nil {
			numbers = append(numbers, AvailableNumber{
				PhoneNumber:  *n.PhoneNumber,
				FriendlyName: *n.FriendlyName,
			})
		}
	}

	return numbers, nil
}

// AvailableNumber represents an available phone number
type AvailableNumber struct {
	PhoneNumber  string
	FriendlyName string
}

// IncomingPhoneNumber represents an owned phone number
type IncomingPhoneNumber struct {
	SID           string
	PhoneNumber   string
	FriendlyName  string
	SMSEnabled    bool
	VoiceEnabled  bool
}

// ListIncomingPhoneNumbers returns phone numbers owned by the account
func (c *Client) ListIncomingPhoneNumbers(ctx context.Context) ([]IncomingPhoneNumber, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.ListIncomingPhoneNumberParams{}

	resp, err := client.Api.ListIncomingPhoneNumber(params)
	if err != nil {
		c.recordFailure()
		return nil, fmt.Errorf("twilio API error: %w", err)
	}

	c.recordSuccess()

	var numbers []IncomingPhoneNumber
	for _, n := range resp {
		number := IncomingPhoneNumber{}
		if n.Sid != nil {
			number.SID = *n.Sid
		}
		if n.PhoneNumber != nil {
			number.PhoneNumber = *n.PhoneNumber
		}
		if n.FriendlyName != nil {
			number.FriendlyName = *n.FriendlyName
		}
		if n.Capabilities != nil {
			number.SMSEnabled = n.Capabilities.Sms
			number.VoiceEnabled = n.Capabilities.Voice
		}
		numbers = append(numbers, number)
	}

	return numbers, nil
}

// PurchasePhoneNumber purchases a phone number
func (c *Client) PurchasePhoneNumber(ctx context.Context, phoneNumber, voiceURL, smsURL string) (string, error) {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return "", fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	c.mu.RUnlock()

	params := &twilioApi.CreateIncomingPhoneNumberParams{}
	params.SetPhoneNumber(phoneNumber)
	if voiceURL != "" {
		params.SetVoiceUrl(voiceURL)
	}
	if smsURL != "" {
		params.SetSmsUrl(smsURL)
	}

	resp, err := client.Api.CreateIncomingPhoneNumber(params)
	if err != nil {
		return "", fmt.Errorf("twilio API error: %w", err)
	}

	if resp.Sid == nil {
		return "", fmt.Errorf("no SID returned from Twilio")
	}

	return *resp.Sid, nil
}

// Health monitoring helpers

func (c *Client) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthy = true
	c.failureCount = 0
	c.lastCheck = time.Now()
}

func (c *Client) recordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failureCount++
	c.lastCheck = time.Now()

	if c.failureCount >= config.TwilioMaxRetries {
		c.healthy = false
	}
}

// CheckHealth performs a health check by validating credentials
func (c *Client) CheckHealth(ctx context.Context) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return fmt.Errorf("twilio client not initialized")
	}
	client := c.client
	accountSID := c.accountSID
	c.mu.RUnlock()

	_, err := client.Api.FetchAccount(accountSID)
	if err != nil {
		c.recordFailure()
		return err
	}

	c.recordSuccess()
	return nil
}

// Start starts background workers (queue processor, health checker)
func (c *Client) Start(ctx context.Context) {
	// Start message queue processor
	go c.queue.Start(ctx)

	// Start health checker
	go c.healthChecker(ctx)
}

func (c *Client) healthChecker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.CheckHealth(ctx)
		}
	}
}

// Stop gracefully stops the client
func (c *Client) Stop() {
	c.queue.Stop()
}

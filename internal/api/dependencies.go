package api

import (
	"context"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/btafoya/gosip/internal/twilio"
	"github.com/btafoya/gosip/pkg/sip"
)

// Dependencies holds all dependencies for API handlers
type Dependencies struct {
	DB       *db.DB
	SIP      *sip.Server
	Twilio   TwilioClient
	Notifier Notifier
	Config   *config.Config
}

// TwilioClient interface for Twilio operations
type TwilioClient interface {
	// SMS/MMS Operations
	SendSMS(from, to, body string, mediaURLs []string) (string, error)
	SendSMSWithCallback(from, to, body string, mediaURLs []string, statusCallback string) (string, error)
	GetMessage(ctx context.Context, messageSID string) (*twilio.TwilioMessage, error)
	ListMessages(ctx context.Context, from, to string, limit int) ([]*twilio.TwilioMessage, error)
	DeleteMessage(ctx context.Context, messageSID string) error
	CancelMessage(ctx context.Context, messageSID string) error
	ResendMessage(ctx context.Context, originalSID string) (string, error)
	GetMediaURLs(ctx context.Context, messageSID string) ([]string, error)

	// Voice Operations
	RequestTranscription(recordingSID string, voicemailID int64) error

	// Account Operations
	UpdateCredentials(accountSID, authToken string)
	IsHealthy() bool
	ListIncomingPhoneNumbers(ctx context.Context) ([]twilio.IncomingPhoneNumber, error)
}

// Notifier interface for sending notifications
type Notifier interface {
	SendVoicemailNotification(voicemail *models.Voicemail) error
	SendSMSNotification(message *models.Message) error
	SendEmail(to, subject, body string) error
	SendPush(title, message string) error
}

// NewDependencies creates a new Dependencies instance
func NewDependencies(cfg *config.Config, database *db.DB, sipServer *sip.Server, twilio TwilioClient, notifier Notifier) *Dependencies {
	return &Dependencies{
		DB:       database,
		SIP:      sipServer,
		Twilio:   twilio,
		Notifier: notifier,
		Config:   cfg,
	}
}

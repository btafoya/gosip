package api

import (
	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
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
	SendSMS(from, to, body string, mediaURLs []string) (string, error)
	UpdateCredentials(accountSID, authToken string)
	IsHealthy() bool
	RequestTranscription(recordingSID string, voicemailID int64) error
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

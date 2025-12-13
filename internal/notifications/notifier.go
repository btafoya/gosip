package notifications

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/smtp"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
)

// Notifier handles all notification types (email, push, webhooks)
type Notifier struct {
	cfg      *config.Config
	database *db.DB
	client   *http.Client
}

// NewNotifier creates a new notifier instance
func NewNotifier(cfg *config.Config, database *db.DB) *Notifier {
	return &Notifier{
		cfg:      cfg,
		database: database,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendVoicemailNotification sends notifications for a new voicemail
func (n *Notifier) SendVoicemailNotification(voicemail *models.Voicemail) error {
	ctx := context.Background()

	// Get DID info - UserID is a pointer
	var did *models.DID
	var err error
	if voicemail.UserID != nil {
		did, err = n.database.DIDs.GetByID(ctx, *voicemail.UserID)
		if err != nil {
			return fmt.Errorf("failed to get DID: %w", err)
		}
	}

	didNumber := ""
	if did != nil {
		didNumber = did.Number
	}

	subject := fmt.Sprintf("New Voicemail from %s", voicemail.FromNumber)
	body := fmt.Sprintf(`
You have a new voicemail:

From: %s
To: %s
Duration: %d seconds
Time: %s

%s

Listen to this voicemail at: %s
`, voicemail.FromNumber, didNumber, voicemail.Duration,
	voicemail.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
	func() string {
		if voicemail.Transcript != "" {
			return "Transcription:\n" + voicemail.Transcript
		}
		return ""
	}(),
	voicemail.AudioURL)

	// Send email notification - get notification email from database config
	if n.cfg.SMTPHost != "" {
		notificationEmail, _ := n.database.Config.Get(ctx, "notification_email")
		if notificationEmail != "" {
			if err := n.SendEmail(notificationEmail, subject, body); err != nil {
				// Log but don't fail
				fmt.Printf("Failed to send email notification: %v\n", err)
			}
		}
	}

	// Send push notification
	if n.cfg.GotifyURL != "" {
		if err := n.SendPush(subject, fmt.Sprintf("From %s - %d seconds", voicemail.FromNumber, voicemail.Duration)); err != nil {
			fmt.Printf("Failed to send push notification: %v\n", err)
		}
	}

	return nil
}

// SendSMSNotification sends notifications for a new SMS message
func (n *Notifier) SendSMSNotification(message *models.Message) error {
	ctx := context.Background()

	// Get DID info - DIDID is a pointer
	var did *models.DID
	var err error
	if message.DIDID != nil {
		did, err = n.database.DIDs.GetByID(ctx, *message.DIDID)
		if err != nil {
			return fmt.Errorf("failed to get DID: %w", err)
		}
	}

	didNumber := ""
	if did != nil {
		didNumber = did.Number
	}

	// Determine remote number based on direction
	remoteNumber := message.FromNumber
	if message.Direction == "outbound" {
		remoteNumber = message.ToNumber
	}

	subject := fmt.Sprintf("New SMS from %s", remoteNumber)
	body := fmt.Sprintf(`
You have a new text message:

From: %s
To: %s
Time: %s

Message:
%s
`, remoteNumber, didNumber, message.CreatedAt.Format("Jan 2, 2006 3:04 PM"), message.Body)

	// Check for media
	var mediaURLs []string
	if len(message.MediaURLs) > 0 {
		json.Unmarshal(message.MediaURLs, &mediaURLs)
		if len(mediaURLs) > 0 {
			body += "\n\nMedia attachments:\n"
			for _, url := range mediaURLs {
				body += url + "\n"
			}
		}
	}

	// Send email notification - get notification email from database config
	if n.cfg.SMTPHost != "" {
		notificationEmail, _ := n.database.Config.Get(ctx, "notification_email")
		if notificationEmail != "" {
			if err := n.SendEmail(notificationEmail, subject, body); err != nil {
				fmt.Printf("Failed to send email notification: %v\n", err)
			}
		}
	}

	// Send push notification
	if n.cfg.GotifyURL != "" {
		pushBody := message.Body
		if len(pushBody) > 100 {
			pushBody = pushBody[:100] + "..."
		}
		if err := n.SendPush(subject, pushBody); err != nil {
			fmt.Printf("Failed to send push notification: %v\n", err)
		}
	}

	return nil
}

// SendEmail sends an email notification
func (n *Notifier) SendEmail(to, subject, body string) error {
	if n.cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP not configured")
	}

	from := n.cfg.SMTPFrom
	if from == "" {
		from = n.cfg.SMTPUser
	}

	// Build message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	var auth smtp.Auth
	if n.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", n.cfg.SMTPUser, n.cfg.SMTPPassword, n.cfg.SMTPHost)
	}

	addr := fmt.Sprintf("%s:%d", n.cfg.SMTPHost, n.cfg.SMTPPort)

	// Retry logic
	var lastErr error
	for attempt := 0; attempt < config.EmailMaxRetries; attempt++ {
		var err error
		if n.cfg.SMTPTLS {
			err = n.sendEmailTLS(addr, auth, from, to, msg)
		} else {
			err = smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
		}

		if err == nil {
			return nil
		}
		lastErr = err

		// Exponential backoff
		time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
	}

	return fmt.Errorf("failed after %d retries: %w", config.EmailMaxRetries, lastErr)
}

func (n *Notifier) sendEmailTLS(addr string, auth smtp.Auth, from, to, msg string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: n.cfg.SMTPHost,
	})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, n.cfg.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL command failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT command failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	return client.Quit()
}

// SendPush sends a push notification via Gotify
func (n *Notifier) SendPush(title, message string) error {
	if n.cfg.GotifyURL == "" {
		return fmt.Errorf("Gotify not configured")
	}

	payload := map[string]interface{}{
		"title":    title,
		"message":  message,
		"priority": 5,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/message?token=%s", n.cfg.GotifyURL, n.cfg.GotifyToken)

	// Retry logic
	var lastErr error
	for attempt := 0; attempt < config.GotifyMaxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := n.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
	}

	return fmt.Errorf("failed after %d retries: %w", config.GotifyMaxRetries, lastErr)
}

// SendWebhook sends a webhook notification
func (n *Notifier) SendWebhook(url string, payload interface{}) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// HTML email template for voicemail notifications
var voicemailHTMLTemplate = template.Must(template.New("voicemail").Parse(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4A90A4; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .field { margin-bottom: 10px; }
        .label { font-weight: bold; color: #666; }
        .transcription { background: white; padding: 15px; border-left: 3px solid #4A90A4; margin: 15px 0; }
        .button { display: inline-block; background: #4A90A4; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>New Voicemail</h1>
        </div>
        <div class="content">
            <div class="field"><span class="label">From:</span> {{.CallerID}}</div>
            <div class="field"><span class="label">To:</span> {{.DIDNumber}}</div>
            <div class="field"><span class="label">Duration:</span> {{.Duration}} seconds</div>
            <div class="field"><span class="label">Time:</span> {{.Time}}</div>
            {{if .Transcription}}
            <div class="transcription">
                <strong>Transcription:</strong><br>
                {{.Transcription}}
            </div>
            {{end}}
            <p>
                <a href="{{.RecordingURL}}" class="button">Listen to Voicemail</a>
            </p>
        </div>
    </div>
</body>
</html>
`))

// SendHTMLEmail sends an HTML email notification
func (n *Notifier) SendHTMLEmail(to, subject, htmlBody string) error {
	if n.cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP not configured")
	}

	from := n.cfg.SMTPFrom
	if from == "" {
		from = n.cfg.SMTPUser
	}

	// Build message with HTML content type
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, htmlBody)

	var auth smtp.Auth
	if n.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", n.cfg.SMTPUser, n.cfg.SMTPPassword, n.cfg.SMTPHost)
	}

	addr := fmt.Sprintf("%s:%d", n.cfg.SMTPHost, n.cfg.SMTPPort)

	if n.cfg.SMTPTLS {
		return n.sendEmailTLS(addr, auth, from, to, msg)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

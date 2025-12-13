package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

// WebhookHandler handles Twilio webhook callbacks
type WebhookHandler struct {
	deps *Dependencies
}

// NewWebhookHandler creates a new WebhookHandler
func NewWebhookHandler(deps *Dependencies) *WebhookHandler {
	return &WebhookHandler{deps: deps}
}

// VoiceIncoming handles incoming voice calls from Twilio
func (h *WebhookHandler) VoiceIncoming(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondTwiML(w, h.errorTwiML("Invalid request"))
		return
	}

	// Validate Twilio signature
	if !h.validateSignature(r) {
		h.respondTwiML(w, h.errorTwiML("Invalid signature"))
		return
	}

	from := r.FormValue("From")
	to := r.FormValue("To")
	callSID := r.FormValue("CallSid")

	// Check blocklist
	isBlocked, _, err := h.deps.DB.Blocklist.IsBlocked(r.Context(), from)
	if err == nil && isBlocked {
		h.respondTwiML(w, h.rejectTwiML("blocked"))
		return
	}

	// Find DID
	did, err := h.deps.DB.DIDs.GetByNumber(r.Context(), to)
	if err != nil {
		h.respondTwiML(w, h.errorTwiML("Number not found"))
		return
	}

	// Get routing rules for this DID
	routes, err := h.deps.DB.Routes.GetEnabledByDID(r.Context(), did.ID)
	if err != nil || len(routes) == 0 {
		// Default: send to voicemail
		h.respondTwiML(w, h.voicemailTwiML(did.ID, from))
		return
	}

	// Evaluate rules in priority order
	for _, route := range routes {
		if h.evaluateCondition(route, from) {
			twiml := h.executeAction(route, did, from, callSID)
			h.respondTwiML(w, twiml)
			return
		}
	}

	// No matching rule, go to voicemail
	h.respondTwiML(w, h.voicemailTwiML(did.ID, from))
}

// VoiceStatus handles voice call status callbacks
func (h *WebhookHandler) VoiceStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	callSID := r.FormValue("CallSid")
	status := r.FormValue("CallStatus")
	duration, _ := strconv.Atoi(r.FormValue("CallDuration"))

	// Update CDR
	cdr, err := h.deps.DB.CDRs.GetByCallSID(r.Context(), callSID)
	if err == nil {
		cdr.Disposition = status
		cdr.Duration = duration
		if status == "completed" || status == "busy" || status == "no-answer" || status == "failed" {
			now := time.Now()
			cdr.EndedAt = &now
		}
		h.deps.DB.CDRs.Update(r.Context(), cdr)
	}

	w.WriteHeader(http.StatusOK)
}

// VoicemailRecording handles voicemail recording completion
func (h *WebhookHandler) VoicemailRecording(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	recordingSID := r.FormValue("RecordingSid")
	recordingURL := r.FormValue("RecordingUrl")
	duration, _ := strconv.Atoi(r.FormValue("RecordingDuration"))
	from := r.FormValue("From")
	didIDStr := r.FormValue("DidId")

	didID, _ := strconv.ParseInt(didIDStr, 10, 64)

	// Create voicemail record
	voicemail := &models.Voicemail{
		UserID:     &didID,
		FromNumber: from,
		Duration:   duration,
		AudioURL:   recordingURL + ".mp3",
		IsRead:     false,
		CreatedAt:  time.Now(),
	}
	_ = recordingSID // Used by Twilio for transcription requests

	h.deps.DB.Voicemails.Create(r.Context(), voicemail)

	// Request transcription if enabled
	if h.deps.Twilio != nil {
		transcriptionEnabled, _ := h.deps.DB.Config.Get(r.Context(), "transcription_enabled")
		if transcriptionEnabled == "true" {
			go h.deps.Twilio.RequestTranscription(recordingSID, voicemail.ID)
		}
	}

	// Send notifications
	go h.sendVoicemailNotification(voicemail)

	w.WriteHeader(http.StatusOK)
}

// VoicemailTranscription handles transcription completion
func (h *WebhookHandler) VoicemailTranscription(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	transcriptionText := r.FormValue("TranscriptionText")
	voicemailIDStr := r.FormValue("VoicemailId")

	voicemailID, _ := strconv.ParseInt(voicemailIDStr, 10, 64)

	// Update voicemail with transcription
	h.deps.DB.Voicemails.UpdateTranscript(r.Context(), voicemailID, transcriptionText)

	w.WriteHeader(http.StatusOK)
}

// Recording is an alias for VoicemailRecording
func (h *WebhookHandler) Recording(w http.ResponseWriter, r *http.Request) {
	h.VoicemailRecording(w, r)
}

// Transcription is an alias for VoicemailTranscription
func (h *WebhookHandler) Transcription(w http.ResponseWriter, r *http.Request) {
	h.VoicemailTranscription(w, r)
}

// SMSIncoming handles incoming SMS messages
func (h *WebhookHandler) SMSIncoming(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondTwiML(w, "")
		return
	}

	from := r.FormValue("From")
	to := r.FormValue("To")
	body := r.FormValue("Body")
	messageSID := r.FormValue("MessageSid")
	numMedia, _ := strconv.Atoi(r.FormValue("NumMedia"))

	// Find DID
	did, err := h.deps.DB.DIDs.GetByNumber(r.Context(), to)
	if err != nil {
		h.respondTwiML(w, "")
		return
	}

	// Collect media URLs
	var mediaURLs []string
	for i := 0; i < numMedia; i++ {
		mediaURL := r.FormValue("MediaUrl" + strconv.Itoa(i))
		if mediaURL != "" {
			mediaURLs = append(mediaURLs, mediaURL)
		}
	}

	mediaURLsJSON, _ := json.Marshal(mediaURLs)

	// Create message record
	didID := did.ID
	message := &models.Message{
		DIDID:      &didID,
		Direction:  "inbound",
		FromNumber: from,
		ToNumber:   to,
		Body:       body,
		MediaURLs:  mediaURLsJSON,
		Status:     "received",
		MessageSID: messageSID,
		CreatedAt:  time.Now(),
	}

	h.deps.DB.Messages.Create(r.Context(), message)

	// Check for auto-reply
	autoReply := h.checkAutoReply(r.Context(), did.ID, body)
	if autoReply != "" {
		h.respondTwiML(w, h.smsTwiML(autoReply))
		return
	}

	// Send notifications
	go h.sendSMSNotification(message)

	h.respondTwiML(w, "")
}

// SMSStatus handles SMS status callbacks
func (h *WebhookHandler) SMSStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	messageSID := r.FormValue("MessageSid")
	status := r.FormValue("MessageStatus")

	// Update message status by finding the message first
	if msg, err := h.deps.DB.Messages.GetByMessageSID(r.Context(), messageSID); err == nil {
		h.deps.DB.Messages.UpdateStatus(r.Context(), msg.ID, status)
	}

	w.WriteHeader(http.StatusOK)
}

// Helper methods

func (h *WebhookHandler) validateSignature(r *http.Request) bool {
	// Get auth token from config
	authToken, err := h.deps.DB.Config.Get(r.Context(), "twilio_auth_token")
	if err != nil || authToken == "" {
		return false
	}

	signature := r.Header.Get("X-Twilio-Signature")
	if signature == "" {
		return false
	}

	// Build validation URL
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	validationURL := scheme + "://" + r.Host + r.URL.Path

	// Sort form values and append to URL
	r.ParseForm()
	keys := make([]string, 0, len(r.PostForm))
	for k := range r.PostForm {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		validationURL += k + r.PostForm.Get(k)
	}

	// Calculate expected signature
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(validationURL))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func (h *WebhookHandler) respondTwiML(w http.ResponseWriter, twiml string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	io.WriteString(w, twiml)
}

func (h *WebhookHandler) errorTwiML(message string) string {
	return `<Response><Say>` + message + `</Say><Hangup/></Response>`
}

func (h *WebhookHandler) rejectTwiML(reason string) string {
	return `<Response><Reject reason="` + reason + `"/></Response>`
}

func (h *WebhookHandler) voicemailTwiML(didID int64, from string) string {
	greeting, _ := h.deps.DB.Config.Get(nil, "voicemail_greeting")
	if greeting == "" {
		greeting = "Please leave a message after the beep."
	}

	// Build action URL with DID ID
	actionURL := "/api/webhooks/voicemail/recording?DidId=" + strconv.FormatInt(didID, 10)

	return `<Response>
		<Say>` + greeting + `</Say>
		<Record maxLength="180" action="` + actionURL + `" transcribe="false" playBeep="true"/>
		<Say>Goodbye.</Say>
	</Response>`
}

func (h *WebhookHandler) smsTwiML(message string) string {
	return `<Response><Message>` + escapeXML(message) + `</Message></Response>`
}

func (h *WebhookHandler) evaluateCondition(route *models.Route, callerID string) bool {
	switch route.ConditionType {
	case "default":
		return true
	case "callerid":
		var data struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(route.ConditionData, &data); err == nil {
			return strings.Contains(callerID, data.Pattern)
		}
	case "time":
		var data struct {
			StartHour int `json:"start_hour"`
			EndHour   int `json:"end_hour"`
			Days      []int `json:"days"`
		}
		if err := json.Unmarshal(route.ConditionData, &data); err == nil {
			now := time.Now()
			hour := now.Hour()
			weekday := int(now.Weekday())

			// Check day
			dayMatch := len(data.Days) == 0
			for _, d := range data.Days {
				if d == weekday {
					dayMatch = true
					break
				}
			}

			// Check time
			timeMatch := hour >= data.StartHour && hour < data.EndHour

			return dayMatch && timeMatch
		}
	}
	return false
}

func (h *WebhookHandler) executeAction(route *models.Route, did *models.DID, from, callSID string) string {
	switch route.ActionType {
	case "ring":
		var data struct {
			Devices []int64 `json:"devices"`
			Timeout int     `json:"timeout"`
		}
		if err := json.Unmarshal(route.ActionData, &data); err == nil {
			// Build dial string to SIP devices
			timeout := data.Timeout
			if timeout == 0 {
				timeout = 30
			}

			var dialTargets []string
			for _, deviceID := range data.Devices {
				device, err := h.deps.DB.Devices.GetByID(nil, deviceID)
				if err == nil {
					dialTargets = append(dialTargets, `<Sip>`+device.Username+`@sip.gosip.local</Sip>`)
				}
			}

			if len(dialTargets) == 0 {
				return h.voicemailTwiML(did.ID, from)
			}

			return `<Response>
				<Dial timeout="` + strconv.Itoa(timeout) + `" action="/api/webhooks/voice/status">
					` + strings.Join(dialTargets, "\n") + `
				</Dial>
				` + h.voicemailTwiML(did.ID, from) + `
			</Response>`
		}

	case "forward":
		var data struct {
			Number string `json:"number"`
		}
		if err := json.Unmarshal(route.ActionData, &data); err == nil {
			return `<Response>
				<Dial callerId="` + did.Number + `">
					<Number>` + data.Number + `</Number>
				</Dial>
			</Response>`
		}

	case "voicemail":
		return h.voicemailTwiML(did.ID, from)

	case "reject":
		return h.rejectTwiML("rejected")
	}

	return h.voicemailTwiML(did.ID, from)
}

func (h *WebhookHandler) checkAutoReply(ctx context.Context, didID int64, body string) string {
	rules, err := h.deps.DB.AutoReplies.ListEnabledByDID(ctx, didID)
	if err != nil {
		return ""
	}

	bodyLower := strings.ToLower(body)

	for _, rule := range rules {
		switch rule.TriggerType {
		case "always":
			return rule.ReplyText
		case "keyword":
			// TriggerData is json.RawMessage, convert to string
			triggerData := string(rule.TriggerData)
			keywords := strings.Split(strings.ToLower(triggerData), ",")
			for _, kw := range keywords {
				if strings.Contains(bodyLower, strings.TrimSpace(kw)) {
					return rule.ReplyText
				}
			}
		case "after_hours":
			// Check if outside business hours
			startHour, _ := h.deps.DB.Config.Get(ctx, "business_hours_start")
			endHour, _ := h.deps.DB.Config.Get(ctx, "business_hours_end")

			start, _ := strconv.Atoi(startHour)
			end, _ := strconv.Atoi(endHour)

			now := time.Now().Hour()
			if now < start || now >= end {
				return rule.ReplyText
			}
		}
	}

	return ""
}

func (h *WebhookHandler) sendVoicemailNotification(voicemail *models.Voicemail) {
	// Email notification
	if h.deps.Notifier != nil {
		h.deps.Notifier.SendVoicemailNotification(voicemail)
	}
}

func (h *WebhookHandler) sendSMSNotification(message *models.Message) {
	// Email notification
	if h.deps.Notifier != nil {
		h.deps.Notifier.SendSMSNotification(message)
	}
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// SIPTrunkHandler handles SIP trunking webhooks for connecting to internal SIP devices
type SIPTrunkHandler struct {
	deps *Dependencies
}

// NewSIPTrunkHandler creates a new SIPTrunkHandler
func NewSIPTrunkHandler(deps *Dependencies) *SIPTrunkHandler {
	return &SIPTrunkHandler{deps: deps}
}

// HandleSIPInvite handles incoming SIP INVITE from Twilio
func (h *SIPTrunkHandler) HandleSIPInvite(w http.ResponseWriter, r *http.Request) {
	// This endpoint receives SIP signaling from Twilio and bridges to internal devices
	// The actual SIP handling is done by the SIP server (pkg/sip)

	from := r.FormValue("From")
	to := r.FormValue("To")

	// Extract extension from SIP URI
	toURI, _ := url.Parse("sip:" + to)
	extension := strings.Split(toURI.User.Username(), "@")[0]

	// Look up device by extension/username
	device, err := h.deps.DB.Devices.GetByUsername(r.Context(), extension)
	if err != nil {
		h.respondTwiML(w, `<Response><Say>Extension not found</Say><Hangup/></Response>`)
		return
	}

	// Check if device is registered
	if h.deps.SIP != nil && !h.deps.SIP.GetRegistrar().IsRegistered(r.Context(), device.ID) {
		h.respondTwiML(w, `<Response><Say>Extension is not available</Say><Hangup/></Response>`)
		return
	}

	// Bridge call to internal SIP device
	h.respondTwiML(w, `<Response>
		<Dial callerId="`+from+`">
			<Sip>`+device.Username+`@`+h.deps.Config.SIPDomain+`</Sip>
		</Dial>
	</Response>`)
}

func (h *SIPTrunkHandler) respondTwiML(w http.ResponseWriter, twiml string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	io.WriteString(w, twiml)
}

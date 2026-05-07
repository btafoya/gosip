package sip

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/models"
	"github.com/emiago/sipgo/sip"
)

// handleRegister processes REGISTER requests
func (s *Server) handleRegister(req *sip.Request, tx sip.ServerTransaction) {
	ctx, cancel := context.WithTimeout(context.Background(), config.SIPRegistrationTimeout)
	defer cancel()

	slog.Debug("Received REGISTER request",
		"from", req.From().Address.String(),
		"contact", req.Contact(),
	)

	// Extract credentials from Authorization header
	authHeader := req.GetHeader("Authorization")
	if authHeader == nil {
		// Send 401 Unauthorized with challenge
		s.sendAuthChallenge(req, tx)
		return
	}

	// Authenticate the request
	device, err := s.auth.Authenticate(ctx, req)
	if err != nil {
		slog.Warn("Authentication failed", "error", err, "from", req.From().Address.String())
		s.sendResponse(tx, req, sip.StatusForbidden, "Forbidden")
		return
	}

	// Get contact and expires
	contact := req.Contact()
	if contact == nil {
		s.sendResponse(tx, req, sip.StatusBadRequest, "Missing Contact header")
		return
	}

	expires := getExpires(req)

	// Handle unregistration (Expires: 0)
	if expires == 0 {
		if err := s.registrar.Unregister(ctx, device.ID); err != nil {
			slog.Error("Failed to unregister device", "error", err, "device_id", device.ID)
			s.sendResponse(tx, req, sip.StatusInternalServerError, "Internal Server Error")
			return
		}
		slog.Info("Device unregistered", "device", device.Username)
		s.sendResponse(tx, req, sip.StatusOK, "OK")
		return
	}

	// Create or update registration
	reg := &models.Registration{
		DeviceID:  device.ID,
		Contact:   contact.Address.String(),
		ExpiresAt: time.Now().Add(time.Duration(expires) * time.Second),
		UserAgent: getUserAgent(req),
		IPAddress: getSourceIP(req),
		Transport: getTransport(req),
	}

	if err := s.registrar.Register(ctx, reg); err != nil {
		slog.Error("Failed to register device", "error", err, "device_id", device.ID)
		s.sendResponse(tx, req, sip.StatusInternalServerError, "Internal Server Error")
		return
	}

	slog.Info("Device registered",
		"device", device.Username,
		"contact", contact.Address.String(),
		"expires", expires,
	)

	// Send 200 OK
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	res.AppendHeader(sip.NewHeader("Contact", contact.Value()))
	res.AppendHeader(sip.NewHeader("Expires", string(rune(expires))))

	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to send REGISTER response", "error", err)
	}
}

// handleInvite processes INVITE requests for incoming calls
func (s *Server) handleInvite(req *sip.Request, tx sip.ServerTransaction) {
	ctx, cancel := context.WithTimeout(context.Background(), config.CallSetupTimeout)
	defer cancel()

	callID := req.CallID().Value()

	slog.Debug("Received INVITE request",
		"call_id", callID,
		"from", req.From().Address.String(),
		"to", req.To().Address.String(),
	)

	// Check if this is a re-INVITE for an existing session (hold/resume)
	existingSession := s.sessions.Get(callID)
	if existingSession != nil {
		// This is a re-INVITE - handle via hold manager
		if err := s.holdMgr.HandleReInvite(req, tx); err != nil {
			slog.Error("Re-INVITE handling failed", "error", err, "call_id", callID)
		}
		return
	}

	// Send 100 Trying immediately for new call
	s.sendResponse(tx, req, sip.StatusTrying, "Trying")

	// Extract call information
	fromURI := req.From().Address
	toURI := req.To().Address

	// Check if this is an authenticated internal call or an external incoming call
	authHeader := req.GetHeader("Authorization")
	if authHeader != nil {
		// Internal call - authenticate device
		device, err := s.auth.Authenticate(ctx, req)
		if err != nil {
			slog.Warn("INVITE authentication failed", "error", err)
			s.sendResponse(tx, req, sip.StatusForbidden, "Forbidden")
			return
		}

		// Create session for outbound call
		session := NewCallSession(req, CallDirectionOutbound)
		session.DeviceID = device.ID
		s.sessions.Add(session)
		s.incrementCallCount()

		slog.Debug("Authenticated outbound call",
			"device", device.Username,
			"call_id", callID,
		)
		// TODO: Route outbound call through Twilio
		s.sendResponse(tx, req, sip.StatusNotImplemented, "Outbound calls not yet implemented")
		return
	}

	// External incoming call - should be from Twilio
	// Create session for inbound call
	session := NewCallSession(req, CallDirectionInbound)
	s.sessions.Add(session)
	s.incrementCallCount()

	// Source IP is logged for forensics. We deliberately do NOT classify "is this
	// from Twilio?" via From-host string match or coarse IP prefix — both are
	// trivially spoofed and would mislead operators. Per-trunk min_attestation
	// (planned) or TLS cert pinning are the right enforcement mechanisms.
	sourceIP := getSourceIP(req)
	slog.Info("Incoming call",
		"call_id", callID,
		"from", fromURI.String(),
		"to", toURI.String(),
		"source_ip", sourceIP,
		"source_in_twilio_cidr", isTwilioSignalingIP(sourceIP),
	)

	// Extract called number and look up DID
	calledNumber := extractNumber(toURI.String())
	if calledNumber == "" {
		slog.Warn("Cannot extract called number from To URI", "to", toURI.String())
		s.sendResponse(tx, req, sip.StatusNotFound, "Not Found")
		s.decrementCallCount()
		return
	}

	did, err := s.db.DIDs.GetByNumber(ctx, calledNumber)
	if err != nil {
		slog.Warn("DID lookup failed", "number", calledNumber, "error", err)
		// Default to voicemail if DID not found
		s.sendResponse(tx, req, sip.StatusMovedTemporarily, "Moved Temporarily")
		return
	}

	session.DIDID = &did.ID

	// Get enabled routes for this DID
	routes, err := s.db.Routes.GetEnabledByDID(ctx, did.ID)
	if err != nil {
		slog.Error("Failed to get routes for DID", "did_id", did.ID, "error", err)
		s.sendResponse(tx, req, sip.StatusMovedTemporarily, "Moved Temporarily")
		return
	}

	// Evaluate routes and execute first matching action
	matched := s.evaluateAndExecuteRoute(ctx, req, tx, session, routes)
	if !matched {
		// No route matched - default to voicemail
		slog.Info("No route matched, sending to voicemail", "did", did.Number)
		s.sendResponse(tx, req, sip.StatusMovedTemporarily, "Moved Temporarily")
	}
}

// evaluateAndExecuteRoute evaluates routes in priority order and executes the first match.
// It returns true if a route was matched and handled.
func (s *Server) evaluateAndExecuteRoute(ctx context.Context, req *sip.Request, tx sip.ServerTransaction, session *CallSession, routes []*models.Route) bool {
	for _, route := range routes {
		if !s.routeConditionMatches(route) {
			continue
		}

		switch route.ActionType {
		case "ring":
			s.handleRingAction(ctx, req, tx, session, route)
			return true
		case "voicemail":
			s.sendResponse(tx, req, sip.StatusMovedTemporarily, "Moved Temporarily")
			return true
		case "reject":
			var status sip.StatusCode = sip.StatusBusyHere
			if len(route.ActionData) > 0 {
				var data map[string]interface{}
				if err := json.Unmarshal(route.ActionData, &data); err == nil {
					if reason, ok := data["reason"].(string); ok && reason == "decline" {
						status = sip.StatusCode(603)
					}
				}
			}
			s.sendResponse(tx, req, status, "")
			return true
		default:
			slog.Warn("Unknown route action type", "action", route.ActionType, "route_id", route.ID)
		}
	}
	return false
}

// routeConditionMatches checks whether a route's condition is currently satisfied.
func (s *Server) routeConditionMatches(route *models.Route) bool {
	switch route.ConditionType {
	case "default":
		return true
	case "time":
		var tc models.TimeCondition
		if err := json.Unmarshal(route.ConditionData, &tc); err != nil {
			slog.Warn("Failed to parse time condition", "route_id", route.ID, "error", err)
			return false
		}
		return timeConditionMatches(&tc)
	default:
		// Unknown condition types are treated as non-matching for safety
		return false
	}
}

// timeConditionMatches returns true if the current time satisfies the condition.
func timeConditionMatches(tc *models.TimeCondition) bool {
	loc := time.Local
	if tc.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(tc.Timezone)
		if err != nil {
			slog.Warn("Failed to load timezone", "timezone", tc.Timezone, "error", err)
			loc = time.Local
		}
	}

	now := time.Now().In(loc)

	// Check day of week
	if len(tc.Days) > 0 {
		found := false
		for _, d := range tc.Days {
			if int(now.Weekday()) == d {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Parse and check hour ranges
	if tc.StartTime != "" && tc.EndTime != "" {
		start, err1 := time.ParseInLocation("15:04", tc.StartTime, loc)
		end, err2 := time.ParseInLocation("15:04", tc.EndTime, loc)
		if err1 != nil || err2 != nil {
			slog.Warn("Failed to parse time range", "start", tc.StartTime, "end", tc.EndTime)
			return false
		}

		currentMinutes := now.Hour()*60 + now.Minute()
		startMinutes := start.Hour()*60 + start.Minute()
		endMinutes := end.Hour()*60 + end.Minute()

		if endMinutes < startMinutes {
			// Wraps past midnight
			if currentMinutes < startMinutes && currentMinutes > endMinutes {
				return false
			}
		} else {
			if currentMinutes < startMinutes || currentMinutes > endMinutes {
				return false
			}
		}
	}

	return true
}

// handleRingAction processes a "ring" route action by looking up devices and
// returning a 302 Redirect to their registered contacts.
func (s *Server) handleRingAction(ctx context.Context, req *sip.Request, tx sip.ServerTransaction, session *CallSession, route *models.Route) {
	var action models.RingAction
	if err := json.Unmarshal(route.ActionData, &action); err != nil {
		slog.Error("Failed to parse ring action data", "route_id", route.ID, "error", err)
		s.sendResponse(tx, req, sip.StatusMovedTemporarily, "Moved Temporarily")
		return
	}

	if len(action.DeviceIDs) == 0 {
		slog.Warn("Ring action has no devices", "route_id", route.ID)
		s.sendResponse(tx, req, sip.StatusMovedTemporarily, "Moved Temporarily")
		return
	}

	// Build redirect contacts from device registrations
	var contacts []string
	for _, deviceID := range action.DeviceIDs {
		device, err := s.db.Devices.GetByID(ctx, deviceID)
		if err != nil {
			slog.Warn("Failed to look up device for ring action", "device_id", deviceID, "error", err)
			continue
		}

		// Find active registration for device
		regs, err := s.registrar.GetActiveRegistrations(ctx)
		if err != nil {
			slog.Warn("Failed to get registrations", "error", err)
			continue
		}

		for _, reg := range regs {
			if reg.DeviceID == device.ID {
				contacts = append(contacts, reg.Contact)
				break
			}
		}
	}

	if len(contacts) == 0 {
		slog.Info("No registered devices for ring action, falling back to voicemail", "route_id", route.ID)
		s.sendResponse(tx, req, sip.StatusMovedTemporarily, "Moved Temporarily")
		return
	}

	// Send 302 Redirect with device contacts
	res := sip.NewResponseFromRequest(req, sip.StatusMovedTemporarily, "Moved Temporarily", nil)
	for _, contact := range contacts {
		res.AppendHeader(sip.NewHeader("Contact", "<"+contact+">"))
	}
	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to send 302 redirect", "error", err)
	}
}

// handleAck processes ACK requests
func (s *Server) handleAck(req *sip.Request, tx sip.ServerTransaction) {
	slog.Debug("Received ACK request", "call_id", req.CallID().Value())
	// ACK doesn't require a response
}

// handleBye processes BYE requests to end calls
func (s *Server) handleBye(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().Value()
	slog.Debug("Received BYE request", "call_id", callID)

	// Find and terminate the session
	session := s.sessions.Get(callID)
	if session != nil {
		if !validateDialog(req, session) {
			slog.Warn("BYE dialog validation failed", "call_id", callID)
			s.sendResponse(tx, req, sip.StatusForbidden, "Forbidden")
			return
		}
		// Stop MOH if active
		if s.mohMgr != nil && s.mohMgr.IsActive(callID) {
			s.mohMgr.Stop(callID)
		}

		// Clean up SRTP context if active
		if s.srtpMgr != nil {
			if err := s.srtpMgr.Remove(callID); err != nil {
				slog.Warn("Failed to cleanup SRTP context", "error", err, "call_id", callID)
			}
		}

		// Clean up ZRTP session if active
		if s.zrtpMgr != nil {
			if err := s.zrtpMgr.EndSession(callID); err != nil {
				slog.Warn("Failed to cleanup ZRTP session", "error", err, "call_id", callID)
			}
		}

		// Update session state
		if err := session.SetState(CallStateTerminated); err != nil {
			slog.Warn("Failed to set terminated state", "error", err, "call_id", callID)
		}

		s.decrementCallCount()

		slog.Info("Call terminated",
			"call_id", callID,
			"duration", session.Duration(),
		)

		// TODO: Update CDR record
	}

	s.sendResponse(tx, req, sip.StatusOK, "OK")
}

// handleCancel processes CANCEL requests
func (s *Server) handleCancel(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().Value()
	slog.Debug("Received CANCEL request", "call_id", callID)

	// Find and terminate the session if in ringing state
	session := s.sessions.Get(callID)
	if session != nil {
		if !validateDialog(req, session) {
			slog.Warn("CANCEL dialog validation failed", "call_id", callID)
			s.sendResponse(tx, req, sip.StatusForbidden, "Forbidden")
			return
		}
		if session.GetState() == CallStateRinging {
			if err := session.SetState(CallStateTerminated); err != nil {
				slog.Warn("Failed to set terminated state", "error", err, "call_id", callID)
			}
			s.decrementCallCount()
			slog.Info("Call cancelled", "call_id", callID)
		}
	}

	s.sendResponse(tx, req, sip.StatusOK, "OK")
}

// handleRefer processes REFER requests for call transfers
func (s *Server) handleRefer(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().Value()
	slog.Debug("Received REFER request", "call_id", callID)

	// Validate dialog before allowing transfer
	session := s.sessions.Get(callID)
	if session != nil && !validateDialog(req, session) {
		slog.Warn("REFER dialog validation failed", "call_id", callID)
		s.sendResponse(tx, req, sip.StatusForbidden, "Forbidden")
		return
	}

	// Delegate to transfer manager
	if err := s.transferMgr.HandleRefer(req, tx); err != nil {
		slog.Error("REFER handling failed", "error", err, "call_id", callID)
	}
}

// handleOptions processes OPTIONS requests (health check / capabilities)
func (s *Server) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	slog.Debug("Received OPTIONS request", "from", req.From().Address.String())

	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	res.AppendHeader(sip.NewHeader("Allow", "INVITE, ACK, CANCEL, OPTIONS, BYE, REGISTER, REFER, NOTIFY"))
	res.AppendHeader(sip.NewHeader("Accept", "application/sdp"))
	res.AppendHeader(sip.NewHeader("Accept-Language", "en"))
	res.AppendHeader(sip.NewHeader("Supported", "replaces, timer"))

	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to send OPTIONS response", "error", err)
	}
}

// sendResponse sends a simple response
func (s *Server) sendResponse(tx sip.ServerTransaction, req *sip.Request, statusCode sip.StatusCode, reason string) {
	res := sip.NewResponseFromRequest(req, statusCode, reason, nil)
	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to send response", "error", err, "status", statusCode)
	}
}

// sendAuthChallenge sends a 401 Unauthorized with WWW-Authenticate header
func (s *Server) sendAuthChallenge(req *sip.Request, tx sip.ServerTransaction) {
	res := sip.NewResponseFromRequest(req, sip.StatusUnauthorized, "Unauthorized", nil)

	nonce := s.auth.GenerateNonce()
	realm := "gosip"
	authValue := `Digest realm="` + realm + `", nonce="` + nonce + `", algorithm=MD5`
	res.AppendHeader(sip.NewHeader("WWW-Authenticate", authValue))

	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to send auth challenge", "error", err)
	}
}

// Helper functions to extract info from SIP requests

func getExpires(req *sip.Request) int {
	// First check Expires header
	if h := req.GetHeader("Expires"); h != nil {
		var expires int
		if _, err := fmt.Sscanf(h.Value(), "%d", &expires); err == nil {
			return expires
		}
	}
	// Check Contact expires parameter
	if contact := req.Contact(); contact != nil {
		// TODO: Parse expires param from Contact
	}
	// Default expires
	return config.RegistrationExpires
}

func getUserAgent(req *sip.Request) string {
	if h := req.GetHeader("User-Agent"); h != nil {
		return h.Value()
	}
	return ""
}

func getSourceIP(req *sip.Request) string {
	// Get source IP from Via header or connection info
	if via := req.Via(); via != nil {
		return via.Host
	}
	return ""
}

func getTransport(req *sip.Request) string {
	if via := req.Via(); via != nil {
		return via.Transport
	}
	return "udp"
}

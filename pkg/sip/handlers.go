package sip

import (
	"context"
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

	// TODO: Validate request is from Twilio and route to appropriate device
	slog.Info("Incoming call",
		"call_id", callID,
		"from", fromURI.String(),
		"to", toURI.String(),
	)

	// For now, send 486 Busy Here until call routing is implemented
	s.sendResponse(tx, req, sip.StatusBusyHere, "Busy Here")
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

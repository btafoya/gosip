package sip

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emiago/sipgo/sip"
)

// handleSubscribe handles SIP SUBSCRIBE requests
func (s *Server) handleSubscribe(req *sip.Request, tx sip.ServerTransaction) {
	ctx := context.Background()

	// Get the Event header
	eventHeader := req.GetHeader("Event")
	if eventHeader == nil {
		s.respondToSubscribe(tx, req, sip.StatusCode(489), "Bad Event") // Bad Event
		return
	}

	event := strings.ToLower(eventHeader.Value())

	switch {
	case strings.HasPrefix(event, "message-summary"):
		s.handleMWISubscribe(ctx, req, tx)
	default:
		slog.Debug("Unsupported SUBSCRIBE event",
			slog.String("event", event),
		)
		s.respondToSubscribe(tx, req, sip.StatusCode(489), "Bad Event")
	}
}

// handleMWISubscribe handles MWI SUBSCRIBE requests
func (s *Server) handleMWISubscribe(ctx context.Context, req *sip.Request, tx sip.ServerTransaction) {
	// Extract subscription info from request
	fromHeader := req.From()
	if fromHeader == nil {
		s.respondToSubscribe(tx, req, sip.StatusCode(400), "Missing From header")
		return
	}

	toHeader := req.To()
	if toHeader == nil {
		s.respondToSubscribe(tx, req, sip.StatusCode(400), "Missing To header")
		return
	}

	// Get AOR from To header (the mailbox being subscribed to)
	aor := toHeader.Address.String()

	// Get Contact header for sending NOTIFY
	contactHeader := req.GetHeader("Contact")
	contactURI := ""
	if contactHeader != nil {
		contactURI = contactHeader.Value()
		// Clean up contact URI (remove < and >)
		contactURI = strings.Trim(strings.TrimSpace(contactURI), "<>")
	}
	if contactURI == "" {
		// Fall back to Via header
		via := req.Via()
		if via != nil {
			contactURI = fmt.Sprintf("sip:%s:%d", via.Host, via.Port)
		}
	}

	// Get Expires header (default to 3600 seconds per RFC)
	expires := 3600
	if expiresHeader := req.GetHeader("Expires"); expiresHeader != nil {
		if _, err := fmt.Sscanf(expiresHeader.Value(), "%d", &expires); err != nil {
			expires = 3600
		}
	}

	// Handle unsubscribe (Expires: 0)
	if expires == 0 {
		s.handleMWIUnsubscribe(ctx, req, tx)
		return
	}

	// Create subscription ID from Call-ID + From tag
	fromTag := ""
	if fromHeader.Params != nil {
		fromTag, _ = fromHeader.Params.Get("tag")
	}
	subID := fmt.Sprintf("%s-%s", req.CallID().Value(), fromTag)

	// Create or refresh subscription
	sub := &MWISubscription{
		ID:         subID,
		AOR:        aor,
		ContactURI: contactURI,
		FromURI:    fromHeader.Address.String(),
		ToURI:      toHeader.Address.String(),
		CallID:     req.CallID().Value(),
		FromTag:    fromTag,
		Expires:    expires,
	}

	// Check if this is a refresh of existing subscription
	existing := s.mwiMgr.GetSubscription(subID)
	if existing != nil {
		if err := s.mwiMgr.RefreshSubscription(subID, expires); err != nil {
			slog.Error("Failed to refresh MWI subscription", "error", err)
			s.respondToSubscribe(tx, req, sip.StatusCode(500), "Internal Server Error")
			return
		}
	} else {
		s.mwiMgr.AddSubscription(sub)
	}

	// Generate To tag for response
	toTag := fmt.Sprintf("mwi-%d", time.Now().UnixNano())

	// Send 200 OK response
	resp := sip.NewResponseFromRequest(req, 200, "OK", nil)

	// Add Contact header
	resp.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<%s>", s.getLocalContact(req))))

	// Add Expires header
	resp.AppendHeader(sip.NewHeader("Expires", fmt.Sprintf("%d", expires)))

	// Add To tag
	if toHeader != nil && resp.To() != nil {
		if resp.To().Params == nil {
			resp.To().Params = sip.NewParams()
		}
		resp.To().Params.Add("tag", toTag)
	}

	if err := tx.Respond(resp); err != nil {
		slog.Error("Failed to send SUBSCRIBE 200 OK", "error", err)
		return
	}

	slog.Info("MWI subscription accepted",
		slog.String("id", subID),
		slog.String("aor", aor),
		slog.String("contact", contactURI),
		slog.Int("expires", expires),
	)

	// Send initial NOTIFY with current state
	if sub.ToTag == "" {
		sub.ToTag = toTag
	}
	if err := s.mwiMgr.NotifyAllSubscribers(ctx, aor); err != nil {
		slog.Error("Failed to send initial MWI NOTIFY", "error", err)
	}
}

// handleMWIUnsubscribe handles MWI unsubscribe (Expires: 0)
func (s *Server) handleMWIUnsubscribe(ctx context.Context, req *sip.Request, tx sip.ServerTransaction) {
	fromHeader := req.From()
	fromTag := ""
	if fromHeader != nil && fromHeader.Params != nil {
		fromTag, _ = fromHeader.Params.Get("tag")
	}

	subID := fmt.Sprintf("%s-%s", req.CallID().Value(), fromTag)

	s.mwiMgr.RemoveSubscription(subID)

	// Send 200 OK
	resp := sip.NewResponseFromRequest(req, 200, "OK", nil)
	resp.AppendHeader(sip.NewHeader("Expires", "0"))

	if err := tx.Respond(resp); err != nil {
		slog.Error("Failed to send SUBSCRIBE 200 OK (unsubscribe)", "error", err)
		return
	}

	slog.Info("MWI subscription removed",
		slog.String("id", subID),
	)
}

// respondToSubscribe sends a response to a SUBSCRIBE request
func (s *Server) respondToSubscribe(tx sip.ServerTransaction, req *sip.Request, statusCode sip.StatusCode, reason string) {
	resp := sip.NewResponseFromRequest(req, statusCode, reason, nil)
	if err := tx.Respond(resp); err != nil {
		slog.Error("Failed to send SUBSCRIBE response",
			slog.Int("status", int(statusCode)),
			slog.String("error", err.Error()),
		)
	}
}

// getLocalContact returns the local contact URI for this server
func (s *Server) getLocalContact(req *sip.Request) string {
	// Try to get from Via header
	via := req.Via()
	if via != nil {
		return fmt.Sprintf("sip:%s:%d", via.Host, s.cfg.Port)
	}
	return fmt.Sprintf("sip:gosip@127.0.0.1:%d", s.cfg.Port)
}

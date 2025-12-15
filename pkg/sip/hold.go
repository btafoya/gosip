// Package sip provides hold/resume functionality for GoSIP
package sip

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/emiago/sipgo/sip"
)

// HoldType indicates the type of hold operation
type HoldType string

const (
	HoldTypeSendOnly HoldType = "sendonly" // We put them on hold (they hear MOH)
	HoldTypeRecvOnly HoldType = "recvonly" // They put us on hold
	HoldTypeInactive HoldType = "inactive" // Both directions held
)

// HoldManager handles SIP hold/resume operations
type HoldManager struct {
	server     *Server
	sessions   *SessionManager
	mohManager *MOHManager
}

// NewHoldManager creates a new hold manager
func NewHoldManager(server *Server, sessions *SessionManager, mohManager *MOHManager) *HoldManager {
	return &HoldManager{
		server:     server,
		sessions:   sessions,
		mohManager: mohManager,
	}
}

// HandleReInvite processes re-INVITE requests for hold/resume
func (h *HoldManager) HandleReInvite(req *sip.Request, tx sip.ServerTransaction) error {
	callID := req.CallID().Value()
	session := h.sessions.Get(callID)

	if session == nil {
		slog.Warn("Re-INVITE for unknown session", "call_id", callID)
		h.sendResponse(tx, req, 481, "Call/Transaction Does Not Exist") // 481 Call/Transaction Does Not Exist
		return fmt.Errorf("session not found: %s", callID)
	}

	// Parse SDP to determine hold state
	sdpBody := req.Body()
	if sdpBody == nil {
		slog.Warn("Re-INVITE without SDP", "call_id", callID)
		h.sendResponse(tx, req, sip.StatusBadRequest, "SDP Required")
		return fmt.Errorf("no SDP in re-INVITE")
	}

	holdType := ParseHoldFromSDP(sdpBody)

	slog.Debug("Re-INVITE received",
		"call_id", callID,
		"hold_type", holdType,
		"current_state", session.GetState(),
	)

	switch holdType {
	case HoldTypeSendOnly:
		// Remote party is putting us on hold
		return h.handleRemoteHold(session, req, tx)
	case HoldTypeRecvOnly:
		// This is unusual - they want to receive only
		return h.handleRemoteHold(session, req, tx)
	case HoldTypeInactive:
		// Both directions inactive
		return h.handleRemoteHold(session, req, tx)
	default:
		// No hold indication - this is a resume or media update
		if session.GetState() == CallStateHeld {
			return h.handleRemoteResume(session, req, tx)
		}
		// Normal re-INVITE for codec change or similar
		return h.handleMediaUpdate(session, req, tx)
	}
}

// handleRemoteHold processes when remote party puts us on hold
func (h *HoldManager) handleRemoteHold(session *CallSession, req *sip.Request, tx sip.ServerTransaction) error {
	if err := session.SetState(CallStateHeld); err != nil {
		slog.Error("Failed to set held state", "error", err, "call_id", session.CallID)
		h.sendResponse(tx, req, sip.StatusInternalServerError, "Internal Server Error")
		return err
	}

	// Store the held SDP for resume
	session.mu.Lock()
	session.HeldSDP = req.Body()
	session.mu.Unlock()

	// Generate response SDP accepting hold
	responseSDP := h.generateHoldResponseSDP(session, req.Body())

	// Send 200 OK with SDP
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", responseSDP)
	res.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))

	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to respond to hold", "error", err)
		return err
	}

	slog.Info("Call placed on hold by remote",
		"call_id", session.CallID,
		"from", session.FromNumber,
	)

	return nil
}

// handleRemoteResume processes when remote party takes us off hold
func (h *HoldManager) handleRemoteResume(session *CallSession, req *sip.Request, tx sip.ServerTransaction) error {
	if err := session.SetState(CallStateActive); err != nil {
		slog.Error("Failed to set active state", "error", err, "call_id", session.CallID)
		h.sendResponse(tx, req, sip.StatusInternalServerError, "Internal Server Error")
		return err
	}

	// Stop MOH if playing
	if h.mohManager != nil {
		h.mohManager.Stop(session.CallID)
	}

	// Generate normal response SDP
	responseSDP := h.generateActiveResponseSDP(session, req.Body())

	// Send 200 OK with SDP
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", responseSDP)
	res.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))

	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to respond to resume", "error", err)
		return err
	}

	// Update remote SDP
	session.mu.Lock()
	session.RemoteSDP = req.Body()
	session.HeldSDP = nil
	session.mu.Unlock()

	slog.Info("Call resumed by remote",
		"call_id", session.CallID,
		"from", session.FromNumber,
	)

	return nil
}

// handleMediaUpdate processes non-hold re-INVITEs
func (h *HoldManager) handleMediaUpdate(session *CallSession, req *sip.Request, tx sip.ServerTransaction) error {
	// Update remote SDP
	session.mu.Lock()
	session.RemoteSDP = req.Body()
	session.mu.Unlock()

	// Generate response SDP
	responseSDP := h.generateActiveResponseSDP(session, req.Body())

	// Send 200 OK
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", responseSDP)
	res.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))

	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to respond to media update", "error", err)
		return err
	}

	slog.Debug("Media update processed", "call_id", session.CallID)
	return nil
}

// PutOnHold initiates hold from our side (GoSIP putting remote on hold)
func (h *HoldManager) PutOnHold(ctx context.Context, session *CallSession) error {
	if session.GetState() != CallStateActive {
		return fmt.Errorf("can only hold active calls, current state: %s", session.GetState())
	}

	// Generate hold SDP (sendonly - we stop sending, they receive silence/MOH)
	holdSDP := h.generateHoldSDP(session)

	// Create re-INVITE request
	req := h.createReInviteRequest(session, holdSDP)

	// Send re-INVITE
	tx, err := h.server.client.TransactionRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send hold re-INVITE: %w", err)
	}
	defer tx.Terminate()

	// Wait for response
	select {
	case res := <-tx.Responses():
		if res.IsSuccess() {
			if err := session.SetState(CallStateHolding); err != nil {
				return err
			}
			// Start MOH for the held party
			if h.mohManager != nil {
				h.mohManager.Start(session.CallID, session)
			}
			slog.Info("Call put on hold", "call_id", session.CallID)
			return nil
		}
		return fmt.Errorf("hold rejected: %d %s", res.StatusCode, res.Reason)
	case <-tx.Done():
		return fmt.Errorf("hold transaction failed: %w", tx.Err())
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Resume takes a call off hold
func (h *HoldManager) Resume(ctx context.Context, session *CallSession) error {
	state := session.GetState()
	if state != CallStateHeld && state != CallStateHolding {
		return fmt.Errorf("can only resume held calls, current state: %s", state)
	}

	// Stop MOH first
	if h.mohManager != nil {
		h.mohManager.Stop(session.CallID)
	}

	// Generate active SDP (sendrecv)
	activeSDP := h.generateResumeSDP(session)

	// Create re-INVITE request
	req := h.createReInviteRequest(session, activeSDP)

	// Send re-INVITE
	tx, err := h.server.client.TransactionRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send resume re-INVITE: %w", err)
	}
	defer tx.Terminate()

	// Wait for response
	select {
	case res := <-tx.Responses():
		if res.IsSuccess() {
			if err := session.SetState(CallStateActive); err != nil {
				return err
			}
			slog.Info("Call resumed", "call_id", session.CallID)
			return nil
		}
		return fmt.Errorf("resume rejected: %d %s", res.StatusCode, res.Reason)
	case <-tx.Done():
		return fmt.Errorf("resume transaction failed: %w", tx.Err())
	case <-ctx.Done():
		return ctx.Err()
	}
}

// createReInviteRequest creates a re-INVITE request within a dialog
func (h *HoldManager) createReInviteRequest(session *CallSession, sdp []byte) *sip.Request {
	session.mu.RLock()
	defer session.mu.RUnlock()

	req := sip.NewRequest(sip.INVITE, sip.Uri{})
	req.SetBody(sdp)
	req.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	req.AppendHeader(sip.NewHeader("Call-ID", session.CallID))

	// Set From/To based on direction using simple header strings
	if session.Direction == CallDirectionOutbound {
		fromValue := fmt.Sprintf("<sip:%s@gosip>;tag=%s", extractNumber(session.LocalURI), session.FromTag)
		toValue := fmt.Sprintf("<sip:%s@gosip>;tag=%s", extractNumber(session.RemoteURI), session.ToTag)
		req.AppendHeader(sip.NewHeader("From", fromValue))
		req.AppendHeader(sip.NewHeader("To", toValue))
	} else {
		fromValue := fmt.Sprintf("<sip:%s@gosip>;tag=%s", extractNumber(session.LocalURI), session.ToTag)
		toValue := fmt.Sprintf("<sip:%s@gosip>;tag=%s", extractNumber(session.RemoteURI), session.FromTag)
		req.AppendHeader(sip.NewHeader("From", fromValue))
		req.AppendHeader(sip.NewHeader("To", toValue))
	}

	return req
}

// generateHoldSDP creates SDP with sendonly for putting remote on hold
func (h *HoldManager) generateHoldSDP(session *CallSession) []byte {
	session.mu.RLock()
	localSDP := session.LocalSDP
	session.mu.RUnlock()

	if localSDP == nil {
		// Generate basic hold SDP
		return []byte(`v=0
o=gosip 0 0 IN IP4 0.0.0.0
s=GoSIP Call
c=IN IP4 0.0.0.0
t=0 0
m=audio 0 RTP/AVP 0 8 101
a=sendonly
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:101 telephone-event/8000
`)
	}

	// Modify existing SDP to be sendonly
	return ModifySDPDirection(localSDP, "sendonly")
}

// generateResumeSDP creates SDP with sendrecv for resuming
func (h *HoldManager) generateResumeSDP(session *CallSession) []byte {
	session.mu.RLock()
	localSDP := session.LocalSDP
	heldSDP := session.HeldSDP
	session.mu.RUnlock()

	baseSDP := localSDP
	if baseSDP == nil {
		baseSDP = heldSDP
	}

	if baseSDP == nil {
		// Generate basic active SDP
		return []byte(`v=0
o=gosip 0 0 IN IP4 0.0.0.0
s=GoSIP Call
c=IN IP4 0.0.0.0
t=0 0
m=audio 0 RTP/AVP 0 8 101
a=sendrecv
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:101 telephone-event/8000
`)
	}

	return ModifySDPDirection(baseSDP, "sendrecv")
}

// generateHoldResponseSDP creates response SDP accepting hold
func (h *HoldManager) generateHoldResponseSDP(session *CallSession, offerSDP []byte) []byte {
	// Mirror the offer with recvonly (we receive their silence)
	return ModifySDPDirection(offerSDP, "recvonly")
}

// generateActiveResponseSDP creates response SDP for active call
func (h *HoldManager) generateActiveResponseSDP(session *CallSession, offerSDP []byte) []byte {
	return ModifySDPDirection(offerSDP, "sendrecv")
}

// sendResponse sends a SIP response
func (h *HoldManager) sendResponse(tx sip.ServerTransaction, req *sip.Request, statusCode sip.StatusCode, reason string) {
	res := sip.NewResponseFromRequest(req, statusCode, reason, nil)
	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to send response", "error", err, "status", statusCode)
	}
}

// ParseHoldFromSDP determines hold type from SDP content
func ParseHoldFromSDP(sdp []byte) HoldType {
	sdpStr := string(sdp)

	// Check for direction attributes
	if strings.Contains(sdpStr, "a=sendonly") {
		return HoldTypeSendOnly
	}
	if strings.Contains(sdpStr, "a=recvonly") {
		return HoldTypeRecvOnly
	}
	if strings.Contains(sdpStr, "a=inactive") {
		return HoldTypeInactive
	}

	// Check for 0.0.0.0 connection address (RFC 2543 style hold)
	if strings.Contains(sdpStr, "c=IN IP4 0.0.0.0") {
		return HoldTypeInactive
	}

	return "" // No hold indication
}

// ModifySDPDirection modifies the direction attribute in SDP
func ModifySDPDirection(sdp []byte, newDirection string) []byte {
	sdpStr := string(sdp)

	// Remove existing direction attributes
	directionRegex := regexp.MustCompile(`a=(sendrecv|sendonly|recvonly|inactive)\r?\n`)
	sdpStr = directionRegex.ReplaceAllString(sdpStr, "")

	// Find the media line and add direction after it
	mediaRegex := regexp.MustCompile(`(m=audio[^\r\n]*\r?\n)`)
	if mediaRegex.MatchString(sdpStr) {
		sdpStr = mediaRegex.ReplaceAllString(sdpStr, "${1}a="+newDirection+"\r\n")
	} else {
		// No media line, append at end
		sdpStr = strings.TrimRight(sdpStr, "\r\n") + "\r\na=" + newDirection + "\r\n"
	}

	return []byte(sdpStr)
}

// IsHoldSDP checks if SDP indicates a hold request
func IsHoldSDP(sdp []byte) bool {
	return ParseHoldFromSDP(sdp) != ""
}

// NormalizeSDP ensures SDP has proper line endings
func NormalizeSDP(sdp []byte) []byte {
	// Replace \n with \r\n if not already
	result := bytes.ReplaceAll(sdp, []byte("\r\n"), []byte("\n"))
	result = bytes.ReplaceAll(result, []byte("\n"), []byte("\r\n"))
	return result
}

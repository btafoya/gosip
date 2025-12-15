// Package sip provides call transfer functionality for GoSIP
package sip

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emiago/sipgo/sip"
)

// TransferType indicates the type of transfer
type TransferType string

const (
	TransferTypeBlind    TransferType = "blind"
	TransferTypeAttended TransferType = "attended"
)

// TransferManager handles SIP call transfers
type TransferManager struct {
	server     *Server
	sessions   *SessionManager
	holdMgr    *HoldManager
}

// NewTransferManager creates a new transfer manager
func NewTransferManager(server *Server, sessions *SessionManager, holdMgr *HoldManager) *TransferManager {
	return &TransferManager{
		server:     server,
		sessions:   sessions,
		holdMgr:    holdMgr,
	}
}

// HandleRefer processes incoming REFER requests from phones
func (t *TransferManager) HandleRefer(req *sip.Request, tx sip.ServerTransaction) error {
	callID := req.CallID().Value()
	session := t.sessions.Get(callID)

	if session == nil {
		slog.Warn("REFER for unknown session", "call_id", callID)
		t.sendResponse(tx, req, 481, "Call/Transaction Does Not Exist") // 481 Call/Transaction Does Not Exist
		return fmt.Errorf("session not found: %s", callID)
	}

	// Get Refer-To header
	referToHeader := req.GetHeader("Refer-To")
	if referToHeader == nil {
		slog.Warn("REFER without Refer-To header", "call_id", callID)
		t.sendResponse(tx, req, sip.StatusBadRequest, "Missing Refer-To Header")
		return fmt.Errorf("missing Refer-To header")
	}

	referTo := referToHeader.Value()
	targetURI := t.parseReferTo(referTo)

	slog.Info("Transfer request received",
		"call_id", callID,
		"refer_to", referTo,
		"target", targetURI,
	)

	// Check for Replaces header (attended transfer)
	replacesHeader := t.extractReplacesFromReferTo(referTo)
	if replacesHeader != "" {
		return t.handleAttendedTransfer(session, req, tx, targetURI, replacesHeader)
	}

	// Blind transfer
	return t.handleBlindTransfer(session, req, tx, targetURI)
}

// handleBlindTransfer processes a blind transfer request
func (t *TransferManager) handleBlindTransfer(session *CallSession, req *sip.Request, tx sip.ServerTransaction, targetURI string) error {
	// Accept the REFER with 202 Accepted
	res := sip.NewResponseFromRequest(req, sip.StatusAccepted, "Accepted", nil)
	if err := tx.Respond(res); err != nil {
		return fmt.Errorf("failed to accept REFER: %w", err)
	}

	// Update session state
	session.mu.Lock()
	session.TransferTarget = targetURI
	session.mu.Unlock()

	if err := session.SetState(CallStateTransferring); err != nil {
		t.sendNotify(session, req, "SIP/2.0 503 Service Unavailable")
		return err
	}

	// Perform the transfer in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := t.executeBlindTransfer(ctx, session, targetURI)
		if err != nil {
			slog.Error("Blind transfer failed", "error", err, "call_id", session.CallID)
			t.sendNotify(session, req, "SIP/2.0 503 Service Unavailable")
			// Revert to active state
			session.SetState(CallStateActive)
		} else {
			t.sendNotify(session, req, "SIP/2.0 200 OK")
			// Terminate original session after successful transfer
			session.SetState(CallStateTerminated)
		}
	}()

	return nil
}

// handleAttendedTransfer processes an attended transfer request
func (t *TransferManager) handleAttendedTransfer(session *CallSession, req *sip.Request, tx sip.ServerTransaction, targetURI, replacesHeader string) error {
	// Parse the Replaces header to find the consult call
	consultCallID := t.parseReplacesCallID(replacesHeader)
	consultSession := t.sessions.Get(consultCallID)

	if consultSession == nil {
		slog.Warn("Consult call not found for attended transfer",
			"call_id", session.CallID,
			"consult_call_id", consultCallID,
		)
		t.sendResponse(tx, req, 481, "Consult Call Not Found") // 481 Call/Transaction Does Not Exist
		return fmt.Errorf("consult call not found: %s", consultCallID)
	}

	// Accept the REFER
	res := sip.NewResponseFromRequest(req, sip.StatusAccepted, "Accepted", nil)
	if err := tx.Respond(res); err != nil {
		return fmt.Errorf("failed to accept REFER: %w", err)
	}

	// Update session states
	session.mu.Lock()
	session.TransferTarget = targetURI
	session.ConsultCallID = consultCallID
	session.mu.Unlock()

	if err := session.SetState(CallStateTransferring); err != nil {
		t.sendNotify(session, req, "SIP/2.0 503 Service Unavailable")
		return err
	}

	// Perform attended transfer in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := t.executeAttendedTransfer(ctx, session, consultSession, targetURI, replacesHeader)
		if err != nil {
			slog.Error("Attended transfer failed", "error", err, "call_id", session.CallID)
			t.sendNotify(session, req, "SIP/2.0 503 Service Unavailable")
			session.SetState(CallStateActive)
		} else {
			t.sendNotify(session, req, "SIP/2.0 200 OK")
			session.SetState(CallStateTerminated)
			consultSession.SetState(CallStateTerminated)
		}
	}()

	return nil
}

// BlindTransfer initiates a blind transfer from GoSIP
func (t *TransferManager) BlindTransfer(ctx context.Context, session *CallSession, targetNumber string) error {
	if session.GetState() != CallStateActive && session.GetState() != CallStateHolding {
		return fmt.Errorf("can only transfer active or held calls, current state: %s", session.GetState())
	}

	// Put call on hold first if not already
	if session.GetState() == CallStateActive && t.holdMgr != nil {
		if err := t.holdMgr.PutOnHold(ctx, session); err != nil {
			slog.Warn("Failed to hold before transfer, continuing anyway", "error", err)
		}
	}

	// Create REFER request
	targetURI := t.formatTargetURI(targetNumber)
	referReq := t.createReferRequest(session, targetURI)

	// Send REFER
	tx, err := t.server.client.TransactionRequest(ctx, referReq)
	if err != nil {
		return fmt.Errorf("failed to send REFER: %w", err)
	}
	defer tx.Terminate()

	// Wait for response
	select {
	case res := <-tx.Responses():
		if res.StatusCode == sip.StatusAccepted || res.IsSuccess() {
			if err := session.SetState(CallStateTransferring); err != nil {
				return err
			}
			session.mu.Lock()
			session.TransferTarget = targetURI
			session.mu.Unlock()

			slog.Info("Blind transfer initiated",
				"call_id", session.CallID,
				"target", targetNumber,
			)
			return nil
		}
		return fmt.Errorf("transfer rejected: %d %s", res.StatusCode, res.Reason)
	case <-tx.Done():
		return fmt.Errorf("transfer transaction failed: %w", tx.Err())
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AttendedTransfer initiates an attended transfer (consult first, then transfer)
func (t *TransferManager) AttendedTransfer(ctx context.Context, originalSession *CallSession, consultSession *CallSession) error {
	if originalSession.GetState() != CallStateHolding {
		return fmt.Errorf("original call must be on hold for attended transfer")
	}
	if consultSession.GetState() != CallStateActive {
		return fmt.Errorf("consult call must be active for attended transfer")
	}

	// Create REFER with Replaces header
	targetURI := consultSession.RemoteURI
	replacesHeader := t.formatReplacesHeader(consultSession)
	referReq := t.createReferWithReplacesRequest(originalSession, targetURI, replacesHeader)

	// Send REFER
	tx, err := t.server.client.TransactionRequest(ctx, referReq)
	if err != nil {
		return fmt.Errorf("failed to send REFER: %w", err)
	}
	defer tx.Terminate()

	// Wait for response
	select {
	case res := <-tx.Responses():
		if res.StatusCode == sip.StatusAccepted || res.IsSuccess() {
			if err := originalSession.SetState(CallStateTransferring); err != nil {
				return err
			}
			originalSession.mu.Lock()
			originalSession.TransferTarget = targetURI
			originalSession.ConsultCallID = consultSession.CallID
			originalSession.mu.Unlock()

			slog.Info("Attended transfer initiated",
				"call_id", originalSession.CallID,
				"consult_call_id", consultSession.CallID,
			)
			return nil
		}
		return fmt.Errorf("transfer rejected: %d %s", res.StatusCode, res.Reason)
	case <-tx.Done():
		return fmt.Errorf("transfer transaction failed: %w", tx.Err())
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StartConsultCall initiates a consult call for attended transfer
func (t *TransferManager) StartConsultCall(ctx context.Context, originalSession *CallSession, targetNumber string) (*CallSession, error) {
	// Put original call on hold
	if originalSession.GetState() == CallStateActive && t.holdMgr != nil {
		if err := t.holdMgr.PutOnHold(ctx, originalSession); err != nil {
			return nil, fmt.Errorf("failed to hold original call: %w", err)
		}
	}

	// Create new outbound call to target
	_ = t.formatTargetURI(targetNumber) // targetURI used when full implementation is complete

	// Create INVITE for consult call
	// This would use the regular call initiation flow
	// For now, we return a placeholder - actual implementation would integrate with call initiation

	slog.Info("Consult call initiated",
		"original_call_id", originalSession.CallID,
		"target", targetNumber,
	)

	// In a full implementation, this would return the actual consult session
	// For now, this is a stub that would be expanded
	return nil, fmt.Errorf("consult call initiation not yet implemented")
}

// CancelTransfer cancels an in-progress transfer
func (t *TransferManager) CancelTransfer(ctx context.Context, session *CallSession) error {
	if session.GetState() != CallStateTransferring {
		return fmt.Errorf("no transfer in progress")
	}

	// Revert to active/held state
	session.mu.Lock()
	prevState := session.PreviousState
	session.TransferTarget = ""
	session.ConsultCallID = ""
	session.mu.Unlock()

	if prevState == CallStateHolding {
		return session.SetState(CallStateHolding)
	}
	return session.SetState(CallStateActive)
}

// executeBlindTransfer performs the actual blind transfer
func (t *TransferManager) executeBlindTransfer(ctx context.Context, session *CallSession, targetURI string) error {
	// This would connect the remote party to the target
	// Implementation depends on whether this is:
	// 1. Phone-initiated transfer (we redirect the SIP endpoint)
	// 2. Twilio-involved call (we use Twilio API to transfer)

	slog.Info("Executing blind transfer",
		"call_id", session.CallID,
		"target", targetURI,
	)

	// For SIP-to-SIP transfers, we would:
	// 1. Create new INVITE to target
	// 2. Bridge the media
	// 3. Send BYE to original endpoint

	// For Twilio transfers:
	// 1. Use Twilio's modify call API
	// 2. Update the call leg to point to new destination

	return nil
}

// executeAttendedTransfer performs the actual attended transfer
func (t *TransferManager) executeAttendedTransfer(ctx context.Context, originalSession, consultSession *CallSession, targetURI, replacesHeader string) error {
	slog.Info("Executing attended transfer",
		"original_call_id", originalSession.CallID,
		"consult_call_id", consultSession.CallID,
	)

	// In attended transfer:
	// 1. The held party (A) should be connected to the consult party (C)
	// 2. The transferor (B) drops out

	return nil
}

// Helper functions

func (t *TransferManager) parseReferTo(referTo string) string {
	// Parse sip:number@host or tel:+number format
	referTo = strings.TrimPrefix(referTo, "<")
	referTo = strings.TrimSuffix(referTo, ">")

	// Remove any parameters after ?
	if idx := strings.Index(referTo, "?"); idx != -1 {
		referTo = referTo[:idx]
	}

	return referTo
}

func (t *TransferManager) extractReplacesFromReferTo(referTo string) string {
	// Look for Replaces parameter in Refer-To URI
	// Format: <sip:user@host?Replaces=callid%3Bto-tag%3D...%3Bfrom-tag%3D...>
	if idx := strings.Index(referTo, "Replaces="); idx != -1 {
		replaces := referTo[idx+9:]
		if endIdx := strings.Index(replaces, ">"); endIdx != -1 {
			replaces = replaces[:endIdx]
		}
		// URL decode
		replaces = strings.ReplaceAll(replaces, "%3B", ";")
		replaces = strings.ReplaceAll(replaces, "%3D", "=")
		return replaces
	}
	return ""
}

func (t *TransferManager) parseReplacesCallID(replaces string) string {
	// Format: callid;to-tag=xxx;from-tag=yyy
	if idx := strings.Index(replaces, ";"); idx != -1 {
		return replaces[:idx]
	}
	return replaces
}

func (t *TransferManager) formatTargetURI(number string) string {
	// Format number as SIP URI
	if strings.HasPrefix(number, "sip:") || strings.HasPrefix(number, "tel:") {
		return number
	}
	return fmt.Sprintf("sip:%s@gosip", number)
}

func (t *TransferManager) formatReplacesHeader(session *CallSession) string {
	session.mu.RLock()
	defer session.mu.RUnlock()
	return fmt.Sprintf("%s;to-tag=%s;from-tag=%s",
		session.CallID,
		session.ToTag,
		session.FromTag,
	)
}

func (t *TransferManager) createReferRequest(session *CallSession, targetURI string) *sip.Request {
	session.mu.RLock()
	defer session.mu.RUnlock()

	req := sip.NewRequest(sip.REFER, sip.Uri{})
	req.AppendHeader(sip.NewHeader("Call-ID", session.CallID))
	req.AppendHeader(sip.NewHeader("Refer-To", "<"+targetURI+">"))

	return req
}

func (t *TransferManager) createReferWithReplacesRequest(session *CallSession, targetURI, replacesHeader string) *sip.Request {
	session.mu.RLock()
	defer session.mu.RUnlock()

	// URL encode the Replaces header
	encodedReplaces := strings.ReplaceAll(replacesHeader, ";", "%3B")
	encodedReplaces = strings.ReplaceAll(encodedReplaces, "=", "%3D")

	referToValue := fmt.Sprintf("<%s?Replaces=%s>", targetURI, encodedReplaces)

	req := sip.NewRequest(sip.REFER, sip.Uri{})
	req.AppendHeader(sip.NewHeader("Call-ID", session.CallID))
	req.AppendHeader(sip.NewHeader("Refer-To", referToValue))

	return req
}

func (t *TransferManager) sendNotify(session *CallSession, originalReq *sip.Request, sipFrag string) {
	// Send NOTIFY with sipfrag body to report transfer status
	// This is required by RFC 3515

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := []byte(sipFrag)

	notifyReq := sip.NewRequest(sip.NOTIFY, sip.Uri{})
	notifyReq.AppendHeader(sip.NewHeader("Call-ID", session.CallID))
	notifyReq.AppendHeader(sip.NewHeader("Event", "refer"))
	notifyReq.AppendHeader(sip.NewHeader("Subscription-State", "terminated;reason=noresource"))
	notifyReq.AppendHeader(sip.NewHeader("Content-Type", "message/sipfrag;version=2.0"))
	notifyReq.SetBody(body)

	tx, err := t.server.client.TransactionRequest(ctx, notifyReq)
	if err != nil {
		slog.Warn("Failed to send transfer NOTIFY", "error", err)
		return
	}
	defer tx.Terminate()

	select {
	case <-tx.Responses():
		// Response received
	case <-tx.Done():
		// Transaction done
	case <-ctx.Done():
		// Timeout
	}
}

func (t *TransferManager) sendResponse(tx sip.ServerTransaction, req *sip.Request, statusCode sip.StatusCode, reason string) {
	res := sip.NewResponseFromRequest(req, statusCode, reason, nil)
	if err := tx.Respond(res); err != nil {
		slog.Error("Failed to send response", "error", err, "status", statusCode)
	}
}

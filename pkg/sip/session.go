// Package sip provides call session management for GoSIP
package sip

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emiago/sipgo/sip"
)

// CallState represents the current state of a call
type CallState string

const (
	CallStateRinging     CallState = "ringing"
	CallStateActive      CallState = "active"
	CallStateHeld        CallState = "held"
	CallStateHolding     CallState = "holding"      // We put the other party on hold
	CallStateTransferring CallState = "transferring"
	CallStateTerminated  CallState = "terminated"
)

// CallDirection indicates inbound or outbound call
type CallDirection string

const (
	CallDirectionInbound  CallDirection = "inbound"
	CallDirectionOutbound CallDirection = "outbound"
)

// CallSession represents an active call with full state tracking
type CallSession struct {
	mu sync.RWMutex

	// SIP identifiers
	CallID    string `json:"call_id"`
	FromTag   string `json:"from_tag"`
	ToTag     string `json:"to_tag"`
	LocalURI  string `json:"local_uri"`
	RemoteURI string `json:"remote_uri"`

	// Call metadata
	Direction    CallDirection `json:"direction"`
	DeviceID     int64         `json:"device_id,omitempty"`
	DIDID        *int64        `json:"did_id,omitempty"`
	FromNumber   string        `json:"from_number"`
	ToNumber     string        `json:"to_number"`

	// State management
	State         CallState `json:"state"`
	PreviousState CallState `json:"previous_state,omitempty"`

	// Timing
	CreatedAt   time.Time  `json:"created_at"`
	AnsweredAt  *time.Time `json:"answered_at,omitempty"`
	HeldAt      *time.Time `json:"held_at,omitempty"`
	TerminatedAt *time.Time `json:"terminated_at,omitempty"`

	// SDP information for hold/resume
	LocalSDP  []byte `json:"-"`
	RemoteSDP []byte `json:"-"`
	HeldSDP   []byte `json:"-"` // SDP when hold was initiated

	// Transfer information
	TransferTarget   string `json:"transfer_target,omitempty"`
	TransferredFrom  string `json:"transferred_from,omitempty"`
	ConsultCallID    string `json:"consult_call_id,omitempty"` // For attended transfer

	// SIP transaction references (not serialized)
	serverTx sip.ServerTransaction `json:"-"`
	clientTx sip.ClientTransaction `json:"-"`
	dialog   *Dialog               `json:"-"`
}

// Dialog holds SIP dialog state for mid-call requests
type Dialog struct {
	CallID     string
	LocalTag   string
	RemoteTag  string
	LocalSeq   uint32
	RemoteSeq  uint32
	LocalURI   string
	RemoteURI  string
	RouteSet   []string
	LocalContact string
	RemoteContact string
}

// NewCallSession creates a new call session from an INVITE request
func NewCallSession(req *sip.Request, direction CallDirection) *CallSession {
	now := time.Now()

	session := &CallSession{
		CallID:      req.CallID().Value(),
		FromTag:     getTag(req.From()),
		LocalURI:    req.To().Address.String(),
		RemoteURI:   req.From().Address.String(),
		Direction:   direction,
		State:       CallStateRinging,
		CreatedAt:   now,
		FromNumber:  extractNumber(req.From().Address.String()),
		ToNumber:    extractNumber(req.To().Address.String()),
	}

	// Extract SDP if present
	if req.Body() != nil {
		session.RemoteSDP = req.Body()
	}

	return session
}

// SetState transitions the call to a new state with validation
func (s *CallSession) SetState(newState CallState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate state transition
	if !s.isValidTransition(newState) {
		return fmt.Errorf("invalid state transition: %s -> %s", s.State, newState)
	}

	s.PreviousState = s.State
	s.State = newState

	// Update timing based on state
	now := time.Now()
	switch newState {
	case CallStateActive:
		if s.AnsweredAt == nil {
			s.AnsweredAt = &now
		}
	case CallStateHeld, CallStateHolding:
		s.HeldAt = &now
	case CallStateTerminated:
		s.TerminatedAt = &now
	}

	slog.Debug("Call state changed",
		"call_id", s.CallID,
		"from_state", s.PreviousState,
		"to_state", s.State,
	)

	return nil
}

// isValidTransition checks if a state transition is allowed
func (s *CallSession) isValidTransition(newState CallState) bool {
	validTransitions := map[CallState][]CallState{
		CallStateRinging: {
			CallStateActive,
			CallStateTerminated,
		},
		CallStateActive: {
			CallStateHeld,
			CallStateHolding,
			CallStateTransferring,
			CallStateTerminated,
		},
		CallStateHeld: {
			CallStateActive,
			CallStateTerminated,
		},
		CallStateHolding: {
			CallStateActive,
			CallStateTerminated,
		},
		CallStateTransferring: {
			CallStateActive,
			CallStateHolding,
			CallStateTerminated,
		},
		CallStateTerminated: {}, // No transitions from terminated
	}

	allowed, ok := validTransitions[s.State]
	if !ok {
		return false
	}

	for _, state := range allowed {
		if state == newState {
			return true
		}
	}
	return false
}

// GetState returns the current call state
func (s *CallSession) GetState() CallState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// IsActive returns true if the call is not terminated
func (s *CallSession) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State != CallStateTerminated
}

// Duration returns the call duration in seconds
func (s *CallSession) Duration() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.AnsweredAt == nil {
		return 0
	}

	endTime := time.Now()
	if s.TerminatedAt != nil {
		endTime = *s.TerminatedAt
	}

	return int(endTime.Sub(*s.AnsweredAt).Seconds())
}

// ToJSON serializes the session for API responses
func (s *CallSession) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s)
}

// SessionManager manages all active call sessions
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*CallSession // keyed by CallID
	byDevice map[int64][]*CallSession // sessions by device ID
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*CallSession),
		byDevice: make(map[int64][]*CallSession),
	}
}

// Add adds a new session to the manager
func (m *SessionManager) Add(session *CallSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[session.CallID] = session

	if session.DeviceID > 0 {
		m.byDevice[session.DeviceID] = append(m.byDevice[session.DeviceID], session)
	}

	slog.Debug("Session added",
		"call_id", session.CallID,
		"direction", session.Direction,
		"state", session.State,
	)
}

// Get retrieves a session by CallID
func (m *SessionManager) Get(callID string) *CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[callID]
}

// GetByDevice returns all sessions for a device
func (m *SessionManager) GetByDevice(deviceID int64) []*CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := m.byDevice[deviceID]
	// Filter out terminated sessions
	active := make([]*CallSession, 0)
	for _, s := range sessions {
		if s.IsActive() {
			active = append(active, s)
		}
	}
	return active
}

// Remove removes a session from the manager
func (m *SessionManager) Remove(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[callID]
	if !ok {
		return
	}

	delete(m.sessions, callID)

	// Remove from device index
	if session.DeviceID > 0 {
		deviceSessions := m.byDevice[session.DeviceID]
		for i, s := range deviceSessions {
			if s.CallID == callID {
				m.byDevice[session.DeviceID] = append(deviceSessions[:i], deviceSessions[i+1:]...)
				break
			}
		}
	}

	slog.Debug("Session removed", "call_id", callID)
}

// GetAll returns all active sessions
func (m *SessionManager) GetAll() []*CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*CallSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.IsActive() {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// GetAllCallIDs returns all call IDs for active sessions
func (m *SessionManager) GetAllCallIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	callIDs := make([]string, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.IsActive() {
			callIDs = append(callIDs, s.CallID)
		}
	}
	return callIDs
}

// Count returns the number of active sessions
func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, s := range m.sessions {
		if s.IsActive() {
			count++
		}
	}
	return count
}

// Cleanup removes terminated sessions older than the given duration
func (m *SessionManager) Cleanup(ctx context.Context, maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for callID, session := range m.sessions {
		if session.State == CallStateTerminated && session.TerminatedAt != nil {
			if session.TerminatedAt.Before(cutoff) {
				delete(m.sessions, callID)
				removed++
			}
		}
	}

	if removed > 0 {
		slog.Debug("Cleaned up terminated sessions", "count", removed)
	}

	return removed
}

// Helper functions

func getTag(header *sip.FromHeader) string {
	if header == nil {
		return ""
	}
	// Get tag from params using the standard accessor
	if tag, ok := header.Params.Get("tag"); ok {
		return tag
	}
	return ""
}

func extractNumber(uri string) string {
	// Extract number from sip:number@host format
	// Simple extraction - can be enhanced
	if len(uri) > 4 && uri[:4] == "sip:" {
		uri = uri[4:]
	}
	for i, c := range uri {
		if c == '@' {
			return uri[:i]
		}
	}
	return uri
}

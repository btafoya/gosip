package sip

import (
	"context"
	"testing"
	"time"
)

func TestCallSession_StateTransitions(t *testing.T) {
	tests := []struct {
		name        string
		fromState   CallState
		toState     CallState
		shouldError bool
	}{
		{"ringing to active", CallStateRinging, CallStateActive, false},
		{"ringing to terminated", CallStateRinging, CallStateTerminated, false},
		{"ringing to held", CallStateRinging, CallStateHeld, true},
		{"active to held", CallStateActive, CallStateHeld, false},
		{"active to holding", CallStateActive, CallStateHolding, false},
		{"active to transferring", CallStateActive, CallStateTransferring, false},
		{"active to terminated", CallStateActive, CallStateTerminated, false},
		{"active to ringing", CallStateActive, CallStateRinging, true},
		{"held to active", CallStateHeld, CallStateActive, false},
		{"held to terminated", CallStateHeld, CallStateTerminated, false},
		{"holding to active", CallStateHolding, CallStateActive, false},
		{"holding to terminated", CallStateHolding, CallStateTerminated, false},
		{"transferring to active", CallStateTransferring, CallStateActive, false},
		{"transferring to terminated", CallStateTransferring, CallStateTerminated, false},
		{"terminated to active", CallStateTerminated, CallStateActive, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &CallSession{
				CallID: "test-call-id",
				State:  tt.fromState,
			}

			err := session.SetState(tt.toState)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for transition %s -> %s", tt.fromState, tt.toState)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for transition %s -> %s: %v", tt.fromState, tt.toState, err)
			}
		})
	}
}

func TestCallSession_Duration(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-30 * time.Second)
	terminated := now.Add(-10 * time.Second)

	tests := []struct {
		name         string
		answeredAt   *time.Time
		terminatedAt *time.Time
		expected     int
	}{
		{"not answered", nil, nil, 0},
		{"active call", &earlier, nil, 30},
		{"terminated call", &earlier, &terminated, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &CallSession{
				AnsweredAt:   tt.answeredAt,
				TerminatedAt: tt.terminatedAt,
			}
			duration := session.Duration()
			// Allow 1 second tolerance for timing
			if duration < tt.expected-1 || duration > tt.expected+1 {
				t.Errorf("expected duration ~%d, got %d", tt.expected, duration)
			}
		})
	}
}

func TestCallSession_IsActive(t *testing.T) {
	tests := []struct {
		state    CallState
		isActive bool
	}{
		{CallStateRinging, true},
		{CallStateActive, true},
		{CallStateHeld, true},
		{CallStateHolding, true},
		{CallStateTransferring, true},
		{CallStateTerminated, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			session := &CallSession{State: tt.state}
			if session.IsActive() != tt.isActive {
				t.Errorf("state %s: expected IsActive=%v, got %v", tt.state, tt.isActive, session.IsActive())
			}
		})
	}
}

func TestSessionManager_AddAndGet(t *testing.T) {
	mgr := NewSessionManager()

	session := &CallSession{
		CallID:    "call-123",
		Direction: CallDirectionInbound,
		State:     CallStateRinging,
		DeviceID:  42,
		CreatedAt: time.Now(),
	}

	mgr.Add(session)

	// Get by CallID
	retrieved := mgr.Get("call-123")
	if retrieved == nil {
		t.Fatal("failed to retrieve session by CallID")
	}
	if retrieved.CallID != session.CallID {
		t.Errorf("expected CallID %s, got %s", session.CallID, retrieved.CallID)
	}

	// Get by Device
	deviceSessions := mgr.GetByDevice(42)
	if len(deviceSessions) != 1 {
		t.Errorf("expected 1 session for device, got %d", len(deviceSessions))
	}

	// Get non-existent
	notFound := mgr.Get("non-existent")
	if notFound != nil {
		t.Error("expected nil for non-existent session")
	}
}

func TestSessionManager_Remove(t *testing.T) {
	mgr := NewSessionManager()

	session := &CallSession{
		CallID:   "call-456",
		DeviceID: 10,
		State:    CallStateActive,
	}

	mgr.Add(session)
	if mgr.Get("call-456") == nil {
		t.Fatal("session not added")
	}

	mgr.Remove("call-456")
	if mgr.Get("call-456") != nil {
		t.Error("session not removed")
	}

	// Remove non-existent should not panic
	mgr.Remove("non-existent")
}

func TestSessionManager_GetAll(t *testing.T) {
	mgr := NewSessionManager()

	sessions := []*CallSession{
		{CallID: "call-1", State: CallStateActive},
		{CallID: "call-2", State: CallStateRinging},
		{CallID: "call-3", State: CallStateTerminated},
	}

	for _, s := range sessions {
		mgr.Add(s)
	}

	active := mgr.GetAll()
	if len(active) != 2 {
		t.Errorf("expected 2 active sessions, got %d", len(active))
	}
}

func TestSessionManager_Count(t *testing.T) {
	mgr := NewSessionManager()

	if mgr.Count() != 0 {
		t.Error("expected count 0 for empty manager")
	}

	mgr.Add(&CallSession{CallID: "call-1", State: CallStateActive})
	mgr.Add(&CallSession{CallID: "call-2", State: CallStateActive})
	mgr.Add(&CallSession{CallID: "call-3", State: CallStateTerminated})

	if mgr.Count() != 2 {
		t.Errorf("expected count 2, got %d", mgr.Count())
	}
}

func TestSessionManager_Cleanup(t *testing.T) {
	mgr := NewSessionManager()
	now := time.Now()
	old := now.Add(-15 * time.Minute)
	recent := now.Add(-2 * time.Minute)

	sessions := []*CallSession{
		{CallID: "old-terminated", State: CallStateTerminated, TerminatedAt: &old},
		{CallID: "recent-terminated", State: CallStateTerminated, TerminatedAt: &recent},
		{CallID: "active", State: CallStateActive},
	}

	for _, s := range sessions {
		mgr.Add(s)
	}

	ctx := context.Background()
	removed := mgr.Cleanup(ctx, 10*time.Minute)

	if removed != 1 {
		t.Errorf("expected 1 session cleaned up, got %d", removed)
	}

	if mgr.Get("old-terminated") != nil {
		t.Error("old terminated session should be removed")
	}
	if mgr.Get("recent-terminated") == nil {
		t.Error("recent terminated session should remain")
	}
	if mgr.Get("active") == nil {
		t.Error("active session should remain")
	}
}

func TestSessionManager_Concurrent(t *testing.T) {
	mgr := NewSessionManager()
	done := make(chan bool, 100)

	// Concurrent adds
	for i := 0; i < 50; i++ {
		go func(id int) {
			session := &CallSession{
				CallID: string(rune('a' + id)),
				State:  CallStateActive,
			}
			mgr.Add(session)
			done <- true
		}(i)
	}

	// Concurrent gets
	for i := 0; i < 50; i++ {
		go func() {
			mgr.GetAll()
			mgr.Count()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"sip:1234@host", "1234"},
		{"sip:+15551234567@provider.com", "+15551234567"},
		{"1234@host", "1234"},
		{"sip:alice", "alice"},
		{"alice", "alice"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result := extractNumber(tt.uri)
			if result != tt.expected {
				t.Errorf("extractNumber(%q) = %q, want %q", tt.uri, result, tt.expected)
			}
		})
	}
}

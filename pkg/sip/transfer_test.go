package sip

import (
	"testing"
)

func TestTransferType_Constants(t *testing.T) {
	if TransferTypeBlind != "blind" {
		t.Errorf("TransferTypeBlind = %q, want %q", TransferTypeBlind, "blind")
	}
	if TransferTypeAttended != "attended" {
		t.Errorf("TransferTypeAttended = %q, want %q", TransferTypeAttended, "attended")
	}
}

func TestParseReferTo(t *testing.T) {
	mgr := &TransferManager{}

	tests := []struct {
		name     string
		referTo  string
		expected string
	}{
		{
			name:     "simple SIP URI",
			referTo:  "sip:1234@host.com",
			expected: "sip:1234@host.com",
		},
		{
			name:     "SIP URI with angle brackets",
			referTo:  "<sip:1234@host.com>",
			expected: "sip:1234@host.com",
		},
		{
			name:     "SIP URI with parameters",
			referTo:  "<sip:1234@host.com?Replaces=abc123>",
			expected: "sip:1234@host.com",
		},
		{
			name:     "tel URI",
			referTo:  "<tel:+15551234567>",
			expected: "tel:+15551234567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.parseReferTo(tt.referTo)
			if result != tt.expected {
				t.Errorf("parseReferTo(%q) = %q, want %q", tt.referTo, result, tt.expected)
			}
		})
	}
}

func TestExtractReplacesFromReferTo(t *testing.T) {
	mgr := &TransferManager{}

	tests := []struct {
		name     string
		referTo  string
		expected string
	}{
		{
			name:     "no Replaces",
			referTo:  "<sip:1234@host.com>",
			expected: "",
		},
		{
			name:     "with Replaces parameter",
			referTo:  "<sip:1234@host.com?Replaces=call-id-123%3Bto-tag%3Dabc%3Bfrom-tag%3Dxyz>",
			expected: "call-id-123;to-tag=abc;from-tag=xyz",
		},
		{
			name:     "Replaces with URL encoding",
			referTo:  "<sip:user@host?Replaces=id%3Bto-tag%3Dt1%3Bfrom-tag%3Df1>",
			expected: "id;to-tag=t1;from-tag=f1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.extractReplacesFromReferTo(tt.referTo)
			if result != tt.expected {
				t.Errorf("extractReplacesFromReferTo(%q) = %q, want %q", tt.referTo, result, tt.expected)
			}
		})
	}
}

func TestParseReplacesCallID(t *testing.T) {
	mgr := &TransferManager{}

	tests := []struct {
		name     string
		replaces string
		expected string
	}{
		{
			name:     "with tags",
			replaces: "call-id-123;to-tag=abc;from-tag=xyz",
			expected: "call-id-123",
		},
		{
			name:     "call ID only",
			replaces: "call-id-only",
			expected: "call-id-only",
		},
		{
			name:     "empty string",
			replaces: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.parseReplacesCallID(tt.replaces)
			if result != tt.expected {
				t.Errorf("parseReplacesCallID(%q) = %q, want %q", tt.replaces, result, tt.expected)
			}
		})
	}
}

func TestFormatTargetURI(t *testing.T) {
	mgr := &TransferManager{}

	tests := []struct {
		name     string
		number   string
		expected string
	}{
		{
			name:     "plain number",
			number:   "1234",
			expected: "sip:1234@gosip",
		},
		{
			name:     "E.164 number",
			number:   "+15551234567",
			expected: "sip:+15551234567@gosip",
		},
		{
			name:     "already SIP URI",
			number:   "sip:user@external.com",
			expected: "sip:user@external.com",
		},
		{
			name:     "already tel URI",
			number:   "tel:+15551234567",
			expected: "tel:+15551234567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.formatTargetURI(tt.number)
			if result != tt.expected {
				t.Errorf("formatTargetURI(%q) = %q, want %q", tt.number, result, tt.expected)
			}
		})
	}
}

func TestFormatReplacesHeader(t *testing.T) {
	mgr := &TransferManager{}

	session := &CallSession{
		CallID:  "call-123",
		ToTag:   "to-tag-abc",
		FromTag: "from-tag-xyz",
	}

	result := mgr.formatReplacesHeader(session)
	expected := "call-123;to-tag=to-tag-abc;from-tag=from-tag-xyz"

	if result != expected {
		t.Errorf("formatReplacesHeader() = %q, want %q", result, expected)
	}
}

func TestCancelTransfer(t *testing.T) {
	sessions := NewSessionManager()
	mgr := &TransferManager{sessions: sessions}

	tests := []struct {
		name          string
		currentState  CallState
		previousState CallState
		expectError   bool
		expectedState CallState
	}{
		{
			name:          "cancel active transfer",
			currentState:  CallStateTransferring,
			previousState: CallStateActive,
			expectError:   false,
			expectedState: CallStateActive,
		},
		{
			name:          "cancel transfer from held",
			currentState:  CallStateTransferring,
			previousState: CallStateHolding,
			expectError:   false,
			expectedState: CallStateHolding,
		},
		{
			name:          "no transfer in progress",
			currentState:  CallStateActive,
			previousState: "",
			expectError:   true,
			expectedState: CallStateActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &CallSession{
				CallID:         "test-cancel",
				State:          tt.currentState,
				PreviousState:  tt.previousState,
				TransferTarget: "sip:target@host",
			}

			err := mgr.CancelTransfer(nil, session)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if session.GetState() != tt.expectedState {
					t.Errorf("state = %s, want %s", session.GetState(), tt.expectedState)
				}
				if session.TransferTarget != "" {
					t.Errorf("TransferTarget = %q, want empty", session.TransferTarget)
				}
			}
		})
	}
}

func TestBlindTransfer_StateValidation(t *testing.T) {
	sessions := NewSessionManager()
	mgr := &TransferManager{sessions: sessions}

	// Only test invalid states since valid states require a working server
	tests := []struct {
		name         string
		state        CallState
		errorContain string
	}{
		{
			name:         "ringing call",
			state:        CallStateRinging,
			errorContain: "can only transfer active or held calls",
		},
		{
			name:         "terminated call",
			state:        CallStateTerminated,
			errorContain: "can only transfer active or held calls",
		},
		{
			name:         "held call (not holding)",
			state:        CallStateHeld,
			errorContain: "can only transfer active or held calls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &CallSession{
				CallID: "test-blind",
				State:  tt.state,
			}

			err := mgr.BlindTransfer(nil, session, "1234")

			if err == nil {
				t.Error("expected error, got nil")
			} else if !contains(err.Error(), tt.errorContain) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errorContain)
			}
		})
	}
}

func TestAttendedTransfer_StateValidation(t *testing.T) {
	sessions := NewSessionManager()
	mgr := &TransferManager{sessions: sessions}

	// Only test invalid states since valid states require a working server
	tests := []struct {
		name           string
		originalState  CallState
		consultState   CallState
		errorContain   string
	}{
		{
			name:           "original not held",
			originalState:  CallStateActive,
			consultState:   CallStateActive,
			errorContain:   "original call must be on hold",
		},
		{
			name:           "consult not active",
			originalState:  CallStateHolding,
			consultState:   CallStateHeld,
			errorContain:   "consult call must be active",
		},
		{
			name:           "original ringing",
			originalState:  CallStateRinging,
			consultState:   CallStateActive,
			errorContain:   "original call must be on hold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSession := &CallSession{
				CallID: "original-call",
				State:  tt.originalState,
			}
			consultSession := &CallSession{
				CallID:    "consult-call",
				State:     tt.consultState,
				RemoteURI: "sip:target@host",
			}

			err := mgr.AttendedTransfer(nil, originalSession, consultSession)

			if err == nil {
				t.Error("expected error, got nil")
			} else if !contains(err.Error(), tt.errorContain) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errorContain)
			}
		})
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

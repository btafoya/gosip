package sip

import (
	"testing"
)

func TestParseHoldFromSDP(t *testing.T) {
	tests := []struct {
		name     string
		sdp      string
		expected HoldType
	}{
		{
			name: "sendonly (remote hold)",
			sdp: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=sendonly`,
			expected: HoldTypeSendOnly,
		},
		{
			name: "recvonly",
			sdp: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=recvonly`,
			expected: HoldTypeRecvOnly,
		},
		{
			name: "inactive (both directions held)",
			sdp: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=inactive`,
			expected: HoldTypeInactive,
		},
		{
			name: "RFC 2543 style hold (0.0.0.0)",
			sdp: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 0.0.0.0
t=0 0
m=audio 5000 RTP/AVP 0`,
			expected: HoldTypeInactive,
		},
		{
			name: "normal call (sendrecv)",
			sdp: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=sendrecv`,
			expected: "",
		},
		{
			name: "no direction attribute (default sendrecv)",
			sdp: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseHoldFromSDP([]byte(tt.sdp))
			if result != tt.expected {
				t.Errorf("ParseHoldFromSDP() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIsHoldSDP(t *testing.T) {
	holdSDP := []byte(`v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=sendonly`)

	normalSDP := []byte(`v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=sendrecv`)

	if !IsHoldSDP(holdSDP) {
		t.Error("IsHoldSDP should return true for hold SDP")
	}

	if IsHoldSDP(normalSDP) {
		t.Error("IsHoldSDP should return false for normal SDP")
	}
}

func TestModifySDPDirection(t *testing.T) {
	tests := []struct {
		name         string
		inputSDP     string
		newDirection string
		expected     string
	}{
		{
			name: "change sendrecv to sendonly",
			inputSDP: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=sendrecv
a=rtpmap:0 PCMU/8000`,
			newDirection: "sendonly",
			expected:     "sendonly",
		},
		{
			name: "change sendonly to sendrecv",
			inputSDP: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=sendonly
a=rtpmap:0 PCMU/8000`,
			newDirection: "sendrecv",
			expected:     "sendrecv",
		},
		{
			name: "add recvonly to SDP without direction",
			inputSDP: `v=0
o=- 123 456 IN IP4 192.168.1.1
s=Call
c=IN IP4 192.168.1.1
t=0 0
m=audio 5000 RTP/AVP 0
a=rtpmap:0 PCMU/8000`,
			newDirection: "recvonly",
			expected:     "recvonly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ModifySDPDirection([]byte(tt.inputSDP), tt.newDirection)
			resultType := ParseHoldFromSDP(result)

			// For sendrecv, the result should be empty (no hold)
			if tt.expected == "sendrecv" {
				if resultType != "" {
					t.Errorf("ModifySDPDirection() result has hold type %q, want no hold", resultType)
				}
			} else {
				if string(resultType) != tt.expected {
					t.Errorf("ModifySDPDirection() result has hold type %q, want %q", resultType, tt.expected)
				}
			}
		})
	}
}

func TestNormalizeSDP(t *testing.T) {
	// Test that SDP gets proper CRLF line endings
	input := []byte("v=0\no=- 123 456 IN IP4 192.168.1.1\ns=Call\n")
	result := NormalizeSDP(input)

	// Check that all \n are converted to \r\n
	expected := []byte("v=0\r\no=- 123 456 IN IP4 192.168.1.1\r\ns=Call\r\n")
	if string(result) != string(expected) {
		t.Errorf("NormalizeSDP() = %q, want %q", result, expected)
	}
}

func TestHoldType_Constants(t *testing.T) {
	if HoldTypeSendOnly != "sendonly" {
		t.Error("HoldTypeSendOnly should be 'sendonly'")
	}
	if HoldTypeRecvOnly != "recvonly" {
		t.Error("HoldTypeRecvOnly should be 'recvonly'")
	}
	if HoldTypeInactive != "inactive" {
		t.Error("HoldTypeInactive should be 'inactive'")
	}
}

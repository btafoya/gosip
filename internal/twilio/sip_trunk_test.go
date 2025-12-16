package twilio

import (
	"testing"

	"github.com/btafoya/gosip/internal/config"
)

func TestIsSecureSIPURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected bool
	}{
		{
			name:     "sips scheme",
			uri:      "sips:example.com",
			expected: true,
		},
		{
			name:     "sips scheme with port",
			uri:      "sips:example.com:5061",
			expected: true,
		},
		{
			name:     "sip scheme with port 5061",
			uri:      "sip:example.com:5061",
			expected: true,
		},
		{
			name:     "sip scheme with port 5060",
			uri:      "sip:example.com:5060",
			expected: false,
		},
		{
			name:     "sip scheme without port",
			uri:      "sip:example.com",
			expected: false,
		},
		{
			name:     "empty string",
			uri:      "",
			expected: false,
		},
		{
			name:     "short string",
			uri:      "sip",
			expected: false,
		},
		{
			name:     "sips with user",
			uri:      "sips:user@example.com:5061",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSecureSIPURI(tt.uri)
			if result != tt.expected {
				t.Errorf("isSecureSIPURI(%q) = %v, want %v", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "contains substring",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "does not contain",
			s:        "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "both empty",
			s:        "",
			substr:   "",
			expected: true,
		},
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "hi",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "port in URI",
			s:        "sip:example.com:5061",
			substr:   ":5061",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestConvertToSecureURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "sip to sips",
			uri:      "sip:example.com",
			expected: "sips:example.com",
		},
		{
			name:     "port 5060 to 5061",
			uri:      "sip:example.com:5060",
			expected: "sips:example.com:5061",
		},
		{
			name:     "already secure sips",
			uri:      "sips:example.com",
			expected: "sips:example.com",
		},
		{
			name:     "already secure 5061",
			uri:      "sips:example.com:5061",
			expected: "sips:example.com:5061",
		},
		{
			name:     "with user info",
			uri:      "sip:user@example.com:5060",
			expected: "sips:user@example.com:5061",
		},
		{
			name:     "non-standard port",
			uri:      "sip:example.com:5080",
			expected: "sips:example.com:5080",
		},
		{
			name:     "empty string",
			uri:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToSecureURI(tt.uri)
			if result != tt.expected {
				t.Errorf("convertToSecureURI(%q) = %q, want %q", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestSIPTrunk_Struct(t *testing.T) {
	trunk := SIPTrunk{
		SID:               "TK123456",
		FriendlyName:      "Main Trunk",
		DomainName:        "example.sip.twilio.com",
		Secure:            true,
		TransferMode:      "enable-all",
		CnamLookupEnabled: true,
	}

	if trunk.SID != "TK123456" {
		t.Errorf("SID = %s, want TK123456", trunk.SID)
	}
	if trunk.FriendlyName != "Main Trunk" {
		t.Errorf("FriendlyName = %s, want Main Trunk", trunk.FriendlyName)
	}
	if trunk.DomainName != "example.sip.twilio.com" {
		t.Errorf("DomainName mismatch")
	}
	if !trunk.Secure {
		t.Error("Secure should be true")
	}
	if trunk.TransferMode != "enable-all" {
		t.Errorf("TransferMode = %s, want enable-all", trunk.TransferMode)
	}
	if !trunk.CnamLookupEnabled {
		t.Error("CnamLookupEnabled should be true")
	}
}

func TestOriginationURL_Struct(t *testing.T) {
	url := OriginationURL{
		SID:          "OU123456",
		SipURL:       "sips:pbx.example.com:5061",
		FriendlyName: "Primary PBX",
		Priority:     10,
		Weight:       50,
		Enabled:      true,
	}

	if url.SID != "OU123456" {
		t.Errorf("SID = %s, want OU123456", url.SID)
	}
	if url.SipURL != "sips:pbx.example.com:5061" {
		t.Errorf("SipURL mismatch")
	}
	if url.Priority != 10 {
		t.Errorf("Priority = %d, want 10", url.Priority)
	}
	if url.Weight != 50 {
		t.Errorf("Weight = %d, want 50", url.Weight)
	}
	if !url.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestSIPDomain_Struct(t *testing.T) {
	domain := SIPDomain{
		SID:                    "SD123456",
		DomainName:             "example.sip.us1.twilio.com",
		FriendlyName:           "Main Domain",
		VoiceURL:               "https://example.com/voice",
		VoiceFallbackURL:       "https://example.com/voice-fallback",
		VoiceStatusCallbackURL: "https://example.com/voice-status",
	}

	if domain.SID != "SD123456" {
		t.Errorf("SID = %s, want SD123456", domain.SID)
	}
	if domain.DomainName != "example.sip.us1.twilio.com" {
		t.Errorf("DomainName mismatch")
	}
	if domain.VoiceURL != "https://example.com/voice" {
		t.Errorf("VoiceURL mismatch")
	}
}

func TestTrunkTLSStatus_Struct(t *testing.T) {
	status := TrunkTLSStatus{
		TrunkSID:     "TK123456",
		FriendlyName: "Secure Trunk",
		SecureMode:   true,
		OriginationURLs: []*OriginationURL{
			{SID: "OU1", SipURL: "sips:pbx.example.com:5061", Enabled: true},
			{SID: "OU2", SipURL: "sip:pbx2.example.com:5060", Enabled: true},
		},
		AllSecure:        false,
		InsecureURLCount: 1,
	}

	if status.TrunkSID != "TK123456" {
		t.Errorf("TrunkSID mismatch")
	}
	if status.AllSecure {
		t.Error("AllSecure should be false when there's an insecure URL")
	}
	if status.InsecureURLCount != 1 {
		t.Errorf("InsecureURLCount = %d, want 1", status.InsecureURLCount)
	}
	if len(status.OriginationURLs) != 2 {
		t.Errorf("OriginationURLs length = %d, want 2", len(status.OriginationURLs))
	}
}

// Tests for SIP trunk API methods (no client)

func TestClient_CreateSIPTrunk_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.CreateSIPTrunk(nil, "Test Trunk", true)
	if err == nil {
		t.Error("CreateSIPTrunk should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_GetSIPTrunk_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.GetSIPTrunk(nil, "TK123")
	if err == nil {
		t.Error("GetSIPTrunk should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_ListSIPTrunks_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.ListSIPTrunks(nil)
	if err == nil {
		t.Error("ListSIPTrunks should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_DeleteSIPTrunk_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.DeleteSIPTrunk(nil, "TK123")
	if err == nil {
		t.Error("DeleteSIPTrunk should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_AssignPhoneNumberToTrunk_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.AssignPhoneNumberToTrunk(nil, "TK123", "PN123")
	if err == nil {
		t.Error("AssignPhoneNumberToTrunk should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_CreateSIPDomain_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.CreateSIPDomain(nil, "example.sip.twilio.com", "Test", "")
	if err == nil {
		t.Error("CreateSIPDomain should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_GetSIPDomain_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.GetSIPDomain(nil, "SD123")
	if err == nil {
		t.Error("GetSIPDomain should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_UpdateSIPDomain_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.UpdateSIPDomain(nil, "SD123", "https://example.com", "", "")
	if err == nil {
		t.Error("UpdateSIPDomain should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_DeleteSIPDomain_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.DeleteSIPDomain(nil, "SD123")
	if err == nil {
		t.Error("DeleteSIPDomain should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_CreateCredentialList_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.CreateCredentialList(nil, "Test Creds")
	if err == nil {
		t.Error("CreateCredentialList should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_AddCredential_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.AddCredential(nil, "CL123", "user", "pass")
	if err == nil {
		t.Error("AddCredential should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_MapCredentialListToDomain_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.MapCredentialListToDomain(nil, "SD123", "CL123")
	if err == nil {
		t.Error("MapCredentialListToDomain should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_SetOriginationURI_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.SetOriginationURI(nil, "TK123", "sip:example.com", 10, 50)
	if err == nil {
		t.Error("SetOriginationURI should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_UpdateSIPTrunk_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.UpdateSIPTrunk(nil, "TK123", true, "New Name")
	if err == nil {
		t.Error("UpdateSIPTrunk should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_EnableTLSForTrunk_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.EnableTLSForTrunk(nil, "TK123")
	if err == nil {
		t.Error("EnableTLSForTrunk should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_DisableTLSForTrunk_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.DisableTLSForTrunk(nil, "TK123")
	if err == nil {
		t.Error("DisableTLSForTrunk should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_SetSecureOriginationURI_InvalidURI(t *testing.T) {
	cfg := &config.Config{
		TwilioAccountSID: "AC123",
		TwilioAuthToken:  "token123",
	}
	client := NewClient(cfg)

	// Should fail validation before trying to call API
	err := client.SetSecureOriginationURI(nil, "TK123", "sip:example.com:5060", 10, 50)
	if err == nil {
		t.Error("SetSecureOriginationURI should error with insecure URI")
	}
	if err.Error() != "origination URI must use sips: scheme or port 5061 for TLS: sip:example.com:5060" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_ListOriginationURLs_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	_, err := client.ListOriginationURLs(nil, "TK123")
	if err == nil {
		t.Error("ListOriginationURLs should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_UpdateOriginationURL_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.UpdateOriginationURL(nil, "TK123", "OU123", "sip:example.com", 10, 50, true)
	if err == nil {
		t.Error("UpdateOriginationURL should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_DeleteOriginationURL_NoClient(t *testing.T) {
	cfg := &config.Config{}
	client := NewClient(cfg)

	err := client.DeleteOriginationURL(nil, "TK123", "OU123")
	if err == nil {
		t.Error("DeleteOriginationURL should error when client not initialized")
	}
	if err.Error() != "twilio client not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

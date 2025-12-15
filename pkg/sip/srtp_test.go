package sip

import (
	"bytes"
	"testing"
)

func TestGenerateKeyMaterial(t *testing.T) {
	tests := []struct {
		name        string
		profile     SRTPProfile
		keyLen      int
		saltLen     int
		shouldError bool
	}{
		{
			name:    "AES_CM_128_HMAC_SHA1_80",
			profile: SRTPProfileAES128CMHMACSHA180,
			keyLen:  16,
			saltLen: 14,
		},
		{
			name:    "AES_CM_128_HMAC_SHA1_32",
			profile: SRTPProfileAES128CMHMACSHA132,
			keyLen:  16,
			saltLen: 14,
		},
		{
			name:    "AEAD_AES_128_GCM",
			profile: SRTPProfileAEADAES128GCM,
			keyLen:  16,
			saltLen: 14,
		},
		{
			name:    "AEAD_AES_256_GCM",
			profile: SRTPProfileAEADAES256GCM,
			keyLen:  32,
			saltLen: 12,
		},
		{
			name:    "empty profile defaults to AES_CM_128",
			profile: "",
			keyLen:  16,
			saltLen: 14,
		},
		{
			name:        "invalid profile",
			profile:     "INVALID_PROFILE",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			material, err := GenerateKeyMaterial(tt.profile)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(material.MasterKey) != tt.keyLen {
				t.Errorf("Expected key length %d, got %d", tt.keyLen, len(material.MasterKey))
			}

			if len(material.MasterSalt) != tt.saltLen {
				t.Errorf("Expected salt length %d, got %d", tt.saltLen, len(material.MasterSalt))
			}

			// Keys should be non-zero
			allZero := true
			for _, b := range material.MasterKey {
				if b != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				t.Error("Master key should not be all zeros")
			}
		})
	}
}

func TestGenerateKeyMaterial_Uniqueness(t *testing.T) {
	// Generate multiple key materials and ensure they're unique
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		material, err := GenerateKeyMaterial(SRTPProfileAES128CMHMACSHA180)
		if err != nil {
			t.Fatalf("Failed to generate key material: %v", err)
		}

		keyStr := string(material.MasterKey)
		if keys[keyStr] {
			t.Error("Duplicate key generated")
		}
		keys[keyStr] = true
	}
}

func TestNewSRTPContext(t *testing.T) {
	tests := []struct {
		name        string
		material    *SRTPKeyMaterial
		shouldError bool
	}{
		{
			name:        "nil material",
			material:    nil,
			shouldError: true,
		},
		{
			name: "valid AES_CM_128 material",
			material: &SRTPKeyMaterial{
				MasterKey:  make([]byte, 16),
				MasterSalt: make([]byte, 14),
				Profile:    SRTPProfileAES128CMHMACSHA180,
			},
			shouldError: false,
		},
		{
			name: "valid AEAD_AES_128_GCM material",
			material: &SRTPKeyMaterial{
				MasterKey:  make([]byte, 16),
				MasterSalt: make([]byte, 12),
				Profile:    SRTPProfileAEADAES128GCM,
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := NewSRTPContext(tt.material)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if ctx == nil {
				t.Error("Context should not be nil")
			}

			// Clean up
			ctx.Close()
		})
	}
}

func TestSRTPContext_EncryptDecrypt(t *testing.T) {
	material, err := GenerateKeyMaterial(SRTPProfileAES128CMHMACSHA180)
	if err != nil {
		t.Fatalf("Failed to generate key material: %v", err)
	}

	ctx, err := NewSRTPContext(material)
	if err != nil {
		t.Fatalf("Failed to create SRTP context: %v", err)
	}
	defer ctx.Close()

	// Create a simple RTP header
	header := &RTPHeader{
		Version:        2,
		PayloadType:    0,
		SequenceNumber: 1,
		Timestamp:      12345,
		SSRC:           67890,
	}

	// Test payload data
	payload := []byte("test audio payload data")

	// Build a complete RTP packet (header + payload)
	// RTP header is 12 bytes minimum (no CSRCs)
	rtpHeader := header.toRTPHeader()
	headerBytes, err := rtpHeader.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal RTP header: %v", err)
	}

	// Create full RTP packet
	rtpPacket := append(headerBytes, payload...)

	// Encrypt the full packet
	encrypted, err := ctx.EncryptRTP(nil, rtpPacket, header)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Encrypted data should be different from original
	if bytes.Equal(encrypted, rtpPacket) {
		t.Error("Encrypted data should be different from original")
	}

	// Encrypted data should be larger (includes auth tag)
	if len(encrypted) <= len(rtpPacket) {
		t.Error("Encrypted data should include auth tag")
	}
}

func TestSDPCryptoAttribute_String(t *testing.T) {
	tests := []struct {
		name     string
		attr     SDPCryptoAttribute
		expected string
	}{
		{
			name: "basic crypto attribute",
			attr: SDPCryptoAttribute{
				Tag:         1,
				CryptoSuite: "AES_CM_128_HMAC_SHA1_80",
				KeyMethod:   "inline",
				KeyInfo:     "dGVzdGtleQ==",
			},
			expected: "a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:dGVzdGtleQ==",
		},
		{
			name: "with session params",
			attr: SDPCryptoAttribute{
				Tag:           1,
				CryptoSuite:   "AES_CM_128_HMAC_SHA1_80",
				KeyMethod:     "inline",
				KeyInfo:       "dGVzdGtleQ==",
				SessionParams: []string{"UNENCRYPTED_SRTP"},
			},
			expected: "a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:dGVzdGtleQ== UNENCRYPTED_SRTP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.attr.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseSDPCryptoAttribute(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedTag int
		expectedCS  string
		shouldError bool
	}{
		{
			name:        "valid crypto attribute",
			input:       "1 AES_CM_128_HMAC_SHA1_80 inline:dGVzdGtleQ==",
			expectedTag: 1,
			expectedCS:  "AES_CM_128_HMAC_SHA1_80",
		},
		{
			name:        "with a=crypto prefix",
			input:       "a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:dGVzdGtleQ==",
			expectedTag: 1,
			expectedCS:  "AES_CM_128_HMAC_SHA1_80",
		},
		{
			name:        "with session params",
			input:       "1 AES_CM_128_HMAC_SHA1_80 inline:dGVzdGtleQ== UNENCRYPTED_SRTP",
			expectedTag: 1,
			expectedCS:  "AES_CM_128_HMAC_SHA1_80",
		},
		{
			name:        "invalid format - too few parts",
			input:       "1 AES_CM_128_HMAC_SHA1_80",
			shouldError: true,
		},
		{
			name:        "invalid tag",
			input:       "abc AES_CM_128_HMAC_SHA1_80 inline:key",
			shouldError: true,
		},
		{
			name:        "invalid key method format",
			input:       "1 AES_CM_128_HMAC_SHA1_80 inlinekey",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr, err := ParseSDPCryptoAttribute(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if attr.Tag != tt.expectedTag {
				t.Errorf("Expected tag %d, got %d", tt.expectedTag, attr.Tag)
			}

			if attr.CryptoSuite != tt.expectedCS {
				t.Errorf("Expected crypto suite %q, got %q", tt.expectedCS, attr.CryptoSuite)
			}
		})
	}
}

func TestSRTPKeyMaterial_ToSDPCryptoAttribute(t *testing.T) {
	material := &SRTPKeyMaterial{
		MasterKey:  []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		MasterSalt: []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e},
		Profile:    SRTPProfileAES128CMHMACSHA180,
	}

	attr := material.ToSDPCryptoAttribute(1)

	if attr.Tag != 1 {
		t.Errorf("Expected tag 1, got %d", attr.Tag)
	}

	if attr.CryptoSuite != string(SRTPProfileAES128CMHMACSHA180) {
		t.Errorf("Expected crypto suite %s, got %s", SRTPProfileAES128CMHMACSHA180, attr.CryptoSuite)
	}

	if attr.KeyMethod != "inline" {
		t.Errorf("Expected key method 'inline', got %s", attr.KeyMethod)
	}

	// Verify we can extract the key material back
	extracted, err := attr.ExtractKeyMaterial()
	if err != nil {
		t.Fatalf("Failed to extract key material: %v", err)
	}

	if !bytes.Equal(extracted.MasterKey, material.MasterKey) {
		t.Error("Master key mismatch after extraction")
	}

	if !bytes.Equal(extracted.MasterSalt, material.MasterSalt) {
		t.Error("Master salt mismatch after extraction")
	}
}

func TestSRTPSessionManager(t *testing.T) {
	mgr := NewSRTPSessionManager()
	defer mgr.Close()

	material, err := GenerateKeyMaterial(SRTPProfileAES128CMHMACSHA180)
	if err != nil {
		t.Fatalf("Failed to generate key material: %v", err)
	}

	t.Run("GetOrCreate", func(t *testing.T) {
		ctx, err := mgr.GetOrCreate("call-1", material)
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}
		if ctx == nil {
			t.Error("Context should not be nil")
		}

		// Get the same context again
		ctx2, err := mgr.GetOrCreate("call-1", material)
		if err != nil {
			t.Fatalf("Failed to get context: %v", err)
		}
		if ctx != ctx2 {
			t.Error("Should return the same context for same call ID")
		}
	})

	t.Run("Get", func(t *testing.T) {
		ctx, ok := mgr.Get("call-1")
		if !ok {
			t.Error("Should find existing context")
		}
		if ctx == nil {
			t.Error("Context should not be nil")
		}

		_, ok = mgr.Get("nonexistent")
		if ok {
			t.Error("Should not find nonexistent context")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		err := mgr.Remove("call-1")
		if err != nil {
			t.Errorf("Failed to remove context: %v", err)
		}

		_, ok := mgr.Get("call-1")
		if ok {
			t.Error("Context should be removed")
		}

		// Removing nonexistent should not error
		err = mgr.Remove("nonexistent")
		if err != nil {
			t.Errorf("Removing nonexistent should not error: %v", err)
		}
	})
}

func TestValidSRTPProfiles(t *testing.T) {
	profiles := ValidSRTPProfiles()

	expected := []SRTPProfile{
		SRTPProfileAES128CMHMACSHA180,
		SRTPProfileAES128CMHMACSHA132,
		SRTPProfileAEADAES128GCM,
		SRTPProfileAEADAES256GCM,
	}

	if len(profiles) != len(expected) {
		t.Errorf("Expected %d profiles, got %d", len(expected), len(profiles))
	}

	for _, p := range expected {
		found := false
		for _, actual := range profiles {
			if actual == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected profile %s not found", p)
		}
	}
}

func TestIsValidSRTPProfile(t *testing.T) {
	tests := []struct {
		profile string
		valid   bool
	}{
		{"AES_CM_128_HMAC_SHA1_80", true},
		{"AES_CM_128_HMAC_SHA1_32", true},
		{"AEAD_AES_128_GCM", true},
		{"AEAD_AES_256_GCM", true},
		{"INVALID", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			if IsValidSRTPProfile(tt.profile) != tt.valid {
				t.Errorf("IsValidSRTPProfile(%q) = %v, want %v", tt.profile, !tt.valid, tt.valid)
			}
		})
	}
}

func TestRTPHeader_ToRTPHeader(t *testing.T) {
	header := &RTPHeader{
		Version:        2,
		Padding:        true,
		Extension:      false,
		Marker:         true,
		PayloadType:    96,
		SequenceNumber: 12345,
		Timestamp:      67890,
		SSRC:           11223344,
		CSRC:           []uint32{1, 2, 3},
	}

	rtpHeader := header.toRTPHeader()

	if rtpHeader.Version != header.Version {
		t.Errorf("Version mismatch: %d != %d", rtpHeader.Version, header.Version)
	}
	if rtpHeader.Padding != header.Padding {
		t.Errorf("Padding mismatch: %v != %v", rtpHeader.Padding, header.Padding)
	}
	if rtpHeader.Extension != header.Extension {
		t.Errorf("Extension mismatch: %v != %v", rtpHeader.Extension, header.Extension)
	}
	if rtpHeader.Marker != header.Marker {
		t.Errorf("Marker mismatch: %v != %v", rtpHeader.Marker, header.Marker)
	}
	if rtpHeader.PayloadType != header.PayloadType {
		t.Errorf("PayloadType mismatch: %d != %d", rtpHeader.PayloadType, header.PayloadType)
	}
	if rtpHeader.SequenceNumber != header.SequenceNumber {
		t.Errorf("SequenceNumber mismatch: %d != %d", rtpHeader.SequenceNumber, header.SequenceNumber)
	}
	if rtpHeader.Timestamp != header.Timestamp {
		t.Errorf("Timestamp mismatch: %d != %d", rtpHeader.Timestamp, header.Timestamp)
	}
	if rtpHeader.SSRC != header.SSRC {
		t.Errorf("SSRC mismatch: %d != %d", rtpHeader.SSRC, header.SSRC)
	}
}

func TestRTPHeader_ToRTPHeader_Nil(t *testing.T) {
	var header *RTPHeader
	if header.toRTPHeader() != nil {
		t.Error("Nil header should return nil")
	}
}

// Package sip provides SIP server functionality using sipgo
package sip

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/srtp/v2"
)

// SRTPProfile represents the SRTP encryption profile
type SRTPProfile string

const (
	// SRTPProfileAES128CMHMACSHA180 is the default SRTP profile
	SRTPProfileAES128CMHMACSHA180 SRTPProfile = "AES_CM_128_HMAC_SHA1_80"
	// SRTPProfileAES128CMHMACSHA132 is an alternative SRTP profile with smaller auth tag
	SRTPProfileAES128CMHMACSHA132 SRTPProfile = "AES_CM_128_HMAC_SHA1_32"
	// SRTPProfileAEADAES128GCM is the AEAD AES-128-GCM profile
	SRTPProfileAEADAES128GCM SRTPProfile = "AEAD_AES_128_GCM"
	// SRTPProfileAEADAES256GCM is the AEAD AES-256-GCM profile
	SRTPProfileAEADAES256GCM SRTPProfile = "AEAD_AES_256_GCM"
)

// SRTPKeyMaterial holds the keying material for SRTP
type SRTPKeyMaterial struct {
	MasterKey  []byte
	MasterSalt []byte
	Profile    SRTPProfile
}

// SRTPContext wraps pion/srtp for media encryption
type SRTPContext struct {
	encryptCtx *srtp.Context
	decryptCtx *srtp.Context
	profile    SRTPProfile
	mu         sync.RWMutex
}

// NewSRTPContext creates SRTP encryption/decryption contexts
func NewSRTPContext(material *SRTPKeyMaterial) (*SRTPContext, error) {
	if material == nil {
		return nil, fmt.Errorf("key material required")
	}

	profile, err := getProtectionProfile(material.Profile)
	if err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}

	// Create encryption context
	encryptCtx, err := srtp.CreateContext(material.MasterKey, material.MasterSalt, profile)
	if err != nil {
		return nil, fmt.Errorf("create encrypt context: %w", err)
	}

	// Create decryption context with replay protection
	decryptCtx, err := srtp.CreateContext(material.MasterKey, material.MasterSalt, profile, srtp.SRTPReplayProtection(256))
	if err != nil {
		return nil, fmt.Errorf("create decrypt context: %w", err)
	}

	return &SRTPContext{
		encryptCtx: encryptCtx,
		decryptCtx: decryptCtx,
		profile:    material.Profile,
	}, nil
}

// getProtectionProfile converts our profile string to pion/srtp profile
func getProtectionProfile(profile SRTPProfile) (srtp.ProtectionProfile, error) {
	switch profile {
	case SRTPProfileAES128CMHMACSHA180, "":
		return srtp.ProtectionProfileAes128CmHmacSha1_80, nil
	case SRTPProfileAES128CMHMACSHA132:
		return srtp.ProtectionProfileAes128CmHmacSha1_32, nil
	case SRTPProfileAEADAES128GCM:
		return srtp.ProtectionProfileAeadAes128Gcm, nil
	case SRTPProfileAEADAES256GCM:
		return srtp.ProtectionProfileAeadAes256Gcm, nil
	default:
		return 0, fmt.Errorf("unknown SRTP profile: %s", profile)
	}
}

// EncryptRTP encrypts an RTP packet
func (s *SRTPContext) EncryptRTP(dst, src []byte, header *RTPHeader) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.encryptCtx == nil {
		return nil, fmt.Errorf("encryption context not initialized")
	}

	return s.encryptCtx.EncryptRTP(dst, src, header.toRTPHeader())
}

// DecryptRTP decrypts an SRTP packet
func (s *SRTPContext) DecryptRTP(dst, src []byte, header *RTPHeader) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.decryptCtx == nil {
		return nil, fmt.Errorf("decryption context not initialized")
	}

	return s.decryptCtx.DecryptRTP(dst, src, header.toRTPHeader())
}

// EncryptRTCP encrypts an RTCP packet
func (s *SRTPContext) EncryptRTCP(dst, src []byte, header *rtcp.Header) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.encryptCtx == nil {
		return nil, fmt.Errorf("encryption context not initialized")
	}

	return s.encryptCtx.EncryptRTCP(dst, src, header)
}

// DecryptRTCP decrypts an SRTCP packet
func (s *SRTPContext) DecryptRTCP(dst, src []byte, header *rtcp.Header) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.decryptCtx == nil {
		return nil, fmt.Errorf("decryption context not initialized")
	}

	return s.decryptCtx.DecryptRTCP(dst, src, header)
}

// Close releases the SRTP context resources
func (s *SRTPContext) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// pion/srtp contexts don't have a Close method
	// Just clear references
	s.encryptCtx = nil
	s.decryptCtx = nil
	return nil
}

// RTPHeader represents a minimal RTP header for SRTP operations
type RTPHeader struct {
	Version        uint8
	Padding        bool
	Extension      bool
	Marker         bool
	PayloadType    uint8
	SequenceNumber uint16
	Timestamp      uint32
	SSRC           uint32
	CSRC           []uint32
}

// toRTPHeader converts to pion/rtp Header format
func (h *RTPHeader) toRTPHeader() *rtp.Header {
	if h == nil {
		return nil
	}
	return &rtp.Header{
		Version:        h.Version,
		Padding:        h.Padding,
		Extension:      h.Extension,
		Marker:         h.Marker,
		PayloadType:    h.PayloadType,
		SequenceNumber: h.SequenceNumber,
		Timestamp:      h.Timestamp,
		SSRC:           h.SSRC,
		CSRC:           h.CSRC,
	}
}

// SDPCryptoAttribute represents an SDP crypto attribute for SRTP key exchange
type SDPCryptoAttribute struct {
	Tag           int
	CryptoSuite   string
	KeyMethod     string
	KeyInfo       string // Base64-encoded key material
	SessionParams []string
}

// GenerateKeyMaterial generates new random SRTP key material
func GenerateKeyMaterial(profile SRTPProfile) (*SRTPKeyMaterial, error) {
	var keyLen, saltLen int

	switch profile {
	case SRTPProfileAES128CMHMACSHA180, SRTPProfileAES128CMHMACSHA132, SRTPProfileAEADAES128GCM, "":
		keyLen = 16  // 128 bits
		saltLen = 14 // 112 bits
	case SRTPProfileAEADAES256GCM:
		keyLen = 32  // 256 bits
		saltLen = 12 // 96 bits for GCM
	default:
		return nil, fmt.Errorf("unknown SRTP profile: %s", profile)
	}

	masterKey := make([]byte, keyLen)
	if _, err := rand.Read(masterKey); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}

	masterSalt := make([]byte, saltLen)
	if _, err := rand.Read(masterSalt); err != nil {
		return nil, fmt.Errorf("generate master salt: %w", err)
	}

	if profile == "" {
		profile = SRTPProfileAES128CMHMACSHA180
	}

	return &SRTPKeyMaterial{
		MasterKey:  masterKey,
		MasterSalt: masterSalt,
		Profile:    profile,
	}, nil
}

// ToSDPCryptoAttribute converts key material to an SDP crypto attribute
func (m *SRTPKeyMaterial) ToSDPCryptoAttribute(tag int) *SDPCryptoAttribute {
	// Combine key and salt for inline encoding
	combined := append(m.MasterKey, m.MasterSalt...)
	keyInfo := base64.StdEncoding.EncodeToString(combined)

	return &SDPCryptoAttribute{
		Tag:         tag,
		CryptoSuite: string(m.Profile),
		KeyMethod:   "inline",
		KeyInfo:     keyInfo,
	}
}

// String formats the crypto attribute for SDP
// Format: a=crypto:<tag> <crypto-suite> <key-method>:<key-info> [session-params]
func (c *SDPCryptoAttribute) String() string {
	result := fmt.Sprintf("a=crypto:%d %s %s:%s",
		c.Tag,
		c.CryptoSuite,
		c.KeyMethod,
		c.KeyInfo,
	)

	if len(c.SessionParams) > 0 {
		result += " " + strings.Join(c.SessionParams, " ")
	}

	return result
}

// ParseSDPCryptoAttribute parses an SDP crypto attribute line
// Format: a=crypto:<tag> <crypto-suite> <key-method>:<key-info> [session-params]
func ParseSDPCryptoAttribute(line string) (*SDPCryptoAttribute, error) {
	// Remove "a=crypto:" prefix if present
	line = strings.TrimPrefix(line, "a=crypto:")

	parts := strings.Fields(line)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid crypto attribute format")
	}

	var tag int
	if _, err := fmt.Sscanf(parts[0], "%d", &tag); err != nil {
		return nil, fmt.Errorf("invalid crypto tag: %w", err)
	}

	cryptoSuite := parts[1]

	// Parse key method and info
	keyParts := strings.SplitN(parts[2], ":", 2)
	if len(keyParts) != 2 {
		return nil, fmt.Errorf("invalid key method format")
	}

	attr := &SDPCryptoAttribute{
		Tag:         tag,
		CryptoSuite: cryptoSuite,
		KeyMethod:   keyParts[0],
		KeyInfo:     keyParts[1],
	}

	// Parse session params if present
	if len(parts) > 3 {
		attr.SessionParams = parts[3:]
	}

	return attr, nil
}

// ExtractKeyMaterial extracts key material from a parsed SDP crypto attribute
func (c *SDPCryptoAttribute) ExtractKeyMaterial() (*SRTPKeyMaterial, error) {
	if c.KeyMethod != "inline" {
		return nil, fmt.Errorf("unsupported key method: %s", c.KeyMethod)
	}

	// Handle optional lifetime and MKI in key info
	// Format: base64key|lifetime|MKI:length
	keyInfoParts := strings.Split(c.KeyInfo, "|")
	keyBase64 := keyInfoParts[0]

	combined, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode key info: %w", err)
	}

	profile := SRTPProfile(c.CryptoSuite)

	var keyLen, saltLen int
	switch profile {
	case SRTPProfileAES128CMHMACSHA180, SRTPProfileAES128CMHMACSHA132, SRTPProfileAEADAES128GCM:
		keyLen = 16
		saltLen = 14
	case SRTPProfileAEADAES256GCM:
		keyLen = 32
		saltLen = 12
	default:
		return nil, fmt.Errorf("unknown crypto suite: %s", c.CryptoSuite)
	}

	expectedLen := keyLen + saltLen
	if len(combined) < expectedLen {
		return nil, fmt.Errorf("key material too short: got %d, expected %d", len(combined), expectedLen)
	}

	return &SRTPKeyMaterial{
		MasterKey:  combined[:keyLen],
		MasterSalt: combined[keyLen : keyLen+saltLen],
		Profile:    profile,
	}, nil
}

// SRTPSessionManager manages SRTP contexts for multiple sessions
type SRTPSessionManager struct {
	sessions map[string]*SRTPContext // keyed by call ID
	mu       sync.RWMutex
}

// NewSRTPSessionManager creates a new SRTP session manager
func NewSRTPSessionManager() *SRTPSessionManager {
	return &SRTPSessionManager{
		sessions: make(map[string]*SRTPContext),
	}
}

// GetOrCreate gets an existing SRTP context or creates a new one
func (m *SRTPSessionManager) GetOrCreate(callID string, material *SRTPKeyMaterial) (*SRTPContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx, ok := m.sessions[callID]; ok {
		return ctx, nil
	}

	ctx, err := NewSRTPContext(material)
	if err != nil {
		return nil, err
	}

	m.sessions[callID] = ctx
	return ctx, nil
}

// Get retrieves an existing SRTP context
func (m *SRTPSessionManager) Get(callID string) (*SRTPContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctx, ok := m.sessions[callID]
	return ctx, ok
}

// Remove removes and closes an SRTP context
func (m *SRTPSessionManager) Remove(callID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx, ok := m.sessions[callID]; ok {
		delete(m.sessions, callID)
		return ctx.Close()
	}
	return nil
}

// Close closes all SRTP contexts
func (m *SRTPSessionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ctx := range m.sessions {
		ctx.Close()
	}
	m.sessions = make(map[string]*SRTPContext)
	return nil
}

// ValidSRTPProfiles returns a list of supported SRTP profiles
func ValidSRTPProfiles() []SRTPProfile {
	return []SRTPProfile{
		SRTPProfileAES128CMHMACSHA180,
		SRTPProfileAES128CMHMACSHA132,
		SRTPProfileAEADAES128GCM,
		SRTPProfileAEADAES256GCM,
	}
}

// IsValidSRTPProfile checks if a profile string is valid
func IsValidSRTPProfile(profile string) bool {
	for _, p := range ValidSRTPProfiles() {
		if string(p) == profile {
			return true
		}
	}
	return false
}

// AddCryptoToSDP adds SRTP crypto attributes to an SDP body
// This modifies the media line from RTP/AVP to RTP/SAVP and adds a=crypto line
func AddCryptoToSDP(sdp []byte, material *SRTPKeyMaterial) ([]byte, error) {
	if material == nil {
		return nil, fmt.Errorf("key material required")
	}

	sdpStr := string(sdp)

	// Convert RTP/AVP to RTP/SAVP for SRTP
	sdpStr = strings.Replace(sdpStr, " RTP/AVP ", " RTP/SAVP ", -1)

	// Generate crypto attribute
	cryptoAttr := material.ToSDPCryptoAttribute(1)
	cryptoLine := cryptoAttr.String()

	// Add crypto line after the first m= line
	lines := strings.Split(sdpStr, "\r\n")
	var result []string
	mediaFound := false

	for _, line := range lines {
		result = append(result, line)
		if strings.HasPrefix(line, "m=audio") && !mediaFound {
			// Add crypto line after the media line
			result = append(result, cryptoLine)
			mediaFound = true
		}
	}

	// Ensure proper CRLF line endings
	return []byte(strings.Join(result, "\r\n")), nil
}

// ExtractCryptoFromSDP extracts SRTP key material from an SDP body
func ExtractCryptoFromSDP(sdp []byte) (*SRTPKeyMaterial, error) {
	lines := strings.Split(string(sdp), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, "\r")

		if strings.HasPrefix(line, "a=crypto:") {
			attr, err := ParseSDPCryptoAttribute(line)
			if err != nil {
				continue // Try next crypto line
			}

			material, err := attr.ExtractKeyMaterial()
			if err != nil {
				continue
			}

			return material, nil
		}
	}

	return nil, fmt.Errorf("no valid crypto attribute found in SDP")
}

// HasCryptoInSDP checks if SDP contains crypto attributes
func HasCryptoInSDP(sdp []byte) bool {
	return strings.Contains(string(sdp), "a=crypto:")
}

// IsSAVP checks if SDP uses SAVP (secure) profile
func IsSAVP(sdp []byte) bool {
	return strings.Contains(string(sdp), "RTP/SAVP")
}

// RequiresSRTP checks if the SDP requires SRTP encryption
func RequiresSRTP(sdp []byte) bool {
	return IsSAVP(sdp) || HasCryptoInSDP(sdp)
}

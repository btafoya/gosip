// Package sip provides SIP server functionality using sipgo
package sip

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ZRTPMode defines the ZRTP operation mode
type ZRTPMode string

const (
	// ZRTPModeDisabled means ZRTP is not used
	ZRTPModeDisabled ZRTPMode = "disabled"
	// ZRTPModeOptional means ZRTP is offered but not required
	ZRTPModeOptional ZRTPMode = "optional"
	// ZRTPModeRequired means ZRTP is mandatory for calls
	ZRTPModeRequired ZRTPMode = "required"
)

// ZRTPConfig holds ZRTP-specific configuration
type ZRTPConfig struct {
	// Enabled enables ZRTP support
	Enabled bool
	// Mode defines whether ZRTP is optional or required
	Mode ZRTPMode
	// CacheExpiryDays is how long cached keys are valid
	CacheExpiryDays int
	// ZID is this endpoint's ZRTP identifier (96 bits)
	ZID []byte
}

// ZRTPState represents the state of a ZRTP session
type ZRTPState string

const (
	ZRTPStateIdle        ZRTPState = "idle"
	ZRTPStateDiscovery   ZRTPState = "discovery"
	ZRTPStateKeyExchange ZRTPState = "key_exchange"
	ZRTPStateSecured     ZRTPState = "secured"
	ZRTPStateFailed      ZRTPState = "failed"
)

// ZRTPSession represents a ZRTP session for a call
type ZRTPSession struct {
	CallID    string
	State     ZRTPState
	LocalZID  []byte
	RemoteZID []byte

	// Key material
	S0        []byte // Shared secret from DH exchange
	SRTPKeys  *SRTPKeyMaterial
	SRTPCKeyi []byte // SRTP keys initiator
	SRTPCKeyr []byte // SRTP keys responder
	SRTPSalti []byte // SRTP salt initiator
	SRTPSaltr []byte // SRTP salt responder

	// SAS (Short Authentication String)
	SAS     string // The 4-character SAS for voice verification
	SASType string // "B32" or "B256"

	// Cache data for rs1/rs2
	RS1      []byte // Retained secret 1
	RS2      []byte // Retained secret 2
	IsCached bool   // Whether we have cached keys for this peer

	// Timing
	StartedAt  time.Time
	SecuredAt  time.Time
	ExpiresAt  time.Time
	LastUpdate time.Time

	mu sync.RWMutex
}

// SASVerificationCallback is called when SAS needs to be verified
// Returns true if user confirmed SAS matches, false otherwise
type SASVerificationCallback func(callID, sas string) bool

// ZRTPEventCallback is called for ZRTP state changes
type ZRTPEventCallback func(session *ZRTPSession, event string)

// ZRTPManager manages ZRTP sessions
type ZRTPManager struct {
	config    *ZRTPConfig
	sessions  map[string]*ZRTPSession
	cache     *ZRTPCache
	sasVerify SASVerificationCallback
	onEvent   ZRTPEventCallback
	mu        sync.RWMutex
	logger    *slog.Logger
}

// ZRTPCache stores persistent ZRTP data
type ZRTPCache struct {
	// Maps peer ZID (hex) -> cached data
	entries map[string]*ZRTPCacheEntry
	mu      sync.RWMutex
}

// ZRTPCacheEntry is a cached ZRTP peer
type ZRTPCacheEntry struct {
	PeerZID   []byte
	RS1       []byte
	RS2       []byte
	Verified  bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewZRTPManager creates a new ZRTP manager
func NewZRTPManager(cfg *ZRTPConfig, logger *slog.Logger) (*ZRTPManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("ZRTP config required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Generate ZID if not provided
	zid := cfg.ZID
	if len(zid) != 12 {
		zid = make([]byte, 12)
		if _, err := rand.Read(zid); err != nil {
			return nil, fmt.Errorf("generate ZID: %w", err)
		}
	}
	cfg.ZID = zid

	mgr := &ZRTPManager{
		config:   cfg,
		sessions: make(map[string]*ZRTPSession),
		cache: &ZRTPCache{
			entries: make(map[string]*ZRTPCacheEntry),
		},
		logger: logger,
	}

	logger.Info("ZRTP manager initialized",
		"zid", hex.EncodeToString(zid),
		"mode", cfg.Mode,
	)

	return mgr, nil
}

// SetSASVerificationCallback sets the callback for SAS verification
func (m *ZRTPManager) SetSASVerificationCallback(cb SASVerificationCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sasVerify = cb
}

// SetEventCallback sets the callback for ZRTP events
func (m *ZRTPManager) SetEventCallback(cb ZRTPEventCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEvent = cb
}

// StartSession initiates a ZRTP session for a call
func (m *ZRTPManager) StartSession(callID string) (*ZRTPSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[callID]; exists {
		return nil, fmt.Errorf("ZRTP session already exists for call %s", callID)
	}

	session := &ZRTPSession{
		CallID:     callID,
		State:      ZRTPStateDiscovery,
		LocalZID:   m.config.ZID,
		SASType:    "B32",
		StartedAt:  time.Now(),
		LastUpdate: time.Now(),
	}

	m.sessions[callID] = session
	m.logger.Info("ZRTP session started",
		"call_id", callID,
		"local_zid", hex.EncodeToString(m.config.ZID),
	)

	m.emitEvent(session, "started")
	return session, nil
}

// GetSession retrieves a ZRTP session
func (m *ZRTPManager) GetSession(callID string) (*ZRTPSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[callID]
	return session, ok
}

// EndSession terminates a ZRTP session
func (m *ZRTPManager) EndSession(callID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[callID]
	if !ok {
		return nil // Session doesn't exist, nothing to do
	}

	// Store retained secrets if session was secured
	if session.State == ZRTPStateSecured && session.S0 != nil {
		m.cacheSession(session)
	}

	delete(m.sessions, callID)
	m.logger.Info("ZRTP session ended",
		"call_id", callID,
		"was_secured", session.State == ZRTPStateSecured,
	)

	m.emitEvent(session, "ended")
	return nil
}

// ProcessHello processes a ZRTP Hello message from peer
func (m *ZRTPManager) ProcessHello(callID string, remoteZID []byte) error {
	session, ok := m.GetSession(callID)
	if !ok {
		return fmt.Errorf("no ZRTP session for call %s", callID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.RemoteZID = remoteZID
	session.LastUpdate = time.Now()

	// Check if we have cached keys for this peer
	if entry := m.getCacheEntry(remoteZID); entry != nil {
		session.RS1 = entry.RS1
		session.RS2 = entry.RS2
		session.IsCached = true
		m.logger.Info("Using cached ZRTP keys for peer",
			"call_id", callID,
			"peer_zid", hex.EncodeToString(remoteZID),
		)
	}

	m.emitEvent(session, "hello_received")
	return nil
}

// CompleteKeyExchange marks the key exchange as complete
// In a full implementation, this would be called after DH exchange
func (m *ZRTPManager) CompleteKeyExchange(callID string, s0 []byte) error {
	session, ok := m.GetSession(callID)
	if !ok {
		return fmt.Errorf("no ZRTP session for call %s", callID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.S0 = s0
	session.State = ZRTPStateKeyExchange
	session.LastUpdate = time.Now()

	// Generate SAS from S0
	session.SAS = generateSAS(s0, session.LocalZID, session.RemoteZID)

	m.logger.Info("ZRTP key exchange complete",
		"call_id", callID,
		"sas", session.SAS,
	)

	m.emitEvent(session, "key_exchange_complete")
	return nil
}

// VerifySAS attempts to verify the SAS with the user
func (m *ZRTPManager) VerifySAS(callID string) (bool, error) {
	session, ok := m.GetSession(callID)
	if !ok {
		return false, fmt.Errorf("no ZRTP session for call %s", callID)
	}

	session.mu.RLock()
	sas := session.SAS
	session.mu.RUnlock()

	if sas == "" {
		return false, fmt.Errorf("SAS not yet generated for call %s", callID)
	}

	m.mu.RLock()
	cb := m.sasVerify
	m.mu.RUnlock()

	if cb == nil {
		// No callback set, assume verified (for testing)
		m.logger.Warn("No SAS verification callback set, assuming verified",
			"call_id", callID,
			"sas", sas,
		)
		return true, nil
	}

	verified := cb(callID, sas)

	if verified {
		session.mu.Lock()
		session.State = ZRTPStateSecured
		session.SecuredAt = time.Now()
		session.LastUpdate = time.Now()
		session.mu.Unlock()

		m.logger.Info("ZRTP SAS verified - call is secured",
			"call_id", callID,
			"sas", sas,
		)
		m.emitEvent(session, "secured")
	} else {
		m.logger.Warn("ZRTP SAS verification failed",
			"call_id", callID,
			"sas", sas,
		)
		m.emitEvent(session, "sas_mismatch")
	}

	return verified, nil
}

// GetSAS returns the SAS for a call
func (m *ZRTPManager) GetSAS(callID string) (string, error) {
	session, ok := m.GetSession(callID)
	if !ok {
		return "", fmt.Errorf("no ZRTP session for call %s", callID)
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	if session.SAS == "" {
		return "", fmt.Errorf("SAS not yet generated")
	}

	return session.SAS, nil
}

// IsSecured returns whether a call has completed ZRTP verification
func (m *ZRTPManager) IsSecured(callID string) bool {
	session, ok := m.GetSession(callID)
	if !ok {
		return false
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	return session.State == ZRTPStateSecured
}

// DeriveKeys derives SRTP keys from the ZRTP shared secret
func (m *ZRTPManager) DeriveKeys(callID string) (*SRTPKeyMaterial, error) {
	session, ok := m.GetSession(callID)
	if !ok {
		return nil, fmt.Errorf("no ZRTP session for call %s", callID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.S0 == nil {
		return nil, fmt.Errorf("no shared secret available")
	}

	// Derive SRTP keys using KDF from RFC 6189
	// This is a simplified implementation
	hash := sha256.New()
	hash.Write(session.S0)
	hash.Write(session.LocalZID)
	hash.Write(session.RemoteZID)
	hash.Write([]byte("ZRTP-SRTP"))

	derived := hash.Sum(nil)

	// Split into key and salt (16 byte key, 14 byte salt for AES-CM)
	masterKey := derived[:16]
	masterSalt := derived[16:30]

	session.SRTPKeys = &SRTPKeyMaterial{
		MasterKey:  masterKey,
		MasterSalt: masterSalt,
		Profile:    SRTPProfileAES128CMHMACSHA180,
	}

	m.logger.Debug("ZRTP keys derived for call",
		"call_id", callID,
	)

	return session.SRTPKeys, nil
}

// cacheSession stores retained secrets for a ZRTP session
func (m *ZRTPManager) cacheSession(session *ZRTPSession) {
	if session.RemoteZID == nil || session.S0 == nil {
		return
	}

	// Derive RS1 and RS2 for caching
	// RS1 = HMAC-SHA256(S0, "retained secret 1")
	hash := sha256.New()
	hash.Write(session.S0)
	hash.Write([]byte("retained secret 1"))
	rs1 := hash.Sum(nil)

	hash.Reset()
	hash.Write(session.S0)
	hash.Write([]byte("retained secret 2"))
	rs2 := hash.Sum(nil)

	expiryDays := m.config.CacheExpiryDays
	if expiryDays <= 0 {
		expiryDays = 90
	}

	entry := &ZRTPCacheEntry{
		PeerZID:   session.RemoteZID,
		RS1:       rs1,
		RS2:       rs2,
		Verified:  session.State == ZRTPStateSecured,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(expiryDays) * 24 * time.Hour),
	}

	m.cache.mu.Lock()
	m.cache.entries[hex.EncodeToString(session.RemoteZID)] = entry
	m.cache.mu.Unlock()

	m.logger.Info("ZRTP session cached",
		"peer_zid", hex.EncodeToString(session.RemoteZID),
		"expires", entry.ExpiresAt,
	)
}

// getCacheEntry retrieves cached data for a peer
func (m *ZRTPManager) getCacheEntry(peerZID []byte) *ZRTPCacheEntry {
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()

	entry, ok := m.cache.entries[hex.EncodeToString(peerZID)]
	if !ok {
		return nil
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry
}

// emitEvent sends an event to the callback if set
func (m *ZRTPManager) emitEvent(session *ZRTPSession, event string) {
	m.mu.RLock()
	cb := m.onEvent
	m.mu.RUnlock()

	if cb != nil {
		cb(session, event)
	}
}

// generateSAS generates a Short Authentication String
// Uses base32 encoding for human readability
func generateSAS(s0, localZID, remoteZID []byte) string {
	hash := sha256.New()
	hash.Write(s0)
	hash.Write(localZID)
	hash.Write(remoteZID)
	hash.Write([]byte("SAS"))

	digest := hash.Sum(nil)

	// Take first 4 bytes for a 4-character SAS
	// Use base32 encoding (letters only, no confusing characters)
	sasBytes := digest[:4]

	// Custom base32 alphabet without confusing chars (0/O, 1/l)
	encoder := base32.NewEncoding("ABCDEFGHJKMNPQRSTUVWXYZ23456789=")
	sas := encoder.EncodeToString(sasBytes)

	// Return first 4 characters
	if len(sas) > 4 {
		sas = sas[:4]
	}

	return sas
}

// Close cleans up the ZRTP manager
func (m *ZRTPManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for callID, session := range m.sessions {
		if session.State == ZRTPStateSecured {
			m.cacheSession(session)
		}
		delete(m.sessions, callID)
	}

	m.logger.Info("ZRTP manager closed")
	return nil
}

// GetStats returns ZRTP statistics
func (m *ZRTPManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.cache.mu.RLock()
	cachedPeers := len(m.cache.entries)
	m.cache.mu.RUnlock()

	return map[string]interface{}{
		"active_sessions": len(m.sessions),
		"cached_peers":    cachedPeers,
		"mode":            m.config.Mode,
		"enabled":         m.config.Enabled,
	}
}

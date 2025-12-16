// Package sip provides SIP server functionality using sipgo
package sip

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// Config holds SIP server configuration
type Config struct {
	Port       int
	UserAgent  string
	MOHEnabled bool
	MOHPath    string
	DataDir    string // Data directory for certificates
	TLS        *config.TLSConfig
	SRTP       *config.SRTPConfig
	ZRTP       *config.ZRTPConfig
}

// Server wraps sipgo server with GoSIP-specific functionality
type Server struct {
	cfg       Config
	ua        *sipgo.UserAgent
	srv       *sipgo.Server
	client    *sipgo.Client
	db        *db.DB
	registrar *Registrar
	auth      *Authenticator

	// TLS/Certificate management
	certMgr *CertManager

	// SRTP session management
	srtpMgr *SRTPSessionManager

	// ZRTP session management
	zrtpMgr *ZRTPManager

	// Call control managers
	sessions    *SessionManager
	holdMgr     *HoldManager
	transferMgr *TransferManager
	mohMgr      *MOHManager
	mwiMgr      *MWIManager

	mu          sync.RWMutex
	running     bool
	cancelFn    context.CancelFunc
	activeCalls int // Track number of active calls
}

// NewServer creates a new SIP server
func NewServer(cfg Config, database *db.DB) (*Server, error) {
	// Create user agent
	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent(cfg.UserAgent),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user agent: %w", err)
	}

	// Create server
	srv, err := sipgo.NewServer(ua)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	// Create client for outbound requests
	client, err := sipgo.NewClient(ua)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Initialize session manager
	sessions := NewSessionManager()

	// Initialize MOH manager
	mohMgr := NewMOHManager(MOHConfig{
		Enabled:   cfg.MOHEnabled,
		AudioPath: cfg.MOHPath,
	})

	// Initialize MWI manager
	mwiMgr := NewMWIManager(slog.Default())

	server := &Server{
		cfg:       cfg,
		ua:        ua,
		srv:       srv,
		client:    client,
		db:        database,
		registrar: NewRegistrar(database),
		auth:      NewAuthenticator(database),
		sessions:  sessions,
		mohMgr:    mohMgr,
		mwiMgr:    mwiMgr,
		srtpMgr:   NewSRTPSessionManager(),
	}

	// Validate TLS configuration
	if cfg.TLS != nil && cfg.TLS.DisableUnencrypted && !cfg.TLS.Enabled {
		return nil, fmt.Errorf("cannot disable unencrypted SIP without enabling TLS - set GOSIP_TLS_ENABLED=true")
	}

	// Initialize TLS certificate manager if TLS is enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		certMgr, err := NewCertManager(cfg.TLS, cfg.DataDir)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize TLS certificate manager: %w", err)
		}
		server.certMgr = certMgr
		slog.Info("TLS certificate manager initialized",
			"mode", cfg.TLS.CertMode,
			"port", cfg.TLS.Port,
			"unencrypted_disabled", cfg.TLS.DisableUnencrypted,
		)
	}

	// Log SRTP configuration
	if cfg.SRTP != nil && cfg.SRTP.Enabled {
		slog.Info("SRTP media encryption enabled",
			"profile", cfg.SRTP.Profile,
		)
	}

	// Initialize ZRTP manager if enabled
	if cfg.ZRTP != nil && cfg.ZRTP.Enabled {
		zrtpCfg := &ZRTPConfig{
			Enabled:         cfg.ZRTP.Enabled,
			Mode:            ZRTPMode(cfg.ZRTP.Mode),
			CacheExpiryDays: cfg.ZRTP.CacheExpiryDays,
		}
		zrtpMgr, err := NewZRTPManager(zrtpCfg, slog.Default())
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ZRTP manager: %w", err)
		}
		server.zrtpMgr = zrtpMgr
		slog.Info("ZRTP end-to-end encryption enabled",
			"mode", cfg.ZRTP.Mode,
			"cache_expiry_days", cfg.ZRTP.CacheExpiryDays,
		)
	}

	// Initialize hold manager (needs server reference)
	server.holdMgr = NewHoldManager(server, sessions, mohMgr)

	// Initialize transfer manager (needs server reference)
	server.transferMgr = NewTransferManager(server, sessions, server.holdMgr)

	// Set server reference on MWI manager for sending NOTIFY
	mwiMgr.SetServer(server)

	return server, nil
}

// Start begins listening for SIP messages
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Create cancelable context
	ctx, cancel := context.WithCancel(ctx)
	s.cancelFn = cancel

	// Register handlers
	s.srv.OnRegister(s.handleRegister)
	s.srv.OnInvite(s.handleInvite)
	s.srv.OnAck(s.handleAck)
	s.srv.OnBye(s.handleBye)
	s.srv.OnCancel(s.handleCancel)
	s.srv.OnOptions(s.handleOptions)
	s.srv.OnRefer(s.handleRefer)
	s.srv.OnSubscribe(s.handleSubscribe)

	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Port)

	// Check if unencrypted SIP should be disabled
	disableUnencrypted := s.cfg.TLS != nil && s.cfg.TLS.DisableUnencrypted

	if disableUnencrypted {
		slog.Warn("Unencrypted SIP disabled - UDP/TCP listeners on port 5060 will NOT start",
			"tls_only", true,
			"tls_port", s.cfg.TLS.Port,
		)
	} else {
		// Start UDP listener (unencrypted)
		go func() {
			slog.Info("Starting SIP UDP listener", "addr", addr)
			if err := s.srv.ListenAndServe(ctx, "udp", addr); err != nil {
				slog.Error("SIP UDP listener error", "error", err)
			}
		}()

		// Start TCP listener (unencrypted)
		go func() {
			slog.Info("Starting SIP TCP listener", "addr", addr)
			if err := s.srv.ListenAndServe(ctx, "tcp", addr); err != nil {
				slog.Error("SIP TCP listener error", "error", err)
			}
		}()
	}

	// Start TLS listener if TLS is enabled
	if s.certMgr != nil && s.cfg.TLS != nil {
		tlsConfig := s.certMgr.GetTLSConfig()
		if tlsConfig != nil {
			tlsAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.TLS.Port)
			go func() {
				slog.Info("Starting SIP TLS listener (SIPS)", "addr", tlsAddr)
				if err := s.srv.ListenAndServeTLS(ctx, "tcp", tlsAddr, tlsConfig); err != nil {
					slog.Error("SIP TLS listener error", "error", err)
				}
			}()

			// Start WSS listener if configured
			if s.cfg.TLS.WSSPort > 0 {
				wssAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.TLS.WSSPort)
				go func() {
					slog.Info("Starting SIP WSS listener", "addr", wssAddr)
					if err := s.srv.ListenAndServeTLS(ctx, "wss", wssAddr, tlsConfig); err != nil {
						slog.Error("SIP WSS listener error", "error", err)
					}
				}()
			}
		}
	}

	// Start registration cleanup goroutine
	go s.cleanupExpiredRegistrations(ctx)

	// Start session cleanup goroutine
	go s.cleanupTerminatedSessions(ctx)

	// Start MWI subscription cleanup goroutine
	go s.cleanupExpiredMWISubscriptions(ctx)

	return nil
}

// Stop gracefully shuts down the SIP server
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	if s.cancelFn != nil {
		s.cancelFn()
	}

	// Close certificate manager
	if s.certMgr != nil {
		if err := s.certMgr.Close(); err != nil {
			slog.Error("Failed to close certificate manager", "error", err)
		}
	}

	// Close ZRTP manager
	if s.zrtpMgr != nil {
		if err := s.zrtpMgr.Close(); err != nil {
			slog.Error("Failed to close ZRTP manager", "error", err)
		}
	}

	s.running = false
	slog.Info("SIP server stopped")
}

// cleanupExpiredRegistrations periodically removes expired registrations
func (s *Server) cleanupExpiredRegistrations(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := s.db.Registrations.DeleteExpired(ctx)
			if err != nil {
				slog.Error("Failed to cleanup expired registrations", "error", err)
			} else if count > 0 {
				slog.Info("Cleaned up expired registrations", "count", count)
			}
		}
	}
}

// GetRegistrar returns the registrar for external access
func (s *Server) GetRegistrar() *Registrar {
	return s.registrar
}

// GetActiveRegistrations returns all active registrations
func (s *Server) GetActiveRegistrations(ctx context.Context) ([]RegistrationInfo, error) {
	return s.registrar.GetActiveRegistrations(ctx)
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetActiveCallCount returns the number of currently active calls
func (s *Server) GetActiveCallCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeCalls
}

// incrementCallCount increases the active call count
func (s *Server) incrementCallCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeCalls++
}

// decrementCallCount decreases the active call count
func (s *Server) decrementCallCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeCalls > 0 {
		s.activeCalls--
	}
}

// cleanupTerminatedSessions periodically removes terminated sessions
func (s *Server) cleanupTerminatedSessions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Remove sessions terminated more than 10 minutes ago
			count := s.sessions.Cleanup(ctx, 10*time.Minute)
			if count > 0 {
				slog.Debug("Cleaned up terminated sessions", "count", count)
			}
		}
	}
}

// GetSessions returns the session manager for external access
func (s *Server) GetSessions() *SessionManager {
	return s.sessions
}

// GetHoldManager returns the hold manager for external access
func (s *Server) GetHoldManager() *HoldManager {
	return s.holdMgr
}

// GetTransferManager returns the transfer manager for external access
func (s *Server) GetTransferManager() *TransferManager {
	return s.transferMgr
}

// GetMOHManager returns the MOH manager for external access
func (s *Server) GetMOHManager() *MOHManager {
	return s.mohMgr
}

// GetMWIManager returns the MWI manager for external access
func (s *Server) GetMWIManager() *MWIManager {
	return s.mwiMgr
}

// cleanupExpiredMWISubscriptions periodically removes expired MWI subscriptions
func (s *Server) cleanupExpiredMWISubscriptions(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.mwiMgr != nil {
				s.mwiMgr.CleanupExpired()
			}
		}
	}
}

// SendMWINotify sends an MWI NOTIFY message to a subscriber
// This is called by MWIManager when state changes
func (s *Server) SendMWINotify(ctx context.Context, sub *MWISubscription, body string) error {
	if s.client == nil {
		return fmt.Errorf("SIP client not initialized")
	}

	// Calculate remaining subscription time
	remaining := int(time.Until(sub.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	// Build NOTIFY request per RFC 3265 (SIP Events) and RFC 3842 (MWI)
	// Note: The actual destination is derived from the Contact header
	notifyReq := sip.NewRequest(sip.NOTIFY, sip.Uri{})

	// Add Contact header for routing
	notifyReq.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<%s>", sub.ContactURI)))

	// Set the essential headers
	notifyReq.AppendHeader(sip.NewHeader("Call-ID", sub.CallID))
	notifyReq.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<%s>;tag=%s", sub.FromURI, sub.FromTag)))
	notifyReq.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<%s>;tag=%s", sub.ToURI, sub.ToTag)))
	notifyReq.AppendHeader(sip.NewHeader("CSeq", fmt.Sprintf("%d NOTIFY", sub.CSeq)))

	// Event header per RFC 3265
	notifyReq.AppendHeader(sip.NewHeader("Event", "message-summary"))

	// Subscription-State header per RFC 3265
	subscriptionState := "active"
	if remaining <= 0 {
		subscriptionState = "terminated;reason=timeout"
	} else {
		subscriptionState = fmt.Sprintf("active;expires=%d", remaining)
	}
	notifyReq.AppendHeader(sip.NewHeader("Subscription-State", subscriptionState))

	// Content-Type for MWI body per RFC 3842
	notifyReq.AppendHeader(sip.NewHeader("Content-Type", "application/simple-message-summary"))

	// Set the MWI body
	notifyReq.SetBody([]byte(body))

	slog.Info("Sending MWI NOTIFY",
		slog.String("aor", sub.AOR),
		slog.String("contact", sub.ContactURI),
		slog.String("call_id", sub.CallID),
		slog.Uint64("cseq", uint64(sub.CSeq)),
		slog.Int("expires", remaining),
	)

	// Send the NOTIFY request
	tx, err := s.client.TransactionRequest(ctx, notifyReq)
	if err != nil {
		return fmt.Errorf("failed to send MWI NOTIFY: %w", err)
	}
	defer tx.Terminate()

	// Wait for response
	select {
	case res := <-tx.Responses():
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			slog.Debug("MWI NOTIFY accepted",
				slog.String("aor", sub.AOR),
				slog.Int("status", int(res.StatusCode)),
			)
			return nil
		}
		slog.Warn("MWI NOTIFY rejected",
			slog.String("aor", sub.AOR),
			slog.Int("status", int(res.StatusCode)),
			slog.String("reason", res.Reason),
		)
		return fmt.Errorf("NOTIFY rejected: %d %s", res.StatusCode, res.Reason)
	case <-tx.Done():
		return fmt.Errorf("NOTIFY transaction terminated without response")
	case <-ctx.Done():
		return fmt.Errorf("NOTIFY timeout: %w", ctx.Err())
	}
}

// GetCertManager returns the certificate manager for external access
func (s *Server) GetCertManager() *CertManager {
	return s.certMgr
}

// GetTLSStatus returns the current TLS certificate status
func (s *Server) GetTLSStatus() *CertStatus {
	if s.certMgr == nil {
		return &CertStatus{Enabled: false}
	}
	status := s.certMgr.GetStatus()
	return &status
}

// IsTLSEnabled returns whether TLS is enabled on the server
func (s *Server) IsTLSEnabled() bool {
	return s.certMgr != nil && s.cfg.TLS != nil && s.cfg.TLS.Enabled
}

// ForceRenewal triggers immediate certificate renewal (ACME mode only)
func (s *Server) ForceRenewal(ctx context.Context) error {
	if s.certMgr == nil {
		return fmt.Errorf("TLS not enabled")
	}
	return s.certMgr.ForceRenewal(ctx)
}

// ReloadCertificates reloads certificates from files (manual mode only)
func (s *Server) ReloadCertificates() error {
	if s.certMgr == nil {
		return fmt.Errorf("TLS not enabled")
	}
	return s.certMgr.ReloadCertificates()
}

// IsSRTPEnabled returns whether SRTP is enabled on the server
func (s *Server) IsSRTPEnabled() bool {
	return s.cfg.SRTP != nil && s.cfg.SRTP.Enabled
}

// GetSRTPProfile returns the configured SRTP profile
func (s *Server) GetSRTPProfile() SRTPProfile {
	if s.cfg.SRTP == nil || s.cfg.SRTP.Profile == "" {
		return SRTPProfileAES128CMHMACSHA180
	}
	return SRTPProfile(s.cfg.SRTP.Profile)
}

// GenerateSRTPMaterial generates new SRTP key material for a call
func (s *Server) GenerateSRTPMaterial() (*SRTPKeyMaterial, error) {
	if !s.IsSRTPEnabled() {
		return nil, fmt.Errorf("SRTP not enabled")
	}
	return GenerateKeyMaterial(s.GetSRTPProfile())
}

// SetupSRTPForCall sets up SRTP context for a call
func (s *Server) SetupSRTPForCall(callID string, material *SRTPKeyMaterial) (*SRTPContext, error) {
	return s.srtpMgr.GetOrCreate(callID, material)
}

// GetSRTPForCall retrieves the SRTP context for a call
func (s *Server) GetSRTPForCall(callID string) (*SRTPContext, bool) {
	return s.srtpMgr.Get(callID)
}

// CleanupSRTPForCall removes SRTP context when call ends
func (s *Server) CleanupSRTPForCall(callID string) error {
	return s.srtpMgr.Remove(callID)
}

// GetSRTPManager returns the SRTP session manager for external access
func (s *Server) GetSRTPManager() *SRTPSessionManager {
	return s.srtpMgr
}

// IsZRTPEnabled returns whether ZRTP is enabled on the server
func (s *Server) IsZRTPEnabled() bool {
	return s.zrtpMgr != nil && s.cfg.ZRTP != nil && s.cfg.ZRTP.Enabled
}

// GetZRTPMode returns the configured ZRTP mode
func (s *Server) GetZRTPMode() string {
	if s.cfg.ZRTP == nil {
		return "disabled"
	}
	return s.cfg.ZRTP.Mode
}

// GetZRTPManager returns the ZRTP manager for external access
func (s *Server) GetZRTPManager() *ZRTPManager {
	return s.zrtpMgr
}

// StartZRTPSession initiates a ZRTP session for a call
func (s *Server) StartZRTPSession(callID string) (*ZRTPSession, error) {
	if s.zrtpMgr == nil {
		return nil, fmt.Errorf("ZRTP not enabled")
	}
	return s.zrtpMgr.StartSession(callID)
}

// GetZRTPSession retrieves the ZRTP session for a call
func (s *Server) GetZRTPSession(callID string) (*ZRTPSession, bool) {
	if s.zrtpMgr == nil {
		return nil, false
	}
	return s.zrtpMgr.GetSession(callID)
}

// EndZRTPSession terminates a ZRTP session for a call
func (s *Server) EndZRTPSession(callID string) error {
	if s.zrtpMgr == nil {
		return nil
	}
	return s.zrtpMgr.EndSession(callID)
}

// GetZRTPSAS returns the Short Authentication String for a call
func (s *Server) GetZRTPSAS(callID string) (string, error) {
	if s.zrtpMgr == nil {
		return "", fmt.Errorf("ZRTP not enabled")
	}
	return s.zrtpMgr.GetSAS(callID)
}

// IsCallZRTPSecured returns whether a call has completed ZRTP verification
func (s *Server) IsCallZRTPSecured(callID string) bool {
	if s.zrtpMgr == nil {
		return false
	}
	return s.zrtpMgr.IsSecured(callID)
}

// DeriveZRTPKeys derives SRTP keys from ZRTP shared secret
func (s *Server) DeriveZRTPKeys(callID string) (*SRTPKeyMaterial, error) {
	if s.zrtpMgr == nil {
		return nil, fmt.Errorf("ZRTP not enabled")
	}
	return s.zrtpMgr.DeriveKeys(callID)
}

// SetZRTPSASCallback sets the callback for SAS verification
func (s *Server) SetZRTPSASCallback(cb SASVerificationCallback) {
	if s.zrtpMgr != nil {
		s.zrtpMgr.SetSASVerificationCallback(cb)
	}
}

// SetZRTPEventCallback sets the callback for ZRTP events
func (s *Server) SetZRTPEventCallback(cb ZRTPEventCallback) {
	if s.zrtpMgr != nil {
		s.zrtpMgr.SetEventCallback(cb)
	}
}

// GetZRTPStats returns ZRTP statistics
func (s *Server) GetZRTPStats() map[string]interface{} {
	if s.zrtpMgr == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	return s.zrtpMgr.GetStats()
}

// GetEncryptionStatus returns a summary of all encryption configurations
func (s *Server) GetEncryptionStatus() map[string]interface{} {
	status := map[string]interface{}{
		"tls": map[string]interface{}{
			"enabled":              s.IsTLSEnabled(),
			"unencrypted_disabled": s.cfg.TLS != nil && s.cfg.TLS.DisableUnencrypted,
		},
		"srtp": map[string]interface{}{
			"enabled": s.IsSRTPEnabled(),
			"profile": s.GetSRTPProfile(),
		},
		"zrtp": s.GetZRTPStats(),
	}

	if s.IsTLSEnabled() {
		tlsStatus := s.GetTLSStatus()
		status["tls"].(map[string]interface{})["cert_mode"] = tlsStatus.CertMode
		status["tls"].(map[string]interface{})["cert_valid"] = tlsStatus.Valid
		status["tls"].(map[string]interface{})["cert_expires"] = tlsStatus.CertExpiry
	}

	return status
}

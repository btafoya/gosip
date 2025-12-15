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

	// Start UDP listener
	go func() {
		slog.Info("Starting SIP UDP listener", "addr", addr)
		if err := s.srv.ListenAndServe(ctx, "udp", addr); err != nil {
			slog.Error("SIP UDP listener error", "error", err)
		}
	}()

	// Start TCP listener
	go func() {
		slog.Info("Starting SIP TCP listener", "addr", addr)
		if err := s.srv.ListenAndServe(ctx, "tcp", addr); err != nil {
			slog.Error("SIP TCP listener error", "error", err)
		}
	}()

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

	// Build NOTIFY request
	// Note: In production, you would use sipgo.NewRequest and set proper headers
	// For now, we'll log the notification attempt
	slog.Info("Sending MWI NOTIFY",
		slog.String("aor", sub.AOR),
		slog.String("contact", sub.ContactURI),
		slog.String("call_id", sub.CallID),
		slog.Uint64("cseq", uint64(sub.CSeq)),
	)

	// TODO: Implement actual SIP NOTIFY sending using sipgo
	// This requires building the NOTIFY request with:
	// - Event: message-summary
	// - Subscription-State: active;expires=<remaining>
	// - Content-Type: application/simple-message-summary
	// - Body: message-summary content

	return nil
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

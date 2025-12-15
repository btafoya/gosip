// Package sip provides SIP server functionality using sipgo
package sip

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/emiago/sipgo"
)

// Config holds SIP server configuration
type Config struct {
	Port       int
	UserAgent  string
	MOHEnabled bool
	MOHPath    string
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

	// Call control managers
	sessions    *SessionManager
	holdMgr     *HoldManager
	transferMgr *TransferManager
	mohMgr      *MOHManager

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
	}

	// Initialize hold manager (needs server reference)
	server.holdMgr = NewHoldManager(server, sessions, mohMgr)

	// Initialize transfer manager (needs server reference)
	server.transferMgr = NewTransferManager(server, sessions, server.holdMgr)

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

	// Start registration cleanup goroutine
	go s.cleanupExpiredRegistrations(ctx)

	// Start session cleanup goroutine
	go s.cleanupTerminatedSessions(ctx)

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

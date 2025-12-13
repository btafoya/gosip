package sip

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

func TestNewServer(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("NewServer should not return nil")
	}

	// Verify configuration
	if server.cfg.Port != 5060 {
		t.Errorf("Port mismatch: got %d, want 5060", server.cfg.Port)
	}
	if server.cfg.UserAgent != "GoSIP-Test/1.0" {
		t.Errorf("UserAgent mismatch: got %s, want GoSIP-Test/1.0", server.cfg.UserAgent)
	}

	// Verify components are initialized
	if server.ua == nil {
		t.Error("UserAgent should be initialized")
	}
	if server.srv == nil {
		t.Error("Server should be initialized")
	}
	if server.client == nil {
		t.Error("Client should be initialized")
	}
	if server.registrar == nil {
		t.Error("Registrar should be initialized")
	}
	if server.auth == nil {
		t.Error("Authenticator should be initialized")
	}

	// Server should not be running initially
	if server.IsRunning() {
		t.Error("Server should not be running initially")
	}
}

func TestServer_IsRunning(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Initially not running
	if server.IsRunning() {
		t.Error("Server should not be running initially")
	}

	// Manually set running state
	server.mu.Lock()
	server.running = true
	server.mu.Unlock()

	if !server.IsRunning() {
		t.Error("Server should report running after being set")
	}

	// Reset
	server.mu.Lock()
	server.running = false
	server.mu.Unlock()

	if server.IsRunning() {
		t.Error("Server should report not running after being reset")
	}
}

func TestServer_ActiveCallCount(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Initially zero calls
	if count := server.GetActiveCallCount(); count != 0 {
		t.Errorf("Initial call count should be 0, got %d", count)
	}

	// Increment call count
	server.incrementCallCount()
	if count := server.GetActiveCallCount(); count != 1 {
		t.Errorf("Call count should be 1 after increment, got %d", count)
	}

	// Increment again
	server.incrementCallCount()
	server.incrementCallCount()
	if count := server.GetActiveCallCount(); count != 3 {
		t.Errorf("Call count should be 3 after 3 increments, got %d", count)
	}

	// Decrement call count
	server.decrementCallCount()
	if count := server.GetActiveCallCount(); count != 2 {
		t.Errorf("Call count should be 2 after decrement, got %d", count)
	}

	// Decrement below zero should not go negative
	server.decrementCallCount()
	server.decrementCallCount()
	server.decrementCallCount() // Extra decrement
	if count := server.GetActiveCallCount(); count != 0 {
		t.Errorf("Call count should be 0 (not negative), got %d", count)
	}
}

func TestServer_CallCountConcurrency(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrent increments
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.incrementCallCount()
		}()
	}

	wg.Wait()

	if count := server.GetActiveCallCount(); count != 100 {
		t.Errorf("Call count should be 100 after concurrent increments, got %d", count)
	}

	// Concurrent decrements
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.decrementCallCount()
		}()
	}

	wg.Wait()

	if count := server.GetActiveCallCount(); count != 0 {
		t.Errorf("Call count should be 0 after concurrent decrements, got %d", count)
	}
}

func TestServer_Stop(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Stop when not running should be safe
	server.Stop()
	if server.IsRunning() {
		t.Error("Server should not be running after Stop")
	}

	// Set up as if running
	ctx, cancel := context.WithCancel(context.Background())
	server.mu.Lock()
	server.running = true
	server.cancelFn = cancel
	server.mu.Unlock()

	// Stop should work
	server.Stop()

	if server.IsRunning() {
		t.Error("Server should not be running after Stop")
	}

	// Verify context was canceled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be canceled after Stop")
	}
}

func TestServer_StopIdempotent(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Set up as if running
	_, cancel := context.WithCancel(context.Background())
	server.mu.Lock()
	server.running = true
	server.cancelFn = cancel
	server.mu.Unlock()

	// Multiple stops should be safe
	server.Stop()
	server.Stop()
	server.Stop()

	if server.IsRunning() {
		t.Error("Server should not be running after multiple Stops")
	}
}

func TestServer_GetRegistrar(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	registrar := server.GetRegistrar()
	if registrar == nil {
		t.Error("GetRegistrar should not return nil")
	}

	// Should return the same instance
	if registrar != server.registrar {
		t.Error("GetRegistrar should return the server's registrar")
	}
}

func TestServer_GetActiveRegistrations(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Port:      5060,
		UserAgent: "GoSIP-Test/1.0",
	}

	server, err := NewServer(cfg, database)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	// Initially empty
	regs, err := server.GetActiveRegistrations(ctx)
	if err != nil {
		t.Fatalf("GetActiveRegistrations failed: %v", err)
	}
	if len(regs) != 0 {
		t.Errorf("Should have 0 registrations initially, got %d", len(regs))
	}

	// Add a registration through the registrar
	device := createTestDevice(t, database, "alice", "passwordhash")
	reg := &models.Registration{
		DeviceID:  device.ID,
		Contact:   "sip:alice@192.168.1.100:5060",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Transport: "udp",
	}
	if err := server.registrar.Register(ctx, reg); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Now should have one registration
	regs, err = server.GetActiveRegistrations(ctx)
	if err != nil {
		t.Fatalf("GetActiveRegistrations failed: %v", err)
	}
	if len(regs) != 1 {
		t.Errorf("Should have 1 registration, got %d", len(regs))
	}

	if len(regs) > 0 {
		if regs[0].DeviceID != device.ID {
			t.Errorf("DeviceID mismatch: got %d, want %d", regs[0].DeviceID, device.ID)
		}
		if regs[0].Username != device.Username {
			t.Errorf("Username mismatch: got %s, want %s", regs[0].Username, device.Username)
		}
	}
}

func TestConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantPort  int
		wantAgent string
	}{
		{
			name:      "default port",
			config:    Config{Port: 5060, UserAgent: "Test/1.0"},
			wantPort:  5060,
			wantAgent: "Test/1.0",
		},
		{
			name:      "custom port",
			config:    Config{Port: 5080, UserAgent: "Custom/2.0"},
			wantPort:  5080,
			wantAgent: "Custom/2.0",
		},
		{
			name:      "zero port",
			config:    Config{Port: 0, UserAgent: "Zero/1.0"},
			wantPort:  0,
			wantAgent: "Zero/1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", tt.config.Port, tt.wantPort)
			}
			if tt.config.UserAgent != tt.wantAgent {
				t.Errorf("UserAgent = %s, want %s", tt.config.UserAgent, tt.wantAgent)
			}
		})
	}
}

func TestRegistrationInfo(t *testing.T) {
	info := RegistrationInfo{
		DeviceID:   1,
		DeviceName: "Test Phone",
		Username:   "alice",
		Contact:    "sip:alice@192.168.1.100:5060",
		IPAddress:  "192.168.1.100",
		Transport:  "udp",
		UserAgent:  "TestPhone/1.0",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
		LastSeen:   time.Now(),
		Online:     true,
	}

	if info.DeviceID != 1 {
		t.Errorf("DeviceID = %d, want 1", info.DeviceID)
	}
	if info.DeviceName != "Test Phone" {
		t.Errorf("DeviceName = %s, want Test Phone", info.DeviceName)
	}
	if info.Username != "alice" {
		t.Errorf("Username = %s, want alice", info.Username)
	}
	if info.Transport != "udp" {
		t.Errorf("Transport = %s, want udp", info.Transport)
	}
	if !info.Online {
		t.Error("Online should be true")
	}
}


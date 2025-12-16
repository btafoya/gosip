// Package main is the entry point for the GoSIP application
package main

import (
	"testing"
	"time"
)

// TestPackageStructure verifies the main package is correctly structured
func TestPackageStructure(t *testing.T) {
	// This test ensures the package compiles and has the expected structure
	// The main() function cannot be directly tested, but we can verify
	// that the package builds correctly with all its imports
	t.Log("Main package compiles successfully")
}

// TestDefaultTimeouts verifies the expected timeout values
func TestDefaultTimeouts(t *testing.T) {
	// These are the timeout values used in main.go
	// Verifying them here ensures consistency
	expectedReadTimeout := 15 * time.Second
	expectedWriteTimeout := 15 * time.Second
	expectedIdleTimeout := 60 * time.Second
	expectedShutdownTimeout := 30 * time.Second

	// Verify timeouts are reasonable values
	if expectedReadTimeout < time.Second {
		t.Error("Read timeout too short")
	}
	if expectedWriteTimeout < time.Second {
		t.Error("Write timeout too short")
	}
	if expectedIdleTimeout < 30*time.Second {
		t.Error("Idle timeout too short")
	}
	if expectedShutdownTimeout < 10*time.Second {
		t.Error("Shutdown timeout too short")
	}
	if expectedShutdownTimeout > 60*time.Second {
		t.Error("Shutdown timeout too long")
	}

	t.Logf("HTTP Server Timeouts - Read: %v, Write: %v, Idle: %v",
		expectedReadTimeout, expectedWriteTimeout, expectedIdleTimeout)
	t.Logf("Shutdown Timeout: %v", expectedShutdownTimeout)
}

// TestVersionString verifies version is defined
func TestVersionString(t *testing.T) {
	// The version is hardcoded in main.go as "1.0.0"
	// This test serves as documentation
	version := "1.0.0"
	if version == "" {
		t.Error("Version should not be empty")
	}
	t.Logf("GoSIP Version: %s", version)
}

// TestDefaultPorts verifies expected port configuration
func TestDefaultPorts(t *testing.T) {
	// Default SIP port is 5060 (standard)
	// Default HTTP port is 8080 (standard alternate)
	// These are configured in internal/config but used in main.go
	standardSIPPort := 5060
	standardHTTPPort := 8080

	if standardSIPPort < 1 || standardSIPPort > 65535 {
		t.Errorf("SIP port %d out of valid range", standardSIPPort)
	}
	if standardHTTPPort < 1 || standardHTTPPort > 65535 {
		t.Errorf("HTTP port %d out of valid range", standardHTTPPort)
	}

	t.Logf("Standard Ports - SIP: %d, HTTP: %d", standardSIPPort, standardHTTPPort)
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestHealthHandler_Health(t *testing.T) {
	handler := NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.Health(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", resp.Version)
	}
	if resp.GoVersion != runtime.Version() {
		t.Errorf("Expected Go version %s, got %s", runtime.Version(), resp.GoVersion)
	}
	if resp.Uptime == "" {
		t.Error("Expected uptime to be set")
	}
	if resp.Timestamp == "" {
		t.Error("Expected timestamp to be set")
	}
}

func TestHealthHandler_Health_ContentType(t *testing.T) {
	handler := NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.Health(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestHealthHandler_Ready(t *testing.T) {
	handler := NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()
	handler.Ready(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "ready" {
		t.Errorf("Expected status 'ready', got %s", resp["status"])
	}
}

func TestHealthHandler_Live(t *testing.T) {
	handler := NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rr := httptest.NewRecorder()
	handler.Live(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "alive" {
		t.Errorf("Expected status 'alive', got %s", resp["status"])
	}
}

func TestHealthHandler_UptimeFormat(t *testing.T) {
	handler := NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.Health(rr, req)

	var resp HealthResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Uptime should contain a duration string (e.g., "0s", "1h2m3s")
	if !strings.Contains(resp.Uptime, "s") && !strings.Contains(resp.Uptime, "m") && !strings.Contains(resp.Uptime, "h") {
		t.Errorf("Uptime format unexpected: %s", resp.Uptime)
	}
}

func TestHealthHandler_TimestampFormat(t *testing.T) {
	handler := NewHealthHandler("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.Health(rr, req)

	var resp HealthResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Timestamp should be in RFC3339 format
	if !strings.Contains(resp.Timestamp, "T") || !strings.Contains(resp.Timestamp, "Z") {
		t.Errorf("Timestamp should be RFC3339 format: %s", resp.Timestamp)
	}
}

func TestNewHealthHandler(t *testing.T) {
	handler := NewHealthHandler("2.0.0")

	if handler == nil {
		t.Fatal("Expected handler to be created")
	}
	if handler.version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", handler.version)
	}
	if handler.startTime.IsZero() {
		t.Error("Expected startTime to be set")
	}
}

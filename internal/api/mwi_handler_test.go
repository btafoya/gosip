package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMWIHandler_GetStatus_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewMWIHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/mwi/status", nil)
	rr := httptest.NewRecorder()
	handler.GetStatus(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Response should indicate MWI is disabled when SIP is nil
	body := rr.Body.String()
	if !strings.Contains(body, `"enabled":false`) {
		t.Errorf("Expected enabled=false, got: %s", body)
	}
}

func TestMWIHandler_TriggerNotification_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewMWIHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/mwi/notify", nil)
	rr := httptest.NewRecorder()
	handler.TriggerNotification(rr, req)

	assertStatus(t, rr, http.StatusServiceUnavailable)

	body := rr.Body.String()
	if !strings.Contains(body, "SIP server not available") {
		t.Errorf("Expected SIP server not available error, got: %s", body)
	}
}

func TestMWIHandler_GetStatus_EmptyStates(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewMWIHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/mwi/status", nil)
	rr := httptest.NewRecorder()
	handler.GetStatus(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Response should have empty arrays for states and subscriptions
	body := rr.Body.String()
	if !strings.Contains(body, `"states":[]`) {
		t.Errorf("Expected empty states array, got: %s", body)
	}
	if !strings.Contains(body, `"subscriptions":[]`) {
		t.Errorf("Expected empty subscriptions array, got: %s", body)
	}
}

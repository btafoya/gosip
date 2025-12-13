package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestDIDHandler_List(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	// Create test DIDs
	createTestDID(t, setup.DB, "+15551234567")
	createTestDID(t, setup.DB, "+15559876543")

	req := httptest.NewRequest(http.MethodGet, "/api/dids", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp []*DIDResponse
	decodeResponse(t, rr, &resp)

	if len(resp) != 2 {
		t.Errorf("Expected 2 DIDs, got %d", len(resp))
	}
}

func TestDIDHandler_Create(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	reqBody := CreateDIDRequest{
		Number:       "+15551234567",
		TwilioSID:    "PN123456789",
		Name:         "Main Line",
		SMSEnabled:   true,
		VoiceEnabled: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/dids", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp DIDResponse
	decodeResponse(t, rr, &resp)

	if resp.Number != "+15551234567" {
		t.Errorf("Expected number +15551234567, got %s", resp.Number)
	}
	if resp.TwilioSID != "PN123456789" {
		t.Errorf("Expected TwilioSID PN123456789, got %s", resp.TwilioSID)
	}
	if !resp.SMSEnabled {
		t.Error("Expected SMSEnabled to be true")
	}
	if !resp.VoiceEnabled {
		t.Error("Expected VoiceEnabled to be true")
	}
}

func TestDIDHandler_Create_MissingNumber(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	reqBody := CreateDIDRequest{
		Name:       "Missing Number",
		SMSEnabled: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/dids", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertErrorCode(t, rr, ErrCodeValidation)
}

func TestDIDHandler_Create_InvalidJSON(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/dids", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestDIDHandler_Get(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	did := createTestDID(t, setup.DB, "+15551234567")

	req := httptest.NewRequest(http.MethodGet, "/api/dids/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp DIDResponse
	decodeResponse(t, rr, &resp)

	if resp.Number != did.Number {
		t.Errorf("Expected number %s, got %s", did.Number, resp.Number)
	}
}

func TestDIDHandler_Get_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/dids/9999", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertErrorCode(t, rr, ErrCodeNotFound)
}

func TestDIDHandler_Get_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/dids/invalid", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestDIDHandler_Update(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	createTestDID(t, setup.DB, "+15551234567")

	smsEnabled := false
	reqBody := UpdateDIDRequest{
		Name:       "Updated Name",
		SMSEnabled: &smsEnabled,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/dids/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp DIDResponse
	decodeResponse(t, rr, &resp)

	if resp.Name != "Updated Name" {
		t.Errorf("Expected name Updated Name, got %s", resp.Name)
	}
	if resp.SMSEnabled {
		t.Error("Expected SMSEnabled to be false")
	}
}

func TestDIDHandler_Update_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	reqBody := UpdateDIDRequest{
		Name: "Updated Name",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/dids/9999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestDIDHandler_Delete(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	createTestDID(t, setup.DB, "+15551234567")

	req := httptest.NewRequest(http.MethodDelete, "/api/dids/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify deleted
	_, err := setup.DB.DIDs.GetByID(context.Background(), 1)
	if err == nil {
		t.Error("Expected DID to be deleted")
	}
}

func TestDIDHandler_Delete_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewDIDHandler(deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/dids/invalid", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestDIDResponse_Format(t *testing.T) {
	did := &models.DID{
		ID:           1,
		Number:       "+15551234567",
		TwilioSID:    "PN123",
		Name:         "Test DID",
		SMSEnabled:   true,
		VoiceEnabled: false,
	}

	resp := toDIDResponse(did)

	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %d", resp.ID)
	}
	if resp.Number != "+15551234567" {
		t.Errorf("Expected number +15551234567, got %s", resp.Number)
	}
	if resp.TwilioSID != "PN123" {
		t.Errorf("Expected TwilioSID PN123, got %s", resp.TwilioSID)
	}
	if resp.Name != "Test DID" {
		t.Errorf("Expected name Test DID, got %s", resp.Name)
	}
	if !resp.SMSEnabled {
		t.Error("Expected SMSEnabled to be true")
	}
	if resp.VoiceEnabled {
		t.Error("Expected VoiceEnabled to be false")
	}
}

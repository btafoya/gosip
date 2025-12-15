package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallHandler_ListActiveCalls_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/calls", nil)
	rr := httptest.NewRecorder()
	handler.ListActiveCalls(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp struct {
		Data  []ActiveCallResponse `json:"data"`
		Count int                  `json:"count"`
	}
	decodeResponse(t, rr, &resp)

	if len(resp.Data) != 0 {
		t.Errorf("Expected 0 calls with no SIP, got %d", len(resp.Data))
	}
	if resp.Count != 0 {
		t.Errorf("Expected count 0 with no SIP, got %d", resp.Count)
	}
}

func TestCallHandler_GetCall_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/calls/test-call-id", nil)
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.GetCall(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertErrorCode(t, rr, "NOT_FOUND")
}

func TestCallHandler_HoldCall_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	reqBody := HoldRequest{Hold: true}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/calls/test-call-id/hold", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.HoldCall(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestCallHandler_HoldCall_InvalidBody(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/calls/test-call-id/hold", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.HoldCall(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestCallHandler_TransferCall_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	reqBody := TransferRequest{Type: "blind", Target: "+15551234567"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/calls/test-call-id/transfer", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.TransferCall(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestCallHandler_TransferCall_InvalidType(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	reqBody := TransferRequest{Type: "invalid", Target: "+15551234567"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/calls/test-call-id/transfer", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.TransferCall(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestCallHandler_TransferCall_BlindMissingTarget(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	reqBody := TransferRequest{Type: "blind", Target: ""}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/calls/test-call-id/transfer", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.TransferCall(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestCallHandler_TransferCall_AttendedMissingConsultID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	reqBody := TransferRequest{Type: "attended", ConsultID: ""}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/calls/test-call-id/transfer", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.TransferCall(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestCallHandler_TransferCall_InvalidBody(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/calls/test-call-id/transfer", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.TransferCall(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestCallHandler_CancelTransferCall_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/calls/test-call-id/transfer", nil)
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.CancelTransferCall(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestCallHandler_HangupCall_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/calls/test-call-id", nil)
	req = withURLParams(req, map[string]string{"callID": "test-call-id"})

	rr := httptest.NewRecorder()
	handler.HangupCall(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestCallHandler_GetMOHStatus_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/calls/moh", nil)
	rr := httptest.NewRecorder()
	handler.GetMOHStatus(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp struct {
		Data MOHStatusResponse `json:"data"`
	}
	decodeResponse(t, rr, &resp)

	if resp.Data.Enabled {
		t.Error("Expected MOH disabled with no SIP")
	}
	if resp.Data.ActiveCount != 0 {
		t.Errorf("Expected 0 active MOH, got %d", resp.Data.ActiveCount)
	}
}

func TestCallHandler_UpdateMOH_NoSIP(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	enabled := true
	reqBody := UpdateMOHRequest{Enabled: &enabled}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/calls/moh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateMOH(rr, req)

	assertStatus(t, rr, http.StatusServiceUnavailable)
}

func TestCallHandler_UpdateMOH_InvalidBody(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewCallHandler(deps)

	req := httptest.NewRequest(http.MethodPut, "/api/calls/moh", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UpdateMOH(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

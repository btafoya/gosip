package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestCDRHandler_List(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DID and CDRs
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestCDR(t, setup.DB, did.ID, "inbound", "+15559876543", did.Number)
	createTestCDR(t, setup.DB, did.ID, "outbound", did.Number, "+15559876543")

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 2 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 2 CDRs, got %d", total)
	}
}

func TestCDRHandler_List_FilterByDirection(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DID and CDRs
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestCDR(t, setup.DB, did.ID, "inbound", "+15559876543", did.Number)
	createTestCDR(t, setup.DB, did.ID, "outbound", did.Number, "+15559876543")

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs?direction=inbound", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 inbound CDR, got %d", total)
	}
}

func TestCDRHandler_List_FilterByDID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DIDs and CDRs
	did1 := createTestDID(t, setup.DB, "+15551234567")
	did2 := createTestDID(t, setup.DB, "+15559999999")
	createTestCDR(t, setup.DB, did1.ID, "inbound", "+15559876543", did1.Number)
	createTestCDR(t, setup.DB, did2.ID, "inbound", "+15559876543", did2.Number)

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs?did_id=1", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 CDR for DID 1, got %d", total)
	}
}

func TestCDRHandler_List_FilterByDateRange(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DID and CDR
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestCDR(t, setup.DB, did.ID, "inbound", "+15559876543", did.Number)

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs?start_date=2020-01-01&end_date=2030-12-31", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestCDRHandler_List_Pagination(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DID and multiple CDRs
	did := createTestDID(t, setup.DB, "+15551234567")
	for i := 0; i < 5; i++ {
		createTestCDR(t, setup.DB, did.ID, "inbound", "+15559876543", did.Number)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs?limit=2&offset=0", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 5 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected total 5, got %d", total)
	}
	if resp.Pagination == nil || resp.Pagination.Limit != 2 {
		limit := 0
		if resp.Pagination != nil {
			limit = resp.Pagination.Limit
		}
		t.Errorf("Expected limit 2, got %d", limit)
	}
}

func TestCDRHandler_Get(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DID and CDR
	did := createTestDID(t, setup.DB, "+15551234567")
	cdr := createTestCDR(t, setup.DB, did.ID, "inbound", "+15559876543", did.Number)

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp map[string]interface{}
	decodeResponse(t, rr, &resp)

	if resp["from_number"] != cdr.FromNumber {
		t.Errorf("Expected from_number %s, got %v", cdr.FromNumber, resp["from_number"])
	}
}

func TestCDRHandler_Get_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs/9999", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertErrorCode(t, rr, ErrCodeNotFound)
}

func TestCDRHandler_Get_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs/invalid", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestCDRHandler_GetStats(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DID and CDRs
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestCDR(t, setup.DB, did.ID, "inbound", "+15559876543", did.Number)
	createTestCDR(t, setup.DB, did.ID, "outbound", did.Number, "+15559876543")

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs/stats", nil)
	rr := httptest.NewRecorder()
	handler.GetStats(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp map[string]interface{}
	decodeResponse(t, rr, &resp)

	if resp["period"] == nil {
		t.Error("Expected period in response")
	}
	if resp["by_disposition"] == nil {
		t.Error("Expected by_disposition in response")
	}
}

func TestCDRHandler_GetStats_WithDateRange(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs/stats?start_date=2020-01-01&end_date=2030-12-31", nil)
	rr := httptest.NewRecorder()
	handler.GetStats(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp map[string]interface{}
	decodeResponse(t, rr, &resp)

	period := resp["period"].(map[string]interface{})
	if period["start"] != "2020-01-01" {
		t.Errorf("Expected start date 2020-01-01, got %v", period["start"])
	}
}

func TestCDRHandler_List_MaxPageSize(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Request with limit exceeding max
	req := httptest.NewRequest(http.MethodGet, "/api/cdrs?limit=500", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Limit should be capped at MaxPageSize (100)
	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination != nil && resp.Pagination.Limit > 100 {
		t.Errorf("Expected limit to be capped at 100, got %d", resp.Pagination.Limit)
	}
}

func TestCDRHandler_List_FilterByDisposition(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewCDRHandler(deps)

	// Create test DID and CDRs
	did := createTestDID(t, setup.DB, "+15551234567")
	createTestCDR(t, setup.DB, did.ID, "inbound", "+15559876543", did.Number)

	// Create a CDR with different disposition
	didID := did.ID
	cdr := &models.CDR{
		DIDID:       &didID,
		Direction:   "inbound",
		FromNumber:  "+15558888888",
		ToNumber:    did.Number,
		Duration:    0,
		Disposition: "no_answer",
	}
	setup.DB.CDRs.Create(context.Background(), cdr)

	req := httptest.NewRequest(http.MethodGet, "/api/cdrs?disposition=answered", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp ListResponse
	decodeResponse(t, rr, &resp)

	if resp.Pagination == nil || resp.Pagination.Total != 1 {
		total := 0
		if resp.Pagination != nil {
			total = resp.Pagination.Total
		}
		t.Errorf("Expected 1 answered CDR, got %d", total)
	}
}

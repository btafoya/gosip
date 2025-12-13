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

func createTestRoute(t *testing.T, setup *testSetup, name string, didID *int64) *models.Route {
	t.Helper()

	route := &models.Route{
		DIDID:         didID,
		Priority:      1,
		Name:          name,
		ConditionType: "default",
		ActionType:    "ring",
		Enabled:       true,
	}

	if err := setup.DB.Routes.Create(context.Background(), route); err != nil {
		t.Fatalf("Failed to create test route: %v", err)
	}

	return route
}

func TestRouteHandler_List(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	// Create test routes
	createTestRoute(t, setup, "Route 1", nil)
	createTestRoute(t, setup, "Route 2", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/routes", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp []*RouteResponse
	decodeResponse(t, rr, &resp)

	if len(resp) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(resp))
	}
}

func TestRouteHandler_List_FilterByDID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	// Create DID
	did := createTestDID(t, setup.DB, "+15551234567")

	// Create routes
	createTestRoute(t, setup, "Route for DID", &did.ID)
	createTestRoute(t, setup, "Route without DID", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/routes?did_id=1", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp []*RouteResponse
	decodeResponse(t, rr, &resp)

	if len(resp) != 1 {
		t.Errorf("Expected 1 route for DID, got %d", len(resp))
	}
}

func TestRouteHandler_Create(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	reqBody := CreateRouteRequest{
		Name:          "Business Hours",
		ConditionType: "time",
		ActionType:    "ring",
		Enabled:       true,
		Priority:      1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/routes", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp RouteResponse
	decodeResponse(t, rr, &resp)

	if resp.Name != "Business Hours" {
		t.Errorf("Expected name 'Business Hours', got %s", resp.Name)
	}
	if resp.ConditionType != "time" {
		t.Errorf("Expected condition type 'time', got %s", resp.ConditionType)
	}
}

func TestRouteHandler_Create_ValidationError(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	tests := []struct {
		name    string
		reqBody CreateRouteRequest
	}{
		{
			name: "Missing name",
			reqBody: CreateRouteRequest{
				ConditionType: "default",
				ActionType:    "ring",
			},
		},
		{
			name: "Invalid condition type",
			reqBody: CreateRouteRequest{
				Name:          "Test",
				ConditionType: "invalid",
				ActionType:    "ring",
			},
		},
		{
			name: "Invalid action type",
			reqBody: CreateRouteRequest{
				Name:          "Test",
				ConditionType: "default",
				ActionType:    "invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/routes", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Create(rr, req)

			assertStatus(t, rr, http.StatusBadRequest)
			assertErrorCode(t, rr, ErrCodeValidation)
		})
	}
}

func TestRouteHandler_Get(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	route := createTestRoute(t, setup, "Test Route", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/routes/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp RouteResponse
	decodeResponse(t, rr, &resp)

	if resp.Name != route.Name {
		t.Errorf("Expected name %s, got %s", route.Name, resp.Name)
	}
}

func TestRouteHandler_Get_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/routes/9999", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertErrorCode(t, rr, ErrCodeNotFound)
}

func TestRouteHandler_Update(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	createTestRoute(t, setup, "Original Name", nil)

	reqBody := CreateRouteRequest{
		Name:          "Updated Name",
		ConditionType: "callerid",
		ActionType:    "voicemail",
		Priority:      2,
		Enabled:       false,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/routes/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp RouteResponse
	decodeResponse(t, rr, &resp)

	if resp.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", resp.Name)
	}
	if resp.ConditionType != "callerid" {
		t.Errorf("Expected condition type 'callerid', got %s", resp.ConditionType)
	}
	if resp.ActionType != "voicemail" {
		t.Errorf("Expected action type 'voicemail', got %s", resp.ActionType)
	}
	if resp.Enabled {
		t.Error("Expected Enabled to be false")
	}
}

func TestRouteHandler_Delete(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	createTestRoute(t, setup, "Delete Me", nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/routes/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify deleted
	_, err := setup.DB.Routes.GetByID(context.Background(), 1)
	if err == nil {
		t.Error("Expected route to be deleted")
	}
}

func TestRouteHandler_Reorder(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	// Create routes
	createTestRoute(t, setup, "Route A", nil)
	createTestRoute(t, setup, "Route B", nil)
	createTestRoute(t, setup, "Route C", nil)

	reqBody := ReorderRequest{
		Priorities: map[int64]int{
			1: 3,
			2: 1,
			3: 2,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/routes/reorder", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Reorder(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify priorities updated
	route1, _ := setup.DB.Routes.GetByID(context.Background(), 1)
	route2, _ := setup.DB.Routes.GetByID(context.Background(), 2)

	if route1.Priority != 3 {
		t.Errorf("Expected route 1 priority 3, got %d", route1.Priority)
	}
	if route2.Priority != 1 {
		t.Errorf("Expected route 2 priority 1, got %d", route2.Priority)
	}
}

func TestRouteHandler_ListBlocklist(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	// Create blocklist entries
	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Spam",
	}
	setup.DB.Blocklist.Create(context.Background(), entry)

	req := httptest.NewRequest(http.MethodGet, "/api/blocklist", nil)
	rr := httptest.NewRecorder()
	handler.ListBlocklist(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp []*models.BlocklistEntry
	decodeResponse(t, rr, &resp)

	if len(resp) != 1 {
		t.Errorf("Expected 1 blocklist entry, got %d", len(resp))
	}
}

func TestRouteHandler_AddToBlocklist(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	reqBody := AddBlocklistRequest{
		Pattern:     "+15551234567",
		PatternType: "exact",
		Reason:      "Spam caller",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/blocklist", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.AddToBlocklist(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp models.BlocklistEntry
	decodeResponse(t, rr, &resp)

	if resp.Pattern != "+15551234567" {
		t.Errorf("Expected pattern +15551234567, got %s", resp.Pattern)
	}
}

func TestRouteHandler_AddToBlocklist_Validation(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	tests := []struct {
		name    string
		reqBody AddBlocklistRequest
	}{
		{
			name: "Missing pattern",
			reqBody: AddBlocklistRequest{
				PatternType: "exact",
			},
		},
		{
			name: "Invalid pattern type",
			reqBody: AddBlocklistRequest{
				Pattern:     "+15551234567",
				PatternType: "invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/blocklist", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.AddToBlocklist(rr, req)

			assertStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestRouteHandler_AddToBlocklist_DefaultPatternType(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	// Pattern type not specified, should default to "exact"
	reqBody := AddBlocklistRequest{
		Pattern: "+15551234567",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/blocklist", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.AddToBlocklist(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp models.BlocklistEntry
	decodeResponse(t, rr, &resp)

	if resp.PatternType != "exact" {
		t.Errorf("Expected pattern type 'exact', got %s", resp.PatternType)
	}
}

func TestRouteHandler_RemoveFromBlocklist(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewRouteHandler(deps)

	// Create blocklist entry
	entry := &models.BlocklistEntry{
		Pattern:     "+15551234567",
		PatternType: "exact",
	}
	setup.DB.Blocklist.Create(context.Background(), entry)

	req := httptest.NewRequest(http.MethodDelete, "/api/blocklist/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.RemoveFromBlocklist(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify deleted
	_, err := setup.DB.Blocklist.GetByID(context.Background(), 1)
	if err == nil {
		t.Error("Expected blocklist entry to be deleted")
	}
}

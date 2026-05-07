package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/btafoya/gosip/internal/twilio"
)

func TestTrunkHandler_List(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewTrunkHandler(deps)

	createTestTrunkInDB(t, setup.DB, "TK001", "Trunk A")
	createTestTrunkInDB(t, setup.DB, "TK002", "Trunk B")

	req := httptest.NewRequest(http.MethodGet, "/api/trunks", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var wrapper struct {
		Data []*TrunkResponse `json:"data"`
	}
	decodeResponse(t, rr, &wrapper)

	if len(wrapper.Data) != 2 {
		t.Errorf("Expected 2 trunks, got %d", len(wrapper.Data))
	}
}

func TestTrunkHandler_Get(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewTrunkHandler(deps)

	trunk := createTestTrunkInDB(t, setup.DB, "TK123", "Main Trunk")

	req := httptest.NewRequest(http.MethodGet, "/api/trunks/"+strconv.FormatInt(trunk.ID, 10), nil)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(trunk.ID, 10)})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp TrunkResponse
	decodeResponse(t, rr, &resp)
	if resp.TwilioSID != "TK123" {
		t.Errorf("Expected TwilioSID TK123, got %s", resp.TwilioSID)
	}
}

func TestTrunkHandler_Create_Validation(t *testing.T) {
	setup := setupTestAPI(t)
	mockTwilio := setup.Twilio
	mockTwilio.CreateSIPTrunkFunc = func(ctx context.Context, friendlyName string, secure bool) (*twilio.SIPTrunk, error) {
		return &twilio.SIPTrunk{SID: "TK999", FriendlyName: friendlyName, Secure: secure}, nil
	}
	deps := &Dependencies{DB: setup.DB, Twilio: mockTwilio}
	handler := NewTrunkHandler(deps)

	reqBody := CreateTrunkRequest{FriendlyName: ""}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/trunks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestTrunkHandler_SyncFromTwilio(t *testing.T) {
	setup := setupTestAPI(t)
	mockTwilio := setup.Twilio
	mockTwilio.ListSIPTrunksFunc = func(ctx context.Context) ([]*twilio.SIPTrunk, error) {
		return []*twilio.SIPTrunk{
			{SID: "TK001", FriendlyName: "Trunk One", Secure: true, TransferMode: "enable-all"},
			{SID: "TK002", FriendlyName: "Trunk Two", Secure: false, TransferMode: "disable-all"},
		}, nil
	}
	deps := &Dependencies{DB: setup.DB, Twilio: mockTwilio}
	handler := NewTrunkHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/trunks/sync", nil)
	rr := httptest.NewRecorder()
	handler.SyncFromTwilio(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var result struct {
		Message string `json:"message"`
		Created int    `json:"created"`
		Updated int    `json:"updated"`
		Total   int    `json:"total"`
	}
	decodeResponse(t, rr, &result)

	if result.Created != 2 {
		t.Errorf("Expected 2 created, got %d", result.Created)
	}
	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
}

func TestTrunkHandler_AssignDID(t *testing.T) {
	setup := setupTestAPI(t)
	mockTwilio := setup.Twilio
	mockTwilio.AssignPhoneNumberToTrunkFunc = func(ctx context.Context, trunkSID, phoneNumberSID string) error {
		return nil
	}
	deps := &Dependencies{DB: setup.DB, Twilio: mockTwilio}
	handler := NewTrunkHandler(deps)

	trunk := createTestTrunkInDB(t, setup.DB, "TK555", "Assign Trunk")
	did := createTestDID(t, setup.DB, "+15551234567")
	did.TwilioSID = "PN789"
	setup.DB.DIDs.Update(context.Background(), did)

	reqBody := AssignDIDRequest{DIDID: did.ID}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/trunks/assign-did", bytes.NewBuffer(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(trunk.ID, 10)})
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.AssignDID(rr, req)

	assertStatus(t, rr, http.StatusOK)

	updated, _ := setup.DB.DIDs.GetByID(context.Background(), did.ID)
	if updated.TrunkID == nil || *updated.TrunkID != trunk.ID {
		t.Error("Expected DID to be assigned to trunk")
	}
}

func TestTrunkHandler_UnassignDID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewTrunkHandler(deps)

	trunk := createTestTrunkInDB(t, setup.DB, "TK777", "Unassign Trunk")
	did := createTestDID(t, setup.DB, "+15559876543")
	did.TrunkID = &trunk.ID
	setup.DB.DIDs.Update(context.Background(), did)

	reqBody := AssignDIDRequest{DIDID: did.ID}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/trunks/unassign-did", bytes.NewBuffer(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(trunk.ID, 10)})
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UnassignDID(rr, req)

	assertStatus(t, rr, http.StatusOK)

	updated, _ := setup.DB.DIDs.GetByID(context.Background(), did.ID)
	if updated.TrunkID != nil {
		t.Error("Expected DID trunk_id to be nil after unassign")
	}
}

func TestTrunkHandler_UnassignDID_NotAssigned(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB}
	handler := NewTrunkHandler(deps)

	trunk := createTestTrunkInDB(t, setup.DB, "TK888", "Unassign Trunk")
	did := createTestDID(t, setup.DB, "+15551111111")
	// did has no trunk assigned

	reqBody := AssignDIDRequest{DIDID: did.ID}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/trunks/unassign-did", bytes.NewBuffer(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(trunk.ID, 10)})
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.UnassignDID(rr, req)

	assertStatus(t, rr, http.StatusConflict)
}

// Helper to create trunk directly in DB for tests
func createTestTrunkInDB(t *testing.T, database *db.DB, twilioSID, friendlyName string) *models.Trunk {
	t.Helper()
	trunk := &models.Trunk{
		TwilioSID:    twilioSID,
		FriendlyName: friendlyName,
		DomainName:   "example.sip.twilio.com",
		Secure:       true,
		TransferMode: "enable-all",
	}
	if err := database.Trunks.Create(context.Background(), trunk); err != nil {
		t.Fatalf("Failed to create test trunk: %v", err)
	}
	return trunk
}

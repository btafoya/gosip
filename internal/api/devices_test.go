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

func TestDeviceHandler_List(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	// Create test devices
	createTestDevice(t, setup.DB, "Device 1", "user1")
	createTestDevice(t, setup.DB, "Device 2", "user2")

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
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
		t.Errorf("Expected 2 devices, got %d", total)
	}
}

func TestDeviceHandler_List_Pagination(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	// Create multiple devices
	for i := 0; i < 5; i++ {
		device := &models.Device{
			Name:       "Device",
			Username:   "user" + string(rune('a'+i)),
			DeviceType: "softphone",
		}
		setup.DB.Devices.Create(context.Background(), device)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/devices?limit=2&offset=1", nil)
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

func TestDeviceHandler_Create(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	reqBody := CreateDeviceRequest{
		Name:             "Office Phone",
		Username:         "office1",
		Password:         "secretpassword",
		DeviceType:       "grandstream",
		RecordingEnabled: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp DeviceResponse
	decodeResponse(t, rr, &resp)

	if resp.Name != "Office Phone" {
		t.Errorf("Expected name 'Office Phone', got %s", resp.Name)
	}
	if resp.Username != "office1" {
		t.Errorf("Expected username 'office1', got %s", resp.Username)
	}
	if resp.DeviceType != "grandstream" {
		t.Errorf("Expected device type 'grandstream', got %s", resp.DeviceType)
	}
	if !resp.RecordingEnabled {
		t.Error("Expected RecordingEnabled to be true")
	}
}

func TestDeviceHandler_Create_DefaultDeviceType(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	// No device type specified
	reqBody := CreateDeviceRequest{
		Name:     "Test Device",
		Username: "testuser",
		Password: "testpassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusCreated)

	var resp DeviceResponse
	decodeResponse(t, rr, &resp)

	// Should default to "softphone"
	if resp.DeviceType != "softphone" {
		t.Errorf("Expected default device type 'softphone', got %s", resp.DeviceType)
	}
}

func TestDeviceHandler_Create_ValidationError(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	tests := []struct {
		name    string
		reqBody CreateDeviceRequest
	}{
		{
			name: "Missing name",
			reqBody: CreateDeviceRequest{
				Username: "user1",
				Password: "password",
			},
		},
		{
			name: "Missing username",
			reqBody: CreateDeviceRequest{
				Name:     "Device",
				Password: "password",
			},
		},
		{
			name: "Missing password",
			reqBody: CreateDeviceRequest{
				Name:     "Device",
				Username: "user1",
			},
		},
		{
			name: "Invalid device type",
			reqBody: CreateDeviceRequest{
				Name:       "Device",
				Username:   "user1",
				Password:   "password",
				DeviceType: "invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Create(rr, req)

			assertStatus(t, rr, http.StatusBadRequest)
			assertErrorCode(t, rr, ErrCodeValidation)
		})
	}
}

func TestDeviceHandler_Create_InvalidJSON(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestDeviceHandler_Get(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	device := createTestDevice(t, setup.DB, "Test Device", "testuser")

	req := httptest.NewRequest(http.MethodGet, "/api/devices/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp DeviceResponse
	decodeResponse(t, rr, &resp)

	if resp.Name != device.Name {
		t.Errorf("Expected name %s, got %s", device.Name, resp.Name)
	}
}

func TestDeviceHandler_Get_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/9999", nil)
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertErrorCode(t, rr, ErrCodeNotFound)
}

func TestDeviceHandler_Get_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/invalid", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestDeviceHandler_Update(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	createTestDevice(t, setup.DB, "Original Name", "testuser")

	recordingEnabled := true
	reqBody := UpdateDeviceRequest{
		Name:             "Updated Name",
		DeviceType:       "webrtc",
		RecordingEnabled: &recordingEnabled,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/devices/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var resp DeviceResponse
	decodeResponse(t, rr, &resp)

	if resp.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", resp.Name)
	}
	if resp.DeviceType != "webrtc" {
		t.Errorf("Expected device type 'webrtc', got %s", resp.DeviceType)
	}
	if !resp.RecordingEnabled {
		t.Error("Expected RecordingEnabled to be true")
	}
}

func TestDeviceHandler_Update_NotFound(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	reqBody := UpdateDeviceRequest{
		Name: "Updated Name",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/devices/9999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"id": "9999"})

	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
}

func TestDeviceHandler_Delete(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	createTestDevice(t, setup.DB, "Delete Me", "deleteme")

	req := httptest.NewRequest(http.MethodDelete, "/api/devices/1", nil)
	req = withURLParams(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusOK)

	// Verify deleted
	_, err := setup.DB.Devices.GetByID(context.Background(), 1)
	if err == nil {
		t.Error("Expected device to be deleted")
	}
}

func TestDeviceHandler_Delete_InvalidID(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	req := httptest.NewRequest(http.MethodDelete, "/api/devices/invalid", nil)
	req = withURLParams(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
}

func TestDeviceHandler_GetRegistrations(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/registrations", nil)
	rr := httptest.NewRecorder()
	handler.GetRegistrations(rr, req)

	assertStatus(t, rr, http.StatusOK)
}

func TestDeviceResponse_Format(t *testing.T) {
	device := &models.Device{
		ID:               1,
		Name:             "Test Device",
		Username:         "testuser",
		DeviceType:       "softphone",
		RecordingEnabled: true,
	}

	resp := toDeviceResponse(device, true)

	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %d", resp.ID)
	}
	if resp.Name != "Test Device" {
		t.Errorf("Expected name 'Test Device', got %s", resp.Name)
	}
	if resp.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", resp.Username)
	}
	if !resp.Online {
		t.Error("Expected Online to be true")
	}
	if !resp.RecordingEnabled {
		t.Error("Expected RecordingEnabled to be true")
	}
}

func TestDeviceHandler_Create_ValidDeviceTypes(t *testing.T) {
	setup := setupTestAPI(t)
	deps := &Dependencies{DB: setup.DB, SIP: nil}
	handler := NewDeviceHandler(deps)

	validTypes := []string{"grandstream", "softphone", "webrtc"}

	for _, deviceType := range validTypes {
		t.Run(deviceType, func(t *testing.T) {
			reqBody := CreateDeviceRequest{
				Name:       "Test Device",
				Username:   "user_" + deviceType,
				Password:   "password123",
				DeviceType: deviceType,
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Create(rr, req)

			assertStatus(t, rr, http.StatusCreated)

			var resp DeviceResponse
			decodeResponse(t, rr, &resp)

			if resp.DeviceType != deviceType {
				t.Errorf("Expected device type '%s', got %s", deviceType, resp.DeviceType)
			}
		})
	}
}

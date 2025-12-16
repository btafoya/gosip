package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		code         string
		message      string
		details      []FieldError
		wantStatus   int
		wantCode     string
		wantMessage  string
	}{
		{
			name:        "basic error",
			statusCode:  http.StatusBadRequest,
			code:        ErrCodeBadRequest,
			message:     "Invalid input",
			details:     nil,
			wantStatus:  http.StatusBadRequest,
			wantCode:    ErrCodeBadRequest,
			wantMessage: "Invalid input",
		},
		{
			name:        "error with details",
			statusCode:  http.StatusConflict,
			code:        ErrCodeConflict,
			message:     "Resource conflict",
			details:     []FieldError{{Field: "email", Message: "Already exists"}},
			wantStatus:  http.StatusConflict,
			wantCode:    ErrCodeConflict,
			wantMessage: "Resource conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			WriteError(rr, tt.statusCode, tt.code, tt.message, tt.details)

			if rr.Code != tt.wantStatus {
				t.Errorf("WriteError() status = %v, want %v", rr.Code, tt.wantStatus)
			}

			var resp ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if resp.Error.Code != tt.wantCode {
				t.Errorf("WriteError() code = %v, want %v", resp.Error.Code, tt.wantCode)
			}

			if resp.Error.Message != tt.wantMessage {
				t.Errorf("WriteError() message = %v, want %v", resp.Error.Message, tt.wantMessage)
			}
		})
	}
}

func TestWriteInternalError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteInternalError(rr)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("WriteInternalError() status = %v, want %v", rr.Code, http.StatusInternalServerError)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != ErrCodeInternal {
		t.Errorf("WriteInternalError() code = %v, want %v", resp.Error.Code, ErrCodeInternal)
	}

	if resp.Error.Message != "Internal server error" {
		t.Errorf("WriteInternalError() message = %v, want 'Internal server error'", resp.Error.Message)
	}
}

func TestWriteForbiddenError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteForbiddenError(rr)

	if rr.Code != http.StatusForbidden {
		t.Errorf("WriteForbiddenError() status = %v, want %v", rr.Code, http.StatusForbidden)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != ErrCodeAuthorization {
		t.Errorf("WriteForbiddenError() code = %v, want %v", resp.Error.Code, ErrCodeAuthorization)
	}
}

func TestWriteValidationError(t *testing.T) {
	rr := httptest.NewRecorder()
	errors := []FieldError{
		{Field: "email", Message: "Email is required"},
		{Field: "password", Message: "Password too short"},
	}
	WriteValidationError(rr, "Validation failed", errors)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("WriteValidationError() status = %v, want %v", rr.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != ErrCodeValidation {
		t.Errorf("WriteValidationError() code = %v, want %v", resp.Error.Code, ErrCodeValidation)
	}

	if resp.Error.Message != "Validation failed" {
		t.Errorf("WriteValidationError() message = %v, want 'Validation failed'", resp.Error.Message)
	}
}

func TestWriteNotFoundError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteNotFoundError(rr, "User")

	if rr.Code != http.StatusNotFound {
		t.Errorf("WriteNotFoundError() status = %v, want %v", rr.Code, http.StatusNotFound)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != ErrCodeNotFound {
		t.Errorf("WriteNotFoundError() code = %v, want %v", resp.Error.Code, ErrCodeNotFound)
	}

	if resp.Error.Message != "User not found" {
		t.Errorf("WriteNotFoundError() message = %v, want 'User not found'", resp.Error.Message)
	}
}

func TestWriteUnauthorizedError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteUnauthorizedError(rr)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("WriteUnauthorizedError() status = %v, want %v", rr.Code, http.StatusUnauthorized)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error.Code != ErrCodeAuthentication {
		t.Errorf("WriteUnauthorizedError() code = %v, want %v", resp.Error.Code, ErrCodeAuthentication)
	}
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       interface{}
		wantStatus int
	}{
		{
			name:       "simple object",
			statusCode: http.StatusOK,
			data:       map[string]string{"message": "success"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "created status",
			statusCode: http.StatusCreated,
			data:       map[string]int{"id": 123},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "empty slice",
			statusCode: http.StatusOK,
			data:       []string{},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			WriteJSON(rr, tt.statusCode, tt.data)

			if rr.Code != tt.wantStatus {
				t.Errorf("WriteJSON() status = %v, want %v", rr.Code, tt.wantStatus)
			}

			contentType := rr.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("WriteJSON() Content-Type = %v, want 'application/json'", contentType)
			}
		})
	}
}

func TestWriteList(t *testing.T) {
	rr := httptest.NewRecorder()
	items := []map[string]string{
		{"name": "item1"},
		{"name": "item2"},
	}
	WriteList(rr, items, 100, 20, 0)

	if rr.Code != http.StatusOK {
		t.Errorf("WriteList() status = %v, want %v", rr.Code, http.StatusOK)
	}

	var resp struct {
		Data       []map[string]string `json:"data"`
		Pagination struct {
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("WriteList() data length = %v, want 2", len(resp.Data))
	}

	if resp.Pagination.Total != 100 {
		t.Errorf("WriteList() pagination.total = %v, want 100", resp.Pagination.Total)
	}

	if resp.Pagination.Limit != 20 {
		t.Errorf("WriteList() pagination.limit = %v, want 20", resp.Pagination.Limit)
	}

	if resp.Pagination.Offset != 0 {
		t.Errorf("WriteList() pagination.offset = %v, want 0", resp.Pagination.Offset)
	}
}

func TestErrorCodes(t *testing.T) {
	// Test that error code constants are defined correctly
	codes := map[string]string{
		"ErrCodeValidation":     ErrCodeValidation,
		"ErrCodeAuthentication": ErrCodeAuthentication,
		"ErrCodeAuthorization":  ErrCodeAuthorization,
		"ErrCodeNotFound":       ErrCodeNotFound,
		"ErrCodeConflict":       ErrCodeConflict,
		"ErrCodeInternal":       ErrCodeInternal,
		"ErrCodeBadRequest":     ErrCodeBadRequest,
		"ErrCodeRateLimited":    ErrCodeRateLimited,
	}

	for name, code := range codes {
		if code == "" {
			t.Errorf("%s is empty", name)
		}
	}

	// Verify codes are unique
	seen := make(map[string]string)
	for name, code := range codes {
		if prev, exists := seen[code]; exists {
			t.Errorf("Duplicate error code %q used by both %s and %s", code, prev, name)
		}
		seen[code] = name
	}
}

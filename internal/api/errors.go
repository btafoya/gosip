package api

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse follows the standard API error format from REQUIREMENTS.md
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Standard error codes
const (
	ErrCodeValidation         = "VALIDATION_ERROR"
	ErrCodeAuthentication     = "AUTHENTICATION_ERROR"
	ErrCodeAuthorization      = "AUTHORIZATION_ERROR"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeRateLimited        = "RATE_LIMITED"
	ErrCodeInternal           = "INTERNAL_ERROR"
	ErrCodeConflict           = "CONFLICT"
	ErrCodeBadRequest         = "BAD_REQUEST"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeBadGateway         = "BAD_GATEWAY"
)

// WriteError writes a standardized error response
func WriteError(w http.ResponseWriter, statusCode int, code, message string, details []FieldError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// WriteValidationError is a helper for validation errors
func WriteValidationError(w http.ResponseWriter, message string, details []FieldError) {
	WriteError(w, http.StatusBadRequest, ErrCodeValidation, message, details)
}

// WriteNotFoundError is a helper for not found errors
func WriteNotFoundError(w http.ResponseWriter, resource string) {
	WriteError(w, http.StatusNotFound, ErrCodeNotFound, resource+" not found", nil)
}

// WriteInternalError is a helper for internal server errors
func WriteInternalError(w http.ResponseWriter) {
	WriteError(w, http.StatusInternalServerError, ErrCodeInternal, "Internal server error", nil)
}

// WriteUnauthorizedError is a helper for authentication errors
func WriteUnauthorizedError(w http.ResponseWriter) {
	WriteError(w, http.StatusUnauthorized, ErrCodeAuthentication, "Authentication required", nil)
}

// WriteForbiddenError is a helper for authorization errors
func WriteForbiddenError(w http.ResponseWriter) {
	WriteError(w, http.StatusForbidden, ErrCodeAuthorization, "Access denied", nil)
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// Response helpers for common patterns

// ListResponse represents a paginated list response
type ListResponse struct {
	Data       interface{} `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// WriteList writes a paginated list response
func WriteList(w http.ResponseWriter, data interface{}, total, limit, offset int) {
	WriteJSON(w, http.StatusOK, ListResponse{
		Data: data,
		Pagination: &Pagination{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	})
}

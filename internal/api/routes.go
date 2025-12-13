package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
	"github.com/go-chi/chi/v5"
)

// RouteHandler handles routing rule API endpoints
type RouteHandler struct {
	deps *Dependencies
}

// NewRouteHandler creates a new RouteHandler
func NewRouteHandler(deps *Dependencies) *RouteHandler {
	return &RouteHandler{deps: deps}
}

// RouteResponse represents a route in API responses
type RouteResponse struct {
	ID            int64           `json:"id"`
	DIDID         *int64          `json:"did_id,omitempty"`
	Priority      int             `json:"priority"`
	Name          string          `json:"name"`
	ConditionType string          `json:"condition_type"`
	ConditionData json.RawMessage `json:"condition_data,omitempty"`
	ActionType    string          `json:"action_type"`
	ActionData    json.RawMessage `json:"action_data,omitempty"`
	Enabled       bool            `json:"enabled"`
}

// List returns all routes
func (h *RouteHandler) List(w http.ResponseWriter, r *http.Request) {
	// Optionally filter by DID
	didIDStr := r.URL.Query().Get("did_id")
	var routes []*models.Route
	var err error

	if didIDStr != "" {
		didID, parseErr := strconv.ParseInt(didIDStr, 10, 64)
		if parseErr != nil {
			WriteValidationError(w, "Invalid DID ID", nil)
			return
		}
		routes, err = h.deps.DB.Routes.GetByDID(r.Context(), didID)
	} else {
		routes, err = h.deps.DB.Routes.List(r.Context())
	}

	if err != nil {
		WriteInternalError(w)
		return
	}

	var response []*RouteResponse
	for _, r := range routes {
		response = append(response, toRouteResponse(r))
	}

	WriteJSON(w, http.StatusOK, response)
}

// CreateRouteRequest represents a route creation request
type CreateRouteRequest struct {
	DIDID         *int64          `json:"did_id,omitempty"`
	Priority      int             `json:"priority"`
	Name          string          `json:"name"`
	ConditionType string          `json:"condition_type"`
	ConditionData json.RawMessage `json:"condition_data,omitempty"`
	ActionType    string          `json:"action_type"`
	ActionData    json.RawMessage `json:"action_data,omitempty"`
	Enabled       bool            `json:"enabled"`
}

// Create creates a new route
func (h *RouteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	var errors []FieldError
	if req.Name == "" {
		errors = append(errors, FieldError{Field: "name", Message: "Name is required"})
	}
	if req.ConditionType != "time" && req.ConditionType != "callerid" && req.ConditionType != "default" {
		errors = append(errors, FieldError{Field: "condition_type", Message: "Invalid condition type"})
	}
	if req.ActionType != "ring" && req.ActionType != "forward" && req.ActionType != "voicemail" && req.ActionType != "reject" {
		errors = append(errors, FieldError{Field: "action_type", Message: "Invalid action type"})
	}

	if len(errors) > 0 {
		WriteValidationError(w, "Validation failed", errors)
		return
	}

	route := &models.Route{
		DIDID:         req.DIDID,
		Priority:      req.Priority,
		Name:          req.Name,
		ConditionType: req.ConditionType,
		ConditionData: req.ConditionData,
		ActionType:    req.ActionType,
		ActionData:    req.ActionData,
		Enabled:       req.Enabled,
	}

	if err := h.deps.DB.Routes.Create(r.Context(), route); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusCreated, toRouteResponse(route))
}

// Get returns a specific route
func (h *RouteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid route ID", nil)
		return
	}

	route, err := h.deps.DB.Routes.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrRouteNotFound {
			WriteNotFoundError(w, "Route")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toRouteResponse(route))
}

// Update updates a route
func (h *RouteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid route ID", nil)
		return
	}

	route, err := h.deps.DB.Routes.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrRouteNotFound {
			WriteNotFoundError(w, "Route")
			return
		}
		WriteInternalError(w)
		return
	}

	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if req.Name != "" {
		route.Name = req.Name
	}
	if req.ConditionType != "" {
		route.ConditionType = req.ConditionType
	}
	if req.ConditionData != nil {
		route.ConditionData = req.ConditionData
	}
	if req.ActionType != "" {
		route.ActionType = req.ActionType
	}
	if req.ActionData != nil {
		route.ActionData = req.ActionData
	}
	route.Priority = req.Priority
	route.Enabled = req.Enabled
	route.DIDID = req.DIDID

	if err := h.deps.DB.Routes.Update(r.Context(), route); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, toRouteResponse(route))
}

// Delete removes a route
func (h *RouteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid route ID", nil)
		return
	}

	if err := h.deps.DB.Routes.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Route deleted successfully"})
}

// ReorderRequest represents a route reordering request
type ReorderRequest struct {
	Priorities map[int64]int `json:"priorities"`
}

// Reorder updates the priority of multiple routes
func (h *RouteHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var req ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	if err := h.deps.DB.Routes.UpdatePriorities(r.Context(), req.Priorities); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Routes reordered successfully"})
}

// Blocklist endpoints

// ListBlocklist returns all blocklist entries
func (h *RouteHandler) ListBlocklist(w http.ResponseWriter, r *http.Request) {
	entries, err := h.deps.DB.Blocklist.List(r.Context())
	if err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, entries)
}

// AddBlocklistRequest represents a blocklist addition request
type AddBlocklistRequest struct {
	Pattern     string `json:"pattern"`
	PatternType string `json:"pattern_type"`
	Reason      string `json:"reason,omitempty"`
}

// AddToBlocklist adds a number to the blocklist
func (h *RouteHandler) AddToBlocklist(w http.ResponseWriter, r *http.Request) {
	var req AddBlocklistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}

	// Validate
	if req.Pattern == "" {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "pattern", Message: "Pattern is required"},
		})
		return
	}
	if req.PatternType == "" {
		req.PatternType = "exact"
	}
	if req.PatternType != "exact" && req.PatternType != "prefix" && req.PatternType != "regex" {
		WriteValidationError(w, "Validation failed", []FieldError{
			{Field: "pattern_type", Message: "Invalid pattern type"},
		})
		return
	}

	entry := &models.BlocklistEntry{
		Pattern:     req.Pattern,
		PatternType: req.PatternType,
		Reason:      req.Reason,
	}

	if err := h.deps.DB.Blocklist.Create(r.Context(), entry); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusCreated, entry)
}

// RemoveFromBlocklist removes a number from the blocklist
func (h *RouteHandler) RemoveFromBlocklist(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid blocklist entry ID", nil)
		return
	}

	if err := h.deps.DB.Blocklist.Delete(r.Context(), id); err != nil {
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Entry removed from blocklist"})
}

func toRouteResponse(route *models.Route) *RouteResponse {
	return &RouteResponse{
		ID:            route.ID,
		DIDID:         route.DIDID,
		Priority:      route.Priority,
		Name:          route.Name,
		ConditionType: route.ConditionType,
		ConditionData: route.ConditionData,
		ActionType:    route.ActionType,
		ActionData:    route.ActionData,
		Enabled:       route.Enabled,
	}
}

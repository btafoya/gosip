package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/btafoya/gosip/internal/config"
	"github.com/btafoya/gosip/internal/db"
	"github.com/go-chi/chi/v5"
)

// CDRHandler handles CDR (Call Detail Record) API endpoints
type CDRHandler struct {
	deps *Dependencies
}

// NewCDRHandler creates a new CDRHandler
func NewCDRHandler(deps *Dependencies) *CDRHandler {
	return &CDRHandler{deps: deps}
}

// List returns CDRs with filtering and pagination
func (h *CDRHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	direction := r.URL.Query().Get("direction")
	disposition := r.URL.Query().Get("disposition")
	didIDStr := r.URL.Query().Get("did_id")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	if limit == 0 {
		limit = config.DefaultPageSize
	}
	if limit > config.MaxPageSize {
		limit = config.MaxPageSize
	}

	filter := db.CDRFilter{
		Direction:   direction,
		Disposition: disposition,
		Limit:       limit,
		Offset:      offset,
	}

	if didIDStr != "" {
		didID, err := strconv.ParseInt(didIDStr, 10, 64)
		if err == nil {
			filter.DIDID = &didID
		}
	}

	if startDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			filter.StartDate = &startDate
		}
	}

	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			// Add a day to include the entire end date
			endDate = endDate.Add(24 * time.Hour)
			filter.EndDate = &endDate
		}
	}

	cdrs, err := h.deps.DB.CDRs.List(r.Context(), filter)
	if err != nil {
		WriteInternalError(w)
		return
	}

	total, _ := h.deps.DB.CDRs.Count(r.Context(), filter)

	WriteList(w, cdrs, total, limit, offset)
}

// Get returns a specific CDR
func (h *CDRHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid CDR ID", nil)
		return
	}

	cdr, err := h.deps.DB.CDRs.GetByID(r.Context(), id)
	if err != nil {
		if err == db.ErrCDRNotFound {
			WriteNotFoundError(w, "CDR")
			return
		}
		WriteInternalError(w)
		return
	}

	WriteJSON(w, http.StatusOK, cdr)
}

// GetStats returns call statistics
func (h *CDRHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Default to last 30 days
	endDate := time.Now()
	startDate := endDate.Add(-30 * 24 * time.Hour)

	// Parse custom date range if provided
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	if startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = parsed
		}
	}

	if endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = parsed.Add(24 * time.Hour)
		}
	}

	stats, err := h.deps.DB.CDRs.GetStatsByDisposition(r.Context(), startDate, endDate)
	if err != nil {
		WriteInternalError(w)
		return
	}

	// Calculate totals
	total := 0
	for _, count := range stats {
		total += count
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"period": map[string]string{
			"start": startDate.Format("2006-01-02"),
			"end":   endDate.Add(-24 * time.Hour).Format("2006-01-02"),
		},
		"total":          total,
		"by_disposition": stats,
	})
}

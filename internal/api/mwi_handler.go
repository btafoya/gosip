package api

import (
	"net/http"
)

// MWIHandler handles MWI-related API endpoints
type MWIHandler struct {
	deps *Dependencies
}

// NewMWIHandler creates a new MWI handler
func NewMWIHandler(deps *Dependencies) *MWIHandler {
	return &MWIHandler{deps: deps}
}

// MWIStatusResponse represents the MWI status
type MWIStatusResponse struct {
	Enabled            bool                    `json:"enabled"`
	SubscriptionCount  int                     `json:"subscription_count"`
	States             []MWIStateResponse      `json:"states"`
	Subscriptions      []MWISubscriptionResponse `json:"subscriptions"`
}

// MWIStateResponse represents mailbox state
type MWIStateResponse struct {
	AOR          string `json:"aor"`
	NewMessages  int    `json:"new_messages"`
	OldMessages  int    `json:"old_messages"`
	NewUrgent    int    `json:"new_urgent"`
	OldUrgent    int    `json:"old_urgent"`
	LastUpdated  string `json:"last_updated"`
}

// MWISubscriptionResponse represents an MWI subscription
type MWISubscriptionResponse struct {
	ID         string `json:"id"`
	AOR        string `json:"aor"`
	ContactURI string `json:"contact_uri"`
	Expires    int    `json:"expires"`
	ExpiresAt  string `json:"expires_at"`
}

// GetStatus returns the current MWI status
func (h *MWIHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	response := MWIStatusResponse{
		Enabled:           false,
		SubscriptionCount: 0,
		States:            []MWIStateResponse{},
		Subscriptions:     []MWISubscriptionResponse{},
	}

	if h.deps.SIP == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{"data": response})
		return
	}

	mwiMgr := h.deps.SIP.GetMWIManager()
	if mwiMgr == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{"data": response})
		return
	}

	response.Enabled = true
	response.SubscriptionCount = mwiMgr.GetSubscriptionCount()

	// Get all states
	states := mwiMgr.GetAllStates()
	for aor, state := range states {
		response.States = append(response.States, MWIStateResponse{
			AOR:         aor,
			NewMessages: state.NewMessages,
			OldMessages: state.OldMessages,
			NewUrgent:   state.NewUrgent,
			OldUrgent:   state.OldUrgent,
			LastUpdated: state.LastUpdated.Format("2006-01-02T15:04:05Z"),
		})
	}

	// Get subscriptions by iterating through known AORs
	// Note: This is a simplified approach - in production you'd have a GetAllSubscriptions method
	for aor := range states {
		subs := mwiMgr.GetSubscriptionsForAOR(aor)
		for _, sub := range subs {
			response.Subscriptions = append(response.Subscriptions, MWISubscriptionResponse{
				ID:         sub.ID,
				AOR:        sub.AOR,
				ContactURI: sub.ContactURI,
				Expires:    sub.Expires,
				ExpiresAt:  sub.ExpiresAt.Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": response})
}

// TriggerNotification manually triggers an MWI notification for testing
func (h *MWIHandler) TriggerNotification(w http.ResponseWriter, r *http.Request) {
	if h.deps.SIP == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "SIP server not available",
		})
		return
	}

	mwiMgr := h.deps.SIP.GetMWIManager()
	if mwiMgr == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "MWI manager not available",
		})
		return
	}

	// Get all states and notify all subscribers
	states := mwiMgr.GetAllStates()
	notified := 0

	for aor := range states {
		if err := mwiMgr.NotifyAllSubscribers(r.Context(), aor); err == nil {
			notified++
		}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "MWI notifications triggered",
		"notified": notified,
	})
}

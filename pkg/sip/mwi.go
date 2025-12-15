package sip

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MWIState represents the message waiting indicator state for a mailbox
type MWIState struct {
	AOR              string    // Address of Record (sip:user@domain)
	NewMessages      int       // Count of new (unread) voicemails
	OldMessages      int       // Count of read voicemails
	NewUrgent        int       // Count of new urgent messages
	OldUrgent        int       // Count of old urgent messages
	LastUpdated      time.Time // When the state was last updated
}

// MWISubscription represents an MWI subscription from a device
type MWISubscription struct {
	ID           string
	AOR          string    // Address of Record being monitored
	ContactURI   string    // Where to send NOTIFY
	FromURI      string    // From header for NOTIFY
	ToURI        string    // To header for NOTIFY
	CallID       string    // Call-ID for this dialog
	FromTag      string    // From tag
	ToTag        string    // To tag
	CSeq         uint32    // Current CSeq
	Expires      int       // Subscription duration in seconds
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// MWIManager handles Message Waiting Indicator state and notifications
type MWIManager struct {
	logger        *slog.Logger
	server        *Server // Reference to SIP server for sending NOTIFY

	mu            sync.RWMutex
	states        map[string]*MWIState         // AOR -> state
	subscriptions map[string]*MWISubscription  // subscription ID -> subscription
	aorSubs       map[string][]string          // AOR -> subscription IDs

	// Event callbacks
	onStateChange func(aor string, state *MWIState)
}

// NewMWIManager creates a new MWI manager
func NewMWIManager(logger *slog.Logger) *MWIManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &MWIManager{
		logger:        logger,
		states:        make(map[string]*MWIState),
		subscriptions: make(map[string]*MWISubscription),
		aorSubs:       make(map[string][]string),
	}
}

// SetServer sets the SIP server reference for sending NOTIFY messages
func (m *MWIManager) SetServer(server *Server) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.server = server
}

// SetOnStateChange sets the callback for state changes
func (m *MWIManager) SetOnStateChange(fn func(aor string, state *MWIState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = fn
}

// UpdateState updates the MWI state for a mailbox and triggers notifications
func (m *MWIManager) UpdateState(ctx context.Context, aor string, newMessages, oldMessages int) error {
	m.mu.Lock()
	state := m.states[aor]
	if state == nil {
		state = &MWIState{AOR: aor}
		m.states[aor] = state
	}

	// Check if state actually changed
	changed := state.NewMessages != newMessages || state.OldMessages != oldMessages

	state.NewMessages = newMessages
	state.OldMessages = oldMessages
	state.LastUpdated = time.Now()

	// Copy state for notification
	stateCopy := *state

	// Get subscriptions for this AOR
	subIDs := m.aorSubs[aor]
	subs := make([]*MWISubscription, 0, len(subIDs))
	for _, id := range subIDs {
		if sub := m.subscriptions[id]; sub != nil {
			subs = append(subs, sub)
		}
	}

	// Call state change callback
	onStateChange := m.onStateChange
	m.mu.Unlock()

	if onStateChange != nil && changed {
		onStateChange(aor, &stateCopy)
	}

	if !changed {
		return nil
	}

	m.logger.Info("MWI state updated",
		slog.String("aor", aor),
		slog.Int("new_messages", newMessages),
		slog.Int("old_messages", oldMessages),
		slog.Int("subscriptions", len(subs)),
	)

	// Send NOTIFY to all subscribers
	for _, sub := range subs {
		if err := m.sendNotify(ctx, sub, &stateCopy); err != nil {
			m.logger.Error("Failed to send MWI NOTIFY",
				slog.String("aor", aor),
				slog.String("contact", sub.ContactURI),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// GetState returns the current MWI state for an AOR
func (m *MWIManager) GetState(aor string) *MWIState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if state := m.states[aor]; state != nil {
		copy := *state
		return &copy
	}
	return nil
}

// AddSubscription adds a new MWI subscription
func (m *MWIManager) AddSubscription(sub *MWISubscription) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove any existing subscription with same ID
	if existing := m.subscriptions[sub.ID]; existing != nil {
		m.removeSubscriptionLocked(sub.ID)
	}

	sub.CreatedAt = time.Now()
	sub.ExpiresAt = time.Now().Add(time.Duration(sub.Expires) * time.Second)

	m.subscriptions[sub.ID] = sub
	m.aorSubs[sub.AOR] = append(m.aorSubs[sub.AOR], sub.ID)

	m.logger.Info("MWI subscription added",
		slog.String("id", sub.ID),
		slog.String("aor", sub.AOR),
		slog.String("contact", sub.ContactURI),
		slog.Int("expires", sub.Expires),
	)
}

// RemoveSubscription removes an MWI subscription
func (m *MWIManager) RemoveSubscription(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeSubscriptionLocked(id)
}

func (m *MWIManager) removeSubscriptionLocked(id string) {
	sub := m.subscriptions[id]
	if sub == nil {
		return
	}

	delete(m.subscriptions, id)

	// Remove from AOR mapping
	subs := m.aorSubs[sub.AOR]
	for i, sid := range subs {
		if sid == id {
			m.aorSubs[sub.AOR] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	m.logger.Info("MWI subscription removed",
		slog.String("id", id),
		slog.String("aor", sub.AOR),
	)
}

// GetSubscription returns a subscription by ID
func (m *MWIManager) GetSubscription(id string) *MWISubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sub := m.subscriptions[id]; sub != nil {
		copy := *sub
		return &copy
	}
	return nil
}

// GetSubscriptionsForAOR returns all subscriptions for an AOR
func (m *MWIManager) GetSubscriptionsForAOR(aor string) []*MWISubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subIDs := m.aorSubs[aor]
	subs := make([]*MWISubscription, 0, len(subIDs))
	for _, id := range subIDs {
		if sub := m.subscriptions[id]; sub != nil {
			copy := *sub
			subs = append(subs, &copy)
		}
	}
	return subs
}

// CleanupExpired removes expired subscriptions
func (m *MWIManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var expired []string

	for id, sub := range m.subscriptions {
		if sub.ExpiresAt.Before(now) {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		m.removeSubscriptionLocked(id)
	}

	if len(expired) > 0 {
		m.logger.Info("Cleaned up expired MWI subscriptions",
			slog.Int("count", len(expired)),
		)
	}

	return len(expired)
}

// sendNotify sends an MWI NOTIFY to a subscriber
func (m *MWIManager) sendNotify(ctx context.Context, sub *MWISubscription, state *MWIState) error {
	m.mu.RLock()
	server := m.server
	m.mu.RUnlock()

	if server == nil {
		return fmt.Errorf("SIP server not set")
	}

	// Increment CSeq for next NOTIFY
	m.mu.Lock()
	if s := m.subscriptions[sub.ID]; s != nil {
		s.CSeq++
		sub.CSeq = s.CSeq
	}
	m.mu.Unlock()

	// Build MWI NOTIFY body per RFC 3842
	body := m.buildMWIBody(state)

	// Send NOTIFY through SIP server
	return server.SendMWINotify(ctx, sub, body)
}

// buildMWIBody creates the message-summary body per RFC 3842
func (m *MWIManager) buildMWIBody(state *MWIState) string {
	// Messages-Waiting header
	waiting := "no"
	if state.NewMessages > 0 {
		waiting = "yes"
	}

	body := fmt.Sprintf("Messages-Waiting: %s\r\n", waiting)
	body += fmt.Sprintf("Message-Account: %s\r\n", state.AOR)
	body += fmt.Sprintf("Voice-Message: %d/%d (%d/%d)\r\n",
		state.NewMessages, state.OldMessages,
		state.NewUrgent, state.OldUrgent)

	return body
}

// GetAllStates returns all MWI states (for debugging/monitoring)
func (m *MWIManager) GetAllStates() map[string]*MWIState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*MWIState, len(m.states))
	for aor, state := range m.states {
		copy := *state
		result[aor] = &copy
	}
	return result
}

// GetSubscriptionCount returns the total number of active subscriptions
func (m *MWIManager) GetSubscriptionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.subscriptions)
}

// RefreshSubscription refreshes an existing subscription
func (m *MWIManager) RefreshSubscription(id string, expires int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub := m.subscriptions[id]
	if sub == nil {
		return fmt.Errorf("subscription not found: %s", id)
	}

	sub.Expires = expires
	sub.ExpiresAt = time.Now().Add(time.Duration(expires) * time.Second)

	m.logger.Info("MWI subscription refreshed",
		slog.String("id", id),
		slog.String("aor", sub.AOR),
		slog.Int("expires", expires),
	)

	return nil
}

// NotifyAllSubscribers sends NOTIFY to all subscribers for an AOR
// Used when we need to force a notification (e.g., on initial subscription)
func (m *MWIManager) NotifyAllSubscribers(ctx context.Context, aor string) error {
	m.mu.RLock()
	state := m.states[aor]
	if state == nil {
		state = &MWIState{AOR: aor, LastUpdated: time.Now()}
	}
	stateCopy := *state

	subIDs := m.aorSubs[aor]
	subs := make([]*MWISubscription, 0, len(subIDs))
	for _, id := range subIDs {
		if sub := m.subscriptions[id]; sub != nil {
			subs = append(subs, sub)
		}
	}
	m.mu.RUnlock()

	var lastErr error
	for _, sub := range subs {
		if err := m.sendNotify(ctx, sub, &stateCopy); err != nil {
			m.logger.Error("Failed to send MWI NOTIFY",
				slog.String("aor", aor),
				slog.String("contact", sub.ContactURI),
				slog.String("error", err.Error()),
			)
			lastErr = err
		}
	}

	return lastErr
}

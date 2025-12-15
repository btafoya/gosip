package sip

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestMWIManager_NewMWIManager(t *testing.T) {
	mgr := NewMWIManager(slog.Default())
	if mgr == nil {
		t.Fatal("Expected non-nil MWI manager")
	}
	if mgr.GetSubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", mgr.GetSubscriptionCount())
	}
}

func TestMWIManager_UpdateState(t *testing.T) {
	mgr := NewMWIManager(slog.Default())
	ctx := context.Background()

	aor := "sip:user@example.com"
	err := mgr.UpdateState(ctx, aor, 5, 10)
	if err != nil {
		t.Fatalf("UpdateState failed: %v", err)
	}

	state := mgr.GetState(aor)
	if state == nil {
		t.Fatal("Expected state to be set")
	}
	if state.NewMessages != 5 {
		t.Errorf("Expected 5 new messages, got %d", state.NewMessages)
	}
	if state.OldMessages != 10 {
		t.Errorf("Expected 10 old messages, got %d", state.OldMessages)
	}
	if state.AOR != aor {
		t.Errorf("Expected AOR %s, got %s", aor, state.AOR)
	}
}

func TestMWIManager_GetState_NotFound(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	state := mgr.GetState("sip:unknown@example.com")
	if state != nil {
		t.Error("Expected nil state for unknown AOR")
	}
}

func TestMWIManager_AddSubscription(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	sub := &MWISubscription{
		ID:         "sub-123",
		AOR:        "sip:user@example.com",
		ContactURI: "sip:device@192.168.1.100:5060",
		CallID:     "abc123",
		FromTag:    "from-tag",
		Expires:    3600,
	}

	mgr.AddSubscription(sub)

	if mgr.GetSubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription, got %d", mgr.GetSubscriptionCount())
	}

	retrieved := mgr.GetSubscription("sub-123")
	if retrieved == nil {
		t.Fatal("Expected to find subscription")
	}
	if retrieved.AOR != sub.AOR {
		t.Errorf("Expected AOR %s, got %s", sub.AOR, retrieved.AOR)
	}
	if retrieved.ContactURI != sub.ContactURI {
		t.Errorf("Expected contact %s, got %s", sub.ContactURI, retrieved.ContactURI)
	}
}

func TestMWIManager_RemoveSubscription(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	sub := &MWISubscription{
		ID:      "sub-123",
		AOR:     "sip:user@example.com",
		Expires: 3600,
	}
	mgr.AddSubscription(sub)

	if mgr.GetSubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription, got %d", mgr.GetSubscriptionCount())
	}

	mgr.RemoveSubscription("sub-123")

	if mgr.GetSubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after removal, got %d", mgr.GetSubscriptionCount())
	}

	retrieved := mgr.GetSubscription("sub-123")
	if retrieved != nil {
		t.Error("Expected subscription to be removed")
	}
}

func TestMWIManager_GetSubscriptionsForAOR(t *testing.T) {
	mgr := NewMWIManager(slog.Default())
	aor := "sip:user@example.com"

	// Add multiple subscriptions for same AOR
	mgr.AddSubscription(&MWISubscription{
		ID:      "sub-1",
		AOR:     aor,
		Expires: 3600,
	})
	mgr.AddSubscription(&MWISubscription{
		ID:      "sub-2",
		AOR:     aor,
		Expires: 3600,
	})
	mgr.AddSubscription(&MWISubscription{
		ID:      "sub-3",
		AOR:     "sip:other@example.com",
		Expires: 3600,
	})

	subs := mgr.GetSubscriptionsForAOR(aor)
	if len(subs) != 2 {
		t.Errorf("Expected 2 subscriptions for AOR, got %d", len(subs))
	}
}

func TestMWIManager_RefreshSubscription(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	sub := &MWISubscription{
		ID:      "sub-123",
		AOR:     "sip:user@example.com",
		Expires: 3600,
	}
	mgr.AddSubscription(sub)

	// Refresh with new expiry
	err := mgr.RefreshSubscription("sub-123", 7200)
	if err != nil {
		t.Fatalf("RefreshSubscription failed: %v", err)
	}

	retrieved := mgr.GetSubscription("sub-123")
	if retrieved == nil {
		t.Fatal("Expected subscription to exist")
	}
	if retrieved.Expires != 7200 {
		t.Errorf("Expected expires 7200, got %d", retrieved.Expires)
	}
}

func TestMWIManager_RefreshSubscription_NotFound(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	err := mgr.RefreshSubscription("nonexistent", 3600)
	if err == nil {
		t.Error("Expected error for nonexistent subscription")
	}
}

func TestMWIManager_CleanupExpired(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	// Add subscription that's already expired
	sub := &MWISubscription{
		ID:        "expired-sub",
		AOR:       "sip:user@example.com",
		Expires:   -1, // Already expired
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	mgr.AddSubscription(sub)

	// Manually set expiry time
	mgr.mu.Lock()
	if s := mgr.subscriptions["expired-sub"]; s != nil {
		s.ExpiresAt = time.Now().Add(-time.Hour)
	}
	mgr.mu.Unlock()

	// Add non-expired subscription
	mgr.AddSubscription(&MWISubscription{
		ID:      "active-sub",
		AOR:     "sip:user@example.com",
		Expires: 3600,
	})

	count := mgr.CleanupExpired()
	if count != 1 {
		t.Errorf("Expected 1 expired subscription cleaned, got %d", count)
	}

	if mgr.GetSubscriptionCount() != 1 {
		t.Errorf("Expected 1 remaining subscription, got %d", mgr.GetSubscriptionCount())
	}
}

func TestMWIManager_OnStateChange(t *testing.T) {
	mgr := NewMWIManager(slog.Default())
	ctx := context.Background()

	callbackCalled := false
	var callbackAOR string
	var callbackState *MWIState

	mgr.SetOnStateChange(func(aor string, state *MWIState) {
		callbackCalled = true
		callbackAOR = aor
		callbackState = state
	})

	aor := "sip:user@example.com"
	mgr.UpdateState(ctx, aor, 3, 5)

	if !callbackCalled {
		t.Error("Expected state change callback to be called")
	}
	if callbackAOR != aor {
		t.Errorf("Expected callback AOR %s, got %s", aor, callbackAOR)
	}
	if callbackState == nil || callbackState.NewMessages != 3 {
		t.Error("Expected callback state to have 3 new messages")
	}
}

func TestMWIManager_GetAllStates(t *testing.T) {
	mgr := NewMWIManager(slog.Default())
	ctx := context.Background()

	mgr.UpdateState(ctx, "sip:user1@example.com", 1, 2)
	mgr.UpdateState(ctx, "sip:user2@example.com", 3, 4)

	states := mgr.GetAllStates()
	if len(states) != 2 {
		t.Errorf("Expected 2 states, got %d", len(states))
	}

	if state := states["sip:user1@example.com"]; state == nil || state.NewMessages != 1 {
		t.Error("Expected user1 state with 1 new message")
	}
	if state := states["sip:user2@example.com"]; state == nil || state.NewMessages != 3 {
		t.Error("Expected user2 state with 3 new messages")
	}
}

func TestMWIManager_BuildMWIBody(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	state := &MWIState{
		AOR:         "sip:user@example.com",
		NewMessages: 2,
		OldMessages: 5,
		NewUrgent:   1,
		OldUrgent:   0,
	}

	body := mgr.buildMWIBody(state)

	// Check that body contains required headers
	if body == "" {
		t.Error("Expected non-empty MWI body")
	}
	if !contains(body, "Messages-Waiting: yes") {
		t.Error("Expected Messages-Waiting: yes")
	}
	if !contains(body, "Message-Account: sip:user@example.com") {
		t.Error("Expected Message-Account header")
	}
	if !contains(body, "Voice-Message: 2/5 (1/0)") {
		t.Error("Expected Voice-Message header with correct counts")
	}
}

func TestMWIManager_BuildMWIBody_NoMessages(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	state := &MWIState{
		AOR:         "sip:user@example.com",
		NewMessages: 0,
		OldMessages: 3,
	}

	body := mgr.buildMWIBody(state)

	if !contains(body, "Messages-Waiting: no") {
		t.Error("Expected Messages-Waiting: no when no new messages")
	}
}

func TestMWIManager_ReplaceExistingSubscription(t *testing.T) {
	mgr := NewMWIManager(slog.Default())

	// Add initial subscription
	mgr.AddSubscription(&MWISubscription{
		ID:         "sub-123",
		AOR:        "sip:user@example.com",
		ContactURI: "sip:old@192.168.1.1",
		Expires:    1800,
	})

	// Add another subscription with same ID (should replace)
	mgr.AddSubscription(&MWISubscription{
		ID:         "sub-123",
		AOR:        "sip:user@example.com",
		ContactURI: "sip:new@192.168.1.2",
		Expires:    3600,
	})

	if mgr.GetSubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription (replaced), got %d", mgr.GetSubscriptionCount())
	}

	sub := mgr.GetSubscription("sub-123")
	if sub.ContactURI != "sip:new@192.168.1.2" {
		t.Error("Expected subscription to be replaced with new contact")
	}
	if sub.Expires != 3600 {
		t.Errorf("Expected expires 3600, got %d", sub.Expires)
	}
}

// Note: uses contains() helper from transfer_test.go

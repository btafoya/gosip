package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/btafoya/gosip/internal/models"
)

func TestRouteRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create a DID first
	did := &models.DID{
		Number:       "+15551234567",
		VoiceEnabled: true,
	}
	if err := db.DIDs.Create(ctx, did); err != nil {
		t.Fatalf("Failed to create DID: %v", err)
	}

	conditionData, _ := json.Marshal(map[string]interface{}{
		"days":       []int{1, 2, 3, 4, 5},
		"start_time": "09:00",
		"end_time":   "17:00",
	})
	actionData, _ := json.Marshal(map[string]interface{}{
		"device_ids": []int64{1, 2},
		"timeout":    30,
	})

	route := &models.Route{
		DIDID:         &did.ID,
		Priority:      1,
		Name:          "Business Hours",
		ConditionType: "time",
		ConditionData: conditionData,
		ActionType:    "ring",
		ActionData:    actionData,
		Enabled:       true,
	}

	err := db.Routes.Create(ctx, route)
	if err != nil {
		t.Fatalf("Failed to create route: %v", err)
	}

	if route.ID == 0 {
		t.Error("Expected route ID to be set after creation")
	}
}

func TestRouteRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", VoiceEnabled: true}
	db.DIDs.Create(ctx, did)

	route := &models.Route{
		DIDID:         &did.ID,
		Priority:      1,
		Name:          "Test Route",
		ConditionType: "default",
		ActionType:    "voicemail",
		Enabled:       true,
	}
	if err := db.Routes.Create(ctx, route); err != nil {
		t.Fatalf("Failed to create route: %v", err)
	}

	retrieved, err := db.Routes.GetByID(ctx, route.ID)
	if err != nil {
		t.Fatalf("Failed to get route by ID: %v", err)
	}

	if retrieved.Name != route.Name {
		t.Errorf("Expected name %s, got %s", route.Name, retrieved.Name)
	}
	if retrieved.Priority != route.Priority {
		t.Errorf("Expected priority %d, got %d", route.Priority, retrieved.Priority)
	}
}

func TestRouteRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Routes.GetByID(ctx, 9999)
	if err != ErrRouteNotFound {
		t.Errorf("Expected ErrRouteNotFound, got %v", err)
	}
}

func TestRouteRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", VoiceEnabled: true}
	db.DIDs.Create(ctx, did)

	route := &models.Route{
		DIDID:         &did.ID,
		Priority:      1,
		Name:          "Original Name",
		ConditionType: "default",
		ActionType:    "ring",
		Enabled:       true,
	}
	if err := db.Routes.Create(ctx, route); err != nil {
		t.Fatalf("Failed to create route: %v", err)
	}

	// Update the route
	route.Name = "Updated Name"
	route.Enabled = false
	route.Priority = 2
	if err := db.Routes.Update(ctx, route); err != nil {
		t.Fatalf("Failed to update route: %v", err)
	}

	// Verify update
	retrieved, err := db.Routes.GetByID(ctx, route.ID)
	if err != nil {
		t.Fatalf("Failed to get updated route: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", retrieved.Name)
	}
	if retrieved.Enabled {
		t.Error("Expected Enabled to be false")
	}
	if retrieved.Priority != 2 {
		t.Errorf("Expected priority 2, got %d", retrieved.Priority)
	}
}

func TestRouteRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", VoiceEnabled: true}
	db.DIDs.Create(ctx, did)

	route := &models.Route{
		DIDID:         &did.ID,
		Priority:      1,
		Name:          "Delete Me",
		ConditionType: "default",
		ActionType:    "voicemail",
		Enabled:       true,
	}
	if err := db.Routes.Create(ctx, route); err != nil {
		t.Fatalf("Failed to create route: %v", err)
	}

	if err := db.Routes.Delete(ctx, route.ID); err != nil {
		t.Fatalf("Failed to delete route: %v", err)
	}

	_, err := db.Routes.GetByID(ctx, route.ID)
	if err != ErrRouteNotFound {
		t.Errorf("Expected ErrRouteNotFound after delete, got %v", err)
	}
}

func TestRouteRepository_GetByDID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create two DIDs
	did1 := &models.DID{Number: "+15551111111", VoiceEnabled: true}
	did2 := &models.DID{Number: "+15552222222", VoiceEnabled: true}
	db.DIDs.Create(ctx, did1)
	db.DIDs.Create(ctx, did2)

	// Create routes for did1
	routes := []struct {
		didID    *int64
		priority int
		name     string
	}{
		{&did1.ID, 1, "DID1 Route 1"},
		{&did1.ID, 2, "DID1 Route 2"},
		{&did1.ID, 3, "DID1 Route 3"},
		{&did2.ID, 1, "DID2 Route 1"},
	}

	for _, r := range routes {
		route := &models.Route{
			DIDID:         r.didID,
			Priority:      r.priority,
			Name:          r.name,
			ConditionType: "default",
			ActionType:    "ring",
			Enabled:       true,
		}
		if err := db.Routes.Create(ctx, route); err != nil {
			t.Fatalf("Failed to create route: %v", err)
		}
	}

	// Get routes for did1
	did1Routes, err := db.Routes.GetByDID(ctx, did1.ID)
	if err != nil {
		t.Fatalf("Failed to get routes by DID: %v", err)
	}

	if len(did1Routes) != 3 {
		t.Errorf("Expected 3 routes for DID1, got %d", len(did1Routes))
	}

	// Verify ordering by priority
	for i := 0; i < len(did1Routes)-1; i++ {
		if did1Routes[i].Priority > did1Routes[i+1].Priority {
			t.Error("Routes should be ordered by priority ASC")
		}
	}
}

func TestRouteRepository_GetEnabledByDID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", VoiceEnabled: true}
	db.DIDs.Create(ctx, did)

	// Create enabled and disabled routes
	routes := []struct {
		priority int
		enabled  bool
	}{
		{1, true},
		{2, false},
		{3, true},
		{4, false},
	}

	for i, r := range routes {
		route := &models.Route{
			DIDID:         &did.ID,
			Priority:      r.priority,
			Name:          "Route " + string(rune('A'+i)),
			ConditionType: "default",
			ActionType:    "ring",
			Enabled:       r.enabled,
		}
		if err := db.Routes.Create(ctx, route); err != nil {
			t.Fatalf("Failed to create route: %v", err)
		}
	}

	enabledRoutes, err := db.Routes.GetEnabledByDID(ctx, did.ID)
	if err != nil {
		t.Fatalf("Failed to get enabled routes: %v", err)
	}

	if len(enabledRoutes) != 2 {
		t.Errorf("Expected 2 enabled routes, got %d", len(enabledRoutes))
	}

	for _, r := range enabledRoutes {
		if !r.Enabled {
			t.Error("Found disabled route in enabled routes list")
		}
	}
}

func TestRouteRepository_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", VoiceEnabled: true}
	db.DIDs.Create(ctx, did)

	for i := 0; i < 5; i++ {
		route := &models.Route{
			DIDID:         &did.ID,
			Priority:      i + 1,
			Name:          "Route " + string(rune('A'+i)),
			ConditionType: "default",
			ActionType:    "ring",
			Enabled:       true,
		}
		if err := db.Routes.Create(ctx, route); err != nil {
			t.Fatalf("Failed to create route: %v", err)
		}
	}

	routes, err := db.Routes.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list routes: %v", err)
	}

	if len(routes) != 5 {
		t.Errorf("Expected 5 routes, got %d", len(routes))
	}
}

func TestRouteRepository_UpdatePriorities(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	did := &models.DID{Number: "+15551234567", VoiceEnabled: true}
	db.DIDs.Create(ctx, did)

	// Create routes
	var routeIDs []int64
	for i := 0; i < 3; i++ {
		route := &models.Route{
			DIDID:         &did.ID,
			Priority:      i + 1,
			Name:          "Route " + string(rune('A'+i)),
			ConditionType: "default",
			ActionType:    "ring",
			Enabled:       true,
		}
		if err := db.Routes.Create(ctx, route); err != nil {
			t.Fatalf("Failed to create route: %v", err)
		}
		routeIDs = append(routeIDs, route.ID)
	}

	// Update priorities (reverse order)
	priorities := map[int64]int{
		routeIDs[0]: 3,
		routeIDs[1]: 2,
		routeIDs[2]: 1,
	}

	if err := db.Routes.UpdatePriorities(ctx, priorities); err != nil {
		t.Fatalf("Failed to update priorities: %v", err)
	}

	// Verify updates
	for id, expectedPriority := range priorities {
		route, err := db.Routes.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("Failed to get route: %v", err)
		}
		if route.Priority != expectedPriority {
			t.Errorf("Route %d: expected priority %d, got %d", id, expectedPriority, route.Priority)
		}
	}
}

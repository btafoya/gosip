package rules

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

func createTestDID(t *testing.T, database *db.DB, number string) *models.DID {
	t.Helper()

	did := &models.DID{
		Number: number,
	}

	if err := database.DIDs.Create(context.Background(), did); err != nil {
		t.Fatalf("Failed to create test DID: %v", err)
	}

	return did
}

func createTestRoute(t *testing.T, database *db.DB, route *models.Route) *models.Route {
	t.Helper()

	if err := database.Routes.Create(context.Background(), route); err != nil {
		t.Fatalf("Failed to create test route: %v", err)
	}

	return route
}

func TestNewEngine(t *testing.T) {
	database := setupTestDB(t)

	t.Run("valid timezone", func(t *testing.T) {
		engine := NewEngine(database, "America/New_York")
		if engine == nil {
			t.Fatal("Engine should not be nil")
		}
		if engine.timezone.String() != "America/New_York" {
			t.Errorf("Expected America/New_York timezone, got %s", engine.timezone.String())
		}
	})

	t.Run("invalid timezone defaults to UTC", func(t *testing.T) {
		engine := NewEngine(database, "Invalid/Timezone")
		if engine == nil {
			t.Fatal("Engine should not be nil")
		}
		if engine.timezone.String() != "UTC" {
			t.Errorf("Expected UTC timezone for invalid input, got %s", engine.timezone.String())
		}
	})

	t.Run("UTC timezone", func(t *testing.T) {
		engine := NewEngine(database, "UTC")
		if engine == nil {
			t.Fatal("Engine should not be nil")
		}
		if engine.timezone.String() != "UTC" {
			t.Errorf("Expected UTC timezone, got %s", engine.timezone.String())
		}
	})
}

func TestNormalizeNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+1 (555) 123-4567", "+15551234567"},
		{"555.123.4567", "5551234567"},
		{"+1-555-123-4567", "+15551234567"},
		{"15551234567", "15551234567"},
		{"+15551234567", "+15551234567"},
		{"", ""},
		{"abc123def", "123"},
		{"+++111", "+++111"}, // Multiple plus signs are preserved
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeNumber(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeNumber(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSortRoutesByPriority(t *testing.T) {
	routes := []*models.Route{
		{ID: 1, Priority: 50},
		{ID: 2, Priority: 10},
		{ID: 3, Priority: 30},
		{ID: 4, Priority: 20},
	}

	sortRoutesByPriority(routes)

	expected := []int64{2, 4, 3, 1}
	for i, route := range routes {
		if route.ID != expected[i] {
			t.Errorf("Position %d: expected ID %d, got %d", i, expected[i], route.ID)
		}
	}
}

func TestEvaluateCallerIDCondition(t *testing.T) {
	database := setupTestDB(t)
	engine := NewEngine(database, "UTC")

	tests := []struct {
		name      string
		condition CallerIDCondition
		callerID  string
		expected  bool
	}{
		{
			name:      "exact match",
			condition: CallerIDCondition{Pattern: "+15551234567", MatchType: "exact"},
			callerID:  "+15551234567",
			expected:  true,
		},
		{
			name:      "exact match normalized",
			condition: CallerIDCondition{Pattern: "5551234567", MatchType: "exact"},
			callerID:  "+1 (555) 123-4567",
			expected:  false, // normalizes to +15551234567 vs 5551234567
		},
		{
			name:      "contains match",
			condition: CallerIDCondition{Pattern: "555", MatchType: "contains"},
			callerID:  "+15551234567",
			expected:  true,
		},
		{
			name:      "prefix match",
			condition: CallerIDCondition{Pattern: "+1555", MatchType: "prefix"},
			callerID:  "+15551234567",
			expected:  true,
		},
		{
			name:      "prefix no match",
			condition: CallerIDCondition{Pattern: "+1666", MatchType: "prefix"},
			callerID:  "+15551234567",
			expected:  false,
		},
		{
			name:      "regex match",
			condition: CallerIDCondition{Pattern: `^\+1555\d+$`, MatchType: "regex"},
			callerID:  "+15551234567",
			expected:  true,
		},
		{
			name:      "anonymous caller match",
			condition: CallerIDCondition{Anonymous: true},
			callerID:  "Anonymous",
			expected:  true,
		},
		{
			name:      "anonymous blocked caller",
			condition: CallerIDCondition{Anonymous: true},
			callerID:  "Blocked",
			expected:  true,
		},
		{
			name:      "anonymous private caller",
			condition: CallerIDCondition{Anonymous: true},
			callerID:  "Private Number",
			expected:  true,
		},
		{
			name:      "anonymous empty caller",
			condition: CallerIDCondition{Anonymous: true},
			callerID:  "",
			expected:  true,
		},
		{
			name:      "anonymous regular caller no match",
			condition: CallerIDCondition{Anonymous: true},
			callerID:  "+15551234567",
			expected:  false,
		},
		{
			name:      "default to contains",
			condition: CallerIDCondition{Pattern: "555"},
			callerID:  "+15551234567",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.condition)
			result := engine.evaluateCallerIDCondition(data, tt.callerID)
			if result != tt.expected {
				t.Errorf("evaluateCallerIDCondition() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluateTimeCondition(t *testing.T) {
	database := setupTestDB(t)
	engine := NewEngine(database, "UTC")

	// Wednesday 10:00 UTC
	wednesday10am := time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)
	// Saturday 14:00 UTC
	saturday2pm := time.Date(2024, 1, 13, 14, 0, 0, 0, time.UTC)
	// Wednesday 22:00 UTC
	wednesday10pm := time.Date(2024, 1, 10, 22, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		condition TimeCondition
		callTime  time.Time
		expected  bool
	}{
		{
			name:      "business hours - during business hours",
			condition: TimeCondition{BusinessHours: true},
			callTime:  wednesday10am,
			expected:  true,
		},
		{
			name:      "business hours - after hours",
			condition: TimeCondition{BusinessHours: true},
			callTime:  wednesday10pm,
			expected:  false,
		},
		{
			name:      "business hours - weekend",
			condition: TimeCondition{BusinessHours: true},
			callTime:  saturday2pm,
			expected:  false,
		},
		{
			name:      "after hours - during business",
			condition: TimeCondition{AfterHours: true},
			callTime:  wednesday10am,
			expected:  false,
		},
		{
			name:      "after hours - evening",
			condition: TimeCondition{AfterHours: true},
			callTime:  wednesday10pm,
			expected:  true,
		},
		{
			name:      "after hours - weekend",
			condition: TimeCondition{AfterHours: true},
			callTime:  saturday2pm,
			expected:  true,
		},
		{
			name:      "specific hours - match",
			condition: TimeCondition{StartHour: 9, EndHour: 17},
			callTime:  wednesday10am,
			expected:  true,
		},
		{
			name:      "specific hours - no match",
			condition: TimeCondition{StartHour: 9, EndHour: 17},
			callTime:  wednesday10pm,
			expected:  false,
		},
		{
			name:      "overnight hours - match evening",
			condition: TimeCondition{StartHour: 22, EndHour: 6},
			callTime:  wednesday10pm,
			expected:  true,
		},
		{
			name:      "specific days - match",
			condition: TimeCondition{Days: []int{3}, StartHour: 0, EndHour: 24}, // Wednesday
			callTime:  wednesday10am,
			expected:  true,
		},
		{
			name:      "specific days - no match",
			condition: TimeCondition{Days: []int{0, 6}, StartHour: 0, EndHour: 24}, // Sunday, Saturday
			callTime:  wednesday10am,
			expected:  false,
		},
		{
			name:      "weekend days - match",
			condition: TimeCondition{Days: []int{0, 6}, StartHour: 0, EndHour: 24}, // Sunday, Saturday
			callTime:  saturday2pm,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.condition)
			result := engine.evaluateTimeCondition(data, tt.callTime)
			if result != tt.expected {
				t.Errorf("evaluateTimeCondition() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEngine_Evaluate_Blocklist(t *testing.T) {
	database := setupTestDB(t)
	engine := NewEngine(database, "UTC")
	ctx := context.Background()

	did := createTestDID(t, database, "+15551234567")

	// Add caller to blocklist
	if err := database.Blocklist.Create(ctx, &models.BlocklistEntry{
		Pattern:     "+15559999999",
		PatternType: "exact",
		Reason:      "Spam",
	}); err != nil {
		t.Fatalf("Failed to create blocklist entry: %v", err)
	}

	callCtx := &CallContext{
		CallerID:     "+15559999999",
		CalledNumber: "+15551234567",
		DIDID:        did.ID,
		Time:         time.Now(),
	}

	action, err := engine.Evaluate(ctx, callCtx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if action.Type != "reject" {
		t.Errorf("Expected reject action for blocked caller, got %s", action.Type)
	}
	if action.RouteName != "Blocklist" {
		t.Errorf("Expected Blocklist route name, got %s", action.RouteName)
	}
}

func TestEngine_Evaluate_DefaultToVoicemail(t *testing.T) {
	database := setupTestDB(t)
	engine := NewEngine(database, "UTC")
	ctx := context.Background()

	did := createTestDID(t, database, "+15551234567")

	// No routes configured
	callCtx := &CallContext{
		CallerID:     "+15559876543",
		CalledNumber: "+15551234567",
		DIDID:        did.ID,
		Time:         time.Now(),
	}

	action, err := engine.Evaluate(ctx, callCtx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if action.Type != "voicemail" {
		t.Errorf("Expected voicemail action as default, got %s", action.Type)
	}
	if action.RouteName != "Default" {
		t.Errorf("Expected Default route name, got %s", action.RouteName)
	}
}

func TestEngine_Evaluate_MatchingRoute(t *testing.T) {
	database := setupTestDB(t)
	engine := NewEngine(database, "UTC")
	ctx := context.Background()

	did := createTestDID(t, database, "+15551234567")

	// Create a default ring route
	ringData, _ := json.Marshal(RingAction{Devices: []int64{1, 2}, Timeout: 30})
	route := &models.Route{
		Name:          "Ring All",
		DIDID:         &did.ID,
		Priority:      10,
		Enabled:       true,
		ConditionType: "default",
		ActionType:    "ring",
		ActionData:    ringData,
	}
	createTestRoute(t, database, route)

	callCtx := &CallContext{
		CallerID:     "+15559876543",
		CalledNumber: "+15551234567",
		DIDID:        did.ID,
		Time:         time.Now(),
	}

	action, err := engine.Evaluate(ctx, callCtx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if action.Type != "ring" {
		t.Errorf("Expected ring action, got %s", action.Type)
	}
	if action.RouteName != "Ring All" {
		t.Errorf("Expected 'Ring All' route name, got %s", action.RouteName)
	}
}

func TestEngine_Evaluate_PriorityOrder(t *testing.T) {
	database := setupTestDB(t)
	engine := NewEngine(database, "UTC")
	ctx := context.Background()

	did := createTestDID(t, database, "+15551234567")

	// Create routes with different priorities
	route1 := &models.Route{
		Name:          "Low Priority",
		DIDID:         &did.ID,
		Priority:      100,
		Enabled:       true,
		ConditionType: "default",
		ActionType:    "voicemail",
	}
	createTestRoute(t, database, route1)

	route2 := &models.Route{
		Name:          "High Priority",
		DIDID:         &did.ID,
		Priority:      10,
		Enabled:       true,
		ConditionType: "default",
		ActionType:    "ring",
	}
	createTestRoute(t, database, route2)

	callCtx := &CallContext{
		CallerID:     "+15559876543",
		CalledNumber: "+15551234567",
		DIDID:        did.ID,
		Time:         time.Now(),
	}

	action, err := engine.Evaluate(ctx, callCtx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Higher priority (lower number) should win
	if action.RouteName != "High Priority" {
		t.Errorf("Expected 'High Priority' route (priority 10), got %s", action.RouteName)
	}
}

func TestValidateRule(t *testing.T) {
	tests := []struct {
		name           string
		route          *models.Route
		expectedErrors int
	}{
		{
			name: "valid default rule",
			route: &models.Route{
				ConditionType: "default",
				ActionType:    "ring",
				ActionData:    json.RawMessage(`{"devices":[1],"timeout":30}`),
			},
			expectedErrors: 0,
		},
		{
			name: "invalid condition type",
			route: &models.Route{
				ConditionType: "invalid",
				ActionType:    "ring",
			},
			expectedErrors: 1,
		},
		{
			name: "invalid action type",
			route: &models.Route{
				ConditionType: "default",
				ActionType:    "invalid",
			},
			expectedErrors: 1,
		},
		{
			name: "ring without devices",
			route: &models.Route{
				ConditionType: "default",
				ActionType:    "ring",
				ActionData:    json.RawMessage(`{"devices":[],"timeout":30}`),
			},
			expectedErrors: 1,
		},
		{
			name: "ring with invalid timeout",
			route: &models.Route{
				ConditionType: "default",
				ActionType:    "ring",
				ActionData:    json.RawMessage(`{"devices":[1],"timeout":500}`),
			},
			expectedErrors: 1,
		},
		{
			name: "forward without number",
			route: &models.Route{
				ConditionType: "default",
				ActionType:    "forward",
				ActionData:    json.RawMessage(`{"number":""}`),
			},
			expectedErrors: 1,
		},
		{
			name: "valid forward",
			route: &models.Route{
				ConditionType: "default",
				ActionType:    "forward",
				ActionData:    json.RawMessage(`{"number":"+15551234567"}`),
			},
			expectedErrors: 0,
		},
		{
			name: "invalid time condition hours",
			route: &models.Route{
				ConditionType: "time",
				ConditionData: json.RawMessage(`{"start_hour":25,"end_hour":17}`),
				ActionType:    "ring",
			},
			expectedErrors: 1,
		},
		{
			name: "invalid time condition day",
			route: &models.Route{
				ConditionType: "time",
				ConditionData: json.RawMessage(`{"days":[7]}`),
				ActionType:    "ring",
			},
			expectedErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateRule(tt.route)
			if len(errors) != tt.expectedErrors {
				t.Errorf("ValidateRule() returned %d errors, want %d: %v", len(errors), tt.expectedErrors, errors)
			}
		})
	}
}

func TestParseAction(t *testing.T) {
	tests := []struct {
		name       string
		action     *Action
		shouldWork bool
	}{
		{
			name: "ring action",
			action: &Action{
				Type: "ring",
				Data: json.RawMessage(`{"devices":[1,2],"timeout":30}`),
			},
			shouldWork: true,
		},
		{
			name: "forward action",
			action: &Action{
				Type: "forward",
				Data: json.RawMessage(`{"number":"+15551234567"}`),
			},
			shouldWork: true,
		},
		{
			name: "voicemail action",
			action: &Action{
				Type: "voicemail",
			},
			shouldWork: true,
		},
		{
			name: "reject action",
			action: &Action{
				Type: "reject",
			},
			shouldWork: true,
		},
		{
			name: "invalid ring action data",
			action: &Action{
				Type: "ring",
				Data: json.RawMessage(`{invalid json}`),
			},
			shouldWork: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAction(tt.action)
			if tt.shouldWork && err != nil {
				t.Errorf("ParseAction() error = %v, expected no error", err)
			}
			if !tt.shouldWork && err == nil && result != nil {
				t.Error("ParseAction() should have returned error")
			}

			// Verify parsed types
			if err == nil && result != nil {
				switch tt.action.Type {
				case "ring":
					if _, ok := result.(*RingAction); !ok {
						t.Error("Expected RingAction type")
					}
				case "forward":
					if _, ok := result.(*ForwardAction); !ok {
						t.Error("Expected ForwardAction type")
					}
				}
			}
		})
	}
}

func TestGetPresetRules(t *testing.T) {
	presets := GetPresetRules()

	if len(presets) == 0 {
		t.Error("Should have preset rules")
	}

	// Verify all presets have required fields
	for _, preset := range presets {
		if preset.Name == "" {
			t.Error("Preset should have a name")
		}
		if preset.Description == "" {
			t.Error("Preset should have a description")
		}
		if preset.ConditionType == "" {
			t.Error("Preset should have a condition type")
		}
		if preset.ActionType == "" {
			t.Error("Preset should have an action type")
		}
	}

	// Check for expected presets
	expectedNames := []string{"Block Anonymous", "After Hours Voicemail", "Weekend Voicemail", "Business Hours Ring"}
	for _, name := range expectedNames {
		found := false
		for _, preset := range presets {
			if preset.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected preset %q not found", name)
		}
	}
}

func TestCallContext(t *testing.T) {
	ctx := &CallContext{
		CallerID:     "+15551234567",
		CalledNumber: "+15559876543",
		DIDID:        1,
		Time:         time.Now(),
	}

	if ctx.CallerID != "+15551234567" {
		t.Errorf("CallerID mismatch")
	}
	if ctx.CalledNumber != "+15559876543" {
		t.Errorf("CalledNumber mismatch")
	}
	if ctx.DIDID != 1 {
		t.Errorf("DIDID mismatch")
	}
}

func TestAction(t *testing.T) {
	action := &Action{
		Type:      "ring",
		Data:      json.RawMessage(`{"devices":[1]}`),
		RouteName: "Test Route",
		Priority:  10,
	}

	if action.Type != "ring" {
		t.Errorf("Type mismatch")
	}
	if action.RouteName != "Test Route" {
		t.Errorf("RouteName mismatch")
	}
	if action.Priority != 10 {
		t.Errorf("Priority mismatch")
	}
}

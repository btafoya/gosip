package rules

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/btafoya/gosip/internal/db"
	"github.com/btafoya/gosip/internal/models"
)

// Engine evaluates call routing rules and determines actions
type Engine struct {
	database *db.DB
	timezone *time.Location
}

// NewEngine creates a new rules engine
func NewEngine(database *db.DB, timezone string) *Engine {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	return &Engine{
		database: database,
		timezone: loc,
	}
}

// CallContext contains information about an incoming call for rule evaluation
type CallContext struct {
	CallerID     string
	CalledNumber string
	DIDID        int64
	Time         time.Time
}

// Action represents the action to take for a call
type Action struct {
	Type       string          // ring, forward, voicemail, reject
	Data       json.RawMessage // Action-specific data
	RouteName  string          // Name of the matching route for logging
	Priority   int             // Priority of the matching rule
}

// RingAction contains data for the "ring" action
type RingAction struct {
	Devices []int64 `json:"devices"`
	Timeout int     `json:"timeout"`
}

// ForwardAction contains data for the "forward" action
type ForwardAction struct {
	Number string `json:"number"`
}

// Evaluate evaluates all rules for the given call context and returns the action
func (e *Engine) Evaluate(ctx context.Context, callCtx *CallContext) (*Action, error) {
	// Check blocklist first
	isBlocked, _, err := e.database.Blocklist.IsBlocked(ctx, callCtx.CallerID)
	if err == nil && isBlocked {
		return &Action{
			Type:      "reject",
			RouteName: "Blocklist",
		}, nil
	}

	// Get active routes for this DID, ordered by priority
	routes, err := e.database.Routes.GetEnabledByDID(ctx, callCtx.DIDID)
	if err != nil {
		return nil, err
	}

	// Also get global routes (no DID specified) - get all and filter
	allRoutes, err := e.database.Routes.List(ctx)
	if err == nil {
		for _, route := range allRoutes {
			if route.DIDID == nil && route.Enabled {
				routes = append(routes, route)
			}
		}
	}

	// Sort by priority (lower number = higher priority)
	sortRoutesByPriority(routes)

	// Evaluate each rule
	for _, route := range routes {
		if e.evaluateCondition(route, callCtx) {
			return &Action{
				Type:      route.ActionType,
				Data:      route.ActionData,
				RouteName: route.Name,
				Priority:  route.Priority,
			}, nil
		}
	}

	// Default action: voicemail
	return &Action{
		Type:      "voicemail",
		RouteName: "Default",
	}, nil
}

func (e *Engine) evaluateCondition(route *models.Route, callCtx *CallContext) bool {
	switch route.ConditionType {
	case "default":
		return true

	case "callerid":
		return e.evaluateCallerIDCondition(route.ConditionData, callCtx.CallerID)

	case "time":
		return e.evaluateTimeCondition(route.ConditionData, callCtx.Time)

	default:
		return false
	}
}

// CallerIDCondition defines caller ID matching rules
type CallerIDCondition struct {
	Pattern     string `json:"pattern"`
	MatchType   string `json:"match_type"` // exact, contains, prefix, regex
	Anonymous   bool   `json:"anonymous"`  // Match anonymous/blocked callers
}

func (e *Engine) evaluateCallerIDCondition(data json.RawMessage, callerID string) bool {
	var condition CallerIDCondition
	if err := json.Unmarshal(data, &condition); err != nil {
		return false
	}

	// Check for anonymous caller
	if condition.Anonymous {
		anonymousPatterns := []string{"anonymous", "blocked", "private", "unavailable", "unknown", "restricted"}
		callerLower := strings.ToLower(callerID)
		for _, pattern := range anonymousPatterns {
			if strings.Contains(callerLower, pattern) || callerID == "" {
				return true
			}
		}
		return false
	}

	// Match based on match type
	switch condition.MatchType {
	case "exact":
		return normalizeNumber(callerID) == normalizeNumber(condition.Pattern)

	case "contains":
		return strings.Contains(normalizeNumber(callerID), normalizeNumber(condition.Pattern))

	case "prefix":
		return strings.HasPrefix(normalizeNumber(callerID), normalizeNumber(condition.Pattern))

	case "regex":
		matched, _ := regexp.MatchString(condition.Pattern, callerID)
		return matched

	default:
		// Default to contains
		return strings.Contains(normalizeNumber(callerID), normalizeNumber(condition.Pattern))
	}
}

// TimeCondition defines time-based routing rules
type TimeCondition struct {
	StartHour   int   `json:"start_hour"`   // 0-23
	EndHour     int   `json:"end_hour"`     // 0-23
	Days        []int `json:"days"`         // 0=Sunday, 6=Saturday
	BusinessHours bool `json:"business_hours"` // Use system business hours
	AfterHours   bool `json:"after_hours"`    // Inverse of business hours
}

func (e *Engine) evaluateTimeCondition(data json.RawMessage, callTime time.Time) bool {
	var condition TimeCondition
	if err := json.Unmarshal(data, &condition); err != nil {
		return false
	}

	// Convert to configured timezone
	localTime := callTime.In(e.timezone)
	hour := localTime.Hour()
	weekday := int(localTime.Weekday())

	// Check business hours shortcut
	if condition.BusinessHours || condition.AfterHours {
		// Default business hours: Monday-Friday 9am-5pm
		isBusinessHours := weekday >= 1 && weekday <= 5 && hour >= 9 && hour < 17

		if condition.BusinessHours {
			return isBusinessHours
		}
		return !isBusinessHours
	}

	// Check specific days
	if len(condition.Days) > 0 {
		dayMatch := false
		for _, d := range condition.Days {
			if d == weekday {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Check time range
	if condition.StartHour <= condition.EndHour {
		// Same day range (e.g., 9am-5pm)
		return hour >= condition.StartHour && hour < condition.EndHour
	} else {
		// Overnight range (e.g., 10pm-6am)
		return hour >= condition.StartHour || hour < condition.EndHour
	}
}

// ParseAction parses action data into the appropriate struct
func ParseAction(action *Action) (interface{}, error) {
	switch action.Type {
	case "ring":
		var ringAction RingAction
		if err := json.Unmarshal(action.Data, &ringAction); err != nil {
			return nil, err
		}
		return &ringAction, nil

	case "forward":
		var forwardAction ForwardAction
		if err := json.Unmarshal(action.Data, &forwardAction); err != nil {
			return nil, err
		}
		return &forwardAction, nil

	case "voicemail", "reject":
		return nil, nil

	default:
		return nil, nil
	}
}

// Helper functions

func normalizeNumber(number string) string {
	// Remove non-digit characters except +
	var normalized strings.Builder
	for _, r := range number {
		if r >= '0' && r <= '9' || r == '+' {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}

func sortRoutesByPriority(routes []*models.Route) {
	// Simple bubble sort for small arrays
	for i := 0; i < len(routes)-1; i++ {
		for j := 0; j < len(routes)-i-1; j++ {
			if routes[j].Priority > routes[j+1].Priority {
				routes[j], routes[j+1] = routes[j+1], routes[j]
			}
		}
	}
}

// ValidateRule validates a routing rule configuration
func ValidateRule(route *models.Route) []string {
	var errors []string

	// Validate condition type
	validConditions := map[string]bool{"default": true, "callerid": true, "time": true}
	if !validConditions[route.ConditionType] {
		errors = append(errors, "Invalid condition type: "+route.ConditionType)
	}

	// Validate action type
	validActions := map[string]bool{"ring": true, "forward": true, "voicemail": true, "reject": true}
	if !validActions[route.ActionType] {
		errors = append(errors, "Invalid action type: "+route.ActionType)
	}

	// Validate condition data
	if route.ConditionType == "time" && len(route.ConditionData) > 0 {
		var condition TimeCondition
		if err := json.Unmarshal(route.ConditionData, &condition); err != nil {
			errors = append(errors, "Invalid time condition data: "+err.Error())
		} else {
			if condition.StartHour < 0 || condition.StartHour > 23 {
				errors = append(errors, "Start hour must be between 0 and 23")
			}
			if condition.EndHour < 0 || condition.EndHour > 23 {
				errors = append(errors, "End hour must be between 0 and 23")
			}
			for _, day := range condition.Days {
				if day < 0 || day > 6 {
					errors = append(errors, "Day must be between 0 (Sunday) and 6 (Saturday)")
				}
			}
		}
	}

	// Validate action data
	if route.ActionType == "ring" && len(route.ActionData) > 0 {
		var action RingAction
		if err := json.Unmarshal(route.ActionData, &action); err != nil {
			errors = append(errors, "Invalid ring action data: "+err.Error())
		} else {
			if len(action.Devices) == 0 {
				errors = append(errors, "Ring action requires at least one device")
			}
			if action.Timeout < 0 || action.Timeout > 300 {
				errors = append(errors, "Timeout must be between 0 and 300 seconds")
			}
		}
	}

	if route.ActionType == "forward" && len(route.ActionData) > 0 {
		var action ForwardAction
		if err := json.Unmarshal(route.ActionData, &action); err != nil {
			errors = append(errors, "Invalid forward action data: "+err.Error())
		} else {
			if action.Number == "" {
				errors = append(errors, "Forward action requires a phone number")
			}
		}
	}

	return errors
}

// PresetRule represents a preset routing rule template
type PresetRule struct {
	Name          string
	Description   string
	ConditionType string
	ConditionData json.RawMessage
	ActionType    string
	ActionData    json.RawMessage
}

// GetPresetRules returns common preset routing rules
func GetPresetRules() []PresetRule {
	return []PresetRule{
		{
			Name:          "Block Anonymous",
			Description:   "Reject calls from anonymous or blocked callers",
			ConditionType: "callerid",
			ConditionData: json.RawMessage(`{"anonymous": true}`),
			ActionType:    "reject",
		},
		{
			Name:          "After Hours Voicemail",
			Description:   "Send calls to voicemail outside business hours",
			ConditionType: "time",
			ConditionData: json.RawMessage(`{"after_hours": true}`),
			ActionType:    "voicemail",
		},
		{
			Name:          "Weekend Voicemail",
			Description:   "Send weekend calls to voicemail",
			ConditionType: "time",
			ConditionData: json.RawMessage(`{"days": [0, 6]}`),
			ActionType:    "voicemail",
		},
		{
			Name:          "Business Hours Ring",
			Description:   "Ring devices during business hours",
			ConditionType: "time",
			ConditionData: json.RawMessage(`{"business_hours": true}`),
			ActionType:    "ring",
			ActionData:    json.RawMessage(`{"timeout": 30}`),
		},
	}
}

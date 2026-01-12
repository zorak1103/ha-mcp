// Package homeassistant provides a WebSocket client for the Home Assistant API.
package homeassistant

import (
	"context"
	"fmt"
	"time"
)

// Service name constants used across multiple methods.
const (
	serviceTurnOn   = "turn_on"
	serviceTurnOff  = "turn_off"
	serviceSetValue = "set_value"
)

// Client defines the interface for Home Assistant operations.
// All operations are performed via WebSocket connection.
type Client interface {
	// Entity operations
	GetStates(ctx context.Context) ([]Entity, error)
	GetState(ctx context.Context, entityID string) (*Entity, error)
	SetState(ctx context.Context, entityID string, state StateUpdate) (*Entity, error)

	// History operations
	GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error)

	// Automation operations
	ListAutomations(ctx context.Context) ([]Automation, error)
	GetAutomation(ctx context.Context, automationID string) (*Automation, error)
	CreateAutomation(ctx context.Context, automation AutomationConfig) error
	UpdateAutomation(ctx context.Context, automationID string, automation AutomationConfig) error
	DeleteAutomation(ctx context.Context, automationID string) error
	ToggleAutomation(ctx context.Context, entityID string, enabled bool) error

	// Helper operations
	ListHelpers(ctx context.Context) ([]Entity, error)
	CreateHelper(ctx context.Context, helper HelperConfig) error
	UpdateHelper(ctx context.Context, helperID string, helper HelperConfig) error
	DeleteHelper(ctx context.Context, helperID string) error
	SetHelperValue(ctx context.Context, entityID string, value any) error

	// Script operations
	ListScripts(ctx context.Context) ([]Entity, error)
	GetScript(ctx context.Context, scriptID string) (*Script, error)
	CreateScript(ctx context.Context, scriptID string, script ScriptConfig) error
	UpdateScript(ctx context.Context, scriptID string, script ScriptConfig) error
	DeleteScript(ctx context.Context, scriptID string) error

	// Scene operations
	ListScenes(ctx context.Context) ([]Entity, error)
	CreateScene(ctx context.Context, sceneID string, scene SceneConfig) error
	UpdateScene(ctx context.Context, sceneID string, scene SceneConfig) error
	DeleteScene(ctx context.Context, sceneID string) error

	// Service operations
	CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error)

	// Registry operations
	GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error)
	GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error)
	GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error)

	// Media operations
	SignPath(ctx context.Context, path string, expires int) (string, error)
	GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error)
	BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error)

	// Configuration operations
	GetLovelaceConfig(ctx context.Context) (map[string]any, error)

	// Statistics operations
	GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error)

	// Target operations - get applicable triggers, conditions, and services for targets
	GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error)

	// Config operations - get full configuration for helpers
	GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error)
}

// APIError represents an error response from the Home Assistant API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Home Assistant API error (status %d): %s", e.StatusCode, e.Message)
}

// Helper functions

// getStringAttr safely extracts a string value from an attributes map.
// Returns an empty string if the key doesn't exist or the value is not a string.
func getStringAttr(attrs map[string]any, key string) string {
	if v, ok := attrs[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractPlatform determines the helper platform type from an entity ID.
// It checks for input_boolean, input_number, input_text, input_select, and input_datetime prefixes.
// Returns an empty string if no matching platform is found.
func extractPlatform(entityID string) string {
	platforms := []string{"input_boolean", "input_number", "input_text", "input_select", "input_datetime"}
	for _, p := range platforms {
		if len(entityID) > len(p)+1 && entityID[:len(p)+1] == p+"." {
			return p
		}
	}
	return ""
}

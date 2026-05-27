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
	GetScene(ctx context.Context, sceneID string) (*Scene, error)
	CreateScene(ctx context.Context, sceneID string, scene SceneConfig) error
	UpdateScene(ctx context.Context, sceneID string, scene SceneConfig) error
	DeleteScene(ctx context.Context, sceneID string) error

	// Service operations
	CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error)

	// Registry operations
	GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error)
	GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error)
	GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error)

	// Area registry modification operations
	CreateArea(ctx context.Context, config AreaConfig) (*AreaRegistryEntry, error)
	UpdateArea(ctx context.Context, areaID string, config AreaConfig) (*AreaRegistryEntry, error)
	DeleteArea(ctx context.Context, areaID string) error

	// Label registry operations
	GetLabelRegistry(ctx context.Context) ([]LabelRegistryEntry, error)
	CreateLabel(ctx context.Context, config LabelConfig) (*LabelRegistryEntry, error)
	UpdateLabel(ctx context.Context, labelID string, config LabelConfig) (*LabelRegistryEntry, error)
	DeleteLabel(ctx context.Context, labelID string) error

	// Floor registry operations
	GetFloorRegistry(ctx context.Context) ([]FloorRegistryEntry, error)
	CreateFloor(ctx context.Context, config FloorConfig) (*FloorRegistryEntry, error)
	UpdateFloor(ctx context.Context, floorID string, config FloorConfig) (*FloorRegistryEntry, error)
	DeleteFloor(ctx context.Context, floorID string) error

	// Zone operations
	GetZones(ctx context.Context) ([]ZoneRegistryEntry, error)
	CreateZone(ctx context.Context, config ZoneConfig) (*ZoneRegistryEntry, error)
	UpdateZone(ctx context.Context, zoneID string, config ZoneConfig) (*ZoneRegistryEntry, error)
	DeleteZone(ctx context.Context, zoneID string) error

	// Person registry operations
	GetPersons(ctx context.Context) ([]PersonRegistryEntry, error)
	CreatePerson(ctx context.Context, config PersonConfig) (*PersonRegistryEntry, error)
	UpdatePerson(ctx context.Context, personID string, config PersonConfig) (*PersonRegistryEntry, error)
	DeletePerson(ctx context.Context, personID string) error

	// Tag registry operations
	GetTags(ctx context.Context) ([]TagRegistryEntry, error)
	CreateTag(ctx context.Context, config TagConfig) (*TagRegistryEntry, error)
	UpdateTag(ctx context.Context, tagID string, config TagConfig) (*TagRegistryEntry, error)
	DeleteTag(ctx context.Context, tagID string) error

	// Entity registry modification operations
	RemoveEntityRegistryEntry(ctx context.Context, entityID string) error
	UpdateEntityRegistryEntry(ctx context.Context, entityID string, config EntityRegistryUpdateConfig) (*EntityRegistryEntry, error)

	// Device registry modification operations
	RemoveDeviceConfigEntry(ctx context.Context, deviceID, configEntryID string) error
	UpdateDeviceRegistryEntry(ctx context.Context, deviceID string, config DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error)

	// Media operations
	SignPath(ctx context.Context, path string, expires int) (string, error)
	GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error)
	BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error)

	// Dashboard operations
	GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error)
	SaveLovelaceConfig(ctx context.Context, urlPath string, config map[string]any) error
	ListDashboards(ctx context.Context) ([]DashboardEntry, error)
	CreateDashboard(ctx context.Context, config DashboardConfig) (*DashboardEntry, error)
	UpdateDashboard(ctx context.Context, dashboardID string, config DashboardConfig) (*DashboardEntry, error)
	DeleteDashboard(ctx context.Context, dashboardID string) error

	// Statistics operations
	GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error)

	// Target operations - get applicable triggers, conditions, and services for targets
	GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error)

	// Config operations - get full configuration for helpers
	GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error)

	// Config entry operations - get config entries with full details
	GetConfigEntries(ctx context.Context, domain string) ([]ConfigEntryFull, error)
	GetConfigEntry(ctx context.Context, entryID string) (*ConfigEntryFull, error)
	GetConfigEntryOptions(ctx context.Context, entryID string) (map[string]any, error)

	// Service discovery operations
	GetServices(ctx context.Context) ([]Service, error)

	// System configuration operations
	GetConfig(ctx context.Context) (*Config, error)

	// Template operations
	RenderTemplate(ctx context.Context, template string) (string, error)

	// Logbook operations
	GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error)

	// Configuration validation operations
	CheckConfig(ctx context.Context) (*ConfigCheckResult, error)

	// HACS operations
	SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error)

	// Service call with response
	CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error)

	// Calendar operations
	GetCalendars(ctx context.Context) ([]CalendarEntry, error)
	GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error)

	// Camera operations
	GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error)

	// System log operations
	GetSystemLog(ctx context.Context) ([]SystemLogEntry, error)
	ClearSystemLog(ctx context.Context) error
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
// It checks for all supported helper platform prefixes.
// Returns an empty string if no matching platform is found.
func extractPlatform(entityID string) string {
	platforms := []string{
		"input_boolean", "input_number", "input_text", "input_select", "input_datetime", "input_button",
		"counter", "timer", "schedule", "group",
	}
	for _, p := range platforms {
		if len(entityID) > len(p)+1 && entityID[:len(p)+1] == p+"." {
			return p
		}
	}
	return ""
}

// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// UniversalMockClient is a flexible mock for all handler tests.
// It implements the homeassistant.Client interface with configurable function hooks.
// If a hook is nil, the method returns a sensible default (empty slice or zero value).
type UniversalMockClient struct {
	homeassistant.Client

	// Entity operations
	GetStatesFn func(ctx context.Context) ([]homeassistant.Entity, error)
	GetStateFn  func(ctx context.Context, entityID string) (*homeassistant.Entity, error)
	SetStateFn  func(ctx context.Context, entityID string, state homeassistant.StateUpdate) (*homeassistant.Entity, error)

	// History operations
	GetHistoryFn func(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error)

	// Automation operations
	ListAutomationsFn  func(ctx context.Context) ([]homeassistant.Automation, error)
	GetAutomationFn    func(ctx context.Context, automationID string) (*homeassistant.Automation, error)
	CreateAutomationFn func(ctx context.Context, config homeassistant.AutomationConfig) error
	UpdateAutomationFn func(ctx context.Context, automationID string, config homeassistant.AutomationConfig) error
	DeleteAutomationFn func(ctx context.Context, automationID string) error
	ToggleAutomationFn func(ctx context.Context, entityID string, enabled bool) error

	// Helper operations
	ListHelpersFn    func(ctx context.Context) ([]homeassistant.Entity, error)
	CreateHelperFn   func(ctx context.Context, helper homeassistant.HelperConfig) error
	UpdateHelperFn   func(ctx context.Context, helperID string, helper homeassistant.HelperConfig) error
	DeleteHelperFn   func(ctx context.Context, helperID string) error
	SetHelperValueFn func(ctx context.Context, entityID string, value any) error

	// Script operations
	ListScriptsFn  func(ctx context.Context) ([]homeassistant.Entity, error)
	GetScriptFn    func(ctx context.Context, scriptID string) (*homeassistant.Script, error)
	CreateScriptFn func(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error
	UpdateScriptFn func(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error
	DeleteScriptFn func(ctx context.Context, scriptID string) error

	// Scene operations
	ListScenesFn  func(ctx context.Context) ([]homeassistant.Entity, error)
	GetSceneFn    func(ctx context.Context, sceneID string) (*homeassistant.Scene, error)
	CreateSceneFn func(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error
	UpdateSceneFn func(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error
	DeleteSceneFn func(ctx context.Context, sceneID string) error

	ConfigFileEntryExistsFn func(ctx context.Context, domain, configID string) (bool, error)

	// Service operations
	CallServiceFn             func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
	CallServiceWithResponseFn func(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error)

	// Calendar operations
	GetCalendarsFn      func(ctx context.Context) ([]homeassistant.CalendarEntry, error)
	GetCalendarEventsFn func(ctx context.Context, entityID, start, end string) ([]homeassistant.CalendarEvent, error)

	// Camera operations
	GetCameraSnapshotFn func(ctx context.Context, entityID string) ([]byte, string, error)

	// Registry operations
	GetEntityRegistryFn         func(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error)
	GetDeviceRegistryFn         func(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error)
	GetAreaRegistryFn           func(ctx context.Context) ([]homeassistant.AreaRegistryEntry, error)
	CreateAreaFn                func(ctx context.Context, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error)
	UpdateAreaFn                func(ctx context.Context, areaID string, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error)
	DeleteAreaFn                func(ctx context.Context, areaID string) error
	GetLabelRegistryFn          func(ctx context.Context) ([]homeassistant.LabelRegistryEntry, error)
	CreateLabelFn               func(ctx context.Context, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error)
	UpdateLabelFn               func(ctx context.Context, labelID string, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error)
	DeleteLabelFn               func(ctx context.Context, labelID string) error
	GetFloorRegistryFn          func(ctx context.Context) ([]homeassistant.FloorRegistryEntry, error)
	CreateFloorFn               func(ctx context.Context, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error)
	UpdateFloorFn               func(ctx context.Context, floorID string, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error)
	DeleteFloorFn               func(ctx context.Context, floorID string) error
	GetZonesFn                  func(ctx context.Context) ([]homeassistant.ZoneRegistryEntry, error)
	CreateZoneFn                func(ctx context.Context, config homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error)
	UpdateZoneFn                func(ctx context.Context, zoneID string, config homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error)
	DeleteZoneFn                func(ctx context.Context, zoneID string) error
	GetPersonsFn                func(ctx context.Context) ([]homeassistant.PersonRegistryEntry, error)
	CreatePersonFn              func(ctx context.Context, config homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error)
	UpdatePersonFn              func(ctx context.Context, personID string, config homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error)
	DeletePersonFn              func(ctx context.Context, personID string) error
	GetTagsFn                   func(ctx context.Context) ([]homeassistant.TagRegistryEntry, error)
	CreateTagFn                 func(ctx context.Context, config homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error)
	UpdateTagFn                 func(ctx context.Context, tagID string, config homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error)
	DeleteTagFn                 func(ctx context.Context, tagID string) error
	RemoveEntityRegistryEntryFn func(ctx context.Context, entityID string) error
	UpdateEntityRegistryEntryFn func(ctx context.Context, entityID string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error)
	RemoveDeviceConfigEntryFn   func(ctx context.Context, deviceID, configEntryID string) error
	UpdateDeviceRegistryEntryFn func(ctx context.Context, deviceID string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error)

	// Media operations
	SignPathFn        func(ctx context.Context, path string, expires int) (string, error)
	GetCameraStreamFn func(ctx context.Context, entityID string) (*homeassistant.StreamInfo, error)
	BrowseMediaFn     func(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error)

	// Dashboard operations
	GetLovelaceConfigFn  func(ctx context.Context, urlPath string) (map[string]any, error)
	SaveLovelaceConfigFn func(ctx context.Context, urlPath string, config map[string]any) error
	ListDashboardsFn     func(ctx context.Context) ([]homeassistant.DashboardEntry, error)
	CreateDashboardFn    func(ctx context.Context, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error)
	UpdateDashboardFn    func(ctx context.Context, dashboardID string, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error)
	DeleteDashboardFn    func(ctx context.Context, dashboardID string) error

	// Statistics operations
	GetStatisticsFn func(ctx context.Context, statIDs []string, period string) ([]homeassistant.StatisticsResult, error)

	// Target operations
	GetTriggersForTargetFn   func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	GetConditionsForTargetFn func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	GetServicesForTargetFn   func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	ExtractFromTargetFn      func(ctx context.Context, target homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error)

	// Config operations
	GetHelperConfigFn func(ctx context.Context, platform, entityID string) (map[string]any, error)

	// Config entry operations
	GetConfigEntriesFn      func(ctx context.Context, domain string) ([]homeassistant.ConfigEntryFull, error)
	GetConfigEntryFn        func(ctx context.Context, entryID string) (*homeassistant.ConfigEntryFull, error)
	GetConfigEntryOptionsFn func(ctx context.Context, entryID string) (map[string]any, error)

	// Config entry delete operations
	DeleteConfigEntryFn func(ctx context.Context, entryID string) (bool, error)

	// Service discovery operations
	GetServicesFn func(ctx context.Context) ([]homeassistant.Service, error)

	// System configuration operations
	GetConfigFn func(ctx context.Context) (*homeassistant.Config, error)

	// Template operations
	RenderTemplateFn func(ctx context.Context, template string) (string, error)

	// Logbook operations
	GetLogbookFn func(ctx context.Context, startTime, endTime, entityID string) ([]homeassistant.LogbookEntry, error)

	// System log operations
	GetSystemLogFn   func(ctx context.Context) ([]homeassistant.SystemLogEntry, error)
	ClearSystemLogFn func(ctx context.Context) error

	// Configuration validation operations
	CheckConfigFn func(ctx context.Context) (*homeassistant.ConfigCheckResult, error)

	// HACS operations
	SendHACSCommandFn func(ctx context.Context, command string, data map[string]any) (any, error)
}

// Entity operations implementation

func (m *UniversalMockClient) GetStates(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.GetStatesFn != nil {
		return m.GetStatesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	if m.GetStateFn != nil {
		return m.GetStateFn(ctx, entityID)
	}
	return &homeassistant.Entity{EntityID: entityID, State: "unknown"}, nil
}

func (m *UniversalMockClient) SetState(ctx context.Context, entityID string, state homeassistant.StateUpdate) (*homeassistant.Entity, error) {
	if m.SetStateFn != nil {
		return m.SetStateFn(ctx, entityID, state)
	}
	return &homeassistant.Entity{EntityID: entityID, State: "updated"}, nil
}

// History operations implementation

func (m *UniversalMockClient) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error) {
	if m.GetHistoryFn != nil {
		return m.GetHistoryFn(ctx, entityID, start, end)
	}
	return [][]homeassistant.HistoryEntry{}, nil
}

// Automation operations implementation

func (m *UniversalMockClient) ListAutomations(ctx context.Context) ([]homeassistant.Automation, error) {
	if m.ListAutomationsFn != nil {
		return m.ListAutomationsFn(ctx)
	}
	return []homeassistant.Automation{}, nil
}

func (m *UniversalMockClient) GetAutomation(ctx context.Context, automationID string) (*homeassistant.Automation, error) {
	if m.GetAutomationFn != nil {
		return m.GetAutomationFn(ctx, automationID)
	}
	return &homeassistant.Automation{EntityID: "automation." + automationID}, nil
}

func (m *UniversalMockClient) CreateAutomation(ctx context.Context, config homeassistant.AutomationConfig) error {
	if m.CreateAutomationFn != nil {
		return m.CreateAutomationFn(ctx, config)
	}
	return nil
}

func (m *UniversalMockClient) UpdateAutomation(ctx context.Context, automationID string, config homeassistant.AutomationConfig) error {
	if m.UpdateAutomationFn != nil {
		return m.UpdateAutomationFn(ctx, automationID, config)
	}
	return nil
}

func (m *UniversalMockClient) DeleteAutomation(ctx context.Context, automationID string) error {
	if m.DeleteAutomationFn != nil {
		return m.DeleteAutomationFn(ctx, automationID)
	}
	return nil
}

func (m *UniversalMockClient) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	if m.ToggleAutomationFn != nil {
		return m.ToggleAutomationFn(ctx, entityID, enabled)
	}
	return nil
}

// Helper operations implementation

func (m *UniversalMockClient) ListHelpers(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListHelpersFn != nil {
		return m.ListHelpersFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.CreateHelperFn != nil {
		return m.CreateHelperFn(ctx, helper)
	}
	return nil
}

func (m *UniversalMockClient) UpdateHelper(ctx context.Context, helperID string, helper homeassistant.HelperConfig) error {
	if m.UpdateHelperFn != nil {
		return m.UpdateHelperFn(ctx, helperID, helper)
	}
	return nil
}

func (m *UniversalMockClient) DeleteHelper(ctx context.Context, helperID string) error {
	if m.DeleteHelperFn != nil {
		return m.DeleteHelperFn(ctx, helperID)
	}
	return nil
}

func (m *UniversalMockClient) SetHelperValue(ctx context.Context, entityID string, value any) error {
	if m.SetHelperValueFn != nil {
		return m.SetHelperValueFn(ctx, entityID, value)
	}
	return nil
}

// Script operations implementation

func (m *UniversalMockClient) ListScripts(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListScriptsFn != nil {
		return m.ListScriptsFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) GetScript(ctx context.Context, scriptID string) (*homeassistant.Script, error) {
	if m.GetScriptFn != nil {
		return m.GetScriptFn(ctx, scriptID)
	}
	return &homeassistant.Script{EntityID: "script." + scriptID}, nil
}

func (m *UniversalMockClient) CreateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	if m.CreateScriptFn != nil {
		return m.CreateScriptFn(ctx, scriptID, config)
	}
	return nil
}

func (m *UniversalMockClient) UpdateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	if m.UpdateScriptFn != nil {
		return m.UpdateScriptFn(ctx, scriptID, config)
	}
	return nil
}

func (m *UniversalMockClient) DeleteScript(ctx context.Context, scriptID string) error {
	if m.DeleteScriptFn != nil {
		return m.DeleteScriptFn(ctx, scriptID)
	}
	return nil
}

// Scene operations implementation

func (m *UniversalMockClient) ListScenes(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListScenesFn != nil {
		return m.ListScenesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) GetScene(ctx context.Context, sceneID string) (*homeassistant.Scene, error) {
	if m.GetSceneFn != nil {
		return m.GetSceneFn(ctx, sceneID)
	}
	return &homeassistant.Scene{EntityID: "scene." + sceneID}, nil
}

func (m *UniversalMockClient) ConfigFileEntryExists(ctx context.Context, domain, configID string) (bool, error) {
	if m.ConfigFileEntryExistsFn != nil {
		return m.ConfigFileEntryExistsFn(ctx, domain, configID)
	}
	return true, nil
}

func (m *UniversalMockClient) CreateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	if m.CreateSceneFn != nil {
		return m.CreateSceneFn(ctx, sceneID, config)
	}
	return nil
}

func (m *UniversalMockClient) UpdateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	if m.UpdateSceneFn != nil {
		return m.UpdateSceneFn(ctx, sceneID, config)
	}
	return nil
}

func (m *UniversalMockClient) DeleteScene(ctx context.Context, sceneID string) error {
	if m.DeleteSceneFn != nil {
		return m.DeleteSceneFn(ctx, sceneID)
	}
	return nil
}

// Service operations implementation

func (m *UniversalMockClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.CallServiceFn != nil {
		return m.CallServiceFn(ctx, domain, service, data)
	}
	return []homeassistant.Entity{}, nil
}

func (m *UniversalMockClient) CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	if m.CallServiceWithResponseFn != nil {
		return m.CallServiceWithResponseFn(ctx, domain, service, data)
	}
	return map[string]any{}, nil
}

// Calendar operations implementation

func (m *UniversalMockClient) GetCalendars(ctx context.Context) ([]homeassistant.CalendarEntry, error) {
	if m.GetCalendarsFn != nil {
		return m.GetCalendarsFn(ctx)
	}
	return []homeassistant.CalendarEntry{}, nil
}

func (m *UniversalMockClient) GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]homeassistant.CalendarEvent, error) {
	if m.GetCalendarEventsFn != nil {
		return m.GetCalendarEventsFn(ctx, entityID, start, end)
	}
	return []homeassistant.CalendarEvent{}, nil
}

// Camera operations implementation

func (m *UniversalMockClient) GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error) {
	if m.GetCameraSnapshotFn != nil {
		return m.GetCameraSnapshotFn(ctx, entityID)
	}
	return []byte{}, "image/jpeg", nil
}

// Registry operations implementation

func (m *UniversalMockClient) GetEntityRegistry(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	if m.GetEntityRegistryFn != nil {
		return m.GetEntityRegistryFn(ctx)
	}
	return []homeassistant.EntityRegistryEntry{}, nil
}

func (m *UniversalMockClient) GetDeviceRegistry(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	if m.GetDeviceRegistryFn != nil {
		return m.GetDeviceRegistryFn(ctx)
	}
	return []homeassistant.DeviceRegistryEntry{}, nil
}

func (m *UniversalMockClient) GetAreaRegistry(ctx context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	if m.GetAreaRegistryFn != nil {
		return m.GetAreaRegistryFn(ctx)
	}
	return []homeassistant.AreaRegistryEntry{}, nil
}

func (m *UniversalMockClient) CreateArea(ctx context.Context, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	if m.CreateAreaFn != nil {
		return m.CreateAreaFn(ctx, config)
	}
	return &homeassistant.AreaRegistryEntry{AreaID: "area_123", Name: config.Name}, nil
}

func (m *UniversalMockClient) UpdateArea(ctx context.Context, areaID string, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	if m.UpdateAreaFn != nil {
		return m.UpdateAreaFn(ctx, areaID, config)
	}
	return &homeassistant.AreaRegistryEntry{AreaID: areaID, Name: config.Name}, nil
}

func (m *UniversalMockClient) DeleteArea(ctx context.Context, areaID string) error {
	if m.DeleteAreaFn != nil {
		return m.DeleteAreaFn(ctx, areaID)
	}
	return nil
}

func (m *UniversalMockClient) GetLabelRegistry(ctx context.Context) ([]homeassistant.LabelRegistryEntry, error) {
	if m.GetLabelRegistryFn != nil {
		return m.GetLabelRegistryFn(ctx)
	}
	return []homeassistant.LabelRegistryEntry{}, nil
}

func (m *UniversalMockClient) CreateLabel(ctx context.Context, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	if m.CreateLabelFn != nil {
		return m.CreateLabelFn(ctx, config)
	}
	return &homeassistant.LabelRegistryEntry{LabelID: "label_123", Name: config.Name}, nil
}

func (m *UniversalMockClient) UpdateLabel(ctx context.Context, labelID string, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	if m.UpdateLabelFn != nil {
		return m.UpdateLabelFn(ctx, labelID, config)
	}
	return &homeassistant.LabelRegistryEntry{LabelID: labelID, Name: config.Name}, nil
}

func (m *UniversalMockClient) DeleteLabel(ctx context.Context, labelID string) error {
	if m.DeleteLabelFn != nil {
		return m.DeleteLabelFn(ctx, labelID)
	}
	return nil
}

func (m *UniversalMockClient) GetFloorRegistry(ctx context.Context) ([]homeassistant.FloorRegistryEntry, error) {
	if m.GetFloorRegistryFn != nil {
		return m.GetFloorRegistryFn(ctx)
	}
	return []homeassistant.FloorRegistryEntry{}, nil
}

func (m *UniversalMockClient) CreateFloor(ctx context.Context, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	if m.CreateFloorFn != nil {
		return m.CreateFloorFn(ctx, config)
	}
	return &homeassistant.FloorRegistryEntry{FloorID: "floor_123", Name: config.Name}, nil
}

func (m *UniversalMockClient) UpdateFloor(ctx context.Context, floorID string, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	if m.UpdateFloorFn != nil {
		return m.UpdateFloorFn(ctx, floorID, config)
	}
	return &homeassistant.FloorRegistryEntry{FloorID: floorID, Name: config.Name}, nil
}

func (m *UniversalMockClient) DeleteFloor(ctx context.Context, floorID string) error {
	if m.DeleteFloorFn != nil {
		return m.DeleteFloorFn(ctx, floorID)
	}
	return nil
}

func (m *UniversalMockClient) GetZones(ctx context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
	if m.GetZonesFn != nil {
		return m.GetZonesFn(ctx)
	}
	return []homeassistant.ZoneRegistryEntry{}, nil
}

func (m *UniversalMockClient) CreateZone(ctx context.Context, config homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	if m.CreateZoneFn != nil {
		return m.CreateZoneFn(ctx, config)
	}
	return &homeassistant.ZoneRegistryEntry{ID: "zone_123", Name: config.Name}, nil
}

func (m *UniversalMockClient) UpdateZone(ctx context.Context, zoneID string, config homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	if m.UpdateZoneFn != nil {
		return m.UpdateZoneFn(ctx, zoneID, config)
	}
	return &homeassistant.ZoneRegistryEntry{ID: zoneID, Name: config.Name}, nil
}

func (m *UniversalMockClient) DeleteZone(ctx context.Context, zoneID string) error {
	if m.DeleteZoneFn != nil {
		return m.DeleteZoneFn(ctx, zoneID)
	}
	return nil
}

func (m *UniversalMockClient) GetPersons(ctx context.Context) ([]homeassistant.PersonRegistryEntry, error) {
	if m.GetPersonsFn != nil {
		return m.GetPersonsFn(ctx)
	}
	return []homeassistant.PersonRegistryEntry{}, nil
}

func (m *UniversalMockClient) CreatePerson(ctx context.Context, config homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	if m.CreatePersonFn != nil {
		return m.CreatePersonFn(ctx, config)
	}
	return &homeassistant.PersonRegistryEntry{ID: "person_123", Name: config.Name}, nil
}

func (m *UniversalMockClient) UpdatePerson(ctx context.Context, personID string, config homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	if m.UpdatePersonFn != nil {
		return m.UpdatePersonFn(ctx, personID, config)
	}
	return &homeassistant.PersonRegistryEntry{ID: personID, Name: config.Name}, nil
}

func (m *UniversalMockClient) DeletePerson(ctx context.Context, personID string) error {
	if m.DeletePersonFn != nil {
		return m.DeletePersonFn(ctx, personID)
	}
	return nil
}

func (m *UniversalMockClient) GetTags(ctx context.Context) ([]homeassistant.TagRegistryEntry, error) {
	if m.GetTagsFn != nil {
		return m.GetTagsFn(ctx)
	}
	return []homeassistant.TagRegistryEntry{}, nil
}

func (m *UniversalMockClient) CreateTag(ctx context.Context, config homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	if m.CreateTagFn != nil {
		return m.CreateTagFn(ctx, config)
	}
	return &homeassistant.TagRegistryEntry{TagID: "tag_123", Name: config.Name}, nil
}

func (m *UniversalMockClient) UpdateTag(ctx context.Context, tagID string, config homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	if m.UpdateTagFn != nil {
		return m.UpdateTagFn(ctx, tagID, config)
	}
	return &homeassistant.TagRegistryEntry{TagID: tagID, Name: config.Name}, nil
}

func (m *UniversalMockClient) DeleteTag(ctx context.Context, tagID string) error {
	if m.DeleteTagFn != nil {
		return m.DeleteTagFn(ctx, tagID)
	}
	return nil
}

func (m *UniversalMockClient) RemoveEntityRegistryEntry(ctx context.Context, entityID string) error {
	if m.RemoveEntityRegistryEntryFn != nil {
		return m.RemoveEntityRegistryEntryFn(ctx, entityID)
	}
	return nil
}

func (m *UniversalMockClient) UpdateEntityRegistryEntry(ctx context.Context, entityID string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
	if m.UpdateEntityRegistryEntryFn != nil {
		return m.UpdateEntityRegistryEntryFn(ctx, entityID, config)
	}
	return nil, nil
}

func (m *UniversalMockClient) RemoveDeviceConfigEntry(ctx context.Context, deviceID, configEntryID string) error {
	if m.RemoveDeviceConfigEntryFn != nil {
		return m.RemoveDeviceConfigEntryFn(ctx, deviceID, configEntryID)
	}
	return nil
}

func (m *UniversalMockClient) UpdateDeviceRegistryEntry(ctx context.Context, deviceID string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
	if m.UpdateDeviceRegistryEntryFn != nil {
		return m.UpdateDeviceRegistryEntryFn(ctx, deviceID, config)
	}
	return nil, nil
}

// Media operations implementation

func (m *UniversalMockClient) SignPath(ctx context.Context, path string, expires int) (string, error) {
	if m.SignPathFn != nil {
		return m.SignPathFn(ctx, path, expires)
	}
	return path + "?signed=true", nil
}

func (m *UniversalMockClient) GetCameraStream(ctx context.Context, entityID string) (*homeassistant.StreamInfo, error) {
	if m.GetCameraStreamFn != nil {
		return m.GetCameraStreamFn(ctx, entityID)
	}
	return &homeassistant.StreamInfo{URL: "http://example.com/stream"}, nil
}

func (m *UniversalMockClient) BrowseMedia(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error) {
	if m.BrowseMediaFn != nil {
		return m.BrowseMediaFn(ctx, mediaContentID)
	}
	return &homeassistant.MediaBrowseResult{}, nil
}

// Dashboard operations implementation

func (m *UniversalMockClient) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	if m.GetLovelaceConfigFn != nil {
		return m.GetLovelaceConfigFn(ctx, urlPath)
	}
	return map[string]any{}, nil
}

func (m *UniversalMockClient) SaveLovelaceConfig(ctx context.Context, urlPath string, config map[string]any) error {
	if m.SaveLovelaceConfigFn != nil {
		return m.SaveLovelaceConfigFn(ctx, urlPath, config)
	}
	return nil
}

func (m *UniversalMockClient) ListDashboards(ctx context.Context) ([]homeassistant.DashboardEntry, error) {
	if m.ListDashboardsFn != nil {
		return m.ListDashboardsFn(ctx)
	}
	return []homeassistant.DashboardEntry{}, nil
}

func (m *UniversalMockClient) CreateDashboard(ctx context.Context, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	if m.CreateDashboardFn != nil {
		return m.CreateDashboardFn(ctx, config)
	}
	return &homeassistant.DashboardEntry{}, nil
}

func (m *UniversalMockClient) UpdateDashboard(ctx context.Context, dashboardID string, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	if m.UpdateDashboardFn != nil {
		return m.UpdateDashboardFn(ctx, dashboardID, config)
	}
	return &homeassistant.DashboardEntry{}, nil
}

func (m *UniversalMockClient) DeleteDashboard(ctx context.Context, dashboardID string) error {
	if m.DeleteDashboardFn != nil {
		return m.DeleteDashboardFn(ctx, dashboardID)
	}
	return nil
}

// Statistics operations implementation

func (m *UniversalMockClient) GetStatistics(ctx context.Context, statIDs []string, period string) ([]homeassistant.StatisticsResult, error) {
	if m.GetStatisticsFn != nil {
		return m.GetStatisticsFn(ctx, statIDs, period)
	}
	return []homeassistant.StatisticsResult{}, nil
}

// Target operations implementation

func (m *UniversalMockClient) GetTriggersForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetTriggersForTargetFn != nil {
		return m.GetTriggersForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *UniversalMockClient) GetConditionsForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetConditionsForTargetFn != nil {
		return m.GetConditionsForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *UniversalMockClient) GetServicesForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetServicesForTargetFn != nil {
		return m.GetServicesForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *UniversalMockClient) ExtractFromTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error) {
	if m.ExtractFromTargetFn != nil {
		return m.ExtractFromTargetFn(ctx, target, expandGroup)
	}
	return &homeassistant.ExtractFromTargetResult{}, nil
}

// Config operations implementation

func (m *UniversalMockClient) GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error) {
	if m.GetHelperConfigFn != nil {
		return m.GetHelperConfigFn(ctx, platform, entityID)
	}
	return map[string]any{}, nil
}

// Config entry operations implementation

func (m *UniversalMockClient) GetConfigEntries(ctx context.Context, domain string) ([]homeassistant.ConfigEntryFull, error) {
	if m.GetConfigEntriesFn != nil {
		return m.GetConfigEntriesFn(ctx, domain)
	}
	return []homeassistant.ConfigEntryFull{}, nil
}

func (m *UniversalMockClient) GetConfigEntry(ctx context.Context, entryID string) (*homeassistant.ConfigEntryFull, error) {
	if m.GetConfigEntryFn != nil {
		return m.GetConfigEntryFn(ctx, entryID)
	}
	return &homeassistant.ConfigEntryFull{EntryID: entryID}, nil
}

func (m *UniversalMockClient) GetConfigEntryOptions(ctx context.Context, entryID string) (map[string]any, error) {
	if m.GetConfigEntryOptionsFn != nil {
		return m.GetConfigEntryOptionsFn(ctx, entryID)
	}
	return map[string]any{}, nil
}

func (m *UniversalMockClient) DeleteConfigEntry(ctx context.Context, entryID string) (bool, error) {
	if m.DeleteConfigEntryFn != nil {
		return m.DeleteConfigEntryFn(ctx, entryID)
	}
	return false, nil
}

// Service discovery operations implementation

func (m *UniversalMockClient) GetServices(ctx context.Context) ([]homeassistant.Service, error) {
	if m.GetServicesFn != nil {
		return m.GetServicesFn(ctx)
	}
	return []homeassistant.Service{}, nil
}

// System configuration operations implementation

func (m *UniversalMockClient) GetConfig(ctx context.Context) (*homeassistant.Config, error) {
	if m.GetConfigFn != nil {
		return m.GetConfigFn(ctx)
	}
	return &homeassistant.Config{
		Version:      "2024.1.0",
		State:        "RUNNING",
		LocationName: "Home",
		TimeZone:     "UTC",
	}, nil
}

// Template operations implementation

func (m *UniversalMockClient) RenderTemplate(ctx context.Context, template string) (string, error) {
	if m.RenderTemplateFn != nil {
		return m.RenderTemplateFn(ctx, template)
	}
	return "rendered: " + template, nil
}

// Logbook operations implementation

func (m *UniversalMockClient) GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]homeassistant.LogbookEntry, error) {
	if m.GetLogbookFn != nil {
		return m.GetLogbookFn(ctx, startTime, endTime, entityID)
	}
	return []homeassistant.LogbookEntry{}, nil
}

// System log operations implementation

func (m *UniversalMockClient) GetSystemLog(ctx context.Context) ([]homeassistant.SystemLogEntry, error) {
	if m.GetSystemLogFn != nil {
		return m.GetSystemLogFn(ctx)
	}
	return []homeassistant.SystemLogEntry{}, nil
}

func (m *UniversalMockClient) ClearSystemLog(ctx context.Context) error {
	if m.ClearSystemLogFn != nil {
		return m.ClearSystemLogFn(ctx)
	}
	return nil
}

// Configuration validation operations implementation

func (m *UniversalMockClient) CheckConfig(ctx context.Context) (*homeassistant.ConfigCheckResult, error) {
	if m.CheckConfigFn != nil {
		return m.CheckConfigFn(ctx)
	}
	return &homeassistant.ConfigCheckResult{
		Result: "valid",
		Errors: nil,
	}, nil
}

// HACS operations implementation

func (m *UniversalMockClient) SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error) {
	if m.SendHACSCommandFn != nil {
		return m.SendHACSCommandFn(ctx, command, data)
	}
	return map[string]any{}, nil
}

// =============================================================================
// Test Helper Functions
// =============================================================================

// handlerTestCase represents a standard test case for handler functions.
type handlerTestCase struct {
	name            string
	args            map[string]any
	setupMock       func(*UniversalMockClient)
	wantError       bool
	wantContains    []string
	wantNotContains []string
}

// runHandlerTestCases executes a set of test cases for a handler function.
func runHandlerTestCases(
	t *testing.T,
	tests []handlerTestCase,
	handlerFunc func(context.Context, homeassistant.Client, map[string]any) (*mcp.ToolsCallResult, error),
) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			// Use a fast wait config so tests don't block on real timeouts.
			// The poller resolves within one poll interval when the mock responds instantly.
			ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
				Timeout:      50 * time.Millisecond,
				PollInterval: 5 * time.Millisecond,
			})

			result, err := handlerFunc(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("handler returned unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("handler returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handler returned empty content")
			}

			content := result.Content[0].Text
			assertContainsAll(t, content, tt.wantContains)
			assertNotContainsAny(t, content, tt.wantNotContains)
		})
	}
}

// paramRequiredTestCases generates standard test cases for required parameters.
func paramRequiredTestCases(paramName string) []handlerTestCase {
	return []handlerTestCase{
		{
			name:         "missing " + paramName,
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{paramName + " is required"},
		},
		{
			name:         "empty " + paramName,
			args:         map[string]any{paramName: ""},
			wantError:    true,
			wantContains: []string{paramName + " is required"},
		},
	}
}

// =============================================================================
// Tool Schema Validation Helpers
// =============================================================================

// toolSchemaExpectation defines expectations for a tool's schema.
type toolSchemaExpectation struct {
	ExpectedName    string
	RequiredParams  []string
	OptionalParams  []string
	WantDescription bool
}

// verifyToolSchema validates a tool's schema against expectations.
func verifyToolSchema(t *testing.T, tool mcp.Tool, expect toolSchemaExpectation) {
	t.Helper()

	// Name check
	if tool.Name != expect.ExpectedName {
		t.Errorf("tool.Name = %q, want %q", tool.Name, expect.ExpectedName)
	}

	// Description check
	if expect.WantDescription && tool.Description == "" {
		t.Error("tool.Description is empty, want non-empty")
	}

	// Schema type check
	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Required parameters check
	requiredMap := make(map[string]bool)
	for _, req := range tool.InputSchema.Required {
		requiredMap[req] = true
	}

	for _, param := range expect.RequiredParams {
		if !requiredMap[param] {
			t.Errorf("Required parameter %q not found in schema.Required", param)
		}
	}

	// Properties check (required + optional)
	allParams := make([]string, 0, len(expect.RequiredParams)+len(expect.OptionalParams))
	allParams = append(allParams, expect.RequiredParams...)
	allParams = append(allParams, expect.OptionalParams...)
	for _, param := range allParams {
		if _, ok := tool.InputSchema.Properties[param]; !ok {
			t.Errorf("Property %q missing from schema.Properties", param)
		}
	}
}

// =============================================================================
// Content Assertion Helpers
// =============================================================================

// assertContainsAll checks that content contains all expected strings.
func assertContainsAll(t *testing.T, content string, want []string) {
	t.Helper()
	for _, expected := range want {
		if !strings.Contains(content, expected) {
			t.Errorf("Content missing expected string %q\nGot: %s", expected, truncateForError(content))
		}
	}
}

// assertNotContainsAny checks that content does not contain any of the unwanted strings.
func assertNotContainsAny(t *testing.T, content string, notWant []string) {
	t.Helper()
	for _, unexpected := range notWant {
		if strings.Contains(content, unexpected) {
			t.Errorf("Content should not contain %q\nGot: %s", unexpected, truncateForError(content))
		}
	}
}

// truncateForError truncates long content for readable error messages.
func truncateForError(content string) string {
	const maxLen = 500
	if len(content) > maxLen {
		return content[:maxLen] + "... (truncated)"
	}
	return content
}

// =============================================================================
// Common Test Data
// =============================================================================

// testEntity creates a standard test entity.
func testEntity(entityID, state string) homeassistant.Entity {
	return homeassistant.Entity{
		EntityID: entityID,
		State:    state,
		Attributes: map[string]any{
			"friendly_name": "Test " + entityID,
		},
	}
}

// testAutomation creates a standard test automation.
func testAutomation(id, state, friendlyName string) homeassistant.Automation {
	return homeassistant.Automation{
		EntityID:      "automation." + id,
		State:         state,
		FriendlyName:  friendlyName,
		LastTriggered: "2024-01-15T10:30:00Z",
	}
}

// storageManagedRegistry returns a GetEntityRegistryFn reporting entityID as storage/UI-managed
// (non-empty unique_id), so the isYAMLDefinedEntity write guard (#122) lets update/patch proceed.
// Tests that exercise the update/patch write path must supply this — the mock's default empty
// registry is otherwise indistinguishable from a YAML-defined entity.
func storageManagedRegistry(entityID string) func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	return func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
		return []homeassistant.EntityRegistryEntry{
			{EntityID: entityID, UniqueID: "test_unique_id"},
		}, nil
	}
}

// =============================================================================
// Tests for Testing Helpers (Self-Tests)
// =============================================================================

func TestUniversalMockClient_DefaultBehavior(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	ctx := context.Background()

	// Test default entity operations
	t.Run("GetStates returns empty slice", func(t *testing.T) {
		t.Parallel()
		states, err := client.GetStates(ctx)
		if err != nil {
			t.Errorf("GetStates() error = %v", err)
		}
		if len(states) != 0 {
			t.Errorf("GetStates() len = %d, want 0", len(states))
		}
	})

	t.Run("GetState returns default entity", func(t *testing.T) {
		t.Parallel()
		entity, err := client.GetState(ctx, "test.entity")
		if err != nil {
			t.Errorf("GetState() error = %v", err)
		}
		if entity.EntityID != "test.entity" {
			t.Errorf("GetState() EntityID = %q, want 'test.entity'", entity.EntityID)
		}
	})

	t.Run("ListAutomations returns empty slice", func(t *testing.T) {
		t.Parallel()
		autos, err := client.ListAutomations(ctx)
		if err != nil {
			t.Errorf("ListAutomations() error = %v", err)
		}
		if len(autos) != 0 {
			t.Errorf("ListAutomations() len = %d, want 0", len(autos))
		}
	})
}

func TestUniversalMockClient_CustomHooks(t *testing.T) {
	t.Parallel()

	t.Run("GetStateFn hook is called", func(t *testing.T) {
		t.Parallel()

		called := false
		client := &UniversalMockClient{
			GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
				called = true
				return &homeassistant.Entity{EntityID: entityID, State: "custom"}, nil
			},
		}

		entity, _ := client.GetState(context.Background(), "test.entity")
		if !called {
			t.Error("GetStateFn was not called")
		}
		if entity.State != "custom" {
			t.Errorf("GetState() State = %q, want 'custom'", entity.State)
		}
	})

	t.Run("ListAutomationsFn hook is called", func(t *testing.T) {
		t.Parallel()

		client := &UniversalMockClient{
			ListAutomationsFn: func(_ context.Context) ([]homeassistant.Automation, error) {
				return []homeassistant.Automation{
					{EntityID: "automation.test"},
				}, nil
			},
		}

		autos, _ := client.ListAutomations(context.Background())
		if len(autos) != 1 {
			t.Errorf("ListAutomations() len = %d, want 1", len(autos))
		}
	})
}

func TestTestEntity(t *testing.T) {
	t.Parallel()

	entity := testEntity("light.living_room", "on")

	if entity.EntityID != "light.living_room" {
		t.Errorf("testEntity() EntityID = %q, want 'light.living_room'", entity.EntityID)
	}
	if entity.State != "on" {
		t.Errorf("testEntity() State = %q, want 'on'", entity.State)
	}
	if entity.Attributes["friendly_name"] != "Test light.living_room" {
		t.Errorf("testEntity() friendly_name = %q, want 'Test light.living_room'", entity.Attributes["friendly_name"])
	}
}

func TestTestAutomation(t *testing.T) {
	t.Parallel()

	auto := testAutomation("morning_routine", "on", "Morning Routine")

	if auto.EntityID != "automation.morning_routine" {
		t.Errorf("testAutomation() EntityID = %q, want 'automation.morning_routine'", auto.EntityID)
	}
	if auto.State != "on" {
		t.Errorf("testAutomation() State = %q, want 'on'", auto.State)
	}
	if auto.FriendlyName != "Morning Routine" {
		t.Errorf("testAutomation() FriendlyName = %q, want 'Morning Routine'", auto.FriendlyName)
	}
}

func TestAssertContainsAll(t *testing.T) {
	t.Parallel()

	// Create a mock T to capture errors
	tests := []struct {
		name       string
		content    string
		want       []string
		shouldFail bool
	}{
		{
			name:       "all strings present",
			content:    "hello world test",
			want:       []string{"hello", "world"},
			shouldFail: false,
		},
		{
			name:       "empty want list",
			content:    "any content",
			want:       []string{},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Just verify no panic occurs
			assertContainsAll(t, tt.content, tt.want)
		})
	}
}

func TestAssertNotContainsAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		notWant []string
	}{
		{
			name:    "no unwanted strings",
			content: "hello world",
			notWant: []string{"foo", "bar"},
		},
		{
			name:    "empty not want list",
			content: "any content",
			notWant: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Just verify no panic occurs
			assertNotContainsAny(t, tt.content, tt.notWant)
		})
	}
}

func TestTruncateForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantTail string
	}{
		{
			name:     "short content unchanged",
			content:  "short",
			wantLen:  5,
			wantTail: "short",
		},
		{
			name:     "long content truncated",
			content:  strings.Repeat("a", 600),
			wantLen:  515, // 500 + len("... (truncated)")
			wantTail: "... (truncated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := truncateForError(tt.content)
			if len(result) != tt.wantLen {
				t.Errorf("truncateForError() len = %d, want %d", len(result), tt.wantLen)
			}
			if !strings.HasSuffix(result, tt.wantTail) {
				t.Errorf("truncateForError() should end with %q", tt.wantTail)
			}
		})
	}
}

func TestParamRequiredTestCases(t *testing.T) {
	t.Parallel()

	cases := paramRequiredTestCases("entity_id")

	if len(cases) != 2 {
		t.Fatalf("paramRequiredTestCases() len = %d, want 2", len(cases))
	}

	if cases[0].name != "missing entity_id" {
		t.Errorf("cases[0].name = %q, want 'missing entity_id'", cases[0].name)
	}
	if cases[1].name != "empty entity_id" {
		t.Errorf("cases[1].name = %q, want 'empty entity_id'", cases[1].name)
	}
	if !cases[0].wantError {
		t.Error("cases[0].wantError should be true")
	}
	if len(cases[0].wantContains) == 0 || cases[0].wantContains[0] != "entity_id is required" {
		t.Error("cases[0].wantContains should include 'entity_id is required'")
	}
}

func TestVerifyToolSchema(t *testing.T) {
	t.Parallel()

	tool := mcp.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: mcp.JSONSchema{
			Type: testSchemaTypeObject,
			Properties: map[string]mcp.JSONSchema{
				"required_param": {Type: "string"},
				"optional_param": {Type: "boolean"},
			},
			Required: []string{"required_param"},
		},
	}

	// This should pass without errors
	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "test_tool",
		RequiredParams:  []string{"required_param"},
		OptionalParams:  []string{"optional_param"},
		WantDescription: true,
	})
}

func TestRunHandlerTestCases(t *testing.T) {
	t.Parallel()

	// Create a simple handler for testing
	handler := func(_ context.Context, _ homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
		entityID := getString(args, "entity_id")
		if entityID == "" {
			return &mcp.ToolsCallResult{
				IsError: true,
				Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			}, nil
		}
		return &mcp.ToolsCallResult{
			IsError: false,
			Content: []mcp.ContentBlock{mcp.NewTextContent("Success: " + entityID)},
		}, nil
	}

	tests := []handlerTestCase{
		{
			name:         "success case",
			args:         map[string]any{"entity_id": "light.test"},
			wantError:    false,
			wantContains: []string{"Success", "light.test"},
		},
		{
			name:         "error case - missing param",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
	}

	runHandlerTestCases(t, tests, handler)
}

func TestHandlerTestCase_WithMock(t *testing.T) {
	t.Parallel()

	handler := func(_ context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
		states, err := client.GetStates(context.Background())
		if err != nil {
			return &mcp.ToolsCallResult{
				IsError: true,
				Content: []mcp.ContentBlock{mcp.NewTextContent("Error: " + err.Error())},
			}, err
		}
		return &mcp.ToolsCallResult{
			IsError: false,
			Content: []mcp.ContentBlock{mcp.NewTextContent("Found " + string(rune('0'+len(states))) + " states")},
		}, nil
	}

	tests := []handlerTestCase{
		{
			name: "with mock setup",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "light.one"},
						{EntityID: "light.two"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 states"},
		},
	}

	runHandlerTestCases(t, tests, handler)
}

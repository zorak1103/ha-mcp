package enforce

import (
	"context"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Fail-closed stubs: one per homeassistant.Client method, in the order of
// client.go. Each records a denial and returns zero values until its real
// enforcement replaces it (and the stub is deleted from this file).

// Entity operations.

func (c *client) GetStates(context.Context) ([]homeassistant.Entity, error) {
	return nil, c.unclassified("GetStates")
}

func (c *client) GetState(context.Context, string) (*homeassistant.Entity, error) {
	return nil, c.unclassified("GetState")
}

func (c *client) SetState(context.Context, string, homeassistant.StateUpdate) (*homeassistant.Entity, error) {
	return nil, c.unclassified("SetState")
}

// History operations.

func (c *client) GetHistory(context.Context, string, time.Time, time.Time) ([][]homeassistant.HistoryEntry, error) {
	return nil, c.unclassified("GetHistory")
}

// Automation operations.

func (c *client) ListAutomations(context.Context) ([]homeassistant.Automation, error) {
	return nil, c.unclassified("ListAutomations")
}

func (c *client) GetAutomation(context.Context, string) (*homeassistant.Automation, error) {
	return nil, c.unclassified("GetAutomation")
}

func (c *client) CreateAutomation(context.Context, homeassistant.AutomationConfig) error {
	return c.unclassified("CreateAutomation")
}

func (c *client) UpdateAutomation(context.Context, string, homeassistant.AutomationConfig) error {
	return c.unclassified("UpdateAutomation")
}

func (c *client) DeleteAutomation(context.Context, string) error {
	return c.unclassified("DeleteAutomation")
}

func (c *client) ToggleAutomation(context.Context, string, bool) error {
	return c.unclassified("ToggleAutomation")
}

// Helper operations.

func (c *client) ListHelpers(context.Context) ([]homeassistant.Entity, error) {
	return nil, c.unclassified("ListHelpers")
}

func (c *client) CreateHelper(context.Context, homeassistant.HelperConfig) error {
	return c.unclassified("CreateHelper")
}

func (c *client) CreateHelperEntity(context.Context, homeassistant.HelperConfig) (string, error) {
	return "", c.unclassified("CreateHelperEntity")
}

func (c *client) UpdateHelper(context.Context, string, homeassistant.HelperConfig) error {
	return c.unclassified("UpdateHelper")
}

func (c *client) DeleteHelper(context.Context, string) error {
	return c.unclassified("DeleteHelper")
}

func (c *client) SetHelperValue(context.Context, string, any) error {
	return c.unclassified("SetHelperValue")
}

// Script operations.

func (c *client) ListScripts(context.Context) ([]homeassistant.Entity, error) {
	return nil, c.unclassified("ListScripts")
}

func (c *client) GetScript(context.Context, string) (*homeassistant.Script, error) {
	return nil, c.unclassified("GetScript")
}

func (c *client) CreateScript(context.Context, string, homeassistant.ScriptConfig) error {
	return c.unclassified("CreateScript")
}

func (c *client) UpdateScript(context.Context, string, homeassistant.ScriptConfig) error {
	return c.unclassified("UpdateScript")
}

func (c *client) DeleteScript(context.Context, string) error {
	return c.unclassified("DeleteScript")
}

// Scene operations.

func (c *client) ListScenes(context.Context) ([]homeassistant.Entity, error) {
	return nil, c.unclassified("ListScenes")
}

func (c *client) GetScene(context.Context, string) (*homeassistant.Scene, error) {
	return nil, c.unclassified("GetScene")
}

func (c *client) CreateScene(context.Context, string, homeassistant.SceneConfig) error {
	return c.unclassified("CreateScene")
}

func (c *client) UpdateScene(context.Context, string, homeassistant.SceneConfig) error {
	return c.unclassified("UpdateScene")
}

func (c *client) DeleteScene(context.Context, string) error {
	return c.unclassified("DeleteScene")
}

func (c *client) ConfigFileEntryExists(context.Context, string, string) (bool, error) {
	return false, c.unclassified("ConfigFileEntryExists")
}

// Service operations.

func (c *client) CallService(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
	return nil, c.unclassified("CallService")
}

// Registry operations.

func (c *client) GetEntityRegistry(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	return nil, c.unclassified("GetEntityRegistry")
}

func (c *client) GetEntityRegistryEntry(context.Context, string) (*homeassistant.EntityRegistryEntry, error) {
	return nil, c.unclassified("GetEntityRegistryEntry")
}

func (c *client) GetDeviceRegistry(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	return nil, c.unclassified("GetDeviceRegistry")
}

func (c *client) GetAreaRegistry(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	return nil, c.unclassified("GetAreaRegistry")
}

// Area registry modification operations.

func (c *client) CreateArea(context.Context, homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	return nil, c.unclassified("CreateArea")
}

func (c *client) UpdateArea(context.Context, string, homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	return nil, c.unclassified("UpdateArea")
}

func (c *client) DeleteArea(context.Context, string) error {
	return c.unclassified("DeleteArea")
}

// Label registry operations.

func (c *client) GetLabelRegistry(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
	return nil, c.unclassified("GetLabelRegistry")
}

func (c *client) CreateLabel(context.Context, homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	return nil, c.unclassified("CreateLabel")
}

func (c *client) UpdateLabel(context.Context, string, homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	return nil, c.unclassified("UpdateLabel")
}

func (c *client) DeleteLabel(context.Context, string) error {
	return c.unclassified("DeleteLabel")
}

// Floor registry operations.

func (c *client) GetFloorRegistry(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
	return nil, c.unclassified("GetFloorRegistry")
}

func (c *client) CreateFloor(context.Context, homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	return nil, c.unclassified("CreateFloor")
}

func (c *client) UpdateFloor(context.Context, string, homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	return nil, c.unclassified("UpdateFloor")
}

func (c *client) DeleteFloor(context.Context, string) error {
	return c.unclassified("DeleteFloor")
}

// Zone operations.

func (c *client) GetZones(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
	return nil, c.unclassified("GetZones")
}

func (c *client) CreateZone(context.Context, homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	return nil, c.unclassified("CreateZone")
}

func (c *client) UpdateZone(context.Context, string, homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	return nil, c.unclassified("UpdateZone")
}

func (c *client) DeleteZone(context.Context, string) error {
	return c.unclassified("DeleteZone")
}

// Person registry operations.

func (c *client) GetPersons(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
	return nil, c.unclassified("GetPersons")
}

func (c *client) CreatePerson(context.Context, homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	return nil, c.unclassified("CreatePerson")
}

func (c *client) UpdatePerson(context.Context, string, homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	return nil, c.unclassified("UpdatePerson")
}

func (c *client) DeletePerson(context.Context, string) error {
	return c.unclassified("DeletePerson")
}

// Tag registry operations.

func (c *client) GetTags(context.Context) ([]homeassistant.TagRegistryEntry, error) {
	return nil, c.unclassified("GetTags")
}

func (c *client) CreateTag(context.Context, homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	return nil, c.unclassified("CreateTag")
}

func (c *client) UpdateTag(context.Context, string, homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	return nil, c.unclassified("UpdateTag")
}

func (c *client) DeleteTag(context.Context, string) error {
	return c.unclassified("DeleteTag")
}

// Entity registry modification operations.

func (c *client) RemoveEntityRegistryEntry(context.Context, string) error {
	return c.unclassified("RemoveEntityRegistryEntry")
}

func (c *client) UpdateEntityRegistryEntry(context.Context, string, homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
	return nil, c.unclassified("UpdateEntityRegistryEntry")
}

// Device registry modification operations.

func (c *client) RemoveDeviceConfigEntry(context.Context, string, string) error {
	return c.unclassified("RemoveDeviceConfigEntry")
}

func (c *client) UpdateDeviceRegistryEntry(context.Context, string, homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
	return nil, c.unclassified("UpdateDeviceRegistryEntry")
}

// Media operations.

func (c *client) SignPath(context.Context, string, int) (string, error) {
	return "", c.unclassified("SignPath")
}

func (c *client) GetCameraStream(context.Context, string) (*homeassistant.StreamInfo, error) {
	return nil, c.unclassified("GetCameraStream")
}

func (c *client) BrowseMedia(context.Context, string) (*homeassistant.MediaBrowseResult, error) {
	return nil, c.unclassified("BrowseMedia")
}

// Dashboard operations.

func (c *client) GetLovelaceConfig(context.Context, string) (map[string]any, error) {
	return nil, c.unclassified("GetLovelaceConfig")
}

func (c *client) SaveLovelaceConfig(context.Context, string, map[string]any) error {
	return c.unclassified("SaveLovelaceConfig")
}

func (c *client) ListDashboards(context.Context) ([]homeassistant.DashboardEntry, error) {
	return nil, c.unclassified("ListDashboards")
}

func (c *client) CreateDashboard(context.Context, homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	return nil, c.unclassified("CreateDashboard")
}

func (c *client) UpdateDashboard(context.Context, string, homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	return nil, c.unclassified("UpdateDashboard")
}

func (c *client) DeleteDashboard(context.Context, string) error {
	return c.unclassified("DeleteDashboard")
}

// Statistics operations.

func (c *client) GetStatistics(context.Context, []string, string) ([]homeassistant.StatisticsResult, error) {
	return nil, c.unclassified("GetStatistics")
}

// Target operations.

func (c *client) GetTriggersForTarget(context.Context, homeassistant.Target, *bool) ([]string, error) {
	return nil, c.unclassified("GetTriggersForTarget")
}

func (c *client) GetConditionsForTarget(context.Context, homeassistant.Target, *bool) ([]string, error) {
	return nil, c.unclassified("GetConditionsForTarget")
}

func (c *client) GetServicesForTarget(context.Context, homeassistant.Target, *bool) ([]string, error) {
	return nil, c.unclassified("GetServicesForTarget")
}

func (c *client) ExtractFromTarget(context.Context, homeassistant.Target, *bool) (*homeassistant.ExtractFromTargetResult, error) {
	return nil, c.unclassified("ExtractFromTarget")
}

// Config operations.

func (c *client) GetHelperConfig(context.Context, string, string) (map[string]any, error) {
	return nil, c.unclassified("GetHelperConfig")
}

// Config entry operations.

func (c *client) GetConfigEntries(context.Context, string) ([]homeassistant.ConfigEntryFull, error) {
	return nil, c.unclassified("GetConfigEntries")
}

func (c *client) GetConfigEntry(context.Context, string) (*homeassistant.ConfigEntryFull, error) {
	return nil, c.unclassified("GetConfigEntry")
}

func (c *client) GetConfigEntryOptions(context.Context, string) (map[string]any, error) {
	return nil, c.unclassified("GetConfigEntryOptions")
}

func (c *client) DeleteConfigEntry(context.Context, string) (bool, error) {
	return false, c.unclassified("DeleteConfigEntry")
}

// Service discovery operations.

func (c *client) GetServices(context.Context) ([]homeassistant.Service, error) {
	return nil, c.unclassified("GetServices")
}

// System configuration operations.

func (c *client) GetConfig(context.Context) (*homeassistant.Config, error) {
	return nil, c.unclassified("GetConfig")
}

// Template operations.

func (c *client) RenderTemplate(context.Context, string) (string, error) {
	return "", c.unclassified("RenderTemplate")
}

// Logbook operations.

func (c *client) GetLogbook(context.Context, string, string, []string) ([]homeassistant.LogbookEntry, error) {
	return nil, c.unclassified("GetLogbook")
}

// Configuration validation operations.

func (c *client) CheckConfig(context.Context) (*homeassistant.ConfigCheckResult, error) {
	return nil, c.unclassified("CheckConfig")
}

// HACS operations.

func (c *client) SendHACSCommand(context.Context, string, map[string]any) (any, error) {
	return nil, c.unclassified("SendHACSCommand")
}

// Service call with response.

func (c *client) CallServiceWithResponse(context.Context, string, string, map[string]any) (map[string]any, error) {
	return nil, c.unclassified("CallServiceWithResponse")
}

// Recorder statistics operations.

func (c *client) ListStatisticIDs(context.Context, string) ([]homeassistant.StatisticMeta, error) {
	return nil, c.unclassified("ListStatisticIDs")
}

func (c *client) ValidateStatistics(context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
	return nil, c.unclassified("ValidateStatistics")
}

func (c *client) ClearStatistics(context.Context, []string) error {
	return c.unclassified("ClearStatistics")
}

// Calendar operations.

func (c *client) GetCalendars(context.Context) ([]homeassistant.CalendarEntry, error) {
	return nil, c.unclassified("GetCalendars")
}

func (c *client) GetCalendarEvents(context.Context, string, string, string) ([]homeassistant.CalendarEvent, error) {
	return nil, c.unclassified("GetCalendarEvents")
}

// Camera operations.

func (c *client) GetCameraSnapshot(context.Context, string) ([]byte, string, error) {
	return nil, "", c.unclassified("GetCameraSnapshot")
}

// System log operations.

func (c *client) GetSystemLog(context.Context) ([]homeassistant.SystemLogEntry, error) {
	return nil, c.unclassified("GetSystemLog")
}

func (c *client) ClearSystemLog(context.Context) error {
	return c.unclassified("ClearSystemLog")
}

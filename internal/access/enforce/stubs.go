package enforce

import (
	"context"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Fail-closed stubs: one per homeassistant.Client method, in the order of
// client.go. Each records a denial and returns zero values until its real
// enforcement replaces it (and the stub is deleted from this file).

// Automation operations.

func (c *client) CreateAutomation(context.Context, homeassistant.AutomationConfig) error {
	return c.unclassified("CreateAutomation")
}

func (c *client) UpdateAutomation(context.Context, string, homeassistant.AutomationConfig) error {
	return c.unclassified("UpdateAutomation")
}

func (c *client) DeleteAutomation(context.Context, string) error {
	return c.unclassified("DeleteAutomation")
}

// Helper operations.

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

// Script operations.

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

func (c *client) CreateScene(context.Context, string, homeassistant.SceneConfig) error {
	return c.unclassified("CreateScene")
}

func (c *client) UpdateScene(context.Context, string, homeassistant.SceneConfig) error {
	return c.unclassified("UpdateScene")
}

func (c *client) DeleteScene(context.Context, string) error {
	return c.unclassified("DeleteScene")
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

// Dashboard operations.

func (c *client) SaveLovelaceConfig(context.Context, string, map[string]any) error {
	return c.unclassified("SaveLovelaceConfig")
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

// Service discovery operations.

func (c *client) GetServices(context.Context) ([]homeassistant.Service, error) {
	return nil, c.unclassified("GetServices")
}

// System configuration operations.

func (c *client) GetConfig(context.Context) (*homeassistant.Config, error) {
	return nil, c.unclassified("GetConfig")
}

// Configuration validation operations.

func (c *client) CheckConfig(context.Context) (*homeassistant.ConfigCheckResult, error) {
	return nil, c.unclassified("CheckConfig")
}

// Recorder statistics operations.

func (c *client) ValidateStatistics(context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
	return nil, c.unclassified("ValidateStatistics")
}

// System log operations.

func (c *client) GetSystemLog(context.Context) ([]homeassistant.SystemLogEntry, error) {
	return nil, c.unclassified("GetSystemLog")
}

// Package homeassistant provides a hybrid client combining WebSocket and REST APIs.
// coverage-exempt: multi-step Config Entry and Options Flow routing requires real HA API responses
package homeassistant

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Constants for entity domains and platforms used in Config Entry Flow logic.
const (
	domainSensor       = "sensor"
	domainBinarySensor = "binary_sensor"
	domainSwitch       = "switch"
	domainLight        = "light"
	domainCover        = "cover"
	domainFan          = "fan"
	domainLock         = "lock"
	domainInputNumber  = "input_number"
	domainNumber       = "number"
	domainClimate      = "climate"
	domainHumidifier   = "humidifier"
	domainSelect       = "select"
	domainSiren        = "siren"
	domainValve        = "valve"

	platformTemplate   = "template"
	platformGroup      = "group"
	platformRandom     = "random"
	platformSwitchAsX  = "switch_as_x"
	platformStatistics = "statistics"
	platformTrend      = "trend"
	platformFilter     = "filter"

	flowTypeMenu        = "menu"
	flowTypeForm        = "form"
	flowTypeCreateEntry = "create_entry"
)

// binaryDeviceClasses maps device classes that indicate a binary sensor.
var binaryDeviceClasses = map[string]bool{
	"battery": true, "cold": true, "door": true, "garage_door": true,
	"heat": true, "moisture": true, "motion": true, "moving": true,
	"occupancy": true, "opening": true, "plug": true, "presence": true,
	"problem": true, "safety": true, "smoke": true, "sound": true,
	"tamper": true, "vibration": true, "window": true,
}

// entityDomainToGroupType maps entity domains to their corresponding group types.
var entityDomainToGroupType = map[string]string{
	domainSensor:       domainSensor,
	domainInputNumber:  domainSensor,
	domainNumber:       domainSensor,
	domainBinarySensor: domainBinarySensor,
	domainSwitch:       domainSwitch,
	domainLight:        domainLight,
	domainCover:        domainCover,
	domainFan:          domainFan,
	domainLock:         domainLock,
}

// sensorGroupDomains are entity domains that result in sensor groups.
var sensorGroupDomains = map[string]bool{
	domainSensor:      true,
	domainInputNumber: true,
	domainNumber:      true,
}

// WSOperations is an interface for WebSocket client operations.
// It matches the subset of Client methods that are delegated to WebSocket.
// This allows mocking the WebSocket client for testing.
type WSOperations interface {
	GetStates(ctx context.Context) ([]Entity, error)
	GetState(ctx context.Context, entityID string) (*Entity, error)
	SetState(ctx context.Context, entityID string, state StateUpdate) (*Entity, error)
	GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error)
	CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error)
	CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error)
	ListAutomations(ctx context.Context) ([]Automation, error)
	GetAutomation(ctx context.Context, automationID string) (*Automation, error)
	ToggleAutomation(ctx context.Context, entityID string, enabled bool) error
	ListHelpers(ctx context.Context) ([]Entity, error)
	CreateHelper(ctx context.Context, config HelperConfig) error
	UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error
	DeleteHelper(ctx context.Context, helperID string) error
	SetHelperValue(ctx context.Context, entityID string, value any) error
	ListScripts(ctx context.Context) ([]Entity, error)
	GetScript(ctx context.Context, scriptID string) (*Script, error)
	ListScenes(ctx context.Context) ([]Entity, error)
	GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error)
	GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error)
	GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error)
	CreateArea(ctx context.Context, config AreaConfig) (*AreaRegistryEntry, error)
	UpdateArea(ctx context.Context, areaID string, config AreaConfig) (*AreaRegistryEntry, error)
	DeleteArea(ctx context.Context, areaID string) error
	GetLabelRegistry(ctx context.Context) ([]LabelRegistryEntry, error)
	CreateLabel(ctx context.Context, config LabelConfig) (*LabelRegistryEntry, error)
	UpdateLabel(ctx context.Context, labelID string, config LabelConfig) (*LabelRegistryEntry, error)
	DeleteLabel(ctx context.Context, labelID string) error
	GetFloorRegistry(ctx context.Context) ([]FloorRegistryEntry, error)
	CreateFloor(ctx context.Context, config FloorConfig) (*FloorRegistryEntry, error)
	UpdateFloor(ctx context.Context, floorID string, config FloorConfig) (*FloorRegistryEntry, error)
	DeleteFloor(ctx context.Context, floorID string) error
	GetZones(ctx context.Context) ([]ZoneRegistryEntry, error)
	CreateZone(ctx context.Context, config ZoneConfig) (*ZoneRegistryEntry, error)
	UpdateZone(ctx context.Context, zoneID string, config ZoneConfig) (*ZoneRegistryEntry, error)
	DeleteZone(ctx context.Context, zoneID string) error
	GetPersons(ctx context.Context) ([]PersonRegistryEntry, error)
	CreatePerson(ctx context.Context, config PersonConfig) (*PersonRegistryEntry, error)
	UpdatePerson(ctx context.Context, personID string, config PersonConfig) (*PersonRegistryEntry, error)
	DeletePerson(ctx context.Context, personID string) error
	GetTags(ctx context.Context) ([]TagRegistryEntry, error)
	CreateTag(ctx context.Context, config TagConfig) (*TagRegistryEntry, error)
	UpdateTag(ctx context.Context, tagID string, config TagConfig) (*TagRegistryEntry, error)
	DeleteTag(ctx context.Context, tagID string) error
	RemoveEntityRegistryEntry(ctx context.Context, entityID string) error
	UpdateEntityRegistryEntry(ctx context.Context, entityID string, config EntityRegistryUpdateConfig) (*EntityRegistryEntry, error)
	RemoveDeviceConfigEntry(ctx context.Context, deviceID, configEntryID string) error
	UpdateDeviceRegistryEntry(ctx context.Context, deviceID string, config DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error)
	SignPath(ctx context.Context, path string, expires int) (string, error)
	GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error)
	BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error)
	GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error)
	SaveLovelaceConfig(ctx context.Context, urlPath string, config map[string]any) error
	ListDashboards(ctx context.Context) ([]DashboardEntry, error)
	CreateDashboard(ctx context.Context, config DashboardConfig) (*DashboardEntry, error)
	UpdateDashboard(ctx context.Context, dashboardID string, config DashboardConfig) (*DashboardEntry, error)
	DeleteDashboard(ctx context.Context, dashboardID string) error
	GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error)
	GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error)
	GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error)
	GetConfigEntries(ctx context.Context, domain string) ([]ConfigEntryFull, error)
	GetConfigEntry(ctx context.Context, entryID string) (*ConfigEntryFull, error)
	SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error)

	// System log operations (WebSocket-native)
	GetSystemLog(ctx context.Context) ([]SystemLogEntry, error)
	ClearSystemLog(ctx context.Context) error
}

// RESTOperations is an interface for REST client operations.
// This allows mocking the REST client for testing.
type RESTOperations interface {
	// Automation operations (REST-only for create/update/delete)
	CreateAutomation(ctx context.Context, config AutomationConfig) error
	UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error
	DeleteAutomation(ctx context.Context, automationID string) error

	// Script operations (REST-only for create/update/delete)
	CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error
	UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error
	DeleteScript(ctx context.Context, scriptID string) error

	// Scene operations (REST-only for create/update/delete/get)
	GetScene(ctx context.Context, sceneID string) (*Scene, error)
	CreateScene(ctx context.Context, sceneID string, config SceneConfig) error
	UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error
	DeleteScene(ctx context.Context, sceneID string) error

	// Config Entry Flow operations (for helpers requiring HTTP-based flow)
	InitConfigEntryFlow(ctx context.Context, handler string) (*ConfigEntryFlowResult, error)
	SubmitConfigEntryFlowStep(ctx context.Context, flowID string, data map[string]any) (*ConfigEntryFlowResult, error)
	DeleteConfigEntry(ctx context.Context, entryID string) error

	// Config Entry Options Flow operations (for reading current option values)
	InitConfigEntryOptionsFlow(ctx context.Context, entryID string) (*OptionsFlowResult, error)
	SubmitConfigEntryOptionsFlowStep(ctx context.Context, flowID string, data map[string]any) (*OptionsFlowResult, error)
	AbortConfigEntryOptionsFlow(ctx context.Context, flowID string) error

	// Service discovery
	GetServices(ctx context.Context) ([]Service, error)

	// System configuration
	GetConfig(ctx context.Context) (*Config, error)

	// Template rendering
	RenderTemplate(ctx context.Context, platformTemplate string) (string, error)

	// Logbook
	GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error)

	// Configuration validation
	CheckConfig(ctx context.Context) (*ConfigCheckResult, error)

	// Calendar operations
	GetCalendars(ctx context.Context) ([]CalendarEntry, error)
	GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error)

	// Camera operations
	GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error)
}

// HybridClient combines WebSocket and REST API clients for Home Assistant.
// It uses WebSocket for most operations but falls back to REST for operations
// that are not supported via WebSocket (e.g., deleting automations/scripts/scenes).
type HybridClient struct {
	ws   WSOperations   // WebSocket client for most operations
	rest RESTOperations // REST client for delete operations
}

// NewHybridClient creates a new hybrid client with the given WebSocket and REST clients.
func NewHybridClient(ws *WSClient, rest *RESTClient) *HybridClient {
	return &HybridClient{
		ws:   &wsClientImpl{ws: ws},
		rest: rest,
	}
}

// NewHybridClientWithInterfaces creates a new hybrid client with custom interfaces.
// This is useful for testing with mock implementations.
func NewHybridClientWithInterfaces(ws WSOperations, rest RESTOperations) *HybridClient {
	return &HybridClient{
		ws:   ws,
		rest: rest,
	}
}

// Ensure HybridClient implements Client interface at compile time.
var _ Client = (*HybridClient)(nil)

// =============================================================================
// Core State Operations (delegated to WebSocket)
// =============================================================================

// GetStates retrieves all entity states.
func (c *HybridClient) GetStates(ctx context.Context) ([]Entity, error) {
	return c.ws.GetStates(ctx)
}

// GetState retrieves the state of a specific entity.
func (c *HybridClient) GetState(ctx context.Context, entityID string) (*Entity, error) {
	return c.ws.GetState(ctx, entityID)
}

// SetState sets the state of an entity.
func (c *HybridClient) SetState(ctx context.Context, entityID string, state StateUpdate) (*Entity, error) {
	return c.ws.SetState(ctx, entityID, state)
}

// GetHistory retrieves historical state changes for an entity.
func (c *HybridClient) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error) {
	return c.ws.GetHistory(ctx, entityID, start, end)
}

// CallService calls a Home Assistant service.
func (c *HybridClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	return c.ws.CallService(ctx, domain, service, data)
}

// CallServiceWithResponse calls a service and returns the response data.
func (c *HybridClient) CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	return c.ws.CallServiceWithResponse(ctx, domain, service, data)
}

// =============================================================================
// Automation Operations (hybrid: WebSocket + REST for delete)
// =============================================================================

// ListAutomations lists all automations.
func (c *HybridClient) ListAutomations(ctx context.Context) ([]Automation, error) {
	return c.ws.ListAutomations(ctx)
}

// GetAutomation retrieves a specific automation by ID.
func (c *HybridClient) GetAutomation(ctx context.Context, automationID string) (*Automation, error) {
	return c.ws.GetAutomation(ctx, automationID)
}

// CreateAutomation creates a new automation using the REST API.
// Note: Call automation.reload after creation to make the entity visible.
// The WebSocket config/automation/create command is not available in all HA versions.
func (c *HybridClient) CreateAutomation(ctx context.Context, config AutomationConfig) error {
	return c.rest.CreateAutomation(ctx, config)
}

// UpdateAutomation updates an existing automation using the REST API.
// Note: Call automation.reload after update to make changes visible.
func (c *HybridClient) UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error {
	return c.rest.UpdateAutomation(ctx, automationID, config)
}

// DeleteAutomation deletes an automation using the REST API.
// The WebSocket API does not support automation deletion reliably.
func (c *HybridClient) DeleteAutomation(ctx context.Context, automationID string) error {
	return c.rest.DeleteAutomation(ctx, automationID)
}

// ToggleAutomation enables or disables an automation.
func (c *HybridClient) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	return c.ws.ToggleAutomation(ctx, entityID, enabled)
}

// =============================================================================
// Helper Operations (hybrid: WebSocket for most, REST Config Entry Flow for some)
// =============================================================================

// ListHelpers lists all input helpers.
func (c *HybridClient) ListHelpers(ctx context.Context) ([]Entity, error) {
	return c.ws.ListHelpers(ctx)
}

// CreateHelper creates a new input helper.
// For Config Entry platforms (threshold, derivative, integration, group, platformTemplate),
// this uses the HTTP Config Entry Flow mechanism with icon support via Entity Registry.
// For standard helpers (input_*, counter, timer, schedule), this uses WebSocket.
func (c *HybridClient) CreateHelper(ctx context.Context, config HelperConfig) error {
	if RequiresConfigEntryFlow(config.Platform) {
		// Extract icon before creating helper (Config Entry Flow doesn't support icons in create)
		icon, hasIcon := config.Config["icon"].(string)
		if hasIcon && icon != "" {
			// Remove icon from config to prevent API error
			delete(config.Config, "icon")
		}

		// Create helper via Config Entry Flow
		if err := c.createHelperViaConfigFlow(ctx, config); err != nil {
			return err
		}

		// Set icon via Entity Registry if provided
		if hasIcon && icon != "" {
			// Predict entity ID based on name and platform
			entityID, err := c.predictEntityIDForConfigEntry(ctx, config)
			if err != nil {
				// Non-fatal: helper was created successfully
				return fmt.Errorf("helper created but failed to predict entity ID for icon update: %w", err)
			}

			// Wait for entity to appear in registry before setting icon
			WaitForEntityAppear(ctx, c.ws.GetState, entityID, DefaultEntityPollerConfig())

			// Update icon via Entity Registry
			updateCfg := EntityRegistryUpdateConfig{
				Icon: &icon,
			}
			if _, err := c.ws.UpdateEntityRegistryEntry(ctx, entityID, updateCfg); err != nil {
				// Non-fatal: helper was created successfully
				return fmt.Errorf("helper created as %s but failed to set icon: %w", entityID, err)
			}
		}

		return nil
	}
	return c.ws.CreateHelper(ctx, config)
}

// UpdateHelper updates an existing input helper.
// For Config Entry platforms, this looks up the config_entry_id from the entity registry
// and updates via Options Flow REST API.
// For standard helpers, this uses WebSocket.
func (c *HybridClient) UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error {
	// Check if this entity belongs to a Config Entry platform
	// Config Entry entities have their config_entry_id in the entity registry
	entries, err := c.ws.GetEntityRegistry(ctx)
	if err != nil {
		// Fall back to WebSocket if registry lookup fails
		return c.ws.UpdateHelper(ctx, helperID, config)
	}

	// Find the entity and check if it has a config_entry_id
	for _, entry := range entries {
		if entry.EntityID == helperID && entry.ConfigEntryID != "" {
			// This is a Config Entry-based helper, update via Options Flow
			return c.updateHelperViaOptionsFlow(ctx, helperID, entry.ConfigEntryID, config)
		}
	}

	// Not a Config Entry helper, use WebSocket
	return c.ws.UpdateHelper(ctx, helperID, config)
}

// DeleteHelper deletes an input helper.
// For Config Entry platforms, this looks up the config_entry_id from the entity registry
// and deletes the config entry via REST API.
// For standard helpers, this uses WebSocket.
func (c *HybridClient) DeleteHelper(ctx context.Context, helperID string) error {
	// Check if this entity belongs to a Config Entry platform
	// Config Entry entities have their config_entry_id in the entity registry
	entries, err := c.ws.GetEntityRegistry(ctx)
	if err != nil {
		// Fall back to WebSocket if registry lookup fails
		return c.ws.DeleteHelper(ctx, helperID)
	}

	// Find the entity and check if it has a config_entry_id
	for _, entry := range entries {
		if entry.EntityID == helperID && entry.ConfigEntryID != "" {
			// This is a Config Entry-based helper, delete via REST
			return c.rest.DeleteConfigEntry(ctx, entry.ConfigEntryID)
		}
	}

	// Not a Config Entry helper, use WebSocket
	return c.ws.DeleteHelper(ctx, helperID)
}

// updateHelperViaOptionsFlow updates a Config Entry-based helper via Options Flow REST API.
func (c *HybridClient) updateHelperViaOptionsFlow(ctx context.Context, entityID, configEntryID string, config HelperConfig) error {
	// Extract icon from config - Options Flow doesn't support icons
	icon, hasIcon := config.Config["icon"].(string)
	if hasIcon {
		delete(config.Config, "icon")
	}

	// Init Options Flow
	result, err := c.rest.InitConfigEntryOptionsFlow(ctx, configEntryID)
	if err != nil {
		return fmt.Errorf("init options flow: %w", err)
	}

	// Navigate menu if needed (e.g., template helpers have sensor/binary_sensor menu)
	if result.Type == flowTypeMenu {
		result, err = c.navigateOptionsFlowMenu(ctx, configEntryID, result)
		if err != nil {
			// Abort flow on error
			_ = c.rest.AbortConfigEntryOptionsFlow(ctx, result.FlowID)
			return fmt.Errorf("navigate options flow menu: %w", err)
		}
	}

	// Extract current values from schema
	currentValues := extractOptionsFromSchema(result.DataSchema)

	// Merge user-provided values with current values
	mergedConfig := mergeOptionsFlowConfig(currentValues, config.Config)

	// Submit merged config
	submitResult, err := c.rest.SubmitConfigEntryOptionsFlowStep(ctx, result.FlowID, mergedConfig)
	if err != nil {
		_ = c.rest.AbortConfigEntryOptionsFlow(ctx, result.FlowID)
		return fmt.Errorf("submit options flow: %w", err)
	}

	// Validate result
	if submitResult.Type != flowTypeCreateEntry {
		_ = c.rest.AbortConfigEntryOptionsFlow(ctx, result.FlowID)
		return fmt.Errorf("unexpected options flow result type: %s", submitResult.Type)
	}

	// Update icon via Entity Registry if provided
	if hasIcon && icon != "" {
		WaitForEntityAppear(ctx, c.ws.GetState, entityID, DefaultEntityPollerConfig())
		updateCfg := EntityRegistryUpdateConfig{Icon: &icon}
		if _, err := c.ws.UpdateEntityRegistryEntry(ctx, entityID, updateCfg); err != nil {
			return fmt.Errorf("helper updated, but failed to set icon: %w", err)
		}
	}

	return nil
}

// mergeOptionsFlowConfig merges user-provided config values with current schema values.
// Only fields present in userConfig override the current values.
func mergeOptionsFlowConfig(currentValues, userConfig map[string]any) map[string]any {
	merged := make(map[string]any)

	// Start with current values
	for k, v := range currentValues {
		merged[k] = v
	}

	// Override with user-provided values
	for k, v := range userConfig {
		merged[k] = v
	}

	return merged
}

// SetHelperValue sets the value of an input helper.
func (c *HybridClient) SetHelperValue(ctx context.Context, entityID string, value any) error {
	return c.ws.SetHelperValue(ctx, entityID, value)
}

// predictEntityIDForConfigEntry predicts the entity ID that will be created for a Config Entry helper.
// This is needed to update the icon via Entity Registry after creation.
func (c *HybridClient) predictEntityIDForConfigEntry(_ context.Context, config HelperConfig) (string, error) {
	name, ok := config.Config["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name not found in config")
	}

	// Determine entity domain based on platform
	domain := c.determineEntityDomainForConfigEntry(config)
	if domain == "" {
		return "", fmt.Errorf("could not determine entity domain for platform %s", config.Platform)
	}

	// Slugify name (same logic as Home Assistant)
	slug := slugifyEntityName(name)

	return fmt.Sprintf("%s.%s", domain, slug), nil
}

// staticPlatformDomains maps Config Entry platforms to their static entity domains.
var staticPlatformDomains = map[string]string{
	"threshold":          domainBinarySensor,
	"derivative":         domainSensor,
	"integration":        domainSensor,
	"utility_meter":      domainSensor,
	"min_max":            domainSensor,
	"statistics":         domainSensor,
	"trend":              domainBinarySensor,
	"filter":             domainSensor,
	"tod":                domainBinarySensor,
	"generic_thermostat": domainClimate,
	"generic_hygrostat":  domainHumidifier,
}

// determineEntityDomainForConfigEntry determines the entity domain for a Config Entry helper.
func (c *HybridClient) determineEntityDomainForConfigEntry(config HelperConfig) string {
	// Check static domain mapping first
	if domain, ok := staticPlatformDomains[config.Platform]; ok {
		return domain
	}

	// Handle dynamic domain determination
	switch config.Platform {
	case platformTemplate:
		// Template can be sensor or binary_sensor
		return c.determineTemplateSubtype(config)
	case platformGroup:
		// Group type depends on member entities
		return c.determineGroupSubtype(config)
	case platformRandom:
		// Random can be sensor or binary_sensor
		return c.determineRandomSubtype(config)
	case platformSwitchAsX:
		// Switch_as_x creates entities based on target_domain
		return c.determineSwitchAsXSubtype(config)
	default:
		return ""
	}
}

// slugifyEntityName converts a name to a valid entity ID slug (same logic as Home Assistant).
func slugifyEntityName(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces and special characters with underscores
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, slug)
	// Remove leading/trailing underscores
	slug = strings.Trim(slug, "_")
	// Collapse multiple underscores
	for strings.Contains(slug, "__") {
		slug = strings.ReplaceAll(slug, "__", "_")
	}
	return slug
}

// createHelperViaConfigFlow creates a helper using the HTTP Config Entry Flow.
// This handles multi-step flows: init -> (menu) -> form(s) -> verify creation.
func (c *HybridClient) createHelperViaConfigFlow(ctx context.Context, config HelperConfig) error {
	// Step 1: Initialize the config entry flow
	flowResult, err := c.rest.InitConfigEntryFlow(ctx, config.Platform)
	if err != nil {
		return fmt.Errorf("init config entry flow: %w", err)
	}

	// Step 2: Handle menu step if present (for helpers requiring subtype selection)
	if flowResult.Type == "menu" {
		subtype := c.determineHelperSubtype(config)
		if subtype == "" {
			return fmt.Errorf("config entry flow requires subtype selection but none provided")
		}
		flowResult, err = c.rest.SubmitConfigEntryFlowStep(ctx, flowResult.FlowID, map[string]any{
			"next_step_id": subtype,
		})
		if err != nil {
			return fmt.Errorf("submit config entry flow menu step: %w", err)
		}
	}

	// Step 3: Handle intermediate form steps
	// Some platforms (statistics, trend, filter) require multiple form submissions
	maxSteps := 5 // Safety limit to prevent infinite loops
	for i := 0; i < maxSteps && flowResult.Type == flowTypeForm; i++ {
		// Save step ID before submission (in case of error, flowResult might be nil)
		currentStepID := flowResult.StepID

		// Transform config for the current step
		stepConfig := c.buildConfigForFlowStep(config, currentStepID)

		flowResult, err = c.rest.SubmitConfigEntryFlowStep(ctx, flowResult.FlowID, stepConfig)
		if err != nil {
			return fmt.Errorf("submit config entry flow step %s: %w", currentStepID, err)
		}

		// Check if we're done
		if flowResult.Type == "create_entry" {
			return nil // Success
		}

		if flowResult.Type == "abort" {
			return fmt.Errorf("config entry flow aborted: %s", flowResult.Description)
		}

		// Check for validation errors
		if flowResult.Type == flowTypeForm && len(flowResult.Errors) > 0 {
			return fmt.Errorf("config entry flow validation errors: %v", flowResult.Errors)
		}
	}

	// Still in form state after max steps
	if flowResult.Type == flowTypeForm {
		return fmt.Errorf("config entry flow exceeded max steps (last step_id: %s)", flowResult.StepID)
	}

	return fmt.Errorf("unexpected config entry flow result type: %s", flowResult.Type)
}

// buildConfigForFlowStep builds configuration data for a specific flow step.
// Different platforms have different step IDs that require specific subsets of config.
func (c *HybridClient) buildConfigForFlowStep(config HelperConfig, stepID string) map[string]any {
	// Platform-specific step handling
	switch config.Platform {
	case platformStatistics:
		// Statistics "state_characteristic" step needs ONLY state_characteristic field
		if stepID == "state_characteristic" {
			result := make(map[string]any)
			if characteristic, ok := config.Config["state_characteristic"].(string); ok {
				result["state_characteristic"] = characteristic
			} else {
				result["state_characteristic"] = "mean" // Default
			}
			return result
		}
		// Statistics "options" step wants entity_id and sampling_size/max_age (NO name)
		if stepID == "options" {
			result := c.transformConfigForFlow(config)
			delete(result, "name")                 // name goes in next step
			delete(result, "state_characteristic") // Already set in previous step
			return result
		}
		// Statistics "user" step wants entity_id (and possibly name)
		if stepID == "user" {
			result := c.transformConfigForFlow(config)
			delete(result, "sampling_size")        // Already set in options step
			delete(result, "max_age")              // Already set in options step
			delete(result, "state_characteristic") // Already set in first step
			return result
		}
		// Other steps get full config
		return c.transformConfigForFlow(config)

	case platformTrend:
		// Trend "settings" step does NOT want "name" field
		if stepID == "settings" {
			result := c.transformConfigForFlow(config)
			delete(result, "name") // Remove name from settings step
			return result
		}
		// Other steps get full config
		return c.transformConfigForFlow(config)

	case platformFilter:
		// Filter "user" step wants entity_id, name, and filter (all together)
		if stepID == "user" {
			return c.transformConfigForFlow(config)
		}
		// Second step (filter-specific like "outlier", "lowpass", etc.) wants entity_id only (NO name, NO filter)
		result := c.transformConfigForFlow(config)
		delete(result, "name")   // name already set in user step
		delete(result, "filter") // filter already set in user step
		return result

	default:
		// Default: return full transformed config
		return c.transformConfigForFlow(config)
	}
}

// determineHelperSubtype extracts the subtype for multi-step flows.
func (c *HybridClient) determineHelperSubtype(config HelperConfig) string {
	// Check for explicit "group_type" field for menu selection
	if gt, ok := config.Config["group_type"].(string); ok {
		return gt
	}

	switch config.Platform {
	case platformTemplate:
		return c.determineTemplateSubtype(config)
	case platformGroup:
		return c.determineGroupSubtype(config)
	case platformRandom:
		return c.determineRandomSubtype(config)
	case platformSwitchAsX:
		return c.determineSwitchAsXSubtype(config)
	case platformStatistics:
		// Statistics requires menu step for state_characteristic
		if characteristic, ok := config.Config["state_characteristic"].(string); ok {
			return characteristic
		}
		return "mean" // Default to mean if not specified
	}
	return ""
}

// determineTemplateSubtype determines whether a platformTemplate is a sensor or binary_sensor.
func (c *HybridClient) determineTemplateSubtype(config HelperConfig) string {
	// Check explicit type field first (for backward compatibility)
	if t, ok := config.Config["type"].(string); ok {
		return t
	}
	// Check platformTemplate_type field (set by buildTemplateConfig)
	if tt, ok := config.Config["platformTemplate_type"].(string); ok {
		return tt
	}
	// Infer from device_class
	if dc, ok := config.Config["device_class"].(string); ok && binaryDeviceClasses[dc] {
		return domainBinarySensor
	}
	return domainSensor
}

// determineRandomSubtype determines whether a random helper is a sensor or binary_sensor.
func (c *HybridClient) determineRandomSubtype(config HelperConfig) string {
	// Check explicit type field (routing field)
	if t, ok := config.Config["type"].(string); ok {
		return t
	}
	// Default to sensor
	return domainSensor
}

// determineSwitchAsXSubtype determines the target domain for switch_as_x helper.
func (c *HybridClient) determineSwitchAsXSubtype(config HelperConfig) string {
	// Check explicit target_domain field (routing field)
	if td, ok := config.Config["target_domain"].(string); ok {
		return td
	}
	// Default to light
	return domainLight
}

// firstEntityDomainFromConfig extracts the domain from the first entity in the config.
// Handles both []string and []any types (from JSON unmarshaling).
func firstEntityDomainFromConfig(config map[string]any) string {
	// Try []string first
	if entities, ok := config["entities"].([]string); ok && len(entities) > 0 {
		return extractEntityDomain(entities[0])
	}
	// Try []any (from JSON unmarshaling)
	if entities, ok := config["entities"].([]any); ok && len(entities) > 0 {
		if first, ok := entities[0].(string); ok {
			return extractEntityDomain(first)
		}
	}
	return ""
}

// determineGroupSubtype infers the group type from member entities.
func (c *HybridClient) determineGroupSubtype(config HelperConfig) string {
	if domain := firstEntityDomainFromConfig(config.Config); domain != "" {
		if groupType, ok := entityDomainToGroupType[domain]; ok {
			return groupType
		}
	}
	return domainSensor
}

// extractEntityDomain extracts the domain prefix from an entity ID.
func extractEntityDomain(entityID string) string {
	if idx := strings.Index(entityID, "."); idx > 0 {
		return entityID[:idx]
	}
	return ""
}

// transformConfigForFlow transforms helper config to match Config Entry Flow schema.
func (c *HybridClient) transformConfigForFlow(config HelperConfig) map[string]any {
	result := make(map[string]any)

	for k, v := range config.Config {
		if c.shouldSkipConfigField(k, config.Platform) {
			continue
		}
		result[k] = c.transformFieldValue(k, v)
	}

	c.addSensorGroupDefaults(config, result)
	return result
}

// platformSkipFields maps platforms to fields that should be skipped during transformation.
var platformSkipFields = map[string]map[string]bool{
	platformTemplate:     {"type": true, "template_type": true},
	platformRandom:       {"type": true},
	platformSwitchAsX:    {"target_domain": true},
	platformStatistics:   {"state_characteristic": true},
	"generic_thermostat": {"heater_entity_id": true, "target_sensor_entity_id": true},
	"generic_hygrostat":  {"humidifier_entity_id": true, "target_sensor_entity_id": true},
}

// shouldSkipConfigField checks if a config field should be skipped during transformation.
func (c *HybridClient) shouldSkipConfigField(key, platform string) bool {
	// Always skip group_type (internal routing field)
	if key == "group_type" {
		return true
	}

	// Check platform-specific skip fields
	if skipFields, ok := platformSkipFields[platform]; ok {
		if skipFields[key] {
			return true
		}
	}

	// Skip icon for Config Entry Flow platforms (not supported in create flow)
	// Icons should be set via entity registry after creation
	if key == "icon" && RequiresConfigEntryFlow(platform) {
		return true
	}

	return false
}

// transformFieldValue transforms a config field value if needed.
func (c *HybridClient) transformFieldValue(key string, value any) any {
	if strVal, ok := value.(string); ok && isDurationField(key) {
		if duration := parseDurationString(strVal); duration != nil {
			return duration
		}
	}
	return value
}

// addSensorGroupDefaults adds default aggregation type for sensor groups.
func (c *HybridClient) addSensorGroupDefaults(config HelperConfig, result map[string]any) {
	if config.Platform != "group" {
		return
	}
	domain := firstEntityDomainFromConfig(config.Config)
	if sensorGroupDomains[domain] {
		if _, hasType := result["type"]; !hasType {
			result["type"] = "sum"
		}
	}
}

// isDurationField checks if a field name typically contains duration values.
func isDurationField(fieldName string) bool {
	durationFields := map[string]bool{
		"time_window":      true,
		"delay_on":         true,
		"delay_off":        true,
		"max_sub_interval": true,
	}
	return durationFields[fieldName]
}

// parseDurationString converts "HH:MM:SS" format to Config Entry Flow dict format.
func parseDurationString(s string) map[string]int {
	var hours, minutes, seconds int
	n, err := fmt.Sscanf(s, "%d:%d:%d", &hours, &minutes, &seconds)
	if err != nil || n != 3 {
		return nil
	}
	return map[string]int{
		"hours":   hours,
		"minutes": minutes,
		"seconds": seconds,
	}
}

// =============================================================================
// Script Operations (hybrid: WebSocket + REST for delete)
// =============================================================================

// ListScripts lists all scripts.
func (c *HybridClient) ListScripts(ctx context.Context) ([]Entity, error) {
	return c.ws.ListScripts(ctx)
}

// GetScript retrieves a specific script by ID.
func (c *HybridClient) GetScript(ctx context.Context, scriptID string) (*Script, error) {
	return c.ws.GetScript(ctx, scriptID)
}

// CreateScript creates a new script using the REST API.
// The WebSocket API does not support script creation reliably.
func (c *HybridClient) CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	return c.rest.CreateScript(ctx, scriptID, config)
}

// UpdateScript updates an existing script using the REST API.
// The WebSocket API does not support script updates reliably.
func (c *HybridClient) UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	return c.rest.UpdateScript(ctx, scriptID, config)
}

// DeleteScript deletes a script using the REST API.
// The WebSocket API may not support script deletion reliably.
func (c *HybridClient) DeleteScript(ctx context.Context, scriptID string) error {
	return c.rest.DeleteScript(ctx, scriptID)
}

// =============================================================================
// Scene Operations (hybrid: WebSocket + REST for delete)
// =============================================================================

// ListScenes lists all scenes.
func (c *HybridClient) ListScenes(ctx context.Context) ([]Entity, error) {
	return c.ws.ListScenes(ctx)
}

// GetScene retrieves the full configuration of a scene by ID using the REST API.
func (c *HybridClient) GetScene(ctx context.Context, sceneID string) (*Scene, error) {
	return c.rest.GetScene(ctx, sceneID)
}

// CreateScene creates a new scene using the REST API.
// Note: The REST API stores the config but the entity may not appear until
// Home Assistant is restarted or scene.reload is called. The WebSocket
// config/scene/create command is not available in all HA versions.
func (c *HybridClient) CreateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	return c.rest.CreateScene(ctx, sceneID, config)
}

// UpdateScene updates an existing scene using the REST API.
// Note: The REST API stores the config but changes may not appear until
// Home Assistant is restarted or scene.reload is called.
func (c *HybridClient) UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	return c.rest.UpdateScene(ctx, sceneID, config)
}

// DeleteScene deletes a scene using the REST API.
// The WebSocket API may not support scene deletion reliably.
func (c *HybridClient) DeleteScene(ctx context.Context, sceneID string) error {
	return c.rest.DeleteScene(ctx, sceneID)
}

// =============================================================================
// Registry Operations (delegated to WebSocket)
// =============================================================================

// GetEntityRegistry retrieves the entity registry.
func (c *HybridClient) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	return c.ws.GetEntityRegistry(ctx)
}

// GetDeviceRegistry retrieves the device registry.
func (c *HybridClient) GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error) {
	return c.ws.GetDeviceRegistry(ctx)
}

// GetAreaRegistry retrieves the area registry.
func (c *HybridClient) GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error) {
	return c.ws.GetAreaRegistry(ctx)
}

// CreateArea creates a new area in the area registry.
func (c *HybridClient) CreateArea(ctx context.Context, config AreaConfig) (*AreaRegistryEntry, error) {
	return c.ws.CreateArea(ctx, config)
}

// UpdateArea updates an existing area in the area registry.
func (c *HybridClient) UpdateArea(ctx context.Context, areaID string, config AreaConfig) (*AreaRegistryEntry, error) {
	return c.ws.UpdateArea(ctx, areaID, config)
}

// DeleteArea deletes an area from the area registry.
func (c *HybridClient) DeleteArea(ctx context.Context, areaID string) error {
	return c.ws.DeleteArea(ctx, areaID)
}

// GetLabelRegistry retrieves the label registry.
func (c *HybridClient) GetLabelRegistry(ctx context.Context) ([]LabelRegistryEntry, error) {
	return c.ws.GetLabelRegistry(ctx)
}

// CreateLabel creates a new label in the label registry.
func (c *HybridClient) CreateLabel(ctx context.Context, config LabelConfig) (*LabelRegistryEntry, error) {
	return c.ws.CreateLabel(ctx, config)
}

// UpdateLabel updates an existing label in the label registry.
func (c *HybridClient) UpdateLabel(ctx context.Context, labelID string, config LabelConfig) (*LabelRegistryEntry, error) {
	return c.ws.UpdateLabel(ctx, labelID, config)
}

// DeleteLabel deletes a label from the label registry.
func (c *HybridClient) DeleteLabel(ctx context.Context, labelID string) error {
	return c.ws.DeleteLabel(ctx, labelID)
}

// GetFloorRegistry retrieves the floor registry.
func (c *HybridClient) GetFloorRegistry(ctx context.Context) ([]FloorRegistryEntry, error) {
	return c.ws.GetFloorRegistry(ctx)
}

// CreateFloor creates a new floor in the floor registry.
func (c *HybridClient) CreateFloor(ctx context.Context, config FloorConfig) (*FloorRegistryEntry, error) {
	return c.ws.CreateFloor(ctx, config)
}

// UpdateFloor updates an existing floor in the floor registry.
func (c *HybridClient) UpdateFloor(ctx context.Context, floorID string, config FloorConfig) (*FloorRegistryEntry, error) {
	return c.ws.UpdateFloor(ctx, floorID, config)
}

// DeleteFloor deletes a floor from the floor registry.
func (c *HybridClient) DeleteFloor(ctx context.Context, floorID string) error {
	return c.ws.DeleteFloor(ctx, floorID)
}

// GetZones retrieves all zones.
func (c *HybridClient) GetZones(ctx context.Context) ([]ZoneRegistryEntry, error) {
	return c.ws.GetZones(ctx)
}

// CreateZone creates a new zone.
func (c *HybridClient) CreateZone(ctx context.Context, config ZoneConfig) (*ZoneRegistryEntry, error) {
	return c.ws.CreateZone(ctx, config)
}

// UpdateZone updates an existing zone.
func (c *HybridClient) UpdateZone(ctx context.Context, zoneID string, config ZoneConfig) (*ZoneRegistryEntry, error) {
	return c.ws.UpdateZone(ctx, zoneID, config)
}

// DeleteZone deletes a zone.
func (c *HybridClient) DeleteZone(ctx context.Context, zoneID string) error {
	return c.ws.DeleteZone(ctx, zoneID)
}

// GetPersons retrieves all persons.
func (c *HybridClient) GetPersons(ctx context.Context) ([]PersonRegistryEntry, error) {
	return c.ws.GetPersons(ctx)
}

// CreatePerson creates a new person.
func (c *HybridClient) CreatePerson(ctx context.Context, config PersonConfig) (*PersonRegistryEntry, error) {
	return c.ws.CreatePerson(ctx, config)
}

// UpdatePerson updates an existing person.
func (c *HybridClient) UpdatePerson(ctx context.Context, personID string, config PersonConfig) (*PersonRegistryEntry, error) {
	return c.ws.UpdatePerson(ctx, personID, config)
}

// DeletePerson deletes a person.
func (c *HybridClient) DeletePerson(ctx context.Context, personID string) error {
	return c.ws.DeletePerson(ctx, personID)
}

// GetTags retrieves all tags.
func (c *HybridClient) GetTags(ctx context.Context) ([]TagRegistryEntry, error) {
	return c.ws.GetTags(ctx)
}

// CreateTag creates a new tag.
func (c *HybridClient) CreateTag(ctx context.Context, config TagConfig) (*TagRegistryEntry, error) {
	return c.ws.CreateTag(ctx, config)
}

// UpdateTag updates an existing tag.
func (c *HybridClient) UpdateTag(ctx context.Context, tagID string, config TagConfig) (*TagRegistryEntry, error) {
	return c.ws.UpdateTag(ctx, tagID, config)
}

// DeleteTag deletes a tag.
func (c *HybridClient) DeleteTag(ctx context.Context, tagID string) error {
	return c.ws.DeleteTag(ctx, tagID)
}

// RemoveEntityRegistryEntry removes an entity from the entity registry.
func (c *HybridClient) RemoveEntityRegistryEntry(ctx context.Context, entityID string) error {
	return c.ws.RemoveEntityRegistryEntry(ctx, entityID)
}

// UpdateEntityRegistryEntry updates an existing entity in the entity registry.
func (c *HybridClient) UpdateEntityRegistryEntry(ctx context.Context, entityID string, config EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
	return c.ws.UpdateEntityRegistryEntry(ctx, entityID, config)
}

// RemoveDeviceConfigEntry removes a config entry from a device.
func (c *HybridClient) RemoveDeviceConfigEntry(ctx context.Context, deviceID, configEntryID string) error {
	return c.ws.RemoveDeviceConfigEntry(ctx, deviceID, configEntryID)
}

// UpdateDeviceRegistryEntry updates an existing device in the device registry.
func (c *HybridClient) UpdateDeviceRegistryEntry(ctx context.Context, deviceID string, config DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error) {
	return c.ws.UpdateDeviceRegistryEntry(ctx, deviceID, config)
}

// =============================================================================
// Media Operations (delegated to WebSocket)
// =============================================================================

// SignPath generates a signed URL for authenticated access.
func (c *HybridClient) SignPath(ctx context.Context, path string, expires int) (string, error) {
	return c.ws.SignPath(ctx, path, expires)
}

// GetCameraStream gets a camera stream URL.
func (c *HybridClient) GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error) {
	return c.ws.GetCameraStream(ctx, entityID)
}

// BrowseMedia browses media content.
func (c *HybridClient) BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error) {
	return c.ws.BrowseMedia(ctx, mediaContentID)
}

// =============================================================================
// Dashboard Operations (delegated to WebSocket)
// =============================================================================

// GetLovelaceConfig retrieves the Lovelace dashboard configuration.
func (c *HybridClient) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	return c.ws.GetLovelaceConfig(ctx, urlPath)
}

// SaveLovelaceConfig saves configuration for a Lovelace dashboard.
func (c *HybridClient) SaveLovelaceConfig(ctx context.Context, urlPath string, config map[string]any) error {
	return c.ws.SaveLovelaceConfig(ctx, urlPath, config)
}

// ListDashboards retrieves all Lovelace dashboards.
func (c *HybridClient) ListDashboards(ctx context.Context) ([]DashboardEntry, error) {
	return c.ws.ListDashboards(ctx)
}

// CreateDashboard creates a new Lovelace dashboard.
func (c *HybridClient) CreateDashboard(ctx context.Context, config DashboardConfig) (*DashboardEntry, error) {
	return c.ws.CreateDashboard(ctx, config)
}

// UpdateDashboard updates an existing Lovelace dashboard.
func (c *HybridClient) UpdateDashboard(ctx context.Context, dashboardID string, config DashboardConfig) (*DashboardEntry, error) {
	return c.ws.UpdateDashboard(ctx, dashboardID, config)
}

// DeleteDashboard deletes a Lovelace dashboard.
func (c *HybridClient) DeleteDashboard(ctx context.Context, dashboardID string) error {
	return c.ws.DeleteDashboard(ctx, dashboardID)
}

// =============================================================================
// Statistics Operations (delegated to WebSocket)
// =============================================================================

// GetStatistics retrieves long-term statistics for entities.
func (c *HybridClient) GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error) {
	return c.ws.GetStatistics(ctx, statIDs, period)
}

// =============================================================================
// Target Operations (delegated to WebSocket)
// =============================================================================

// GetTriggersForTarget retrieves all applicable triggers for the given target.
func (c *HybridClient) GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.ws.GetTriggersForTarget(ctx, target, expandGroup)
}

// GetConditionsForTarget retrieves all applicable conditions for the given target.
func (c *HybridClient) GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.ws.GetConditionsForTarget(ctx, target, expandGroup)
}

// GetServicesForTarget retrieves all applicable services for the given target.
func (c *HybridClient) GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.ws.GetServicesForTarget(ctx, target, expandGroup)
}

// ExtractFromTarget extracts entities, devices, and areas from the specified target.
func (c *HybridClient) ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error) {
	return c.ws.ExtractFromTarget(ctx, target, expandGroup)
}

// =============================================================================
// Schedule Config Operations (delegated to WebSocket)
// =============================================================================

// GetScheduleConfig retrieves the full configuration of a schedule helper.
func (c *HybridClient) GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error) {
	return c.ws.GetScheduleConfig(ctx, scheduleID)
}

// =============================================================================
// Config Entry Operations (delegated to WebSocket)
// =============================================================================

// GetConfigEntries retrieves config entries, optionally filtered by domain.
func (c *HybridClient) GetConfigEntries(ctx context.Context, domain string) ([]ConfigEntryFull, error) {
	return c.ws.GetConfigEntries(ctx, domain)
}

// GetConfigEntry retrieves a single config entry by its entry ID.
func (c *HybridClient) GetConfigEntry(ctx context.Context, entryID string) (*ConfigEntryFull, error) {
	return c.ws.GetConfigEntry(ctx, entryID)
}

// GetConfigEntryOptions retrieves the current option values for a config entry
// using the Options Flow REST API. This is necessary because the WebSocket API's
// config_entries/get_single command does not populate the Options field.
func (c *HybridClient) GetConfigEntryOptions(ctx context.Context, entryID string) (map[string]any, error) {
	// Initialize the options flow
	result, err := c.rest.InitConfigEntryOptionsFlow(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("init options flow: %w", err)
	}

	// Always abort the flow when we're done (cleanup)
	defer func() {
		_ = c.rest.AbortConfigEntryOptionsFlow(context.Background(), result.FlowID)
	}()

	// If the response is a menu (e.g., template helpers show sensor/binary_sensor menu),
	// we need to navigate to the actual form step
	if result.Type == "menu" {
		result, err = c.navigateOptionsFlowMenu(ctx, entryID, result)
		if err != nil {
			return nil, err
		}
	}

	// Extract option values from data_schema suggested_value fields
	return extractOptionsFromSchema(result.DataSchema), nil
}

// DeleteConfigEntry deletes a config entry and all its associated devices/entities.
// REST-only: Home Assistant has no reliable config_entries/{delete,remove} WS command,
// same class of gap as automation/script/scene CRUD (see CLAUDE.md).
func (c *HybridClient) DeleteConfigEntry(ctx context.Context, entryID string) error {
	return c.rest.DeleteConfigEntry(ctx, entryID)
}

// navigateOptionsFlowMenu navigates an options flow menu to the correct form step.
func (c *HybridClient) navigateOptionsFlowMenu(ctx context.Context, entryID string, result *OptionsFlowResult) (*OptionsFlowResult, error) {
	// Find the entity domain for this config entry
	entityDomain, err := c.findEntityDomainForConfigEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}

	// Find matching menu option
	menuChoice := findMatchingMenuOption(result.MenuOptions, entityDomain)
	if menuChoice == "" {
		return result, nil // No match found, return original result
	}

	// Navigate to the selected form
	navigatedResult, err := c.rest.SubmitConfigEntryOptionsFlowStep(ctx, result.FlowID, map[string]any{
		"next_step_id": menuChoice,
	})
	if err != nil {
		return nil, fmt.Errorf("navigate menu: %w", err)
	}

	return navigatedResult, nil
}

// findEntityDomainForConfigEntry finds the entity domain for a config entry.
func (c *HybridClient) findEntityDomainForConfigEntry(ctx context.Context, entryID string) (string, error) {
	configEntry, err := c.ws.GetConfigEntry(ctx, entryID)
	if err != nil {
		return "", fmt.Errorf("get config entry for menu navigation: %w", err)
	}

	registry, err := c.ws.GetEntityRegistry(ctx)
	if err != nil {
		return "", fmt.Errorf("get entity registry for menu navigation: %w", err)
	}

	for _, regEntry := range registry {
		if regEntry.ConfigEntryID == configEntry.EntryID {
			// Extract domain from entity_id (format: domain.name)
			parts := strings.SplitN(regEntry.EntityID, ".", 2)
			if len(parts) == 2 {
				return parts[0], nil
			}
		}
	}

	return "", nil
}

// findMatchingMenuOption finds a menu option that matches the entity domain.
func findMatchingMenuOption(menuOptions []string, entityDomain string) string {
	if entityDomain == "" || len(menuOptions) == 0 {
		return ""
	}

	for _, option := range menuOptions {
		if strings.Contains(option, entityDomain) {
			return option
		}
	}

	return ""
}

// extractOptionsFromSchema extracts option values from data schema suggested_value fields.
func extractOptionsFromSchema(dataSchema []OptionsFlowField) map[string]any {
	options := make(map[string]any)
	for _, field := range dataSchema {
		if field.Description != nil {
			if suggestedValue, ok := field.Description["suggested_value"]; ok && suggestedValue != nil {
				options[field.Name] = suggestedValue
			}
		}
	}
	return options
}

// =============================================================================
// Service Discovery Operations (delegated to REST)
// =============================================================================

// GetServices retrieves all available services from Home Assistant.
func (c *HybridClient) GetServices(ctx context.Context) ([]Service, error) {
	return c.rest.GetServices(ctx)
}

// =============================================================================
// System Configuration Operations (delegated to REST)
// =============================================================================

// GetConfig retrieves the Home Assistant system configuration.
func (c *HybridClient) GetConfig(ctx context.Context) (*Config, error) {
	return c.rest.GetConfig(ctx)
}

// =============================================================================
// Template Operations (delegated to REST)
// =============================================================================

// RenderTemplate renders a Jinja2 platformTemplate using Home Assistant state.
func (c *HybridClient) RenderTemplate(ctx context.Context, platformTemplate string) (string, error) {
	return c.rest.RenderTemplate(ctx, platformTemplate)
}

// =============================================================================
// Logbook Operations (delegated to REST)
// =============================================================================

// GetLogbook retrieves logbook entries from Home Assistant.
func (c *HybridClient) GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error) {
	return c.rest.GetLogbook(ctx, startTime, endTime, entityID)
}

// =============================================================================
// Configuration Validation Operations (delegated to REST)
// =============================================================================

// CheckConfig validates the Home Assistant configuration.
func (c *HybridClient) CheckConfig(ctx context.Context) (*ConfigCheckResult, error) {
	return c.rest.CheckConfig(ctx)
}

// =============================================================================
// Calendar Operations (delegated to REST)
// =============================================================================

// GetCalendars retrieves all calendars.
func (c *HybridClient) GetCalendars(ctx context.Context) ([]CalendarEntry, error) {
	return c.rest.GetCalendars(ctx)
}

// GetCalendarEvents retrieves calendar events for a specific calendar.
func (c *HybridClient) GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error) {
	return c.rest.GetCalendarEvents(ctx, entityID, start, end)
}

// =============================================================================
// Camera Operations (delegated to REST for snapshot, WebSocket for stream)
// =============================================================================

// GetCameraSnapshot retrieves a camera snapshot as binary image data.
func (c *HybridClient) GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error) {
	return c.rest.GetCameraSnapshot(ctx, entityID)
}

// =============================================================================
// HACS Operations (delegated to WebSocket)
// =============================================================================

// SendHACSCommand sends a generic HACS WebSocket command.
func (c *HybridClient) SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error) {
	return c.ws.SendHACSCommand(ctx, command, data)
}

// =============================================================================
// System Log Operations (delegated to WebSocket)
// =============================================================================

// GetSystemLog retrieves system log entries from the Home Assistant ring buffer.
func (c *HybridClient) GetSystemLog(ctx context.Context) ([]SystemLogEntry, error) {
	return c.ws.GetSystemLog(ctx)
}

// ClearSystemLog clears the Home Assistant system log ring buffer.
func (c *HybridClient) ClearSystemLog(ctx context.Context) error {
	return c.ws.ClearSystemLog(ctx)
}

// =============================================================================
// HybridClientCloser - implements ClientCloser for proper cleanup
// =============================================================================

// HybridClientCloser extends HybridClient to implement ClientCloser.
type HybridClientCloser struct {
	*HybridClient
	wsClient *WSClient // Keep reference for closing
}

// NewHybridClientCloser creates a hybrid client that implements ClientCloser.
func NewHybridClientCloser(ws *WSClient, rest *RESTClient) *HybridClientCloser {
	return &HybridClientCloser{
		HybridClient: NewHybridClient(ws, rest),
		wsClient:     ws,
	}
}

// Close closes the underlying WebSocket connection.
func (c *HybridClientCloser) Close() error {
	return c.wsClient.Close()
}

// IsConnected returns true if the underlying WebSocket client is connected.
func (c *HybridClientCloser) IsConnected() bool {
	return c.wsClient.IsConnected()
}

// WaitForConnection waits until the underlying WebSocket client is connected.
func (c *HybridClientCloser) WaitForConnection(ctx context.Context) error {
	return c.wsClient.WaitForConnection(ctx)
}

// Ensure HybridClientCloser implements both Client and ClientCloser.
var (
	_ Client       = (*HybridClientCloser)(nil)
	_ ClientCloser = (*HybridClientCloser)(nil)
)

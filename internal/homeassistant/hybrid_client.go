// Package homeassistant provides a hybrid client combining WebSocket and REST APIs.
// coverage-exempt: multi-step Config Entry and Options Flow routing requires real HA API responses
package homeassistant

import (
	"context"
	"fmt"
	"math"
	"strconv"
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

	platformTemplate          = "template"
	platformGroup             = "group"
	platformRandom            = "random"
	platformSwitchAsX         = "switch_as_x"
	platformStatistics        = "statistics"
	platformTrend             = "trend"
	platformFilter            = "filter"
	platformGenericThermostat = "generic_thermostat"
	platformGenericHygrostat  = "generic_hygrostat"

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
	GetEntityRegistryEntry(ctx context.Context, entityID string) (*EntityRegistryEntry, error)
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
	GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error)
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

	// ConfigFileEntryExists checks whether id exists in the config file HA's config API writes
	// to for domain, used to guard against silently creating a duplicate orphan entity.
	ConfigFileEntryExists(ctx context.Context, domain, configID string) (bool, error)

	// Config Entry Flow operations (for helpers requiring HTTP-based flow)
	InitConfigEntryFlow(ctx context.Context, handler string) (*ConfigEntryFlowResult, error)
	SubmitConfigEntryFlowStep(ctx context.Context, flowID string, data map[string]any) (*ConfigEntryFlowResult, error)
	AbortConfigEntryFlow(ctx context.Context, flowID string) error
	DeleteConfigEntry(ctx context.Context, entryID string) (requireRestart bool, err error)

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
			// This is a Config Entry-based helper, delete via REST. require_restart is
			// not surfaced here: DeleteHelper's contract is a plain error, and a helper
			// stuck pending restart still reports success today (pre-existing behavior,
			// unrelated to the manage_config_entry delete action added alongside this).
			_, err := c.rest.DeleteConfigEntry(ctx, entry.ConfigEntryID)
			return err
		}
	}

	// Not a Config Entry helper, use WebSocket
	return c.ws.DeleteHelper(ctx, helperID)
}

// updateHelperViaOptionsFlow updates a Config Entry-based helper via Options Flow REST API.
func (c *HybridClient) updateHelperViaOptionsFlow(ctx context.Context, entityID, configEntryID string, config HelperConfig) error {
	// Extract icon from config - Options Flow doesn't support icons; applied
	// via the Entity Registry after a successful submission instead.
	icon, hasIcon := config.Config["icon"].(string)
	if hasIcon {
		delete(config.Config, "icon")
	}

	// Extract name the same way: a config-entry helper's display name lives
	// in the entity registry, not in any Options Flow schema - most declare
	// no "name" field at all (e.g. min_max, filter), so leaving it in
	// config.Config would either be rejected outright by a PREVENT_EXTRA
	// schema or, after the schema-field check below, treated as an
	// unsupported field.
	name, hasName := config.Config["name"].(string)
	if hasName {
		delete(config.Config, "name")
	}

	result, err := c.rest.InitConfigEntryOptionsFlow(ctx, configEntryID)
	if err != nil {
		return fmt.Errorf("init options flow: %w", err)
	}
	initFlowID := result.FlowID

	// Navigate menu if needed (e.g., template helpers have sensor/binary_sensor menu)
	if result.Type == flowTypeMenu {
		result, err = c.navigateOptionsFlowMenu(ctx, configEntryID, result)
		if err != nil {
			_ = c.rest.AbortConfigEntryOptionsFlow(ctx, initFlowID)
			return fmt.Errorf("navigate options flow menu: %w", err)
		}
	}

	consumed := seedConsumedRoutingKeys(config)
	result, err = c.runOptionsFlowSteps(ctx, initFlowID, result, config.Config, consumed)
	if err != nil {
		return err
	}

	if result.Type != flowTypeCreateEntry {
		_ = c.rest.AbortConfigEntryOptionsFlow(ctx, initFlowID)
		return fmt.Errorf("unexpected options flow result type: %s", result.Type)
	}

	// Reject (not silently drop) any user-supplied field no step's schema
	// declared. Home Assistant's Options Flow forms use PREVENT_EXTRA
	// voluptuous schemas, so a stray key would fail the whole request with
	// an opaque "extra keys not allowed" error if it ever reached HA - but
	// silently dropping it here instead would report the update as
	// successful while quietly discarding a change the caller explicitly
	// asked for. Checked only after every step has had a chance to claim
	// it, so a field belonging to a later step (e.g. generic_thermostat's
	// presets) is not mistaken for an unsupported one.
	if unconsumed := unconsumedUserFields(config.Config, consumed); len(unconsumed) > 0 {
		return fmt.Errorf("helper %q does not support updating field(s): %s", entityID, strings.Join(unconsumed, ", "))
	}

	// Update name/icon via Entity Registry if provided - neither is part of
	// any Options Flow schema.
	return c.applyNameIconViaRegistry(ctx, entityID, icon, hasIcon, name, hasName)
}

// runOptionsFlowSteps submits successive Options Flow form steps, routing
// caller fields via buildStepSubmission, until HA returns something other
// than a form (create_entry, abort, or an unexpected type) or the step cap
// is hit. Subsumes the former single-shot submission plus the
// generic_thermostat-specific submitOptionsFlowPresetsStep special case:
// a presets step is just another iteration where no user field matches and
// the payload is the round-tripped current values.
func (c *HybridClient) runOptionsFlowSteps(ctx context.Context, initFlowID string, first *OptionsFlowResult, userConfig map[string]any, consumed map[string]bool) (*OptionsFlowResult, error) {
	const maxUpdateSteps = 8
	result := first
	for i := 0; i < maxUpdateSteps && result.Type == flowTypeForm; i++ {
		submission := buildStepSubmission(flowModeUpdate, indexStepSchema(result.DataSchema), userConfig, consumed, result.StepID)

		submitResult, err := c.rest.SubmitConfigEntryOptionsFlowStep(ctx, result.FlowID, submission)
		if err != nil {
			_ = c.rest.AbortConfigEntryOptionsFlow(ctx, initFlowID)
			return nil, fmt.Errorf("submit options flow: %w", err)
		}
		result = submitResult

		if result.Type == flowTypeCreateEntry {
			break
		}
		// Surface HA's validation reason instead of the opaque "unexpected
		// result type" the caller would otherwise report.
		if result.Type == flowTypeForm && len(result.Errors) > 0 {
			_ = c.rest.AbortConfigEntryOptionsFlow(ctx, initFlowID)
			return nil, fmt.Errorf("options flow validation errors: %v", result.Errors)
		}
	}
	return result, nil
}

// submitOptionsFlowPresetsStep completes generic_thermostat's Options Flow
// when it advances to a trailing "presets" step. Split out of
// updateHelperViaOptionsFlow to keep that function's length down (same
// reason applyNameIconViaRegistry/normalizeOptionsFlowDurations were
// already split out).
//
// generic_thermostat's OPTIONS_FLOW has the same "init" -> "presets" shape
// as its CONFIG_FLOW (see buildGenericThermostatStepConfig) - the flow
// always advances to "presets" after "init", whose schema is all-Optional
// and rejects any of the core fields. An empty submission completes it.
// Gated on the step id HA itself reports, not on config.Platform: on this
// update path Platform is the entity *domain* ("climate"), not the helper
// type ("generic_thermostat") - see CLAUDE.md's ParseHelperEntityID
// gotcha - so it can't be used to recognize this platform here.
//
// result is returned unchanged for every other step/type so the caller's
// existing create_entry check still applies.
// normalizeOptionsFlowDurations converts each userConfig value that is
// duration-shaped into Home Assistant's {"hours":.,"minutes":.,"seconds":.}
// dict form in place. Split out of updateHelperViaOptionsFlow to keep that
// function's cognitive complexity down.
//
// Home Assistant renders a DurationSelector field's current value as a
// dict when the field already has a value - that's the primary way a
// duration field is detected here, generically, without a hardcoded list
// of field names - and rejects anything else on submission ("expected
// dict"). A duration field with no current value (e.g.
// template_binary_sensor's delay_on/delay_off, unset by default) never
// appears in currentValues at all, so it also falls back to
// isDurationField(key) - the same name list transformFieldValue uses on
// create - to catch a first-time override the dict-shape heuristic alone
// would miss. This is what buildFilterStepConfig already does for filter's
// window_size on create; the options-flow update path had no equivalent
// before.
//
// window_size is deliberately excluded from isDurationField (it's a
// duration only for two of filter's seven subtypes, and that list is keyed
// on field name alone), so the fallback above can't catch a first-time
// window_size override either. stepID - the filter's actual subtype,
// immutable after creation (CLAUDE.md's manage_helper update field docs) -
// is what buildFilterStepConfig already keys off on create via
// filterDurationWindowSteps; reused here as the update-path equivalent of
// the isDurationField fallback.
// applyNameIconViaRegistry sets name/icon via the Entity Registry after a
// successful Options Flow submission - neither is part of any Options Flow
// schema. Split out of updateHelperViaOptionsFlow to keep that function's
// cognitive complexity down.
func (c *HybridClient) applyNameIconViaRegistry(ctx context.Context, entityID, icon string, hasIcon bool, name string, hasName bool) error {
	if (!hasIcon || icon == "") && (!hasName || name == "") {
		return nil
	}
	WaitForEntityAppear(ctx, c.ws.GetState, entityID, DefaultEntityPollerConfig())
	updateCfg := EntityRegistryUpdateConfig{}
	if hasIcon && icon != "" {
		updateCfg.Icon = &icon
	}
	if hasName && name != "" {
		updateCfg.Name = &name
	}
	if _, err := c.ws.UpdateEntityRegistryEntry(ctx, entityID, updateCfg); err != nil {
		return fmt.Errorf("helper updated, but failed to set name/icon: %w", err)
	}
	return nil
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
	flowResult, err := c.rest.InitConfigEntryFlow(ctx, config.Platform)
	if err != nil {
		return fmt.Errorf("init config entry flow: %w", err)
	}

	consumed := seedConsumedRoutingKeys(config)
	userConfig := config.Config
	c.applyClientLevelCreateDefaults(config, userConfig)

	const maxSteps = 8 // Safety limit; also bounds a single menu hop handled inside the loop.
	for i := 0; i < maxSteps; i++ {
		if flowResult.Type == flowTypeMenu {
			flowResult, err = c.submitConfigFlowMenuChoice(ctx, config, flowResult)
			if err != nil {
				return err
			}
			continue
		}

		if flowResult.Type != flowTypeForm {
			break
		}

		currentStepID := flowResult.StepID
		currentFlowID := flowResult.FlowID
		stepConfig := buildStepSubmission(flowModeCreate, indexStepSchema(flowResult.DataSchema), userConfig, consumed, currentStepID)

		flowResult, err = c.rest.SubmitConfigEntryFlowStep(ctx, currentFlowID, stepConfig)
		if err != nil {
			_ = c.rest.AbortConfigEntryFlow(ctx, currentFlowID)
			return fmt.Errorf("submit config entry flow step %s: %w", currentStepID, err)
		}

		if flowResult.Type == flowTypeCreateEntry {
			break
		}
		if flowResult.Type == "abort" {
			return fmt.Errorf("config entry flow aborted: %s", flowResult.Description)
		}
		if flowResult.Type == flowTypeForm && len(flowResult.Errors) > 0 {
			_ = c.rest.AbortConfigEntryFlow(ctx, currentFlowID)
			return fmt.Errorf("config entry flow validation errors: %v", flowResult.Errors)
		}
	}

	if flowResult.Type == flowTypeCreateEntry {
		// "name" is unconditionally injected by buildHelperConfig for every
		// config-entry helper type, regardless of whether that platform's
		// flow schema actually wants it (switch_as_x's has no "name" field
		// at all - it derives the wrapped entity's name from the source
		// entity). A platform not claiming it is not a caller error, unlike
		// a genuinely-unrecognized field the caller explicitly supplied.
		consumed["name"] = true
		if unconsumed := unconsumedUserFields(userConfig, consumed); len(unconsumed) > 0 {
			return fmt.Errorf("helper created, but field(s) %s were not accepted by any step of the %s config flow", strings.Join(unconsumed, ", "), config.Platform)
		}
		return nil
	}

	// Still in form state after max steps
	if flowResult.Type == flowTypeForm {
		_ = c.rest.AbortConfigEntryFlow(ctx, flowResult.FlowID)
		return fmt.Errorf("config entry flow exceeded max steps (last step_id: %s)", flowResult.StepID)
	}

	// An unrecognized terminal type (neither form, create_entry, nor abort)
	// still leaves the flow open on HA's side - abort it rather than leaking
	// it, same as every other exit path above.
	_ = c.rest.AbortConfigEntryFlow(ctx, flowResult.FlowID)
	return fmt.Errorf("unexpected config entry flow result type: %s", flowResult.Type)
}

// applyClientLevelCreateDefaults fills in defaults HA's own schema doesn't
// provide but callers of this client have always been able to rely on -
// statistics' "state_characteristic" step is vol.Required with no HA-side
// default, and a sensor-domain group's aggregation "type" defaults to "sum"
// (addSensorGroupDefaults). Both used to be applied unconditionally by the
// deleted transformConfigForFlow on every create. Kept as small, explicit
// exceptions rather than reintroduced into the generic engine: they are
// genuine convenience defaults for a caller who supplied nothing, not
// step-shape knowledge.
func (c *HybridClient) applyClientLevelCreateDefaults(config HelperConfig, userConfig map[string]any) {
	if config.Platform == platformStatistics {
		if _, ok := userConfig["state_characteristic"]; !ok {
			userConfig["state_characteristic"] = "mean"
		}
	}
	c.addSensorGroupDefaults(config, userConfig)
}

// submitConfigFlowMenuChoice submits the subtype selection for a config
// entry flow menu step. Handled inside createHelperViaConfigFlow's loop
// (rather than once before it) so a flow with more than one menu hop is
// not structurally impossible - none of today's platforms need it, but
// nothing prevents a future one from doing so.
func (c *HybridClient) submitConfigFlowMenuChoice(ctx context.Context, config HelperConfig, flowResult *ConfigEntryFlowResult) (*ConfigEntryFlowResult, error) {
	subtype := c.determineHelperSubtype(config)
	if subtype == "" {
		_ = c.rest.AbortConfigEntryFlow(ctx, flowResult.FlowID)
		return nil, fmt.Errorf("config entry flow requires subtype selection but none provided")
	}
	// Captured before the call: on error the new flowResult may be nil, so
	// it can't be relied on to abort the flow it was just about to replace.
	menuFlowID := flowResult.FlowID
	next, err := c.rest.SubmitConfigEntryFlowStep(ctx, menuFlowID, map[string]any{
		"next_step_id": subtype,
	})
	if err != nil {
		_ = c.rest.AbortConfigEntryFlow(ctx, menuFlowID)
		return nil, fmt.Errorf("submit config entry flow menu step: %w", err)
	}
	return next, nil
}

// buildConfigForFlowStep builds configuration data for a specific flow step.
// Different platforms have different step IDs that require specific subsets of config.
// buildGenericThermostatStepConfig builds config for generic_thermostat's
// flow steps. Split out of buildConfigForFlowStep to keep that function's
// length down (same reason platformFilter delegates to
// buildFilterStepConfig).
//
// generic_thermostat's CONFIG_FLOW has a trailing "presets" step whose
// PRESETS_SCHEMA is all-Optional preset-temperature fields (away/eco/...)
// we don't currently expose as tool parameters. Its schema is
// PREVENT_EXTRA, so resubmitting the full config there fails with "extra
// keys not allowed @ data['ac_mode']" etc. (see CLAUDE.md, issue #194). An
// empty submission is the only valid payload for this step.
// filterStepFields is the exact key set Home Assistant's filter config-entry
// flow accepts at each step. Step "user" is DATA_SCHEMA_SETUP; every other
// step id IS the filter type name itself (HA's get_next_step returns
// user_input[CONF_FILTER_NAME]), and its schema is the per-type schema
// extended with entity_id/filter/precision. Without this allow-list, a
// stray key surviving from the "user" step (like the removed "filters"
// parameter) reaches HA's PREVENT_EXTRA voluptuous schema and produces
// "extra keys not allowed" instead of a working create.
// filterDurationWindowSteps are the filter steps whose window_size is a
// DurationSelector (HA's cv.positive_time_period_dict) rather than a plain
// sample-count NumberSelector. This is the create-path, filter-specific
// counterpart to isDurationField's generic by-field-name list - see that
// function's doc comment for why window_size can't just be added there.
var filterDurationWindowSteps = map[string]bool{
	"time_simple_moving_average": true,
	"time_throttle":              true,
}

// buildFilterStepConfig shapes a filter helper's config for one step of
// Home Assistant's config-entry flow. Keyed on stepID - HA's own answer to
// "which filter is this" - rather than on config.Config["filter"], so an
// unknown future filter type degrades to forwarding the full transformed
// config (this function's pre-allow-list behavior) instead of silently
// emptying the payload. Restricted to that step's schema fields
// (filterStepFields), with window_size converted to a duration dict for
// the two subtypes that need it (filterDurationWindowSteps).
//
// If toDurationDict fails for a duration-shaped window_size, this
// deliberately falls through to submitting the raw value rather than
// failing the create call locally: consistent with this file's general
// convention of letting Home Assistant's own config-flow validation produce
// the error (e.g. its "expected dict" message) instead of duplicating that
// validation here. The tradeoff is that a malformed window_size surfaces as
// HA's opaque error rather than argReader's own clearer one - argReader.raw
// only bounds window_size's size and top-level shape at read time, it
// doesn't know here which filter subtype makes it duration-shaped.
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
// platformSkipFields maps platforms to fields that are routing-only markers
// - never a real submission value for any step of that platform's flow, so
// they must never reach seedConsumedRoutingKeys' companion buildStepSubmission
// loop as an unmatched (and therefore falsely "unsupported") field. Do NOT
// add a key here just because the OLD per-platform step builders used to
// strip it to keep it out of the WRONG step - under the schema-driven
// engine (flow_steps.go), a step's own DataSchema already limits it to the
// step(s) that actually declare it. Two entries used to conflate these two
// concerns and broke real creates as a result: target_domain IS switch_as_x's
// real (and only) "user" step field (HA has no menu for switch_as_x at all),
// and state_characteristic IS statistics' dedicated step's real field -
// pre-consuming both meant the one step that legitimately wants them could
// never claim them, and HA rejected the submission with "required key not
// provided".
var platformSkipFields = map[string]map[string]bool{
	platformTemplate:          {"type": true, "template_type": true},
	platformRandom:            {"type": true},
	platformGenericThermostat: {"heater_entity_id": true, "target_sensor_entity_id": true},
	platformGenericHygrostat:  {"humidifier_entity_id": true, "target_sensor_entity_id": true},
}

// transformFieldValue transforms a config field value if needed.
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
// Used by transformFieldValue (create) and, as a no-current-value fallback,
// by updateHelperViaOptionsFlow's duration-normalisation loop (update) - see
// that loop's doc comment for how the two paths fit together.
//
// filter's "window_size" is deliberately NOT in this list even though it's
// a duration for two of the seven filter types (time_simple_moving_average,
// time_throttle): this list is keyed on field name alone and is used
// generically across every helper type, but window_size is a plain
// sample-count NumberSelector for the other five filter types
// (outlier/lowpass/range/throttle) - adding it here would make
// transformFieldValue convert a sample count into a bogus duration dict for
// those. window_size's duration-ness is per-filter-type, not per-field-name,
// so it's handled separately and explicitly by filterDurationWindowSteps
// (create, scoped to buildFilterStepConfig) instead.
func isDurationField(fieldName string) bool {
	durationFields := map[string]bool{
		"time_window":      true,
		"delay_on":         true,
		"delay_off":        true,
		"max_sub_interval": true,
	}
	return durationFields[fieldName]
}

// parseDurationString parses "H:MM:SS", "H:MM", or "SS" into Home
// Assistant's DurationSelector dict form. A 2-part string is HH:MM, matching
// Home Assistant's own cv.time_period_str - NOT MM:SS. Returns nil for
// anything else.
func parseDurationString(s string) map[string]int {
	parts := strings.Split(s, ":")
	var hours, minutes, seconds int
	var err error
	switch len(parts) {
	case 3:
		hours, err = strconv.Atoi(parts[0])
		if err == nil {
			minutes, err = strconv.Atoi(parts[1])
		}
		if err == nil {
			seconds, err = strconv.Atoi(parts[2])
		}
	case 2:
		// Matches Home Assistant's own cv.time_period_str, which parses a
		// 2-part string as HH:MM (hour = parts[0], minute = parts[1]), not
		// MM:SS - "1:30" is 1h30m, not 90 seconds.
		hours, err = strconv.Atoi(parts[0])
		if err == nil {
			minutes, err = strconv.Atoi(parts[1])
		}
	case 1:
		seconds, err = strconv.Atoi(parts[0])
	default:
		return nil
	}
	if err != nil {
		return nil
	}
	// Mirrors durationDictFromMap/secondsToDurationDict's guard: a negative
	// or out-of-range component is a syntactically valid but nonsensical
	// duration that must be rejected here, not forwarded to Home Assistant.
	if hours < 0 || hours > maxDurationComponent ||
		minutes < 0 || minutes > maxDurationComponent ||
		seconds < 0 || seconds > maxDurationComponent {
		return nil
	}
	return map[string]int{
		"hours":   hours,
		"minutes": minutes,
		"seconds": seconds,
	}
}

// toDurationDict normalises a value into Home Assistant's DurationSelector
// dict form. Accepted input, in priority order: a map already in that shape
// (durationDictFromMap/durationDictFromIntMap - any key outside
// days/hours/minutes/seconds/milliseconds, or a recognized key with a
// non-numeric value, is ok=false, not silently dropped, since that would
// produce a syntactically valid but wrong duration HA has no way to
// detect); a "H:MM:SS"/"H:MM"/"SS" string; or a bare number interpreted as
// seconds - the form most naturally produced by a caller who doesn't know
// HA expects a dict. Returns ok=false for anything else, so the caller can
// forward the raw value and let Home Assistant produce its own validation
// error rather than this fabricating a dict from unrecognized input.
func toDurationDict(v any) (map[string]int, bool) {
	switch val := v.(type) {
	case map[string]any:
		return durationDictFromMap(val)
	case map[string]int:
		return durationDictFromIntMap(val)
	case string:
		if d := parseDurationString(val); d != nil {
			return d, true
		}
		return nil, false
	case float64:
		return secondsToDurationDict(val)
	case int:
		return secondsToDurationDict(float64(val))
	default:
		return nil, false
	}
}

// durationKeys are every key Home Assistant's cv.time_period_dict accepts.
var durationKeys = []string{"days", "hours", "minutes", "seconds", "milliseconds"}

// maxDurationComponent bounds every individual component (hours, minutes,
// a bare seconds count, ...) accepted by parseDurationString,
// durationDictFromMap, durationDictFromIntMap, and secondsToDurationDict.
// One named constant instead of a bare math.MaxInt32 repeated at each call
// site, so the four duration-conversion guards can't silently drift out of
// sync with each other. int32 range, not int64, because converting a wider
// value to int is implementation-defined in Go for out-of-range floats
// (int(1e20) silently becomes garbage, not a clamp or a panic) and no real
// helper field needs a wider range than that.
const maxDurationComponent = math.MaxInt32

// durationDictFromMap converts a map[string]any into Home Assistant's
// DurationSelector dict form. Every key in val must be one of durationKeys -
// an unrecognized key (e.g. a typo, or a shape that isn't actually a
// duration) fails loudly rather than being silently dropped, which would
// otherwise turn {"days": 1} into an empty (zero-second) duration with no
// indication anything was lost. A recognized key with a non-numeric value
// fails the same way, for the same reason.
func durationDictFromMap(val map[string]any) (map[string]int, bool) {
	allowed := make(map[string]bool, len(durationKeys))
	for _, key := range durationKeys {
		allowed[key] = true
	}
	for key := range val {
		if !allowed[key] {
			return nil, false
		}
	}

	out := make(map[string]int, len(durationKeys))
	for _, key := range durationKeys {
		raw, exists := val[key]
		if !exists {
			continue
		}
		switch n := raw.(type) {
		case float64:
			// Mirrors secondsToDurationDict's guard: converting an
			// out-of-range/NaN/Inf float to int is implementation-defined
			// in Go (e.g. int(1e300) silently becomes garbage, not a clamp
			// or a panic) - reject rather than forward a syntactically
			// valid but nonsensical duration component to Home Assistant.
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > maxDurationComponent {
				return nil, false
			}
			out[key] = int(n)
		case int:
			if n < 0 || n > maxDurationComponent {
				return nil, false
			}
			out[key] = n
		default:
			// A recognized key with a value of the wrong type (e.g. a
			// numeric string) must fail loudly rather than being
			// silently omitted - dropping it here would produce a
			// syntactically valid but wrong duration (e.g.
			// {"hours":"1","minutes":30} silently becoming a 30-second
			// duration instead of 1h30m) that HA has no way to detect
			// as wrong, since the resulting dict is well-formed.
			return nil, false
		}
	}
	return out, true
}

// durationDictFromIntMap validates a map[string]int the same way
// durationDictFromMap validates a map[string]any, so both accepted map
// shapes reject the same unrecognized keys instead of one silently
// forwarding them.
func durationDictFromIntMap(val map[string]int) (map[string]int, bool) {
	allowed := make(map[string]bool, len(durationKeys))
	for _, key := range durationKeys {
		allowed[key] = true
	}
	for key := range val {
		if !allowed[key] {
			return nil, false
		}
	}
	return val, true
}

// secondsToDurationDict converts a seconds count into Home Assistant's
// DurationSelector dict form. Returns ok=false for anything that isn't a
// sane, non-negative, in-range duration (NaN/Inf, negative, or too large to
// convert to int without silently overflowing - int(1e20) wraps to a
// garbage negative value rather than erroring) so the caller forwards the
// raw value untouched and lets Home Assistant produce its own validation
// error, per toDurationDict's doc comment, instead of this function
// fabricating a nonsensical duration.
func secondsToDurationDict(totalSeconds float64) (map[string]int, bool) {
	if math.IsNaN(totalSeconds) || math.IsInf(totalSeconds, 0) || totalSeconds < 0 || totalSeconds > maxDurationComponent {
		return nil, false
	}
	total := int(totalSeconds)
	return map[string]int{
		"hours":   total / 3600,
		"minutes": (total % 3600) / 60,
		"seconds": total % 60,
	}, true
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

// ConfigFileEntryExists reports whether configID exists in the config file Home Assistant's
// config API manages for domain, via the REST API.
func (c *HybridClient) ConfigFileEntryExists(ctx context.Context, domain, configID string) (bool, error) {
	return c.rest.ConfigFileEntryExists(ctx, domain, configID)
}

// =============================================================================
// Registry Operations (delegated to WebSocket)
// =============================================================================

// GetEntityRegistry retrieves the entity registry.
func (c *HybridClient) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	return c.ws.GetEntityRegistry(ctx)
}

// GetEntityRegistryEntry retrieves a single entity registry entry by entity_id.
func (c *HybridClient) GetEntityRegistryEntry(ctx context.Context, entityID string) (*EntityRegistryEntry, error) {
	return c.ws.GetEntityRegistryEntry(ctx, entityID)
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

// GetHelperConfig retrieves the full stored configuration of a WebSocket helper entity.
func (c *HybridClient) GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error) {
	return c.ws.GetHelperConfig(ctx, platform, entityID)
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
	if result.Type == flowTypeMenu {
		result, err = c.navigateOptionsFlowMenu(ctx, entryID, result)
		if err != nil {
			return nil, err
		}
	}

	return c.readAllOptionsFlowSteps(ctx, result), nil
}

// readAllOptionsFlowSteps walks forward through every remaining form step
// of an options flow, merging each step's current values into a single
// flat result (section values nested under their section key, the same
// shape a single step's own submission would have). It never sends real
// user input - buildStepSubmission in update mode round-trips each step's
// own suggested_value fields, so submitting the result back to HA (done
// only to discover what step comes next) is a no-op.
//
// When a step's own response reports last_step:true, its values are read
// directly from that response without submitting it - submitting the
// flow's actual last step would commit it (create_entry), which a pure
// read should avoid. Without that signal (older HA, or a step that omits
// it), the walk falls back to submitting anyway to find out whether
// there's a successor; the resulting no-op commit is harmless since only
// round-tripped values are ever sent.
func (c *HybridClient) readAllOptionsFlowSteps(ctx context.Context, result *OptionsFlowResult) map[string]any {
	merged := make(map[string]any)
	const maxReadSteps = 8
	for i := 0; i < maxReadSteps && result.Type == flowTypeForm; i++ {
		stepValues := buildStepSubmission(flowModeUpdate, indexStepSchema(result.DataSchema), map[string]any{}, map[string]bool{}, result.StepID)
		for k, v := range stepValues {
			merged[k] = v
		}
		if result.LastStep != nil && *result.LastStep {
			break
		}
		next, err := c.rest.SubmitConfigEntryOptionsFlowStep(ctx, result.FlowID, stepValues)
		if err != nil {
			break
		}
		result = next
	}
	return merged
}

// DeleteConfigEntry deletes a config entry and all its associated devices/entities.
// REST-only: Home Assistant has no reliable config_entries/{delete,remove} WS command,
// same class of gap as automation/script/scene CRUD (see CLAUDE.md).
func (c *HybridClient) DeleteConfigEntry(ctx context.Context, entryID string) (bool, error) {
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

	// Exact match first: entity domain "sensor" is a substring of the menu
	// option "binary_sensor", so a substring-only search can pick the
	// wrong option whenever both are present (HA sorts its menus, so
	// "binary_sensor" often precedes "sensor").
	for _, option := range menuOptions {
		if option == entityDomain {
			return option
		}
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

// Package homeassistant provides a hybrid client combining WebSocket and REST APIs.
package homeassistant

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Constants for entity domains used in Config Entry Flow logic.
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
	ListAutomations(ctx context.Context) ([]Automation, error)
	GetAutomation(ctx context.Context, automationID string) (*Automation, error)
	CreateAutomation(ctx context.Context, config AutomationConfig) error
	UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error
	ToggleAutomation(ctx context.Context, entityID string, enabled bool) error
	ListHelpers(ctx context.Context) ([]Entity, error)
	CreateHelper(ctx context.Context, config HelperConfig) error
	UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error
	DeleteHelper(ctx context.Context, helperID string) error
	SetHelperValue(ctx context.Context, entityID string, value any) error
	ListScripts(ctx context.Context) ([]Entity, error)
	GetScript(ctx context.Context, scriptID string) (*Script, error)
	CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error
	UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error
	ListScenes(ctx context.Context) ([]Entity, error)
	CreateScene(ctx context.Context, sceneID string, config SceneConfig) error
	UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error
	GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error)
	GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error)
	GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error)
	SignPath(ctx context.Context, path string, expires int) (string, error)
	GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error)
	BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error)
	GetLovelaceConfig(ctx context.Context) (map[string]any, error)
	GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error)
	GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error)
	GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error)
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

	// Scene operations (REST-only for create/update/delete)
	CreateScene(ctx context.Context, sceneID string, config SceneConfig) error
	UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error
	DeleteScene(ctx context.Context, sceneID string) error

	// Config Entry Flow operations (for helpers requiring HTTP-based flow)
	InitConfigEntryFlow(ctx context.Context, handler string) (*ConfigEntryFlowResult, error)
	SubmitConfigEntryFlowStep(ctx context.Context, flowID string, data map[string]any) (*ConfigEntryFlowResult, error)
	DeleteConfigEntry(ctx context.Context, entryID string) error

	// Service discovery
	GetServices(ctx context.Context) ([]Service, error)

	// System configuration
	GetConfig(ctx context.Context) (*Config, error)

	// Template rendering
	RenderTemplate(ctx context.Context, template string) (string, error)

	// Logbook
	GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error)

	// Configuration validation
	CheckConfig(ctx context.Context) (*ConfigCheckResult, error)
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
// Note: The REST API stores the config but the entity may not appear until
// Home Assistant is restarted or automation.reload is called. The WebSocket
// config/automation/create command is not available in all HA versions.
func (c *HybridClient) CreateAutomation(ctx context.Context, config AutomationConfig) error {
	return c.rest.CreateAutomation(ctx, config)
}

// UpdateAutomation updates an existing automation using the REST API.
// Note: The REST API stores the config but changes may not appear until
// Home Assistant is restarted or automation.reload is called.
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
// For Config Entry platforms (threshold, derivative, integration, group, template),
// this uses the HTTP Config Entry Flow mechanism.
// For standard helpers (input_*, counter, timer, schedule), this uses WebSocket.
func (c *HybridClient) CreateHelper(ctx context.Context, config HelperConfig) error {
	if RequiresConfigEntryFlow(config.Platform) {
		return c.createHelperViaConfigFlow(ctx, config)
	}
	return c.ws.CreateHelper(ctx, config)
}

// UpdateHelper updates an existing input helper.
func (c *HybridClient) UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error {
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

// SetHelperValue sets the value of an input helper.
func (c *HybridClient) SetHelperValue(ctx context.Context, entityID string, value any) error {
	return c.ws.SetHelperValue(ctx, entityID, value)
}

// createHelperViaConfigFlow creates a helper using the HTTP Config Entry Flow.
// This handles the multi-step flow: init -> (menu selection) -> submit data -> verify creation.
func (c *HybridClient) createHelperViaConfigFlow(ctx context.Context, config HelperConfig) error {
	// Step 1: Initialize the config entry flow
	flowResult, err := c.rest.InitConfigEntryFlow(ctx, config.Platform)
	if err != nil {
		return fmt.Errorf("init config entry flow: %w", err)
	}

	// Step 2: Handle menu step if present (for group and template helpers)
	// These require selecting a subtype first (e.g., "binary_sensor", "sensor")
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

	// Step 3: Submit the helper configuration
	// Transform config to match Config Entry Flow schema
	flowConfig := c.transformConfigForFlow(config)
	flowResult, err = c.rest.SubmitConfigEntryFlowStep(ctx, flowResult.FlowID, flowConfig)
	if err != nil {
		return fmt.Errorf("submit config entry flow step: %w", err)
	}

	// Step 4: Verify the result
	if flowResult.Type == "abort" {
		return fmt.Errorf("config entry flow aborted: %s", flowResult.Description)
	}

	if flowResult.Type == "create_entry" {
		// Success - entity was created
		return nil
	}

	// If we get another form step, there may be validation errors
	if flowResult.Type == "form" {
		if len(flowResult.Errors) > 0 {
			return fmt.Errorf("config entry flow validation errors: %v", flowResult.Errors)
		}
		return fmt.Errorf("config entry flow requires additional steps (step_id: %s)", flowResult.StepID)
	}

	return fmt.Errorf("unexpected config entry flow result type: %s", flowResult.Type)
}

// determineHelperSubtype extracts the subtype for multi-step flows (group, template).
func (c *HybridClient) determineHelperSubtype(config HelperConfig) string {
	// Check for explicit "group_type" field for menu selection
	if gt, ok := config.Config["group_type"].(string); ok {
		return gt
	}

	switch config.Platform {
	case "template":
		return c.determineTemplateSubtype(config)
	case "group":
		return c.determineGroupSubtype(config)
	}
	return ""
}

// determineTemplateSubtype determines whether a template is a sensor or binary_sensor.
func (c *HybridClient) determineTemplateSubtype(config HelperConfig) string {
	if t, ok := config.Config["type"].(string); ok {
		return t
	}
	if dc, ok := config.Config["device_class"].(string); ok && binaryDeviceClasses[dc] {
		return domainBinarySensor
	}
	return domainSensor
}

// determineGroupSubtype infers the group type from member entities.
func (c *HybridClient) determineGroupSubtype(config HelperConfig) string {
	entities, ok := config.Config["entities"].([]string)
	if !ok || len(entities) == 0 {
		return domainSensor
	}
	if domain := extractEntityDomain(entities[0]); domain != "" {
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

// shouldSkipConfigField checks if a config field should be skipped during transformation.
func (c *HybridClient) shouldSkipConfigField(key, platform string) bool {
	if key == "group_type" {
		return true
	}
	return key == "type" && platform == "template"
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
	entities, ok := config.Config["entities"].([]string)
	if !ok || len(entities) == 0 {
		return
	}
	domain := extractEntityDomain(entities[0])
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
// Configuration Operations (delegated to WebSocket)
// =============================================================================

// GetLovelaceConfig retrieves the Lovelace dashboard configuration.
func (c *HybridClient) GetLovelaceConfig(ctx context.Context) (map[string]any, error) {
	return c.ws.GetLovelaceConfig(ctx)
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

// RenderTemplate renders a Jinja2 template using Home Assistant state.
func (c *HybridClient) RenderTemplate(ctx context.Context, template string) (string, error) {
	return c.rest.RenderTemplate(ctx, template)
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

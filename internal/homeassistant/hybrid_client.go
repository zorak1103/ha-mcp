// Package homeassistant provides a hybrid client combining WebSocket and REST APIs.
package homeassistant

import (
	"context"
	"time"
)

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
	DeleteAutomation(ctx context.Context, automationID string) error
	DeleteScript(ctx context.Context, scriptID string) error
	DeleteScene(ctx context.Context, sceneID string) error
	GetServices(ctx context.Context) ([]Service, error)
	GetConfig(ctx context.Context) (*Config, error)
	RenderTemplate(ctx context.Context, template string) (string, error)
	GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error)
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

// CreateAutomation creates a new automation.
func (c *HybridClient) CreateAutomation(ctx context.Context, config AutomationConfig) error {
	return c.ws.CreateAutomation(ctx, config)
}

// UpdateAutomation updates an existing automation.
func (c *HybridClient) UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error {
	return c.ws.UpdateAutomation(ctx, automationID, config)
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
// Helper Operations (delegated to WebSocket)
// =============================================================================

// ListHelpers lists all input helpers.
func (c *HybridClient) ListHelpers(ctx context.Context) ([]Entity, error) {
	return c.ws.ListHelpers(ctx)
}

// CreateHelper creates a new input helper.
func (c *HybridClient) CreateHelper(ctx context.Context, config HelperConfig) error {
	return c.ws.CreateHelper(ctx, config)
}

// UpdateHelper updates an existing input helper.
func (c *HybridClient) UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error {
	return c.ws.UpdateHelper(ctx, helperID, config)
}

// DeleteHelper deletes an input helper.
func (c *HybridClient) DeleteHelper(ctx context.Context, helperID string) error {
	return c.ws.DeleteHelper(ctx, helperID)
}

// SetHelperValue sets the value of an input helper.
func (c *HybridClient) SetHelperValue(ctx context.Context, entityID string, value any) error {
	return c.ws.SetHelperValue(ctx, entityID, value)
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

// CreateScript creates a new script.
func (c *HybridClient) CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	return c.ws.CreateScript(ctx, scriptID, config)
}

// UpdateScript updates an existing script.
func (c *HybridClient) UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	return c.ws.UpdateScript(ctx, scriptID, config)
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

// CreateScene creates a new scene.
func (c *HybridClient) CreateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	return c.ws.CreateScene(ctx, sceneID, config)
}

// UpdateScene updates an existing scene.
func (c *HybridClient) UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	return c.ws.UpdateScene(ctx, sceneID, config)
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

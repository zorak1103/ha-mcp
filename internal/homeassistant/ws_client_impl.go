// Package homeassistant provides the WebSocket-based Client implementation.
// coverage-exempt: 68 WS dispatch methods require a live Home Assistant WebSocket server; extensive tests already exist
package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Entity domain prefixes for filtering.
const (
	automationPrefix = "automation."
	scriptPrefix     = "script."
	scenePrefix      = "scene."
)

// CommandSender is an interface for sending WebSocket commands.
// This interface allows mocking the WebSocket client for testing.
type CommandSender interface {
	SendCommand(ctx context.Context, cmdType string, params map[string]any) (*WSResultMessage, error)
}

// wsClientImpl implements WSOperations using WebSocket commands. It does not implement
// the full Client interface: automation/script/scene create/update/delete and other
// REST-only operations are handled exclusively by HybridClient via the REST client,
// since Home Assistant has no reliable WebSocket commands for them.
// It wraps a CommandSender for low-level WebSocket communication.
type wsClientImpl struct {
	ws CommandSender
}

// newWSClientImplWithSender creates a new WebSocket-based WSOperations implementation
// with a custom CommandSender. This is useful for testing.
func newWSClientImplWithSender(sender CommandSender) *wsClientImpl {
	return &wsClientImpl{ws: sender}
}

// Ensure wsClientImpl implements WSOperations at compile time.
var _ WSOperations = (*wsClientImpl)(nil)

// =============================================================================
// Core State Operations
// =============================================================================

// GetStates retrieves all entity states via WebSocket.
func (c *wsClientImpl) GetStates(ctx context.Context) ([]Entity, error) {
	result, err := c.ws.SendCommand(ctx, "get_states", nil)
	if err != nil {
		return nil, fmt.Errorf("get_states command failed: %w", err)
	}

	var entities []Entity
	if err := json.Unmarshal(result.Result, &entities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal states: %w", err)
	}

	return entities, nil
}

// GetState retrieves the state of a specific entity.
func (c *wsClientImpl) GetState(ctx context.Context, entityID string) (*Entity, error) {
	entities, err := c.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	for i := range entities {
		if entities[i].EntityID == entityID {
			return &entities[i], nil
		}
	}

	return nil, fmt.Errorf("entity not found: %s", entityID)
}

// SetState sets the state of an entity (uses call_service internally).
func (c *wsClientImpl) SetState(_ context.Context, _ string, _ StateUpdate) (*Entity, error) {
	// WebSocket API doesn't have a direct set_state equivalent
	// We use REST API behavior simulation via call_service for some domains
	// For now, return error as this is primarily a REST API feature
	return nil, fmt.Errorf("SetState not supported via WebSocket API, use CallService instead")
}

// GetHistory retrieves historical state changes for an entity.
func (c *wsClientImpl) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error) {
	params := map[string]any{
		"start_time": start.Format(time.RFC3339),
		"entity_ids": []string{entityID},
	}
	if !end.IsZero() {
		params["end_time"] = end.Format(time.RFC3339)
	}

	result, err := c.ws.SendCommand(ctx, "history/history_during_period", params)
	if err != nil {
		return nil, fmt.Errorf("history command failed: %w", err)
	}

	// History returns map[entity_id][]entry
	var historyMap map[string][]HistoryEntry
	if err := json.Unmarshal(result.Result, &historyMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history: %w", err)
	}

	// Convert to [][]HistoryEntry format (one array per entity)
	var history [][]HistoryEntry
	if entries, ok := historyMap[entityID]; ok {
		history = append(history, entries)
	}

	return history, nil
}

// CallService calls a Home Assistant service and returns affected entities.
func (c *wsClientImpl) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	params := map[string]any{
		"domain":  domain,
		"service": service,
	}
	if data != nil {
		params["service_data"] = data
	}

	result, err := c.ws.SendCommand(ctx, "call_service", params)
	if err != nil {
		return nil, fmt.Errorf("call_service failed: %w", err)
	}

	// call_service returns context and optionally changed entities
	var response struct {
		Context  Context  `json:"context"`
		Response []Entity `json:"response,omitempty"`
	}
	if result.Result != nil {
		if err := json.Unmarshal(result.Result, &response); err != nil {
			// Some service calls (e.g., script.turn_on, automation.trigger) return only
			// a context without entities. Unmarshal fails because the response structure
			// differs. This is expected behavior, not an error.
			return []Entity{}, nil //nolint:nilerr // service calls return context-only response without entities
		}
	}

	return response.Response, nil
}

// CallServiceWithResponse calls a Home Assistant service with return_response and returns the response data.
func (c *wsClientImpl) CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	params := map[string]any{
		"domain":          domain,
		"service":         service,
		"return_response": true,
	}
	if data != nil {
		params["service_data"] = data
	}

	result, err := c.ws.SendCommand(ctx, "call_service", params)
	if err != nil {
		return nil, fmt.Errorf("call_service with response failed: %w", err)
	}

	// Response structure: {"context": {...}, "response": {...}}
	var response struct {
		Context  Context        `json:"context"`
		Response map[string]any `json:"response,omitempty"`
	}
	if err := json.Unmarshal(result.Result, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service response: %w", err)
	}

	return response.Response, nil
}

// =============================================================================
// Automation Operations
// =============================================================================

// ListAutomations lists all automations.
func (c *wsClientImpl) ListAutomations(ctx context.Context) ([]Automation, error) {
	// Get all states and filter for automation domain
	entities, err := c.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var automations []Automation
	for _, entity := range entities {
		if strings.HasPrefix(entity.EntityID, automationPrefix) {
			automations = append(automations, Automation{
				EntityID:      entity.EntityID,
				State:         entity.State,
				FriendlyName:  getStringAttr(entity.Attributes, "friendly_name"),
				LastTriggered: getStringAttr(entity.Attributes, "last_triggered"),
			})
		}
	}

	return automations, nil
}

// GetAutomation retrieves a specific automation by ID.
// Uses the automation/config WebSocket command with entity_id parameter.
// The response contains the full automation configuration including triggers, conditions, and actions.
func (c *wsClientImpl) GetAutomation(ctx context.Context, automationID string) (*Automation, error) {
	// Build entity_id from automation_id if needed
	entityID := automationID
	if !strings.HasPrefix(automationID, automationPrefix) {
		entityID = automationPrefix + automationID
	}

	result, err := c.ws.SendCommand(ctx, "automation/config", map[string]any{
		"entity_id": entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get automation failed: %w", err)
	}

	// Response is wrapped in {"config": {...}}
	var response struct {
		Config AutomationConfig `json:"config"`
	}
	if err := json.Unmarshal(result.Result, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal automation config: %w", err)
	}

	return &Automation{
		EntityID: entityID,
		Config:   &response.Config,
	}, nil
}

// ToggleAutomation enables or disables an automation.
func (c *wsClientImpl) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	service := serviceTurnOn
	if !enabled {
		service = serviceTurnOff
	}

	_, err := c.CallService(ctx, "automation", service, map[string]any{
		"entity_id": entityID,
	})
	return err
}

// =============================================================================
// Helper Operations
// =============================================================================

// helperPrefixes defines all known input helper domain prefixes.
var helperPrefixes = []string{
	"input_boolean.",
	"input_number.",
	"input_text.",
	"input_select.",
	"input_datetime.",
}

// ListHelpers lists all input helpers.
func (c *wsClientImpl) ListHelpers(ctx context.Context) ([]Entity, error) {
	entities, err := c.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var helpers []Entity
	for _, entity := range entities {
		for _, prefix := range helperPrefixes {
			if strings.HasPrefix(entity.EntityID, prefix) {
				helpers = append(helpers, entity)
				break
			}
		}
	}

	return helpers, nil
}

// CreateHelper creates a new input helper.
// Note: Home Assistant generates the entity ID from the "name" field automatically.
// The HelperConfig.ID field is only used for update/delete operations.
func (c *wsClientImpl) CreateHelper(ctx context.Context, config HelperConfig) error {
	cmdType := fmt.Sprintf("%s/create", config.Platform)
	params := map[string]any{}

	// Add config fields - the entity ID is derived from the "name" field by HA
	if config.Config != nil {
		for k, v := range config.Config {
			params[k] = v
		}
	}

	_, err := c.ws.SendCommand(ctx, cmdType, params)
	if err != nil {
		return fmt.Errorf("create helper failed: %w", err)
	}

	return nil
}

// wsHelperPlatforms lists helper platforms managed entirely over the WebSocket API
// (input_* helpers, counter, timer, schedule). Config Entry helper platforms (sensor,
// binary_sensor, climate, humidifier, select, group, template, threshold, derivative, ...)
// have no "<platform>/update" or "<platform>/delete" WS command - they must go through
// the Options Flow / Config Entry Flow REST API instead (see HybridClient.UpdateHelper
// and HybridClient.DeleteHelper). Sending them here would surface as a confusing
// unknown_command error from Home Assistant.
var wsHelperPlatforms = map[string]bool{
	"input_boolean":  true,
	"input_button":   true,
	"input_number":   true,
	"input_text":     true,
	"input_select":   true,
	"input_datetime": true,
	"counter":        true,
	"timer":          true,
	"schedule":       true,
}

// isWSHelperPlatform reports whether platform has real "<platform>/update|delete" WS commands.
func isWSHelperPlatform(platform string) bool {
	return wsHelperPlatforms[platform]
}

// UpdateHelper updates an existing input helper.
func (c *wsClientImpl) UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error {
	if !isWSHelperPlatform(config.Platform) {
		return fmt.Errorf("cannot update %s helper via websocket: config-entry helpers require options flow (entity may be missing from the registry)", config.Platform)
	}

	// Accept either a full entity_id ("input_number.my_helper") or a bare id
	// ("my_helper") - callers historically used both conventions.
	id := helperID
	if p := extractPlatform(helperID); p != "" {
		id = helperID[len(p)+1:]
	}

	cmdType := fmt.Sprintf("%s/update", config.Platform)
	params := map[string]any{
		config.Platform + "_id": id,
	}

	// Add config fields
	if config.Config != nil {
		for k, v := range config.Config {
			params[k] = v
		}
	}

	_, err := c.ws.SendCommand(ctx, cmdType, params)
	if err != nil {
		return fmt.Errorf("update helper failed: %w", err)
	}

	return nil
}

// DeleteHelper deletes an input helper.
func (c *wsClientImpl) DeleteHelper(ctx context.Context, helperID string) error {
	// Determine platform from entity ID
	platform := extractPlatform(helperID)
	if platform == "" {
		return fmt.Errorf("unable to determine platform for helper %s", helperID)
	}

	if !isWSHelperPlatform(platform) {
		return fmt.Errorf("cannot delete %s helper via websocket: config-entry helpers require the config entry API (entity may be missing from the registry)", platform)
	}

	// Extract ID without prefix
	id := helperID[len(platform)+1:]
	cmdType := fmt.Sprintf("%s/delete", platform)

	_, err := c.ws.SendCommand(ctx, cmdType, map[string]any{
		platform + "_id": id,
	})
	if err != nil {
		return fmt.Errorf("delete helper failed: %w", err)
	}

	return nil
}

// SetHelperValue sets the value of an input helper.
func (c *wsClientImpl) SetHelperValue(ctx context.Context, entityID string, value any) error {
	platform := extractPlatform(entityID)
	if platform == "" {
		return fmt.Errorf("unable to determine platform for helper %s", entityID)
	}

	var service string
	var data map[string]any

	switch platform {
	case "input_boolean":
		boolVal, ok := value.(bool)
		if !ok {
			return fmt.Errorf("input_boolean requires a boolean value")
		}
		if boolVal {
			service = serviceTurnOn
		} else {
			service = serviceTurnOff
		}
		data = map[string]any{"entity_id": entityID}
	case "input_number":
		service = serviceSetValue
		data = map[string]any{"entity_id": entityID, "value": value}
	case "input_text":
		service = serviceSetValue
		data = map[string]any{"entity_id": entityID, "value": value}
	case "input_select":
		service = "select_option"
		data = map[string]any{"entity_id": entityID, "option": value}
	case "input_datetime":
		service = "set_datetime"
		data = map[string]any{"entity_id": entityID}
		switch v := value.(type) {
		case string:
			data["datetime"] = v
		case map[string]any:
			for k, val := range v {
				data[k] = val
			}
		default:
			data["datetime"] = value
		}
	default:
		return fmt.Errorf("unsupported helper platform: %s", platform)
	}

	_, err := c.CallService(ctx, platform, service, data)
	return err
}

// =============================================================================
// Script Operations
// =============================================================================

// ListScripts lists all scripts.
func (c *wsClientImpl) ListScripts(ctx context.Context) ([]Entity, error) {
	entities, err := c.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var scripts []Entity
	for _, entity := range entities {
		if strings.HasPrefix(entity.EntityID, scriptPrefix) {
			scripts = append(scripts, entity)
		}
	}

	return scripts, nil
}

// GetScript retrieves a specific script by ID including its full configuration.
// Uses the script/config WebSocket command with entity_id parameter.
// The response contains the full script configuration including sequence, fields, and mode.
func (c *wsClientImpl) GetScript(ctx context.Context, scriptID string) (*Script, error) {
	// Build entity_id from script_id if needed
	entityID := scriptID
	if !strings.HasPrefix(scriptID, scriptPrefix) {
		entityID = scriptPrefix + scriptID
	}

	// Get the entity state first for metadata
	state, err := c.GetState(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("get script state failed: %w", err)
	}

	script := &Script{
		EntityID:      entityID,
		State:         state.State,
		FriendlyName:  getStringAttr(state.Attributes, "friendly_name"),
		LastTriggered: getStringAttr(state.Attributes, "last_triggered"),
	}

	// Get the full script configuration via WebSocket
	result, err := c.ws.SendCommand(ctx, "script/config", map[string]any{
		"entity_id": entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get script config failed: %w", err)
	}

	// Response is wrapped in {"config": {...}}
	var response struct {
		Config ScriptConfig `json:"config"`
	}
	if err := json.Unmarshal(result.Result, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal script config: %w", err)
	}

	script.Config = &response.Config
	return script, nil
}

// =============================================================================
// Scene Operations
// =============================================================================

// ListScenes lists all scenes.
func (c *wsClientImpl) ListScenes(ctx context.Context) ([]Entity, error) {
	entities, err := c.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var scenes []Entity
	for _, entity := range entities {
		if strings.HasPrefix(entity.EntityID, scenePrefix) {
			scenes = append(scenes, entity)
		}
	}

	return scenes, nil
}

// =============================================================================
// Helper Config Operations (WebSocket-only)
// =============================================================================

// ErrHelperNotFoundInStorage is returned by GetHelperConfig when
// entityID's object_id isn't present in "<platform>/list"'s result. This can
// mean the entity was defined in configuration.yaml (never registered in
// storage) or that it was renamed via the entity registry after creation -
// storage keeps the id assigned at creation time, so a later entity_id
// change desyncs it from the object_id GetHelperConfig looks up by. Callers
// needing to tell the caller which is more likely should use errors.Is
// against this rather than matching on error text.
var ErrHelperNotFoundInStorage = errors.New("not found in storage list")

// GetHelperConfig retrieves the full stored configuration of a WebSocket
// helper entity for platform (e.g. "schedule", "input_number", "timer").
// Every such platform registers "<platform>/list" alongside "<platform>/update"
// via the same collection.StorageCollectionWebsocket setup
// (homeassistant/helpers/collection.py) - it returns the raw stored config
// for every entity of that platform, unlike GetState's entity attributes,
// which are a lossy, sometimes-conditional projection of it (e.g. timer's
// "restore" attribute is only present when true; template config fields
// like input_number's "initial" don't appear in state attributes at all
// for several input_* types).
func (c *wsClientImpl) GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error) {
	if !isWSHelperPlatform(platform) {
		return nil, fmt.Errorf("cannot get %s helper config via websocket: config-entry helpers have no \"<platform>/list\" command (entity may be missing from the registry)", platform)
	}

	// Extract the object_id part without the "<platform>." prefix.
	id := entityID
	if prefix := platform + "."; strings.HasPrefix(entityID, prefix) {
		id = entityID[len(prefix):]
	}

	result, err := c.ws.SendCommand(ctx, platform+"/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get %s list failed: %w", platform, err)
	}

	var items []map[string]any
	if err := json.Unmarshal(result.Result, &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s list: %w", platform, err)
	}

	for _, item := range items {
		if itemID, ok := item["id"].(string); ok && itemID == id {
			return item, nil
		}
	}

	return nil, fmt.Errorf("%s not found: %s: %w", platform, entityID, ErrHelperNotFoundInStorage)
}

// =============================================================================
// Registry Operations (WebSocket-only)
// =============================================================================

// GetEntityRegistry retrieves the entity registry.
func (c *wsClientImpl) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "config/entity_registry/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get entity registry failed: %w", err)
	}

	var entries []EntityRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity registry: %w", err)
	}

	return entries, nil
}

// GetEntityRegistryEntry retrieves a single entity registry entry by entity_id, avoiding a full
// registry fetch. Backed by "config/entity_registry/get" (HA components/config/entity_registry.py
// websocket_get_entity), which returns the entry's extended_dict - a superset of the fields
// "config/entity_registry/list" returns per entry, including unique_id.
func (c *wsClientImpl) GetEntityRegistryEntry(ctx context.Context, entityID string) (*EntityRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "config/entity_registry/get", map[string]any{"entity_id": entityID})
	if err != nil {
		return nil, fmt.Errorf("get entity registry entry failed: %w", err)
	}

	var entry EntityRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity registry entry: %w", err)
	}

	return &entry, nil
}

// GetDeviceRegistry retrieves the device registry.
func (c *wsClientImpl) GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "config/device_registry/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get device registry failed: %w", err)
	}

	var entries []DeviceRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal device registry: %w", err)
	}

	return entries, nil
}

// GetAreaRegistry retrieves the area registry.
func (c *wsClientImpl) GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "config/area_registry/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get area registry failed: %w", err)
	}

	var entries []AreaRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal area registry: %w", err)
	}

	return entries, nil
}

// RemoveEntityRegistryEntry removes an entity from the entity registry.
func (c *wsClientImpl) RemoveEntityRegistryEntry(ctx context.Context, entityID string) error {
	_, err := c.ws.SendCommand(ctx, "config/entity_registry/remove", map[string]any{
		"entity_id": entityID,
	})
	if err != nil {
		return fmt.Errorf("remove entity registry entry failed: %w", err)
	}
	return nil
}

// UpdateEntityRegistryEntry updates an existing entity in the entity registry.
func (c *wsClientImpl) UpdateEntityRegistryEntry(ctx context.Context, entityID string, config EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
	params := map[string]any{
		"entity_id": entityID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.Name != nil {
		params["name"] = *config.Name
	}
	if config.Icon != nil {
		params["icon"] = *config.Icon
	}
	if config.AreaID != nil {
		params["area_id"] = *config.AreaID
	}
	if config.DisabledBy != nil {
		params["disabled_by"] = *config.DisabledBy
	}
	if config.HiddenBy != nil {
		params["hidden_by"] = *config.HiddenBy
	}
	if config.Labels != nil {
		params["labels"] = config.Labels
	}
	if config.Aliases != nil {
		params["aliases"] = config.Aliases
	}
	if config.NewEntityID != nil {
		params["new_entity_id"] = *config.NewEntityID
	}

	result, err := c.ws.SendCommand(ctx, "config/entity_registry/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update entity: %w", err)
	}

	// Home Assistant wraps the response in an "entity_entry" key
	var wrapper struct {
		EntityEntry EntityRegistryEntry `json:"entity_entry"`
	}
	if err := json.Unmarshal(result.Result, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated entity: %w", err)
	}

	return &wrapper.EntityEntry, nil
}

// RemoveDeviceConfigEntry removes a config entry from a device.
// When all config entries are removed, Home Assistant deletes the device.
func (c *wsClientImpl) RemoveDeviceConfigEntry(ctx context.Context, deviceID, configEntryID string) error {
	_, err := c.ws.SendCommand(ctx, "config/device_registry/remove_config_entry", map[string]any{
		"device_id":       deviceID,
		"config_entry_id": configEntryID,
	})
	if err != nil {
		return fmt.Errorf("remove device config entry failed: %w", err)
	}
	return nil
}

// UpdateDeviceRegistryEntry updates an existing device in the device registry.
func (c *wsClientImpl) UpdateDeviceRegistryEntry(ctx context.Context, deviceID string, config DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error) {
	params := map[string]any{
		"device_id": deviceID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.NameByUser != nil {
		params["name_by_user"] = *config.NameByUser
	}
	if config.AreaID != nil {
		params["area_id"] = *config.AreaID
	}
	if config.DisabledBy != nil {
		params["disabled_by"] = *config.DisabledBy
	}
	if config.Labels != nil {
		params["labels"] = config.Labels
	}

	result, err := c.ws.SendCommand(ctx, "config/device_registry/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}

	// Home Assistant wraps the response in a "device_entry" key
	var wrapper struct {
		DeviceEntry DeviceRegistryEntry `json:"device_entry"`
	}
	if err := json.Unmarshal(result.Result, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated device: %w", err)
	}

	return &wrapper.DeviceEntry, nil
}

// CreateArea creates a new area in the area registry.
func (c *wsClientImpl) CreateArea(ctx context.Context, config AreaConfig) (*AreaRegistryEntry, error) {
	params := make(map[string]any)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Picture != "" {
		params["picture"] = config.Picture
	}
	if config.FloorID != "" {
		params["floor_id"] = config.FloorID
	}
	if config.Aliases != nil {
		params["aliases"] = config.Aliases
	}
	if config.Labels != nil {
		params["labels"] = config.Labels
	}

	result, err := c.ws.SendCommand(ctx, "config/area_registry/create", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create area: %w", err)
	}

	var entry AreaRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created area: %w", err)
	}

	return &entry, nil
}

// UpdateArea updates an existing area in the area registry.
func (c *wsClientImpl) UpdateArea(ctx context.Context, areaID string, config AreaConfig) (*AreaRegistryEntry, error) {
	params := map[string]any{
		"area_id": areaID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Picture != "" {
		params["picture"] = config.Picture
	}
	if config.FloorID != "" {
		params["floor_id"] = config.FloorID
	}
	if config.Aliases != nil {
		params["aliases"] = config.Aliases
	}
	if config.Labels != nil {
		params["labels"] = config.Labels
	}

	result, err := c.ws.SendCommand(ctx, "config/area_registry/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update area: %w", err)
	}

	var entry AreaRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated area: %w", err)
	}

	return &entry, nil
}

// DeleteArea deletes an area from the area registry.
func (c *wsClientImpl) DeleteArea(ctx context.Context, areaID string) error {
	_, err := c.ws.SendCommand(ctx, "config/area_registry/delete", map[string]any{
		"area_id": areaID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete area: %w", err)
	}
	return nil
}

// GetLabelRegistry retrieves the label registry.
func (c *wsClientImpl) GetLabelRegistry(ctx context.Context) ([]LabelRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "config/label_registry/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get label registry failed: %w", err)
	}

	var entries []LabelRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal label registry: %w", err)
	}

	return entries, nil
}

// CreateLabel creates a new label in the label registry.
func (c *wsClientImpl) CreateLabel(ctx context.Context, config LabelConfig) (*LabelRegistryEntry, error) {
	params := make(map[string]any)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Color != "" {
		params["color"] = config.Color
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Description != "" {
		params["description"] = config.Description
	}

	result, err := c.ws.SendCommand(ctx, "config/label_registry/create", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create label: %w", err)
	}

	var entry LabelRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created label: %w", err)
	}

	return &entry, nil
}

// UpdateLabel updates an existing label in the label registry.
func (c *wsClientImpl) UpdateLabel(ctx context.Context, labelID string, config LabelConfig) (*LabelRegistryEntry, error) {
	params := map[string]any{
		"label_id": labelID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Color != "" {
		params["color"] = config.Color
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Description != "" {
		params["description"] = config.Description
	}

	result, err := c.ws.SendCommand(ctx, "config/label_registry/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update label: %w", err)
	}

	var entry LabelRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated label: %w", err)
	}

	return &entry, nil
}

// DeleteLabel deletes a label from the label registry.
func (c *wsClientImpl) DeleteLabel(ctx context.Context, labelID string) error {
	_, err := c.ws.SendCommand(ctx, "config/label_registry/delete", map[string]any{
		"label_id": labelID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete label: %w", err)
	}
	return nil
}

// GetFloorRegistry retrieves the floor registry.
func (c *wsClientImpl) GetFloorRegistry(ctx context.Context) ([]FloorRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "config/floor_registry/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get floor registry failed: %w", err)
	}

	var entries []FloorRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal floor registry: %w", err)
	}

	return entries, nil
}

// CreateFloor creates a new floor in the floor registry.
func (c *wsClientImpl) CreateFloor(ctx context.Context, config FloorConfig) (*FloorRegistryEntry, error) {
	params := make(map[string]any)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Level != nil {
		params["level"] = *config.Level
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if len(config.Aliases) > 0 {
		params["aliases"] = config.Aliases
	}

	result, err := c.ws.SendCommand(ctx, "config/floor_registry/create", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create floor: %w", err)
	}

	var entry FloorRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created floor: %w", err)
	}

	return &entry, nil
}

// UpdateFloor updates an existing floor in the floor registry.
func (c *wsClientImpl) UpdateFloor(ctx context.Context, floorID string, config FloorConfig) (*FloorRegistryEntry, error) {
	params := map[string]any{
		"floor_id": floorID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Level != nil {
		params["level"] = *config.Level
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Aliases != nil {
		params["aliases"] = config.Aliases
	}

	result, err := c.ws.SendCommand(ctx, "config/floor_registry/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update floor: %w", err)
	}

	var entry FloorRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated floor: %w", err)
	}

	return &entry, nil
}

// DeleteFloor deletes a floor from the floor registry.
func (c *wsClientImpl) DeleteFloor(ctx context.Context, floorID string) error {
	_, err := c.ws.SendCommand(ctx, "config/floor_registry/delete", map[string]any{
		"floor_id": floorID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete floor: %w", err)
	}
	return nil
}

// GetZones retrieves all zones.
func (c *wsClientImpl) GetZones(ctx context.Context) ([]ZoneRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "zone/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get zones failed: %w", err)
	}

	var entries []ZoneRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal zones: %w", err)
	}

	return entries, nil
}

// CreateZone creates a new zone.
func (c *wsClientImpl) CreateZone(ctx context.Context, config ZoneConfig) (*ZoneRegistryEntry, error) {
	params := make(map[string]any)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Latitude != nil {
		params["latitude"] = *config.Latitude
	}
	if config.Longitude != nil {
		params["longitude"] = *config.Longitude
	}
	if config.Radius != nil {
		params["radius"] = *config.Radius
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Passive != nil {
		params["passive"] = *config.Passive
	}

	result, err := c.ws.SendCommand(ctx, "zone/create", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create zone: %w", err)
	}

	var entry ZoneRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created zone: %w", err)
	}

	return &entry, nil
}

// UpdateZone updates an existing zone.
func (c *wsClientImpl) UpdateZone(ctx context.Context, zoneID string, config ZoneConfig) (*ZoneRegistryEntry, error) {
	params := map[string]any{
		"zone_id": zoneID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Latitude != nil {
		params["latitude"] = *config.Latitude
	}
	if config.Longitude != nil {
		params["longitude"] = *config.Longitude
	}
	if config.Radius != nil {
		params["radius"] = *config.Radius
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Passive != nil {
		params["passive"] = *config.Passive
	}

	result, err := c.ws.SendCommand(ctx, "zone/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update zone: %w", err)
	}

	var entry ZoneRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated zone: %w", err)
	}

	return &entry, nil
}

// DeleteZone deletes a zone.
func (c *wsClientImpl) DeleteZone(ctx context.Context, zoneID string) error {
	_, err := c.ws.SendCommand(ctx, "zone/delete", map[string]any{
		"zone_id": zoneID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete zone: %w", err)
	}
	return nil
}

// GetPersons retrieves all persons.
// GetPersons retrieves all persons.
//
// Unlike zone/list, person/list responds with {"storage": [...], "config":
// [...]} - Home Assistant's person integration separates storage-managed
// persons from YAML-configured ones, using a custom collection handler
// instead of the generic list response the other collection APIs return.
func (c *wsClientImpl) GetPersons(ctx context.Context) ([]PersonRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "person/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get persons failed: %w", err)
	}

	var response struct {
		Storage []PersonRegistryEntry `json:"storage"`
		Config  []PersonRegistryEntry `json:"config"`
	}
	if err := json.Unmarshal(result.Result, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal persons: %w", err)
	}

	entries := make([]PersonRegistryEntry, 0, len(response.Storage)+len(response.Config))
	entries = append(entries, response.Storage...)
	entries = append(entries, response.Config...)

	return entries, nil
}

// CreatePerson creates a new person.
func (c *wsClientImpl) CreatePerson(ctx context.Context, config PersonConfig) (*PersonRegistryEntry, error) {
	params := make(map[string]any)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.UserID != "" {
		params["user_id"] = config.UserID
	}
	if len(config.DeviceTrackers) > 0 {
		params["device_trackers"] = config.DeviceTrackers
	}
	if config.Picture != "" {
		params["picture"] = config.Picture
	}

	result, err := c.ws.SendCommand(ctx, "person/create", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create person: %w", err)
	}

	var entry PersonRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created person: %w", err)
	}

	return &entry, nil
}

// UpdatePerson updates an existing person.
func (c *wsClientImpl) UpdatePerson(ctx context.Context, personID string, config PersonConfig) (*PersonRegistryEntry, error) {
	params := map[string]any{
		"person_id": personID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.UserID != "" {
		params["user_id"] = config.UserID
	}
	if config.DeviceTrackers != nil {
		params["device_trackers"] = config.DeviceTrackers
	}
	if config.Picture != "" {
		params["picture"] = config.Picture
	}

	result, err := c.ws.SendCommand(ctx, "person/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update person: %w", err)
	}

	var entry PersonRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated person: %w", err)
	}

	return &entry, nil
}

// DeletePerson deletes a person.
func (c *wsClientImpl) DeletePerson(ctx context.Context, personID string) error {
	_, err := c.ws.SendCommand(ctx, "person/delete", map[string]any{
		"person_id": personID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete person: %w", err)
	}
	return nil
}

// GetTags retrieves all tags.
func (c *wsClientImpl) GetTags(ctx context.Context) ([]TagRegistryEntry, error) {
	result, err := c.ws.SendCommand(ctx, "tag/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get tags failed: %w", err)
	}

	var entries []TagRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}

	return entries, nil
}

// CreateTag creates a new tag.
func (c *wsClientImpl) CreateTag(ctx context.Context, config TagConfig) (*TagRegistryEntry, error) {
	params := make(map[string]any)
	if config.TagID != "" {
		params["tag_id"] = config.TagID
	}
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Description != "" {
		params["description"] = config.Description
	}

	result, err := c.ws.SendCommand(ctx, "tag/create", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	var entry TagRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created tag: %w", err)
	}

	return &entry, nil
}

// UpdateTag updates an existing tag.
func (c *wsClientImpl) UpdateTag(ctx context.Context, tagID string, config TagConfig) (*TagRegistryEntry, error) {
	params := map[string]any{
		"tag_id": tagID,
	}

	// Only include fields that are provided (to support partial updates)
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Description != "" {
		params["description"] = config.Description
	}

	result, err := c.ws.SendCommand(ctx, "tag/update", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update tag: %w", err)
	}

	var entry TagRegistryEntry
	if err := json.Unmarshal(result.Result, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated tag: %w", err)
	}

	return &entry, nil
}

// DeleteTag deletes a tag.
func (c *wsClientImpl) DeleteTag(ctx context.Context, tagID string) error {
	_, err := c.ws.SendCommand(ctx, "tag/delete", map[string]any{
		"tag_id": tagID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// =============================================================================
// Media Operations (WebSocket-only)
// =============================================================================

// SignPath generates a signed URL for authenticated access.
func (c *wsClientImpl) SignPath(ctx context.Context, path string, expires int) (string, error) {
	params := map[string]any{
		"path": path,
	}
	if expires > 0 {
		params["expires"] = expires
	}

	result, err := c.ws.SendCommand(ctx, "auth/sign_path", params)
	if err != nil {
		return "", fmt.Errorf("sign path failed: %w", err)
	}

	var response struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(result.Result, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal sign path response: %w", err)
	}

	return response.Path, nil
}

// GetCameraStream gets a camera stream URL.
func (c *wsClientImpl) GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error) {
	result, err := c.ws.SendCommand(ctx, "camera/stream", map[string]any{
		"entity_id": entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get camera stream failed: %w", err)
	}

	var info StreamInfo
	if err := json.Unmarshal(result.Result, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream info: %w", err)
	}

	return &info, nil
}

// BrowseMedia browses media content.
func (c *wsClientImpl) BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error) {
	params := map[string]any{}
	if mediaContentID != "" {
		params["media_content_id"] = mediaContentID
	}

	result, err := c.ws.SendCommand(ctx, "media_source/browse_media", params)
	if err != nil {
		return nil, fmt.Errorf("browse media failed: %w", err)
	}

	var browseResult MediaBrowseResult
	if err := json.Unmarshal(result.Result, &browseResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal browse result: %w", err)
	}

	return &browseResult, nil
}

// =============================================================================
// Configuration Operations (WebSocket-only)
// =============================================================================

// GetLovelaceConfig retrieves the Lovelace dashboard configuration.
func (c *wsClientImpl) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	params := make(map[string]any)
	if urlPath != "" {
		params["url_path"] = urlPath
	}

	result, err := c.ws.SendCommand(ctx, "lovelace/config", params)
	if err != nil {
		return nil, fmt.Errorf("get lovelace config failed: %w", err)
	}

	var config map[string]any
	if err := json.Unmarshal(result.Result, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lovelace config: %w", err)
	}

	return config, nil
}

// SaveLovelaceConfig saves configuration for a Lovelace dashboard.
func (c *wsClientImpl) SaveLovelaceConfig(ctx context.Context, urlPath string, config map[string]any) error {
	params := map[string]any{
		"config": config,
	}
	if urlPath != "" {
		params["url_path"] = urlPath
	}

	_, err := c.ws.SendCommand(ctx, "lovelace/config/save", params)
	if err != nil {
		return fmt.Errorf("save lovelace config failed: %w", err)
	}

	return nil
}

// ListDashboards retrieves all Lovelace dashboards.
func (c *wsClientImpl) ListDashboards(ctx context.Context) ([]DashboardEntry, error) {
	result, err := c.ws.SendCommand(ctx, "lovelace/dashboards/list", nil)
	if err != nil {
		return nil, fmt.Errorf("list dashboards failed: %w", err)
	}

	var dashboards []DashboardEntry
	if err := json.Unmarshal(result.Result, &dashboards); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dashboards: %w", err)
	}

	return dashboards, nil
}

// CreateDashboard creates a new Lovelace dashboard.
func (c *wsClientImpl) CreateDashboard(ctx context.Context, config DashboardConfig) (*DashboardEntry, error) {
	params := map[string]any{
		"url_path": config.URLPath,
		"title":    config.Title,
	}
	if config.Mode != "" {
		params["mode"] = config.Mode
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.RequireAdmin != nil {
		params["require_admin"] = *config.RequireAdmin
	}
	if config.ShowInSidebar != nil {
		params["show_in_sidebar"] = *config.ShowInSidebar
	}

	result, err := c.ws.SendCommand(ctx, "lovelace/dashboards/create", params)
	if err != nil {
		return nil, fmt.Errorf("create dashboard failed: %w", err)
	}

	var dashboard DashboardEntry
	if err := json.Unmarshal(result.Result, &dashboard); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dashboard: %w", err)
	}

	return &dashboard, nil
}

// UpdateDashboard updates an existing Lovelace dashboard.
func (c *wsClientImpl) UpdateDashboard(ctx context.Context, dashboardID string, config DashboardConfig) (*DashboardEntry, error) {
	params := map[string]any{
		"dashboard_id": dashboardID,
	}
	if config.Title != "" {
		params["title"] = config.Title
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.RequireAdmin != nil {
		params["require_admin"] = *config.RequireAdmin
	}
	if config.ShowInSidebar != nil {
		params["show_in_sidebar"] = *config.ShowInSidebar
	}

	result, err := c.ws.SendCommand(ctx, "lovelace/dashboards/update", params)
	if err != nil {
		return nil, fmt.Errorf("update dashboard failed: %w", err)
	}

	var dashboard DashboardEntry
	if err := json.Unmarshal(result.Result, &dashboard); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dashboard: %w", err)
	}

	return &dashboard, nil
}

// DeleteDashboard deletes a Lovelace dashboard.
func (c *wsClientImpl) DeleteDashboard(ctx context.Context, dashboardID string) error {
	params := map[string]any{
		"dashboard_id": dashboardID,
	}

	_, err := c.ws.SendCommand(ctx, "lovelace/dashboards/delete", params)
	if err != nil {
		return fmt.Errorf("delete dashboard failed: %w", err)
	}

	return nil
}

// =============================================================================
// Statistics Operations (WebSocket-only)
// =============================================================================

// GetStatistics retrieves long-term statistics for entities.
func (c *wsClientImpl) GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error) {
	// Default to last 24 hours if no start_time provided
	startTime := time.Now().Add(-24 * time.Hour)

	params := map[string]any{
		"statistic_ids": statIDs,
		"period":        period,
		"start_time":    startTime.Format(time.RFC3339),
	}

	result, err := c.ws.SendCommand(ctx, "recorder/statistics_during_period", params)
	if err != nil {
		return nil, fmt.Errorf("get statistics failed: %w", err)
	}

	// Statistics returns map[stat_id][]statistics
	var statsMap map[string][]StatisticsResult
	if err := json.Unmarshal(result.Result, &statsMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal statistics: %w", err)
	}

	var allStats []StatisticsResult
	for statID, stats := range statsMap {
		// Populate StatisticID from map key (HA API returns it only as key, not in entries)
		for i := range stats {
			stats[i].StatisticID = statID
		}
		allStats = append(allStats, stats...)
	}

	return allStats, nil
}

// =============================================================================
// Target Operations (WebSocket-only)
// =============================================================================

// getForTarget is a helper function for get_triggers_for_target, get_conditions_for_target,
// and get_services_for_target commands.
func (c *wsClientImpl) getForTarget(ctx context.Context, cmdType string, target Target, expandGroup *bool) ([]string, error) {
	params := map[string]any{
		"target": target,
	}
	if expandGroup != nil {
		params["expand_group"] = *expandGroup
	}

	result, err := c.ws.SendCommand(ctx, cmdType, params)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", cmdType, err)
	}

	var identifiers []string
	if err := json.Unmarshal(result.Result, &identifiers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s response: %w", cmdType, err)
	}

	return identifiers, nil
}

// GetTriggersForTarget retrieves all applicable triggers for the given target.
// The target can include entity IDs, device IDs, area IDs, and label IDs.
// When expandGroup is true (default), group entities are expanded to their members.
func (c *wsClientImpl) GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.getForTarget(ctx, "get_triggers_for_target", target, expandGroup)
}

// GetConditionsForTarget retrieves all applicable conditions for the given target.
// The target can include entity IDs, device IDs, area IDs, and label IDs.
// When expandGroup is true (default), group entities are expanded to their members.
func (c *wsClientImpl) GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.getForTarget(ctx, "get_conditions_for_target", target, expandGroup)
}

// GetServicesForTarget retrieves all applicable services for the given target.
// The target can include entity IDs, device IDs, area IDs, and label IDs.
// When expandGroup is true (default), group entities are expanded to their members.
func (c *wsClientImpl) GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return c.getForTarget(ctx, "get_services_for_target", target, expandGroup)
}

// ExtractFromTarget extracts entities, devices, and areas from the specified target.
// It resolves all referenced entities, devices, and areas while also reporting any
// missing devices, areas, floors, or labels. When expandGroup is true, group entities
// are expanded to their member entities instead of the group entity itself.
func (c *wsClientImpl) ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error) {
	params := map[string]any{
		"target": target,
	}
	if expandGroup != nil {
		params["expand_group"] = *expandGroup
	}

	result, err := c.ws.SendCommand(ctx, "extract_from_target", params)
	if err != nil {
		return nil, fmt.Errorf("extract_from_target failed: %w", err)
	}

	var extractResult ExtractFromTargetResult
	if err := json.Unmarshal(result.Result, &extractResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extract_from_target response: %w", err)
	}

	return &extractResult, nil
}

// =============================================================================
// Config Entry Operations (WebSocket-only)
// =============================================================================

// GetConfigEntries retrieves config entries, optionally filtered by domain.
// Uses the config_entries/get WebSocket command.
// Note: The options field is not populated by this API - it returns metadata only.
// To read options, use GetConfigEntryOptions() which uses the Options Flow REST API.
func (c *wsClientImpl) GetConfigEntries(ctx context.Context, domain string) ([]ConfigEntryFull, error) {
	payload := map[string]any{}
	if domain != "" {
		payload["domain"] = domain
	}
	result, err := c.ws.SendCommand(ctx, "config_entries/get", payload)
	if err != nil {
		return nil, fmt.Errorf("get config entries failed: %w", err)
	}
	var entries []ConfigEntryFull
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config entries: %w", err)
	}
	return entries, nil
}

// GetConfigEntry retrieves a single config entry by its entry ID.
// Uses the config_entries/get_single WebSocket command.
// Note: The options field is not populated by this API - use GetConfigEntries for listing.
// To read options, use GetConfigEntryOptions() which uses the Options Flow REST API.
func (c *wsClientImpl) GetConfigEntry(ctx context.Context, entryID string) (*ConfigEntryFull, error) {
	result, err := c.ws.SendCommand(ctx, "config_entries/get_single", map[string]any{
		"entry_id": entryID,
	})
	if err != nil {
		return nil, fmt.Errorf("get config entry failed: %w", err)
	}

	// Home Assistant wraps the response in a "config_entry" key
	var wrapper struct {
		ConfigEntry ConfigEntryFull `json:"config_entry"`
	}
	if err := json.Unmarshal(result.Result, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config entry: %w", err)
	}
	return &wrapper.ConfigEntry, nil
}

// =============================================================================
// HACS Operations
// =============================================================================

// SendHACSCommand sends a generic HACS WebSocket command.
// HACS is an optional third-party add-on, so commands may fail if not installed.
// The result is returned as untyped JSON since HACS responses vary by command.
func (c *wsClientImpl) SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error) {
	result, err := c.ws.SendCommand(ctx, command, data)
	if err != nil {
		return nil, fmt.Errorf("hacs command failed: %w", err)
	}

	// Return raw JSON result - HACS responses are untyped
	var response any
	if err := json.Unmarshal(result.Result, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hacs response: %w", err)
	}

	return response, nil
}

// =============================================================================
// System Log Operations (WebSocket-only)
// =============================================================================

// GetSystemLog retrieves the system log entries from the Home Assistant ring buffer.
func (c *wsClientImpl) GetSystemLog(ctx context.Context) ([]SystemLogEntry, error) {
	result, err := c.ws.SendCommand(ctx, "system_log/list", nil)
	if err != nil {
		return nil, fmt.Errorf("get system log failed: %w", err)
	}

	var entries []SystemLogEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal system log: %w", err)
	}

	return entries, nil
}

// ClearSystemLog clears the Home Assistant system log ring buffer.
func (c *wsClientImpl) ClearSystemLog(ctx context.Context) error {
	_, err := c.CallService(ctx, "system_log", "clear", nil)
	if err != nil {
		return fmt.Errorf("clear system log failed: %w", err)
	}
	return nil
}

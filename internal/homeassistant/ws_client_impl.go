// Package homeassistant provides the WebSocket-based Client implementation.
package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// wsClientImpl implements the Client interface using WebSocket commands.
// It wraps the WSClient for low-level WebSocket communication.
type wsClientImpl struct {
	ws *WSClient
}

// NewWSClientImpl creates a new WebSocket-based Client implementation.
func NewWSClientImpl(ws *WSClient) Client {
	return &wsClientImpl{ws: ws}
}

// Ensure wsClientImpl implements Client interface at compile time.
var _ Client = (*wsClientImpl)(nil)

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
			return []Entity{}, nil //nolint:nilerr
		}
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
		if len(entity.EntityID) > 11 && entity.EntityID[:11] == "automation." {
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
	if len(automationID) < 11 || automationID[:11] != "automation." {
		entityID = "automation." + automationID
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

// CreateAutomation creates a new automation.
func (c *wsClientImpl) CreateAutomation(ctx context.Context, config AutomationConfig) error {
	params := map[string]any{}
	if config.ID != "" {
		params["automation_id"] = config.ID
	}
	if config.Alias != "" {
		params["alias"] = config.Alias
	}
	if config.Description != "" {
		params["description"] = config.Description
	}
	if config.Triggers != nil {
		params["trigger"] = config.Triggers
	}
	if config.Conditions != nil {
		params["condition"] = config.Conditions
	}
	if config.Actions != nil {
		params["action"] = config.Actions
	}
	if config.Mode != "" {
		params["mode"] = config.Mode
	}

	_, err := c.ws.SendCommand(ctx, "config/automation/create", params)
	if err != nil {
		return fmt.Errorf("create automation failed: %w", err)
	}

	return nil
}

// UpdateAutomation updates an existing automation.
func (c *wsClientImpl) UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error {
	params := map[string]any{
		"automation_id": automationID,
	}
	if config.Alias != "" {
		params["alias"] = config.Alias
	}
	if config.Description != "" {
		params["description"] = config.Description
	}
	if config.Triggers != nil {
		params["trigger"] = config.Triggers
	}
	if config.Conditions != nil {
		params["condition"] = config.Conditions
	}
	if config.Actions != nil {
		params["action"] = config.Actions
	}
	if config.Mode != "" {
		params["mode"] = config.Mode
	}

	_, err := c.ws.SendCommand(ctx, "config/automation/update", params)
	if err != nil {
		return fmt.Errorf("update automation failed: %w", err)
	}

	return nil
}

// DeleteAutomation deletes an automation.
func (c *wsClientImpl) DeleteAutomation(ctx context.Context, automationID string) error {
	_, err := c.ws.SendCommand(ctx, "config/automation/delete", map[string]any{
		"automation_id": automationID,
	})
	if err != nil {
		return fmt.Errorf("delete automation failed: %w", err)
	}

	return nil
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

// ListHelpers lists all input helpers.
func (c *wsClientImpl) ListHelpers(ctx context.Context) ([]Entity, error) {
	entities, err := c.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var helpers []Entity
	helperPrefixes := []string{"input_boolean.", "input_number.", "input_text.", "input_select.", "input_datetime."}

	for _, entity := range entities {
		for _, prefix := range helperPrefixes {
			if len(entity.EntityID) > len(prefix) && entity.EntityID[:len(prefix)] == prefix {
				helpers = append(helpers, entity)
				break
			}
		}
	}

	return helpers, nil
}

// CreateHelper creates a new input helper.
func (c *wsClientImpl) CreateHelper(ctx context.Context, config HelperConfig) error {
	cmdType := fmt.Sprintf("%s/create", config.Platform)
	params := map[string]any{}

	// Add ID if provided
	if config.ID != "" {
		params[config.Platform+"_id"] = config.ID
	}

	// Add config fields
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

// UpdateHelper updates an existing input helper.
func (c *wsClientImpl) UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error {
	cmdType := fmt.Sprintf("%s/update", config.Platform)
	params := map[string]any{
		config.Platform + "_id": helperID,
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
		if len(entity.EntityID) > 7 && entity.EntityID[:7] == "script." {
			scripts = append(scripts, entity)
		}
	}

	return scripts, nil
}

// CreateScript creates a new script.
func (c *wsClientImpl) CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	params := map[string]any{
		"script_id": scriptID,
	}
	if config.Alias != "" {
		params["alias"] = config.Alias
	}
	if config.Description != "" {
		params["description"] = config.Description
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Mode != "" {
		params["mode"] = config.Mode
	}
	if config.Sequence != nil {
		params["sequence"] = config.Sequence
	}
	if config.Fields != nil {
		params["fields"] = config.Fields
	}

	_, err := c.ws.SendCommand(ctx, "config/script/create", params)
	if err != nil {
		return fmt.Errorf("create script failed: %w", err)
	}

	return nil
}

// UpdateScript updates an existing script.
func (c *wsClientImpl) UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	params := map[string]any{
		"script_id": scriptID,
	}
	if config.Alias != "" {
		params["alias"] = config.Alias
	}
	if config.Description != "" {
		params["description"] = config.Description
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Mode != "" {
		params["mode"] = config.Mode
	}
	if config.Sequence != nil {
		params["sequence"] = config.Sequence
	}
	if config.Fields != nil {
		params["fields"] = config.Fields
	}

	_, err := c.ws.SendCommand(ctx, "config/script/update", params)
	if err != nil {
		return fmt.Errorf("update script failed: %w", err)
	}

	return nil
}

// DeleteScript deletes a script.
func (c *wsClientImpl) DeleteScript(ctx context.Context, scriptID string) error {
	_, err := c.ws.SendCommand(ctx, "config/script/delete", map[string]any{
		"script_id": scriptID,
	})
	if err != nil {
		return fmt.Errorf("delete script failed: %w", err)
	}

	return nil
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
		if len(entity.EntityID) > 6 && entity.EntityID[:6] == "scene." {
			scenes = append(scenes, entity)
		}
	}

	return scenes, nil
}

// CreateScene creates a new scene.
func (c *wsClientImpl) CreateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	params := map[string]any{
		"scene_id": sceneID,
		"name":     config.Name,
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Entities != nil {
		params["entities"] = config.Entities
	}

	_, err := c.ws.SendCommand(ctx, "config/scene/create", params)
	if err != nil {
		return fmt.Errorf("create scene failed: %w", err)
	}

	return nil
}

// UpdateScene updates an existing scene.
func (c *wsClientImpl) UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	params := map[string]any{
		"scene_id": sceneID,
	}
	if config.Name != "" {
		params["name"] = config.Name
	}
	if config.Icon != "" {
		params["icon"] = config.Icon
	}
	if config.Entities != nil {
		params["entities"] = config.Entities
	}

	_, err := c.ws.SendCommand(ctx, "config/scene/update", params)
	if err != nil {
		return fmt.Errorf("update scene failed: %w", err)
	}

	return nil
}

// DeleteScene deletes a scene.
func (c *wsClientImpl) DeleteScene(ctx context.Context, sceneID string) error {
	_, err := c.ws.SendCommand(ctx, "config/scene/delete", map[string]any{
		"scene_id": sceneID,
	})
	if err != nil {
		return fmt.Errorf("delete scene failed: %w", err)
	}

	return nil
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
func (c *wsClientImpl) GetLovelaceConfig(ctx context.Context) (map[string]any, error) {
	result, err := c.ws.SendCommand(ctx, "lovelace/config", nil)
	if err != nil {
		return nil, fmt.Errorf("get lovelace config failed: %w", err)
	}

	var config map[string]any
	if err := json.Unmarshal(result.Result, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lovelace config: %w", err)
	}

	return config, nil
}

// =============================================================================
// Statistics Operations (WebSocket-only)
// =============================================================================

// GetStatistics retrieves long-term statistics for entities.
func (c *wsClientImpl) GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error) {
	params := map[string]any{
		"statistic_ids": statIDs,
		"period":        period,
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
	for _, stats := range statsMap {
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

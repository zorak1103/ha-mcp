// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Scene action constants.
const (
	sceneActionList     = "list"
	sceneActionGet      = "get"
	sceneActionCreate   = "create"
	sceneActionUpdate   = "update"
	sceneActionDelete   = "delete"
	sceneActionActivate = "activate"
	sceneActionPatch    = "patch"
)

// SceneHandlers provides handlers for scene-related MCP tools.
type SceneHandlers struct{}

// NewSceneHandlers creates a new SceneHandlers instance.
func NewSceneHandlers() *SceneHandlers {
	return &SceneHandlers{}
}

// RegisterTools registers the consolidated manage_scene tool with the registry.
func (h *SceneHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageSceneTool(), h.handleManageScene)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *SceneHandlers) manageSceneTool() mcp.Tool {
	return mcp.Tool{
		Name: "manage_scene",
		Description: `Manage Home Assistant scenes - list, get, create, update, delete, activate, or patch.

Actions:
- list: List all scenes (optional filters: name_contains, entity_contains)
- get: Get details of a specific scene (requires scene_id)
- create: Create a new scene (requires scene_id, name, entities)
- update: Update an existing scene (requires scene_id)
- delete: Delete a scene (requires scene_id)
- activate: Activate a scene (requires scene_id, optional transition)
- patch: Apply RFC 6902 JSON Patch operations to a scene config (requires scene_id, operations)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Scene management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: list, get, create, update, delete, activate, patch",
					Enum:        []string{"list", "get", "create", "update", "delete", "activate", "patch"},
				},
				"scene_id": {
					Type:        "string",
					Description: "Scene identifier. For create: use bare ID without 'scene.' prefix (e.g., 'movie_night'). For other actions: accepts entity_id (scene.xyz) or friendly_name (case-insensitive partial match).",
				},
				"name": {
					Type:        "string",
					Description: "Friendly name for the scene (required for create)",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the scene (e.g., mdi:lightbulb)",
				},
				"entities": {
					Type:        "object",
					Description: "Entity states to set when scene is activated. Keys are entity IDs, values are state objects with 'state' and optional 'attributes'",
				},
				"name_contains": {
					Type:        "string",
					Description: "Filter by scene name or entity_id containing this string (for list action, case-insensitive)",
				},
				"entity_contains": {
					Type:        "string",
					Description: "Filter to scenes that contain this entity ID in their entity list (for list action)",
				},
				"transition": {
					Type:        "number",
					Description: "Transition time in seconds (for activate action)",
				},
				"operations": patchOperationsSchema(),
				"dry_run":    dryRunSchema(),
				"format": {
					Type:        "string",
					Enum:        []string{"natural", "json"},
					Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				},
			},
			Required: []string{"action"},
		},
	}
}

// =============================================================================
// Main Handler
// =============================================================================

func (h *SceneHandlers) handleManageScene(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case sceneActionList:
		return h.handleList(ctx, client, args)
	case sceneActionGet:
		return h.handleGet(ctx, client, args)
	case sceneActionCreate:
		return h.handleCreate(ctx, client, args)
	case sceneActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case sceneActionDelete:
		return h.handleDelete(ctx, client, args)
	case sceneActionActivate:
		return h.handleActivate(ctx, client, args)
	case sceneActionPatch:
		return h.handlePatch(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, delete, activate, or patch)", action)), nil
	}
}

// =============================================================================
// Action Handlers
// =============================================================================

// sceneInfo represents scene information for list output.
type sceneInfo struct {
	EntityID     string   `json:"entity_id"`
	State        string   `json:"state"`
	FriendlyName string   `json:"friendly_name,omitempty"`
	EntityIDs    []string `json:"entity_ids,omitempty"`
}

// sceneFilters holds filter parameters for listing scenes.
type sceneFilters struct {
	nameContains   string
	entityContains string
}

func (h *SceneHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing scenes: %v", err)), nil
	}

	filters := parseSceneFilters(args)
	result := filterScenes(scenes, filters)

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Convert to SceneInfo for formatter
	sceneInfoList := make([]formatter.SceneInfo, 0, len(result))
	for _, info := range result {
		sceneInfoList = append(sceneInfoList, formatter.SceneInfo{
			EntityID:     info.EntityID,
			State:        info.State,
			FriendlyName: info.FriendlyName,
			EntityIDs:    info.EntityIDs,
		})
	}

	f := formatter.NewSceneFormatter(format)
	opts := formatter.SceneListOptions{}

	output, err := f.FormatList(ctx, sceneInfoList, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting scenes: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *SceneHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for get action"), nil
	}

	entityID := "scene." + sceneID
	state, err := client.GetState(ctx, entityID)
	if err != nil {
		// Fallback: search by friendly_name
		state, err = h.findSceneByID(ctx, client, sceneID)
		if err != nil {
			return errorResult(fmt.Sprintf("Error getting scene: %v", err)), nil
		}
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)
	f := formatter.NewSceneFormatter(format)

	output, err := f.FormatDetail(ctx, *state)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting scene: %v", err)), nil
	}

	return successResult(output), nil
}

// findSceneByID searches for a scene by various ID formats.
// Search order: entity_id (scene.xyz), friendly_name (case-insensitive partial match).
func (h *SceneHandlers) findSceneByID(ctx context.Context, client homeassistant.Client, searchID string) (*homeassistant.Entity, error) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list scenes: %w", err)
	}

	// 1. Try as entity_id (scene.xyz)
	if strings.HasPrefix(searchID, "scene.") {
		for _, s := range scenes {
			if s.EntityID == searchID {
				return client.GetState(ctx, s.EntityID)
			}
		}
	}

	// 2. Search by friendly_name (case-insensitive partial match)
	searchLower := strings.ToLower(searchID)
	for _, s := range scenes {
		friendlyName, _ := s.Attributes["friendly_name"].(string)
		if strings.Contains(strings.ToLower(friendlyName), searchLower) {
			return client.GetState(ctx, s.EntityID)
		}
	}

	return nil, fmt.Errorf("scene not found: %s (tried as entity_id and friendly_name)", searchID)
}

// errAmbiguousSceneMatch signals that a scene identifier resolved to more than one candidate
// during a write-path fallback search - a write must never guess between multiple scenes.
var errAmbiguousSceneMatch = errors.New("ambiguous scene identifier")

// errSceneNotFoundForWrite signals that neither an exact id/entity_id lookup nor a fuzzy
// friendly-name search resolved to any scene.
var errSceneNotFoundForWrite = errors.New("scene not found")

// sceneConfigMissingError signals that GetScene 404'd for entityID/configID while the entity
// itself still exists (GetState succeeded) - i.e. it is YAML-defined under a different
// config-file key than its entity_id's object_id would suggest (#122/#164).
type sceneConfigMissingError struct {
	entityID, configID string
}

func (e *sceneConfigMissingError) Error() string {
	return fmt.Sprintf("scene config entry missing for %s (%s)", e.entityID, e.configID)
}

// joinSceneCandidates renders ambiguous-match candidate entity_ids for an error message.
func joinSceneCandidates(entities []*homeassistant.Entity) string {
	ids := make([]string, 0, len(entities))
	for _, e := range entities {
		ids = append(ids, e.EntityID)
	}
	return strings.Join(ids, ", ")
}

// findSceneForWrite resolves a scene for a write operation (update/patch) by friendly-name
// search, refusing an ambiguous match rather than silently picking the first candidate HA
// happens to list - mirrors scripts.go's findScriptForWrite (#160). Search order matches
// findSceneByID's: exact entity_id first (inherently unambiguous), then case-insensitive
// friendly_name substring match - but here a substring match is only accepted automatically
// when it is the sole substring match, or when exactly one of several substring matches is also
// an EXACT (not just substring) case-insensitive match. Uses the friendly_name already present on
// ListScenes' Entity.Attributes - no per-item GetState fetch needed, unlike the script equivalent.
func (h *SceneHandlers) findSceneForWrite(ctx context.Context, client homeassistant.Client, searchID string) (*homeassistant.Entity, error) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list scenes: %w", err)
	}

	if strings.HasPrefix(searchID, "scene.") {
		for i := range scenes {
			if scenes[i].EntityID == searchID {
				return &scenes[i], nil
			}
		}
	}

	searchLower := strings.ToLower(searchID)
	var substringMatches, exactMatches []*homeassistant.Entity
	for i := range scenes {
		friendly, _ := scenes[i].Attributes["friendly_name"].(string)
		if !strings.Contains(strings.ToLower(friendly), searchLower) {
			continue
		}
		substringMatches = append(substringMatches, &scenes[i])
		if strings.EqualFold(friendly, searchID) {
			exactMatches = append(exactMatches, &scenes[i])
		}
	}

	switch {
	case len(exactMatches) == 1:
		return exactMatches[0], nil
	case len(substringMatches) == 1:
		return substringMatches[0], nil
	case len(substringMatches) > 1:
		return nil, fmt.Errorf("%w: %q matches %d scenes: %s - use the exact entity_id",
			errAmbiguousSceneMatch, searchID, len(substringMatches), joinSceneCandidates(substringMatches))
	default:
		return nil, fmt.Errorf("scene not found: %s (tried as entity_id and friendly_name)", searchID)
	}
}

// resolveSceneForWrite resolves sceneID to (entityID, configID, current config) for a write
// operation, falling back to findSceneForWrite (friendly-name search) only when a direct GetScene
// lookup using the guessed config id returns a not-found error. Any other error (transient
// WS/REST failure, timeout, auth) is returned as-is without triggering the fallback search - a
// momentary hiccup on an otherwise-correct scene_id must never silently retarget a write via fuzzy
// matching. This was the gap that let the pre-remediation manage_scene patch retarget to an
// unrelated scene on any GetScene error, not just a genuine 404.
// Returns the entity_id/config_id re-derived from the entity actually resolved - not the raw
// guess from normalizeSceneID - since the fallback search may match a different underlying
// entity than what the caller's input implied.
func (h *SceneHandlers) resolveSceneForWrite(
	ctx context.Context,
	client homeassistant.Client,
	sceneID string,
) (entityID, configID string, current *homeassistant.Scene, err error) {
	guessedEntityID, guessedConfigID := normalizeSceneID(sceneID)

	scene, getErr := client.GetScene(ctx, guessedConfigID)
	if getErr == nil {
		return guessedEntityID, guessedConfigID, scene, nil
	}
	if !isNotFoundError(getErr) {
		return "", "", nil, getErr
	}

	resolvedEntity, findErr := h.findSceneForWrite(ctx, client, sceneID)
	if findErr != nil {
		if errors.Is(findErr, errAmbiguousSceneMatch) {
			return "", "", nil, findErr
		}
		// No fuzzy match either - fall back to the exact-id discrimination #122/#164 relies on:
		// if the entity still exists under the guessed entity_id, it's YAML-defined under a
		// different config-file key; otherwise it's a genuine not-found.
		if _, stateErr := client.GetState(ctx, guessedEntityID); stateErr == nil {
			return "", "", nil, &sceneConfigMissingError{entityID: guessedEntityID, configID: guessedConfigID}
		}
		return "", "", nil, fmt.Errorf("%w: %s", errSceneNotFoundForWrite, sceneID)
	}

	entityID, configID = normalizeSceneID(resolvedEntity.EntityID)
	scene, getErr = client.GetScene(ctx, configID)
	if getErr == nil {
		return entityID, configID, scene, nil
	}
	if !isNotFoundError(getErr) {
		return "", "", nil, getErr
	}
	return "", "", nil, &sceneConfigMissingError{entityID: entityID, configID: configID}
}

// describeSceneTarget renders the target scene for a success/error message, naming the resolved
// entity_id alongside the caller's input whenever resolveSceneForWrite's fallback search
// retargeted the write to a different entity than the input implied (mirrors scripts.go's
// describeScriptTarget, #160).
func describeSceneTarget(sceneID, entityID string) string {
	guessedEntityID, _ := normalizeSceneID(sceneID)
	if guessedEntityID == entityID {
		return fmt.Sprintf("'%s'", sceneID)
	}
	return fmt.Sprintf("'%s' (%s)", sceneID, entityID)
}

// sceneWriteResolveErrorResult renders a resolveSceneForWrite error into the tool result the
// caller (update/patch) should return, sharing one message set between both actions.
func sceneWriteResolveErrorResult(ctx context.Context, client homeassistant.Client, action, sceneID string, err error) *mcp.ToolsCallResult {
	var missing *sceneConfigMissingError
	switch {
	case errors.As(err, &missing):
		return errorResult(configFileMissingWriteError(ctx, client, "scene", action, sceneID, missing.entityID, missing.configID))
	case errors.Is(err, errSceneNotFoundForWrite):
		return errorResult(fmt.Sprintf("scene not found: %s", sceneID))
	case errors.Is(err, errAmbiguousSceneMatch):
		return errorResult(err.Error())
	default:
		return errorResult(fmt.Sprintf("Error getting current scene: %v", err))
	}
}

func (h *SceneHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for create action"), nil
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return errorResult("name is required for create action"), nil
	}

	entitiesRaw, ok := args["entities"].(map[string]any)
	if !ok || len(entitiesRaw) == 0 {
		return errorResult("entities is required for create action and must be a non-empty object"), nil
	}

	entities, invalidEntity := parseSceneEntities(entitiesRaw)
	if invalidEntity != "" {
		return errorResult(fmt.Sprintf("Invalid state format for entity %s", invalidEntity)), nil
	}

	config := homeassistant.SceneConfig{Name: name, Entities: entities}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}

	if err := client.CreateScene(ctx, sceneID, config); err != nil {
		return errorResult(fmt.Sprintf("Error creating scene: %v", err)), nil
	}

	// HA derives entity_id from name (slugified), not from scene_id (config key).
	nameSlug := slugifyName(name)
	entityID := "scene." + nameSlug
	successMsg := fmt.Sprintf("Scene '%s' created successfully (entity_id: %s, config_id: %s)", name, entityID, sceneID)
	if _, appeared := reloadAndWaitForEntity(ctx, client, "scene", entityID); !appeared {
		successMsg += " (note: scene may require Home Assistant restart to become visible)"
	}

	return successResult(successMsg), nil
}

func (h *SceneHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for update action"), nil
	}

	entityID, configID, current, err := h.resolveSceneForWrite(ctx, client, sceneID)
	if err != nil {
		return sceneWriteResolveErrorResult(ctx, client, "update", sceneID, err), nil
	}
	target := describeSceneTarget(sceneID, entityID)

	if current.Config == nil || len(current.Config.Entities) == 0 {
		return errorResult(fmt.Sprintf("scene %s has no configuration to update (missing entities)", target)), nil
	}

	// The fetched config is the write base - only fields present in args are overwritten,
	// everything else (icon, metadata) survives untouched (#173).
	config := *current.Config
	beforeMap, _ := configToMap(config)
	invalidEntity := applySceneConfigUpdates(&config, args)
	if invalidEntity != "" {
		return errorResult(fmt.Sprintf("Invalid state format for entity %s", invalidEntity)), nil
	}

	// Skip a no-op write: avoids a needless scenes.yaml rewrite (which reformats hand-authored
	// shorthand entity values via SceneState.MarshalJSON) and the reload HA's config API triggers.
	afterMap, _ := configToMap(config)
	if reflect.DeepEqual(beforeMap, afterMap) {
		return successResult(fmt.Sprintf("Scene %s: no changes detected, skipping write (reload avoided)", target)), nil
	}

	if err := client.UpdateScene(ctx, configID, config); err != nil {
		msg := fmt.Sprintf("Error updating scene: %v", err)
		return errorResult(enrichConfigError(msg, err, sceneErrorHints)), nil
	}

	return successResult(fmt.Sprintf("Scene %s updated successfully", target)), nil
}

func (h *SceneHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for delete action"), nil
	}

	// Normalize ID to handle prefix variations - use configID for REST API
	_, configID := normalizeSceneID(sceneID)

	if err := client.DeleteScene(ctx, configID); err != nil {
		return errorResult(fmt.Sprintf("Error deleting scene: %v", err)), nil
	}

	entityID, _ := normalizeSceneID(sceneID)
	successMsg := fmt.Sprintf("Scene '%s' deleted successfully", sceneID)
	if !waitForEntityDisappear(ctx, client, entityID) {
		successMsg += " (note: scene may remain visible until Home Assistant restart)"
	}

	return successResult(successMsg), nil
}

func (h *SceneHandlers) handleActivate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for activate action"), nil
	}

	// Normalize ID to handle prefix variations
	entityID, _ := normalizeSceneID(sceneID)

	data := map[string]any{
		"entity_id": entityID,
	}

	if transition, ok := args["transition"].(float64); ok {
		data["transition"] = transition
	}

	if _, err := client.CallService(ctx, "scene", "turn_on", data); err != nil {
		return errorResult(fmt.Sprintf("Error activating scene: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Scene '%s' activated successfully", sceneID)), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// normalizeSceneID normalizes scene ID inputs to handle prefix variations.
// Returns: (entityID with "scene." prefix, configID without prefix)
// Examples:
//   - "scene.movie_night" -> ("scene.movie_night", "movie_night")
//   - "movie_night" -> ("scene.movie_night", "movie_night")
func normalizeSceneID(sceneID string) (entityID, configID string) {
	if strings.HasPrefix(sceneID, "scene.") {
		return sceneID, strings.TrimPrefix(sceneID, "scene.")
	}
	return "scene." + sceneID, sceneID
}

// parseSceneFilters extracts filter parameters from args.
func parseSceneFilters(args map[string]any) sceneFilters {
	nameContains, _ := args["name_contains"].(string)
	entityContains, _ := args["entity_contains"].(string)
	return sceneFilters{
		nameContains:   nameContains,
		entityContains: entityContains,
	}
}

// entityToSceneInfo converts an Entity to sceneInfo.
func entityToSceneInfo(s homeassistant.Entity) sceneInfo {
	info := sceneInfo{
		EntityID: s.EntityID,
		State:    s.State,
	}
	if name, ok := s.Attributes["friendly_name"].(string); ok {
		info.FriendlyName = name
	}
	if entityIDs, ok := s.Attributes["entity_id"].([]any); ok {
		info.EntityIDs = extractStringSlice(entityIDs)
	}
	return info
}

// extractStringSlice converts []any to []string.
func extractStringSlice(items []any) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// matchesSceneFilters checks if scene info matches all filters.
func matchesSceneFilters(info sceneInfo, filters sceneFilters) bool {
	if !matchesSceneNameFilter(info, filters.nameContains) {
		return false
	}
	if !matchesSceneEntityFilter(info, filters.entityContains) {
		return false
	}
	return true
}

// matchesSceneNameFilter checks if scene matches name filter.
func matchesSceneNameFilter(info sceneInfo, nameContains string) bool {
	if nameContains == "" {
		return true
	}
	nameLower := strings.ToLower(nameContains)
	return strings.Contains(strings.ToLower(info.EntityID), nameLower) ||
		strings.Contains(strings.ToLower(info.FriendlyName), nameLower)
}

// matchesSceneEntityFilter checks if scene contains the entity.
func matchesSceneEntityFilter(info sceneInfo, entityContains string) bool {
	if entityContains == "" {
		return true
	}
	entityLower := strings.ToLower(entityContains)
	for _, eid := range info.EntityIDs {
		if strings.Contains(strings.ToLower(eid), entityLower) {
			return true
		}
	}
	return false
}

// filterScenes applies filters to scenes and converts to sceneInfo.
func filterScenes(scenes []homeassistant.Entity, filters sceneFilters) []sceneInfo {
	result := make([]sceneInfo, 0, len(scenes))
	for _, s := range scenes {
		info := entityToSceneInfo(s)
		if matchesSceneFilters(info, filters) {
			result = append(result, info)
		}
	}
	return result
}

// parseSceneState converts raw state data to SceneState.
func parseSceneState(stateRaw any) (homeassistant.SceneState, bool) {
	sceneState := homeassistant.SceneState{}

	switch v := stateRaw.(type) {
	case string:
		sceneState.State = v
		return sceneState, true
	case map[string]any:
		if state, ok := v["state"].(string); ok {
			sceneState.State = state
		}
		if attrs, ok := v["attributes"].(map[string]any); ok {
			sceneState.Attributes = attrs
		}
		return sceneState, true
	default:
		return sceneState, false
	}
}

// parseSceneEntities converts raw entities map to SceneState map.
func parseSceneEntities(entitiesRaw map[string]any) (map[string]homeassistant.SceneState, string) {
	entities := make(map[string]homeassistant.SceneState, len(entitiesRaw))
	for entityID, stateRaw := range entitiesRaw {
		sceneState, ok := parseSceneState(stateRaw)
		if !ok {
			return nil, entityID
		}
		entities[entityID] = sceneState
	}
	return entities, ""
}

// applySceneConfigUpdates merges caller-supplied args onto an existing SceneConfig fetched via
// GetScene. Only fields present in args are overwritten - name, icon, and metadata are otherwise
// left untouched. entities, when present, replaces the whole map wholesale (mirrors how
// applyScriptConfigUpdates replaces sequence); surgical single-entity edits are action=patch's job.
// applySceneConfigUpdates merges caller-supplied args onto an existing SceneConfig fetched via
// GetScene. Only fields present in args are overwritten - name, icon, and metadata are otherwise
// left untouched. entities, when present, replaces the whole map wholesale (mirrors how
// applyScriptConfigUpdates replaces sequence); surgical single-entity edits are action=patch's job.
// Reuses parseSceneEntities - the same validator handleCreate uses - instead of re-implementing
// its loop, so a malformed entity state (e.g. a bare number) is rejected here exactly as it would
// be on create, rather than silently stored as an empty SceneState. Returns the offending
// entity_id, or "" if entities is absent or every value parsed cleanly.
func applySceneConfigUpdates(config *homeassistant.SceneConfig, args map[string]any) (invalidEntity string) {
	if name, ok := args["name"].(string); ok {
		config.Name = name
	}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}
	if entitiesRaw, ok := args["entities"].(map[string]any); ok {
		entities, invalid := parseSceneEntities(entitiesRaw)
		if invalid != "" {
			return invalid
		}
		config.Entities = entities
	}
	return ""
}

func (h *SceneHandlers) handlePatch(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for patch action"), nil
	}

	ops, errResult := parseOperations(args)
	if errResult != nil {
		return errResult, nil
	}

	entityID, configID, current, err := h.resolveSceneForWrite(ctx, client, sceneID)
	if err != nil {
		return sceneWriteResolveErrorResult(ctx, client, "patch", sceneID, err), nil
	}
	target := describeSceneTarget(sceneID, entityID)

	if current.Config == nil || len(current.Config.Entities) == 0 {
		return errorResult(fmt.Sprintf("scene %s has no configuration to patch (missing entities)", target)), nil
	}

	configMap, err := configToMap(current.Config)
	if err != nil {
		return errorResult(fmt.Sprintf("error processing scene config: %v", err)), nil
	}

	patchedMap, resolvedOps, patchErr := applyPatchWithSemantics(configMap, ops)
	if patchErr != nil {
		return errorResult(fmt.Sprintf("error applying patch: %v", patchErr)), nil
	}

	if dryRun, _ := args["dry_run"].(bool); dryRun {
		return dryRunPatchResult(configMap, resolvedOps, "scene", sceneID, len(ops))
	}

	var newConfig homeassistant.SceneConfig
	if err := mapToStruct(patchedMap, &newConfig); err != nil {
		return errorResult(fmt.Sprintf("error parsing patched config: %v", err)), nil
	}

	if err := client.UpdateScene(ctx, configID, newConfig); err != nil {
		msg := fmt.Sprintf("error saving patched scene: %v", err)
		return errorResult(enrichConfigError(msg, err, sceneErrorHints)), nil
	}

	return successResult(fmt.Sprintf("Scene %s patched successfully (%d operations applied)", target, len(ops))), nil
}

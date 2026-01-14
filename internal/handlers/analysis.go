// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// AnalysisHandlers provides MCP tool handlers for entity analysis operations.
type AnalysisHandlers struct{}

// NewAnalysisHandlers creates a new AnalysisHandlers instance.
func NewAnalysisHandlers() *AnalysisHandlers {
	return &AnalysisHandlers{}
}

// RegisterTools registers all analysis-related tools with the registry.
func (h *AnalysisHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.analyzeEntityTool(), h.handleAnalyzeEntity)
	registry.RegisterTool(h.getEntityDependenciesTool(), h.handleGetEntityDependencies)
}

func (h *AnalysisHandlers) analyzeEntityTool() mcp.Tool {
	return mcp.Tool{
		Name:        "analyze_entity",
		Description: "Analyze an entity and find all automations, scripts, and scenes that reference it. Returns a comprehensive overview of how the entity is controlled and used in Home Assistant.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for analyzing an entity",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The entity ID to analyze (e.g., 'light.living_room', 'sensor.temperature')",
				},
				"include_history": {
					Type:        "boolean",
					Description: "If true, include recent state history (last 24 hours). Default: false",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *AnalysisHandlers) getEntityDependenciesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_entity_dependencies",
		Description: "Get all entities that an automation or script depends on. Shows triggers, conditions, and action targets.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for getting entity dependencies",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The automation or script entity ID (e.g., 'automation.my_automation', 'script.my_script')",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

// EntityAnalysis represents the comprehensive analysis of an entity.
type EntityAnalysis struct {
	EntityID     string            `json:"entity_id"`
	State        string            `json:"state"`
	FriendlyName string            `json:"friendly_name,omitempty"`
	Domain       string            `json:"domain"`
	Attributes   map[string]any    `json:"attributes,omitempty"`
	LastChanged  string            `json:"last_changed,omitempty"`
	References   *EntityReferences `json:"references"`
	Summary      string            `json:"summary"`
	History      []HistoryEntry    `json:"history,omitempty"`
}

// EntityReferences contains all automations, scripts, and scenes referencing an entity.
type EntityReferences struct {
	Automations     []AutomationReference `json:"automations,omitempty"`
	Scripts         []ScriptReference     `json:"scripts,omitempty"`
	Scenes          []SceneReference      `json:"scenes,omitempty"`
	Groups          []string              `json:"groups,omitempty"`
	AreaReferences  []AreaReference       `json:"area_references,omitempty"`
	TotalReferences int                   `json:"total_references"`
}

// AreaReference describes how an automation/script references an entity via its area.
type AreaReference struct {
	EntityID string   `json:"entity_id"`
	Alias    string   `json:"alias,omitempty"`
	Type     string   `json:"type"` // "automation" or "script"
	AreaID   string   `json:"area_id"`
	UsedIn   []string `json:"used_in"` // "trigger", "condition", "action"
}

// AutomationReference describes how an automation references an entity.
type AutomationReference struct {
	EntityID      string   `json:"entity_id"`
	Alias         string   `json:"alias,omitempty"`
	State         string   `json:"state"`
	LastTriggered string   `json:"last_triggered,omitempty"`
	UsedIn        []string `json:"used_in"` // "trigger", "condition", "action"
}

// ScriptReference describes how a script references an entity.
type ScriptReference struct {
	EntityID     string `json:"entity_id"`
	FriendlyName string `json:"friendly_name,omitempty"`
	UsedIn       string `json:"used_in"` // "action"
}

// SceneReference describes how a scene references an entity.
type SceneReference struct {
	EntityID     string `json:"entity_id"`
	FriendlyName string `json:"friendly_name,omitempty"`
}

// HistoryEntry represents a state change in history.
type HistoryEntry struct {
	State       string `json:"state"`
	LastChanged string `json:"last_changed"`
}

// EntityDependencies represents all dependencies of an automation or script.
type EntityDependencies struct {
	EntityID     string                `json:"entity_id"`
	FriendlyName string                `json:"friendly_name,omitempty"`
	Type         string                `json:"type"` // "automation" or "script"
	Dependencies *DependencyCategories `json:"dependencies"`
	Summary      string                `json:"summary"`
}

// DependencyCategories organizes dependencies by their role.
type DependencyCategories struct {
	Triggers   []DependencyEntry `json:"triggers,omitempty"`
	Conditions []DependencyEntry `json:"conditions,omitempty"`
	Actions    []DependencyEntry `json:"actions,omitempty"`
	Variables  []string          `json:"variables,omitempty"`
	Areas      []string          `json:"areas,omitempty"`
	Devices    []string          `json:"devices,omitempty"`
	Services   []string          `json:"services,omitempty"`
}

// DependencyEntry represents a single dependency.
type DependencyEntry struct {
	EntityID    string `json:"entity_id"`
	Type        string `json:"type,omitempty"`        // e.g., "state", "numeric_state", "time"
	Description string `json:"description,omitempty"` // Human-readable description
}

func (h *AnalysisHandlers) handleAnalyzeEntity(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	includeHistory, _ := args["include_history"].(bool)

	analysis, err := h.buildEntityAnalysis(ctx, client, entityID, includeHistory)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(err.Error())},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting analysis: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

func (h *AnalysisHandlers) buildEntityAnalysis(ctx context.Context, client homeassistant.Client, entityID string, includeHistory bool) (*EntityAnalysis, error) {
	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("error getting entity state: %w", err)
	}

	parts := strings.SplitN(entityID, ".", 2)
	domain := ""
	if len(parts) > 0 {
		domain = parts[0]
	}

	friendlyName := ""
	if fn, ok := state.Attributes["friendly_name"].(string); ok {
		friendlyName = fn
	}

	analysis := &EntityAnalysis{
		EntityID:     entityID,
		State:        state.State,
		FriendlyName: friendlyName,
		Domain:       domain,
		Attributes:   state.Attributes,
		LastChanged:  state.LastChanged.Format(time.RFC3339),
		References:   &EntityReferences{},
	}

	// Find all references
	h.findAutomationReferences(ctx, client, entityID, analysis.References)
	h.findScriptReferences(ctx, client, entityID, analysis.References)
	h.findSceneReferences(ctx, client, entityID, analysis.References)
	h.findGroupReferences(ctx, client, entityID, analysis.References)

	// Find area-based references (entity controlled via area_id in automations/scripts)
	h.findAreaReferences(ctx, client, entityID, analysis.References)

	// Calculate total references
	analysis.References.TotalReferences = len(analysis.References.Automations) +
		len(analysis.References.Scripts) +
		len(analysis.References.Scenes) +
		len(analysis.References.Groups) +
		len(analysis.References.AreaReferences)

	// Include history if requested
	if includeHistory {
		analysis.History = h.getEntityHistory(ctx, client, entityID)
	}

	analysis.Summary = h.generateEntitySummary(analysis)

	return analysis, nil
}

func (h *AnalysisHandlers) findAutomationReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return
	}

	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		fullAuto, getErr := client.GetAutomation(ctx, autoID)
		if getErr != nil || fullAuto.Config == nil {
			continue
		}

		usedIn := h.findEntityUsageInAutomation(fullAuto.Config, entityID)
		if len(usedIn) > 0 {
			refs.Automations = append(refs.Automations, AutomationReference{
				EntityID:      auto.EntityID,
				Alias:         auto.FriendlyName,
				State:         auto.State,
				LastTriggered: auto.LastTriggered,
				UsedIn:        usedIn,
			})
		}
	}
}

func (h *AnalysisHandlers) findScriptReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) {
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return
	}

	for _, script := range scripts {
		sequence, ok := script.Attributes["sequence"].([]any)
		if !ok || !searchInConfigSlice(sequence, entityID) {
			continue
		}

		fn := ""
		if name, ok := script.Attributes["friendly_name"].(string); ok {
			fn = name
		}
		refs.Scripts = append(refs.Scripts, ScriptReference{
			EntityID:     script.EntityID,
			FriendlyName: fn,
			UsedIn:       "action",
		})
	}
}

func (h *AnalysisHandlers) findSceneReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return
	}

	for _, scene := range scenes {
		entities, ok := scene.Attributes["entity_id"].([]any)
		if !ok {
			continue
		}

		for _, e := range entities {
			if eStr, ok := e.(string); ok && eStr == entityID {
				fn := ""
				if name, ok := scene.Attributes["friendly_name"].(string); ok {
					fn = name
				}
				refs.Scenes = append(refs.Scenes, SceneReference{
					EntityID:     scene.EntityID,
					FriendlyName: fn,
				})
				break
			}
		}
	}
}

func (h *AnalysisHandlers) findGroupReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) {
	allStates, err := client.GetStates(ctx)
	if err != nil {
		return
	}

	for _, s := range allStates {
		if !strings.HasPrefix(s.EntityID, "group.") {
			continue
		}

		entities, ok := s.Attributes["entity_id"].([]any)
		if !ok {
			continue
		}

		for _, e := range entities {
			if eStr, ok := e.(string); ok && eStr == entityID {
				refs.Groups = append(refs.Groups, s.EntityID)
				break
			}
		}
	}
}

// findAreaReferences finds automations and scripts that reference the entity's area.
// This detects indirect control where automations target an area_id instead of specific entities.
func (h *AnalysisHandlers) findAreaReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) {
	// First, find which area the entity belongs to
	entityArea := h.getEntityArea(ctx, client, entityID)
	if entityArea == "" {
		return // Entity is not assigned to any area
	}

	// Search automations for area-based references
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return
	}

	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		fullAuto, getErr := client.GetAutomation(ctx, autoID)
		if getErr != nil || fullAuto.Config == nil {
			continue
		}

		usedIn := h.findAreaUsageInConfig(fullAuto.Config.Triggers, fullAuto.Config.Conditions, fullAuto.Config.Actions, entityArea)
		if len(usedIn) > 0 {
			refs.AreaReferences = append(refs.AreaReferences, AreaReference{
				EntityID: auto.EntityID,
				Alias:    auto.FriendlyName,
				Type:     "automation",
				AreaID:   entityArea,
				UsedIn:   usedIn,
			})
		}
	}

	// Search scripts for area-based references
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return
	}

	for _, script := range scripts {
		sequence, ok := script.Attributes["sequence"].([]any)
		if !ok {
			continue
		}

		if h.searchAreaInSlice(sequence, entityArea) {
			fn := ""
			if name, ok := script.Attributes["friendly_name"].(string); ok {
				fn = name
			}
			refs.AreaReferences = append(refs.AreaReferences, AreaReference{
				EntityID: script.EntityID,
				Alias:    fn,
				Type:     "script",
				AreaID:   entityArea,
				UsedIn:   []string{"action"},
			})
		}
	}
}

// getEntityArea returns the area_id for an entity, either directly or via its device.
func (h *AnalysisHandlers) getEntityArea(ctx context.Context, client homeassistant.Client, entityID string) string {
	// Check entity registry for direct area assignment
	entities, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return ""
	}

	var entityEntry *homeassistant.EntityRegistryEntry
	for _, e := range entities {
		if e.EntityID == entityID {
			entityEntry = &e
			break
		}
	}

	if entityEntry == nil {
		return ""
	}

	// If entity has a direct area_id, return it
	if entityEntry.AreaID != "" {
		return entityEntry.AreaID
	}

	// Otherwise, check the device's area
	if entityEntry.DeviceID != "" {
		devices, err := client.GetDeviceRegistry(ctx)
		if err != nil {
			return ""
		}

		for _, d := range devices {
			if d.ID == entityEntry.DeviceID {
				return d.AreaID
			}
		}
	}

	return ""
}

// findAreaUsageInConfig searches for area usage in automation config.
func (h *AnalysisHandlers) findAreaUsageInConfig(triggers, conditions, actions []any, areaID string) []string {
	var usedIn []string

	if h.searchAreaInSlice(triggers, areaID) {
		usedIn = append(usedIn, "trigger")
	}
	if h.searchAreaInSlice(conditions, areaID) {
		usedIn = append(usedIn, "condition")
	}
	if h.searchAreaInSlice(actions, areaID) {
		usedIn = append(usedIn, "action")
	}

	return usedIn
}

// searchAreaInSlice recursively searches for an area_id in a config slice.
func (h *AnalysisHandlers) searchAreaInSlice(items []any, areaID string) bool {
	for _, item := range items {
		if h.searchAreaInValue(item, areaID) {
			return true
		}
	}
	return false
}

// searchAreaInValue recursively searches for an area_id in any config value.
// It delegates to specialized functions based on the value type.
func (h *AnalysisHandlers) searchAreaInValue(val any, areaID string) bool {
	if val == nil {
		return false
	}

	switch v := val.(type) {
	case string:
		return v == areaID
	case []any:
		return h.searchAreaInSlice(v, areaID)
	case map[string]any:
		return h.searchAreaInMap(v, areaID)
	default:
		return false
	}
}

// searchAreaInMap searches for an area_id in a map structure.
func (h *AnalysisHandlers) searchAreaInMap(m map[string]any, areaID string) bool {
	// Check direct area_id field
	if h.matchAreaIDField(m["area_id"], areaID) {
		return true
	}

	// Check target.area_id
	if target, ok := m["target"].(map[string]any); ok {
		if h.matchAreaIDField(target["area_id"], areaID) {
			return true
		}
	}

	// Recursively search nested structures
	for _, subval := range m {
		if h.searchAreaInValue(subval, areaID) {
			return true
		}
	}
	return false
}

// matchAreaIDField checks if an area_id field (string or []any) matches the target areaID.
func (h *AnalysisHandlers) matchAreaIDField(field any, areaID string) bool {
	if field == nil {
		return false
	}

	switch v := field.(type) {
	case string:
		return v == areaID
	case []any:
		for _, item := range v {
			if str, ok := item.(string); ok && str == areaID {
				return true
			}
		}
	}
	return false
}

func (h *AnalysisHandlers) getEntityHistory(ctx context.Context, client homeassistant.Client, entityID string) []HistoryEntry {
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)
	historyData, err := client.GetHistory(ctx, entityID, startTime, endTime)
	if err != nil || len(historyData) == 0 || len(historyData[0]) == 0 {
		return nil
	}

	var history []HistoryEntry
	for _, entry := range historyData[0] {
		if len(history) >= 20 {
			break
		}
		history = append(history, HistoryEntry{
			State:       entry.State,
			LastChanged: entry.LastChangedTime().Format(time.RFC3339),
		})
	}
	return history
}

func (h *AnalysisHandlers) handleGetEntityDependencies(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	var deps *EntityDependencies
	var err error

	switch {
	case strings.HasPrefix(entityID, "automation."):
		deps, err = h.getAutomationDependencies(ctx, client, entityID)
	case strings.HasPrefix(entityID, "script."):
		deps, err = h.getScriptDependencies(ctx, client, entityID)
	default:
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an automation or script (e.g., 'automation.my_automation' or 'script.my_script')")},
			IsError: true,
		}, nil
	}

	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting dependencies: %v", err))},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting dependencies: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

func (h *AnalysisHandlers) getAutomationDependencies(ctx context.Context, client homeassistant.Client, entityID string) (*EntityDependencies, error) {
	automationID := strings.TrimPrefix(entityID, "automation.")
	automation, err := client.GetAutomation(ctx, automationID)
	if err != nil {
		return nil, err
	}

	deps := &EntityDependencies{
		EntityID:     entityID,
		FriendlyName: automation.FriendlyName,
		Type:         "automation",
		Dependencies: &DependencyCategories{},
	}

	if automation.Config == nil {
		deps.Summary = "Automation configuration not available"
		return deps, nil
	}

	// Extract triggers
	deps.Dependencies.Triggers = h.extractDependenciesFromSlice(automation.Config.Triggers, "trigger")

	// Extract conditions
	deps.Dependencies.Conditions = h.extractDependenciesFromSlice(automation.Config.Conditions, "condition")

	// Extract actions
	deps.Dependencies.Actions = h.extractDependenciesFromSlice(automation.Config.Actions, "action")

	// Extract services used
	deps.Dependencies.Services = h.extractServicesFromSlice(automation.Config.Actions)

	// Extract areas and devices
	deps.Dependencies.Areas = h.extractAreasFromSlice(automation.Config.Actions)
	deps.Dependencies.Devices = h.extractDevicesFromSlice(automation.Config.Actions)

	// Generate summary
	deps.Summary = h.generateDependencySummary(deps)

	return deps, nil
}

func (h *AnalysisHandlers) getScriptDependencies(ctx context.Context, client homeassistant.Client, entityID string) (*EntityDependencies, error) {
	scriptID := strings.TrimPrefix(entityID, "script.")
	state, err := client.GetState(ctx, "script."+scriptID)
	if err != nil {
		return nil, err
	}

	friendlyName := ""
	if fn, ok := state.Attributes["friendly_name"].(string); ok {
		friendlyName = fn
	}

	deps := &EntityDependencies{
		EntityID:     entityID,
		FriendlyName: friendlyName,
		Type:         "script",
		Dependencies: &DependencyCategories{},
	}

	// Scripts only have actions (sequence)
	if sequence, ok := state.Attributes["sequence"].([]any); ok {
		deps.Dependencies.Actions = h.extractDependenciesFromSlice(sequence, "action")
		deps.Dependencies.Services = h.extractServicesFromSlice(sequence)
		deps.Dependencies.Areas = h.extractAreasFromSlice(sequence)
		deps.Dependencies.Devices = h.extractDevicesFromSlice(sequence)
	}

	deps.Summary = h.generateDependencySummary(deps)

	return deps, nil
}

// findEntityUsageInAutomation determines where an entity is used in an automation.
func (h *AnalysisHandlers) findEntityUsageInAutomation(config *homeassistant.AutomationConfig, entityID string) []string {
	var usedIn []string

	if searchInConfigSlice(config.Triggers, entityID) {
		usedIn = append(usedIn, "trigger")
	}
	if searchInConfigSlice(config.Conditions, entityID) {
		usedIn = append(usedIn, "condition")
	}
	if searchInConfigSlice(config.Actions, entityID) {
		usedIn = append(usedIn, "action")
	}

	return usedIn
}

// extractDependenciesFromSlice extracts entity dependencies from a config slice.
func (h *AnalysisHandlers) extractDependenciesFromSlice(items []any, _ string) []DependencyEntry {
	seen := make(map[string]DependencyEntry)

	for _, item := range items {
		h.extractDependenciesRecursive(item, seen)
	}

	return h.dependenciesToSortedSlice(seen)
}

// dependenciesToSortedSlice converts a dependency map to a sorted slice.
func (h *AnalysisHandlers) dependenciesToSortedSlice(seen map[string]DependencyEntry) []DependencyEntry {
	result := make([]DependencyEntry, 0, len(seen))
	for _, dep := range seen {
		result = append(result, dep)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].EntityID < result[j].EntityID
	})
	return result
}

// extractDependenciesRecursive traverses a value recursively and extracts dependencies.
func (h *AnalysisHandlers) extractDependenciesRecursive(val any, seen map[string]DependencyEntry) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []any:
		h.extractDependenciesFromSliceRecursive(v, seen)
	case map[string]any:
		h.extractDependenciesFromMap(v, seen)
	}
}

// extractDependenciesFromSliceRecursive processes a slice of items recursively.
func (h *AnalysisHandlers) extractDependenciesFromSliceRecursive(items []any, seen map[string]DependencyEntry) {
	for _, item := range items {
		h.extractDependenciesRecursive(item, seen)
	}
}

// extractDependenciesFromMap extracts dependencies from a map structure.
func (h *AnalysisHandlers) extractDependenciesFromMap(m map[string]any, seen map[string]DependencyEntry) {
	h.extractDirectEntityDependency(m, seen)
	h.extractTargetEntityDependency(m, seen)
	h.recurseIntoNestedStructures(m, seen)
}

// extractDirectEntityDependency extracts entity_id directly from a map.
func (h *AnalysisHandlers) extractDirectEntityDependency(m map[string]any, seen map[string]DependencyEntry) {
	entityID := h.extractEntityID(m)
	if entityID == "" {
		return
	}

	if _, exists := seen[entityID]; exists {
		return
	}

	triggerType := h.extractTriggerType(m)
	seen[entityID] = DependencyEntry{
		EntityID:    entityID,
		Type:        triggerType,
		Description: h.generateTriggerDescription(m, triggerType),
	}
}

// extractTargetEntityDependency extracts entity_id from the target field.
func (h *AnalysisHandlers) extractTargetEntityDependency(m map[string]any, seen map[string]DependencyEntry) {
	target, ok := m["target"].(map[string]any)
	if !ok {
		return
	}

	entityID := h.extractEntityID(target)
	if entityID == "" {
		return
	}

	if _, exists := seen[entityID]; exists {
		return
	}

	seen[entityID] = DependencyEntry{
		EntityID:    entityID,
		Type:        "target",
		Description: "Action target",
	}
}

// recurseIntoNestedStructures recurses into nested structures for dependency extraction.
func (h *AnalysisHandlers) recurseIntoNestedStructures(m map[string]any, seen map[string]DependencyEntry) {
	for key, subval := range m {
		if h.shouldRecurseIntoKey(key) {
			h.extractDependenciesRecursive(subval, seen)
		}
	}
}

// shouldRecurseIntoKey determines if a key should be recursively searched for dependencies.
func (h *AnalysisHandlers) shouldRecurseIntoKey(key string) bool {
	recursiveKeys := map[string]bool{
		"data":       true,
		"choose":     true,
		"sequence":   true,
		"conditions": true,
		"then":       true,
		"else":       true,
		"default":    true,
	}
	return recursiveKeys[key]
}

func (h *AnalysisHandlers) extractEntityID(m map[string]any) string {
	if entityID, ok := m["entity_id"].(string); ok {
		return entityID
	}
	if entityIDs, ok := m["entity_id"].([]any); ok && len(entityIDs) > 0 {
		if first, ok := entityIDs[0].(string); ok {
			return first
		}
	}
	return ""
}

func (h *AnalysisHandlers) extractTriggerType(m map[string]any) string {
	if t, ok := m["trigger"].(string); ok {
		return t
	}
	if t, ok := m["platform"].(string); ok {
		return t
	}
	if _, ok := m["condition"]; ok {
		return "condition"
	}
	if _, ok := m["action"].(string); ok {
		return "action"
	}
	if _, ok := m["service"].(string); ok {
		return "service_call"
	}
	return ""
}

func (h *AnalysisHandlers) generateTriggerDescription(m map[string]any, triggerType string) string {
	switch triggerType {
	case "state":
		to := ""
		from := ""
		if t, ok := m["to"].(string); ok {
			to = t
		}
		if f, ok := m["from"].(string); ok {
			from = f
		}
		if from != "" && to != "" {
			return fmt.Sprintf("State change from '%s' to '%s'", from, to)
		} else if to != "" {
			return fmt.Sprintf("State changes to '%s'", to)
		}
		return "State trigger"
	case "numeric_state":
		if above, ok := m["above"]; ok {
			return fmt.Sprintf("Numeric state above %v", above)
		}
		if below, ok := m["below"]; ok {
			return fmt.Sprintf("Numeric state below %v", below)
		}
		return "Numeric state trigger"
	case "time":
		if at, ok := m["at"].(string); ok {
			return fmt.Sprintf("At time %s", at)
		}
		return "Time trigger"
	case "condition":
		if condType, ok := m["condition"].(string); ok {
			return fmt.Sprintf("%s condition", condType)
		}
		return "Condition"
	default:
		return ""
	}
}

func (h *AnalysisHandlers) extractServicesFromSlice(items []any) []string {
	seen := make(map[string]bool)
	for _, item := range items {
		h.extractServicesRecursive(item, seen)
	}

	result := make([]string, 0, len(seen))
	for svc := range seen {
		result = append(result, svc)
	}
	sort.Strings(result)
	return result
}

func (h *AnalysisHandlers) extractServicesRecursive(val any, seen map[string]bool) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []any:
		for _, item := range v {
			h.extractServicesRecursive(item, seen)
		}
	case map[string]any:
		// Check for service or action key
		if svc, ok := v["service"].(string); ok {
			seen[svc] = true
		}
		if action, ok := v["action"].(string); ok {
			seen[action] = true
		}

		// Recurse
		for _, subval := range v {
			h.extractServicesRecursive(subval, seen)
		}
	}
}

func (h *AnalysisHandlers) extractAreasFromSlice(items []any) []string {
	seen := make(map[string]bool)
	for _, item := range items {
		h.extractAreasRecursive(item, seen)
	}

	result := make([]string, 0, len(seen))
	for area := range seen {
		result = append(result, area)
	}
	sort.Strings(result)
	return result
}

func (h *AnalysisHandlers) extractAreasRecursive(val any, seen map[string]bool) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []any:
		for _, item := range v {
			h.extractAreasRecursive(item, seen)
		}
	case map[string]any:
		// Check for area_id
		if areaID, ok := v["area_id"].(string); ok {
			seen[areaID] = true
		}
		if areaIDs, ok := v["area_id"].([]any); ok {
			for _, a := range areaIDs {
				if aStr, ok := a.(string); ok {
					seen[aStr] = true
				}
			}
		}
		// Check target.area_id
		if target, ok := v["target"].(map[string]any); ok {
			if areaID, ok := target["area_id"].(string); ok {
				seen[areaID] = true
			}
		}

		// Recurse
		for _, subval := range v {
			h.extractAreasRecursive(subval, seen)
		}
	}
}

func (h *AnalysisHandlers) extractDevicesFromSlice(items []any) []string {
	seen := make(map[string]bool)
	for _, item := range items {
		h.extractDevicesRecursive(item, seen)
	}

	result := make([]string, 0, len(seen))
	for device := range seen {
		result = append(result, device)
	}
	sort.Strings(result)
	return result
}

func (h *AnalysisHandlers) extractDevicesRecursive(val any, seen map[string]bool) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []any:
		for _, item := range v {
			h.extractDevicesRecursive(item, seen)
		}
	case map[string]any:
		// Check for device_id
		if deviceID, ok := v["device_id"].(string); ok {
			seen[deviceID] = true
		}
		if deviceIDs, ok := v["device_id"].([]any); ok {
			for _, d := range deviceIDs {
				if dStr, ok := d.(string); ok {
					seen[dStr] = true
				}
			}
		}
		// Check target.device_id
		if target, ok := v["target"].(map[string]any); ok {
			if deviceID, ok := target["device_id"].(string); ok {
				seen[deviceID] = true
			}
		}

		// Recurse
		for _, subval := range v {
			h.extractDevicesRecursive(subval, seen)
		}
	}
}

func (h *AnalysisHandlers) generateEntitySummary(analysis *EntityAnalysis) string {
	var parts []string

	// Entity info
	name := analysis.EntityID
	if analysis.FriendlyName != "" {
		name = analysis.FriendlyName
	}
	parts = append(parts, fmt.Sprintf("'%s' (%s) is currently %s.", name, analysis.Domain, analysis.State))

	// References
	if analysis.References.TotalReferences == 0 {
		parts = append(parts, "This entity is not referenced by any automations, scripts, or scenes.")
	} else {
		refParts := []string{}
		if len(analysis.References.Automations) > 0 {
			refParts = append(refParts, fmt.Sprintf("%d automation(s)", len(analysis.References.Automations)))
		}
		if len(analysis.References.Scripts) > 0 {
			refParts = append(refParts, fmt.Sprintf("%d script(s)", len(analysis.References.Scripts)))
		}
		if len(analysis.References.Scenes) > 0 {
			refParts = append(refParts, fmt.Sprintf("%d scene(s)", len(analysis.References.Scenes)))
		}
		if len(analysis.References.Groups) > 0 {
			refParts = append(refParts, fmt.Sprintf("%d group(s)", len(analysis.References.Groups)))
		}
		parts = append(parts, fmt.Sprintf("Referenced by %s.", strings.Join(refParts, ", ")))
	}

	// Automation details
	for _, auto := range analysis.References.Automations {
		usedInStr := strings.Join(auto.UsedIn, ", ")
		autoName := auto.Alias
		if autoName == "" {
			autoName = auto.EntityID
		}
		parts = append(parts, fmt.Sprintf("- Automation '%s' uses it in: %s", autoName, usedInStr))
	}

	return strings.Join(parts, " ")
}

func (h *AnalysisHandlers) generateDependencySummary(deps *EntityDependencies) string {
	var parts []string

	name := deps.EntityID
	if deps.FriendlyName != "" {
		name = deps.FriendlyName
	}
	parts = append(parts, fmt.Sprintf("'%s' (%s) dependencies:", name, deps.Type))

	if len(deps.Dependencies.Triggers) > 0 {
		parts = append(parts, fmt.Sprintf("- %d trigger entity/entities", len(deps.Dependencies.Triggers)))
	}
	if len(deps.Dependencies.Conditions) > 0 {
		parts = append(parts, fmt.Sprintf("- %d condition entity/entities", len(deps.Dependencies.Conditions)))
	}
	if len(deps.Dependencies.Actions) > 0 {
		parts = append(parts, fmt.Sprintf("- %d action target(s)", len(deps.Dependencies.Actions)))
	}
	if len(deps.Dependencies.Services) > 0 {
		parts = append(parts, fmt.Sprintf("- Services: %s", strings.Join(deps.Dependencies.Services, ", ")))
	}
	if len(deps.Dependencies.Areas) > 0 {
		parts = append(parts, fmt.Sprintf("- Areas: %s", strings.Join(deps.Dependencies.Areas, ", ")))
	}

	return strings.Join(parts, " ")
}

// RegisterAnalysisTools registers all analysis-related tools with the registry.
func RegisterAnalysisTools(registry *mcp.Registry) {
	h := NewAnalysisHandlers()
	h.RegisterTools(registry)
}

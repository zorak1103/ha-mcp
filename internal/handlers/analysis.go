// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	usedInAction    = "action"
	triggerTypeTime = "time"
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
		Description: "Analyze an entity and find all automations, scripts, scenes, dashboards, and template-helper templates that reference it. Returns a comprehensive overview including registry metadata (platform, area, device, labels) and how the entity is controlled and used in Home Assistant. The response's references.scanned_sources field lists exactly which config types were searched, so a \"no references found\" result can be trusted.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for analyzing an entity",
			Properties: map[string]mcp.JSONSchema{
				attrEntityID: {
					Type:        "string",
					Description: "The entity ID to analyze (e.g., 'light.living_room', 'sensor.temperature')",
				},
				"include_history": {
					Type:        "boolean",
					Description: "If true, include recent state history (last 24 hours). Default: false",
				},
				"format": {
					Type:        "string",
					Enum:        []string{"natural", formatJSON},
					Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, include per-reference config excerpts showing how the entity is used (trigger state values, condition states, action services). Default: false",
				},
			},
			Required: []string{attrEntityID},
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
				attrEntityID: {
					Type:        "string",
					Description: "The automation or script entity ID (e.g., 'automation.my_automation', 'script.my_script')",
				},
				"format": {
					Type:        "string",
					Enum:        []string{"natural", formatJSON},
					Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				},
			},
			Required: []string{attrEntityID},
		},
	}
}

// RegistryInfo holds entity registry metadata for an entity.
type RegistryInfo struct {
	Platform      string   `json:"platform,omitempty"`
	AreaID        string   `json:"area_id,omitempty"`
	AreaName      string   `json:"area_name,omitempty"`
	DeviceID      string   `json:"device_id,omitempty"`
	DeviceName    string   `json:"device_name,omitempty"`
	Manufacturer  string   `json:"manufacturer,omitempty"`
	Model         string   `json:"model,omitempty"`
	ConfigEntryID string   `json:"config_entry_id,omitempty"`
	DisabledBy    string   `json:"disabled_by,omitempty"`
	HiddenBy      string   `json:"hidden_by,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
}

// EntityAnalysis represents the comprehensive analysis of an entity.
type EntityAnalysis struct {
	EntityID     string            `json:"entity_id"`
	State        string            `json:"state"`
	FriendlyName string            `json:"friendly_name,omitempty"`
	Domain       string            `json:"domain"`
	Attributes   map[string]any    `json:"attributes,omitempty"`
	LastChanged  string            `json:"last_changed,omitempty"`
	Registry     *RegistryInfo     `json:"registry,omitempty"`
	References   *EntityReferences `json:"references"`
	Summary      string            `json:"summary"`
	History      []HistoryEntry    `json:"history,omitempty"`
}

// EntityReferences contains all automations, scripts, and scenes referencing an entity.
type EntityReferences struct {
	Automations     []AutomationReference     `json:"automations,omitempty"`
	Scripts         []ScriptReference         `json:"scripts,omitempty"`
	Scenes          []SceneReference          `json:"scenes,omitempty"`
	Groups          []string                  `json:"groups,omitempty"`
	AreaReferences  []AreaReference           `json:"area_references,omitempty"`
	Dashboards      []DashboardReference      `json:"dashboards,omitempty"`
	HelperTemplates []HelperTemplateReference `json:"helper_templates,omitempty"`
	TotalReferences int                       `json:"total_references"`
	ScannedSources  []string                  `json:"scanned_sources"`
	FailedSources   []string                  `json:"failed_sources,omitempty"`
}

// DashboardReference describes how a dashboard references an entity, directly
// as a card/chip "entity" field or embedded in a card's Jinja template text.
type DashboardReference struct {
	URLPath string          `json:"url_path"`
	Paths   []ReferencePath `json:"paths,omitempty"`
}

// HelperTemplateReference describes how a template-helper's Jinja template(s)
// reference an entity.
type HelperTemplateReference struct {
	EntityID string   `json:"entity_id"`
	Fields   []string `json:"fields"` // "state", "availability"
}

// AreaReference describes how an automation/script references an entity via its area.
type AreaReference struct {
	EntityID string   `json:"entity_id"`
	Alias    string   `json:"alias,omitempty"`
	Type     string   `json:"type"` // "automation" or "script"
	AreaID   string   `json:"area_id"`
	UsedIn   []string `json:"used_in"` // "trigger", "condition", "action"
}

// UsageExcerpt describes how an entity is referenced in a specific config node.
type UsageExcerpt struct {
	Section string `json:"section"` // "trigger", "condition", "action"
	Summary string `json:"summary"` // compact one-line description
}

// AutomationReference describes how an automation references an entity.
type AutomationReference struct {
	EntityID      string          `json:"entity_id"`
	Alias         string          `json:"alias,omitempty"`
	State         string          `json:"state"`
	LastTriggered string          `json:"last_triggered,omitempty"`
	UsedIn        []string        `json:"used_in"` // "trigger", "condition", "action"
	Paths         []ReferencePath `json:"paths,omitempty"`
	Excerpts      []UsageExcerpt  `json:"excerpts,omitempty"`
}

// ScriptReference describes how a script references an entity.
type ScriptReference struct {
	EntityID     string          `json:"entity_id"`
	FriendlyName string          `json:"friendly_name,omitempty"`
	UsedIn       string          `json:"used_in"` // "action"
	Paths        []ReferencePath `json:"paths,omitempty"`
	Excerpts     []UsageExcerpt  `json:"excerpts,omitempty"`
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
	entityID, ok := args[attrEntityID].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	includeHistory, _ := args["include_history"].(bool)
	verbose, _ := args["verbose"].(bool)

	analysis, err := h.buildEntityAnalysis(ctx, client, entityID, includeHistory, verbose)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(err.Error())},
			IsError: true,
		}, nil
	}

	format := formatter.ParseFormat(getStringArg(args, "format"))
	if format == formatter.FormatNatural {
		return successResult(h.formatAnalysisNatural(analysis, verbose)), nil
	}

	// JSON format
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

func (h *AnalysisHandlers) getEntityState(ctx context.Context, snapshot *AnalysisSnapshot, client homeassistant.Client, entityID string) (*homeassistant.Entity, error) {
	if entity := snapshot.FindEntityByID(entityID); entity != nil {
		return entity, nil
	}
	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("error getting entity state: %w", err)
	}
	return state, nil
}

func (h *AnalysisHandlers) buildEntityAnalysis(ctx context.Context, client homeassistant.Client, entityID string, includeHistory, verbose bool) (*EntityAnalysis, error) {
	// Create snapshot of all required data in parallel for better performance
	snapshot := CreateAnalysisSnapshot(ctx, client)

	state, err := h.getEntityState(ctx, snapshot, client, entityID)
	if err != nil {
		return nil, err
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
	analysis.Registry = h.extractRegistryInfo(snapshot, entityID)

	// Find all references. groups/areas are sourced from the pre-existing
	// AnalysisSnapshot, which has no per-field error tracking of its own, so
	// they're not included in outcomes below - see the plan's Global Constraints.
	outcomes := []ScanOutcome{
		{Source: "automations", Err: h.findAutomationReferences(ctx, client, entityID, analysis.References, verbose)},
		{Source: "scripts", Err: h.findScriptReferences(ctx, client, entityID, analysis.References, verbose)},
		{Source: "scenes", Err: h.findSceneReferences(ctx, client, entityID, analysis.References)},
		{Source: "dashboards", Err: h.findDashboardReferences(ctx, client, entityID, analysis.References)},
		{Source: "helper_templates", Err: h.findHelperTemplateReferences(ctx, client, entityID, analysis.References)},
	}
	h.findGroupReferencesWithSnapshot(snapshot, entityID, analysis.References)

	// Find area-based references using snapshot (entity controlled via area_id in automations/scripts)
	h.findAreaReferencesWithSnapshot(ctx, client, snapshot, entityID, analysis.References)

	scanned, failed := splitScanOutcomes(outcomes)
	scanned = append(scanned, "groups", "areas")
	analysis.References.ScannedSources = scanned
	analysis.References.FailedSources = failed

	// Calculate total references
	analysis.References.TotalReferences = len(analysis.References.Automations) +
		len(analysis.References.Scripts) +
		len(analysis.References.Scenes) +
		len(analysis.References.Groups) +
		len(analysis.References.AreaReferences) +
		len(analysis.References.Dashboards) +
		len(analysis.References.HelperTemplates)

	// Include history if requested
	if includeHistory {
		analysis.History = h.getEntityHistory(ctx, client, entityID)
	}

	analysis.Summary = h.generateEntitySummary(analysis)

	return analysis, nil
}

func (h *AnalysisHandlers) findAutomationReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences, verbose bool) error {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return err
	}

	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		fullAuto, getErr := client.GetAutomation(ctx, autoID)
		if getErr != nil || fullAuto.Config == nil {
			continue
		}

		usedIn := h.findEntityUsageInAutomation(fullAuto.Config, entityID)
		if len(usedIn) > 0 {
			ref := AutomationReference{
				EntityID:      auto.EntityID,
				Alias:         auto.FriendlyName,
				State:         auto.State,
				LastTriggered: auto.LastTriggered,
				UsedIn:        usedIn,
			}
			ref.Paths = collectAutomationReferencePaths(fullAuto.Config, entityID)
			if verbose {
				ref.Excerpts = collectEntityExcerpts(fullAuto.Config, entityID)
			}
			refs.Automations = append(refs.Automations, ref)
		}
	}
	return nil
}

func (h *AnalysisHandlers) findScriptReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences, verbose bool) error {
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return err
	}

	for _, script := range scripts {
		full, getErr := client.GetScript(ctx, script.EntityID)
		if getErr != nil || full.Config == nil {
			continue
		}
		sequence := full.Config.Sequence
		if !searchInConfigSlice(sequence, entityID) {
			continue
		}

		ref := ScriptReference{
			EntityID:     script.EntityID,
			FriendlyName: full.FriendlyName,
			UsedIn:       usedInAction,
		}
		ref.Paths = collectSectionReferencePaths(sequence, "sequence", "action", entityID)
		if verbose {
			ref.Excerpts = collectSequenceExcerpts(sequence, entityID)
		}
		refs.Scripts = append(refs.Scripts, ref)
	}
	return nil
}

func (h *AnalysisHandlers) findSceneReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) error {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return err
	}

	for _, scene := range scenes {
		entities, ok := scene.Attributes[attrEntityID].([]any)
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
	return nil
}

// findDashboardReferences scans every dashboard (including the default one) for
// entity references, both as a direct card/chip "entity" field and embedded in
// a card's Jinja template text (e.g. an icon_color template calling
// states('entity_id')). Dashboards were previously not scanned at all, so an
// entity referenced only this way was invisible to analyze_entity.
func (h *AnalysisHandlers) findDashboardReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) error {
	urlPaths, err := allDashboardURLPaths(ctx, client)
	if err != nil {
		return err
	}

	match := func(s string) bool { return strings.Contains(s, entityID) }
	for _, urlPath := range urlPaths {
		config, err := client.GetLovelaceConfig(ctx, urlPath)
		if err != nil {
			continue
		}
		hits := scanDashboardConfig(urlPath, config, match)
		if len(hits) == 0 {
			continue
		}
		paths := make([]ReferencePath, 0, len(hits))
		for _, hit := range hits {
			paths = append(paths, ReferencePath{Path: hit.Path, Context: hit.Context})
		}
		refs.Dashboards = append(refs.Dashboards, DashboardReference{URLPath: urlPath, Paths: paths})
	}
	return nil
}

// findHelperTemplateReferences scans template-helper state/availability Jinja
// templates for entity references. These were previously unscanned, so an
// entity referenced only inside a helper template's Jinja was invisible to
// analyze_entity.
func (h *AnalysisHandlers) findHelperTemplateReferences(ctx context.Context, client homeassistant.Client, entityID string, refs *EntityReferences) error {
	match := func(s string) bool { return strings.Contains(s, entityID) }
	hits, err := scanHelperTemplates(ctx, client, match)
	if err != nil {
		return err
	}

	fieldsByEntity := make(map[string][]string)
	var order []string
	for _, hit := range hits {
		if _, seen := fieldsByEntity[hit.ObjectID]; !seen {
			order = append(order, hit.ObjectID)
		}
		fieldsByEntity[hit.ObjectID] = append(fieldsByEntity[hit.ObjectID], hit.Context)
	}
	for _, id := range order {
		refs.HelperTemplates = append(refs.HelperTemplates, HelperTemplateReference{
			EntityID: id,
			Fields:   fieldsByEntity[id],
		})
	}
	return nil
}

// findGroupReferencesWithSnapshot finds groups that contain the entity using pre-fetched data.
func (h *AnalysisHandlers) findGroupReferencesWithSnapshot(snapshot *AnalysisSnapshot, entityID string, refs *EntityReferences) {
	if snapshot.AllStates == nil {
		return
	}

	for _, s := range snapshot.AllStates {
		if !strings.HasPrefix(s.EntityID, "group.") {
			continue
		}

		entities, ok := s.Attributes[attrEntityID].([]any)
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

// findAreaReferencesWithSnapshot finds automations and scripts that reference the entity's area using pre-fetched data.
func (h *AnalysisHandlers) findAreaReferencesWithSnapshot(ctx context.Context, client homeassistant.Client, snapshot *AnalysisSnapshot, entityID string, refs *EntityReferences) {
	// Get entity's area from snapshot
	entityArea := snapshot.GetEntityArea(entityID)
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
				Type:     traceDomainAutomation,
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
				Type:     traceDomainScript,
				AreaID:   entityArea,
				UsedIn:   []string{usedInAction},
			})
		}
	}
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
		usedIn = append(usedIn, usedInAction)
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
	entityID, ok := args[attrEntityID].(string)
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

	format := formatter.ParseFormat(getStringArg(args, "format"))
	if format == formatter.FormatNatural {
		return successResult(h.formatDependenciesNatural(deps)), nil
	}

	// JSON format
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
	deps.Dependencies.Actions = h.extractDependenciesFromSlice(automation.Config.Actions, usedInAction)

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
		deps.Dependencies.Actions = h.extractDependenciesFromSlice(sequence, usedInAction)
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
		usedIn = append(usedIn, usedInAction)
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
	slices.SortFunc(result, func(a, b DependencyEntry) int {
		return cmp.Compare(a.EntityID, b.EntityID)
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
		"data":               true,
		"choose":             true,
		"sequence":           true,
		targetInfoConditions: true,
		"then":               true,
		"else":               true,
		"default":            true,
	}
	return recursiveKeys[key]
}

func (h *AnalysisHandlers) extractEntityID(m map[string]any) string {
	if entityID, ok := m[attrEntityID].(string); ok {
		return entityID
	}
	if entityIDs, ok := m[attrEntityID].([]any); ok && len(entityIDs) > 0 {
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
		return excerptConditionState
	}
	if _, ok := m["action"].(string); ok {
		return usedInAction
	}
	if _, ok := m["service"].(string); ok {
		return "service_call"
	}
	return ""
}

func (h *AnalysisHandlers) generateTriggerDescription(m map[string]any, triggerType string) string {
	switch triggerType {
	case excerptTriggerState:
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
	case excerptTriggerNumericState:
		if above, ok := m["above"]; ok {
			return fmt.Sprintf("Numeric state above %v", above)
		}
		if below, ok := m["below"]; ok {
			return fmt.Sprintf("Numeric state below %v", below)
		}
		return "Numeric state trigger"
	case triggerTypeTime:
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

// extractRegistryInfo builds a RegistryInfo from the snapshot for a given entityID.
// Returns nil if the entity is not found in the entity registry.
func (h *AnalysisHandlers) extractRegistryInfo(snapshot *AnalysisSnapshot, entityID string) *RegistryInfo {
	entry := snapshot.FindEntityRegistryEntry(entityID)
	if entry == nil {
		return nil
	}

	reg := &RegistryInfo{
		Platform:      entry.Platform,
		DeviceID:      entry.DeviceID,
		ConfigEntryID: entry.ConfigEntryID,
		DisabledBy:    entry.DisabledBy,
		HiddenBy:      entry.HiddenBy,
		Icon:          entry.Icon,
		Labels:        entry.Labels,
		Aliases:       entry.Aliases,
	}

	// Resolve area: entity area takes precedence, then device area
	areaID := snapshot.GetEntityArea(entityID)
	if areaID != "" {
		reg.AreaID = areaID
		if area := snapshot.FindAreaByID(areaID); area != nil {
			reg.AreaName = area.Name
		}
	}

	// Resolve device details
	if entry.DeviceID != "" {
		if device := snapshot.FindDeviceByID(entry.DeviceID); device != nil {
			name := device.NameByUser
			if name == "" {
				name = device.Name
			}
			reg.DeviceName = name
			reg.Manufacturer = device.Manufacturer
			reg.Model = string(device.Model)
		}
	}

	return reg
}

// formatRegistry formats the registry section for natural language output.
func (h *AnalysisHandlers) formatRegistry(parts []string, reg *RegistryInfo) []string {
	parts = append(parts, "\nRegistry:")
	if reg.Platform != "" {
		parts = append(parts, fmt.Sprintf("- Platform: %s", reg.Platform))
	}
	if reg.AreaName != "" {
		parts = append(parts, fmt.Sprintf("- Area: %s (%s)", reg.AreaName, reg.AreaID))
	} else if reg.AreaID != "" {
		parts = append(parts, fmt.Sprintf("- Area: %s", reg.AreaID))
	}
	parts = h.formatDeviceDetails(parts, reg)
	if len(reg.Labels) > 0 {
		parts = append(parts, fmt.Sprintf("- Labels: %s", strings.Join(reg.Labels, ", ")))
	}
	if len(reg.Aliases) > 0 {
		parts = append(parts, fmt.Sprintf("- Aliases: %s", strings.Join(reg.Aliases, ", ")))
	}
	if reg.DisabledBy != "" {
		parts = append(parts, fmt.Sprintf("- Disabled by: %s", reg.DisabledBy))
	}
	if reg.HiddenBy != "" {
		parts = append(parts, fmt.Sprintf("- Hidden by: %s", reg.HiddenBy))
	}
	return parts
}

// formatDeviceDetails formats device name, manufacturer, and model lines.
func (h *AnalysisHandlers) formatDeviceDetails(parts []string, reg *RegistryInfo) []string {
	if reg.DeviceName == "" && reg.Manufacturer == "" && reg.Model == "" {
		return parts
	}
	device := reg.DeviceName
	if reg.Manufacturer != "" || reg.Model != "" {
		extra := strings.TrimSpace(reg.Manufacturer + " " + reg.Model)
		if extra != "" && device != "" {
			device = fmt.Sprintf("%s [%s]", device, extra)
		} else if extra != "" {
			device = extra
		}
	}
	return append(parts, fmt.Sprintf("- Device: %s", device))
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
		parts = append(parts, fmt.Sprintf(
			"No references found (scanned: %s).",
			strings.Join(analysis.References.ScannedSources, ", "),
		))
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
		if len(analysis.References.Dashboards) > 0 {
			refParts = append(refParts, fmt.Sprintf("%d dashboard(s)", len(analysis.References.Dashboards)))
		}
		if len(analysis.References.HelperTemplates) > 0 {
			refParts = append(refParts, fmt.Sprintf("%d helper template(s)", len(analysis.References.HelperTemplates)))
		}
		parts = append(parts, fmt.Sprintf("Referenced by %s.", strings.Join(refParts, ", ")))
	}

	if len(analysis.References.FailedSources) > 0 {
		parts = append(parts, fmt.Sprintf(
			scanFailureWarningFormat,
			len(analysis.References.FailedSources), strings.Join(analysis.References.FailedSources, ", "),
		))
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

// formatAnalysisNatural formats an EntityAnalysis in natural language.
func (h *AnalysisHandlers) formatAnalysisNatural(analysis *EntityAnalysis, verbose bool) string {
	var parts []string

	// Entity info with friendly name
	name := analysis.FriendlyName
	if name == "" {
		name = analysis.EntityID
	}
	parts = append(parts,
		fmt.Sprintf("%s is %s", formatter.FormatNameWithID(name, analysis.EntityID), analysis.State),
		fmt.Sprintf("Domain: %s | Last changed: %s", analysis.Domain, analysis.LastChanged),
	)

	// Registry metadata
	if analysis.Registry != nil {
		parts = h.formatRegistry(parts, analysis.Registry)
	}

	// References section
	if analysis.References.TotalReferences == 0 {
		parts = append(parts, fmt.Sprintf(
			"\nNo references found (scanned: %s).",
			strings.Join(analysis.References.ScannedSources, ", "),
		))
	} else {
		parts = h.formatReferences(parts, analysis.References, verbose)
	}

	if len(analysis.References.FailedSources) > 0 {
		parts = append(parts, fmt.Sprintf(
			scanFailureWarningFormat,
			len(analysis.References.FailedSources), strings.Join(analysis.References.FailedSources, ", "),
		))
	}

	// History
	if len(analysis.History) > 0 {
		parts = h.formatHistory(parts, analysis.History)
	}

	return strings.Join(parts, "\n")
}

func (h *AnalysisHandlers) formatReferences(parts []string, refs *EntityReferences, verbose bool) []string {
	parts = append(parts, fmt.Sprintf("\nReferences (%d total):", refs.TotalReferences))
	parts = h.formatAutomationRefs(parts, refs.Automations, verbose)
	parts = h.formatScriptRefs(parts, refs.Scripts, verbose)

	// Scenes
	if len(refs.Scenes) > 0 {
		parts = append(parts, fmt.Sprintf("- %d scenes:", len(refs.Scenes)))
		for _, scene := range refs.Scenes {
			parts = append(parts, fmt.Sprintf("  • %s", scene.EntityID))
		}
	}

	// Groups
	if len(refs.Groups) > 0 {
		parts = append(parts, fmt.Sprintf("- %d groups:", len(refs.Groups)))
		for _, group := range refs.Groups {
			parts = append(parts, fmt.Sprintf("  • %s", group))
		}
	}

	// Area references
	if len(refs.AreaReferences) > 0 {
		parts = append(parts, fmt.Sprintf("- Area references: %d", len(refs.AreaReferences)))
	}

	// Dashboards
	if len(refs.Dashboards) > 0 {
		parts = append(parts, fmt.Sprintf("- %d dashboard(s):", len(refs.Dashboards)))
		for _, dash := range refs.Dashboards {
			label := dash.URLPath
			if label == "" {
				label = "default"
			}
			parts = append(parts, fmt.Sprintf("  • %s (%d location(s))", label, len(dash.Paths)))
		}
	}

	// Helper templates
	if len(refs.HelperTemplates) > 0 {
		parts = append(parts, fmt.Sprintf("- %d helper template(s):", len(refs.HelperTemplates)))
		for _, ht := range refs.HelperTemplates {
			parts = append(parts, fmt.Sprintf("  • %s (%s)", ht.EntityID, strings.Join(ht.Fields, ", ")))
		}
	}

	return parts
}

func (h *AnalysisHandlers) formatAutomationRefs(parts []string, autos []AutomationReference, verbose bool) []string {
	if len(autos) == 0 {
		return parts
	}
	parts = append(parts, fmt.Sprintf("- %d automations:", len(autos)))
	for _, auto := range autos {
		name := auto.Alias
		if name == "" {
			name = auto.EntityID
		}
		parts = append(parts, fmt.Sprintf("  • %s (%s, %s)", name, auto.State, strings.Join(auto.UsedIn, "+")))
		for _, rp := range auto.Paths {
			parts = append(parts, formatReferencePath(rp))
		}
		if verbose {
			for _, ex := range auto.Excerpts {
				parts = append(parts, fmt.Sprintf("    %s: %s", ex.Section, ex.Summary))
			}
		}
	}
	return parts
}

func (h *AnalysisHandlers) formatScriptRefs(parts []string, scripts []ScriptReference, verbose bool) []string {
	if len(scripts) == 0 {
		return parts
	}
	parts = append(parts, fmt.Sprintf("- %d scripts:", len(scripts)))
	for _, script := range scripts {
		name := script.FriendlyName
		if name == "" {
			name = script.EntityID
		}
		parts = append(parts, fmt.Sprintf("  • %s (%s)", name, script.UsedIn))
		for _, rp := range script.Paths {
			parts = append(parts, formatReferencePath(rp))
		}
		if verbose {
			for _, ex := range script.Excerpts {
				parts = append(parts, fmt.Sprintf("    %s: %s", ex.Section, ex.Summary))
			}
		}
	}
	return parts
}

// formatReferencePath formats a single ReferencePath as an indented line.
func formatReferencePath(rp ReferencePath) string {
	if rp.Context != "" {
		return fmt.Sprintf("    %s  (%s)", rp.Path, rp.Context)
	}
	return fmt.Sprintf("    %s", rp.Path)
}

func (h *AnalysisHandlers) formatHistory(parts []string, history []HistoryEntry) []string {
	totalCount := len(history)
	parts = append(parts, fmt.Sprintf("\nHistory (last 24h): %d state changes", totalCount))

	// Show up to 10 most recent changes
	const maxEntries = 10
	showCount := totalCount
	if showCount > maxEntries {
		showCount = maxEntries
	}

	// Start from the end to show newest entries first
	startIdx := totalCount - showCount
	for i := startIdx; i < totalCount; i++ {
		entry := history[i]
		parts = append(parts, fmt.Sprintf("- State: %s at %s", entry.State, entry.LastChanged))
	}

	// Add truncation message if we're showing a subset
	if totalCount > maxEntries {
		parts = append(parts, fmt.Sprintf("(Showing %d of %d changes)", showCount, totalCount))
	}

	return parts
}

// formatDependenciesNatural formats EntityDependencies in natural language.
func (h *AnalysisHandlers) formatDependenciesNatural(deps *EntityDependencies) string {
	var parts []string

	// Entity info
	name := deps.FriendlyName
	if name == "" {
		name = deps.EntityID
	}
	parts = append(parts, fmt.Sprintf("%s (%s) - %s", deps.EntityID, name, deps.Type))

	// Dependencies section
	totalDeps := len(deps.Dependencies.Triggers) + len(deps.Dependencies.Conditions) + len(deps.Dependencies.Actions)
	if totalDeps == 0 {
		parts = append(parts, "\nNo entity dependencies found.")
		return strings.Join(parts, "\n")
	}

	parts = append(parts, "\nDependencies:")

	// Triggers
	if len(deps.Dependencies.Triggers) > 0 {
		triggerIDs := make([]string, 0, len(deps.Dependencies.Triggers))
		for _, t := range deps.Dependencies.Triggers {
			triggerIDs = append(triggerIDs, t.EntityID)
		}
		parts = append(parts, fmt.Sprintf("- Triggers (%d): %s", len(deps.Dependencies.Triggers), strings.Join(triggerIDs, ", ")))
	}

	// Conditions
	if len(deps.Dependencies.Conditions) > 0 {
		conditionIDs := make([]string, 0, len(deps.Dependencies.Conditions))
		for _, c := range deps.Dependencies.Conditions {
			conditionIDs = append(conditionIDs, c.EntityID)
		}
		parts = append(parts, fmt.Sprintf("- Conditions (%d): %s", len(deps.Dependencies.Conditions), strings.Join(conditionIDs, ", ")))
	}

	// Actions
	if len(deps.Dependencies.Actions) > 0 {
		actionIDs := make([]string, 0, len(deps.Dependencies.Actions))
		for _, a := range deps.Dependencies.Actions {
			actionIDs = append(actionIDs, a.EntityID)
		}
		parts = append(parts, fmt.Sprintf("- Actions (%d): %s", len(deps.Dependencies.Actions), strings.Join(actionIDs, ", ")))
	}

	// Services
	if len(deps.Dependencies.Services) > 0 {
		parts = append(parts, fmt.Sprintf("- Services: %s", strings.Join(deps.Dependencies.Services, ", ")))
	}

	// Areas
	if len(deps.Dependencies.Areas) > 0 {
		parts = append(parts, fmt.Sprintf("- Areas: %s", strings.Join(deps.Dependencies.Areas, ", ")))
	}

	// Summary
	parts = append(parts, fmt.Sprintf("\nSummary: Uses %d entities across triggers, conditions, and actions.", totalDeps))

	return strings.Join(parts, "\n")
}

// RegisterAnalysisTools registers all analysis-related tools with the registry.
func RegisterAnalysisTools(registry *mcp.Registry) {
	h := NewAnalysisHandlers()
	h.RegisterTools(registry)
}

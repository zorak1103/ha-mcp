package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// AutomationDebugReport is the top-level consolidated debug report for an automation.
type AutomationDebugReport struct {
	Config        *AutomationDebugConfig `json:"config"`
	Trace         *AutomationDebugTrace  `json:"trace,omitempty"`
	TriggerStates []TriggerEntityState   `json:"trigger_states,omitempty"`
	Logbook       []DebugLogbookEntry    `json:"logbook,omitempty"`
	LogbookHours  float64                `json:"logbook_hours"`
}

// AutomationDebugConfig is a summary of automation configuration.
type AutomationDebugConfig struct {
	Name           string   `json:"name"`
	EntityID       string   `json:"entity_id"`
	State          string   `json:"state"`
	Mode           string   `json:"mode,omitempty"`
	LastTriggered  string   `json:"last_triggered,omitempty"`
	TriggerCount   int      `json:"trigger_count"`
	ConditionCount int      `json:"condition_count"`
	ActionCount    int      `json:"action_count"`
	TriggerTypes   []string `json:"trigger_types,omitempty"`
}

// AutomationDebugTrace is a simplified view of a single trace execution.
type AutomationDebugTrace struct {
	RunID     string  `json:"run_id"`
	State     string  `json:"state"`
	Timestamp string  `json:"timestamp"`
	Duration  float64 `json:"duration,omitempty"`
	Trigger   string  `json:"trigger,omitempty"`
}

// TriggerEntityState holds the current state of a trigger entity.
type TriggerEntityState struct {
	EntityID    string `json:"entity_id"`
	State       string `json:"state"`
	LastChanged string `json:"last_changed"`
	TriggerType string `json:"trigger_type"`
}

// DebugLogbookEntry is a simplified logbook entry for the debug report.
type DebugLogbookEntry struct {
	When    string `json:"when"`
	Name    string `json:"name"`
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// triggerEntityInfo holds entity ID and trigger platform type.
type triggerEntityInfo struct {
	entityID    string
	triggerType string
}

// handleDebugTrace handles the debug action: builds a consolidated automation debug report.
func (h *TraceHandlers) handleDebugTrace(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	automationID, _ := args["automation_id"].(string)
	if automationID == "" {
		return errorResult("automation_id is required for debug action"), nil
	}

	// Reject non-automation entity IDs (e.g., "light.living_room")
	if strings.Contains(automationID, ".") && !strings.HasPrefix(automationID, "automation.") {
		return errorResult(fmt.Sprintf("debug action only supports automations, got: %q", automationID)), nil
	}

	entityID, _ := normalizeAutomationID(automationID)

	hours := getHoursFromArgs(args)
	if hours <= 0 {
		hours = 6.0
	}

	report, err := h.buildDebugReport(ctx, client, entityID, hours)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build debug report: %v", err)), nil
	}

	if format == formatJSON {
		jsonData, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return errorResult(fmt.Sprintf("failed to marshal debug report: %v", marshalErr)), nil
		}
		return successResult(string(jsonData)), nil
	}

	return successResult(formatDebugNatural(report)), nil
}

// buildDebugReport orchestrates the 4 data fetches and assembles the debug report.
func (h *TraceHandlers) buildDebugReport(ctx context.Context, client homeassistant.Client, entityID string, hours float64) (*AutomationDebugReport, error) {
	cfg, triggers, err := fetchDebugAutomationConfig(ctx, client, entityID)
	if err != nil {
		return nil, err
	}

	latestTrace := h.fetchLatestTrace(ctx, client, entityID)
	triggerStates := fetchTriggerStates(ctx, client, triggers)
	logbook := fetchDebugLogbook(ctx, client, entityID, hours)

	return &AutomationDebugReport{
		Config:        cfg,
		Trace:         latestTrace,
		TriggerStates: triggerStates,
		Logbook:       logbook,
		LogbookHours:  hours,
	}, nil
}

// fetchDebugAutomationConfig retrieves the automation config summary and raw triggers list.
func fetchDebugAutomationConfig(ctx context.Context, client homeassistant.Client, entityID string) (*AutomationDebugConfig, []any, error) {
	_, configID := normalizeAutomationID(entityID)
	auto, err := client.GetAutomation(ctx, configID)
	if err != nil {
		return nil, nil, fmt.Errorf("automation not found: %w", err)
	}

	// The HA config endpoint does not return runtime state; fetch it separately.
	// Best-effort: leave State empty on error rather than failing the whole report.
	runtimeState := auto.State
	if entity, stateErr := client.GetState(ctx, entityID); stateErr == nil {
		runtimeState = entity.State
	}

	cfg := &AutomationDebugConfig{
		EntityID:      entityID,
		State:         runtimeState,
		LastTriggered: auto.LastTriggered,
	}

	var triggers []any
	if auto.Config != nil {
		cfg.Name = auto.Config.Alias
		cfg.Mode = auto.Config.Mode
		cfg.TriggerCount = len(auto.Config.Triggers)
		cfg.ConditionCount = len(auto.Config.Conditions)
		cfg.ActionCount = len(auto.Config.Actions)
		cfg.TriggerTypes = extractTriggerTypes(auto.Config.Triggers)
		triggers = auto.Config.Triggers
	}

	if cfg.Name == "" {
		cfg.Name = auto.FriendlyName
	}
	if cfg.Name == "" {
		cfg.Name = entityID
	}

	return cfg, triggers, nil
}

// fetchLatestTrace retrieves the most recent trace for the automation (best-effort, nil on failure).
func (h *TraceHandlers) fetchLatestTrace(ctx context.Context, client homeassistant.Client, entityID string) *AutomationDebugTrace {
	data := map[string]any{
		"domain":  traceDomainAutomation,
		"item_id": entityID,
	}
	response, err := client.SendHACSCommand(ctx, "trace/list", data)
	if err != nil {
		return nil
	}

	traces, _ := response.([]any)
	if len(traces) == 0 {
		return nil
	}

	// Find the trace with the most recent timestamp (ISO timestamps sort lexicographically)
	var latest map[string]any
	var latestTS string
	for _, t := range traces {
		traceMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		ts := getMapString(traceMap, "timestamp", "")
		if latest == nil || ts > latestTS {
			latest = traceMap
			latestTS = ts
		}
	}
	if latest == nil {
		return nil
	}

	return buildDebugTrace(latest)
}

// buildDebugTrace converts a raw trace map to an AutomationDebugTrace.
func buildDebugTrace(latest map[string]any) *AutomationDebugTrace {
	t := &AutomationDebugTrace{
		RunID:     getMapString(latest, "run_id", ""),
		State:     getMapString(latest, "state", ""),
		Timestamp: getMapString(latest, "timestamp", ""),
	}
	if d, ok := latest["duration"].(float64); ok {
		t.Duration = d
	}
	switch v := latest["trigger"].(type) {
	case map[string]any:
		t.Trigger = getMapString(v, "description", "")
		if t.Trigger == "" {
			t.Trigger = getMapString(v, "platform", "")
		}
	case string:
		t.Trigger = v
	}
	return t
}

// fetchTriggerStates retrieves the current state of each entity-based trigger (best-effort).
func fetchTriggerStates(ctx context.Context, client homeassistant.Client, triggers []any) []TriggerEntityState {
	infos := extractTriggerEntityIDs(triggers)
	if len(infos) == 0 {
		return nil
	}

	states := make([]TriggerEntityState, 0, len(infos))
	for _, info := range infos {
		entity, err := client.GetState(ctx, info.entityID)
		if err != nil {
			continue
		}
		lastChanged := ""
		if !entity.LastChanged.IsZero() {
			lastChanged = entity.LastChanged.Format(time.RFC3339)
		}
		states = append(states, TriggerEntityState{
			EntityID:    info.entityID,
			State:       entity.State,
			LastChanged: lastChanged,
			TriggerType: info.triggerType,
		})
	}
	return states
}

// fetchDebugLogbook retrieves logbook entries for the automation (best-effort, max 50 entries).
func fetchDebugLogbook(ctx context.Context, client homeassistant.Client, entityID string, hours float64) []DebugLogbookEntry {
	const maxLogbookEntries = 50
	now := time.Now()
	startTime := now.Add(-time.Duration(hours) * time.Hour)

	entries, err := client.GetLogbook(ctx, startTime.Format(time.RFC3339), now.Format(time.RFC3339), entityID)
	if err != nil {
		return nil
	}

	if len(entries) > maxLogbookEntries {
		entries = entries[len(entries)-maxLogbookEntries:]
	}

	result := make([]DebugLogbookEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, DebugLogbookEntry{
			When:    e.When,
			Name:    e.Name,
			State:   e.State,
			Message: e.Message,
		})
	}
	return result
}

// extractTriggerEntityIDs walks the trigger slice and extracts entity_id + trigger platform.
// Deduplicates entity IDs to avoid redundant GetState calls.
func extractTriggerEntityIDs(triggers []any) []triggerEntityInfo {
	seen := make(map[string]bool)
	var result []triggerEntityInfo

	for _, t := range triggers {
		tMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		triggerType := getMapString(tMap, "platform", "")
		entityID := getMapString(tMap, configKeyEntityID, "")
		if entityID != "" && !seen[entityID] {
			seen[entityID] = true
			result = append(result, triggerEntityInfo{
				entityID:    entityID,
				triggerType: triggerType,
			})
		}
	}
	return result
}

// extractTriggerTypes returns deduplicated list of trigger platform types.
func extractTriggerTypes(triggers []any) []string {
	seen := make(map[string]bool)
	var types []string

	for _, t := range triggers {
		tMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		platform := getMapString(tMap, "platform", "")
		if platform != "" && !seen[platform] {
			seen[platform] = true
			types = append(types, platform)
		}
	}
	return types
}

// formatDebugNatural formats the debug report as a multi-section natural language string.
func formatDebugNatural(report *AutomationDebugReport) string {
	parts := []string{
		"=== Automation Debug Report ===",
		formatDebugConfigSection(report.Config),
		formatDebugTraceSection(report.Trace),
		formatDebugTriggerStatesSection(report.TriggerStates),
		formatDebugLogbookSection(report.Logbook, report.LogbookHours),
	}
	return strings.Join(parts, "\n\n")
}

// formatDebugConfigSection formats the automation configuration summary.
func formatDebugConfigSection(cfg *AutomationDebugConfig) string {
	if cfg == nil {
		return "Automation: (not found)"
	}
	name := cfg.Name
	if name == "" {
		name = cfg.EntityID
	}

	lines := []string{fmt.Sprintf("Automation: %s (%s)", name, cfg.EntityID)}

	stateLine := "State: " + cfg.State
	if cfg.Mode != "" {
		stateLine += " | Mode: " + cfg.Mode
	}
	if cfg.LastTriggered != "" {
		stateLine += " | Last triggered: " + cfg.LastTriggered
	}
	lines = append(lines, stateLine, fmt.Sprintf("Config: %d triggers, %d conditions, %d actions",
		cfg.TriggerCount, cfg.ConditionCount, cfg.ActionCount))

	if len(cfg.TriggerTypes) > 0 {
		lines = append(lines, "Trigger types: "+strings.Join(cfg.TriggerTypes, ", "))
	}
	return strings.Join(lines, "\n")
}

// formatDebugTraceSection formats the latest trace section.
func formatDebugTraceSection(trace *AutomationDebugTrace) string {
	lines := []string{"--- Latest Trace ---"}
	if trace == nil {
		lines = append(lines, "No traces found.")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "Run ID: "+trace.RunID)

	stateLine := "State: " + trace.State
	if trace.Duration > 0 {
		stateLine += fmt.Sprintf(" | Duration: %.2fs", trace.Duration)
	}
	if trace.Timestamp != "" {
		stateLine += " | Timestamp: " + trace.Timestamp
	}
	lines = append(lines, stateLine)

	if trace.Trigger != "" {
		lines = append(lines, "Trigger: "+trace.Trigger)
	}
	return strings.Join(lines, "\n")
}

// formatDebugTriggerStatesSection formats the trigger entity states section.
func formatDebugTriggerStatesSection(states []TriggerEntityState) string {
	lines := []string{"--- Trigger Entity States ---"}
	if len(states) == 0 {
		lines = append(lines, "No entity-based triggers found.")
		return strings.Join(lines, "\n")
	}

	for _, s := range states {
		line := fmt.Sprintf("- %s (%s): %s", s.EntityID, s.TriggerType, s.State)
		if s.LastChanged != "" {
			line += " (last changed: " + s.LastChanged + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// formatDebugLogbookSection formats the logbook section.
func formatDebugLogbookSection(entries []DebugLogbookEntry, hours float64) string {
	lines := []string{fmt.Sprintf("--- Logbook (last %.0f hours) ---", hours)}
	if len(entries) == 0 {
		lines = append(lines, "No logbook entries found.")
		return strings.Join(lines, "\n")
	}

	for _, e := range entries {
		when := formatLogbookWhen(e.When)
		line := "- " + when + " | " + e.Name
		if e.State != "" {
			line += " | " + e.State
		} else if e.Message != "" {
			line += " | " + e.Message
		}
		lines = append(lines, line)
	}
	lines = append(lines, fmt.Sprintf("(%d entries)", len(entries)))
	return strings.Join(lines, "\n")
}

// formatLogbookWhen formats a logbook timestamp to HH:MM:SS if possible.
func formatLogbookWhen(when string) string {
	if t, err := time.Parse(time.RFC3339, when); err == nil {
		return t.Format("15:04:05")
	}
	return when
}

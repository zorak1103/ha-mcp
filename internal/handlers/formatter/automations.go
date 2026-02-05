package formatter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Message constants for automation formatter.
const (
	MsgNoAutomationsFound = "No automations found."
)

// State and mode constants.
const (
	stateOff   = "off"
	modeSingle = "single"
)

// AutomationListOptions configures automation list formatting.
type AutomationListOptions struct {
	Verbose bool
	Limit   int
}

// AutomationFormatter defines the interface for formatting automation responses.
type AutomationFormatter interface {
	// FormatList formats a list of automations.
	FormatList(ctx context.Context, automations []homeassistant.Automation,
		configs map[string]*homeassistant.AutomationConfig,
		opts AutomationListOptions) (string, error)

	// FormatDetail formats a single automation with full details.
	FormatDetail(ctx context.Context, automation homeassistant.Automation) (string, error)
}

// NewAutomationFormatter creates a new AutomationFormatter for the specified format.
func NewAutomationFormatter(format Format) AutomationFormatter {
	switch format {
	case FormatJSON:
		return NewJSONAutomationFormatter()
	case FormatNatural:
		return NewNaturalAutomationFormatter()
	default:
		return NewNaturalAutomationFormatter()
	}
}

// =============================================================================
// Natural Language Formatter
// =============================================================================

// NaturalAutomationFormatter produces human-readable automation output.
type NaturalAutomationFormatter struct {
	now time.Time
}

// NewNaturalAutomationFormatter creates a new NaturalAutomationFormatter.
func NewNaturalAutomationFormatter() *NaturalAutomationFormatter {
	return &NaturalAutomationFormatter{
		now: time.Now(),
	}
}

// FormatList formats a list of automations in natural language.
func (f *NaturalAutomationFormatter) FormatList(
	_ context.Context,
	automations []homeassistant.Automation,
	configs map[string]*homeassistant.AutomationConfig,
	opts AutomationListOptions,
) (string, error) {
	if len(automations) == 0 {
		return MsgNoAutomationsFound, nil
	}

	var result strings.Builder

	// Count enabled/disabled
	enabled, disabled := f.countByState(automations)

	// Summary line
	fmt.Fprintf(&result, "%d automations (%d enabled, %d disabled)\n\n", len(automations), enabled, disabled)

	// Mode breakdown
	modeCounts := f.countByMode(automations, configs)
	if len(modeCounts) > 0 {
		result.WriteString("By mode: ")
		result.WriteString(f.formatModeCounts(modeCounts))
		result.WriteString("\n\n")
	}

	// Recently triggered section (enabled automations)
	recentlyTriggered := f.getRecentlyTriggered(automations, 5)
	if len(recentlyTriggered) > 0 {
		result.WriteString("Recently triggered:\n")
		for _, auto := range recentlyTriggered {
			f.writeAutomationLine(&result, auto, configs, opts.Verbose)
		}
		result.WriteString("\n")
	}

	// Verbose: list all automations with details
	if opts.Verbose {
		result.WriteString("All automations:\n")
		for _, auto := range automations {
			f.writeAutomationLine(&result, auto, configs, true)
		}
	} else {
		// Non-verbose: show disabled section only
		disabledAutomations := f.getDisabled(automations)
		if len(disabledAutomations) > 0 {
			fmt.Fprintf(&result, "Disabled (%d):\n", len(disabledAutomations))
			for _, auto := range disabledAutomations {
				fmt.Fprintf(&result, "- %s\n", f.getDisplayName(auto))
			}
		}
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatDetail formats a single automation with full details.
func (f *NaturalAutomationFormatter) FormatDetail(
	_ context.Context,
	automation homeassistant.Automation,
) (string, error) {
	var result strings.Builder

	// Header: Name (state)
	name := f.getDisplayName(automation)
	state := "enabled"
	if automation.State == stateOff {
		state = "disabled"
	}
	fmt.Fprintf(&result, "Automation: %s (%s)\n", name, state)

	// Mode and last triggered
	if automation.Config != nil {
		mode := automation.Config.Mode
		if mode == "" {
			mode = modeSingle
		}
		result.WriteString("Mode: " + mode)
	}

	if automation.LastTriggered != "" {
		lastTime, err := time.Parse(time.RFC3339, automation.LastTriggered)
		if err == nil {
			result.WriteString(" | Last triggered: " + FormatTimeSince(lastTime, f.now))
		}
	}
	result.WriteString("\n")

	// Description
	if automation.Config != nil && automation.Config.Description != "" {
		result.WriteString("\n" + automation.Config.Description + "\n")
	}

	// Triggers
	if automation.Config != nil {
		result.WriteString("\n")
		f.writeTriggersSection(&result, automation.Config.Triggers)
	}

	// Conditions
	if automation.Config != nil && len(automation.Config.Conditions) > 0 {
		result.WriteString("\n")
		f.writeConditionsSection(&result, automation.Config.Conditions)
	} else if automation.Config != nil {
		result.WriteString("\nConditions: None\n")
	}

	// Actions
	if automation.Config != nil {
		result.WriteString("\n")
		f.writeActionsSection(&result, automation.Config.Actions)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// Helper methods for NaturalAutomationFormatter

func (f *NaturalAutomationFormatter) countByState(automations []homeassistant.Automation) (enabled, disabled int) {
	for _, auto := range automations {
		if auto.State == "on" {
			enabled++
		} else {
			disabled++
		}
	}
	return
}

func (f *NaturalAutomationFormatter) countByMode(
	automations []homeassistant.Automation,
	configs map[string]*homeassistant.AutomationConfig,
) map[string]int {
	counts := make(map[string]int)
	for _, auto := range automations {
		mode := modeSingle // default mode
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		if cfg, ok := configs[autoID]; ok && cfg != nil && cfg.Mode != "" {
			mode = cfg.Mode
		}
		counts[mode]++
	}
	return counts
}

func (f *NaturalAutomationFormatter) formatModeCounts(counts map[string]int) string {
	// Sort modes by count descending
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Value != sorted[j].Value {
			return sorted[i].Value > sorted[j].Value
		}
		return sorted[i].Key < sorted[j].Key
	})

	var parts []string
	for _, kv := range sorted {
		parts = append(parts, fmt.Sprintf("%s: %d", kv.Key, kv.Value))
	}
	return strings.Join(parts, ", ")
}

func (f *NaturalAutomationFormatter) getRecentlyTriggered(automations []homeassistant.Automation, limit int) []homeassistant.Automation {
	// Filter enabled automations with last_triggered
	var triggered []homeassistant.Automation
	for _, auto := range automations {
		if auto.State == "on" && auto.LastTriggered != "" {
			triggered = append(triggered, auto)
		}
	}

	// Sort by last_triggered descending
	sort.Slice(triggered, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, triggered[i].LastTriggered)
		tj, _ := time.Parse(time.RFC3339, triggered[j].LastTriggered)
		return ti.After(tj)
	})

	if len(triggered) > limit {
		return triggered[:limit]
	}
	return triggered
}

func (f *NaturalAutomationFormatter) getDisabled(automations []homeassistant.Automation) []homeassistant.Automation {
	var disabled []homeassistant.Automation
	for _, auto := range automations {
		if auto.State == stateOff {
			disabled = append(disabled, auto)
		}
	}
	return disabled
}

func (f *NaturalAutomationFormatter) getDisplayName(auto homeassistant.Automation) string {
	if auto.FriendlyName != "" {
		return auto.FriendlyName
	}
	return strings.TrimPrefix(auto.EntityID, "automation.")
}

func (f *NaturalAutomationFormatter) writeAutomationLine(
	result *strings.Builder,
	auto homeassistant.Automation,
	configs map[string]*homeassistant.AutomationConfig,
	verbose bool,
) {
	name := f.getDisplayName(auto)
	state := "enabled"
	if auto.State == stateOff {
		state = "disabled"
	}

	fmt.Fprintf(result, "- %s (%s)", name, state)

	// Add last triggered time
	if auto.LastTriggered != "" {
		lastTime, err := time.Parse(time.RFC3339, auto.LastTriggered)
		if err == nil {
			fmt.Fprintf(result, " - %s", FormatTimeSince(lastTime, f.now))
		}
	}

	result.WriteString("\n")

	// Verbose: add trigger/action summary
	if verbose {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		if cfg, ok := configs[autoID]; ok && cfg != nil {
			triggerCount := len(cfg.Triggers)
			actionCount := len(cfg.Actions)
			fmt.Fprintf(result, "  %d trigger(s), %d action(s)\n", triggerCount, actionCount)
		}
	}
}

func (f *NaturalAutomationFormatter) writeTriggersSection(result *strings.Builder, triggers []any) {
	if len(triggers) == 0 {
		result.WriteString("Triggers: None\n")
		return
	}

	fmt.Fprintf(result, "Triggers (%d):\n", len(triggers))
	for _, t := range triggers {
		result.WriteString("- ")
		result.WriteString(f.formatTrigger(t))
		result.WriteString("\n")
	}
}

func (f *NaturalAutomationFormatter) formatTrigger(trigger any) string {
	triggerMap, ok := trigger.(map[string]any)
	if !ok {
		return "unknown trigger"
	}

	platform, _ := triggerMap["platform"].(string)
	if platform == "" {
		platform, _ = triggerMap["trigger"].(string)
	}

	entityID, _ := triggerMap["entity_id"].(string)

	switch platform {
	case "state":
		from, _ := triggerMap["from"].(string)
		to, _ := triggerMap["to"].(string)
		if from != "" && to != "" {
			return fmt.Sprintf("state: %s (%s -> %s)", entityID, from, to)
		}
		return fmt.Sprintf("state: %s", entityID)
	case "time":
		at, _ := triggerMap["at"].(string)
		return fmt.Sprintf("time: %s", at)
	case "sun":
		event, _ := triggerMap["event"].(string)
		return fmt.Sprintf("sun: %s", event)
	case "device":
		deviceID, _ := triggerMap["device_id"].(string)
		return fmt.Sprintf("device: %s", deviceID)
	default:
		if entityID != "" {
			return fmt.Sprintf("%s: %s", platform, entityID)
		}
		return platform
	}
}

func (f *NaturalAutomationFormatter) writeConditionsSection(result *strings.Builder, conditions []any) {
	fmt.Fprintf(result, "Conditions (%d):\n", len(conditions))
	for _, c := range conditions {
		result.WriteString("- ")
		result.WriteString(f.formatCondition(c))
		result.WriteString("\n")
	}
}

func (f *NaturalAutomationFormatter) formatCondition(condition any) string {
	condMap, ok := condition.(map[string]any)
	if !ok {
		return "unknown condition"
	}

	condType, _ := condMap["condition"].(string)
	entityID, _ := condMap["entity_id"].(string)

	switch condType {
	case "state":
		state, _ := condMap["state"].(string)
		return fmt.Sprintf("state: %s = %s", entityID, state)
	case "time":
		after, _ := condMap["after"].(string)
		before, _ := condMap["before"].(string)
		if after != "" && before != "" {
			return fmt.Sprintf("time: %s - %s", after, before)
		}
		return "time condition"
	case "numeric_state":
		return fmt.Sprintf("numeric_state: %s", entityID)
	default:
		if entityID != "" {
			return fmt.Sprintf("%s: %s", condType, entityID)
		}
		return condType
	}
}

func (f *NaturalAutomationFormatter) writeActionsSection(result *strings.Builder, actions []any) {
	if len(actions) == 0 {
		result.WriteString("Actions: None\n")
		return
	}

	fmt.Fprintf(result, "Actions (%d):\n", len(actions))
	for _, a := range actions {
		result.WriteString("- ")
		result.WriteString(f.formatAction(a))
		result.WriteString("\n")
	}
}

func (f *NaturalAutomationFormatter) formatAction(action any) string {
	actionMap, ok := action.(map[string]any)
	if !ok {
		return "unknown action"
	}

	service, _ := actionMap["service"].(string)
	if service == "" {
		// Check for action key
		actionType, _ := actionMap["action"].(string)
		if actionType != "" {
			return actionType
		}
		return "unknown action"
	}

	// Extract target entity
	target := ""
	if targetMap, ok := actionMap["target"].(map[string]any); ok {
		if entityID, ok := targetMap["entity_id"].(string); ok {
			target = entityID
		} else if entityIDs, ok := targetMap["entity_id"].([]any); ok && len(entityIDs) > 0 {
			if eid, ok := entityIDs[0].(string); ok {
				target = eid
				if len(entityIDs) > 1 {
					target += fmt.Sprintf(" (+%d more)", len(entityIDs)-1)
				}
			}
		}
	}

	// Extract data attributes
	dataInfo := ""
	if data, ok := actionMap["data"].(map[string]any); ok && len(data) > 0 {
		var attrs []string
		for k, v := range data {
			attrs = append(attrs, fmt.Sprintf("%s: %v", k, v))
		}
		sort.Strings(attrs)
		dataInfo = " (" + strings.Join(attrs, ", ") + ")"
	}

	if target != "" {
		return fmt.Sprintf("%s: %s%s", service, target, dataInfo)
	}
	return service + dataInfo
}

// =============================================================================
// JSON Formatter
// =============================================================================

// JSONAutomationFormatter produces JSON output for automation data.
type JSONAutomationFormatter struct{}

// NewJSONAutomationFormatter creates a new JSONAutomationFormatter.
func NewJSONAutomationFormatter() *JSONAutomationFormatter {
	return &JSONAutomationFormatter{}
}

// FormatList formats a list of automations as JSON.
func (f *JSONAutomationFormatter) FormatList(
	_ context.Context,
	automations []homeassistant.Automation,
	_ map[string]*homeassistant.AutomationConfig,
	_ AutomationListOptions,
) (string, error) {
	if automations == nil {
		automations = []homeassistant.Automation{}
	}

	// Create compact output without configs for list
	type compactAutomation struct {
		EntityID      string `json:"entity_id"`
		State         string `json:"state"`
		Alias         string `json:"alias,omitempty"`
		LastTriggered string `json:"last_triggered,omitempty"`
	}

	compact := make([]compactAutomation, 0, len(automations))
	for _, auto := range automations {
		compact = append(compact, compactAutomation{
			EntityID:      auto.EntityID,
			State:         auto.State,
			Alias:         auto.FriendlyName,
			LastTriggered: auto.LastTriggered,
		})
	}

	data, err := json.MarshalIndent(compact, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal automations: %w", err)
	}
	return string(data), nil
}

// FormatDetail formats a single automation as JSON.
func (f *JSONAutomationFormatter) FormatDetail(
	_ context.Context,
	automation homeassistant.Automation,
) (string, error) {
	data, err := json.MarshalIndent(automation, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal automation: %w", err)
	}
	return string(data), nil
}

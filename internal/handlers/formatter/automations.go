// coverage-exempt: mechanical output formatting with no business logic
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
	autoID := strings.TrimPrefix(auto.EntityID, "automation.")
	state := "enabled"
	if auto.State == stateOff {
		state = "disabled"
	}

	fmt.Fprintf(result, "- %s [%s] (%s)", name, autoID, state)

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

	if result := f.formatKnownTrigger(platform, entityID, triggerMap); result != "" {
		return result
	}

	return f.formatUnknownTrigger(platform, entityID, triggerMap)
}

func (f *NaturalAutomationFormatter) formatKnownTrigger(platform, entityID string, triggerMap map[string]any) string {
	switch platform {
	case "state":
		return f.formatStateTrigger(entityID, triggerMap)
	case "time":
		at, _ := triggerMap["at"].(string)
		return fmt.Sprintf("time: %s", at)
	case "sun":
		event, _ := triggerMap["event"].(string)
		return fmt.Sprintf("sun: %s", event)
	case "device":
		return f.formatDeviceTrigger(triggerMap)
	case "numeric_state":
		return f.formatNumericStateTrigger(entityID, triggerMap)
	case "zone":
		return f.formatZoneTrigger(entityID, triggerMap)
	case "event":
		eventType, _ := triggerMap["event_type"].(string)
		return fmt.Sprintf("event: %s", eventType)
	case "template":
		return f.formatTemplateTrigger(triggerMap)
	case "homeassistant":
		event, _ := triggerMap["event"].(string)
		return fmt.Sprintf("homeassistant: %s", event)
	case "mqtt":
		topic, _ := triggerMap["topic"].(string)
		return fmt.Sprintf("mqtt: %s", topic)
	case "webhook":
		webhookID, _ := triggerMap["webhook_id"].(string)
		return fmt.Sprintf("webhook: %s", webhookID)
	default:
		return ""
	}
}

func (f *NaturalAutomationFormatter) formatStateTrigger(entityID string, triggerMap map[string]any) string {
	from, _ := triggerMap["from"].(string)
	to, _ := triggerMap["to"].(string)

	var parts []string
	parts = append(parts, entityID)

	if from != "" {
		parts = append(parts, "from: "+from)
	}
	if to != "" {
		parts = append(parts, "to: "+to)
	}
	if forDur := formatForDuration(triggerMap); forDur != "" {
		parts = append(parts, "for: "+forDur)
	}

	return fmt.Sprintf("state: %s", strings.Join(parts, ", "))
}

func (f *NaturalAutomationFormatter) formatNumericStateTrigger(entityID string, triggerMap map[string]any) string {
	parts := []string{entityID}
	if v, ok := triggerMap["above"]; ok {
		parts = append(parts, fmt.Sprintf("above %v", v))
	}
	if v, ok := triggerMap["below"]; ok {
		parts = append(parts, fmt.Sprintf("below %v", v))
	}
	if forDur := formatForDuration(triggerMap); forDur != "" {
		parts = append(parts, "for "+forDur)
	}
	return "numeric_state: " + strings.Join(parts, " ")
}

// formatForDuration extracts a "for" duration from a trigger/condition map.
// HA supports both string form ("00:05:00") and map form ({hours, minutes, seconds}).
func formatForDuration(m map[string]any) string {
	if s, ok := m["for"].(string); ok {
		return s
	}
	if dm, ok := m["for"].(map[string]any); ok {
		h, _ := dm["hours"].(float64)
		mins, _ := dm["minutes"].(float64)
		sec, _ := dm["seconds"].(float64)
		return fmt.Sprintf("%d:%02d:%02d", int(h), int(mins), int(sec))
	}
	return ""
}

func (f *NaturalAutomationFormatter) formatZoneTrigger(entityID string, triggerMap map[string]any) string {
	zone, _ := triggerMap["zone"].(string)
	event, _ := triggerMap["event"].(string)
	return fmt.Sprintf("zone: %s %s %s", entityID, event, zone)
}

func (f *NaturalAutomationFormatter) formatDeviceTrigger(triggerMap map[string]any) string {
	deviceID, _ := triggerMap["device_id"].(string)
	dtype, _ := triggerMap["type"].(string)
	subtype, _ := triggerMap["subtype"].(string)

	if dtype != "" && subtype != "" {
		return fmt.Sprintf("device: %s (%s/%s)", deviceID, dtype, subtype)
	}
	if dtype != "" {
		return fmt.Sprintf("device: %s (%s)", deviceID, dtype)
	}
	return fmt.Sprintf("device: %s", deviceID)
}

func (f *NaturalAutomationFormatter) formatTemplateTrigger(triggerMap map[string]any) string {
	tmpl, _ := triggerMap["value_template"].(string)
	if tmpl != "" {
		const maxLen = 80
		if len(tmpl) > maxLen {
			tmpl = tmpl[:maxLen-3] + "..."
		}
		return fmt.Sprintf("template: %s", tmpl)
	}
	return "template trigger"
}

func (f *NaturalAutomationFormatter) formatUnknownTrigger(platform, entityID string, triggerMap map[string]any) string {
	if entityID != "" {
		return fmt.Sprintf("%s: %s", platform, entityID)
	}
	if platform != "" {
		return platform
	}
	// Fallback: show available keys
	var keys []string
	for k := range triggerMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("trigger (%s)", strings.Join(keys, ", "))
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
		parts := []string{entityID}
		if v, ok := condMap["above"]; ok {
			parts = append(parts, fmt.Sprintf("above %v", v))
		}
		if v, ok := condMap["below"]; ok {
			parts = append(parts, fmt.Sprintf("below %v", v))
		}
		return "numeric_state: " + strings.Join(parts, " ")
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
		return f.formatNonServiceAction(actionMap)
	}

	return f.formatServiceAction(service, actionMap)
}

func (f *NaturalAutomationFormatter) formatNonServiceAction(actionMap map[string]any) string {
	// Modern HA uses "action" key instead of legacy "service" — same target/data extraction applies.
	if actionType, ok := actionMap["action"].(string); ok && actionType != "" {
		return f.formatServiceAction(actionType, actionMap)
	}

	// Check for complex action types
	if result := f.formatComplexAction(actionMap); result != "" {
		return result
	}

	// Fallback: show available keys
	return formatMapKeys(actionMap, "action")
}

func (f *NaturalAutomationFormatter) formatComplexAction(actionMap map[string]any) string {
	if choices, hasChoose := actionMap["choose"]; hasChoose {
		return f.formatChooseBlock(choices)
	}
	if _, hasIf := actionMap["if"]; hasIf {
		return f.formatIfBlock(actionMap)
	}
	if repeatVal, hasRepeat := actionMap["repeat"]; hasRepeat {
		return f.formatRepeatBlock(repeatVal)
	}
	if parallelVal, hasParallel := actionMap["parallel"]; hasParallel {
		return f.formatParallelBlock(parallelVal)
	}
	return ""
}

// formatRepeatAction formats a repeat action headline (count summary only).
// Used by TestFormatRepeatAction which tests the standalone summary format.
func formatRepeatAction(repeatVal any) string {
	repeatMap, ok := repeatVal.(map[string]any)
	if !ok {
		return "repeat action"
	}
	seq, _ := repeatMap["sequence"].([]any)
	seqLen := len(seq)
	if whileVal, hasWhile := repeatMap["while"]; hasWhile {
		conditions, _ := whileVal.([]any)
		return fmt.Sprintf("repeat while [%d condition(s)]: %d action(s) in sequence", len(conditions), seqLen)
	}
	if untilVal, hasUntil := repeatMap["until"]; hasUntil {
		conditions, _ := untilVal.([]any)
		return fmt.Sprintf("repeat until [%d condition(s)]: %d action(s) in sequence", len(conditions), seqLen)
	}
	if count, hasCount := repeatMap["count"]; hasCount {
		return fmt.Sprintf("repeat %v times: %d action(s) in sequence", count, seqLen)
	}
	return fmt.Sprintf("repeat: %d action(s) in sequence", seqLen)
}

// formatRepeatBlock renders a repeat action with its sequence inline.
func (f *NaturalAutomationFormatter) formatRepeatBlock(repeatVal any) string {
	repeatMap, ok := repeatVal.(map[string]any)
	if !ok {
		return "repeat action"
	}
	headline := formatRepeatAction(repeatVal)
	seq, _ := repeatMap["sequence"].([]any)
	if len(seq) == 0 {
		return headline
	}
	var b strings.Builder
	b.WriteString(headline)
	for _, a := range seq {
		b.WriteString("\n    - ")
		b.WriteString(f.formatAction(a))
	}
	return b.String()
}

// formatChooseBlock renders a choose block with each option's conditions and sequence.
func (f *NaturalAutomationFormatter) formatChooseBlock(choicesVal any) string {
	choices, _ := choicesVal.([]any)
	var b strings.Builder
	fmt.Fprintf(&b, "choose: %d option(s)", len(choices))
	for i, choice := range choices {
		cm, ok := choice.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "\n  option %d:", i+1)
		if conds, ok := cm["conditions"].([]any); ok && len(conds) > 0 {
			for _, c := range conds {
				b.WriteString("\n    if: ")
				b.WriteString(f.formatCondition(c))
			}
		}
		if seq, ok := cm["sequence"].([]any); ok {
			for _, a := range seq {
				b.WriteString("\n    then: ")
				b.WriteString(f.formatAction(a))
			}
		}
	}
	return b.String()
}

// formatIfBlock renders an if/then/else block with branch actions.
func (f *NaturalAutomationFormatter) formatIfBlock(actionMap map[string]any) string {
	var b strings.Builder
	b.WriteString("conditional: if/then/else")
	if conds, ok := actionMap["if"].([]any); ok {
		for _, c := range conds {
			b.WriteString("\n  if: ")
			b.WriteString(f.formatCondition(c))
		}
	}
	if then, ok := actionMap["then"].([]any); ok {
		for _, a := range then {
			b.WriteString("\n  then: ")
			b.WriteString(f.formatAction(a))
		}
	}
	if els, ok := actionMap["else"].([]any); ok {
		for _, a := range els {
			b.WriteString("\n  else: ")
			b.WriteString(f.formatAction(a))
		}
	}
	return b.String()
}

// formatParallelBlock renders a parallel block with each branch.
func (f *NaturalAutomationFormatter) formatParallelBlock(parallelVal any) string {
	items, _ := parallelVal.([]any)
	if len(items) == 0 {
		return "parallel actions"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "parallel: %d branch(es)", len(items))
	for _, item := range items {
		b.WriteString("\n  - ")
		b.WriteString(f.formatAction(item))
	}
	return b.String()
}

func (f *NaturalAutomationFormatter) formatServiceAction(service string, actionMap map[string]any) string {
	target := f.extractActionTarget(actionMap)
	dataInfo := f.extractDataInfo(actionMap)

	if target != "" {
		return fmt.Sprintf("%s: %s%s", service, target, dataInfo)
	}
	return service + dataInfo
}

func (f *NaturalAutomationFormatter) extractActionTarget(actionMap map[string]any) string {
	// 1. target.entity_id (modern HA format)
	if targetMap, ok := actionMap["target"].(map[string]any); ok {
		if entityID, ok := targetMap["entity_id"].(string); ok && entityID != "" {
			return entityID
		}
		if entityIDs, ok := targetMap["entity_id"].([]any); ok && len(entityIDs) > 0 {
			if eid, ok := entityIDs[0].(string); ok {
				if len(entityIDs) > 1 {
					return eid + fmt.Sprintf(" (+%d more)", len(entityIDs)-1)
				}
				return eid
			}
		}
	}
	// 2. Top-level entity_id (legacy HA automation format, pre-2022)
	if entityID, ok := actionMap["entity_id"].(string); ok && entityID != "" {
		return entityID
	}
	// 3. data.entity_id (some service calls embed the target in the data map)
	if data, ok := actionMap["data"].(map[string]any); ok {
		if entityID, ok := data["entity_id"].(string); ok && entityID != "" {
			return entityID
		}
	}
	return ""
}

func (f *NaturalAutomationFormatter) extractDataInfo(actionMap map[string]any) string {
	data, ok := actionMap["data"].(map[string]any)
	if !ok || len(data) == 0 {
		return ""
	}

	var attrs []string
	for k, v := range data {
		attrs = append(attrs, fmt.Sprintf("%s: %v", k, v))
	}
	sort.Strings(attrs)
	return " (" + strings.Join(attrs, ", ") + ")"
}

func formatMapKeys(m map[string]any, prefix string) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("%s (%s)", prefix, strings.Join(keys, ", "))
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

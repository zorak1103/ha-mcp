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

// Message constants for script formatter.
const (
	MsgNoScriptsFound = "No scripts found."
)

// ScriptListOptions configures script list formatting.
type ScriptListOptions struct {
	Verbose bool
	Limit   int
}

// ScriptFormatter defines the interface for formatting script responses.
type ScriptFormatter interface {
	// FormatList formats a list of scripts.
	FormatList(ctx context.Context, scripts []homeassistant.Script, opts ScriptListOptions) (string, error)

	// FormatDetail formats a single script with full details.
	FormatDetail(ctx context.Context, script homeassistant.Script) (string, error)
}

// NewScriptFormatter creates a new ScriptFormatter for the specified format.
func NewScriptFormatter(format Format) ScriptFormatter {
	switch format {
	case FormatJSON:
		return NewJSONScriptFormatter()
	case FormatNatural:
		return NewNaturalScriptFormatter()
	default:
		return NewNaturalScriptFormatter()
	}
}

// =============================================================================
// Natural Language Formatter
// =============================================================================

// NaturalScriptFormatter produces human-readable script output.
type NaturalScriptFormatter struct {
	now time.Time
}

// NewNaturalScriptFormatter creates a new NaturalScriptFormatter.
func NewNaturalScriptFormatter() *NaturalScriptFormatter {
	return &NaturalScriptFormatter{
		now: time.Now(),
	}
}

// FormatList formats a list of scripts in natural language.
func (f *NaturalScriptFormatter) FormatList(
	_ context.Context,
	scripts []homeassistant.Script,
	opts ScriptListOptions,
) (string, error) {
	if len(scripts) == 0 {
		return MsgNoScriptsFound, nil
	}

	var result strings.Builder

	// Summary line
	fmt.Fprintf(&result, "%d scripts\n\n", len(scripts))

	// Mode breakdown
	modeCounts := f.countByMode(scripts)
	if len(modeCounts) > 0 {
		result.WriteString("By mode: ")
		result.WriteString(f.formatModeCounts(modeCounts))
		result.WriteString("\n\n")
	}

	// Script list
	result.WriteString("Scripts:\n")
	for _, script := range scripts {
		f.writeScriptLine(&result, script, opts.Verbose)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatDetail formats a single script with full details.
func (f *NaturalScriptFormatter) FormatDetail(
	_ context.Context,
	script homeassistant.Script,
) (string, error) {
	var result strings.Builder

	// Header: Name
	name := f.getDisplayName(script)
	fmt.Fprintf(&result, "Script: %s\n", name)

	// Mode and last triggered
	if script.Config != nil {
		mode := script.Config.Mode
		if mode == "" {
			mode = "single"
		}
		result.WriteString("Mode: " + mode)
	}

	if script.LastTriggered != "" {
		lastTime, err := time.Parse(time.RFC3339, script.LastTriggered)
		if err == nil {
			result.WriteString(" | Last run: " + FormatTimeSince(lastTime, f.now))
		}
	}
	result.WriteString("\n")

	// Icon
	if script.Config != nil && script.Config.Icon != "" {
		fmt.Fprintf(&result, "Icon: %s\n", script.Config.Icon)
	}

	// Description
	if script.Config != nil && script.Config.Description != "" {
		result.WriteString("\n" + script.Config.Description + "\n")
	}

	// Sequence
	if script.Config != nil {
		result.WriteString("\n")
		f.writeSequenceSection(&result, script.Config.Sequence)
	}

	// Fields (if any)
	if script.Config != nil && len(script.Config.Fields) > 0 {
		result.WriteString("\n")
		f.writeFieldsSection(&result, script.Config.Fields)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// Helper methods for NaturalScriptFormatter

func (f *NaturalScriptFormatter) countByMode(scripts []homeassistant.Script) map[string]int {
	counts := make(map[string]int)
	for _, script := range scripts {
		mode := "single" // default mode
		if script.Config != nil && script.Config.Mode != "" {
			mode = script.Config.Mode
		}
		counts[mode]++
	}
	return counts
}

func (f *NaturalScriptFormatter) formatModeCounts(counts map[string]int) string {
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

func (f *NaturalScriptFormatter) getDisplayName(script homeassistant.Script) string {
	if script.FriendlyName != "" {
		return script.FriendlyName
	}
	return strings.TrimPrefix(script.EntityID, "script.")
}

func (f *NaturalScriptFormatter) writeScriptLine(result *strings.Builder, script homeassistant.Script, verbose bool) {
	name := f.getDisplayName(script)

	stepCount := 0
	if script.Config != nil {
		stepCount = len(script.Config.Sequence)
	}

	fmt.Fprintf(result, "- %s", name)

	// Add step count
	if stepCount > 0 {
		fmt.Fprintf(result, " - %d step(s)", stepCount)
	}

	// Add last triggered time
	if script.LastTriggered != "" {
		lastTime, err := time.Parse(time.RFC3339, script.LastTriggered)
		if err == nil {
			fmt.Fprintf(result, ", last run: %s", FormatTimeSince(lastTime, f.now))
		}
	} else {
		result.WriteString(", never run")
	}

	result.WriteString("\n")

	// Verbose: add more details
	if verbose && script.Config != nil {
		if script.Config.Description != "" {
			fmt.Fprintf(result, "  %s\n", script.Config.Description)
		}
	}
}

func (f *NaturalScriptFormatter) writeSequenceSection(result *strings.Builder, sequence []any) {
	if len(sequence) == 0 {
		result.WriteString("Sequence: (empty)\n")
		return
	}

	fmt.Fprintf(result, "Sequence (%d steps):\n", len(sequence))
	for i, step := range sequence {
		fmt.Fprintf(result, "%d. %s\n", i+1, f.formatSequenceStep(step))
	}
}

func (f *NaturalScriptFormatter) formatSequenceStep(step any) string {
	stepMap, ok := step.(map[string]any)
	if !ok {
		return "unknown step"
	}

	// Check for service call
	if service, ok := stepMap["service"].(string); ok {
		target := f.extractTarget(stepMap)
		if target != "" {
			return fmt.Sprintf("%s: %s", service, target)
		}
		return service
	}

	// Check for delay
	if delay, ok := stepMap["delay"].(string); ok {
		return fmt.Sprintf("delay: %s", delay)
	}
	if delay, ok := stepMap["delay"].(map[string]any); ok {
		return fmt.Sprintf("delay: %v", delay)
	}

	// Check for wait_template
	if waitTemplate, ok := stepMap["wait_template"].(string); ok {
		return fmt.Sprintf("wait_template: %s", waitTemplate)
	}

	// Check for condition
	if condition, ok := stepMap["condition"].(string); ok {
		return fmt.Sprintf("condition: %s", condition)
	}

	// Check for choose
	if _, ok := stepMap["choose"]; ok {
		return "choose (conditional)"
	}

	// Check for repeat
	if _, ok := stepMap["repeat"]; ok {
		return "repeat (loop)"
	}

	// Check for event
	if event, ok := stepMap["event"].(string); ok {
		return fmt.Sprintf("event: %s", event)
	}

	// Generic action type
	if action, ok := stepMap["action"].(string); ok {
		return action
	}

	return "action"
}

func (f *NaturalScriptFormatter) extractTarget(stepMap map[string]any) string {
	if target, ok := stepMap["target"].(map[string]any); ok {
		if entityID, ok := target["entity_id"].(string); ok {
			return entityID
		}
		if entityIDs, ok := target["entity_id"].([]any); ok && len(entityIDs) > 0 {
			if eid, ok := entityIDs[0].(string); ok {
				if len(entityIDs) > 1 {
					return fmt.Sprintf("%s (+%d more)", eid, len(entityIDs)-1)
				}
				return eid
			}
		}
	}
	return ""
}

func (f *NaturalScriptFormatter) writeFieldsSection(result *strings.Builder, fields map[string]any) {
	fmt.Fprintf(result, "Fields (%d):\n", len(fields))
	for name, config := range fields {
		desc := ""
		if configMap, ok := config.(map[string]any); ok {
			if d, ok := configMap["description"].(string); ok {
				desc = d
			}
		}
		if desc != "" {
			fmt.Fprintf(result, "- %s: %s\n", name, desc)
		} else {
			fmt.Fprintf(result, "- %s\n", name)
		}
	}
}

// =============================================================================
// JSON Formatter
// =============================================================================

// JSONScriptFormatter produces JSON output for script data.
type JSONScriptFormatter struct{}

// NewJSONScriptFormatter creates a new JSONScriptFormatter.
func NewJSONScriptFormatter() *JSONScriptFormatter {
	return &JSONScriptFormatter{}
}

// FormatList formats a list of scripts as JSON.
func (f *JSONScriptFormatter) FormatList(
	_ context.Context,
	scripts []homeassistant.Script,
	_ ScriptListOptions,
) (string, error) {
	if scripts == nil {
		scripts = []homeassistant.Script{}
	}

	// Create compact output for list
	type compactScript struct {
		EntityID      string `json:"entity_id"`
		State         string `json:"state"`
		FriendlyName  string `json:"friendly_name,omitempty"`
		LastTriggered string `json:"last_triggered,omitempty"`
	}

	compact := make([]compactScript, 0, len(scripts))
	for _, s := range scripts {
		compact = append(compact, compactScript{
			EntityID:      s.EntityID,
			State:         s.State,
			FriendlyName:  s.FriendlyName,
			LastTriggered: s.LastTriggered,
		})
	}

	data, err := json.MarshalIndent(compact, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal scripts: %w", err)
	}
	return string(data), nil
}

// FormatDetail formats a single script as JSON.
func (f *JSONScriptFormatter) FormatDetail(
	_ context.Context,
	script homeassistant.Script,
) (string, error) {
	data, err := json.MarshalIndent(script, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal script: %w", err)
	}
	return string(data), nil
}

// coverage-exempt: mechanical output formatting with no business logic
package formatter

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Message constants for helper formatter.
const (
	MsgNoHelpersFound = "No helpers found."
)

// Helper type constants.
const (
	helperTypeInputBoolean = "input_boolean"
	helperTypeInputNumber  = "input_number"
	helperTypeInputSelect  = "input_select"
	helperStateActive      = "active"
)

// HelperListOptions configures helper list formatting.
type HelperListOptions struct {
	Verbose     bool
	GroupByType bool
}

// HelperFormatter defines the interface for formatting helper responses.
type HelperFormatter interface {
	// FormatList formats a list of helpers.
	FormatList(ctx context.Context, helpers []homeassistant.Entity, opts HelperListOptions) (string, error)

	// FormatScheduleDetail formats schedule details.
	FormatScheduleDetail(ctx context.Context, detail map[string]any) (string, error)

	// FormatCounterDetail formats counter details.
	FormatCounterDetail(ctx context.Context, detail map[string]any) (string, error)

	// FormatTimerDetail formats timer details.
	FormatTimerDetail(ctx context.Context, detail map[string]any) (string, error)

	// FormatHelperDetail formats helper details for all other helper types.
	FormatHelperDetail(ctx context.Context, helperType string, detail map[string]any) (string, error)
}

// NewHelperFormatter creates a new HelperFormatter for the specified format.
func NewHelperFormatter(format Format) HelperFormatter {
	switch format {
	case FormatJSON:
		return NewJSONHelperFormatter()
	case FormatNatural:
		return NewNaturalHelperFormatter()
	default:
		return NewNaturalHelperFormatter()
	}
}

// =============================================================================
// Natural Language Formatter
// =============================================================================

// NaturalHelperFormatter produces human-readable helper output.
type NaturalHelperFormatter struct{}

// NewNaturalHelperFormatter creates a new NaturalHelperFormatter.
func NewNaturalHelperFormatter() *NaturalHelperFormatter {
	return &NaturalHelperFormatter{}
}

// FormatList formats a list of helpers in natural language.
func (f *NaturalHelperFormatter) FormatList(
	_ context.Context,
	helpers []homeassistant.Entity,
	opts HelperListOptions,
) (string, error) {
	if len(helpers) == 0 {
		return MsgNoHelpersFound, nil
	}

	var result strings.Builder

	// Count by type
	typeCounts := f.countByType(helpers)

	// Summary line
	fmt.Fprintf(&result, "%d helpers across %d types\n\n", len(helpers), len(typeCounts))

	if opts.GroupByType {
		f.writeGroupedList(&result, helpers, typeCounts, opts.Verbose)
	} else {
		f.writeFlatList(&result, helpers, typeCounts, opts.Verbose)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatScheduleDetail formats schedule details in natural language.
func (f *NaturalHelperFormatter) FormatScheduleDetail(
	_ context.Context,
	detail map[string]any,
) (string, error) {
	var result strings.Builder

	// Header
	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	entityID := f.getDetailString(detail, "entity_id")
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Schedule: %s is %s\n", FormatNameWithID(name, entityID), state)

	// Next event
	if nextEvent := f.getDetailString(detail, "next_event"); nextEvent != "" {
		fmt.Fprintf(&result, "Next: %s\n", nextEvent)
	}

	// Schedule blocks
	if schedule, ok := detail["schedule"].(map[string][]map[string]string); ok && len(schedule) > 0 {
		result.WriteString("\n")
		f.writeScheduleBlocks(&result, schedule)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatCounterDetail formats counter details in natural language.
func (f *NaturalHelperFormatter) FormatCounterDetail(
	_ context.Context,
	detail map[string]any,
) (string, error) {
	var result strings.Builder

	// Header
	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	entityID := f.getDetailString(detail, "entity_id")
	value := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Counter: %s\n", FormatNameWithID(name, entityID))
	fmt.Fprintf(&result, "Current value: %s\n", value)

	// Counter configuration
	if initial := f.getDetailString(detail, "initial"); initial != "" {
		fmt.Fprintf(&result, "Initial value: %s\n", initial)
	}
	if minimum := f.getDetailString(detail, "minimum"); minimum != "" {
		fmt.Fprintf(&result, "Minimum: %s\n", minimum)
	}
	if maximum := f.getDetailString(detail, "maximum"); maximum != "" {
		fmt.Fprintf(&result, "Maximum: %s\n", maximum)
	}
	if step := f.getDetailString(detail, "step"); step != "" {
		fmt.Fprintf(&result, "Step: %s", step)
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatTimerDetail formats timer details in natural language.
func (f *NaturalHelperFormatter) FormatTimerDetail(
	_ context.Context,
	detail map[string]any,
) (string, error) {
	var result strings.Builder

	// Header
	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	entityID := f.getDetailString(detail, "entity_id")
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Timer: %s\n", FormatNameWithID(name, entityID))
	fmt.Fprintf(&result, "State: %s\n", state)

	// Timer details
	if duration := f.getDetailString(detail, "duration"); duration != "" {
		fmt.Fprintf(&result, "Duration: %s\n", duration)
	}

	// Only show remaining and finishes_at if timer is active
	if state == helperStateActive {
		if remaining := f.getDetailString(detail, "remaining"); remaining != "" {
			fmt.Fprintf(&result, "Remaining: %s\n", remaining)
		}
		if finishesAt := f.getDetailString(detail, "finishes_at"); finishesAt != "" {
			fmt.Fprintf(&result, "Finishes at: %s", finishesAt)
		}
	}

	return strings.TrimSuffix(result.String(), "\n"), nil
}

// FormatHelperDetail formats helper details for all other helper types in natural language.
func (f *NaturalHelperFormatter) FormatHelperDetail(
	_ context.Context,
	helperType string,
	detail map[string]any,
) (string, error) {
	switch helperType {
	case helperTypeInputBoolean:
		return f.formatInputBooleanDetail(detail), nil
	case helperTypeInputNumber:
		return f.formatInputNumberDetail(detail), nil
	case "input_text":
		return f.formatInputTextDetail(detail), nil
	case helperTypeInputSelect:
		return f.formatInputSelectDetail(detail), nil
	case "input_datetime":
		return f.formatInputDatetimeDetail(detail), nil
	case "input_button":
		return f.formatInputButtonDetail(detail), nil
	case "group":
		return f.formatGroupDetail(detail), nil
	case "sensor":
		return f.formatSensorDetail(detail), nil
	case "binary_sensor":
		return f.formatBinarySensorDetail(detail), nil
	default:
		return "", fmt.Errorf("unsupported helper type: %s", helperType)
	}
}

func (f *NaturalHelperFormatter) formatInputBooleanDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	entityID := f.getDetailString(detail, "entity_id")
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Toggle: %s is %s\n", FormatNameWithID(name, entityID), state)

	if icon := f.getDetailString(detail, "icon"); icon != "" {
		fmt.Fprintf(&result, "Icon: %s\n", icon)
	}

	editable := f.getDetailBool(detail, "editable")
	fmt.Fprintf(&result, "Editable: %t", editable)

	return result.String()
}

func (f *NaturalHelperFormatter) formatInputNumberDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Number: %s\n", name)

	unit := f.getDetailString(detail, "unit_of_measurement")
	if unit != "" {
		fmt.Fprintf(&result, "Current value: %s %s\n", state, unit)
	} else {
		fmt.Fprintf(&result, "Current value: %s\n", state)
	}

	minVal := f.getDetailString(detail, "min")
	maxVal := f.getDetailString(detail, "max")
	if minVal != "" && maxVal != "" {
		fmt.Fprintf(&result, "Range: %s - %s\n", minVal, maxVal)
	}

	if step := f.getDetailString(detail, "step"); step != "" {
		fmt.Fprintf(&result, "Step: %s\n", step)
	}

	if mode := f.getDetailString(detail, "mode"); mode != "" {
		fmt.Fprintf(&result, "Mode: %s", mode)
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func (f *NaturalHelperFormatter) formatInputTextDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Text: %s\n", name)
	fmt.Fprintf(&result, "Current value: %s\n", state)

	minVal := f.getDetailString(detail, "min")
	maxVal := f.getDetailString(detail, "max")
	if minVal != "" && maxVal != "" {
		fmt.Fprintf(&result, "Length: %s - %s\n", minVal, maxVal)
	}

	if mode := f.getDetailString(detail, "mode"); mode != "" {
		fmt.Fprintf(&result, "Mode: %s\n", mode)
	}

	if pattern := f.getDetailString(detail, "pattern"); pattern != "" {
		fmt.Fprintf(&result, "Pattern: %s", pattern)
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func (f *NaturalHelperFormatter) formatInputSelectDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Select: %s\n", name)
	fmt.Fprintf(&result, "Selected: %s\n", state)

	if options, ok := detail["options"].([]any); ok && len(options) > 0 {
		optStrs := make([]string, 0, len(options))
		for _, opt := range options {
			if optStr, ok := opt.(string); ok {
				optStrs = append(optStrs, optStr)
			}
		}
		fmt.Fprintf(&result, "Options: %s", strings.Join(optStrs, ", "))
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func (f *NaturalHelperFormatter) formatInputDatetimeDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Date/Time: %s\n", name)
	fmt.Fprintf(&result, "Current value: %s\n", state)

	hasDate := f.getDetailBool(detail, "has_date")
	hasTime := f.getDetailBool(detail, "has_time")

	switch {
	case hasDate && hasTime:
		fmt.Fprintf(&result, "Type: Date and time")
	case hasDate:
		fmt.Fprintf(&result, "Type: Date only")
	case hasTime:
		fmt.Fprintf(&result, "Type: Time only")
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func (f *NaturalHelperFormatter) formatInputButtonDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Button: %s\n", name)
	fmt.Fprintf(&result, "Last pressed: %s", state)

	return result.String()
}

func (f *NaturalHelperFormatter) formatGroupDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	entityID := f.getDetailString(detail, "entity_id")
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Group: %s is %s\n", FormatNameWithID(name, entityID), state)

	all := f.getDetailBool(detail, "all")
	if all {
		fmt.Fprintf(&result, "Mode: all (all members must be on)\n")
	} else {
		fmt.Fprintf(&result, "Mode: any (at least one member)\n")
	}

	if members, ok := detail["members"].([]any); ok && len(members) > 0 {
		fmt.Fprintf(&result, "Members (%d):\n", len(members))
		for _, member := range members {
			if memberStr, ok := member.(string); ok {
				fmt.Fprintf(&result, "  • %s\n", memberStr)
			}
		}
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func (f *NaturalHelperFormatter) formatSensorDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	entityID := f.getDetailString(detail, "entity_id")
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Sensor: %s\n", FormatNameWithID(name, entityID))

	unit := f.getDetailString(detail, "unit_of_measurement")
	if unit != "" {
		fmt.Fprintf(&result, "Current value: %s %s\n", state, unit)
	} else {
		fmt.Fprintf(&result, "Current value: %s\n", state)
	}

	if deviceClass := f.getDetailString(detail, "device_class"); deviceClass != "" {
		fmt.Fprintf(&result, "Device class: %s\n", deviceClass)
	}

	if stateClass := f.getDetailString(detail, "state_class"); stateClass != "" {
		fmt.Fprintf(&result, "State class: %s\n", stateClass)
	}

	if source := f.getDetailString(detail, "source"); source != "" {
		fmt.Fprintf(&result, "Source: %s", source)
	}

	f.writeTemplateConfig(&result, detail)

	return strings.TrimSuffix(result.String(), "\n")
}

func (f *NaturalHelperFormatter) formatBinarySensorDetail(detail map[string]any) string {
	var result strings.Builder

	name := f.getDetailString(detail, "friendly_name")
	if name == "" {
		name = f.getDetailString(detail, "entity_id")
	}
	entityID := f.getDetailString(detail, "entity_id")
	state := f.getDetailString(detail, "state")

	fmt.Fprintf(&result, "Binary Sensor: %s\n", FormatNameWithID(name, entityID))
	fmt.Fprintf(&result, "Current state: %s\n", state)

	if deviceClass := f.getDetailString(detail, "device_class"); deviceClass != "" {
		fmt.Fprintf(&result, "Device class: %s\n", deviceClass)
	}

	if source := f.getDetailString(detail, "source_entity"); source != "" {
		fmt.Fprintf(&result, "Source entity: %s", source)
	}

	f.writeTemplateConfig(&result, detail)

	return strings.TrimSuffix(result.String(), "\n")
}

// writeTemplateConfig writes template configuration section if present.
func (f *NaturalHelperFormatter) writeTemplateConfig(result *strings.Builder, detail map[string]any) {
	configType := f.getDetailString(detail, "config_entry_type")
	if configType != "template" {
		return
	}

	result.WriteString("\n\nTemplate Configuration:")

	if stateTemplate := f.getDetailString(detail, "state_template"); stateTemplate != "" {
		fmt.Fprintf(result, "\nState template: %s", stateTemplate)
	}

	if availTemplate := f.getDetailString(detail, "availability_template"); availTemplate != "" {
		fmt.Fprintf(result, "\nAvailability template: %s", availTemplate)
	}

	if delayOn := detail["delay_on"]; delayOn != nil {
		fmt.Fprintf(result, "\nDelay on: %v", delayOn)
	}

	if delayOff := detail["delay_off"]; delayOff != nil {
		fmt.Fprintf(result, "\nDelay off: %v", delayOff)
	}
}

func (f *NaturalHelperFormatter) getDetailBool(detail map[string]any, key string) bool {
	if val, ok := detail[key].(bool); ok {
		return val
	}
	return false
}

// Helper methods for NaturalHelperFormatter

func (f *NaturalHelperFormatter) countByType(helpers []homeassistant.Entity) map[string]int {
	counts := make(map[string]int)
	for _, h := range helpers {
		helperType := f.extractType(h.EntityID)
		counts[helperType]++
	}
	return counts
}

func (f *NaturalHelperFormatter) extractType(entityID string) string {
	parts := strings.SplitN(entityID, ".", 2)
	if len(parts) < 2 {
		return stateUnknown
	}
	return parts[0]
}

func (f *NaturalHelperFormatter) writeGroupedList(
	result *strings.Builder,
	helpers []homeassistant.Entity,
	typeCounts map[string]int,
	verbose bool,
) {
	// Group helpers by type
	byType := make(map[string][]homeassistant.Entity)
	for _, h := range helpers {
		helperType := f.extractType(h.EntityID)
		byType[helperType] = append(byType[helperType], h)
	}

	// Sort types by count descending, then alphabetically
	types := f.sortTypesByCount(typeCounts)

	for _, helperType := range types {
		typeHelpers := byType[helperType]
		fmt.Fprintf(result, "%s (%d):\n", f.formatTypeName(helperType), len(typeHelpers))

		for _, h := range typeHelpers {
			f.writeHelperLine(result, h, verbose)
		}
		result.WriteString("\n")
	}
}

func (f *NaturalHelperFormatter) writeFlatList(
	result *strings.Builder,
	helpers []homeassistant.Entity,
	typeCounts map[string]int,
	verbose bool,
) {
	// Type breakdown
	result.WriteString("By type: ")
	types := f.sortTypesByCount(typeCounts)
	var parts []string
	for _, t := range types {
		parts = append(parts, fmt.Sprintf("%s: %d", t, typeCounts[t]))
	}
	result.WriteString(strings.Join(parts, ", "))
	result.WriteString("\n\n")

	// List all helpers
	result.WriteString("Helpers:\n")
	for _, h := range helpers {
		f.writeHelperLine(result, h, verbose)
	}
}

func (f *NaturalHelperFormatter) sortTypesByCount(counts map[string]int) []string {
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	slices.SortFunc(sorted, func(a, b kv) int {
		return cmp.Or(
			cmp.Compare(b.Value, a.Value),
			cmp.Compare(a.Key, b.Key),
		)
	})

	result := make([]string, 0, len(sorted))
	for _, kv := range sorted {
		result = append(result, kv.Key)
	}
	return result
}

func (f *NaturalHelperFormatter) formatTypeName(helperType string) string {
	nameMap := map[string]string{
		"input_boolean":  "Input Booleans",
		"input_number":   "Input Numbers",
		"input_text":     "Input Texts",
		"input_select":   "Input Selects",
		"input_datetime": "Input Datetimes",
		"input_button":   "Input Buttons",
		"counter":        "Counters",
		"timer":          "Timers",
		"schedule":       "Schedules",
		"group":          "Groups",
		"sensor":         "Sensors",
		"binary_sensor":  "Binary Sensors",
	}
	if name, ok := nameMap[helperType]; ok {
		return name
	}
	// Fallback: capitalize first letter and pluralize
	formatted := strings.ReplaceAll(helperType, "_", " ")
	if formatted != "" {
		formatted = strings.ToUpper(formatted[:1]) + formatted[1:]
	}
	return formatted + "s"
}

func (f *NaturalHelperFormatter) writeHelperLine(result *strings.Builder, h homeassistant.Entity, verbose bool) {
	name := GetFriendlyName(h.EntityID, h.Attributes)

	helperType := f.extractType(h.EntityID)
	stateInfo := f.formatStateForType(helperType, h)

	fmt.Fprintf(result, "  • %s: %s\n", FormatNameWithID(name, h.EntityID), stateInfo)

	if verbose {
		f.writeVerboseDetails(result, h)
	}
}

func (f *NaturalHelperFormatter) formatStateForType(helperType string, h homeassistant.Entity) string {
	switch helperType {
	case "counter":
		return h.State
	case helperTypeInputBoolean:
		return h.State
	case "input_number":
		unit := GetStringAttr(h.Attributes, "unit_of_measurement")
		if unit != "" {
			return h.State + " " + unit
		}
		return h.State
	case "input_text":
		if len(h.State) > 30 {
			return "\"" + h.State[:27] + "...\""
		}
		return "\"" + h.State + "\""
	case "input_select":
		return h.State
	case "input_datetime":
		return h.State
	case "timer":
		return f.formatTimerState(h)
	case "schedule":
		return h.State
	default:
		return h.State
	}
}

func (f *NaturalHelperFormatter) formatTimerState(h homeassistant.Entity) string {
	state := h.State
	switch state {
	case "idle":
		duration := GetStringAttr(h.Attributes, "duration")
		if duration != "" {
			return fmt.Sprintf("idle (%s default)", duration)
		}
		return "idle"
	case "active":
		remaining := GetStringAttr(h.Attributes, "remaining")
		if remaining != "" {
			return fmt.Sprintf("active, %s remaining", remaining)
		}
		return "active"
	case "paused":
		remaining := GetStringAttr(h.Attributes, "remaining")
		if remaining != "" {
			return fmt.Sprintf("paused, %s remaining", remaining)
		}
		return "paused"
	default:
		return state
	}
}

func (f *NaturalHelperFormatter) writeVerboseDetails(result *strings.Builder, h homeassistant.Entity) {
	helperType := f.extractType(h.EntityID)

	switch helperType {
	case "counter":
		if minVal := GetFloatAttr(h.Attributes, "minimum"); minVal != 0 {
			fmt.Fprintf(result, "    min: %.0f", minVal)
		}
		if maxVal := GetFloatAttr(h.Attributes, "maximum"); maxVal != 0 {
			fmt.Fprintf(result, ", max: %.0f", maxVal)
		}
		if step := GetFloatAttr(h.Attributes, "step"); step != 0 {
			fmt.Fprintf(result, ", step: %.0f", step)
		}
		result.WriteString("\n")
	case "input_number":
		minVal := GetFloatAttr(h.Attributes, "min")
		maxVal := GetFloatAttr(h.Attributes, "max")
		if minVal != 0 || maxVal != 0 {
			fmt.Fprintf(result, "    range: %.1f - %.1f\n", minVal, maxVal)
		}
	case "input_select":
		if options, ok := h.Attributes["options"].([]any); ok && len(options) > 0 {
			var optStrs []string
			for _, opt := range options {
				if s, ok := opt.(string); ok {
					optStrs = append(optStrs, s)
				}
			}
			fmt.Fprintf(result, "    options: %s\n", strings.Join(optStrs, ", "))
		}
	}
}

func (f *NaturalHelperFormatter) writeScheduleBlocks(result *strings.Builder, schedule map[string][]map[string]string) {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	dayAbbrevs := map[string]string{
		"monday": "Mon", "tuesday": "Tue", "wednesday": "Wed",
		"thursday": "Thu", "friday": "Fri", "saturday": "Sat", "sunday": "Sun",
	}

	result.WriteString("Schedule:\n")

	for _, day := range days {
		if blocks, ok := schedule[day]; ok && len(blocks) > 0 {
			var times []string
			for _, block := range blocks {
				from := block["from"]
				to := block["to"]
				if from != "" && to != "" {
					times = append(times, fmt.Sprintf("%s-%s", from, to))
				}
			}
			if len(times) > 0 {
				fmt.Fprintf(result, "  %s: %s\n", dayAbbrevs[day], strings.Join(times, ", "))
			}
		}
	}
}

func (f *NaturalHelperFormatter) getDetailString(detail map[string]any, key string) string {
	if v, ok := detail[key].(string); ok {
		return v
	}
	return ""
}

// =============================================================================
// JSON Formatter
// =============================================================================

// JSONHelperFormatter produces JSON output for helper data.
type JSONHelperFormatter struct{}

// NewJSONHelperFormatter creates a new JSONHelperFormatter.
func NewJSONHelperFormatter() *JSONHelperFormatter {
	return &JSONHelperFormatter{}
}

// FormatList formats a list of helpers as JSON.
func (f *JSONHelperFormatter) FormatList(
	_ context.Context,
	helpers []homeassistant.Entity,
	_ HelperListOptions,
) (string, error) {
	if helpers == nil {
		helpers = []homeassistant.Entity{}
	}

	// Create compact output for list
	type compactHelper struct {
		EntityID     string `json:"entity_id"`
		State        string `json:"state"`
		FriendlyName string `json:"friendly_name,omitempty"`
		HelperType   string `json:"helper_type"`
	}

	compact := make([]compactHelper, 0, len(helpers))
	for _, h := range helpers {
		helperType := ""
		if parts := strings.SplitN(h.EntityID, ".", 2); len(parts) > 0 {
			helperType = parts[0]
		}

		friendlyName := ""
		if fn, ok := h.Attributes["friendly_name"].(string); ok {
			friendlyName = fn
		}

		compact = append(compact, compactHelper{
			EntityID:     h.EntityID,
			State:        h.State,
			FriendlyName: friendlyName,
			HelperType:   helperType,
		})
	}

	data, err := json.MarshalIndent(compact, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal helpers: %w", err)
	}
	return string(data), nil
}

// FormatScheduleDetail formats schedule details as JSON.
func (f *JSONHelperFormatter) FormatScheduleDetail(
	_ context.Context,
	detail map[string]any,
) (string, error) {
	if detail == nil {
		detail = map[string]any{}
	}

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal schedule detail: %w", err)
	}
	return string(data), nil
}

// FormatCounterDetail formats counter details in JSON.
func (f *JSONHelperFormatter) FormatCounterDetail(
	_ context.Context,
	detail map[string]any,
) (string, error) {
	if detail == nil {
		detail = map[string]any{}
	}

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal counter detail: %w", err)
	}
	return string(data), nil
}

// FormatTimerDetail formats timer details in JSON.
func (f *JSONHelperFormatter) FormatTimerDetail(
	_ context.Context,
	detail map[string]any,
) (string, error) {
	if detail == nil {
		detail = map[string]any{}
	}

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal timer detail: %w", err)
	}
	return string(data), nil
}

// FormatHelperDetail formats helper details for all other helper types in JSON.
func (f *JSONHelperFormatter) FormatHelperDetail(
	_ context.Context,
	_ string,
	detail map[string]any,
) (string, error) {
	if detail == nil {
		detail = map[string]any{}
	}

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal helper detail: %w", err)
	}
	return string(data), nil
}

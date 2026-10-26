package handlers

import (
	"slices"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// validatableSchemaTypes are the property types mcp.InvalidArguments checks.
var validatableSchemaTypes = []string{"string", "integer", "number", "boolean", "array", "object"}

// untypedByDesign lists the "tool:argument" properties that deliberately omit
// Type, with the reason. mcp.JSONSchema has no oneOf/anyOf, so a property that
// accepts several JSON shapes is left untyped and mcp.InvalidArguments skips
// it; its handler validates the shape. Must stay small: a property with one
// shape needs a Type instead. Every template action argument (tplAction) is
// untyped for the same reason and is derived by templateActionArgs.
var untypedByDesign = map[string]string{
	"manage_helper:initial":     "bool for input_boolean, number for input_number, whole number for counter, string for input_text/input_select/input_datetime",
	"manage_helper:window_size": "a sample count (number) or a duration (string or object) depending on the filter type",
	"analyze_target:entity_id":  "a string or an array of strings (parseTargetIDList)",
	"analyze_target:device_id":  "a string or an array of strings (parseTargetIDList)",
	"analyze_target:area_id":    "a string or an array of strings (parseTargetIDList)",
	"analyze_target:label_id":   "a string or an array of strings (parseTargetIDList)",
}

// templateActionArgs returns the manage_helper arguments that carry an HA
// action value: a single action object or a list of them.
func templateActionArgs() map[string]bool {
	args := make(map[string]bool)
	collect := func(fields []templateField) {
		for _, f := range fields {
			if f.kind == tplAction {
				args[f.arg] = true
			}
		}
	}
	for _, subtype := range templateSubtypes {
		collect(subtype.fields)
	}
	collect(templateUniversalFields)
	return args
}

// TestToolSchemas_EveryPropertyIsValidatable pins the contract the server-side
// argument validation relies on: every property of every registered tool
// declares a type the validator understands (or is listed in untypedByDesign),
// and an enum only appears on a string property. A property without a type
// would silently skip validation.
func TestToolSchemas_EveryPropertyIsValidatable(t *testing.T) {
	t.Parallel()

	actionArgs := templateActionArgs()
	seenUntyped := make(map[string]bool)
	for _, tool := range auditRegisteredTools() {
		if tool.InputSchema.Type != "object" {
			t.Errorf("%s: InputSchema.Type = %q, want \"object\"", tool.Name, tool.InputSchema.Type)
		}
		for name, prop := range tool.InputSchema.Properties {
			key := tool.Name + ":" + name
			if prop.Type == "" {
				seenUntyped[key] = true
				_, listed := untypedByDesign[key]
				isTemplateAction := tool.Name == "manage_helper" && actionArgs[name]
				if !listed && !isTemplateAction {
					t.Errorf("%s.%s: has no type, so its value cannot be validated; declare one, or list it in untypedByDesign with the reason", tool.Name, name)
				}
			} else if !slices.Contains(validatableSchemaTypes, prop.Type) {
				t.Errorf("%s.%s: type %q cannot be validated, want one of %v", tool.Name, name, prop.Type, validatableSchemaTypes)
			}
			if len(prop.Enum) > 0 && prop.Type != "string" {
				t.Errorf("%s.%s: enum on a %q property is not validated, want type \"string\"", tool.Name, name, prop.Type)
			}
		}
	}
	for key := range untypedByDesign {
		if !seenUntyped[key] {
			t.Errorf("untypedByDesign lists %s, but that property is typed or gone; remove the entry", key)
		}
	}
}

// TestManageHelper_StateCharacteristicEnumCoversBinarySensor pins that the
// enum accepts every statistics characteristic Home Assistant supports for a
// binary_sensor source; the server rejects a value the enum omits, and
// count_on/count_off were once missing.
func TestManageHelper_StateCharacteristicEnumCoversBinarySensor(t *testing.T) {
	t.Parallel()

	var enum []string
	for _, tool := range auditRegisteredTools() {
		if tool.Name == "manage_helper" {
			enum = tool.InputSchema.Properties["state_characteristic"].Enum
		}
	}
	// STATS_BINARY_SUPPORT in HA core's statistics sensor.
	for _, want := range []string{
		"count", "count_on", "count_off", "mean", "average_step", "average_timeless", "datetime_newest", "datetime_oldest",
		// Numeric characteristics: percentile has its own `percentile` property.
		"percentile", "standard_deviation", "sum", "sum_differences", "sum_differences_nonnegative",
		"mean_circular", "noisiness", "distance_95_percent_of_values", "distance_99_percent_of_values", "distance_absolute",
	} {
		if !slices.Contains(enum, want) {
			t.Errorf("state_characteristic enum lacks %q", want)
		}
	}
}

// TestToolSchemas_AcceptValidClientCalls pins calls a handler or Home Assistant
// accepts that a narrower schema once would have made the server reject: an
// enum that omits a value the handler forwards, a case-insensitive value, and
// a property that takes a string as well as an array.
func TestToolSchemas_AcceptValidClientCalls(t *testing.T) {
	t.Parallel()

	tools := make(map[string]mcp.Tool)
	for _, tool := range auditRegisteredTools() {
		tools[tool.Name] = tool
	}

	tests := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"system log level in upper case", "manage_system_log", map[string]any{"action": "list", "level": "ERROR"}},
		{"analyze_target bare string entity_id", "analyze_target", map[string]any{"entity_id": "light.kitchen"}},
		{"analyze_target bare string device_id", "analyze_target", map[string]any{"device_id": "abc"}},
		{"analyze_target bare string area_id", "analyze_target", map[string]any{"area_id": "kitchen"}},
		{"analyze_target bare string label_id", "analyze_target", map[string]any{"label_id": "lights"}},
		{"analyze_target list entity_id", "analyze_target", map[string]any{"entity_id": []any{"light.kitchen"}}},
		{"utility_meter that never resets", "manage_helper", map[string]any{"cycle": "none"}},
		{"statistics percentile", "manage_helper", map[string]any{"state_characteristic": "percentile", "percentile": float64(95)}},
		{"statistics standard_deviation", "manage_helper", map[string]any{"state_characteristic": "standard_deviation"}},
		{"media_player group", "manage_helper", map[string]any{"group_type": "media_player"}},
		{"HACS template category", "manage_hacs", map[string]any{"category": "template"}},
		{"calendar THISANDFUTURE range", "manage_calendar", map[string]any{"recurrence_range": "THISANDFUTURE"}},
		{"calendar THIS_AND_FUTURE range", "manage_calendar", map[string]any{"recurrence_range": "THIS_AND_FUTURE"}},
		{"empty array for a scalar", "get_logbook", map[string]any{"mode": []any{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tool, ok := tools[tt.tool]
			if !ok {
				t.Fatalf("tool %q is not registered", tt.tool)
			}
			if got := mcp.InvalidArguments(tool.InputSchema, tt.args); got != nil {
				t.Errorf("%s %v is rejected: %v", tt.tool, tt.args, got)
			}
		})
	}
}

// TestToolSchemas_DefaultsPassValidation checks the validator against real
// schema values: a property's declared default must itself be a valid argument.
func TestToolSchemas_DefaultsPassValidation(t *testing.T) {
	t.Parallel()

	for _, tool := range auditRegisteredTools() {
		for name, prop := range tool.InputSchema.Properties {
			if prop.Default == nil {
				continue
			}
			args := map[string]any{name: prop.Default}
			if got := mcp.InvalidArguments(tool.InputSchema, args); got != nil {
				t.Errorf("%s: default %#v of %q is rejected: %v", tool.Name, prop.Default, name, got)
			}
		}
	}
}

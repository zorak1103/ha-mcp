package handlers

import (
	"testing"
)

// helperArgKind describes the Go type a manage_helper config builder
// demands for a given schema property. This table is authored independently
// of both the JSON schema (helpers_consolidated.go's manageHelperTool,
// helpers_schema_extended.go's buildExtendedHelperProperties) and the
// builders that read args - see realHelperStorageConfig's doc comment
// (helpers_consolidated_test.go) for why an independently-authored model is
// the point: a table generated from either side could never disagree with
// it, making the test a tautology.
type helperArgKind int

const (
	kindString helperArgKind = iota
	kindNumber
	kindInt
	kindBool
	kindStringArray
	kindObjectArray
	// kindPolymorphic marks a field whose Go type legitimately varies by
	// helper type (initial) or by a value read from a nested field the
	// table can't see (filter's window_size - see hybrid_client.go's
	// toDurationDict). Its schema Type MUST be omitted.
	kindPolymorphic
)

// helperArgKinds is the default kind for every manage_helper schema
// property that isn't dispatch-only (see nonBuilderSchemaFields) and whose
// kind doesn't vary by helper type (see helperArgKindOverrides).
var helperArgKinds = map[string]helperArgKind{
	"name":                    kindString,
	"icon":                    kindString,
	"initial":                 kindPolymorphic,
	"min":                     kindNumber,
	"max":                     kindNumber,
	"step":                    kindNumber,
	"mode":                    kindString,
	"unit_of_measurement":     kindString,
	"pattern":                 kindString,
	"options":                 kindStringArray,
	"has_date":                kindBool,
	"has_time":                kindBool,
	"minimum":                 kindNumber,
	"maximum":                 kindNumber,
	"duration":                kindString,
	"restore":                 kindBool,
	"monday":                  kindObjectArray,
	"tuesday":                 kindObjectArray,
	"wednesday":               kindObjectArray,
	"thursday":                kindObjectArray,
	"friday":                  kindObjectArray,
	"saturday":                kindObjectArray,
	"sunday":                  kindObjectArray,
	"entities":                kindStringArray,
	"all":                     kindBool,
	"group_type":              kindString,
	"state":                   kindString,
	"device_class":            kindString,
	"state_class":             kindString,
	"delay_on":                kindString,
	"delay_off":               kindString,
	"lower":                   kindNumber,
	"upper":                   kindNumber,
	"hysteresis":              kindNumber,
	"source":                  kindString,
	"round":                   kindInt,
	"time_window":             kindString,
	"unit_time":               kindString,
	"unit_prefix":             kindString,
	"method":                  kindString,
	"entity_id":               kindString,
	"cycle":                   kindString,
	"offset":                  kindNumber,
	"delta_values":            kindBool,
	"net_consumption":         kindBool,
	"periodically_resetting":  kindBool,
	"tariffs":                 kindStringArray,
	"entity_ids":              kindStringArray,
	"round_digits":            kindInt,
	"min_max_type":            kindString,
	"state_characteristic":    kindString,
	"sampling_size":           kindInt,
	"max_age":                 kindString,
	"percentile":              kindNumber,
	"precision":               kindInt,
	"min_gradient":            kindNumber,
	"min_samples":             kindInt,
	"sample_duration":         kindNumber,
	"max_samples":             kindInt,
	"invert":                  kindBool,
	"filter":                  kindString,
	"window_size":             kindPolymorphic,
	"radius":                  kindNumber,
	"time_constant":           kindNumber,
	"lower_bound":             kindNumber,
	"upper_bound":             kindNumber,
	"after_time":              kindString,
	"before_time":             kindString,
	"after_offset":            kindString,
	"before_offset":           kindString,
	"heater_entity_id":        kindString,
	"target_sensor_entity_id": kindString,
	"ac_mode":                 kindBool,
	"min_temp":                kindNumber,
	"max_temp":                kindNumber,
	"target_temp":             kindNumber,
	"cold_tolerance":          kindNumber,
	"hot_tolerance":           kindNumber,
	"target_domain":           kindString,
	"humidifier_entity_id":    kindString,
	"min_humidity":            kindNumber,
	"max_humidity":            kindNumber,
	"target_humidity":         kindNumber,
	"dry_tolerance":           kindNumber,
	"wet_tolerance":           kindNumber,
}

// helperArgKindOverrides handles args whose kind depends on the helper
// type being built. "initial" is the canonical case: input_boolean reads
// it as a bool, input_number as a number, counter as a whole number, and
// the three string-valued input_* types as a string.
var helperArgKindOverrides = map[string]map[string]helperArgKind{
	"input_boolean":  {"initial": kindBool},
	"input_number":   {"initial": kindNumber},
	"counter":        {"initial": kindInt},
	"input_text":     {"initial": kindString},
	"input_select":   {"initial": kindString},
	"input_datetime": {"initial": kindString},
}

// nonBuilderSchemaFields are manage_helper schema properties consumed by
// action dispatch (handleManageHelper/handleCreate/handleUpdate/...)
// rather than by any config builder, so they have no meaningful "builder
// arg kind" and are exempt from both agreement-test phases.
var nonBuilderSchemaFields = map[string]bool{
	"action":  true,
	"format":  true,
	"verbose": true,
	"type":    true,
	"id":      true,
}

func kindForType(typeName, field string) (helperArgKind, bool) {
	if perType, ok := helperArgKindOverrides[typeName]; ok {
		if kind, ok := perType[field]; ok {
			return kind, true
		}
	}
	kind, ok := helperArgKinds[field]
	return kind, ok
}

func schemaTypeFor(kind helperArgKind) string {
	switch kind {
	case kindString:
		return "string"
	case kindNumber, kindInt:
		return "number"
	case kindBool:
		return "boolean"
	case kindStringArray, kindObjectArray:
		return "array"
	case kindPolymorphic:
		return ""
	default:
		return ""
	}
}

// TestHelperSchemaTypes_MatchBuilderArgTypes pins the manage_helper JSON
// schema's declared property Type against the Go type the config builders
// actually demand for that property. Before the fix for the input_number
// "initial" bug (tmp/issue.md) this failed on exactly four properties:
// initial, time_window, delay_on, delay_off - each declared "string" in
// the schema while at least one builder read it as a number.
func TestHelperSchemaTypes_MatchBuilderArgTypes(t *testing.T) {
	t.Parallel()

	props := NewConsolidatedHelperHandlers().manageHelperTool().InputSchema.Properties

	t.Run("table completeness", func(t *testing.T) {
		t.Parallel()

		for name := range props {
			if nonBuilderSchemaFields[name] {
				continue
			}
			if _, ok := helperArgKinds[name]; !ok {
				t.Errorf("schema property %q has no entry in helperArgKinds - add one", name)
			}
		}
		for name := range helperArgKinds {
			if _, ok := props[name]; !ok {
				t.Errorf("helperArgKinds has an entry for %q which is not a manage_helper schema property", name)
			}
		}
	})

	t.Run("declared kind matches schema Type", func(t *testing.T) {
		t.Parallel()

		for name, kind := range helperArgKinds {
			prop, ok := props[name]
			if !ok {
				continue // reported by the completeness subtest
			}
			want := schemaTypeFor(kind)
			if prop.Type != want {
				t.Errorf(
					"schema property %q declares Type=%q but helperArgKinds says %v (want Type=%q)",
					name, prop.Type, kind, want,
				)
			}
			if kind == kindStringArray || kind == kindObjectArray {
				if prop.Items == nil {
					t.Errorf("array property %q has no Items schema", name)
				}
			}
		}
	})
}

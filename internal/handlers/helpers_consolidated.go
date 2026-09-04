// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Platform constants for helpers.
const (
	platformInputBoolean           = "input_boolean"
	platformInputNumber            = "input_number"
	platformInputText              = "input_text"
	platformInputSelect            = "input_select"
	platformInputDatetime          = "input_datetime"
	platformInputButton            = "input_button"
	platformCounter                = "counter"
	platformTimer                  = "timer"
	platformSchedule               = "schedule"
	platformGroup                  = "group"
	platformIntegration            = "integration"
	platformSensorEntity           = "sensor"
	platformBinarySensorEntity     = "binary_sensor"
	platformTemplate               = "template"
	helperTypeTemplateSensor       = "template_sensor"
	helperTypeTemplateBinarySensor = "template_binary_sensor"
	platformUtilityMeter           = "utility_meter"
	platformMinMax                 = "min_max"
	platformStatistics             = "statistics"
	platformTrend                  = "trend"
	platformRandom                 = "random"
	platformFilter                 = "filter"
	platformTod                    = "tod"
	platformGenericThermostat      = "generic_thermostat"
	platformSwitchAsX              = "switch_as_x"
	platformGenericHygrostat       = "generic_hygrostat"
	helperTypeRandomSensor         = "random_sensor"
	helperTypeRandomBinarySensor   = "random_binary_sensor"
	serviceSetValue                = "set_value"
	helperActionUpdate             = "update"
	hygrostatEntityDomain          = "humidifier" // generic_hygrostat's validEntityDomains entry - an entity domain, not an integration platform name
	// thermostatEntityDomain is generic_thermostat's validEntityDomains
	// entry - gates addExtendedConfigEntryFields' preset-temperature fields
	// to this platform only. Unlike the min_max_type gate, which resolves
	// the real integration PLATFORM via the entity registry because a
	// domain can host more than one platform, this gates on the entity
	// DOMAIN directly - correct only because "climate" is used by exactly
	// one helperTypes entry (generic_thermostat). If a second config-entry
	// helper type is ever added under the "climate" domain, this gate would
	// need the same registry-platform resolution min_max_type uses instead
	// of a plain domain comparison. TestThermostatEntityDomain_IsUniqueAcrossHelperTypes
	// pins the single-owner assumption so that addition fails a test rather
	// than silently leaking preset fields into the new type's updates.
	thermostatEntityDomain = "climate"
	// hygrostatDeviceClass is generic_hygrostat's required device_class
	// config value - a different field than hygrostatEntityDomain above
	// (that one identifies the entity's domain; this one is a value written
	// into the helper's own config), which happens to share the same
	// literal. Kept as one named constant so the two create/update builders
	// that both need it don't drift into differently-named local copies of
	// the same string.
	hygrostatDeviceClass = "humidifier"
)

// sourceEntityConstraint restricts one args field of a helper type to a set
// of allowed entity domains (e.g. generic_thermostat's heater_entity_id must
// be a switch.*). A helper type can have more than one constrained field -
// generic_thermostat and generic_hygrostat each constrain both their
// actuator field and their target_sensor_entity_id field.
type sourceEntityConstraint struct {
	field         string   // args key holding the source entity_id; never empty
	domains       []string // allowed domains for that entity_id; never empty
	deviceClasses []string // allowed device_class values within domains, or empty for unconstrained
}

// helperTypeMetadata defines metadata for each helper type.
type helperTypeMetadata struct {
	platform           string                   // Home Assistant platform name
	entityPrefix       string                   // Prefix for resulting entity_id (e.g., "sensor" for derivative)
	supportedActions   []string                 // Actions supported by this helper type
	requiredFields     []string                 // Fields required for create operation
	optionalFields     []string                 // Optional fields for create operation
	validEntityDomains []string                 // Valid domains for delete/action validation
	sourceEntities     []sourceEntityConstraint // Domain constraints on source-entity args fields; empty = unconstrained
}

// perTypeUpdateExcludedFields lists (helper type -> field) exclusions that
// apply only to that specific type, because a sibling type sharing the same
// field name doesn't have the same problem: filter's "filter" field is a
// create-only selector the update builder never reads on update (see
// CLAUDE.md's "manage_helper update field docs" gotcha) - not a
// storage-config gap, since GetHelperConfig's "<platform>/list" does return
// it; the update builder just never forwards it.
var perTypeUpdateExcludedFields = buildPerTypeUpdateExcludedFields()

// buildPerTypeUpdateExcludedFields returns the base per-type update-excluded
// field map, merged with device_class exclusions for template subtypes whose
// device_class is config-flow-only (not present in their OPTIONS_FLOW schema).
func buildPerTypeUpdateExcludedFields() map[string]map[string]bool {
	m := map[string]map[string]bool{
		"filter": {"filter": true},
	}
	for typeName := range perTypeDeviceClassSupport {
		if perTypeDeviceClassUpdateSupport[typeName] {
			// Also declared in this type's OPTIONS_FLOW schema (currently
			// only template_number) - not excluded from update.
			continue
		}
		if m[typeName] == nil {
			m[typeName] = map[string]bool{}
		}
		m[typeName]["device_class"] = true
	}
	return m
}

// isUpdateIdentifierField reports whether field is the tool's own "which
// helper are we updating" identifier for the update action (handleUpdate
// reads args["entity_id"] for exactly that), rather than a per-platform
// config value. "entity_id" is also a legitimate create-time config field
// for several types (statistics, trend, filter, switch_as_x, threshold) -
// on update the identifier meaning always wins, so no per-platform value
// can ever be forwarded under that name. isUpdateExcludedField,
// updatableSourceEntities, and buildConfigEntryUpdateConfig (which simply
// never reads args["entity_id"]) all derive from this single answer.
func isUpdateIdentifierField(field string) bool {
	return field == attrEntityID
}

// isUpdateExcludedField reports whether field is never forwarded to HA on
// update for helper type typeName.
//
// TestUpdatableFields_AreActuallyReadByUpdatePath enforces that every
// remaining name updatableFieldNames() returns actually round-trips through
// the real update builder, so a future field that stops being read on
// update fails a test rather than silently drifting from the generated
// docs.
func isUpdateExcludedField(typeName, field string) bool {
	return isUpdateIdentifierField(field) || perTypeUpdateExcludedFields[typeName][field]
}

// updatableFieldNames returns the field names accepted on update for this
// helper type (identified by typeName, its key in helperTypes):
// requiredFields (required only at create time) plus optionalFields, minus
// whatever isUpdateExcludedField reports as never forwarded to HA on
// update.
func updatableFieldNames(typeName string) []string {
	meta := helperTypes[typeName]
	all := append(append([]string{}, meta.requiredFields...), meta.optionalFields...)
	names := make([]string, 0, len(all))
	for _, name := range all {
		if isUpdateExcludedField(typeName, name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

// updatableFieldsDescription renders a compact per-type "what update
// accepts" reference, generated from helperTypes so it can never drift
// from the code — same intent as manage_entity/manage_device's generated
// "Safe fields" lists. "icon" is omitted from every per-type line (it's
// accepted on all 41 types, so listing it 41 times would triple this
// block's size for zero information) - the caller documents it once instead.
//
// Renders to roughly 3.3KB across the 41 types (issue #206's 15 template_*
// subtypes account for more than half of that), added to manage_helper's
// tool description on every tools/list call. Only one group of types
// (input_boolean/input_button/random_binary_sensor) shares an identical
// remaining field set (icon-only, i.e. empty after the icon hoist) - not
// enough duplication across the other types to justify grouping
// identical-field-set types onto shared lines; revisit if a future helper
// type addition changes that balance.
func updatableFieldsDescription() string {
	names := make([]string, 0, len(helperTypes))
	for name := range helperTypes {
		names = append(names, name)
	}
	slices.Sort(names)

	var b strings.Builder
	for _, name := range names {
		updatable := updatableFieldNames(name)
		fields := make([]string, 0, len(updatable))
		for _, field := range updatable {
			if field != "icon" {
				fields = append(fields, field)
			}
		}
		fmt.Fprintf(&b, "\n  - %s: %s", name, strings.Join(fields, ", "))
	}
	return b.String()
}

// helperTypes contains metadata for all supported helper types.
var helperTypes = buildHelperTypesRegistry()

// buildInputHelperTypesGroup returns metadata for the basic WS-backed input_* helpers.
func buildInputHelperTypesGroup() map[string]helperTypeMetadata {
	return map[string]helperTypeMetadata{
		"input_boolean": {
			platform:           "input_boolean",
			entityPrefix:       "input_boolean",
			supportedActions:   []string{"toggle"},
			requiredFields:     []string{},
			optionalFields:     []string{"icon", "initial"},
			validEntityDomains: []string{"input_boolean"},
		},
		"input_number": {
			platform:           "input_number",
			entityPrefix:       "input_number",
			supportedActions:   []string{"set"},
			requiredFields:     []string{"min", "max"},
			optionalFields:     []string{"icon", "step", "initial", "mode", "unit_of_measurement"},
			validEntityDomains: []string{"input_number"},
		},
		"input_text": {
			platform:           "input_text",
			entityPrefix:       "input_text",
			supportedActions:   []string{"set"},
			requiredFields:     []string{},
			optionalFields:     []string{"icon", "min", "max", "mode", "pattern", "initial"},
			validEntityDomains: []string{"input_text"},
		},
		"input_select": {
			platform:           "input_select",
			entityPrefix:       "input_select",
			supportedActions:   []string{"select", "set_options"},
			requiredFields:     []string{"options"},
			optionalFields:     []string{"icon", "initial"},
			validEntityDomains: []string{"input_select"},
		},
		"input_datetime": {
			platform:           "input_datetime",
			entityPrefix:       "input_datetime",
			supportedActions:   []string{"set"},
			requiredFields:     []string{},
			optionalFields:     []string{"icon", "has_date", "has_time", "initial"},
			validEntityDomains: []string{"input_datetime"},
		},
		"input_button": {
			platform:           "input_button",
			entityPrefix:       "input_button",
			supportedActions:   []string{"press"},
			requiredFields:     []string{},
			optionalFields:     []string{"icon"},
			validEntityDomains: []string{"input_button"},
		},
	}
}

// buildStateHelperTypesGroup returns metadata for the remaining WS-backed
// state helpers (counter/timer/schedule/group) plus the two base template_*
// helper types (template_sensor/template_binary_sensor - not to be confused
// with the newer template_* subtypes merged in via templateHelperTypes()).
func buildStateHelperTypesGroup() map[string]helperTypeMetadata {
	return map[string]helperTypeMetadata{
		"counter": {
			platform:           "counter",
			entityPrefix:       "counter",
			supportedActions:   []string{"increment", "decrement", "reset", "set"},
			requiredFields:     []string{},
			optionalFields:     []string{"icon", "initial", "step", "minimum", "maximum", "restore"},
			validEntityDomains: []string{"counter"},
		},
		"timer": {
			platform:           "timer",
			entityPrefix:       "timer",
			supportedActions:   []string{"start", "pause", "cancel", "finish", "change"},
			requiredFields:     []string{},
			optionalFields:     []string{"icon", "duration", "restore"},
			validEntityDomains: []string{"timer"},
		},
		"schedule": {
			platform:           "schedule",
			entityPrefix:       "schedule",
			supportedActions:   []string{"reload"},
			requiredFields:     []string{},
			optionalFields:     []string{"icon", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"},
			validEntityDomains: []string{"schedule"},
		},
		"group": {
			platform:           "group",
			entityPrefix:       "group",
			supportedActions:   []string{"add_entities", "remove_entities", "reload"},
			requiredFields:     []string{"entities"},
			optionalFields:     []string{"icon", "all", "group_type"},
			validEntityDomains: []string{"group"},
		},
		"template_sensor": {
			platform:           platformTemplate,
			entityPrefix:       "sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"state"},
			optionalFields:     []string{"icon", "unit_of_measurement", "device_class", "state_class"},
			validEntityDomains: []string{"sensor"},
		},
		"template_binary_sensor": {
			platform:           platformTemplate,
			entityPrefix:       "binary_sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"state"},
			optionalFields:     []string{"icon", "device_class", "delay_on", "delay_off"},
			validEntityDomains: []string{"binary_sensor"},
		},
	}
}

// buildSensorCalcHelperTypesGroup returns metadata for the Config Entry
// helpers that derive a sensor/binary_sensor from a single source entity.
func buildSensorCalcHelperTypesGroup() map[string]helperTypeMetadata {
	return map[string]helperTypeMetadata{
		"threshold": {
			platform:           "threshold",
			entityPrefix:       "binary_sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"entity_id"},
			optionalFields:     []string{"icon", "lower", "upper", "hysteresis", "device_class"},
			validEntityDomains: []string{"binary_sensor"},
		},
		"derivative": {
			platform:           "derivative",
			entityPrefix:       "sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"source"},
			optionalFields:     []string{"icon", "round", "time_window", "unit_time", "unit_prefix"},
			validEntityDomains: []string{"sensor"},
		},
		"integral": {
			platform:           "integration",
			entityPrefix:       "sensor",
			supportedActions:   []string{"reset"},
			requiredFields:     []string{"source"},
			optionalFields:     []string{"icon", "method", "round", "unit_time", "unit_prefix"},
			validEntityDomains: []string{"sensor"},
		},
		"utility_meter": {
			platform:           platformUtilityMeter,
			entityPrefix:       "sensor",
			supportedActions:   []string{"calibrate"},
			requiredFields:     []string{"source"},
			optionalFields:     []string{"icon", "cycle", "offset", "delta_values", "net_consumption", "periodically_resetting", "tariffs"},
			validEntityDomains: []string{"sensor", "select"},
			sourceEntities:     []sourceEntityConstraint{{field: "source", domains: []string{"sensor"}}},
		},
		"min_max": {
			platform:           platformMinMax,
			entityPrefix:       "sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"entity_ids", "min_max_type"},
			optionalFields:     []string{"icon", "round_digits"},
			validEntityDomains: []string{"sensor"},
		},
	}
}

// buildSensorAggregateHelperTypesGroup returns metadata for the remaining
// Config Entry sensor/binary_sensor helpers (statistics, trend, random,
// filter, time-of-day).
func buildSensorAggregateHelperTypesGroup() map[string]helperTypeMetadata {
	return map[string]helperTypeMetadata{
		"statistics": {
			platform:           platformStatistics,
			entityPrefix:       "sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"entity_id"},
			optionalFields:     []string{"icon", "state_characteristic", "sampling_size", "max_age", "percentile", "precision"},
			validEntityDomains: []string{"sensor"},
			sourceEntities:     []sourceEntityConstraint{{field: attrEntityID, domains: []string{"sensor", "binary_sensor"}}},
		},
		"trend": {
			platform:           platformTrend,
			entityPrefix:       "binary_sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"entity_id"},
			optionalFields:     []string{"icon", "min_gradient", "min_samples", "sample_duration", "max_samples", "invert"},
			validEntityDomains: []string{"binary_sensor"},
			sourceEntities:     []sourceEntityConstraint{{field: attrEntityID, domains: []string{"sensor", "counter"}}},
		},
		"random_sensor": {
			platform:           platformRandom,
			entityPrefix:       "sensor",
			supportedActions:   []string{},
			requiredFields:     []string{},
			optionalFields:     []string{"icon", "minimum", "maximum"},
			validEntityDomains: []string{"sensor"},
		},
		"random_binary_sensor": {
			platform:           platformRandom,
			entityPrefix:       "binary_sensor",
			supportedActions:   []string{},
			requiredFields:     []string{},
			optionalFields:     []string{"icon"},
			validEntityDomains: []string{"binary_sensor"},
		},
		"filter": {
			platform:           platformFilter,
			entityPrefix:       "sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"entity_id", "filter"},
			optionalFields:     []string{"icon", "window_size", "radius", "time_constant", "lower_bound", "upper_bound", "precision"},
			validEntityDomains: []string{"sensor"},
			sourceEntities:     []sourceEntityConstraint{{field: attrEntityID, domains: []string{"sensor"}}},
		},
		"tod": {
			platform:           platformTod,
			entityPrefix:       "binary_sensor",
			supportedActions:   []string{},
			requiredFields:     []string{"after_time", "before_time"},
			optionalFields:     []string{"icon", "after_offset", "before_offset"},
			validEntityDomains: []string{"binary_sensor"},
		},
	}
}

// buildClimateHelperTypesGroup returns metadata for the climate/humidifier/
// switch_as_x Config Entry helpers.
func buildClimateHelperTypesGroup() map[string]helperTypeMetadata {
	return map[string]helperTypeMetadata{
		"generic_thermostat": {
			platform:         platformGenericThermostat,
			entityPrefix:     "climate",
			supportedActions: []string{},
			requiredFields:   []string{"heater_entity_id", "target_sensor_entity_id"},
			// slices.Clip drops the spare capacity append() leaves on a
			// full-literal slice, so a future in-place append elsewhere can't
			// silently share/mutate this backing array.
			optionalFields: slices.Clip(append(
				[]string{"icon", "ac_mode", "min_temp", "max_temp", "target_temp", "cold_tolerance", "hot_tolerance"},
				genericThermostatPresetFieldNames()...,
			)),
			validEntityDomains: []string{"climate"},
			sourceEntities: []sourceEntityConstraint{
				{field: "heater_entity_id", domains: []string{"switch", "fan"}},
				{field: "target_sensor_entity_id", domains: []string{"sensor"}, deviceClasses: []string{"temperature"}},
			},
		},
		"switch_as_x": {
			platform:           platformSwitchAsX,
			entityPrefix:       "light",
			supportedActions:   []string{},
			requiredFields:     []string{"entity_id", "target_domain"},
			optionalFields:     []string{"icon", "invert"},
			validEntityDomains: []string{"cover", "fan", "light", "lock", "siren", "valve"},
			sourceEntities:     []sourceEntityConstraint{{field: attrEntityID, domains: []string{"switch"}}},
		},
		"generic_hygrostat": {
			platform:           platformGenericHygrostat,
			entityPrefix:       "humidifier",
			supportedActions:   []string{},
			requiredFields:     []string{"humidifier_entity_id", "target_sensor_entity_id"},
			optionalFields:     []string{"icon", "min_humidity", "max_humidity", "target_humidity", "dry_tolerance", "wet_tolerance"},
			validEntityDomains: []string{"humidifier"},
			sourceEntities: []sourceEntityConstraint{
				{field: "humidifier_entity_id", domains: []string{"switch", "fan"}},
				{field: "target_sensor_entity_id", domains: []string{"sensor"}, deviceClasses: []string{"humidity"}},
			},
		},
	}
}

// buildHelperTypesRegistry returns the base helperTypeMetadata registry for
// all non-template-subtype helpers, merged with metadata for the
// template_* subtypes (see helpers_template_types.go).
func buildHelperTypesRegistry() map[string]helperTypeMetadata {
	m := map[string]helperTypeMetadata{}
	for _, group := range []map[string]helperTypeMetadata{
		buildInputHelperTypesGroup(),
		buildStateHelperTypesGroup(),
		buildSensorCalcHelperTypesGroup(),
		buildSensorAggregateHelperTypesGroup(),
		buildClimateHelperTypesGroup(),
	} {
		for typeName, meta := range group {
			m[typeName] = meta
		}
	}
	for typeName, meta := range templateHelperTypes() {
		m[typeName] = meta
	}
	return m
}

// allSupportedActions lists all valid actions for the helper_action tool.
var allSupportedActions = []string{
	"toggle", "set", "increment", "decrement", "reset",
	"start", "pause", "cancel", "finish", "change",
	"press", "select", "set_options", "reload",
	"add_entities", "remove_entities", "calibrate",
}

// allHelperTypeNames lists all valid helper types for the manage_helper create action (sorted).
var allHelperTypeNames = sortedHelperTypeNames()

// sortedHelperTypeNames returns the full, sorted list of helper type names -
// the base set plus the template_* subtype names (see helpers_template_types.go).
func sortedHelperTypeNames() []string {
	names := []string{
		"counter", "derivative", "filter", "generic_hygrostat", "generic_thermostat",
		"group", "input_boolean", "input_button", "input_datetime", "input_number",
		"input_select", "input_text", "integral", "min_max", "random_binary_sensor",
		"random_sensor", "schedule", "statistics", "switch_as_x", "template_binary_sensor",
		"template_sensor", "threshold", "timer", "tod", "trend", "utility_meter",
	}
	names = append(names, templateSubtypeNames()...)
	slices.Sort(names)
	return names
}

// ConsolidatedHelperHandlers provides unified MCP tool handlers for all helper types.
type ConsolidatedHelperHandlers struct{}

// NewConsolidatedHelperHandlers creates a new ConsolidatedHelperHandlers instance.
func NewConsolidatedHelperHandlers() *ConsolidatedHelperHandlers {
	return &ConsolidatedHelperHandlers{}
}

// RegisterTools registers the consolidated helper tools with the registry.
func (h *ConsolidatedHelperHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageHelperTool(), h.handleManageHelper)
	registry.RegisterTool(h.helperActionTool(), h.handleHelperAction)
}

// =============================================================================
// Tool Definitions
// =============================================================================

//nolint:funlen // Large schema definition requires many properties
func (h *ConsolidatedHelperHandlers) manageHelperTool() mcp.Tool {
	typeNames := make([]string, 0, len(helperTypes))
	for name := range helperTypes {
		typeNames = append(typeNames, name)
	}

	// Build base properties
	props := map[string]mcp.JSONSchema{
		"action": {
			Type:        "string",
			Description: "Operation to perform: list, create, update, delete, or get_details",
			Enum:        []string{"list", "create", "update", "delete", "get_details"},
		},
		"format": {
			Type:        "string",
			Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
			Enum:        []string{"natural", "json"},
		},
		"verbose": {
			Type:        "boolean",
			Description: "Include additional details in list output (default: false)",
		},
		"type": {
			Type:        "string",
			Description: fmt.Sprintf("Helper type (required for create) - see Enum for the full list of %d supported types", len(typeNames)),
			Enum:        typeNames,
		},
		"entity_id": {
			Type:        "string",
			Description: "Full entity ID (required for delete/get_details). Also required as source entity for threshold create.",
		},
		"id": {
			Type:        "string",
			Description: "Unique identifier without platform prefix (required for create)",
		},
		"name": {
			Type:        "string",
			Description: "Human-readable name (required for create)",
		},
		"icon": {
			Type:        "string",
			Description: "Icon for the helper (e.g., mdi:counter)",
		},
		"initial": {
			// Type intentionally omitted: the valid shape depends on the
			// helper type being created - bool for input_boolean, number for
			// input_number, whole number for counter, string for
			// input_text/input_select/input_datetime.
			Description: "Initial value: true/false (input_boolean), a number (input_number), a whole " +
				"number (counter), or a string (input_text, input_select, input_datetime)",
		},
		"min": {
			Type:        "number",
			Description: "Minimum value (required for input_number, optional for input_text)",
		},
		"max": {
			Type:        "number",
			Description: "Maximum value (required for input_number, optional for input_text)",
		},
		"step": {
			Type:        "number",
			Description: "Step value (input_number, counter)",
		},
		"mode": {
			Type:        "string",
			Description: "Display mode (input_number: box/slider, input_text: text/password)",
		},
		"unit_of_measurement": {
			Type:        "string",
			Description: "Unit of measurement (input_number, template_sensor)",
		},
		"pattern": {
			Type:        "string",
			Description: "Regex pattern for validation (input_text)",
		},
		"options": {
			Type:        "array",
			Description: "List of options (input_select, required)",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"has_date": {
			Type:        "boolean",
			Description: "Include date component. At least one of has_date or has_time must be true for input_datetime.",
		},
		"has_time": {
			Type:        "boolean",
			Description: "Include time component. At least one of has_date or has_time must be true for input_datetime.",
		},
		"minimum": {
			Type:        "number",
			Description: "Minimum allowed value (counter)",
		},
		"maximum": {
			Type:        "number",
			Description: "Maximum allowed value (counter)",
		},
		"duration": {
			Type:        "string",
			Description: "Default duration in HH:MM:SS format (timer)",
		},
		"restore": {
			Type:        "boolean",
			Description: "Restore state after restart (timer, counter)",
		},
		"monday": {
			Type:        "array",
			Description: "Time blocks for Monday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"tuesday": {
			Type:        "array",
			Description: "Time blocks for Tuesday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"wednesday": {
			Type:        "array",
			Description: "Time blocks for Wednesday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"thursday": {
			Type:        "array",
			Description: "Time blocks for Thursday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"friday": {
			Type:        "array",
			Description: "Time blocks for Friday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"saturday": {
			Type:        "array",
			Description: "Time blocks for Saturday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"sunday": {
			Type:        "array",
			Description: "Time blocks for Sunday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"entities": {
			Type:        "array",
			Description: "Entity IDs to include (group, required)",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"all": {
			Type:        "boolean",
			Description: "Require all entities to be on for group to be on (group)",
		},
		"group_type": {
			Type:        "string",
			Description: "Explicit group type override (group, optional): sensor, binary_sensor, light, cover, switch, fan, lock. If not provided, automatically inferred from member entity domains.",
			Enum:        []string{"sensor", "binary_sensor", "light", "cover", "switch", "fan", "lock"},
		},
		"state": {
			Type:        "string",
			Description: "Jinja2 template for state (template_sensor and most template_* subtypes - required for template_sensor and for most template_* subtypes, optional for template_binary_sensor/template_alarm_control_panel/template_switch)",
		},
		"device_class": {
			Type:        "string",
			Description: "Device class (template_sensor, template_binary_sensor, threshold)",
		},
		"state_class": {
			Type:        "string",
			Description: "State class (template_sensor: measurement, total, total_increasing)",
		},
		"delay_on": {
			Type:        "string",
			Description: "Delay before turning on (template_binary_sensor)",
		},
		"delay_off": {
			Type:        "string",
			Description: "Delay before turning off (template_binary_sensor)",
		},
		"lower": {
			Type:        "number",
			Description: "Lower threshold value (threshold)",
		},
		"upper": {
			Type:        "number",
			Description: "Upper threshold value (threshold)",
		},
		"hysteresis": {
			Type:        "number",
			Description: "Hysteresis value (threshold)",
		},
		"source": {
			Type:        "string",
			Description: "Source sensor entity ID (derivative, integral, required)",
		},
		"round": {
			Type:        "number",
			Description: "Decimal places to round (derivative, integral)",
		},
		"time_window": {
			Type:        "string",
			Description: "Time window for derivative calculation (derivative)",
		},
		"unit_time": {
			Type:        "string",
			Description: "Time unit: s, min, h, d (derivative, integral)",
		},
		"unit_prefix": {
			Type:        "string",
			Description: "Unit prefix: none, k, M, G, T (derivative, integral)",
		},
		"method": {
			Type:        "string",
			Description: "Integration method: trapezoidal, left, right (integral)",
		},
	}

	// Merge extended properties for new helper types
	for k, v := range buildExtendedHelperProperties() {
		props[k] = v
	}

	// Merge template subtype properties (button/cover/device_tracker/event/
	// fan/image/light/lock/number/select/switch/update/vacuum/weather/
	// alarm_control_panel) - skip any name already declared above (state,
	// min, max, step, device_class, unit_of_measurement, icon, ...) so a
	// shared field name keeps its existing single definition.
	for k, v := range templateHelperProperties() {
		if _, exists := props[k]; !exists {
			props[k] = v
		}
	}

	return mcp.Tool{
		Name: "manage_helper",
		Description: `Manage Home Assistant helpers - list, create, update, delete, or get details.

Helper Types:
- Input helpers: input_boolean, input_number, input_text, input_select, input_datetime, input_button
- Stateful helpers: counter, timer, schedule
- Entity grouping: group
- Advanced helpers: template_sensor, template_binary_sensor, threshold, derivative, integral
- Template subtypes: template_alarm_control_panel, template_button, template_cover, template_device_tracker, template_event, template_fan, template_image, template_light, template_lock, template_number, template_select, template_switch, template_update, template_vacuum, template_weather
- Utility helpers: utility_meter, min_max, statistics, trend, filter
- Random generators: random_sensor, random_binary_sensor
- Time-based: tod (Time of Day)
- Climate/Environment: generic_thermostat, generic_hygrostat
- Entity converters: switch_as_x

Actions:
- list: List all helpers (optional: format=natural|json, verbose=true|false)
- create: Create a new helper (requires type, id, name)
- update: Update an existing helper (requires entity_id; supports all helper types including Config Entry helpers via Options Flow). icon is accepted on every type. Other fields accepted per type:` + updatableFieldsDescription() + `
- delete: Delete an existing helper (requires entity_id)
- get_details: Get helper details (requires entity_id)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper management operation",
			Properties:  props,
			Required:    []string{"action"},
		},
	}
}

func (h *ConsolidatedHelperHandlers) helperActionTool() mcp.Tool {
	return mcp.Tool{
		Name: "helper_action",
		Description: `Execute runtime actions on Home Assistant helpers.

Actions by helper type:
- input_boolean: toggle
- input_number/text/datetime: set (requires value)
- input_select: select (requires option), set_options (requires options)
- input_button: press
- counter: increment, decrement, reset, set (requires value)
- timer: start (optional duration), pause, cancel, finish, change (requires duration)
- schedule: reload
- group: add_entities, remove_entities, reload
- integral: reset`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper action parameters",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "Full entity ID of the helper",
				},
				"action": {
					Type:        "string",
					Description: "Action to perform",
					Enum:        allSupportedActions,
				},
				"value": {
					Type:        "string",
					Description: "Value for set actions (type depends on helper)",
				},
				"duration": {
					Type:        "string",
					Description: "Duration in HH:MM:SS format for timer start/change",
				},
				"option": {
					Type:        "string",
					Description: "Option to select for input_select",
				},
				"options": {
					Type:        "array",
					Description: "New options list for input_select set_options",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
				"add_entities": {
					Type:        "array",
					Description: "Entities to add to group",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
				"remove_entities": {
					Type:        "array",
					Description: "Entities to remove from group",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
			},
			Required: []string{"entity_id", "action"},
		},
	}
}

// =============================================================================
// Handler: manage_helper
// =============================================================================

func (h *ConsolidatedHelperHandlers) handleManageHelper(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case "list":
		return h.handleList(ctx, client, args)
	case "create":
		return h.handleCreate(ctx, client, args)
	case helperActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case "delete":
		return h.handleDelete(ctx, client, args)
	case "get_details":
		return h.handleGetDetails(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, create, update, delete, or get_details)", action)), nil
	}
}

func (h *ConsolidatedHelperHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	helpers, err := client.ListHelpers(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing helpers: %v", err)), nil
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)
	verbose, _ := args["verbose"].(bool)

	helperFormatter := formatter.NewHelperFormatter(format)
	opts := formatter.HelperListOptions{
		Verbose:     verbose,
		GroupByType: true,
	}

	output, err := helperFormatter.FormatList(ctx, helpers, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting helpers: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	helperType, _ := args["type"].(string)
	if helperType == "" {
		return errorResult("type is required for create action"), nil
	}

	meta, ok := helperTypes[helperType]
	if !ok {
		return errorResult(fmt.Sprintf("invalid helper type: %s (valid types: %s)", helperType, strings.Join(allHelperTypeNames, ", "))), nil
	}

	if err := checkSourceEntityDomain(ctx, client, helperType, meta.sourceEntities, args); err != nil {
		return errorResult(err.Error()), nil
	}

	id, _ := args["id"].(string)
	if id == "" {
		return errorResult("id is required for create action"), nil
	}

	name, _ := args["name"].(string)
	if name == "" {
		return errorResult("name is required for create action"), nil
	}

	// Validate type-specific required fields
	if err := validateRequiredFields(helperType, meta, args); err != nil {
		return errorResult(err.Error()), nil
	}

	// Build config
	config, err := buildHelperConfig(helperType, name, args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	// Determine if this is a WebSocket helper or Config Entry Flow helper
	isWSHelper := !homeassistant.RequiresConfigEntryFlow(meta.platform)

	var entityID string
	if isWSHelper {
		entityID, err = h.createWSHelper(ctx, client, id, name, config, meta, helperType)
	} else {
		entityID, err = h.createConfigEntryHelper(ctx, client, id, name, config, meta, helperType)
	}

	return renderCreateResult(helperType, name, entityID, err), nil
}

// renderCreateResult builds the final MCP result for a helper create call,
// handling three outcomes uniformly: a hard failure (errorResult), a
// success where the helper exists but carries a warning (unclaimed fields
// via *homeassistant.PartialApplyError, or an unresolved entity_id via
// entityUnresolvedError - see createConfigEntryHelper), and a clean
// success.
func renderCreateResult(helperType, name, entityID string, err error) *mcp.ToolsCallResult {
	var successMsg string
	if entityID != "" {
		successMsg = fmt.Sprintf("%s '%s' created successfully as %s", formatHelperType(helperType), name, entityID)
	} else {
		// Config Entry resolution couldn't confirm the real entity_id in
		// time, and the platform (currently only switch_as_x) is one where
		// name-based prediction is known to be unreliable - see
		// entityUnresolvedError.
		successMsg = fmt.Sprintf("%s '%s' created successfully (entity_id could not be confirmed)", formatHelperType(helperType), name)
	}

	var partial *homeassistant.PartialApplyError
	var unresolved *entityUnresolvedError
	hasPartial := errors.As(err, &partial)
	hasUnresolved := errors.As(err, &unresolved)
	if hasPartial || hasUnresolved {
		// The helper (and, for a PartialApplyError, its config entry)
		// already exists on Home Assistant's side by this point - this
		// must be a SUCCESS result carrying a warning, not an error
		// result. IsError:true here would read as "nothing happened" and
		// risk a caller retrying create, which would duplicate the helper.
		var warnings []string
		if hasPartial {
			warnings = append(warnings, renderPartialApplyWarning(partial)+
				". The helper exists - do not retry create; use manage_helper update, or delete it.")
		}
		if hasUnresolved {
			warnings = append(warnings, unresolved.Error())
		}
		return successResult(fmt.Sprintf("%s\nWARNING: %s", successMsg, strings.Join(warnings, " ")))
	}
	if err != nil {
		return errorResult(err.Error())
	}

	return successResult(successMsg)
}

// createWSHelper creates a WebSocket-based helper, using id to control entity slug.
func (h *ConsolidatedHelperHandlers) createWSHelper(
	ctx context.Context,
	client homeassistant.Client,
	id, name string,
	config map[string]any,
	meta helperTypeMetadata,
	helperType string,
) (string, error) {
	idSlug := slugifyName(id)
	nameSlug := slugifyName(name)
	needsUpdate := idSlug != nameSlug

	if needsUpdate {
		// Override config name with id to control entity slug
		config["name"] = id
	}

	helper := homeassistant.HelperConfig{
		Platform: meta.platform,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return "", fmt.Errorf("error creating %s: %w", helperType, err)
	}

	if needsUpdate {
		// Restore display name and re-send full config: HA requires all type-specific
		// mandatory fields (e.g. min/max for input_number, has_date/has_time for
		// input_datetime) even in update calls — a name-only payload is rejected.
		config["name"] = name
		updateConfig := homeassistant.HelperConfig{
			Platform: meta.platform,
			Config:   config,
		}
		if updateErr := client.UpdateHelper(ctx, id, updateConfig); updateErr != nil {
			return "", fmt.Errorf("%s created as %s.%s, but failed to set display name: %w",
				formatHelperType(helperType), meta.entityPrefix, idSlug, updateErr)
		}
	}

	return fmt.Sprintf("%s.%s", meta.entityPrefix, idSlug), nil
}

// createConfigEntryHelper creates a Config Entry Flow helper, using name for entity slug.
// If an icon is provided in config, it will be set via Entity Registry after creation.
func (h *ConsolidatedHelperHandlers) createConfigEntryHelper(
	ctx context.Context,
	client homeassistant.Client,
	id, name string,
	config map[string]any,
	meta helperTypeMetadata,
	helperType string,
) (string, error) {
	// Extract and REMOVE icon before creation
	// Config Entry Flow doesn't support icon in create flow
	icon, hasIcon := config["icon"].(string)
	if hasIcon {
		delete(config, "icon")
	}

	helper := homeassistant.HelperConfig{Platform: meta.platform, ID: id, Config: config}
	realEntityID, partialErr, err := h.createHelperEntityAllowingPartial(ctx, client, helper, helperType)
	if err != nil {
		return "", err
	}

	// CreateHelperEntity resolves the real id HA assigned via the entity
	// registry - authoritative when available. It returns "" when
	// resolution can't confirm an id in time (or for platforms it doesn't
	// apply to). Name-based prediction is structurally unable to get
	// switch_as_x right (HA derives that entity's name from the wrapped
	// source switch, not from any field this tool submits, #224), so it is
	// used as a fallback only for platforms where entityIDPredictable
	// reports it's safe.
	entityID := realEntityID
	if entityID == "" && entityIDPredictable(meta.platform) {
		entityID = predictConfigEntryEntityID(name, config, meta)
	}

	// Unresolved AND prediction unsafe: never report or write to a guessed
	// id - not even as informational text in the success message, since a
	// caller-controlled guess like <target_domain>.<slugified name> may
	// legitimately name an unrelated, pre-existing entity. Report it
	// instead via entityUnresolvedError; the helper still exists, so this
	// is a warning, not a failure.
	if entityID == "" {
		return "", errors.Join(partialErr, &entityUnresolvedError{helperType: helperType, iconRequested: hasIcon && icon != ""})
	}

	if hasIcon && icon != "" {
		return h.applyCreateIcon(ctx, client, entityID, icon, helperType, partialErr)
	}

	return entityID, partialErr
}

// createHelperEntityAllowingPartial calls CreateHelperEntity and separates a
// *homeassistant.PartialApplyError (the config entry already exists on
// Home Assistant's side - callers fall through to resolve/predict the
// entity id and apply the icon exactly as on a full success, instead of
// treating creation as never having happened) from a genuine failure (no
// entity to report on, so err is returned directly and callers must stop).
//
// partialErr is returned as a plain error, not the typed pointer: a nil
// *PartialApplyError boxed directly in the plain error return would be a
// non-nil interface holding a nil pointer (the classic Go typed-nil-in-
// interface trap) - a caller's `err != nil` check would then be true, and
// calling Error() on it would panic dereferencing the nil receiver.
func (h *ConsolidatedHelperHandlers) createHelperEntityAllowingPartial(
	ctx context.Context, client homeassistant.Client, helper homeassistant.HelperConfig, helperType string,
) (entityID string, partialErr, err error) {
	var partial *homeassistant.PartialApplyError
	realEntityID, createErr := client.CreateHelperEntity(configEntryResolveContext(ctx), helper)
	if createErr != nil {
		if !errors.As(createErr, &partial) {
			return "", nil, fmt.Errorf("error creating %s: %w", helperType, createErr)
		}
		partialErr = partial
	}
	return realEntityID, partialErr, nil
}

// applyCreateIcon sets the icon via Entity Registry on the entity created by
// createConfigEntryHelper. entityID is guaranteed non-empty here -
// createConfigEntryHelper returns early via entityUnresolvedError before
// ever reaching this call when resolution failed and prediction was unsafe
// for the platform (currently only switch_as_x, #224), so this never writes
// to a guessed id.
func (h *ConsolidatedHelperHandlers) applyCreateIcon(
	ctx context.Context, client homeassistant.Client, entityID, icon, helperType string, partialErr error,
) (string, error) {
	waitForEntityAppear(ctx, client, entityID)

	updateCfg := homeassistant.EntityRegistryUpdateConfig{Icon: &icon}
	if _, err := client.UpdateEntityRegistryEntry(ctx, entityID, updateCfg); err != nil {
		// Non-fatal: entity was created successfully, just couldn't set icon
		return entityID, errors.Join(partialErr, fmt.Errorf("%s created as %s, but failed to set icon: %w", formatHelperType(helperType), entityID, err))
	}

	return entityID, partialErr
}

// entityUnresolvedError signals a Config Entry helper was created
// successfully but its real entity_id could not be confirmed via the entity
// registry, and - because name-based prediction is unsafe for this platform
// (entityIDPredictable, currently only switch_as_x, #224) - neither the
// entity_id itself nor (when requested) its icon could be reported/applied.
// handleCreate renders this as a success result with an appended WARNING,
// the same as a *homeassistant.PartialApplyError, rather than as a failure:
// the helper genuinely exists.
type entityUnresolvedError struct {
	helperType    string
	iconRequested bool
}

func (w *entityUnresolvedError) Error() string {
	msg := fmt.Sprintf("%s created, but its entity_id could not be confirmed in time", formatHelperType(w.helperType))
	if w.iconRequested {
		msg += "; icon not applied"
	}
	return msg + "."
}

// entityIDPredictable reports whether a Config Entry platform's created
// entity_id can be derived from name via predictConfigEntryEntityID.
// switch_as_x cannot (#224): HA names the wrapper after the wrapped source
// switch, not from any field submitted to the flow, so the "prediction" is
// really just <target_domain>.<slugified name> - a caller-controlled guess
// that may legitimately name an unrelated, pre-existing entity. That makes
// it unsafe to use as an icon-update write target once we already know the
// guess can be wrong, so callers must skip prediction entirely for these
// platforms rather than use it as a lesser-confidence fallback.
func entityIDPredictable(platform string) bool {
	return platform != platformSwitchAsX
}

// configEntryResolveContext stamps the tool's configured wait onto ctx so
// CreateHelperEntity's entity-registry resolution honors HA_WAIT_TIMEOUT_MS
// instead of a hardcoded default. Each poll tick is a FULL entity-registry
// fetch (unlike a single-entity GetState poll), so the interval is floored
// at 500ms even when the configured wait is tuned tighter for fast
// single-entity waits elsewhere in the same create call.
func configEntryResolveContext(ctx context.Context) context.Context {
	cfg := pollerConfigFromContext(ctx)
	const minInterval = 500 * time.Millisecond
	cfg.PollInterval = max(cfg.PollInterval, minInterval)
	return homeassistant.WithEntityPollerConfig(ctx, cfg)
}

// predictConfigEntryEntityID guesses the entity_id a Config Entry helper
// create will produce, for use only when CreateHelperEntity's registry
// resolution couldn't confirm the real id in time. This is a best-effort
// fallback, not a source of truth - it cannot be correct for switch_as_x
// (see #224), whose real id HA derives from the wrapped source entity, not
// from name.
func predictConfigEntryEntityID(name string, config map[string]any, meta helperTypeMetadata) string {
	predictedSlug := slugifyName(name)
	prefix := meta.entityPrefix

	// Determine dynamic entity prefix for menu-based helpers
	switch meta.platform {
	case platformGroup:
		prefix = groupEntityPrefix(config)
	case platformRandom:
		// random helper creates sensor or binary_sensor based on type field
		if typeVal, ok := config["type"].(string); ok {
			prefix = typeVal
		}
	case platformSwitchAsX:
		// switch_as_x creates entity in target_domain
		if targetDomain, ok := config["target_domain"].(string); ok {
			prefix = targetDomain
		}
	}

	return fmt.Sprintf("%s.%s", prefix, predictedSlug)
}

func (h *ConsolidatedHelperHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for update action"), nil
	}

	if err := ValidateEntityID(entityID); err != nil {
		return errorResult(err.Error()), nil
	}

	// Extract entity domain and helper ID from entity_id. entityDomain is the
	// entity's domain (e.g. "sensor", "climate", "humidifier"), NOT the
	// integration platform name (e.g. "statistics", "generic_hygrostat") -
	// meta.platform and homeassistant.RequiresConfigEntryFlow below take the
	// latter. Conflating the two here was the root cause update's
	// source-domain check (checkUpdateSourceEntityDomain) had to work around.
	entityDomain, helperID := ParseHelperEntityID(entityID)
	if entityDomain == "" || helperID == "" {
		return errorResult(fmt.Sprintf("invalid entity_id format: %s (expected format: 'domain.object_id')", entityID)), nil
	}

	// Reject a same-domain entity that isn't actually a helper before doing
	// anything else - see checkHelperOnlyDomain's doc comment.
	if err := checkHelperOnlyDomain(ctx, client, entityID, entityDomain); err != nil {
		return errorResult(err.Error()), nil
	}

	// Determine helper type from entity domain
	// For most platforms, the helper type matches the entity domain
	// Special cases: sensor/binary_sensor could be template/threshold/derivative/integral/group
	helperType := entityDomain

	// Get metadata for validation
	meta, ok := helperTypes[helperType]

	// Re-validate domain-constrained source fields (utility_meter,
	// generic_thermostat, generic_hygrostat) before touching them - create's
	// checkSourceEntityDomain preflight doesn't cover update, which can
	// repoint the same fields at a mismatched entity and only surface HA's
	// opaque config-flow error. Skipped for genuine WebSocket helper types
	// (input_*, counter, timer, schedule, group): none of the 7
	// source-constrained types can ever reach this branch (their entities
	// live under sensor/binary_sensor/climate/humidifier/select, not under a
	// helperTypes key matching their own platform name), so paying a full
	// entity-registry fetch here on every input_boolean/counter/... update
	// would buy nothing.
	if !ok || homeassistant.RequiresConfigEntryFlow(meta.platform) {
		if err := checkUpdateSourceEntityDomain(ctx, client, entityID, args); err != nil {
			return errorResult(err.Error()), nil
		}
	}

	var config map[string]any
	var err error

	if ok {
		// The map key matching entityDomain ("ok") only tells us this entity
		// domain has metadata - it does NOT mean it's a WebSocket helper. See
		// buildKnownTypeUpdateConfig for why "group" must not take the merge path.
		config, err = buildKnownTypeUpdateConfig(ctx, client, entityID, helperType, meta, args)
	} else {
		// Unknown helper type (sensor/binary_sensor without metadata) - a
		// Config Entry Flow helper.
		config, err = buildUnknownTypeUpdateConfig(ctx, client, entityID, entityDomain, args)
	}
	if err != nil {
		return errorResult(err.Error()), nil
	}

	// Snapshot which keys actually made it into the built config BEFORE
	// calling UpdateHelper - client.UpdateHelper's config-entry path
	// (updateHelperViaOptionsFlow) deletes "icon"/"name" from this same map
	// in place after consuming them, so reading it afterwards would
	// misreport icon/name as never-applied even when they were (see
	// splitAppliedFields).
	appliedKeys := make(map[string]bool, len(config))
	for k := range config {
		appliedKeys[k] = true
	}

	// Create UpdateHelper request
	updateConfig := homeassistant.HelperConfig{
		Platform: entityDomain,
		Config:   config,
	}

	// Pass the FULL entity_id, not the stripped helperID: HybridClient.UpdateHelper
	// routes config-entry helpers (template, threshold, group, ...) to the Options
	// Flow REST API by matching the registry's full entity_id. A bare id never
	// matches, so the call silently falls through to the WS "<platform>/update"
	// command, which config-entry domains (sensor, binary_sensor, ...) don't have
	// and produces "unknown_command".
	var partial *homeassistant.PartialApplyError
	if updateErr := client.UpdateHelper(ctx, entityID, updateConfig); updateErr != nil {
		if !errors.As(updateErr, &partial) {
			return errorResult(fmt.Sprintf("error updating helper: %v", updateErr)), nil
		}
	}

	return successResult(renderUpdateResultMessage(entityID, entityDomain, args, appliedKeys, partial)), nil
}

// renderUpdateResultMessage renders handleUpdate's success text, folding
// in a *homeassistant.PartialApplyError as a WARNING rather than an error -
// split out of handleUpdate to keep its cognitive complexity down. A
// partial apply is still a successful result: the fields every options
// flow step DID accept have already been committed to Home Assistant.
func renderUpdateResultMessage(entityID, entityDomain string, args map[string]any, appliedKeys map[string]bool, partial *homeassistant.PartialApplyError) string {
	applied, ignored, skipped := splitAppliedFields(entityDomain, args, appliedKeys)
	if partial == nil {
		return updateSuccessMessage(entityID, applied, ignored, skipped)
	}

	// Every field every options flow step DID accept has already been
	// committed by this point - move the rejected ones OUT of "applied"
	// (they reached the locally-built payload, which is all
	// splitAppliedFields can see, but HA's options flow rejected them) and
	// into the WARNING appended below.
	rejectedConfigKeys := make(map[string]bool, len(partial.Fields))
	for _, key := range partial.Fields {
		rejectedConfigKeys[key] = true
	}
	var stillApplied []string
	for _, argName := range applied {
		key := resolveUpdateConfigKey(entityDomain, argName)
		if !rejectedConfigKeys[key] {
			stillApplied = append(stillApplied, argName)
		}
	}
	msg := updateSuccessMessage(entityID, stillApplied, ignored, skipped)
	return fmt.Sprintf("%s\nWARNING: %s.", msg, renderPartialApplyWarning(partial))
}

// updateSuccessMessage renders manage_helper update's success message,
// echoing which caller-supplied fields actually reached the payload sent to
// Home Assistant, and separately which ones the caller supplied but no
// builder ever read - a bare "updated successfully" left a caller unable
// to tell full application from a partial one, and echoing every
// non-dispatch arg name as "applied" unconditionally is wrong: manage_helper's
// own format/verbose output args, a typo'd field name, or a field silently
// dropped by a platform-specific gate like addGenericThermostatPresetFields
// would all be reported as successfully applied when nothing was ever sent
// for them. See splitAppliedFields for how the two lists are derived.
func updateSuccessMessage(entityID string, applied, ignored, skipped []string) string {
	if len(applied) == 0 && len(ignored) == 0 && len(skipped) == 0 {
		return fmt.Sprintf("Helper '%s' updated successfully", entityID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Helper '%s' updated successfully", entityID)
	if len(applied) > 0 {
		sorted := append([]string{}, applied...)
		slices.Sort(sorted)
		fmt.Fprintf(&b, " (applied: %s)", homeassistant.BoundedFieldList(sorted))
	}
	if len(ignored) > 0 {
		sorted := append([]string{}, ignored...)
		slices.Sort(sorted)
		fmt.Fprintf(&b, " (ignored - not accepted by this helper type: %s)", homeassistant.BoundedFieldList(sorted))
	}
	if len(skipped) > 0 {
		sorted := append([]string{}, skipped...)
		slices.Sort(sorted)
		fmt.Fprintf(&b, " (ignored - no value supplied (null/empty): %s)", homeassistant.BoundedFieldList(sorted))
	}
	return b.String()
}

// renderPartialApplyWarning renders a *homeassistant.PartialApplyError's
// rejected fields for a manage_helper WARNING line, translating HA's
// config key names back to the caller's own arg names via
// argNamesForConfigKeys - updateConfigKeyAliases lives in this package,
// not internal/homeassistant, so the translation happens at this boundary
// rather than inside PartialApplyError.Error() itself.
func renderPartialApplyWarning(partial *homeassistant.PartialApplyError) string {
	argNames := argNamesForConfigKeys(partial.Fields)
	if partial.Op == homeassistant.PartialApplyUpdate {
		return fmt.Sprintf("field(s) %s were not accepted by any step of its options flow and have NOT been applied",
			homeassistant.BoundedFieldList(argNames))
	}
	return fmt.Sprintf("field(s) %s were not accepted by any step of the %s config flow and have NOT been applied",
		homeassistant.BoundedFieldList(argNames), partial.Platform)
}

// splitAppliedFields separates a caller's update args into fields that
// actually reached the payload sent to Home Assistant (present as a key in
// appliedKeys, a snapshot of the built config map taken before
// client.UpdateHelper runs) versus fields the caller supplied that no
// builder ever read (a typo, or a field belonging to a different helper
// type - e.g. generic_thermostat's preset fields on a non-climate helper,
// silently dropped by addGenericThermostatPresetFields' domain gate).
//
// appliedKeys must be snapshotted BEFORE client.UpdateHelper runs: for
// config-entry helpers, updateHelperViaOptionsFlow deletes "icon"/"name"
// from the same config map in place after using them, so reading config's
// keys after the call would misreport icon/name as ignored even when they
// were applied via the entity registry.
func splitAppliedFields(entityDomain string, args map[string]any, appliedKeys map[string]bool) (applied, ignored, skipped []string) {
	for name := range args {
		if updateDispatchOnlyArgNames[name] {
			continue
		}
		if isSkippable(args[name]) {
			// A null or empty-string value never reaches argReader's writers
			// (isSkippable short-circuits every str/num/strID/... method), so
			// it belongs in neither "applied" (nothing was sent for it,
			// even if a merged current value happens to share the config
			// key) nor "ignored - not accepted" (the field IS accepted;
			// the caller just supplied no value for it).
			skipped = append(skipped, name)
			continue
		}
		key := resolveUpdateConfigKey(entityDomain, name)
		if appliedKeys[key] {
			applied = append(applied, name)
		} else {
			ignored = append(ignored, name)
		}
	}
	return applied, ignored, skipped
}

// updateDispatchOnlyArgNames are manage_helper's own top-level dispatch/
// output args - never a helper config field, so splitAppliedFields must
// never classify them as applied or ignored regardless of which helper
// type is being updated.
var updateDispatchOnlyArgNames = map[string]bool{
	"action": true, "type": true, "id": true, attrEntityID: true,
	"format": true, "verbose": true,
}

// buildUnknownTypeUpdateConfig builds the update config for a helper whose
// entity domain has no helperTypes entry (sensor/binary_sensor/...) - these
// are always Config Entry Flow helpers, built as loose config rather than
// through a typed helperTypes-driven builder. Split out of handleUpdate to
// keep that function under the funlen limit.
//
// minMaxPlatform is resolved only when the caller actually supplied
// min_max_type, to avoid a registry fetch on every other config-entry
// update - and a mismatch here is a hard error, not a degraded skip: see
// resolveConfigEntryPlatformForMinMaxType's doc comment for why.
func buildUnknownTypeUpdateConfig(
	ctx context.Context, client homeassistant.Client, entityID, entityDomain string, args map[string]any,
) (map[string]any, error) {
	minMaxPlatform := ""
	if _, hasMinMaxType := args["min_max_type"]; hasMinMaxType {
		resolvedPlatform, err := resolveConfigEntryPlatformForMinMaxType(ctx, client, entityID)
		if err != nil {
			return nil, err
		}
		if resolvedPlatform != platformMinMax {
			return nil, fmt.Errorf(
				"min_max_type is only valid for min_max helpers; %s is a %s helper", entityID, resolvedPlatform,
			)
		}
		minMaxPlatform = resolvedPlatform
	}
	return buildConfigEntryUpdateConfig(configEntryUpdateContext{
		entityDomain:   entityDomain,
		minMaxPlatform: minMaxPlatform,
	}, args)
}

// buildKnownTypeUpdateConfig builds the update config for a helperType that
// has an entry in helperTypes. See CLAUDE.md's "Partial update merge" gotcha
// for why the merge gate below is !RequiresConfigEntryFlow(meta.platform)
// rather than helperTypes key presence (group is a helperTypes key but a
// Config Entry Flow platform, so it skips the merge).
//
// "threshold" and "derivative" are also Config Entry Flow platforms with map
// key == platform name, but unlike "group" they never even reach this
// function: their entities live under binary_sensor.*/sensor.*, so
// ParseHelperEntityID's platform for them is "binary_sensor"/"sensor" (not
// "threshold"/"derivative"), and handleUpdate's helperTypes[helperType]
// lookup misses entirely, routing them through buildConfigEntryUpdateConfig
// instead. "group" is the only type where a helperTypes key match still
// requires the Config-Entry-Flow branch below, because plain "group"
// entities keep their own domain as both their entity prefix and their map
// key.
func buildKnownTypeUpdateConfig(ctx context.Context, client homeassistant.Client, entityID, helperType string, meta helperTypeMetadata, args map[string]any) (map[string]any, error) {
	var updateName string
	effectiveArgs := args

	if homeassistant.RequiresConfigEntryFlow(meta.platform) {
		// Config Entry platform reached via the metadata branch: Options Flow
		// submission (see buildStepSubmission in internal/homeassistant/flow_steps.go)
		// already merges unchanged fields on the HA side, so no local merge
		// is needed here - args alone is the correct payload.
		updateName, _ = args["name"].(string)
	} else {
		// Known WebSocket helper type - merge current state so an update that
		// omits an optional/required field keeps its current value instead of
		// HA's <platform>/update resetting it to empty.
		merged, currentName, fetchErr := mergeCurrentHelperState(ctx, client, entityID, helperType, meta, args)
		if fetchErr != nil {
			return nil, fmt.Errorf(
				"cannot update %s: failed to read its current configuration (%w), and %s's update API replaces all fields (omitted fields would be reset to their defaults). %s",
				entityID, fetchErr, meta.platform, updateFetchFailureHint(fetchErr, entityID),
			)
		}
		effectiveArgs = merged
		if name, hasName := args["name"].(string); hasName && name != "" {
			updateName = name
		} else {
			updateName = currentName
		}
	}

	config, err := buildHelperConfig(helperType, updateName, effectiveArgs)
	if err != nil {
		return nil, err
	}

	// Remove name from config unless the caller supplied one or the merge
	// found a current friendly_name to preserve (buildHelperConfig adds an
	// empty name by default).
	if updateName == "" {
		delete(config, "name")
	}

	return config, nil
}

// mergeCurrentHelperState fetches entity_id's current stored config and
// merges its fields with the caller's args, so an update omitting an
// optional/required field keeps the current value instead of HA resetting
// it to empty. Filters strictly to updatableFieldNames(typeName).
//
// fetchErr is non-nil when the fetch itself failed - the caller must fail
// the update rather than proceed with a partial payload; see CLAUDE.md's
// "Merge-fetch failure hard-fails the update" gotcha for why this is NOT
// the configWriteGuardError "checked=false, proceed anyway" convention.
func mergeCurrentHelperState(ctx context.Context, client homeassistant.Client, entityID, typeName string, meta helperTypeMetadata, args map[string]any) (merged map[string]any, currentName string, fetchErr error) {
	merged = make(map[string]any)
	fields := updatableFieldNames(typeName)

	current, err := client.GetHelperConfig(ctx, meta.platform, entityID)
	if err != nil {
		return nil, "", err
	}
	for _, field := range fields {
		if v, exists := current[field]; exists && v != nil {
			merged[field] = v
		}
	}
	// GetHelperConfig's map is "<platform>/list"'s raw stored entry, which
	// includes "name" - unlike GetState's Attributes, name isn't itself in
	// updatableFieldNames() (it's not a create/update field, it's passed as
	// buildHelperConfig's separate name argument). Without this, an update
	// omitting "name" would delete it from the payload and HA's
	// "<platform>/update" (every WS helper platform requires "name") would
	// reject the call.
	if name, exists := current["name"].(string); exists && name != "" {
		currentName = name
	}

	// Caller-supplied values always win, EXCEPT: the two "leave unset"
	// spellings argReader itself treats identically (see isSkippable) - an
	// explicit JSON null, and an empty string, since this API has no "clear
	// this field" spelling and letting either through would overwrite a good
	// merged value with one that argReader.str/num/etc. then silently skips
	// writing into the built config; and any field isUpdateExcludedField
	// reports for typeName - the exclusion table above exists precisely to
	// stop a field from reaching the update payload, and a caller naming it
	// directly here must not be a back door around that, the same as the
	// first loop already refuses to reintroduce it from stored config.
	for k, v := range args {
		if isSkippable(v) || isUpdateExcludedField(typeName, k) {
			continue
		}
		merged[k] = v
	}

	return merged, currentName, nil
}

// updateFetchFailureHint gives an actionable next step for a failed
// mergeCurrentHelperState fetch. When the failure is specifically
// homeassistant.ErrHelperNotFoundInStorage, the two known causes are a
// YAML-defined helper (never registered in storage) or one renamed via the
// entity registry after creation (storage keeps the id assigned at
// creation, so it desyncs from the object_id the lookup uses) - matched via
// errors.Is rather than message text so an unrelated transport error whose
// text happens to contain "not found" doesn't get misattributed to either.
func updateFetchFailureHint(fetchErr error, entityID string) string {
	if errors.Is(fetchErr, homeassistant.ErrHelperNotFoundInStorage) {
		return fmt.Sprintf(
			"%s was not found among storage-managed helpers of this type. Either it was defined in "+
				"configuration.yaml rather than created via manage_helper or the HA UI (edit the YAML file "+
				"directly and reload instead), or its entity_id was renamed via the entity registry after "+
				"creation (retry using the entity's original entity_id, or recreate it).",
			entityID,
		)
	}
	return "Retry, or verify the entity exists."
}

func (h *ConsolidatedHelperHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for delete action"), nil
	}

	if err := ValidateEntityID(entityID); err != nil {
		return errorResult(err.Error()), nil
	}

	// Reject a same-domain entity that isn't actually a helper before doing
	// anything else - same gap checkHelperOnlyDomain closed for update
	// (issue #206 review, W4): without this, DeleteHelper routes any entity
	// carrying a config_entry_id to DeleteConfigEntry, which for a real
	// integration entity (light.hue_office, a Zigbee switch.*, ...) deletes
	// the whole integration, not just that entity.
	entityDomain, _ := ParseHelperEntityID(entityID)
	if err := checkHelperOnlyDomain(ctx, client, entityID, entityDomain); err != nil {
		return errorResult(err.Error()), nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return errorResult(fmt.Sprintf("Error deleting helper: %v", err)), nil
	}

	successMsg := fmt.Sprintf("Helper '%s' deleted successfully", entityID)
	if !waitForEntityDisappear(ctx, client, entityID) {
		successMsg += " (warning: helper entity may still be visible briefly)"
	}

	return successResult(successMsg), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetails(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for get_details action"), nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	switch platform {
	case platformSchedule:
		return h.handleGetDetailsSchedule(ctx, client, args)
	case platformCounter:
		return h.handleGetDetailsCounter(ctx, client, args)
	case platformTimer:
		return h.handleGetDetailsTimer(ctx, client, args)
	case platformInputBoolean, platformInputNumber, platformInputText, platformInputSelect,
		platformInputDatetime, platformInputButton, platformGroup, platformSensorEntity, platformBinarySensorEntity,
		"climate", "humidifier", "select":
		return h.handleGetDetailsGeneric(ctx, client, args, platform)
	default:
		// Entity domains created by the 15 new template_* subtypes (issue
		// #206) are plain state-backed entities, same as climate/humidifier/
		// select above - reuse templateSubtypeDomains rather than listing
		// all 15 domains again here.
		if templateSubtypeDomains[platform] {
			return h.handleGetDetailsGeneric(ctx, client, args, platform)
		}
		return errorResult(fmt.Sprintf("get_details is not supported for helper type: %s", platform)), nil
	}
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsSchedule(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting schedule state: %v", err)), nil
	}

	config, configErr := client.GetHelperConfig(ctx, platformSchedule, entityID)
	if configErr != nil {
		config = make(map[string]any)
	}

	timeBlocks := parseTimeBlocks(config)
	details := buildScheduleDetails(state, timeBlocks)

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatScheduleDetail(ctx, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting schedule: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsCounter(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting counter state: %v", err)), nil
	}

	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	// Extract counter-specific attributes and convert to strings for natural format
	if initial, ok := state.Attributes["initial"]; ok {
		details["initial"] = fmt.Sprintf("%v", initial)
	}
	if minimum, ok := state.Attributes["minimum"]; ok {
		details["minimum"] = fmt.Sprintf("%v", minimum)
	}
	if maximum, ok := state.Attributes["maximum"]; ok {
		details["maximum"] = fmt.Sprintf("%v", maximum)
	}
	if step, ok := state.Attributes["step"]; ok {
		details["step"] = fmt.Sprintf("%v", step)
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatCounterDetail(ctx, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting counter: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting timer state: %v", err)), nil
	}

	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	// Extract timer-specific attributes (already strings in HA)
	if duration, ok := state.Attributes["duration"]; ok {
		details["duration"] = fmt.Sprintf("%v", duration)
	}
	if remaining, ok := state.Attributes["remaining"]; ok {
		details["remaining"] = fmt.Sprintf("%v", remaining)
	}
	if finishesAt, ok := state.Attributes["finishes_at"]; ok {
		details["finishes_at"] = fmt.Sprintf("%v", finishesAt)
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatTimerDetail(ctx, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting timer: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsGeneric(ctx context.Context, client homeassistant.Client, args map[string]any, helperType string) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting helper state: %v", err)), nil
	}

	details := buildHelperDetails(state, helperType)

	// Enrich with template config for template sensors/binary_sensors
	if helperType == platformSensorEntity || helperType == platformBinarySensorEntity {
		enrichConfigEntryOptions(ctx, client, entityID, details)
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatHelperDetail(ctx, helperType, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting helper: %v", err)), nil
	}

	return successResult(output), nil
}

func buildHelperDetails(state *homeassistant.Entity, helperType string) map[string]any {
	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	switch helperType {
	case platformInputBoolean:
		addInputBooleanDetails(details, state)
	case platformInputNumber:
		addInputNumberDetails(details, state)
	case platformInputText:
		addInputTextDetails(details, state)
	case platformInputSelect:
		addInputSelectDetails(details, state)
	case platformInputDatetime:
		addInputDatetimeDetails(details, state)
	case platformInputButton:
		addInputButtonDetails(details, state)
	case platformGroup:
		addGroupDetails(details, state)
	case platformSensorEntity:
		addSensorDetails(details, state)
	case platformBinarySensorEntity:
		addBinarySensorDetails(details, state)
	default:
		addGenericDetails(details, state)
	}

	return details
}

// genericDetailSkipAttributes lists state attributes never surfaced by the
// generic get_details fallback: friendly_name is already a top-level detail
// key, supported_features is an opaque bitmask, and attribution is boilerplate
// noise repeated across most HA entities.
var genericDetailSkipAttributes = map[string]bool{
	"friendly_name":      true,
	"supported_features": true,
	"attribution":        true,
}

// addGenericDetails copies every state attribute not on genericDetailSkipAttributes
// into details. Used for domains without a dedicated add*Details builder
// (climate, humidifier, select, and the 15 template_* subtype domains) so
// get_details returns real content instead of just entity_id/state/friendly_name.
func addGenericDetails(details map[string]any, state *homeassistant.Entity) {
	for k, v := range state.Attributes {
		if genericDetailSkipAttributes[k] {
			continue
		}
		details[k] = v
	}
}

func addInputBooleanDetails(details map[string]any, state *homeassistant.Entity) {
	if icon, ok := state.Attributes["icon"]; ok {
		details["icon"] = icon
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputNumberDetails(details map[string]any, state *homeassistant.Entity) {
	if minVal, ok := state.Attributes["min"]; ok {
		details["min"] = fmt.Sprintf("%v", minVal)
	}
	if maxVal, ok := state.Attributes["max"]; ok {
		details["max"] = fmt.Sprintf("%v", maxVal)
	}
	if step, ok := state.Attributes["step"]; ok {
		details["step"] = fmt.Sprintf("%v", step)
	}
	if mode, ok := state.Attributes["mode"]; ok {
		details["mode"] = mode
	}
	if unit, ok := state.Attributes["unit_of_measurement"]; ok {
		details["unit_of_measurement"] = unit
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputTextDetails(details map[string]any, state *homeassistant.Entity) {
	if minVal, ok := state.Attributes["min"]; ok {
		details["min"] = fmt.Sprintf("%v", minVal)
	}
	if maxVal, ok := state.Attributes["max"]; ok {
		details["max"] = fmt.Sprintf("%v", maxVal)
	}
	if mode, ok := state.Attributes["mode"]; ok {
		details["mode"] = mode
	}
	if pattern, ok := state.Attributes["pattern"]; ok {
		details["pattern"] = pattern
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputSelectDetails(details map[string]any, state *homeassistant.Entity) {
	if options, ok := state.Attributes["options"]; ok {
		details["options"] = options
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputDatetimeDetails(details map[string]any, state *homeassistant.Entity) {
	if hasDate, ok := state.Attributes["has_date"]; ok {
		details["has_date"] = hasDate
	}
	if hasTime, ok := state.Attributes["has_time"]; ok {
		details["has_time"] = hasTime
	}
	if year, ok := state.Attributes["year"]; ok {
		details["year"] = year
	}
	if month, ok := state.Attributes["month"]; ok {
		details["month"] = month
	}
	if day, ok := state.Attributes["day"]; ok {
		details["day"] = day
	}
	if hour, ok := state.Attributes["hour"]; ok {
		details["hour"] = hour
	}
	if minute, ok := state.Attributes["minute"]; ok {
		details["minute"] = minute
	}
	if second, ok := state.Attributes["second"]; ok {
		details["second"] = second
	}
	if timestamp, ok := state.Attributes["timestamp"]; ok {
		details["timestamp"] = timestamp
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputButtonDetails(details map[string]any, state *homeassistant.Entity) {
	if icon, ok := state.Attributes["icon"]; ok {
		details["icon"] = icon
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addGroupDetails(details map[string]any, state *homeassistant.Entity) {
	if members, ok := state.Attributes["entity_id"]; ok {
		details["members"] = members
	}
	if all, ok := state.Attributes["all"]; ok {
		details["all"] = all
	}
}

func addSensorDetails(details map[string]any, state *homeassistant.Entity) {
	if unit, ok := state.Attributes["unit_of_measurement"]; ok {
		details["unit_of_measurement"] = unit
	}
	if deviceClass, ok := state.Attributes["device_class"]; ok {
		details["device_class"] = deviceClass
	}
	if stateClass, ok := state.Attributes["state_class"]; ok {
		details["state_class"] = stateClass
	}
	if source, ok := state.Attributes["source"]; ok {
		details["source"] = source
	}
}

func addBinarySensorDetails(details map[string]any, state *homeassistant.Entity) {
	if deviceClass, ok := state.Attributes["device_class"]; ok {
		details["device_class"] = deviceClass
	}
	if sourceEntity, ok := state.Attributes["entity_id"]; ok {
		details["source_entity"] = sourceEntity
	}
	if hysteresis, ok := state.Attributes["hysteresis"]; ok {
		details["hysteresis"] = hysteresis
	}
	if sensorValue, ok := state.Attributes["sensor_value"]; ok {
		details["sensor_value"] = sensorValue
	}
}

// enrichConfigEntryOptions enriches details with config entry options for template helpers.
// Gracefully degrades: if registry lookup fails or entity is not template-based, returns silently.
func enrichConfigEntryOptions(ctx context.Context, client homeassistant.Client, entityID string, details map[string]any) {
	// Look up entity in registry
	registry, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return // Graceful degradation
	}

	// Find the entity entry
	var entry *homeassistant.EntityRegistryEntry
	for i := range registry {
		if registry[i].EntityID == entityID {
			entry = &registry[i]
			break
		}
	}

	if entry == nil || entry.Platform != platformTemplate || entry.ConfigEntryID == "" {
		return // Not a template entity or no config entry
	}

	// Fetch config entry options via Options Flow REST API
	// (WebSocket GetConfigEntry does not populate Options field)
	options, err := client.GetConfigEntryOptions(ctx, entry.ConfigEntryID)
	if err != nil {
		return // Graceful degradation
	}

	// Extract template options if present
	if len(options) > 0 {
		addTemplateOptionsToDetails(options, details)
	}
}

// addTemplateOptionsToDetails extracts template options from config entry Options.
func addTemplateOptionsToDetails(options, details map[string]any) {
	details["config_entry_type"] = platformTemplate

	if state, ok := options["state"].(string); ok && state != "" {
		details["state_template"] = state
	}
	if availability, ok := options["availability"].(string); ok && availability != "" {
		details["availability_template"] = availability
	}
	if unit, ok := options["unit_of_measurement"].(string); ok && unit != "" {
		details["unit_of_measurement"] = unit
	}
	if deviceClass, ok := options["device_class"].(string); ok && deviceClass != "" {
		details["device_class"] = deviceClass
	}
	if stateClass, ok := options["state_class"].(string); ok && stateClass != "" {
		details["state_class"] = stateClass
	}
	if delayOn, ok := options["delay_on"]; ok && delayOn != nil {
		details["delay_on"] = delayOn
	}
	if delayOff, ok := options["delay_off"]; ok && delayOff != nil {
		details["delay_off"] = delayOff
	}
}

// =============================================================================
// Handler: helper_action
// =============================================================================

func (h *ConsolidatedHelperHandlers) handleHelperAction(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required"), nil
	}

	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	// Validate action is in supported list
	if !isValidAction(action) {
		return errorResult(fmt.Sprintf("invalid action: %s (valid actions: %s)", action, strings.Join(allSupportedActions, ", "))), nil
	}

	// Get entity domain
	domain, _ := ParseHelperEntityID(entityID)
	if domain == "" {
		return errorResult(fmt.Sprintf("invalid entity_id format: %s (expected format: 'domain.object_id')", entityID)), nil
	}

	// Route to appropriate handler based on action
	return h.routeAction(ctx, client, entityID, domain, action, args)
}

func (h *ConsolidatedHelperHandlers) routeAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Simple actions (no additional args needed)
	if result, handled := h.routeSimpleAction(ctx, client, entityID, domain, action); handled {
		return result, nil
	}
	// Actions that require args
	return h.routeArgsAction(ctx, client, entityID, domain, action, args)
}

func (h *ConsolidatedHelperHandlers) routeSimpleAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string) (*mcp.ToolsCallResult, bool) {
	switch action {
	case "toggle":
		result, _ := h.handleToggle(ctx, client, entityID, domain)
		return result, true
	case "increment":
		result, _ := h.handleCounterAction(ctx, client, entityID, domain, "increment")
		return result, true
	case "decrement":
		result, _ := h.handleCounterAction(ctx, client, entityID, domain, "decrement")
		return result, true
	case "reset":
		result, _ := h.handleReset(ctx, client, entityID, domain)
		return result, true
	case "pause":
		result, _ := h.handleTimerAction(ctx, client, entityID, domain, "pause", "paused")
		return result, true
	case "cancel":
		result, _ := h.handleTimerAction(ctx, client, entityID, domain, "cancel", "canceled")
		return result, true
	case "finish":
		result, _ := h.handleTimerAction(ctx, client, entityID, domain, "finish", "finished")
		return result, true
	case "press":
		result, _ := h.handlePress(ctx, client, entityID, domain)
		return result, true
	case "reload":
		result, _ := h.handleReload(ctx, client, entityID, domain)
		return result, true
	}
	return nil, false
}

func (h *ConsolidatedHelperHandlers) routeArgsAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string, args map[string]any) (*mcp.ToolsCallResult, error) {
	switch action {
	case "set":
		return h.handleSet(ctx, client, entityID, domain, args)
	case "start":
		return h.handleTimerStart(ctx, client, entityID, domain, args)
	case "change":
		return h.handleTimerChange(ctx, client, entityID, domain, args)
	case "select":
		return h.handleSelect(ctx, client, entityID, domain, args)
	case "set_options":
		return h.handleSetOptions(ctx, client, entityID, domain, args)
	case "add_entities":
		return h.handleGroupEntities(ctx, client, entityID, domain, args, true)
	case "remove_entities":
		return h.handleGroupEntities(ctx, client, entityID, domain, args, false)
	default:
		return errorResult(fmt.Sprintf("unsupported action: %s", action)), nil
	}
}

// =============================================================================
// Action Handlers
// =============================================================================

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleToggle(ctx context.Context, client homeassistant.Client, entityID, domain string) (*mcp.ToolsCallResult, error) {
	if domain != platformInputBoolean {
		return errorResult("toggle action is only supported for input_boolean helpers"), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, domain, "toggle", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error toggling %s: %v", entityID, err)), nil
	}

	return successResult(fmt.Sprintf("'%s' toggled successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleSet(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	value, hasValue := args["value"]
	if !hasValue {
		return errorResult("value is required for set action"), nil
	}

	var serviceDomain, serviceName string
	serviceData := map[string]any{"entity_id": entityID}

	switch domain {
	case platformInputNumber:
		serviceDomain = platformInputNumber
		serviceName = serviceSetValue
		serviceData["value"] = value
	case platformInputText:
		serviceDomain = platformInputText
		serviceName = serviceSetValue
		serviceData["value"] = value
	case platformInputDatetime:
		serviceDomain = platformInputDatetime
		serviceName = "set_datetime"
		serviceData["datetime"] = value
	case platformCounter:
		serviceDomain = platformCounter
		serviceName = serviceSetValue
		if v, ok := value.(float64); ok {
			serviceData["value"] = int(v)
		} else {
			serviceData["value"] = value
		}
	default:
		return errorResult(fmt.Sprintf("set action is not supported for %s helpers", domain)), nil
	}

	if _, err := client.CallService(ctx, serviceDomain, serviceName, serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error setting value: %v", err)), nil
	}

	return successResult(fmt.Sprintf("'%s' value set successfully", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleCounterAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string) (*mcp.ToolsCallResult, error) {
	if domain != platformCounter {
		return errorResult(fmt.Sprintf("%s action is only supported for counter helpers", action)), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, platformCounter, action, serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error %sing counter: %v", action, err)), nil
	}

	return successResult(fmt.Sprintf("'%s' %sed successfully", entityID, action)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleReset(ctx context.Context, client homeassistant.Client, entityID, domain string) (*mcp.ToolsCallResult, error) {
	serviceData := map[string]any{"entity_id": entityID}

	switch domain {
	case platformCounter:
		if _, err := client.CallService(ctx, platformCounter, "reset", serviceData); err != nil {
			return errorResult(fmt.Sprintf("Error resetting counter: %v", err)), nil
		}
	case platformSensorEntity:
		// Integral sensors use the "integration" service domain
		if _, err := client.CallService(ctx, platformIntegration, "reset", serviceData); err != nil {
			return errorResult(fmt.Sprintf("Error resetting integral: %v", err)), nil
		}
	default:
		return errorResult(fmt.Sprintf("reset action is not supported for %s helpers", domain)), nil
	}

	return successResult(fmt.Sprintf("'%s' reset successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleTimerStart(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformTimer {
		return errorResult("start action is only supported for timer helpers"), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if duration, ok := args["duration"].(string); ok && duration != "" {
		serviceData["duration"] = duration
	}

	if _, err := client.CallService(ctx, platformTimer, "start", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error starting timer: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Timer '%s' started successfully", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleTimerAction(ctx context.Context, client homeassistant.Client, entityID, domain, action, pastTense string) (*mcp.ToolsCallResult, error) {
	if domain != platformTimer {
		return errorResult(fmt.Sprintf("%s action is only supported for timer helpers", action)), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, platformTimer, action, serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error %sing timer: %v", action[:len(action)-1], err)), nil
	}

	return successResult(fmt.Sprintf("Timer '%s' %s successfully", entityID, pastTense)), nil
}

func (h *ConsolidatedHelperHandlers) handleTimerChange(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformTimer {
		return errorResult("change action is only supported for timer helpers"), nil
	}

	duration, ok := args["duration"].(string)
	if !ok || duration == "" {
		return errorResult("duration is required for change action"), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"duration":  duration,
	}

	if _, err := client.CallService(ctx, platformTimer, "change", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error changing timer: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Timer '%s' duration changed successfully", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handlePress(ctx context.Context, client homeassistant.Client, entityID, domain string) (*mcp.ToolsCallResult, error) {
	if domain != platformInputButton {
		return errorResult("press action is only supported for input_button helpers"), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, platformInputButton, "press", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error pressing button: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Button '%s' pressed successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleSelect(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformInputSelect {
		return errorResult("select action is only supported for input_select helpers"), nil
	}

	option, ok := args["option"].(string)
	if !ok || option == "" {
		return errorResult("option is required for select action"), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"option":    option,
	}

	if _, err := client.CallService(ctx, platformInputSelect, "select_option", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error selecting option: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Option '%s' selected successfully for '%s'", option, entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleSetOptions(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformInputSelect {
		return errorResult("set_options action is only supported for input_select helpers"), nil
	}

	options, ok := args["options"].([]any)
	if !ok {
		return errorResult("options parameter is required for set_options action"), nil
	}
	if len(options) == 0 {
		return errorResult("options must contain at least one value"), nil
	}

	strOptions := convertToStringSlice(options)
	if len(strOptions) == 0 {
		return errorResult("options must contain at least one string value"), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"options":   strOptions,
	}

	if _, err := client.CallService(ctx, platformInputSelect, "set_options", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error setting options: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Options updated successfully for '%s'", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleReload(ctx context.Context, client homeassistant.Client, _, domain string) (*mcp.ToolsCallResult, error) {
	var serviceDomain string
	switch domain {
	case platformSchedule:
		serviceDomain = platformSchedule
	case platformGroup:
		serviceDomain = "group"
	default:
		return errorResult(fmt.Sprintf("reload action is not supported for %s helpers", domain)), nil
	}

	serviceData := map[string]any{}
	if _, err := client.CallService(ctx, serviceDomain, "reload", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error reloading: %v", err)), nil
	}

	return successResult(fmt.Sprintf("%s reloaded successfully", capitalizeFirst(domain))), nil
}

func (h *ConsolidatedHelperHandlers) handleGroupEntities(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any, isAdd bool) (*mcp.ToolsCallResult, error) {
	if domain != platformGroup {
		return errorResult("add_entities/remove_entities actions are only supported for group helpers"), nil
	}

	var entities []any
	var key, verb string
	var ok bool
	if isAdd {
		key = "add_entities"
		verb = "added to"
		entities, ok = args["add_entities"].([]any)
	} else {
		key = "remove_entities"
		verb = "removed from"
		entities, ok = args["remove_entities"].([]any)
	}

	if !ok {
		return errorResult(fmt.Sprintf("%s parameter is required", key)), nil
	}
	if len(entities) == 0 {
		return errorResult(fmt.Sprintf("%s must contain at least one entity", key)), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		key:         entities,
	}

	if _, err := client.CallService(ctx, platformGroup, "set", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error modifying group: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Entities %s '%s' successfully", verb, entityID)), nil
}

// =============================================================================
// Config Builders
// =============================================================================

// configEntryUpdateContext groups the two platform-identifying strings needed
// to build a config-entry helper's update payload. Both entityDomain and
// minMaxPlatform are plain strings with unrelated meanings (the entity's HA
// domain vs. a resolved min_max integration platform, only ever non-empty
// when the caller supplied min_max_type) - as adjacent positional parameters
// of the same type they were silently transposable at any call site with no
// compiler error. Named fields close that off.
type configEntryUpdateContext struct {
	entityDomain   string
	minMaxPlatform string
}

// buildConfigEntryUpdateConfig builds a loose config for Config Entry helper
// updates. Extracts all recognized Config Entry fields from args.
//
// Deliberately never reads "entity_id" - see isUpdateIdentifierField for why
// it's the tool's own "which helper are we updating" identifier, not a
// per-platform config value. Forwarding it once silently overwrote e.g. a
// threshold's monitored entity with the helper's own id (see CLAUDE.md's
// "buildConfigEntryUpdateConfig leaked entity_id" gotcha).
//
//nolint:gocyclo // Routing to type-specific builders requires switch over all helper types
func buildConfigEntryUpdateConfig(entryCtx configEntryUpdateContext, args map[string]any) (map[string]any, error) {
	config := make(map[string]any)
	r := newArgReader(config, args)

	// Common fields
	r.str("name")
	r.str("icon")

	// Template helper fields
	r.str("state")
	r.strID("source")
	r.str("unit_of_measurement")
	// device_class is skipped here for template subtype domains whose
	// OPTIONS_FLOW schema doesn't declare it (issue #206) - most of the 15
	// don't (see perTypeUpdateExcludedFields' doc comment); template_number
	// is the one exception, and deviceClassSupportedOnTemplateUpdate is what
	// lets it through while still excluding the other 14. Reading it here
	// unconditionally for every template domain, as an earlier version of
	// this fix did, made splitAppliedFields report a caller-supplied
	// device_class as "applied" for those 14 even though no options flow
	// step ever accepted it.
	if deviceClassSupportedOnTemplateUpdate(entryCtx.entityDomain) {
		r.str("device_class")
	}
	r.str("state_class")

	// Threshold helper fields
	r.num("lower")
	r.num("upper")
	r.num("hysteresis")

	// Derivative/Integral helper fields. time_window is read here as a
	// plain string, but internal/homeassistant's isDurationField reinterprets
	// it by name and converts it to a duration dict before submission - a
	// new duration-shaped field added here must also be added to that list.
	r.integer("round")
	r.str("time_window")
	r.str("unit_time")
	r.str("unit_prefix")
	r.str("method")

	// Group helper fields
	r.anySlice("entities")
	r.boolean("all")
	r.str("group_type")

	// Template binary sensor fields. delay_on/delay_off are read here as
	// plain strings but reinterpreted as durations by name in
	// internal/homeassistant's isDurationField - see the time_window
	// comment above.
	r.str("delay_on")
	r.str("delay_off")

	if err := r.err(); err != nil {
		return nil, err
	}

	// Add fields for extended helper types
	if err := addExtendedConfigEntryFields(config, args, entryCtx); err != nil {
		return nil, err
	}

	return config, nil
}

// configBuilderFunc is a function that builds type-specific helper configuration.
type configBuilderFunc func(config, args map[string]any) error

// helperConfigBuilders maps helper types to their configuration builders.
var helperConfigBuilders = buildHelperConfigBuildersRegistry()

// buildHelperConfigBuildersRegistry returns the base configBuilderFunc
// registry, merged with per-subtype builders for the template_* subtypes
// (see helpers_template_types.go).
func buildHelperConfigBuildersRegistry() map[string]configBuilderFunc {
	m := map[string]configBuilderFunc{
		platformInputBoolean:           buildInputBooleanConfig,
		platformInputButton:            buildInputButtonConfig,
		platformInputNumber:            buildInputNumberConfig,
		platformInputText:              buildInputTextConfig,
		platformInputSelect:            buildInputSelectConfig,
		platformInputDatetime:          buildInputDatetimeConfig,
		platformCounter:                buildCounterConfig,
		platformTimer:                  buildTimerConfig,
		platformSchedule:               buildScheduleConfig,
		platformGroup:                  buildGroupConfig,
		helperTypeTemplateSensor:       buildTemplateSensorConfig,
		helperTypeTemplateBinarySensor: buildTemplateBinarySensorConfig,
		"threshold":                    buildThresholdConfig,
		"derivative":                   buildDerivativeConfig,
		"integral":                     buildIntegralConfig,
		platformUtilityMeter:           buildUtilityMeterConfig,
		platformMinMax:                 buildMinMaxConfig,
		platformStatistics:             buildStatisticsConfig,
		platformTrend:                  buildTrendConfig,
		helperTypeRandomSensor:         buildRandomSensorConfig,
		helperTypeRandomBinarySensor:   buildRandomBinarySensorConfig,
		platformFilter:                 buildFilterConfig,
		platformTod:                    buildTodConfig,
		platformGenericThermostat:      buildGenericThermostatConfig,
		platformSwitchAsX:              buildSwitchAsXConfig,
		platformGenericHygrostat:       buildGenericHygrostatConfig,
	}
	for typeName := range templateSubtypes {
		m[typeName] = buildTemplateHelperConfig(typeName)
	}
	return m
}

func buildHelperConfig(helperType, name string, args map[string]any) (map[string]any, error) {
	config := map[string]any{"name": name}
	r := newArgReader(config, args)
	r.str("icon")
	if err := r.err(); err != nil {
		return config, err
	}

	if builder, exists := helperConfigBuilders[helperType]; exists {
		return config, builder(config, args)
	}

	return config, nil
}

func buildInputBooleanConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.boolean("initial")
	return r.err()
}

func buildInputButtonConfig(_, _ map[string]any) error {
	return nil
}

func buildInputNumberConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.num("min")
	r.num("max")
	r.num("step")
	r.num("initial")
	r.str("mode")
	r.str("unit_of_measurement")
	if err := r.err(); err != nil {
		return err
	}
	if minVal, hasMin := config["min"].(float64); hasMin {
		if maxVal, hasMax := config["max"].(float64); hasMax {
			if err := ValidateRange(minVal, maxVal, "input_number"); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildInputTextConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.num("min")
	r.num("max")
	r.str("mode")
	r.str("pattern")
	r.str("initial")
	return r.err()
}

func buildInputSelectConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.str("initial")
	r.strSlice("options")
	return r.err()
}

func buildInputDatetimeConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.boolean("has_date")
	r.boolean("has_time")
	r.str("initial")
	return r.err()
}

func buildCounterConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.integer("initial")
	r.integer("step")
	r.integer("minimum")
	r.integer("maximum")
	r.boolean("restore")
	if err := r.err(); err != nil {
		return err
	}
	if minVal, hasMin := config["minimum"].(int); hasMin {
		if maxVal, hasMax := config["maximum"].(int); hasMax {
			if err := ValidateRange(float64(minVal), float64(maxVal), "counter"); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildTimerConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.str("duration")
	r.boolean("restore")
	return r.err()
}

func buildScheduleConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		r.anySlice(day)
	}
	return r.err()
}

func buildGroupConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.boolean("all")
	r.str("group_type")
	r.anySlice("entities")
	return r.err()
}

func buildTemplateSensorConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.str("state")
	r.str("unit_of_measurement")
	r.str("state_class")
	config["template_type"] = "sensor"
	r.str("device_class")
	return r.err()
}

// buildTemplateBinarySensorConfig builds configuration for a template
// binary_sensor helper. delay_on/delay_off are read here as plain strings
// but reinterpreted as durations by name in internal/homeassistant's
// isDurationField - see buildDerivativeConfig's time_window comment.
func buildTemplateBinarySensorConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.str("state")
	r.str("delay_on")
	r.str("delay_off")
	config["template_type"] = "binary_sensor"
	r.str("device_class")
	return r.err()
}

func buildThresholdConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.num("lower")
	r.num("upper")
	r.num("hysteresis")
	r.str("device_class")
	r.strID("entity_id")
	return r.err()
}

// buildDerivativeConfig builds configuration for derivative helper.
// time_window is read here as a plain string but reinterpreted as a
// duration by name in internal/homeassistant's isDurationField - a new
// duration-shaped field must also be added to that list to convert
// correctly.
func buildDerivativeConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.integer("round")
	r.str("time_window")
	r.str("unit_time")
	r.str("unit_prefix")
	r.strID("source")
	return r.err()
}

func buildIntegralConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.str("method")
	r.integer("round")
	r.str("unit_time")
	r.str("unit_prefix")
	r.strID("source")
	return r.err()
}

// =============================================================================
// Validation Helpers
// =============================================================================

func validateRequiredFields(helperType string, meta helperTypeMetadata, args map[string]any) error {
	for _, field := range meta.requiredFields {
		if err := validateSingleField(field, helperType, args); err != nil {
			return err
		}
	}
	return nil
}

// checkSourceEntityDomain validates that every source entity referenced by a
// domain-restricted Config Entry helper (utility_meter, statistics, trend,
// filter, generic_thermostat, switch_as_x, generic_hygrostat) has the domain
// (and, where constrained, device_class) HA's config flow actually requires,
// failing with an actionable wrapper recipe instead of letting an opaque
// HA-side validation error through. A type may constrain more than one field
// (generic_thermostat and generic_hygrostat each constrain both their
// actuator field and target_sensor_entity_id); every constraint is checked,
// not just the first. The device_class check only fires when the domain
// already matches and the constraint declares deviceClasses (currently only
// generic_thermostat/generic_hygrostat's target_sensor_entity_id, which HA
// requires to carry device_class=temperature/humidity respectively, not just
// domain=sensor) - and it fetches the source entity's state to read that
// attribute, so a fetch failure degrades to the domain-only result already
// computed rather than failing the whole call, mirroring
// checkUpdateSourceEntityDomain's registry-lookup-failure convention.
func checkSourceEntityDomain(ctx context.Context, client homeassistant.Client, helperType string, constraints []sourceEntityConstraint, args map[string]any) error {
	var errs []error
	for _, constraint := range constraints {
		if len(constraint.domains) == 0 {
			continue // malformed constraint (should never happen - see sourceEntityConstraint's doc comment); nothing to validate against
		}
		sourceEntityID, _ := args[constraint.field].(string)
		if sourceEntityID == "" {
			continue // missing entirely - let the existing validateRequiredFields report it, don't duplicate that error
		}
		domain := extractDomain(sourceEntityID)
		if !slices.Contains(constraint.domains, domain) {
			msg := fmt.Sprintf(
				"%q field %q requires a %s.* entity, got %q (domain %q).",
				helperType, constraint.field, strings.Join(constraint.domains, "/"), sourceEntityID, domain,
			)
			// wrapperRecipeFor returns "" for a malformed sourceEntityID rather
			// than interpolating it into the recipe - see its doc comment.
			if recipe := wrapperRecipeFor(constraint, sourceEntityID); recipe != "" {
				msg += " " + recipe
			}
			errs = append(errs, errors.New(msg))
			continue
		}
		if err := checkSourceEntityDeviceClass(ctx, client, helperType, constraint, sourceEntityID); err != nil {
			errs = append(errs, err)
		}
	}
	// generic_thermostat/generic_hygrostat constrain two fields each - join
	// so a caller who gets both wrong sees both problems in one round-trip
	// instead of fixing one, retrying, and hitting the other.
	return errors.Join(errs...)
}

// checkSourceEntityDeviceClass returns an error if sourceEntityID's
// device_class doesn't match constraint.deviceClasses, or nil if the
// constraint declares no device_class requirement, the entity already
// matches, or the state fetch fails (degrades to unchecked - see
// checkSourceEntityDomain's doc comment).
func checkSourceEntityDeviceClass(ctx context.Context, client homeassistant.Client, helperType string, constraint sourceEntityConstraint, sourceEntityID string) error {
	if len(constraint.deviceClasses) == 0 {
		return nil
	}
	state, err := client.GetState(ctx, sourceEntityID)
	if err != nil {
		slog.WarnContext(ctx, "source-entity device_class validation skipped: state fetch failed",
			"entity_id", sourceEntityID, "error", err)
		return nil //nolint:nilerr // fetch failure degrades to unchecked, see checkSourceEntityDomain's doc comment
	}
	deviceClass, _ := state.Attributes["device_class"].(string)
	if slices.Contains(constraint.deviceClasses, deviceClass) {
		return nil
	}
	msg := fmt.Sprintf(
		"%q field %q requires a %s.* entity with device_class %s, got %q (device_class %q).",
		helperType, constraint.field, strings.Join(constraint.domains, "/"), strings.Join(constraint.deviceClasses, "/"), sourceEntityID, deviceClass,
	)
	if recipe := wrapperRecipeFor(constraint, sourceEntityID); recipe != "" {
		msg += " " + recipe
	}
	return errors.New(msg)
}

// sourceConstrainedTypes indexes helperTypes entries that have at least one
// sourceEntityConstraint, keyed by their platform name. Used by
// checkUpdateSourceEntityDomain to resolve the real integration platform via
// the entity registry - ParseHelperEntityID (used elsewhere in handleUpdate)
// only recovers the entity DOMAIN (e.g. "sensor" for a statistics helper),
// which is not sufficient to look up helperTypes by key. All 7 currently
// constrained types have map key == platform, so today this index is a
// clean 1:1 - but that is NOT structurally guaranteed: helperTypes already
// has two entries sharing platformTemplate (template_sensor,
// template_binary_sensor), and if either one ever gains a sourceEntities
// entry, buildSourceConstrainedTypes below would silently keep whichever one
// Go's map iteration visits last. TestSourceConstrainedTypes_PlatformsAreUnique
// guards against that collision going unnoticed.
var sourceConstrainedTypes = buildSourceConstrainedTypes()

func buildSourceConstrainedTypes() map[string]helperTypeMetadata {
	result := make(map[string]helperTypeMetadata, len(helperTypes))
	for _, meta := range helperTypes {
		if len(meta.sourceEntities) > 0 {
			result[meta.platform] = meta
		}
	}
	return result
}

// updatableSourceEntities filters out any constraint whose field is the
// update-identifier field (see isUpdateIdentifierField). There is no
// caller-suppliable value behind that constraint on update. Validating it
// anyway would either vacuously pass (statistics/trend/filter, whose own
// domain always matches) or always fail (switch_as_x, whose own domain
// never matches "switch").
func updatableSourceEntities(constraints []sourceEntityConstraint) []sourceEntityConstraint {
	filtered := make([]sourceEntityConstraint, 0, len(constraints))
	for _, c := range constraints {
		if isUpdateIdentifierField(c.field) {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

// updatableSourceEntityFields is the set of args keys any
// checkUpdateSourceEntityDomain call could possibly validate - every field
// name updatableSourceEntities(meta.sourceEntities) keeps, across every
// source-constrained type, built once from sourceConstrainedTypes so it can
// never drift from the constraint table. Lets checkUpdateSourceEntityDomain
// skip its entity-registry fetch entirely when the caller's update args
// touch none of these fields: checkSourceEntityDomain would return nil
// anyway in that case (it `continue`s past every constraint whose field is
// absent from args), so the short-circuit reaches the same answer without
// paying for a fetch - a real cost, since HybridClient.UpdateHelper does its
// own registry lookup right after via c.ws.GetEntityRegistry, which
// bypasses CachedClient entirely.
var updatableSourceEntityFields = buildUpdatableSourceEntityFields()

func buildUpdatableSourceEntityFields() map[string]bool {
	fields := make(map[string]bool)
	for _, meta := range sourceConstrainedTypes {
		for _, c := range updatableSourceEntities(meta.sourceEntities) {
			fields[c.field] = true
		}
	}
	return fields
}

func hasAnyUpdatableSourceEntityField(args map[string]any) bool {
	for field := range updatableSourceEntityFields {
		if _, present := args[field]; present {
			return true
		}
	}
	return false
}

// preExistingHelperOnlyDomains are the entity domains helperTypes already
// legitimately owned before issue #206's 15 template_* subtypes (sensor,
// binary_sensor, climate, humidifier, select) plus the original WS-helper
// domains (input_*, counter, timer, schedule, group) - domains
// checkHelperOnlyDomain deliberately does NOT gate. They have the same
// real-integration-collision ambiguity in principle (sensor/binary_sensor
// already had it before this branch), but are partially covered by
// checkUpdateSourceEntityDomain/resolveConfigEntryPlatformForMinMaxType
// already; conflating the two would grow this fix well beyond the
// regression it's closing. This is the one hand-maintained literal in this
// group - newlyWidenedHelperDomains below is derived from it plus
// helperTypes so it can't drift out of sync with a newly added helper type.
var preExistingHelperOnlyDomains = map[string]bool{
	"sensor": true, "binary_sensor": true, "climate": true, "humidifier": true, "select": true,
	"input_boolean": true, "input_number": true, "input_text": true, "input_select": true,
	"input_datetime": true, "input_button": true, "counter": true, "timer": true,
	"schedule": true, "group": true,
}

// newlyWidenedHelperDomains is the set of entity domains HelperPlatforms
// (helper_common.go) gained for the 15 new template_* subtypes (issue
// #206) plus switch_as_x's siren/valve targets - domains a real,
// non-helper integration can also create entities in (a Hue light is
// domain "light" too, a Zigbee plug is domain "switch"). Derived from
// helperTypes' validEntityDomains minus preExistingHelperOnlyDomains, so a
// future helper type touching a new domain is automatically covered by
// checkHelperOnlyDomain without anyone having to remember to update this
// set by hand - the exact drift risk a hand-maintained list here would
// otherwise reopen.
var newlyWidenedHelperDomains = buildNewlyWidenedHelperDomains()

func buildNewlyWidenedHelperDomains() map[string]bool {
	m := map[string]bool{}
	for _, meta := range helperTypes {
		for _, domain := range meta.validEntityDomains {
			if !preExistingHelperOnlyDomains[domain] {
				m[domain] = true
			}
		}
	}
	return m
}

// widenedHelperOnlyDomains maps each of newlyWidenedHelperDomains to the
// set of real HA integration platforms allowed to own an entity there,
// derived from helperTypes' validEntityDomains so it can't drift (e.g.
// switch_as_x legitimately owns light.*/cover.*/.../valve.*, the 15
// template_* subtypes each legitimately own exactly their own domain).
var widenedHelperOnlyDomains = buildWidenedHelperOnlyDomains()

func buildWidenedHelperOnlyDomains() map[string]map[string]bool {
	m := make(map[string]map[string]bool, len(newlyWidenedHelperDomains))
	for _, meta := range helperTypes {
		for _, domain := range meta.validEntityDomains {
			if !newlyWidenedHelperDomains[domain] {
				continue
			}
			if m[domain] == nil {
				m[domain] = map[string]bool{}
			}
			m[domain][meta.platform] = true
		}
	}
	return m
}

// checkHelperOnlyDomain rejects handleUpdate's target entity when its
// domain is one of newlyWidenedHelperDomains and the entity registry's
// actual platform isn't one of the platforms widenedHelperOnlyDomains
// allows for that domain. Before this check, ParseHelperEntityID's
// domain-only match was handleUpdate's only gate: a real, non-helper
// entity sharing one of these domains (light.hue_office, a Zigbee
// switch.*, ...) would reach client.UpdateHelper, which routes any entity
// with a config_entry_id to an Options Flow submission against THAT
// entity's real config entry - silently editing/reloading a real
// integration instead of a helper. Unlike checkUpdateSourceEntityDomain, a
// registry fetch failure or a not-found entity hard-fails rather than
// degrading to an unchecked update: proceeding unchecked is exactly the
// gap this closes, so there is nothing safe to degrade to.
func checkHelperOnlyDomain(ctx context.Context, client homeassistant.Client, entityID, entityDomain string) error {
	allowed, gated := widenedHelperOnlyDomains[entityDomain]
	if !gated {
		return nil
	}
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return fmt.Errorf("cannot verify %s is a helper entity: entity registry fetch failed: %w", entityID, err)
	}
	for _, entry := range entries {
		if entry.EntityID != entityID {
			continue
		}
		if allowed[entry.Platform] {
			return nil
		}
		return fmt.Errorf(
			"%s is not a helper entity - it belongs to the %q integration; manage_helper only manages helper-created entities",
			entityID, entry.Platform,
		)
	}
	return fmt.Errorf("cannot verify %s is a helper entity: entity not found in entity registry", entityID)
}

// checkUpdateSourceEntityDomain re-validates domain-constrained source
// fields on update (create-time validation alone left helpers with domain-
// constrained source fields updatable into a mismatched source with no
// preflight, since update never reused checkSourceEntityDomain). A registry lookup failure degrades to an
// unchecked update rather than blocking a legitimate edit, mirroring
// configWriteGuardError's "checked=false, proceed anyway" convention
// (yaml_defined.go:19-20) - unlike mergeCurrentHelperState's merge-fetch
// failure, skipping this check only skips a validation, not a data-loss risk.
//
// This fetch, checkHelperOnlyDomain's own fetch, and
// HybridClient.UpdateHelper's routing fetch immediately afterward are not
// deduplicated - up to three GetEntityRegistry round-trips per update.
// UpdateHelper's fetch goes through its internal c.ws.GetEntityRegistry
// (bypassing CachedClient by construction, the same way DeleteHelper's
// routing fetch does), so even routing this call through a caching client
// would not save that round-trip; checkHelperOnlyDomain's fetch only fires
// for the 16 domains gated in widenedHelperOnlyDomains. Sharing one fetch
// across the handlers/homeassistant package boundary would mean threading
// entity-registry entries through the Client interface's UpdateHelper
// signature - a bigger interface change than the fetch cost justifies
// today; hasAnyUpdatableSourceEntityField already skips this particular
// fetch entirely for the common case where the update touches no
// source-constrained field.
func checkUpdateSourceEntityDomain(ctx context.Context, client homeassistant.Client, entityID string, args map[string]any) error {
	if !hasAnyUpdatableSourceEntityField(args) {
		return nil
	}
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		// Degrades to an unchecked update rather than blocking a legitimate
		// edit (see doc comment above), but a permanently-broken registry
		// fetch is otherwise indistinguishable from a passing check - log it
		// so the gap is at least visible.
		slog.WarnContext(ctx, "source-domain update validation skipped: entity registry fetch failed",
			"entity_id", entityID, "error", err)
		return nil //nolint:nilerr // registry lookup failure degrades to an unchecked update, see doc comment above
	}
	for _, entry := range entries {
		if entry.EntityID != entityID {
			continue
		}
		meta, ok := sourceConstrainedTypes[entry.Platform]
		if !ok {
			return nil
		}
		return checkSourceEntityDomain(ctx, client, entry.Platform, updatableSourceEntities(meta.sourceEntities), args)
	}
	slog.WarnContext(ctx, "source-domain update validation skipped: entity not found in registry", "entity_id", entityID)
	return nil
}

// resolveConfigEntryPlatformForMinMaxType resolves the real integration
// platform for entityID via the entity registry. It exists to gate
// min_max_type out of addExtendedConfigEntryFields for every config-entry
// helper type other than min_max: that builder is a one-size-fits-all
// update payload shared by template, threshold, group (sensor-domain),
// statistics, ... - none of which have a helperTypes key matching their
// entity domain, so they all reach buildConfigEntryUpdateConfig the same
// way min_max does. Without this check, a caller could pass min_max_type on
// e.g. a sensor-domain group update and silently rewrite its aggregation
// type, since HA's group CONF_TYPE enum is a superset of min_max's.
//
// Unlike checkUpdateSourceEntityDomain, a lookup failure here does NOT
// degrade to an unchecked update: that function only skips a validation on
// failure, but silently dropping min_max_type here would report the update
// as successful while discarding the one field the caller asked to change -
// the same class of risk mergeCurrentHelperState guards against for
// WebSocket helpers (see CLAUDE.md's "Merge-fetch failure hard-fails the
// update" gotcha).
func resolveConfigEntryPlatformForMinMaxType(ctx context.Context, client homeassistant.Client, entityID string) (string, error) {
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot verify min_max_type applies to %s: entity registry fetch failed: %w", entityID, err)
	}
	for _, entry := range entries {
		if entry.EntityID == entityID {
			return entry.Platform, nil
		}
	}
	return "", fmt.Errorf("cannot verify min_max_type applies to %s: entity not found in entity registry", entityID)
}

// wrapperRecipeFor returns an actionable next step for a source-domain
// mismatch, or "" if sourceEntityID is not a well-formed entity_id. The
// well-formedness check is enforced here, at the sink, rather than at each
// call site: wrapperRecipeFor interpolates sourceEntityID unescaped into a
// Jinja template string inside a ready-to-run manage_helper call, and a
// malformed value (e.g. containing "') }}{{ ") could otherwise turn an
// error message into a template-injection payload that an agent might
// copy-paste and execute. When "sensor" is an allowed domain, it's a
// ready-to-run manage_helper template_sensor wrapper recipe (manage_helper
// can create that type today) - and includes the constraint's device_class
// when one is required (e.g. generic_thermostat's target_sensor_entity_id
// needs device_class=temperature), since a wrapper without it would just
// trade this error for HA's device_class rejection. When "switch" is
// allowed instead, it's an honest pointer at the Home Assistant UI, since
// manage_helper cannot create a template switch wrapper yet - fabricating a
// manage_helper call for a helper type that doesn't exist would just trade
// one failure for another. Returns "" when neither applies - the caller's
// message already lists the allowed domains, and no better recipe exists.
func wrapperRecipeFor(constraint sourceEntityConstraint, sourceEntityID string) string {
	if ValidateEntityID(sourceEntityID) != nil {
		return ""
	}
	if slices.Contains(constraint.domains, platformSensorEntity) {
		deviceClass := ""
		if len(constraint.deviceClasses) > 0 {
			deviceClass = fmt.Sprintf(", device_class=%q", constraint.deviceClasses[0])
		}
		return fmt.Sprintf(
			`Wrap it first: manage_helper(action="create", type="template_sensor", id="<wrapper_id>", name="<Wrapper Name>", state="{{ states('%s') | float(0) }}", unit_of_measurement="<unit>", state_class="measurement"%s)`,
			sourceEntityID, deviceClass,
		)
	}
	if slices.Contains(constraint.domains, "switch") {
		return fmt.Sprintf(
			"manage_helper cannot create a template switch wrapper yet - create one via "+
				"Settings → Devices & Services → Helpers → Template → Switch in the Home "+
				"Assistant UI, with turn_on/turn_off calling input_boolean.turn_on/turn_off "+
				"on %s, then retry with the wrapper's entity_id",
			sourceEntityID,
		)
	}
	return ""
}

//nolint:gocyclo // Validation switch with many specific field types
func validateSingleField(field, helperType string, args map[string]any) error {
	switch field {
	case "options", "entities", "entity_ids":
		return validateNonEmptyArrayField(field, helperType, args)
	case "state":
		return validateNonEmptyStringField(field, "Jinja2 template", helperType, args)
	case configKeyEntityID:
		return validateNonEmptyStringField(field, "source entity", helperType, args)
	case "source":
		return validateNonEmptyStringField(field, "source sensor entity ID", helperType, args)
	case "min", "max":
		// Accept the same numeric-or-numeric-string values the config
		// builders coerce via argReader.num, so a required min/max isn't
		// rejected here only to have succeeded downstream.
		if _, ok := toFloat(args[field]); !ok {
			return fmt.Errorf("%s must be a number for %s", field, helperType)
		}
	default:
		// Presence-only: whether a present value is well-formed for this
		// field is the config builder's job (argReader reports a proper
		// type-mismatch error there). Every default-case field happens to
		// be string-typed today, but asserting to string here and treating
		// a failed assertion as "absent" would misreport "wrong type" as
		// "is required" - a lie, since the caller did supply it.
		if v, exists := args[field]; !exists || isSkippable(v) {
			return fmt.Errorf("%s is required for %s", field, helperType)
		}
	}
	return nil
}

// validateNonEmptyArrayField validates args[field] is a non-empty []any,
// the shape shared by options/entities/entity_ids - extracted to keep
// validateSingleField's cognitive complexity down.
func validateNonEmptyArrayField(field, helperType string, args map[string]any) error {
	arr, ok := args[field].([]any)
	if !ok {
		return fmt.Errorf("%s is required for %s and must be an array", field, helperType)
	}
	if len(arr) == 0 {
		return fmt.Errorf("%s must be a non-empty array for %s", field, helperType)
	}
	return nil
}

// validateNonEmptyStringField validates args[field] is a non-empty string,
// the shape shared by state/entity_id/source - extracted to keep
// validateSingleField's cognitive complexity down. description is the
// human-readable parenthetical each field's error message already used
// (e.g. "Jinja2 template", "source entity").
func validateNonEmptyStringField(field, description, helperType string, args map[string]any) error {
	s, _ := args[field].(string)
	if s == "" {
		return fmt.Errorf("%s (%s) is required for %s", field, description, helperType)
	}
	return nil
}

func isValidAction(action string) bool {
	for _, a := range allSupportedActions {
		if a == action {
			return true
		}
	}
	return false
}

// =============================================================================
// Utility Functions
// =============================================================================

func errorResult(msg string) *mcp.ToolsCallResult {
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(msg)},
		IsError: true,
	}
}

func successResult(msg string) *mcp.ToolsCallResult {
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(msg)},
	}
}

func formatHelperType(helperType string) string {
	switch helperType {
	case "template_sensor":
		return "Template sensor"
	case "template_binary_sensor":
		return "Template binary sensor"
	default:
		return strings.ReplaceAll(helperType, "_", " ")
	}
}

func convertToStringSlice(arr []any) []string {
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// slugifyName converts a name to a valid helper ID using the same logic as automation IDs.
func slugifyName(name string) string {
	return generateAutomationID(name)
}

// groupEntityPrefix derives the entity prefix for group helpers based on member entity domains.
// Returns the domain of the first member entity, or "group" as fallback.
func groupEntityPrefix(config map[string]any) string {
	if entities, ok := config["entities"].([]any); ok && len(entities) > 0 {
		if first, ok := entities[0].(string); ok {
			if idx := strings.Index(first, "."); idx > 0 {
				return first[:idx]
			}
		}
	}
	return "group"
}

// =============================================================================
// Schedule Detail Helpers
// =============================================================================

// parseTimeBlocks extracts time blocks from schedule config for each day.
func parseTimeBlocks(config map[string]any) map[string][]map[string]string {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	result := make(map[string][]map[string]string)

	for _, day := range days {
		if daySchedule, ok := config[day].([]any); ok {
			if blocks := parseDaySchedule(daySchedule); len(blocks) > 0 {
				result[day] = blocks
			}
		}
	}

	return result
}

// parseDaySchedule parses time blocks for a single day.
func parseDaySchedule(daySchedule []any) []map[string]string {
	blocks := make([]map[string]string, 0, len(daySchedule))
	for _, block := range daySchedule {
		if timeBlock := parseTimeBlock(block); len(timeBlock) > 0 {
			blocks = append(blocks, timeBlock)
		}
	}
	return blocks
}

// parseTimeBlock parses a single time block with from/to times.
func parseTimeBlock(block any) map[string]string {
	blockMap, ok := block.(map[string]any)
	if !ok {
		return nil
	}
	timeBlock := make(map[string]string)
	if from, ok := blockMap["from"].(string); ok {
		timeBlock["from"] = from
	}
	if to, ok := blockMap["to"].(string); ok {
		timeBlock["to"] = to
	}
	return timeBlock
}

// buildScheduleDetails creates a formatted details object for schedule entities.
func buildScheduleDetails(state *homeassistant.Entity, timeBlocks map[string][]map[string]string) map[string]any {
	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	if len(timeBlocks) > 0 {
		details["schedule"] = timeBlocks
	}

	if nextEvent, ok := state.Attributes["next_event"].(string); ok && nextEvent != "" {
		details["next_event"] = nextEvent
	}

	return details
}

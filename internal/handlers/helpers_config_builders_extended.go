// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

// genericThermostatPresetFieldNames returns generic_thermostat's optional
// preset temperature fields (HA's PRESETS_SCHEMA, CONF_PRESETS.values()) -
// the tool arg name matches HA's own config key exactly, no renaming
// needed. Shared by the create and update builders so the two field lists
// cannot drift. Returns a fresh slice on every call so callers appending
// to it (e.g. helperTypes' optionalFields) can never alias or mutate a
// shared backing array.
func genericThermostatPresetFieldNames() []string {
	names := make([]string, len(genericThermostatPresets))
	for i, p := range genericThermostatPresets {
		names[i] = p.field
	}
	return names
}

// genericThermostatPreset pairs a preset config field with the label used
// in its generated schema description.
type genericThermostatPreset struct {
	field string
	label string
}

// genericThermostatPresets is the single source of truth for
// generic_thermostat's six optional preset temperature fields (HA's
// CONF_PRESETS.values()) - genericThermostatPresetFieldNames() (read by the
// create/update config builders) and buildGenericThermostatPresetSchema()
// (the manage_helper JSON schema, helpers_schema_extended.go) both derive
// from this one list, so a field can't be added to the builders' side
// without also appearing in the schema, or vice versa.
var genericThermostatPresets = []genericThermostatPreset{
	{field: "away_temp", label: "Away"},
	{field: "eco_temp", label: "Eco"},
	{field: "home_temp", label: "Home"},
	{field: "comfort_temp", label: "Comfort"},
	{field: "sleep_temp", label: "Sleep"},
	{field: "activity_temp", label: "Activity"},
}

// buildUtilityMeterConfig builds configuration for utility_meter helper.
func buildUtilityMeterConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.strID("source")
	r.str("cycle")
	r.num("offset")
	r.boolean("delta_values")
	r.boolean("net_consumption")
	r.boolean("periodically_resetting")
	r.strSlice("tariffs")
	return r.err()
}

// buildMinMaxConfig builds configuration for min_max helper.
func buildMinMaxConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.strSlice("entity_ids")
	// min_max_type maps to HA's "type" config field. Kept distinct from the
	// tool's own top-level helper-type selector ("type": "min_max"), which
	// shares the same args map - reading args["type"] here would always see
	// the selector value instead of the caller's intended calculation.
	r.strAs("min_max_type", "type")
	r.integer("round_digits")
	return r.err()
}

// buildStatisticsConfig builds configuration for statistics helper.
func buildStatisticsConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.strID("entity_id")
	r.str("state_characteristic")
	r.integer("sampling_size")
	r.str("max_age")
	r.num("percentile")
	r.integer("precision")
	if err := r.err(); err != nil {
		return err
	}
	// HA's state_characteristic step schema is vol.Required with no HA-side
	// default; this was previously injected by buildConfigForFlowStep's
	// hardcoded statistics step handling. Moved here so it applies
	// regardless of how the flow-step submission is built.
	if _, ok := config["state_characteristic"]; !ok {
		config["state_characteristic"] = "mean"
	}
	return nil
}

// buildTrendConfig builds configuration for trend helper.
func buildTrendConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.strID("entity_id")
	r.num("min_gradient")
	r.integer("min_samples")
	r.num("sample_duration")
	r.integer("max_samples")
	r.boolean("invert")
	return r.err()
}

// buildRandomSensorConfig builds configuration for random sensor helper.
func buildRandomSensorConfig(config, args map[string]any) error {
	// Set type routing field for menu navigation
	config["type"] = "sensor"

	r := newArgReader(config, args)
	r.num("minimum")
	r.num("maximum")
	return r.err()
}

// buildRandomBinarySensorConfig builds configuration for random binary_sensor helper.
func buildRandomBinarySensorConfig(config, _ map[string]any) error {
	// Set type routing field for menu navigation
	config["type"] = "binary_sensor"

	return nil
}

// buildFilterConfig builds configuration for filter helper. window_size is
// read via raw() rather than a typed reader because its valid shape
// depends on the filter type: a plain sample-count number for
// outlier/lowpass/throttle, but a duration for
// time_simple_moving_average/time_throttle - normalised later by
// hybrid_client.go's toDurationDict, the one place that owns the duration
// question. The "filters" array parameter is deliberately not supported:
// Home Assistant's config-entry flow for filter takes exactly one filter
// type per helper and rejects an additional "filters" key outright (see
// CLAUDE.md's filter gotcha).
func buildFilterConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.strID("entity_id")
	r.strID("filter")
	r.raw("window_size")
	r.num("radius")
	r.num("time_constant")
	r.num("lower_bound")
	r.num("upper_bound")
	r.integer("precision")
	return r.err()
}

// buildTodConfig builds configuration for tod (Time of Day) helper.
func buildTodConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	r.str("after_time")
	r.str("before_time")
	r.str("after_offset")
	r.str("before_offset")
	return r.err()
}

// buildGenericThermostatConfig builds configuration for generic_thermostat helper.
func buildGenericThermostatConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	// Map user-friendly field names to API field names
	// API expects: heater, target_sensor, ac_mode (not heater_entity_id, target_sensor_entity_id)
	r.strIDAs("heater_entity_id", "heater")
	r.strIDAs("target_sensor_entity_id", "target_sensor")

	// ac_mode is required by the API; default to heating mode when the
	// caller omits it (or sends an explicit null). A caller-supplied value
	// of the wrong type is a hard error via r.boolean, not silently
	// replaced by the default.
	r.boolean("ac_mode")
	if v, has := args["ac_mode"]; !has || isSkippable(v) {
		config["ac_mode"] = false
	}

	r.num("min_temp")
	r.num("max_temp")
	r.num("target_temp")
	r.num("cold_tolerance")
	r.num("hot_tolerance")
	for _, field := range genericThermostatPresetFieldNames() {
		r.num(field)
	}
	return r.err()
}

// buildSwitchAsXConfig builds configuration for switch_as_x helper.
func buildSwitchAsXConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	// Required: entity_id (switch entity)
	// name is unconditionally seeded by buildHelperConfig for every
	// config-entry type, but switch_as_x's flow schema has no "name" field
	// at all - it's simply never claimed by any step (see the
	// createHelperViaConfigFlow gotcha in CLAUDE.md).
	r.strID("entity_id")

	// target_domain is switch_as_x's real (and only) "user"-step field, not
	// a routing-only key - platformSkipFields does NOT filter it out.
	r.str("target_domain")

	r.boolean("invert")
	return r.err()
}

// buildGenericHygrostatConfig builds configuration for generic_hygrostat helper.
func buildGenericHygrostatConfig(config, args map[string]any) error {
	r := newArgReader(config, args)
	// Map user-friendly field names to API field names
	// API expects: humidifier, target_sensor, device_class (not humidifier_entity_id, target_sensor_entity_id)
	r.strIDAs("humidifier_entity_id", "humidifier")
	r.strIDAs("target_sensor_entity_id", "target_sensor")

	// device_class is required by API
	config["device_class"] = hygrostatDeviceClass

	r.num("min_humidity")
	r.num("max_humidity")
	r.num("target_humidity")
	r.num("dry_tolerance")
	r.num("wet_tolerance")
	return r.err()
}

// addExtendedConfigEntryFields adds extended helper fields to update config.
// This function is called by buildConfigEntryUpdateConfig to handle new helper types.
//
//nolint:funlen // Large number of fields for extended helper types
func addExtendedConfigEntryFields(config, args map[string]any, entryCtx configEntryUpdateContext) error {
	r := newArgReader(config, args)

	// utility_meter fields
	r.strID("source")
	r.str("cycle")
	r.num("offset")
	r.boolean("delta_values")
	r.boolean("net_consumption")
	r.boolean("periodically_resetting")
	r.strSlice("tariffs")

	// min_max fields
	r.strSlice("entity_ids")
	addMinMaxTypeField(r, entryCtx.minMaxPlatform)
	r.integer("round_digits")

	// statistics fields
	r.str("state_characteristic")
	r.integer("sampling_size")
	r.str("max_age")
	r.num("percentile")
	r.integer("precision")

	// trend fields
	r.num("min_gradient")
	r.integer("min_samples")
	r.num("sample_duration")
	r.integer("max_samples")
	r.boolean("invert")

	// random fields
	r.num("minimum")
	r.num("maximum")

	// filter fields
	r.raw("window_size")
	r.num("radius")
	r.num("time_constant")
	r.num("lower_bound")
	r.num("upper_bound")

	// tod fields
	r.str("after_time")
	r.str("before_time")
	r.str("after_offset")
	r.str("before_offset")

	// generic_thermostat fields (map to API field names)
	r.strIDAs("heater_entity_id", "heater")
	r.strIDAs("target_sensor_entity_id", "target_sensor")
	// Unlike buildGenericThermostatConfig's create path, ac_mode gets NO
	// default here when omitted - and that's deliberate, not a gap to
	// "fix" by mirroring create. buildStepSubmission's update-mode
	// round-trip (internal/homeassistant/flow_steps.go) already preserves
	// the helper's current ac_mode when userConfig omits the key; defaulting
	// it to false here would override that preserved value on every update
	// that doesn't explicitly pass ac_mode, defeating the merge entirely.
	r.boolean("ac_mode")
	r.num("min_temp")
	r.num("max_temp")
	r.num("target_temp")
	r.num("cold_tolerance")
	r.num("hot_tolerance")
	addGenericThermostatPresetFields(r, entryCtx.entityDomain)

	// switch_as_x fields
	r.str("target_domain")

	// generic_hygrostat fields (map to API field names)
	r.strIDAs("humidifier_entity_id", "humidifier")
	r.strIDAs("target_sensor_entity_id", "target_sensor")
	// device_class is required for hygrostat - only default it when the
	// helper being updated is actually a generic_hygrostat (its entity domain
	// is "humidifier"), not every config-entry helper type that shares this
	// one-size-fits-all update builder.
	if entryCtx.entityDomain == hygrostatEntityDomain {
		if _, exists := config["device_class"]; !exists {
			config["device_class"] = hygrostatDeviceClass
		}
	}
	r.num("min_humidity")
	r.num("max_humidity")
	r.num("target_humidity")
	r.num("dry_tolerance")
	r.num("wet_tolerance")

	// template subtype fields (button/cover/device_tracker/event/fan/image/
	// light/lock/number/select/switch/update/vacuum/weather/alarm_control_panel)
	addTemplateConfigEntryUpdateFields(r, entryCtx.entityDomain)

	return r.err()
}

// addMinMaxTypeField writes config["type"] from args["min_max_type"] only
// when minMaxPlatform == platformMinMax - i.e. handleUpdate has already
// verified, via resolveConfigEntryPlatformForMinMaxType, that the entity
// being updated is actually a min_max helper. addExtendedConfigEntryFields
// is a one-size-fits-all update builder shared by every config-entry helper
// type (template, threshold, sensor-domain group, statistics, ...); an
// unconditional write here previously let min_max_type leak into "type" for
// any of them, silently changing e.g. a sensor group's aggregation type
// since HA's group CONF_TYPE enum is a superset of min_max's.
func addMinMaxTypeField(r *argReader, minMaxPlatform string) {
	if minMaxPlatform != platformMinMax {
		return
	}
	r.strAs("min_max_type", "type")
}

// addGenericThermostatPresetFields reads generic_thermostat's optional
// preset temperature fields only when the helper being updated is
// actually a generic_thermostat (its entity domain is "climate") - mirrors
// addMinMaxTypeField's gate above and the device_class gate below.
// addExtendedConfigEntryFields is a one-size-fits-all update builder
// shared by every config-entry helper type; an unconditional read here
// would let away_temp/eco_temp/... leak into any of them even though no
// other type's Options Flow schema declares those keys.
func addGenericThermostatPresetFields(r *argReader, entityDomain string) {
	if entityDomain != thermostatEntityDomain {
		return
	}
	for _, field := range genericThermostatPresetFieldNames() {
		r.num(field)
	}
}

// updateConfigKeyAliases maps a manage_helper arg name to the config map
// key it lands under once the update builder runs, for the handful of
// fields Home Assistant's API renames on write (see CLAUDE.md's "Config
// Entry API Field Mapping" gotcha). splitAppliedFields
// (helpers_consolidated.go) uses this to report a renamed field as applied
// under the caller's own arg name rather than mistaking the rename for the
// field never having been read at all - the two contract tests
// (TestUpdatableFields_AreActuallyReadByUpdatePath,
// TestCreatableFields_AreActuallyReadByCreatePath) already depend on this
// exact table to look up the right config key.
var updateConfigKeyAliases = map[string]string{
	"heater_entity_id":        "heater",
	"target_sensor_entity_id": "target_sensor",
	"humidifier_entity_id":    "humidifier",
	"min_max_type":            "type",
	"fan_speed_list":          "fan_speeds",
	"lock_code_format":        "code_format",
	"options_template":        "options",
}

// configKeyToArgName is the reverse of updateConfigKeyAliases: HA's
// options/config flow reports rejected fields by their CONFIG key (e.g.
// "heater"), but a PartialApplyError warning reads better naming the
// caller's own arg name (e.g. "heater_entity_id") - the name actually used
// in the manage_helper call that produced it.
var configKeyToArgName = reverseAliasMap(updateConfigKeyAliases)

func reverseAliasMap(aliases map[string]string) map[string]string {
	reversed := make(map[string]string, len(aliases))
	for argName, configKey := range aliases {
		reversed[configKey] = argName
	}
	return reversed
}

// argNamesForConfigKeys translates a *homeassistant.PartialApplyError's
// Fields (HA's config key names) back to the caller's manage_helper arg
// names via configKeyToArgName, passing through any field with no known
// alias unchanged.
func argNamesForConfigKeys(configKeys []string) []string {
	argNames := make([]string, len(configKeys))
	for i, key := range configKeys {
		if argName, ok := configKeyToArgName[key]; ok {
			argNames[i] = argName
		} else {
			argNames[i] = key
		}
	}
	return argNames
}

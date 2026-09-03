package handlers

import (
	"fmt"
	"slices"
	"sync"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// templateFieldKind selects which argReader method reads a templateField's
// value and, via templateSchemaType, what schema Type (if any) the tool
// declares for it.
type templateFieldKind int

const (
	tplTemplate    templateFieldKind = iota // a Jinja template string
	tplAction                               // an HA ActionSelector value (argReader.actionValue)
	tplString                               // a plain string, not template-evaluated
	tplNumber                               // argReader.num
	tplBool                                 // argReader.boolean
	tplStringArray                          // argReader.strSlice
)

// templateField describes one manage_helper argument for a template
// subtype: its tool-facing name (arg), the key actually sent to Home
// Assistant when it differs from arg (haKey - empty means "same as arg"),
// which argReader method reads it, whether HA's CONFIG_FLOW schema
// requires it, and its schema description.
type templateField struct {
	arg      string
	haKey    string
	kind     templateFieldKind
	required bool
	desc     string
}

func (f templateField) configKey() string {
	if f.haKey != "" {
		return f.haKey
	}
	return f.arg
}

// templateSubtype describes one Home Assistant template platform domain
// (HA's TEMPLATE_TYPES) and the fields its CONFIG_FLOW/OPTIONS_FLOW schema
// declares. domain doubles as the entity domain (entityPrefix), the
// create-flow menu's next_step_id, and the template_type value HA stores -
// all three are the same string for every template subtype.
type templateSubtype struct {
	domain string
	fields []templateField
	// inclusivePairs lists field-name pairs HA's schema marks
	// vol.Inclusive: both must be supplied together, or neither. Only
	// cover's open/close pair has this today.
	inclusivePairs [][2]string
}

// templateSubtypes covers the 15 HA template domains beyond the existing
// template_sensor/template_binary_sensor (which keep their own
// hand-written builders - see buildTemplateSensorConfig/
// buildTemplateBinarySensorConfig). Every subtype implicitly also accepts
// "icon" (universal manage_helper field), "device_id" and "availability"
// (HA's CONF_DEVICE_ID/CONF_ADDITIONAL_OPTIONS, added to every domain's
// schema - see templateUniversalFields).
//
// Keyed by manage_helper type name ("template_<domain>"), not by domain
// alone, matching the existing template_sensor/template_binary_sensor
// naming and the random_sensor/random_binary_sensor precedent for two
// type names sharing one platform.
var templateSubtypes = map[string]templateSubtype{
	"template_alarm_control_panel": {
		domain: "alarm_control_panel",
		fields: []templateField{
			{arg: "state", haKey: "value_template", kind: tplTemplate, desc: "Alarm state template (template_alarm_control_panel)"},
			{arg: "disarm", kind: tplAction, desc: "Action to disarm (template_alarm_control_panel)"},
			{arg: "arm_away", kind: tplAction, desc: "Action to arm away (template_alarm_control_panel)"},
			{arg: "arm_custom_bypass", kind: tplAction, desc: "Action to arm with custom bypass (template_alarm_control_panel)"},
			{arg: "arm_home", kind: tplAction, desc: "Action to arm home (template_alarm_control_panel)"},
			{arg: "arm_night", kind: tplAction, desc: "Action to arm night (template_alarm_control_panel)"},
			{arg: "arm_vacation", kind: tplAction, desc: "Action to arm vacation (template_alarm_control_panel)"},
			{arg: "trigger", kind: tplAction, desc: "Action to trigger the alarm (template_alarm_control_panel)"},
			{arg: "code_arm_required", kind: tplBool, desc: "Whether a code is required to arm (template_alarm_control_panel, HA default true)"},
			{arg: "code_format", kind: tplString, desc: "Code format: number or text (template_alarm_control_panel, HA default number)"},
		},
	},
	"template_button": {
		domain: "button",
		fields: []templateField{
			{arg: "press", kind: tplAction, desc: "Action to run when pressed (template_button)"},
		},
	},
	"template_cover": {
		domain: "cover",
		fields: []templateField{
			{arg: "state", kind: tplTemplate, required: true, desc: "Cover state template (template_cover, required)"},
			{arg: "open", haKey: "open_cover", kind: tplAction, desc: "Action to open (template_cover; must be supplied together with close, or neither) or to open a lock, e.g. a door strike (template_lock)"},
			{arg: "close", haKey: "close_cover", kind: tplAction, desc: "Action to close (template_cover; must be supplied together with open, or neither)"},
			{arg: "stop", haKey: "stop_cover", kind: tplAction, desc: "Action to stop (template_cover) or to stop a vacuum (template_vacuum)"},
			{arg: "position", kind: tplTemplate, desc: "Current position template, 0-100 (template_cover)"},
			{arg: "set_position", haKey: "set_cover_position", kind: tplAction, desc: "Action to set position (template_cover)"},
		},
		inclusivePairs: [][2]string{{"open", "close"}},
	},
	"template_device_tracker": {
		domain: "device_tracker",
		fields: []templateField{
			{arg: "in_zones", kind: tplTemplate, desc: "Zones template (template_device_tracker)"},
			{arg: "latitude", kind: tplTemplate, desc: "Latitude template (template_device_tracker)"},
			{arg: "longitude", kind: tplTemplate, desc: "Longitude template (template_device_tracker)"},
			{arg: "location_accuracy", kind: tplTemplate, desc: "Location accuracy template, meters (template_device_tracker)"},
		},
	},
	"template_event": {
		domain: "event",
		fields: []templateField{
			{arg: "event_type", kind: tplTemplate, required: true, desc: "Event type template (template_event, required)"},
			{arg: "event_types", kind: tplTemplate, required: true, desc: "Possible event types template (template_event, required)"},
		},
	},
	"template_fan": {
		domain: "fan",
		fields: []templateField{
			{arg: "state", kind: tplTemplate, required: true, desc: "Fan state template (template_fan, required)"},
			{arg: "turn_on", kind: tplAction, required: true, desc: "Action to turn on (required for template_fan/template_light, optional for template_switch)"},
			{arg: "turn_off", kind: tplAction, required: true, desc: "Action to turn off (required for template_fan/template_light, optional for template_switch)"},
			{arg: "percentage", kind: tplTemplate, desc: "Speed percentage template (template_fan)"},
			{arg: "set_percentage", kind: tplAction, desc: "Action to set speed percentage (template_fan)"},
			{arg: "speed_count", kind: tplNumber, desc: "Number of discrete speeds, 1-100 (template_fan)"},
		},
	},
	"template_image": {
		domain: "image",
		fields: []templateField{
			{arg: "url", kind: tplTemplate, required: true, desc: "Image URL template (template_image, required)"},
			{arg: "verify_ssl", kind: tplBool, desc: "Verify SSL certificates (template_image, HA default true) - setting false disables TLS verification for HA's server-side fetch of the caller-controlled url template; only disable for a known, trusted, non-HTTPS/self-signed source"},
		},
	},
	"template_light": {
		domain: "light",
		fields: []templateField{
			{arg: "state", kind: tplTemplate, required: true, desc: "Light state template (template_light, required)"},
			{arg: "turn_on", kind: tplAction, required: true, desc: "Action to turn on (template_light, required)"},
			{arg: "turn_off", kind: tplAction, required: true, desc: "Action to turn off (template_light, required)"},
			{arg: "level", kind: tplTemplate, desc: "Brightness level template, 0-255 (template_light)"},
			{arg: "set_level", kind: tplAction, desc: "Action to set brightness level (template_light)"},
			{arg: "hs", kind: tplTemplate, desc: "Hue/saturation template (template_light)"},
			{arg: "set_hs", kind: tplAction, desc: "Action to set hue/saturation (template_light)"},
			{arg: "temperature", kind: tplTemplate, desc: "Color temperature template (template_light) or current temperature template (template_weather)"},
			{arg: "set_temperature", kind: tplAction, desc: "Action to set color temperature (template_light)"},
		},
	},
	"template_lock": {
		domain: "lock",
		fields: []templateField{
			{arg: "state", kind: tplTemplate, required: true, desc: "Lock state template (template_lock, required)"},
			{arg: "lock", kind: tplAction, required: true, desc: "Action to lock (template_lock, required)"},
			{arg: "unlock", kind: tplAction, required: true, desc: "Action to unlock (template_lock, required)"},
			{arg: "lock_code_format", haKey: "code_format", kind: tplTemplate, desc: "Code format template (template_lock)"},
			{arg: "open", kind: tplAction, desc: "Action to open, e.g. a door strike (template_lock)"},
		},
	},
	"template_number": {
		domain: "number",
		fields: []templateField{
			{arg: "state", kind: tplTemplate, required: true, desc: "Number state template (template_number, required)"},
			{arg: "min", kind: tplNumber, desc: "Minimum value (template_number, HA default 0)"},
			{arg: "max", kind: tplNumber, desc: "Maximum value (template_number, HA default 100)"},
			{arg: "step", kind: tplNumber, desc: "Step size (template_number, HA default 1)"},
			{arg: "unit_of_measurement", kind: tplString, desc: "Unit of measurement (template_number)"},
			{arg: "set_value", kind: tplAction, required: true, desc: "Action to set the value (template_number, required)"},
		},
	},
	"template_select": {
		domain: "select",
		fields: []templateField{
			{arg: "state", kind: tplTemplate, required: true, desc: "Select state template (template_select, required)"},
			{arg: "options_template", haKey: "options", kind: tplTemplate, required: true, desc: "Options list template (template_select, required) - not a fixed array, unlike input_select's options"},
			{arg: "select_option", kind: tplAction, desc: "Action to select an option (template_select)"},
		},
	},
	"template_switch": {
		domain: "switch",
		fields: []templateField{
			{arg: "state", haKey: "value_template", kind: tplTemplate, desc: "Switch state template (template_switch)"},
			{arg: "turn_on", kind: tplAction, desc: "Action to turn on (template_switch)"},
			{arg: "turn_off", kind: tplAction, desc: "Action to turn off (template_switch)"},
		},
	},
	"template_update": {
		domain: "update",
		fields: []templateField{
			{arg: "installed_version", kind: tplTemplate, desc: "Installed version template (template_update)"},
			{arg: "latest_version", kind: tplTemplate, desc: "Latest version template (template_update)"},
			{arg: "install", kind: tplAction, desc: "Action to install the update (template_update)"},
			{arg: "in_progress", kind: tplTemplate, desc: "Update-in-progress template (template_update)"},
			{arg: "release_summary", kind: tplTemplate, desc: "Release summary template (template_update)"},
			{arg: "release_url", kind: tplTemplate, desc: "Release URL template (template_update)"},
			{arg: "title", kind: tplTemplate, desc: "Title template (template_update)"},
			{arg: "update_percentage", kind: tplTemplate, desc: "Update progress percentage template (template_update)"},
			{arg: "backup", kind: tplBool, desc: "Whether the update supports backup (template_update)"},
			{arg: "specific_version", kind: tplBool, desc: "Whether a specific version can be installed (template_update)"},
		},
	},
	"template_vacuum": {
		domain: "vacuum",
		fields: []templateField{
			{arg: "state", kind: tplTemplate, required: true, desc: "Vacuum state template (template_vacuum, required)"},
			{arg: "start", kind: tplAction, required: true, desc: "Action to start cleaning (template_vacuum, required)"},
			{arg: "fan_speed", kind: tplTemplate, desc: "Current fan speed template (template_vacuum)"},
			{arg: "fan_speed_list", haKey: "fan_speeds", kind: tplStringArray, desc: "Available fan speeds (template_vacuum)"},
			{arg: "set_fan_speed", kind: tplAction, desc: "Action to set fan speed (template_vacuum)"},
			{arg: "stop", kind: tplAction, desc: "Action to stop (template_vacuum)"},
			{arg: "pause", kind: tplAction, desc: "Action to pause (template_vacuum)"},
			{arg: "return_to_base", kind: tplAction, desc: "Action to return to base (template_vacuum)"},
			{arg: "clean_spot", kind: tplAction, desc: "Action to clean a spot (template_vacuum)"},
			{arg: "locate", kind: tplAction, desc: "Action to locate the vacuum (template_vacuum)"},
		},
	},
	"template_weather": {
		domain: "weather",
		fields: []templateField{
			{arg: "condition", kind: tplTemplate, required: true, desc: "Weather condition template (template_weather, required)"},
			{arg: "humidity", kind: tplTemplate, required: true, desc: "Humidity template (template_weather, required)"},
			{arg: "temperature", kind: tplTemplate, required: true, desc: "Current temperature template (template_weather, required) - shares the same key as template_light's color temperature"},
			{arg: "temperature_unit", kind: tplString, desc: "Temperature unit (template_weather)"},
			{arg: "forecast_daily", kind: tplTemplate, desc: "Daily forecast template (template_weather)"},
			{arg: "forecast_hourly", kind: tplTemplate, desc: "Hourly forecast template (template_weather)"},
		},
	},
}

// templateUniversalFields are accepted by every template subtype (HA adds
// device_id and an "additional_options" section with availability to
// every domain's schema unconditionally). Not added to
// template_sensor/template_binary_sensor's existing, separately
// maintained field lists - out of scope for this addition.
var templateUniversalFields = []templateField{
	{arg: "availability", kind: tplTemplate, desc: "Availability template (any template_* helper)"},
	{arg: "device_id", kind: tplString, desc: "Device to attach this entity to (any template_* helper)"},
}

// perTypeDeviceClassSupport lists template subtypes whose CONFIG_FLOW
// schema declares device_class, per HA's generate_schema
// (homeassistant/components/template/config_flow.py). Read on create by
// buildTemplateHelperConfig regardless of whether the type also supports
// device_class on update (see perTypeDeviceClassUpdateSupport for that,
// separate, question) - button/cover/event/update declare device_class
// only inside generate_schema's `if flow_type == "config":` guard
// (config-flow only), while number declares it unconditionally (both
// flows) and is also in perTypeDeviceClassUpdateSupport.
var perTypeDeviceClassSupport = map[string]bool{
	"template_button": true,
	"template_cover":  true,
	"template_event":  true,
	"template_number": true,
	"template_update": true,
}

// perTypeDeviceClassUpdateSupport lists template subtypes whose
// OPTIONS_FLOW schema, not just CONFIG_FLOW, declares device_class -
// number is the only one: its generate_schema block has no `if
// flow_type == "config":` guard around device_class, unlike
// button/cover/event/update. Consulted by buildPerTypeUpdateExcludedFields
// (whether device_class is excluded from update at all) and
// buildConfigEntryUpdateConfig's device_class read gate (whether it's
// actually forwarded for a given entity domain).
var perTypeDeviceClassUpdateSupport = map[string]bool{
	"template_number": true,
}

// templateSubtypeDomains is the set of entity domains any of the 15
// template_* subtypes create (issue #206), derived from templateSubtypes so
// it can't drift. Used to gate reads in buildConfigEntryUpdateConfig that
// pre-date these subtypes and were designed only for the sensor/binary_sensor
// domains it was originally shared with.
var templateSubtypeDomains = buildTemplateSubtypeDomains()

func buildTemplateSubtypeDomains() map[string]bool {
	m := make(map[string]bool, len(templateSubtypes))
	for _, subtype := range templateSubtypes {
		m[subtype.domain] = true
	}
	return m
}

// deviceClassSupportedOnTemplateUpdate reports whether device_class should
// be forwarded on a Config Entry helper update for entityDomain. For a
// domain none of the 15 template subtypes own, it defers entirely (true -
// device_class handling for sensor/binary_sensor/etc. is unrelated to this
// package). For a domain a template subtype DOES own (including one also
// shared with switch_as_x, e.g. "light"/"cover" - see
// checkHelperOnlyDomain's doc comment in helpers_consolidated.go for why
// that sharing is safe), it defers to that subtype's own
// perTypeDeviceClassUpdateSupport entry via the domain<->type-name
// convention TestTemplateSubtypeTable_CoversHATemplateTypes pins
// ("template_" + domain).
func deviceClassSupportedOnTemplateUpdate(entityDomain string) bool {
	if !templateSubtypeDomains[entityDomain] {
		return true
	}
	return perTypeDeviceClassUpdateSupport["template_"+entityDomain]
}

// buildTemplateHelperConfig returns a configBuilderFunc for the given
// template_* type name, shared by every subtype: read each declared field
// with the reader its kind selects, apply device_class if this subtype's
// CONFIG_FLOW schema supports it, enforce any HA vol.Inclusive field
// pairing, then stamp template_type - the routing key
// determineTemplateSubtype (internal/homeassistant/hybrid_client.go) reads
// to pick the create-flow menu option and predict the entity domain.
func buildTemplateHelperConfig(typeName string) configBuilderFunc {
	return func(config, args map[string]any) error {
		subtype := templateSubtypes[typeName]
		r := newArgReader(config, args)
		for _, f := range subtype.fields {
			readTemplateField(r, f)
		}
		for _, f := range templateUniversalFields {
			readTemplateField(r, f)
		}
		if perTypeDeviceClassSupport[typeName] {
			r.str("device_class")
		}
		if err := r.err(); err != nil {
			return err
		}
		if err := checkInclusivePairs(subtype, config); err != nil {
			return err
		}
		config["template_type"] = subtype.domain
		return nil
	}
}

// checkInclusivePairs enforces subtype's vol.Inclusive field pairs against
// config (post-argReader, keyed by each field's real HA configKey), not
// against the caller-facing arg names the pairs are declared in:
// readTemplateField renames each field to its configKey (e.g. "open" ->
// "open_cover") before this runs, so checking config[pair[0]]/config[pair[1]]
// directly - the caller-facing arg names - as an earlier version did, always
// misses. subtypeConfigKey resolves each pair element's arg name to the
// configKey actually present in config.
func checkInclusivePairs(subtype templateSubtype, config map[string]any) error {
	for _, pair := range subtype.inclusivePairs {
		_, hasA := config[subtypeConfigKey(subtype, pair[0])]
		_, hasB := config[subtypeConfigKey(subtype, pair[1])]
		if hasA != hasB {
			return fmt.Errorf("%s and %s must be supplied together or not at all", pair[0], pair[1])
		}
	}
	return nil
}

// subtypeConfigKey resolves arg to the configKey subtype's own field
// declaration uses, falling back to arg itself if subtype declares no field
// by that name (shouldn't happen for a well-formed inclusivePairs entry -
// see TestTemplateSubtypeTable_InclusivePairsReferenceDeclaredFields).
func subtypeConfigKey(subtype templateSubtype, arg string) string {
	for _, f := range subtype.fields {
		if f.arg == arg {
			return f.configKey()
		}
	}
	return arg
}

func readTemplateField(r *argReader, f templateField) {
	switch f.kind {
	case tplAction:
		r.actionValue(f.arg)
	case tplNumber:
		r.num(f.arg)
	case tplBool:
		r.boolean(f.arg)
	case tplStringArray:
		r.strSlice(f.arg)
	default: // tplTemplate, tplString - both plain strings on the wire
		r.str(f.arg)
	}
	// Every reader above writes to r.config[f.arg]. Rename to the real HA
	// key here, uniformly across every kind - argReader only has an "As"
	// rename variant for str()/strID(), not for num()/boolean()/strSlice()/
	// actionValue(), and a per-kind rename call is easy to forget (as the
	// first version of this function did for exactly that reason).
	if key := f.configKey(); key != f.arg {
		if v, ok := r.config[f.arg]; ok {
			delete(r.config, f.arg)
			r.config[key] = v
		}
	}
}

// addTemplateConfigEntryUpdateFields reads every template subtype field
// (deduplicated across subtypes by arg name) into an update config, called
// from addExtendedConfigEntryFields alongside every other config-entry
// type's update fields - the same one-size-fits-all shape those already
// use. Safe to over-read: buildStepSubmission
// (internal/homeassistant/flow_steps.go) only forwards a field to a step
// whose schema actually declares it, so a switch update never sees a
// stray "temperature" just because this function also knows how to read it.
func addTemplateConfigEntryUpdateFields(r *argReader, entityDomain string) {
	for _, f := range resolveTemplateFieldsForDomain(entityDomain) {
		readTemplateField(r, f)
	}
	for _, f := range templateUniversalFields {
		readTemplateField(r, f)
	}
	// device_class is deliberately not read here - see buildConfigEntryUpdateConfig's
	// device_class comment for why no template subtype domain accepts it on update.
}

// templateFieldsForDomainCache memoizes computeTemplateFieldsForDomain by
// entityDomain: templateSubtypes is immutable after package init, so the
// same domain always produces the same result - see
// computeTemplateFieldsForDomain's doc comment for why memoizing matters.
// Guarded by a mutex rather than sync.Map since the value (a slice) needs
// a plain read-if-present-else-compute-and-store sequence, and contention
// here is negligible (one lock per distinct domain, ever).
var (
	templateFieldsForDomainCache   = map[string][]templateField{}
	templateFieldsForDomainCacheMu sync.Mutex
)

// resolveTemplateFieldsForDomain returns one templateField per distinct
// arg name declared by any template subtype, resolving the small number of
// names two subtypes declare with DIFFERENT HA keys - "state" (bare for
// most subtypes, "value_template" for template_alarm_control_panel/
// template_switch), "open" (bare for template_lock, "open_cover" for
// template_cover), "stop" (bare for template_vacuum, "stop_cover" for
// template_cover) - to whichever subtype's domain matches entityDomain,
// the entity actually being updated. Without this, addExtendedConfigEntryFields'
// single shared update builder (which has no per-subtype call site, unlike
// create's buildTemplateHelperConfig(typeName)) would pick a haKey at
// random per Go's map iteration order and intermittently write the wrong
// config key.
//
// A non-ambiguous name (declared identically by every subtype that has it,
// or by only one) resolves the same way regardless of entityDomain - the
// sorted type-name iteration just makes that fallback deterministic.
func resolveTemplateFieldsForDomain(entityDomain string) []templateField {
	templateFieldsForDomainCacheMu.Lock()
	defer templateFieldsForDomainCacheMu.Unlock()
	if cached, ok := templateFieldsForDomainCache[entityDomain]; ok {
		return cached
	}
	result := computeTemplateFieldsForDomain(entityDomain)
	templateFieldsForDomainCache[entityDomain] = result
	return result
}

// computeTemplateFieldsForDomain does the actual work resolveTemplateFieldsForDomain
// memoizes: templateSubtypes is immutable after package init, and this
// function was previously being rebuilt (three maps, two sorts over ~90
// fields) on every single call - once per arg inside splitAppliedFields'
// loop via resolveUpdateConfigKey, plus once more from
// addTemplateConfigEntryUpdateFields, for every manage_helper update on a
// template subtype entity.
func computeTemplateFieldsForDomain(entityDomain string) []templateField {
	typeNames := make([]string, 0, len(templateSubtypes))
	for typeName := range templateSubtypes {
		typeNames = append(typeNames, typeName)
	}
	slices.Sort(typeNames)

	configKeysByArg := make(map[string]map[string]bool, 32)
	firstByArg := make(map[string]templateField, 32)
	matchedByArg := make(map[string]templateField, 32)
	for _, typeName := range typeNames {
		subtype := templateSubtypes[typeName]
		for _, f := range subtype.fields {
			if configKeysByArg[f.arg] == nil {
				configKeysByArg[f.arg] = make(map[string]bool)
			}
			configKeysByArg[f.arg][f.configKey()] = true
			if _, seen := firstByArg[f.arg]; !seen {
				firstByArg[f.arg] = f
			}
			if subtype.domain == entityDomain {
				matchedByArg[f.arg] = f
			}
		}
	}

	// matchedConfigKeys collects the HA config keys the domain's OWN matched
	// fields resolve to, so a same-key fallback field (see the loop below)
	// can be dropped instead of colliding with it.
	matchedConfigKeys := make(map[string]bool, len(matchedByArg))
	for _, f := range matchedByArg {
		matchedConfigKeys[f.configKey()] = true
	}

	args := make([]string, 0, len(firstByArg))
	for arg := range firstByArg {
		args = append(args, arg)
	}
	slices.Sort(args)

	result := make([]templateField, 0, len(firstByArg))
	for _, arg := range args {
		if matched, ok := matchedByArg[arg]; ok {
			result = append(result, matched)
			continue
		}
		// Ambiguous (two subtypes declare this arg with different HA keys,
		// e.g. "state"/"open"/"stop") and no subtype's domain matches the
		// entity actually being updated: skip rather than guess. This is
		// what stops, e.g., a template_sensor update (domain "sensor",
		// which matches none of these 15 subtypes) from picking an
		// arbitrary subtype's "state" definition and renaming a config key
		// the pre-existing top-level "state" field (buildConfigEntryUpdateConfig)
		// already set correctly.
		if len(configKeysByArg[arg]) > 1 {
			continue
		}
		// Unambiguous (single owner) but its config key COLLIDES with a key
		// one of the domain's own matched fields already writes - e.g.
		// template_alarm_control_panel's bare "code_format" is single-owner,
		// but for domain "lock" it would collide with template_lock's own
		// "lock_code_format" (haKey "code_format"). Without this check, W5
		// of the issue #206 review: a caller updating a lock via the plain
		// "code_format" arg name would silently write straight through
		// instead of being reported as unresolved/ignored, defeating the
		// rename template_lock exists to disambiguate.
		field := firstByArg[arg]
		if matchedConfigKeys[field.configKey()] {
			continue
		}
		result = append(result, field)
	}
	return result
}

// resolveUpdateConfigKey resolves the HA config key a caller-facing
// manage_helper update arg name is renamed to for entityDomain, so
// splitAppliedFields/renderUpdateResultMessage report which fields actually
// reached Home Assistant using the same rename table the update builder
// itself used. Template subtype renames are domain-dependent - "open"
// renames to "open_cover" only for template_cover but stays bare for
// template_lock, "state" renames to "value_template" only for
// template_alarm_control_panel/template_switch - so a single static map
// (updateConfigKeyAliases) cannot express them; resolveTemplateFieldsForDomain
// already resolves that ambiguity per-domain, so it is consulted first.
// Falls back to the static updateConfigKeyAliases map, then to argName
// itself, for every rename that isn't a template subtype field.
func resolveUpdateConfigKey(entityDomain, argName string) string {
	for _, f := range resolveTemplateFieldsForDomain(entityDomain) {
		if f.arg == argName {
			return f.configKey()
		}
	}
	if alias, ok := updateConfigKeyAliases[argName]; ok {
		return alias
	}
	return argName
}

// templateHelperTypes derives helperTypes metadata for every template
// subtype from templateSubtypes, so the field list can't drift between
// the tool schema, the builders, and what create/update validate against.
func templateHelperTypes() map[string]helperTypeMetadata {
	result := make(map[string]helperTypeMetadata, len(templateSubtypes))
	for typeName, subtype := range templateSubtypes {
		var required, optional []string
		for _, f := range subtype.fields {
			if f.required {
				required = append(required, f.arg)
			} else {
				optional = append(optional, f.arg)
			}
		}
		optional = append(optional, "icon", "availability", "device_id")
		if perTypeDeviceClassSupport[typeName] {
			optional = append(optional, "device_class")
		}
		result[typeName] = helperTypeMetadata{
			platform:           platformTemplate,
			entityPrefix:       subtype.domain,
			supportedActions:   []string{},
			requiredFields:     required,
			optionalFields:     optional,
			validEntityDomains: []string{subtype.domain},
		}
	}
	return result
}

// templateSubtypeNames returns the type-name keys of templateSubtypes.
func templateSubtypeNames() []string {
	names := make([]string, 0, len(templateSubtypes))
	for typeName := range templateSubtypes {
		names = append(names, typeName)
	}
	slices.Sort(names)
	return names
}

// templateHelperProperties derives manage_helper JSON schema properties for
// every field introduced by a template subtype, deduplicated by arg name
// (a name shared by two subtypes, e.g. "state" or "turn_on", gets exactly
// one property definition). Fields already declared elsewhere in the base
// schema (e.g. "state", "min", "device_class") are skipped by the caller's
// merge, not here - see manageHelperTool()'s merge loop.
func templateHelperProperties() map[string]mcp.JSONSchema {
	props := make(map[string]mcp.JSONSchema)
	addField := func(f templateField) {
		if _, exists := props[f.arg]; exists {
			return
		}
		desc := f.desc
		prop := mcp.JSONSchema{}
		switch f.kind {
		case tplNumber:
			prop.Type = "number"
		case tplBool:
			prop.Type = "boolean"
		case tplStringArray:
			prop.Type = "array"
			prop.Items = &mcp.JSONSchema{Type: "string"}
		case tplTemplate, tplString:
			prop.Type = "string"
		case tplAction:
			// Type intentionally omitted: an HA action value is a JSON
			// object or array of objects, not a scalar - spelled out in the
			// description instead, since mcp.JSONSchema has no oneOf/anyOf
			// to express that shape structurally.
			desc += " - a single action object (e.g. {\"action\": \"switch.turn_on\", \"target\": {...}}) or a list of action objects, the same shape as an automation's action: block"
		}
		prop.Description = desc
		props[f.arg] = prop
	}
	// Sorted type-name iteration: templateSubtypes is a map, and a shared
	// arg name (e.g. "turn_on", "open", "temperature") is declared by more
	// than one subtype with a different desc - without a deterministic
	// order, addField's "first one wins" pick (and thus the emitted
	// manage_helper schema text) would vary between process starts.
	for _, typeName := range templateSubtypeNames() {
		for _, f := range templateSubtypes[typeName].fields {
			addField(f)
		}
	}
	for _, f := range templateUniversalFields {
		addField(f)
	}
	return props
}

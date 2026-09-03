package homeassistant

import "slices"

// flowMode distinguishes the create-path and update-path submission
// semantics for buildStepSubmission. Home Assistant's
// SchemaCommonFlowHandler._update_and_remove_omitted_optional_keys pops
// every vol.Optional key of a step's own schema that is absent from the
// submitted payload. On create nothing is stored yet, so an empty
// submission is a genuine no-op. On update those keys are the config
// entry's *existing* stored values, so an empty submission for an
// omitted optional field would delete it. The two modes exist so a
// single algorithm expresses both without silently losing data on
// update.
type flowMode int

const (
	flowModeCreate flowMode = iota
	flowModeUpdate
)

// stepSchemaIndex resolves one flow step's DataSchema into the two
// placements a submitted key can have: top-level, or nested one level
// inside a section (HA's "expandable" sections nest exactly one level -
// allow_section=False is enforced HA-side, so a second level never occurs
// in practice).
type stepSchemaIndex struct {
	top      map[string]OptionsFlowField
	sections map[string]sectionIndex
	empty    bool
}

type sectionIndex struct {
	field    OptionsFlowField
	children map[string]OptionsFlowField
}

// indexStepSchema builds a lookup index from a flow step's raw DataSchema.
// An empty/absent schema (nil or zero-length) is recorded via the empty
// flag so buildStepSubmission can degrade to forwarding every field
// un-nested, rather than silently discarding everything - a step HA
// describes with no schema at all is not something the real API produces
// (a schema-less step is auto-advanced before it ever reaches us), but
// this keeps mocks that return a bare {Type:"form"} meaningful.
func indexStepSchema(schema []OptionsFlowField) stepSchemaIndex {
	ix := stepSchemaIndex{
		top:      make(map[string]OptionsFlowField),
		sections: make(map[string]sectionIndex),
	}
	if len(schema) == 0 {
		ix.empty = true
		return ix
	}
	for _, field := range schema {
		if field.Type == "expandable" {
			children := make(map[string]OptionsFlowField, len(field.Schema))
			for _, child := range field.Schema {
				children[child.Name] = child
			}
			ix.sections[field.Name] = sectionIndex{field: field, children: children}
			continue
		}
		ix.top[field.Name] = field
	}
	return ix
}

// placementOf reports where a submitted key belongs in this step: an
// empty section name means top-level, a non-empty one names the
// containing section.
func (ix stepSchemaIndex) placementOf(key string) (section string, field OptionsFlowField, ok bool) {
	if f, found := ix.top[key]; found {
		return "", f, true
	}
	for sectionName, si := range ix.sections {
		if f, found := si.children[key]; found {
			return sectionName, f, true
		}
	}
	return "", OptionsFlowField{}, false
}

// fieldIsReadOnly reports whether HA marked this field's selector
// read-only (e.g. statistics' entity_id/state_characteristic in its
// Options Flow schema). A read-only field's stored value must never be
// treated as caller-changeable.
func fieldIsReadOnly(f OptionsFlowField) bool {
	for _, cfg := range f.Selector {
		if m, ok := cfg.(map[string]any); ok {
			if ro, ok := m["read_only"].(bool); ok && ro {
				return true
			}
		}
	}
	return false
}

// fieldIsDuration reports whether a value for this field should be
// coerced into HA's {hours,minutes,seconds} duration dict shape. The
// field's own selector is authoritative when present. When the schema
// gives no selector information (degraded/mocked steps), it falls back
// to the pre-existing name-keyed table and the window_size/filter-step
// special case.
func fieldIsDuration(f OptionsFlowField, key, stepID string) bool {
	if f.Selector != nil {
		_, isDuration := f.Selector["duration"]
		return isDuration
	}
	return isDurationField(key) || (key == "window_size" && filterDurationWindowSteps[stepID])
}

// coerceForField converts v into the shape this field's selector expects.
// Currently only duration coercion is schema-aware; every other value
// passes through unchanged. Falls back to the raw value if coercion
// fails, so HA's own validation reports the error rather than this
// function silently swallowing a bad input.
func coerceForField(f OptionsFlowField, v any, key, stepID string) any {
	if fieldIsDuration(f, key, stepID) {
		if d, ok := toDurationDict(v); ok {
			return d
		}
	}
	return v
}

// buildStepSubmission is the schema-driven replacement for the
// hardcoded per-platform step builders (buildFilterStepConfig,
// buildGenericThermostatStepConfig, and the statistics/trend branches of
// the former buildConfigForFlowStep). A step's own DataSchema is the
// allow-list: a user-supplied field is routed into this step's payload
// only if the step's schema declares it, and is marked consumed so a
// later step (or the final unconsumed-field check) does not see it
// again.
//
// The create/update asymmetry lives entirely in the starting value of
// base: empty on create (nothing stored yet, so an omitted optional is a
// genuine omission), or the round-tripped suggested_value of every field
// this step declares on update (so HA's
// _update_and_remove_omitted_optional_keys finds every optional field
// present and pops nothing). Sections get the same treatment one level
// down, and are always re-emitted on update even when the caller
// supplies nothing for them - the section key is itself vol.Optional, so
// omitting it would delete the whole nested dict.
func buildStepSubmission(mode flowMode, ix stepSchemaIndex, userConfig map[string]any, consumed map[string]bool, stepID string) map[string]any {
	if ix.empty {
		return forwardEverythingUnnested(userConfig, consumed, stepID)
	}

	base := make(map[string]any)
	if mode == flowModeUpdate {
		for name, f := range ix.top {
			assignSuggestedValue(base, name, f)
		}
		for sectionName, si := range ix.sections {
			sectionValues := make(map[string]any)
			for name, f := range si.children {
				assignSuggestedValue(sectionValues, name, f)
			}
			base[sectionName] = sectionValues
		}
	}

	for key, val := range userConfig {
		if consumed[key] {
			continue
		}
		section, field, ok := ix.placementOf(key)
		if !ok {
			continue
		}
		if mode == flowModeUpdate && fieldIsReadOnly(field) {
			continue
		}
		coerced := coerceForField(field, val, key, stepID)
		if section == "" {
			base[key] = coerced
		} else {
			sectionValues, _ := base[section].(map[string]any)
			if sectionValues == nil {
				sectionValues = make(map[string]any)
				base[section] = sectionValues
			}
			sectionValues[key] = coerced
		}
		consumed[key] = true
	}

	return base
}

func assignSuggestedValue(dst map[string]any, name string, f OptionsFlowField) {
	if f.Description == nil {
		return
	}
	if v, ok := f.Description["suggested_value"]; ok && v != nil {
		dst[name] = v
	}
}

// forwardEverythingUnnested is buildStepSubmission's degradation path for
// a step whose schema could not be inspected (empty/absent DataSchema).
// It mirrors the pre-engine default behavior: forward every not-yet-consumed
// field flat, with name-based duration coercion, and mark all of them
// consumed so the deferred unconsumed-field check doesn't then fail on a
// step it had no way to filter.
func forwardEverythingUnnested(userConfig map[string]any, consumed map[string]bool, stepID string) map[string]any {
	result := make(map[string]any, len(userConfig))
	for key, val := range userConfig {
		if consumed[key] {
			continue
		}
		if isDurationField(key) || (key == "window_size" && filterDurationWindowSteps[stepID]) {
			if d, ok := toDurationDict(val); ok {
				result[key] = d
				consumed[key] = true
				continue
			}
		}
		result[key] = val
		consumed[key] = true
	}
	return result
}

// seedConsumedRoutingKeys pre-marks keys that are never a submission
// value in their own right - either global routing markers or
// platform-specific ones already renamed/consumed by the config
// builders (e.g. heater_entity_id -> heater). Without this, a routing
// key that legitimately never appears in any step's schema would be
// misreported as an unsupported field.
func seedConsumedRoutingKeys(config HelperConfig) map[string]bool {
	consumed := map[string]bool{"group_type": true}
	if skipFields, ok := platformSkipFields[config.Platform]; ok {
		for key := range skipFields {
			consumed[key] = true
		}
	}
	if RequiresConfigEntryFlow(config.Platform) {
		consumed["icon"] = true
	}
	return consumed
}

// unconsumedUserFields lists the caller-supplied keys no step's schema
// ever claimed, sorted for a deterministic error message.
func unconsumedUserFields(userConfig map[string]any, consumed map[string]bool) []string {
	var fields []string
	for key := range userConfig {
		if !consumed[key] {
			fields = append(fields, key)
		}
	}
	slices.Sort(fields)
	return fields
}

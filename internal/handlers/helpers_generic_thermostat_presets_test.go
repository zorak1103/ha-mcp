package handlers

import (
	"reflect"
	"testing"
)

func TestBuildGenericThermostatConfig_ReadsPresetTemperatures(t *testing.T) {
	t.Parallel()

	presets := map[string]float64{
		"away_temp":     16.0,
		"eco_temp":      17.0,
		"home_temp":     20.0,
		"comfort_temp":  21.0,
		"sleep_temp":    18.0,
		"activity_temp": 19.0,
	}

	args := map[string]any{
		"heater_entity_id":        "switch.heater",
		"target_sensor_entity_id": "sensor.temp",
	}
	for k, v := range presets {
		args[k] = v
	}

	config := map[string]any{}
	if err := buildGenericThermostatConfig(config, args); err != nil {
		t.Fatalf("buildGenericThermostatConfig() error = %v", err)
	}

	for k, want := range presets {
		got, ok := config[k].(float64)
		if !ok || got != want {
			t.Errorf("config[%q] = %v, want %v", k, config[k], want)
		}
	}
}

func TestAddExtendedConfigEntryFields_ReadsPresetTemperaturesOnUpdate(t *testing.T) {
	t.Parallel()

	args := map[string]any{"away_temp": 15.0}
	config := map[string]any{}

	if err := addExtendedConfigEntryFields(config, args, configEntryUpdateContext{entityDomain: "climate"}); err != nil {
		t.Fatalf("addExtendedConfigEntryFields() error = %v", err)
	}

	got, ok := config["away_temp"].(float64)
	if !ok || got != 15.0 {
		t.Errorf("config[away_temp] = %v, want 15.0", config["away_temp"])
	}
}

// TestAddExtendedConfigEntryFields_PresetsGatedToThermostatDomain guards
// the W3 fix: addExtendedConfigEntryFields is the one-size-fits-all update
// builder shared by every config-entry helper type (template, threshold,
// sensor-domain group, statistics, ...), none of which declare away_temp
// in their own Options Flow schema. Reading it unconditionally would let a
// caller updating an unrelated helper type silently populate a field HA's
// own schema for that type never asked for, mirroring the min_max_type
// leak this same builder already guards against via addMinMaxTypeField.
func TestAddExtendedConfigEntryFields_PresetsGatedToThermostatDomain(t *testing.T) {
	t.Parallel()

	args := map[string]any{"away_temp": 15.0}
	config := map[string]any{}

	if err := addExtendedConfigEntryFields(config, args, configEntryUpdateContext{entityDomain: "sensor"}); err != nil {
		t.Fatalf("addExtendedConfigEntryFields() error = %v", err)
	}

	if _, present := config["away_temp"]; present {
		t.Errorf("config[away_temp] = %v, want field NOT read for a non-climate entity domain", config["away_temp"])
	}
}

// TestGenericThermostatPresetSchemaProperties_MatchSharedFieldList is N1's
// regression test: the six preset schema properties used to be six
// hand-written, byte-identical map entries with no binding to
// genericThermostatPresetFieldNames() (the list the create/update builders
// actually read) - a 7th preset field added to the builders' side would
// have passed every existing contract test while staying invisible to
// clients, since nothing checked the schema and the field list agreed.
func TestGenericThermostatPresetSchemaProperties_MatchSharedFieldList(t *testing.T) {
	t.Parallel()

	schemaProps := genericThermostatPresetSchemaProperties()
	fieldNames := genericThermostatPresetFieldNames()

	if len(schemaProps) != len(fieldNames) {
		t.Fatalf("genericThermostatPresetSchemaProperties() has %d entries, genericThermostatPresetFieldNames() has %d - want equal",
			len(schemaProps), len(fieldNames))
	}
	for _, name := range fieldNames {
		prop, ok := schemaProps[name]
		if !ok {
			t.Errorf("genericThermostatPresetFieldNames() lists %q, but genericThermostatPresetSchemaProperties() has no matching schema property", name)
			continue
		}
		if prop.Type != "number" {
			t.Errorf("schema property %q has Type=%q, want \"number\"", name, prop.Type)
		}
	}

	// buildExtendedHelperProperties must actually merge in the generated
	// properties, not a separately hand-maintained copy - assert the full
	// manage_helper schema contains every generated preset property
	// unchanged.
	merged := buildExtendedHelperProperties()
	for name, prop := range schemaProps {
		got, ok := merged[name]
		if !ok {
			t.Errorf("buildExtendedHelperProperties() is missing preset property %q", name)
			continue
		}
		if !reflect.DeepEqual(got, prop) {
			t.Errorf("buildExtendedHelperProperties()[%q] = %+v, want it to equal the generated property %+v", name, got, prop)
		}
	}
}

// TestGenericThermostatOptionalFields_HasNoSpareCapacity is N2's regression
// test. helperTypes["generic_thermostat"].optionalFields is built via
// append([]string{...}, genericThermostatPresetFieldNames()...) - appending
// onto a full-literal slice reallocates with Go's growth headroom, so
// without slices.Clip this slice alone (of every helperTypes entry) would
// carry spare capacity a future in-place append elsewhere could silently
// share and corrupt.
func TestGenericThermostatOptionalFields_HasNoSpareCapacity(t *testing.T) {
	t.Parallel()

	fields := helperTypes["generic_thermostat"].optionalFields
	if len(fields) != cap(fields) {
		t.Errorf("generic_thermostat optionalFields len=%d cap=%d, want cap==len (use slices.Clip)", len(fields), cap(fields))
	}
}

// TestThermostatEntityDomain_IsUniqueAcrossHelperTypes is N3's regression
// test. addGenericThermostatPresetFields gates on the entity DOMAIN
// ("climate") rather than resolving the real integration PLATFORM the way
// the min_max_type gate does - correct only as long as "climate" is used by
// exactly one helperTypes entry. If a second config-entry helper type is
// ever added under the "climate" domain, this test fails instead of the
// preset fields silently leaking into that new type's updates.
func TestThermostatEntityDomain_IsUniqueAcrossHelperTypes(t *testing.T) {
	t.Parallel()

	var owners []string
	for name, meta := range helperTypes {
		for _, domain := range meta.validEntityDomains {
			if domain == thermostatEntityDomain {
				owners = append(owners, name)
			}
		}
	}
	if len(owners) != 1 {
		t.Errorf("helperTypes entries claiming entity domain %q: %v, want exactly [\"generic_thermostat\"] - "+
			"addGenericThermostatPresetFields' domain-only gate assumes single ownership", thermostatEntityDomain, owners)
	}
}

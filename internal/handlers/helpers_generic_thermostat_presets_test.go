package handlers

import "testing"

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

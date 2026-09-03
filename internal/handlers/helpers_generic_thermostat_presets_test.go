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

	if err := addExtendedConfigEntryFields(config, args, configEntryUpdateContext{}); err != nil {
		t.Fatalf("addExtendedConfigEntryFields() error = %v", err)
	}

	got, ok := config["away_temp"].(float64)
	if !ok || got != 15.0 {
		t.Errorf("config[away_temp] = %v, want 15.0", config["away_temp"])
	}
}

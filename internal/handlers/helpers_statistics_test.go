package handlers

import "testing"

func TestBuildStatisticsConfig_DefaultsStateCharacteristicToMean(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	args := map[string]any{"entity_id": "sensor.x"}

	if err := buildStatisticsConfig(config, args); err != nil {
		t.Fatalf("buildStatisticsConfig() error = %v", err)
	}

	got, _ := config["state_characteristic"].(string)
	if got != "mean" {
		t.Errorf("config[state_characteristic] = %q, want %q when omitted", got, "mean")
	}
}

func TestBuildStatisticsConfig_RespectsExplicitStateCharacteristic(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	args := map[string]any{"entity_id": "sensor.x", "state_characteristic": "median"}

	if err := buildStatisticsConfig(config, args); err != nil {
		t.Fatalf("buildStatisticsConfig() error = %v", err)
	}

	got, _ := config["state_characteristic"].(string)
	if got != "median" {
		t.Errorf("config[state_characteristic] = %q, want %q", got, "median")
	}
}

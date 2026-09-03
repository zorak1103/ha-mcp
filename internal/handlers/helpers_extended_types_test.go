// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestManageHelper_SchemaIncludesExtendedTypes verifies all 41 helper types are included in the schema.
func TestManageHelper_SchemaIncludesExtendedTypes(t *testing.T) {
	handlers := NewConsolidatedHelperHandlers()
	tool := handlers.manageHelperTool()

	typeEnum := tool.InputSchema.Properties["type"].Enum
	require.Len(t, typeEnum, 41, "Expected 41 helper types in schema (26 previous + 15 template subtypes, issue #206)")

	// Verify new types are present
	newTypes := []string{
		"utility_meter", "min_max", "statistics", "trend",
		"random_sensor", "random_binary_sensor", "filter", "tod",
		"generic_thermostat", "switch_as_x", "generic_hygrostat",
	}

	enumSet := make(map[string]bool)
	for _, e := range typeEnum {
		enumSet[e] = true
	}

	for _, newType := range newTypes {
		assert.True(t, enumSet[newType], "Expected type %s in schema enum", newType)
	}
}

// TestManageHelper_UtilityMeter tests utility_meter helper creation.
func TestManageHelper_UtilityMeter(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create utility_meter success",
			args: map[string]any{
				"action": "create",
				"type":   "utility_meter",
				"id":     "test_meter",
				"name":   "Test Utility Meter",
				"source": "sensor.energy_consumption",
				"cycle":  "daily",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "sensor.test_utility_meter"},
		},
		{
			name: "create utility_meter missing source",
			args: map[string]any{
				"action": "create",
				"type":   "utility_meter",
				"id":     "test_meter",
				"name":   "Test Utility Meter",
			},
			wantError:    true,
			wantContains: []string{"source", "required"},
		},
		{
			name: "create utility_meter with tariffs",
			args: map[string]any{
				"action":  "create",
				"type":    "utility_meter",
				"id":      "test_meter",
				"name":    "Test Utility Meter",
				"source":  "sensor.energy_consumption",
				"tariffs": []any{"peak", "offpeak"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "sensor.test_utility_meter"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_MinMax tests min_max helper creation.
func TestManageHelper_MinMax(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create min_max success",
			args: map[string]any{
				"action":       "create",
				"type":         "min_max",
				"id":           "test_minmax",
				"name":         "Test Min Max",
				"entity_ids":   []any{"sensor.temp1", "sensor.temp2"},
				"min_max_type": "mean",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "sensor.test_min_max"},
		},
		{
			name: "create min_max missing entity_ids",
			args: map[string]any{
				"action":       "create",
				"type":         "min_max",
				"id":           "test_minmax",
				"name":         "Test Min Max",
				"min_max_type": "mean",
			},
			wantError:    true,
			wantContains: []string{"entity_ids", "required"},
		},
		{
			name: "create min_max missing min_max_type",
			args: map[string]any{
				"action":     "create",
				"type":       "min_max",
				"id":         "test_minmax",
				"name":       "Test Min Max",
				"entity_ids": []any{"sensor.temp1", "sensor.temp2"},
			},
			wantError:    true,
			wantContains: []string{"min_max_type", "required"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_MinMax_CreateMapsMinMaxTypeToConfigType is a regression
// test for a bug where handleCreate read args["type"] to select the helper
// type ("min_max"), and buildMinMaxConfig then read the same args["type"]
// key to populate the calculation-type field HA expects - so the config
// sent to Home Assistant always carried type: "min_max" instead of the
// caller's intended calculation (min/max/mean/...). The tool argument is
// now the distinct name "min_max_type", mapped to HA's "type" config key.
func TestManageHelper_MinMax_CreateMapsMinMaxTypeToConfigType(t *testing.T) {
	t.Parallel()

	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.CreateHelperFn = func(_ context.Context, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":       "create",
		"type":         "min_max",
		"id":           "test_minmax",
		"name":         "Test Min Max",
		"entity_ids":   []any{"sensor.temp1", "sensor.temp2"},
		"min_max_type": "mean",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if got := capturedConfig.Config["type"]; got != "mean" {
		t.Errorf(`capturedConfig.Config["type"] = %v, want "mean" (the helper-type selector "min_max" must not leak into the calculation-type field)`, got)
	}
}

// TestManageHelper_Statistics tests statistics helper creation.
func TestManageHelper_Statistics(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create statistics success",
			args: map[string]any{
				"action":        "create",
				"type":          "statistics",
				"id":            "test_stats",
				"name":          "Test Statistics",
				"entity_id":     "sensor.temperature",
				"sampling_size": float64(20), // Either sampling_size or max_age is required
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "sensor.test_statistics"},
		},
		{
			name: "create statistics missing entity_id",
			args: map[string]any{
				"action":        "create",
				"type":          "statistics",
				"id":            "test_stats",
				"name":          "Test Statistics",
				"sampling_size": float64(20),
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
		{
			name: "create statistics with sampling_size",
			args: map[string]any{
				"action":        "create",
				"type":          "statistics",
				"id":            "test_stats",
				"name":          "Test Statistics",
				"entity_id":     "sensor.temperature",
				"sampling_size": float64(10),
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "sensor.test_statistics"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_Trend tests trend helper creation.
func TestManageHelper_Trend(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create trend success",
			args: map[string]any{
				"action":    "create",
				"type":      "trend",
				"id":        "test_trend",
				"name":      "Test Trend",
				"entity_id": "sensor.temperature",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "binary_sensor.test_trend"},
		},
		{
			name: "create trend missing entity_id",
			args: map[string]any{
				"action": "create",
				"type":   "trend",
				"id":     "test_trend",
				"name":   "Test Trend",
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_Random tests random helper creation.
func TestManageHelper_Random(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create random_sensor success",
			args: map[string]any{
				"action": "create",
				"type":   "random_sensor",
				"id":     "test_random",
				"name":   "Test Random Sensor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "sensor.test_random"},
		},
		{
			name: "create random_binary_sensor success",
			args: map[string]any{
				"action": "create",
				"type":   "random_binary_sensor",
				"id":     "test_random_bin",
				"name":   "Test Random Binary",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "binary_sensor.test_random_bin"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_Filter tests filter helper creation.
func TestManageHelper_Filter(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create filter success",
			args: map[string]any{
				"action":    "create",
				"type":      "filter",
				"id":        "test_filter",
				"name":      "Test Filter",
				"entity_id": "sensor.noisy_data",
				"filter":    "outlier",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "sensor.test_filter"},
		},
		{
			name: "create filter missing entity_id",
			args: map[string]any{
				"action": "create",
				"type":   "filter",
				"id":     "test_filter",
				"name":   "Test Filter",
				"filter": "outlier",
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
		{
			name: "create filter missing filter",
			args: map[string]any{
				"action":    "create",
				"type":      "filter",
				"id":        "test_filter",
				"name":      "Test Filter",
				"entity_id": "sensor.noisy_data",
			},
			wantError:    true,
			wantContains: []string{"filter", "required"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_Tod tests tod (Time of Day) helper creation.
func TestManageHelper_Tod(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create tod success",
			args: map[string]any{
				"action":      "create",
				"type":        "tod",
				"id":          "test_tod",
				"name":        "Test Time of Day",
				"after_time":  "06:00:00",
				"before_time": "22:00:00",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "binary_sensor.test_time_of_day"},
		},
		{
			name: "create tod missing after_time",
			args: map[string]any{
				"action":      "create",
				"type":        "tod",
				"id":          "test_tod",
				"name":        "Test Time of Day",
				"before_time": "22:00:00",
			},
			wantError:    true,
			wantContains: []string{"after_time", "required"},
		},
		{
			name: "create tod missing before_time",
			args: map[string]any{
				"action":     "create",
				"type":       "tod",
				"id":         "test_tod",
				"name":       "Test Time of Day",
				"after_time": "06:00:00",
			},
			wantError:    true,
			wantContains: []string{"before_time", "required"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_GenericThermostat tests generic_thermostat helper creation.
func TestManageHelper_GenericThermostat(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create generic_thermostat success",
			args: map[string]any{
				"action":                  "create",
				"type":                    "generic_thermostat",
				"id":                      "test_thermo",
				"name":                    "Test Thermostat",
				"heater_entity_id":        "switch.heater",
				"target_sensor_entity_id": "sensor.room_temp",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "temperature"},
					}, nil
				}
			},
			wantContains: []string{"created", "climate.test_thermo"},
		},
		{
			name: "create generic_thermostat missing heater_entity_id",
			args: map[string]any{
				"action":                  "create",
				"type":                    "generic_thermostat",
				"id":                      "test_thermo",
				"name":                    "Test Thermostat",
				"target_sensor_entity_id": "sensor.room_temp",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "temperature"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"heater_entity_id", "required"},
		},
		{
			name: "create generic_thermostat missing target_sensor_entity_id",
			args: map[string]any{
				"action":           "create",
				"type":             "generic_thermostat",
				"id":               "test_thermo",
				"name":             "Test Thermostat",
				"heater_entity_id": "switch.heater",
			},
			wantError:    true,
			wantContains: []string{"target_sensor_entity_id", "required"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_SwitchAsX tests switch_as_x helper creation.
func TestManageHelper_SwitchAsX(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create switch_as_x as light",
			args: map[string]any{
				"action":        "create",
				"type":          "switch_as_x",
				"id":            "test_switch",
				"name":          "Test Switch as Light",
				"entity_id":     "switch.outlet",
				"target_domain": "light",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantContains: []string{"created", "light.test_switch"},
		},
		{
			name: "create switch_as_x missing entity_id",
			args: map[string]any{
				"action":        "create",
				"type":          "switch_as_x",
				"id":            "test_switch",
				"name":          "Test Switch",
				"target_domain": "light",
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
		{
			name: "create switch_as_x missing target_domain",
			args: map[string]any{
				"action":    "create",
				"type":      "switch_as_x",
				"id":        "test_switch",
				"name":      "Test Switch",
				"entity_id": "switch.outlet",
			},
			wantError:    true,
			wantContains: []string{"target_domain", "required"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_GenericHygrostat tests generic_hygrostat helper creation.
func TestManageHelper_GenericHygrostat(t *testing.T) {
	tests := []handlerTestCase{
		{
			name: "create generic_hygrostat success",
			args: map[string]any{
				"action":                  "create",
				"type":                    "generic_hygrostat",
				"id":                      "test_hygro",
				"name":                    "Test Hygrostat",
				"humidifier_entity_id":    "switch.humidifier",
				"target_sensor_entity_id": "sensor.room_humidity",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(context.Context, homeassistant.HelperConfig) error {
					return nil
				}
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "humidity"},
					}, nil
				}
			},
			wantContains: []string{"created", "humidifier.test_hygro"},
		},
		{
			name: "create generic_hygrostat missing humidifier_entity_id",
			args: map[string]any{
				"action":                  "create",
				"type":                    "generic_hygrostat",
				"id":                      "test_hygro",
				"name":                    "Test Hygrostat",
				"target_sensor_entity_id": "sensor.room_humidity",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "humidity"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"humidifier_entity_id", "required"},
		},
		{
			name: "create generic_hygrostat missing target_sensor_entity_id",
			args: map[string]any{
				"action":               "create",
				"type":                 "generic_hygrostat",
				"id":                   "test_hygro",
				"name":                 "Test Hygrostat",
				"humidifier_entity_id": "switch.humidifier",
			},
			wantError:    true,
			wantContains: []string{"target_sensor_entity_id", "required"},
		},
	}

	handler := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, handler.handleManageHelper)
}

// TestManageHelper_ExtendedTypesMetadata verifies all extended types are registered in helperTypes.
func TestManageHelper_ExtendedTypesMetadata(t *testing.T) {
	extendedTypes := []string{
		"utility_meter", "min_max", "statistics", "trend",
		"random_sensor", "random_binary_sensor", "filter", "tod",
		"generic_thermostat", "switch_as_x", "generic_hygrostat",
	}

	for _, helperType := range extendedTypes {
		t.Run(helperType, func(t *testing.T) {
			meta, ok := helperTypes[helperType]
			require.True(t, ok, "Helper type %s not found in helperTypes map", helperType)
			assert.NotEmpty(t, meta.platform, "Platform should not be empty")
			assert.NotEmpty(t, meta.entityPrefix, "Entity prefix should not be empty")
			assert.NotEmpty(t, meta.validEntityDomains, "Valid entity domains should not be empty")
		})
	}
}

// TestManageHelper_ExtendedTypesConfigBuilders verifies all extended types have config builders.
func TestManageHelper_ExtendedTypesConfigBuilders(t *testing.T) {
	// Test that all extended types have registered config builders
	extendedTypes := []string{
		platformUtilityMeter, platformMinMax, platformStatistics, platformTrend,
		helperTypeRandomSensor, helperTypeRandomBinarySensor, platformFilter,
		platformTod, platformGenericThermostat, platformSwitchAsX, platformGenericHygrostat,
	}

	for _, helperType := range extendedTypes {
		t.Run(helperType, func(t *testing.T) {
			builder, ok := helperConfigBuilders[helperType]
			require.True(t, ok, "Config builder not found for %s", helperType)
			assert.NotNil(t, builder, "Config builder should not be nil")

			// Test that builder can be called without panic
			config := make(map[string]any)
			args := make(map[string]any)
			err := builder(config, args)
			assert.NoError(t, err, "Builder should not error on empty args")
		})
	}
}

// TestManageHelper_ExtendedSchemaProperties verifies extended schema properties are present.
func TestManageHelper_ExtendedSchemaProperties(t *testing.T) {
	handlers := NewConsolidatedHelperHandlers()
	tool := handlers.manageHelperTool()

	// Check for some key extended properties
	extendedProps := []string{
		"source", "cycle", "entity_ids", "state_characteristic",
		"after_time", "before_time", "heater_entity_id", "target_sensor_entity_id",
		"target_domain", "humidifier_entity_id",
	}

	for _, prop := range extendedProps {
		t.Run(prop, func(t *testing.T) {
			schema, ok := tool.InputSchema.Properties[prop]
			assert.True(t, ok, "Property %s should be in schema", prop)
			assert.NotEmpty(t, schema.Type, "Property %s should have a type", prop)
		})
	}
}

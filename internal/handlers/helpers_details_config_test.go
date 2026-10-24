package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// runGetDetails calls manage_helper get_details for entityID with the given
// format against m and fails the test on a Go-level error.
func runGetDetails(t *testing.T, m *UniversalMockClient, entityID, format string) *mcp.ToolsCallResult {
	t.Helper()
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})
	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, m, map[string]any{
		"action":    "get_details",
		"entity_id": entityID,
		"format":    format,
	})
	if err != nil {
		t.Fatalf("handleManageHelper() error = %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("handleManageHelper() returned no content: %+v", result)
	}
	if result.IsError {
		t.Fatalf("handleManageHelper() IsError = true: %s", result.Content[0].Text)
	}
	return result
}

// getDetailsJSON runs get_details in JSON format and decodes the first block.
func getDetailsJSON(t *testing.T, m *UniversalMockClient, entityID string) (map[string]any, *mcp.ToolsCallResult) {
	t.Helper()
	result := runGetDetails(t, m, entityID, "json")
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
		t.Fatalf("first content block is not JSON: %v\n%s", err, result.Content[0].Text)
	}
	return decoded, result
}

func stateFn(state string, attrs map[string]any) func(context.Context, string) (*homeassistant.Entity, error) {
	return func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
		return &homeassistant.Entity{EntityID: entityID, State: state, Attributes: attrs}, nil
	}
}

func registryFn(entries ...homeassistant.EntityRegistryEntry) func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	return func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
		return entries, nil
	}
}

func optionsFn(options map[string]any) func(context.Context, string) (map[string]any, error) {
	return func(context.Context, string) (map[string]any, error) {
		return options, nil
	}
}

func storageFn(cfg map[string]any) func(context.Context, string, string) (map[string]any, error) {
	return func(context.Context, string, string) (map[string]any, error) {
		return cfg, nil
	}
}

func statisticsMock() *UniversalMockClient {
	return &UniversalMockClient{
		GetStateFn: stateFn("12.5", map[string]any{"friendly_name": "Stats"}),
		GetEntityRegistryFn: registryFn(homeassistant.EntityRegistryEntry{
			EntityID: "sensor.stats", Platform: "statistics", ConfigEntryID: "ce",
		}),
		GetConfigEntryOptionsFn: optionsFn(map[string]any{
			"entity_id":            "sensor.src",
			"state_characteristic": "mean",
			"sampling_size":        20.0,
			"max_age":              map[string]any{"hours": 0.0, "minutes": 10.0, "seconds": 0.0},
			"keep_last_sample":     false,
			"precision":            2.0,
		}),
	}
}

func TestGetDetails_StatisticsShowsOptionsConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("natural", func(t *testing.T) {
		t.Parallel()
		out := runGetDetails(t, statisticsMock(), "sensor.stats", "natural").Content[0].Text
		for _, want := range []string{"Configuration:", "Max age: 00:10:00", "Sampling size: 20", "Entity id: sensor.src", "Keep last sample: false"} {
			if !strings.Contains(out, want) {
				t.Errorf("natural output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		decoded, _ := getDetailsJSON(t, statisticsMock(), "sensor.stats")
		if decoded["entity_id"] != "sensor.stats" {
			t.Errorf("top-level entity_id = %v, want sensor.stats", decoded["entity_id"])
		}
		cfg, ok := decoded["configuration"].(map[string]any)
		if !ok {
			t.Fatalf("configuration missing or not a map: %v", decoded)
		}
		if cfg["entity_id"] != "sensor.src" {
			t.Errorf("configuration.entity_id = %v, want sensor.src", cfg["entity_id"])
		}
		maxAge, ok := cfg["max_age"].(map[string]any)
		if !ok {
			t.Fatalf("configuration.max_age is not a map: %v", cfg["max_age"])
		}
		if maxAge["minutes"] != 10.0 {
			t.Errorf("configuration.max_age.minutes = %v, want 10", maxAge["minutes"])
		}
	})
}

func TestGetDetails_InputNumberStorageConfigNoDuplicates(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("5", map[string]any{
			"friendly_name": "N", "min": 0.0, "max": 100.0, "step": 1.0, "mode": "slider",
		}),
		GetHelperConfigFn: storageFn(map[string]any{
			"id": "n", "name": "N", "min": 0.0, "max": 100.0, "step": 1.0,
			"mode": "slider", "initial": 5.0, "icon": "mdi:x",
		}),
	}

	decoded, _ := getDetailsJSON(t, m, "input_number.n")
	want := map[string]any{"initial": 5.0, "icon": "mdi:x"}
	if !reflect.DeepEqual(decoded["configuration"], want) {
		t.Errorf("configuration = %v, want %v", decoded["configuration"], want)
	}
}

func TestGetDetails_CounterAndTimerShowRestore(t *testing.T) {
	t.Parallel()

	t.Run("counter", func(t *testing.T) {
		t.Parallel()
		m := &UniversalMockClient{
			GetStateFn: stateFn("0", map[string]any{"friendly_name": "C"}),
			GetHelperConfigFn: storageFn(map[string]any{
				"id": "c", "name": "C", "initial": 0.0, "step": 1.0, "restore": false,
			}),
		}
		out := runGetDetails(t, m, "counter.c", "natural").Content[0].Text
		if !strings.Contains(out, "Restore: false") {
			t.Errorf("natural output missing %q:\n%s", "Restore: false", out)
		}
	})

	t.Run("timer", func(t *testing.T) {
		t.Parallel()
		m := &UniversalMockClient{
			GetStateFn: stateFn("idle", map[string]any{"friendly_name": "T", "duration": "0:05:00"}),
			GetHelperConfigFn: storageFn(map[string]any{
				"id": "t", "duration": "0:05:00", "restore": true,
			}),
		}
		decoded, _ := getDetailsJSON(t, m, "timer.t")
		want := map[string]any{"restore": true}
		if !reflect.DeepEqual(decoded["configuration"], want) {
			t.Errorf("configuration = %v, want %v", decoded["configuration"], want)
		}
	})
}

func TestGetDetails_NonHelperConfigEntryEntity_SkipsOptionsFlow(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("21.5", map[string]any{"friendly_name": "ZHA Temp", "unit_of_measurement": "°C"}),
		GetEntityRegistryFn: registryFn(homeassistant.EntityRegistryEntry{
			EntityID: "sensor.zha_temp", Platform: "zha", ConfigEntryID: "z",
		}),
		GetConfigEntryOptionsFn: func(context.Context, string) (map[string]any, error) {
			t.Error("options flow must not be started for a non-helper integration")
			return map[string]any{"leak": true}, nil
		},
	}

	decoded, result := getDetailsJSON(t, m, "sensor.zha_temp")
	if len(result.Content) != 1 {
		t.Errorf("len(Content) = %d, want 1: %+v", len(result.Content), result.Content)
	}
	if _, found := decoded["configuration"]; found {
		t.Errorf("unexpected configuration key: %v", decoded["configuration"])
	}
}

// assertWarningShape checks the two-block success-with-warning result shape.
func assertWarningShape(t *testing.T, result *mcp.ToolsCallResult, wantInWarning string) {
	t.Helper()
	if result.IsError {
		t.Fatalf("IsError = true, want false")
	}
	if len(result.Content) != 2 {
		t.Fatalf("len(Content) = %d, want 2: %+v", len(result.Content), result.Content)
	}
	if !strings.Contains(result.Content[0].Text, "Sensor:") {
		t.Errorf("Content[0] missing %q:\n%s", "Sensor:", result.Content[0].Text)
	}
	warning := result.Content[1].Text
	if !strings.HasPrefix(warning, "WARNING: configuration could not be read: ") {
		t.Errorf("warning prefix wrong: %q", warning)
	}
	if !strings.Contains(warning, wantInWarning) {
		t.Errorf("warning %q does not contain %q", warning, wantInWarning)
	}
	if strings.ContainsAny(warning, "\r\n") {
		t.Errorf("warning contains a line break: %q", warning)
	}
}

func TestGetDetails_OptionsFetchFailure_WarnsButSucceeds(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("1", map[string]any{"friendly_name": "Tpl"}),
		GetEntityRegistryFn: registryFn(homeassistant.EntityRegistryEntry{
			EntityID: "sensor.tpl", Platform: "template", ConfigEntryID: "ce",
		}),
		GetConfigEntryOptionsFn: func(context.Context, string) (map[string]any, error) {
			return nil, errors.New("boom\nsecond line")
		},
	}

	result := runGetDetails(t, m, "sensor.tpl", "natural")
	assertWarningShape(t, result, "boom second line")
}

func TestGetDetails_RegistryFailure_Warns(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("1", map[string]any{"friendly_name": "Tpl"}),
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return nil, errors.New("registry down")
		},
	}

	result := runGetDetails(t, m, "sensor.tpl", "natural")
	assertWarningShape(t, result, "registry down")
}

func TestGetDetails_WSStorageNotFound_NoWarning(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("on", map[string]any{"friendly_name": "YAML One"}),
		GetHelperConfigFn: func(context.Context, string, string) (map[string]any, error) {
			return nil, fmt.Errorf("input_boolean not found: x: %w", homeassistant.ErrHelperNotFoundInStorage)
		},
	}

	result := runGetDetails(t, m, "input_boolean.yaml_one", "natural")
	if len(result.Content) != 1 {
		t.Errorf("len(Content) = %d, want 1: %+v", len(result.Content), result.Content)
	}
}

func TestGetDetails_ConfigurationAttributeCollision(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("heat", map[string]any{"friendly_name": "Thermo", "configuration": "attr-value"}),
		GetEntityRegistryFn: registryFn(homeassistant.EntityRegistryEntry{
			EntityID: "climate.thermo", Platform: "generic_thermostat", ConfigEntryID: "ce",
		}),
		GetConfigEntryOptionsFn: optionsFn(map[string]any{"heater": "switch.h"}),
	}

	decoded, _ := getDetailsJSON(t, m, "climate.thermo")
	want := map[string]any{"heater": "switch.h"}
	if !reflect.DeepEqual(decoded["configuration"], want) {
		t.Errorf("configuration = %v, want %v", decoded["configuration"], want)
	}
	if decoded["attr_configuration"] != "attr-value" {
		t.Errorf("attr_configuration = %v, want attr-value", decoded["attr_configuration"])
	}
}

func TestGetDetails_RegistryIconShownForConfigEntryHelper(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("3", map[string]any{"friendly_name": "Stats"}),
		GetEntityRegistryFn: registryFn(homeassistant.EntityRegistryEntry{
			EntityID: "sensor.stats", Platform: "statistics", ConfigEntryID: "ce", Icon: "mdi:chart",
		}),
		GetConfigEntryOptionsFn: optionsFn(map[string]any{"sampling_size": 20.0}),
	}

	decoded, _ := getDetailsJSON(t, m, "sensor.stats")
	cfg, ok := decoded["configuration"].(map[string]any)
	if !ok {
		t.Fatalf("configuration missing or not a map: %v", decoded)
	}
	if cfg["icon"] != "mdi:chart" {
		t.Errorf("configuration.icon = %v, want mdi:chart", cfg["icon"])
	}
}

func TestGetDetails_ScheduleConfigurationShowsIconNotDays(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("off", map[string]any{"friendly_name": "S"}),
		GetHelperConfigFn: storageFn(map[string]any{
			"id": "s", "name": "S", "icon": "mdi:cal",
			"monday": []any{map[string]any{"from": "08:00:00", "to": "09:00:00"}},
		}),
	}

	decoded, _ := getDetailsJSON(t, m, "schedule.s")
	want := map[string]any{"icon": "mdi:cal"}
	if !reflect.DeepEqual(decoded["configuration"], want) {
		t.Errorf("configuration = %v, want %v", decoded["configuration"], want)
	}
	schedule, ok := decoded["schedule"].(map[string]any)
	if !ok {
		t.Fatalf("schedule missing or not a map: %v", decoded)
	}
	if _, found := schedule["monday"]; !found {
		t.Errorf("schedule.monday missing: %v", schedule)
	}
}

// TestGetDetails_ScheduleStorageErrors pins handleGetDetailsSchedule's own
// storage-read error handling: YAML-defined is silent, anything else warns.
func TestGetDetails_ScheduleStorageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantBlocks int
	}{
		{"yaml-defined schedule has no warning", fmt.Errorf("schedule not found: s: %w", homeassistant.ErrHelperNotFoundInStorage), 1},
		{"other storage error warns", errors.New("ws closed\nreconnecting"), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := &UniversalMockClient{
				GetStateFn: stateFn("off", map[string]any{"friendly_name": "S"}),
				GetHelperConfigFn: func(context.Context, string, string) (map[string]any, error) {
					return nil, tt.err
				},
			}
			result := runGetDetails(t, m, "schedule.s", "natural")
			if len(result.Content) != tt.wantBlocks {
				t.Fatalf("len(Content) = %d, want %d: %+v", len(result.Content), tt.wantBlocks, result.Content)
			}
			if tt.wantBlocks == 2 && result.Content[1].Text != "WARNING: configuration could not be read: ws closed reconnecting" {
				t.Errorf("warning = %q", result.Content[1].Text)
			}
		})
	}
}

func templateConsumedMock() *UniversalMockClient {
	return &UniversalMockClient{
		GetStateFn: stateFn("1", map[string]any{"friendly_name": "Tpl", "unit_of_measurement": "W"}),
		GetEntityRegistryFn: registryFn(homeassistant.EntityRegistryEntry{
			EntityID: "sensor.tpl", Platform: "template", ConfigEntryID: "ce",
		}),
		GetConfigEntryOptionsFn: optionsFn(map[string]any{
			"state":               "{{ 1 }}",
			"availability":        "{{ true }}",
			"unit_of_measurement": "W",
		}),
	}
}

func TestGetDetails_TemplateConsumedKeysNotRepeated(t *testing.T) {
	t.Parallel()

	t.Run("natural", func(t *testing.T) {
		t.Parallel()
		out := runGetDetails(t, templateConsumedMock(), "sensor.tpl", "natural").Content[0].Text
		if got := strings.Count(out, "{{ 1 }}"); got != 1 {
			t.Errorf("strings.Count(out, %q) = %d, want 1:\n%s", "{{ 1 }}", got, out)
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		decoded, _ := getDetailsJSON(t, templateConsumedMock(), "sensor.tpl")
		raw, found := decoded["configuration"]
		if !found {
			return
		}
		cfg, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("configuration is not a map: %v", raw)
		}
		for _, key := range []string{"state", "availability", "unit_of_measurement"} {
			if _, dup := cfg[key]; dup {
				t.Errorf("configuration repeats consumed key %q: %v", key, cfg)
			}
		}
	})
}

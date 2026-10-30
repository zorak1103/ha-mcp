package handlers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	modeCurrent    = "current"
	modeHistory    = "history"
	modeStatistics = "statistics"
	modeDomains    = "domains"
)

func TestConsolidatedEntityQueryHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedEntityQueryHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	if tools[0].Name != "query_entities" {
		t.Errorf("Tool name = %q, want %q", tools[0].Name, "query_entities")
	}
}

func TestQueryEntitiesTool_Schema(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedEntityQueryHandlers()
	tool := h.queryEntitiesTool()

	if tool.Name != "query_entities" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "query_entities")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Verify required fields
	requiredFields := make(map[string]bool)
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	if !requiredFields["mode"] {
		t.Error("Required field 'mode' not found")
	}

	// Verify mode enum
	modeSchema, ok := tool.InputSchema.Properties["mode"]
	if !ok {
		t.Fatal("mode property not found")
	}

	expectedModes := []string{modeCurrent, modeHistory, modeStatistics, modeDomains, "presence", "health"}
	if len(modeSchema.Enum) != len(expectedModes) {
		t.Errorf("mode enum has %d values, want %d", len(modeSchema.Enum), len(expectedModes))
	}

	// Verify format enum
	formatSchema, ok := tool.InputSchema.Properties["format"]
	if !ok {
		t.Fatal("format property not found")
	}

	expectedFormats := []string{"natural", "json"}
	if len(formatSchema.Enum) != len(expectedFormats) {
		t.Errorf("format enum has %d values, want %d", len(formatSchema.Enum), len(expectedFormats))
	}

	// Verify period enum has 5 values
	periodSchema, ok := tool.InputSchema.Properties["period"]
	if !ok {
		t.Fatal("period property not found")
	}
	expectedPeriods := 5
	if len(periodSchema.Enum) != expectedPeriods {
		t.Errorf("period enum has %d values, want %d", len(periodSchema.Enum), expectedPeriods)
	}
}

func TestQueryEntities_MissingMode(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedEntityQueryHandlers()
	result, err := h.handleQueryEntities(context.Background(), &UniversalMockClient{}, map[string]any{})

	if err != nil {
		t.Fatalf("handleQueryEntities() error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError = true")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "mode") || !strings.Contains(content, "required") {
		t.Errorf("Content = %q, want error about missing mode", content)
	}
}

func TestQueryEntities_InvalidMode(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedEntityQueryHandlers()
	result, err := h.handleQueryEntities(context.Background(), &UniversalMockClient{}, map[string]any{
		"mode": "invalid",
	})

	if err != nil {
		t.Fatalf("handleQueryEntities() error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError = true")
	}

	content := result.Content[0].Text
	assertContainsAll(t, content, []string{"Invalid mode", "current", "history", "statistics", "domains"})
}

func TestQueryEntities_Current(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Living Room Light",
			},
		},
		{
			EntityID: "light.bedroom",
			State:    "off",
			Attributes: map[string]any{
				"friendly_name": "Bedroom Light",
			},
		},
		{
			EntityID: "switch.kitchen",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Kitchen Switch",
			},
		},
	}

	tests := []handlerTestCase{
		{
			name: "basic - all entities",
			args: map[string]any{"mode": modeCurrent, "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light.living_room", "light.bedroom", "switch.kitchen"},
		},
		{
			name: "domain filter",
			args: map[string]any{"mode": modeCurrent, "domain": "light", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room", "light.bedroom"},
			wantNotContains: []string{"switch.kitchen"},
		},
		{
			name: "state filter",
			args: map[string]any{"mode": modeCurrent, "state": "on", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room", "switch.kitchen"},
			wantNotContains: []string{"light.bedroom"},
		},
		{
			name: "state_not filter",
			args: map[string]any{"mode": modeCurrent, "state_not": "off", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room", "switch.kitchen"},
			wantNotContains: []string{"light.bedroom"},
		},
		{
			name: "name_contains filter",
			args: map[string]any{"mode": modeCurrent, "name_contains": "living", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room"},
			wantNotContains: []string{"light.bedroom", "switch.kitchen"},
		},
		{
			name: "name_contains comma-separated multi-keyword (OR)",
			args: map[string]any{"mode": modeCurrent, "name_contains": "living,kitchen", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room", "switch.kitchen"},
			wantNotContains: []string{"light.bedroom"},
		},
		{
			name: "device_class filter - motion",
			args: map[string]any{"mode": modeCurrent, "domain": "binary_sensor", "device_class": "motion", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID: "binary_sensor.motion_hall",
							State:    "on",
							Attributes: map[string]any{
								"device_class":  "motion",
								"friendly_name": "Hall Motion",
							},
						},
						{
							EntityID: "binary_sensor.motion_living",
							State:    "off",
							Attributes: map[string]any{
								"device_class":  "motion",
								"friendly_name": "Living Room Motion",
							},
						},
						{
							EntityID: "binary_sensor.door_front",
							State:    "off",
							Attributes: map[string]any{
								"device_class":  "door",
								"friendly_name": "Front Door",
							},
						},
					}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"binary_sensor.motion_hall", "binary_sensor.motion_living"},
			wantNotContains: []string{"binary_sensor.door_front"},
		},
		{
			name: "device_class filter - no match",
			args: map[string]any{"mode": modeCurrent, "device_class": "temperature", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"[]"}, // Empty result
		},
		{
			name: "device_class filter - entity without device_class",
			args: map[string]any{"mode": modeCurrent, "device_class": "motion", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "light.test", State: "on", Attributes: map[string]any{"brightness": 255}},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"[]"}, // Empty result - no device_class
		},
		{
			name: "pagination",
			args: map[string]any{"mode": modeCurrent, "limit": float64(2), "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"pagination", "limit"},
		},
		{
			name: "client error",
			args: map[string]any{"mode": modeCurrent},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return nil, errors.New("connection refused")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "connection refused"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Current_FormatJSON(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.test", State: "on", Attributes: map[string]any{"brightness": 255}},
	}

	tests := []handlerTestCase{
		{
			name: "compact JSON",
			args: map[string]any{"mode": modeCurrent, "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.test", `"state"`},
			wantNotContains: []string{"brightness"},
		},
		{
			name: "verbose JSON",
			args: map[string]any{"mode": modeCurrent, "format": "json", "verbose": true},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light.test", "brightness", "255"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Current_FormatNatural(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on", Attributes: map[string]any{"friendly_name": "Living Room"}},
		{EntityID: "light.bedroom", State: "off", Attributes: map[string]any{"friendly_name": "Bedroom"}},
	}

	tests := []handlerTestCase{
		{
			name: "natural format compact list (default non-verbose)",
			args: map[string]any{"mode": modeCurrent, "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light", "2", "entities", "light.living_room", "light.bedroom"},
		},
		{
			name: "natural format verbose mode",
			args: map[string]any{"mode": modeCurrent, "format": "natural", "verbose": true},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"entities", "Living Room", "Bedroom"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Current_AreaFilter(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on", Attributes: map[string]any{"friendly_name": "Living Room"}},
		{EntityID: "light.bedroom", State: "off", Attributes: map[string]any{"friendly_name": "Bedroom"}},
		{EntityID: "light.kitchen", State: "on", Attributes: map[string]any{"friendly_name": "Kitchen"}},
		{EntityID: "switch.garage", State: "off", Attributes: map[string]any{"friendly_name": "Garage"}},
	}

	entityRegistry := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.living_room", AreaID: "living_room", DeviceID: ""},
		{EntityID: "light.bedroom", AreaID: "bedroom", DeviceID: ""},
		{EntityID: "light.kitchen", AreaID: "", DeviceID: "device_kitchen"}, // via device
		{EntityID: "switch.garage", AreaID: "garage", DeviceID: ""},
	}

	deviceRegistry := []homeassistant.DeviceRegistryEntry{
		{ID: "device_kitchen", AreaID: "living_room"}, // kitchen light is in living_room via device
	}

	tests := []handlerTestCase{
		{
			name: "area_id direct match",
			args: map[string]any{"mode": modeCurrent, "area_id": "living_room", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room", "light.kitchen"},
			wantNotContains: []string{"light.bedroom", "switch.garage"},
		},
		{
			name: "area_id with domain filter",
			args: map[string]any{"mode": modeCurrent, "area_id": "living_room", "domain": "light", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room", "light.kitchen"},
			wantNotContains: []string{"light.bedroom", "switch.garage"},
		},
		{
			name: "area_id with state filter",
			args: map[string]any{"mode": modeCurrent, "area_id": "living_room", "state": "on", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"light.living_room", "light.kitchen"},
			wantNotContains: []string{"light.bedroom", "switch.garage"},
		},
		{
			name: "area_id no matches",
			args: map[string]any{"mode": modeCurrent, "area_id": "nonexistent", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError:       false,
			wantNotContains: []string{"light.living_room", "light.bedroom", "light.kitchen", "switch.garage"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_History(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testHistory := map[string][]homeassistant.HistoryEntry{
		"sensor.temp": {
			{EntityID: "sensor.temp", State: "20.0", LastChanged: float64(now.Add(-2 * time.Hour).Unix())},
			{EntityID: "sensor.temp", State: "21.0", LastChanged: float64(now.Add(-1 * time.Hour).Unix())},
			{EntityID: "sensor.temp", State: "22.0", LastChanged: float64(now.Unix())},
		},
	}

	tests := []handlerTestCase{
		{
			name: "basic history",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.temp", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"20.0", "21.0", "22.0", "Found 3 history entries"},
		},
		{
			name: "history with hours",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.temp", "hours": float64(6), "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries"},
		},
		{
			name: "history with state filter",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.temp", "state": "21.0", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"21.0", "filtered by state='21.0'"},
			wantNotContains: []string{"20.0", "22.0"},
		},
		{
			name: "history with limit",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.temp", "limit": float64(2), "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"Showing 2 of 3", "21.0", "22.0"},
			wantNotContains: []string{"20.0"},
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{"mode": modeHistory},
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name: "client error",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.temp"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return nil, errors.New("database unavailable")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "database unavailable"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_History_FormatJSON(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testHistory := map[string][]homeassistant.HistoryEntry{
		"sensor.test": {{EntityID: "sensor.test", State: "on", LastChanged: float64(now.Unix()), Attributes: map[string]any{"brightness": 255}}},
	}

	tests := []handlerTestCase{
		{
			name: "compact JSON",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.test", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:       false,
			wantContains:    []string{`"state"`, `"last_changed"`},
			wantNotContains: []string{"brightness"},
		},
		{
			name: "verbose JSON",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.test", "format": "json", "verbose": true},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"brightness", "255"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_History_FormatNatural(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testHistory := map[string][]homeassistant.HistoryEntry{
		"sensor.test": {
			{EntityID: "sensor.test", State: "on", LastChanged: float64(now.Unix())},
			{EntityID: "sensor.test", State: "off", LastChanged: float64(now.Add(-1 * time.Hour).Unix())},
		},
	}

	tests := []handlerTestCase{
		{
			name: "natural language history",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.test", "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"sensor.test", "History"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

// TestHandleHistory_SingleEntityOutputPinned pins the exact single-entity history output in
// natural, compact json and verbose json. Multi-entity history must keep these byte-identical.
// Timestamps are rendered in the local zone (time.Unix), so the expected strings derive them the
// same way instead of hard-coding a zone.
func TestHandleHistory_SingleEntityOutputPinned(t *testing.T) {
	t.Parallel()

	const lc1, lc2 = 1700000000, 1700003600
	history := map[string][]homeassistant.HistoryEntry{
		"sensor.temp": {
			{EntityID: "sensor.temp", State: "20.5", LastChanged: lc1, Attributes: map[string]any{
				"friendly_name": "Living Room Temp", "unit_of_measurement": "°C",
			}},
			{EntityID: "sensor.temp", State: "21.0", LastChanged: lc2, LastUpdated: lc2},
		},
	}
	minute := func(ts int64) string { return time.Unix(ts, 0).Format("2006-01-02 15:04") }
	rfc := func(ts int64) string { return time.Unix(ts, 0).Format(time.RFC3339) }

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "natural",
			args: map[string]any{"format": "natural"},
			want: "History for Living Room Temp: 2 state changes.\n" +
				minute(lc1) + " → 20.5\n" +
				minute(lc2) + " → 21.0",
		},
		{
			name: "natural verbose",
			args: map[string]any{"format": "natural", "verbose": true},
			want: "History for Living Room Temp: 2 state changes.\n" +
				minute(lc1) + " → 20.5 (unit_of_measurement=°C)\n" +
				minute(lc2) + " → 21.0",
		},
		{
			name: "json compact",
			args: map[string]any{"format": "json"},
			want: "Found 2 history entries for sensor.temp (use verbose=true for full details)\n\n" +
				"[\n" +
				"  {\n    \"state\": \"20.5\",\n    \"last_changed\": \"" + rfc(lc1) + "\"\n  },\n" +
				"  {\n    \"state\": \"21.0\",\n    \"last_changed\": \"" + rfc(lc2) + "\"\n  }\n" +
				"]",
		},
		{
			name: "json verbose",
			args: map[string]any{"format": "json", "verbose": true},
			want: `Found 2 history entries for sensor.temp

[
  {
    "entity_id": "sensor.temp",
    "s": "20.5",
    "a": {
      "friendly_name": "Living Room Temp",
      "unit_of_measurement": "°C"
    },
    "lc": 1700000000
  },
  {
    "entity_id": "sensor.temp",
    "s": "21.0",
    "lc": 1700003600,
    "lu": 1700003600
  }
]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				GetHistoryFn: func(_ context.Context, ids []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					if len(ids) != 1 || ids[0] != "sensor.temp" {
						t.Errorf("GetHistory entityIDs = %v, want %q", ids, "sensor.temp")
					}
					return history, nil
				},
			}
			args := map[string]any{"mode": modeHistory, "entity_id": "sensor.temp"}
			maps.Copy(args, tt.args)

			result, err := NewConsolidatedEntityQueryHandlers().handleQueryEntities(context.Background(), client, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("unexpected error result: %s", result.Content[0].Text)
			}
			if len(result.Content) != 1 {
				t.Fatalf("len(Content) = %d, want 1", len(result.Content))
			}
			if got := result.Content[0].Text; got != tt.want {
				t.Errorf("output mismatch\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestQueryEntities_History_EmptyHistory_EntityExists(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "entity exists but no history",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.new_sensor"},
			setupMock: func(m *UniversalMockClient) {
				// Empty history
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return map[string][]homeassistant.HistoryEntry{}, nil
				}
				// Entity exists (GetState succeeds)
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{EntityID: entityID, State: "20.5"}, nil
				}
			},
			wantError: false,
			wantContains: []string{
				"No history found",
				"sensor.new_sensor",
				"newly created",
				"requested period",
			},
			wantNotContains: []string{"not found", "verify"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_History_EntityNotFound(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "entity does not exist",
			args: map[string]any{"mode": modeHistory, "entity_id": "sensor.nonexistent"},
			setupMock: func(m *UniversalMockClient) {
				// Empty history
				m.GetHistoryFn = func(_ context.Context, _ []string, _, _ time.Time, _ homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					return map[string][]homeassistant.HistoryEntry{}, nil
				}
				// Entity doesn't exist (GetState fails)
				m.GetStateFn = func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return nil, errors.New("Entity not found")
				}
			},
			wantError: false,
			wantContains: []string{
				"not found",
				"sensor.nonexistent",
				"verify",
				"entity_id",
			},
			wantNotContains: []string{"newly created", "requested period"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Statistics(t *testing.T) {
	t.Parallel()

	meanVal := 100.5
	minVal := 50.0
	maxVal := 150.0
	testStatistics := []homeassistant.StatisticsResult{
		{StatisticID: "sensor.energy", Start: 1704067200, Mean: &meanVal, Min: &minVal, Max: &maxVal},
	}

	tests := []handlerTestCase{
		{
			name: "basic statistics",
			args: map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.energy"}},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatisticsFn = func(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
					return testStatistics, nil
				}
			},
			wantError:    false,
			wantContains: []string{"sensor.energy"},
		},
		{
			name: "statistics with period",
			args: map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.energy"}, "period": "day"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatisticsFn = func(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
					return testStatistics, nil
				}
			},
			wantError:    false,
			wantContains: []string{"sensor.energy"},
		},
		{
			name: "statistics with pagination",
			args: map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.energy"}, "limit": float64(10)},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatisticsFn = func(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
					return testStatistics, nil
				}
			},
			wantError:    false,
			wantContains: []string{"sensor.energy"},
		},
		{
			name:         "missing statistic_ids",
			args:         map[string]any{"mode": modeStatistics},
			wantError:    true,
			wantContains: []string{"statistic_ids is required"},
		},
		{
			name:         "invalid statistic_ids type",
			args:         map[string]any{"mode": modeStatistics, "statistic_ids": "not-an-array"},
			wantError:    true,
			wantContains: []string{"statistic_ids must be an array"},
		},
		{
			name: "client error",
			args: map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.energy"}},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatisticsFn = func(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
					return nil, errors.New("statistics unavailable")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "statistics unavailable"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Statistics_FormatNatural(t *testing.T) {
	t.Parallel()

	meanVal := 100.5
	testStats := []homeassistant.StatisticsResult{
		{StatisticID: "sensor.energy", Start: 1704067200, Mean: &meanVal},
	}

	tests := []handlerTestCase{
		{
			name: "natural format shows stat values",
			args: map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.energy"}, "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatisticsFn = func(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
					return testStats, nil
				}
			},
			wantError:    false,
			wantContains: []string{"sensor.energy", "mean"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Statistics_FormatJSON(t *testing.T) {
	t.Parallel()

	meanVal := 100.5
	testStats := []homeassistant.StatisticsResult{
		{StatisticID: "sensor.energy", Start: 1704067200, Mean: &meanVal},
	}

	tests := []handlerTestCase{
		{
			name: "backward-compatible JSON",
			args: map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.energy"}, "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatisticsFn = func(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
					return testStats, nil
				}
			},
			wantError:    false,
			wantContains: []string{"sensor.energy", "mean"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Domains(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "light.bedroom", State: "off"},
		{EntityID: "switch.kitchen", State: "on"},
	}

	tests := []handlerTestCase{
		{
			name: "basic domain listing",
			args: map[string]any{"mode": modeDomains},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light", "switch"},
		},
		{
			name: "empty states",
			args: map[string]any{"mode": modeDomains},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{}, nil
				}
			},
			wantError: false,
		},
		{
			name: "client error",
			args: map[string]any{"mode": modeDomains},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return nil, errors.New("connection timeout")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "connection timeout"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Domains_FormatNatural(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.one", State: "on"},
		{EntityID: "light.two", State: "off"},
		{EntityID: "switch.three", State: "on"},
	}

	tests := []handlerTestCase{
		{
			name: "natural format summary",
			args: map[string]any{"mode": modeDomains, "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"2 domains", "3 entities"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Domains_FormatJSON(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.one", State: "on"},
		{EntityID: "switch.two", State: "on"},
	}

	tests := []handlerTestCase{
		{
			name: "JSON array format",
			args: map[string]any{"mode": modeDomains, "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{`"domain"`, `"entity_count"`, "light", "switch"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestFormatStatValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stat    homeassistant.StatisticsResult
		want    string
		notWant []string
	}{
		{
			name: "all values present",
			stat: homeassistant.StatisticsResult{
				Mean:   ptrFloat(100.5),
				Min:    ptrFloat(50.0),
				Max:    ptrFloat(150.0),
				Sum:    ptrFloat(1000.0),
				State:  ptrFloat(125.0),
				Change: ptrFloat(25.0),
			},
			want:    "mean=100.5",
			notWant: []string{},
		},
		{
			name: "only mean",
			stat: homeassistant.StatisticsResult{
				Mean: ptrFloat(100.5),
			},
			want:    "mean=100.5",
			notWant: []string{"min=", "max=", "sum="},
		},
		{
			name:    "no values",
			stat:    homeassistant.StatisticsResult{},
			want:    "",
			notWant: []string{"mean=", "min=", "max=", "sum="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatStatValues(tt.stat)

			if tt.want != "" && !strings.Contains(result, tt.want) {
				t.Errorf("formatStatValues() = %q, want to contain %q", result, tt.want)
			}

			for _, notWant := range tt.notWant {
				if strings.Contains(result, notWant) {
					t.Errorf("formatStatValues() = %q, should not contain %q", result, notWant)
				}
			}
		})
	}
}

func ptrFloat(f float64) *float64 {
	return &f
}

func TestQueryEntities_Current_Sorting(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testStates := []homeassistant.Entity{
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Zebra Light",
			},
			LastChanged: now.Add(-10 * time.Minute),
		},
		{
			EntityID: "light.bedroom",
			State:    "off",
			Attributes: map[string]any{
				"friendly_name": "Apple Light",
			},
			LastChanged: now.Add(-5 * time.Minute),
		},
		{
			EntityID: "light.kitchen",
			State:    "unavailable",
			Attributes: map[string]any{
				"friendly_name": "Mango Light",
			},
			LastChanged: now,
		},
	}

	tests := []handlerTestCase{
		{
			name: "sort by entity_id (default)",
			args: map[string]any{"mode": modeCurrent, "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError: false,
			// Should be sorted: bedroom, kitchen, living_room
		},
		{
			name: "sort by state",
			args: map[string]any{"mode": modeCurrent, "sort_by": "state", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError: false,
			// Should be sorted: off, on, unavailable
		},
		{
			name: "sort by friendly_name",
			args: map[string]any{"mode": modeCurrent, "sort_by": "friendly_name", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError: false,
			// Should be sorted: Apple, Mango, Zebra
		},
		{
			name: "sort by last_changed",
			args: map[string]any{"mode": modeCurrent, "sort_by": "last_changed", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError: false,
			// Should be sorted: oldest to newest (living_room, bedroom, kitchen)
		},
		{
			name: "invalid sort_by",
			args: map[string]any{"mode": modeCurrent, "sort_by": "invalid"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    true,
			wantContains: []string{"invalid sort_by"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Current_Grouping(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on", Attributes: map[string]any{"friendly_name": "Living Room"}, LastChanged: time.Now().Add(-2 * time.Hour)},
		{EntityID: "light.bedroom", State: "off", Attributes: map[string]any{"friendly_name": "Bedroom"}},
		{EntityID: "switch.kitchen", State: "on", Attributes: map[string]any{"friendly_name": "Kitchen"}},
	}

	entityRegistry := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.living_room", AreaID: "living_room"},
		{EntityID: "light.bedroom", AreaID: "bedroom"},
		{EntityID: "switch.kitchen", AreaID: "living_room"},
	}

	deviceRegistry := []homeassistant.DeviceRegistryEntry{}

	// Dedicated fixture for the timestamp-suffix assertions below, independent of
	// testStates — mutating testStates for an unrelated subtest must not silently
	// change what these two verbose/non-verbose cases are actually asserting.
	timestampFixtureStates := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on", Attributes: map[string]any{"friendly_name": "Living Room"}, LastChanged: time.Now().Add(-2 * time.Hour)},
	}
	timestampFixtureRegistry := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.living_room", AreaID: "living_room"},
	}

	tests := []handlerTestCase{
		{
			name: "group by domain (natural format default)",
			args: map[string]any{"mode": modeCurrent, "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light", "switch"},
		},
		{
			name: "group by area_id (natural format)",
			args: map[string]any{"mode": modeCurrent, "group_by": "area_id", "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Area: living_room", "Area: bedroom", "- Living Room (light.living_room) is on", "Kitchen (switch.kitchen) is on"},
		},
		{
			name: "group by area_id non-verbose omits timestamp but keeps entity_id",
			args: map[string]any{"mode": modeCurrent, "group_by": "area_id", "format": "natural", "verbose": false},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return timestampFixtureStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return timestampFixtureRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"Living Room (light.living_room) is on"},
			wantNotContains: []string{". Changed"},
		},
		{
			name: "group by area_id verbose includes timestamp and entity_id",
			args: map[string]any{"mode": modeCurrent, "group_by": "area_id", "format": "natural", "verbose": true},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return timestampFixtureStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return timestampFixtureRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Living Room (light.living_room) is on", ". Changed"},
		},
		{
			name: "invalid group_by",
			args: map[string]any{"mode": modeCurrent, "group_by": "invalid"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    true,
			wantContains: []string{"invalid group_by"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

// TestQueryEntities_Current_GroupingCapped verifies that writeEntityList (used by
// group_by=area_id/device_class/integration) caps its per-group output the same way
// the ungrouped CompactList path already does. Before this fix, these grouped natural
// paths had no cap at all and no default limit applies (limit=0 means "no limit"), so
// a single large area could return every entity's full domain-aware detail line in one
// uncapped response.
func TestQueryEntities_Current_GroupingCapped(t *testing.T) {
	t.Parallel()

	const totalInArea = formatter.CompactListCap + 10

	testStates := make([]homeassistant.Entity, 0, totalInArea)
	entityRegistry := make([]homeassistant.EntityRegistryEntry, 0, totalInArea)
	for i := 0; i < totalInArea; i++ {
		entityID := fmt.Sprintf("light.office_%03d", i)
		testStates = append(testStates, homeassistant.Entity{
			EntityID:   entityID,
			State:      "on",
			Attributes: map[string]any{"friendly_name": fmt.Sprintf("Office Light %03d", i)},
		})
		entityRegistry = append(entityRegistry, homeassistant.EntityRegistryEntry{
			EntityID: entityID,
			AreaID:   "office",
		})
	}
	lastEntityID := testStates[totalInArea-1].EntityID

	tests := []handlerTestCase{
		{
			name: "group by area_id caps per-group output and notes the overflow",
			args: map[string]any{"mode": modeCurrent, "group_by": "area_id", "format": "natural", "verbose": false},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return nil, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"and 10 more (use pagination or verbose=true for full list)"},
			wantNotContains: []string{lastEntityID},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Current_GroupingWithSorting(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testStates := []homeassistant.Entity{
		{EntityID: "light.zzz", State: "on", LastChanged: now.Add(-10 * time.Minute), Attributes: map[string]any{"friendly_name": "ZZZ Light"}},
		{EntityID: "light.aaa", State: "off", LastChanged: now.Add(-5 * time.Minute), Attributes: map[string]any{"friendly_name": "AAA Light"}},
		{EntityID: "switch.bbb", State: "on", LastChanged: now.Add(-1 * time.Minute), Attributes: map[string]any{"friendly_name": "BBB Switch"}},
	}

	entityRegistry := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.zzz", AreaID: "area_1"},
		{EntityID: "light.aaa", AreaID: "area_1"},
		{EntityID: "switch.bbb", AreaID: "area_2"},
	}

	deviceRegistry := []homeassistant.DeviceRegistryEntry{}

	tests := []handlerTestCase{
		{
			name: "group_by=domain + sort_by=entity_id shows sorted entities",
			args: map[string]any{"mode": modeCurrent, "group_by": "domain", "sort_by": "entity_id", "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError: false,
			wantContains: []string{
				"**light**",  // domain header
				"AAA Light",  // should be first (alphabetically by entity_id)
				"ZZZ Light",  // should be second
				"**switch**", // domain header
				"BBB Switch", // only switch
			},
		},
		{
			name: "group_by=area_id + sort_by=last_changed shows sorted entities with criterion",
			args: map[string]any{"mode": modeCurrent, "group_by": "area_id", "sort_by": "last_changed", "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return deviceRegistry, nil
				}
			},
			wantError: false,
			wantContains: []string{
				"Area: area_1", // area header
				"light.aaa",    // should be first (most recent)
				"light.zzz",    // should be second (older)
				"Area: area_2", // area header
				"switch.bbb",   // only entity in area_2
			},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Current_GroupByDeviceClass(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{
			EntityID: "binary_sensor.motion_hall",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Hall Motion",
				"device_class":  "motion",
			},
		},
		{
			EntityID: "binary_sensor.motion_kitchen",
			State:    "off",
			Attributes: map[string]any{
				"friendly_name": "Kitchen Motion",
				"device_class":  "motion",
			},
		},
		{
			EntityID: "binary_sensor.door_front",
			State:    "off",
			Attributes: map[string]any{
				"friendly_name": "Front Door",
				"device_class":  "door",
			},
		},
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Living Room",
				// No device_class
			},
		},
	}

	tests := []handlerTestCase{
		{
			name: "group by device_class",
			args: map[string]any{"mode": modeCurrent, "group_by": "device_class", "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Class: motion", "Device Class: door", "binary_sensor.motion_hall", "binary_sensor.door_front"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

func TestQueryEntities_Current_GroupByIntegration(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{EntityID: "light.hue_1", State: "on", Attributes: map[string]any{"friendly_name": "Hue Light 1"}},
		{EntityID: "light.hue_2", State: "off", Attributes: map[string]any{"friendly_name": "Hue Light 2"}},
		{EntityID: "switch.tasmota_1", State: "on", Attributes: map[string]any{"friendly_name": "Tasmota Switch"}},
	}

	entityRegistry := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.hue_1", Platform: "hue"},
		{EntityID: "light.hue_2", Platform: "hue"},
		{EntityID: "switch.tasmota_1", Platform: "tasmota"},
	}

	tests := []handlerTestCase{
		{
			name: "group by integration",
			args: map[string]any{"mode": modeCurrent, "group_by": "integration", "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return entityRegistry, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Integration: hue", "Integration: tasmota", "light.hue_1", "light.hue_2", "switch.tasmota_1"},
		},
	}

	h := NewConsolidatedEntityQueryHandlers()
	runHandlerTestCases(t, tests, h.handleQueryEntities)
}

const argNotApplicableMarker = "not applicable to"

func TestQueryEntities_RejectsArgsOfOtherModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		wantContains []string
	}{
		{
			name:         "current rejects entity_id and points to get_state",
			args:         map[string]any{"mode": modeCurrent, "entity_id": "light.a"},
			wantContains: []string{"entity_id (only for mode=history)", "use get_state", "mode=history"},
		},
		{
			name:         "current rejects statistic_ids",
			args:         map[string]any{"mode": modeCurrent, "statistic_ids": []any{"sensor.x"}},
			wantContains: []string{"statistic_ids (only for mode=statistics)"},
		},
		{
			name:         "current rejects hours",
			args:         map[string]any{"mode": modeCurrent, "hours": float64(3)},
			wantContains: []string{"hours (only for mode=history)"},
		},
		{
			name:         "current rejects stale_days",
			args:         map[string]any{"mode": modeCurrent, "stale_days": float64(3)},
			wantContains: []string{"stale_days (only for mode=health)"},
		},
		{
			name:         "history rejects domain",
			args:         map[string]any{"mode": modeHistory, "entity_id": "sensor.t", "domain": "light"},
			wantContains: []string{"domain (only for mode=current|health)"},
		},
		{
			name:         "history rejects cursor",
			args:         map[string]any{"mode": modeHistory, "entity_id": "sensor.t", "cursor": "abc"},
			wantContains: []string{"cursor (only for mode=current|statistics)"},
		},
		{
			name:         "history rejects offset",
			args:         map[string]any{"mode": modeHistory, "entity_id": "sensor.t", "offset": float64(5)},
			wantContains: []string{"offset (only for mode=current|statistics)"},
		},
		{
			name:         "statistics rejects state",
			args:         map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.x"}, "state": "on"},
			wantContains: []string{"state (only for mode=current|history)"},
		},
		{
			name:         "statistics rejects start_time and end_time together",
			args:         map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.x"}, "start_time": "2024-01-01T00:00:00Z", "end_time": "2024-01-02T00:00:00Z"},
			wantContains: []string{"end_time (only for mode=history)", "start_time (only for mode=history)"},
		},
		{
			name:         "domains rejects limit",
			args:         map[string]any{"mode": modeDomains, "limit": float64(5)},
			wantContains: []string{"limit (only for mode=current|history|statistics)"},
		},
		{
			name:         "presence rejects domain",
			args:         map[string]any{"mode": "presence", "domain": "person"},
			wantContains: []string{"domain (only for mode=current|health)"},
		},
		{
			name:         "health rejects state",
			args:         map[string]any{"mode": "health", "state": "on"},
			wantContains: []string{"state (only for mode=current|history)"},
		},
		{
			name:         "current rejects period and categories together",
			args:         map[string]any{"mode": modeCurrent, "period": "day", "categories": []any{"stale"}},
			wantContains: []string{"categories (only for mode=health)", "period (only for mode=statistics)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				GetStatesFn: func(context.Context) ([]homeassistant.Entity, error) {
					t.Error("GetStates must not be called when an argument is rejected")
					return nil, nil
				},
				GetHistoryFn: func(context.Context, []string, time.Time, time.Time, homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
					t.Error("GetHistory must not be called when an argument is rejected")
					return nil, nil
				},
				GetStatisticsFn: func(context.Context, []string, string) ([]homeassistant.StatisticsResult, error) {
					t.Error("GetStatistics must not be called when an argument is rejected")
					return nil, nil
				},
			}

			h := NewConsolidatedEntityQueryHandlers()
			result, err := h.handleQueryEntities(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleQueryEntities() error = %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result, got: %s", result.Content[0].Text)
			}
			assertContainsAll(t, result.Content[0].Text, append([]string{argNotApplicableMarker}, tt.wantContains...))
		})
	}
}

func TestQueryEntities_AcceptsArgsOfTheSelectedMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"history with state filter", map[string]any{"mode": modeHistory, "entity_id": "sensor.t", "state": "on", "limit": float64(3), "hours": float64(1)}},
		{"health with domain", map[string]any{"mode": "health", "domain": "light", "categories": []any{"stale"}, "stale_days": float64(7)}},
		{"current with format and verbose", map[string]any{"mode": modeCurrent, "format": "json", "verbose": true}},
		{"current with all current filters", map[string]any{"mode": modeCurrent, "domain": "light", "state": "on", "state_not": "off", "name_contains": "a", "device_class": "motion", "area_id": "x", "sort_by": "state", "group_by": "domain", "limit": float64(2), "offset": float64(0)}},
		{"statistics with period and limit", map[string]any{"mode": modeStatistics, "statistic_ids": []any{"sensor.x"}, "period": "day", "limit": float64(2)}},
		{"domains with format and verbose only", map[string]any{"mode": modeDomains, "format": "natural", "verbose": false}},
		{"placeholders count as absent", map[string]any{"mode": modeCurrent, "entity_id": "", "statistic_ids": []any{}, "hours": nil, "categories": nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewConsolidatedEntityQueryHandlers()
			result, err := h.handleQueryEntities(context.Background(), &UniversalMockClient{}, tt.args)
			if err != nil {
				t.Fatalf("handleQueryEntities() error = %v", err)
			}
			if len(result.Content) > 0 && strings.Contains(result.Content[0].Text, argNotApplicableMarker) {
				t.Errorf("a valid argument was rejected: %s", result.Content[0].Text)
			}
		})
	}
}

func TestQueryEntities_NonStringModeIsRejected(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedEntityQueryHandlers()
	for name, mode := range map[string]any{"number": float64(1), "bool": true, "object": map[string]any{"a": 1}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := h.handleQueryEntities(context.Background(), &UniversalMockClient{}, map[string]any{"mode": mode})
			if err != nil {
				t.Fatalf("handleQueryEntities() error = %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result, got: %s", result.Content[0].Text)
			}
			assertContainsAll(t, result.Content[0].Text, []string{"Invalid mode", "expected a string"})
		})
	}
}

func TestQueryEntities_EmptyModeStillRequired(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedEntityQueryHandlers()
	for name, mode := range map[string]any{"empty string": "", "nil": nil} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := h.handleQueryEntities(context.Background(), &UniversalMockClient{}, map[string]any{"mode": mode})
			if err != nil {
				t.Fatalf("handleQueryEntities() error = %v", err)
			}
			if !result.IsError || !strings.Contains(result.Content[0].Text, "mode parameter is required") {
				t.Errorf("expected 'mode parameter is required', got: %s", result.Content[0].Text)
			}
		})
	}
}

func TestQueryEntitiesProperties_DescribeAppliesToModes(t *testing.T) {
	t.Parallel()

	props := queryEntitiesProperties()
	want := map[string]string{
		"state":  "mode=current and mode=history",
		"domain": "mode=current and mode=health",
		"limit":  "mode=current/history/statistics",
		"cursor": "mode=current/statistics",
	}
	for name, fragment := range want {
		if !strings.Contains(props[name].Description, fragment) {
			t.Errorf("%s description %q does not mention %q", name, props[name].Description, fragment)
		}
	}
}

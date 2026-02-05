// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// =============================================================================
// Tool Schema Tests
// =============================================================================

func TestConsolidatedHelperHandlers_ManageHelperToolSchema(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedHelperHandlers()
	tool := h.manageHelperTool()

	if tool.Name != "manage_helper" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_helper")
	}

	if tool.Description == "" {
		t.Error("tool.Description should not be empty")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Check required parameters
	requiredMap := make(map[string]bool)
	for _, req := range tool.InputSchema.Required {
		requiredMap[req] = true
	}

	if !requiredMap["action"] {
		t.Error("action should be in required parameters")
	}

	// Check properties exist
	requiredProps := []string{"action", "type", "entity_id", "id", "name"}
	for _, prop := range requiredProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Property %q missing from schema.Properties", prop)
		}
	}
}

func TestConsolidatedHelperHandlers_HelperActionToolSchema(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedHelperHandlers()
	tool := h.helperActionTool()

	if tool.Name != "helper_action" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "helper_action")
	}

	if tool.Description == "" {
		t.Error("tool.Description should not be empty")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Check required parameters
	requiredMap := make(map[string]bool)
	for _, req := range tool.InputSchema.Required {
		requiredMap[req] = true
	}

	if !requiredMap["entity_id"] {
		t.Error("entity_id should be in required parameters")
	}
	if !requiredMap["action"] {
		t.Error("action should be in required parameters")
	}
}

// =============================================================================
// manage_helper - Create Tests
// =============================================================================

func TestManageHelper_CreateInputBoolean(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create input_boolean successfully",
			args: map[string]any{
				"action": "create",
				"type":   "input_boolean",
				"id":     "test_switch",
				"name":   "Test Switch",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(_ context.Context, helper homeassistant.HelperConfig) error {
					if helper.Platform != "input_boolean" {
						return fmt.Errorf("unexpected platform: %s", helper.Platform)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"input_boolean.test_switch", "created"},
		},
		{
			name: "create input_boolean with optional fields",
			args: map[string]any{
				"action":  "create",
				"type":    "input_boolean",
				"id":      "test_switch",
				"name":    "Test Switch",
				"icon":    "mdi:toggle-switch",
				"initial": true,
			},
			wantError:    false,
			wantContains: []string{"created"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateCounter(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create counter successfully",
			args: map[string]any{
				"action": "create",
				"type":   "counter",
				"id":     "test_counter",
				"name":   "Test Counter",
			},
			wantError:    false,
			wantContains: []string{"counter.test_counter", "created"},
		},
		{
			name: "create counter with min/max",
			args: map[string]any{
				"action":  "create",
				"type":    "counter",
				"id":      "bounded_counter",
				"name":    "Bounded Counter",
				"minimum": float64(0),
				"maximum": float64(100),
				"step":    float64(5),
			},
			wantError:    false,
			wantContains: []string{"created"},
		},
		{
			name: "create counter with invalid min/max",
			args: map[string]any{
				"action":  "create",
				"type":    "counter",
				"id":      "invalid_counter",
				"name":    "Invalid Counter",
				"minimum": float64(100),
				"maximum": float64(10),
			},
			wantError:    true,
			wantContains: []string{"min", "max"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateTimer(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create timer successfully",
			args: map[string]any{
				"action": "create",
				"type":   "timer",
				"id":     "test_timer",
				"name":   "Test Timer",
			},
			wantError:    false,
			wantContains: []string{"timer.test_timer", "created"},
		},
		{
			name: "create timer with duration",
			args: map[string]any{
				"action":   "create",
				"type":     "timer",
				"id":       "kitchen_timer",
				"name":     "Kitchen Timer",
				"duration": "00:05:00",
				"restore":  true,
			},
			wantError:    false,
			wantContains: []string{"created"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateInputSelect(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create input_select successfully",
			args: map[string]any{
				"action":  "create",
				"type":    "input_select",
				"id":      "test_dropdown",
				"name":    "Test Dropdown",
				"options": []any{"Option A", "Option B", "Option C"},
			},
			wantError:    false,
			wantContains: []string{"input_select.test_dropdown", "created"},
		},
		{
			name: "create input_select without options",
			args: map[string]any{
				"action": "create",
				"type":   "input_select",
				"id":     "no_options",
				"name":   "No Options",
			},
			wantError:    true,
			wantContains: []string{"options", "required"},
		},
		{
			name: "create input_select with empty options",
			args: map[string]any{
				"action":  "create",
				"type":    "input_select",
				"id":      "empty_options",
				"name":    "Empty Options",
				"options": []any{},
			},
			wantError:    true,
			wantContains: []string{"options"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateGroup(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create group successfully",
			args: map[string]any{
				"action":   "create",
				"type":     "group",
				"id":       "test_group",
				"name":     "Test Group",
				"entities": []any{"light.one", "light.two"},
			},
			wantError:    false,
			wantContains: []string{"group.test_group", "created"},
		},
		{
			name: "create group without entities",
			args: map[string]any{
				"action": "create",
				"type":   "group",
				"id":     "no_entities",
				"name":   "No Entities",
			},
			wantError:    true,
			wantContains: []string{"entities", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateSchedule(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create schedule successfully",
			args: map[string]any{
				"action": "create",
				"type":   "schedule",
				"id":     "work_hours",
				"name":   "Work Hours",
				"monday": []any{
					map[string]any{"from": "09:00:00", "to": "17:00:00"},
				},
			},
			wantError:    false,
			wantContains: []string{"schedule.work_hours", "created"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateTemplateSensor(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create template_sensor successfully",
			args: map[string]any{
				"action": "create",
				"type":   "template_sensor",
				"id":     "avg_temp",
				"name":   "Average Temperature",
				"state":  "{{ (states('sensor.temp1') | float + states('sensor.temp2') | float) / 2 }}",
			},
			wantError:    false,
			wantContains: []string{"sensor.avg_temp", "created"},
		},
		{
			name: "create template_sensor without state",
			args: map[string]any{
				"action": "create",
				"type":   "template_sensor",
				"id":     "no_state",
				"name":   "No State",
			},
			wantError:    true,
			wantContains: []string{"state", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateTemplateBinarySensor(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create template_binary_sensor successfully",
			args: map[string]any{
				"action": "create",
				"type":   "template_binary_sensor",
				"id":     "is_home",
				"name":   "Is Home",
				"state":  "{{ is_state('person.john', 'home') }}",
			},
			wantError:    false,
			wantContains: []string{"binary_sensor.is_home", "created"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateThreshold(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create threshold successfully",
			args: map[string]any{
				"action":    "create",
				"type":      "threshold",
				"id":        "temp_high",
				"name":      "Temperature High",
				"entity_id": "sensor.temperature",
				"upper":     float64(25),
			},
			wantError:    false,
			wantContains: []string{"binary_sensor.temp_high", "created"},
		},
		{
			name: "create threshold without entity_id",
			args: map[string]any{
				"action": "create",
				"type":   "threshold",
				"id":     "no_source",
				"name":   "No Source",
				"upper":  float64(25),
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateDerivative(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create derivative successfully",
			args: map[string]any{
				"action": "create",
				"type":   "derivative",
				"id":     "power_rate",
				"name":   "Power Rate",
				"source": "sensor.power",
			},
			wantError:    false,
			wantContains: []string{"sensor.power_rate", "created"},
		},
		{
			name: "create derivative without source",
			args: map[string]any{
				"action": "create",
				"type":   "derivative",
				"id":     "no_source",
				"name":   "No Source",
			},
			wantError:    true,
			wantContains: []string{"source", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

func TestManageHelper_CreateIntegral(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create integral successfully",
			args: map[string]any{
				"action": "create",
				"type":   "integral",
				"id":     "energy_total",
				"name":   "Energy Total",
				"source": "sensor.power",
			},
			wantError:    false,
			wantContains: []string{"sensor.energy_total", "created"},
		},
		{
			name: "create integral with method",
			args: map[string]any{
				"action": "create",
				"type":   "integral",
				"id":     "energy_trap",
				"name":   "Energy Trapezoidal",
				"source": "sensor.power",
				"method": "trapezoidal",
			},
			wantError:    false,
			wantContains: []string{"created"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// =============================================================================
// manage_helper - Validation Tests
// =============================================================================

func TestManageHelper_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name:         "missing action",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"action", "required"},
		},
		{
			name: "invalid action",
			args: map[string]any{
				"action": "invalid_action",
			},
			wantError:    true,
			wantContains: []string{"action"},
		},
		{
			name: "create without type",
			args: map[string]any{
				"action": "create",
				"id":     "test",
				"name":   "Test",
			},
			wantError:    true,
			wantContains: []string{"type", "required"},
		},
		{
			name: "create with invalid type",
			args: map[string]any{
				"action": "create",
				"type":   "invalid_type",
				"id":     "test",
				"name":   "Test",
			},
			wantError:    true,
			wantContains: []string{"type"},
		},
		{
			name: "create without id",
			args: map[string]any{
				"action": "create",
				"type":   "counter",
				"name":   "Test Counter",
			},
			wantError:    true,
			wantContains: []string{"id", "required"},
		},
		{
			name: "create without name",
			args: map[string]any{
				"action": "create",
				"type":   "counter",
				"id":     "test",
			},
			wantError:    true,
			wantContains: []string{"name", "required"},
		},
		{
			name: "delete without entity_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
		{
			name: "get_details without entity_id",
			args: map[string]any{
				"action": "get_details",
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// =============================================================================
// manage_helper - Delete Tests
// =============================================================================

func TestManageHelper_Delete(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "delete counter successfully",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "counter.test_counter",
			},
			wantError:    false,
			wantContains: []string{"deleted"},
		},
		{
			name: "delete timer successfully",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "timer.test_timer",
			},
			wantError:    false,
			wantContains: []string{"deleted"},
		},
		{
			name: "delete template sensor",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "sensor.my_template",
			},
			wantError:    false,
			wantContains: []string{"deleted"},
		},
		{
			name: "delete with invalid entity_id format",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "invalid",
			},
			wantError:    true,
			wantContains: []string{"entity_id"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// =============================================================================
// manage_helper - GetDetails Tests
// =============================================================================

func TestManageHelper_GetDetails(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get_details for schedule with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "schedule.work_hours",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "on",
						Attributes: map[string]any{
							"friendly_name": "Work Hours",
						},
					}, nil
				}
				m.GetScheduleConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return map[string]any{
						"monday": []any{
							map[string]any{"from": "09:00:00", "to": "17:00:00"},
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"schedule.work_hours"},
		},
		{
			name: "get_details for schedule with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "schedule.work_hours",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "on",
						Attributes: map[string]any{
							"friendly_name": "Work Hours",
						},
					}, nil
				}
				m.GetScheduleConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return map[string]any{
						"monday": []any{
							map[string]any{"from": "09:00:00", "to": "17:00:00"},
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Work Hours", "on", "Mon"},
		},
		{
			name: "get_details for non-schedule entity",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "counter.test_counter",
			},
			wantError:    true,
			wantContains: []string{"schedule"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// =============================================================================
// manage_helper - List Tests
// =============================================================================

func TestManageHelper_List(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "list helpers with natural format (default)",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListHelpersFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "counter.visitors", State: "42", Attributes: map[string]any{"friendly_name": "Visitor Counter"}},
						{EntityID: "input_boolean.vacation", State: "off", Attributes: map[string]any{"friendly_name": "Vacation Mode"}},
						{EntityID: "timer.pomodoro", State: "idle", Attributes: map[string]any{"friendly_name": "Pomodoro Timer"}},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"3 helpers", "Counters", "Input Booleans", "Timers"},
		},
		{
			name: "list helpers with json format",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListHelpersFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "counter.visitors", State: "42", Attributes: map[string]any{"friendly_name": "Visitor Counter"}},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"counter.visitors", "entity_id", "state"},
		},
		{
			name: "list helpers empty",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListHelpersFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No helpers found"},
		},
		{
			name: "list helpers with verbose",
			args: map[string]any{
				"action":  "list",
				"verbose": true,
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListHelpersFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "counter.visitors", State: "42", Attributes: map[string]any{
							"friendly_name": "Visitor Counter",
							"minimum":       float64(0),
							"maximum":       float64(100),
						}},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Counters", "Visitor Counter"},
		},
		{
			name: "list helpers error",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListHelpersFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "listing"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// =============================================================================
// helper_action - Toggle Tests
// =============================================================================

func TestHelperAction_Toggle(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "toggle input_boolean successfully",
			args: map[string]any{
				"entity_id": "input_boolean.test_switch",
				"action":    "toggle",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CallServiceFn = func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
					if domain != "input_boolean" || service != "toggle" {
						return nil, fmt.Errorf("unexpected call: %s.%s", domain, service)
					}
					return nil, nil
				}
			},
			wantError:    false,
			wantContains: []string{"toggled"},
		},
		{
			name: "toggle non-input_boolean fails",
			args: map[string]any{
				"entity_id": "counter.test",
				"action":    "toggle",
			},
			wantError:    true,
			wantContains: []string{"toggle", "input_boolean"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Set Tests
// =============================================================================

func TestHelperAction_Set(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "set input_number value",
			args: map[string]any{
				"entity_id": "input_number.temperature",
				"action":    "set",
				"value":     float64(25.5),
			},
			setupMock: func(m *UniversalMockClient) {
				m.CallServiceFn = func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
					if domain != "input_number" || service != "set_value" {
						return nil, fmt.Errorf("unexpected call: %s.%s", domain, service)
					}
					if data["value"] != float64(25.5) {
						return nil, fmt.Errorf("unexpected value: %v", data["value"])
					}
					return nil, nil
				}
			},
			wantError:    false,
			wantContains: []string{"set"},
		},
		{
			name: "set input_text value",
			args: map[string]any{
				"entity_id": "input_text.message",
				"action":    "set",
				"value":     "Hello World",
			},
			wantError:    false,
			wantContains: []string{"set"},
		},
		{
			name: "set input_datetime value",
			args: map[string]any{
				"entity_id": "input_datetime.alarm",
				"action":    "set",
				"value":     "2024-01-15 08:00:00",
			},
			wantError:    false,
			wantContains: []string{"set"},
		},
		{
			name: "set counter value",
			args: map[string]any{
				"entity_id": "counter.visitors",
				"action":    "set",
				"value":     float64(42),
			},
			wantError:    false,
			wantContains: []string{"set"},
		},
		{
			name: "set without value",
			args: map[string]any{
				"entity_id": "input_number.test",
				"action":    "set",
			},
			wantError:    true,
			wantContains: []string{"value", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Counter Actions
// =============================================================================

func TestHelperAction_CounterActions(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "increment counter",
			args: map[string]any{
				"entity_id": "counter.visitors",
				"action":    "increment",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CallServiceFn = func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
					if domain != "counter" || service != "increment" {
						return nil, fmt.Errorf("unexpected call: %s.%s", domain, service)
					}
					return nil, nil
				}
			},
			wantError:    false,
			wantContains: []string{"incremented"},
		},
		{
			name: "decrement counter",
			args: map[string]any{
				"entity_id": "counter.visitors",
				"action":    "decrement",
			},
			wantError:    false,
			wantContains: []string{"decremented"},
		},
		{
			name: "reset counter",
			args: map[string]any{
				"entity_id": "counter.visitors",
				"action":    "reset",
			},
			wantError:    false,
			wantContains: []string{"reset"},
		},
		{
			name: "increment non-counter fails",
			args: map[string]any{
				"entity_id": "input_number.test",
				"action":    "increment",
			},
			wantError:    true,
			wantContains: []string{"counter"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Timer Actions
// =============================================================================

func TestHelperAction_TimerActions(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "start timer",
			args: map[string]any{
				"entity_id": "timer.kitchen",
				"action":    "start",
			},
			wantError:    false,
			wantContains: []string{"started"},
		},
		{
			name: "start timer with duration",
			args: map[string]any{
				"entity_id": "timer.kitchen",
				"action":    "start",
				"duration":  "00:05:00",
			},
			wantError:    false,
			wantContains: []string{"started"},
		},
		{
			name: "pause timer",
			args: map[string]any{
				"entity_id": "timer.kitchen",
				"action":    "pause",
			},
			wantError:    false,
			wantContains: []string{"paused"},
		},
		{
			name: "cancel timer",
			args: map[string]any{
				"entity_id": "timer.kitchen",
				"action":    "cancel",
			},
			wantError:    false,
			wantContains: []string{"canceled"},
		},
		{
			name: "finish timer",
			args: map[string]any{
				"entity_id": "timer.kitchen",
				"action":    "finish",
			},
			wantError:    false,
			wantContains: []string{"finished"},
		},
		{
			name: "change timer duration",
			args: map[string]any{
				"entity_id": "timer.kitchen",
				"action":    "change",
				"duration":  "00:01:00",
			},
			wantError:    false,
			wantContains: []string{"changed"},
		},
		{
			name: "change timer without duration",
			args: map[string]any{
				"entity_id": "timer.kitchen",
				"action":    "change",
			},
			wantError:    true,
			wantContains: []string{"duration", "required"},
		},
		{
			name: "start non-timer fails",
			args: map[string]any{
				"entity_id": "counter.test",
				"action":    "start",
			},
			wantError:    true,
			wantContains: []string{"timer"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Input Select Actions
// =============================================================================

func TestHelperAction_InputSelectActions(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "select option",
			args: map[string]any{
				"entity_id": "input_select.mode",
				"action":    "select",
				"option":    "Option A",
			},
			wantError:    false,
			wantContains: []string{"selected"},
		},
		{
			name: "select without option",
			args: map[string]any{
				"entity_id": "input_select.mode",
				"action":    "select",
			},
			wantError:    true,
			wantContains: []string{"option", "required"},
		},
		{
			name: "set_options",
			args: map[string]any{
				"entity_id": "input_select.mode",
				"action":    "set_options",
				"options":   []any{"New A", "New B"},
			},
			wantError:    false,
			wantContains: []string{"Options", "updated"},
		},
		{
			name: "set_options without options",
			args: map[string]any{
				"entity_id": "input_select.mode",
				"action":    "set_options",
			},
			wantError:    true,
			wantContains: []string{"options", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Input Button
// =============================================================================

func TestHelperAction_InputButton(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "press input_button",
			args: map[string]any{
				"entity_id": "input_button.doorbell",
				"action":    "press",
			},
			wantError:    false,
			wantContains: []string{"pressed"},
		},
		{
			name: "press non-input_button fails",
			args: map[string]any{
				"entity_id": "counter.test",
				"action":    "press",
			},
			wantError:    true,
			wantContains: []string{"input_button"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Group Actions
// =============================================================================

func TestHelperAction_GroupActions(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "add_entities to group",
			args: map[string]any{
				"entity_id":    "group.lights",
				"action":       "add_entities",
				"add_entities": []any{"light.new_one", "light.new_two"},
			},
			wantError:    false,
			wantContains: []string{"added"},
		},
		{
			name: "remove_entities from group",
			args: map[string]any{
				"entity_id":       "group.lights",
				"action":          "remove_entities",
				"remove_entities": []any{"light.old"},
			},
			wantError:    false,
			wantContains: []string{"removed"},
		},
		{
			name: "reload group",
			args: map[string]any{
				"entity_id": "group.lights",
				"action":    "reload",
			},
			wantError:    false,
			wantContains: []string{"reload"},
		},
		{
			name: "add_entities without entities list",
			args: map[string]any{
				"entity_id": "group.lights",
				"action":    "add_entities",
			},
			wantError:    true,
			wantContains: []string{"add_entities", "required"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Schedule Actions
// =============================================================================

func TestHelperAction_ScheduleActions(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "reload schedule",
			args: map[string]any{
				"entity_id": "schedule.work_hours",
				"action":    "reload",
			},
			wantError:    false,
			wantContains: []string{"reload"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Integral Reset
// =============================================================================

func TestHelperAction_IntegralReset(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "reset integral sensor",
			args: map[string]any{
				"entity_id": "sensor.energy_total",
				"action":    "reset",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CallServiceFn = func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
					if domain != "integration" || service != "reset" {
						return nil, fmt.Errorf("unexpected call: %s.%s", domain, service)
					}
					return nil, nil
				}
			},
			wantError:    false,
			wantContains: []string{"reset"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - Validation Tests
// =============================================================================

func TestHelperAction_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name:         "missing entity_id",
			args:         map[string]any{"action": "toggle"},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
		{
			name:         "missing action",
			args:         map[string]any{"entity_id": "input_boolean.test"},
			wantError:    true,
			wantContains: []string{"action", "required"},
		},
		{
			name: "invalid action",
			args: map[string]any{
				"entity_id": "input_boolean.test",
				"action":    "invalid_action",
			},
			wantError:    true,
			wantContains: []string{"action"},
		},
		{
			name: "action not supported for type",
			args: map[string]any{
				"entity_id": "input_boolean.test",
				"action":    "increment",
			},
			wantError:    true,
			wantContains: []string{"increment"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// helper_action - API Error Handling
// =============================================================================

func TestHelperAction_APIErrors(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "service call error",
			args: map[string]any{
				"entity_id": "input_boolean.test",
				"action":    "toggle",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CallServiceFn = func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, fmt.Errorf("connection timeout")
				}
			},
			wantError:    true,
			wantContains: []string{"Error"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleHelperAction)
}

// =============================================================================
// manage_helper - API Error Handling
// =============================================================================

func TestManageHelper_APIErrors(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create helper error",
			args: map[string]any{
				"action": "create",
				"type":   "counter",
				"id":     "test",
				"name":   "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(_ context.Context, _ homeassistant.HelperConfig) error {
					return fmt.Errorf("helper already exists")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "creating"},
		},
		{
			name: "delete helper error",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "counter.test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.DeleteHelperFn = func(_ context.Context, _ string) error {
					return fmt.Errorf("helper not found")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "deleting"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// =============================================================================
// Helper Type Metadata Tests
// =============================================================================

func TestHelperTypeMetadata(t *testing.T) {
	t.Parallel()

	expectedTypes := []string{
		"input_boolean", "input_number", "input_text", "input_select",
		"input_datetime", "input_button", "counter", "timer", "schedule",
		"group", "template_sensor", "template_binary_sensor", "threshold",
		"derivative", "integral",
	}

	for _, helperType := range expectedTypes {
		t.Run(helperType, func(t *testing.T) {
			t.Parallel()
			meta, ok := helperTypes[helperType]
			if !ok {
				t.Errorf("helperTypes missing entry for %q", helperType)
				return
			}
			if meta.platform == "" {
				t.Errorf("helperTypes[%q].platform is empty", helperType)
			}
		})
	}
}

// =============================================================================
// Registration Tests
// =============================================================================

func TestConsolidatedHelperHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedHelperHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify manage_helper is registered
	tools := registry.ListTools()
	foundManageHelper := false
	foundHelperAction := false

	for _, tool := range tools {
		if tool.Name == "manage_helper" {
			foundManageHelper = true
		}
		if tool.Name == "helper_action" {
			foundHelperAction = true
		}
	}

	if !foundManageHelper {
		t.Error("manage_helper tool not registered")
	}
	if !foundHelperAction {
		t.Error("helper_action tool not registered")
	}
}

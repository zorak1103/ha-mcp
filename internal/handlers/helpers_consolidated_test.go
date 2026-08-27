// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestManageHelper_CreateInputNumber(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create input_number without min/max",
			args: map[string]any{
				"action": "create",
				"type":   "input_number",
				"id":     "test_number",
				"name":   "Test Number",
			},
			wantError:    true,
			wantContains: []string{"min must be a number"},
		},
		{
			name: "create input_number with min/max",
			args: map[string]any{
				"action": "create",
				"type":   "input_number",
				"id":     "test_number",
				"name":   "Test Number",
				"min":    float64(0),
				"max":    float64(100),
			},
			wantError:    false,
			wantContains: []string{"input_number.test_number", "created"},
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
			name: "create light group successfully",
			args: map[string]any{
				"action":   "create",
				"type":     "group",
				"id":       "test_group",
				"name":     "Test Group",
				"entities": []any{"light.one", "light.two"},
			},
			wantError:    false,
			wantContains: []string{"light.test_group", "created"},
		},
		{
			name: "create sensor group successfully",
			args: map[string]any{
				"action":   "create",
				"type":     "group",
				"id":       "sensor_group",
				"name":     "Sensor Group",
				"entities": []any{"sensor.temp1", "sensor.temp2"},
			},
			wantError:    false,
			wantContains: []string{"sensor.sensor_group", "created"},
		},
		{
			name: "create binary_sensor group successfully",
			args: map[string]any{
				"action":   "create",
				"type":     "group",
				"id":       "motion_group",
				"name":     "Motion Group",
				"entities": []any{"binary_sensor.motion1", "binary_sensor.motion2"},
			},
			wantError:    false,
			wantContains: []string{"binary_sensor.motion_group", "created"},
		},
		{
			name: "create group with explicit group_type override",
			args: map[string]any{
				"action":     "create",
				"type":       "group",
				"id":         "override_group",
				"name":       "Override Group",
				"entities":   []any{"light.one"},
				"group_type": "sensor",
			},
			wantError:    false,
			wantContains: []string{"created"},
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
			wantContains: []string{"sensor.average_temperature", "created"},
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
			wantContains: []string{"binary_sensor.temperature_high", "created"},
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

// TestManageHelper_Create_SourceDomainMismatch asserts that manage_helper
// create fails fast with an actionable message when the source entity
// referenced by a domain-restricted Config Entry helper (utility_meter,
// statistics, trend, filter, generic_thermostat, switch_as_x,
// generic_hygrostat) has the wrong domain, instead of surfacing HA's opaque
// config-entry-flow validation error. Types with no source-domain
// constraint must remain unaffected.
func TestManageHelper_Create_SourceDomainMismatch(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "utility_meter with input_number source rejected",
			args: map[string]any{
				"action": "create",
				"type":   "utility_meter",
				"id":     "test_meter",
				"name":   "Test Utility Meter",
				"source": "input_number.x",
			},
			wantError:    true,
			wantContains: []string{"requires a sensor", "template_sensor"},
		},
		{
			name: "switch_as_x with input_boolean entity_id rejected",
			args: map[string]any{
				"action":        "create",
				"type":          "switch_as_x",
				"id":            "test_switch",
				"name":          "Test Switch",
				"entity_id":     "input_boolean.x",
				"target_domain": "light",
			},
			wantError:       true,
			wantContains:    []string{"requires a switch", "Template → Switch"},
			wantNotContains: []string{`manage_helper(action="create", type="template_switch"`},
		},
		{
			name: "utility_meter with sensor source proceeds unaffected",
			args: map[string]any{
				"action": "create",
				"type":   "utility_meter",
				"id":     "test_meter",
				"name":   "Test Utility Meter",
				"source": "sensor.x",
			},
			wantError:    false,
			wantContains: []string{"created", "sensor.test_utility_meter"},
		},
		{
			name: "malformed source entity_id omits the wrapper recipe",
			args: map[string]any{
				"action": "create",
				"type":   "utility_meter",
				"id":     "test_meter",
				"name":   "Test Utility Meter",
				"source": "input_number.x') }}{{ 7*7 }}",
			},
			wantError:       true,
			wantContains:    []string{"requires a sensor"},
			wantNotContains: []string{"Wrap it first", "{{ states("},
		},
		{
			name: "unconstrained type with any entity_id domain unaffected",
			args: map[string]any{
				"action":    "create",
				"type":      "threshold",
				"id":        "temp_high",
				"name":      "Temperature High",
				"entity_id": "input_boolean.x",
				"upper":     float64(25),
			},
			wantError:    false,
			wantContains: []string{"binary_sensor.temperature_high", "created"},
		},
		{
			name: "generic_thermostat with sensor target lacking temperature device_class rejected",
			args: map[string]any{
				"action":                  "create",
				"type":                    "generic_thermostat",
				"id":                      "test_thermostat",
				"name":                    "Test Thermostat",
				"heater_entity_id":        "switch.heater",
				"target_sensor_entity_id": "sensor.humidity_sensor",
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
			wantContains: []string{"requires a sensor", "device_class", "temperature", "Wrap it first"},
		},
		{
			name: "generic_thermostat with sensor target carrying temperature device_class proceeds",
			args: map[string]any{
				"action":                  "create",
				"type":                    "generic_thermostat",
				"id":                      "test_thermostat",
				"name":                    "Test Thermostat",
				"heater_entity_id":        "switch.heater",
				"target_sensor_entity_id": "sensor.temp_sensor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "temperature"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created"},
		},
		{
			name: "generic_thermostat with fan heater unaffected by widened domain",
			args: map[string]any{
				"action":                  "create",
				"type":                    "generic_thermostat",
				"id":                      "test_thermostat",
				"name":                    "Test Thermostat",
				"heater_entity_id":        "fan.heater",
				"target_sensor_entity_id": "sensor.temp_sensor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "temperature"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// TestManageHelper_Update_SourceDomainMismatch verifies that update, not just
// create, rejects a source-entity domain mismatch for domain-constrained
// Config Entry helpers. On update, ParseHelperEntityID only recovers the
// entity DOMAIN (e.g. "sensor" for a statistics helper) - not the helper
// type - so the check must resolve the real integration platform via the
// entity registry's Platform field instead of a helperTypes map lookup.
//
// Only utility_meter/generic_thermostat/generic_hygrostat are exercised
// here, not statistics/trend/filter/switch_as_x: those four types constrain
// a field literally named "entity_id", which on update IS the tool's own
// "which helper are we updating" identifier (handleUpdate reads args
// ["entity_id"] for that) and is never forwarded to HA as a config value
// (see CLAUDE.md's buildConfigEntryUpdateConfig gotcha) - so there is no
// caller-suppliable value to validate for those four on update.
func TestManageHelper_Update_SourceDomainMismatch(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "utility_meter update with input_number source rejected",
			args: map[string]any{
				"action":    "update",
				"entity_id": "sensor.my_meter",
				"source":    "input_number.x",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.my_meter", Platform: "utility_meter", ConfigEntryID: "config123"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"requires a sensor"},
		},
		{
			name: "generic_thermostat update with input_boolean heater_entity_id rejected",
			args: map[string]any{
				"action":           "update",
				"entity_id":        "climate.my_thermostat",
				"heater_entity_id": "input_boolean.x",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "climate.my_thermostat", Platform: "generic_thermostat", ConfigEntryID: "config123"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"requires a switch"},
		},
		{
			name: "utility_meter update with sensor source proceeds unaffected",
			args: map[string]any{
				"action":    "update",
				"entity_id": "sensor.my_meter",
				"source":    "sensor.x",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.my_meter", Platform: "utility_meter", ConfigEntryID: "config123"},
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated"},
		},
		{
			name: "registry lookup failure degrades to unchecked update",
			args: map[string]any{
				"action":    "update",
				"entity_id": "sensor.my_meter",
				"source":    "input_number.x",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return nil, errors.New("registry unavailable")
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated"},
		},
		{
			name: "generic_thermostat update with wrong device_class target_sensor_entity_id rejected",
			args: map[string]any{
				"action":                  "update",
				"entity_id":               "climate.my_thermostat",
				"target_sensor_entity_id": "sensor.humidity_sensor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "climate.my_thermostat", Platform: "generic_thermostat", ConfigEntryID: "config123"},
					}, nil
				}
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "humidity"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"requires a sensor", "device_class", "temperature"},
		},
		{
			name: "generic_thermostat update with correct device_class target_sensor_entity_id proceeds",
			args: map[string]any{
				"action":                  "update",
				"entity_id":               "climate.my_thermostat",
				"target_sensor_entity_id": "sensor.temp_sensor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "climate.my_thermostat", Platform: "generic_thermostat", ConfigEntryID: "config123"},
					}, nil
				}
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   entityID,
						Attributes: map[string]any{"device_class": "temperature"},
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// TestCheckUpdateSourceEntityDomain_SkipsRegistryFetchWithoutConstrainedField
// guards the W3 perf fix: checkUpdateSourceEntityDomain must not fetch the
// entity registry at all when the update args touch none of the fields any
// source constraint could apply to - checkSourceEntityDomain would return
// nil anyway in that case (every constraint `continue`s past an absent
// field), so paying for a registry fetch to reach the same nil answer is
// pure waste, doubled up with the second, uncached fetch
// HybridClient.UpdateHelper performs right after via c.ws.GetEntityRegistry.
func TestCheckUpdateSourceEntityDomain_SkipsRegistryFetchWithoutConstrainedField(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			t.Fatal("GetEntityRegistry should not be called when args has no source-constrained field")
			return nil, nil
		},
	}

	err := checkUpdateSourceEntityDomain(context.Background(), client, "input_number.x", map[string]any{
		"name": "New Name",
		"min":  0.0,
		"max":  100.0,
	})
	if err != nil {
		t.Errorf("checkUpdateSourceEntityDomain() = %v, want nil", err)
	}
}

// TestCheckUpdateSourceEntityDomain_FetchesRegistryWithConstrainedField is
// the mirror of the skip test above: a constrained field present in args
// must still trigger the registry fetch and the real validation.
func TestCheckUpdateSourceEntityDomain_FetchesRegistryWithConstrainedField(t *testing.T) {
	t.Parallel()

	fetched := false
	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			fetched = true
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "sensor.my_meter", Platform: "utility_meter", ConfigEntryID: "config123"},
			}, nil
		},
	}

	err := checkUpdateSourceEntityDomain(context.Background(), client, "sensor.my_meter", map[string]any{
		"source": "input_number.x",
	})
	if !fetched {
		t.Error("GetEntityRegistry should be called when args has a source-constrained field")
	}
	if err == nil {
		t.Error("checkUpdateSourceEntityDomain() = nil, want a domain-mismatch error")
	}
}

// TestBuildConfigEntryUpdateConfig_MinMaxTypeGatedByPlatform is the
// regression test for W1: addExtendedConfigEntryFields must not forward
// min_max_type into config["type"] unless the caller has been verified to
// target an actual min_max helper. It's a one-size-fits-all builder shared
// by every config-entry platform (template, threshold, sensor-domain group,
// statistics, ...), so an unconditional write here would let min_max_type
// leak into e.g. a sensor group's update and silently change its
// aggregation type - HA's group CONF_TYPE enum is a superset of min_max's,
// so the write would validate and succeed with no error anywhere.
func TestBuildConfigEntryUpdateConfig_MinMaxTypeGatedByPlatform(t *testing.T) {
	t.Parallel()

	args := map[string]any{"min_max_type": "max"}

	notMinMax, err := buildConfigEntryUpdateConfig("sensor", "group", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := notMinMax["type"]; present {
		t.Errorf(`buildConfigEntryUpdateConfig("sensor", "group", ...) wrote config["type"] = %v, want absent - min_max_type must not leak into a non-min_max platform's update`, notMinMax["type"])
	}

	isMinMax, err := buildConfigEntryUpdateConfig("sensor", platformMinMax, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := isMinMax["type"]; got != "max" {
		t.Errorf(`buildConfigEntryUpdateConfig("sensor", %q, ...) config["type"] = %v, want "max"`, platformMinMax, got)
	}
}

// TestManageHelper_Update_MinMaxTypeRejectedForNonMinMaxHelper covers the
// handler-level half of W1: min_max_type on an update targeting a
// non-min_max config-entry helper (here, a sensor-domain group) must be
// rejected with an actionable error, not silently applied.
func TestManageHelper_Update_MinMaxTypeRejectedForNonMinMaxHelper(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "sensor.my_group", Platform: "group", ConfigEntryID: "config123"},
			}, nil
		},
		UpdateHelperFn: func(context.Context, string, homeassistant.HelperConfig) error {
			t.Fatal("UpdateHelper should not be called when min_max_type targets a non-min_max helper")
			return nil
		},
	}

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(context.Background(), client, map[string]any{
		"action":       "update",
		"entity_id":    "sensor.my_group",
		"min_max_type": "max",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for min_max_type on a non-min_max helper")
	}
	if msg := result.Content[0].Text; !strings.Contains(msg, "min_max_type") || !strings.Contains(msg, "group") {
		t.Errorf("error message = %q, want it to name both min_max_type and the actual helper type (group)", msg)
	}
}

// TestManageHelper_Update_MinMaxTypeHardFailsOnRegistryFetchError guards
// against the degraded-skip convention used elsewhere (e.g.
// checkUpdateSourceEntityDomain): unlike a validation, silently dropping
// min_max_type on a registry fetch failure would report the update as
// successful while discarding the field the caller asked to change.
func TestManageHelper_Update_MinMaxTypeHardFailsOnRegistryFetchError(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return nil, fmt.Errorf("registry unavailable")
		},
		UpdateHelperFn: func(context.Context, string, homeassistant.HelperConfig) error {
			t.Fatal("UpdateHelper should not be called when the registry fetch needed to verify min_max_type fails")
			return nil
		},
	}

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(context.Background(), client, map[string]any{
		"action":       "update",
		"entity_id":    "sensor.my_min_max",
		"min_max_type": "max",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result when the registry fetch fails")
	}
}

// TestUpdatableSourceEntities_ExcludesEntityIDField guards the collision
// checkUpdateSourceEntityDomain must avoid: a sourceEntityConstraint whose
// field is "entity_id" is, on update, the tool's own "which helper is being
// updated" identifier (handleUpdate reads args["entity_id"] for that) - not
// a caller-suppliable source value - so it must never be validated against
// on the update path. Exercised directly (rather than through the handler)
// because switch_as_x's real target domains (cover/fan/light/lock/siren/
// valve) aren't in HelperPlatforms, so a handler-level test can't reach this
// case via ParseHelperEntityID at all.
func TestUpdatableSourceEntities_ExcludesEntityIDField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		constraint []sourceEntityConstraint
		wantEmpty  bool
	}{
		{
			name:       "entity_id-only constraint (switch_as_x, statistics, trend, filter) filtered to empty",
			constraint: helperTypes["switch_as_x"].sourceEntities,
			wantEmpty:  true,
		},
		{
			name:       "source field (utility_meter) survives unfiltered",
			constraint: helperTypes["utility_meter"].sourceEntities,
			wantEmpty:  false,
		},
		{
			name:       "mixed constraints (generic_thermostat) keep the non-entity_id field only",
			constraint: helperTypes["generic_thermostat"].sourceEntities,
			wantEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := updatableSourceEntities(tt.constraint)
			if tt.wantEmpty && len(got) != 0 {
				t.Errorf("updatableSourceEntities(%+v) = %+v, want empty", tt.constraint, got)
			}
			if !tt.wantEmpty && len(got) == 0 {
				t.Errorf("updatableSourceEntities(%+v) = empty, want at least one constraint", tt.constraint)
			}
			for _, c := range got {
				if c.field == attrEntityID {
					t.Errorf("updatableSourceEntities leaked an %q-field constraint: %+v", attrEntityID, c)
				}
			}
		})
	}
}

// =============================================================================
// manage_helper - ID vs Name Entity Slug Tests
// =============================================================================

func TestHelperCreate_IDControlsEntitySlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		args             map[string]any
		setupMock        func(m *UniversalMockClient)
		wantEntityID     string
		wantUpdateCalled bool
		wantUpdateName   string
		wantError        bool
		wantContains     []string
	}{
		{
			name: "WebSocket helper - different id and name - uses id for entity, updates name",
			args: map[string]any{
				"action": "create",
				"type":   "input_boolean",
				"id":     "mcp_test_bug1",
				"name":   "MCP Test Bug 1 Check",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(_ context.Context, helper homeassistant.HelperConfig) error {
					// Verify create uses id as name (to control slug)
					if name, ok := helper.Config["name"].(string); !ok || name != "mcp_test_bug1" {
						return fmt.Errorf("expected name=mcp_test_bug1, got %v", helper.Config["name"])
					}
					return nil
				}
				m.UpdateHelperFn = func(_ context.Context, helperID string, config homeassistant.HelperConfig) error {
					// Verify update sets the friendly name
					if helperID != "mcp_test_bug1" {
						return fmt.Errorf("expected helperID=mcp_test_bug1, got %s", helperID)
					}
					if name, ok := config.Config["name"].(string); !ok || name != "MCP Test Bug 1 Check" {
						return fmt.Errorf("expected name=MCP Test Bug 1 Check, got %v", config.Config["name"])
					}
					return nil
				}
			},
			wantEntityID:     "input_boolean.mcp_test_bug1",
			wantUpdateCalled: true,
			wantUpdateName:   "MCP Test Bug 1 Check",
			wantError:        false,
			wantContains:     []string{"input_boolean.mcp_test_bug1", "created"},
		},
		{
			name: "WebSocket helper - id matches slugified name - no update needed",
			args: map[string]any{
				"action": "create",
				"type":   "counter",
				"id":     "test_counter",
				"name":   "Test Counter",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(_ context.Context, helper homeassistant.HelperConfig) error {
					// Since slugify(id) == slugify(name), name should be used directly
					if name, ok := helper.Config["name"].(string); !ok || name != "Test Counter" {
						return fmt.Errorf("expected name=Test Counter, got %v", helper.Config["name"])
					}
					return nil
				}
				m.UpdateHelperFn = func(_ context.Context, _ string, _ homeassistant.HelperConfig) error {
					return fmt.Errorf("UpdateHelper should not be called when id matches slugified name")
				}
			},
			wantEntityID:     "counter.test_counter",
			wantUpdateCalled: false,
			wantError:        false,
			wantContains:     []string{"counter.test_counter", "created"},
		},
		{
			name: "Config Entry helper (threshold) - always uses name for entity ID",
			args: map[string]any{
				"action":    "create",
				"type":      "threshold",
				"id":        "temp_high",
				"name":      "Temperature High",
				"entity_id": "sensor.temperature",
				"upper":     float64(25),
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateHelperFn = func(_ context.Context, helper homeassistant.HelperConfig) error {
					// Config Entry helpers use name, not id
					if name, ok := helper.Config["name"].(string); !ok || name != "Temperature High" {
						return fmt.Errorf("expected name=Temperature High, got %v", helper.Config["name"])
					}
					return nil
				}
				m.UpdateHelperFn = func(_ context.Context, _ string, _ homeassistant.HelperConfig) error {
					return fmt.Errorf("UpdateHelper should not be called for config entry helpers")
				}
			},
			wantEntityID:     "binary_sensor.temperature_high",
			wantUpdateCalled: false,
			wantError:        false,
			wantContains:     []string{"binary_sensor.temperature_high", "created"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup mock client
			mockClient := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			// Execute handler
			h := NewConsolidatedHelperHandlers()
			result, err := h.handleManageHelper(context.Background(), mockClient, tt.args)

			// Check error
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check result contains expected strings
			if result != nil && len(result.Content) > 0 {
				text := result.Content[0].Text
				assertContainsAll(t, text, tt.wantContains)
			}
		})
	}
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
		{
			name: "create input_select with missing options",
			args: map[string]any{
				"action": "create",
				"type":   "input_select",
				"id":     "test",
				"name":   "Test Select",
			},
			wantError:    true,
			wantContains: []string{"options", "required", "array"},
		},
		{
			name: "create input_select with empty options",
			args: map[string]any{
				"action":  "create",
				"type":    "input_select",
				"id":      "test",
				"name":    "Test Select",
				"options": []any{},
			},
			wantError:    true,
			wantContains: []string{"options", "non-empty"},
		},
		{
			name: "create group with missing entities",
			args: map[string]any{
				"action": "create",
				"type":   "group",
				"id":     "test",
				"name":   "Test Group",
			},
			wantError:    true,
			wantContains: []string{"entities", "required", "array"},
		},
		{
			name: "create group with empty entities",
			args: map[string]any{
				"action":   "create",
				"type":     "group",
				"id":       "test",
				"name":     "Test Group",
				"entities": []any{},
			},
			wantError:    true,
			wantContains: []string{"entities", "non-empty"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// =============================================================================
// manage_helper - Update Tests
// =============================================================================

func TestManageHelper_Update(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update input_number successfully",
			args: map[string]any{
				"action":    "update",
				"entity_id": "input_number.test_number",
				"min":       float64(10),
				"max":       float64(50),
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateHelperFn = func(_ context.Context, helperID string, config homeassistant.HelperConfig) error {
					// The client must receive the FULL entity_id, not the bare object_id -
					// HybridClient.UpdateHelper matches config-entry helpers against the
					// registry's full entity_id, so a stripped id silently falls through
					// to the WS unknown_command path.
					if helperID != "input_number.test_number" {
						return fmt.Errorf("unexpected helperID: %s", helperID)
					}
					if config.Platform != "input_number" {
						return fmt.Errorf("unexpected platform: %s", config.Platform)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "input_number.test_number"},
		},
		{
			name: "update input_select options",
			args: map[string]any{
				"action":    "update",
				"entity_id": "input_select.mode",
				"options":   []any{"Mode A", "Mode B", "Mode C"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateHelperFn = func(_ context.Context, helperID string, _ homeassistant.HelperConfig) error {
					if helperID != "input_select.mode" {
						return fmt.Errorf("unexpected helperID: %s", helperID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated"},
		},
		{
			name: "update counter with step and bounds",
			args: map[string]any{
				"action":    "update",
				"entity_id": "counter.visitors",
				"step":      float64(5),
				"minimum":   float64(0),
				"maximum":   float64(200),
			},
			wantError:    false,
			wantContains: []string{"updated"},
		},
		{
			name: "update timer duration",
			args: map[string]any{
				"action":    "update",
				"entity_id": "timer.kitchen",
				"duration":  "00:10:00",
			},
			wantError:    false,
			wantContains: []string{"updated"},
		},
		{
			name: "update schedule blocks",
			args: map[string]any{
				"action":    "update",
				"entity_id": "schedule.work_hours",
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "16:00:00"},
				},
			},
			wantError:    false,
			wantContains: []string{"updated"},
		},
		{
			name: "update without entity_id",
			args: map[string]any{
				"action": "update",
				"min":    float64(10),
			},
			wantError:    true,
			wantContains: []string{"entity_id", "required"},
		},
		{
			name: "update config entry helper success",
			args: map[string]any{
				"action":    "update",
				"entity_id": "binary_sensor.threshold_test",
				"upper":     float64(30),
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "binary_sensor.threshold_test", ConfigEntryID: "config123"},
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "binary_sensor.threshold_test"},
		},
		{
			name: "update group helper success",
			args: map[string]any{
				"action":    "update",
				"entity_id": "group.lights",
				"entities":  []any{"light.one", "light.two"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "group.lights", ConfigEntryID: "config456"},
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "group.lights"},
		},
		{
			name: "update template_sensor success",
			args: map[string]any{
				"action":    "update",
				"entity_id": "sensor.my_template",
				"state":     "{{ 100 }}",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.my_template", ConfigEntryID: "config789"},
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "sensor.my_template"},
		},
		{
			name: "update template_sensor partial (only device_class)",
			args: map[string]any{
				"action":       "update",
				"entity_id":    "sensor.my_template",
				"device_class": "temperature",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.my_template", ConfigEntryID: "config789"},
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "sensor.my_template"},
		},
		{
			name: "update threshold helper with multiple fields",
			args: map[string]any{
				"action":     "update",
				"entity_id":  "binary_sensor.threshold_test",
				"upper":      float64(30),
				"lower":      float64(10),
				"hysteresis": float64(1.5),
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "binary_sensor.threshold_test", ConfigEntryID: "config123"},
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "binary_sensor.threshold_test"},
		},
		{
			name: "update websocket helper (counter) - fallback path",
			args: map[string]any{
				"action":    "update",
				"entity_id": "counter.test",
				"step":      float64(5),
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "counter.test", ConfigEntryID: ""}, // No config entry
					}, nil
				}
				m.UpdateHelperFn = func(context.Context, string, homeassistant.HelperConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "counter.test"},
		},
		{
			name: "update API error",
			args: map[string]any{
				"action":    "update",
				"entity_id": "counter.test",
				"step":      float64(10),
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateHelperFn = func(_ context.Context, _ string, _ homeassistant.HelperConfig) error {
					return fmt.Errorf("helper not found")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "updating"},
		},
		{
			// Regression test: manage_helper update failed with
			// "unknown_command" for config-entry template helpers because the handler
			// passed the bare object_id ("my_template") to client.UpdateHelper, but
			// HybridClient.UpdateHelper routes config-entry helpers by matching the
			// registry's FULL entity_id ("sensor.my_template"). This mock reproduces
			// that real routing logic - it only succeeds when it receives the full id.
			name: "update template config-entry helper routes on full entity_id",
			args: map[string]any{
				"action":    "update",
				"entity_id": "sensor.my_template",
				"state":     "{{ 42 }}",
			},
			setupMock: func(m *UniversalMockClient) {
				registry := map[string]string{"sensor.my_template": "config789"} // EntityID -> ConfigEntryID
				m.UpdateHelperFn = func(_ context.Context, id string, config homeassistant.HelperConfig) error {
					if _, ok := registry[id]; ok {
						return nil // options-flow success
					}
					// Reproduces the real WS fallback failure for a config-entry domain.
					return fmt.Errorf("update helper failed: command failed: unknown_command: %s/update", config.Platform)
				}
			},
			wantError:    false,
			wantContains: []string{"updated", "sensor.my_template"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// TestManageHelper_Update_NameField verifies that the name passed to update is
// forwarded to the API correctly, including unicode characters (umlauts etc.).
func TestManageHelper_Update_NameField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        map[string]any
		wantHasName bool
		wantName    string
	}{
		{
			name: "update with ASCII name",
			args: map[string]any{
				"action":    "update",
				"entity_id": "input_number.test_number",
				"name":      "New Name",
				"min":       float64(0),
				"max":       float64(100),
			},
			wantHasName: true,
			wantName:    "New Name",
		},
		{
			name: "update with unicode name (umlauts)",
			args: map[string]any{
				"action":    "update",
				"entity_id": "input_number.test_number",
				"name":      "Wärme Büro",
				"min":       float64(0),
				"max":       float64(100),
			},
			wantHasName: true,
			wantName:    "Wärme Büro",
		},
		{
			name: "update without name omits name key",
			args: map[string]any{
				"action":    "update",
				"entity_id": "input_number.test_number",
				"min":       float64(0),
				"max":       float64(100),
			},
			wantHasName: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedConfig homeassistant.HelperConfig
			client := &UniversalMockClient{}
			client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
				capturedConfig = cfg
				return nil
			}

			ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
				Timeout:      50 * time.Millisecond,
				PollInterval: 5 * time.Millisecond,
			})

			h := NewConsolidatedHelperHandlers()
			result, err := h.handleManageHelper(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("unexpected error result: %s", result.Content[0].Text)
			}

			if tt.wantHasName {
				gotName, ok := capturedConfig.Config["name"].(string)
				if !ok {
					t.Errorf("Config[\"name\"] missing or wrong type; got: %v", capturedConfig.Config["name"])
				} else if gotName != tt.wantName {
					t.Errorf("Config[\"name\"] = %q, want %q", gotName, tt.wantName)
				}
			} else {
				if _, hasName := capturedConfig.Config["name"]; hasName {
					t.Errorf("Config[\"name\"] present but should be absent; got: %v", capturedConfig.Config["name"])
				}
			}
		})
	}
}

// TestManageHelper_Update_ConfigEntryDoesNotLeakEntityID is a regression test for
// a bug where buildConfigEntryUpdateConfig forwarded the tool's top-level
// entity_id (identifying which helper to update) into the update config
// payload sent to Home Assistant, corrupting/rejecting the request. See the
// entity_id leak fix in buildConfigEntryUpdateConfig.
func TestManageHelper_Update_ConfigEntryDoesNotLeakEntityID(t *testing.T) {
	t.Parallel()

	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": "sensor.my_template",
		"state":     "{{ 42 }}",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if _, leaked := capturedConfig.Config["entity_id"]; leaked {
		t.Errorf("config-entry update config must not contain \"entity_id\" (the tool's top-level target identifier leaked into the HA payload); got: %v", capturedConfig.Config)
	}
}

// TestManageHelper_Update_ConfigEntryDoesNotDefaultDeviceClassForNonHygrostat is a
// regression test for a bug where addExtendedConfigEntryFields unconditionally
// defaulted device_class to "humidifier" for every config-entry helper's update,
// not just generic_hygrostat, corrupting/rejecting updates for every other type
// (template, threshold, group, ...).
func TestManageHelper_Update_ConfigEntryDoesNotDefaultDeviceClassForNonHygrostat(t *testing.T) {
	t.Parallel()

	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": "sensor.my_template",
		"state":     "{{ 42 }}",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if dc, leaked := capturedConfig.Config["device_class"]; leaked {
		t.Errorf("config-entry update for a non-hygrostat helper must not default device_class (got %q); it's only valid for generic_hygrostat, got config: %v", dc, capturedConfig.Config)
	}
}

// TestManageHelper_Update_GenericHygrostatStillDefaultsDeviceClass confirms the
// fix above doesn't remove the legitimate default for the helper type it was
// actually meant for.
func TestManageHelper_Update_GenericHygrostatStillDefaultsDeviceClass(t *testing.T) {
	t.Parallel()

	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":       "update",
		"entity_id":    "humidifier.my_hygrostat",
		"min_humidity": float64(30),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if dc, ok := capturedConfig.Config["device_class"].(string); !ok || dc != "humidifier" {
		t.Errorf("generic_hygrostat update should still default device_class to \"humidifier\"; got config: %v", capturedConfig.Config)
	}
}

// =============================================================================
// manage_helper - Update Partial-Merge Tests
//
// wsClientImpl.UpdateHelper sends the caller's config.Config as a full
// replacement to HA's <platform>/update WS command, so updating a WebSocket
// helper's "name" alone would otherwise silently reset every other field
// (e.g. an input_number's min/max) to empty. These tests verify
// mergeCurrentHelperState fills in the caller's omitted fields from the
// entity's current state/config before the update payload is built.
// =============================================================================

// TestManageHelper_Update_PartialInputNumber verifies that updating an
// input_number with only "name" preserves its current min/max/step by
// merging them in from GetState before building the update config.
func TestManageHelper_Update_PartialInputNumber(t *testing.T) {
	t.Parallel()

	entityID := "input_number.test_number"
	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return map[string]any{"min": 10.0, "max": 100.0, "step": 1.0}, nil
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      "New Name",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if got, ok := capturedConfig.Config["min"].(float64); !ok || got != 10.0 {
		t.Errorf("Config[\"min\"] = %v, want 10.0 (should be preserved from current state)", capturedConfig.Config["min"])
	}
	if got, ok := capturedConfig.Config["max"].(float64); !ok || got != 100.0 {
		t.Errorf("Config[\"max\"] = %v, want 100.0 (should be preserved from current state)", capturedConfig.Config["max"])
	}
}

// TestManageHelper_Update_PartialInputSelect verifies the same merge
// behavior for a slice-valued field (options).
func TestManageHelper_Update_PartialInputSelect(t *testing.T) {
	t.Parallel()

	entityID := "input_select.mode"
	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return map[string]any{"options": []any{"a", "b", "c"}}, nil
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      "New Name",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	options, ok := capturedConfig.Config["options"].([]string)
	if !ok {
		t.Fatalf("Config[\"options\"] missing or wrong type; got: %v", capturedConfig.Config["options"])
	}
	want := []string{"a", "b", "c"}
	if len(options) != len(want) {
		t.Fatalf("Config[\"options\"] = %v, want %v", options, want)
	}
	for i, v := range want {
		if options[i] != v {
			t.Errorf("Config[\"options\"][%d] = %q, want %q", i, options[i], v)
		}
	}
}

// TestManageHelper_Update_PartialSchedule verifies the merge for the
// schedule helper type.
// TestManageHelper_Update_PartialSchedule verifies that updating a schedule
// while omitting both "name" and every day but the one being changed
// preserves the existing day blocks AND the current name - regression test
// for the W1 gap where the schedule branch of mergeCurrentHelperState never
// populated currentName, so an update omitting "name" deleted it from the
// payload and HA's schedule/update (which requires "name") rejected the
// call outright.
func TestManageHelper_Update_PartialSchedule(t *testing.T) {
	t.Parallel()

	entityID := "schedule.work_hours"
	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return map[string]any{
			"name": "Work Hours",
			"monday": []any{
				map[string]any{"from": "08:00:00", "to": "09:00:00"},
			},
		}, nil
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if capturedConfig.Config["name"] != "Work Hours" {
		t.Errorf("Config[\"name\"] = %v, want the current name to survive an update that omits it", capturedConfig.Config["name"])
	}

	monday, ok := capturedConfig.Config["monday"].([]any)
	if !ok || len(monday) != 1 {
		t.Fatalf("Config[\"monday\"] = %v, want the day-block from GetHelperConfig to survive", capturedConfig.Config["monday"])
	}
	block, ok := monday[0].(map[string]any)
	if !ok || block["from"] != "08:00:00" || block["to"] != "09:00:00" {
		t.Errorf("Config[\"monday\"][0] = %v, want {from: 08:00:00, to: 09:00:00}", monday[0])
	}
}

// TestManageHelper_Update_ExplicitNullDoesNotOverwriteMergedValue guards
// against a caller-supplied JSON null defeating the partial-update merge: this API has
// no "clear this field" spelling, so a stray {"tuesday": null} must not
// overwrite the day block mergeCurrentHelperState already recovered from the
// entity's current stored config. Before this fix, mergeCurrentHelperState's
// final maps.Copy(merged, args) copied args unconditionally, so an explicit
// null in args always won over the merged value - and since a nil value then
// fails buildScheduleConfig's args[day].([]any) type assertion, the field
// silently vanished from the outgoing payload and HA's schedule/update reset
// that day to its default ([]).
func TestManageHelper_Update_ExplicitNullDoesNotOverwriteMergedValue(t *testing.T) {
	t.Parallel()

	entityID := "schedule.work_hours"
	tuesdayBlock := []any{map[string]any{"from": "10:00:00", "to": "11:00:00"}}
	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return map[string]any{"name": "Work Hours", "tuesday": tuesdayBlock}, nil
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"tuesday":   nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	tuesday, ok := capturedConfig.Config["tuesday"].([]any)
	if !ok || len(tuesday) != 1 {
		t.Fatalf("Config[\"tuesday\"] = %v, want the merged day-block to survive an explicit null in args", capturedConfig.Config["tuesday"])
	}
}

// TestManageHelper_Update_PreservesName verifies that when the caller omits
// "name", the merge falls back to the entity's current friendly_name instead
// of stripping the name field from the update payload.
func TestManageHelper_Update_PreservesName(t *testing.T) {
	t.Parallel()

	entityID := "input_number.test_number"
	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return map[string]any{"name": "Old Display Name"}, nil
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"min":       10.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if got, ok := capturedConfig.Config["name"].(string); !ok || got != "Old Display Name" {
		t.Errorf("Config[\"name\"] = %v, want \"Old Display Name\" (should fall back to current friendly_name)", capturedConfig.Config["name"])
	}
}

// TestManageHelper_Update_StateFetchFails_DegradesGracefully verifies that a
// failed current-state fetch degrades to args-only behavior (today's
// behavior) rather than failing the update.
// TestManageHelper_Update_StateFetchFails_ReturnsError verifies that a failed
// current-state fetch fails the update instead of silently proceeding with a
// partial config. WS "<platform>/update" commands replace the entire config,
// so writing a partial payload on a degraded read would reset every omitted
// field to its schema default (the same failure mode the partial-update merge
// exists to prevent, reintroduced on the error path). UpdateHelper must never be called in this case.
func TestManageHelper_Update_StateFetchFails_ReturnsError(t *testing.T) {
	t.Parallel()

	entityID := "input_number.test_number"
	updateHelperCalled := false
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return nil, errors.New("boom")
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, _ homeassistant.HelperConfig) error {
		updateHelperCalled = true
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"min":       10.0,
		"max":       100.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected update to fail when the current-state fetch fails, got success")
	}
	if updateHelperCalled {
		t.Error("UpdateHelper must not be called when the current-state fetch fails")
	}
}

// TestManageHelper_Update_NotFoundInStorage_HintsYAMLDefined verifies that
// when GetHelperConfig fails specifically because the entity isn't in the
// platform's storage list (the exact error wsClientImpl.GetHelperConfig
// returns for an entity that exists only via YAML configuration), the
// resulting error names YAML-definition as the likely cause instead of the
// generic "verify the entity exists" - a YAML-defined input_number does
// exist, so that generic phrasing would send the caller looking in the
// wrong place.
func TestManageHelper_Update_NotFoundInStorage_HintsYAMLDefinedOrRenamed(t *testing.T) {
	t.Parallel()

	entityID := "input_number.yaml_defined"
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return nil, fmt.Errorf("input_number not found: %s: %w", entityID, homeassistant.ErrHelperNotFoundInStorage)
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"min":       10.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected update to fail when the entity isn't in the storage list, got success")
	}
	if !strings.Contains(result.Content[0].Text, "configuration.yaml") {
		t.Errorf("error message = %q, want it to mention configuration.yaml as one likely cause", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "renamed") {
		t.Errorf("error message = %q, want it to mention a post-creation rename as the other likely cause", result.Content[0].Text)
	}
}

// TestManageHelper_Update_ScheduleConfigFetchFails_DoesNotWipe is the
// regression guard for the most severe form of the merge-fetch-failure bug:
// a schedule's weekday fields (vol.Optional(day, default=[])) are erased if
// an update payload omits them. If GetHelperConfig fails and the merge
// proceeded anyway, an update passing only "name" would silently wipe every
// weekday's schedule. UpdateHelper must never be called in this case.
func TestManageHelper_Update_ScheduleConfigFetchFails_DoesNotWipe(t *testing.T) {
	t.Parallel()

	entityID := "schedule.work_hours"
	updateHelperCalled := false
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return nil, errors.New("boom")
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, _ homeassistant.HelperConfig) error {
		updateHelperCalled = true
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      "New",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected update to fail when GetHelperConfig fails, got success")
	}
	if updateHelperCalled {
		t.Error("UpdateHelper must not be called when GetHelperConfig fails - it would wipe every weekday's schedule")
	}
}

// TestManageHelper_Update_GroupNotMerged is a regression guard for the
// branch-label bug: "group" is a key in helperTypes (so the naive `if ok`
// check would treat it as a WebSocket helper), but group is actually a
// Config Entry Flow platform (homeassistant.RequiresConfigEntryFlow("group")
// == true) and must NOT go through mergeCurrentHelperState. If it did, a
// GetState-sourced "entities" value would leak into the update config even
// though the caller never supplied "entities".
func TestManageHelper_Update_GroupNotMerged(t *testing.T) {
	t.Parallel()

	entityID := "group.lights"
	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.GetStateFn = func(_ context.Context, _ string) (*homeassistant.Entity, error) {
		return &homeassistant.Entity{
			EntityID:   entityID,
			Attributes: map[string]any{"entities": []any{"light.leaked_from_merge"}},
		}, nil
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      "New Group Name",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if entities, present := capturedConfig.Config["entities"]; present {
		t.Errorf("group update must not go through mergeCurrentHelperState (it's a Config Entry platform) - entities leaked into config: %v", entities)
	}
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
				m.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
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
				m.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
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
			name: "get_details for counter with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "counter.visitors",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "42",
						Attributes: map[string]any{
							"friendly_name": "Visitor Counter",
							"initial":       float64(0),
							"minimum":       float64(0),
							"maximum":       float64(100),
							"step":          float64(1),
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"counter.visitors", "42", "initial", "minimum", "maximum"},
		},
		{
			name: "get_details for counter with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "counter.visitors",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "42",
						Attributes: map[string]any{
							"friendly_name": "Visitor Counter",
							"initial":       float64(0),
							"minimum":       float64(0),
							"maximum":       float64(100),
							"step":          float64(1),
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Visitor Counter", "42", "Initial value", "Minimum", "Maximum"},
		},
		{
			name: "get_details for timer with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "timer.pomodoro",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "active",
						Attributes: map[string]any{
							"friendly_name": "Pomodoro Timer",
							"duration":      "0:25:00",
							"remaining":     "0:15:30",
							"finishes_at":   "2024-01-15T12:25:00+00:00",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"timer.pomodoro", "active", "duration", "remaining"},
		},
		{
			name: "get_details for timer with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "timer.pomodoro",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "idle",
						Attributes: map[string]any{
							"friendly_name": "Pomodoro Timer",
							"duration":      "0:25:00",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Pomodoro Timer", "idle", "Duration"},
		},
		{
			name: "get_details for input_boolean with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_boolean.living_room_light",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "on",
						Attributes: map[string]any{
							"friendly_name": "Living Room Light",
							"icon":          "mdi:lightbulb",
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Toggle", "Living Room Light", "on", "Editable: true"},
		},
		{
			name: "get_details for input_boolean with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_boolean.living_room_light",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "on",
						Attributes: map[string]any{
							"friendly_name": "Living Room Light",
							"icon":          "mdi:lightbulb",
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"input_boolean.living_room_light", "\"state\"", "\"on\"", "\"icon\""},
		},
		{
			name: "get_details for input_number with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_number.target_temperature",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "22.5",
						Attributes: map[string]any{
							"friendly_name":       "Target Temperature",
							"min":                 float64(15),
							"max":                 float64(30),
							"step":                float64(0.5),
							"mode":                "slider",
							"unit_of_measurement": "°C",
							"editable":            true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Number", "Target Temperature", "22.5", "Range: 15 - 30", "Step: 0.5", "°C"},
		},
		{
			name: "get_details for input_number with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_number.target_temperature",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "22.5",
						Attributes: map[string]any{
							"friendly_name":       "Target Temperature",
							"min":                 float64(15),
							"max":                 float64(30),
							"step":                float64(0.5),
							"mode":                "slider",
							"unit_of_measurement": "°C",
							"editable":            true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"input_number.target_temperature", "\"min\"", "\"max\"", "\"step\""},
		},
		{
			name: "get_details for input_text with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_text.notification_message",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "Hello World",
						Attributes: map[string]any{
							"friendly_name": "Notification Message",
							"min":           float64(0),
							"max":           float64(100),
							"mode":          "text",
							"pattern":       "[A-Za-z0-9 ]+",
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Text", "Notification Message", "Hello World", "Length: 0 - 100", "Pattern"},
		},
		{
			name: "get_details for input_text with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_text.notification_message",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "Hello World",
						Attributes: map[string]any{
							"friendly_name": "Notification Message",
							"min":           float64(0),
							"max":           float64(100),
							"mode":          "text",
							"pattern":       "[A-Za-z0-9 ]+",
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"input_text.notification_message", "\"min\"", "\"max\"", "\"pattern\""},
		},
		{
			name: "get_details for input_select with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_select.theme",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "dark",
						Attributes: map[string]any{
							"friendly_name": "Theme",
							"options":       []any{"light", "dark", "auto"},
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Select", "Theme", "Selected: dark", "Options: light, dark, auto"},
		},
		{
			name: "get_details for input_select with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_select.theme",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "dark",
						Attributes: map[string]any{
							"friendly_name": "Theme",
							"options":       []any{"light", "dark", "auto"},
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"input_select.theme", "\"options\"", "\"dark\""},
		},
		{
			name: "get_details for input_datetime with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_datetime.alarm_time",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "2024-01-15 07:30:00",
						Attributes: map[string]any{
							"friendly_name": "Alarm Time",
							"has_date":      true,
							"has_time":      true,
							"year":          float64(2024),
							"month":         float64(1),
							"day":           float64(15),
							"hour":          float64(7),
							"minute":        float64(30),
							"second":        float64(0),
							"timestamp":     float64(1705305000),
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Date/Time", "Alarm Time", "2024-01-15 07:30:00", "Date and time"},
		},
		{
			name: "get_details for input_datetime with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_datetime.alarm_time",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "2024-01-15 07:30:00",
						Attributes: map[string]any{
							"friendly_name": "Alarm Time",
							"has_date":      true,
							"has_time":      true,
							"year":          float64(2024),
							"month":         float64(1),
							"day":           float64(15),
							"hour":          float64(7),
							"minute":        float64(30),
							"second":        float64(0),
							"timestamp":     float64(1705305000),
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"input_datetime.alarm_time", "\"has_date\"", "\"has_time\"", "\"timestamp\""},
		},
		{
			name: "get_details for input_button with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_button.doorbell",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "2024-01-15T10:30:00+00:00",
						Attributes: map[string]any{
							"friendly_name": "Doorbell",
							"icon":          "mdi:doorbell",
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Button", "Doorbell", "Last pressed"},
		},
		{
			name: "get_details for input_button with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "input_button.doorbell",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "2024-01-15T10:30:00+00:00",
						Attributes: map[string]any{
							"friendly_name": "Doorbell",
							"icon":          "mdi:doorbell",
							"editable":      true,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"input_button.doorbell", "\"icon\""},
		},
		{
			name: "get_details for group with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "group.all_lights",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "on",
						Attributes: map[string]any{
							"friendly_name": "All Lights",
							"entity_id":     []any{"light.living_room", "light.bedroom", "light.kitchen"},
							"all":           false,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Group", "All Lights", "on", "Mode: any", "Members (3)", "light.living_room"},
		},
		{
			name: "get_details for group with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "group.all_lights",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "on",
						Attributes: map[string]any{
							"friendly_name": "All Lights",
							"entity_id":     []any{"light.living_room", "light.bedroom", "light.kitchen"},
							"all":           false,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"group.all_lights", "\"members\"", "\"all\""},
		},
		{
			name: "get_details for sensor (Config Entry) with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.power_usage",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "125.5",
						Attributes: map[string]any{
							"friendly_name":       "Power Usage",
							"unit_of_measurement": "W",
							"device_class":        "power",
							"state_class":         "measurement",
							"source":              "sensor.raw_power",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Sensor", "Power Usage", "125.5", "W", "Device class: power"},
		},
		{
			name: "get_details for sensor (Config Entry) with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.power_usage",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "125.5",
						Attributes: map[string]any{
							"friendly_name":       "Power Usage",
							"unit_of_measurement": "W",
							"device_class":        "power",
							"state_class":         "measurement",
							"source":              "sensor.raw_power",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"sensor.power_usage", "\"unit_of_measurement\"", "\"device_class\""},
		},
		{
			name: "get_details for binary_sensor (Config Entry) with natural format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "binary_sensor.garage_door",
				"format":    "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "off",
						Attributes: map[string]any{
							"friendly_name": "Garage Door",
							"device_class":  "door",
							"entity_id":     "binary_sensor.raw_garage",
							"hysteresis":    float64(0.0),
							"sensor_value":  "off",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Binary Sensor", "Garage Door", "off", "Device class: door"},
		},
		{
			name: "get_details for binary_sensor (Config Entry) with json format",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "binary_sensor.garage_door",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "off",
						Attributes: map[string]any{
							"friendly_name": "Garage Door",
							"device_class":  "door",
							"entity_id":     "binary_sensor.raw_garage",
							"hysteresis":    float64(0.0),
							"sensor_value":  "off",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"binary_sensor.garage_door", "\"device_class\"", "\"source_entity\""},
		},
		{
			name: "get_details for template sensor with options (natural)",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.template_test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "42",
						Attributes: map[string]any{
							"friendly_name":       "Template Test",
							"unit_of_measurement": "°C",
							"device_class":        "temperature",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID:      "sensor.template_test",
							Platform:      "template",
							ConfigEntryID: "config123",
						},
					}, nil
				}
				m.GetConfigEntryOptionsFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{
						"state":               "{{ states('sensor.source') | float }}",
						"availability":        "{{ states('sensor.source') != 'unavailable' }}",
						"unit_of_measurement": "°C",
						"device_class":        "temperature",
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Sensor: Template Test", "Template Configuration:", "State template:", "{{ states('sensor.source') | float }}", "Availability template:"},
		},
		{
			name: "get_details for template sensor with options (json)",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.template_test",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "42",
						Attributes: map[string]any{
							"friendly_name": "Template Test",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID:      "sensor.template_test",
							Platform:      "template",
							ConfigEntryID: "config123",
						},
					}, nil
				}
				m.GetConfigEntryOptionsFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{
						"state": "{{ states('sensor.source') | float }}",
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"\"state_template\"", "\"config_entry_type\"", "template"},
		},
		{
			name: "get_details for template binary_sensor with delays",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "binary_sensor.template_test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "on",
						Attributes: map[string]any{
							"friendly_name": "Template Binary",
							"device_class":  "motion",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID:      "binary_sensor.template_test",
							Platform:      "template",
							ConfigEntryID: "config456",
						},
					}, nil
				}
				m.GetConfigEntryOptionsFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{
						"state":     "{{ states('binary_sensor.source') }}",
						"delay_on":  map[string]any{"seconds": 5},
						"delay_off": map[string]any{"seconds": 10},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Binary Sensor: Template Binary", "Template Configuration:", "Delay on:", "Delay off:"},
		},
		{
			name: "get_details for non-template sensor (no enrichment)",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.derivative_test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "5.5",
						Attributes: map[string]any{
							"friendly_name":       "Derivative Test",
							"unit_of_measurement": "W",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID:      "sensor.derivative_test",
							Platform:      "derivative",
							ConfigEntryID: "config789",
						},
					}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"Sensor: Derivative Test", "5.5"},
			wantNotContains: []string{"Template Configuration"},
		},
		{
			name: "get_details for sensor not in registry (graceful)",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.unknown",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "123",
						Attributes: map[string]any{
							"friendly_name": "Unknown Sensor",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"Sensor: Unknown Sensor", "123"},
			wantNotContains: []string{"Template Configuration"},
		},
		{
			name: "get_details for template sensor with empty options (graceful)",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.template_empty",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "100",
						Attributes: map[string]any{
							"friendly_name": "Empty Template",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID:      "sensor.template_empty",
							Platform:      "template",
							ConfigEntryID: "config_empty",
						},
					}, nil
				}
				m.GetConfigEntryOptionsFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"Sensor: Empty Template", "100"},
			wantNotContains: []string{"Template Configuration"},
		},
		{
			name: "get_details for sensor with GetEntityRegistry error (graceful)",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.error_test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "99",
						Attributes: map[string]any{
							"friendly_name": "Error Test",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return nil, fmt.Errorf("registry unavailable")
				}
			},
			wantError:       false,
			wantContains:    []string{"Sensor: Error Test", "99"},
			wantNotContains: []string{"Template Configuration"},
		},
		{
			name: "get_details for template sensor with GetConfigEntry error (graceful)",
			args: map[string]any{
				"action":    "get_details",
				"entity_id": "sensor.config_error",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: entityID,
						State:    "77",
						Attributes: map[string]any{
							"friendly_name": "Config Error",
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID:      "sensor.config_error",
							Platform:      "template",
							ConfigEntryID: "config_err",
						},
					}, nil
				}
				m.GetConfigEntryOptionsFn = func(context.Context, string) (map[string]any, error) {
					return nil, fmt.Errorf("config entry not found")
				}
			},
			wantError:       false,
			wantContains:    []string{"Sensor: Config Error", "77"},
			wantNotContains: []string{"Template Configuration"},
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
		{
			name: "set_options with missing options parameter",
			args: map[string]any{
				"entity_id": "input_select.test",
				"action":    "set_options",
			},
			wantError:    true,
			wantContains: []string{"options", "required"},
		},
		{
			name: "set_options with empty options array",
			args: map[string]any{
				"entity_id": "input_select.test",
				"action":    "set_options",
				"options":   []any{},
			},
			wantError:    true,
			wantContains: []string{"at least one value"},
		},
		{
			name: "add_entities with missing parameter",
			args: map[string]any{
				"entity_id": "group.test",
				"action":    "add_entities",
			},
			wantError:    true,
			wantContains: []string{"add_entities", "required"},
		},
		{
			name: "add_entities with empty array",
			args: map[string]any{
				"entity_id":    "group.test",
				"action":       "add_entities",
				"add_entities": []any{},
			},
			wantError:    true,
			wantContains: []string{"at least one entity"},
		},
		{
			name: "remove_entities with missing parameter",
			args: map[string]any{
				"entity_id": "group.test",
				"action":    "remove_entities",
			},
			wantError:    true,
			wantContains: []string{"remove_entities", "required"},
		},
		{
			name: "remove_entities with empty array",
			args: map[string]any{
				"entity_id":       "group.test",
				"action":          "remove_entities",
				"remove_entities": []any{},
			},
			wantError:    true,
			wantContains: []string{"at least one entity"},
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
			wantContains: []string{"error", "creating"},
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

// TestHelperTypeMetadata_SourceEntityFields asserts that exactly the 7 helper
// types with a source-entity domain constraint carry sourceEntityDomains and
// sourceEntityField, and every other helper type is left at the zero value.
// TestHelperTypeMetadata_SourceEntityConstraints asserts the exact set of
// domain-constrained source-entity fields per helper type, including both
// constraints on generic_thermostat/generic_hygrostat (actuator field AND
// target_sensor_entity_id) - a prior version of this test checked only the
// first constraint and would not have caught a missing second one. Also
// asserts every constraint has a non-empty field and non-empty domains list,
// since an empty field would silently look up args[""] at runtime.
// TestPerTypeUpdateExcludedFields_KeysAreKnownHelperTypes guards
// perTypeUpdateExcludedFields against silent drift: its keys are free
// strings, not tied to helperTypes by the type system, so a typo or a
// future rename of a helperTypes key would silently disable the exclusion -
// re-advertising an unrecoverable field as updatable and reintroducing the
// exact data-loss bug perTypeUpdateExcludedFields exists to prevent, with
// no compile error. This also catches the mirror mistake: an excluded field
// name that no longer appears in that type's required/optional fields (a
// stale exclusion left behind after the field itself was removed).
func TestPerTypeUpdateExcludedFields_KeysAreKnownHelperTypes(t *testing.T) {
	t.Parallel()

	for typeName, excluded := range perTypeUpdateExcludedFields {
		t.Run(typeName, func(t *testing.T) {
			t.Parallel()

			meta, ok := helperTypes[typeName]
			if !ok {
				t.Fatalf("perTypeUpdateExcludedFields has key %q, which is not a known helperTypes entry", typeName)
			}

			allFields := make(map[string]bool, len(meta.requiredFields)+len(meta.optionalFields))
			for _, f := range meta.requiredFields {
				allFields[f] = true
			}
			for _, f := range meta.optionalFields {
				allFields[f] = true
			}

			for field := range excluded {
				if !allFields[field] {
					t.Errorf("perTypeUpdateExcludedFields[%q] excludes field %q, which is not in helperTypes[%q]'s required/optional fields", typeName, field, typeName)
				}
			}
		})
	}
}

// TestSourceConstrainedTypes_PlatformsAreUnique guards
// buildSourceConstrainedTypes against a silent platform collision.
// sourceConstrainedTypes is keyed by platform, but helperTypes already has
// two entries sharing platformTemplate (template_sensor,
// template_binary_sensor) - harmless today since neither is constrained,
// but if either ever gains a sourceEntities entry, buildSourceConstrainedTypes
// would keep whichever one Go's random map iteration order visits last,
// silently and nondeterministically dropping the other's constraint. This
// test compares counts rather than iterating helperTypes and building a
// second index (which would just duplicate buildSourceConstrainedTypes'
// own logic and could share its bugs) - a real collision necessarily makes
// the counts diverge.
func TestSourceConstrainedTypes_PlatformsAreUnique(t *testing.T) {
	t.Parallel()

	constrainedTypeCount := 0
	for _, meta := range helperTypes {
		if len(meta.sourceEntities) > 0 {
			constrainedTypeCount++
		}
	}

	if constrainedTypeCount != len(sourceConstrainedTypes) {
		t.Errorf("helperTypes has %d source-constrained types but sourceConstrainedTypes has %d entries - "+
			"two constrained types share a platform name, so buildSourceConstrainedTypes silently dropped one",
			constrainedTypeCount, len(sourceConstrainedTypes))
	}
}

// TestSourceConstrainedTypes_AreAllConfigEntryFlow enforces the reachability
// invariant handleUpdate's comment asserts only in prose: no
// source-constrained helper type can reach the
// "ok && !RequiresConfigEntryFlow" merge branch, so
// checkUpdateSourceEntityDomain's skip on that branch is safe. If a future
// source-constrained type were ever added as a genuine WS helper (not a
// Config Entry Flow platform), this invariant would silently break and
// update-time domain validation would stop running for it, with no test
// failure pointing at the cause.
func TestSourceConstrainedTypes_AreAllConfigEntryFlow(t *testing.T) {
	t.Parallel()

	for name, meta := range helperTypes {
		if len(meta.sourceEntities) == 0 {
			continue
		}
		if !homeassistant.RequiresConfigEntryFlow(meta.platform) {
			t.Errorf("helperTypes[%q] has sourceEntities but platform %q is not a Config Entry Flow platform - "+
				"handleUpdate's merge branch (ok && !RequiresConfigEntryFlow) would reach this type, and its "+
				"skip of checkUpdateSourceEntityDomain on that branch assumes this can never happen",
				name, meta.platform)
		}
	}
}

// TestWrapperRecipeFor_RejectsInjectionPayloads pins the security guard on
// wrapperRecipeFor: it interpolates a caller-supplied entity_id unescaped
// into a Jinja template string inside a ready-to-run, copy-pasteable
// manage_helper(...) call. That's currently safe only because
// ValidateEntityID's entityIDPattern (validation.go) admits no quotes or
// braces - a control that lives in a different file, serves many unrelated
// callers, and has no test tying it to this specific sink. If that pattern
// is ever loosened for some unrelated caller, this test is what catches
// wrapperRecipeFor turning into a live template-injection vector.
func TestWrapperRecipeFor_RejectsInjectionPayloads(t *testing.T) {
	t.Parallel()

	payloads := []string{
		`sensor.x') }}{{ states('secret`,
		`sensor.x'); import os; #`,
		`sensor.x"`,
		"Sensor.X",
		"no_dot",
		"",
		"sensor.",
		".x",
	}

	sensorConstraint := sourceEntityConstraint{field: attrEntityID, domains: []string{"sensor"}}

	for _, payload := range payloads {
		t.Run(payload, func(t *testing.T) {
			t.Parallel()
			if got := wrapperRecipeFor(sensorConstraint, payload); got != "" {
				t.Errorf("wrapperRecipeFor(%+v, %q) = %q, want empty string for a malformed/injection-shaped entity_id", sensorConstraint, payload, got)
			}
		})
	}
}

func TestHelperTypeMetadata_SourceEntityConstraints(t *testing.T) {
	t.Parallel()

	want := map[string][]sourceEntityConstraint{
		"utility_meter": {{field: "source", domains: []string{"sensor"}}},
		"statistics":    {{field: "entity_id", domains: []string{"sensor", "binary_sensor"}}},
		"trend":         {{field: "entity_id", domains: []string{"sensor", "counter"}}},
		"filter":        {{field: "entity_id", domains: []string{"sensor"}}},
		"switch_as_x":   {{field: "entity_id", domains: []string{"switch"}}},
		"generic_thermostat": {
			{field: "heater_entity_id", domains: []string{"switch", "fan"}},
			{field: "target_sensor_entity_id", domains: []string{"sensor"}, deviceClasses: []string{"temperature"}},
		},
		"generic_hygrostat": {
			{field: "humidifier_entity_id", domains: []string{"switch", "fan"}},
			{field: "target_sensor_entity_id", domains: []string{"sensor"}, deviceClasses: []string{"humidity"}},
		},
	}

	for name, meta := range helperTypes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			wantConstraints := want[name] // nil for unconstrained types
			if !reflect.DeepEqual(meta.sourceEntities, wantConstraints) {
				t.Errorf("helperTypes[%q].sourceEntities = %+v, want %+v", name, meta.sourceEntities, wantConstraints)
			}
			for _, c := range meta.sourceEntities {
				if c.field == "" {
					t.Errorf("helperTypes[%q] has a sourceEntityConstraint with an empty field", name)
				}
				if len(c.domains) == 0 {
					t.Errorf("helperTypes[%q] sourceEntityConstraint %q has no domains", name, c.field)
				}
			}
		})
	}
}

// updatableFieldSentinel returns a type-appropriate probe value for field on
// helper type name, matching whatever Go type assertion the real update
// builder (buildHelperConfig for WS types, buildConfigEntryUpdateConfig for
// Config Entry types) performs on that args key. "initial" is the only name
// whose expected type varies by helper type (bool/float64/string/int-via-
// float64), so it's special-cased; every other name has one consistent type
// across every helper type that declares it.
func updatableFieldSentinel(name, field string) any {
	if field == "initial" {
		switch name {
		case "input_boolean":
			return true
		case "input_number", "counter":
			return float64(5)
		default: // input_text, input_select, input_datetime: string
			return "test-value"
		}
	}

	switch field {
	case "options", "entities", "tariffs", "entity_ids",
		"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday":
		return []any{"probe"}
	case "has_date", "has_time", "restore", "all", "delta_values", "net_consumption",
		"periodically_resetting", "invert", "ac_mode":
		return true
	case "lower", "upper", "hysteresis", "min", "max", "step", "offset", "percentile",
		"min_gradient", "sample_duration", "min_temp", "max_temp", "target_temp",
		"cold_tolerance", "hot_tolerance", "min_humidity", "max_humidity",
		"target_humidity", "dry_tolerance", "wet_tolerance", "minimum", "maximum",
		"round", "time_window", "round_digits", "sampling_size", "precision",
		"min_samples", "max_samples", "delay_on", "delay_off",
		"radius", "time_constant", "lower_bound", "upper_bound":
		// argReader's num/integer/str readers all accept a float64 (str
		// coerces it to a decimal string), so one numeric sentinel exercises
		// every numeric-or-numeric-string field uniformly.
		return float64(1)
	default:
		// icon, state, source, unit_of_measurement, device_class, state_class,
		// unit_time, unit_prefix, method, group_type, mode, pattern, duration,
		// cycle, max_age, after_time, before_time, after_offset, before_offset,
		// heater_entity_id, target_sensor_entity_id, humidifier_entity_id,
		// target_domain, window_size (raw - accepts a string): every
		// remaining name is read as a plain string.
		return "test-value"
	}
}

// updateConfigKeyAliases maps an args field name to the config key it lands
// under after the update builder runs, for the few fields HA's API renames.
// See CLAUDE.md's "Config Entry API Field Mapping" gotcha.
var updateConfigKeyAliases = map[string]string{
	"heater_entity_id":        "heater",
	"target_sensor_entity_id": "target_sensor",
	"humidifier_entity_id":    "humidifier",
	"min_max_type":            "type",
}

// TestUpdatableFields_AreActuallyReadByUpdatePath asserts every field name
// updatableFieldNames() advertises as accepted on update is actually
// consumed by the real update builder - the property updateExcludedFields
// exists to make explicit. Without this, the generated per-type description
// in manage_helper's schema can silently drift from the update code (as it
// already had for "entity_id", "filter", and min_max's "type" - all three
// are excluded via updateExcludedFields, not just documented as such).
func TestUpdatableFields_AreActuallyReadByUpdatePath(t *testing.T) {
	t.Parallel()

	for name, meta := range helperTypes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fields := updatableFieldNames(name)
			args := make(map[string]any, len(fields))
			for _, field := range fields {
				args[field] = updatableFieldSentinel(name, field)
			}

			var config map[string]any
			if homeassistant.RequiresConfigEntryFlow(meta.platform) {
				// Mirrors handleUpdate: buildConfigEntryUpdateConfig is called
				// with the entity DOMAIN (meta.entityPrefix), not the
				// helperTypes map key - that distinction matters for the
				// platform=="humidifier" device_class default gate. meta.platform
				// is passed as the resolved min_max_type platform too - it's the
				// real integration platform for every entry here, same as what
				// resolveConfigEntryPlatformForMinMaxType would return.
				var err error
				config, err = buildConfigEntryUpdateConfig(meta.entityPrefix, meta.platform, args)
				if err != nil {
					t.Fatalf("buildConfigEntryUpdateConfig(%q, ...) returned error: %v", name, err)
				}
			} else {
				var err error
				config, err = buildHelperConfig(name, "Test Name", args)
				if err != nil {
					t.Fatalf("buildHelperConfig(%q, ...) returned error: %v", name, err)
				}
			}

			for _, field := range fields {
				key := field
				if alias, ok := updateConfigKeyAliases[field]; ok {
					key = alias
				}
				if _, present := config[key]; !present {
					t.Errorf("field %q (config key %q) is listed as updatable for %q but was not read by the update builder - add it to perTypeUpdateExcludedFields or fix the builder", field, key, name)
				}
			}
		})
	}
}

// reservedManageHelperArgNames are manage_helper's own top-level dispatch
// arguments. min_max shipped with a real bug caused by its per-instance
// "type" config field sharing the literal args key handleCreate already
// consumes to pick which helper type to build - buildMinMaxConfig always
// read back the already-consumed "min_max" selector instead of the
// caller's intended calculation. TestHelperTypes_NoReservedArgNameCollisions
// guards the class, not just that one instance: no helperTypes entry may
// declare a create-time field with one of these names. "entity_id" and
// "name" are deliberately excluded - both are legitimate create-time config
// fields for several types (e.g. threshold's monitored source, every
// Config Entry type's display name); "entity_id"'s distinct update-side
// collision (colliding with "which helper is being updated") is already
// guarded by isUpdateIdentifierField.
var reservedManageHelperArgNames = map[string]bool{
	"action": true,
	"type":   true,
	"id":     true,
}

// TestHelperTypes_NoReservedArgNameCollisions is a regression test for the
// reserved-arg-name-collision bug class: it fails if any helperTypes entry's requiredFields or
// optionalFields reuses a name manage_helper's own dispatcher already
// consumes from the same args map before a type-specific builder ever sees
// it. See reservedManageHelperArgNames' doc comment for why min_max's fix
// was renaming to min_max_type rather than special-casing the read.
func TestHelperTypes_NoReservedArgNameCollisions(t *testing.T) {
	t.Parallel()

	for name, meta := range helperTypes {
		for _, field := range append(append([]string{}, meta.requiredFields...), meta.optionalFields...) {
			if reservedManageHelperArgNames[field] {
				t.Errorf("helperTypes[%q] declares field %q, which collides with manage_helper's own top-level dispatch argument of the same name - rename it (see min_max_type, renamed from a colliding \"type\" field, for the precedent)", name, field)
			}
		}
	}
}

// realHelperStorageConfig models the stored-config keys real Home Assistant
// returns from a WebSocket helper platform's "<platform>/list" command (what
// mergeCurrentHelperState now reads via GetHelperConfig, for every genuine WS
// helper type - not just schedule, which is why this fixture no longer
// special-cases it) - verified against each component's STORAGE_FIELDS
// vol.Schema in Home Assistant core
// (homeassistant/components/<platform>/__init__.py). Unlike entity state
// attributes (GetState), which are a runtime, sometimes-conditional
// projection of config, "<platform>/list" echoes back the raw stored
// config verbatim: every field the helper was created or updated with is
// present, whether or not the entity's current state happens to surface it.
// "icon" is added by the caller for every type (STORAGE_FIELDS declares it
// uniformly as vol.Optional(CONF_ICON) with no default, so it is only
// present when actually set - the caller sets it to model that case).
//
// This fixture models "the helper was created with every optional field
// explicitly set" - the case that matters for merge recoverability, since a
// field genuinely never configured has nothing to preserve.
//
// This is deliberately the true STORAGE_FIELDS set for each platform, NOT a
// copy of updatableFieldNames() - a fixture that mirrored updatableFieldNames
// could never disagree with it, making
// TestMergeCurrentHelperState_OnlyAdvertisesRecoverableFields a tautology
// instead of a genuine regression guard. Both directions are checked against
// this one independent model: every updatableFieldNames() entry must appear
// here (a field advertised as updatable that real HA never stores would be
// silently unrecoverable), and every key here must appear in
// updatableFieldNames() or perTypeUpdateExcludedFields (a real stored field
// missing from updatableFieldNames() is exactly how counter's "restore" went
// unnoticed - HA stores and returns it, but it was absent from counter's
// optionalFields, so mergeCurrentHelperState silently dropped it on every
// update).
func realHelperStorageConfig(typeName string) map[string]any {
	switch typeName {
	case "input_boolean":
		return map[string]any{"initial": true}
	case "input_button":
		// input_button has no "initial" STORAGE_FIELD - a momentary press has
		// no value to restore, only icon (added by the caller below).
		return map[string]any{}
	case "input_number":
		return map[string]any{
			"initial": 5.0, "min": 0.0, "max": 100.0,
			"step": 1.0, "mode": "slider", "unit_of_measurement": "%",
		}
	case "input_text":
		return map[string]any{
			"min": 0.0, "max": 100.0, "mode": "text", "pattern": "^[a-z]+$", "initial": "hello",
		}
	case "input_select":
		return map[string]any{"options": []any{"a", "b"}, "initial": "a"}
	case "input_datetime":
		return map[string]any{"has_date": true, "has_time": true, "initial": "2024-01-01"}
	case "counter":
		return map[string]any{
			"initial": 0.0, "step": 1.0, "minimum": 0.0, "maximum": 100.0, "restore": true,
		}
	case "timer":
		return map[string]any{"duration": "0:05:00", "restore": true}
	case "schedule":
		block := []any{map[string]any{"from": "08:00:00", "to": "09:00:00"}}
		return map[string]any{
			"monday": block, "tuesday": block, "wednesday": block, "thursday": block,
			"friday": block, "saturday": block, "sunday": block,
		}
	default:
		return map[string]any{}
	}
}

// TestMergeCurrentHelperState_OnlyAdvertisesRecoverableFields is the mirror
// of TestUpdatableFields_AreActuallyReadByUpdatePath: that test proves every
// updatableFieldNames() entry is READ by the update builder given the field;
// this test proves every entry can actually be RECOVERED by
// mergeCurrentHelperState from what real Home Assistant's "<platform>/list"
// stored config exposes, given a helper that was created with every optional
// field explicitly set. Without this, updatableFieldNames() can advertise a
// field the merge can never fill in.
//
// The reverse direction also matters and is checked here too: every key
// realHelperStorageConfig models for a type must be reachable via either
// updatableFieldNames() or perTypeUpdateExcludedFields - otherwise a field
// real Home Assistant stores and returns (and would happily preserve across
// an update) is invisible to helperTypes entirely, so mergeCurrentHelperState
// filters it out and every update to that helper silently resets it to HA's
// default. This is precisely how counter's "restore" field went unnoticed:
// storage config always returns it, but it was absent from
// counter.optionalFields, so no assertion in either direction previously
// caught the gap.
func TestMergeCurrentHelperState_OnlyAdvertisesRecoverableFields(t *testing.T) {
	t.Parallel()

	for name, meta := range helperTypes {
		if homeassistant.RequiresConfigEntryFlow(meta.platform) {
			continue // not reached via mergeCurrentHelperState - see buildKnownTypeUpdateConfig
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			entityID := meta.entityPrefix + ".test_entity"
			client := &UniversalMockClient{}
			config := realHelperStorageConfig(name)
			config["icon"] = "mdi:test"
			config["name"] = "Current Name"
			client.GetHelperConfigFn = func(context.Context, string, string) (map[string]any, error) {
				return config, nil
			}

			merged, _, err := mergeCurrentHelperState(context.Background(), client, entityID, name, meta, map[string]any{})
			if err != nil {
				t.Fatalf("mergeCurrentHelperState returned err: %v", err)
			}

			for _, field := range updatableFieldNames(name) {
				if _, present := merged[field]; !present {
					t.Errorf("field %q is advertised as updatable for %q but real Home Assistant's stored config does not expose it, so an omitted value can never be recovered - add it to perTypeUpdateExcludedFields", field, name)
				}
			}

			updatable := make(map[string]bool, len(updatableFieldNames(name)))
			for _, field := range updatableFieldNames(name) {
				updatable[field] = true
			}
			for field := range realHelperStorageConfig(name) {
				if updatable[field] || perTypeUpdateExcludedFields[name][field] {
					continue
				}
				t.Errorf("field %q is a real stored-config field for %q (per realHelperStorageConfig) but is not in updatableFieldNames() or perTypeUpdateExcludedFields - "+
					"it will be silently dropped by mergeCurrentHelperState and reset to Home Assistant's default on every update; add it to optionalFields (or requiredFields) for this type", field, name)
			}
		})
	}
}

// TestMergeCurrentHelperState_TimerRestoreFalseSurvivesMerge guards a
// specific way the merge could silently regress: timer's "restore" field
// has an explicit default in Home Assistant's STORAGE_FIELDS
// (vol.Optional(CONF_RESTORE, default=DEFAULT_RESTORE)), so "<platform>/list"
// always returns it - including the common case where it's false. The merge
// filters on `v != nil`, not `v != false`/`v != ""`/etc., specifically so a
// zero-value config field still round-trips; this test pins that down for
// the boolean case, where "reset to zero value" and "field absent" are easy
// to conflate.
func TestMergeCurrentHelperState_TimerRestoreFalseSurvivesMerge(t *testing.T) {
	t.Parallel()

	entityID := "timer.test_entity"
	client := &UniversalMockClient{
		GetHelperConfigFn: func(context.Context, string, string) (map[string]any, error) {
			return map[string]any{"duration": "0:05:00", "restore": false, "icon": "mdi:test"}, nil
		},
	}

	merged, _, err := mergeCurrentHelperState(context.Background(), client, entityID, "timer", helperTypes["timer"], map[string]any{})
	if err != nil {
		t.Fatalf("mergeCurrentHelperState returned err: %v", err)
	}

	if restore, present := merged["restore"]; !present || restore != false {
		t.Errorf(`merged["restore"] = %v (present=%v), want false (present=true) to survive the merge`, restore, present)
	}
	if merged["duration"] != "0:05:00" {
		t.Errorf(`merged["duration"] = %v, want "0:05:00"`, merged["duration"])
	}
}

// TestMergeCurrentHelperState_CounterRestoreFalseSurvivesMerge is counter's
// analog of TestMergeCurrentHelperState_TimerRestoreFalseSurvivesMerge:
// counter's STORAGE_FIELDS also declares
// vol.Optional(CONF_RESTORE, default=DEFAULT_RESTORE), so a counter created
// with restore=false must keep it on an update that doesn't mention restore
// at all, rather than having it silently reset to the default.
func TestMergeCurrentHelperState_CounterRestoreFalseSurvivesMerge(t *testing.T) {
	t.Parallel()

	entityID := "counter.test_entity"
	client := &UniversalMockClient{
		GetHelperConfigFn: func(context.Context, string, string) (map[string]any, error) {
			return map[string]any{"step": 1.0, "restore": false, "icon": "mdi:test"}, nil
		},
	}

	merged, _, err := mergeCurrentHelperState(context.Background(), client, entityID, "counter", helperTypes["counter"], map[string]any{"step": 2.0})
	if err != nil {
		t.Fatalf("mergeCurrentHelperState returned err: %v", err)
	}

	if restore, present := merged["restore"]; !present || restore != false {
		t.Errorf(`merged["restore"] = %v (present=%v), want false (present=true) to survive the merge`, restore, present)
	}
}

// TestMergeCurrentHelperState_EmptyStringArgDoesNotOverridePreservedField
// guards against the merge loop treating a caller-sent empty string as a
// real overriding value. argReader's own contract (helpers_arg_reader.go)
// declares empty string a universal "leave unset" spelling, same as an
// absent key or an explicit null - so a client following that contract to
// mean "don't touch unit_of_measurement" must not have it silently cleared
// just because the merge loop's override check only excluded nil.
func TestMergeCurrentHelperState_EmptyStringArgDoesNotOverridePreservedField(t *testing.T) {
	t.Parallel()

	entityID := "input_number.test_entity"
	client := &UniversalMockClient{
		GetHelperConfigFn: func(context.Context, string, string) (map[string]any, error) {
			return map[string]any{"min": 0.0, "max": 100.0, "unit_of_measurement": "%", "icon": "mdi:test"}, nil
		},
	}

	merged, _, err := mergeCurrentHelperState(context.Background(), client, entityID, "input_number", helperTypes["input_number"],
		map[string]any{"min": 0.0, "max": 100.0, "unit_of_measurement": ""})
	if err != nil {
		t.Fatalf("mergeCurrentHelperState returned err: %v", err)
	}

	if got := merged["unit_of_measurement"]; got != "%" {
		t.Errorf(`merged["unit_of_measurement"] = %q, want "%%" (empty-string arg must not override the preserved value)`, got)
	}
}

// TestValidateSingleField_DefaultCasePresentWrongTypedValueIsNotMisreportedAsMissing
// guards validateSingleField's default branch (used by every required field
// without a dedicated case, e.g. "filter", "min_max_type", "after_time",
// "heater_entity_id", ...): a value that IS present but isn't a string used
// to be silently coerced to "" via a failed type assertion and then
// reported as "is required" - a lie, since the caller did supply it, and a
// duplicate of the type error argReader.str already gives on the actual
// build*Config call. Presence must be checked independently of type; type
// checking is the builder's job.
func TestValidateSingleField_DefaultCasePresentWrongTypedValueIsNotMisreportedAsMissing(t *testing.T) {
	t.Parallel()

	if err := validateSingleField("filter", "filter", map[string]any{"filter": 5.0}); err != nil {
		t.Errorf(`validateSingleField("filter", present but non-string) = %v, want nil`, err)
	}

	if err := validateSingleField("filter", "filter", map[string]any{}); err == nil {
		t.Error(`validateSingleField("filter", absent) = nil, want an error for a genuinely missing required field`)
	}

	if err := validateSingleField("filter", "filter", map[string]any{"filter": ""}); err == nil {
		t.Error(`validateSingleField("filter", empty string) = nil, want an error - empty string is still "unset" for a required field`)
	}
}

// TestManageHelper_Update_CounterPreservesRestore is the end-to-end
// regression test for the counter.restore data-loss bug found in review:
// updating a counter without mentioning "restore" must not reset it to
// Home Assistant's default (true) for a counter that was configured with
// restore=false.
func TestManageHelper_Update_CounterPreservesRestore(t *testing.T) {
	t.Parallel()

	entityID := "counter.test_entity"
	var capturedConfig homeassistant.HelperConfig
	client := &UniversalMockClient{}
	client.GetHelperConfigFn = func(_ context.Context, _, _ string) (map[string]any, error) {
		return map[string]any{"name": "Test Counter", "step": 1.0, "restore": false}, nil
	}
	client.UpdateHelperFn = func(_ context.Context, _ string, cfg homeassistant.HelperConfig) error {
		capturedConfig = cfg
		return nil
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      50 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})

	h := NewConsolidatedHelperHandlers()
	result, err := h.handleManageHelper(ctx, client, map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"step":      2.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	if restore, ok := capturedConfig.Config["restore"].(bool); !ok || restore != false {
		t.Errorf("Config[\"restore\"] = %v (ok=%v), want false - update must not reset restore to HA's default", capturedConfig.Config["restore"], ok)
	}
}

// TestManageHelperTool_DescriptionListsUpdatableFields asserts the tool
// description documents per-type accepted update fields, generated from
// helperTypes, and that the summary line no longer omits "update".
// TestManageHelperTool_DescriptionListsUpdatableFields asserts structural
// properties of the generated per-type field list rather than hardcoding
// its exact contents (a hardcoded expectation would have hidden the W2
// inaccuracies this test replaces - it can't catch a field wrongly listed
// as updatable if the expectation was copy-pasted from the same wrong
// list): every helperTypes entry appears exactly once, no
// updateExcludedFields member ever leaks into a per-type line, and "icon"
// is hoisted out (documented once) rather than repeated 25 times.
func TestManageHelperTool_DescriptionListsUpdatableFields(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedHelperHandlers()
	desc := h.manageHelperTool().Description

	if !strings.Contains(desc, "Manage Home Assistant helpers - list, create, update, delete, or get details.") {
		t.Error("description summary line should mention update")
	}
	if !strings.Contains(desc, "icon is accepted on every type") {
		t.Error("description should document icon once instead of per-type")
	}

	generated := updatableFieldsDescription()
	if !strings.Contains(desc, generated) {
		t.Fatal("description does not embed updatableFieldsDescription()'s current output - tool wiring drifted")
	}

	seen := make(map[string]bool, len(helperTypes))
	for _, line := range strings.Split(strings.TrimPrefix(generated, "\n"), "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed generated line: %q", line)
		}
		name := strings.TrimPrefix(parts[0], "  - ")
		if seen[name] {
			t.Errorf("helper type %q listed more than once", name)
		}
		seen[name] = true

		if _, known := helperTypes[name]; !known {
			t.Errorf("generated line references unknown helper type %q", name)
		}

		for _, field := range strings.Split(parts[1], ", ") {
			if field == "icon" {
				t.Errorf("%q: icon should be hoisted out of per-type lines, not repeated", name)
			}
			if isUpdateExcludedField(name, field) {
				t.Errorf("%q: excluded field %q leaked into the generated description", name, field)
			}
		}
	}

	for name := range helperTypes {
		if !seen[name] {
			t.Errorf("helper type %q missing from generated description", name)
		}
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

// =============================================================================
// Helper ID Slugification Tests
// =============================================================================

func TestSlugifyName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "test",
			expected: "test",
		},
		{
			name:     "uppercase to lowercase",
			input:    "TEST",
			expected: "test",
		},
		{
			name:     "spaces to underscores",
			input:    "MCP Test Boolean",
			expected: "mcp_test_boolean",
		},
		{
			name:     "special characters removed",
			input:    "Test! @Helper# $%",
			expected: "test_helper",
		},
		{
			name:     "multiple spaces collapsed",
			input:    "test   spaces",
			expected: "test_spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := slugifyName(tt.input)
			if got != tt.expected {
				t.Errorf("slugifyName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHelperCreate_SuccessMessageUsesSlugifiedName(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "counter with name that needs slugification - entity ID from id",
			args: map[string]any{
				"action": "create",
				"type":   "counter",
				"id":     "mcp_test_bool",
				"name":   "MCP Test Boolean",
			},
			wantContains: []string{"counter.mcp_test_bool"},
		},
		{
			name: "boolean with special characters in name - entity ID from id",
			args: map[string]any{
				"action": "create",
				"type":   "input_boolean",
				"id":     "test_bool",
				"name":   "Test! Boolean@",
			},
			wantContains: []string{"input_boolean.test_bool"},
		},
		{
			name: "text with spaces in name - entity ID from id",
			args: map[string]any{
				"action": "create",
				"type":   "input_text",
				"id":     "text_id",
				"name":   "My   Test   Text",
			},
			wantContains: []string{"input_text.text_id"},
		},
	}

	h := NewConsolidatedHelperHandlers()
	runHandlerTestCases(t, tests, h.handleManageHelper)
}

// TestHelperCreate_FullConfigOnRename verifies that when a WebSocket helper's id
// differs from the slugified name (triggering the create-then-update path), the
// internal update call forwards ALL type-specific required fields — not just the
// display name. Regression test covering input_number and input_datetime.
func TestHelperCreate_FullConfigOnRename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          map[string]any
		requiredField string // field that MUST appear in the UpdateHelper payload
	}{
		{
			name: "input_number rename forwards min and max",
			args: map[string]any{
				"action": "create",
				"type":   "input_number",
				"id":     "num_ev_start",
				"name":   "EV Session Energy Start",
				"min":    float64(0),
				"max":    float64(9999),
				"step":   float64(0.001),
			},
			requiredField: "min",
		},
		{
			name: "input_datetime rename forwards has_date and has_time",
			args: map[string]any{
				"action":   "create",
				"type":     "input_datetime",
				"id":       "dt_ev_start",
				"name":     "EV Session Start Datetime",
				"has_date": true,
				"has_time": true,
			},
			requiredField: "has_date",
		},
		{
			name: "input_select rename forwards options",
			args: map[string]any{
				"action":  "create",
				"type":    "input_select",
				"id":      "sel_ev_vehicle",
				"name":    "EV Session Vehicle Type",
				"options": []any{"Ioniq 6", "Twingo", "Unbekannt"},
			},
			requiredField: "options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var updatePayload map[string]any
			m := &UniversalMockClient{}
			m.CreateHelperFn = func(_ context.Context, _ homeassistant.HelperConfig) error {
				return nil
			}
			m.UpdateHelperFn = func(_ context.Context, _ string, config homeassistant.HelperConfig) error {
				updatePayload = config.Config
				return nil
			}

			h := NewConsolidatedHelperHandlers()
			result, err := h.handleManageHelper(context.Background(), m, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != nil && result.IsError {
				text := ""
				if len(result.Content) > 0 {
					text = result.Content[0].Text
				}
				t.Fatalf("handler returned error result: %s", text)
			}

			if updatePayload == nil {
				t.Fatal("UpdateHelper was not called; expected create-then-update path to fire (id != slugified name)")
			}
			if _, ok := updatePayload[tt.requiredField]; !ok {
				t.Errorf("UpdateHelper payload missing required field %q; got keys: %v", tt.requiredField, helperPayloadKeys(updatePayload))
			}
			if name, ok := updatePayload["name"].(string); !ok || name != tt.args["name"].(string) {
				t.Errorf("UpdateHelper payload has wrong name: want %q, got %v", tt.args["name"], updatePayload["name"])
			}
		})
	}
}

// helperPayloadKeys returns the keys of a config map for diagnostic messages.
func helperPayloadKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

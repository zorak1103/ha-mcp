// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
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
					if helperID != "test_number" {
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
					if helperID != "mode" {
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
// display name. Regression test for issues #70 (input_number) and #71 (input_datetime).
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

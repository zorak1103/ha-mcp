// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Test constants to avoid goconst warnings
const (
	testSchemaTypeObject     = "object"
	testRequiredAutomationID = "automation_id"
)

// mockAutomationClient is a thin wrapper for backward compatibility.
// New tests should use UniversalMockClient directly.
type mockAutomationClient struct {
	homeassistant.Client

	// Convenience fields for common test setups
	automations    []homeassistant.Automation
	automationsErr error

	automation    *homeassistant.Automation
	automationMap map[string]*homeassistant.Automation
	automationErr error

	createErr error
	updateErr error
	deleteErr error
	toggleErr error
}

func (m *mockAutomationClient) ListAutomations(_ context.Context) ([]homeassistant.Automation, error) {
	if m.automationsErr != nil {
		return nil, m.automationsErr
	}
	return m.automations, nil
}

func (m *mockAutomationClient) GetAutomation(_ context.Context, automationID string) (*homeassistant.Automation, error) {
	if m.automationErr != nil {
		return nil, m.automationErr
	}
	if m.automationMap != nil {
		if auto, ok := m.automationMap[automationID]; ok {
			return auto, nil
		}
	}
	return m.automation, nil
}

func (m *mockAutomationClient) CreateAutomation(_ context.Context, _ homeassistant.AutomationConfig) error {
	return m.createErr
}

func (m *mockAutomationClient) UpdateAutomation(_ context.Context, _ string, _ homeassistant.AutomationConfig) error {
	return m.updateErr
}

func (m *mockAutomationClient) DeleteAutomation(_ context.Context, _ string) error {
	return m.deleteErr
}

func (m *mockAutomationClient) ToggleAutomation(_ context.Context, _ string, _ bool) error {
	return m.toggleErr
}

func TestNewAutomationHandlers(t *testing.T) {
	h := NewAutomationHandlers()
	if h == nil {
		t.Fatal("NewAutomationHandlers() returned nil")
	}
}

func TestAutomationHandlersRegisterTools(t *testing.T) {
	h := NewAutomationHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify all expected tools are registered
	tools := registry.ListTools()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"list_automations",
		"get_automation",
		"create_automation",
		"update_automation",
		"delete_automation",
		"toggle_automation",
	}
	for _, toolName := range expectedTools {
		if !toolNames[toolName] {
			t.Errorf("Expected tool %q to be registered", toolName)
		}
	}
}

func TestListAutomationsTool(t *testing.T) {
	h := &AutomationHandlers{}
	tool := h.listAutomationsTool()

	if tool.Name != "list_automations" {
		t.Errorf("Expected tool name 'list_automations', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}
	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("Expected input schema type %q, got %q", testSchemaTypeObject, tool.InputSchema.Type)
	}

	// Check expected properties exist
	expectedProps := []string{"state", "alias", "verbose"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Expected property %q in input schema", prop)
		}
	}
}

func TestGetAutomationTool(t *testing.T) {
	h := &AutomationHandlers{}
	tool := h.getAutomationTool()

	if tool.Name != "get_automation" {
		t.Errorf("Expected tool name 'get_automation', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check automation_id is required
	if len(tool.InputSchema.Required) == 0 || tool.InputSchema.Required[0] != testRequiredAutomationID {
		t.Errorf("Expected %q to be required", testRequiredAutomationID)
	}
}

func TestCreateAutomationTool(t *testing.T) {
	h := &AutomationHandlers{}
	tool := h.createAutomationTool()

	if tool.Name != "create_automation" {
		t.Errorf("Expected tool name 'create_automation', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check expected properties exist
	expectedProps := []string{"alias", "description", "trigger", "condition", "action", "mode"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Expected property %q in input schema", prop)
		}
	}

	// Check required fields
	requiredFields := map[string]bool{"alias": false, "trigger": false, "action": false}
	for _, req := range tool.InputSchema.Required {
		requiredFields[req] = true
	}
	for field, found := range requiredFields {
		if !found {
			t.Errorf("Expected %q to be required", field)
		}
	}
}

func TestUpdateAutomationTool(t *testing.T) {
	h := &AutomationHandlers{}
	tool := h.updateAutomationTool()

	if tool.Name != "update_automation" {
		t.Errorf("Expected tool name 'update_automation', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check expected properties exist
	expectedProps := []string{"automation_id", "alias", "description", "trigger", "condition", "action", "mode"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Expected property %q in input schema", prop)
		}
	}

	// Check automation_id is required
	if len(tool.InputSchema.Required) == 0 || tool.InputSchema.Required[0] != testRequiredAutomationID {
		t.Errorf("Expected %q to be required", testRequiredAutomationID)
	}
}

func TestDeleteAutomationTool(t *testing.T) {
	h := &AutomationHandlers{}
	tool := h.deleteAutomationTool()

	if tool.Name != "delete_automation" {
		t.Errorf("Expected tool name 'delete_automation', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check automation_id is required
	if len(tool.InputSchema.Required) == 0 || tool.InputSchema.Required[0] != testRequiredAutomationID {
		t.Errorf("Expected %q to be required", testRequiredAutomationID)
	}
}

func TestToggleAutomationTool(t *testing.T) {
	h := &AutomationHandlers{}
	tool := h.toggleAutomationTool()

	if tool.Name != "toggle_automation" {
		t.Errorf("Expected tool name 'toggle_automation', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check expected properties exist
	expectedProps := []string{"automation_id", "enabled"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Expected property %q in input schema", prop)
		}
	}

	// Check required fields
	requiredFields := map[string]bool{"automation_id": false, "enabled": false}
	for _, req := range tool.InputSchema.Required {
		requiredFields[req] = true
	}
	for field, found := range requiredFields {
		if !found {
			t.Errorf("Expected %q to be required", field)
		}
	}
}

func TestHandleListAutomations(t *testing.T) {
	testAutomations := []homeassistant.Automation{
		{
			EntityID:      "automation.turn_on_lights",
			State:         "on",
			FriendlyName:  "Turn On Lights",
			LastTriggered: "2024-01-15T10:30:00Z",
		},
		{
			EntityID:      "automation.turn_off_lights",
			State:         "off",
			FriendlyName:  "Turn Off Lights",
			LastTriggered: "2024-01-14T22:00:00Z",
		},
		{
			EntityID:      "automation.morning_routine",
			State:         "on",
			FriendlyName:  "Morning Routine",
			LastTriggered: "2024-01-15T07:00:00Z",
		},
		{
			EntityID:      "automation.night_mode",
			State:         "on",
			FriendlyName:  "Night Mode Activation",
			LastTriggered: "2024-01-15T21:00:00Z",
		},
	}

	tests := []struct {
		name                string
		args                map[string]any
		client              *mockAutomationClient
		wantError           bool
		wantAutomationCount int
		wantContains        []string
		wantNotContains     []string
	}{
		{
			name: "success - no filters, compact output",
			args: map[string]any{},
			client: &mockAutomationClient{
				automations: testAutomations,
			},
			wantError:           false,
			wantAutomationCount: 4,
			wantContains: []string{
				"automation.turn_on_lights",
				"Turn On Lights",
				"Found 4 automations",
			},
		},
		{
			name: "success - filter by state on",
			args: map[string]any{"state": "on"},
			client: &mockAutomationClient{
				automations: testAutomations,
			},
			wantError:           false,
			wantAutomationCount: 3,
			wantContains: []string{
				"automation.turn_on_lights",
				"automation.morning_routine",
				"automation.night_mode",
				"Found 3 automations",
			},
			wantNotContains: []string{
				"automation.turn_off_lights",
			},
		},
		{
			name: "success - filter by state off",
			args: map[string]any{"state": "off"},
			client: &mockAutomationClient{
				automations: testAutomations,
			},
			wantError:           false,
			wantAutomationCount: 1,
			wantContains: []string{
				"automation.turn_off_lights",
				"Found 1 automations",
			},
			wantNotContains: []string{
				"automation.turn_on_lights",
				"automation.morning_routine",
			},
		},
		{
			name: "success - filter by alias case-insensitive",
			args: map[string]any{"alias": "LIGHTS"},
			client: &mockAutomationClient{
				automations: testAutomations,
			},
			wantError:           false,
			wantAutomationCount: 2,
			wantContains: []string{
				"automation.turn_on_lights",
				"automation.turn_off_lights",
				"Found 2 automations",
			},
			wantNotContains: []string{
				"automation.morning_routine",
				"automation.night_mode",
			},
		},
		{
			name: "success - filter by alias partial match",
			args: map[string]any{"alias": "morning"},
			client: &mockAutomationClient{
				automations: testAutomations,
			},
			wantError:           false,
			wantAutomationCount: 1,
			wantContains: []string{
				"automation.morning_routine",
				"Morning Routine",
				"Found 1 automations",
			},
			wantNotContains: []string{
				"automation.turn_on_lights",
			},
		},
		{
			name: "success - combined filters state and alias",
			args: map[string]any{"state": "on", "alias": "night"},
			client: &mockAutomationClient{
				automations: testAutomations,
			},
			wantError:           false,
			wantAutomationCount: 1,
			wantContains: []string{
				"automation.night_mode",
				"Found 1 automations",
			},
			wantNotContains: []string{
				"automation.turn_on_lights",
				"automation.turn_off_lights",
			},
		},
		{
			name: "success - verbose output",
			args: map[string]any{"verbose": true},
			client: &mockAutomationClient{
				automations: testAutomations,
				automationMap: map[string]*homeassistant.Automation{
					"turn_on_lights": {
						EntityID:      "automation.turn_on_lights",
						State:         "on",
						FriendlyName:  "Turn On Lights",
						LastTriggered: "2024-01-15T10:30:00Z",
						Config: &homeassistant.AutomationConfig{
							ID:       "turn_on_lights",
							Alias:    "Turn On Lights",
							Mode:     "single",
							Triggers: []any{map[string]any{"platform": "state"}},
							Actions:  []any{map[string]any{"service": "light.turn_on"}},
						},
					},
					"turn_off_lights": {
						EntityID:      "automation.turn_off_lights",
						State:         "off",
						FriendlyName:  "Turn Off Lights",
						LastTriggered: "2024-01-14T22:00:00Z",
						Config: &homeassistant.AutomationConfig{
							ID:       "turn_off_lights",
							Alias:    "Turn Off Lights",
							Mode:     "single",
							Triggers: []any{map[string]any{"platform": "state"}},
							Actions:  []any{map[string]any{"service": "light.turn_off"}},
						},
					},
					"morning_routine": {
						EntityID:      "automation.morning_routine",
						State:         "on",
						FriendlyName:  "Morning Routine",
						LastTriggered: "2024-01-15T07:00:00Z",
						Config: &homeassistant.AutomationConfig{
							ID:       "morning_routine",
							Alias:    "Morning Routine",
							Mode:     "single",
							Triggers: []any{map[string]any{"platform": "time"}},
							Actions:  []any{map[string]any{"service": "scene.activate"}},
						},
					},
					"night_mode": {
						EntityID:      "automation.night_mode",
						State:         "on",
						FriendlyName:  "Night Mode Activation",
						LastTriggered: "2024-01-15T21:00:00Z",
						Config: &homeassistant.AutomationConfig{
							ID:       "night_mode",
							Alias:    "Night Mode Activation",
							Mode:     "single",
							Triggers: []any{map[string]any{"platform": "time"}},
							Actions:  []any{map[string]any{"service": "scene.activate"}},
						},
					},
				},
			},
			wantError:           false,
			wantAutomationCount: 4,
			wantContains: []string{
				"automation.turn_on_lights",
				"entity_id",
				"state",
				"friendly_name",
				"last_triggered",
				"config",
				"triggers",
				"actions",
			},
		},
		{
			name: "success - empty result",
			args: map[string]any{"alias": "nonexistent"},
			client: &mockAutomationClient{
				automations: testAutomations,
			},
			wantError:           false,
			wantAutomationCount: 0,
			wantContains:        []string{"Found 0 automations"},
		},
		{
			name: "success - empty automation list",
			args: map[string]any{},
			client: &mockAutomationClient{
				automations: []homeassistant.Automation{},
			},
			wantError:           false,
			wantAutomationCount: 0,
			wantContains:        []string{"Found 0 automations"},
		},
		{
			name: "error - client error",
			args: map[string]any{},
			client: &mockAutomationClient{
				automationsErr: errors.New("connection refused"),
			},
			wantError:    true,
			wantContains: []string{"Error listing automations", "connection refused"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AutomationHandlers{}

			result, err := h.handleListAutomations(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleListAutomations() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleListAutomations() returned no content")
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, got: %s", want, content)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, got: %s", notWant, content)
				}
			}
		})
	}
}

func TestHandleGetAutomation(t *testing.T) {
	testAutomation := &homeassistant.Automation{
		EntityID:      "automation.test_automation",
		State:         "on",
		FriendlyName:  "Test Automation",
		LastTriggered: "2024-01-15T10:30:00Z",
		Config: &homeassistant.AutomationConfig{
			ID:          "test_automation",
			Alias:       "Test Automation",
			Description: "A test automation",
			Mode:        "single",
			Triggers:    []any{map[string]any{"platform": "state"}},
			Actions:     []any{map[string]any{"service": "light.turn_on"}},
		},
	}

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name: "success - returns automation details",
			args: map[string]any{"automation_id": "test_automation"},
			client: &mockAutomationClient{
				automation: testAutomation,
			},
			wantError: false,
			wantContains: []string{
				"automation.test_automation",
				"Test Automation",
				"single",
				"light.turn_on",
			},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - empty automation_id",
			args:         map[string]any{"automation_id": ""},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{"automation_id": "nonexistent"},
			client: &mockAutomationClient{
				automationErr: errors.New("automation not found"),
			},
			wantError:    true,
			wantContains: []string{"Error getting automation", "automation not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AutomationHandlers{}

			result, err := h.handleGetAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleGetAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetAutomation() returned no content")
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, got: %s", want, content)
				}
			}
		})
	}
}

func TestHandleCreateAutomation(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name: "success - creates automation",
			args: map[string]any{
				"alias":   "Turn On Living Room Lights",
				"trigger": []any{map[string]any{"platform": "state", "entity_id": "binary_sensor.motion"}},
				"action":  []any{map[string]any{"service": "light.turn_on", "entity_id": "light.living_room"}},
			},
			client:       &mockAutomationClient{},
			wantError:    false,
			wantContains: []string{"Automation 'Turn On Living Room Lights' created successfully", "turn_on_living_room_lights"},
		},
		{
			name: "success - creates automation with all optional fields",
			args: map[string]any{
				"alias":       "Full Automation",
				"description": "A complete automation",
				"trigger":     []any{map[string]any{"platform": "state"}},
				"condition":   []any{map[string]any{"condition": "state"}},
				"action":      []any{map[string]any{"service": "light.turn_on"}},
				"mode":        "restart",
			},
			client:       &mockAutomationClient{},
			wantError:    false,
			wantContains: []string{"Automation 'Full Automation' created successfully"},
		},
		{
			name: "error - missing alias",
			args: map[string]any{
				"trigger": []any{map[string]any{"platform": "state"}},
				"action":  []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"alias is required"},
		},
		{
			name: "error - empty alias",
			args: map[string]any{
				"alias":   "",
				"trigger": []any{map[string]any{"platform": "state"}},
				"action":  []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"alias is required"},
		},
		{
			name: "error - missing trigger",
			args: map[string]any{
				"alias":  "Test Automation",
				"action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"trigger is required"},
		},
		{
			name: "error - empty trigger",
			args: map[string]any{
				"alias":   "Test Automation",
				"trigger": []any{},
				"action":  []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"trigger is required"},
		},
		{
			name: "error - missing action",
			args: map[string]any{
				"alias":   "Test Automation",
				"trigger": []any{map[string]any{"platform": "state"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"action is required"},
		},
		{
			name: "error - empty action",
			args: map[string]any{
				"alias":   "Test Automation",
				"trigger": []any{map[string]any{"platform": "state"}},
				"action":  []any{},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"action is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{
				"alias":   "Test Automation",
				"trigger": []any{map[string]any{"platform": "state"}},
				"action":  []any{map[string]any{"service": "light.turn_on"}},
			},
			client: &mockAutomationClient{
				createErr: errors.New("failed to create automation"),
			},
			wantError:    true,
			wantContains: []string{"Error creating automation", "failed to create automation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AutomationHandlers{}

			result, err := h.handleCreateAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleCreateAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleCreateAutomation() returned no content")
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, got: %s", want, content)
				}
			}
		})
	}
}

func TestHandleUpdateAutomation(t *testing.T) {
	existingAutomation := &homeassistant.Automation{
		EntityID:     "automation.test_automation",
		State:        "on",
		FriendlyName: "Test Automation",
		Config: &homeassistant.AutomationConfig{
			ID:          "test_automation",
			Alias:       "Test Automation",
			Description: "Original description",
			Mode:        "single",
			Triggers:    []any{map[string]any{"platform": "state"}},
			Actions:     []any{map[string]any{"service": "light.turn_on"}},
		},
	}

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name: "success - updates alias",
			args: map[string]any{
				"automation_id": "test_automation",
				"alias":         "Updated Automation",
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name: "success - updates description",
			args: map[string]any{
				"automation_id": "test_automation",
				"description":   "New description",
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name: "success - updates trigger",
			args: map[string]any{
				"automation_id": "test_automation",
				"trigger":       []any{map[string]any{"platform": "time"}},
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name: "success - updates condition",
			args: map[string]any{
				"automation_id": "test_automation",
				"condition":     []any{map[string]any{"condition": "state"}},
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name: "success - updates action",
			args: map[string]any{
				"automation_id": "test_automation",
				"action":        []any{map[string]any{"service": "switch.turn_on"}},
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name: "success - updates mode",
			args: map[string]any{
				"automation_id": "test_automation",
				"mode":          "parallel",
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name: "success - updates multiple fields",
			args: map[string]any{
				"automation_id": "test_automation",
				"alias":         "New Alias",
				"description":   "New description",
				"mode":          "restart",
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name: "success - handles nil config",
			args: map[string]any{
				"automation_id": "test_automation",
				"alias":         "New Alias",
			},
			client: &mockAutomationClient{
				automation: &homeassistant.Automation{
					EntityID:     "automation.test_automation",
					State:        "on",
					FriendlyName: "Test Automation",
					Config:       nil,
				},
			},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' updated successfully"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - empty automation_id",
			args:         map[string]any{"automation_id": ""},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name: "error - automation not found",
			args: map[string]any{"automation_id": "nonexistent"},
			client: &mockAutomationClient{
				automationErr: errors.New("automation not found"),
			},
			wantError:    true,
			wantContains: []string{"Error getting current automation", "automation not found"},
		},
		{
			name: "error - update fails",
			args: map[string]any{
				"automation_id": "test_automation",
				"alias":         "New Alias",
			},
			client: &mockAutomationClient{
				automation: existingAutomation,
				updateErr:  errors.New("failed to update"),
			},
			wantError:    true,
			wantContains: []string{"Error updating automation", "failed to update"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AutomationHandlers{}

			result, err := h.handleUpdateAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleUpdateAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleUpdateAutomation() returned no content")
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, got: %s", want, content)
				}
			}
		})
	}
}

func TestHandleDeleteAutomation(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name:         "success - deletes automation",
			args:         map[string]any{"automation_id": "test_automation"},
			client:       &mockAutomationClient{},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' deleted successfully"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - empty automation_id",
			args:         map[string]any{"automation_id": ""},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{"automation_id": "test_automation"},
			client: &mockAutomationClient{
				deleteErr: errors.New("failed to delete automation"),
			},
			wantError:    true,
			wantContains: []string{"Error deleting automation", "failed to delete automation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AutomationHandlers{}

			result, err := h.handleDeleteAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleDeleteAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleDeleteAutomation() returned no content")
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, got: %s", want, content)
				}
			}
		})
	}
}

func TestHandleToggleAutomation(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name:         "success - enables automation",
			args:         map[string]any{"automation_id": "test_automation", "enabled": true},
			client:       &mockAutomationClient{},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' enabled successfully"},
		},
		{
			name:         "success - disables automation",
			args:         map[string]any{"automation_id": "test_automation", "enabled": false},
			client:       &mockAutomationClient{},
			wantError:    false,
			wantContains: []string{"Automation 'test_automation' disabled successfully"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{"enabled": true},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - empty automation_id",
			args:         map[string]any{"automation_id": "", "enabled": true},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - missing enabled",
			args:         map[string]any{"automation_id": "test_automation"},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"enabled is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{"automation_id": "test_automation", "enabled": true},
			client: &mockAutomationClient{
				toggleErr: errors.New("failed to toggle automation"),
			},
			wantError:    true,
			wantContains: []string{"Error toggling automation", "failed to toggle automation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AutomationHandlers{}

			result, err := h.handleToggleAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleToggleAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleToggleAutomation() returned no content")
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, got: %s", want, content)
				}
			}
		})
	}
}

// Test helper functions

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		key      string
		expected string
	}{
		{
			name:     "existing string key",
			args:     map[string]any{"key": "value"},
			key:      "key",
			expected: "value",
		},
		{
			name:     "non-existing key",
			args:     map[string]any{"other": "value"},
			key:      "key",
			expected: "",
		},
		{
			name:     "non-string value",
			args:     map[string]any{"key": 123},
			key:      "key",
			expected: "",
		},
		{
			name:     "nil map",
			args:     nil,
			key:      "key",
			expected: "",
		},
		{
			name:     "empty string value",
			args:     map[string]any{"key": ""},
			key:      "key",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.args, tt.key)
			if result != tt.expected {
				t.Errorf("getString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetSlice(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		key         string
		expectNil   bool
		expectCount int
	}{
		{
			name:        "existing slice key",
			args:        map[string]any{"key": []any{"a", "b", "c"}},
			key:         "key",
			expectNil:   false,
			expectCount: 3,
		},
		{
			name:        "non-existing key",
			args:        map[string]any{"other": []any{"a"}},
			key:         "key",
			expectNil:   true,
			expectCount: 0,
		},
		{
			name:        "non-slice value",
			args:        map[string]any{"key": "string"},
			key:         "key",
			expectNil:   true,
			expectCount: 0,
		},
		{
			name:        "nil map",
			args:        nil,
			key:         "key",
			expectNil:   true,
			expectCount: 0,
		},
		{
			name:        "empty slice value",
			args:        map[string]any{"key": []any{}},
			key:         "key",
			expectNil:   false,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSlice(tt.args, tt.key)
			if tt.expectNil {
				if result != nil {
					t.Errorf("getSlice() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Error("getSlice() = nil, want non-nil")
				} else if len(result) != tt.expectCount {
					t.Errorf("getSlice() len = %d, want %d", len(result), tt.expectCount)
				}
			}
		})
	}
}

// Tests for new refactored helper functions

func TestParseAutomationFilters(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		expected automationFilters
	}{
		{name: "empty args", args: map[string]any{}, expected: automationFilters{}},
		{
			name:     "all filters set",
			args:     map[string]any{"state": "on", "alias": "test", "entity_id": "light.living_room"},
			expected: automationFilters{state: "on", alias: "test", entityID: "light.living_room"},
		},
		{name: "partial filters", args: map[string]any{"state": "off"}, expected: automationFilters{state: "off"}},
		{
			name:     "non-string values ignored",
			args:     map[string]any{"state": 123, "alias": "test", "entity_id": true},
			expected: automationFilters{alias: "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAutomationFilters(tt.args)
			if result != tt.expected {
				t.Errorf("parseAutomationFilters() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestMatchesStateFilter(t *testing.T) {
	auto := homeassistant.Automation{State: "on"}
	tests := []struct {
		name        string
		stateFilter string
		want        bool
	}{
		{name: "empty filter matches all", stateFilter: "", want: true},
		{name: "matching state", stateFilter: "on", want: true},
		{name: "non-matching state", stateFilter: "off", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesStateFilter(auto, tt.stateFilter); got != tt.want {
				t.Errorf("matchesStateFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesAliasFilter(t *testing.T) {
	tests := []struct {
		name         string
		friendlyName string
		aliasFilter  string
		want         bool
	}{
		{name: "empty filter matches all", friendlyName: "Test Auto", aliasFilter: "", want: true},
		{name: "exact match", friendlyName: "Test", aliasFilter: "Test", want: true},
		{name: "partial match", friendlyName: "Turn On Lights", aliasFilter: "on", want: true},
		{name: "case insensitive", friendlyName: "Turn On Lights", aliasFilter: "LIGHTS", want: true},
		{name: "no match", friendlyName: "Turn On Lights", aliasFilter: "switch", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auto := homeassistant.Automation{FriendlyName: tt.friendlyName}
			if got := matchesAliasFilter(auto, tt.aliasFilter); got != tt.want {
				t.Errorf("matchesAliasFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesEntityIDFilter(t *testing.T) {
	tests := []struct {
		name           string
		config         *homeassistant.AutomationConfig
		entityIDFilter string
		want           bool
	}{
		{name: "empty filter matches all", config: nil, entityIDFilter: "", want: true},
		{name: "nil config with filter", config: nil, entityIDFilter: "light.test", want: false},
		{
			name:           "entity in triggers",
			config:         &homeassistant.AutomationConfig{Triggers: []any{map[string]any{"entity_id": "light.test"}}},
			entityIDFilter: "light.test",
			want:           true,
		},
		{
			name:           "entity in actions",
			config:         &homeassistant.AutomationConfig{Actions: []any{map[string]any{"entity_id": "switch.test"}}},
			entityIDFilter: "switch.test",
			want:           true,
		},
		{
			name:           "entity in conditions",
			config:         &homeassistant.AutomationConfig{Conditions: []any{map[string]any{"entity_id": "sensor.test"}}},
			entityIDFilter: "sensor.test",
			want:           true,
		},
		{
			name: "entity not found",
			config: &homeassistant.AutomationConfig{
				Triggers: []any{map[string]any{"entity_id": "light.bedroom"}},
				Actions:  []any{map[string]any{"entity_id": "switch.kitchen"}},
			},
			entityIDFilter: "sensor.temperature",
			want:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesEntityIDFilter(tt.config, tt.entityIDFilter); got != tt.want {
				t.Errorf("matchesEntityIDFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutomationFilters_NeedsConfigForFiltering(t *testing.T) {
	tests := []struct {
		name    string
		filters automationFilters
		want    bool
	}{
		{name: "no entity_id filter", filters: automationFilters{state: "on", alias: "test"}, want: false},
		{name: "with entity_id filter", filters: automationFilters{entityID: "light.test"}, want: true},
		{name: "empty filters", filters: automationFilters{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filters.needsConfigForFiltering(); got != tt.want {
				t.Errorf("needsConfigForFiltering() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCompactAutomationOutput(t *testing.T) {
	automations := []homeassistant.Automation{
		{EntityID: "automation.test1", State: "on", FriendlyName: "Test 1", LastTriggered: "2024-01-15T10:00:00Z"},
		{EntityID: "automation.test2", State: "off", FriendlyName: "Test 2"},
	}
	output, err := buildCompactAutomationOutput(automations)
	if err != nil {
		t.Fatalf("buildCompactAutomationOutput() error = %v", err)
	}
	result := string(output)
	expectedStrings := []string{"automation.test1", "on", "Test 1", "automation.test2", "off"}
	for _, expected := range expectedStrings {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected output to contain %q", expected)
		}
	}
}

func TestBuildCompactAutomationOutput_Empty(t *testing.T) {
	output, err := buildCompactAutomationOutput([]homeassistant.Automation{})
	if err != nil {
		t.Fatalf("buildCompactAutomationOutput() error = %v", err)
	}
	if result := string(output); result != "[]" {
		t.Errorf("Expected empty array '[]', got: %s", result)
	}
}

func TestBuildAutomationSummary(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		verbose bool
		want    string
	}{
		{name: "compact with zero", count: 0, verbose: false, want: "Found 0 automations"},
		{name: "compact with count", count: 5, verbose: false, want: "Found 5 automations"},
		{name: "verbose", count: 3, verbose: true, want: "Found 3 automations"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAutomationSummary(tt.count, tt.verbose)
			if !strings.Contains(got, tt.want) {
				t.Errorf("buildAutomationSummary() = %q, want to contain %q", got, tt.want)
			}
			if !tt.verbose && !strings.Contains(got, "verbose") {
				t.Errorf("buildAutomationSummary() compact should contain verbose hint")
			}
		})
	}
}

func TestGenerateAutomationID(t *testing.T) {
	tests := []struct {
		name     string
		alias    string
		expected string
	}{
		{
			name:     "simple lowercase",
			alias:    "test",
			expected: "test",
		},
		{
			name:     "uppercase to lowercase",
			alias:    "TEST",
			expected: "test",
		},
		{
			name:     "mixed case",
			alias:    "TeSt AuToMaTiOn",
			expected: "test_automation",
		},
		{
			name:     "spaces to underscores",
			alias:    "Turn On Living Room Lights",
			expected: "turn_on_living_room_lights",
		},
		{
			name:     "hyphens to underscores",
			alias:    "turn-on-lights",
			expected: "turn_on_lights",
		},
		{
			name:     "underscores preserved",
			alias:    "turn_on_lights",
			expected: "turn_on_lights",
		},
		{
			name:     "numbers preserved",
			alias:    "Test 123 Automation",
			expected: "test_123_automation",
		},
		{
			name:     "special characters removed",
			alias:    "Test! @Automation# $%",
			expected: "test_automation",
		},
		{
			name:     "multiple spaces collapsed",
			alias:    "test   multiple   spaces",
			expected: "test_multiple_spaces",
		},
		{
			name:     "leading special characters",
			alias:    "   Test Automation",
			expected: "test_automation",
		},
		{
			name:     "trailing special characters",
			alias:    "Test Automation   ",
			expected: "test_automation",
		},
		{
			name:     "mixed separators",
			alias:    "Test - Automation _ Name",
			expected: "test_automation_name",
		},
		{
			name:     "empty string",
			alias:    "",
			expected: "",
		},
		{
			name:     "only special characters",
			alias:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "unicode letters",
			alias:    "Tëst Àutomàtion",
			expected: "tëst_àutomàtion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateAutomationID(tt.alias)
			if result != tt.expected {
				t.Errorf("generateAutomationID(%q) = %q, want %q", tt.alias, result, tt.expected)
			}
		})
	}
}

func TestCompactAutomationEntryJSON(t *testing.T) {
	// Test that compactAutomationEntry JSON serialization works correctly
	entry := compactAutomationEntry{
		EntityID:      "automation.test",
		State:         "on",
		Alias:         "Test Automation",
		LastTriggered: "2024-01-15T10:30:00Z",
	}

	// Verify the struct can be marshaled
	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "automation.test") {
		t.Errorf("Expected entity_id in output, got: %s", result)
	}
	if !strings.Contains(result, "on") {
		t.Errorf("Expected state in output, got: %s", result)
	}
	if !strings.Contains(result, "Test Automation") {
		t.Errorf("Expected alias in output, got: %s", result)
	}
}

func TestCompactAutomationEntryOmitsEmpty(t *testing.T) {
	entry := compactAutomationEntry{
		EntityID: "automation.test",
		State:    "on",
		// Alias and LastTriggered intentionally empty
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	// Check that empty fields with omitempty are not present
	if strings.Contains(result, "alias") && strings.Contains(result, `"alias":""`) {
		// omitempty should remove empty strings
		t.Errorf("Expected empty alias to be omitted, got: %s", result)
	}
}

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
type mockAutomationClient struct {
	homeassistant.Client

	automations    []homeassistant.Automation
	automationsErr error

	automation    *homeassistant.Automation
	automationMap map[string]*homeassistant.Automation
	automationErr error

	createErr error
	updateErr error
	deleteErr error
	toggleErr error

	// Capture IDs and configs passed to methods
	lastGetID         string
	lastUpdateID      string
	lastDeleteID      string
	lastToggleID      string
	lastCreatedConfig *homeassistant.AutomationConfig

	// entityExists tracks whether the mock entity is currently "visible".
	// Set to true after successful CreateAutomation, false after successful DeleteAutomation.
	entityExists bool

	// updateCalled tracks whether UpdateAutomation was invoked (used by dry-run tests).
	updateCalled bool
}

func (m *mockAutomationClient) ListAutomations(_ context.Context) ([]homeassistant.Automation, error) {
	if m.automationsErr != nil {
		return nil, m.automationsErr
	}
	return m.automations, nil
}

func (m *mockAutomationClient) GetAutomation(_ context.Context, automationID string) (*homeassistant.Automation, error) {
	m.lastGetID = automationID
	if m.automationErr != nil {
		return nil, m.automationErr
	}
	if m.automationMap != nil {
		if auto, ok := m.automationMap[automationID]; ok {
			return auto, nil
		}
		// If map exists but key not found, return not found error
		return nil, errors.New("automation not found")
	}
	return m.automation, nil
}

func (m *mockAutomationClient) CreateAutomation(_ context.Context, config homeassistant.AutomationConfig) error {
	if m.createErr == nil {
		configCopy := config
		m.lastCreatedConfig = &configCopy
		m.entityExists = true
	}
	return m.createErr
}

func (m *mockAutomationClient) UpdateAutomation(_ context.Context, automationID string, _ homeassistant.AutomationConfig) error {
	m.lastUpdateID = automationID
	m.updateCalled = true
	return m.updateErr
}

func (m *mockAutomationClient) DeleteAutomation(_ context.Context, automationID string) error {
	m.lastDeleteID = automationID
	if m.deleteErr == nil {
		m.entityExists = false
	}
	return m.deleteErr
}

func (m *mockAutomationClient) ToggleAutomation(_ context.Context, automationID string, _ bool) error {
	m.lastToggleID = automationID
	return m.toggleErr
}

// GetState returns the entity if entityExists is true, otherwise returns not-found error.
// This allows waitForEntityAppear (after create) and waitForEntityDisappear (after delete)
// to resolve immediately without real timeouts.
func (m *mockAutomationClient) GetState(_ context.Context, entityID string) (*homeassistant.Entity, error) {
	if !m.entityExists {
		return nil, errors.New("entity not found")
	}
	return &homeassistant.Entity{EntityID: entityID, State: "on"}, nil
}

// CallService is a no-op for reload calls triggered by create/delete operations.
func (m *mockAutomationClient) CallService(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
	return nil, nil
}

func TestNewAutomationHandlers(t *testing.T) {
	t.Parallel()

	h := NewAutomationHandlers()
	if h == nil {
		t.Fatal("NewAutomationHandlers() returned nil")
	}
}

func TestAutomationHandlersRegisterTools(t *testing.T) {
	t.Parallel()

	h := NewAutomationHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 1 // manage_automation only
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	if tools[0].Name != "manage_automation" {
		t.Errorf("Expected tool name 'manage_automation', got %q", tools[0].Name)
	}
}

func TestManageAutomationTool_Schema(t *testing.T) {
	t.Parallel()

	h := &AutomationHandlers{}
	tool := h.manageAutomationTool()

	if tool.Name != "manage_automation" {
		t.Errorf("Expected tool name 'manage_automation', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}
	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("Expected input schema type %q, got %q", testSchemaTypeObject, tool.InputSchema.Type)
	}

	// Check expected properties exist
	expectedProps := []string{"action", "automation_id", "alias", "trigger", "condition", "automation_action", "mode", "enabled", "state", "verbose", "limit", "cursor", "format"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Expected property %q in input schema", prop)
		}
	}

	// action is required
	if len(tool.InputSchema.Required) != 1 || tool.InputSchema.Required[0] != "action" {
		t.Error("Expected 'action' to be the only required field")
	}
}

func TestManageAutomation_List(t *testing.T) {
	t.Parallel()

	testAutomations := []homeassistant.Automation{
		{EntityID: "automation.turn_on_lights", State: "on", FriendlyName: "Turn On Lights", LastTriggered: "2024-01-15T10:30:00Z"},
		{EntityID: "automation.turn_off_lights", State: "off", FriendlyName: "Turn Off Lights", LastTriggered: "2024-01-14T22:00:00Z"},
		{EntityID: "automation.morning_routine", State: "on", FriendlyName: "Morning Routine", LastTriggered: "2024-01-15T07:00:00Z"},
	}

	tests := []struct {
		name            string
		args            map[string]any
		client          *mockAutomationClient
		wantError       bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "success - no filters (json)",
			args:         map[string]any{"action": "list", "format": "json"},
			client:       &mockAutomationClient{automations: testAutomations},
			wantContains: []string{"automation.turn_on_lights"},
		},
		{
			name:         "success - no filters (natural)",
			args:         map[string]any{"action": "list"},
			client:       &mockAutomationClient{automations: testAutomations},
			wantContains: []string{"3 automations", "enabled", "disabled"},
		},
		{
			name:            "success - filter by state (json)",
			args:            map[string]any{"action": "list", "state": "on", "format": "json"},
			client:          &mockAutomationClient{automations: testAutomations},
			wantContains:    []string{"automation.turn_on_lights", "automation.morning_routine"},
			wantNotContains: []string{"automation.turn_off_lights"},
		},
		{
			name:            "success - filter by alias (json)",
			args:            map[string]any{"action": "list", "alias": "lights", "format": "json"},
			client:          &mockAutomationClient{automations: testAutomations},
			wantContains:    []string{"automation.turn_on_lights", "automation.turn_off_lights"},
			wantNotContains: []string{"automation.morning_routine"},
		},
		{
			name:         "error - client error",
			args:         map[string]any{"action": "list"},
			client:       &mockAutomationClient{automationsErr: errors.New("connection refused")},
			wantError:    true,
			wantContains: []string{"Error listing automations"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AutomationHandlers{}
			result, err := h.handleManageAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q", want)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q", notWant)
				}
			}
		})
	}
}

func TestManageAutomation_Get(t *testing.T) {
	t.Parallel()

	testAutomation := &homeassistant.Automation{
		EntityID:     "automation.test_automation",
		State:        "on",
		FriendlyName: "Test Automation",
		Config:       &homeassistant.AutomationConfig{ID: "test_automation", Alias: "Test Automation"},
	}

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name:         "success (json)",
			args:         map[string]any{"action": "get", "automation_id": "test_automation", "format": "json"},
			client:       &mockAutomationClient{automation: testAutomation},
			wantContains: []string{"automation.test_automation", "Test Automation"},
		},
		{
			name:         "success (natural)",
			args:         map[string]any{"action": "get", "automation_id": "test_automation"},
			client:       &mockAutomationClient{automation: testAutomation},
			wantContains: []string{"Test Automation", "enabled"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{"action": "get"},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - client error",
			args:         map[string]any{"action": "get", "automation_id": "nonexistent"},
			client:       &mockAutomationClient{automationErr: errors.New("not found")},
			wantError:    true,
			wantContains: []string{"Error getting automation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AutomationHandlers{}
			result, err := h.handleManageAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q", want)
				}
			}
		})
	}
}

func TestManageAutomation_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name: "success",
			args: map[string]any{
				"action":            "create",
				"alias":             "Turn On Living Room Lights",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantContains: []string{"created successfully", "turn_on_living_room_lights"},
		},
		{
			name: "error - missing alias",
			args: map[string]any{
				"action":            "create",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"alias is required"},
		},
		{
			name: "error - missing trigger",
			args: map[string]any{
				"action":            "create",
				"alias":             "Test",
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"trigger is required"},
		},
		{
			name: "error - missing automation_action",
			args: map[string]any{
				"action":  "create",
				"alias":   "Test",
				"trigger": []any{map[string]any{"platform": "state"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_action is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{
				"action":            "create",
				"alias":             "Test",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{createErr: errors.New("failed")},
			wantError:    true,
			wantContains: []string{"Error creating automation"},
		},
		{
			name: "success - empty trigger array creates manual-only automation",
			args: map[string]any{
				"action":            "create",
				"alias":             "Manual Only Test",
				"trigger":           []any{},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantContains: []string{"created successfully", "manual-only", "placeholder trigger inserted"},
		},
		{
			name: "error - empty automation_action array",
			args: map[string]any{
				"action":            "create",
				"alias":             "Test",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"at least one action"},
		},
		{
			name: "create with explicit automation_id",
			args: map[string]any{
				"action":            "create",
				"alias":             "Wärme Büro",
				"automation_id":     "warme_buro",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantContains: []string{"created successfully", "entity_id: automation.warme_buro", "config_id: warme_buro"},
		},
		{
			name: "create with prefixed automation_id strips prefix",
			args: map[string]any{
				"action":            "create",
				"alias":             "My Automation",
				"automation_id":     "automation.my_id",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantContains: []string{"created successfully", "entity_id: automation.my_automation", "config_id: my_id"},
		},
		{
			name: "error - invalid automation_id with spaces",
			args: map[string]any{
				"action":            "create",
				"alias":             "My Automation",
				"automation_id":     "Mein Ding",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id must contain only"},
		},
		{
			name: "create without automation_id falls back to alias",
			args: map[string]any{
				"action":            "create",
				"alias":             "Turn Off All Lights",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_off"}},
			},
			client:       &mockAutomationClient{},
			wantContains: []string{"created successfully", "entity_id: automation.turn_off_all_lights", "config_id: turn_off_all_lights"},
		},
		{
			name: "create with mismatched automation_id and alias shows both ids",
			args: map[string]any{
				"action":            "create",
				"alias":             "Spülmaschine bei Solarüberschuss einschalten",
				"automation_id":     "dishwasher_solar_surplus",
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "switch.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantContains: []string{"entity_id: automation.spulmaschine_bei_solaruberschuss_einschalten", "config_id: dishwasher_solar_surplus"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AutomationHandlers{}
			result, err := h.handleManageAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q", want)
				}
			}
		})
	}
}

func TestManageAutomation_Create_PlaceholderTriggerVerification(t *testing.T) {
	t.Parallel()

	client := &mockAutomationClient{}
	h := &AutomationHandlers{}

	args := map[string]any{
		"action":            "create",
		"alias":             "Manual Only Test",
		"trigger":           []any{},
		"automation_action": []any{map[string]any{"service": "light.turn_on"}},
	}

	result, err := h.handleManageAutomation(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleManageAutomation() unexpected error = %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.Content[0].Text)
	}

	// Verify placeholder trigger was inserted
	if client.lastCreatedConfig == nil {
		t.Fatal("Expected config to be captured")
	}

	if len(client.lastCreatedConfig.Triggers) != 1 {
		t.Errorf("Expected 1 trigger, got %d", len(client.lastCreatedConfig.Triggers))
	}

	// Verify it's the placeholder trigger
	trigger, ok := client.lastCreatedConfig.Triggers[0].(map[string]any)
	if !ok {
		t.Fatal("Expected trigger to be map[string]any")
	}

	if trigger["event_type"] != "ha_mcp_manual_only_placeholder" {
		t.Errorf("Expected placeholder trigger, got %v", trigger)
	}
}

func TestResolveAutomationTriggers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		wantTriggers   []any
		wantAutoFilled bool
	}{
		{
			name:           "missing trigger key",
			args:           map[string]any{},
			wantTriggers:   nil,
			wantAutoFilled: false,
		},
		{
			name:           "empty trigger array",
			args:           map[string]any{"trigger": []any{}},
			wantTriggers:   manualOnlyTrigger,
			wantAutoFilled: true,
		},
		{
			name: "non-empty trigger array",
			args: map[string]any{
				"trigger": []any{map[string]any{"platform": "state"}},
			},
			wantTriggers:   []any{map[string]any{"platform": "state"}},
			wantAutoFilled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			triggers, autoFilled := resolveAutomationTriggers(tt.args)

			if autoFilled != tt.wantAutoFilled {
				t.Errorf("autoFilled = %v, want %v", autoFilled, tt.wantAutoFilled)
			}

			if tt.wantTriggers == nil {
				if triggers != nil {
					t.Errorf("Expected nil triggers, got %v", triggers)
				}
			} else {
				if len(triggers) != len(tt.wantTriggers) {
					t.Errorf("len(triggers) = %d, want %d", len(triggers), len(tt.wantTriggers))
				}
			}
		})
	}
}

func TestManageAutomation_Update(t *testing.T) {
	t.Parallel()

	// Helper to create a fresh automation for each test to avoid data races
	newExistingAutomation := func() *homeassistant.Automation {
		return &homeassistant.Automation{
			EntityID: "automation.test_automation",
			State:    "on",
			Config:   &homeassistant.AutomationConfig{ID: "test_automation", Alias: "Test Automation"},
		}
	}

	tests := []struct {
		name         string
		args         map[string]any
		setupClient  func() *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name:         "success",
			args:         map[string]any{"action": "update", "automation_id": "test_automation", "alias": "Updated"},
			setupClient:  func() *mockAutomationClient { return &mockAutomationClient{automation: newExistingAutomation()} },
			wantContains: []string{"updated successfully"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{"action": "update"},
			setupClient:  func() *mockAutomationClient { return &mockAutomationClient{} },
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - automation not found",
			args:         map[string]any{"action": "update", "automation_id": "nonexistent"},
			setupClient:  func() *mockAutomationClient { return &mockAutomationClient{automationErr: errors.New("not found")} },
			wantError:    true,
			wantContains: []string{"Error getting current automation"},
		},
		{
			name: "error - update fails",
			args: map[string]any{"action": "update", "automation_id": "test_automation", "alias": "New"},
			setupClient: func() *mockAutomationClient {
				return &mockAutomationClient{automation: newExistingAutomation(), updateErr: errors.New("failed")}
			},
			wantError:    true,
			wantContains: []string{"Error updating automation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AutomationHandlers{}
			result, err := h.handleManageAutomation(context.Background(), tt.setupClient(), tt.args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q", want)
				}
			}
		})
	}
}

func TestManageAutomation_Delete(t *testing.T) {
	t.Parallel()

	// Default test automation
	defaultAutomation := &homeassistant.Automation{
		EntityID:     "automation.test",
		State:        "on",
		FriendlyName: "Test Automation",
		Config: &homeassistant.AutomationConfig{
			ID:    "test",
			Alias: "Test Automation",
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
			name:         "success",
			args:         map[string]any{"action": "delete", "automation_id": "test_automation"},
			client:       &mockAutomationClient{automation: defaultAutomation},
			wantContains: []string{"deleted successfully"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{"action": "delete"},
			client:       &mockAutomationClient{automation: defaultAutomation},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - client error",
			args:         map[string]any{"action": "delete", "automation_id": "test"},
			client:       &mockAutomationClient{automation: defaultAutomation, deleteErr: errors.New("failed")},
			wantError:    true,
			wantContains: []string{"Error deleting automation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AutomationHandlers{}
			result, err := h.handleManageAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q", want)
				}
			}
		})
	}
}

func TestManageAutomation_Toggle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name:         "success - enable",
			args:         map[string]any{"action": "toggle", "automation_id": "test", "enabled": true},
			client:       &mockAutomationClient{},
			wantContains: []string{"enabled successfully"},
		},
		{
			name:         "success - disable",
			args:         map[string]any{"action": "toggle", "automation_id": "test", "enabled": false},
			client:       &mockAutomationClient{},
			wantContains: []string{"disabled successfully"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{"action": "toggle", "enabled": true},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - missing enabled",
			args:         map[string]any{"action": "toggle", "automation_id": "test"},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"enabled is required"},
		},
		{
			name:         "error - client error",
			args:         map[string]any{"action": "toggle", "automation_id": "test", "enabled": true},
			client:       &mockAutomationClient{toggleErr: errors.New("failed")},
			wantError:    true,
			wantContains: []string{"Error toggling automation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AutomationHandlers{}
			result, err := h.handleManageAutomation(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q", want)
				}
			}
		})
	}
}

func TestManageAutomation_InvalidAction(t *testing.T) {
	t.Parallel()

	h := &AutomationHandlers{}
	client := &mockAutomationClient{}

	// Test missing action
	result, err := h.handleManageAutomation(context.Background(), client, map[string]any{})
	if err != nil {
		t.Fatalf("handleManageAutomation() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for missing action")
	}
	if !strings.Contains(result.Content[0].Text, "action is required") {
		t.Errorf("Expected 'action is required' error, got: %s", result.Content[0].Text)
	}

	// Test invalid action
	result, err = h.handleManageAutomation(context.Background(), client, map[string]any{"action": "invalid"})
	if err != nil {
		t.Fatalf("handleManageAutomation() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid action")
	}
	if !strings.Contains(result.Content[0].Text, "invalid action") {
		t.Errorf("Expected 'invalid action' error, got: %s", result.Content[0].Text)
	}
}

func TestManageAutomation_Patch(t *testing.T) {
	t.Parallel()

	baseConfig := &homeassistant.AutomationConfig{
		ID:       "morning_routine",
		Alias:    "Morning Routine",
		Mode:     "single",
		Triggers: []any{map[string]any{"trigger": "time", "at": "07:00"}},
		Actions:  []any{map[string]any{"action": "light.turn_on"}},
	}

	h := &AutomationHandlers{}

	tests := []handlerTestCase{
		{
			name: "patch - missing automation_id",
			args: map[string]any{
				"action":     "patch",
				"operations": []any{map[string]any{"op": "replace", "path": "/mode", "value": "queued"}},
			},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name: "patch - missing operations",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
			},
			wantError:    true,
			wantContains: []string{"operations is required"},
		},
		{
			name: "patch - empty operations",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations":    []any{},
			},
			wantError:    true,
			wantContains: []string{"must contain at least one"},
		},
		{
			name: "patch - success replace mode",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
				m.UpdateAutomationFn = func(_ context.Context, _ string, _ homeassistant.AutomationConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully", "1 operations"},
		},
		{
			name: "patch - success add action",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{"op": "add", "path": "/actions/-", "value": map[string]any{"action": "light.turn_off"}},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
				m.UpdateAutomationFn = func(_ context.Context, _ string, _ homeassistant.AutomationConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "patch - invalid op returns error",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{"op": "invalid_op", "path": "/mode"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"invalid operation"},
		},
		{
			name: "patch - bad path returns error",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/nonexistent_field/0", "value": "x"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"error applying patch"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageAutomation)
}

func TestManageAutomation_SemanticPatch(t *testing.T) {
	t.Parallel()

	baseConfig := &homeassistant.AutomationConfig{
		ID:    "morning_routine",
		Alias: "Morning Routine",
		Mode:  "single",
		Triggers: []any{
			map[string]any{"platform": "state", "entity_id": "binary_sensor.door", "to": "on"},
			map[string]any{"platform": "time", "at": "07:00"},
		},
		Conditions: []any{
			map[string]any{"condition": "state", "entity_id": "input_boolean.vacation", "state": "off"},
		},
		Actions: []any{
			map[string]any{"action": "light.turn_on"},
		},
	}

	h := &AutomationHandlers{}

	tests := []handlerTestCase{
		{
			name: "semantic patch - add for to trigger by entity_id",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{
						"op":      "add",
						"match":   map[string]any{"entity_id": "binary_sensor.door"},
						"section": "triggers",
						"field":   "for",
						"value":   "00:05:00",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
				m.UpdateAutomationFn = func(_ context.Context, _ string, _ homeassistant.AutomationConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "semantic patch - replace condition state",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{
						"op":      "replace",
						"match":   map[string]any{"entity_id": "input_boolean.vacation"},
						"section": "conditions",
						"field":   "state",
						"value":   "on",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
				m.UpdateAutomationFn = func(_ context.Context, _ string, _ homeassistant.AutomationConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "semantic patch - no match returns error",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{
						"op":      "add",
						"match":   map[string]any{"entity_id": "binary_sensor.nonexistent"},
						"section": "triggers",
						"field":   "for",
						"value":   "00:05:00",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"error applying patch", "no elements"},
		},
		{
			name: "semantic patch - move op rejected",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{
						"op":      "move",
						"match":   map[string]any{"entity_id": "binary_sensor.door"},
						"section": "triggers",
						"field":   "for",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"move/copy"},
		},
		{
			name: "semantic patch - missing section rejected",
			args: map[string]any{
				"action":        "patch",
				"automation_id": "morning_routine",
				"operations": []any{
					map[string]any{
						"op":    "replace",
						"match": map[string]any{"entity_id": "binary_sensor.door"},
						"field": "for",
						"value": "x",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAutomationFn = func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					cfg := *baseConfig
					return &homeassistant.Automation{EntityID: "automation.morning_routine", Config: &cfg}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"'section' is required"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageAutomation)
}

// Test helper functions

func TestGetString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, key, expected string
		args                map[string]any
	}{
		{"existing string key", "key", "value", map[string]any{"key": "value"}},
		{"non-existing key", "key", "", map[string]any{"other": "value"}},
		{"non-string value", "key", "", map[string]any{"key": 123}},
		{"nil map", "key", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := getString(tt.args, tt.key); got != tt.expected {
				t.Errorf("getString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      map[string]any
		key       string
		expectNil bool
		expectLen int
	}{
		{"existing slice", map[string]any{"key": []any{"a", "b"}}, "key", false, 2},
		{"non-existing key", map[string]any{"other": []any{"a"}}, "key", true, 0},
		{"non-slice value", map[string]any{"key": "string"}, "key", true, 0},
		{"nil map", nil, "key", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getSlice(tt.args, tt.key)
			if tt.expectNil && result != nil {
				t.Errorf("getSlice() = %v, want nil", result)
			}
			if !tt.expectNil && (result == nil || len(result) != tt.expectLen) {
				t.Errorf("getSlice() len = %d, want %d", len(result), tt.expectLen)
			}
		})
	}
}

func TestParseAutomationFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     map[string]any
		expected automationFilters
	}{
		{"empty args", map[string]any{}, automationFilters{}},
		{"all filters", map[string]any{"state": "on", "alias": "test", "entity_id": "light.test"}, automationFilters{state: "on", alias: "test", entityID: "light.test"}},
		{"partial filters", map[string]any{"state": "off"}, automationFilters{state: "off"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseAutomationFilters(tt.args); got != tt.expected {
				t.Errorf("parseAutomationFilters() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestMatchesStateFilter(t *testing.T) {
	t.Parallel()

	auto := homeassistant.Automation{State: "on"}
	tests := []struct {
		name        string
		stateFilter string
		want        bool
	}{
		{"empty filter", "", true},
		{"matching state", "on", true},
		{"non-matching state", "off", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := matchesStateFilter(auto, tt.stateFilter); got != tt.want {
				t.Errorf("matchesStateFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesAliasFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, friendlyName, aliasFilter string
		want                            bool
	}{
		{"empty filter", "Test", "", true},
		{"exact match", "Test", "Test", true},
		{"partial match", "Turn On Lights", "on", true},
		{"case insensitive", "Turn On Lights", "LIGHTS", true},
		{"no match", "Turn On Lights", "switch", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			auto := homeassistant.Automation{FriendlyName: tt.friendlyName}
			if got := matchesAliasFilter(auto, tt.aliasFilter); got != tt.want {
				t.Errorf("matchesAliasFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesEntityIDFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         *homeassistant.AutomationConfig
		entityIDFilter string
		want           bool
	}{
		{"empty filter", nil, "", true},
		{"nil config with filter", nil, "light.test", false},
		{"entity in triggers", &homeassistant.AutomationConfig{Triggers: []any{map[string]any{"entity_id": "light.test"}}}, "light.test", true},
		{"entity in actions", &homeassistant.AutomationConfig{Actions: []any{map[string]any{"entity_id": "switch.test"}}}, "switch.test", true},
		{"entity not found", &homeassistant.AutomationConfig{Triggers: []any{map[string]any{"entity_id": "light.bedroom"}}}, "sensor.temp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := matchesEntityIDFilter(tt.config, tt.entityIDFilter); got != tt.want {
				t.Errorf("matchesEntityIDFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutomationFilters_NeedsConfigForFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filters automationFilters
		want    bool
	}{
		{"no entity_id", automationFilters{state: "on"}, false},
		{"with entity_id", automationFilters{entityID: "light.test"}, true},
		{"empty filters", automationFilters{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.filters.needsConfigForFiltering(); got != tt.want {
				t.Errorf("needsConfigForFiltering() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCompactAutomationOutput(t *testing.T) {
	t.Parallel()

	automations := []homeassistant.Automation{
		{EntityID: "automation.test1", State: "on", FriendlyName: "Test 1"},
		{EntityID: "automation.test2", State: "off", FriendlyName: "Test 2"},
	}

	output, err := buildCompactAutomationOutput(automations)
	if err != nil {
		t.Fatalf("buildCompactAutomationOutput() error = %v", err)
	}

	result := string(output)
	for _, expected := range []string{"automation.test1", "on", "Test 1", "automation.test2", "off"} {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected output to contain %q", expected)
		}
	}
}

func TestBuildCompactAutomationOutput_Empty(t *testing.T) {
	t.Parallel()

	output, err := buildCompactAutomationOutput([]homeassistant.Automation{})
	if err != nil {
		t.Fatalf("buildCompactAutomationOutput() error = %v", err)
	}
	if string(output) != "[]" {
		t.Errorf("Expected empty array '[]', got: %s", output)
	}
}

func TestGenerateAutomationID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, alias, expected string
	}{
		{"simple lowercase", "test", "test"},
		{"uppercase to lowercase", "TEST", "test"},
		{"spaces to underscores", "Turn On Lights", "turn_on_lights"},
		{"hyphens to underscores", "turn-on-lights", "turn_on_lights"},
		{"numbers preserved", "Test 123", "test_123"},
		{"special characters removed", "Test! @Auto# $%", "test_auto"},
		{"multiple spaces collapsed", "test   spaces", "test_spaces"},
		{"empty string", "", ""},
		{"only special characters", "!@#$%", ""},
		{"german umlauts transliterated", "Wärme Büro", "warme_buro"},
		{"issue 58 alias", "Spülmaschine bei Solarüberschuss einschalten", "spulmaschine_bei_solaruberschuss_einschalten"},
		{"accented latin", "Café résumé", "cafe_resume"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := generateAutomationID(tt.alias); got != tt.expected {
				t.Errorf("generateAutomationID(%q) = %q, want %q", tt.alias, got, tt.expected)
			}
		})
	}
}

func TestCompactAutomationEntryJSON(t *testing.T) {
	t.Parallel()

	entry := compactAutomationEntry{
		EntityID:      "automation.test",
		State:         "on",
		Alias:         "Test Automation",
		LastTriggered: "2024-01-15T10:30:00Z",
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	for _, expected := range []string{"automation.test", "on", "Test Automation"} {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected output to contain %q", expected)
		}
	}
}

func TestFindAutomationByAlias(t *testing.T) {
	t.Parallel()

	automationWithAlias := &homeassistant.Automation{
		EntityID:     "automation.morning_lights",
		State:        "on",
		FriendlyName: "Morning Lights Routine",
		Config: &homeassistant.AutomationConfig{
			ID:    "morning_lights",
			Alias: "Morning Lights Routine",
		},
	}

	tests := []struct {
		name          string
		searchID      string
		automations   []homeassistant.Automation
		automationMap map[string]*homeassistant.Automation
		wantFound     bool
		wantEntityID  string
	}{
		{
			name:     "find by alias - exact match",
			searchID: "Morning Lights Routine",
			automations: []homeassistant.Automation{
				{EntityID: "automation.morning_lights", State: "on", FriendlyName: "Morning Lights Routine"},
			},
			automationMap: map[string]*homeassistant.Automation{
				"morning_lights": automationWithAlias,
			},
			wantFound:    true,
			wantEntityID: "automation.morning_lights",
		},
		{
			name:     "find by alias - partial match",
			searchID: "morning lights",
			automations: []homeassistant.Automation{
				{EntityID: "automation.morning_lights", State: "on", FriendlyName: "Morning Lights Routine"},
			},
			automationMap: map[string]*homeassistant.Automation{
				"morning_lights": automationWithAlias,
			},
			wantFound:    true,
			wantEntityID: "automation.morning_lights",
		},
		{
			name:     "find by alias - case insensitive",
			searchID: "MORNING LIGHTS",
			automations: []homeassistant.Automation{
				{EntityID: "automation.morning_lights", State: "on", FriendlyName: "Morning Lights Routine"},
			},
			automationMap: map[string]*homeassistant.Automation{
				"morning_lights": automationWithAlias,
			},
			wantFound:    true,
			wantEntityID: "automation.morning_lights",
		},
		{
			name:     "find by friendly_name - partial match",
			searchID: "Routine",
			automations: []homeassistant.Automation{
				{EntityID: "automation.morning_lights", State: "on", FriendlyName: "Morning Lights Routine"},
			},
			automationMap: map[string]*homeassistant.Automation{
				"morning_lights": automationWithAlias,
			},
			wantFound:    true,
			wantEntityID: "automation.morning_lights",
		},
		{
			name:     "not found - no matching alias or friendly_name",
			searchID: "nonexistent",
			automations: []homeassistant.Automation{
				{EntityID: "automation.morning_lights", State: "on", FriendlyName: "Morning Lights Routine"},
			},
			automationMap: map[string]*homeassistant.Automation{
				"morning_lights": automationWithAlias,
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockAutomationClient{
				automations:   tt.automations,
				automationMap: tt.automationMap,
			}

			h := &AutomationHandlers{}
			result, err := h.findAutomationByID(context.Background(), client, tt.searchID)

			if tt.wantFound {
				if err != nil {
					t.Errorf("findAutomationByID() unexpected error = %v", err)
					return
				}
				if result == nil {
					t.Error("findAutomationByID() returned nil, want automation")
					return
				}
				if result.EntityID != tt.wantEntityID {
					t.Errorf("findAutomationByID() EntityID = %q, want %q", result.EntityID, tt.wantEntityID)
				}
			} else {
				if err == nil {
					t.Error("findAutomationByID() expected error, got nil")
				}
			}
		})
	}
}

func TestManageAutomation_GetByFriendlyName(t *testing.T) {
	t.Parallel()

	testAutomation := &homeassistant.Automation{
		EntityID:     "automation.test_automation",
		State:        "on",
		FriendlyName: "Test Morning Routine",
		Config: &homeassistant.AutomationConfig{
			ID:    "test_automation",
			Alias: "Test Morning Routine",
		},
	}

	tests := []struct {
		name         string
		args         map[string]any
		setupClient  func() *mockAutomationClient
		wantError    bool
		wantContains []string
	}{
		{
			name: "get by alias - partial match",
			args: map[string]any{"action": "get", "automation_id": "morning routine"},
			setupClient: func() *mockAutomationClient {
				// Don't set automationErr - use automationMap only to return proper results for valid IDs
				return &mockAutomationClient{
					automations: []homeassistant.Automation{
						{EntityID: "automation.test_automation", State: "on", FriendlyName: "Test Morning Routine"},
					},
					automationMap: map[string]*homeassistant.Automation{
						"test_automation": testAutomation,
					},
				}
			},
			wantContains: []string{"Test Morning Routine"},
		},
		{
			name: "get by friendly_name - case insensitive",
			args: map[string]any{"action": "get", "automation_id": "TEST MORNING"},
			setupClient: func() *mockAutomationClient {
				// Don't set automationErr - use automationMap only
				return &mockAutomationClient{
					automations: []homeassistant.Automation{
						{EntityID: "automation.test_automation", State: "on", FriendlyName: "Test Morning Routine"},
					},
					automationMap: map[string]*homeassistant.Automation{
						"test_automation": testAutomation,
					},
				}
			},
			wantContains: []string{"Test Morning Routine"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &AutomationHandlers{}
			result, err := h.handleManageAutomation(context.Background(), tt.setupClient(), tt.args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q", want)
				}
			}
		})
	}
}

func TestNormalizeAutomationID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantEntityID string
		wantConfigID string
	}{
		{
			name:         "with automation prefix",
			input:        "automation.test_auto",
			wantEntityID: "automation.test_auto",
			wantConfigID: "test_auto",
		},
		{
			name:         "without prefix",
			input:        "test_auto",
			wantEntityID: "automation.test_auto",
			wantConfigID: "test_auto",
		},
		{
			name:         "with prefix - complex ID",
			input:        "automation.my_morning_routine",
			wantEntityID: "automation.my_morning_routine",
			wantConfigID: "my_morning_routine",
		},
		{
			name:         "without prefix - complex ID",
			input:        "my_morning_routine",
			wantEntityID: "automation.my_morning_routine",
			wantConfigID: "my_morning_routine",
		},
		{
			name:         "empty string",
			input:        "",
			wantEntityID: "automation.",
			wantConfigID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotEntityID, gotConfigID := normalizeAutomationID(tt.input)
			if gotEntityID != tt.wantEntityID {
				t.Errorf("normalizeAutomationID(%q) entityID = %q, want %q", tt.input, gotEntityID, tt.wantEntityID)
			}
			if gotConfigID != tt.wantConfigID {
				t.Errorf("normalizeAutomationID(%q) configID = %q, want %q", tt.input, gotConfigID, tt.wantConfigID)
			}
		})
	}
}

func TestAutomationHandlers_IDNormalization(t *testing.T) {
	t.Parallel()

	// Factory function to create fresh automation instances per subtest
	newTestAutomation := func() *homeassistant.Automation {
		config := &homeassistant.AutomationConfig{
			ID:    "test_auto",
			Alias: "Test Automation",
		}
		return &homeassistant.Automation{
			EntityID:     "automation.test_auto",
			State:        "on",
			FriendlyName: "Test Automation",
			Config:       config,
		}
	}

	tests := []struct {
		name           string
		action         string
		inputID        string
		wantGetID      string
		wantUpdateID   string
		wantDeleteID   string
		wantToggleID   string
		additionalArgs map[string]any
	}{
		{
			name:      "get - with prefix",
			action:    "get",
			inputID:   "automation.test_auto",
			wantGetID: "test_auto",
		},
		{
			name:      "get - without prefix",
			action:    "get",
			inputID:   "test_auto",
			wantGetID: "test_auto",
		},
		{
			name:         "update - with prefix",
			action:       "update",
			inputID:      "automation.test_auto",
			wantGetID:    "test_auto",
			wantUpdateID: "test_auto",
			additionalArgs: map[string]any{
				"alias": "Updated Name",
			},
		},
		{
			name:         "update - without prefix",
			action:       "update",
			inputID:      "test_auto",
			wantGetID:    "test_auto",
			wantUpdateID: "test_auto",
			additionalArgs: map[string]any{
				"alias": "Updated Name",
			},
		},
		{
			name:         "delete - with prefix",
			action:       "delete",
			inputID:      "automation.test_auto",
			wantDeleteID: "test_auto",
		},
		{
			name:         "delete - without prefix",
			action:       "delete",
			inputID:      "test_auto",
			wantDeleteID: "test_auto",
		},
		{
			name:         "toggle - with prefix",
			action:       "toggle",
			inputID:      "automation.test_auto",
			wantToggleID: "automation.test_auto",
			additionalArgs: map[string]any{
				"enabled": true,
			},
		},
		{
			name:         "toggle - without prefix",
			action:       "toggle",
			inputID:      "test_auto",
			wantToggleID: "automation.test_auto",
			additionalArgs: map[string]any{
				"enabled": false,
			},
		},
	}

	// Run tests with matching IDs (current behavior)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create fresh automation instance for this subtest
			testAutomation := newTestAutomation()
			client := &mockAutomationClient{
				automation: testAutomation,
				automationMap: map[string]*homeassistant.Automation{
					"test_auto": testAutomation,
				},
			}

			h := &AutomationHandlers{}
			args := map[string]any{
				"action":        tt.action,
				"automation_id": tt.inputID,
			}
			for k, v := range tt.additionalArgs {
				args[k] = v
			}

			_, err := h.handleManageAutomation(context.Background(), client, args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			// Verify the correct ID format was passed to client methods
			if tt.wantGetID != "" && client.lastGetID != tt.wantGetID {
				t.Errorf("GetAutomation called with ID %q, want %q", client.lastGetID, tt.wantGetID)
			}
			if tt.wantUpdateID != "" && client.lastUpdateID != tt.wantUpdateID {
				t.Errorf("UpdateAutomation called with ID %q, want %q", client.lastUpdateID, tt.wantUpdateID)
			}
			if tt.wantDeleteID != "" && client.lastDeleteID != tt.wantDeleteID {
				t.Errorf("DeleteAutomation called with ID %q, want %q", client.lastDeleteID, tt.wantDeleteID)
			}
			if tt.wantToggleID != "" && client.lastToggleID != tt.wantToggleID {
				t.Errorf("ToggleAutomation called with ID %q, want %q", client.lastToggleID, tt.wantToggleID)
			}
		})
	}

	// NEW: Test cases with mismatched config IDs (UI-created automations)
	// These expose the bug where Config.ID differs from entity_id suffix
	mismatchTests := []struct {
		name           string
		action         string
		inputID        string
		entityID       string
		configID       string
		wantGetID      string
		wantUpdateID   string
		wantDeleteID   string
		additionalArgs map[string]any
	}{
		{
			name:         "update - UI automation with numeric config ID",
			action:       "update",
			inputID:      "automation.office_light_motion",
			entityID:     "automation.office_light_motion",
			configID:     "1702630691980",
			wantGetID:    "office_light_motion",
			wantUpdateID: "1702630691980", // Should use Config.ID, not entity suffix
			additionalArgs: map[string]any{
				"alias": "Updated Office Light",
			},
		},
		{
			name:         "update - UI automation without prefix in input",
			action:       "update",
			inputID:      "office_light_motion",
			entityID:     "automation.office_light_motion",
			configID:     "1702630691980",
			wantGetID:    "office_light_motion",
			wantUpdateID: "1702630691980",
			additionalArgs: map[string]any{
				"alias": "Updated Office Light",
			},
		},
		{
			name:         "delete - UI automation with numeric config ID",
			action:       "delete",
			inputID:      "automation.office_light_motion",
			entityID:     "automation.office_light_motion",
			configID:     "1702630691980",
			wantGetID:    "office_light_motion",
			wantDeleteID: "1702630691980", // Should use Config.ID
		},
		{
			name:         "delete - UI automation without prefix",
			action:       "delete",
			inputID:      "office_light_motion",
			entityID:     "automation.office_light_motion",
			configID:     "1702630691980",
			wantGetID:    "office_light_motion",
			wantDeleteID: "1702630691980",
		},
		{
			name:         "update - direct config ID input",
			action:       "update",
			inputID:      "1702630691980",
			entityID:     "automation.office_light_motion",
			configID:     "1702630691980",
			wantGetID:    "1702630691980",
			wantUpdateID: "1702630691980",
			additionalArgs: map[string]any{
				"alias": "Updated via Config ID",
			},
		},
		{
			name:         "update - YAML automation with matching IDs",
			action:       "update",
			inputID:      "automation.test_auto",
			entityID:     "automation.test_auto",
			configID:     "test_auto",
			wantGetID:    "test_auto",
			wantUpdateID: "test_auto", // Config.ID matches entity suffix
			additionalArgs: map[string]any{
				"alias": "Updated YAML Auto",
			},
		},
	}

	for _, tt := range mismatchTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create automation with mismatched config ID
			config := &homeassistant.AutomationConfig{
				ID:    tt.configID,
				Alias: "Test Automation",
			}
			testAutomation := &homeassistant.Automation{
				EntityID:     tt.entityID,
				State:        "on",
				FriendlyName: "Test Automation",
				Config:       config,
			}

			// Set up mock to return automation for various ID lookups
			automationMap := map[string]*homeassistant.Automation{
				strings.TrimPrefix(tt.entityID, "automation."): testAutomation,
				tt.configID: testAutomation,
			}

			client := &mockAutomationClient{
				automation:    testAutomation,
				automationMap: automationMap,
			}

			h := &AutomationHandlers{}
			args := map[string]any{
				"action":        tt.action,
				"automation_id": tt.inputID,
			}
			for k, v := range tt.additionalArgs {
				args[k] = v
			}

			_, err := h.handleManageAutomation(context.Background(), client, args)
			if err != nil {
				t.Fatalf("handleManageAutomation() unexpected error = %v", err)
			}

			// Verify the correct config ID was used for REST operations
			if tt.wantGetID != "" && client.lastGetID != tt.wantGetID {
				t.Errorf("GetAutomation called with ID %q, want %q", client.lastGetID, tt.wantGetID)
			}
			if tt.wantUpdateID != "" && client.lastUpdateID != tt.wantUpdateID {
				t.Errorf("UpdateAutomation called with ID %q, want %q", client.lastUpdateID, tt.wantUpdateID)
			}
			if tt.wantDeleteID != "" && client.lastDeleteID != tt.wantDeleteID {
				t.Errorf("DeleteAutomation called with ID %q, want %q", client.lastDeleteID, tt.wantDeleteID)
			}
		})
	}
}

// TestSafeDeref tests the safeDeref helper function.
func TestSafeDeref(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer returns empty string", func(t *testing.T) {
		t.Parallel()
		if got := safeDeref(nil); got != "" {
			t.Errorf("safeDeref(nil) = %q, want %q", got, "")
		}
	})

	t.Run("non-nil pointer returns value", func(t *testing.T) {
		t.Parallel()
		s := "hello"
		if got := safeDeref(&s); got != "hello" {
			t.Errorf("safeDeref(&s) = %q, want %q", got, "hello")
		}
	})
}

// TestFormatNaturalPaginationNote tests the formatNaturalPaginationNote helper.
func TestFormatNaturalPaginationNote(t *testing.T) {
	t.Parallel()

	cursor := "abc123"
	paginated := PaginatedResponse[homeassistant.Automation]{
		Items: make([]homeassistant.Automation, 2),
		Pagination: PaginationMetadata{
			Total:      10,
			NextCursor: &cursor,
		},
	}

	note := formatNaturalPaginationNote(paginated)
	if !strings.Contains(note, "2") {
		t.Errorf("note should contain item count, got %q", note)
	}
	if !strings.Contains(note, "10") {
		t.Errorf("note should contain total count, got %q", note)
	}
	if !strings.Contains(note, "abc123") {
		t.Errorf("note should contain cursor, got %q", note)
	}
}

// TestBuildPaginatedAutomationResponse tests buildPaginatedAutomationResponse.
func TestBuildPaginatedAutomationResponse(t *testing.T) {
	t.Parallel()

	t.Run("no pagination limit returns raw items", func(t *testing.T) {
		t.Parallel()
		items := json.RawMessage(`[{"id":"1"}]`)
		result := buildPaginatedAutomationResponse(PaginatedResponse[homeassistant.Automation]{
			Pagination: PaginationMetadata{Limit: 0},
		}, items)
		if string(result) != `[{"id":"1"}]` {
			t.Errorf("result = %s, want raw items", result)
		}
	})

	t.Run("with pagination wraps items", func(t *testing.T) {
		t.Parallel()
		items := json.RawMessage(`[{"id":"1"}]`)
		result := buildPaginatedAutomationResponse(PaginatedResponse[homeassistant.Automation]{
			Pagination: PaginationMetadata{Limit: 10, Total: 1},
		}, items)
		resultStr := string(result)
		if !strings.Contains(resultStr, "pagination") {
			t.Errorf("result should contain pagination, got %s", resultStr)
		}
	})
}

// TestApplyAutomationConfigUpdates_EmptyTrigger tests applyAutomationConfigUpdates with empty trigger array.
func TestApplyAutomationConfigUpdates_EmptyTrigger(t *testing.T) {
	t.Parallel()

	config := &homeassistant.AutomationConfig{
		Alias:    "Test",
		Triggers: []any{map[string]any{"trigger": "state", "entity_id": "light.test"}},
	}

	// Empty trigger array should use manual-only placeholder
	applyAutomationConfigUpdates(config, map[string]any{
		"trigger": []any{},
	})

	if len(config.Triggers) != 1 {
		t.Errorf("config.Triggers len = %d, want 1 (manual-only placeholder)", len(config.Triggers))
	}

	// Non-empty trigger replaces existing
	applyAutomationConfigUpdates(config, map[string]any{
		"trigger": []any{map[string]any{"trigger": "time", "at": "07:00"}},
	})
	if len(config.Triggers) != 1 {
		t.Errorf("config.Triggers len = %d, want 1", len(config.Triggers))
	}

	// Mode update
	applyAutomationConfigUpdates(config, map[string]any{
		"mode": "queued",
	})
	if config.Mode != "queued" {
		t.Errorf("config.Mode = %q, want 'queued'", config.Mode)
	}

	// Description update
	applyAutomationConfigUpdates(config, map[string]any{
		"description": "My automation",
	})
	if config.Description != "My automation" {
		t.Errorf("config.Description = %q, want 'My automation'", config.Description)
	}
}

// TestSearchInConfigSliceValue tests the searchInConfigSliceValue helper.
func TestSearchInConfigSliceValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		items    []any
		entityID string
		want     bool
	}{
		{
			name:     "empty slice returns false",
			items:    []any{},
			entityID: "light.test",
			want:     false,
		},
		{
			name:     "string match returns true",
			items:    []any{"light.test", "sensor.temp"},
			entityID: "light.test",
			want:     true,
		},
		{
			name:     "no match returns false",
			items:    []any{"sensor.temp", "switch.fan"},
			entityID: "light.test",
			want:     false,
		},
		{
			name: "nested map match",
			items: []any{
				map[string]any{"entity_id": "light.test"},
			},
			entityID: "light.test",
			want:     true,
		},
		{
			name: "nested slice match",
			items: []any{
				[]any{"light.test", "sensor.other"},
			},
			entityID: "light.test",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := searchInConfigSliceValue(tt.items, tt.entityID)
			if got != tt.want {
				t.Errorf("searchInConfigSliceValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSearchInConfigMapEntry tests searchInConfigMapEntry with target key.
func TestSearchInConfigMapEntry(t *testing.T) {
	t.Parallel()

	t.Run("target key with entity_id", func(t *testing.T) {
		t.Parallel()
		// "target" key should search for entity_id inside the target map
		val := map[string]any{
			"entity_id": "light.test",
		}
		got := searchInConfigMapEntry("target", val, "light.test")
		if !got {
			t.Error("searchInConfigMapEntry(target) should return true for matching entity_id")
		}
	})

	t.Run("target key no match", func(t *testing.T) {
		t.Parallel()
		val := map[string]any{
			"entity_id": "light.other",
		}
		got := searchInConfigMapEntry("target", val, "light.test")
		if got {
			t.Error("searchInConfigMapEntry(target) should return false for non-matching entity_id")
		}
	})

	t.Run("other key recurses into value", func(t *testing.T) {
		t.Parallel()
		got := searchInConfigMapEntry("entity_id", "light.test", "light.test")
		if !got {
			t.Error("searchInConfigMapEntry(entity_id) should return true for matching value")
		}
	})
}

// TestManageAutomation_List_WithPagination covers formatJSONPaginatedAutomations and formatNaturalPaginationNote.
func TestManageAutomation_List_WithPagination(t *testing.T) {
	t.Parallel()

	testAutomations := []homeassistant.Automation{
		{EntityID: "automation.first", State: "on", FriendlyName: "First"},
		{EntityID: "automation.second", State: "on", FriendlyName: "Second"},
		{EntityID: "automation.third", State: "on", FriendlyName: "Third"},
	}

	t.Run("json format with limit", func(t *testing.T) {
		t.Parallel()

		client := &mockAutomationClient{automations: testAutomations}
		h := &AutomationHandlers{}
		result, err := h.handleManageAutomation(context.Background(), client, map[string]any{
			"action": "list",
			"format": "json",
			"limit":  float64(2),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %v", result.Content)
		}
		content := result.Content[0].Text
		// Should be wrapped in pagination response
		if !strings.Contains(content, "pagination") {
			t.Errorf("paginated JSON should contain 'pagination', got: %s", content[:min(200, len(content))])
		}
	})

	t.Run("natural format with limit shows pagination note", func(t *testing.T) {
		t.Parallel()

		client := &mockAutomationClient{automations: testAutomations}
		h := &AutomationHandlers{}
		result, err := h.handleManageAutomation(context.Background(), client, map[string]any{
			"action": "list",
			"format": "natural",
			"limit":  float64(1),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %v", result.Content)
		}
		content := result.Content[0].Text
		// HasMore=true, should include pagination note
		if !strings.Contains(content, "Showing") {
			t.Errorf("paginated natural should contain 'Showing', got: %s", content[:min(300, len(content))])
		}
	})

	t.Run("json format verbose with limit", func(t *testing.T) {
		t.Parallel()

		client := &mockAutomationClient{
			automations: testAutomations,
			automationMap: map[string]*homeassistant.Automation{
				"first": {
					EntityID: "automation.first",
					State:    "on",
					Config:   &homeassistant.AutomationConfig{Alias: "First", ID: "uuid-first"},
				},
			},
		}
		h := &AutomationHandlers{}
		result, err := h.handleManageAutomation(context.Background(), client, map[string]any{
			"action":  "list",
			"format":  "json",
			"limit":   float64(1),
			"verbose": true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %v", result.Content)
		}
		content := result.Content[0].Text
		if !strings.Contains(content, "pagination") {
			t.Errorf("verbose paginated JSON should contain 'pagination', got: %s", content[:min(300, len(content))])
		}
	})
}

// TestFindAutomationByID_Alias tests findAutomationByID by alias/friendly_name.
func TestFindAutomationByID_Alias(t *testing.T) {
	t.Parallel()

	testAutomations := []homeassistant.Automation{
		{EntityID: "automation.morning", State: "on", FriendlyName: "Morning Routine"},
	}

	// newClient returns a fresh mock per subtest — prevents concurrent writes
	// to lastGetID when parallel subtests share a single *mockAutomationClient.
	newClient := func() *mockAutomationClient {
		return &mockAutomationClient{
			automations: testAutomations,
			automationMap: map[string]*homeassistant.Automation{
				"morning": {
					EntityID:     "automation.morning",
					State:        "on",
					FriendlyName: "Morning Routine",
					Config:       &homeassistant.AutomationConfig{Alias: "Morning Routine", ID: "uuid-abc"},
				},
			},
		}
	}

	h := &AutomationHandlers{}

	t.Run("find by alias", func(t *testing.T) {
		t.Parallel()
		auto, err := h.findAutomationByID(context.Background(), newClient(), "morning routine")
		if err != nil {
			t.Fatalf("findAutomationByID() error = %v", err)
		}
		if auto == nil {
			t.Fatal("expected automation, got nil")
		}
	})

	t.Run("find by entity_id prefix", func(t *testing.T) {
		t.Parallel()
		c := &mockAutomationClient{
			automations: testAutomations,
			automationMap: map[string]*homeassistant.Automation{
				"morning": {EntityID: "automation.morning", State: "on"},
			},
		}
		auto, err := h.findAutomationByID(context.Background(), c, "automation.morning")
		if err != nil {
			t.Fatalf("findAutomationByID() error = %v", err)
		}
		if auto == nil {
			t.Fatal("expected automation, got nil")
		}
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()
		_, err := h.findAutomationByID(context.Background(), newClient(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for not found")
		}
	})
}

// TestApplyAutomationConfigUpdates_AutomationAction tests the automation_action field.
func TestApplyAutomationConfigUpdates_AutomationAction(t *testing.T) {
	t.Parallel()

	config := &homeassistant.AutomationConfig{Alias: "Test"}

	// automation_action takes priority over action
	newAction := []any{map[string]any{"action": "light.turn_on"}}
	applyAutomationConfigUpdates(config, map[string]any{
		"automation_action": newAction,
	})

	if len(config.Actions) != 1 {
		t.Errorf("config.Actions len = %d, want 1", len(config.Actions))
	}

	// legacy "action" field also works
	legacyAction := []any{map[string]any{"action": "switch.turn_off"}}
	applyAutomationConfigUpdates(config, map[string]any{
		"action": legacyAction,
	})
	if len(config.Actions) != 1 {
		t.Errorf("config.Actions via action key len = %d, want 1", len(config.Actions))
	}
}

// TestBuildVerboseAutomationOutput tests buildVerboseAutomationOutput.
func TestBuildVerboseAutomationOutput(t *testing.T) {
	t.Parallel()

	automations := []homeassistant.Automation{
		{EntityID: "automation.test", State: "on", FriendlyName: "Test"},
	}

	// With pre-fetched configs
	configs := map[string]*homeassistant.AutomationConfig{
		"test": {Alias: "Test", ID: "uuid-test"},
	}

	result, err := buildVerboseAutomationOutput(context.Background(), &mockAutomationClient{}, automations, configs)
	if err != nil {
		t.Fatalf("buildVerboseAutomationOutput() error = %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(string(result), "automation.test") {
		t.Errorf("result should contain entity_id, got: %s", result)
	}
}

// TestFetchAutomationConfigs tests fetchAutomationConfigs.
func TestFetchAutomationConfigs(t *testing.T) {
	t.Parallel()

	t.Run("fetches configs for valid automations", func(t *testing.T) {
		t.Parallel()

		client := &mockAutomationClient{
			automationMap: map[string]*homeassistant.Automation{
				"morning": {
					EntityID: "automation.morning",
					Config:   &homeassistant.AutomationConfig{Alias: "Morning Routine"},
				},
			},
		}

		automations := []homeassistant.Automation{
			{EntityID: "automation.morning", State: "on"},
		}

		configs := fetchAutomationConfigs(context.Background(), client, automations)
		if _, ok := configs["morning"]; !ok {
			t.Error("expected config for 'morning'")
		}
	})

	t.Run("skips automations without automation. prefix", func(t *testing.T) {
		t.Parallel()

		client := &mockAutomationClient{}
		// entity without automation. prefix
		automations := []homeassistant.Automation{
			{EntityID: "scene.morning", State: "on"},
		}

		configs := fetchAutomationConfigs(context.Background(), client, automations)
		if len(configs) != 0 {
			t.Errorf("expected empty configs for non-automation entity, got %d", len(configs))
		}
	})
}

func TestEnrichAutomationError(t *testing.T) {
	tests := []struct {
		name        string
		msg         string
		err         error
		wantContain string
		wantExact   string
	}{
		{
			name:      "non-API error unchanged",
			msg:       "Error creating automation: connection refused",
			err:       errors.New("connection refused"),
			wantExact: "Error creating automation: connection refused",
		},
		{
			name:      "API error with non-400 status unchanged",
			msg:       "Error creating automation: server error",
			err:       &homeassistant.APIError{StatusCode: 500, Message: "internal server error"},
			wantExact: "Error creating automation: server error",
		},
		{
			// data['which'] now matches the specific rule (before the generic "extra keys" rule).
			name: "extra keys not allowed triggers hint",
			msg:  "error saving patched automation: Home Assistant API error (status 400): invalid automation config: {\"message\":\"Message malformed: extra keys not allowed @ data['which']\"}",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid automation config: {\"message\":\"Message malformed: extra keys not allowed @ data['which']\"}",
			},
			wantContain: "'which' is not a valid key for the sun condition",
		},
		{
			name: "required key not provided triggers hint",
			msg:  "Error creating automation: HA error",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid automation config: {\"message\":\"required key not provided @ data['trigger']\"}",
			},
			wantContain: "required field is missing",
		},
		{
			name: "expected a list triggers hint",
			msg:  "Error creating automation: HA error",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid automation config: {\"message\":\"expected a list for trigger\"}",
			},
			wantContain: "must be an array",
		},
		{
			name: "unable to find action triggers hint",
			msg:  "Error creating automation: HA error",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid automation config: {\"message\":\"Unable to find action light.trun_on\"}",
			},
			wantContain: "domain.service",
		},
		{
			name: "invalid template triggers hint",
			msg:  "Error updating automation: HA error",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid automation config: {\"message\":\"invalid template\"}",
			},
			wantContain: "Jinja2",
		},
		{
			name: "unrecognized 400 error unchanged",
			msg:  "Error creating automation: HA error",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid automation config: {\"message\":\"some unknown error\"}",
			},
			wantExact: "Error creating automation: HA error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enrichAutomationError(tt.msg, tt.err)

			if tt.wantExact != "" {
				if result != tt.wantExact {
					t.Errorf("expected exact %q, got %q", tt.wantExact, result)
				}
			}
			if tt.wantContain != "" {
				if !strings.Contains(result, tt.wantContain) {
					t.Errorf("expected result to contain %q, got %q", tt.wantContain, result)
				}
			}
		})
	}
}

func TestManageAutomation_Schema(t *testing.T) {
	h := &AutomationHandlers{}
	result, err := h.handleManageAutomation(context.Background(), nil, map[string]any{"action": "schema"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	for _, want := range []string{"triggers", "conditions", "actions", "notes", "state", "numeric_state", "time", "sun", "template"} {
		if !strings.Contains(text, want) {
			t.Errorf("schema missing expected field/type %q", want)
		}
	}
}

func TestManageAutomation_PatchDryRun(t *testing.T) {
	h := &AutomationHandlers{}

	automation := &homeassistant.Automation{
		EntityID: "automation.test",
		State:    "on",
		Config: &homeassistant.AutomationConfig{
			ID:    "test",
			Alias: "Test Automation",
			Mode:  "single",
			Triggers: []any{
				map[string]any{"trigger": "state", "entity_id": "binary_sensor.motion"},
			},
			Actions: []any{
				map[string]any{"action": "light.turn_on"},
			},
		},
	}

	client := &mockAutomationClient{
		automationMap: map[string]*homeassistant.Automation{
			"test": automation,
		},
	}

	result, err := h.handleManageAutomation(context.Background(), client, map[string]any{
		"action":        "patch",
		"automation_id": "test",
		"dry_run":       true,
		"operations": []any{
			map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "NOT saved") {
		t.Errorf("dry-run result should indicate NOT saved, got: %s", text)
	}
	if !strings.Contains(text, "queued") {
		t.Errorf("dry-run result should show patched value 'queued', got: %s", text)
	}
	// Must NOT have called UpdateAutomation
	if client.updateCalled {
		t.Error("UpdateAutomation should NOT be called during dry-run")
	}
}

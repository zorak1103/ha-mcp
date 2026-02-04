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
	expectedProps := []string{"action", "automation_id", "alias", "trigger", "condition", "automation_action", "mode", "enabled", "state", "verbose", "limit", "cursor"}
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
			name:         "success - no filters",
			args:         map[string]any{"action": "list"},
			client:       &mockAutomationClient{automations: testAutomations},
			wantContains: []string{"automation.turn_on_lights", "Found 3 automations"},
		},
		{
			name:            "success - filter by state",
			args:            map[string]any{"action": "list", "state": "on"},
			client:          &mockAutomationClient{automations: testAutomations},
			wantContains:    []string{"automation.turn_on_lights", "automation.morning_routine"},
			wantNotContains: []string{"automation.turn_off_lights"},
		},
		{
			name:            "success - filter by alias",
			args:            map[string]any{"action": "list", "alias": "lights"},
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
			name:         "success",
			args:         map[string]any{"action": "get", "automation_id": "test_automation"},
			client:       &mockAutomationClient{automation: testAutomation},
			wantContains: []string{"automation.test_automation", "Test Automation"},
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
			client:       &mockAutomationClient{},
			wantContains: []string{"deleted successfully"},
		},
		{
			name:         "error - missing automation_id",
			args:         map[string]any{"action": "delete"},
			client:       &mockAutomationClient{},
			wantError:    true,
			wantContains: []string{"automation_id is required"},
		},
		{
			name:         "error - client error",
			args:         map[string]any{"action": "delete", "automation_id": "test"},
			client:       &mockAutomationClient{deleteErr: errors.New("failed")},
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

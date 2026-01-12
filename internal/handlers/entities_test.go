// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// mockEntityClient implements homeassistant.Client for testing.
type mockEntityClient struct {
	homeassistant.Client
	states       []homeassistant.Entity
	statesErr    error
	state        *homeassistant.Entity
	stateErr     error
	history      [][]homeassistant.HistoryEntry
	historyErr   error
}

func (m *mockEntityClient) GetStates(_ context.Context) ([]homeassistant.Entity, error) {
	if m.statesErr != nil {
		return nil, m.statesErr
	}
	return m.states, nil
}

func (m *mockEntityClient) GetState(_ context.Context, _ string) (*homeassistant.Entity, error) {
	if m.stateErr != nil {
		return nil, m.stateErr
	}
	return m.state, nil
}

func (m *mockEntityClient) GetHistory(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
	if m.historyErr != nil {
		return nil, m.historyErr
	}
	return m.history, nil
}

func TestHandleGetStates(t *testing.T) {
	testStates := []homeassistant.Entity{
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Living Room Light",
				"brightness":    255,
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
		{
			EntityID: "sensor.temperature",
			State:    "21.5",
			Attributes: map[string]any{
				"friendly_name":       "Temperature",
				"unit_of_measurement": "°C",
			},
		},
	}

	tests := []struct {
		name            string
		args            map[string]any
		wantEntityCount int
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "no filters - compact output",
			args:            map[string]any{},
			wantEntityCount: 4,
			wantContains:    []string{"light.living_room", "Living Room Light", "switch.kitchen", "sensor.temperature"},
			wantNotContains: []string{"brightness", "unit_of_measurement", "last_changed"},
		},
		{
			name:            "domain filter - light only",
			args:            map[string]any{"domain": "light"},
			wantEntityCount: 2,
			wantContains:    []string{"light.living_room", "light.bedroom"},
			wantNotContains: []string{"switch.kitchen", "sensor.temperature"},
		},
		{
			name:            "domain filter - switch only",
			args:            map[string]any{"domain": "switch"},
			wantEntityCount: 1,
			wantContains:    []string{"switch.kitchen", "Kitchen Switch"},
			wantNotContains: []string{"light.living_room", "sensor.temperature"},
		},
		{
			name:            "state filter - on only",
			args:            map[string]any{"state": "on"},
			wantEntityCount: 2,
			wantContains:    []string{"light.living_room", "switch.kitchen"},
			wantNotContains: []string{"light.bedroom"},
		},
		{
			name:            "state filter - off only",
			args:            map[string]any{"state": "off"},
			wantEntityCount: 1,
			wantContains:    []string{"light.bedroom"},
			wantNotContains: []string{"light.living_room", "switch.kitchen", "sensor.temperature"},
		},
		{
			name:            "state_not filter - exclude off",
			args:            map[string]any{"state_not": "off"},
			wantEntityCount: 3,
			wantContains:    []string{"light.living_room", "switch.kitchen", "sensor.temperature"},
			wantNotContains: []string{"light.bedroom"},
		},
		{
			name:            "name_contains filter - by entity_id",
			args:            map[string]any{"name_contains": "living"},
			wantEntityCount: 1,
			wantContains:    []string{"light.living_room"},
			wantNotContains: []string{"light.bedroom", "switch.kitchen"},
		},
		{
			name:            "name_contains filter - by friendly_name",
			args:            map[string]any{"name_contains": "kitchen"},
			wantEntityCount: 1,
			wantContains:    []string{"switch.kitchen", "Kitchen Switch"},
			wantNotContains: []string{"light.living_room"},
		},
		{
			name:            "name_contains filter - case insensitive",
			args:            map[string]any{"name_contains": "BEDROOM"},
			wantEntityCount: 1,
			wantContains:    []string{"light.bedroom"},
		},
		{
			name:            "combined filters - domain and state",
			args:            map[string]any{"domain": "light", "state": "on"},
			wantEntityCount: 1,
			wantContains:    []string{"light.living_room"},
			wantNotContains: []string{"light.bedroom", "switch.kitchen"},
		},
		{
			name:            "verbose mode includes all attributes",
			args:            map[string]any{"verbose": true, "domain": "light"},
			wantEntityCount: 2,
			wantContains:    []string{"brightness", "255", "attributes"},
		},
		{
			name:            "verbose mode includes timestamps",
			args:            map[string]any{"verbose": true, "domain": "sensor"},
			wantEntityCount: 1,
			wantContains:    []string{"unit_of_measurement", "°C", "last_changed"},
		},
		{
			name:            "compact mode shows state",
			args:            map[string]any{"domain": "light"},
			wantEntityCount: 2,
			wantContains:    []string{`"state": "on"`, `"state": "off"`},
		},
		{
			name:            "empty result after filtering - no matches",
			args:            map[string]any{"domain": "nonexistent"},
			wantEntityCount: 0,
			wantContains:    []string{"Found 0 entities"},
			wantNotContains: []string{"light.living_room", "switch.kitchen"},
		},
		{
			name:            "all filters combined",
			args:            map[string]any{"domain": "light", "state": "on", "state_not": "off", "name_contains": "living"},
			wantEntityCount: 1,
			wantContains:    []string{"light.living_room", "Living Room Light"},
			wantNotContains: []string{"light.bedroom", "switch.kitchen", "sensor.temperature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &EntityHandlers{}
			client := &mockEntityClient{states: testStates}

			result, err := h.handleGetStates(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleGetStates() error = %v", err)
			}

			if result.IsError {
				t.Fatalf("handleGetStates() returned error: %v", result.Content)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetStates() returned no content")
			}

			content := result.Content[0].Text

			// Check entity count in summary
			expectedSummary := "Found " + itoa(tt.wantEntityCount) + " entities"
			if !strings.Contains(content, expectedSummary) {
				t.Errorf("Expected summary %q not found in content:\n%s", expectedSummary, content[:min(200, len(content))])
			}

			// Check contains
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent: %s", want, content)
				}
			}

			// Check not contains
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did.\nContent: %s", notWant, content)
				}
			}
		})
	}
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func TestCompactEntityStateOmitsEmpty(t *testing.T) {
	entry := compactEntityState{
		EntityID:     "light.test",
		State:        "on",
		FriendlyName: "",
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	if strings.Contains(result, "friendly_name") {
		t.Errorf("Expected empty friendly_name to be omitted, got: %s", result)
	}
	if !strings.Contains(result, "entity_id") {
		t.Errorf("Expected entity_id to be present, got: %s", result)
	}
	if !strings.Contains(result, "state") {
		t.Errorf("Expected state to be present, got: %s", result)
	}
}

func TestCompactEntityStateIncludesFriendlyName(t *testing.T) {
	entry := compactEntityState{
		EntityID:     "light.test",
		State:        "on",
		FriendlyName: "Test Light",
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "friendly_name") {
		t.Errorf("Expected friendly_name to be present, got: %s", result)
	}
	if !strings.Contains(result, "Test Light") {
		t.Errorf("Expected 'Test Light' value to be present, got: %s", result)
	}
}

func TestHandleGetStatesClientError(t *testing.T) {
	h := &EntityHandlers{}
	client := &mockEntityClient{
		statesErr: errors.New("connection refused"),
	}

	result, err := h.handleGetStates(context.Background(), client, map[string]any{})
	if err != nil {
		t.Fatalf("handleGetStates() unexpected error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "Error getting states") {
		t.Errorf("Expected error message, got: %s", content)
	}
	if !strings.Contains(content, "connection refused") {
		t.Errorf("Expected original error in message, got: %s", content)
	}
}

func TestHandleGetState(t *testing.T) {
	testEntity := &homeassistant.Entity{
		EntityID: "light.living_room",
		State:    "on",
		Attributes: map[string]any{
			"friendly_name": "Living Room Light",
			"brightness":    255,
		},
		LastChanged: time.Now(),
		LastUpdated: time.Now(),
	}

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockEntityClient
		wantError    bool
		wantContains []string
	}{
		{
			name: "success - returns entity state",
			args: map[string]any{"entity_id": "light.living_room"},
			client: &mockEntityClient{
				state: testEntity,
			},
			wantError:    false,
			wantContains: []string{"light.living_room", "on", "Living Room Light", "brightness", "255"},
		},
		{
			name:         "error - missing entity_id",
			args:         map[string]any{},
			client:       &mockEntityClient{},
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name:         "error - empty entity_id",
			args:         map[string]any{"entity_id": ""},
			client:       &mockEntityClient{},
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{"entity_id": "light.nonexistent"},
			client: &mockEntityClient{
				stateErr: errors.New("entity not found"),
			},
			wantError:    true,
			wantContains: []string{"Error getting state", "entity not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &EntityHandlers{}

			result, err := h.handleGetState(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleGetState() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetState() returned no content")
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

func TestHandleGetHistory(t *testing.T) {
	now := time.Now()
	testHistory := [][]homeassistant.HistoryEntry{
		{
			{
				EntityID:    "sensor.temperature",
				State:       "21.5",
				LastChanged: float64(now.Add(-2 * time.Hour).Unix()),
			},
			{
				EntityID:    "sensor.temperature",
				State:       "22.0",
				LastChanged: float64(now.Add(-1 * time.Hour).Unix()),
			},
			{
				EntityID:    "sensor.temperature",
				State:       "22.5",
				LastChanged: float64(now.Unix()),
			},
		},
	}

	tests := []struct {
		name            string
		args            map[string]any
		client          *mockEntityClient
		wantError       bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "success - basic history retrieval",
			args: map[string]any{"entity_id": "sensor.temperature"},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{"21.5", "22.0", "22.5", "Found 3 history entries"},
		},
		{
			name: "success - with hours parameter",
			args: map[string]any{"entity_id": "sensor.temperature", "hours": float64(6)},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries"},
		},
		{
			name: "success - with state filter",
			args: map[string]any{"entity_id": "sensor.temperature", "state": "22.0"},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:       false,
			wantContains:    []string{"22.0", "filtered by state='22.0'"},
			wantNotContains: []string{"21.5", "22.5"},
		},
		{
			name: "success - with limit",
			args: map[string]any{"entity_id": "sensor.temperature", "limit": float64(2)},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:       false,
			wantContains:    []string{"Showing 2 of 3", "22.0", "22.5"},
			wantNotContains: []string{"21.5"},
		},
		{
			name: "success - verbose mode",
			args: map[string]any{"entity_id": "sensor.temperature", "verbose": true},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{"21.5", "22.0", "22.5"},
		},
		{
			name: "success - compact mode shows state and timestamp",
			args: map[string]any{"entity_id": "sensor.temperature"},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{`"state"`, `"last_changed"`},
		},
		{
			name:         "error - missing entity_id",
			args:         map[string]any{},
			client:       &mockEntityClient{},
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name:         "error - empty entity_id",
			args:         map[string]any{"entity_id": ""},
			client:       &mockEntityClient{},
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name: "error - invalid start_time format",
			args: map[string]any{
				"entity_id":  "sensor.temperature",
				"start_time": "not-a-date",
			},
			client:       &mockEntityClient{},
			wantError:    true,
			wantContains: []string{"Invalid start_time format"},
		},
		{
			name: "error - invalid end_time format",
			args: map[string]any{
				"entity_id": "sensor.temperature",
				"end_time":  "not-a-date",
			},
			client:       &mockEntityClient{},
			wantError:    true,
			wantContains: []string{"Invalid end_time format"},
		},
		{
			name: "error - client error",
			args: map[string]any{"entity_id": "sensor.temperature"},
			client: &mockEntityClient{
				historyErr: errors.New("database unavailable"),
			},
			wantError:    true,
			wantContains: []string{"Error getting history", "database unavailable"},
		},
		{
			name: "success - valid RFC3339 start_time",
			args: map[string]any{
				"entity_id":  "sensor.temperature",
				"start_time": now.Add(-12 * time.Hour).Format(time.RFC3339),
			},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries"},
		},
		{
			name: "success - valid RFC3339 end_time",
			args: map[string]any{
				"entity_id": "sensor.temperature",
				"end_time":  now.Format(time.RFC3339),
			},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries"},
		},
		{
			name: "success - empty history",
			args: map[string]any{"entity_id": "sensor.temperature"},
			client: &mockEntityClient{
				history: [][]homeassistant.HistoryEntry{},
			},
			wantError:    false,
			wantContains: []string{"Found 0 history entries"},
		},
		{
			name: "hours parameter overrides start_time",
			args: map[string]any{
				"entity_id":  "sensor.temperature",
				"hours":      float64(1),
				"start_time": now.Add(-48 * time.Hour).Format(time.RFC3339), // should be ignored
			},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries"},
		},
		{
			name: "limit and state filter combined",
			args: map[string]any{
				"entity_id": "sensor.temperature",
				"state":     "22.0",
				"limit":     float64(1),
			},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:       false,
			wantContains:    []string{"22.0", "Showing 1 of 1"},
			wantNotContains: []string{"21.5", "22.5"},
		},
		{
			name: "limit larger than results returns all",
			args: map[string]any{
				"entity_id": "sensor.temperature",
				"limit":     float64(100),
			},
			client: &mockEntityClient{
				history: testHistory,
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries", "21.5", "22.0", "22.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &EntityHandlers{}

			result, err := h.handleGetHistory(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleGetHistory() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetHistory() returned no content")
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

func TestHandleListDomains(t *testing.T) {
	testStates := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "light.bedroom", State: "off"},
		{EntityID: "light.kitchen", State: "on"},
		{EntityID: "switch.garage", State: "off"},
		{EntityID: "sensor.temperature", State: "21.5"},
		{EntityID: "sensor.humidity", State: "45"},
		{EntityID: "binary_sensor.door", State: "off"},
	}

	tests := []struct {
		name            string
		client          *mockEntityClient
		wantError       bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "success - lists all domains with counts",
			client: &mockEntityClient{
				states: testStates,
			},
			wantError: false,
			wantContains: []string{
				`"domain": "light"`,
				`"entity_count": 3`,
				`"domain": "switch"`,
				`"entity_count": 1`,
				`"domain": "sensor"`,
				`"entity_count": 2`,
				`"domain": "binary_sensor"`,
			},
		},
		{
			name: "success - empty state list",
			client: &mockEntityClient{
				states: []homeassistant.Entity{},
			},
			wantError:    false,
			wantContains: []string{"[]"},
		},
		{
			name: "error - client error",
			client: &mockEntityClient{
				statesErr: errors.New("connection timeout"),
			},
			wantError:    true,
			wantContains: []string{"Error getting states", "connection timeout"},
		},
		{
			name: "handles entity_id without dot gracefully",
			client: &mockEntityClient{
				states: []homeassistant.Entity{
					{EntityID: "light.valid", State: "on"},
					{EntityID: "invalidnodot", State: "on"},
					{EntityID: "switch.also_valid", State: "off"},
				},
			},
			wantError: false,
			wantContains: []string{
				`"domain": "light"`,
				`"domain": "switch"`,
			},
			wantNotContains: []string{"invalidnodot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &EntityHandlers{}

			result, err := h.handleListDomains(context.Background(), tt.client, map[string]any{})
			if err != nil {
				t.Fatalf("handleListDomains() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleListDomains() returned no content")
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

func TestCompactHistoryEntryFormat(t *testing.T) {
	entry := compactHistoryEntry{
		State:       "on",
		LastChanged: "2024-01-15T10:30:00Z",
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	if !strings.Contains(result, `"state":"on"`) {
		t.Errorf("Expected state to be present, got: %s", result)
	}
	if !strings.Contains(result, `"last_changed":"2024-01-15T10:30:00Z"`) {
		t.Errorf("Expected last_changed to be present, got: %s", result)
	}
}

func TestNewEntityHandlers(t *testing.T) {
	h := NewEntityHandlers()
	if h == nil {
		t.Fatal("NewEntityHandlers() returned nil")
	}
}

func TestEntityHandlersRegisterTools(t *testing.T) {
	h := NewEntityHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify all expected tools are registered by checking ListTools
	tools := registry.ListTools()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{"get_states", "get_state", "get_history", "list_domains"}
	for _, toolName := range expectedTools {
		if !toolNames[toolName] {
			t.Errorf("Expected tool %q to be registered", toolName)
		}
	}
}

func TestGetStatesTool(t *testing.T) {
	h := &EntityHandlers{}
	tool := h.getStatesTool()

	if tool.Name != "get_states" {
		t.Errorf("Expected tool name 'get_states', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}
	if tool.InputSchema.Type != "object" {
		t.Errorf("Expected input schema type 'object', got %q", tool.InputSchema.Type)
	}

	// Check expected properties exist
	expectedProps := []string{"domain", "state", "state_not", "name_contains", "verbose"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Expected property %q in input schema", prop)
		}
	}
}

func TestGetStateTool(t *testing.T) {
	h := &EntityHandlers{}
	tool := h.getStateTool()

	if tool.Name != "get_state" {
		t.Errorf("Expected tool name 'get_state', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check entity_id is required
	if len(tool.InputSchema.Required) == 0 || tool.InputSchema.Required[0] != "entity_id" {
		t.Error("Expected 'entity_id' to be required")
	}
}

func TestGetHistoryTool(t *testing.T) {
	h := &EntityHandlers{}
	tool := h.getHistoryTool()

	if tool.Name != "get_history" {
		t.Errorf("Expected tool name 'get_history', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check expected properties exist
	expectedProps := []string{"entity_id", "start_time", "end_time", "hours", "state", "limit", "verbose"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Expected property %q in input schema", prop)
		}
	}

	// Check entity_id is required
	if len(tool.InputSchema.Required) == 0 || tool.InputSchema.Required[0] != "entity_id" {
		t.Error("Expected 'entity_id' to be required")
	}
}

func TestListDomainsTool(t *testing.T) {
	h := &EntityHandlers{}
	tool := h.listDomainsTool()

	if tool.Name != "list_domains" {
		t.Errorf("Expected tool name 'list_domains', got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Expected non-empty description")
	}
	if tool.InputSchema.Type != "object" {
		t.Errorf("Expected input schema type 'object', got %q", tool.InputSchema.Type)
	}
}


// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Tests use UniversalMockClient from testing_helpers_test.go

func TestHandleGetStates(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			h := &EntityHandlers{}
			client := &UniversalMockClient{
				GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				},
			}

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
			assertContainsAll(t, content, tt.wantContains)

			// Check not contains
			assertNotContainsAny(t, content, tt.wantNotContains)
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
	t.Parallel()

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
	if !strings.Contains(result, configKeyEntityID) {
		t.Errorf("Expected entity_id to be present, got: %s", result)
	}
	if !strings.Contains(result, "state") {
		t.Errorf("Expected state to be present, got: %s", result)
	}
}

func TestCompactEntityStateIncludesFriendlyName(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	h := &EntityHandlers{}
	client := &UniversalMockClient{
		GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
			return nil, errors.New("connection refused")
		},
	}

	result, err := h.handleGetStates(context.Background(), client, map[string]any{})
	if err != nil {
		t.Fatalf("handleGetStates() unexpected error = %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}

	content := result.Content[0].Text
	assertContainsAll(t, content, []string{"Error getting states", "connection refused"})
}

func TestHandleGetState(t *testing.T) {
	t.Parallel()

	testEntityData := &homeassistant.Entity{
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
		setupMock    func(*UniversalMockClient)
		wantError    bool
		wantContains []string
	}{
		{
			name: "success - returns entity state",
			args: map[string]any{"entity_id": "light.living_room"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return testEntityData, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light.living_room", "on", "Living Room Light", "brightness", "255"},
		},
		{
			name:         "error - missing entity_id",
			args:         map[string]any{},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name:         "error - empty entity_id",
			args:         map[string]any{"entity_id": ""},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{"entity_id": "light.nonexistent"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return nil, errors.New("entity not found")
				}
			},
			wantError:    true,
			wantContains: []string{"Error getting state", "entity not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &EntityHandlers{}
			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			result, err := h.handleGetState(context.Background(), client, tt.args)
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
			assertContainsAll(t, content, tt.wantContains)
		})
	}
}

func TestHandleGetHistory(t *testing.T) {
	t.Parallel()

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
		setupMock       func(*UniversalMockClient)
		wantError       bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "success - basic history retrieval",
			args: map[string]any{"entity_id": "sensor.temperature"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"21.5", "22.0", "22.5", "Found 3 history entries"},
		},
		{
			name: "success - with hours parameter",
			args: map[string]any{"entity_id": "sensor.temperature", "hours": float64(6)},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries"},
		},
		{
			name: "success - with state filter",
			args: map[string]any{"entity_id": "sensor.temperature", "state": "22.0"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"22.0", "filtered by state='22.0'"},
			wantNotContains: []string{"21.5", "22.5"},
		},
		{
			name: "success - with limit",
			args: map[string]any{"entity_id": "sensor.temperature", "limit": float64(2)},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"Showing 2 of 3", "22.0", "22.5"},
			wantNotContains: []string{"21.5"},
		},
		{
			name: "success - verbose mode",
			args: map[string]any{"entity_id": "sensor.temperature", "verbose": true},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"21.5", "22.0", "22.5"},
		},
		{
			name: "success - compact mode shows state and timestamp",
			args: map[string]any{"entity_id": "sensor.temperature"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{`"state"`, `"last_changed"`},
		},
		{
			name:         "error - missing entity_id",
			args:         map[string]any{},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name:         "error - empty entity_id",
			args:         map[string]any{"entity_id": ""},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name: "error - invalid start_time format",
			args: map[string]any{
				"entity_id":  "sensor.temperature",
				"start_time": "not-a-date",
			},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"invalid start_time format"},
		},
		{
			name: "error - invalid end_time format",
			args: map[string]any{
				"entity_id": "sensor.temperature",
				"end_time":  "not-a-date",
			},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"invalid end_time format"},
		},
		{
			name: "error - client error",
			args: map[string]any{"entity_id": "sensor.temperature"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return nil, errors.New("database unavailable")
				}
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
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
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
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries"},
		},
		{
			name: "success - empty history",
			args: map[string]any{"entity_id": "sensor.temperature"},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return [][]homeassistant.HistoryEntry{}, nil
				}
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
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
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
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"22.0", "Found 1 history entries"},
			wantNotContains: []string{"21.5", "22.5"},
		},
		{
			name: "limit larger than results returns all",
			args: map[string]any{
				"entity_id": "sensor.temperature",
				"limit":     float64(100),
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetHistoryFn = func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return testHistory, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 3 history entries", "21.5", "22.0", "22.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &EntityHandlers{}
			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			result, err := h.handleGetHistory(context.Background(), client, tt.args)
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
			assertContainsAll(t, content, tt.wantContains)
			assertNotContainsAny(t, content, tt.wantNotContains)
		})
	}
}

func TestHandleListDomains(t *testing.T) {
	t.Parallel()

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
		setupMock       func(*UniversalMockClient)
		wantError       bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "success - lists all domains with counts",
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
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
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"[]"},
		},
		{
			name: "error - client error",
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return nil, errors.New("connection timeout")
				}
			},
			wantError:    true,
			wantContains: []string{"Error getting states", "connection timeout"},
		},
		{
			name: "handles entity_id without dot gracefully",
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "light.valid", State: "on"},
						{EntityID: "invalidnodot", State: "on"},
						{EntityID: "switch.also_valid", State: "off"},
					}, nil
				}
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
			t.Parallel()

			h := &EntityHandlers{}
			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			result, err := h.handleListDomains(context.Background(), client, map[string]any{})
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
			assertContainsAll(t, content, tt.wantContains)
			assertNotContainsAny(t, content, tt.wantNotContains)
		})
	}
}

func TestCompactHistoryEntryFormat(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	h := NewEntityHandlers()
	if h == nil {
		t.Fatal("NewEntityHandlers() returned nil")
	}
}

func TestEntityHandlersRegisterTools(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	h := &EntityHandlers{}
	tool := h.getStatesTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "get_states",
		RequiredParams:  []string{},
		OptionalParams:  []string{"domain", "state", "state_not", "name_contains", "verbose"},
		WantDescription: true,
	})
}

func TestGetStateTool(t *testing.T) {
	t.Parallel()

	h := &EntityHandlers{}
	tool := h.getStateTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "get_state",
		RequiredParams:  []string{configKeyEntityID},
		OptionalParams:  []string{},
		WantDescription: true,
	})
}

func TestGetHistoryTool(t *testing.T) {
	t.Parallel()

	h := &EntityHandlers{}
	tool := h.getHistoryTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "get_history",
		RequiredParams:  []string{configKeyEntityID},
		OptionalParams:  []string{"start_time", "end_time", "hours", "state", "limit", "verbose"},
		WantDescription: true,
	})
}

func TestListDomainsTool(t *testing.T) {
	t.Parallel()

	h := &EntityHandlers{}
	tool := h.listDomainsTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "list_domains",
		RequiredParams:  []string{},
		OptionalParams:  []string{},
		WantDescription: true,
	})
}

// Tests for extracted history helper functions

func TestParseHistoryParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
		validate    func(t *testing.T, p *historyParams)
	}{
		{
			name:        "missing entity_id",
			args:        map[string]any{},
			wantErr:     true,
			errContains: "entity_id is required",
		},
		{
			name:        "empty entity_id",
			args:        map[string]any{"entity_id": ""},
			wantErr:     true,
			errContains: "entity_id is required",
		},
		{
			name:    "minimal valid - entity_id only",
			args:    map[string]any{"entity_id": "sensor.test"},
			wantErr: false,
			validate: func(t *testing.T, p *historyParams) {
				t.Helper()
				if p.entityID != "sensor.test" {
					t.Errorf("entityID = %q, want %q", p.entityID, "sensor.test")
				}
				if p.verbose {
					t.Error("verbose should be false by default")
				}
				if p.limit != 0 {
					t.Errorf("limit = %d, want 0", p.limit)
				}
			},
		},
		{
			name: "all parameters",
			args: map[string]any{
				"entity_id": "sensor.temp",
				"hours":     float64(6),
				"state":     "on",
				"limit":     float64(10),
				"verbose":   true,
			},
			wantErr: false,
			validate: func(t *testing.T, p *historyParams) {
				t.Helper()
				if p.entityID != "sensor.temp" {
					t.Errorf("entityID = %q, want %q", p.entityID, "sensor.temp")
				}
				if p.stateFilter != "on" {
					t.Errorf("stateFilter = %q, want %q", p.stateFilter, "on")
				}
				if p.limit != 10 {
					t.Errorf("limit = %d, want 10", p.limit)
				}
				if !p.verbose {
					t.Error("verbose should be true")
				}
			},
		},
		{
			name: "invalid start_time propagates error",
			args: map[string]any{
				"entity_id":  "sensor.test",
				"start_time": "not-a-date",
			},
			wantErr:     true,
			errContains: "invalid start_time format",
		},
		{
			name: "negative limit treated as zero",
			args: map[string]any{
				"entity_id": "sensor.test",
				"limit":     float64(-5),
			},
			wantErr: false,
			validate: func(t *testing.T, p *historyParams) {
				t.Helper()
				if p.limit != 0 {
					t.Errorf("limit = %d, want 0 for negative input", p.limit)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params, err := parseHistoryParams(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, params)
			}
		})
	}
}

func TestParseTimeRange(t *testing.T) {
	t.Parallel()

	now := time.Now()
	validTime := now.Add(-6 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
		validate    func(t *testing.T, start, end time.Time)
	}{
		{
			name:    "defaults - 24 hours lookback",
			args:    map[string]any{},
			wantErr: false,
			validate: func(t *testing.T, start, end time.Time) {
				t.Helper()
				duration := end.Sub(start)
				if duration < 23*time.Hour || duration > 25*time.Hour {
					t.Errorf("default duration = %v, want ~24h", duration)
				}
			},
		},
		{
			name:    "hours parameter sets start time",
			args:    map[string]any{"hours": float64(6)},
			wantErr: false,
			validate: func(t *testing.T, start, end time.Time) {
				t.Helper()
				duration := end.Sub(start)
				if duration < 5*time.Hour || duration > 7*time.Hour {
					t.Errorf("duration = %v, want ~6h", duration)
				}
			},
		},
		{
			name:    "start_time parameter",
			args:    map[string]any{"start_time": validTime},
			wantErr: false,
			validate: func(t *testing.T, start, _ time.Time) {
				t.Helper()
				expected, _ := time.Parse(time.RFC3339, validTime)
				if !start.Equal(expected) {
					t.Errorf("start = %v, want %v", start, expected)
				}
			},
		},
		{
			name:    "hours overrides start_time",
			args:    map[string]any{"hours": float64(2), "start_time": validTime},
			wantErr: false,
			validate: func(t *testing.T, start, end time.Time) {
				t.Helper()
				duration := end.Sub(start)
				if duration < 1*time.Hour || duration > 3*time.Hour {
					t.Errorf("duration = %v, want ~2h (hours should override start_time)", duration)
				}
			},
		},
		{
			name:        "invalid start_time format",
			args:        map[string]any{"start_time": "2024-01-15"},
			wantErr:     true,
			errContains: "invalid start_time format",
		},
		{
			name:        "invalid end_time format",
			args:        map[string]any{"end_time": "tomorrow"},
			wantErr:     true,
			errContains: "invalid end_time format",
		},
		{
			name:    "valid end_time",
			args:    map[string]any{"end_time": now.Add(-1 * time.Hour).Format(time.RFC3339)},
			wantErr: false,
			validate: func(t *testing.T, _, end time.Time) {
				t.Helper()
				expected := now.Add(-1 * time.Hour)
				if end.Sub(expected) > time.Second || expected.Sub(end) > time.Second {
					t.Errorf("end = %v, want ~%v", end, expected)
				}
			},
		},
		{
			name:    "zero hours falls back to default",
			args:    map[string]any{"hours": float64(0)},
			wantErr: false,
			validate: func(t *testing.T, start, end time.Time) {
				t.Helper()
				duration := end.Sub(start)
				if duration < 23*time.Hour || duration > 25*time.Hour {
					t.Errorf("duration = %v, want ~24h for zero hours", duration)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			start, end, err := parseTimeRange(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, start, end)
			}
		})
	}
}

func TestProcessHistoryEntries(t *testing.T) {
	t.Parallel()

	baseEntries := [][]homeassistant.HistoryEntry{
		{
			{EntityID: "sensor.test", State: "on", LastChanged: 1.0},
			{EntityID: "sensor.test", State: "off", LastChanged: 2.0},
			{EntityID: "sensor.test", State: "on", LastChanged: 3.0},
			{EntityID: "sensor.test", State: "off", LastChanged: 4.0},
			{EntityID: "sensor.test", State: "on", LastChanged: 5.0},
		},
	}

	tests := []struct {
		name        string
		history     [][]homeassistant.HistoryEntry
		stateFilter string
		limit       int
		wantCount   int
		wantTotal   int
		wantStates  []string
	}{
		{
			name:       "no filter, no limit",
			history:    baseEntries,
			wantCount:  5,
			wantTotal:  5,
			wantStates: []string{"on", "off", "on", "off", "on"},
		},
		{
			name:        "state filter - on only",
			history:     baseEntries,
			stateFilter: "on",
			wantCount:   3,
			wantTotal:   3,
			wantStates:  []string{"on", "on", "on"},
		},
		{
			name:        "state filter - off only",
			history:     baseEntries,
			stateFilter: "off",
			wantCount:   2,
			wantTotal:   2,
			wantStates:  []string{"off", "off"},
		},
		{
			name:       "limit - takes most recent",
			history:    baseEntries,
			limit:      2,
			wantCount:  2,
			wantTotal:  5,
			wantStates: []string{"off", "on"},
		},
		{
			name:        "filter and limit combined",
			history:     baseEntries,
			stateFilter: "on",
			limit:       2,
			wantCount:   2,
			wantTotal:   3,
			wantStates:  []string{"on", "on"},
		},
		{
			name:       "limit larger than entries",
			history:    baseEntries,
			limit:      100,
			wantCount:  5,
			wantTotal:  5,
			wantStates: []string{"on", "off", "on", "off", "on"},
		},
		{
			name:       "empty history",
			history:    [][]homeassistant.HistoryEntry{},
			wantCount:  0,
			wantTotal:  0,
			wantStates: []string{},
		},
		{
			name: "multiple inner arrays flattened",
			history: [][]homeassistant.HistoryEntry{
				{{EntityID: "a", State: "1"}},
				{{EntityID: "b", State: "2"}},
				{{EntityID: "c", State: "3"}},
			},
			wantCount:  3,
			wantTotal:  3,
			wantStates: []string{"1", "2", "3"},
		},
		{
			name:        "filter yields empty result",
			history:     baseEntries,
			stateFilter: "unknown",
			wantCount:   0,
			wantTotal:   0,
			wantStates:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := processHistoryEntries(tt.history, tt.stateFilter, tt.limit)

			if len(result.entries) != tt.wantCount {
				t.Errorf("entries count = %d, want %d", len(result.entries), tt.wantCount)
			}

			if result.totalCount != tt.wantTotal {
				t.Errorf("totalCount = %d, want %d", result.totalCount, tt.wantTotal)
			}

			if len(tt.wantStates) > 0 {
				for i, entry := range result.entries {
					if i < len(tt.wantStates) && entry.State != tt.wantStates[i] {
						t.Errorf("entry[%d].State = %q, want %q", i, entry.State, tt.wantStates[i])
					}
				}
			}
		})
	}
}

func TestFormatHistoryOutput(t *testing.T) {
	t.Parallel()

	entries := []homeassistant.HistoryEntry{
		{
			EntityID:    "sensor.test",
			State:       "on",
			LastChanged: float64(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).Unix()),
			Attributes:  map[string]any{"brightness": 255},
		},
	}

	tests := []struct {
		name         string
		entries      []homeassistant.HistoryEntry
		verbose      bool
		wantContains []string
		wantMissing  []string
	}{
		{
			name:         "compact output - state and last_changed only",
			entries:      entries,
			verbose:      false,
			wantContains: []string{`"state"`, `"last_changed"`},
			wantMissing:  []string{"brightness", "entity_id"},
		},
		{
			name:         "verbose output - includes all fields",
			entries:      entries,
			verbose:      true,
			wantContains: []string{`"s"`, `"entity_id"`, `"brightness"`, "255"},
		},
		{
			name:         "empty entries - compact",
			entries:      []homeassistant.HistoryEntry{},
			verbose:      false,
			wantContains: []string{"[]"},
		},
		{
			name:         "empty entries - verbose",
			entries:      []homeassistant.HistoryEntry{},
			verbose:      true,
			wantContains: []string{"[]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, err := formatHistoryOutput(tt.entries, tt.verbose)
			if err != nil {
				t.Fatalf("formatHistoryOutput() error = %v", err)
			}

			result := string(output)
			assertContainsAll(t, result, tt.wantContains)
			assertNotContainsAny(t, result, tt.wantMissing)
		})
	}
}

func TestBuildHistorySummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		entityID     string
		result       historyResult
		stateFilter  string
		verbose      bool
		wantContains []string
		wantMissing  []string
	}{
		{
			name:     "basic summary",
			entityID: "sensor.temp",
			result: historyResult{
				entries:    make([]homeassistant.HistoryEntry, 5),
				totalCount: 5,
			},
			wantContains: []string{"Found 5 history entries", "sensor.temp"},
		},
		{
			name:     "limited results",
			entityID: "sensor.temp",
			result: historyResult{
				entries:    make([]homeassistant.HistoryEntry, 10),
				totalCount: 50,
			},
			wantContains: []string{"Showing 10 of 50", "sensor.temp", "(limited)"},
		},
		{
			name:         "with state filter",
			entityID:     "light.test",
			result:       historyResult{entries: make([]homeassistant.HistoryEntry, 3), totalCount: 3},
			stateFilter:  "on",
			wantContains: []string{"Found 3 history entries", "filtered by state='on'"},
		},
		{
			name:         "compact mode includes verbose hint",
			entityID:     "sensor.test",
			result:       historyResult{entries: make([]homeassistant.HistoryEntry, 1), totalCount: 1},
			verbose:      false,
			wantContains: []string{VerboseHint},
		},
		{
			name:        "verbose mode excludes verbose hint",
			entityID:    "sensor.test",
			result:      historyResult{entries: make([]homeassistant.HistoryEntry, 1), totalCount: 1},
			verbose:     true,
			wantMissing: []string{VerboseHint},
		},
		{
			name:         "limited with state filter",
			entityID:     "switch.garage",
			result:       historyResult{entries: make([]homeassistant.HistoryEntry, 5), totalCount: 20},
			stateFilter:  "off",
			wantContains: []string{"Showing 5 of 20", "switch.garage", "(limited)", "filtered by state='off'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			summary := buildHistorySummary(tt.entityID, tt.result, tt.stateFilter, tt.verbose)

			assertContainsAll(t, summary, tt.wantContains)
			assertNotContainsAny(t, summary, tt.wantMissing)
		})
	}
}

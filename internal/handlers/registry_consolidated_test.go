// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockConsolidatedRegistryClient implements homeassistant.Client for consolidated registry tests.
type mockConsolidatedRegistryClient struct {
	homeassistant.Client
	entityRegistry []homeassistant.EntityRegistryEntry
	deviceRegistry []homeassistant.DeviceRegistryEntry
	areaRegistry   []homeassistant.AreaRegistryEntry
	entityErr      error
	deviceErr      error
	areaErr        error
}

func (m *mockConsolidatedRegistryClient) GetEntityRegistry(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	if m.entityErr != nil {
		return nil, m.entityErr
	}
	return m.entityRegistry, nil
}

func (m *mockConsolidatedRegistryClient) GetDeviceRegistry(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	if m.deviceErr != nil {
		return nil, m.deviceErr
	}
	return m.deviceRegistry, nil
}

func (m *mockConsolidatedRegistryClient) GetAreaRegistry(_ context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	if m.areaErr != nil {
		return nil, m.areaErr
	}
	return m.areaRegistry, nil
}

func TestNewConsolidatedRegistryHandlers(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedRegistryHandlers()
	if h == nil {
		t.Error("NewConsolidatedRegistryHandlers() returned nil, want non-nil")
	}
}

func TestConsolidatedRegistryHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedRegistryHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	if len(tools) > 0 && tools[0].Name != "get_registry" {
		t.Errorf("Registered tool name = %q, want %q", tools[0].Name, "get_registry")
	}
}

func TestConsolidatedRegistryHandlers_GetRegistryTool_Schema(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedRegistryHandlers()
	tool := h.getRegistryTool()

	tests := []struct {
		name      string
		checkFunc func(t *testing.T, tool mcp.Tool)
	}{
		{
			name: "has correct name",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Name != "get_registry" {
					t.Errorf("tool.Name = %q, want %q", tool.Name, "get_registry")
				}
			},
		},
		{
			name: "has type parameter with enum",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				prop, ok := tool.InputSchema.Properties["type"]
				if !ok {
					t.Fatal("type property missing")
				}
				if len(prop.Enum) != 4 {
					t.Errorf("type enum has %d values, want 4 (entities, devices, areas, all)", len(prop.Enum))
				}
			},
		},
		{
			name: "has filter parameters",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				filters := []string{"domain", "platform", "device_id", "area_id", "manufacturer", "model"}
				for _, f := range filters {
					if _, ok := tool.InputSchema.Properties[f]; !ok {
						t.Errorf("filter property %q missing", f)
					}
				}
			},
		},
		{
			name: "has verbose parameter",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if _, ok := tool.InputSchema.Properties["verbose"]; !ok {
					t.Error("verbose property missing")
				}
			},
		},
		{
			name: "has pagination parameters",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if _, ok := tool.InputSchema.Properties["limit"]; !ok {
					t.Error("limit property missing")
				}
				if _, ok := tool.InputSchema.Properties["cursor"]; !ok {
					t.Error("cursor property missing")
				}
			},
		},
		{
			name: "has type as required",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				found := false
				for _, r := range tool.InputSchema.Required {
					if r == "type" {
						found = true
						break
					}
				}
				if !found {
					t.Error("type should be required")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.checkFunc(t, tool)
		})
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Entities(t *testing.T) {
	t.Parallel()

	testEntities := []homeassistant.EntityRegistryEntry{
		{EntityID: "switch.test1", Platform: "fritz", DeviceID: "dev1", AreaID: "area1"},
		{EntityID: "switch.test2", Platform: "hue", DeviceID: "dev2", AreaID: "area1"},
		{EntityID: "light.test3", Platform: "hue", DeviceID: "dev2", AreaID: "area2"},
		{EntityID: "sensor.disabled", Platform: "mqtt", DeviceID: "dev3", DisabledBy: "user"},
	}

	tests := []struct {
		name            string
		args            map[string]any
		wantEntityCount int
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "entities - no filters",
			args:            map[string]any{"type": "entities", "format": "json"},
			wantEntityCount: 3,
			wantContains:    []string{"switch.test1", "switch.test2", "light.test3"},
			wantNotContains: []string{"sensor.disabled"},
		},
		{
			name:            "entities - domain filter",
			args:            map[string]any{"type": "entities", "domain": "switch", "format": "json"},
			wantEntityCount: 2,
			wantContains:    []string{"switch.test1", "switch.test2"},
			wantNotContains: []string{"light.test3"},
		},
		{
			name:            "entities - platform filter",
			args:            map[string]any{"type": "entities", "platform": "hue", "format": "json"},
			wantEntityCount: 2,
			wantContains:    []string{"switch.test2", "light.test3"},
			wantNotContains: []string{"switch.test1"},
		},
		{
			name:            "entities - include_disabled",
			args:            map[string]any{"type": "entities", "include_disabled": true, "format": "json"},
			wantEntityCount: 4,
			wantContains:    []string{"sensor.disabled"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewConsolidatedRegistryHandlers()
			client := &mockConsolidatedRegistryClient{entityRegistry: testEntities}

			result, err := h.handleGetRegistry(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleGetRegistry() error = %v", err)
			}

			if result.IsError {
				t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetRegistry() returned no content")
			}

			content := result.Content[0].Text

			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent: %s", want, content[:min(500, len(content))])
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did", notWant)
				}
			}
		})
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Devices(t *testing.T) {
	t.Parallel()

	testDevices := []homeassistant.DeviceRegistryEntry{
		{ID: "dev1", Name: "Device 1", Manufacturer: "Philips", Model: "Hue Bridge", AreaID: "area1"},
		{ID: "dev2", Name: "Device 2", Manufacturer: "IKEA", Model: "Tradfri Gateway", AreaID: "area2"},
		{ID: "dev3", Name: "Device 3", Manufacturer: "Philips", Model: "Hue Bulb", AreaID: "area1", DisabledBy: "user"},
	}

	tests := []struct {
		name           string
		args           map[string]any
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "devices - no filters",
			args:           map[string]any{"type": "devices", "format": "json"},
			wantContains:   []string{"dev1", "dev2"},
			wantNotContain: []string{"dev3"}, // disabled
		},
		{
			name:           "devices - manufacturer filter",
			args:           map[string]any{"type": "devices", "manufacturer": "Philips", "format": "json"},
			wantContains:   []string{"dev1"},
			wantNotContain: []string{"dev2"},
		},
		{
			name:           "devices - area filter",
			args:           map[string]any{"type": "devices", "area_id": "area1", "format": "json"},
			wantContains:   []string{"dev1"},
			wantNotContain: []string{"dev2"},
		},
		{
			name:         "devices - include_disabled",
			args:         map[string]any{"type": "devices", "include_disabled": true, "format": "json"},
			wantContains: []string{"dev1", "dev2", "dev3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewConsolidatedRegistryHandlers()
			client := &mockConsolidatedRegistryClient{deviceRegistry: testDevices}

			result, err := h.handleGetRegistry(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleGetRegistry() error = %v", err)
			}

			if result.IsError {
				t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
			}

			content := result.Content[0].Text

			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent: %s", want, content[:min(500, len(content))])
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did", notWant)
				}
			}
		})
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Areas(t *testing.T) {
	t.Parallel()

	testAreas := []homeassistant.AreaRegistryEntry{
		{AreaID: "area1", Name: "Living Room"},
		{AreaID: "area2", Name: "Kitchen"},
	}

	h := NewConsolidatedRegistryHandlers()
	client := &mockConsolidatedRegistryClient{areaRegistry: testAreas}

	// Use format=json for backward compatibility (tests expect JSON field names)
	result, err := h.handleGetRegistry(context.Background(), client, map[string]any{"type": "areas", "format": "json"})
	if err != nil {
		t.Fatalf("handleGetRegistry() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	if !strings.Contains(content, "area1") {
		t.Error("Expected content to contain area1")
	}
	if !strings.Contains(content, "Living Room") {
		t.Error("Expected content to contain Living Room")
	}
	if !strings.Contains(content, "area2") {
		t.Error("Expected content to contain area2")
	}
	if !strings.Contains(content, "Kitchen") {
		t.Error("Expected content to contain Kitchen")
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_All(t *testing.T) {
	t.Parallel()

	testEntities := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.test1", Platform: "hue"},
		{EntityID: "switch.test2", Platform: "mqtt"},
	}
	testDevices := []homeassistant.DeviceRegistryEntry{
		{ID: "dev1", Name: "Device 1", Manufacturer: "Philips"},
		{ID: "dev2", Name: "Device 2", Manufacturer: "IKEA"},
	}
	testAreas := []homeassistant.AreaRegistryEntry{
		{AreaID: "area1", Name: "Living Room"},
	}

	h := NewConsolidatedRegistryHandlers()
	client := &mockConsolidatedRegistryClient{
		entityRegistry: testEntities,
		deviceRegistry: testDevices,
		areaRegistry:   testAreas,
	}

	result, err := h.handleGetRegistry(context.Background(), client, map[string]any{"type": "all"})
	if err != nil {
		t.Fatalf("handleGetRegistry() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// Should contain entities section with domain summary
	if !strings.Contains(content, "## Entities") {
		t.Error("Expected content to contain '## Entities' section header")
	}
	if !strings.Contains(content, "light: 1") {
		t.Error("Expected content to contain 'light: 1' domain count")
	}

	// Should contain devices section with manufacturer summary
	if !strings.Contains(content, "## Devices") {
		t.Error("Expected content to contain '## Devices' section header")
	}
	if !strings.Contains(content, "Philips") {
		t.Error("Expected content to contain 'Philips' manufacturer")
	}

	// Should contain areas section
	if !strings.Contains(content, "## Areas") {
		t.Error("Expected content to contain '## Areas' section header")
	}
	if !strings.Contains(content, "Living Room") {
		t.Error("Expected content to contain 'Living Room' area name")
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockConsolidatedRegistryClient
		wantContains string
	}{
		{
			name:         "missing type parameter",
			args:         map[string]any{},
			client:       &mockConsolidatedRegistryClient{},
			wantContains: "type",
		},
		{
			name:         "invalid type parameter",
			args:         map[string]any{"type": "invalid"},
			client:       &mockConsolidatedRegistryClient{},
			wantContains: "Invalid type",
		},
		{
			name:         "entity registry error",
			args:         map[string]any{"type": "entities"},
			client:       &mockConsolidatedRegistryClient{entityErr: errors.New("connection failed")},
			wantContains: "Error",
		},
		{
			name:         "device registry error",
			args:         map[string]any{"type": "devices"},
			client:       &mockConsolidatedRegistryClient{deviceErr: errors.New("connection failed")},
			wantContains: "Error",
		},
		{
			name:         "area registry error",
			args:         map[string]any{"type": "areas"},
			client:       &mockConsolidatedRegistryClient{areaErr: errors.New("connection failed")},
			wantContains: "Error",
		},
		{
			name: "entities with area_id filter propagates device registry error",
			args: map[string]any{"type": "entities", "area_id": "area1"},
			client: &mockConsolidatedRegistryClient{
				entityRegistry: []homeassistant.EntityRegistryEntry{{EntityID: "light.one"}},
				deviceErr:      errors.New("connection failed"),
			},
			wantContains: "Error getting device registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewConsolidatedRegistryHandlers()
			result, err := h.handleGetRegistry(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Fatalf("handleGetRegistry() returned unexpected Go error: %v", err)
			}

			if !result.IsError {
				t.Errorf("handleGetRegistry() IsError = false, want true")
			}

			content := result.Content[0].Text
			if !strings.Contains(content, tt.wantContains) {
				t.Errorf("Error message should contain %q, got: %s", tt.wantContains, content)
			}
		})
	}
}

func TestRegisterConsolidatedRegistryTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterConsolidatedRegistryTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterConsolidatedRegistryTools() registered %d tools, want 1", len(tools))
	}

	if len(tools) > 0 && tools[0].Name != "get_registry" {
		t.Errorf("Tool name = %q, want %q", tools[0].Name, "get_registry")
	}
}

func TestConsolidatedRegistryHandlers_GetRegistryTool_HasFormatParameter(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedRegistryHandlers()
	tool := h.getRegistryTool()

	prop, ok := tool.InputSchema.Properties["format"]
	if !ok {
		t.Fatal("format property missing")
	}

	if len(prop.Enum) != 2 {
		t.Errorf("format enum has %d values, want 2 (natural, json)", len(prop.Enum))
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Entities_FormatJSON(t *testing.T) {
	t.Parallel()

	testEntities := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.test1", Platform: "hue", Name: "Test Light"},
	}

	h := NewConsolidatedRegistryHandlers()
	client := &mockConsolidatedRegistryClient{entityRegistry: testEntities}

	result, err := h.handleGetRegistry(context.Background(), client, map[string]any{
		"type":   "entities",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("handleGetRegistry() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// JSON format should contain entity_id as JSON field
	if !strings.Contains(content, `"entity_id"`) {
		t.Errorf("Expected JSON output to contain entity_id field, got: %s", content[:min(500, len(content))])
	}
	if !strings.Contains(content, `"light.test1"`) {
		t.Errorf("Expected JSON output to contain entity_id value, got: %s", content[:min(500, len(content))])
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Entities_FormatNatural(t *testing.T) {
	t.Parallel()

	testEntities := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.test1", Platform: "hue", Name: "Test Light"},
		{EntityID: "sensor.test2", Platform: "mqtt", Name: "Test Sensor"},
	}

	h := NewConsolidatedRegistryHandlers()
	client := &mockConsolidatedRegistryClient{entityRegistry: testEntities}

	result, err := h.handleGetRegistry(context.Background(), client, map[string]any{
		"type":   "entities",
		"format": "natural",
	})
	if err != nil {
		t.Fatalf("handleGetRegistry() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// Natural format should contain summary
	if !strings.Contains(content, "Found 2 entities") {
		t.Errorf("Expected natural output to contain count summary, got: %s", content[:min(500, len(content))])
	}
	// Should contain domain breakdown
	if !strings.Contains(content, "By Domain") {
		t.Errorf("Expected natural output to contain domain breakdown, got: %s", content[:min(500, len(content))])
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Devices_FormatJSON(t *testing.T) {
	t.Parallel()

	testDevices := []homeassistant.DeviceRegistryEntry{
		{ID: "dev1", Name: "Test Device", Manufacturer: "Philips"},
	}

	h := NewConsolidatedRegistryHandlers()
	client := &mockConsolidatedRegistryClient{deviceRegistry: testDevices}

	result, err := h.handleGetRegistry(context.Background(), client, map[string]any{
		"type":   "devices",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("handleGetRegistry() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// JSON format should contain id as JSON field
	if !strings.Contains(content, `"id"`) {
		t.Errorf("Expected JSON output to contain id field, got: %s", content[:min(500, len(content))])
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Areas_FormatJSON(t *testing.T) {
	t.Parallel()

	testAreas := []homeassistant.AreaRegistryEntry{
		{AreaID: "living_room", Name: "Living Room"},
	}

	h := NewConsolidatedRegistryHandlers()
	client := &mockConsolidatedRegistryClient{areaRegistry: testAreas}

	result, err := h.handleGetRegistry(context.Background(), client, map[string]any{
		"type":   "areas",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("handleGetRegistry() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// JSON format should contain area_id as JSON field
	if !strings.Contains(content, `"area_id"`) {
		t.Errorf("Expected JSON output to contain area_id field, got: %s", content[:min(500, len(content))])
	}
}

func TestConsolidatedRegistryHandlers_HandleGetRegistry_Devices_IncludeEntities(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedRegistryHandlers()

	deviceRegistry := []homeassistant.DeviceRegistryEntry{
		{ID: "device_1", Name: "Hue Bridge", Manufacturer: "Philips", AreaID: "living_room"},
		{ID: "device_2", Name: "Smart Plug", Manufacturer: "Sonoff", AreaID: "bedroom"},
	}

	entityRegistry := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.living_1", DeviceID: "device_1", Name: "Living Light 1"},
		{EntityID: "light.living_2", DeviceID: "device_1", Name: "Living Light 2"},
		{EntityID: "switch.bedroom", DeviceID: "device_2", Name: "Bedroom Switch"},
		{EntityID: "sensor.temperature", DeviceID: "", Name: "Temp Sensor"}, // no device
	}

	client := &mockConsolidatedRegistryClient{
		deviceRegistry: deviceRegistry,
		entityRegistry: entityRegistry,
	}

	tests := []struct {
		name            string
		args            map[string]any
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "include_entities true - natural format",
			args:         map[string]any{"type": "devices", "include_entities": true, "format": "natural"},
			wantContains: []string{"Hue Bridge", "light.living_1", "light.living_2", "Entities (2):"},
		},
		{
			name:            "include_entities false - natural format",
			args:            map[string]any{"type": "devices", "include_entities": false, "format": "natural"},
			wantNotContains: []string{"light.living_1", "Entities ("},
		},
		{
			name:         "include_entities true - json format",
			args:         map[string]any{"type": "devices", "include_entities": true, "format": "json"},
			wantContains: []string{"\"entities\"", "\"light.living_1\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.handleGetRegistry(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleGetRegistry() error = %v", err)
			}

			if result.IsError {
				t.Fatalf("handleGetRegistry() returned error: %v", result.Content)
			}

			content := result.Content[0].Text

			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected output to contain %q, got: %s", want, content[:min(500, len(content))])
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected output NOT to contain %q, got: %s", notWant, content[:min(500, len(content))])
				}
			}
		})
	}
}

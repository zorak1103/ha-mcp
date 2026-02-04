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

// mockConsolidatedTargetsClient implements homeassistant.Client for consolidated targets tests.
type mockConsolidatedTargetsClient struct {
	homeassistant.Client
	triggers      []string
	conditions    []string
	services      []string
	extract       *homeassistant.ExtractFromTargetResult
	triggersErr   error
	conditionsErr error
	servicesErr   error
	extractErr    error
}

func (m *mockConsolidatedTargetsClient) GetTriggersForTarget(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
	if m.triggersErr != nil {
		return nil, m.triggersErr
	}
	return m.triggers, nil
}

func (m *mockConsolidatedTargetsClient) GetConditionsForTarget(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
	if m.conditionsErr != nil {
		return nil, m.conditionsErr
	}
	return m.conditions, nil
}

func (m *mockConsolidatedTargetsClient) GetServicesForTarget(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
	if m.servicesErr != nil {
		return nil, m.servicesErr
	}
	return m.services, nil
}

func (m *mockConsolidatedTargetsClient) ExtractFromTarget(_ context.Context, _ homeassistant.Target, _ *bool) (*homeassistant.ExtractFromTargetResult, error) {
	if m.extractErr != nil {
		return nil, m.extractErr
	}
	return m.extract, nil
}

func TestNewConsolidatedTargetHandlers(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	if h == nil {
		t.Error("NewConsolidatedTargetHandlers() returned nil, want non-nil")
	}
}

func TestConsolidatedTargetHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	if len(tools) > 0 && tools[0].Name != "analyze_target" {
		t.Errorf("Registered tool name = %q, want %q", tools[0].Name, "analyze_target")
	}
}

func TestConsolidatedTargetHandlers_AnalyzeTargetTool_Schema(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	tool := h.analyzeTargetTool()

	tests := []struct {
		name      string
		checkFunc func(t *testing.T, tool mcp.Tool)
	}{
		{
			name: "has correct name",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Name != "analyze_target" {
					t.Errorf("tool.Name = %q, want %q", tool.Name, "analyze_target")
				}
			},
		},
		{
			name: "has info parameter with enum",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				prop, ok := tool.InputSchema.Properties["info"]
				if !ok {
					t.Fatal("info property missing")
				}
				if len(prop.Enum) != 5 {
					t.Errorf("info enum has %d values, want 5 (triggers, conditions, services, entities, all)", len(prop.Enum))
				}
			},
		},
		{
			name: "has target parameters",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				targets := []string{"entity_id", "device_id", "area_id", "label_id"}
				for _, target := range targets {
					if _, ok := tool.InputSchema.Properties[target]; !ok {
						t.Errorf("target property %q missing", target)
					}
				}
			},
		},
		{
			name: "has expand_group parameter",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if _, ok := tool.InputSchema.Properties["expand_group"]; !ok {
					t.Error("expand_group property missing")
				}
			},
		},
		{
			name: "has info as required",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				found := false
				for _, r := range tool.InputSchema.Required {
					if r == "info" {
						found = true
						break
					}
				}
				if !found {
					t.Error("info should be required")
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

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Triggers(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers: []string{"state", "numeric_state", "time"},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "triggers",
		"entity_id": []any{"light.living_room"},
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	if !strings.Contains(content, "state") {
		t.Error("Expected content to contain 'state' trigger")
	}
	if !strings.Contains(content, "numeric_state") {
		t.Error("Expected content to contain 'numeric_state' trigger")
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Conditions(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		conditions: []string{"state", "numeric_state"},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "conditions",
		"entity_id": []any{"light.living_room"},
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	if !strings.Contains(content, "state") {
		t.Error("Expected content to contain 'state' condition")
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Services(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		services: []string{"light.turn_on", "light.turn_off", "light.toggle"},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "services",
		"entity_id": []any{"light.living_room"},
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	if !strings.Contains(content, "light.turn_on") {
		t.Error("Expected content to contain 'light.turn_on' service")
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Entities(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		extract: &homeassistant.ExtractFromTargetResult{
			ReferencedEntities: []string{"light.living_room_1", "light.living_room_2"},
			ReferencedDevices:  []string{"device_123"},
			ReferencedAreas:    []string{"living_room"},
		},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":    "entities",
		"area_id": []any{"living_room"},
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	if !strings.Contains(content, "light.living_room_1") {
		t.Error("Expected content to contain 'light.living_room_1' entity")
	}
	if !strings.Contains(content, "device_123") {
		t.Error("Expected content to contain 'device_123' device")
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_All(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers:   []string{"state"},
		conditions: []string{"state"},
		services:   []string{"light.turn_on"},
		extract: &homeassistant.ExtractFromTargetResult{
			ReferencedEntities: []string{"light.living_room"},
		},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "all",
		"entity_id": []any{"light.living_room"},
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// Should contain all sections
	if !strings.Contains(content, "Triggers") {
		t.Error("Expected content to contain 'Triggers' section")
	}
	if !strings.Contains(content, "Conditions") {
		t.Error("Expected content to contain 'Conditions' section")
	}
	if !strings.Contains(content, "Services") {
		t.Error("Expected content to contain 'Services' section")
	}
	if !strings.Contains(content, "Entities") {
		t.Error("Expected content to contain 'Entities' section")
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockConsolidatedTargetsClient
		wantContains string
	}{
		{
			name:         "missing info parameter",
			args:         map[string]any{"entity_id": []any{"light.test"}},
			client:       &mockConsolidatedTargetsClient{},
			wantContains: "info",
		},
		{
			name:         "invalid info parameter",
			args:         map[string]any{"info": "invalid", "entity_id": []any{"light.test"}},
			client:       &mockConsolidatedTargetsClient{},
			wantContains: "Invalid info",
		},
		{
			name:         "missing target",
			args:         map[string]any{"info": "triggers"},
			client:       &mockConsolidatedTargetsClient{},
			wantContains: "at least one",
		},
		{
			name:         "triggers error",
			args:         map[string]any{"info": "triggers", "entity_id": []any{"light.test"}},
			client:       &mockConsolidatedTargetsClient{triggersErr: errors.New("connection failed")},
			wantContains: "Error",
		},
		{
			name:         "conditions error",
			args:         map[string]any{"info": "conditions", "entity_id": []any{"light.test"}},
			client:       &mockConsolidatedTargetsClient{conditionsErr: errors.New("connection failed")},
			wantContains: "Error",
		},
		{
			name:         "services error",
			args:         map[string]any{"info": "services", "entity_id": []any{"light.test"}},
			client:       &mockConsolidatedTargetsClient{servicesErr: errors.New("connection failed")},
			wantContains: "Error",
		},
		{
			name:         "extract error",
			args:         map[string]any{"info": "entities", "entity_id": []any{"light.test"}},
			client:       &mockConsolidatedTargetsClient{extractErr: errors.New("connection failed")},
			wantContains: "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewConsolidatedTargetHandlers()
			result, err := h.handleAnalyzeTarget(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Fatalf("handleAnalyzeTarget() returned unexpected Go error: %v", err)
			}

			if !result.IsError {
				t.Errorf("handleAnalyzeTarget() IsError = false, want true")
			}

			content := result.Content[0].Text
			if !strings.Contains(content, tt.wantContains) {
				t.Errorf("Error message should contain %q, got: %s", tt.wantContains, content)
			}
		})
	}
}

func TestRegisterConsolidatedTargetTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterConsolidatedTargetTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterConsolidatedTargetTools() registered %d tools, want 1", len(tools))
	}

	if len(tools) > 0 && tools[0].Name != "analyze_target" {
		t.Errorf("Tool name = %q, want %q", tools[0].Name, "analyze_target")
	}
}

func TestConsolidatedTargetHandlers_AnalyzeTargetTool_HasFormatParameter(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	tool := h.analyzeTargetTool()

	prop, ok := tool.InputSchema.Properties["format"]
	if !ok {
		t.Fatal("format property missing")
	}

	if len(prop.Enum) != 2 {
		t.Errorf("format enum has %d values, want 2 (natural, json)", len(prop.Enum))
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Triggers_FormatJSON(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers: []string{"state", "numeric_state"},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "triggers",
		"entity_id": []any{"light.living_room"},
		"format":    "json",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// JSON format should contain quoted strings
	if !strings.Contains(content, `"state"`) {
		t.Errorf("Expected JSON output to contain quoted trigger name, got: %s", content[:min(500, len(content))])
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Triggers_FormatNatural(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers: []string{"state", "numeric_state"},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "triggers",
		"entity_id": []any{"light.living_room"},
		"format":    "natural",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// Natural format should contain count summary
	if !strings.Contains(content, "2 applicable triggers") {
		t.Errorf("Expected natural output to contain count summary, got: %s", content[:min(500, len(content))])
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_Entities_FormatJSON(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		extract: &homeassistant.ExtractFromTargetResult{
			ReferencedEntities: []string{"light.living_room"},
		},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "entities",
		"entity_id": []any{"light.living_room"},
		"format":    "json",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// JSON format should contain referenced_entities JSON field
	if !strings.Contains(content, `"referenced_entities"`) {
		t.Errorf("Expected JSON output to contain referenced_entities field, got: %s", content[:min(500, len(content))])
	}
}

func TestConsolidatedTargetHandlers_HandleAnalyzeTarget_All_FormatJSON(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers:   []string{"state"},
		conditions: []string{"state"},
		services:   []string{"light.turn_on"},
		extract: &homeassistant.ExtractFromTargetResult{
			ReferencedEntities: []string{"light.living_room"},
		},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "all",
		"entity_id": []any{"light.living_room"},
		"format":    "json",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleAnalyzeTarget() returned error: %v", result.Content)
	}

	content := result.Content[0].Text

	// JSON format should contain all JSON fields
	if !strings.Contains(content, `"triggers"`) {
		t.Errorf("Expected JSON output to contain triggers field, got: %s", content[:min(500, len(content))])
	}
	if !strings.Contains(content, `"conditions"`) {
		t.Errorf("Expected JSON output to contain conditions field, got: %s", content[:min(500, len(content))])
	}
	if !strings.Contains(content, `"services"`) {
		t.Errorf("Expected JSON output to contain services field, got: %s", content[:min(500, len(content))])
	}
}

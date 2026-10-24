// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"errors"
	"slices"
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

	// gotTarget and gotExpandGroup record the last GetTriggersForTarget call.
	gotTarget      homeassistant.Target
	gotExpandGroup *bool
}

func (m *mockConsolidatedTargetsClient) GetTriggersForTarget(_ context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	m.gotTarget = target
	m.gotExpandGroup = expandGroup
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

// TestConsolidatedTargetHandlers_AllInfo_NaturalWithErrors covers formatSectionWithError error branch
// and formatExtractedSectionWithError error branch via handleAllInfo (natural format).
func TestConsolidatedTargetHandlers_AllInfo_NaturalWithErrors(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggersErr:   errors.New("triggers unavailable"),
		conditionsErr: errors.New("conditions unavailable"),
		extractErr:    errors.New("extract failed"),
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "all",
		"entity_id": []any{"light.test"},
		"format":    "natural",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "Error") {
		t.Errorf("Expected content to contain 'Error', got: %s", content)
	}
}

// TestConsolidatedTargetHandlers_AllInfo_NaturalWithMissingItems covers formatMissingItems.
func TestConsolidatedTargetHandlers_AllInfo_NaturalWithMissingItems(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers:   []string{"state"},
		conditions: []string{},
		services:   []string{},
		extract: &homeassistant.ExtractFromTargetResult{
			ReferencedEntities: []string{"light.living_room"},
			MissingDevices:     []string{"device_abc"},
			MissingAreas:       []string{"area_xyz"},
		},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "all",
		"entity_id": []any{"light.test"},
		"format":    "natural",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "Missing") {
		t.Errorf("Expected content to contain 'Missing', got: %s", content)
	}
}

// TestConsolidatedTargetHandlers_AllInfo_NaturalNilExtracted covers nil extracted in formatExtractedSectionWithError.
func TestConsolidatedTargetHandlers_AllInfo_NaturalNilExtracted(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	// extract returns nil with no error
	client := &mockConsolidatedTargetsClient{
		triggers: []string{},
		extract:  nil,
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "all",
		"entity_id": []any{"light.test"},
		"format":    "natural",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "Entities") {
		t.Errorf("Expected content to contain 'Entities' section, got: %s", content)
	}
}

// TestConsolidatedTargetHandlers_ExpandGroup covers parseTargetParams expand_group branch.
func TestConsolidatedTargetHandlers_ExpandGroup(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers: []string{"state"},
	}

	// Test with expand_group=true
	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":         "triggers",
		"entity_id":    []any{"group.lights"},
		"expand_group": true,
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}
}

func TestConsolidatedTargetHandlers_TargetArgParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       map[string]any
		wantErr    string
		wantEntity []string
		wantArea   []string
		wantExpand *bool
	}{
		{
			name:       "bare string entity_id is coerced to a one-element list",
			args:       map[string]any{"entity_id": "light.a"},
			wantEntity: []string{"light.a"},
		},
		{
			name:     "empty-string entity_id is absent next to area_id",
			args:     map[string]any{"area_id": []any{"x"}, "entity_id": ""},
			wantArea: []string{"x"},
		},
		{
			name:    "non-string element is rejected",
			args:    map[string]any{"entity_id": []any{"light.a", 5.0}},
			wantErr: "entity_id[1] must be a non-empty string",
		},
		{
			name:    "number entity_id is rejected",
			args:    map[string]any{"entity_id": 5.0},
			wantErr: "entity_id must be a string or an array of strings, got float64",
		},
		{
			name:    "string expand_group is rejected",
			args:    map[string]any{"entity_id": []any{"a"}, "expand_group": "yes"},
			wantErr: "expand_group must be a boolean, got string",
		},
		{
			name:       "bool expand_group is forwarded",
			args:       map[string]any{"entity_id": []any{"a"}, "expand_group": false},
			wantEntity: []string{"a"},
			wantExpand: func() *bool { b := false; return &b }(),
		},
		{
			name:    "nothing supplied keeps the at-least-one error",
			args:    map[string]any{},
			wantErr: "at least one of entity_id, device_id, area_id, or label_id is required",
		},
		{
			name:    "only placeholders keeps the at-least-one error",
			args:    map[string]any{"entity_id": "", "device_id": []any{}, "area_id": nil},
			wantErr: "at least one of entity_id, device_id, area_id, or label_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockConsolidatedTargetsClient{triggers: []string{"state"}}
			args := map[string]any{"info": "triggers"}
			for k, v := range tt.args {
				args[k] = v
			}

			result, err := NewConsolidatedTargetHandlers().handleAnalyzeTarget(context.Background(), client, args)
			if err != nil {
				t.Fatalf("handleAnalyzeTarget() unexpected Go error: %v", err)
			}

			if tt.wantErr != "" {
				if !result.IsError {
					t.Fatalf("IsError = false, want error containing %q", tt.wantErr)
				}
				if text := result.Content[0].Text; !strings.Contains(text, "Invalid parameters: ") || !strings.Contains(text, tt.wantErr) {
					t.Errorf("error = %q, want Invalid parameters: ... %q", text, tt.wantErr)
				}
				return
			}

			if result.IsError {
				t.Fatalf("IsError = true: %s", result.Content[0].Text)
			}
			if !slices.Equal(client.gotTarget.EntityID, tt.wantEntity) {
				t.Errorf("Target.EntityID = %v, want %v", client.gotTarget.EntityID, tt.wantEntity)
			}
			if !slices.Equal(client.gotTarget.AreaID, tt.wantArea) {
				t.Errorf("Target.AreaID = %v, want %v", client.gotTarget.AreaID, tt.wantArea)
			}
			switch {
			case tt.wantExpand == nil && client.gotExpandGroup != nil:
				t.Errorf("expand_group = %v, want nil", *client.gotExpandGroup)
			case tt.wantExpand != nil && (client.gotExpandGroup == nil || *client.gotExpandGroup != *tt.wantExpand):
				t.Errorf("expand_group = %v, want %v", client.gotExpandGroup, *tt.wantExpand)
			}
		})
	}
}

// TestConsolidatedTargetHandlers_AllInfo_WithDeviceAndAreaIDs covers parseTargetIDList for different target types.
func TestConsolidatedTargetHandlers_AllInfo_WithDeviceAndAreaIDs(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers: []string{"state"},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "triggers",
		"device_id": []any{"device_abc"},
		"area_id":   []any{"living_room"},
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}
}

// TestConsolidatedTargetHandlers_ExtractedWithReferencedDevicesAreas covers formatExtractedSectionWithError
// when devices and areas are referenced.
func TestConsolidatedTargetHandlers_ExtractedWithReferencedDevicesAreas(t *testing.T) {
	t.Parallel()

	h := NewConsolidatedTargetHandlers()
	client := &mockConsolidatedTargetsClient{
		triggers:   []string{},
		conditions: []string{},
		services:   []string{},
		extract: &homeassistant.ExtractFromTargetResult{
			ReferencedEntities: []string{"light.living_room"},
			ReferencedDevices:  []string{"device_abc"},
			ReferencedAreas:    []string{"living_room"},
		},
	}

	result, err := h.handleAnalyzeTarget(context.Background(), client, map[string]any{
		"info":      "all",
		"entity_id": []any{"light.test"},
		"format":    "natural",
	})
	if err != nil {
		t.Fatalf("handleAnalyzeTarget() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "Entities") {
		t.Errorf("Expected content to contain 'Entities', got: %s", content)
	}
}

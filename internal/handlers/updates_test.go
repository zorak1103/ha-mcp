package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestManageUpdateSchema verifies the schema for manage_update tool.
func TestManageUpdateSchema(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterUpdateTools(registry)

	tool, exists := registry.GetTool("manage_update")
	if !exists {
		t.Fatal("manage_update tool not registered")
	}

	// Verify basic properties
	if tool.Name != "manage_update" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_update")
	}

	// Verify schema
	schema := tool.InputSchema
	props := schema.Properties

	// Check action field
	actionSchema, ok := props["action"]
	if !ok {
		t.Fatal("action property missing from schema")
	}
	if len(actionSchema.Enum) != 4 {
		t.Errorf("action enum count = %d, want 4 (list, release_notes, install, skip)", len(actionSchema.Enum))
	}

	// Check required fields
	if len(schema.Required) != 1 {
		t.Errorf("required count = %d, want 1 (action)", len(schema.Required))
	}
}

// TestManageUpdate_MissingAction verifies validation when action is missing.
func TestManageUpdate_MissingAction(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewUpdateHandlers()

	result, err := handler.HandleManageUpdate(context.Background(), client, map[string]any{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageUpdate_List verifies list action.
func TestManageUpdate_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        map[string]any
		mockStates  []homeassistant.Entity
		wantContain string
	}{
		{
			name: "list all updates",
			args: map[string]any{
				"action": "list",
			},
			mockStates: []homeassistant.Entity{
				{
					EntityID: "update.hass_os",
					State:    "on",
					Attributes: map[string]any{
						"friendly_name":     "Home Assistant OS Update",
						"installed_version": "10.0",
						"latest_version":    "10.1",
						"release_summary":   "Bug fixes",
					},
				},
				{
					EntityID: "update.core",
					State:    "off",
					Attributes: map[string]any{
						"friendly_name":     "Core Update",
						"installed_version": "2024.1.0",
						"latest_version":    "2024.1.0",
					},
				},
			},
			wantContain: "Home Assistant OS Update",
		},
		{
			name: "list pending only",
			args: map[string]any{
				"action":       "list",
				"pending_only": true,
			},
			mockStates: []homeassistant.Entity{
				{
					EntityID: "update.hass_os",
					State:    "on",
					Attributes: map[string]any{
						"friendly_name":     "Home Assistant OS Update",
						"installed_version": "10.0",
						"latest_version":    "10.1",
					},
				},
			},
			wantContain: "Home Assistant OS Update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				GetStatesFn: func(context.Context) ([]homeassistant.Entity, error) {
					return tt.mockStates, nil
				},
			}

			handler := NewUpdateHandlers()
			result, err := handler.HandleManageUpdate(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || len(result.Content) == 0 {
				t.Fatal("expected result content")
			}

			text := result.Content[0].Text
			if !strings.Contains(text, tt.wantContain) {
				t.Errorf("result text does not contain %q: %s", tt.wantContain, text)
			}
		})
	}
}

// TestManageUpdate_ReleaseNotes verifies release_notes action.
func TestManageUpdate_ReleaseNotes(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
			if cmd != "update/release_notes" {
				return nil, fmt.Errorf("wrong command: %s", cmd)
			}
			if data["entity_id"] != "update.hass_os" {
				return nil, fmt.Errorf("data[entity_id] = %v, want %q", data["entity_id"], "update.hass_os")
			}
			return map[string]any{
				"release_notes": "### New Features\n- Feature 1\n- Feature 2",
			}, nil
		},
	}

	handler := NewUpdateHandlers()
	result, err := handler.HandleManageUpdate(context.Background(), client, map[string]any{
		"action":    "release_notes",
		"entity_id": "update.hass_os",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Feature 1") {
		t.Errorf("result text does not contain release notes: %s", text)
	}
}

// TestManageUpdate_ReleaseNotesMissingEntityID verifies validation.
func TestManageUpdate_ReleaseNotesMissingEntityID(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewUpdateHandlers()

	result, err := handler.HandleManageUpdate(context.Background(), client, map[string]any{
		"action": "release_notes",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageUpdate_Install verifies install action.
func TestManageUpdate_Install(t *testing.T) {
	t.Parallel()

	var capturedData map[string]any
	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "update" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "install" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			capturedData = data
			return nil, nil
		},
	}

	handler := NewUpdateHandlers()
	result, err := handler.HandleManageUpdate(context.Background(), client, map[string]any{
		"action":    "install",
		"entity_id": "update.hass_os",
		"backup":    false,
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}

	// Verify backup parameter was passed
	if capturedData["backup"] != false {
		t.Errorf("backup = %v, want false", capturedData["backup"])
	}
}

// TestManageUpdate_List_JSONFormat verifies list action returns JSON when requested.
func TestManageUpdate_List_JSONFormat(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetStatesFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{
				{
					EntityID: "update.hass_os",
					State:    "on",
					Attributes: map[string]any{
						"friendly_name":     "Home Assistant OS",
						"installed_version": "10.0",
						"latest_version":    "10.1",
					},
				},
			}, nil
		},
	}

	handler := NewUpdateHandlers()
	result, err := handler.HandleManageUpdate(context.Background(), client, map[string]any{
		"action": "list",
		"format": "json",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "update.hass_os") {
		t.Errorf("JSON result does not contain entity_id: %s", text)
	}
}

// TestManageUpdate_Install_WithVersion verifies install action passes version parameter.
func TestManageUpdate_Install_WithVersion(t *testing.T) {
	t.Parallel()

	var capturedData map[string]any
	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "update" || service != "install" {
				return nil, fmt.Errorf("wrong call: %s.%s", domain, service)
			}
			capturedData = data
			return nil, nil
		},
	}

	handler := NewUpdateHandlers()
	result, err := handler.HandleManageUpdate(context.Background(), client, map[string]any{
		"action":    "install",
		"entity_id": "update.hass_os",
		"version":   "10.1",
		"backup":    true,
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}
	if capturedData["version"] != "10.1" {
		t.Errorf("version = %v, want %q", capturedData["version"], "10.1")
	}
}

// TestManageUpdate_Skip verifies skip action.
func TestManageUpdate_Skip(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, name string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "update" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if name != "skip" {
				return nil, fmt.Errorf("wrong service: %s", name)
			}
			if data["entity_id"] != "update.core" {
				return nil, fmt.Errorf("data[entity_id] = %v, want %q", data["entity_id"], "update.core")
			}
			return nil, nil
		},
	}

	handler := NewUpdateHandlers()
	result, err := handler.HandleManageUpdate(context.Background(), client, map[string]any{
		"action":    "skip",
		"entity_id": "update.core",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "skipped") {
		t.Errorf("result does not indicate skip: %s", text)
	}
}

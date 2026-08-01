package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestManageBlueprintSchema verifies the schema for manage_blueprint tool.
func TestManageBlueprintSchema(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterBlueprintTools(registry)

	tool, exists := registry.GetTool("manage_blueprint")
	if !exists {
		t.Fatal("manage_blueprint tool not registered")
	}

	// Verify basic properties
	if tool.Name != "manage_blueprint" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_blueprint")
	}
	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	// Verify schema properties
	schema := tool.InputSchema
	props := schema.Properties

	// Check action field
	actionSchema, ok := props["action"]
	if !ok {
		t.Fatal("action property missing from schema")
	}
	if actionSchema.Type != "string" {
		t.Errorf("action type = %q, want %q", actionSchema.Type, "string")
	}
	if len(actionSchema.Enum) != 2 {
		t.Errorf("action enum count = %d, want 2 (list, import)", len(actionSchema.Enum))
	}

	// Check domain field
	domainSchema, ok := props["domain"]
	if !ok {
		t.Fatal("domain property missing from schema")
	}
	if len(domainSchema.Enum) != 2 {
		t.Errorf("domain enum count = %d, want 2 (automation, script)", len(domainSchema.Enum))
	}

	// Check format field
	formatSchema, ok := props["format"]
	if !ok {
		t.Fatal("format property missing from schema")
	}
	if len(formatSchema.Enum) != 2 {
		t.Errorf("format enum count = %d, want 2", len(formatSchema.Enum))
	}

	// Check required fields
	if len(schema.Required) != 1 {
		t.Errorf("required count = %d, want 1 (action)", len(schema.Required))
	}
	if schema.Required[0] != "action" {
		t.Errorf("required[0] = %q, want %q", schema.Required[0], "action")
	}
}

// TestManageBlueprint_MissingAction verifies validation when action is missing.
func TestManageBlueprint_MissingAction(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewBlueprintHandlers()

	result, err := handler.HandleManageBlueprint(context.Background(), client, map[string]any{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result with error message")
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

// TestManageBlueprint_InvalidAction verifies validation for invalid action.
func TestManageBlueprint_InvalidAction(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewBlueprintHandlers()

	result, err := handler.HandleManageBlueprint(context.Background(), client, map[string]any{
		"action": "invalid_action",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result with error message")
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

// TestManageBlueprint_List verifies list action.
func TestManageBlueprint_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		mockResponse any
		wantContain  string
	}{
		{
			name: "list automation blueprints natural format",
			args: map[string]any{
				"action": "list",
				"domain": "automation",
			},
			mockResponse: map[string]any{
				"blueprints/automation/homeassistant/motion_light.yaml": map[string]any{
					"metadata": map[string]any{
						"name":   "Motion-activated Light",
						"source": "builtin",
					},
				},
			},
			wantContain: "Motion-activated Light",
		},
		{
			name: "list script blueprints json format",
			args: map[string]any{
				"action": "list",
				"domain": "script",
				"format": "json",
			},
			mockResponse: map[string]any{
				"blueprints/script/custom/notify.yaml": map[string]any{
					"metadata": map[string]any{
						"name": "Notification Script",
					},
				},
			},
			wantContain: "notify.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
					if cmd != "blueprint/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					if domain, _ := tt.args["domain"].(string); domain != "" {
						if data["domain"] != domain {
							return nil, fmt.Errorf("data[domain] = %v, want %q", data["domain"], domain)
						}
					}
					return tt.mockResponse, nil
				},
			}

			handler := NewBlueprintHandlers()
			result, err := handler.HandleManageBlueprint(context.Background(), client, tt.args)

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

// TestValidateBlueprintURL tests the URL validator in isolation.
func TestValidateBlueprintURL(t *testing.T) {
	t.Parallel()

	longURL := "https://example.com/" + strings.Repeat("a", blueprintURLMaxLength)

	tests := []struct {
		name        string
		url         string
		wantErr     bool
		wantContain string
	}{
		{name: "valid https public", url: "https://example.com/bp.yaml", wantErr: false},
		{name: "valid github raw", url: "https://raw.githubusercontent.com/user/repo/main/bp.yaml", wantErr: false},
		{name: "http rejected", url: "http://example.com/bp.yaml", wantErr: true, wantContain: "https"},
		{name: "file scheme rejected", url: "file:///etc/passwd", wantErr: true, wantContain: "https"},
		{name: "no scheme rejected", url: "example.com/bp.yaml", wantErr: true, wantContain: "invalid url"},
		{name: "invalid url rejected", url: "::::", wantErr: true, wantContain: "invalid url"},
		{name: "too long rejected", url: longURL, wantErr: true, wantContain: "too long"},
		{name: "loopback IPv4 rejected", url: "https://127.0.0.1/bp.yaml", wantErr: true, wantContain: "loopback"},
		{name: "loopback IPv6 rejected", url: "https://[::1]/bp.yaml", wantErr: true, wantContain: "loopback"},
		{name: "localhost rejected", url: "https://localhost/bp.yaml", wantErr: true, wantContain: "loopback"},
		{name: "link-local cloud metadata rejected", url: "https://169.254.169.254/latest/meta-data/", wantErr: true, wantContain: "link-local"},
		{name: "RFC1918 10/8 rejected", url: "https://10.0.0.5/bp.yaml", wantErr: true, wantContain: "private"},
		{name: "RFC1918 172.16/12 rejected", url: "https://172.16.0.1/bp.yaml", wantErr: true, wantContain: "private"},
		{name: "RFC1918 192.168/16 rejected", url: "https://192.168.1.1/bp.yaml", wantErr: true, wantContain: "private"},
		{name: "unspecified 0.0.0.0 rejected", url: "https://0.0.0.0/bp.yaml", wantErr: true, wantContain: "unspecified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateBlueprintURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateBlueprintURL(%q) = nil, want error containing %q", tt.url, tt.wantContain)
					return
				}
				if tt.wantContain != "" && !strings.Contains(err.Error(), tt.wantContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantContain)
				}
			} else {
				if err != nil {
					t.Errorf("validateBlueprintURL(%q) = %v, want nil", tt.url, err)
				}
			}
		})
	}
}

// TestManageBlueprint_Import verifies import action.
func TestManageBlueprint_Import(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		wantErr      bool
		wantContain  string
		expectWSCall bool
	}{
		{
			name: "successful import",
			args: map[string]any{
				"action": "import",
				"url":    "https://example.com/blueprint.yaml",
			},
			wantErr:      false,
			wantContain:  "successfully imported",
			expectWSCall: true,
		},
		{
			name: "missing url",
			args: map[string]any{
				"action": "import",
			},
			wantErr:      true,
			wantContain:  "url is required",
			expectWSCall: false,
		},
		{
			name:         "http url rejected",
			args:         map[string]any{"action": "import", "url": "http://example.com/bp.yaml"},
			wantErr:      true,
			wantContain:  "invalid blueprint url",
			expectWSCall: false,
		},
		{
			name:         "cloud metadata endpoint rejected",
			args:         map[string]any{"action": "import", "url": "https://169.254.169.254/latest/meta-data/"},
			wantErr:      true,
			wantContain:  "invalid blueprint url",
			expectWSCall: false,
		},
		{
			name:         "loopback rejected",
			args:         map[string]any{"action": "import", "url": "https://127.0.0.1/bp.yaml"},
			wantErr:      true,
			wantContain:  "invalid blueprint url",
			expectWSCall: false,
		},
		{
			name:         "private network rejected",
			args:         map[string]any{"action": "import", "url": "https://192.168.1.1/bp.yaml"},
			wantErr:      true,
			wantContain:  "invalid blueprint url",
			expectWSCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wsCalled := false
			client := &UniversalMockClient{
				SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
					wsCalled = true
					if !tt.expectWSCall {
						t.Errorf("SendHACSCommand must not be called for rejected url, but was called with cmd=%q data=%v", cmd, data)
					}
					if cmd != "blueprint/import" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					if data["url"] == nil || data["url"] == "" {
						return nil, fmt.Errorf("expected url in data, got: %v", data)
					}
					return map[string]any{"success": true}, nil
				},
			}

			handler := NewBlueprintHandlers()
			result, err := handler.HandleManageBlueprint(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || len(result.Content) == 0 {
				t.Fatal("expected result content")
			}
			if result.IsError != tt.wantErr {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantErr)
			}
			text := result.Content[0].Text
			if !strings.Contains(text, tt.wantContain) {
				t.Errorf("result text does not contain %q: %s", tt.wantContain, text)
			}
			if tt.expectWSCall && !wsCalled {
				t.Error("expected SendHACSCommand to be called but it was not")
			}
		})
	}
}

// TestManageBlueprint_ListMissingDomain verifies that domain is required for list.
func TestManageBlueprint_ListMissingDomain(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewBlueprintHandlers()

	result, err := handler.HandleManageBlueprint(context.Background(), client, map[string]any{
		"action": "list",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result with error message")
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

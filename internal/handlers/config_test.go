package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestConfigHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewConfigHandlers()
	registry := mcp.NewRegistry()
	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "validate_config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validate_config tool to be registered")
	}
}

func TestConfigHandlers_ValidateConfigTool(t *testing.T) {
	t.Parallel()

	h := NewConfigHandlers()
	tool := h.validateConfigTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "validate_config",
		RequiredParams:  []string{},
		OptionalParams:  []string{},
		WantDescription: true,
	})
}

func TestConfigHandlers_HandleValidateConfig(t *testing.T) {
	t.Parallel()

	errorMessage := "Integration 'invalid_integration' not found"

	tests := []handlerTestCase{
		{
			name: "valid configuration",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.CheckConfigFn = func(_ context.Context) (*homeassistant.ConfigCheckResult, error) {
					return &homeassistant.ConfigCheckResult{
						Result: "valid",
						Errors: nil,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Configuration is valid", `"valid": true`, `"result": "valid"`},
		},
		{
			name: "invalid configuration",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.CheckConfigFn = func(_ context.Context) (*homeassistant.ConfigCheckResult, error) {
					return &homeassistant.ConfigCheckResult{
						Result: "invalid",
						Errors: &errorMessage,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Configuration is invalid", `"valid": false`, `"result": "invalid"`, "invalid_integration"},
		},
		{
			name: "API error",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.CheckConfigFn = func(_ context.Context) (*homeassistant.ConfigCheckResult, error) {
					return nil, errors.New("connection refused")
				}
			},
			wantError:    true,
			wantContains: []string{"Error checking configuration", "connection refused"},
		},
	}

	h := NewConfigHandlers()
	runHandlerTestCases(t, tests, h.handleValidateConfig)
}

func TestConfigHandlers_ResponseFormat(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CheckConfigFn: func(_ context.Context) (*homeassistant.ConfigCheckResult, error) {
			return &homeassistant.ConfigCheckResult{
				Result: "valid",
				Errors: nil,
			}, nil
		},
	}

	h := NewConfigHandlers()
	result, err := h.handleValidateConfig(context.Background(), client, map[string]any{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("expected no error in result")
	}

	content := result.Content[0].Text

	expectedFields := []string{
		"valid",
		"result",
	}

	for _, field := range expectedFields {
		if !strings.Contains(content, field) {
			t.Errorf("expected field %q in output", field)
		}
	}

	if !strings.Contains(content, "Configuration is valid") {
		t.Error("expected summary line in output")
	}
}

func TestConfigHandlers_InvalidConfigWithErrors(t *testing.T) {
	t.Parallel()

	errorDetails := "Invalid configuration entry light.living_room: Unknown schema"

	client := &UniversalMockClient{
		CheckConfigFn: func(_ context.Context) (*homeassistant.ConfigCheckResult, error) {
			return &homeassistant.ConfigCheckResult{
				Result: "invalid",
				Errors: &errorDetails,
			}, nil
		},
	}

	h := NewConfigHandlers()
	result, err := h.handleValidateConfig(context.Background(), client, map[string]any{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("expected no error in result (invalid config is a valid response)")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "Configuration is invalid") {
		t.Error("expected 'Configuration is invalid' in output")
	}
	if !strings.Contains(content, "Unknown schema") {
		t.Error("expected error details in output")
	}
}

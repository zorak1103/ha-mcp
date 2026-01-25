package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestServiceHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewServiceHandlers()
	registry := mcp.NewRegistry()
	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "list_services" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected list_services tool to be registered")
	}
}

func TestServiceHandlers_ListServicesTool(t *testing.T) {
	t.Parallel()

	h := NewServiceHandlers()
	tool := h.listServicesTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "list_services",
		RequiredParams:  []string{},
		OptionalParams:  []string{"domain"},
		WantDescription: true,
	})
}

func TestServiceHandlers_HandleListServices(t *testing.T) {
	t.Parallel()

	testServices := []homeassistant.Service{
		{
			Domain: "light",
			Services: map[string]homeassistant.ServiceDefinition{
				"turn_on": {
					Name:        "Turn on",
					Description: "Turn on a light",
					Fields: map[string]homeassistant.ServiceField{
						"brightness": {
							Name:        "Brightness",
							Description: "Brightness level",
							Required:    false,
						},
					},
				},
				"turn_off": {
					Name:        "Turn off",
					Description: "Turn off a light",
				},
			},
		},
		{
			Domain: "switch",
			Services: map[string]homeassistant.ServiceDefinition{
				"turn_on":  {Name: "Turn on"},
				"turn_off": {Name: "Turn off"},
				"toggle":   {Name: "Toggle"},
			},
		},
	}

	tests := []handlerTestCase{
		{
			name: "list all services",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetServicesFn = func(_ context.Context) ([]homeassistant.Service, error) {
					return testServices, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light", "switch", "service_count"},
		},
		{
			name: "filter by domain",
			args: map[string]any{"domain": "light"},
			setupMock: func(m *UniversalMockClient) {
				m.GetServicesFn = func(_ context.Context) ([]homeassistant.Service, error) {
					return testServices, nil
				}
			},
			wantError:    false,
			wantContains: []string{"turn_on", "turn_off", "brightness"},
		},
		{
			name: "filter by domain - case insensitive",
			args: map[string]any{"domain": "LIGHT"},
			setupMock: func(m *UniversalMockClient) {
				m.GetServicesFn = func(_ context.Context) ([]homeassistant.Service, error) {
					return testServices, nil
				}
			},
			wantError:    false,
			wantContains: []string{"turn_on", "turn_off"},
		},
		{
			name: "domain not found",
			args: map[string]any{"domain": "unknown"},
			setupMock: func(m *UniversalMockClient) {
				m.GetServicesFn = func(_ context.Context) ([]homeassistant.Service, error) {
					return testServices, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No services found for domain 'unknown'"},
		},
		{
			name: "empty services",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetServicesFn = func(_ context.Context) ([]homeassistant.Service, error) {
					return []homeassistant.Service{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 0 service domain(s)"},
		},
		{
			name: "API error",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetServicesFn = func(_ context.Context) ([]homeassistant.Service, error) {
					return nil, errors.New("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"Error getting services", "connection failed"},
		},
	}

	h := NewServiceHandlers()
	runHandlerTestCases(t, tests, h.handleListServices)
}

func TestServiceHandlers_ServiceWithTarget(t *testing.T) {
	t.Parallel()

	testServices := []homeassistant.Service{
		{
			Domain: "light",
			Services: map[string]homeassistant.ServiceDefinition{
				"turn_on": {
					Name:        "Turn on",
					Description: "Turn on a light",
					Target: &homeassistant.ServiceTarget{
						Entity: []homeassistant.TargetSelector{
							{Domain: "light"},
						},
					},
				},
			},
		},
	}

	client := &UniversalMockClient{
		GetServicesFn: func(_ context.Context) ([]homeassistant.Service, error) {
			return testServices, nil
		},
	}

	h := NewServiceHandlers()
	result, err := h.handleListServices(context.Background(), client, map[string]any{"domain": "light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("expected no error in result")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "has_target") {
		t.Error("expected has_target field in output")
	}
}

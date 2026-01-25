package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestSystemHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewSystemHandlers()
	registry := mcp.NewRegistry()
	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "get_system_info" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected get_system_info tool to be registered")
	}
}

func TestSystemHandlers_GetSystemInfoTool(t *testing.T) {
	t.Parallel()

	h := NewSystemHandlers()
	tool := h.getSystemInfoTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "get_system_info",
		RequiredParams:  []string{},
		OptionalParams:  []string{},
		WantDescription: true,
	})
}

func TestSystemHandlers_HandleGetSystemInfo(t *testing.T) {
	t.Parallel()

	testConfig := &homeassistant.Config{
		Version:      "2024.1.0",
		State:        "RUNNING",
		LocationName: "Test Home",
		TimeZone:     "Europe/Berlin",
		Latitude:     52.52,
		Longitude:    13.405,
		Elevation:    34,
		Currency:     "EUR",
		Country:      "DE",
		Language:     "de",
		SafeMode:     false,
		InternalURL:  "http://homeassistant.local:8123",
		ExternalURL:  "https://home.example.com",
		Components:   []string{"light", "switch", "sensor", "automation"},
		UnitSystem: homeassistant.UnitSystem{
			Length:      "km",
			Mass:        "kg",
			Pressure:    "Pa",
			Temperature: "°C",
			Volume:      "L",
			WindSpeed:   "m/s",
		},
	}

	tests := []handlerTestCase{
		{
			name: "successful system info",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigFn = func(_ context.Context) (*homeassistant.Config, error) {
					return testConfig, nil
				}
			},
			wantError: false,
			wantContains: []string{
				"2024.1.0",
				"RUNNING",
				"Test Home",
				"Europe/Berlin",
				"component_count",
			},
		},
		{
			name: "API error",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigFn = func(_ context.Context) (*homeassistant.Config, error) {
					return nil, errors.New("connection refused")
				}
			},
			wantError:    true,
			wantContains: []string{"Error getting system config", "connection refused"},
		},
	}

	h := NewSystemHandlers()
	runHandlerTestCases(t, tests, h.handleGetSystemInfo)
}

func TestSystemHandlers_ResponseFormat(t *testing.T) {
	t.Parallel()

	testConfig := &homeassistant.Config{
		Version:      "2024.2.0",
		State:        "RUNNING",
		LocationName: "My House",
		TimeZone:     "UTC",
		Latitude:     0.0,
		Longitude:    0.0,
		Components:   []string{"a", "b", "c"},
		UnitSystem: homeassistant.UnitSystem{
			Temperature: "°C",
		},
	}

	client := &UniversalMockClient{
		GetConfigFn: func(_ context.Context) (*homeassistant.Config, error) {
			return testConfig, nil
		},
	}

	h := NewSystemHandlers()
	result, err := h.handleGetSystemInfo(context.Background(), client, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("expected no error in result")
	}

	content := result.Content[0].Text

	expectedFields := []string{
		"version",
		"state",
		"location_name",
		"time_zone",
		"unit_system",
	}

	for _, field := range expectedFields {
		if !strings.Contains(content, field) {
			t.Errorf("expected field %q in output", field)
		}
	}

	if !strings.Contains(content, "Home Assistant 2024.2.0 (RUNNING)") {
		t.Error("expected summary line in output")
	}
}

func TestSystemHandlers_ComponentCount(t *testing.T) {
	t.Parallel()

	testConfig := &homeassistant.Config{
		Version:    "2024.1.0",
		State:      "RUNNING",
		Components: []string{"comp1", "comp2", "comp3", "comp4", "comp5"},
	}

	client := &UniversalMockClient{
		GetConfigFn: func(_ context.Context) (*homeassistant.Config, error) {
			return testConfig, nil
		},
	}

	h := NewSystemHandlers()
	result, err := h.handleGetSystemInfo(context.Background(), client, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := result.Content[0].Text
	if !strings.Contains(content, `"component_count": 5`) {
		t.Error("expected component_count to be 5")
	}
}

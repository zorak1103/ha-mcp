package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestSystemLogHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewSystemLogHandlers()
	registry := mcp.NewRegistry()
	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Fatalf("RegisterTools() registered %d tools, want 1", len(tools))
	}
	if tools[0].Name != "manage_system_log" {
		t.Errorf("tool name = %q, want manage_system_log", tools[0].Name)
	}
}

func TestSystemLogHandlers_Schema(t *testing.T) {
	t.Parallel()

	h := NewSystemLogHandlers()
	tool := h.manageSystemLogTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "manage_system_log",
		RequiredParams:  []string{"action"},
		OptionalParams:  []string{"level", "limit", "integration", "include_exception", "format"},
		WantDescription: true,
	})
}

func TestSystemLogHandlers_HandleManageSystemLog(t *testing.T) {
	t.Parallel()

	mqttEntry := homeassistant.SystemLogEntry{
		Name:          "homeassistant.components.mqtt",
		Message:       []string{"Connection lost"},
		Level:         "WARNING",
		Source:        []any{"components/mqtt/client.py", float64(123)},
		Timestamp:     1705312800.0,
		Exception:     "",
		Count:         2,
		FirstOccurred: 1705312800.0,
	}
	coreErrorEntry := homeassistant.SystemLogEntry{
		Name:          "homeassistant.core",
		Message:       []string{"Something went wrong"},
		Level:         "ERROR",
		Source:        []any{"homeassistant/core.py", float64(456)},
		Timestamp:     1705312900.0,
		Exception:     "Traceback (most recent call last):\n  File 'core.py', line 456, in run\nValueError: bad value",
		Count:         1,
		FirstOccurred: 1705312900.0,
	}
	zwaveEntry := homeassistant.SystemLogEntry{
		Name:          "homeassistant.components.zwave_js",
		Message:       []string{"Node unavailable"},
		Level:         "WARNING",
		Source:        []any{"components/zwave_js/node.py", float64(789)},
		Timestamp:     1705312850.0,
		Exception:     "",
		Count:         3,
		FirstOccurred: 1705312850.0,
	}

	h := NewSystemLogHandlers()

	tests := []handlerTestCase{
		{
			name: "list happy path - all entries returned",
			args: map[string]any{"action": "list"},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return []homeassistant.SystemLogEntry{mqttEntry, coreErrorEntry, zwaveEntry}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 3 system log", "homeassistant.components.mqtt", "homeassistant.core", "homeassistant.components.zwave_js"},
		},
		{
			name: "list with level=error filter",
			args: map[string]any{"action": "list", "level": "error"},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return []homeassistant.SystemLogEntry{mqttEntry, coreErrorEntry, zwaveEntry}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"homeassistant.core", "ERROR"},
			wantNotContains: []string{"homeassistant.components.mqtt", "homeassistant.components.zwave_js"},
		},
		{
			name: "list with integration substring filter",
			args: map[string]any{"action": "list", "integration": "mqtt"},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return []homeassistant.SystemLogEntry{mqttEntry, coreErrorEntry, zwaveEntry}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"homeassistant.components.mqtt"},
			wantNotContains: []string{"homeassistant.core", "homeassistant.components.zwave_js"},
		},
		{
			name: "list with include_exception=false strips stack traces",
			args: map[string]any{"action": "list", "include_exception": false},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return []homeassistant.SystemLogEntry{coreErrorEntry}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"homeassistant.core"},
			wantNotContains: []string{"Traceback", "ValueError"},
		},
		{
			name: "list with limit=1 truncates results",
			args: map[string]any{"action": "list", "limit": float64(1)},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return []homeassistant.SystemLogEntry{mqttEntry, coreErrorEntry, zwaveEntry}, nil
				}
			},
			wantError:       false,
			wantContains:    []string{"Found 1 system log entry", "homeassistant.components.mqtt"},
			wantNotContains: []string{"homeassistant.core", "homeassistant.components.zwave_js"},
		},
		{
			name: "list with format=json returns valid JSON",
			args: map[string]any{"action": "list", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return []homeassistant.SystemLogEntry{mqttEntry}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"homeassistant.components.mqtt", "WARNING"},
		},
		{
			name: "list empty result",
			args: map[string]any{"action": "list", "level": "critical"},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return []homeassistant.SystemLogEntry{mqttEntry, coreErrorEntry}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No system log entries"},
		},
		{
			name: "list propagates client error",
			args: map[string]any{"action": "list"},
			setupMock: func(m *UniversalMockClient) {
				m.GetSystemLogFn = func(_ context.Context) ([]homeassistant.SystemLogEntry, error) {
					return nil, errors.New("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"connection failed"},
		},
		{
			name: "clear happy path",
			args: map[string]any{"action": "clear"},
			setupMock: func(m *UniversalMockClient) {
				m.ClearSystemLogFn = func(_ context.Context) error { return nil }
			},
			wantError:    false,
			wantContains: []string{"System log cleared"},
		},
		{
			name: "clear propagates error",
			args: map[string]any{"action": "clear"},
			setupMock: func(m *UniversalMockClient) {
				m.ClearSystemLogFn = func(_ context.Context) error { return errors.New("ws disconnected") }
			},
			wantError:    true,
			wantContains: []string{"ws disconnected"},
		},
		{
			name:         "invalid action",
			args:         map[string]any{"action": "delete"},
			wantError:    true,
			wantContains: []string{"invalid action", "delete"},
		},
		{
			name:         "missing action",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"invalid action"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageSystemLog)
}

func TestSystemLogHandlers_JSONFormatValid(t *testing.T) {
	t.Parallel()

	entry := homeassistant.SystemLogEntry{
		Name:    "homeassistant.core",
		Message: []string{"error msg"},
		Level:   "ERROR",
	}

	output, err := formatSystemLogJSON([]homeassistant.SystemLogEntry{entry}, true)
	if err != nil {
		t.Fatalf("formatSystemLogJSON() error = %v", err)
	}

	var parsed []homeassistant.SystemLogEntry
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nGot: %s", err, output)
	}
	if len(parsed) != 1 {
		t.Errorf("parsed %d entries, want 1", len(parsed))
	}
	if parsed[0].Name != "homeassistant.core" {
		t.Errorf("entry name = %q, want homeassistant.core", parsed[0].Name)
	}
}

func TestSystemLogHandlers_JSONFormatStripsException(t *testing.T) {
	t.Parallel()

	entry := homeassistant.SystemLogEntry{
		Name:      "homeassistant.core",
		Message:   []string{"err"},
		Level:     "ERROR",
		Exception: "Traceback...",
	}

	output, err := formatSystemLogJSON([]homeassistant.SystemLogEntry{entry}, false)
	if err != nil {
		t.Fatalf("formatSystemLogJSON() error = %v", err)
	}

	var parsed []homeassistant.SystemLogEntry
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v", err)
	}
	if parsed[0].Exception != "" {
		t.Errorf("exception not stripped; got %q", parsed[0].Exception)
	}
}

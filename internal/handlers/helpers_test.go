package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockHelpersClient implements homeassistant.Client for helpers tests.
type mockHelpersClient struct {
	homeassistant.Client
	ListHelpersFn func(ctx context.Context) ([]homeassistant.Entity, error)
}

func (m *mockHelpersClient) ListHelpers(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListHelpersFn != nil {
		return m.ListHelpersFn(ctx)
	}
	return nil, nil
}

func TestNewHelperHandlers(t *testing.T) {
	t.Parallel()

	h := NewHelperHandlers()

	if h == nil {
		t.Error("NewHelperHandlers() returned nil, want non-nil")
	}
}

func TestHelperHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewHelperHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	// Check that list_helpers is registered
	found := false
	for _, tool := range tools {
		if tool.Name == "list_helpers" {
			found = true
			break
		}
	}
	if !found {
		t.Error("RegisterTools() did not register 'list_helpers' tool")
	}
}

func TestHelperHandlers_listHelpersTool(t *testing.T) {
	t.Parallel()

	h := NewHelperHandlers()
	tool := h.listHelpersTool()

	tests := []struct {
		name      string
		checkFunc func(t *testing.T, tool mcp.Tool)
	}{
		{
			name: "has correct name",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Name != "list_helpers" {
					t.Errorf("tool.Name = %q, want %q", tool.Name, "list_helpers")
				}
			},
		},
		{
			name: "has description",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Description == "" {
					t.Error("tool.Description is empty, want non-empty")
				}
			},
		},
		{
			name: "has object schema type",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.InputSchema.Type != testSchemaTypeObject {
					t.Errorf("tool.InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
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

func TestHelperHandlers_handleListHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		client       *mockHelpersClient
		wantContains string
		wantError    bool
	}{
		{
			name: "success with helpers",
			client: &mockHelpersClient{
				ListHelpersFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID: "input_boolean.test_helper_1",
							State:    "on",
						},
						{
							EntityID: "input_number.test_helper_2",
							State:    "42",
						},
					}, nil
				},
			},
			wantContains: "input_boolean.test_helper_1",
			wantError:    false,
		},
		{
			name: "success with empty list",
			client: &mockHelpersClient{
				ListHelpersFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{}, nil
				},
			},
			wantContains: "[]",
			wantError:    false,
		},
		{
			name: "client error",
			client: &mockHelpersClient{
				ListHelpersFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return nil, errors.New("connection failed")
				},
			},
			wantContains: "Error listing helpers",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewHelperHandlers()
			result, err := h.handleListHelpers(context.Background(), tt.client, nil)

			if err != nil {
				t.Errorf("handleListHelpers() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("handleListHelpers() returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleListHelpers() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleListHelpers() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleListHelpers() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

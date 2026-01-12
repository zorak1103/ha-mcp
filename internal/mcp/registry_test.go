// Package mcp implements the Model Context Protocol (MCP) server.
//
//nolint:nilnil // Test file - mock handlers returning nil,nil is acceptable for testing existence checks
package mcp

import (
	"context"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.tools == nil {
		t.Error("tools map is nil")
	}
	if r.resources == nil {
		t.Error("resources map is nil")
	}
	if r.ToolCount() != 0 {
		t.Errorf("ToolCount() = %d, want 0", r.ToolCount())
	}
	if r.ResourceCount() != 0 {
		t.Errorf("ResourceCount() = %d, want 0", r.ResourceCount())
	}
}

func TestRegistry_RegisterTool(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: JSONSchema{Type: "object"},
	}
	handler := func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
		return &ToolsCallResult{Content: []ContentBlock{NewTextContent("test")}}, nil
	}

	r.RegisterTool(tool, handler)

	if r.ToolCount() != 1 {
		t.Errorf("ToolCount() = %d, want 1", r.ToolCount())
	}

	got, exists := r.GetTool("test_tool")
	if !exists {
		t.Fatal("GetTool() returned false, want true")
	}
	if diff := cmp.Diff(tool, got); diff != "" {
		t.Errorf("GetTool() mismatch (-want +got):\n%s", diff)
	}
}

func TestRegistry_RegisterResource(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	resource := Resource{
		URI:         "test://resource",
		Name:        "Test Resource",
		Description: "A test resource",
		MimeType:    "text/plain",
	}
	handler := func(_ context.Context, _ homeassistant.Client, _ string) (*ResourcesReadResult, error) {
		return &ResourcesReadResult{}, nil
	}

	r.RegisterResource(resource, handler)

	if r.ResourceCount() != 1 {
		t.Errorf("ResourceCount() = %d, want 1", r.ResourceCount())
	}

	got, exists := r.GetResource("test://resource")
	if !exists {
		t.Fatal("GetResource() returned false, want true")
	}
	if diff := cmp.Diff(resource, got); diff != "" {
		t.Errorf("GetResource() mismatch (-want +got):\n%s", diff)
	}
}

func TestRegistry_ListTools(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	// Empty registry
	tools := r.ListTools()
	if len(tools) != 0 {
		t.Errorf("ListTools() on empty registry returned %d tools, want 0", len(tools))
	}

	// Add tools
	tool1 := Tool{Name: "tool_a", Description: "Tool A"}
	tool2 := Tool{Name: "tool_b", Description: "Tool B"}
	r.RegisterTool(tool1, nil)
	r.RegisterTool(tool2, nil)

	tools = r.ListTools()
	if len(tools) != 2 {
		t.Errorf("ListTools() returned %d tools, want 2", len(tools))
	}

	// Check all tools are present (order may vary)
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}
	if !toolNames["tool_a"] || !toolNames["tool_b"] {
		t.Errorf("ListTools() missing expected tools, got %v", toolNames)
	}
}

func TestRegistry_ListResources(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	// Empty registry
	resources := r.ListResources()
	if len(resources) != 0 {
		t.Errorf("ListResources() on empty registry returned %d resources, want 0", len(resources))
	}

	// Add resources
	res1 := Resource{URI: "test://a", Name: "Resource A"}
	res2 := Resource{URI: "test://b", Name: "Resource B"}
	r.RegisterResource(res1, nil)
	r.RegisterResource(res2, nil)

	resources = r.ListResources()
	if len(resources) != 2 {
		t.Errorf("ListResources() returned %d resources, want 2", len(resources))
	}

	// Check all resources are present
	resourceURIs := make(map[string]bool)
	for _, res := range resources {
		resourceURIs[res.URI] = true
	}
	if !resourceURIs["test://a"] || !resourceURIs["test://b"] {
		t.Errorf("ListResources() missing expected resources, got %v", resourceURIs)
	}
}

func TestRegistry_GetHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupTools []string
		lookupName string
		wantExists bool
	}{
		{
			name:       "existing tool",
			setupTools: []string{"tool_a", "tool_b"},
			lookupName: "tool_a",
			wantExists: true,
		},
		{
			name:       "non-existing tool",
			setupTools: []string{"tool_a"},
			lookupName: "tool_b",
			wantExists: false,
		},
		{
			name:       "empty registry",
			setupTools: nil,
			lookupName: "any",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewRegistry()
			for _, name := range tt.setupTools {
				r.RegisterTool(Tool{Name: name}, func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
					return nil, nil
				})
			}

			handler, exists := r.GetHandler(tt.lookupName)
			if exists != tt.wantExists {
				t.Errorf("GetHandler() exists = %v, want %v", exists, tt.wantExists)
			}
			if tt.wantExists && handler == nil {
				t.Error("GetHandler() returned nil handler for existing tool")
			}
		})
	}
}

func TestRegistry_GetResourceHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupURIs  []string
		lookupURI  string
		wantExists bool
	}{
		{
			name:       "existing resource",
			setupURIs:  []string{"test://a", "test://b"},
			lookupURI:  "test://a",
			wantExists: true,
		},
		{
			name:       "non-existing resource",
			setupURIs:  []string{"test://a"},
			lookupURI:  "test://b",
			wantExists: false,
		},
		{
			name:       "empty registry",
			setupURIs:  nil,
			lookupURI:  "test://any",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewRegistry()
			for _, uri := range tt.setupURIs {
				r.RegisterResource(Resource{URI: uri}, func(_ context.Context, _ homeassistant.Client, _ string) (*ResourcesReadResult, error) {
					return nil, nil
				})
			}

			handler, exists := r.GetResourceHandler(tt.lookupURI)
			if exists != tt.wantExists {
				t.Errorf("GetResourceHandler() exists = %v, want %v", exists, tt.wantExists)
			}
			if tt.wantExists && handler == nil {
				t.Error("GetResourceHandler() returned nil handler for existing resource")
			}
		})
	}
}

func TestRegistry_GetTool(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tool := Tool{Name: "my_tool", Description: "My Tool"}
	r.RegisterTool(tool, nil)

	// Existing tool
	got, exists := r.GetTool("my_tool")
	if !exists {
		t.Error("GetTool() returned false for existing tool")
	}
	if got.Name != "my_tool" {
		t.Errorf("GetTool().Name = %q, want %q", got.Name, "my_tool")
	}

	// Non-existing tool
	_, exists = r.GetTool("nonexistent")
	if exists {
		t.Error("GetTool() returned true for non-existing tool")
	}
}

func TestRegistry_GetResource(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	resource := Resource{URI: "test://uri", Name: "Test"}
	r.RegisterResource(resource, nil)

	// Existing resource
	got, exists := r.GetResource("test://uri")
	if !exists {
		t.Error("GetResource() returned false for existing resource")
	}
	if got.URI != "test://uri" {
		t.Errorf("GetResource().URI = %q, want %q", got.URI, "test://uri")
	}

	// Non-existing resource
	_, exists = r.GetResource("nonexistent")
	if exists {
		t.Error("GetResource() returned true for non-existing resource")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	const numGoroutines = 100
	const numOperations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 different operation types

	// Concurrent tool registration
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				r.RegisterTool(Tool{Name: "tool_" + string(rune('a'+id%26))}, nil)
			}
		}(i)
	}

	// Concurrent resource registration
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				r.RegisterResource(Resource{URI: "test://" + string(rune('a'+id%26))}, nil)
			}
		}(i)
	}

	// Concurrent tool listing
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = r.ListTools()
			}
		}()
	}

	// Concurrent resource listing
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = r.ListResources()
			}
		}()
	}

	wg.Wait()

	// Should not panic and should have some tools/resources
	if r.ToolCount() == 0 {
		t.Error("ToolCount() = 0 after concurrent writes")
	}
	if r.ResourceCount() == 0 {
		t.Error("ResourceCount() = 0 after concurrent writes")
	}
}

func TestTruncateDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		desc   string
		maxLen int
		want   string
	}{
		{
			name:   "short description",
			desc:   "Short",
			maxLen: 10,
			want:   "Short",
		},
		{
			name:   "exact length",
			desc:   "Exactly10!",
			maxLen: 10,
			want:   "Exactly10!",
		},
		{
			name:   "long description",
			desc:   "This is a very long description that needs truncation",
			maxLen: 20,
			want:   "This is a very lo...",
		},
		{
			name:   "empty description",
			desc:   "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := truncateDescription(tt.desc, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateDescription(%q, %d) = %q, want %q", tt.desc, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestRegistry_LogRegisteredTools(t *testing.T) {
	t.Parallel()

	t.Run("nil logger", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		r.RegisterTool(Tool{Name: "test_tool"}, nil)

		// Should not panic
		r.LogRegisteredTools(nil)
	})

	t.Run("debug disabled", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		r.RegisterTool(Tool{Name: "test_tool"}, nil)

		logger := logging.New(logging.LevelInfo) // Debug is disabled
		// Should not panic and should return early
		r.LogRegisteredTools(logger)
	})

	t.Run("debug enabled with tools and resources", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		r.RegisterTool(Tool{Name: "tool_b", Description: "Tool B"}, nil)
		r.RegisterTool(Tool{Name: "tool_a", Description: "Tool A"}, nil)
		r.RegisterResource(Resource{URI: "test://b", Name: "Resource B"}, nil)
		r.RegisterResource(Resource{URI: "test://a", Name: "Resource A"}, nil)

		// Create a logger with debug enabled - logs go to stdout
		logger := logging.New(logging.LevelDebug)
		// Should not panic
		r.LogRegisteredTools(logger)
	})

	t.Run("empty registry with debug", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		logger := logging.New(logging.LevelDebug)
		// Should not panic with empty registry
		r.LogRegisteredTools(logger)
	})

	t.Run("tools only no resources", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		r.RegisterTool(Tool{Name: "tool_only"}, nil)
		logger := logging.New(logging.LevelDebug)
		// Should not panic
		r.LogRegisteredTools(logger)
	})
}

func TestRegistry_ToolCount_ResourceCount(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	if got := r.ToolCount(); got != 0 {
		t.Errorf("initial ToolCount() = %d, want 0", got)
	}
	if got := r.ResourceCount(); got != 0 {
		t.Errorf("initial ResourceCount() = %d, want 0", got)
	}

	r.RegisterTool(Tool{Name: "t1"}, nil)
	r.RegisterTool(Tool{Name: "t2"}, nil)
	r.RegisterTool(Tool{Name: "t3"}, nil)

	if got := r.ToolCount(); got != 3 {
		t.Errorf("ToolCount() after 3 registrations = %d, want 3", got)
	}

	r.RegisterResource(Resource{URI: "r1"}, nil)
	r.RegisterResource(Resource{URI: "r2"}, nil)

	if got := r.ResourceCount(); got != 2 {
		t.Errorf("ResourceCount() after 2 registrations = %d, want 2", got)
	}

	// Overwrite existing
	r.RegisterTool(Tool{Name: "t1"}, nil)
	if got := r.ToolCount(); got != 3 {
		t.Errorf("ToolCount() after overwrite = %d, want 3", got)
	}
}

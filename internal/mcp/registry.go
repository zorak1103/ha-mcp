// Package mcp implements the Model Context Protocol (MCP) server.
package mcp

import (
	"context"
	"sort"
	"sync"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/logging"
)

// ToolHandler is a function that handles a tool call.
type ToolHandler func(ctx context.Context, client homeassistant.Client, args map[string]any) (*ToolsCallResult, error)

// ResourceHandler is a function that handles a resource read.
type ResourceHandler func(ctx context.Context, client homeassistant.Client, uri string) (*ResourcesReadResult, error)

// toolEntry holds a tool definition and its handler.
type toolEntry struct {
	tool    Tool
	handler ToolHandler
}

// resourceEntry holds a resource definition and its handler.
type resourceEntry struct {
	resource Resource
	handler  ResourceHandler
}

// Registry manages MCP tools and resources.
type Registry struct {
	mu        sync.RWMutex
	tools     map[string]toolEntry
	resources map[string]resourceEntry
}

// NewRegistry creates a new tool and resource registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:     make(map[string]toolEntry),
		resources: make(map[string]resourceEntry),
	}
}

// RegisterTool registers a tool with its handler.
func (r *Registry) RegisterTool(tool Tool, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = toolEntry{
		tool:    tool,
		handler: handler,
	}
}

// RegisterResource registers a resource with its handler.
func (r *Registry) RegisterResource(resource Resource, handler ResourceHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resources[resource.URI] = resourceEntry{
		resource: resource,
		handler:  handler,
	}
}

// ListTools returns all registered tools.
func (r *Registry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, entry := range r.tools {
		tools = append(tools, entry.tool)
	}
	return tools
}

// ListResources returns all registered resources.
func (r *Registry) ListResources() []Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	resources := make([]Resource, 0, len(r.resources))
	for _, entry := range r.resources {
		resources = append(resources, entry.resource)
	}
	return resources
}

// GetHandler returns the handler for a tool by name.
func (r *Registry) GetHandler(name string) (ToolHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.tools[name]
	if !exists {
		return nil, false
	}
	return entry.handler, true
}

// GetResourceHandler returns the handler for a resource by URI.
func (r *Registry) GetResourceHandler(uri string) (ResourceHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.resources[uri]
	if !exists {
		return nil, false
	}
	return entry.handler, true
}

// GetTool returns a tool by name.
func (r *Registry) GetTool(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.tools[name]
	if !exists {
		return Tool{}, false
	}
	return entry.tool, true
}

// GetResource returns a resource by URI.
func (r *Registry) GetResource(uri string) (Resource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.resources[uri]
	if !exists {
		return Resource{}, false
	}
	return entry.resource, true
}

// ToolCount returns the number of registered tools.
func (r *Registry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// ResourceCount returns the number of registered resources.
func (r *Registry) ResourceCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.resources)
}

// maxDescriptionLen is the maximum length for tool descriptions in log output.
const maxDescriptionLen = 80

// LogRegisteredTools logs all registered tools at Debug level.
// This is useful for debugging and verifying that all expected tools are available.
func (r *Registry) LogRegisteredTools(logger *logging.Logger) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if logger == nil || !logger.IsDebugEnabled() {
		return
	}

	// Collect and sort tool names for consistent output
	toolNames := make([]string, 0, len(r.tools))
	for name := range r.tools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	logger.Debug("Registered MCP tools:")
	for _, name := range toolNames {
		entry := r.tools[name]
		logger.Debug("  - "+name, "description", truncateDescription(entry.tool.Description, maxDescriptionLen))
	}

	// Also log resources if any
	if len(r.resources) > 0 {
		resourceURIs := make([]string, 0, len(r.resources))
		for uri := range r.resources {
			resourceURIs = append(resourceURIs, uri)
		}
		sort.Strings(resourceURIs)

		logger.Debug("Registered MCP resources:")
		for _, uri := range resourceURIs {
			entry := r.resources[uri]
			logger.Debug("  - "+uri, "name", entry.resource.Name)
		}
	}
}

// truncateDescription truncates a description to maxLen characters.
func truncateDescription(desc string, maxLen int) string {
	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen-3] + "..."
}

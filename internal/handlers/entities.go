// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// EntityHandlers provides MCP tool handlers for entity operations.
type EntityHandlers struct{}

// NewEntityHandlers creates a new EntityHandlers instance.
func NewEntityHandlers() *EntityHandlers {
	return &EntityHandlers{}
}

// RegisterTools registers all entity-related tools with the registry.
func (h *EntityHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getStatesTool(), h.handleGetStates)
	registry.RegisterTool(h.getStateTool(), h.handleGetState)
	registry.RegisterTool(h.getHistoryTool(), h.handleGetHistory)
	registry.RegisterTool(h.listDomainsTool(), h.handleListDomains)
}

func (h *EntityHandlers) getStatesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_states",
		Description: "Get all entity states from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Optional filters for entity states",
			Properties: map[string]mcp.JSONSchema{
				"domain": {
					Type:        "string",
					Description: "Filter by domain (e.g., 'light', 'switch', 'sensor')",
				},
			},
		},
	}
}

func (h *EntityHandlers) getStateTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_state",
		Description: "Get the state of a specific entity",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for getting entity state",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The entity ID (e.g., 'light.living_room')",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *EntityHandlers) getHistoryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_history",
		Description: "Get historical state changes for an entity",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for getting entity history",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The entity ID (e.g., 'sensor.temperature')",
				},
				"start_time": {
					Type:        "string",
					Description: "Start time in RFC3339 format (default: 24 hours ago)",
				},
				"end_time": {
					Type:        "string",
					Description: "End time in RFC3339 format (default: now)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *EntityHandlers) listDomainsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_domains",
		Description: "List all available entity domains in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
		},
	}
}

func (h *EntityHandlers) handleGetStates(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	states, err := client.GetStates(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting states: %v", err))},
			IsError: true,
		}, nil
	}

	// Filter by domain if specified
	domain, _ := args["domain"].(string)
	if domain != "" {
		filtered := make([]homeassistant.Entity, 0)
		prefix := domain + "."
		for _, state := range states {
			if strings.HasPrefix(state.EntityID, prefix) {
				filtered = append(filtered, state)
			}
		}
		states = filtered
	}

	// Format output
	output, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting states: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

func (h *EntityHandlers) handleGetState(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting state: %v", err))},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting state: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

func (h *EntityHandlers) handleGetHistory(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Parse time parameters
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr, ok := args["start_time"].(string); ok && startStr != "" {
		parsed, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Invalid start_time format: %v", err))},
				IsError: true,
			}, nil
		}
		startTime = parsed
	}

	if endStr, ok := args["end_time"].(string); ok && endStr != "" {
		parsed, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Invalid end_time format: %v", err))},
				IsError: true,
			}, nil
		}
		endTime = parsed
	}

	history, err := client.GetHistory(ctx, entityID, startTime, endTime)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting history: %v", err))},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting history: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

func (h *EntityHandlers) handleListDomains(ctx context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
	states, err := client.GetStates(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting states: %v", err))},
			IsError: true,
		}, nil
	}

	// Extract unique domains using index lookup for efficiency
	domainSet := make(map[string]int)
	for _, state := range states {
		if idx := strings.Index(state.EntityID, "."); idx > 0 {
			domainSet[state.EntityID[:idx]]++
		}
	}

	// Build result
	type domainInfo struct {
		Domain string `json:"domain"`
		Count  int    `json:"entity_count"`
	}

	domains := make([]domainInfo, 0, len(domainSet))
	for domain, count := range domainSet {
		domains = append(domains, domainInfo{
			Domain: domain,
			Count:  count,
		})
	}

	output, err := json.MarshalIndent(domains, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting domains: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const configKeyEntityID = attrEntityID

// EntityHandlers provides MCP tool handlers for entity operations.
type EntityHandlers struct{}

// NewEntityHandlers creates a new EntityHandlers instance.
func NewEntityHandlers() *EntityHandlers {
	return &EntityHandlers{}
}

// RegisterTools registers all entity-related tools with the registry.
func (h *EntityHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getStateTool(), h.handleGetState)
}

func (h *EntityHandlers) getStateTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_state",
		Description: "Get the state of one or more entities. By default returns natural language format optimized for LLMs. Use 'format=json' for structured data.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for getting entity state(s)",
			Properties: map[string]mcp.JSONSchema{
				attrEntityID: {
					Type:        "string",
					Description: "Single entity ID (e.g., 'light.living_room'). Use entity_id OR entity_ids, not both.",
				},
				"entity_ids": {
					Type:        "array",
					Description: "Array of entity IDs for batch query (e.g., ['light.living_room', 'light.bedroom']). Use entity_id OR entity_ids, not both.",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'natural' (default, human-readable LLM-optimized) or 'json' (structured data)",
					Enum:        []string{"natural", "json"},
				},
			},
		},
	}
}

// getStringArg safely extracts a string argument.
func getStringArg(args map[string]any, key string) string {
	val, _ := args[key].(string)
	return val
}

// getBoolArg safely extracts a boolean argument.
func getBoolArg(args map[string]any, key string) bool {
	val, _ := args[key].(bool)
	return val
}

func (h *EntityHandlers) handleGetState(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, hasSingle := args[attrEntityID].(string)
	entityIDs, hasBatch := args["entity_ids"]

	// Validate: exactly one of entity_id or entity_ids must be provided
	if hasSingle && hasBatch {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("Cannot specify both entity_id and entity_ids. Use one or the other.")},
			IsError: true,
		}, nil
	}

	if !hasSingle && !hasBatch {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id or entity_ids is required")},
			IsError: true,
		}, nil
	}

	// Single entity mode
	if hasSingle {
		if entityID == "" {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
				IsError: true,
			}, nil
		}
		return h.handleGetStateSingle(ctx, client, entityID, args)
	}

	// Batch mode
	return h.handleGetStateBatch(ctx, client, entityIDs, args)
}

func (h *EntityHandlers) handleGetStateSingle(ctx context.Context, client homeassistant.Client, entityID string, args map[string]any) (*mcp.ToolsCallResult, error) {
	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting state: %v", err))},
			IsError: true,
		}, nil
	}

	// Use formatter based on format parameter
	format := formatter.ParseFormat(getStringArg(args, "format"))
	f := formatter.New(format)

	output, err := f.FormatEntity(ctx, *state)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting state: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(output)},
	}, nil
}

func (h *EntityHandlers) handleGetStateBatch(ctx context.Context, client homeassistant.Client, entityIDsArg any, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Parse entity_ids array
	entityIDsArray, ok := entityIDsArg.([]any)
	if !ok {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_ids must be an array")},
			IsError: true,
		}, nil
	}

	var entityIDs []string
	for _, id := range entityIDsArray {
		if idStr, ok := id.(string); ok && idStr != "" {
			entityIDs = append(entityIDs, idStr)
		}
	}

	if len(entityIDs) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_ids array is empty")},
			IsError: true,
		}, nil
	}

	// Get all states and filter
	allStates, err := client.GetStates(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting states: %v", err))},
			IsError: true,
		}, nil
	}

	// Build map for quick lookup
	stateMap := make(map[string]homeassistant.Entity)
	for _, state := range allStates {
		stateMap[state.EntityID] = state
	}

	// Collect states in order requested
	var foundStates []homeassistant.Entity
	var notFound []string
	for _, entityID := range entityIDs {
		if state, ok := stateMap[entityID]; ok {
			foundStates = append(foundStates, state)
		} else {
			notFound = append(notFound, entityID)
		}
	}

	// Use formatter based on format parameter
	format := formatter.ParseFormat(getStringArg(args, "format"))

	if format == formatter.FormatNatural {
		return h.formatBatchNatural(ctx, foundStates, notFound)
	}

	return h.formatBatchJSON(foundStates, notFound)
}

func (h *EntityHandlers) formatBatchNatural(ctx context.Context, states []homeassistant.Entity, notFound []string) (*mcp.ToolsCallResult, error) {
	var output strings.Builder
	f := formatter.New(formatter.FormatNatural)

	for _, state := range states {
		line, err := f.FormatEntity(ctx, state)
		if err != nil {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting state: %v", err))},
				IsError: true,
			}, nil
		}
		output.WriteString(line)
		output.WriteString("\n")
	}

	// Add not found entities
	for _, entityID := range notFound {
		fmt.Fprintf(&output, "%s: not found\n", entityID)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(strings.TrimSuffix(output.String(), "\n"))},
	}, nil
}

func (h *EntityHandlers) formatBatchJSON(states []homeassistant.Entity, notFound []string) (*mcp.ToolsCallResult, error) {
	type batchResult struct {
		States   []homeassistant.Entity `json:"states"`
		NotFound []string               `json:"not_found,omitempty"`
	}

	result := batchResult{
		States:   states,
		NotFound: notFound,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting JSON: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(data))},
	}, nil
}

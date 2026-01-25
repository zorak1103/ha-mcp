// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// ConfigHandlers provides handlers for Home Assistant configuration validation operations.
type ConfigHandlers struct{}

// NewConfigHandlers creates a new ConfigHandlers instance.
func NewConfigHandlers() *ConfigHandlers {
	return &ConfigHandlers{}
}

// RegisterTools registers all configuration-related tools with the registry.
func (h *ConfigHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.validateConfigTool(), h.handleValidateConfig)
}

// validateConfigTool returns the tool definition for validating Home Assistant configuration.
func (h *ConfigHandlers) validateConfigTool() mcp.Tool {
	return mcp.Tool{
		Name:        "validate_config",
		Description: "Validate Home Assistant configuration.yaml for syntax and integration errors. Returns whether the configuration is valid and any error messages.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
		},
	}
}

// configValidationResponse represents the configuration validation result.
type configValidationResponse struct {
	Valid  bool    `json:"valid"`
	Result string  `json:"result"`
	Errors *string `json:"errors,omitempty"`
}

// handleValidateConfig handles requests to validate Home Assistant configuration.
func (h *ConfigHandlers) handleValidateConfig(
	ctx context.Context,
	client homeassistant.Client,
	_ map[string]any,
) (*mcp.ToolsCallResult, error) {
	result, err := client.CheckConfig(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error checking configuration: %v", err)),
			},
			IsError: true,
		}, nil
	}

	response := configValidationResponse{
		Valid:  result.Result == "valid",
		Result: result.Result,
		Errors: result.Errors,
	}

	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	var summary string
	if response.Valid {
		summary = "Configuration is valid"
	} else {
		summary = "Configuration is invalid"
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + string(output)),
		},
	}, nil
}

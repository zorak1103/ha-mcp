// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// SystemHandlers provides handlers for Home Assistant system information operations.
type SystemHandlers struct{}

// NewSystemHandlers creates a new SystemHandlers instance.
func NewSystemHandlers() *SystemHandlers {
	return &SystemHandlers{}
}

// RegisterTools registers all system-related tools with the registry.
func (h *SystemHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getSystemInfoTool(), h.handleGetSystemInfo)
}

// getSystemInfoTool returns the tool definition for getting system info.
func (h *SystemHandlers) getSystemInfoTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_system_info",
		Description: "Get Home Assistant system configuration including version, timezone, location, unit system, and loaded components. Useful for debugging and understanding the HA installation.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
		},
	}
}

// systemInfoResponse represents a curated view of system information.
type systemInfoResponse struct {
	Version      string     `json:"version"`
	State        string     `json:"state"`
	LocationName string     `json:"location_name"`
	TimeZone     string     `json:"time_zone"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	Elevation    int        `json:"elevation"`
	Currency     string     `json:"currency,omitempty"`
	Country      string     `json:"country,omitempty"`
	Language     string     `json:"language,omitempty"`
	UnitSystem   unitSystem `json:"unit_system"`
	SafeMode     bool       `json:"safe_mode"`
	InternalURL  string     `json:"internal_url,omitempty"`
	ExternalURL  string     `json:"external_url,omitempty"`
	Components   int        `json:"component_count"`
}

// unitSystem represents the configured unit system in a simplified format.
type unitSystem struct {
	Length      string `json:"length"`
	Mass        string `json:"mass"`
	Pressure    string `json:"pressure"`
	Temperature string `json:"temperature"`
	Volume      string `json:"volume"`
	WindSpeed   string `json:"wind_speed"`
}

// handleGetSystemInfo handles requests to get system information.
func (h *SystemHandlers) handleGetSystemInfo(
	ctx context.Context,
	client homeassistant.Client,
	_ map[string]any,
) (*mcp.ToolsCallResult, error) {
	config, err := client.GetConfig(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting system config: %v", err)),
			},
			IsError: true,
		}, nil
	}

	info := systemInfoResponse{
		Version:      config.Version,
		State:        config.State,
		LocationName: config.LocationName,
		TimeZone:     config.TimeZone,
		Latitude:     config.Latitude,
		Longitude:    config.Longitude,
		Elevation:    config.Elevation,
		Currency:     config.Currency,
		Country:      config.Country,
		Language:     config.Language,
		SafeMode:     config.SafeMode,
		InternalURL:  config.InternalURL,
		ExternalURL:  config.ExternalURL,
		Components:   len(config.Components),
		UnitSystem: unitSystem{
			Length:      config.UnitSystem.Length,
			Mass:        config.UnitSystem.Mass,
			Pressure:    config.UnitSystem.Pressure,
			Temperature: config.UnitSystem.Temperature,
			Volume:      config.UnitSystem.Volume,
			WindSpeed:   config.UnitSystem.WindSpeed,
		},
	}

	output, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Home Assistant %s (%s)", config.Version, config.State)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + string(output)),
		},
	}, nil
}

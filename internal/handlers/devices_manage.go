package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Device action constants.
const (
	deviceActionGet    = "get"
	deviceActionUpdate = "update"
	deviceActionDelete = "delete"
)

// DeviceManageHandlers provides handlers for device registry management.
type DeviceManageHandlers struct{}

// NewDeviceManageHandlers creates a new DeviceManageHandlers instance.
func NewDeviceManageHandlers() *DeviceManageHandlers {
	return &DeviceManageHandlers{}
}

// RegisterTools registers the manage_device tool with the registry.
func (h *DeviceManageHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageDeviceTool(), h.handleManageDevice)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *DeviceManageHandlers) manageDeviceTool() mcp.Tool {
	return mcp.Tool{
		Name: "manage_device",
		Description: `Manage Home Assistant device registry entries - get, update, or delete.

Actions:
- get: Get device registry details (requires device_id)
- update: Update device registry entry (requires device_id and at least one field to update)
- delete: Delete device from registry (requires device_id; the integration must support device removal)

Safe fields that can be updated:
- name_by_user: Custom display name (empty string removes override)
- area_id: Area assignment (empty string removes)
- disabled_by: 'user' to disable, 'none' to enable
- labels: Array of label strings; use label_mode to control merge behavior
- label_mode: 'add' (default, append), 'remove' (subtract), 'replace' (full replacement)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Device registry management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: get, update, delete",
					Enum:        []string{"get", "update", "delete"},
				},
				"device_id": {
					Type:        "string",
					Description: "Device ID. Required for all actions.",
				},
				"name_by_user": {
					Type:        "string",
					Description: "Custom display name (update only, empty string removes override)",
				},
				"area_id": {
					Type:        "string",
					Description: "Area ID (update only, empty string removes assignment)",
				},
				"disabled_by": {
					Type:        "string",
					Description: "Disable status (update only): 'user' to disable, 'none' to enable",
					Enum:        []string{"user", "none"},
				},
				"labels": {
					Type:        "array",
					Description: "Labels array (update only); use label_mode to control merge behavior",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
				"label_mode": arrayModeSchema("labels"),
				"format": {
					Type:        "string",
					Description: "Output format: 'natural' (human-readable, default) or 'json' (structured)",
					Enum:        []string{"natural", "json"},
				},
			},
			Required: []string{"action"},
		},
	}
}

// =============================================================================
// Handler Implementation
// =============================================================================

func (h *DeviceManageHandlers) handleManageDevice(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return errorResult("action is required and must be a string (get, update, or delete)"), nil
	}

	format := formatNatural
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	switch action {
	case deviceActionGet:
		return h.handleGetDevice(ctx, client, args, format)
	case deviceActionUpdate:
		return h.handleUpdateDevice(ctx, client, args, format)
	case deviceActionDelete:
		return h.handleDeleteDevice(ctx, client, args, format)
	default:
		return errorResult(fmt.Sprintf("unsupported action '%s'. Valid actions: get, update, delete", action)), nil
	}
}

func (h *DeviceManageHandlers) handleGetDevice(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return errorResult("device_id is required for get action"), nil
	}

	// Get device registry
	registry, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device registry: %w", err)
	}

	// Find device
	for _, entry := range registry {
		if entry.ID == deviceID {
			if format == formatJSON {
				return h.formatDeviceJSON(&entry)
			}
			return h.formatDeviceNatural(&entry), nil
		}
	}

	return errorResult(fmt.Sprintf("device '%s' not found in registry", deviceID)), nil
}

func (h *DeviceManageHandlers) handleUpdateDevice(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return errorResult("device_id is required for update action"), nil
	}

	config, hasFields := h.buildDeviceUpdateConfig(args)

	labelMode := getArrayMode(args, "label_mode")
	labels, hasLabels := getStringSlice(args, "labels")

	if hasLabels {
		entry, fetchErr := h.fetchDeviceForMerge(ctx, client, deviceID, labelMode)
		if fetchErr != nil {
			return errorResult(fetchErr.Error()), nil
		}
		config.Labels = applyArrayMode(entry.Labels, labels, labelMode)
		hasFields = true
	}

	if !hasFields {
		return errorResult("at least one field must be provided for update (name_by_user, area_id, disabled_by, labels)"), nil
	}

	// Update device
	updated, err := client.UpdateDeviceRegistryEntry(ctx, deviceID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}

	if format == formatJSON {
		return h.formatDeviceJSON(updated)
	}
	return h.formatDeviceNaturalWithSuccess(updated), nil
}

func (h *DeviceManageHandlers) handleDeleteDevice(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return errorResult("device_id is required for delete action"), nil
	}

	// Fetch device registry to find the device
	devices, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device registry: %w", err)
	}

	var device homeassistant.DeviceRegistryEntry
	found := false
	for _, d := range devices {
		if d.ID == deviceID {
			device = d
			found = true
			break
		}
	}
	if !found {
		return errorResult(fmt.Sprintf("device '%s' not found in registry", deviceID)), nil
	}

	// Fetch config entries needed for removal
	configEntries, err := client.GetConfigEntries(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get config entries: %w", err)
	}

	configEntryMap := buildConfigEntryMap(configEntries)
	success, errMsg := removeDeviceConfigEntries(ctx, client, device, configEntryMap)
	if !success {
		return errorResult(fmt.Sprintf("failed to delete device '%s': %s", deviceID, errMsg)), nil
	}

	if format == formatJSON {
		data, err := json.Marshal(map[string]any{"device_id": deviceID, "deleted": true})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
		return textResult(string(data)), nil
	}
	return textResult(fmt.Sprintf("Device '%s' deleted from registry.", deviceID)), nil
}

// fetchDeviceForMerge fetches the device registry entry for label merge.
// Skips the registry call when label mode is replace. Returns a not-found error
// for add/remove mode when the device is absent. Never returns a nil pointer when
// err is nil.
func (h *DeviceManageHandlers) fetchDeviceForMerge(
	ctx context.Context,
	client homeassistant.Client,
	deviceID string,
	labelMode string,
) (*homeassistant.DeviceRegistryEntry, error) {
	if labelMode == arrayModeReplace {
		return &homeassistant.DeviceRegistryEntry{}, nil
	}
	registry, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device registry: %w", err)
	}
	for i := range registry {
		if registry[i].ID == deviceID {
			return &registry[i], nil
		}
	}
	return nil, fmt.Errorf("device '%s' not found in registry", deviceID)
}

// =============================================================================
// Helper Functions
// =============================================================================

func (h *DeviceManageHandlers) buildDeviceUpdateConfig(args map[string]any) (homeassistant.DeviceRegistryUpdateConfig, bool) {
	config := homeassistant.DeviceRegistryUpdateConfig{}
	hasFields := false

	if nameByUser, ok := args["name_by_user"].(string); ok {
		config.NameByUser = &nameByUser
		hasFields = true
	}

	if areaID, ok := args["area_id"].(string); ok {
		config.AreaID = &areaID
		hasFields = true
	}

	if disabledBy, ok := args["disabled_by"].(string); ok {
		// Map "none" to empty string for HA API
		if disabledBy == "none" {
			disabledBy = ""
		}
		config.DisabledBy = &disabledBy
		hasFields = true
	}

	return config, hasFields
}

// =============================================================================
// Formatters
// =============================================================================

func (h *DeviceManageHandlers) formatDeviceNatural(entry *homeassistant.DeviceRegistryEntry) *mcp.ToolsCallResult {
	var parts []string
	parts = append(parts, fmt.Sprintf("Device ID: %s", entry.ID))

	if entry.Name != "" {
		parts = append(parts, fmt.Sprintf("Name: %s", entry.Name))
	}

	if entry.NameByUser != "" {
		parts = append(parts, fmt.Sprintf("Name by user: %s", entry.NameByUser))
	}

	if entry.Manufacturer != "" {
		parts = append(parts, fmt.Sprintf("Manufacturer: %s", entry.Manufacturer))
	}

	if string(entry.Model) != "" {
		parts = append(parts, fmt.Sprintf("Model: %s", string(entry.Model)))
	}

	if entry.AreaID != "" {
		parts = append(parts, fmt.Sprintf("Area ID: %s", entry.AreaID))
	}

	if entry.DisabledBy != "" {
		parts = append(parts, fmt.Sprintf("Disabled by: %s", entry.DisabledBy))
	}

	if len(entry.Labels) > 0 {
		parts = append(parts, fmt.Sprintf("Labels: %s", strings.Join(entry.Labels, ", ")))
	}

	if string(entry.SWVersion) != "" {
		parts = append(parts, fmt.Sprintf("SW Version: %s", string(entry.SWVersion)))
	}

	if string(entry.HWVersion) != "" {
		parts = append(parts, fmt.Sprintf("HW Version: %s", string(entry.HWVersion)))
	}

	if len(entry.ConfigEntries) > 0 {
		parts = append(parts, fmt.Sprintf("Config Entries: %d", len(entry.ConfigEntries)))
	}

	return textResult(strings.Join(parts, "\n"))
}

func (h *DeviceManageHandlers) formatDeviceNaturalWithSuccess(entry *homeassistant.DeviceRegistryEntry) *mcp.ToolsCallResult {
	details := h.formatDeviceNatural(entry).Content[0].Text
	return textResult(fmt.Sprintf("Device '%s' updated successfully.\n\n%s", entry.ID, details))
}

func (h *DeviceManageHandlers) formatDeviceJSON(entry *homeassistant.DeviceRegistryEntry) (*mcp.ToolsCallResult, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device: %w", err)
	}
	return textResult(string(data)), nil
}

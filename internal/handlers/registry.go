// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// entityRegistryFilter encapsulates all filter criteria for entity registry queries.
type entityRegistryFilter struct {
	domain          string
	platform        string
	deviceID        string
	areaID          string
	includeDisabled bool
	deviceIDsInArea map[string]bool
}

// newEntityRegistryFilterFromArgs creates a filter from tool arguments.
func newEntityRegistryFilterFromArgs(args map[string]any) *entityRegistryFilter {
	domain, _ := args["domain"].(string)
	platform, _ := args["platform"].(string)
	deviceID, _ := args["device_id"].(string)
	areaID, _ := args["area_id"].(string)
	includeDisabled, _ := args["include_disabled"].(bool)

	return &entityRegistryFilter{
		domain:          domain,
		platform:        platform,
		deviceID:        deviceID,
		areaID:          areaID,
		includeDisabled: includeDisabled,
		deviceIDsInArea: make(map[string]bool),
	}
}

// matches returns true if the entry passes all filter criteria.
func (f *entityRegistryFilter) matches(entry homeassistant.EntityRegistryEntry) bool {
	if !f.includeDisabled && entry.DisabledBy != "" {
		return false
	}

	if f.domain != "" && extractDomain(entry.EntityID) != f.domain {
		return false
	}

	if f.platform != "" && entry.Platform != f.platform {
		return false
	}

	if f.deviceID != "" && entry.DeviceID != f.deviceID {
		return false
	}

	if f.areaID != "" {
		directMatch := entry.AreaID == f.areaID
		deviceMatch := entry.DeviceID != "" && f.deviceIDsInArea[entry.DeviceID]
		if !directMatch && !deviceMatch {
			return false
		}
	}

	return true
}

// buildDeviceIDsInArea populates the deviceIDsInArea map with devices in the target area.
func (f *entityRegistryFilter) buildDeviceIDsInArea(ctx context.Context, client homeassistant.Client) {
	if f.areaID == "" {
		return
	}

	devices, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return
	}

	for _, device := range devices {
		if device.AreaID == f.areaID {
			f.deviceIDsInArea[device.ID] = true
		}
	}
}

// filterEntityRegistry applies the filter to a list of entries.
func (f *entityRegistryFilter) filterEntityRegistry(entries []homeassistant.EntityRegistryEntry) []homeassistant.EntityRegistryEntry {
	filtered := make([]homeassistant.EntityRegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if f.matches(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// buildEntityRegistryFiltersMap creates a map of filter values for pagination hash.
func buildEntityRegistryFiltersMap(filter *entityRegistryFilter) map[string]any {
	filters := make(map[string]any)
	if filter.domain != "" {
		filters["domain"] = filter.domain
	}
	if filter.platform != "" {
		filters["platform"] = filter.platform
	}
	if filter.deviceID != "" {
		filters["device_id"] = filter.deviceID
	}
	if filter.areaID != "" {
		filters["area_id"] = filter.areaID
	}
	if filter.includeDisabled {
		filters["include_disabled"] = true
	}
	return filters
}

// paginatedEntityRegistryResponse wraps entity registry output with pagination metadata.
type paginatedEntityRegistryResponse struct {
	Items      json.RawMessage    `json:"items"`
	Pagination PaginationMetadata `json:"pagination"`
}

// buildPaginatedEntityRegistryResponse creates the final response JSON.
func buildPaginatedEntityRegistryResponse(paginated PaginatedResponse[homeassistant.EntityRegistryEntry], itemsOutput string) string {
	// If no pagination was applied (limit=0), return items directly for backwards compatibility
	if paginated.Pagination.Limit == 0 {
		return itemsOutput
	}

	response := paginatedEntityRegistryResponse{
		Items:      json.RawMessage(itemsOutput),
		Pagination: paginated.Pagination,
	}
	result, _ := json.MarshalIndent(response, "", "  ")
	return string(result)
}

// extractDomain extracts the domain from an entity_id (e.g., "light" from "light.living_room").
func extractDomain(entityID string) string {
	for i, c := range entityID {
		if c == '.' {
			return entityID[:i]
		}
	}
	return ""
}

// deviceRegistryFilter encapsulates filter criteria for device registry queries.
type deviceRegistryFilter struct {
	areaID          string
	manufacturer    string
	model           string
	includeDisabled bool
}

// parseDeviceRegistryFilter creates a filter from tool arguments.
func parseDeviceRegistryFilter(args map[string]any) deviceRegistryFilter {
	areaID, _ := args["area_id"].(string)
	manufacturer, _ := args["manufacturer"].(string)
	model, _ := args["model"].(string)
	includeDisabled, _ := args["include_disabled"].(bool)
	return deviceRegistryFilter{
		areaID:          areaID,
		manufacturer:    manufacturer,
		model:           model,
		includeDisabled: includeDisabled,
	}
}

// matches returns true if the entry passes all filter criteria.
func (f deviceRegistryFilter) matches(entry homeassistant.DeviceRegistryEntry) bool {
	if !f.includeDisabled && entry.DisabledBy != "" {
		return false
	}
	if f.areaID != "" && entry.AreaID != f.areaID {
		return false
	}
	if f.manufacturer != "" && !strings.Contains(strings.ToLower(entry.Manufacturer), strings.ToLower(f.manufacturer)) {
		return false
	}
	if f.model != "" && !strings.Contains(strings.ToLower(string(entry.Model)), strings.ToLower(f.model)) {
		return false
	}
	return true
}

// filterDeviceRegistry applies the filter to entries.
func filterDeviceRegistry(entries []homeassistant.DeviceRegistryEntry, f deviceRegistryFilter) []homeassistant.DeviceRegistryEntry {
	filtered := make([]homeassistant.DeviceRegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if f.matches(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// buildDeviceRegistryFiltersMap creates a map of filter values for pagination hash.
func buildDeviceRegistryFiltersMap(filter deviceRegistryFilter) map[string]any {
	filters := make(map[string]any)
	if filter.areaID != "" {
		filters["area_id"] = filter.areaID
	}
	if filter.manufacturer != "" {
		filters["manufacturer"] = filter.manufacturer
	}
	if filter.model != "" {
		filters["model"] = filter.model
	}
	if filter.includeDisabled {
		filters["include_disabled"] = true
	}
	return filters
}

// paginatedDeviceRegistryResponse wraps device registry output with pagination metadata.
type paginatedDeviceRegistryResponse struct {
	Items      json.RawMessage    `json:"items"`
	Pagination PaginationMetadata `json:"pagination"`
}

// buildPaginatedDeviceRegistryResponse creates the final response JSON.
func buildPaginatedDeviceRegistryResponse(paginated PaginatedResponse[homeassistant.DeviceRegistryEntry], itemsOutput []byte) []byte {
	// If no pagination was applied (limit=0), return items directly for backwards compatibility
	if paginated.Pagination.Limit == 0 {
		return itemsOutput
	}

	response := paginatedDeviceRegistryResponse{
		Items:      itemsOutput,
		Pagination: paginated.Pagination,
	}
	result, _ := json.MarshalIndent(response, "", "  ")
	return result
}

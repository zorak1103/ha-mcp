package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// DeviceHealthReport represents the analysis results for device health.
type DeviceHealthReport struct {
	Issues     []DeviceHealthIssue    `json:"issues"`
	Statistics DeviceHealthStatistics `json:"statistics"`
}

// DeviceHealthIssue represents a single health issue detected for a device.
type DeviceHealthIssue struct {
	DeviceID     string `json:"device_id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Details      string `json:"details"`
	Manufacturer string `json:"manufacturer,omitempty"`
}

// DeviceHealthStatistics provides aggregate counts from the health analysis.
type DeviceHealthStatistics struct {
	TotalDevices       int            `json:"total_devices"`
	HealthyDevices     int            `json:"healthy_devices"`
	ProblematicDevices int            `json:"problematic_devices"`
	ByCategory         map[string]int `json:"by_category"`
}

// handleDeviceHealth handles health mode analysis for devices.
func (h *DeviceQueryHandlers) handleDeviceHealth(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	return h.handleDeviceHealthAnalyze(ctx, client, args)
}

func (h *DeviceQueryHandlers) handleDeviceHealthAnalyze(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	// Create snapshot for parallel fetching
	snapshot := CreateAnalysisSnapshot(ctx, client)

	// Fetch config entries (needed for orphaned_config_entry and config_entry_error detection)
	configEntries, _ := client.GetConfigEntries(ctx, "")
	configEntryMap := buildConfigEntryMap(configEntries)

	// Build entity-to-device map (for no_entities detection)
	entityDeviceMap := buildEntityDeviceMap(snapshot.EntityRegistry)

	// Parse filters
	categoryFilter := parseDeviceCategoriesFilter(args)
	manufacturerFilter, _ := args["manufacturer"].(string)

	// Detect issues
	var allIssues []DeviceHealthIssue
	for _, device := range snapshot.DeviceRegistry {
		// Apply manufacturer filter
		if manufacturerFilter != "" && device.Manufacturer != manufacturerFilter {
			continue
		}

		// Detect issues for this device
		issues := detectDeviceIssues(device, configEntryMap, entityDeviceMap, categoryFilter)
		allIssues = append(allIssues, issues...)
	}

	// Build report
	report := DeviceHealthReport{
		Issues: allIssues,
		Statistics: DeviceHealthStatistics{
			TotalDevices:       len(snapshot.DeviceRegistry),
			ProblematicDevices: countUniqueDevices(allIssues),
			ByCategory:         countByCategory(allIssues),
		},
	}
	report.Statistics.HealthyDevices = report.Statistics.TotalDevices - report.Statistics.ProblematicDevices

	// Format output
	format, _ := args["format"].(string)
	if format == "json" {
		return formatDeviceHealthReportJSON(report)
	}
	return formatDeviceHealthReportNatural(report)
}

// removeDeviceConfigEntries removes all config entries for a device.
// Returns (true, "") on success, (false, errorMsg) on failure.
// Kept here as it is reused by manage_device delete action.
func removeDeviceConfigEntries(
	ctx context.Context,
	client homeassistant.Client,
	device homeassistant.DeviceRegistryEntry,
	configEntryMap map[string]homeassistant.ConfigEntryFull,
) (bool, string) {
	for _, ceID := range device.ConfigEntries {
		ce, ceFound := configEntryMap[ceID]
		if !ceFound {
			continue // Config entry already gone
		}

		if !ce.SupportsRemoveDevice {
			return false, fmt.Sprintf("integration %s does not support device removal", ce.Domain)
		}

		if err := client.RemoveDeviceConfigEntry(ctx, device.ID, ceID); err != nil {
			return false, err.Error()
		}
	}
	return true, ""
}

// detectDeviceIssues detects all issues for a single device.
func detectDeviceIssues(
	device homeassistant.DeviceRegistryEntry,
	configEntryMap map[string]homeassistant.ConfigEntryFull,
	entityDeviceMap map[string][]string,
	categoryFilter map[string]bool,
) []DeviceHealthIssue {
	var issues []DeviceHealthIssue

	issues = append(issues, detectDisabledDevice(device, categoryFilter)...)
	issues = append(issues, detectOrphanedConfigEntries(device, configEntryMap, categoryFilter)...)
	issues = append(issues, detectConfigEntryErrors(device, configEntryMap, categoryFilter)...)
	issues = append(issues, detectNoEntities(device, entityDeviceMap, categoryFilter)...)
	issues = append(issues, detectNoConfigEntries(device, categoryFilter)...)

	return issues
}

func detectDisabledDevice(device homeassistant.DeviceRegistryEntry, filter map[string]bool) []DeviceHealthIssue {
	if !shouldDetect(deviceCategoryDisabled, filter) || device.DisabledBy == "" {
		return nil
	}
	return []DeviceHealthIssue{{
		DeviceID:     device.ID,
		Name:         device.Name,
		Category:     deviceCategoryDisabled,
		Details:      fmt.Sprintf("disabled by %s", device.DisabledBy),
		Manufacturer: device.Manufacturer,
	}}
}

func detectOrphanedConfigEntries(
	device homeassistant.DeviceRegistryEntry,
	configEntryMap map[string]homeassistant.ConfigEntryFull,
	filter map[string]bool,
) []DeviceHealthIssue {
	if !shouldDetect(deviceCategoryOrphanedConfigEntry, filter) {
		return nil
	}

	var issues []DeviceHealthIssue
	for _, ceID := range device.ConfigEntries {
		if _, found := configEntryMap[ceID]; !found {
			issues = append(issues, DeviceHealthIssue{
				DeviceID:     device.ID,
				Name:         device.Name,
				Category:     deviceCategoryOrphanedConfigEntry,
				Details:      fmt.Sprintf("references non-existent config entry %s", ceID),
				Manufacturer: device.Manufacturer,
			})
		}
	}
	return issues
}

func detectConfigEntryErrors(
	device homeassistant.DeviceRegistryEntry,
	configEntryMap map[string]homeassistant.ConfigEntryFull,
	filter map[string]bool,
) []DeviceHealthIssue {
	if !shouldDetect(deviceCategoryConfigEntryError, filter) {
		return nil
	}

	var issues []DeviceHealthIssue
	for _, ceID := range device.ConfigEntries {
		if ce, found := configEntryMap[ceID]; found && ce.State != "loaded" {
			issues = append(issues, DeviceHealthIssue{
				DeviceID:     device.ID,
				Name:         device.Name,
				Category:     deviceCategoryConfigEntryError,
				Details:      fmt.Sprintf("config entry %s has state %s", ceID, ce.State),
				Manufacturer: device.Manufacturer,
			})
		}
	}
	return issues
}

func detectNoEntities(
	device homeassistant.DeviceRegistryEntry,
	entityDeviceMap map[string][]string,
	filter map[string]bool,
) []DeviceHealthIssue {
	if !shouldDetect(deviceCategoryNoEntities, filter) || len(entityDeviceMap[device.ID]) > 0 {
		return nil
	}
	return []DeviceHealthIssue{{
		DeviceID:     device.ID,
		Name:         device.Name,
		Category:     deviceCategoryNoEntities,
		Details:      "no entities reference this device",
		Manufacturer: device.Manufacturer,
	}}
}

func detectNoConfigEntries(device homeassistant.DeviceRegistryEntry, filter map[string]bool) []DeviceHealthIssue {
	if !shouldDetect(deviceCategoryNoConfigEntries, filter) || len(device.ConfigEntries) > 0 {
		return nil
	}
	return []DeviceHealthIssue{{
		DeviceID:     device.ID,
		Name:         device.Name,
		Category:     deviceCategoryNoConfigEntries,
		Details:      "device has no config entries",
		Manufacturer: device.Manufacturer,
	}}
}

// Helper functions

func parseDeviceCategoriesFilter(args map[string]any) map[string]bool {
	categoriesRaw, ok := args["categories"]
	if !ok {
		return nil
	}

	categoriesAny, ok := categoriesRaw.([]any)
	if !ok {
		return nil
	}

	categorySet := make(map[string]bool, len(categoriesAny))
	for _, catAny := range categoriesAny {
		if catStr, ok := catAny.(string); ok {
			categorySet[catStr] = true
		}
	}

	if len(categorySet) == 0 {
		return nil
	}
	return categorySet
}

func buildEntityDeviceMap(entities []homeassistant.EntityRegistryEntry) map[string][]string {
	m := make(map[string][]string)
	for _, entity := range entities {
		if entity.DeviceID != "" {
			m[entity.DeviceID] = append(m[entity.DeviceID], entity.EntityID)
		}
	}
	return m
}

func shouldDetect(category string, filter map[string]bool) bool {
	if filter == nil {
		return true // No filter = detect all
	}
	return filter[category]
}

func countUniqueDevices(issues []DeviceHealthIssue) int {
	seen := make(map[string]bool)
	for _, issue := range issues {
		seen[issue.DeviceID] = true
	}
	return len(seen)
}

func countByCategory(issues []DeviceHealthIssue) map[string]int {
	counts := make(map[string]int)
	for _, issue := range issues {
		counts[issue.Category]++
	}
	return counts
}

// Formatting functions

func formatDeviceHealthReportJSON(report DeviceHealthReport) (*mcp.ToolsCallResult, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report: %w", err)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(data))},
	}, nil
}

func formatDeviceHealthReportNatural(report DeviceHealthReport) (*mcp.ToolsCallResult, error) {
	var sb strings.Builder

	sb.WriteString("# Device Health Report\n\n")

	// Summary
	sb.WriteString("## Summary\n")
	fmt.Fprintf(&sb, "- Total devices: %d\n", report.Statistics.TotalDevices)
	fmt.Fprintf(&sb, "- Healthy devices: %d\n", report.Statistics.HealthyDevices)
	fmt.Fprintf(&sb, "- Problematic devices: %d\n\n", report.Statistics.ProblematicDevices)

	if len(report.Issues) == 0 {
		sb.WriteString("No issues detected. All devices are healthy.\n")
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(sb.String())},
		}, nil
	}

	// Issues by category
	sb.WriteString("## Issues\n\n")

	// Group issues by category
	byCategory := make(map[string][]DeviceHealthIssue)
	for _, issue := range report.Issues {
		byCategory[issue.Category] = append(byCategory[issue.Category], issue)
	}

	// Display each category
	categoryNames := map[string]string{
		deviceCategoryDisabled:            "Disabled Devices",
		deviceCategoryOrphanedConfigEntry: "Orphaned Config Entries",
		deviceCategoryConfigEntryError:    "Config Entry Errors",
		deviceCategoryNoEntities:          "No Entities",
		deviceCategoryNoConfigEntries:     "No Config Entries",
	}

	for _, cat := range []string{
		deviceCategoryDisabled,
		deviceCategoryOrphanedConfigEntry,
		deviceCategoryConfigEntryError,
		deviceCategoryNoEntities,
		deviceCategoryNoConfigEntries,
	} {
		issues := byCategory[cat]
		if len(issues) == 0 {
			continue
		}

		fmt.Fprintf(&sb, "### %s (%d)\n", categoryNames[cat], len(issues))
		for _, issue := range issues {
			fmt.Fprintf(&sb, "- **%s** (`%s`)", issue.Name, issue.DeviceID)
			if issue.Manufacturer != "" {
				fmt.Fprintf(&sb, " [%s]", issue.Manufacturer)
			}
			if issue.Details != "" {
				fmt.Fprintf(&sb, ": %s", issue.Details)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(sb.String())},
	}, nil
}

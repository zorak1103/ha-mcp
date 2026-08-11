package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Health categories for entity detection.
const (
	categoryUnavailable         = "unavailable"
	categoryUnknown             = "unknown"
	categoryDisabled            = "disabled"
	categoryOrphanedIntegration = "orphaned_integration"
	categoryOrphanedDevice      = "orphaned_device"
	categoryIntegrationError    = "integration_error"
	categoryRegistryOnly        = "registry_only"
	categoryStale               = "stale"
)

// State constants for health detection.
const (
	stateUnavailable = "unavailable"
	stateUnknown     = "unknown"
)

// Default stale threshold in days.
const defaultStaleDays = 30

// Domains excluded from stale detection (entities that rarely change by design).
var staleExcludedDomains = map[string]bool{
	"automation":              true,
	"script":                  true,
	"scene":                   true,
	"person":                  true,
	"zone":                    true,
	"sun":                     true,
	"persistent_notification": true,
}

// HealthReport represents the analysis results for entity health.
type HealthReport struct {
	Issues     []HealthIssue    `json:"issues"`
	Statistics HealthStatistics `json:"statistics"`
}

// HealthIssue represents a single health issue detected for an entity.
type HealthIssue struct {
	EntityID    string `json:"entity_id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Details     string `json:"details"`
	Platform    string `json:"platform,omitempty"`
	Area        string `json:"area,omitempty"`
	State       string `json:"state,omitempty"`
	LastChanged string `json:"last_changed,omitempty"`
}

// HealthStatistics provides aggregate counts from the health analysis.
type HealthStatistics struct {
	TotalEntities       int            `json:"total_entities"`
	HealthyEntities     int            `json:"healthy_entities"`
	ProblematicEntities int            `json:"problematic_entities"`
	ByCategory          map[string]int `json:"by_category"`
}

// handleHealth handles health mode analysis for entities.
func (h *ConsolidatedEntityQueryHandlers) handleHealth(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	return h.handleHealthAnalyze(ctx, client, args)
}

// handleHealthAnalyze performs health analysis on entities.
func (h *ConsolidatedEntityQueryHandlers) handleHealthAnalyze(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	// Parse parameters
	formatStr := formatter.ParseFormat(getStringArg(args, "format"))
	categories := parseCategoriesFilter(args)
	domain, _ := args["domain"].(string)

	staleDays := defaultStaleDays
	if sd, ok := args["stale_days"].(float64); ok {
		if sd > 0 {
			staleDays = int(sd)
		}
	}

	// Fetch data in parallel
	snapshot := CreateAnalysisSnapshot(ctx, client)

	// Fetch config entries
	configEntries, err := client.GetConfigEntries(ctx, "")
	if err != nil {
		return errorResult(fmt.Sprintf("failed to fetch config entries: %v", err)), nil
	}

	// Build lookup maps
	stateMap := buildStateMap(snapshot.AllStates)
	configEntryMap := buildConfigEntryMap(configEntries)

	// Run detectors
	var allIssues []HealthIssue
	allIssues = append(allIssues, detectUnavailableEntities(snapshot.AllStates)...)
	allIssues = append(allIssues, detectUnknownEntities(snapshot.AllStates)...)
	allIssues = append(allIssues, detectDisabledEntities(snapshot.EntityRegistry)...)
	allIssues = append(allIssues, detectOrphanedIntegration(snapshot.EntityRegistry, configEntryMap)...)
	allIssues = append(allIssues, detectOrphanedDevice(snapshot.EntityRegistry, snapshot)...)
	allIssues = append(allIssues, detectIntegrationErrors(snapshot.EntityRegistry, configEntryMap)...)
	allIssues = append(allIssues, detectRegistryOnlyEntities(snapshot.EntityRegistry, stateMap)...)
	allIssues = append(allIssues, detectStaleEntities(snapshot.AllStates, staleDays)...)

	// Apply filters
	filtered := filterIssues(allIssues, categories, domain)

	// Sort by category, then entity_id
	sortIssues(filtered)

	// Calculate statistics
	stats := calculateStatistics(len(snapshot.EntityRegistry), filtered)

	report := HealthReport{
		Issues:     filtered,
		Statistics: stats,
	}

	// Format output
	if formatStr == formatter.FormatJSON {
		return jsonResult(report)
	}
	return textResult(formatHealthReportNatural(report, staleDays)), nil
}

// Detector functions

func detectUnavailableEntities(states []homeassistant.Entity) []HealthIssue {
	var issues []HealthIssue
	for _, entity := range states {
		if entity.State == stateUnavailable {
			issues = append(issues, HealthIssue{
				EntityID: entity.EntityID,
				Name:     formatter.GetFriendlyName(entity.EntityID, entity.Attributes),
				Category: categoryUnavailable,
				Details:  "entity state is unavailable",
				State:    entity.State,
			})
		}
	}
	return issues
}

func detectUnknownEntities(states []homeassistant.Entity) []HealthIssue {
	var issues []HealthIssue
	for _, entity := range states {
		if entity.State == stateUnknown {
			issues = append(issues, HealthIssue{
				EntityID: entity.EntityID,
				Name:     formatter.GetFriendlyName(entity.EntityID, entity.Attributes),
				Category: categoryUnknown,
				Details:  "entity state is unknown",
				State:    entity.State,
			})
		}
	}
	return issues
}

func detectDisabledEntities(registry []homeassistant.EntityRegistryEntry) []HealthIssue {
	var issues []HealthIssue
	for _, entry := range registry {
		if entry.DisabledBy != "" {
			issues = append(issues, HealthIssue{
				EntityID: entry.EntityID,
				Name:     entry.Name,
				Category: categoryDisabled,
				Details:  fmt.Sprintf("disabled by %s", entry.DisabledBy),
				Platform: entry.Platform,
			})
		}
	}
	return issues
}

func detectOrphanedIntegration(
	registry []homeassistant.EntityRegistryEntry,
	configEntryMap map[string]homeassistant.ConfigEntryFull,
) []HealthIssue {
	var issues []HealthIssue
	for _, entry := range registry {
		if entry.ConfigEntryID != "" {
			if _, exists := configEntryMap[entry.ConfigEntryID]; !exists {
				issues = append(issues, HealthIssue{
					EntityID: entry.EntityID,
					Name:     entry.Name,
					Category: categoryOrphanedIntegration,
					Details:  fmt.Sprintf("config entry %s not found", entry.ConfigEntryID),
					Platform: entry.Platform,
				})
			}
		}
	}
	return issues
}

func detectOrphanedDevice(
	registry []homeassistant.EntityRegistryEntry,
	snapshot *AnalysisSnapshot,
) []HealthIssue {
	var issues []HealthIssue
	for _, entry := range registry {
		if entry.DeviceID != "" {
			if snapshot.FindDeviceByID(entry.DeviceID) == nil {
				issues = append(issues, HealthIssue{
					EntityID: entry.EntityID,
					Name:     entry.Name,
					Category: categoryOrphanedDevice,
					Details:  fmt.Sprintf("device %s not found", entry.DeviceID),
					Platform: entry.Platform,
				})
			}
		}
	}
	return issues
}

func detectIntegrationErrors(
	registry []homeassistant.EntityRegistryEntry,
	configEntryMap map[string]homeassistant.ConfigEntryFull,
) []HealthIssue {
	var issues []HealthIssue
	for _, entry := range registry {
		if entry.ConfigEntryID != "" {
			if configEntry, exists := configEntryMap[entry.ConfigEntryID]; exists {
				if configEntry.State != "loaded" {
					issues = append(issues, HealthIssue{
						EntityID: entry.EntityID,
						Name:     entry.Name,
						Category: categoryIntegrationError,
						Details:  fmt.Sprintf("integration state: %s", configEntry.State),
						Platform: entry.Platform,
					})
				}
			}
		}
	}
	return issues
}

func detectRegistryOnlyEntities(
	registry []homeassistant.EntityRegistryEntry,
	stateMap map[string]homeassistant.Entity,
) []HealthIssue {
	var issues []HealthIssue
	for _, entry := range registry {
		if _, hasState := stateMap[entry.EntityID]; !hasState {
			issues = append(issues, HealthIssue{
				EntityID: entry.EntityID,
				Name:     entry.Name,
				Category: categoryRegistryOnly,
				Details:  "entity in registry but has no state",
				Platform: entry.Platform,
			})
		}
	}
	return issues
}

func detectStaleEntities(states []homeassistant.Entity, staleDays int) []HealthIssue {
	var issues []HealthIssue
	threshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	for _, entity := range states {
		// Skip domains that rarely change by design
		domain := extractDomain(entity.EntityID)
		if staleExcludedDomains[domain] {
			continue
		}

		if entity.LastChanged.Before(threshold) {
			daysSinceChange := int(time.Since(entity.LastChanged).Hours() / 24)
			issues = append(issues, HealthIssue{
				EntityID:    entity.EntityID,
				Name:        formatter.GetFriendlyName(entity.EntityID, entity.Attributes),
				Category:    categoryStale,
				Details:     fmt.Sprintf("last changed %d days ago", daysSinceChange),
				State:       entity.State,
				LastChanged: entity.LastChanged.Format(time.RFC3339),
			})
		}
	}
	return issues
}

// Helper functions

func buildStateMap(states []homeassistant.Entity) map[string]homeassistant.Entity {
	m := make(map[string]homeassistant.Entity, len(states))
	for _, state := range states {
		m[state.EntityID] = state
	}
	return m
}

func buildConfigEntryMap(entries []homeassistant.ConfigEntryFull) map[string]homeassistant.ConfigEntryFull {
	m := make(map[string]homeassistant.ConfigEntryFull, len(entries))
	for _, entry := range entries {
		m[entry.EntryID] = entry
	}
	return m
}

func parseCategoriesFilter(args map[string]any) map[string]bool {
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

func filterIssues(issues []HealthIssue, categories map[string]bool, domain string) []HealthIssue {
	if categories == nil && domain == "" {
		return issues
	}

	filtered := make([]HealthIssue, 0, len(issues))
	for _, issue := range issues {
		if categories != nil && !categories[issue.Category] {
			continue
		}
		if domain != "" {
			entityDomain := extractDomain(issue.EntityID)
			if entityDomain != domain {
				continue
			}
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func sortIssues(issues []HealthIssue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Category != issues[j].Category {
			return issues[i].Category < issues[j].Category
		}
		return issues[i].EntityID < issues[j].EntityID
	})
}

func calculateStatistics(totalEntities int, issues []HealthIssue) HealthStatistics {
	// Count unique entities with issues
	uniqueEntities := make(map[string]bool)
	byCategory := make(map[string]int)

	for _, issue := range issues {
		uniqueEntities[issue.EntityID] = true
		byCategory[issue.Category]++
	}

	problematicCount := len(uniqueEntities)
	healthyCount := totalEntities - problematicCount

	return HealthStatistics{
		TotalEntities:       totalEntities,
		HealthyEntities:     healthyCount,
		ProblematicEntities: problematicCount,
		ByCategory:          byCategory,
	}
}

// Natural language formatters

func formatHealthReportNatural(report HealthReport, staleDays int) string {
	var b strings.Builder

	b.WriteString("Entity Health Report\n\n")

	// Summary
	stats := report.Statistics
	healthyPct := 0.0
	problemsPct := 0.0
	if stats.TotalEntities > 0 {
		healthyPct = float64(stats.HealthyEntities) / float64(stats.TotalEntities) * 100
		problemsPct = float64(stats.ProblematicEntities) / float64(stats.TotalEntities) * 100
	}

	fmt.Fprintf(&b, "Summary: %d entities total, %d healthy (%.1f%%), %d with issues (%.1f%%)\n\n",
		stats.TotalEntities, stats.HealthyEntities, healthyPct,
		stats.ProblematicEntities, problemsPct)

	// No issues
	if len(report.Issues) == 0 {
		b.WriteString("No health issues detected.")
		return b.String()
	}

	// Issues by category
	b.WriteString("Issues by Category:\n")
	categories := make([]string, 0, len(stats.ByCategory))
	for cat := range stats.ByCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		count := stats.ByCategory[cat]
		label := categoryLabel(cat, staleDays)
		fmt.Fprintf(&b, "  %s: %d\n", label, count)
	}
	b.WriteString("\n")

	// Group issues by category
	issuesByCategory := groupIssuesByCategory(report.Issues)
	for _, cat := range categories {
		issues := issuesByCategory[cat]
		if len(issues) == 0 {
			continue
		}

		label := categoryLabel(cat, staleDays)
		fmt.Fprintf(&b, "--- %s (%d) ---\n", label, len(issues))
		for _, issue := range issues {
			fmt.Fprintf(&b, "  %s [%s] - %s\n",
				formatter.FormatNameWithID(issue.Name, issue.EntityID), issue.Platform, issue.Details)
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func categoryLabel(category string, staleDays int) string {
	switch category {
	case categoryUnavailable:
		return "Unavailable"
	case categoryUnknown:
		return "Unknown"
	case categoryDisabled:
		return "Disabled"
	case categoryOrphanedIntegration:
		return "Orphaned Integration"
	case categoryOrphanedDevice:
		return "Orphaned Device"
	case categoryIntegrationError:
		return "Integration Error"
	case categoryRegistryOnly:
		return "Registry Only"
	case categoryStale:
		return fmt.Sprintf("Stale (>%d days unchanged)", staleDays)
	default:
		return category
	}
}

func groupIssuesByCategory(issues []HealthIssue) map[string][]HealthIssue {
	grouped := make(map[string][]HealthIssue)
	for _, issue := range issues {
		grouped[issue.Category] = append(grouped[issue.Category], issue)
	}
	return grouped
}

// Helper functions for result creation

func textResult(text string) *mcp.ToolsCallResult {
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(text)},
	}
}

func jsonResult(v any) (*mcp.ToolsCallResult, error) {
	jsonData, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal JSON: %v", err)), nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(jsonData))},
	}, nil
}

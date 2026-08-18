package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Constants for coverage analysis.
const (
	attrEntityID = "entity_id"
)

// Actionable domains for coverage analysis - domains that typically respond to automations.
var actionableDomains = map[string]bool{
	"light":               true,
	"switch":              true,
	"climate":             true,
	"cover":               true,
	"fan":                 true,
	"media_player":        true,
	"lock":                true,
	"alarm_control_panel": true,
}

// CoverageReport represents automation coverage statistics.
type CoverageReport struct {
	TotalEntities     int                 `json:"total_entities"`
	CoveredEntities   int                 `json:"covered_entities"`
	CoveragePercent   float64             `json:"coverage_percent"`
	AreaCoverage      []AreaCoverageInfo  `json:"area_coverage"`
	UncoveredByDomain map[string][]string `json:"uncovered_by_domain,omitempty"`
}

// AreaCoverageInfo represents coverage stats for a single area.
type AreaCoverageInfo struct {
	AreaID          string  `json:"area_id"`
	AreaName        string  `json:"area_name"`
	TotalEntities   int     `json:"total_entities"`
	CoveredEntities int     `json:"covered_entities"`
	CoveragePercent float64 `json:"coverage_percent"`
}

// handleCoverage analyzes which areas/entities lack automation coverage.
func (h *AutomationHandlers) handleCoverage(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	// Parse format parameter
	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Create snapshot - parallel fetch of all registry data
	snapshot := CreateAnalysisSnapshot(ctx, client)

	// Load all automations
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error loading automations: %v", err)), nil
	}

	// Build set of all referenced entities across all automations
	referencedEntities := make(map[string]bool)
	for _, auto := range automations {
		// Fetch full automation config
		fullAuto, getErr := client.GetAutomation(ctx, auto.EntityID)
		if getErr != nil || fullAuto == nil || fullAuto.Config == nil {
			continue
		}

		// Extract entities from this automation's config
		entities := extractEntitiesFromAutomation(fullAuto.Config)
		for _, entityID := range entities {
			referencedEntities[entityID] = true
		}
	}

	// Filter to actionable entities only
	actionableEntities := make([]homeassistant.Entity, 0)
	for _, entity := range snapshot.AllStates {
		if isActionableDomain(entity.EntityID) {
			actionableEntities = append(actionableEntities, entity)
		}
	}

	// Calculate coverage
	report := h.calculateCoverageReport(snapshot, actionableEntities, referencedEntities)

	// Format output
	var output string
	if format == formatter.FormatJSON {
		data, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return errorResult(fmt.Sprintf("Error formatting coverage: %v", marshalErr)), nil
		}
		output = string(data)
	} else {
		output = h.formatCoverageNatural(report)
	}

	return successResult(output), nil
}

// extractEntitiesFromAutomation extracts all entity IDs referenced in an automation config.
func extractEntitiesFromAutomation(config *homeassistant.AutomationConfig) []string {
	if config == nil {
		return nil
	}

	entities := make(map[string]bool)

	// Search in triggers
	extractEntitiesFromSlice(config.Triggers, entities)

	// Search in conditions
	extractEntitiesFromSlice(config.Conditions, entities)

	// Search in actions
	extractEntitiesFromSlice(config.Actions, entities)

	// Convert set to slice
	result := make([]string, 0, len(entities))
	for entityID := range entities {
		result = append(result, entityID)
	}

	return result
}

// extractEntitiesFromSlice recursively extracts entity IDs from a config slice.
func extractEntitiesFromSlice(items []any, entities map[string]bool) {
	for _, item := range items {
		extractEntitiesFromValue(item, entities)
	}
}

// extractEntitiesFromValue recursively extracts entity IDs from any config value.
func extractEntitiesFromValue(val any, entities map[string]bool) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case string:
		// Check if this looks like an entity ID
		if isEntityID(v) {
			entities[v] = true
		}
	case []any:
		extractEntitiesFromSlice(v, entities)
	case map[string]any:
		extractEntitiesFromMap(v, entities)
	}
}

// extractEntitiesFromMap recursively extracts entity IDs from a map value.
func extractEntitiesFromMap(m map[string]any, entities map[string]bool) {
	for key, subval := range m {
		// Check if this is an entity_id key
		if key == attrEntityID {
			switch entityVal := subval.(type) {
			case string:
				if isEntityID(entityVal) {
					entities[entityVal] = true
				}
			case []any:
				for _, item := range entityVal {
					if str, ok := item.(string); ok && isEntityID(str) {
						entities[str] = true
					}
				}
			}
		} else {
			// Recursively search other keys
			extractEntitiesFromValue(subval, entities)
		}
	}
}

// isEntityID checks if a string looks like a Home Assistant entity ID.
func isEntityID(s string) bool {
	return strings.Contains(s, ".") && !strings.HasPrefix(s, ".")
}

// isActionableDomain checks if an entity is in an actionable domain.
func isActionableDomain(entityID string) bool {
	parts := strings.SplitN(entityID, ".", 2)
	if len(parts) != 2 {
		return false
	}
	return actionableDomains[parts[0]]
}

// calculateCoverageReport builds the coverage report from entities and coverage data.
func (h *AutomationHandlers) calculateCoverageReport(
	snapshot *AnalysisSnapshot,
	actionableEntities []homeassistant.Entity,
	referencedEntities map[string]bool,
) CoverageReport {
	// Group entities by area
	areaEntities := groupCoverageEntitiesByArea(snapshot, actionableEntities)

	// Calculate per-area coverage
	areaStats := buildCoverageAreaStats(snapshot, areaEntities, referencedEntities)

	// Calculate overall coverage
	totalEntities := len(actionableEntities)
	coveredCount := countCoverageEntities(actionableEntities, referencedEntities)
	overallPercent := calculateCoveragePercent(coveredCount, totalEntities)

	// Build uncovered by domain
	uncoveredByDomain := buildCoverageUncoveredByDomain(actionableEntities, referencedEntities)

	return CoverageReport{
		TotalEntities:     totalEntities,
		CoveredEntities:   coveredCount,
		CoveragePercent:   overallPercent,
		AreaCoverage:      areaStats,
		UncoveredByDomain: uncoveredByDomain,
	}
}

// groupCoverageEntitiesByArea groups entities by their area ID for coverage analysis.
func groupCoverageEntitiesByArea(
	snapshot *AnalysisSnapshot,
	entities []homeassistant.Entity,
) map[string][]homeassistant.Entity {
	areaEntities := make(map[string][]homeassistant.Entity)
	for _, entity := range entities {
		areaID := snapshot.GetEntityArea(entity.EntityID)
		if areaID == "" {
			areaID = "no_area"
		}
		areaEntities[areaID] = append(areaEntities[areaID], entity)
	}
	return areaEntities
}

// buildCoverageAreaStats builds coverage statistics for each area.
func buildCoverageAreaStats(
	snapshot *AnalysisSnapshot,
	areaEntities map[string][]homeassistant.Entity,
	referencedEntities map[string]bool,
) []AreaCoverageInfo {
	areaStats := make([]AreaCoverageInfo, 0, len(areaEntities))
	for areaID, entities := range areaEntities {
		covered := countCoverageEntities(entities, referencedEntities)
		areaName := findCoverageAreaName(snapshot, areaID)
		percent := calculateCoveragePercent(covered, len(entities))

		areaStats = append(areaStats, AreaCoverageInfo{
			AreaID:          areaID,
			AreaName:        areaName,
			TotalEntities:   len(entities),
			CoveredEntities: covered,
			CoveragePercent: percent,
		})
	}

	// Sort by area name
	slices.SortFunc(areaStats, func(a, b AreaCoverageInfo) int {
		return cmp.Compare(a.AreaName, b.AreaName)
	})

	return areaStats
}

// findCoverageAreaName finds the human-readable name for an area ID.
func findCoverageAreaName(snapshot *AnalysisSnapshot, areaID string) string {
	for _, area := range snapshot.AreaRegistry {
		if area.AreaID == areaID {
			return area.Name
		}
	}
	return areaID
}

// countCoverageEntities counts how many entities are referenced in automations.
func countCoverageEntities(
	entities []homeassistant.Entity,
	referencedEntities map[string]bool,
) int {
	covered := 0
	for _, entity := range entities {
		if referencedEntities[entity.EntityID] {
			covered++
		}
	}
	return covered
}

// calculateCoveragePercent calculates coverage percentage with division-by-zero protection.
func calculateCoveragePercent(covered, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(covered) / float64(total) * 100
}

// buildCoverageUncoveredByDomain groups uncovered entities by domain.
func buildCoverageUncoveredByDomain(
	entities []homeassistant.Entity,
	referencedEntities map[string]bool,
) map[string][]string {
	uncoveredByDomain := make(map[string][]string)
	for _, entity := range entities {
		if !referencedEntities[entity.EntityID] {
			parts := strings.SplitN(entity.EntityID, ".", 2)
			if len(parts) == 2 {
				domain := parts[0]
				uncoveredByDomain[domain] = append(uncoveredByDomain[domain], entity.EntityID)
			}
		}
	}

	// Sort uncovered entities
	for domain := range uncoveredByDomain {
		sort.Strings(uncoveredByDomain[domain])
	}

	return uncoveredByDomain
}

// formatCoverageNatural formats coverage report in natural language.
func (h *AutomationHandlers) formatCoverageNatural(report CoverageReport) string {
	var result strings.Builder

	// Overall summary
	fmt.Fprintf(&result, "Automation Coverage Analysis\n\n")
	fmt.Fprintf(&result, "Overall Coverage: %d/%d entities (%.1f%%)\n\n",
		report.CoveredEntities, report.TotalEntities, report.CoveragePercent)

	// Per-area breakdown
	if len(report.AreaCoverage) > 0 {
		result.WriteString("Coverage by Area:\n")
		for _, area := range report.AreaCoverage {
			fmt.Fprintf(&result, "  %s: %d/%d (%.1f%%)\n",
				area.AreaName, area.CoveredEntities, area.TotalEntities, area.CoveragePercent)
		}
		result.WriteString("\n")
	}

	// Uncovered entities by domain
	if len(report.UncoveredByDomain) > 0 {
		result.WriteString("Uncovered Entities by Domain:\n")

		domains := make([]string, 0, len(report.UncoveredByDomain))
		for domain := range report.UncoveredByDomain {
			domains = append(domains, domain)
		}
		sort.Strings(domains)

		for _, domain := range domains {
			entities := report.UncoveredByDomain[domain]
			fmt.Fprintf(&result, "  %s (%d): %s\n",
				domain, len(entities), strings.Join(entities, ", "))
		}
	}

	return strings.TrimSuffix(result.String(), "\n")
}

package formatter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Domain constants
const (
	domainLight = "light"
)

// compactListCap is the maximum number of entities shown in CompactList mode before
// an overflow note is appended. Keeps non-verbose output token-bounded (~3k chars max).
const compactListCap = 50

// State constants
const (
	stateUnknown = "unknown"
	stateIsOff   = "is off"
)

// NaturalFormatter produces human-readable, LLM-optimized output.
type NaturalFormatter struct {
	now time.Time
}

// NewNaturalFormatter creates a new NaturalFormatter.
func NewNaturalFormatter() *NaturalFormatter {
	return &NaturalFormatter{
		now: time.Now(),
	}
}

// WithNow returns a copy of the formatter with a specific time for testing.
func (f *NaturalFormatter) WithNow(now time.Time) *NaturalFormatter {
	return &NaturalFormatter{now: now}
}

// FormatEntity formats a single entity in natural language.
func (f *NaturalFormatter) FormatEntity(_ context.Context, entity homeassistant.Entity) (string, error) {
	return f.formatEntityNL(entity, true), nil
}

// FormatEntities formats a list of entities in natural language.
func (f *NaturalFormatter) FormatEntities(_ context.Context, entities []homeassistant.Entity, opts EntityListOptions) (string, error) {
	if len(entities) == 0 {
		return "No entities found.", nil
	}

	var parts []string

	// Summary statistics
	if opts.IncludeSummary || !opts.Verbose {
		parts = append(parts, f.buildEntitiesSummary(entities))
	}

	// Group by domain or list entities
	switch {
	case opts.GroupByDomain:
		parts = append(parts, f.formatEntitiesByDomain(entities))
	case opts.Verbose:
		// List all entities with full detail
		for _, e := range entities {
			parts = append(parts, "- "+f.formatEntityNL(e, false))
		}
	case opts.CompactList:
		// Compact per-entity list capped at compactListCap — makes non-verbose output actionable
		capped := entities
		if len(capped) > compactListCap {
			capped = capped[:compactListCap]
		}
		lines := make([]string, 0, len(capped)+1)
		for _, e := range capped {
			lines = append(lines, "- "+f.formatEntityNL(e, false))
		}
		if len(entities) > compactListCap {
			lines = append(lines, fmt.Sprintf("... and %d more (use pagination or verbose=true for full list)", len(entities)-compactListCap))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	return strings.Join(parts, "\n\n"), nil
}

// FormatHistory formats history entries in natural language.
// entries is a flat list of HistoryEntry (already processed from [][]HistoryEntry).
// Always shows entries with absolute timestamps; verbose adds attribute details.
func (f *NaturalFormatter) FormatHistory(_ context.Context, entityID string, entries []homeassistant.HistoryEntry, opts HistoryOptions) (string, error) {
	if len(entries) == 0 {
		// Differentiate between "entity not found" vs "entity exists but no history"
		if !opts.EntityExists {
			return fmt.Sprintf(`Entity %s was not found in Home Assistant.

Please verify:
- The entity_id is spelled correctly
- The entity exists in your Home Assistant instance
- You have access permissions to view this entity`, entityID), nil
		}
		return fmt.Sprintf(`No history found for %s.

Possible reasons:
- Entity may be excluded from recorder configuration
- Entity may have been newly created
- No state changes occurred in the requested period

Tips:
- Check recorder configuration to ensure entity is not excluded
- For new entities, history may not be available yet
- Try increasing the time range if using filters`, entityID), nil
	}

	var parts []string
	friendlyName := entityID

	// Get friendly name from first entry with attributes
	for _, entry := range entries {
		if name := GetStringAttr(entry.Attributes, "friendly_name"); name != "" {
			friendlyName = name
			break
		}
	}

	parts = append(parts, fmt.Sprintf("History for %s: %d state changes.", friendlyName, len(entries)))

	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	showCount := min(limit, len(entries))
	for i := range showCount {
		entry := entries[i]
		ts := entry.LastChangedTime().Format("2006-01-02 15:04")
		if opts.Verbose {
			if attrs := formatHistoryAttributes(entry.Attributes); attrs != "" {
				parts = append(parts, fmt.Sprintf("%s → %s (%s)", ts, entry.State, attrs))
			} else {
				parts = append(parts, fmt.Sprintf("%s → %s", ts, entry.State))
			}
		} else {
			parts = append(parts, fmt.Sprintf("%s → %s", ts, entry.State))
		}
	}
	if len(entries) > showCount {
		parts = append(parts, fmt.Sprintf("…and %d more", len(entries)-showCount))
	}

	return strings.Join(parts, "\n"), nil
}

// formatHistoryAttributes formats relevant attributes for verbose history output.
// Skips noisy keys (friendly_name, entity_id). Returns at most 5 key=value pairs.
func formatHistoryAttributes(attrs map[string]any) string {
	if len(attrs) == 0 {
		return ""
	}
	skipKeys := map[string]bool{"friendly_name": true, "entity_id": true}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		if !skipKeys[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	const maxAttrs = 5
	if len(keys) > maxAttrs {
		keys = keys[:maxAttrs]
	}
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, attrs[k]))
	}
	return strings.Join(pairs, ", ")
}

// FormatServiceSuccess formats a successful service call response.
func (f *NaturalFormatter) FormatServiceSuccess(_ context.Context, domain, service string, targets []string, _ map[string]any) (string, error) {
	action := formatServiceAction(domain, service)

	if len(targets) == 0 {
		return fmt.Sprintf("OK %s.", action), nil
	}

	if len(targets) == 1 {
		return fmt.Sprintf("OK %s %s.", action, targets[0]), nil
	}

	return fmt.Sprintf("OK %s %d entities.", action, len(targets)), nil
}

// FormatError formats an error response.
func (f *NaturalFormatter) FormatError(_ context.Context, err error) string {
	return fmt.Sprintf("Error: %s", err.Error())
}

// formatEntityNL formats a single entity with domain-specific details.
func (f *NaturalFormatter) formatEntityNL(entity homeassistant.Entity, includeTime bool) string {
	name := GetFriendlyName(entity.EntityID, entity.Attributes)
	domain := ExtractDomain(entity.EntityID)
	state := entity.State

	var details string
	switch domain {
	case domainLight:
		details = f.formatLightDetails(entity)
	case "climate":
		details = f.formatClimateDetails(entity)
	case "sensor":
		details = f.formatSensorDetails(entity)
	case "binary_sensor":
		details = f.formatBinarySensorDetails(entity)
	case "switch", "input_boolean":
		details = f.formatSwitchDetails(entity)
	case "cover":
		details = f.formatCoverDetails(entity)
	case "media_player":
		details = f.formatMediaPlayerDetails(entity)
	case "update":
		details = f.formatUpdateDetails(entity)
	default:
		details = fmt.Sprintf("is %s", state)
	}

	result := fmt.Sprintf("%s (%s) %s", name, entity.EntityID, details)

	if includeTime && !entity.LastChanged.IsZero() {
		timeSince := FormatTimeSince(entity.LastChanged, f.now)
		if timeSince != stateUnknown {
			result += fmt.Sprintf(". Changed %s", timeSince)
		}
	}

	return result
}

// formatLightDetails formats light-specific attributes.
func (f *NaturalFormatter) formatLightDetails(entity homeassistant.Entity) string {
	if entity.State != "on" {
		return stateIsOff
	}

	var parts []string
	parts = append(parts, "is on")

	// Brightness
	if brightness := GetIntAttr(entity.Attributes, "brightness"); brightness > 0 {
		percent := BrightnessToPercent(brightness)
		parts = append(parts, fmt.Sprintf("at %d%% brightness", percent))
	}

	// Color temperature
	if colorTemp := GetIntAttr(entity.Attributes, "color_temp_kelvin"); colorTemp > 0 {
		desc := ColorTempToDescription(colorTemp)
		if desc != "" {
			parts = append(parts, fmt.Sprintf("(%s)", desc))
		}
	}

	return strings.Join(parts, " ")
}

// formatClimateDetails formats climate-specific attributes.
func (f *NaturalFormatter) formatClimateDetails(entity homeassistant.Entity) string {
	state := entity.State
	if state == stateOff {
		return stateIsOff
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("is %s", state))

	// Current temperature
	if currentTemp := GetFloatAttr(entity.Attributes, "current_temperature"); currentTemp > 0 {
		parts = append(parts, fmt.Sprintf("at %.1f°", currentTemp))
	}

	// Target temperature
	if targetTemp := GetFloatAttr(entity.Attributes, "temperature"); targetTemp > 0 {
		parts = append(parts, fmt.Sprintf("(target: %.1f°)", targetTemp))
	}

	return strings.Join(parts, " ")
}

// formatSensorDetails formats sensor-specific attributes.
func (f *NaturalFormatter) formatSensorDetails(entity homeassistant.Entity) string {
	state := entity.State
	unit := GetStringAttr(entity.Attributes, "unit_of_measurement")

	if unit != "" {
		return fmt.Sprintf("is %s %s", state, unit)
	}
	return fmt.Sprintf("is %s", state)
}

// formatBinarySensorDetails formats binary_sensor-specific attributes.
func (f *NaturalFormatter) formatBinarySensorDetails(entity homeassistant.Entity) string {
	state := entity.State
	deviceClass := GetStringAttr(entity.Attributes, "device_class")

	// Map device class to human-readable state
	switch deviceClass {
	case "motion":
		if state == "on" {
			return "detected motion"
		}
		return "no motion"
	case "door":
		if state == "on" {
			return "is open"
		}
		return "is closed"
	case "window":
		if state == "on" {
			return "is open"
		}
		return "is closed"
	case "occupancy":
		if state == "on" {
			return "is occupied"
		}
		return "is not occupied"
	case "presence":
		if state == "on" {
			return "is home"
		}
		return "is away"
	case "connectivity":
		if state == "on" {
			return "is connected"
		}
		return "is disconnected"
	default:
		return fmt.Sprintf("is %s", state)
	}
}

// formatSwitchDetails formats switch-specific attributes.
func (f *NaturalFormatter) formatSwitchDetails(entity homeassistant.Entity) string {
	if entity.State == "on" {
		return "is on"
	}
	return stateIsOff
}

// formatCoverDetails formats cover-specific attributes.
func (f *NaturalFormatter) formatCoverDetails(entity homeassistant.Entity) string {
	state := entity.State
	position := GetIntAttr(entity.Attributes, "current_position")

	if position > 0 && position < 100 {
		return fmt.Sprintf("is %s (%d%% open)", state, position)
	}
	return fmt.Sprintf("is %s", state)
}

// formatMediaPlayerDetails formats media_player-specific attributes.
func (f *NaturalFormatter) formatMediaPlayerDetails(entity homeassistant.Entity) string {
	state := entity.State
	if state == stateOff || state == "standby" {
		return stateIsOff
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("is %s", state))

	// Media title
	if title := GetStringAttr(entity.Attributes, "media_title"); title != "" {
		artist := GetStringAttr(entity.Attributes, "media_artist")
		if artist != "" {
			parts = append(parts, fmt.Sprintf("- %s by %s", title, artist))
		} else {
			parts = append(parts, fmt.Sprintf("- %s", title))
		}
	}

	return strings.Join(parts, " ")
}

// formatUpdateDetails formats update-specific attributes.
func (f *NaturalFormatter) formatUpdateDetails(entity homeassistant.Entity) string {
	inProgress, _ := entity.Attributes["in_progress"].(bool)
	if inProgress {
		return "Installing update"
	}

	installedVersion := GetStringAttr(entity.Attributes, "installed_version")
	latestVersion := GetStringAttr(entity.Attributes, "latest_version")

	if entity.State == "on" && installedVersion != "" && latestVersion != "" {
		return fmt.Sprintf("Update available (%s -> %s)", installedVersion, latestVersion)
	}

	if entity.State == stateOff && installedVersion != "" {
		return fmt.Sprintf("Up to date (%s)", installedVersion)
	}

	return fmt.Sprintf("is %s", entity.State)
}

// buildEntitiesSummary builds a summary of entities by domain.
func (f *NaturalFormatter) buildEntitiesSummary(entities []homeassistant.Entity) string {
	total := len(entities)

	// Count by domain
	domains := make(map[string]int)
	unavailable := 0
	for _, e := range entities {
		domain := ExtractDomain(e.EntityID)
		domains[domain]++
		if e.State == "unavailable" || e.State == stateUnknown {
			unavailable++
		}
	}

	// Build summary
	var parts []string
	parts = append(parts, fmt.Sprintf("%d entities total.", total))

	// Domain counts (sorted)
	var domainParts []string
	sortedDomains := make([]string, 0, len(domains))
	for d := range domains {
		sortedDomains = append(sortedDomains, d)
	}
	sort.Strings(sortedDomains)

	for _, domain := range sortedDomains {
		count := domains[domain]
		domainParts = append(domainParts, fmt.Sprintf("%s: %d", domain, count))
	}
	if len(domainParts) > 0 {
		parts = append(parts, strings.Join(domainParts, ", "))
	}

	// Unavailable warning
	if unavailable > 0 {
		parts = append(parts, fmt.Sprintf("Warning: %d entities unavailable.", unavailable))
	}

	return strings.Join(parts, " ")
}

// formatEntitiesByDomain groups entities by domain and formats them.
func (f *NaturalFormatter) formatEntitiesByDomain(entities []homeassistant.Entity) string {
	// Group by domain
	byDomain := make(map[string][]homeassistant.Entity)
	for _, e := range entities {
		domain := ExtractDomain(e.EntityID)
		byDomain[domain] = append(byDomain[domain], e)
	}

	// Sort domains
	sortedDomains := make([]string, 0, len(byDomain))
	for d := range byDomain {
		sortedDomains = append(sortedDomains, d)
	}
	sort.Strings(sortedDomains)

	var parts []string
	for _, domain := range sortedDomains {
		domainEntities := byDomain[domain]
		parts = append(parts, f.formatDomainGroup(domain, domainEntities))
	}

	return strings.Join(parts, "\n\n")
}

// formatDomainGroup formats a group of entities from the same domain.
func (f *NaturalFormatter) formatDomainGroup(domain string, entities []homeassistant.Entity) string {
	var parts []string

	// Domain header with count
	header := fmt.Sprintf("**%s** (%d)", domain, len(entities))

	// For lights/switches, show on/off count
	if domain == domainLight || domain == "switch" || domain == "input_boolean" {
		onCount := 0
		for _, e := range entities {
			if e.State == "on" {
				onCount++
			}
		}
		offCount := len(entities) - onCount
		header = fmt.Sprintf("**%s** (%d on, %d off)", domain, onCount, offCount)
	}

	parts = append(parts, header)

	// Always show entities when domain grouping is used (explicit grouping)
	// Use compact format if not verbose
	for _, e := range entities {
		parts = append(parts, "- "+f.formatEntityNL(e, false))
	}

	return strings.Join(parts, "\n")
}

// formatServiceAction returns a human-readable action description.
func formatServiceAction(domain, service string) string {
	// Common patterns
	switch service {
	case "turn_on":
		return "Turned on"
	case "turn_off":
		return "Turned off"
	case "toggle":
		return "Toggled"
	case "reload":
		return "Reloaded"
	}

	// Domain-specific
	switch domain {
	case domainLight:
		return fmt.Sprintf("Set light %s", service)
	case "climate":
		return fmt.Sprintf("Set climate %s", service)
	default:
		return fmt.Sprintf("Called %s.%s", domain, service)
	}
}

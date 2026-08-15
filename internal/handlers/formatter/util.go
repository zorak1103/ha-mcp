package formatter

import (
	"strconv"
	"strings"
	"time"
)

// FormatTimeSince returns a human-readable relative time string.
// Examples: "just now", "5 min ago", "2 hours ago", "1 day ago"
func FormatTimeSince(t, now time.Time) string {
	if t.IsZero() {
		return stateUnknown
	}

	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}

	if diff < time.Hour {
		mins := int(diff.Minutes())
		return pluralize(mins, "min") + " ago"
	}

	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return pluralize(hours, "hour") + " ago"
	}

	days := int(diff.Hours() / 24)
	return pluralize(days, "day") + " ago"
}

// pluralize returns singular or plural form based on count.
func pluralize(count int, singular string) string {
	if count == 1 {
		return "1 " + singular
	}
	if singular == "min" || singular == "day" {
		return strconv.Itoa(count) + " " + singular + "s"
	}
	return strconv.Itoa(count) + " " + singular + "s"
}

// ExtractDomain extracts the domain from an entity_id.
// Example: "light.living_room" -> "light"
func ExtractDomain(entityID string) string {
	if entityID == "" {
		return stateUnknown
	}
	idx := strings.Index(entityID, ".")
	if idx == -1 {
		return stateUnknown
	}
	return entityID[:idx]
}

// GetFriendlyName returns the friendly_name attribute or falls back to entity_id.
// For update entities, it also checks the title attribute.
func GetFriendlyName(entityID string, attributes map[string]any) string {
	if attributes == nil {
		return entityID
	}
	if name, ok := attributes["friendly_name"].(string); ok && name != "" {
		return name
	}
	// Update entities use "title" instead of "friendly_name"
	if title, ok := attributes["title"].(string); ok && title != "" {
		return title
	}
	return entityID
}

// FormatNameWithID formats an entity's display name alongside its entity_id in the
// canonical "Name (entity_id)" shape used throughout natural-language output, so a
// caller can always recover the addressable id positionally. If name and
// entityID are identical (no distinct friendly name), the id is not duplicated in
// parentheses.
//
// name is user-controlled (Home Assistant friendly_name/title, editable via the UI or
// customize.yaml) and is sanitized before interpolation: newlines/carriage returns are
// collapsed to spaces so a single entity's name cannot inject additional lines into a
// rendered list, and parentheses are stripped so a name cannot forge a fake
// "(entity_id)" suffix that would displace the real one.
func FormatNameWithID(name, entityID string) string {
	sanitized := sanitizeDisplayName(name)
	if sanitized == entityID {
		return entityID
	}
	return sanitized + " (" + entityID + ")"
}

// sanitizeDisplayName strips characters from a user-controlled display name that could
// be used to forge or break the "Name (entity_id)" line shape.
func sanitizeDisplayName(name string) string {
	replacer := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "(", "", ")", "")
	return replacer.Replace(name)
}

// ColorTempToDescription converts color temperature in Kelvin to a description.
func ColorTempToDescription(kelvin int) string {
	if kelvin <= 0 {
		return ""
	}
	switch {
	case kelvin <= 3000:
		return "warm white"
	case kelvin <= 5000:
		return "neutral"
	case kelvin <= 6000:
		return "cool white"
	default:
		return "daylight"
	}
}

// BrightnessToPercent converts 0-255 brightness to 0-100 percent.
func BrightnessToPercent(brightness int) int {
	if brightness <= 0 {
		return 0
	}
	if brightness >= 255 {
		return 100
	}
	return (brightness * 100) / 255
}

// GetStringAttr safely extracts a string attribute.
func GetStringAttr(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if v, ok := attrs[key].(string); ok {
		return v
	}
	return ""
}

// GetIntAttr safely extracts an integer attribute (handles float64 from JSON).
func GetIntAttr(attrs map[string]any, key string) int {
	if attrs == nil {
		return 0
	}
	switch v := attrs[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

// GetFloatAttr safely extracts a float64 attribute.
func GetFloatAttr(attrs map[string]any, key string) float64 {
	if attrs == nil {
		return 0
	}
	switch v := attrs[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

package formatter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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

// titleCaseWords splits s on sep and upper-cases the first rune of each
// non-empty segment, rejoining with a single space. Used to render a helper
// domain ("alarm_control_panel") or attribute key as a natural-language
// label ("Alarm Control Panel"). Rune-safe: uses utf8.DecodeRuneInString
// rather than byte-slicing (s[:1]), which corrupts a multi-byte first rune.
func titleCaseWords(s, sep string) string {
	words := strings.Split(s, sep)
	for i, w := range words {
		if w == "" {
			continue
		}
		r, size := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[size:]
	}
	return strings.Join(words, " ")
}

// sentenceCaseKey renders an attribute key like "current_temperature" as
// "Current temperature": underscores become spaces and only the first rune
// of the whole result is upper-cased, matching the sentence-case style
// already used for hand-written detail labels elsewhere in this package
// (e.g. "Device class", "Unit of measurement") - deliberately not the
// per-word title case titleCaseWords applies to a helper *type* header.
// Rune-safe: uses utf8.DecodeRuneInString rather than byte-slicing
// (formatted[:1]), which corrupts a multi-byte first rune.
// sentenceCaseKey renders an attribute key like "current_temperature" as
// "Current temperature": underscores become spaces and only the first rune
// of the whole result is upper-cased, matching the sentence-case style
// already used for hand-written detail labels elsewhere in this package
// (e.g. "Device class", "Unit of measurement") - deliberately not the
// per-word title case titleCaseWords applies to a helper *type* header.
// Rune-safe: uses utf8.DecodeRuneInString rather than byte-slicing
// (formatted[:1]), which corrupts a multi-byte first rune.
// Sanitized via sanitizeDisplayValue for the same reason attribute values
// are: formatGenericDetail emits the result directly into a line-oriented
// "%s: %s\n" line, so an unsanitized key could forge an extra line the same
// way an unsanitized value could (second review round finding).
func sentenceCaseKey(key string) string {
	formatted := sanitizeDisplayValue(strings.ReplaceAll(key, "_", " "))
	if formatted == "" {
		return formatted
	}
	r, size := utf8.DecodeRuneInString(formatted)
	return string(unicode.ToUpper(r)) + formatted[size:]
}

// maxDetailValueChars/maxDetailListItems/maxDetailValueDepth bound the
// generic attribute-value renderer (formatDetailValue) used by get_details'
// fallback path for helper domains with no dedicated renderer (climate,
// humidifier, select, the 15 template_* subtypes). 400 chars keeps
// legitimately useful full-length values (climate.hvac_modes, select.options,
// light.effect_list) intact while still bounding pathological ones
// (update.release_summary, a large weather.forecast rendered as one line).
// maxDetailValueDepth bounds recursion into nested list/map values.
const (
	maxDetailValueChars = 400
	maxDetailListItems  = 20
	maxDetailValueDepth = 4
)

// sanitizeDisplayValue collapses newlines/carriage returns in a natural-format
// attribute value to spaces, so a value cannot forge additional "Key: value"
// lines in line-oriented output. Unlike sanitizeDisplayName (used for entity
// display names), parentheses are left intact - "(eco)" is a legitimate
// attribute value, not an attempt to forge an "(entity_id)" suffix.
func sanitizeDisplayValue(s string) string {
	replacer := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
	return replacer.Replace(s)
}

// truncateRunes truncates s to at most maxRunes runes, appending "..." if
// truncated. Rune-safe: counts and slices by rune, not by byte, so a
// multi-byte rune is never split.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// renderDetailValue renders v for natural-format display. Only the item-count
// cap (maxDetailListItems) is applied here, for nested lists at depth > 0;
// the character-length cap is deliberately deferred to formatDetailValue's
// top-level handling, since applying it here too would double-truncate:
// every list element would be pre-truncated before joining, and a long
// list's "… +N more" suffix could then itself be truncated away.
func renderDetailValue(v any, depth int) string {
	if depth >= maxDetailValueDepth {
		return fmt.Sprintf("%v", v)
	}
	switch val := v.(type) {
	case string:
		return val
	case []any:
		shown, more := capDetailList(val)
		parts := make([]string, 0, len(shown))
		for _, item := range shown {
			parts = append(parts, renderDetailValue(item, depth+1))
		}
		joined := strings.Join(parts, ", ")
		if more > 0 {
			joined += fmt.Sprintf(", … +%d more", more)
		}
		return joined
	case map[string]any:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// capDetailList caps list to maxDetailListItems, returning the shown prefix
// and the count of omitted trailing items.
func capDetailList(list []any) (shown []any, more int) {
	// `<=` vs `<` here is a proven-equivalent mutant (same reasoning as
	// BoundedFieldList in internal/homeassistant/field_list.go): at
	// len(list) == maxDetailListItems exactly, list[:maxDetailListItems] is
	// a no-op slice of an already-that-length slice (same ptr/len/cap) and
	// more computes to 0 either way, so which branch runs makes no
	// observable difference. No test can kill it - both forms produce
	// byte-identical output for every input.
	if len(list) <= maxDetailListItems { //mutest:skip
		return list, 0
	}
	return list[:maxDetailListItems], len(list) - maxDetailListItems
}

// formatDetailListValue renders a top-level list attribute for natural
// format: the item-count cap's "… +N more" suffix must survive even when the
// joined shown items alone exceed maxDetailValueChars, so it is appended
// AFTER truncating only the joined items - never truncated away itself.
func formatDetailListValue(list []any) string {
	shown, more := capDetailList(list)
	parts := make([]string, 0, len(shown))
	for _, item := range shown {
		parts = append(parts, renderDetailValue(item, 1))
	}
	joined := truncateRunes(sanitizeDisplayValue(strings.Join(parts, ", ")), maxDetailValueChars)
	if more > 0 {
		joined += fmt.Sprintf(", … +%d more", more)
	}
	return joined
}

// formatDetailValue renders an attribute value of unknown shape for natural
// format: nil renders as "" (callers skip the whole line when this is
// empty), everything else is rendered via renderDetailValue then sanitized
// and truncated once at the top level.
func formatDetailValue(v any) string {
	if v == nil {
		return ""
	}
	// See formatDetailListValue's doc comment for why the "… +N more" suffix
	// must be appended after truncation, not before.
	if list, ok := v.([]any); ok {
		return formatDetailListValue(list)
	}
	return truncateRunes(sanitizeDisplayValue(renderDetailValue(v, 0)), maxDetailValueChars)
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

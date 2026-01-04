// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import "strings"

// HelperPlatforms defines all known helper platform prefixes.
// This list is used across helper handlers for entity ID parsing and validation.
var HelperPlatforms = []string{
	// Input helpers
	"input_boolean",
	"input_number",
	"input_text",
	"input_select",
	"input_datetime",
	"input_button",
	// Other helpers
	"counter",
	"timer",
	"schedule",
	"group",
	"template",
	"threshold",
	"derivative",
	"integration", // Note: Home Assistant uses "integration" not "integral"
}

// ParseHelperEntityID extracts platform and ID from an entity_id like "input_boolean.my_switch".
// It iterates through known helper platforms to find a matching prefix.
// Returns empty strings if the entity_id doesn't match any known helper platform.
func ParseHelperEntityID(entityID string) (platform, id string) {
	for _, p := range HelperPlatforms {
		prefix := p + "."
		if strings.HasPrefix(entityID, prefix) {
			return p, strings.TrimPrefix(entityID, prefix)
		}
	}
	return "", ""
}

// IsValidHelperPlatform checks if the given platform is a valid helper platform.
func IsValidHelperPlatform(platform string) bool {
	for _, p := range HelperPlatforms {
		if p == platform {
			return true
		}
	}
	return false
}

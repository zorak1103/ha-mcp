// Package homeassistant provides helper detection for Config Entry Flow platforms.
package homeassistant

// ConfigEntryPlatforms defines helper platforms that require HTTP Config Entry Flow.
// These platforms cannot be created/deleted via WebSocket API and must use the
// HTTP-based Config Entry Flow mechanism instead.
var ConfigEntryPlatforms = map[string]bool{
	"threshold":   true, // Creates binary_sensor entities
	"derivative":  true, // Creates sensor entities
	"integration": true, // Creates sensor entities (Home Assistant's name for integral)
	"group":       true, // Creates group.* entities
	"template":    true, // Creates sensor or binary_sensor entities
}

// RequiresConfigEntryFlow checks if the given platform requires HTTP Config Entry Flow.
// Returns true for platforms that cannot be created/deleted via WebSocket API.
func RequiresConfigEntryFlow(platform string) bool {
	return ConfigEntryPlatforms[platform]
}

// ConfigEntryEntityDomain maps Config Entry platforms to their entity domain.
// This is used to determine the expected entity_id prefix for each platform.
var ConfigEntryEntityDomain = map[string]string{
	"threshold":   "binary_sensor",
	"derivative":  "sensor",
	"integration": "sensor",
	"group":       "group",
	"template":    "", // Can be either sensor or binary_sensor
}

// GetConfigEntryEntityDomain returns the expected entity domain for a Config Entry platform.
// Returns empty string for platforms with variable entity domains (like template).
func GetConfigEntryEntityDomain(platform string) string {
	return ConfigEntryEntityDomain[platform]
}

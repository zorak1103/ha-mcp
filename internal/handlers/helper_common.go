// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"slices"
	"strings"
)

// VerboseHint is the hint message shown when verbose mode is available.
const VerboseHint = " (use verbose=true for full details)"

// helperPlatformNamePrefixes are the config-entry platform names that
// ParseHelperEntityID accepts as an entity_id prefix without being an entity
// domain of any helperTypes entry. Every other prefix is derived from
// helperTypes' validEntityDomains, so a new helper type needs no edit here.
var helperPlatformNamePrefixes = []string{
	"template",
	"threshold",
	"derivative",
	platformIntegration, // Note: Home Assistant uses "integration" not "integral"
}

// HelperPlatforms defines all known helper platform prefixes, sorted.
// This list is used across helper handlers for entity ID parsing and validation.
var HelperPlatforms = buildHelperPlatforms()

// buildHelperPlatforms derives the ParseHelperEntityID prefix list from every
// helperTypes entry's validEntityDomains plus helperPlatformNamePrefixes -
// the same single-source pattern as helperEntityDomains and
// templateSubtypeDomains. A domain missing from a hand-written list broke
// update/delete with "invalid entity_id format" while create still worked
// (issue #211). The result is sorted and de-duplicated: helperTypes is a map,
// so its range order is randomized per iteration.
func buildHelperPlatforms() []string {
	platforms := slices.Clone(helperPlatformNamePrefixes)
	for _, meta := range helperTypes {
		platforms = append(platforms, meta.validEntityDomains...)
	}
	slices.Sort(platforms)
	return slices.Compact(platforms)
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

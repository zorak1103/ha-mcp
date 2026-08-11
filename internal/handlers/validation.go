// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"fmt"
	"regexp"
	"strings"
)

// entityIDPattern matches valid entity IDs in format domain.object_id.
// Domain: lowercase letters and underscores.
// Object ID: lowercase letters, numbers, and underscores.
//
// Security-relevant consumer: helpers_consolidated.go's wrapperRecipeFor
// relies on this pattern admitting no quotes or braces to safely interpolate
// a caller-supplied entity_id, unescaped, into a Jinja template string
// inside a ready-to-run manage_helper(...) call. Loosening this pattern
// (e.g. to allow uppercase, hyphens, or other punctuation) without
// re-checking that call site risks turning it into a template-injection
// vector - see TestWrapperRecipeFor_RejectsInjectionPayloads.
var entityIDPattern = regexp.MustCompile(`^[a-z_]+\.[a-z0-9_]+$`)

// ValidateEntityID validates entity ID format (domain.object_id).
// Returns an error if the entity ID is empty, missing a dot separator,
// or contains invalid characters.
func ValidateEntityID(entityID string) error {
	if entityID == "" {
		return fmt.Errorf("entity_id is required")
	}
	if !strings.Contains(entityID, ".") {
		return fmt.Errorf("entity_id must be in format 'domain.object_id': %s", entityID)
	}
	if !entityIDPattern.MatchString(entityID) {
		return fmt.Errorf("entity_id contains invalid characters: %s", entityID)
	}
	return nil
}

// ValidateRange validates that min <= max.
// Returns an error if min is greater than max.
func ValidateRange(minVal, maxVal float64, fieldName string) error {
	if minVal > maxVal {
		return fmt.Errorf("%s: min (%v) must be <= max (%v)", fieldName, minVal, maxVal)
	}
	return nil
}

// GetOptionalString extracts an optional string argument from the args map.
// Returns the value and true if present, or empty string and false if not.
func GetOptionalString(args map[string]any, key string) (string, bool) {
	val, ok := args[key].(string)
	return val, ok && val != ""
}

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

// ValidateEntityIDWithPlatform validates entity ID and checks that it belongs
// to the expected platform (e.g., "input_number", "counter").
func ValidateEntityIDWithPlatform(entityID, expectedPlatform string) error {
	if err := ValidateEntityID(entityID); err != nil {
		return err
	}
	platform, _ := ParseHelperEntityID(entityID)
	if platform != expectedPlatform {
		return fmt.Errorf("entity_id must be a %s helper: %s", expectedPlatform, entityID)
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

// ValidateValueInRange validates that value is within [min, max].
// Returns an error if value is outside the specified range.
func ValidateValueInRange(value, minVal, maxVal float64, fieldName string) error {
	if value < minVal || value > maxVal {
		return fmt.Errorf("%s: value %v outside range [%v, %v]", fieldName, value, minVal, maxVal)
	}
	return nil
}

// GetRequiredString extracts a required string argument from the args map.
// Returns an error if the key doesn't exist or the value is empty.
func GetRequiredString(args map[string]any, key string) (string, error) {
	val, ok := args[key].(string)
	if !ok || val == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return val, nil
}

// GetOptionalString extracts an optional string argument from the args map.
// Returns the value and true if present, or empty string and false if not.
func GetOptionalString(args map[string]any, key string) (string, bool) {
	val, ok := args[key].(string)
	return val, ok && val != ""
}

// GetOptionalFloat64 extracts an optional float64 argument from the args map.
// Returns the value and true if present, or 0 and false if not.
func GetOptionalFloat64(args map[string]any, key string) (float64, bool) {
	val, ok := args[key].(float64)
	return val, ok
}

// GetOptionalInt extracts an optional int argument from the args map.
// JSON numbers are float64, so this handles the conversion.
// Returns the value and true if present, or 0 and false if not.
func GetOptionalInt(args map[string]any, key string) (int, bool) {
	val, ok := args[key].(float64)
	if !ok {
		return 0, false
	}
	return int(val), true
}

// GetOptionalBool extracts an optional bool argument from the args map.
// Returns the value and true if present, or false and false if not.
func GetOptionalBool(args map[string]any, key string) (bool, bool) {
	val, ok := args[key].(bool)
	return val, ok
}

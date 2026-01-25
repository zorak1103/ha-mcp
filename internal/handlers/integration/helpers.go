//go:build integration

// Package integration provides integration tests for Home Assistant write operations.
// These tests run against a real Home Assistant instance and use strict naming
// conventions to ensure test isolation and safe cleanup.
package integration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// TestEntityPrefix is the prefix used for all test entities.
// This ensures test entities can be identified and cleaned up safely.
// Note: We use "mcptest_" without leading underscores because Home Assistant
// removes leading underscores when slugifying names to entity IDs.
const TestEntityPrefix = "mcptest_"

// GenerateTestID creates a unique test entity ID with the test prefix.
// Format: mcptest_<uuid>_<name>
func GenerateTestID(name string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	shortID := hex.EncodeToString(b)
	return fmt.Sprintf("%s%s_%s", TestEntityPrefix, shortID, name)
}

// IsTestEntity checks if an entity ID contains the test prefix.
// This is used to identify entities created by integration tests.
func IsTestEntity(entityID string) bool {
	// Handle both formats: "domain.entity_id" and just "entity_id"
	parts := strings.SplitN(entityID, ".", 2)
	if len(parts) == 2 {
		return strings.HasPrefix(parts[1], TestEntityPrefix)
	}
	return strings.HasPrefix(entityID, TestEntityPrefix)
}

// ValidateTestEntityID ensures an entity ID is a test entity before
// performing destructive operations. Returns an error if the entity
// is not a test entity, preventing accidental modification of
// production entities.
func ValidateTestEntityID(entityID string) error {
	if !IsTestEntity(entityID) {
		return fmt.Errorf("refusing to operate on non-test entity %q: missing prefix %q", entityID, TestEntityPrefix)
	}
	return nil
}

// ExtractEntityID extracts the entity ID part from a full entity ID.
// For "counter.__mcptest_abc_test", returns "__mcptest_abc_test".
func ExtractEntityID(fullEntityID string) string {
	parts := strings.SplitN(fullEntityID, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullEntityID
}

// BuildEntityID constructs a full entity ID from domain and ID.
func BuildEntityID(domain, id string) string {
	return fmt.Sprintf("%s.%s", domain, id)
}

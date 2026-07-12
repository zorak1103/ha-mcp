// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// isYAMLDefinedEntity reports whether entityID is YAML-defined rather than storage/UI-managed,
// and thus not writable via the config REST API (POST /api/config/<domain>/config/<id>). Only
// storage-managed scripts/automations get an entity-registry entry with a non-empty unique_id;
// YAML-defined entities have no unique_id (HA's own log confirms this: "Platform script does
// not generate unique IDs"). Writing to a YAML entity via the config API returns 200 but
// silently creates a duplicate orphan (<entity>_2) instead of updating the original (#122).
//
// checked=false means the registry lookup itself failed; the caller should proceed with the
// write rather than block a legitimate edit because the check couldn't run.
func isYAMLDefinedEntity(ctx context.Context, client homeassistant.Client, entityID string) (isYAML, checked bool) {
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return false, false
	}
	for _, entry := range entries {
		if entry.EntityID == entityID {
			return entry.UniqueID == "", true
		}
	}
	// No registry entry at all - YAML-defined entities aren't tracked in the entity registry.
	return true, true
}

// yamlDefinedWriteError builds the refusal message for a write attempt against a YAML-defined
// entity. domain is "script" or "automation"; action is "update" or "patch" (the action being
// refused); displayID is the user-supplied identifier as echoed in other error messages;
// entityID is the resolved "<domain>.<id>" entity_id.
func yamlDefinedWriteError(domain, action, displayID, entityID string) string {
	return fmt.Sprintf(
		"cannot %s '%s': %s is YAML-defined, not storage-managed. The %s config API "+
			"cannot edit YAML-defined entities - writing would silently create a duplicate "+
			"orphan entity (%s_2) instead of updating the original. Edit the YAML file directly "+
			"and call %s.reload to apply changes.",
		action, displayID, entityID, domain, entityID, domain,
	)
}

// yamlWriteGuardError checks whether entityID is YAML-defined and, if so, returns the refusal
// result to short-circuit the caller's write. Returns nil when the write should proceed
// (storage-managed entity, or the registry check itself failed - see isYAMLDefinedEntity).
// Centralizing the checked&&isYAML branching here keeps handleUpdate/handlePatch below the
// gocyclo threshold.
func yamlWriteGuardError(ctx context.Context, client homeassistant.Client, domain, action, displayID, entityID string) *mcp.ToolsCallResult {
	isYAML, checked := isYAMLDefinedEntity(ctx, client, entityID)
	if !checked || !isYAML {
		return nil
	}
	return errorResult(yamlDefinedWriteError(domain, action, displayID, entityID))
}

// resolveWriteCheckEntityID picks the entity_id to use for the YAML-defined write guard,
// preferring the entity actually resolved from HA (resolvedEntityID, e.g. via a fallback
// alias/UUID search) over the id guessed purely from user input (guessedEntityID) - the
// fallback search may have matched a different underlying entity than the guess.
func resolveWriteCheckEntityID(guessedEntityID, resolvedEntityID string) string {
	if resolvedEntityID != "" {
		return resolvedEntityID
	}
	return guessedEntityID
}

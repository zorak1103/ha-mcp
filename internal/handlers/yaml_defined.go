// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// configFileName maps a domain to the config file Home Assistant's config API manages for it.
func configFileName(domain string) string {
	return domain + "s.yaml"
}

// configFileMissingWriteError builds the refusal message for a write attempt against an id that
// is absent from the domain's managed config file (e.g. it is defined elsewhere - a YAML block
// under !include_dir_merge_list, or inline in configuration.yaml). domain is "automation",
// "script", or "scene"; action is "update" or "patch"; displayID is the user-supplied identifier
// as echoed in other error messages; entityID is the resolved "<domain>.<id>" entity_id;
// configID is the id that was actually probed (and would be written).
func configFileMissingWriteError(ctx context.Context, client homeassistant.Client, domain, action, displayID, entityID, configID string) string {
	fileName := configFileName(domain)

	// ConfigDir is included deliberately: the MCP client already holds a token with full
	// config-read access (GetConfig itself requires no more privilege than any other call this
	// tool makes), so surfacing it is not privilege-escalating - and it's the actionable half of
	// this message (the caller needs to know where to go edit the source file).
	dirHint := ""
	if cfg, err := client.GetConfig(ctx); err == nil && cfg != nil && cfg.ConfigDir != "" {
		dirHint = fmt.Sprintf(" (config directory: %s)", cfg.ConfigDir)
	}

	extra := ""
	if action == "patch" {
		extra = " Use dry_run: true first to see exactly what the write would produce, then paste that into the file by hand."
	}

	return fmt.Sprintf(
		"cannot %s '%s': %s (id %q) is not present in %s%s - it is defined elsewhere (e.g. a block "+
			"in configuration.yaml, or a file pulled in via !include/!include_dir_merge_list). The %s "+
			"config API can only edit entries that already exist in %s; writing this id would silently "+
			"append a duplicate orphan entity (%s_2) instead of updating the original. Edit the source "+
			"file directly and call %s.reload to apply changes.%s",
		action, displayID, entityID, configID, fileName, dirHint, domain, fileName, entityID, domain, extra,
	)
}

// configWriteGuardError checks whether configID exists in the config file Home Assistant's
// config API manages for domain and, if it is confirmed absent, returns the refusal result to
// short-circuit the caller's write. Returns nil when the write should proceed - either the
// entry is confirmed present, or the probe itself failed and the write proceeds rather than
// blocking a legitimate edit because a check couldn't run (same convention the prior
// unique_id-based guard used).
func configWriteGuardError(
	ctx context.Context,
	client homeassistant.Client,
	domain, action, displayID, entityID, configID string,
) *mcp.ToolsCallResult {
	exists, err := client.ConfigFileEntryExists(ctx, domain, configID)
	if err != nil || exists {
		return nil
	}
	return errorResult(configFileMissingWriteError(ctx, client, domain, action, displayID, entityID, configID))
}

// resolveWriteCheckEntityID picks the entity_id to use for the config-file write guard,
// preferring the entity actually resolved from HA (resolvedEntityID, e.g. via a fallback
// alias/UUID search) over the id guessed purely from user input (guessedEntityID) - the
// fallback search may have matched a different underlying entity than the guess.
func resolveWriteCheckEntityID(guessedEntityID, resolvedEntityID string) string {
	if resolvedEntityID != "" {
		return resolvedEntityID
	}
	return guessedEntityID
}

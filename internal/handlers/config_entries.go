// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	configEntryActionList   = "list"
	configEntryActionGet    = "get"
	configEntryActionDelete = "delete"
)

// ConfigEntryHandlers provides MCP tool handlers for config entry operations.
type ConfigEntryHandlers struct{}

// NewConfigEntryHandlers creates a new ConfigEntryHandlers instance.
func NewConfigEntryHandlers() *ConfigEntryHandlers {
	return &ConfigEntryHandlers{}
}

// RegisterTools registers all config entry tools with the registry.
func (h *ConfigEntryHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageConfigEntryTool(), h.handleManageConfigEntry)
}

// manageConfigEntryTool returns the consolidated tool definition for config entry operations.
func (h *ConfigEntryHandlers) manageConfigEntryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "manage_config_entry",
		Description: "Manage Home Assistant config entries. Config entries store metadata about integrations and helpers (domain, title, state, entry_id). By default returns natural language format optimized for LLMs. Use 'format=json' for structured data. Note: Template definitions are stored but not exposed through this API.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Config entry management parameters",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type: "string",
					Description: "Action to perform: 'list' (list config entries, optionally filtered by domain), " +
						"'get' (get a single config entry by entry_id), 'delete' (remove a config entry and all its " +
						"associated devices/entities — requires entry_id; destructive, blocked in read-only mode)",
					Enum: []string{configEntryActionList, configEntryActionGet, configEntryActionDelete},
				},
				"domain": {
					Type:        "string",
					Description: "(For action=list) Filter by domain (e.g., 'template', 'hue', 'zwave_js'). If not specified, returns all config entries.",
				},
				"entry_id": {
					Type:        "string",
					Description: "(For action=get) The config entry ID to retrieve (e.g., from entity registry's config_entry_id field). Use get_registry with type=entities and verbose=true to find the config_entry_id for a specific entity.",
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'natural' (default, human-readable LLM-optimized) or 'json' (structured data)",
					Enum:        []string{"natural", "json"},
				},
			},
			Required: []string{"action"},
		},
	}
}

// handleManageConfigEntry dispatches config entry management requests.
func (h *ConfigEntryHandlers) handleManageConfigEntry(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action := getString(args, "action")
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case configEntryActionList:
		return h.handleListConfigEntries(ctx, client, args)
	case configEntryActionGet:
		return h.handleGetConfigEntry(ctx, client, args)
	case configEntryActionDelete:
		return h.handleDeleteConfigEntry(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action %q, must be one of: list, get, delete", action)), nil
	}
}

// handleListConfigEntries handles requests to list config entries.
func (h *ConfigEntryHandlers) handleListConfigEntries(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	domain := getString(args, "domain")

	entries, err := client.GetConfigEntries(ctx, domain)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting config entries: %v", err)), nil
	}

	format := formatter.ParseFormat(getString(args, "format"))
	if format == formatter.FormatNatural {
		return h.formatListNatural(entries, domain), nil
	}

	return h.formatListJSON(entries, domain)
}

// handleGetConfigEntry handles requests to get a single config entry.
func (h *ConfigEntryHandlers) handleGetConfigEntry(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	entryID := getString(args, "entry_id")
	if entryID == "" {
		return errorResult("entry_id is required"), nil
	}

	entry, err := client.GetConfigEntry(ctx, entryID)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting config entry: %v", err)), nil
	}

	format := formatter.ParseFormat(getString(args, "format"))
	if format == formatter.FormatNatural {
		return h.formatGetNatural(entry), nil
	}

	return h.formatGetJSON(entry)
}

// handleDeleteConfigEntry handles requests to delete a config entry and its
// associated devices/entities. Fetches the entry first so the success message
// can name what was removed, and so an unknown entry_id fails before any mutation.
func (h *ConfigEntryHandlers) handleDeleteConfigEntry(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	entryID := getString(args, "entry_id")
	if entryID == "" {
		return errorResult("entry_id is required"), nil
	}

	entry, err := client.GetConfigEntry(ctx, entryID)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting config entry: %v", err)), nil
	}

	// Counted before deletion: once the entry is gone, the registries no longer
	// show these entities/devices as belonging to it (and the registry cache is
	// invalidated), so counting after deletion would always yield zero.
	entityCount, deviceCount, sampleEntityID := countConfigEntryResources(ctx, client, entryID)

	requireRestart, err := client.DeleteConfigEntry(ctx, entryID)
	if err != nil {
		return errorResult(fmt.Sprintf("error deleting config entry: %v", err)), nil
	}

	msg := buildDeleteConfigEntryMessage(entry, entryID, entityCount, deviceCount, requireRestart)

	// Smart Wait: config entry removal unloads asynchronously, so an immediate
	// follow-up read can still see the entry's entities. Skip the probe when HA
	// already reported require_restart — it told us directly that unload didn't
	// finish, so polling one entity would just rediscover the same fact slower.
	if !requireRestart && sampleEntityID != "" && !waitForEntityDisappear(ctx, client, sampleEntityID) {
		msg += "\nWarning: at least one associated entity is still visible after the wait timeout; a Home Assistant restart may be required to finish removing it."
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(msg)},
	}, nil
}

// countConfigEntryResources counts how many entity registry entries and device
// registry entries belong to the given config entry, for use in the delete
// success message. A registry fetch failure is non-fatal (best-effort,
// -1 signals "unknown") — the delete itself has already been validated via the
// preflight GetConfigEntry call and should not be blocked by this enrichment.
// sampleEntityID is one arbitrary entity belonging to the entry (empty if none
// found), used as a Smart Wait probe after the delete completes.
func countConfigEntryResources(ctx context.Context, client homeassistant.Client, entryID string) (entityCount, deviceCount int, sampleEntityID string) {
	entityCount = -1
	if entries, err := client.GetEntityRegistry(ctx); err == nil {
		entityCount = 0
		for _, entry := range entries {
			if entry.ConfigEntryID == entryID {
				entityCount++
				if sampleEntityID == "" {
					sampleEntityID = entry.EntityID
				}
			}
		}
	}

	deviceCount = -1
	if devices, err := client.GetDeviceRegistry(ctx); err == nil {
		deviceCount = 0
		for _, device := range devices {
			if slices.Contains(device.ConfigEntries, entryID) {
				deviceCount++
			}
		}
	}

	return entityCount, deviceCount, sampleEntityID
}

// buildDeleteConfigEntryMessage builds the delete success message, including
// entity/device counts when known. Either count may be -1 (unknown) if its
// registry fetch failed; the wording degrades gracefully in that case rather
// than reporting a misleading zero. Counts are phrased in the past tense
// ("had") since they were measured before the delete call, not after —
// Home Assistant does not report the actual resources removed.
func buildDeleteConfigEntryMessage(entry *homeassistant.ConfigEntryFull, entryID string, entityCount, deviceCount int, requireRestart bool) string {
	title := entry.Title
	if title == "" {
		title = entry.Domain
	}
	base := fmt.Sprintf("Deleted config entry '%s' (domain: %s, entry_id: %s)", title, entry.Domain, entryID)

	entityPhrase := pluralCount(entityCount, "associated entity", "associated entities")
	devicePhrase := pluralCount(deviceCount, "associated device", "associated devices")

	var msg string
	switch {
	case entityCount >= 0 && deviceCount >= 0:
		msg = fmt.Sprintf("%s — it had %s and %s.", base, entityPhrase, devicePhrase)
	case entityCount >= 0:
		msg = fmt.Sprintf("%s — it had %s (device count unknown: registry lookup failed).", base, entityPhrase)
	case deviceCount >= 0:
		msg = fmt.Sprintf("%s — it had %s (entity count unknown: registry lookup failed).", base, devicePhrase)
	default:
		msg = fmt.Sprintf("%s (entity/device counts unknown: registry lookup failed).", base)
	}

	if requireRestart {
		msg += " Home Assistant reports a restart is required to finish unloading it; its entities/devices remain registered until Home Assistant restarts."
	}

	return msg
}

// pluralCount renders a count with the singular or plural form of a noun
// phrase. A count of -1 (unknown) is never passed here — callers branch on
// that before calling.
func pluralCount(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// formatListNatural formats config entry list in natural language.
func (h *ConfigEntryHandlers) formatListNatural(entries []homeassistant.ConfigEntryFull, domain string) *mcp.ToolsCallResult {
	var output strings.Builder

	summary := fmt.Sprintf("Found %d config entries", len(entries))
	if domain != "" {
		summary += fmt.Sprintf(" for domain '%s'", domain)
	}
	output.WriteString(summary)

	if len(entries) > 0 {
		output.WriteString("\n\n")
		for i, entry := range entries {
			if i > 0 {
				output.WriteString("\n")
			}
			fmt.Fprintf(&output, "- %s (%s)\n", entry.Title, entry.Domain)
			fmt.Fprintf(&output, "  Entry ID: %s\n", entry.EntryID)
			fmt.Fprintf(&output, "  State: %s", entry.State)
		}
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(output.String())},
	}
}

// formatListJSON formats config entry list as JSON.
func (h *ConfigEntryHandlers) formatListJSON(entries []homeassistant.ConfigEntryFull, domain string) (*mcp.ToolsCallResult, error) {
	output, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("error formatting response: %v", err)), nil
	}

	summary := fmt.Sprintf("Found %d config entries", len(entries))
	if domain != "" {
		summary += fmt.Sprintf(" for domain '%s'", domain)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(output))},
	}, nil
}

// formatGetNatural formats single config entry in natural language.
func (h *ConfigEntryHandlers) formatGetNatural(entry *homeassistant.ConfigEntryFull) *mcp.ToolsCallResult {
	var output strings.Builder

	fmt.Fprintf(&output, "Config Entry: %s\n", entry.Title)
	fmt.Fprintf(&output, "Domain: %s\n", entry.Domain)
	fmt.Fprintf(&output, "Entry ID: %s\n", entry.EntryID)
	fmt.Fprintf(&output, "State: %s", entry.State)

	if len(entry.Options) > 0 {
		output.WriteString("\n\nOptions:")
		optionsJSON, _ := json.MarshalIndent(entry.Options, "  ", "  ")
		fmt.Fprintf(&output, "\n  %s", string(optionsJSON))
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(output.String())},
	}
}

// formatGetJSON formats single config entry as JSON.
func (h *ConfigEntryHandlers) formatGetJSON(entry *homeassistant.ConfigEntryFull) (*mcp.ToolsCallResult, error) {
	output, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("error formatting response: %v", err)), nil
	}

	summary := fmt.Sprintf("Config entry '%s' (%s)", entry.Title, entry.Domain)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(output))},
	}, nil
}

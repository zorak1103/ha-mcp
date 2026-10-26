package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// HACS action constants.
const (
	hacsActionInfo             = "info"
	hacsActionList             = "list"
	hacsActionGet              = "get"
	hacsActionReleases         = "releases"
	hacsActionReleaseNotes     = "release_notes"
	hacsActionCritical         = "critical"
	hacsActionDownload         = "download"
	hacsActionUninstall        = "uninstall"
	hacsActionAddRepository    = "add_repository"
	hacsActionRemoveRepository = "remove_repository"
	hacsActionRefresh          = "refresh"
	hacsActionToggleBeta       = "toggle_beta"
)

// HACSHandlers handles HACS (Home Assistant Community Store) operations.
type HACSHandlers struct{}

// NewHACSHandlers creates a new HACSHandlers instance.
func NewHACSHandlers() *HACSHandlers {
	return &HACSHandlers{}
}

// RegisterTools registers HACS tools with the MCP registry.
func (h *HACSHandlers) RegisterTools(registry *mcp.Registry) {
	tool := buildHACSSchema()
	registry.RegisterTool(*tool, h.HandleManageHACS)
}

//nolint:funlen // Schema properties require detailed descriptions for proper tool documentation
func buildHACSSchema() *mcp.Tool {
	return &mcp.Tool{
		Name:        "manage_hacs",
		Description: "Manage HACS (Home Assistant Community Store) repositories. HACS is an optional third-party add-on for installing custom integrations, plugins, themes, and Python scripts. Supports listing repositories, checking updates, installing/uninstalling, adding custom repositories, and toggling beta versions.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Action to perform",
					Enum: []string{
						hacsActionInfo, hacsActionList, hacsActionGet,
						hacsActionReleases, hacsActionReleaseNotes, hacsActionCritical,
						hacsActionDownload, hacsActionUninstall,
						hacsActionAddRepository, hacsActionRemoveRepository,
						hacsActionRefresh, hacsActionToggleBeta,
					},
				},
				"format": {
					Type:        "string",
					Description: "Output format (natural: human-readable, json: structured JSON)",
					Enum:        []string{"natural", "json"},
					Default:     "natural",
				},
				"repository_id": {
					Type:        "string",
					Description: "Repository ID (required for get, releases, release_notes, download, uninstall, remove_repository, refresh, toggle_beta)",
				},
				"repository": {
					Type:        "string",
					Description: "Repository full name (owner/repo) (required for add_repository)",
				},
				"category": {
					Type:        "string",
					Description: "Repository category (required for add_repository, client-side filter for list)",
					Enum:        []string{"integration", "plugin", "theme", "python_script", "appdaemon", "netdaemon", "template"},
				},
				"search": {
					Type:        "string",
					Description: "Search filter for list action - case-insensitive substring match against name, description, and full_name",
				},
				"version": {
					Type:        "string",
					Description: "Specific version to download (optional for download, defaults to latest)",
				},
				"show_beta": {
					Type:        "boolean",
					Description: "Enable/disable beta versions (required for toggle_beta)",
				},
				"installed_only": {
					Type:        "boolean",
					Description: "Filter to only show installed repositories (optional for list)",
				},
				"pending_update": {
					Type:        "boolean",
					Description: "Filter to only show repositories with pending updates (optional for list)",
				},
			},
			Required: []string{"action"},
		},
	}
}

// HandleManageHACS handles HACS operations.
func (h *HACSHandlers) HandleManageHACS(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	// Extract action
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return errorResult("action parameter is required"), nil
	}

	// Extract format
	format := getStringArg(args, "format")
	if format == "" {
		format = "natural"
	}

	// Dispatch to action handlers
	return h.dispatchAction(ctx, client, action, args, format)
}

// dispatchAction routes the action to the appropriate handler.
func (h *HACSHandlers) dispatchAction(
	ctx context.Context,
	client homeassistant.Client,
	action string,
	args map[string]any,
	format string,
) (*mcp.ToolsCallResult, error) {
	switch action {
	case hacsActionInfo:
		return h.handleInfo(ctx, client, format)
	case hacsActionList:
		return h.handleList(ctx, client, args, format)
	case hacsActionGet:
		return h.handleGet(ctx, client, args, format)
	case hacsActionReleases:
		return h.handleReleases(ctx, client, args, format)
	case hacsActionReleaseNotes:
		return h.handleReleaseNotes(ctx, client, args, format)
	case hacsActionCritical:
		return h.handleCritical(ctx, client, format)
	case hacsActionDownload:
		return h.handleDownload(ctx, client, args)
	case hacsActionUninstall:
		return h.handleUninstall(ctx, client, args)
	case hacsActionAddRepository:
		return h.handleAddRepository(ctx, client, args)
	case hacsActionRemoveRepository:
		return h.handleRemoveRepository(ctx, client, args)
	case hacsActionRefresh:
		return h.handleRefresh(ctx, client, args)
	case hacsActionToggleBeta:
		return h.handleToggleBeta(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (valid: info, list, get, releases, release_notes, critical, download, uninstall, add_repository, remove_repository, refresh, toggle_beta)", action)), nil
	}
}

// =============================================================================
// Read Actions
// =============================================================================

func (h *HACSHandlers) handleInfo(ctx context.Context, client homeassistant.Client, format string) (*mcp.ToolsCallResult, error) {
	result, err := client.SendHACSCommand(ctx, "hacs/info", nil)
	if err != nil {
		return h.handleHACSError(err), nil
	}

	if format == formatJSON {
		b, _ := json.MarshalIndent(result, "", "  ")
		return successResult(string(b)), nil
	}
	return successResult(h.formatInfo(result)), nil
}

func (h *HACSHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// HACS API does not accept any server-side filters - all filtering is client-side
	result, err := client.SendHACSCommand(ctx, "hacs/repositories/list", nil)
	if err != nil {
		return h.handleHACSError(err), nil
	}

	// Client-side filters
	category := getStringArg(args, "category")
	search := getStringArg(args, "search")
	installedOnly := getBoolArg(args, "installed_only")
	pendingUpdate := getBoolArg(args, "pending_update")
	if repos, ok := result.([]any); ok {
		result = filterHACSRepos(repos, category, search, installedOnly, pendingUpdate)
	}

	if format == formatJSON {
		b, _ := json.MarshalIndent(result, "", "  ")
		return successResult(string(b)), nil
	}
	return successResult(h.formatList(result)), nil
}

func (h *HACSHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	result, err := client.SendHACSCommand(ctx, "hacs/repository/info", map[string]any{
		"repository_id": repoID,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	if format == formatJSON {
		b, _ := json.MarshalIndent(result, "", "  ")
		return successResult(string(b)), nil
	}
	return successResult(h.formatRepository(result)), nil
}

func (h *HACSHandlers) handleReleases(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	result, err := client.SendHACSCommand(ctx, "hacs/repository/releases", map[string]any{
		"repository_id": repoID,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	if format == formatJSON {
		b, _ := json.MarshalIndent(result, "", "  ")
		return successResult(string(b)), nil
	}
	return successResult(h.formatReleases(result)), nil
}

func (h *HACSHandlers) handleReleaseNotes(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	result, err := client.SendHACSCommand(ctx, "hacs/repository/release_notes", map[string]any{
		"repository_id": repoID,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	if format == formatJSON {
		b, _ := json.MarshalIndent(result, "", "  ")
		return successResult(string(b)), nil
	}
	return successResult(h.formatReleaseNotes(result)), nil
}

func (h *HACSHandlers) handleCritical(ctx context.Context, client homeassistant.Client, format string) (*mcp.ToolsCallResult, error) {
	result, err := client.SendHACSCommand(ctx, "hacs/critical/list", nil)
	if err != nil {
		return h.handleHACSError(err), nil
	}

	if format == formatJSON {
		b, _ := json.MarshalIndent(result, "", "  ")
		return successResult(string(b)), nil
	}
	return successResult(h.formatCritical(result)), nil
}

// =============================================================================
// Write Actions
// =============================================================================

func (h *HACSHandlers) handleDownload(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	data := map[string]any{"repository": repoID}
	if version, ok := args["version"].(string); ok && version != "" {
		data["version"] = version
	}

	_, err := client.SendHACSCommand(ctx, "hacs/repository/download", data)
	if err != nil {
		return h.handleHACSError(err), nil
	}

	msg := fmt.Sprintf("Repository %s downloaded successfully", repoID)
	if version, ok := args["version"].(string); ok && version != "" {
		msg = fmt.Sprintf("Repository %s version %s downloaded successfully", repoID, version)
	}
	return successResult(msg), nil
}

func (h *HACSHandlers) handleUninstall(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	_, err := client.SendHACSCommand(ctx, "hacs/repository/remove", map[string]any{
		"repository_id": repoID,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	return successResult(fmt.Sprintf("Repository %s uninstalled successfully", repoID)), nil
}

func (h *HACSHandlers) handleAddRepository(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	repository, ok := args["repository"].(string)
	if !ok || repository == "" {
		return errorResult("repository parameter is required"), nil
	}

	category, ok := args["category"].(string)
	if !ok || category == "" {
		return errorResult("category parameter is required"), nil
	}

	_, err := client.SendHACSCommand(ctx, "hacs/repositories/add", map[string]any{
		"repository": repository,
		"category":   category,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	return successResult(fmt.Sprintf("Repository %s added successfully", repository)), nil
}

func (h *HACSHandlers) handleRemoveRepository(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	_, err := client.SendHACSCommand(ctx, "hacs/repositories/remove", map[string]any{
		"repository_id": repoID,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	return successResult(fmt.Sprintf("Repository %s removed successfully", repoID)), nil
}

func (h *HACSHandlers) handleRefresh(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	_, err := client.SendHACSCommand(ctx, "hacs/repository/refresh", map[string]any{
		"repository_id": repoID,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	return successResult(fmt.Sprintf("Repository %s refreshed successfully", repoID)), nil
}

func (h *HACSHandlers) handleToggleBeta(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	repoID, ok := args["repository_id"].(string)
	if !ok || repoID == "" {
		return errorResult("repository_id parameter is required"), nil
	}

	showBeta, ok := args["show_beta"].(bool)
	if !ok {
		return errorResult("show_beta parameter is required"), nil
	}

	_, err := client.SendHACSCommand(ctx, "hacs/repository/beta", map[string]any{
		"repository_id": repoID,
		"show_beta":     showBeta,
	})
	if err != nil {
		return h.handleHACSError(err), nil
	}

	status := "disabled"
	if showBeta {
		status = "enabled"
	}
	return successResult(fmt.Sprintf("Beta versions %s for repository %s", status, repoID)), nil
}

// =============================================================================
// Formatting Helpers (Natural Language)
// =============================================================================

func (h *HACSHandlers) formatInfo(data any) string {
	info, ok := data.(map[string]any)
	if !ok {
		return "HACS information unavailable"
	}

	// Format common fields in specific order, then add any additional fields
	var parts []string
	if version, ok := info["version"].(string); ok {
		parts = append(parts, fmt.Sprintf("Version: %s", version))
	}
	if mode, ok := info["lovelace_mode"].(string); ok {
		parts = append(parts, fmt.Sprintf("Lovelace mode: %s", mode))
	}
	if status, ok := info["status"].(string); ok {
		parts = append(parts, fmt.Sprintf("Status: %s", status))
	}

	// Add any other fields not already handled
	for key, value := range info {
		if key != "version" && key != "lovelace_mode" && key != "status" {
			if strVal, ok := value.(string); ok {
				// Convert snake_case to readable format
				displayKey := strings.ReplaceAll(key, "_", " ")
				// Capitalize first letter
				if displayKey != "" {
					displayKey = strings.ToUpper(string(displayKey[0])) + displayKey[1:]
				}
				parts = append(parts, fmt.Sprintf("%s: %s", displayKey, strVal))
			}
		}
	}

	if len(parts) == 0 {
		return "HACS information unavailable"
	}
	return strings.Join(parts, "\n")
}

func (h *HACSHandlers) formatList(data any) string {
	repos, ok := data.([]any)
	if !ok {
		return "No repositories found"
	}

	if len(repos) == 0 {
		return "No repositories found"
	}

	var parts []string
	for _, r := range repos {
		repo, ok := r.(map[string]any)
		if !ok {
			continue
		}
		name := getMapString(repo, "name", "unknown")
		category := getMapString(repo, "category", "")
		status := getMapString(repo, "status", "")

		line := fmt.Sprintf("- %s", name)
		if category != "" {
			line += fmt.Sprintf(" (%s)", category)
		}
		if status != "" {
			line += fmt.Sprintf(" [%s]", status)
		}
		parts = append(parts, line)
	}

	return strings.Join(parts, "\n")
}

func (h *HACSHandlers) formatRepository(data any) string {
	repo, ok := data.(map[string]any)
	if !ok {
		return "Repository information unavailable"
	}

	var parts []string
	if name, ok := repo["name"].(string); ok {
		parts = append(parts, fmt.Sprintf("Name: %s", name))
	}
	if category, ok := repo["category"].(string); ok {
		parts = append(parts, fmt.Sprintf("Category: %s", category))
	}
	if status, ok := repo["status"].(string); ok {
		parts = append(parts, fmt.Sprintf("Status: %s", status))
	}

	if len(parts) == 0 {
		return "Repository information unavailable"
	}
	return strings.Join(parts, "\n")
}

func (h *HACSHandlers) formatReleases(data any) string {
	releases, ok := data.([]any)
	if !ok {
		return "No releases found"
	}

	if len(releases) == 0 {
		return "No releases found"
	}

	var parts []string
	for _, r := range releases {
		rel, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if tag, ok := rel["tag"].(string); ok {
			parts = append(parts, fmt.Sprintf("- %s", tag))
		}
	}

	return strings.Join(parts, "\n")
}

func (h *HACSHandlers) formatReleaseNotes(data any) string {
	notes, ok := data.(map[string]any)
	if !ok {
		return "Release notes unavailable"
	}

	var parts []string
	if tag, ok := notes["tag"].(string); ok {
		parts = append(parts, fmt.Sprintf("Version: %s", tag))
	}
	if body, ok := notes["body"].(string); ok {
		parts = append(parts, fmt.Sprintf("\n%s", body))
	}

	if len(parts) == 0 {
		return "Release notes unavailable"
	}
	return strings.Join(parts, "\n")
}

func (h *HACSHandlers) formatCritical(data any) string {
	critical, ok := data.([]any)
	if !ok {
		return "No critical repositories found"
	}

	if len(critical) == 0 {
		return "No critical repositories found"
	}

	var parts []string
	for _, c := range critical {
		item, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if repo, ok := item["repository"].(string); ok {
			parts = append(parts, fmt.Sprintf("- %s", repo))
		}
	}

	return strings.Join(parts, "\n")
}

// =============================================================================
// Error Handling
// =============================================================================

func (h *HACSHandlers) handleHACSError(err error) *mcp.ToolsCallResult {
	errMsg := err.Error()
	if strings.Contains(errMsg, "unknown_command") {
		return errorResult("HACS is not installed or not accessible. Please install HACS first: https://hacs.xyz/docs/setup/download")
	}
	return errorResult(fmt.Sprintf("HACS operation failed: %s", errMsg))
}

// =============================================================================
// Helper Functions
// =============================================================================

func getMapString(m map[string]any, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

// filterHACSRepos applies client-side filtering to repository list.
func filterHACSRepos(repos []any, category, search string, installedOnly, pendingUpdate bool) []any {
	if category == "" && search == "" && !installedOnly && !pendingUpdate {
		return repos
	}

	var filtered []any
	searchLower := strings.ToLower(search)

	for _, r := range repos {
		repo, ok := r.(map[string]any)
		if !ok {
			continue
		}

		if shouldIncludeHACSRepo(repo, category, searchLower, installedOnly, pendingUpdate) {
			filtered = append(filtered, repo)
		}
	}

	return filtered
}

// shouldIncludeHACSRepo checks if a repository passes all active filters.
func shouldIncludeHACSRepo(repo map[string]any, category, searchLower string, installedOnly, pendingUpdate bool) bool {
	// Apply installed_only filter
	if installedOnly {
		installed, _ := repo["installed"].(bool)
		if !installed {
			return false
		}
	}

	// Apply pending_update filter
	if pendingUpdate {
		pendingUpgrade, _ := repo["pending_upgrade"].(bool)
		if !pendingUpgrade {
			return false
		}
	}

	// Apply category filter (exact match)
	if category != "" {
		repoCategory := getMapString(repo, "category", "")
		if repoCategory != category {
			return false
		}
	}

	// Apply search filter (substring match)
	if searchLower != "" {
		if !matchesHACSSearch(repo, searchLower) {
			return false
		}
	}

	return true
}

// matchesHACSSearch checks if a repository matches the search query.
func matchesHACSSearch(repo map[string]any, searchLower string) bool {
	// Check name
	name := strings.ToLower(getMapString(repo, "name", ""))
	if strings.Contains(name, searchLower) {
		return true
	}

	// Check description
	description := strings.ToLower(getMapString(repo, "description", ""))
	if strings.Contains(description, searchLower) {
		return true
	}

	// Check full_name
	fullName := strings.ToLower(getMapString(repo, "full_name", ""))
	return strings.Contains(fullName, searchLower)
}

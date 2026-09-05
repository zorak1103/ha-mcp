// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	statActionList     = "list"
	statActionValidate = "validate"
	statActionClear    = "clear"

	statTypeMean = "mean"
	statTypeSum  = "sum"

	// StatisticMeanType values (HA >= 2024.11): NONE=0, ARITHMETIC=1,
	// CIRCULAR=2. has_sum is an independent flag, not mean_type==2.
	meanTypeNone       = 0
	meanTypeArithmetic = 1
	meanTypeCircular   = 2
)

// statisticIssueLabels describes known recorder validation issue types in
// natural output; unknown types render with the raw type string only.
var statisticIssueLabels = map[string]string{
	"no_state": "backing entity no longer exists (orphaned statistic)",
}

// StatisticsHandlers provides handlers for Home Assistant recorder
// statistics operations (Developer Tools -> Statistics).
type StatisticsHandlers struct{}

// NewStatisticsHandlers creates a new StatisticsHandlers instance.
func NewStatisticsHandlers() *StatisticsHandlers {
	return &StatisticsHandlers{}
}

// RegisterTools registers the statistics management tool with the registry.
func (h *StatisticsHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageStatisticsTool(), h.handleManageStatistics)
}

// manageStatisticsTool returns the tool definition for statistics management.
func (h *StatisticsHandlers) manageStatisticsTool() mcp.Tool {
	return mcp.Tool{
		Name: "manage_statistics",
		Description: "Manage long-term statistics in the Home Assistant recorder database. " +
			"Use action=list to enumerate known statistic ids (including orphaned ids whose backing entity was removed, e.g. after a Zigbee re-pair), " +
			"action=validate to get the validation issue list driving Developer Tools -> Statistics (issue type no_state marks orphaned statistics), " +
			"action=clear to permanently remove statistics data and metadata for given ids.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []string{statActionList, statActionValidate, statActionClear},
					Description: "Action to perform: 'list' known statistic ids, 'validate' for the validation issue list, 'clear' to remove data+metadata",
				},
				"statistic_type": {
					Type:        "string",
					Enum:        []string{statTypeMean, statTypeSum},
					Description: "Optional filter for action=list: only ids supporting this statistic type",
				},
				"statistic_ids": {
					Type:        "array",
					Items:       &mcp.JSONSchema{Type: "string"},
					Description: "Required for action=clear: statistic ids to purge (e.g. sensor.0x00124b002a50e881_battery)",
				},
				"limit": {
					Type: "integer",
					Description: "Optional maximum number of statistic ids to return for action=list, " +
						"or statistic ids to include for action=validate; ignored for action=clear",
				},
				"format": {
					Type:        "string",
					Enum:        []string{formatNatural, formatJSON},
					Description: "Output format for action=list/validate: 'natural' for readable text (default), 'json' for structured JSON; ignored for action=clear",
				},
			},
			Required: []string{"action"},
		},
	}
}

// handleManageStatistics dispatches to the appropriate action handler.
func (h *StatisticsHandlers) handleManageStatistics(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action := getString(args, "action")
	switch action {
	case statActionList:
		return h.handleStatisticList(ctx, client, args)
	case statActionValidate:
		return h.handleStatisticValidate(ctx, client, args)
	case statActionClear:
		return h.handleStatisticClear(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action %q: must be one of [list, validate, clear]", action)), nil
	}
}

// handleStatisticList lists recorder statistic ids (list_statistic_ids).
func (h *StatisticsHandlers) handleStatisticList(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	statisticType := getString(args, "statistic_type")
	if statisticType != "" && statisticType != statTypeMean && statisticType != statTypeSum {
		return errorResult(fmt.Sprintf("invalid statistic_type %q: must be %q or %q", statisticType, statTypeMean, statTypeSum)), nil
	}
	if err := validateLimitArg(args); err != nil {
		return errorResult(err.Error()), nil
	}

	metas, err := client.ListStatisticIDs(ctx, statisticType)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing statistic ids: %v", err)), nil
	}

	metas, total, truncated := applyStatisticLimit(metas, getInt(args, "limit"))

	format := getString(args, "format")
	if format == formatJSON {
		out, err := formatStatisticListJSON(metas, total, truncated)
		if err != nil {
			return errorResult(fmt.Sprintf("Error formatting output: %v", err)), nil
		}
		return successResult(out), nil
	}

	return successResult(formatStatisticListNatural(metas, total)), nil
}

// handleStatisticValidate returns the recorder validation issue list.
func (h *StatisticsHandlers) handleStatisticValidate(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	if err := validateLimitArg(args); err != nil {
		return errorResult(err.Error()), nil
	}

	issues, err := client.ValidateStatistics(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error validating statistics: %v", err)), nil
	}

	issues, totalIDs, truncated := applyValidateLimit(issues, getInt(args, "limit"))

	format := getString(args, "format")
	if format == formatJSON {
		out, err := formatValidateJSON(issues, totalIDs, truncated)
		if err != nil {
			return errorResult(fmt.Sprintf("Error formatting output: %v", err)), nil
		}
		return successResult(out), nil
	}

	return successResult(formatValidateNatural(issues, totalIDs, truncated)), nil
}

// handleStatisticClear removes statistics data and metadata for given ids.
func (h *StatisticsHandlers) handleStatisticClear(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	statIDs, err := parseStatisticIDsStrict(args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	if err := client.ClearStatistics(ctx, statIDs); err != nil {
		return errorResult(fmt.Sprintf("Error clearing statistics: %v", err)), nil
	}

	return successResult(fmt.Sprintf(
		"Cleared statistics for %d %s: %s",
		len(statIDs), pluralize(len(statIDs), "statistic id", "statistic ids"),
		strings.Join(statIDs, ", "),
	)), nil
}

// maxClearStatisticIDs bounds one clear call's batch size, so a single
// request can't build an oversized WebSocket frame.
const maxClearStatisticIDs = 500

// statisticIDPattern matches HA's two statistic_id shapes: an entity-backed
// id ("domain.object_id") or an integration-provided external statistic id
// ("domain:key", e.g. "growatt:total_energy").
var statisticIDPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*[.:][a-z0-9_]+$`)

// parseStatisticIDsStrict extracts and strictly validates statistic_ids for
// the destructive clear action. Unlike parseStatisticIDs (used by the
// read-only get_statistics path, which silently drops malformed elements),
// clear is irreversible: a silently dropped or malformed id would make the
// tool's success message misrepresent what was actually purged. Every
// element must be a well-formed, non-empty statistic id; duplicates are
// dropped (order-preserving) and the batch is capped.
func parseStatisticIDsStrict(args map[string]any) ([]string, error) {
	statIDsRaw, ok := args["statistic_ids"]
	if !ok {
		return nil, fmt.Errorf("statistic_ids is required")
	}

	statIDsSlice, ok := statIDsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("statistic_ids must be an array")
	}

	if len(statIDsSlice) == 0 {
		return nil, fmt.Errorf("at least one statistic_id is required")
	}
	if len(statIDsSlice) > maxClearStatisticIDs {
		return nil, fmt.Errorf("too many statistic_ids: got %d, max %d", len(statIDsSlice), maxClearStatisticIDs)
	}

	seen := make(map[string]bool, len(statIDsSlice))
	statIDs := make([]string, 0, len(statIDsSlice))
	for i, raw := range statIDsSlice {
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("statistic_ids[%d] must be a string, got %T", i, raw)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, fmt.Errorf("statistic_ids[%d] must not be empty", i)
		}
		if !statisticIDPattern.MatchString(s) {
			return nil, fmt.Errorf(
				"statistic_ids[%d] %q is not a valid statistic id (expected domain.object_id or domain:key)", i, s)
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		statIDs = append(statIDs, s)
	}

	return statIDs, nil
}

// validateLimitArg rejects a limit argument that isn't a non-negative
// integer. getInt silently reads anything non-numeric (a string "10") or
// negative as "no limit" - explicit validation here mirrors the
// statistic_type check above it instead of hiding a caller's mistake behind
// unbounded output.
func validateLimitArg(args map[string]any) error {
	raw, ok := args["limit"]
	if !ok {
		return nil
	}
	limitF, isNum := raw.(float64)
	if !isNum || limitF < 0 || limitF != math.Trunc(limitF) {
		return fmt.Errorf("invalid limit %v: must be a non-negative integer", raw)
	}
	return nil
}

// applyStatisticLimit truncates the list client-side and reports the total.
func applyStatisticLimit(metas []homeassistant.StatisticMeta, limit int) ([]homeassistant.StatisticMeta, int, bool) {
	total := len(metas)
	if limit > 0 && total > limit {
		return metas[:limit], total, true
	}
	return metas, total, false
}

// applyValidateLimit truncates the validation issue map to at most limit
// statistic ids, chosen by sorted id for determinism, and reports the total
// id count and whether truncation occurred. Mirrors applyStatisticLimit for
// action=validate, which previously had no limit at all.
func applyValidateLimit(
	issues map[string][]homeassistant.StatisticValidationIssue,
	limit int,
) (map[string][]homeassistant.StatisticValidationIssue, int, bool) {
	total := len(issues)
	if limit <= 0 || total <= limit {
		return issues, total, false
	}

	ids := make([]string, 0, total)
	for id := range issues {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	limited := make(map[string][]homeassistant.StatisticValidationIssue, limit)
	for _, id := range ids[:limit] {
		limited[id] = issues[id]
	}
	return limited, total, true
}

// statisticCapabilities renders mean/sum capability from whichever HA
// generation's fields are present (mean_type preferred, has_mean fallback).
func statisticCapabilities(m homeassistant.StatisticMeta) string {
	var caps []string
	switch {
	case m.MeanType != nil && *m.MeanType == meanTypeArithmetic:
		caps = append(caps, statTypeMean)
	case m.MeanType != nil && *m.MeanType == meanTypeCircular:
		caps = append(caps, "circular "+statTypeMean)
	case m.MeanType != nil && *m.MeanType == meanTypeNone:
		// No mean capability; explicit case so the fallback below doesn't
		// misrender NONE as an unrecognized mean_type.
	case m.MeanType != nil:
		caps = append(caps, fmt.Sprintf("mean_type: %d", *m.MeanType))
	case m.HasMean != nil && *m.HasMean:
		caps = append(caps, statTypeMean)
	}
	if m.HasSum != nil && *m.HasSum {
		caps = append(caps, statTypeSum)
	}
	return strings.Join(caps, ", ")
}

// formatStatisticListNatural produces a human-readable statistic id list.
func formatStatisticListNatural(metas []homeassistant.StatisticMeta, total int) string {
	if len(metas) == 0 {
		return "No statistic ids found in the recorder database."
	}

	var sb strings.Builder
	if total > len(metas) {
		fmt.Fprintf(&sb, "Found %d statistic %s in the recorder database (showing %d):\n",
			total, pluralize(total, "id", "ids"), len(metas))
	} else {
		fmt.Fprintf(&sb, "Found %d statistic %s in the recorder database:\n",
			len(metas), pluralize(len(metas), "id", "ids"))
	}

	for _, m := range metas {
		statID := formatter.SanitizeDisplayName(m.StatisticID)
		source := formatter.SanitizeDisplayName(m.Source)
		namePart := ""
		if m.Name != nil && *m.Name != "" && *m.Name != m.StatisticID {
			namePart = fmt.Sprintf(" (%s)", formatter.SanitizeDisplayName(*m.Name))
		}
		unit := ""
		if m.StatisticsUnit != nil && *m.StatisticsUnit != "" {
			unit = *m.StatisticsUnit
		} else if m.DisplayUnit != nil && *m.DisplayUnit != "" {
			unit = *m.DisplayUnit
		}
		unitPart := ""
		if unit != "" {
			unitPart = fmt.Sprintf(" — unit: %s", formatter.SanitizeDisplayName(unit))
		}
		capPart := ""
		if caps := statisticCapabilities(m); caps != "" {
			capPart = fmt.Sprintf(" — %s", caps)
		}
		fmt.Fprintf(&sb, "- %s%s (source: %s)%s%s\n", statID, namePart, source, unitPart, capPart)
	}
	return sb.String()
}

// statisticListJSONResponse wraps action=list's JSON output with the
// truncation metadata the natural-format output already conveys via its
// "(showing N)" header — without it, limit + format=json would silently
// hide that more ids exist beyond the returned page.
type statisticListJSONResponse struct {
	StatisticIDs []homeassistant.StatisticMeta `json:"statistic_ids"`
	Total        int                           `json:"total"`
	Returned     int                           `json:"returned"`
	Truncated    bool                          `json:"truncated"`
}

// formatStatisticListJSON serializes the metadata entries as indented JSON.
func formatStatisticListJSON(metas []homeassistant.StatisticMeta, total int, truncated bool) (string, error) {
	if metas == nil {
		metas = []homeassistant.StatisticMeta{}
	}
	resp := statisticListJSONResponse{
		StatisticIDs: metas,
		Total:        total,
		Returned:     len(metas),
		Truncated:    truncated,
	}
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// formatValidateNatural groups validation issues by type with an orphan
// hint for no_state. Output is deterministic: types, ids, and data keys are
// sorted because map iteration order is randomized per range statement.
func formatValidateNatural(issues map[string][]homeassistant.StatisticValidationIssue, totalIDs int, truncated bool) string {
	if len(issues) == 0 {
		return "Statistics validation passed: no issues found in the recorder database."
	}

	totalIssues := 0
	for _, list := range issues {
		totalIssues += len(list)
	}
	var sb strings.Builder
	if truncated {
		fmt.Fprintf(&sb, "Statistics validation found %d %s across %d %s (showing %d, ids may have multiple issues):\n",
			totalIssues, pluralize(totalIssues, "issue", "issues"),
			totalIDs, pluralize(totalIDs, "statistic id", "statistic ids"), len(issues))
	} else {
		fmt.Fprintf(&sb, "Statistics validation found %d %s across %d %s (ids may have multiple issues):\n",
			totalIssues, pluralize(totalIssues, "issue", "issues"),
			len(issues), pluralize(len(issues), "statistic id", "statistic ids"))
	}

	byType := groupIssueIDsByType(issues)
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	slices.Sort(types)

	for _, t := range types {
		ids := byType[t]
		slices.Sort(ids)
		if label, known := statisticIssueLabels[t]; known {
			fmt.Fprintf(&sb, "\n%s (%d %s) — %s:\n", t, len(ids), pluralize(len(ids), "id", "ids"), label)
		} else {
			fmt.Fprintf(&sb, "\n%s (%d %s):\n", t, len(ids), pluralize(len(ids), "id", "ids"))
		}
		for _, id := range ids {
			fmt.Fprintf(&sb, "  - %s\n", formatter.SanitizeDisplayName(id))
			for _, issue := range issues[id] {
				if issue.Type == t {
					sb.WriteString(sortedIssueData(issue))
				}
			}
		}
	}
	return sb.String()
}

// groupIssueIDsByType groups validation issues by issue type, mapping each
// type to the statistic ids that have it.
func groupIssueIDsByType(issues map[string][]homeassistant.StatisticValidationIssue) map[string][]string {
	byType := map[string][]string{}
	seen := map[string]map[string]bool{}
	for id, list := range issues {
		for _, issue := range list {
			if seen[issue.Type] == nil {
				seen[issue.Type] = map[string]bool{}
			}
			if !seen[issue.Type][id] {
				seen[issue.Type][id] = true
				byType[issue.Type] = append(byType[issue.Type], id)
			}
		}
	}
	return byType
}

// sortedIssueData renders one issue's data entries as indented
// "key: value" lines, sorted for deterministic output (map iteration
// order is randomized per range statement).
func sortedIssueData(issue homeassistant.StatisticValidationIssue) string {
	dataKeys := make([]string, 0, len(issue.Data))
	for k := range issue.Data {
		dataKeys = append(dataKeys, k)
	}
	slices.Sort(dataKeys)
	var sb strings.Builder
	for _, k := range dataKeys {
		fmt.Fprintf(&sb, "      %s: %s\n", k, formatter.FormatDetailValue(issue.Data[k]))
	}
	return sb.String()
}

// statisticValidateJSONResponse wraps action=validate's JSON output with the
// same truncation metadata the natural-format output conveys, so a limited
// JSON response doesn't silently look like the complete issue list.
type statisticValidateJSONResponse struct {
	Issues      map[string][]homeassistant.StatisticValidationIssue `json:"issues"`
	TotalIDs    int                                                 `json:"total_ids"`
	ReturnedIDs int                                                 `json:"returned_ids"`
	Truncated   bool                                                `json:"truncated"`
}

// formatValidateJSON serializes the issues map as indented JSON.
func formatValidateJSON(
	issues map[string][]homeassistant.StatisticValidationIssue,
	totalIDs int,
	truncated bool,
) (string, error) {
	if issues == nil {
		issues = map[string][]homeassistant.StatisticValidationIssue{}
	}
	resp := statisticValidateJSONResponse{
		Issues:      issues,
		TotalIDs:    totalIDs,
		ReturnedIDs: len(issues),
		Truncated:   truncated,
	}
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

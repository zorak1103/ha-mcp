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

	// defaultStatisticListLimit caps action=list/validate output when the
	// caller omits limit, so the common no-argument call can't return every
	// row of a large recorder database in one response. Pass limit=0
	// explicitly for unlimited.
	defaultStatisticListLimit = 100
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
			"action=clear to permanently remove statistics data and metadata for given ids (set dry_run=true to preview which ids would actually be cleared, without purging).",
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
					Description: fmt.Sprintf(
						"Optional maximum number of statistic ids to return for action=list, "+
							"or statistic ids to include for action=validate; ignored for action=clear. "+
							"Defaults to %d when omitted; pass 0 explicitly for unlimited.",
						defaultStatisticListLimit),
				},
				"format": {
					Type:        "string",
					Enum:        []string{formatNatural, formatJSON},
					Description: "Output format for action=list/validate: 'natural' for readable text (default), 'json' for structured JSON; ignored for action=clear",
				},
				"dry_run": {
					Type: "boolean",
					Description: "For action=clear only: if true, preview which requested ids are present in the recorder database " +
						"(and would be cleared) vs. absent (and would be skipped), without purging anything.",
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

	metas, total, truncated := applyStatisticLimit(metas, resolveLimitArg(args))

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

	limited, summary := applyValidateLimit(issues, resolveLimitArg(args))

	format := getString(args, "format")
	if format == formatJSON {
		out, err := formatValidateJSON(limited, summary)
		if err != nil {
			return errorResult(fmt.Sprintf("Error formatting output: %v", err)), nil
		}
		return successResult(out), nil
	}

	return successResult(formatValidateNatural(limited, summary)), nil
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

	dryRun, _ := args["dry_run"].(bool)

	known, unknown, checked := resolveKnownStatisticIDs(ctx, client, statIDs)
	if !checked {
		// The pre-check itself failed - degrade rather than block clear (or
		// its preview) on a read-side error, but say so: without this, the
		// success message below would misreport verified-present ids as a
		// guess (issue W2).
		if dryRun {
			return successResult(fmt.Sprintf(
				"Would attempt to clear statistics for %d %s: %s\n"+
					"WARNING: could not verify which ids exist in the recorder database before this preview.",
				len(statIDs), pluralize(len(statIDs), "statistic id", "statistic ids"), strings.Join(statIDs, ", "),
			)), nil
		}
		if err := client.ClearStatistics(ctx, statIDs); err != nil {
			return errorResult(fmt.Sprintf("Error clearing statistics: %v", err)), nil
		}
		return successResult(fmt.Sprintf(
			"Cleared statistics for %d %s: %s\n"+
				"WARNING: could not verify which of these ids actually existed in the recorder database before clearing.",
			len(statIDs), pluralize(len(statIDs), "statistic id", "statistic ids"), strings.Join(statIDs, ", "),
		)), nil
	}

	if dryRun {
		return successResult(formatClearMessage("Would clear", known, unknown)), nil
	}

	if err := client.ClearStatistics(ctx, statIDs); err != nil {
		return errorResult(fmt.Sprintf("Error clearing statistics: %v", err)), nil
	}

	return successResult(formatClearMessage("Cleared", known, unknown)), nil
}

// resolveKnownStatisticIDs splits the requested ids into those the recorder
// currently lists and those it doesn't. recorder/clear_statistics silently
// filters its input to ids it actually knows (see the manage_statistics:clear
// gotcha in CLAUDE.md), so echoing the caller's request verbatim as
// "cleared" would misreport success for an id that was never present -
// issue W2. checked is false when the lookup itself failed, so the caller
// can degrade instead of blocking the clear on a read-side error.
func resolveKnownStatisticIDs(
	ctx context.Context,
	client homeassistant.Client,
	statIDs []string,
) (known, unknown []string, checked bool) {
	metas, err := client.ListStatisticIDs(ctx, "")
	if err != nil {
		return nil, nil, false
	}

	present := make(map[string]bool, len(metas))
	for _, m := range metas {
		present[m.StatisticID] = true
	}

	for _, id := range statIDs {
		if present[id] {
			known = append(known, id)
		} else {
			unknown = append(unknown, id)
		}
	}
	return known, unknown, true
}

// formatClearMessage renders action=clear's result (or, with verb "Would
// clear", its dry_run preview): which requested ids the recorder actually
// knows (and were cleared/would be cleared), and which were skipped because
// the recorder never had them.
func formatClearMessage(verb string, known, unknown []string) string {
	var sb strings.Builder
	if len(known) == 0 {
		fmt.Fprintf(&sb, "%s statistics for 0 statistic ids: none of the requested ids were present in the recorder database.", verb)
	} else {
		fmt.Fprintf(&sb, "%s statistics for %d %s: %s",
			verb, len(known), pluralize(len(known), "statistic id", "statistic ids"), strings.Join(known, ", "))
	}
	if len(unknown) > 0 {
		fmt.Fprintf(&sb, "\n%d %s not present in the recorder database (skipped): %s",
			len(unknown), pluralize(len(unknown), "id", "ids"), strings.Join(unknown, ", "))
	}
	return sb.String()
}

// maxClearStatisticIDs bounds one clear call's batch size, so a single
// request can't build an oversized WebSocket frame.
const maxClearStatisticIDs = 500

// statisticIDPattern matches HA's two statistic_id shapes: an entity-backed
// id ("domain.object_id") or an integration-provided external statistic id
// ("domain:key", e.g. "growatt:total_energy" or "17track:total_distance").
// The first character of each half may be a digit - HA's own
// VALID_STATISTIC_ID (recorder/statistics.py) allows [\da-z_] throughout,
// and integration domains like "17track" start with one.
var statisticIDPattern = regexp.MustCompile(`^[a-z0-9_]+[.:][a-z0-9_]+$`)

// parseStatisticIDsStrict extracts and strictly validates statistic_ids for
// the destructive clear action. Unlike parseStatisticIDsLenient (used by the
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

// resolveLimitArg returns the effective limit for list/validate actions.
// getInt treats an absent "limit" key the same as an explicit 0, which would
// make the common no-argument call return every row in the recorder
// database (issue C1) - a mature HA install can have thousands of statistic
// ids. Distinguishing "absent" from "explicitly 0" here lets 0 keep meaning
// "unlimited" only when the caller says so, while an omitted limit gets a
// safe default. validateLimitArg has already rejected a malformed explicit
// value by the time this runs.
func resolveLimitArg(args map[string]any) int {
	if _, ok := args["limit"]; !ok {
		return defaultStatisticListLimit
	}
	return getInt(args, "limit")
}

// applyStatisticLimit truncates the list client-side and reports the total.
func applyStatisticLimit(metas []homeassistant.StatisticMeta, limit int) ([]homeassistant.StatisticMeta, int, bool) {
	total := len(metas)
	if limit > 0 && total > limit {
		return metas[:limit], total, true
	}
	return metas, total, false
}

// validateSummary bundles the id/issue counts formatValidateNatural and
// formatValidateJSON need to describe a possibly-truncated validation
// result. Shown and total are tracked separately for both ids and issues so
// a truncated header can't conflate the two populations (issue W1: an id can
// carry more than one issue, so "N ids shown" and "M issues shown" are
// different numbers and summing issues over an already-truncated map gives
// the wrong total).
type validateSummary struct {
	ShownIDs    int
	TotalIDs    int
	ShownIssues int
	TotalIssues int
	Truncated   bool
}

// countIssues sums the issue lists across every statistic id in the map.
func countIssues(issues map[string][]homeassistant.StatisticValidationIssue) int {
	n := 0
	for _, list := range issues {
		n += len(list)
	}
	return n
}

// applyValidateLimit truncates the validation issue map to at most limit
// statistic ids, chosen by sorted id for determinism, and returns the
// truncated map plus a validateSummary describing both the shown and total
// id/issue counts. Mirrors applyStatisticLimit for action=validate.
func applyValidateLimit(
	issues map[string][]homeassistant.StatisticValidationIssue,
	limit int,
) (map[string][]homeassistant.StatisticValidationIssue, validateSummary) {
	totalIssues := countIssues(issues)
	if limit <= 0 || len(issues) <= limit {
		return issues, validateSummary{
			ShownIDs:    len(issues),
			TotalIDs:    len(issues),
			ShownIssues: totalIssues,
			TotalIssues: totalIssues,
		}
	}

	ids := make([]string, 0, len(issues))
	for id := range issues {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	limited := make(map[string][]homeassistant.StatisticValidationIssue, limit)
	for _, id := range ids[:limit] {
		limited[id] = issues[id]
	}
	return limited, validateSummary{
		ShownIDs:    len(limited),
		TotalIDs:    len(issues),
		ShownIssues: countIssues(limited),
		TotalIssues: totalIssues,
		Truncated:   true,
	}
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
			// FormatDetailValue (newline-safe, keeps parentheses), not
			// SanitizeDisplayName (strips parentheses to protect the
			// "Name (entity_id)" line shape, which doesn't apply to a unit
			// like "kWh (net)" - issue N6).
			unitPart = fmt.Sprintf(" — unit: %s", formatter.FormatDetailValue(unit))
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
func formatValidateNatural(issues map[string][]homeassistant.StatisticValidationIssue, summary validateSummary) string {
	if len(issues) == 0 {
		return "Statistics validation passed: no issues found in the recorder database."
	}

	var sb strings.Builder
	if summary.Truncated {
		fmt.Fprintf(&sb, "Statistics validation found %d %s across %d %s (showing %d %s / %d %s, ids may have multiple issues):\n",
			summary.TotalIssues, pluralize(summary.TotalIssues, "issue", "issues"),
			summary.TotalIDs, pluralize(summary.TotalIDs, "statistic id", "statistic ids"),
			summary.ShownIssues, pluralize(summary.ShownIssues, "issue", "issues"),
			summary.ShownIDs, pluralize(summary.ShownIDs, "id", "ids"))
	} else {
		fmt.Fprintf(&sb, "Statistics validation found %d %s across %d %s (ids may have multiple issues):\n",
			summary.TotalIssues, pluralize(summary.TotalIssues, "issue", "issues"),
			summary.TotalIDs, pluralize(summary.TotalIDs, "statistic id", "statistic ids"))
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
	Issues         map[string][]homeassistant.StatisticValidationIssue `json:"issues"`
	TotalIDs       int                                                 `json:"total_ids"`
	ReturnedIDs    int                                                 `json:"returned_ids"`
	TotalIssues    int                                                 `json:"total_issues"`
	ReturnedIssues int                                                 `json:"returned_issues"`
	Truncated      bool                                                `json:"truncated"`
}

// formatValidateJSON serializes the issues map as indented JSON.
func formatValidateJSON(
	issues map[string][]homeassistant.StatisticValidationIssue,
	summary validateSummary,
) (string, error) {
	if issues == nil {
		issues = map[string][]homeassistant.StatisticValidationIssue{}
	}
	resp := statisticValidateJSONResponse{
		Issues:         issues,
		TotalIDs:       summary.TotalIDs,
		ReturnedIDs:    summary.ShownIDs,
		TotalIssues:    summary.TotalIssues,
		ReturnedIssues: summary.ShownIssues,
		Truncated:      summary.Truncated,
	}
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

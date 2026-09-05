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
							"or statistic ids to include for action=validate; ignored for action=clear and when cursor is set. "+
							"Defaults to %d when omitted; pass 0 explicitly for unlimited (capped at %d). Ignored for action=clear.",
						defaultStatisticListLimit, DefaultMaxLimit),
				},
				"offset": {
					Type:        "integer",
					Description: "Optional number of statistic ids/issues to skip before applying limit, for action=list/validate; ignored when cursor is set.",
				},
				"cursor": {
					Type: "string",
					Description: "Optional pagination cursor from a previous action=list/validate response's next_cursor, " +
						"to fetch the next page. Overrides limit/offset.",
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

	params, err := buildStatisticListPaginationParams(args, statisticType)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	metas, err := client.ListStatisticIDs(ctx, statisticType)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing statistic ids: %v", err)), nil
	}

	page := ApplyPagination(metas, params)

	format := getString(args, "format")
	if format == formatJSON {
		out, err := formatStatisticListJSON(page)
		if err != nil {
			return errorResult(fmt.Sprintf("Error formatting output: %v", err)), nil
		}
		return successResult(out), nil
	}

	return successResult(formatStatisticListNatural(page.Items, page.Pagination.Total)), nil
}

// handleStatisticValidate returns the recorder validation issue list.
func (h *StatisticsHandlers) handleStatisticValidate(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	params, err := buildStatisticListPaginationParams(args, "")
	if err != nil {
		return errorResult(err.Error()), nil
	}

	issues, err := client.ValidateStatistics(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error validating statistics: %v", err)), nil
	}

	limited, summary := paginateValidateIssues(issues, params)

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
	// dry_run is the only thing standing between a preview and an
	// irreversible purge, and the MCP server does no schema validation of
	// its own (InputSchema is advisory) - a non-bool value must error, not
	// silently become false and clear for real (issue C1).
	dryRun, err := parseBoolArg(args, "dry_run")
	if err != nil {
		return errorResult(err.Error()), nil
	}

	statIDs, unrecognized, err := parseStatisticIDsStrict(args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	known, unknown, checked := resolveKnownStatisticIDs(ctx, client, statIDs)
	if !checked {
		// The pre-check itself failed - degrade rather than block clear (or
		// its preview) on a read-side error, but say so: without this, the
		// success message below would misreport verified-present ids as a
		// guess (issue W2).
		if dryRun {
			return successResult(fmt.Sprintf(
				"Would attempt to clear statistics for %d %s: %s\n"+
					"WARNING: could not verify which ids exist in the recorder database before this preview.%s",
				len(statIDs), pluralize(len(statIDs), "statistic id", "statistic ids"), strings.Join(statIDs, ", "),
				unrecognizedNote(unrecognized),
			)), nil
		}
		if err := client.ClearStatistics(ctx, statIDs); err != nil {
			return errorResult(fmt.Sprintf("Error clearing statistics: %v", err)), nil
		}
		return successResult(fmt.Sprintf(
			"Cleared statistics for %d %s: %s\n"+
				"WARNING: could not verify which of these ids actually existed in the recorder database before clearing.%s",
			len(statIDs), pluralize(len(statIDs), "statistic id", "statistic ids"), strings.Join(statIDs, ", "),
			unrecognizedNote(unrecognized),
		)), nil
	}

	if dryRun {
		return successResult(formatClearMessage("Would clear", known, unknown, unrecognized)), nil
	}

	if err := client.ClearStatistics(ctx, statIDs); err != nil {
		return errorResult(fmt.Sprintf("Error clearing statistics: %v", err)), nil
	}

	return successResult(formatClearMessage("Cleared", known, unknown, unrecognized)), nil
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
func formatClearMessage(verb string, known, unknown, unrecognized []string) string {
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
	sb.WriteString(unrecognizedNote(unrecognized))
	return sb.String()
}

// unrecognizedNote renders the ids parseStatisticIDsStrict flagged as not
// matching HA's statistic_id shape (issue W6) but forwarded anyway - unlike
// known/unknown, which come from statIDs and are therefore already
// regex-valid, an unrecognized id was never shape-checked, so it is
// sanitized before joining into a line-oriented message.
func unrecognizedNote(unrecognized []string) string {
	if len(unrecognized) == 0 {
		return ""
	}
	sanitized := make([]string, len(unrecognized))
	for i, id := range unrecognized {
		sanitized[i] = formatter.SanitizeDisplayName(id)
	}
	return fmt.Sprintf("\n%d %s not a recognizable statistic id (skipped): %s",
		len(unrecognized), pluralize(len(unrecognized), "id", "ids"), strings.Join(sanitized, ", "))
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

// parseStatisticIDsStrict extracts statistic_ids for the destructive clear
// action, splitting into ids that match HA's known statistic_id shape
// (valid) and ids that don't (unrecognized, issue W6). A shape mismatch
// alone no longer hard-fails the whole batch: an orphaned statistic can
// carry an id shape older or buggier than HA's own validator currently
// allows, and cleaning that up is exactly this tool's purpose.
// resolveKnownStatisticIDs is the real correctness gate downstream - it
// checks against the recorder's actual known ids, which is strictly
// stronger than a shape guess. Structural problems (wrong type, too many,
// empty, oversized) still hard-fail: they indicate a caller mistake, not a
// legitimate orphan. If every id turns out unrecognized, this still errors -
// there is nothing left to act on.
func parseStatisticIDsStrict(args map[string]any) (valid, unrecognized []string, err error) {
	statIDsRaw, ok := args["statistic_ids"]
	if !ok {
		return nil, nil, fmt.Errorf("statistic_ids is required")
	}

	statIDsSlice, ok := statIDsRaw.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("statistic_ids must be an array")
	}

	if len(statIDsSlice) == 0 {
		return nil, nil, fmt.Errorf("at least one statistic_id is required")
	}
	if len(statIDsSlice) > maxClearStatisticIDs {
		return nil, nil, fmt.Errorf("too many statistic_ids: got %d, max %d", len(statIDsSlice), maxClearStatisticIDs)
	}

	seen := make(map[string]bool, len(statIDsSlice))
	for i, raw := range statIDsSlice {
		s, ok := raw.(string)
		if !ok {
			return nil, nil, fmt.Errorf("statistic_ids[%d] must be a string, got %T", i, raw)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil, fmt.Errorf("statistic_ids[%d] must not be empty", i)
		}
		// maxScalarStringLen (helpers_arg_reader.go) bounds every other
		// caller-supplied string field in the codebase; statistic_ids had no
		// per-element length bound despite maxClearStatisticIDs's comment
		// implying one (issue W2 - that constant only bounds element count).
		if len(s) > maxScalarStringLen {
			return nil, nil, fmt.Errorf("statistic_ids[%d] exceeds maximum length of %d bytes", i, maxScalarStringLen)
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		if statisticIDPattern.MatchString(s) {
			valid = append(valid, s)
		} else {
			unrecognized = append(unrecognized, s)
		}
	}

	if len(valid) == 0 {
		return nil, nil, fmt.Errorf(
			"no recognizable statistic ids (expected domain.object_id or domain:key): %s",
			strings.Join(unrecognized, ", "))
	}

	return valid, unrecognized, nil
}

// parseBoolArg reads an optional boolean arg strictly. A non-bool value is
// an error, never a silent false: the MCP server performs no schema
// validation of its own (InputSchema is advisory metadata only), so nothing
// else stops a stringified "true" from reaching a handler. For action=clear
// this is the difference between a preview and an irreversible purge
// (issue C1) - it must fail closed, not open.
func parseBoolArg(args map[string]any, key string) (bool, error) {
	raw, ok := args[key]
	if !ok {
		return false, nil
	}
	b, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("invalid %s %v: must be a boolean", key, raw)
	}
	return b, nil
}

// validateLimitArg rejects a limit argument that isn't a finite, in-range,
// non-negative integer. Accepts both float64 (the normal JSON-RPC decoding)
// and int (a caller reaching the handler without a JSON round-trip, e.g. the
// integration test suite's CallTool). A value that is finite and integral
// but outside int range (e.g. 1e19) must still be rejected here: converting
// an out-of-range float to int is implementation-defined and can produce a
// negative result, which would silently defeat every limit/offset guard
// downstream and restore the unbounded output this validation exists to
// prevent (issue C2).
func validateLimitArg(args map[string]any) error {
	raw, ok := args["limit"]
	if !ok {
		return nil
	}
	var limitF float64
	switch v := raw.(type) {
	case float64:
		limitF = v
	case int:
		limitF = float64(v)
	default:
		return fmt.Errorf("invalid limit %v: must be a non-negative integer", raw)
	}
	if math.IsNaN(limitF) || math.IsInf(limitF, 0) || limitF < 0 || limitF != math.Trunc(limitF) {
		return fmt.Errorf("invalid limit %v: must be a non-negative integer", raw)
	}
	if limitF > math.MaxInt32 {
		return fmt.Errorf("invalid limit %v: must not exceed %d", raw, math.MaxInt32)
	}
	return nil
}

// buildStatisticListPaginationParams parses cursor/limit/offset for
// action=list/validate via the shared pagination helpers (ParsePaginationParams,
// pagination.go), converging manage_statistics onto the same DefaultMaxLimit
// clamp and cursor mechanism as every other paginated tool (issue W5) instead
// of a bespoke, unbounded-by-default limiter.
//
// ParsePaginationParams cannot be called as-is for two reasons:
//  1. validateLimitArg must reject a malformed limit before
//     ParsePaginationParams silently coerces it to NoLimit (unlimited).
//  2. ParsePaginationParams leaves an absent limit at its zero value, which
//     means NoLimit/unlimited - the same value it produces for an explicit
//     limit=0. Left alone, the common no-argument call would return every
//     row in the recorder database (issue C1). The default is therefore
//     applied here, and only when the caller supplied neither limit nor
//     cursor - a cursor already carries its own limit from the page that
//     produced it, and an explicit limit=0 must keep meaning "unlimited".
func buildStatisticListPaginationParams(args map[string]any, statisticType string) (PaginationParams, error) {
	if err := validateLimitArg(args); err != nil {
		return PaginationParams{}, err
	}

	filters := map[string]any{"statistic_type": statisticType}
	params, err := ParsePaginationParams(args, filters)
	if err != nil {
		return PaginationParams{}, err
	}

	if params.Cursor == "" {
		if _, hasLimit := args["limit"]; !hasLimit {
			params.Limit = defaultStatisticListLimit
		}
	}
	return params, nil
}

// validateSummary bundles the id/issue counts formatValidateNatural and
// formatValidateJSON need to describe a possibly-truncated validation
// result. Shown and total are tracked separately for both ids and issues so
// a truncated header can't conflate the two populations (issue W1: an id can
// carry more than one issue, so "N ids shown" and "M issues shown" are
// different numbers and summing issues over an already-truncated map gives
// the wrong total). Pagination carries the id-level cursor/offset/limit
// metadata for action=validate's JSON output.
type validateSummary struct {
	ShownIDs    int
	TotalIDs    int
	ShownIssues int
	TotalIssues int
	Truncated   bool
	Pagination  PaginationMetadata
}

// countIssues sums the issue lists across every statistic id in the map.
func countIssues(issues map[string][]homeassistant.StatisticValidationIssue) int {
	n := 0
	for _, list := range issues {
		n += len(list)
	}
	return n
}

// paginateValidateIssues applies the shared pagination helpers to the
// validation issue map, chosen by sorted id for determinism, and returns the
// paginated map plus a validateSummary describing both the shown and total
// id/issue counts. Ids (not issues) are the paginated unit, matching the
// pre-pagination behavior this replaces.
func paginateValidateIssues(
	issues map[string][]homeassistant.StatisticValidationIssue,
	params PaginationParams,
) (map[string][]homeassistant.StatisticValidationIssue, validateSummary) {
	totalIssues := countIssues(issues)

	ids := make([]string, 0, len(issues))
	for id := range issues {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	page := ApplyPagination(ids, params)

	limited := make(map[string][]homeassistant.StatisticValidationIssue, len(page.Items))
	for _, id := range page.Items {
		limited[id] = issues[id]
	}

	return limited, validateSummary{
		ShownIDs:    page.Pagination.Count,
		TotalIDs:    page.Pagination.Total,
		ShownIssues: countIssues(limited),
		TotalIssues: totalIssues,
		// HasMore, not Total>Count - see the identical comment in
		// formatStatisticListJSON for why this must mean "more exists
		// beyond this page", not "this page is smaller than the total".
		Truncated:  page.Pagination.HasMore,
		Pagination: page.Pagination,
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
	NextCursor   *string                       `json:"next_cursor,omitempty"`
}

// formatStatisticListJSON serializes a paginated page of metadata entries as
// indented JSON. NextCursor lets a caller page past a truncated response
// (issue W5) instead of the previous all-or-nothing limit=0 escape hatch.
func formatStatisticListJSON(page PaginatedResponse[homeassistant.StatisticMeta]) (string, error) {
	metas := page.Items
	if metas == nil {
		metas = []homeassistant.StatisticMeta{}
	}
	resp := statisticListJSONResponse{
		StatisticIDs: metas,
		Total:        page.Pagination.Total,
		Returned:     page.Pagination.Count,
		// HasMore, not Total>Count: on the true last page of a cursor-paged
		// walk, Total can still exceed this page's Count (e.g. total=3,
		// offset=2, limit=2 -> Count=1) even though nothing remains to
		// fetch. Truncated must mean "more exists beyond this page", which
		// is exactly what HasMore/NextCursor already answer.
		Truncated:  page.Pagination.HasMore,
		NextCursor: page.Pagination.NextCursor,
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
		// t is server-provided (HA's recorder/validate_statistics issue
		// type) and rendered into its own header line - sanitized the same
		// way the id on the neighboring line already is (issue W3), so it
		// can't forge an extra line. SanitizeDisplayName, not
		// FormatDetailValue: an issue type is a fixed HA-defined identifier
		// (e.g. "no_state", "units_changed") with no legitimate use for
		// parentheses, unlike a unit value such as "kWh (net)" - stripping
		// them is safe and matches how the id on the next line is handled.
		safeType := formatter.SanitizeDisplayName(t)
		if label, known := statisticIssueLabels[t]; known {
			fmt.Fprintf(&sb, "\n%s (%d %s) — %s:\n", safeType, len(ids), pluralize(len(ids), "id", "ids"), label)
		} else {
			fmt.Fprintf(&sb, "\n%s (%d %s):\n", safeType, len(ids), pluralize(len(ids), "id", "ids"))
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
		// k is a server-provided map key (HA's issue.Data) rendered
		// line-oriented alongside its value - sanitized the same way the
		// value already is, so a key can't forge an extra line (issue W3;
		// mirrors sentenceCaseKey's key sanitization in formatter/util.go).
		fmt.Fprintf(&sb, "      %s: %s\n", formatter.FormatDetailValue(k), formatter.FormatDetailValue(issue.Data[k]))
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
	NextCursor     *string                                             `json:"next_cursor,omitempty"`
}

// formatValidateJSON serializes the issues map as indented JSON. NextCursor
// (from summary.Pagination) lets a caller page past a truncated response
// (issue W5).
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
		NextCursor:     summary.Pagination.NextCursor,
	}
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

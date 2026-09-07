package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// maxUnknownLabelsListed caps how many unknown label values unknownLabelsMessage echoes back
// verbatim - a caller can supply an arbitrarily large labels array, and each unknown value is
// otherwise repeated in full in the refusal text.
const maxUnknownLabelsListed = 10

// maxLabelValueChars bounds each individual unknown label value echoed into the refusal
// message, mirroring the maxErrorValueLen precedent in helpers_arg_reader.go.
const maxLabelValueChars = 80

// knownLabelIDs returns the set of label_ids present in the label registry.
func knownLabelIDs(labels []homeassistant.LabelRegistryEntry) map[string]struct{} {
	known := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		known[label.LabelID] = struct{}{}
	}
	return known
}

// unknownLabelIDs returns the elements of supplied that are not a label_id in labels,
// preserving the order they were supplied in.
func unknownLabelIDs(labels []homeassistant.LabelRegistryEntry, supplied []string) []string {
	known := knownLabelIDs(labels)
	var unknown []string
	for _, id := range supplied {
		if _, ok := known[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	return unknown
}

// findLabelIDByName returns the label_id of the registry entry whose Name matches input
// case-insensitively in full. Unlike LabelHandlers.findLabelByInput (labels.go), this is a
// strict full-name match, not a substring match - a substring match would make this too
// lenient to validate against (e.g. "a" would "match" any label whose name contains an "a").
func findLabelIDByName(labels []homeassistant.LabelRegistryEntry, input string) (string, bool) {
	for _, label := range labels {
		if strings.EqualFold(label.Name, input) {
			return label.LabelID, true
		}
	}
	return "", false
}

// unknownLabelsMessage builds the refusal message for labelWriteGuardError, naming each
// unknown value and, when it matches an existing label's display Name, suggesting the
// label_id to use instead.
func unknownLabelsMessage(labels []homeassistant.LabelRegistryEntry, unknown []string) string {
	shown := unknown
	more := 0
	// `>` vs `>=` here is a proven-equivalent mutant (verified by hand): at
	// len(shown) == maxUnknownLabelsListed exactly, shown[:maxUnknownLabelsListed]
	// is a no-op slice of an already-that-length slice and more computes to 0
	// either way, so which branch runs makes no observable difference. No test
	// can kill it - both forms produce byte-identical output for every input.
	// See BoundedFieldList in internal/homeassistant/field_list.go for the same shape.
	if len(shown) > maxUnknownLabelsListed { //mutest:skip
		shown = shown[:maxUnknownLabelsListed]
		more = len(unknown) - maxUnknownLabelsListed
	}
	parts := make([]string, 0, len(shown))
	for _, id := range shown {
		part := fmt.Sprintf("%q", formatter.TruncateRunes(id, maxLabelValueChars))
		if suggestion, ok := findLabelIDByName(labels, id); ok {
			part += fmt.Sprintf(" (did you mean %q?)", suggestion)
		}
		parts = append(parts, part)
	}
	listed := strings.Join(parts, ", ")
	if more > 0 {
		listed += fmt.Sprintf(", … +%d more", more)
	}
	return fmt.Sprintf(
		"unknown label(s): %s - labels are referenced by label_id, and Home Assistant silently "+
			"drops ids that are not in the label registry instead of returning an error. List "+
			"existing ids with manage_label action=list, or create one with manage_label action=create.",
		listed,
	)
}

// labelGuardResult is labelWriteGuardError's outcome.
type labelGuardResult struct {
	// Refusal is non-nil when the write must not proceed; the caller should return it directly
	// instead of performing the write.
	Refusal *mcp.ToolsCallResult
	// Warning is non-empty when the write should proceed despite an unresolved validation
	// uncertainty (the label registry could not be read). Callers append it to their success
	// message rather than reporting an unqualified success.
	Warning string
}

// degradedLabelCheckWarning renders the warning appended to a write's success message when the
// label registry could not be consulted (or re-consulted) to validate caller-supplied labels.
func degradedLabelCheckWarning(err error) string {
	reason := "unknown error"
	if err != nil {
		reason = formatter.TruncateRunes(scanFailureLineBreaks.Replace(err.Error()), maxScanErrorReasonChars)
	}
	return fmt.Sprintf(
		"could not verify labels against the label registry (%s) - labels were written unchecked; "+
			"Home Assistant silently drops any id that turns out not to exist",
		reason,
	)
}

// labelWriteGuardError checks caller-supplied labels against the label registry before a
// create/update write and, if any are unknown, returns a refusal to short-circuit the write.
// Before refusing, it retries once against a freshly-fetched registry when the client exposes an
// InvalidateLabelRegistryCache() capability (CachedClient does; an uncached HybridClient does
// not) - the label registry can be cached for up to HA_CACHE_AREA_REG_TTL_MIN minutes, and a
// label created moments ago (in the HA UI, or by another client) must not be refused as
// "unknown" just because this process's cache hasn't caught up yet.
//
// Returns a zero labelGuardResult (proceed, no warning) when:
//   - supplied is empty (an empty array clears the field rather than adding labels)
//   - mode is arrayModeRemove (removed labels are subtracted from what HA already stored, so
//     the result is always a subset of an already-valid set; applyRemoveMode's contract is a
//     silent no-op for items not present)
//   - every supplied label exists in the registry (on the first pass, or after the retry)
//
// Returns a non-empty Warning (proceed, but flag the uncertainty) when the label registry fetch
// itself fails, on the initial pass or the retry - skipping the check only skips a validation,
// with no data-loss risk, but proceeding silently would recreate the invisible-no-op bug #242
// fixed in the first place, so the caller must surface this rather than reporting bare success.
func labelWriteGuardError(ctx context.Context, client homeassistant.Client, supplied []string, mode string) labelGuardResult {
	if len(supplied) == 0 || mode == arrayModeRemove {
		return labelGuardResult{}
	}

	labels, err := client.GetLabelRegistry(ctx)
	if err != nil {
		return labelGuardResult{Warning: degradedLabelCheckWarning(err)}
	}

	unknown := unknownLabelIDs(labels, supplied)
	if len(unknown) == 0 {
		return labelGuardResult{}
	}

	if refresher, ok := client.(interface{ InvalidateLabelRegistryCache() }); ok {
		refresher.InvalidateLabelRegistryCache()
		fresh, ferr := client.GetLabelRegistry(ctx)
		if ferr != nil {
			return labelGuardResult{Warning: degradedLabelCheckWarning(ferr)}
		}
		labels = fresh
		unknown = unknownLabelIDs(labels, supplied)
		if len(unknown) == 0 {
			return labelGuardResult{}
		}
	}

	return labelGuardResult{Refusal: errorResult(unknownLabelsMessage(labels, unknown))}
}

// parseLabelsArg extracts and strictly validates a "labels" argument, distinguishing "not
// provided" from "provided but malformed" so a malformed value can never fall through as a
// zero-length-but-non-nil []string and be mistaken for an intentional "clear all labels": a
// bare string (e.g. labels: "kitchen", a common LLM mistake) or an array containing a
// non-string element previously produced exactly that zero-length slice via getStringSlice,
// which bypasses labelWriteGuardError's own len(supplied)==0 exemption and - on update, where
// ws_client_impl.go sends params["labels"] for any non-nil config.Labels, even an empty one -
// silently wiped every existing label instead of refusing the write.
func parseLabelsArg(args map[string]any) (labels []string, hasLabels bool, refusal *mcp.ToolsCallResult) {
	val, exists := args["labels"]
	if !exists || val == nil {
		return nil, false, nil
	}
	arr, ok := val.([]any)
	if !ok {
		return nil, false, errorResult(fmt.Sprintf("labels must be an array of strings, got %T", val))
	}
	result := make([]string, 0, len(arr))
	for i, item := range arr {
		str, ok := item.(string)
		if !ok {
			return nil, false, errorResult(fmt.Sprintf("labels[%d] must be a string, got %T", i, item))
		}
		result = append(result, str)
	}
	return result, true, nil
}

// appendResultWarning appends a WARNING content block to a successful result. Appending a new
// block, rather than concatenating onto Content[0].Text, keeps a format=json result's single
// text block valid JSON - a caller requesting JSON gets the warning as a second block instead
// of trailing text that would corrupt the parse.
func appendResultWarning(res *mcp.ToolsCallResult, warning string) *mcp.ToolsCallResult {
	if res == nil || warning == "" || res.IsError {
		return res
	}
	res.Content = append(res.Content, mcp.NewTextContent("WARNING: "+warning))
	return res
}

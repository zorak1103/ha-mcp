package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

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
	parts := make([]string, 0, len(unknown))
	for _, id := range unknown {
		part := fmt.Sprintf("%q", id)
		if suggestion, ok := findLabelIDByName(labels, id); ok {
			part += fmt.Sprintf(" (did you mean %q?)", suggestion)
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf(
		"unknown label(s): %s - labels are referenced by label_id, and Home Assistant silently "+
			"drops ids that are not in the label registry instead of returning an error. List "+
			"existing ids with manage_label action=list, or create one with manage_label action=create.",
		strings.Join(parts, ", "),
	)
}

// labelWriteGuardError checks caller-supplied labels against the label registry before a
// create/update write and, if any are unknown, returns the refusal result to short-circuit
// the write. Returns nil when the write should proceed:
//   - supplied is empty (an empty array clears the field rather than adding labels)
//   - mode is arrayModeRemove (removed labels are subtracted from what HA already stored, so
//     the result is always a subset of an already-valid set; applyRemoveMode's contract is a
//     silent no-op for items not present)
//   - the label registry fetch itself fails (skipping only skips a validation - there is no
//     data-loss risk, same convention as configWriteGuardError)
//   - every supplied label exists in the registry
func labelWriteGuardError(ctx context.Context, client homeassistant.Client, supplied []string, mode string) *mcp.ToolsCallResult {
	if len(supplied) == 0 || mode == arrayModeRemove {
		return nil
	}
	labels, err := client.GetLabelRegistry(ctx)
	if err != nil {
		return nil
	}
	unknown := unknownLabelIDs(labels, supplied)
	if len(unknown) == 0 {
		return nil
	}
	return errorResult(unknownLabelsMessage(labels, unknown))
}

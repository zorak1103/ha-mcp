package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/zorak1103/ha-mcp/internal/jsonpatch"
)

// Patch operation type constants used for semantic op validation.
const (
	patchOpAdd     = "add"
	patchOpReplace = "replace"
	patchOpTest    = "test"
)

// SemanticOperation extends jsonpatch.Operation with optional property-based addressing.
// When Match is non-nil, the operation targets elements found by matching properties
// rather than by numeric index. Path must be empty when Match is set.
type SemanticOperation struct {
	jsonpatch.Operation
	// Match holds key-value pairs to identify target element(s) within Section.
	// When non-nil, Path must be empty (mutually exclusive).
	Match map[string]any
	// Section is the top-level array key to search (e.g. "triggers", "conditions",
	// "actions", "sequence", "views"). Required when Match is set.
	Section string
	// Field is the field within matched element(s) to target.
	// Required for replace/add/test with semantic match.
	// Must be empty for remove (removes the whole matched element).
	Field string
	// MatchIndex optionally selects a specific match (0-based) when multiple
	// elements match. If nil, all matching elements are updated.
	MatchIndex *int
}

// resolvedOp is a jsonpatch.Operation annotated with its original array index path
// for global descending-index sort of remove operations.
type resolvedOp struct {
	op      jsonpatch.Operation
	section string // non-empty only for semantic remove ops
	index   int    // array index, used for sorting semantic removes
}

// resolveSemanticOps converts semantic operations into standard jsonpatch.Operations
// by resolving match criteria against the config document.
// Standard operations (Match == nil) are passed through unchanged.
//
// NOTE: Semantic ops are resolved against the original document snapshot. If
// standard ops structurally modify a section (e.g. inserting/removing elements
// from the same array) before semantic ops that target that section, the resolved
// paths may be stale. To avoid this, do not mix standard structural ops and
// semantic ops on the same section within a single call.
//
// Semantic remove operations are returned in descending index order (per section)
// so that applying them sequentially does not shift unprocessed array indices.
func resolveSemanticOps(doc map[string]any, ops []SemanticOperation) ([]jsonpatch.Operation, error) {
	annotated := make([]resolvedOp, 0, len(ops))

	for i, op := range ops {
		if op.Match == nil {
			annotated = append(annotated, resolvedOp{op: op.Operation})
			continue
		}
		expanded, err := resolveOneSemanticOp(doc, op, i)
		if err != nil {
			return nil, err
		}
		annotated = append(annotated, expanded...)
	}

	// Sort all semantic remove ops in descending index order per section so that
	// removing multiple elements from the same array is safe regardless of how
	// many semantic ops contributed remove paths to that section.
	sortSemanticRemoves(annotated)

	result := make([]jsonpatch.Operation, 0, len(annotated))
	for _, r := range annotated {
		result = append(result, r.op)
	}
	return result, nil
}

// sortSemanticRemoves performs a stable sort of the annotated ops such that
// semantic remove ops with higher array indices come before lower ones within
// each section. Non-remove ops and standard ops keep their original order.
func sortSemanticRemoves(ops []resolvedOp) {
	// We want to reorder ONLY the remove ops for a given section, leaving all
	// other ops in place. We use a stable sort that preserves relative order
	// of non-remove entries.
	sort.SliceStable(ops, func(i, j int) bool {
		a, b := ops[i], ops[j]
		// Only reorder entries that are semantic removes of the same section
		if a.op.Op != arrayModeRemove || b.op.Op != arrayModeRemove {
			return false
		}
		if a.section == "" || b.section == "" || a.section != b.section {
			return false
		}
		// Higher index comes first (descending)
		return a.index > b.index
	})
}

// resolveOneSemanticOp resolves a single semantic operation into one or more
// annotated operations.
func resolveOneSemanticOp(doc map[string]any, op SemanticOperation, opIdx int) ([]resolvedOp, error) {
	if err := validateSemanticOp(op, opIdx); err != nil {
		return nil, err
	}

	rawSection, ok := doc[op.Section]
	if !ok {
		return nil, fmt.Errorf("section %q not found in config (operation %d)", op.Section, opIdx)
	}

	sectionSlice, ok := rawSection.([]any)
	if !ok {
		return nil, fmt.Errorf("section %q is not an array (operation %d)", op.Section, opIdx)
	}

	indices := findMatchingIndices(sectionSlice, op.Match)
	if len(indices) == 0 {
		return nil, fmt.Errorf("no elements in section %q match criteria %v (operation %d)", op.Section, op.Match, opIdx)
	}

	if op.MatchIndex != nil {
		idx := *op.MatchIndex
		if idx < 0 || idx >= len(indices) {
			return nil, fmt.Errorf("match_index %d out of range: only %d elements matched in section %q (operation %d)",
				idx, len(indices), op.Section, opIdx)
		}
		indices = []int{indices[idx]}
	}

	result := make([]resolvedOp, 0, len(indices))
	for _, idx := range indices {
		path := buildResolvedPath(op.Section, idx, op.Field)
		r := resolvedOp{
			op: jsonpatch.Operation{
				Op:    op.Op,
				Path:  path,
				Value: op.Value,
			},
		}
		// Annotate remove ops for later global sort
		if op.Op == arrayModeRemove {
			r.section = op.Section
			r.index = idx
		}
		result = append(result, r)
	}
	return result, nil
}

// validateSemanticOp checks semantic-specific constraints for an operation.
func validateSemanticOp(op SemanticOperation, idx int) error {
	if op.Path != "" {
		return fmt.Errorf("operation must specify either 'path' or 'match', not both (operation %d)", idx)
	}
	if len(op.Match) == 0 {
		return fmt.Errorf("'match' must not be empty (operation %d)", idx)
	}
	if op.Section == "" {
		return fmt.Errorf("'section' is required when using semantic match (operation %d)", idx)
	}
	if op.Op == "move" || op.Op == "copy" {
		return fmt.Errorf("semantic match does not support move/copy operations (operation %d)", idx)
	}
	if op.Op == arrayModeRemove && op.Field != "" {
		return fmt.Errorf("'field' must be empty for 'remove' operations with semantic match (operation %d)", idx)
	}
	needsField := op.Op == patchOpReplace || op.Op == patchOpAdd || op.Op == patchOpTest
	if needsField && op.Field == "" {
		return fmt.Errorf("'field' is required for %q operations with semantic match (operation %d)", op.Op, idx)
	}
	return nil
}

// matchesElement returns true if elem satisfies all match criteria.
// Comparison uses JSON-semantic equality to handle float64/int differences.
func matchesElement(elem, match map[string]any) bool {
	for k, wantVal := range match {
		gotVal, ok := elem[k]
		if !ok {
			return false
		}
		if !jsonValEqual(gotVal, wantVal) {
			return false
		}
	}
	return true
}

// findMatchingIndices returns the indices of elements in section that match all criteria.
func findMatchingIndices(section []any, match map[string]any) []int {
	var indices []int
	for i, item := range section {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if matchesElement(m, match) {
			indices = append(indices, i)
		}
	}
	return indices
}

// buildResolvedPath constructs a JSON Pointer for the matched element.
// If field is empty, targets the element itself (e.g., for remove).
// Otherwise targets a specific field within the element.
func buildResolvedPath(section string, index int, field string) string {
	if field == "" {
		return fmt.Sprintf("/%s/%d", section, index)
	}
	return fmt.Sprintf("/%s/%d/%s", section, index, field)
}

// jsonValEqual compares two values using JSON-semantic equality.
// This handles float64/int differences from Go's JSON unmarshaling.
func jsonValEqual(a, b any) bool {
	aj, err1 := json.Marshal(a)
	bj, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(aj, bj)
}

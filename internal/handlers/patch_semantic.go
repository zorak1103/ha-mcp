package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

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

// resolvedOp is a jsonpatch.Operation annotated with its resolved element path
// (without the trailing field segment) for path-based descending sort of
// semantic remove operations.
type resolvedOp struct {
	op               jsonpatch.Operation
	isSemanticRemove bool
	path             string // matched element path, set only for semantic remove ops
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
// semantic remove ops are ordered so that applying them sequentially never
// invalidates a not-yet-processed match: deeper (nested) paths are removed
// before their ancestors, and within the same array, higher indices are
// removed before lower ones. Non-remove ops and standard ops keep their
// original relative order.
func sortSemanticRemoves(ops []resolvedOp) {
	sort.SliceStable(ops, func(i, j int) bool {
		a, b := ops[i], ops[j]
		if !a.isSemanticRemove || !b.isSemanticRemove {
			return false
		}
		return removeBeforePaths(a.path, b.path)
	})
}

// removeBeforePaths reports whether the element at path a must be removed
// before the element at path b. Comparison walks both paths' RFC 6901
// segments left to right: at the first differing pair of numeric (array
// index) segments, the higher index is ordered first (safe removal of
// siblings). If one path is a prefix of the other, the longer (deeper,
// descendant) path is ordered first. Differing non-numeric segments (distinct
// object keys or sections) have no defined removal order and are left stable.
func removeBeforePaths(a, b string) bool {
	segsA, errA := jsonpatch.Segments(a)
	segsB, errB := jsonpatch.Segments(b)
	if errA != nil || errB != nil {
		return false
	}
	n := min(len(segsB), len(segsA))
	for i := range n {
		if segsA[i] == segsB[i] {
			continue
		}
		numA, errNA := strconv.Atoi(segsA[i])
		numB, errNB := strconv.Atoi(segsB[i])
		if errNA == nil && errNB == nil {
			return numA > numB
		}
		return false
	}
	return len(segsA) > len(segsB)
}

// resolveOneSemanticOp resolves a single semantic operation into one or more
// annotated operations. Matching recurses into nested arrays/objects within
// the named section (e.g. a dashboard card/chip nested several levels below
// "views"), so the resolved path reflects the actual depth of the match.
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

	paths := findMatchingPaths(sectionSlice, "/"+op.Section, op.Match)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no elements in section %q (including nested cards/actions) match criteria %v (operation %d)", op.Section, op.Match, opIdx)
	}

	if op.MatchIndex != nil {
		idx := *op.MatchIndex
		if idx < 0 || idx >= len(paths) {
			return nil, fmt.Errorf("match_index %d out of range: only %d elements matched in section %q (operation %d)",
				idx, len(paths), op.Section, opIdx)
		}
		paths = []string{paths[idx]}
	}

	result := make([]resolvedOp, 0, len(paths))
	for _, p := range paths {
		fullPath := p
		if op.Field != "" {
			fullPath = p + "/" + jsonpatch.EscapeSegment(op.Field)
		}
		r := resolvedOp{
			op: jsonpatch.Operation{
				Op:    op.Op,
				Path:  fullPath,
				Value: op.Value,
			},
		}
		// Annotate remove ops for later global sort
		if op.Op == arrayModeRemove {
			r.isSemanticRemove = true
			r.path = p
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

// findMatchingPaths recursively walks node looking for map elements that
// satisfy match, returning the RFC 6901 pointer path to each matching
// element, rooted at prefix. Arrays are visited by index and objects by
// sorted key for deterministic ordering (this determines match_index
// selection). Recursion continues into a matching element's children too, so
// a match at one depth does not prevent finding further nested matches
// beneath it — this is what allows "match"+"section" addressing to reach
// dashboard cards/chips nested arbitrarily deep below a top-level section
// (issue #144), while still finding direct top-level matches exactly as
// before.
func findMatchingPaths(node any, prefix string, match map[string]any) []string {
	var paths []string
	switch v := node.(type) {
	case map[string]any:
		if matchesElement(v, match) {
			paths = append(paths, prefix)
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			paths = append(paths, findMatchingPaths(v[k], prefix+"/"+jsonpatch.EscapeSegment(k), match)...)
		}
	case []any:
		for i, item := range v {
			paths = append(paths, findMatchingPaths(item, prefix+"/"+strconv.Itoa(i), match)...)
		}
	}
	return paths
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

package handlers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/zorak1103/ha-mcp/internal/jsonpatch"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// patchAction is the action constant for JSON Patch operations.
const patchAction = "patch"

// patchOperationsSchema returns the MCP JSONSchema for the operations parameter.
func patchOperationsSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "array",
		Description: "RFC 6902 JSON Patch operations to apply. Supports standard path-based addressing (path) and semantic property-based addressing (match + section + field).",
		Items: &mcp.JSONSchema{
			Type:        "object",
			Description: "A single JSON Patch operation. Use 'path' for standard RFC 6902 addressing or 'match'+'section'+'field' for semantic addressing by element properties.",
			Properties: map[string]mcp.JSONSchema{
				"op": {
					Type:        "string",
					Description: "Operation type",
					Enum:        []string{"add", "remove", "replace", "move", "copy", "test"},
				},
				"path": {
					Type:        "string",
					Description: "RFC 6901 JSON Pointer target path (e.g., '/triggers/2/entity_id'). Mutually exclusive with 'match'.",
				},
				"value": {
					Description: "Value to use for add, replace, or test operations",
				},
				"from": {
					Type:        "string",
					Description: "Source path for move and copy operations",
				},
				"match": {
					Type:        "object",
					Description: "Semantic match criteria: key-value pairs to find target element(s) in a section by their properties. Mutually exclusive with 'path'.",
				},
				"section": {
					Type:        "string",
					Description: "Array section to search when using 'match'. For automations: 'triggers', 'conditions', 'actions'. For scripts: 'sequence'. For scenes: 'entities'. For dashboards: 'views'. Matching recurses into nested arrays/objects within the section (e.g. a dashboard card/chip nested several levels below 'views', or a nested action block), not just the section's direct elements.",
				},
				"field": {
					Type:        "string",
					Description: "Field within matched element(s) to target (e.g., 'for', 'entity_id'). Required for replace/add/test with 'match'. Omit for remove to remove the whole element.",
				},
				"match_index": {
					Type:        "integer",
					Description: "0-based index to select a specific match when multiple elements satisfy 'match'. If omitted, all matching elements are updated.",
				},
			},
			Required: []string{"op"},
		},
	}
}

// toAnySlice normalises various slice representations to []any.
// Accepts []any (standard JSON decode), json.RawMessage (pre-encoded arguments),
// and any other slice type via JSON round-trip (e.g. []map[string]any).
func toAnySlice(v any) ([]any, error) {
	if s, ok := v.([]any); ok {
		return s, nil
	}
	// Pre-encoded JSON bytes from clients that defer argument parsing.
	if raw, ok := v.(json.RawMessage); ok {
		var result []any
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("operations must be a JSON array; got json.RawMessage that failed to parse: %w", err)
		}
		return result, nil
	}
	// Typed slices (e.g. []map[string]any) from internal callers or custom decoders.
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("operations must be a JSON array; got %T", v)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("operations must be a JSON array; could not normalise %T: %w", v, err)
	}
	var result []any
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("operations must be a JSON array; normalisation of %T failed: %w", v, err)
	}
	return result, nil
}

// parseOperations extracts and validates operations from MCP args.
// Returns nil result on success; returns an error result on validation failure.
func parseOperations(args map[string]any) ([]SemanticOperation, *mcp.ToolsCallResult) {
	raw, ok := args["operations"]
	if !ok {
		return nil, errorResult("operations is required for patch action")
	}

	rawSlice, err := toAnySlice(raw)
	if err != nil {
		return nil, errorResult(err.Error())
	}
	if len(rawSlice) == 0 {
		return nil, errorResult("operations must contain at least one operation")
	}

	ops := make([]SemanticOperation, 0, len(rawSlice))
	for i, rawOp := range rawSlice {
		op, err := parseOneOperation(rawOp, i)
		if err != nil {
			return nil, errorResult(err.Error())
		}
		ops = append(ops, op)
	}

	// Validate standard ops (semantic ops are validated during resolution)
	var standardOps []jsonpatch.Operation
	for _, op := range ops {
		if op.Match == nil {
			standardOps = append(standardOps, op.Operation)
		}
	}
	if len(standardOps) > 0 {
		if err := jsonpatch.Validate(standardOps); err != nil {
			return nil, errorResult(err.Error())
		}
	}

	return ops, nil
}

// parseOneOperation converts a raw map[string]any to a SemanticOperation.
func parseOneOperation(raw any, idx int) (SemanticOperation, error) {
	opMap, ok := raw.(map[string]any)
	if !ok {
		return SemanticOperation{}, fmt.Errorf("operation at index %d must be an object", idx)
	}

	op := SemanticOperation{
		Operation: jsonpatch.Operation{
			Op:   getString(opMap, "op"),
			Path: getString(opMap, "path"),
			From: getString(opMap, "from"),
		},
		Section: getString(opMap, "section"),
		Field:   getString(opMap, "field"),
	}

	// Preserve Value even if nil (null is valid JSON Patch value)
	if v, hasValue := opMap["value"]; hasValue {
		op.Value = v
	}

	if rawMatch, hasMatch := opMap["match"]; hasMatch {
		matchMap, ok := rawMatch.(map[string]any)
		if !ok {
			return SemanticOperation{}, fmt.Errorf("'match' at operation %d must be an object", idx)
		}
		op.Match = matchMap
	}

	if rawIdx, hasIdx := opMap["match_index"]; hasIdx {
		switch v := rawIdx.(type) {
		case float64:
			i := int(v)
			op.MatchIndex = &i
		case int:
			op.MatchIndex = &v
		}
	}

	return op, nil
}

// applyPatchWithSemantics resolves semantic operations and applies all operations
// to the config map atomically. Returns the patched map and the resolved concrete
// operations (semantic match+section addressing expanded to real JSON Pointer
// paths) on success — the resolved ops let callers render a compact dry-run diff
// (see dryRunPatchResult) without re-deriving affected paths.
func applyPatchWithSemantics(configMap map[string]any, ops []SemanticOperation) (map[string]any, []jsonpatch.Operation, error) {
	resolved, err := resolveSemanticOps(configMap, ops)
	if err != nil {
		return nil, nil, err
	}

	patchedAny, err := jsonpatch.Apply(configMap, resolved)
	if err != nil {
		return nil, nil, err
	}

	patchedMap, ok := patchedAny.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("patch result must be an object")
	}

	return patchedMap, resolved, nil
}

// configToMap converts a typed config struct to map[string]any via JSON round-trip.
func configToMap(config any) (map[string]any, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to deserialize config: %w", err)
	}
	return m, nil
}

// dryRunValueTruncateLen bounds each before/after value shown in a dry-run diff so
// that replacing or removing a large subtree (e.g. a dashboard card) cannot
// reproduce the token blow-up dry-run is meant to avoid.
const dryRunValueTruncateLen = 200

// dryRunPatchResult returns a compact preview of a patch: only the affected paths
// with their before/after values, not the entire patched config. original is the
// pre-patch config (used to look up "before" values); resolved are the concrete
// operations that were applied (semantic match+section ops already expanded to
// real JSON Pointer paths by applyPatchWithSemantics).
// dryRunPatchResult returns a compact preview of a patch: only the affected
// paths with their before/after values, not the entire patched config.
// original is the pre-patch config; resolved are the concrete operations that
// will be applied (semantic match+section ops already expanded to real JSON
// Pointer paths by applyPatchWithSemantics), in the same order they are
// applied to storage.
//
// Each op's diff is rendered against a working copy that is advanced op by
// op (via jsonpatch.Apply), rather than diffed against the pristine original
// for every op — this keeps "before" values correct when a later op targets
// a path an earlier op in the same call already touched.
func dryRunPatchResult(original map[string]any, resolved []jsonpatch.Operation, entityType, entityID string, opCount int) (*mcp.ToolsCallResult, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Dry-run result for %s '%s' (%d operations, NOT saved):\n", entityType, entityID, opCount)

	working := original
	for i, op := range resolved {
		next, err := renderDryRunOp(&b, working, op, i+1)
		if err != nil {
			return errorResult(fmt.Sprintf("error rendering dry-run diff: %v", err)), nil
		}
		working = next
	}

	return successResult(b.String()), nil
}

// renderDryRunOp writes op's numbered before/after diff line(s) to b and
// returns working advanced by applying op, so the next call observes this
// op's effect as its own "before" state.
func renderDryRunOp(b *strings.Builder, working map[string]any, op jsonpatch.Operation, num int) (map[string]any, error) {
	fmt.Fprintf(b, "%d. %s %s\n", num, op.Op, op.Path)
	before := dryRunFormatValue(working, op.Path)

	nextAny, err := jsonpatch.Apply(working, []jsonpatch.Operation{op})
	if err != nil {
		return nil, err
	}
	next, ok := nextAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("dry-run intermediate result must be an object")
	}

	switch op.Op {
	case "move", "copy":
		fmt.Fprintf(b, "   from: %s\n", op.From)
		fmt.Fprintf(b, "   after:  %s\n", dryRunFormatValue(next, op.Path))
	case arrayModeRemove:
		fmt.Fprintf(b, "   before: %s\n", before)
		b.WriteString("   after:  (removed)\n")
	default: // add, replace, test
		fmt.Fprintf(b, "   before: %s\n", before)
		fmt.Fprintf(b, "   after:  %s\n", truncateDiffValue(op.Value))
	}
	return next, nil
}

// dryRunFormatValue looks up path in doc and renders it truncated for diff display.
// A missing path (e.g. a field that doesn't exist yet before an "add") renders as
// "(absent)" rather than an error.
func dryRunFormatValue(doc map[string]any, path string) string {
	val, err := jsonpatch.Get(doc, path)
	if err != nil {
		return "(absent)"
	}
	return truncateDiffValue(val)
}

// truncateDiffValue renders v as compact single-line JSON, truncated to
// dryRunValueTruncateLen characters.
// truncateDiffValue renders v as compact single-line JSON for human display,
// truncated to dryRunValueTruncateLen characters. The truncated form is a
// preview, not guaranteed-valid JSON: the trailing "…" marker replaces
// whatever content followed, so a truncated object/array/string cannot be
// parsed back as-is. The cut point is pulled back to the nearest UTF-8 rune
// boundary so a multi-byte character (e.g. an umlaut in an HA entity or
// friendly name) is never split in half.
func truncateDiffValue(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	s := string(b)
	if len(s) > dryRunValueTruncateLen {
		return s[:runeSafeCutoff(s, dryRunValueTruncateLen)] + "…"
	}
	return s
}

// runeSafeCutoff returns the largest index <= n at which s can be sliced
// without splitting a multi-byte UTF-8 rune.
func runeSafeCutoff(s string, n int) int {
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return n
}

// dryRunSchema returns the MCP JSONSchema for the dry_run parameter.
func dryRunSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "boolean",
		Description: "If true, preview the patched result without saving (for patch action)",
	}
}

// mapToStruct converts map[string]any back to a typed struct via JSON round-trip.
func mapToStruct(data map[string]any, target any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize patched config: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return fmt.Errorf("failed to deserialize patched config: %w", err)
	}
	return nil
}

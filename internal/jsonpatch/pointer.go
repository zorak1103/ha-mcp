// Package jsonpatch implements RFC 6902 JSON Patch operations.
package jsonpatch

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Segments parses an RFC 6901 JSON Pointer string into path segments.
// An empty string represents the root document.
// Returns an error for non-empty paths that do not start with "/".
func Segments(path string) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q: must start with '/'", path)
	}
	parts := strings.Split(path[1:], "/")
	for i, p := range parts {
		parts[i] = unescapeSegment(p)
	}
	return parts, nil
}

// unescapeSegment applies RFC 6901 escape decoding:
// ~1 → /, ~0 → ~ (in that order, per spec).
func unescapeSegment(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// EscapeSegment applies RFC 6901 escape encoding to a single path segment:
// ~ → ~0, / → ~1 (in that order — inverse of unescapeSegment).
func EscapeSegment(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// Get retrieves the value at path in doc.
// An empty path returns the whole document.
func Get(doc any, path string) (any, error) {
	segs, err := Segments(path)
	if err != nil {
		return nil, err
	}
	return getAtSegs(doc, segs, path)
}

// getAtSegs recursively navigates doc following segments.
func getAtSegs(doc any, segs []string, origPath string) (any, error) {
	if len(segs) == 0 {
		return doc, nil
	}

	seg := segs[0]
	rest := segs[1:]

	switch d := doc.(type) {
	case map[string]any:
		val, ok := d[seg]
		if !ok {
			return nil, keyNotFoundError(seg, origPath, rest, sortedMapKeys(d))
		}
		return getAtSegs(val, rest, origPath)
	case []any:
		idx, err := parseIndex(seg, len(d), origPath, rest)
		if err != nil {
			return nil, err
		}
		return getAtSegs(d[idx], rest, origPath)
	default:
		return nil, fmt.Errorf("cannot index into %T with %q at %s", doc, seg, describeLocation(origPath, rest))
	}
}

// parseIndex parses a numeric array index segment and checks bounds [0, length).
// rest is the tail of segments remaining after seg, used to report the JSON Pointer
// prefix actually navigated when the index is invalid or out of bounds.
func parseIndex(seg string, length int, path string, rest []string) (int, error) {
	idx, err := strconv.Atoi(seg)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q at %s", seg, describeLocation(path, rest))
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("array index %d out of bounds (length %d) at %s", idx, length, describeLocation(path, rest))
	}
	return idx, nil
}

// parseInsertIndex parses an array index for insert operations.
// Allows idx == length (append to end). Returns the index. rest is the tail of
// segments remaining after seg, used to report the navigated prefix (see parseIndex).
func parseInsertIndex(seg string, length int, path string, rest []string) (int, error) {
	idx, err := strconv.Atoi(seg)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q at %s", seg, describeLocation(path, rest))
	}
	if idx < 0 || idx > length {
		return 0, fmt.Errorf("array index %d out of bounds (length %d) at %s", idx, length, describeLocation(path, rest))
	}
	return idx, nil
}

// sortedMapKeys returns the keys of a map in sorted order for deterministic error messages.
func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// navigatedPrefix reconstructs the JSON Pointer prefix successfully navigated
// before a failing segment. fullPath is the original operation path (unchanged
// throughout recursion); rest is the tail of segments still remaining after the
// failing segment. Returns "" when the failure occurred on the first segment
// (i.e. at the document root).
func navigatedPrefix(fullPath string, rest []string) string {
	segs, err := Segments(fullPath)
	if err != nil {
		return fullPath
	}
	consumed := len(segs) - len(rest) - 1
	if consumed <= 0 {
		return ""
	}
	var b strings.Builder
	for _, s := range segs[:consumed] {
		b.WriteByte('/')
		b.WriteString(EscapeSegment(s))
	}
	return b.String()
}

// actionBlockKeyHint returns structural guidance for Home Assistant action-block
// keywords that are commonly mistaken for children of a sibling key.
// Returns "" for keys with no known structural gotcha. This is a heuristic keyed
// only on the missing segment name — it assumes an HA automation/script document
// and may fire on an unrelated schema that happens to reuse one of these key names.
func actionBlockKeyHint(key string) string {
	switch key {
	case "then", "else":
		return `"then" and "else" are siblings of "if" at the same level, not nested inside the "if" array (e.g. /actions/0/then/0)`
	case "sequence":
		return `"sequence" is nested inside each "choose" option or inside "repeat", not at this level (e.g. /actions/0/choose/0/sequence/0)`
	case "default":
		return `"default" is a sibling of "choose" at the same level, not nested inside it (e.g. /actions/0/default/0)`
	default:
		return ""
	}
}

// isStructuralConfigKey reports whether key is one of automation/script's top-level structural
// keys. Shared by unloadedConfigHint's trigger check and its available-keys scan so the two
// lists cannot drift apart.
func isStructuralConfigKey(key string) bool {
	switch key {
	case "triggers", "conditions", "actions", "sequence":
		return true
	default:
		return false
	}
}

// unloadedConfigHint is a defensive hint for a root-level miss on one of automation/script's
// top-level structural keys, surfacing a clearer error than a bare "key not found" for the case
// where the patch's base config carries none of those keys at all - most plausibly because it
// was built from something other than a full config fetch. No handler in internal/handlers is
// currently known to hit this: every patch handler (scripts.go, automations.go, scenes.go)
// sources its base via a Get* call and already refuses a nil Config before patching, so this
// hint guards against a config that is present but empty of structure, not a specific reachable
// caller mistake. Fires only for a root-level miss, and only when NONE of the structural keys
// are present anywhere in the available set - a genuinely nested miss, or a root-level miss
// where some structural keys ARE present (more likely a typo in the path than an unloaded
// config), is left to actionBlockKeyHint or the bare error.
func unloadedConfigHint(seg, fullPath string, rest, available []string) string {
	if !isStructuralConfigKey(seg) {
		return ""
	}
	if navigatedPrefix(fullPath, rest) != "" {
		return "" // not a root-level miss
	}
	for _, k := range available {
		if isStructuralConfigKey(k) {
			return "" // at least one structural key is present - a genuine path error, not an unloaded config
		}
	}
	return "the base config for this patch is missing all of triggers/actions/conditions/sequence at the root - " +
		"if this is an automation or script, list operations don't populate Config; fetch the full config via 'get' first"
}

// describeLocation renders a navigation-failure location: the JSON Pointer prefix
// successfully navigated before the failing segment, or a "document root" descriptor
// when the failure occurred on the first segment. Shared by all navigation-failure
// errors (key-not-found, array-index, type-mismatch) so the engine reports one
// consistent location format regardless of failure kind (a navigation-location
// fix identified during adversarial review).
func describeLocation(fullPath string, rest []string) string {
	if loc := navigatedPrefix(fullPath, rest); loc != "" {
		return fmt.Sprintf("%q", loc)
	}
	return fmt.Sprintf("document root (requested %q)", fullPath)
}

// keyNotFoundError builds the "key not found" error for map navigation failures,
// reporting the prefix actually navigated (not the full submitted path) and
// appending a structural hint when the missing key is a commonly-confused
// Home Assistant action-block keyword (see actionBlockKeyHint).
func keyNotFoundError(seg, fullPath string, rest, available []string) error {
	err := fmt.Errorf("key %q not found at %s (available keys: %v)", seg, describeLocation(fullPath, rest), available)
	if hint := unloadedConfigHint(seg, fullPath, rest, available); hint != "" {
		return fmt.Errorf("%w; %s", err, hint)
	}
	if hint := actionBlockKeyHint(seg); hint != "" {
		err = fmt.Errorf("%w; %s", err, hint)
	}
	return err
}

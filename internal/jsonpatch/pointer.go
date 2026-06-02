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
			return nil, fmt.Errorf("key %q not found at path %q (available keys: %v)", seg, origPath, sortedMapKeys(d))
		}
		return getAtSegs(val, rest, origPath)
	case []any:
		idx, err := parseIndex(seg, len(d), origPath)
		if err != nil {
			return nil, err
		}
		return getAtSegs(d[idx], rest, origPath)
	default:
		return nil, fmt.Errorf("cannot index into %T with %q at path %q", doc, seg, origPath)
	}
}

// parseIndex parses a numeric array index segment and checks bounds [0, length).
func parseIndex(seg string, length int, path string) (int, error) {
	idx, err := strconv.Atoi(seg)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q at path %q", seg, path)
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("array index %d out of bounds (length %d) at path %q", idx, length, path)
	}
	return idx, nil
}

// parseInsertIndex parses an array index for insert operations.
// Allows idx == length (append to end). Returns the index.
func parseInsertIndex(seg string, length int, path string) (int, error) {
	idx, err := strconv.Atoi(seg)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q at path %q", seg, path)
	}
	if idx < 0 || idx > length {
		return 0, fmt.Errorf("array index %d out of bounds (length %d) at path %q", idx, length, path)
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

package jsonpatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Operation represents a single RFC 6902 JSON Patch operation.
type Operation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
	From  string `json:"from,omitempty"`
}

// Apply applies RFC 6902 operations to doc atomically.
// If any operation fails, the original document is returned unchanged.
func Apply(doc any, ops []Operation) (any, error) {
	if err := Validate(ops); err != nil {
		return doc, err
	}

	clone, err := deepClone(doc)
	if err != nil {
		return doc, fmt.Errorf("failed to clone document: %w", err)
	}

	for i, op := range ops {
		clone, err = applyOne(clone, op, i)
		if err != nil {
			return doc, err
		}
	}
	return clone, nil
}

// deepClone creates a deep copy via JSON round-trip.
func deepClone(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// applyOne dispatches a single operation.
func applyOne(doc any, op Operation, idx int) (any, error) {
	var err error
	var result any
	switch op.Op {
	case "add":
		result, err = applyAdd(doc, op.Path, op.Value, idx)
	case "remove":
		result, err = applyRemove(doc, op.Path, idx)
	case "replace":
		result, err = applyReplace(doc, op.Path, op.Value, idx)
	case "move":
		result, err = applyMove(doc, op.From, op.Path, idx)
	case "copy":
		result, err = applyCopy(doc, op.From, op.Path, idx)
	case "test":
		result, err = doc, applyTest(doc, op.Path, op.Value, idx)
	default:
		return doc, fmt.Errorf("unknown operation %q (operation %d)", op.Op, idx)
	}
	return result, err
}

// applyAdd implements the RFC 6902 "add" operation.
func applyAdd(doc any, path string, value any, opIdx int) (any, error) {
	segs, err := Segments(path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	result, err := setAtPath(doc, segs, value, true, path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	return result, nil
}

// applyRemove implements the RFC 6902 "remove" operation.
func applyRemove(doc any, path string, opIdx int) (any, error) {
	segs, err := Segments(path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	result, err := removeAtPath(doc, segs, path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	return result, nil
}

// applyReplace implements the RFC 6902 "replace" operation.
// Unlike add, replace requires the key to already exist.
func applyReplace(doc any, path string, value any, opIdx int) (any, error) {
	segs, err := Segments(path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	// Verify the target exists before replacing
	if len(segs) > 0 {
		if _, getErr := Get(doc, path); getErr != nil {
			return doc, opError(opIdx, getErr)
		}
	}
	result, err := setAtPath(doc, segs, value, false, path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	return result, nil
}

// applyMove implements the RFC 6902 "move" operation.
// Per RFC 6902 §4.4, path must not be a proper prefix of from.
func applyMove(doc any, from, path string, opIdx int) (any, error) {
	if from != path && strings.HasPrefix(path+"/", from+"/") {
		return doc, opError(opIdx, fmt.Errorf("move: path %q is a child of from %q (RFC 6902 §4.4)", path, from))
	}

	val, err := Get(doc, from)
	if err != nil {
		return doc, opError(opIdx, fmt.Errorf("from path: %w", err))
	}

	fromSegs, err := Segments(from)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	intermediate, err := removeAtPath(doc, fromSegs, from)
	if err != nil {
		return doc, opError(opIdx, err)
	}

	toSegs, err := Segments(path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	result, err := setAtPath(intermediate, toSegs, val, true, path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	return result, nil
}

// applyCopy implements the RFC 6902 "copy" operation.
func applyCopy(doc any, from, path string, opIdx int) (any, error) {
	val, err := Get(doc, from)
	if err != nil {
		return doc, opError(opIdx, fmt.Errorf("from path: %w", err))
	}

	toSegs, err := Segments(path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	result, err := setAtPath(doc, toSegs, val, true, path)
	if err != nil {
		return doc, opError(opIdx, err)
	}
	return result, nil
}

// applyTest implements the RFC 6902 "test" operation.
// Uses JSON-semantic equality to handle float64 vs int comparisons (RFC 7159).
func applyTest(doc any, path string, value any, opIdx int) error {
	actual, err := Get(doc, path)
	if err != nil {
		return opError(opIdx, err)
	}
	if !jsonEqual(actual, value) {
		return fmt.Errorf("test failed at %q: expected %v, got %v (operation %d)", path, value, actual, opIdx)
	}
	return nil
}

// jsonEqual compares two values using JSON-semantic equality.
// Both values are marshaled to JSON and the representations are compared.
func jsonEqual(a, b any) bool {
	aj, err1 := json.Marshal(a)
	bj, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return reflect.DeepEqual(a, b)
	}
	return bytes.Equal(aj, bj)
}

// setAtPath sets value at segs in doc, returning the potentially-new root.
// insert=true uses array insertion semantics; insert=false replaces.
func setAtPath(doc any, segs []string, value any, insert bool, path string) (any, error) {
	if len(segs) == 0 {
		return value, nil
	}
	seg := segs[0]
	rest := segs[1:]

	switch d := doc.(type) {
	case map[string]any:
		return setInMap(d, seg, rest, value, insert, path)
	case []any:
		return setInSlice(d, seg, rest, value, insert, path)
	default:
		return nil, fmt.Errorf("cannot navigate into %T at path %q", doc, path)
	}
}

// setInMap handles setting a value in a map.
func setInMap(d map[string]any, seg string, rest []string, value any, insert bool, path string) (any, error) {
	if len(rest) == 0 {
		d[seg] = value
		return d, nil
	}
	child, ok := d[seg]
	if !ok {
		return nil, keyNotFoundError(seg, path, rest, sortedMapKeys(d))
	}
	newChild, err := setAtPath(child, rest, value, insert, path)
	if err != nil {
		return nil, err
	}
	d[seg] = newChild
	return d, nil
}

// setInSlice handles setting a value in a slice.
func setInSlice(d []any, seg string, rest []string, value any, insert bool, path string) (any, error) {
	if seg == "-" {
		return setInSliceAtEnd(d, rest, value, insert, path)
	}

	if len(rest) == 0 && insert {
		idx, err := parseInsertIndex(seg, len(d), path)
		if err != nil {
			return nil, err
		}
		return insertAt(d, idx, value), nil
	}

	idx, err := parseIndex(seg, len(d), path)
	if err != nil {
		return nil, err
	}

	if len(rest) == 0 {
		d[idx] = value
		return d, nil
	}

	newChild, err := setAtPath(d[idx], rest, value, insert, path)
	if err != nil {
		return nil, err
	}
	d[idx] = newChild
	return d, nil
}

// setInSliceAtEnd handles the "-" append case for arrays.
func setInSliceAtEnd(d []any, rest []string, value any, insert bool, path string) (any, error) {
	if len(rest) > 0 {
		return nil, fmt.Errorf("'-' must be the last path segment at path %q", path)
	}
	if !insert {
		return nil, fmt.Errorf("'-' is only valid for add operations at path %q", path)
	}
	return append(d, value), nil
}

// removeAtPath removes the value at segs from doc, returning the new root.
func removeAtPath(doc any, segs []string, path string) (any, error) {
	if len(segs) == 0 {
		return nil, fmt.Errorf("cannot remove root document at path %q", path)
	}
	seg := segs[0]
	rest := segs[1:]

	switch d := doc.(type) {
	case map[string]any:
		return removeFromMap(d, seg, rest, path)
	case []any:
		return removeFromSlice(d, seg, rest, path)
	default:
		return nil, fmt.Errorf("cannot navigate into %T at path %q", doc, path)
	}
}

// removeFromMap handles removing from a map.
func removeFromMap(d map[string]any, seg string, rest []string, path string) (any, error) {
	if _, ok := d[seg]; !ok {
		return nil, keyNotFoundError(seg, path, rest, sortedMapKeys(d))
	}
	if len(rest) == 0 {
		delete(d, seg)
		return d, nil
	}
	newChild, err := removeAtPath(d[seg], rest, path)
	if err != nil {
		return nil, err
	}
	d[seg] = newChild
	return d, nil
}

// removeFromSlice handles removing from a slice.
func removeFromSlice(d []any, seg string, rest []string, path string) (any, error) {
	idx, err := parseIndex(seg, len(d), path)
	if err != nil {
		return nil, err
	}
	if len(rest) == 0 {
		return append(d[:idx], d[idx+1:]...), nil
	}
	newChild, err := removeAtPath(d[idx], rest, path)
	if err != nil {
		return nil, err
	}
	d[idx] = newChild
	return d, nil
}

// insertAt inserts value into slice s at index idx, shifting elements right.
func insertAt(s []any, idx int, v any) []any {
	s = append(s, nil)
	copy(s[idx+1:], s[idx:])
	s[idx] = v
	return s
}

// opError wraps an error with an operation index suffix.
func opError(opIdx int, err error) error {
	return fmt.Errorf("%w (operation %d)", err, opIdx)
}

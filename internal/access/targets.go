package access

import (
	"maps"
	"slices"
	"strings"
)

// TargetSpec is what a service call addresses. All is set when any target key
// holds the literal "all". Malformed is set when a target key is present with a
// value of an unusable type (number, map, null, list with non-strings, ...); the
// caller must refuse such a call because its objects cannot be decided.
type TargetSpec struct {
	EntityIDs []string
	DeviceIDs []string
	AreaIDs   []string
	FloorIDs  []string
	LabelIDs  []string
	All       bool
	Malformed bool
}

const (
	targetMatchAll  = "all"
	targetMatchNone = "none"
	targetNestedKey = "target"
)

// ParseTarget extracts the call target from service data. Top-level keys and the
// keys of a nested "target" map are merged. Ids are trimmed and lower-cased;
// each key accepts a string (optionally comma-separated), a list of strings, or
// the literals "all" (sets All) and "none" (matches nothing).
func ParseTarget(data map[string]any) TargetSpec {
	var t TargetSpec
	t.merge(data)
	if raw, ok := data[targetNestedKey]; ok {
		nested, isMap := raw.(map[string]any)
		if !isMap {
			t.Malformed = true
		} else {
			t.merge(nested)
		}
	}
	return t
}

func (t *TargetSpec) merge(m map[string]any) {
	t.addKey(m, "entity_id", &t.EntityIDs)
	t.addKey(m, "device_id", &t.DeviceIDs)
	t.addKey(m, "area_id", &t.AreaIDs)
	t.addKey(m, "floor_id", &t.FloorIDs)
	t.addKey(m, "label_id", &t.LabelIDs)
}

func (t *TargetSpec) addKey(m map[string]any, key string, dst *[]string) {
	raw, present := m[key]
	if !present {
		return
	}
	ids, ok := idList(raw)
	if !ok {
		t.Malformed = true
	}
	for _, id := range ids {
		switch id {
		case targetMatchAll:
			t.All = true
		case targetMatchNone:
		default:
			if !slices.Contains(*dst, id) {
				*dst = append(*dst, id)
			}
		}
	}
}

// idList flattens a target value into normalised, non-empty ids. ok is false for
// an unsupported type or element; the ids found before that are still returned.
func idList(v any) ([]string, bool) {
	switch x := v.(type) {
	case string:
		return splitIDs(x), true
	case []string:
		var out []string
		for _, s := range x {
			out = append(out, splitIDs(s)...)
		}
		return out, true
	case []any:
		return idsFromAnyList(x)
	default:
		return nil, false
	}
}

func idsFromAnyList(list []any) ([]string, bool) {
	var out []string
	ok := true
	for _, e := range list {
		s, isStr := e.(string)
		if !isStr {
			ok = false
			continue
		}
		out = append(out, splitIDs(s)...)
	}
	return out, ok
}

// EntityIDsFromData extracts the entity ids a call names under the given
// call-data keys (see ServiceClass.EntityDataKeys). A value may be a string,
// comma string, list of strings, or a map whose keys are the entity ids (the
// values are ignored). Ids are trimmed, lower-cased and de-duplicated, in key
// order (ids from a map sorted). malformed is true when a present key holds an
// unsupported type or element (including null); ids found elsewhere are still
// returned and the caller must refuse the call.
func EntityIDsFromData(data map[string]any, keys []string) (ids []string, malformed bool) {
	for _, key := range keys {
		raw, present := data[key]
		if !present {
			continue
		}
		found, ok := entityDataIDs(raw)
		if !ok {
			malformed = true
		}
		for _, id := range found {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return ids, malformed
}

func entityDataIDs(v any) ([]string, bool) {
	m, isMap := v.(map[string]any)
	if !isMap {
		return idList(v)
	}
	var out []string
	for k := range maps.Keys(m) {
		out = append(out, splitIDs(k)...)
	}
	slices.Sort(out) // map order is random; sort the normalised ids for determinism
	return out, true
}

// splitIDs splits a comma-separated string into trimmed, lower-cased, non-empty ids.
func splitIDs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if id := strings.ToLower(strings.TrimSpace(part)); id != "" {
			out = append(out, id)
		}
	}
	return out
}

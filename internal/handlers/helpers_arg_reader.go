// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// argReader accumulates errors while reading typed values out of a
// manage_helper args map into a config map, so a value the schema declared
// but the receiving HA field can't use fails loudly instead of being
// silently dropped (the root cause of both the input_number "initial" bug
// and the filter window_size gap - see CLAUDE.md's schema<->builder type
// contract gotcha). Every read is one statement; call err() once all
// fields for a builder have been read.
//
// Three input values are always a silent skip, never an error, on every
// method below: the key absent from args, an explicit JSON null, and an
// empty string. Absent/null must stay silent because
// mergeCurrentHelperState relies on a caller's null never overwriting a
// merged value (helpers_consolidated.go). Empty string is the de-facto
// "unset" spelling MCP clients emit for an optional field - a defined value
// semantic, not a type failure.
//
// Method naming: str/strAs/strID/num are short because they're the readers
// used at the overwhelming majority of call sites; integer/boolean/anySlice
// are spelled out because an abbreviation (int/bool) would collide with a
// Go built-in type name at the call site, which reads worse than it saves.
type argReader struct {
	config map[string]any
	args   map[string]any
	errs   []error
}

func newArgReader(config, args map[string]any) *argReader {
	return &argReader{config: config, args: args}
}

// err joins every failure recorded so far, or nil if none.
func (r *argReader) err() error {
	return errors.Join(r.errs...)
}

func (r *argReader) fail(key, want string, got any) {
	r.errs = append(r.errs, argTypeError(key, want, got))
}

// failElem is fail for one element of an array-typed field, so a bad
// element in a large array is reported by its index and its own value -
// not by dumping the entire surrounding array into the error message.
func (r *argReader) failElem(key string, index int, want string, got any) {
	r.errs = append(r.errs, argTypeError(fmt.Sprintf("%s[%d]", key, index), want, got))
}

// argTypeError is the single message constructor for every reader failure,
// so wording stays consistent across all ~170 call sites. The value is
// rendered through truncateArgValue so one oversized or malicious field
// (a large array, a long pasted string/token) can't blow up the size of the
// error returned to the MCP client, and callers echoing a slice/map failure
// don't leak the entire container's contents into the response and logs.
func argTypeError(key, want string, got any) error {
	return fmt.Errorf("invalid value for %q: expected %s, got %T (%s)", key, want, got, truncateArgValue(got))
}

// maxErrorValueLen bounds how much of a rejected value's rendered form is
// echoed back in an error message.
const maxErrorValueLen = 80

// truncateArgValue renders v for inclusion in an error message, cut to
// maxErrorValueLen runes so an oversized container (e.g. a 1000-element
// array with one bad element) doesn't dominate the response.
//
// Slices and maps are summarized by kind and length instead of being run
// through fmt.Sprintf: %v on a container recurses into every element before
// truncation ever gets a chance to run, so a large array/map sent to a
// scalar-typed field would pay the full render cost regardless of
// maxErrorValueLen - defeating the exact cost bound this function exists to
// provide. Only these two container types reach this function today (raw()
// is the only reader that accepts a bare map and already length-checks it
// via checkMapLen; every array-typed reader length-checks via
// checkArrayLen), but a type switch here can't miss a large scalar since
// scalars have no "size" to summarize away.
func truncateArgValue(v any) string {
	switch val := v.(type) {
	case []any:
		return fmt.Sprintf("array with %d elements", len(val))
	case map[string]any:
		return fmt.Sprintf("object with %d keys", len(val))
	}
	s := fmt.Sprintf("%v", v)
	r := []rune(s)
	if len(r) <= maxErrorValueLen {
		return s
	}
	return string(r[:maxErrorValueLen]) + "…"
}

func isSkippable(v any) bool {
	if v == nil {
		return true
	}
	s, isString := v.(string)
	return isString && s == ""
}

// maxArrayElements bounds every array-typed field the argReader accepts.
// By the time args reaches this reader, the JSON decoder has already
// materialized the full []any in memory - this cap does not prevent that
// allocation. What it bounds is everything downstream of it: the output
// slice this package allocates from the input's length, and the size of
// the config payload eventually sent to Home Assistant. No real helper
// field plausibly needs more than a few hundred elements.
const maxArrayElements = 1000

// checkArrayLen records a failure and returns false when arr exceeds
// maxArrayElements, so callers can bail out before allocating from its
// length.
func (r *argReader) checkArrayLen(key string, arr []any) bool {
	if len(arr) > maxArrayElements {
		r.errs = append(r.errs, fmt.Errorf(
			"invalid value for %q: array has %d elements, exceeds maximum of %d", key, len(arr), maxArrayElements,
		))
		return false
	}
	return true
}

// checkMapLen records a failure and returns false when m exceeds
// maxArrayElements entries. raw() is the only reader that accepts a map
// (e.g. filter's window_size as a {"hours":.,"minutes":.,"seconds":.}
// object) verbatim; without this it would be the one field type exempt
// from the size bound every array-typed field already gets.
func (r *argReader) checkMapLen(key string, m map[string]any) bool {
	if len(m) > maxArrayElements {
		r.errs = append(r.errs, fmt.Errorf(
			"invalid value for %q: object has %d keys, exceeds maximum of %d", key, len(m), maxArrayElements,
		))
		return false
	}
	return true
}

// maxNumericStringLen bounds a string this reader will attempt to parse as a
// number, whole number, or boolean (num/integer/boolean). A real value in
// this position - a numeric literal, a boolean spelling - is always a
// handful of characters; anything past this is either malformed input or an
// attempt to force strconv to scan a large buffer for no legitimate reason.
// Checked before any parsing is attempted, unlike the array/map bounds
// above which only bound what happens after a successful type match.
const maxNumericStringLen = 64

// maxScalarStringLen bounds any string field accepted verbatim or coerced
// through str/strAs/strSlice (e.g. name, icon, a template's state or
// availability expression). Generous enough for any real Home Assistant
// template or identifier while still closing off a multi-megabyte string in
// a single field, which only costs memory and outbound bandwidth to Home
// Assistant with no legitimate use.
const maxScalarStringLen = 65536

// checkStringLen records a failure and returns false when s exceeds max
// bytes, so callers can bail out before an expensive parse/lower-case pass
// or before storing the value verbatim.
//
// Every method that can produce a string-shaped leaf value - at any nesting
// level - must call this before storing/parsing it; there is no single
// place in this file where a length check would cover every case, so a new
// coercion method must pick the shape matching its own value handling
// rather than skip the check because none of the existing examples matches
// exactly: str()/strAs() check the string case inside their type switch;
// num()/integer()/boolean() check up front, before parsing is attempted,
// since a numeric/boolean string is always short; checkFlatMapValues checks
// each string value found one level inside a map (used by raw() and
// anySlice() for values whose shape they can't otherwise validate). raw()
// and anySlice() (its string-element case) both also call this directly for
// a value at their own top level.
func (r *argReader) checkStringLen(key, s string, maxLen int) bool {
	if len(s) > maxLen {
		r.errs = append(r.errs, fmt.Errorf(
			"invalid value for %q: string has %d bytes, exceeds maximum of %d", key, len(s), maxLen,
		))
		return false
	}
	return true
}

// checkFlatMapValues records a failure and returns false if any value in m
// is itself a nested container (map or array) or an oversized string.
// Enforced one level deep - e.g. filter's window_size as a
// {"hours":.,"minutes":.,"seconds":.} duration dict, or a schedule day's
// {"from":.,"to":.} time block are always flat, every value a plain number
// or bounded string. Shared by raw() and anySlice(), the two readers that
// accept a value shape this package can't fully validate up front and must
// instead just bound, so a caller can't smuggle an arbitrarily large or
// deeply nested payload through under the guise of a legitimate flat value.
func (r *argReader) checkFlatMapValues(key string, m map[string]any) bool {
	for elemKey, elem := range m {
		switch e := elem.(type) {
		case map[string]any, []any:
			r.fail(key, fmt.Sprintf("a flat object (key %q must not itself be a nested object or array)", elemKey), elem)
			return false
		case string:
			if !r.checkStringLen(fmt.Sprintf("%s.%s", key, elemKey), e, maxScalarStringLen) {
				return false
			}
		}
	}
	return true
}

// str reads args[key] into config[key] as a string. A number is coerced to
// its decimal form (an MCP client may send "3000" or 3000 for the same
// field); a bool, array, or map is a hard error.
func (r *argReader) str(key string) {
	r.strAs(key, key)
}

// strAs is str but the config key differs from the args key (e.g.
// heater_entity_id -> heater).
func (r *argReader) strAs(argKey, configKey string) {
	v, ok := r.args[argKey]
	if !ok || isSkippable(v) {
		return
	}
	switch val := v.(type) {
	case string:
		if !r.checkStringLen(argKey, val, maxScalarStringLen) {
			return
		}
		r.config[configKey] = val
	case float64:
		r.config[configKey] = strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		r.config[configKey] = strconv.Itoa(val)
	case int64:
		r.config[configKey] = strconv.FormatInt(val, 10)
	default:
		r.fail(argKey, "a string", v)
	}
}

// strID reads args[key] as a strict string with no numeric coercion, for
// fields that identify an entity or config object (entity_id, source,
// filter, ...) rather than holding a numeric value spelled as a string.
// str()/strAs() silently stringify a float64/int/int64 - correct for a
// genuinely numeric field expressed as a string (e.g. offset, cycle), but
// the wrong call for an identifier: a caller bug that sends entity_id:
// 12345 would otherwise be accepted here and only surface later as an
// opaque Home Assistant "entity not found" error instead of this reader's
// own clear type message.
func (r *argReader) strID(key string) {
	r.strIDAs(key, key)
}

// strIDAs is strID with a different arg/config key, for identifier fields
// Home Assistant's API renames (e.g. heater_entity_id -> heater).
func (r *argReader) strIDAs(argKey, configKey string) {
	v, ok := r.args[argKey]
	if !ok || isSkippable(v) {
		return
	}
	s, ok := v.(string)
	if !ok {
		r.fail(argKey, "a string", v)
		return
	}
	if !r.checkStringLen(argKey, s, maxScalarStringLen) {
		return
	}
	r.config[configKey] = s
}

// num reads args[key] into config[key] as a float64. A numeric string is
// parsed; NaN/Inf and unparseable strings are hard errors.
func (r *argReader) num(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	if s, isString := v.(string); isString && !r.checkStringLen(key, s, maxNumericStringLen) {
		return
	}
	f, ok := toFloat(v)
	if !ok {
		r.fail(key, "a number", v)
		return
	}
	r.config[key] = f
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, false
		}
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// integer reads args[key] into config[key] as an int. A float64 is
// accepted only when it carries no fractional part (int(2.7) silently
// truncating to 2 was itself a latent bug this replaces) and is otherwise
// a hard error, matching a whole-number string.
func (r *argReader) integer(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	if s, isString := v.(string); isString && !r.checkStringLen(key, s, maxNumericStringLen) {
		return
	}
	n, ok := toInt(v)
	if !ok {
		r.fail(key, "a whole number", v)
		return
	}
	r.config[key] = n
}

// outOfInt32Range reports whether f (already confirmed integral by the
// caller) exceeds the int32 bound every toInt branch enforces - mirrors
// secondsToDurationDict's guard, since no real helper field (round_digits,
// sampling_size, min/max_samples, ...) needs a wider range and every
// platform's int is at least 32 bits.
func outOfInt32Range(f float64) bool {
	return f < math.MinInt32 || f > math.MaxInt32
}

func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		if outOfInt32Range(float64(val)) {
			return 0, false
		}
		return val, true
	case int64:
		if outOfInt32Range(float64(val)) {
			return 0, false
		}
		return int(val), true
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) || val != math.Trunc(val) || outOfInt32Range(val) {
			// Converting an out-of-range float to int is
			// implementation-defined in Go (e.g. int(1e20) silently
			// becomes math.MinInt64, not a clamp or a panic) - reject
			// rather than forward a garbage value into the config sent to
			// Home Assistant.
			return 0, false
		}
		return int(val), true
	case string:
		return intFromString(val)
	default:
		return 0, false
	}
}

// intFromString is toInt's string branch, split out to keep toInt's
// cognitive complexity down. strconv.Atoi's fast path and the
// strconv.ParseFloat whole-number fallback each need their own int32
// bound check, mirroring toInt's other branches.
func intFromString(s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		// strconv.Atoi parses into platform int (64-bit on amd64/arm64),
		// so a whole-number string like "5000000000" succeeds here
		// without ever reaching the float64 fallback below - needs the
		// same int32 bound toInt's other branches enforce.
		if outOfInt32Range(float64(n)) {
			return 0, false
		}
		return n, true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsInf(f, 0) && f == math.Trunc(f) && !outOfInt32Range(f) {
		return int(f), true
	}
	return 0, false
}

// boolean reads args[key] into config[key] as a bool. Accepts strconv's
// ParseBool spellings plus HA's own YAML bool vocabulary (yes/no/on/off),
// case-insensitively, and the numbers 0/1. Any other string or number is a
// hard error.
func (r *argReader) boolean(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	switch val := v.(type) {
	case bool:
		r.config[key] = val
	case string:
		if !r.checkStringLen(key, val, maxNumericStringLen) {
			return
		}
		if b, ok := parseBoolLoose(val); ok {
			r.config[key] = b
			return
		}
		r.fail(key, "a boolean", v)
	case float64:
		switch val {
		case 0:
			r.config[key] = false
		case 1:
			r.config[key] = true
		default:
			r.fail(key, "a boolean", v)
		}
	default:
		r.fail(key, "a boolean", v)
	}
}

func parseBoolLoose(s string) (bool, bool) {
	switch strings.ToLower(s) {
	case "1", "t", "true", "yes", "y", "on":
		return true, true
	case "0", "f", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

// strSlice reads args[key] into config[key] as a []string. Numeric
// elements are coerced to their decimal string form; any element that
// isn't a string or number is a hard error (today's convertToStringSlice
// silently dropped such elements instead).
func (r *argReader) strSlice(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	arr, ok := v.([]any)
	if !ok {
		r.fail(key, "an array of strings", v)
		return
	}
	if !r.checkArrayLen(key, arr) {
		return
	}
	out := make([]string, 0, len(arr))
	for i, elem := range arr {
		switch e := elem.(type) {
		case string:
			if len(e) > maxScalarStringLen {
				r.failElem(key, i, fmt.Sprintf("a string under %d bytes", maxScalarStringLen), elem)
				return
			}
			out = append(out, e)
		case float64:
			out = append(out, strconv.FormatFloat(e, 'f', -1, 64))
		case int:
			out = append(out, strconv.Itoa(e))
		default:
			r.failElem(key, i, "a string", elem)
			return
		}
	}
	r.config[key] = out
}

// anySlice reads args[key] into config[key] verbatim as a []any (for
// fields whose elements are objects, e.g. schedule's per-day time blocks).
// Each element is still bounded the same way raw()'s value is: a flat map
// can't nest a further container and its string values are length-capped,
// and a bare string element is length-capped - checkArrayLen alone only
// bounds the number of elements, not their size, so without this a single
// element could carry an arbitrarily large or deeply nested payload through
// unbounded.
func (r *argReader) anySlice(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	arr, ok := v.([]any)
	if !ok {
		r.fail(key, "an array", v)
		return
	}
	if !r.checkArrayLen(key, arr) {
		return
	}
	for i, elem := range arr {
		elemKey := fmt.Sprintf("%s[%d]", key, i)
		switch e := elem.(type) {
		case map[string]any:
			if !r.checkMapLen(elemKey, e) || !r.checkFlatMapValues(elemKey, e) {
				return
			}
		case string:
			if !r.checkStringLen(elemKey, e, maxScalarStringLen) {
				return
			}
		}
	}
	r.config[key] = arr
}

// raw reads args[key] into config[key] without coercion, for fields whose
// valid shape depends on context the reader can't see - e.g. filter's
// window_size, which is a sample-count number for some filter types and a
// duration for others (normalised later in hybrid_client.go's
// toDurationDict). Only bools and arrays are rejected outright; everything
// else passes through for HA (or that later normalisation step) to
// validate - but a string value, at the top level or one level inside a
// duration-dict-shaped map, is still length-capped via checkStringLen, the
// same as every other reader in this file.
func (r *argReader) raw(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	switch val := v.(type) {
	case bool, []any:
		r.fail(key, "a number, string, or duration object", v)
		return
	case string:
		if !r.checkStringLen(key, val, maxScalarStringLen) {
			return
		}
	case map[string]any:
		if !r.checkMapLen(key, val) {
			return
		}
		// A legitimate value here (e.g. filter's window_size as a
		// {"hours":.,"minutes":.,"seconds":.} duration dict) is always
		// flat - every value a plain number or bounded string.
		// checkFlatMapValues rejects a nested container one level down
		// and bounds any string value, so a caller can't smuggle an
		// arbitrarily large or deeply nested payload through to the
		// HTTP body sent to Home Assistant under the guise of a
		// duration dict.
		if !r.checkFlatMapValues(key, val) {
			return
		}
	}
	r.config[key] = v
}

// maxActionDepth and maxActionNodes bound an HA action sequence
// (ActionSelector) value read by actionValue. Unlike raw()'s duration
// dicts, an action's shape (target/data) is arbitrary by nature and can't
// be validated as flat, so it's bounded by nesting depth and total node
// count instead.
const (
	maxActionDepth = 8
	maxActionNodes = 2000
)

// actionValue reads an HA action sequence (ActionSelector): a single
// action object (e.g. {"action": "switch.turn_on", "target": {...}}) or a
// list of them. Bounded by depth/node count rather than checked for a
// specific shape, since HA validates the actual action semantics itself -
// this only guards against an unbounded payload being forwarded verbatim
// into the HTTP body sent to Home Assistant.
func (r *argReader) actionValue(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	switch v.(type) {
	case map[string]any, []any:
	default:
		r.fail(key, "an action object or a list of action objects", v)
		return
	}
	nodes := 0
	if !boundedActionShape(v, 1, &nodes) {
		r.fail(key, fmt.Sprintf("an action value within depth %d and %d total values", maxActionDepth, maxActionNodes), v)
		return
	}
	r.config[key] = v
}

// boundedActionShape reports whether v's nesting depth and total node
// count both stay within actionValue's limits.
func boundedActionShape(v any, depth int, nodes *int) bool {
	*nodes++
	if *nodes > maxActionNodes || depth > maxActionDepth {
		return false
	}
	switch val := v.(type) {
	case map[string]any:
		for _, elem := range val {
			if !boundedActionShape(elem, depth+1, nodes) {
				return false
			}
		}
	case []any:
		for _, elem := range val {
			if !boundedActionShape(elem, depth+1, nodes) {
				return false
			}
		}
	}
	return true
}

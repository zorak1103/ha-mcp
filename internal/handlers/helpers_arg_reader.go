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

// argTypeError is the single message constructor for every reader failure,
// so wording stays consistent across all ~170 call sites.
func argTypeError(key, want string, got any) error {
	return fmt.Errorf("invalid value for %q: expected %s, got %T (%v)", key, want, got, got)
}

func isSkippable(v any) bool {
	if v == nil {
		return true
	}
	s, isString := v.(string)
	return isString && s == ""
}

// maxArrayElements bounds every array-typed field the argReader accepts.
// strSlice/anySlice pre-allocate their output slice sized directly from the
// caller-supplied array's length (make([]string, 0, len(arr))) - without a
// cap, a single manage_helper call with an absurdly large array (entity_ids,
// tariffs, options, a schedule day's time blocks, ...) forces a
// correspondingly large allocation before any per-element validation runs.
// No real helper field plausibly needs more than a few hundred elements.
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

// num reads args[key] into config[key] as a float64. A numeric string is
// parsed; NaN/Inf and unparseable strings are hard errors.
func (r *argReader) num(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
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
	n, ok := toInt(v)
	if !ok {
		r.fail(key, "a whole number", v)
		return
	}
	r.config[key] = n
}

func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) || val != math.Trunc(val) {
			return 0, false
		}
		return int(val), true
	case string:
		if n, err := strconv.Atoi(val); err == nil {
			return n, true
		}
		if f, err := strconv.ParseFloat(val, 64); err == nil && !math.IsInf(f, 0) && f == math.Trunc(f) {
			return int(f), true
		}
		return 0, false
	default:
		return 0, false
	}
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
	for _, elem := range arr {
		switch e := elem.(type) {
		case string:
			out = append(out, e)
		case float64:
			out = append(out, strconv.FormatFloat(e, 'f', -1, 64))
		case int:
			out = append(out, strconv.Itoa(e))
		default:
			r.fail(key, "an array of strings", v)
			return
		}
	}
	r.config[key] = out
}

// anySlice reads args[key] into config[key] verbatim as a []any (for
// fields whose elements are objects, e.g. schedule's per-day time blocks).
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
	r.config[key] = arr
}

// raw reads args[key] into config[key] without coercion, for fields whose
// valid shape depends on context the reader can't see - e.g. filter's
// window_size, which is a sample-count number for some filter types and a
// duration for others (normalised later in hybrid_client.go's
// toDurationDict). Only bools and arrays are rejected outright; everything
// else passes through for HA (or that later normalisation step) to
// validate.
func (r *argReader) raw(key string) {
	v, ok := r.args[key]
	if !ok || isSkippable(v) {
		return
	}
	switch v.(type) {
	case bool, []any:
		r.fail(key, "a number, string, or duration object", v)
		return
	}
	r.config[key] = v
}

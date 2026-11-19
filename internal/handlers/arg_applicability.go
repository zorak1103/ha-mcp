package handlers

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// isAbsentArg reports whether a tool argument value is a placeholder that
// callers (and some MCP clients) send to mean "not supplied": nil, an empty
// string or a zero-length array. Zero numbers and false are real values.
func isAbsentArg(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return val == ""
	case []any:
		return len(val) == 0
	default:
		return false
	}
}

// stringArgValue is the strict reader for an optional string argument, such as
// a list/query/find filter or an ID. An absent value (see isAbsentArg) yields
// ("", ""); anything that is not a string yields an error message naming the
// argument and the offending type. Read filters with it, not getString or a
// bare .(string): the server's type check lets a JSON number through for a
// string property, and a loose read turns it into "no filter".
func stringArgValue(name string, v any) (value, errMsg string) {
	if isAbsentArg(v) {
		return "", ""
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Sprintf("%s must be a string, got %T", name, v)
	}
	return s, ""
}

// stringFilterField names one string filter argument and where its value goes.
type stringFilterField struct {
	name string
	dst  *string
}

// readStringFilters reads each field with stringArgValue, in order, and stops at
// the first present non-string value with an error naming it.
func readStringFilters(args map[string]any, fields []stringFilterField) error {
	for _, f := range fields {
		value, errMsg := stringArgValue(f.name, args[f.name])
		if errMsg != "" {
			return errors.New(errMsg)
		}
		*f.dst = value
	}
	return nil
}

// parseModeArg reads the selector argument key (e.g. "mode") from args and
// validates it against validModes. An absent value (see isAbsentArg) yields
// defaultMode; a non-string value or a string outside validModes yields an
// error result instead of being silently coerced or ignored.
func parseModeArg(args map[string]any, key, defaultMode string, validModes []string) (string, *mcp.ToolsCallResult) {
	value, present := args[key]
	if !present || isAbsentArg(value) {
		return defaultMode, nil
	}
	valid := strings.Join(validModes, ", ")
	text, ok := value.(string)
	if !ok {
		return "", errorResult(fmt.Sprintf("Invalid %s: expected a string, got %T. Must be one of: %s", key, value, valid))
	}
	if !slices.Contains(validModes, text) {
		return "", errorResult(fmt.Sprintf("Invalid %s %q. Must be one of: %s", key, text, valid))
	}
	return text, nil
}

// rejectInapplicableArgs returns an error result when args holds a supplied
// (non-absent, see isAbsentArg) argument that onlyFor restricts to modes other
// than active; selector names the argument that selects the mode (e.g. "mode").
// Arguments missing from onlyFor are never rejected. The offending arguments
// are listed sorted by name, each with its modes in declared order, and hint
// (when non-empty) is appended to the message. It returns nil when every
// supplied argument applies to active.
//
// The output-shaping arguments format and verbose are exempt everywhere: they
// are deliberately never listed in an onlyFor map.
func rejectInapplicableArgs(args map[string]any, selector, active string, onlyFor map[string][]string, hint string) *mcp.ToolsCallResult {
	var offenders []string
	for name, modes := range onlyFor {
		if value, present := args[name]; !present || isAbsentArg(value) || slices.Contains(modes, active) {
			continue
		}
		offenders = append(offenders, name)
	}
	if len(offenders) == 0 {
		return nil
	}
	slices.Sort(offenders)

	parts := make([]string, len(offenders))
	for i, name := range offenders {
		parts[i] = fmt.Sprintf("%s (only for %s=%s)", name, selector, strings.Join(onlyFor[name], "|"))
	}
	msg := fmt.Sprintf("argument(s) not applicable to %s=%s: %s.", selector, active, strings.Join(parts, ", "))
	if hint != "" {
		msg += " " + hint
	}
	return errorResult(msg)
}

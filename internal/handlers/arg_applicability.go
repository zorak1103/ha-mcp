package handlers

import (
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

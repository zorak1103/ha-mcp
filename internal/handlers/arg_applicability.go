package handlers

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

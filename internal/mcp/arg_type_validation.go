package mcp

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	// maxArgValueRunes bounds how much of one rejected string value is echoed back.
	maxArgValueRunes = 64
	// maxListedInvalidArgs bounds how many violations are listed in one error.
	maxListedInvalidArgs = 10
)

// InvalidArguments returns one message per argument in args whose value
// violates the type or enum its property declares in schema, ordered by
// argument name and capped at maxListedInvalidArgs lines plus a summary line.
// It returns nil when every supplied value is acceptable.
//
// The check rejects only what no handler could coerce: the handlers' own
// readers (argReader and friends) still accept numeric strings for
// integer/number properties, a number for a string property and the loose
// boolean spellings, so those pass here too and are left for the handler to
// convert (a handler that reads with a bare type assertion ignores such a
// value, as it did before this check). JSON null, the empty string and an
// empty array count as "not supplied", as they do for the handlers. Undeclared names are UnknownArguments' concern, and a
// property without a Type is checked against its Enum only.
//
// Element types of arrays and the shape of objects are not checked.
func InvalidArguments(schema JSONSchema, args map[string]any) []string {
	names := make([]string, 0, len(args))
	for name := range args {
		if _, declared := schema.Properties[name]; declared {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var violations []string
	for _, name := range names {
		if msg := invalidArgument(name, schema.Properties[name], args[name]); msg != "" {
			violations = append(violations, msg)
		}
	}
	if extra := len(violations) - maxListedInvalidArgs; extra > 0 {
		violations = append(violations[:maxListedInvalidArgs:maxListedInvalidArgs],
			fmt.Sprintf("(+%d more invalid arguments)", extra))
	}
	return violations
}

// invalidArgument returns the violation message for one supplied value, or ""
// when the value satisfies prop.
func invalidArgument(name string, prop JSONSchema, value any) string {
	if isUnsetArgument(value) {
		return ""
	}
	if !matchesType(prop.Type, value) {
		return fmt.Sprintf("argument %q must be %s, got %s", name, typeNoun(prop.Type), describeReceived(value))
	}
	if len(prop.Enum) > 0 && (prop.Type == "" || prop.Type == typeString) {
		if s, ok := enumCandidate(value); !ok || !slices.Contains(prop.Enum, s) {
			return fmt.Sprintf("argument %q must be one of: %s; got %s",
				name, strings.Join(prop.Enum, ", "), describeEnumReceived(value))
		}
	}
	return ""
}

// invalidArgumentsMessage renders the tool error for rejected argument values.
func invalidArgumentsMessage(toolName string, violations []string) string {
	return fmt.Sprintf("invalid argument(s) for tool %q: %s", toolName, strings.Join(violations, "; "))
}

// JSON Schema type names the validator understands.
const (
	typeString  = "string"
	typeInteger = "integer"
	typeNumber  = "number"
	typeBoolean = "boolean"
	typeArray   = "array"
	typeObject  = "object"
)

// isUnsetArgument reports whether a handler treats value as not supplied: null,
// the empty string or an empty array (handlers' isAbsentArg). Zero numbers and
// false are real values.
func isUnsetArgument(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	default:
		return false
	}
}

// matchesType reports whether value can be coerced to the declared JSON
// Schema type. An empty or unrecognized type matches everything.
func matchesType(schemaType string, value any) bool {
	switch schemaType {
	case typeString:
		switch value.(type) {
		case bool, []any, map[string]any:
			return false
		}
		return true
	case typeInteger:
		f, ok := numericValue(value)
		return ok && f == math.Trunc(f)
	case typeNumber:
		_, ok := numericValue(value)
		return ok
	case typeBoolean:
		return booleanValue(value)
	case typeArray:
		_, ok := value.([]any)
		return ok
	case typeObject:
		_, ok := value.(map[string]any)
		return ok
	default:
		return true
	}
}

// numericValue returns value as a finite float64 when it is a number or a
// string holding one.
func numericValue(value any) (float64, bool) {
	var f float64
	switch v := value.(type) {
	case float64:
		f = v
	case int:
		f = float64(v)
	case int64:
		f = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		f = parsed
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// booleanValue reports whether value is a bool or one of the spellings the
// handlers' boolean reader accepts (argReader.boolean in internal/handlers):
// yes/no, on/off, true/false, t/f, y/n, 1/0 in any case, and the numbers 0/1.
func booleanValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return true
	case string:
		switch strings.ToLower(v) {
		case "1", "t", "true", "yes", "y", "on", "0", "f", "false", "no", "n", "off":
			return true
		}
		return false
	default:
		f, ok := numericValue(value)
		return ok && (f == 0 || f == 1)
	}
}

// enumCandidate returns the string form of value for an enum membership test:
// a string as is, a number in its decimal form.
func enumCandidate(value any) (string, bool) {
	if s, ok := value.(string); ok {
		return s, true
	}
	if f, ok := numericValue(value); ok {
		return formatNumber(f), true
	}
	return "", false
}

// typeNoun returns schemaType with its article, for "must be ...".
func typeNoun(schemaType string) string {
	switch schemaType {
	case typeInteger, typeArray, typeObject:
		return "an " + schemaType
	default:
		return "a " + schemaType
	}
}

// describeReceived names the kind of value received. Strings and numbers are
// echoed (a string cut to maxArgValueRunes and quoted, escaping control
// characters); arrays and objects are not, since their content can be large.
func describeReceived(value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("string %q", truncateRunes(v, maxArgValueRunes))
	case bool:
		return typeBoolean
	case []any:
		return typeArray
	case map[string]any:
		return typeObject
	default:
		if f, ok := numericValue(value); ok {
			return "number " + formatNumber(f)
		}
		return fmt.Sprintf("%T", value)
	}
}

// describeEnumReceived is describeReceived for an enum violation, where the
// listed values already show a string is expected: a string is echoed bare.
func describeEnumReceived(value any) string {
	if s, ok := value.(string); ok {
		return fmt.Sprintf("%q", truncateRunes(s, maxArgValueRunes))
	}
	return describeReceived(value)
}

func formatNumber(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// InvalidArgumentsResult returns the error tool result the server answers with
// when a supplied argument has a value schema's type or enum rules out, or nil
// when the call may proceed. Exported so test harnesses that bypass the server
// can return exactly what a client would see.
func InvalidArgumentsResult(toolName string, schema JSONSchema, args map[string]any) *ToolsCallResult {
	violations := InvalidArguments(schema, args)
	if len(violations) == 0 {
		return nil
	}
	return &ToolsCallResult{
		Content: []ContentBlock{NewTextContent(invalidArgumentsMessage(toolName, violations))},
		IsError: true,
	}
}

// rejectInvalidArguments is InvalidArgumentsResult for a registered tool.
func (s *Server) rejectInvalidArguments(toolName string, args map[string]any) *ToolsCallResult {
	tool, ok := s.registry.GetTool(toolName)
	if !ok {
		return nil
	}
	rejected := InvalidArgumentsResult(toolName, tool.InputSchema, args)
	if rejected != nil {
		s.logger.Warn("Tool call with invalid arguments", "tool", toolName)
	}
	return rejected
}

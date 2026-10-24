package mcp

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// maxArgNameRunes bounds how much of one unknown argument name is echoed back.
	maxArgNameRunes = 64
	// maxListedUnknownArgs bounds how many unknown argument names are echoed back.
	maxListedUnknownArgs = 10
)

// UnknownArguments returns the keys of args that schema does not declare in
// Properties, sorted. It returns nil when every key is declared. A schema
// without Properties accepts only an empty or nil args map.
func UnknownArguments(schema JSONSchema, args map[string]any) []string {
	var unknown []string
	for name := range args {
		if _, ok := schema.Properties[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// unknownArgumentsMessage renders the tool error for undeclared arguments.
// The names come from the caller, so each is cut to maxArgNameRunes, quoted
// (escaping control characters) and at most maxListedUnknownArgs are listed.
func unknownArgumentsMessage(toolName string, unknown []string, schema JSONSchema) string {
	listed := unknown
	if len(listed) > maxListedUnknownArgs {
		listed = listed[:maxListedUnknownArgs]
	}
	quoted := make([]string, len(listed))
	for i, name := range listed {
		quoted[i] = fmt.Sprintf("%q", truncateRunes(name, maxArgNameRunes))
	}
	more := ""
	if extra := len(unknown) - len(listed); extra > 0 {
		more = fmt.Sprintf(" (+%d more)", extra)
	}

	valid := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		valid = append(valid, name)
	}
	sort.Strings(valid)
	validList := "(none)"
	if len(valid) > 0 {
		validList = strings.Join(valid, ", ")
	}

	return fmt.Sprintf("unknown argument(s) %s%s for tool %q; valid arguments: %s",
		strings.Join(quoted, ", "), more, toolName, validList)
}

// truncateRunes cuts s to at most limit runes and appends an ellipsis when it cut.
func truncateRunes(s string, limit int) string {
	count := 0
	for i := range s {
		if count == limit {
			return s[:i] + "…"
		}
		count++
	}
	return s
}

// rejectUnknownArguments returns an error tool result when args contains names
// the tool's schema does not declare, or nil when the call may proceed.
func (s *Server) rejectUnknownArguments(toolName string, args map[string]any) *ToolsCallResult {
	tool, ok := s.registry.GetTool(toolName)
	if !ok {
		return nil
	}
	unknown := UnknownArguments(tool.InputSchema, args)
	if len(unknown) == 0 {
		return nil
	}
	s.logger.Warn("Tool call with unknown arguments", "tool", toolName, "unknown", len(unknown))
	return &ToolsCallResult{
		Content: []ContentBlock{NewTextContent(unknownArgumentsMessage(toolName, unknown, tool.InputSchema))},
		IsError: true,
	}
}

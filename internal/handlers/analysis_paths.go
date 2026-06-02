// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/jsonpatch"
)

// ReferencePath is a single location where an entity_id appears in a config,
// expressed as an RFC 6901 JSON Pointer path with a compact action context label.
type ReferencePath struct {
	Path    string `json:"path"`
	Context string `json:"context,omitempty"`
}

// collectEntityPaths recursively walks node looking for leaves equal to entityID
// and returns the RFC 6901 pointer path to each match, rooted at prefix.
// Map keys are visited in sorted order for deterministic output.
func collectEntityPaths(node any, entityID, prefix string) []string {
	if node == nil {
		return nil
	}
	switch v := node.(type) {
	case string:
		if v == entityID {
			return []string{prefix}
		}
		return nil
	case []any:
		var paths []string
		for i, item := range v {
			sub := collectEntityPaths(item, entityID, prefix+"/"+strconv.Itoa(i))
			paths = append(paths, sub...)
		}
		return paths
	case map[string]any:
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var paths []string
		for _, k := range keys {
			sub := collectEntityPaths(v[k], entityID, prefix+"/"+jsonpatch.EscapeSegment(k))
			paths = append(paths, sub...)
		}
		return paths
	}
	return nil
}

// collectSectionReferencePaths collects all ReferencePaths for an entity_id
// within a top-level section array (triggers, conditions, actions, sequence).
// Each top-level step that contains the entity yields one ReferencePath per match,
// all sharing the step-level context label.
func collectSectionReferencePaths(section []any, sectionName, kind, entityID string) []ReferencePath {
	var result []ReferencePath
	for i, item := range section {
		stepPrefix := fmt.Sprintf("/%s/%d", sectionName, i)
		paths := collectEntityPaths(item, entityID, stepPrefix)
		if len(paths) == 0 {
			continue
		}
		// Derive context from the top-level step node.
		ctx := ""
		if stepMap, ok := item.(map[string]any); ok {
			ctx = referencePathContext(kind, stepMap)
		}
		for _, p := range paths {
			result = append(result, ReferencePath{Path: p, Context: ctx})
		}
	}
	return result
}

// collectAutomationReferencePaths collects all ReferencePaths for an entity_id
// across all three sections of an AutomationConfig (triggers, conditions, actions).
func collectAutomationReferencePaths(config *homeassistant.AutomationConfig, entityID string) []ReferencePath {
	var paths []ReferencePath
	paths = append(paths, collectSectionReferencePaths(config.Triggers, "triggers", "trigger", entityID)...)
	paths = append(paths, collectSectionReferencePaths(config.Conditions, "conditions", "condition", entityID)...)
	paths = append(paths, collectSectionReferencePaths(config.Actions, "actions", "action", entityID)...)
	return paths
}

// referencePathContext returns a compact context label for a top-level config step.
// kind is "action", "trigger", or "condition".
func referencePathContext(kind string, step map[string]any) string {
	switch kind {
	case "action":
		if svc, ok := step["action"].(string); ok {
			return "action: " + svc
		}
		if svc, ok := step["service"].(string); ok {
			return "action: " + svc
		}
		if _, ok := step["choose"]; ok {
			return "action: choose"
		}
		if _, ok := step["if"]; ok {
			return "action: if/then"
		}
		return "action"
	case "trigger":
		if p, ok := step["platform"].(string); ok {
			return "trigger: " + p
		}
		if p, ok := step["trigger"].(string); ok {
			return "trigger: " + p
		}
		return "trigger"
	case "condition":
		if c, ok := step["condition"].(string); ok {
			return "condition: " + c
		}
		return "condition"
	}
	return kind
}

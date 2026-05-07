// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Config node type constants for excerpt summarization.
const (
	excerptTriggerState        = "state"
	excerptTriggerNumericState = "numeric_state"
	excerptConditionState      = "condition"
)

// collectEntityExcerpts collects one-line summaries of every top-level config node
// in an automation that references entityID.
func collectEntityExcerpts(config *homeassistant.AutomationConfig, entityID string) []UsageExcerpt {
	sections := []struct {
		name  string
		items []any
	}{
		{excerptSectionTrigger, config.Triggers},
		{excerptConditionState, config.Conditions},
		{usedInAction, config.Actions},
	}

	var excerpts []UsageExcerpt
	for _, sec := range sections {
		for _, item := range sec.items {
			if !searchInConfigValue(item, entityID) {
				continue
			}
			node, ok := item.(map[string]any)
			if !ok {
				continue
			}
			excerpts = append(excerpts, UsageExcerpt{
				Section: sec.name,
				Summary: summarizeConfigNode(sec.name, node, entityID),
			})
		}
	}
	return excerpts
}

// collectSequenceExcerpts collects excerpts from a script's sequence (action list).
func collectSequenceExcerpts(sequence []any, entityID string) []UsageExcerpt {
	var excerpts []UsageExcerpt
	for _, item := range sequence {
		if !searchInConfigValue(item, entityID) {
			continue
		}
		node, ok := item.(map[string]any)
		if !ok {
			continue
		}
		excerpts = append(excerpts, UsageExcerpt{
			Section: usedInAction,
			Summary: summarizeActionNode(node, entityID),
		})
	}
	return excerpts
}

// excerptSectionTrigger is the trigger section name used in excerpts.
const excerptSectionTrigger = "trigger"

// summarizeConfigNode produces a compact one-line description of a config node.
func summarizeConfigNode(section string, node map[string]any, entityID string) string {
	switch section {
	case excerptSectionTrigger:
		return summarizeTriggerNode(node, entityID)
	case excerptConditionState:
		return summarizeConditionNode(node, entityID)
	case usedInAction:
		return summarizeActionNode(node, entityID)
	}
	return fmt.Sprintf("references %s", entityID)
}

// summarizeTriggerNode produces a one-line summary of a trigger config node.
func summarizeTriggerNode(node map[string]any, entityID string) string {
	// HA uses "platform" in older configs and "trigger" key in newer ones
	platform, _ := node["platform"].(string)
	if platform == "" {
		platform, _ = node["trigger"].(string)
	}

	eid := getNodeEntityID(node, entityID)

	switch platform {
	case excerptTriggerState:
		var parts []string
		if from, ok := node["from"].(string); ok {
			parts = append(parts, fmt.Sprintf("from %q", from))
		}
		if to, ok := node["to"].(string); ok {
			parts = append(parts, fmt.Sprintf("→ %q", to))
		}
		if id, ok := node["id"].(string); ok {
			parts = append(parts, fmt.Sprintf("(id: %q)", id))
		}
		if len(parts) > 0 {
			return fmt.Sprintf("state %s %s", eid, strings.Join(parts, " "))
		}
		return fmt.Sprintf("state %s", eid)
	case excerptTriggerNumericState:
		var parts []string
		if above, ok := node["above"]; ok {
			parts = append(parts, fmt.Sprintf("above %v", above))
		}
		if below, ok := node["below"]; ok {
			parts = append(parts, fmt.Sprintf("below %v", below))
		}
		if len(parts) > 0 {
			return fmt.Sprintf("numeric_state %s %s", eid, strings.Join(parts, " "))
		}
		return fmt.Sprintf("numeric_state %s", eid)
	}

	if platform != "" {
		return fmt.Sprintf("%s %s", platform, entityID)
	}
	return fmt.Sprintf("references %s", entityID)
}

// summarizeConditionNode produces a one-line summary of a condition config node.
func summarizeConditionNode(node map[string]any, entityID string) string {
	condType, _ := node[excerptConditionState].(string)
	eid := getNodeEntityID(node, entityID)

	switch condType {
	case excerptTriggerState:
		if state, ok := node[excerptTriggerState].(string); ok {
			return fmt.Sprintf("state %s = %q", eid, state)
		}
		return fmt.Sprintf("state %s", eid)
	case excerptTriggerNumericState:
		var parts []string
		if above, ok := node["above"]; ok {
			parts = append(parts, fmt.Sprintf("above %v", above))
		}
		if below, ok := node["below"]; ok {
			parts = append(parts, fmt.Sprintf("below %v", below))
		}
		if len(parts) > 0 {
			return fmt.Sprintf("numeric_state %s %s", eid, strings.Join(parts, " "))
		}
		return fmt.Sprintf("numeric_state %s", eid)
	}

	if condType != "" {
		return fmt.Sprintf("%s referencing %s", condType, entityID)
	}
	return fmt.Sprintf("references %s", entityID)
}

// summarizeActionNode produces a one-line summary of an action config node.
func summarizeActionNode(node map[string]any, entityID string) string {
	// Service/action call (newer HA uses "action", older uses "service")
	if svc, ok := node["action"].(string); ok {
		return fmt.Sprintf("service %s", svc)
	}
	if svc, ok := node["service"].(string); ok {
		return fmt.Sprintf("service %s", svc)
	}
	// choose block
	if _, ok := node["choose"]; ok {
		return fmt.Sprintf("choose (contains %s)", entityID)
	}
	// if/then block
	if _, ok := node["if"]; ok {
		return fmt.Sprintf("if/then (contains %s)", entityID)
	}
	return fmt.Sprintf("references %s", entityID)
}

// getNodeEntityID returns the entity_id from a config node, falling back to the searched entityID.
func getNodeEntityID(node map[string]any, fallback string) string {
	if eid, ok := node[configKeyEntityID].(string); ok {
		return eid
	}
	return fallback
}

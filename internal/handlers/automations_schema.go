// coverage-exempt: static data structure with unreachable json.Marshal error path
package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// automationSchemaType describes a trigger, condition, or action type with its fields.
type automationSchemaType struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    []string `json:"required,omitempty"`
	Optional    []string `json:"optional,omitempty"`
}

// automationSchema holds the reference for trigger, condition, and action types.
var automationSchema = struct {
	Triggers   []automationSchemaType `json:"triggers"`
	Conditions []automationSchemaType `json:"conditions"`
	Actions    []automationSchemaType `json:"actions"`
	Notes      []string               `json:"notes"`
}{
	Triggers: []automationSchemaType{
		{Type: "state", Description: "Fires when entity state changes", Required: []string{attrEntityID}, Optional: []string{"from", "to", "for", "attribute"}},
		{Type: "numeric_state", Description: "Fires when numeric value crosses threshold", Required: []string{attrEntityID}, Optional: []string{"above", "below", "for", "attribute", "value_template"}},
		{Type: "time", Description: "Fires at a specific time of day", Required: []string{"at"}},
		{Type: "time_pattern", Description: "Fires on a cron-like time pattern", Optional: []string{"hours", "minutes", "seconds"}},
		{Type: "sun", Description: "Fires at sunrise or sunset", Required: []string{"event"}, Optional: []string{"offset"}},
		{Type: "homeassistant", Description: "Fires on HA start or shutdown", Required: []string{"event"}},
		{Type: "event", Description: "Fires when a specific event is fired", Required: []string{"event_type"}, Optional: []string{"event_data", "context"}},
		{Type: "template", Description: "Fires when template evaluates to true", Required: []string{"value_template"}, Optional: []string{"for"}},
		{Type: "webhook", Description: "Fires when a webhook is called", Required: []string{"webhook_id"}, Optional: []string{"allowed_methods", "local_only"}},
		{Type: "mqtt", Description: "Fires when MQTT message received", Required: []string{"topic"}, Optional: []string{"payload", "encoding", "qos"}},
		{Type: "zone", Description: "Fires when entity enters or leaves a zone", Required: []string{attrEntityID, "zone", "event"}},
		{Type: "device", Description: "Fires on a device-specific trigger", Required: []string{"device_id", "domain", "type"}},
		{Type: "tag", Description: "Fires when an NFC/QR tag is scanned", Required: []string{"tag_id"}, Optional: []string{"device_id"}},
		{Type: "calendar", Description: "Fires on calendar event start/end", Required: []string{attrEntityID, "event"}, Optional: []string{"offset"}},
		{Type: "conversation", Description: "Fires when voice command matches", Required: []string{"command"}},
	},
	Conditions: []automationSchemaType{
		{Type: "state", Description: "Checks entity state or attribute", Required: []string{attrEntityID, "state"}, Optional: []string{"attribute", "for", "match"}},
		{Type: "numeric_state", Description: "Checks numeric value against threshold", Required: []string{attrEntityID}, Optional: []string{"above", "below", "attribute", "value_template"}},
		{Type: "time", Description: "Checks current time", Optional: []string{"after", "before", "weekday"}},
		{Type: "template", Description: "Evaluates Jinja2 template", Required: []string{"value_template"}},
		{Type: "zone", Description: "Checks if entity is in a zone", Required: []string{attrEntityID, "zone"}},
		{Type: "sun", Description: "Checks sun elevation or position", Optional: []string{"above", "below"}},
		{Type: "trigger", Description: "Checks which trigger fired (by ID)", Required: []string{"id"}},
		{Type: "device", Description: "Device-specific condition", Required: []string{"device_id", "domain", "type"}},
		{Type: "and", Description: "All sub-conditions must pass", Required: []string{"conditions"}},
		{Type: "or", Description: "At least one sub-condition must pass", Required: []string{"conditions"}},
		{Type: "not", Description: "Sub-condition must NOT pass", Required: []string{"conditions"}},
	},
	Actions: []automationSchemaType{
		{Type: "service/action (domain.service)", Description: "Call a HA service", Required: []string{"action"}, Optional: []string{"target", "data", "response_variable"}},
		{Type: "delay", Description: "Wait for a time period", Required: []string{"delay"}},
		{Type: "wait_template", Description: "Wait until template is true", Required: []string{"wait_template"}, Optional: []string{"timeout", "continue_on_timeout"}},
		{Type: "wait_for_trigger", Description: "Wait for a trigger to fire", Required: []string{"wait_for_trigger"}, Optional: []string{"timeout", "continue_on_timeout"}},
		{Type: "condition", Description: "Inline condition guard — stops sequence if false", Required: []string{"condition"}},
		{Type: "event", Description: "Fire a custom event", Required: []string{"event"}, Optional: []string{"event_data"}},
		{Type: "choose", Description: "Execute first matching option (if/elif/else)", Required: []string{"choose"}, Optional: []string{"default"}},
		{Type: "if", Description: "If/then/else branch", Required: []string{"if", "then"}, Optional: []string{"else"}},
		{Type: "repeat", Description: "Repeat sequence while/until/count times", Required: []string{"repeat"}, Optional: []string{"sequence"}},
		{Type: "parallel", Description: "Run actions in parallel", Required: []string{"parallel"}},
		{Type: "sequence", Description: "Run actions in sequence (inside parallel)", Required: []string{"sequence"}},
		{Type: "variables", Description: "Set local variables", Required: []string{"variables"}},
		{Type: "stop", Description: "Stop the automation", Optional: []string{"stop", "error"}},
	},
	Notes: []string{
		"Triggers use key 'trigger' (not 'platform') in modern HA YAML syntax",
		"Conditions use key 'condition' to specify the type",
		"Actions use key 'action' (service call) or a structural key (delay, choose, repeat, etc.)",
		"'sun' is a valid trigger and condition type — do NOT confuse with the sun.sun entity",
		"For sun elevation condition, use {condition: sun, above: -0.5} — NOT a state condition",
		"For above_horizon check, either use sun condition OR {condition: state, entity_id: sun.sun, state: above_horizon}",
		"'repeat/while' expects an array of conditions; 'repeat/count' expects a number; 'repeat/until' expects conditions",
	},
}

func (h *AutomationHandlers) handleSchema() (*mcp.ToolsCallResult, error) {
	data, err := json.MarshalIndent(automationSchema, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("error formatting schema: %v", err)), nil
	}
	return successResult(string(data)), nil
}

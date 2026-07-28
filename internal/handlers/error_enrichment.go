package handlers

import (
	"errors"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// configErrorHint maps a substring found in HA 400-error bodies to an actionable suggestion.
type configErrorHint struct {
	pattern    string
	suggestion string
}

// enrichConfigError appends an actionable hint when HA returns a 400 validation
// error whose body matches one of the provided hint patterns. Returns the original
// message unchanged when no match is found or the error is not a 400 APIError.
func enrichConfigError(msg string, err error, hints []configErrorHint) string {
	var apiErr *homeassistant.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		return msg
	}
	body := strings.ToLower(apiErr.Message)
	for _, hint := range hints {
		if strings.Contains(body, hint.pattern) {
			return msg + "\n\nHint: " + hint.suggestion
		}
	}
	return msg
}

//nolint:gochecknoglobals // static lookup tables for error enrichment

var automationErrorHints = []configErrorHint{
	{
		// Specific match before the generic "extra keys not allowed" catch-all.
		pattern:    "data['which']",
		suggestion: "'which' is not a valid key for the sun condition. Use: {\"condition\": \"state\", \"entity_id\": \"sun.sun\", \"state\": \"above_horizon\"} or \"below_horizon\" instead.",
	},
	{
		pattern:    "extra keys not allowed",
		suggestion: "The config contains an unrecognized key. Common mistakes: (1) 'sun' condition does not exist — use {\"condition\": \"state\", \"entity_id\": \"sun.sun\", \"state\": \"above_horizon\"} instead. (2) Check that trigger/condition/action keys match the HA schema for that type.",
	},
	{
		pattern:    "required key not provided",
		suggestion: "A required field is missing. Triggers need 'trigger' (type) key; conditions need 'condition' key; actions need 'action' (service) key.",
	},
	{
		pattern:    "expected a list",
		suggestion: "The 'trigger', 'condition', or 'automation_action' field must be an array (list), not a single object.",
	},
	{
		pattern:    "unable to find action",
		suggestion: "The service name in 'action' may be wrong. Use 'domain.service' format, e.g. 'light.turn_on', 'notify.mobile_app'. Check available services with call_service tool.",
	},
	{
		// Specific match before the generic "invalid template" catch-all below.
		pattern: "templatesyntaxerror",
		suggestion: "Jinja2 template syntax error. If the expression is complex, split it into " +
			"multiple '{% set %}' lines and keep the final '{{ ... }}' simple — chaining filters " +
			"(e.g. from_json) with and/or in one block often trips the parser. Example: " +
			"{% set days = states('input_text.x') %}{% set empty = days in ['unknown','unavailable',''] %}{{ empty }}. " +
			"The data[...] path shown in the error above identifies which template failed.",
	},
	{
		pattern:    "invalid template",
		suggestion: "Template syntax error. HA uses Jinja2: {{ states('sensor.x') }}, {{ is_state('entity', 'on') }}, etc.",
	},
}

var scriptErrorHints = []configErrorHint{
	{
		pattern:    "extra keys not allowed",
		suggestion: "The script config contains an unrecognized key. Script actions use the 'action' key (not 'service'). Check that all sequence step keys match the HA schema.",
	},
	{
		pattern:    "required key not provided",
		suggestion: "A required field is missing from the script config. Each sequence step needs an 'action' key.",
	},
	{
		// Specific match before the generic "invalid template" catch-all below.
		pattern: "templatesyntaxerror",
		suggestion: "Jinja2 template syntax error in script. If the expression is complex, split it into " +
			"multiple '{% set %}' lines and keep the final '{{ ... }}' simple — chaining filters " +
			"(e.g. from_json) with and/or in one block often trips the parser. Example: " +
			"{% set days = states('input_text.x') %}{% set empty = days in ['unknown','unavailable',''] %}{{ empty }}. " +
			"The data[...] path shown in the error above identifies which template failed.",
	},
	{
		pattern:    "invalid template",
		suggestion: "Template syntax error in script. HA uses Jinja2: {{ states('sensor.x') }}, {{ is_state('entity', 'on') }}, etc.",
	},
}

var sceneErrorHints = []configErrorHint{
	{
		pattern:    "extra keys not allowed",
		suggestion: "The scene config contains an unrecognized key. Entity entries use 'state' and attribute keys only — remove any unknown fields.",
	},
	{
		pattern:    "required key not provided",
		suggestion: "A required field is missing from the scene config. Each entity entry needs at least a 'state' field.",
	},
}

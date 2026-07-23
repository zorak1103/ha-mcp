// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Platform constants for helpers.
const (
	platformInputBoolean           = "input_boolean"
	platformInputNumber            = "input_number"
	platformInputText              = "input_text"
	platformInputSelect            = "input_select"
	platformInputDatetime          = "input_datetime"
	platformInputButton            = "input_button"
	platformCounter                = "counter"
	platformTimer                  = "timer"
	platformSchedule               = "schedule"
	platformGroup                  = "group"
	platformIntegration            = "integration"
	platformSensorEntity           = "sensor"
	platformBinarySensorEntity     = "binary_sensor"
	platformTemplate               = "template"
	helperTypeTemplateSensor       = "template_sensor"
	helperTypeTemplateBinarySensor = "template_binary_sensor"
	platformUtilityMeter           = "utility_meter"
	platformMinMax                 = "min_max"
	platformStatistics             = "statistics"
	platformTrend                  = "trend"
	platformRandom                 = "random"
	platformFilter                 = "filter"
	platformTod                    = "tod"
	platformGenericThermostat      = "generic_thermostat"
	platformSwitchAsX              = "switch_as_x"
	platformGenericHygrostat       = "generic_hygrostat"
	helperTypeRandomSensor         = "random_sensor"
	helperTypeRandomBinarySensor   = "random_binary_sensor"
	serviceSetValue                = "set_value"
	helperActionUpdate             = "update"
)

// helperTypeMetadata defines metadata for each helper type.
type helperTypeMetadata struct {
	platform           string   // Home Assistant platform name
	entityPrefix       string   // Prefix for resulting entity_id (e.g., "sensor" for derivative)
	supportedActions   []string // Actions supported by this helper type
	requiredFields     []string // Fields required for create operation
	optionalFields     []string // Optional fields for create operation
	validEntityDomains []string // Valid domains for delete/action validation
}

// helperTypes contains metadata for all supported helper types.
var helperTypes = map[string]helperTypeMetadata{
	"input_boolean": {
		platform:           "input_boolean",
		entityPrefix:       "input_boolean",
		supportedActions:   []string{"toggle"},
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "initial"},
		validEntityDomains: []string{"input_boolean"},
	},
	"input_number": {
		platform:           "input_number",
		entityPrefix:       "input_number",
		supportedActions:   []string{"set"},
		requiredFields:     []string{"min", "max"},
		optionalFields:     []string{"icon", "step", "initial", "mode", "unit_of_measurement"},
		validEntityDomains: []string{"input_number"},
	},
	"input_text": {
		platform:           "input_text",
		entityPrefix:       "input_text",
		supportedActions:   []string{"set"},
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "min", "max", "mode", "pattern", "initial"},
		validEntityDomains: []string{"input_text"},
	},
	"input_select": {
		platform:           "input_select",
		entityPrefix:       "input_select",
		supportedActions:   []string{"select", "set_options"},
		requiredFields:     []string{"options"},
		optionalFields:     []string{"icon", "initial"},
		validEntityDomains: []string{"input_select"},
	},
	"input_datetime": {
		platform:           "input_datetime",
		entityPrefix:       "input_datetime",
		supportedActions:   []string{"set"},
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "has_date", "has_time", "initial"},
		validEntityDomains: []string{"input_datetime"},
	},
	"input_button": {
		platform:           "input_button",
		entityPrefix:       "input_button",
		supportedActions:   []string{"press"},
		requiredFields:     []string{},
		optionalFields:     []string{"icon"},
		validEntityDomains: []string{"input_button"},
	},
	"counter": {
		platform:           "counter",
		entityPrefix:       "counter",
		supportedActions:   []string{"increment", "decrement", "reset", "set"},
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "initial", "step", "minimum", "maximum"},
		validEntityDomains: []string{"counter"},
	},
	"timer": {
		platform:           "timer",
		entityPrefix:       "timer",
		supportedActions:   []string{"start", "pause", "cancel", "finish", "change"},
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "duration", "restore"},
		validEntityDomains: []string{"timer"},
	},
	"schedule": {
		platform:           "schedule",
		entityPrefix:       "schedule",
		supportedActions:   []string{"reload"},
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"},
		validEntityDomains: []string{"schedule"},
	},
	"group": {
		platform:           "group",
		entityPrefix:       "group",
		supportedActions:   []string{"add_entities", "remove_entities", "reload"},
		requiredFields:     []string{"entities"},
		optionalFields:     []string{"icon", "all", "group_type"},
		validEntityDomains: []string{"group"},
	},
	"template_sensor": {
		platform:           platformTemplate,
		entityPrefix:       "sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"state"},
		optionalFields:     []string{"icon", "unit_of_measurement", "device_class", "state_class"},
		validEntityDomains: []string{"sensor"},
	},
	"template_binary_sensor": {
		platform:           platformTemplate,
		entityPrefix:       "binary_sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"state"},
		optionalFields:     []string{"icon", "device_class", "delay_on", "delay_off"},
		validEntityDomains: []string{"binary_sensor"},
	},
	"threshold": {
		platform:           "threshold",
		entityPrefix:       "binary_sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"entity_id"},
		optionalFields:     []string{"icon", "lower", "upper", "hysteresis", "device_class"},
		validEntityDomains: []string{"binary_sensor"},
	},
	"derivative": {
		platform:           "derivative",
		entityPrefix:       "sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"source"},
		optionalFields:     []string{"icon", "round", "time_window", "unit_time", "unit_prefix"},
		validEntityDomains: []string{"sensor"},
	},
	"integral": {
		platform:           "integration",
		entityPrefix:       "sensor",
		supportedActions:   []string{"reset"},
		requiredFields:     []string{"source"},
		optionalFields:     []string{"icon", "method", "round", "unit_time", "unit_prefix"},
		validEntityDomains: []string{"sensor"},
	},
	"utility_meter": {
		platform:           platformUtilityMeter,
		entityPrefix:       "sensor",
		supportedActions:   []string{"calibrate"},
		requiredFields:     []string{"source"},
		optionalFields:     []string{"icon", "cycle", "offset", "delta_values", "net_consumption", "periodically_resetting", "tariffs"},
		validEntityDomains: []string{"sensor", "select"},
	},
	"min_max": {
		platform:           platformMinMax,
		entityPrefix:       "sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"entity_ids"},
		optionalFields:     []string{"icon", "round_digits", "type"},
		validEntityDomains: []string{"sensor"},
	},
	"statistics": {
		platform:           platformStatistics,
		entityPrefix:       "sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"entity_id"},
		optionalFields:     []string{"icon", "state_characteristic", "sampling_size", "max_age", "percentile", "precision"},
		validEntityDomains: []string{"sensor"},
	},
	"trend": {
		platform:           platformTrend,
		entityPrefix:       "binary_sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"entity_id"},
		optionalFields:     []string{"icon", "min_gradient", "min_samples", "sample_duration", "max_samples", "invert"},
		validEntityDomains: []string{"binary_sensor"},
	},
	"random_sensor": {
		platform:           platformRandom,
		entityPrefix:       "sensor",
		supportedActions:   []string{},
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "minimum", "maximum"},
		validEntityDomains: []string{"sensor"},
	},
	"random_binary_sensor": {
		platform:           platformRandom,
		entityPrefix:       "binary_sensor",
		supportedActions:   []string{},
		requiredFields:     []string{},
		optionalFields:     []string{"icon"},
		validEntityDomains: []string{"binary_sensor"},
	},
	"filter": {
		platform:           platformFilter,
		entityPrefix:       "sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"entity_id", "filter"},
		optionalFields:     []string{"icon", "filters"},
		validEntityDomains: []string{"sensor"},
	},
	"tod": {
		platform:           platformTod,
		entityPrefix:       "binary_sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"after_time", "before_time"},
		optionalFields:     []string{"icon", "after_offset", "before_offset"},
		validEntityDomains: []string{"binary_sensor"},
	},
	"generic_thermostat": {
		platform:           platformGenericThermostat,
		entityPrefix:       "climate",
		supportedActions:   []string{},
		requiredFields:     []string{"heater_entity_id", "target_sensor_entity_id"},
		optionalFields:     []string{"icon", "ac_mode", "min_temp", "max_temp", "target_temp", "cold_tolerance", "hot_tolerance"},
		validEntityDomains: []string{"climate"},
	},
	"switch_as_x": {
		platform:           platformSwitchAsX,
		entityPrefix:       "light",
		supportedActions:   []string{},
		requiredFields:     []string{"entity_id", "target_domain"},
		optionalFields:     []string{"icon", "invert"},
		validEntityDomains: []string{"cover", "fan", "light", "lock", "siren", "valve"},
	},
	"generic_hygrostat": {
		platform:           platformGenericHygrostat,
		entityPrefix:       "humidifier",
		supportedActions:   []string{},
		requiredFields:     []string{"humidifier_entity_id", "target_sensor_entity_id"},
		optionalFields:     []string{"icon", "min_humidity", "max_humidity", "target_humidity", "dry_tolerance", "wet_tolerance"},
		validEntityDomains: []string{"humidifier"},
	},
}

// allSupportedActions lists all valid actions for the helper_action tool.
var allSupportedActions = []string{
	"toggle", "set", "increment", "decrement", "reset",
	"start", "pause", "cancel", "finish", "change",
	"press", "select", "set_options", "reload",
	"add_entities", "remove_entities", "calibrate",
}

// allHelperTypeNames lists all valid helper types for the manage_helper create action (sorted).
var allHelperTypeNames = []string{
	"counter", "derivative", "filter", "generic_hygrostat", "generic_thermostat",
	"group", "input_boolean", "input_button", "input_datetime", "input_number",
	"input_select", "input_text", "integral", "min_max", "random_binary_sensor",
	"random_sensor", "schedule", "statistics", "switch_as_x", "template_binary_sensor",
	"template_sensor", "threshold", "timer", "tod", "trend", "utility_meter",
}

// ConsolidatedHelperHandlers provides unified MCP tool handlers for all helper types.
type ConsolidatedHelperHandlers struct{}

// NewConsolidatedHelperHandlers creates a new ConsolidatedHelperHandlers instance.
func NewConsolidatedHelperHandlers() *ConsolidatedHelperHandlers {
	return &ConsolidatedHelperHandlers{}
}

// RegisterTools registers the consolidated helper tools with the registry.
func (h *ConsolidatedHelperHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageHelperTool(), h.handleManageHelper)
	registry.RegisterTool(h.helperActionTool(), h.handleHelperAction)
}

// =============================================================================
// Tool Definitions
// =============================================================================

//nolint:funlen // Large schema definition requires many properties
func (h *ConsolidatedHelperHandlers) manageHelperTool() mcp.Tool {
	typeNames := make([]string, 0, len(helperTypes))
	for name := range helperTypes {
		typeNames = append(typeNames, name)
	}

	// Build base properties
	props := map[string]mcp.JSONSchema{
		"action": {
			Type:        "string",
			Description: "Operation to perform: list, create, update, delete, or get_details",
			Enum:        []string{"list", "create", "update", "delete", "get_details"},
		},
		"format": {
			Type:        "string",
			Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
			Enum:        []string{"natural", "json"},
		},
		"verbose": {
			Type:        "boolean",
			Description: "Include additional details in list output (default: false)",
		},
		"type": {
			Type:        "string",
			Description: "Helper type (required for create): input_boolean, input_number, input_text, input_select, input_datetime, input_button, counter, timer, schedule, group, template_sensor, template_binary_sensor, threshold, derivative, integral, utility_meter, min_max, statistics, trend, random_sensor, random_binary_sensor, filter, tod, generic_thermostat, switch_as_x, generic_hygrostat",
			Enum:        typeNames,
		},
		"entity_id": {
			Type:        "string",
			Description: "Full entity ID (required for delete/get_details). Also required as source entity for threshold create.",
		},
		"id": {
			Type:        "string",
			Description: "Unique identifier without platform prefix (required for create)",
		},
		"name": {
			Type:        "string",
			Description: "Human-readable name (required for create)",
		},
		"icon": {
			Type:        "string",
			Description: "Icon for the helper (e.g., mdi:counter)",
		},
		"initial": {
			Type:        "string",
			Description: "Initial value (type depends on helper type)",
		},
		"min": {
			Type:        "number",
			Description: "Minimum value (required for input_number, optional for input_text)",
		},
		"max": {
			Type:        "number",
			Description: "Maximum value (required for input_number, optional for input_text)",
		},
		"step": {
			Type:        "number",
			Description: "Step value (input_number, counter)",
		},
		"mode": {
			Type:        "string",
			Description: "Display mode (input_number: box/slider, input_text: text/password)",
		},
		"unit_of_measurement": {
			Type:        "string",
			Description: "Unit of measurement (input_number, template_sensor)",
		},
		"pattern": {
			Type:        "string",
			Description: "Regex pattern for validation (input_text)",
		},
		"options": {
			Type:        "array",
			Description: "List of options (input_select, required)",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"has_date": {
			Type:        "boolean",
			Description: "Include date component. At least one of has_date or has_time must be true for input_datetime.",
		},
		"has_time": {
			Type:        "boolean",
			Description: "Include time component. At least one of has_date or has_time must be true for input_datetime.",
		},
		"minimum": {
			Type:        "number",
			Description: "Minimum allowed value (counter)",
		},
		"maximum": {
			Type:        "number",
			Description: "Maximum allowed value (counter)",
		},
		"duration": {
			Type:        "string",
			Description: "Default duration in HH:MM:SS format (timer)",
		},
		"restore": {
			Type:        "boolean",
			Description: "Restore state after restart (timer)",
		},
		"monday": {
			Type:        "array",
			Description: "Time blocks for Monday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"tuesday": {
			Type:        "array",
			Description: "Time blocks for Tuesday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"wednesday": {
			Type:        "array",
			Description: "Time blocks for Wednesday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"thursday": {
			Type:        "array",
			Description: "Time blocks for Thursday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"friday": {
			Type:        "array",
			Description: "Time blocks for Friday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"saturday": {
			Type:        "array",
			Description: "Time blocks for Saturday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"sunday": {
			Type:        "array",
			Description: "Time blocks for Sunday [{from, to}] (schedule)",
			Items:       &mcp.JSONSchema{Type: "object"},
		},
		"entities": {
			Type:        "array",
			Description: "Entity IDs to include (group, required)",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"all": {
			Type:        "boolean",
			Description: "Require all entities to be on for group to be on (group)",
		},
		"group_type": {
			Type:        "string",
			Description: "Explicit group type override (group, optional): sensor, binary_sensor, light, cover, switch, fan, lock. If not provided, automatically inferred from member entity domains.",
			Enum:        []string{"sensor", "binary_sensor", "light", "cover", "switch", "fan", "lock"},
		},
		"state": {
			Type:        "string",
			Description: "Jinja2 template for state (template_sensor, template_binary_sensor, required)",
		},
		"device_class": {
			Type:        "string",
			Description: "Device class (template_sensor, template_binary_sensor, threshold)",
		},
		"state_class": {
			Type:        "string",
			Description: "State class (template_sensor: measurement, total, total_increasing)",
		},
		"delay_on": {
			Type:        "string",
			Description: "Delay before turning on (template_binary_sensor)",
		},
		"delay_off": {
			Type:        "string",
			Description: "Delay before turning off (template_binary_sensor)",
		},
		"lower": {
			Type:        "number",
			Description: "Lower threshold value (threshold)",
		},
		"upper": {
			Type:        "number",
			Description: "Upper threshold value (threshold)",
		},
		"hysteresis": {
			Type:        "number",
			Description: "Hysteresis value (threshold)",
		},
		"source": {
			Type:        "string",
			Description: "Source sensor entity ID (derivative, integral, required)",
		},
		"round": {
			Type:        "number",
			Description: "Decimal places to round (derivative, integral)",
		},
		"time_window": {
			Type:        "string",
			Description: "Time window for derivative calculation (derivative)",
		},
		"unit_time": {
			Type:        "string",
			Description: "Time unit: s, min, h, d (derivative, integral)",
		},
		"unit_prefix": {
			Type:        "string",
			Description: "Unit prefix: none, k, M, G, T (derivative, integral)",
		},
		"method": {
			Type:        "string",
			Description: "Integration method: trapezoidal, left, right (integral)",
		},
	}

	// Merge extended properties for new helper types
	for k, v := range buildExtendedHelperProperties() {
		props[k] = v
	}

	return mcp.Tool{
		Name: "manage_helper",
		Description: `Manage Home Assistant helpers - list, create, delete, or get details.

Helper Types:
- Input helpers: input_boolean, input_number, input_text, input_select, input_datetime, input_button
- Stateful helpers: counter, timer, schedule
- Entity grouping: group
- Advanced helpers: template_sensor, template_binary_sensor, threshold, derivative, integral
- Utility helpers: utility_meter, min_max, statistics, trend, filter
- Random generators: random_sensor, random_binary_sensor
- Time-based: tod (Time of Day)
- Climate/Environment: generic_thermostat, generic_hygrostat
- Entity converters: switch_as_x

Actions:
- list: List all helpers (optional: format=natural|json, verbose=true|false)
- create: Create a new helper (requires type, id, name)
- update: Update an existing helper (requires entity_id; supports all helper types including Config Entry helpers via Options Flow)
- delete: Delete an existing helper (requires entity_id)
- get_details: Get helper details (requires entity_id)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper management operation",
			Properties:  props,
			Required:    []string{"action"},
		},
	}
}

func (h *ConsolidatedHelperHandlers) helperActionTool() mcp.Tool {
	return mcp.Tool{
		Name: "helper_action",
		Description: `Execute runtime actions on Home Assistant helpers.

Actions by helper type:
- input_boolean: toggle
- input_number/text/datetime: set (requires value)
- input_select: select (requires option), set_options (requires options)
- input_button: press
- counter: increment, decrement, reset, set (requires value)
- timer: start (optional duration), pause, cancel, finish, change (requires duration)
- schedule: reload
- group: add_entities, remove_entities, reload
- integral: reset`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper action parameters",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "Full entity ID of the helper",
				},
				"action": {
					Type:        "string",
					Description: "Action to perform",
					Enum:        allSupportedActions,
				},
				"value": {
					Type:        "string",
					Description: "Value for set actions (type depends on helper)",
				},
				"duration": {
					Type:        "string",
					Description: "Duration in HH:MM:SS format for timer start/change",
				},
				"option": {
					Type:        "string",
					Description: "Option to select for input_select",
				},
				"options": {
					Type:        "array",
					Description: "New options list for input_select set_options",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
				"add_entities": {
					Type:        "array",
					Description: "Entities to add to group",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
				"remove_entities": {
					Type:        "array",
					Description: "Entities to remove from group",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
			},
			Required: []string{"entity_id", "action"},
		},
	}
}

// =============================================================================
// Handler: manage_helper
// =============================================================================

func (h *ConsolidatedHelperHandlers) handleManageHelper(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case "list":
		return h.handleList(ctx, client, args)
	case "create":
		return h.handleCreate(ctx, client, args)
	case helperActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case "delete":
		return h.handleDelete(ctx, client, args)
	case "get_details":
		return h.handleGetDetails(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, create, update, delete, or get_details)", action)), nil
	}
}

func (h *ConsolidatedHelperHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	helpers, err := client.ListHelpers(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing helpers: %v", err)), nil
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)
	verbose, _ := args["verbose"].(bool)

	helperFormatter := formatter.NewHelperFormatter(format)
	opts := formatter.HelperListOptions{
		Verbose:     verbose,
		GroupByType: true,
	}

	output, err := helperFormatter.FormatList(ctx, helpers, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting helpers: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	helperType, _ := args["type"].(string)
	if helperType == "" {
		return errorResult("type is required for create action"), nil
	}

	meta, ok := helperTypes[helperType]
	if !ok {
		return errorResult(fmt.Sprintf("invalid helper type: %s (valid types: %s)", helperType, strings.Join(allHelperTypeNames, ", "))), nil
	}

	id, _ := args["id"].(string)
	if id == "" {
		return errorResult("id is required for create action"), nil
	}

	name, _ := args["name"].(string)
	if name == "" {
		return errorResult("name is required for create action"), nil
	}

	// Validate type-specific required fields
	if err := validateRequiredFields(helperType, meta, args); err != nil {
		return errorResult(err.Error()), nil
	}

	// Build config
	config, err := buildHelperConfig(helperType, name, args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	// Determine if this is a WebSocket helper or Config Entry Flow helper
	isWSHelper := !homeassistant.RequiresConfigEntryFlow(meta.platform)

	var entityID string
	if isWSHelper {
		entityID, err = h.createWSHelper(ctx, client, id, name, config, meta, helperType)
	} else {
		entityID, err = h.createConfigEntryHelper(ctx, client, id, name, config, meta, helperType)
	}

	if err != nil {
		return errorResult(err.Error()), nil
	}

	return successResult(fmt.Sprintf("%s '%s' created successfully as %s", formatHelperType(helperType), name, entityID)), nil
}

// createWSHelper creates a WebSocket-based helper, using id to control entity slug.
func (h *ConsolidatedHelperHandlers) createWSHelper(
	ctx context.Context,
	client homeassistant.Client,
	id, name string,
	config map[string]any,
	meta helperTypeMetadata,
	helperType string,
) (string, error) {
	idSlug := slugifyName(id)
	nameSlug := slugifyName(name)
	needsUpdate := idSlug != nameSlug

	if needsUpdate {
		// Override config name with id to control entity slug
		config["name"] = id
	}

	helper := homeassistant.HelperConfig{
		Platform: meta.platform,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return "", fmt.Errorf("error creating %s: %w", helperType, err)
	}

	if needsUpdate {
		// Restore display name and re-send full config: HA requires all type-specific
		// mandatory fields (e.g. min/max for input_number, has_date/has_time for
		// input_datetime) even in update calls — a name-only payload is rejected.
		config["name"] = name
		updateConfig := homeassistant.HelperConfig{
			Platform: meta.platform,
			Config:   config,
		}
		if updateErr := client.UpdateHelper(ctx, id, updateConfig); updateErr != nil {
			return "", fmt.Errorf("%s created as %s.%s, but failed to set display name: %w",
				formatHelperType(helperType), meta.entityPrefix, idSlug, updateErr)
		}
	}

	return fmt.Sprintf("%s.%s", meta.entityPrefix, idSlug), nil
}

// createConfigEntryHelper creates a Config Entry Flow helper, using name for entity slug.
// If an icon is provided in config, it will be set via Entity Registry after creation.
func (h *ConsolidatedHelperHandlers) createConfigEntryHelper(
	ctx context.Context,
	client homeassistant.Client,
	id, name string,
	config map[string]any,
	meta helperTypeMetadata,
	helperType string,
) (string, error) {
	// Extract and REMOVE icon before creation
	// Config Entry Flow doesn't support icon in create flow
	icon, hasIcon := config["icon"].(string)
	if hasIcon {
		// Remove icon from config to prevent it from being sent to Config Entry Flow
		delete(config, "icon")
	}

	helper := homeassistant.HelperConfig{
		Platform: meta.platform,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return "", fmt.Errorf("error creating %s: %w", helperType, err)
	}

	predictedSlug := slugifyName(name)
	prefix := meta.entityPrefix

	// Determine dynamic entity prefix for menu-based helpers
	switch meta.platform {
	case platformGroup:
		prefix = groupEntityPrefix(config)
	case platformRandom:
		// random helper creates sensor or binary_sensor based on type field
		if typeVal, ok := config["type"].(string); ok {
			prefix = typeVal
		}
	case platformSwitchAsX:
		// switch_as_x creates entity in target_domain
		if targetDomain, ok := config["target_domain"].(string); ok {
			prefix = targetDomain
		}
	}

	entityID := fmt.Sprintf("%s.%s", prefix, predictedSlug)

	// Set icon via Entity Registry if provided
	// Wait for entity to appear in registry before setting icon
	if hasIcon && icon != "" {
		waitForEntityAppear(ctx, client, entityID)

		updateCfg := homeassistant.EntityRegistryUpdateConfig{
			Icon: &icon,
		}
		if _, err := client.UpdateEntityRegistryEntry(ctx, entityID, updateCfg); err != nil {
			// Non-fatal: entity was created successfully, just couldn't set icon
			return entityID, fmt.Errorf("%s created as %s, but failed to set icon: %w", formatHelperType(helperType), entityID, err)
		}
	}

	return entityID, nil
}

func (h *ConsolidatedHelperHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for update action"), nil
	}

	if err := ValidateEntityID(entityID); err != nil {
		return errorResult(err.Error()), nil
	}

	// Extract platform and helper ID from entity_id
	platform, helperID := ParseHelperEntityID(entityID)
	if platform == "" || helperID == "" {
		return errorResult(fmt.Sprintf("invalid entity_id format: %s (expected format: 'domain.object_id')", entityID)), nil
	}

	// Determine helper type from platform
	// For most platforms, the helper type matches the platform
	// Special cases: sensor/binary_sensor could be template/threshold/derivative/integral/group
	helperType := platform

	// Get metadata for validation
	_, ok := helperTypes[helperType]
	var config map[string]any
	var err error

	if ok {
		// Known WebSocket helper type - use metadata-driven config builder
		updateName, _ := args["name"].(string)
		config, err = buildHelperConfig(helperType, updateName, args)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Remove name from config if it wasn't explicitly provided
		// (buildHelperConfig adds empty name by default)
		if _, hasName := args["name"]; !hasName {
			delete(config, "name")
		}
	} else {
		// Unknown helper type (sensor/binary_sensor without metadata)
		// These are Config Entry Flow helpers - build loose config
		config = buildConfigEntryUpdateConfig(platform, args)
	}

	// Create UpdateHelper request
	updateConfig := homeassistant.HelperConfig{
		Platform: platform,
		Config:   config,
	}

	// Pass the FULL entity_id, not the stripped helperID: HybridClient.UpdateHelper
	// routes config-entry helpers (template, threshold, group, ...) to the Options
	// Flow REST API by matching the registry's full entity_id. A bare id never
	// matches, so the call silently falls through to the WS "<platform>/update"
	// command, which config-entry domains (sensor, binary_sensor, ...) don't have
	// and produces "unknown_command" (issue #135).
	if err := client.UpdateHelper(ctx, entityID, updateConfig); err != nil {
		return errorResult(fmt.Sprintf("error updating helper: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Helper '%s' updated successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for delete action"), nil
	}

	if err := ValidateEntityID(entityID); err != nil {
		return errorResult(err.Error()), nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return errorResult(fmt.Sprintf("Error deleting helper: %v", err)), nil
	}

	successMsg := fmt.Sprintf("Helper '%s' deleted successfully", entityID)
	if !waitForEntityDisappear(ctx, client, entityID) {
		successMsg += " (warning: helper entity may still be visible briefly)"
	}

	return successResult(successMsg), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetails(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for get_details action"), nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	switch platform {
	case platformSchedule:
		return h.handleGetDetailsSchedule(ctx, client, args)
	case platformCounter:
		return h.handleGetDetailsCounter(ctx, client, args)
	case platformTimer:
		return h.handleGetDetailsTimer(ctx, client, args)
	case platformInputBoolean, platformInputNumber, platformInputText, platformInputSelect,
		platformInputDatetime, platformInputButton, platformGroup, platformSensorEntity, platformBinarySensorEntity,
		"climate", "humidifier", "select":
		return h.handleGetDetailsGeneric(ctx, client, args, platform)
	default:
		return errorResult(fmt.Sprintf("get_details is not supported for helper type: %s", platform)), nil
	}
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsSchedule(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting schedule state: %v", err)), nil
	}

	config, configErr := client.GetScheduleConfig(ctx, entityID)
	if configErr != nil {
		config = make(map[string]any)
	}

	timeBlocks := parseTimeBlocks(config)
	details := buildScheduleDetails(state, timeBlocks)

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatScheduleDetail(ctx, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting schedule: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsCounter(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting counter state: %v", err)), nil
	}

	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	// Extract counter-specific attributes and convert to strings for natural format
	if initial, ok := state.Attributes["initial"]; ok {
		details["initial"] = fmt.Sprintf("%v", initial)
	}
	if minimum, ok := state.Attributes["minimum"]; ok {
		details["minimum"] = fmt.Sprintf("%v", minimum)
	}
	if maximum, ok := state.Attributes["maximum"]; ok {
		details["maximum"] = fmt.Sprintf("%v", maximum)
	}
	if step, ok := state.Attributes["step"]; ok {
		details["step"] = fmt.Sprintf("%v", step)
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatCounterDetail(ctx, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting counter: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting timer state: %v", err)), nil
	}

	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	// Extract timer-specific attributes (already strings in HA)
	if duration, ok := state.Attributes["duration"]; ok {
		details["duration"] = fmt.Sprintf("%v", duration)
	}
	if remaining, ok := state.Attributes["remaining"]; ok {
		details["remaining"] = fmt.Sprintf("%v", remaining)
	}
	if finishesAt, ok := state.Attributes["finishes_at"]; ok {
		details["finishes_at"] = fmt.Sprintf("%v", finishesAt)
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatTimerDetail(ctx, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting timer: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetailsGeneric(ctx context.Context, client homeassistant.Client, args map[string]any, helperType string) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting helper state: %v", err)), nil
	}

	details := buildHelperDetails(state, helperType)

	// Enrich with template config for template sensors/binary_sensors
	if helperType == platformSensorEntity || helperType == platformBinarySensorEntity {
		enrichConfigEntryOptions(ctx, client, entityID, details)
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	helperFormatter := formatter.NewHelperFormatter(format)
	output, err := helperFormatter.FormatHelperDetail(ctx, helperType, details)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting helper: %v", err)), nil
	}

	return successResult(output), nil
}

func buildHelperDetails(state *homeassistant.Entity, helperType string) map[string]any {
	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	switch helperType {
	case platformInputBoolean:
		addInputBooleanDetails(details, state)
	case platformInputNumber:
		addInputNumberDetails(details, state)
	case platformInputText:
		addInputTextDetails(details, state)
	case platformInputSelect:
		addInputSelectDetails(details, state)
	case platformInputDatetime:
		addInputDatetimeDetails(details, state)
	case platformInputButton:
		addInputButtonDetails(details, state)
	case platformGroup:
		addGroupDetails(details, state)
	case platformSensorEntity:
		addSensorDetails(details, state)
	case platformBinarySensorEntity:
		addBinarySensorDetails(details, state)
	}

	return details
}

func addInputBooleanDetails(details map[string]any, state *homeassistant.Entity) {
	if icon, ok := state.Attributes["icon"]; ok {
		details["icon"] = icon
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputNumberDetails(details map[string]any, state *homeassistant.Entity) {
	if minVal, ok := state.Attributes["min"]; ok {
		details["min"] = fmt.Sprintf("%v", minVal)
	}
	if maxVal, ok := state.Attributes["max"]; ok {
		details["max"] = fmt.Sprintf("%v", maxVal)
	}
	if step, ok := state.Attributes["step"]; ok {
		details["step"] = fmt.Sprintf("%v", step)
	}
	if mode, ok := state.Attributes["mode"]; ok {
		details["mode"] = mode
	}
	if unit, ok := state.Attributes["unit_of_measurement"]; ok {
		details["unit_of_measurement"] = unit
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputTextDetails(details map[string]any, state *homeassistant.Entity) {
	if minVal, ok := state.Attributes["min"]; ok {
		details["min"] = fmt.Sprintf("%v", minVal)
	}
	if maxVal, ok := state.Attributes["max"]; ok {
		details["max"] = fmt.Sprintf("%v", maxVal)
	}
	if mode, ok := state.Attributes["mode"]; ok {
		details["mode"] = mode
	}
	if pattern, ok := state.Attributes["pattern"]; ok {
		details["pattern"] = pattern
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputSelectDetails(details map[string]any, state *homeassistant.Entity) {
	if options, ok := state.Attributes["options"]; ok {
		details["options"] = options
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputDatetimeDetails(details map[string]any, state *homeassistant.Entity) {
	if hasDate, ok := state.Attributes["has_date"]; ok {
		details["has_date"] = hasDate
	}
	if hasTime, ok := state.Attributes["has_time"]; ok {
		details["has_time"] = hasTime
	}
	if year, ok := state.Attributes["year"]; ok {
		details["year"] = year
	}
	if month, ok := state.Attributes["month"]; ok {
		details["month"] = month
	}
	if day, ok := state.Attributes["day"]; ok {
		details["day"] = day
	}
	if hour, ok := state.Attributes["hour"]; ok {
		details["hour"] = hour
	}
	if minute, ok := state.Attributes["minute"]; ok {
		details["minute"] = minute
	}
	if second, ok := state.Attributes["second"]; ok {
		details["second"] = second
	}
	if timestamp, ok := state.Attributes["timestamp"]; ok {
		details["timestamp"] = timestamp
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addInputButtonDetails(details map[string]any, state *homeassistant.Entity) {
	if icon, ok := state.Attributes["icon"]; ok {
		details["icon"] = icon
	}
	if editable, ok := state.Attributes["editable"]; ok {
		details["editable"] = editable
	}
}

func addGroupDetails(details map[string]any, state *homeassistant.Entity) {
	if members, ok := state.Attributes["entity_id"]; ok {
		details["members"] = members
	}
	if all, ok := state.Attributes["all"]; ok {
		details["all"] = all
	}
}

func addSensorDetails(details map[string]any, state *homeassistant.Entity) {
	if unit, ok := state.Attributes["unit_of_measurement"]; ok {
		details["unit_of_measurement"] = unit
	}
	if deviceClass, ok := state.Attributes["device_class"]; ok {
		details["device_class"] = deviceClass
	}
	if stateClass, ok := state.Attributes["state_class"]; ok {
		details["state_class"] = stateClass
	}
	if source, ok := state.Attributes["source"]; ok {
		details["source"] = source
	}
}

func addBinarySensorDetails(details map[string]any, state *homeassistant.Entity) {
	if deviceClass, ok := state.Attributes["device_class"]; ok {
		details["device_class"] = deviceClass
	}
	if sourceEntity, ok := state.Attributes["entity_id"]; ok {
		details["source_entity"] = sourceEntity
	}
	if hysteresis, ok := state.Attributes["hysteresis"]; ok {
		details["hysteresis"] = hysteresis
	}
	if sensorValue, ok := state.Attributes["sensor_value"]; ok {
		details["sensor_value"] = sensorValue
	}
}

// enrichConfigEntryOptions enriches details with config entry options for template helpers.
// Gracefully degrades: if registry lookup fails or entity is not template-based, returns silently.
func enrichConfigEntryOptions(ctx context.Context, client homeassistant.Client, entityID string, details map[string]any) {
	// Look up entity in registry
	registry, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return // Graceful degradation
	}

	// Find the entity entry
	var entry *homeassistant.EntityRegistryEntry
	for i := range registry {
		if registry[i].EntityID == entityID {
			entry = &registry[i]
			break
		}
	}

	if entry == nil || entry.Platform != platformTemplate || entry.ConfigEntryID == "" {
		return // Not a template entity or no config entry
	}

	// Fetch config entry options via Options Flow REST API
	// (WebSocket GetConfigEntry does not populate Options field)
	options, err := client.GetConfigEntryOptions(ctx, entry.ConfigEntryID)
	if err != nil {
		return // Graceful degradation
	}

	// Extract template options if present
	if len(options) > 0 {
		addTemplateOptionsToDetails(options, details)
	}
}

// addTemplateOptionsToDetails extracts template options from config entry Options.
func addTemplateOptionsToDetails(options, details map[string]any) {
	details["config_entry_type"] = platformTemplate

	if state, ok := options["state"].(string); ok && state != "" {
		details["state_template"] = state
	}
	if availability, ok := options["availability"].(string); ok && availability != "" {
		details["availability_template"] = availability
	}
	if unit, ok := options["unit_of_measurement"].(string); ok && unit != "" {
		details["unit_of_measurement"] = unit
	}
	if deviceClass, ok := options["device_class"].(string); ok && deviceClass != "" {
		details["device_class"] = deviceClass
	}
	if stateClass, ok := options["state_class"].(string); ok && stateClass != "" {
		details["state_class"] = stateClass
	}
	if delayOn, ok := options["delay_on"]; ok && delayOn != nil {
		details["delay_on"] = delayOn
	}
	if delayOff, ok := options["delay_off"]; ok && delayOff != nil {
		details["delay_off"] = delayOff
	}
}

// =============================================================================
// Handler: helper_action
// =============================================================================

func (h *ConsolidatedHelperHandlers) handleHelperAction(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required"), nil
	}

	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	// Validate action is in supported list
	if !isValidAction(action) {
		return errorResult(fmt.Sprintf("invalid action: %s (valid actions: %s)", action, strings.Join(allSupportedActions, ", "))), nil
	}

	// Get entity domain
	domain, _ := ParseHelperEntityID(entityID)
	if domain == "" {
		return errorResult(fmt.Sprintf("invalid entity_id format: %s (expected format: 'domain.object_id')", entityID)), nil
	}

	// Route to appropriate handler based on action
	return h.routeAction(ctx, client, entityID, domain, action, args)
}

func (h *ConsolidatedHelperHandlers) routeAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Simple actions (no additional args needed)
	if result, handled := h.routeSimpleAction(ctx, client, entityID, domain, action); handled {
		return result, nil
	}
	// Actions that require args
	return h.routeArgsAction(ctx, client, entityID, domain, action, args)
}

func (h *ConsolidatedHelperHandlers) routeSimpleAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string) (*mcp.ToolsCallResult, bool) {
	switch action {
	case "toggle":
		result, _ := h.handleToggle(ctx, client, entityID, domain)
		return result, true
	case "increment":
		result, _ := h.handleCounterAction(ctx, client, entityID, domain, "increment")
		return result, true
	case "decrement":
		result, _ := h.handleCounterAction(ctx, client, entityID, domain, "decrement")
		return result, true
	case "reset":
		result, _ := h.handleReset(ctx, client, entityID, domain)
		return result, true
	case "pause":
		result, _ := h.handleTimerAction(ctx, client, entityID, domain, "pause", "paused")
		return result, true
	case "cancel":
		result, _ := h.handleTimerAction(ctx, client, entityID, domain, "cancel", "canceled")
		return result, true
	case "finish":
		result, _ := h.handleTimerAction(ctx, client, entityID, domain, "finish", "finished")
		return result, true
	case "press":
		result, _ := h.handlePress(ctx, client, entityID, domain)
		return result, true
	case "reload":
		result, _ := h.handleReload(ctx, client, entityID, domain)
		return result, true
	}
	return nil, false
}

func (h *ConsolidatedHelperHandlers) routeArgsAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string, args map[string]any) (*mcp.ToolsCallResult, error) {
	switch action {
	case "set":
		return h.handleSet(ctx, client, entityID, domain, args)
	case "start":
		return h.handleTimerStart(ctx, client, entityID, domain, args)
	case "change":
		return h.handleTimerChange(ctx, client, entityID, domain, args)
	case "select":
		return h.handleSelect(ctx, client, entityID, domain, args)
	case "set_options":
		return h.handleSetOptions(ctx, client, entityID, domain, args)
	case "add_entities":
		return h.handleGroupEntities(ctx, client, entityID, domain, args, true)
	case "remove_entities":
		return h.handleGroupEntities(ctx, client, entityID, domain, args, false)
	default:
		return errorResult(fmt.Sprintf("unsupported action: %s", action)), nil
	}
}

// =============================================================================
// Action Handlers
// =============================================================================

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleToggle(ctx context.Context, client homeassistant.Client, entityID, domain string) (*mcp.ToolsCallResult, error) {
	if domain != platformInputBoolean {
		return errorResult("toggle action is only supported for input_boolean helpers"), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, domain, "toggle", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error toggling %s: %v", entityID, err)), nil
	}

	return successResult(fmt.Sprintf("'%s' toggled successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleSet(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	value, hasValue := args["value"]
	if !hasValue {
		return errorResult("value is required for set action"), nil
	}

	var serviceDomain, serviceName string
	serviceData := map[string]any{"entity_id": entityID}

	switch domain {
	case platformInputNumber:
		serviceDomain = platformInputNumber
		serviceName = serviceSetValue
		serviceData["value"] = value
	case platformInputText:
		serviceDomain = platformInputText
		serviceName = serviceSetValue
		serviceData["value"] = value
	case platformInputDatetime:
		serviceDomain = platformInputDatetime
		serviceName = "set_datetime"
		serviceData["datetime"] = value
	case platformCounter:
		serviceDomain = platformCounter
		serviceName = serviceSetValue
		if v, ok := value.(float64); ok {
			serviceData["value"] = int(v)
		} else {
			serviceData["value"] = value
		}
	default:
		return errorResult(fmt.Sprintf("set action is not supported for %s helpers", domain)), nil
	}

	if _, err := client.CallService(ctx, serviceDomain, serviceName, serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error setting value: %v", err)), nil
	}

	return successResult(fmt.Sprintf("'%s' value set successfully", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleCounterAction(ctx context.Context, client homeassistant.Client, entityID, domain, action string) (*mcp.ToolsCallResult, error) {
	if domain != platformCounter {
		return errorResult(fmt.Sprintf("%s action is only supported for counter helpers", action)), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, platformCounter, action, serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error %sing counter: %v", action, err)), nil
	}

	return successResult(fmt.Sprintf("'%s' %sed successfully", entityID, action)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleReset(ctx context.Context, client homeassistant.Client, entityID, domain string) (*mcp.ToolsCallResult, error) {
	serviceData := map[string]any{"entity_id": entityID}

	switch domain {
	case platformCounter:
		if _, err := client.CallService(ctx, platformCounter, "reset", serviceData); err != nil {
			return errorResult(fmt.Sprintf("Error resetting counter: %v", err)), nil
		}
	case platformSensorEntity:
		// Integral sensors use the "integration" service domain
		if _, err := client.CallService(ctx, platformIntegration, "reset", serviceData); err != nil {
			return errorResult(fmt.Sprintf("Error resetting integral: %v", err)), nil
		}
	default:
		return errorResult(fmt.Sprintf("reset action is not supported for %s helpers", domain)), nil
	}

	return successResult(fmt.Sprintf("'%s' reset successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleTimerStart(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformTimer {
		return errorResult("start action is only supported for timer helpers"), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if duration, ok := args["duration"].(string); ok && duration != "" {
		serviceData["duration"] = duration
	}

	if _, err := client.CallService(ctx, platformTimer, "start", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error starting timer: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Timer '%s' started successfully", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleTimerAction(ctx context.Context, client homeassistant.Client, entityID, domain, action, pastTense string) (*mcp.ToolsCallResult, error) {
	if domain != platformTimer {
		return errorResult(fmt.Sprintf("%s action is only supported for timer helpers", action)), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, platformTimer, action, serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error %sing timer: %v", action[:len(action)-1], err)), nil
	}

	return successResult(fmt.Sprintf("Timer '%s' %s successfully", entityID, pastTense)), nil
}

func (h *ConsolidatedHelperHandlers) handleTimerChange(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformTimer {
		return errorResult("change action is only supported for timer helpers"), nil
	}

	duration, ok := args["duration"].(string)
	if !ok || duration == "" {
		return errorResult("duration is required for change action"), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"duration":  duration,
	}

	if _, err := client.CallService(ctx, platformTimer, "change", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error changing timer: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Timer '%s' duration changed successfully", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handlePress(ctx context.Context, client homeassistant.Client, entityID, domain string) (*mcp.ToolsCallResult, error) {
	if domain != platformInputButton {
		return errorResult("press action is only supported for input_button helpers"), nil
	}

	serviceData := map[string]any{"entity_id": entityID}
	if _, err := client.CallService(ctx, platformInputButton, "press", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error pressing button: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Button '%s' pressed successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleSelect(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformInputSelect {
		return errorResult("select action is only supported for input_select helpers"), nil
	}

	option, ok := args["option"].(string)
	if !ok || option == "" {
		return errorResult("option is required for select action"), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"option":    option,
	}

	if _, err := client.CallService(ctx, platformInputSelect, "select_option", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error selecting option: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Option '%s' selected successfully for '%s'", option, entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleSetOptions(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any) (*mcp.ToolsCallResult, error) {
	if domain != platformInputSelect {
		return errorResult("set_options action is only supported for input_select helpers"), nil
	}

	options, ok := args["options"].([]any)
	if !ok {
		return errorResult("options parameter is required for set_options action"), nil
	}
	if len(options) == 0 {
		return errorResult("options must contain at least one value"), nil
	}

	strOptions := convertToStringSlice(options)
	if len(strOptions) == 0 {
		return errorResult("options must contain at least one string value"), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"options":   strOptions,
	}

	if _, err := client.CallService(ctx, platformInputSelect, "set_options", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error setting options: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Options updated successfully for '%s'", entityID)), nil
}

//nolint:unparam // Error return follows handler interface pattern; errors wrapped in result
func (h *ConsolidatedHelperHandlers) handleReload(ctx context.Context, client homeassistant.Client, _, domain string) (*mcp.ToolsCallResult, error) {
	var serviceDomain string
	switch domain {
	case platformSchedule:
		serviceDomain = platformSchedule
	case platformGroup:
		serviceDomain = "group"
	default:
		return errorResult(fmt.Sprintf("reload action is not supported for %s helpers", domain)), nil
	}

	serviceData := map[string]any{}
	if _, err := client.CallService(ctx, serviceDomain, "reload", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error reloading: %v", err)), nil
	}

	return successResult(fmt.Sprintf("%s reloaded successfully", capitalizeFirst(domain))), nil
}

func (h *ConsolidatedHelperHandlers) handleGroupEntities(ctx context.Context, client homeassistant.Client, entityID, domain string, args map[string]any, isAdd bool) (*mcp.ToolsCallResult, error) {
	if domain != platformGroup {
		return errorResult("add_entities/remove_entities actions are only supported for group helpers"), nil
	}

	var entities []any
	var key, verb string
	var ok bool
	if isAdd {
		key = "add_entities"
		verb = "added to"
		entities, ok = args["add_entities"].([]any)
	} else {
		key = "remove_entities"
		verb = "removed from"
		entities, ok = args["remove_entities"].([]any)
	}

	if !ok {
		return errorResult(fmt.Sprintf("%s parameter is required", key)), nil
	}
	if len(entities) == 0 {
		return errorResult(fmt.Sprintf("%s must contain at least one entity", key)), nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		key:         entities,
	}

	if _, err := client.CallService(ctx, platformGroup, "set", serviceData); err != nil {
		return errorResult(fmt.Sprintf("Error modifying group: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Entities %s '%s' successfully", verb, entityID)), nil
}

// =============================================================================
// Config Builders
// =============================================================================

// buildConfigEntryUpdateConfig builds a loose config for Config Entry helper updates.
// Extracts all recognized Config Entry fields from args.
//
//nolint:gocyclo // Routing to type-specific builders requires switch over all helper types
func buildConfigEntryUpdateConfig(platform string, args map[string]any) map[string]any {
	config := make(map[string]any)

	// Common fields
	addOptionalString(config, args, "name")
	addOptionalString(config, args, "icon")

	// Template helper fields
	addOptionalString(config, args, "state")
	addOptionalString(config, args, "source")
	addOptionalString(config, args, "unit_of_measurement")
	addOptionalString(config, args, "device_class")
	addOptionalString(config, args, "state_class")

	// Threshold helper fields
	addOptionalFloat(config, args, "lower")
	addOptionalFloat(config, args, "upper")
	addOptionalFloat(config, args, "hysteresis")

	// Derivative/Integral helper fields
	addOptionalInt(config, args, "round")
	addOptionalInt(config, args, "time_window")
	addOptionalString(config, args, "unit_time")
	addOptionalString(config, args, "unit_prefix")
	addOptionalString(config, args, "method")

	// Group helper fields
	if entities, ok := args["entities"].([]any); ok {
		config["entities"] = entities
	}
	if all, ok := args["all"].(bool); ok {
		config["all"] = all
	}
	addOptionalString(config, args, "group_type")

	// Template binary sensor fields
	if delayOn, ok := args["delay_on"].(float64); ok {
		config["delay_on"] = int(delayOn)
	}
	if delayOff, ok := args["delay_off"].(float64); ok {
		config["delay_off"] = int(delayOff)
	}

	// Add fields for extended helper types
	addExtendedConfigEntryFields(config, args, platform)

	return config
}

// configBuilderFunc is a function that builds type-specific helper configuration.
type configBuilderFunc func(config, args map[string]any) error

// helperConfigBuilders maps helper types to their configuration builders.
var helperConfigBuilders = map[string]configBuilderFunc{
	platformInputBoolean:           buildInputBooleanConfig,
	platformInputButton:            buildInputButtonConfig,
	platformInputNumber:            buildInputNumberConfig,
	platformInputText:              buildInputTextConfigWrapper,
	platformInputSelect:            buildInputSelectConfigWrapper,
	platformInputDatetime:          buildInputDatetimeConfigWrapper,
	platformCounter:                buildCounterConfig,
	platformTimer:                  buildTimerConfigWrapper,
	platformSchedule:               buildScheduleConfigWrapper,
	platformGroup:                  buildGroupConfigWrapper,
	helperTypeTemplateSensor:       buildTemplateSensorConfig,
	helperTypeTemplateBinarySensor: buildTemplateBinarySensorConfig,
	"threshold":                    buildThresholdConfigWrapper,
	"derivative":                   buildDerivativeConfigWrapper,
	"integral":                     buildIntegralConfigWrapper,
	platformUtilityMeter:           buildUtilityMeterConfig,
	platformMinMax:                 buildMinMaxConfig,
	platformStatistics:             buildStatisticsConfig,
	platformTrend:                  buildTrendConfig,
	helperTypeRandomSensor:         buildRandomSensorConfig,
	helperTypeRandomBinarySensor:   buildRandomBinarySensorConfig,
	platformFilter:                 buildFilterConfig,
	platformTod:                    buildTodConfig,
	platformGenericThermostat:      buildGenericThermostatConfig,
	platformSwitchAsX:              buildSwitchAsXConfig,
	platformGenericHygrostat:       buildGenericHygrostatConfig,
}

func buildHelperConfig(helperType, name string, args map[string]any) (map[string]any, error) {
	config := map[string]any{"name": name}
	addOptionalString(config, args, "icon")

	if builder, exists := helperConfigBuilders[helperType]; exists {
		return config, builder(config, args)
	}

	return config, nil
}

func buildInputBooleanConfig(config, args map[string]any) error {
	addOptionalBool(config, args, "initial")
	return nil
}

func buildInputButtonConfig(_, _ map[string]any) error {
	return nil
}

func buildInputNumberConfig(config, args map[string]any) error {
	addOptionalFloat(config, args, "min")
	addOptionalFloat(config, args, "max")
	addOptionalFloat(config, args, "step")
	addOptionalFloat(config, args, "initial")
	addOptionalString(config, args, "mode")
	addOptionalString(config, args, "unit_of_measurement")
	if minVal, hasMin := args["min"].(float64); hasMin {
		if maxVal, hasMax := args["max"].(float64); hasMax {
			if err := ValidateRange(minVal, maxVal, "input_number"); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildInputTextConfig(config, args map[string]any) {
	addOptionalFloat(config, args, "min")
	addOptionalFloat(config, args, "max")
	addOptionalString(config, args, "mode")
	addOptionalString(config, args, "pattern")
	addOptionalString(config, args, "initial")
}

func buildInputTextConfigWrapper(config, args map[string]any) error {
	buildInputTextConfig(config, args)
	return nil
}

func buildInputSelectConfig(config, args map[string]any) {
	addOptionalString(config, args, "initial")
	if options, ok := args["options"].([]any); ok {
		config["options"] = convertToStringSlice(options)
	}
}

func buildInputSelectConfigWrapper(config, args map[string]any) error {
	buildInputSelectConfig(config, args)
	return nil
}

func buildInputDatetimeConfig(config, args map[string]any) {
	addOptionalBool(config, args, "has_date")
	addOptionalBool(config, args, "has_time")
	addOptionalString(config, args, "initial")
}

func buildInputDatetimeConfigWrapper(config, args map[string]any) error {
	buildInputDatetimeConfig(config, args)
	return nil
}

func buildCounterConfig(config, args map[string]any) error {
	addOptionalInt(config, args, "initial")
	addOptionalInt(config, args, "step")
	addOptionalInt(config, args, "minimum")
	addOptionalInt(config, args, "maximum")
	if minVal, hasMin := args["minimum"].(float64); hasMin {
		if maxVal, hasMax := args["maximum"].(float64); hasMax {
			if err := ValidateRange(minVal, maxVal, "counter"); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildTimerConfig(config, args map[string]any) {
	addOptionalString(config, args, "duration")
	addOptionalBool(config, args, "restore")
}

func buildTimerConfigWrapper(config, args map[string]any) error {
	buildTimerConfig(config, args)
	return nil
}

func buildScheduleConfig(config, args map[string]any) {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		if daySchedule, ok := args[day].([]any); ok {
			config[day] = daySchedule
		}
	}
}

func buildScheduleConfigWrapper(config, args map[string]any) error {
	buildScheduleConfig(config, args)
	return nil
}

func buildGroupConfig(config, args map[string]any) {
	addOptionalBool(config, args, "all")
	addOptionalString(config, args, "group_type")
	if entities, ok := args["entities"].([]any); ok {
		config["entities"] = entities
	}
}

func buildGroupConfigWrapper(config, args map[string]any) error {
	buildGroupConfig(config, args)
	return nil
}

func buildTemplateSensorConfig(config, args map[string]any) error {
	config["state"] = args["state"]
	addOptionalString(config, args, "unit_of_measurement")
	addOptionalString(config, args, "state_class")
	config["template_type"] = "sensor"
	addOptionalString(config, args, "device_class")
	return nil
}

func buildTemplateBinarySensorConfig(config, args map[string]any) error {
	config["state"] = args["state"]
	addOptionalString(config, args, "delay_on")
	addOptionalString(config, args, "delay_off")
	config["template_type"] = "binary_sensor"
	addOptionalString(config, args, "device_class")
	return nil
}

func buildThresholdConfigConsolidated(config, args map[string]any) {
	addOptionalFloat(config, args, "lower")
	addOptionalFloat(config, args, "upper")
	addOptionalFloat(config, args, "hysteresis")
	addOptionalString(config, args, "device_class")
	config["entity_id"] = args["entity_id"]
}

func buildThresholdConfigWrapper(config, args map[string]any) error {
	buildThresholdConfigConsolidated(config, args)
	return nil
}

func buildDerivativeConfigConsolidated(config, args map[string]any) {
	addOptionalInt(config, args, "round")
	addOptionalString(config, args, "time_window")
	addOptionalString(config, args, "unit_time")
	addOptionalString(config, args, "unit_prefix")
	config["source"] = args["source"]
}

func buildDerivativeConfigWrapper(config, args map[string]any) error {
	buildDerivativeConfigConsolidated(config, args)
	return nil
}

func buildIntegralConfigConsolidated(config, args map[string]any) {
	addOptionalString(config, args, "method")
	addOptionalInt(config, args, "round")
	addOptionalString(config, args, "unit_time")
	addOptionalString(config, args, "unit_prefix")
	config["source"] = args["source"]
}

func buildIntegralConfigWrapper(config, args map[string]any) error {
	buildIntegralConfigConsolidated(config, args)
	return nil
}

// =============================================================================
// Validation Helpers
// =============================================================================

func validateRequiredFields(helperType string, meta helperTypeMetadata, args map[string]any) error {
	for _, field := range meta.requiredFields {
		if err := validateSingleField(field, helperType, args); err != nil {
			return err
		}
	}
	return nil
}

//nolint:gocyclo // Validation switch with many specific field types
func validateSingleField(field, helperType string, args map[string]any) error {
	switch field {
	case "options":
		opts, ok := args["options"].([]any)
		if !ok {
			return fmt.Errorf("options is required for %s and must be an array", helperType)
		}
		if len(opts) == 0 {
			return fmt.Errorf("options must be a non-empty array for %s", helperType)
		}
	case "entities":
		ents, ok := args["entities"].([]any)
		if !ok {
			return fmt.Errorf("entities is required for %s and must be an array", helperType)
		}
		if len(ents) == 0 {
			return fmt.Errorf("entities must be a non-empty array for %s", helperType)
		}
	case "state":
		state, _ := args["state"].(string)
		if state == "" {
			return fmt.Errorf("state (Jinja2 template) is required for %s", helperType)
		}
	case configKeyEntityID:
		entityID, _ := args[configKeyEntityID].(string)
		if entityID == "" {
			return fmt.Errorf("entity_id (source entity) is required for %s", helperType)
		}
	case "source":
		source, _ := args["source"].(string)
		if source == "" {
			return fmt.Errorf("source (source sensor entity ID) is required for %s", helperType)
		}
	case "entity_ids":
		entityIDs, ok := args["entity_ids"].([]any)
		if !ok {
			return fmt.Errorf("entity_ids is required for %s and must be an array", helperType)
		}
		if len(entityIDs) == 0 {
			return fmt.Errorf("entity_ids must be a non-empty array for %s", helperType)
		}
	case "min", "max":
		// Check for numeric fields (float64)
		_, ok := args[field].(float64)
		if !ok {
			return fmt.Errorf("%s must be a number for %s", field, helperType)
		}
	default:
		val, _ := args[field].(string)
		if val == "" {
			return fmt.Errorf("%s is required for %s", field, helperType)
		}
	}
	return nil
}

func isValidAction(action string) bool {
	for _, a := range allSupportedActions {
		if a == action {
			return true
		}
	}
	return false
}

// =============================================================================
// Utility Functions
// =============================================================================

func errorResult(msg string) *mcp.ToolsCallResult {
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(msg)},
		IsError: true,
	}
}

func successResult(msg string) *mcp.ToolsCallResult {
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(msg)},
	}
}

func formatHelperType(helperType string) string {
	switch helperType {
	case "template_sensor":
		return "Template sensor"
	case "template_binary_sensor":
		return "Template binary sensor"
	default:
		return strings.ReplaceAll(helperType, "_", " ")
	}
}

func addOptionalString(config, args map[string]any, key string) {
	if val, ok := args[key].(string); ok && val != "" {
		config[key] = val
	}
}

func addOptionalFloat(config, args map[string]any, key string) {
	if val, ok := args[key].(float64); ok {
		config[key] = val
	}
}

func addOptionalInt(config, args map[string]any, key string) {
	if val, ok := args[key].(float64); ok {
		config[key] = int(val)
	}
}

func addOptionalBool(config, args map[string]any, key string) {
	if val, ok := args[key].(bool); ok {
		config[key] = val
	}
}

func convertToStringSlice(arr []any) []string {
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// slugifyName converts a name to a valid helper ID using the same logic as automation IDs.
func slugifyName(name string) string {
	return generateAutomationID(name)
}

// groupEntityPrefix derives the entity prefix for group helpers based on member entity domains.
// Returns the domain of the first member entity, or "group" as fallback.
func groupEntityPrefix(config map[string]any) string {
	if entities, ok := config["entities"].([]any); ok && len(entities) > 0 {
		if first, ok := entities[0].(string); ok {
			if idx := strings.Index(first, "."); idx > 0 {
				return first[:idx]
			}
		}
	}
	return "group"
}

// =============================================================================
// Schedule Detail Helpers
// =============================================================================

// parseTimeBlocks extracts time blocks from schedule config for each day.
func parseTimeBlocks(config map[string]any) map[string][]map[string]string {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	result := make(map[string][]map[string]string)

	for _, day := range days {
		if daySchedule, ok := config[day].([]any); ok {
			if blocks := parseDaySchedule(daySchedule); len(blocks) > 0 {
				result[day] = blocks
			}
		}
	}

	return result
}

// parseDaySchedule parses time blocks for a single day.
func parseDaySchedule(daySchedule []any) []map[string]string {
	blocks := make([]map[string]string, 0, len(daySchedule))
	for _, block := range daySchedule {
		if timeBlock := parseTimeBlock(block); len(timeBlock) > 0 {
			blocks = append(blocks, timeBlock)
		}
	}
	return blocks
}

// parseTimeBlock parses a single time block with from/to times.
func parseTimeBlock(block any) map[string]string {
	blockMap, ok := block.(map[string]any)
	if !ok {
		return nil
	}
	timeBlock := make(map[string]string)
	if from, ok := blockMap["from"].(string); ok {
		timeBlock["from"] = from
	}
	if to, ok := blockMap["to"].(string); ok {
		timeBlock["to"] = to
	}
	return timeBlock
}

// buildScheduleDetails creates a formatted details object for schedule entities.
func buildScheduleDetails(state *homeassistant.Entity, timeBlocks map[string][]map[string]string) map[string]any {
	details := map[string]any{
		"entity_id":     state.EntityID,
		"state":         state.State,
		"friendly_name": state.Attributes["friendly_name"],
	}

	if len(timeBlocks) > 0 {
		details["schedule"] = timeBlocks
	}

	if nextEvent, ok := state.Attributes["next_event"].(string); ok && nextEvent != "" {
		details["next_event"] = nextEvent
	}

	return details
}

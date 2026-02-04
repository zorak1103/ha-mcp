// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	helperTypeTemplateSensor       = "template_sensor"
	helperTypeTemplateBinarySensor = "template_binary_sensor"
	serviceSetValue                = "set_value"
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
		requiredFields:     []string{},
		optionalFields:     []string{"icon", "min", "max", "step", "initial", "mode", "unit_of_measurement"},
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
		optionalFields:     []string{"icon", "all"},
		validEntityDomains: []string{"group"},
	},
	"template_sensor": {
		platform:           "template",
		entityPrefix:       "sensor",
		supportedActions:   []string{},
		requiredFields:     []string{"state"},
		optionalFields:     []string{"icon", "unit_of_measurement", "device_class", "state_class"},
		validEntityDomains: []string{"sensor"},
	},
	"template_binary_sensor": {
		platform:           "template",
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
}

// allSupportedActions lists all valid actions for the helper_action tool.
var allSupportedActions = []string{
	"toggle", "set", "increment", "decrement", "reset",
	"start", "pause", "cancel", "finish", "change",
	"press", "select", "set_options", "reload",
	"add_entities", "remove_entities",
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

	return mcp.Tool{
		Name: "manage_helper",
		Description: `Manage Home Assistant helpers - create, delete, or get details.

Helper Types:
- Input helpers: input_boolean, input_number, input_text, input_select, input_datetime, input_button
- Stateful helpers: counter, timer, schedule
- Entity grouping: group
- Advanced helpers: template_sensor, template_binary_sensor, threshold, derivative, integral

Actions:
- create: Create a new helper (requires type, id, name)
- delete: Delete an existing helper (requires entity_id)
- get_details: Get schedule details (requires entity_id, only for schedule)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: create, delete, or get_details",
					Enum:        []string{"create", "delete", "get_details"},
				},
				"type": {
					Type:        "string",
					Description: "Helper type (required for create): input_boolean, input_number, input_text, input_select, input_datetime, input_button, counter, timer, schedule, group, template_sensor, template_binary_sensor, threshold, derivative, integral",
					Enum:        typeNames,
				},
				"entity_id": {
					Type:        "string",
					Description: "Full entity ID (required for delete/get_details)",
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
					Description: "Minimum value (input_number)",
				},
				"max": {
					Type:        "number",
					Description: "Maximum value (input_number)",
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
					Description: "Include date component (input_datetime)",
				},
				"has_time": {
					Type:        "boolean",
					Description: "Include time component (input_datetime)",
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
				},
				"tuesday": {
					Type:        "array",
					Description: "Time blocks for Tuesday [{from, to}] (schedule)",
				},
				"wednesday": {
					Type:        "array",
					Description: "Time blocks for Wednesday [{from, to}] (schedule)",
				},
				"thursday": {
					Type:        "array",
					Description: "Time blocks for Thursday [{from, to}] (schedule)",
				},
				"friday": {
					Type:        "array",
					Description: "Time blocks for Friday [{from, to}] (schedule)",
				},
				"saturday": {
					Type:        "array",
					Description: "Time blocks for Saturday [{from, to}] (schedule)",
				},
				"sunday": {
					Type:        "array",
					Description: "Time blocks for Sunday [{from, to}] (schedule)",
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
			},
			Required: []string{"action"},
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
	case "create":
		return h.handleCreate(ctx, client, args)
	case "delete":
		return h.handleDelete(ctx, client, args)
	case "get_details":
		return h.handleGetDetails(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be create, delete, or get_details)", action)), nil
	}
}

func (h *ConsolidatedHelperHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	helperType, _ := args["type"].(string)
	if helperType == "" {
		return errorResult("type is required for create action"), nil
	}

	meta, ok := helperTypes[helperType]
	if !ok {
		return errorResult(fmt.Sprintf("invalid helper type: %s", helperType)), nil
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

	helper := homeassistant.HelperConfig{
		Platform: meta.platform,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return errorResult(fmt.Sprintf("Error creating %s: %v", helperType, err)), nil
	}

	entityID := fmt.Sprintf("%s.%s", meta.entityPrefix, id)
	return successResult(fmt.Sprintf("%s '%s' created successfully as %s", formatHelperType(helperType), name, entityID)), nil
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

	return successResult(fmt.Sprintf("Helper '%s' deleted successfully", entityID)), nil
}

func (h *ConsolidatedHelperHandlers) handleGetDetails(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for get_details action"), nil
	}

	// Currently only schedule supports get_details
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformSchedule {
		return errorResult("get_details is only supported for schedule entities"), nil
	}

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

	output, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting schedule: %v", err)), nil
	}

	return successResult(string(output)), nil
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
		return errorResult(fmt.Sprintf("invalid action: %s", action)), nil
	}

	// Get entity domain
	domain, _ := ParseHelperEntityID(entityID)
	if domain == "" {
		return errorResult(fmt.Sprintf("invalid entity_id format: %s", entityID)), nil
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
	if !ok || len(options) == 0 {
		return errorResult("options is required for set_options action"), nil
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
	if isAdd {
		key = "add_entities"
		verb = "added to"
		entities, _ = args["add_entities"].([]any)
	} else {
		key = "remove_entities"
		verb = "removed from"
		entities, _ = args["remove_entities"].([]any)
	}

	if len(entities) == 0 {
		return errorResult(fmt.Sprintf("%s is required for %s action", key, key)), nil
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

//nolint:gocyclo // Routing to type-specific builders requires switch over all helper types
func buildHelperConfig(helperType, name string, args map[string]any) (map[string]any, error) {
	config := map[string]any{"name": name}
	addOptionalString(config, args, "icon")

	// Route to type-specific config builders
	switch helperType {
	case platformInputBoolean, platformInputButton:
		buildInputSimpleConfig(config, args, helperType)
	case platformInputNumber:
		if err := buildInputNumberConfig(config, args); err != nil {
			return nil, err
		}
	case platformInputText:
		buildInputTextConfig(config, args)
	case platformInputSelect:
		buildInputSelectConfig(config, args)
	case platformInputDatetime:
		buildInputDatetimeConfig(config, args)
	case platformCounter:
		if err := buildCounterConfig(config, args); err != nil {
			return nil, err
		}
	case platformTimer:
		buildTimerConfig(config, args)
	case platformSchedule:
		buildScheduleConfig(config, args)
	case platformGroup:
		buildGroupConfig(config, args)
	case helperTypeTemplateSensor, helperTypeTemplateBinarySensor:
		buildTemplateConfig(config, args, helperType)
	case "threshold":
		buildThresholdConfigConsolidated(config, args)
	case "derivative":
		buildDerivativeConfigConsolidated(config, args)
	case "integral":
		buildIntegralConfigConsolidated(config, args)
	}

	return config, nil
}

func buildInputSimpleConfig(config, args map[string]any, helperType string) {
	if helperType == "input_boolean" {
		addOptionalBool(config, args, "initial")
	}
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

func buildInputSelectConfig(config, args map[string]any) {
	addOptionalString(config, args, "initial")
	if options, ok := args["options"].([]any); ok && len(options) > 0 {
		config["options"] = convertToStringSlice(options)
	}
}

func buildInputDatetimeConfig(config, args map[string]any) {
	addOptionalBool(config, args, "has_date")
	addOptionalBool(config, args, "has_time")
	addOptionalString(config, args, "initial")
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

func buildScheduleConfig(config, args map[string]any) {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		if daySchedule, ok := args[day].([]any); ok && len(daySchedule) > 0 {
			config[day] = daySchedule
		}
	}
}

func buildGroupConfig(config, args map[string]any) {
	addOptionalBool(config, args, "all")
	if entities, ok := args["entities"].([]any); ok {
		config["entities"] = entities
	}
}

func buildTemplateConfig(config, args map[string]any, helperType string) {
	config["state"] = args["state"]
	if helperType == helperTypeTemplateSensor {
		addOptionalString(config, args, "unit_of_measurement")
		addOptionalString(config, args, "state_class")
		config["template_type"] = "sensor"
	} else {
		addOptionalString(config, args, "delay_on")
		addOptionalString(config, args, "delay_off")
		config["template_type"] = "binary_sensor"
	}
	addOptionalString(config, args, "device_class")
}

func buildThresholdConfigConsolidated(config, args map[string]any) {
	addOptionalFloat(config, args, "lower")
	addOptionalFloat(config, args, "upper")
	addOptionalFloat(config, args, "hysteresis")
	addOptionalString(config, args, "device_class")
	config["entity_id"] = args["entity_id"]
}

func buildDerivativeConfigConsolidated(config, args map[string]any) {
	addOptionalInt(config, args, "round")
	addOptionalString(config, args, "time_window")
	addOptionalString(config, args, "unit_time")
	addOptionalString(config, args, "unit_prefix")
	config["source"] = args["source"]
}

func buildIntegralConfigConsolidated(config, args map[string]any) {
	addOptionalString(config, args, "method")
	addOptionalInt(config, args, "round")
	addOptionalString(config, args, "unit_time")
	addOptionalString(config, args, "unit_prefix")
	config["source"] = args["source"]
}

// =============================================================================
// Validation Helpers
// =============================================================================

func validateRequiredFields(helperType string, meta helperTypeMetadata, args map[string]any) error {
	for _, field := range meta.requiredFields {
		switch field {
		case "options":
			opts, ok := args["options"].([]any)
			if !ok || len(opts) == 0 {
				return fmt.Errorf("options is required for %s and must be a non-empty array", helperType)
			}
		case "entities":
			ents, ok := args["entities"].([]any)
			if !ok || len(ents) == 0 {
				return fmt.Errorf("entities is required for %s and must be a non-empty array", helperType)
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
		default:
			val, _ := args[field].(string)
			if val == "" {
				return fmt.Errorf("%s is required for %s", field, helperType)
			}
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

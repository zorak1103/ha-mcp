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

// Target info type constants.
const (
	targetInfoTriggers   = "triggers"
	targetInfoConditions = "conditions"
	targetInfoServices   = "services"
	targetInfoEntities   = "entities"
	targetInfoAll        = "all"
)

// ConsolidatedTargetHandlers provides consolidated handlers for Home Assistant target operations.
// This replaces the individual get_triggers_for_target, get_conditions_for_target,
// get_services_for_target, and extract_from_target tools.
type ConsolidatedTargetHandlers struct{}

// NewConsolidatedTargetHandlers creates a new ConsolidatedTargetHandlers instance.
func NewConsolidatedTargetHandlers() *ConsolidatedTargetHandlers {
	return &ConsolidatedTargetHandlers{}
}

// RegisterTools registers the consolidated analyze_target tool.
func (h *ConsolidatedTargetHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.analyzeTargetTool(), h.handleAnalyzeTarget)
}

// analyzeTargetTool returns the tool definition for the consolidated target analysis tool.
func (h *ConsolidatedTargetHandlers) analyzeTargetTool() mcp.Tool {
	return mcp.Tool{
		Name:        "analyze_target",
		Description: getAnalyzeTargetDescription(),
		InputSchema: mcp.JSONSchema{
			Type:       "object",
			Properties: getAnalyzeTargetProperties(),
			Required:   []string{"info"},
		},
	}
}

func getAnalyzeTargetDescription() string {
	return `Analyze a target (entities, devices, areas, labels) for automation capabilities.

Actions:
- info=triggers: Get applicable automation triggers
- info=conditions: Get applicable automation conditions
- info=services: Get callable services
- info=entities: Extract all referenced entities, devices, and areas
- info=all: Get all of the above combined

Examples:
- Get triggers for a light: {"info": "triggers", "entity_id": ["light.living_room"]}
- Get services for an area: {"info": "services", "area_id": ["living_room"]}
- Get all info for a device: {"info": "all", "device_id": ["device_123"]}`
}

func getAnalyzeTargetProperties() map[string]mcp.JSONSchema {
	return map[string]mcp.JSONSchema{
		"info": {
			Type:        "string",
			Enum:        []string{targetInfoTriggers, targetInfoConditions, targetInfoServices, targetInfoEntities, targetInfoAll},
			Description: "Type of information to retrieve: triggers, conditions, services, entities, or all",
		},
		"format": {
			Type:        "string",
			Enum:        []string{"natural", "json"},
			Description: "Output format: 'natural' for LLM-optimized human-readable output (default), 'json' for structured JSON",
		},
		attrEntityID: {
			Type:        "array",
			Description: "List of entity IDs (e.g., ['light.living_room', 'switch.kitchen']). A single string is accepted as a one-element list.",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"device_id": {
			Type:        "array",
			Description: "List of device IDs. A single string is accepted as a one-element list.",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"area_id": {
			Type:        "array",
			Description: "List of area IDs. A single string is accepted as a one-element list.",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"label_id": {
			Type:        "array",
			Description: "List of label IDs. A single string is accepted as a one-element list.",
			Items:       &mcp.JSONSchema{Type: "string"},
		},
		"expand_group": {
			Type:        "boolean",
			Description: "When true (default), group entities are expanded to their members",
		},
	}
}

// handleAnalyzeTarget handles the consolidated analyze_target tool.
func (h *ConsolidatedTargetHandlers) handleAnalyzeTarget(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	infoType, ok := args["info"].(string)
	if !ok || infoType == "" {
		return errorResult("info parameter is required"), nil
	}

	target, expandGroup, err := h.parseTargetParams(args)
	if err != nil {
		return errorResult(fmt.Sprintf("Invalid parameters: %v", err)), nil
	}

	// Parse format parameter
	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	switch infoType {
	case targetInfoTriggers:
		return h.handleTriggers(ctx, client, target, expandGroup, format)
	case targetInfoConditions:
		return h.handleConditions(ctx, client, target, expandGroup, format)
	case targetInfoServices:
		return h.handleServices(ctx, client, target, expandGroup, format)
	case targetInfoEntities:
		return h.handleTargetEntities(ctx, client, target, expandGroup, format)
	case targetInfoAll:
		return h.handleAllInfo(ctx, client, target, expandGroup, format)
	default:
		return errorResult(fmt.Sprintf("Invalid info %q. Must be one of: triggers, conditions, services, entities, all", infoType)), nil
	}
}

// parseTargetParams extracts target and expand_group from parameters.
// It returns the first malformed argument as an error.
func (*ConsolidatedTargetHandlers) parseTargetParams(params map[string]any) (homeassistant.Target, *bool, error) {
	var target homeassistant.Target
	for _, field := range []struct {
		key string
		dst *[]string
	}{
		{attrEntityID, &target.EntityID},
		{"device_id", &target.DeviceID},
		{"area_id", &target.AreaID},
		{"label_id", &target.LabelID},
	} {
		ids, err := parseTargetIDList(params, field.key)
		if err != nil {
			return target, nil, err
		}
		*field.dst = ids
	}

	expandGroup, err := parseExpandGroup(params)
	if err != nil {
		return target, nil, err
	}

	if len(target.EntityID) == 0 && len(target.DeviceID) == 0 &&
		len(target.AreaID) == 0 && len(target.LabelID) == 0 {
		return target, nil, fmt.Errorf("at least one of entity_id, device_id, area_id, or label_id is required")
	}

	return target, expandGroup, nil
}

// parseTargetIDList reads one target list argument. Absent placeholders (see
// isAbsentArg) yield nil; a bare string is coerced to a one-element list; an
// array must contain only non-empty strings.
func parseTargetIDList(params map[string]any, key string) ([]string, error) {
	raw, ok := params[key]
	if !ok || isAbsentArg(raw) {
		return nil, nil
	}

	switch v := raw.(type) {
	case string:
		return []string{v}, nil
	case []any:
		return parseNonEmptyStringArray(key, v)
	default:
		return nil, fmt.Errorf("%s must be a string or an array of strings, got %T", key, raw)
	}
}

// parseExpandGroup reads the optional expand_group flag; a non-boolean is an error.
func parseExpandGroup(params map[string]any) (*bool, error) {
	var expandGroup *bool // stays nil when the flag is not supplied

	raw, ok := params["expand_group"]
	if !ok || isAbsentArg(raw) {
		return expandGroup, nil
	}

	b, ok := raw.(bool)
	if !ok {
		return nil, fmt.Errorf("expand_group must be a boolean, got %T", raw)
	}
	expandGroup = &b

	return expandGroup, nil
}

// handleTriggers handles info=triggers requests.
func (h *ConsolidatedTargetHandlers) handleTriggers(
	ctx context.Context,
	client homeassistant.Client,
	target homeassistant.Target,
	expandGroup *bool,
	format formatter.Format,
) (*mcp.ToolsCallResult, error) {
	triggers, err := client.GetTriggersForTarget(ctx, target, expandGroup)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting triggers: %v", err)), nil
	}

	f := formatter.NewTargetFormatter(format)
	output, err := f.FormatTriggers(ctx, triggers)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(output),
		},
	}, nil
}

// handleConditions handles info=conditions requests.
func (h *ConsolidatedTargetHandlers) handleConditions(
	ctx context.Context,
	client homeassistant.Client,
	target homeassistant.Target,
	expandGroup *bool,
	format formatter.Format,
) (*mcp.ToolsCallResult, error) {
	conditions, err := client.GetConditionsForTarget(ctx, target, expandGroup)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting conditions: %v", err)), nil
	}

	f := formatter.NewTargetFormatter(format)
	output, err := f.FormatConditions(ctx, conditions)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(output),
		},
	}, nil
}

// handleServices handles info=services requests.
func (h *ConsolidatedTargetHandlers) handleServices(
	ctx context.Context,
	client homeassistant.Client,
	target homeassistant.Target,
	expandGroup *bool,
	format formatter.Format,
) (*mcp.ToolsCallResult, error) {
	services, err := client.GetServicesForTarget(ctx, target, expandGroup)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting services: %v", err)), nil
	}

	f := formatter.NewTargetFormatter(format)
	output, err := f.FormatServices(ctx, services)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(output),
		},
	}, nil
}

// handleTargetEntities handles info=entities requests for target analysis.
func (h *ConsolidatedTargetHandlers) handleTargetEntities(
	ctx context.Context,
	client homeassistant.Client,
	target homeassistant.Target,
	expandGroup *bool,
	format formatter.Format,
) (*mcp.ToolsCallResult, error) {
	result, err := client.ExtractFromTarget(ctx, target, expandGroup)
	if err != nil {
		return errorResult(fmt.Sprintf("Error extracting entities: %v", err)), nil
	}

	f := formatter.NewTargetFormatter(format)
	output, err := f.FormatExtractResult(ctx, result)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(output),
		},
	}, nil
}

// handleAllInfo handles info=all requests - returns all target analysis combined.
func (h *ConsolidatedTargetHandlers) handleAllInfo(
	ctx context.Context,
	client homeassistant.Client,
	target homeassistant.Target,
	expandGroup *bool,
	format formatter.Format,
) (*mcp.ToolsCallResult, error) {
	// Fetch all data
	triggers, triggersErr := client.GetTriggersForTarget(ctx, target, expandGroup)
	conditions, conditionsErr := client.GetConditionsForTarget(ctx, target, expandGroup)
	services, servicesErr := client.GetServicesForTarget(ctx, target, expandGroup)
	extracted, extractedErr := client.ExtractFromTarget(ctx, target, expandGroup)

	// For natural format, we can show partial results with errors
	if format == formatter.FormatNatural {
		var result strings.Builder
		h.formatSectionWithError(&result, "Triggers", triggers, triggersErr)
		h.formatSectionWithError(&result, "Conditions", conditions, conditionsErr)
		h.formatSectionWithError(&result, "Services", services, servicesErr)
		h.formatExtractedSectionWithError(&result, extracted, extractedErr)
		return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(result.String())}}, nil
	}

	// For JSON, errors are fatal
	if triggersErr != nil {
		return errorResult(fmt.Sprintf("Error getting triggers: %v", triggersErr)), nil
	}
	if conditionsErr != nil {
		return errorResult(fmt.Sprintf("Error getting conditions: %v", conditionsErr)), nil
	}
	if servicesErr != nil {
		return errorResult(fmt.Sprintf("Error getting services: %v", servicesErr)), nil
	}
	if extractedErr != nil {
		return errorResult(fmt.Sprintf("Error extracting entities: %v", extractedErr)), nil
	}

	f := formatter.NewTargetFormatter(format)
	output, err := f.FormatAllTargetInfo(ctx, triggers, conditions, services, extracted)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(output)}}, nil
}

func (h *ConsolidatedTargetHandlers) formatSectionWithError(result *strings.Builder, name string, items []string, err error) {
	fmt.Fprintf(result, "## %s\n\n", name)
	if err != nil {
		fmt.Fprintf(result, "Error: %v\n\n", err)
		return
	}
	fmt.Fprintf(result, "%d items:\n", len(items))
	for _, item := range items {
		fmt.Fprintf(result, "- %s\n", item)
	}
	result.WriteString("\n")
}

func (h *ConsolidatedTargetHandlers) formatExtractedSectionWithError(result *strings.Builder, extracted *homeassistant.ExtractFromTargetResult, err error) {
	result.WriteString("## Entities\n\n")
	if err != nil {
		fmt.Fprintf(result, "Error: %v\n\n", err)
		return
	}
	if extracted == nil {
		result.WriteString("No entities found.\n")
		return
	}
	fmt.Fprintf(result, "Referenced entities (%d):\n", len(extracted.ReferencedEntities))
	for _, e := range extracted.ReferencedEntities {
		fmt.Fprintf(result, "- %s\n", e)
	}
	result.WriteString("\n")

	if len(extracted.ReferencedDevices) > 0 {
		fmt.Fprintf(result, "Referenced devices (%d):\n", len(extracted.ReferencedDevices))
		for _, d := range extracted.ReferencedDevices {
			fmt.Fprintf(result, "- %s\n", d)
		}
		result.WriteString("\n")
	}

	if len(extracted.ReferencedAreas) > 0 {
		fmt.Fprintf(result, "Referenced areas (%d):\n", len(extracted.ReferencedAreas))
		for _, a := range extracted.ReferencedAreas {
			fmt.Fprintf(result, "- %s\n", a)
		}
		result.WriteString("\n")
	}

	h.formatMissingItems(extracted, result)
}

func (h *ConsolidatedTargetHandlers) formatMissingItems(
	extracted *homeassistant.ExtractFromTargetResult,
	result *strings.Builder,
) {
	hasMissing := len(extracted.MissingDevices) > 0 ||
		len(extracted.MissingAreas) > 0 ||
		len(extracted.MissingFloors) > 0 ||
		len(extracted.MissingLabels) > 0

	if !hasMissing {
		return
	}

	result.WriteString("Missing references:\n")
	if len(extracted.MissingDevices) > 0 {
		fmt.Fprintf(result, "- Devices: %v\n", extracted.MissingDevices)
	}
	if len(extracted.MissingAreas) > 0 {
		fmt.Fprintf(result, "- Areas: %v\n", extracted.MissingAreas)
	}
	if len(extracted.MissingFloors) > 0 {
		fmt.Fprintf(result, "- Floors: %v\n", extracted.MissingFloors)
	}
	if len(extracted.MissingLabels) > 0 {
		fmt.Fprintf(result, "- Labels: %v\n", extracted.MissingLabels)
	}
}

// RegisterConsolidatedTargetTools registers the consolidated analyze_target tool.
func RegisterConsolidatedTargetTools(registry *mcp.Registry) {
	h := NewConsolidatedTargetHandlers()
	h.RegisterTools(registry)
}

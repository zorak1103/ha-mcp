package formatter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Natural output messages.
const (
	// MsgNoEntitiesFound is the message for no entities found.
	MsgNoEntitiesFound   = "No entities found."
	msgNoTriggersAvail   = "No triggers available."
	msgNoConditionsAvail = "No conditions available."
	msgNoServicesAvail   = "No services available."
)

// NaturalTargetFormatter produces human-readable target analysis output.
type NaturalTargetFormatter struct{}

// NewNaturalTargetFormatter creates a new NaturalTargetFormatter.
func NewNaturalTargetFormatter() *NaturalTargetFormatter {
	return &NaturalTargetFormatter{}
}

// FormatTriggers formats applicable triggers for a target.
func (f *NaturalTargetFormatter) FormatTriggers(_ context.Context, triggers []string) (string, error) {
	if len(triggers) == 0 {
		return msgNoTriggersAvail, nil
	}
	return formatStringList("Found %d applicable triggers:\n", triggers), nil
}

// FormatConditions formats applicable conditions for a target.
func (f *NaturalTargetFormatter) FormatConditions(_ context.Context, conditions []string) (string, error) {
	if len(conditions) == 0 {
		return msgNoConditionsAvail, nil
	}
	return formatStringList("Found %d applicable conditions:\n", conditions), nil
}

// FormatServices formats callable services for a target.
func (f *NaturalTargetFormatter) FormatServices(_ context.Context, services []string) (string, error) {
	if len(services) == 0 {
		return msgNoServicesAvail, nil
	}
	return formatStringList("Found %d callable services:\n", services), nil
}

// formatStringList formats a list of strings with a header.
func formatStringList(headerFmt string, items []string) string {
	var result strings.Builder
	fmt.Fprintf(&result, headerFmt, len(items))
	for _, item := range items {
		fmt.Fprintf(&result, "- %s\n", item)
	}
	return strings.TrimSuffix(result.String(), "\n")
}

// FormatExtractResult formats entity extraction results.
func (f *NaturalTargetFormatter) FormatExtractResult(_ context.Context, result *homeassistant.ExtractFromTargetResult) (string, error) {
	if result == nil {
		return MsgNoEntitiesFound, nil
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Found %d entities, %d devices, %d areas.\n\n",
		len(result.ReferencedEntities),
		len(result.ReferencedDevices),
		len(result.ReferencedAreas))

	if len(result.ReferencedEntities) > 0 {
		out.WriteString("Entities:\n")
		for _, e := range result.ReferencedEntities {
			fmt.Fprintf(&out, "- %s\n", e)
		}
		out.WriteString("\n")
	}

	if len(result.ReferencedDevices) > 0 {
		out.WriteString("Devices:\n")
		for _, d := range result.ReferencedDevices {
			fmt.Fprintf(&out, "- %s\n", d)
		}
		out.WriteString("\n")
	}

	if len(result.ReferencedAreas) > 0 {
		out.WriteString("Areas:\n")
		for _, a := range result.ReferencedAreas {
			fmt.Fprintf(&out, "- %s\n", a)
		}
		out.WriteString("\n")
	}

	f.formatMissingItems(result, &out)

	return strings.TrimSuffix(out.String(), "\n"), nil
}

func (f *NaturalTargetFormatter) formatMissingItems(result *homeassistant.ExtractFromTargetResult, out *strings.Builder) {
	hasMissing := len(result.MissingDevices) > 0 ||
		len(result.MissingAreas) > 0 ||
		len(result.MissingFloors) > 0 ||
		len(result.MissingLabels) > 0

	if !hasMissing {
		return
	}

	out.WriteString("Missing references:\n")
	if len(result.MissingDevices) > 0 {
		fmt.Fprintf(out, "- Devices: %v\n", result.MissingDevices)
	}
	if len(result.MissingAreas) > 0 {
		fmt.Fprintf(out, "- Areas: %v\n", result.MissingAreas)
	}
	if len(result.MissingFloors) > 0 {
		fmt.Fprintf(out, "- Floors: %v\n", result.MissingFloors)
	}
	if len(result.MissingLabels) > 0 {
		fmt.Fprintf(out, "- Labels: %v\n", result.MissingLabels)
	}
}

// FormatAllTargetInfo formats all target analysis combined.
func (f *NaturalTargetFormatter) FormatAllTargetInfo(
	_ context.Context,
	triggers, conditions, services []string,
	result *homeassistant.ExtractFromTargetResult,
) (string, error) {
	var out strings.Builder

	f.formatSection(&out, "Triggers", triggers, msgNoTriggersAvail, "%d applicable triggers:\n")
	f.formatSection(&out, "Conditions", conditions, msgNoConditionsAvail, "%d applicable conditions:\n")
	f.formatSection(&out, "Services", services, msgNoServicesAvail, "%d callable services:\n")
	f.formatEntitiesSection(&out, result)

	return strings.TrimSuffix(out.String(), "\n"), nil
}

// formatSection writes a section header and list of items to the output.
func (f *NaturalTargetFormatter) formatSection(out *strings.Builder, name string, items []string, emptyMsg, countFmt string) {
	fmt.Fprintf(out, "## %s\n\n", name)
	if len(items) == 0 {
		out.WriteString(emptyMsg + "\n\n")
		return
	}
	fmt.Fprintf(out, countFmt, len(items))
	for _, item := range items {
		fmt.Fprintf(out, "- %s\n", item)
	}
	out.WriteString("\n")
}

// formatEntitiesSection writes the entities section to the output.
func (f *NaturalTargetFormatter) formatEntitiesSection(out *strings.Builder, result *homeassistant.ExtractFromTargetResult) {
	out.WriteString("## Entities\n\n")
	if result == nil || len(result.ReferencedEntities) == 0 {
		out.WriteString(MsgNoEntitiesFound + "\n")
		return
	}

	writeRefList(out, "Referenced entities", result.ReferencedEntities)
	writeRefList(out, "Referenced devices", result.ReferencedDevices)
	writeRefList(out, "Referenced areas", result.ReferencedAreas)
	f.formatMissingItems(result, out)
}

// writeRefList writes a reference list section if not empty.
func writeRefList(out *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(out, "%s (%d):\n", label, len(items))
	for _, item := range items {
		fmt.Fprintf(out, "- %s\n", item)
	}
	out.WriteString("\n")
}

// JSONTargetFormatter produces JSON output for target analysis.
type JSONTargetFormatter struct{}

// NewJSONTargetFormatter creates a new JSONTargetFormatter.
func NewJSONTargetFormatter() *JSONTargetFormatter {
	return &JSONTargetFormatter{}
}

// FormatTriggers formats triggers as JSON.
func (f *JSONTargetFormatter) FormatTriggers(_ context.Context, triggers []string) (string, error) {
	if triggers == nil {
		triggers = []string{}
	}
	data, err := json.MarshalIndent(triggers, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal triggers: %w", err)
	}
	return string(data), nil
}

// FormatConditions formats conditions as JSON.
func (f *JSONTargetFormatter) FormatConditions(_ context.Context, conditions []string) (string, error) {
	if conditions == nil {
		conditions = []string{}
	}
	data, err := json.MarshalIndent(conditions, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal conditions: %w", err)
	}
	return string(data), nil
}

// FormatServices formats services as JSON.
func (f *JSONTargetFormatter) FormatServices(_ context.Context, services []string) (string, error) {
	if services == nil {
		services = []string{}
	}
	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal services: %w", err)
	}
	return string(data), nil
}

// FormatExtractResult formats extraction result as JSON.
func (f *JSONTargetFormatter) FormatExtractResult(_ context.Context, result *homeassistant.ExtractFromTargetResult) (string, error) {
	if result == nil {
		result = &homeassistant.ExtractFromTargetResult{}
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal extract result: %w", err)
	}
	return string(data), nil
}

// FormatAllTargetInfo formats all target info as combined JSON.
func (f *JSONTargetFormatter) FormatAllTargetInfo(
	_ context.Context,
	triggers, conditions, services []string,
	result *homeassistant.ExtractFromTargetResult,
) (string, error) {
	if triggers == nil {
		triggers = []string{}
	}
	if conditions == nil {
		conditions = []string{}
	}
	if services == nil {
		services = []string{}
	}
	if result == nil {
		result = &homeassistant.ExtractFromTargetResult{}
	}

	combined := map[string]any{
		"triggers":   triggers,
		"conditions": conditions,
		"services":   services,
		"entities":   result,
	}

	data, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal target info: %w", err)
	}
	return string(data), nil
}

package formatter

import (
	"context"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestNaturalTargetFormatter_FormatTriggers_Empty(t *testing.T) {
	f := NewNaturalTargetFormatter()
	result, err := f.FormatTriggers(context.Background(), nil)
	if err != nil {
		t.Fatalf("FormatTriggers() error = %v", err)
	}
	if !strings.Contains(result, "No triggers available") {
		t.Errorf("FormatTriggers() = %q, want to contain 'No triggers available'", result)
	}
}

func TestNaturalTargetFormatter_FormatTriggers_WithItems(t *testing.T) {
	f := NewNaturalTargetFormatter()
	triggers := []string{"state", "numeric_state", "time_pattern"}

	result, err := f.FormatTriggers(context.Background(), triggers)
	if err != nil {
		t.Fatalf("FormatTriggers() error = %v", err)
	}

	if !strings.Contains(result, "3 applicable triggers") {
		t.Errorf("FormatTriggers() should contain count, got %q", result)
	}
	if !strings.Contains(result, "state") {
		t.Errorf("FormatTriggers() should contain trigger name, got %q", result)
	}
}

func TestNaturalTargetFormatter_FormatConditions_Empty(t *testing.T) {
	f := NewNaturalTargetFormatter()
	result, err := f.FormatConditions(context.Background(), nil)
	if err != nil {
		t.Fatalf("FormatConditions() error = %v", err)
	}
	if !strings.Contains(result, "No conditions available") {
		t.Errorf("FormatConditions() = %q, want to contain 'No conditions available'", result)
	}
}

func TestNaturalTargetFormatter_FormatConditions_WithItems(t *testing.T) {
	f := NewNaturalTargetFormatter()
	conditions := []string{"state", "numeric_state"}

	result, err := f.FormatConditions(context.Background(), conditions)
	if err != nil {
		t.Fatalf("FormatConditions() error = %v", err)
	}

	if !strings.Contains(result, "2 applicable conditions") {
		t.Errorf("FormatConditions() should contain count, got %q", result)
	}
}

func TestNaturalTargetFormatter_FormatServices_Empty(t *testing.T) {
	f := NewNaturalTargetFormatter()
	result, err := f.FormatServices(context.Background(), nil)
	if err != nil {
		t.Fatalf("FormatServices() error = %v", err)
	}
	if !strings.Contains(result, "No services available") {
		t.Errorf("FormatServices() = %q, want to contain 'No services available'", result)
	}
}

func TestNaturalTargetFormatter_FormatServices_WithItems(t *testing.T) {
	f := NewNaturalTargetFormatter()
	services := []string{"light.turn_on", "light.turn_off", "light.toggle"}

	result, err := f.FormatServices(context.Background(), services)
	if err != nil {
		t.Fatalf("FormatServices() error = %v", err)
	}

	if !strings.Contains(result, "3 callable services") {
		t.Errorf("FormatServices() should contain count, got %q", result)
	}
	if !strings.Contains(result, "light.turn_on") {
		t.Errorf("FormatServices() should contain service name, got %q", result)
	}
}

func TestNaturalTargetFormatter_FormatExtractResult_Nil(t *testing.T) {
	f := NewNaturalTargetFormatter()
	result, err := f.FormatExtractResult(context.Background(), nil)
	if err != nil {
		t.Fatalf("FormatExtractResult() error = %v", err)
	}
	if !strings.Contains(result, "No entities found") {
		t.Errorf("FormatExtractResult() = %q, want to contain 'No entities found'", result)
	}
}

func TestNaturalTargetFormatter_FormatExtractResult_WithData(t *testing.T) {
	f := NewNaturalTargetFormatter()
	extractResult := &homeassistant.ExtractFromTargetResult{
		ReferencedEntities: []string{"light.living_room", "switch.kitchen"},
		ReferencedDevices:  []string{"dev_123"},
		ReferencedAreas:    []string{"living_room"},
	}

	result, err := f.FormatExtractResult(context.Background(), extractResult)
	if err != nil {
		t.Fatalf("FormatExtractResult() error = %v", err)
	}

	if !strings.Contains(result, "2 entities") {
		t.Errorf("FormatExtractResult() should contain entity count, got %q", result)
	}
	if !strings.Contains(result, "light.living_room") {
		t.Errorf("FormatExtractResult() should contain entity_id, got %q", result)
	}
	if !strings.Contains(result, "dev_123") {
		t.Errorf("FormatExtractResult() should contain device_id, got %q", result)
	}
}

func TestNaturalTargetFormatter_FormatExtractResult_WithMissing(t *testing.T) {
	f := NewNaturalTargetFormatter()
	extractResult := &homeassistant.ExtractFromTargetResult{
		ReferencedEntities: []string{"light.living_room"},
		MissingDevices:     []string{"missing_dev"},
		MissingAreas:       []string{"missing_area"},
	}

	result, err := f.FormatExtractResult(context.Background(), extractResult)
	if err != nil {
		t.Fatalf("FormatExtractResult() error = %v", err)
	}

	if !strings.Contains(result, "Missing references") {
		t.Errorf("FormatExtractResult() should contain missing section, got %q", result)
	}
	if !strings.Contains(result, "missing_dev") {
		t.Errorf("FormatExtractResult() should contain missing device, got %q", result)
	}
}

func TestNaturalTargetFormatter_FormatAllTargetInfo(t *testing.T) {
	f := NewNaturalTargetFormatter()

	triggers := []string{"state", "numeric_state"}
	conditions := []string{"state"}
	services := []string{"light.turn_on", "light.turn_off"}
	extractResult := &homeassistant.ExtractFromTargetResult{
		ReferencedEntities: []string{"light.living_room"},
	}

	result, err := f.FormatAllTargetInfo(context.Background(), triggers, conditions, services, extractResult)
	if err != nil {
		t.Fatalf("FormatAllTargetInfo() error = %v", err)
	}

	// Should contain all sections
	if !strings.Contains(result, "## Triggers") {
		t.Errorf("FormatAllTargetInfo() should contain Triggers section, got %q", result)
	}
	if !strings.Contains(result, "## Conditions") {
		t.Errorf("FormatAllTargetInfo() should contain Conditions section, got %q", result)
	}
	if !strings.Contains(result, "## Services") {
		t.Errorf("FormatAllTargetInfo() should contain Services section, got %q", result)
	}
	if !strings.Contains(result, "## Entities") {
		t.Errorf("FormatAllTargetInfo() should contain Entities section, got %q", result)
	}
}

func TestJSONTargetFormatter_FormatTriggers(t *testing.T) {
	f := NewJSONTargetFormatter()
	triggers := []string{"state", "numeric_state"}

	result, err := f.FormatTriggers(context.Background(), triggers)
	if err != nil {
		t.Fatalf("FormatTriggers() error = %v", err)
	}

	if !strings.Contains(result, `"state"`) {
		t.Errorf("FormatTriggers() should contain trigger as JSON string, got %q", result)
	}
}

func TestJSONTargetFormatter_FormatConditions(t *testing.T) {
	f := NewJSONTargetFormatter()
	conditions := []string{"state"}

	result, err := f.FormatConditions(context.Background(), conditions)
	if err != nil {
		t.Fatalf("FormatConditions() error = %v", err)
	}

	if !strings.Contains(result, `"state"`) {
		t.Errorf("FormatConditions() should contain condition as JSON string, got %q", result)
	}
}

func TestJSONTargetFormatter_FormatServices(t *testing.T) {
	f := NewJSONTargetFormatter()
	services := []string{"light.turn_on"}

	result, err := f.FormatServices(context.Background(), services)
	if err != nil {
		t.Fatalf("FormatServices() error = %v", err)
	}

	if !strings.Contains(result, `"light.turn_on"`) {
		t.Errorf("FormatServices() should contain service as JSON string, got %q", result)
	}
}

func TestJSONTargetFormatter_FormatExtractResult(t *testing.T) {
	f := NewJSONTargetFormatter()
	extractResult := &homeassistant.ExtractFromTargetResult{
		ReferencedEntities: []string{"light.living_room"},
	}

	result, err := f.FormatExtractResult(context.Background(), extractResult)
	if err != nil {
		t.Fatalf("FormatExtractResult() error = %v", err)
	}

	if !strings.Contains(result, `"referenced_entities"`) {
		t.Errorf("FormatExtractResult() should contain referenced_entities JSON field, got %q", result)
	}
}

func TestJSONTargetFormatter_FormatAllTargetInfo(t *testing.T) {
	f := NewJSONTargetFormatter()

	triggers := []string{"state"}
	conditions := []string{"state"}
	services := []string{"light.turn_on"}
	extractResult := &homeassistant.ExtractFromTargetResult{
		ReferencedEntities: []string{"light.living_room"},
	}

	result, err := f.FormatAllTargetInfo(context.Background(), triggers, conditions, services, extractResult)
	if err != nil {
		t.Fatalf("FormatAllTargetInfo() error = %v", err)
	}

	// Should contain all JSON fields
	if !strings.Contains(result, `"triggers"`) {
		t.Errorf("FormatAllTargetInfo() should contain triggers JSON field, got %q", result)
	}
	if !strings.Contains(result, `"conditions"`) {
		t.Errorf("FormatAllTargetInfo() should contain conditions JSON field, got %q", result)
	}
	if !strings.Contains(result, `"services"`) {
		t.Errorf("FormatAllTargetInfo() should contain services JSON field, got %q", result)
	}
	if !strings.Contains(result, `"entities"`) {
		t.Errorf("FormatAllTargetInfo() should contain entities JSON field, got %q", result)
	}
}

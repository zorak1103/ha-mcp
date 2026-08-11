package formatter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// JSONFormatter produces structured JSON output for backward compatibility.
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSONFormatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// FormatEntity formats a single entity as JSON.
func (f *JSONFormatter) FormatEntity(_ context.Context, entity homeassistant.Entity) (string, error) {
	data, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entity: %w", err)
	}
	return string(data), nil
}

// FormatEntityCompact formats a single entity as JSON. The natural formatter's
// timestamp suffix has no JSON representation, so this delegates to FormatEntity.
func (f *JSONFormatter) FormatEntityCompact(ctx context.Context, entity homeassistant.Entity) (string, error) {
	return f.FormatEntity(ctx, entity)
}

// FormatEntities formats a list of entities as JSON.
func (f *JSONFormatter) FormatEntities(_ context.Context, entities []homeassistant.Entity, opts EntityListOptions) (string, error) {
	if opts.Verbose {
		data, err := json.MarshalIndent(entities, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal entities: %w", err)
		}
		return string(data), nil
	}

	// Compact format
	compact := toCompactEntities(entities)
	data, err := json.MarshalIndent(compact, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entities: %w", err)
	}
	return string(data), nil
}

// FormatHistory formats history entries as JSON.
func (f *JSONFormatter) FormatHistory(_ context.Context, _ string, entries []homeassistant.HistoryEntry, _ HistoryOptions) (string, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal history: %w", err)
	}
	return string(data), nil
}

// FormatServiceSuccess formats a successful service call as JSON.
func (f *JSONFormatter) FormatServiceSuccess(_ context.Context, domain, service string, targets []string, data map[string]any) (string, error) {
	result := map[string]any{
		"success":           true,
		"domain":            domain,
		"service":           service,
		"affected_entities": len(targets),
	}
	if len(targets) > 0 {
		result["entity_ids"] = targets
	}
	if len(data) > 0 {
		result["data"] = data
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(output), nil
}

// FormatError formats an error as JSON.
func (f *JSONFormatter) FormatError(_ context.Context, err error) string {
	result := map[string]any{
		"error":   true,
		"message": err.Error(),
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output)
}

// compactEntity represents a compact entity for JSON output.
type compactEntity struct {
	EntityID     string `json:"entity_id"`
	State        string `json:"state"`
	FriendlyName string `json:"friendly_name,omitempty"`
}

// toCompactEntities converts entities to compact format.
func toCompactEntities(entities []homeassistant.Entity) []compactEntity {
	compact := make([]compactEntity, 0, len(entities))
	for _, e := range entities {
		ce := compactEntity{
			EntityID: e.EntityID,
			State:    e.State,
		}
		if name, ok := e.Attributes["friendly_name"].(string); ok {
			ce.FriendlyName = name
		}
		compact = append(compact, ce)
	}
	return compact
}

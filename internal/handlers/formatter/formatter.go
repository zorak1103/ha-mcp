// Package formatter provides formatting utilities for MCP tool responses.
// It supports both natural language (LLM-optimized) and JSON output formats.
package formatter

import (
	"context"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Format represents the output format type.
type Format string

const (
	// FormatNatural produces human-readable, LLM-optimized output.
	FormatNatural Format = "natural"
	// FormatJSON produces structured JSON output (backward compatible).
	FormatJSON Format = "json"
)

// ParseFormat converts a string to Format, defaulting to FormatNatural.
func ParseFormat(s string) Format {
	switch s {
	case "json":
		return FormatJSON
	case "natural", "":
		return FormatNatural
	default:
		return FormatNatural
	}
}

// EntityListOptions configures entity list formatting.
type EntityListOptions struct {
	Verbose        bool
	IncludeSummary bool
	GroupByDomain  bool
	Limit          int
}

// HistoryOptions configures history entry formatting.
type HistoryOptions struct {
	Verbose bool
	Limit   int
}

// RegistryOptions configures registry entry formatting.
type RegistryOptions struct {
	Verbose         bool
	IncludeDisabled bool
	Limit           int
}

// TargetAnalysisOptions configures target analysis formatting.
type TargetAnalysisOptions struct {
	Verbose bool
}

// Formatter defines the interface for formatting MCP tool responses.
type Formatter interface {
	// FormatEntity formats a single entity.
	FormatEntity(ctx context.Context, entity homeassistant.Entity) (string, error)

	// FormatEntities formats a list of entities.
	FormatEntities(ctx context.Context, entities []homeassistant.Entity, opts EntityListOptions) (string, error)

	// FormatHistory formats history entries (flat list after processing).
	FormatHistory(ctx context.Context, entityID string, entries []homeassistant.HistoryEntry, opts HistoryOptions) (string, error)

	// FormatServiceSuccess formats a successful service call response.
	FormatServiceSuccess(ctx context.Context, domain, service string, targets []string, data map[string]any) (string, error)

	// FormatError formats an error response.
	FormatError(ctx context.Context, err error) string
}

// RegistryFormatter defines the interface for formatting registry responses.
type RegistryFormatter interface {
	// FormatEntityRegistry formats entity registry entries.
	FormatEntityRegistry(ctx context.Context, entries []homeassistant.EntityRegistryEntry, opts RegistryOptions) (string, error)

	// FormatDeviceRegistry formats device registry entries.
	FormatDeviceRegistry(ctx context.Context, entries []homeassistant.DeviceRegistryEntry, opts RegistryOptions) (string, error)

	// FormatAreaRegistry formats area registry entries.
	FormatAreaRegistry(ctx context.Context, entries []homeassistant.AreaRegistryEntry) (string, error)

	// FormatAllRegistries formats a combined summary of all registries.
	FormatAllRegistries(ctx context.Context, entities []homeassistant.EntityRegistryEntry,
		devices []homeassistant.DeviceRegistryEntry, areas []homeassistant.AreaRegistryEntry,
		opts RegistryOptions) (string, error)
}

// TargetFormatter defines the interface for formatting target analysis responses.
type TargetFormatter interface {
	// FormatTriggers formats applicable triggers for a target.
	FormatTriggers(ctx context.Context, triggers []string) (string, error)

	// FormatConditions formats applicable conditions for a target.
	FormatConditions(ctx context.Context, conditions []string) (string, error)

	// FormatServices formats callable services for a target.
	FormatServices(ctx context.Context, services []string) (string, error)

	// FormatExtractResult formats entity extraction results.
	FormatExtractResult(ctx context.Context, result *homeassistant.ExtractFromTargetResult) (string, error)

	// FormatAllTargetInfo formats all target analysis combined.
	FormatAllTargetInfo(ctx context.Context, triggers, conditions, services []string,
		result *homeassistant.ExtractFromTargetResult) (string, error)
}

// FormattingContext provides context for formatting operations.
type FormattingContext struct {
	// Now is the current time for relative time calculations.
	Now time.Time
	// Timezone for time formatting (optional, defaults to UTC).
	Timezone *time.Location
}

// NewFormattingContext creates a new FormattingContext with defaults.
func NewFormattingContext() *FormattingContext {
	return &FormattingContext{
		Now:      time.Now(),
		Timezone: time.UTC,
	}
}

// New creates a new Formatter for the specified format.
func New(format Format) Formatter {
	switch format {
	case FormatJSON:
		return NewJSONFormatter()
	case FormatNatural:
		return NewNaturalFormatter()
	default:
		return NewNaturalFormatter()
	}
}

// NewRegistryFormatter creates a new RegistryFormatter for the specified format.
func NewRegistryFormatter(format Format) RegistryFormatter {
	switch format {
	case FormatJSON:
		return NewJSONRegistryFormatter()
	case FormatNatural:
		return NewNaturalRegistryFormatter()
	default:
		return NewNaturalRegistryFormatter()
	}
}

// NewTargetFormatter creates a new TargetFormatter for the specified format.
func NewTargetFormatter(format Format) TargetFormatter {
	switch format {
	case FormatJSON:
		return NewJSONTargetFormatter()
	case FormatNatural:
		return NewNaturalTargetFormatter()
	default:
		return NewNaturalTargetFormatter()
	}
}

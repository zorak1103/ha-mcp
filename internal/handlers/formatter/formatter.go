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

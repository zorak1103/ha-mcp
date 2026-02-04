package formatter

import (
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Format
	}{
		{
			name:     "natural explicit",
			input:    "natural",
			expected: FormatNatural,
		},
		{
			name:     "json explicit",
			input:    "json",
			expected: FormatJSON,
		},
		{
			name:     "empty string defaults to natural",
			input:    "",
			expected: FormatNatural,
		},
		{
			name:     "unknown defaults to natural",
			input:    "xml",
			expected: FormatNatural,
		},
		{
			name:     "case sensitive - JSON not recognized",
			input:    "JSON",
			expected: FormatNatural,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		format         Format
		expectedType   string
		notExpectedNil bool
	}{
		{
			name:           "natural format creates NaturalFormatter",
			format:         FormatNatural,
			expectedType:   "*formatter.NaturalFormatter",
			notExpectedNil: true,
		},
		{
			name:           "json format creates JSONFormatter",
			format:         FormatJSON,
			expectedType:   "*formatter.JSONFormatter",
			notExpectedNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := New(tt.format)
			if result == nil {
				t.Fatal("New() returned nil")
			}

			switch tt.format {
			case FormatNatural:
				if _, ok := result.(*NaturalFormatter); !ok {
					t.Errorf("New(%v) did not return *NaturalFormatter", tt.format)
				}
			case FormatJSON:
				if _, ok := result.(*JSONFormatter); !ok {
					t.Errorf("New(%v) did not return *JSONFormatter", tt.format)
				}
			}
		})
	}
}

func TestNewFormattingContext(t *testing.T) {
	ctx := NewFormattingContext()

	if ctx == nil {
		t.Fatal("NewFormattingContext() returned nil")
	}

	if ctx.Now.IsZero() {
		t.Error("FormattingContext.Now should not be zero")
	}

	if ctx.Timezone == nil {
		t.Error("FormattingContext.Timezone should not be nil")
	}
}

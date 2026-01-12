// Package logging provides tests for the logging wrapper.
package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    slog.Level
		wantErr bool
	}{
		{
			name:    "trace lowercase",
			input:   "trace",
			want:    LevelTrace,
			wantErr: false,
		},
		{
			name:    "TRACE uppercase",
			input:   "TRACE",
			want:    LevelTrace,
			wantErr: false,
		},
		{
			name:    "debug lowercase",
			input:   "debug",
			want:    LevelDebug,
			wantErr: false,
		},
		{
			name:    "DEBUG uppercase",
			input:   "DEBUG",
			want:    LevelDebug,
			wantErr: false,
		},
		{
			name:    "info lowercase",
			input:   "info",
			want:    LevelInfo,
			wantErr: false,
		},
		{
			name:    "INFO uppercase",
			input:   "INFO",
			want:    LevelInfo,
			wantErr: false,
		},
		{
			name:    "warn lowercase",
			input:   "warn",
			want:    LevelWarn,
			wantErr: false,
		},
		{
			name:    "WARN uppercase",
			input:   "WARN",
			want:    LevelWarn,
			wantErr: false,
		},
		{
			name:    "WARNING uppercase",
			input:   "WARNING",
			want:    LevelWarn,
			wantErr: false,
		},
		{
			name:    "warning lowercase",
			input:   "warning",
			want:    LevelWarn,
			wantErr: false,
		},
		{
			name:    "error lowercase",
			input:   "error",
			want:    LevelError,
			wantErr: false,
		},
		{
			name:    "ERROR uppercase",
			input:   "ERROR",
			want:    LevelError,
			wantErr: false,
		},
		{
			name:    "with leading whitespace",
			input:   "  INFO",
			want:    LevelInfo,
			wantErr: false,
		},
		{
			name:    "with trailing whitespace",
			input:   "INFO  ",
			want:    LevelInfo,
			wantErr: false,
		},
		{
			name:    "mixed case",
			input:   "InFo",
			want:    LevelInfo,
			wantErr: false,
		},
		{
			name:    "unknown level",
			input:   "UNKNOWN",
			want:    LevelInfo,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    LevelInfo,
			wantErr: true,
		},
		{
			name:    "invalid level",
			input:   "FATAL",
			want:    LevelInfo,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseLevel(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("ParseLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		want  string
	}{
		{
			name:  "trace level",
			level: LevelTrace,
			want:  "TRACE",
		},
		{
			name:  "below trace level",
			level: slog.Level(-10),
			want:  "TRACE",
		},
		{
			name:  "debug level",
			level: LevelDebug,
			want:  "DEBUG",
		},
		{
			name:  "between trace and debug",
			level: slog.Level(-6),
			want:  "DEBUG",
		},
		{
			name:  "info level",
			level: LevelInfo,
			want:  "INFO",
		},
		{
			name:  "between debug and info",
			level: slog.Level(-2),
			want:  "INFO",
		},
		{
			name:  "warn level",
			level: LevelWarn,
			want:  "WARN",
		},
		{
			name:  "between info and warn",
			level: slog.Level(2),
			want:  "WARN",
		},
		{
			name:  "error level",
			level: LevelError,
			want:  "ERROR",
		},
		{
			name:  "above error level",
			level: slog.Level(10),
			want:  "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := LevelString(tt.level)

			if got != tt.want {
				t.Errorf("LevelString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanHandler_Enabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		handlerLevel slog.Level
		checkLevel   slog.Level
		want         bool
	}{
		{
			name:         "trace handler allows trace",
			handlerLevel: LevelTrace,
			checkLevel:   LevelTrace,
			want:         true,
		},
		{
			name:         "trace handler allows debug",
			handlerLevel: LevelTrace,
			checkLevel:   LevelDebug,
			want:         true,
		},
		{
			name:         "trace handler allows info",
			handlerLevel: LevelTrace,
			checkLevel:   LevelInfo,
			want:         true,
		},
		{
			name:         "info handler blocks trace",
			handlerLevel: LevelInfo,
			checkLevel:   LevelTrace,
			want:         false,
		},
		{
			name:         "info handler blocks debug",
			handlerLevel: LevelInfo,
			checkLevel:   LevelDebug,
			want:         false,
		},
		{
			name:         "info handler allows info",
			handlerLevel: LevelInfo,
			checkLevel:   LevelInfo,
			want:         true,
		},
		{
			name:         "info handler allows warn",
			handlerLevel: LevelInfo,
			checkLevel:   LevelWarn,
			want:         true,
		},
		{
			name:         "error handler blocks warn",
			handlerLevel: LevelError,
			checkLevel:   LevelWarn,
			want:         false,
		},
		{
			name:         "error handler allows error",
			handlerLevel: LevelError,
			checkLevel:   LevelError,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &cleanHandler{
				level: tt.handlerLevel,
				out:   &bytes.Buffer{},
			}

			got := h.Enabled(context.Background(), tt.checkLevel)

			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanHandler_Handle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		level      slog.Level
		message    string
		attrs      []slog.Attr
		wantParts  []string
		wantErr    bool
	}{
		{
			name:      "info message without attrs",
			level:     LevelInfo,
			message:   "test message",
			attrs:     nil,
			wantParts: []string{"INFO", "test message"},
			wantErr:   false,
		},
		{
			name:      "debug message with single attr",
			level:     LevelDebug,
			message:   "debug log",
			attrs:     []slog.Attr{slog.String("key", "value")},
			wantParts: []string{"DEBUG", "debug log", "key=value"},
			wantErr:   false,
		},
		{
			name:    "error message with multiple attrs",
			level:   LevelError,
			message: "error occurred",
			attrs: []slog.Attr{
				slog.String("error", "something failed"),
				slog.Int("code", 500),
			},
			wantParts: []string{"ERROR", "error occurred", "error=something failed", "code=500"},
			wantErr:   false,
		},
		{
			name:      "trace message",
			level:     LevelTrace,
			message:   "trace log",
			attrs:     nil,
			wantParts: []string{"TRACE", "trace log"},
			wantErr:   false,
		},
		{
			name:      "warn message",
			level:     LevelWarn,
			message:   "warning",
			attrs:     []slog.Attr{slog.Bool("critical", false)},
			wantParts: []string{"WARN", "warning", "critical=false"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			h := &cleanHandler{
				level: LevelTrace,
				out:   &buf,
			}

			record := slog.NewRecord(time.Date(2026, 1, 12, 20, 30, 45, 0, time.UTC), tt.level, tt.message, 0)
			for _, attr := range tt.attrs {
				record.AddAttrs(attr)
			}

			err := h.Handle(context.Background(), record)

			if (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := buf.String()

			// Verify timestamp format is present
			if !strings.Contains(output, "2026-01-12 20:30:45") {
				t.Errorf("Handle() output missing timestamp, got: %s", output)
			}

			// Verify all expected parts are present
			for _, part := range tt.wantParts {
				if !strings.Contains(output, part) {
					t.Errorf("Handle() output missing %q, got: %s", part, output)
				}
			}

			// Verify newline at end
			if !strings.HasSuffix(output, "\n") {
				t.Errorf("Handle() output should end with newline, got: %s", output)
			}
		})
	}
}

func TestCleanHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	h := &cleanHandler{
		level: LevelInfo,
		out:   &bytes.Buffer{},
	}

	newHandler := h.WithAttrs([]slog.Attr{slog.String("key", "value")})

	// Verify it returns a handler (the same one in this implementation)
	if newHandler == nil {
		t.Error("WithAttrs() returned nil")
	}

	// Verify it's the same handler (implementation returns self)
	if newHandler != h {
		t.Error("WithAttrs() should return the same handler in this implementation")
	}
}

func TestCleanHandler_WithGroup(t *testing.T) {
	t.Parallel()

	h := &cleanHandler{
		level: LevelInfo,
		out:   &bytes.Buffer{},
	}

	newHandler := h.WithGroup("testgroup")

	// Verify it returns a handler (the same one in this implementation)
	if newHandler == nil {
		t.Error("WithGroup() returned nil")
	}

	// Verify it's the same handler (implementation returns self)
	if newHandler != h {
		t.Error("WithGroup() should return the same handler in this implementation")
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
	}{
		{
			name:  "trace level logger",
			level: LevelTrace,
		},
		{
			name:  "debug level logger",
			level: LevelDebug,
		},
		{
			name:  "info level logger",
			level: LevelInfo,
		},
		{
			name:  "warn level logger",
			level: LevelWarn,
		},
		{
			name:  "error level logger",
			level: LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := New(tt.level)

			if logger == nil {
				t.Fatal("New() returned nil")
			}

			if logger.Logger == nil {
				t.Error("New() Logger.Logger is nil")
			}

			if logger.level != tt.level {
				t.Errorf("New() level = %v, want %v", logger.level, tt.level)
			}
		})
	}
}

func TestLogger_Trace(t *testing.T) {
	t.Parallel()

	// Create a logger with trace level to capture output
	// Since New() uses os.Stdout, we test via the handler directly
	var buf bytes.Buffer
	handler := &cleanHandler{
		level: LevelTrace,
		out:   &buf,
	}
	logger := &Logger{
		Logger: slog.New(handler),
		level:  LevelTrace,
	}

	logger.Trace("trace message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "TRACE") {
		t.Errorf("Trace() should log at TRACE level, got: %s", output)
	}
	if !strings.Contains(output, "trace message") {
		t.Errorf("Trace() should contain message, got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Trace() should contain attributes, got: %s", output)
	}
}

func TestLogger_IsTraceEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		want  bool
	}{
		{
			name:  "trace level enabled",
			level: LevelTrace,
			want:  true,
		},
		{
			name:  "below trace level enabled",
			level: slog.Level(-10),
			want:  true,
		},
		{
			name:  "debug level disabled",
			level: LevelDebug,
			want:  false,
		},
		{
			name:  "info level disabled",
			level: LevelInfo,
			want:  false,
		},
		{
			name:  "error level disabled",
			level: LevelError,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &Logger{
				Logger: slog.Default(),
				level:  tt.level,
			}

			got := logger.IsTraceEnabled()

			if got != tt.want {
				t.Errorf("IsTraceEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger_IsDebugEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		want  bool
	}{
		{
			name:  "trace level enabled",
			level: LevelTrace,
			want:  true,
		},
		{
			name:  "debug level enabled",
			level: LevelDebug,
			want:  true,
		},
		{
			name:  "info level disabled",
			level: LevelInfo,
			want:  false,
		},
		{
			name:  "warn level disabled",
			level: LevelWarn,
			want:  false,
		},
		{
			name:  "error level disabled",
			level: LevelError,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &Logger{
				Logger: slog.Default(),
				level:  tt.level,
			}

			got := logger.IsDebugEnabled()

			if got != tt.want {
				t.Errorf("IsDebugEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger_Level(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
	}{
		{
			name:  "trace level",
			level: LevelTrace,
		},
		{
			name:  "debug level",
			level: LevelDebug,
		},
		{
			name:  "info level",
			level: LevelInfo,
		},
		{
			name:  "warn level",
			level: LevelWarn,
		},
		{
			name:  "error level",
			level: LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &Logger{
				Logger: slog.Default(),
				level:  tt.level,
			}

			got := logger.Level()

			if got != tt.level {
				t.Errorf("Level() = %v, want %v", got, tt.level)
			}
		})
	}
}

func TestSetDefault(_ *testing.T) {
	// Note: This test modifies global state, so we don't run it in parallel
	// and we restore the default logger afterwards

	originalDefault := slog.Default()
	defer slog.SetDefault(originalDefault)

	logger := New(LevelInfo)
	SetDefault(logger)

	// Verify the default was set (we can check by comparing handlers)
	// Since we can't directly compare, we just verify no panic occurs
	// and the function executes successfully
}

func TestLevelConstants(t *testing.T) {
	t.Parallel()

	// Verify level constants are correctly defined
	tests := []struct {
		name     string
		level    slog.Level
		expected slog.Level
	}{
		{
			name:     "LevelTrace is -8",
			level:    LevelTrace,
			expected: slog.Level(-8),
		},
		{
			name:     "LevelDebug matches slog.LevelDebug",
			level:    LevelDebug,
			expected: slog.LevelDebug,
		},
		{
			name:     "LevelInfo matches slog.LevelInfo",
			level:    LevelInfo,
			expected: slog.LevelInfo,
		},
		{
			name:     "LevelWarn matches slog.LevelWarn",
			level:    LevelWarn,
			expected: slog.LevelWarn,
		},
		{
			name:     "LevelError matches slog.LevelError",
			level:    LevelError,
			expected: slog.LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tt.expected, tt.level); diff != "" {
				t.Errorf("Level constant mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// errorWriter is a writer that always returns an error for testing error paths.
type errorWriter struct{}

func (e *errorWriter) Write(_ []byte) (int, error) {
	return 0, &testWriteError{}
}

type testWriteError struct{}

func (e *testWriteError) Error() string {
	return "test write error"
}

func TestCleanHandler_Handle_WriteError(t *testing.T) {
	t.Parallel()

	h := &cleanHandler{
		level: LevelTrace,
		out:   &errorWriter{},
	}

	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	err := h.Handle(context.Background(), record)

	if err == nil {
		t.Error("Handle() should return error when write fails")
	}
}

func TestLogger_Trace_NotEnabled(t *testing.T) {
	t.Parallel()

	// Test that trace messages are filtered when level is higher
	var buf bytes.Buffer
	handler := &cleanHandler{
		level: LevelInfo, // Trace is below Info, so should be filtered
		out:   &buf,
	}
	logger := &Logger{
		Logger: slog.New(handler),
		level:  LevelInfo,
	}

	logger.Trace("should not appear")

	output := buf.String()
	if output != "" {
		t.Errorf("Trace() should not log when level is INFO, got: %s", output)
	}
}

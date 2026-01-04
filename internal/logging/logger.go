// Package logging provides a thin wrapper around log/slog with TRACE level support.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Custom log levels extending slog - re-export for convenience.
const (
	// LevelTrace is below DEBUG for very verbose logging.
	LevelTrace = slog.Level(-8)
	// LevelDebug re-exports slog.LevelDebug.
	LevelDebug = slog.LevelDebug
	// LevelInfo re-exports slog.LevelInfo.
	LevelInfo = slog.LevelInfo
	// LevelWarn re-exports slog.LevelWarn.
	LevelWarn = slog.LevelWarn
	// LevelError re-exports slog.LevelError.
	LevelError = slog.LevelError
)

// ParseLevel parses a string into a slog.Level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return LevelTrace, nil
	case "DEBUG":
		return LevelDebug, nil
	case "INFO":
		return LevelInfo, nil
	case "WARN", "WARNING":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level: %s", s)
	}
}

// LevelString returns the string representation of a log level.
func LevelString(level slog.Level) string {
	switch {
	case level <= LevelTrace:
		return "TRACE"
	case level <= LevelDebug:
		return "DEBUG"
	case level <= LevelInfo:
		return "INFO"
	case level <= LevelWarn:
		return "WARN"
	default:
		return "ERROR"
	}
}

// Logger wraps slog.Logger with convenience methods including TRACE level.
type Logger struct {
	*slog.Logger
	level slog.Level
}

// cleanHandler implements slog.Handler with a simplified log format:
// "YYYY-MM-DD HH:MM:SS LEVEL message key=value key=value..."
type cleanHandler struct {
	level slog.Level
	out   io.Writer
}

// Enabled reports whether the handler handles records at the given level.
func (h *cleanHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and writes the log record.
func (h *cleanHandler) Handle(_ context.Context, r slog.Record) error {
	// Format: "2026-01-03 20:36:42 INFO Home Assistant URL url=http://..."
	timeStr := r.Time.Format(time.DateOnly + " " + time.TimeOnly)
	levelStr := LevelString(r.Level)

	// Build the output
	var sb strings.Builder
	sb.WriteString(timeStr)
	sb.WriteString(" ")
	sb.WriteString(levelStr)
	sb.WriteString(" ")
	sb.WriteString(r.Message)

	// Append attributes
	r.Attrs(func(a slog.Attr) bool {
		sb.WriteString(" ")
		sb.WriteString(a.Key)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprintf("%v", a.Value.Any()))
		return true
	})

	sb.WriteString("\n")
	_, err := h.out.Write([]byte(sb.String()))
	return err
}

// WithAttrs returns a new handler with the given attributes.
func (h *cleanHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a new handler with the given group name.
func (h *cleanHandler) WithGroup(_ string) slog.Handler {
	return h
}

// New creates a new Logger with the specified level.
func New(level slog.Level) *Logger {
	handler := &cleanHandler{
		level: level,
		out:   os.Stdout,
	}
	return &Logger{
		Logger: slog.New(handler),
		level:  level,
	}
}

// SetDefault sets the default slog logger.
func SetDefault(logger *Logger) {
	slog.SetDefault(logger.Logger)
}

// Trace logs at TRACE level (below DEBUG).
func (l *Logger) Trace(msg string, args ...any) {
	l.Log(context.Background(), LevelTrace, msg, args...)
}

// IsTraceEnabled returns true if TRACE level is enabled.
func (l *Logger) IsTraceEnabled() bool {
	return l.level <= LevelTrace
}

// IsDebugEnabled returns true if DEBUG level is enabled.
func (l *Logger) IsDebugEnabled() bool {
	return l.level <= LevelDebug
}

// Level returns the current log level.
func (l *Logger) Level() slog.Level {
	return l.level
}

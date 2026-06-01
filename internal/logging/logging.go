package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	DefaultLevel = "info"

	LevelTrace slog.Level = -8
	LevelOff   slog.Level = 100
)

type Logger struct {
	logger *slog.Logger
	level  string
}

// NormalizeLevel returns a canonical log level.
func NormalizeLevel(raw string) (string, error) {
	_, canonical, err := ParseLevel(raw)
	return canonical, err
}

// ParseLevel parses the configured log level into slog's numeric level.
func ParseLevel(raw string) (slog.Level, string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return slog.LevelInfo, DefaultLevel, nil
	case "trace":
		return LevelTrace, "trace", nil
	case "debug":
		return slog.LevelDebug, "debug", nil
	case "info":
		return slog.LevelInfo, "info", nil
	case "warn", "warning":
		return slog.LevelWarn, "warn", nil
	case "error":
		return slog.LevelError, "error", nil
	case "off", "none", "disabled":
		return LevelOff, "off", nil
	default:
		return slog.LevelInfo, "", fmt.Errorf("log level must be one of trace, debug, info, warn, error, or off")
	}
}

// New creates a leveled text logger.
func New(out io.Writer, levelName string) (*Logger, error) {
	if out == nil {
		out = io.Discard
	}
	level, canonical, err := ParseLevel(levelName)
	if err != nil {
		return nil, err
	}
	levelVar := new(slog.LevelVar)
	levelVar.Set(level)
	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		Level:       levelVar,
		ReplaceAttr: replaceLevelAttr,
	})
	return &Logger{
		logger: slog.New(handler),
		level:  canonical,
	}, nil
}

// NewStderr creates a stderr logger, falling back to info for manually built configs.
func NewStderr(levelName string) *Logger {
	logger, err := New(os.Stderr, levelName)
	if err == nil {
		return logger
	}
	logger, _ = New(os.Stderr, DefaultLevel)
	return logger
}

// NewDiscard creates a logger that drops all records.
func NewDiscard() *Logger {
	logger, _ := New(io.Discard, "off")
	return logger
}

// replaceLevelAttr formats custom slog levels with readable names in text output.
func replaceLevelAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key != slog.LevelKey {
		return attr
	}
	level, ok := attr.Value.Any().(slog.Level)
	if !ok {
		return attr
	}
	attr.Value = slog.StringValue(LevelName(level))
	return attr
}

// LevelName returns the display name for standard and custom log levels.
func LevelName(level slog.Level) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return level.String()
	}
}

// Level returns the canonical configured level name.
func (l *Logger) Level() string {
	if l == nil {
		return "off"
	}
	return l.level
}

// IsVerbose reports whether the level enables detailed diagnostic logs.
func IsVerbose(levelName string) bool {
	level, _, err := ParseLevel(levelName)
	return err == nil && level <= slog.LevelDebug
}

// Enabled reports whether the wrapped slog logger would emit a record at the given level.
func (l *Logger) Enabled(level slog.Level) bool {
	return l != nil && l.logger != nil && l.logger.Enabled(context.Background(), level)
}

// Log emits one structured log record at the requested level.
func (l *Logger) Log(level slog.Level, msg string, args ...any) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Log(context.Background(), level, msg, args...)
}

// Trace emits a trace-level structured log record.
func (l *Logger) Trace(msg string, args ...any) {
	l.Log(LevelTrace, msg, args...)
}

// Debug emits a debug-level structured log record.
func (l *Logger) Debug(msg string, args ...any) {
	l.Log(slog.LevelDebug, msg, args...)
}

// Info emits an info-level structured log record.
func (l *Logger) Info(msg string, args ...any) {
	l.Log(slog.LevelInfo, msg, args...)
}

// Warn emits a warning-level structured log record.
func (l *Logger) Warn(msg string, args ...any) {
	l.Log(slog.LevelWarn, msg, args...)
}

// Error emits an error-level structured log record.
func (l *Logger) Error(msg string, args ...any) {
	l.Log(slog.LevelError, msg, args...)
}

// Tracef formats and emits a trace-level log record when trace logging is enabled.
func (l *Logger) Tracef(format string, args ...any) {
	if l.Enabled(LevelTrace) {
		l.Trace(fmt.Sprintf(format, args...))
	}
}

// Debugf formats and emits a debug-level log record when debug logging is enabled.
func (l *Logger) Debugf(format string, args ...any) {
	if l.Enabled(slog.LevelDebug) {
		l.Debug(fmt.Sprintf(format, args...))
	}
}

// Warnf formats and emits a warning-level log record when warning logging is enabled.
func (l *Logger) Warnf(format string, args ...any) {
	if l.Enabled(slog.LevelWarn) {
		l.Warn(fmt.Sprintf(format, args...))
	}
}

// Errorf formats and emits an error-level log record when error logging is enabled.
func (l *Logger) Errorf(format string, args ...any) {
	if l.Enabled(slog.LevelError) {
		l.Error(fmt.Sprintf(format, args...))
	}
}

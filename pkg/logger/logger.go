package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Level represents the log level.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Config holds the logger configuration.
type Config struct {
	Level  string
	Format string // "json" or "text"
	Output io.Writer
}

// Logger is a wrapper around slog.Logger.
type Logger struct {
	*slog.Logger
}

var globalLogger *Logger

func init() {
	// Initialize a default logger
	globalLogger = New(&Config{
		Level:  "info",
		Format: "text",
		Output: os.Stdout,
	})
}

// New creates a new Logger instance.
func New(cfg *Config) *Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = LevelDebug
	case "info":
		level = LevelInfo
	case "warn":
		level = LevelWarn
	case "error":
		level = LevelError
	default:
		level = LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	l := slog.New(handler)
	return &Logger{l}
}

// SetGlobal sets the global logger instance.
func SetGlobal(l *Logger) {
	globalLogger = l
}

// Get returns the global logger instance.
func Get() *Logger {
	return globalLogger
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	globalLogger.Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	globalLogger.Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	globalLogger.Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	globalLogger.Error(msg, args...)
}

// With returns a logger with the given attributes.
func With(args ...any) *Logger {
	return &Logger{globalLogger.With(args...)}
}

// Context returns a logger with the given context.
func Context(ctx context.Context) *Logger {
	// For now just return the logger, in future we can extract context fields
	return globalLogger
}

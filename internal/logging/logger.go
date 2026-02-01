package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Level represents log levels
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Config holds logging configuration
type Config struct {
	Level  Level
	Format string // "json" or "text"
	Output string // "stdout", "stderr", or file path
}

var defaultLogger *slog.Logger

// Init initializes the global logger
func Init(cfg Config) error {
	level := parseLevel(cfg.Level)

	var output io.Writer
	switch strings.ToLower(cfg.Output) {
	case "", "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		output = file
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)

	return nil
}

// parseLevel converts a string level to slog.Level
func parseLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Logger returns the default logger
func Logger() *slog.Logger {
	if defaultLogger == nil {
		// Return default if not initialized
		return slog.Default()
	}
	return defaultLogger
}

// Debug logs at debug level
func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

// Info logs at info level
func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

// Warn logs at warn level
func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

// Error logs at error level
func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

// With returns a logger with additional attributes
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

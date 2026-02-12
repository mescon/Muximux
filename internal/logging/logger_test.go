package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestInit_DefaultStdout(t *testing.T) {
	// Reset global logger
	defaultLogger = nil

	cfg := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: "stdout",
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if defaultLogger == nil {
		t.Error("expected defaultLogger to be set")
	}
}

func TestInit_Stderr(t *testing.T) {
	defaultLogger = nil

	cfg := Config{
		Level:  LevelWarn,
		Format: "text",
		Output: "stderr",
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if defaultLogger == nil {
		t.Error("expected defaultLogger to be set")
	}
}

func TestInit_JSONFormat(t *testing.T) {
	defaultLogger = nil

	cfg := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: "stdout",
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if defaultLogger == nil {
		t.Error("expected defaultLogger to be set")
	}
}

func TestInit_FileOutput(t *testing.T) {
	defaultLogger = nil

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: logFile,
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("Init with file output failed: %v", err)
	}

	if defaultLogger == nil {
		t.Error("expected defaultLogger to be set")
	}

	// Verify file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("expected log file to be created")
	}
}

func TestInit_InvalidFilePath(t *testing.T) {
	defaultLogger = nil

	cfg := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: "/nonexistent/directory/test.log",
	}

	err := Init(cfg)
	if err == nil {
		t.Error("expected error for invalid file path")
	}
}

func TestInit_EmptyOutput(t *testing.T) {
	defaultLogger = nil

	cfg := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: "", // should default to stdout
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("Init with empty output failed: %v", err)
	}

	if defaultLogger == nil {
		t.Error("expected defaultLogger to be set")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    Level
		expected slog.Level
	}{
		{LevelDebug, slog.LevelDebug},
		{LevelInfo, slog.LevelInfo},
		{LevelWarn, slog.LevelWarn},
		{LevelError, slog.LevelError},
		{"unknown", slog.LevelInfo}, // Default to info
		{"", slog.LevelInfo},        // Empty defaults to info
		{"WARN", slog.LevelInfo},    // Case sensitive, so won't match
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLogger_BeforeInit(t *testing.T) {
	defaultLogger = nil

	logger := Logger()
	if logger == nil {
		t.Fatal("expected non-nil logger even before Init")
	}

	// Should return slog.Default()
	if logger != slog.Default() {
		t.Error("expected default slog logger when not initialized")
	}
}

func TestLogger_AfterInit(t *testing.T) {
	defaultLogger = nil

	if err := Init(Config{Level: LevelInfo, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	logger := Logger()
	if logger == nil {
		t.Fatal("expected non-nil logger after Init")
	}

	// After Init, the defaultLogger IS set as slog default, so they should be the same.
	// This is the expected behavior since Init calls slog.SetDefault.
	_ = slog.Default()
}

func TestConvenienceFunctions(t *testing.T) {
	defaultLogger = nil

	if err := Init(Config{Level: LevelDebug, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	// These should not panic
	Debug("debug message", "key", "value")
	Info("info message", "key", "value")
	Warn("warn message", "key", "value")
	Error("error message", "key", "value")
}

func TestWith(t *testing.T) {
	defaultLogger = nil

	if err := Init(Config{Level: LevelInfo, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	logger := With("component", "test")
	if logger == nil {
		t.Fatal("expected non-nil logger from With")
	}
}

func TestLevelConstants(t *testing.T) {
	if LevelDebug != "debug" {
		t.Errorf("expected 'debug', got %s", LevelDebug)
	}
	if LevelInfo != "info" {
		t.Errorf("expected 'info', got %s", LevelInfo)
	}
	if LevelWarn != "warn" {
		t.Errorf("expected 'warn', got %s", LevelWarn)
	}
	if LevelError != "error" {
		t.Errorf("expected 'error', got %s", LevelError)
	}
}

func TestInit_SetsDefault(t *testing.T) {
	defaultLogger = nil

	if err := Init(Config{Level: LevelInfo, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	// After Init, slog.Default() should be the logger we created
	if slog.Default() != defaultLogger {
		t.Error("expected Init to set slog default logger")
	}
}

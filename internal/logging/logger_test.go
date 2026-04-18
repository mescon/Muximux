package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

// newTestHandler creates a BroadcastHandler backed by a LogBuffer for testing.
func newTestHandler(buf *LogBuffer) *BroadcastHandler {
	inner := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	return NewBroadcastHandler(inner, buf)
}

func TestBroadcastHandler_CapturesAttrs(t *testing.T) {
	buf := NewLogBuffer(100)
	logger := slog.New(newTestHandler(buf))

	logger.Info("App created", "app", "Plex", "source", "config")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Source != "config" {
		t.Errorf("expected source 'config', got %q", e.Source)
	}
	if e.Attrs == nil {
		t.Fatal("expected Attrs to be non-nil")
	}
	if e.Attrs["app"] != "Plex" {
		t.Errorf("expected attrs[app]='Plex', got %q", e.Attrs["app"])
	}
}

func TestBroadcastHandler_SourceNotInAttrs(t *testing.T) {
	buf := NewLogBuffer(100)
	logger := slog.New(newTestHandler(buf))

	logger.Info("test", "source", "config", "app", "Sonarr")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries[0].Attrs["source"]; ok {
		t.Error("source should not appear in Attrs map — it has its own field")
	}
}

func TestBroadcastHandler_NoExtraAttrs(t *testing.T) {
	buf := NewLogBuffer(100)
	logger := slog.New(newTestHandler(buf))

	logger.Info("Server started", "source", "server")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Attrs != nil {
		t.Errorf("expected nil Attrs when only source is present, got %v", entries[0].Attrs)
	}
}

func TestBroadcastHandler_WithAttrsPreset(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := newTestHandler(buf)
	// Simulate slog.With("source", "config", "component", "apps")
	child := handler.WithAttrs([]slog.Attr{
		slog.String("source", "config"),
		slog.String("component", "apps"),
	})
	logger := slog.New(child.(*BroadcastHandler))

	logger.Info("App deleted", "app", "Radarr")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Source != "config" {
		t.Errorf("expected source 'config', got %q", e.Source)
	}
	if e.Attrs["component"] != "apps" {
		t.Errorf("expected attrs[component]='apps', got %q", e.Attrs["component"])
	}
	if e.Attrs["app"] != "Radarr" {
		t.Errorf("expected attrs[app]='Radarr', got %q", e.Attrs["app"])
	}
}

func TestBroadcastHandler_RecordOverridesPreset(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := newTestHandler(buf)
	child := handler.WithAttrs([]slog.Attr{
		slog.String("env", "staging"),
	})
	logger := slog.New(child.(*BroadcastHandler))

	// Record-level attr with same key should override preset
	logger.Info("test", "env", "production")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Attrs["env"] != "production" {
		t.Errorf("expected record-level attr to override preset, got %q", entries[0].Attrs["env"])
	}
}

// ---------------------------------------------------------------------------
// Subscribe / Unsubscribe tests
// ---------------------------------------------------------------------------

func TestSubscribe_ReceivesNewEntries(t *testing.T) {
	buf := NewLogBuffer(100)
	ch := buf.Subscribe()
	defer buf.Unsubscribe(ch)

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "hello subscriber",
		Source:    "test",
	}
	buf.Add(entry)

	select {
	case got := <-ch:
		if got.Message != "hello subscriber" {
			t.Errorf("expected message 'hello subscriber', got %q", got.Message)
		}
		if got.Source != "test" {
			t.Errorf("expected source 'test', got %q", got.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for entry on subscriber channel")
	}
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	buf := NewLogBuffer(100)
	ch1 := buf.Subscribe()
	ch2 := buf.Subscribe()
	ch3 := buf.Subscribe()
	defer buf.Unsubscribe(ch1)
	defer buf.Unsubscribe(ch2)
	defer buf.Unsubscribe(ch3)

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "warn",
		Message:   "broadcast to all",
	}
	buf.Add(entry)

	for i, ch := range []chan LogEntry{ch1, ch2, ch3} {
		select {
		case got := <-ch:
			if got.Message != "broadcast to all" {
				t.Errorf("subscriber %d: expected 'broadcast to all', got %q", i, got.Message)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for entry", i)
		}
	}
}

func TestSubscribe_DoesNotReceiveOldEntries(t *testing.T) {
	buf := NewLogBuffer(100)

	// Add entry before subscribing
	buf.Add(LogEntry{Message: "old entry"})

	ch := buf.Subscribe()
	defer buf.Unsubscribe(ch)

	// The old entry should not appear on the channel
	select {
	case got := <-ch:
		t.Errorf("did not expect to receive old entry, got %q", got.Message)
	case <-time.After(50 * time.Millisecond):
		// Expected: no entry received
	}
}

func TestSubscribe_DropsWhenFull(t *testing.T) {
	buf := NewLogBuffer(100)
	ch := buf.Subscribe() // buffered channel with capacity 64
	defer buf.Unsubscribe(ch)

	// Fill the channel beyond capacity (64) to verify non-blocking behavior
	for i := 0; i < 100; i++ {
		buf.Add(LogEntry{Message: "flood"})
	}

	// Should not block or panic. We got 64 entries in the channel; 36 were dropped.
	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}
done:
	if received != 64 {
		t.Errorf("expected 64 buffered entries (channel capacity), got %d", received)
	}
}

func TestSubscribe_ConcurrentAddAndSubscribe(t *testing.T) {
	buf := NewLogBuffer(1000)

	var wg sync.WaitGroup

	// Spawn subscribers concurrently
	channels := make([]chan LogEntry, 5)
	for i := range channels {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			channels[idx] = buf.Subscribe()
		}(i)
	}
	wg.Wait()

	// Add entries concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf.Add(LogEntry{Message: "concurrent"})
		}()
	}
	wg.Wait()

	// Clean up
	for _, ch := range channels {
		buf.Unsubscribe(ch)
	}
}

func TestUnsubscribe_ClosesChannel(t *testing.T) {
	buf := NewLogBuffer(100)
	ch := buf.Subscribe()

	buf.Unsubscribe(ch)

	// Reading from a closed channel should return zero value + ok=false
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after Unsubscribe")
	}
}

func TestUnsubscribe_StopsReceiving(t *testing.T) {
	buf := NewLogBuffer(100)
	ch := buf.Subscribe()
	buf.Unsubscribe(ch)

	// Add an entry after unsubscribe; it should not be sent to the closed channel.
	// This must not panic.
	buf.Add(LogEntry{Message: "after unsubscribe"})

	// Drain the channel; should only get zero values from the closed channel
	drained := 0
	for range ch {
		drained++
	}
	if drained != 0 {
		t.Errorf("expected 0 entries after unsubscribe, drained %d", drained)
	}
}

func TestUnsubscribe_OnlyAffectsTarget(t *testing.T) {
	buf := NewLogBuffer(100)
	ch1 := buf.Subscribe()
	ch2 := buf.Subscribe()

	// Unsubscribe only ch1
	buf.Unsubscribe(ch1)

	entry := LogEntry{Message: "still listening"}
	buf.Add(entry)

	// ch2 should still receive entries
	select {
	case got := <-ch2:
		if got.Message != "still listening" {
			t.Errorf("expected 'still listening', got %q", got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("ch2 timed out, should still receive after ch1 unsubscribed")
	}

	buf.Unsubscribe(ch2)
}

func TestUnsubscribe_MultipleSubscribers_AllRemoved(t *testing.T) {
	buf := NewLogBuffer(100)

	channels := make([]chan LogEntry, 5)
	for i := range channels {
		channels[i] = buf.Subscribe()
	}

	for _, ch := range channels {
		buf.Unsubscribe(ch)
	}

	// After removing all subscribers, Add should not panic
	buf.Add(LogEntry{Message: "no subscribers"})

	// Verify all channels are closed
	for i, ch := range channels {
		_, ok := <-ch
		if ok {
			t.Errorf("channel %d should be closed", i)
		}
	}
}

// ---------------------------------------------------------------------------
// WithGroup tests
// ---------------------------------------------------------------------------

func TestBroadcastHandler_WithGroup_ReturnsNewHandler(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := newTestHandler(buf)

	grouped := handler.WithGroup("mygroup")
	if grouped == nil {
		t.Fatal("expected non-nil handler from WithGroup")
	}

	bh, ok := grouped.(*BroadcastHandler)
	if !ok {
		t.Fatalf("expected *BroadcastHandler, got %T", grouped)
	}

	// The grouped handler should share the same buffer
	if bh.buffer != buf {
		t.Error("expected grouped handler to share the same buffer")
	}
}

func TestBroadcastHandler_WithGroup_SharesBuffer(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := newTestHandler(buf)

	grouped := handler.WithGroup("group1")
	logger := slog.New(grouped.(*BroadcastHandler))

	logger.Info("grouped message", "key", "val")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in shared buffer, got %d", len(entries))
	}
	if entries[0].Message != "grouped message" {
		t.Errorf("expected 'grouped message', got %q", entries[0].Message)
	}
}

func TestBroadcastHandler_WithGroup_PreservesAttrs(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := newTestHandler(buf)

	// Set attrs first, then group
	withAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("source", "proxy"),
		slog.String("component", "router"),
	})
	grouped := withAttrs.(*BroadcastHandler).WithGroup("request")

	bh := grouped.(*BroadcastHandler)
	if len(bh.attrs) != 2 {
		t.Errorf("expected 2 preserved attrs, got %d", len(bh.attrs))
	}

	logger := slog.New(bh)
	logger.Info("request handled", "status", "200")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != "proxy" {
		t.Errorf("expected source 'proxy', got %q", entries[0].Source)
	}
	if entries[0].Attrs["component"] != "router" {
		t.Errorf("expected component 'router', got %q", entries[0].Attrs["component"])
	}
}

func TestBroadcastHandler_WithGroup_InnerHandlerGrouped(t *testing.T) {
	buf := NewLogBuffer(100)

	// Use a real text handler writing to a buffer so we can verify grouping
	var logOutput strings.Builder
	inner := slog.NewTextHandler(&logOutput, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewBroadcastHandler(inner, buf)

	grouped := handler.WithGroup("request")
	logger := slog.New(grouped.(*BroadcastHandler))

	logger.Info("handled", "method", "GET")

	output := logOutput.String()
	// The inner text handler should prefix attrs with the group name
	if !strings.Contains(output, "request.method=GET") {
		t.Errorf("expected grouped attr 'request.method=GET' in output, got: %s", output)
	}
}

func TestBroadcastHandler_WithGroup_Enabled(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := newTestHandler(buf)

	grouped := handler.WithGroup("test")
	bh := grouped.(*BroadcastHandler)

	// The Enabled method should delegate to the inner handler
	if !bh.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info level to be enabled")
	}
	if !bh.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected Debug level to be enabled (inner handler is LevelDebug)")
	}
}

// ---------------------------------------------------------------------------
// Buffer tests
// ---------------------------------------------------------------------------

func TestBuffer_NilBeforeInit(t *testing.T) {
	// Reset global state
	oldBuffer := buffer
	oldLogger := defaultLogger
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
	}()

	buffer = nil
	defaultLogger = nil

	got := Buffer()
	if got != nil {
		t.Error("expected Buffer() to return nil before Init")
	}
}

func TestBuffer_NonNilAfterInit(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
	}()

	buffer = nil
	defaultLogger = nil

	if err := Init(Config{Level: LevelInfo, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	got := Buffer()
	if got == nil {
		t.Fatal("expected Buffer() to return non-nil after Init")
	}

	// Verify the buffer is functional
	got.Add(LogEntry{Message: "buffer test"})
	entries := got.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "buffer test" {
		t.Errorf("expected 'buffer test', got %q", entries[0].Message)
	}
}

func TestBuffer_ReturnsGlobalBuffer(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
	}()

	buffer = nil
	defaultLogger = nil

	if err := Init(Config{Level: LevelInfo, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	b1 := Buffer()
	b2 := Buffer()
	if b1 != b2 {
		t.Error("expected Buffer() to return the same instance on consecutive calls")
	}
}

func TestBuffer_ReceivesLoggedEntries(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
	}()

	buffer = nil
	defaultLogger = nil

	if err := Init(Config{Level: LevelDebug, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	// Log through the global convenience function
	Info("integration test message", "source", "test")

	buf := Buffer()
	entries := buf.Recent(10)
	found := false
	for _, e := range entries {
		if e.Message == "integration test message" {
			found = true
			if e.Source != "test" {
				t.Errorf("expected source 'test', got %q", e.Source)
			}
			break
		}
	}
	if !found {
		t.Error("expected to find 'integration test message' in buffer")
	}
}

// ---------------------------------------------------------------------------
// SetLevel tests
// ---------------------------------------------------------------------------

func TestSetLevel_ChangesLevel(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
	}()

	buffer = nil
	defaultLogger = nil

	// Init with Info level
	if err := Init(Config{Level: LevelInfo, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	// Verify info is current level
	if levelVar.Level() != slog.LevelInfo {
		t.Fatalf("expected initial level Info, got %v", levelVar.Level())
	}

	// Change to Debug
	SetLevel(LevelDebug)
	if levelVar.Level() != slog.LevelDebug {
		t.Errorf("expected level Debug after SetLevel, got %v", levelVar.Level())
	}

	// Change to Error
	SetLevel(LevelError)
	if levelVar.Level() != slog.LevelError {
		t.Errorf("expected level Error after SetLevel, got %v", levelVar.Level())
	}

	// Change to Warn
	SetLevel(LevelWarn)
	if levelVar.Level() != slog.LevelWarn {
		t.Errorf("expected level Warn after SetLevel, got %v", levelVar.Level())
	}
}

func TestSetLevel_AllLevels(t *testing.T) {
	tests := []struct {
		level    Level
		expected slog.Level
	}{
		{LevelDebug, slog.LevelDebug},
		{LevelInfo, slog.LevelInfo},
		{LevelWarn, slog.LevelWarn},
		{LevelError, slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			SetLevel(tt.level)
			if levelVar.Level() != tt.expected {
				t.Errorf("SetLevel(%q): levelVar = %v, want %v", tt.level, levelVar.Level(), tt.expected)
			}
		})
	}
}

func TestSetLevel_UnknownDefaultsToInfo(t *testing.T) {
	SetLevel("nonexistent")
	if levelVar.Level() != slog.LevelInfo {
		t.Errorf("expected unknown level to default to Info, got %v", levelVar.Level())
	}
}

func TestSetLevel_AffectsLogging(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
	}()

	buffer = nil
	defaultLogger = nil

	if err := Init(Config{Level: LevelError, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	buf := Buffer()

	// At Error level, Info messages should be filtered
	Info("should not appear")
	entries := buf.Recent(10)
	for _, e := range entries {
		if e.Message == "should not appear" {
			t.Error("Info message should be filtered at Error level")
		}
	}

	// Change to Debug level; now Info messages should appear
	SetLevel(LevelDebug)
	Info("should appear now")

	entries = buf.Recent(10)
	found := false
	for _, e := range entries {
		if e.Message == "should appear now" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Info message should appear after SetLevel(Debug)")
	}
}

func TestSetLevel_DebugMessagesFiltered(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
	}()

	buffer = nil
	defaultLogger = nil

	if err := Init(Config{Level: LevelInfo, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	buf := Buffer()

	// Debug should be filtered at Info level
	Debug("debug filtered")
	entries := buf.Recent(10)
	for _, e := range entries {
		if e.Message == "debug filtered" {
			t.Error("Debug message should be filtered at Info level")
		}
	}

	// After lowering to Debug, debug messages should appear
	SetLevel(LevelDebug)
	Debug("debug visible")

	entries = buf.Recent(10)
	found := false
	for _, e := range entries {
		if e.Message == "debug visible" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Debug message should appear after SetLevel(Debug)")
	}
}

func TestSetRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = SetRequestID(ctx, "req-123")

	val, ok := ctx.Value(ctxRequestID).(string)
	if !ok || val != "req-123" {
		t.Errorf("expected request_id=req-123, got %q (ok=%v)", val, ok)
	}
}

func TestSetUser(t *testing.T) {
	ctx := context.Background()
	ctx = SetUser(ctx, "alice")

	val, ok := ctx.Value(ctxUser).(string)
	if !ok || val != "alice" {
		t.Errorf("expected user=alice, got %q (ok=%v)", val, ok)
	}
}

func TestFrom_NilContext(t *testing.T) {
	defaultLogger = nil
	buffer = nil
	if err := Init(Config{Level: LevelDebug, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	l := From(context.TODO()) //nolint:staticcheck // testing unenriched context
	if l == nil {
		t.Fatal("From(unenriched context) should return non-nil logger")
	}
}

func TestFrom_EmptyContext(t *testing.T) {
	defaultLogger = nil
	buffer = nil
	if err := Init(Config{Level: LevelDebug, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	l := From(context.Background())
	if l == nil {
		t.Fatal("From(background) should return non-nil logger")
	}
}

func TestFrom_WithRequestIDAndUser(t *testing.T) {
	defaultLogger = nil
	buffer = nil
	if err := Init(Config{Level: LevelDebug, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ctx = SetRequestID(ctx, "req-456")
	ctx = SetUser(ctx, "bob")

	l := From(ctx)
	l.Info("test message", "source", "test")

	buf := Buffer()
	entries := buf.Recent(10)

	var found bool
	for _, e := range entries {
		if e.Message == "test message" {
			found = true
			if e.Attrs["request_id"] != "req-456" {
				t.Errorf("expected request_id=req-456, got %q", e.Attrs["request_id"])
			}
			if e.Attrs["user"] != "bob" {
				t.Errorf("expected user=bob, got %q", e.Attrs["user"])
			}
			break
		}
	}
	if !found {
		t.Error("expected to find 'test message' in buffer")
	}
}

func TestFrom_WithRequestIDOnly(t *testing.T) {
	defaultLogger = nil
	buffer = nil
	if err := Init(Config{Level: LevelDebug, Format: "text", Output: "stdout"}); err != nil {
		t.Fatal(err)
	}

	ctx := SetRequestID(context.Background(), "req-789")

	l := From(ctx)
	l.Info("rid only")

	entries := Buffer().Recent(10)
	for _, e := range entries {
		if e.Message == "rid only" {
			if e.Attrs["request_id"] != "req-789" {
				t.Errorf("expected request_id=req-789, got %q", e.Attrs["request_id"])
			}
			if _, hasUser := e.Attrs["user"]; hasUser {
				t.Error("user attr should not be present")
			}
			return
		}
	}
	t.Error("expected to find 'rid only' in buffer")
}

// ---------------------------------------------------------------------------
// rotatingWriter tests
// ---------------------------------------------------------------------------

func TestRotatingWriter_RotatesOnSize(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(logPath, 100, 3) // rotate at 100 bytes
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()

	// Write enough to trigger rotation
	data := strings.Repeat("x", 60)
	rw.Write([]byte(data)) // 60 bytes
	rw.Write([]byte(data)) // 120 > 100 → rotates, then writes

	// .log.1 should exist with the first write's data
	rotated, err := os.ReadFile(logPath + ".1")
	if err != nil {
		t.Fatalf("expected .log.1 to exist: %v", err)
	}
	if string(rotated) != data {
		t.Errorf("rotated file should contain first write, got %d bytes", len(rotated))
	}

	// .log should contain only the second write
	current, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != data {
		t.Errorf("current log should contain second write, got %d bytes", len(current))
	}
}

func TestRotatingWriter_ShiftsFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(logPath, 50, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()

	chunk := strings.Repeat("a", 51) // each write triggers rotation

	// Write 5 times to trigger 4 rotations (first write doesn't rotate)
	for i := 0; i < 5; i++ {
		rw.Write([]byte(chunk))
	}

	// Should have .log.1, .log.2, .log.3 but NOT .log.4
	for i := 1; i <= 3; i++ {
		path := fmt.Sprintf("%s.%d", logPath, i)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", path)
		}
	}
	path4 := logPath + ".4"
	if _, err := os.Stat(path4); !os.IsNotExist(err) {
		t.Errorf("expected %s to NOT exist (max 3 rotated files)", path4)
	}
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(logPath, 500, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rw.Write([]byte("concurrent write\n"))
		}()
	}
	wg.Wait()

	// Verify no panic and some data was written
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected some data in log file after concurrent writes")
	}
}

// ---------------------------------------------------------------------------
// parseTextLogLine tests
// ---------------------------------------------------------------------------

func TestParseTextLogLine_Valid(t *testing.T) {
	line := `time=2026-03-05T12:00:00.000+01:00 level=INFO msg="HTTP request" source=http method=GET path=/api/config status=200`

	entry, err := parseTextLogLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Level != "info" {
		t.Errorf("expected level 'info', got %q", entry.Level)
	}
	if entry.Message != "HTTP request" {
		t.Errorf("expected message 'HTTP request', got %q", entry.Message)
	}
	if entry.Source != "http" {
		t.Errorf("expected source 'http', got %q", entry.Source)
	}
	if entry.Attrs["method"] != "GET" {
		t.Errorf("expected method=GET, got %q", entry.Attrs["method"])
	}
	if entry.Attrs["status"] != "200" {
		t.Errorf("expected status=200, got %q", entry.Attrs["status"])
	}
}

func TestParseTextLogLine_QuotedValues(t *testing.T) {
	line := `time=2026-03-05T12:00:00.000+01:00 level=WARN msg="something went wrong" source=proxy error="connection refused"`

	entry, err := parseTextLogLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Message != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %q", entry.Message)
	}
	if entry.Attrs["error"] != "connection refused" {
		t.Errorf("expected error='connection refused', got %q", entry.Attrs["error"])
	}
}

func TestParseTextLogLine_UnquotedMsg(t *testing.T) {
	line := `time=2026-03-05T12:00:00.000+01:00 level=INFO msg=started source=server`

	entry, err := parseTextLogLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Message != "started" {
		t.Errorf("expected 'started', got %q", entry.Message)
	}
}

func TestParseTextLogLine_Invalid(t *testing.T) {
	tests := []string{
		"",
		"this is not a log line",
		"garbage=data",
	}
	for _, line := range tests {
		_, err := parseTextLogLine(line)
		if err == nil {
			t.Errorf("expected error for line %q", line)
		}
	}
}

// ---------------------------------------------------------------------------
// LoadRecentFromFile tests
// ---------------------------------------------------------------------------

func TestLoadRecentFromFile_PopulatesBuffer(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	oldPath := logFilePath
	oldWriter := logWriter
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
		logFilePath = oldPath
		logWriter = oldWriter
	}()

	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	// Write some slog text lines
	lines := []string{
		`time=2026-03-05T10:00:00.000+01:00 level=INFO msg="first entry" source=test`,
		`time=2026-03-05T10:01:00.000+01:00 level=WARN msg="second entry" source=test key=val`,
		`time=2026-03-05T10:02:00.000+01:00 level=ERROR msg="third entry" source=test`,
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Initialize with the test log file
	buffer = NewLogBuffer(1000)
	logFilePath = logFile
	logWriter = nil

	LoadRecentFromFile()

	entries := buffer.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Message != "first entry" {
		t.Errorf("expected 'first entry', got %q", entries[0].Message)
	}
	if entries[1].Level != "warn" {
		t.Errorf("expected level 'warn', got %q", entries[1].Level)
	}
	if entries[2].Message != "third entry" {
		t.Errorf("expected 'third entry', got %q", entries[2].Message)
	}
}

func TestLoadRecentFromFile_NoFile(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	oldPath := logFilePath
	oldWriter := logWriter
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
		logFilePath = oldPath
		logWriter = oldWriter
	}()

	buffer = NewLogBuffer(1000)
	logFilePath = "/nonexistent/path/test.log"
	logWriter = nil

	// Should not panic
	LoadRecentFromFile()

	entries := buffer.Recent(10)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for missing file, got %d", len(entries))
	}
}

func TestLoadRecentFromFile_SkipsInvalidLines(t *testing.T) {
	oldBuffer := buffer
	oldLogger := defaultLogger
	oldPath := logFilePath
	oldWriter := logWriter
	defer func() {
		buffer = oldBuffer
		defaultLogger = oldLogger
		logFilePath = oldPath
		logWriter = oldWriter
	}()

	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	lines := []string{
		`time=2026-03-05T10:00:00.000+01:00 level=INFO msg="valid" source=test`,
		`this is garbage`,
		`also not valid`,
		`time=2026-03-05T10:01:00.000+01:00 level=INFO msg="also valid" source=test`,
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	buffer = NewLogBuffer(1000)
	logFilePath = logFile
	logWriter = nil

	LoadRecentFromFile()

	entries := buffer.Recent(10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 valid entries (skipping garbage), got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Close tests
// ---------------------------------------------------------------------------

func TestClose_NilWriter(t *testing.T) {
	oldWriter := logWriter
	defer func() { logWriter = oldWriter }()

	logWriter = nil
	Close() // should not panic
}

func TestClose_ClosesWriter(t *testing.T) {
	oldWriter := logWriter
	defer func() { logWriter = oldWriter }()

	dir := t.TempDir()
	rw, err := newRotatingWriter(filepath.Join(dir, "test.log"), defaultMaxLogSize, defaultMaxLogFiles)
	if err != nil {
		t.Fatal(err)
	}
	logWriter = rw
	Close()

	if logWriter != nil {
		t.Error("expected logWriter to be nil after Close")
	}
}

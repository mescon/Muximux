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
	"time"
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
	Level   Level
	Format  string // "json" or "text"
	Output  string // "stdout", "stderr", or file path
	LogFile string // optional additional log file path (tees output)
}

// LogEntry represents a single log entry stored in the ring buffer
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Source    string            `json:"source"`
	Attrs     map[string]string `json:"attrs,omitempty"`
}

// LogBuffer is a thread-safe ring buffer that stores log entries and supports
// real-time subscriptions via channels.
type LogBuffer struct {
	entries []LogEntry
	size    int
	head    int
	count   int
	mu      sync.RWMutex

	subMu       sync.Mutex
	subscribers map[chan LogEntry]struct{}
}

// NewLogBuffer creates a new ring buffer with the given capacity.
func NewLogBuffer(size int) *LogBuffer {
	return &LogBuffer{
		entries:     make([]LogEntry, size),
		size:        size,
		subscribers: make(map[chan LogEntry]struct{}),
	}
}

// Add appends an entry to the ring buffer and notifies all subscribers.
func (b *LogBuffer) Add(entry LogEntry) { //nolint:gocritic // value receiver is intentional — stored in value slice
	b.mu.Lock()
	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.size
	if b.count < b.size {
		b.count++
	}
	b.mu.Unlock()

	// Notify subscribers (non-blocking send)
	b.subMu.Lock()
	for ch := range b.subscribers {
		select {
		case ch <- entry:
		default:
			// Drop entry if subscriber is too slow
		}
	}
	b.subMu.Unlock()
}

// Recent returns the last n log entries in chronological order.
func (b *LogBuffer) Recent(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n > b.count {
		n = b.count
	}
	if n == 0 {
		return nil
	}

	result := make([]LogEntry, n)
	// Start index: the oldest entry we want
	start := (b.head - n + b.size) % b.size
	for i := 0; i < n; i++ {
		result[i] = b.entries[(start+i)%b.size]
	}
	return result
}

// Subscribe creates a buffered channel that receives new log entries in real time.
func (b *LogBuffer) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 64)
	b.subMu.Lock()
	b.subscribers[ch] = struct{}{}
	b.subMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (b *LogBuffer) Unsubscribe(ch chan LogEntry) {
	b.subMu.Lock()
	delete(b.subscribers, ch)
	b.subMu.Unlock()
	close(ch)
}

// BroadcastHandler is an slog.Handler that captures log records into a LogBuffer
// while forwarding them to an underlying handler.
type BroadcastHandler struct {
	inner  slog.Handler
	buffer *LogBuffer
	attrs  []slog.Attr
}

// NewBroadcastHandler wraps an existing slog.Handler to also write to a LogBuffer.
func NewBroadcastHandler(inner slog.Handler, buf *LogBuffer) *BroadcastHandler {
	return &BroadcastHandler{
		inner:  inner,
		buffer: buf,
	}
}

func (h *BroadcastHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *BroadcastHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver
	source := ""
	attrs := map[string]string{}

	// Pre-set attrs (from WithAttrs)
	for _, a := range h.attrs {
		if a.Key == "source" {
			source = a.Value.String()
		} else {
			attrs[a.Key] = a.Value.String()
		}
	}

	// Record-level attrs (override pre-set)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "source" {
			source = a.Value.String()
		} else {
			attrs[a.Key] = a.Value.String()
		}
		return true
	})

	entry := LogEntry{
		Timestamp: r.Time,
		Level:     strings.ToLower(r.Level.String()),
		Message:   r.Message,
		Source:    source,
	}
	if len(attrs) > 0 {
		entry.Attrs = attrs
	}
	h.buffer.Add(entry)

	return h.inner.Handle(ctx, r)
}

func (h *BroadcastHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BroadcastHandler{
		inner:  h.inner.WithAttrs(attrs),
		buffer: h.buffer,
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *BroadcastHandler) WithGroup(name string) slog.Handler {
	return &BroadcastHandler{
		inner:  h.inner.WithGroup(name),
		buffer: h.buffer,
		attrs:  h.attrs,
	}
}

var (
	defaultLogger *slog.Logger
	buffer        *LogBuffer
	levelVar      slog.LevelVar
)

// Init initializes the global logger with a BroadcastHandler that captures
// log entries into an in-memory ring buffer.
// If LogFile is set, logs are written to both stdout and the file.
func Init(cfg Config) error {
	buffer = NewLogBuffer(1000)

	levelVar.Set(parseLevel(cfg.Level))

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

	// If a log file is configured, tee output to both the primary writer and the file.
	if cfg.LogFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0755); err != nil {
			return fmt.Errorf("create log directory: %w", err)
		}
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		output = io.MultiWriter(output, file)
	}

	opts := &slog.HandlerOptions{Level: &levelVar}

	var inner slog.Handler
	if strings.EqualFold(cfg.Format, "json") {
		inner = slog.NewJSONHandler(output, opts)
	} else {
		inner = slog.NewTextHandler(output, opts)
	}

	handler := NewBroadcastHandler(inner, buffer)
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)

	return nil
}

// Buffer returns the global log buffer, or nil if Init has not been called.
func Buffer() *LogBuffer {
	return buffer
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

// SetLevel dynamically changes the log level without restarting.
func SetLevel(l Level) {
	levelVar.Set(parseLevel(l))
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

package logging

import (
	"bufio"
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

const (
	defaultMaxLogSize  = 10 * 1024 * 1024 // 10 MB
	defaultMaxLogFiles = 3
)

// rotatingWriter is an io.Writer that rotates the underlying log file
// when it exceeds maxSize. It keeps at most maxFiles rotated copies
// (e.g. .log.1, .log.2, .log.3).
type rotatingWriter struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	size     int64
	maxSize  int64
	maxFiles int
}

func newRotatingWriter(path string, maxSize int64, maxFiles int) (*rotatingWriter, error) { //nolint:unparam // maxFiles is a constant in production but parameterized for testability
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return &rotatingWriter{
		file:     f,
		path:     path,
		size:     info.Size(),
		maxSize:  maxSize,
		maxFiles: maxFiles,
	}, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("log rotation failed: %w", err)
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}

	// Shift existing rotated files: .log.N → .log.N+1, delete beyond max
	for i := w.maxFiles; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		if i == w.maxFiles {
			os.Remove(src) // delete the oldest
			continue
		}
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		_ = os.Rename(src, dst) // file may not exist
	}

	// Rename current .log → .log.1
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		return err
	}

	// Open a fresh .log
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w.file = f
	w.size = 0
	return nil
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

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
	logFilePath   string
	logWriter     *rotatingWriter
)

// Init initializes the global logger with a BroadcastHandler that captures
// log entries into an in-memory ring buffer.
// If LogFile is set, logs are written to both stdout and the file via a
// rotating writer that prevents unbounded growth.
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
		rw, err := newRotatingWriter(cfg.LogFile, defaultMaxLogSize, defaultMaxLogFiles)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		logWriter = rw
		logFilePath = cfg.LogFile
		output = io.MultiWriter(output, rw)
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

// Close closes the rotating log writer. Call on shutdown.
func Close() {
	if logWriter != nil {
		logWriter.Close()
		logWriter = nil
	}
}

// parseTextLogLine parses a single slog text-format line into a LogEntry.
// Expected format: time=... level=... msg=... [key=value ...]
func parseTextLogLine(line string) (LogEntry, error) {
	pairs := parseKeyValuePairs(line)
	if len(pairs) == 0 {
		return LogEntry{}, fmt.Errorf("unparseable log line")
	}

	var entry LogEntry
	attrs := map[string]string{}

	for _, kv := range pairs {
		switch kv.key {
		case "time":
			t, err := time.Parse(time.RFC3339Nano, kv.value)
			if err != nil {
				// Try with millisecond precision
				t, err = time.Parse("2006-01-02T15:04:05.000-07:00", kv.value)
				if err != nil {
					return LogEntry{}, fmt.Errorf("invalid time: %w", err)
				}
			}
			entry.Timestamp = t
		case "level":
			entry.Level = strings.ToLower(kv.value)
		case "msg":
			entry.Message = kv.value
		case "source":
			entry.Source = kv.value
		default:
			attrs[kv.key] = kv.value
		}
	}

	if entry.Timestamp.IsZero() || entry.Level == "" {
		return LogEntry{}, fmt.Errorf("missing required fields")
	}

	if len(attrs) > 0 {
		entry.Attrs = attrs
	}

	return entry, nil
}

type keyValue struct {
	key, value string
}

// parseKeyValuePairs parses slog text format: key=value or key="quoted value" pairs.
func parseKeyValuePairs(line string) []keyValue {
	var pairs []keyValue
	i := 0
	n := len(line)

	for i < n {
		// Skip whitespace
		for i < n && line[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}

		// Find '='
		eqIdx := strings.IndexByte(line[i:], '=')
		if eqIdx < 0 {
			break
		}
		key := line[i : i+eqIdx]
		i += eqIdx + 1

		if i >= n {
			pairs = append(pairs, keyValue{key, ""})
			break
		}

		var value string
		if line[i] == '"' {
			// Quoted value — find closing quote (handle escaped quotes)
			i++ // skip opening quote
			var sb strings.Builder
			for i < n {
				if line[i] == '\\' && i+1 < n {
					// slog escapes with backslash
					sb.WriteByte(line[i+1])
					i += 2
					continue
				}
				if line[i] == '"' {
					i++ // skip closing quote
					break
				}
				sb.WriteByte(line[i])
				i++
			}
			value = sb.String()
		} else {
			// Unquoted value — read until next space
			end := strings.IndexByte(line[i:], ' ')
			if end < 0 {
				value = line[i:]
				i = n
			} else {
				value = line[i : i+end]
				i += end
			}
		}

		pairs = append(pairs, keyValue{key, value})
	}

	return pairs
}

// LoadRecentFromFile reads the current log file and populates the ring buffer
// with parsed entries. Call after Init(). Skips unparseable lines gracefully.
func LoadRecentFromFile() {
	if logFilePath == "" || buffer == nil {
		return
	}

	f, err := os.Open(logFilePath)
	if err != nil {
		return // file may not exist yet — that's fine
	}
	defer f.Close()

	// Read all lines, keeping only the last buffer.size entries
	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	// Increase scanner buffer for long lines
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		entry, err := parseTextLogLine(scanner.Text())
		if err != nil {
			continue // skip unparseable lines
		}
		entries = append(entries, entry)
		// Keep bounded — only store last buffer.size entries
		if len(entries) > buffer.size {
			entries = entries[len(entries)-buffer.size:]
		}
	}

	// Add entries to the ring buffer (without broadcasting to subscribers)
	for i := range entries {
		buffer.mu.Lock()
		buffer.entries[buffer.head] = entries[i]
		buffer.head = (buffer.head + 1) % buffer.size
		if buffer.count < buffer.size {
			buffer.count++
		}
		buffer.mu.Unlock()
	}
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

// Audit logs a security-relevant event at info level with source=audit.
// Use for authentication events, user management, config changes, and access control.
func Audit(msg string, args ...any) {
	Logger().Info(msg, append([]any{"source", "audit"}, args...)...)
}

// With returns a logger with additional attributes
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

// Context key type for logging-specific context values.
type ctxKey string

const (
	ctxRequestID ctxKey = "request_id"
	ctxUser      ctxKey = "user"
)

// SetRequestID returns a context enriched with a request ID.
func SetRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxRequestID, id)
}

// SetUser returns a context enriched with a username for log correlation.
func SetUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, ctxUser, user)
}

// From returns a logger enriched with context fields (request_id, user).
// Safe to call with nil or unenriched context — returns the default logger.
func From(ctx context.Context) *slog.Logger {
	l := Logger()
	if ctx == nil {
		return l
	}
	if rid, ok := ctx.Value(ctxRequestID).(string); ok {
		l = l.With("request_id", rid)
	}
	if u, ok := ctx.Value(ctxUser).(string); ok {
		l = l.With("user", u)
	}
	return l
}

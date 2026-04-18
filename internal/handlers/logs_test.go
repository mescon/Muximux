package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/logging"
)

func TestNewLogsHandler(t *testing.T) {
	h := NewLogsHandler()
	if h == nil {
		t.Fatal("expected NewLogsHandler to return non-nil handler")
	}
}

func TestGetRecentMethodNotAllowed(t *testing.T) {
	h := NewLogsHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/logs", nil)
	w := httptest.NewRecorder()

	h.GetRecent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestGetRecentNilBuffer(t *testing.T) {
	// Ensure the global buffer is nil by not initializing logging.
	// We cannot easily unset the global, but if logging.Buffer() returns nil
	// the handler should return an empty JSON array.
	// This test only works if logging.Init has NOT been called in this process.
	// Since other tests might call it, we test the nil-buffer path indirectly
	// via an integration-style approach: if the buffer is nil, we still get
	// a valid JSON response.

	h := NewLogsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	w := httptest.NewRecorder()

	h.GetRecent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var entries []logging.LogEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Either nil buffer returns empty array, or initialized buffer returns whatever is in it.
	// Both are valid — the key assertion is that JSON decoding succeeds.
}

func TestGetRecentWithEntries(t *testing.T) {
	// Initialize logging so Buffer() returns a non-nil buffer.
	if err := logging.Init(logging.Config{Level: logging.LevelDebug, Format: "text", Output: "stdout"}); err != nil {
		t.Fatalf("failed to init logging: %v", err)
	}

	buf := logging.Buffer()
	if buf == nil {
		t.Fatal("expected logging.Buffer() to be non-nil after Init")
	}

	// Add some test entries
	now := time.Now()
	buf.Add(logging.LogEntry{Timestamp: now, Level: "info", Message: "test message 1", Source: "proxy"})
	buf.Add(logging.LogEntry{Timestamp: now, Level: "warn", Message: "test warning", Source: "config"})
	buf.Add(logging.LogEntry{Timestamp: now, Level: "info", Message: "test message 2", Source: "proxy"})
	buf.Add(logging.LogEntry{Timestamp: now, Level: "error", Message: "test error", Source: "health"})

	h := NewLogsHandler()

	t.Run("default limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(entries) < 4 {
			t.Errorf("expected at least 4 entries, got %d", len(entries))
		}
	})

	t.Run("custom limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=2", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(entries) != 2 {
			t.Errorf("expected 2 entries with limit=2, got %d", len(entries))
		}
	})

	t.Run("invalid limit falls back to default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=notanumber", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should use default limit (200), so we get all entries
		if len(entries) < 4 {
			t.Errorf("expected at least 4 entries with invalid limit, got %d", len(entries))
		}
	})

	t.Run("filter by level", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=200&level=info", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		for _, e := range entries {
			if e.Level != "info" {
				t.Errorf("expected all entries to have level 'info', got %q for message %q", e.Level, e.Message)
			}
		}

		// We added 2 info entries
		if len(entries) < 2 {
			t.Errorf("expected at least 2 info entries, got %d", len(entries))
		}
	})

	t.Run("filter by source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=200&source=proxy", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		for _, e := range entries {
			if e.Source != "proxy" {
				t.Errorf("expected all entries to have source 'proxy', got %q for message %q", e.Source, e.Message)
			}
		}

		// We added 2 proxy entries
		if len(entries) < 2 {
			t.Errorf("expected at least 2 proxy entries, got %d", len(entries))
		}
	})

	t.Run("filter by level and source combined", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=200&level=info&source=proxy", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		for _, e := range entries {
			if e.Level != "info" || e.Source != "proxy" {
				t.Errorf("expected level=info source=proxy, got level=%q source=%q", e.Level, e.Source)
			}
		}

		// We added 2 entries matching both filters
		if len(entries) < 2 {
			t.Errorf("expected at least 2 entries matching both filters, got %d", len(entries))
		}
	})

	t.Run("filter returns no matches", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?level=nonexistent", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected 0 entries for nonexistent level, got %d", len(entries))
		}
	})

	t.Run("case insensitive level filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?level=INFO", nil)
		w := httptest.NewRecorder()

		h.GetRecent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var entries []logging.LogEntry
		if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(entries) < 2 {
			t.Errorf("expected at least 2 info entries with case-insensitive filter, got %d", len(entries))
		}
	})
}

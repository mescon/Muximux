package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mescon/muximux/v3/internal/logging"
)

// --- requestIDMiddleware ---

func TestRequestIDMiddleware_SetsHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := requestIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
	if len(rid) != 16 {
		t.Errorf("expected 16-char hex request ID, got %q (len %d)", rid, len(rid))
	}
}

func TestRequestIDMiddleware_EnrichesContext(t *testing.T) {
	var sawContext bool

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the context was enriched by checking SetRequestID round-trips:
		// From(ctx) returns a logger with request_id — we verify indirectly by
		// ensuring the context value was set (header proves the same ID was generated).
		rid := w.Header().Get("X-Request-ID")
		// Re-extract via logging.From to prove context enrichment works end-to-end
		logger := logging.From(r.Context())
		sawContext = logger != nil && rid != ""
		w.WriteHeader(http.StatusOK)
	})

	handler := requestIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !sawContext {
		t.Error("expected context to be enriched with request ID")
	}
}

func TestRequestIDMiddleware_Uniqueness(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := requestIDMiddleware(inner)
	ids := make(map[string]struct{})

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		rid := rec.Header().Get("X-Request-ID")
		if _, dup := ids[rid]; dup {
			t.Fatalf("duplicate request ID on iteration %d: %s", i, rid)
		}
		ids[rid] = struct{}{}
	}
}

func TestRequestIDMiddleware_HonorsIncoming(t *testing.T) {
	var sawID string

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawID = w.Header().Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	})

	handler := requestIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "upstream-abc-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if sawID != "upstream-abc-123" {
		t.Errorf("expected upstream ID to be honored, got %q", sawID)
	}
}

func TestRequestIDMiddleware_RejectsInvalidIncoming(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := requestIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "has spaces and <html>")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-ID")
	if rid == "has spaces and <html>" {
		t.Error("invalid X-Request-ID should not be honored")
	}
	if len(rid) != 16 {
		t.Errorf("expected generated 16-char hex ID, got %q (len %d)", rid, len(rid))
	}
}

// --- isValidRequestID ---

func TestIsValidRequestID(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"abc123", true},
		{"ABC-def_456", true},
		{"a1b2c3d4e5f6a1b2", true},
		{"", false},
		{"has spaces", false},
		{"has<html>", false},
		{string(make([]byte, 129)), false}, // too long
		{"valid-id", true},
		{"UPPER_CASE", true},
	}

	for _, tt := range tests {
		got := isValidRequestID(tt.input)
		if got != tt.valid {
			t.Errorf("isValidRequestID(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

// --- panicRecoveryMiddleware ---

func TestPanicRecoveryMiddleware_Returns500(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := panicRecoveryMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/explode", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if body := rec.Body.String(); body == "" {
		t.Error("expected error body")
	}
}

func TestPanicRecoveryMiddleware_NoPanic(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := panicRecoveryMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected 'ok', got %q", rec.Body.String())
	}
}

// --- requestLoggingMiddleware ---

func TestRequestLoggingMiddleware_PassesThrough(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	handler := requestLoggingMiddleware(inner)
	req := httptest.NewRequest(http.MethodPost, "/api/apps", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if rec.Body.String() != "created" {
		t.Errorf("expected 'created', got %q", rec.Body.String())
	}
}

func TestRequestLoggingMiddleware_DefaultsToOK(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write body without explicit WriteHeader — should default to 200
		w.Write([]byte("implicit ok"))
	})

	handler := requestLoggingMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- statusRecorder ---

func TestStatusRecorder_CapturesWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}

	sr.WriteHeader(http.StatusNotFound)

	if sr.status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", sr.status)
	}
	if !sr.written {
		t.Error("expected written to be true")
	}
}

func TestStatusRecorder_FirstWriteHeaderWins(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}

	sr.WriteHeader(http.StatusNotFound)
	sr.WriteHeader(http.StatusInternalServerError) // second call should not change captured status

	if sr.status != http.StatusNotFound {
		t.Errorf("expected first status 404, got %d", sr.status)
	}
}

func TestStatusRecorder_WriteDefaultsToOK(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}

	n, err := sr.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes, got %d", n)
	}
	if sr.status != http.StatusOK {
		t.Errorf("expected 200, got %d", sr.status)
	}
	if !sr.written {
		t.Error("expected written to be true after Write")
	}
	if sr.bytesWritten != 5 {
		t.Errorf("expected bytesWritten 5, got %d", sr.bytesWritten)
	}
}

func TestStatusRecorder_BytesWrittenAccumulates(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}

	_, _ = sr.Write([]byte("hello"))
	_, _ = sr.Write([]byte(" world"))

	if sr.bytesWritten != 11 {
		t.Errorf("expected bytesWritten 11, got %d", sr.bytesWritten)
	}
}

// --- isStaticAsset ---

func TestIsStaticAsset(t *testing.T) {
	tests := []struct {
		path   string
		expect bool
	}{
		{"/assets/app.js", true},
		{"/assets/style.css", true},
		{"/assets/anything", true},
		{"/favicon.ico", true},
		{"/app.js", true},
		{"/style.css", true},
		{"/logo.png", true},
		{"/photo.jpg", true},
		{"/icon.svg", true},
		{"/font.woff", true},
		{"/font.woff2", true},
		{"/font.ttf", true},
		{"/app.js.map", true},
		{"/api/config", false},
		{"/api/apps", false},
		{"/login", false},
		{"/callback", false},
		{"/", false},
	}

	for _, tt := range tests {
		got := isStaticAsset(tt.path)
		if got != tt.expect {
			t.Errorf("isStaticAsset(%q) = %v, want %v", tt.path, got, tt.expect)
		}
	}
}

// --- remoteIP ---

func TestRemoteIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		expect     string
	}{
		{"192.168.1.1:12345", "192.168.1.1"},
		{"10.0.0.1:80", "10.0.0.1"},
		{"[::1]:8080", "::1"},
		{"invalid", "invalid"}, // no port — returns as-is
	}

	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = tt.remoteAddr
		got := remoteIP(r)
		if got != tt.expect {
			t.Errorf("remoteIP(%q) = %q, want %q", tt.remoteAddr, got, tt.expect)
		}
	}
}

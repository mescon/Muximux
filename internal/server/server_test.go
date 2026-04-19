package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/handlers"
	"github.com/mescon/muximux/v3/internal/health"
	"github.com/mescon/muximux/v3/internal/websocket"
)

// --- rateLimiter ---

func TestRateLimiter_Allow(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      3,
		window:   1 * time.Minute,
	}

	ip := "192.168.1.1"

	// Under limit
	for i := 0; i < 3; i++ {
		if !rl.allow(ip) {
			t.Fatalf("expected allow on attempt %d", i+1)
		}
	}

	// Over limit
	if rl.allow(ip) {
		t.Error("expected deny when over limit")
	}

	// Different IP should still be allowed
	if !rl.allow("10.0.0.1") {
		t.Error("expected allow for different IP")
	}
}

func TestRateLimiter_Window(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      2,
		window:   50 * time.Millisecond,
	}

	ip := "10.0.0.1"

	// Use up the limit
	rl.allow(ip)
	rl.allow(ip)
	if rl.allow(ip) {
		t.Error("expected deny at limit")
	}

	// Wait for the window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !rl.allow(ip) {
		t.Error("expected allow after window expired")
	}
}

func TestRateLimiter_PurgeStale(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      10,
		window:   50 * time.Millisecond,
	}

	// Add some entries
	rl.allow("1.1.1.1")
	rl.allow("2.2.2.2")

	// Wait for them to expire
	time.Sleep(60 * time.Millisecond)

	rl.purgeStaleEntries()

	rl.mu.Lock()
	count := len(rl.attempts)
	rl.mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 entries after purge, got %d", count)
	}
}

func TestRateLimiter_PurgeStale_KeepsRecent(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      10,
		window:   1 * time.Hour,
	}

	rl.allow("1.1.1.1")
	rl.allow("2.2.2.2")

	rl.purgeStaleEntries()

	rl.mu.Lock()
	count := len(rl.attempts)
	rl.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 entries to remain, got %d", count)
	}
}

func TestRateLimiter_MapBounded(t *testing.T) {
	// With a small cap, feeding many distinct IPs must never let the map
	// grow beyond the cap. This is the defense against the "one login
	// attempt per forged IP" memory-exhaustion pattern called out in
	// findings.md C6.
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      3,
		maxIPs:   5,
		window:   1 * time.Minute,
	}

	for i := 0; i < 50; i++ {
		ip := fmt.Sprintf("203.0.113.%d", i)
		if !rl.allow(ip) {
			t.Fatalf("expected allow for fresh IP %s", ip)
		}
	}

	rl.mu.Lock()
	size := len(rl.attempts)
	rl.mu.Unlock()
	if size > 5 {
		t.Errorf("map exceeded cap: got %d entries, want <= 5", size)
	}
}

func TestRateLimiter_EvictLeastRecent(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      10,
		maxIPs:   3,
		window:   1 * time.Hour,
	}

	rl.allow("1.1.1.1")
	time.Sleep(2 * time.Millisecond)
	rl.allow("2.2.2.2")
	time.Sleep(2 * time.Millisecond)
	rl.allow("3.3.3.3")
	time.Sleep(2 * time.Millisecond)

	// Touch 1.1.1.1 again so it becomes the most-recently-seen entry.
	rl.allow("1.1.1.1")
	time.Sleep(2 * time.Millisecond)

	// Adding a fourth IP must evict the oldest (2.2.2.2), not the caller
	// and not the recently-touched entry.
	rl.allow("4.4.4.4")

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.attempts) != 3 {
		t.Fatalf("expected 3 entries after eviction, got %d", len(rl.attempts))
	}
	if _, ok := rl.attempts["2.2.2.2"]; ok {
		t.Error("expected 2.2.2.2 (oldest) to be evicted")
	}
	for _, ip := range []string{"1.1.1.1", "3.3.3.3", "4.4.4.4"} {
		if _, ok := rl.attempts[ip]; !ok {
			t.Errorf("expected %s to remain", ip)
		}
	}
}

func TestRateLimiter_EvictionDoesNotDropCurrentCaller(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      10,
		maxIPs:   2,
		window:   1 * time.Hour,
	}

	rl.allow("1.1.1.1")
	rl.allow("2.2.2.2")

	// The caller of allow() must never evict itself.
	rl.allow("3.3.3.3")

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if _, ok := rl.attempts["3.3.3.3"]; !ok {
		t.Error("current caller was evicted, which would break its own rate limit accounting")
	}
	if len(rl.attempts) != 2 {
		t.Errorf("expected map size 2, got %d", len(rl.attempts))
	}
}

func TestRateLimiter_Wrap(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      2,
		window:   1 * time.Minute,
	}

	callCount := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.wrap(inner)

	t.Run("GET requests are not rate limited", func(t *testing.T) {
		callCount = 0
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
			req.RemoteAddr = "5.5.5.5:1234"
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("GET request %d: expected 200, got %d", i, rec.Code)
			}
		}
		if callCount != 10 {
			t.Errorf("expected 10 calls, got %d", callCount)
		}
	})

	t.Run("POST requests are rate limited", func(t *testing.T) {
		// Reset the limiter
		rl.mu.Lock()
		rl.attempts = make(map[string]*rateLimitEntry)
		rl.mu.Unlock()

		ip := "6.6.6.6"
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			req.RemoteAddr = ip + ":1234"
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("POST request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// Third POST should be rate limited
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected 429, got %d", rec.Code)
		}

		// Check Retry-After header
		retryAfter := rec.Header().Get("Retry-After")
		expected := fmt.Sprintf("%d", int(rl.window.Seconds()))
		if retryAfter != expected {
			t.Errorf("expected Retry-After %s, got %s", expected, retryAfter)
		}
	})
}

func TestRateLimiter_Wrap_NoPort(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		max:      1,
		window:   1 * time.Minute,
	}

	callCount := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.wrap(inner)

	// RemoteAddr without port
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "1.2.3.4" // No port
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- parseDuration ---

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		defVal   time.Duration
		expected time.Duration
	}{
		{"empty returns default", "", 5 * time.Second, 5 * time.Second},
		{"day suffix", "7d", 0, 7 * 24 * time.Hour},
		{"single day", "1d", 0, 24 * time.Hour},
		{"standard hours", "2h", 0, 2 * time.Hour},
		{"standard minutes", "30m", 0, 30 * time.Minute},
		{"standard seconds", "10s", 0, 10 * time.Second},
		{"invalid returns default", "garbage", 99 * time.Second, 99 * time.Second},
		{"invalid day suffix", "xyzd", 42 * time.Second, 42 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDuration(tt.input, tt.defVal)
			if got != tt.expected {
				t.Errorf("parseDuration(%q, %v) = %v, want %v", tt.input, tt.defVal, got, tt.expected)
			}
		})
	}
}

// --- securityHeadersMiddleware ---

func TestSecurityHeadersMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("without script hash", func(t *testing.T) {
		handler := securityHeadersMiddleware(inner, "")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		expected := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "SAMEORIGIN",
			"Referrer-Policy":        "strict-origin-when-cross-origin",
		}
		for header, value := range expected {
			got := rec.Header().Get(header)
			if got != value {
				t.Errorf("header %s = %q, want %q", header, got, value)
			}
		}

		// Permissions-Policy must permit `*` for features we want to be
		// delegatable via iframe `allow` attributes.
		permPolicy := rec.Header().Get("Permissions-Policy")
		for _, feature := range []string{"camera=*", "microphone=*", "geolocation=*", "fullscreen=*"} {
			if !strings.Contains(permPolicy, feature) {
				t.Errorf("Permissions-Policy missing %q: %s", feature, permPolicy)
			}
		}

		csp := rec.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self'") {
			t.Errorf("CSP missing script-src 'self': %s", csp)
		}
		if strings.Contains(csp, "sha256") {
			t.Errorf("CSP should not contain sha256 hash without base path: %s", csp)
		}
		// findings.md H10 hardening: frame-ancestors, form-action, and
		// a tightened connect-src (no wildcard ws:/wss:) must appear.
		for _, want := range []string{
			"frame-ancestors 'self'",
			"form-action 'self'",
			"connect-src 'self'",
		} {
			if !strings.Contains(csp, want) {
				t.Errorf("CSP missing %q: %s", want, csp)
			}
		}
		if strings.Contains(csp, "connect-src 'self' ws:") {
			t.Errorf("CSP still contains wildcard ws: directive: %s", csp)
		}
	})

	t.Run("with script hash", func(t *testing.T) {
		hash := "'sha256-abc123'"
		handler := securityHeadersMiddleware(inner, hash)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		csp := rec.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self' 'sha256-abc123'") {
			t.Errorf("CSP missing script hash: %s", csp)
		}
	})

	t.Run("proxy paths skip CSP and X-Frame-Options", func(t *testing.T) {
		handler := securityHeadersMiddleware(inner, "")
		req := httptest.NewRequest(http.MethodGet, "/proxy/myapp/index.html", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Security-Policy") != "" {
			t.Error("proxy path should not have CSP header")
		}
		if rec.Header().Get("X-Frame-Options") != "" {
			t.Error("proxy path should not have X-Frame-Options header")
		}
		// nosniff and Referrer-Policy should still be set
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("proxy path should still have X-Content-Type-Options")
		}
		if rec.Header().Get("Referrer-Policy") == "" {
			t.Error("proxy path should still have Referrer-Policy")
		}
	})
}

// --- csrfMiddleware ---

func TestCSRFMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := csrfMiddleware(inner)

	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		wantCode    int
	}{
		{"GET API passes through", "GET", "/api/config", "", http.StatusOK},
		{"POST API with JSON passes", "POST", "/api/config", "application/json", http.StatusOK},
		{"POST API with multipart passes", "POST", "/api/icons/custom", "multipart/form-data; boundary=abc", http.StatusOK},
		{"POST API with x-yaml passes", "POST", "/api/config/import", "application/x-yaml", http.StatusOK},
		{"POST API without content-type blocked", "POST", "/api/config", "", http.StatusForbidden},
		{"POST API with text/plain blocked", "POST", "/api/config", "text/plain", http.StatusForbidden},
		{"PUT API with JSON passes", "PUT", "/api/config", "application/json", http.StatusOK},
		{"PUT API without content-type blocked", "PUT", "/api/config", "", http.StatusForbidden},
		{"DELETE API passes (not simple method)", "DELETE", "/api/app/test", "", http.StatusOK},
		{"POST non-API passes", "POST", "/login", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rec.Code)
			}
		})
	}
}

// --- bodySizeLimitMiddleware ---

func TestBodySizeLimitMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Just check that the body was wrapped - not that it limits.
		// The middleware wraps r.Body with MaxBytesReader.
		w.WriteHeader(http.StatusOK)
	})
	handler := bodySizeLimitMiddleware(inner)

	t.Run("API request has body limited", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/something", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("non-API request passes without limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/other", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestBodySizeLimitMiddleware_Paths(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := bodySizeLimitMiddleware(inner)

	tests := []struct {
		name string
		path string
	}{
		{"config endpoint", "/api/config"},
		{"themes endpoint", "/api/themes"},
		{"icons custom endpoint", "/api/icons/custom"},
		{"general API endpoint", "/api/other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader("test body")
			req := httptest.NewRequest(http.MethodPost, tt.path, body)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		})
	}
}

func TestBodySizeLimitMiddleware_NilBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := bodySizeLimitMiddleware(inner)

	// API request with nil body
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Body = nil
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- validThemeName regex ---

func TestValidThemeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"simple theme", "nord-dark.css", true},
		{"underscore", "my_theme.css", true},
		{"dots", "theme.v2.css", true},
		{"no extension", "theme", false},
		{"path traversal", "../evil.css", false},
		{"leading dot", ".hidden.css", false},
		{"non-css", "theme.js", false},
		{"empty", "", false},
		{"just css", ".css", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validThemeName.MatchString(tt.input)
			if got != tt.valid {
				t.Errorf("validThemeName.MatchString(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

// --- spaHandlerDev ---

func TestSPAHandlerDev(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := tmpDir + "/index.html"
	cssPath := tmpDir + "/style.css"
	if err := os.WriteFile(indexPath, []byte("<html><head></head><body>SPA</body></html>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cssPath, []byte("body{}"), 0600); err != nil {
		t.Fatal(err)
	}

	fileServer := http.FileServer(http.Dir(tmpDir))
	handler, _ := spaHandlerDev(fileServer, tmpDir, "")

	t.Run("root serves index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SPA") {
			t.Error("expected SPA content")
		}
	})

	t.Run("SPA route serves index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SPA") {
			t.Error("expected SPA content for SPA route")
		}
	})

	t.Run("static file served directly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("API paths not treated as SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "SPA") {
			t.Error("API paths should not serve SPA index")
		}
	})

	t.Run("ws paths not treated as SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "SPA") {
			t.Error("ws paths should not serve SPA index")
		}
	})

	t.Run("proxy paths not treated as SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/proxy/app1/page", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "SPA") {
			t.Error("proxy paths should not serve SPA index")
		}
	})

	t.Run("icons paths not treated as SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/icons/something", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "SPA") {
			t.Error("icons paths should not serve SPA index")
		}
	})

	t.Run("base path injection", func(t *testing.T) {
		handler2, hash := spaHandlerDev(fileServer, tmpDir, "/app")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler2.ServeHTTP(rec, req)
		body := rec.Body.String()
		if !strings.Contains(body, `window.__MUXIMUX_BASE__="/app"`) {
			t.Errorf("expected base path injection, got: %s", body)
		}
		if hash == "" || !strings.HasPrefix(hash, "'sha256-") {
			t.Errorf("expected CSP hash, got: %s", hash)
		}
	})

	t.Run("no injection without base path", func(t *testing.T) {
		_, hash := spaHandlerDev(fileServer, tmpDir, "")
		if hash != "" {
			t.Errorf("expected empty hash without base path, got: %s", hash)
		}
	})
}

// --- spaHandlerEmbed ---

func TestSPAHandlerEmbed(t *testing.T) {
	testFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>Embedded SPA</html>")},
		"style.css":  &fstest.MapFile{Data: []byte("body{}")},
	}

	fileServer := http.FileServer(http.FS(testFS))
	handler, _ := spaHandlerEmbed(fileServer, testFS, "")

	t.Run("root serves index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Embedded SPA") {
			t.Error("expected embedded SPA content")
		}
	})

	t.Run("SPA route serves index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/settings", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Embedded SPA") {
			t.Error("expected embedded SPA content for SPA route")
		}
	})

	t.Run("content type is HTML for SPA route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Errorf("expected Content-Type text/html, got %s", ct)
		}
	})

	t.Run("API paths not treated as SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "Embedded SPA") {
			t.Error("API paths should not serve SPA index")
		}
	})

	t.Run("ws paths not treated as SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "Embedded SPA") {
			t.Error("ws paths should not serve SPA index")
		}
	})
}

func TestSPAHandlerEmbed_BasePath(t *testing.T) {
	testFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><head></head><body>SPA</body></html>")},
	}

	fileServer := http.FileServer(http.FS(testFS))
	handler, scriptHash := spaHandlerEmbed(fileServer, testFS, "/dash")

	t.Run("injects base path script", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `window.__MUXIMUX_BASE__="/dash"`) {
			t.Errorf("expected base path injection in HTML, got: %s", body)
		}
	})

	t.Run("returns script hash for CSP", func(t *testing.T) {
		if scriptHash == "" {
			t.Error("expected non-empty script hash when base path is set")
		}
		if !strings.HasPrefix(scriptHash, "'sha256-") || !strings.HasSuffix(scriptHash, "'") {
			t.Errorf("script hash should be CSP-formatted, got: %s", scriptHash)
		}
	})

	t.Run("no injection with empty base path", func(t *testing.T) {
		handler2, hash2 := spaHandlerEmbed(fileServer, testFS, "")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler2.ServeHTTP(rec, req)
		body := rec.Body.String()
		if strings.Contains(body, "__MUXIMUX_BASE__") {
			t.Errorf("expected no base path injection, got: %s", body)
		}
		if hash2 != "" {
			t.Errorf("expected empty script hash without base path, got: %s", hash2)
		}
	})
}

func TestSPAHandlerEmbed_MissingIndex(t *testing.T) {
	testFS := fstest.MapFS{
		"other.txt": &fstest.MapFile{Data: []byte("not index")},
	}

	fileServer := http.FileServer(http.FS(testFS))
	handler, _ := spaHandlerEmbed(fileServer, testFS, "")

	if handler == nil {
		t.Fatal("expected non-nil handler even with missing index")
	}
}

// --- handleServiceWorker ---

func TestHandleServiceWorker(t *testing.T) {
	swContent := `const CACHE_NAME = 'muximux-v1';`
	testFS := fstest.MapFS{
		"sw.js": &fstest.MapFile{Data: []byte(swContent)},
	}

	s := &Server{version: "3.0.4", commit: "abcdef1234567890"}
	handler := s.handleServiceWorker(testFS)

	req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "muximux-v1") {
		t.Error("sw.js should not contain the placeholder cache name")
	}
	if !strings.Contains(body, "muximux-3.0.4-abcdef12") {
		t.Errorf("sw.js should contain version-aware cache name, got: %s", body)
	}
	if rec.Header().Get("Content-Type") != "application/javascript; charset=utf-8" {
		t.Errorf("wrong content type: %s", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("sw.js should have no-cache: %s", rec.Header().Get("Cache-Control"))
	}
}

// --- wrapMiddleware ---

func TestWrapMiddleware(t *testing.T) {
	inner := http.NewServeMux()
	inner.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("auth method none skips auth", func(t *testing.T) {
		cfg := &config.Config{Auth: config.AuthConfig{Method: "none"}}
		ss := auth.NewSessionStore("test", time.Hour, false)
		us := auth.NewUserStore()
		am := auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, ss, us)
		handler := wrapMiddleware(inner, cfg, am, "")

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("auth method empty skips auth", func(t *testing.T) {
		cfg := &config.Config{Auth: config.AuthConfig{Method: ""}}
		ss := auth.NewSessionStore("test", time.Hour, false)
		us := auth.NewUserStore()
		am := auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, ss, us)
		handler := wrapMiddleware(inner, cfg, am, "")

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("security headers are added", func(t *testing.T) {
		cfg := &config.Config{Auth: config.AuthConfig{Method: "none"}}
		ss := auth.NewSessionStore("test", time.Hour, false)
		us := auth.NewUserStore()
		am := auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, ss, us)
		handler := wrapMiddleware(inner, cfg, am, "")

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("expected X-Content-Type-Options: nosniff")
		}
		if rec.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
			t.Error("expected X-Frame-Options: SAMEORIGIN")
		}
	})

	t.Run("CSRF middleware blocks invalid POST", func(t *testing.T) {
		cfg := &config.Config{Auth: config.AuthConfig{Method: "none"}}
		ss := auth.NewSessionStore("test", time.Hour, false)
		us := auth.NewUserStore()
		am := auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, ss, us)
		handler := wrapMiddleware(inner, cfg, am, "")

		req := httptest.NewRequest(http.MethodPost, "/api/config", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 for POST without JSON content-type, got %d", rec.Code)
		}
	})

	t.Run("builtin auth requires session", func(t *testing.T) {
		cfg := &config.Config{Auth: config.AuthConfig{Method: "builtin"}}
		ss := auth.NewSessionStore("test", time.Hour, false)
		us := auth.NewUserStore()
		am := auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodBuiltin}, ss, us)
		handler := wrapMiddleware(inner, cfg, am, "")

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for unauthenticated request, got %d", rec.Code)
		}
	})
}

// --- setupAuth ---

func TestSetupAuth(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Method:        "builtin",
			SessionMaxAge: "1h",
			SecureCookies: true,
			Users: []config.UserConfig{
				{Username: "admin", PasswordHash: "$2a$10$fake", Role: "admin"},
				{Username: "user", PasswordHash: "$2a$10$fake", Role: "user"},
			},
			TrustedProxies: []string{"10.0.0.0/8"},
			Headers: map[string]string{
				"user":  "X-User",
				"email": "X-Email",
			},
		},
	}

	ss, us, am := setupAuth(cfg)

	if ss == nil {
		t.Fatal("expected non-nil session store")
	}
	if us == nil {
		t.Fatal("expected non-nil user store")
	}
	if am == nil {
		t.Fatal("expected non-nil auth middleware")
	}
	if us.Count() != 2 {
		t.Errorf("expected 2 users, got %d", us.Count())
	}
	if us.Get("admin") == nil {
		t.Error("expected admin user to be loaded")
	}
	if us.Get("user") == nil {
		t.Error("expected user to be loaded")
	}
}

func TestSetupAuth_DefaultSessionMaxAge(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Method:        "builtin",
			SessionMaxAge: "",
		},
	}

	ss, _, _ := setupAuth(cfg)
	if ss == nil {
		t.Fatal("expected non-nil session store")
	}
}

func TestSetupAuth_NoUsers(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Method: "none",
		},
	}

	ss, us, am := setupAuth(cfg)
	if ss == nil || us == nil || am == nil {
		t.Fatal("expected non-nil stores and middleware")
	}
	if us.Count() != 0 {
		t.Errorf("expected 0 users, got %d", us.Count())
	}
}

// --- setupOIDC ---

func TestSetupOIDC(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			OIDC: config.OIDCConfig{
				Enabled:      true,
				IssuerURL:    "http://idp.example.com",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				RedirectURL:  "http://localhost:3000/callback",
				Scopes:       []string{"openid"},
				AdminGroups:  []string{"admins"},
			},
		},
	}

	ss := auth.NewSessionStore("test", time.Hour, false)
	us := auth.NewUserStore()

	provider := setupOIDC(cfg, ss, us)
	if provider == nil {
		t.Fatal("expected non-nil OIDC provider")
	}
	if !provider.Enabled() {
		t.Error("expected OIDC provider to be enabled")
	}
}

// --- adminGuard ---

func TestAdminGuard(t *testing.T) {
	requireAdmin := adminGuard(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if user.Role != auth.RoleAdmin {
				http.Error(w, "Forbidden: admin role required", http.StatusForbidden)
				return
			}
			next(w, r)
		}
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("admin user passes through", func(t *testing.T) {
		handler := requireAdmin(inner)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, &auth.User{Role: auth.RoleAdmin})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("non-admin user gets 403", func(t *testing.T) {
		handler := requireAdmin(inner)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, &auth.User{Role: auth.RoleUser})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("no user gets 401", func(t *testing.T) {
		handler := requireAdmin(inner)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})
}

// --- registerAPIRoutes ---

func TestRegisterAPIRoutes(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Title: "Test"},
		Apps: []config.AppConfig{
			{Name: "App1", URL: "http://a:8080", Enabled: true},
		},
		Groups: []config.GroupConfig{
			{Name: "Group1", Color: "#ff0000"},
		},
	}
	api := handlers.NewAPIHandler(cfg, "", &sync.RWMutex{})

	noopAdmin := adminGuard(func(next http.HandlerFunc) http.HandlerFunc {
		return next
	})

	mux := http.NewServeMux()
	registerAPIRoutes(mux, api, noopAdmin)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	t.Run("GET /api/config returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/config")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DELETE /api/config returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/config", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/apps returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/apps")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DELETE /api/apps returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/apps", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/groups returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/groups")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DELETE /api/groups returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/groups", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/app/App1 returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/app/App1")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/app/ without name returns 400", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/app/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/group/Group1 returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/group/Group1")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/group/ without name returns 400", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/group/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("PATCH /api/app/App1 returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/app/App1", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("PATCH /api/group/Group1 returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/group/Group1", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/health returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// Exercise POST/PUT/DELETE routes through the admin guard closures.
	// The handler may return errors (empty configPath) but the route handler code is covered.
	t.Run("PUT /api/config exercises SaveConfig route", func(t *testing.T) {
		body := strings.NewReader(`{"title":"Updated"}`)
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/config", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// May succeed or fail depending on configPath; just verify the route is reachable
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("PUT should not return 405")
		}
	})

	t.Run("POST /api/apps exercises CreateApp route", func(t *testing.T) {
		body := strings.NewReader(`{"name":"NewApp","url":"http://localhost:9999","enabled":true}`)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/apps", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("POST should not return 405")
		}
	})

	t.Run("POST /api/groups exercises CreateGroup route", func(t *testing.T) {
		body := strings.NewReader(`{"name":"NewGroup","color":"#aabbcc"}`)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/groups", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("POST should not return 405")
		}
	})

	t.Run("PUT /api/app/App1 exercises UpdateApp route", func(t *testing.T) {
		body := strings.NewReader(`{"name":"App1","url":"http://localhost:9999","enabled":true}`)
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/app/App1", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("PUT should not return 405")
		}
	})

	t.Run("DELETE /api/app/App1 exercises DeleteApp route", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/app/App1", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("DELETE should not return 405")
		}
	})

	t.Run("PUT /api/group/Group1 exercises UpdateGroup route", func(t *testing.T) {
		body := strings.NewReader(`{"name":"Group1","color":"#112233"}`)
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/group/Group1", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("PUT should not return 405")
		}
	})

	t.Run("DELETE /api/group/Group1 exercises DeleteGroup route", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/group/Group1", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("DELETE should not return 405")
		}
	})
}

// --- registerAuthRoutes ---

func TestRegisterAuthRoutes(t *testing.T) {
	ss := auth.NewSessionStore("test", time.Hour, false)
	us := auth.NewUserStore()
	authHandler := handlers.NewAuthHandler(ss, us, nil, "", nil, &sync.RWMutex{})
	wsHub := websocket.NewHub()
	go wsHub.Run()

	am := auth.NewMiddleware(&auth.AuthConfig{}, ss, us)

	mux := http.NewServeMux()
	registerAuthRoutes(mux, authHandler, wsHub, am)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	t.Run("GET /api/auth/status returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/auth/status")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/auth/login returns 405 (needs POST)", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/auth/login")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/auth/logout returns 405 (needs POST)", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/auth/logout")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/auth/oidc/login returns 404 (no OIDC)", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/auth/oidc/login")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// --- registerThemeRoutes ---

func TestRegisterThemeRoutes(t *testing.T) {
	testFS := fstest.MapFS{
		"themes/bundled.css": &fstest.MapFile{Data: []byte("/* bundled theme */")},
	}

	distFS, _ := fs.Sub(testFS, ".")

	noopAdmin := adminGuard(func(next http.HandlerFunc) http.HandlerFunc {
		return next
	})

	staticHandler := http.FileServer(http.FS(testFS))

	mux := http.NewServeMux()
	registerThemeRoutes(mux, distFS, noopAdmin, &staticHandler, t.TempDir())

	ts := httptest.NewServer(mux)
	defer ts.Close()

	t.Run("GET /api/themes returns 200", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/themes")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid theme name returns 404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/themes/.hidden")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("DELETE /api/themes returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/themes", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("POST /api/themes exercises SaveTheme route", func(t *testing.T) {
		body := strings.NewReader(`{"id":"test","name":"Test","family":"test","variables":{}}`)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/themes", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("POST should not return 405")
		}
	})

	t.Run("DELETE /api/themes/some-id exercises DeleteTheme route", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/themes/nonexistent", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// Route should be reachable (not 405)
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("DELETE should not return 405")
		}
	})

	t.Run("GET /themes/valid.css falls through to static handler", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/themes/valid.css")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// File doesn't exist locally, will fall through to static handler
		// which also won't have it, so 404 is expected
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /themes/ path traversal returns 404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/themes/..%2F..%2Fetc%2Fpasswd")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestRegisterThemeRoutes_ServeLocalTheme(t *testing.T) {
	themesDir := t.TempDir()
	testFile := filepath.Join(themesDir, "local-test-theme.css")
	if err := os.WriteFile(testFile, []byte("/* test theme */\n:root { --bg: #000; }"), 0o600); err != nil {
		t.Fatal(err)
	}

	testFS := fstest.MapFS{}
	distFS, _ := fs.Sub(testFS, ".")
	noopAdmin := adminGuard(func(next http.HandlerFunc) http.HandlerFunc { return next })
	staticHandler := http.FileServer(http.FS(testFS))

	mux := http.NewServeMux()
	registerThemeRoutes(mux, distFS, noopAdmin, &staticHandler, themesDir)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/themes/local-test-theme.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for local theme, got %d", resp.StatusCode)
	}
}

// --- registerIconRoutes ---

func TestRegisterIconRoutes(t *testing.T) {
	cfg := &config.Config{
		Icons: config.IconsConfig{
			DashboardIcons: config.DashboardIconsConfig{
				CacheDir: t.TempDir(),
				CacheTTL: "1h",
			},
		},
	}
	noopAdmin := adminGuard(func(next http.HandlerFunc) http.HandlerFunc {
		return next
	})

	mux := http.NewServeMux()
	registerIconRoutes(mux, cfg, noopAdmin, t.TempDir())

	ts := httptest.NewServer(mux)
	defer ts.Close()

	t.Run("GET /api/icons/dashboard reachable", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/icons/dashboard")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Error("route should be registered")
		}
	})

	t.Run("GET /api/icons/lucide reachable", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/icons/lucide")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Error("route should be registered")
		}
	})

	t.Run("GET /api/icons/custom returns list", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/icons/custom")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Error("route should be registered")
		}
	})

	t.Run("DELETE /api/icons/custom returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/icons/custom", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("POST /api/icons/custom exercises UploadCustomIcon", func(t *testing.T) {
		body := strings.NewReader(`not a valid multipart form`)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/icons/custom", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Error("POST should not return 405")
		}
	})

	t.Run("GET /icons/test.png exercises ServeIcon", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/icons/test.png")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// Icon likely not found, but route is exercised
	})
}

// --- setupCaddy ---

func TestSetupCaddy_NoCaddy(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Listen: ":3000"},
	}
	s := &Server{config: cfg}

	addr := setupCaddy(s, cfg)

	if addr != ":3000" {
		t.Errorf("expected :3000, got %s", addr)
	}
	if s.proxyServer != nil {
		t.Error("expected nil proxy server when caddy not needed")
	}
}

// --- Server GetHub ---

func TestServer_GetHub(t *testing.T) {
	s := &Server{wsHub: nil}
	if s.GetHub() != nil {
		t.Error("expected nil hub for empty server")
	}

	hub := websocket.NewHub()
	s.wsHub = hub
	if s.GetHub() != hub {
		t.Error("expected GetHub to return the set hub")
	}
}

// --- Server Stop ---

func TestServer_Stop_NilComponents(t *testing.T) {
	s := &Server{
		httpServer:    &http.Server{ReadHeaderTimeout: 5 * time.Second},
		healthMonitor: nil,
		proxyServer:   nil,
	}

	// Stop with nil health monitor and proxy server should not panic
	_ = s.Stop()
}

func TestServer_Stop_WithHealthMonitor(t *testing.T) {
	mon := health.NewMonitor(1*time.Second, 1*time.Second)
	s := &Server{
		httpServer:    &http.Server{ReadHeaderTimeout: 5 * time.Second},
		healthMonitor: mon,
	}

	if err := s.Stop(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- setupHealthRoutes ---

func TestSetupHealthRoutes_Disabled(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{Enabled: false},
	}
	wsHub := websocket.NewHub()
	mux := http.NewServeMux()
	s := &Server{config: cfg}

	s.setupHealthRoutes(mux, cfg, wsHub)

	if s.healthMonitor != nil {
		t.Error("expected nil health monitor when disabled")
	}
}

func TestSetupHealthRoutes_Enabled(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Enabled:  true,
			Interval: "30s",
			Timeout:  "5s",
		},
		Apps: []config.AppConfig{
			{Name: "TestApp", URL: "http://localhost:9999", Enabled: true},
		},
	}
	wsHub := websocket.NewHub()
	mux := http.NewServeMux()
	s := &Server{config: cfg}

	s.setupHealthRoutes(mux, cfg, wsHub)

	if s.healthMonitor == nil {
		t.Fatal("expected non-nil health monitor when enabled")
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/apps/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSetupHealthRoutes_AppSubPaths(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Enabled:  true,
			Interval: "30s",
			Timeout:  "5s",
		},
		Apps: []config.AppConfig{
			{Name: "TestApp", URL: "http://localhost:9999", Enabled: true},
		},
	}
	wsHub := websocket.NewHub()
	mux := http.NewServeMux()
	s := &Server{config: cfg}

	s.setupHealthRoutes(mux, cfg, wsHub)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	t.Run("health endpoint", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/apps/TestApp/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// The handler may return 404 if app not found in monitor, but the route should exist
		if resp.StatusCode == 0 {
			t.Error("expected a response")
		}
	})

	t.Run("health check endpoint", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/apps/TestApp/health/check")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 0 {
			t.Error("expected a response")
		}
	})

	t.Run("unknown subpath returns 404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/apps/TestApp/unknown")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// --- Setup Mode ---

func defaultTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Listen: ":8080", Title: "Test"},
		Auth:   config.AuthConfig{Method: "none"},
	}
}

// testSetupToken is the canned value every pre-setup test uses. The
// validateSetupToken check is constant-time and not value-dependent, so
// this just has to match between the Server field and the request header.
const testSetupToken = "test-setup-token"

// withSetupToken stamps the canned token onto both the Server state and
// the inbound request so tests can exercise the path past the
// authorization gate introduced in findings.md C1.
func withSetupToken(s *Server, r *http.Request) {
	s.setupToken = testSetupToken
	r.Header.Set(setupTokenHeader, testSetupToken)
}

func TestSetupGuardMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("allows all when setup complete", func(t *testing.T) {
		s := &Server{}
		s.needsSetup.Store(false)
		handler := s.setupGuardMiddleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("allows auth endpoints during setup", func(t *testing.T) {
		s := &Server{}
		s.needsSetup.Store(true)
		handler := s.setupGuardMiddleware(inner)

		for _, path := range []string{"/api/auth/status", "/api/auth/setup", "/api/auth/login"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("path %s: expected 200, got %d", path, rec.Code)
			}
		}
	})

	t.Run("allows health endpoint during setup", func(t *testing.T) {
		s := &Server{}
		s.needsSetup.Store(true)
		handler := s.setupGuardMiddleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("allows static assets during setup", func(t *testing.T) {
		s := &Server{}
		s.needsSetup.Store(true)
		handler := s.setupGuardMiddleware(inner)

		for _, path := range []string{"/", "/login", "/assets/style.css", "/themes/dark.css"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("path %s: expected 200, got %d", path, rec.Code)
			}
		}
	})

	t.Run("blocks API endpoints during setup", func(t *testing.T) {
		s := &Server{}
		s.needsSetup.Store(true)
		handler := s.setupGuardMiddleware(inner)

		for _, path := range []string{"/api/config", "/api/apps", "/api/groups"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("path %s: expected 503, got %d", path, rec.Code)
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["error"] != "setup_required" {
				t.Errorf("expected error=setup_required, got %v", body["error"])
			}
		}
	})
}

func TestSetupGuardMiddleware_AllowsConfigRestore(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	s := &Server{}
	s.needsSetup.Store(true)
	handler := s.setupGuardMiddleware(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /api/config/restore during setup, got %d", rec.Code)
	}
}

func TestHandleConfigRestore_WrongMethod(t *testing.T) {
	s := &Server{}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodGet, "/api/config/restore", nil)
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// TestHandleConfigRestore_MissingToken and TestHandleSetup_MissingToken
// pin down the pre-setup authorization gate introduced for findings.md
// C1: an unauthenticated network attacker racing to seed admin credentials
// must be rejected with 401 before any parsing or state changes happen.
func TestHandleConfigRestore_MissingToken(t *testing.T) {
	s := &Server{setupToken: "the-real-token"}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader("apps: []"))
	req.Header.Set("Content-Type", "application/x-yaml")
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with no token, got %d", rec.Code)
	}
}

func TestHandleConfigRestore_WrongToken(t *testing.T) {
	s := &Server{setupToken: "the-real-token"}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader("apps: []"))
	req.Header.Set("Content-Type", "application/x-yaml")
	req.Header.Set(setupTokenHeader, "wrong-token")
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong token, got %d", rec.Code)
	}
}

func TestHandleSetup_MissingToken(t *testing.T) {
	s := &Server{setupToken: "the-real-token"}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"method":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with no token, got %d", rec.Code)
	}
}

// TestHandleSetup_ClearsTokenOnSuccess verifies the one-time nature of the
// token: once setup succeeds the in-memory value is zeroed and the on-disk
// file is removed, so a compromised token cannot be replayed to re-seed
// credentials later.
func TestHandleSetup_ClearsTokenOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, setupTokenFilename)
	if err := os.WriteFile(tokenPath, []byte("the-real-token\n"), 0o600); err != nil {
		t.Fatalf("seed token file: %v", err)
	}

	s := &Server{
		config:     defaultTestConfig(),
		configPath: filepath.Join(tmpDir, "config.yaml"),
		dataDir:    tmpDir,
		setupToken: "the-real-token",
	}
	s.needsSetup.Store(true)
	s.sessionStore = auth.NewSessionStore("test", time.Hour, false)
	s.userStore = auth.NewUserStore()
	s.authMiddleware = auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, s.sessionStore, s.userStore)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"method":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(setupTokenHeader, "the-real-token")
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on successful setup, got %d: %s", rec.Code, rec.Body.String())
	}
	if s.setupToken != "" {
		t.Errorf("expected setupToken cleared, got %q", s.setupToken)
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Errorf("expected token file removed, got err=%v", err)
	}
}

// TestEnsureSetupToken_GeneratesAndPersists covers the startup path: an
// instance that boots needing setup generates a random token, writes it
// with 0600 mode, and reuses it on restart.
func TestEnsureSetupToken_GeneratesAndPersists(t *testing.T) {
	tmpDir := t.TempDir()
	s := &Server{dataDir: tmpDir}

	if err := s.ensureSetupToken(); err != nil {
		t.Fatalf("ensureSetupToken failed: %v", err)
	}
	if s.setupToken == "" {
		t.Fatal("expected setupToken to be populated")
	}
	originalTok := s.setupToken

	info, err := os.Stat(filepath.Join(tmpDir, setupTokenFilename))
	if err != nil {
		t.Fatalf("token file missing: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("token file mode = %o, want 0600", mode)
	}

	// Second boot: token should be reused, not regenerated.
	s2 := &Server{dataDir: tmpDir}
	if err := s2.ensureSetupToken(); err != nil {
		t.Fatalf("ensureSetupToken second call failed: %v", err)
	}
	if s2.setupToken != originalTok {
		t.Errorf("expected reuse of existing token; got %q, want %q", s2.setupToken, originalTok)
	}
}

func TestHandleConfigRestore_AlreadyComplete(t *testing.T) {
	s := &Server{}
	s.needsSetup.Store(false)

	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader("apps: []"))
	req.Header.Set("Content-Type", "application/x-yaml")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestHandleConfigRestore_InvalidYAML(t *testing.T) {
	s := &Server{}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader(":::not yaml"))
	req.Header.Set("Content-Type", "application/x-yaml")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestHandleConfigRestore_RollbackOnSaveFailure covers findings.md H9.
// When Save fails, the in-memory config must NOT be swapped to the
// restored state, or a restart would revert to the old disk config
// while the running instance ran the new one.
func TestHandleConfigRestore_RollbackOnSaveFailure(t *testing.T) {
	tmpDir := t.TempDir()
	// Point configPath at a directory, not a file. config.Save writes a
	// temp file and renames over the path, which fails when the target
	// is an existing directory.
	configPath := filepath.Join(tmpDir, "cfgdir")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatal(err)
	}

	original := *defaultTestConfig()
	original.Server.Title = "Original"
	s := &Server{
		config:     &original,
		configPath: configPath,
		dataDir:    tmpDir,
	}
	s.needsSetup.Store(true)

	yamlContent := `server:
  title: "Restored"
apps: []
`
	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader(yamlContent))
	req.Header.Set("Content-Type", "application/x-yaml")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when Save fails, got %d: %s", rec.Code, rec.Body.String())
	}
	if s.config.Server.Title != "Original" {
		t.Errorf("in-memory config was NOT rolled back: Title = %q", s.config.Server.Title)
	}
	if !s.needsSetup.Load() {
		t.Error("needsSetup should stay true when restore failed")
	}
}

func TestHandleConfigRestore_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := defaultTestConfig()
	s := &Server{
		config:     cfg,
		configPath: configPath,
		dataDir:    tmpDir,
	}
	s.needsSetup.Store(true)

	yamlContent := `server:
  listen: ":8080"
  title: "Restored"
apps:
  - name: TestApp
    url: http://localhost:9999
    enabled: true
groups:
  - name: TestGroup
    color: "#ff0000"
`
	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader(yamlContent))
	req.Header.Set("Content-Type", "application/x-yaml")
	withSetupToken(s, req)
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify response
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != "true" {
		t.Errorf("expected success=true, got %v", resp["success"])
	}

	// Verify setup is complete
	if s.needsSetup.Load() {
		t.Error("expected needsSetup=false after restore")
	}

	// Verify config was applied
	if s.config.Auth.SetupComplete != true {
		t.Error("expected SetupComplete=true")
	}
	if len(s.config.Apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(s.config.Apps))
	}
	if s.config.Apps[0].Name != "TestApp" {
		t.Errorf("expected app name TestApp, got %s", s.config.Apps[0].Name)
	}
	if len(s.config.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(s.config.Groups))
	}

	// Verify config was saved to disk
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to be saved")
	}
}

func TestHandleSetup_WrongMethod(t *testing.T) {
	s := &Server{}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/setup", nil)
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleSetup_AlreadyComplete(t *testing.T) {
	s := &Server{}
	s.needsSetup.Store(false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"method":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestHandleSetup_InvalidMethod(t *testing.T) {
	s := &Server{}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"method":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSetup_InvalidJSON(t *testing.T) {
	s := &Server{}
	s.needsSetup.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSetup_Builtin_WeakPassword(t *testing.T) {
	s := &Server{
		config:     defaultTestConfig(),
		configPath: filepath.Join(t.TempDir(), "config.yaml"),
	}
	s.needsSetup.Store(true)
	s.sessionStore = auth.NewSessionStore("test", time.Hour, false)
	s.userStore = auth.NewUserStore()
	s.authMiddleware = auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, s.sessionStore, s.userStore)

	body := `{"method":"builtin","username":"admin","password":"short"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSetup_Builtin_EmptyUsername(t *testing.T) {
	s := &Server{
		config:     defaultTestConfig(),
		configPath: filepath.Join(t.TempDir(), "config.yaml"),
	}
	s.needsSetup.Store(true)
	s.sessionStore = auth.NewSessionStore("test", time.Hour, false)
	s.userStore = auth.NewUserStore()
	s.authMiddleware = auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, s.sessionStore, s.userStore)

	body := `{"method":"builtin","username":"","password":"longenoughpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSetup_ForwardAuth_MissingProxy(t *testing.T) {
	s := &Server{
		config:     defaultTestConfig(),
		configPath: filepath.Join(t.TempDir(), "config.yaml"),
	}
	s.needsSetup.Store(true)
	s.sessionStore = auth.NewSessionStore("test", time.Hour, false)
	s.userStore = auth.NewUserStore()
	s.authMiddleware = auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, s.sessionStore, s.userStore)

	body := `{"method":"forward_auth","trusted_proxies":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSetup_Builtin_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := defaultTestConfig()
	s := &Server{
		config:     cfg,
		configPath: configPath,
	}
	s.needsSetup.Store(true)
	s.sessionStore = auth.NewSessionStore("test", time.Hour, false)
	s.userStore = auth.NewUserStore()
	s.authMiddleware = auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, s.sessionStore, s.userStore)

	body := `{"method":"builtin","username":"admin","password":"securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify response
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success=true")
	}
	if resp["method"] != "builtin" {
		t.Errorf("expected method=builtin, got %v", resp["method"])
	}

	// Verify setup is complete
	if s.needsSetup.Load() {
		t.Error("expected needsSetup=false after setup")
	}

	// Verify config was updated
	if s.config.Auth.Method != "builtin" {
		t.Errorf("expected auth method builtin, got %s", s.config.Auth.Method)
	}
	if !s.config.Auth.SetupComplete {
		t.Error("expected SetupComplete=true")
	}
	if len(s.config.Auth.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(s.config.Auth.Users))
	}

	// Verify session cookie was set
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "test" {
			found = true
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}

	// Verify user was added to live store
	if s.userStore.Get("admin") == nil {
		t.Error("expected admin user in live store")
	}

	// Verify config was saved to disk
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to be saved")
	}
}

func TestHandleSetup_ForwardAuth_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := defaultTestConfig()
	s := &Server{
		config:     cfg,
		configPath: configPath,
	}
	s.needsSetup.Store(true)
	s.sessionStore = auth.NewSessionStore("test", time.Hour, false)
	s.userStore = auth.NewUserStore()
	s.authMiddleware = auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, s.sessionStore, s.userStore)

	body := `{"method":"forward_auth","trusted_proxies":["10.0.0.0/8"],"headers":{"user":"Remote-User","email":"Remote-Email"},"logout_url":"https://auth.example.com/logout"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if s.config.Auth.Method != "forward_auth" {
		t.Errorf("expected forward_auth, got %s", s.config.Auth.Method)
	}
	if len(s.config.Auth.TrustedProxies) != 1 {
		t.Errorf("expected 1 trusted proxy, got %d", len(s.config.Auth.TrustedProxies))
	}
	if s.config.Auth.LogoutURL != "https://auth.example.com/logout" {
		t.Errorf("expected logout_url, got %q", s.config.Auth.LogoutURL)
	}
}

func TestHandleSetup_None_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := defaultTestConfig()
	s := &Server{
		config:     cfg,
		configPath: configPath,
	}
	s.needsSetup.Store(true)
	s.sessionStore = auth.NewSessionStore("test", time.Hour, false)
	s.userStore = auth.NewUserStore()
	s.authMiddleware = auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethodNone}, s.sessionStore, s.userStore)

	body := `{"method":"none"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if s.config.Auth.Method != "none" {
		t.Errorf("expected none, got %s", s.config.Auth.Method)
	}
}

// TestAppearanceResponse_ResolveAndParse covers issue #321's
// /api/appearance endpoint: the handler resolves (family, variant)
// to the theme_id the frontend would actually apply, reads the
// matching CSS file, and extracts the allowlisted custom properties.
func TestAppearanceResponse_ResolveAndParse(t *testing.T) {
	t.Run("resolveThemeID", func(t *testing.T) {
		cases := []struct {
			family, variant string
			wantID          string
			wantDark        bool
		}{
			{"default", "dark", "muximux", true},
			{"default", "light", "muximux-light", false},
			{"default", "system", "muximux", true}, // server-side default
			{"", "dark", "muximux", true},          // empty family treated as default
			{"catppuccin", "dark", "catppuccin", true},
			{"catppuccin", "light", "catppuccin-light", false},
		}
		for _, c := range cases {
			id, isDark := resolveThemeID(c.family, c.variant)
			if id != c.wantID || isDark != c.wantDark {
				t.Errorf("resolveThemeID(%q,%q) = (%q,%v), want (%q,%v)",
					c.family, c.variant, id, isDark, c.wantID, c.wantDark)
			}
		}
	})

	t.Run("extractCSSVars", func(t *testing.T) {
		css := []byte(`
			[data-theme="test"] {
				--bg-base: #111;
				--bg-surface: rgb(0,0,0);
				--not-allowed: #fff;
				--text-primary: #eee;
			}
			:root {
				--bg-base: #000;
				--accent-primary: #0f0;
			}
		`)
		got := extractCSSVars(css, "test", []string{"--bg-base", "--bg-surface", "--text-primary", "--accent-primary"})
		want := map[string]string{
			"--bg-base":        "#111", // [data-theme="test"] overrides :root, matching browser cascade
			"--bg-surface":     "rgb(0,0,0)",
			"--text-primary":   "#eee",
			"--accent-primary": "#0f0", // only present in :root
		}
		if len(got) != len(want) {
			t.Fatalf("got %v vars, want %v (%v vs %v)", len(got), len(want), got, want)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %q, want %q", k, got[k], v)
			}
		}
	})

	t.Run("missing theme yields empty colors", func(t *testing.T) {
		got := extractCSSVars(nil, "nonexistent", []string{"--bg-base"})
		if got != nil {
			t.Errorf("expected nil for empty input, got %v", got)
		}
	})
}

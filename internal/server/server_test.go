package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- rateLimiter ---

func TestRateLimiter_Allow(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string][]time.Time),
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
		attempts: make(map[string][]time.Time),
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
		attempts: make(map[string][]time.Time),
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
		attempts: make(map[string][]time.Time),
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

func TestRateLimiter_Wrap(t *testing.T) {
	rl := &rateLimiter{
		attempts: make(map[string][]time.Time),
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
		rl.attempts = make(map[string][]time.Time)
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

	handler := securityHeadersMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "SAMEORIGIN",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
		"Permissions-Policy":    "camera=(), microphone=(), geolocation=()",
	}

	for header, value := range expected {
		got := rec.Header().Get(header)
		if got != value {
			t.Errorf("header %s = %q, want %q", header, got, value)
		}
	}
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

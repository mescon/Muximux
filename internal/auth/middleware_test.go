package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// okHandler is a simple handler that writes 200 OK.
func okHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// newTestMiddleware creates a Middleware with the given config, using fresh stores.
func newTestMiddleware(cfg AuthConfig) (*Middleware, *SessionStore, *UserStore) {
	ss := NewSessionStore("test_session", time.Hour, false)
	us := NewUserStore()
	m := NewMiddleware(cfg, ss, us)
	return m, ss, us
}

// --- RequireAuth ---

func TestRequireAuth_Disabled(t *testing.T) {
	m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodNone})

	var captured *User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := m.RequireAuth(inner)
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if captured == nil {
		t.Fatal("expected virtual admin user in context")
	}
	if captured.Role != RoleAdmin {
		t.Errorf("expected role admin, got %s", captured.Role)
	}
	if captured.Username != "admin" {
		t.Errorf("expected username admin, got %s", captured.Username)
	}
}

func TestRequireAuth_Session(t *testing.T) {
	cfg := AuthConfig{Method: AuthMethodBuiltin}
	m, ss, us := newTestMiddleware(cfg)

	// Create a user and session
	us.LoadFromConfig([]UserConfig{
		{Username: "testuser", PasswordHash: mustHash("pass"), Role: RoleUser},
	})
	session, err := ss.Create("testuser", "testuser", RoleUser)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid session passes through", func(t *testing.T) {
		var captured *User
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = GetUserFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := m.RequireAuth(inner)
		req := httptest.NewRequest(http.MethodGet, "/api/something", nil)
		req.AddCookie(&http.Cookie{Name: "test_session", Value: session.ID})
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if captured == nil {
			t.Fatal("expected user in context")
		}
		if captured.Username != "testuser" {
			t.Errorf("expected testuser, got %s", captured.Username)
		}
	})

	t.Run("no session returns 401 or redirect", func(t *testing.T) {
		handler := m.RequireAuth(okHandler())
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for API request, got %d", rec.Code)
		}
	})

	t.Run("expired session returns 401", func(t *testing.T) {
		shortSS := NewSessionStore("short_session", 1*time.Millisecond, false)
		shortM := NewMiddleware(cfg, shortSS, us)

		s, _ := shortSS.Create("testuser", "testuser", RoleUser)
		time.Sleep(5 * time.Millisecond) // Let it expire

		handler := shortM.RequireAuth(okHandler())
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.AddCookie(&http.Cookie{Name: "short_session", Value: s.ID})
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("browser request redirects to login", func(t *testing.T) {
		handler := m.RequireAuth(okHandler())
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("expected 302 redirect, got %d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if loc != "/login" {
			t.Errorf("expected redirect to /login, got %s", loc)
		}
	})
}

func TestRequireAuth_Bypass(t *testing.T) {
	cfg := AuthConfig{
		Method: AuthMethodBuiltin,
		BypassRules: []BypassRule{
			{Path: "/public"},
		},
	}
	m, _, _ := newTestMiddleware(cfg)

	handler := m.RequireAuth(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for bypassed path, got %d", rec.Code)
	}
}

// --- shouldBypass ---

func TestShouldBypass(t *testing.T) {
	tests := []struct {
		name    string
		rules   []BypassRule
		path    string
		method  string
		headers map[string]string
		remote  string
		want    bool
	}{
		{
			name:   "matching path",
			rules:  []BypassRule{{Path: "/public"}},
			path:   "/public",
			method: "GET",
			remote: "127.0.0.1:1234",
			want:   true,
		},
		{
			name:   "wildcard path match",
			rules:  []BypassRule{{Path: "/assets/*"}},
			path:   "/assets/style.css",
			method: "GET",
			remote: "127.0.0.1:1234",
			want:   true,
		},
		{
			name:   "non-matching path",
			rules:  []BypassRule{{Path: "/public"}},
			path:   "/private",
			method: "GET",
			remote: "127.0.0.1:1234",
			want:   false,
		},
		{
			name:   "method match",
			rules:  []BypassRule{{Path: "/hook", Methods: []string{"POST"}}},
			path:   "/hook",
			method: "POST",
			remote: "127.0.0.1:1234",
			want:   true,
		},
		{
			name:   "method mismatch",
			rules:  []BypassRule{{Path: "/hook", Methods: []string{"POST"}}},
			path:   "/hook",
			method: "GET",
			remote: "127.0.0.1:1234",
			want:   false,
		},
		{
			name:   "IP match",
			rules:  []BypassRule{{Path: "/restricted", AllowedIPs: []string{"10.0.0.1"}}},
			path:   "/restricted",
			method: "GET",
			remote: "10.0.0.1:5000",
			want:   true,
		},
		{
			name:   "IP no match",
			rules:  []BypassRule{{Path: "/restricted", AllowedIPs: []string{"10.0.0.1"}}},
			path:   "/restricted",
			method: "GET",
			remote: "10.0.0.2:5000",
			want:   false,
		},
		{
			name:   "CIDR match",
			rules:  []BypassRule{{Path: "/internal", AllowedIPs: []string{"192.168.1.0/24"}}},
			path:   "/internal",
			method: "GET",
			remote: "192.168.1.50:8080",
			want:   true,
		},
		{
			name:   "CIDR no match",
			rules:  []BypassRule{{Path: "/internal", AllowedIPs: []string{"192.168.1.0/24"}}},
			path:   "/internal",
			method: "GET",
			remote: "192.168.2.50:8080",
			want:   false,
		},
		{
			name:    "API key match",
			rules:   []BypassRule{{Path: "/api/webhook", RequireAPIKey: true}},
			path:    "/api/webhook",
			method:  "GET",
			headers: map[string]string{"X-Api-Key": "secret123"},
			remote:  "127.0.0.1:1234",
			want:    true,
		},
		{
			name:   "API key required but missing",
			rules:  []BypassRule{{Path: "/api/webhook", RequireAPIKey: true}},
			path:   "/api/webhook",
			method: "GET",
			remote: "127.0.0.1:1234",
			want:   false,
		},
		{
			name:    "API key wrong value",
			rules:   []BypassRule{{Path: "/api/webhook", RequireAPIKey: true}},
			path:    "/api/webhook",
			method:  "GET",
			headers: map[string]string{"X-Api-Key": "wrongkey"},
			remote:  "127.0.0.1:1234",
			want:    false,
		},
		{
			name:   "empty rules means no bypass",
			rules:  nil,
			path:   "/anything",
			method: "GET",
			remote: "127.0.0.1:1234",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AuthConfig{
				Method:      AuthMethodBuiltin,
				BypassRules: tt.rules,
				APIKey:      "secret123",
			}
			m, _, _ := newTestMiddleware(cfg)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := m.shouldBypass(req)
			if got != tt.want {
				t.Errorf("shouldBypass() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- matchPath ---

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		rulePath    string
		want        bool
	}{
		{"exact match", "/api/health", "/api/health", true},
		{"exact no match", "/api/health", "/api/config", false},
		{"wildcard match", "/assets/style.css", "/assets/*", true},
		{"wildcard prefix", "/assets/js/app.js", "/assets/*", true},
		{"wildcard no match", "/other/file", "/assets/*", false},
		{"empty rule path matches all", "/anything", "", true},
		{"root exact", "/", "/", true},
		{"no wildcard suffix is exact match", "/app.js", "/*.js", false}, // /*.js has no trailing *, so it's exact match
		{"exact match with star in path", "/*.js", "/*.js", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := BypassRule{Path: tt.rulePath}
			got := matchPath(tt.requestPath, rule)
			if got != tt.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.requestPath, tt.rulePath, got, tt.want)
			}
		})
	}
}

// --- matchAllowedIPs ---

func TestMatchAllowedIPs(t *testing.T) {
	tests := []struct {
		name       string
		allowedIPs []string
		remoteAddr string
		want       bool
	}{
		{
			name:       "empty allowlist permits all",
			allowedIPs: nil,
			remoteAddr: "1.2.3.4:80",
			want:       true,
		},
		{
			name:       "single IP match",
			allowedIPs: []string{"10.0.0.1"},
			remoteAddr: "10.0.0.1:80",
			want:       true,
		},
		{
			name:       "single IP no match",
			allowedIPs: []string{"10.0.0.1"},
			remoteAddr: "10.0.0.2:80",
			want:       false,
		},
		{
			name:       "CIDR range match",
			allowedIPs: []string{"172.16.0.0/12"},
			remoteAddr: "172.20.5.3:9090",
			want:       true,
		},
		{
			name:       "CIDR range no match",
			allowedIPs: []string{"172.16.0.0/12"},
			remoteAddr: "192.168.1.1:80",
			want:       false,
		},
		{
			name:       "multiple entries second matches",
			allowedIPs: []string{"10.0.0.0/8", "192.168.0.0/16"},
			remoteAddr: "192.168.1.5:80",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin})
			rule := BypassRule{
				Path:       "/test",
				AllowedIPs: tt.allowedIPs,
			}
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			got := m.matchAllowedIPs(req, rule)
			if got != tt.want {
				t.Errorf("matchAllowedIPs() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- getClientIP ---

func TestGetClientIP(t *testing.T) {
	t.Run("no X-Forwarded-For from untrusted proxy", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{
			Method:         AuthMethodBuiltin,
			TrustedProxies: []string{"10.0.0.0/8"},
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		req.Header.Set("X-Forwarded-For", "1.2.3.4")

		got := m.getClientIP(req)
		// Not from trusted proxy so XFF is ignored
		if got != "192.168.1.1" {
			t.Errorf("expected 192.168.1.1, got %s", got)
		}
	})

	t.Run("X-Forwarded-For trusted proxy", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{
			Method:         AuthMethodBuiltin,
			TrustedProxies: []string{"10.0.0.0/8"},
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.1")

		got := m.getClientIP(req)
		if got != "203.0.113.50" {
			t.Errorf("expected 203.0.113.50, got %s", got)
		}
	})

	t.Run("X-Real-IP trusted proxy", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{
			Method:         AuthMethodBuiltin,
			TrustedProxies: []string{"10.0.0.1"},
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		req.Header.Set("X-Real-IP", "198.51.100.10")

		got := m.getClientIP(req)
		if got != "198.51.100.10" {
			t.Errorf("expected 198.51.100.10, got %s", got)
		}
	})

	t.Run("no forwarded headers falls back to remote addr", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{
			Method:         AuthMethodBuiltin,
			TrustedProxies: []string{"10.0.0.0/8"},
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.5:4444"

		got := m.getClientIP(req)
		if got != "10.0.0.5" {
			t.Errorf("expected 10.0.0.5, got %s", got)
		}
	})

	t.Run("no trusted proxies configured", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.5.5.5:80"
		req.Header.Set("X-Forwarded-For", "1.2.3.4")

		got := m.getClientIP(req)
		if got != "5.5.5.5" {
			t.Errorf("expected 5.5.5.5, got %s", got)
		}
	})
}

// --- authenticateRequest ---

func TestAuthenticateRequest(t *testing.T) {
	t.Run("builtin with valid session", func(t *testing.T) {
		cfg := AuthConfig{Method: AuthMethodBuiltin}
		m, ss, us := newTestMiddleware(cfg)

		us.LoadFromConfig([]UserConfig{
			{Username: "alice", PasswordHash: mustHash("pass"), Role: RoleAdmin},
		})
		session, _ := ss.Create("alice", "alice", RoleAdmin)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "test_session", Value: session.ID})

		user, sess := m.authenticateRequest(req)
		if user == nil {
			t.Fatal("expected user from session")
		}
		if user.Username != "alice" {
			t.Errorf("expected alice, got %s", user.Username)
		}
		if sess == nil {
			t.Fatal("expected session")
		}
	})

	t.Run("builtin with no session", func(t *testing.T) {
		cfg := AuthConfig{Method: AuthMethodBuiltin}
		m, _, _ := newTestMiddleware(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		user, sess := m.authenticateRequest(req)
		if user != nil {
			t.Error("expected nil user")
		}
		if sess != nil {
			t.Error("expected nil session")
		}
	})

	t.Run("forward_auth with trusted proxy", func(t *testing.T) {
		cfg := AuthConfig{
			Method:         AuthMethodForwardAuth,
			TrustedProxies: []string{"10.0.0.0/8"},
		}
		m, _, _ := newTestMiddleware(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:80"
		req.Header.Set("Remote-User", "bob")
		req.Header.Set("Remote-Email", "bob@example.com")

		user, _ := m.authenticateRequest(req)
		if user == nil {
			t.Fatal("expected user from forward auth")
		}
		if user.Username != "bob" {
			t.Errorf("expected bob, got %s", user.Username)
		}
	})

	t.Run("forward_auth from untrusted proxy", func(t *testing.T) {
		cfg := AuthConfig{
			Method:         AuthMethodForwardAuth,
			TrustedProxies: []string{"10.0.0.0/8"},
		}
		m, _, _ := newTestMiddleware(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:80"
		req.Header.Set("Remote-User", "eve")

		user, _ := m.authenticateRequest(req)
		if user != nil {
			t.Error("expected nil user from untrusted proxy")
		}
	})

	t.Run("forward_auth admin groups", func(t *testing.T) {
		cfg := AuthConfig{
			Method:         AuthMethodForwardAuth,
			TrustedProxies: []string{"10.0.0.0/8"},
		}
		m, _, _ := newTestMiddleware(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:80"
		req.Header.Set("Remote-User", "charlie")
		req.Header.Set("Remote-Groups", "users, admins")

		user, _ := m.authenticateRequest(req)
		if user == nil {
			t.Fatal("expected user")
		}
		if user.Role != RoleAdmin {
			t.Errorf("expected admin role, got %s", user.Role)
		}
	})

	t.Run("unknown auth method returns nil", func(t *testing.T) {
		cfg := AuthConfig{Method: "something_unknown"}
		m, _, _ := newTestMiddleware(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		user, sess := m.authenticateRequest(req)
		if user != nil || sess != nil {
			t.Error("expected nil for unknown auth method")
		}
	})
}

// --- RequireRole ---

func TestRequireRole(t *testing.T) {
	m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodNone})

	t.Run("admin role allowed", func(t *testing.T) {
		inner := okHandler()
		handler := m.RequireRole(RoleAdmin)(inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), ContextKeyUser, &User{Role: RoleAdmin})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("user role forbidden", func(t *testing.T) {
		inner := okHandler()
		handler := m.RequireRole(RoleAdmin)(inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), ContextKeyUser, &User{Role: RoleUser})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("no user returns 401", func(t *testing.T) {
		inner := okHandler()
		handler := m.RequireRole(RoleAdmin)(inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("multiple roles accepted", func(t *testing.T) {
		inner := okHandler()
		handler := m.RequireRole(RoleAdmin, RoleUser)(inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), ContextKeyUser, &User{Role: RoleUser})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

// --- GetUserFromContext / GetSessionFromContext ---

func TestContextHelpers(t *testing.T) {
	t.Run("user from context", func(t *testing.T) {
		user := &User{ID: "u1", Username: "test", Role: RoleUser}
		ctx := context.WithValue(context.Background(), ContextKeyUser, user)

		got := GetUserFromContext(ctx)
		if got == nil || got.Username != "test" {
			t.Error("expected user from context")
		}
	})

	t.Run("nil user from empty context", func(t *testing.T) {
		got := GetUserFromContext(context.Background())
		if got != nil {
			t.Error("expected nil user from empty context")
		}
	})

	t.Run("session from context", func(t *testing.T) {
		session := &Session{ID: "s1", UserID: "u1"}
		ctx := context.WithValue(context.Background(), ContextKeySession, session)

		got := GetSessionFromContext(ctx)
		if got == nil || got.ID != "s1" {
			t.Error("expected session from context")
		}
	})

	t.Run("nil session from empty context", func(t *testing.T) {
		got := GetSessionFromContext(context.Background())
		if got != nil {
			t.Error("expected nil session from empty context")
		}
	})
}

// --- NewMiddleware trusted proxy parsing ---

func TestNewMiddleware_TrustedProxyParsing(t *testing.T) {
	t.Run("CIDR notation", func(t *testing.T) {
		cfg := AuthConfig{
			Method:         AuthMethodBuiltin,
			TrustedProxies: []string{"10.0.0.0/8"},
		}
		m := NewMiddleware(cfg, NewSessionStore("t", time.Hour, false), NewUserStore())
		if len(m.trustedNets) != 1 {
			t.Fatalf("expected 1 trusted network, got %d", len(m.trustedNets))
		}
	})

	t.Run("single IPv4", func(t *testing.T) {
		cfg := AuthConfig{
			Method:         AuthMethodBuiltin,
			TrustedProxies: []string{"127.0.0.1"},
		}
		m := NewMiddleware(cfg, NewSessionStore("t", time.Hour, false), NewUserStore())
		if len(m.trustedNets) != 1 {
			t.Fatalf("expected 1 trusted network, got %d", len(m.trustedNets))
		}
	})

	t.Run("invalid string is ignored", func(t *testing.T) {
		cfg := AuthConfig{
			Method:         AuthMethodBuiltin,
			TrustedProxies: []string{"not-an-ip"},
		}
		m := NewMiddleware(cfg, NewSessionStore("t", time.Hour, false), NewUserStore())
		if len(m.trustedNets) != 0 {
			t.Fatalf("expected 0 trusted networks for invalid input, got %d", len(m.trustedNets))
		}
	})
}

// --- matchMethod ---

func TestMatchMethod(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		methods []string
		want    bool
	}{
		{"empty methods matches all", "GET", nil, true},
		{"exact match", "POST", []string{"POST"}, true},
		{"case insensitive", "get", []string{"GET"}, true},
		{"no match", "DELETE", []string{"GET", "POST"}, false},
		{"multiple methods", "PUT", []string{"GET", "PUT", "DELETE"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := BypassRule{Methods: tt.methods}
			got := matchMethod(tt.method, rule)
			if got != tt.want {
				t.Errorf("matchMethod(%q, %v) = %v, want %v", tt.method, tt.methods, got, tt.want)
			}
		})
	}
}

// --- matchAPIKey ---

func TestMatchAPIKey(t *testing.T) {
	t.Run("not required always passes", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin, APIKey: "secret"})
		rule := BypassRule{RequireAPIKey: false}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if !m.matchAPIKey(req, rule) {
			t.Error("expected true when not required")
		}
	})

	t.Run("correct key", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin, APIKey: "mysecret"})
		rule := BypassRule{RequireAPIKey: true}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Api-Key", "mysecret")
		if !m.matchAPIKey(req, rule) {
			t.Error("expected true for correct API key")
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin, APIKey: "mysecret"})
		rule := BypassRule{RequireAPIKey: true}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Api-Key", "wrong")
		if m.matchAPIKey(req, rule) {
			t.Error("expected false for wrong API key")
		}
	})

	t.Run("missing key header", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin, APIKey: "mysecret"})
		rule := BypassRule{RequireAPIKey: true}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if m.matchAPIKey(req, rule) {
			t.Error("expected false for missing API key")
		}
	})

	t.Run("empty configured key", func(t *testing.T) {
		m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin, APIKey: ""})
		rule := BypassRule{RequireAPIKey: true}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Api-Key", "anything")
		if m.matchAPIKey(req, rule) {
			t.Error("expected false when no API key configured")
		}
	})
}

// --- handleUnauthenticated ---

func TestHandleUnauthenticated(t *testing.T) {
	m, _, _ := newTestMiddleware(AuthConfig{Method: AuthMethodBuiltin})

	t.Run("API path returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/something", nil)
		rec := httptest.NewRecorder()
		m.handleUnauthenticated(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("non-API path redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()
		m.handleUnauthenticated(rec, req)
		if rec.Code != http.StatusFound {
			t.Errorf("expected 302, got %d", rec.Code)
		}
	})
}

func TestUpdateConfig_SwitchMethod(t *testing.T) {
	cfg := AuthConfig{Method: AuthMethodNone}
	m, ss, us := newTestMiddleware(cfg)

	// Start with none — should get virtual admin
	var captured *User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := m.RequireAuth(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == nil || captured.Username != "admin" {
		t.Fatal("expected virtual admin before update")
	}

	// Load a user and switch to builtin
	hash, _ := HashPassword("testpass")
	us.LoadFromConfig([]UserConfig{{Username: "alice", PasswordHash: hash, Role: RoleAdmin}})
	session, _ := ss.Create("alice", "alice", RoleAdmin)

	m.UpdateConfig(AuthConfig{Method: AuthMethodBuiltin})

	// Now without a session, should get 401
	captured = nil
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after switching to builtin, got %d", rec.Code)
	}

	// With session, should get alice
	captured = nil
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "test_session", Value: session.ID})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == nil || captured.Username != "alice" {
		t.Fatal("expected alice after switching to builtin with session")
	}
}

func TestUpdateConfig_TrustedProxies(t *testing.T) {
	cfg := AuthConfig{Method: AuthMethodForwardAuth}
	m, _, _ := newTestMiddleware(cfg)

	// Before update — no trusted proxies, should reject
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:80"
	req.Header.Set("Remote-User", "bob")
	user, _ := m.authenticateRequest(req)
	if user != nil {
		t.Error("expected nil user before updating trusted proxies")
	}

	// Update with trusted proxies
	m.UpdateConfig(AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	})

	user, _ = m.authenticateRequest(req)
	if user == nil {
		t.Fatal("expected user after updating trusted proxies")
	}
	if user.Username != "bob" {
		t.Errorf("expected bob, got %s", user.Username)
	}
}

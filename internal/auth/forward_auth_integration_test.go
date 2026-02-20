package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestForwardAuth_Authelia simulates a request from Authelia using its
// default header names (Remote-User, Remote-Email, Remote-Groups, Remote-Name).
func TestForwardAuth_Authelia(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
		// Authelia uses the default header names, so no custom headers needed
	}
	m, _, _ := newTestMiddleware(cfg)

	var capturedUser *User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := m.RequireAuth(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.RemoteAddr = "10.0.0.1:80"
	req.Header.Set("Remote-User", "authelia_user")
	req.Header.Set("Remote-Email", "user@authelia.example.com")
	req.Header.Set("Remote-Name", "Authelia User")
	req.Header.Set("Remote-Groups", "users, admins")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedUser == nil {
		t.Fatal("expected user in context")
	}
	if capturedUser.Username != "authelia_user" {
		t.Errorf("Username = %q, want authelia_user", capturedUser.Username)
	}
	if capturedUser.Email != "user@authelia.example.com" {
		t.Errorf("Email = %q, want user@authelia.example.com", capturedUser.Email)
	}
	if capturedUser.DisplayName != "Authelia User" {
		t.Errorf("DisplayName = %q, want 'Authelia User'", capturedUser.DisplayName)
	}
	if capturedUser.Role != RoleAdmin {
		t.Errorf("Role = %q, want admin (user in 'admins' group)", capturedUser.Role)
	}
}

// TestForwardAuth_Authentik simulates a request from Authentik using custom
// header names (X-authentik-username, X-authentik-email, etc.).
func TestForwardAuth_Authentik(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"172.16.0.0/12"},
		Headers: ForwardAuthHeaders{
			User:   "X-authentik-username",
			Email:  "X-authentik-email",
			Groups: "X-authentik-groups",
			Name:   "X-authentik-name",
		},
	}
	m, _, _ := newTestMiddleware(cfg)

	var capturedUser *User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := m.RequireAuth(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	req.RemoteAddr = "172.20.1.5:443"
	req.Header.Set("X-authentik-username", "authentik_admin")
	req.Header.Set("X-authentik-email", "admin@authentik.example.com")
	req.Header.Set("X-authentik-name", "Authentik Admin")
	req.Header.Set("X-authentik-groups", "administrators, devops")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedUser == nil {
		t.Fatal("expected user in context")
	}
	if capturedUser.Username != "authentik_admin" {
		t.Errorf("Username = %q, want authentik_admin", capturedUser.Username)
	}
	if capturedUser.Email != "admin@authentik.example.com" {
		t.Errorf("Email = %q, want admin@authentik.example.com", capturedUser.Email)
	}
	if capturedUser.DisplayName != "Authentik Admin" {
		t.Errorf("DisplayName = %q, want 'Authentik Admin'", capturedUser.DisplayName)
	}
	if capturedUser.Role != RoleAdmin {
		t.Errorf("Role = %q, want admin (user in 'administrators' group)", capturedUser.Role)
	}
}

// TestForwardAuth_TraefikForwardAuth simulates a request from thomseddon/traefik-forward-auth,
// which uses X-Forwarded-User and X-Forwarded-Email.
func TestForwardAuth_TraefikForwardAuth(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"192.168.0.0/16"},
		Headers: ForwardAuthHeaders{
			User:  "X-Forwarded-User",
			Email: "X-Forwarded-Email",
		},
	}
	m, _, _ := newTestMiddleware(cfg)

	var capturedUser *User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := m.RequireAuth(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.RemoteAddr = "192.168.1.10:8080"
	req.Header.Set("X-Forwarded-User", "traefik_user")
	req.Header.Set("X-Forwarded-Email", "user@traefik.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedUser == nil {
		t.Fatal("expected user in context")
	}
	if capturedUser.Username != "traefik_user" {
		t.Errorf("Username = %q, want traefik_user", capturedUser.Username)
	}
	if capturedUser.Email != "user@traefik.example.com" {
		t.Errorf("Email = %q, want user@traefik.example.com", capturedUser.Email)
	}
	// No groups header → should be regular user
	if capturedUser.Role != RoleUser {
		t.Errorf("Role = %q, want user (no groups header)", capturedUser.Role)
	}
}

// TestForwardAuth_TrustedProxyCIDR tests that forward auth correctly
// enforces trusted proxy CIDR ranges across different network configurations.
func TestForwardAuth_TrustedProxyCIDR(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		remoteAddr     string
		wantAuth       bool
	}{
		{
			name:           "single /8 range allows 10.x.x.x",
			trustedProxies: []string{"10.0.0.0/8"},
			remoteAddr:     "10.255.255.1:80",
			wantAuth:       true,
		},
		{
			name:           "single /8 range rejects 11.x.x.x",
			trustedProxies: []string{"10.0.0.0/8"},
			remoteAddr:     "11.0.0.1:80",
			wantAuth:       false,
		},
		{
			name:           "/16 range allows matching subnet",
			trustedProxies: []string{"172.16.0.0/16"},
			remoteAddr:     "172.16.5.100:443",
			wantAuth:       true,
		},
		{
			name:           "/16 range rejects different subnet",
			trustedProxies: []string{"172.16.0.0/16"},
			remoteAddr:     "172.17.0.1:443",
			wantAuth:       false,
		},
		{
			name:           "/32 single IP exact match",
			trustedProxies: []string{"192.168.1.100/32"},
			remoteAddr:     "192.168.1.100:80",
			wantAuth:       true,
		},
		{
			name:           "/32 single IP rejects neighbor",
			trustedProxies: []string{"192.168.1.100/32"},
			remoteAddr:     "192.168.1.101:80",
			wantAuth:       false,
		},
		{
			name:           "bare IP (no CIDR) matches exact",
			trustedProxies: []string{"10.0.0.5"},
			remoteAddr:     "10.0.0.5:8080",
			wantAuth:       true,
		},
		{
			name:           "bare IP (no CIDR) rejects different",
			trustedProxies: []string{"10.0.0.5"},
			remoteAddr:     "10.0.0.6:8080",
			wantAuth:       false,
		},
		{
			name:           "multiple ranges, second matches",
			trustedProxies: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
			remoteAddr:     "192.168.50.1:80",
			wantAuth:       true,
		},
		{
			name:           "multiple ranges, none match",
			trustedProxies: []string{"10.0.0.0/8", "172.16.0.0/12"},
			remoteAddr:     "192.168.1.1:80",
			wantAuth:       false,
		},
		{
			name:           "no trusted proxies configured rejects all",
			trustedProxies: nil,
			remoteAddr:     "127.0.0.1:80",
			wantAuth:       false,
		},
		{
			name:           "empty trusted proxies rejects all",
			trustedProxies: []string{},
			remoteAddr:     "10.0.0.1:80",
			wantAuth:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AuthConfig{
				Method:         AuthMethodForwardAuth,
				TrustedProxies: tt.trustedProxies,
			}
			m, _, _ := newTestMiddleware(cfg)

			var capturedUser *User
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedUser = GetUserFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := m.RequireAuth(inner)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("Remote-User", "proxytest")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tt.wantAuth {
				if rec.Code != http.StatusOK {
					t.Errorf("expected 200, got %d", rec.Code)
				}
				if capturedUser == nil {
					t.Error("expected user in context")
				}
			} else if capturedUser != nil {
				t.Error("expected no user from untrusted proxy")
			}
		})
	}
}

// TestForwardAuth_AdminGroupDetection tests the admin group detection
// across different group formats used by various providers.
func TestForwardAuth_AdminGroupDetection(t *testing.T) {
	tests := []struct {
		name     string
		groups   string
		wantRole string
	}{
		{"exact 'admin'", "admin", RoleAdmin},
		{"exact 'admins'", "admins", RoleAdmin},
		{"exact 'administrators'", "administrators", RoleAdmin},
		{"comma-separated with admin", "users, admin, editors", RoleAdmin},
		{"comma-separated with admins", "users, admins", RoleAdmin},
		{"comma-separated with administrators", "staff, administrators", RoleAdmin},
		{"no admin group", "users, editors, viewers", RoleUser},
		{"empty groups", "", RoleUser},
		{"single non-admin group", "users", RoleUser},
		{"admin as substring (should NOT match)", "super-admins", RoleUser},
		{"padded with spaces", "  admins  ", RoleAdmin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AuthConfig{
				Method:         AuthMethodForwardAuth,
				TrustedProxies: []string{"10.0.0.0/8"},
			}
			m, _, _ := newTestMiddleware(cfg)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "10.0.0.1:80"
			req.Header.Set("Remote-User", "grouptest")
			if tt.groups != "" {
				req.Header.Set("Remote-Groups", tt.groups)
			}

			user, _ := m.authenticateRequest(req, m.snapshot())
			if user == nil {
				t.Fatal("expected user")
			}
			if user.Role != tt.wantRole {
				t.Errorf("Role = %q, want %q", user.Role, tt.wantRole)
			}
		})
	}
}

// TestForwardAuth_MissingUserHeader verifies that requests without a user
// header are rejected even when from a trusted proxy.
func TestForwardAuth_MissingUserHeader(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	handler := m.RequireAuth(okHandler())

	// Request from trusted proxy but without Remote-User header
	req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	req.RemoteAddr = "10.0.0.1:80"
	// No Remote-User header
	req.Header.Set("Remote-Email", "orphan@example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing user header, got %d", rec.Code)
	}
}

// TestForwardAuth_EmptyUserHeader verifies that an empty user header is
// treated the same as a missing one.
func TestForwardAuth_EmptyUserHeader(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:80"
	req.Header.Set("Remote-User", "") // Empty

	user, _ := m.authenticateRequest(req, m.snapshot())
	if user != nil {
		t.Error("expected nil user for empty Remote-User header")
	}
}

// TestForwardAuth_HeaderSpoofFromUntrustedProxy verifies that auth headers
// from untrusted sources are rejected, preventing header spoofing attacks.
func TestForwardAuth_HeaderSpoofFromUntrustedProxy(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	var capturedUser *User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := m.RequireAuth(inner)

	// Attacker sends valid-looking headers from outside the trusted network
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.RemoteAddr = "203.0.113.50:80" // Public IP, not trusted
	req.Header.Set("Remote-User", "admin")
	req.Header.Set("Remote-Groups", "admins")
	req.Header.Set("Remote-Email", "admin@example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Must be rejected - untrusted source
	if capturedUser != nil {
		t.Fatal("spoofed headers from untrusted proxy must be rejected")
	}
	if rec.Code == http.StatusOK {
		t.Error("request with spoofed headers should not succeed")
	}
}

// TestForwardAuth_DirectIPOnly verifies that trusted proxy check uses
// the direct TCP connection IP (RemoteAddr), NOT X-Forwarded-For.
// This prevents an attacker from spoofing a trusted proxy IP via XFF.
func TestForwardAuth_DirectIPOnly(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	// Attacker at 203.0.113.50 claims to come from 10.0.0.1 via XFF
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.50:80"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("Remote-User", "spoofed_admin")
	req.Header.Set("Remote-Groups", "admins")

	user, _ := m.authenticateRequest(req, m.snapshot())
	if user != nil {
		t.Fatal("must not trust X-Forwarded-For for trusted proxy verification")
	}
}

// TestForwardAuth_FullMiddlewareChain tests the complete middleware stack:
// RequireAuth → RequireRole → handler, simulating a real protected endpoint.
func TestForwardAuth_FullMiddlewareChain(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	t.Run("admin user accesses admin endpoint", func(t *testing.T) {
		var capturedUser *User
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedUser = GetUserFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := m.RequireAuth(m.RequireRole(RoleAdmin)(inner))

		req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
		req.RemoteAddr = "10.0.0.1:80"
		req.Header.Set("Remote-User", "superadmin")
		req.Header.Set("Remote-Groups", "admins, devops")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if capturedUser == nil || capturedUser.Username != "superadmin" {
			t.Error("expected superadmin user in context")
		}
	})

	t.Run("regular user blocked from admin endpoint", func(t *testing.T) {
		handler := m.RequireAuth(m.RequireRole(RoleAdmin)(okHandler()))

		req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
		req.RemoteAddr = "10.0.0.1:80"
		req.Header.Set("Remote-User", "viewer")
		req.Header.Set("Remote-Groups", "users, viewers")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("unauthenticated API request gets 401", func(t *testing.T) {
		handler := m.RequireAuth(m.RequireRole(RoleAdmin)(okHandler()))

		req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
		req.RemoteAddr = "203.0.113.50:80" // Untrusted
		req.Header.Set("Remote-User", "hacker")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})
}

// TestForwardAuth_ConfigUpdate verifies that updating the auth config
// at runtime correctly applies new trusted proxies and header mappings.
func TestForwardAuth_ConfigUpdate(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	// Before update: only 10.x.x.x trusted
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.16.0.1:80"
	req.Header.Set("Remote-User", "newproxy_user")

	user, _ := m.authenticateRequest(req, m.snapshot())
	if user != nil {
		t.Error("172.16.x.x should not be trusted before config update")
	}

	// Update to also trust 172.16.0.0/12
	m.UpdateConfig(&AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8", "172.16.0.0/12"},
		Headers: ForwardAuthHeaders{
			User:  "X-Custom-User",
			Email: "X-Custom-Email",
		},
	})

	// After update: 172.16.x.x should be trusted with new headers
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "172.16.0.1:80"
	req2.Header.Set("X-Custom-User", "updated_user")
	req2.Header.Set("X-Custom-Email", "updated@example.com")

	user, _ = m.authenticateRequest(req2, m.snapshot())
	if user == nil {
		t.Fatal("expected user after config update with new trusted proxy")
	}
	if user.Username != "updated_user" {
		t.Errorf("Username = %q, want updated_user", user.Username)
	}
	if user.Email != "updated@example.com" {
		t.Errorf("Email = %q, want updated@example.com", user.Email)
	}
}

// TestForwardAuth_SessionFallback verifies that forward auth mode also
// accepts existing sessions (e.g., after OIDC login switched to forward auth).
func TestForwardAuth_SessionFallback(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	ss := NewSessionStore("test_session", time.Hour, false)
	us := NewUserStore()
	m := NewMiddleware(cfg, ss, us)

	// Forward auth does not use sessions — it reads headers each time.
	// A request without trusted proxy headers but with a session cookie
	// should NOT authenticate (forward_auth path doesn't check sessions).
	us.LoadFromConfig([]UserConfig{
		{Username: "session_user", PasswordHash: mustHash("pass"), Role: RoleAdmin},
	})
	session, _ := ss.Create("session_user", "session_user", RoleAdmin)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.50:80" // Not trusted
	req.AddCookie(&http.Cookie{Name: "test_session", Value: session.ID})

	user, _ := m.authenticateRequest(req, m.snapshot())
	// Forward auth mode should NOT fall back to session auth
	if user != nil {
		t.Error("forward_auth should not accept session cookies without trusted proxy headers")
	}
}

// TestForwardAuth_BrowserRedirectVsAPIResponse verifies that unauthenticated
// browser requests get redirected while API requests get 401.
func TestForwardAuth_BrowserRedirectVsAPIResponse(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)
	handler := m.RequireAuth(okHandler())

	t.Run("API path returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
		req.RemoteAddr = "203.0.113.50:80" // Untrusted
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("browser path redirects to login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		req.RemoteAddr = "203.0.113.50:80" // Untrusted
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("expected 302, got %d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/login" {
			t.Errorf("redirect location = %q, want /login", loc)
		}
	})
}

// TestForwardAuth_BasePathRedirect verifies that unauthenticated browser
// requests redirect to the correct login path when base_path is set.
func TestForwardAuth_BasePathRedirect(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
		BasePath:       "/muximux",
	}
	m, _, _ := newTestMiddleware(cfg)
	handler := m.RequireAuth(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/muximux/dashboard", nil)
	req.RemoteAddr = "203.0.113.50:80"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/muximux/login" {
		t.Errorf("redirect location = %q, want /muximux/login", loc)
	}
}

// TestForwardAuth_UserIDConsistency verifies that the user ID is set to
// the username for forward auth users (needed for session-less identification).
func TestForwardAuth_UserIDConsistency(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:80"
	req.Header.Set("Remote-User", "consistent_user")

	user, _ := m.authenticateRequest(req, m.snapshot())
	if user == nil {
		t.Fatal("expected user")
	}
	if user.ID != user.Username {
		t.Errorf("user.ID = %q, user.Username = %q; expected them to match", user.ID, user.Username)
	}
	if user.ID != "consistent_user" {
		t.Errorf("user.ID = %q, want consistent_user", user.ID)
	}
}

// TestForwardAuth_MultipleConcurrentRequests verifies that the middleware
// correctly handles concurrent requests from different users without
// cross-contamination.
func TestForwardAuth_MultipleConcurrentRequests(t *testing.T) {
	cfg := &AuthConfig{
		Method:         AuthMethodForwardAuth,
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	m, _, _ := newTestMiddleware(cfg)

	users := []struct {
		name   string
		groups string
		role   string
	}{
		{"alice", "admins", RoleAdmin},
		{"bob", "users", RoleUser},
		{"charlie", "admins, devops", RoleAdmin},
		{"dave", "viewers", RoleUser},
	}

	for _, u := range users {
		t.Run(u.name, func(t *testing.T) {
			t.Parallel()

			var capturedUser *User
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedUser = GetUserFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := m.RequireAuth(inner)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.RemoteAddr = "10.0.0.1:80"
			req.Header.Set("Remote-User", u.name)
			req.Header.Set("Remote-Groups", u.groups)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			if capturedUser == nil {
				t.Fatal("expected user in context")
			}
			if capturedUser.Username != u.name {
				t.Errorf("Username = %q, want %q", capturedUser.Username, u.name)
			}
			if capturedUser.Role != u.role {
				t.Errorf("Role = %q, want %q", capturedUser.Role, u.role)
			}
		})
	}
}

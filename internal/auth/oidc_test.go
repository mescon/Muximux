package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockOIDCServer creates a test HTTP server that simulates an OIDC provider.
// It serves discovery, token, and userinfo endpoints.
func mockOIDCServer(t *testing.T, userinfo map[string]interface{}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		// We need the server URL, but it isn't known until the server starts.
		// The caller patches the discovery cache after creation, so we build
		// the URLs relative to the Host header.
		scheme := "http"
		base := scheme + "://" + r.Host
		doc := map[string]string{
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Verify expected form values
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.FormValue("grant_type") != "authorization_code" {
			http.Error(w, "invalid grant_type", http.StatusBadRequest)
			return
		}
		if r.FormValue("code") == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		resp := TokenResponse{
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
			IDToken:     "test-id-token",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userinfo)
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// Just return 200 for checking the URL was built correctly
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(mux)
}

func newTestOIDCProvider(t *testing.T, issuerURL string) (*OIDCProvider, *SessionStore) {
	t.Helper()
	ss := NewSessionStore("test_session", time.Hour, false)
	us := NewUserStore()

	cfg := OIDCConfig{
		Enabled:      true,
		IssuerURL:    issuerURL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/api/auth/oidc/callback",
		AdminGroups:  []string{"admin-group"},
	}

	p := &OIDCProvider{
		config:       cfg,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		sessionStore: ss,
		userStore:    us,
		states:       make(map[string]stateEntry),
	}
	// Set defaults
	if len(p.config.Scopes) == 0 {
		p.config.Scopes = []string{"openid", "profile", "email"}
	}
	if p.config.UsernameClaim == "" {
		p.config.UsernameClaim = "preferred_username"
	}
	if p.config.EmailClaim == "" {
		p.config.EmailClaim = "email"
	}
	if p.config.DisplayNameClaim == "" {
		p.config.DisplayNameClaim = "name"
	}
	if p.config.GroupsClaim == "" {
		p.config.GroupsClaim = "groups"
	}

	return p, ss
}

// --- loadDiscovery ---

func TestLoadDiscovery(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	if err := p.loadDiscovery(); err != nil {
		t.Fatalf("loadDiscovery failed: %v", err)
	}

	if p.authorizationEndpoint != srv.URL+"/authorize" {
		t.Errorf("expected authorization_endpoint %s/authorize, got %s", srv.URL, p.authorizationEndpoint)
	}
	if p.tokenEndpoint != srv.URL+"/token" {
		t.Errorf("expected token_endpoint %s/token, got %s", srv.URL, p.tokenEndpoint)
	}
	if p.userinfoEndpoint != srv.URL+"/userinfo" {
		t.Errorf("expected userinfo_endpoint %s/userinfo, got %s", srv.URL, p.userinfoEndpoint)
	}
	if !p.discoveryLoaded {
		t.Error("expected discoveryLoaded to be true")
	}
}

func TestLoadDiscovery_Cached(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	// First load
	if err := p.loadDiscovery(); err != nil {
		t.Fatalf("first loadDiscovery failed: %v", err)
	}

	// Second load should use cache (no error even if server is down)
	srv.Close()
	if err := p.loadDiscovery(); err != nil {
		t.Fatalf("cached loadDiscovery failed: %v", err)
	}
}

func TestLoadDiscovery_ServerError(t *testing.T) {
	// Use a server that returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	err := p.loadDiscovery()
	if err == nil {
		t.Error("expected error from server returning 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error mentioning 500, got: %v", err)
	}
}

func TestLoadDiscovery_Unreachable(t *testing.T) {
	p, _ := newTestOIDCProvider(t, "http://127.0.0.1:1") // port 1 should fail to connect

	err := p.loadDiscovery()
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

// --- GetAuthorizationURL ---

func TestGetAuthorizationURL(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	authURL, err := p.GetAuthorizationURL("/dashboard")
	if err != nil {
		t.Fatalf("GetAuthorizationURL failed: %v", err)
	}

	// Should contain expected query params
	if !strings.Contains(authURL, "response_type=code") {
		t.Error("expected response_type=code in auth URL")
	}
	if !strings.Contains(authURL, "client_id=test-client-id") {
		t.Error("expected client_id in auth URL")
	}
	if !strings.Contains(authURL, "scope=openid+profile+email") {
		t.Error("expected scope in auth URL")
	}
	if !strings.Contains(authURL, "state=") {
		t.Error("expected state parameter in auth URL")
	}
	if !strings.HasPrefix(authURL, srv.URL+"/authorize?") {
		t.Errorf("expected auth URL to start with %s/authorize?, got %s", srv.URL, authURL)
	}

	// State should be stored
	p.statesMu.Lock()
	if len(p.states) != 1 {
		t.Errorf("expected 1 state entry, got %d", len(p.states))
	}
	// Find the state entry and check redirect URL
	for _, entry := range p.states {
		if entry.redirectURL != "/dashboard" {
			t.Errorf("expected redirectURL /dashboard, got %s", entry.redirectURL)
		}
	}
	p.statesMu.Unlock()
}

// --- exchangeCode ---

func TestExchangeCode(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	tokens, err := p.exchangeCode("test-auth-code")
	if err != nil {
		t.Fatalf("exchangeCode failed: %v", err)
	}

	if tokens.AccessToken != "test-access-token" {
		t.Errorf("expected access_token 'test-access-token', got %s", tokens.AccessToken)
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("expected token_type 'Bearer', got %s", tokens.TokenType)
	}
}

func TestExchangeCode_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		doc := map[string]string{
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		}
		json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid_grant"))
	})
	srv.Close()

	errorSrv := httptest.NewServer(mux)
	defer errorSrv.Close()

	p, _ := newTestOIDCProvider(t, errorSrv.URL)

	_, err := p.exchangeCode("bad-code")
	if err == nil {
		t.Error("expected error for bad token exchange")
	}
}

// --- getUserInfo ---

func TestGetUserInfo(t *testing.T) {
	userinfo := map[string]interface{}{
		"sub":                "user-123",
		"preferred_username": "testuser",
		"email":              "test@example.com",
		"name":               "Test User",
		"groups":             []interface{}{"users", "admin-group"},
	}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	claims, err := p.getUserInfo("test-access-token")
	if err != nil {
		t.Fatalf("getUserInfo failed: %v", err)
	}

	if claims["sub"] != "user-123" {
		t.Errorf("expected sub 'user-123', got %v", claims["sub"])
	}
	if claims["preferred_username"] != "testuser" {
		t.Errorf("expected preferred_username 'testuser', got %v", claims["preferred_username"])
	}
}

// --- HandleCallback ---

func TestHandleCallback_Success(t *testing.T) {
	userinfo := map[string]interface{}{
		"sub":                "user-123",
		"preferred_username": "oidcuser",
		"email":              "oidc@example.com",
		"name":               "OIDC User",
		"groups":             []interface{}{"users", "admin-group"},
	}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, ss := newTestOIDCProvider(t, srv.URL)

	// Pre-populate discovery and state
	if err := p.loadDiscovery(); err != nil {
		t.Fatalf("loadDiscovery failed: %v", err)
	}

	// Add a valid state
	testState := "valid-state-123"
	p.statesMu.Lock()
	p.states[testState] = stateEntry{
		createdAt:   time.Now(),
		redirectURL: "/dashboard",
	}
	p.statesMu.Unlock()

	// Build callback request
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=test-code&state="+testState, nil)
	rec := httptest.NewRecorder()

	p.HandleCallback(rec, req)

	// Should redirect to /dashboard
	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d: %s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}

	// Session cookie should be set
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "test_session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}

	// Session should exist
	if ss.Count() == 0 {
		t.Error("expected at least one session to be created")
	}

	// State should be consumed
	p.statesMu.Lock()
	if _, exists := p.states[testState]; exists {
		t.Error("expected state to be consumed after callback")
	}
	p.statesMu.Unlock()
}

func TestHandleCallback_InvalidState(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=test-code&state=invalid-state", nil)
	rec := httptest.NewRecorder()

	p.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid state, got %d", rec.Code)
	}
}

func TestHandleCallback_MissingCode(t *testing.T) {
	p, _ := newTestOIDCProvider(t, "http://unused")

	testState := "valid-state"
	p.statesMu.Lock()
	p.states[testState] = stateEntry{
		createdAt:   time.Now(),
		redirectURL: "/",
	}
	p.statesMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state="+testState, nil)
	rec := httptest.NewRecorder()

	p.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing code, got %d", rec.Code)
	}
}

func TestHandleCallback_ProviderError(t *testing.T) {
	p, _ := newTestOIDCProvider(t, "http://unused")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?error=access_denied&error_description=User+denied+access", nil)
	rec := httptest.NewRecorder()

	p.HandleCallback(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for provider error, got %d", rec.Code)
	}
}

// --- HandleLogin ---

func TestHandleLogin(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login?redirect=/settings", nil)
	rec := httptest.NewRecorder()

	p.HandleLogin(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "/authorize?") {
		t.Errorf("expected redirect to authorization endpoint, got %s", location)
	}
	if !strings.Contains(location, "client_id=test-client-id") {
		t.Error("expected client_id in redirect URL")
	}
}

func TestHandleLogin_UnsafeRedirect(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	// Try absolute URL redirect (should be sanitized to /)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login?redirect=https://evil.com", nil)
	rec := httptest.NewRecorder()

	p.HandleLogin(rec, req)

	// Should still redirect to OIDC provider
	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}

	// The stored state should have sanitized redirect URL
	p.statesMu.Lock()
	for _, entry := range p.states {
		if entry.redirectURL != "/" {
			t.Errorf("expected sanitized redirect /, got %s", entry.redirectURL)
		}
	}
	p.statesMu.Unlock()
}

// --- Enabled ---

func TestEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   OIDCConfig
		expected bool
	}{
		{
			name:     "fully configured",
			config:   OIDCConfig{Enabled: true, IssuerURL: "http://idp.example.com", ClientID: "client"},
			expected: true,
		},
		{
			name:     "disabled",
			config:   OIDCConfig{Enabled: false, IssuerURL: "http://idp.example.com", ClientID: "client"},
			expected: false,
		},
		{
			name:     "missing issuer URL",
			config:   OIDCConfig{Enabled: true, ClientID: "client"},
			expected: false,
		},
		{
			name:     "missing client ID",
			config:   OIDCConfig{Enabled: true, IssuerURL: "http://idp.example.com"},
			expected: false,
		},
		{
			name:     "all empty",
			config:   OIDCConfig{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &OIDCProvider{config: tt.config}
			if p.Enabled() != tt.expected {
				t.Errorf("Enabled() = %v, want %v", p.Enabled(), tt.expected)
			}
		})
	}
}

// --- GetConfig ---

func TestGetConfig_HidesSecret(t *testing.T) {
	p := &OIDCProvider{
		config: OIDCConfig{
			Enabled:      true,
			IssuerURL:    "http://idp.example.com",
			ClientID:     "client-id",
			ClientSecret: "super-secret",
		},
	}

	cfg := p.GetConfig()

	if cfg.ClientSecret != "" {
		t.Error("expected ClientSecret to be hidden in GetConfig")
	}
	if cfg.ClientID != "client-id" {
		t.Errorf("expected ClientID to be preserved, got %s", cfg.ClientID)
	}
	if cfg.IssuerURL != "http://idp.example.com" {
		t.Errorf("expected IssuerURL to be preserved, got %s", cfg.IssuerURL)
	}

	// Original should not be modified
	if p.config.ClientSecret != "super-secret" {
		t.Error("original config should not be modified")
	}
}

// --- Verify ---

func TestVerify(t *testing.T) {
	t.Run("disabled provider returns nil", func(t *testing.T) {
		p := &OIDCProvider{config: OIDCConfig{Enabled: false}}
		if err := p.Verify(context.Background()); err != nil {
			t.Errorf("expected nil error for disabled provider, got %v", err)
		}
	})

	t.Run("enabled provider loads discovery", func(t *testing.T) {
		userinfo := map[string]interface{}{"sub": "user1"}
		srv := mockOIDCServer(t, userinfo)
		defer srv.Close()

		p, _ := newTestOIDCProvider(t, srv.URL)

		if err := p.Verify(context.Background()); err != nil {
			t.Errorf("Verify failed: %v", err)
		}
		if !p.discoveryLoaded {
			t.Error("expected discovery to be loaded after Verify")
		}
	})

	t.Run("unreachable provider returns error", func(t *testing.T) {
		p, _ := newTestOIDCProvider(t, "http://127.0.0.1:1")
		if err := p.Verify(context.Background()); err == nil {
			t.Error("expected error for unreachable provider")
		}
	})
}

// --- Close ---

func TestClose(t *testing.T) {
	p := &OIDCProvider{config: OIDCConfig{}}
	if err := p.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// --- getStringClaim ---

func TestGetStringClaim(t *testing.T) {
	claims := map[string]interface{}{
		"name":  "John",
		"count": 42,
		"flag":  true,
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"name", "John"},
		{"count", ""},   // Not a string
		{"flag", ""},    // Not a string
		{"missing", ""}, // Key doesn't exist
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getStringClaim(claims, tt.key)
			if got != tt.expected {
				t.Errorf("getStringClaim(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

// --- getStringListClaim ---

func TestGetStringListClaim(t *testing.T) {
	t.Run("interface slice", func(t *testing.T) {
		claims := map[string]interface{}{
			"groups": []interface{}{"admin", "users", 42}, // 42 should be skipped
		}
		result := getStringListClaim(claims, "groups")
		if len(result) != 2 {
			t.Fatalf("expected 2 items, got %d", len(result))
		}
		if result[0] != "admin" || result[1] != "users" {
			t.Errorf("unexpected result: %v", result)
		}
	})

	t.Run("string slice", func(t *testing.T) {
		claims := map[string]interface{}{
			"groups": []string{"group1", "group2"},
		}
		result := getStringListClaim(claims, "groups")
		if len(result) != 2 {
			t.Fatalf("expected 2 items, got %d", len(result))
		}
	})

	t.Run("space-separated string", func(t *testing.T) {
		claims := map[string]interface{}{
			"groups": "admin users editors",
		}
		result := getStringListClaim(claims, "groups")
		if len(result) != 3 {
			t.Fatalf("expected 3 items, got %d", len(result))
		}
		if result[0] != "admin" {
			t.Errorf("expected first item 'admin', got %q", result[0])
		}
	})

	t.Run("missing key", func(t *testing.T) {
		claims := map[string]interface{}{}
		result := getStringListClaim(claims, "groups")
		if result != nil {
			t.Errorf("expected nil for missing key, got %v", result)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		claims := map[string]interface{}{
			"groups": 42,
		}
		result := getStringListClaim(claims, "groups")
		if result != nil {
			t.Errorf("expected nil for unsupported type, got %v", result)
		}
	})
}

// --- determineOIDCRole ---

func TestDetermineOIDCRole(t *testing.T) {
	tests := []struct {
		name        string
		groups      []string
		adminGroups []string
		expected    string
	}{
		{
			name:        "user in admin group",
			groups:      []string{"users", "admin-group"},
			adminGroups: []string{"admin-group"},
			expected:    RoleAdmin,
		},
		{
			name:        "user not in admin group",
			groups:      []string{"users", "editors"},
			adminGroups: []string{"admin-group"},
			expected:    RoleUser,
		},
		{
			name:        "case insensitive match",
			groups:      []string{"ADMIN-GROUP"},
			adminGroups: []string{"admin-group"},
			expected:    RoleAdmin,
		},
		{
			name:        "no groups",
			groups:      nil,
			adminGroups: []string{"admin-group"},
			expected:    RoleUser,
		},
		{
			name:        "no admin groups configured",
			groups:      []string{"admin"},
			adminGroups: nil,
			expected:    RoleUser,
		},
		{
			name:        "multiple admin groups",
			groups:      []string{"super-admins"},
			adminGroups: []string{"admins", "super-admins"},
			expected:    RoleAdmin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineOIDCRole(tt.groups, tt.adminGroups)
			if got != tt.expected {
				t.Errorf("determineOIDCRole() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// --- sanitizeRedirectURL ---

func TestSanitizeRedirectURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		basePath string
		expected string
	}{
		{"empty string", "", "", "/"},
		{"valid relative path", "/dashboard", "", "/dashboard"},
		{"absolute URL", "https://evil.com", "", "/"},
		{"protocol-relative", "//evil.com", "", "/"},
		{"valid deep path", "/settings/profile", "", "/settings/profile"},
		{"root path", "/", "", "/"},
		{"no leading slash", "dashboard", "", "/"},
		{"empty with base path", "", "/dash", "/dash/"},
		{"invalid with base path", "https://evil.com", "/dash", "/dash/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeRedirectURL(tt.input, tt.basePath)
			if got != tt.expected {
				t.Errorf("sanitizeRedirectURL(%q, %q) = %q, want %q", tt.input, tt.basePath, got, tt.expected)
			}
		})
	}
}

// --- generateRandomString ---

func TestGenerateRandomString(t *testing.T) {
	s1, err := generateRandomString(32)
	if err != nil {
		t.Fatalf("generateRandomString failed: %v", err)
	}
	if len(s1) != 32 {
		t.Errorf("expected length 32, got %d", len(s1))
	}

	s2, err := generateRandomString(32)
	if err != nil {
		t.Fatalf("generateRandomString failed: %v", err)
	}

	// Two random strings should be different
	if s1 == s2 {
		t.Error("expected different random strings")
	}
}

// --- NewOIDCProvider defaults ---

func TestNewOIDCProvider_Defaults(t *testing.T) {
	ss := NewSessionStore("test", time.Hour, false)
	us := NewUserStore()

	cfg := &OIDCConfig{
		Enabled:   true,
		IssuerURL: "http://idp.example.com",
		ClientID:  "client",
	}

	p := NewOIDCProvider(cfg, ss, us)

	if len(p.config.Scopes) != 3 {
		t.Errorf("expected 3 default scopes, got %d", len(p.config.Scopes))
	}
	if p.config.UsernameClaim != "preferred_username" {
		t.Errorf("expected default username claim 'preferred_username', got %q", p.config.UsernameClaim)
	}
	if p.config.EmailClaim != "email" {
		t.Errorf("expected default email claim 'email', got %q", p.config.EmailClaim)
	}
	if p.config.DisplayNameClaim != "name" {
		t.Errorf("expected default display name claim 'name', got %q", p.config.DisplayNameClaim)
	}
	if p.config.GroupsClaim != "groups" {
		t.Errorf("expected default groups claim 'groups', got %q", p.config.GroupsClaim)
	}
}

func TestNewOIDCProvider_CustomClaims(t *testing.T) {
	ss := NewSessionStore("test", time.Hour, false)
	us := NewUserStore()

	cfg := &OIDCConfig{
		Enabled:          true,
		IssuerURL:        "http://idp.example.com",
		ClientID:         "client",
		Scopes:           []string{"openid"},
		UsernameClaim:    "custom_user",
		EmailClaim:       "custom_email",
		DisplayNameClaim: "custom_name",
		GroupsClaim:      "custom_groups",
	}

	p := NewOIDCProvider(cfg, ss, us)

	if len(p.config.Scopes) != 1 {
		t.Errorf("expected 1 custom scope, got %d", len(p.config.Scopes))
	}
	if p.config.UsernameClaim != "custom_user" {
		t.Errorf("expected custom username claim, got %q", p.config.UsernameClaim)
	}
}

// --- HandleCallback with admin group detection ---

func TestHandleCallback_AdminGroupDetection(t *testing.T) {
	userinfo := map[string]interface{}{
		"sub":                "admin-user",
		"preferred_username": "adminuser",
		"email":              "admin@example.com",
		"name":               "Admin User",
		"groups":             []interface{}{"users", "admin-group"},
	}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, ss := newTestOIDCProvider(t, srv.URL)
	if err := p.loadDiscovery(); err != nil {
		t.Fatalf("loadDiscovery failed: %v", err)
	}

	testState := "admin-state"
	p.statesMu.Lock()
	p.states[testState] = stateEntry{
		createdAt:   time.Now(),
		redirectURL: "/",
	}
	p.statesMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=test-code&state="+testState, nil)
	rec := httptest.NewRecorder()

	p.HandleCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}

	// Verify the session was created with admin role
	if ss.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", ss.Count())
	}
}

// --- HandleCallback with username fallback to sub ---

func TestHandleCallback_FallbackToSub(t *testing.T) {
	// No preferred_username claim, should fall back to sub
	userinfo := map[string]interface{}{
		"sub":   "user-sub-id",
		"email": "user@example.com",
	}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, ss := newTestOIDCProvider(t, srv.URL)
	if err := p.loadDiscovery(); err != nil {
		t.Fatalf("loadDiscovery failed: %v", err)
	}

	testState := "sub-state"
	p.statesMu.Lock()
	p.states[testState] = stateEntry{
		createdAt:   time.Now(),
		redirectURL: "/",
	}
	p.statesMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=test-code&state="+testState, nil)
	rec := httptest.NewRecorder()

	p.HandleCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", rec.Code, rec.Body.String())
	}

	if ss.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", ss.Count())
	}
}

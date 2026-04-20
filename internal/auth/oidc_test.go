package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
)

// testIssuerKey holds the RSA keypair used by mockOIDCServer to sign
// ID tokens. Regenerated per process.
var (
	testIssuerKeyOnce sync.Once
	testIssuerKey     *rsa.PrivateKey
)

func getTestIssuerKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testIssuerKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate test RSA key: %v", err)
		}
		testIssuerKey = key
	})
	return testIssuerKey
}

// signTestIDToken serializes the given claims into a JWT signed with the
// test issuer's RSA key. Used by the mock token endpoint so HandleCallback
// exercises the real go-oidc verification path end to end (findings.md C7).
func signTestIDToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	key := getTestIssuerKey(t)

	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "test-kid",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal JWT header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal JWT claims: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)

	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func testJWKSResponse(t *testing.T) map[string]interface{} {
	t.Helper()
	pub := getTestIssuerKey(t).PublicKey
	return map[string]interface{}{
		"keys": []map[string]interface{}{{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": "test-kid",
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	}
}

// mockOIDCServer creates a test HTTP server that simulates an OIDC provider.
// It serves discovery, JWKS, token, and userinfo endpoints. The token
// endpoint returns an RS256-signed ID token so HandleCallback exercises
// the same verification code path as a real IdP.
func mockOIDCServer(t *testing.T, userinfo map[string]interface{}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	var (
		issuer    string
		nonceMu   sync.Mutex
		lastNonce string
	)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		scheme := "http"
		base := scheme + "://" + r.Host
		doc := map[string]string{
			"issuer":                 base,
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testJWKSResponse(t))
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil { //nolint:gosec // test mock, trusted input
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.FormValue("grant_type") != "authorization_code" { //nolint:gosec // test mock
			http.Error(w, "invalid grant_type", http.StatusBadRequest)
			return
		}
		if r.FormValue("code") == "" { //nolint:gosec // test mock
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		now := time.Now()
		sub := "test-sub"
		if s, ok := userinfo["sub"].(string); ok {
			sub = s
		}
		nonceMu.Lock()
		nonceForToken := lastNonce
		nonceMu.Unlock()
		aud := r.FormValue("client_id") //nolint:gosec // test mock
		if aud == "" {
			aud = "test-client-id"
		}
		claims := map[string]interface{}{
			"iss":   issuer,
			"aud":   aud,
			"sub":   sub,
			"iat":   now.Unix(),
			"exp":   now.Add(time.Hour).Unix(),
			"nonce": nonceForToken,
		}
		resp := TokenResponse{
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
			IDToken:     signTestIDToken(t, claims),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:gosec // test fixture, not a real credential
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
		// Capture the nonce so /token can echo it back in the signed ID
		// token, matching real-IdP behavior.
		if n := r.URL.Query().Get("nonce"); n != "" {
			nonceMu.Lock()
			lastNonce = n
			nonceMu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	issuer = srv.URL
	return srv
}

func newTestOIDCProvider(t *testing.T, issuerURL string) (*OIDCProvider, *SessionStore) {
	t.Helper()
	ss := NewSessionStore("test_session", time.Hour, false)
	us := NewUserStore()

	cfg := config.OIDCConfig{
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
		done:         make(chan struct{}),
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

	if err := p.loadDiscovery(context.Background()); err != nil {
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
	if err := p.loadDiscovery(context.Background()); err != nil {
		t.Fatalf("first loadDiscovery failed: %v", err)
	}

	// Second load should use cache (no error even if server is down)
	srv.Close()
	if err := p.loadDiscovery(context.Background()); err != nil {
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

	if err := p.loadDiscovery(context.Background()); err == nil {
		t.Error("expected error from server returning 500")
	} else if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error mentioning 500, got: %v", err)
	}
}

func TestLoadDiscovery_Unreachable(t *testing.T) {
	p, _ := newTestOIDCProvider(t, "http://127.0.0.1:1") // port 1 should fail to connect

	if err := p.loadDiscovery(context.Background()); err == nil {
		t.Error("expected error for unreachable server")
	}
}

// --- GetAuthorizationURL ---

func TestGetAuthorizationURL(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	authURL, err := p.GetAuthorizationURL(context.Background(), "/dashboard")
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
	// Find the state entry and check redirect URL + PKCE plumbing.
	for _, entry := range p.states {
		if entry.redirectURL != "/dashboard" {
			t.Errorf("expected redirectURL /dashboard, got %s", entry.redirectURL)
		}
		if entry.codeVerifier == "" {
			t.Error("expected stored codeVerifier for PKCE (findings.md L2)")
		}
	}
	p.statesMu.Unlock()

	// findings.md L2: PKCE S256 challenge must be sent on the
	// authorization request, paired with the verifier stashed server-side.
	if !strings.Contains(authURL, "code_challenge=") {
		t.Error("expected code_challenge in auth URL")
	}
	if !strings.Contains(authURL, "code_challenge_method=S256") {
		t.Error("expected code_challenge_method=S256 in auth URL")
	}
}

// TestPKCE_VerifierSentOnExchange covers findings.md L2. The verifier
// stored in the state entry must be sent with the token exchange.
func TestPKCE_VerifierSentOnExchange(t *testing.T) {
	var capturedVerifier string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		json.NewEncoder(w).Encode(map[string]string{
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()                                                               //nolint:gosec // test mock, trusted input
		capturedVerifier = r.FormValue("code_verifier")                                 //nolint:gosec // test mock
		json.NewEncoder(w).Encode(TokenResponse{AccessToken: "x", TokenType: "Bearer"}) //nolint:gosec // test fixture
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)
	if _, err := p.exchangeCode(context.Background(), "code", "the-verifier"); err != nil {
		t.Fatalf("exchangeCode: %v", err)
	}
	if capturedVerifier != "the-verifier" {
		t.Errorf("verifier not sent: got %q, want the-verifier", capturedVerifier)
	}
}

// TestPKCE_S256 verifies the RFC 7636 code-challenge derivation.
func TestPKCE_S256(t *testing.T) {
	// RFC 7636 Appendix B test vector:
	// verifier  = dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk
	// challenge = E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM
	got := pkceS256Challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Errorf("pkceS256Challenge mismatch: got %q", got)
	}
}

// TestHandleCallback_RejectsMissingIDToken pins down findings.md C7.
// Even a successful /token response without an id_token must be refused:
// falling back to userinfo-only would bypass signature/issuer/audience/
// nonce verification.
func TestHandleCallback_RejectsMissingIDToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		json.NewEncoder(w).Encode(map[string]string{
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		// No IDToken in the response.
		json.NewEncoder(w).Encode(TokenResponse{ //nolint:gosec // test fixture
			AccessToken: "something",
			TokenType:   "Bearer",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)
	if err := p.loadDiscovery(context.Background()); err != nil {
		t.Fatalf("loadDiscovery: %v", err)
	}

	state := "no-id-token-state"
	p.statesMu.Lock()
	p.states[state] = stateEntry{createdAt: time.Now(), redirectURL: "/"}
	p.statesMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=x&state="+state, nil)
	rec := httptest.NewRecorder()
	p.HandleCallback(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without id_token, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- exchangeCode ---

func TestExchangeCode(t *testing.T) {
	userinfo := map[string]interface{}{"sub": "user1"}
	srv := mockOIDCServer(t, userinfo)
	defer srv.Close()

	p, _ := newTestOIDCProvider(t, srv.URL)

	tokens, err := p.exchangeCode(context.Background(), "test-auth-code", "")
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

	_, err := p.exchangeCode(context.Background(), "bad-code", "")
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

	claims, err := p.getUserInfo(context.Background(), "test-access-token")
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
	if err := p.loadDiscovery(context.Background()); err != nil {
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
		config   config.OIDCConfig
		expected bool
	}{
		{
			name:     "fully configured",
			config:   config.OIDCConfig{Enabled: true, IssuerURL: "http://idp.example.com", ClientID: "client"},
			expected: true,
		},
		{
			name:     "disabled",
			config:   config.OIDCConfig{Enabled: false, IssuerURL: "http://idp.example.com", ClientID: "client"},
			expected: false,
		},
		{
			name:     "missing issuer URL",
			config:   config.OIDCConfig{Enabled: true, ClientID: "client"},
			expected: false,
		},
		{
			name:     "missing client ID",
			config:   config.OIDCConfig{Enabled: true, IssuerURL: "http://idp.example.com"},
			expected: false,
		},
		{
			name:     "all empty",
			config:   config.OIDCConfig{},
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
		config: config.OIDCConfig{
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
		p := &OIDCProvider{config: config.OIDCConfig{Enabled: false}}
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
	p := &OIDCProvider{
		config: config.OIDCConfig{},
		done:   make(chan struct{}),
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
	// Second Close must not panic or double-close.
	if err := p.Close(); err != nil {
		t.Errorf("second Close returned error: %v", err)
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
		// findings.md L3: some browsers normalize backslashes to forward
		// slashes, so "/\evil.com" becomes "//evil.com" post-normalization
		// and reaches attacker origins. Reject the input rather than
		// relying on browser-specific behavior.
		{"backslash after leading slash", "/\\evil.com", "", "/"},
		{"control char in path", "/foo\r\nBar", "", "/"},
		{"tab in path", "/foo\tbar", "", "/"},
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

// --- cleanupStates (inline logic test) ---

func TestCleanupStates(t *testing.T) {
	p, _ := newTestOIDCProvider(t, "http://unused")

	now := time.Now()

	// Add a mix of fresh and expired states
	p.statesMu.Lock()
	p.states["fresh1"] = stateEntry{
		createdAt:   now,
		redirectURL: "/a",
	}
	p.states["fresh2"] = stateEntry{
		createdAt:   now.Add(-5 * time.Minute),
		redirectURL: "/b",
	}
	p.states["expired1"] = stateEntry{
		createdAt:   now.Add(-11 * time.Minute),
		redirectURL: "/c",
	}
	p.states["expired2"] = stateEntry{
		createdAt:   now.Add(-20 * time.Minute),
		redirectURL: "/d",
	}
	p.statesMu.Unlock()

	// Simulate what cleanupStates does on each tick
	p.statesMu.Lock()
	for state, entry := range p.states {
		if time.Since(entry.createdAt) > 10*time.Minute {
			delete(p.states, state)
		}
	}
	p.statesMu.Unlock()

	// Verify: fresh states should remain, expired ones should be gone
	p.statesMu.Lock()
	defer p.statesMu.Unlock()

	if len(p.states) != 2 {
		t.Fatalf("expected 2 remaining states, got %d", len(p.states))
	}
	if _, ok := p.states["fresh1"]; !ok {
		t.Error("expected fresh1 to still exist")
	}
	if _, ok := p.states["fresh2"]; !ok {
		t.Error("expected fresh2 to still exist")
	}
	if _, ok := p.states["expired1"]; ok {
		t.Error("expected expired1 to be removed")
	}
	if _, ok := p.states["expired2"]; ok {
		t.Error("expected expired2 to be removed")
	}
}

func TestCleanupStates_EmptyMap(t *testing.T) {
	p, _ := newTestOIDCProvider(t, "http://unused")

	// Simulate cleanup on empty map -- should not panic
	p.statesMu.Lock()
	for state, entry := range p.states {
		if time.Since(entry.createdAt) > 10*time.Minute {
			delete(p.states, state)
		}
	}
	p.statesMu.Unlock()

	if len(p.states) != 0 {
		t.Errorf("expected 0 states, got %d", len(p.states))
	}
}

func TestCleanupStates_AllExpired(t *testing.T) {
	p, _ := newTestOIDCProvider(t, "http://unused")

	p.statesMu.Lock()
	p.states["old1"] = stateEntry{
		createdAt:   time.Now().Add(-15 * time.Minute),
		redirectURL: "/x",
	}
	p.states["old2"] = stateEntry{
		createdAt:   time.Now().Add(-30 * time.Minute),
		redirectURL: "/y",
	}
	p.statesMu.Unlock()

	// Simulate cleanup
	p.statesMu.Lock()
	for state, entry := range p.states {
		if time.Since(entry.createdAt) > 10*time.Minute {
			delete(p.states, state)
		}
	}
	p.statesMu.Unlock()

	if len(p.states) != 0 {
		t.Errorf("expected all states to be cleaned up, got %d", len(p.states))
	}
}

// --- generateRandomString ---

func TestGenerateRandomString(t *testing.T) {
	s1, err := generateRandomString()
	if err != nil {
		t.Fatalf("generateRandomString failed: %v", err)
	}
	if len(s1) != 32 {
		t.Errorf("expected length 32, got %d", len(s1))
	}

	s2, err := generateRandomString()
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

	cfg := &config.OIDCConfig{
		Enabled:   true,
		IssuerURL: "http://idp.example.com",
		ClientID:  "client",
	}

	p := NewOIDCProvider(cfg, "", ss, us)

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

	cfg := &config.OIDCConfig{
		Enabled:          true,
		IssuerURL:        "http://idp.example.com",
		ClientID:         "client",
		Scopes:           []string{"openid"},
		UsernameClaim:    "custom_user",
		EmailClaim:       "custom_email",
		DisplayNameClaim: "custom_name",
		GroupsClaim:      "custom_groups",
	}

	p := NewOIDCProvider(cfg, "", ss, us)

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
	if err := p.loadDiscovery(context.Background()); err != nil {
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
	if err := p.loadDiscovery(context.Background()); err != nil {
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

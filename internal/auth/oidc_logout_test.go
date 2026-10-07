package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
)

// mockIDP serves a discovery document (with optional extra fields) and
// the test JWKS. The issuer is the server URL.
func mockIDP(t *testing.T, extra map[string]interface{}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		doc := map[string]interface{}{
			"issuer": srv.URL, "authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint": srv.URL + "/token", "userinfo_endpoint": srv.URL + "/userinfo",
			"jwks_uri": srv.URL + "/jwks",
		}
		for k, v := range extra {
			doc[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testJWKSResponse(t))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestProvider(t *testing.T, issuer string, mutate func(*config.OIDCConfig)) *OIDCProvider {
	t.Helper()
	cfg := config.OIDCConfig{Enabled: true, IssuerURL: issuer, ClientID: "muximux",
		RedirectURL: "https://dash.example.com/mx/api/auth/oidc/callback"}
	if mutate != nil {
		mutate(&cfg)
	}
	p := NewOIDCProvider(&cfg, "/mx", NewSessionStore("muximux_session", time.Hour, false), nil)
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func oidcSession(idToken string) *Session {
	return &Session{ID: "s1", Data: map[string]interface{}{"oidc_id_token": idToken, "oidc_sub": "u1"}}
}

func TestDiscover_ReadsLogoutCapabilities(t *testing.T) {
	idp := mockIDP(t, map[string]interface{}{"end_session_endpoint": "https://idp/logout", "backchannel_logout_supported": true})
	p := newTestProvider(t, idp.URL, nil)
	if err := p.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if es, bc := p.Capabilities(); !es || !bc {
		t.Errorf("capabilities = %v, %v", es, bc)
	}
}

func TestEndSessionURL(t *testing.T) {
	idp := mockIDP(t, map[string]interface{}{"end_session_endpoint": "https://idp.example.com/logout"})

	t.Run("off", func(t *testing.T) {
		p := newTestProvider(t, idp.URL, nil)
		_ = p.Discover(context.Background())
		if _, ok := p.EndSessionURL(oidcSession("tok")); ok {
			t.Error("provider_logout off must not redirect")
		}
	})

	t.Run("discovered endpoint with default return", func(t *testing.T) {
		p := newTestProvider(t, idp.URL, func(c *config.OIDCConfig) { c.ProviderLogout = true })
		_ = p.Discover(context.Background())
		raw, ok := p.EndSessionURL(oidcSession("tok"))
		if !ok {
			t.Fatal("no redirect")
		}
		u, _ := url.Parse(raw)
		q := u.Query()
		if u.Host != "idp.example.com" || q.Get("id_token_hint") != "tok" || q.Get("client_id") != "muximux" ||
			q.Get("post_logout_redirect_uri") != "https://dash.example.com/mx/login?logged_out=1" || q.Get("state") == "" {
			t.Errorf("redirect = %s", raw)
		}
	})

	t.Run("override and custom return", func(t *testing.T) {
		p := newTestProvider(t, idp.URL, func(c *config.OIDCConfig) {
			c.ProviderLogout = true
			c.LogoutURL = "https://idp.example.com/custom/logout?x=1"
			c.PostLogoutRedirectURL = "https://dash.example.com/bye"
		})
		_ = p.Discover(context.Background())
		raw, _ := p.EndSessionURL(oidcSession("tok"))
		u, _ := url.Parse(raw)
		if u.Path != "/custom/logout" || u.Query().Get("x") != "1" || u.Query().Get("post_logout_redirect_uri") != "https://dash.example.com/bye" {
			t.Errorf("redirect = %s", raw)
		}
	})

	t.Run("local session", func(t *testing.T) {
		p := newTestProvider(t, idp.URL, func(c *config.OIDCConfig) { c.ProviderLogout = true })
		_ = p.Discover(context.Background())
		if _, ok := p.EndSessionURL(&Session{ID: "s", Data: map[string]interface{}{}}); ok {
			t.Error("a local session must not get a provider redirect")
		}
	})

	t.Run("no endpoint anywhere", func(t *testing.T) {
		bare := mockIDP(t, nil)
		p := newTestProvider(t, bare.URL, func(c *config.OIDCConfig) { c.ProviderLogout = true })
		_ = p.Discover(context.Background())
		if _, ok := p.EndSessionURL(oidcSession("tok-2")); ok {
			t.Error("no endpoint must mean no redirect")
		}
	})
}

const backchannelEvent = "http://schemas.openid.net/event/backchannel-logout"

func logoutClaims(iss string) map[string]interface{} {
	return map[string]interface{}{
		"iss": iss, "aud": "muximux", "iat": time.Now().Unix(), "jti": randomJTI(),
		"sub": "u1", "sid": "sid-1",
		"events": map[string]interface{}{backchannelEvent: map[string]interface{}{}},
	}
}

var jtiCounter int64

func randomJTI() string { return "jti-" + strconv.FormatInt(atomic.AddInt64(&jtiCounter, 1), 10) }

func TestVerifyLogoutToken(t *testing.T) {
	idp := mockIDP(t, nil)
	p := newTestProvider(t, idp.URL, nil)
	ctx := context.Background()

	sub, sid, err := p.VerifyLogoutToken(ctx, signTestIDToken(t, logoutClaims(idp.URL)))
	if err != nil || sub != "u1" || sid != "sid-1" {
		t.Fatalf("valid token: %q %q %v", sub, sid, err)
	}

	bad := map[string]func(c map[string]interface{}){
		"wrong issuer":   func(c map[string]interface{}) { c["iss"] = "https://other" },
		"wrong audience": func(c map[string]interface{}) { c["aud"] = "someone-else" },
		"missing event":  func(c map[string]interface{}) { c["events"] = map[string]interface{}{} },
		"nonce present":  func(c map[string]interface{}) { c["nonce"] = "n" },
		"nonce null":     func(c map[string]interface{}) { c["nonce"] = nil },
		"no sub or sid":  func(c map[string]interface{}) { delete(c, "sub"); delete(c, "sid") },
		"no jti":         func(c map[string]interface{}) { delete(c, "jti") },
		"no iat":         func(c map[string]interface{}) { delete(c, "iat") },
		"stale iat":      func(c map[string]interface{}) { c["iat"] = time.Now().Add(-11 * time.Minute).Unix() },
		"future iat":     func(c map[string]interface{}) { c["iat"] = time.Now().Add(3 * time.Minute).Unix() },
	}
	for name, mutate := range bad {
		c := logoutClaims(idp.URL)
		mutate(c)
		if _, _, err := p.VerifyLogoutToken(ctx, signTestIDToken(t, c)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	if _, _, err := p.VerifyLogoutToken(ctx, "not-a-jwt"); err == nil {
		t.Error("garbage accepted")
	}

	replay := signTestIDToken(t, logoutClaims(idp.URL))
	if _, _, err := p.VerifyLogoutToken(ctx, replay); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.VerifyLogoutToken(ctx, replay); err == nil {
		t.Error("replayed jti accepted")
	}
}

func TestRememberJTI_Bounded(t *testing.T) {
	p := newTestProvider(t, "https://unused.example.com", nil)
	now := time.Now()
	for i := 0; i < maxSeenJTI+50; i++ {
		if !p.rememberJTI("j"+strconv.Itoa(i), now.Add(time.Duration(i)*time.Millisecond)) {
			t.Fatalf("jti %d rejected", i)
		}
	}
	if len(p.seenJTI) > maxSeenJTI {
		t.Errorf("cache size %d exceeds cap", len(p.seenJTI))
	}
	// Old entries are pruned once past the window.
	if !p.rememberJTI("late", now.Add(time.Hour)) || len(p.seenJTI) != 1 {
		t.Errorf("expired entries not pruned: %d", len(p.seenJTI))
	}
}

func TestVerifyLogoutToken_ProviderAndClaimsErrors(t *testing.T) {
	dead := newTestProvider(t, "http://127.0.0.1:1", nil)
	if _, _, err := dead.VerifyLogoutToken(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "provider:") {
		t.Errorf("unreachable issuer: %v", err)
	}

	idp := mockIDP(t, nil)
	p := newTestProvider(t, idp.URL, nil)
	c := logoutClaims(idp.URL)
	c["events"] = "not-an-object"
	if _, _, err := p.VerifyLogoutToken(context.Background(), signTestIDToken(t, c)); err == nil || !strings.Contains(err.Error(), "claims:") {
		t.Errorf("malformed events: %v", err)
	}
}

func TestEndSessions(t *testing.T) {
	idp := mockIDP(t, nil)
	p := newTestProvider(t, idp.URL, nil)
	mk := func(sub, sid string) *Session {
		s, _ := p.sessionStore.Create(sub, sub, "user")
		s.Data["oidc_sub"] = sub
		if sid != "" {
			s.Data["oidc_sid"] = sid
		}
		return s
	}
	a1, a2, b := mk("alice", "sid-a1"), mk("alice", "sid-a2"), mk("bob", "sid-b")
	if n := p.EndSessions("alice", "sid-a1"); n != 1 || p.sessionStore.Get(a1.ID) != nil || p.sessionStore.Get(a2.ID) == nil {
		t.Errorf("by sid: n=%d", n)
	}
	if n := p.EndSessions("alice", ""); n != 1 || p.sessionStore.Get(a2.ID) != nil || p.sessionStore.Get(b.ID) == nil {
		t.Errorf("by sub: n=%d", n)
	}
}

func TestEndSessions_EmptyIdentifiersDeleteNothing(t *testing.T) {
	p := newTestProvider(t, "https://unused.example.com", nil)
	local, _ := p.sessionStore.Create("local", "local", "user")
	oidc, _ := p.sessionStore.Create("alice", "alice", "user")
	oidc.Data["oidc_sub"] = "alice"
	if n := p.EndSessions("", ""); n != 0 || p.sessionStore.Get(local.ID) == nil || p.sessionStore.Get(oidc.ID) == nil {
		t.Errorf("empty identifiers deleted sessions: n=%d", n)
	}
}

func TestConfig_OmitsClientSecret(t *testing.T) {
	p := newTestProvider(t, "https://unused.example.com", func(c *config.OIDCConfig) {
		c.ClientSecret = "s3cret"
		c.DisableLocalLogin = true
	})
	got := p.Config()
	if got.ClientSecret != "" {
		t.Error("Config() exposed the client secret")
	}
	if !got.DisableLocalLogin || got.ClientID != "muximux" || got.UsernameClaim == "" {
		t.Errorf("Config() lost settings or defaults: %+v", got)
	}
}

func TestPostLogoutRedirect_Fallbacks(t *testing.T) {
	cases := []struct {
		name, redirectURL, want string
	}{
		{"host-less redirect_url", "/api/auth/oidc/callback", "/mx/login?logged_out=1"},
		{"unparsable redirect_url", "http://bad host/%zz", "/mx/login?logged_out=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestProvider(t, "https://unused.example.com", func(c *config.OIDCConfig) { c.RedirectURL = tc.redirectURL })
			if got := p.postLogoutRedirect(); got != tc.want {
				t.Errorf("postLogoutRedirect() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEndSessions_UnknownSidEndsNothing(t *testing.T) {
	p := newTestProvider(t, "https://unused.example.com", nil)
	s, _ := p.sessionStore.CreateWithData("alice", "alice", "user", map[string]interface{}{"oidc_sub": "alice"})
	// No session carries this sid; matching by sid must not fall back to sub.
	if n := p.EndSessions("alice", "sid-unknown"); n != 0 || p.sessionStore.Get(s.ID) == nil {
		t.Errorf("unknown sid ended sessions: n=%d", n)
	}
}

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
	p := NewOIDCProvider(&cfg, "/mx", NewSessionStore("muximux_session", 0, false), nil)
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

package handlers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// loadConfigWithEnv writes yaml to a temp config file, sets the given
// environment variables and loads it.
func loadConfigWithEnv(t *testing.T, yaml string, env map[string]string) (*config.Config, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, path
}

func newOIDCSettingsHandler(cfg *config.Config, path string) *AuthHandler {
	return NewAuthHandler(auth.NewSessionStore("muximux_session", time.Hour, false), auth.NewUserStore(), cfg, path, nil, &sync.RWMutex{})
}

func TestGetOIDCSettings_NoSecretAndEnvFields(t *testing.T) {
	cfg, path := loadConfigWithEnv(t, `auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://idp.example.com/realms/home
    client_id: muximux
    client_secret: ${OIDC_CLIENT_SECRET}
    redirect_url: https://dash.example.com/api/auth/oidc/callback
`, map[string]string{"OIDC_CLIENT_SECRET": "s3cret"})
	h := newOIDCSettingsHandler(cfg, path)
	rec := httptest.NewRecorder()
	h.GetOIDCSettings(rec, httptest.NewRequest(http.MethodGet, "/api/auth/settings/oidc", nil))
	body := rec.Body.String()
	if rec.Code != 200 || strings.Contains(body, "s3cret") {
		t.Fatalf("code=%d body=%s", rec.Code, body)
	}
	var got oidcSettingsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.ClientSecretSet || got.EnvFields["client_secret"] != "OIDC_CLIENT_SECRET" || got.ClientID != "muximux" {
		t.Errorf("got %+v", got)
	}
	if !strings.HasSuffix(got.BackchannelURL, "/api/auth/oidc/backchannel-logout") {
		t.Errorf("backchannel url = %q", got.BackchannelURL)
	}
	if !strings.HasSuffix(got.DefaultCallbackURL, "/api/auth/oidc/callback") {
		t.Errorf("callback url = %q", got.DefaultCallbackURL)
	}
	if got.CurrentSessionIsOIDC {
		t.Error("no session cookie, should not be an OIDC session")
	}
}

func TestGetOIDCSettings_MethodAndNilConfig(t *testing.T) {
	h := newOIDCSettingsHandler(nil, "")
	rec := httptest.NewRecorder()
	h.GetOIDCSettings(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST code = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.GetOIDCSettings(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil config code = %d", rec.Code)
	}
}

func TestGetOIDCSettings_OIDCSession(t *testing.T) {
	cfg, path := loadConfigWithEnv(t, "auth:\n  method: none\n", nil)
	h := newOIDCSettingsHandler(cfg, path)
	sess, err := h.sessionStore.CreateWithData("u1", "alice", "admin", map[string]interface{}{"oidc_id_token": "x"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	rec := httptest.NewRecorder()
	h.GetOIDCSettings(rec, req)
	var got oidcSettingsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.CurrentSessionIsOIDC {
		t.Errorf("expected OIDC session: %s", rec.Body.String())
	}
}

func TestRequestIsOIDCSession(t *testing.T) {
	store := auth.NewSessionStore("muximux_session", time.Hour, false)
	withReq := func(sess *auth.Session) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if sess != nil {
			req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
		}
		return req
	}
	if requestIsOIDCSession(store, withReq(nil)) {
		t.Error("no cookie: want false")
	}
	local, _ := store.Create("u1", "alice", "admin")
	if requestIsOIDCSession(store, withReq(local)) {
		t.Error("local session: want false")
	}
	oidc, _ := store.CreateWithData("u2", "bob", "admin", map[string]interface{}{"oidc_id_token": "x"})
	if !requestIsOIDCSession(store, withReq(oidc)) {
		t.Error("oidc session: want true")
	}
}

func TestPublicOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://dash.example.com/x", nil)
	if got := publicOrigin(req); got != "http://dash.example.com" {
		t.Errorf("plain = %q", got)
	}
	req.TLS = &tls.ConnectionState{}
	if got := publicOrigin(req); got != "https://dash.example.com" {
		t.Errorf("tls = %q", got)
	}
	req = httptest.NewRequest(http.MethodGet, "http://dash.example.com/x", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := publicOrigin(req); got != "http://dash.example.com" {
		t.Errorf("raw header must be ignored, got %q", got)
	}
	req = req.WithContext(context.WithValue(req.Context(), auth.ContextKeyClientScheme, "https"))
	if got := publicOrigin(req); got != "https://dash.example.com" {
		t.Errorf("context scheme = %q", got)
	}
}

func postOIDCTest(h *AuthHandler, body string) (*httptest.ResponseRecorder, oidcTestResponse) {
	rec := httptest.NewRecorder()
	h.TestOIDCProvider(rec, httptest.NewRequest(http.MethodPost, "/api/auth/settings/oidc/test", strings.NewReader(body)))
	var r oidcTestResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	return rec, r
}

func TestTestOIDCProvider(t *testing.T) {
	idp := mockOIDCDiscoveryServer(t)
	h := newOIDCSettingsHandler(nil, "")
	post := func(issuer string) oidcTestResponse {
		_, r := postOIDCTest(h, `{"issuer_url":"`+issuer+`"}`)
		return r
	}
	if r := post(idp.URL); !r.Reachable || !r.Authorization || !r.Token || !r.Userinfo || !r.JWKS || !r.EndSession {
		t.Errorf("reachable provider: %+v", r)
	}
	if r := post(idp.URL + "/"); !r.Reachable {
		t.Errorf("trailing slash: %+v", r)
	}
	if r := post("http://127.0.0.1:1"); r.Reachable || r.Error == "" {
		t.Errorf("unreachable: %+v", r)
	}
	if r := post("ftp://idp.example.com"); r.Reachable || r.Error == "" {
		t.Errorf("bad scheme: %+v", r)
	}
}

func TestTestOIDCProvider_RequestErrors(t *testing.T) {
	h := newOIDCSettingsHandler(nil, "")
	if rec, _ := postOIDCTest(h, `{not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid body code = %d", rec.Code)
	}
	rec := httptest.NewRecorder()
	h.TestOIDCProvider(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET code = %d", rec.Code)
	}
}

func TestTestOIDCProvider_BadDiscovery(t *testing.T) {
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>`))
	}))
	defer badJSON.Close()
	h := newOIDCSettingsHandler(nil, "")
	for name, issuer := range map[string]string{"non-200": notFound.URL, "invalid json": badJSON.URL} {
		_, r := postOIDCTest(h, `{"issuer_url":"`+issuer+`"}`)
		if r.Reachable || r.Error == "" {
			t.Errorf("%s: %+v", name, r)
		}
	}
}

func TestTestOIDCProvider_LimitsAndRedirects(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"x","pad":"` + strings.Repeat("a", 2<<20) + `"}`))
	}))
	defer big.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer other.Close()
	hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/.well-known/openid-configuration", http.StatusFound)
	}))
	defer hop.Close()
	h := newOIDCSettingsHandler(nil, "")
	for name, issuer := range map[string]string{"oversize": big.URL, "cross-host redirect": hop.URL} {
		_, r := postOIDCTest(h, `{"issuer_url":"`+issuer+`"}`)
		if r.Reachable {
			t.Errorf("%s accepted: %+v", name, r)
		}
	}
}

func TestTestOIDCProvider_SameHostRedirect(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			http.Redirect(w, r, srv.URL+"/real", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`{"issuer":"x"}`))
	}))
	defer srv.Close()
	_, r := postOIDCTest(newOIDCSettingsHandler(nil, ""), `{"issuer_url":"`+srv.URL+`"}`)
	if !r.Reachable {
		t.Errorf("same-host redirect: %+v", r)
	}
}

func TestApplyOIDCSettings(t *testing.T) {
	s := func(v string) *string { return &v }
	b := func(v bool) *bool { return &v }
	cur := config.OIDCConfig{IssuerURL: "https://a", ClientID: "c", ClientSecret: "keep", Scopes: []string{"openid"}}
	got := applyOIDCSettings(&cur, &oidcSettingsRequest{
		IssuerURL: s("https://b"), ClientSecret: s(""), Scopes: []string{"openid", "email"}, ProviderLogout: b(true),
	}, map[string]bool{})
	if got.IssuerURL != "https://b" || got.ClientSecret != "keep" || len(got.Scopes) != 2 || !got.ProviderLogout || !got.Enabled || got.ClientID != "c" {
		t.Errorf("got %+v", got)
	}
	locked := applyOIDCSettings(&cur, &oidcSettingsRequest{ClientSecret: s("typed"), IssuerURL: s("https://x")},
		map[string]bool{"client_secret": true, "issuer_url": true})
	if locked.ClientSecret != "keep" || locked.IssuerURL != "https://a" {
		t.Errorf("env-locked field overwritten: %+v", locked)
	}
	replaced := applyOIDCSettings(&cur, &oidcSettingsRequest{ClientSecret: s("new")}, map[string]bool{})
	if replaced.ClientSecret != "new" {
		t.Error("typed secret not applied")
	}
	all := applyOIDCSettings(&cur, &oidcSettingsRequest{
		ClientID: s(" cid "), RedirectURL: s("https://d/cb"), UsernameClaim: s("sub"), EmailClaim: s("mail"),
		GroupsClaim: s("roles"), DisplayNameClaim: s("nick"), AdminGroups: []string{"ops"},
		PostLogoutRedirectURL: s("https://d/bye"), LogoutURL: s("https://idp/logout"),
		AutoRedirect: b(true), DisableLocalLogin: b(true),
	}, nil)
	want := config.OIDCConfig{
		Enabled: true, IssuerURL: "https://a", ClientID: "cid", ClientSecret: "keep", RedirectURL: "https://d/cb",
		Scopes: []string{"openid"}, UsernameClaim: "sub", EmailClaim: "mail", GroupsClaim: "roles", DisplayNameClaim: "nick",
		AdminGroups: []string{"ops"}, PostLogoutRedirectURL: "https://d/bye", LogoutURL: "https://idp/logout",
		AutoRedirect: true, DisableLocalLogin: true,
	}
	if !reflect.DeepEqual(all, want) {
		t.Errorf("got  %+v\nwant %+v", all, want)
	}
}

// newOIDCSaveHandler wires a handler the way the server does: an admin
// in the user store, a middleware enforcing cfg's method and a provider
// when oidc.enabled is set (built by the test, not discovered).
func newOIDCSaveHandler(t *testing.T, cfg *config.Config, path string) (*AuthHandler, *auth.Middleware) {
	t.Helper()
	store := auth.NewSessionStore("muximux_session", time.Hour, false)
	users := auth.NewUserStore()
	users.LoadFromConfig([]auth.UserConfig{{Username: "admin", PasswordHash: "x", Role: "admin"}})
	mw := auth.NewMiddleware(&auth.AuthConfig{Method: auth.AuthMethod(cfg.Auth.Method)}, store, users)
	h := NewAuthHandler(store, users, cfg, path, mw, &sync.RWMutex{})
	t.Cleanup(func() { _ = h.CloseOIDC() })
	return h, mw
}

// putAuthMethod sends PUT /api/auth/method, optionally with a session cookie.
func putAuthMethod(h *AuthHandler, body interface{}, sess *auth.Session) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(raw))
	if sess != nil {
		req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	}
	rec := httptest.NewRecorder()
	h.UpdateAuthMethod(rec, req)
	return rec
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestUpdateAuthMethod_OIDCMatchesStartup(t *testing.T) {
	idp := mockOIDCDiscoveryServer(t)
	cfg, path := loadConfigWithEnv(t, "auth:\n  method: none\n", nil)
	h, mw := newOIDCSaveHandler(t, cfg, path)

	rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]interface{}{
		"issuer_url": idp.URL, "client_id": "muximux", "client_secret": "s3cret", "auto_redirect": true,
	}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "s3cret") {
		t.Error("response echoes the client secret")
	}

	// The file is what a restart would load.
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	o := reloaded.Auth.OIDC
	if reloaded.Auth.Method != "oidc" || !o.Enabled || o.IssuerURL != idp.URL || o.ClientSecret != "s3cret" || !o.AutoRedirect {
		t.Errorf("file config = %+v", reloaded.Auth)
	}
	if o.RedirectURL != "http://example.com/api/auth/oidc/callback" {
		t.Errorf("default redirect url = %q", o.RedirectURL)
	}

	// The live state is what startup would build from that file.
	if mw.Method() != auth.AuthMethodOIDC {
		t.Errorf("middleware method = %q", mw.Method())
	}
	p := h.provider()
	if p == nil || !p.Enabled() || p.Config().ClientID != "muximux" {
		t.Fatalf("provider = %+v", p)
	}
	status := httptest.NewRecorder()
	h.AuthStatus(status, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
	var st map[string]interface{}
	_ = json.Unmarshal(status.Body.Bytes(), &st)
	if st["oidc_enabled"] != true || st["auth_method"] != "oidc" || st["oidc_auto_redirect"] != true {
		t.Errorf("auth status = %v", st)
	}

	// An unauthenticated request is refused, an OIDC session gets in.
	protected := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	anon := httptest.NewRecorder()
	protected.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/api/apps", nil))
	if anon.Code != http.StatusUnauthorized {
		t.Errorf("anonymous request code = %d", anon.Code)
	}
	sess, _ := h.sessionStore.CreateWithData("sso-1", "alice", "admin", map[string]interface{}{"oidc_id_token": "x"})
	withSess := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	withSess.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	ok := httptest.NewRecorder()
	protected.ServeHTTP(ok, withSess)
	if ok.Code != http.StatusNoContent {
		t.Errorf("OIDC session request code = %d", ok.Code)
	}
}

func TestUpdateAuthMethod_OIDCRejected(t *testing.T) {
	idp := mockOIDCDiscoveryServer(t)
	cases := []struct {
		name    string
		oidc    interface{}
		code    int
		message string
	}{
		{"no oidc object", nil, http.StatusBadRequest, "oidc settings are required"},
		{"missing issuer", map[string]string{"client_id": "c"}, http.StatusBadRequest, "issuer URL and client ID are required"},
		{"missing client id", map[string]string{"issuer_url": idp.URL}, http.StatusBadRequest, "issuer URL and client ID are required"},
		{"invalid logout url", map[string]string{"issuer_url": idp.URL, "client_id": "c", "logout_url": "ftp://idp/logout"},
			http.StatusBadRequest, "auth.oidc.logout_url must be an absolute http(s) URL"},
		{"unreachable issuer", map[string]string{"issuer_url": "http://127.0.0.1:1", "client_id": "c", "client_secret": "s3cret"},
			http.StatusBadRequest, "OIDC discovery failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, path := loadConfigWithEnv(t, "auth:\n  method: none\n", nil)
			h, mw := newOIDCSaveHandler(t, cfg, path)
			before := readFile(t, path)
			body := map[string]interface{}{"method": "oidc"}
			if tc.oidc != nil {
				body["oidc"] = tc.oidc
			}
			rec := putAuthMethod(h, body, nil)
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.message) {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "s3cret") {
				t.Error("error echoes the client secret")
			}
			if readFile(t, path) != before || cfg.Auth.Method != "none" || cfg.Auth.OIDC.Enabled {
				t.Error("rejected save changed the config")
			}
			if h.provider() != nil || mw.Method() != auth.AuthMethodNone {
				t.Error("rejected save changed the live state")
			}
		})
	}
}

func TestUpdateAuthMethod_OIDCDiscoveryFailureKeepsProvider(t *testing.T) {
	idp := mockOIDCDiscoveryServer(t)
	cfg, path := loadConfigWithEnv(t, fmt.Sprintf("auth:\n  method: oidc\n  oidc:\n    enabled: true\n    issuer_url: %s\n    client_id: c\n", idp.URL), nil)
	h, _ := newOIDCSaveHandler(t, cfg, path)
	if err := h.ReplaceOIDCProvider(context.Background(), &cfg.Auth.OIDC, ""); err != nil {
		t.Fatal(err)
	}
	live := h.provider()
	rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]string{"issuer_url": "http://127.0.0.1:1"}}, nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "OIDC discovery failed") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if h.provider() != live || cfg.Auth.OIDC.IssuerURL != idp.URL {
		t.Error("failed discovery changed the provider or config")
	}
}

func TestUpdateAuthMethod_OIDCLocalLoginGuard(t *testing.T) {
	idp := mockOIDCDiscoveryServer(t)
	settings := map[string]interface{}{"issuer_url": idp.URL, "client_id": "c", "disable_local_login": true}
	body := map[string]interface{}{"method": "oidc", "oidc": settings}

	t.Run("password session turning it on", func(t *testing.T) {
		cfg, path := loadConfigWithEnv(t, "auth:\n  method: builtin\n", nil)
		h, _ := newOIDCSaveHandler(t, cfg, path)
		sess, _ := h.sessionStore.Create("admin", "admin", "admin")
		rec := putAuthMethod(h, body, sess)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "sign in with SSO once before turning off local login") {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
		if cfg.Auth.Method != "builtin" || h.provider() != nil {
			t.Error("refused save changed state")
		}
	})

	t.Run("password session after leaving oidc with it stored on", func(t *testing.T) {
		// oidc.enabled false: local login is effectively on, so this is a
		// change from on to off even though the stored flag is already set.
		cfg, path := loadConfigWithEnv(t, fmt.Sprintf("auth:\n  method: builtin\n  oidc:\n    enabled: false\n    issuer_url: %s\n    client_id: c\n    disable_local_login: true\n", idp.URL), nil)
		h, _ := newOIDCSaveHandler(t, cfg, path)
		sess, _ := h.sessionStore.Create("admin", "admin", "admin")
		if rec := putAuthMethod(h, body, sess); rec.Code != http.StatusConflict {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("OIDC session turning it on", func(t *testing.T) {
		cfg, path := loadConfigWithEnv(t, "auth:\n  method: builtin\n", nil)
		h, _ := newOIDCSaveHandler(t, cfg, path)
		sess, _ := h.sessionStore.CreateWithData("sso-1", "alice", "admin", map[string]interface{}{"oidc_id_token": "x"})
		rec := putAuthMethod(h, body, sess)
		if rec.Code != http.StatusOK || !cfg.Auth.OIDC.DisableLocalLogin {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("password session re-saving with it already on", func(t *testing.T) {
		cfg, path := loadConfigWithEnv(t, fmt.Sprintf("auth:\n  method: oidc\n  oidc:\n    enabled: true\n    issuer_url: %s\n    client_id: c\n    disable_local_login: true\n", idp.URL), nil)
		h, _ := newOIDCSaveHandler(t, cfg, path)
		sess, _ := h.sessionStore.Create("admin", "admin", "admin")
		rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]interface{}{"scopes": []string{"openid"}, "disable_local_login": true}}, sess)
		if rec.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestUpdateAuthMethod_OIDCKeepsEnvReferences(t *testing.T) {
	idp := mockOIDCDiscoveryServer(t)
	cfg, path := loadConfigWithEnv(t, fmt.Sprintf(`auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: %s
    client_id: muximux
    client_secret: ${OIDC_CLIENT_SECRET}
`, idp.URL), map[string]string{"OIDC_CLIENT_SECRET": "expanded-secret"})
	h, _ := newOIDCSaveHandler(t, cfg, path)

	typed := "typed-over-env"
	rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]interface{}{
		"scopes": []string{"openid", "groups"}, "client_secret": typed,
	}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	file := readFile(t, path)
	if !strings.Contains(file, "${OIDC_CLIENT_SECRET}") || strings.Contains(file, "expanded-secret") || strings.Contains(file, "typed-over-env") {
		t.Errorf("file lost the env reference:\n%s", file)
	}
	if !strings.Contains(file, "groups") {
		t.Errorf("scopes not saved:\n%s", file)
	}
	if cfg.Auth.OIDC.ClientSecret != "expanded-secret" {
		t.Errorf("in-memory secret = %q", cfg.Auth.OIDC.ClientSecret)
	}
}

func TestUpdateAuthMethod_OIDCSaveFailureRollsBack(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	idp := mockOIDCDiscoveryServer(t)
	cfg, path := loadConfigWithEnv(t, fmt.Sprintf("auth:\n  method: oidc\n  oidc:\n    enabled: true\n    issuer_url: %s\n    client_id: c\n    client_secret: old\n    scopes: [openid]\n", idp.URL), nil)
	h, mw := newOIDCSaveHandler(t, cfg, path)
	h.authMiddleware.UpdateConfig(&auth.AuthConfig{Method: auth.AuthMethodOIDC})
	if err := h.ReplaceOIDCProvider(context.Background(), &cfg.Auth.OIDC, ""); err != nil {
		t.Fatal(err)
	}
	live := h.provider()
	prior := copyOIDCConfig(&cfg.Auth.OIDC)

	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]interface{}{
		"client_secret": "new", "scopes": []string{"openid", "email"}, "provider_logout": true,
	}}, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !reflect.DeepEqual(cfg.Auth.OIDC, prior) || cfg.Auth.Method != "oidc" {
		t.Errorf("config not rolled back: %+v", cfg.Auth.OIDC)
	}
	if h.provider() != live || mw.Method() != auth.AuthMethodOIDC {
		t.Error("save failure changed the live provider or middleware")
	}
}

func TestUpdateAuthMethod_LeaveAndReturnToOIDC(t *testing.T) {
	idp := mockOIDCDiscoveryServer(t)
	cfg, path := loadConfigWithEnv(t, "auth:\n  method: none\n", nil)
	h, mw := newOIDCSaveHandler(t, cfg, path)

	if rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]string{
		"issuer_url": idp.URL, "client_id": "muximux", "client_secret": "stored",
	}}, nil); rec.Code != http.StatusOK {
		t.Fatalf("to oidc: code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := putAuthMethod(h, map[string]string{"method": "builtin"}, nil); rec.Code != http.StatusOK {
		t.Fatalf("to builtin: code=%d body=%s", rec.Code, rec.Body.String())
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if o := reloaded.Auth.OIDC; o.Enabled || o.IssuerURL != idp.URL || o.ClientID != "muximux" || reloaded.Auth.Method != "builtin" {
		t.Errorf("file after leaving oidc = %+v", reloaded.Auth)
	}
	if h.provider() != nil || mw.Method() != auth.AuthMethodBuiltin {
		t.Error("provider still installed after leaving oidc")
	}

	if rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]interface{}{}}, nil); rec.Code != http.StatusOK {
		t.Fatalf("back to oidc: code=%d body=%s", rec.Code, rec.Body.String())
	}
	if cfg.Auth.OIDC.ClientSecret != "stored" || !cfg.Auth.OIDC.Enabled || h.provider() == nil {
		t.Errorf("returning to oidc lost settings: %+v", cfg.Auth.OIDC)
	}
}

func TestUpdateAuthMethod_BuiltinKeepsOIDCAddon(t *testing.T) {
	// OIDC enabled next to the builtin method stays as it is when the
	// method is re-saved: only leaving the oidc method turns it off.
	idp := mockOIDCDiscoveryServer(t)
	cfg, path := loadConfigWithEnv(t, fmt.Sprintf("auth:\n  method: builtin\n  oidc:\n    enabled: true\n    issuer_url: %s\n    client_id: c\n", idp.URL), nil)
	h, _ := newOIDCSaveHandler(t, cfg, path)
	if err := h.ReplaceOIDCProvider(context.Background(), &cfg.Auth.OIDC, ""); err != nil {
		t.Fatal(err)
	}
	live := h.provider()
	if rec := putAuthMethod(h, map[string]string{"method": "builtin"}, nil); rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !cfg.Auth.OIDC.Enabled || h.provider() != live {
		t.Error("re-saving builtin turned off OIDC")
	}
}

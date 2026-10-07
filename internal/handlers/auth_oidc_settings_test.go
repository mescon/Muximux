package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

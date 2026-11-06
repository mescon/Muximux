package server

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

func TestAppBypassPath(t *testing.T) {
	tests := []struct {
		rule string
		want string
		ok   bool
	}{
		{"/api/*", "/proxy/sonarr/api/*", true},
		{"api/*", "/proxy/sonarr/api/*", true},
		{"  /feed/*  ", "/proxy/sonarr/feed/*", true},
		{"/feed/rss", "/proxy/sonarr/feed/rss", true},
		{"/", "/proxy/sonarr/", true},
		{"*", "/proxy/sonarr/*", true},
		{"/*", "/proxy/sonarr/*", true},
		// Already the full path under this app.
		{"/proxy/sonarr/hooks/github", "/proxy/sonarr/hooks/github", true},
		{"/proxy/sonarr", "/proxy/sonarr", true},
		// "/proxy/sonarr*" must not also cover /proxy/sonarr2/.
		{"/proxy/sonarr*", "/proxy/sonarr/*", true},
		// Refused: empty (would match everything), other apps, escapes.
		{"", "", false},
		{"   ", "", false},
		{"/proxy/radarr/api/*", "", false},
		{"/proxy/sonarr2/api/*", "", false},
		{"/proxy/", "", false},
		{"/../api/*", "", false},
		{"/api/../../admin", "", false},
		{"/proxy/sonarr/../radarr/*", "", false},
	}
	for _, tt := range tests {
		got, ok := appBypassPath("sonarr", tt.rule)
		if got != tt.want || ok != tt.ok {
			t.Errorf("appBypassPath(%q) = %q, %v; want %q, %v", tt.rule, got, ok, tt.want, tt.ok)
		}
	}
	if _, ok := appBypassPath("", "/api/*"); ok {
		t.Error("an empty slug must be refused")
	}
}

func TestAppBypassRules(t *testing.T) {
	apps := []config.AppConfig{
		{
			Name: "Sonarr", URL: "http://sonarr:8989", Proxy: true, Enabled: true,
			AuthBypass: []config.AuthBypassRule{
				{Path: "/api/*", Methods: []string{"GET", "POST"}, RequireAPIKey: true, AllowedIPs: []string{"192.168.0.0/16"}},
				{Path: "/proxy/radarr/*"}, // another app's prefix: skipped
				{Path: ""},                // would match everything: skipped
				{Path: "/feed/*", AllowedIPs: []string{"not-an-ip"}},
			},
		},
		{Name: "Direct", URL: "http://direct", Enabled: true, AuthBypass: []config.AuthBypassRule{{Path: "/x"}}},
		{Name: "Off", URL: "http://off", Proxy: true, AuthBypass: []config.AuthBypassRule{{Path: "/x"}}},
		{Name: "Plain", URL: "http://plain", Proxy: true, Enabled: true},
	}
	want := []auth.BypassRule{
		{Path: "/proxy/sonarr/api/*", Methods: []string{"GET", "POST"}, RequireAPIKey: true, AllowedIPs: []string{"192.168.0.0/16"}},
		{Path: "/proxy/sonarr/feed/*", AllowedIPs: []string{"not-an-ip"}},
	}
	got := appBypassRules(apps)
	for i := range got {
		if len(got[i].Methods) == 0 {
			got[i].Methods = nil
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appBypassRules =\n %+v\nwant\n %+v", got, want)
	}
	// The rules are copies: editing the config afterwards does not reach them.
	apps[0].AuthBypass[0].Methods[0] = "DELETE"
	if got[0].Methods[0] != "GET" {
		t.Error("appBypassRules must copy the methods slice")
	}
	if appBypassRules(nil) != nil {
		t.Error("no apps should give no rules")
	}
}

// TestAppBypass_EndToEnd drives the documented Sonarr example through the
// real handler chain: builtin auth, an API key, and a proxied app with
// auth_bypass on /api/*.
func TestAppBypass_EndToEnd(t *testing.T) {
	var hits atomic.Int32
	var lastPath, lastKey atomic.Value
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		lastPath.Store(r.URL.Path)
		lastKey.Store(r.Header.Get("X-Api-Key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	const key = "muximux-test-key"
	setup := completeSetup(t)
	s := newServerForTest(t, func(cfg *config.Config) {
		setup(cfg)
		cfg.Auth.APIKeyHash = auth.HashAPIKey(key)
		cfg.Apps = []config.AppConfig{{
			Name: "Sonarr", URL: backend.URL, Proxy: true, Enabled: true,
			ProxyHeaders: map[string]string{"X-Api-Key": "sonarr-own-key"},
			AuthBypass: []config.AuthBypassRule{
				{Path: "/api/*", Methods: []string{"GET", "POST"}, RequireAPIKey: true},
			},
		}}
	})

	do := func(method, path, apiKey string) int {
		req := httptest.NewRequest(method, path, nil)
		if apiKey != "" {
			req.Header.Set("X-Api-Key", apiKey)
		}
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do(http.MethodGet, "/proxy/sonarr/api/v3/series", key); code != http.StatusOK {
		t.Fatalf("key on an allowlisted path: status %d, want 200", code)
	}
	if got := lastPath.Load(); got != "/api/v3/series" {
		t.Errorf("backend saw path %v, want /api/v3/series", got)
	}
	if got := lastKey.Load(); got != "sonarr-own-key" {
		t.Errorf("backend saw X-Api-Key %v, want the app's own key from proxy_headers", got)
	}

	before := hits.Load()
	for _, c := range []struct {
		method, path, key, why string
	}{
		{http.MethodGet, "/proxy/sonarr/api/v3/series", "", "no key"},
		{http.MethodGet, "/proxy/sonarr/api/v3/series", "wrong", "wrong key"},
		{http.MethodDelete, "/proxy/sonarr/api/v3/series/1", key, "method not in the rule"},
		{http.MethodGet, "/proxy/sonarr/settings", key, "path outside the rule"},
	} {
		if code := do(c.method, c.path, c.key); code == http.StatusOK {
			t.Errorf("%s: status 200, want auth to stop it", c.why)
		}
	}
	if hits.Load() != before {
		t.Error("a refused request reached the backend")
	}

	// A Settings save rebuilds the rules: dropping the rule closes the path.
	s.config.Apps[0].AuthBypass = nil
	s.rebuildProxyRoutes()
	if code := do(http.MethodGet, "/proxy/sonarr/api/v3/series", key); code == http.StatusOK {
		t.Error("the rule still applied after it was removed from the config")
	}
}

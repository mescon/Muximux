package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// newTestGatewayAuthHandler wires the handler with a fresh session
// store and the given gateway sites. dashboardURL defaults to the
// canonical test value so redirects are predictable.
func newTestGatewayAuthHandler(t *testing.T, sites []config.GatewaySite) (*GatewayAuthHandler, *auth.SessionStore) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server.GatewaySites = sites
	store := auth.NewSessionStore("muximux_session", time.Hour, false)
	t.Cleanup(func() { store.Close() })
	return NewGatewayAuthHandler(store, cfg, &sync.RWMutex{}, "https://muximux.example.com"), store
}

func TestGatewayAuth_RejectsNonGet(t *testing.T) {
	h, _ := newTestGatewayAuthHandler(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.Code)
	}
}

func TestGatewayAuth_MissingHostHeaderIs400(t *testing.T) {
	// Direct browser hit (no X-Forwarded-Host) should not leak any
	// session-related response.
	h, _ := newTestGatewayAuthHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400 for missing X-Forwarded-Host", w.Code)
	}
}

func TestGatewayAuth_UnknownHostIs500(t *testing.T) {
	// Caddy calling us for a host we don't recognise: server
	// misconfiguration. Fail closed.
	h, _ := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "evil.example.com")
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status %d, want 500 for unknown host", w.Code)
	}
}

func TestGatewayAuth_UngatedSiteFailsOpenWith200(t *testing.T) {
	// Caddy shouldn't call us for ungated sites, but if it does
	// we fail open with a 200 (no auth requested = no auth
	// blocked) and log loudly so the misconfig surfaces.
	h, _ := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "open.example.com", BackendURL: "http://10.0.0.5:80", RequireAuth: false},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "open.example.com")
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status %d, want 200 for ungated site", w.Code)
	}
}

func TestGatewayAuth_AnonymousRedirectsToLoginWithNext(t *testing.T) {
	h, _ := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "sonarr.example.com")
	req.Header.Set("X-Forwarded-Uri", "/series/123")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("status %d, want 302", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://muximux.example.com/login?") {
		t.Errorf("Location does not point at dashboard /login: %q", loc)
	}
	if !strings.Contains(loc, "next=https%3A%2F%2Fsonarr.example.com%2Fseries%2F123") {
		t.Errorf("Location missing next= param with original URL: %q", loc)
	}
}

func TestGatewayAuth_AnonymousReturns503WhenNoDashboardURL(t *testing.T) {
	// Operator hasn't configured tls.domain; we can't build a valid
	// absolute login redirect. 503 with an actionable message
	// rather than a relative redirect that would 404 at the gated
	// host.
	cfg := &config.Config{}
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	}
	store := auth.NewSessionStore("muximux_session", time.Hour, false)
	t.Cleanup(func() { store.Close() })
	h := NewGatewayAuthHandler(store, cfg, &sync.RWMutex{}, "") // no dashboard URL

	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "sonarr.example.com")
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503 without dashboard URL", w.Code)
	}
	if !strings.Contains(w.Body.String(), "server.tls.domain") {
		t.Errorf("503 body should hint at tls.domain config; got %q", w.Body.String())
	}
}

func TestGatewayAuth_ValidSessionAllowed(t *testing.T) {
	h, store := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	})
	sess, err := store.Create("u1", "alice", auth.RoleUser)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "sonarr.example.com")
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	w := httptest.NewRecorder()
	h.Forward(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Muximux-User") != "alice" {
		t.Errorf("X-Muximux-User = %q", w.Header().Get("X-Muximux-User"))
	}
	if w.Header().Get("X-Muximux-Role") != auth.RoleUser {
		t.Errorf("X-Muximux-Role = %q", w.Header().Get("X-Muximux-Role"))
	}
}

func TestGatewayAuth_RoleInsufficient_403(t *testing.T) {
	h, store := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "admin.example.com", BackendURL: "http://10.0.0.5:80", RequireAuth: true, MinRole: "admin"},
	})
	sess, _ := store.Create("u1", "alice", auth.RoleUser) // not admin
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "admin.example.com")
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "alice") || !strings.Contains(body, "admin.example.com") {
		t.Errorf("forbidden body missing username/host: %q", body)
	}
	if !strings.Contains(body, "role_insufficient") {
		t.Errorf("forbidden body missing reason tag: %q", body)
	}
}

func TestGatewayAuth_AdminBypassesRoleAndGroups(t *testing.T) {
	// Admin should pass even when MinRole + AllowedGroups would
	// reject other users.
	h, store := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "admin.example.com", BackendURL: "http://10.0.0.5:80", RequireAuth: true,
			MinRole: "admin", AllowedGroups: []string{"impossible"}},
	})
	sess, _ := store.Create("u1", "boss", auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "admin.example.com")
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("admin should bypass role+group checks; got %d body=%q", w.Code, w.Body.String())
	}
}

func TestGatewayAuth_AllowedGroupsMatch(t *testing.T) {
	h, store := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "family.example.com", BackendURL: "http://10.0.0.5:80", RequireAuth: true,
			AllowedGroups: []string{"Family", "ADMINS"}},
	})
	sess, _ := store.Create("u1", "bob", auth.RoleUser)
	sess.Data = map[string]interface{}{"groups": []string{"family"}} // case-insensitive match
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "family.example.com")
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("group match should pass; got %d", w.Code)
	}
}

func TestGatewayAuth_AllowedGroupsMismatch_403(t *testing.T) {
	h, store := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "family.example.com", BackendURL: "http://10.0.0.5:80", RequireAuth: true,
			AllowedGroups: []string{"family"}},
	})
	sess, _ := store.Create("u1", "bob", auth.RoleUser)
	sess.Data = map[string]interface{}{"groups": []string{"friends"}} // wrong group
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "family.example.com")
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("group mismatch should 403; got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "group_mismatch") {
		t.Errorf("403 body missing group_mismatch tag: %q", w.Body.String())
	}
}

func TestGatewayAuth_HostWithPortIsStripped(t *testing.T) {
	// When server.gateway_listen is set Caddy passes
	// "sonarr.example.com:8443" as X-Forwarded-Host; we must
	// strip the port to find the site.
	h, store := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	})
	sess, _ := store.Create("u1", "alice", auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-Host", "sonarr.example.com:8443")
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	w := httptest.NewRecorder()
	h.Forward(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("hostname:port should match the site by hostname only; got %d", w.Code)
	}
}

func TestHTMLEscape_HandlesUnsafeChars(t *testing.T) {
	got := htmlEscape(`<script>alert("x")</script>`)
	for _, raw := range []string{"<", ">", `"`, "<script>"} {
		if strings.Contains(got, raw) {
			t.Errorf("htmlEscape left %q in output: %q", raw, got)
		}
	}
}

// TestGatewayAuth_RejectsNonLoopbackCaller pins the loopback gate.
// The handler is registered on the shared HTTP mux for routing
// reasons, but only Caddy's in-process forward_auth (which always
// calls back via 127.0.0.1) should ever reach it. Any other caller
// must be rejected so a public visitor can't probe session validity
// or gateway-site configuration by hitting /api/auth/forward
// directly.
func TestGatewayAuth_RejectsNonLoopbackCaller(t *testing.T) {
	h, store := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	})
	sess, _ := store.Create("u1", "alice", auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
	req.RemoteAddr = "203.0.113.7:54321" // TEST-NET-3, not loopback
	req.Header.Set("X-Forwarded-Host", "sonarr.example.com")
	req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
	w := httptest.NewRecorder()
	h.Forward(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("non-loopback caller got %d, want 403", w.Code)
	}
	// The 403 must NOT leak session-related state - no
	// X-Muximux-User / X-Muximux-Role headers should appear.
	if w.Header().Get("X-Muximux-User") != "" || w.Header().Get("X-Muximux-Role") != "" {
		t.Errorf("non-loopback rejection leaked identity headers: user=%q role=%q",
			w.Header().Get("X-Muximux-User"), w.Header().Get("X-Muximux-Role"))
	}
}

// TestGatewayAuth_RedirectNextUsesValidatedHostname pins the
// open-redirect defence. Even if a malicious header tries to inject
// a different hostname suffix or a userinfo segment, the rebuilt
// next= URL must use the config-validated site.Domain. Only the
// port suffix from the header is inherited (legitimate when
// gateway_listen sets a non-default port).
func TestGatewayAuth_RedirectNextUsesValidatedHostname(t *testing.T) {
	h, _ := newTestGatewayAuthHandler(t, []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	})

	t.Run("ignores attacker-controlled port-less host", func(t *testing.T) {
		// Suppose a future regression makes hostOnly match by some
		// prefix logic; the rebuilt URL must still use site.Domain
		// verbatim. We can't construct that path here without
		// breaking findGatewaySite, so simulate the next-best thing:
		// matching X-Forwarded-Host gets rebuilt from site.Domain
		// exactly (no userinfo, no fragment, just scheme://host/uri).
		req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set("X-Forwarded-Host", "sonarr.example.com")
		req.Header.Set("X-Forwarded-Uri", "/queue")
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		h.Forward(w, req)

		loc := w.Header().Get("Location")
		if !strings.Contains(loc, "next=https%3A%2F%2Fsonarr.example.com%2Fqueue") {
			t.Errorf("next= should encode the validated host verbatim; got %q", loc)
		}
		// Must not contain raw "sonarr.example.com" in the
		// pre-encode position (URL encoding turns / into %2F).
		if strings.Contains(loc, "next=https://sonarr.example.com") {
			t.Errorf("next= value should be URL-encoded; got %q", loc)
		}
	})

	t.Run("inherits port suffix from header", func(t *testing.T) {
		// gateway_listen=:8443 makes Caddy pass hostname:8443. The
		// rebuilt URL needs to preserve the port so the post-login
		// bounce lands on the right listener.
		req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set("X-Forwarded-Host", "sonarr.example.com:8443")
		req.Header.Set("X-Forwarded-Uri", "/queue")
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		h.Forward(w, req)

		loc := w.Header().Get("Location")
		if !strings.Contains(loc, "next=https%3A%2F%2Fsonarr.example.com%3A8443%2Fqueue") {
			t.Errorf("next= should preserve port suffix from header; got %q", loc)
		}
	})
}

// TestPortSuffix covers the tiny helper that decides whether the
// raw X-Forwarded-Host carries a port we need to inherit onto the
// rebuilt next= URL.
func TestPortSuffix(t *testing.T) {
	cases := map[string]string{
		"sonarr.example.com":      "",
		"sonarr.example.com:8443": ":8443",
		"":                        "",
		":8443":                   ":8443", // degenerate but harmless
	}
	for in, want := range cases {
		if got := portSuffix(in); got != want {
			t.Errorf("portSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsLoopbackRemote covers IPv4, IPv6, and malformed RemoteAddr.
func TestIsLoopbackRemote(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:54321", true},
		{"127.255.255.254:1", true},
		{"[::1]:8080", true},
		{"203.0.113.7:54321", false},
		{"10.0.0.1:443", false},
		{"", false},
		{"not-an-address", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = tc.addr
		if got := isLoopbackRemote(req); got != tc.want {
			t.Errorf("isLoopbackRemote(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

// BenchmarkGatewayAuth_ValidSessionHot mirrors the production hot
// path: Caddy calls /api/auth/forward once per gated request with a
// valid session cookie attached. This is the request the perf budget
// has to absorb at typical homelab traffic (dozens of req/s) and at
// occasional bursts (a tab reload firing 10+ subresource fetches).
//
// Tracked as the smoke-test baseline for the 3.1.0 auth gate so
// future changes that regress it surface in CI rather than as a
// user-visible slowdown.
func BenchmarkGatewayAuth_ValidSessionHot(b *testing.B) {
	cfg := &config.Config{}
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	}
	store := auth.NewSessionStore("muximux_session", time.Hour, false)
	defer store.Close()
	h := NewGatewayAuthHandler(store, cfg, &sync.RWMutex{}, "https://muximux.example.com")
	sess, err := store.Create("u1", "alice", auth.RoleUser)
	if err != nil {
		b.Fatalf("Create session: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set("X-Forwarded-Host", "sonarr.example.com")
		req.AddCookie(&http.Cookie{Name: "muximux_session", Value: sess.ID})
		w := httptest.NewRecorder()
		h.Forward(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("hot path returned %d, want 200", w.Code)
		}
	}
}

// BenchmarkGatewayAuth_AnonymousRedirect covers the cold path: an
// unauthenticated visitor hits a gated subdomain and gets the
// login-redirect HTML. Slower than the hot path (renders an HTML
// body, builds a signed return-to URL) but still bounded - a
// regression here would slow first-paint for every public visitor.
func BenchmarkGatewayAuth_AnonymousRedirect(b *testing.B) {
	cfg := &config.Config{}
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.5:8989", RequireAuth: true},
	}
	store := auth.NewSessionStore("muximux_session", time.Hour, false)
	defer store.Close()
	h := NewGatewayAuthHandler(store, cfg, &sync.RWMutex{}, "https://muximux.example.com")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/forward", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set("X-Forwarded-Host", "sonarr.example.com")
		req.Header.Set("X-Forwarded-Uri", "/sonarr/queue")
		w := httptest.NewRecorder()
		h.Forward(w, req)
		// Anonymous visitor gets a redirect-page response (200 with
		// HTML, or a 302 - either way the handler completed without
		// 5xx). Don't pin the exact status here; failure mode would
		// be a 500/panic.
		if w.Code >= 500 {
			b.Fatalf("anonymous path returned %d, body=%s", w.Code, w.Body.String())
		}
	}
}

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

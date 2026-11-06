package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// TestActionClient_DoesNotFollowRedirects: http_action targets are
// admin-configured (a private-IP homelab webhook is legitimate, so the
// initial request is not IP-filtered), but the redirect Location is
// controlled by the target server -- an external target that 302s to
// 169.254.169.254 or 127.0.0.1 would let it drive internal requests
// server-side. The client must not follow redirects.
func TestActionClient_DoesNotFollowRedirects(t *testing.T) {
	internalHit := false
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer internal.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL+"/latest/meta-data/", http.StatusFound)
	}))
	defer redirector.Close()

	h := NewAPIHandler(&config.Config{}, "", &sync.RWMutex{})
	req, err := http.NewRequest(http.MethodGet, redirector.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := h.actionClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusFound {
		t.Errorf("redirect must not be followed; want 302, got %d", resp.StatusCode)
	}
	if internalHit {
		t.Error("actionClient followed the redirect to the internal target (SSRF)")
	}
}

func newFireActionTest(t *testing.T, app *config.AppConfig, upstream http.HandlerFunc) *APIHandler {
	t.Helper()
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)

	if app.URL == "RESOLVE_UPSTREAM" || app.URL == "RESOLVE_UPSTREAM_WITH_QUERY" {
		app.URL = server.URL
	} else if app.URL != "" && !strings.HasPrefix(app.URL, "http") {
		app.URL = server.URL + app.URL
	}

	cfg := &config.Config{Apps: []config.AppConfig{*app}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	h.actionClient = &http.Client{Timeout: 200 * time.Millisecond}
	return h
}

func fireWithUser(h *APIHandler, name string, user *auth.User) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/apps/"+url.PathEscape(name)+"/fire-action", nil)
	if user != nil {
		req = req.WithContext(context.WithValue(req.Context(), auth.ContextKeyUser, user))
	}
	w := httptest.NewRecorder()
	h.FireAppAction(w, req, name)
	return w
}

func defaultUser() *auth.User {
	return &auth.User{ID: "u", Username: "alice", Role: auth.RoleUser}
}

func TestFireAppAction_Success(t *testing.T) {
	var gotMethod, gotAuth string
	upstream := func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}
	h := newFireActionTest(t, &config.AppConfig{
		Name:              "Webhook",
		URL:               "RESOLVE_UPSTREAM",
		Enabled:           true,
		OpenMode:          "http_action",
		HTTPActionMethod:  "POST",
		HTTPActionHeaders: map[string]string{"Authorization": "Bearer secret"},
	}, upstream)

	w := fireWithUser(h, "Webhook", defaultUser())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	if gotMethod != "POST" {
		t.Errorf("upstream method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("upstream Authorization = %q, want Bearer secret", gotAuth)
	}
	var resp fireActionResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusAccepted {
		t.Errorf("resp.Status = %d, want 202", resp.Status)
	}
	if resp.Method != "POST" {
		t.Errorf("resp.Method = %q, want POST", resp.Method)
	}
	if resp.URLHost == "" {
		t.Errorf("resp.URLHost should be set")
	}
	if resp.Error != "" {
		t.Errorf("resp.Error should be empty, got %q", resp.Error)
	}
}

func TestFireAppAction_DefaultMethodIsPOST(t *testing.T) {
	var gotMethod string
	h := newFireActionTest(t, &config.AppConfig{
		Name:     "Webhook",
		URL:      "RESOLVE_UPSTREAM",
		Enabled:  true,
		OpenMode: "http_action",
	}, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	})
	w := fireWithUser(h, "Webhook", defaultUser())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotMethod != "POST" {
		t.Errorf("default method = %q, want POST", gotMethod)
	}
}

func TestFireAppAction_AllVerbs(t *testing.T) {
	for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		t.Run(m, func(t *testing.T) {
			var gotMethod string
			h := newFireActionTest(t, &config.AppConfig{
				Name:             "Webhook",
				URL:              "RESOLVE_UPSTREAM",
				Enabled:          true,
				OpenMode:         "http_action",
				HTTPActionMethod: m,
			}, func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(http.StatusOK)
			})
			w := fireWithUser(h, "Webhook", defaultUser())
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if gotMethod != m {
				t.Errorf("upstream method = %q, want %q", gotMethod, m)
			}
		})
	}
}

func TestFireAppAction_Backend4xx(t *testing.T) {
	h := newFireActionTest(t, &config.AppConfig{
		Name: "Webhook", URL: "RESOLVE_UPSTREAM", Enabled: true, OpenMode: "http_action",
	}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	w := fireWithUser(h, "Webhook", defaultUser())
	if w.Code != http.StatusOK {
		t.Fatalf("expected our 200 (status surfaced in body), got %d", w.Code)
	}
	var resp fireActionResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Errorf("resp.Status = %d, want 400", resp.Status)
	}
}

func TestFireAppAction_Backend5xx(t *testing.T) {
	h := newFireActionTest(t, &config.AppConfig{
		Name: "Webhook", URL: "RESOLVE_UPSTREAM", Enabled: true, OpenMode: "http_action",
	}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	w := fireWithUser(h, "Webhook", defaultUser())
	if w.Code != http.StatusOK {
		t.Fatalf("expected our 200 (status surfaced in body), got %d", w.Code)
	}
	var resp fireActionResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusBadGateway {
		t.Errorf("resp.Status = %d, want 502", resp.Status)
	}
}

func TestFireAppAction_Timeout(t *testing.T) {
	h := newFireActionTest(t, &config.AppConfig{
		Name: "Webhook", URL: "RESOLVE_UPSTREAM", Enabled: true, OpenMode: "http_action",
	}, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	w := fireWithUser(h, "Webhook", defaultUser())
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on timeout, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp fireActionResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Errorf("expected resp.Error to be set on timeout")
	}
}

func TestFireAppAction_NetworkError(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{
		Name: "Webhook", URL: "http://127.0.0.1:1", Enabled: true, OpenMode: "http_action",
	}}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	h.actionClient = &http.Client{Timeout: 200 * time.Millisecond}
	w := fireWithUser(h, "Webhook", defaultUser())
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on connection refused, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp fireActionResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Errorf("expected resp.Error to be set on network failure")
	}
}

func TestFireAppAction_NotAnHTTPAction(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{
		Name: "App", URL: "http://example.com", OpenMode: "iframe",
	}}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	w := fireWithUser(h, "App", defaultUser())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-http_action app, got %d", w.Code)
	}
}

func TestFireAppAction_AppNotFound(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	w := fireWithUser(h, "Missing", defaultUser())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestFireAppAction_AccessDenied_Role(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{
		Name: "Admin Webhook", URL: "http://example.com", Enabled: true, OpenMode: "http_action", MinRole: "admin",
	}}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	w := fireWithUser(h, "Admin Webhook", defaultUser())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for role mismatch, got %d", w.Code)
	}
}

func TestFireAppAction_AccessDenied_Group(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{
		Name: "Restricted Webhook", URL: "http://example.com", Enabled: true, OpenMode: "http_action", AllowedGroups: []string{"ops"},
	}}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	user := &auth.User{ID: "u", Username: "alice", Role: auth.RoleUser, Groups: []string{"devs"}}
	w := fireWithUser(h, "Restricted Webhook", user)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for group mismatch, got %d", w.Code)
	}
}

func TestFireAppAction_AdminBypassesGroupGate(t *testing.T) {
	var hit bool
	upstream := func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}
	h := newFireActionTest(t, &config.AppConfig{
		Name: "Restricted", URL: "RESOLVE_UPSTREAM", Enabled: true, OpenMode: "http_action", AllowedGroups: []string{"ops"},
	}, upstream)
	admin := &auth.User{ID: "u", Username: "root", Role: auth.RoleAdmin}
	w := fireWithUser(h, "Restricted", admin)
	if w.Code != http.StatusOK {
		t.Fatalf("admin should bypass group gate; got %d (body: %s)", w.Code, w.Body.String())
	}
	if !hit {
		t.Error("upstream should have been called")
	}
}

func TestFireAppAction_QueryStringNotInAuditLog(t *testing.T) {
	h := newFireActionTest(t, &config.AppConfig{
		Name: "Webhook", URL: "RESOLVE_UPSTREAM_WITH_QUERY", Enabled: true, OpenMode: "http_action",
	}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	base := h.config.Apps[0].URL
	h.config.Apps[0].URL = base + "/path?token=should-not-appear"
	w := fireWithUser(h, "Webhook", defaultUser())
	var resp fireActionResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resp.URLHost, "?") || strings.Contains(resp.URLHost, "token") {
		t.Errorf("URLHost leaks query string: %q", resp.URLHost)
	}
	if strings.Contains(resp.URLHost, "/") {
		t.Errorf("URLHost leaks path: %q", resp.URLHost)
	}
}

func TestSafeURLHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://example.com/path?q=1", "example.com"},
		{"http://example.com:8080/", "example.com:8080"},
		{"https://user:pass@example.com/", "example.com"},
		{"", ""},
		{"::bad::", ""},
	}
	for _, tc := range cases {
		got := safeURLHost(tc.in)
		if got != tc.want {
			t.Errorf("safeURLHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

var _ = fmt.Sprintf
var _ = io.Discard

// A disabled action is hidden from the menu, and a direct POST must not
// fire it either. It answers like an unknown app.
func TestFireAppAction_DisabledIsNotFound(t *testing.T) {
	hit := false
	h := newFireActionTest(t, &config.AppConfig{
		Name: "Webhook", URL: "RESOLVE_UPSTREAM", Enabled: false, OpenMode: "http_action",
	}, func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	})
	for _, u := range []*auth.User{defaultUser(), {ID: "a", Username: "root", Role: auth.RoleAdmin}} {
		if w := fireWithUser(h, "Webhook", u); w.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", u.Role, w.Code)
		}
	}
	if hit {
		t.Error("a disabled action reached its target")
	}
}

// access.roles / access.users gate the action like min_role and
// allowed_groups do: matching either list is enough; admins pass.
func TestFireAppAction_AccessList(t *testing.T) {
	h := newFireActionTest(t, &config.AppConfig{
		Name: "Webhook", URL: "RESOLVE_UPSTREAM", Enabled: true, OpenMode: "http_action",
		Access: config.AppAccessConfig{Roles: []string{"power-user"}, Users: []string{"Bob"}},
	}, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	cases := []struct {
		user *auth.User
		want int
	}{
		{&auth.User{ID: "1", Username: "alice", Role: auth.RoleUser}, http.StatusForbidden},
		{&auth.User{ID: "2", Username: "carol", Role: auth.RolePowerUser}, http.StatusOK},
		{&auth.User{ID: "3", Username: "bob", Role: auth.RoleUser}, http.StatusOK}, // username, case-insensitive
		{&auth.User{ID: "4", Username: "root", Role: auth.RoleAdmin}, http.StatusOK},
	}
	for _, c := range cases {
		if w := fireWithUser(h, "Webhook", c.user); w.Code != c.want {
			t.Errorf("%s (%s): status %d, want %d", c.user.Username, c.user.Role, w.Code, c.want)
		}
	}
}

// A failed relay must not echo the target URL: webhook URLs often carry
// a secret in the query string.
func TestFireAppAction_ErrorHidesURL(t *testing.T) {
	const secret = "s3cr3t-token"
	cfg := &config.Config{Apps: []config.AppConfig{{
		Name: "Webhook", URL: "http://127.0.0.1:1/hook?token=" + secret, Enabled: true, OpenMode: "http_action",
	}}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	h.actionClient = &http.Client{Timeout: 200 * time.Millisecond}
	w := fireWithUser(h, "Webhook", defaultUser())
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", w.Code)
	}
	if strings.Contains(w.Body.String(), secret) || strings.Contains(w.Body.String(), "/hook") {
		t.Errorf("response leaks the URL: %s", w.Body.String())
	}
}

func TestRelayErrorText(t *testing.T) {
	const secret = "s3cr3t-token"
	_, parseErr := url.Parse("http://exa mple.com/?token=" + secret) //nolint:staticcheck // deliberately invalid: a parse error quotes the URL
	wrapped := &url.Error{Op: "Post", URL: "https://example.com/?token=" + secret, Err: errors.New("dial tcp: connection refused")}
	for _, err := range []error{parseErr, wrapped} {
		if got := relayErrorText(err); strings.Contains(got, secret) || got == "" {
			t.Errorf("relayErrorText(%v) = %q", err, got)
		}
	}
	if got := relayErrorText(wrapped); got != "Post: dial tcp: connection refused" {
		t.Errorf("relayErrorText = %q, want the op and cause", got)
	}
	if got := relayErrorText(errors.New("plain")); got != "plain" {
		t.Errorf("non-URL error = %q, want it unchanged", got)
	}
}

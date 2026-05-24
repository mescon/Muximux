package handlers

import (
	"context"
	"encoding/json"
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

func newFireActionTest(t *testing.T, app config.AppConfig, upstream http.HandlerFunc) (*APIHandler, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)

	if app.URL == "RESOLVE_UPSTREAM" || app.URL == "RESOLVE_UPSTREAM_WITH_QUERY" {
		app.URL = server.URL
	} else if app.URL != "" && !strings.HasPrefix(app.URL, "http") {
		app.URL = server.URL + app.URL
	}

	cfg := &config.Config{Apps: []config.AppConfig{app}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	h.actionClient = &http.Client{Timeout: 200 * time.Millisecond}
	return h, server
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
	h, _ := newFireActionTest(t, config.AppConfig{
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
	h, _ := newFireActionTest(t, config.AppConfig{
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
			h, _ := newFireActionTest(t, config.AppConfig{
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
	h, _ := newFireActionTest(t, config.AppConfig{
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
	h, _ := newFireActionTest(t, config.AppConfig{
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
	h, _ := newFireActionTest(t, config.AppConfig{
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
		Name: "Admin Webhook", URL: "http://example.com", OpenMode: "http_action", MinRole: "admin",
	}}}
	h := NewAPIHandler(cfg, "", &sync.RWMutex{})
	w := fireWithUser(h, "Admin Webhook", defaultUser())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for role mismatch, got %d", w.Code)
	}
}

func TestFireAppAction_AccessDenied_Group(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{
		Name: "Restricted Webhook", URL: "http://example.com", OpenMode: "http_action", AllowedGroups: []string{"ops"},
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
	h, _ := newFireActionTest(t, config.AppConfig{
		Name: "Restricted", URL: "RESOLVE_UPSTREAM", OpenMode: "http_action", AllowedGroups: []string{"ops"},
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
	h, _ := newFireActionTest(t, config.AppConfig{
		Name: "Webhook", URL: "RESOLVE_UPSTREAM_WITH_QUERY", OpenMode: "http_action",
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

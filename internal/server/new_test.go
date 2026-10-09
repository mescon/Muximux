package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/websocket"
)

// newServerForTest builds a Server through New, exactly as main does, from
// the default configuration in a scratch data directory. Nothing is started:
// requests go straight to the assembled handler chain, so the test covers the
// route table and middleware order without opening a listener.
func newServerForTest(t *testing.T, mutate func(cfg *config.Config)) *Server {
	t.Helper()
	dataDir := t.TempDir()
	configPath := filepath.Join(dataDir, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.Server.Listen = "127.0.0.1:0"
	if mutate != nil {
		mutate(cfg)
	}
	s, err := New(cfg, configPath, dataDir, "test", "abcdef0", "2026-01-01")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func get(t *testing.T, s *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)
	return rec
}

// completeSetup marks the configuration as set up with one builtin admin,
// so the setup guard steps aside and the real auth middleware is exercised.
func completeSetup(t *testing.T) func(cfg *config.Config) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return func(cfg *config.Config) {
		cfg.Auth.Method = "builtin"
		cfg.Auth.SetupComplete = true
		cfg.Auth.Users = []config.UserConfig{{Username: "admin", PasswordHash: string(hash), Role: "admin"}}
	}
}

func TestNew_BeforeSetup(t *testing.T) {
	s := newServerForTest(t, nil)

	t.Run("records build metadata and assembles a handler", func(t *testing.T) {
		if s.version != "test" || s.commit != "abcdef0" || s.buildDate != "2026-01-01" {
			t.Fatalf("metadata not stored: %q %q %q", s.version, s.commit, s.buildDate)
		}
		if s.httpServer == nil || s.httpServer.Handler == nil {
			t.Fatal("http server or handler not assembled")
		}
	})

	t.Run("mutating api routes are held behind the setup guard", func(t *testing.T) {
		for _, p := range []string{"/api/apps", "/api/config", "/api/groups"} {
			rec := get(t, s, p)
			if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "setup_required") {
				t.Errorf("GET %s = %d %q, want 503 setup_required", p, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("the onboarding wizard's read-only routes stay reachable", func(t *testing.T) {
		for _, p := range []string{"/api/auth/status", "/api/icons/custom"} {
			if rec := get(t, s, p); rec.Code != http.StatusOK {
				t.Errorf("GET %s = %d, want 200", p, rec.Code)
			}
		}
	})
}

func TestNew_AfterSetup(t *testing.T) {
	s := newServerForTest(t, completeSetup(t))

	t.Run("api routes require a session", func(t *testing.T) {
		for _, p := range []string{"/api/apps", "/api/config", "/api/groups", "/api/auth/users"} {
			if rec := get(t, s, p); rec.Code != http.StatusUnauthorized {
				t.Errorf("GET %s = %d, want 401", p, rec.Code)
			}
		}
	})

	t.Run("security headers are applied", func(t *testing.T) {
		rec := get(t, s, "/api/auth/status")
		for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options"} {
			if rec.Header().Get(h) == "" {
				t.Errorf("missing %s header", h)
			}
		}
	})

	t.Run("encoded dot segments are rejected before auth", func(t *testing.T) {
		rec := get(t, s, "/api/..%2f..%2fetc/passwd")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("traversal = %d, want 400", rec.Code)
		}
	})

	t.Run("browser-form posts without the CSRF header are refused", func(t *testing.T) {
		// A CORS-simple content type and no X-Requested-With is exactly what a
		// cross-origin HTML form can send; the middleware must stop it.
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("username=admin&password=correct+horse"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("form POST without X-Requested-With = %d, want 403", rec.Code)
		}
	})

	t.Run("login with the CSRF header issues a session that unlocks the api", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"correct horse"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("login = %d %s", rec.Code, rec.Body.String())
		}
		cookies := rec.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("login set no cookie")
		}
		req2 := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
		for _, c := range cookies {
			req2.AddCookie(c)
		}
		rec2 := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("GET /api/apps with session = %d", rec2.Code)
		}
	})
}

func TestNew_BasePath(t *testing.T) {
	s := newServerForTest(t, func(cfg *config.Config) { cfg.Server.BasePath = "/mux" })

	t.Run("bare base path redirects to the trailing slash", func(t *testing.T) {
		rec := get(t, s, "/mux")
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/mux/" {
			t.Fatalf("GET /mux = %d %q", rec.Code, rec.Header().Get("Location"))
		}
	})

	t.Run("paths outside the base are not found", func(t *testing.T) {
		if rec := get(t, s, "/other"); rec.Code != http.StatusNotFound {
			t.Fatalf("GET /other = %d, want 404", rec.Code)
		}
	})

	t.Run("prefix is stripped before dispatch", func(t *testing.T) {
		rec := get(t, s, "/mux/api/auth/status")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /mux/api/auth/status = %d, want 200", rec.Code)
		}
	})
}

func TestNew_AuthMethodNone(t *testing.T) {
	s := newServerForTest(t, func(cfg *config.Config) {
		cfg.Auth.Method = "none"
		cfg.Auth.SetupComplete = true
	})
	rec := get(t, s, "/api/apps")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/apps with auth none = %d, want 200 (body %q)", rec.Code, rec.Body.String()[:min(80, rec.Body.Len())])
	}
}

// loginCookies signs in through the real handler chain, sending what the SPA
// sends (JSON body plus X-Requested-With), and returns the session cookies.
func loginCookies(t *testing.T, s *Server, username, password string) []*http.Cookie {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s = %d %s", username, rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()
}

func doJSON(s *Server, method, path, body string, cookies []*http.Cookie, csrf bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if csrf {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)
	return rec
}

func TestOIDCSettingsRoutes_RequireAdmin(t *testing.T) {
	const secret = "super-secret-client-value"
	hash, err := bcrypt.GenerateFromPassword([]byte("pw-user"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	setup := completeSetup(t)
	s := newServerForTest(t, func(cfg *config.Config) {
		setup(cfg)
		cfg.Auth.Users = append(cfg.Auth.Users, config.UserConfig{Username: "bob", PasswordHash: string(hash), Role: "user"})
		cfg.Auth.OIDC.ClientSecret = secret
	})

	paths := []struct{ method, path string }{
		{http.MethodGet, "/api/auth/settings/oidc"},
		{http.MethodPost, "/api/auth/settings/oidc/test"},
	}

	t.Run("anonymous callers get 401", func(t *testing.T) {
		for _, tc := range paths {
			if rec := doJSON(s, tc.method, tc.path, `{}`, nil, true); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s anonymous = %d, want 401", tc.method, tc.path, rec.Code)
			}
		}
	})

	t.Run("non-admin sessions get 403", func(t *testing.T) {
		cookies := loginCookies(t, s, "bob", "pw-user")
		cases := append([]struct{ method, path string }{}, paths...)
		cases = append(cases, struct{ method, path string }{http.MethodPut, "/api/auth/method"})
		for _, tc := range cases {
			if rec := doJSON(s, tc.method, tc.path, `{}`, cookies, true); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as user = %d, want 403", tc.method, tc.path, rec.Code)
			}
		}
	})

	t.Run("admin reads settings without the secret", func(t *testing.T) {
		cookies := loginCookies(t, s, "admin", "correct horse")
		rec := doJSON(s, http.MethodGet, "/api/auth/settings/oidc", "", cookies, false)
		if rec.Code != http.StatusOK {
			t.Fatalf("admin GET = %d %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("response leaks the client secret: %s", rec.Body.String())
		}
	})

	t.Run("the test endpoint keeps CSRF protection", func(t *testing.T) {
		cookies := loginCookies(t, s, "admin", "correct horse")
		req := httptest.NewRequest(http.MethodPost, "/api/auth/settings/oidc/test", strings.NewReader("a=b"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("form POST without CSRF marker = %d, want 403", rec.Code)
		}
	})
}

// dialWS opens a WebSocket to srv's /ws endpoint with the given session
// cookies, the way a browser tab does, and returns the event types it
// receives. A reader goroutine owns the connection: gorilla connections are
// unusable after a read deadline expires, so the 300 ms quiet window lives
// on the channel instead.
func dialWS(t *testing.T, srv *httptest.Server, cookies []*http.Cookie) <-chan string {
	t.Helper()
	header := http.Header{"Origin": []string{srv.URL}}
	for _, c := range cookies {
		header.Add("Cookie", c.String())
	}
	conn, resp, err := gws.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", header)
	if err != nil {
		t.Fatalf("dial /ws: %v", err)
	}
	resp.Body.Close()
	t.Cleanup(func() { conn.Close() })
	events := make(chan string, 64)
	go func() {
		defer close(events)
		for {
			_, msg, readErr := conn.ReadMessage()
			if readErr != nil {
				return
			}
			var ev struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msg, &ev) == nil {
				events <- ev.Type
			}
		}
	}()
	return events
}

// countConfigUpdates drains events until they are quiet for 300 ms and
// returns how many config_updated events arrived. Other types are ignored.
func countConfigUpdates(events <-chan string) int {
	n := 0
	for {
		select {
		case typ, ok := <-events:
			if !ok {
				return n
			}
			if typ == "config_updated" {
				n++
			}
		case <-time.After(300 * time.Millisecond):
			return n
		}
	}
}

// S-03: every config mutation, whichever handler makes it, reaches every
// connected client, admin or not, as exactly one config_updated event.
func TestInstallConfigSaveHook_BroadcastsOnEveryMutation(t *testing.T) {
	s := newServerForTest(t, completeSetup(t))
	go s.wsHub.Run()
	t.Cleanup(s.wsHub.Close)
	s.installConfigSaveHook()
	s.installConfigSaveHook() // idempotent
	srv := httptest.NewServer(s.httpServer.Handler)
	defer srv.Close()

	admin := loginCookies(t, s, "admin", "correct horse")
	if rec := doJSON(s, http.MethodPost, "/api/auth/users",
		`{"username":"viewer","password":"viewer-password","role":"user"}`, admin, true); rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("create viewer = %d %s", rec.Code, rec.Body.String())
	}
	viewer := loginCookies(t, s, "viewer", "viewer-password")

	adminConn := dialWS(t, srv, admin)
	viewerConn := dialWS(t, srv, viewer)
	time.Sleep(50 * time.Millisecond)
	if got := s.wsHub.ClientCount(); got != 2 {
		t.Fatalf("expected 2 ws clients, got %d", got)
	}
	// The viewer creation above saved before either socket connected, so
	// nothing is queued yet; drain anyway to start from a clean slate.
	countConfigUpdates(adminConn)
	countConfigUpdates(viewerConn)

	current := doJSON(s, http.MethodGet, "/api/config", "", admin, true)
	if current.Code != http.StatusOK {
		t.Fatalf("GET /api/config = %d", current.Code)
	}

	mutations := []struct{ method, path, body string }{
		{http.MethodPut, "/api/config", current.Body.String()},
		{http.MethodPost, "/api/apps", `{"name":"Hooked","url":"http://hooked.local:8080","enabled":true}`},
		{http.MethodPut, "/api/discovery/docker/config", `{"enabled":false}`},
		{http.MethodPost, "/api/gateway/sites", `{"domain":"hooked.example.test","backend_url":"http://10.0.0.5:8080"}`},
		{http.MethodPut, "/api/auth/method", `{"method":"builtin"}`},
		{http.MethodPost, "/api/auth/users", `{"username":"second","password":"second-password","role":"user"}`},
	}
	for _, m := range mutations {
		rec := doJSON(s, m.method, m.path, m.body, admin, true)
		if rec.Code < 200 || rec.Code > 299 {
			t.Fatalf("%s %s = %d %s", m.method, m.path, rec.Code, rec.Body.String())
		}
		if got := countConfigUpdates(adminConn); got != 1 {
			t.Errorf("%s %s: admin got %d config_updated, want 1", m.method, m.path, got)
		}
		if got := countConfigUpdates(viewerConn); got != 1 {
			t.Errorf("%s %s: viewer got %d config_updated, want 1", m.method, m.path, got)
		}
	}
}

// A restore swaps the whole config struct; the save hook must survive it so
// later saves still broadcast.
func TestHandleConfigRestore_KeepsSaveHook(t *testing.T) {
	s := newServerForTest(t, nil)
	go s.wsHub.Run()
	t.Cleanup(s.wsHub.Close)
	s.installConfigSaveHook()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(s.wsHub, w, r, false)
	}))
	defer srv.Close()
	events := dialWS(t, srv, nil)
	time.Sleep(50 * time.Millisecond)

	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader("server:\n  title: \"Restored\"\napps: []\n"))
	req.Header.Set("Content-Type", "application/x-yaml")
	withSetupToken(s, req)
	rec := httptest.NewRecorder()
	s.handleConfigRestore(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body.String())
	}
	if got := countConfigUpdates(events); got != 1 {
		t.Fatalf("restore save: got %d config_updated, want 1", got)
	}

	s.configMu.Lock()
	err := s.config.Save(s.configPath)
	s.configMu.Unlock()
	if err != nil {
		t.Fatalf("Save after restore: %v", err)
	}
	if got := countConfigUpdates(events); got != 1 {
		t.Fatalf("save after restore: got %d config_updated, want 1", got)
	}
}

func TestDiscoveryDockerConfigRoute_GetRequiresAdmin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("pw-user"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	setup := completeSetup(t)
	s := newServerForTest(t, func(cfg *config.Config) {
		setup(cfg)
		cfg.Auth.Users = append(cfg.Auth.Users, config.UserConfig{Username: "bob", PasswordHash: string(hash), Role: "user"})
	})
	const path = "/api/discovery/docker/config"

	if rec := doJSON(s, http.MethodGet, path, "", nil, false); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous GET = %d, want 401", rec.Code)
	}
	if rec := doJSON(s, http.MethodGet, path, "", loginCookies(t, s, "bob", "pw-user"), false); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin GET = %d, want 403", rec.Code)
	}
	rec := doJSON(s, http.MethodGet, path, "", loginCookies(t, s, "admin", "correct horse"), false)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin GET = %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body.Config["enabled"]; !ok {
		t.Errorf("config.enabled missing from %s", rec.Body.String())
	}
}

// postRestore sends a backup to /api/config/restore through the full
// handler chain with the server's setup token, as the onboarding UI does.
func postRestore(s *Server, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/config/restore", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-yaml")
	req.Header.Set(setupTokenHeader, s.setupToken)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)
	return rec
}

const restoredPassword = "restored-password"

// builtinBackup is a backup with builtin auth and one admin, "owner", whose
// password is restoredPassword. extra is appended as more top-level keys.
func builtinBackup(t *testing.T, extra string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(restoredPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return "auth:\n  method: builtin\n  users:\n    - username: owner\n      password_hash: \"" + string(hash) +
		"\"\n      role: admin\n      email: owner@example.test\n" + extra
}

// saveInitialConfig writes the live config to disk and returns the bytes,
// so a test can check that a rejected restore leaves the file alone.
func saveInitialConfig(t *testing.T, s *Server) []byte {
	t.Helper()
	s.configMu.Lock()
	err := s.config.Save(s.configPath)
	s.configMu.Unlock()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return data
}

func readConfigFile(t *testing.T, s *Server) string {
	t.Helper()
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(data)
}

// S-01: restoring a builtin backup on a fresh install (auth none) switches
// the running instance to builtin auth at once, as a restart would.
func TestConfigRestore_BuiltinBackupRequiresLogin(t *testing.T) {
	s := newServerForTest(t, nil)
	go s.wsHub.Run()
	t.Cleanup(s.wsHub.Close)
	s.installConfigSaveHook()
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(s.wsHub, w, r, false)
	}))
	defer wsSrv.Close()
	events := dialWS(t, wsSrv, nil)
	time.Sleep(50 * time.Millisecond)
	stale, err := s.sessionStore.Create("owner", "owner", "admin")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if rec := postRestore(s, builtinBackup(t, "")); rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body.String())
	}
	if s.sessionStore.Get(stale.ID) != nil {
		t.Error("a session from before the restore survived it")
	}
	if got := countConfigUpdates(events); got != 1 {
		t.Errorf("restore save: got %d config_updated, want 1", got)
	}
	if s.needsSetup.Load() {
		t.Error("setup still pending after restore")
	}

	if rec := doJSON(s, http.MethodGet, "/api/config", "", nil, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous GET /api/config = %d, want 401 (auth none still active?)", rec.Code)
	}
	found := false
	for _, u := range s.userStore.List() {
		if u.Username == "owner" && u.Role == "admin" {
			found = true
		}
	}
	if !found {
		t.Fatalf("user store does not hold the restored admin: %+v", s.userStore.List())
	}

	cookies := loginCookies(t, s, "owner", restoredPassword)
	countConfigUpdates(events) // a login may rehash the MinCost password and save
	current := doJSON(s, http.MethodGet, "/api/config", "", cookies, false)
	if current.Code != http.StatusOK {
		t.Fatalf("GET /api/config as restored admin = %d", current.Code)
	}
	if rec := doJSON(s, http.MethodPut, "/api/config", current.Body.String(), cookies, true); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config as restored admin = %d %s", rec.Code, rec.Body.String())
	}
	if got := countConfigUpdates(events); got != 1 {
		t.Errorf("save after restore: got %d config_updated, want 1 (hook not inherited)", got)
	}
}

func TestConfigRestore_InvalidConfigIs400(t *testing.T) {
	s := newServerForTest(t, nil)
	before := saveInitialConfig(t, s)
	title := s.config.Server.Title

	backup := builtinBackup(t, `server:
  title: Gated
  gateway_sites:
    - domain: app.example.test
      backend_url: http://10.0.0.5:8080
      require_auth: true
`)
	rec := postRestore(s, backup)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("restore = %d %s, want 400", rec.Code, rec.Body.String())
	}
	if s.config.Server.Title != title || s.config.Auth.Method == "builtin" {
		t.Errorf("in-memory config changed: title %q method %q", s.config.Server.Title, s.config.Auth.Method)
	}
	if after := readConfigFile(t, s); after != string(before) {
		t.Errorf("config file changed by a rejected restore:\n%s", after)
	}
	if !s.needsSetup.Load() {
		t.Error("setup no longer pending after a rejected restore")
	}
}

func TestConfigRestore_ProxyRoutesRebuilt(t *testing.T) {
	s := newServerForTest(t, nil)
	if rec := get(t, s, "/proxy/proxied/"); rec.Code != http.StatusNotFound {
		t.Fatalf("before restore /proxy/proxied/ = %d, want 404", rec.Code)
	}
	backup := `auth:
  method: none
apps:
  - name: Proxied
    url: http://127.0.0.1:1
    enabled: true
    proxy: true
`
	if rec := postRestore(s, backup); rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, s, "/proxy/proxied/"); rec.Code == http.StatusNotFound {
		t.Fatalf("/proxy/proxied/ still 404 after restore: %s", rec.Body.String())
	}
}

// OP-7: a restored ${VAR} is expanded at runtime and stays a reference on
// disk, through the restore and a later save.
func TestConfigRestore_ExpandsEnvRefs(t *testing.T) {
	t.Setenv("MX_TITLE", "Home")
	s := newServerForTest(t, nil)
	if rec := postRestore(s, builtinBackup(t, "server:\n  title: ${MX_TITLE}\n")); rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body.String())
	}
	if s.config.Server.Title != "Home" {
		t.Errorf("live title = %q, want Home", s.config.Server.Title)
	}
	if file := readConfigFile(t, s); !strings.Contains(file, "${MX_TITLE}") {
		t.Fatalf("restore wrote the expanded value:\n%s", file)
	}

	cookies := loginCookies(t, s, "owner", restoredPassword)
	current := doJSON(s, http.MethodGet, "/api/config", "", cookies, false)
	if rec := doJSON(s, http.MethodPut, "/api/config", current.Body.String(), cookies, true); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config = %d %s", rec.Code, rec.Body.String())
	}
	if file := readConfigFile(t, s); !strings.Contains(file, "${MX_TITLE}") {
		t.Errorf("save after restore wrote the expanded value:\n%s", file)
	}
}

// OP-7: an environment override stays live across a restore, and the file
// gets the backup's own value.
func TestConfigRestore_KeepsEnvOverrides(t *testing.T) {
	s := newServerForTest(t, nil)
	s.configMu.Lock()
	s.config.ApplyOverride(config.OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")
	s.configMu.Unlock()

	if rec := postRestore(s, "auth:\n  method: none\nserver:\n  log_level: warn\n"); rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body.String())
	}
	if s.config.Server.LogLevel != "debug" {
		t.Errorf("live log level = %q, want the override debug", s.config.Server.LogLevel)
	}
	rec := doJSON(s, http.MethodGet, "/api/config", "", nil, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config = %d", rec.Code)
	}
	var body struct {
		EnvOverrides map[string]string `json:"env_overrides"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.EnvOverrides["log_level"] != "MUXIMUX_LOG_LEVEL" {
		t.Errorf("env_overrides = %v, want log_level from MUXIMUX_LOG_LEVEL", body.EnvOverrides)
	}
	if file := readConfigFile(t, s); !strings.Contains(file, "log_level: warn") {
		t.Errorf("file does not hold the backup's log level:\n%s", file)
	}
}

// OP-7: restore does not migrate a legacy server.gateway backup.
func TestConfigRestore_LegacyGatewayIs400(t *testing.T) {
	s := newServerForTest(t, nil)
	cases := map[string]string{
		"scalar": "server:\n  gateway: /etc/caddy/Caddyfile\n",
		"map":    "server:\n  gateway:\n    file: /etc/caddy/Caddyfile\n    sites: [a]\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := postRestore(s, body)
			var resp map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v (%s)", err, rec.Body.String())
			}
			if rec.Code != http.StatusBadRequest || resp["error"] != config.ErrLegacyGateway.Error() {
				t.Fatalf("restore = %d %q, want 400 with the legacy gateway message", rec.Code, resp["error"])
			}
			if strings.Contains(resp["error"], "Invalid YAML") {
				t.Errorf("legacy gateway message wrapped as invalid YAML: %q", resp["error"])
			}
			if _, err := os.Stat(s.configPath); !os.IsNotExist(err) {
				t.Errorf("config file written by a rejected restore (err=%v)", err)
			}
		})
	}
}

// OP-7: an unresolved ${VAR} in a backup is logged as at boot.
func TestConfigRestore_MissingEnvVarWarns(t *testing.T) {
	if logging.Buffer() == nil {
		if err := logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout"}); err != nil {
			t.Fatalf("init logging: %v", err)
		}
	}
	t.Setenv("MX_UNSET_494", "")
	os.Unsetenv("MX_UNSET_494")
	s := newServerForTest(t, nil)
	if rec := postRestore(s, "auth:\n  method: none\nserver:\n  title: ${MX_UNSET_494}\n"); rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body.String())
	}
	for _, e := range logging.Buffer().Recent(50) {
		if e.Level == "warn" && strings.Contains(e.Attrs["missing"], "MX_UNSET_494") {
			return
		}
	}
	t.Error("no warning naming MX_UNSET_494 in the log buffer")
}

func oidcDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": srv.URL, "authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint": srv.URL + "/token", "userinfo_endpoint": srv.URL + "/userinfo",
			"jwks_uri": srv.URL + "/jwks",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Section 9: a restore whose OIDC provider cannot be built clears the
// provider from the previous config instead of keeping it (fail closed).
func TestConfigRestore_OIDCDiscoveryFailureClearsProvider(t *testing.T) {
	s := newServerForTest(t, nil)
	idp := oidcDiscoveryServer(t)
	prior := config.OIDCConfig{Enabled: true, IssuerURL: idp.URL, ClientID: "old", RedirectURL: "http://localhost:8080/api/auth/oidc/callback"}
	if err := s.authHandler.ReplaceOIDCProvider(context.Background(), &prior, ""); err != nil {
		t.Fatalf("install prior provider: %v", err)
	}
	t.Cleanup(func() { _ = s.authHandler.CloseOIDC() })
	if rec := get(t, s, "/api/auth/oidc/login"); rec.Code != http.StatusFound {
		t.Fatalf("prior provider login = %d, want 302", rec.Code)
	}

	backup := builtinBackup(t, "") + `  oidc:
    enabled: true
    issuer_url: http://127.0.0.1:1
    client_id: muximux
    client_secret: secret
    redirect_url: http://localhost:8080/api/auth/oidc/callback
`
	if rec := postRestore(s, backup); rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, s, "/api/auth/oidc/login"); rec.Code == http.StatusFound {
		t.Fatalf("OIDC login still redirects to %q after a failed provider rebuild", rec.Header().Get("Location"))
	}
}

func TestSetupAuth_UsesConfigHelpers(t *testing.T) {
	users := []config.UserConfig{{
		Username: "u", PasswordHash: "h", Role: "admin",
		Email: "u@example.test", DisplayName: "U", Groups: []string{"g1", "g2"},
	}}
	got := usersFromConfig(users)
	want := []auth.UserConfig{{
		Username: "u", PasswordHash: "h", Role: "admin",
		Email: "u@example.test", DisplayName: "U", Groups: []string{"g1", "g2"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("usersFromConfig = %+v, want %+v", got, want)
	}
	if len(usersFromConfig(nil)) != 0 {
		t.Error("usersFromConfig(nil) not empty")
	}

	cfg := &config.Config{}
	cfg.Server.BasePath = "muximux/"
	cfg.Auth.Method = "forward_auth"
	cfg.Auth.TrustedProxies = []string{"10.0.0.0/8"}
	cfg.Auth.APIKeyHash = "sha256:abc"
	cfg.Auth.Headers = map[string]string{"user": "X-User"}
	cfg.Auth.ForwardAuthAdminGroups = []string{"ops"}
	ac := authConfigFromConfig(cfg)
	if ac.Method != auth.AuthMethodForwardAuth || ac.APIKeyHash != "sha256:abc" ||
		!reflect.DeepEqual(ac.TrustedProxies, cfg.Auth.TrustedProxies) ||
		!reflect.DeepEqual(ac.ForwardAuthAdminGroups, cfg.Auth.ForwardAuthAdminGroups) ||
		ac.Headers != auth.ForwardAuthHeadersFromMap(cfg.Auth.Headers) {
		t.Errorf("authConfigFromConfig = %+v", ac)
	}
	if ac.BasePath != cfg.Server.NormalizedBasePath() || ac.BasePath != "/muximux" {
		t.Errorf("base path = %q, want normalised /muximux", ac.BasePath)
	}
	if !reflect.DeepEqual(ac.BypassRules, defaultBypassRules) {
		t.Error("authConfigFromConfig does not use defaultBypassRules")
	}
}

// The setup rollback rebuilds the user store and the middleware snapshot
// through the shared helpers when the config cannot be saved. The prior
// state carries forward-auth fields, an API key hash and a base path, so
// both arms show every field coming back.
func TestHandleSetup_RollbackOnSaveFailure(t *testing.T) {
	arms := []struct{ name, body string }{
		{"builtin", `{"method":"builtin","username":"owner","password":"long-enough-pw"}`},
		{"forward_auth", `{"method":"forward_auth","trusted_proxies":["192.168.0.0/16"],` +
			`"headers":{"user":"X-New-User"},"forward_auth_admin_groups":["new-admins"]}`},
	}
	for _, arm := range arms {
		t.Run(arm.name, func(t *testing.T) {
			s := newServerForTest(t, func(cfg *config.Config) {
				// Method stays none so setup is pending; the
				// forward-auth fields are leftovers the rollback
				// must still put back.
				cfg.Auth.TrustedProxies = []string{"10.0.0.0/8"}
				cfg.Auth.Headers = map[string]string{"user": "X-Prior-User"}
				cfg.Auth.ForwardAuthAdminGroups = []string{"prior-admins"}
				cfg.Auth.APIKeyHash = "sha256:prior"
				cfg.Server.BasePath = "/prior"
			})
			want := s.authMiddleware.Config()
			if want.BasePath != "/prior" || want.APIKeyHash != "sha256:prior" {
				t.Fatalf("fixture snapshot = %+v", want)
			}
			s.configPath = t.TempDir() // a directory: Save cannot rename over it

			req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(arm.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(setupTokenHeader, s.setupToken)
			rec := httptest.NewRecorder()
			s.handleSetup(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("setup = %d %s, want 500", rec.Code, rec.Body.String())
			}
			if got := s.authMiddleware.Config(); !reflect.DeepEqual(got, want) {
				t.Errorf("middleware after rollback = %+v\nwant %+v", got, want)
			}
			if n := len(s.userStore.List()); n != 0 {
				t.Errorf("user store holds %d users after rollback", n)
			}
			a := s.config.Auth
			if a.Method != "none" || len(a.Users) != 0 ||
				!reflect.DeepEqual(a.TrustedProxies, []string{"10.0.0.0/8"}) ||
				!reflect.DeepEqual(a.ForwardAuthAdminGroups, []string{"prior-admins"}) ||
				a.Headers["user"] != "X-Prior-User" {
				t.Errorf("config not rolled back: %+v", a)
			}
			if s.sessionStore.Count() != 0 {
				t.Error("setup session kept after rollback")
			}
			if !s.needsSetup.Load() {
				t.Error("setup no longer pending after a failed save")
			}
		})
	}
}

// A restore whose save fails leaves every piece of live auth state alone:
// sessions, users, the middleware and the OIDC provider.
func TestHandleConfigRestore_SaveFailure(t *testing.T) {
	s := newServerForTest(t, nil)
	idp := oidcDiscoveryServer(t)
	prior := config.OIDCConfig{Enabled: true, IssuerURL: idp.URL, ClientID: "old", RedirectURL: "http://localhost:8080/api/auth/oidc/callback"}
	if err := s.authHandler.ReplaceOIDCProvider(context.Background(), &prior, ""); err != nil {
		t.Fatalf("install provider: %v", err)
	}
	t.Cleanup(func() { _ = s.authHandler.CloseOIDC() })
	sess, err := s.sessionStore.Create("someone", "someone", "user")
	if err != nil {
		t.Fatal(err)
	}
	gen := s.sessionStore.Generation()
	wantAuth := s.authMiddleware.Config()
	s.configPath = t.TempDir() // a directory: Save cannot rename over it

	if rec := postRestore(s, builtinBackup(t, "")); rec.Code != http.StatusInternalServerError {
		t.Fatalf("restore = %d %s, want 500", rec.Code, rec.Body.String())
	}
	if s.sessionStore.Get(sess.ID) == nil || s.sessionStore.Generation() != gen {
		t.Error("sessions touched by a failed restore")
	}
	if n := len(s.userStore.List()); n != 0 {
		t.Errorf("user store holds %d users after a failed restore", n)
	}
	if got := s.authMiddleware.Config(); !reflect.DeepEqual(got, wantAuth) || got.Method != auth.AuthMethodNone {
		t.Errorf("middleware changed by a failed restore: %+v", got)
	}
	if rec := get(t, s, "/api/auth/oidc/login"); rec.Code != http.StatusFound {
		t.Errorf("OIDC login = %d after a failed restore, want the old provider's 302", rec.Code)
	}
	if !s.needsSetup.Load() {
		t.Error("setup no longer pending after a failed restore")
	}
}

// S-53: before setup an anonymous caller without the setup token must not
// reach the admin auth endpoints, which run as the virtual admin while auth
// is none. The wizard's own calls still work.
func TestPreSetup_AdminAuthEndpointsRefused(t *testing.T) {
	s := newServerForTest(t, nil)
	if !s.needsSetup.Load() {
		t.Fatal("expected pre-setup state")
	}
	refused := []struct{ method, path, body string }{
		{http.MethodPost, "/api/auth/users", `{"username":"mallory","password":"password123","role":"admin"}`},
		{http.MethodPost, "/api/auth/api-key", ""},
		{http.MethodPut, "/api/auth/method", `{"method":"none"}`},
		{http.MethodGet, "/api/auth/settings/oidc", ""},
		{http.MethodPut, "/api/auth/settings/oidc", `{"issuer_url":"https://evil.example"}`},
		{http.MethodPost, "/api/auth/settings/oidc/test", `{"issuer_url":"https://evil.example"}`},
	}
	for _, c := range refused {
		rec := doJSON(s, c.method, c.path, c.body, nil, true)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "setup_required") {
			t.Errorf("%s %s: got %d %s, want 503 setup_required", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	s.configMu.RLock()
	users, keyHash := len(s.config.Auth.Users), s.config.Auth.APIKeyHash
	s.configMu.RUnlock()
	if users != 0 || keyHash != "" {
		t.Fatalf("pre-setup state changed: users=%d apiKeyHash=%q", users, keyHash)
	}

	// The endpoints the login page and the wizard use are still reachable.
	if rec := get(t, s, "/api/auth/status"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"setup_required":true`) {
		t.Errorf("status: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, s, "/api/auth/me"); rec.Code == http.StatusServiceUnavailable {
		t.Errorf("me refused before setup: %d", rec.Code)
	}
	if rec := doJSON(s, http.MethodPost, "/api/auth/logout", "", nil, true); rec.Code == http.StatusServiceUnavailable {
		t.Errorf("logout refused before setup: %d", rec.Code)
	}
	if rec := doJSON(s, http.MethodPost, "/api/auth/login", `{"username":"x","password":"y"}`, nil, true); rec.Code == http.StatusServiceUnavailable {
		t.Errorf("login refused before setup: %d", rec.Code)
	}
	if rec := get(t, s, "/api/auth/oidc/login"); rec.Code == http.StatusServiceUnavailable {
		t.Errorf("oidc login refused before setup: %d", rec.Code)
	}

	// Setup with the token completes, after which the admin endpoints are
	// behind normal authentication.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"method":"builtin","username":"owner","password":"correct horse battery"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set(setupTokenHeader, s.setupToken)
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(s, http.MethodPost, "/api/auth/api-key", "", nil, true); rec.Code != http.StatusUnauthorized {
		t.Errorf("api-key after setup without a session: got %d, want 401", rec.Code)
	}
}

// Before setup an anonymous caller is the virtual admin, so the log viewer
// and update check must stay closed; the wizard's theme and icon reads stay
// open.
func TestPreSetup_LogsAndSystemEndpointsRefused(t *testing.T) {
	s := newServerForTest(t, nil)
	if !s.needsSetup.Load() {
		t.Fatal("expected pre-setup state")
	}
	for _, p := range []string{"/api/logs/recent", "/api/logs/recent?limit=1000", "/api/system/updates", "/api/system/info"} {
		rec := get(t, s, p)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "setup_required") {
			t.Errorf("GET %s = %d %q, want 503 setup_required", p, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), s.setupToken) {
			t.Errorf("GET %s leaked the setup token", p)
		}
	}
	for _, p := range []string{"/api/themes", "/api/icons/custom"} {
		if rec := get(t, s, p); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}
}

// The setup token is printed to stdout for the operator but never reaches
// the log ring buffer (served by the log viewer) or the log file.
func TestSetupToken_ConsoleOnly(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "muximux.log")
	if err := logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout", LogFile: logFile}); err != nil {
		t.Fatalf("init logging: %v", err)
	}
	t.Cleanup(func() {
		logging.Close()
		_ = logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout"})
	})

	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = stdoutFile
	s := newServerForTest(t, nil)
	os.Stdout = origStdout
	_ = stdoutFile.Close()

	tok := s.setupToken
	if tok == "" {
		t.Fatal("expected a setup token before setup")
	}
	stdout, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stdout), "token="+tok) {
		t.Errorf("setup token not printed to stdout: %q", stdout)
	}

	sawNotice := false
	for _, e := range logging.Buffer().Recent(1000) {
		if strings.Contains(e.Message, tok) {
			t.Errorf("setup token in log buffer message: %+v", e)
		}
		for k, v := range e.Attrs {
			if strings.Contains(v, tok) {
				t.Errorf("setup token in log buffer attr %s: %+v", k, e)
			}
		}
		if strings.Contains(e.Message, "Setup required") {
			sawNotice = true
		}
	}
	if !sawNotice {
		t.Error("expected a setup-required notice in the log buffer")
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), tok) {
		t.Error("setup token written to the log file")
	}
	if !strings.Contains(string(data), "Setup required") {
		t.Error("expected the setup-required notice in the log file")
	}
}

// A setup token that an earlier release wrote into muximux.log is replaced
// at startup: the new one is persisted and the old one no longer completes
// setup.
func TestSetupToken_RotatedWhenFoundInOldLog(t *testing.T) {
	const oldTok = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, setupTokenFilename), []byte(oldTok+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(dataDir, "muximux.log")
	legacy := `time=2026-03-05T10:00:00.000+01:00 level=INFO msg="Generated new setup token; present it via X-Setup-Token to complete setup or restore" source=server token=` + oldTok + "\n"
	if err := os.WriteFile(logFile, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout", LogFile: logFile}); err != nil {
		t.Fatalf("init logging: %v", err)
	}
	t.Cleanup(func() {
		logging.Close()
		_ = logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout"})
	})

	configPath := filepath.Join(dataDir, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.Server.Listen = "127.0.0.1:0"
	s, err := New(cfg, configPath, dataDir, "test", "abcdef0", "2026-01-01")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if s.setupToken == "" || s.setupToken == oldTok {
		t.Fatalf("setup token not rotated: %q", s.setupToken)
	}
	data, err := os.ReadFile(filepath.Join(dataDir, setupTokenFilename))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != s.setupToken {
		t.Errorf("persisted token %q, want the new token %q", got, s.setupToken)
	}

	setup := func(tok string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"method":"builtin","username":"owner","password":"correct horse battery"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set(setupTokenHeader, tok)
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := setup(oldTok); rec.Code != http.StatusUnauthorized {
		t.Fatalf("setup with the leaked token = %d %s, want 401", rec.Code, rec.Body.String())
	}
	if rec := setup(s.setupToken); rec.Code != http.StatusOK {
		t.Fatalf("setup with the new token = %d %s, want 200", rec.Code, rec.Body.String())
	}
}

// A token that is not in the log is reused across restarts.
func TestSetupToken_ReusedWhenNotInLog(t *testing.T) {
	dataDir := t.TempDir()
	logFile := filepath.Join(dataDir, "muximux.log")
	if err := os.WriteFile(logFile, []byte("time=2026-03-05T10:00:00.000+01:00 level=INFO msg=hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout", LogFile: logFile}); err != nil {
		t.Fatalf("init logging: %v", err)
	}
	t.Cleanup(func() {
		logging.Close()
		_ = logging.Init(logging.Config{Level: logging.LevelInfo, Format: "text", Output: "stdout"})
	})
	first := &Server{dataDir: dataDir}
	if err := first.ensureSetupToken(); err != nil {
		t.Fatal(err)
	}
	second := &Server{dataDir: dataDir}
	if err := second.ensureSetupToken(); err != nil {
		t.Fatal(err)
	}
	if second.setupToken != first.setupToken {
		t.Errorf("token rotated without a leak: %q -> %q", first.setupToken, second.setupToken)
	}
}

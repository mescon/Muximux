package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/mescon/muximux/v3/internal/config"
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

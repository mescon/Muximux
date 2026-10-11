package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// A fresh container (no config.yaml) with MUXIMUX_AUTH_METHOD=oidc starts
// configured: no setup wizard, no setup token, and OIDC enabled.
func TestNew_EnvOIDCSkipsSetup(t *testing.T) {
	for _, n := range config.AuthEnvVarNames() {
		t.Setenv(n, "")
	}
	t.Setenv(config.EnvAuthMethod, "oidc")
	t.Setenv(config.EnvOIDCIssuerURL, "https://id.example.com")
	t.Setenv(config.EnvOIDCClientID, "muximux")
	t.Setenv(config.EnvOIDCRedirectURL, "http://localhost:8080/api/auth/oidc/callback")

	s := newServerForTest(t, nil)
	t.Cleanup(func() { _ = s.authHandler.CloseOIDC() })
	if s.needsSetup.Load() {
		t.Error("needsSetup = true with MUXIMUX_AUTH_METHOD=oidc")
	}
	if s.setupToken != "" {
		t.Error("a setup token was created")
	}
	if _, err := os.Stat(filepath.Join(s.dataDir, setupTokenFilename)); !os.IsNotExist(err) {
		t.Errorf("setup token file exists: %v", err)
	}
	// New builds the OIDC provider whenever OIDC is enabled.
	if s.config.Auth.Method != "oidc" || !s.config.Auth.OIDC.Enabled {
		t.Errorf("auth = %q enabled %v, want oidc enabled", s.config.Auth.Method, s.config.Auth.OIDC.Enabled)
	}
}

// MUXIMUX_AUTH_METHOD=builtin with no users still needs setup, so the
// first admin can be created.
func TestNew_EnvBuiltinWithoutUsersNeedsSetup(t *testing.T) {
	for _, n := range config.AuthEnvVarNames() {
		t.Setenv(n, "")
	}
	t.Setenv(config.EnvAuthMethod, "builtin")

	s := newServerForTest(t, nil)
	if !s.needsSetup.Load() || s.setupToken == "" {
		t.Errorf("needsSetup = %v, token set = %v; want setup with a token", s.needsSetup.Load(), s.setupToken != "")
	}
}

// postSetup sends POST /api/auth/setup with the server's token.
func postSetup(s *Server, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(setupTokenHeader, s.setupToken)
	rec := httptest.NewRecorder()
	s.handleSetup(rec, req)
	return rec
}

// The wizard is locked to MUXIMUX_AUTH_METHOD: another method is refused
// with 409 naming the variable, and builtin with no users still creates
// the first admin.
func TestHandleSetup_LockedToEnvMethod(t *testing.T) {
	for _, n := range config.AuthEnvVarNames() {
		t.Setenv(n, "")
	}
	t.Setenv(config.EnvAuthMethod, "builtin")
	s := newServerForTest(t, nil)
	if !s.needsSetup.Load() {
		t.Fatal("env builtin without users should need setup")
	}

	for _, body := range []string{`{"method":"none"}`, `{"method":"forward_auth","trusted_proxies":["10.0.0.1/32"]}`} {
		rec := postSetup(s, body)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), config.EnvAuthMethod) {
			t.Errorf("%s: code=%d body=%s", body, rec.Code, rec.Body.String())
		}
	}
	if !s.needsSetup.Load() || s.config.Auth.Method != "builtin" {
		t.Fatal("a refused setup changed the state")
	}

	rec := postSetup(s, `{"method":"builtin","username":"admin","password":"correct-horse-battery"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("builtin setup: code=%d body=%s", rec.Code, rec.Body.String())
	}
	if s.needsSetup.Load() || len(s.config.Auth.Users) != 1 || s.config.Auth.Method != "builtin" {
		t.Errorf("after setup: needsSetup=%v users=%d method=%q", s.needsSetup.Load(), len(s.config.Auth.Users), s.config.Auth.Method)
	}
}

// Without MUXIMUX_AUTH_METHOD the wizard may pick any method.
func TestSetupMethodEnvConflict_NoEnv(t *testing.T) {
	s := &Server{config: defaultTestConfig()}
	if msg := s.setupMethodEnvConflict("forward_auth"); msg != "" {
		t.Errorf("conflict without env: %q", msg)
	}
}

// Completing the wizard under MUXIMUX_AUTH_METHOD=builtin persists
// method: builtin with the new admin, so dropping the variable later still
// requires a password instead of starting with no auth and no wizard.
func TestHandleSetup_EnvBuiltinPersistsMethod(t *testing.T) {
	for _, n := range config.AuthEnvVarNames() {
		t.Setenv(n, "")
	}
	t.Setenv(config.EnvAuthMethod, "builtin")
	s := newServerForTest(t, nil)

	rec := postSetup(s, `{"method":"builtin","username":"admin","password":"correct-horse-battery"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("builtin setup: code=%d body=%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if !strings.Contains(string(data), "method: builtin") {
		t.Errorf("config.yaml does not hold method: builtin:\n%s", data)
	}

	// Drop the variable and start again from the saved file.
	t.Setenv(config.EnvAuthMethod, "")
	cfg, err := config.Load(s.configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.IsOverridden(config.OverrideAuthMethod) {
		t.Fatal("auth.method still overridden without the variable")
	}
	if cfg.Auth.Method != "builtin" || len(cfg.Auth.Users) != 1 || cfg.NeedsSetup() {
		t.Fatalf("reloaded auth: method=%q users=%d needsSetup=%v", cfg.Auth.Method, len(cfg.Auth.Users), cfg.NeedsSetup())
	}
	cfg.Server.Listen = "127.0.0.1:0"
	s2, err := New(cfg, s.configPath, s.dataDir, "test", "abcdef0", "2026-01-01")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s2.needsSetup.Load() {
		t.Error("restart without the variable went back to the setup wizard")
	}
	if rec := get(t, s2, "/api/config"); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/config without a session = %d, want 401", rec.Code)
	}
}

// A setup whose save fails under MUXIMUX_AUTH_METHOD also puts back the
// file value of the method, so a later save still writes the file's own.
func TestHandleSetup_EnvBuiltinRollbackRestoresFileMethod(t *testing.T) {
	for _, n := range config.AuthEnvVarNames() {
		t.Setenv(n, "")
	}
	t.Setenv(config.EnvAuthMethod, "builtin")
	s := newServerForTest(t, nil)
	realPath := s.configPath
	s.configPath = t.TempDir() // a directory: Save cannot rename over it

	rec := postSetup(s, `{"method":"builtin","username":"admin","password":"correct-horse-battery"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("setup = %d %s, want 500", rec.Code, rec.Body.String())
	}
	if s.config.Auth.Method != "builtin" || len(s.config.Auth.Users) != 0 {
		t.Fatalf("live auth after rollback: method=%q users=%d", s.config.Auth.Method, len(s.config.Auth.Users))
	}
	if err := s.config.Save(realPath); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if strings.Contains(string(data), "method: builtin") {
		t.Errorf("rolled-back file value still builtin:\n%s", data)
	}
}

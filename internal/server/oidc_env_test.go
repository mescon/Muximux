package server

import (
	"os"
	"path/filepath"
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

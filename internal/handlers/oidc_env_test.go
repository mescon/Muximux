package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

const envSecretValue = "env-secret-do-not-leak" //nolint:gosec // test value

// envOIDCConfig returns a test config with OIDC applied from a fake
// environment, the client secret read from a fake *_FILE.
func envOIDCConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := createTestConfig()
	cfg.Auth.Method = "none"
	env := map[string]string{
		config.EnvAuthMethod:                 "oidc",
		config.EnvOIDCIssuerURL:              "https://env-idp.example.com",
		config.EnvOIDCClientID:               "env-client",
		config.EnvOIDCClientSecret + "_FILE": "/run/secrets/oidc",
		config.EnvOIDCAdminGroups:            "env-admins",
	}
	getenv := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	readFile := func(string) ([]byte, error) { return []byte(envSecretValue + "\n"), nil }
	if err := config.ApplyAuthEnv(cfg, getenv, readFile); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Export writes the live values (issue #513 decision 3): env-provided OIDC
// settings appear in the backup. The client secret is still blanked, as for
// every export.
func TestExportConfig_WritesLiveEnvValues(t *testing.T) {
	cfg := envOIDCConfig(t)
	handler := NewAPIHandler(cfg, "", &sync.RWMutex{})
	w := httptest.NewRecorder()
	handler.ExportConfig(w, httptest.NewRequest(http.MethodGet, "/api/config/export", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if strings.Contains(w.Body.String(), envSecretValue) {
		t.Fatal("export contains the client secret")
	}
	var exported config.Config
	if err := yaml.Unmarshal(w.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	o := exported.Auth.OIDC
	if exported.Auth.Method != "oidc" || !o.Enabled || o.IssuerURL != "https://env-idp.example.com" ||
		o.ClientID != "env-client" || len(o.AdminGroups) != 1 || o.AdminGroups[0] != "env-admins" {
		t.Errorf("export does not carry the live env values: %+v", exported.Auth)
	}
	if cfg.Auth.OIDC.ClientSecret != envSecretValue {
		t.Error("export blanked the live secret")
	}
}

// GET /api/config reports env-set fields by variable name only, and never
// carries the secret.
func TestGetConfig_AuthEnvOverridesNamesOnly(t *testing.T) {
	cfg := envOIDCConfig(t)
	resp := getConfigAs(t, cfg, auth.RoleAdmin)
	var overrides map[string]string
	if err := json.Unmarshal(resp["env_overrides"], &overrides); err != nil {
		t.Fatalf("env_overrides: %v", err)
	}
	want := map[string]string{
		"auth.method":             config.EnvAuthMethod,
		"auth.oidc.enabled":       config.EnvAuthMethod,
		"auth.oidc.issuer_url":    config.EnvOIDCIssuerURL,
		"auth.oidc.client_id":     config.EnvOIDCClientID,
		"auth.oidc.client_secret": config.EnvOIDCClientSecret + "_FILE",
		"auth.oidc.admin_groups":  config.EnvOIDCAdminGroups,
	}
	for k, v := range want {
		if overrides[k] != v {
			t.Errorf("env_overrides[%s] = %q, want %q", k, overrides[k], v)
		}
	}
	raw, _ := json.Marshal(resp)
	if strings.Contains(string(raw), envSecretValue) {
		t.Error("GET /api/config carries the client secret")
	}
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadWithEnv(t *testing.T, yaml string, env map[string]string) (*Config, string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, path
}

func TestSave_KeepsEnvReferences(t *testing.T) {
	cfg, path := loadWithEnv(t, `server:
  title: Dash
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://${IDP_HOST}/realms/home
    client_id: muximux
    client_secret: ${OIDC_CLIENT_SECRET}
    redirect_url: https://dash.example.com/api/auth/oidc/callback
apps:
  - name: Sonarr
    url: http://sonarr:8989
    proxy_headers:
      X-Api-Key: ${SONARR_KEY}
`, map[string]string{"IDP_HOST": "idp.example.com", "OIDC_CLIENT_SECRET": "s3cret", "SONARR_KEY": "k1"})

	if cfg.Auth.OIDC.ClientSecret != "s3cret" {
		t.Fatalf("expansion broken: %q", cfg.Auth.OIDC.ClientSecret)
	}
	cfg.Server.Title = "Changed" // an unrelated edit
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	for _, want := range []string{"${OIDC_CLIENT_SECRET}", "https://${IDP_HOST}/realms/home", "${SONARR_KEY}"} {
		if !strings.Contains(s, want) {
			t.Errorf("saved file lost %q:\n%s", want, s)
		}
	}
	for _, leaked := range []string{"s3cret", "k1"} {
		if strings.Contains(s, leaked) {
			t.Errorf("saved file contains the expanded secret %q", leaked)
		}
	}
}

func TestSave_ChangedValueReplacesReference(t *testing.T) {
	cfg, path := loadWithEnv(t, "auth:\n  oidc:\n    client_secret: ${OIDC_CLIENT_SECRET}\n",
		map[string]string{"OIDC_CLIENT_SECRET": "old"})
	cfg.Auth.OIDC.ClientSecret = "typed-in-ui"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "typed-in-ui") || strings.Contains(string(out), "${OIDC_CLIENT_SECRET}") {
		t.Errorf("changed value should replace the reference:\n%s", out)
	}
}

func TestSave_SequenceItemsMatchedByName(t *testing.T) {
	cfg, path := loadWithEnv(t, `apps:
  - name: A
    url: http://a
  - name: B
    url: http://b
    proxy_headers:
      X-Key: ${B_KEY}
`, map[string]string{"B_KEY": "bk"})
	cfg.Apps[0], cfg.Apps[1] = cfg.Apps[1], cfg.Apps[0] // reorder
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "${B_KEY}") || strings.Contains(string(out), "bk") {
		t.Errorf("reference lost after reorder:\n%s", out)
	}
}

func TestEnvRefVar(t *testing.T) {
	cfg, _ := loadWithEnv(t, "auth:\n  oidc:\n    client_secret: ${OIDC_CLIENT_SECRET}\n    issuer_url: https://${H}/x\n",
		map[string]string{"OIDC_CLIENT_SECRET": "s", "H": "h"})
	if v, ok := cfg.EnvRefVar("auth", "oidc", "client_secret"); !ok || v != "OIDC_CLIENT_SECRET" {
		t.Errorf("client_secret ref = %q, %v", v, ok)
	}
	if _, ok := cfg.EnvRefVar("auth", "oidc", "issuer_url"); ok {
		t.Error("a partial reference is not a whole-value variable")
	}
	if _, ok := cfg.EnvRefVar("auth", "oidc", "client_id"); ok {
		t.Error("plain field reported as env-sourced")
	}
}

func TestSave_NoRefsForProgrammaticConfig(t *testing.T) {
	cfg := defaultConfig()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("saved file does not reload: %v", err)
	}
}

func TestLoad_FlowCollectionWithEnvRef(t *testing.T) {
	cfg, path := loadWithEnv(t, "auth:\n  oidc: {client_secret: ${FS}}\n", map[string]string{"FS": "flowsecret"})
	if cfg.Auth.OIDC.ClientSecret != "flowsecret" {
		t.Fatalf("not expanded: %q", cfg.Auth.OIDC.ClientSecret)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
}

func TestSave_EnvRefInItemName(t *testing.T) {
	cfg, path := loadWithEnv(t, `apps:
  - name: ${APP_NAME}
    url: http://a
    proxy_headers:
      X-Key: ${APP_KEY}
`, map[string]string{"APP_NAME": "Sonarr", "APP_KEY": "plainkey"})
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "${APP_KEY}") || strings.Contains(string(out), "plainkey") {
		t.Errorf("secret reference lost:\n%s", out)
	}
}

func TestSave_EnvRefInNonStringField(t *testing.T) {
	cfg, path := loadWithEnv(t, "auth:\n  oidc:\n    enabled: ${OIDC_ON}\n", map[string]string{"OIDC_ON": "true"})
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "enabled: ${OIDC_ON}") || strings.Contains(string(out), "!!") {
		t.Errorf("typed reference not clean:\n%s", out)
	}
}

func TestSave_IdentityPlaceholderAndMissingVarStable(t *testing.T) {
	cfg, path := loadWithEnv(t, `apps:
  - name: A
    url: http://a
    proxy_headers:
      X-User: ${user}
      X-Missing: ${NOT_SET_ANYWHERE_XYZ}
`, nil)
	for i := 0; i < 2; i++ {
		if err := cfg.Save(path); err != nil {
			t.Fatal(err)
		}
		out, _ := os.ReadFile(path)
		if !strings.Contains(string(out), "${user}") || !strings.Contains(string(out), "${NOT_SET_ANYWHERE_XYZ}") {
			t.Fatalf("value changed on save %d:\n%s", i, out)
		}
		var err error
		if cfg, err = Load(path); err != nil {
			t.Fatal(err)
		}
	}
}

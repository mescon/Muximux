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

func saveAndRead(t *testing.T, cfg *Config, path string) string {
	t.Helper()
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

const twoKeyedApps = `apps:
  - name: Sonarr
    url: http://sonarr
    proxy_headers:
      X-Api-Key: ${SONARR_KEY}
  - name: Radarr
    url: http://radarr
    proxy_headers:
      X-Api-Key: ${RADARR_KEY}
`

func TestSave_RenamedItemKeepsReference(t *testing.T) {
	cfg, path := loadWithEnv(t, twoKeyedApps, map[string]string{"SONARR_KEY": "sonarr-secret", "RADARR_KEY": "radarr-secret"})
	cfg.Apps[0].Name = "Sonarr 4K"
	s := saveAndRead(t, cfg, path)
	if !strings.Contains(s, "${SONARR_KEY}") || !strings.Contains(s, "${RADARR_KEY}") {
		t.Errorf("rename lost a reference:\n%s", s)
	}
	if strings.Contains(s, "sonarr-secret") || strings.Contains(s, "radarr-secret") {
		t.Errorf("rename leaked an expanded secret:\n%s", s)
	}
}

func TestSave_RenamedItemWithNewValueWritesValue(t *testing.T) {
	cfg, path := loadWithEnv(t, twoKeyedApps, map[string]string{"SONARR_KEY": "sonarr-secret", "RADARR_KEY": "radarr-secret"})
	cfg.Apps[0].Name = "Sonarr 4K"
	cfg.Apps[0].ProxyHeaders["X-Api-Key"] = "typed-in-ui"
	s := saveAndRead(t, cfg, path)
	if !strings.Contains(s, "typed-in-ui") || strings.Contains(s, "${SONARR_KEY}") {
		t.Errorf("changed value should replace the reference:\n%s", s)
	}
	if !strings.Contains(s, "${RADARR_KEY}") {
		t.Errorf("untouched sibling lost its reference:\n%s", s)
	}
}

func TestSave_SwappedItemsDoNotCrossAssignReferences(t *testing.T) {
	cfg, path := loadWithEnv(t, twoKeyedApps, map[string]string{"SONARR_KEY": "sonarr-secret", "RADARR_KEY": "radarr-secret"})
	cfg.Apps[0], cfg.Apps[1] = cfg.Apps[1], cfg.Apps[0]
	s := saveAndRead(t, cfg, path)
	if strings.Contains(s, "sonarr-secret") || strings.Contains(s, "radarr-secret") {
		t.Errorf("reorder leaked an expanded secret:\n%s", s)
	}
	radarr := strings.Index(s, "name: Radarr")
	sonarr := strings.Index(s, "name: Sonarr")
	rKey := strings.Index(s, "${RADARR_KEY}")
	sKey := strings.Index(s, "${SONARR_KEY}")
	inOrder := radarr >= 0 && radarr < rKey && rKey < sonarr && sonarr < sKey
	if !inOrder {
		t.Errorf("references not attached to their own items:\n%s", s)
	}

	// Swapped and renamed: the positional fallback lands on the other
	// item, whose value differs, so neither reference is misapplied.
	cfg2, path2 := loadWithEnv(t, twoKeyedApps, map[string]string{"SONARR_KEY": "sonarr-secret", "RADARR_KEY": "radarr-secret"})
	cfg2.Apps[0], cfg2.Apps[1] = cfg2.Apps[1], cfg2.Apps[0]
	cfg2.Apps[0].Name = "Radarr HD"
	cfg2.Apps[1].Name = "Sonarr HD"
	s2 := saveAndRead(t, cfg2, path2)
	if strings.Contains(s2, "${SONARR_KEY}") || strings.Contains(s2, "${RADARR_KEY}") {
		// Cross-assigned references would expand to the wrong secret.
		t.Errorf("reference applied to a different item's value:\n%s", s2)
	}
}

func TestSave_DuplicateItemNamesKeepTheirOwnReferences(t *testing.T) {
	cfg, path := loadWithEnv(t, `apps:
  - name: Media
    url: http://one
    proxy_headers:
      X-Api-Key: ${ONE_KEY}
  - name: Media
    url: http://two
    proxy_headers:
      X-Api-Key: ${TWO_KEY}
`, map[string]string{"ONE_KEY": "one-secret", "TWO_KEY": "two-secret"})
	cfg.Server.Title = "Changed"
	s := saveAndRead(t, cfg, path)
	if strings.Contains(s, "one-secret") || strings.Contains(s, "two-secret") {
		t.Errorf("duplicate names leaked an expanded secret:\n%s", s)
	}
	one := strings.Index(s, "${ONE_KEY}")
	two := strings.Index(s, "${TWO_KEY}")
	if one < 0 || two < 0 || one > two {
		t.Errorf("duplicate-name references misplaced:\n%s", s)
	}
}

func TestOIDCClientSecretInPlaintext(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		env  map[string]string
		want bool
	}{
		{"env reference", "auth:\n  oidc:\n    enabled: true\n    client_secret: ${OIDC_CLIENT_SECRET}\n",
			map[string]string{"OIDC_CLIENT_SECRET": "from-env"}, false},
		{"literal secret", "auth:\n  oidc:\n    enabled: true\n    client_secret: literal-secret\n", nil, true},
		{"unresolved reference", "auth:\n  oidc:\n    enabled: true\n    client_secret: ${MUXIMUX_TEST_UNSET_SECRET}\n", nil, false},
		{"oidc disabled", "auth:\n  oidc:\n    enabled: false\n    client_secret: literal-secret\n", nil, false},
		{"no secret", "auth:\n  oidc:\n    enabled: true\n", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := loadWithEnv(t, tc.yaml, tc.env)
			if got := cfg.OIDCClientSecretInPlaintext(); got != tc.want {
				t.Errorf("OIDCClientSecretInPlaintext() = %v, want %v (secret %q)", got, tc.want, cfg.Auth.OIDC.ClientSecret)
			}
		})
	}
}

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

const lockSecret = "env-lock-secret-do-not-leak" //nolint:gosec // test value

// clearAuthEnvVars unsets every auth variable for the test, so only what
// the test sets reaches config.Load.
func clearAuthEnvVars(t *testing.T) {
	t.Helper()
	for _, n := range config.AuthEnvVarNames() {
		t.Setenv(n, "")
		_ = os.Unsetenv(n)
	}
}

// envLockedOIDC loads a config whose OIDC settings come mostly from the
// environment, issuer pointing at a mock discovery server. Claims other
// than username stay editable.
func envLockedOIDC(t *testing.T) (cfg *config.Config, path, issuer string) {
	t.Helper()
	clearAuthEnvVars(t)
	idp := mockOIDCDiscoveryServer(t)
	cfg, path = loadConfigWithEnv(t, "auth:\n  method: none\n  oidc:\n    username_claim: ${FILE_CLAIM}\n    client_id: ${FILE_CID}\n",
		map[string]string{
			"FILE_CLAIM":                        "preferred_username",
			"FILE_CID":                          "file-client",
			config.EnvAuthMethod:                "oidc",
			config.EnvOIDCIssuerURL:             idp.URL,
			config.EnvOIDCClientID:              "env-client",
			config.EnvOIDCClientSecret:          lockSecret,
			config.EnvOIDCRedirectURL:           "https://dash.example.com/api/auth/oidc/callback",
			config.EnvOIDCScopes:                "openid env",
			config.EnvOIDCAdminGroups:           "env-admins",
			config.EnvOIDCProviderLogout:        "false",
			config.EnvOIDCAutoRedirect:          "true",
			config.EnvOIDCDisableLocalLogin:     "false",
			config.EnvOIDCPostLogoutRedirectURL: "https://dash.example.com/bye",
		})
	return cfg, path, idp.URL
}

func TestGetOIDCSettings_EnvOverridesLockEveryField(t *testing.T) {
	cfg, path, _ := envLockedOIDC(t)
	h := newOIDCSettingsHandler(cfg, path)
	rec := httptest.NewRecorder()
	h.GetOIDCSettings(rec, httptest.NewRequest(http.MethodGet, "/api/auth/settings/oidc", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), lockSecret) {
		t.Fatal("settings response carries the client secret")
	}
	var got oidcSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"enabled":                  config.EnvAuthMethod,
		"issuer_url":               config.EnvOIDCIssuerURL,
		"client_id":                config.EnvOIDCClientID, // override wins over the ${FILE_CID} reference
		"client_secret":            config.EnvOIDCClientSecret,
		"redirect_url":             config.EnvOIDCRedirectURL,
		"scopes":                   config.EnvOIDCScopes,
		"admin_groups":             config.EnvOIDCAdminGroups,
		"provider_logout":          config.EnvOIDCProviderLogout,
		"auto_redirect":            config.EnvOIDCAutoRedirect,
		"disable_local_login":      config.EnvOIDCDisableLocalLogin,
		"post_logout_redirect_url": config.EnvOIDCPostLogoutRedirectURL,
		"username_claim":           "FILE_CLAIM", // a ${VAR} reference still locks
	}
	if !reflect.DeepEqual(got.EnvFields, want) {
		t.Errorf("env_fields =\n %v\nwant\n %v", got.EnvFields, want)
	}
	if !got.ClientSecretSet {
		t.Error("client_secret_set = false with an env secret")
	}
}

// Settings keep every env-owned value live, lists and booleans included,
// and apply the fields the environment does not own.
func TestUpdateAuthMethod_KeepsEnvOwnedOIDCFields(t *testing.T) {
	cfg, path, issuer := envLockedOIDC(t)
	h, _ := newOIDCSaveHandler(t, cfg, path)
	rec := putAuthMethod(h, map[string]interface{}{"method": "oidc", "oidc": map[string]interface{}{
		"issuer_url": "https://other.example.com", "client_id": "typed", "client_secret": "typed-secret",
		"redirect_url": "https://other.example.com/cb", "scopes": []string{"openid"}, "admin_groups": []string{"typed"},
		"provider_logout": true, "auto_redirect": false, "disable_local_login": true,
		"post_logout_redirect_url": "https://other.example.com/bye", "username_claim": "typed",
		"email_claim": "mail",
	}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), lockSecret) {
		t.Error("response carries the client secret")
	}
	o := cfg.Auth.OIDC
	if o.IssuerURL != issuer || o.ClientID != "env-client" || o.ClientSecret != lockSecret ||
		o.RedirectURL != "https://dash.example.com/api/auth/oidc/callback" ||
		!reflect.DeepEqual(o.Scopes, []string{"openid", "env"}) || !reflect.DeepEqual(o.AdminGroups, []string{"env-admins"}) ||
		o.ProviderLogout || !o.AutoRedirect || o.DisableLocalLogin || o.PostLogoutRedirectURL != "https://dash.example.com/bye" ||
		o.UsernameClaim != "preferred_username" {
		t.Errorf("an env-owned field changed: %+v", o)
	}
	if o.EmailClaim != "mail" {
		t.Errorf("unlocked email_claim = %q, want mail", o.EmailClaim)
	}
	// Nothing from the environment reaches the file.
	data := readFile(t, path)
	for _, v := range []string{lockSecret, issuer, "env-client", "env-admins"} {
		if strings.Contains(data, v) {
			t.Errorf("config.yaml contains env value %q:\n%s", v, data)
		}
	}
}

// The method is refused (409) while MUXIMUX_AUTH_METHOD owns it; the same
// method may be re-saved.
func TestUpdateAuthMethod_EnvMethodLocked(t *testing.T) {
	clearAuthEnvVars(t)
	cfg, path := loadConfigWithEnv(t, "auth:\n  method: builtin\n", map[string]string{config.EnvAuthMethod: "none"})
	h, mw := newOIDCSaveHandler(t, cfg, path)
	before := readFile(t, path)
	for _, method := range []string{"builtin", "forward_auth", "oidc"} {
		body := map[string]interface{}{"method": method, "trusted_proxies": []string{"10.0.0.1/32"},
			"oidc": map[string]string{"issuer_url": "https://id", "client_id": "c"}}
		rec := putAuthMethod(h, body, nil)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), config.EnvAuthMethod) {
			t.Errorf("%s: code=%d body=%s", method, rec.Code, rec.Body.String())
		}
	}
	if cfg.Auth.Method != "none" || mw.Method() != auth.AuthMethodNone || readFile(t, path) != before {
		t.Error("a refused change altered the config")
	}
	if rec := putAuthMethod(h, map[string]string{"method": "none"}, nil); rec.Code != http.StatusOK {
		t.Errorf("re-saving the env method: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

// MUXIMUX_OIDC_ENABLED=false refuses choosing oidc.
func TestUpdateAuthMethod_EnvOIDCDisabled(t *testing.T) {
	clearAuthEnvVars(t)
	cfg, path := loadConfigWithEnv(t, "auth:\n  method: builtin\n", map[string]string{config.EnvOIDCEnabled: "false"})
	h, _ := newOIDCSaveHandler(t, cfg, path)
	rec := putAuthMethod(h, map[string]interface{}{"method": "oidc",
		"oidc": map[string]string{"issuer_url": "https://id", "client_id": "c"}}, nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), config.EnvOIDCEnabled) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if cfg.Auth.Method != "builtin" || cfg.Auth.OIDC.Enabled {
		t.Errorf("refused change altered the config: %+v", cfg.Auth)
	}
}

// While variables configure OIDC without MUXIMUX_AUTH_METHOD, leaving oidc
// is refused: the next start would fail closed.
func TestUpdateAuthMethod_LeavingEnvConfiguredOIDCRefused(t *testing.T) {
	clearAuthEnvVars(t)
	idp := mockOIDCDiscoveryServer(t)
	cfg, path := loadConfigWithEnv(t, "auth:\n  method: oidc\n  oidc:\n    enabled: true\n    client_id: c\n",
		map[string]string{config.EnvOIDCIssuerURL: idp.URL, config.EnvOIDCClientSecret: lockSecret})
	h, _ := newOIDCSaveHandler(t, cfg, path)
	rec := putAuthMethod(h, map[string]string{"method": "builtin"}, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, config.EnvOIDCIssuerURL) || !strings.Contains(body, config.EnvOIDCClientSecret) {
		t.Errorf("message does not name the variables: %s", body)
	}
	if strings.Contains(body, lockSecret) || strings.Contains(body, idp.URL) {
		t.Errorf("message carries a value: %s", body)
	}
	if cfg.Auth.Method != "oidc" || !cfg.Auth.OIDC.Enabled {
		t.Errorf("refused change altered the config: %+v", cfg.Auth)
	}
}

func TestAuthMethodEnvConflict(t *testing.T) {
	tests := []struct {
		name   string
		file   string
		env    map[string]string
		method string
		want   string // substring; "" means allowed
	}{
		{"no env", "none", nil, "builtin", ""},
		{"same env method", "none", map[string]string{config.EnvAuthMethod: "builtin"}, "builtin", ""},
		{"other env method", "none", map[string]string{config.EnvAuthMethod: "builtin"}, "none", config.EnvAuthMethod},
		{"oidc allowed", "builtin", map[string]string{config.EnvOIDCScopes: "openid"}, "oidc", ""},
		{"oidc env enabled true", "oidc", map[string]string{config.EnvOIDCEnabled: "true"}, "oidc", ""},
		{"leave oidc, enabled true", "oidc", map[string]string{config.EnvOIDCEnabled: "true"}, "none", config.EnvOIDCEnabled},
		{"leave oidc, tuning only", "oidc", map[string]string{config.EnvOIDCAutoRedirect: "true"}, "none", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig()
			cfg.Auth.Method = tt.file
			if tt.file == "oidc" {
				cfg.Auth.OIDC = config.OIDCConfig{Enabled: true, IssuerURL: "https://id", ClientID: "c"}
			}
			getenv := func(k string) (string, bool) { v, ok := tt.env[k]; return v, ok }
			if err := config.ApplyAuthEnv(cfg, getenv, nil); err != nil {
				t.Fatal(err)
			}
			got := authMethodEnvConflict(cfg, tt.method)
			if (tt.want == "") != (got == "") || !strings.Contains(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// GET /api/auth/status names the variable behind the method (for the
// setup wizard), and only when there is one.
func TestAuthStatus_AuthMethodEnv(t *testing.T) {
	cfg := createTestConfig()
	h := newOIDCSettingsHandler(cfg, "")
	status := func() map[string]interface{} {
		rec := httptest.NewRecorder()
		h.AuthStatus(rec, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
		var st map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &st)
		return st
	}
	if _, ok := status()["auth_method_env"]; ok {
		t.Error("auth_method_env present without an override")
	}
	cfg.ApplyOverride(config.OverrideAuthMethod, config.EnvAuthMethod, "builtin")
	st := status()
	if st["auth_method_env"] != config.EnvAuthMethod || st["auth_method"] != "builtin" {
		t.Errorf("status = %v", st)
	}
}

// PUT /api/config carries no auth block: env-owned auth fields keep their
// live values whatever the body says, and the response never holds the
// secret.
func TestSaveConfig_KeepsEnvOwnedAuth(t *testing.T) {
	cfg := envOIDCConfig(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	handler := NewAPIHandler(cfg, path, &sync.RWMutex{})
	update := map[string]interface{}{
		"title": cfg.Server.Title, "navigation": cfg.Navigation, "groups": cfg.Groups,
		"apps": []ClientAppConfig{{Name: "App1", URL: "http://localhost:8080", Enabled: true}},
		"auth": map[string]interface{}{"method": "none", "oidc": map[string]interface{}{
			"enabled": false, "issuer_url": "https://evil.example.com", "client_secret": "typed"}},
	}
	body, _ := json.Marshal(update)
	w := httptest.NewRecorder()
	handler.SaveConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), envSecretValue) {
		t.Error("response carries the client secret")
	}
	o := cfg.Auth.OIDC
	if cfg.Auth.Method != "oidc" || !o.Enabled || o.IssuerURL != "https://env-idp.example.com" || o.ClientSecret != envSecretValue {
		t.Errorf("PUT /api/config changed env-owned auth: %+v", cfg.Auth)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), envSecretValue) || strings.Contains(string(data), "env-idp") {
		t.Errorf("config.yaml carries env values:\n%s", data)
	}
}

// Export blanks the client secret whether it came from the file or the
// environment (part 1 ruling); other values are exported live.
func TestExportConfig_BlanksFileAndEnvSecrets(t *testing.T) {
	for _, fromEnv := range []bool{false, true} {
		t.Run(fmt.Sprintf("env=%v", fromEnv), func(t *testing.T) {
			cfg := createTestConfig()
			cfg.Auth.Method = "oidc"
			cfg.Auth.OIDC = config.OIDCConfig{Enabled: true, IssuerURL: "https://id", ClientID: "c", ClientSecret: "file-secret"}
			if fromEnv {
				cfg.ApplyOverride(config.OverrideOIDCClientSecret, config.EnvOIDCClientSecret, envSecretValue)
			}
			w := httptest.NewRecorder()
			NewAPIHandler(cfg, "", &sync.RWMutex{}).ExportConfig(w, httptest.NewRequest(http.MethodGet, "/api/config/export", nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status %d", w.Code)
			}
			var exported config.Config
			if err := yaml.Unmarshal(w.Body.Bytes(), &exported); err != nil {
				t.Fatal(err)
			}
			if exported.Auth.OIDC.ClientSecret != "" || strings.Contains(w.Body.String(), "file-secret") ||
				strings.Contains(w.Body.String(), envSecretValue) {
				t.Error("export carries a client secret")
			}
			if exported.Auth.OIDC.IssuerURL != "https://id" {
				t.Errorf("issuer not exported live: %q", exported.Auth.OIDC.IssuerURL)
			}
		})
	}
}

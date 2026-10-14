package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// fakeEnv is an injected getenv backed by a map.
func fakeEnv(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

// fakeFiles is an injected readFile backed by a map; a missing path
// returns fs.ErrNotExist wrapped in a PathError, as os.ReadFile does.
func fakeFiles(m map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		v, ok := m[p]
		if !ok {
			return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
		}
		return []byte(v), nil
	}
}

func noFiles(string) ([]byte, error) { return nil, errors.New("no files in this test") }

// clearAuthEnv unsets every auth variable for the test (restored after),
// so Load and Parse see only what the test sets.
func clearAuthEnv(t *testing.T) {
	t.Helper()
	for _, n := range AuthEnvVarNames() {
		t.Setenv(n, "")
		os.Unsetenv(n)
	}
}

func TestAuthEnvVarNames(t *testing.T) {
	names := AuthEnvVarNames()
	if len(names) != 19 {
		t.Fatalf("got %d names, want 19: %v", len(names), names)
	}
	for _, want := range []string{EnvAuthMethod, EnvOIDCClientID + "_FILE", EnvOIDCClientSecret + "_FILE", EnvOIDCDisableLocalLogin} {
		found := false
		for _, n := range names {
			found = found || n == want
		}
		if !found {
			t.Errorf("%s missing from %v", want, names)
		}
	}
}

// Every variable sets its field, records an override with the variable as
// source, and leaves the file value for Save.
func TestApplyAuthEnv_EachVariable(t *testing.T) {
	tests := []struct {
		name  string
		value string
		field OverrideField
		check func(o *OIDCConfig) bool
	}{
		{EnvOIDCEnabled, "true", OverrideOIDCEnabled, func(o *OIDCConfig) bool { return o.Enabled }},
		{EnvOIDCIssuerURL, " https://id.example.com ", OverrideOIDCIssuerURL, func(o *OIDCConfig) bool { return o.IssuerURL == "https://id.example.com" }},
		{EnvOIDCClientID, "muximux", OverrideOIDCClientID, func(o *OIDCConfig) bool { return o.ClientID == "muximux" }},
		{EnvOIDCClientSecret, "s3cret", OverrideOIDCClientSecret, func(o *OIDCConfig) bool { return o.ClientSecret == "s3cret" }},
		{EnvOIDCRedirectURL, "https://dash.example.com/api/auth/oidc/callback", OverrideOIDCRedirectURL, func(o *OIDCConfig) bool {
			return o.RedirectURL == "https://dash.example.com/api/auth/oidc/callback"
		}},
		{EnvOIDCScopes, "openid, profile  email,groups", OverrideOIDCScopes, func(o *OIDCConfig) bool {
			return reflect.DeepEqual(o.Scopes, []string{"openid", "profile", "email", "groups"})
		}},
		{EnvOIDCUsernameClaim, "sub", OverrideOIDCUsernameClaim, func(o *OIDCConfig) bool { return o.UsernameClaim == "sub" }},
		{EnvOIDCEmailClaim, "mail", OverrideOIDCEmailClaim, func(o *OIDCConfig) bool { return o.EmailClaim == "mail" }},
		{EnvOIDCGroupsClaim, "roles", OverrideOIDCGroupsClaim, func(o *OIDCConfig) bool { return o.GroupsClaim == "roles" }},
		{EnvOIDCDisplayNameClaim, "nickname", OverrideOIDCDisplayNameClaim, func(o *OIDCConfig) bool { return o.DisplayNameClaim == "nickname" }},
		{EnvOIDCAdminGroups, "Dashboard Admins, ops,,", OverrideOIDCAdminGroups, func(o *OIDCConfig) bool {
			return reflect.DeepEqual(o.AdminGroups, []string{"Dashboard Admins", "ops"})
		}},
		{EnvOIDCProviderLogout, "yes", OverrideOIDCProviderLogout, func(o *OIDCConfig) bool { return o.ProviderLogout }},
		{EnvOIDCPostLogoutRedirectURL, "https://dash.example.com/bye", OverrideOIDCPostLogoutRedirectURL, func(o *OIDCConfig) bool {
			return o.PostLogoutRedirectURL == "https://dash.example.com/bye"
		}},
		{EnvOIDCLogoutURL, "https://id.example.com/logout", OverrideOIDCLogoutURL, func(o *OIDCConfig) bool { return o.LogoutURL == "https://id.example.com/logout" }},
		{EnvOIDCAutoRedirect, "on", OverrideOIDCAutoRedirect, func(o *OIDCConfig) bool { return o.AutoRedirect }},
		{EnvOIDCDisableLocalLogin, "1", OverrideOIDCDisableLocalLogin, func(o *OIDCConfig) bool { return o.DisableLocalLogin }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Auth.Method = "oidc" // file method oidc: no MUXIMUX_AUTH_METHOD needed
			cfg.Auth.OIDC.Enabled = true
			err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{tt.name: tt.value}), noFiles)
			if err != nil {
				t.Fatalf("ApplyAuthEnv: %v", err)
			}
			if !tt.check(&cfg.Auth.OIDC) {
				t.Errorf("field not applied: %+v", cfg.Auth.OIDC)
			}
			if got := cfg.EnvOverrides()[string(tt.field)]; got != tt.name {
				t.Errorf("EnvOverrides()[%s] = %q, want %q", tt.field, got, tt.name)
			}
			if tt.field != OverrideOIDCEnabled && len(cfg.EnvOverrides()) != 1 {
				t.Errorf("EnvOverrides() = %v, want only %s", cfg.EnvOverrides(), tt.field)
			}
		})
	}
}

// An empty or blank value, or a list of separators only, counts as unset.
func TestApplyAuthEnv_EmptyIsUnset(t *testing.T) {
	for _, n := range AuthEnvVarNames() {
		for _, v := range []string{"", "   "} {
			cfg := defaultConfig()
			if err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{n: v}), noFiles); err != nil {
				t.Fatalf("%s=%q: %v", n, v, err)
			}
			if len(cfg.EnvOverrides()) != 0 {
				t.Errorf("%s=%q recorded %v", n, v, cfg.EnvOverrides())
			}
		}
	}
	for _, n := range []string{EnvOIDCScopes, EnvOIDCAdminGroups} {
		cfg := defaultConfig()
		cfg.Auth.Method = "oidc"
		cfg.Auth.OIDC.Enabled = true
		cfg.Auth.OIDC.Scopes = []string{"openid"}
		if err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{n: " , ,"}), noFiles); err != nil {
			t.Fatal(err)
		}
		if len(cfg.EnvOverrides()) != 0 || !reflect.DeepEqual(cfg.Auth.OIDC.Scopes, []string{"openid"}) {
			t.Errorf("%s of separators only applied: %v %v", n, cfg.EnvOverrides(), cfg.Auth.OIDC.Scopes)
		}
	}
}

func TestSplitLists(t *testing.T) {
	if got := splitScopes("openid\tprofile\nemail , x"); !reflect.DeepEqual(got, []string{"openid", "profile", "email", "x"}) {
		t.Errorf("splitScopes = %q", got)
	}
	if got := splitCSV(" a b ,c,, "); !reflect.DeepEqual(got, []string{"a b", "c"}) {
		t.Errorf("splitCSV = %q", got)
	}
}

func TestApplyAuthEnv_Bools(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"true", true}, {"TRUE", true}, {"1", true}, {"yes", true}, {"on", true}, {" On ", true},
		{"false", false}, {"0", false}, {"no", false}, {"off", false}, {"OFF", false},
	} {
		cfg := defaultConfig()
		cfg.Auth.Method = "oidc"
		cfg.Auth.OIDC.Enabled = true
		cfg.Auth.OIDC.AutoRedirect = !tt.want
		if err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvOIDCAutoRedirect: tt.value}), noFiles); err != nil {
			t.Fatal(err)
		}
		if cfg.Auth.OIDC.AutoRedirect != tt.want || !cfg.IsOverridden(OverrideOIDCAutoRedirect) {
			t.Errorf("%q: auto_redirect = %v, want %v", tt.value, cfg.Auth.OIDC.AutoRedirect, tt.want)
		}
	}
}

// An invalid boolean warns (deferred until logging is up) and keeps the
// file value.
func TestApplyAuthEnv_InvalidBoolWarnsAndKeepsFile(t *testing.T) {
	for _, n := range []string{EnvOIDCEnabled, EnvOIDCProviderLogout, EnvOIDCAutoRedirect, EnvOIDCDisableLocalLogin} {
		cfg := defaultConfig()
		cfg.Auth.Method = "oidc"
		cfg.Auth.OIDC.Enabled = true
		cfg.Auth.OIDC.ProviderLogout = true
		if err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{n: "maybe"}), noFiles); err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if cfg.IsOverridden(OverrideOIDCAutoRedirect) || cfg.IsOverridden(OverrideOIDCProviderLogout) ||
			cfg.IsOverridden(OverrideOIDCDisableLocalLogin) || cfg.IsOverridden(OverrideOIDCEnabled) {
			t.Errorf("%s: invalid bool recorded an override: %v", n, cfg.EnvOverrides())
		}
		if !cfg.Auth.OIDC.Enabled || !cfg.Auth.OIDC.ProviderLogout {
			t.Errorf("%s: file value changed: %+v", n, cfg.Auth.OIDC)
		}
		if len(cfg.envWarnings) != 1 || !strings.Contains(strings.Join(argStrings(cfg.envWarnings[0].args), " "), n) {
			t.Fatalf("%s: warnings = %+v", n, cfg.envWarnings)
		}
		if got := cfg.LogEnvWarnings(); got != 1 {
			t.Errorf("LogEnvWarnings() = %d, want 1", got)
		}
	}
}

func argStrings(args []any) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if s, ok := a.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestApplyAuthEnv_Method(t *testing.T) {
	for _, v := range []string{"none", "builtin", "forward_auth", "oidc", " OIDC "} {
		cfg := defaultConfig()
		env := map[string]string{EnvAuthMethod: v}
		if err := ApplyAuthEnv(cfg, fakeEnv(env), noFiles); err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		want := strings.ToLower(strings.TrimSpace(v))
		if cfg.Auth.Method != want || cfg.EnvOverrides()["auth.method"] != EnvAuthMethod {
			t.Errorf("%q: method = %q overrides %v", v, cfg.Auth.Method, cfg.EnvOverrides())
		}
		if v := cfg.fileView().Auth.Method; v != "none" {
			t.Errorf("file view method = %q, want none", v)
		}
	}
}

func TestApplyAuthEnv_InvalidMethodFatal(t *testing.T) {
	cfg := defaultConfig()
	err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvAuthMethod: "ldap"}), noFiles)
	if err == nil || !strings.Contains(err.Error(), EnvAuthMethod) {
		t.Fatalf("err = %v, want one naming %s", err, EnvAuthMethod)
	}
	if cfg.Auth.Method != "none" {
		t.Errorf("method changed to %q", cfg.Auth.Method)
	}
}

// MUXIMUX_AUTH_METHOD=oidc implies enabled, sourced from the method variable.
func TestApplyAuthEnv_OIDCMethodImpliesEnabled(t *testing.T) {
	cfg := defaultConfig()
	if err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvAuthMethod: "oidc"}), noFiles); err != nil {
		t.Fatal(err)
	}
	if !cfg.Auth.OIDC.Enabled || cfg.EnvOverrides()["auth.oidc.enabled"] != EnvAuthMethod {
		t.Fatalf("enabled = %v, overrides %v", cfg.Auth.OIDC.Enabled, cfg.EnvOverrides())
	}
	if cfg.fileView().Auth.OIDC.Enabled {
		t.Error("file view should keep enabled=false")
	}

	// Explicit ENABLED=true keeps its own source.
	cfg = defaultConfig()
	if err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvAuthMethod: "oidc", EnvOIDCEnabled: "true"}), noFiles); err != nil {
		t.Fatal(err)
	}
	if cfg.EnvOverrides()["auth.oidc.enabled"] != EnvOIDCEnabled {
		t.Errorf("source = %q", cfg.EnvOverrides()["auth.oidc.enabled"])
	}

	// A file method of oidc with no env method leaves enabled alone.
	cfg = defaultConfig()
	cfg.Auth.Method = "oidc"
	if err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvOIDCClientID: "x"}), noFiles); err != nil {
		t.Fatal(err)
	}
	if cfg.IsOverridden(OverrideOIDCEnabled) {
		t.Error("enabled overridden without MUXIMUX_AUTH_METHOD")
	}
}

func TestApplyAuthEnv_EnabledFalseWithOIDCMethodFatal(t *testing.T) {
	cfg := defaultConfig()
	err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvAuthMethod: "oidc", EnvOIDCEnabled: "false"}), noFiles)
	if err == nil || !strings.Contains(err.Error(), EnvOIDCEnabled) || !strings.Contains(err.Error(), EnvAuthMethod) {
		t.Fatalf("err = %v", err)
	}
	cfg = defaultConfig()
	cfg.Auth.Method = "oidc"
	err = ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvOIDCEnabled: "no"}), noFiles)
	if err == nil || !strings.Contains(err.Error(), "config file") {
		t.Fatalf("file method oidc: err = %v", err)
	}
}

// Fail closed: an OIDC variable without MUXIMUX_AUTH_METHOD refuses to start
// unless the file's method is already oidc.
func TestApplyAuthEnv_FailClosedWithoutMethod(t *testing.T) {
	tests := []struct {
		name       string
		fileMethod string
		env        map[string]string
		wantErr    bool
	}{
		{"oidc var, file none", "none", map[string]string{EnvOIDCIssuerURL: "https://id"}, true},
		{"oidc var, file empty", "", map[string]string{EnvOIDCClientID: "x"}, true},
		{"oidc var, file builtin", "builtin", map[string]string{EnvOIDCRedirectURL: "https://m/cb"}, true}, //nolint:gosec // test URL, not a credential
		{"enabled true, file none", "none", map[string]string{EnvOIDCEnabled: "yes"}, true},
		{"client id file variant", "none", map[string]string{EnvOIDCClientID + "_FILE": "/run/secrets/id"}, true},
		// Part 1 ruling: ENABLED=false and tuning variables do not configure
		// sign-in, so on their own they never refuse to start.
		{"enabled false alone", "builtin", map[string]string{EnvOIDCEnabled: "false"}, false},
		{"enabled invalid alone", "none", map[string]string{EnvOIDCEnabled: "maybe"}, false},
		{"tuning vars alone", "builtin", map[string]string{EnvOIDCAutoRedirect: "true", EnvOIDCScopes: "openid", EnvOIDCAdminGroups: "a"}, false},
		{"file variant only", "none", map[string]string{EnvOIDCClientSecret + "_FILE": "/run/secrets/x"}, true},
		{"method var empty", "none", map[string]string{EnvAuthMethod: "", EnvOIDCClientID: "x"}, true},
		{"file oidc", "oidc", map[string]string{EnvOIDCClientID: "x"}, false},
		{"explicit builtin", "none", map[string]string{EnvAuthMethod: "builtin", EnvOIDCClientID: "x"}, false},
		{"explicit oidc", "none", map[string]string{EnvAuthMethod: "oidc", EnvOIDCClientID: "x"}, false},
		{"no oidc vars", "none", map[string]string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Auth.Method = tt.fileMethod
			err := ApplyAuthEnv(cfg, fakeEnv(tt.env), noFiles)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), EnvAuthMethod) {
					t.Fatalf("err = %v, want one naming %s", err, EnvAuthMethod)
				}
				if len(cfg.EnvOverrides()) != 0 {
					t.Errorf("overrides applied before refusing: %v", cfg.EnvOverrides())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEnvOrFile(t *testing.T) {
	const secretContent = "TOPSECRET-value"
	big := strings.Repeat("x", maxEnvFileSize+1)
	exact := strings.Repeat("y", maxEnvFileSize)
	files := fakeFiles(map[string]string{
		"/s/lf":    secretContent + "\n",
		"/s/crlf":  secretContent + "\r\n\r\n",
		"/s/inner": "  a b\tc  \n",
		"/s/empty": "",
		"/s/nl":    "\r\n",
		"/s/big":   big,
		"/s/exact": exact,
	})
	tests := []struct {
		name      string
		env       map[string]string
		want      string
		wantSrc   string
		wantOK    bool
		errSubstr []string
	}{
		{"unset", map[string]string{}, "", "", false, nil},
		{"direct trimmed", map[string]string{"X": "  v  "}, "v", "X", true, nil},
		{"both", map[string]string{"X": "v", "X_FILE": "/s/lf"}, "", "", false, []string{"X", "X_FILE", "not both"}},
		{"lf trimmed", map[string]string{"X_FILE": "/s/lf"}, secretContent, "X_FILE", true, nil},
		{"crlf trimmed", map[string]string{"X_FILE": "/s/crlf"}, secretContent, "X_FILE", true, nil},
		{"inner and leading kept", map[string]string{"X_FILE": "/s/inner"}, "  a b\tc  ", "X_FILE", true, nil},
		{"path trimmed", map[string]string{"X_FILE": " /s/lf "}, secretContent, "X_FILE", true, nil},
		{"exactly the cap", map[string]string{"X_FILE": "/s/exact"}, exact, "X_FILE", true, nil},
		{"missing", map[string]string{"X_FILE": "/s/nope"}, "", "", false, []string{"X_FILE", "/s/nope", "not exist"}},
		{"empty", map[string]string{"X_FILE": "/s/empty"}, "", "", false, []string{"X_FILE", "/s/empty", "empty"}},
		{"newline only", map[string]string{"X_FILE": "/s/nl"}, "", "", false, []string{"X_FILE", "/s/nl", "empty"}},
		{"over the cap", map[string]string{"X_FILE": "/s/big"}, "", "", false, []string{"X_FILE", "/s/big", "larger"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, src, ok, err := envOrFile(nonEmptyEnv(fakeEnv(tt.env)), files, "X")
			if tt.errSubstr != nil {
				if err == nil {
					t.Fatal("want error")
				}
				for _, s := range tt.errSubstr {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("error %q does not contain %q", err, s)
					}
				}
				if strings.Contains(err.Error(), secretContent) || strings.Contains(err.Error(), "xxxx") {
					t.Errorf("error leaks file content: %q", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v != tt.want || src != tt.wantSrc || ok != tt.wantOK {
				t.Errorf("got (%q, %q, %v), want (%q, %q, %v)", v, src, ok, tt.want, tt.wantSrc, tt.wantOK)
			}
		})
	}
}

// A generic read error (not a PathError) is reported as is, still without
// content.
func TestEnvOrFile_OtherReadError(t *testing.T) {
	read := func(string) ([]byte, error) { return nil, errors.New("permission denied") }
	_, _, _, err := envOrFile(fakeEnv(map[string]string{"X_FILE": "/p"}), read, "X")
	if err == nil || !strings.Contains(err.Error(), "X_FILE: cannot read /p: permission denied") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadEnvFile(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small")
	if err := os.WriteFile(small, []byte("abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := readEnvFile(small); err != nil || string(b) != "abc\n" {
		t.Fatalf("readEnvFile(small) = %q, %v", b, err)
	}
	big := filepath.Join(dir, "big")
	if err := os.WriteFile(big, make([]byte, 3*maxEnvFileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := readEnvFile(big); err != nil || len(b) != maxEnvFileSize+1 {
		t.Fatalf("readEnvFile(big) read %d bytes, %v; want the cap plus one", len(b), err)
	}
	if _, err := readEnvFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing file: want error")
	}
	// A directory opens but cannot be read.
	_, _, _, err := envOrFile(fakeEnv(map[string]string{"X_FILE": dir}), readEnvFile, "X")
	if err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatalf("directory: err = %v", err)
	}
}

// The *_FILE variants apply through ApplyAuthEnv with the file variable as
// source, and their errors are fatal.
func TestApplyAuthEnv_FileVariants(t *testing.T) {
	files := fakeFiles(map[string]string{"/run/secrets/id": "client-1\n", "/run/secrets/secret": "s3cret\r\n"})
	cfg := defaultConfig()
	env := map[string]string{
		EnvAuthMethod:                 "oidc",
		EnvOIDCClientID + "_FILE":     "/run/secrets/id",
		EnvOIDCClientSecret + "_FILE": "/run/secrets/secret",
	}
	if err := ApplyAuthEnv(cfg, fakeEnv(env), files); err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.OIDC.ClientID != "client-1" || cfg.Auth.OIDC.ClientSecret != "s3cret" {
		t.Errorf("values = %q %q", cfg.Auth.OIDC.ClientID, cfg.Auth.OIDC.ClientSecret)
	}
	ov := cfg.EnvOverrides()
	if ov["auth.oidc.client_id"] != EnvOIDCClientID+"_FILE" || ov["auth.oidc.client_secret"] != EnvOIDCClientSecret+"_FILE" {
		t.Errorf("sources = %v", ov)
	}
	for _, v := range ov {
		if strings.Contains(v, "s3cret") || strings.Contains(v, "client-1") {
			t.Errorf("env_overrides carries a value: %v", ov)
		}
	}

	for _, n := range []string{EnvOIDCClientID, EnvOIDCClientSecret} {
		cfg := defaultConfig()
		err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{EnvAuthMethod: "oidc", n: "a", n + "_FILE": "/run/secrets/id"}), files)
		if err == nil || !strings.Contains(err.Error(), "not both") {
			t.Errorf("%s both set: err = %v", n, err)
		}
		err = ApplyAuthEnv(defaultConfig(), fakeEnv(map[string]string{EnvAuthMethod: "oidc", n + "_FILE": "/run/secrets/none"}), files)
		if err == nil || !strings.Contains(err.Error(), n+"_FILE") {
			t.Errorf("%s missing file: err = %v", n, err)
		}
	}
}

func TestApplyAuthEnv_AdminGroupsWarning(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		users []UserConfig
		warn  bool
	}{
		{"no groups, no admin", map[string]string{EnvAuthMethod: "oidc"}, nil, true},
		{"only a non-admin user", map[string]string{EnvAuthMethod: "oidc"}, []UserConfig{{Username: "u", Role: "user"}}, true},
		{"groups from env", map[string]string{EnvAuthMethod: "oidc", EnvOIDCAdminGroups: "admins"}, nil, false},
		{"local admin", map[string]string{EnvAuthMethod: "oidc"}, []UserConfig{{Username: "a", Role: "admin"}}, false},
		{"method builtin", map[string]string{EnvAuthMethod: "builtin"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Auth.Users = tt.users
			if err := ApplyAuthEnv(cfg, fakeEnv(tt.env), noFiles); err != nil {
				t.Fatal(err)
			}
			if got := len(cfg.envWarnings) == 1; got != tt.warn {
				t.Errorf("warned = %v, want %v (%+v)", got, tt.warn, cfg.envWarnings)
			}
		})
	}
}

func TestApplyAuthEnv_NilConfig(t *testing.T) {
	if err := ApplyAuthEnv(nil, fakeEnv(map[string]string{EnvAuthMethod: "x"}), noFiles); err != nil {
		t.Fatal(err)
	}
}

// Lists round-trip through the override encoding, keeping nil apart from
// an empty list.
func TestOverride_ListRoundTrip(t *testing.T) {
	for _, file := range [][]string{nil, {}, {"openid", "email"}} {
		cfg := defaultConfig()
		cfg.Auth.OIDC.Scopes = file
		cfg.Auth.OIDC.AdminGroups = file
		cfg.ApplyOverride(OverrideOIDCScopes, EnvOIDCScopes, encodeList([]string{"a"}))
		cfg.ApplyOverride(OverrideOIDCAdminGroups, EnvOIDCAdminGroups, encodeList([]string{"g"}))
		v := cfg.fileView()
		if !reflect.DeepEqual(v.Auth.OIDC.Scopes, file) || !reflect.DeepEqual(v.Auth.OIDC.AdminGroups, file) {
			t.Errorf("file %#v: view %#v %#v", file, v.Auth.OIDC.Scopes, v.Auth.OIDC.AdminGroups)
		}
		if !reflect.DeepEqual(cfg.Auth.OIDC.Scopes, []string{"a"}) || !reflect.DeepEqual(cfg.Auth.OIDC.AdminGroups, []string{"g"}) {
			t.Errorf("live changed by fileView: %v %v", cfg.Auth.OIDC.Scopes, cfg.Auth.OIDC.AdminGroups)
		}
	}
	if decodeList("not json") != nil {
		t.Error("decodeList of garbage should be nil")
	}
}

const oidcFileYAML = `auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://file-idp.example.com
    client_id: file-client
    client_secret: ${FILE_OIDC_SECRET}
    redirect_url: https://dash.example.com/cb
    scopes: [openid]
    username_claim: preferred_username
    email_claim: email
    groups_claim: groups
    display_name_claim: name
    admin_groups: [file-admins]
    provider_logout: false
    post_logout_redirect_url: https://dash.example.com/out
    logout_url: https://file-idp.example.com/logout
    auto_redirect: false
    disable_local_login: false
`

// allOIDCEnv sets every variable to a value different from oidcFileYAML.
func allOIDCEnv(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{ //nolint:gosec // fake test values
		EnvAuthMethod:                "oidc",
		EnvOIDCEnabled:               "true",
		EnvOIDCIssuerURL:             "https://env-idp.example.com",
		EnvOIDCClientID:              "env-client",
		EnvOIDCClientSecret:          "env-secret-value",
		EnvOIDCRedirectURL:           "https://env.example.com/cb",
		EnvOIDCScopes:                "openid profile",
		EnvOIDCUsernameClaim:         "sub",
		EnvOIDCEmailClaim:            "mail",
		EnvOIDCGroupsClaim:           "roles",
		EnvOIDCDisplayNameClaim:      "nick",
		EnvOIDCAdminGroups:           "env-admins",
		EnvOIDCProviderLogout:        "true",
		EnvOIDCPostLogoutRedirectURL: "https://env.example.com/out",
		EnvOIDCLogoutURL:             "https://env-idp.example.com/logout",
		EnvOIDCAutoRedirect:          "true",
		EnvOIDCDisableLocalLogin:     "true",
	} {
		t.Setenv(k, v)
	}
}

// readSavedOIDC parses a saved file without applying the environment.
func readSavedOIDC(t *testing.T, path string) (AuthConfig, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return raw.Auth, string(data)
}

// Save writes the file's own values for every env-overridden field, and the
// ${VAR} reference for the secret.
func TestApplyAuthEnv_SaveWritesFileValues(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("FILE_OIDC_SECRET", "file-secret-value")
	allOIDCEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(oidcFileYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	o := &cfg.Auth.OIDC
	if o.IssuerURL != "https://env-idp.example.com" || o.ClientSecret != "env-secret-value" ||
		!reflect.DeepEqual(o.Scopes, []string{"openid", "profile"}) || !o.DisableLocalLogin {
		t.Fatalf("env not applied: %+v", o)
	}
	if n := len(cfg.EnvOverrides()); n != 17 {
		t.Errorf("EnvOverrides() has %d entries, want 17: %v", n, cfg.EnvOverrides())
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if o.ClientSecret != "env-secret-value" || o.ClientID != "env-client" {
		t.Errorf("Save changed live values: %+v", o)
	}
	saved, text := readSavedOIDC(t, path)
	so := saved.OIDC
	if saved.Method != "oidc" || so.IssuerURL != "https://file-idp.example.com" || so.ClientID != "file-client" ||
		so.RedirectURL != "https://dash.example.com/cb" || !reflect.DeepEqual(so.Scopes, []string{"openid"}) ||
		so.UsernameClaim != "preferred_username" || so.EmailClaim != "email" || so.GroupsClaim != "groups" ||
		so.DisplayNameClaim != "name" || !reflect.DeepEqual(so.AdminGroups, []string{"file-admins"}) ||
		so.ProviderLogout || so.PostLogoutRedirectURL != "https://dash.example.com/out" ||
		so.LogoutURL != "https://file-idp.example.com/logout" || so.AutoRedirect || so.DisableLocalLogin {
		t.Errorf("saved file has env values: %+v", saved)
	}
	if !strings.Contains(text, "${FILE_OIDC_SECRET}") {
		t.Errorf("secret reference not written back:\n%s", text)
	}
	for _, leak := range []string{"env-secret-value", "env-client", "env-idp"} {
		if strings.Contains(text, leak) {
			t.Errorf("saved file contains env value %q", leak)
		}
	}
}

// A fresh container (no config.yaml) applies every environment override,
// discovery included, and validates.
func TestLoad_MissingFileAppliesEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv(EnvAutoImport, "sync")
	t.Setenv(EnvRequireExplicitEnable, "true")
	t.Setenv(EnvAuthMethod, "oidc")
	t.Setenv(EnvOIDCIssuerURL, "https://id.example.com")
	t.Setenv(EnvOIDCClientID, "muximux")
	t.Setenv(EnvOIDCAdminGroups, "admins")
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Discovery.Docker.AutoImport != AutoImportSync || !cfg.Discovery.Docker.RequireExplicitEnable {
		t.Errorf("discovery env not applied: %+v", cfg.Discovery.Docker)
	}
	if cfg.Auth.Method != "oidc" || !cfg.Auth.OIDC.Enabled || cfg.Auth.OIDC.ClientID != "muximux" {
		t.Errorf("oidc env not applied: %+v", cfg.Auth)
	}
	if cfg.NeedsSetup() {
		t.Error("NeedsSetup() = true with MUXIMUX_AUTH_METHOD=oidc")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Load created config.yaml: %v", err)
	}
	if cfg.Server.Listen != ":8080" || cfg.Navigation.IconScale != 1.0 {
		t.Errorf("defaults lost: %+v", cfg.Server)
	}
}

func TestLoad_MissingFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	clearAuthEnv(t)
	t.Setenv(EnvAuthMethod, "bogus")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), EnvAuthMethod) {
		t.Errorf("invalid method: err = %v", err)
	}

	// Validation runs on the no-file path too.
	clearAuthEnv(t)
	t.Setenv(EnvAuthMethod, "oidc")
	t.Setenv(EnvOIDCIssuerURL, "https://id.example.com")
	t.Setenv(EnvOIDCClientID, "muximux")
	t.Setenv(EnvOIDCRedirectURL, "/relative")
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "MUXIMUX_OIDC_REDIRECT_URL (auth.oidc.redirect_url) must be an absolute http(s) URL") {
		t.Errorf("redirect: err = %v", err)
	}

	// Fail closed with no file: default method none.
	clearAuthEnv(t)
	t.Setenv(EnvOIDCClientID, "muximux")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), EnvAuthMethod) {
		t.Errorf("fail closed: err = %v", err)
	}
}

func TestLoad_FileWithInvalidAuthEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv(EnvAuthMethod, "bogus")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server: {title: x}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), EnvAuthMethod) {
		t.Errorf("err = %v", err)
	}
	if _, err := Parse([]byte("server: {title: x}\n")); err == nil {
		t.Error("Parse: want error")
	}
}

// minYAML is the smallest non-empty config (an empty document does not
// parse).
const minYAML = "server: {title: x}\n"

// Validation names the variable for an env value and the YAML path for a
// file value, and never includes the value.
func TestValidate_OIDCLabels(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		env     map[string]string
		wantErr string
	}{
		{"yaml redirect", "auth: {method: oidc, oidc: {enabled: true, redirect_url: not-a-url}}\n", nil,
			"auth.oidc.redirect_url must be an absolute http(s) URL"},
		{"env redirect", "auth: {method: oidc, oidc: {enabled: true, issuer_url: https://id, client_id: c}}\n",
			map[string]string{EnvOIDCRedirectURL: "ftp://x"},
			"MUXIMUX_OIDC_REDIRECT_URL (auth.oidc.redirect_url) must be an absolute http(s) URL"},
		{"env logout url", "auth: {method: oidc, oidc: {enabled: true, issuer_url: https://id, client_id: c}}\n",
			map[string]string{EnvOIDCLogoutURL: "nope"},
			"MUXIMUX_OIDC_LOGOUT_URL (auth.oidc.logout_url) must be"},
		{"env post logout", minYAML, map[string]string{EnvAuthMethod: "oidc", EnvOIDCIssuerURL: "https://id", EnvOIDCClientID: "c", EnvOIDCPostLogoutRedirectURL: "x"},
			"MUXIMUX_OIDC_POST_LOGOUT_REDIRECT_URL (auth.oidc.post_logout_redirect_url) must be"},
		{"env method, no issuer", minYAML, map[string]string{EnvAuthMethod: "oidc", EnvOIDCClientID: "c"},
			"set MUXIMUX_OIDC_ISSUER_URL"},
		{"env issuer not a URL", minYAML, map[string]string{EnvAuthMethod: "oidc", EnvOIDCIssuerURL: "id.example.com", EnvOIDCClientID: "c"},
			"MUXIMUX_OIDC_ISSUER_URL (auth.oidc.issuer_url) must be an absolute http(s) URL"},
		{"file issuer not a URL, env method", "auth: {oidc: {issuer_url: id.example.com}}\n", map[string]string{EnvAuthMethod: "oidc", EnvOIDCClientID: "c"},
			"auth.oidc.issuer_url must be an absolute http(s) URL"},
		{"env method, no client id", minYAML, map[string]string{EnvAuthMethod: "oidc", EnvOIDCIssuerURL: "https://id"},
			"set MUXIMUX_OIDC_CLIENT_ID or MUXIMUX_OIDC_CLIENT_ID_FILE"},
		{"env client id only, file oidc without issuer", "auth: {method: oidc, oidc: {enabled: true}}\n", map[string]string{EnvOIDCClientID: "c"},
			"set MUXIMUX_OIDC_ISSUER_URL"},
		{"yaml only oidc without issuer still loads", "auth: {method: oidc, oidc: {enabled: true}}\n", nil, ""},
		{"env oidc complete", minYAML, map[string]string{EnvAuthMethod: "oidc", EnvOIDCIssuerURL: "https://id", EnvOIDCClientID: "c"}, ""},
		{"env builtin ignores issuer", minYAML, map[string]string{EnvAuthMethod: "builtin"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAuthEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := Parse([]byte(tt.yaml))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
			}
			for _, v := range tt.env {
				if len(v) > 3 && strings.Contains(err.Error(), v) && v != "oidc" {
					t.Errorf("error includes the value %q: %v", v, err)
				}
			}
		})
	}
}

// Exported ValidateOIDC keeps the plain YAML path.
func TestValidateOIDC_PlainLabels(t *testing.T) {
	err := ValidateOIDC(&OIDCConfig{Enabled: true, RedirectURL: "x"})
	if err == nil || err.Error() != "auth.oidc.redirect_url must be an absolute http(s) URL" {
		t.Fatalf("err = %v", err)
	}
}

func TestOIDCClientSecretInPlaintext_EnvSecret(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv(EnvAuthMethod, "oidc")
	t.Setenv(EnvOIDCIssuerURL, "https://id")
	t.Setenv(EnvOIDCClientID, "c")
	t.Setenv(EnvOIDCClientSecret, "env-secret")

	cfg, err := Parse([]byte(minYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDCClientSecretInPlaintext() {
		t.Error("an env-only secret is not plaintext in config.yaml")
	}

	cfg, err = Parse([]byte("auth: {oidc: {client_secret: literal-in-file}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OIDCClientSecretInPlaintext() {
		t.Error("a literal secret in the file counts even while env overrides it")
	}
}

// A restore parses the backup with the environment applied, then inherits
// the live overrides: the env values stay live, Save writes the backup's.
func TestInheritRuntime_AuthEnv(t *testing.T) {
	clearAuthEnv(t)
	allOIDCEnv(t)
	t.Setenv("FILE_OIDC_SECRET", "file-secret-value")
	prev, err := Parse([]byte(oidcFileYAML))
	if err != nil {
		t.Fatal(err)
	}
	backup := strings.ReplaceAll(oidcFileYAML, "file-client", "backup-client")
	backup = strings.ReplaceAll(backup, "[file-admins]", "[backup-admins]")
	next, err := Parse([]byte(backup))
	if err != nil {
		t.Fatal(err)
	}
	next.InheritRuntime(prev)
	if next.Auth.OIDC.ClientID != "env-client" || !reflect.DeepEqual(next.Auth.OIDC.AdminGroups, []string{"env-admins"}) ||
		next.Auth.OIDC.ClientSecret != "env-secret-value" {
		t.Errorf("live values lost: %+v", next.Auth.OIDC)
	}
	if !reflect.DeepEqual(prev.EnvOverrides(), next.EnvOverrides()) {
		t.Errorf("overrides differ: %v vs %v", prev.EnvOverrides(), next.EnvOverrides())
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := next.Save(path); err != nil {
		t.Fatal(err)
	}
	saved, text := readSavedOIDC(t, path)
	if saved.OIDC.ClientID != "backup-client" || !reflect.DeepEqual(saved.OIDC.AdminGroups, []string{"backup-admins"}) {
		t.Errorf("saved %+v, want the backup's values", saved.OIDC)
	}
	if strings.Contains(text, "env-") {
		t.Errorf("saved file has env values:\n%s", text)
	}

	// A backup without OIDC plus env OIDC still ends up on OIDC.
	bare, err := Parse([]byte("server: {title: x}\n"))
	if err != nil {
		t.Fatal(err)
	}
	bare.InheritRuntime(prev)
	if bare.Auth.Method != "oidc" || !bare.Auth.OIDC.Enabled || bare.fileView().Auth.Method != "none" {
		t.Errorf("bare restore: method %q enabled %v", bare.Auth.Method, bare.Auth.OIDC.Enabled)
	}
}

func TestNeedsSetup_EnvMethod(t *testing.T) {
	tests := []struct {
		method string
		users  []UserConfig
		apps   int
		done   bool
		want   bool
	}{
		{"oidc", nil, 0, false, false},
		{"none", nil, 0, false, false},
		{"forward_auth", nil, 0, false, false},
		{"builtin", nil, 0, false, true},
		{"builtin", nil, 2, true, true},
		{"builtin", []UserConfig{{Username: "a", Role: "admin"}}, 0, false, false},
	}
	for _, tt := range tests {
		cfg := defaultConfig()
		cfg.Auth.Users = tt.users
		cfg.Auth.SetupComplete = tt.done
		for i := 0; i < tt.apps; i++ {
			cfg.Apps = append(cfg.Apps, AppConfig{Name: "a"})
		}
		cfg.ApplyOverride(OverrideAuthMethod, EnvAuthMethod, tt.method)
		if got := cfg.NeedsSetup(); got != tt.want {
			t.Errorf("method %s users %d apps %d done %v: NeedsSetup() = %v, want %v",
				tt.method, len(tt.users), tt.apps, tt.done, got, tt.want)
		}
	}
	// Without the env, the file semantics are unchanged.
	if !defaultConfig().NeedsSetup() {
		t.Error("fresh default config should need setup")
	}
}

func TestApplyOverride_AuthFieldsAll(t *testing.T) {
	cfg := defaultConfig()
	for i := range oidcEnvVars {
		v := &oidcEnvVars[i]
		val := "x"
		switch v.kind {
		case envBool:
			val = "true"
		case envScopes, envCSV:
			val = encodeList([]string{"x"})
		}
		cfg.ApplyOverride(v.field, v.name, val)
	}
	cfg.ApplyOverride(OverrideAuthMethod, EnvAuthMethod, "oidc")
	if len(cfg.EnvOverrides()) != 17 {
		t.Fatalf("EnvOverrides() = %d entries, want 17", len(cfg.EnvOverrides()))
	}
	v := cfg.fileView()
	if !reflect.DeepEqual(v.Auth, defaultConfig().Auth) {
		t.Errorf("file view auth = %+v, want the defaults", v.Auth)
	}
	if cfg.fieldLabel("auth.oidc.issuer_url") != "MUXIMUX_OIDC_ISSUER_URL (auth.oidc.issuer_url)" || cfg.fieldLabel("server.listen") != "server.listen" {
		t.Errorf("fieldLabel wrong")
	}
}

// An unknown MUXIMUX_OIDC_* name logs one WARN naming it; known names,
// other prefixes and the value never appear.
func TestWarnUnknownOIDCEnv(t *testing.T) {
	cfg := defaultConfig()
	WarnUnknownOIDCEnv(cfg, []string{
		"MUXIMUX_OIDC_CLIENT_SECERT=hunter2",
		"MUXIMUX_OIDC_ISSUER_URL=https://id",
		"MUXIMUX_OIDC_CLIENT_SECRET_FILE=/run/secrets/x",
		"MUXIMUX_OIDC_SCOPE",
		"MUXIMUX_LOG_LEVEL=debug",
		"PATH=/usr/bin",
	})
	if len(cfg.envWarnings) != 2 {
		t.Fatalf("warnings = %d, want 2: %+v", len(cfg.envWarnings), cfg.envWarnings)
	}
	for i, want := range []string{"MUXIMUX_OIDC_CLIENT_SECERT", "MUXIMUX_OIDC_SCOPE"} {
		args := strings.Join(argStrings(cfg.envWarnings[i].args), " ")
		if !strings.Contains(args, want) {
			t.Errorf("warning %d args %q do not name %s", i, args, want)
		}
		if strings.Contains(args, "hunter2") {
			t.Errorf("warning %d echoes the value", i)
		}
	}
	WarnUnknownOIDCEnv(nil, []string{"MUXIMUX_OIDC_X=1"}) // no panic
}

// Load runs the unknown-name check against the real environment.
func TestLoad_WarnsUnknownOIDCEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("MUXIMUX_OIDC_ISSUER", "https://id")
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range cfg.envWarnings {
		if strings.Contains(strings.Join(argStrings(w.args), " "), "MUXIMUX_OIDC_ISSUER") {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning for MUXIMUX_OIDC_ISSUER: %+v", cfg.envWarnings)
	}
}

func TestOIDCEnvConfiguringVars(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"none", map[string]string{}, nil},
		{"tuning only", map[string]string{EnvOIDCScopes: "openid", EnvOIDCAutoRedirect: "true"}, nil},
		{"enabled false", map[string]string{EnvAuthMethod: "builtin", EnvOIDCEnabled: "false"}, nil},
		{"enabled true and issuer", map[string]string{EnvOIDCEnabled: "true", EnvOIDCIssuerURL: "https://id"},
			[]string{EnvOIDCEnabled, EnvOIDCIssuerURL}},
		{"secret file", map[string]string{EnvOIDCClientSecret + "_FILE": "/s"}, []string{EnvOIDCClientSecret + "_FILE"}},
		{"implied enabled", map[string]string{EnvAuthMethod: "oidc", EnvOIDCClientID: "c"}, []string{EnvOIDCClientID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Auth.Method = "oidc"
			if err := ApplyAuthEnv(cfg, fakeEnv(tt.env), fakeFiles(map[string]string{"/s": "x"})); err != nil {
				t.Fatal(err)
			}
			if got := cfg.OIDCEnvConfiguringVars(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// OIDC variables set under a method that does not use them log one WARN
// naming them (never the values); oidc, builtin with OIDC enabled, and no
// method at all stay quiet.
func TestWarnIgnoredOIDCEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		enabled bool
		want    string // "" means no warning
	}{
		{"none with oidc vars", map[string]string{EnvAuthMethod: "none", EnvOIDCIssuerURL: "https://id", EnvOIDCClientSecret + "_FILE": "/s"},
			false, EnvOIDCIssuerURL + ", " + EnvOIDCClientSecret + "_FILE"},
		{"forward_auth with tuning var", map[string]string{EnvAuthMethod: "Forward_Auth", EnvOIDCScopes: "openid"}, false, EnvOIDCScopes},
		{"builtin without oidc enabled", map[string]string{EnvAuthMethod: "builtin", EnvOIDCClientID: "c"}, false, EnvOIDCClientID},
		{"builtin enabled true but provider off", map[string]string{EnvAuthMethod: "builtin", EnvOIDCEnabled: "true"}, false, EnvOIDCEnabled},
		{"builtin with oidc enabled", map[string]string{EnvAuthMethod: "builtin", EnvOIDCClientID: "c"}, true, ""},
		{"enabled false agrees", map[string]string{EnvAuthMethod: "none", EnvOIDCEnabled: "false"}, false, ""},
		{"oidc", map[string]string{EnvAuthMethod: "oidc", EnvOIDCClientID: "c"}, true, ""},
		{"no method", map[string]string{EnvOIDCScopes: "openid"}, false, ""},
		{"blank method", map[string]string{EnvAuthMethod: " ", EnvOIDCScopes: "openid"}, false, ""},
		{"none and nothing else", map[string]string{EnvAuthMethod: "none"}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Auth.OIDC.Enabled = tt.enabled
			WarnIgnoredOIDCEnv(cfg, fakeEnv(tt.env))
			if tt.want == "" {
				if len(cfg.envWarnings) != 0 {
					t.Fatalf("unexpected warnings: %+v", cfg.envWarnings)
				}
				return
			}
			if len(cfg.envWarnings) != 1 {
				t.Fatalf("warnings = %+v, want one", cfg.envWarnings)
			}
			args := argStrings(cfg.envWarnings[0].args)
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("args %q do not name %q", joined, tt.want)
			}
			for _, v := range tt.env {
				if strings.HasPrefix(v, "/") || strings.HasPrefix(v, "https") {
					if strings.Contains(joined, v) {
						t.Errorf("warning echoes the value %q", v)
					}
				}
			}
		})
	}
	WarnIgnoredOIDCEnv(nil, fakeEnv(nil)) // no panic
}

// The environment WARNs come from Load only: Parse (used by restore) does
// not repeat them, so each is logged once per process.
func TestStartupAuthEnvWarnings_LoadOnly(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("MUXIMUX_OIDC_ISSUER", "https://id")
	t.Setenv(EnvAuthMethod, "none")
	t.Setenv(EnvOIDCScopes, "openid")

	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.envWarnings) != 2 {
		t.Fatalf("Load warnings = %+v, want unknown name and ignored vars", cfg.envWarnings)
	}

	parsed, err := Parse([]byte("server:\n  title: Dash\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.envWarnings) != 0 {
		t.Errorf("Parse repeated the startup warnings: %+v", parsed.envWarnings)
	}
}

// An empty X_FILE counts as unset, so X=val works next to X_FILE=.
func TestApplyAuthEnv_EmptyFileVarIsUnset(t *testing.T) {
	cfg := defaultConfig()
	cfg.Auth.Method = "oidc"
	cfg.Auth.OIDC.Enabled = true
	err := ApplyAuthEnv(cfg, fakeEnv(map[string]string{
		EnvOIDCClientID:                 "cid",
		EnvOIDCClientID + envFileSuffix: "",
	}), noFiles)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.OIDC.ClientID != "cid" || cfg.EnvOverrides()[string(OverrideOIDCClientID)] != EnvOIDCClientID {
		t.Errorf("client id = %q, %v", cfg.Auth.OIDC.ClientID, cfg.EnvOverrides())
	}
}

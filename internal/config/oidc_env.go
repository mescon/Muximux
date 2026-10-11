package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/mescon/muximux/v3/internal/logging"
)

// Environment variables that configure authentication (#513). Every value
// is applied with ApplyOverride: it changes the live config only, Save
// writes the file's own value, and env_overrides reports the variable name.
//
//nolint:gosec // variable names, not credentials
const (
	EnvAuthMethod                = "MUXIMUX_AUTH_METHOD"
	EnvOIDCEnabled               = "MUXIMUX_OIDC_ENABLED"
	EnvOIDCIssuerURL             = "MUXIMUX_OIDC_ISSUER_URL"
	EnvOIDCClientID              = "MUXIMUX_OIDC_CLIENT_ID"
	EnvOIDCClientSecret          = "MUXIMUX_OIDC_CLIENT_SECRET"
	EnvOIDCRedirectURL           = "MUXIMUX_OIDC_REDIRECT_URL"
	EnvOIDCScopes                = "MUXIMUX_OIDC_SCOPES"
	EnvOIDCUsernameClaim         = "MUXIMUX_OIDC_USERNAME_CLAIM"
	EnvOIDCEmailClaim            = "MUXIMUX_OIDC_EMAIL_CLAIM"
	EnvOIDCGroupsClaim           = "MUXIMUX_OIDC_GROUPS_CLAIM"
	EnvOIDCDisplayNameClaim      = "MUXIMUX_OIDC_DISPLAY_NAME_CLAIM"
	EnvOIDCAdminGroups           = "MUXIMUX_OIDC_ADMIN_GROUPS"
	EnvOIDCProviderLogout        = "MUXIMUX_OIDC_PROVIDER_LOGOUT"
	EnvOIDCPostLogoutRedirectURL = "MUXIMUX_OIDC_POST_LOGOUT_REDIRECT_URL"
	EnvOIDCLogoutURL             = "MUXIMUX_OIDC_LOGOUT_URL"
	EnvOIDCAutoRedirect          = "MUXIMUX_OIDC_AUTO_REDIRECT"
	EnvOIDCDisableLocalLogin     = "MUXIMUX_OIDC_DISABLE_LOCAL_LOGIN"

	// envFileSuffix names the Docker-secrets variant of a variable:
	// MUXIMUX_OIDC_CLIENT_SECRET_FILE=/run/secrets/x reads the value from x.
	envFileSuffix = "_FILE"

	// maxEnvFileSize caps a *_FILE read. A client ID or secret is far
	// smaller; anything larger is the wrong file.
	maxEnvFileSize = 64 << 10
)

// envKind says how an OIDC variable's value is parsed.
type envKind int

const (
	envString envKind = iota // trimmed string
	envBool                  // parseEnvBool; invalid warns and keeps the file value
	envScopes                // list split on commas and whitespace
	envCSV                   // list split on commas only (group names may hold spaces)
)

// oidcEnvVar maps one MUXIMUX_OIDC_* variable to its field.
type oidcEnvVar struct {
	name  string
	field OverrideField
	kind  envKind
	// file allows the NAME_FILE variant (Docker secrets).
	file bool
	// configures marks a variable that sets up OIDC sign-in: setting it
	// without MUXIMUX_AUTH_METHOD fails closed (see ApplyAuthEnv). For
	// MUXIMUX_OIDC_ENABLED only the value true counts.
	configures bool
}

// oidcEnvVars lists every MUXIMUX_OIDC_* variable in the order applied.
var oidcEnvVars = []oidcEnvVar{
	{name: EnvOIDCEnabled, field: OverrideOIDCEnabled, kind: envBool, configures: true},
	{name: EnvOIDCIssuerURL, field: OverrideOIDCIssuerURL, kind: envString, configures: true},
	{name: EnvOIDCClientID, field: OverrideOIDCClientID, kind: envString, file: true, configures: true},
	{name: EnvOIDCClientSecret, field: OverrideOIDCClientSecret, kind: envString, file: true, configures: true},
	{name: EnvOIDCRedirectURL, field: OverrideOIDCRedirectURL, kind: envString, configures: true},
	{name: EnvOIDCScopes, field: OverrideOIDCScopes, kind: envScopes},
	{name: EnvOIDCUsernameClaim, field: OverrideOIDCUsernameClaim, kind: envString},
	{name: EnvOIDCEmailClaim, field: OverrideOIDCEmailClaim, kind: envString},
	{name: EnvOIDCGroupsClaim, field: OverrideOIDCGroupsClaim, kind: envString},
	{name: EnvOIDCDisplayNameClaim, field: OverrideOIDCDisplayNameClaim, kind: envString},
	{name: EnvOIDCAdminGroups, field: OverrideOIDCAdminGroups, kind: envCSV},
	{name: EnvOIDCProviderLogout, field: OverrideOIDCProviderLogout, kind: envBool},
	{name: EnvOIDCPostLogoutRedirectURL, field: OverrideOIDCPostLogoutRedirectURL, kind: envString},
	{name: EnvOIDCLogoutURL, field: OverrideOIDCLogoutURL, kind: envString},
	{name: EnvOIDCAutoRedirect, field: OverrideOIDCAutoRedirect, kind: envBool},
	{name: EnvOIDCDisableLocalLogin, field: OverrideOIDCDisableLocalLogin, kind: envBool},
}

// AuthEnvVarNames returns every variable ApplyAuthEnv reads, including the
// *_FILE variants, for documentation and the settings lock.
func AuthEnvVarNames() []string {
	out := []string{EnvAuthMethod}
	for i := range oidcEnvVars {
		out = append(out, oidcEnvVars[i].name)
		if oidcEnvVars[i].file {
			out = append(out, oidcEnvVars[i].name+envFileSuffix)
		}
	}
	return out
}

// validAuthMethods are the values MUXIMUX_AUTH_METHOD accepts.
var validAuthMethods = map[string]bool{"none": true, "builtin": true, "forward_auth": true, "oidc": true}

// envWarning is a WARN raised while the config loads. Load runs before
// logging.Init, so these are kept on the Config and emitted by
// LogEnvWarnings once logging is up. Args must never hold a secret.
type envWarning struct {
	msg  string
	args []any
}

func (c *Config) addEnvWarning(msg string, args ...any) {
	c.envWarnings = append(c.envWarnings, envWarning{msg: msg, args: append([]any{"source", "config"}, args...)})
}

// LogEnvWarnings emits the warnings raised while applying environment
// variables at load and returns how many there were.
func (c *Config) LogEnvWarnings() int {
	for _, w := range c.envWarnings {
		logging.Warn(w.msg, w.args...)
	}
	return len(c.envWarnings)
}

// nonEmptyEnv wraps a lookup so an empty or blank value counts as unset:
// a blank compose entry never clears a configured value.
func nonEmptyEnv(getenv func(string) (string, bool)) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := getenv(name)
		if !ok || strings.TrimSpace(v) == "" {
			return "", false
		}
		return v, true
	}
}

// readEnvFile reads a *_FILE target, at most maxEnvFileSize+1 bytes, so an
// oversized file is detected without reading all of it.
func readEnvFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // the operator names the file
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxEnvFileSize+1))
}

// envOrFile returns the value of name, or of the file named by name_FILE.
// source is the variable that supplied it. ok is false when neither is
// set. Setting both, or a file that is missing, unreadable, too large or
// empty, is an error. Errors name the variable and the path, never the
// file's content. A file value has only its trailing \r and \n removed.
func envOrFile(getenv func(string) (string, bool), readFile func(string) ([]byte, error), name string) (value, source string, ok bool, err error) {
	fileVar := name + envFileSuffix
	direct, hasDirect := getenv(name)
	path, hasFile := getenv(fileVar)
	switch {
	case hasDirect && hasFile:
		return "", "", false, fmt.Errorf("set either %s or %s, not both", name, fileVar)
	case hasDirect:
		return strings.TrimSpace(direct), name, true, nil
	case !hasFile:
		return "", "", false, nil
	}
	path = strings.TrimSpace(path)
	data, err := readFile(path)
	if err != nil {
		// Report the OS cause only: a PathError's own text repeats the
		// path, which is already in the message.
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		return "", "", false, fmt.Errorf("%s: cannot read %s: %w", fileVar, path, err)
	}
	if len(data) > maxEnvFileSize {
		return "", "", false, fmt.Errorf("%s: %s is larger than %d bytes", fileVar, path, maxEnvFileSize)
	}
	v := strings.TrimRight(string(data), "\r\n")
	if v == "" {
		return "", "", false, fmt.Errorf("%s: %s is empty", fileVar, path)
	}
	return v, fileVar, true, nil
}

// splitScopes splits on commas and whitespace and drops empty items.
func splitScopes(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}

// splitCSV splits on commas, trims each item and drops empty ones.
func splitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ApplyAuthEnv applies MUXIMUX_AUTH_METHOD and the MUXIMUX_OIDC_* variables
// on top of the file's values, each recorded with ApplyOverride. getenv and
// readFile are injected so tests need not touch the real environment or
// disk. An empty value counts as unset.
//
// Fatal (returned) errors: an unknown MUXIMUX_AUTH_METHOD; an OIDC
// variable set while MUXIMUX_AUTH_METHOD is not and the method from the
// file is not oidc (fail closed: OIDC from the environment must not leave
// an instance on another method); method oidc with
// MUXIMUX_OIDC_ENABLED=false; and every *_FILE error. An invalid boolean
// only warns and keeps the file value.
func ApplyAuthEnv(cfg *Config, getenv func(string) (string, bool), readFile func(string) ([]byte, error)) error {
	if cfg == nil {
		return nil
	}
	getenv = nonEmptyEnv(getenv)

	method, methodSet := getenv(EnvAuthMethod)
	if methodSet {
		method = strings.ToLower(strings.TrimSpace(method))
		if !validAuthMethods[method] {
			return fmt.Errorf("%s=%q is not a valid auth method; use none, builtin, forward_auth or oidc", EnvAuthMethod, method)
		}
	}

	if set := setOIDCEnvVars(getenv); len(set) > 0 && !methodSet && cfg.Auth.Method != "oidc" {
		return fmt.Errorf("%s set but %s is not, and the configured auth method is %q; set %s=oidc to sign in with OIDC, or remove the OIDC variables",
			strings.Join(set, ", "), EnvAuthMethod, cfg.Auth.Method, EnvAuthMethod)
	}

	if methodSet {
		cfg.ApplyOverride(OverrideAuthMethod, EnvAuthMethod, method)
	}

	for i := range oidcEnvVars {
		if err := applyOIDCEnvVar(cfg, &oidcEnvVars[i], getenv, readFile); err != nil {
			return err
		}
	}

	if cfg.Auth.Method == "oidc" {
		if err := ensureOIDCEnabled(cfg, methodSet); err != nil {
			return err
		}
	}

	if methodSet && method == "oidc" && len(cfg.Auth.OIDC.AdminGroups) == 0 && !hasLocalAdmin(cfg.Auth.Users) {
		cfg.addEnvWarning("OIDC is the auth method but no admin groups are configured and there is no local admin user, so nobody will be an admin; set "+EnvOIDCAdminGroups,
			"env", EnvAuthMethod)
	}
	return nil
}

// setOIDCEnvVars returns the variables (and *_FILE variants) that
// configure OIDC sign-in and are set, in table order. Tuning variables
// (scopes, claims, logout options) and MUXIMUX_OIDC_ENABLED with any value
// other than true do not count: on their own they cannot leave an instance
// expecting OIDC sign-in.
func setOIDCEnvVars(getenv func(string) (string, bool)) []string {
	var set []string
	for i := range oidcEnvVars {
		v := &oidcEnvVars[i]
		if !v.configures {
			continue
		}
		if raw, ok := getenv(v.name); ok {
			if b, valid := parseEnvBool(strings.TrimSpace(raw)); v.kind != envBool || (valid && b) {
				set = append(set, v.name)
			}
		}
		if v.file {
			if _, ok := getenv(v.name + envFileSuffix); ok {
				set = append(set, v.name+envFileSuffix)
			}
		}
	}
	return set
}

// OIDCEnvConfiguringVars returns the variables behind the live OIDC fields
// that configure sign-in (see setOIDCEnvVars), sorted by table order. While
// any is set and MUXIMUX_AUTH_METHOD is not, the next start refuses a
// method other than oidc, so Settings must not switch away from it.
func (c *Config) OIDCEnvConfiguringVars() []string {
	var out []string
	for i := range oidcEnvVars {
		v := &oidcEnvVars[i]
		o, ok := c.overrides[v.field]
		if !v.configures || !ok {
			continue
		}
		if v.field == OverrideOIDCEnabled && (o.source != EnvOIDCEnabled || !c.Auth.OIDC.Enabled) {
			continue
		}
		out = append(out, o.source)
	}
	return out
}

// warnStartupAuthEnv records the auth environment WARNs that describe the
// process environment rather than a config file. Only Load (once, at
// startup) raises them, so a restore through Parse does not repeat them.
func warnStartupAuthEnv(cfg *Config) {
	WarnUnknownOIDCEnv(cfg, os.Environ())
	WarnIgnoredOIDCEnv(cfg, os.LookupEnv)
}

// WarnIgnoredOIDCEnv records one WARN naming the MUXIMUX_OIDC_* variables
// that are set but have no effect under the method MUXIMUX_AUTH_METHOD
// picks: none or forward_auth, or builtin while OIDC is not enabled
// (builtin accepts OIDC sessions alongside passwords when it is). This
// often means a mistyped but still valid method. Not fatal: the operator
// chose the method explicitly. MUXIMUX_OIDC_ENABLED=false is left out, as
// it agrees with the method. Run it after ApplyAuthEnv. Only names are
// logged, never values.
func WarnIgnoredOIDCEnv(cfg *Config, getenv func(string) (string, bool)) {
	if cfg == nil {
		return
	}
	getenv = nonEmptyEnv(getenv)
	method, ok := getenv(EnvAuthMethod)
	method = strings.ToLower(strings.TrimSpace(method))
	if !ok || method == "oidc" || (method == "builtin" && cfg.Auth.OIDC.Enabled) {
		return
	}
	var set []string
	for i := range oidcEnvVars {
		v := &oidcEnvVars[i]
		if raw, ok := getenv(v.name); ok {
			if b, valid := parseEnvBool(strings.TrimSpace(raw)); v.field != OverrideOIDCEnabled || !valid || b {
				set = append(set, v.name)
			}
		}
		if v.file {
			if _, ok := getenv(v.name + envFileSuffix); ok {
				set = append(set, v.name+envFileSuffix)
			}
		}
	}
	if len(set) > 0 {
		cfg.addEnvWarning("OIDC variables have no effect with this "+EnvAuthMethod+"; set "+EnvAuthMethod+"=oidc to sign in with OIDC, or remove them",
			"env", strings.Join(set, ", "), "method", method)
	}
}

// WarnUnknownOIDCEnv records a WARN for each MUXIMUX_OIDC_* entry in
// environ (KEY=VALUE pairs, as os.Environ returns them) that is not a
// known variable, so a misspelt name does not go silently unused. Only
// the name is logged, never the value.
func WarnUnknownOIDCEnv(cfg *Config, environ []string) {
	if cfg == nil {
		return
	}
	known := map[string]bool{}
	for _, n := range AuthEnvVarNames() {
		known[n] = true
	}
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "MUXIMUX_OIDC_") && !known[name] {
			cfg.addEnvWarning("Ignoring unknown environment variable; check the spelling against the documented MUXIMUX_OIDC_* names",
				"env", name)
		}
	}
}

// applyOIDCEnvVar applies one variable. Values are never logged.
func applyOIDCEnvVar(cfg *Config, v *oidcEnvVar, getenv func(string) (string, bool), readFile func(string) ([]byte, error)) error {
	raw, source, ok := "", v.name, false
	if v.file {
		var err error
		raw, source, ok, err = envOrFile(getenv, readFile, v.name)
		if err != nil {
			return err
		}
	} else {
		raw, ok = getenv(v.name)
		raw = strings.TrimSpace(raw)
	}
	if !ok {
		return nil
	}
	switch v.kind {
	case envBool:
		b, valid := parseEnvBool(raw)
		if !valid {
			// A boolean is not secret, so echoing it is safe.
			cfg.addEnvWarning("Ignoring invalid boolean in environment variable; using the config file value",
				"env", v.name, "value", raw)
			return nil
		}
		cfg.ApplyOverride(v.field, source, strconv.FormatBool(b))
	case envScopes, envCSV:
		list := splitCSV(raw)
		if v.kind == envScopes {
			list = splitScopes(raw)
		}
		if len(list) == 0 {
			return nil // only separators: same as unset
		}
		cfg.ApplyOverride(v.field, source, encodeList(list))
	default:
		cfg.ApplyOverride(v.field, source, raw)
	}
	return nil
}

// ensureOIDCEnabled makes method oidc imply auth.oidc.enabled, as choosing
// oidc in Settings does. When the method came from MUXIMUX_AUTH_METHOD and
// MUXIMUX_OIDC_ENABLED is unset, enabled is overridden to true with
// MUXIMUX_AUTH_METHOD as its source. MUXIMUX_OIDC_ENABLED=false with method
// oidc would leave nobody able to sign in, so it is fatal.
func ensureOIDCEnabled(cfg *Config, methodSet bool) error {
	if cfg.IsOverridden(OverrideOIDCEnabled) {
		if !cfg.Auth.OIDC.Enabled {
			methodSrc := "auth.method in the config file"
			if methodSet {
				methodSrc = EnvAuthMethod
			}
			return fmt.Errorf("%s=false contradicts auth method oidc (from %s); remove %s or choose another method",
				EnvOIDCEnabled, methodSrc, EnvOIDCEnabled)
		}
		return nil
	}
	if methodSet {
		cfg.ApplyOverride(OverrideOIDCEnabled, EnvAuthMethod, "true")
	}
	return nil
}

// hasLocalAdmin reports whether any configured local user is an admin.
func hasLocalAdmin(users []UserConfig) bool {
	for i := range users {
		if users[i].Role == "admin" {
			return true
		}
	}
	return false
}

// authEnvConfigured reports whether auth.method or any auth.oidc field
// came from the environment.
func (c *Config) authEnvConfigured() bool {
	if c.IsOverridden(OverrideAuthMethod) {
		return true
	}
	for i := range oidcEnvVars {
		if c.IsOverridden(oidcEnvVars[i].field) {
			return true
		}
	}
	return false
}

// fieldLabel names a config field in an error: "VAR (path)" when the live
// value came from a flag or environment variable, the YAML path otherwise.
// It never includes the value.
func (c *Config) fieldLabel(path string) string {
	if o, ok := c.overrides[OverrideField(path)]; ok {
		return o.source + " (" + path + ")"
	}
	return path
}

// validateAuthEnv adds the checks that apply only when auth is configured
// from the environment, so a YAML-only config never starts failing: with
// method oidc and OIDC enabled, issuer_url must be an absolute http(s) URL
// and client_id must be set.
func (c *Config) validateAuthEnv() error {
	o := &c.Auth.OIDC
	if !c.authEnvConfigured() || c.Auth.Method != "oidc" || !o.Enabled {
		return nil
	}
	if o.IssuerURL == "" {
		return fmt.Errorf("auth.oidc.issuer_url is required for OIDC configured from the environment; set %s", EnvOIDCIssuerURL)
	}
	if !isAbsHTTPURL(o.IssuerURL) {
		return fmt.Errorf("%s must be an absolute http(s) URL", c.fieldLabel(string(OverrideOIDCIssuerURL)))
	}
	if o.ClientID == "" {
		return fmt.Errorf("auth.oidc.client_id is required for OIDC configured from the environment; set %s or %s",
			EnvOIDCClientID, EnvOIDCClientID+envFileSuffix)
	}
	return nil
}

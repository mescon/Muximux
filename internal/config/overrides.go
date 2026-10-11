package config

import (
	"encoding/json"
	"strconv"
)

// OverrideField names a config field that a command-line flag or an
// environment variable can override at startup. An override changes the
// live value only: Save writes the value the file had, so a temporary
// MUXIMUX_LOG_LEVEL=debug never ends up in config.yaml.
type OverrideField string

// Fields that can be overridden from a flag or the environment.
//
//nolint:gosec // field paths, not credentials
const (
	OverrideLogLevel   OverrideField = "log_level"
	OverrideLogFormat  OverrideField = "log_format"
	OverrideListen     OverrideField = "listen"
	OverrideBasePath   OverrideField = "base_path"
	OverrideAutoImport OverrideField = "discovery.docker.auto_import"

	OverrideRequireExplicitEnable OverrideField = "discovery.docker.require_explicit_enable"

	// Auth fields set from MUXIMUX_AUTH_METHOD and MUXIMUX_OIDC_* (#513).
	// Named by YAML path so env_overrides keys match the config file.
	OverrideAuthMethod                OverrideField = "auth.method"
	OverrideOIDCEnabled               OverrideField = "auth.oidc.enabled"
	OverrideOIDCIssuerURL             OverrideField = "auth.oidc.issuer_url"
	OverrideOIDCClientID              OverrideField = "auth.oidc.client_id"
	OverrideOIDCClientSecret          OverrideField = "auth.oidc.client_secret"
	OverrideOIDCRedirectURL           OverrideField = "auth.oidc.redirect_url"
	OverrideOIDCScopes                OverrideField = "auth.oidc.scopes"
	OverrideOIDCUsernameClaim         OverrideField = "auth.oidc.username_claim"
	OverrideOIDCEmailClaim            OverrideField = "auth.oidc.email_claim"
	OverrideOIDCGroupsClaim           OverrideField = "auth.oidc.groups_claim"
	OverrideOIDCDisplayNameClaim      OverrideField = "auth.oidc.display_name_claim"
	OverrideOIDCAdminGroups           OverrideField = "auth.oidc.admin_groups"
	OverrideOIDCProviderLogout        OverrideField = "auth.oidc.provider_logout"
	OverrideOIDCPostLogoutRedirectURL OverrideField = "auth.oidc.post_logout_redirect_url"
	OverrideOIDCLogoutURL             OverrideField = "auth.oidc.logout_url"
	OverrideOIDCAutoRedirect          OverrideField = "auth.oidc.auto_redirect"
	OverrideOIDCDisableLocalLogin     OverrideField = "auth.oidc.disable_local_login"
)

// override remembers where an overridden value came from and the value
// the file held before the override replaced it. List fields are encoded
// as JSON (see encodeList), so a nil list and an empty list round-trip.
type override struct {
	source    string
	fileValue string
}

// ApplyOverride sets the live value of field and remembers the file's own
// value (recorded once, the first time a field is overridden) and the source
// (env var or flag name) for GET /api/config.
func (c *Config) ApplyOverride(field OverrideField, source, value string) {
	if c.overrides == nil {
		c.overrides = map[OverrideField]override{}
	}
	if o, done := c.overrides[field]; done {
		o.source = source
		c.overrides[field] = o
	} else {
		c.overrides[field] = override{source: source, fileValue: c.overrideValue(field)}
	}
	c.setOverrideValue(field, value)
}

// IsOverridden reports whether field's live value came from a flag or the
// environment rather than from config.yaml.
func (c *Config) IsOverridden(field OverrideField) bool {
	_, ok := c.overrides[field]
	return ok
}

// OverrideSource returns the flag or environment variable that set field,
// and whether field is overridden.
func (c *Config) OverrideSource(field OverrideField) (string, bool) {
	o, ok := c.overrides[field]
	return o.source, ok
}

// SetOverrideFileValue changes the value Save writes for an overridden
// field, without touching the live value or the source. It returns the
// previous file value, so a caller can put it back if the save fails, and
// false (changing nothing) when field is not overridden. The setup wizard
// uses it to persist the method it completed under MUXIMUX_AUTH_METHOD, so
// dropping the variable later does not leave the instance without auth.
func (c *Config) SetOverrideFileValue(field OverrideField, value string) (string, bool) {
	o, ok := c.overrides[field]
	if !ok {
		return "", false
	}
	prev := o.fileValue
	o.fileValue = value
	c.overrides[field] = o
	return prev, true
}

// EnvOverrides maps each overridden field to the flag or environment
// variable that set it. It returns nil when nothing is overridden.
func (c *Config) EnvOverrides() map[string]string {
	if len(c.overrides) == 0 {
		return nil
	}
	out := make(map[string]string, len(c.overrides))
	for field, o := range c.overrides {
		out[string(field)] = o.source
	}
	return out
}

// overrideValue returns the current value of field.
func (c *Config) overrideValue(field OverrideField) string {
	switch field {
	case OverrideLogLevel:
		return c.Server.LogLevel
	case OverrideLogFormat:
		return c.Server.LogFormat
	case OverrideListen:
		return c.Server.Listen
	case OverrideBasePath:
		return c.Server.BasePath
	case OverrideAutoImport:
		return string(c.Discovery.Docker.AutoImport)
	case OverrideRequireExplicitEnable:
		return strconv.FormatBool(c.Discovery.Docker.RequireExplicitEnable)
	}
	if p := c.stringOverrideField(field); p != nil {
		return *p
	}
	if p := c.boolOverrideField(field); p != nil {
		return strconv.FormatBool(*p)
	}
	if p := c.listOverrideField(field); p != nil {
		return encodeList(*p)
	}
	return ""
}

// setOverrideValue sets the current value of field. Unknown fields are
// ignored.
func (c *Config) setOverrideValue(field OverrideField, value string) {
	switch field {
	case OverrideLogLevel:
		c.Server.LogLevel = value
	case OverrideLogFormat:
		c.Server.LogFormat = value
	case OverrideListen:
		c.Server.Listen = value
	case OverrideBasePath:
		c.Server.BasePath = value
	case OverrideAutoImport:
		c.Discovery.Docker.AutoImport = AutoImportMode(value)
	case OverrideRequireExplicitEnable:
		c.Discovery.Docker.RequireExplicitEnable = value == "true"
	}
	if p := c.stringOverrideField(field); p != nil {
		*p = value
	}
	if p := c.boolOverrideField(field); p != nil {
		*p = value == "true"
	}
	if p := c.listOverrideField(field); p != nil {
		*p = decodeList(value)
	}
}

// stringOverrideField returns the address of a string auth field, or nil.
func (c *Config) stringOverrideField(field OverrideField) *string {
	o := &c.Auth.OIDC
	switch field {
	case OverrideAuthMethod:
		return &c.Auth.Method
	case OverrideOIDCIssuerURL:
		return &o.IssuerURL
	case OverrideOIDCClientID:
		return &o.ClientID
	case OverrideOIDCClientSecret:
		return &o.ClientSecret
	case OverrideOIDCRedirectURL:
		return &o.RedirectURL
	case OverrideOIDCUsernameClaim:
		return &o.UsernameClaim
	case OverrideOIDCEmailClaim:
		return &o.EmailClaim
	case OverrideOIDCGroupsClaim:
		return &o.GroupsClaim
	case OverrideOIDCDisplayNameClaim:
		return &o.DisplayNameClaim
	case OverrideOIDCPostLogoutRedirectURL:
		return &o.PostLogoutRedirectURL
	case OverrideOIDCLogoutURL:
		return &o.LogoutURL
	}
	return nil
}

// boolOverrideField returns the address of a bool OIDC field, or nil.
func (c *Config) boolOverrideField(field OverrideField) *bool {
	o := &c.Auth.OIDC
	switch field {
	case OverrideOIDCEnabled:
		return &o.Enabled
	case OverrideOIDCProviderLogout:
		return &o.ProviderLogout
	case OverrideOIDCAutoRedirect:
		return &o.AutoRedirect
	case OverrideOIDCDisableLocalLogin:
		return &o.DisableLocalLogin
	}
	return nil
}

// listOverrideField returns the address of a list OIDC field, or nil.
func (c *Config) listOverrideField(field OverrideField) *[]string {
	switch field {
	case OverrideOIDCScopes:
		return &c.Auth.OIDC.Scopes
	case OverrideOIDCAdminGroups:
		return &c.Auth.OIDC.AdminGroups
	}
	return nil
}

// encodeList stores a list field as an override string. JSON keeps nil
// ("null") apart from an empty list ("[]"), so Save writes back exactly
// what the file held.
func encodeList(v []string) string {
	b, _ := json.Marshal(v) //nolint:errcheck // a []string always marshals
	return string(b)
}

// decodeList is the inverse of encodeList. A value that is not a JSON
// list decodes to nil.
func decodeList(s string) []string {
	var v []string
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

// fileView returns a shallow copy of c with every overridden field set
// back to the value the file held and every quarantined app and gateway
// site appended, which is what Save writes. The copy
// keeps c's recorded ${VAR} references, so a field that was a reference in
// the file is written back as that reference.
func (c *Config) fileView() *Config {
	if len(c.overrides) == 0 && len(c.quarantined) == 0 {
		return c
	}
	v := *c
	for field, o := range c.overrides {
		v.setOverrideValue(field, o.fileValue)
	}
	if len(c.quarantined) > 0 {
		// Fresh slices: appending to c.Apps directly could write the
		// quarantined entries into the live slice's spare capacity.
		// fileView does not change c: Save drops the superseded entries
		// from memory only after the write succeeds.
		qApps, qSites := c.quarantinedFileEntries()
		v.Apps = append(append([]AppConfig(nil), c.Apps...), qApps...)
		v.Server.GatewaySites = append(append([]GatewaySite(nil), c.Server.GatewaySites...), qSites...)
	}
	return &v
}

// InheritRuntime carries the live runtime state of prev onto c: the save hook
// and every recorded override (c records its own parsed value as the file value,
// the live value stays prev's override). A config parsed for a restore uses it
// so the restored file replaces only what config.yaml holds.
func (c *Config) InheritRuntime(prev *Config) {
	if prev == nil {
		return
	}
	c.onSaved = prev.onSaved
	for field, o := range prev.overrides {
		c.ApplyOverride(field, o.source, prev.overrideValue(field))
	}
}

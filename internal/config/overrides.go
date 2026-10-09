package config

// OverrideField names a config field that a command-line flag or an
// environment variable can override at startup. An override changes the
// live value only: Save writes the value the file had, so a temporary
// MUXIMUX_LOG_LEVEL=debug never ends up in config.yaml.
type OverrideField string

// Fields that can be overridden from a flag or the environment.
const (
	OverrideLogLevel   OverrideField = "log_level"
	OverrideLogFormat  OverrideField = "log_format"
	OverrideListen     OverrideField = "listen"
	OverrideBasePath   OverrideField = "base_path"
	OverrideAutoImport OverrideField = "discovery.docker.auto_import"
)

// override remembers where an overridden value came from and the value
// the file held before the override replaced it.
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
	}
}

// fileView returns a shallow copy of c with every overridden field set
// back to the value the file held, which is what Save writes. The copy
// keeps c's recorded ${VAR} references, so a field that was a reference in
// the file is written back as that reference.
func (c *Config) fileView() *Config {
	if len(c.overrides) == 0 {
		return c
	}
	v := *c
	for field, o := range c.overrides {
		v.setOverrideValue(field, o.fileValue)
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

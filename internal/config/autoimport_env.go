package config

import (
	"strconv"
	"strings"

	"github.com/mescon/muximux/v3/internal/logging"
)

// EnvAutoImport is the direct override for discovery.docker.auto_import.
const EnvAutoImport = "MUXIMUX_DISCOVERY_AUTO_IMPORT"

// ApplyAutoImportEnv overrides the configured auto-import mode from the
// environment when EnvAutoImport is set. The value is normalized (unknown
// -> off) and recorded as an override, so Save keeps the file's own mode.
// getenv is injected so tests need not touch the real environment.
func ApplyAutoImportEnv(cfg *Config, getenv func(string) (string, bool)) {
	if cfg == nil {
		return
	}
	if v, ok := getenv(EnvAutoImport); ok {
		cfg.ApplyOverride(OverrideAutoImport, EnvAutoImport, string(NormalizeAutoImport(AutoImportMode(v))))
	}
}

// EnvRequireExplicitEnable is the direct override for
// discovery.docker.require_explicit_enable.
const EnvRequireExplicitEnable = "MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE"

// ApplyRequireExplicitEnableEnv overrides require_explicit_enable from the
// environment when EnvRequireExplicitEnable is set to a recognised boolean.
// It is recorded as an override, so Save keeps the file's own value. An
// unrecognised value is ignored with a warning and the file value stays.
func ApplyRequireExplicitEnableEnv(cfg *Config, getenv func(string) (string, bool)) {
	if cfg == nil {
		return
	}
	v, ok := getenv(EnvRequireExplicitEnable)
	if !ok {
		return
	}
	val, valid := parseEnvBool(v)
	if !valid {
		logging.Warn("Ignoring invalid boolean in environment variable; using the config file value",
			"source", "config", "env", EnvRequireExplicitEnable, "value", v)
		return
	}
	cfg.ApplyOverride(OverrideRequireExplicitEnable, EnvRequireExplicitEnable, strconv.FormatBool(val))
}

// parseEnvBool parses true/1/yes/on and false/0/no/off (case-insensitive,
// trimmed). ok is false for anything else. It duplicates discovery.boolish
// because config cannot import discovery.
func parseEnvBool(v string) (val, ok bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true, true
	case "false", "0", "no", "off":
		return false, true
	}
	return false, false
}

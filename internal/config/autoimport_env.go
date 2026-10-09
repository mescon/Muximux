package config

import (
	"strconv"
	"strings"
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
// environment when EnvRequireExplicitEnable is set. It is recorded as an
// override, so Save keeps the file's own value.
func ApplyRequireExplicitEnableEnv(cfg *Config, getenv func(string) (string, bool)) {
	if cfg == nil {
		return
	}
	if v, ok := getenv(EnvRequireExplicitEnable); ok {
		cfg.ApplyOverride(OverrideRequireExplicitEnable, EnvRequireExplicitEnable, strconv.FormatBool(envBool(v)))
	}
}

// envBool reports whether v is true, 1, yes or on (case-insensitive,
// trimmed). It duplicates discovery.boolish because config cannot import
// discovery.
func envBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

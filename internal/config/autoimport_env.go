package config

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

package config

import "testing"

func TestApplyAutoImportEnv(t *testing.T) {
	cfg := &Config{}
	cfg.Discovery.Docker.AutoImport = AutoImportOff

	// env set: overrides yaml
	ApplyAutoImportEnv(cfg, func(k string) (string, bool) {
		if k == "MUXIMUX_DISCOVERY_AUTO_IMPORT" {
			return "sync", true
		}
		return "", false
	})
	if cfg.Discovery.Docker.AutoImport != AutoImportSync {
		t.Fatalf("env override failed: got %q", cfg.Discovery.Docker.AutoImport)
	}

	// env absent: yaml value preserved
	cfg.Discovery.Docker.AutoImport = AutoImportAdd
	ApplyAutoImportEnv(cfg, func(string) (string, bool) { return "", false })
	if cfg.Discovery.Docker.AutoImport != AutoImportAdd {
		t.Fatalf("absent env should preserve yaml: got %q", cfg.Discovery.Docker.AutoImport)
	}

	// env garbage: fails closed to off
	ApplyAutoImportEnv(cfg, func(string) (string, bool) { return "nonsense", true })
	if cfg.Discovery.Docker.AutoImport != AutoImportOff {
		t.Fatalf("garbage env should fail closed: got %q", cfg.Discovery.Docker.AutoImport)
	}

	// nil cfg: guarded, must not panic
	ApplyAutoImportEnv(nil, func(string) (string, bool) { return "sync", true })
}

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

func TestApplyRequireExplicitEnableEnv_NilConfig(t *testing.T) {
	ApplyRequireExplicitEnableEnv(nil, func(string) (string, bool) { return "true", true })
}

func envMap(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestApplyAutoImportEnv_BlankCountsAsUnset(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t\n"} {
		cfg := &Config{}
		cfg.Discovery.Docker.AutoImport = AutoImportSync
		ApplyAutoImportEnv(cfg, envMap(map[string]string{EnvAutoImport: blank}))
		if cfg.Discovery.Docker.AutoImport != AutoImportSync || cfg.IsOverridden(OverrideAutoImport) {
			t.Errorf("blank %q forced %q", blank, cfg.Discovery.Docker.AutoImport)
		}
	}
}

func TestApplyRequireExplicitEnableEnv_BlankCountsAsUnset(t *testing.T) {
	for _, blank := range []string{"", "  "} {
		cfg := &Config{}
		cfg.Discovery.Docker.RequireExplicitEnable = true
		ApplyRequireExplicitEnableEnv(cfg, envMap(map[string]string{EnvRequireExplicitEnable: blank}))
		if !cfg.Discovery.Docker.RequireExplicitEnable || cfg.IsOverridden(OverrideRequireExplicitEnable) {
			t.Errorf("blank %q changed the value", blank)
		}
	}
}

func TestEnvValue(t *testing.T) {
	t.Setenv("MUXIMUX_TEST_VALUE", "  x  ")
	if v, ok := EnvValue("MUXIMUX_TEST_VALUE"); !ok || v != "x" {
		t.Errorf("EnvValue = %q, %v", v, ok)
	}
	for _, blank := range []string{"", " \t"} {
		t.Setenv("MUXIMUX_TEST_VALUE", blank)
		if _, ok := EnvValue("MUXIMUX_TEST_VALUE"); ok {
			t.Errorf("blank %q counted as set", blank)
		}
	}
	if _, ok := EnvValue("MUXIMUX_TEST_UNSET_VALUE"); ok {
		t.Error("unset counted as set")
	}
}

func TestParseBool_EmptyIsInvalid(t *testing.T) {
	for _, v := range []string{"", "  ", "tru", "2"} {
		if _, ok := ParseBool(v); ok {
			t.Errorf("ParseBool(%q) ok = true", v)
		}
	}
}

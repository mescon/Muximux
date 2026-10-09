package config

import (
	"os"
	"strings"
	"testing"
)

const overrideTestYAML = `server:
  title: Dash
  listen: ":8080"
  log_level: info
  log_format: text
discovery:
  docker:
    enabled: true
    auto_import: add
`

// S-13: an environment override changes the live value only; Save writes
// the file's own value.
func TestApplyOverride_SaveWritesFileValue(t *testing.T) {
	cfg, path := loadWithEnv(t, overrideTestYAML, nil)
	cfg.ApplyOverride(OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")
	if cfg.Server.LogLevel != "debug" {
		t.Fatalf("live log level = %q, want debug", cfg.Server.LogLevel)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Save changed the live log level to %q", cfg.Server.LogLevel)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Server.LogLevel != "info" {
		t.Errorf("saved log level = %q, want info", reloaded.Server.LogLevel)
	}
	if got := cfg.EnvOverrides()["log_level"]; got != "MUXIMUX_LOG_LEVEL" {
		t.Errorf("EnvOverrides()[log_level] = %q, want MUXIMUX_LOG_LEVEL", got)
	}
}

func TestApplyAutoImportEnv_RecordsOverride(t *testing.T) {
	// Unset the real variable (restored after the test) so Load's own
	// lookup records nothing and only the injected getenv applies.
	t.Setenv(EnvAutoImport, "")
	os.Unsetenv(EnvAutoImport)
	cfg, path := loadWithEnv(t, overrideTestYAML, nil)
	if cfg.IsOverridden(OverrideAutoImport) || cfg.Discovery.Docker.AutoImport != AutoImportAdd {
		t.Fatalf("precondition: loaded auto-import %q", cfg.Discovery.Docker.AutoImport)
	}

	ApplyAutoImportEnv(cfg, func(k string) (string, bool) {
		if k == EnvAutoImport {
			return "sync", true
		}
		return "", false
	})
	if !cfg.IsOverridden(OverrideAutoImport) {
		t.Fatal("auto-import override not recorded")
	}
	if cfg.Discovery.Docker.AutoImport != AutoImportSync {
		t.Fatalf("live auto-import = %q, want sync", cfg.Discovery.Docker.AutoImport)
	}
	if got := cfg.EnvOverrides()[string(OverrideAutoImport)]; got != EnvAutoImport {
		t.Errorf("source = %q, want %s", got, EnvAutoImport)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "auto_import: add") {
		t.Errorf("saved file lost the file's auto_import add:\n%s", out)
	}
	if cfg.Discovery.Docker.AutoImport != AutoImportSync {
		t.Errorf("Save changed the live auto-import to %q", cfg.Discovery.Docker.AutoImport)
	}
}

func TestApplyOverride_SecondCallKeepsOriginalFileValue(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Listen = ":8080"
	cfg.ApplyOverride(OverrideListen, "MUXIMUX_LISTEN", ":9090")
	cfg.ApplyOverride(OverrideListen, "--listen", ":7070")

	if cfg.Server.Listen != ":7070" {
		t.Errorf("live listen = %q, want :7070", cfg.Server.Listen)
	}
	if got := cfg.EnvOverrides()["listen"]; got != "--listen" {
		t.Errorf("source = %q, want --listen", got)
	}
	if got := cfg.fileView().Server.Listen; got != ":8080" {
		t.Errorf("file value = %q, want the first recorded :8080", got)
	}
}

func TestApplyOverride_AllFields(t *testing.T) {
	cfg := &Config{}
	cfg.Server.LogLevel = "info"
	cfg.Server.LogFormat = "text"
	cfg.Server.Listen = ":8080"
	cfg.Server.BasePath = "/dash"
	cfg.Discovery.Docker.AutoImport = AutoImportOff

	cfg.ApplyOverride(OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")
	cfg.ApplyOverride(OverrideLogFormat, "MUXIMUX_LOG_FORMAT", "json")
	cfg.ApplyOverride(OverrideListen, "--listen", ":9090")
	cfg.ApplyOverride(OverrideBasePath, "MUXIMUX_BASE_PATH", "/mux")
	cfg.ApplyOverride(OverrideAutoImport, EnvAutoImport, "update")

	if cfg.Server.LogLevel != "debug" || cfg.Server.LogFormat != "json" ||
		cfg.Server.Listen != ":9090" || cfg.Server.BasePath != "/mux" ||
		cfg.Discovery.Docker.AutoImport != AutoImportUpdate {
		t.Fatalf("live values not applied: %+v %q", cfg.Server, cfg.Discovery.Docker.AutoImport)
	}
	v := cfg.fileView()
	if v == cfg {
		t.Fatal("fileView must not return the live config when fields are overridden")
	}
	if v.Server.LogLevel != "info" || v.Server.LogFormat != "text" ||
		v.Server.Listen != ":8080" || v.Server.BasePath != "/dash" ||
		v.Discovery.Docker.AutoImport != AutoImportOff {
		t.Errorf("file view wrong: %+v %q", v.Server, v.Discovery.Docker.AutoImport)
	}
	if len(cfg.EnvOverrides()) != 5 {
		t.Errorf("EnvOverrides() = %v, want 5 entries", cfg.EnvOverrides())
	}
}

func TestApplyOverride_NoneRecorded(t *testing.T) {
	cfg := &Config{}
	if cfg.IsOverridden(OverrideLogLevel) {
		t.Error("IsOverridden true with no overrides")
	}
	if cfg.EnvOverrides() != nil {
		t.Errorf("EnvOverrides() = %v, want nil", cfg.EnvOverrides())
	}
	if cfg.fileView() != cfg {
		t.Error("fileView should return c itself when nothing is overridden")
	}
}

func TestApplyOverride_UnknownFieldIgnored(t *testing.T) {
	cfg := &Config{}
	cfg.Server.LogLevel = "info"
	cfg.ApplyOverride(OverrideField("nope"), "X", "y")
	if cfg.overrideValue(OverrideField("nope")) != "" {
		t.Error("unknown field should read as empty")
	}
	if cfg.Server.LogLevel != "info" {
		t.Error("unknown field changed a real field")
	}
}

// An overridden field that the file wrote as a ${VAR} reference is saved
// back as the reference, not the override and not the expanded value.
func TestApplyOverride_KeepsEnvReference(t *testing.T) {
	cfg, path := loadWithEnv(t, `server:
  title: Dash
  log_level: ${LVL}
`, map[string]string{"LVL": "warn"})
	cfg.ApplyOverride(OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	if !strings.Contains(s, "log_level: ${LVL}") {
		t.Errorf("saved file lost the ${LVL} reference:\n%s", s)
	}
	if strings.Contains(s, "debug") {
		t.Errorf("saved file contains the override:\n%s", s)
	}
	// A second save still matches the re-recorded reference.
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	out, _ = os.ReadFile(path)
	if !strings.Contains(string(out), "log_level: ${LVL}") {
		t.Errorf("second save lost the ${LVL} reference:\n%s", out)
	}
}

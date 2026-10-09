package config

import (
	"os"
	"path/filepath"
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

func TestInheritRuntime_KeepsOverridesAndHook(t *testing.T) {
	prev, err := Parse([]byte("server: {listen: \":8080\", log_level: info}\n"))
	if err != nil {
		t.Fatal(err)
	}
	prev.ApplyOverride(OverrideLogLevel, "MUXIMUX_LOG_LEVEL", "debug")
	calls := 0
	prev.SetOnSaved(func() { calls++ })

	next, err := Parse([]byte("server: {listen: \":8080\", log_level: warn}\n"))
	if err != nil {
		t.Fatal(err)
	}
	next.InheritRuntime(prev)

	if next.Server.LogLevel != "debug" {
		t.Errorf("live log level = %q, want debug", next.Server.LogLevel)
	}
	if got := next.EnvOverrides()["log_level"]; got != "MUXIMUX_LOG_LEVEL" {
		t.Errorf("EnvOverrides()[log_level] = %q, want MUXIMUX_LOG_LEVEL", got)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := next.Save(path); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Server.LogLevel != "warn" {
		t.Errorf("saved log level = %q, want warn (the restored file's value)", reloaded.Server.LogLevel)
	}
	if calls != 1 {
		t.Errorf("save hook calls = %d, want 1", calls)
	}
}

// The auto-import override is recorded by Parse itself when the variable is
// set; inheriting it again keeps the restored file's own value for Save.
func TestInheritRuntime_AutoImportKeepsRestoredFileValue(t *testing.T) {
	t.Setenv(EnvAutoImport, "sync")
	prev, err := Parse([]byte(overrideTestYAML))
	if err != nil {
		t.Fatal(err)
	}
	next, err := Parse([]byte(strings.Replace(overrideTestYAML, "auto_import: add", "auto_import: off", 1)))
	if err != nil {
		t.Fatal(err)
	}
	next.InheritRuntime(prev)
	if next.Discovery.Docker.AutoImport != AutoImportSync {
		t.Errorf("live auto-import = %q, want sync", next.Discovery.Docker.AutoImport)
	}
	if got := next.fileView().Discovery.Docker.AutoImport; got != AutoImportOff {
		t.Errorf("file auto-import = %q, want off", got)
	}
}

func TestInheritRuntime_NilPrev(t *testing.T) {
	next, err := Parse([]byte("server: {listen: \":8080\", log_level: warn}\n"))
	if err != nil {
		t.Fatal(err)
	}
	next.InheritRuntime(nil)
	if next.Server.LogLevel != "warn" || next.IsOverridden(OverrideLogLevel) {
		t.Errorf("nil prev changed the config: %q", next.Server.LogLevel)
	}
}

// A field absent from the file and set only by an override is not written
// into the file by a save: the file keeps what it had (absent, or the
// load default it decodes to), never the override's value.
func TestApplyOverride_AbsentInFileStaysOutOfFile(t *testing.T) {
	cfg, path := loadWithEnv(t, "server:\n  title: Dash\n", nil)
	before := cfg.Server.LogFormat
	cfg.ApplyOverride(OverrideLogFormat, "MUXIMUX_LOG_FORMAT", "json")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "json") {
		t.Errorf("override value written to the file:\n%s", data)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Server.LogFormat != before {
		t.Errorf("saved log_format = %q, want the file's own %q", reloaded.Server.LogFormat, before)
	}
	if cfg.Server.LogFormat != "json" {
		t.Errorf("live log_format = %q, want json", cfg.Server.LogFormat)
	}
}

func TestApplyRequireExplicitEnableEnv_RecordsOverride(t *testing.T) {
	cfg := defaultConfig()
	getenv := func(k string) (string, bool) { return "yes", k == EnvRequireExplicitEnable }
	ApplyRequireExplicitEnableEnv(cfg, getenv)
	if !cfg.Discovery.Docker.RequireExplicitEnable || !cfg.IsOverridden(OverrideRequireExplicitEnable) {
		t.Fatalf("live=%v overridden=%v", cfg.Discovery.Docker.RequireExplicitEnable, cfg.IsOverridden(OverrideRequireExplicitEnable))
	}
	if cfg.EnvOverrides()[string(OverrideRequireExplicitEnable)] != EnvRequireExplicitEnable {
		t.Fatalf("EnvOverrides = %v", cfg.EnvOverrides())
	}
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "require_explicit_enable: true") {
		t.Fatalf("override written to file:\n%s", raw)
	}
}

func TestApplyRequireExplicitEnableEnv_FalseAndUnset(t *testing.T) {
	cfg := defaultConfig()
	cfg.Discovery.Docker.RequireExplicitEnable = true
	ApplyRequireExplicitEnableEnv(cfg, func(string) (string, bool) { return "0", true })
	if cfg.Discovery.Docker.RequireExplicitEnable {
		t.Fatal("env=0 should turn it off")
	}
	cfg2 := defaultConfig()
	cfg2.Discovery.Docker.RequireExplicitEnable = true
	ApplyRequireExplicitEnableEnv(cfg2, func(string) (string, bool) { return "", false })
	if !cfg2.Discovery.Docker.RequireExplicitEnable || cfg2.IsOverridden(OverrideRequireExplicitEnable) {
		t.Fatal("unset env must not touch the file value")
	}
}

func TestParseEnvBool(t *testing.T) {
	for _, v := range []string{"true", "1", "YES", " on "} {
		if val, ok := parseEnvBool(v); !val || !ok {
			t.Errorf("parseEnvBool(%q) = %v, %v", v, val, ok)
		}
	}
	for _, v := range []string{"false", "0", "No", " OFF "} {
		if val, ok := parseEnvBool(v); val || !ok {
			t.Errorf("parseEnvBool(%q) = %v, %v", v, val, ok)
		}
	}
	for _, v := range []string{"", "nope", "2", "tru"} {
		if _, ok := parseEnvBool(v); ok {
			t.Errorf("parseEnvBool(%q) ok = true", v)
		}
	}
}

func TestApplyRequireExplicitEnableEnv_OffAndNo(t *testing.T) {
	for _, v := range []string{"off", "no"} {
		cfg := defaultConfig()
		cfg.Discovery.Docker.RequireExplicitEnable = true
		ApplyRequireExplicitEnableEnv(cfg, func(string) (string, bool) { return v, true })
		if cfg.Discovery.Docker.RequireExplicitEnable || !cfg.IsOverridden(OverrideRequireExplicitEnable) {
			t.Errorf("%q: live=%v overridden=%v", v, cfg.Discovery.Docker.RequireExplicitEnable, cfg.IsOverridden(OverrideRequireExplicitEnable))
		}
	}
}

func TestApplyRequireExplicitEnableEnv_InvalidIgnored(t *testing.T) {
	cfg := defaultConfig()
	cfg.Discovery.Docker.RequireExplicitEnable = true
	ApplyRequireExplicitEnableEnv(cfg, func(string) (string, bool) { return "maybe", true })
	if !cfg.Discovery.Docker.RequireExplicitEnable {
		t.Error("invalid value changed the file value")
	}
	if cfg.IsOverridden(OverrideRequireExplicitEnable) || cfg.EnvOverrides() != nil {
		t.Errorf("invalid value recorded an override: %v", cfg.EnvOverrides())
	}
}

func TestInheritRuntime_KeepsRequireExplicitEnableOverride(t *testing.T) {
	t.Setenv(EnvRequireExplicitEnable, "true")
	prev, err := Parse([]byte("discovery:\n  docker:\n    enabled: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvRequireExplicitEnable, "")
	os.Unsetenv(EnvRequireExplicitEnable)
	next, err := Parse([]byte("discovery:\n  docker:\n    enabled: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	next.InheritRuntime(prev)
	if !next.Discovery.Docker.RequireExplicitEnable || !next.IsOverridden(OverrideRequireExplicitEnable) {
		t.Fatalf("live=%v overridden=%v", next.Discovery.Docker.RequireExplicitEnable, next.IsOverridden(OverrideRequireExplicitEnable))
	}
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := next.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "require_explicit_enable: true") {
		t.Fatalf("override written to file:\n%s", raw)
	}
}

func TestParse_AppliesRequireExplicitEnableEnv(t *testing.T) {
	t.Setenv(EnvRequireExplicitEnable, "true")
	cfg, err := Parse([]byte("discovery:\n  docker:\n    enabled: true\n"))
	if err != nil || !cfg.Discovery.Docker.RequireExplicitEnable {
		t.Fatalf("err=%v live=%v", err, cfg.Discovery.Docker.RequireExplicitEnable)
	}
}

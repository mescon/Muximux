package main

import (
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

func TestApplyOverrides_FlagsWinAndAreRecorded(t *testing.T) {
	t.Setenv("MUXIMUX_LISTEN", ":7000")
	t.Setenv("MUXIMUX_BASE_PATH", "/env")
	cfg := &config.Config{}
	cfg.Server.Listen = ":8080"

	applyOverrides(cfg, ":9090", "/flag")

	if cfg.Server.Listen != ":9090" || cfg.Server.BasePath != "/flag" {
		t.Fatalf("live values = %q %q", cfg.Server.Listen, cfg.Server.BasePath)
	}
	got := cfg.EnvOverrides()
	if got["listen"] != "--listen" || got["base_path"] != "--base-path" {
		t.Errorf("sources = %v", got)
	}
}

func TestApplyOverrides_EnvFallback(t *testing.T) {
	t.Setenv("MUXIMUX_LISTEN", ":7000")
	t.Setenv("MUXIMUX_BASE_PATH", "/env")
	cfg := &config.Config{}

	applyOverrides(cfg, "", "")

	if cfg.Server.Listen != ":7000" || cfg.Server.BasePath != "/env" {
		t.Fatalf("live values = %q %q", cfg.Server.Listen, cfg.Server.BasePath)
	}
	got := cfg.EnvOverrides()
	if got["listen"] != "MUXIMUX_LISTEN" || got["base_path"] != "MUXIMUX_BASE_PATH" {
		t.Errorf("sources = %v", got)
	}
}

func TestApplyOverrides_NoneSet(t *testing.T) {
	t.Setenv("MUXIMUX_LISTEN", "")
	t.Setenv("MUXIMUX_BASE_PATH", "")
	cfg := &config.Config{}
	cfg.Server.Listen = ":8080"

	applyOverrides(cfg, "", "")

	if cfg.Server.Listen != ":8080" || cfg.EnvOverrides() != nil {
		t.Errorf("unexpected override: %q %v", cfg.Server.Listen, cfg.EnvOverrides())
	}
}

func TestApplyOverrides_BlankEnvCountsAsUnset(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t"} {
		t.Setenv("MUXIMUX_LISTEN", blank)
		t.Setenv("MUXIMUX_BASE_PATH", blank)
		cfg := &config.Config{}
		cfg.Server.Listen = ":8080"
		cfg.Server.BasePath = "/keep"

		applyOverrides(cfg, "", "")

		if cfg.Server.Listen != ":8080" || cfg.Server.BasePath != "/keep" || cfg.EnvOverrides() != nil {
			t.Errorf("blank %q forced a value: %q %q %v", blank, cfg.Server.Listen, cfg.Server.BasePath, cfg.EnvOverrides())
		}
	}
}

func TestApplyLogEnv(t *testing.T) {
	t.Setenv("MUXIMUX_LOG_LEVEL", " debug ")
	t.Setenv("MUXIMUX_LOG_FORMAT", "json")
	cfg := &config.Config{}
	cfg.Server.LogLevel = "info"
	cfg.Server.LogFormat = "text"
	applyLogEnv(cfg)
	if cfg.Server.LogLevel != "debug" || cfg.Server.LogFormat != "json" {
		t.Fatalf("live values = %q %q", cfg.Server.LogLevel, cfg.Server.LogFormat)
	}
}

func TestApplyLogEnv_BlankCountsAsUnset(t *testing.T) {
	for _, blank := range []string{"", "  "} {
		t.Setenv("MUXIMUX_LOG_LEVEL", blank)
		t.Setenv("MUXIMUX_LOG_FORMAT", blank)
		cfg := &config.Config{}
		cfg.Server.LogLevel = "info"
		cfg.Server.LogFormat = "text"
		applyLogEnv(cfg)
		if cfg.Server.LogLevel != "info" || cfg.Server.LogFormat != "text" || cfg.EnvOverrides() != nil {
			t.Errorf("blank %q forced a value: %q %q", blank, cfg.Server.LogLevel, cfg.Server.LogFormat)
		}
	}
}

func TestEnvOrDefault_BlankFallsBack(t *testing.T) {
	t.Setenv("MUXIMUX_DATA_TEST", "  ")
	if got := envOrDefault("MUXIMUX_DATA_TEST", "data"); got != "data" {
		t.Errorf("blank env = %q", got)
	}
	t.Setenv("MUXIMUX_DATA_TEST", " /x ")
	if got := envOrDefault("MUXIMUX_DATA_TEST", "data"); got != "/x" {
		t.Errorf("set env = %q", got)
	}
}

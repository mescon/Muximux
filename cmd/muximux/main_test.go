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

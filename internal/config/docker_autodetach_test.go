package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAutoDetach_AppOnURLEdit pins the file-edit symmetry contract:
// when the operator hand-edits apps[].url on a Docker-tracked entry,
// Load() detaches the tracking instead of letting the next poller
// tick silently overwrite the edit.
func TestAutoDetach_AppOnURLEdit(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	// Operator edited the URL of a tracked app. docker_managed_url
	// still records what the poller last wrote, so Load() sees the
	// divergence and detaches.
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
auth:
  method: none
apps:
  - name: Sonarr
    url: http://my-stable-host:8989
    color: "#fff"
    group: Media
    docker_key: "label:sonarr"
    docker_endpoint: unix:///var/run/docker.sock
    docker_strategy: container_ip
    docker_managed_url: http://10.0.0.5:8989
groups:
  - name: Media
    color: "#fff"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
	}
	app := cfg.Apps[0]
	if app.URL != "http://my-stable-host:8989" {
		t.Errorf("operator URL edit should survive; got %q", app.URL)
	}
	if app.DockerKey != "" {
		t.Errorf("docker_key should be cleared after operator edit; got %q", app.DockerKey)
	}
	if app.DockerEndpoint != "" || app.DockerStrategy != "" || app.DockerManagedURL != "" {
		t.Errorf("all docker_* fields should be cleared after detach; got endpoint=%q strategy=%q managed=%q",
			app.DockerEndpoint, app.DockerStrategy, app.DockerManagedURL)
	}
}

// TestAutoDetach_AppMatchingURLKeepsTracking covers the no-op case:
// when url and docker_managed_url agree (the poller's last write
// hasn't been overridden) the tracking stays put.
func TestAutoDetach_AppMatchingURLKeepsTracking(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
auth:
  method: none
apps:
  - name: Sonarr
    url: http://10.0.0.5:8989
    color: "#fff"
    group: Media
    docker_key: "label:sonarr"
    docker_endpoint: unix:///var/run/docker.sock
    docker_strategy: container_ip
    docker_managed_url: http://10.0.0.5:8989
groups:
  - name: Media
    color: "#fff"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	app := cfg.Apps[0]
	if app.DockerKey != "label:sonarr" {
		t.Errorf("matching url+docker_managed_url should keep tracking; got %q", app.DockerKey)
	}
	if app.DockerManagedURL != "http://10.0.0.5:8989" {
		t.Errorf("docker_managed_url should remain; got %q", app.DockerManagedURL)
	}
}

// TestAutoDetach_AppGrandfathersEmptyManagedURL covers the upgrade
// path from a pre-detach build: existing tracked apps have no
// docker_managed_url recorded yet. Load() must not detach those;
// the poller will populate the field on its next tick.
func TestAutoDetach_AppGrandfathersEmptyManagedURL(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
auth:
  method: none
apps:
  - name: Sonarr
    url: http://10.0.0.5:8989
    color: "#fff"
    group: Media
    docker_key: "label:sonarr"
    docker_endpoint: unix:///var/run/docker.sock
    docker_strategy: container_ip
groups:
  - name: Media
    color: "#fff"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	app := cfg.Apps[0]
	if app.DockerKey != "label:sonarr" {
		t.Errorf("missing docker_managed_url should grandfather, not detach; got DockerKey=%q", app.DockerKey)
	}
}

// TestAutoDetach_GatewaySiteOnBackendEdit mirrors the app case for
// gateway sites: editing backend_url on a Docker-tracked gateway
// site causes Load() to detach the tracking so the next poller
// tick stops overwriting.
func TestAutoDetach_GatewaySiteOnBackendEdit(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
  gateway_sites:
    - domain: sonarr.example.com
      backend_url: http://my-stable-host:8989
      tls: auto
      docker_key: "label:sonarr"
      docker_endpoint: unix:///var/run/docker.sock
      docker_strategy: container_ip
      docker_managed_url: http://10.0.0.5:8989
auth:
  method: none
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Server.GatewaySites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(cfg.Server.GatewaySites))
	}
	site := cfg.Server.GatewaySites[0]
	if site.BackendURL != "http://my-stable-host:8989" {
		t.Errorf("operator backend_url edit should survive; got %q", site.BackendURL)
	}
	if site.DockerKey != "" {
		t.Errorf("docker_key should be cleared after operator edit; got %q", site.DockerKey)
	}
	if site.DockerEndpoint != "" || site.DockerStrategy != "" || site.DockerManagedURL != "" {
		t.Errorf("all docker_* fields should be cleared after detach; got endpoint=%q strategy=%q managed=%q",
			site.DockerEndpoint, site.DockerStrategy, site.DockerManagedURL)
	}
}

// TestAutoDetach_NoTrackingNoChange covers the boring case: apps
// without docker_key are untouched, full stop.
func TestAutoDetach_NoTrackingNoChange(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
auth:
  method: none
apps:
  - name: Plex
    url: http://plex:32400
    color: "#fff"
    group: Media
groups:
  - name: Media
    color: "#fff"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	app := cfg.Apps[0]
	if app.URL != "http://plex:32400" {
		t.Errorf("untracked app should pass through unchanged; got %q", app.URL)
	}
}

// TestAutoDetach_LogShape exists to document the audit-log
// expectation. We don't capture log output here (the loud channel
// is `logging.Info` which goes to slog); the test asserts the
// detach behavior is observable so an operator reading the boot
// log can correlate the lost tracking to the URL edit.
func TestAutoDetach_LogShape(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`server:
  listen: ":8080"
auth:
  method: none
apps:
  - name: A
    url: http://new
    color: "#fff"
    group: G
    docker_key: "label:a"
    docker_endpoint: unix:///var/run/docker.sock
    docker_strategy: container_ip
    docker_managed_url: http://old
groups:
  - name: G
    color: "#fff"
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.HasPrefix(cfg.Apps[0].URL, "http://new") {
		t.Errorf("operator URL should win; got %q", cfg.Apps[0].URL)
	}
	if cfg.Apps[0].DockerKey != "" {
		t.Error("detach should clear DockerKey")
	}
}

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
)

// newTestDiscoveryHandler wires the handler against an in-memory
// config + temp configPath so UpdateDockerConfig can write through
// without touching the real data dir.
func newTestDiscoveryHandler(t *testing.T, initial *config.DiscoveryDockerConfig) (*DiscoveryHandler, *config.Config, string) {
	t.Helper()
	cfg := &config.Config{}
	if initial != nil {
		cfg.Discovery.Docker = *initial
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	var svc *discovery.Service
	if initial != nil {
		svc = discovery.NewService(initial)
	}
	return NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}, nil), cfg, configPath
}

func TestGetDockerStatus_NilService(t *testing.T) {
	// On first boot before discovery is wired, service is nil. The
	// handler must not panic and must return Configured=false so the
	// frontend's CTA-mode kicks in.
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{}, nil)
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/status")
	w := httptest.NewRecorder()
	h.GetDockerStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got discovery.StatusResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Configured {
		t.Errorf("Configured = true, want false for nil service")
	}
}

func TestGetDockerStatus_DisabledConfig(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/status")
	w := httptest.NewRecorder()
	h.GetDockerStatus(w, req)

	var got discovery.StatusResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got.Configured {
		t.Errorf("Configured = true, want false")
	}
}

func TestGetDockerStatus_RejectsNonGet(t *testing.T) {
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{}, nil)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := adminCtxRequest(method, "/api/discovery/docker/status")
		w := httptest.NewRecorder()
		h.GetDockerStatus(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s -> status %d, want 405", method, w.Code)
		}
	}
}

func TestListDockerNetworks_NilServiceReturnsEmpty(t *testing.T) {
	// On first boot or with discovery off the service is nil. We must
	// not panic and we must return an empty array so the UI degrades
	// to a free-text input without rendering broken autocomplete.
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{}, nil)
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/networks")
	w := httptest.NewRecorder()
	h.ListDockerNetworks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got struct {
		Networks []string `json:"networks"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Networks == nil || len(got.Networks) != 0 {
		t.Errorf("Networks = %v, want non-nil empty slice", got.Networks)
	}
}

func TestListDockerNetworks_RejectsNonGet(t *testing.T) {
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{}, nil)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := adminCtxRequest(method, "/api/discovery/docker/networks")
		w := httptest.NewRecorder()
		h.ListDockerNetworks(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s -> status %d, want 405", method, w.Code)
		}
	}
}

func TestListDockerNetworks_DaemonUnreachableSurfaces502(t *testing.T) {
	// When the daemon is unreachable (or our client wasn't initialised
	// at all), the underlying service returns an error. The handler
	// must propagate that as 502 so the frontend can fall back to a
	// free-text input rather than rendering a stale or misleading
	// autocomplete list.
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{
		Enabled:  true,
		Endpoint: "ssh://nope", // invalid scheme keeps the client uninitialised
	})
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/networks")
	w := httptest.NewRecorder()
	h.ListDockerNetworks(w, req)
	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestGetDockerStatus_BadEndpointSurfacesLastError(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{
		Enabled:  true,
		Endpoint: "ssh://nope",
	})
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/status")
	w := httptest.NewRecorder()
	h.GetDockerStatus(w, req)

	var got discovery.StatusResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if !got.Configured {
		t.Errorf("Configured should be true (operator opted in), got false")
	}
	if got.LastError == "" {
		t.Errorf("LastError empty; want client-construction error")
	}
}

func TestValidateDiscoveryDockerConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.DiscoveryDockerConfig
		wantErr bool
	}{
		{"disabled accepts anything", config.DiscoveryDockerConfig{Enabled: false}, false},
		{"enabled needs endpoint", config.DiscoveryDockerConfig{Enabled: true}, true},
		{"unix endpoint ok", config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix:///x"}, false},
		{"tcp endpoint ok", config.DiscoveryDockerConfig{Enabled: true, Endpoint: "tcp://h:2376"}, false},
		{"http rejected", config.DiscoveryDockerConfig{Enabled: true, Endpoint: "http://h"}, true},
		{"unknown strategy rejected", config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix:///x", NetworkStrategy: "moonbeam",
		}, true},
		{"empty strategy ok (defaulted in Load)", config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix:///x", NetworkStrategy: "",
		}, false},
		{"bad refresh interval rejected", config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix:///x", NetworkStrategy: "host_port", RefreshInterval: "soon",
		}, true},
		{"good refresh interval", config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix:///x", NetworkStrategy: "host_port", RefreshInterval: "30s",
		}, false},
		{"tls enabled needs all paths", config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "tcp://h:2376", NetworkStrategy: "host_port",
			TLS: config.DiscoveryTLSConfig{Enabled: true},
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateDiscoveryDockerConfig(&c.cfg)
			if (err != nil) != c.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

func TestUpdateDockerConfig_PersistsAndRebuildsService(t *testing.T) {
	h, cfg, configPath := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})

	body, _ := json.Marshal(config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix:///tmp/never-exists.sock",
		NetworkStrategy: "host_port",
		RefreshInterval: "120s",
	})
	req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
	req.Body = http.NoBody
	req = req.WithContext(req.Context())
	req2 := req.Clone(req.Context())
	req2.Body = httpBody(body)

	w := httptest.NewRecorder()
	h.UpdateDockerConfig(w, req2)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}

	if !cfg.Discovery.Docker.Enabled {
		t.Errorf("config.Discovery.Docker.Enabled was not updated in memory")
	}
	if cfg.Discovery.Docker.RefreshInterval != "120s" {
		t.Errorf("RefreshInterval = %q, want 120s", cfg.Discovery.Docker.RefreshInterval)
	}

	// Verify on-disk persisted.
	persisted, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !persisted.Discovery.Docker.Enabled || persisted.Discovery.Docker.RefreshInterval != "120s" {
		t.Errorf("persisted config not updated: %+v", persisted.Discovery.Docker)
	}

	// Verify service was rebuilt: handler.Service() returns non-nil
	// (unlike the initial state where the seed config was disabled).
	if h.Service() == nil {
		t.Errorf("Service() returned nil after enabling")
	}
}

// TestUpdateDockerConfig_AcceptsSnakeCaseBody mirrors what the
// frontend sends. Before the json struct tags were added, fields
// like network_strategy / network_filter / refresh_interval silently
// dropped because Go's encoding/json case-insensitive match doesn't
// span underscores - "network_strategy" never matched the
// "NetworkStrategy" field name. Operators saw their saved strategy
// reset to "" with every save through the UI. This test pins the
// snake_case wire shape so a regression is caught locally.
func TestUpdateDockerConfig_AcceptsSnakeCaseBody(t *testing.T) {
	h, cfg, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})

	// Hand-built snake_case body matching what the SvelteKit form
	// actually sends. Do NOT use json.Marshal on the Go struct -
	// that would round-trip through Go field names and bypass the
	// regression we're guarding against.
	body := []byte(`{
		"enabled": true,
		"endpoint": "unix:///tmp/x.sock",
		"tls": {"enabled": false},
		"network_strategy": "container_ip",
		"network_filter": "muximux-test",
		"refresh_interval": "60s"
	}`)
	req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
	req.Body = httpBody(body)
	w := httptest.NewRecorder()
	h.UpdateDockerConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if cfg.Discovery.Docker.NetworkStrategy != "container_ip" {
		t.Errorf("network_strategy = %q, want container_ip (json tag missing?)", cfg.Discovery.Docker.NetworkStrategy)
	}
	if cfg.Discovery.Docker.NetworkFilter != "muximux-test" {
		t.Errorf("network_filter = %q, want muximux-test", cfg.Discovery.Docker.NetworkFilter)
	}
	if cfg.Discovery.Docker.RefreshInterval != "60s" {
		t.Errorf("refresh_interval = %q, want 60s", cfg.Discovery.Docker.RefreshInterval)
	}
}

func TestUpdateDockerConfig_RejectsBadShape(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})

	body, _ := json.Marshal(config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "ftp://nope", // invalid scheme
		NetworkStrategy: "host_port",
	})
	req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
	req.Body = httpBody(body)
	w := httptest.NewRecorder()
	h.UpdateDockerConfig(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestScanDocker_NilService(t *testing.T) {
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{}, nil)
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/scan")
	w := httptest.NewRecorder()
	h.ScanDocker(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got discovery.ScanResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ScanBlocked == "" {
		t.Errorf("ScanBlocked empty, want a hint about enabling discovery")
	}
}

func TestScanDocker_DisabledServiceBlocks(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})
	req := adminCtxRequest(http.MethodGet, "/api/discovery/docker/scan")
	w := httptest.NewRecorder()
	h.ScanDocker(w, req)
	var got discovery.ScanResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got.ScanBlocked == "" {
		t.Errorf("ScanBlocked empty, want a hint")
	}
}

func TestScanDocker_RejectsNonGet(t *testing.T) {
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{}, nil)
	req := adminCtxRequest(http.MethodPost, "/api/discovery/docker/scan")
	w := httptest.NewRecorder()
	h.ScanDocker(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestTestDockerConfig_RoundtripsCandidateWithoutPersisting(t *testing.T) {
	h, cfg, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})

	candidate := config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix:///nope-does-not-exist.sock",
		NetworkStrategy: "host_port",
	}
	body, _ := json.Marshal(candidate)
	req := adminCtxRequest(http.MethodPost, "/api/discovery/docker/test")
	req.Body = httpBody(body)
	w := httptest.NewRecorder()
	h.TestDockerConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var got discovery.StatusResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if !got.Configured {
		t.Errorf("Configured should be true for the candidate, got false")
	}
	if got.Reachable {
		t.Errorf("Reachable should be false (socket missing), got true")
	}
	// Persisted config must be untouched.
	if cfg.Discovery.Docker.Enabled {
		t.Errorf("test endpoint persisted the candidate; should be no-op")
	}
}

// httpBody wraps a byte slice as the http.Request body.
func httpBody(b []byte) *byteReader { return &byteReader{Buffer: bytes.NewBuffer(b)} }

type byteReader struct{ *bytes.Buffer }

func (b *byteReader) Close() error { return nil }

// adminCtxRequest is defined in system_test.go in this package and
// reused here. It seeds an admin user into the request context so the
// handler sees a privileged caller, matching the requireAdmin wrap
// at registration time.

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
	return NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}), cfg, configPath
}

func TestGetDockerStatus_NilService(t *testing.T) {
	// On first boot before discovery is wired, service is nil. The
	// handler must not panic and must return Configured=false so the
	// frontend's CTA-mode kicks in.
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{})
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
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{})
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := adminCtxRequest(method, "/api/discovery/docker/status")
		w := httptest.NewRecorder()
		h.GetDockerStatus(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s -> status %d, want 405", method, w.Code)
		}
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

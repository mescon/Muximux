package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/auth"
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

func TestUpdateDockerConfig_RejectsUnknownLifecycleMinRole(t *testing.T) {
	// An unknown min-role must be rejected at the PUT, not persisted:
	// HasMinRole against an unknown role is level 0, so it would fail OPEN
	// (every authenticated user passes the gate). The load path already
	// rejects it; the PUT path must agree.
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})
	body, _ := json.Marshal(config.DiscoveryDockerConfig{
		Enabled:          true,
		Endpoint:         "unix:///var/run/docker.sock",
		NetworkStrategy:  "container_ip",
		LifecycleEnabled: true,
		LifecycleMinRole: "superuser", // not a real role
	})
	req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
	req.Body = httpBody(body)
	w := httptest.NewRecorder()
	h.UpdateDockerConfig(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (unknown lifecycle_min_role must not persist fail-open)", w.Code)
	}
}

func TestUpdateDockerConfig_NormalizesGarbageAutoImportToOff(t *testing.T) {
	// A PUT carrying an unknown auto_import value must NOT land unnormalized
	// in the live config: the poller treats only the literal "off" as off, so
	// a garbage value would fall through Reconcile and silently auto-import.
	// The handler must normalize the mode the same way config.Load does,
	// failing closed to off.
	h, cfg, configPath := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})

	body := []byte(`{
		"enabled": true,
		"endpoint": "unix:///tmp/x.sock",
		"network_strategy": "container_ip",
		"auto_import": "definitely-not-a-mode"
	}`)
	req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
	req.Body = httpBody(body)
	w := httptest.NewRecorder()
	h.UpdateDockerConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}

	if cfg.Discovery.Docker.AutoImport != config.AutoImportOff {
		t.Errorf("in-memory AutoImport = %q, want off (garbage must normalize)", cfg.Discovery.Docker.AutoImport)
	}
	// And the on-disk copy must agree so a restart can't resurrect the
	// garbage value either.
	persisted, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if persisted.Discovery.Docker.AutoImport != config.AutoImportOff {
		t.Errorf("persisted AutoImport = %q, want off", persisted.Discovery.Docker.AutoImport)
	}
}

// storedDockerConfig is the config the S-08 reproduction started from.
func storedDockerConfig() *config.DiscoveryDockerConfig {
	return &config.DiscoveryDockerConfig{
		Enabled:              true,
		Endpoint:             "unix:///tmp/never-exists.sock",
		NetworkStrategy:      config.StrategyHostPort,
		HostIP:               "127.0.0.1",
		NetworkFilter:        "bridge",
		RefreshInterval:      "30s",
		AutoImport:           config.AutoImportAdd,
		HealthBadgePlacement: "overview_and_nav",
	}
}

func putDockerConfig(t *testing.T, h *DiscoveryHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
	req.Body = httpBody([]byte(body))
	w := httptest.NewRecorder()
	h.DockerConfig(w, req)
	return w
}

func getDockerConfig(t *testing.T, h *DiscoveryHandler) dockerConfigResponse {
	t.Helper()
	w := httptest.NewRecorder()
	h.DockerConfig(w, adminCtxRequest(http.MethodGet, "/api/discovery/docker/config"))
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %q", w.Code, w.Body.String())
	}
	var got dockerConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got
}

func TestGetDockerConfig_ReturnsStoredConfig(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, storedDockerConfig())
	got := getDockerConfig(t, h)
	c := got.Config
	if c.HostIP != "127.0.0.1" || c.NetworkFilter != "bridge" || c.AutoImport != config.AutoImportAdd ||
		c.RefreshInterval != "30s" || c.HealthBadgePlacement != "overview_and_nav" || !c.Enabled {
		t.Errorf("config = %+v, want the stored values", c)
	}
	if got.EnvOverrides != nil {
		t.Errorf("env_overrides = %v, want none", got.EnvOverrides)
	}
}

func TestGetDockerConfig_ReportsAutoImportOverride(t *testing.T) {
	h, cfg, _ := newTestDiscoveryHandler(t, storedDockerConfig())
	config.ApplyAutoImportEnv(cfg, func(string) (string, bool) { return "sync", true })
	got := getDockerConfig(t, h)
	if got.EnvOverrides["auto_import"] != config.EnvAutoImport {
		t.Errorf("env_overrides = %v, want auto_import -> %s", got.EnvOverrides, config.EnvAutoImport)
	}
	if got.Config.AutoImport != config.AutoImportSync {
		t.Errorf("auto_import = %q, want the live value sync", got.Config.AutoImport)
	}
}

func TestGetDockerConfig_RejectsNonGet(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, nil)
	w := httptest.NewRecorder()
	h.GetDockerConfig(w, adminCtxRequest(http.MethodPost, "/api/discovery/docker/config"))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestDockerConfig_RejectsOtherMethods(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, nil)
	w := httptest.NewRecorder()
	h.DockerConfig(w, adminCtxRequest(http.MethodDelete, "/api/discovery/docker/config"))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestUpdateDockerConfig_MergesOntoStored(t *testing.T) {
	h, cfg, configPath := newTestDiscoveryHandler(t, storedDockerConfig())
	// The body the Discovery tab sent in the reproduction (scenario A),
	// without host_ip: no auto_import, no TLS paths.
	w := putDockerConfig(t, h, `{"enabled":true,"endpoint":"unix:///tmp/never-exists.sock","tls":{"enabled":false},`+
		`"network_strategy":"host_port","network_filter":"host","refresh_interval":"60s","lifecycle_enabled":false,`+
		`"lifecycle_min_role":"admin","lifecycle_allowed_groups":[],"health_badge_placement":"off"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	d := cfg.Discovery.Docker
	if d.AutoImport != config.AutoImportAdd {
		t.Errorf("auto_import = %q, want add (kept)", d.AutoImport)
	}
	if d.HostIP != "127.0.0.1" {
		t.Errorf("host_ip = %q, want 127.0.0.1 (kept)", d.HostIP)
	}
	if d.NetworkFilter != "host" || d.HealthBadgePlacement != "off" || d.RefreshInterval != "60s" {
		t.Errorf("sent fields not applied: %+v", d)
	}
	persisted, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if persisted.Discovery.Docker.AutoImport != config.AutoImportAdd || persisted.Discovery.Docker.HostIP != "127.0.0.1" {
		t.Errorf("persisted = %+v, want auto_import add and host_ip kept", persisted.Discovery.Docker)
	}
}

func TestUpdateDockerConfig_AppliesLoadDefaults(t *testing.T) {
	h, cfg, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})
	w := putDockerConfig(t, h, `{
		"enabled": true,
		"endpoint": "unix:///tmp/never-exists.sock",
		"network_strategy": "",
		"refresh_interval": ""
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	d := cfg.Discovery.Docker
	if d.NetworkStrategy != config.StrategyContainerIP || d.RefreshInterval != "60s" {
		t.Errorf("stored = %+v, want container_ip and 60s", d)
	}
	if d.HealthBadgePlacement != "overview" || d.AutoImport != config.AutoImportOff {
		t.Errorf("stored = %+v, want placement overview and auto_import off", d)
	}
	// The response is the reconfigured service's status, so its strategy
	// shows what svc.Reconfigure received.
	var st discovery.StatusResult
	if err := json.NewDecoder(w.Body).Decode(&st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if st.Strategy != config.StrategyContainerIP {
		t.Errorf("service strategy = %q, want container_ip", st.Strategy)
	}
}

func TestUpdateDockerConfig_TrimsLifecycleGroups(t *testing.T) {
	initial := storedDockerConfig()
	initial.LifecycleAllowedGroups = []string{" ops "}
	h, _, _ := newTestDiscoveryHandler(t, initial)
	prior := h.config.Discovery.Docker.LifecycleAllowedGroups
	w := putDockerConfig(t, h, `{"lifecycle_allowed_groups": [" ops "]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	got := h.config.Discovery.Docker.LifecycleAllowedGroups
	if len(got) != 1 || got[0] != "ops" {
		t.Errorf("stored groups = %q, want [ops]", got)
	}
	if prior[0] != " ops " {
		t.Errorf("prior[0] = %q: the decode or trim wrote into the stored slice", prior[0])
	}
}

func TestUpdateDockerConfig_AcceptsNpipe(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})
	w := putDockerConfig(t, h, `{"enabled": true, "endpoint": "npipe:////./pipe/docker_engine"}`)
	if w.Code == http.StatusBadRequest {
		t.Fatalf("status = 400, body = %q", w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("must start with")) {
		t.Errorf("body carries the scheme error: %q", w.Body.String())
	}
}

func TestTestDockerConfig_AcceptsNpipe(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: false})
	req := adminCtxRequest(http.MethodPost, "/api/discovery/docker/test")
	req.Body = httpBody([]byte(`{"enabled": true, "endpoint": "npipe:////./pipe/docker_engine", "network_strategy": "container_ip"}`))
	w := httptest.NewRecorder()
	h.TestDockerConfig(w, req)
	if w.Code == http.StatusBadRequest {
		t.Fatalf("status = 400, body = %q", w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("must start with")) {
		t.Errorf("body carries the scheme error: %q", w.Body.String())
	}
}

func TestUpdateDockerConfig_KeepsOverriddenAutoImport(t *testing.T) {
	h, cfg, configPath := newTestDiscoveryHandler(t, storedDockerConfig())
	config.ApplyAutoImportEnv(cfg, func(string) (string, bool) { return "sync", true })
	w := putDockerConfig(t, h, `{"auto_import": "off", "network_filter": "host"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if cfg.Discovery.Docker.AutoImport != config.AutoImportSync {
		t.Errorf("live auto_import = %q, want the override sync", cfg.Discovery.Docker.AutoImport)
	}
	if cfg.Discovery.Docker.NetworkFilter != "host" {
		t.Errorf("network_filter = %q, want host", cfg.Discovery.Docker.NetworkFilter)
	}
	persisted, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if persisted.Discovery.Docker.AutoImport != config.AutoImportAdd {
		t.Errorf("file auto_import = %q, want its own value add", persisted.Discovery.Docker.AutoImport)
	}
}

// Concurrent saves of different fields: each merges onto what the other
// stored (decode under the write lock), so neither field is lost.
func TestUpdateDockerConfig_ConcurrentSavesKeepBothFields(t *testing.T) {
	for round := 0; round < 20; round++ {
		h, cfg, _ := newTestDiscoveryHandler(t, storedDockerConfig())
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i, body := range []string{`{"network_filter": "host"}`, `{"health_badge_placement": "off"}`} {
			wg.Add(1)
			go func(i int, body string) {
				defer wg.Done()
				req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
				req.Body = httpBody([]byte(body))
				w := httptest.NewRecorder()
				h.UpdateDockerConfig(w, req)
				codes[i] = w.Code
			}(i, body)
		}
		wg.Wait()
		if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
			t.Fatalf("round %d: codes %v", round, codes)
		}
		h.configMu.RLock()
		d := cfg.Discovery.Docker
		h.configMu.RUnlock()
		if d.NetworkFilter != "host" || d.HealthBadgePlacement != "off" {
			t.Fatalf("round %d: lost a field: filter %q placement %q", round, d.NetworkFilter, d.HealthBadgePlacement)
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestUpdateDockerConfig_BodyReadError(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, storedDockerConfig())
	req := adminCtxRequest(http.MethodPut, "/api/discovery/docker/config")
	req.Body = io.NopCloser(failingReader{})
	w := httptest.NewRecorder()
	h.UpdateDockerConfig(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUpdateDockerConfig_RejectsBadJSON(t *testing.T) {
	h, _, _ := newTestDiscoveryHandler(t, storedDockerConfig())
	if w := putDockerConfig(t, h, `{`); w.Code != http.StatusBadRequest {
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

func TestGetDockerStatus_IncludesSocketWritableAndLifecycle(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discovery.Docker.Enabled = true
	cfg.Discovery.Docker.LifecycleEnabled = true
	svc := discovery.NewService(&cfg.Discovery.Docker)
	// Simulate a successful probe.
	svc.SetSocketWritableForTest(true)

	h := NewDiscoveryHandler(svc, cfg, "/tmp/config.yaml", &sync.RWMutex{}, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/discovery/docker/status", nil)
	w := httptest.NewRecorder()
	h.GetDockerStatus(w, r)

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := body["socket_writable"].(bool); !got {
		t.Fatalf("want socket_writable=true, got %+v", body["socket_writable"])
	}
	if got, _ := body["lifecycle_enabled"].(bool); !got {
		t.Fatalf("want lifecycle_enabled=true, got %+v", body["lifecycle_enabled"])
	}
}

func TestGetDockerStateMap_ReturnsCachedState(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "sonarr"}}}
	cfg.Discovery.Docker.Enabled = true
	svc := discovery.NewService(&cfg.Discovery.Docker)
	svc.SetDockerStateForApp("sonarr", &discovery.DockerState{Status: "running", Health: "healthy", Image: "img"})

	h := NewDiscoveryHandler(svc, cfg, "/tmp/config.yaml", &sync.RWMutex{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/discovery/docker-state", nil)
	r = r.WithContext(auth.WithUserContext(r.Context(), &auth.User{Username: "admin", Role: auth.RoleAdmin}))
	w := httptest.NewRecorder()
	h.GetDockerStateMap(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body map[string]discovery.DockerState
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["sonarr"].Status != "running" {
		t.Fatalf("want sonarr running, got %+v", body)
	}
}

// TestGetDockerStateMap_FiltersByAppVisibility guards the read-side
// companion to the #13 lifecycle access gate: the docker-state endpoint
// must not expose container state for apps the requesting user is not
// allowed to see (min_role / allowed_groups), or a non-admin could learn
// the existence, image, and status of hidden apps.
func TestGetDockerStateMap_FiltersByAppVisibility(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{
		{Name: "public"},
		{Name: "secret", AllowedGroups: []string{"admins"}},
	}}
	cfg.Discovery.Docker.Enabled = true
	svc := discovery.NewService(&cfg.Discovery.Docker)
	svc.SetDockerStateForApp("public", &discovery.DockerState{Status: "running", Image: "pub"})
	svc.SetDockerStateForApp("secret", &discovery.DockerState{Status: "running", Image: "sec"})
	h := NewDiscoveryHandler(svc, cfg, "/tmp/config.yaml", &sync.RWMutex{}, nil)

	get := func(user *auth.User) map[string]discovery.DockerState {
		r := httptest.NewRequest(http.MethodGet, "/api/discovery/docker-state", nil)
		r = r.WithContext(auth.WithUserContext(r.Context(), user))
		w := httptest.NewRecorder()
		h.GetDockerStateMap(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body map[string]discovery.DockerState
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body
	}

	// A non-admin with no groups sees only the unrestricted app.
	nonAdmin := get(&auth.User{Username: "bob", Role: auth.RoleUser})
	if _, ok := nonAdmin["public"]; !ok {
		t.Error("non-admin should see the public app's state")
	}
	if _, ok := nonAdmin["secret"]; ok {
		t.Error("non-admin must NOT see the group-restricted app's state")
	}

	// An admin sees everything.
	admin := get(&auth.User{Username: "admin", Role: auth.RoleAdmin})
	if len(admin) != 2 {
		t.Errorf("admin should see both apps, got %d: %+v", len(admin), admin)
	}
}

func TestGetDockerConfig_ReportsRequireExplicitEnableOverride(t *testing.T) {
	h, cfg, _ := newTestDiscoveryHandler(t, storedDockerConfig())
	config.ApplyAutoImportEnv(cfg, func(string) (string, bool) { return "sync", true })
	config.ApplyRequireExplicitEnableEnv(cfg, func(string) (string, bool) { return "true", true })
	got := getDockerConfig(t, h)
	if got.EnvOverrides["require_explicit_enable"] != config.EnvRequireExplicitEnable {
		t.Errorf("env_overrides = %v, want require_explicit_enable -> %s", got.EnvOverrides, config.EnvRequireExplicitEnable)
	}
	if got.EnvOverrides["auto_import"] != config.EnvAutoImport {
		t.Errorf("env_overrides = %v, want auto_import kept", got.EnvOverrides)
	}
	if !got.Config.RequireExplicitEnable {
		t.Error("require_explicit_enable = false, want the live value true")
	}
}

func TestUpdateDockerConfig_KeepsOverriddenRequireExplicitEnable(t *testing.T) {
	h, cfg, configPath := newTestDiscoveryHandler(t, storedDockerConfig())
	config.ApplyRequireExplicitEnableEnv(cfg, func(string) (string, bool) { return "true", true })
	w := putDockerConfig(t, h, `{"require_explicit_enable": false, "network_filter": "host"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if !cfg.Discovery.Docker.RequireExplicitEnable {
		t.Error("live require_explicit_enable changed while overridden")
	}
	persisted, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if persisted.Discovery.Docker.RequireExplicitEnable {
		t.Error("override leaked into the file")
	}
}

func TestUpdateDockerConfig_SetsRequireExplicitEnable(t *testing.T) {
	h, cfg, _ := newTestDiscoveryHandler(t, storedDockerConfig())
	w := putDockerConfig(t, h, `{"require_explicit_enable": true}`)
	if w.Code != http.StatusOK || !cfg.Discovery.Docker.RequireExplicitEnable {
		t.Fatalf("status = %d, live = %v", w.Code, cfg.Discovery.Docker.RequireExplicitEnable)
	}
}

func TestScanDocker_HidesOptedOutRows(t *testing.T) {
	media := discovery.ContainerNetworks{Networks: map[string]discovery.ContainerNetwork{"media": {IPAddress: "10.0.0.5"}}}
	socket, cleanup := fakeDockerForLifecycle(t, []discovery.ContainerSummary{
		{ID: "a", Names: []string{"/optout"}, Image: "acme/optout", NetworkSettings: media,
			Labels: map[string]string{"muximux.app.enabled": "false", "muximux.app.port": "80"}},
		{ID: "b", Names: []string{"/kept"}, Image: "acme/kept", NetworkSettings: media,
			Labels: map[string]string{"muximux.app.port": "80"}},
	})
	defer cleanup()
	h, _, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket,
		NetworkStrategy: "container_ip", NetworkFilter: "media"})
	w := httptest.NewRecorder()
	h.ScanDocker(w, adminCtxRequest(http.MethodGet, "/api/discovery/docker/scan"))
	var got discovery.ScanResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Suggestions) != 1 || got.Suggestions[0].ContainerName != "kept" || got.OptedOut != 1 {
		t.Fatalf("got %+v", got)
	}
}

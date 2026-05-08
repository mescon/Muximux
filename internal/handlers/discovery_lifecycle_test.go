package handlers

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
)

// fakeDockerForLifecycle stands up a unix-socket fake daemon that
// serves a fixed /containers/json response. Mirrors the helper in
// the discovery package's tests but kept local to handlers so we
// avoid exporting test infrastructure.
func fakeDockerForLifecycle(t *testing.T, summaries []discovery.ContainerSummary) (string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(summaries)
	})
	dir := t.TempDir()
	socket := filepath.Join(dir, "docker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{Handler: mux} //nolint:gosec // test-only
	go func() { _ = srv.Serve(listener) }()
	return socket, func() {
		_ = srv.Close()
		_ = os.Remove(socket)
	}
}

// seedLifecycleHandler returns a DiscoveryHandler with a config that
// holds the given pre-tracked apps + sites and a discovery endpoint
// of "unix:///var/run/docker.sock". Tests then exercise the
// lifecycle endpoints (list / detach / re-link probe / re-link
// confirm) against this seeded state.
func seedLifecycleHandler(t *testing.T, apps []config.AppConfig, sites []config.GatewaySite) (*DiscoveryHandler, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: config.DiscoveryDockerConfig{
			Enabled:  true,
			Endpoint: "unix:///var/run/docker.sock",
		}},
	}
	cfg.Apps = apps
	cfg.Server.GatewaySites = sites
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	svc := discovery.NewService(&cfg.Discovery.Docker)
	return NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}, nil), cfg
}

func TestListTracked_FiltersUnlinkedAndFlagsEndpointMismatch(t *testing.T) {
	h, _ := seedLifecycleHandler(t,
		[]config.AppConfig{
			{Name: "tracked-current", URL: "http://10.0.0.1:80",
				DockerKey: "label:current", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip"},
			{Name: "tracked-stale", URL: "http://10.0.0.2:80",
				DockerKey: "label:stale", DockerEndpoint: "tcp://old:2375", DockerStrategy: "container_ip"},
			{Name: "manual", URL: "http://10.0.0.3:80"}, // no docker_key -> excluded
		},
		[]config.GatewaySite{
			{Domain: "site.example.com", BackendURL: "http://10.0.0.4:80",
				DockerKey: "name:foo", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_dns"},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/discovery/docker/tracked", nil)
	w := httptest.NewRecorder()
	h.ListTracked(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got TrackedListResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CurrentEndpoint != "unix:///var/run/docker.sock" {
		t.Errorf("current_endpoint = %q", got.CurrentEndpoint)
	}
	if len(got.Entries) != 3 {
		t.Fatalf("entries = %d, want 3 (2 tracked apps + 1 site)", len(got.Entries))
	}
	// Indexing: app order matches config insertion, then sites.
	if got.Entries[0].Name != "tracked-current" || !got.Entries[0].EndpointMatches {
		t.Errorf("first entry = %+v, want tracked-current with endpoint match", got.Entries[0])
	}
	if got.Entries[1].Name != "tracked-stale" || got.Entries[1].EndpointMatches {
		t.Errorf("second entry = %+v, want tracked-stale with endpoint mismatch", got.Entries[1])
	}
	if got.Entries[2].Kind != "gateway" || got.Entries[2].Name != "site.example.com" {
		t.Errorf("third entry = %+v, want gateway site", got.Entries[2])
	}
}

func TestListTracked_RejectsNonGet(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/discovery/docker/tracked", nil)
	w := httptest.NewRecorder()
	h.ListTracked(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.Code)
	}
}

func TestDetachTracked_ClearsMatchingEntries(t *testing.T) {
	h, cfg := seedLifecycleHandler(t,
		[]config.AppConfig{
			{Name: "matched", URL: "http://10.0.0.1:80",
				DockerKey: "label:foo", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip"},
			{Name: "different-key", URL: "http://10.0.0.2:80",
				DockerKey: "label:bar", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip"},
		},
		[]config.GatewaySite{
			{Domain: "matched.example.com", BackendURL: "http://10.0.0.3:80",
				DockerKey: "label:foo", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_dns"},
		},
	)

	// Pre-stamp LastSeen so we can verify ForgetTrackedKey ran.
	h.Service().RecordSeen("label:foo")

	req := httptest.NewRequest(http.MethodDelete, "/api/discovery/docker/track/label:foo", nil)
	w := httptest.NewRecorder()
	h.DetachTracked(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if cfg.Apps[0].DockerKey != "" || cfg.Apps[0].DockerEndpoint != "" || cfg.Apps[0].DockerStrategy != "" {
		t.Errorf("matched app not cleared: %+v", cfg.Apps[0])
	}
	if cfg.Apps[1].DockerKey != "label:bar" {
		t.Errorf("different-key app should still be tracked: %+v", cfg.Apps[1])
	}
	if cfg.Server.GatewaySites[0].DockerKey != "" {
		t.Errorf("matched site not cleared: %+v", cfg.Server.GatewaySites[0])
	}
	if !h.Service().LastSeen("label:foo").IsZero() {
		t.Errorf("ForgetTrackedKey did not drop the LastSeen entry")
	}
}

func TestDetachTracked_NoMatchReturns404(t *testing.T) {
	h, _ := seedLifecycleHandler(t,
		[]config.AppConfig{
			{Name: "x", URL: "http://10.0.0.1:80",
				DockerKey: "label:exists", DockerEndpoint: "unix:///var/run/docker.sock"},
		},
		nil,
	)
	req := httptest.NewRequest(http.MethodDelete, "/api/discovery/docker/track/label:does-not-match", nil)
	w := httptest.NewRecorder()
	h.DetachTracked(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", w.Code)
	}
}

func TestDetachTracked_StaleEndpointDoesNotMatch(t *testing.T) {
	// An app whose DockerKey matches the URL parameter but whose
	// DockerEndpoint points elsewhere should NOT be detached. The
	// detach endpoint is scoped to the current endpoint so the
	// re-link flow can still surface stranded entries.
	h, cfg := seedLifecycleHandler(t,
		[]config.AppConfig{
			{Name: "stale", URL: "http://10.0.0.1:80",
				DockerKey: "label:foo", DockerEndpoint: "tcp://old:2375"},
		},
		nil,
	)
	req := httptest.NewRequest(http.MethodDelete, "/api/discovery/docker/track/label:foo", nil)
	w := httptest.NewRecorder()
	h.DetachTracked(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404 (stale endpoint -> not matched)", w.Code)
	}
	if cfg.Apps[0].DockerKey != "label:foo" {
		t.Errorf("stale app should still be tracked: %+v", cfg.Apps[0])
	}
}

func TestDetachTracked_RejectsNonDelete(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/discovery/docker/track/x", nil)
	w := httptest.NewRecorder()
	h.DetachTracked(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.Code)
	}
}

func TestDetachTracked_EmptyKeyIs400(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/discovery/docker/track/", nil)
	w := httptest.NewRecorder()
	h.DetachTracked(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

func TestRelinkProbe_RejectsNonPost(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/discovery/docker/relink/probe", nil)
	w := httptest.NewRecorder()
	h.RelinkProbe(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.Code)
	}
}

func TestRelinkProbe_BadBody(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/discovery/docker/relink/probe", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	h.RelinkProbe(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

func TestRelinkProbe_EmptyKey(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	body, _ := json.Marshal(RelinkProbeRequest{Key: "  "})
	req := httptest.NewRequest(http.MethodPost, "/api/discovery/docker/relink/probe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.RelinkProbe(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

func TestRelinkProbe_ServiceNil(t *testing.T) {
	h := NewDiscoveryHandler(nil, &config.Config{}, "", &sync.RWMutex{}, nil)
	body, _ := json.Marshal(RelinkProbeRequest{Key: "label:foo"})
	req := httptest.NewRequest(http.MethodPost, "/api/discovery/docker/relink/probe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.RelinkProbe(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", w.Code)
	}
}

func TestRelinkProbe_DaemonError_SurfacedInBody(t *testing.T) {
	// Service has a client pointed at a non-existent socket; the
	// daemon list call fails. The handler returns 200 with the
	// error embedded so the modal can render it inline.
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix:///nonexistent.sock",
		}},
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	_ = cfg.Save(configPath)
	svc := discovery.NewService(&cfg.Discovery.Docker)
	h := NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}, nil)

	body, _ := json.Marshal(RelinkProbeRequest{Key: "label:foo"})
	req := httptest.NewRequest(http.MethodPost, "/api/discovery/docker/relink/probe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.RelinkProbe(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got RelinkProbeResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error == "" {
		t.Errorf("expected error in body; got %+v", got)
	}
}

func TestRelinkConfirm_RewritesMatchingEntries(t *testing.T) {
	h, cfg := seedLifecycleHandler(t,
		[]config.AppConfig{
			{Name: "stranded-app", URL: "http://10.0.0.1:80",
				DockerKey: "label:old", DockerEndpoint: "tcp://old:2375", DockerStrategy: "container_ip"},
		},
		[]config.GatewaySite{
			{Domain: "stranded.example.com", BackendURL: "http://10.0.0.2:80",
				DockerKey: "label:old", DockerEndpoint: "tcp://old:2375", DockerStrategy: "container_dns"},
		},
	)

	req := mustReqJSON(t, http.MethodPost, "/api/discovery/docker/relink/confirm",
		RelinkConfirmRequest{OldKey: "label:old", NewKey: "label:new", Strategy: "host_port"})
	w := httptest.NewRecorder()
	h.RelinkConfirm(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got RelinkConfirmResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if len(got.UpdatedApps) != 1 || got.UpdatedApps[0] != "stranded-app" {
		t.Errorf("updated apps = %+v", got.UpdatedApps)
	}
	if len(got.UpdatedSites) != 1 || got.UpdatedSites[0] != "stranded.example.com" {
		t.Errorf("updated sites = %+v", got.UpdatedSites)
	}
	if cfg.Apps[0].DockerKey != "label:new" {
		t.Errorf("app key not updated: %s", cfg.Apps[0].DockerKey)
	}
	if cfg.Apps[0].DockerEndpoint != "unix:///var/run/docker.sock" {
		t.Errorf("app endpoint not updated to current: %s", cfg.Apps[0].DockerEndpoint)
	}
	if cfg.Apps[0].DockerStrategy != "host_port" {
		t.Errorf("app strategy not updated: %s", cfg.Apps[0].DockerStrategy)
	}
	if cfg.Server.GatewaySites[0].DockerKey != "label:new" {
		t.Errorf("site key not updated: %s", cfg.Server.GatewaySites[0].DockerKey)
	}
}

func TestRelinkConfirm_NoMatchIs404(t *testing.T) {
	h, _ := seedLifecycleHandler(t,
		[]config.AppConfig{
			{Name: "x", URL: "http://10.0.0.1:80", DockerKey: "label:other"},
		},
		nil,
	)
	req := mustReqJSON(t, http.MethodPost, "/api/discovery/docker/relink/confirm",
		RelinkConfirmRequest{OldKey: "label:absent", NewKey: "label:new"})
	w := httptest.NewRecorder()
	h.RelinkConfirm(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", w.Code)
	}
}

func TestRelinkConfirm_BadBody(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/discovery/docker/relink/confirm", strings.NewReader("nope"))
	w := httptest.NewRecorder()
	h.RelinkConfirm(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

func TestRelinkConfirm_EmptyKeysAre400(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := mustReqJSON(t, http.MethodPost, "/api/discovery/docker/relink/confirm",
		RelinkConfirmRequest{OldKey: "", NewKey: "label:new"})
	w := httptest.NewRecorder()
	h.RelinkConfirm(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

func TestRelinkConfirm_RejectsNonPost(t *testing.T) {
	h, _ := seedLifecycleHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/discovery/docker/relink/confirm", nil)
	w := httptest.NewRecorder()
	h.RelinkConfirm(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.Code)
	}
}

func TestMatchByKey_Variants(t *testing.T) {
	containers := []discovery.ContainerSummary{
		{ID: "abc1234567", Names: []string{"/sonarr"}, Labels: map[string]string{discovery.LabelDiscoveryID: "labeled"}},
		{ID: "def4567890", Names: []string{"/radarr"}},
	}
	cases := []struct {
		key        string
		wantNotNil bool
		wantName   string
	}{
		{"label:labeled", true, "sonarr"},
		{"name:radarr", true, "radarr"},
		{"id:def456", true, "radarr"},     // prefix match on id
		{"id:def4567890", true, "radarr"}, // exact match
		{"label:absent", false, ""},
		{"malformed-without-colon", false, ""},
	}
	for _, tc := range cases {
		got := matchByKey(containers, tc.key)
		if tc.wantNotNil && got == nil {
			t.Errorf("matchByKey(%q) = nil, want non-nil", tc.key)
			continue
		}
		if !tc.wantNotNil && got != nil {
			t.Errorf("matchByKey(%q) = %+v, want nil", tc.key, got)
			continue
		}
		if got != nil && got.PrimaryName() != tc.wantName {
			t.Errorf("matchByKey(%q).PrimaryName = %q, want %q", tc.key, got.PrimaryName(), tc.wantName)
		}
	}
}

// mustReqJSON builds an *http.Request whose body is the JSON-encoded
// payload. Eliminates the bytes.NewReader / json.Marshal boilerplate
// across the lifecycle tests.
//
//nolint:unparam // method is always POST today but the helper signature is general so future tests can pass PUT/PATCH without refactoring callers.
func mustReqJSON(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestRelinkProbe_FoundOnLiveDaemon(t *testing.T) {
	// The tracked key matches a running container on the current
	// endpoint -> RelinkProbeResult.Found = true with the matched
	// container surfaced as the suggested re-link target.
	containers := []discovery.ContainerSummary{
		{
			ID:     "abc123",
			Names:  []string{"/sonarr"},
			Image:  "linuxserver/sonarr",
			Labels: map[string]string{discovery.LabelDiscoveryID: "sonarr-prod"},
		},
	}
	socket, cleanup := fakeDockerForLifecycle(t, containers)
	defer cleanup()

	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix://" + socket,
		}},
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	_ = cfg.Save(configPath)
	svc := discovery.NewService(&cfg.Discovery.Docker)
	h := NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}, nil)

	req := mustReqJSON(t, http.MethodPost, "/api/discovery/docker/relink/probe",
		RelinkProbeRequest{Key: "label:sonarr-prod"})
	w := httptest.NewRecorder()
	h.RelinkProbe(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	var got RelinkProbeResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if !got.Found || got.Container == nil {
		t.Fatalf("expected Found=true with non-nil Container, got %+v", got)
	}
	if got.Container.Name != "sonarr" {
		t.Errorf("Container.Name = %q, want sonarr", got.Container.Name)
	}
	if got.Container.Image != "linuxserver/sonarr" {
		t.Errorf("Container.Image = %q", got.Container.Image)
	}
	// candidateFromContainer derives the key with KeyForContainer's
	// label > name > id priority; the label takes precedence.
	if got.Container.Key != "label:sonarr-prod" {
		t.Errorf("Container.Key = %q, want label:sonarr-prod", got.Container.Key)
	}
}

func TestRelinkProbe_NotFoundReturnsCandidates(t *testing.T) {
	// The tracked key has no match on the current endpoint, but
	// the daemon does have other containers running. The handler
	// returns Found=false with all running containers as
	// candidates, sorted alphabetically by name.
	containers := []discovery.ContainerSummary{
		{ID: "b2", Names: []string{"/zebra"}, Image: "zoo/zebra"},
		{ID: "a1", Names: []string{"/aardvark"}, Image: "zoo/aardvark"},
	}
	socket, cleanup := fakeDockerForLifecycle(t, containers)
	defer cleanup()

	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix://" + socket,
		}},
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	_ = cfg.Save(configPath)
	svc := discovery.NewService(&cfg.Discovery.Docker)
	h := NewDiscoveryHandler(svc, cfg, configPath, &sync.RWMutex{}, nil)

	req := mustReqJSON(t, http.MethodPost, "/api/discovery/docker/relink/probe",
		RelinkProbeRequest{Key: "label:absent"})
	w := httptest.NewRecorder()
	h.RelinkProbe(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	var got RelinkProbeResult
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got.Found {
		t.Errorf("expected Found=false")
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(got.Candidates))
	}
	// Alphabetical order: aardvark before zebra.
	if got.Candidates[0].Name != "aardvark" {
		t.Errorf("candidates[0] = %+v, want aardvark first", got.Candidates[0])
	}
}

func TestFormatLastSeen_NilService(t *testing.T) {
	if got := formatLastSeen(nil, "label:foo"); got != "" {
		t.Errorf("formatLastSeen(nil, _) = %q, want empty", got)
	}
}

func TestFormatLastSeen_NeverSeenReturnsEmpty(t *testing.T) {
	svc := discovery.NewService(&config.DiscoveryDockerConfig{})
	if got := formatLastSeen(svc, "label:never"); got != "" {
		t.Errorf("never-seen key returned %q, want empty", got)
	}
}

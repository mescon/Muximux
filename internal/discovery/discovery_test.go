package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
)

func TestNewService_DisabledReturnsConfiguredFalse(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: false})
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	got := svc.Status(context.Background())
	if got.Configured {
		t.Errorf("Configured = true, want false for disabled config")
	}
	if got.Reachable {
		t.Errorf("Reachable = true, want false")
	}
}

func TestNewService_EnabledButEmptyEndpointStaysConfiguredFalse(t *testing.T) {
	// Endpoint empty -> NewClient short-circuits, Service has no
	// client. We treat this as "not configured" rather than as a
	// daemon-unreachable error.
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: true})
	got := svc.Status(context.Background())
	if got.Configured {
		t.Errorf("Configured = true, want false when endpoint empty")
	}
}

func TestNewService_EnabledWithBadEndpointSurfaceLastError(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:  true,
		Endpoint: "ssh://nope",
	})
	got := svc.Status(context.Background())
	if !got.Configured {
		t.Errorf("Configured = false, want true (operator did set enabled+endpoint)")
	}
	if got.LastError == "" {
		t.Errorf("LastError empty, want error message about ssh:// scheme")
	}
}

func TestService_Status_Reachable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(VersionInfo{APIVersion: "1.43"})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "host_port", // doesn't need self-detect
	})
	got := svc.Status(context.Background())
	if !got.Configured || !got.Reachable {
		t.Errorf("want Configured && Reachable, got %+v", got)
	}
	if !got.StrategyOK {
		t.Errorf("host_port strategy should always be StrategyOK; got %+v", got)
	}
	if got.APIVersion != "1.43" {
		t.Errorf("APIVersion = %q, want 1.43", got.APIVersion)
	}
}

func TestService_Status_UnknownStrategy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(VersionInfo{APIVersion: "1.43"})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "moonbeam",
	})
	got := svc.Status(context.Background())
	if got.StrategyOK {
		t.Errorf("StrategyOK = true for unknown strategy")
	}
	if got.LastError == "" {
		t.Errorf("LastError empty, want message about unknown strategy")
	}
}

func TestService_Status_ContainerStrategyAcceptsNetworkFilterAsSubstitute(t *testing.T) {
	// When self-detect would fail but the operator has set
	// network_filter explicitly, StrategyOK should be true. We
	// simulate self-detect failure by having the daemon's inspects
	// 404, then turn network_filter on.
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(VersionInfo{APIVersion: "1.43"})
	})
	mux.HandleFunc("/v1.41/containers/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.41/containers/json" {
			_, _ = w.Write([]byte("[]"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "container_ip",
		NetworkFilter:   "media", // operator-supplied substitute for self-membership
	})
	got := svc.Status(context.Background())
	// On a non-docker dev host, self-detect fails (cgroup empty,
	// hostname inspect 404s); with network_filter set, StrategyOK
	// should still be true.
	if !got.StrategyOK && got.SelfDetect != string(SelfDetectNone) {
		// On a real docker host where self-detect succeeds, we'd get
		// StrategyOK = true via SelfDetectCgroup or SelfDetectHostname
		// against the real container. Both outcomes are acceptable.
		t.Skipf("dev host appears to be a docker container with successful self-detect; got %+v", got)
	}
	if !got.StrategyOK {
		t.Errorf("StrategyOK = false even with network_filter set; got %+v", got)
	}
}

func TestService_Status_CachedBetweenCalls(t *testing.T) {
	var pingCount int
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) {
		pingCount++
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1.41/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(VersionInfo{APIVersion: "1.43"})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	svc := NewService(&config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "host_port",
	})
	for i := 0; i < 5; i++ {
		_ = svc.Status(context.Background())
	}
	if pingCount != 1 {
		t.Errorf("expected 1 ping (cached), got %d", pingCount)
	}
}

func TestTLSHygieneWarning_WorldReadable(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "client.key")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o644); err != nil { //nolint:gosec // intentionally world-readable to exercise the warning path
		t.Fatal(err)
	}
	w := tlsHygieneWarning(&config.DiscoveryTLSConfig{
		Enabled:   true,
		ClientKey: keyPath,
	})
	if w == "" {
		t.Errorf("expected hygiene warning for 0644 key, got empty")
	}
}

func TestTLSHygieneWarning_GoodPermissions(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "client.key")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := tlsHygieneWarning(&config.DiscoveryTLSConfig{
		Enabled:   true,
		ClientKey: keyPath,
	})
	if w != "" {
		t.Errorf("expected no warning for 0600 key, got %q", w)
	}
}

func TestTLSHygieneWarning_DisabledSilent(t *testing.T) {
	w := tlsHygieneWarning(&config.DiscoveryTLSConfig{Enabled: false})
	if w != "" {
		t.Errorf("expected no warning when TLS disabled, got %q", w)
	}
}

func TestIsLikelySelf(t *testing.T) {
	cases := []struct {
		name      string
		container ContainerSummary
		want      bool
	}{
		{
			name:      "canonical image",
			container: ContainerSummary{Image: "ghcr.io/mescon/muximux:latest", Names: []string{"/muximux"}},
			want:      true,
		},
		{
			name:      "custom built image, name still contains muximux",
			container: ContainerSummary{Image: "private.registry/me/dashboard:v3", Names: []string{"/homelab-muximux"}},
			want:      true,
		},
		{
			name:      "prefix convention on name only",
			container: ContainerSummary{Image: "someimage", Names: []string{"/homelab_muximux_1"}},
			want:      true,
		},
		{
			name:      "uppercase preserved in image",
			container: ContainerSummary{Image: "Muximux", Names: []string{"/dashboard"}},
			want:      true,
		},
		{
			name:      "totally unrelated app",
			container: ContainerSummary{Image: "linuxserver/sonarr", Names: []string{"/homelab-sonarr"}},
			want:      false,
		},
		{
			name:      "no names at all",
			container: ContainerSummary{Image: "linuxserver/radarr"},
			want:      false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isLikelySelf(&c.container)
			if got != c.want {
				t.Errorf("isLikelySelf = %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsSocketWritable_WritableReturnsTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", r.Method)
		}
		// Writable socket: daemon accepts POST, container missing.
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: srv.URL}
	if !c.IsSocketWritable(context.Background()) {
		t.Fatalf("expected writable=true for 404 response")
	}
}

func TestIsSocketWritable_ReadOnlyProxyReturnsFalse(t *testing.T) {
	for _, code := range []int{http.StatusForbidden, http.StatusMethodNotAllowed, http.StatusUnauthorized} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))
		c := &Client{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: srv.URL}
		if c.IsSocketWritable(context.Background()) {
			t.Fatalf("expected writable=false for status %d", code)
		}
		srv.Close()
	}
}

func TestNewService_ProbesSocketWritability(t *testing.T) {
	// End-to-end: NewService should run the probe and stamp
	// socketWritable. Hard to drive through unix sockets in a test,
	// so this case asserts the probe was *called* via a fake client
	// injection. NewService wires Probe via the configured endpoint;
	// we use httptest as a tcp endpoint instead.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/containers/"):
			w.WriteHeader(http.StatusNotFound) // writable
		case r.URL.Path == "/_ping" || strings.HasSuffix(r.URL.Path, "/_ping"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cfg := &config.DiscoveryDockerConfig{
		Enabled:  true,
		Endpoint: "tcp://" + strings.TrimPrefix(srv.URL, "http://"),
	}
	s := NewService(cfg)
	// The probe runs lazily on first SocketWritable() call (or eagerly
	// in NewService - either is acceptable). Trigger it explicitly via
	// the Service's ProbeSocket method.
	s.ProbeSocket(context.Background())
	if !s.SocketWritable() {
		t.Fatalf("expected socketWritable=true after probe")
	}
}

func TestService_LifecycleOps_AgainstFakeDaemon(t *testing.T) {
	// Exercises the Service.*Op closures (the production glue server.go
	// wires into the lifecycle handler) end-to-end against a fake daemon,
	// and pins that Stop/Restart pass the 10s grace as t=10 on the wire.
	var stopT, restartT string
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]ContainerSummary{{ID: "c1", Names: []string{"/sonarr"}}})
	})
	mux.HandleFunc("/v1.41/containers/c1/start", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1.41/containers/c1/stop", func(w http.ResponseWriter, r *http.Request) {
		stopT = r.URL.Query().Get("t")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1.41/containers/c1/restart", func(w http.ResponseWriter, r *http.Request) {
		restartT = r.URL.Query().Get("t")
		w.WriteHeader(http.StatusNoContent)
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	svc := NewService(&config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket})
	if svc.client == nil {
		t.Fatal("Service has no client; setup failure")
	}
	ctx := context.Background()

	id, ok := svc.ResolveContainerID(ctx, "name:sonarr")
	if !ok || id != "c1" {
		t.Fatalf("ResolveContainerID = %q, %v; want c1, true", id, ok)
	}
	if err := svc.StartContainerOp()(ctx, "c1"); err != nil {
		t.Fatalf("StartContainerOp: %v", err)
	}
	if err := svc.StopContainerOp(10)(ctx, "c1"); err != nil {
		t.Fatalf("StopContainerOp: %v", err)
	}
	if err := svc.RestartContainerOp(10)(ctx, "c1"); err != nil {
		t.Fatalf("RestartContainerOp: %v", err)
	}
	if stopT != "10" {
		t.Errorf("stop grace on the wire = %q, want 10", stopT)
	}
	if restartT != "10" {
		t.Errorf("restart grace on the wire = %q, want 10", restartT)
	}
}

func TestService_LifecycleOps_NilClient_ReturnError(t *testing.T) {
	svc := &Service{} // no client built
	ctx := context.Background()
	ops := map[string]func(context.Context, string) error{
		"start":   svc.StartContainerOp(),
		"stop":    svc.StopContainerOp(10),
		"restart": svc.RestartContainerOp(10),
	}
	for name, op := range ops {
		if err := op(ctx, "x"); err == nil {
			t.Errorf("%s op with nil client: want error, got nil (must not panic)", name)
		}
	}
}

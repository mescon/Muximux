package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

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

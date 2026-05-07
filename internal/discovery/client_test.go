package discovery

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

func TestParseEndpoint(t *testing.T) {
	cases := []struct {
		in         string
		wantScheme string
		wantAddr   string
		wantErr    bool
	}{
		{"unix:///var/run/docker.sock", "unix", "/var/run/docker.sock", false},
		{"unix:///tmp/d.sock", "unix", "/tmp/d.sock", false},
		{"tcp://10.0.0.5:2376", "tcp", "10.0.0.5:2376", false},
		{"tcp://docker.local:2375", "tcp", "docker.local:2375", false},
		{"unix://", "", "", true},
		{"tcp://", "", "", true},
		{"npipe:///pipe/docker_engine", "", "", true},
		{"http://localhost", "", "", true},
		{"", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			scheme, addr, err := parseEndpoint(c.in)
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", c.in)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if scheme != c.wantScheme {
				t.Errorf("scheme = %q, want %q", scheme, c.wantScheme)
			}
			if addr != c.wantAddr {
				t.Errorf("addr = %q, want %q", addr, c.wantAddr)
			}
		})
	}
}

func TestNewClient_RejectsEmptyEndpoint(t *testing.T) {
	_, err := NewClient(&config.DiscoveryDockerConfig{})
	if err == nil {
		t.Error("expected error for empty endpoint")
	}
}

func TestNewClient_RejectsUnsupportedScheme(t *testing.T) {
	_, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "ssh://docker@host"})
	if err == nil {
		t.Error("expected error for ssh:// scheme")
	}
}

// fakeDockerOverUnix sets up a unix-socket HTTP server that mimics the
// Docker engine API responses we actually call (/_ping, /version,
// /containers/json, /containers/{id}/json). Returns the socket path
// and a cleanup function.
func fakeDockerOverUnix(t *testing.T, handler http.Handler) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	socket := filepath.Join(dir, "docker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{Handler: handler} //nolint:gosec // test-only; no need to set timeouts
	go func() { _ = srv.Serve(listener) }()
	return socket, func() {
		_ = srv.Close()
		_ = os.Remove(socket)
	}
}

func TestClient_Ping_OverUnixSocket(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	c, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix://" + socket})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestClient_Ping_DaemonReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	c, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix://" + socket})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Ping(context.Background()); err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestClient_Ping_SocketMissing(t *testing.T) {
	c, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix:///tmp/nope-" + t.Name() + ".sock"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Ping(context.Background()); err == nil {
		t.Error("expected error for missing socket")
	}
}

func TestClient_Version(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.41/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(VersionInfo{
			Version: "24.0.7", APIVersion: "1.43", OS: "linux", Arch: "amd64",
		})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	c, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix://" + socket})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v.APIVersion != "1.43" {
		t.Errorf("APIVersion = %q, want %q", v.APIVersion, "1.43")
	}
	if v.Version != "24.0.7" {
		t.Errorf("Version = %q, want %q", v.Version, "24.0.7")
	}
}

func TestClient_ListContainers_NoFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]ContainerSummary{
			{ID: "abc123", Names: []string{"/sonarr"}, Image: "linuxserver/sonarr"},
			{ID: "def456", Names: []string{"/radarr"}, Image: "linuxserver/radarr"},
		})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	c, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix://" + socket})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	containers, err := c.ListContainers(context.Background(), ListContainersOpts{})
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("want 2 containers, got %d", len(containers))
	}
}

func TestClient_ListContainers_FilterByNetwork(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]ContainerSummary{
			{
				ID:    "in-target",
				Names: []string{"/sonarr"},
				NetworkSettings: ContainerNetworks{
					Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.5"}},
				},
			},
			{
				ID:    "out-of-target",
				Names: []string{"/postgres"},
				NetworkSettings: ContainerNetworks{
					Networks: map[string]ContainerNetwork{"backend": {IPAddress: "10.0.1.5"}},
				},
			},
		})
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	c, _ := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix://" + socket})
	got, err := c.ListContainers(context.Background(), ListContainersOpts{Network: "media"})
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(got) != 1 || got[0].ID != "in-target" {
		t.Errorf("want one container 'in-target', got %+v", got)
	}
}

func TestClient_InspectContainer_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.41/containers/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	c, _ := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix://" + socket})
	_, err := c.InspectContainer(context.Background(), "abc123")
	if err == nil || !errorsIsContainerNotFound(err) {
		t.Errorf("want ErrContainerNotFound, got %v", err)
	}
}

func errorsIsContainerNotFound(err error) bool {
	return err != nil && err.Error() == ErrContainerNotFound.Error()
}

func TestContainerSummary_PrimaryName(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"/sonarr"}, "sonarr"},
		{[]string{"/sonarr", "/sonarr-alias"}, "sonarr"},
		{[]string{"sonarr"}, "sonarr"}, // no leading slash
	}
	for _, c := range cases {
		got := (&ContainerSummary{Names: c.in}).PrimaryName()
		if got != c.want {
			t.Errorf("Names=%v PrimaryName=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeBody(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"trim only", "  hello  ", "hello"},
		{"newlines collapse", "a\nb\rc\td", "a b c d"},
		{"runs squeezed", "a   b", "a b"},
		{"truncated with ellipsis", strings.Repeat("x", 300), strings.Repeat("x", 256) + "…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeBody([]byte(c.in))
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestClient_TCP_Roundtrip exercises the tcp:// (no TLS) path against
// a httptest.Server, which is enough to prove the dial path differs
// from unix-socket. The TLS path is exercised by buildTLSConfig tests.
func TestClient_TCP_Roundtrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	c, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "tcp://" + host})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestBuildTLSConfig_RequiresAllThreePaths(t *testing.T) {
	cases := []config.DiscoveryTLSConfig{
		{Enabled: true},                                    // all empty
		{Enabled: true, ClientCert: "/x"},                  // missing key + ca
		{Enabled: true, ClientCert: "/x", ClientKey: "/y"}, // missing ca
	}
	for i, c := range cases {
		_, err := buildTLSConfig(&c)
		if err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

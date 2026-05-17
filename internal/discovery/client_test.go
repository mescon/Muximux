package discovery

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		// Windows named pipe URI form. parseEndpoint converts the
		// `//./pipe/foo` URL path into Windows form `\\.\pipe\foo`.
		{"npipe:////./pipe/docker_engine", "npipe", `\\.\pipe\docker_engine`, false},
		{"unix://", "", "", true},
		{"tcp://", "", "", true},
		{"npipe://", "", "", true},
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

func TestClient_ListNetworks_ReturnsNamesOnly(t *testing.T) {
	// Docker's GET /networks returns rich objects (IPAM, driver, labels
	// etc.). We only need the names for the Settings UI's
	// network_filter autocomplete; the test pins that contract so we
	// don't accidentally start leaking the rest into the wire payload.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.41/networks", func(w http.ResponseWriter, _ *http.Request) {
		// Mix of well-formed entries, an empty-name entry (Docker has
		// been seen to return these mid-network-create), and an entry
		// with extra fields we expect to ignore.
		_, _ = w.Write([]byte(`[
			{"Name":"bridge","Driver":"bridge"},
			{"Name":"host","Driver":"host"},
			{"Name":"media","Driver":"bridge","Labels":{"com.docker.compose.project":"arr"}},
			{"Name":""}
		]`))
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()

	c, err := NewClient(&config.DiscoveryDockerConfig{Endpoint: "unix://" + socket})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	names, err := c.ListNetworks(context.Background())
	if err != nil {
		t.Fatalf("ListNetworks: %v", err)
	}
	want := []string{"bridge", "host", "media"}
	if len(names) != len(want) {
		t.Fatalf("want %d names %v, got %d %v", len(want), want, len(names), names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("position %d: want %q, got %q", i, n, names[i])
		}
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

// TestClient_TLS_MutualAuthRoundtrip proves the end-to-end behavior
// of an mTLS (dockerd --tlsverify) endpoint: our client presents its
// signed cert (so the daemon accepts it) AND validates the daemon's
// server cert against the operator-supplied CA (so we wouldn't talk
// to a MITM). Both arms of the handshake have to succeed; the test
// then sends a real /_ping over the TLS connection.
//
// All material is generated in-memory so the test is hermetic and
// doesn't depend on the host's CA store.
func TestClient_TLS_MutualAuthRoundtrip(t *testing.T) {
	caPEM, caKey := makeCA(t, "muximux-test-ca")
	srvCertPEM, srvKeyPEM := makeLeaf(t, "127.0.0.1", caPEM, caKey, true /* isServer */, []net.IP{net.IPv4(127, 0, 0, 1)})
	cliCertPEM, cliKeyPEM := makeLeaf(t, "muximux-client", caPEM, caKey, false, nil)

	// Server: require + verify a client cert signed by our CA.
	serverCert, err := tls.X509KeyPair(srvCertPEM, srvKeyPEM)
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewUnstartedServer(mux)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}
	srv.StartTLS()
	defer srv.Close()

	// Persist client material to disk so we exercise the real config
	// path (file-based cert/key/CA loading) end-to-end.
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	cliCertPath := filepath.Join(dir, "client.pem")
	cliKeyPath := filepath.Join(dir, "client.key")
	for path, data := range map[string][]byte{caPath: caPEM, cliCertPath: cliCertPEM, cliKeyPath: cliKeyPEM} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// httptest URLs come back as https://127.0.0.1:PORT; reuse the
	// port for our tcp:// endpoint.
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse srv URL: %v", err)
	}

	c, err := NewClient(&config.DiscoveryDockerConfig{
		Endpoint: "tcp://" + u.Host,
		TLS: config.DiscoveryTLSConfig{
			Enabled:    true,
			CACert:     caPath,
			ClientCert: cliCertPath,
			ClientKey:  cliKeyPath,
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping over mTLS: %v", err)
	}
}

// TestClient_TLS_RejectsCertFromUntrustedCA verifies the validate-
// the-server arm of the contract: if the daemon presents a cert
// signed by a CA we don't trust, the handshake must fail. This is
// the "I would not talk to a MITM" guarantee that mutual TLS gives
// us beyond plain "TCP works."
func TestClient_TLS_RejectsCertFromUntrustedCA(t *testing.T) {
	// Two independent CAs: "real" signs the server, "trusted" is the
	// one we hand the client. The two must not validate each other.
	realCAPEM, realCAKey := makeCA(t, "real-ca")
	trustedCAPEM, trustedCAKey := makeCA(t, "trusted-ca")

	srvCertPEM, srvKeyPEM := makeLeaf(t, "127.0.0.1", realCAPEM, realCAKey, true, []net.IP{net.IPv4(127, 0, 0, 1)})
	cliCertPEM, cliKeyPEM := makeLeaf(t, "muximux-client", trustedCAPEM, trustedCAKey, false, nil)

	serverCert, err := tls.X509KeyPair(srvCertPEM, srvKeyPEM)
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := httptest.NewUnstartedServer(mux)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	srv.StartTLS()
	defer srv.Close()

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	cliCertPath := filepath.Join(dir, "client.pem")
	cliKeyPath := filepath.Join(dir, "client.key")
	for path, data := range map[string][]byte{caPath: trustedCAPEM, cliCertPath: cliCertPEM, cliKeyPath: cliKeyPEM} {
		_ = os.WriteFile(path, data, 0o600)
	}

	u, _ := url.Parse(srv.URL)
	c, err := NewClient(&config.DiscoveryDockerConfig{
		Endpoint: "tcp://" + u.Host,
		TLS: config.DiscoveryTLSConfig{
			Enabled:    true,
			CACert:     caPath,
			ClientCert: cliCertPath,
			ClientKey:  cliKeyPath,
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	err = c.Ping(context.Background())
	if err == nil {
		t.Fatalf("Ping succeeded but server cert was signed by an untrusted CA; mTLS validate-server arm is broken")
	}
	// Be specific: the failure should be a TLS verification error,
	// not a connection-refused or timeout. Anything else suggests
	// the test setup is wrong, not the production code.
	if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "tls") {
		t.Errorf("expected TLS verification error, got: %v", err)
	}
}

// makeCA produces a self-signed CA certificate (PEM) and its
// in-memory ECDSA P-256 private key. Used to sign leaves for the
// mTLS roundtrip tests.
func makeCA(t *testing.T, commonName string) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("sign CA: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), key
}

// makeLeaf signs a server or client cert under the given CA. ips is
// only used for server certs (Go's TLS verifier checks IP SANs when
// connecting to a raw IP host); client certs leave it nil.
func makeLeaf(t *testing.T, commonName string, caCertPEM []byte, caKey *ecdsa.PrivateKey, isServer bool, ips []net.IP) ([]byte, []byte) {
	t.Helper()
	caBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("parse CA: %v", err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		IPAddresses:  ips,
	}
	if isServer {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("sign leaf: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

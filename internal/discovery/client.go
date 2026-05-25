// Package discovery provides on-demand enumeration of services
// running in a Docker daemon and an opt-in refresh poller that keeps
// already-imported apps and gateway sites pointed at the right
// container IP/port across restarts.
//
// We talk to the Docker engine API over raw HTTP rather than
// importing the official docker/docker SDK. The SDK pulls in a large
// transitive dependency tree (~50k LoC, swarm types, k8s adapters,
// etc.) for a use case where we only need three endpoints: /version,
// /containers/json, /containers/{id}/json. Raw HTTP keeps the
// surface narrow and the binary lean.
package discovery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
)

// dockerAPIVersion is the engine API version Muximux compiles against.
// 1.41 ships with Docker 20.10 (October 2020), so anything modern is
// covered. Older daemons reject the version with HTTP 400 from /version
// and the capability check surfaces the failure cleanly.
const dockerAPIVersion = "v1.41"

// ErrSelfNotIdentified is returned by InspectSelf when none of the
// fallback strategies (cgroups v1, /etc/hostname cross-check,
// hostname-against-container-IDs) succeed. The scan path treats this
// as a hard block when network_strategy needs network membership.
var ErrSelfNotIdentified = errors.New("could not identify self container")

// ErrContainerNotFound is returned when a tracked container key no
// longer resolves to a running container. The poller logs a Warn but
// leaves the tracked App in place so the operator can recover when
// the container comes back.
var ErrContainerNotFound = errors.New("container not found")

// Client is a thin wrapper around an *http.Client that talks to the
// Docker engine API over either a unix socket or a tcp(+TLS) endpoint.
// All calls take a context so callers can cancel slow daemons.
type Client struct {
	httpClient *http.Client
	baseURL    string // e.g. "http://docker" for unix socket, "https://10.0.0.5:2376" for tcp+tls
}

// NewClient builds a Client from a DiscoveryDockerConfig. It does not
// dial anything - the first network call happens on Ping/Version/etc.
// Returns an error only when the configuration is structurally invalid
// (unknown scheme, unparseable endpoint, TLS cert files missing when
// TLS is enabled).
func NewClient(cfg *config.DiscoveryDockerConfig) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("discovery.docker config is nil")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("discovery.docker.endpoint is empty")
	}
	scheme, addr, err := parseEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		MaxIdleConns:        4,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // Docker payloads are small JSON; gzip costs more than it saves
		TLSHandshakeTimeout: 10 * time.Second,
	}

	var baseURL string
	switch scheme {
	case "unix":
		// Override the dialer to use the unix socket regardless of
		// the host portion of the URL. The baseURL keeps a synthetic
		// "http://docker" so the http.Client URL parser is happy.
		socket := addr
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		}
		baseURL = "http://docker"
	case "npipe":
		// Windows named pipe transport. dialNpipe has a Windows-only
		// implementation that uses go-winio; on other platforms it
		// returns an explanatory error so the UI surfaces a clear
		// "this scheme is Windows-only" message rather than a
		// confusing connect failure.
		pipe := addr
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialNpipe(ctx, pipe)
		}
		baseURL = "http://docker"
	case "tcp":
		if cfg.TLS.Enabled {
			tlsConf, err := buildTLSConfig(&cfg.TLS)
			if err != nil {
				return nil, fmt.Errorf("docker tls: %w", err)
			}
			transport.TLSClientConfig = tlsConf
			baseURL = "https://" + addr
		} else {
			baseURL = "http://" + addr
		}
	default:
		return nil, fmt.Errorf("docker endpoint scheme %q not supported (want unix://, npipe://, or tcp://)", scheme)
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
		baseURL: baseURL,
	}, nil
}

// parseEndpoint splits a Docker endpoint URI into scheme and address.
// Supports unix:///var/run/docker.sock (Linux/macOS),
// npipe:////./pipe/docker_engine (Windows), and tcp://host:port.
func parseEndpoint(endpoint string) (scheme, addr string, err error) {
	// npipe URIs like `npipe:////./pipe/docker_engine` look like four
	// leading slashes; url.Parse handles them as scheme + path, but the
	// canonical form on Windows is `\\.\pipe\docker_engine` which we
	// reconstruct from the trimmed path.
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("parse endpoint %q: %w", endpoint, err)
	}
	switch u.Scheme {
	case "unix":
		// url.Parse puts the path on u.Path for unix:///abs/path
		if u.Path == "" {
			return "", "", fmt.Errorf("unix endpoint missing socket path: %q", endpoint)
		}
		return "unix", u.Path, nil
	case "npipe":
		// Path-only after the scheme: `npipe:////./pipe/docker_engine`
		// → u.Path == "//./pipe/docker_engine". Trim the leading
		// double slash and convert to Windows form `\\.\pipe\...`.
		raw := u.Path
		if raw == "" {
			return "", "", fmt.Errorf("npipe endpoint missing pipe path: %q", endpoint)
		}
		raw = strings.TrimPrefix(raw, "//")
		pipe := `\\` + strings.ReplaceAll(raw, "/", `\`)
		return "npipe", pipe, nil
	case "tcp":
		if u.Host == "" {
			return "", "", fmt.Errorf("tcp endpoint missing host:port: %q", endpoint)
		}
		return "tcp", u.Host, nil
	default:
		return "", "", fmt.Errorf("docker endpoint scheme %q not supported", u.Scheme)
	}
}

// buildTLSConfig assembles a *tls.Config from the cert / key / CA
// paths in DiscoveryTLSConfig. All three must be readable when
// TLS.Enabled is true; we surface the first failure with a clear
// message.
func buildTLSConfig(t *config.DiscoveryTLSConfig) (*tls.Config, error) {
	if t.ClientCert == "" || t.ClientKey == "" || t.CACert == "" {
		return nil, fmt.Errorf("tls.enabled requires client_cert, client_key and ca_cert")
	}
	cert, err := tls.LoadX509KeyPair(t.ClientCert, t.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("load client keypair: %w", err)
	}
	caBytes, err := os.ReadFile(t.CACert) //nolint:gosec // operator-supplied path; checked at config validate time
	if err != nil {
		return nil, fmt.Errorf("read ca_cert %q: %w", t.CACert, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("ca_cert %q contains no valid PEM certs", t.CACert)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// VersionInfo is the subset of /version we care about for the
// capability check.
type VersionInfo struct {
	Version    string `json:"Version"`    // daemon version, e.g. "24.0.7"
	APIVersion string `json:"ApiVersion"` // wire API version, e.g. "1.43"
	OS         string `json:"Os"`         // "linux" / "windows"
	Arch       string `json:"Arch"`       // "amd64", "arm64", ...
}

// Ping confirms the daemon is reachable. Cheaper than Version because
// the response body is empty. Used by the capability cache.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.baseURL+"/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("docker ping: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker ping returned %d", resp.StatusCode)
	}
	return nil
}

// Version returns daemon version info. Used by the Settings tab to
// display "connected to Docker 24.0.7 (API 1.43)".
func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	var v VersionInfo
	if err := c.getJSON(ctx, "/"+dockerAPIVersion+"/version", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// ContainerSummary mirrors the subset of /containers/json we use.
// We deliberately include only the fields the suggest / catalog /
// urlBuilder pipeline reads, so future Docker API additions don't
// silently bloat our internal state.
type ContainerSummary struct {
	ID              string            `json:"Id"`
	Names           []string          `json:"Names"` // includes leading "/" - strip before use
	Image           string            `json:"Image"` // "linuxserver/sonarr:latest"
	ImageID         string            `json:"ImageID"`
	State           string            `json:"State"`  // "running", "exited", ...
	Status          string            `json:"Status"` // human-readable, e.g. "Up 2 days"
	Ports           []ContainerPort   `json:"Ports"`
	Labels          map[string]string `json:"Labels"`
	NetworkSettings ContainerNetworks `json:"NetworkSettings"`
}

// ContainerPort is one entry in the Ports array. PublicPort is 0 when
// the container's port isn't published to the host.
type ContainerPort struct {
	PrivatePort uint16 `json:"PrivatePort"` // container-internal port
	PublicPort  uint16 `json:"PublicPort"`  // host-mapped port (0 = not published)
	Type        string `json:"Type"`        // "tcp" / "udp"
	IP          string `json:"IP"`          // host IP for the published port
}

// ContainerNetworks is the per-network info embedded in the summary.
type ContainerNetworks struct {
	Networks map[string]ContainerNetwork `json:"Networks"`
}

// ContainerNetwork is the per-network entry; IPAddress is what the
// container_ip strategy reads. Empty when the container is on the
// host network or has no network attachment.
type ContainerNetwork struct {
	IPAddress  string `json:"IPAddress"`
	NetworkID  string `json:"NetworkID"`
	EndpointID string `json:"EndpointID"`
	Gateway    string `json:"Gateway"`
}

// PrimaryName returns the first Names entry without the leading "/"
// that Docker prepends. Empty if no name is set.
func (c *ContainerSummary) PrimaryName() string {
	for _, n := range c.Names {
		if n == "" {
			continue
		}
		return strings.TrimPrefix(n, "/")
	}
	return ""
}

// ListContainersOpts narrows what /containers/json returns. We default
// to all=false (running only) because tracking stopped containers
// would be misleading - stopped means no IP, and the URL we'd build
// from a stopped container would 404 every request.
type ListContainersOpts struct {
	All     bool   // include non-running containers; default false
	Network string // filter to one docker network name; empty = no filter (apply network_filter from config)
}

// ListContainers returns running containers from the daemon. The
// network filter is applied client-side (Docker's filter API uses
// a different format and we want consistent behaviour across versions).
func (c *Client) ListContainers(ctx context.Context, opts ListContainersOpts) ([]ContainerSummary, error) {
	q := url.Values{}
	if opts.All {
		q.Set("all", "1")
	}
	path := "/" + dockerAPIVersion + "/containers/json"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var containers []ContainerSummary
	if err := c.getJSON(ctx, path, &containers); err != nil {
		return nil, err
	}
	if opts.Network == "" {
		return containers, nil
	}
	filtered := containers[:0]
	for i := range containers {
		if _, ok := containers[i].NetworkSettings.Networks[opts.Network]; ok {
			filtered = append(filtered, containers[i])
		}
	}
	return filtered, nil
}

// InspectContainer returns the full /containers/{id}/json payload as
// raw JSON. Callers that need typed fields decode into their own
// struct - inspect responses are large and we don't want to maintain
// a mirror of every field.
func (c *Client) InspectContainer(ctx context.Context, id string) (json.RawMessage, error) {
	path := "/" + dockerAPIVersion + "/containers/" + url.PathEscape(id) + "/json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker inspect %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrContainerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("docker inspect %s returned %d: %s", id, resp.StatusCode, sanitizeBody(body))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap on inspect payload
}

// postContainerAction is the shared shape behind Start / Stop /
// Restart. The Docker engine API uses POST verbs with empty bodies
// and HTTP 204 on success; 304 means the container was already in
// the requested state, which we collapse into success because the
// operator's intent is satisfied either way.
//
// The 35s timeout (vs the 15s default on the package's main client)
// is set per-call: graceful stop can take up to the configured
// stop_grace_period (default 10s) plus daemon overhead, and a
// container that is *just barely* shutting down still needs to
// resolve before we surface a timeout error.
func (c *Client) postContainerAction(ctx context.Context, id, action string, query url.Values) error {
	path := "/" + dockerAPIVersion + "/containers/" + url.PathEscape(id) + "/" + action
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	cl := c.httpClient
	if cl.Timeout < 35*time.Second {
		// Shadow the client with a longer timeout for this single
		// call without mutating the package-level client used by
		// discovery.
		cl = &http.Client{Transport: cl.Transport, Timeout: 35 * time.Second}
	}
	resp, err := cl.Do(req)
	if err != nil {
		return fmt.Errorf("docker %s %s: %w", action, id, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusNotModified:
		return nil
	case http.StatusNotFound:
		return ErrContainerNotFound
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("docker %s %s returned %d: %s", action, id, resp.StatusCode, sanitizeBody(body))
	}
}

// StartContainer asks the daemon to start the given container. A 304
// (already running) is treated as success.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.postContainerAction(ctx, id, "start", nil)
}

// StopContainer requests a graceful stop. timeoutSec is the SIGTERM->SIGKILL
// grace window; 0 or negative falls back to Docker's default of 10s
// (we pass it explicitly so the daemon's compiled-in default change
// across versions cannot move the goalposts under us).
func (c *Client) StopContainer(ctx context.Context, id string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	q := url.Values{}
	q.Set("t", fmt.Sprintf("%d", timeoutSec))
	return c.postContainerAction(ctx, id, "stop", q)
}

// RestartContainer is equivalent to Stop + Start but is a single daemon
// call. timeoutSec semantics match StopContainer.
func (c *Client) RestartContainer(ctx context.Context, id string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	q := url.Values{}
	q.Set("t", fmt.Sprintf("%d", timeoutSec))
	return c.postContainerAction(ctx, id, "restart", q)
}

// networkSummary is the shape we extract from Docker's GET /networks
// response. The endpoint returns a lot more (driver, IPAM config,
// labels, etc.) but the Settings UI only needs the name. Worth
// keeping the type narrow so changes to the upstream payload don't
// invalidate cached decodes.
type networkSummary struct {
	Name string `json:"Name"`
}

// ListNetworks returns the names of every Docker network visible
// to this daemon. Used by the Settings UI to drive the
// network_filter input's autocomplete -- without it the operator
// has to guess at network names. Empty names (Docker has been
// observed to return empty entries for in-flight networks) are
// filtered out so the UI never offers an unhelpful choice.
func (c *Client) ListNetworks(ctx context.Context) ([]string, error) {
	var nets []networkSummary
	if err := c.getJSON(ctx, "/"+dockerAPIVersion+"/networks", &nets); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(nets))
	for _, n := range nets {
		if n.Name == "" {
			continue
		}
		out = append(out, n.Name)
	}
	return out, nil
}

// getJSON does GET + JSON decode with the standard error envelope.
// Used by the small endpoints; large endpoints (inspect) read raw.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("docker GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("docker GET %s returned %d: %s", path, resp.StatusCode, sanitizeBody(body))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

// sanitizeBody collapses CR/LF and caps the length so a multi-KiB
// HTML error page doesn't produce an unreadable multi-line error.
// Borrowed from the OIDC discovery sanitiser pattern.
func sanitizeBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	if len(s) > 256 {
		s = s[:256] + "…"
	}
	return s
}

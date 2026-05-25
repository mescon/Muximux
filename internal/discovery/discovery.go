package discovery

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
)

// Service is the entry point used by HTTP handlers and the (later)
// refresh poller. It owns the docker client, caches capability
// results to keep the /status endpoint cheap, and tracks the
// last-divergence state for the Settings banner.
//
// Service is constructed once per Server lifecycle. When discovery
// config changes at runtime the Server replaces the Service rather
// than mutating the existing one - the client transport may need
// to be rebuilt (TLS settings, endpoint scheme), and rebuild-on-swap
// is simpler than diff-and-reconfigure.
type Service struct {
	mu     sync.RWMutex
	cfg    config.DiscoveryDockerConfig
	client *Client // nil when discovery is disabled or NewClient failed

	// Capability cache. statusCacheTTL is short (30s) because the
	// /status endpoint is hit on every Settings page load and the
	// daemon ping is cheap but we still want to avoid hammering it.
	statusCache    StatusResult
	statusCachedAt time.Time

	// Self-detect cache. The container we're running in doesn't
	// change at runtime, so cache forever (until Service is rebuilt).
	selfInfo    *SelfInfo
	selfErr     error
	selfChecked bool

	// Refresh-state telemetry. Populated by the poller via
	// RecordRefreshTick / RecordDivergence. In-memory only - resets
	// on restart. The durable record of a divergence is the audit log
	// line emitted by the poller, captured in data/muximux.log.
	refreshMu            sync.Mutex
	divergences          int
	lastDivergenceAt     time.Time
	recoveredAt          time.Time
	lastRefreshSuccessAt time.Time

	// LastSeenAt tracks when each docker_key was last successfully
	// resolved against the daemon. Used to render "last refresh"
	// timestamps in the Currently-tracked panel. Keyed on docker key
	// (e.g. "label:sonarr-prod"). DELETE /track/{key} clears the
	// matching entry to keep the map bounded as operators track
	// and detach over time.
	lastSeenAt sync.Map // map[string]time.Time

	// dockerState is the poller-managed snapshot of every tracked
	// app's container state. Replaced wholesale by the poller each
	// tick; individual entries are updated by the lifecycle handlers
	// after a successful Start/Stop/Restart. Reads are RLocked behind
	// dockerStateMu.
	dockerStateMu sync.RWMutex
	dockerState   dockerStateCache

	// socketWritable is set once at Service construction by the
	// capability probe. The lifecycle handlers return 503 when this
	// is false; the Settings tab surfaces "socket is read-only" as a
	// status line.
	socketWritable bool
}

const statusCacheTTL = 30 * time.Second

// StatusResult is the body returned by GET /api/discovery/docker/status.
//
// The four-state UI gating ladder reads:
//
//	Configured == false                                                -> CTA mode ("Set up Docker discovery")
//	Configured && !Reachable                                           -> disabled with tooltip
//	Configured &&  Reachable && !StrategyOK                            -> disabled with tooltip
//	Configured &&  Reachable &&  StrategyOK                            -> active
//
// All boolean fields default to false; the JSON omitempty pattern is
// avoided so the frontend can rely on field presence.
type StatusResult struct {
	Configured bool                   `json:"configured"`                   // discovery.docker.enabled
	Reachable  bool                   `json:"reachable"`                    // last Ping succeeded
	StrategyOK bool                   `json:"strategy_ok"`                  // selfDetect succeeded for network strategies
	Endpoint   string                 `json:"endpoint,omitempty"`           // configured endpoint string
	APIVersion string                 `json:"api_version,omitempty"`        // from /version
	Strategy   config.NetworkStrategy `json:"strategy,omitempty"`           // configured network strategy
	SelfDetect string                 `json:"self_detect_method,omitempty"` // see SelfDetectMethod
	LastError  string                 `json:"last_error,omitempty"`         // human-readable cause when !Reachable

	// Refresh state surfaced for the Settings banner.
	Divergences          int    `json:"refresh_divergences,omitempty"`
	LastDivergenceAt     string `json:"last_divergence_at,omitempty"`
	RecoveredAt          string `json:"recovered_at,omitempty"`
	LastRefreshSuccessAt string `json:"last_refresh_at,omitempty"`

	// TLS hygiene warning (non-blocking). Surfaces e.g.
	// "client_key permissions are world-readable; chmod 600 recommended".
	TLSWarning string `json:"tls_warning,omitempty"`
}

// NewService constructs a Service from a discovery config. Always
// returns a non-nil *Service - even when the config is invalid or
// disabled - so callers don't need nil checks at every call site.
// When the client cannot be built, the error is captured in
// statusCache.LastError and surfaced via Status().
func NewService(cfg *config.DiscoveryDockerConfig) *Service {
	if cfg == nil {
		return &Service{}
	}
	s := &Service{cfg: *cfg}
	if cfg.Enabled && cfg.Endpoint != "" {
		client, err := NewClient(cfg)
		if err == nil {
			s.client = client
		} else {
			s.statusCache = StatusResult{
				Configured: true,
				Endpoint:   cfg.Endpoint,
				Strategy:   cfg.NetworkStrategy,
				LastError:  err.Error(),
			}
			s.statusCachedAt = time.Now()
		}
	}
	return s
}

// Status returns the cached capability state, refreshing the cache
// from the daemon if it's stale. Cheap to call repeatedly.
func (s *Service) Status(ctx context.Context) StatusResult {
	s.mu.RLock()
	cached := s.statusCache
	cachedAt := s.statusCachedAt
	clientPresent := s.client != nil
	s.mu.RUnlock()

	if !s.cfg.Enabled {
		// Disabled - no client, no probe.
		return StatusResult{
			Configured: false,
			Endpoint:   s.cfg.Endpoint,
			Strategy:   s.cfg.NetworkStrategy,
		}
	}
	if !clientPresent {
		// NewService failed to build a client; surface the error.
		return cached
	}
	if time.Since(cachedAt) < statusCacheTTL && cached.Endpoint != "" {
		// Cache covers the expensive probes (ping, version, self-
		// detect). Divergence + refresh-success timestamps are cheap
		// to read; keep them current on every call so the banner
		// transitions promptly when a new divergence fires inside the
		// cache window.
		s.fillRefreshTelemetry(&cached)
		return cached
	}
	return s.refreshStatus(ctx)
}

// fillRefreshTelemetry stamps the live divergence + refresh-success
// fields onto the given StatusResult.
func (s *Service) fillRefreshTelemetry(r *StatusResult) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	r.Divergences = s.divergences
	r.LastDivergenceAt = ""
	r.RecoveredAt = ""
	r.LastRefreshSuccessAt = ""
	if !s.lastDivergenceAt.IsZero() {
		r.LastDivergenceAt = s.lastDivergenceAt.Format(time.RFC3339)
	}
	if !s.recoveredAt.IsZero() {
		r.RecoveredAt = s.recoveredAt.Format(time.RFC3339)
	}
	if !s.lastRefreshSuccessAt.IsZero() {
		r.LastRefreshSuccessAt = s.lastRefreshSuccessAt.Format(time.RFC3339)
	}
}

// RecordRefreshTickSuccess marks now as the most recent successful
// refresh. The poller calls this at the end of every clean tick.
func (s *Service) RecordRefreshTickSuccess() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.lastRefreshSuccessAt = time.Now()
	if !s.lastDivergenceAt.IsZero() && s.recoveredAt.IsZero() {
		// First clean tick after a divergence - record recovery so
		// the banner can transition from active to recovered state.
		s.recoveredAt = time.Now()
	}
}

// RecordDivergence increments the in-memory counter and stamps the
// timestamp. Called by the poller when proxy.ApplyGatewaySites
// returns ErrDiverged.
func (s *Service) RecordDivergence() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.divergences++
	s.lastDivergenceAt = time.Now()
	// New divergence wipes the recovered marker so the banner
	// transitions back to "active".
	s.recoveredAt = time.Time{}
}

// RecordSeen stamps a tracked key as last-resolved at now. Poller
// calls this for every container it successfully looks up.
func (s *Service) RecordSeen(key string) {
	s.lastSeenAt.Store(key, time.Now())
}

// ForgetTrackedKey removes the LastSeenAt entry for a detached key.
// Called from the DELETE /track handler to keep the map bounded.
func (s *Service) ForgetTrackedKey(key string) {
	s.lastSeenAt.Delete(key)
}

// LastSeen returns the most recent time the given key was resolved,
// or zero if never seen.
func (s *Service) LastSeen(key string) time.Time {
	if v, ok := s.lastSeenAt.Load(key); ok {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}

// ListLiveContainers proxies to the underlying daemon client. Used by
// the Re-link probe handler so the handlers package can list
// containers without reaching for the unexported client field. The
// returned error is the daemon's, surfaced verbatim by the probe
// handler so the operator sees the actual cause inline.
func (s *Service) ListLiveContainers(ctx context.Context) ([]ContainerSummary, error) {
	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	if c == nil {
		return nil, errClientNotInitialised
	}
	return c.ListContainers(ctx, ListContainersOpts{All: false})
}

// ListNetworks returns the names of every Docker network the
// configured daemon exposes. Powers the network_filter autocomplete
// in the Settings UI so operators pick from real choices instead of
// guessing. Errors are passed through verbatim: if the daemon is
// unreachable the caller already surfaces that elsewhere via Status,
// and a silent empty list here would mask the actual root cause.
func (s *Service) ListNetworks(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	if c == nil {
		return nil, errClientNotInitialised
	}
	return c.ListNetworks(ctx)
}

// errClientNotInitialised is the static error returned by
// ListLiveContainers when the discovery service has no client yet
// (e.g., discovery is enabled but the configured endpoint failed to
// parse). Stable error so handlers can errors.Is-match without
// depending on the message text.
var errClientNotInitialised = errors.New("discovery client not initialised; check Settings → Discovery")

// refreshStatus does the actual probe + selfDetect + TLS-hygiene
// check, then caches the result.
func (s *Service) refreshStatus(ctx context.Context) StatusResult {
	r := StatusResult{
		Configured: true,
		Endpoint:   s.cfg.Endpoint,
		Strategy:   s.cfg.NetworkStrategy,
	}

	// TLS file hygiene runs even when the daemon is unreachable so
	// the operator gets the warning early.
	if w := tlsHygieneWarning(&s.cfg.TLS); w != "" {
		r.TLSWarning = w
	}

	if err := s.client.Ping(ctx); err != nil {
		r.LastError = err.Error()
		s.cacheStatus(&r)
		return r
	}
	r.Reachable = true

	if v, err := s.client.Version(ctx); err == nil {
		r.APIVersion = v.APIVersion
	}

	// Strategy probe: only network-membership strategies need self-detect.
	switch s.cfg.NetworkStrategy {
	case config.StrategyContainerIP, config.StrategyContainerDNS:
		s.mu.RLock()
		checked := s.selfChecked
		info := s.selfInfo
		selfErr := s.selfErr
		s.mu.RUnlock()
		if !checked {
			info, selfErr = s.client.InspectSelf(ctx)
			s.mu.Lock()
			s.selfChecked = true
			s.selfInfo = info
			s.selfErr = selfErr
			s.mu.Unlock()
		}
		switch {
		case selfErr == nil && info != nil:
			r.StrategyOK = true
			r.SelfDetect = string(info.Method)
		case s.cfg.NetworkFilter != "":
			// selfDetect failed but operator scoped via network_filter,
			// which substitutes for self-membership.
			r.StrategyOK = true
			r.SelfDetect = string(SelfDetectNone)
		default:
			r.StrategyOK = false
			r.SelfDetect = string(SelfDetectNone)
		}
	case config.StrategyHostPort, config.StrategyHostDockerInternal:
		// These strategies don't need self-detect.
		r.StrategyOK = true
	default:
		r.StrategyOK = false
		r.LastError = "unknown network_strategy: " + string(s.cfg.NetworkStrategy)
	}

	// Refresh-state telemetry (in-memory; resets on restart). Single
	// source of truth lives in fillRefreshTelemetry so a regression
	// like adding a field here but not on the cache-hit path can't
	// diverge the two callers.
	s.fillRefreshTelemetry(&r)

	s.cacheStatus(&r)
	return r
}

func (s *Service) cacheStatus(r *StatusResult) {
	s.mu.Lock()
	s.statusCache = *r
	s.statusCachedAt = time.Now()
	s.mu.Unlock()
}

// ScanResult is the body returned by GET /api/discovery/docker/scan.
//
// Two shapes:
//
//   - When the daemon is reachable and the strategy is satisfied,
//     Suggestions is populated and ScanBlocked is empty.
//   - When the strategy needs network membership (container_ip /
//     container_dns) and self-detect failed AND no NetworkFilter is
//     set, Suggestions is nil and ScanBlocked carries an actionable
//     message. The frontend renders that message inline rather than
//     showing an empty list.
type ScanResult struct {
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	ScanBlocked string       `json:"scan_blocked,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// Scan enumerates the daemon's running containers and produces a
// suggestion per container. Honors the discovery config's
// NetworkFilter (limits to containers attached to that docker
// network) and refuses to enumerate when network strategy + self-
// detect failure would expose containers across trust boundaries.
//
// dashboardDomain is read from the calling Server's configured
// server.tls.domain so the modal can prefill suggested gateway-site
// domains as "<container>.<dashboard-domain>" without us reaching
// into the wider config from inside this package.
func (s *Service) Scan(ctx context.Context, dashboardDomain string) ScanResult {
	if !s.cfg.Enabled {
		return ScanResult{ScanBlocked: "Docker discovery is disabled. Enable it in Settings → Discovery."}
	}
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return ScanResult{Error: "discovery client not initialised; check Settings → Discovery"}
	}

	// Strategy gating: container_ip / container_dns need either a
	// successful self-detect OR an explicit network_filter. Without
	// either, we'd be enumerating containers across every network
	// visible to the daemon - a scope we don't control.
	switch s.cfg.NetworkStrategy {
	case "", config.StrategyContainerIP, config.StrategyContainerDNS:
		if s.cfg.NetworkFilter == "" {
			s.mu.RLock()
			info := s.selfInfo
			selfErr := s.selfErr
			checked := s.selfChecked
			s.mu.RUnlock()
			if !checked {
				info, selfErr = client.InspectSelf(ctx)
				s.mu.Lock()
				s.selfChecked = true
				s.selfInfo = info
				s.selfErr = selfErr
				s.mu.Unlock()
			}
			if selfErr != nil || info == nil {
				return ScanResult{ScanBlocked: "Could not identify the container Muximux is running in. " +
					"This usually means: (1) cgroups v2 without a recognisable container ID in /proc/self/cgroup, " +
					"or (2) the container was started with --hostname overriding the default. " +
					"To proceed: set discovery.docker.network_filter to scope discovery to a specific docker network, " +
					"or switch network_strategy to host_port."}
			}
		}
	}

	containers, err := client.ListContainers(ctx, ListContainersOpts{
		All:     false,
		Network: s.cfg.NetworkFilter,
	})
	if err != nil {
		return ScanResult{Error: err.Error()}
	}

	out := ScanResult{Suggestions: make([]Suggestion, 0, len(containers))}
	for i := range containers {
		// Skip Muximux's own container - importing it would create
		// an iframe loop. Detected by image-name substring AND
		// container-name substring so the filter survives operator
		// prefix conventions ("homelab-muximux", "homelab_muximux",
		// custom image registries, etc.).
		if isLikelySelf(&containers[i]) {
			continue
		}
		out.Suggestions = append(out.Suggestions, suggestForContainer(
			&containers[i],
			s.cfg.NetworkStrategy,
			s.cfg.HostIP,
			dashboardDomain,
		))
	}
	return out
}

// isLikelySelf reports whether the container is plausibly a Muximux
// instance and should be excluded from the Discover suggestions list.
// We check both image and container name (case-insensitive) so the
// filter doesn't depend on whether the operator pulled the canonical
// ghcr.io/mescon/muximux image or built their own with a custom tag.
func isLikelySelf(c *ContainerSummary) bool {
	if strings.Contains(strings.ToLower(c.Image), "muximux") {
		return true
	}
	if strings.Contains(strings.ToLower(c.PrimaryName()), "muximux") {
		return true
	}
	return false
}

// tlsHygieneWarning reads file modes for the configured client_key
// and returns a non-empty warning string when the key is world-readable.
// Other file errors (missing, unparseable) surface via NewClient and
// land in LastError; this function is just for the soft "fix your
// chmod" prompt.
func tlsHygieneWarning(t *config.DiscoveryTLSConfig) string {
	if t == nil {
		return ""
	}
	if !t.Enabled || t.ClientKey == "" {
		return ""
	}
	info, err := os.Stat(t.ClientKey)
	if err != nil {
		return "" // surface the real error elsewhere
	}
	if mode := info.Mode().Perm(); mode&0o044 != 0 {
		return "client_key permissions are world- or group-readable (mode " + mode.String() + "); chmod 600 recommended"
	}
	return ""
}

// DockerStateForApp returns the cached state for the given app name.
// ok is false when the app is not tracked or the poller has not yet
// inspected it.
func (s *Service) DockerStateForApp(name string) (DockerState, bool) {
	s.dockerStateMu.RLock()
	defer s.dockerStateMu.RUnlock()
	st, ok := s.dockerState[name]
	return st, ok
}

// SetDockerStateForApp inserts or replaces a single entry. The
// lifecycle handlers call this after a successful action so the
// /api/discovery/docker-state endpoint serves fresh data before the
// next poll tick.
func (s *Service) SetDockerStateForApp(name string, st *DockerState) {
	s.dockerStateMu.Lock()
	defer s.dockerStateMu.Unlock()
	if s.dockerState == nil {
		s.dockerState = make(dockerStateCache)
	}
	s.dockerState[name] = *st
}

// SetDockerStateCache replaces the entire cache atomically. The poller
// calls this at the end of each tick with the freshly-built map so
// apps that disappeared from the tracked set are pruned in one step.
func (s *Service) SetDockerStateCache(next dockerStateCache) {
	s.dockerStateMu.Lock()
	defer s.dockerStateMu.Unlock()
	s.dockerState = next
}

// DockerStateSnapshot returns a defensive copy of the current cache
// for callers (handlers, tests) that want to range over the map
// without holding the lock.
func (s *Service) DockerStateSnapshot() map[string]DockerState {
	s.dockerStateMu.RLock()
	defer s.dockerStateMu.RUnlock()
	out := make(map[string]DockerState, len(s.dockerState))
	for k, v := range s.dockerState {
		out[k] = v
	}
	return out
}

// SocketWritable reports whether the daemon socket accepts writes.
// Set once at Service startup; static across the Service's lifetime.
func (s *Service) SocketWritable() bool {
	return s.socketWritable
}

// ResolveContainerID looks up the live container ID for a tracking
// key like "label:sonarr-prod" or "name:/sonarr". Returns ok=false
// when the tracker key is malformed or no running container matches.
// Used by the lifecycle handlers to translate App.DockerKey into the
// id the Docker engine API needs.
func (s *Service) ResolveContainerID(ctx context.Context, key string) (string, bool) {
	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	if c == nil {
		return "", false
	}
	tk, err := ParseTrackingKey(key)
	if err != nil {
		return "", false
	}
	containers, err := c.ListContainers(ctx, ListContainersOpts{All: true})
	if err != nil {
		return "", false
	}
	matched := tk.FindContainer(containers)
	if matched == nil {
		return "", false
	}
	return matched.ID, true
}

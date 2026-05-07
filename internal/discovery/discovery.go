package discovery

import (
	"context"
	"os"
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
	Configured bool   `json:"configured"`                   // discovery.docker.enabled
	Reachable  bool   `json:"reachable"`                    // last Ping succeeded
	StrategyOK bool   `json:"strategy_ok"`                  // selfDetect succeeded for network strategies
	Endpoint   string `json:"endpoint,omitempty"`           // configured endpoint string
	APIVersion string `json:"api_version,omitempty"`        // from /version
	Strategy   string `json:"strategy,omitempty"`           // configured network strategy
	SelfDetect string `json:"self_detect_method,omitempty"` // see SelfDetectMethod
	LastError  string `json:"last_error,omitempty"`         // human-readable cause when !Reachable

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
		return cached
	}
	return s.refreshStatus(ctx)
}

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
	case "container_ip", "container_dns":
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
	case "host_port", "host_docker_internal":
		// These strategies don't need self-detect.
		r.StrategyOK = true
	default:
		r.StrategyOK = false
		r.LastError = "unknown network_strategy: " + s.cfg.NetworkStrategy
	}

	s.cacheStatus(&r)
	return r
}

func (s *Service) cacheStatus(r *StatusResult) {
	s.mu.Lock()
	s.statusCache = *r
	s.statusCachedAt = time.Now()
	s.mu.Unlock()
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

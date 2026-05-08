package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/proxy"
)

// DiscoveryHandler exposes discovery-related HTTP endpoints.
//
// Phase B ships:
//   - GET    /api/discovery/docker/status         capability + cache
//   - PUT    /api/discovery/docker/config         persist + rebuild service
//   - POST   /api/discovery/docker/test           probe a candidate config without persisting
//
// Subsequent phases add /scan, /import, /track/{key}, /refresh, /relink/probe.
type DiscoveryHandler struct {
	config     *config.Config
	configPath string
	configMu   *sync.RWMutex

	// service is rebuilt whenever the operator updates discovery
	// config. The pointer swap is guarded by serviceMu so concurrent
	// Status / Scan / etc. calls always see a consistent service.
	serviceMu sync.RWMutex
	service   *discovery.Service

	// proxyServer is the live Caddy controller. Non-nil when Muximux
	// booted with a proxy configured. Discovery import calls
	// ApplyGatewaySites on this to push newly-imported gateway sites
	// to Caddy without waiting for a restart.
	proxyServer *proxy.Proxy
}

// NewDiscoveryHandler binds the handler to its initial Service plus
// the config + lock it needs to persist updates. The service may be
// nil when discovery isn't enabled at startup; the handler surfaces
// Configured=false in that case. proxyServer may also be nil (no-
// proxy boot); import then skips the Caddy reload step and the
// gateway sites land on disk only.
func NewDiscoveryHandler(svc *discovery.Service, cfg *config.Config, configPath string, configMu *sync.RWMutex, proxyServer *proxy.Proxy) *DiscoveryHandler {
	return &DiscoveryHandler{
		config:      cfg,
		configPath:  configPath,
		configMu:    configMu,
		service:     svc,
		proxyServer: proxyServer,
	}
}

// Service returns the current discovery service pointer. Used by
// later-phase code (refresh poller wiring) to grab the live service
// without going through the handler. Safe to call from any goroutine.
func (h *DiscoveryHandler) Service() *discovery.Service {
	h.serviceMu.RLock()
	defer h.serviceMu.RUnlock()
	return h.service
}

// GetDockerStatus handles GET /api/discovery/docker/status. Admin-only
// at registration. The body is the StatusResult struct directly so
// the frontend gets the four-state UI gating ladder (Configured,
// Reachable, StrategyOK, plus error/warning text).
func (h *DiscoveryHandler) GetDockerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	svc := h.Service()
	if svc == nil {
		// Service is nil only when discovery wasn't configured at
		// startup. Treat as "discovery is off"; the operator can
		// enable it via Settings later (which rebuilds the service).
		sendJSON(w, http.StatusOK, discovery.StatusResult{Configured: false})
		return
	}
	sendJSON(w, http.StatusOK, svc.Status(r.Context()))
}

// UpdateDockerConfig handles PUT /api/discovery/docker/config. The
// body is a config.DiscoveryDockerConfig (full struct, not patch).
// On success the in-memory + on-disk config are updated and the
// discovery service is rebuilt so the next /status reflects the new
// endpoint.
func (h *DiscoveryHandler) UpdateDockerConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var newCfg config.DiscoveryDockerConfig
	if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}
	if err := validateDiscoveryDockerConfig(&newCfg); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "config")
		return
	}

	// Snapshot, mutate, save, rollback on failure - same shape as the
	// auth-config update path. Persist BEFORE rebuilding the service so
	// a save failure leaves the running service untouched.
	h.configMu.Lock()
	prior := h.config.Discovery.Docker
	h.config.Discovery.Docker = newCfg
	if err := h.config.Save(h.configPath); err != nil {
		h.config.Discovery.Docker = prior
		h.configMu.Unlock()
		logging.From(r.Context()).Error("Save discovery config failed; in-memory rolled back",
			"source", "audit",
			"error", err)
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "error", err)
		return
	}
	h.configMu.Unlock()

	// Rebuild the service. NewService is cheap (no network calls)
	// and returns a non-nil pointer even when the new config is
	// disabled or has a structurally bad endpoint.
	newService := discovery.NewService(&newCfg)
	h.serviceMu.Lock()
	h.service = newService
	h.serviceMu.Unlock()

	logging.Audit("Discovery config updated",
		"endpoint", newCfg.Endpoint,
		"strategy", newCfg.NetworkStrategy,
		"enabled", newCfg.Enabled)

	// Return the fresh status so the UI can update without a follow-up GET.
	sendJSON(w, http.StatusOK, newService.Status(r.Context()))
}

// ScanDocker handles GET /api/discovery/docker/scan. Walks the
// configured daemon's running containers and returns a Suggestion per
// container. Refuses to enumerate when the strategy needs network
// membership and self-detect failed (see ScanResult.ScanBlocked).
func (h *DiscoveryHandler) ScanDocker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	svc := h.Service()
	if svc == nil {
		sendJSON(w, http.StatusOK, discovery.ScanResult{
			ScanBlocked: "Docker discovery is not configured. Enable it in Settings → Discovery.",
		})
		return
	}
	// Read the configured tls.domain under configMu so the
	// suggested-gateway-domain default is consistent with the
	// running config (the operator may change it concurrently).
	h.configMu.RLock()
	dashboardDomain := h.config.Server.TLS.Domain
	h.configMu.RUnlock()
	// Apply a request-scoped timeout so a wedged daemon doesn't park
	// the connection until net/http's idle timeout.
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	sendJSON(w, http.StatusOK, svc.Scan(ctx, dashboardDomain))
}

// TestDockerConfig handles POST /api/discovery/docker/test. The body
// is a candidate config.DiscoveryDockerConfig that we probe WITHOUT
// persisting. Lets the operator click "Test connection" before
// hitting Save so they don't blow away their working setup with a
// typo.
func (h *DiscoveryHandler) TestDockerConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var candidate config.DiscoveryDockerConfig
	if err := json.NewDecoder(r.Body).Decode(&candidate); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}
	if err := validateDiscoveryDockerConfig(&candidate); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "config")
		return
	}
	probe := discovery.NewService(&candidate)
	// Use a tighter timeout for the probe than the regular status
	// path - the operator is sitting in front of the modal waiting
	// for an answer.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	sendJSON(w, http.StatusOK, probe.Status(ctx))
}

// validateDiscoveryDockerConfig checks the structural shape of a
// candidate config: endpoint scheme is unix:// or tcp://, strategy is
// one of the four known values, refresh_interval parses (when set).
// More semantic checks (cert paths exist, ip_strategy compatibility)
// happen later in NewClient/NewService and surface via the probe path.
func validateDiscoveryDockerConfig(c *config.DiscoveryDockerConfig) error {
	if !c.Enabled {
		return nil // anything goes when disabled
	}
	if c.Endpoint == "" {
		return errBadDiscoveryEmptyEndpoint
	}
	if !strings.HasPrefix(c.Endpoint, "unix://") && !strings.HasPrefix(c.Endpoint, "tcp://") {
		return errBadDiscoveryEndpointScheme
	}
	switch c.NetworkStrategy {
	case "", "container_ip", "container_dns", "host_port", "host_docker_internal":
		// "" is allowed because config.Load defaults it to container_ip.
	default:
		return errBadDiscoveryNetworkStrategy
	}
	if c.RefreshInterval != "" {
		if _, err := time.ParseDuration(c.RefreshInterval); err != nil {
			return errBadDiscoveryRefreshInterval
		}
	}
	if c.TLS.Enabled {
		if c.TLS.ClientCert == "" || c.TLS.ClientKey == "" || c.TLS.CACert == "" {
			return errBadDiscoveryTLSPaths
		}
	}
	return nil
}

// Sentinel errors give consistent client-facing messages without
// exposing internal validation logic.
var (
	errBadDiscoveryEmptyEndpoint   = sentinelError("discovery.docker.endpoint is required when enabled")
	errBadDiscoveryEndpointScheme  = sentinelError("discovery.docker.endpoint must start with unix:// or tcp://")
	errBadDiscoveryNetworkStrategy = sentinelError("discovery.docker.network_strategy must be container_ip, container_dns, host_port, or host_docker_internal")
	errBadDiscoveryRefreshInterval = sentinelError("discovery.docker.refresh_interval is not a valid duration (e.g. \"60s\")")
	errBadDiscoveryTLSPaths        = sentinelError("discovery.docker.tls.enabled requires ca_cert, client_cert and client_key paths")
)

// sentinelError builds a constant error value usable in respondError
// where the message is the user-facing string.
type sentinelError string

func (e sentinelError) Error() string { return string(e) }

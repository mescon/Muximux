package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/proxy"
)

// DiscoveryHandler exposes discovery-related HTTP endpoints:
//
//   - GET    /api/discovery/docker/status          capability + cache
//   - GET    /api/discovery/docker/config          stored config + env overrides
//   - PUT    /api/discovery/docker/config          merge, persist + rebuild service
//   - POST   /api/discovery/docker/test            probe a candidate config without persisting
//   - GET    /api/discovery/docker/scan            list current containers as Suggestions
//   - POST   /api/discovery/docker/import          atomic batch import
//   - GET    /api/discovery/docker/tracked         current tracked apps + sites
//   - DELETE /api/discovery/docker/track/{key}     detach tracking for matching entries
//   - POST   /api/discovery/docker/relink/probe    re-link target probe
//   - POST   /api/discovery/docker/relink/confirm  apply re-link
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

	// onConfigSave is invoked after every successful config write
	// driven by this handler (currently: ImportDocker, DetachTracked,
	// RelinkConfirm). Wired to the same rebuild-reverse-proxy-routes
	// callback the APIHandler uses, so a freshly imported App.Proxy
	// entry's /proxy/<slug>/ route starts working without a restart.
	onConfigSave func()
}

// SetOnConfigSave installs the post-save callback. server.go calls
// this with the same closure the APIHandler uses (rebuild the
// reverse-proxy route table) so all three mutation paths converge
// on the same hook.
func (h *DiscoveryHandler) SetOnConfigSave(fn func()) {
	h.onConfigSave = fn
}

func (h *DiscoveryHandler) notifyConfigSaved() {
	if h.onConfigSave != nil {
		h.onConfigSave()
	}
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

// ListDockerNetworks handles GET /api/discovery/docker/networks.
// Returns the names of every Docker network the configured daemon
// exposes, used by the Settings UI to power the network_filter
// input's autocomplete. Returns an empty array (not an error) when
// discovery is off so the UI can hide the affordance gracefully;
// daemon-reachability failures bubble up as 502 so the frontend can
// fall back to a free-text input.
func (h *DiscoveryHandler) ListDockerNetworks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	svc := h.Service()
	if svc == nil {
		sendJSON(w, http.StatusOK, struct {
			Networks []string `json:"networks"`
		}{Networks: []string{}})
		return
	}
	names, err := svc.ListNetworks(r.Context())
	if err != nil {
		respondError(w, r, http.StatusBadGateway, "list networks: "+err.Error())
		return
	}
	sendJSON(w, http.StatusOK, struct {
		Networks []string `json:"networks"`
	}{Networks: names})
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

// GetDockerStateMap handles GET /api/discovery/docker-state. Returns
// the current docker-state cache as a map keyed by app name. The
// frontend fetches this once on mount and then receives delta
// updates via the docker_state_changed WebSocket event.
//
// No admin gate: state visibility mirrors HealthIndicator's
// authenticated-user-only level. Mutations remain admin-gated by
// the lifecycle handlers themselves.
func (h *DiscoveryHandler) GetDockerStateMap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	svc := h.Service()
	if svc == nil {
		sendJSON(w, http.StatusOK, map[string]discovery.DockerState{})
		return
	}

	// Filter to apps the caller is allowed to see. The state map is keyed
	// by app name and, without this, exposed the container status, image,
	// uptime, and restart count of every tracked app to any authenticated
	// user -- including apps hidden from them by min_role / allowed_groups.
	// This mirrors the per-app gate the lifecycle (control) handlers apply.
	// Fail closed: a request with no user in context sees nothing.
	full := svc.DockerStateSnapshot()
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		sendJSON(w, http.StatusOK, map[string]discovery.DockerState{})
		return
	}
	h.configMu.RLock()
	accessible := make(map[string]bool, len(h.config.Apps))
	for i := range h.config.Apps {
		if appAccessible(user, &h.config.Apps[i]) {
			accessible[h.config.Apps[i].Name] = true
		}
	}
	h.configMu.RUnlock()

	visible := make(map[string]discovery.DockerState, len(full))
	for name, st := range full {
		if accessible[name] {
			visible[name] = st
		}
	}
	sendJSON(w, http.StatusOK, visible)
}

// dockerConfigResponse is the body of GET /api/discovery/docker/config.
// EnvOverrides names the environment variable behind each field whose live
// value does not come from config.yaml, so the tab can lock it.
type dockerConfigResponse struct {
	Config       config.DiscoveryDockerConfig `json:"config"`
	EnvOverrides map[string]string            `json:"env_overrides,omitempty"`
}

// DockerConfig dispatches /api/discovery/docker/config: GET reads the
// stored config, PUT merges an update onto it.
func (h *DiscoveryHandler) DockerConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetDockerConfig(w, r)
	case http.MethodPut:
		h.UpdateDockerConfig(w, r)
	default:
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

// GetDockerConfig handles GET /api/discovery/docker/config. It returns the
// live discovery.docker block so the Settings tab can seed its form from
// what is stored instead of from defaults.
func (h *DiscoveryHandler) GetDockerConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	h.configMu.RLock()
	resp := dockerConfigResponse{Config: h.config.Discovery.Docker}
	resp.Config.LifecycleAllowedGroups = append([]string(nil), resp.Config.LifecycleAllowedGroups...)
	if src, ok := h.config.EnvOverrides()[string(config.OverrideAutoImport)]; ok {
		resp.EnvOverrides = map[string]string{"auto_import": src}
	}
	if src, ok := h.config.EnvOverrides()[string(config.OverrideRequireExplicitEnable)]; ok {
		if resp.EnvOverrides == nil {
			resp.EnvOverrides = map[string]string{}
		}
		resp.EnvOverrides["require_explicit_enable"] = src
	}
	h.configMu.RUnlock()
	sendJSON(w, http.StatusOK, resp)
}

// UpdateDockerConfig handles PUT /api/discovery/docker/config. The
// body is decoded onto a copy of the stored config.DiscoveryDockerConfig,
// so fields the client omits keep their stored values. The load-time
// defaults are then applied and the result validated. On success the
// in-memory + on-disk config are updated and the discovery service is
// rebuilt so the next /status reflects the new endpoint.
func (h *DiscoveryHandler) UpdateDockerConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	// Read the body before taking the lock, so a slow client never holds it.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	// Decode, merge, save and roll back all under the write lock: two
	// concurrent saves each merge onto what the other stored, instead of
	// onto a stale snapshot that would drop the other's fields.
	h.configMu.Lock()
	newCfg, err := h.mergeDockerConfigLocked(body)
	if err != nil {
		h.configMu.Unlock()
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "config")
		return
	}
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

	// Reconfigure the shared service IN PLACE. The same *Service is held
	// by the poller and the lifecycle op closures, so rebuilding it in
	// place (rather than swapping only this handler's pointer) makes the
	// change take effect everywhere without a restart -- enabling
	// discovery, changing the endpoint, etc. all reach the poller and
	// lifecycle handlers on the next tick / action.
	h.serviceMu.RLock()
	svc := h.service
	h.serviceMu.RUnlock()
	svc.Reconfigure(&newCfg)

	logging.Audit("Discovery config updated",
		"caller", auditCaller(r),
		"endpoint", newCfg.Endpoint,
		"strategy", newCfg.NetworkStrategy,
		"enabled", newCfg.Enabled)

	// Return the fresh status so the UI can update without a follow-up GET.
	sendJSON(w, http.StatusOK, svc.Status(r.Context()))
}

// mergeDockerConfigLocked decodes body onto a copy of the stored
// discovery.docker block and normalises and validates the result like
// config.Load. The caller holds configMu for writing.
func (h *DiscoveryHandler) mergeDockerConfigLocked(body []byte) (config.DiscoveryDockerConfig, error) {
	newCfg := h.config.Discovery.Docker
	// json.Decode reuses a slice's backing array: copy it so the decode and
	// the trim below never write into the stored config.
	newCfg.LifecycleAllowedGroups = append([]string(nil), newCfg.LifecycleAllowedGroups...)
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&newCfg); err != nil {
		return newCfg, errors.New(errInvalidJSON + err.Error())
	}
	// While MUXIMUX_DISCOVERY_AUTO_IMPORT is set the live mode is the
	// override; Save writes the file's own value through fileView.
	if h.config.IsOverridden(config.OverrideAutoImport) {
		newCfg.AutoImport = h.config.Discovery.Docker.AutoImport
	}
	if h.config.IsOverridden(config.OverrideRequireExplicitEnable) {
		newCfg.RequireExplicitEnable = h.config.Discovery.Docker.RequireExplicitEnable
	}
	for i := range newCfg.LifecycleAllowedGroups {
		newCfg.LifecycleAllowedGroups[i] = strings.TrimSpace(newCfg.LifecycleAllowedGroups[i])
	}
	// Apply the SAME defaults and normalisation as config.Load (strategy,
	// refresh interval, min role, placement, auto-import mode). The poller
	// shares this live *config.Config and treats only the literal "off" as
	// off, so a raw auto_import stored here would fall through Reconcile and
	// silently auto-import until the next restart re-normalized it.
	config.ApplyDiscoveryDockerDefaults(&newCfg)
	if err := validateDiscoveryDockerConfig(&newCfg); err != nil {
		return newCfg, err
	}
	// Validate the lifecycle fields with the SAME rules the load path
	// uses, so a PUT can't persist an unknown lifecycle_min_role (which
	// would fail OPEN -- HasMinRole against an unknown role is level 0,
	// i.e. every authenticated user passes) or an allowed_groups entry
	// that bricks the next restart's load-time validation.
	if err := config.ValidateDiscoveryLifecycle(&newCfg); err != nil {
		return newCfg, err
	}
	return newCfg, nil
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
	res := svc.Scan(ctx, dashboardDomain)
	// Containers that opted out (muximux.app.enabled=false) are counted in
	// OptedOut but not listed.
	kept := make([]discovery.Suggestion, 0, len(res.Suggestions))
	for i := range res.Suggestions {
		if sk := res.Suggestions[i].AutoImportSkip; sk != nil && sk.Code == discovery.SkipDisabled {
			continue
		}
		kept = append(kept, res.Suggestions[i])
	}
	res.Suggestions = kept
	h.configMu.RLock()
	annotateTracked(h.config, res.Suggestions)
	h.configMu.RUnlock()
	sendJSON(w, http.StatusOK, res)
}

// annotateTracked marks suggestions the config already tracks and flags
// names that collide with an untracked app. Matching is by key alone, like
// ImportDocker's dedupe; an entry on another endpoint is still tracked and
// carries that endpoint. The caller holds configMu.
func annotateTracked(cfg *config.Config, sugs []discovery.Suggestion) {
	current := cfg.Discovery.Docker.Endpoint
	otherEndpoint := func(e string) string {
		if e != "" && e != current {
			return e
		}
		return ""
	}
	refs := map[string]*discovery.TrackedRef{}
	// Lowest precedence first so later kinds overwrite: quarantined, site, app.
	// Quarantined keys are annotated even though ImportDocker's dedupe covers
	// only apps and sites: a quarantined entry should not be re-offered.
	for _, q := range cfg.Quarantined() {
		if q.Key != "" {
			refs[q.Key] = &discovery.TrackedRef{Kind: discovery.TrackedQuarantined, Name: q.Name, Endpoint: otherEndpoint(q.Endpoint)}
		}
	}
	for i := range cfg.Server.GatewaySites {
		s := &cfg.Server.GatewaySites[i]
		if s.DockerKey != "" {
			refs[s.DockerKey] = &discovery.TrackedRef{Kind: discovery.TrackedSite, Name: s.Domain, Endpoint: otherEndpoint(s.DockerEndpoint)}
		}
	}
	names := make(map[string]bool, len(cfg.Apps))
	for i := range cfg.Apps {
		a := &cfg.Apps[i]
		names[a.Name] = true
		if a.DockerKey != "" {
			refs[a.DockerKey] = &discovery.TrackedRef{Kind: discovery.TrackedApp, Name: a.Name, AutoImported: a.DockerAutoImported, Endpoint: otherEndpoint(a.DockerEndpoint)}
		}
	}
	for i := range sugs {
		if ref, ok := refs[sugs[i].Key]; ok {
			sugs[i].Tracked = ref
			continue
		}
		sugs[i].NameTaken = names[sugs[i].Name]
	}
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
// candidate config: endpoint scheme is unix://, tcp:// or npipe://
// (the Windows default config.Load fills in), strategy is
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
	if !strings.HasPrefix(c.Endpoint, "unix://") &&
		!strings.HasPrefix(c.Endpoint, "tcp://") &&
		!strings.HasPrefix(c.Endpoint, "npipe://") {
		return errBadDiscoveryEndpointScheme
	}
	switch c.NetworkStrategy {
	case "",
		config.StrategyContainerIP,
		config.StrategyContainerDNS,
		config.StrategyHostPort,
		config.StrategyHostDockerInternal:
		// "" is allowed because config.Load defaults it to
		// StrategyContainerIP.
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
	errBadDiscoveryEndpointScheme  = sentinelError("discovery.docker.endpoint must start with unix://, tcp:// or npipe://")
	errBadDiscoveryNetworkStrategy = sentinelError("discovery.docker.network_strategy must be container_ip, container_dns, host_port, or host_docker_internal")
	errBadDiscoveryRefreshInterval = sentinelError("discovery.docker.refresh_interval is not a valid duration (e.g. \"60s\")")
	errBadDiscoveryTLSPaths        = sentinelError("discovery.docker.tls.enabled requires ca_cert, client_cert and client_key paths")
)

// sentinelError builds a constant error value usable in respondError
// where the message is the user-facing string.
type sentinelError string

func (e sentinelError) Error() string { return string(e) }

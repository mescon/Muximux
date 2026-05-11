package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/proxy"
)

// gatewayProxy is the slice of proxy.Proxy the handler depends on.
// Defining it as an interface lets tests substitute a fake whose
// Reload returns a controlled error, so the divergence-and-rollback
// branches of applyAndPersist get exercised without spinning Caddy
// up. *proxy.Proxy satisfies the interface in production.
type gatewayProxy interface {
	IsRunning() bool
	SetGatewaySites(sites []proxy.GatewaySite)
	Reload() error
	CaddyfilePreview(sites []proxy.GatewaySite) string
}

// GatewayHandler serves the structured gateway-sites CRUD endpoints
// the Settings UI talks to. Mutating endpoints follow the same
// validate-then-reload-then-persist sequence used elsewhere in the
// project, with explicit rollback on each failure point so the
// running Caddy and the on-disk config never drift apart.
type GatewayHandler struct {
	config      *config.Config
	configPath  string
	configMu    *sync.RWMutex
	proxyServer gatewayProxy
}

// NewGatewayHandler wires the handler against the live config and the
// running proxy. proxyServer may be nil when Muximux booted without
// any TLS or gateway sites configured; in that case create/update
// operations succeed but include `restart_required: true` in the
// response so the UI can prompt the operator.
//
// The proxyServer parameter is typed as the production *proxy.Proxy
// (rather than the gatewayProxy interface) at the public boundary so
// callers cannot accidentally pass a bare interface; tests use the
// unexported newGatewayHandlerWithProxy helper to inject fakes.
func NewGatewayHandler(cfg *config.Config, configPath string, configMu *sync.RWMutex, proxyServer *proxy.Proxy) *GatewayHandler {
	var gp gatewayProxy
	if proxyServer != nil {
		gp = proxyServer
	}
	return &GatewayHandler{
		config:      cfg,
		configPath:  configPath,
		configMu:    configMu,
		proxyServer: gp,
	}
}

// newGatewayHandlerWithProxy is the unexported test seam for
// substituting a fake proxy. Pass nil to simulate the no-Caddy boot
// path; pass a real implementation to drive the reload paths.
func newGatewayHandlerWithProxy(cfg *config.Config, configPath string, configMu *sync.RWMutex, p gatewayProxy) *GatewayHandler {
	return &GatewayHandler{
		config:      cfg,
		configPath:  configPath,
		configMu:    configMu,
		proxyServer: p,
	}
}

// gatewayMutationResponse is the shape every mutating endpoint
// returns on success. `restart_required` is true when Muximux booted
// without Caddy running and a restart is needed for the new config to
// actually serve traffic; the UI surfaces a banner in that case.
//
// `Mismatch` is set when Caddy is serving a candidate config but the
// disk is at the prior config (a save-failed-then-rollback-Reload-
// failed scenario). It signals the UI to show a sticky banner asking
// the operator to restart Muximux. This is rare and indicates a real
// problem with the data directory.
type gatewayMutationResponse struct {
	Success         bool                 `json:"success"`
	Site            *config.GatewaySite  `json:"site,omitempty"`
	Sites           []config.GatewaySite `json:"sites,omitempty"`
	RestartRequired bool                 `json:"restart_required,omitempty"`
	Mismatch        bool                 `json:"mismatch,omitempty"`
}

// ListSites handles GET /api/gateway/sites.
func (h *GatewayHandler) ListSites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	h.configMu.RLock()
	out := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)
	h.configMu.RUnlock()
	sendJSON(w, http.StatusOK, out)
}

// ValidateSite handles POST /api/gateway/validate.
//
// Used by the UI to lint the form fields as the operator types: returns
// 200 with `{valid: bool, error: "..."}` regardless of validity so the
// frontend can render error text inline without treating it as an HTTP
// failure.
func (h *GatewayHandler) ValidateSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	var candidate config.GatewaySite
	if err := json.NewDecoder(r.Body).Decode(&candidate); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	// Snapshot the relevant slices under the read lock so the
	// validator never walks a slice the live config is concurrently
	// mutating. We need both Server (TLS-domain collision) and Apps
	// (AppName cross-reference) from the live config; copying both
	// slices into a local *Config keeps the validator side-effect
	// free without holding the lock across its run.
	h.configMu.RLock()
	cfgCopy := config.Config{
		Server: h.config.Server,
		Apps:   append([]config.AppConfig(nil), h.config.Apps...),
	}
	cfgCopy.Server.GatewaySites = append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)
	h.configMu.RUnlock()

	resp := map[string]interface{}{"valid": true}
	if err := config.ValidateGatewaySites([]config.GatewaySite{candidate}, &cfgCopy); err != nil {
		resp["valid"] = false
		resp["error"] = err.Error()
	}
	sendJSON(w, http.StatusOK, resp)
}

// CreateSite handles POST /api/gateway/sites.
func (h *GatewayHandler) CreateSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	var site config.GatewaySite
	if err := json.NewDecoder(r.Body).Decode(&site); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	// Reject create when a site with this domain already exists; the
	// operator should use PUT to update.
	for i := range h.config.Server.GatewaySites {
		if strings.EqualFold(h.config.Server.GatewaySites[i].Domain, site.Domain) {
			sendJSON(w, http.StatusConflict, map[string]interface{}{
				"success": false,
				"message": "a gateway site with that domain already exists; use PUT to update it",
			})
			return
		}
	}

	candidate := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)
	candidate = append(candidate, site)

	restartRequired, status, err := h.applyAndPersist(candidate)
	if err != nil {
		writeApplyError(w, r, status, err, "create", site.Domain)
		return
	}

	logging.From(r.Context()).Info("Gateway site created", "source", "audit", "domain", site.Domain)
	sendJSON(w, http.StatusOK, gatewayMutationResponse{
		Success:         true,
		Site:            &site,
		RestartRequired: restartRequired,
	})
}

// UpdateSite handles PUT /api/gateway/sites/{domain}. The path domain
// is the site to update; the request body's Domain takes effect (so
// renaming is supported), but only if the new domain doesn't collide
// with another existing site.
func (h *GatewayHandler) UpdateSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	pathDomain := strings.TrimPrefix(r.URL.Path, "/api/gateway/sites/")
	if pathDomain == "" {
		respondError(w, r, http.StatusBadRequest, "missing domain in URL")
		return
	}

	var site config.GatewaySite
	if err := json.NewDecoder(r.Body).Decode(&site); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}
	if site.Domain == "" {
		// Allow a body that omits Domain by treating the URL path as
		// the source of truth (the UI's read-only domain case).
		site.Domain = pathDomain
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	idx := -1
	for i := range h.config.Server.GatewaySites {
		if strings.EqualFold(h.config.Server.GatewaySites[i].Domain, pathDomain) {
			idx = i
			break
		}
	}
	if idx < 0 {
		respondError(w, r, http.StatusNotFound, "gateway site not found", "source", "gateway", "domain", pathDomain)
		return
	}

	// If the body renames the site, make sure the new name doesn't
	// collide with another existing entry.
	if !strings.EqualFold(site.Domain, pathDomain) {
		for i := range h.config.Server.GatewaySites {
			if i == idx {
				continue
			}
			if strings.EqualFold(h.config.Server.GatewaySites[i].Domain, site.Domain) {
				sendJSON(w, http.StatusConflict, map[string]interface{}{
					"success": false,
					"message": "renaming would collide with an existing gateway site",
				})
				return
			}
		}
	}

	candidate := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)
	candidate[idx] = site

	restartRequired, status, err := h.applyAndPersist(candidate)
	if err != nil {
		writeApplyError(w, r, status, err, "update", pathDomain)
		return
	}

	logging.From(r.Context()).Info("Gateway site updated", "source", "audit", "domain", site.Domain, "previous_domain", pathDomain)
	sendJSON(w, http.StatusOK, gatewayMutationResponse{
		Success:         true,
		Site:            &site,
		RestartRequired: restartRequired,
	})
}

// DeleteSite handles DELETE /api/gateway/sites/{domain}.
func (h *GatewayHandler) DeleteSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	pathDomain := strings.TrimPrefix(r.URL.Path, "/api/gateway/sites/")
	if pathDomain == "" {
		respondError(w, r, http.StatusBadRequest, "missing domain in URL")
		return
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	idx := -1
	for i := range h.config.Server.GatewaySites {
		if strings.EqualFold(h.config.Server.GatewaySites[i].Domain, pathDomain) {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Idempotent: a delete of a site that doesn't exist is a no-op
		// from the operator's point of view. Returning 404 here would
		// force the UI to special-case retries after a flaky network.
		// We still surface restart_required honestly so a UI loaded
		// against an unsaved-changes-but-Caddy-not-running instance
		// shows the same banner whether the delete hit a row or not.
		sendJSON(w, http.StatusOK, gatewayMutationResponse{
			Success:         true,
			RestartRequired: h.proxyServer == nil || !h.proxyServer.IsRunning(),
		})
		return
	}

	candidate := append([]config.GatewaySite(nil), h.config.Server.GatewaySites[:idx]...)
	candidate = append(candidate, h.config.Server.GatewaySites[idx+1:]...)

	restartRequired, status, err := h.applyAndPersist(candidate)
	if err != nil {
		writeApplyError(w, r, status, err, "delete", pathDomain)
		return
	}

	logging.From(r.Context()).Info("Gateway site deleted", "source", "audit", "domain", pathDomain)
	sendJSON(w, http.StatusOK, gatewayMutationResponse{
		Success:         true,
		RestartRequired: restartRequired,
	})
}

// writeApplyError formats the failure path of any mutating endpoint.
// 503 with `mismatch=true` is the divergence case (Caddy and disk are
// out of step); other statuses get a plain error response. We log
// failed-mutation attempts at the audit level so monitoring can pick
// them up alongside the success entries each handler already emits.
func writeApplyError(w http.ResponseWriter, r *http.Request, status int, err error, op, domain string) {
	logging.From(r.Context()).Warn("Gateway "+op+" failed", "source", "audit", "domain", domain, "status", status, "error", err)
	if status == http.StatusServiceUnavailable {
		// Divergence: emit the mismatch flag so the UI's sticky banner
		// can pin until the operator restarts. If the encode itself
		// fails (client disconnected mid-write) we want a log line so
		// the absence of a UI banner does not look like the divergence
		// "self-healed".
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusServiceUnavailable)
		if encErr := json.NewEncoder(w).Encode(gatewayMutationResponse{
			Success:  false,
			Mismatch: true,
		}); encErr != nil {
			logging.From(r.Context()).Warn("Failed to deliver gateway divergence response",
				"source", "audit",
				"error", encErr)
		}
		return
	}
	respondError(w, r, status, err.Error(), "source", "gateway", "domain", domain)
}

// applyAndPersist is the shared mutation engine for create / update /
// delete. It validates the candidate site list, dry-runs it through
// the Caddyfile generator + parser, reloads Caddy if running,
// persists to disk, and rolls back on any failure so the in-memory
// config, the on-disk config, and the running Caddy never drift
// apart.
//
// Caller must hold h.configMu's write lock. We hold the lock across
// the Caddy reload as well: that is necessary because the in-memory
// and on-disk views must remain consistent throughout, and it is
// safe today because Caddy's request handlers do not call back into
// any Muximux endpoint (which would deadlock). Be careful about
// adding such a callback in future work.
//
// Returns:
//   - restartRequired: true when no proxy is running and the
//     persisted change will not take effect until Muximux restarts.
//   - status: HTTP status code the caller should respond with on
//     error. 200 on success, 400 for validation errors (operator
//     typo), 500 for runtime errors (Caddy reload failed, disk write
//     failed but rollback succeeded), 503 for the divergence case
//     where Caddy is serving the new config but disk holds the old.
//   - err: nil on success.
func (h *GatewayHandler) applyAndPersist(candidate []config.GatewaySite) (restartRequired bool, status int, err error) {
	prior := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)

	// 1. Structural validation: catches missing fields, bad URLs,
	// invalid header names, dangling app_name references, etc.
	// Mirrors the YAML loader's checks so any error returned here
	// would have failed config.Load too.
	if err := config.ValidateGatewaySites(candidate, h.config); err != nil {
		return false, http.StatusBadRequest, err
	}

	// 2. Caddyfile-level validation: render the candidate Caddyfile
	// and run it through the same adapter `Reload` would. Catches
	// errors the structural validator cannot (e.g., a directive that
	// generates malformed Caddy syntax). Skipped if no proxy was set
	// up at startup, since there is nothing to render against.
	canReload := h.proxyServer != nil && h.proxyServer.IsRunning()
	if canReload {
		caddyfile := h.proxyServer.CaddyfilePreview(toProxyGatewaySites(candidate))
		if err := proxy.Validate(caddyfile); err != nil {
			return false, http.StatusBadRequest, err
		}
	}

	// 3. Apply in-memory.
	h.config.Server.GatewaySites = candidate

	// 4. Reload Caddy if it is running.
	//
	// caddy.Load is transactional for the *parse* step (Adapt failure
	// rolls back cleanly), but post-parse failures — listener
	// collisions, async cert provisioning errors, panics in module
	// Provision — can leave Caddy in a state where the previous
	// config is no longer fully active. The "previous config still
	// running" comment in proxy.Reload is best-effort, not a
	// guarantee.
	//
	// To make the rollback path truly safe, we do two things on
	// reload failure:
	//   a) Restore the prior site list in-memory and on the proxy
	//      snapshot (so any future read sees the right state).
	//   b) Call Reload again with the prior list to re-assert the
	//      previous Caddy config. If THAT also fails, we are in the
	//      divergence case — fall through to the catastrophic-503
	//      path the save-failure branch uses.
	if canReload {
		h.proxyServer.SetGatewaySites(toProxyGatewaySites(candidate))
		if reloadErr := h.proxyServer.Reload(); reloadErr != nil {
			h.config.Server.GatewaySites = prior
			h.proxyServer.SetGatewaySites(toProxyGatewaySites(prior))
			if reassertErr := h.proxyServer.Reload(); reassertErr != nil {
				logging.Error("Gateway divergence: candidate reload failed and re-assert reload also failed; running Caddy may not match config.yaml until restart",
					"source", "audit",
					"reload_error", reloadErr,
					"reassert_error", reassertErr)
				return false, http.StatusServiceUnavailable, fmt.Errorf("reload failed (%w) and re-assert reload also failed (%v); running Caddy may not match config.yaml - restart Muximux to recover", reloadErr, reassertErr)
			}
			logging.Error("Gateway reload failed; prior config re-asserted", "source", "gateway", "error", reloadErr)
			return false, http.StatusInternalServerError, reloadErr
		}
		logging.Info("Gateway candidate applied to Caddy",
			"source", "caddy",
			"sites", len(candidate))
	}

	restartRequired = !canReload

	// 5. Persist to disk. On disk-write failure, undo the Caddy
	// reload (if we did one) and the in-memory mutation so a restart
	// picks up the prior config rather than the half-applied one.
	if saveErr := h.config.Save(h.configPath); saveErr != nil {
		h.config.Server.GatewaySites = prior
		if canReload {
			h.proxyServer.SetGatewaySites(toProxyGatewaySites(prior))
			if rollbackErr := h.proxyServer.Reload(); rollbackErr != nil {
				// The catastrophic case: disk save failed AND we
				// could not rewind Caddy. Caddy is now serving the
				// candidate config while disk holds the prior one.
				// Log at audit-level Error and return 503 with a
				// dedicated marker so the UI shows a sticky banner
				// asking the operator to restart Muximux.
				logging.Error("Gateway divergence: save failed and rollback reload failed; running Caddy mismatches config.yaml until restart",
					"source", "audit",
					"save_error", saveErr,
					"rollback_error", rollbackErr)
				return false, http.StatusServiceUnavailable, fmt.Errorf("config save failed (%w) and rollback reload failed (%v); running Caddy currently mismatches config.yaml - restart Muximux to recover", saveErr, rollbackErr)
			}
			// Save failed but Caddy was rewound to prior state.
			// Still note the brief window where Caddy served the
			// candidate so monitoring can correlate the spike.
			logging.Warn("Gateway candidate briefly served before save failure; Caddy rewound to prior config",
				"source", "audit",
				"save_error", saveErr)
		}
		logging.Error("Gateway config save failed", "source", "gateway", "error", saveErr)
		return false, http.StatusInternalServerError, saveErr
	}

	return restartRequired, http.StatusOK, nil
}

// ConfigGatewaySitesToProxy is re-exported here for backwards
// compatibility with callers (server.setupCaddy, etc.) that imported
// it from this package. The actual implementation lives in the proxy
// package so the discovery poller can call it without creating a
// circular dependency (handlers -> discovery -> handlers).
func ConfigGatewaySitesToProxy(sites []config.GatewaySite) []proxy.GatewaySite {
	return proxy.ConfigGatewaySitesToProxy(sites)
}

// toProxyGatewaySites is kept as the unexported alias used by the
// gateway handler so the in-file call sites stay short.
func toProxyGatewaySites(sites []config.GatewaySite) []proxy.GatewaySite {
	return proxy.ConfigGatewaySitesToProxy(sites)
}

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/proxy"
)

// sameGatewaySlice returns true when a and b describe the same set
// of gateway sites in the same order with the same field values.
// Used to skip a needless Caddy reload when the import touched only
// app entries (proxy-mode and direct-mode without a gateway).
// reflect.DeepEqual handles GatewaySite's embedded map cleanly; the
// slice is small (one row per imported gateway) so the cost is fine.
func sameGatewaySlice(a, b []config.GatewaySite) bool {
	return reflect.DeepEqual(a, b)
}

// ImportRequest is the body of POST /api/discovery/docker/import.
// The frontend sends one item per checked row in the Discover modal.
type ImportRequest struct {
	Items []ImportItem `json:"items"`
}

// ImportItem describes one container the operator wants to import.
// Either App, Gateway, or both can be set; nil for either means
// "skip this side". Every committed item gets DockerKey /
// DockerEndpoint / DockerStrategy stamped on it from Item.Key etc.
type ImportItem struct {
	Key      string              `json:"key"`               // discovery key, e.g. "label:sonarr-prod"
	Strategy string              `json:"strategy"`          // EffectiveStrategy from the suggestion
	App      *ClientAppConfig    `json:"app,omitempty"`     // when set, create an App with these fields
	Gateway  *config.GatewaySite `json:"gateway,omitempty"` // when set, create a GatewaySite
	// Routing controls how the App's menu link is wired when App is
	// non-nil. Three modes:
	//   ""       - same as "direct"; default for backward compat
	//   "direct" - menu links to the discovered container URL; the
	//              poller refreshes App.URL every tick. App.proxy stays
	//              false. Suitable when the dashboard machine can
	//              reach the container's IP+port directly.
	//   "proxy"  - menu links to /proxy/<slug> (Muximux's built-in
	//              path-prefix reverse proxy). App.proxy = true; the
	//              container URL is the upstream the proxy hits. Same
	//              tracking semantics as direct (poller refreshes
	//              App.URL).
	//   "gateway" - menu links to https://<gateway.domain>. App.URL is
	//               STATIC at the gateway domain; the gateway site's
	//               BackendURL is what the poller refreshes (so the
	//               gateway is the docker-managed entry, not the App).
	//               Requires Gateway to be non-nil.
	Routing      string `json:"routing,omitempty"`
	SkipIfExists *bool  `json:"skip_if_exists,omitempty"` // wire-default true when nil
}

// ImportItemResult carries the per-item outcome. Status values:
//   - "created"                  - app + (optional) gateway site committed
//   - "skipped_exists"           - already tracked or duplicate name in
//     store; no-op (default for re-import)
//   - "validation_failed"        - this item failed validation; Error explains
//   - "name_collision_in_batch"  - this item's name duplicates an earlier
//     item in the same submit
//   - "aborted_by_batch_failure" - this item passed validation but the
//     batch was rolled back because another
//     item failed. Error names the failing
//     item's key.
type ImportItemResult struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	AppName string `json:"app_name,omitempty"`
	Domain  string `json:"domain,omitempty"`
}

// ImportResult is what the handler returns. Success is true iff
// every item committed; per-item Items are populated even on overall
// failure so the modal can mark each row distinctly.
type ImportResult struct {
	Success bool               `json:"success"`
	Error   string             `json:"error,omitempty"`
	Items   []ImportItemResult `json:"items"`
}

// ImportDocker handles POST /api/discovery/docker/import. Atomic:
// either every item commits or none does (rollback on validation /
// save failure). Per-item statuses tell the operator exactly which
// row to fix vs which were collateral.
func (h *DiscoveryHandler) ImportDocker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}
	if len(req.Items) == 0 {
		sendJSON(w, http.StatusOK, ImportResult{Success: true, Items: []ImportItemResult{}})
		return
	}

	// Acquire the live config + endpoint under the lock; we hold
	// configMu for the whole transaction so a concurrent SaveConfig
	// can't race a partially-applied batch.
	h.configMu.Lock()
	defer h.configMu.Unlock()

	currentEndpoint := h.config.Discovery.Docker.Endpoint
	priorApps := append([]config.AppConfig(nil), h.config.Apps...)
	priorSites := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)

	results := make([]ImportItemResult, len(req.Items))
	// Tracks names + domains we've already produced in this batch so
	// the dedup-in-batch + cross-reference checks fire deterministically.
	batchAppNames := map[string]int{}    // name -> index of first item that introduced it
	batchSiteDomains := map[string]int{} // domain -> index

	// Working copies. We append to these as items pass validation;
	// on per-item failure we throw both away and bail.
	apps := append([]config.AppConfig(nil), priorApps...)
	sites := append([]config.GatewaySite(nil), priorSites...)
	existingAppNames := map[string]bool{}
	for i := range priorApps {
		existingAppNames[priorApps[i].Name] = true
	}
	existingDomains := map[string]bool{}
	for i := range priorSites {
		existingDomains[strings.ToLower(priorSites[i].Domain)] = true
	}
	existingDockerKeys := map[string]bool{}
	for i := range priorApps {
		if priorApps[i].DockerKey != "" {
			existingDockerKeys[priorApps[i].DockerKey] = true
		}
	}
	for i := range priorSites {
		if priorSites[i].DockerKey != "" {
			existingDockerKeys[priorSites[i].DockerKey] = true
		}
	}

	failingIdx := -1
	failingKey := ""

	for i := range req.Items {
		item := req.Items[i]
		results[i] = ImportItemResult{Key: item.Key}

		if item.Key == "" {
			results[i].Status = "validation_failed"
			results[i].Error = "key is required"
			failingIdx = i
			failingKey = "(empty key)"
			break
		}

		// SkipIfExists default = true (per dev/docker-discovery-plan.md
		// "wire default" for the import contract).
		skipIfExists := true
		if item.SkipIfExists != nil {
			skipIfExists = *item.SkipIfExists
		}

		// Already tracked? Treat as no-op so a re-import is idempotent.
		if existingDockerKeys[item.Key] {
			if skipIfExists {
				results[i].Status = "skipped_exists"
				continue
			}
			results[i].Status = "validation_failed"
			results[i].Error = fmt.Sprintf("docker key %q is already tracked; set skip_if_exists=false only when you mean to overwrite", item.Key)
			failingIdx = i
			failingKey = item.Key
			break
		}

		if item.App == nil && item.Gateway == nil {
			results[i].Status = "validation_failed"
			results[i].Error = "must set at least one of app or gateway"
			failingIdx = i
			failingKey = item.Key
			break
		}

		// Validate routing up front so the operator gets an actionable
		// error before we touch anything else. Empty string normalises
		// to "direct" for backward compat with pre-Phase-G clients.
		// Unknown values fail the whole batch; using `if` (not switch)
		// so `break` exits the outer for loop on failure.
		routing := item.Routing
		if routing == "" {
			routing = "direct"
		}
		if routing != "direct" && routing != "proxy" && routing != "gateway" {
			results[i].Status = "validation_failed"
			results[i].Error = fmt.Sprintf("unknown routing %q (want direct|proxy|gateway)", routing)
			failingIdx = i
			failingKey = item.Key
			break
		}
		if routing == "gateway" && item.Gateway == nil {
			results[i].Status = "validation_failed"
			results[i].Error = "routing=gateway requires a gateway site to be created in the same item"
			failingIdx = i
			failingKey = item.Key
			break
		}

		// App side.
		var addedApp *config.AppConfig
		if item.App != nil {
			if strings.TrimSpace(item.App.Name) == "" {
				results[i].Status = "validation_failed"
				results[i].Error = "app.name is required"
				failingIdx = i
				failingKey = item.Key
				break
			}
			if strings.TrimSpace(item.App.URL) == "" {
				results[i].Status = "validation_failed"
				results[i].Error = "app.url is required"
				failingIdx = i
				failingKey = item.Key
				break
			}
			if existingAppNames[item.App.Name] {
				if skipIfExists {
					results[i].Status = "skipped_exists"
					results[i].AppName = item.App.Name
					// If a gateway site is also requested, still try
					// to create it but link to the existing app.
					if item.Gateway == nil {
						continue
					}
					// fall through to gateway creation
				} else {
					results[i].Status = "validation_failed"
					results[i].Error = fmt.Sprintf("app named %q already exists", item.App.Name)
					failingIdx = i
					failingKey = item.Key
					break
				}
			} else if firstIdx, dup := batchAppNames[item.App.Name]; dup {
				results[i].Status = "name_collision_in_batch"
				results[i].Error = fmt.Sprintf("app name %q already used by item %d in this batch", item.App.Name, firstIdx)
				failingIdx = i
				failingKey = item.Key
				break
			} else {
				newApp := clientAppToConfig(item.App)
				// Apply the routing decision. The discovered URL
				// (item.App.URL) lives in newApp.URL by default; for
				// gateway-routed apps we override it with the public
				// domain and detach the App from auto-management
				// (the gateway site becomes the docker-managed entry).
				switch routing {
				case "proxy":
					newApp.Proxy = true
					newApp.DockerKey = item.Key
					newApp.DockerEndpoint = currentEndpoint
					newApp.DockerStrategy = item.Strategy
				case "gateway":
					// Replace App.URL with the gateway domain so the
					// menu loads via the public hostname. https:// is
					// the default since Caddy handles cert issuance
					// for managed sites; an operator with TLS=none
					// can post-edit later.
					scheme := "https"
					if item.Gateway.TLS == "none" {
						scheme = "http"
					}
					newApp.URL = scheme + "://" + item.Gateway.Domain
					newApp.Proxy = false
					// App is NOT tracked - the gateway is. This
					// avoids a tug-of-war where the poller would
					// rewrite App.URL away from the operator-chosen
					// gateway domain on every tick.
					newApp.DockerKey = ""
					newApp.DockerEndpoint = ""
					newApp.DockerStrategy = ""
				default: // "direct"
					newApp.Proxy = false
					newApp.DockerKey = item.Key
					newApp.DockerEndpoint = currentEndpoint
					newApp.DockerStrategy = item.Strategy
				}
				apps = append(apps, newApp)
				addedApp = &apps[len(apps)-1]
				batchAppNames[newApp.Name] = i
				results[i].AppName = newApp.Name
			}
		}

		// Gateway side.
		if item.Gateway != nil {
			if strings.TrimSpace(item.Gateway.Domain) == "" {
				results[i].Status = "validation_failed"
				results[i].Error = "gateway.domain is required"
				failingIdx = i
				failingKey = item.Key
				break
			}
			if strings.TrimSpace(item.Gateway.BackendURL) == "" {
				results[i].Status = "validation_failed"
				results[i].Error = "gateway.backend_url is required"
				failingIdx = i
				failingKey = item.Key
				break
			}
			domLower := strings.ToLower(item.Gateway.Domain)
			if existingDomains[domLower] {
				if !skipIfExists {
					results[i].Status = "validation_failed"
					results[i].Error = fmt.Sprintf("gateway domain %q already exists", item.Gateway.Domain)
					failingIdx = i
					failingKey = item.Key
					break
				}
				// skipping
			} else if firstIdx, dup := batchSiteDomains[domLower]; dup {
				results[i].Status = "name_collision_in_batch"
				results[i].Error = fmt.Sprintf("gateway domain %q already used by item %d in this batch", item.Gateway.Domain, firstIdx)
				failingIdx = i
				failingKey = item.Key
				break
			} else {
				newSite := *item.Gateway
				newSite.DockerKey = item.Key
				newSite.DockerEndpoint = currentEndpoint
				newSite.DockerStrategy = item.Strategy
				if newSite.AppName == "" && addedApp != nil {
					newSite.AppName = addedApp.Name
				}
				sites = append(sites, newSite)
				batchSiteDomains[domLower] = i
				results[i].Domain = newSite.Domain
			}
		}

		// Successful row (anything created or skipped). If we
		// created at least one of {app, gateway}, mark "created";
		// pure-skip rows already have "skipped_exists" set.
		if results[i].Status == "" {
			results[i].Status = "created"
		}
	}

	// Per-item failure: roll the in-memory edits back, mark the
	// successful predecessors as aborted, and return the per-item
	// statuses untouched for the failing item.
	if failingIdx >= 0 {
		for i := 0; i < failingIdx; i++ {
			if results[i].Status == "created" {
				results[i].Status = "aborted_by_batch_failure"
				results[i].Error = fmt.Sprintf("rolled back because item %d (%s) failed validation", failingIdx, failingKey)
			}
		}
		for i := failingIdx + 1; i < len(results); i++ {
			results[i].Key = req.Items[i].Key
			results[i].Status = "aborted_by_batch_failure"
			results[i].Error = fmt.Sprintf("rolled back because item %d (%s) failed validation", failingIdx, failingKey)
		}
		sendJSON(w, http.StatusOK, ImportResult{Success: false, Items: results})
		return
	}

	// Cross-reference + structural validation against the candidate
	// shape. ValidateGatewaySites is a *Config method that walks
	// gateway sites and checks AppName references existing apps.
	candidate := *h.config // shallow copy
	candidate.Apps = apps
	candidate.Server.GatewaySites = sites
	if err := config.ValidateGatewaySites(sites, &candidate); err != nil {
		// Find the offending site to attribute the failure. Best
		// effort: scan the batch's site domains and pick the first
		// match in the error message; if not found, attribute to
		// the whole batch.
		failKey := ""
		for i, item := range req.Items {
			if item.Gateway != nil && strings.Contains(err.Error(), item.Gateway.Domain) {
				results[i].Status = "validation_failed"
				results[i].Error = err.Error()
				failKey = item.Key
				failingIdx = i
				break
			}
		}
		for i := range results {
			if i != failingIdx && results[i].Status == "created" {
				results[i].Status = "aborted_by_batch_failure"
				results[i].Error = fmt.Sprintf("rolled back because gateway-site validation failed (%s)", failKey)
			}
		}
		sendJSON(w, http.StatusOK, ImportResult{Success: false, Error: err.Error(), Items: results})
		return
	}

	// Commit in-memory first so any subsequent failure has a clean
	// rollback target.
	h.config.Apps = apps
	h.config.Server.GatewaySites = sites

	// Push gateway sites to Caddy so https://imported.example.com
	// actually serves something. Without this the import lands on
	// disk only and Caddy ignores the new site until restart - the
	// bug discovered during Phase G live testing. Skip when no proxy
	// was started (CLI-only / tests).
	gatewayChanged := !sameGatewaySlice(priorSites, sites)
	if gatewayChanged && h.proxyServer != nil && h.proxyServer.IsRunning() {
		newProxy := proxy.ConfigGatewaySitesToProxy(sites)
		priorProxy := proxy.ConfigGatewaySitesToProxy(priorSites)
		if err := h.proxyServer.ApplyGatewaySites(newProxy, priorProxy); err != nil {
			h.config.Apps = priorApps
			h.config.Server.GatewaySites = priorSites
			// ErrDiverged signals BOTH the candidate AND the internal
			// rollback Reload failed - Caddy's running state is
			// unknown. Surface that to the operator via the
			// divergence banner; the next clean refresh tick will
			// clear it.
			divergenceMsg := "rolled back: caddy reload rejected the candidate gateway sites"
			if errors.Is(err, proxy.ErrDiverged) {
				if svc := h.Service(); svc != nil {
					svc.RecordDivergence()
				}
				divergenceMsg = "diverged: caddy candidate AND rollback both failed - running gateway may not match config"
				logging.From(r.Context()).Error("Discovery import caddy diverged",
					"source", "audit",
					"divergence_detected", true,
					"error", err)
			} else {
				logging.From(r.Context()).Error("Discovery import caddy reload failed; in-memory rolled back",
					"source", "audit",
					"error", err)
			}
			for i := range results {
				if results[i].Status == "created" {
					results[i].Status = "aborted_by_batch_failure"
					results[i].Error = divergenceMsg
				}
			}
			sendJSON(w, http.StatusOK, ImportResult{Success: false, Error: err.Error(), Items: results})
			return
		}
	}

	if err := h.config.Save(h.configPath); err != nil {
		// Save failed AFTER a successful Caddy candidate apply.
		// Revert in-memory and ask Caddy to re-assert the prior
		// shape so config + Caddy + disk converge. Capture the
		// candidate shape before reverting so the re-assert call
		// can name what Caddy is currently running (passing
		// priorSites twice would be a no-op rollback).
		var candidateForCaddy []proxy.GatewaySite
		if gatewayChanged && h.proxyServer != nil && h.proxyServer.IsRunning() {
			candidateForCaddy = proxy.ConfigGatewaySitesToProxy(sites)
		}
		h.config.Apps = priorApps
		h.config.Server.GatewaySites = priorSites
		statusErr := "rolled back: failed to save config to disk"
		if gatewayChanged && h.proxyServer != nil && h.proxyServer.IsRunning() {
			reassertErr := h.proxyServer.ApplyGatewaySites(
				proxy.ConfigGatewaySitesToProxy(priorSites),
				candidateForCaddy,
			)
			if reassertErr != nil {
				if errors.Is(reassertErr, proxy.ErrDiverged) {
					if svc := h.Service(); svc != nil {
						svc.RecordDivergence()
					}
					statusErr = "diverged: save failed and caddy re-assert also failed - running gateway may not match config"
					logging.From(r.Context()).Error("Discovery import save failed AND Caddy re-assert diverged",
						"source", "audit",
						"divergence_detected", true,
						"save_error", err.Error(),
						"reassert_error", reassertErr.Error())
				} else {
					statusErr = "rolled back: save failed and caddy re-assert errored - check audit log"
					logging.From(r.Context()).Error("Discovery import save failed AND Caddy re-assert errored",
						"source", "audit",
						"save_error", err.Error(),
						"reassert_error", reassertErr.Error())
				}
				for i := range results {
					if results[i].Status == "created" {
						results[i].Status = "aborted_by_batch_failure"
						results[i].Error = statusErr
					}
				}
				sendJSON(w, http.StatusOK, ImportResult{Success: false, Error: err.Error(), Items: results})
				return
			}
		}
		for i := range results {
			if results[i].Status == "created" {
				results[i].Status = "aborted_by_batch_failure"
				results[i].Error = statusErr
			}
		}
		logging.From(r.Context()).Error("Discovery import save failed; in-memory rolled back",
			"source", "audit",
			"error", err)
		sendJSON(w, http.StatusOK, ImportResult{Success: false, Error: err.Error(), Items: results})
		return
	}

	// Notify the post-save hook so the reverse-proxy route table
	// picks up newly-imported App.Proxy=true entries. Without this,
	// a freshly imported Proxy-routed app shows up in the menu but
	// /proxy/<slug>/ returns 404 until the next restart or the next
	// SaveConfig call. server.go wires this to the same callback
	// APIHandler uses.
	h.notifyConfigSaved()

	// Audit log per committed entry. Single line per item so a search
	// for the docker_key gives full provenance.
	for i := range results {
		switch results[i].Status {
		case "created":
			logging.Audit("Docker discovery import",
				"key", results[i].Key,
				"app", results[i].AppName,
				"gateway", results[i].Domain)
		case "skipped_exists":
			logging.Audit("Docker discovery import skipped",
				"key", results[i].Key,
				"reason", "already exists")
		}
	}

	sendJSON(w, http.StatusOK, ImportResult{Success: true, Items: results})
}

package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
	"github.com/mescon/muximux/v3/internal/logging"
)

// Discovery-lifecycle endpoints (detach + re-link):
//
//   - GET    /api/discovery/docker/tracked          list tracked apps + sites
//   - DELETE /api/discovery/docker/track/{key}      detach all matching entries
//   - POST   /api/discovery/docker/relink/probe     probe a tracked key on the current endpoint
//   - POST   /api/discovery/docker/relink/confirm   apply a re-link decision
//
// All routes are admin-only; status/scan/import already enforce that
// at registration time and these are added with the same wrapper.

// TrackedEntry is one row in the Currently-tracked listing. Mirrors
// what the Discovery tab needs to render: identity, key, last seen,
// and the endpoint-mismatch flag that drives the Re-link button.
type TrackedEntry struct {
	Kind            EntryKind `json:"kind"`
	Name            string    `json:"name"`     // app name or gateway domain
	Key             string    `json:"key"`      // tracking key (e.g. "label:foo")
	Strategy        string    `json:"strategy"` // saved DockerStrategy
	Endpoint        string    `json:"endpoint"` // saved DockerEndpoint
	URL             string    `json:"url"`      // current URL (BackendURL for gateway)
	LastSeenAt      string    `json:"last_seen_at,omitempty"`
	EndpointMatches bool      `json:"endpoint_matches"` // false -> show Re-link
}

// EntryKind names the two kinds of tracked entries the Currently-
// tracked panel renders. The audit-log paths also stamp this string
// verbatim, so the wire form has to match across handler + log.
type EntryKind string

const (
	EntryKindApp     EntryKind = "app"
	EntryKindGateway EntryKind = "gateway"
)

// TrackedListResult is what GET /tracked returns.
type TrackedListResult struct {
	Entries         []TrackedEntry `json:"entries"`
	CurrentEndpoint string         `json:"current_endpoint"`
}

// ListTracked handles GET /api/discovery/docker/tracked. The response
// is admin-shape only (we leak app name + gateway domain + tracking
// key) so registration must wrap with requireAdmin.
func (h *DiscoveryHandler) ListTracked(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	h.configMu.RLock()
	currentEndpoint := h.config.Discovery.Docker.Endpoint
	apps := append([]config.AppConfig(nil), h.config.Apps...)
	sites := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)
	h.configMu.RUnlock()

	svc := h.Service()
	out := TrackedListResult{
		Entries:         []TrackedEntry{},
		CurrentEndpoint: currentEndpoint,
	}
	for i := range apps {
		a := &apps[i]
		if a.DockerKey == "" {
			continue
		}
		out.Entries = append(out.Entries, TrackedEntry{
			Kind:            EntryKindApp,
			Name:            a.Name,
			Key:             a.DockerKey,
			Strategy:        a.DockerStrategy,
			Endpoint:        a.DockerEndpoint,
			URL:             a.URL,
			LastSeenAt:      formatLastSeen(svc, a.DockerKey),
			EndpointMatches: a.DockerEndpoint == currentEndpoint,
		})
	}
	for i := range sites {
		s := &sites[i]
		if s.DockerKey == "" {
			continue
		}
		out.Entries = append(out.Entries, TrackedEntry{
			Kind:            EntryKindGateway,
			Name:            s.Domain,
			Key:             s.DockerKey,
			Strategy:        s.DockerStrategy,
			Endpoint:        s.DockerEndpoint,
			URL:             s.BackendURL,
			LastSeenAt:      formatLastSeen(svc, s.DockerKey),
			EndpointMatches: s.DockerEndpoint == currentEndpoint,
		})
	}
	sendJSON(w, http.StatusOK, out)
}

// formatLastSeen returns the RFC3339 timestamp of the most recent
// successful resolve for key, or "" when never seen. The poller calls
// RecordSeen on every successful tick; the result here doubles as a
// heartbeat for "is the daemon still showing this container".
func formatLastSeen(svc *discovery.Service, key string) string {
	if svc == nil {
		return ""
	}
	t := svc.LastSeen(key)
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// DetachTracked handles DELETE /api/discovery/docker/track/{key}.
// Detaches every app and gateway site whose DockerKey == key AND
// DockerEndpoint matches the current endpoint. Returns 404 when no
// entries match (idempotency for scripted callers — see plan v4
// "DELETE /track/{key} mutation spec").
func (h *DiscoveryHandler) DetachTracked(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	rawKey := strings.TrimPrefix(r.URL.Path, "/api/discovery/docker/track/")
	if rawKey == "" {
		respondError(w, r, http.StatusBadRequest, "tracking key required")
		return
	}
	// Reject paths with embedded slashes: the mux registers
	// "/api/discovery/docker/track/" as a prefix, so a URL like
	// /api/discovery/docker/track/relink/probe would hit this
	// handler with key="relink/probe" and quietly attempt a
	// detach against a key the operator never typed. Valid keys
	// are "label:foo", "name:bar", "id:hex" - none contain a /.
	if strings.Contains(rawKey, "/") {
		respondError(w, r, http.StatusBadRequest, "tracking key must not contain /")
		return
	}
	// URL-decode so a label key containing %-encoded characters
	// (uncommon but legal per Docker's label syntax) matches what
	// the operator-supplied DockerKey actually holds.
	key, err := url.PathUnescape(rawKey)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "tracking key url-decode failed: "+err.Error())
		return
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	currentEndpoint := h.config.Discovery.Docker.Endpoint
	priorApps := append([]config.AppConfig(nil), h.config.Apps...)
	priorSites := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)

	var affectedApps []string
	var affectedSites []string
	for i := range h.config.Apps {
		a := &h.config.Apps[i]
		if a.DockerKey == key && a.DockerEndpoint == currentEndpoint {
			a.DockerKey = ""
			a.DockerEndpoint = ""
			a.DockerStrategy = ""
			affectedApps = append(affectedApps, a.Name)
		}
	}
	for i := range h.config.Server.GatewaySites {
		s := &h.config.Server.GatewaySites[i]
		if s.DockerKey == key && s.DockerEndpoint == currentEndpoint {
			s.DockerKey = ""
			s.DockerEndpoint = ""
			s.DockerStrategy = ""
			affectedSites = append(affectedSites, s.Domain)
		}
	}
	if len(affectedApps)+len(affectedSites) == 0 {
		respondError(w, r, http.StatusNotFound, "no tracked entries match key")
		return
	}

	if err := h.config.Save(h.configPath); err != nil {
		// Roll back the in-memory clears so the next request sees
		// the same state Caddy / disk are still serving.
		h.config.Apps = priorApps
		h.config.Server.GatewaySites = priorSites
		logging.Error("Detach save failed; in-memory rolled back",
			"source", "audit",
			"key", key,
			"error", err)
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig)
		return
	}

	// Drop the LastSeenAt entry so the map stays bounded as operators
	// detach over time. Done BEFORE the audit so a scripted caller
	// observing the map sees the cleanup before the log line.
	if svc := h.Service(); svc != nil {
		svc.ForgetTrackedKey(key)
	}

	// Rebuild the reverse-proxy route table so a detached App.Proxy
	// entry stops routing through /proxy/<slug>/ (or, more commonly,
	// keeps routing - detach clears docker_key but leaves proxy/url
	// intact, so the route just becomes stable rather than auto-
	// refreshed).
	h.notifyConfigSaved()

	for _, name := range affectedApps {
		// logging.Audit already stamps source=audit; do not pass it
		// again or the slog handler emits a duplicate attribute.
		logging.Audit("Docker tracking detached",
			"kind", "app", "name", name,
			"previous_key", key, "previous_endpoint", currentEndpoint)
	}
	for _, domain := range affectedSites {
		logging.Audit("Docker tracking detached",
			"kind", "gateway", "domain", domain,
			"previous_key", key, "previous_endpoint", currentEndpoint)
	}
	w.WriteHeader(http.StatusNoContent)
}

// RelinkProbeRequest is the body of POST /relink/probe.
type RelinkProbeRequest struct {
	Key string `json:"key"`
}

// RelinkCandidate is one row in the picker shown when no container
// matches the tracked key on the current endpoint.
type RelinkCandidate struct {
	Key   string `json:"key"` // tracking-key for this candidate (label > name > id)
	Name  string `json:"name"`
	Image string `json:"image"`
}

// RelinkProbeResult is the body of POST /relink/probe's response.
// Daemon-unreachable / refused / TLS errors are surfaced via HTTP 502
// instead of a sidecar Error field on a 200; the frontend's normal
// catch path handles them.
type RelinkProbeResult struct {
	Found      bool              `json:"found"`
	Container  *RelinkCandidate  `json:"container,omitempty"`
	Candidates []RelinkCandidate `json:"candidates,omitempty"`
}

// RelinkProbe handles POST /api/discovery/docker/relink/probe. The
// caller asks "is the tracked key still resolvable on the current
// daemon?" and receives either a single match (frontend confirms +
// re-link) or a candidate list (frontend picker).
func (h *DiscoveryHandler) RelinkProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req RelinkProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}
	req.Key = strings.TrimSpace(req.Key)
	if req.Key == "" {
		respondError(w, r, http.StatusBadRequest, "key is required")
		return
	}
	// Tracking keys always carry a "<source>:<value>" prefix
	// (label / name / id). Reject a malformed key here so the
	// operator gets a 400 with a clear message instead of an
	// empty-candidate response that looks like a valid "no match"
	// outcome.
	if !strings.Contains(req.Key, ":") {
		respondError(w, r, http.StatusBadRequest, "key must be in source:value format (label, name, or id)")
		return
	}

	svc := h.Service()
	if svc == nil {
		respondError(w, r, http.StatusServiceUnavailable, "discovery service not initialised")
		return
	}
	containers, err := svc.ListLiveContainers(r.Context())
	if err != nil {
		// Daemon unreachable / refused / TLS handshake failed.
		// Return 502 (bad gateway upstream from us) so HTTP-aware
		// callers (scripts, fetch wrappers) treat this as a
		// genuine error rather than a successful probe with an
		// "error" sidecar field. The frontend's existing
		// ApiError-based catch in DiscoveryRelinkModal surfaces
		// the message in the same inline banner that the prior
		// shape used.
		respondError(w, r, http.StatusBadGateway, err.Error())
		return
	}

	matched := matchByKey(containers, req.Key)
	if matched != nil {
		cand := candidateFromContainer(matched)
		sendJSON(w, http.StatusOK, RelinkProbeResult{Found: true, Container: cand})
		return
	}

	// No match: build a sorted candidate list. Tracked-entry image
	// is not persisted, so we sort by name alphabetically. The
	// frontend renders image alongside name to help the operator
	// identify the right container.
	candidates := make([]RelinkCandidate, 0, len(containers))
	for i := range containers {
		candidates = append(candidates, *candidateFromContainer(&containers[i]))
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})
	sendJSON(w, http.StatusOK, RelinkProbeResult{Found: false, Candidates: candidates})
}

// matchByKey resolves a tracking key to a single ContainerSummary,
// or nil when none matches. Mirror of the poller's resolution
// priority: label > name > id-prefix.
func matchByKey(containers []discovery.ContainerSummary, key string) *discovery.ContainerSummary {
	source, value, ok := strings.Cut(key, ":")
	if !ok {
		return nil
	}
	for i := range containers {
		c := &containers[i]
		switch source {
		case "label":
			if c.Labels[discovery.LabelDiscoveryID] == value {
				return c
			}
		case "name":
			if c.PrimaryName() == value {
				return c
			}
		case "id":
			if c.ID == value || strings.HasPrefix(c.ID, value) {
				return c
			}
		}
	}
	return nil
}

// candidateFromContainer builds a RelinkCandidate. The candidate's
// tracking key follows the label > name > id priority the rest of
// discovery uses (KeyForContainer).
func candidateFromContainer(c *discovery.ContainerSummary) *RelinkCandidate {
	key, _ := discovery.KeyForContainer(c)
	return &RelinkCandidate{
		Key:   key,
		Name:  c.PrimaryName(),
		Image: c.Image,
	}
}

// RelinkConfirmRequest is the body of POST /relink/confirm. The
// caller sends both the existing tracked key (so we know which
// entries to move) and the new key + endpoint to point them at.
type RelinkConfirmRequest struct {
	OldKey   string `json:"old_key"`
	NewKey   string `json:"new_key"`
	Strategy string `json:"strategy,omitempty"` // optional; preserves existing if empty
}

// RelinkConfirmResult reports per-entry outcomes.
type RelinkConfirmResult struct {
	UpdatedApps  []string `json:"updated_apps"`
	UpdatedSites []string `json:"updated_sites"`
}

// RelinkConfirm handles POST /api/discovery/docker/relink/confirm. The
// transaction:
//
//  1. Snapshot prior apps + sites under configMu.Lock.
//  2. Move all DockerKey == old_key entries to new_key + new endpoint
//     (and new_strategy when provided).
//  3. If any gateway site moved, the BackendURL for those sites is
//     left as-is — the next refresh tick will rewrite it. Caddy is
//     not reloaded here because the Caddyfile output for the same
//     BackendURL doesn't change.
//  4. Save config; on failure roll back in-memory and report 500.
//  5. Audit log per affected entry.
//
// The handler does NOT validate that new_key is currently resolvable;
// the operator just confirmed via the picker UI which already showed
// the candidate from a probe response. A separate explicit re-probe
// would be needless round-trip.
func (h *DiscoveryHandler) RelinkConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req RelinkConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}
	req.OldKey = strings.TrimSpace(req.OldKey)
	req.NewKey = strings.TrimSpace(req.NewKey)
	if req.OldKey == "" || req.NewKey == "" {
		respondError(w, r, http.StatusBadRequest, "old_key and new_key are required")
		return
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	endpoint := h.config.Discovery.Docker.Endpoint
	priorApps := append([]config.AppConfig(nil), h.config.Apps...)
	priorSites := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)

	out := RelinkConfirmResult{UpdatedApps: []string{}, UpdatedSites: []string{}}
	for i := range h.config.Apps {
		a := &h.config.Apps[i]
		if a.DockerKey == req.OldKey {
			a.DockerKey = req.NewKey
			a.DockerEndpoint = endpoint
			if req.Strategy != "" {
				a.DockerStrategy = req.Strategy
			}
			out.UpdatedApps = append(out.UpdatedApps, a.Name)
		}
	}
	for i := range h.config.Server.GatewaySites {
		s := &h.config.Server.GatewaySites[i]
		if s.DockerKey == req.OldKey {
			s.DockerKey = req.NewKey
			s.DockerEndpoint = endpoint
			if req.Strategy != "" {
				s.DockerStrategy = req.Strategy
			}
			out.UpdatedSites = append(out.UpdatedSites, s.Domain)
		}
	}
	if len(out.UpdatedApps)+len(out.UpdatedSites) == 0 {
		respondError(w, r, http.StatusNotFound, "no tracked entries match old_key")
		return
	}

	if err := h.config.Save(h.configPath); err != nil {
		h.config.Apps = priorApps
		h.config.Server.GatewaySites = priorSites
		logging.Error("Re-link save failed; in-memory rolled back",
			"source", "audit",
			"old_key", req.OldKey,
			"new_key", req.NewKey,
			"error", err)
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig)
		return
	}

	// Drop LastSeenAt for the old key so a scripted caller observing
	// the map doesn't see ghost entries; the new key will be stamped
	// on the next successful tick.
	if svc := h.Service(); svc != nil {
		svc.ForgetTrackedKey(req.OldKey)
	}

	// Rebuild the reverse-proxy route table. Re-link may have moved
	// a docker_key onto a different App (or, more commonly, the same
	// App with a different docker_key); either way the URL fields
	// the route table reads from could have changed.
	h.notifyConfigSaved()

	for _, name := range out.UpdatedApps {
		logging.Audit("Docker tracking re-linked",
			"kind", "app", "name", name,
			"old_key", req.OldKey, "new_key", req.NewKey, "endpoint", endpoint)
	}
	for _, domain := range out.UpdatedSites {
		logging.Audit("Docker tracking re-linked",
			"kind", "gateway", "domain", domain,
			"old_key", req.OldKey, "new_key", req.NewKey, "endpoint", endpoint)
	}
	sendJSON(w, http.StatusOK, out)
}

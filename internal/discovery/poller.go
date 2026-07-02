package discovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
	"github.com/mescon/muximux/v3/internal/proxy"
)

// PollerDeps is the bundle of cross-package dependencies the refresh
// poller needs. All three are required; nil means the caller is in
// a state where the poller shouldn't run.
type PollerDeps struct {
	Service    *Service
	Config     *config.Config
	ConfigPath string
	ConfigMu   *sync.RWMutex
	Proxy      *proxy.Proxy // ApplyGatewaySites + ErrDiverged
	OnSave     func() error // optional: defaults to Config.Save(ConfigPath)
	// OnConfigSaved is invoked after every successful refresh-batch
	// commit so the reverse-proxy route table picks up new URLs for
	// App.Proxy=true entries. Wired by server.go to the same
	// rebuild closure APIHandler + DiscoveryHandler use. Optional
	// (no-op when nil) so unit tests can construct PollerDeps
	// without dragging in the route-table dependency.
	OnConfigSaved func()

	// BroadcastDockerStateChanged, when non-nil, is called once per
	// changed app at the end of each tick. Wired by server.go to the
	// websocket Hub, where the closure converts this DockerState into a
	// websocket.DockerStatePayload (the two are deliberately distinct
	// structs to keep the websocket package free of a discovery import).
	// Optional (no-op when nil) so unit tests don't need a Hub.
	BroadcastDockerStateChanged func(appName string, state DockerState)
}

// Poller refreshes URLs on tracked Apps + GatewaySites at a fixed
// interval. It NEVER adds new tracked entries; the operator does that
// via the Discover modal. The poller's only job is to keep the URLs
// of containers we're already tracking pointed at the current state.
//
// Lifecycle: created by Server.Start when discovery is enabled and
// reachable, stopped by Server.Stop via the Run context's cancel.
type Poller struct {
	deps  PollerDeps
	stop  context.CancelFunc
	doneM sync.Mutex
	done  chan struct{}
	// lastBadInterval dedupes the "invalid refresh_interval"
	// warning - we want to surface a config typo once when it
	// happens (or on the first tick after edit), not every tick.
	lastBadInterval string
	// syncAbsent tracks how many consecutive successful scans each
	// sync-mode removal candidate has been missing (removal
	// hysteresis). See gateSyncRemovals. Only touched from tick(),
	// which runs one at a time, so it needs no lock.
	syncAbsent map[string]int
	// daemonDown dedupes the "daemon unreachable" warning: we log once
	// when the container listing starts failing and once when it
	// recovers, instead of every tick for the duration of an outage.
	// Only touched from tick().
	daemonDown bool
}

// syncRemovalGraceTicks is how many consecutive successful scans a
// tracked container must be absent before sync mode removes its app and
// gateway site. This stops a single transient empty or partial scan (a
// mass container restart, or a network_filter that briefly matches
// nothing) from wiping every auto-imported entry, while still honoring
// the GitOps-mirror contract once the absence persists. At the default
// 60s refresh interval this is roughly a 2-minute grace.
const syncRemovalGraceTicks = 3

// gateSyncRemovals applies removal hysteresis to the sync-mode removal
// candidates. It advances a per-key absence counter and returns only the
// keys that have now been absent for syncRemovalGraceTicks consecutive
// scans. Candidates that reappear (or that this tick no longer proposes)
// drop out of the counter, so a container that returns within the grace
// window is never removed and re-added.
func (p *Poller) gateSyncRemovals(candidates []string) []string {
	if len(candidates) == 0 {
		p.syncAbsent = nil
		return nil
	}
	next := make(map[string]int, len(candidates))
	var ready []string
	for _, k := range candidates {
		n := p.syncAbsent[k] + 1
		if n >= syncRemovalGraceTicks {
			ready = append(ready, k)
			continue
		}
		next[k] = n
	}
	p.syncAbsent = next
	return ready
}

// NewPoller returns a configured Poller. It does NOT start the
// goroutine; call Run to do that.
func NewPoller(deps PollerDeps) *Poller {
	return &Poller{deps: deps}
}

// Run blocks until ctx is cancelled, ticking every refresh_interval
// (default 60s; bounded to [10s, 1h] to keep operators from
// accidentally polling once a millisecond or only every quarter).
//
// The first tick fires immediately so an operator restart picks up
// any URL drift right away. Subsequent ticks honor the configured
// interval.
func (p *Poller) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	// Publish stop + done under doneM: Stop() may be called (from a
	// concurrent goroutine) before or during Run's startup, and both
	// fields are read there. Guarding only done left a race on stop.
	p.doneM.Lock()
	p.stop = cancel
	p.done = make(chan struct{})
	p.doneM.Unlock()
	defer close(p.done)

	// Initial tick at startup. Then settle into the configured cadence.
	p.tick(ctx)

	t := time.NewTicker(p.interval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.tick(ctx)
			// Re-read interval on every tick: an operator may have
			// changed it via Settings → Discovery between ticks.
			t.Reset(p.interval())
		}
	}
}

// Stop signals Run to exit and waits up to 5 seconds for it. Safe to
// call multiple times.
func (p *Poller) Stop() {
	p.doneM.Lock()
	stop := p.stop
	d := p.done
	p.doneM.Unlock()
	if stop != nil {
		stop()
	}
	if d == nil {
		return
	}
	select {
	case <-d:
	case <-time.After(5 * time.Second):
		logging.Warn("Discovery poller did not stop within 5s; abandoning",
			"source", "discovery")
	}
}

// interval reads refresh_interval under the config lock and clamps
// it to [10s, 1h]. A zero / unparseable value falls back to 60s.
func (p *Poller) interval() time.Duration {
	const (
		minInterval     = 10 * time.Second
		maxInterval     = time.Hour
		defaultInterval = 60 * time.Second
	)
	p.deps.ConfigMu.RLock()
	raw := p.deps.Config.Discovery.Docker.RefreshInterval
	p.deps.ConfigMu.RUnlock()
	if raw == "" {
		return defaultInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		// Surface the fallback. The API endpoint's validator
		// catches bad durations on PUT, but a config.yaml edited
		// directly with a typo (e.g. "5minute") bypasses that and
		// would silently run at 60s. The Warn-once dedup is via
		// the lastBadInterval check so we don't spam every tick.
		if raw != p.lastBadInterval {
			p.lastBadInterval = raw
			logging.Warn("Refresh interval is invalid; falling back to 60s default",
				"source", "discovery",
				"value", raw,
				"parse_error", fmt.Sprintf("%v", err))
		}
		return defaultInterval
	}
	if d < minInterval {
		return minInterval
	}
	if d > maxInterval {
		return maxInterval
	}
	return d
}

// tick is one refresh cycle. Reads tracked entries under the config
// lock, resolves each against the daemon (no lock), then commits the
// resulting batch under the lock with rollback on failure.
func (p *Poller) tick(ctx context.Context) {
	// Snapshot enabled + endpoint under the read lock so a mid-tick
	// disable kills the tick cleanly without holding the lock during
	// the docker calls.
	p.deps.ConfigMu.RLock()
	enabled := p.deps.Config.Discovery.Docker.Enabled
	endpoint := p.deps.Config.Discovery.Docker.Endpoint
	hostIP := p.deps.Config.Discovery.Docker.HostIP
	// Normalize defensively: the config PUT path normalizes before storing,
	// but snapshotting through NormalizeAutoImport here guarantees the poller
	// fails closed to off even if some future path ever stores a raw value.
	autoImport := config.NormalizeAutoImport(p.deps.Config.Discovery.Docker.AutoImport)
	dashboardDomain := p.deps.Config.Server.TLS.Domain
	tracked := p.collectTracked()
	// Snapshot the current apps and gateway sites for the reconcile diff
	// while we hold the read lock. Sites are needed so a gateway-only
	// label change is detected. Only relevant when auto-import is active.
	var currentApps []config.AppConfig
	var currentSites []config.GatewaySite
	if autoImport != config.AutoImportOff {
		currentApps = append([]config.AppConfig(nil), p.deps.Config.Apps...)
		currentSites = append([]config.GatewaySite(nil), p.deps.Config.Server.GatewaySites...)
	}
	p.deps.ConfigMu.RUnlock()

	if !enabled {
		return
	}
	// Proceed when something is tracked OR auto-import is on. Auto-import
	// must run before anything is tracked (the first boot of a declared
	// container). Only bail when there is nothing to refresh AND no
	// auto-import to perform.
	if len(tracked.apps) == 0 && len(tracked.sites) == 0 && autoImport == config.AutoImportOff {
		return // nothing to do
	}

	// Keys of tracked gateway sites. A tracked app sharing a key with a
	// gateway site is gateway-routed: its URL is the static public domain,
	// not a container URL, so the refresh pass must NOT resolve/rewrite it
	// (doing so would clobber the domain and break routing). The sibling
	// site's BackendURL refresh below keeps routing pointed at the live
	// container.
	gatewaySiteKeys := make(map[string]bool, len(tracked.sites))
	for i := range tracked.sites {
		gatewaySiteKeys[tracked.sites[i].key] = true
	}

	svc := p.deps.Service
	if svc == nil || svc.client == nil {
		return
	}

	// Fetch the daemon's container list once for the entire tick.
	// Previously resolveURL called ListContainers itself, which
	// fanned out into N+M HTTP round-trips per tick (one per
	// tracked app and gateway site). The list is small and the
	// resolve loop is fast enough that a single snapshot is fine
	// for the full tick. A daemon listing failure aborts the tick
	// cleanly - we don't want to half-resolve and possibly mark
	// containers as "not found" because the daemon was unreachable.
	containers, err := svc.client.ListContainers(ctx, ListContainersOpts{All: false})
	if err != nil {
		if !p.daemonDown {
			p.daemonDown = true
			logging.Warn("Discovery refresh skipped: ListContainers failed (retrying each tick; suppressing repeats until recovery)",
				"source", "discovery", "error", err.Error())
		}
		return
	}
	if p.daemonDown {
		p.daemonDown = false
		logging.Info("Discovery daemon reachable again", "source", "discovery")
	}

	// Resolve each tracked entry's container -> new URL. Skips
	// entries whose DockerEndpoint differs from the live endpoint
	// (operator changed daemons; the re-link flow handles that
	// case interactively). Skips containers that disappeared with
	// a Warn; the next tick may find them again.
	batch := newRefreshBatch()
	for i := range tracked.apps {
		t := &tracked.apps[i]
		if t.endpoint != endpoint {
			continue
		}
		// Gateway-routed app: URL is the static gateway domain, not a
		// container URL. Skip resolution so the domain is preserved; the
		// sibling gateway site's BackendURL refresh (below) handles the
		// container IP change.
		if gatewaySiteKeys[t.key] {
			continue
		}
		newURL, err := p.resolveURLFrom(containers, t.key, t.strategy, hostIP)
		if errors.Is(err, ErrContainerNotFound) {
			logging.Warn("Tracked docker container not found",
				"source", "discovery", "kind", "app", "name", t.name, "key", t.key)
			continue
		}
		if err != nil {
			// Daemon disconnect, malformed key, no-port-on-container,
			// URL-builder failure - all silent before, all surfaced now.
			// Operator sees the cause without correlating stale URLs.
			logging.Warn("Tracked docker resolve failed",
				"source", "discovery", "kind", "app", "name", t.name, "key", t.key,
				"error", err.Error())
			continue
		}
		svc.RecordSeen(t.key)
		if newURL != t.currentURL {
			batch.appURLChanges[t.name] = newURL
		}
	}
	for i := range tracked.sites {
		t := &tracked.sites[i]
		if t.endpoint != endpoint {
			continue
		}
		newURL, err := p.resolveURLFrom(containers, t.key, t.strategy, hostIP)
		if errors.Is(err, ErrContainerNotFound) {
			logging.Warn("Tracked docker container not found",
				"source", "discovery", "kind", "gateway", "domain", t.domain, "key", t.key)
			continue
		}
		if err != nil {
			logging.Warn("Tracked docker resolve failed",
				"source", "discovery", "kind", "gateway", "domain", t.domain, "key", t.key,
				"error", err.Error())
			continue
		}
		svc.RecordSeen(t.key)
		if newURL != t.currentURL {
			batch.siteURLChanges[t.domain] = newURL
		}
	}

	// Auto-import reconcile. Scan the daemon for labeled containers, map
	// each Suggestion to its Desired config, diff against the current
	// apps AND gateway sites, and fold the resulting plan into the SAME
	// batch so the refresh and the auto-import commit atomically (one
	// SaveConfig, one rollback) in applyRefreshBatch below.
	//
	// Reconcile's Update fires when either an app field OR a label-derived
	// gateway-site field differs, so gateway-only labels (require_auth,
	// min_role, allowed_groups, streaming, strip_frame_blockers,
	// forwarded_headers, skip_tls_verify) propagate on the next tick even
	// when no app field changed. Dropping the gateway.domain label reverts
	// the app to its container URL and drops the orphaned site
	// (applyReconcile). currentSites is the snapshot Reconcile diffs the
	// desired sites against.
	//
	// Overlap note: for a non-gateway auto app whose container IP changed
	// with no label change, the URL-refresh pass above and Reconcile's
	// Update below may both target the same app in one batch. Harmless:
	// both resolve to the same final URL and commit in a single save, so
	// this is not a double-write bug.
	if autoImport != config.AutoImportOff {
		scan := svc.Scan(ctx, dashboardDomain)
		// A failed or blocked scan yields no suggestions. Treating that
		// empty result as the desired set would make sync mode conclude
		// every labeled container vanished and delete every auto-imported
		// app + gateway site (data loss on a transient daemon hiccup or a
		// self-detect failure). Skip reconcile until the scan succeeds
		// again; the URL-refresh batch built above still commits below.
		if scan.Error != "" || scan.ScanBlocked != "" {
			logging.Warn("Discovery auto-import skipped: scan did not complete",
				"source", "discovery", "error", scan.Error, "blocked", scan.ScanBlocked)
		} else {
			desired := make([]Desired, 0, len(scan.Suggestions))
			for i := range scan.Suggestions {
				desired = append(desired, BuildDesired(&scan.Suggestions[i], endpoint))
			}
			plan := Reconcile(autoImport, desired, currentApps, currentSites)
			for i := range plan.Add {
				batch.addApps = append(batch.addApps, plan.Add[i].App)
				if plan.Add[i].Site != nil {
					batch.addSites = append(batch.addSites, *plan.Add[i].Site)
				}
			}
			for i := range plan.Update {
				batch.updateApps = append(batch.updateApps, plan.Update[i].App)
				if plan.Update[i].Site != nil {
					batch.updateSites = append(batch.updateSites, *plan.Update[i].Site)
				}
			}
			batch.removeKeys = append(batch.removeKeys, p.gateSyncRemovals(plan.RemoveKeys)...)
		}
	}

	if batch.empty() {
		svc.RecordRefreshTickSuccess()
	} else {
		p.applyRefreshBatch(batch)
	}

	p.refreshDockerState(ctx, tracked)
}

// trackedAppEntry / trackedSiteEntry are the per-entry snapshot the
// resolver works against. Captured under the read lock so the
// resolver phase can run unlocked.
type trackedAppEntry struct {
	name       string
	key        string
	endpoint   string
	strategy   string
	currentURL string
}
type trackedSiteEntry struct {
	domain     string
	key        string
	endpoint   string
	strategy   string
	currentURL string
}

// trackedSet bundles both for one tick.
type trackedSet struct {
	apps  []trackedAppEntry
	sites []trackedSiteEntry
}

// collectTracked walks the live config and returns the subset that
// has DockerKey set. Caller holds ConfigMu (read or write).
func (p *Poller) collectTracked() trackedSet {
	out := trackedSet{}
	for i := range p.deps.Config.Apps {
		a := &p.deps.Config.Apps[i]
		if a.DockerKey == "" {
			continue
		}
		out.apps = append(out.apps, trackedAppEntry{
			name:       a.Name,
			key:        a.DockerKey,
			endpoint:   a.DockerEndpoint,
			strategy:   a.DockerStrategy,
			currentURL: a.URL,
		})
	}
	for i := range p.deps.Config.Server.GatewaySites {
		s := &p.deps.Config.Server.GatewaySites[i]
		if s.DockerKey == "" {
			continue
		}
		out.sites = append(out.sites, trackedSiteEntry{
			domain:     s.Domain,
			key:        s.DockerKey,
			endpoint:   s.DockerEndpoint,
			strategy:   s.DockerStrategy,
			currentURL: s.BackendURL,
		})
	}
	return out
}

// resolveURLFrom looks up a container by tracking key in a
// pre-fetched container list and rebuilds its URL with the recorded
// strategy. The caller is expected to fetch the list once per tick
// rather than once per tracked entry - that was the original shape
// and it produced N+1 Docker round-trips for N tracked items. Now
// it's one round-trip per tick.
//
// Returns ErrContainerNotFound when no container in the list
// matches the tracking key (deleted, renamed, or filtered out).
// Other errors are derivative - a malformed key, no port on the
// matched container, or a URL-builder failure.
func (p *Poller) resolveURLFrom(containers []ContainerSummary, key, strategy, hostIP string) (string, error) {
	tk, err := ParseTrackingKey(key)
	if err != nil {
		return "", err
	}
	matched := tk.FindContainer(containers)
	if matched == nil {
		return "", ErrContainerNotFound
	}

	// Rebuild URL. The catalog is consulted for the port + scheme;
	// labels override; we don't have a Suggestion struct in the
	// poller so we replicate that priority chain here.
	port := containerPortForRefresh(matched)
	if port == 0 {
		return "", errors.New("container has no port; cannot rebuild URL")
	}
	scheme := containerSchemeForRefresh(matched)
	return buildURLForSuggestion(strategy, matched, port, scheme, hostIP)
}

// containerPortForRefresh picks the port to use when rebuilding URLs.
// Priority: label > catalog > first exposed.
func containerPortForRefresh(c *ContainerSummary) int {
	labels := ParseAppLabels(c.Labels)
	if labels.Port != 0 {
		return labels.Port
	}
	if entry, ok := MatchImage(c.Image); ok && entry.Port != 0 && containerExposesPort(c, entry.Port) {
		return entry.Port
	}
	return pickFirstExposedPort(c)
}

// containerSchemeForRefresh returns "http" / "https" using the same
// label > catalog > default-http priority.
func containerSchemeForRefresh(c *ContainerSummary) string {
	labels := ParseAppLabels(c.Labels)
	if labels.Scheme != "" {
		return labels.Scheme
	}
	if entry, ok := MatchImage(c.Image); ok && entry.Scheme != "" {
		return entry.Scheme
	}
	return "http"
}

// refreshBatch accumulates URL changes for one tick, plus the
// auto-import reconcile output so both commit through the same atomic
// SaveConfig + rollback in applyRefreshBatch.
type refreshBatch struct {
	appURLChanges  map[string]string // app name -> new URL
	siteURLChanges map[string]string // gateway domain -> new BackendURL

	// Auto-import reconcile output, folded into the same transaction as
	// the URL refresh. addApps/updateApps carry their own URL (gateway
	// apps carry the static domain); updates are matched by DockerKey.
	// removeKeys drops apps AND any gateway site sharing that DockerKey.
	addApps     []config.AppConfig
	addSites    []config.GatewaySite
	updateApps  []config.AppConfig
	updateSites []config.GatewaySite
	removeKeys  []string
}

func newRefreshBatch() *refreshBatch {
	return &refreshBatch{
		appURLChanges:  map[string]string{},
		siteURLChanges: map[string]string{},
	}
}

func (b *refreshBatch) empty() bool {
	return len(b.appURLChanges) == 0 && len(b.siteURLChanges) == 0 &&
		len(b.addApps) == 0 && len(b.addSites) == 0 &&
		len(b.updateApps) == 0 && len(b.updateSites) == 0 &&
		len(b.removeKeys) == 0
}

// reconcileChangesApps reports whether the auto-import plan touches any
// app (added, updated, or removed). Used to fire the route-table
// rebuild hook the same way an app URL change does.
func (b *refreshBatch) reconcileChangesApps() bool {
	return len(b.addApps) > 0 || len(b.updateApps) > 0 || len(b.removeKeys) > 0
}

func (b *refreshBatch) touchesGateway() bool {
	return len(b.siteURLChanges) > 0
}

// applyRefreshBatch is the transactional write. See
// dev/docker-discovery-plan.md "applyRefreshBatch" for the full
// pseudocode. Acquires configMu.Lock for the whole transaction and
// rolls back on every failure mode.
func (p *Poller) applyRefreshBatch(batch *refreshBatch) {
	p.deps.ConfigMu.Lock()
	defer p.deps.ConfigMu.Unlock()

	priorApps := append([]config.AppConfig(nil), p.deps.Config.Apps...)
	priorSites := append([]config.GatewaySite(nil), p.deps.Config.Server.GatewaySites...)

	// Apply changes in memory. Mirror each URL write into
	// DockerManagedURL so Load() can detect operator hand-edits
	// at next startup: the invariant "URL == DockerManagedURL"
	// holds whenever this tracking entry is poller-managed.
	for i := range p.deps.Config.Apps {
		a := &p.deps.Config.Apps[i]
		if newURL, ok := batch.appURLChanges[a.Name]; ok {
			a.URL = newURL
			a.DockerManagedURL = newURL
		} else if a.DockerKey != "" && a.DockerManagedURL == "" {
			// Grandfather: tracked entry from a pre-3.1.0 build
			// has no managed-URL baseline yet. Record the current
			// URL as the baseline so the next operator edit is
			// detectable.
			a.DockerManagedURL = a.URL
		}
	}
	for i := range p.deps.Config.Server.GatewaySites {
		s := &p.deps.Config.Server.GatewaySites[i]
		if newURL, ok := batch.siteURLChanges[s.Domain]; ok {
			s.BackendURL = newURL
			s.DockerManagedURL = newURL
		} else if s.DockerKey != "" && s.DockerManagedURL == "" {
			s.DockerManagedURL = s.BackendURL
		}
	}

	// Fold the auto-import reconcile plan into the same in-memory
	// transaction. Returns whether it added/updated/removed any gateway
	// site so the Caddy reload below fires for auto-import too.
	reconcileTouchedGateway := p.applyReconcile(batch)
	gatewayTouched := batch.touchesGateway() || reconcileTouchedGateway

	// Caddy reload, batched once for the whole tick. Only fires when
	// the batch actually touches gateway sites - app-only changes
	// don't need Caddy at all.
	if gatewayTouched && p.deps.Proxy != nil {
		newProxySites := proxy.ConfigGatewaySitesToProxy(p.deps.Config.Server.GatewaySites)
		priorProxySites := proxy.ConfigGatewaySitesToProxy(priorSites)
		err := p.deps.Proxy.ApplyGatewaySites(newProxySites, priorProxySites)
		if err != nil {
			// On ErrDiverged: restore in-memory snapshot before
			// exiting so config + Caddy + disk converge on what was
			// running before the candidate. Never leave in-memory
			// drifting from disk: subsequent reads would lie.
			if errors.Is(err, proxy.ErrDiverged) {
				p.deps.Config.Apps = priorApps
				p.deps.Config.Server.GatewaySites = priorSites
				p.deps.Service.RecordDivergence()
				logging.Error("Docker refresh divergence; in-memory restored to prior shape",
					"source", "audit",
					"error", err)
				return
			}
			// Reload-rejected-but-rolled-back: in-memory + disk
			// match prior; Caddy is also serving prior. Restore
			// in-memory and skip the Save (no change to persist).
			p.deps.Config.Apps = priorApps
			p.deps.Config.Server.GatewaySites = priorSites
			logging.Warn("Caddy reload during refresh failed; rolled back",
				"source", "discovery",
				"error", err)
			return
		}
	}

	// Persist. On Save failure, rollback in-memory AND ask Caddy to
	// re-assert the prior shape so the running gateway converges
	// with what's on disk.
	if err := p.save(); err != nil {
		// Snapshot the candidate shape BEFORE reverting in-memory so
		// the Caddy re-assert call can name what Caddy is currently
		// running. After the next two lines, Config.Server.GatewaySites
		// == priorSites, so reading it for the second argument would
		// pass priorSites twice and rollback would be a no-op.
		var candidateProxy []proxy.GatewaySite
		if gatewayTouched && p.deps.Proxy != nil {
			candidateProxy = proxy.ConfigGatewaySitesToProxy(p.deps.Config.Server.GatewaySites)
		}
		p.deps.Config.Apps = priorApps
		p.deps.Config.Server.GatewaySites = priorSites
		if gatewayTouched && p.deps.Proxy != nil {
			reassertErr := p.deps.Proxy.ApplyGatewaySites(
				proxy.ConfigGatewaySitesToProxy(priorSites),
				candidateProxy,
			)
			if reassertErr != nil {
				// Re-assert reload itself failed. If it returned
				// ErrDiverged the previous successful candidate
				// reload AND the rollback reload both failed - we
				// genuinely don't know what Caddy is serving. Mark
				// divergence so the operator sees the banner; the
				// next clean tick clears it.
				if errors.Is(reassertErr, proxy.ErrDiverged) {
					p.deps.Service.RecordDivergence()
					logging.Error("Save failed during refresh tick AND Caddy re-assert diverged",
						"source", "audit",
						"divergence_detected", true,
						"save_error", err.Error(),
						"reassert_error", reassertErr.Error())
					return
				}
				logging.Error("Save failed during refresh tick AND Caddy re-assert errored",
					"source", "audit",
					"save_error", err.Error(),
					"reassert_error", reassertErr.Error())
				return
			}
		}
		logging.Error("Save failed during refresh tick; in-memory rolled back",
			"source", "audit",
			"error", err)
		return
	}

	// Successful tick.
	p.deps.Service.RecordRefreshTickSuccess()
	// Rebuild the reverse-proxy route table if any tracked app's URL
	// changed - App.Proxy=true entries route through /proxy/<slug>/
	// and the route table caches each route's upstream URL, so a
	// silent IP shift would otherwise leave the proxy hitting the
	// stale address. Skip when no apps changed (gateway-only batches
	// don't touch the route table).
	if (len(batch.appURLChanges) > 0 || batch.reconcileChangesApps()) && p.deps.OnConfigSaved != nil {
		p.deps.OnConfigSaved()
	}
	for name, url := range batch.appURLChanges {
		logging.Info("Docker app URL refreshed",
			"source", "discovery", "app", name, "new_url", url)
	}
	for domain, url := range batch.siteURLChanges {
		logging.Info("Docker gateway-site URL refreshed",
			"source", "discovery", "domain", domain, "new_backend_url", url)
	}
	for i := range batch.addApps {
		logging.Info("Docker container auto-imported",
			"source", "audit", "app", batch.addApps[i].Name, "key", batch.addApps[i].DockerKey)
	}
	for i := range batch.updateApps {
		logging.Info("Docker auto-imported app re-synced",
			"source", "audit", "app", batch.updateApps[i].Name, "key", batch.updateApps[i].DockerKey)
	}
	for _, k := range batch.removeKeys {
		logging.Info("Docker auto-imported entry removed (container vanished)",
			"source", "audit", "key", k)
	}
}

// applyReconcile folds the auto-import plan carried on the batch into
// the live config. The caller (applyRefreshBatch) already holds the
// write lock and has snapshotted priorApps/priorSites for rollback, so
// this mutates p.deps.Config in place. Returns true when it added,
// updated, or removed any gateway site, so the caller knows to reload
// Caddy.
func (p *Poller) applyReconcile(batch *refreshBatch) bool {
	cfg := p.deps.Config
	touchedGateway := false

	// Removals first: drop vanished auto-imported apps and any gateway
	// site sharing their DockerKey.
	if len(batch.removeKeys) > 0 {
		remove := make(map[string]bool, len(batch.removeKeys))
		for _, k := range batch.removeKeys {
			remove[k] = true
		}
		keptApps := cfg.Apps[:0]
		for i := range cfg.Apps {
			if cfg.Apps[i].DockerKey != "" && remove[cfg.Apps[i].DockerKey] {
				continue
			}
			keptApps = append(keptApps, cfg.Apps[i])
		}
		cfg.Apps = keptApps
		keptSites := cfg.Server.GatewaySites[:0]
		for i := range cfg.Server.GatewaySites {
			if cfg.Server.GatewaySites[i].DockerKey != "" && remove[cfg.Server.GatewaySites[i].DockerKey] {
				touchedGateway = true
				continue
			}
			keptSites = append(keptSites, cfg.Server.GatewaySites[i])
		}
		cfg.Server.GatewaySites = keptSites
	}

	// Updates: replace the existing app/site matched by DockerKey. A
	// site with no current match is inserted (a label that newly added a
	// gateway domain to an already-tracked app).
	for i := range batch.updateApps {
		na := batch.updateApps[i]
		for j := range cfg.Apps {
			if cfg.Apps[j].DockerKey == na.DockerKey {
				cfg.Apps[j] = na
				break
			}
		}
	}
	for i := range batch.updateSites {
		ns := batch.updateSites[i]
		found := false
		for j := range cfg.Server.GatewaySites {
			if cfg.Server.GatewaySites[j].DockerKey == ns.DockerKey {
				cfg.Server.GatewaySites[j] = ns
				found = true
				break
			}
		}
		if !found {
			cfg.Server.GatewaySites = append(cfg.Server.GatewaySites, ns)
		}
		touchedGateway = true
	}

	// Drop orphaned sites: an updated entry whose app was replaced but
	// that carries no desired site (its gateway domain label was removed)
	// leaves a stale site behind, since the update loop above only
	// replaces or inserts. Reconcile it away by DockerKey.
	//
	// This relies on an invariant: dropping the gateway.domain label
	// always changes App.URL (public domain -> container URL), so the
	// entry is always present in updateApps whenever its site must be
	// orphaned. sameManagedFields compares URL, so the Update is
	// guaranteed. If that ever stops holding, a dropped site could be
	// stranded here.
	if len(batch.updateApps) > 0 {
		updatedKeys := make(map[string]bool, len(batch.updateApps))
		for i := range batch.updateApps {
			if batch.updateApps[i].DockerKey != "" {
				updatedKeys[batch.updateApps[i].DockerKey] = true
			}
		}
		keptSiteKeys := make(map[string]bool, len(batch.updateSites))
		for i := range batch.updateSites {
			keptSiteKeys[batch.updateSites[i].DockerKey] = true
		}
		keptSites := cfg.Server.GatewaySites[:0]
		for i := range cfg.Server.GatewaySites {
			k := cfg.Server.GatewaySites[i].DockerKey
			if k != "" && updatedKeys[k] && !keptSiteKeys[k] {
				touchedGateway = true
				continue // gateway domain was dropped from this entry
			}
			keptSites = append(keptSites, cfg.Server.GatewaySites[i])
		}
		cfg.Server.GatewaySites = keptSites
	}

	// Additions.
	cfg.Apps = append(cfg.Apps, batch.addApps...)
	if len(batch.addSites) > 0 {
		cfg.Server.GatewaySites = append(cfg.Server.GatewaySites, batch.addSites...)
		touchedGateway = true
	}

	return touchedGateway
}

// save invokes the configured save function (typically Config.Save).
// Pulled out so tests can swap it for an in-memory failure injection.
func (p *Poller) save() error {
	if p.deps.OnSave != nil {
		return p.deps.OnSave()
	}
	return p.deps.Config.Save(p.deps.ConfigPath)
}

// stateInspector is the narrow surface buildDockerStateCache needs.
// Pulled out as a func type so tests can inject a stub without
// constructing a full Client.
type stateInspector func(ctx context.Context, id string) (DockerState, error)

// dockerStateDiff is one element of the change-list returned by
// diffDockerStates. The poller broadcasts one EventDockerStateChanged
// per entry.
type dockerStateDiff struct {
	Name  string
	State DockerState
}

// buildDockerStateCache walks the tracked-app entries and assembles
// the next-tick state cache. Apps without a resolved container ID
// are recorded as Status="missing" (the UI shows them as deleted
// rather than as the previous "still running" state). Transient
// inspect errors fall back to the previous tick's state so the UI
// doesn't flap during a daemon hiccup.
func buildDockerStateCache(
	ctx context.Context,
	tracked []trackedAppEntry,
	resolved map[string]string,
	inspect stateInspector,
	prev map[string]DockerState,
) map[string]DockerState {
	next := make(map[string]DockerState, len(tracked))
	for _, t := range tracked {
		id, ok := resolved[t.name]
		if !ok || id == "" {
			next[t.name] = DockerState{Status: StatusMissing}
			continue
		}
		st, err := inspect(ctx, id)
		if err != nil {
			// Keep previous state on transient inspect failure so the
			// UI doesn't blink "missing" every time the daemon stalls.
			if p, hadPrev := prev[t.name]; hadPrev {
				next[t.name] = p
				continue
			}
			next[t.name] = DockerState{Status: StatusMissing}
			continue
		}
		next[t.name] = st
	}
	return next
}

// diffDockerStates returns the entries whose Status or Health changed
// from prev to next. Apps newly added to the tracked set also count as
// a change.
func diffDockerStates(prev, next map[string]DockerState) []dockerStateDiff {
	var out []dockerStateDiff
	for name, st := range next {
		old, ok := prev[name]
		if !ok || old.Status != st.Status || old.Health != st.Health {
			out = append(out, dockerStateDiff{Name: name, State: st})
		}
	}
	return out
}

// refreshDockerState inspects each tracked app's container, updates the
// Service cache, diffs against the previous tick, and broadcasts the
// changes. Runs at the end of every tick after the URL refresh batch.
func (p *Poller) refreshDockerState(ctx context.Context, tracked trackedSet) {
	svc := p.deps.Service
	if svc == nil || svc.client == nil {
		return
	}

	// State resolution lists ALL containers, including stopped ones.
	// The URL-refresh scan above is running-only (a stopped container
	// has no IP to resolve), but a stopped *tracked* container must
	// still read as its real state ("exited") so the dashboard keeps
	// offering Start. Resolving it against the running-only list would
	// drop it to "missing" and the lifecycle actions would vanish.
	containers, err := svc.client.ListContainers(ctx, ListContainersOpts{All: true})
	if err != nil {
		// Transient daemon failure: leave the cache untouched rather
		// than blinking every tracked app to "missing" for one tick.
		logging.Warn("Docker state refresh skipped: ListContainers failed",
			"source", "discovery", "error", err.Error())
		return
	}

	// Resolve each tracked app's docker_key against the container list.
	resolved := make(map[string]string, len(tracked.apps))
	for i := range tracked.apps {
		t := &tracked.apps[i]
		tk, err := ParseTrackingKey(t.key)
		if err != nil {
			// Consistent with the URL-refresh pass: surface a malformed
			// tracking key instead of silently skipping it.
			logging.Warn("Docker state refresh: unparseable tracking key",
				"source", "discovery", "name", t.name, "key", t.key, "error", err.Error())
			continue
		}
		if m := tk.FindContainer(containers); m != nil {
			resolved[t.name] = m.ID
		}
	}

	prev := svc.DockerStateSnapshot()
	inspect := func(ctx context.Context, id string) (DockerState, error) {
		return svc.client.InspectContainerState(ctx, id)
	}
	next := buildDockerStateCache(ctx, tracked.apps, resolved, inspect, prev)
	svc.SetDockerStateCache(next)

	diffs := diffDockerStates(prev, next)
	if p.deps.BroadcastDockerStateChanged != nil {
		for i := range diffs {
			p.deps.BroadcastDockerStateChanged(diffs[i].Name, diffs[i].State)
		}
	}
}

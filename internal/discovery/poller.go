package discovery

import (
	"context"
	"errors"
	"strings"
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
	ctx, p.stop = context.WithCancel(ctx)
	p.doneM.Lock()
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
	if p.stop != nil {
		p.stop()
	}
	p.doneM.Lock()
	d := p.done
	p.doneM.Unlock()
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
	tracked := p.collectTracked()
	p.deps.ConfigMu.RUnlock()

	if !enabled {
		return
	}
	if len(tracked.apps) == 0 && len(tracked.sites) == 0 {
		return // nothing to do
	}

	svc := p.deps.Service
	if svc == nil || svc.client == nil {
		return
	}

	// Resolve each tracked entry's container -> new URL. Skips
	// entries whose DockerEndpoint differs from the live endpoint
	// (operator changed daemons; Phase F's re-link flow handles
	// that). Skips containers that disappeared with a Warn; the
	// next tick may find them again.
	batch := newRefreshBatch()
	for i := range tracked.apps {
		t := &tracked.apps[i]
		if t.endpoint != endpoint {
			continue
		}
		newURL, err := p.resolveURL(ctx, t.key, t.strategy, hostIP)
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
		newURL, err := p.resolveURL(ctx, t.key, t.strategy, hostIP)
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

	if batch.empty() {
		svc.RecordRefreshTickSuccess()
		return
	}
	p.applyRefreshBatch(batch)
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

// resolveURL looks up a container by tracking key and rebuilds its
// URL with the recorded strategy. Returns ErrContainerNotFound when
// the daemon no longer has a matching container (deleted or
// renamed). Other errors (timeout, daemon down) bubble up so the
// caller can skip the entry and try again next tick.
func (p *Poller) resolveURL(ctx context.Context, key, strategy, hostIP string) (string, error) {
	svc := p.deps.Service
	containers, err := svc.client.ListContainers(ctx, ListContainersOpts{All: false})
	if err != nil {
		return "", err
	}

	// key shape is "<source>:<value>"; resolve per source.
	source, value, ok := strings.Cut(key, ":")
	if !ok {
		return "", errors.New("malformed tracking key (no source prefix)")
	}

	var matched *ContainerSummary
	for i := range containers {
		c := &containers[i]
		switch source {
		case "label":
			if c.Labels[LabelDiscoveryID] == value {
				matched = c
			}
		case "name":
			if c.PrimaryName() == value {
				matched = c
			}
		case "id":
			if c.ID == value || strings.HasPrefix(c.ID, value) {
				matched = c
			}
		}
		if matched != nil {
			break
		}
	}
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

// refreshBatch accumulates URL changes for one tick.
type refreshBatch struct {
	appURLChanges  map[string]string // app name -> new URL
	siteURLChanges map[string]string // gateway domain -> new BackendURL
}

func newRefreshBatch() *refreshBatch {
	return &refreshBatch{
		appURLChanges:  map[string]string{},
		siteURLChanges: map[string]string{},
	}
}

func (b *refreshBatch) empty() bool {
	return len(b.appURLChanges) == 0 && len(b.siteURLChanges) == 0
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

	// Apply changes in memory.
	for i := range p.deps.Config.Apps {
		a := &p.deps.Config.Apps[i]
		if newURL, ok := batch.appURLChanges[a.Name]; ok {
			a.URL = newURL
		}
	}
	for i := range p.deps.Config.Server.GatewaySites {
		s := &p.deps.Config.Server.GatewaySites[i]
		if newURL, ok := batch.siteURLChanges[s.Domain]; ok {
			s.BackendURL = newURL
		}
	}

	// Caddy reload, batched once for the whole tick. Only fires when
	// the batch actually touches gateway sites - app-only changes
	// don't need Caddy at all.
	if batch.touchesGateway() && p.deps.Proxy != nil {
		newProxySites := proxy.ConfigGatewaySitesToProxy(p.deps.Config.Server.GatewaySites)
		priorProxySites := proxy.ConfigGatewaySitesToProxy(priorSites)
		err := p.deps.Proxy.ApplyGatewaySites(newProxySites, priorProxySites)
		if err != nil {
			// On ErrDiverged: restore in-memory snapshot before
			// exiting so config + Caddy + disk converge on what was
			// running before the candidate. (Plan v4 NEW-V3-2: do
			// not leave in-memory drifting from disk.)
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
		if batch.touchesGateway() && p.deps.Proxy != nil {
			candidateProxy = proxy.ConfigGatewaySitesToProxy(p.deps.Config.Server.GatewaySites)
		}
		p.deps.Config.Apps = priorApps
		p.deps.Config.Server.GatewaySites = priorSites
		if batch.touchesGateway() && p.deps.Proxy != nil {
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
	if len(batch.appURLChanges) > 0 && p.deps.OnConfigSaved != nil {
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
}

// save invokes the configured save function (typically Config.Save).
// Pulled out so tests can swap it for an in-memory failure injection.
func (p *Poller) save() error {
	if p.deps.OnSave != nil {
		return p.deps.OnSave()
	}
	return p.deps.Config.Save(p.deps.ConfigPath)
}

package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/proxy"
)

func TestPoller_Interval_DefaultsAndClamps(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"empty -> 60s default", "", 60 * time.Second},
		{"junk -> 60s default", "this-is-not-a-duration", 60 * time.Second},
		{"zero -> 60s default", "0s", 60 * time.Second},
		{"too short clamps to 10s floor", "1s", 10 * time.Second},
		{"too long clamps to 1h ceiling", "24h", time.Hour},
		{"in-range parses through", "5m", 5 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Discovery: config.DiscoveryConfig{
					Docker: config.DiscoveryDockerConfig{RefreshInterval: tc.raw},
				},
			}
			var mu sync.RWMutex
			p := &Poller{deps: PollerDeps{Config: cfg, ConfigMu: &mu}}
			if got := p.interval(); got != tc.want {
				t.Errorf("interval(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestPoller_CollectTracked_FiltersUnlinkedEntries(t *testing.T) {
	cfg := &config.Config{
		Apps: []config.AppConfig{
			{Name: "linked", URL: "http://10.0.0.1:8080", DockerKey: "label:foo", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_ip"},
			{Name: "manual", URL: "http://10.0.0.2:8080"}, // no DockerKey -> excluded
		},
		Server: config.ServerConfig{
			GatewaySites: []config.GatewaySite{
				{Domain: "linked.example.com", BackendURL: "http://10.0.0.3:8080", DockerKey: "name:bar", DockerEndpoint: "unix:///var/run/docker.sock", DockerStrategy: "container_dns"},
				{Domain: "manual.example.com", BackendURL: "http://10.0.0.4:8080"},
			},
		},
	}
	var mu sync.RWMutex
	p := &Poller{deps: PollerDeps{Config: cfg, ConfigMu: &mu}}
	got := p.collectTracked()

	if len(got.apps) != 1 || got.apps[0].name != "linked" {
		t.Errorf("apps tracked = %+v, want only the linked app", got.apps)
	}
	if got.apps[0].key != "label:foo" {
		t.Errorf("app key = %q, want label:foo", got.apps[0].key)
	}
	if len(got.sites) != 1 || got.sites[0].domain != "linked.example.com" {
		t.Errorf("sites tracked = %+v, want only the linked site", got.sites)
	}
	if got.sites[0].key != "name:bar" {
		t.Errorf("site key = %q, want name:bar", got.sites[0].key)
	}
}

func TestRefreshBatch_EmptyAndTouchesGateway(t *testing.T) {
	b := newRefreshBatch()
	if !b.empty() {
		t.Errorf("fresh batch should be empty")
	}
	if b.touchesGateway() {
		t.Errorf("fresh batch should not touch gateway")
	}

	b.appURLChanges["app"] = "http://1.2.3.4:8080"
	if b.empty() {
		t.Errorf("batch with app change should not be empty")
	}
	if b.touchesGateway() {
		t.Errorf("batch with only app changes should not touch gateway")
	}

	b.siteURLChanges["x.example.com"] = "http://1.2.3.5:8080"
	if !b.touchesGateway() {
		t.Errorf("batch with site change should touch gateway")
	}
}

// applyRefreshBatch exercises the transactional write the poller
// performs at the end of each tick. Tests below drive the proxy
// through its testReloadHook so we cover the rollback / divergence
// branches without booting Caddy. OnSave is also injected so we can
// trigger Save-failure rollback deterministically.

// fakeDaemonForPoller stands up a unix-socket Docker daemon that
// returns a fixed container set on /containers/json. The poller
// resolves tracked entries against this set.
func fakeDaemonForPoller(t *testing.T, summaries []ContainerSummary) (string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(summaries)
	})
	return fakeDockerOverUnix(t, mux)
}

func TestPoller_Tick_GatewaySiteRefreshFromDaemon(t *testing.T) {
	// End-to-end tick over the gateway side: tracked site -> daemon
	// -> ApplyGatewaySites with the test reload hook -> Save.
	containers := []ContainerSummary{
		{
			ID:     "g1",
			Names:  []string{"/grafana"},
			Image:  "grafana/grafana",
			Labels: map[string]string{LabelDiscoveryID: "grafana-stable", "muximux.app.port": "3000"},
			NetworkSettings: ContainerNetworks{
				Networks: map[string]ContainerNetwork{"obs": {IPAddress: "10.0.0.50"}},
			},
			Ports: []ContainerPort{{PrivatePort: 3000, Type: "tcp"}},
		},
	}
	socket, cleanup := fakeDaemonForPoller(t, containers)
	defer cleanup()

	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled: true, Endpoint: "unix://" + socket, NetworkStrategy: "container_ip",
	}
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *dockerCfg},
		Server: config.ServerConfig{
			GatewaySites: []config.GatewaySite{
				{
					Domain:         "grafana.example.com",
					BackendURL:     "http://10.0.0.1:3000", // stale; should rewrite to .50
					DockerKey:      "label:grafana-stable",
					DockerEndpoint: "unix://" + socket,
					DockerStrategy: "container_ip",
					TLS:            "auto",
				},
			},
		},
	}

	priorProxy := []proxy.GatewaySite{{Domain: "grafana.example.com", BackendURL: "http://10.0.0.1:3000", TLS: "auto"}}
	reloadCalls := 0
	pxy := newProxyForBatchTest(priorProxy, func() error { reloadCalls++; return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	if svc.client == nil {
		t.Fatalf("Service has no client")
	}
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { saveCalls++; return nil },
	})

	p.tick(context.Background())

	if got := cfg.Server.GatewaySites[0].BackendURL; got != "http://10.0.0.50:3000" {
		t.Errorf("site BackendURL = %q, want http://10.0.0.50:3000", got)
	}
	if reloadCalls != 1 {
		t.Errorf("Caddy reload called %d times, want 1 (single batched reload per tick)", reloadCalls)
	}
	if saveCalls != 1 {
		t.Errorf("Save called %d times, want 1", saveCalls)
	}
}

func TestPoller_Tick_AppRefreshFromDaemon(t *testing.T) {
	containers := []ContainerSummary{
		{
			ID:     "a1",
			Names:  []string{"/sonarr"},
			Image:  "linuxserver/sonarr",
			Labels: map[string]string{LabelDiscoveryID: "sonarr-stable"},
			NetworkSettings: ContainerNetworks{
				Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.42"}},
			},
			Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}},
		},
	}
	socket, cleanup := fakeDaemonForPoller(t, containers)
	defer cleanup()

	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "container_ip",
	}
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *dockerCfg},
		Apps: []config.AppConfig{
			{
				Name:           "sonarr",
				URL:            "http://10.0.0.1:8989", // stale: poller should rewrite to .42
				DockerKey:      "label:sonarr-stable",
				DockerEndpoint: "unix://" + socket,
				DockerStrategy: "container_ip",
			},
		},
	}
	var mu sync.RWMutex

	svc := NewService(dockerCfg)
	// Wait for the lazy client init - NewService is synchronous,
	// so client is set unless endpoint parsing failed.
	if svc.client == nil {
		t.Fatalf("Service has no client; setup failure")
	}

	saveCalled := 0
	p := NewPoller(PollerDeps{
		Config:   cfg,
		ConfigMu: &mu,
		Service:  svc,
		OnSave:   func() error { saveCalled++; return nil },
	})

	p.tick(context.Background())

	if got := cfg.Apps[0].URL; got != "http://10.0.0.42:8989" {
		t.Errorf("app URL after tick = %q, want http://10.0.0.42:8989", got)
	}
	if saveCalled != 1 {
		t.Errorf("Save called %d times, want 1", saveCalled)
	}
	if svc.LastSeen("label:sonarr-stable").IsZero() {
		t.Errorf("RecordSeen not called for tracked key")
	}
}

func TestPoller_Tick_SkipsEntriesPointingAtDifferentEndpoint(t *testing.T) {
	// An app's DockerEndpoint differs from the live config endpoint
	// (operator changed daemons). The poller must skip those
	// entries without trying to resolve them, so re-link flows can
	// keep them in a "stale but visible" state for Phase F UX.
	socket, cleanup := fakeDaemonForPoller(t, []ContainerSummary{})
	defer cleanup()

	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled: true, Endpoint: "unix://" + socket,
	}
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *dockerCfg},
		Apps: []config.AppConfig{
			{
				Name: "stale-link", URL: "http://10.0.0.1:8080",
				DockerKey:      "label:stale",
				DockerEndpoint: "tcp://192.168.1.99:2375", // different daemon
				DockerStrategy: "container_ip",
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalled := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalled++; return nil },
	})
	p.tick(context.Background())
	if saveCalled != 0 {
		t.Errorf("Save should not fire when all tracked entries point elsewhere; got %d", saveCalled)
	}
}

func TestPoller_Tick_DisabledIsNoop(t *testing.T) {
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: config.DiscoveryDockerConfig{Enabled: false}},
		Apps: []config.AppConfig{
			{Name: "x", URL: "http://10.0.0.1:8080", DockerKey: "label:x"},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})
	saveCalled := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalled++; return nil },
	})
	p.tick(context.Background())
	if saveCalled != 0 {
		t.Errorf("Save should not be called when disabled; got %d", saveCalled)
	}
}

func TestPoller_Tick_ContainerDisappeared_NoOp(t *testing.T) {
	// Tracked container references a key that no live container
	// matches. The poller should log a warning and skip the entry,
	// not blow up or write anything.
	socket, cleanup := fakeDaemonForPoller(t, []ContainerSummary{})
	defer cleanup()

	dockerCfg := &config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket}
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *dockerCfg},
		Apps: []config.AppConfig{
			{Name: "gone", URL: "http://10.0.0.1:8080", DockerKey: "label:vanished",
				DockerEndpoint: "unix://" + socket, DockerStrategy: "container_ip"},
		},
	}
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalled := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalled++; return nil },
	})
	p.tick(context.Background())
	if saveCalled != 0 {
		t.Errorf("Save should not be called when no changes apply; got %d", saveCalled)
	}
	// A clean tick with nothing to apply is still "successful" -
	// it's a recovery signal for divergence state. Only daemon
	// failure or rollback paths skip the success stamp.
	if svc.lastRefreshSuccessAt.IsZero() {
		t.Errorf("expected RecordRefreshTickSuccess to stamp on a clean no-change tick")
	}
}

func TestTitleizeName_EdgeCases(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"sonarr", "Sonarr"},
		{"Sonarr", "Sonarr"},   // already capitalised
		{"123name", "123name"}, // leading digit untouched
	}
	for _, c := range cases {
		if got := titleizeName(c.in); got != c.want {
			t.Errorf("titleizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitiseSubdomain_EdgeCases(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sonarr", "sonarr"},
		{"Sonarr", "sonarr"},
		{"my_app.v2", "my-app-v2"},
		{"---only-dashes---", "only-dashes"},
		{"!@#$%", ""}, // all stripped
	}
	for _, c := range cases {
		if got := sanitiseSubdomain(c.in); got != c.want {
			t.Errorf("sanitiseSubdomain(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNewPoller_StoresDeps(t *testing.T) {
	cfg := &config.Config{}
	var mu sync.RWMutex
	deps := PollerDeps{Config: cfg, ConfigMu: &mu}
	p := NewPoller(deps)
	if p == nil {
		t.Fatal("NewPoller returned nil")
	}
	if p.deps.Config != cfg {
		t.Errorf("deps.Config not stored")
	}
}

func TestPoller_Stop_BeforeRun_IsSafe(t *testing.T) {
	// Stop must be idempotent and safe to call before Run() ever
	// fires - the server shutdown path could otherwise panic if
	// Start failed before the poller goroutine got going.
	p := NewPoller(PollerDeps{})
	p.Stop()
	p.Stop()
}

func TestPoller_RunWithNoTracked_TicksAndExits(t *testing.T) {
	// Driven with an empty tracked set, the Run loop should:
	//  - perform the immediate-tick at startup (no-op)
	//  - sit on its ticker
	//  - exit cleanly when the context cancels
	//
	// We use a tight refresh_interval and a short cancel deadline
	// to keep the test fast. No daemon calls happen because there
	// are no tracked entries.
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{
			Docker: config.DiscoveryDockerConfig{Enabled: true, RefreshInterval: "10s"},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: svc})

	ctx, cancel := contextWithDeadline(20 * time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not exit within 500ms after context cancel")
	}
}

// contextWithDeadline returns a context that auto-cancels after d.
// Lifted out of the test so the goroutine in
// TestPoller_RunWithNoTracked stays readable.
func contextWithDeadline(d time.Duration) (*testCtx, func()) {
	c := &testCtx{done: make(chan struct{})}
	timer := time.AfterFunc(d, func() {
		select {
		case <-c.done:
		default:
			close(c.done)
		}
	})
	cancel := func() {
		timer.Stop()
		select {
		case <-c.done:
		default:
			close(c.done)
		}
	}
	return c, cancel
}

type testCtx struct {
	done chan struct{}
}

func (*testCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *testCtx) Done() <-chan struct{}     { return c.done }
func (c *testCtx) Err() error {
	select {
	case <-c.done:
		return errCanceled
	default:
		return nil
	}
}
func (*testCtx) Value(any) any { return nil }

var errCanceled = errors.New("canceled")

func TestApplyRefreshBatch_AppOnlyChange_NoCaddyReload(t *testing.T) {
	cfg := &config.Config{
		Apps: []config.AppConfig{
			{Name: "alpha", URL: "http://10.0.0.1:8080", DockerKey: "label:alpha"},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	saveCalled := 0
	p := &Poller{deps: PollerDeps{
		Config:   cfg,
		ConfigMu: &mu,
		Service:  svc,
		Proxy:    nil,
		OnSave:   func() error { saveCalled++; return nil },
	}}
	batch := newRefreshBatch()
	batch.appURLChanges["alpha"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)

	if cfg.Apps[0].URL != "http://10.0.0.99:8080" {
		t.Errorf("app URL after apply = %q, want updated", cfg.Apps[0].URL)
	}
	if saveCalled != 1 {
		t.Errorf("Save called %d times, want 1", saveCalled)
	}
	if svc.lastRefreshSuccessAt.IsZero() {
		t.Errorf("RecordRefreshTickSuccess not called on success")
	}
}

func TestApplyRefreshBatch_SaveFailureRollsBackInMemory(t *testing.T) {
	cfg := &config.Config{
		Apps: []config.AppConfig{
			{Name: "alpha", URL: "http://10.0.0.1:8080", DockerKey: "label:alpha"},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	saveErr := &saveFailureError{}
	p := &Poller{deps: PollerDeps{
		Config:   cfg,
		ConfigMu: &mu,
		Service:  svc,
		Proxy:    nil,
		OnSave:   func() error { return saveErr },
	}}
	batch := newRefreshBatch()
	batch.appURLChanges["alpha"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)

	if cfg.Apps[0].URL != "http://10.0.0.1:8080" {
		t.Errorf("app URL after Save failure = %q, want rollback to original", cfg.Apps[0].URL)
	}
	if !svc.lastRefreshSuccessAt.IsZero() {
		t.Errorf("RecordRefreshTickSuccess called despite Save failure")
	}
}

type saveFailureError struct{}

func (saveFailureError) Error() string { return "synthetic save failure" }

// newProxyForBatchTest spins up a proxy.Proxy with the gateway side
// pre-seeded so applyRefreshBatch's ApplyGatewaySites call has the
// "prior" shape it needs to roll back to. The reload hook is
// driven by the test.
func newProxyForBatchTest(prior []proxy.GatewaySite, reload func() error) *proxy.Proxy {
	p := proxy.New(&proxy.Config{ListenAddr: ":8080", InternalAddr: "127.0.0.1:18080"})
	p.SetGatewaySites(prior)
	p.SetTestReloadHook(reload)
	return p
}

func TestApplyRefreshBatch_GatewayChange_TriggersReload(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GatewaySites: []config.GatewaySite{
				{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", DockerKey: "label:x", TLS: "auto"},
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	reloadCalls := 0
	priorProxy := []proxy.GatewaySite{{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	pxy := newProxyForBatchTest(priorProxy, func() error { reloadCalls++; return nil })

	p := &Poller{deps: PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return nil },
	}}
	batch := newRefreshBatch()
	batch.siteURLChanges["x.example.com"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)

	if reloadCalls != 1 {
		t.Errorf("Caddy reload called %d times, want exactly 1", reloadCalls)
	}
	if cfg.Server.GatewaySites[0].BackendURL != "http://10.0.0.99:8080" {
		t.Errorf("site BackendURL = %q, want updated", cfg.Server.GatewaySites[0].BackendURL)
	}
	if svc.lastRefreshSuccessAt.IsZero() {
		t.Errorf("RecordRefreshTickSuccess not called on success")
	}
}

func TestApplyRefreshBatch_DivergedReload_RecordsDivergenceAndRollsBack(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GatewaySites: []config.GatewaySite{
				{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", DockerKey: "label:x", TLS: "auto"},
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	priorProxy := []proxy.GatewaySite{{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	calls := 0
	pxy := newProxyForBatchTest(priorProxy, func() error {
		calls++
		// Both candidate and rollback fail -> ApplyGatewaySites
		// returns ErrDiverged.
		return errors.New("synthetic reload failure")
	})

	p := &Poller{deps: PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { t.Fatalf("Save should not be called when reload fails"); return nil },
	}}
	batch := newRefreshBatch()
	batch.siteURLChanges["x.example.com"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)

	if cfg.Server.GatewaySites[0].BackendURL != "http://10.0.0.1:8080" {
		t.Errorf("site BackendURL = %q, want rollback to original after divergence", cfg.Server.GatewaySites[0].BackendURL)
	}
	if svc.divergences != 1 {
		t.Errorf("divergences counter = %d, want 1", svc.divergences)
	}
	if !svc.lastRefreshSuccessAt.IsZero() {
		t.Errorf("RecordRefreshTickSuccess incorrectly called on divergence")
	}
	if calls != 2 {
		t.Errorf("reload hook called %d times, want 2 (candidate + rollback)", calls)
	}
}

func TestContainerPortForRefresh_NoPortReturnsZero(t *testing.T) {
	// Container with no labels, no catalog match, no exposed ports
	// -> returns 0 so resolveURL can produce a clean error rather
	// than building a URL with port=0.
	c := &ContainerSummary{Image: "obscure/image:latest"}
	if got := containerPortForRefresh(c); got != 0 {
		t.Errorf("containerPortForRefresh on empty container = %d, want 0", got)
	}
}

func TestContainerPortForRefresh_CatalogPortMustBeExposed(t *testing.T) {
	// Catalog declares port X but the container does NOT expose
	// it -> fall through to pickFirstExposedPort.
	c := &ContainerSummary{
		Image: "linuxserver/sonarr:latest",
		// Sonarr's catalog port is 8989, but this container only
		// exposes 9090 - so containerExposesPort returns false
		// and we should fall through.
		Ports: []ContainerPort{{PrivatePort: 9090, Type: "tcp"}},
	}
	if got := containerPortForRefresh(c); got != 9090 {
		t.Errorf("containerPortForRefresh = %d, want 9090 (catalog skipped, fell through)", got)
	}
}

func TestPoller_ContainerPortAndScheme_Refresh(t *testing.T) {
	c := &ContainerSummary{
		Image:  "homarr/homarr:latest",
		Labels: map[string]string{"muximux.app.port": "7575", "muximux.app.scheme": "https"},
		Ports: []ContainerPort{
			{PrivatePort: 7575, Type: "tcp"},
		},
	}
	if got := containerPortForRefresh(c); got != 7575 {
		t.Errorf("containerPortForRefresh = %d, want 7575 (label override)", got)
	}
	if got := containerSchemeForRefresh(c); got != "https" {
		t.Errorf("containerSchemeForRefresh = %q, want https (label override)", got)
	}

	c2 := &ContainerSummary{Image: "linuxserver/sonarr:latest", Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}}}
	if got := containerSchemeForRefresh(c2); got != "http" {
		t.Errorf("containerSchemeForRefresh fallback = %q, want http", got)
	}
}

// TestApplyRefreshBatch_SaveFailureWithGateway_ReassertSucceeds covers
// the path where Caddy initially applies the candidate, the in-memory
// + disk save then fails, and the re-assert Reload that walks Caddy
// back to prior shape succeeds. The fix in commit "review-fix 1"
// makes this branch capture the candidate shape BEFORE reverting
// in-memory so the second ApplyGatewaySites argument names the
// shape Caddy was running (not priorSites twice). The reload hook
// snapshots p.GatewaySites() at each call so the test can verify
// the second call really saw the prior shape.
func TestApplyRefreshBatch_SaveFailureWithGateway_ReassertSucceeds(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GatewaySites: []config.GatewaySite{
				{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", DockerKey: "label:x", TLS: "auto"},
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	priorProxy := []proxy.GatewaySite{{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	var reloadSnapshots [][]proxy.GatewaySite
	var pxy *proxy.Proxy
	pxy = newProxyForBatchTest(priorProxy, func() error {
		reloadSnapshots = append(reloadSnapshots, append([]proxy.GatewaySite(nil), pxy.GatewaySites()...))
		return nil
	})

	saveErr := errors.New("synthetic save failure")
	p := &Poller{deps: PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return saveErr },
	}}
	batch := newRefreshBatch()
	batch.siteURLChanges["x.example.com"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)

	// Two reload calls: 1) candidate applied, 2) re-assert prior.
	if len(reloadSnapshots) != 2 {
		t.Fatalf("expected 2 reload calls (candidate + reassert), got %d", len(reloadSnapshots))
	}
	if reloadSnapshots[0][0].BackendURL != "http://10.0.0.99:8080" {
		t.Errorf("first reload saw %q, want candidate", reloadSnapshots[0][0].BackendURL)
	}
	if reloadSnapshots[1][0].BackendURL != "http://10.0.0.1:8080" {
		t.Errorf("re-assert reload saw %q, want priorSites (regression: was passing priorSites twice on the wire)", reloadSnapshots[1][0].BackendURL)
	}
	if cfg.Server.GatewaySites[0].BackendURL != "http://10.0.0.1:8080" {
		t.Errorf("in-memory not rolled back: %q", cfg.Server.GatewaySites[0].BackendURL)
	}
	if svc.divergences != 0 {
		t.Errorf("save failure with successful re-assert should NOT mark divergence; got %d", svc.divergences)
	}
}

// TestApplyRefreshBatch_SaveFailureWithGateway_ReassertDiverges
// covers the worst-case path: save fails AND the re-assert Reload
// also fails. Before commit "review-fix 1" this path silently
// discarded the re-assert error with `_ = ` and never called
// RecordDivergence, leaving the Settings banner green while Caddy
// was running an unknown shape.
func TestApplyRefreshBatch_SaveFailureWithGateway_ReassertDiverges(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GatewaySites: []config.GatewaySite{
				{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", DockerKey: "label:x", TLS: "auto"},
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	priorProxy := []proxy.GatewaySite{{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	calls := 0
	pxy := newProxyForBatchTest(priorProxy, func() error {
		calls++
		// First call succeeds (candidate applied), second + third
		// fail (re-assert candidate fails, re-assert rollback also
		// fails -> ErrDiverged inside ApplyGatewaySites).
		if calls == 1 {
			return nil
		}
		return errors.New("synthetic reload failure")
	})

	saveErr := errors.New("synthetic save failure")
	p := &Poller{deps: PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return saveErr },
	}}
	batch := newRefreshBatch()
	batch.siteURLChanges["x.example.com"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)

	if svc.divergences != 1 {
		t.Errorf("re-assert diverged path should call RecordDivergence; divergences=%d", svc.divergences)
	}
	if !svc.lastRefreshSuccessAt.IsZero() {
		t.Errorf("RecordRefreshTickSuccess wrongly called on divergence")
	}
	if cfg.Server.GatewaySites[0].BackendURL != "http://10.0.0.1:8080" {
		t.Errorf("in-memory not rolled back: %q", cfg.Server.GatewaySites[0].BackendURL)
	}
}

// TestApplyRefreshBatch_FiresOnConfigSavedForAppChanges pins the
// reverse-proxy route-rebuild hook the poller exposes via
// PollerDeps.OnConfigSaved. Without this callback firing, an
// App.Proxy=true entry whose IP shifted would have its config
// updated on disk but the route table would still point at the
// stale URL.
func TestApplyRefreshBatch_FiresOnConfigSavedForAppChanges(t *testing.T) {
	cfg := &config.Config{
		Apps: []config.AppConfig{
			{Name: "alpha", URL: "http://10.0.0.1:8080", DockerKey: "label:alpha", Proxy: true},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	fired := 0
	p := &Poller{deps: PollerDeps{
		Config:        cfg,
		ConfigMu:      &mu,
		Service:       svc,
		OnSave:        func() error { return nil },
		OnConfigSaved: func() { fired++ },
	}}
	batch := newRefreshBatch()
	batch.appURLChanges["alpha"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)
	if fired != 1 {
		t.Errorf("OnConfigSaved should fire once when an app URL changes; got %d", fired)
	}
}

// TestApplyRefreshBatch_DoesNotFireOnConfigSavedForGatewayOnlyChange
// guards the optimisation: gateway-only batches don't touch the
// reverse-proxy route table (App.Proxy entries aren't involved), so
// the callback would be a no-op rebuild. Skipping it cheaply avoids
// a useless map walk every gateway-IP-drift tick.
func TestApplyRefreshBatch_DoesNotFireOnConfigSavedForGatewayOnlyChange(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GatewaySites: []config.GatewaySite{
				{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", DockerKey: "label:x", TLS: "auto"},
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})

	priorProxy := []proxy.GatewaySite{{Domain: "x.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	pxy := newProxyForBatchTest(priorProxy, func() error { return nil })

	fired := 0
	p := &Poller{deps: PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave:        func() error { return nil },
		OnConfigSaved: func() { fired++ },
	}}
	batch := newRefreshBatch()
	batch.siteURLChanges["x.example.com"] = "http://10.0.0.99:8080"

	p.applyRefreshBatch(batch)
	if fired != 0 {
		t.Errorf("gateway-only changes should not trigger route-table rebuild; got %d fires", fired)
	}
}

// TestTick_LogsResolveErrors_NotJustNotFound pins the fix for the
// silent-continue regression in `tick()`. Before commit "review-fix
// 1", every non-NotFound resolveURL error (daemon disconnect,
// malformed key, no exposed ports) was swallowed with bare
// `continue`. The fix adds a Warn log; this test exercises the
// malformed-key branch which is the cheapest non-NotFound error to
// trigger.
func TestTick_LogsResolveErrors_NotJustNotFound(t *testing.T) {
	// Pre-populate one tracked app with a malformed DockerKey
	// (missing source prefix). resolveURL will fail with the
	// "malformed tracking key" error - NOT ErrContainerNotFound -
	// hitting the previously-silent continue branch.
	socket, cleanup := fakeDaemonForPoller(t, []ContainerSummary{
		{ID: "abc", Names: []string{"/foo"}},
	})
	defer cleanup()

	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: config.DiscoveryDockerConfig{
			Enabled: true, Endpoint: "unix://" + socket,
		}},
		Apps: []config.AppConfig{
			{
				Name: "x", URL: "http://10.0.0.1:8080",
				DockerKey:      "no-colon-here", // malformed -> non-NotFound err
				DockerEndpoint: "unix://" + socket,
				DockerStrategy: "container_ip",
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(&cfg.Discovery.Docker)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { return nil },
	})

	// The test asserts behaviour: the malformed-key entry is skipped
	// (no save, no panic, no batch entry) but the tick completes.
	// The log line itself is hard to assert in-process without
	// hijacking slog; we settle for "no save fired, no panic, no
	// stale entry stuck in batch".
	p.tick(context.Background())

	if !svc.LastSeen("no-colon-here").IsZero() {
		t.Errorf("RecordSeen wrongly fired for a key that failed to resolve")
	}
}

func TestPoller_PopulatesDockerStateCache(t *testing.T) {
	svc := &Service{}
	// Two running containers; the poller should inspect each and
	// populate the dockerState cache.
	svc.SetDockerStateCache(nil)
	callCount := 0
	var seenIDs []string
	inspector := func(_ context.Context, id string) (DockerState, error) {
		callCount++
		seenIDs = append(seenIDs, id)
		return DockerState{Status: "running", Health: "healthy", Image: "img"}, nil
	}

	cache := buildDockerStateCache(context.Background(), []trackedAppEntry{
		{name: "sonarr", key: "name:/sonarr"},
		{name: "radarr", key: "name:/radarr"},
	}, map[string]string{"sonarr": "id1", "radarr": "id2"}, inspector, svc.DockerStateSnapshot())

	if callCount != 2 {
		t.Fatalf("want 2 inspect calls, got %d", callCount)
	}
	if len(cache) != 2 {
		t.Fatalf("cache wrong size: %d", len(cache))
	}
	if cache["sonarr"].Status != "running" {
		t.Fatalf("sonarr state wrong: %+v", cache["sonarr"])
	}
}

func TestPoller_DiffEmitsBroadcastOnStateChange(t *testing.T) {
	prev := map[string]DockerState{"sonarr": {Status: "running", Health: "healthy"}}
	next := map[string]DockerState{"sonarr": {Status: "exited", Health: "none"}}
	diffs := diffDockerStates(prev, next)
	if len(diffs) != 1 {
		t.Fatalf("want 1 diff, got %d", len(diffs))
	}
	if diffs[0].Name != "sonarr" || diffs[0].State.Status != "exited" {
		t.Fatalf("diff mismatch: %+v", diffs[0])
	}
}

func TestPoller_DiffEmitsBroadcastOnHealthChange(t *testing.T) {
	prev := map[string]DockerState{"sonarr": {Status: "running", Health: "healthy"}}
	next := map[string]DockerState{"sonarr": {Status: "running", Health: "unhealthy"}}
	diffs := diffDockerStates(prev, next)
	if len(diffs) != 1 || diffs[0].State.Health != "unhealthy" {
		t.Fatalf("diff mismatch: %+v", diffs)
	}
}

func TestPoller_DiffNoChange_NoBroadcast(t *testing.T) {
	prev := map[string]DockerState{"sonarr": {Status: "running", Health: "healthy"}}
	next := map[string]DockerState{"sonarr": {Status: "running", Health: "healthy"}}
	if d := diffDockerStates(prev, next); len(d) != 0 {
		t.Fatalf("want no diff, got %d", len(d))
	}
}

func TestPoller_TransientInspectFailure_KeepsPreviousState(t *testing.T) {
	prev := map[string]DockerState{"sonarr": {Status: "running"}}
	inspector := func(_ context.Context, _ string) (DockerState, error) {
		return DockerState{}, errors.New("docker daemon timeout")
	}
	cache := buildDockerStateCache(context.Background(),
		[]trackedAppEntry{{name: "sonarr", key: "name:/sonarr"}},
		map[string]string{"sonarr": "id1"},
		inspector,
		prev,
	)
	if cache["sonarr"].Status != "running" {
		t.Fatalf("expected previous state preserved on transient failure, got %+v", cache["sonarr"])
	}
}

func TestPoller_MissingContainer_RecordsMissingStatus(t *testing.T) {
	inspector := func(_ context.Context, _ string) (DockerState, error) {
		// Unused; no container_id is resolved for "ghost".
		return DockerState{}, nil
	}
	// Empty resolved map -> ghost has no container ID -> status=missing.
	cache := buildDockerStateCache(context.Background(),
		[]trackedAppEntry{{name: "ghost", key: "name:/ghost"}},
		map[string]string{}, // not resolved
		inspector,
		nil,
	)
	if cache["ghost"].Status != "missing" {
		t.Fatalf("expected missing, got %q", cache["ghost"].Status)
	}
}

// fakeDaemonAllAware serves a container list that respects the ?all query
// param the way the real daemon does: a stopped container only appears when
// all=1. It also serves inspect with the given status. Used to prove the
// poller resolves tracked-app state against the ALL list, not the
// running-only list (a stopped tracked container must read as its real
// state, not "missing").
func fakeDaemonAllAware(t *testing.T, stopped *ContainerSummary, inspectStatus string) (string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, r *http.Request) {
		out := []ContainerSummary{}
		if r.URL.Query().Get("all") == "1" {
			out = []ContainerSummary{*stopped}
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	// Inspect: anything under /v1.41/containers/<id>/json that isn't the
	// list endpoint. Returns the configured status with an exit code.
	mux.HandleFunc("/v1.41/containers/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"State":{"Status":"` + inspectStatus + `","ExitCode":137,"StartedAt":"2026-01-01T00:00:00Z","FinishedAt":"2026-01-02T00:00:00Z"},"RestartCount":0,"Config":{"Image":"busybox"}}`))
	})
	return fakeDockerOverUnix(t, mux)
}

func TestPoller_Tick_StoppedContainerResolvesExitedNotMissing(t *testing.T) {
	// A tracked container that is stopped no longer appears in the
	// running-only scan used for URL refresh. State resolution must
	// fall back to the ALL list so the app reads as "exited" (Start
	// remains offered) rather than "missing" (lifecycle actions vanish).
	stopped := ContainerSummary{
		ID:    "s1",
		Names: []string{"/smoke-target"},
		Image: "busybox",
	}
	socket, cleanup := fakeDaemonAllAware(t, &stopped, "exited")
	defer cleanup()

	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled: true, Endpoint: "unix://" + socket,
	}
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *dockerCfg},
		Apps: []config.AppConfig{
			{
				Name:           "smoke-target",
				URL:            "http://10.0.0.1:80",
				DockerKey:      "name:smoke-target",
				DockerEndpoint: "unix://" + socket,
			},
		},
	}
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	if svc.client == nil {
		t.Fatalf("Service has no client; setup failure")
	}
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { return nil },
	})

	p.tick(context.Background())

	got := svc.DockerStateSnapshot()["smoke-target"]
	if got.Status != "exited" {
		t.Fatalf("stopped tracked container state = %q, want \"exited\" (a running-only scan would yield \"missing\")", got.Status)
	}
}

// --- Auto-import reconcile in the tick (Task 6) ---------------------------

// mutableDaemonForPoller serves a container set the test can swap
// between ticks (via the *set pointer), so update / vanish scenarios
// can change what the daemon reports across reconcile passes.
func mutableDaemonForPoller(t *testing.T, set *[]ContainerSummary) (string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(*set)
	})
	return fakeDockerOverUnix(t, mux)
}

// labeledSonarr is a running, media-network container carrying a stable
// discovery id -> tracking key "label:sonarr-auto", URL via container_ip
// -> http://10.0.0.42:8989.
func labeledSonarr() ContainerSummary {
	return ContainerSummary{
		ID:     "auto-sonarr",
		Names:  []string{"/sonarr"},
		Image:  "linuxserver/sonarr",
		Labels: map[string]string{LabelDiscoveryID: "sonarr-auto", LabelAppColor: "#00ff00"},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.42"}},
		},
		Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}},
	}
}

func findAppByKey(cfg *config.Config, key string) *config.AppConfig {
	for i := range cfg.Apps {
		if cfg.Apps[i].DockerKey == key {
			return &cfg.Apps[i]
		}
	}
	return nil
}

func findSiteByKey(cfg *config.Config, key string) *config.GatewaySite {
	for i := range cfg.Server.GatewaySites {
		if cfg.Server.GatewaySites[i].DockerKey == key {
			return &cfg.Server.GatewaySites[i]
		}
	}
	return nil
}

// autoImportCfg builds a Config whose discovery is enabled, points at
// the given socket, and uses container_ip with a network filter set so
// Scan bypasses self-detect gating.
func autoImportCfg(socket string, mode config.AutoImportMode) (*config.Config, *config.DiscoveryDockerConfig) {
	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: "container_ip",
		NetworkFilter:   "media",
		AutoImport:      mode,
	}
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *dockerCfg},
	}
	return cfg, dockerCfg
}

func TestTick_AutoImportAdd(t *testing.T) {
	set := []ContainerSummary{labeledSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	if svc.client == nil {
		t.Fatalf("service has no client")
	}
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalls++; return nil },
	})

	p.tick(context.Background())

	got := findAppByKey(cfg, "label:sonarr-auto")
	if got == nil || !got.DockerAutoImported {
		t.Fatalf("expected auto-imported Sonarr, apps=%+v", cfg.Apps)
	}
	if got.URL != "http://10.0.0.42:8989" {
		t.Errorf("imported URL = %q, want container URL", got.URL)
	}
	if saveCalls != 1 {
		t.Errorf("Save called %d times, want 1", saveCalls)
	}
}

func TestTick_AutoImportOff_DoesNotImport(t *testing.T) {
	set := []ContainerSummary{labeledSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportOff)
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalls++; return nil },
	})

	p.tick(context.Background())

	if findAppByKey(cfg, "label:sonarr-auto") != nil {
		t.Errorf("off mode must not import; apps=%+v", cfg.Apps)
	}
	if saveCalls != 0 {
		t.Errorf("Save called %d times, want 0 in off mode", saveCalls)
	}
}

func TestTick_AutoImportUpdate_ResyncsChangedFieldOnly(t *testing.T) {
	set := []ContainerSummary{labeledSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalls++; return nil },
	})

	// First tick (add) imports the app with color #00ff00.
	p.tick(context.Background())
	app := findAppByKey(cfg, "label:sonarr-auto")
	if app == nil || app.Color != "#00ff00" {
		t.Fatalf("add import failed, app=%+v", app)
	}
	if saveCalls != 1 {
		t.Fatalf("save after add = %d, want 1", saveCalls)
	}

	// Switch to update mode, no container change -> no update.
	cfg.Discovery.Docker.AutoImport = config.AutoImportUpdate
	p.tick(context.Background())
	if saveCalls != 1 {
		t.Errorf("unchanged update tick saved (%d); want no write", saveCalls)
	}

	// Change a label-controlled field -> update re-syncs it.
	set[0].Labels[LabelAppColor] = "#ff0000"
	p.tick(context.Background())
	app = findAppByKey(cfg, "label:sonarr-auto")
	if app == nil || app.Color != "#ff0000" {
		t.Fatalf("update did not re-sync color, app=%+v", app)
	}
	if saveCalls != 2 {
		t.Errorf("save after update = %d, want 2", saveCalls)
	}
}

func TestTick_AutoImportSyncRemovesVanished(t *testing.T) {
	// No containers reported; an auto app + its gateway site exist.
	set := []ContainerSummary{}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportSync)
	cfg.Apps = []config.AppConfig{{
		Name: "Sonarr", URL: "https://sonarr.example.com", Proxy: false,
		DockerKey: "label:gone", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "https://sonarr.example.com",
		DockerAutoImported: true,
	}}
	cfg.Server.GatewaySites = []config.GatewaySite{{
		Domain: "sonarr.example.com", BackendURL: "http://10.0.0.1:8989", TLS: "auto",
		DockerKey: "label:gone", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "http://10.0.0.1:8989",
	}}

	priorProxy := []proxy.GatewaySite{{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.1:8989", TLS: "auto"}}
	pxy := newProxyForBatchTest(priorProxy, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { saveCalls++; return nil },
	})

	p.tick(context.Background())

	if findAppByKey(cfg, "label:gone") != nil {
		t.Errorf("sync mode should remove vanished auto app; apps=%+v", cfg.Apps)
	}
	if findSiteByKey(cfg, "label:gone") != nil {
		t.Errorf("sync mode should remove the vanished gateway site; sites=%+v", cfg.Server.GatewaySites)
	}
	if saveCalls != 1 {
		t.Errorf("Save called %d times, want 1", saveCalls)
	}
}

func TestTick_AutoImportAddMode_DoesNotRemoveVanished(t *testing.T) {
	set := []ContainerSummary{}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	cfg.Apps = []config.AppConfig{{
		Name: "Sonarr", URL: "http://10.0.0.1:8989",
		DockerKey: "label:gone", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "http://10.0.0.1:8989",
		DockerAutoImported: true,
	}}
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalls++; return nil },
	})

	p.tick(context.Background())

	if findAppByKey(cfg, "label:gone") == nil {
		t.Errorf("add mode must NOT remove a vanished auto app")
	}
}

func TestTick_AutoImportLeavesManual(t *testing.T) {
	// Manual app (tracked but not auto-imported), container gone, sync mode.
	set := []ContainerSummary{}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportSync)
	cfg.Apps = []config.AppConfig{{
		Name: "Manual", URL: "http://10.0.0.1:8989",
		DockerKey: "label:manual", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "http://10.0.0.1:8989",
		DockerAutoImported: false,
	}}
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { saveCalls++; return nil },
	})

	p.tick(context.Background())

	if findAppByKey(cfg, "label:manual") == nil {
		t.Errorf("manual app must never be removed by sync; apps=%+v", cfg.Apps)
	}
	if saveCalls != 0 {
		t.Errorf("nothing should change for a manual app; saves=%d", saveCalls)
	}
}

func TestTick_GatewayAppURLNotClobbered_SiteRefreshes(t *testing.T) {
	// A gateway-routed tracked app: App.URL is the static domain, the
	// sibling tracked site carries the container backend. The refresh
	// pass must leave the app URL alone while updating the site backend.
	containers := []ContainerSummary{{
		ID:     "gw1",
		Names:  []string{"/sonarr"},
		Image:  "linuxserver/sonarr",
		Labels: map[string]string{LabelDiscoveryID: "gw-sonarr"},
		NetworkSettings: ContainerNetworks{
			Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.99"}},
		},
		Ports: []ContainerPort{{PrivatePort: 8989, Type: "tcp"}},
	}}
	socket, cleanup := fakeDaemonForPoller(t, containers)
	defer cleanup()

	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled: true, Endpoint: "unix://" + socket, NetworkStrategy: "container_ip",
	}
	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *dockerCfg},
		Apps: []config.AppConfig{{
			Name: "Sonarr", URL: "https://sonarr.example.com", Proxy: false,
			DockerKey: "label:gw-sonarr", DockerEndpoint: "unix://" + socket,
			DockerStrategy: "container_ip", DockerManagedURL: "https://sonarr.example.com",
			DockerAutoImported: true,
		}},
		Server: config.ServerConfig{GatewaySites: []config.GatewaySite{{
			Domain: "sonarr.example.com", BackendURL: "http://10.0.0.1:8989", TLS: "auto",
			DockerKey: "label:gw-sonarr", DockerEndpoint: "unix://" + socket,
			DockerStrategy: "container_ip", DockerManagedURL: "http://10.0.0.1:8989",
		}}},
	}

	priorProxy := []proxy.GatewaySite{{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.1:8989", TLS: "auto"}}
	pxy := newProxyForBatchTest(priorProxy, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return nil },
	})

	p.tick(context.Background())

	if cfg.Apps[0].URL != "https://sonarr.example.com" {
		t.Errorf("gateway app URL clobbered to %q; must stay the domain", cfg.Apps[0].URL)
	}
	if cfg.Server.GatewaySites[0].BackendURL != "http://10.0.0.99:8989" {
		t.Errorf("gateway site backend = %q, want refreshed to .99", cfg.Server.GatewaySites[0].BackendURL)
	}
}

// TestTick_GatewaySyncReconcile_AppURLStable_SiteRefreshes exercises the
// gateway path with auto-import ACTIVE (sync mode), unlike
// TestTick_GatewayAppURLNotClobbered_SiteRefreshes which leaves AutoImport
// unset. A properly gateway-labeled container is already tracked as an
// auto-imported gateway app + site. Across a tick where its IP changed, the
// reconcile Update must NOT clobber the gateway app's static https://domain
// URL (sameManagedFields sees the app URL unchanged) while the sibling
// gateway Site's BackendURL still refreshes to the new container IP.
func TestTick_GatewaySyncReconcile_AppURLStable_SiteRefreshes(t *testing.T) {
	// gwSonarr resolves to 10.0.0.42 via container_ip; bump the IP so the
	// site backend must refresh this tick.
	c := gwSonarr()
	c.NetworkSettings.Networks["media"] = ContainerNetwork{IPAddress: "10.0.0.77"}
	set := []ContainerSummary{c}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportSync)
	// Seed the already-imported gateway app + site (prior IP .42) so the tick
	// hits the reconcile Update + the site-refresh path rather than an Add.
	cfg.Apps = []config.AppConfig{{
		Name: "Sonarr", URL: "https://sonarr.example.com", Proxy: false, Enabled: true,
		Icon: config.AppIconConfig{Type: "dashboard", Name: "sonarr"},
		// Color differs from the container's #00ff00 label so Reconcile emits
		// an Update this tick: that proves the Update path re-syncs fields
		// WITHOUT clobbering the static gateway URL.
		Color:     "#111111",
		DockerKey: "label:sonarr-auto", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "https://sonarr.example.com",
		DockerAutoImported: true,
	}}
	cfg.Server.GatewaySites = []config.GatewaySite{{
		Domain: "sonarr.example.com", BackendURL: "http://10.0.0.42:8989", TLS: "auto",
		DockerKey: "label:sonarr-auto", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "http://10.0.0.42:8989",
	}}

	priorProxy := []proxy.GatewaySite{{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.42:8989", TLS: "auto"}}
	pxy := newProxyForBatchTest(priorProxy, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return nil },
	})

	p.tick(context.Background())

	app := findAppByKey(cfg, "label:sonarr-auto")
	if app == nil || app.URL != "https://sonarr.example.com" {
		t.Errorf("gateway app URL = %v, want stable https://sonarr.example.com (reconcile must not clobber)", app)
	}
	if app != nil && app.Color != "#00ff00" {
		t.Errorf("gateway app color = %q, want re-synced to #00ff00 (Update path ran)", app.Color)
	}
	site := findSiteByKey(cfg, "label:sonarr-auto")
	if site == nil || site.BackendURL != "http://10.0.0.77:8989" {
		t.Errorf("gateway site backend = %v, want refreshed to .77", site)
	}
}

// TestTick_AutoImportRollbackOnSaveFailure_Gateway is the gateway-path
// sibling of TestTick_AutoImportRollbackOnSaveFailure. A sync-mode tick wants
// to remove a vanished gateway app + its site, but SaveConfig fails. BOTH the
// app and the gateway site must be restored to their prior state.
func TestTick_AutoImportRollbackOnSaveFailure_Gateway(t *testing.T) {
	// Empty daemon: the tracked gateway container has vanished, so sync mode
	// plans a removal of both the app and its sibling site.
	set := []ContainerSummary{}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportSync)
	cfg.Apps = []config.AppConfig{{
		Name: "Sonarr", URL: "https://sonarr.example.com", Proxy: false, Enabled: true,
		DockerKey: "label:sonarr-auto", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "https://sonarr.example.com",
		DockerAutoImported: true,
	}}
	cfg.Server.GatewaySites = []config.GatewaySite{{
		Domain: "sonarr.example.com", BackendURL: "http://10.0.0.42:8989", TLS: "auto",
		DockerKey: "label:sonarr-auto", DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "http://10.0.0.42:8989",
	}}

	priorProxy := []proxy.GatewaySite{{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.42:8989", TLS: "auto"}}
	pxy := newProxyForBatchTest(priorProxy, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return errors.New("synthetic save failure") },
	})

	p.tick(context.Background())

	if findAppByKey(cfg, "label:sonarr-auto") == nil {
		t.Errorf("save failure must roll back the removal; app missing, apps=%+v", cfg.Apps)
	}
	site := findSiteByKey(cfg, "label:sonarr-auto")
	if site == nil || site.BackendURL != "http://10.0.0.42:8989" {
		t.Errorf("save failure must roll back the gateway site; got %v", site)
	}
}

// gwSonarr is labeledSonarr plus a gateway-domain label, so Scan
// suggests a gateway site and BuildDesired produces a Desired.Site.
func gwSonarr() ContainerSummary {
	c := labeledSonarr()
	c.Labels[LabelAppGatewayDomain] = "sonarr.example.com"
	c.Labels[LabelGatewayTLS] = "auto"
	return c
}

func TestTick_AutoImportAdd_GatewaySite(t *testing.T) {
	set := []ContainerSummary{gwSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	pxy := newProxyForBatchTest([]proxy.GatewaySite{}, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	saveCalls := 0
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { saveCalls++; return nil },
	})

	p.tick(context.Background())

	app := findAppByKey(cfg, "label:sonarr-auto")
	if app == nil || app.URL != "https://sonarr.example.com" || app.Proxy {
		t.Fatalf("gateway app not imported correctly: %+v", app)
	}
	site := findSiteByKey(cfg, "label:sonarr-auto")
	if site == nil || site.Domain != "sonarr.example.com" || site.BackendURL != "http://10.0.0.42:8989" {
		t.Fatalf("gateway site not imported correctly: %+v", site)
	}
	if saveCalls != 1 {
		t.Errorf("Save called %d times, want 1", saveCalls)
	}

	// Update mode + a changed app field re-syncs the app AND replaces the
	// existing gateway site (updateSites found-replace path).
	cfg.Discovery.Docker.AutoImport = config.AutoImportUpdate
	set[0].Labels[LabelAppColor] = "#abcdef"
	p.tick(context.Background())
	app = findAppByKey(cfg, "label:sonarr-auto")
	if app == nil || app.Color != "#abcdef" {
		t.Fatalf("gateway app update did not re-sync color: %+v", app)
	}
	if s := findSiteByKey(cfg, "label:sonarr-auto"); s == nil || s.Domain != "sonarr.example.com" {
		t.Fatalf("gateway site lost on update: %+v", s)
	}
}

// TestTick_AutoImportUpdate_InsertsGatewaySiteWhenLabelAdded covers the
// updateSites not-found insert path: a direct app is auto-imported, then
// a gateway-domain label is added to the container. Update mode then
// changes App.URL (direct -> gateway domain) so Reconcile emits an
// Update whose Desired.Site has no current match and must be inserted.
func TestTick_AutoImportUpdate_InsertsGatewaySiteWhenLabelAdded(t *testing.T) {
	set := []ContainerSummary{labeledSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	pxy := newProxyForBatchTest([]proxy.GatewaySite{}, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return nil },
	})

	p.tick(context.Background()) // direct import, no site
	if findSiteByKey(cfg, "label:sonarr-auto") != nil {
		t.Fatalf("direct import should not create a gateway site")
	}

	cfg.Discovery.Docker.AutoImport = config.AutoImportUpdate
	set[0].Labels[LabelAppGatewayDomain] = "sonarr.example.com"
	set[0].Labels[LabelGatewayTLS] = "auto"
	p.tick(context.Background())

	app := findAppByKey(cfg, "label:sonarr-auto")
	if app == nil || app.URL != "https://sonarr.example.com" {
		t.Fatalf("app URL should switch to the gateway domain: %+v", app)
	}
	site := findSiteByKey(cfg, "label:sonarr-auto")
	if site == nil || site.BackendURL != "http://10.0.0.42:8989" {
		t.Fatalf("gateway site should be inserted on the label add: %+v", site)
	}
}

// TestTick_AutoImportUpdate_GatewayOnlyLabelChangePropagates is #1's core
// case: a gateway app is imported, then a gateway-only label
// (require_auth) is added with no app-field change. Update mode must
// still re-sync the site, proving the site diff (not just the app diff)
// drives the update.
func TestTick_AutoImportUpdate_GatewayOnlyLabelChangePropagates(t *testing.T) {
	set := []ContainerSummary{gwSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	pxy := newProxyForBatchTest([]proxy.GatewaySite{}, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return nil },
	})

	p.tick(context.Background())
	if s := findSiteByKey(cfg, "label:sonarr-auto"); s == nil || s.RequireAuth {
		t.Fatalf("precondition: imported site must start without require_auth: %+v", s)
	}
	appBefore := findAppByKey(cfg, "label:sonarr-auto")

	// Add a gateway-only label; no app field changes.
	cfg.Discovery.Docker.AutoImport = config.AutoImportUpdate
	set[0].Labels[LabelGatewayRequireAuth] = "true"
	p.tick(context.Background())

	site := findSiteByKey(cfg, "label:sonarr-auto")
	if site == nil || !site.RequireAuth {
		t.Fatalf("gateway-only label change must propagate to the site: %+v", site)
	}
	if a := findAppByKey(cfg, "label:sonarr-auto"); a == nil || a.URL != appBefore.URL {
		t.Errorf("app URL must be unchanged by a gateway-only label: before=%q after=%+v", appBefore.URL, a)
	}
}

// TestTick_AutoImportUpdate_GatewayDomainDroppedRemovesSite: removing the
// gateway-domain label from a tracked container reverts its app to the
// direct container URL and drops the now-orphaned gateway site.
func TestTick_AutoImportUpdate_GatewayDomainDroppedRemovesSite(t *testing.T) {
	set := []ContainerSummary{gwSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	pxy := newProxyForBatchTest([]proxy.GatewaySite{}, func() error { return nil })

	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy,
		OnSave: func() error { return nil },
	})

	p.tick(context.Background())
	site := findSiteByKey(cfg, "label:sonarr-auto")
	if site == nil {
		t.Fatalf("precondition: gateway site must import")
	}
	backend := site.BackendURL // the direct container URL the app should revert to

	cfg.Discovery.Docker.AutoImport = config.AutoImportUpdate
	delete(set[0].Labels, LabelAppGatewayDomain)
	delete(set[0].Labels, LabelGatewayTLS)
	p.tick(context.Background())

	app := findAppByKey(cfg, "label:sonarr-auto")
	if app == nil || app.URL != backend {
		t.Fatalf("app should revert to its direct container URL %q: %+v", backend, app)
	}
	if s := findSiteByKey(cfg, "label:sonarr-auto"); s != nil {
		t.Fatalf("orphaned gateway site must be removed: %+v", s)
	}
}

func TestTick_AutoImportRollbackOnSaveFailure(t *testing.T) {
	set := []ContainerSummary{labeledSonarr()}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { return errors.New("synthetic save failure") },
	})

	p.tick(context.Background())

	if len(cfg.Apps) != 0 {
		t.Errorf("save failure must roll back the import; apps=%+v", cfg.Apps)
	}
}

func TestTick_AutoImportSkipsSelfAndNetworkFiltered(t *testing.T) {
	// Three containers: a self (muximux) container, an off-network
	// container, and the real sonarr on media. Only sonarr imports.
	set := []ContainerSummary{
		{
			ID: "self", Names: []string{"/muximux"}, Image: "ghcr.io/mescon/muximux:latest",
			Labels: map[string]string{LabelDiscoveryID: "self-id"},
			NetworkSettings: ContainerNetworks{
				Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.10"}},
			},
			Ports: []ContainerPort{{PrivatePort: 8080, Type: "tcp"}},
		},
		{
			ID: "other", Names: []string{"/grafana"}, Image: "grafana/grafana",
			Labels: map[string]string{LabelDiscoveryID: "grafana-id"},
			NetworkSettings: ContainerNetworks{
				Networks: map[string]ContainerNetwork{"isolated": {IPAddress: "10.9.9.9"}},
			},
			Ports: []ContainerPort{{PrivatePort: 3000, Type: "tcp"}},
		},
		labeledSonarr(),
	}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()

	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportAdd)
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: svc,
		OnSave: func() error { return nil },
	})

	p.tick(context.Background())

	if findAppByKey(cfg, "label:self-id") != nil {
		t.Errorf("self container must not be imported")
	}
	if findAppByKey(cfg, "label:grafana-id") != nil {
		t.Errorf("off-network container must not be imported")
	}
	if findAppByKey(cfg, "label:sonarr-auto") == nil {
		t.Errorf("media-network sonarr should import; apps=%+v", cfg.Apps)
	}
}

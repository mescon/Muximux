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

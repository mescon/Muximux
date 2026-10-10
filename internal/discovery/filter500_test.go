package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// Tests for #500 follow-ups 5 and 6: the poller tick honours network_filter
// like Scan, and reads Swarm /services once per tick.

func filteredContainer(name, id string, nets map[string]string) ContainerSummary {
	c := ContainerSummary{
		ID:     "id-" + name,
		Names:  []string{"/" + name},
		Image:  "acme/" + name,
		Labels: map[string]string{LabelDiscoveryID: id, "muximux.app.name": name, "muximux.app.port": "8080"},
		Ports:  []ContainerPort{{PrivatePort: 8080, Type: "tcp"}},
	}
	c.NetworkSettings.Networks = map[string]ContainerNetwork{}
	for n, ip := range nets {
		c.NetworkSettings.Networks[n] = ContainerNetwork{IPAddress: ip}
	}
	return c
}

func filterPoller(t *testing.T, set *[]ContainerSummary, mode config.AutoImportMode, services *atomic.Int32) (*Poller, *config.Config, *Service) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(*set) })
	mux.HandleFunc("/v1.41/services", func(w http.ResponseWriter, _ *http.Request) {
		if services != nil {
			services.Add(1)
		}
		_, _ = w.Write([]byte(whoamiService))
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	t.Cleanup(cleanup)
	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: config.StrategyContainerIP,
		NetworkFilter:   "stack_net",
		AutoImport:      mode,
	}
	cfg := &config.Config{Discovery: config.DiscoveryConfig{Docker: *dockerCfg}}
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: svc, OnSave: func() error { return nil }})
	return p, cfg, svc
}

func TestTick_ListsServicesOncePerTick(t *testing.T) {
	var calls atomic.Int32
	set := []ContainerSummary{swarmTask("stack_whoami.1.aaaaaaaaaaaaaaaaaaaaaaaaa", "traefik/whoami",
		map[string]string{LabelSwarmServiceID: "svc1"})}
	p, cfg, _ := filterPoller(t, &set, config.AutoImportSync, &calls)
	for tick := 1; tick <= 2; tick++ {
		p.tick(context.Background())
		if got := int(calls.Load()); got != tick {
			t.Fatalf("after tick %d: ListServices called %d times, want %d", tick, got, tick)
		}
	}
	if len(cfg.Apps) != 1 {
		t.Fatalf("apps = %+v, want the service imported once", cfg.Apps)
	}
}

func TestTick_MultiNetworkURLComesFromFilteredNetwork(t *testing.T) {
	// "other" sorts before "stack_net", so an unfiltered pick would win.
	set := []ContainerSummary{filteredContainer("multi", "multi", map[string]string{"other": "10.9.0.9", "stack_net": "10.0.1.5"})}
	p, cfg, _ := filterPoller(t, &set, config.AutoImportAdd, nil)
	p.tick(context.Background())
	a := findAppByKey(cfg, "label:multi")
	if a == nil || a.URL != "http://10.0.1.5:8080" {
		t.Fatalf("app = %+v, want URL from stack_net", a)
	}
	// The refresh path: a stale URL is rewritten from the filtered network.
	a.URL = "http://10.9.0.9:8080"
	p.tick(context.Background())
	if a = findAppByKey(cfg, "label:multi"); a.URL != "http://10.0.1.5:8080" {
		t.Fatalf("refreshed URL = %q, want the stack_net address", a.URL)
	}
}

func TestTick_FilteredOutContainerAbsentFromRefreshAndSync(t *testing.T) {
	set := []ContainerSummary{
		filteredContainer("in", "in", map[string]string{"stack_net": "10.0.1.5"}),
		filteredContainer("out", "out", map[string]string{"other": "10.9.0.9"}),
	}
	p, cfg, svc := filterPoller(t, &set, config.AutoImportSync, nil)
	// A previously imported app of the container outside the filter.
	cfg.Apps = []config.AppConfig{{
		Name: "out", URL: "http://10.9.0.9:8080", DockerKey: "label:out", DockerAutoImported: true,
		DockerEndpoint: cfg.Discovery.Docker.Endpoint, DockerStrategy: "container_ip",
	}}
	p.tick(context.Background())
	// Refresh agrees with sync: the container is not present for either.
	if svc.MissingSince("label:out").IsZero() {
		t.Fatal("refresh treated the filtered-out container as present")
	}
	if findAppByKey(cfg, "label:in") == nil {
		t.Fatal("filtered-in container not imported")
	}
	for i := 0; i < syncRemovalGraceTicks; i++ {
		p.tick(context.Background())
	}
	if findAppByKey(cfg, "label:out") != nil {
		t.Fatal("sync kept an app whose container is outside the filter")
	}
}

func TestTick_RekeyDoesNotMigrateFilteredOutContainers(t *testing.T) {
	set := []ContainerSummary{
		filteredContainer("in", "in", map[string]string{"stack_net": "10.0.1.5"}),
		filteredContainer("out", "out", map[string]string{"other": "10.9.0.9"}),
	}
	p, cfg, _ := filterPoller(t, &set, config.AutoImportOff, nil)
	ep := cfg.Discovery.Docker.Endpoint
	cfg.Apps = []config.AppConfig{
		{Name: "in", URL: "http://10.0.1.5:8080", DockerKey: "name:in", DockerEndpoint: ep, DockerStrategy: "container_ip"},
		{Name: "out", URL: "http://10.9.0.9:8080", DockerKey: "name:out", DockerEndpoint: ep, DockerStrategy: "container_ip"},
	}
	p.tick(context.Background())
	if findAppByKey(cfg, "label:in") == nil {
		t.Fatalf("filtered-in key not migrated: %+v", cfg.Apps)
	}
	if findAppByKey(cfg, "name:out") == nil || findAppByKey(cfg, "label:out") != nil {
		t.Fatalf("filtered-out key migrated: %+v", cfg.Apps)
	}
}

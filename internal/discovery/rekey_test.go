package discovery

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/proxy"
)

const ep = "unix:///var/run/docker.sock"

func TestPlanRekeys_ResolvedContainerGetsStableKey(t *testing.T) {
	cs := []ContainerSummary{{ID: "1", Names: []string{"/bindery_web.1.new"}, Labels: map[string]string{LabelSwarmServiceName: "bindery_web", LabelSwarmServiceID: "x"}},
		{ID: "2", Names: []string{"/home-sonarr-1"}, Labels: map[string]string{LabelComposeProject: "home", LabelComposeService: "sonarr", LabelDiscoveryID: "sonarr"}},
		{ID: "3", Names: []string{"/home-radarr-1"}, Labels: map[string]string{LabelComposeProject: "home", LabelComposeService: "radarr"}}}
	tr := trackedSet{apps: []trackedAppEntry{{key: "name:bindery_web.1.new", endpoint: ep}, {key: "name:home-sonarr-1", endpoint: ep},
		{key: "label:keep", endpoint: ep}, {key: "id:1", endpoint: ep}, {key: "name:home-radarr-1", endpoint: ep}}}
	got := planRekeys(&rekeyInput{Endpoint: ep, Tracked: tr, Containers: cs, HeldKeys: heldKeys(&tr, nil)})
	// id:1 loses: swarm:bindery_web is already planned for the first entry.
	want := map[string]string{"name:bindery_web.1.new": "swarm:bindery_web", "name:home-sonarr-1": "label:sonarr", "name:home-radarr-1": "compose:home:radarr"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestPlanRekeys_OrphanedTaskNameMatchesRunningService(t *testing.T) {
	cs := []ContainerSummary{{ID: "1", Names: []string{"/bindery_web.1.new"}, Labels: map[string]string{LabelSwarmServiceName: "bindery_web", LabelSwarmServiceID: "x"}}}
	tr := trackedSet{apps: []trackedAppEntry{{key: "name:bindery_web.1.71e9k1i0wfiyk5sbbjku668er", endpoint: ep}}}
	got := planRekeys(&rekeyInput{Endpoint: ep, Tracked: tr, Containers: cs, HeldKeys: heldKeys(&tr, nil)})
	if got["name:bindery_web.1.71e9k1i0wfiyk5sbbjku668er"] != "swarm:bindery_web" {
		t.Fatalf("got %v", got)
	}
	// A plain name with no task suffix is never guessed.
	tr = trackedSet{apps: []trackedAppEntry{{key: "name:bindery_web", endpoint: ep}}}
	if got = planRekeys(&rekeyInput{Endpoint: ep, Tracked: tr, Containers: cs, HeldKeys: heldKeys(&tr, nil)}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestPlanRekeys_ConflictFirstWins(t *testing.T) { // Review Focus 2; 25-char task ids match swarmTaskPattern
	cs := []ContainerSummary{{ID: "1", Names: []string{"/svc.1.cccccccccccccccccccccccccc"}, Labels: map[string]string{LabelSwarmServiceName: "svc", LabelSwarmServiceID: "x"}}}
	old1, old2 := "name:svc.1.aaaaaaaaaaaaaaaaaaaaaaaaa", "name:svc.1.bbbbbbbbbbbbbbbbbbbbbbbbb"
	tr := trackedSet{apps: []trackedAppEntry{{key: old1, endpoint: ep}, {key: old2, endpoint: ep}}}
	got := planRekeys(&rekeyInput{Endpoint: ep, Tracked: tr, Containers: cs, HeldKeys: heldKeys(&tr, nil)})
	if len(got) != 1 || got[old1] != "swarm:svc" {
		t.Fatalf("got %v", got)
	}
	// A target already held by another entry (live or quarantined) is never taken.
	q := []config.QuarantinedEntry{{Kind: "app", Key: "swarm:svc"}}
	if got = planRekeys(&rekeyInput{Endpoint: ep, Tracked: tr, Containers: cs, HeldKeys: heldKeys(&tr, q)}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestPlanRekeys_SkipsForeignEndpoint(t *testing.T) { // ruling 6
	cs := []ContainerSummary{{ID: "1", Names: []string{"/svc.1.cccccccccccccccccccccccccc"}, Labels: map[string]string{LabelSwarmServiceName: "svc", LabelSwarmServiceID: "x"}}}
	tr := trackedSet{apps: []trackedAppEntry{{key: "name:svc.1.aaaaaaaaaaaaaaaaaaaaaaaaa", endpoint: "tcp://other:2375"}}}
	if got := planRekeys(&rekeyInput{Endpoint: ep, Tracked: tr, Containers: cs, HeldKeys: heldKeys(&tr, nil)}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestHeldKeys_AllEndpointsAndQuarantine(t *testing.T) { // ruling 5
	tr := trackedSet{apps: []trackedAppEntry{{key: "label:a", endpoint: "tcp://other:2375"}}, sites: []trackedSiteEntry{{key: "label:s", endpoint: ep}}}
	got := heldKeys(&tr, []config.QuarantinedEntry{{Kind: "gateway", Key: "label:q"}})
	if !reflect.DeepEqual(got, map[string]bool{"label:a": true, "label:s": true, "label:q": true}) {
		t.Fatalf("got %v", got)
	}
}

func TestSwarmServiceFromTaskName(t *testing.T) {
	if s, ok := swarmServiceFromTaskName("bindery_web.1.71e9k1i0wfiyk5sbbjku668er"); !ok || s != "bindery_web" {
		t.Fatalf("got %q %v", s, ok)
	}
	for _, n := range []string{"bindery_web", "svc.1.short", "", ".1.71e9k1i0wfiyk5sbbjku668er"} {
		if _, ok := swarmServiceFromTaskName(n); ok {
			t.Errorf("%q matched", n)
		}
	}
}

// Sites are planned after apps, a site sharing its app's key is planned
// once, and keys that are not name:/id:, malformed, or whose container has
// no stable key are left alone.
func TestPlanRekeys_SitesAndUnmigratableKeys(t *testing.T) {
	cs := []ContainerSummary{
		{ID: "1", Names: []string{"/web-1"}, Labels: map[string]string{LabelComposeProject: "p", LabelComposeService: "web"}},
		{ID: "2", Names: []string{"/plain"}},
	}
	tr := trackedSet{
		apps:  []trackedAppEntry{{key: "name:web-1", endpoint: ep}, {key: "name:plain", endpoint: ep}, {key: "bogus", endpoint: ep}, {key: "swarm:x", endpoint: ep}},
		sites: []trackedSiteEntry{{key: "name:web-1", endpoint: ep}, {key: "id:2", endpoint: ep}, {key: "id:gone", endpoint: ep}},
	}
	got := planRekeys(&rekeyInput{Endpoint: ep, Tracked: tr, Containers: cs, HeldKeys: heldKeys(&tr, nil)})
	if !reflect.DeepEqual(got, map[string]string{"name:web-1": "compose:p:web"}) {
		t.Fatalf("got %v", got)
	}
}

func TestApplyRekeysToTracked_LiveEndpointOnly(t *testing.T) {
	rk := map[string]string{"name:a": "swarm:a"}
	tr := trackedSet{
		apps:  []trackedAppEntry{{key: "name:a", endpoint: ep}, {key: "name:a", endpoint: "tcp://other:2375"}},
		sites: []trackedSiteEntry{{key: "name:a", endpoint: ep}},
	}
	apps := []config.AppConfig{{DockerKey: "name:a", DockerEndpoint: ep}, {DockerKey: "name:a", DockerEndpoint: "tcp://other:2375"}, {Name: "manual"}}
	sites := []config.GatewaySite{{DockerKey: "name:a", DockerEndpoint: ep}}
	applyRekeysToTracked(rk, ep, &tr, apps, sites)
	if tr.apps[0].key != "swarm:a" || tr.apps[1].key != "name:a" || tr.sites[0].key != "swarm:a" {
		t.Fatalf("tracked = %+v", tr)
	}
	if apps[0].DockerKey != "swarm:a" || apps[1].DockerKey != "name:a" || apps[2].DockerKey != "" || sites[0].DockerKey != "swarm:a" {
		t.Fatalf("apps = %+v sites = %+v", apps, sites)
	}
	applyRekeysToTracked(nil, ep, &tr, apps, sites) // no-op
}

func rekeyTestPoller(cfg *config.Config, pxy *proxy.Proxy, save func() error) (*Poller, *Service) {
	var mu sync.RWMutex
	svc := NewService(&config.DiscoveryDockerConfig{})
	return NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: svc, Proxy: pxy, OnSave: save}), svc
}

// An app and its gateway site share the key and move together; an entry
// with the same old key on another endpoint is left for Re-link.
func TestApplyRefreshBatch_RekeyMovesAppAndSiteOnEndpoint(t *testing.T) {
	const old = "name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa"
	cfg := &config.Config{
		Apps: []config.AppConfig{
			{Name: "B", URL: "https://b.example.com", DockerKey: old, DockerEndpoint: ep, DockerManagedURL: "https://b.example.com", Enabled: true},
			{Name: "Other", URL: "http://o:1", DockerKey: old, DockerEndpoint: "tcp://other:2375", DockerManagedURL: "http://o:1", Enabled: true},
		},
		Server: config.ServerConfig{GatewaySites: []config.GatewaySite{{Domain: "b.example.com", BackendURL: "http://b:1", DockerKey: old, DockerEndpoint: ep, TLS: "auto"}}},
	}
	p, svc := rekeyTestPoller(cfg, nil, func() error { return nil })
	svc.MarkMissing(old)
	p.syncAbsent = map[string]int{old: 2}
	saved := 0
	p.deps.OnConfigSaved = func() { saved++ }
	b := newRefreshBatch()
	b.endpoint = ep
	b.rekeys = map[string]string{old: "swarm:b"}
	if b.empty() || !b.reconcileChangesApps() {
		t.Fatal("a re-key must count as a change")
	}
	p.applyRefreshBatch(b)
	if cfg.Apps[0].DockerKey != "swarm:b" || cfg.Server.GatewaySites[0].DockerKey != "swarm:b" || cfg.Apps[1].DockerKey != old {
		t.Fatalf("apps = %+v sites = %+v", cfg.Apps, cfg.Server.GatewaySites)
	}
	if svc.MissingSince("swarm:b").IsZero() || !svc.MissingSince(old).IsZero() {
		t.Fatal("missing record did not move to the new key")
	}
	if p.syncAbsent["swarm:b"] != 2 || p.syncAbsent[old] != 0 || saved != 1 {
		t.Fatalf("syncAbsent = %v saved = %d", p.syncAbsent, saved)
	}
}

// A target key taken between the tick's snapshot and the commit (by a live
// entry or a quarantined one) aborts the whole batch: nothing is renamed,
// saved or moved, and the next tick replans.
func TestApplyRefreshBatch_RekeyTargetTakenAbortsBatch(t *testing.T) {
	const old = "name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tc := range []struct {
		name  string
		setup func(cfg *config.Config)
	}{
		{"live", func(cfg *config.Config) {
			cfg.Apps = append(cfg.Apps, config.AppConfig{Name: "New", URL: "http://n:1", DockerKey: "swarm:b", Enabled: true})
		}},
		{"quarantined", func(cfg *config.Config) {
			cfg.QuarantineApp(&config.AppConfig{Name: "Q", DockerKey: "swarm:b", DockerAutoImported: true}, "r")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Apps: []config.AppConfig{{Name: "B", URL: "http://b:1", DockerKey: old, DockerManagedURL: "http://b:1", Enabled: true}}}
			tc.setup(cfg)
			saves := 0
			p, svc := rekeyTestPoller(cfg, nil, func() error { saves++; return nil })
			svc.MarkMissing(old)
			b := newRefreshBatch()
			b.rekeys = map[string]string{old: "swarm:b"}
			b.appURLChanges["swarm:b"] = "http://changed:1"
			p.applyRefreshBatch(b)
			if saves != 0 || cfg.Apps[0].DockerKey != old || svc.MissingSince(old).IsZero() {
				t.Fatalf("saves=%d apps=%+v", saves, cfg.Apps)
			}
			for i := range cfg.Apps {
				if cfg.Apps[i].URL == "http://changed:1" {
					t.Fatalf("batch partly applied: %+v", cfg.Apps)
				}
			}
		})
	}
}

// An invalid candidate rolls the re-key back with everything else.
func TestApplyRefreshBatch_RekeyRolledBackOnInvalidCandidate(t *testing.T) {
	const old = "name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa"
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "B", URL: "http://b:1", DockerKey: old, DockerManagedURL: "http://b:1", Enabled: true}}}
	p, svc := rekeyTestPoller(cfg, nil, func() error { t.Fatal("invalid candidate saved"); return nil })
	svc.MarkMissing(old)
	b := newRefreshBatch()
	b.rekeys = map[string]string{old: "swarm:b"}
	b.addApps = []config.AppConfig{{Name: "Broken", URL: "", DockerKey: "label:x", DockerAutoImported: true, Enabled: true}}
	p.applyRefreshBatch(b)
	if cfg.Apps[0].DockerKey != old || len(cfg.Apps) != 1 || svc.MissingSince(old).IsZero() || !svc.MissingSince("swarm:b").IsZero() {
		t.Fatalf("apps = %+v", cfg.Apps)
	}
}

// A Caddy reload failure (plain and diverged) and a save failure with a
// gateway re-assert all restore the old keys and leave Service records.
func TestApplyRefreshBatch_RekeyRolledBackOnReloadAndSaveFailure(t *testing.T) {
	const old = "name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa"
	cases := []struct {
		name   string
		reload func() error
		save   func() error
	}{
		{"reload", func() error { return errors.New("bad caddyfile") }, func() error { return nil }},
		{"save", func() error { return nil }, func() error { return errors.New("disk full") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Server: config.ServerConfig{GatewaySites: []config.GatewaySite{
				{Domain: "b.example.com", BackendURL: "http://10.0.0.1:1", DockerKey: old, DockerEndpoint: ep, TLS: "auto"},
			}}}
			pxy := newProxyForBatchTest([]proxy.GatewaySite{{Domain: "b.example.com", BackendURL: "http://10.0.0.1:1", TLS: "auto"}}, tc.reload)
			p, svc := rekeyTestPoller(cfg, pxy, tc.save)
			svc.MarkMissing(old)
			b := newRefreshBatch()
			b.endpoint = ep
			b.rekeys = map[string]string{old: "swarm:b"}
			b.siteURLChanges["b.example.com"] = "http://10.0.0.2:1"
			p.applyRefreshBatch(b)
			s := cfg.Server.GatewaySites[0]
			if s.DockerKey != old || s.BackendURL != "http://10.0.0.1:1" || svc.MissingSince(old).IsZero() {
				t.Fatalf("site = %+v", s)
			}
		})
	}
}

// With auto-import off, a manually tracked task-name app is still
// migrated, and a quarantined holder of the target blocks it.
func TestTick_RekeyRunsWithAutoImportOff(t *testing.T) {
	const old = "name:bindery_web.1.71e9k1i0wfiyk5sbbjku668er"
	for _, quarantined := range []bool{false, true} {
		set := []ContainerSummary{swarmTask("bindery_web.1.newtaskidnewtaskidnewtaskid", "bindery/web", map[string]string{"muximux.app.port": "8080"})}
		p, cfg := swarmPoller(t, &set, config.AutoImportOff)
		endpoint := cfg.Discovery.Docker.Endpoint
		cfg.Apps = []config.AppConfig{{Name: "Bindery", URL: "http://old:8080", DockerKey: old, DockerEndpoint: endpoint,
			DockerStrategy: string(config.StrategyContainerDNS), DockerManagedURL: "http://old:8080", Enabled: true}}
		if quarantined {
			cfg.QuarantineApp(&config.AppConfig{Name: "Q", DockerKey: "swarm:bindery_web", DockerAutoImported: true}, "r")
		}
		p.tick(context.Background())
		want := "swarm:bindery_web"
		if quarantined {
			want = old
		}
		if cfg.Apps[0].DockerKey != want {
			t.Fatalf("quarantined=%v: key %q", quarantined, cfg.Apps[0].DockerKey)
		}
		if !quarantined && cfg.Apps[0].URL != "http://bindery_web:8080" {
			t.Fatalf("URL = %q", cfg.Apps[0].URL)
		}
	}
}

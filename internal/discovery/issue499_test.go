package discovery

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// Regression tests for issue #499: Docker auto-import wrote unlabelled
// containers and apps without a URL, which bricked the next startup.

func swarmTask(name, image string, labels map[string]string, ports ...uint16) ContainerSummary {
	c := ContainerSummary{
		ID:     "id-" + name,
		Names:  []string{"/" + name},
		Image:  image,
		Labels: labels,
		NetworkSettings: ContainerNetworks{Networks: map[string]ContainerNetwork{
			"stack_net": {IPAddress: "10.0.1.5"},
		}},
	}
	if c.Labels == nil {
		c.Labels = map[string]string{}
	}
	// Swarm-ish labels every task carries.
	svc := strings.SplitN(name, ".", 2)[0]
	c.Labels["com.docker.swarm.service.name"] = svc
	c.Labels["com.docker.stack.namespace"] = strings.SplitN(svc, "_", 2)[0]
	for _, p := range ports {
		c.Ports = append(c.Ports, ContainerPort{PrivatePort: p, Type: "tcp"})
	}
	return c
}

func swarmPoller(t *testing.T, set *[]ContainerSummary, mode config.AutoImportMode) (*Poller, *config.Config) {
	t.Helper()
	socket, cleanup := mutableDaemonForPoller(t, set)
	t.Cleanup(cleanup)
	dockerCfg := &config.DiscoveryDockerConfig{
		Enabled:         true,
		Endpoint:        "unix://" + socket,
		NetworkStrategy: config.StrategyContainerDNS,
		NetworkFilter:   "stack_net",
		AutoImport:      mode,
	}
	cfg := &config.Config{Discovery: config.DiscoveryConfig{Docker: *dockerCfg}}
	var mu sync.RWMutex
	svc := NewService(dockerCfg)
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: svc, OnSave: func() error { return nil }})
	return p, cfg
}

// saveAndLoad writes cfg to a temp file and loads it back, failing the
// test when the saved file would not load (the startup brick of #499).
func saveAndLoad(t *testing.T, cfg *config.Config) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	again, err := config.Load(path)
	if err != nil {
		t.Fatalf("saved config does not load: %v", err)
	}
	return again
}

// Original reproduction (symptoms 1 + 2), inverted: unlabelled Swarm tasks
// and tasks with no EXPOSEd port must not be auto-imported, so nothing is
// written with an empty URL and the saved file loads.
func TestIssue499_UnlabeledSwarmTasksNotImported(t *testing.T) {
	set := []ContainerSummary{
		swarmTask("authentik_server.1.iigxo04fr5oc1ej3g25xnhblo", "ghcr.io/goauthentik/server:2025.8.1", nil),
		swarmTask("vaultwarden_app.1.aaaaaaaaaaaaaaaaaaaaaaaaa", "vaultwarden/server:latest", nil, 80),
		swarmTask("dockupdater_service.1.iuvsv0cw0furvgviw384nbnsj", "dockupdater/dockupdater:latest", nil),
		swarmTask("homarr_homarr.1.zzzzzzzzzzzzzzzzzzzzzzzzz", "ghcr.io/homarr-labs/homarr:latest",
			map[string]string{"muximux.app.name": "Homarr", "muximux.app.port": "7575"}),
	}
	p, cfg := swarmPoller(t, &set, config.AutoImportSync)
	p.tick(context.Background())

	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "Homarr" || cfg.Apps[0].URL == "" {
		t.Fatalf("apps = %+v, want only Homarr with a URL", cfg.Apps)
	}
	for i := range set[:3] {
		key, _ := KeyForContainer(&set[i])
		if a := findAppByKey(cfg, key); a != nil {
			t.Fatalf("unlabelled container %s imported: %+v", key, a)
		}
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	saveAndLoad(t, cfg)
}

func TestIssue499_UnlabeledNoPortNotImported(t *testing.T) { // F-01, F-04, Review Focus 6
	set := []ContainerSummary{
		swarmTask("authentik_server.1.iigxo04fr5oc1ej3g25xnhblo", "ghcr.io/goauthentik/server:2025.8.1", nil),
		swarmTask("vaultwarden_app.1.aaaaaaaaaaaaaaaaaaaaaaaaa", "vaultwarden/server:latest", map[string]string{"muximux.app.enabled": "true"}, 80),
		swarmTask("dockupdater_service.1.iuvsv0cw0furvgviw384nbnsj", "dockupdater/dockupdater:latest", nil),
		swarmTask("noport_app.1.zzzzzzzzzzzzzzzzzzzzzzzzz", "acme/noport:latest", map[string]string{"muximux.app.name": "NoPort"}),
		swarmTask("homarr_homarr.1.zzzzzzzzzzzzzzzzzzzzzzzzz", "ghcr.io/homarr-labs/homarr:latest",
			map[string]string{"muximux.app.name": "Homarr", "muximux.app.port": "7575"}),
	}
	p, cfg := swarmPoller(t, &set, config.AutoImportSync)
	p.tick(context.Background())
	names := []string{}
	for i := range cfg.Apps {
		names = append(names, cfg.Apps[i].Name)
		if cfg.Apps[i].URL == "" {
			t.Fatalf("app %q written without URL", cfg.Apps[i].Name)
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "Homarr,Vaultwarden" {
		t.Fatalf("apps = %v", names)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("saved config does not load: %v", err)
	}
	// The no-port container is present: further ticks must not remove anything.
	for i := 0; i < syncRemovalGraceTicks; i++ {
		p.tick(context.Background())
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("apps after the grace period = %d", len(cfg.Apps))
	}
}

// Ported from the original UpdateBlanksURL reproduction: losing the port
// must not blank the URL of an existing auto-imported app under update.
func TestIssue499_UpdateKeepsURLWhenPortLost(t *testing.T) { // F-03
	c := swarmTask("homarr_homarr.1.zzzzzzzzzzzzzzzzzzzzzzzzz", "ghcr.io/homarr-labs/homarr:latest",
		map[string]string{"muximux.app.name": "Homarr", "muximux.app.port": "7575", LabelDiscoveryID: "homarr"})
	set := []ContainerSummary{c}
	p, cfg := swarmPoller(t, &set, config.AutoImportUpdate)
	p.tick(context.Background())
	before := findAppByKey(cfg, "label:homarr").URL
	delete(set[0].Labels, "muximux.app.port")
	p.tick(context.Background())
	a := findAppByKey(cfg, "label:homarr")
	if a == nil || a.URL != before || a.DockerManagedURL != before || !a.DockerAutoImported {
		t.Fatalf("after: %+v (want url %q kept)", a, before)
	}
	if p.skipWarned["label:homarr"] != SkipNoPort {
		t.Fatalf("skipWarned = %v, want no_port recorded", p.skipWarned)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	saveAndLoad(t, cfg)
}

// Scan-level: labelled containers with no port, or whose URL cannot be
// built, are reported as skipped by Scan and are never imported, not even
// after the sync grace period.
func TestIssue499_ScanReportsNoPortNoURLAndPollerSkipsThem(t *testing.T) {
	noIP := ContainerSummary{
		ID: "id-noip", Names: []string{"/noip"}, Image: "acme/noip",
		Labels: map[string]string{LabelDiscoveryID: "noip", "muximux.app.name": "NoIP", "muximux.app.port": "8080"},
		// Attached to the filtered network but with no address, so the
		// container_ip URL builder fails.
		NetworkSettings: ContainerNetworks{Networks: map[string]ContainerNetwork{"media": {}}},
	}
	noPort := ContainerSummary{
		ID: "id-noport", Names: []string{"/noport"}, Image: "acme/noport",
		Labels:          map[string]string{LabelDiscoveryID: "noport", "muximux.app.name": "NoPort"},
		NetworkSettings: ContainerNetworks{Networks: map[string]ContainerNetwork{"media": {IPAddress: "10.0.0.7"}}},
	}
	set := []ContainerSummary{labeledSonarr(), noIP, noPort}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()
	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportSync)
	svc := NewService(dockerCfg)

	scan := svc.Scan(context.Background(), "")
	if scan.Error != "" || scan.ScanBlocked != "" {
		t.Fatalf("scan failed: %+v", scan)
	}
	codes := map[string]string{}
	for i := range scan.Suggestions {
		code := ""
		if s := scan.Suggestions[i].AutoImportSkip; s != nil {
			code = s.Code
		}
		codes[scan.Suggestions[i].Key] = code
	}
	want := map[string]string{"label:sonarr-auto": "", "label:noip": SkipNoURL, "label:noport": SkipNoPort}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("skip codes = %v, want %v", codes, want)
	}

	var mu sync.RWMutex
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: svc, OnSave: func() error { return nil }})
	for i := 0; i <= syncRemovalGraceTicks; i++ {
		p.tick(context.Background())
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0].DockerKey != "label:sonarr-auto" {
		t.Fatalf("apps = %+v, want only the eligible container", cfg.Apps)
	}
	if p.skipWarned["label:noip"] != SkipNoURL || p.skipWarned["label:noport"] != SkipNoPort {
		t.Fatalf("skipWarned = %v", p.skipWarned)
	}
}

func TestTick_UnlabeledPresentContainerDetachesNotRemoves(t *testing.T) { // Review Focus 1
	set := []ContainerSummary{swarmTask("bindery_web.1.aaaaaaaaaaaaaaaaaaaaaaaaa", "bindery/web", nil, 8080)}
	p, cfg := swarmPoller(t, &set, config.AutoImportSync)
	key, _ := KeyForContainer(&set[0]) // name: before Task 13, swarm: after it
	cfg.Apps = []config.AppConfig{{Name: "Bindery", URL: "http://bindery_web:8080", DockerKey: key,
		DockerEndpoint: cfg.Discovery.Docker.Endpoint, DockerStrategy: "container_dns", DockerManagedURL: "http://bindery_web:8080",
		DockerAutoImported: true, Enabled: true, Pinned: true}}
	for i := 0; i <= syncRemovalGraceTicks; i++ {
		p.tick(context.Background())
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0].DockerAutoImported || cfg.Apps[0].DockerKey == "" || !cfg.Apps[0].Pinned {
		t.Fatalf("apps = %+v", cfg.Apps)
	}
}

func TestTick_ExplicitModeDetachesUnenabled(t *testing.T) { // Review Focus 4
	for _, mode := range []config.AutoImportMode{config.AutoImportAdd, config.AutoImportUpdate} {
		set := []ContainerSummary{swarmTask("homarr_homarr.1.zzzzzzzzzzzzzzzzzzzzzzzzz", "ghcr.io/homarr-labs/homarr",
			map[string]string{"muximux.app.name": "Homarr", "muximux.app.port": "7575", LabelDiscoveryID: "homarr"})}
		p, cfg := swarmPoller(t, &set, mode)
		p.tick(context.Background()) // imported (default mode)
		cfg.Discovery.Docker.RequireExplicitEnable = true
		p.deps.Service.Reconfigure(&cfg.Discovery.Docker)
		p.tick(context.Background())
		a := findAppByKey(cfg, "label:homarr")
		if a == nil {
			t.Fatalf("mode %s: app removed", mode)
		}
		if wantAuto := mode == config.AutoImportAdd; a.DockerAutoImported != wantAuto {
			t.Fatalf("mode %s: auto=%v want %v", mode, a.DockerAutoImported, wantAuto)
		}
	}
}

func TestTick_DisabledLabelRemovesUnderSyncOnly(t *testing.T) {
	for _, mode := range []config.AutoImportMode{config.AutoImportUpdate, config.AutoImportSync} {
		set := []ContainerSummary{swarmTask("homarr_homarr.1.zzzzzzzzzzzzzzzzzzzzzzzzz", "ghcr.io/homarr-labs/homarr",
			map[string]string{"muximux.app.name": "Homarr", "muximux.app.port": "7575", LabelDiscoveryID: "homarr"})}
		p, cfg := swarmPoller(t, &set, mode)
		p.tick(context.Background())
		if findAppByKey(cfg, "label:homarr") == nil {
			t.Fatalf("mode %s: not imported", mode)
		}
		set[0].Labels["muximux.app.enabled"] = "false"
		for i := 0; i <= syncRemovalGraceTicks; i++ {
			p.tick(context.Background())
		}
		a := findAppByKey(cfg, "label:homarr")
		switch mode {
		case config.AutoImportSync:
			if a != nil {
				t.Fatalf("sync: opted-out app kept: %+v", a)
			}
		default:
			if a == nil || !a.DockerAutoImported {
				t.Fatalf("update: app = %+v, want kept and still auto", a)
			}
		}
	}
}

func TestTick_QuarantinedEntryReplacedByValidDesired(t *testing.T) {
	set := []ContainerSummary{swarmTask("vaultwarden_app.1.aaaaaaaaaaaaaaaaaaaaaaaaa", "vaultwarden/server:latest",
		map[string]string{LabelDiscoveryID: "vw"}, 80)}
	p, cfg := swarmPoller(t, &set, config.AutoImportUpdate)
	cfg.QuarantineApp(&config.AppConfig{Name: "Vaultwarden", DockerKey: "label:vw", DockerEndpoint: cfg.Discovery.Docker.Endpoint,
		DockerStrategy: "container_dns", DockerAutoImported: true, Enabled: true}, "url is required")
	p.tick(context.Background())
	a := findAppByKey(cfg, "label:vw")
	if a == nil || a.URL == "" || len(cfg.Quarantined()) != 0 {
		t.Fatalf("app=%+v quarantined=%+v", a, cfg.Quarantined())
	}
	again := saveAndLoad(t, cfg)
	n := 0
	for i := range again.Apps {
		if again.Apps[i].DockerKey == "label:vw" {
			n++
		}
	}
	if n != 1 || len(again.Quarantined()) != 0 {
		t.Fatalf("after reload: %d apps with the key, quarantined=%+v", n, again.Quarantined())
	}
}

func TestTick_QuarantinedEntryDroppedWhenContainerGoneInSync(t *testing.T) {
	set := []ContainerSummary{}
	p, cfg := swarmPoller(t, &set, config.AutoImportSync)
	cfg.QuarantineApp(&config.AppConfig{Name: "Gone", DockerKey: "label:gone", DockerAutoImported: true, Enabled: true}, "url is required")
	cfg.QuarantineSite(&config.GatewaySite{Domain: "gone.example.com", DockerKey: "label:gone"}, "its app is quarantined")
	p.tick(context.Background())
	if len(cfg.Quarantined()) != 2 {
		t.Fatalf("removal must wait for the grace period: %+v", cfg.Quarantined())
	}
	for i := 0; i < syncRemovalGraceTicks; i++ {
		p.tick(context.Background())
	}
	if len(cfg.Quarantined()) != 0 {
		t.Fatalf("quarantined = %+v", cfg.Quarantined())
	}
}

// Row 16 / ruling 4: a label rename that collides with a manual app by slug
// is dropped by dedupe; the existing auto app must not be removed by sync.
func TestTick_DedupeDroppedRenameIsNotRemovedInSync(t *testing.T) { // Review Focus 6
	set := []ContainerSummary{swarmTask("ha_ha.1.zzzzzzzzzzzzzzzzzzzzzzzzz", "homeassistant/home-assistant",
		map[string]string{LabelDiscoveryID: "ha", "muximux.app.name": "HA", "muximux.app.port": "8123"})}
	p, cfg := swarmPoller(t, &set, config.AutoImportSync)
	cfg.Apps = []config.AppConfig{{Name: "Home Assistant", URL: "http://manual:8123", Enabled: true}}
	p.tick(context.Background())
	if findAppByKey(cfg, "label:ha") == nil {
		t.Fatal("not imported")
	}
	set[0].Labels["muximux.app.name"] = "Home-Assistant"
	for i := 0; i <= syncRemovalGraceTicks; i++ {
		p.tick(context.Background())
	}
	if a := findAppByKey(cfg, "label:ha"); a == nil || a.Name != "HA" {
		t.Fatalf("app = %+v, want the existing HA kept", a)
	}
}

func TestBuildDesired_SkipsAndRecordsCodes(t *testing.T) {
	p := NewPoller(PollerDeps{})
	f := false
	scan := ScanResult{Suggestions: []Suggestion{
		{Key: "label:ok", Name: "Ok", URL: "http://ok:1", Labeled: true},
		{Key: "label:off", Name: "Off", URL: "http://off:1", Labeled: true, LabelEnabled: &f, AutoImportSkip: &AutoImportSkip{Code: SkipDisabled}},
		{Key: "label:bad", Name: "Bad", URL: "javascript:alert(1)", Labeled: true},
		// Gateway requested but no container URL: the app URL becomes the
		// public domain (valid), the site would have no backend.
		{Key: "label:gw", Name: "Gw", Labeled: true, GatewayRequested: true, SuggestedDomain: "gw.example.com"},
		{Key: "label:ha2", Name: "Home-Assistant", URL: "http://ha2:8123", Labeled: true},
	}}
	current := []config.AppConfig{{Name: "Home Assistant", URL: "http://ha:8123", Enabled: true}}
	desired, skipped := p.buildDesired(&scan, "unix:///s", current, nil)
	if len(desired) != 1 || desired[0].App.DockerKey != "label:ok" {
		t.Fatalf("desired = %+v", desired)
	}
	want := map[string]string{"label:off": SkipDisabled, "label:bad": SkipInvalid, "label:gw": SkipNoURL, "label:ha2": SkipInvalid}
	if !reflect.DeepEqual(skipped, want) {
		t.Fatalf("skipped = %v, want %v", skipped, want)
	}
	if p.skipWarned["label:ha2"] != SkipInvalid || p.skipWarned["label:off"] != SkipDisabled {
		t.Fatalf("skipWarned = %v", p.skipWarned)
	}
	// A repeat of the same skip does not log again (the record is unchanged).
	p.buildDesired(&scan, "unix:///s", current, nil)
	if p.skipWarned["label:bad"] != SkipInvalid {
		t.Fatalf("skipWarned = %v", p.skipWarned)
	}
	// Eligible again: the warned record clears so a later skip warns again.
	scan.Suggestions = scan.Suggestions[:1]
	scan.Suggestions[0].Key = "label:bad"
	p.buildDesired(&scan, "unix:///s", nil, nil)
	if _, ok := p.skipWarned["label:bad"]; ok {
		t.Fatal("skipWarned not cleared on recovery")
	}
}

func TestQuarantinedAppKeys_AppsOnly(t *testing.T) { // ruling 2
	got := quarantinedAppKeys([]config.QuarantinedEntry{{Kind: "app", Key: "label:a"}, {Kind: "gateway", Key: "label:b"}})
	if !reflect.DeepEqual(got, map[string]bool{"label:a": true}) {
		t.Fatalf("got %v", got)
	}
}

func TestApplyReconcile_UpdateDropsQuarantinedSite(t *testing.T) { // ruling 2
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "VW", URL: "https://vw.example.com", DockerKey: "label:vw", DockerAutoImported: true, Enabled: true}}}
	cfg.QuarantineSite(&config.GatewaySite{Domain: "vw.example.com", BackendURL: "", DockerKey: "label:vw"}, "backend_url is required")
	var mu sync.RWMutex
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: NewService(&config.DiscoveryDockerConfig{}), OnSave: func() error { return nil }})
	b := newRefreshBatch()
	b.updateApps = []config.AppConfig{cfg.Apps[0]}
	b.updateSites = []config.GatewaySite{{Domain: "vw.example.com", BackendURL: "http://vw:80", DockerKey: "label:vw"}}
	p.applyReconcile(b)
	if cfg.HasQuarantined("label:vw") || findSiteByKey(cfg, "label:vw") == nil {
		t.Fatalf("quarantined=%+v sites=%+v", cfg.Quarantined(), cfg.Server.GatewaySites)
	}
}

func TestApplyReconcile_DetachClearsAutoOnly(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{
		{Name: "A", URL: "http://a", DockerKey: "label:a", DockerAutoImported: true, Enabled: true},
		{Name: "M", URL: "http://m", DockerKey: "label:m", Enabled: true},
	}}
	var mu sync.RWMutex
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: NewService(&config.DiscoveryDockerConfig{}), OnSave: func() error { return nil }})
	b := newRefreshBatch()
	b.detach = map[string]string{"label:a": SkipUnlabeled, "label:m": SkipUnlabeled}
	_, rec := p.applyReconcile(b)
	detached := rec.detached
	if cfg.Apps[0].DockerAutoImported || cfg.Apps[0].DockerKey != "label:a" || cfg.Apps[1].DockerKey != "label:m" {
		t.Fatalf("apps = %+v", cfg.Apps)
	}
	if len(detached) != 1 || detached[0].key != "label:a" || detached[0].reason != SkipUnlabeled {
		t.Fatalf("detached = %+v", detached)
	}
}

func TestApplyRefreshBatch_RollsBackInvalidReconcile(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "Keep", URL: "http://old", DockerKey: "label:keep", DockerManagedURL: "http://old", Enabled: true}}}
	cfg.QuarantineApp(&config.AppConfig{Name: "Q", DockerKey: "label:q", DockerAutoImported: true}, "r")
	var mu sync.RWMutex
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: NewService(&config.DiscoveryDockerConfig{}), OnSave: func() error { return nil }})
	batch := func() *refreshBatch {
		b := newRefreshBatch()
		b.appURLChanges["label:keep"] = "http://new"
		b.addApps = []config.AppConfig{{Name: "Broken", URL: "", DockerKey: "label:q", DockerAutoImported: true, Enabled: true}}
		return b
	}
	p.applyRefreshBatch(batch())
	// The whole candidate is rolled back, the URL refresh included.
	if findAppByKey(cfg, "label:q") != nil || cfg.Apps[0].URL != "http://old" || cfg.Validate() != nil {
		t.Fatalf("apps = %+v", cfg.Apps)
	}
	if !cfg.HasQuarantined("label:q") {
		t.Fatal("rollback must restore the quarantine the Add dropped")
	}
	if p.candidateInvalid == "" {
		t.Fatal("rollback not recorded (the ERROR is logged on this transition only)")
	}
	first := p.candidateInvalid
	p.applyRefreshBatch(batch())
	if p.candidateInvalid != first {
		t.Fatalf("candidateInvalid changed on a repeat: %q -> %q", first, p.candidateInvalid)
	}
	ok := newRefreshBatch()
	ok.appURLChanges["label:keep"] = "http://newer"
	p.applyRefreshBatch(ok)
	if p.candidateInvalid != "" {
		t.Fatal("a valid apply must clear candidateInvalid")
	}
}

// An invalid candidate config is never saved: OnSave is not called and the
// URL refresh in the same batch is rolled back with the reconcile.
func TestApplyRefreshBatch_InvalidCandidateSkipsSave(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "Keep", URL: "http://old", DockerKey: "label:keep", DockerManagedURL: "http://old", Enabled: true}}}
	var mu sync.RWMutex
	saves := 0
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: NewService(&config.DiscoveryDockerConfig{}),
		OnSave: func() error { saves++; return nil }})
	b := newRefreshBatch()
	b.appURLChanges["label:keep"] = "http://new"
	b.addApps = []config.AppConfig{{Name: "Broken", URL: "", DockerKey: "label:x", DockerAutoImported: true, Enabled: true}}
	p.applyRefreshBatch(b)
	if saves != 0 || len(cfg.Apps) != 1 || cfg.Apps[0].URL != "http://old" || p.candidateInvalid == "" {
		t.Fatalf("saves=%d apps=%+v invalid=%q", saves, cfg.Apps, p.candidateInvalid)
	}
}

// A desired gateway site that fails the per-site load rule is skipped as
// invalid for that container only.
func TestBuildDesired_InvalidSiteSkipsOnlyThatKey(t *testing.T) {
	p := NewPoller(PollerDeps{})
	authOn := true
	scan := ScanResult{Suggestions: []Suggestion{
		{Key: "label:ok", Name: "Ok", URL: "http://ok:1", BackendURL: "http://ok:1", Labeled: true,
			GatewayRequested: true, SuggestedDomain: "ok.example.com"},
		{Key: "label:gated", Name: "Gated", URL: "http://gated:1", BackendURL: "http://gated:1", Labeled: true,
			GatewayRequested: true, SuggestedDomain: "gated.example.com",
			SuggestedGateway: &SuggestedGatewayConfig{RequireAuth: &authOn}},
		{Key: "label:clash", Name: "Clash", URL: "http://clash:1", BackendURL: "http://clash:1", Labeled: true,
			GatewayRequested: true, SuggestedDomain: "dash.example.com"},
	}}
	srv := &config.ServerConfig{TLS: config.TLSConfig{Domain: "dash.example.com"}}
	desired, skipped := p.buildDesired(&scan, "unix:///s", nil, srv)
	if len(desired) != 1 || desired[0].App.DockerKey != "label:ok" {
		t.Fatalf("desired = %+v", desired)
	}
	if !reflect.DeepEqual(skipped, map[string]string{"label:gated": SkipInvalid, "label:clash": SkipInvalid}) {
		t.Fatalf("skipped = %v", skipped)
	}
}

// skipWarned only holds keys present in the latest scan.
func TestBuildDesired_PrunesVanishedSkipWarned(t *testing.T) {
	p := NewPoller(PollerDeps{})
	scan := ScanResult{Suggestions: []Suggestion{
		{Key: "label:a", Name: "A", AutoImportSkip: &AutoImportSkip{Code: SkipNoPort}},
		{Key: "label:b", Name: "B", AutoImportSkip: &AutoImportSkip{Code: SkipUnlabeled}},
	}}
	p.buildDesired(&scan, "unix:///s", nil, nil)
	if len(p.skipWarned) != 2 {
		t.Fatalf("skipWarned = %v", p.skipWarned)
	}
	scan.Suggestions = scan.Suggestions[1:]
	p.buildDesired(&scan, "unix:///s", nil, nil)
	if !reflect.DeepEqual(p.skipWarned, map[string]string{"label:b": SkipUnlabeled}) {
		t.Fatalf("skipWarned = %v, want label:a pruned", p.skipWarned)
	}
}

// One container with an invalid gateway site must not block the others:
// the good container is imported and the bad one is skipped, every tick.
func TestTick_BadSiteDoesNotBlockGoodContainer(t *testing.T) {
	bad := gwSonarr()
	bad.ID = "auto-bad"
	bad.Names = []string{"/bad"}
	bad.Labels = map[string]string{LabelDiscoveryID: "bad", "muximux.app.name": "Bad",
		LabelAppGatewayDomain: "bad.example.com", LabelGatewayRequireAuth: "true"} // no session_cookie_domain
	set := []ContainerSummary{labeledSonarr(), bad}
	socket, cleanup := mutableDaemonForPoller(t, &set)
	defer cleanup()
	cfg, dockerCfg := autoImportCfg(socket, config.AutoImportSync)
	var mu sync.RWMutex
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: NewService(dockerCfg), OnSave: func() error { return nil }})
	for i := 0; i <= syncRemovalGraceTicks; i++ {
		p.tick(context.Background())
	}
	if findAppByKey(cfg, "label:sonarr-auto") == nil || findAppByKey(cfg, "label:bad") != nil || findSiteByKey(cfg, "label:bad") != nil {
		t.Fatalf("apps=%+v sites=%+v", cfg.Apps, cfg.Server.GatewaySites)
	}
	if p.skipWarned["label:bad"] != SkipInvalid || p.candidateInvalid != "" {
		t.Fatalf("skipWarned=%v candidateInvalid=%q", p.skipWarned, p.candidateInvalid)
	}
	saveAndLoad(t, cfg)
}

// A failed save rolls the quarantine back with the apps: a re-added key
// whose quarantined copy was dropped in memory gets it back.
func TestApplyRefreshBatch_SaveFailureRestoresQuarantine(t *testing.T) {
	cfg := &config.Config{}
	cfg.QuarantineApp(&config.AppConfig{Name: "Q", DockerKey: "label:q", DockerAutoImported: true}, "r")
	var mu sync.RWMutex
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: NewService(&config.DiscoveryDockerConfig{}),
		OnSave: func() error { return saveFailureError{} }})
	b := newRefreshBatch()
	b.addApps = []config.AppConfig{{Name: "Q", URL: "http://q", DockerKey: "label:q", DockerAutoImported: true, Enabled: true}}
	b.detach = map[string]string{"label:none": SkipUnlabeled}
	p.applyRefreshBatch(b)
	if len(cfg.Apps) != 0 || !cfg.HasQuarantined("label:q") {
		t.Fatalf("apps=%+v quarantined=%+v", cfg.Apps, cfg.Quarantined())
	}
}

func TestRefreshBatch_DetachCountsAsAppChange(t *testing.T) {
	b := newRefreshBatch()
	if !b.empty() || b.reconcileChangesApps() {
		t.Fatal("new batch must be empty")
	}
	b.detach["label:a"] = SkipUnlabeled
	if b.empty() || !b.reconcileChangesApps() {
		t.Fatal("a detach must make the batch non-empty and touch apps")
	}
}

// A tracked container that is gone is removed only by sync mode, and only
// for auto-imported entries, once it has been absent past the grace period.
// Hand-added apps and apps in off mode are never removed.
func TestIssue499_AbsentContainerRemovedOnlyBySyncOfAutoImported(t *testing.T) {
	for _, tc := range []struct {
		mode     config.AutoImportMode
		auto     bool
		wantApps int
	}{
		{config.AutoImportSync, true, 0},
		{config.AutoImportSync, false, 1},
		{config.AutoImportOff, true, 1},
		{config.AutoImportAdd, true, 1},
		{config.AutoImportUpdate, true, 1},
	} {
		set := []ContainerSummary{}
		p, cfg := swarmPoller(t, &set, tc.mode)
		cfg.Apps = []config.AppConfig{{
			Name: "Bindery_web.1.71e9k1i0wfiyk5sbbjku668er", URL: "http://bindery_web.1.71e9k1i0wfiyk5sbbjku668er:8080",
			DockerKey: "name:bindery_web.1.71e9k1i0wfiyk5sbbjku668er", DockerEndpoint: cfg.Discovery.Docker.Endpoint,
			DockerStrategy: "container_dns", DockerAutoImported: tc.auto, Enabled: true,
		}}
		for i := 0; i < 6; i++ {
			p.tick(context.Background())
		}
		if len(cfg.Apps) != tc.wantApps {
			t.Errorf("mode=%s auto_imported=%v: apps left = %d, want %d", tc.mode, tc.auto, len(cfg.Apps), tc.wantApps)
		}
	}
}

func TestIssue499_RedeployKeepsOneAppAndState(t *testing.T) { // F-07
	old := swarmTask("bindery_web.1.71e9k1i0wfiyk5sbbjku668er", "bindery/web", map[string]string{"muximux.app.port": "8080"})
	for _, mode := range []config.AutoImportMode{config.AutoImportUpdate, config.AutoImportSync} {
		set := []ContainerSummary{old}
		p, cfg := swarmPoller(t, &set, mode)
		p.tick(context.Background())
		cfg.Apps[0].DockerKey = "name:bindery_web.1.71e9k1i0wfiyk5sbbjku668er" // a 3.5.0 install tracked it by task name
		cfg.Apps[0].Pinned = true
		set = []ContainerSummary{swarmTask("bindery_web.1.newtaskidnewtaskidnewtaskid", "bindery/web", map[string]string{"muximux.app.port": "8080"})}
		for i := 0; i <= syncRemovalGraceTicks; i++ {
			p.tick(context.Background())
			if len(cfg.Apps) != 1 {
				t.Fatalf("mode %s tick %d: %d apps", mode, i, len(cfg.Apps))
			}
		}
		a := &cfg.Apps[0]
		if a.DockerKey != "swarm:bindery_web" || !a.Pinned || a.URL != "http://bindery_web:8080" {
			t.Fatalf("mode %s: %+v", mode, a)
		}
		if p.deps.Service.LastSeen("swarm:bindery_web").IsZero() {
			t.Fatalf("mode %s: Service records not on the new key", mode)
		}
	}
}

// Ruling 16: on a failed save the config rolls back to the old keys, so the
// Service records must stay on the old keys too.
func TestApplyRefreshBatch_RekeyRenamesServiceOnlyAfterSave(t *testing.T) {
	cfg := &config.Config{Apps: []config.AppConfig{{Name: "B", URL: "http://b:1", DockerKey: "name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa", DockerManagedURL: "http://b:1", Enabled: true}}}
	svc := NewService(&config.DiscoveryDockerConfig{})
	svc.MarkMissing("name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa")
	var mu sync.RWMutex
	fail := true
	p := NewPoller(PollerDeps{Config: cfg, ConfigMu: &mu, Service: svc, OnSave: func() error {
		if fail {
			return errors.New("disk full")
		}
		return nil
	}})
	b := newRefreshBatch()
	b.rekeys = map[string]string{"name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa": "swarm:b"}
	p.applyRefreshBatch(b)
	if cfg.Apps[0].DockerKey != "name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa" || svc.MissingSince("name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa").IsZero() {
		t.Fatalf("failed save: key=%q", cfg.Apps[0].DockerKey)
	}
	fail = false
	b = newRefreshBatch()
	b.rekeys = map[string]string{"name:b.1.aaaaaaaaaaaaaaaaaaaaaaaaa": "swarm:b"}
	p.applyRefreshBatch(b)
	if cfg.Apps[0].DockerKey != "swarm:b" || svc.MissingSince("swarm:b").IsZero() {
		t.Fatalf("successful save: key=%q", cfg.Apps[0].DockerKey)
	}
}

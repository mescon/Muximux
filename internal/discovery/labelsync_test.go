package discovery

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// Label re-sync of tracked apps (#500 items 1 and 2).

const lsKey = "label:sonarr-auto" // tracking key of labeledSonarr()

// manualSonarr is labeledSonarr() imported by hand: tracked, not
// auto-imported, URL in sync with the container so the refresh pass has
// nothing to write.
func manualSonarr(socket string) config.AppConfig {
	return config.AppConfig{
		Name: "Sonarr", URL: "http://10.0.0.42:8989", Enabled: true,
		Icon:  config.AppIconConfig{Type: "lucide", Name: "tv", Color: "#123456"},
		Group: "Media", Order: 1,
		DockerKey: lsKey, DockerEndpoint: "unix://" + socket,
		DockerStrategy: "container_ip", DockerManagedURL: "http://10.0.0.42:8989",
	}
}

type labelSyncFixture struct {
	set   []ContainerSummary
	cfg   *config.Config
	p     *Poller
	saves int
	fail  error
}

func newLabelSyncFixture(t *testing.T, mode config.AutoImportMode) *labelSyncFixture {
	t.Helper()
	f := &labelSyncFixture{set: []ContainerSummary{labeledSonarr()}}
	socket, cleanup := mutableDaemonForPoller(t, &f.set)
	t.Cleanup(cleanup)
	cfg, dockerCfg := autoImportCfg(socket, mode)
	cfg.Groups = []config.GroupConfig{{Name: "Media"}, {Name: "Infra"}}
	cfg.Apps = []config.AppConfig{manualSonarr(socket)}
	f.cfg = cfg
	var mu sync.RWMutex
	f.p = NewPoller(PollerDeps{
		Config: cfg, ConfigMu: &mu, Service: NewService(dockerCfg),
		OnSave: func() error { f.saves++; return f.fail },
	})
	return f
}

func (f *labelSyncFixture) label(k, v string) { f.set[0].Labels[k] = v }

func TestLabelSync_ManualImportResyncedOnNextTick(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.p.tick(context.Background())
	if f.saves != 0 {
		t.Fatalf("no label set: saves = %d, want 0", f.saves)
	}

	f.label(LabelAppName, " TV ")
	f.label(LabelAppIcon, " Traefik-Proxy ")
	f.label(LabelAppGroup, "infra")
	f.label(LabelAppOrder, " 7")
	f.p.tick(context.Background())

	a := findAppByKey(f.cfg, lsKey)
	if a == nil || a.Name != "TV" || a.Group != "Infra" || a.Order != 7 {
		t.Fatalf("labels not re-synced: %+v", a)
	}
	if a.Icon.Type != "dashboard" || a.Icon.Name != "traefik-proxy" || a.Icon.Color != "#123456" {
		t.Errorf("icon = %+v, want dashboard/traefik-proxy keeping the colour", a.Icon)
	}
	if a.DockerAutoImported || a.DockerKey != lsKey {
		t.Errorf("tracking changed: %+v", a)
	}
	if f.saves != 1 {
		t.Errorf("saves = %d, want 1", f.saves)
	}
	if err := f.cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	saveAndLoad(t, f.cfg)

	// In sync now: no further writes.
	f.p.tick(context.Background())
	if f.saves != 1 {
		t.Errorf("in-sync tick saved again: saves = %d", f.saves)
	}
}

func TestLabelSync_UnsetLabelKeepsUserValue(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.label(LabelAppOrder, "4")
	f.p.tick(context.Background())

	a := findAppByKey(f.cfg, lsKey)
	if a.Order != 4 {
		t.Fatalf("order = %d, want 4", a.Order)
	}
	if a.Name != "Sonarr" || a.Group != "Media" || a.Icon.Type != "lucide" || a.Icon.Name != "tv" {
		t.Errorf("unset labels overwrote user values: %+v", a)
	}
}

func TestLabelSync_DetachedAndHandAddedUntouched(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	// Detached: tracking cleared. Hand-added: never tracked. Same
	// container still carries labels.
	f.cfg.Apps[0].DockerKey = ""
	f.cfg.Apps[0].DockerEndpoint = ""
	f.cfg.Apps[0].DockerStrategy = ""
	f.cfg.Apps[0].DockerManagedURL = ""
	f.cfg.Apps = append(f.cfg.Apps, config.AppConfig{Name: "Hand", URL: "http://10.0.0.9", Enabled: true, Group: "Media"})
	f.label(LabelAppName, "TV")
	f.label(LabelAppGroup, "Infra")
	f.label(LabelAppOrder, "9")
	f.p.tick(context.Background())

	if f.saves != 0 {
		t.Fatalf("saves = %d, want 0", f.saves)
	}
	if f.cfg.Apps[0].Name != "Sonarr" || f.cfg.Apps[0].Group != "Media" || f.cfg.Apps[0].Order != 1 {
		t.Errorf("detached app changed: %+v", f.cfg.Apps[0])
	}
	if f.cfg.Apps[1].Name != "Hand" || f.cfg.Apps[1].Order != 0 {
		t.Errorf("hand-added app changed: %+v", f.cfg.Apps[1])
	}
}

func TestLabelSync_AutoImportOffNeverImportsOrRemoves(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	// A labelled container nobody tracks, and a tracked manual app whose
	// container is gone.
	other := labeledSonarr()
	other.ID = "radarr"
	other.Names = []string{"/radarr"}
	other.Labels = map[string]string{LabelDiscoveryID: "radarr", LabelAppName: "Radarr", LabelAppEnabled: "true"}
	f.set = append(f.set, other)
	gone := manualSonarr(strings.TrimPrefix(f.cfg.Discovery.Docker.Endpoint, "unix://"))
	gone.Name = "Gone"
	gone.DockerKey = "label:gone"
	f.cfg.Apps = append(f.cfg.Apps, gone)
	f.label(LabelAppName, "TV")

	for i := 0; i < syncRemovalGraceTicks+1; i++ {
		f.p.tick(context.Background())
	}

	if len(f.cfg.Apps) != 2 {
		t.Fatalf("apps = %+v, want the two tracked apps only", f.cfg.Apps)
	}
	if findAppByKey(f.cfg, "label:radarr") != nil {
		t.Error("auto_import off imported a container")
	}
	if a := findAppByKey(f.cfg, "label:gone"); a == nil || a.Name != "Gone" {
		t.Errorf("app of an absent container removed or changed: %+v", a)
	}
	if a := findAppByKey(f.cfg, lsKey); a == nil || a.Name != "TV" {
		t.Errorf("tracked label not re-synced with auto_import off: %+v", a)
	}
	if f.saves != 1 {
		t.Errorf("saves = %d, want 1", f.saves)
	}
}

func TestLabelSync_SaveFailureRollsBack(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.QuarantineApp(&config.AppConfig{Name: "Q", DockerKey: "label:q", DockerEndpoint: "unix:///other", DockerAutoImported: true}, "bad")
	f.fail = errors.New("disk full")
	f.label(LabelAppName, "TV")
	f.label(LabelAppGroup, "Infra")
	f.label(LabelAppIcon, "traefik")
	f.label(LabelAppOrder, "5")
	f.p.tick(context.Background())

	if f.saves != 1 {
		t.Fatalf("saves = %d, want 1 attempt", f.saves)
	}
	a := findAppByKey(f.cfg, lsKey)
	want := manualSonarr(strings.TrimPrefix(f.cfg.Discovery.Docker.Endpoint, "unix://"))
	if a.Name != want.Name || a.Group != want.Group || a.Order != want.Order || a.Icon != want.Icon {
		t.Errorf("not rolled back: %+v", a)
	}
	if q := f.cfg.Quarantined(); len(q) != 1 || q[0].Key != "label:q" {
		t.Errorf("quarantine not restored: %+v", q)
	}

	// The next good save applies it.
	f.fail = nil
	f.p.tick(context.Background())
	if a := findAppByKey(f.cfg, lsKey); a.Name != "TV" {
		t.Errorf("retry did not apply: %+v", a)
	}
}

func TestLabelSync_AutoImportedAppNoDoubleWrite(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportUpdate)
	f.cfg.Apps = nil
	f.p.tick(context.Background()) // update mode adds it
	a := findAppByKey(f.cfg, lsKey)
	if a == nil || !a.DockerAutoImported {
		t.Fatalf("auto import failed: %+v", f.cfg.Apps)
	}
	saves := f.saves

	f.label(LabelAppName, "TV")
	f.label(LabelAppOrder, "3")
	f.label(LabelAppGroup, "Infra")
	tracked := f.p.collectTracked()
	ctx := snapshotLabelSyncContext(f.cfg)
	if plan := f.p.planLabelSync(tracked.apps, f.set, f.cfg.Discovery.Docker.Endpoint, &ctx, newRefreshBatch()); len(plan) != 0 {
		t.Fatalf("label pass planned an auto-imported app: %+v", plan)
	}
	f.p.tick(context.Background())
	a = findAppByKey(f.cfg, lsKey)
	if a.Name != "TV" || a.Order != 3 || a.Group != "Infra" {
		t.Errorf("Reconcile did not re-sync the auto app: %+v", a)
	}
	if f.saves != saves+1 {
		t.Errorf("saves = %d, want exactly one more", f.saves-saves)
	}
}

func TestLabelSync_AutoImportedAppInAddModeUntouched(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportAdd)
	f.cfg.Apps[0].DockerAutoImported = true
	f.label(LabelAppName, "TV")
	f.p.tick(context.Background())
	if a := findAppByKey(f.cfg, lsKey); a.Name != "Sonarr" || f.saves != 0 {
		t.Errorf("add mode auto app changed by label pass: %+v saves=%d", a, f.saves)
	}
}

func TestLabelSync_NameTakenAndMissingGroupHeld(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Apps = append(f.cfg.Apps, config.AppConfig{Name: "tv", URL: "http://10.0.0.9", Enabled: true})
	f.label(LabelAppName, "TV")
	f.label(LabelAppGroup, "Nowhere")
	f.label(LabelAppOrder, "8")
	f.p.tick(context.Background())

	a := findAppByKey(f.cfg, lsKey)
	if a.Name != "Sonarr" || a.Group != "Media" {
		t.Errorf("held labels applied: %+v", a)
	}
	if a.Order != 8 {
		t.Errorf("order = %d, want 8 (other labels still apply)", a.Order)
	}
	held := f.p.labelHeld[lsKey]
	if !strings.Contains(held, "already used") || !strings.Contains(held, "does not match") {
		t.Errorf("held reason = %q", held)
	}

	// Freed name and created group: both apply, and the hold clears.
	f.cfg.Apps = f.cfg.Apps[:1]
	f.cfg.Groups = append(f.cfg.Groups, config.GroupConfig{Name: "Nowhere"})
	f.p.tick(context.Background())
	a = findAppByKey(f.cfg, lsKey)
	if a.Name != "TV" || a.Group != "Nowhere" {
		t.Errorf("labels not applied once possible: %+v", a)
	}
	if _, ok := f.p.labelHeld[lsKey]; ok {
		t.Error("hold not cleared")
	}
}

func TestPlanLabelSync_Rules(t *testing.T) {
	c1 := labeledSonarr()
	c1.Labels = map[string]string{LabelDiscoveryID: "a", LabelAppName: "Same"}
	c2 := labeledSonarr()
	c2.ID = "b"
	c2.Labels = map[string]string{LabelDiscoveryID: "b", LabelAppName: "Same"}
	c3 := labeledSonarr()
	c3.ID = "c"
	c3.Labels = map[string]string{LabelDiscoveryID: "c", LabelAppName: "SONARR", LabelAppIcon: "sonarr"}
	c4 := labeledSonarr()
	c4.ID = "d"
	c4.Labels = map[string]string{LabelDiscoveryID: "d", LabelAppName: strings.Repeat("x", 101)}
	c5 := labeledSonarr()
	c5.ID = "e"
	c5.Labels = map[string]string{LabelDiscoveryID: "e", LabelAppName: "Claimed"}
	containers := []ContainerSummary{c1, c2, c3, c4, c5}

	apps := []trackedAppEntry{
		{name: "B", key: "label:b", endpoint: "ep"},
		{name: "A", key: "label:a", endpoint: "ep"},
		{name: "Sonarr", key: "label:c", endpoint: "ep", icon: config.AppIconConfig{Type: "dashboard", Name: "sonarr"}},
		{name: "D", key: "label:d", endpoint: "ep"},
		{name: "E", key: "label:e", endpoint: "ep"},
		{name: "Other", key: "label:a", endpoint: "elsewhere"},
		{name: "Bad", key: "bogus", endpoint: "ep"},
		{name: "Gone", key: "label:gone", endpoint: "ep"},
	}
	cfg := &config.Config{}
	for i := range apps {
		cfg.Apps = append(cfg.Apps, config.AppConfig{Name: apps[i].name})
	}
	ctx := snapshotLabelSyncContext(cfg)
	batch := newRefreshBatch()
	batch.addApps = []config.AppConfig{{Name: "claimed"}}
	batch.updateApps = []config.AppConfig{{Name: "Updated"}}
	p := &Poller{}
	plan := p.planLabelSync(apps, containers, "ep", &ctx, batch)

	if plan["label:a"].name != "Same" {
		t.Errorf("lower key should win the shared name: %+v", plan)
	}
	if _, ok := plan["label:b"]; ok {
		t.Errorf("second rename to the same name not held: %+v", plan["label:b"])
	}
	if s := plan["label:c"]; s.name != "SONARR" || s.icon != "" {
		t.Errorf("case-only rename / synced icon: %+v", s)
	}
	if _, ok := plan["label:d"]; ok || !strings.Contains(p.labelHeld["label:d"], "100") {
		t.Errorf("over-long name not held: %+v %q", plan["label:d"], p.labelHeld["label:d"])
	}
	if _, ok := plan["label:e"]; ok {
		t.Errorf("name claimed by an auto-import add reused: %+v", plan["label:e"])
	}
	if len(plan) != 2 {
		t.Errorf("plan = %+v, want only a and c", plan)
	}

	// Held reasons log once per transition.
	before := p.labelHeld["label:b"]
	p.noteLabelHeld(&apps[0], before)
	if p.labelHeld["label:b"] != before {
		t.Error("repeat changed the hold")
	}
}

func TestResolveLabelGroup(t *testing.T) {
	groups := []string{"Media Server", "infra"}
	cases := map[string]string{"Media Server": "Media Server", "media-server": "Media Server", "INFRA": "infra"}
	for in, want := range cases {
		if got, ok := resolveLabelGroup(groups, in); !ok || got != want {
			t.Errorf("resolveLabelGroup(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"Nope", "!!!"} {
		if _, ok := resolveLabelGroup(groups, in); ok {
			t.Errorf("resolveLabelGroup(%q) matched", in)
		}
	}
}

func TestApplyLabelSyncs_UnderLock(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.GroupConfig{{Name: "Infra"}},
		Apps: []config.AppConfig{
			{Name: "Old", DockerKey: "label:m", Icon: config.AppIconConfig{Type: "custom", File: "x.png", Invert: true}},
			{Name: "Auto", DockerKey: "label:auto", DockerAutoImported: true},
			{Name: "Hand"},
			{Name: "Same", DockerKey: "label:same", Icon: config.AppIconConfig{Type: "dashboard", Name: "same"}, Group: "Infra", Order: 2},
		},
	}
	cfg.Server.GatewaySites = []config.GatewaySite{{Domain: "old.example.com", AppName: "Old"}, {Domain: "x.example.com", AppName: "Hand"}}
	got := applyLabelSyncs(cfg, map[string]labelSync{
		"label:m":    {name: "New", icon: "traefik", group: "Deleted", order: intp(3)},
		"label:auto": {name: "Nope"},
		"label:same": {name: "Same", icon: "same", group: "Infra", order: intp(2)},
	})
	a := cfg.Apps[0]
	if a.Name != "New" || a.Order != 3 || a.Group != "" {
		t.Errorf("app = %+v", a)
	}
	if a.Icon.Type != "dashboard" || a.Icon.Name != "traefik" || a.Icon.File != "" || !a.Icon.Invert {
		t.Errorf("icon = %+v", a.Icon)
	}
	if cfg.Server.GatewaySites[0].AppName != "New" || cfg.Server.GatewaySites[1].AppName != "Hand" {
		t.Errorf("site app_name cascade wrong: %+v", cfg.Server.GatewaySites)
	}
	if cfg.Apps[1].Name != "Auto" {
		t.Error("auto-imported app changed under the lock")
	}
	if len(got) != 1 || got[0].key != "label:m" || strings.Join(got[0].fields, ",") != "name,icon,order" {
		t.Errorf("synced = %+v", got)
	}
	if applyLabelSyncs(cfg, nil) != nil {
		t.Error("empty plan should return nil")
	}
}

func TestLabelSync_PruneDropsUntrackedHolds(t *testing.T) {
	p := &Poller{labelHeld: map[string]string{"label:keep": "x", "label:drop": "y"}}
	svc := NewService(&config.DiscoveryDockerConfig{})
	p.pruneTrackedState(svc, &trackedSet{apps: []trackedAppEntry{{key: "label:keep"}}})
	if _, ok := p.labelHeld["label:drop"]; ok || p.labelHeld["label:keep"] != "x" {
		t.Errorf("labelHeld = %+v", p.labelHeld)
	}
}

func TestParseAppLabels_TrimsDisplayLabels(t *testing.T) {
	got := ParseAppLabels(map[string]string{
		LabelAppName: "  My App ", LabelAppIcon: " Traefik-Proxy\t", LabelAppGroup: " Infra ", LabelAppOrder: " 12 ",
	})
	if got.Name != "My App" || got.Icon != "traefik-proxy" || got.Group != "Infra" || got.Order != 12 {
		t.Errorf("labels = %+v", got)
	}
}

func intp(v int) *int { return &v }

// Chained renames in one tick (X: N->C, A: M->N) relink each gateway site
// to the app it belonged to, not to whichever app now holds its old name.
func TestApplyLabelSyncs_ChainedRenamesRelinkSitesOnce(t *testing.T) {
	cfg := &config.Config{
		Apps: []config.AppConfig{
			{Name: "M", DockerKey: "label:a", Enabled: true},
			{Name: "N", DockerKey: "label:x", Enabled: true},
		},
	}
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "n.example.com", AppName: "N"},
		{Domain: "m.example.com", AppName: "M"},
	}
	got := applyLabelSyncs(cfg, map[string]labelSync{
		"label:x": {name: "C"},
		"label:a": {name: "N"},
	})
	if cfg.Apps[0].Name != "N" || cfg.Apps[1].Name != "C" {
		t.Fatalf("apps = %+v", cfg.Apps)
	}
	if cfg.Server.GatewaySites[0].AppName != "C" || cfg.Server.GatewaySites[1].AppName != "N" {
		t.Errorf("sites relinked wrongly: %+v", cfg.Server.GatewaySites)
	}
	if len(got) != 2 {
		t.Errorf("synced = %+v", got)
	}
}

// An app that freed its name in the plan but is skipped at apply time
// keeps that name, so the rename that took it is dropped (and any rename
// depending on that one), instead of failing validation for the tick.
func TestApplyLabelSyncs_DropsRenameOntoNameStillHeld(t *testing.T) {
	cfg := &config.Config{
		Apps: []config.AppConfig{
			{Name: "N", DockerKey: "label:x", DockerAutoImported: true, Enabled: true}, // became auto since the plan
			{Name: "M", DockerKey: "label:a", Enabled: true, Order: 1},
			{Name: "P", DockerKey: "label:b", Enabled: true},
		},
	}
	cfg.Server.GatewaySites = []config.GatewaySite{{Domain: "m.example.com", AppName: "M"}}
	got := applyLabelSyncs(cfg, map[string]labelSync{
		"label:x": {name: "C"},
		"label:a": {name: "N", order: intp(4)}, // N is still held by x
		"label:b": {name: "M"},                 // M is only free if a's rename lands
	})
	if cfg.Apps[0].Name != "N" || cfg.Apps[1].Name != "M" || cfg.Apps[2].Name != "P" {
		t.Fatalf("apps = %+v", cfg.Apps)
	}
	if cfg.Apps[1].Order != 4 {
		t.Errorf("other fields of a dropped rename must still apply: %+v", cfg.Apps[1])
	}
	if cfg.Server.GatewaySites[0].AppName != "M" {
		t.Errorf("site relinked for a dropped rename: %+v", cfg.Server.GatewaySites)
	}
	if err := config.ValidateUniqueAppSlugs(cfg.Apps); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].key != "label:a" || strings.Join(got[0].fields, ",") != "order" {
		t.Errorf("synced = %+v", got)
	}
}

func TestLabelSync_OrderZeroCanBeSet(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.label(LabelAppOrder, "0")
	f.p.tick(context.Background())
	if a := findAppByKey(f.cfg, lsKey); a.Order != 0 || f.saves != 1 {
		t.Fatalf("order = %d saves = %d, want 0 and 1", a.Order, f.saves)
	}
	f.label(LabelAppOrder, "bogus") // invalid is unset: the value stays
	f.cfg.Apps[0].Order = 6
	f.p.tick(context.Background())
	if a := findAppByKey(f.cfg, lsKey); a.Order != 6 {
		t.Errorf("invalid order label overwrote: %d", a.Order)
	}
}

// A rename followed by a failed save restores the linked gateway site too.
func TestLabelSync_SaveFailureRestoresSites(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Server.GatewaySites = []config.GatewaySite{{Domain: "sonarr.example.com", BackendURL: "http://10.0.0.42:8989", TLS: "auto", AppName: "Sonarr"}}
	f.fail = errors.New("disk full")
	f.label(LabelAppName, "TV")
	f.p.tick(context.Background())
	if f.saves != 1 {
		t.Fatalf("saves = %d, want 1 attempt", f.saves)
	}
	if a := findAppByKey(f.cfg, lsKey); a.Name != "Sonarr" {
		t.Errorf("app not rolled back: %+v", a)
	}
	if f.cfg.Server.GatewaySites[0].AppName != "Sonarr" {
		t.Errorf("site not rolled back: %+v", f.cfg.Server.GatewaySites)
	}
	f.fail = nil
	f.p.tick(context.Background())
	if f.cfg.Server.GatewaySites[0].AppName != "TV" || findAppByKey(f.cfg, lsKey).Name != "TV" {
		t.Errorf("retry: apps = %+v sites = %+v", f.cfg.Apps, f.cfg.Server.GatewaySites)
	}
}

// A re-key and a label rename in the same tick commit together: the label
// plan works on the new key, which applyRefreshBatch renames first. The
// docker state cache is taken again after the save, under the new name.
func TestLabelSync_RekeyAndRenameSameTick(t *testing.T) {
	const old = "name:bindery_web.1.71e9k1i0wfiyk5sbbjku668er"
	set := []ContainerSummary{swarmTask("bindery_web.1.newtaskidnewtaskidnewtaskid", "bindery/web",
		map[string]string{"muximux.app.port": "8080", LabelAppName: "Bindery Web", LabelAppOrder: "2"})}
	p, cfg := swarmPoller(t, &set, config.AutoImportOff)
	saves := 0
	p.deps.OnSave = func() error { saves++; return nil }
	endpoint := cfg.Discovery.Docker.Endpoint
	cfg.Apps = []config.AppConfig{{Name: "Bindery", URL: "http://old:8080", DockerKey: old, DockerEndpoint: endpoint,
		DockerStrategy: string(config.StrategyContainerDNS), DockerManagedURL: "http://old:8080", Enabled: true}}
	p.tick(context.Background())

	a := cfg.Apps[0]
	if a.DockerKey != "swarm:bindery_web" || a.Name != "Bindery Web" || a.Order != 2 || a.URL != "http://bindery_web:8080" {
		t.Fatalf("app = %+v", a)
	}
	if saves != 1 {
		t.Errorf("saves = %d, want 1", saves)
	}
	if _, ok := p.deps.Service.DockerStateSnapshot()["Bindery Web"]; !ok {
		t.Errorf("state cache not keyed by the new name: %+v", p.deps.Service.DockerStateSnapshot())
	}
}

package discovery

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// muximux.group.* labels on Docker-managed groups (#500).

func findGroup(cfg *config.Config, name string) *config.GroupConfig {
	for i := range cfg.Groups {
		if cfg.Groups[i].Name == name {
			return &cfg.Groups[i]
		}
	}
	return nil
}

func TestParseGroupLabels(t *testing.T) {
	got := ParseGroupLabels(map[string]string{
		LabelGroupIcon:  "  Plex ",
		LabelGroupColor: " #e5a00d ",
		LabelGroupOrder: " 0",
	})
	if got.Icon != "plex" || got.Color != "#e5a00d" || !got.OrderSet || got.Order != 0 || len(got.Invalid) != 0 {
		t.Errorf("valid labels = %+v", got)
	}
	if !got.any() {
		t.Error("any() = false for set labels")
	}

	bad := ParseGroupLabels(map[string]string{
		LabelGroupColor: "red",
		LabelGroupOrder: "10000",
	})
	if bad.Color != "" || bad.OrderSet || bad.any() {
		t.Errorf("invalid values applied: %+v", bad)
	}
	if len(bad.Invalid) != 2 || !strings.Contains(bad.Invalid[0], LabelGroupColor) || !strings.Contains(bad.Invalid[1], LabelGroupOrder) {
		t.Errorf("Invalid = %v, want colour then order", bad.Invalid)
	}
	if neg := ParseGroupLabels(map[string]string{LabelGroupOrder: "-1"}); neg.OrderSet || len(neg.Invalid) != 1 {
		t.Errorf("negative order = %+v", neg)
	}

	blank := ParseGroupLabels(map[string]string{LabelGroupIcon: " ", LabelGroupColor: "", LabelGroupOrder: "  "})
	if blank.any() || len(blank.Invalid) != 0 {
		t.Errorf("blank labels = %+v, want unset", blank)
	}
	if none := ParseGroupLabels(nil); none.any() {
		t.Error("nil labels set something")
	}
}

func TestParseAppLabels_GroupLabelsAreKnown(t *testing.T) {
	got := ParseAppLabels(map[string]string{LabelGroupIcon: "plex", LabelGroupColor: "#fff", LabelGroupOrder: "1"})
	if len(got.Unknown) != 0 {
		t.Errorf("group labels reported unknown: %v", got.Unknown)
	}
}

func TestTick_CreatedGroupTakesItsLabels(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportUpdate)
	f.cfg.Apps = nil
	f.label(LabelAppGroup, "Downloads")
	f.label(LabelGroupIcon, " QBittorrent ")
	f.label(LabelGroupColor, "#112233")
	f.label(LabelGroupOrder, "0")
	f.p.tick(context.Background())

	g := findGroup(f.cfg, "Downloads")
	if g == nil {
		t.Fatalf("group not created: %v", groupNames(f.cfg))
	}
	if !g.DockerManaged {
		t.Error("created group not marked docker_managed")
	}
	if g.Icon.Type != "dashboard" || g.Icon.Name != "qbittorrent" || g.Color != "#112233" || g.Order != 0 || !g.Expanded {
		t.Errorf("group = %+v, want the label values", g)
	}
	if f.saves != 1 {
		t.Errorf("saves = %d, want 1 (app, group and labels in one save)", f.saves)
	}
	loaded := saveAndLoad(t, f.cfg)
	if lg := findGroup(loaded, "Downloads"); lg == nil || !lg.DockerManaged || lg.Color != "#112233" {
		t.Errorf("marker or values not persisted: %+v", lg)
	}

	// Idempotent.
	f.p.tick(context.Background())
	if f.saves != 1 {
		t.Errorf("second tick saved again: %d", f.saves)
	}
}

// A manually imported app (auto-import off) re-syncs the labels onto its
// managed group while nothing else changes.
func TestTick_ManagedGroupResyncedFromLabels(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Groups[0].DockerManaged = true // Media, created by an import
	f.p.tick(context.Background())
	if f.saves != 0 {
		t.Fatalf("no group label: saves = %d", f.saves)
	}

	f.label(LabelGroupColor, "#abcdef")
	f.label(LabelGroupOrder, "5")
	f.p.tick(context.Background())
	g := findGroup(f.cfg, "Media")
	if g.Color != "#abcdef" || g.Order != 5 || !g.DockerManaged {
		t.Fatalf("group not re-synced: %+v", g)
	}
	if f.saves != 1 {
		t.Errorf("saves = %d, want 1", f.saves)
	}

	// Unset labels keep the stored value; a changed one re-syncs.
	delete(f.set[0].Labels, LabelGroupOrder)
	f.label(LabelGroupIcon, "plex")
	f.p.tick(context.Background())
	if g := findGroup(f.cfg, "Media"); g.Order != 5 || g.Icon.Name != "plex" || g.Icon.Type != "dashboard" {
		t.Errorf("group = %+v, want order kept and icon set", g)
	}

	// Steady state writes nothing.
	saves := f.saves
	f.p.tick(context.Background())
	if f.saves != saves {
		t.Errorf("steady tick saved %d times", f.saves-saves)
	}
}

func TestTick_UnmanagedGroupNeverTouched(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.label(LabelGroupColor, "#abcdef")
	f.label(LabelGroupIcon, "plex")
	f.p.tick(context.Background())
	if g := findGroup(f.cfg, "Media"); g.Color != "" || g.Icon.Name != "" {
		t.Errorf("operator group changed: %+v", g)
	}
	if f.saves != 0 {
		t.Errorf("saves = %d, want 0", f.saves)
	}
}

// A user edit in Settings clears the marker (handlers); later label
// changes are then ignored.
func TestTick_ReleasedGroupIgnoresLaterLabels(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Groups[0].DockerManaged = true
	f.label(LabelGroupColor, "#abcdef")
	f.p.tick(context.Background())
	if findGroup(f.cfg, "Media").Color != "#abcdef" {
		t.Fatal("setup: label not applied")
	}

	g := findGroup(f.cfg, "Media")
	g.Color, g.DockerManaged = "#000000", false // what a Settings save does
	f.label(LabelGroupColor, "#ffffff")
	saves := f.saves
	f.p.tick(context.Background())
	if g := findGroup(f.cfg, "Media"); g.Color != "#000000" || g.DockerManaged {
		t.Errorf("released group = %+v, want the user's colour", g)
	}
	if f.saves != saves {
		t.Errorf("saves = %d more, want none", f.saves-saves)
	}
}

func TestTick_GroupLabelRollbackOnSaveFailure(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Groups[0].DockerManaged = true
	f.cfg.Groups[0].Color = "#111111"
	f.label(LabelGroupColor, "#222222")
	f.fail = errors.New("disk full")
	f.p.tick(context.Background())
	if g := findGroup(f.cfg, "Media"); g.Color != "#111111" {
		t.Errorf("group change not rolled back: %+v", g)
	}

	f.fail = nil
	f.p.tick(context.Background())
	if g := findGroup(f.cfg, "Media"); g.Color != "#222222" {
		t.Errorf("retry did not apply: %+v", g)
	}
}

// A new group whose creation is rolled back takes its label values with it.
func TestTick_CreatedLabelledGroupRolledBack(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.label(LabelAppGroup, "Tools")
	f.label(LabelGroupColor, "#222222")
	f.fail = errors.New("disk full")
	f.p.tick(context.Background())
	if len(f.cfg.Groups) != 2 || findAppByKey(f.cfg, lsKey).Group != "Media" {
		t.Errorf("not rolled back: groups %v", groupNames(f.cfg))
	}
}

func TestTick_InvalidGroupLabelsIgnoredAndWarnedOnce(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Groups[0].DockerManaged = true
	f.cfg.Groups[0].Color = "#111111"
	f.label(LabelGroupColor, "blue")
	f.label(LabelGroupOrder, "x")
	f.p.tick(context.Background())
	if g := findGroup(f.cfg, "Media"); g.Color != "#111111" || g.Order != 0 {
		t.Errorf("invalid values applied: %+v", g)
	}
	if f.saves != 0 {
		t.Errorf("saves = %d", f.saves)
	}
	reason := f.p.groupLabelInvalid[lsKey]
	if !strings.Contains(reason, LabelGroupColor) || !strings.Contains(reason, LabelGroupOrder) {
		t.Errorf("invalid record = %q", reason)
	}
	f.label(LabelGroupColor, "#333333")
	f.p.tick(context.Background())
	if r := f.p.groupLabelInvalid[lsKey]; strings.Contains(r, LabelGroupColor) || !strings.Contains(r, LabelGroupOrder) {
		t.Errorf("record not updated on transition: %q", r)
	}
	if g := findGroup(f.cfg, "Media"); g.Color != "#333333" {
		t.Errorf("valid colour not applied: %+v", g)
	}
}

func TestTick_GroupLabelConflictLowestKeyWins(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Groups[0].DockerManaged = true
	// lsKey = label:sonarr-auto; label:radarr-auto sorts first.
	f.label(LabelGroupColor, "#bbbbbb")
	f.label(LabelGroupIcon, "sonarr")
	radarr := labeledSonarr()
	radarr.ID = "auto-radarr"
	radarr.Names = []string{"/radarr"}
	radarr.Labels = map[string]string{LabelDiscoveryID: "radarr-auto", LabelGroupColor: "#aaaaaa"}
	f.set = append(f.set, radarr)
	app := manualSonarr(strings.TrimPrefix(f.cfg.Apps[0].DockerEndpoint, "unix://"))
	app.Name, app.DockerKey = "Radarr", "label:radarr-auto"
	f.cfg.Apps = append(f.cfg.Apps, app)

	f.p.tick(context.Background())
	g := findGroup(f.cfg, "Media")
	if g.Color != "#aaaaaa" {
		t.Errorf("colour = %q, want the lower key's #aaaaaa", g.Color)
	}
	if g.Icon.Name != "sonarr" {
		t.Errorf("icon = %+v, want sonarr (only one container sets it)", g.Icon)
	}
	note := f.p.groupLabelConflict["Media"]
	if !strings.Contains(note, LabelGroupColor) || !strings.Contains(note, lsKey) || !strings.Contains(note, "label:radarr-auto") {
		t.Errorf("conflict record = %q", note)
	}
	if strings.Contains(note, LabelGroupIcon) {
		t.Errorf("icon reported as a conflict: %q", note)
	}

	// Stable: the next tick neither flaps nor saves.
	saves := f.saves
	f.p.tick(context.Background())
	if f.saves != saves || findGroup(f.cfg, "Media").Color != "#aaaaaa" {
		t.Errorf("conflict flapped")
	}
}

func TestGroupOwners(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "B", Group: "G", DockerKey: "k:b", DockerEndpoint: "e"},
		{Name: "A", Group: "G", DockerKey: "k:a", DockerEndpoint: "e"},
		{Name: "Other", Group: "G", DockerKey: "k:o", DockerEndpoint: "other"},
		{Name: "Ungrouped", DockerKey: "k:u", DockerEndpoint: "e"},
		{Name: "Hand", Group: "G"},
		{Name: "Gw", Group: "H"}, // gateway-routed: untracked app, tracked site
	}
	sites := []config.GatewaySite{
		{Domain: "gw.example.com", AppName: "Gw", DockerKey: "k:gw", DockerEndpoint: "e"},
		{Domain: "b.example.com", AppName: "B", DockerKey: "k:b", DockerEndpoint: "e"}, // same key as app B
		{Domain: "x.example.com", DockerKey: "k:x", DockerEndpoint: "e"},
	}
	got := groupOwners(apps, sites, "e")
	want := []groupOwner{{"k:a", "G"}, {"k:b", "G"}, {"k:gw", "H"}}
	if len(got) != len(want) {
		t.Fatalf("owners = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("owners[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestGroupLabelLookup(t *testing.T) {
	containers := []ContainerSummary{labeledSonarr()}
	containers[0].Labels[LabelGroupOrder] = "3"
	lookup := groupLabelLookup(containers)
	for i := 0; i < 2; i++ { // second call hits the cache
		if g, ok := lookup(lsKey); !ok || g.Order != 3 {
			t.Errorf("lookup = %+v,%v", g, ok)
		}
	}
	if _, ok := lookup("bogus"); ok {
		t.Error("malformed key found a container")
	}
	for i := 0; i < 2; i++ {
		if _, ok := lookup("label:nope"); ok {
			t.Error("missing container found")
		}
	}
	if syncGroupLabels(&config.Config{}, &refreshBatch{}) != nil {
		t.Error("no containers: want nil")
	}
}

func TestResolveGroupLabels_PerFieldWinner(t *testing.T) {
	labels := map[string]GroupLabels{
		"k:1": {Order: 2, OrderSet: true},
		"k:2": {Icon: "a", Color: "#ABCDEF", Order: 3, OrderSet: true},
		"k:3": {Icon: "b", Color: "#abcdef"},
		"k:4": {},
	}
	owners := []groupOwner{{"k:1", "G"}, {"k:2", "G"}, {"k:3", "G"}, {"k:4", "G"}, {"k:5", "G"}}
	vals, conflicts := resolveGroupLabels(owners, func(k string) (GroupLabels, bool) {
		g, ok := labels[k]
		return g, ok
	})
	v := vals["G"]
	if v.icon != "a" || v.iconKey != "k:2" || v.color != "#ABCDEF" || *v.order != 2 || v.orderKey != "k:1" {
		t.Errorf("values = %+v", v)
	}
	// k:2 loses order to k:1, k:3 loses icon to k:2; a colour that only
	// differs in case is no conflict.
	if len(conflicts) != 2 ||
		conflicts[0] != (groupLabelConflict{"G", LabelGroupOrder, "k:2", "k:1"}) ||
		conflicts[1] != (groupLabelConflict{"G", LabelGroupIcon, "k:3", "k:2"}) {
		t.Errorf("conflicts = %+v", conflicts)
	}
}

func TestApplyGroupLabelValues(t *testing.T) {
	order := 4
	groups := []config.GroupConfig{
		{Name: "M", DockerManaged: true, Icon: config.AppIconConfig{Type: "custom", File: "x.png", Background: "#000"}},
		{Name: "U", Color: "#fff"},
	}
	vals := map[string]groupLabelValues{
		"M": {icon: "plex", color: "#123", order: &order},
		"U": {color: "#000"},
	}
	got := applyGroupLabelValues(groups, vals)
	if len(got) != 1 || got[0].name != "M" || strings.Join(got[0].fields, ",") != "icon,color,order" {
		t.Errorf("synced = %+v", got)
	}
	if ic := groups[0].Icon; ic.Type != "dashboard" || ic.Name != "plex" || ic.File != "" || ic.Background != "#000" {
		t.Errorf("icon = %+v, want dashboard/plex keeping the background", ic)
	}
	if groups[1].Color != "#fff" {
		t.Error("unmanaged group changed")
	}
	if again := applyGroupLabelValues(groups, vals); len(again) != 0 {
		t.Errorf("second apply changed %+v", again)
	}
}

func TestSuggestion_GroupLabelNotes(t *testing.T) {
	c := labeledSonarr()
	c.Labels[LabelGroupColor] = "nope"
	s := suggestForContainer(&c, config.StrategyContainerIP, "", "")
	if !hasNoteContaining(s.Notes, "Group label ignored: "+LabelGroupColor) {
		t.Errorf("notes = %v, want the invalid colour", s.Notes)
	}

	plain := ContainerSummary{
		ID: "x", Names: []string{"/thing"}, Image: "example/unknown",
		Labels: map[string]string{LabelGroupIcon: "plex"},
		Ports:  []ContainerPort{{PrivatePort: 80, Type: "tcp"}},
	}
	s = suggestForContainer(&plain, config.StrategyHostPort, "10.0.0.1", "")
	if s.Group != "" || !hasNoteContaining(s.Notes, "the app has no group") {
		t.Errorf("group %q notes = %v, want the no-group note", s.Group, s.Notes)
	}
	for _, n := range s.Notes {
		if strings.Contains(n, "Unknown label") {
			t.Errorf("group label reported unknown: %q", n)
		}
	}
}

func TestNoteGroupLabelConflicts(t *testing.T) {
	sug := []Suggestion{
		{Key: "label:z", Group: "media", groupLabels: GroupLabels{Color: "#222222", Icon: "plex"}},
		{Key: "label:a", Group: "Media", groupLabels: GroupLabels{Color: "#111111"}},
		{Key: "label:m", Group: "Other", groupLabels: GroupLabels{Color: "#333333"}},
		{Key: "label:n", Group: "", groupLabels: GroupLabels{Color: "#444444"}},
	}
	noteGroupLabelConflicts(sug)
	if !hasNoteContaining(sug[0].Notes, LabelGroupColor+" ignored: label:a") {
		t.Errorf("loser notes = %v", sug[0].Notes)
	}
	for i := 1; i < len(sug); i++ {
		if len(sug[i].Notes) != 0 {
			t.Errorf("suggestion %s got notes %v", sug[i].Key, sug[i].Notes)
		}
	}
}

func hasNoteContaining(notes []string, sub string) bool {
	for _, n := range notes {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

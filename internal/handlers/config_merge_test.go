package handlers

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

func testApp(name, url string) ClientAppConfig {
	return ClientAppConfig{Name: name, URL: url, Enabled: true, Icon: config.AppIconConfig{Type: "dashboard", Name: "x"}}
}

func TestMergeFields_MineWinsOnlyWhereChanged(t *testing.T) {
	base := config.NavigationConfig{Position: "top", Width: "220px", ShowLabels: true}
	mine := base
	mine.Position = "left"
	theirs := base
	theirs.Width = "300px"
	theirs.ShowLabels = false
	got := mergeFields(&base, &mine, &theirs, nil)
	if got.Position != "left" || got.Width != "300px" || got.ShowLabels {
		t.Errorf("got %+v", got)
	}
}

func TestMergeThreeWay_ConflictMineWins(t *testing.T) { // Review Focus 1
	base := &ClientConfigUpdate{Title: "A", Language: "en"}
	mine := &ClientConfigUpdate{Title: "B", Language: "en"}
	theirs := &ClientConfigUpdate{Title: "C", Language: "sv"}
	got := mergeThreeWay(base, mine, theirs)
	if got.Title != "B" || got.Language != "sv" || got.Base != nil {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_UntouchedURLKeepsTheirs(t *testing.T) { // S-04
	base := []ClientAppConfig{testApp("Whoami", "http://172.17.0.2")}
	mine := []ClientAppConfig{testApp("Whoami", "http://172.17.0.2")}
	mine[0].Color = "#fff"
	theirs := []ClientAppConfig{testApp("Whoami", "http://172.17.0.9")}
	theirs[0].DockerKey = "k1"
	got := mergeApps(base, mine, theirs)
	if len(got) != 1 || got[0].URL != "http://172.17.0.9" || got[0].Color != "#fff" || got[0].DockerKey != "k1" || got[0].OriginalName != "Whoami" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_EditedURLWins(t *testing.T) {
	base := []ClientAppConfig{testApp("W", "http://a")}
	mine := []ClientAppConfig{testApp("W", "http://manual")}
	theirs := []ClientAppConfig{testApp("W", "http://b")}
	if got := mergeApps(base, mine, theirs); got[0].URL != "http://manual" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_ServerAddedKept_UserDeletedDropped(t *testing.T) { // S-05, S-11
	base := []ClientAppConfig{testApp("Keep", "u"), testApp("Gone", "u")}
	mine := []ClientAppConfig{testApp("Keep", "u")}
	theirs := []ClientAppConfig{testApp("Keep", "u"), testApp("Gone", "u"), testApp("New", "u")}
	got := mergeApps(base, mine, theirs)
	names := []string{}
	for i := range got {
		names = append(names, got[i].Name)
	}
	if strings.Join(names, ",") != "Keep,New" {
		t.Errorf("names = %v", names)
	}
}

func TestMergeApps_ServerRemovedDroppedUnlessEdited(t *testing.T) { // S-05 sync removal
	base := []ClientAppConfig{testApp("Old", "u"), testApp("Edited", "u")}
	mine := []ClientAppConfig{testApp("Old", "u"), testApp("Edited", "u2")}
	mine[1].DockerKey = "stale"
	got := mergeApps(base, mine, nil)
	if len(got) != 1 || got[0].Name != "Edited" || got[0].URL != "u2" || got[0].DockerKey != "" || got[0].OriginalName != "" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_EmptyVsNilIsUnchanged(t *testing.T) { // nil vs empty must not read as an edit
	base := []ClientAppConfig{testApp("Gone", "u")}
	mine := []ClientAppConfig{testApp("Gone", "u")}
	mine[0].AllowedGroups = []string{}
	mine[0].Permissions = []string{}
	mine[0].ProxyHeaders = map[string]string{}
	if got := mergeApps(base, mine, nil); len(got) != 0 {
		t.Errorf("untouched app with empty slices counted as edited: %+v", got)
	}
	theirs := []ClientAppConfig{testApp("Gone", "u")}
	theirs[0].AllowedGroups = []string{"ops"}
	got := mergeApps(base, mine, theirs)
	if len(got) != 1 || len(got[0].AllowedGroups) != 1 || got[0].AllowedGroups[0] != "ops" {
		t.Errorf("empty mine overrode theirs: %+v", got)
	}
}

func TestMergeApps_RenameWithServerURLRefresh(t *testing.T) { // Review Focus 2, S-12
	base := []ClientAppConfig{testApp("Old", "http://a")}
	mine := []ClientAppConfig{testApp("New", "http://a")}
	mine[0].OriginalName = "Old"
	theirs := []ClientAppConfig{testApp("Old", "http://b")}
	theirs[0].DockerKey = "k"
	got := mergeApps(base, mine, theirs)
	if got[0].Name != "New" || got[0].URL != "http://b" || got[0].OriginalName != "Old" || got[0].DockerKey != "k" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_ServerRenameMatchedByDockerKey(t *testing.T) {
	base := []ClientAppConfig{testApp("whoami", "u")}
	base[0].DockerKey = "k"
	mine := []ClientAppConfig{testApp("whoami", "u")}
	mine[0].Pinned = true
	theirs := []ClientAppConfig{testApp("Whoami Pretty", "u")}
	theirs[0].DockerKey = "k"
	got := mergeApps(base, mine, theirs)
	if len(got) != 1 || got[0].Name != "Whoami Pretty" || !got[0].Pinned || got[0].OriginalName != "Whoami Pretty" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_DeleteBeatsServerUpdate(t *testing.T) { // Review Focus 3
	base := []ClientAppConfig{testApp("X", "http://a")}
	theirs := []ClientAppConfig{testApp("X", "http://b")}
	if got := mergeApps(base, nil, theirs); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_NewInMineStripsTracking(t *testing.T) {
	mine := []ClientAppConfig{testApp("Brand", "u")}
	mine[0].DockerKey = "forged"
	got := mergeApps(nil, mine, nil)
	if got[0].DockerKey != "" {
		t.Errorf("tracking accepted from a new app: %+v", got[0])
	}
}

func TestMergeGroups_RenameKeepsTheirsNameAsOriginal(t *testing.T) {
	base := []config.GroupConfig{{Name: "Media", Color: "#111"}}
	mine := []config.GroupConfig{{Name: "Video", OriginalName: "Media", Color: "#111"}}
	theirs := []config.GroupConfig{{Name: "Media", Color: "#222"}}
	got := mergeGroups(base, mine, theirs)
	if len(got) != 1 || got[0].Name != "Video" || got[0].OriginalName != "Media" || got[0].Color != "#222" {
		t.Errorf("got %+v", got)
	}
}

func TestCascadeGroupRenames_RepointsApps(t *testing.T) {
	groups := []config.GroupConfig{{Name: "Video", OriginalName: "Media"}}
	apps := []ClientAppConfig{testApp("A", "u"), testApp("B", "u")}
	apps[0].Group = "Media"
	apps[1].Group = "Other"
	cascadeGroupRenames(groups, apps)
	if apps[0].Group != "Video" || apps[1].Group != "Other" {
		t.Errorf("apps = %+v", apps)
	}
}

func TestMergeKeybindings_PerAction(t *testing.T) {
	combo := func(k string) []config.KeyCombo { return []config.KeyCombo{{Key: k}} }
	base := &config.KeybindingsConfig{Bindings: map[string][]config.KeyCombo{"search": combo("a"), "logs": combo("l")}}
	mine := &config.KeybindingsConfig{Bindings: map[string][]config.KeyCombo{"search": combo("b")}} // logs deleted
	theirs := &config.KeybindingsConfig{Bindings: map[string][]config.KeyCombo{"search": combo("a"), "logs": combo("l"), "home": combo("h")}}
	got := mergeKeybindings(base, mine, theirs)
	if got.Bindings["search"][0].Key != "b" || got.Bindings["home"][0].Key != "h" {
		t.Errorf("got %+v", got.Bindings)
	}
	if _, ok := got.Bindings["logs"]; ok {
		t.Error("deleted binding resurrected")
	}
}

func TestMergeHealth_NilMineKeepsTheirs(t *testing.T) {
	theirs := &config.HealthConfig{Enabled: true, Interval: "2m"}
	if got := mergeHealth(nil, nil, theirs); got == nil || got.Interval != "2m" {
		t.Errorf("got %+v", got)
	}
	base := &config.HealthConfig{Enabled: true, Interval: "30s"}
	mine := &config.HealthConfig{Enabled: false, Interval: "30s"}
	if got := mergeHealth(base, mine, theirs); got.Enabled || got.Interval != "2m" {
		t.Errorf("got %+v", got)
	}
}

// Every field of ClientConfigUpdate except Base must be copied from the
// response, so a field added later cannot be silently dropped by the merge.
func TestUpdateFromResponse_CopiesEveryMergedField(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Title = "T"
	cfg.Server.SessionCookieDomain = ".x.org"
	cfg.Server.ProxyTimeout = "45s"
	cfg.Apps = []config.AppConfig{{Name: "A", URL: "u", Enabled: true}}
	cfg.Groups = []config.GroupConfig{{Name: "G"}}
	resp := buildClientConfigResponse(cfg, auth.RoleAdmin, nil)
	got := updateFromResponse(&resp)
	gv := reflect.ValueOf(got).Elem()
	rv := reflect.ValueOf(resp)
	for i := 0; i < gv.NumField(); i++ {
		name := gv.Type().Field(i).Name
		if name == "Base" {
			continue
		}
		rf := rv.FieldByName(name)
		if !rf.IsValid() {
			t.Errorf("clientConfigResponse has no field %s", name)
			continue
		}
		if !jsonEqual(gv.Field(i).Interface(), rf.Interface()) {
			t.Errorf("%s not copied", name)
		}
	}
}

// --- Additional coverage for branches the brief's cases do not reach. ---

func TestIdentity_FallsBackToName(t *testing.T) {
	a := testApp("A", "u")
	if appIdentity(&a) != "A" {
		t.Error("app identity without OriginalName")
	}
	a.OriginalName = "Old"
	if appIdentity(&a) != "Old" {
		t.Error("app identity with OriginalName")
	}
	g := config.GroupConfig{Name: "G"}
	if groupIdentity(&g) != "G" {
		t.Error("group identity without OriginalName")
	}
	g.OriginalName = "OldG"
	if groupIdentity(&g) != "OldG" {
		t.Error("group identity with OriginalName")
	}
}

func TestJSONEqual_WireForm(t *testing.T) {
	cases := []struct {
		name string
		a, b any
		want bool
	}{
		{"nil vs empty slice", []string(nil), []string{}, true},
		{"nil vs empty map", map[string]string(nil), map[string]string{}, true},
		{"untyped nil vs empty slice", nil, []int{}, true},
		{"differing slices", []string{"a"}, []string{"b"}, false},
		{"equal structs", testApp("A", "u"), testApp("A", "u"), true},
		{"differing structs", testApp("A", "u"), testApp("A", "v"), false},
		{"unmarshalable values fall back to Go syntax", math.NaN(), math.Inf(1), false},
		{"same unmarshalable value", func() any { var c chan int; return c }(), func() any { var c chan int; return c }(), true},
	}
	for _, tc := range cases {
		if got := jsonEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: jsonEqual = %v, want %v", tc.name, got, tc.want)
		}
	}
}

type mergeFieldsProbe struct {
	Public  string
	Skipped string
	hidden  string
}

func TestMergeFields_SkipAndUnexported(t *testing.T) {
	base := mergeFieldsProbe{Public: "b", Skipped: "b", hidden: "b"}
	mine := mergeFieldsProbe{Public: "m", Skipped: "m", hidden: "m"}
	theirs := mergeFieldsProbe{Public: "t", Skipped: "t", hidden: "t"}
	got := mergeFields(&base, &mine, &theirs, map[string]bool{"Skipped": true})
	if got.Public != "m" || got.Skipped != "t" || got.hidden != "t" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeHealth_MissingSideTakesMine(t *testing.T) {
	mine := &config.HealthConfig{Enabled: true, Interval: "10s"}
	if got := mergeHealth(nil, mine, &config.HealthConfig{Interval: "1m"}); got != mine {
		t.Errorf("nil base: got %+v", got)
	}
	if got := mergeHealth(&config.HealthConfig{}, mine, nil); got != mine {
		t.Errorf("nil theirs: got %+v", got)
	}
}

func TestMergeKeybindings_NilSides(t *testing.T) {
	theirs := &config.KeybindingsConfig{Bindings: map[string][]config.KeyCombo{"x": {{Key: "x"}}}}
	if got := mergeKeybindings(nil, nil, theirs); got != theirs {
		t.Errorf("nil mine: got %+v", got)
	}
	mine := &config.KeybindingsConfig{Bindings: map[string][]config.KeyCombo{"y": {{Key: "y"}}}}
	got := mergeKeybindings(nil, mine, nil)
	if len(got.Bindings) != 1 || got.Bindings["y"][0].Key != "y" {
		t.Errorf("nil base and theirs: got %+v", got.Bindings)
	}
}

func TestMergeApps_NewInMineCollidesWithServerAdded(t *testing.T) {
	mine := []ClientAppConfig{testApp("Same", "http://mine")}
	mine[0].DockerKey = "forged"
	theirs := []ClientAppConfig{testApp("Same", "http://theirs")}
	theirs[0].DockerKey = "real"
	theirs[0].Color = "#abc"
	got := mergeApps(nil, mine, theirs)
	if len(got) != 1 {
		t.Fatalf("collision duplicated: %+v", got)
	}
	if got[0].URL != "http://mine" || got[0].Color != "#abc" || got[0].DockerKey != "real" || got[0].OriginalName != "Same" {
		t.Errorf("got %+v", got[0])
	}
}

func TestMergeApps_UserDeletedServerRenamedByKey(t *testing.T) {
	base := []ClientAppConfig{testApp("whoami", "u")}
	base[0].DockerKey = "k"
	theirs := []ClientAppConfig{testApp("Whoami Pretty", "u")}
	theirs[0].DockerKey = "k"
	if got := mergeApps(base, nil, theirs); len(got) != 0 {
		t.Errorf("user-deleted app resurrected under its server rename: %+v", got)
	}
}

func TestMergeApps_BaseKeyWithoutServerMatchIsRemoved(t *testing.T) {
	base := []ClientAppConfig{testApp("tracked", "u")}
	base[0].DockerKey = "gone"
	mine := []ClientAppConfig{testApp("tracked", "u")}
	mine[0].DockerKey = "gone"
	if got := mergeApps(base, mine, nil); len(got) != 0 {
		t.Errorf("server-removed untouched app kept: %+v", got)
	}
}

func TestMergeGroups_AllBranches(t *testing.T) {
	base := []config.GroupConfig{
		{Name: "Keep", Color: "#1"},
		{Name: "UserDeleted"},
		{Name: "ServerRemoved"},
		{Name: "ServerRemovedEdited", Color: "#1"},
	}
	mine := []config.GroupConfig{
		{Name: "Keep", Color: "#1", Order: 5},
		{Name: "ServerRemoved"},
		{Name: "ServerRemovedEdited", OriginalName: "ServerRemovedEdited", Color: "#2"},
		{Name: "Brand", OriginalName: "bogus-but-unknown"},
		{Name: "Collide", Color: "#mine"},
	}
	theirs := []config.GroupConfig{
		{Name: "Keep", Color: "#9"},
		{Name: "UserDeleted"},
		{Name: "Collide", Color: "#theirs", Expanded: true},
		{Name: "ServerAdded"},
	}
	got := mergeGroups(base, mine, theirs)
	byName := map[string]config.GroupConfig{}
	names := []string{}
	for i := range got {
		byName[got[i].Name] = got[i]
		names = append(names, got[i].Name)
	}
	if strings.Join(names, ",") != "Keep,ServerRemovedEdited,Brand,Collide,ServerAdded" {
		t.Fatalf("names = %v", names)
	}
	if g := byName["Keep"]; g.Color != "#9" || g.Order != 5 || g.OriginalName != "Keep" {
		t.Errorf("Keep = %+v", g)
	}
	if g := byName["ServerRemovedEdited"]; g.Color != "#2" || g.OriginalName != "" {
		t.Errorf("ServerRemovedEdited = %+v", g)
	}
	if g := byName["Brand"]; g.OriginalName != "" {
		t.Errorf("Brand = %+v", g)
	}
	if g := byName["Collide"]; g.Color != "#mine" || !g.Expanded || g.OriginalName != "Collide" {
		t.Errorf("Collide = %+v", g)
	}
	if g := byName["ServerAdded"]; g.OriginalName != "ServerAdded" {
		t.Errorf("ServerAdded = %+v", g)
	}
}

func TestCascadeGroupRenames_IgnoresUnrenamed(t *testing.T) {
	groups := []config.GroupConfig{{Name: "Same", OriginalName: "Same"}, {Name: "Plain"}}
	apps := []ClientAppConfig{testApp("A", "u")}
	apps[0].Group = "Same"
	cascadeGroupRenames(groups, apps)
	if apps[0].Group != "Same" {
		t.Errorf("apps = %+v", apps)
	}
}

func TestMergeThreeWay_NestedSections(t *testing.T) {
	base := &ClientConfigUpdate{
		Title:      "A",
		Navigation: config.NavigationConfig{Position: "top", Width: "200px"},
		Theme:      config.ThemeConfig{Family: "default", Variant: "dark"},
		Groups:     []config.GroupConfig{{Name: "G"}},
		Apps:       []ClientAppConfig{testApp("X", "u")},
	}
	mine := &ClientConfigUpdate{
		Title:      "A",
		Navigation: config.NavigationConfig{Position: "left", Width: "200px"},
		Theme:      config.ThemeConfig{Family: "default", Variant: "dark"},
		Groups:     []config.GroupConfig{{Name: "G2", OriginalName: "G"}},
		Apps:       []ClientAppConfig{testApp("X", "u")},
		Health:     &config.HealthConfig{Enabled: true},
	}
	mine.Base = base
	theirs := &ClientConfigUpdate{
		Title:      "A",
		Navigation: config.NavigationConfig{Position: "top", Width: "300px"},
		Theme:      config.ThemeConfig{Family: "nord", Variant: "dark"},
		Groups:     []config.GroupConfig{{Name: "G"}},
		Apps:       []ClientAppConfig{testApp("X", "u2"), testApp("Y", "u")},
	}
	got := mergeThreeWay(base, mine, theirs)
	if got.Base != nil {
		t.Error("Base leaked into the merge result")
	}
	if got.Navigation.Position != "left" || got.Navigation.Width != "300px" {
		t.Errorf("navigation = %+v", got.Navigation)
	}
	if got.Theme.Family != "nord" {
		t.Errorf("theme = %+v", got.Theme)
	}
	if got.Health == nil || !got.Health.Enabled {
		t.Errorf("health = %+v", got.Health)
	}
	if len(got.Groups) != 1 || got.Groups[0].Name != "G2" || got.Groups[0].OriginalName != "G" {
		t.Errorf("groups = %+v", got.Groups)
	}
	if len(got.Apps) != 2 || got.Apps[0].URL != "u2" || got.Apps[1].Name != "Y" {
		t.Errorf("apps = %+v", got.Apps)
	}
}

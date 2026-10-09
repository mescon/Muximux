package handlers

import (
	"errors"
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
	got := mustMergeThreeWay(t, base, mine, theirs)
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
	got := mustMergeApps(t, base, mine, theirs)
	if len(got) != 1 || got[0].URL != "http://172.17.0.9" || got[0].Color != "#fff" || got[0].DockerKey != "k1" || got[0].OriginalName != "Whoami" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_EditedURLWins(t *testing.T) {
	base := []ClientAppConfig{testApp("W", "http://a")}
	mine := []ClientAppConfig{testApp("W", "http://manual")}
	theirs := []ClientAppConfig{testApp("W", "http://b")}
	if got := mustMergeApps(t, base, mine, theirs); got[0].URL != "http://manual" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_ServerAddedKept_UserDeletedDropped(t *testing.T) { // S-05, S-11
	base := []ClientAppConfig{testApp("Keep", "u"), testApp("Gone", "u")}
	mine := []ClientAppConfig{testApp("Keep", "u")}
	theirs := []ClientAppConfig{testApp("Keep", "u"), testApp("Gone", "u"), testApp("New", "u")}
	got := mustMergeApps(t, base, mine, theirs)
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
	got := mustMergeApps(t, base, mine, nil)
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
	if got := mustMergeApps(t, base, mine, nil); len(got) != 0 {
		t.Errorf("untouched app with empty slices counted as edited: %+v", got)
	}
	theirs := []ClientAppConfig{testApp("Gone", "u")}
	theirs[0].AllowedGroups = []string{"ops"}
	got := mustMergeApps(t, base, mine, theirs)
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
	got := mustMergeApps(t, base, mine, theirs)
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
	got := mustMergeApps(t, base, mine, theirs)
	if len(got) != 1 || got[0].Name != "Whoami Pretty" || !got[0].Pinned || got[0].OriginalName != "Whoami Pretty" {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_DeleteBeatsServerUpdate(t *testing.T) { // Review Focus 3
	base := []ClientAppConfig{testApp("X", "http://a")}
	theirs := []ClientAppConfig{testApp("X", "http://b")}
	if got := mustMergeApps(t, base, nil, theirs); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestMergeApps_NewInMineStripsTracking(t *testing.T) {
	mine := []ClientAppConfig{testApp("Brand", "u")}
	mine[0].DockerKey = "forged"
	got := mustMergeApps(t, nil, mine, nil)
	if got[0].DockerKey != "" {
		t.Errorf("tracking accepted from a new app: %+v", got[0])
	}
}

func TestMergeGroups_RenameKeepsTheirsNameAsOriginal(t *testing.T) {
	base := []config.GroupConfig{{Name: "Media", Color: "#111"}}
	mine := []config.GroupConfig{{Name: "Video", OriginalName: "Media", Color: "#111"}}
	theirs := []config.GroupConfig{{Name: "Media", Color: "#222"}}
	got := mustMergeGroups(t, base, mine, theirs)
	if len(got) != 1 || got[0].Name != "Video" || got[0].OriginalName != "Media" || got[0].Color != "#222" {
		t.Errorf("got %+v", got)
	}
}

func TestCascadeGroupRenames_RepointsApps(t *testing.T) {
	groups := []config.GroupConfig{{Name: "Video", OriginalName: "Media"}}
	apps := []ClientAppConfig{testApp("A", "u"), testApp("B", "u")}
	apps[0].Group = "Media"
	apps[1].Group = "Other"
	cascadeGroupRenames(groups, apps, neverIn)
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

func TestMergeApps_UserDeletedServerRenamedByKey(t *testing.T) {
	base := []ClientAppConfig{testApp("whoami", "u")}
	base[0].DockerKey = "k"
	theirs := []ClientAppConfig{testApp("Whoami Pretty", "u")}
	theirs[0].DockerKey = "k"
	if got := mustMergeApps(t, base, nil, theirs); len(got) != 0 {
		t.Errorf("user-deleted app resurrected under its server rename: %+v", got)
	}
}

func TestMergeApps_BaseKeyWithoutServerMatchIsRemoved(t *testing.T) {
	base := []ClientAppConfig{testApp("tracked", "u")}
	base[0].DockerKey = "gone"
	mine := []ClientAppConfig{testApp("tracked", "u")}
	mine[0].DockerKey = "gone"
	if got := mustMergeApps(t, base, mine, nil); len(got) != 0 {
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
	}
	theirs := []config.GroupConfig{
		{Name: "Keep", Color: "#9"},
		{Name: "UserDeleted"},
		{Name: "ServerAdded"},
	}
	got := mustMergeGroups(t, base, mine, theirs)
	byName := map[string]config.GroupConfig{}
	names := []string{}
	for i := range got {
		byName[got[i].Name] = got[i]
		names = append(names, got[i].Name)
	}
	if strings.Join(names, ",") != "Keep,ServerRemovedEdited,Brand,ServerAdded" {
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
	if g := byName["ServerAdded"]; g.OriginalName != "ServerAdded" {
		t.Errorf("ServerAdded = %+v", g)
	}
}

func TestCascadeGroupRenames_IgnoresUnrenamed(t *testing.T) {
	groups := []config.GroupConfig{{Name: "Same", OriginalName: "Same"}, {Name: "Plain"}}
	apps := []ClientAppConfig{testApp("A", "u")}
	apps[0].Group = "Same"
	cascadeGroupRenames(groups, apps, neverIn)
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
	got := mustMergeThreeWay(t, base, mine, theirs)
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

// --- Fix round 1: helpers and claim, conflict and dangling-group cases. ---

func mustMergeApps(t *testing.T, base, mine, theirs []ClientAppConfig) []ClientAppConfig {
	t.Helper()
	got, err := mergeApps(base, mine, theirs)
	if err != nil {
		t.Fatalf("mergeApps: %v", err)
	}
	return got
}

func mustMergeGroups(t *testing.T, base, mine, theirs []config.GroupConfig) []config.GroupConfig {
	t.Helper()
	got, err := mergeGroups(base, mine, theirs)
	if err != nil {
		t.Fatalf("mergeGroups: %v", err)
	}
	return got
}

func mustMergeThreeWay(t *testing.T, base, mine, theirs *ClientConfigUpdate) *ClientConfigUpdate {
	t.Helper()
	got, err := mergeThreeWay(base, mine, theirs)
	if err != nil {
		t.Fatalf("mergeThreeWay: %v", err)
	}
	return got
}

func asConflict(t *testing.T, err error) *MergeConflictError {
	t.Helper()
	var ce *MergeConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("want *MergeConflictError, got %v", err)
	}
	return ce
}

func TestMergeApps_RenameClaimsBeforeNameMatch(t *testing.T) {
	renamed := testApp("Y", "http://mine")
	renamed.OriginalName = "X"
	fresh := testApp("X", "http://fresh")
	fresh.DockerKey = "forged"
	orders := map[string][]ClientAppConfig{
		"rename first": {renamed, fresh},
		"rename last":  {fresh, renamed},
	}
	for label, mine := range orders {
		base := []ClientAppConfig{testApp("X", "http://a")}
		theirs := []ClientAppConfig{testApp("X", "http://a")}
		theirs[0].DockerKey = "k"
		theirs[0].Color = "#srv"
		got := mustMergeApps(t, base, mine, theirs)
		byName := map[string]ClientAppConfig{}
		for i := range got {
			byName[got[i].Name] = got[i]
		}
		if len(got) != 2 {
			t.Fatalf("%s: got %+v", label, got)
		}
		y := byName["Y"]
		if y.OriginalName != "X" || y.DockerKey != "k" || y.Color != "#srv" || y.URL != "http://mine" {
			t.Errorf("%s: Y not merged against server X: %+v", label, y)
		}
		x := byName["X"]
		if x.OriginalName != "" || x.DockerKey != "" || x.URL != "http://fresh" {
			t.Errorf("%s: new X not treated as new: %+v", label, x)
		}
	}
}

func TestMergeGroups_RenameClaimsBeforeNameMatch(t *testing.T) {
	renamed := config.GroupConfig{Name: "Y", OriginalName: "X", Color: "#1"}
	fresh := config.GroupConfig{Name: "X", Color: "#new"}
	for label, mine := range map[string][]config.GroupConfig{
		"rename first": {renamed, fresh},
		"rename last":  {fresh, renamed},
	} {
		base := []config.GroupConfig{{Name: "X", Color: "#1"}}
		theirs := []config.GroupConfig{{Name: "X", Color: "#1", Expanded: true}}
		got := mustMergeGroups(t, base, mine, theirs)
		byName := map[string]config.GroupConfig{}
		for i := range got {
			byName[got[i].Name] = got[i]
		}
		if len(got) != 2 {
			t.Fatalf("%s: got %+v", label, got)
		}
		if y := byName["Y"]; y.OriginalName != "X" || !y.Expanded {
			t.Errorf("%s: Y = %+v", label, y)
		}
		if x := byName["X"]; x.OriginalName != "" || x.Color != "#new" || x.Expanded {
			t.Errorf("%s: X = %+v", label, x)
		}
	}
}

func TestMergeApps_RenameOntoServerAddedNameConflicts(t *testing.T) {
	base := []ClientAppConfig{testApp("A", "u")}
	mine := []ClientAppConfig{testApp("B", "u")}
	mine[0].OriginalName = "A"
	theirs := []ClientAppConfig{testApp("A", "u"), testApp("B", "u")}
	_, err := mergeApps(base, mine, theirs)
	ce := asConflict(t, err)
	if ce.Kind != "app" || ce.Name != "B" || ce.RenamedFrom != "A" {
		t.Errorf("conflict = %+v", ce)
	}
	want := `an app named "B" was added on the server while this save renamed "A" to "B"; reload Settings and try again`
	if err.Error() != want {
		t.Errorf("message = %q", err.Error())
	}
}

func TestMergeGroups_RenameOntoServerAddedNameConflicts(t *testing.T) {
	base := []config.GroupConfig{{Name: "A"}}
	mine := []config.GroupConfig{{Name: "B", OriginalName: "A"}}
	theirs := []config.GroupConfig{{Name: "A"}, {Name: "B"}}
	_, err := mergeGroups(base, mine, theirs)
	ce := asConflict(t, err)
	if ce.Kind != "group" || ce.Name != "B" || ce.RenamedFrom != "A" {
		t.Errorf("conflict = %+v", ce)
	}
}

func TestMergeApps_BothSidesAddSameNameConflicts(t *testing.T) {
	// Both sides created "Same" independently. Merging the payload against
	// an empty base would let the server's values win wherever the payload
	// left a zero value, so the merge refuses instead.
	mine := []ClientAppConfig{testApp("Same", "http://mine")}
	theirs := []ClientAppConfig{testApp("Same", "http://theirs")}
	_, err := mergeApps(nil, mine, theirs)
	ce := asConflict(t, err)
	if ce.Kind != "app" || ce.Name != "Same" || ce.RenamedFrom != "" {
		t.Errorf("conflict = %+v", ce)
	}
	want := `an app named "Same" was added on the server while this save also added one; reload Settings and try again`
	if err.Error() != want {
		t.Errorf("message = %q", err.Error())
	}
}

func TestMergeGroups_BothSidesAddSameNameConflicts(t *testing.T) {
	_, err := mergeGroups(nil, []config.GroupConfig{{Name: "G"}}, []config.GroupConfig{{Name: "G"}})
	if ce := asConflict(t, err); ce.Kind != "group" || ce.Name != "G" {
		t.Errorf("conflict = %+v", ce)
	}
	if want := `a group named "G" was added on the server while this save also added one; reload Settings and try again`; err.Error() != want {
		t.Errorf("message = %q", err.Error())
	}
}

func TestMergeApps_DuplicateInPayloadConflicts(t *testing.T) {
	// Two brand-new payload apps of one name reach the final uniqueness
	// check with no rename on either side.
	mine := []ClientAppConfig{testApp("Dup", "u"), testApp("Dup", "v")}
	_, err := mergeApps(nil, mine, nil)
	if ce := asConflict(t, err); ce.Name != "Dup" || ce.RenamedFrom != "" {
		t.Errorf("conflict = %+v", ce)
	}
}

func TestCheckUniqueNames_RenameOnFirstOccurrence(t *testing.T) {
	err := checkUniqueNames("app", []string{"B", "B"}, []string{"A", ""})
	if ce := asConflict(t, err); ce.RenamedFrom != "A" {
		t.Errorf("conflict = %+v", ce)
	}
	if err := checkUniqueNames("app", []string{"A", "B"}, []string{"", ""}); err != nil {
		t.Errorf("unique names reported: %v", err)
	}
}

func TestMergeThreeWay_PropagatesConflicts(t *testing.T) {
	base := &ClientConfigUpdate{Groups: []config.GroupConfig{{Name: "A"}}, Apps: []ClientAppConfig{testApp("A", "u")}}
	groupClash := &ClientConfigUpdate{Groups: []config.GroupConfig{{Name: "B", OriginalName: "A"}}, Apps: base.Apps}
	theirs := &ClientConfigUpdate{Groups: []config.GroupConfig{{Name: "A"}, {Name: "B"}}, Apps: []ClientAppConfig{testApp("A", "u"), testApp("B", "u")}}
	if _, err := mergeThreeWay(base, groupClash, theirs); asConflict(t, err).Kind != "group" {
		t.Errorf("group conflict not returned: %v", err)
	}
	renamedApp := testApp("B", "u")
	renamedApp.OriginalName = "A"
	appClash := &ClientConfigUpdate{Groups: base.Groups, Apps: []ClientAppConfig{renamedApp}}
	theirs.Groups = base.Groups
	if _, err := mergeThreeWay(base, appClash, theirs); asConflict(t, err).Kind != "app" {
		t.Errorf("app conflict not returned: %v", err)
	}
}

func TestMergeThreeWay_ServerAddedAppInDeletedGroupIsUngrouped(t *testing.T) {
	inG := testApp("Mine", "u")
	inG.Group = "G"
	base := &ClientConfigUpdate{
		Groups: []config.GroupConfig{{Name: "G"}, {Name: "Keep"}},
		Apps:   []ClientAppConfig{inG},
	}
	minesApp := inG
	minesApp.Group = "" // the Apps tab ungroups a deleted group's apps
	mine := &ClientConfigUpdate{Groups: []config.GroupConfig{{Name: "Keep"}}, Apps: []ClientAppConfig{minesApp}}
	added := testApp("Added", "u")
	added.Group = "G"
	inKeep := testApp("Kept", "u")
	inKeep.Group = "Keep"
	theirs := &ClientConfigUpdate{
		Groups: []config.GroupConfig{{Name: "G"}, {Name: "Keep"}},
		Apps:   []ClientAppConfig{inG, added, inKeep},
	}
	got := mustMergeThreeWay(t, base, mine, theirs)
	if len(got.Groups) != 1 || got.Groups[0].Name != "Keep" {
		t.Fatalf("groups = %+v", got.Groups)
	}
	groupOf := map[string]string{}
	for i := range got.Apps {
		groupOf[got.Apps[i].Name] = got.Apps[i].Group
	}
	if g, ok := groupOf["Added"]; !ok || g != "" {
		t.Errorf("server-added app left in deleted group: %q (present %v)", g, ok)
	}
	if groupOf["Mine"] != "" || groupOf["Kept"] != "Keep" {
		t.Errorf("apps = %+v", got.Apps)
	}
}

func TestUngroupDeletedGroups_RenamedOrRecreatedGroupKeepsApps(t *testing.T) {
	base := []config.GroupConfig{{Name: "Old"}, {Name: "Re"}}
	mine := []config.GroupConfig{{Name: "New", OriginalName: "Old"}}
	merged := []config.GroupConfig{{Name: "New", OriginalName: "Old"}, {Name: "Re", OriginalName: "Re"}}
	apps := []ClientAppConfig{testApp("A", "u"), testApp("B", "u")}
	apps[0].Group = "Old" // cascadeGroupRenames re-points it later
	apps[1].Group = "Re"  // a group of that name survives the merge
	ungroupDeletedGroups(base, mine, merged, apps)
	if apps[0].Group != "Old" || apps[1].Group != "Re" {
		t.Errorf("apps = %+v", apps)
	}
}

// neverIn is a cascadeGroupRenames wasIn that knows no prior placement.
func neverIn(int, string) bool { return false }

func TestCascadeGroupRenames_NewGroupReusingOldNameKeepsItsApps(t *testing.T) {
	// The payload renames X to Y and adds a new group X.
	groups := []config.GroupConfig{{Name: "Y", OriginalName: "X"}, {Name: "X"}}
	apps := []ClientAppConfig{testApp("Placed", "u"), testApp("Stored", "u"), testApp("Moved", "u")}
	apps[0].Group = "X" // the user put it in the new X
	apps[1].Group = "X" // was in the old X before this save
	apps[2].Group = "Y"
	wasIn := func(i int, g string) bool { return i == 1 && g == "X" }
	cascadeGroupRenames(groups, apps, wasIn)
	if apps[0].Group != "X" || apps[1].Group != "Y" || apps[2].Group != "Y" {
		t.Errorf("apps = %+v", apps)
	}
}

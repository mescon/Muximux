package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// The docker_managed group marker is server-owned: a client can neither set
// nor keep it, a Settings edit of icon, colour or order clears it, and only
// admins see it (#500).

func markerFixture(t *testing.T) (*APIHandler, *config.Config, ClientConfigUpdate) {
	t.Helper()
	cfg := &config.Config{
		Groups: []config.GroupConfig{
			{Name: "Media", Order: 0, Color: "#111111", DockerManaged: true, DockerOrder: true},
			{Name: "Mine", Order: 1},
			{Name: "Free", Order: 2, DockerManaged: true}, // no order label
		},
		Apps: []config.AppConfig{{Name: "Plex", URL: "http://plex:80", Enabled: true, Group: "Media"}},
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	h := NewAPIHandler(cfg, path, &sync.RWMutex{})
	wire, _ := json.Marshal(buildClientConfigResponse(cfg, auth.RoleAdmin, nil))
	var loaded ClientConfigUpdate
	if err := json.Unmarshal(wire, &loaded); err != nil {
		t.Fatal(err)
	}
	return h, cfg, loaded
}

// cloneUpdate deep-copies the groups so editing mine leaves base alone.
func cloneUpdate(u *ClientConfigUpdate) ClientConfigUpdate {
	out := *u
	out.Groups = append([]config.GroupConfig(nil), u.Groups...)
	out.Apps = append([]ClientAppConfig(nil), u.Apps...)
	return out
}

func groupByName(cfg *config.Config, name string) *config.GroupConfig {
	for i := range cfg.Groups {
		if cfg.Groups[i].Name == name {
			return &cfg.Groups[i]
		}
	}
	return nil
}

func TestSaveConfig_MarkerKeptWhenGroupUntouched(t *testing.T) {
	h, cfg, loaded := markerFixture(t)
	base := cloneUpdate(&loaded)
	mine := cloneUpdate(&loaded)
	mine.Title = "Edited"
	mine.Groups[0].Name = "Media Server" // a rename or expand is not a style edit
	mine.Groups[0].OriginalName = "Media"
	mine.Groups[0].Expanded = true
	mine.Base = &base
	putConfig(t, h, &mine)
	if g := groupByName(cfg, "Media Server"); g == nil || !g.DockerManaged {
		t.Errorf("marker lost on an untouched group: %+v", cfg.Groups)
	}
}

func TestSaveConfig_StyleEditClearsMarker(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(g *config.GroupConfig)
	}{
		{"color", func(g *config.GroupConfig) { g.Color = "#222222" }},
		{"order", func(g *config.GroupConfig) { g.Order = 7 }},
		{"icon", func(g *config.GroupConfig) { g.Icon = config.AppIconConfig{Type: "lucide", Name: "film"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cfg, loaded := markerFixture(t)
			base := cloneUpdate(&loaded)
			mine := cloneUpdate(&loaded)
			tc.edit(&mine.Groups[0])
			mine.Groups[0].DockerManaged = true // a client cannot keep it either
			mine.Base = &base
			putConfig(t, h, &mine)
			if g := groupByName(cfg, "Media"); g == nil || g.DockerManaged {
				t.Errorf("marker kept after a %s edit: %+v", tc.name, g)
			}
		})
	}
}

// A server-side label re-sync while the dialog was open is not an edit.
func TestSaveConfig_ServerResyncWhileOpenKeepsMarker(t *testing.T) {
	h, cfg, loaded := markerFixture(t)
	base := cloneUpdate(&loaded)
	mine := cloneUpdate(&loaded)
	mine.Title = "Edited"
	mine.Base = &base
	cfg.Groups[0].Color = "#333333" // the poller applied a new label
	putConfig(t, h, &mine)
	if g := groupByName(cfg, "Media"); !g.DockerManaged || g.Color != "#333333" {
		t.Errorf("group = %+v, want the server colour and the marker", g)
	}
}

func TestSaveConfig_ClientCannotSetMarker(t *testing.T) {
	h, cfg, loaded := markerFixture(t)
	base := cloneUpdate(&loaded)
	base.Groups[1].DockerManaged = true
	mine := cloneUpdate(&loaded)
	mine.Groups[1].DockerManaged = true
	mine.Groups = append(mine.Groups, config.GroupConfig{Name: "New", DockerManaged: true})
	mine.Base = &base
	putConfig(t, h, &mine)
	for _, n := range []string{"Mine", "New"} {
		if g := groupByName(cfg, n); g == nil || g.DockerManaged {
			t.Errorf("client set the marker on %s: %+v", n, g)
		}
	}
	if !groupByName(cfg, "Media").DockerManaged {
		t.Error("stripping the base marker cleared the stored one")
	}
}

// A managed group removed on the server while the dialog was open, and
// left untouched in the payload, stays removed even though the payload
// and base carry different markers.
func TestSaveConfig_RemovedManagedGroupStaysRemoved(t *testing.T) {
	h, cfg, loaded := markerFixture(t)
	base := cloneUpdate(&loaded)
	mine := cloneUpdate(&loaded)
	mine.Groups[0].DockerManaged = false
	mine.Base = &base
	cfg.Groups = cfg.Groups[1:]
	putConfig(t, h, &mine)
	if groupByName(cfg, "Media") != nil {
		t.Errorf("deleted group re-added: %+v", cfg.Groups)
	}
}

func TestSaveConfig_TwoWayMarkerRules(t *testing.T) {
	h, cfg, loaded := markerFixture(t)
	same := cloneUpdate(&loaded)
	same.Groups[1].DockerManaged = true
	putConfig(t, h, &same)
	if !groupByName(cfg, "Media").DockerManaged || groupByName(cfg, "Mine").DockerManaged {
		t.Errorf("two-way unchanged: %+v", cfg.Groups)
	}

	edited := cloneUpdate(&loaded)
	edited.Groups[0].Order = 9
	putConfig(t, h, &edited)
	if g := groupByName(cfg, "Media"); g.DockerManaged || g.Order != 9 {
		t.Errorf("two-way edit: %+v", g)
	}
}

func TestGroupsForRole(t *testing.T) {
	cfg := &config.Config{Groups: []config.GroupConfig{{Name: "Media", DockerManaged: true}}}
	if got := buildClientConfigResponse(cfg, auth.RoleUser, nil); got.Groups[0].DockerManaged {
		t.Error("non-admin config response carries the marker")
	}
	if !cfg.Groups[0].DockerManaged {
		t.Error("stripping for a non-admin mutated the live config")
	}
	if got := buildClientConfigResponse(cfg, auth.RoleAdmin, nil); !got.Groups[0].DockerManaged {
		t.Error("admin config response lacks the marker")
	}
}

func groupRequest(method, path, role, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if role != "" {
		r = r.WithContext(auth.WithUserContext(r.Context(), &auth.User{Username: "u", Role: role}))
	}
	return r
}

func TestGetGroups_MarkerAdminOnly(t *testing.T) {
	h, _, _ := markerFixture(t)
	for _, tc := range []struct {
		role string
		want bool
	}{{auth.RoleUser, false}, {auth.RoleAdmin, true}} {
		w := httptest.NewRecorder()
		h.GetGroups(w, groupRequest(http.MethodGet, "/api/groups", tc.role, ""))
		var list []config.GroupConfig
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if list[0].DockerManaged != tc.want {
			t.Errorf("GetGroups as %s: marker %v", tc.role, list[0].DockerManaged)
		}

		w = httptest.NewRecorder()
		h.GetGroup(w, groupRequest(http.MethodGet, "/api/group/Media", tc.role, ""), "Media")
		var one config.GroupConfig
		if err := json.Unmarshal(w.Body.Bytes(), &one); err != nil {
			t.Fatal(err)
		}
		if one.DockerManaged != tc.want || !strings.Contains(w.Body.String(), "Media") {
			t.Errorf("GetGroup as %s: %s", tc.role, w.Body.String())
		}
	}
}

func TestCreateGroup_IgnoresMarker(t *testing.T) {
	h, cfg, _ := markerFixture(t)
	w := httptest.NewRecorder()
	h.CreateGroup(w, groupRequest(http.MethodPost, "/api/groups", auth.RoleAdmin, `{"name":"X","docker_managed":true}`))
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateGroup = %d %s", w.Code, w.Body.String())
	}
	if g := groupByName(cfg, "X"); g == nil || g.DockerManaged {
		t.Errorf("created group = %+v", g)
	}
}

func TestUpdateGroup_MarkerRules(t *testing.T) {
	h, cfg, _ := markerFixture(t)
	put := func(name, body string) {
		t.Helper()
		w := httptest.NewRecorder()
		h.UpdateGroup(w, groupRequest(http.MethodPut, "/api/group/"+name, auth.RoleAdmin, body), name)
		if w.Code != http.StatusOK {
			t.Fatalf("UpdateGroup = %d %s", w.Code, w.Body.String())
		}
	}
	// Rename and expand only: kept, whatever the payload says.
	put("Media", `{"name":"Media2","color":"#111111","order":0,"icon":{"type":""},"expanded":true,"docker_managed":false}`)
	if g := groupByName(cfg, "Media2"); g == nil || !g.DockerManaged {
		t.Fatalf("marker lost on a rename: %+v", cfg.Groups)
	}
	// A colour edit releases the group.
	put("Media2", `{"name":"Media2","color":"#222222","order":0,"icon":{"type":""},"docker_managed":true}`)
	if g := groupByName(cfg, "Media2"); g.DockerManaged {
		t.Errorf("marker kept after a colour edit: %+v", g)
	}
	// An unmanaged group cannot be made managed.
	put("Mine", `{"name":"Mine","order":1,"icon":{"type":""},"docker_managed":true}`)
	if groupByName(cfg, "Mine").DockerManaged {
		t.Error("payload set the marker")
	}
}

func TestReleasedGroups(t *testing.T) {
	stored := []config.GroupConfig{{Name: "A", DockerManaged: true}, {Name: "B", DockerManaged: true}, {Name: "C"}}
	groups := []config.GroupConfig{
		{Name: "A2", OriginalName: "A"},
		{Name: "B", DockerManaged: true},
		{Name: "C"},
		{Name: "D"},
	}
	got := releasedGroups(stored, groups)
	if len(got) != 1 || got[0] != "A2" {
		t.Errorf("released = %v, want [A2]", got)
	}
}

// The marker is persisted in config.yaml and survives a save round trip.
func TestSaveConfig_MarkerPersisted(t *testing.T) {
	h, _, loaded := markerFixture(t)
	mine := cloneUpdate(&loaded)
	base := cloneUpdate(&loaded)
	mine.Base = &base
	mine.Title = "x"
	body, _ := json.Marshal(&mine)
	w := httptest.NewRecorder()
	h.SaveConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("SaveConfig = %d", w.Code)
	}
	loadedCfg, err := config.Load(h.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if g := groupByName(loadedCfg, "Media"); g == nil || !g.DockerManaged {
		t.Errorf("marker not persisted: %+v", loadedCfg.Groups)
	}
}

// A drag renumbers groups: a managed group whose order comes from a label
// is released, one whose order no label sets keeps the marker.
func TestSaveConfig_ReorderReleasesOnlyLabelOrderedGroups(t *testing.T) {
	for _, threeWay := range []bool{true, false} {
		h, cfg, loaded := markerFixture(t)
		base := cloneUpdate(&loaded)
		mine := cloneUpdate(&loaded)
		mine.Groups[0].Order, mine.Groups[2].Order = 2, 0 // Media <-> Free
		if threeWay {
			mine.Base = &base
		}
		putConfig(t, h, &mine)
		if g := groupByName(cfg, "Media"); g.DockerManaged || g.DockerOrder {
			t.Errorf("threeWay=%v: label-ordered group kept its markers: %+v", threeWay, g)
		}
		if g := groupByName(cfg, "Free"); !g.DockerManaged || g.DockerOrder || g.Order != 0 {
			t.Errorf("threeWay=%v: freely ordered group = %+v", threeWay, g)
		}
	}
}

func TestDockerOrderNeverAcceptedFromClient(t *testing.T) {
	h, cfg, loaded := markerFixture(t)
	base := cloneUpdate(&loaded)
	mine := cloneUpdate(&loaded)
	for i := range mine.Groups {
		mine.Groups[i].DockerOrder = true
		base.Groups[i].DockerOrder = true
	}
	mine.Base = &base
	putConfig(t, h, &mine)
	if groupByName(cfg, "Free").DockerOrder || groupByName(cfg, "Mine").DockerOrder || !groupByName(cfg, "Media").DockerOrder {
		t.Errorf("save took DockerOrder from the client: %+v", cfg.Groups)
	}
	// A two-way save keeps the stored flag, whatever the payload says.
	two := cloneUpdate(&loaded)
	two.Groups[0].DockerOrder = false
	putConfig(t, h, &two)
	if !groupByName(cfg, "Media").DockerOrder {
		t.Error("two-way save dropped the stored flag")
	}

	w := httptest.NewRecorder()
	h.CreateGroup(w, groupRequest(http.MethodPost, "/api/groups", auth.RoleAdmin, `{"name":"Y","docker_order":true}`))
	if g := groupByName(cfg, "Y"); g == nil || g.DockerOrder {
		t.Errorf("CreateGroup took docker_order: %+v", g)
	}
	w = httptest.NewRecorder()
	h.UpdateGroup(w, groupRequest(http.MethodPut, "/api/group/Free", auth.RoleAdmin,
		`{"name":"Free","order":2,"icon":{"type":""},"docker_order":true}`), "Free")
	if g := groupByName(cfg, "Free"); !g.DockerManaged || g.DockerOrder {
		t.Errorf("UpdateGroup took docker_order: %+v", g)
	}

	resp := buildClientConfigResponse(cfg, auth.RoleUser, nil)
	for i := range resp.Groups {
		if resp.Groups[i].DockerOrder {
			t.Errorf("non-admin sees docker_order on %s", resp.Groups[i].Name)
		}
	}
	w = httptest.NewRecorder()
	h.GetGroup(w, groupRequest(http.MethodGet, "/api/group/Media", auth.RoleUser, ""), "Media")
	if strings.Contains(w.Body.String(), "docker_order") {
		t.Errorf("non-admin GetGroup = %s", w.Body.String())
	}
	if admin := buildClientConfigResponse(cfg, auth.RoleAdmin, nil); !admin.Groups[0].DockerOrder {
		t.Error("admin lacks docker_order")
	}
}

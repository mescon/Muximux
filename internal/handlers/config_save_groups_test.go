package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// Groups Docker discovery creates while a Settings dialog is open survive
// a save from that dialog; a group the user deleted stays deleted (#500).

func groupSaveFixture(t *testing.T) (*APIHandler, *config.Config, ClientConfigUpdate) {
	t.Helper()
	cfg := &config.Config{
		Groups: []config.GroupConfig{{Name: "Media", Order: 0}, {Name: "Old", Order: 1}},
		Apps:   []config.AppConfig{{Name: "Plex", URL: "http://plex:80", Enabled: true, Group: "Media"}},
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
	// The server creates a group and an app in it after the client loaded.
	cfg.Groups, _, _ = config.EnsureGroup(cfg.Groups, "Infra")
	cfg.Apps = append(cfg.Apps, config.AppConfig{Name: "Traefik", URL: "http://traefik:80", Enabled: true, Group: "Infra", DockerKey: "label:t", DockerEndpoint: "unix:///x"})
	return h, cfg, loaded
}

func putConfig(t *testing.T, h *APIHandler, update *ClientConfigUpdate) {
	t.Helper()
	body, _ := json.Marshal(update)
	w := httptest.NewRecorder()
	h.SaveConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("SaveConfig = %d: %s", w.Code, w.Body.String())
	}
}

func groupOfApp(cfg *config.Config, name string) (string, bool) {
	for i := range cfg.Apps {
		if cfg.Apps[i].Name == name {
			return cfg.Apps[i].Group, true
		}
	}
	return "", false
}

func TestSaveConfig_KeepsServerAddedGroupAndHonoursDelete(t *testing.T) {
	h, cfg, loaded := groupSaveFixture(t)
	base := loaded
	mine := loaded
	mine.Groups = []config.GroupConfig{loaded.Groups[0]} // the user deletes "Old"
	mine.Title = "Edited"
	mine.Base = &base
	putConfig(t, h, &mine)

	names := map[string]bool{}
	for i := range cfg.Groups {
		names[cfg.Groups[i].Name] = true
	}
	if !names["Infra"] || !names["Media"] || names["Old"] || len(cfg.Groups) != 2 {
		t.Errorf("groups = %+v, want Media and the server-added Infra", cfg.Groups)
	}
	if g, ok := groupOfApp(cfg, "Traefik"); !ok || g != "Infra" {
		t.Errorf("server-added app = %q (present %v)", g, ok)
	}
}

func TestSaveConfig_ServerAddedGroupFoldsIntoSameSlugPayloadGroup(t *testing.T) {
	h, cfg, loaded := groupSaveFixture(t)
	base := loaded
	mine := loaded
	mine.Groups = append(append([]config.GroupConfig(nil), loaded.Groups...), config.GroupConfig{Name: "infra", Order: 2})
	mine.Base = &base
	putConfig(t, h, &mine)

	count := 0
	for i := range cfg.Groups {
		if config.Slugify(cfg.Groups[i].Name) == "infra" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("near-duplicate groups after save: %+v", cfg.Groups)
	}
	if g, _ := groupOfApp(cfg, "Traefik"); g != "infra" {
		t.Errorf("server-added app group = %q, want re-pointed to infra", g)
	}
}

// Without a base (scripts, older frontends) the payload replaces the
// groups as before.
func TestSaveConfig_NoBaseReplacesGroups(t *testing.T) {
	h, cfg, loaded := groupSaveFixture(t)
	putConfig(t, h, &loaded)
	for i := range cfg.Groups {
		if cfg.Groups[i].Name == "Infra" {
			t.Errorf("legacy save kept a group the payload lacks: %+v", cfg.Groups)
		}
	}
}

func TestSlugMatch(t *testing.T) {
	groups := []config.GroupConfig{{Name: "Infra Tools"}, {Name: "!!!"}}
	if got := slugMatch(groups, "infra-tools"); got != "Infra Tools" {
		t.Errorf("slug match = %q", got)
	}
	if got := slugMatch(groups, "Infra Tools"); got != "" {
		t.Errorf("exact name should be left to the conflict check, got %q", got)
	}
	if got := slugMatch(groups, "???"); got != "" {
		t.Errorf("slugless name matched %q", got)
	}
}

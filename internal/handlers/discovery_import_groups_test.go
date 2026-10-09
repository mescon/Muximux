package handlers

import (
	"errors"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// A manual import creates the group an app names when the config does not
// define it (#500), in the same save.

func importItem(key, name, group string) ImportItem {
	return ImportItem{
		Key: key, Strategy: "container_ip",
		App: &ClientAppConfig{Name: name, URL: "http://10.0.0.5:80", Enabled: true, Group: group},
	}
}

func TestImportDocker_CreatesMissingGroup(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	cfg.Groups = []config.GroupConfig{{Name: "Media", Order: 0}}
	res, _ := postImport(t, h, ImportRequest{Items: []ImportItem{
		importItem("name:a", "A", "Infra"),
		importItem("name:b", "B", "infra "), // same group, other spelling
		importItem("name:c", "C", "Downloads"),
		importItem("name:d", "D", ""),
	}})
	if !res.Success {
		t.Fatalf("import failed: %+v", res)
	}
	want := []config.GroupConfig{
		{Name: "Media", Order: 0},
		config.NewAutoGroup("Infra", 1),
		config.NewAutoGroup("Downloads", 2),
	}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("groups = %+v", cfg.Groups)
	}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Errorf("group %d = %+v, want %+v", i, cfg.Groups[i], want[i])
		}
	}
	for name, g := range map[string]string{"A": "Infra", "B": "Infra", "C": "Downloads", "D": ""} {
		for i := range cfg.Apps {
			if cfg.Apps[i].Name == name && cfg.Apps[i].Group != g {
				t.Errorf("app %s group = %q, want %q", name, cfg.Apps[i].Group, g)
			}
		}
	}
	loaded, err := config.Load(h.configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Groups) != 3 {
		t.Errorf("groups not persisted: %+v", loaded.Groups)
	}
}

func TestImportDocker_GroupMatchedBySlugIsIdempotent(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	cfg.Groups = []config.GroupConfig{{Name: "Media Server", Order: 3}}
	res, _ := postImport(t, h, ImportRequest{Items: []ImportItem{importItem("name:a", "A", "media-server")}})
	if !res.Success {
		t.Fatalf("import failed: %+v", res)
	}
	if len(cfg.Groups) != 1 || cfg.Apps[0].Group != "Media Server" {
		t.Errorf("groups = %+v app group = %q", cfg.Groups, cfg.Apps[0].Group)
	}
	// A second import into a group created by the first creates nothing.
	if res, _ = postImport(t, h, ImportRequest{Items: []ImportItem{importItem("name:b", "B", "Tools")}}); !res.Success {
		t.Fatalf("import failed: %+v", res)
	}
	if res, _ = postImport(t, h, ImportRequest{Items: []ImportItem{importItem("name:c", "C", "TOOLS")}}); !res.Success {
		t.Fatalf("import failed: %+v", res)
	}
	if len(cfg.Groups) != 2 || cfg.Groups[1] != config.NewAutoGroup("Tools", 4) {
		t.Errorf("groups = %+v", cfg.Groups)
	}
}

func TestImportDocker_SaveFailureRollsBackGroups(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	cfg.Groups = []config.GroupConfig{{Name: "Media"}}
	h.configPath = "/dev/null/impossible/config.yaml"
	res, _ := postImport(t, h, ImportRequest{Items: []ImportItem{importItem("name:a", "A", "Infra")}})
	if res.Success {
		t.Fatal("expected save failure")
	}
	if len(cfg.Apps) != 0 || len(cfg.Groups) != 1 {
		t.Errorf("not rolled back: apps=%+v groups=%+v", cfg.Apps, cfg.Groups)
	}
}

func TestImportDocker_ItemFailureLeavesGroupsUntouched(t *testing.T) {
	h, cfg := newTestImportHandler(t, nil)
	res, _ := postImport(t, h, ImportRequest{Items: []ImportItem{
		importItem("name:a", "A", "Infra"),
		{Key: "name:b", App: &ClientAppConfig{Name: "B"}}, // no URL
	}})
	if res.Success {
		t.Fatal("expected validation failure")
	}
	if len(cfg.Apps) != 0 || len(cfg.Groups) != 0 {
		t.Errorf("batch failure leaked: apps=%+v groups=%+v", cfg.Apps, cfg.Groups)
	}
}

func TestImportDocker_CaddyFailureRollsBackGroups(t *testing.T) {
	h, cfg := newImportHandlerWithProxy(t, nil, nil, func() error { return errors.New("reload failed") })
	res, _ := postImport(t, h, ImportRequest{Items: []ImportItem{{
		Key: "name:c", Strategy: "container_ip", Routing: "direct",
		App:     &ClientAppConfig{Name: "C", URL: "http://10.0.0.5:80", Enabled: true, Group: "Infra"},
		Gateway: &config.GatewaySite{Domain: "c.example.com", BackendURL: "http://10.0.0.5:80", TLS: "auto"},
	}}})
	if res.Success {
		t.Fatal("expected Caddy failure")
	}
	if len(cfg.Groups) != 0 {
		t.Errorf("group creation not rolled back: %+v", cfg.Groups)
	}
}

package handlers

import (
	"encoding/json"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
)

func TestAnnotateTracked(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discovery.Docker.Endpoint = "unix:///a.sock"
	cfg.Apps = []config.AppConfig{
		{Name: "Manual", DockerKey: "k-app", DockerEndpoint: "unix:///a.sock"},
		{Name: "Auto", DockerKey: "k-auto", DockerAutoImported: true},
		{Name: "Plain"},
		{Name: "OtherDaemon", DockerKey: "k-other", DockerEndpoint: "tcp://elsewhere"},
	}
	cfg.Server.GatewaySites = []config.GatewaySite{
		{Domain: "site.example.com", DockerKey: "k-site"},
	}
	cfg.QuarantineApp(&config.AppConfig{Name: "Bad", DockerKey: "k-quar"}, "invalid")

	sugs := []discovery.Suggestion{
		{Key: "k-app", Name: "Manual"},
		{Key: "k-auto", Name: "Auto"},
		{Key: "k-site", Name: "Whatever"},
		{Key: "k-quar", Name: "Bad"},
		{Key: "k-new", Name: "Plain"},
		{Key: "k-other", Name: "OtherDaemon"},
		{Key: "k-free", Name: "Fresh"},
	}
	annotateTracked(cfg, sugs)

	want := []struct {
		kind, name string
		auto       bool
	}{
		{discovery.TrackedApp, "Manual", false},
		{discovery.TrackedApp, "Auto", true},
		{discovery.TrackedSite, "site.example.com", false},
		{discovery.TrackedQuarantined, "Bad", false},
	}
	for i, w := range want {
		tr := sugs[i].Tracked
		if tr == nil || tr.Kind != w.kind || tr.Name != w.name || tr.AutoImported != w.auto {
			t.Errorf("sugs[%d] tracked = %+v, want %+v", i, tr, w)
		}
		if sugs[i].NameTaken {
			t.Errorf("sugs[%d]: tracked row must not be name_taken", i)
		}
	}
	if sugs[4].Tracked != nil || !sugs[4].NameTaken {
		t.Errorf("name collision: tracked=%v name_taken=%v", sugs[4].Tracked, sugs[4].NameTaken)
	}
	// Entry from another daemon is out of scope: untracked, but its name is taken.
	if sugs[5].Tracked != nil || !sugs[5].NameTaken {
		t.Errorf("other endpoint: tracked=%v name_taken=%v", sugs[5].Tracked, sugs[5].NameTaken)
	}
	if sugs[6].Tracked != nil || sugs[6].NameTaken {
		t.Errorf("fresh: %+v", sugs[6])
	}
	b, err := json.Marshal(sugs[6])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["tracked"]; ok {
		t.Error("untracked suggestion must omit tracked")
	}
	if _, ok := m["name_taken"]; ok {
		t.Error("name_taken must be omitted when false")
	}
}

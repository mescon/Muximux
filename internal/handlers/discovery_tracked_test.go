package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	// Entry from another daemon is still tracked, and says which endpoint.
	if tr := sugs[5].Tracked; tr == nil || tr.Endpoint != "tcp://elsewhere" || sugs[5].NameTaken {
		t.Errorf("other endpoint: tracked=%+v name_taken=%v", tr, sugs[5].NameTaken)
	}
	if sugs[0].Tracked.Endpoint != "" || sugs[1].Tracked.Endpoint != "" {
		t.Error("same-endpoint or empty-endpoint entries must not carry an endpoint")
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

func TestScanDocker_AnnotatesTracked(t *testing.T) {
	media := discovery.ContainerNetworks{Networks: map[string]discovery.ContainerNetwork{"media": {IPAddress: "10.0.0.5"}}}
	socket, cleanup := fakeDockerForLifecycle(t, []discovery.ContainerSummary{
		{ID: "a", Names: []string{"/one"}, Image: "acme/one", NetworkSettings: media,
			Labels: map[string]string{"muximux.app.port": "80"}},
		{ID: "b", Names: []string{"/two"}, Image: "acme/two", NetworkSettings: media,
			Labels: map[string]string{"muximux.app.port": "80"}},
	})
	defer cleanup()
	h, cfg, _ := newTestDiscoveryHandler(t, &config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket,
		NetworkStrategy: "container_ip", NetworkFilter: "media"})
	scan := func() discovery.ScanResult {
		w := httptest.NewRecorder()
		h.ScanDocker(w, adminCtxRequest(http.MethodGet, "/api/discovery/docker/scan"))
		var got discovery.ScanResult
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	first := scan()
	if len(first.Suggestions) != 2 {
		t.Fatalf("got %+v", first)
	}
	keyOne, nameTwo := first.Suggestions[0].Key, first.Suggestions[1].Name
	nameOne := first.Suggestions[0].Name
	cfg.Apps = []config.AppConfig{
		{Name: "Tracked One", DockerKey: keyOne},
		{Name: nameTwo},
	}
	got := scan()
	for i := range got.Suggestions {
		s := got.Suggestions[i]
		switch s.Name {
		case nameOne:
			if s.Tracked == nil || s.Tracked.Kind != discovery.TrackedApp || s.Tracked.Name != "Tracked One" {
				t.Errorf("one: %+v", s.Tracked)
			}
		case nameTwo:
			if s.Tracked != nil || !s.NameTaken {
				t.Errorf("two: tracked=%v name_taken=%v", s.Tracked, s.NameTaken)
			}
		}
	}
}

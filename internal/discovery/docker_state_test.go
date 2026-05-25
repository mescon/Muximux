package discovery

import (
	"testing"
)

func TestDockerStateCache_RoundTrip(t *testing.T) {
	s := &Service{}
	st := DockerState{Status: "running", Health: "healthy", RestartCount: 2, Image: "sonarr:latest"}
	s.SetDockerStateForApp("sonarr", &st)
	got, ok := s.DockerStateForApp("sonarr")
	if !ok {
		t.Fatalf("expected found")
	}
	if got.Status != "running" || got.Health != "healthy" || got.RestartCount != 2 || got.Image != "sonarr:latest" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestDockerStateCache_Missing(t *testing.T) {
	s := &Service{}
	if _, ok := s.DockerStateForApp("nope"); ok {
		t.Fatalf("expected not found")
	}
}

func TestDockerStateCache_ReplaceAll(t *testing.T) {
	s := &Service{}
	s.SetDockerStateForApp("a", &DockerState{Status: "running"})
	s.SetDockerStateForApp("b", &DockerState{Status: "exited"})
	s.SetDockerStateCache(map[string]DockerState{
		"a": {Status: "exited"},
	})
	if got, _ := s.DockerStateForApp("a"); got.Status != "exited" {
		t.Fatalf("want exited, got %q", got.Status)
	}
	if _, ok := s.DockerStateForApp("b"); ok {
		t.Fatalf("expected b to be cleared by replace")
	}
}

func TestDockerStateCache_Snapshot_ReturnsCopy(t *testing.T) {
	s := &Service{}
	s.SetDockerStateForApp("a", &DockerState{Status: "running"})
	snap := s.DockerStateSnapshot()
	snap["a"] = DockerState{Status: "exited"}
	if got, _ := s.DockerStateForApp("a"); got.Status != "running" {
		t.Fatalf("snapshot mutation leaked: %q", got.Status)
	}
}

func TestSocketWritable_DefaultsToFalse(t *testing.T) {
	s := &Service{}
	if s.SocketWritable() {
		t.Fatalf("expected false default")
	}
	s.socketWritable = true
	if !s.SocketWritable() {
		t.Fatalf("expected true after set")
	}
}

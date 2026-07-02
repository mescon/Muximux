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

// TestCommitPolledDockerState_PreservesMidTickLifecycleWrite reproduces the
// lost-update the seq guard closes: a poll tick snapshots the cache, then a
// lifecycle handler writes a fresh post-action state while the tick is still
// inspecting, then the tick commits its wholesale map. The lifecycle write
// must survive -- it exists precisely to show fresh state before the next
// poll. A plain wholesale replace (the old SetDockerStateCache) would revert
// "web" to the poll's staler "exited" read.
func TestCommitPolledDockerState_PreservesMidTickLifecycleWrite(t *testing.T) {
	s := &Service{}
	// Poll snapshot sees the container stopped.
	s.SetDockerStateForApp("web", &DockerState{Status: "exited"})
	prev, sinceSeq := s.snapshotDockerStateForPoll()
	if _, ok := prev["web"]; !ok {
		t.Fatalf("snapshot should contain web")
	}

	// Lifecycle Start lands mid-tick, after the snapshot.
	s.SetDockerStateForApp("web", &DockerState{Status: "running"})

	// The tick built next from its (pre-action) inspect: still "exited".
	next := dockerStateCache{"web": {Status: "exited"}}
	s.commitPolledDockerState(next, sinceSeq)

	if got, _ := s.DockerStateForApp("web"); got.Status != "running" {
		t.Fatalf("mid-tick lifecycle write was clobbered: want running, got %q", got.Status)
	}
}

// TestCommitPolledDockerState_PollWinsAndPrunes verifies the guard does not
// over-preserve: a manual write from BEFORE the snapshot must not override a
// fresher poll read, and apps absent from next are still pruned.
func TestCommitPolledDockerState_PollWinsAndPrunes(t *testing.T) {
	s := &Service{}
	s.SetDockerStateForApp("web", &DockerState{Status: "running"})
	s.SetDockerStateForApp("gone", &DockerState{Status: "exited"})
	prev, sinceSeq := s.snapshotDockerStateForPoll()
	_ = prev

	// No lifecycle write after the snapshot. Poll sees web stopped and no
	// longer tracks "gone".
	next := dockerStateCache{"web": {Status: "exited"}}
	s.commitPolledDockerState(next, sinceSeq)

	if got, _ := s.DockerStateForApp("web"); got.Status != "exited" {
		t.Fatalf("poll read should win when no newer manual write: want exited, got %q", got.Status)
	}
	if _, ok := s.DockerStateForApp("gone"); ok {
		t.Fatalf("vanished app should be pruned")
	}
	// The manual-seq stamp for the pruned app must not linger.
	if _, ok := s.dockerStateManualSeq["gone"]; ok {
		t.Fatalf("manual-seq stamp for pruned app should be dropped")
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

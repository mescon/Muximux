package discovery

import (
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
)

// Refresh-state tests cover the in-memory divergence + LastSeen
// machine independent of the poller and the proxy. These are the
// pieces that drive the Settings banner state machine, so a tight
// unit test catches transition bugs without spinning up Caddy or
// docker.

func TestRecordRefreshTickSuccess_StampsLastSuccess(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{})
	if !svc.lastRefreshSuccessAt.IsZero() {
		t.Fatalf("fresh service has non-zero lastRefreshSuccessAt: %v", svc.lastRefreshSuccessAt)
	}
	svc.RecordRefreshTickSuccess()
	if svc.lastRefreshSuccessAt.IsZero() {
		t.Errorf("lastRefreshSuccessAt should be stamped after RecordRefreshTickSuccess")
	}
	if !svc.recoveredAt.IsZero() {
		t.Errorf("clean tick from cold state should not set recoveredAt; got %v", svc.recoveredAt)
	}
}

func TestRecordDivergence_BumpsCounterAndClearsRecovered(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{})
	// Simulate a prior recovery timestamp.
	svc.recoveredAt = time.Now().Add(-5 * time.Minute)
	svc.RecordDivergence()
	if svc.divergences != 1 {
		t.Errorf("divergences = %d, want 1", svc.divergences)
	}
	if svc.lastDivergenceAt.IsZero() {
		t.Errorf("lastDivergenceAt should be set")
	}
	if !svc.recoveredAt.IsZero() {
		t.Errorf("recoveredAt should be cleared by new divergence; got %v", svc.recoveredAt)
	}

	svc.RecordDivergence()
	if svc.divergences != 2 {
		t.Errorf("second divergence: counter = %d, want 2", svc.divergences)
	}
}

func TestDivergenceLifecycle_ActiveThenRecovered(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{})

	svc.RecordDivergence()
	if svc.recoveredAt.IsZero() == false {
		t.Errorf("after divergence, recoveredAt should be zero")
	}

	// First clean tick after divergence -> recoveredAt is set.
	svc.RecordRefreshTickSuccess()
	if svc.recoveredAt.IsZero() {
		t.Errorf("first clean tick after divergence should set recoveredAt")
	}
	firstRecover := svc.recoveredAt

	// Subsequent clean ticks must NOT keep updating recoveredAt -
	// the banner shows "recovered since X" and would jitter
	// otherwise.
	time.Sleep(2 * time.Millisecond)
	svc.RecordRefreshTickSuccess()
	if !svc.recoveredAt.Equal(firstRecover) {
		t.Errorf("subsequent clean tick moved recoveredAt: %v -> %v", firstRecover, svc.recoveredAt)
	}

	// New divergence wipes the recovered marker again.
	svc.RecordDivergence()
	if !svc.recoveredAt.IsZero() {
		t.Errorf("new divergence did not clear recoveredAt")
	}
}

func TestRecordSeen_AndLastSeen(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{})
	if !svc.LastSeen("label:foo").IsZero() {
		t.Errorf("unseen key has non-zero LastSeen")
	}
	svc.RecordSeen("label:foo")
	if svc.LastSeen("label:foo").IsZero() {
		t.Errorf("LastSeen returned zero after RecordSeen")
	}
}

func TestForgetTrackedKey_DropsLastSeen(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{})
	svc.RecordSeen("label:gone")
	if svc.LastSeen("label:gone").IsZero() {
		t.Fatalf("setup failure: RecordSeen did not stamp")
	}
	svc.ForgetTrackedKey("label:gone")
	if !svc.LastSeen("label:gone").IsZero() {
		t.Errorf("ForgetTrackedKey did not clear the entry")
	}
}

func TestFillRefreshTelemetry_ExposesAllFields(t *testing.T) {
	svc := NewService(&config.DiscoveryDockerConfig{})
	svc.RecordDivergence()
	svc.RecordRefreshTickSuccess() // recovers

	r := StatusResult{}
	svc.fillRefreshTelemetry(&r)
	if r.Divergences != 1 {
		t.Errorf("Divergences = %d, want 1", r.Divergences)
	}
	if r.LastDivergenceAt == "" {
		t.Errorf("LastDivergenceAt empty")
	}
	if r.RecoveredAt == "" {
		t.Errorf("RecoveredAt empty after recovery tick")
	}
	if r.LastRefreshSuccessAt == "" {
		t.Errorf("LastRefreshSuccessAt empty after success tick")
	}
}

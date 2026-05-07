package proxy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// newApplyTestProxy returns a Proxy seeded with a single prior site
// and a test reload hook that the caller drives. The hook records
// each call's site list (a snapshot) so tests can assert
// "the rollback re-asserted priorSites" precisely.
func newApplyTestProxy(prior []GatewaySite) *Proxy {
	p := New(&Config{ListenAddr: ":8080", InternalAddr: "127.0.0.1:18080"})
	p.SetGatewaySites(prior)
	return p
}

func TestApplyGatewaySites_Success(t *testing.T) {
	prior := []GatewaySite{{Domain: "old.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	candidate := []GatewaySite{{Domain: "new.example.com", BackendURL: "http://10.0.0.2:8080", TLS: "auto"}}

	p := newApplyTestProxy(prior)
	calls := 0
	var seen [][]GatewaySite
	p.testReloadHook = func() error {
		calls++
		// Snapshot the current site list so the test can assert
		// what reload would have applied.
		seen = append(seen, append([]GatewaySite(nil), p.GatewaySites()...))
		return nil
	}

	if err := p.ApplyGatewaySites(candidate, prior); err != nil {
		t.Fatalf("ApplyGatewaySites returned %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("reload called %d times, want 1", calls)
	}
	if !reflect.DeepEqual(seen[0], candidate) {
		t.Errorf("first reload saw %v, want candidate %v", seen[0], candidate)
	}
	if !reflect.DeepEqual(p.GatewaySites(), candidate) {
		t.Errorf("post-success sites = %v, want candidate %v", p.GatewaySites(), candidate)
	}
}

func TestApplyGatewaySites_CandidateFailsRollbackSucceeds(t *testing.T) {
	prior := []GatewaySite{{Domain: "old.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	candidate := []GatewaySite{{Domain: "broken.example.com", BackendURL: "http://10.0.0.2:8080", TLS: "auto"}}

	p := newApplyTestProxy(prior)
	candidateErr := errors.New("caddy: synthetic candidate parse failure")

	calls := 0
	var seen [][]GatewaySite
	p.testReloadHook = func() error {
		calls++
		seen = append(seen, append([]GatewaySite(nil), p.GatewaySites()...))
		// First call (candidate) fails; second call (rollback) succeeds.
		if calls == 1 {
			return candidateErr
		}
		return nil
	}

	err := p.ApplyGatewaySites(candidate, prior)
	if err == nil {
		t.Fatal("expected non-nil error after candidate failure, got nil")
	}
	if errors.Is(err, ErrDiverged) {
		t.Errorf("got ErrDiverged but rollback succeeded; want a wrapped non-divergence error")
	}
	if !errors.Is(err, candidateErr) {
		t.Errorf("returned error does not wrap candidate error; got %v", err)
	}
	if calls != 2 {
		t.Errorf("reload called %d times, want 2 (candidate + rollback)", calls)
	}
	if !reflect.DeepEqual(seen[1], prior) {
		t.Errorf("rollback reload saw %v, want prior %v", seen[1], prior)
	}
	if !reflect.DeepEqual(p.GatewaySites(), prior) {
		t.Errorf("after rollback sites = %v, want prior %v", p.GatewaySites(), prior)
	}
}

func TestApplyGatewaySites_CandidateFailsRollbackFailsReturnsDiverged(t *testing.T) {
	prior := []GatewaySite{{Domain: "old.example.com", BackendURL: "http://10.0.0.1:8080", TLS: "auto"}}
	candidate := []GatewaySite{{Domain: "broken.example.com", BackendURL: "http://10.0.0.2:8080", TLS: "auto"}}

	p := newApplyTestProxy(prior)
	candidateErr := errors.New("caddy: candidate adapt failure")
	rollbackErr := errors.New("caddy: post-parse listener collision")

	calls := 0
	p.testReloadHook = func() error {
		calls++
		if calls == 1 {
			return candidateErr
		}
		return rollbackErr
	}

	err := p.ApplyGatewaySites(candidate, prior)
	if err == nil {
		t.Fatal("expected ErrDiverged, got nil")
	}
	if !errors.Is(err, ErrDiverged) {
		t.Errorf("returned error is not ErrDiverged: %v", err)
	}
	if calls != 2 {
		t.Errorf("reload called %d times, want 2", calls)
	}
}

func TestConfigGatewaySitesToProxy_EmptyAndPopulated(t *testing.T) {
	if got := ConfigGatewaySitesToProxy(nil); got != nil {
		t.Errorf("empty input -> %v, want nil", got)
	}
	in := []config.GatewaySite{
		{Domain: "a.example.com", BackendURL: "http://10.0.0.1:8080", TLS: config.TLSModeCustom, TLSCert: "/tmp/c", TLSKey: "/tmp/k", StripFrameBlockers: true, Streaming: true},
	}
	out := ConfigGatewaySitesToProxy(in)
	if len(out) != 1 {
		t.Fatalf("got %d entries, want 1", len(out))
	}
	if out[0].Domain != "a.example.com" || out[0].BackendURL != "http://10.0.0.1:8080" {
		t.Errorf("domain/backend mismatch: %+v", out[0])
	}
	if out[0].TLS != "custom" || out[0].TLSCert != "/tmp/c" || out[0].TLSKey != "/tmp/k" {
		t.Errorf("tls fields not copied: %+v", out[0])
	}
	if !out[0].StripFrameBlockers || !out[0].Streaming {
		t.Errorf("flags not copied: %+v", out[0])
	}
}

func TestSetTestReloadHook_ProductionPathIfNil(t *testing.T) {
	// reloadHookOrDefault must fall back to (*Proxy).Reload when no
	// hook is installed. Verifying via interface comparison is
	// awkward, but we can assert that the returned function is
	// non-nil for both states.
	p := New(&Config{ListenAddr: ":8080", InternalAddr: "127.0.0.1:18080"})
	if p.reloadHookOrDefault() == nil {
		t.Error("reloadHookOrDefault returned nil with no test hook")
	}
	called := false
	p.SetTestReloadHook(func() error { called = true; return nil })
	if err := p.reloadHookOrDefault()(); err != nil || !called {
		t.Errorf("test hook not invoked through reloadHookOrDefault; called=%v err=%v", called, err)
	}
}

package proxy

import (
	"errors"
	"fmt"

	"github.com/mescon/muximux/v3/internal/logging"
)

// ErrDiverged signals that a candidate Reload failed AND the rollback
// Reload that we issued to re-assert the prior shape ALSO failed.
// Caddy's running state is now indeterminate: it may be serving the
// failed candidate, the prior config, neither, or some half-applied
// mix. The caller MUST surface this to the operator (Settings banner)
// because the discovery service cannot recover automatically.
//
// Wrapped errors carry the underlying causes via errors.Is /
// errors.Unwrap so the caller can log details while still matching
// the sentinel.
var ErrDiverged = errors.New("caddy diverged: candidate reload failed and rollback reload also failed")

// ApplyGatewaySites swaps Caddy's running gateway-site configuration
// from priorSites to newSites with rollback semantics:
//
//  1. SetGatewaySites(newSites); Reload()
//  2. On Reload failure: SetGatewaySites(priorSites); Reload() to
//     re-assert what was running before the candidate.
//  3. If the rollback Reload also fails: return ErrDiverged so the
//     caller can surface the divergence (the discovery poller's
//     banner, or the gateway handler's 503-mismatch response).
//
// This function is the only reload path that's safe to call from a
// non-HTTP context (the discovery refresh poller) - the gateway
// handler's applyAndPersist writes a 503 to the response body when
// it diverges, which would panic on a poller-driven call. Both inputs
// are proxy.GatewaySite (NOT config.GatewaySite); callers translate
// via proxy.ConfigGatewaySitesToProxy.
//
// The function takes p.mu briefly via SetGatewaySites; the actual
// Caddy Reload runs without our lock so callers must serialise
// concurrent calls themselves (the gateway handler does this through
// configMu; the poller does too).
func (p *Proxy) ApplyGatewaySites(newSites, priorSites []GatewaySite) error {
	p.SetGatewaySites(newSites)
	if err := p.reloadHookOrDefault()(); err == nil {
		return nil
	} else {
		// Reload failed - try to roll Caddy back to the prior shape.
		logging.Warn("Caddy reload of candidate gateway sites failed; rolling back",
			"source", "caddy",
			"error", err)
		p.SetGatewaySites(priorSites)
		if rollbackErr := p.reloadHookOrDefault()(); rollbackErr != nil {
			logging.Error("Caddy rollback reload also failed; gateway is in indeterminate state",
				"source", "audit",
				"divergence_detected", true,
				"candidate_error", err,
				"rollback_error", rollbackErr)
			return fmt.Errorf("%w: candidate=%v rollback=%v", ErrDiverged, err, rollbackErr)
		}
		// Rollback succeeded; surface the original Reload error so
		// the caller knows the candidate was rejected. The running
		// shape is priorSites; in-memory + disk should match.
		return fmt.Errorf("caddy reload failed (rolled back to prior config): %w", err)
	}
}

// SetTestReloadHook installs a reload function that
// reloadHookOrDefault returns instead of (*Proxy).Reload. Cross-
// package tests use this to drive ApplyGatewaySites' decision tree
// without booting Caddy. Production callers MUST NOT use this; the
// "test" prefix on the field name is enforced socially, not by the
// type system.
func (p *Proxy) SetTestReloadHook(hook func() error) {
	p.testReloadHook = hook
}

// reloadHookOrDefault returns the test-injected reload function if set,
// otherwise the production Reload method. Centralised here so adding a
// new reload-driven helper later doesn't need to replicate the
// nil-check boilerplate.
func (p *Proxy) reloadHookOrDefault() func() error {
	if p.testReloadHook != nil {
		return p.testReloadHook
	}
	return p.Reload
}

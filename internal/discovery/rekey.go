package discovery

import (
	"sort"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// rekeyInput is what planRekeys works from: one tick's tracked set and
// enriched container list, plus every key already in use.
type rekeyInput struct {
	Endpoint   string             // the live endpoint; tracked entries on another endpoint are skipped (they await Re-link)
	Tracked    trackedSet         // from collectTracked(): every app and site with a DockerKey, any endpoint
	Containers []ContainerSummary // enriched and sorted by enrichSwarm
	HeldKeys   map[string]bool    // every key in Tracked (any endpoint) plus every quarantined key
}

// heldKeys returns every tracking key in use: each tracked app and site on
// any endpoint, and each quarantined entry. A re-key never targets one of
// them, so a migration cannot create a duplicate.
func heldKeys(tracked *trackedSet, quarantined []config.QuarantinedEntry) map[string]bool {
	out := map[string]bool{}
	for i := range tracked.apps {
		out[tracked.apps[i].key] = true
	}
	for i := range tracked.sites {
		out[tracked.sites[i].key] = true
	}
	for i := range quarantined {
		out[quarantined[i].Key] = true
	}
	return out
}

// swarmServiceFromTaskName returns the service part of a Swarm task name
// ("bindery_web.1.71e9k1i0wfiyk5sbbjku668er" -> "bindery_web"). A name
// without the task suffix (slot plus a 20+ character task ID) never
// matches, so a plain container name is never guessed to be a service.
func swarmServiceFromTaskName(name string) (string, bool) {
	loc := swarmTaskPattern.FindStringIndex(name)
	if len(loc) < 2 || loc[0] == 0 {
		return "", false
	}
	return name[:loc[0]], true
}

// planRekeys maps the old `name:` or `id:` key of each tracked app and site
// on the live endpoint to the stable key (`label:`, `swarm:` or
// `compose:`) of its container:
//
//   - the key still resolves and KeyForContainer on the matched container
//     yields a stable key: re-key to it;
//   - a `name:` key that no longer resolves but is a Swarm task name whose
//     service still runs: re-key to `swarm:<service>`.
//
// A target held by another entry (HeldKeys) or already planned for an
// earlier old key is skipped: first in config order (apps, then sites)
// wins and the loser keeps its key.
func planRekeys(in *rekeyInput) map[string]string {
	out := map[string]string{}
	planned := map[string]bool{}
	byService := map[string]bool{}
	for i := range in.Containers {
		if s := swarmServiceName(&in.Containers[i]); s != "" {
			byService[s] = true
		}
	}
	type entry struct{ key, endpoint string }
	entries := make([]entry, 0, len(in.Tracked.apps)+len(in.Tracked.sites))
	for i := range in.Tracked.apps {
		entries = append(entries, entry{in.Tracked.apps[i].key, in.Tracked.apps[i].endpoint})
	}
	for i := range in.Tracked.sites {
		entries = append(entries, entry{in.Tracked.sites[i].key, in.Tracked.sites[i].endpoint})
	}
	for _, e := range entries {
		old := e.key
		if _, done := out[old]; done || e.endpoint != in.Endpoint {
			continue
		}
		tk, err := ParseTrackingKey(old)
		if err != nil || (tk.Source != KeySourceName && tk.Source != KeySourceID) {
			continue
		}
		target := ""
		if c := tk.FindContainer(in.Containers); c != nil {
			if k, _ := KeyForContainer(c); k != old {
				if nk, _ := ParseTrackingKey(k); nk.Source == KeySourceLabel || nk.Source == KeySourceSwarm || nk.Source == KeySourceCompose {
					target = k
				}
			}
		} else if tk.Source == KeySourceName {
			if svc, ok := swarmServiceFromTaskName(tk.Value); ok && byService[svc] {
				target = "swarm:" + svc
			}
		}
		if target == "" || planned[target] || in.HeldKeys[target] {
			continue
		}
		out[old] = target
		planned[target] = true
	}
	return out
}

// applyRekeysToTracked rewrites the tick's tracked set and its reconcile
// snapshots through rekeys, so the resolve loop and Reconcile see the new
// keys and the new task is matched to the existing app instead of added.
// Only entries on the live endpoint are rewritten, matching planRekeys.
func applyRekeysToTracked(rekeys map[string]string, endpoint string, tracked *trackedSet, apps []config.AppConfig, sites []config.GatewaySite) {
	if len(rekeys) == 0 {
		return
	}
	for i := range tracked.apps {
		if nk, ok := rekeys[tracked.apps[i].key]; ok && tracked.apps[i].endpoint == endpoint {
			tracked.apps[i].key = nk
		}
	}
	for i := range tracked.sites {
		if nk, ok := rekeys[tracked.sites[i].key]; ok && tracked.sites[i].endpoint == endpoint {
			tracked.sites[i].key = nk
		}
	}
	for i := range apps {
		if nk, ok := rekeys[apps[i].DockerKey]; ok && apps[i].DockerKey != "" && apps[i].DockerEndpoint == endpoint {
			apps[i].DockerKey = nk
		}
	}
	for i := range sites {
		if nk, ok := rekeys[sites[i].DockerKey]; ok && sites[i].DockerKey != "" && sites[i].DockerEndpoint == endpoint {
			sites[i].DockerKey = nk
		}
	}
}

// applyRekeysToConfig renames DockerKey in place on the live apps and
// sites of the batch's endpoint. The caller holds the write lock and has
// snapshotted apps, sites and the quarantine for rollback. It returns the
// re-keys that renamed at least one entry (only those move Service records
// after the save) and false when a target key is now held by an entry the
// plan did not see (an edit between the tick's snapshot and this commit):
// the caller then rolls back and saves nothing, and the next tick replans.
func applyRekeysToConfig(cfg *config.Config, rekeys map[string]string, endpoint string) (applied map[string]string, ok bool) {
	if len(rekeys) == 0 {
		return nil, true
	}
	held := map[string]bool{}
	for i := range cfg.Apps {
		held[cfg.Apps[i].DockerKey] = true
	}
	for i := range cfg.Server.GatewaySites {
		held[cfg.Server.GatewaySites[i].DockerKey] = true
	}
	for _, nk := range rekeys {
		if held[nk] || cfg.HasQuarantined(nk) {
			return nil, false
		}
	}
	applied = map[string]string{}
	for i := range cfg.Apps {
		a := &cfg.Apps[i]
		if nk, found := rekeys[a.DockerKey]; found && a.DockerKey != "" && a.DockerEndpoint == endpoint {
			applied[a.DockerKey] = nk
			a.DockerKey = nk
		}
	}
	for i := range cfg.Server.GatewaySites {
		s := &cfg.Server.GatewaySites[i]
		if nk, found := rekeys[s.DockerKey]; found && s.DockerKey != "" && s.DockerEndpoint == endpoint {
			applied[s.DockerKey] = nk
			s.DockerKey = nk
		}
	}
	return applied, true
}

// finishRekeys runs after a successful save: it moves the Service's
// last-seen and missing records and the sync absence counter of each old
// key to its new key, and writes one audit line per re-key. It is never
// called on a rolled-back tick, so those records stay with the old keys
// the config was restored to.
func (p *Poller) finishRekeys(rekeyed map[string]string) {
	olds := make([]string, 0, len(rekeyed))
	for old := range rekeyed {
		olds = append(olds, old)
	}
	sort.Strings(olds)
	for _, old := range olds {
		nk := rekeyed[old]
		if p.deps.Service != nil {
			p.deps.Service.RenameTrackedKey(old, nk)
		}
		if n, ok := p.syncAbsent[old]; ok {
			delete(p.syncAbsent, old)
			if _, taken := p.syncAbsent[nk]; !taken {
				p.syncAbsent[nk] = n
			}
		}
		logging.Info("Docker tracking key migrated", "source", "audit", "old_key", old, "new_key", nk)
	}
}

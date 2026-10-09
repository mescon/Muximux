package discovery

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// Label re-sync of tracked apps (#500).
//
// While an app is tracked (DockerKey set) and its container is present,
// the display labels that are SET on the container win: muximux.app.name,
// .icon, .group and .order. A label that is not set never overwrites the
// operator's value. Detaching the app (which clears DockerKey) hands the
// fields back to the operator.
//
// Ownership per tick, so no field is written twice: an auto-imported app
// (DockerAutoImported) belongs to Reconcile, which re-syncs every managed
// field under update/sync and leaves it alone under add/off, exactly as
// before. Every other tracked app (a manual import, or an app detached from
// auto-import that is still tracked) belongs to this pass, whatever the
// auto_import mode. The pass only updates: it never adds or removes an
// entry, so it is safe with auto_import off. The URL and health address of
// every tracked app keep coming from the refresh pass.

// labelSync is what one tracked app takes from its labels this tick. A zero
// field means the label is unset or already in sync.
type labelSync struct {
	name  string // new app name
	icon  string // new dashboard icon slug
	group string // canonical name of an existing group
	order int    // new order (a label order of 0 means unset)
}

func (s *labelSync) empty() bool {
	return s.name == "" && s.icon == "" && s.group == "" && s.order == 0
}

// labelSyncContext is the part of the config the plan checks against,
// snapshotted under the read lock: how many apps use each name key (so a
// rename never collides with another app) and the configured group names.
type labelSyncContext struct {
	nameCounts map[string]int
	groups     []string
}

// snapshotLabelSyncContext captures the label re-sync context. The caller
// holds ConfigMu (read or write).
func snapshotLabelSyncContext(cfg *config.Config) labelSyncContext {
	ctx := labelSyncContext{nameCounts: make(map[string]int, len(cfg.Apps))}
	for i := range cfg.Apps {
		ctx.nameCounts[nameKey(cfg.Apps[i].Name)]++
	}
	for i := range cfg.Groups {
		ctx.groups = append(ctx.groups, cfg.Groups[i].Name)
	}
	return ctx
}

// resolveLabelGroup maps a group label to a configured group: an exact
// name match first, then a match by slug (so "infra" finds "Infra"). It
// returns false when no group matches. Creating a missing group is left to
// the group auto-create step; until then the app keeps its current group,
// so a label can never move an app into a group the UI does not list.
func resolveLabelGroup(groups []string, label string) (string, bool) {
	for _, g := range groups {
		if g == label {
			return g, true
		}
	}
	slug := config.Slugify(label)
	if slug == "" {
		return "", false
	}
	for _, g := range groups {
		if config.Slugify(g) == slug {
			return g, true
		}
	}
	return "", false
}

// labelIconSynced reports whether icon already shows the dashboard icon
// slug. The import path stores a label icon as {type: dashboard, name}.
func labelIconSynced(icon *config.AppIconConfig, slug string) bool {
	return icon.Type == "dashboard" && icon.Name == slug
}

// reserveReconcileNames counts the names auto-import adds or updates this tick
// into ctx, so a label rename cannot take a name Reconcile just claimed.
func reserveReconcileNames(ctx *labelSyncContext, batch *refreshBatch) {
	for i := range batch.addApps {
		ctx.nameCounts[nameKey(batch.addApps[i].Name)]++
	}
	for i := range batch.updateApps {
		ctx.nameCounts[nameKey(batch.updateApps[i].Name)]++
	}
}

// planLabelSync decides, for every tracked app Reconcile does not own whose
// container is present this tick, which set labels differ from the stored
// values. Apps are visited in key order so two renames that want the same
// name resolve the same way every tick (the lower key wins). A label that
// cannot be applied (the name is taken, the group does not exist, the name
// is too long) is held: the stored value is kept and the reason is logged
// once per transition.
func (p *Poller) planLabelSync(apps []trackedAppEntry, containers []ContainerSummary, endpoint string, ctx *labelSyncContext, batch *refreshBatch) map[string]labelSync {
	if ctx.nameCounts == nil {
		ctx.nameCounts = map[string]int{}
	}
	reserveReconcileNames(ctx, batch)

	idx := make([]int, 0, len(apps))
	for i := range apps {
		if apps[i].autoImported || apps[i].key == "" || apps[i].endpoint != endpoint {
			continue
		}
		idx = append(idx, i)
	}
	sort.Slice(idx, func(a, b int) bool { return apps[idx[a]].key < apps[idx[b]].key })

	out := map[string]labelSync{}
	for _, i := range idx {
		t := &apps[i]
		tk, err := ParseTrackingKey(t.key)
		if err != nil {
			continue // the refresh pass reports a malformed key
		}
		c := tk.FindContainer(containers)
		if c == nil {
			continue // absent this tick: keep everything as stored
		}
		labels := ParseAppLabels(c.Labels)
		s, held := planOneLabelSync(t, &labels, ctx)
		p.noteLabelHeld(t, strings.Join(held, "; "))
		if !s.empty() {
			out[t.key] = s
		}
	}
	return out
}

// planOneLabelSync compares one app's set labels with its stored values and
// returns the changes plus the reasons any set label was held back. On an
// accepted rename it moves the name in ctx, so later apps see it as taken.
func planOneLabelSync(t *trackedAppEntry, labels *AppLabels, ctx *labelSyncContext) (s labelSync, held []string) {
	if labels.Name != "" && labels.Name != t.name {
		newKey, oldKey := nameKey(labels.Name), nameKey(t.name)
		others := ctx.nameCounts[newKey]
		if newKey == oldKey {
			others-- // a case-only rename: the app itself holds the key
		}
		switch {
		case utf8.RuneCountInString(labels.Name) > 100:
			held = append(held, "name label is longer than 100 characters")
		case others > 0:
			held = append(held, "name label "+labels.Name+" is already used by another app")
		default:
			s.name = labels.Name
			ctx.nameCounts[oldKey]--
			ctx.nameCounts[newKey]++
		}
	}
	if labels.Icon != "" && !labelIconSynced(&t.icon, labels.Icon) {
		s.icon = labels.Icon
	}
	if labels.Group != "" {
		if g, ok := resolveLabelGroup(ctx.groups, labels.Group); !ok {
			held = append(held, "group label "+labels.Group+" does not match a configured group")
		} else if g != t.group {
			s.group = g
		}
	}
	if labels.Order != 0 && labels.Order != t.order {
		s.order = labels.Order
	}
	return s, held
}

// noteLabelHeld logs a held label once per (key, reason) transition and
// clears the record once nothing is held.
func (p *Poller) noteLabelHeld(t *trackedAppEntry, reason string) {
	if reason == "" {
		delete(p.labelHeld, t.key)
		return
	}
	if p.labelHeld[t.key] == reason {
		return
	}
	if p.labelHeld == nil {
		p.labelHeld = map[string]string{}
	}
	p.labelHeld[t.key] = reason
	logging.Warn("Docker label not applied to tracked app; stored value kept",
		"source", "discovery", "app", t.name, "key", t.key, "reason", reason)
}

// labelSynced records one app the label re-sync changed, logged only after
// the save succeeds.
type labelSynced struct {
	name, key string
	fields    []string
}

// applyLabelSyncs writes the planned label values onto the live apps. The
// caller holds the write lock and has snapshotted apps, sites and the
// quarantine for rollback. Ownership is checked again under the lock: an
// app that became auto-imported or lost its tracking since the plan is
// skipped, and a group deleted since the plan is not applied. A rename
// carries any gateway site linked by app_name along, as a Settings rename
// does.
func applyLabelSyncs(cfg *config.Config, syncs map[string]labelSync) []labelSynced {
	if len(syncs) == 0 {
		return nil
	}
	groups := make([]string, 0, len(cfg.Groups))
	for i := range cfg.Groups {
		groups = append(groups, cfg.Groups[i].Name)
	}
	var out []labelSynced
	for i := range cfg.Apps {
		a := &cfg.Apps[i]
		if a.DockerKey == "" || a.DockerAutoImported {
			continue
		}
		s, ok := syncs[a.DockerKey]
		if !ok {
			continue
		}
		if fields := applyOneLabelSync(cfg, a, &s, groups); len(fields) > 0 {
			out = append(out, labelSynced{name: a.Name, key: a.DockerKey, fields: fields})
		}
	}
	return out
}

// applyOneLabelSync applies one plan entry to a and returns the names of
// the fields it changed.
func applyOneLabelSync(cfg *config.Config, a *config.AppConfig, s *labelSync, groups []string) []string {
	var fields []string
	if s.name != "" && s.name != a.Name {
		for j := range cfg.Server.GatewaySites {
			if cfg.Server.GatewaySites[j].AppName == a.Name {
				cfg.Server.GatewaySites[j].AppName = s.name
			}
		}
		a.Name = s.name
		fields = append(fields, "name")
	}
	if s.icon != "" && !labelIconSynced(&a.Icon, s.icon) {
		// Same shape as an import: a dashboard icon by slug. Styling the
		// operator set on the icon (variant, colour, background, invert)
		// is kept.
		a.Icon.Type = "dashboard"
		a.Icon.Name = s.icon
		a.Icon.File = ""
		a.Icon.URL = ""
		fields = append(fields, "icon")
	}
	if s.group != "" && s.group != a.Group {
		if g, ok := resolveLabelGroup(groups, s.group); ok {
			a.Group = g
			fields = append(fields, "group")
		}
	}
	if s.order != 0 && s.order != a.Order {
		a.Order = s.order
		fields = append(fields, "order")
	}
	return fields
}

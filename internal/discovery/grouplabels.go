package discovery

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// Group labels (#500).
//
// muximux.group.icon, .color and .order on a container describe the group
// its app is in. They apply only to a group Docker discovery created
// (GroupConfig.DockerManaged): when the group is created and on every
// later tick while it stays managed. A group the operator made, or one
// they edited in Settings (which clears the marker), is never touched.
// A label that is not set never overwrites a value.
//
// Several containers whose apps share a group may disagree. Per field the
// container with the lowest tracking key wins, so the result does not
// depend on container order; the others get a scan note and a warning.

// GroupLabels is the parsed shape of the muximux.group.* namespace.
type GroupLabels struct {
	Icon     string // dashboard-icons slug, trimmed and lowercased; "" = unset
	Color    string // hex colour; "" = unset or invalid
	Order    int
	OrderSet bool // true when muximux.group.order holds a valid value
	// Invalid lists, in label order, the set values that were ignored and
	// why, for the scan notes and the once-per-transition warning.
	Invalid []string
}

// any reports whether at least one group label holds a usable value.
func (g *GroupLabels) any() bool {
	return g.Icon != "" || g.Color != "" || g.OrderSet
}

// ParseGroupLabels extracts the muximux.group.* namespace from a
// container's label map. Blank values count as unset. A colour must be a
// hex colour (#rgb, #rgba, #rrggbb or #rrggbbaa, as the group colour
// picker writes) and an order a whole number from 0 to 9999, the range of
// muximux.app.order; anything else is ignored and listed in Invalid.
func ParseGroupLabels(labels map[string]string) GroupLabels {
	var out GroupLabels
	if v := strings.TrimSpace(labels[LabelGroupIcon]); v != "" {
		out.Icon = strings.ToLower(v)
	}
	if v := strings.TrimSpace(labels[LabelGroupColor]); v != "" {
		if isHexColor(v) {
			out.Color = v
		} else {
			out.Invalid = append(out.Invalid, fmt.Sprintf("%s %q is not a hex colour such as #3b82f6", LabelGroupColor, v))
		}
	}
	if v := strings.TrimSpace(labels[LabelGroupOrder]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 9999 {
			out.Order, out.OrderSet = n, true
		} else {
			out.Invalid = append(out.Invalid, fmt.Sprintf("%s %q is not a whole number from 0 to 9999", LabelGroupOrder, v))
		}
	}
	return out
}

// groupOwner is a tracked entry whose container's group labels apply to
// group: a tracked app in that group, or a tracked gateway site whose
// linked app is in it.
type groupOwner struct {
	key, group string
}

// groupOwners lists, sorted by key and then group, the tracked entries on
// endpoint whose apps are in a group. A key appears once per group, even
// when an app and its gateway site share it. The caller holds ConfigMu
// (read or write).
func groupOwners(apps []config.AppConfig, sites []config.GatewaySite, endpoint string) []groupOwner {
	seen := map[groupOwner]bool{}
	var out []groupOwner
	add := func(key, group string) {
		o := groupOwner{key: key, group: group}
		if key == "" || group == "" || seen[o] {
			return
		}
		seen[o] = true
		out = append(out, o)
	}
	appGroup := make(map[string]string, len(apps))
	for i := range apps {
		a := &apps[i]
		appGroup[a.Name] = a.Group
		if a.DockerEndpoint == endpoint {
			add(a.DockerKey, a.Group)
		}
	}
	for i := range sites {
		s := &sites[i]
		if s.DockerEndpoint == endpoint && s.AppName != "" {
			add(s.DockerKey, appGroup[s.AppName])
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].key != out[b].key {
			return out[a].key < out[b].key
		}
		return out[a].group < out[b].group
	})
	return out
}

// groupLabelLookup returns the group labels of the container a tracking
// key finds in containers, parsed once per key. ok is false when the key
// is malformed or no container matches.
func groupLabelLookup(containers []ContainerSummary) func(key string) (GroupLabels, bool) {
	cache := map[string]*GroupLabels{}
	return func(key string) (GroupLabels, bool) {
		if g, ok := cache[key]; ok {
			if g == nil {
				return GroupLabels{}, false
			}
			return *g, true
		}
		cache[key] = nil
		tk, err := ParseTrackingKey(key)
		if err != nil {
			return GroupLabels{}, false
		}
		c := tk.FindContainer(containers)
		if c == nil {
			return GroupLabels{}, false
		}
		g := ParseGroupLabels(c.Labels)
		cache[key] = &g
		return g, true
	}
}

// groupLabelValues is what the labels say one group should look like.
// An empty icon or colour, or a nil order, means no container sets it.
// The *Key fields name the winning container's tracking key.
type groupLabelValues struct {
	icon, color       string
	order             *int
	iconKey, colorKey string
	orderKey          string
}

// groupLabelConflict is one container whose group label lost to the
// container with a lower tracking key.
type groupLabelConflict struct {
	group, label, key, winner string
}

// resolveGroupLabels folds the labels of owners (sorted by key) into one
// set of values per group. Per field the first owner that sets it, the
// lowest key, wins; a later owner that sets a different value is
// reported as a conflict.
func resolveGroupLabels(owners []groupOwner, lookup func(string) (GroupLabels, bool)) (map[string]groupLabelValues, []groupLabelConflict) {
	out := map[string]groupLabelValues{}
	var conflicts []groupLabelConflict
	for _, o := range owners {
		gl, ok := lookup(o.key)
		if !ok {
			continue
		}
		v := out[o.group] // present for every group with a container found
		if !gl.any() {
			out[o.group] = v
			continue
		}
		lose := func(label, winner string) {
			conflicts = append(conflicts, groupLabelConflict{group: o.group, label: label, key: o.key, winner: winner})
		}
		if gl.Icon != "" {
			switch {
			case v.iconKey == "":
				v.icon, v.iconKey = gl.Icon, o.key
			case v.icon != gl.Icon:
				lose(LabelGroupIcon, v.iconKey)
			}
		}
		if gl.Color != "" {
			switch {
			case v.colorKey == "":
				v.color, v.colorKey = gl.Color, o.key
			case !strings.EqualFold(v.color, gl.Color):
				lose(LabelGroupColor, v.colorKey)
			}
		}
		if gl.OrderSet {
			switch {
			case v.orderKey == "":
				n := gl.Order
				v.order, v.orderKey = &n, o.key
			case *v.order != gl.Order:
				lose(LabelGroupOrder, v.orderKey)
			}
		}
		out[o.group] = v
	}
	return out, conflicts
}

// groupSynced records one group the labels changed, logged only after the
// save succeeds.
type groupSynced struct {
	name   string
	fields []string
}

// applyGroupLabelValues writes vals onto every DockerManaged group in
// groups (in place) and returns what it changed. A label icon is stored as
// a dashboard icon by slug, the shape muximux.app.icon gives an app;
// styling on the icon (variant, colour, background, invert) is kept.
// DockerOrder is set on every pass to whether a label sets the order, and
// a change to it counts as a change ("order_source").
func applyGroupLabelValues(groups []config.GroupConfig, vals map[string]groupLabelValues) []groupSynced {
	var out []groupSynced
	for i := range groups {
		g := &groups[i]
		v, ok := vals[g.Name]
		if !g.DockerManaged || !ok {
			continue
		}
		var fields []string
		if v.icon != "" && (g.Icon.Type != "dashboard" || g.Icon.Name != v.icon || g.Icon.File != "" || g.Icon.URL != "") {
			g.Icon.Type, g.Icon.Name, g.Icon.File, g.Icon.URL = "dashboard", v.icon, "", ""
			fields = append(fields, "icon")
		}
		if v.color != "" && g.Color != v.color {
			g.Color = v.color
			fields = append(fields, "color")
		}
		if v.order != nil && g.Order != *v.order {
			g.Order = *v.order
			fields = append(fields, "order")
		}
		if fromLabel := v.order != nil; g.DockerOrder != fromLabel {
			g.DockerOrder = fromLabel
			fields = append(fields, "order_source")
		}
		if len(fields) > 0 {
			out = append(out, groupSynced{name: g.Name, fields: fields})
		}
	}
	return out
}

// syncGroupLabels applies the group labels of the containers in batch to
// the DockerManaged groups of cfg, after the label pass and the reconcile
// plan have created and filled this tick's groups. The caller holds the
// write lock and has snapshotted the groups for rollback.
func syncGroupLabels(cfg *config.Config, batch *refreshBatch) []groupSynced {
	if len(batch.containers) == 0 {
		return nil
	}
	owners := groupOwners(cfg.Apps, cfg.Server.GatewaySites, batch.endpoint)
	vals, _ := resolveGroupLabels(owners, groupLabelLookup(batch.containers))
	return applyGroupLabelValues(cfg.Groups, vals)
}

// groupLabelSnapshot is what planGroupLabels needs from the config,
// captured under the read lock: a copy of the groups and the owners.
type groupLabelSnapshot struct {
	groups []config.GroupConfig
	owners []groupOwner
}

// snapshotGroupLabels captures the group label context. The caller holds
// ConfigMu (read or write).
func snapshotGroupLabels(cfg *config.Config, endpoint string) groupLabelSnapshot {
	return groupLabelSnapshot{
		groups: append([]config.GroupConfig(nil), cfg.Groups...),
		owners: groupOwners(cfg.Apps, cfg.Server.GatewaySites, endpoint),
	}
}

// planGroupLabels reports whether the labels of containers would change a
// DockerManaged group of snap, so a tick with nothing else to do still
// commits a group label change. It also logs, once per transition, the
// invalid group labels of each tracked container and the conflicts on
// each managed group. Groups this tick creates are not in snap; the batch
// already commits them, and syncGroupLabels fills them in the same save.
func (p *Poller) planGroupLabels(snap *groupLabelSnapshot, containers []ContainerSummary) bool {
	lookup := groupLabelLookup(containers)
	invalid := map[string]string{}
	for _, o := range snap.owners {
		if gl, ok := lookup(o.key); ok && len(gl.Invalid) > 0 {
			invalid[o.key] = strings.Join(gl.Invalid, "; ")
		}
	}
	for k, reason := range invalid {
		if p.groupLabelInvalid[k] != reason {
			logging.Warn("Docker group label ignored: invalid value",
				"source", "discovery", "key", k, "reason", reason)
		}
	}
	p.groupLabelInvalid = invalid

	vals, conflicts := resolveGroupLabels(snap.owners, lookup)
	managed := make(map[string]bool, len(snap.groups))
	for i := range snap.groups {
		managed[snap.groups[i].Name] = snap.groups[i].DockerManaged
	}
	conflictNotes := map[string]string{}
	for _, c := range conflicts {
		if !managed[c.group] {
			continue
		}
		n := fmt.Sprintf("%s of %s ignored; using %s", c.label, c.key, c.winner)
		if prev := conflictNotes[c.group]; prev != "" {
			n = prev + "; " + n
		}
		conflictNotes[c.group] = n
	}
	for g, reason := range conflictNotes {
		if p.groupLabelConflict[g] != reason {
			logging.Warn("Docker group labels disagree; the container with the lowest tracking key wins",
				"source", "discovery", "group", g, "reason", reason)
		}
	}
	p.groupLabelConflict = conflictNotes

	groups := append([]config.GroupConfig(nil), snap.groups...)
	return len(applyGroupLabelValues(groups, vals)) > 0
}

// noteGroupLabels adds the scan notes for the group labels of one
// suggestion: values that were ignored as invalid, and labels that cannot
// apply because the app has no group.
func noteGroupLabels(s *Suggestion, gl *GroupLabels) {
	for _, r := range gl.Invalid {
		s.Notes = append(s.Notes, "Group label ignored: "+r)
	}
	if gl.any() && s.Group == "" {
		s.Notes = append(s.Notes, "muximux.group.* labels ignored: the app has no group (set muximux.app.group)")
	}
}

// noteGroupLabelConflicts adds a scan note to every suggestion whose group
// label disagrees with the one a suggestion with a lower key sets for the
// same group (matched by name, then slug). The lowest key wins.
func noteGroupLabelConflicts(suggestions []Suggestion) {
	idx := make([]int, 0, len(suggestions))
	for i := range suggestions {
		if suggestions[i].Group != "" && suggestions[i].groupLabels.any() {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool { return suggestions[idx[a]].Key < suggestions[idx[b]].Key })
	var names []string
	owners := make([]groupOwner, 0, len(idx))
	byKey := make(map[string]GroupLabels, len(idx))
	keyIdx := make(map[string]int, len(idx))
	for _, i := range idx {
		s := &suggestions[i]
		g, ok := config.MatchGroupName(names, s.Group)
		if !ok {
			g = s.Group
			names = append(names, g)
		}
		owners = append(owners, groupOwner{key: s.Key, group: g})
		byKey[s.Key] = s.groupLabels
		keyIdx[s.Key] = i
	}
	_, conflicts := resolveGroupLabels(owners, func(k string) (GroupLabels, bool) {
		g, ok := byKey[k]
		return g, ok
	})
	for _, c := range conflicts {
		s := &suggestions[keyIdx[c.key]]
		s.Notes = append(s.Notes, fmt.Sprintf("%s ignored: %s sets a different value for group %s and has the lower key", c.label, c.winner, c.group))
	}
}

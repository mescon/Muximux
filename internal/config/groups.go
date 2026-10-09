package config

import "strings"

// MatchGroupName maps name to one of the configured group names: an exact
// match first, then a match by slug, so "media " or "media" finds "Media"
// and discovery never creates a near-duplicate of an existing group. It
// returns false when nothing matches.
func MatchGroupName(names []string, name string) (string, bool) {
	for _, g := range names {
		if g == name {
			return g, true
		}
	}
	slug := Slugify(name)
	if slug == "" {
		return "", false
	}
	for _, g := range names {
		if Slugify(g) == slug {
			return g, true
		}
	}
	return "", false
}

// NewAutoGroup builds the group Docker discovery creates for a name an app
// references but the config does not define. The defaults match a group
// made in Settings: a lucide folder icon, no colour, expanded. It is
// marked DockerManaged, so the muximux.group.* labels apply to it until
// the operator edits it in Settings.
func NewAutoGroup(name string, order int) GroupConfig {
	return GroupConfig{
		Name:          name,
		Icon:          AppIconConfig{Type: "lucide", Name: "folder"},
		Order:         order,
		Expanded:      true,
		DockerManaged: true,
	}
}

// GroupStyleEdited reports whether b changes the label-managed fields of
// a: the icon, the colour or the order. A save that does releases a
// DockerManaged group to the operator.
func GroupStyleEdited(a, b *GroupConfig) bool {
	return a.Icon != b.Icon || a.Color != b.Color || a.Order != b.Order
}

// EnsureGroup returns the configured group name an app with group name
// should use, appending a new group to groups when none matches (by name,
// then by slug). A new group is placed after every existing one: its order
// is the group count, as CreateGroup assigns, or one past the highest
// order when the existing orders are sparse. An empty or blank name means
// ungrouped and returns "". created reports whether a group was appended.
func EnsureGroup(groups []GroupConfig, name string) (out []GroupConfig, canonical string, created bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return groups, "", false
	}
	names := make([]string, len(groups))
	nextOrder := len(groups)
	for i := range groups {
		names[i] = groups[i].Name
		if groups[i].Order >= nextOrder {
			nextOrder = groups[i].Order + 1
		}
	}
	if g, ok := MatchGroupName(names, name); ok {
		return groups, g, false
	}
	if g, ok := MatchGroupName(names, trimmed); ok {
		return groups, g, false
	}
	return append(groups, NewAutoGroup(trimmed, nextOrder)), trimmed, true
}

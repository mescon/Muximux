package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mescon/muximux/v3/internal/config"
)

// serverOwnedAppFields are ClientAppConfig fields a save never sets from
// the payload: identity transport, computed display hints and Docker
// tracking. mergeFields always keeps theirs for them.
var serverOwnedAppFields = map[string]bool{
	"OriginalName": true, "ProxyURL": true, "GatewayDomain": true,
	"DockerKey": true, "DockerEndpoint": true, "DockerStrategy": true, "DockerManagedURL": true,
}

// topLevelSkip lists the ClientConfigUpdate fields mergeThreeWay merges
// with a dedicated rule instead of the per-field scalar rule.
var topLevelSkip = map[string]bool{
	"Base": true, "Navigation": true, "Theme": true, "Health": true,
	"Keybindings": true, "Groups": true, "Apps": true,
}

// groupSkip keeps the transport-only identity out of the per-field merge.
var groupSkip = map[string]bool{"OriginalName": true}

// appIdentity is the name an app had in base: OriginalName when the client
// sent it, else Name.
func appIdentity(a *ClientAppConfig) string {
	if a.OriginalName != "" {
		return a.OriginalName
	}
	return a.Name
}

// groupIdentity is the name a group had in base: OriginalName when the
// client sent it, else Name.
func groupIdentity(g *config.GroupConfig) string {
	if g.OriginalName != "" {
		return g.OriginalName
	}
	return g.Name
}

// wireBytes is the JSON form of v with an empty slice or map treated as
// nil, so "[]" and "absent" compare equal the way omitempty and the
// client's deepEqual treat them.
func wireBytes(v reflect.Value) []byte {
	if !v.IsValid() {
		return []byte("null")
	}
	if (v.Kind() == reflect.Slice || v.Kind() == reflect.Map) && v.Len() == 0 {
		return []byte("null")
	}
	b, err := json.Marshal(v.Interface())
	if err != nil {
		return []byte(fmt.Sprintf("%#v", v.Interface()))
	}
	return b
}

// jsonEqual compares a and b by their wire form. Struct fields tagged
// omitempty drop nil and empty slices or maps alike, so an untouched app
// whose client copy carries "allowed_groups: []" equals the stored one.
func jsonEqual(a, b any) bool {
	return bytes.Equal(wireBytes(reflect.ValueOf(a)), wireBytes(reflect.ValueOf(b)))
}

// mergeFields copies theirs and overwrites every exported field where mine
// differs from base. Fields named in skip are always theirs.
func mergeFields[T any](base, mine, theirs *T, skip map[string]bool) T {
	out := *theirs
	bv := reflect.ValueOf(base).Elem()
	mv := reflect.ValueOf(mine).Elem()
	ov := reflect.ValueOf(&out).Elem()
	t := ov.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || skip[f.Name] {
			continue
		}
		if !bytes.Equal(wireBytes(mv.Field(i)), wireBytes(bv.Field(i))) {
			ov.Field(i).Set(mv.Field(i))
		}
	}
	return out
}

// mergeHealth merges the optional health block. A payload without one
// keeps the server's; a side that has none takes the payload whole.
func mergeHealth(base, mine, theirs *config.HealthConfig) *config.HealthConfig {
	switch {
	case mine == nil:
		return theirs
	case base == nil || theirs == nil:
		return mine
	}
	out := mergeFields(base, mine, theirs, nil)
	return &out
}

func bindingsOf(k *config.KeybindingsConfig) map[string][]config.KeyCombo {
	if k == nil || k.Bindings == nil {
		return map[string][]config.KeyCombo{}
	}
	return k.Bindings
}

// mergeKeybindings merges per action: an action mine changed wins, an
// action mine removed from base is deleted, every other action is theirs.
func mergeKeybindings(base, mine, theirs *config.KeybindingsConfig) *config.KeybindingsConfig {
	if mine == nil {
		return theirs
	}
	b, m, th := bindingsOf(base), bindingsOf(mine), bindingsOf(theirs)
	out := map[string][]config.KeyCombo{}
	for action := range th {
		out[action] = th[action]
	}
	for action := range m {
		if !jsonEqual(m[action], b[action]) {
			out[action] = m[action]
		}
	}
	for action := range b {
		if _, still := m[action]; !still {
			delete(out, action) // deleted by the user
		}
	}
	return &config.KeybindingsConfig{Bindings: out}
}

// stripServerOwned returns a copy of a with every server-owned field
// cleared, for items a save keeps or creates without a server match.
func stripServerOwned(a *ClientAppConfig) ClientAppConfig {
	out := *a
	out.OriginalName, out.ProxyURL, out.GatewayDomain = "", "", ""
	out.DockerKey, out.DockerEndpoint, out.DockerStrategy, out.DockerManagedURL = "", "", "", ""
	return out
}

// appsEqual reports whether a and b agree on every user-editable field.
func appsEqual(a, b *ClientAppConfig) bool {
	sa, sb := stripServerOwned(a), stripServerOwned(b)
	return jsonEqual(&sa, &sb)
}

func indexApps(apps []ClientAppConfig) map[string]*ClientAppConfig {
	m := make(map[string]*ClientAppConfig, len(apps))
	for i := range apps {
		m[apps[i].Name] = &apps[i]
	}
	return m
}

// mergeApps merges the app lists. mine is matched to base by appIdentity,
// base to theirs by name with a docker-key fallback for the server's own
// renames. Merged and server-added apps carry OriginalName = theirs' name
// so the preservation step finds the stored app; apps without a server
// match carry no server-owned fields.
func mergeApps(base, mine, theirs []ClientAppConfig) []ClientAppConfig {
	baseByName := indexApps(base)
	theirsByName := indexApps(theirs)
	theirsByKey := map[string]*ClientAppConfig{}
	baseKeys := map[string]bool{}
	for i := range theirs {
		if k := theirs[i].DockerKey; k != "" {
			theirsByKey[k] = &theirs[i]
		}
	}
	for i := range base {
		if k := base[i].DockerKey; k != "" {
			baseKeys[k] = true
		}
	}
	resolveTheirs := func(id string) *ClientAppConfig {
		if t, ok := theirsByName[id]; ok {
			return t
		}
		if b, ok := baseByName[id]; ok && b.DockerKey != "" {
			return theirsByKey[b.DockerKey]
		}
		return nil
	}
	consumed := map[*ClientAppConfig]bool{}
	out := make([]ClientAppConfig, 0, len(mine)+len(theirs))
	for i := range mine {
		m := &mine[i]
		id := appIdentity(m)
		b := baseByName[id]
		t := resolveTheirs(id)
		switch {
		case b == nil && t == nil: // brand new; a save never creates tracking
			out = append(out, stripServerOwned(m))
		case b == nil: // new in mine, name collides with a server-added app
			a := mergeFields(&ClientAppConfig{}, m, t, serverOwnedAppFields)
			a.OriginalName = t.Name
			consumed[t] = true
			out = append(out, a)
		case t == nil: // removed by the server
			if appsEqual(m, b) {
				continue
			}
			out = append(out, stripServerOwned(m))
		default:
			a := mergeFields(b, m, t, serverOwnedAppFields)
			a.OriginalName = t.Name
			consumed[t] = true
			out = append(out, a)
		}
	}
	for i := range theirs {
		t := &theirs[i]
		if consumed[t] {
			continue
		}
		if _, inBase := baseByName[t.Name]; inBase || (t.DockerKey != "" && baseKeys[t.DockerKey]) {
			continue // the user deleted it
		}
		a := *t
		a.OriginalName = t.Name
		out = append(out, a)
	}
	return out
}

func indexGroups(groups []config.GroupConfig) map[string]*config.GroupConfig {
	m := make(map[string]*config.GroupConfig, len(groups))
	for i := range groups {
		m[groups[i].Name] = &groups[i]
	}
	return m
}

// groupsEqual compares two groups ignoring the transport-only identity.
func groupsEqual(a, b *config.GroupConfig) bool {
	ca, cb := *a, *b
	ca.OriginalName, cb.OriginalName = "", ""
	return jsonEqual(&ca, &cb)
}

// mergeGroups merges the group lists with the same rules as mergeApps,
// matching base to theirs by name only. Merged and server-added groups
// carry OriginalName = theirs' name; cascadeGroupRenames uses it to
// re-point apps after the merge.
func mergeGroups(base, mine, theirs []config.GroupConfig) []config.GroupConfig {
	baseByName := indexGroups(base)
	theirsByName := indexGroups(theirs)
	consumed := map[*config.GroupConfig]bool{}
	out := make([]config.GroupConfig, 0, len(mine)+len(theirs))
	for i := range mine {
		m := &mine[i]
		id := groupIdentity(m)
		b := baseByName[id]
		t := theirsByName[id]
		switch {
		case b == nil && t == nil: // brand new
			g := *m
			g.OriginalName = ""
			out = append(out, g)
		case b == nil: // new in mine, name collides with a server-added group
			g := mergeFields(&config.GroupConfig{}, m, t, groupSkip)
			g.OriginalName = t.Name
			consumed[t] = true
			out = append(out, g)
		case t == nil: // removed by the server
			if groupsEqual(m, b) {
				continue
			}
			g := *m
			g.OriginalName = ""
			out = append(out, g)
		default:
			g := mergeFields(b, m, t, groupSkip)
			g.OriginalName = t.Name
			consumed[t] = true
			out = append(out, g)
		}
	}
	for i := range theirs {
		t := &theirs[i]
		if consumed[t] {
			continue
		}
		if _, inBase := baseByName[t.Name]; inBase {
			continue // the user deleted it
		}
		g := *t
		g.OriginalName = t.Name
		out = append(out, g)
	}
	return out
}

// cascadeGroupRenames re-points apps that still carry a renamed group's old
// name. The one place this rule lives; mergeConfigUpdate calls it once for
// both the two-way and the three-way path.
func cascadeGroupRenames(groups []config.GroupConfig, apps []ClientAppConfig) {
	renamed := map[string]string{}
	for i := range groups {
		if groups[i].OriginalName != "" && groups[i].OriginalName != groups[i].Name {
			renamed[groups[i].OriginalName] = groups[i].Name
		}
	}
	for i := range apps {
		if n, ok := renamed[apps[i].Group]; ok {
			apps[i].Group = n
		}
	}
}

// mergeThreeWay merges base (what the client loaded), mine (the payload)
// and theirs (the server's current config) per field. It does not cascade
// group renames; merged groups carry OriginalName = theirs' name.
func mergeThreeWay(base, mine, theirs *ClientConfigUpdate) *ClientConfigUpdate {
	out := mergeFields(base, mine, theirs, topLevelSkip)
	out.Navigation = mergeFields(&base.Navigation, &mine.Navigation, &theirs.Navigation, nil)
	out.Theme = mergeFields(&base.Theme, &mine.Theme, &theirs.Theme, nil)
	out.Health = mergeHealth(base.Health, mine.Health, theirs.Health)
	out.Keybindings = mergeKeybindings(base.Keybindings, mine.Keybindings, theirs.Keybindings)
	out.Groups = mergeGroups(base.Groups, mine.Groups, theirs.Groups)
	out.Apps = mergeApps(base.Apps, mine.Apps, theirs.Apps)
	out.Base = nil
	return &out
}

// updateFromResponse renders the server's client-shaped config as an
// update, the "theirs" side of mergeThreeWay.
func updateFromResponse(resp *clientConfigResponse) *ClientConfigUpdate {
	return &ClientConfigUpdate{
		Title:               resp.Title,
		Language:            resp.Language,
		LogLevel:            resp.LogLevel,
		ProxyTimeout:        resp.ProxyTimeout,
		SessionCookieDomain: resp.SessionCookieDomain,
		Navigation:          resp.Navigation,
		Theme:               resp.Theme,
		Health:              resp.Health,
		Keybindings:         resp.Keybindings,
		Groups:              resp.Groups,
		Apps:                resp.Apps,
	}
}

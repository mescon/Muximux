package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/mescon/muximux/v3/internal/logging"
)

// quarantineWarn is the message logged for every quarantined entry, both at
// load and when main.go re-emits the list after logging.Init.
const quarantineWarn = "Invalid Docker auto-imported entry quarantined; it is kept in config.yaml but not loaded"

// QuarantinedEntry describes one docker-owned entry that failed validation
// and was set aside instead of failing the load.
type QuarantinedEntry struct {
	Kind   string `json:"kind"`   // "app" | "gateway"
	Name   string `json:"name"`   // app name or site domain
	Key    string `json:"key"`    // DockerKey
	Reason string `json:"reason"` // validation error
}

// quarantined holds a quarantined entry together with a copy of the app or
// site it came from, so Save can write it back to config.yaml unchanged.
type quarantined struct {
	entry QuarantinedEntry
	app   *AppConfig
	site  *GatewaySite
}

// isDockerOwnedApp reports whether a is owned by Docker auto-import: tracked
// by a container key and created by the poller rather than by the operator.
func isDockerOwnedApp(a *AppConfig) bool { return a.DockerKey != "" && a.DockerAutoImported }

// QuarantineApp records a copy of a as quarantined and logs the WARN. It does
// not touch c.Apps; the caller removes the app from the live list.
func (c *Config) QuarantineApp(a *AppConfig, reason string) {
	cp := *a
	c.quarantined = append(c.quarantined, quarantined{
		entry: QuarantinedEntry{Kind: "app", Name: a.Name, Key: a.DockerKey, Reason: reason},
		app:   &cp,
	})
	logging.Warn(quarantineWarn, "source", "config", "kind", "app", "name", a.Name, "key", a.DockerKey, "reason", reason)
}

// QuarantineSite records a copy of s as quarantined and logs the WARN. It
// does not touch c.Server.GatewaySites.
func (c *Config) QuarantineSite(s *GatewaySite, reason string) {
	cp := *s
	c.quarantined = append(c.quarantined, quarantined{
		entry: QuarantinedEntry{Kind: "gateway", Name: s.Domain, Key: s.DockerKey, Reason: reason},
		site:  &cp,
	})
	logging.Warn(quarantineWarn, "source", "config", "kind", "gateway", "name", s.Domain, "key", s.DockerKey, "reason", reason)
}

// LogQuarantined re-emits the quarantine WARN. config.Load runs before
// logging.Init in main.go, so the lines QuarantineApp/QuarantineSite log at
// load go to the pre-init default logger only; main.go calls this after Init
// so they reach muximux.log and the log viewer.
func LogQuarantined(entries []QuarantinedEntry) int {
	for i := range entries {
		e := &entries[i]
		logging.Warn(quarantineWarn, "source", "config", "kind", e.Kind, "name", e.Name, "key", e.Key, "reason", e.Reason)
	}
	return len(entries)
}

// Quarantined returns a copy of the quarantined entries, in load order.
func (c *Config) Quarantined() []QuarantinedEntry {
	out := make([]QuarantinedEntry, 0, len(c.quarantined))
	for i := range c.quarantined {
		out = append(out, c.quarantined[i].entry)
	}
	return out
}

// HasQuarantined reports whether any quarantined app or site has key.
func (c *Config) HasQuarantined(key string) bool {
	for i := range c.quarantined {
		if c.quarantined[i].entry.Key == key {
			return true
		}
	}
	return false
}

// DropQuarantined removes every quarantined entry (app and site) with key
// and returns how many were removed.
func (c *Config) DropQuarantined(key string) int {
	kept := make([]quarantined, 0, len(c.quarantined))
	n := 0
	for i := range c.quarantined {
		if c.quarantined[i].entry.Key == key {
			n++
			continue
		}
		kept = append(kept, c.quarantined[i])
	}
	c.quarantined = kept
	return n
}

// QuarantineSnapshot returns a copy of the quarantine list for a caller that
// may need to roll back with RestoreQuarantine.
func (c *Config) QuarantineSnapshot() []quarantined {
	return append([]quarantined(nil), c.quarantined...)
}

// RestoreQuarantine puts back a list taken with QuarantineSnapshot.
func (c *Config) RestoreQuarantine(snap []quarantined) { c.quarantined = snap }

// pruneSupersededQuarantine drops every quarantined entry a live entry has
// taken over, and returns how many it dropped. A live app with the same name
// or slug as a quarantined app supersedes it: the operator has resolved the
// conflict, and writing both back would leave two apps with one name (or one
// proxy path) in config.yaml. A quarantined site is superseded by a live site
// with the same domain, or when its app was superseded. Save runs it through
// fileView, under the caller's write lock, so memory and the file agree.
func (c *Config) pruneSupersededQuarantine() int {
	if len(c.quarantined) == 0 {
		return 0
	}
	liveNames := map[string]bool{}
	liveSlugs := map[string]bool{}
	for i := range c.Apps {
		liveNames[c.Apps[i].Name] = true
		if s := Slugify(c.Apps[i].Name); s != "" {
			liveSlugs[s] = true
		}
	}
	liveDomains := map[string]bool{}
	for i := range c.Server.GatewaySites {
		liveDomains[strings.ToLower(c.Server.GatewaySites[i].Domain)] = true
	}
	supersededKeys := map[string]bool{}
	for i := range c.quarantined {
		q := &c.quarantined[i]
		if q.app != nil && (liveNames[q.app.Name] || liveSlugs[Slugify(q.app.Name)]) && q.app.DockerKey != "" {
			supersededKeys[q.app.DockerKey] = true
		}
	}
	kept := make([]quarantined, 0, len(c.quarantined))
	for i := range c.quarantined {
		q := &c.quarantined[i]
		var superseded bool
		switch {
		case q.app != nil:
			superseded = liveNames[q.app.Name] || liveSlugs[Slugify(q.app.Name)]
		case q.site != nil:
			superseded = liveDomains[strings.ToLower(q.site.Domain)] ||
				(q.site.DockerKey != "" && supersededKeys[q.site.DockerKey])
		}
		if superseded {
			logging.Info("Quarantined entry superseded by a live entry; dropped from config.yaml",
				"source", "config", "kind", q.entry.Kind, "name", q.entry.Name, "key", q.entry.Key)
			continue
		}
		kept = append(kept, *q)
	}
	n := len(c.quarantined) - len(kept)
	c.quarantined = kept
	return n
}

// quarantinedApps returns copies of the quarantined apps for fileView,
// after dropping the entries a live entry superseded.
func (c *Config) quarantinedApps() []AppConfig {
	c.pruneSupersededQuarantine()
	var out []AppConfig
	for i := range c.quarantined {
		if c.quarantined[i].app != nil {
			out = append(out, *c.quarantined[i].app)
		}
	}
	return out
}

// quarantinedSites returns copies of the quarantined gateway sites for
// fileView, after dropping the entries a live entry superseded.
func (c *Config) quarantinedSites() []GatewaySite {
	c.pruneSupersededQuarantine()
	var out []GatewaySite
	for i := range c.quarantined {
		if c.quarantined[i].site != nil {
			out = append(out, *c.quarantined[i].site)
		}
	}
	return out
}

// gatedSiteReason mirrors validateSessionCookieDomain for one site: a site
// with require_auth needs server.session_cookie_domain set and its domain
// under it. Empty means the site passes.
func gatedSiteReason(s *GatewaySite, srv *ServerConfig) string {
	if !s.RequireAuth {
		return ""
	}
	if srv.SessionCookieDomain == "" {
		return "require_auth is set but server.session_cookie_domain is empty"
	}
	parent := strings.TrimPrefix(srv.SessionCookieDomain, ".")
	if parent != "" && !hostIsUnderParent(s.Domain, parent) {
		return fmt.Sprintf("domain %q is not under server.session_cookie_domain %q", s.Domain, srv.SessionCookieDomain)
	}
	return ""
}

// DockerSiteReason returns why a docker-owned gateway site on its own would
// be quarantined at load: the per-site checks (domain, backend URL,
// collision with server.tls.domain, self-loop) and the require_auth /
// session_cookie_domain rule. Cross-site checks (duplicate domains, app_name
// links) are not covered. Empty means the site passes. srv may be nil.
func DockerSiteReason(s *GatewaySite, srv *ServerConfig) string {
	if err := validateGatewaySite(s, srv); err != nil {
		return err.Error()
	}
	if srv == nil {
		return ""
	}
	return gatedSiteReason(s, srv)
}

// quarantineReason is the single source of the app rule, shared by load and
// SaveConfig: ValidateApp, then the slug collision against the enabled apps
// kept so far (slugs maps slug -> first kept name). An empty result means
// the app may stay live.
func quarantineReason(a *AppConfig, slugs map[string]string) string {
	if err := ValidateApp(a); err != nil {
		return err.Error()
	}
	if s := Slugify(a.Name); a.Enabled && s != "" {
		if first, dup := slugs[s]; dup {
			return fmt.Sprintf("app %q has the same slug %q as %q", a.Name, s, first)
		}
	}
	return ""
}

// rememberSlug records a's slug in slugs when a is enabled, keeping the
// first name seen for a slug.
func rememberSlug(slugs map[string]string, a *AppConfig) {
	if s := Slugify(a.Name); a.Enabled && s != "" {
		if _, dup := slugs[s]; !dup {
			slugs[s] = a.Name
		}
	}
}

// quarantineInvalidDockerEntries moves docker-owned apps and their gateway
// sites that would fail validate() out of the live lists. Anything else is
// left for validate() to reject, so an operator's own invalid config still
// fails the load loudly.
func quarantineInvalidDockerEntries(cfg *Config) {
	// Seed the slug map with the operator's own apps first, so a
	// docker-owned app listed before a colliding manual app is the one
	// set aside instead of the manual app failing validate().
	slugs := map[string]string{}
	for i := range cfg.Apps {
		if !isDockerOwnedApp(&cfg.Apps[i]) {
			rememberSlug(slugs, &cfg.Apps[i])
		}
	}
	keptApps := make([]AppConfig, 0, len(cfg.Apps))
	for i := range cfg.Apps {
		a := &cfg.Apps[i]
		if isDockerOwnedApp(a) {
			if reason := quarantineReason(a, slugs); reason != "" {
				cfg.QuarantineApp(a, reason)
				continue
			}
		}
		rememberSlug(slugs, a)
		keptApps = append(keptApps, *a)
	}
	cfg.Apps = keptApps
	settleQuarantinedSites(cfg)
}

// sameSavedApp reports whether a, an app after a SaveConfig merge, is the
// stored app p unchanged. It ignores the differences the merge itself makes
// whatever the payload says: an empty map or slice comes back nil (JSON
// omitempty), and DockerManagedURL is reset to URL. Order is ignored too:
// a drag-reorder in Settings is not an edit of the app.
func sameSavedApp(a, p *AppConfig) bool {
	return reflect.DeepEqual(normalizedForCompare(a), normalizedForCompare(p))
}

// normalizedForCompare returns a copy of a with the merge-made differences
// sameSavedApp ignores folded away.
func normalizedForCompare(a *AppConfig) AppConfig {
	n := *a
	n.DockerManagedURL = ""
	n.Order = 0
	if len(n.HTTPActionHeaders) == 0 {
		n.HTTPActionHeaders = nil
	}
	if len(n.ProxyHeaders) == 0 {
		n.ProxyHeaders = nil
	}
	if len(n.AllowedGroups) == 0 {
		n.AllowedGroups = nil
	}
	if len(n.Permissions) == 0 {
		n.Permissions = nil
	}
	return n
}

// QuarantineUnchangedInvalidDockerApps quarantines every docker-owned app
// that fails ValidateApp on its own and is identical to the app with the
// same DockerKey in prior, the list held before this save. An app the
// payload changed is never quarantined, so the operator's own mistakes
// still fail validation loudly. Unlike the load rule, a slug or name
// collision never quarantines here: on save a clash is caused by the
// operator's edit, and quarantining the docker app would let the next
// Save prune it as superseded, so the clash falls through to Validate's
// 400. It then applies the shared site rule and returns how many apps it
// quarantined.
func (c *Config) QuarantineUnchangedInvalidDockerApps(prior []AppConfig) int {
	priorByKey := map[string]*AppConfig{}
	for i := range prior {
		if isDockerOwnedApp(&prior[i]) {
			priorByKey[prior[i].DockerKey] = &prior[i]
		}
	}
	kept := make([]AppConfig, 0, len(c.Apps))
	n := 0
	for i := range c.Apps {
		a := &c.Apps[i]
		if p := priorByKey[a.DockerKey]; isDockerOwnedApp(a) && p != nil && sameSavedApp(a, p) {
			if err := ValidateApp(a); err != nil {
				c.QuarantineApp(a, err.Error())
				n++
				continue
			}
		}
		kept = append(kept, *a)
	}
	c.Apps = kept
	if n > 0 {
		settleQuarantinedSites(c)
	}
	return n
}

// settleQuarantinedSites is the single source of the site rule: a
// docker-owned site of a quarantined app, or one that fails validation
// (including the require_auth / session_cookie_domain rule), is
// quarantined; a manual site whose app_name names a quarantined app has
// that app_name cleared with a WARN. A name a live app still holds is never
// treated as quarantined.
func settleQuarantinedSites(cfg *Config) {
	// A name or key a live app still holds belongs to that app, so it
	// never counts as quarantined: a site linked to it keeps its link.
	liveNames := map[string]bool{}
	liveKeys := map[string]bool{}
	ownedKeys := map[string]bool{}
	for i := range cfg.Apps {
		liveNames[cfg.Apps[i].Name] = true
		if cfg.Apps[i].DockerKey != "" {
			liveKeys[cfg.Apps[i].DockerKey] = true
		}
		if isDockerOwnedApp(&cfg.Apps[i]) {
			ownedKeys[cfg.Apps[i].DockerKey] = true
		}
	}
	quarantinedKeys := map[string]bool{}
	quarantinedNames := map[string]bool{}
	for i := range cfg.quarantined {
		if q := &cfg.quarantined[i]; q.app != nil {
			if !liveKeys[q.entry.Key] {
				quarantinedKeys[q.entry.Key] = true
			}
			if !liveNames[q.entry.Name] {
				quarantinedNames[q.entry.Name] = true
			}
		}
	}
	ownedSite := func(s *GatewaySite) bool {
		return s.DockerKey != "" && (ownedKeys[s.DockerKey] || quarantinedKeys[s.DockerKey])
	}
	// Seed the domain set with the operator's own sites, so a docker-owned
	// site listed before a manual site with the same domain is the one set
	// aside.
	domains := map[string]bool{}
	for i := range cfg.Server.GatewaySites {
		if s := &cfg.Server.GatewaySites[i]; !ownedSite(s) {
			domains[strings.ToLower(s.Domain)] = true
		}
	}
	keptSites := make([]GatewaySite, 0, len(cfg.Server.GatewaySites))
	for i := range cfg.Server.GatewaySites {
		s := &cfg.Server.GatewaySites[i]
		owned := ownedSite(s)
		reason := ""
		switch {
		case !owned:
			if s.AppName != "" && quarantinedNames[s.AppName] {
				logging.Warn(fmt.Sprintf("Gateway site %s was linked to quarantined app %s; link removed. "+
					"Fix the app in Discovery and re-link the site", s.Domain, s.AppName),
					"source", "config", "domain", s.Domain, "app_name", s.AppName)
				s.AppName = ""
			}
		case quarantinedKeys[s.DockerKey] || (s.AppName != "" && quarantinedNames[s.AppName]):
			reason = "its app is quarantined"
		case domains[strings.ToLower(s.Domain)]:
			reason = fmt.Sprintf("duplicate domain %q", s.Domain)
		default:
			if err := validateGatewaySites([]GatewaySite{*s}, cfg); err != nil {
				reason = err.Error()
			} else {
				reason = gatedSiteReason(s, &cfg.Server)
			}
		}
		if reason != "" {
			cfg.QuarantineSite(s, reason)
			continue
		}
		domains[strings.ToLower(s.Domain)] = true
		keptSites = append(keptSites, *s)
	}
	cfg.Server.GatewaySites = keptSites
}

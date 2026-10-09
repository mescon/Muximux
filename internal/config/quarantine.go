package config

import (
	"fmt"
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

// quarantinedApps returns copies of the quarantined apps, for fileView.
func (c *Config) quarantinedApps() []AppConfig {
	var out []AppConfig
	for i := range c.quarantined {
		if c.quarantined[i].app != nil {
			out = append(out, *c.quarantined[i].app)
		}
	}
	return out
}

// quarantinedSites returns copies of the quarantined gateway sites, for
// fileView.
func (c *Config) quarantinedSites() []GatewaySite {
	var out []GatewaySite
	for i := range c.quarantined {
		if c.quarantined[i].site != nil {
			out = append(out, *c.quarantined[i].site)
		}
	}
	return out
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

// settleQuarantinedSites is the single source of the site rule: a
// docker-owned site of a quarantined app, or one that fails validation, is
// quarantined; a manual site whose app_name names a quarantined app has
// that app_name cleared with a WARN.
func settleQuarantinedSites(cfg *Config) {
	quarantinedKeys := map[string]bool{}
	quarantinedNames := map[string]bool{}
	for i := range cfg.quarantined {
		if cfg.quarantined[i].app != nil {
			quarantinedKeys[cfg.quarantined[i].entry.Key] = true
			quarantinedNames[cfg.quarantined[i].entry.Name] = true
		}
	}
	ownedKeys := map[string]bool{}
	for i := range cfg.Apps {
		if isDockerOwnedApp(&cfg.Apps[i]) {
			ownedKeys[cfg.Apps[i].DockerKey] = true
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
				logging.Warn("Gateway site app_name cleared: its app is quarantined",
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

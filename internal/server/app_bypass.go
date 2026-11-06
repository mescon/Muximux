package server

import (
	"net"
	"strings"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/handlers"
	"github.com/mescon/muximux/v3/internal/logging"
)

// appBypassRules turns each proxied app's auth_bypass rules into
// middleware rules on full request paths under /proxy/<slug>/.
//
// A rule's path is relative to the app ("/api/*" on Sonarr covers
// /proxy/sonarr/api/*). A path that already starts with the app's own
// /proxy/<slug>/ prefix is taken as is. A rule is skipped, with a
// warning, when it could open anything outside its app: an empty path
// (which the matcher treats as "everything"), a path under another
// app's prefix, or a path with dot segments. Apps that are disabled or
// not proxied get no rules, since their traffic never passes through
// /proxy/.
func appBypassRules(apps []config.AppConfig) []auth.BypassRule {
	var out []auth.BypassRule
	for i := range apps {
		app := &apps[i]
		if len(app.AuthBypass) == 0 {
			continue
		}
		if !app.Proxy || !app.Enabled {
			logging.Warn("Ignoring auth_bypass on an app that is not proxied and enabled",
				"source", "auth", "app", app.Name)
			continue
		}
		slug := handlers.Slugify(app.Name)
		for j := range app.AuthBypass {
			rule := &app.AuthBypass[j]
			path, ok := appBypassPath(slug, rule.Path)
			if !ok {
				logging.Warn("Ignoring auth_bypass rule: the path must stay inside the app",
					"source", "auth", "app", app.Name, "path", rule.Path)
				continue
			}
			for _, ip := range rule.AllowedIPs {
				if net.ParseIP(ip) == nil {
					if _, _, err := net.ParseCIDR(ip); err != nil {
						logging.Warn("auth_bypass allowed_ips entry is not an IP or CIDR and never matches",
							"source", "auth", "app", app.Name, "path", rule.Path, "entry", ip)
					}
				}
			}
			out = append(out, auth.BypassRule{
				Path:          path,
				Methods:       append([]string(nil), rule.Methods...),
				RequireAPIKey: rule.RequireAPIKey,
				AllowedIPs:    append([]string(nil), rule.AllowedIPs...),
			})
		}
	}
	return out
}

// appBypassPath resolves a rule path against the app's /proxy/<slug>
// prefix and reports whether the result stays inside that app.
func appBypassPath(slug, rulePath string) (string, bool) {
	p := strings.TrimSpace(rulePath)
	if p == "" || slug == "" {
		return "", false
	}
	prefix := proxyPathPrefix + slug // "/proxy/sonarr"
	if !strings.HasPrefix(p, proxyPathPrefix) {
		if !strings.HasPrefix(p, "/") && p != "*" {
			p = "/" + p
		}
		p = prefix + p
	}
	// "*" alone, or "/proxy/sonarr*", means the whole app. Anchor it on
	// the slash so it cannot also match /proxy/sonarr2/.
	if p == prefix+"*" {
		p = prefix + "/*"
	}
	if p != prefix && !strings.HasPrefix(p, prefix+"/") {
		return "", false
	}
	if hasDotSegment(strings.TrimSuffix(p, "*")) {
		return "", false
	}
	return p, true
}

package discovery

import (
	"reflect"

	"github.com/mescon/muximux/v3/internal/config"
)

// Desired is the config Muximux wants for one labeled container: the
// app plus an optional gateway site when the container declares a
// gateway domain. A nil Site means the container routes directly or via
// the built-in path-prefix proxy and needs no Caddy site.
type Desired struct {
	App  config.AppConfig
	Site *config.GatewaySite
}

// BuildDesired maps a label-derived Suggestion to the AppConfig (and
// optional GatewaySite) that auto-import should materialize, deriving
// routing from labels:
//
//   - SuggestedDomain set -> a gateway site fronts the container. The
//     App.URL becomes the public domain (https unless tls=none), the
//     App is NOT proxy-routed, and the GatewaySite forwards to the
//     container URL. This mirrors handlers.ImportDocker's RoutingGateway
//     branch for URL/Proxy/BackendURL.
//   - else Proxy label true -> App.Proxy true (built-in reverse proxy).
//   - else -> direct App pointing at the container URL.
//
// Tracking is stamped on every path so the auto-import reconciler owns
// the entry and config.Load() can detect an operator hand-edit cleanly:
// DockerManagedURL is seeded to the value stored in the matching URL
// field (App.URL for the app, Site.BackendURL for the site), which is
// the baseline detachIfHandEdited / autoDetachEditedDockerEntries
// compare against.
//
// Divergence from ImportDocker, by design: manual gateway import detaches
// the App (only the site is tracked) so the operator's gateway URL is
// never rewritten. Auto-import instead keeps the App tracked (DockerKey
// set, DockerAutoImported true) so the reconciler can remove the app
// when the container disappears. The App.URL is the static gateway
// domain and DockerManagedURL equals it, so no spurious detach or URL
// rewrite results.
func BuildDesired(sug *Suggestion, endpoint string) Desired {
	app := config.AppConfig{
		Name:          sug.Name,
		URL:           sug.URL,
		HealthURL:     sug.HealthURL,
		Icon:          config.AppIconConfig{Type: "dashboard", Name: sug.Icon},
		Color:         sug.Color,
		Group:         sug.Group,
		Order:         sug.Order,
		Enabled:       true,
		OpenMode:      sug.OpenMode,
		MinRole:       sug.MinRole,
		AllowedGroups: sug.AllowedGroups,
		Permissions:   sug.Permissions,
		// Tracking. DockerManagedURL is set after routing decides the
		// final App.URL (gateway routing rewrites it to the domain).
		DockerKey:          sug.Key,
		DockerEndpoint:     endpoint,
		DockerStrategy:     string(sug.EffectiveStrategy),
		DockerAutoImported: true,
	}

	if sug.Proxy != nil {
		app.Proxy = *sug.Proxy
	}
	app.ProxySkipTLSVerify = sug.ProxySkipTLSVerify
	if sug.AllowNotifications != nil {
		app.AllowNotifications = *sug.AllowNotifications
	}
	if sug.Default != nil {
		app.Default = *sug.Default
	}
	if sug.Shortcut != 0 {
		s := sug.Shortcut
		app.Shortcut = &s
	}
	app.HTTPActionMethod = sug.HTTPActionMethod
	app.HTTPActionHeaders = sug.HTTPActionHeaders
	if sug.HTTPActionConfirm != nil {
		app.HTTPActionConfirm = *sug.HTTPActionConfirm
	}
	app.HTTPActionShowToast = sug.HTTPActionShowToast

	d := Desired{}
	if sug.SuggestedDomain != "" {
		d.Site = buildGatewaySite(sug, endpoint)
		// Gateway routing: the menu loads via the public hostname, so
		// App.URL is the domain and the app is not proxy-routed. The
		// gateway site forwards to the container URL (Site.BackendURL).
		app.URL = gatewayScheme(d.Site.TLS) + "://" + sug.SuggestedDomain
		app.Proxy = false
		d.Site.AppName = app.Name
	}

	// Seed the clean-detach baseline to the final App.URL (the container
	// URL for direct/proxy, the gateway domain for gateway routing).
	app.DockerManagedURL = app.URL
	d.App = app
	return d
}

// buildGatewaySite mirrors the config.GatewaySite that a manual import
// produces for the same container: domain from the label, backend
// pointing at the container URL, the muximux.gateway.* fields copied
// through, and the tracking + clean-detach baseline stamped.
func buildGatewaySite(sug *Suggestion, endpoint string) *config.GatewaySite {
	site := config.GatewaySite{
		Domain:     sug.SuggestedDomain,
		BackendURL: sug.URL,
		// Tracking. BackendURL is the URL the reconciler refreshes, so
		// it is also the clean-detach baseline.
		DockerKey:        sug.Key,
		DockerEndpoint:   endpoint,
		DockerStrategy:   string(sug.EffectiveStrategy),
		DockerManagedURL: sug.URL,
	}
	if gw := sug.SuggestedGateway; gw != nil {
		site.TLS = config.TLSMode(gw.TLS)
		if gw.Streaming != nil {
			site.Streaming = *gw.Streaming
		}
		if gw.StripFrameBlockers != nil {
			site.StripFrameBlockers = *gw.StripFrameBlockers
		}
		site.ForwardedHeaders = gw.ForwardedHeaders
		if gw.RequireAuth != nil {
			site.RequireAuth = *gw.RequireAuth
		}
		site.MinRole = gw.MinRole
		site.AllowedGroups = gw.AllowedGroups
	}
	// ProxyHeaders, TLSCert, and TLSKey are intentionally left unset:
	// there is no muximux.gateway.* label vocabulary for them, so auto-
	// import cannot populate them (matches the manual import path).
	return &site
}

// gatewayScheme picks the App.URL scheme for a gateway-fronted app:
// https by default (Caddy issues a cert for auto/custom sites), http
// only when the site is served without TLS. Mirrors ImportDocker.
func gatewayScheme(tls config.TLSMode) string {
	if tls == config.TLSModeNone {
		return "http"
	}
	return "https"
}

// ReconcilePlan is the set of changes auto-import wants to apply this
// tick: apps to create, auto-imported apps to refresh in place, and the
// DockerKeys of auto-imported apps whose containers have vanished.
type ReconcilePlan struct {
	Add        []Desired
	Update     []Desired
	RemoveKeys []string
}

// Reconcile diffs the desired set (from currently labeled containers)
// against the current config apps by DockerKey. It is a pure function:
// no config mutation, no I/O, output depends only on its inputs.
//
//   - off mode: empty plan, no work.
//   - desired key with no current app: Add (every non-off mode).
//   - desired key whose current app is auto-imported and differs in a
//     label-controlled field: Update (update/sync only, never add mode).
//   - desired key whose current app exists but is not auto-imported
//     (manual or detached): left untouched; it still suppresses the Add
//     so no duplicate app is created.
//   - sync only: a current auto-imported app whose DockerKey is absent
//     from the desired set has its key listed in RemoveKeys. Apps without
//     DockerAutoImported are never updated or removed.
func Reconcile(mode config.AutoImportMode, desired []Desired, current []config.AppConfig) ReconcilePlan {
	var plan ReconcilePlan
	if mode == config.AutoImportOff {
		return plan
	}

	curByKey := make(map[string]config.AppConfig, len(current))
	for i := range current {
		if current[i].DockerKey != "" {
			curByKey[current[i].DockerKey] = current[i]
		}
	}
	desiredKeys := make(map[string]bool, len(desired))

	for i := range desired {
		k := desired[i].App.DockerKey
		desiredKeys[k] = true
		cur, exists := curByKey[k]
		switch {
		case !exists:
			plan.Add = append(plan.Add, desired[i])
		case cur.DockerAutoImported && mode != config.AutoImportAdd && !sameManagedFields(&cur, &desired[i].App):
			plan.Update = append(plan.Update, desired[i])
		default:
			// Manual/detached app, add mode, or unchanged auto app:
			// leave the current entry as-is.
		}
	}

	if mode == config.AutoImportSync {
		for i := range current {
			if current[i].DockerAutoImported && current[i].DockerKey != "" && !desiredKeys[current[i].DockerKey] {
				plan.RemoveKeys = append(plan.RemoveKeys, current[i].DockerKey)
			}
		}
	}
	return plan
}

// sameManagedFields reports whether two apps agree on every
// label-controlled field that BuildDesired sets, i.e. the fields
// auto-import owns. It deliberately ignores tracking bookkeeping
// (DockerKey/Endpoint/Strategy/ManagedURL/AutoImported) and unrelated
// operator state (AuthBypass, Access, HealthCheck, Scale, ProxyHeaders,
// ForceIconBackground) so that a hand-set field elsewhere on an
// auto-imported app never triggers a spurious Update. A blanket
// reflect.DeepEqual over the whole AppConfig would compare those unowned
// fields and report false differences, so the comparison is explicit.
func sameManagedFields(a, b *config.AppConfig) bool {
	return a.Name == b.Name &&
		a.URL == b.URL &&
		a.HealthURL == b.HealthURL &&
		a.Icon == b.Icon &&
		a.Color == b.Color &&
		a.Group == b.Group &&
		a.Order == b.Order &&
		a.Enabled == b.Enabled &&
		a.Default == b.Default &&
		a.OpenMode == b.OpenMode &&
		a.Proxy == b.Proxy &&
		a.MinRole == b.MinRole &&
		a.AllowNotifications == b.AllowNotifications &&
		a.HTTPActionMethod == b.HTTPActionMethod &&
		a.HTTPActionConfirm == b.HTTPActionConfirm &&
		reflect.DeepEqual(a.ProxySkipTLSVerify, b.ProxySkipTLSVerify) &&
		reflect.DeepEqual(a.Shortcut, b.Shortcut) &&
		reflect.DeepEqual(a.HTTPActionShowToast, b.HTTPActionShowToast) &&
		reflect.DeepEqual(a.AllowedGroups, b.AllowedGroups) &&
		reflect.DeepEqual(a.Permissions, b.Permissions) &&
		reflect.DeepEqual(a.HTTPActionHeaders, b.HTTPActionHeaders)
}

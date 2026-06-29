package discovery

import "github.com/mescon/muximux/v3/internal/config"

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
func BuildDesired(sug Suggestion, endpoint string) Desired {
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
func buildGatewaySite(sug Suggestion, endpoint string) *config.GatewaySite {
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

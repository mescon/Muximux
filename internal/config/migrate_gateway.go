// Package config: Caddyfile-to-gateway_sites converter.
//
// 3.1.0 removed the `server.gateway:` Caddyfile path in favour of the
// declarative `server.gateway_sites:` YAML. This file holds the pure
// conversion logic so both the `muximux migrate-gateway` CLI
// subcommand and the runtime auto-migration in Load() can share it.
//
// MigrateCaddyfileToSites is the only exported entry point. Everything
// else is package-private.

package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	_ "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile" // Register Caddyfile adapter
	_ "github.com/caddyserver/caddy/v2/modules/standard"
)

// MigrateCaddyfileToSites parses the given Caddyfile bytes and returns
// the structured GatewaySite list it represents, plus any per-site or
// per-directive warnings encountered along the way. The function is
// pure (no I/O, no os.Exit), so tests and the runtime auto-migration
// can drive it directly.
func MigrateCaddyfileToSites(src []byte) ([]GatewaySite, []string, error) {
	return migrateCaddyfileToSites(src)
}

// migrateCaddyfileToSites is the in-package implementation kept for
// the existing tests (which call the unexported name).
func migrateCaddyfileToSites(src []byte) ([]GatewaySite, []string, error) {
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		return nil, nil, fmt.Errorf("caddyfile adapter not registered")
	}
	jsonBytes, _, err := adapter.Adapt(src, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("caddyfile error: %w", err)
	}

	var root struct {
		Apps struct {
			HTTP struct {
				Servers map[string]caddyServer `json:"servers"`
			} `json:"http"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, nil, fmt.Errorf("unexpected Caddy JSON shape: %w", err)
	}

	// Iterate servers in deterministic order so the YAML output is
	// stable across runs (helpful when an operator diffs successive
	// migrations against each other).
	serverNames := make([]string, 0, len(root.Apps.HTTP.Servers))
	for name := range root.Apps.HTTP.Servers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)

	var (
		sites    []GatewaySite
		warnings []string
	)
	for _, name := range serverNames {
		srv := root.Apps.HTTP.Servers[name]
		// An empty listen list means Caddy's default applies (port 443
		// with auto-HTTPS, unless `auto_https off` was set globally).
		// We can't tell from the adapted JSON whether the operator
		// wanted plain HTTP or HTTPS in that case, so emit a warning
		// so the operator double-checks the resulting tls field.
		if len(srv.Listen) == 0 {
			warnings = append(warnings, fmt.Sprintf("server %q has no explicit listen; assuming auto-HTTPS for its sites - verify the tls mode for each site after import", name))
		}
		listenIsHTTPOnly := serverListenIsHTTPOnly(srv.Listen)
		for i := range srv.Routes {
			site, ws := routeToSite(&srv.Routes[i], listenIsHTTPOnly)
			warnings = append(warnings, ws...)
			if site != nil {
				sites = append(sites, *site)
			}
		}
	}

	// Sort by domain so duplicate-host warnings are easy to spot in the
	// output and the YAML diff stays stable.
	sort.Slice(sites, func(i, j int) bool { return sites[i].Domain < sites[j].Domain })

	return sites, warnings, nil
}

// caddyServer mirrors the subset of the Caddy JSON server type the
// migrator reads. The full type lives in caddyhttp; we only need
// listen + routes, so we redeclare the fields rather than depend on
// caddy's internal types (which can drift between Caddy versions).
type caddyServer struct {
	Listen []string     `json:"listen"`
	Routes []caddyRoute `json:"routes"`
}

type caddyRoute struct {
	Match  []caddyMatcher  `json:"match,omitempty"`
	Handle []caddyHandler  `json:"handle,omitempty"`
	Routes []caddyRoute    `json:"routes,omitempty"` // for nested subroute handlers
	Extras json.RawMessage `json:"-"`                // catches anything else for the unrecognised-directive warning
}

type caddyMatcher struct {
	Host []string `json:"host,omitempty"`
}

// caddyHandler is loose-typed because each handler kind has its own
// fields. We extract by handler name rather than dispatching on a
// known struct shape, since the goal is best-effort conversion of the
// common cases.
type caddyHandler struct {
	Handler   string                 `json:"handler"`
	Routes    []caddyRoute           `json:"routes,omitempty"` // subroute
	Upstreams []caddyUpstream        `json:"upstreams,omitempty"`
	FlushIntv json.Number            `json:"flush_interval,omitempty"`
	Headers   *caddyReverseProxyHdrs `json:"headers,omitempty"`
	Response  *caddyHeadersResp      `json:"response,omitempty"`
	Request   *caddyHeadersReq       `json:"request,omitempty"`
	Raw       map[string]interface{} `json:"-"`
}

type caddyUpstream struct {
	Dial string `json:"dial"`
}

type caddyReverseProxyHdrs struct {
	Request *caddyHeadersReq `json:"request,omitempty"`
}

type caddyHeadersReq struct {
	Set    map[string][]string `json:"set,omitempty"`
	Add    map[string][]string `json:"add,omitempty"`
	Delete []string            `json:"delete,omitempty"`
}

type caddyHeadersResp struct {
	Set    map[string][]string `json:"set,omitempty"`
	Add    map[string][]string `json:"add,omitempty"`
	Delete []string            `json:"delete,omitempty"`
}

// routeToSite converts a single top-level Caddy route into a
// GatewaySite, plus any warnings about directives we couldn't migrate.
// Returns (nil, warnings) when the route is unconvertible (e.g., no
// host match, or no reverse_proxy under it).
func routeToSite(route *caddyRoute, httpOnly bool) (*GatewaySite, []string) {
	host, hostWarn := singleHost(route)
	if host == "" {
		// Routes without a host match are typically Caddy-level
		// concerns (redirects, ACME challenges) but they can also
		// be a default-site `:443 { reverse_proxy ... }` block, which
		// IS something the operator wants migrated. Detect that case
		// by walking the handlers; if we find a reverse_proxy under
		// a route with no host, warn loudly so the operator knows
		// they need to set a domain in gateway_sites manually.
		var rpUpstream string
		walkHandlers(route.Handle, func(h *caddyHandler) {
			if h.Handler == "reverse_proxy" && len(h.Upstreams) > 0 && rpUpstream == "" {
				rpUpstream = h.Upstreams[0].Dial
			}
		})
		if rpUpstream != "" {
			hostWarn = append(hostWarn,
				fmt.Sprintf("route with no host matcher contains reverse_proxy %q; cannot migrate to gateway_sites (which requires a domain). Configure this on Muximux's TLS settings or set up a wildcard A record and add a domain entry by hand.", rpUpstream))
		}
		return nil, hostWarn
	}

	site := GatewaySite{Domain: host}
	if httpOnly {
		site.TLS = "none"
	}

	var warnings []string
	warnings = append(warnings, hostWarn...)

	// Walk the handle chain for reverse_proxy + ancillary directives.
	rpFound := false
	walkHandlers(route.Handle, func(h *caddyHandler) {
		switch h.Handler {
		case "reverse_proxy":
			if rpFound {
				warnings = append(warnings, fmt.Sprintf("%s: multiple reverse_proxy directives, kept the first", host))
				return
			}
			rpFound = true
			rpToSite(&site, h, host, &warnings)
		case "headers":
			headersToSite(&site, h, host, &warnings)
		case "":
			// no-op: walkHandlers strips subroute wrappers before
			// invoking us, so this branch only catches handlers with
			// a missing handler field (which shouldn't happen with a
			// Caddyfile-derived config).
		default:
			warnings = append(warnings, fmt.Sprintf("%s: handler %q cannot be migrated; remove or run that service outside Muximux's gateway", host, h.Handler))
		}
	})

	if !rpFound {
		warnings = append(warnings, fmt.Sprintf("%s: no reverse_proxy directive found, skipping", host))
		return nil, warnings
	}

	return &site, warnings
}

// rpToSite copies the salient fields out of a reverse_proxy handler
// into the site under construction.
func rpToSite(site *GatewaySite, h *caddyHandler, host string, warnings *[]string) {
	if len(h.Upstreams) == 0 {
		*warnings = append(*warnings, fmt.Sprintf("%s: reverse_proxy with no upstreams; skipping", host))
		return
	}
	if len(h.Upstreams) > 1 {
		*warnings = append(*warnings, fmt.Sprintf("%s: reverse_proxy with %d upstreams, kept the first; the structured form supports a single backend per site", host, len(h.Upstreams)))
	}
	dial := h.Upstreams[0].Dial
	if !strings.Contains(dial, "://") {
		// Caddyfile-style "host:port" upstreams default to http://; preserve that.
		dial = "http://" + dial
	}
	site.BackendURL = dial

	if h.FlushIntv.String() == "-1" {
		site.Streaming = true
	}

	if h.Headers != nil && h.Headers.Request != nil {
		// reverse_proxy `header_up X-Foo bar` becomes
		// headers.request.set or headers.request.add in JSON. We map
		// either form to ProxyHeaders since the structured form does
		// not distinguish set-vs-add (Caddy's set is what `header_up`
		// emits in practice).
		merged := mergeHeaderMap(h.Headers.Request.Set, h.Headers.Request.Add)
		// Skip the X-Forwarded-* headers that the structured generator
		// already emits, otherwise migrating a vanilla site would
		// duplicate them. Use exact-match (a name-set, not substring)
		// so an operator's custom `X-Forwarded-Client-Cert` does not
		// get clobbered by a substring hit on "X-Forwarded".
		dropExact := map[string]struct{}{
			"X-Forwarded-Proto": {},
			"X-Forwarded-Host":  {},
			"X-Real-IP":         {},
			"X-Forwarded-For":   {},
		}
		for k, v := range merged {
			if _, isAuto := dropExact[k]; isAuto {
				// The operator was customising one of the auto-emitted
				// headers (e.g., setting X-Forwarded-Proto to a fixed
				// value). The structured generator overrides this; warn
				// so they know to toggle "Forward headers" off if they
				// need full control.
				*warnings = append(*warnings, fmt.Sprintf("%s: dropped header_up %s override (set to %q) - the structured form sets X-Forwarded-* automatically; toggle forwarded_headers: false if you need full control", host, k, v))
				continue
			}
			if site.ProxyHeaders == nil {
				site.ProxyHeaders = map[string]string{}
			}
			site.ProxyHeaders[k] = v
		}
		if len(h.Headers.Request.Delete) > 0 {
			*warnings = append(*warnings, fmt.Sprintf("%s: header_up -%v on the reverse_proxy is not supported in the structured form", host, h.Headers.Request.Delete))
		}
	}
}

// headersToSite interprets a top-level `headers` handler. The most
// common pattern we want to detect is the X-Frame-Options strip so it
// maps to strip_frame_blockers: true.
func headersToSite(site *GatewaySite, h *caddyHandler, host string, warnings *[]string) {
	if h.Response != nil {
		for _, name := range h.Response.Delete {
			if strings.EqualFold(name, "X-Frame-Options") {
				site.StripFrameBlockers = true
				continue
			}
			*warnings = append(*warnings, fmt.Sprintf("%s: response-header delete %q has no structured-form equivalent", host, name))
		}
		if len(h.Response.Set) > 0 || len(h.Response.Add) > 0 {
			names := make([]string, 0, len(h.Response.Set)+len(h.Response.Add))
			for name := range h.Response.Set {
				names = append(names, name)
			}
			for name := range h.Response.Add {
				names = append(names, name)
			}
			sort.Strings(names)
			*warnings = append(*warnings, fmt.Sprintf("%s: response-header set/add directives [%s] are not migrated; revisit after import", host, strings.Join(names, ", ")))
		}
	}
	if h.Request != nil {
		// Top-level `header` (request) directives are rare; warn rather
		// than try to coerce them into proxy_headers because the
		// semantics differ (they apply before the reverse_proxy fires
		// any other middleware).
		*warnings = append(*warnings, fmt.Sprintf("%s: top-level request-header directives are not migrated", host))
	}
}

// walkHandlers visits every handler in the given list, recursing into
// subroute handlers' inner routes so the visitor sees a flat stream
// of leaf handlers regardless of how the Caddyfile happened to nest
// them.
func walkHandlers(handlers []caddyHandler, visit func(*caddyHandler)) {
	for i := range handlers {
		h := &handlers[i]
		if h.Handler == "subroute" {
			for j := range h.Routes {
				walkHandlers(h.Routes[j].Handle, visit)
			}
			continue
		}
		visit(h)
	}
}

// singleHost returns the one host the route matches. Multi-host routes
// emit a warning and return the first host so the migrator does not
// silently drop them; the operator can decide whether to split or
// rewrite manually.
func singleHost(route *caddyRoute) (string, []string) {
	hosts := []string{}
	for i := range route.Match {
		hosts = append(hosts, route.Match[i].Host...)
	}
	if len(hosts) == 0 {
		return "", nil
	}
	if len(hosts) == 1 {
		return hosts[0], nil
	}
	return hosts[0], []string{
		fmt.Sprintf("route matches multiple hosts %v; kept the first (%s) - split into separate gateway_sites entries by hand for the others", hosts, hosts[0]),
	}
}

// serverListenIsHTTPOnly returns true when the server only listens on
// port 80, indicating the operator wrote `http://host` or `host:80`
// in the Caddyfile and wants no TLS. Anything else (including the
// default `:443`) is treated as wanting auto-HTTPS.
func serverListenIsHTTPOnly(listen []string) bool {
	if len(listen) == 0 {
		return false
	}
	for _, l := range listen {
		if !strings.HasSuffix(l, ":80") {
			return false
		}
	}
	return true
}

// mergeHeaderMap flattens Caddy's set + add maps into a single
// name -> value map, taking the first value per name. The structured
// gateway form supports one value per header.
func mergeHeaderMap(set, add map[string][]string) map[string]string {
	out := make(map[string]string, len(set)+len(add))
	for _, m := range []map[string][]string{set, add} {
		for k, vs := range m {
			if len(vs) == 0 {
				continue
			}
			if _, ok := out[k]; ok {
				continue // first occurrence wins
			}
			out[k] = vs[0]
		}
	}
	return out
}

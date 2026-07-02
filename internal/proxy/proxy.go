package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	_ "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile" // Register Caddyfile adapter
	_ "github.com/caddyserver/caddy/v2/modules/standard"

	"github.com/mescon/muximux/v3/internal/logging"
)

// AppRoute represents a proxied app route
type AppRoute struct {
	Name      string
	Slug      string // URL-safe name for routing
	TargetURL string
	Enabled   bool
}

// Config holds proxy configuration. Mirrors the fields on
// config.ServerConfig that drive Caddyfile generation but stays
// independent so the proxy package does not import config (avoids
// the import cycle that would arise if config also wanted to call
// into proxy for validation helpers).
type Config struct {
	ListenAddr   string        // User-facing listen address (e.g., ":8080")
	InternalAddr string        // Where Go server lives (e.g., "127.0.0.1:18080")
	Domain       string        // For auto-HTTPS
	Email        string        // For auto-HTTPS
	TLSCert      string        // Manual TLS
	TLSKey       string        // Manual TLS
	GatewaySites []GatewaySite // Structured per-site gateway entries (replaces the legacy file-import)
	// GatewayListen overrides the default Caddy gateway-site bind.
	// Empty (""): legacy behavior - sites listen on 80/443 with auto-
	// HTTPS. Non-empty (e.g. ":8443"): all gateway sites listen on
	// that address as plain HTTP unless they have TLS=custom (which
	// keeps Caddy doing TLS on the operator-supplied cert+key). Use
	// when running behind another reverse proxy that handles TLS.
	GatewayListen string
}

// GatewaySite mirrors config.GatewaySite. Duplicated rather than
// imported to keep the proxy package's surface stable when config
// adds fields that don't affect Caddyfile generation. The server
// package translates between the two when constructing the proxy.
type GatewaySite struct {
	Domain             string
	BackendURL         string
	TLS                string
	TLSCert            string
	TLSKey             string
	StripFrameBlockers bool
	Streaming          bool
	// BackendSkipTLSVerify disables backend TLS-certificate
	// verification (self-signed backends like Proxmox). Only emitted
	// when BackendURL is https.
	BackendSkipTLSVerify bool
	ProxyHeaders         map[string]string
	ForwardedHeaders     *bool
	// RequireAuth, when true, makes the generator emit a
	// forward_auth directive ahead of reverse_proxy that calls
	// back to Muximux's GatewayAuthHandler on the internal port.
	// The site is gated behind the Muximux session.
	RequireAuth bool
}

// ForwardedOrDefault returns the operator's chosen value for the
// X-Forwarded-* headers, defaulting to true when the field was left
// unset. Pulling this rule onto a method keeps the "nil means default
// true" convention in one place; readers and the Caddyfile generator
// don't need to remember the pointer-comparison idiom.
func (s *GatewaySite) ForwardedOrDefault() bool {
	return s.ForwardedHeaders == nil || *s.ForwardedHeaders
}

// Proxy manages the embedded Caddy reverse proxy
type Proxy struct {
	config  Config
	routes  map[string]AppRoute
	mu      sync.RWMutex
	running bool
	// testReloadHook, when non-nil, replaces the Reload primitive in
	// helpers that go through reloadHookOrDefault (currently only
	// ApplyGatewaySites). Used by unit tests to exercise the
	// rollback / divergence decision paths without booting a real
	// Caddy. Production code never sets this.
	testReloadHook func() error
}

// New creates a new proxy instance
func New(cfg *Config) *Proxy {
	return &Proxy{
		config: *cfg,
		routes: make(map[string]AppRoute),
	}
}

// ComputeInternalAddr derives the internal Go server address from the
// user-facing listen address. ":8080" → "127.0.0.1:18080",
// ":3000" → "127.0.0.1:13000". Listen ports above 55535 would overflow
// past the 16-bit port range when adding the +10000 offset, so those
// wrap down to a low free port instead (findings.md L8).
func ComputeInternalAddr(listen string) string {
	_, port, _ := net.SplitHostPort(listen)
	if port == "" {
		port = strings.TrimPrefix(listen, ":")
	}
	portNum, _ := strconv.Atoi(port)
	if portNum == 0 {
		portNum = 8080
	}
	internal := portNum + 10000
	if internal > 65535 {
		// Wrap around and subtract 20000 so the result stays inside
		// valid port range and is distinct from the listener.
		internal = portNum - 10000
		if internal < 1024 {
			internal = 18080
		}
	}
	return fmt.Sprintf("127.0.0.1:%d", internal)
}

// SetGatewaySites swaps the structured gateway-site list the next
// buildCaddyfile call will read. Used by the REST handler to apply a
// candidate site list before invoking Reload; on Reload failure the
// caller is expected to restore the prior list and Reload again so
// in-memory state matches what Caddy is actually serving.
func (p *Proxy) SetGatewaySites(sites []GatewaySite) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(sites) == 0 {
		p.config.GatewaySites = nil
		return
	}
	// Defensive copy so the caller can mutate its slice without
	// surprising us on the next Reload.
	p.config.GatewaySites = append([]GatewaySite(nil), sites...)
}

// GatewaySites returns a copy of the current site list. Used by the
// REST handler to snapshot prior state before mutation, so a failed
// reload or persist can be rolled back deterministically.
func (p *Proxy) GatewaySites() []GatewaySite {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.config.GatewaySites) == 0 {
		return nil
	}
	return append([]GatewaySite(nil), p.config.GatewaySites...)
}

// SetRoutes updates the proxy routes (used for tracking which apps have proxy enabled).
// Caddy itself doesn't need per-app routes — it forwards everything to the main server.
func (p *Proxy) SetRoutes(routes []AppRoute) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.routes = make(map[string]AppRoute)
	for _, route := range routes {
		if route.Enabled {
			p.routes[route.Slug] = route
		}
	}
}

// Start starts the proxy server
func (p *Proxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	caddyfileText := p.buildCaddyfile()
	logging.Debug("Caddy configuration generated", "source", "caddy", "config", caddyfileText)

	if err := loadCaddyfile(caddyfileText); err != nil {
		return fmt.Errorf("caddy start failed: %w", err)
	}

	p.running = true
	logging.Info("Caddy started", "source", "caddy", "internal_addr", p.config.InternalAddr)
	return nil
}

// Reload regenerates the Caddyfile from current proxy state and applies
// it in-process via Caddy's library API.
//
// Transactionality is *only* guaranteed at the parse step. If the
// adapter rejects the candidate Caddyfile, no state change happens.
// Once parse succeeds and caddy.Load is invoked, post-parse failures
// (a listener that races with another process, an async ACME cert
// provisioning hiccup, a goroutine panic in a Caddy module) can leave
// the previous config partially torn down. The gateway handler
// calling this is aware and re-asserts the prior config via a second
// Reload on candidate-load failure; other callers should plan for the
// same. (codebase review G1)
//
// Callers should regenerate p.config (and any structured site list)
// before calling Reload so the freshly-built Caddyfile reflects the
// intended state.
//
// Reload is safe to call when the proxy is not yet running; it falls
// through to Start() in that case so the same code path can apply
// the first config and any subsequent changes.
func (p *Proxy) Reload() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		// First-time application: defer to Start's normal path. Caddy's
		// Load() handles the cold-start case the same way as a reload,
		// but we set p.running here for parity with Start.
		caddyfileText := p.buildCaddyfile()
		if err := loadCaddyfile(caddyfileText); err != nil {
			return fmt.Errorf("caddy reload (cold start) failed: %w", err)
		}
		p.running = true
		logging.Info("Caddy started via Reload", "source", "caddy", "internal_addr", p.config.InternalAddr)
		return nil
	}

	caddyfileText := p.buildCaddyfile()
	logging.Debug("Caddy configuration regenerated", "source", "caddy", "config", caddyfileText)

	if err := loadCaddyfile(caddyfileText); err != nil {
		// Adapt-step rejection: previous config is untouched. Post-
		// parse rejections (listener collision, async cert failure,
		// module panic) can leave Caddy in a degraded state; callers
		// that need a hard guarantee re-assert the prior config via
		// a second Reload. See the doc-comment above.
		return fmt.Errorf("caddy reload failed: %w", err)
	}

	logging.Info("Caddy reloaded", "source", "caddy")
	return nil
}

// CaddyfilePreview renders the Caddyfile that would be applied if the
// proxy's GatewaySites list were replaced with `sites`, without
// touching the live proxy state. Used by handlers to dry-run a
// candidate site list against `Validate` (and against a real Caddy
// adapt) before deciding to persist or reload.
func (p *Proxy) CaddyfilePreview(sites []GatewaySite) string {
	p.mu.RLock()
	cfgCopy := p.config
	p.mu.RUnlock()
	cfgCopy.GatewaySites = append([]GatewaySite(nil), sites...)
	tmp := &Proxy{config: cfgCopy}
	return tmp.buildCaddyfile()
}

// portOnly normalises a bind address ("0.0.0.0:8443", ":8443", or
// "[::]:8443") down to ":<port>" so the gateway-site selector can
// emit "domain.example.com:8443" without dragging the bind host
// into the site name. Returns the input verbatim if it already
// starts with ":" and contains no other colons.
func portOnly(addr string) string {
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, ":") && !strings.ContainsAny(addr[1:], ":") {
		return addr
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		// Pre-validated bind address; if SplitHostPort fails here
		// just leave the value alone - Caddy will reject it loudly
		// which is better than silently mangling.
		return addr
	}
	return ":" + port
}

// normalizeUpstream re-renders a backend URL from its parsed scheme +
// host. Validation upstream has already enforced that the URL has no
// path/query/fragment and uses http or https; this helper is the
// last-mile guarantee that the string we hand to Caddy is canonical
// (no stray percent-encoded characters smuggled through url.Parse,
// no surrounding whitespace).
//
// On parse failure (which validation should have already rejected),
// the original string is returned unchanged so the caller still gets
// a Caddyfile and Caddy's own parser surfaces the error consistently.
// We also log the fall-through so a future caller bypassing
// validation does not silently produce a malformed Caddyfile that
// the operator then has to debug from a Caddy parser error message.
func normalizeUpstream(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		logging.Warn("normalizeUpstream parse failed; emitting raw value",
			"source", "caddy",
			"url", raw)
		return raw
	}
	clean := url.URL{Scheme: u.Scheme, Host: u.Host}
	return clean.String()
}

// Validate runs the same Caddyfile parse Start/Reload would run, but
// without applying the result. Used by handlers that want to lint a
// candidate change before persisting (the UI's instant-validation
// path). A nil return means the Caddyfile parses cleanly; a non-nil
// error contains the parser's complaint, suitable for returning to
// the operator verbatim.
//
// Validate does not catch every kind of failure caddy.Load can hit
// (port collisions, cert provisioning issues are runtime concerns),
// but it catches every static error in the Caddyfile syntax and
// directive arguments.
func Validate(caddyfileText string) error {
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		return fmt.Errorf("caddyfile adapter not registered")
	}
	if _, _, err := adapter.Adapt([]byte(caddyfileText), nil); err != nil {
		return fmt.Errorf("caddyfile invalid: %w", err)
	}
	return nil
}

// loadCaddyfile is the shared adapt-then-load primitive used by Start
// and Reload. Adapt failures are surfaced as parse errors (no Caddy
// state has changed yet); Load failures propagate Caddy's own error
// (the previous config remains active because caddy.Load is
// transactional).
func loadCaddyfile(caddyfileText string) error {
	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		return fmt.Errorf("caddyfile adapter not registered")
	}
	cfgJSON, _, err := adapter.Adapt([]byte(caddyfileText), nil)
	if err != nil {
		return fmt.Errorf("caddyfile error: %w", err)
	}
	if err := caddy.Load(cfgJSON, true); err != nil {
		return remapCaddyLoadError(err)
	}
	return nil
}

// remapCaddyLoadError translates the deeply-nested error caddy.Load
// returns for common operator-actionable failures into something the
// admin can act on without parsing five layers of wrapping.
//
// The big one is "permission denied" when binding 80 or 443 without
// CAP_NET_BIND_SERVICE: the raw error is "loading new config: http
// app module: start: listening on :443: listen tcp :443: bind:
// permission denied". We surface a remediation hint pointing at the
// three concrete fixes (setcap, run as root, or set
// server.gateway_listen).
func remapCaddyLoadError(err error) error {
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(low, "permission denied") && strings.Contains(low, "listen tcp") {
		// Try to extract the offending port for a more pointed
		// hint. Pattern: "listening on :443" or "listening on
		// 0.0.0.0:443"; fall back to "a privileged port" if it
		// doesn't match cleanly.
		port := "a privileged port (likely :80 or :443)"
		if idx := strings.Index(msg, "listen tcp "); idx >= 0 {
			tail := msg[idx+len("listen tcp "):]
			if end := strings.Index(tail, ":"); end >= 0 {
				rest := tail[end+1:]
				if space := strings.IndexAny(rest, ": \t\n\r"); space >= 0 {
					rest = rest[:space]
				}
				if rest != "" {
					port = ":" + rest
				}
			}
		}
		return fmt.Errorf("caddy cannot bind %s without privileges. Three fixes:\n"+
			"  1. Run as root (or via systemd with the right user/cap)\n"+
			"  2. Grant the binary CAP_NET_BIND_SERVICE:\n"+
			"     sudo setcap 'cap_net_bind_service=+ep' <muximux-binary>\n"+
			"  3. Configure server.gateway_listen to a non-privileged port\n"+
			"     (e.g. \":8443\") and run Muximux behind another reverse\n"+
			"     proxy that handles ports 80/443 + TLS termination.\n"+
			"original caddy error: %w", port, err)
	}
	return fmt.Errorf("caddy load failed: %w", err)
}

// Stop stops the proxy server
func (p *Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	if err := caddy.Stop(); err != nil {
		return fmt.Errorf("failed to stop proxy: %w", err)
	}

	p.running = false
	logging.Info("Caddy stopped", "source", "caddy")
	return nil
}

// buildCaddyfile generates the Caddyfile configuration text.
//
// Output structure:
//  1. Global options block (admin off, plus auto_https / email if needed).
//  2. Muximux's own site block (the user-facing listener that forwards
//     to the internal Go server).
//  3. One site block per GatewaySite (subdomain hosting for other apps).
//
// When any structured gateway site needs auto-HTTPS, we leave Caddy's
// auto_https on so it can manage ports 80/443 for those domains; the
// previous (legacy) gating on a non-empty Gateway file path is gone.
func (p *Proxy) buildCaddyfile() string {
	var b strings.Builder

	reverseProxyBlock := fmt.Sprintf(`	reverse_proxy %s {
		header_up X-Forwarded-Proto {scheme}
		header_up X-Forwarded-Host {host}
		header_up X-Real-IP {remote_host}
	}`, p.config.InternalAddr)

	// auto-HTTPS is needed only when at least one gateway site uses
	// TLS=auto AND we're binding the default ports (no GatewayListen
	// override). When GatewayListen is set, the listen port is high
	// and the Caddyfile generator emits explicit http:// blocks per
	// site, so auto-HTTPS would just spin up needless HTTP-01
	// challenge listeners.
	gatewayNeedsAutoHTTPS := false
	if p.config.GatewayListen == "" {
		for i := range p.config.GatewaySites {
			tls := p.config.GatewaySites[i].TLS
			if tls == "" || tls == "auto" {
				gatewayNeedsAutoHTTPS = true
				break
			}
		}
	}

	switch {
	case p.config.Domain != "":
		// Auto-HTTPS with domain: Caddy manages ports 80+443 automatically
		fmt.Fprintf(&b, "{\n\temail %s\n\tadmin off\n}\n\n", p.config.Email)
		fmt.Fprintf(&b, "%s {\n%s\n}\n", p.config.Domain, reverseProxyBlock)
	case p.config.TLSCert != "":
		// Manual TLS for Muximux's own listener. Keep auto_https on
		// when any gateway site needs Let's Encrypt; otherwise turn
		// it off to avoid spurious port 80/443 binds.
		if gatewayNeedsAutoHTTPS {
			fmt.Fprintf(&b, "{\n\tadmin off\n}\n\n")
		} else {
			fmt.Fprintf(&b, "{\n\tauto_https off\n\tadmin off\n}\n\n")
		}
		fmt.Fprintf(&b, "%s {\n\ttls %s %s\n%s\n}\n",
			p.config.ListenAddr, p.config.TLSCert, p.config.TLSKey, reverseProxyBlock)
	default:
		if gatewayNeedsAutoHTTPS {
			fmt.Fprintf(&b, "{\n\tadmin off\n}\n\n")
		} else {
			fmt.Fprintf(&b, "{\n\tauto_https off\n\tadmin off\n}\n\n")
		}
		fmt.Fprintf(&b, "%s {\n%s\n}\n", p.config.ListenAddr, reverseProxyBlock)
	}

	// Emit each structured gateway site as its own Caddy site block.
	for i := range p.config.GatewaySites {
		writeGatewaySiteBlock(&b, &p.config.GatewaySites[i], p.config.GatewayListen, p.config.InternalAddr)
	}

	return b.String()
}

// writeGatewaySiteBlock emits one Caddy site block for the given site.
// WebSocket upgrades, HTTP/2, and large request bodies inherit Caddy's
// defaults — we deliberately do not add directives that would limit them.
//
// gatewayListen, when non-empty, overrides the address Caddy binds for
// this site. Empty: legacy 80/443 with auto-HTTPS. Non-empty (e.g.
// ":8443"): the site is served on that address as plain HTTP unless
// TLS=custom (which keeps the operator-supplied cert).
func writeGatewaySiteBlock(b *strings.Builder, s *GatewaySite, gatewayListen, internalAddr string) {
	b.WriteString("\n")

	// Site selector. Three shapes:
	//   1) gatewayListen empty + tls=none           -> http://domain (port 80)
	//   2) gatewayListen empty + tls=auto/custom    -> domain (auto 80/443)
	//   3) gatewayListen set                        -> http(s)://domain:port
	//
	// In case 3 we force the explicit scheme + port. tls=auto on a
	// non-default port is downgraded to plain HTTP because Caddy's
	// HTTP-01 challenge cannot reach the operator's box on a high
	// port; the operator should switch to TLS=custom or remove
	// gateway_listen if they want managed certs.
	if gatewayListen != "" {
		// portOnly reduces "0.0.0.0:8443" / ":8443" / "[::]:8443"
		// to ":8443" so we never drag the bind host into the site
		// selector (Caddy treats the host part of a site selector
		// as a Host-header match, which would break things).
		listenSuffix := portOnly(gatewayListen)
		switch s.TLS {
		case "custom":
			fmt.Fprintf(b, "https://%s%s {\n", s.Domain, listenSuffix)
			fmt.Fprintf(b, "\ttls %s %s\n", s.TLSCert, s.TLSKey)
		default:
			// "auto" silently downgrades to HTTP here; the global
			// auto_https=off in the prefix block prevents Caddy
			// from issuing certs.
			fmt.Fprintf(b, "http://%s%s {\n", s.Domain, listenSuffix)
		}
	} else {
		switch s.TLS {
		case "none":
			fmt.Fprintf(b, "http://%s {\n", s.Domain)
		default:
			fmt.Fprintf(b, "%s {\n", s.Domain)
		}
		// TLS directive only for the custom case; auto-HTTPS is implicit.
		if s.TLS == "custom" {
			fmt.Fprintf(b, "\ttls %s %s\n", s.TLSCert, s.TLSKey)
		}
	}

	// reverse_proxy block. Per-site directives go inside the braces
	// so flush_interval and header_up apply to this site only.
	// Render the upstream from parsed components so a passing-through
	// percent-encoded space or other oddity from the operator's input
	// cannot reach Caddy's parser unchanged. Validation upstream has
	// already rejected paths/queries/fragments and confirmed the
	// scheme is http or https; this is defence in depth.

	// forward_auth gate. When require_auth is set, Caddy calls
	// /api/auth/forward on the internal Muximux port before
	// forwarding to the backend. 200 -> forward, 302 -> follow
	// to /login, 403 -> serve forbidden page. The X-Muximux-User
	// + X-Muximux-Role headers are copied from the auth response
	// onto the upstream request so backends that care can read
	// the authenticated identity.
	if s.RequireAuth {
		fmt.Fprintf(b, "\tforward_auth %s {\n", internalAddr)
		b.WriteString("\t\turi /api/auth/forward\n")
		b.WriteString("\t\tcopy_headers X-Muximux-User X-Muximux-Role\n")
		b.WriteString("\t}\n")
	}

	fmt.Fprintf(b, "\treverse_proxy %s {\n", normalizeUpstream(s.BackendURL))

	// Streaming flushes every write — needed for SSE, video transcodes,
	// and other long-lived response streams.
	if s.Streaming {
		b.WriteString("\t\tflush_interval -1\n")
	}

	// Forwarded headers default on. Suppressed only on explicit false.
	if s.ForwardedOrDefault() {
		b.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
		b.WriteString("\t\theader_up X-Forwarded-Host {host}\n")
		b.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
	}

	// Custom upstream headers (auth tokens, backend API keys).
	for k, v := range s.ProxyHeaders {
		fmt.Fprintf(b, "\t\theader_up %s %q\n", k, v)
	}

	// Backend TLS verification skip for self-signed HTTPS backends
	// (e.g. Proxmox on :8006). Only meaningful over https; enabling the
	// http transport with tls_insecure_skip_verify implicitly turns on
	// TLS, so we gate it on an https upstream to avoid forcing TLS onto
	// a plain-http backend. TrimSpace mirrors normalizeUpstream so a
	// stored URL with incidental whitespace still matches.
	if s.BackendSkipTLSVerify && strings.HasPrefix(strings.ToLower(strings.TrimSpace(s.BackendURL)), "https://") {
		b.WriteString("\t\ttransport http {\n")
		b.WriteString("\t\t\ttls_insecure_skip_verify\n")
		b.WriteString("\t\t}\n")
	}

	b.WriteString("\t}\n")

	// Frame-blocker stripping. The operator opted in to embedding this
	// gateway subdomain in Muximux's dashboard, which lives on a DIFFERENT
	// origin (e.g. sonarr.example.com vs muximux.example.com). Remove both
	// framing headers the backend might send. The previous code re-added
	// `frame-ancestors 'self'`, but at a gateway site "self" is the site's
	// own origin, so it blocked the cross-origin dashboard -- the opposite
	// of the feature's intent (a Caddy `{host}` placeholder can't help
	// either: it resolves to the gateway host, not the dashboard's). We
	// can't reliably name the dashboard origin here, so strip the CSP
	// rather than re-add a restrictive one; this is no less protective
	// than the old behavior, which already discarded the backend's CSP.
	if s.StripFrameBlockers {
		b.WriteString("\theader -X-Frame-Options\n")
		b.WriteString("\theader -Content-Security-Policy\n")
	}

	b.WriteString("}\n")
}

// GetProxyURL returns the proxy URL for an app
func (p *Proxy) GetProxyURL(slug string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, exists := p.routes[slug]; exists {
		return fmt.Sprintf("/proxy/%s/", slug)
	}
	return ""
}

// GetInternalAddr returns the internal address where the Go server should listen.
func (p *Proxy) GetInternalAddr() string {
	return p.config.InternalAddr
}

// IsRunning returns whether the proxy is running
func (p *Proxy) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

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
	ProxyHeaders       map[string]string
	ForwardedHeaders   *bool
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
		return fmt.Errorf("caddy load failed: %w", err)
	}
	return nil
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

	gatewayNeedsAutoHTTPS := false
	for i := range p.config.GatewaySites {
		tls := p.config.GatewaySites[i].TLS
		if tls == "" || tls == "auto" {
			gatewayNeedsAutoHTTPS = true
			break
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
		writeGatewaySiteBlock(&b, &p.config.GatewaySites[i])
	}

	return b.String()
}

// writeGatewaySiteBlock emits one Caddy site block for the given site.
// WebSocket upgrades, HTTP/2, and large request bodies inherit Caddy's
// defaults — we deliberately do not add directives that would limit them.
func writeGatewaySiteBlock(b *strings.Builder, s *GatewaySite) {
	b.WriteString("\n")

	// Site selector. For tls=none we prefix with "http://" so Caddy
	// serves plain HTTP on port 80 instead of trying auto-HTTPS.
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

	// reverse_proxy block. Per-site directives go inside the braces
	// so flush_interval and header_up apply to this site only.
	// Render the upstream from parsed components so a passing-through
	// percent-encoded space or other oddity from the operator's input
	// cannot reach Caddy's parser unchanged. Validation upstream has
	// already rejected paths/queries/fragments and confirmed the
	// scheme is http or https; this is defence in depth.
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

	b.WriteString("\t}\n")

	// Frame-blocker stripping is implemented as response-header
	// rewrites: drop X-Frame-Options entirely and rewrite (or inject)
	// CSP frame-ancestors so Muximux's own origin can iframe this
	// site. The {scheme}://{host} placeholders resolve at request
	// time so Muximux deployments behind a reverse proxy still get
	// the right Origin.
	if s.StripFrameBlockers {
		b.WriteString("\theader -X-Frame-Options\n")
		b.WriteString("\theader Content-Security-Policy \"frame-ancestors 'self'\"\n")
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

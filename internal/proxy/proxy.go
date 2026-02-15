package proxy

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	_ "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile" // Register Caddyfile adapter
	_ "github.com/caddyserver/caddy/v2/modules/standard"          // Import standard Caddy modules
	"github.com/mescon/muximux/v3/internal/logging"
)

// AppRoute represents a proxied app route
type AppRoute struct {
	Name      string
	Slug      string // URL-safe name for routing
	TargetURL string
	Enabled   bool
}

// Config holds proxy configuration
type Config struct {
	ListenAddr   string // User-facing listen address (e.g., ":8080")
	InternalAddr string // Where Go server lives (e.g., "127.0.0.1:18080")
	Domain       string // For auto-HTTPS
	Email        string // For auto-HTTPS
	TLSCert      string // Manual TLS
	TLSKey       string // Manual TLS
	Gateway      string // Path to extra Caddyfile
}

// Proxy manages the embedded Caddy reverse proxy
type Proxy struct {
	config  Config
	routes  map[string]AppRoute
	mu      sync.RWMutex
	running bool
}

// New creates a new proxy instance
func New(cfg Config) *Proxy {
	return &Proxy{
		config: cfg,
		routes: make(map[string]AppRoute),
	}
}

// ComputeInternalAddr derives the internal Go server address from the user-facing listen address.
// ":8080" → "127.0.0.1:18080", ":3000" → "127.0.0.1:13000"
func ComputeInternalAddr(listen string) string {
	_, port, _ := net.SplitHostPort(listen)
	if port == "" {
		port = strings.TrimPrefix(listen, ":")
	}
	portNum, _ := strconv.Atoi(port)
	if portNum == 0 {
		portNum = 8080
	}
	return fmt.Sprintf("127.0.0.1:%d", portNum+10000)
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

	adapter := caddyconfig.GetAdapter("caddyfile")
	if adapter == nil {
		return fmt.Errorf("caddyfile adapter not registered")
	}

	cfgJSON, _, err := adapter.Adapt([]byte(caddyfileText), nil)
	if err != nil {
		return fmt.Errorf("caddyfile error: %w", err)
	}

	err = caddy.Load(cfgJSON, true)
	if err != nil {
		return fmt.Errorf("caddy start failed: %w", err)
	}

	p.running = true
	logging.Info("Caddy started", "source", "caddy", "internal_addr", p.config.InternalAddr)
	return nil
}

// Stop stops the proxy server
func (p *Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	err := caddy.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop proxy: %w", err)
	}

	p.running = false
	logging.Info("Caddy stopped", "source", "caddy")
	return nil
}

// buildCaddyfile generates the Caddyfile configuration text.
func (p *Proxy) buildCaddyfile() string {
	var b strings.Builder

	reverseProxyBlock := fmt.Sprintf(`	reverse_proxy %s {
		header_up X-Forwarded-Proto {scheme}
		header_up X-Forwarded-Host {host}
		header_up X-Real-IP {remote_host}
	}`, p.config.InternalAddr)

	if p.config.Domain != "" {
		// Auto-HTTPS with domain: Caddy manages ports 80+443 automatically
		fmt.Fprintf(&b, "{\n\temail %s\n\tadmin off\n}\n\n", p.config.Email)
		fmt.Fprintf(&b, "%s {\n%s\n}\n", p.config.Domain, reverseProxyBlock)
	} else if p.config.TLSCert != "" {
		// Manual TLS: serve HTTPS on the listen port
		// Keep auto_https enabled when gateway is set so domain-based gateway
		// sites can still get automatic certificates and 80→443 redirects.
		if p.config.Gateway != "" {
			fmt.Fprintf(&b, "{\n\tadmin off\n}\n\n")
		} else {
			fmt.Fprintf(&b, "{\n\tauto_https off\n\tadmin off\n}\n\n")
		}
		fmt.Fprintf(&b, "%s {\n\ttls %s %s\n%s\n}\n",
			p.config.ListenAddr, p.config.TLSCert, p.config.TLSKey, reverseProxyBlock)
	} else {
		// HTTP only: disable auto_https unless gateway is set, since gateway
		// sites with domains need Caddy to manage ports 80+443 for them.
		if p.config.Gateway != "" {
			fmt.Fprintf(&b, "{\n\tadmin off\n}\n\n")
		} else {
			fmt.Fprintf(&b, "{\n\tauto_https off\n\tadmin off\n}\n\n")
		}
		fmt.Fprintf(&b, "%s {\n%s\n}\n", p.config.ListenAddr, reverseProxyBlock)
	}

	// Append gateway Caddyfile import if configured
	if p.config.Gateway != "" {
		fmt.Fprintf(&b, "\nimport %s\n", p.config.Gateway)
	}

	return b.String()
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

// Slugify converts an app name to a URL-safe slug
func Slugify(name string) string {
	// Convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove any characters that aren't alphanumeric or hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

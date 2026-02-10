package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
	_ "github.com/caddyserver/caddy/v2/modules/standard" // Import standard Caddy modules
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
	Enabled      bool
	Listen       string // e.g., ":8443"
	UpstreamAddr string // Main server address to forward to, e.g., "localhost:8080"
	AutoHTTPS    bool
	ACMEEmail    string
	TLSCert      string
	TLSKey       string
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
	if !p.config.Enabled {
		log.Println("Proxy is disabled")
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg, err := p.buildConfig()
	if err != nil {
		return fmt.Errorf("failed to build proxy config: %w", err)
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy config: %w", err)
	}

	// Load the configuration
	err = caddy.Load(cfgJSON, true)
	if err != nil {
		return fmt.Errorf("failed to load proxy config: %w", err)
	}

	p.running = true
	log.Printf("Proxy started on %s", p.config.Listen)
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
	log.Println("Proxy stopped")
	return nil
}

// buildConfig builds the Caddy JSON configuration
// Caddy acts as a TLS termination proxy, forwarding all requests to the main
// Muximux server which handles authentication, routing, and content rewriting.
func (p *Proxy) buildConfig() (map[string]interface{}, error) {
	// Single route: forward everything to the main Muximux server
	// This ensures all requests go through auth middleware
	routes := []map[string]interface{}{
		{
			"handle": []map[string]interface{}{
				{
					"handler": "reverse_proxy",
					"upstreams": []map[string]interface{}{
						{
							"dial": p.config.UpstreamAddr,
						},
					},
					"headers": map[string]interface{}{
						"request": map[string]interface{}{
							"set": map[string][]string{
								"X-Forwarded-Proto": {"{http.request.scheme}"},
								"X-Forwarded-Host":  {"{http.request.host}"},
								"X-Real-IP":         {"{http.request.remote.host}"},
							},
						},
					},
					"transport": map[string]interface{}{
						"protocol": "http",
					},
				},
			},
		},
	}

	// Build the server configuration
	server := map[string]interface{}{
		"listen": []string{p.config.Listen},
		"routes": routes,
	}

	// Configure TLS
	if p.config.AutoHTTPS {
		server["automatic_https"] = map[string]interface{}{
			"disable": false,
		}
	} else if p.config.TLSCert != "" && p.config.TLSKey != "" {
		server["tls_connection_policies"] = []map[string]interface{}{
			{
				"certificate_selection": map[string]interface{}{
					"any_tag": []string{"muximux"},
				},
			},
		}
	}

	// Build the full Caddy config
	config := map[string]interface{}{
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"proxy": server,
				},
			},
		},
	}

	// Add TLS certificates if provided
	if p.config.TLSCert != "" && p.config.TLSKey != "" {
		config["apps"].(map[string]interface{})["tls"] = map[string]interface{}{
			"certificates": map[string]interface{}{
				"load_files": []map[string]interface{}{
					{
						"certificate": p.config.TLSCert,
						"key":         p.config.TLSKey,
						"tags":        []string{"muximux"},
					},
				},
			},
		}
	}

	// Configure ACME if enabled
	if p.config.AutoHTTPS && p.config.ACMEEmail != "" {
		config["apps"].(map[string]interface{})["tls"] = map[string]interface{}{
			"automation": map[string]interface{}{
				"policies": []map[string]interface{}{
					{
						"issuers": []map[string]interface{}{
							{
								"module": "acme",
								"email":  p.config.ACMEEmail,
							},
						},
					},
				},
			},
		}
	}

	return config, nil
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


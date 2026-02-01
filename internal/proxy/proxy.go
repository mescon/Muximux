package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
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
	Enabled   bool
	Listen    string // e.g., ":8443"
	AutoHTTPS bool
	ACMEEmail string
	TLSCert   string
	TLSKey    string
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

// SetRoutes updates the proxy routes
func (p *Proxy) SetRoutes(routes []AppRoute) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.routes = make(map[string]AppRoute)
	for _, route := range routes {
		if route.Enabled {
			p.routes[route.Slug] = route
		}
	}

	// If running, reload configuration
	if p.running {
		if err := p.reload(); err != nil {
			log.Printf("Failed to reload proxy config: %v", err)
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

// reload reloads the proxy configuration
func (p *Proxy) reload() error {
	cfg, err := p.buildConfig()
	if err != nil {
		return err
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return caddy.Load(cfgJSON, true)
}

// buildConfig builds the Caddy JSON configuration
func (p *Proxy) buildConfig() (map[string]interface{}, error) {
	// Build routes for each app
	var routes []map[string]interface{}

	for slug, route := range p.routes {
		targetURL, err := url.Parse(route.TargetURL)
		if err != nil {
			log.Printf("Invalid URL for %s: %v", route.Name, err)
			continue
		}

		// Create a route that matches /proxy/{slug}/*
		routeConfig := map[string]interface{}{
			"match": []map[string]interface{}{
				{
					"path": []string{fmt.Sprintf("/proxy/%s/*", slug)},
				},
			},
			"handle": []map[string]interface{}{
				// Strip the /proxy/{slug} prefix
				{
					"handler": "rewrite",
					"strip_path_prefix": fmt.Sprintf("/proxy/%s", slug),
				},
				// Reverse proxy to the target
				{
					"handler": "reverse_proxy",
					"upstreams": []map[string]interface{}{
						{
							"dial": targetURL.Host,
						},
					},
					"headers": map[string]interface{}{
						"request": map[string]interface{}{
							"set": map[string][]string{
								"Host":             {targetURL.Host},
								"X-Forwarded-Host": {"{http.request.host}"},
								"X-Real-IP":        {"{http.request.remote.host}"},
							},
						},
						"response": map[string]interface{}{
							// Remove headers that prevent iframe embedding
							"delete": []string{
								"X-Frame-Options",
								"Content-Security-Policy",
								"X-Content-Type-Options",
							},
						},
					},
					// Handle WebSocket upgrades
					"transport": map[string]interface{}{
						"protocol": "http",
					},
				},
			},
		}

		routes = append(routes, routeConfig)
	}

	// Add a fallback route for unmatched proxy paths
	routes = append(routes, map[string]interface{}{
		"match": []map[string]interface{}{
			{
				"path": []string{"/proxy/*"},
			},
		},
		"handle": []map[string]interface{}{
			{
				"handler":     "static_response",
				"status_code": 404,
				"body":        "App not found",
			},
		},
	})

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


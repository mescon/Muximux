package proxy

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	cfg := &Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
	}

	p := New(cfg)

	if p.config.ListenAddr != ":8080" {
		t.Errorf("expected listen addr ':8080', got %q", p.config.ListenAddr)
	}
	if p.config.InternalAddr != "127.0.0.1:18080" {
		t.Errorf("expected internal addr '127.0.0.1:18080', got %q", p.config.InternalAddr)
	}
	if p.routes == nil {
		t.Error("expected routes map to be initialized")
	}
	if p.running {
		t.Error("expected running to be false initially")
	}
}

func TestComputeInternalAddr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{":8080", "127.0.0.1:18080"},
		{":3000", "127.0.0.1:13000"},
		{":80", "127.0.0.1:10080"},
		{":443", "127.0.0.1:10443"},
		{"0.0.0.0:8080", "127.0.0.1:18080"},
		{"localhost:9090", "127.0.0.1:19090"},
		// Edge case: empty or invalid port
		{"", "127.0.0.1:18080"},
		{":abc", "127.0.0.1:18080"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ComputeInternalAddr(tt.input)
			if result != tt.expected {
				t.Errorf("ComputeInternalAddr(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProxy_SetRoutes(t *testing.T) {
	p := New(&Config{})

	routes := []AppRoute{
		{Name: "App1", Slug: "app1", TargetURL: "http://localhost:8080", Enabled: true},
		{Name: "App2", Slug: "app2", TargetURL: "http://localhost:8081", Enabled: true},
		{Name: "App3", Slug: "app3", TargetURL: "http://localhost:8082", Enabled: false},
	}

	p.SetRoutes(routes)

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Only enabled routes should be stored
	if len(p.routes) != 2 {
		t.Errorf("expected 2 enabled routes, got %d", len(p.routes))
	}

	if _, ok := p.routes["app1"]; !ok {
		t.Error("expected 'app1' in routes")
	}
	if _, ok := p.routes["app2"]; !ok {
		t.Error("expected 'app2' in routes")
	}
	if _, ok := p.routes["app3"]; ok {
		t.Error("expected 'app3' to NOT be in routes (disabled)")
	}
}

func TestProxy_SetRoutes_ClearsOld(t *testing.T) {
	p := New(&Config{})

	// Set initial routes
	p.SetRoutes([]AppRoute{
		{Name: "Old", Slug: "old", TargetURL: "http://old:80", Enabled: true},
	})

	// Set new routes
	p.SetRoutes([]AppRoute{
		{Name: "New", Slug: "new", TargetURL: "http://new:80", Enabled: true},
	})

	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(p.routes))
	}
	if _, ok := p.routes["old"]; ok {
		t.Error("expected old route to be removed")
	}
	if _, ok := p.routes["new"]; !ok {
		t.Error("expected new route to be present")
	}
}

func TestProxy_GetProxyURL(t *testing.T) {
	p := New(&Config{})

	p.SetRoutes([]AppRoute{
		{Name: "App1", Slug: "app1", TargetURL: "http://localhost:8080", Enabled: true},
	})

	t.Run("existing route", func(t *testing.T) {
		url := p.GetProxyURL("app1")
		if url != "/proxy/app1/" {
			t.Errorf("expected '/proxy/app1/', got %q", url)
		}
	})

	t.Run("non-existing route", func(t *testing.T) {
		url := p.GetProxyURL("nonexistent")
		if url != "" {
			t.Errorf("expected empty string, got %q", url)
		}
	})
}

func TestProxy_GetInternalAddr(t *testing.T) {
	p := New(&Config{
		InternalAddr: "127.0.0.1:18080",
	})

	addr := p.GetInternalAddr()
	if addr != "127.0.0.1:18080" {
		t.Errorf("expected '127.0.0.1:18080', got %q", addr)
	}
}

func TestProxy_IsRunning(t *testing.T) {
	p := New(&Config{})

	if p.IsRunning() {
		t.Error("expected not running initially")
	}

	// Simulate running state
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	if !p.IsRunning() {
		t.Error("expected running after setting flag")
	}
}

func TestProxy_Stop_NotRunning(t *testing.T) {
	p := New(&Config{})

	// Stop when not running should be a no-op
	err := p.Stop()
	if err != nil {
		t.Errorf("expected no error stopping non-running proxy: %v", err)
	}
}

func TestProxy_buildCaddyfile_HTTPOnly(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
	})

	cf := p.buildCaddyfile()

	// Should have auto_https off
	if !strings.Contains(cf, "auto_https off") {
		t.Error("expected 'auto_https off' for HTTP-only config")
	}
	if !strings.Contains(cf, "admin off") {
		t.Error("expected 'admin off'")
	}
	if !strings.Contains(cf, ":8080") {
		t.Error("expected listen address ':8080'")
	}
	if !strings.Contains(cf, "reverse_proxy 127.0.0.1:18080") {
		t.Error("expected reverse_proxy directive")
	}
	if !strings.Contains(cf, "X-Forwarded-Proto") {
		t.Error("expected X-Forwarded-Proto header")
	}
	if !strings.Contains(cf, "X-Forwarded-Host") {
		t.Error("expected X-Forwarded-Host header")
	}
	if !strings.Contains(cf, "X-Real-IP") {
		t.Error("expected X-Real-IP header")
	}
}

func TestProxy_buildCaddyfile_AutoHTTPS(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":443",
		InternalAddr: "127.0.0.1:10443",
		Domain:       "example.com",
		Email:        "admin@example.com",
	})

	cf := p.buildCaddyfile()

	if !strings.Contains(cf, "email admin@example.com") {
		t.Error("expected email directive")
	}
	if !strings.Contains(cf, "admin off") {
		t.Error("expected 'admin off'")
	}
	if !strings.Contains(cf, "example.com") {
		t.Error("expected domain in server block")
	}
	if !strings.Contains(cf, "reverse_proxy 127.0.0.1:10443") {
		t.Error("expected reverse_proxy to internal addr")
	}
	// Should NOT have auto_https off for domain-based config
	if strings.Contains(cf, "auto_https off") {
		t.Error("should not have 'auto_https off' for domain config")
	}
}

func TestProxy_buildCaddyfile_ManualTLS(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":443",
		InternalAddr: "127.0.0.1:10443",
		TLSCert:      "/path/to/cert.pem",
		TLSKey:       "/path/to/key.pem",
	})

	cf := p.buildCaddyfile()

	if !strings.Contains(cf, "tls /path/to/cert.pem /path/to/key.pem") {
		t.Error("expected tls directive with cert and key paths")
	}
	if !strings.Contains(cf, "auto_https off") {
		t.Error("expected 'auto_https off' for manual TLS without gateway")
	}
	if !strings.Contains(cf, ":443") {
		t.Error("expected listen address ':443'")
	}
}

func TestProxy_buildCaddyfile_ManualTLS_WithGateway(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":443",
		InternalAddr: "127.0.0.1:10443",
		TLSCert:      "/path/to/cert.pem",
		TLSKey:       "/path/to/key.pem",
		Gateway:      "/etc/caddy/gateway.conf",
	})

	cf := p.buildCaddyfile()

	// With gateway, should NOT have auto_https off (gateway sites need auto certs)
	if strings.Contains(cf, "auto_https off") {
		t.Error("should not have 'auto_https off' when gateway is set with manual TLS")
	}
	if !strings.Contains(cf, "import /etc/caddy/gateway.conf") {
		t.Error("expected gateway import")
	}
}

func TestProxy_buildCaddyfile_HTTPWithGateway(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		Gateway:      "/etc/caddy/gateway.conf",
	})

	cf := p.buildCaddyfile()

	// With gateway, should NOT have auto_https off
	if strings.Contains(cf, "auto_https off") {
		t.Error("should not have 'auto_https off' when gateway is set")
	}
	if !strings.Contains(cf, "import /etc/caddy/gateway.conf") {
		t.Error("expected gateway import")
	}
}

func TestProxy_buildCaddyfile_NoGateway(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
	})

	cf := p.buildCaddyfile()

	if strings.Contains(cf, "import") {
		t.Error("should not have import when no gateway is configured")
	}
}

func TestProxy_StartAndStop(t *testing.T) {
	// Use an ephemeral port to avoid conflicts
	p := New(&Config{
		ListenAddr:   ":19876",
		InternalAddr: "127.0.0.1:29876",
	})

	err := p.Start()
	if err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}

	if !p.IsRunning() {
		t.Error("expected proxy to be running after Start")
	}

	// Stop should succeed
	err = p.Stop()
	if err != nil {
		t.Errorf("failed to stop proxy: %v", err)
	}

	if p.IsRunning() {
		t.Error("expected proxy to not be running after Stop")
	}

	// Second stop should be a no-op
	err = p.Stop()
	if err != nil {
		t.Errorf("second stop should not error: %v", err)
	}
}

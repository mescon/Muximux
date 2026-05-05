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
		// findings.md L8: ports above 55535 would push +10000 past the
		// 16-bit port range, so those wrap down to portNum-10000.
		{":60000", "127.0.0.1:50000"},
		{":65000", "127.0.0.1:55000"},
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

func TestProxy_buildCaddyfile_ManualTLS_WithAutoGatewaySite(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":443",
		InternalAddr: "127.0.0.1:10443",
		TLSCert:      "/path/to/cert.pem",
		TLSKey:       "/path/to/key.pem",
		GatewaySites: []GatewaySite{{
			Domain:     "sonarr.example.com",
			BackendURL: "http://sonarr:8989",
			// TLS unset == "auto" → Caddy needs ports 80/443 for the cert
		}},
	})

	cf := p.buildCaddyfile()

	// auto_https must stay enabled so Caddy can issue the LE cert for the site.
	if strings.Contains(cf, "auto_https off") {
		t.Error("should not have 'auto_https off' when a structured site needs auto-HTTPS")
	}
	if !strings.Contains(cf, "sonarr.example.com {") {
		t.Error("expected the gateway site block to be emitted")
	}
}

func TestProxy_buildCaddyfile_HTTPWithAutoGatewaySite(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{{
			Domain:     "grafana.example.com",
			BackendURL: "http://grafana:3000",
		}},
	})

	cf := p.buildCaddyfile()

	if strings.Contains(cf, "auto_https off") {
		t.Error("should not have 'auto_https off' when a structured site needs auto-HTTPS")
	}
	if !strings.Contains(cf, "grafana.example.com {") {
		t.Error("expected the gateway site block to be emitted")
	}
}

func TestProxy_buildCaddyfile_HTTPWithNoneTLSSite(t *testing.T) {
	// A site with tls=none must not force auto_https on. Caddy only
	// listens on the listen port; the http:// prefix on the site
	// selector tells Caddy to serve plaintext.
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{{
			Domain:     "internal.lan",
			BackendURL: "http://app:8080",
			TLS:        "none",
		}},
	})

	cf := p.buildCaddyfile()

	if !strings.Contains(cf, "auto_https off") {
		t.Error("expected 'auto_https off' when no site needs auto-HTTPS")
	}
	if !strings.Contains(cf, "http://internal.lan {") {
		t.Error("expected http:// site selector for tls=none site")
	}
}

func TestProxy_buildCaddyfile_GatewaySite_StreamingFlushInterval(t *testing.T) {
	// Plex / Jellyfin / Grafana use case: long-lived response streams
	// must not be buffered.
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{{
			Domain:     "plex.example.com",
			BackendURL: "http://plex:32400",
			Streaming:  true,
		}},
	})

	cf := p.buildCaddyfile()

	if !strings.Contains(cf, "flush_interval -1") {
		t.Error("expected 'flush_interval -1' when streaming=true")
	}
}

func TestProxy_buildCaddyfile_GatewaySite_StripFrameBlockers(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{{
			Domain:             "embedded.example.com",
			BackendURL:         "http://app:8080",
			StripFrameBlockers: true,
		}},
	})

	cf := p.buildCaddyfile()

	if !strings.Contains(cf, "header -X-Frame-Options") {
		t.Error("expected X-Frame-Options strip directive")
	}
	if !strings.Contains(cf, "Content-Security-Policy") {
		t.Error("expected CSP frame-ancestors directive")
	}
	if !strings.Contains(cf, "frame-ancestors 'self'") {
		t.Error("expected frame-ancestors 'self' value")
	}
}

func TestProxy_buildCaddyfile_GatewaySite_ProxyHeaders(t *testing.T) {
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{{
			Domain:     "sonarr.example.com",
			BackendURL: "http://sonarr:8989",
			ProxyHeaders: map[string]string{
				"X-Api-Key": "abc-123",
			},
		}},
	})

	cf := p.buildCaddyfile()

	if !strings.Contains(cf, "header_up X-Api-Key") {
		t.Error("expected upstream X-Api-Key header injection")
	}
	if !strings.Contains(cf, `"abc-123"`) {
		t.Error("expected quoted header value")
	}
}

func TestProxy_buildCaddyfile_GatewaySite_ForwardedHeadersDisabled(t *testing.T) {
	off := false
	p := New(&Config{
		ListenAddr:   ":8080",
		InternalAddr: "127.0.0.1:18080",
		GatewaySites: []GatewaySite{{
			Domain:           "noxff.example.com",
			BackendURL:       "http://app:8080",
			ForwardedHeaders: &off,
		}},
	})

	cf := p.buildCaddyfile()

	// The site block should not include the X-Forwarded-* headers when explicitly disabled.
	siteStart := strings.Index(cf, "noxff.example.com {")
	if siteStart < 0 {
		t.Fatal("site block missing")
	}
	// Slice from the site to find anything in its block; X-Forwarded headers
	// would still appear in Muximux's own block, so we just check the slice
	// from the site start onward.
	suffix := cf[siteStart:]
	if strings.Contains(suffix, "X-Forwarded-Proto") {
		t.Error("X-Forwarded-Proto should not appear in this site's block")
	}
	if strings.Contains(suffix, "X-Real-IP") {
		t.Error("X-Real-IP should not appear in this site's block")
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

// TestValidate covers the dry-run linter handlers will call before
// persisting a candidate change. Catches Caddyfile syntax errors at
// parse time so the operator sees a useful message before any reload.
func TestValidate(t *testing.T) {
	t.Run("clean Caddyfile parses", func(t *testing.T) {
		caddyfile := `{
			auto_https off
			admin off
		}

		:18081 {
			reverse_proxy 127.0.0.1:28081
		}`
		if err := Validate(caddyfile); err != nil {
			t.Errorf("expected clean parse, got: %v", err)
		}
	})

	t.Run("syntax error surfaces", func(t *testing.T) {
		// Unclosed brace is a parse error.
		caddyfile := `:18082 {
			reverse_proxy 127.0.0.1:28082
		`
		err := Validate(caddyfile)
		if err == nil {
			t.Fatal("expected an error for unclosed brace")
		}
		if !strings.Contains(err.Error(), "caddyfile invalid") {
			t.Errorf("error should mention caddyfile invalid, got: %v", err)
		}
	})

	t.Run("unknown directive surfaces", func(t *testing.T) {
		caddyfile := `:18083 {
			definitely_not_a_caddy_directive arg
		}`
		err := Validate(caddyfile)
		if err == nil {
			t.Fatal("expected an error for unknown directive")
		}
	})

	t.Run("empty input is rejected", func(t *testing.T) {
		// Caddy's adapter rejects an empty Caddyfile with an EOF error.
		// That's the right behaviour for our purposes too: a caller
		// asking us to validate emptiness is a caller about to ship a
		// config that would refuse to load.
		err := Validate("")
		if err == nil {
			t.Fatal("expected empty input to be rejected")
		}
		if !strings.Contains(err.Error(), "caddyfile invalid") {
			t.Errorf("error should mention caddyfile invalid, got: %v", err)
		}
	})
}

// TestProxy_Reload exercises the in-process reload path that the
// gateway-sites Settings UI will drive. Reload from a not-yet-running
// state is the cold-start; a follow-up Reload after mutating config
// is the hot-reload. Both must produce a running Caddy that serves
// the latest config without leaking state from the prior config.
func TestProxy_Reload(t *testing.T) {
	t.Run("cold reload starts caddy from a stopped state", func(t *testing.T) {
		p := New(&Config{
			ListenAddr:   ":19880",
			InternalAddr: "127.0.0.1:29880",
		})

		if err := p.Reload(); err != nil {
			t.Fatalf("cold Reload failed: %v", err)
		}
		if !p.IsRunning() {
			t.Error("expected proxy to be running after cold Reload")
		}
		if err := p.Stop(); err != nil {
			t.Errorf("Stop after cold Reload failed: %v", err)
		}
	})

	t.Run("hot reload swaps config on a running instance", func(t *testing.T) {
		p := New(&Config{
			ListenAddr:   ":19881",
			InternalAddr: "127.0.0.1:29881",
		})

		if err := p.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		t.Cleanup(func() { _ = p.Stop() })

		// Mutate the config the way a Settings change would. The next
		// Reload must apply the new internal address. We don't bind the
		// new addr in this test (no upstream is required to validate
		// the load); we only assert that Reload returns nil.
		p.config.InternalAddr = "127.0.0.1:29882"

		if err := p.Reload(); err != nil {
			t.Fatalf("hot Reload failed: %v", err)
		}
		if !p.IsRunning() {
			t.Error("expected proxy to remain running after hot Reload")
		}
	})
}

package handlers

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// TestContentRewriter tests URL rewriting in HTML/CSS/JS content
func TestContentRewriter(t *testing.T) {
	tests := []struct {
		name        string
		proxyPrefix string
		targetPath  string
		targetHost  string
		input       string
		expected    string
	}{
		{
			name:        "rewrite absolute URL with target host",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			targetHost:  "192.0.2.100",
			input:       `<a href="http://192.0.2.100/admin/settings">`,
			expected:    `<a href="/proxy/app/settings">`,
		},
		{
			name:        "rewrite absolute URL without target path",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			targetHost:  "192.0.2.100",
			input:       `<a href="http://192.0.2.100/other/path">`,
			expected:    `<a href="/proxy/app/other/path">`,
		},
		{
			name:        "rewrite target path in href",
			proxyPrefix: "/proxy/pihole",
			targetPath:  "/admin",
			targetHost:  "",
			input:       `<a href="/admin/settings">`,
			expected:    `<a href="/proxy/pihole/settings">`,
		},
		{
			name:        "rewrite target path exact match",
			proxyPrefix: "/proxy/pihole",
			targetPath:  "/admin",
			targetHost:  "",
			input:       `<a href="/admin">`,
			expected:    `<a href="/proxy/pihole">`,
		},
		{
			name:        "rewrite root-relative path",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "",
			targetHost:  "",
			input:       `<link href="/Content/styles.css">`,
			expected:    `<link href="/proxy/sonarr/Content/styles.css">`,
		},
		{
			name:        "rewrite src attribute",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<script src="/js/app.js"></script>`,
			expected:    `<script src="/proxy/app/js/app.js"></script>`,
		},
		{
			name:        "rewrite CSS url()",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `background: url("/images/bg.png")`,
			expected:    `background: url("/proxy/app/images/bg.png")`,
		},
		{
			name:        "rewrite JS string literal",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			targetHost:  "",
			input:       `fetch("/admin/api/data")`,
			expected:    `fetch("/proxy/app/api/data")`,
		},
		{
			name:        "don't double-rewrite already proxied paths",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<a href="/proxy/app/page">`,
			expected:    `<a href="/proxy/app/page">`,
		},
		{
			name:        "rewrite base href",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			targetHost:  "",
			input:       `<base href="/admin/">`,
			expected:    `<base href="/proxy/app/">`,
		},
		{
			name:        "rewrite data attributes",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<div data-url="/api/endpoint">`,
			expected:    `<div data-url="/proxy/app/api/endpoint">`,
		},
		{
			name:        "handle app without subpath (like Sonarr)",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "/",
			targetHost:  "192.0.2.42:8989",
			input:       `<link rel="icon" href="/Content/Images/Icons/favicon.png">`,
			expected:    `<link rel="icon" href="/proxy/sonarr/Content/Images/Icons/favicon.png">`,
		},
		{
			name:        "rewrite JSON urlBase empty string",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "",
			targetHost:  "",
			input:       `{"urlBase": "", "version": "1.0"}`,
			expected:    `{"urlBase": "/proxy/sonarr", "version": "1.0"}`,
		},
		{
			name:        "rewrite JSON apiRoot path",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "",
			targetHost:  "",
			input:       `{"apiRoot": "/api/v3", "urlBase": ""}`,
			expected:    `{"apiRoot": "/proxy/sonarr/api/v3", "urlBase": "/proxy/sonarr"}`,
		},
		{
			name:        "rewrite JSON generic path keys",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `{"redirectUrl": "/login", "assetsPath": "/static/assets"}`,
			expected:    `{"redirectUrl": "/proxy/app/login", "assetsPath": "/proxy/app/static/assets"}`,
		},
		{
			name:        "don't double-rewrite JSON paths",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `{"apiRoot": "/proxy/app/api"}`,
			expected:    `{"apiRoot": "/proxy/app/api"}`,
		},
		{
			name:        "rewrite JS urlBase assignment",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "",
			targetHost:  "",
			input:       `window.Sonarr = { urlBase: '' };`,
			expected:    `window.Sonarr = { urlBase: "/proxy/sonarr" };`,
		},
		{
			name:        "rewrite JSON array of paths",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `{"images": ["/img1.jpg", "/img2.jpg"]}`,
			expected:    `{"images": ["/proxy/app/img1.jpg", "/proxy/app/img2.jpg"]}`,
		},
		{
			name:        "rewrite CSS @import",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `@import "/styles/main.css";`,
			expected:    `@import "/proxy/app/styles/main.css";`,
		},
		{
			name:        "rewrite CSS @import url()",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `@import url("/styles/main.css");`,
			expected:    `@import url("/proxy/app/styles/main.css");`,
		},
		{
			name:        "rewrite srcset attribute",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<img srcset="/img/sm.jpg 1x, /img/lg.jpg 2x">`,
			expected:    `<img srcset="/proxy/app/img/sm.jpg 1x, /proxy/app/img/lg.jpg 2x">`,
		},
		{
			name:        "rewrite SVG use href",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<use href="/icons.svg#menu"></use>`,
			expected:    `<use href="/proxy/app/icons.svg#menu"></use>`,
		},
		{
			name:        "rewrite SVG image href",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<image href="/images/logo.png" />`,
			expected:    `<image href="/proxy/app/images/logo.png" />`,
		},
		{
			name:        "rewrite CSS image-set()",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `background: image-set("/1x.png" 1x, "/2x.png" 2x)`,
			expected:    `background: image-set("/proxy/app/1x.png" 1x, "/proxy/app/2x.png" 2x)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewriter := newContentRewriter(tt.proxyPrefix, tt.targetPath, tt.targetHost)
			result := string(rewriter.rewrite([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewrite() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestCookiePathRewriting tests Set-Cookie path rewriting
func TestCookiePathRewriting(t *testing.T) {
	tests := []struct {
		name        string
		proxyPrefix string
		targetPath  string
		cookie      string
		expected    string
	}{
		{
			name:        "rewrite cookie with target path",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			cookie:      "session=abc123; Path=/admin; HttpOnly",
			expected:    "session=abc123; Path=/proxy/app/; HttpOnly",
		},
		{
			name:        "rewrite cookie with root path",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "session=abc123; Path=/; HttpOnly",
			expected:    "session=abc123; Path=/proxy/app/; HttpOnly",
		},
		{
			name:        "rewrite cookie path with subpath",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			cookie:      "token=xyz; Path=/admin/api; Secure",
			expected:    "token=xyz; Path=/proxy/app/api; Secure",
		},
		{
			name:        "don't rewrite already correct path",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "session=abc; Path=/proxy/app; HttpOnly",
			expected:    "session=abc; Path=/proxy/app; HttpOnly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewriter := newContentRewriter(tt.proxyPrefix, tt.targetPath, "")
			result := rewriter.rewriteCookiePath(tt.cookie)
			if result != tt.expected {
				t.Errorf("rewriteCookiePath() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestRewriteLocation tests Location header rewriting for redirects
func TestRewriteLocation(t *testing.T) {
	tests := []struct {
		name        string
		location    string
		proxyPrefix string
		targetPath  string
		targetHost  string
		expected    string
	}{
		{
			name:        "rewrite location with target path",
			location:    "/admin/dashboard",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			targetHost:  "",
			expected:    "/proxy/app/dashboard",
		},
		{
			name:        "rewrite location exact target path",
			location:    "/admin",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			targetHost:  "",
			expected:    "/proxy/app/",
		},
		{
			name:        "rewrite root-relative location",
			location:    "/login",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			expected:    "/proxy/app/login",
		},
		{
			name:        "don't rewrite already proxied location",
			location:    "/proxy/app/page",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			expected:    "/proxy/app/page",
		},
		{
			name:        "don't rewrite absolute URLs to different host",
			location:    "https://external.com/page",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "192.0.2.10:32400",
			expected:    "https://external.com/page",
		},
		{
			name:        "rewrite absolute URL matching target host",
			location:    "http://192.0.2.10:32400/web/index.html",
			proxyPrefix: "/proxy/myapp",
			targetPath:  "",
			targetHost:  "192.0.2.10:32400",
			expected:    "/proxy/myapp/web/index.html",
		},
		{
			name:        "rewrite absolute URL with query string",
			location:    "http://192.0.2.10:32400/web/index.html?redirect=1",
			proxyPrefix: "/proxy/myapp",
			targetPath:  "",
			targetHost:  "192.0.2.10:32400",
			expected:    "/proxy/myapp/web/index.html?redirect=1",
		},
		{
			name:        "handle API path redirect",
			location:    "/api/auth",
			proxyPrefix: "/proxy/pihole",
			targetPath:  "/admin",
			targetHost:  "",
			expected:    "/proxy/pihole/api/auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteLocation(tt.location, tt.proxyPrefix, tt.targetPath, tt.targetHost)
			if result != tt.expected {
				t.Errorf("rewriteLocation() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestDirectorPathMapping tests that the Director correctly maps proxy paths to backend paths
func TestDirectorPathMapping(t *testing.T) {
	tests := []struct {
		name           string
		appName        string
		appURL         string
		requestPath    string
		expectedPath   string
		expectedHost   string
		expectedScheme string
	}{
		{
			name:           "app at root path",
			appName:        "Sonarr",
			appURL:         "http://192.0.2.42:8989",
			requestPath:    "/proxy/sonarr/api/series",
			expectedPath:   "/api/series",
			expectedHost:   "192.0.2.42:8989",
			expectedScheme: "http",
		},
		{
			name:           "app with subpath",
			appName:        "Pi-hole",
			appURL:         "http://192.0.2.100/admin",
			requestPath:    "/proxy/pi-hole/settings",
			expectedPath:   "/admin/settings",
			expectedHost:   "192.0.2.100",
			expectedScheme: "http",
		},
		{
			name:           "app with subpath - API at root",
			appName:        "Pi-hole",
			appURL:         "http://192.0.2.100/admin",
			requestPath:    "/proxy/pi-hole/api/auth",
			expectedPath:   "/api/auth",
			expectedHost:   "192.0.2.100",
			expectedScheme: "http",
		},
		{
			name:           "app at root - root request",
			appName:        "Sonarr",
			appURL:         "http://192.0.2.42:8989",
			requestPath:    "/proxy/sonarr/",
			expectedPath:   "/",
			expectedHost:   "192.0.2.42:8989",
			expectedScheme: "http",
		},
		{
			name:           "app with subpath - root request",
			appName:        "Pi-hole",
			appURL:         "http://192.0.2.100/admin",
			requestPath:    "/proxy/pi-hole/",
			expectedPath:   "/admin/",
			expectedHost:   "192.0.2.100",
			expectedScheme: "http",
		},
		{
			name:           "HTTPS app",
			appName:        "SecureApp",
			appURL:         "https://secure.example.com/app",
			requestPath:    "/proxy/secureapp/dashboard",
			expectedPath:   "/app/dashboard",
			expectedHost:   "secure.example.com",
			expectedScheme: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler with the test app
			apps := []config.AppConfig{
				{
					Name:    tt.appName,
					URL:     tt.appURL,
					Enabled: true,
					Proxy:   true,
				},
			}
			handler := NewReverseProxyHandler(apps)

			// Find the route
			slug := slugify(tt.appName)
			route, exists := handler.routes[slug]
			if !exists {
				t.Fatalf("route for %q not found (slug: %q)", tt.appName, slug)
			}

			// Create a test request
			req := httptest.NewRequest("GET", tt.requestPath, nil)

			// Apply the director
			route.proxy.Director(req)

			// Verify the transformed request
			if req.URL.Path != tt.expectedPath {
				t.Errorf("Path = %q, want %q", req.URL.Path, tt.expectedPath)
			}
			if req.URL.Host != tt.expectedHost {
				t.Errorf("Host = %q, want %q", req.URL.Host, tt.expectedHost)
			}
			if req.URL.Scheme != tt.expectedScheme {
				t.Errorf("Scheme = %q, want %q", req.URL.Scheme, tt.expectedScheme)
			}
		})
	}
}

// TestProxyRouteCreation tests that proxy routes are correctly created from config
func TestProxyRouteCreation(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "App One", URL: "http://host1:8080", Enabled: true, Proxy: true},
		{Name: "App Two", URL: "http://host2:9090/subpath", Enabled: true, Proxy: true},
		{Name: "Disabled App", URL: "http://host3:7070", Enabled: false, Proxy: true},
		{Name: "Non-Proxy App", URL: "http://host4:6060", Enabled: true, Proxy: false},
		{Name: "App with Spaces", URL: "http://host5:5050", Enabled: true, Proxy: true},
	}

	handler := NewReverseProxyHandler(apps)

	// Should have routes for enabled proxy apps only
	expectedSlugs := []string{"app-one", "app-two", "app-with-spaces"}
	for _, slug := range expectedSlugs {
		if _, exists := handler.routes[slug]; !exists {
			t.Errorf("expected route %q to exist", slug)
		}
	}

	// Should NOT have routes for disabled or non-proxy apps
	unexpectedSlugs := []string{"disabled-app", "non-proxy-app"}
	for _, slug := range unexpectedSlugs {
		if _, exists := handler.routes[slug]; exists {
			t.Errorf("did not expect route %q to exist", slug)
		}
	}

	// Verify route count
	if len(handler.routes) != len(expectedSlugs) {
		t.Errorf("expected %d routes, got %d", len(expectedSlugs), len(handler.routes))
	}
}

// TestProxyServeHTTP tests the HTTP handler routing
func TestProxyServeHTTP(t *testing.T) {
	// Create a mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK from backend"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)

	apps := []config.AppConfig{
		{Name: "TestApp", URL: backend.URL, Enabled: true, Proxy: true},
	}

	handler := NewReverseProxyHandler(apps)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedPath   string
	}{
		{
			name:           "valid proxy request",
			path:           "/proxy/testapp/page",
			expectedStatus: http.StatusOK,
			expectedPath:   "/page",
		},
		{
			name:           "root proxy request",
			path:           "/proxy/testapp/",
			expectedStatus: http.StatusOK,
			expectedPath:   "/",
		},
		{
			name:           "unknown app",
			path:           "/proxy/unknown/page",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid proxy path",
			path:           "/proxy/",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			if tt.expectedPath != "" {
				gotPath := rec.Header().Get("X-Backend-Path")
				if gotPath != tt.expectedPath {
					t.Errorf("backend received path %q, want %q", gotPath, tt.expectedPath)
				}
			}
		})
	}

	_ = backendURL // Used for debugging if needed
}

// TestShouldRewriteContent tests content-type detection for rewriting
func TestShouldRewriteContent(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"text/html", true},
		{"text/html; charset=utf-8", true},
		{"text/css", true},
		{"text/javascript", true},
		{"application/javascript", true},
		{"application/json", true},
		{"application/xml", true},
		{"text/xml", true},
		{"image/png", false},
		{"image/jpeg", false},
		{"application/octet-stream", false},
		{"font/woff2", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := shouldRewriteContent(tt.contentType)
			if result != tt.expected {
				t.Errorf("shouldRewriteContent(%q) = %v, want %v", tt.contentType, result, tt.expected)
			}
		})
	}
}

// Note: TestSlugify is in api_test.go

// Integration test: verify full request/response cycle with content rewriting
func TestProxyIntegration(t *testing.T) {
	// Create a mock backend that returns HTML with paths to rewrite
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := `<!DOCTYPE html>
<html>
<head>
	<link rel="stylesheet" href="/static/style.css">
	<script src="/js/app.js"></script>
</head>
<body>
	<a href="/dashboard">Dashboard</a>
	<a href="/api/data">API</a>
</body>
</html>`
		w.Write([]byte(html))
	}))
	defer backend.Close()

	apps := []config.AppConfig{
		{Name: "TestApp", URL: backend.URL, Enabled: true, Proxy: true},
	}

	handler := NewReverseProxyHandler(apps)

	req := httptest.NewRequest("GET", "/proxy/testapp/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Check that paths were rewritten
	expectedRewrites := []string{
		`href="/proxy/testapp/static/style.css"`,
		`src="/proxy/testapp/js/app.js"`,
		`href="/proxy/testapp/dashboard"`,
		`href="/proxy/testapp/api/data"`,
	}

	for _, expected := range expectedRewrites {
		if !strings.Contains(body, expected) {
			t.Errorf("expected body to contain %q, got:\n%s", expected, body)
		}
	}

	// Check that original paths are NOT present (except in the test expectation comments)
	unexpectedPaths := []string{
		`href="/static/`,
		`src="/js/`,
		`href="/dashboard"`,
		`href="/api/data"`,
	}

	for _, unexpected := range unexpectedPaths {
		if strings.Contains(body, unexpected) {
			t.Errorf("body should not contain unrewritten path %q", unexpected)
		}
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "standard websocket upgrade",
			headers:  map[string]string{"Connection": "upgrade", "Upgrade": "websocket"},
			expected: true,
		},
		{
			name:     "case insensitive",
			headers:  map[string]string{"Connection": "Upgrade", "Upgrade": "WebSocket"},
			expected: true,
		},
		{
			name:     "multi-value connection header",
			headers:  map[string]string{"Connection": "keep-alive, Upgrade", "Upgrade": "websocket"},
			expected: true,
		},
		{
			name:     "missing upgrade header",
			headers:  map[string]string{"Connection": "upgrade"},
			expected: false,
		},
		{
			name:     "missing connection header",
			headers:  map[string]string{"Upgrade": "websocket"},
			expected: false,
		},
		{
			name:     "normal HTTP request",
			headers:  map[string]string{"Content-Type": "text/html"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/proxy/app/ws", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if got := isWebSocketUpgrade(req); got != tt.expected {
				t.Errorf("isWebSocketUpgrade() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResolveBackendPath(t *testing.T) {
	tests := []struct {
		name        string
		proxyPrefix string
		targetPath  string
		requestPath string
		expected    string
	}{
		{
			name:        "app at root - simple path",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "/",
			requestPath: "/proxy/sonarr/api/v3/series",
			expected:    "/api/v3/series",
		},
		{
			name:        "app with subpath",
			proxyPrefix: "/proxy/pi-hole",
			targetPath:  "/admin",
			requestPath: "/proxy/pi-hole/settings",
			expected:    "/admin/settings",
		},
		{
			name:        "app with subpath - API at root",
			proxyPrefix: "/proxy/pi-hole",
			targetPath:  "/admin",
			requestPath: "/proxy/pi-hole/api/auth",
			expected:    "/api/auth",
		},
		{
			name:        "root request",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "/",
			requestPath: "/proxy/sonarr/",
			expected:    "/",
		},
		{
			name:        "websocket path",
			proxyPrefix: "/proxy/portainer",
			targetPath:  "/",
			requestPath: "/proxy/portainer/api/websocket/exec",
			expected:    "/api/websocket/exec",
		},
		{
			name:        "double prefix stripping",
			proxyPrefix: "/proxy/radarr",
			targetPath:  "/",
			requestPath: "/proxy/radarr/proxy/radarr/api/v3",
			expected:    "/api/v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetURL, _ := url.Parse("http://192.0.2.1:8080" + tt.targetPath)
			route := &proxyRoute{
				proxyPrefix: tt.proxyPrefix,
				targetPath:  tt.targetPath,
				targetURL:   targetURL,
			}
			got := route.resolveBackendPath(tt.requestPath)
			if got != tt.expected {
				t.Errorf("resolveBackendPath(%q) = %q, want %q", tt.requestPath, got, tt.expected)
			}
		})
	}
}

// computeWebSocketAccept computes the Sec-WebSocket-Accept value per RFC 6455
func computeWebSocketAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-5AB5DC11D65A"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func TestWebSocketProxy(t *testing.T) {
	// Create a mock WebSocket backend that performs the upgrade handshake
	// and echoes back a message
	wsBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a WebSocket upgrade
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "not a websocket request", http.StatusBadRequest)
			return
		}

		key := r.Header.Get("Sec-WebSocket-Key")
		accept := computeWebSocketAccept(key)

		// Hijack the connection to perform raw WebSocket
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack not supported", http.StatusInternalServerError)
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()

		// Send 101 upgrade response
		fmt.Fprintf(buf, "HTTP/1.1 101 Switching Protocols\r\n")
		fmt.Fprintf(buf, "Upgrade: websocket\r\n")
		fmt.Fprintf(buf, "Connection: Upgrade\r\n")
		fmt.Fprintf(buf, "Sec-WebSocket-Accept: %s\r\n", accept)
		fmt.Fprintf(buf, "\r\n")
		buf.Flush()

		// Read whatever the client sends and echo it back
		msg := make([]byte, 1024)
		n, err := conn.Read(msg)
		if err != nil {
			return
		}
		_, _ = conn.Write(msg[:n])
	}))
	defer wsBackend.Close()

	// Create proxy handler pointing to the backend
	apps := []config.AppConfig{
		{Name: "WsApp", URL: wsBackend.URL, Enabled: true, Proxy: true},
	}
	handler := NewReverseProxyHandler(apps)

	// Start an HTTP server using our proxy handler
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	// Connect to the proxy as a WebSocket client
	proxyURL, _ := url.Parse(proxyServer.URL)
	conn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// Send WebSocket upgrade request through the proxy
	wsKey := "dGhlIHNhbXBsZSBub25jZQ=="
	upgrade := fmt.Sprintf(
		"GET /proxy/wsapp/ws HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Connection: upgrade\r\n"+
			"Upgrade: websocket\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n"+
			"\r\n",
		proxyURL.Host, wsKey)

	if _, err := conn.Write([]byte(upgrade)); err != nil {
		t.Fatalf("failed to send upgrade request: %v", err)
	}

	// Read the proxy's response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("failed to read upgrade response: %v", err)
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	if !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") {
		t.Errorf("expected Upgrade: websocket header, got %q", resp.Header.Get("Upgrade"))
	}

	expectedAccept := computeWebSocketAccept(wsKey)
	if resp.Header.Get("Sec-WebSocket-Accept") != expectedAccept {
		t.Errorf("Sec-WebSocket-Accept = %q, want %q", resp.Header.Get("Sec-WebSocket-Accept"), expectedAccept)
	}

	// Send a test message through the WebSocket
	testMsg := []byte("hello from proxy test")
	if _, err := conn.Write(testMsg); err != nil {
		t.Fatalf("failed to write test message: %v", err)
	}

	// Read the echo back
	echoBuf := make([]byte, 1024)
	n, err := conn.Read(echoBuf)
	if err != nil {
		t.Fatalf("failed to read echo: %v", err)
	}

	if string(echoBuf[:n]) != string(testMsg) {
		t.Errorf("echo = %q, want %q", string(echoBuf[:n]), string(testMsg))
	}
}

func TestWebSocketNon101Response(t *testing.T) {
	// Backend that rejects WebSocket upgrades
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer backend.Close()

	apps := []config.AppConfig{
		{Name: "NoWs", URL: backend.URL, Enabled: true, Proxy: true},
	}
	handler := NewReverseProxyHandler(apps)

	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	conn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	upgrade := fmt.Sprintf(
		"GET /proxy/nows/ws HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Connection: upgrade\r\n"+
			"Upgrade: websocket\r\n"+
			"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"+
			"Sec-WebSocket-Version: 13\r\n"+
			"\r\n",
		proxyURL.Host)

	_, _ = conn.Write([]byte(upgrade))

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	// Should get the backend's 403, not a crash
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// TestRewriteLocationHeaders tests that Location, Content-Location, and Refresh headers
// are properly rewritten to use the proxy prefix.
func TestRewriteLocationHeaders(t *testing.T) {
	tests := []struct {
		name            string
		proxyPrefix     string
		targetPath      string
		targetHost      string
		headers         map[string]string
		expectedHeaders map[string]string
	}{
		{
			name:        "rewrite Location",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			targetHost:  "",
			headers:     map[string]string{"Location": "/admin/dashboard"},
			expectedHeaders: map[string]string{
				"Location": "/proxy/app/dashboard",
			},
		},
		{
			name:        "rewrite Content-Location",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			headers:     map[string]string{"Content-Location": "/api/data"},
			expectedHeaders: map[string]string{
				"Content-Location": "/proxy/app/api/data",
			},
		},
		{
			name:        "rewrite Refresh header",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			headers:     map[string]string{"Refresh": "5; url=/login"},
			expectedHeaders: map[string]string{
				"Refresh": "5; url=/proxy/app/login",
			},
		},
		{
			name:        "no Location header",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			headers:     map[string]string{},
			expectedHeaders: map[string]string{
				"Location":         "",
				"Content-Location": "",
			},
		},
		{
			name:        "absolute URL Location matching target host",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "192.0.2.10:8080",
			headers:     map[string]string{"Location": "http://192.0.2.10:8080/page"},
			expectedHeaders: map[string]string{
				"Location": "/proxy/app/page",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}
			for k, v := range tt.headers {
				resp.Header.Set(k, v)
			}

			rewriteLocationHeaders(resp, tt.proxyPrefix, tt.targetPath, tt.targetHost)

			for k, expected := range tt.expectedHeaders {
				got := resp.Header.Get(k)
				if got != expected {
					t.Errorf("header %q = %q, want %q", k, got, expected)
				}
			}
		})
	}
}

// TestRewriteCookieHeaders tests that Set-Cookie Path attributes are rewritten.
func TestRewriteCookieHeaders(t *testing.T) {
	tests := []struct {
		name        string
		proxyPrefix string
		targetPath  string
		cookies     []string
		wantCookies []string
	}{
		{
			name:        "single cookie with path",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			cookies:     []string{"session=abc; Path=/admin; HttpOnly"},
			wantCookies: []string{"session=abc; Path=/proxy/app/; HttpOnly"},
		},
		{
			name:        "multiple cookies",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookies: []string{
				"token=xyz; Path=/; Secure",
				"pref=dark; Path=/settings; HttpOnly",
			},
			wantCookies: []string{
				"token=xyz; Path=/proxy/app/; Secure",
				"pref=dark; Path=/proxy/app/settings; HttpOnly",
			},
		},
		{
			name:        "no cookies",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookies:     nil,
			wantCookies: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}
			for _, c := range tt.cookies {
				resp.Header.Add("Set-Cookie", c)
			}

			rewriter := newContentRewriter(tt.proxyPrefix, tt.targetPath, "")
			rewriteCookieHeaders(resp, rewriter)

			got := resp.Header.Values("Set-Cookie")
			if len(got) != len(tt.wantCookies) {
				t.Fatalf("cookie count = %d, want %d", len(got), len(tt.wantCookies))
			}
			for i, want := range tt.wantCookies {
				if got[i] != want {
					t.Errorf("cookie[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestRewriteLinkHeaders tests that Link header paths are rewritten.
func TestRewriteLinkHeaders(t *testing.T) {
	tests := []struct {
		name        string
		proxyPrefix string
		links       []string
		wantLinks   []string
	}{
		{
			name:        "single link",
			proxyPrefix: "/proxy/app",
			links:       []string{`</styles/main.css>; rel="preload"; as="style"`},
			wantLinks:   []string{`</proxy/app/styles/main.css>; rel="preload"; as="style"`},
		},
		{
			name:        "already proxied link",
			proxyPrefix: "/proxy/app",
			links:       []string{`</proxy/app/styles.css>; rel="preload"`},
			wantLinks:   []string{`</proxy/app/styles.css>; rel="preload"`},
		},
		{
			name:        "multiple links",
			proxyPrefix: "/proxy/app",
			links: []string{
				`</js/app.js>; rel="preload"; as="script"`,
				`</css/style.css>; rel="preload"; as="style"`,
			},
			wantLinks: []string{
				`</proxy/app/js/app.js>; rel="preload"; as="script"`,
				`</proxy/app/css/style.css>; rel="preload"; as="style"`,
			},
		},
		{
			name:        "no links",
			proxyPrefix: "/proxy/app",
			links:       nil,
			wantLinks:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}
			for _, l := range tt.links {
				resp.Header.Add("Link", l)
			}

			rewriteLinkHeaders(resp, tt.proxyPrefix)

			got := resp.Header.Values("Link")
			if len(got) != len(tt.wantLinks) {
				t.Fatalf("link count = %d, want %d", len(got), len(tt.wantLinks))
			}
			for i, want := range tt.wantLinks {
				if got[i] != want {
					t.Errorf("link[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestRewriteResponseBody tests body rewriting for different content types.
func TestRewriteResponseBody(t *testing.T) {
	t.Run("rewrites HTML body", func(t *testing.T) {
		rewriter := newContentRewriter("/proxy/app", "", "")

		body := `<html><head><link href="/style.css"></head></html>`
		resp := &http.Response{
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}
		resp.Header.Set("Content-Type", "text/html")

		err := rewriteResponseBody(resp, rewriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rewritten, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(rewritten), "/proxy/app/style.css") {
			t.Errorf("expected rewritten path, got: %s", string(rewritten))
		}
	})

	t.Run("skips binary content", func(t *testing.T) {
		rewriter := newContentRewriter("/proxy/app", "", "")

		body := "binary data here"
		resp := &http.Response{
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}
		resp.Header.Set("Content-Type", "image/png")

		err := rewriteResponseBody(resp, rewriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Body should be untouched for binary content
		result, _ := io.ReadAll(resp.Body)
		if string(result) != body {
			t.Errorf("expected body to be unchanged for binary content")
		}
	})

	t.Run("rewrites JSON body", func(t *testing.T) {
		rewriter := newContentRewriter("/proxy/app", "", "")

		body := `{"apiRoot": "/api/v3"}`
		resp := &http.Response{
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}
		resp.Header.Set("Content-Type", "application/json")

		err := rewriteResponseBody(resp, rewriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rewritten, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(rewritten), "/proxy/app/api/v3") {
			t.Errorf("expected rewritten JSON path, got: %s", string(rewritten))
		}
	})
}

// TestResolveAbsoluteLocation tests absolute URL to path resolution.
func TestResolveAbsoluteLocation(t *testing.T) {
	tests := []struct {
		name       string
		location   string
		targetHost string
		expected   string
	}{
		{
			name:       "matching host",
			location:   "http://192.0.2.10:8080/web/index.html",
			targetHost: "192.0.2.10:8080",
			expected:   "/web/index.html",
		},
		{
			name:       "matching host with query",
			location:   "http://192.0.2.10:8080/login?redirect=1",
			targetHost: "192.0.2.10:8080",
			expected:   "/login?redirect=1",
		},
		{
			name:       "different host",
			location:   "https://external.com/page",
			targetHost: "192.0.2.10:8080",
			expected:   "https://external.com/page",
		},
		{
			name:       "not an absolute URL",
			location:   "/relative/path",
			targetHost: "192.0.2.10:8080",
			expected:   "/relative/path",
		},
		{
			name:       "empty location",
			location:   "",
			targetHost: "192.0.2.10:8080",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveAbsoluteLocation(tt.location, tt.targetHost)
			if result != tt.expected {
				t.Errorf("resolveAbsoluteLocation(%q, %q) = %q, want %q",
					tt.location, tt.targetHost, result, tt.expected)
			}
		})
	}
}

// TestRewritePathWithTarget tests path rewriting with target path stripping.
func TestRewritePathWithTarget(t *testing.T) {
	tests := []struct {
		name        string
		location    string
		proxyPrefix string
		targetPath  string
		expected    string
	}{
		{
			name:        "strip target path",
			location:    "/admin/settings",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			expected:    "/proxy/app/settings",
		},
		{
			name:        "exact target path",
			location:    "/admin",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			expected:    "/proxy/app/",
		},
		{
			name:        "no target path",
			location:    "/login",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			expected:    "/proxy/app/login",
		},
		{
			name:        "root target path",
			location:    "/api/data",
			proxyPrefix: "/proxy/app",
			targetPath:  "/",
			expected:    "/proxy/app/api/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewritePathWithTarget(tt.location, tt.proxyPrefix, tt.targetPath)
			if result != tt.expected {
				t.Errorf("rewritePathWithTarget(%q, %q, %q) = %q, want %q",
					tt.location, tt.proxyPrefix, tt.targetPath, result, tt.expected)
			}
		})
	}
}

// TestBuildSingleProxyRoute tests route creation from app config.
func TestBuildSingleProxyRoute(t *testing.T) {
	t.Run("valid app", func(t *testing.T) {
		app := config.AppConfig{
			Name:    "Test App",
			URL:     "http://192.0.2.1:8080/admin",
			Enabled: true,
			Proxy:   true,
		}

		route := buildSingleProxyRoute(app)

		if route == nil {
			t.Fatal("expected non-nil route")
		}
		if route.name != "Test App" {
			t.Errorf("expected name 'Test App', got %q", route.name)
		}
		if route.slug != "test-app" {
			t.Errorf("expected slug 'test-app', got %q", route.slug)
		}
		if route.proxyPrefix != "/proxy/test-app" {
			t.Errorf("expected proxyPrefix '/proxy/test-app', got %q", route.proxyPrefix)
		}
		if route.targetPath != "/admin" {
			t.Errorf("expected targetPath '/admin', got %q", route.targetPath)
		}
		if route.targetURL.Host != "192.0.2.1:8080" {
			t.Errorf("expected host '192.0.2.1:8080', got %q", route.targetURL.Host)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		app := config.AppConfig{
			Name:    "Bad",
			URL:     "://invalid",
			Enabled: true,
			Proxy:   true,
		}

		route := buildSingleProxyRoute(app)

		if route != nil {
			t.Error("expected nil route for invalid URL")
		}
	})

	t.Run("already proxy path", func(t *testing.T) {
		app := config.AppConfig{
			Name:    "Loop",
			URL:     "/proxy/loop/",
			Enabled: true,
			Proxy:   true,
		}

		route := buildSingleProxyRoute(app)

		if route != nil {
			t.Error("expected nil route for app with proxy path URL")
		}
	})

	t.Run("empty target path defaults to /", func(t *testing.T) {
		app := config.AppConfig{
			Name:    "Root",
			URL:     "http://192.0.2.1:8080",
			Enabled: true,
			Proxy:   true,
		}

		route := buildSingleProxyRoute(app)

		if route == nil {
			t.Fatal("expected non-nil route")
		}
		if route.targetPath != "/" {
			t.Errorf("expected targetPath '/', got %q", route.targetPath)
		}
	})
}

// TestResolveBackendRequestPath tests the backend path resolution logic.
func TestResolveBackendRequestPath(t *testing.T) {
	tests := []struct {
		name       string
		reqPath    string
		targetPath string
		expected   string
	}{
		{
			name:       "root target, simple path",
			reqPath:    "/page",
			targetPath: "/",
			expected:   "/page",
		},
		{
			name:       "root target, root request",
			reqPath:    "/",
			targetPath: "/",
			expected:   "/",
		},
		{
			name:       "subpath target, normal path",
			reqPath:    "/settings",
			targetPath: "/admin",
			expected:   "/admin/settings",
		},
		{
			name:       "subpath target, API path bypasses",
			reqPath:    "/api/auth",
			targetPath: "/admin",
			expected:   "/api/auth",
		},
		{
			name:       "subpath target, /api exact",
			reqPath:    "/api",
			targetPath: "/admin",
			expected:   "/api",
		},
		{
			name:       "empty target path",
			reqPath:    "/page",
			targetPath: "",
			expected:   "/page",
		},
		{
			name:       "subpath target, non-slash path",
			reqPath:    "relative",
			targetPath: "/admin",
			expected:   "/admin/relative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveBackendRequestPath(tt.reqPath, tt.targetPath)
			if result != tt.expected {
				t.Errorf("resolveBackendRequestPath(%q, %q) = %q, want %q",
					tt.reqPath, tt.targetPath, result, tt.expected)
			}
		})
	}
}

// TestSetProxyHeaders tests that proxy headers are correctly added.
func TestSetProxyHeaders(t *testing.T) {
	t.Run("basic headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/proxy/app/page", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		setProxyHeaders(req)

		if got := req.Header.Get("X-Forwarded-For"); got != "192.168.1.100" {
			t.Errorf("X-Forwarded-For = %q, want '192.168.1.100'", got)
		}
		if got := req.Header.Get("X-Real-IP"); got != "192.168.1.100" {
			t.Errorf("X-Real-IP = %q, want '192.168.1.100'", got)
		}
		if got := req.Header.Get("X-Forwarded-Proto"); got != "http" {
			t.Errorf("X-Forwarded-Proto = %q, want 'http'", got)
		}
	})

	t.Run("appends to existing X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/proxy/app/page", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", "1.2.3.4")

		setProxyHeaders(req)

		if got := req.Header.Get("X-Forwarded-For"); got != "1.2.3.4, 10.0.0.1" {
			t.Errorf("X-Forwarded-For = %q, want '1.2.3.4, 10.0.0.1'", got)
		}
	})

	t.Run("preserves existing X-Forwarded-Proto", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/proxy/app/page", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-Proto", "https")

		setProxyHeaders(req)

		if got := req.Header.Get("X-Forwarded-Proto"); got != "https" {
			t.Errorf("X-Forwarded-Proto = %q, want 'https'", got)
		}
	})
}

// TestHasRoutes and TestGetRoutes test route introspection methods.
func TestHasRoutes(t *testing.T) {
	t.Run("no routes", func(t *testing.T) {
		handler := NewReverseProxyHandler(nil)
		if handler.HasRoutes() {
			t.Error("expected HasRoutes() = false for empty handler")
		}
	})

	t.Run("with routes", func(t *testing.T) {
		apps := []config.AppConfig{
			{Name: "App", URL: "http://host:8080", Enabled: true, Proxy: true},
		}
		handler := NewReverseProxyHandler(apps)
		if !handler.HasRoutes() {
			t.Error("expected HasRoutes() = true")
		}
	})
}

func TestGetRoutes(t *testing.T) {
	apps := []config.AppConfig{
		{Name: "App One", URL: "http://host1:8080", Enabled: true, Proxy: true},
		{Name: "App Two", URL: "http://host2:9090", Enabled: true, Proxy: true},
	}
	handler := NewReverseProxyHandler(apps)

	routes := handler.GetRoutes()
	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}

	routeSet := make(map[string]bool)
	for _, r := range routes {
		routeSet[r] = true
	}
	if !routeSet["app-one"] {
		t.Error("expected route 'app-one'")
	}
	if !routeSet["app-two"] {
		t.Error("expected route 'app-two'")
	}
}

// TestStripIntegrity tests the SRI stripping logic.
func TestStripIntegrity(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strip integrity attribute",
			input:    `<script src="/app.js" integrity="sha256-abc123" crossorigin="anonymous"></script>`,
			expected: `<script src="/app.js"></script>`,
		},
		{
			name:     "strip dynamic SRI",
			input:    `b.integrity=b.sriHashes[c],`,
			expected: ``,
		},
		{
			name:     "neutralize sriHashes object",
			input:    `b.sriHashes={foo:"bar",baz:"qux"}`,
			expected: `b.sriHashes={}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriter.stripIntegrity(tt.input)
			if result != tt.expected {
				t.Errorf("stripIntegrity() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

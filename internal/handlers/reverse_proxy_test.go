package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1" //nolint:gosec // SHA-1 required by RFC 6455 WebSocket handshake
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
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
			name:        "preserve JSON apiRoot path (interceptor handles API calls)",
			proxyPrefix: "/proxy/sonarr",
			targetPath:  "",
			targetHost:  "",
			input:       `{"apiRoot": "/api/v3", "urlBase": ""}`,
			expected:    `{"apiRoot": "/api/v3", "urlBase": "/proxy/sonarr"}`,
		},
		{
			name:        "preserve JSON generic path keys (interceptor handles API calls)",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `{"redirectUrl": "/login", "assetsPath": "/static/assets"}`,
			expected:    `{"redirectUrl": "/login", "assetsPath": "/static/assets"}`,
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
			name:        "skip _baseHref in minified Angular code",
			proxyPrefix: "/proxy/ups",
			targetPath:  "",
			targetHost:  "",
			input:       `this._baseHref="",this._removeListenerFns=[]`,
			expected:    `this._baseHref="",this._removeListenerFns=[]`,
		},
		{
			name:        "skip dot-prefixed baseHref property",
			proxyPrefix: "/proxy/ups",
			targetPath:  "",
			targetHost:  "",
			input:       `o._baseHref=""`,
			expected:    `o._baseHref=""`,
		},
		{
			name:        "rewrite standalone baseHref assignment",
			proxyPrefix: "/proxy/ups",
			targetPath:  "",
			targetHost:  "",
			input:       `{baseHref=""}`,
			expected:    `{baseHref= "/proxy/ups"}`,
		},
		{
			name:        "rewrite JSON baseHref key",
			proxyPrefix: "/proxy/ups",
			targetPath:  "",
			targetHost:  "",
			input:       `{"baseHref": ""}`,
			expected:    `{"baseHref": "/proxy/ups"}`,
		},
		{
			name:        "preserve JSON array paths (interceptor handles API calls)",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `{"images": ["/img1.jpg", "/img2.jpg"]}`,
			expected:    `{"images": ["/img1.jpg", "/img2.jpg"]}`,
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
		{
			name:        "rewrite dynamic import in HTML",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<script>const p=()=>import("/_nuxt/page.mjs")</script>`,
			expected:    `<script>const p=()=>import("/proxy/app/_nuxt/page.mjs")</script>`,
		},
		{
			name:        "rewrite static import in HTML module script",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<script type="module">import{createApp}from"/_nuxt/vue.mjs"</script>`,
			expected:    `<script type="module">import{createApp}from"/proxy/app/_nuxt/vue.mjs"</script>`,
		},
		{
			name:        "rewrite meta refresh URL",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<meta http-equiv="refresh" content="5;url=/login">`,
			expected:    `<meta http-equiv="refresh" content="5;url=/proxy/app/login">`,
		},
		{
			name:        "rewrite meta refresh URL with quotes",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<meta http-equiv="refresh" content="0; URL='/dashboard'">`,
			expected:    `<meta http-equiv="refresh" content="0; URL='/proxy/app/dashboard'">`,
		},
		{
			name:        "skip already-proxied meta refresh URL",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			targetHost:  "",
			input:       `<meta http-equiv="refresh" content="5;url=/proxy/app/login">`,
			expected:    `<meta http-equiv="refresh" content="5;url=/proxy/app/login">`,
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

// TestCookieRewriting tests Set-Cookie attribute rewriting (path, domain, secure, samesite)
func TestCookieRewriting(t *testing.T) {
	tests := []struct {
		name        string
		proxyPrefix string
		targetPath  string
		cookie      string
		secure      bool
		expected    string
	}{
		// Existing path-rewriting tests
		{
			name:        "rewrite cookie with target path",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			cookie:      "session=abc123; Path=/admin; HttpOnly",
			secure:      true,
			expected:    "session=abc123; Path=/proxy/app/; HttpOnly",
		},
		{
			name:        "rewrite cookie with root path",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "session=abc123; Path=/; HttpOnly",
			secure:      true,
			expected:    "session=abc123; Path=/proxy/app/; HttpOnly",
		},
		{
			name:        "rewrite cookie path with subpath",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			cookie:      "token=xyz; Path=/admin/api; Secure",
			secure:      true,
			expected:    "token=xyz; Path=/proxy/app/api; Secure",
		},
		{
			name:        "don't rewrite already correct path",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "session=abc; Path=/proxy/app; HttpOnly",
			secure:      true,
			expected:    "session=abc; Path=/proxy/app; HttpOnly",
		},
		// Domain stripping
		{
			name:        "strip domain attribute",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "session=abc; Path=/; Domain=192.0.2.10; HttpOnly",
			secure:      true,
			expected:    "session=abc; Path=/proxy/app/; HttpOnly",
		},
		{
			name:        "strip domain with leading dot",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "token=xyz; Domain=.example.com; Path=/",
			secure:      true,
			expected:    "token=xyz; Path=/proxy/app/",
		},
		// Secure flag management
		{
			name:        "keep Secure on HTTPS frontend",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "session=abc; Path=/; Secure; HttpOnly",
			secure:      true,
			expected:    "session=abc; Path=/proxy/app/; Secure; HttpOnly",
		},
		{
			name:        "strip Secure on HTTP frontend",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "session=abc; Path=/; Secure; HttpOnly",
			secure:      false,
			expected:    "session=abc; Path=/proxy/app/; HttpOnly",
		},
		// SameSite rewriting
		{
			name:        "rewrite SameSite=Strict to Lax",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "csrf=token; Path=/; SameSite=Strict; HttpOnly",
			secure:      true,
			expected:    "csrf=token; Path=/proxy/app/; SameSite=Lax; HttpOnly",
		},
		{
			name:        "keep SameSite=Lax unchanged",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "csrf=token; Path=/; SameSite=Lax; HttpOnly",
			secure:      true,
			expected:    "csrf=token; Path=/proxy/app/; SameSite=Lax; HttpOnly",
		},
		{
			name:        "keep SameSite=None unchanged",
			proxyPrefix: "/proxy/app",
			targetPath:  "",
			cookie:      "track=id; Path=/; SameSite=None; Secure",
			secure:      true,
			expected:    "track=id; Path=/proxy/app/; SameSite=None; Secure",
		},
		// Combined attributes
		{
			name:        "strip domain and secure on HTTP, rewrite strict",
			proxyPrefix: "/proxy/app",
			targetPath:  "/admin",
			cookie:      "session=abc; Path=/admin; Domain=backend.local; Secure; SameSite=Strict; HttpOnly",
			secure:      false,
			expected:    "session=abc; Path=/proxy/app/; SameSite=Lax; HttpOnly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewriter := newContentRewriter(tt.proxyPrefix, tt.targetPath, "")
			result := rewriter.rewriteCookie(tt.cookie, tt.secure)
			if result != tt.expected {
				t.Errorf("rewriteCookie() =\n  got:  %q\n  want: %q", result, tt.expected)
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
			handler := NewReverseProxyHandler(apps, "30s")

			// Find the route
			slug := Slugify(tt.appName)
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

func TestDirectorOriginRefererRewriting(t *testing.T) {
	targetURL, _ := url.Parse("https://192.0.2.42:8989")
	director := buildDirector("/proxy/sonarr", "", targetURL, nil, "")

	tests := []struct {
		name            string
		method          string
		origin          string
		referer         string
		expectedOrigin  string
		originStripped  bool
		expectedReferer string
	}{
		{
			name:           "POST strips origin",
			method:         "POST",
			origin:         "https://muximux.example.com",
			originStripped: true,
		},
		{
			name:           "PUT strips origin",
			method:         "PUT",
			origin:         "https://muximux.example.com",
			originStripped: true,
		},
		{
			name:           "DELETE strips origin",
			method:         "DELETE",
			origin:         "https://muximux.example.com",
			originStripped: true,
		},
		{
			name:           "PATCH strips origin",
			method:         "PATCH",
			origin:         "https://muximux.example.com",
			originStripped: true,
		},
		{
			name:           "GET strips origin",
			method:         "GET",
			origin:         "https://muximux.example.com",
			originStripped: true,
		},
		{
			name:           "HEAD strips origin",
			method:         "HEAD",
			origin:         "https://muximux.example.com",
			originStripped: true,
		},
		{
			name:           "OPTIONS strips origin",
			method:         "OPTIONS",
			origin:         "https://muximux.example.com",
			originStripped: true,
		},
		{
			name:            "rewrites referer host and strips proxy prefix",
			method:          "POST",
			referer:         "https://muximux.example.com/proxy/sonarr/series/123",
			expectedReferer: "https://192.0.2.42:8989/series/123",
		},
		{
			name:            "rewrites referer preserving query string",
			method:          "POST",
			referer:         "https://muximux.example.com/proxy/sonarr/api?key=val",
			expectedReferer: "https://192.0.2.42:8989/api?key=val",
		},
		{
			name:           "no origin header - no rewrite",
			method:         "POST",
			origin:         "",
			expectedOrigin: "",
		},
		{
			name:            "no referer header - no rewrite",
			method:          "POST",
			referer:         "",
			expectedReferer: "",
		},
		{
			name:            "referer without proxy prefix - rewrites host only",
			method:          "POST",
			referer:         "https://muximux.example.com/other/path",
			expectedReferer: "https://192.0.2.42:8989/other/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = "POST"
			}
			req := httptest.NewRequest(method, "/proxy/sonarr/api/action", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}

			director(req)

			switch {
			case tt.originStripped:
				if got := req.Header.Get("Origin"); got != "" {
					t.Errorf("Origin should be stripped for %s, got %q", method, got)
				}
			case tt.expectedOrigin != "":
				if got := req.Header.Get("Origin"); got != tt.expectedOrigin {
					t.Errorf("Origin = %q, want %q", got, tt.expectedOrigin)
				}
			case tt.origin == "":
				if got := req.Header.Get("Origin"); got != "" {
					t.Errorf("Origin should not be set, got %q", got)
				}
			}

			if tt.expectedReferer != "" {
				if got := req.Header.Get("Referer"); got != tt.expectedReferer {
					t.Errorf("Referer = %q, want %q", got, tt.expectedReferer)
				}
			} else if tt.referer == "" {
				if got := req.Header.Get("Referer"); got != "" {
					t.Errorf("Referer should not be set, got %q", got)
				}
			}
		})
	}

	// Test with targetPath to verify Referer includes backend base path
	t.Run("referer includes target path for subpath apps", func(t *testing.T) {
		subURL, _ := url.Parse("http://192.0.2.100/admin")
		subDirector := buildDirector("/proxy/pihole", "/admin", subURL, nil, "")
		req := httptest.NewRequest("POST", "/proxy/pihole/settings", nil)
		req.Header.Set("Referer", "https://muximux.example.com/proxy/pihole/settings")
		subDirector(req)
		want := "http://192.0.2.100/admin/settings"
		if got := req.Header.Get("Referer"); got != want {
			t.Errorf("Referer = %q, want %q", got, want)
		}
	})
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

	handler := NewReverseProxyHandler(apps, "30s")

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

	handler := NewReverseProxyHandler(apps, "30s")

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
			expectedStatus: http.StatusBadRequest,
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

	handler := NewReverseProxyHandler(apps, "30s")

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
		{
			name:        "prefix as substring of path segment is not stripped",
			proxyPrefix: "/proxy/app",
			targetPath:  "/",
			requestPath: "/proxy/app/api/proxy/application/status",
			expected:    "/api/proxy/application/status",
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
	h := sha1.New() //nolint:gosec // SHA-1 required by RFC 6455 WebSocket handshake
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
	handler := NewReverseProxyHandler(apps, "30s")

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
	defer resp.Body.Close()

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

	if !bytes.Equal(echoBuf[:n], testMsg) {
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
	handler := NewReverseProxyHandler(apps, "30s")

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
	defer resp.Body.Close()

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
				"token=xyz; Path=/proxy/app/",
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

// TestRewriteCookieHeaders_SecureFlag validates that the cookie Secure flag
// is only kept when the frontend connection was demonstrably HTTPS, either
// via TLS on the incoming request or via a ClientScheme stamped in the
// request context by ResolveClientIP. X-Forwarded-Proto from the wire must
// not be consulted here because it is client-controllable on a direct HTTP
// connection (findings.md M15).
func TestRewriteCookieHeaders_SecureFlag(t *testing.T) {
	tests := []struct {
		name   string
		build  func() *http.Request
		expect bool // true if Secure should be kept
	}{
		{
			name: "direct TLS connection keeps Secure",
			build: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
				req.TLS = &tls.ConnectionState{}
				return req
			},
			expect: true,
		},
		{
			name: "context scheme https keeps Secure",
			build: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
				ctx := context.WithValue(req.Context(), auth.ContextKeyClientScheme, "https")
				return req.WithContext(ctx)
			},
			expect: true,
		},
		{
			name: "context scheme http strips Secure",
			build: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
				ctx := context.WithValue(req.Context(), auth.ContextKeyClientScheme, "http")
				return req.WithContext(ctx)
			},
			expect: false,
		},
		{
			name: "spoofed X-Forwarded-Proto header alone is ignored",
			build: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
				req.Header.Set("X-Forwarded-Proto", "https")
				return req
			},
			expect: false,
		},
		{
			name: "no TLS, no context, no header strips Secure",
			build: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header:  make(http.Header),
				Request: tt.build(),
			}
			resp.Header.Add("Set-Cookie", "sid=abc; Path=/; Secure; HttpOnly")

			rewriter := newContentRewriter("/proxy/app", "", "")
			rewriteCookieHeaders(resp, rewriter)

			got := resp.Header.Get("Set-Cookie")
			hasSecure := strings.Contains(got, "Secure")
			if hasSecure != tt.expect {
				t.Errorf("Secure flag: got=%v, want=%v (cookie=%q)", hasSecure, tt.expect, got)
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

	t.Run("JSON body uses safe-only rewriting", func(t *testing.T) {
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
		// JSON uses safe-only rewriting (no root-relative path rewriting)
		// Root-relative paths are handled by the runtime interceptor instead
		if strings.Contains(string(rewritten), "/proxy/app/api/v3") {
			t.Errorf("JSON should not statically rewrite root-relative paths, got: %s", string(rewritten))
		}
		if !strings.Contains(string(rewritten), "/api/v3") {
			t.Errorf("JSON root-relative path should be preserved, got: %s", string(rewritten))
		}
	})

	t.Run("HTML preserves SSR payload paths for SPA hydration", func(t *testing.T) {
		rewriter := newContentRewriter("/proxy/mealie", "", "")

		// Simulate Nuxt 3-style SSR HTML with inline JSON payload
		body := `<html><head></head><body>` +
			`<script type="application/json" data-nuxt-data="default">` +
			`{"fullPath":"/recipe/123","path":"/recipe/123","redirectUrl":"/login"}` +
			`</script></body></html>`
		resp := &http.Response{
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")

		err := rewriteResponseBody(resp, rewriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rewritten, _ := io.ReadAll(resp.Body)
		result := string(rewritten)
		// The SSR payload paths must NOT be rewritten — the runtime interceptor
		// handles API calls, and rewriting route paths in the payload would
		// corrupt client-side hydration (SPA router sees /proxy/mealie/recipe/123
		// instead of /recipe/123 → route mismatch → 404).
		if strings.Contains(result, `"/proxy/mealie/recipe/123"`) {
			t.Errorf("SSR payload route path should NOT be rewritten, got: %s", result)
		}
		if strings.Contains(result, `"/proxy/mealie/login"`) {
			t.Errorf("SSR payload redirect path should NOT be rewritten, got: %s", result)
		}
		// But the interceptor script SHOULD be injected
		if !strings.Contains(result, "data-muximux-proxy") {
			t.Error("interceptor script should be injected in HTML response")
		}
	})

	t.Run("HTML rewrites module imports in inline scripts", func(t *testing.T) {
		rewriter := newContentRewriter("/proxy/app", "", "")

		body := `<html><head></head><body>` +
			`<script type="module">import { createApp } from '/_nuxt/entry.mjs'</script>` +
			`</body></html>`
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
		result := string(rewritten)
		if !strings.Contains(result, `from '/proxy/app/_nuxt/entry.mjs'`) {
			t.Errorf("inline module import should be rewritten, got: %s", result)
		}
	})

	t.Run("JS body rewrites dynamic imports", func(t *testing.T) {
		rewriter := newContentRewriter("/proxy/app", "", "")

		body := `const page=()=>import("/_nuxt/pages/recipe.mjs");`
		resp := &http.Response{
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}
		resp.Header.Set("Content-Type", "application/javascript")

		err := rewriteResponseBody(resp, rewriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rewritten, _ := io.ReadAll(resp.Body)
		result := string(rewritten)
		if !strings.Contains(result, `import("/proxy/app/_nuxt/pages/recipe.mjs")`) {
			t.Errorf("dynamic import should be rewritten in JS, got: %s", result)
		}
	})

	t.Run("skips rewriting when Content-Length exceeds limit", func(t *testing.T) {
		rewriter := newContentRewriter("/proxy/app", "", "")

		body := `<html><head><link href="/style.css"></head></html>`
		resp := &http.Response{
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: maxRewriteSize + 1,
		}
		resp.Header.Set("Content-Type", "text/html")

		err := rewriteResponseBody(resp, rewriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Body should be untouched — the response was too large to rewrite
		result, _ := io.ReadAll(resp.Body)
		if string(result) != body {
			t.Errorf("expected body to be unchanged for oversized response")
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

		route := buildSingleProxyRoute(&app, 30*time.Second, "")

		if route == nil {
			t.Fatal("expected non-nil route")
			return
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

		route := buildSingleProxyRoute(&app, 30*time.Second, "")

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

		route := buildSingleProxyRoute(&app, 30*time.Second, "")

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

		route := buildSingleProxyRoute(&app, 30*time.Second, "")

		if route == nil {
			t.Fatal("expected non-nil route")
			return
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
		handler := NewReverseProxyHandler(nil, "30s")
		if handler.HasRoutes() {
			t.Error("expected HasRoutes() = false for empty handler")
		}
	})

	t.Run("with routes", func(t *testing.T) {
		apps := []config.AppConfig{
			{Name: "App", URL: "http://host:8080", Enabled: true, Proxy: true},
		}
		handler := NewReverseProxyHandler(apps, "30s")
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
	handler := NewReverseProxyHandler(apps, "30s")

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
// TestRewriteCSSImports tests @import path rewriting.
func TestRewriteCSSImports(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double-quoted @import",
			input:    `@import "/styles/main.css"`,
			expected: `@import "/proxy/app/styles/main.css"`,
		},
		{
			name:     "single-quoted @import",
			input:    `@import '/fonts/custom.css'`,
			expected: `@import '/proxy/app/fonts/custom.css'`,
		},
		{
			name:     "@import url unquoted",
			input:    `@import url(/vendor/normalize.css)`,
			expected: `@import url(/proxy/app/vendor/normalize.css)`,
		},
		{
			name:     "@import url quoted preserved",
			input:    `@import url("/themes/dark.css")`,
			expected: `@import url("/themes/dark.css")`,
		},
		{
			name:     "already rewritten @import",
			input:    `@import "/proxy/app/styles.css"`,
			expected: `@import "/proxy/app/styles.css"`,
		},
		{
			name:     "already rewritten url @import",
			input:    `@import url(/proxy/app/styles.css)`,
			expected: `@import url(/proxy/app/styles.css)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriter.rewriteCSSImports([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewriteCSSImports() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestRewriteSVGHrefs tests SVG use/image href rewriting.
func TestRewriteSVGHrefs(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "use href",
			input:    `<use href="/icons/sprite.svg#icon-home"></use>`,
			expected: `<use href="/proxy/app/icons/sprite.svg#icon-home"></use>`,
		},
		{
			name:     "image href",
			input:    `<image href="/images/logo.svg" width="100"/>`,
			expected: `<image href="/proxy/app/images/logo.svg" width="100"/>`,
		},
		{
			name:     "use xlink:href",
			input:    `<use xlink:href="/sprites.svg#play"></use>`,
			expected: `<use xlink:href="/proxy/app/sprites.svg#play"></use>`,
		},
		{
			name:     "single-quoted href",
			input:    `<use href='/icons/set.svg#arrow'></use>`,
			expected: `<use href='/proxy/app/icons/set.svg#arrow'></use>`,
		},
		{
			name:     "already rewritten",
			input:    `<use href="/proxy/app/icons/sprite.svg#icon"></use>`,
			expected: `<use href="/proxy/app/icons/sprite.svg#icon"></use>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriter.rewriteSVGHrefs([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewriteSVGHrefs() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestRewriteBaseHref tests <base href> rewriting.
func TestRewriteBaseHref(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		target   string
		input    string
		expected string
	}{
		{
			name:     "root base href",
			prefix:   "/proxy/app",
			target:   "",
			input:    `<base href="/">`,
			expected: `<base href="/proxy/app/">`,
		},
		{
			name:     "target path base href",
			prefix:   "/proxy/app",
			target:   "/admin",
			input:    `<base href="/admin/">`,
			expected: `<base href="/proxy/app/">`,
		},
		{
			name:     "already rewritten",
			prefix:   "/proxy/app",
			target:   "",
			input:    `<base href="/proxy/app/">`,
			expected: `<base href="/proxy/app/">`,
		},
		{
			name:     "single-quoted base href",
			prefix:   "/proxy/app",
			target:   "",
			input:    `<base href='/'>`,
			expected: `<base href='/proxy/app/'>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewriter := newContentRewriter(tt.prefix, tt.target, "")
			result := string(rewriter.rewriteBaseHref([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewriteBaseHref() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestRewriteRootPathURLFunc tests CSS url() root path rewriting.
func TestRewriteRootPathURLFunc(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "url with root path",
			input:    `url("/fonts/roboto.woff2")`,
			expected: `url("/proxy/app/fonts/roboto.woff2")`,
		},
		{
			name:     "url already rewritten",
			input:    `url("/proxy/app/fonts/roboto.woff2")`,
			expected: `url("/proxy/app/fonts/roboto.woff2")`,
		},
		{
			name:     "url single-quoted",
			input:    `url('/images/bg.png')`,
			expected: `url('/proxy/app/images/bg.png')`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriter.rewriteRootPathURLFunc([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewriteRootPathURLFunc() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestRewriteImageSet tests CSS image-set() rewriting.
func TestRewriteImageSet(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "image-set with paths",
			input:    `image-set("/img/1x.png" 1x, "/img/2x.png" 2x)`,
			expected: `image-set("/proxy/app/img/1x.png" 1x, "/proxy/app/img/2x.png" 2x)`,
		},
		{
			name:     "already rewritten",
			input:    `image-set("/proxy/app/img/1x.png" 1x)`,
			expected: `image-set("/proxy/app/img/1x.png" 1x)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriter.rewriteImageSet([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewriteImageSet() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestRewriteModuleImports tests ES module import/export rewriting.
func TestRewriteModuleImports(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "dynamic import",
			input:    `import('/chunk.js')`,
			expected: `import('/proxy/app/chunk.js')`,
		},
		{
			name:     "dynamic import double quotes",
			input:    `import("/_nuxt/pages/recipe.mjs")`,
			expected: `import("/proxy/app/_nuxt/pages/recipe.mjs")`,
		},
		{
			name:     "static import named",
			input:    `import { createApp } from '/_nuxt/vue.mjs'`,
			expected: `import { createApp } from '/proxy/app/_nuxt/vue.mjs'`,
		},
		{
			name:     "static import default",
			input:    `import App from '/components/App.js'`,
			expected: `import App from '/proxy/app/components/App.js'`,
		},
		{
			name:     "static import namespace",
			input:    `import * as utils from "/lib/utils.mjs"`,
			expected: `import * as utils from "/proxy/app/lib/utils.mjs"`,
		},
		{
			name:     "export from",
			input:    `export { default } from '/modules/foo.js'`,
			expected: `export { default } from '/proxy/app/modules/foo.js'`,
		},
		{
			name:     "export all",
			input:    `export * from '/modules/bar.js'`,
			expected: `export * from '/proxy/app/modules/bar.js'`,
		},
		{
			name:     "side-effect import",
			input:    `import '/polyfill.js'`,
			expected: `import '/proxy/app/polyfill.js'`,
		},
		{
			name:     "already proxied dynamic import",
			input:    `import('/proxy/app/chunk.js')`,
			expected: `import('/proxy/app/chunk.js')`,
		},
		{
			name:     "relative import unchanged",
			input:    `import './relative.js'`,
			expected: `import './relative.js'`,
		},
		{
			name:     "bare specifier unchanged",
			input:    `import 'lodash'`,
			expected: `import 'lodash'`,
		},
		{
			name:     "multiple dynamic imports in minified code",
			input:    `import("/a.js");import("/b.js")`,
			expected: `import("/proxy/app/a.js");import("/proxy/app/b.js")`,
		},
		{
			name:     "minified static imports",
			input:    `import{foo}from"/path1.js";import{bar}from"/path2.js"`,
			expected: `import{foo}from"/proxy/app/path1.js";import{bar}from"/proxy/app/path2.js"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriter.rewriteModuleImports([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewriteModuleImports() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

// TestRewriteResponseBodyGzip tests response body rewriting with gzip encoding.
func TestRewriteResponseBodyGzip(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	// Create gzipped content
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	original := `<html><link href="/style.css"></html>`
	if _, err := gzw.Write([]byte(original)); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}

	resp := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(buf.Bytes())),
		ContentLength: int64(buf.Len()),
	}
	resp.Header.Set("Content-Type", "text/html")
	resp.Header.Set("Content-Encoding", "gzip")

	if err := rewriteResponseBody(resp, rewriter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rewritten, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(rewritten), "/proxy/app/style.css") {
		t.Errorf("expected rewritten gzipped content, got: %s", string(rewritten))
	}
	// Content-Encoding should be removed after decompression
	if resp.Header.Get("Content-Encoding") != "" {
		t.Error("expected Content-Encoding to be removed")
	}
}

// TestRewriteRootPathAttrs tests attribute root path rewriting.
func TestRewriteRootPathAttrs(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "href with root path",
			input:    `href="/dashboard/index.html"`,
			expected: `href="/proxy/app/dashboard/index.html"`,
		},
		{
			name:     "src with root path",
			input:    `src="/js/app.js"`,
			expected: `src="/proxy/app/js/app.js"`,
		},
		{
			name:     "already has proxy prefix",
			input:    `href="/proxy/app/index.html"`,
			expected: `href="/proxy/app/index.html"`,
		},
		{
			name:     "srcset skipped",
			input:    `srcset="/img/sm.jpg 1x"`,
			expected: `srcset="/img/sm.jpg 1x"`,
		},
		{
			name:     "single-quoted attr",
			input:    `href='/styles/main.css'`,
			expected: `href='/proxy/app/styles/main.css'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriter.rewriteRootPathAttrs([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("rewriteRootPathAttrs() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

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
			result := string(rewriter.stripIntegrity([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("stripIntegrity() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

func TestStripMetaCSP(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strip meta CSP tag",
			input:    `<head><meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'nonce-abc123'"><script src="/app.js"></script></head>`,
			expected: `<head><script src="/app.js"></script></head>`,
		},
		{
			name:     "strip meta CSP case-insensitive",
			input:    `<meta HTTP-EQUIV="content-security-policy" content="script-src 'self'">`,
			expected: ``,
		},
		{
			name:     "strip report-only variant",
			input:    `<meta http-equiv="Content-Security-Policy-Report-Only" content="default-src 'self'">`,
			expected: ``,
		},
		{
			name:     "self-closing meta",
			input:    `<meta http-equiv="Content-Security-Policy" content="default-src 'self'" />`,
			expected: ``,
		},
		{
			name:     "leave other meta tags untouched",
			input:    `<meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'self'"><meta name="viewport" content="width=device-width">`,
			expected: `<meta charset="utf-8"><meta name="viewport" content="width=device-width">`,
		},
		{
			name:     "no CSP meta present",
			input:    `<meta charset="utf-8"><title>Test</title>`,
			expected: `<meta charset="utf-8"><title>Test</title>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriter.stripMetaCSP([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("stripMetaCSP() =\n  got:  %q\n  want: %q", result, tt.expected)
			}
		})
	}
}

func TestStripEmbeddingHeaders(t *testing.T) {
	// Verify that modifyResponse strips headers that prevent iframe embedding.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "fullscreen=(), clipboard-write=()")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head></head><body>ok</body></html>`))
	}))
	defer backend.Close()

	apps := []config.AppConfig{
		{Name: "TestApp", URL: backend.URL, Enabled: true, Proxy: true},
	}
	handler := NewReverseProxyHandler(apps, "30s")

	req := httptest.NewRequest("GET", "/proxy/testapp/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	for _, hdr := range []string{"X-Frame-Options", "Content-Security-Policy", "Permissions-Policy"} {
		if v := rec.Header().Get(hdr); v != "" {
			t.Errorf("expected %s header to be stripped, got %q", hdr, v)
		}
	}
}

func TestStripMetaCSPInRewriteResponseBody(t *testing.T) {
	// Verify that meta CSP tags are stripped during full HTML body rewriting.
	rewriter := newContentRewriter("/proxy/app", "", "")

	body := `<html><head><meta http-equiv="Content-Security-Policy" content="script-src 'nonce-abc123'"><script src="/app.js"></script></head></html>`
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
	result := string(rewritten)
	if strings.Contains(result, "Content-Security-Policy") {
		t.Errorf("expected meta CSP tag to be stripped, got: %s", result)
	}
	if !strings.Contains(result, "/proxy/app/app.js") {
		t.Errorf("expected script src to be rewritten, got: %s", result)
	}
}

func TestInterceptorScriptIframeIsolation(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")
	script := string(rewriter.interceptorScript())

	// The interceptor must override window.parent and window.top so proxied
	// apps cannot detect they are embedded in an iframe.
	if !strings.Contains(script, `Object.defineProperty(window,"parent"`) {
		t.Error("interceptor script should override window.parent")
	}
	if !strings.Contains(script, `Object.defineProperty(window,"top"`) {
		t.Error("interceptor script should override window.top")
	}

	// The proxy prefix variable must be declared before the window isolation
	// check, since the check compares window.parent.location.pathname against P.
	prefixIdx := strings.Index(script, `var P="`)
	isolationIdx := strings.Index(script, `window.parent.location.pathname.indexOf(P)`)
	if prefixIdx == -1 || isolationIdx == -1 || prefixIdx > isolationIdx {
		t.Error("proxy prefix declaration must appear before the window isolation check")
	}

	// When the parent is within the same proxy app (internal sub-iframe),
	// window.parent must NOT be overridden so internal communication works
	// (e.g. qBittorrent's download dialog reading preferences from parent cache).
	if !strings.Contains(script, `window.parent.location.pathname.indexOf(P)===0`) {
		t.Error("interceptor should check if parent is within same proxy app before overriding")
	}
}

func TestInterceptorScriptURLCoverage(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")
	script := string(rewriter.interceptorScript())

	// R() should exclude protocol-relative URLs (//host/path) from rewriting.
	// The u[1]!=="/" check prevents //cdn.example.com from being prefixed.
	if !strings.Contains(script, `u[1]!=="/"`) {
		t.Error("R() should exclude protocol-relative URLs (//host/path)")
	}

	// R() should handle relative URLs (css/style.css, api/data) by prepending
	// the proxy prefix, so they resolve correctly even after replaceState
	// strips the prefix from the document URL.
	if !strings.Contains(script, `u.indexOf(":")===-1)return P+"/"+u`) {
		t.Error("R() should rewrite relative URLs by prepending proxy prefix")
	}

	// Property setters should cover iframe.src and link.href for dynamically
	// created elements (e.g. SPAs injecting stylesheets or sub-iframes).
	if !strings.Contains(script, `W(HTMLIFrameElement,"src")`) {
		t.Error("interceptor should override HTMLIFrameElement.src setter")
	}
	if !strings.Contains(script, `W(HTMLLinkElement,"href")`) {
		t.Error("interceptor should override HTMLLinkElement.href setter")
	}

	// MutationObserver should watch href and srcset attributes in addition
	// to src and poster, so innerHTML-injected links and responsive images
	// get rewritten.
	if !strings.Contains(script, `"href":1`) {
		t.Error("MutationObserver urlAttrs should include href")
	}
	if !strings.Contains(script, `attributeFilter:["src","poster","href","srcset","action","data","formaction"]`) {
		t.Error("MutationObserver should filter on src, poster, href, srcset, action, data, and formaction")
	}
	if !strings.Contains(script, `fixSrcset`) {
		t.Error("interceptor should include fixSrcset for dynamic srcset rewriting")
	}

	// Anchor href and form action setters should be intercepted so SPAs
	// setting a.href = "/page" or form.action = "/submit" get rewritten.
	if !strings.Contains(script, `W(HTMLAnchorElement,"href")`) {
		t.Error("interceptor should override HTMLAnchorElement.href setter")
	}
	if !strings.Contains(script, `W(HTMLFormElement,"action")`) {
		t.Error("interceptor should override HTMLFormElement.action setter")
	}
	if !strings.Contains(script, `"action":1`) {
		t.Error("MutationObserver urlAttrs should include action")
	}

	// Object data and formaction setters should be intercepted for <object>
	// embeds and submit button overrides.
	if !strings.Contains(script, `W(HTMLObjectElement,"data")`) {
		t.Error("interceptor should override HTMLObjectElement.data setter")
	}
	if !strings.Contains(script, `W(HTMLButtonElement,"formAction")`) {
		t.Error("interceptor should override HTMLButtonElement.formAction setter")
	}
	if !strings.Contains(script, `W(HTMLInputElement,"formAction")`) {
		t.Error("interceptor should override HTMLInputElement.formAction setter")
	}
	if !strings.Contains(script, `"data":1`) {
		t.Error("MutationObserver urlAttrs should include data")
	}
	if !strings.Contains(script, `"formaction":1`) {
		t.Error("MutationObserver urlAttrs should include formaction")
	}

	// Worker/SharedWorker constructors should be patched so worker scripts
	// load through the proxy instead of from the Muximux origin.
	if !strings.Contains(script, `window.Worker=function`) {
		t.Error("interceptor should patch Worker constructor")
	}
	if !strings.Contains(script, `window.SharedWorker=function`) {
		t.Error("interceptor should patch SharedWorker constructor")
	}

	// Location.prototype.pathname getter should be overridden to strip
	// the proxy prefix, so SPA code always sees clean paths.
	if !strings.Contains(script, `Location.prototype,"pathname"`) {
		t.Error("interceptor should attempt to patch Location.prototype.pathname getter")
	}

	// Location.prototype.href getter+setter should be overridden to strip
	// the proxy prefix from reads and add it on writes.
	if !strings.Contains(script, `Location.prototype,"href"`) {
		t.Error("interceptor should attempt to patch Location.prototype.href getter+setter")
	}

	// Location.prototype.toString should return the patched href.
	if !strings.Contains(script, `Location.prototype.toString=function`) {
		t.Error("interceptor should override Location.prototype.toString")
	}

	// document.URL and document.documentURI should be overridden to match
	// the patched href, so frameworks reading these see clean URLs.
	if !strings.Contains(script, `Document.prototype,"URL"`) {
		t.Error("interceptor should attempt to patch Document.prototype.URL getter")
	}
	if !strings.Contains(script, `Document.prototype,"documentURI"`) {
		t.Error("interceptor should attempt to patch Document.prototype.documentURI getter")
	}

	// Element.prototype.setAttribute should be wrapped so that libraries using
	// el.setAttribute("src", url) (e.g. MooTools) get synchronous URL rewriting.
	// Without this, only the MutationObserver catches setAttribute calls, but it
	// fires asynchronously — too late for <script> elements.
	if !strings.Contains(script, `Element.prototype.setAttribute=function`) {
		t.Error("interceptor should patch Element.prototype.setAttribute for synchronous URL rewriting")
	}
	if !strings.Contains(script, `_sA.call(this,n,v)`) {
		t.Error("patched setAttribute should delegate to original _sA")
	}

	// HTMLImageElement.srcset property setter should parse comma-separated
	// "url descriptor" pairs and rewrite each URL individually via R().
	if !strings.Contains(script, `HTMLImageElement.prototype,"srcset"`) {
		t.Error("interceptor should override HTMLImageElement.srcset setter")
	}

	// HTMLBaseElement.href setter should be intercepted so <base href="/">
	// set via JS goes through the proxy prefix.
	if !strings.Contains(script, `W(HTMLBaseElement,"href")`) {
		t.Error("interceptor should override HTMLBaseElement.href setter")
	}

	// Audio constructor should be wrapped so new Audio('/sound.mp3')
	// rewrites the URL via R(). The src setter (via W()) only catches
	// subsequent audio.src = url assignments, not the constructor arg.
	if !strings.Contains(script, `window.Audio=function`) {
		t.Error("interceptor should patch Audio constructor")
	}

	// CSSStyleSheet.insertRule should rewrite url() references in CSS rules
	// so dynamically injected styles load resources through the proxy.
	if !strings.Contains(script, `CSSStyleSheet.prototype.insertRule`) {
		t.Error("interceptor should patch CSSStyleSheet.prototype.insertRule")
	}
	if !strings.Contains(script, `_iR.call(`) && !strings.Contains(script, `_iR.apply(`) {
		t.Error("patched insertRule should delegate to original _iR")
	}

	// insertAdjacentHTML should synchronously fix URLs in injected HTML,
	// because MutationObserver fires too late for <script> elements.
	if !strings.Contains(script, `Element.prototype.insertAdjacentHTML=function`) {
		t.Error("interceptor should patch Element.prototype.insertAdjacentHTML")
	}
	if !strings.Contains(script, `_iAH.call(`) && !strings.Contains(script, `_iAH.apply(`) {
		t.Error("patched insertAdjacentHTML should delegate to original _iAH")
	}
}

func TestInterceptorScriptHistoryAPI(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")
	script := string(rewriter.interceptorScript())

	// The interceptor must strip the proxy prefix from location.pathname on
	// initial load (fallback when getter patches fail), and use the _pG flag
	// to conditionally skip the strip when getter patches succeeded.
	if !strings.Contains(script, `var _pG=false`) {
		t.Error("interceptor should declare _pG flag for getter patch detection")
	}
	if !strings.Contains(script, `if(!_pG){var _il=location.pathname`) {
		t.Error("interceptor should conditionally strip proxy prefix from initial URL")
	}

	// history.pushState/replaceState must be patched to add the proxy prefix,
	// so "Reload frame" requests hit the correct /proxy/slug/... path.
	if !strings.Contains(script, `history.pushState=function`) {
		t.Error("interceptor should patch history.pushState")
	}
	if !strings.Contains(script, `history.replaceState=function`) {
		t.Error("interceptor should patch history.replaceState")
	}

	// When Location.prototype.pathname is non-configurable (Chrome), the proxy
	// prefix added by R() in pushState/replaceState makes location.pathname
	// return the prefixed path. Framework routers reading it during init would
	// fail. The init guard (_sR flag + _S strip helper) keeps the URL clean
	// during the synchronous initialization phase.
	if !strings.Contains(script, `var _sR=!_pG`) {
		t.Error("interceptor should declare _sR init-strip flag based on _pG")
	}
	if !strings.Contains(script, `function _S()`) {
		t.Error("interceptor should define _S strip helper for init guard")
	}
	if !strings.Contains(script, `if(_sR)_S()`) {
		t.Error("interceptor pushState/replaceState should call _S() during init phase")
	}

	// A popstate listener (capture phase) must strip the prefix before the
	// SPA's own popstate handler reads location.pathname on back/forward.
	// Wrapped in if(!_pG) since getter patches make it unnecessary.
	if !strings.Contains(script, `if(!_pG){window.addEventListener("popstate"`) {
		t.Error("interceptor should conditionally add popstate listener to strip prefix on back/forward")
	}

	// After init completes, the proxy prefix must be restored in the URL so
	// browser back/forward navigates to /proxy/slug/... instead of "/",
	// which would load the Muximux SPA shell inside the iframe.
	if !strings.Contains(script, `function _rP()`) {
		t.Error("interceptor should define _rP restore-prefix function")
	}
	if !strings.Contains(script, `function _do(){var p=location.pathname`) {
		t.Error("interceptor restore should check current pathname before restoring prefix")
	}

	// location.assign and location.replace should be patched so programmatic
	// navigation goes through the proxy.
	if !strings.Contains(script, `Location.prototype.assign=function`) {
		t.Error("interceptor should patch Location.prototype.assign")
	}
	if !strings.Contains(script, `Location.prototype.replace=function`) {
		t.Error("interceptor should patch Location.prototype.replace")
	}

	// When Location.prototype.href is non-configurable (Chrome), the
	// Navigation API should intercept location.href assignments and redirect
	// them through the proxy. Guarded by !_pG (skipped when getter patches work).
	if !strings.Contains(script, `if(!_pG&&window.navigation)`) {
		t.Error("interceptor should use Navigation API as fallback for non-configurable href setter")
	}
	if !strings.Contains(script, `e.preventDefault()`) {
		t.Error("interceptor Navigation API handler should cancel unprefixed navigations")
	}

	// window.open should be patched so popups navigate through the proxy.
	if !strings.Contains(script, `window.open=function`) {
		t.Error("interceptor should patch window.open")
	}

	// navigator.sendBeacon should be patched for analytics/logging requests.
	if !strings.Contains(script, `navigator.sendBeacon=function`) {
		t.Error("interceptor should patch navigator.sendBeacon")
	}
}

func TestInterceptorScriptStorageIsolation(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")
	script := string(rewriter.interceptorScript())

	// The interceptor should save references to the real storage objects
	// before overriding them.
	if !strings.Contains(script, `var _ls=window.localStorage,_ss=window.sessionStorage`) {
		t.Error("interceptor should save references to real localStorage and sessionStorage")
	}

	// The NS (namespace) function should create a storage wrapper that
	// prefixes keys with the proxy path to isolate each app's storage.
	if !strings.Contains(script, `function NS(s)`) {
		t.Error("interceptor should define NS function for storage namespacing")
	}

	// Keys should be prefixed with the proxy path and "::" separator.
	if !strings.Contains(script, `var px=P+"::"`) {
		t.Error("interceptor storage should use proxy path + '::' as key prefix")
	}

	// The wrapper must implement the full Storage API: getItem, setItem,
	// removeItem, clear, key, and length.
	for _, method := range []string{"getItem", "setItem", "removeItem", "clear", "key"} {
		if !strings.Contains(script, method+":function") {
			t.Errorf("interceptor storage wrapper should implement %s", method)
		}
	}

	// ES6 Proxy should be used (when available) so property access syntax
	// like localStorage["key"] = val also goes through the namespace.
	if !strings.Contains(script, `new Proxy(m,{`) {
		t.Error("interceptor should use ES6 Proxy for storage property access")
	}

	// The Proxy should handle ownKeys so Object.keys(localStorage) only
	// returns this app's keys (un-prefixed).
	if !strings.Contains(script, `ownKeys:function`) {
		t.Error("interceptor storage Proxy should implement ownKeys trap")
	}

	// window.localStorage and window.sessionStorage should be overridden
	// with the namespaced wrappers.
	if !strings.Contains(script, `"localStorage",{value:NS(_ls)`) {
		t.Error("interceptor should override window.localStorage with namespaced wrapper")
	}
	if !strings.Contains(script, `"sessionStorage",{value:NS(_ss)`) {
		t.Error("interceptor should override window.sessionStorage with namespaced wrapper")
	}

	// Verify the prefix uses the actual proxy path.
	rewriter2 := newContentRewriter("/proxy/mealie", "/mealie", "mealie.local")
	script2 := string(rewriter2.interceptorScript())
	if !strings.Contains(script2, `"/proxy/mealie"`) {
		t.Error("interceptor storage prefix should use the actual proxy path")
	}
}

func TestInjectInterceptorSkipsScriptEmbeddedHead(t *testing.T) {
	rewriter := newContentRewriter("/proxy/qbittorrent", "", "")

	t.Run("normal document-level head", func(t *testing.T) {
		content := []byte(`<html><head><title>Test</title></head></html>`)
		result := rewriter.injectInterceptor(content)
		if !strings.Contains(string(result), "data-muximux-proxy") {
			t.Error("interceptor should be injected into document-level <head>")
		}
	})

	t.Run("head inside script tag is skipped", func(t *testing.T) {
		// This simulates qBittorrent's rss.html where <head> only appears
		// inside a JavaScript template literal within a <script> block.
		content := []byte(`<div><script>"use strict";el.srcdoc=` + "`" +
			`<html><head><link href="style.css"></head><body></body></html>` + "`" +
			`;</script></div>`)
		result := rewriter.injectInterceptor(content)
		if strings.Contains(string(result), "data-muximux-proxy") {
			t.Error("interceptor must NOT be injected into <head> that appears inside a <script> block")
		}
	})

	t.Run("head after script block is fine", func(t *testing.T) {
		// A document where script appears before head — head is outside
		content := []byte(`<html><script>var x = 1;</script><head><title>Test</title></head></html>`)
		result := rewriter.injectInterceptor(content)
		if !strings.Contains(string(result), "data-muximux-proxy") {
			t.Error("interceptor should be injected into <head> that appears after a closed <script> block")
		}
	})

	t.Run("no head at all", func(t *testing.T) {
		content := []byte(`<div><p>No head tag here</p></div>`)
		result := rewriter.injectInterceptor(content)
		if strings.Contains(string(result), "data-muximux-proxy") {
			t.Error("interceptor should not be injected when no <head> exists")
		}
	})

	t.Run("header tag is not confused with head", func(t *testing.T) {
		content := []byte(`<html><header>Not a head tag</header></html>`)
		result := rewriter.injectInterceptor(content)
		if strings.Contains(string(result), "data-muximux-proxy") {
			t.Error("interceptor should not be injected into <header> tag")
		}
	})
}

func TestInjectInterceptorBaseTag(t *testing.T) {
	rewriter := newContentRewriter("/proxy/qbittorrent", "", "")

	t.Run("injects base tag when none exists", func(t *testing.T) {
		content := []byte(`<html><head><title>qBittorrent</title></head><body></body></html>`)
		result := string(rewriter.injectInterceptor(content))
		if !strings.Contains(result, `<base href="/proxy/qbittorrent/">`) {
			t.Error("should inject <base> tag when document has no existing <base>")
		}
	})

	t.Run("base tag appears before interceptor script", func(t *testing.T) {
		content := []byte(`<html><head><title>Test</title></head><body></body></html>`)
		result := string(rewriter.injectInterceptor(content))
		baseIdx := strings.Index(result, `<base href=`)
		scriptIdx := strings.Index(result, `<script data-muximux-proxy`)
		if baseIdx < 0 || scriptIdx < 0 {
			t.Fatalf("missing base (%d) or script (%d)", baseIdx, scriptIdx)
		}
		if baseIdx >= scriptIdx {
			t.Error("<base> tag must appear before the interceptor script")
		}
	})

	t.Run("skips injection when base tag already exists", func(t *testing.T) {
		content := []byte(`<html><head><base href="/admin/"><title>Pi-hole</title></head><body></body></html>`)
		result := string(rewriter.injectInterceptor(content))
		if strings.Contains(result, `<base href="/proxy/qbittorrent/">`) {
			t.Error("should NOT inject <base> tag when document already has one")
		}
	})

	t.Run("skips injection for self-closing base tag", func(t *testing.T) {
		content := []byte(`<html><head><base href="/app/" /><title>App</title></head><body></body></html>`)
		result := string(rewriter.injectInterceptor(content))
		count := strings.Count(result, "<base ")
		if count != 1 {
			t.Errorf("expected exactly 1 <base> tag, got %d", count)
		}
	})
}

func TestRewriteResponseBodyETagStripping(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "example.com")

	tests := []struct {
		name             string
		contentType      string
		body             string
		hasETag          bool
		hasLastModified  bool
		wantETag         bool
		wantLastModified bool
	}{
		{
			name:             "strip ETag and Last-Modified from rewritten HTML",
			contentType:      "text/html",
			body:             "<html><head></head><body>hello</body></html>",
			hasETag:          true,
			hasLastModified:  true,
			wantETag:         false,
			wantLastModified: false,
		},
		{
			name:             "strip from CSS",
			contentType:      "text/css",
			body:             "body { color: red; }",
			hasETag:          true,
			hasLastModified:  true,
			wantETag:         false,
			wantLastModified: false,
		},
		{
			name:             "strip from JavaScript",
			contentType:      "application/javascript",
			body:             "console.log('hello');",
			hasETag:          true,
			hasLastModified:  true,
			wantETag:         false,
			wantLastModified: false,
		},
		{
			name:             "keep ETag on non-rewritable content (image)",
			contentType:      "image/png",
			body:             "PNG binary data",
			hasETag:          true,
			hasLastModified:  true,
			wantETag:         true,
			wantLastModified: true,
		},
		{
			name:             "no ETag present - no error",
			contentType:      "text/html",
			body:             "<html><head></head><body></body></html>",
			hasETag:          false,
			hasLastModified:  false,
			wantETag:         false,
			wantLastModified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader(tt.body)),
				ContentLength: int64(len(tt.body)),
			}
			resp.Header.Set("Content-Type", tt.contentType)
			if tt.hasETag {
				resp.Header.Set("ETag", `"abc123"`)
			}
			if tt.hasLastModified {
				resp.Header.Set("Last-Modified", "Mon, 10 Mar 2026 00:00:00 GMT")
			}

			_ = rewriteResponseBody(resp, rewriter)

			gotETag := resp.Header.Get("ETag") != ""
			gotLastModified := resp.Header.Get("Last-Modified") != ""

			if gotETag != tt.wantETag {
				t.Errorf("ETag present = %v, want %v", gotETag, tt.wantETag)
			}
			if gotLastModified != tt.wantLastModified {
				t.Errorf("Last-Modified present = %v, want %v", gotLastModified, tt.wantLastModified)
			}
		})
	}
}

func TestInterceptorScriptNotificationShim(t *testing.T) {
	rewriter := newContentRewriter("/proxy/app", "", "")
	script := string(rewriter.interceptorScript())

	// The shim replaces window.Notification so proxied apps can use the
	// standard API without knowing about the Muximux bridge.
	if !strings.Contains(script, `Object.defineProperty(window,"Notification"`) {
		t.Error("interceptor should redefine window.Notification")
	}

	// The shim must forward to the parent via postMessage using the bridge
	// protocol (type: muximux:notify).
	if !strings.Contains(script, `"type":"muximux:notify"`) && !strings.Contains(script, `type:"muximux:notify"`) {
		t.Error("interceptor Notification shim should postMessage with type muximux:notify")
	}

	// Permission state comes from the top-level Muximux window via a
	// postMessage handshake, not a hardcoded "granted". The shim starts
	// at "default" and updates when the parent replies with
	// muximux:notify-permission.
	if !strings.Contains(script, `_fakePerm="default"`) {
		t.Error("interceptor Notification shim should default permission to \"default\"")
	}
	if strings.Contains(script, `_fakeN.permission="granted"`) {
		t.Error("interceptor Notification shim should not hardcode permission=granted anymore")
	}
	if !strings.Contains(script, `"muximux:notify-query-permission"`) {
		t.Error("interceptor should query top-level permission on load")
	}
	if !strings.Contains(script, `"muximux:notify-request-permission"`) {
		t.Error("interceptor requestPermission should forward to the top-level window")
	}
	if !strings.Contains(script, `"muximux:notify-permission"`) {
		t.Error("interceptor should consume muximux:notify-permission replies from the parent")
	}
	if !strings.Contains(script, `requestPermission=function`) {
		t.Error("interceptor Notification shim should stub requestPermission")
	}
}

func TestInterceptorScriptNavigationAPISkipFlag(t *testing.T) {
	rewriter := newContentRewriter("/proxy/mealie", "", "")
	script := string(rewriter.interceptorScript())

	// The _skip flag must exist and be used in both _S() and the Navigation API handler
	// to prevent the handler from blocking internal replaceState calls that strip
	// the proxy prefix. Without this, Chrome's Navigation API fires synchronously
	// during replaceState, and the handler's e.preventDefault() blocks the strip.
	t.Run("_skip flag declared", func(t *testing.T) {
		if !strings.Contains(script, "_skip=false") {
			t.Error("interceptor must declare _skip flag")
		}
	})

	t.Run("_S sets _skip before stripping", func(t *testing.T) {
		if !strings.Contains(script, "_skip=true;_hrs.call(history,history.state") {
			t.Error("_S() must set _skip=true before calling _hrs to strip prefix")
		}
	})

	t.Run("Navigation API handler checks _skip", func(t *testing.T) {
		if !strings.Contains(script, "if(_skip||!e.canIntercept") {
			t.Error("Navigation API handler must check _skip flag before intercepting")
		}
	})

	t.Run("popstate handler sets _skip", func(t *testing.T) {
		// The popstate handler also calls _hrs to strip prefix and must set _skip
		popstateIdx := strings.Index(script, `addEventListener("popstate"`)
		if popstateIdx == -1 {
			t.Fatal("popstate handler not found in interceptor")
		}
		popstateSection := script[popstateIdx:]
		// Find the closing of the popstate handler (next },true)
		endIdx := strings.Index(popstateSection, "},true)")
		if endIdx == -1 {
			t.Fatal("could not find end of popstate handler")
		}
		popstateBody := popstateSection[:endIdx]
		if !strings.Contains(popstateBody, "_skip=true") {
			t.Error("popstate handler must set _skip=true before calling _hrs")
		}
	})
}

// TestRewriteResponseBody_GzipOpenFailureErrors covers findings.md H15:
// a malformed gzip body must surface an error up to ReverseProxy's
// ErrorHandler instead of silently returning nil, which had the proxy
// forward partial bytes as a "success".
func TestRewriteResponseBody_GzipOpenFailureErrors(t *testing.T) {
	resp := &http.Response{
		Header: make(http.Header),
		Body:   io.NopCloser(strings.NewReader("not really gzipped")),
	}
	resp.Header.Set(headerContentType, "text/html")
	resp.Header.Set(headerContentEncoding, "gzip")

	rewriter := newContentRewriter("/proxy/app", "", "")
	err := rewriteResponseBody(resp, rewriter)
	if err == nil {
		t.Error("expected error from malformed gzip, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "gzip") {
		t.Errorf("expected gzip-related error, got %v", err)
	}
}

// TestRewriteResponseBody_GzipBombCapped covers findings.md H16: the
// body reader must be capped so a small gzip payload that decompresses
// to gigabytes cannot OOM the process.
func TestRewriteResponseBody_GzipBombCapped(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	// 80 MB of zeroes compresses down to ~80 KB, giving us a realistic
	// ratio without making the test slow.
	chunk := make([]byte, 64*1024)
	for i := 0; i < 80*16; i++ {
		if _, err := gz.Write(chunk); err != nil {
			t.Fatalf("gz write: %v", err)
		}
	}
	gz.Close()

	resp := &http.Response{
		Header: make(http.Header),
		Body:   io.NopCloser(bytes.NewReader(buf.Bytes())),
	}
	resp.Header.Set(headerContentType, "text/html")
	resp.Header.Set(headerContentEncoding, "gzip")

	rewriter := newContentRewriter("/proxy/app", "", "")
	if err := rewriteResponseBody(resp, rewriter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Read back the (possibly truncated) body to make sure we didn't
	// fully allocate the decompressed output.
	body, _ := io.ReadAll(resp.Body)
	if int64(len(body)) > maxRewriteSize+10 {
		t.Errorf("body not capped: got %d bytes, want <= maxRewriteSize+10", len(body))
	}
}

// TestDialBackend_Timeout covers findings.md H17: a backend that never
// answers must not hang the WebSocket path indefinitely.
func TestDialBackend_Timeout(t *testing.T) {
	// A non-routable address (RFC 5737 TEST-NET-1). Dials will time out
	// after the configured timeout rather than ever connecting.
	targetURL, _ := url.Parse("http://192.0.2.1:65535")
	route := &proxyRoute{
		targetURL: targetURL,
		timeout:   150 * time.Millisecond,
	}

	start := time.Now()
	_, err := route.dialBackend()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected dial error against unroutable address")
	}
	if elapsed > 2*time.Second {
		t.Errorf("dial should time out quickly, took %v", elapsed)
	}
}

// TestStripSessionCookie validates that Muximux's own session cookie is
// removed from outgoing Cookie headers. Backends must never see the session
// identifier; leaking it gives any backend operator (or anyone between
// Muximux and that backend) full Muximux access (findings.md C2).
func TestStripSessionCookie(t *testing.T) {
	tests := []struct {
		name     string
		cookie   string
		strip    string
		expected string
		deleted  bool
	}{
		{
			name:     "strips named cookie, keeps others",
			cookie:   "muximux_session=secret123; prefs=dark",
			strip:    "muximux_session",
			expected: "prefs=dark",
		},
		{
			name:    "empty after stripping removes header",
			cookie:  "muximux_session=secret123",
			strip:   "muximux_session",
			deleted: true,
		},
		{
			name:     "empty name is a no-op",
			cookie:   "muximux_session=secret123; prefs=dark",
			strip:    "",
			expected: "muximux_session=secret123; prefs=dark",
		},
		{
			name:     "non-matching name is a no-op",
			cookie:   "other=value",
			strip:    "muximux_session",
			expected: "other=value",
		},
		{
			name:     "handles multiple cookies with the target name",
			cookie:   "muximux_session=a; prefs=dark; muximux_session=b",
			strip:    "muximux_session",
			expected: "prefs=dark",
		},
		{
			name:     "case-sensitive match (cookie names are case-sensitive)",
			cookie:   "Muximux_Session=secret; muximux_session=real",
			strip:    "muximux_session",
			expected: "Muximux_Session=secret",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != "" {
				req.Header.Set("Cookie", tt.cookie)
			}

			stripSessionCookie(req, tt.strip)

			got := req.Header.Get("Cookie")
			if tt.deleted {
				if got != "" {
					t.Errorf("expected Cookie header deleted, got %q", got)
				}
				return
			}
			if got != tt.expected {
				t.Errorf("Cookie = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestBuildDirector_StripsSessionCookie verifies the HTTP Director strips
// the configured session cookie before forwarding the request to the
// backend.
func TestBuildDirector_StripsSessionCookie(t *testing.T) {
	targetURL, _ := url.Parse("https://192.0.2.99:8080")
	director := buildDirector("/proxy/app", "", targetURL, nil, "muximux_session")

	req := httptest.NewRequest(http.MethodGet, "/proxy/app/status", nil)
	req.Header.Set("Cookie", "muximux_session=SECRET; backend_pref=light")

	director(req)

	got := req.Header.Get("Cookie")
	if strings.Contains(got, "muximux_session") {
		t.Errorf("session cookie leaked to backend: %q", got)
	}
	if !strings.Contains(got, "backend_pref=light") {
		t.Errorf("expected backend-owned cookie preserved, got %q", got)
	}
}

// TestBuildUpgradeRequest_DropsCRLFInjection covers findings.md H6.
// An attacker-supplied header name or value that contains CR/LF would
// otherwise smuggle an extra header (or a whole second request) into
// the raw upgrade bytes the proxy writes to the backend socket.
func TestBuildUpgradeRequest_DropsCRLFInjection(t *testing.T) {
	targetURL, _ := url.Parse("http://192.0.2.77:1234")
	route := &proxyRoute{
		name:        "TestApp",
		slug:        "testapp",
		proxyPrefix: "/proxy/testapp",
		targetURL:   targetURL,
		targetPath:  "/",
	}
	req := httptest.NewRequest(http.MethodGet, "/proxy/testapp/ws", nil)
	// Header value containing CRLF and a smuggled header.
	req.Header.Set("X-Benign", "value\r\nX-Admin: injected")
	// Header key containing CRLF.
	req.Header["X-Bad-Key\r\nSmuggled"] = []string{"whatever"}
	req.Header.Set("Upgrade", "websocket")

	out := string(route.buildUpgradeRequest(req, "/ws", "192.0.2.77:1234"))

	if strings.Contains(out, "X-Admin: injected") {
		t.Errorf("smuggled header slipped through:\n%s", out)
	}
	if strings.Contains(out, "X-Bad-Key") {
		t.Errorf("smuggled header name slipped through:\n%s", out)
	}
	// Sanity: the line separator count matches expected header count.
	if strings.Count(out, "\r\n") < 2 {
		t.Errorf("upgrade request missing expected CRLF line terminators:\n%s", out)
	}
}

// TestBuildUpgradeRequest_StripsCookieAndInjectsHeaders covers the WebSocket
// upgrade path. The raw upgrade request must not leak the Muximux session
// cookie (findings.md C2), and per-app ProxyHeaders must be injected so
// header-based auth still works on WS (findings.md H14).
func TestBuildUpgradeRequest_StripsCookieAndInjectsHeaders(t *testing.T) {
	targetURL, _ := url.Parse("http://192.0.2.77:1234")
	route := &proxyRoute{
		name:              "TestApp",
		slug:              "testapp",
		proxyPrefix:       "/proxy/testapp",
		targetURL:         targetURL,
		targetPath:        "/",
		sessionCookieName: "muximux_session",
		customHeaders: map[string]string{
			"Authorization": "Bearer secret-token",
			"X-Api-Key":     "abc123",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/proxy/testapp/ws", nil)
	req.Header.Set("Cookie", "muximux_session=LEAK; app_pref=dark")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	out := string(route.buildUpgradeRequest(req, "/ws", "192.0.2.77:1234"))

	if strings.Contains(out, "muximux_session=LEAK") {
		t.Errorf("session cookie leaked to backend on WS upgrade:\n%s", out)
	}
	if !strings.Contains(out, "Cookie: app_pref=dark") {
		t.Errorf("expected app cookie preserved, got:\n%s", out)
	}
	if !strings.Contains(out, "Authorization: Bearer secret-token") {
		t.Errorf("expected Authorization header injected, got:\n%s", out)
	}
	if !strings.Contains(out, "X-Api-Key: abc123") {
		t.Errorf("expected X-Api-Key header injected, got:\n%s", out)
	}
	if !strings.HasPrefix(out, "GET /ws HTTP/1.1\r\nHost: 192.0.2.77:1234\r\n") {
		t.Errorf("expected request line + Host, got:\n%s", out)
	}
}

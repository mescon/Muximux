package handlers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/mescon/muximux3/internal/config"
)

// ReverseProxyHandler handles reverse proxy requests on the main server
type ReverseProxyHandler struct {
	routes map[string]*proxyRoute
}

type proxyRoute struct {
	name        string
	slug        string
	proxyPrefix string
	targetURL   *url.URL
	targetPath  string
	proxy       *httputil.ReverseProxy
	rewriter    *contentRewriter
}

// contentRewriter handles URL rewriting in response content
type contentRewriter struct {
	proxyPrefix string
	targetPath  string // e.g., "/admin" - gets stripped and replaced with proxyPrefix
	targetHost  string
}

func newContentRewriter(proxyPrefix, targetPath, targetHost string) *contentRewriter {
	return &contentRewriter{
		proxyPrefix: proxyPrefix,
		targetPath:  strings.TrimSuffix(targetPath, "/"),
		targetHost:  targetHost,
	}
}

func (r *contentRewriter) rewrite(content []byte) []byte {
	result := string(content)

	// 1. Rewrite absolute URLs with the target host
	// e.g., http://10.9.0.100/admin/foo -> /proxy/slug/foo
	// e.g., http://10.9.0.100/foo -> /proxy/slug/foo
	if r.targetHost != "" {
		hostPattern := regexp.MustCompile(`https?://` + regexp.QuoteMeta(r.targetHost) + `(/[^"'\s>)]*)`)
		result = hostPattern.ReplaceAllStringFunc(result, func(match string) string {
			// Extract the path after the host
			idx := strings.Index(match, r.targetHost)
			path := match[idx+len(r.targetHost):]
			// Strip target path if present
			if r.targetPath != "" && strings.HasPrefix(path, r.targetPath) {
				path = strings.TrimPrefix(path, r.targetPath)
			}
			if path == "" {
				path = "/"
			}
			return r.proxyPrefix + path
		})
	}

	// 2. Rewrite paths that start with target path
	// e.g., /admin/foo -> /proxy/slug/foo
	if r.targetPath != "" {
		escapedPath := regexp.QuoteMeta(r.targetPath)

		// HTML attributes: href, src, action, data-*, poster, srcset, content (for meta refresh)
		// Match attribute="[targetPath]..." including just attribute="[targetPath]"
		attrPattern := regexp.MustCompile(`((?:href|src|action|poster|srcset|content|data-[a-zA-Z0-9-]+)\s*=\s*["'])` + escapedPath + `([/"']|[^"']*["'])`)
		result = attrPattern.ReplaceAllString(result, "${1}"+r.proxyPrefix+"${2}")

		// CSS url()
		urlPattern := regexp.MustCompile(`(url\s*\(\s*["']?)` + escapedPath + `([^"')]*["']?\s*\))`)
		result = urlPattern.ReplaceAllString(result, "${1}"+r.proxyPrefix+"${2}")

		// JavaScript string literals
		jsPattern := regexp.MustCompile(`(["'])` + escapedPath + `(/[^"']*)(["'])`)
		result = jsPattern.ReplaceAllString(result, "${1}"+r.proxyPrefix+"${2}${3}")
	}

	// 3. Handle srcset attribute FIRST (contains multiple paths separated by commas)
	// srcset="/sm.jpg 1x, /lg.jpg 2x" -> srcset="/proxy/app/sm.jpg 1x, /proxy/app/lg.jpg 2x"
	srcsetPattern := regexp.MustCompile(`(srcset\s*=\s*["'])([^"']+)(["'])`)
	result = srcsetPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Find quote positions
		quoteStart := strings.IndexAny(match, `"'`)
		if quoteStart == -1 {
			return match
		}
		quoteChar := match[quoteStart]
		quoteEnd := strings.LastIndex(match, string(quoteChar))
		if quoteEnd <= quoteStart {
			return match
		}

		srcsetValue := match[quoteStart+1 : quoteEnd]
		// Split by comma, rewrite each path while preserving spacing
		parts := strings.Split(srcsetValue, ",")
		for i, part := range parts {
			trimmed := strings.TrimSpace(part)
			if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "/proxy/") {
				// Preserve leading whitespace
				leadingSpace := ""
				if len(part) > 0 && part[0] == ' ' {
					leadingSpace = " "
				}
				parts[i] = leadingSpace + r.proxyPrefix + trimmed
			}
		}
		return match[:quoteStart+1] + strings.Join(parts, ",") + match[quoteEnd:]
	})

	// 4. Rewrite root-relative paths (/) that don't start with /proxy/
	// This catches ALL paths like /api, /static, /Content, etc.
	// IMPORTANT: This must run for ALL apps, including those without a subpath
	// For any attribute value that starts with / but not /proxy/
	// Skip srcset as it's handled above
	rootPathAttrPattern := regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*\s*=\s*["'])/([a-zA-Z0-9_][^"']*)`)
	result = rootPathAttrPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Skip if already rewritten
		if strings.Contains(match, "/proxy/") {
			return match
		}
		// Skip srcset (handled separately)
		if strings.HasPrefix(strings.ToLower(match), "srcset") {
			return match
		}
		// Find the quote and the path start
		quoteIdx := strings.LastIndex(match, `"`)
		if quoteIdx == -1 {
			quoteIdx = strings.LastIndex(match, `'`)
		}
		if quoteIdx == -1 {
			return match
		}
		// Extract parts
		prefix := match[:quoteIdx+1] // Including the opening quote
		path := match[quoteIdx+1:]   // The /path part
		return prefix + r.proxyPrefix + path
	})

	// CSS url() with root paths
	rootPathUrlPattern := regexp.MustCompile(`(url\s*\(\s*["']?)/([a-zA-Z0-9_-][^"')]*["']?\s*\))`)
	result = rootPathUrlPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		// Find the first / after url(
		idx := strings.Index(match, "/")
		if idx == -1 {
			return match
		}
		return match[:idx] + r.proxyPrefix + match[idx:]
	})

	// 4. Rewrite <base href="..."> tag
	basePattern := regexp.MustCompile(`(<base[^>]*href\s*=\s*["'])([^"']*)(["'])`)
	result = basePattern.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the href value
		startQuote := strings.Index(match, `href`)
		if startQuote == -1 {
			return match
		}
		// Find the quote after href=
		quoteStart := strings.IndexAny(match[startQuote:], `"'`)
		if quoteStart == -1 {
			return match
		}
		quoteStart += startQuote
		quoteChar := match[quoteStart]
		quoteEnd := strings.Index(match[quoteStart+1:], string(quoteChar))
		if quoteEnd == -1 {
			return match
		}
		quoteEnd += quoteStart + 1

		href := match[quoteStart+1 : quoteEnd]

		// Rewrite the href
		if r.targetPath != "" && strings.HasPrefix(href, r.targetPath) {
			href = r.proxyPrefix + strings.TrimPrefix(href, r.targetPath)
		} else if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "/proxy/") {
			href = r.proxyPrefix + href
		}

		return match[:quoteStart+1] + href + match[quoteEnd:]
	})

	// 5. Rewrite JavaScript/JSON base path patterns (for SPAs like Sonarr/Radarr)
	// Handle empty strings: urlBase: '' or "urlBase": ""
	// Match common base path variable names with empty values
	urlBaseEmptyPattern := regexp.MustCompile(`("?)(urlBase|basePath|baseUrl|baseHref)("?)\s*[:=]\s*(['"])(['"])`)
	result = urlBaseEmptyPattern.ReplaceAllString(result, `${1}${2}${3}: "`+r.proxyPrefix+`"`)

	// 6. Rewrite JSON paths generically - any "key": "/path" where path doesn't start with /proxy/
	// This handles apiRoot, basePath, redirectUrl, etc. without hardcoding names
	jsonPathPattern := regexp.MustCompile(`("[\w]+"\s*:\s*")(/[^"/][^"]*)(")`)
	result = jsonPathPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Skip if already has proxy prefix
		if strings.Contains(match, "/proxy/") {
			return match
		}
		// Find the path part (between the second " and third ")
		firstQuote := strings.Index(match, `"/`)
		if firstQuote == -1 {
			return match
		}
		return match[:firstQuote+1] + r.proxyPrefix + match[firstQuote+1:]
	})

	// 7. Rewrite JSON arrays of paths: ["/path1", "/path2"]
	jsonArrayPathPattern := regexp.MustCompile(`(\[|,)\s*"(/[^"]+)"`)
	result = jsonArrayPathPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		// Find the path start
		idx := strings.Index(match, `"/`)
		if idx == -1 {
			return match
		}
		return match[:idx+1] + r.proxyPrefix + match[idx+1:]
	})

	// 8. CSS @import statements
	// @import "/styles.css" or @import '/styles.css'
	cssImportPattern := regexp.MustCompile(`(@import\s+["'])(/[^"']+)(["'])`)
	result = cssImportPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		idx := strings.Index(match, `"/`)
		if idx == -1 {
			idx = strings.Index(match, `'/`)
		}
		if idx == -1 {
			return match
		}
		return match[:idx+1] + r.proxyPrefix + match[idx+1:]
	})

	// @import url("/styles.css") or @import url('/styles.css')
	cssImportUrlPattern := regexp.MustCompile(`(@import\s+url\s*\(\s*["']?)(/[^"')]+)(["']?\s*\))`)
	result = cssImportUrlPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		idx := strings.Index(match, `(/`)
		if idx == -1 {
			return match
		}
		return match[:idx+1] + r.proxyPrefix + match[idx+1:]
	})

	// 9. SVG use/image href attributes
	svgHrefPattern := regexp.MustCompile(`(<(?:use|image)[^>]*(?:href|xlink:href)\s*=\s*["'])(/[^"'#]+)(#[^"']*)?(['"])`)
	result = svgHrefPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		// Find the path (starts with /)
		idx := strings.Index(match, `"/`)
		if idx == -1 {
			idx = strings.Index(match, `'/`)
		}
		if idx == -1 {
			return match
		}
		return match[:idx+1] + r.proxyPrefix + match[idx+1:]
	})

	return []byte(result)
}

// rewriteCookiePath rewrites the Path attribute in Set-Cookie headers
func (r *contentRewriter) rewriteCookiePath(setCookie string) string {
	parts := strings.Split(setCookie, ";")
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "path=") {
			path := trimmed[5:] // Get value after "path=" or "Path="

			// Rewrite path
			if r.targetPath != "" && strings.HasPrefix(path, r.targetPath) {
				path = r.proxyPrefix + strings.TrimPrefix(path, r.targetPath)
				if path == r.proxyPrefix {
					path = r.proxyPrefix + "/"
				}
			} else if strings.HasPrefix(path, "/") && !strings.HasPrefix(path, r.proxyPrefix) {
				path = r.proxyPrefix + path
			}
			parts[i] = " Path=" + path
		}
	}
	return strings.Join(parts, ";")
}

// NewReverseProxyHandler creates a new reverse proxy handler
func NewReverseProxyHandler(apps []config.AppConfig) *ReverseProxyHandler {
	h := &ReverseProxyHandler{
		routes: make(map[string]*proxyRoute),
	}

	for _, app := range apps {
		if !app.Proxy || !app.Enabled {
			continue
		}

		targetURL, err := url.Parse(app.URL)
		if err != nil {
			continue
		}

		// Skip apps that already use a proxy path (to avoid loops)
		if strings.HasPrefix(app.URL, "/proxy/") {
			continue
		}

		slug := slugify(app.Name)
		proxyPrefix := "/proxy/" + slug
		targetPath := targetURL.Path
		if targetPath == "" {
			targetPath = "/"
		}

		// Create content rewriter
		rewriter := newContentRewriter(proxyPrefix, targetPath, targetURL.Host)

		// Capture variables for closure
		capturedProxyPrefix := proxyPrefix
		capturedTargetPath := targetPath
		capturedTargetURL := targetURL

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				// Strip the /proxy/{slug} prefix from the request path
				reqPath := req.URL.Path
				if strings.HasPrefix(reqPath, capturedProxyPrefix) {
					reqPath = strings.TrimPrefix(reqPath, capturedProxyPrefix)
				}
				if reqPath == "" {
					reqPath = "/"
				}

				// Join target path with remaining request path
				// Exception: /api paths typically live at root, not under the target path
				// This handles apps like Pi-hole where UI is at /admin but API is at /api
				trimmedTargetPath := strings.TrimSuffix(capturedTargetPath, "/")
				if trimmedTargetPath != "" && trimmedTargetPath != "/" {
					// Check if this is an API path that should bypass the target path
					if strings.HasPrefix(reqPath, "/api/") || reqPath == "/api" {
						req.URL.Path = reqPath
					} else if strings.HasPrefix(reqPath, "/") {
						req.URL.Path = trimmedTargetPath + reqPath
					} else {
						req.URL.Path = trimmedTargetPath + "/" + reqPath
					}
				} else {
					req.URL.Path = reqPath
				}

				req.URL.Scheme = capturedTargetURL.Scheme
				req.URL.Host = capturedTargetURL.Host

				// Set proper headers
				req.Host = capturedTargetURL.Host
				req.Header.Set("X-Forwarded-Host", req.Host)
				req.Header.Set("X-Real-IP", req.RemoteAddr)
				req.Header.Set("Accept-Encoding", "gzip, identity")
			},
			ModifyResponse: createModifyResponse(capturedProxyPrefix, capturedTargetPath, rewriter),
		}

		h.routes[slug] = &proxyRoute{
			name:        app.Name,
			slug:        slug,
			proxyPrefix: proxyPrefix,
			targetURL:   targetURL,
			targetPath:  targetPath,
			proxy:       proxy,
			rewriter:    rewriter,
		}
	}

	return h
}

func createModifyResponse(proxyPrefix, targetPath string, rewriter *contentRewriter) func(*http.Response) error {
	return func(resp *http.Response) error {
		// Remove headers that prevent iframe embedding
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("X-Content-Type-Options")

		// Rewrite Location headers for redirects
		if location := resp.Header.Get("Location"); location != "" {
			location = rewriteLocation(location, proxyPrefix, targetPath)
			resp.Header.Set("Location", location)
		}

		// Rewrite Content-Location header
		if contentLoc := resp.Header.Get("Content-Location"); contentLoc != "" {
			contentLoc = rewriteLocation(contentLoc, proxyPrefix, targetPath)
			resp.Header.Set("Content-Location", contentLoc)
		}

		// Rewrite Refresh header if present
		if refresh := resp.Header.Get("Refresh"); refresh != "" {
			if idx := strings.Index(strings.ToLower(refresh), "url="); idx != -1 {
				urlPart := strings.TrimSpace(refresh[idx+4:])
				urlPart = rewriteLocation(urlPart, proxyPrefix, targetPath)
				resp.Header.Set("Refresh", refresh[:idx+4]+urlPart)
			}
		}

		// Rewrite Set-Cookie headers
		cookies := resp.Header.Values("Set-Cookie")
		if len(cookies) > 0 {
			resp.Header.Del("Set-Cookie")
			for _, cookie := range cookies {
				rewritten := rewriter.rewriteCookiePath(cookie)
				resp.Header.Add("Set-Cookie", rewritten)
			}
		}

		// Rewrite Link headers (for preload, prefetch, etc.)
		// Link: </style.css>; rel=preload -> Link: </proxy/app/style.css>; rel=preload
		linkHeaders := resp.Header.Values("Link")
		if len(linkHeaders) > 0 {
			resp.Header.Del("Link")
			linkPathPattern := regexp.MustCompile(`<(/[^>]+)>`)
			for _, link := range linkHeaders {
				rewritten := linkPathPattern.ReplaceAllStringFunc(link, func(match string) string {
					if strings.Contains(match, "/proxy/") {
						return match
					}
					// Extract path between < and >
					path := match[1 : len(match)-1]
					return "<" + proxyPrefix + path + ">"
				})
				resp.Header.Add("Link", rewritten)
			}
		}

		// Check if we should rewrite content
		contentType := resp.Header.Get("Content-Type")
		if !shouldRewriteContent(contentType) {
			return nil
		}

		// Read and potentially decompress response body
		var reader io.Reader = resp.Body
		isGzipped := strings.Contains(resp.Header.Get("Content-Encoding"), "gzip")

		if isGzipped {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return nil
			}
			reader = gzReader
			defer gzReader.Close()
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			return nil
		}
		resp.Body.Close()

		// Rewrite content
		rewritten := rewriter.rewrite(body)

		// Update response
		resp.Body = io.NopCloser(bytes.NewReader(rewritten))
		resp.ContentLength = int64(len(rewritten))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(rewritten)))
		resp.Header.Del("Content-Encoding")

		return nil
	}
}

func rewriteLocation(location, proxyPrefix, targetPath string) string {
	if !strings.HasPrefix(location, "/") || strings.HasPrefix(location, "/proxy/") {
		return location
	}

	trimmedTarget := strings.TrimSuffix(targetPath, "/")
	if trimmedTarget != "" && strings.HasPrefix(location, trimmedTarget) {
		remaining := strings.TrimPrefix(location, trimmedTarget)
		if remaining == "" {
			remaining = "/"
		}
		return proxyPrefix + remaining
	}

	return proxyPrefix + location
}

func shouldRewriteContent(contentType string) bool {
	rewriteTypes := []string{
		"text/html",
		"text/css",
		"text/javascript",
		"application/javascript",
		"application/x-javascript",
		"application/json",
		"text/xml",
		"application/xml",
		"application/xhtml",
	}

	contentType = strings.ToLower(contentType)
	for _, t := range rewriteTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}
	return false
}

// ServeHTTP handles proxy requests
func (h *ReverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/proxy/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		http.Error(w, "Invalid proxy path", http.StatusBadRequest)
		return
	}

	slug := parts[0]
	route, exists := h.routes[slug]
	if !exists {
		http.Error(w, "App not found: "+slug, http.StatusNotFound)
		return
	}

	route.proxy.ServeHTTP(w, r)
}

// HasRoutes returns true if there are any proxy routes configured
func (h *ReverseProxyHandler) HasRoutes() bool {
	return len(h.routes) > 0
}

// GetRoutes returns a list of configured proxy slugs
func (h *ReverseProxyHandler) GetRoutes() []string {
	routes := make([]string, 0, len(h.routes))
	for slug := range h.routes {
		routes = append(routes, slug)
	}
	return routes
}

// slugify is defined in api.go

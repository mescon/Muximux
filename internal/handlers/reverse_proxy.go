package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/mescon/muximux/v3/internal/config"
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

	// 0. Strip integrity attributes since we modify content (breaks SRI hash)
	// Also strip crossorigin as it's often paired with integrity
	integrityPattern := regexp.MustCompile(`\s*(integrity|crossorigin)\s*=\s*["'][^"']*["']`)
	result = integrityPattern.ReplaceAllString(result, "")

	// 0b. Strip dynamic SRI assignment in webpack loaders (e.g., Plex)
	// Pattern: f.integrity=b.sriHashes[d] or similar variations
	// This prevents webpack from adding integrity at runtime when loading chunks
	dynamicSriPattern := regexp.MustCompile(`\w+\.integrity\s*=\s*\w+\.sriHashes\[[^\]]+\],?`)
	result = dynamicSriPattern.ReplaceAllString(result, "")

	// Also neutralize the sriHashes object itself if it exists
	// b.sriHashes={...} -> b.sriHashes={}
	sriHashesPattern := regexp.MustCompile(`(\w+\.sriHashes)\s*=\s*\{[^}]+\}`)
	result = sriHashesPattern.ReplaceAllString(result, "${1}={}")

	// 1. Rewrite absolute URLs with the target host
	// e.g., http://192.0.2.100/admin/foo -> /proxy/slug/foo
	// e.g., http://192.0.2.100/foo -> /proxy/slug/foo
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
	// Note: This may cause double-prefixing in some apps, but we handle that in the Director
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

	// 7. CSS image-set() function — must run before JSON array handler
	// because `, "/2x.png"` inside image-set() would otherwise match the JSON array pattern.
	// image-set("/1x.png" 1x, "/2x.png" 2x) -> image-set("/proxy/app/1x.png" 1x, "/proxy/app/2x.png" 2x)
	imageSetPattern := regexp.MustCompile(`(image-set\s*\()([^)]+)(\))`)
	result = imageSetPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Rewrite each quoted root-relative path inside image-set()
		pathInSet := regexp.MustCompile(`(["'])/([a-zA-Z0-9_-][^"']*)(["'])`)
		return pathInSet.ReplaceAllStringFunc(match, func(inner string) string {
			if strings.Contains(inner, "/proxy/") {
				return inner
			}
			q := string(inner[0])
			return q + r.proxyPrefix + "/" + inner[2:len(inner)-1] + q
		})
	})

	// 8. Rewrite JSON arrays of paths: ["/path1", "/path2"]
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
				reqPath := strings.TrimPrefix(req.URL.Path, capturedProxyPrefix)
				if reqPath == "" {
					reqPath = "/"
				}

				// Handle double-prefixing caused by SPAs that construct URLs with urlBase + endpoint
				// e.g., /api/v3/proxy/radarr/movie should become /api/v3/movie
				// This happens when the app does urlBase + endpoint before AJAX adds apiRoot
				if strings.Contains(reqPath, capturedProxyPrefix) {
					reqPath = strings.ReplaceAll(reqPath, capturedProxyPrefix, "")
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

				// Preserve original client information for the backend
				// Extract client IP (RemoteAddr includes port, e.g., "192.0.2.5:54321")
				clientIP := req.RemoteAddr
				if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
					clientIP = host
				}

				// Set standard proxy headers
				// X-Forwarded-For: Client IP (append to existing if present)
				if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
					req.Header.Set("X-Forwarded-For", prior+", "+clientIP)
				} else {
					req.Header.Set("X-Forwarded-For", clientIP)
				}

				// X-Forwarded-Host: Original host requested by client
				if req.Header.Get("X-Forwarded-Host") == "" {
					req.Header.Set("X-Forwarded-Host", req.Host)
				}

				// X-Forwarded-Proto: Original protocol
				proto := "http"
				if req.TLS != nil {
					proto = "https"
				}
				if req.Header.Get("X-Forwarded-Proto") == "" {
					req.Header.Set("X-Forwarded-Proto", proto)
				}

				// X-Real-IP: Original client IP (commonly used by nginx)
				if req.Header.Get("X-Real-IP") == "" {
					req.Header.Set("X-Real-IP", clientIP)
				}

				// Now set the target host
				req.Host = capturedTargetURL.Host
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

		// Rewrite Location headers for redirects (301, 302, 303, 307, 308)
		if location := resp.Header.Get("Location"); location != "" {
			location = rewriteLocation(location, proxyPrefix, targetPath, rewriter.targetHost)
			resp.Header.Set("Location", location)
		}

		// Rewrite Content-Location header
		if contentLoc := resp.Header.Get("Content-Location"); contentLoc != "" {
			contentLoc = rewriteLocation(contentLoc, proxyPrefix, targetPath, rewriter.targetHost)
			resp.Header.Set("Content-Location", contentLoc)
		}

		// Rewrite Refresh header if present (meta refresh redirects)
		if refresh := resp.Header.Get("Refresh"); refresh != "" {
			if idx := strings.Index(strings.ToLower(refresh), "url="); idx != -1 {
				urlPart := strings.TrimSpace(refresh[idx+4:])
				urlPart = rewriteLocation(urlPart, proxyPrefix, targetPath, rewriter.targetHost)
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

func rewriteLocation(location, proxyPrefix, targetPath, targetHost string) string {
	// Handle absolute URLs pointing to the target server
	// e.g., http://192.0.2.10:32400/web/index.html -> /proxy/plex/index.html
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		parsed, err := url.Parse(location)
		if err != nil {
			return location
		}
		// Only rewrite if it's pointing to our target host
		if parsed.Host == targetHost {
			location = parsed.Path
			if parsed.RawQuery != "" {
				location += "?" + parsed.RawQuery
			}
			// Fall through to path rewriting below
		} else {
			return location // Different host, don't rewrite
		}
	}

	// Skip if already rewritten or not a path
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

// isWebSocketUpgrade returns true if the request is a WebSocket upgrade.
// The Connection header can be comma-separated (e.g. "keep-alive, Upgrade"),
// so we check whether "upgrade" appears anywhere in it, not as an exact match.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// resolveBackendPath translates a proxy request path to the backend path,
// applying the same logic as the Director (strip prefix, double-prefix, api bypass).
func (route *proxyRoute) resolveBackendPath(reqPath string) string {
	path := strings.TrimPrefix(reqPath, route.proxyPrefix)
	if path == "" {
		path = "/"
	}

	// Double-prefix stripping
	if strings.Contains(path, route.proxyPrefix) {
		path = strings.ReplaceAll(path, route.proxyPrefix, "")
	}

	trimmed := strings.TrimSuffix(route.targetPath, "/")
	if trimmed != "" && trimmed != "/" {
		if strings.HasPrefix(path, "/api/") || path == "/api" {
			return path
		}
		if strings.HasPrefix(path, "/") {
			return trimmed + path
		}
		return trimmed + "/" + path
	}
	return path
}

// handleWebSocket hijacks the client connection and proxies raw WebSocket frames
// to/from the backend. Path rewriting uses the same logic as normal HTTP requests.
func (route *proxyRoute) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Resolve the backend path
	backendPath := route.resolveBackendPath(r.URL.Path)
	if r.URL.RawQuery != "" {
		backendPath += "?" + r.URL.RawQuery
	}

	// Dial the backend
	targetHost := route.targetURL.Host
	scheme := route.targetURL.Scheme

	var backendConn net.Conn
	var err error
	if scheme == "https" {
		host := targetHost
		if h, _, splitErr := net.SplitHostPort(targetHost); splitErr == nil {
			host = h
		}
		backendConn, err = tls.Dial("tcp", targetHost, &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: true, //nolint:gosec // internal network backends
		})
	} else {
		// If no port in host, default to 80
		dialHost := targetHost
		if _, _, splitErr := net.SplitHostPort(targetHost); splitErr != nil {
			dialHost = targetHost + ":80"
		}
		backendConn, err = net.Dial("tcp", dialHost)
	}
	if err != nil {
		log.Printf("[proxy-ws] %s: failed to dial backend %s: %v", route.name, targetHost, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer backendConn.Close()

	// Add proxy headers to the original request before forwarding
	clientIP := r.RemoteAddr
	if host, _, splitErr := net.SplitHostPort(r.RemoteAddr); splitErr == nil {
		clientIP = host
	}
	if prior := r.Header.Get("X-Forwarded-For"); prior != "" {
		r.Header.Set("X-Forwarded-For", prior+", "+clientIP)
	} else {
		r.Header.Set("X-Forwarded-For", clientIP)
	}
	if r.Header.Get("X-Forwarded-Host") == "" {
		r.Header.Set("X-Forwarded-Host", r.Host)
	}
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	if r.Header.Get("X-Forwarded-Proto") == "" {
		r.Header.Set("X-Forwarded-Proto", proto)
	}
	if r.Header.Get("X-Real-IP") == "" {
		r.Header.Set("X-Real-IP", clientIP)
	}

	// Build the upgrade request to send to the backend
	var reqBuf bytes.Buffer
	fmt.Fprintf(&reqBuf, "%s %s HTTP/1.1\r\n", r.Method, backendPath)
	fmt.Fprintf(&reqBuf, "Host: %s\r\n", targetHost)

	// Forward all client headers except Host (already set above)
	for key, values := range r.Header {
		if strings.EqualFold(key, "Host") {
			continue
		}
		for _, v := range values {
			fmt.Fprintf(&reqBuf, "%s: %s\r\n", key, v)
		}
	}
	reqBuf.WriteString("\r\n")

	// Send upgrade request to backend
	if _, err = backendConn.Write(reqBuf.Bytes()); err != nil {
		log.Printf("[proxy-ws] %s: failed to write upgrade request: %v", route.name, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// Read the backend's response
	backendBuf := bufio.NewReader(backendConn)
	resp, err := http.ReadResponse(backendBuf, r)
	if err != nil {
		log.Printf("[proxy-ws] %s: failed to read upgrade response: %v", route.name, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// If backend didn't upgrade, forward the error response as-is
	if resp.StatusCode != http.StatusSwitchingProtocols {
		log.Printf("[proxy-ws] %s: backend returned %d instead of 101", route.name, resp.StatusCode)
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		if resp.Body != nil {
			io.Copy(w, resp.Body)
			resp.Body.Close()
		}
		return
	}

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("[proxy-ws] %s: response writer does not support hijacking", route.name)
		http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		log.Printf("[proxy-ws] %s: failed to hijack client connection: %v", route.name, err)
		return
	}
	defer clientConn.Close()

	// Forward the 101 response to the client
	var respBuf bytes.Buffer
	fmt.Fprintf(&respBuf, "HTTP/1.1 101 Switching Protocols\r\n")
	for k, vs := range resp.Header {
		for _, v := range vs {
			fmt.Fprintf(&respBuf, "%s: %s\r\n", k, v)
		}
	}
	respBuf.WriteString("\r\n")

	if _, err = clientConn.Write(respBuf.Bytes()); err != nil {
		log.Printf("[proxy-ws] %s: failed to write upgrade response to client: %v", route.name, err)
		return
	}

	// Bidirectional copy: pipe frames between client and backend.
	// If the backend's bufio reader has buffered data (e.g. a frame sent
	// immediately after the handshake), flush it to the client first.
	var wg sync.WaitGroup
	wg.Add(2)

	// Backend → Client
	go func() {
		defer wg.Done()
		// First drain any data already buffered in the reader
		if backendBuf.Buffered() > 0 {
			buffered, _ := backendBuf.Peek(backendBuf.Buffered())
			clientConn.Write(buffered)
			backendBuf.Discard(len(buffered))
		}
		io.Copy(clientConn, backendConn)
	}()

	// Client → Backend
	go func() {
		defer wg.Done()
		// Drain any buffered client data
		if clientBuf.Reader.Buffered() > 0 {
			buffered, _ := clientBuf.Peek(clientBuf.Reader.Buffered())
			backendConn.Write(buffered)
			clientBuf.Reader.Discard(len(buffered))
		}
		io.Copy(backendConn, clientConn)
	}()

	wg.Wait()
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

	// WebSocket upgrade requests use hijack-based proxying
	if isWebSocketUpgrade(r) {
		route.handleWebSocket(w, r)
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

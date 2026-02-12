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
	result = r.stripIntegrity(result)
	result = r.rewriteAbsoluteURLs(result)
	result = r.rewriteTargetPaths(result)
	result = r.rewriteSrcset(result)
	result = r.rewriteRootPaths(result)
	result = r.rewriteURLBase(result)
	result = r.rewriteJSONPaths(result)
	result = r.rewriteImageSet(result)
	result = r.rewriteJSONArrayPaths(result)
	result = r.rewriteCSSImports(result)
	result = r.rewriteSVGHrefs(result)
	return []byte(result)
}

// stripIntegrity removes integrity and crossorigin attributes since we modify
// content (breaks SRI hashes), and neutralises dynamic SRI in webpack loaders.
func (r *contentRewriter) stripIntegrity(result string) string {
	// Strip integrity/crossorigin HTML attributes
	integrityPattern := regexp.MustCompile(`\s*(integrity|crossorigin)\s*=\s*["'][^"']*["']`)
	result = integrityPattern.ReplaceAllString(result, "")

	// Strip dynamic SRI assignment in webpack loaders (e.g., Plex)
	dynamicSriPattern := regexp.MustCompile(`\w+\.integrity\s*=\s*\w+\.sriHashes\[[^\]]+\],?`)
	result = dynamicSriPattern.ReplaceAllString(result, "")

	// Neutralize the sriHashes object itself: b.sriHashes={...} -> b.sriHashes={}
	sriHashesPattern := regexp.MustCompile(`(\w+\.sriHashes)\s*=\s*\{[^}]+\}`)
	result = sriHashesPattern.ReplaceAllString(result, "${1}={}")

	return result
}

// rewriteAbsoluteURLs rewrites absolute URLs containing the target host.
// e.g., http://192.0.2.100/admin/foo -> /proxy/slug/foo
func (r *contentRewriter) rewriteAbsoluteURLs(result string) string {
	if r.targetHost == "" {
		return result
	}
	hostPattern := regexp.MustCompile(`https?://` + regexp.QuoteMeta(r.targetHost) + `(/[^"'\s>)]*)`)
	return hostPattern.ReplaceAllStringFunc(result, func(match string) string {
		idx := strings.Index(match, r.targetHost)
		path := match[idx+len(r.targetHost):]
		if r.targetPath != "" && strings.HasPrefix(path, r.targetPath) {
			path = strings.TrimPrefix(path, r.targetPath)
		}
		if path == "" {
			path = "/"
		}
		return r.proxyPrefix + path
	})
}

// rewriteTargetPaths rewrites paths that start with the target path.
// e.g., /admin/foo -> /proxy/slug/foo
func (r *contentRewriter) rewriteTargetPaths(result string) string {
	if r.targetPath == "" {
		return result
	}
	escapedPath := regexp.QuoteMeta(r.targetPath)

	// HTML attributes: href, src, action, data-*, poster, srcset, content (for meta refresh)
	attrPattern := regexp.MustCompile(`((?:href|src|action|poster|srcset|content|data-[a-zA-Z0-9-]+)\s*=\s*["'])` + escapedPath + `([/"']|[^"']*["'])`)
	result = attrPattern.ReplaceAllString(result, "${1}"+r.proxyPrefix+"${2}")

	// CSS url()
	urlPattern := regexp.MustCompile(`(url\s*\(\s*["']?)` + escapedPath + `([^"')]*["']?\s*\))`)
	result = urlPattern.ReplaceAllString(result, "${1}"+r.proxyPrefix+"${2}")

	// JavaScript string literals
	jsPattern := regexp.MustCompile(`(["'])` + escapedPath + `(/[^"']*)(["'])`)
	result = jsPattern.ReplaceAllString(result, "${1}"+r.proxyPrefix+"${2}${3}")

	return result
}

// rewriteSrcset handles srcset attributes which contain multiple comma-separated paths.
// srcset="/sm.jpg 1x, /lg.jpg 2x" -> srcset="/proxy/app/sm.jpg 1x, /proxy/app/lg.jpg 2x"
func (r *contentRewriter) rewriteSrcset(result string) string {
	srcsetPattern := regexp.MustCompile(`(srcset\s*=\s*["'])([^"']+)(["'])`)
	return srcsetPattern.ReplaceAllStringFunc(result, func(match string) string {
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
		parts := strings.Split(srcsetValue, ",")
		for i, part := range parts {
			trimmed := strings.TrimSpace(part)
			if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "/proxy/") {
				leadingSpace := ""
				if len(part) > 0 && part[0] == ' ' {
					leadingSpace = " "
				}
				parts[i] = leadingSpace + r.proxyPrefix + trimmed
			}
		}
		return match[:quoteStart+1] + strings.Join(parts, ",") + match[quoteEnd:]
	})
}

// rewriteRootPaths rewrites root-relative paths (/) that don't start with /proxy/,
// including attribute values, CSS url(), and <base href="..."> tags.
func (r *contentRewriter) rewriteRootPaths(result string) string {
	// Attribute values starting with / but not /proxy/ (skip srcset, handled above)
	rootPathAttrPattern := regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*\s*=\s*["'])/([a-zA-Z0-9_][^"']*)`)
	result = rootPathAttrPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		if strings.HasPrefix(strings.ToLower(match), "srcset") {
			return match
		}
		quoteIdx := strings.LastIndex(match, `"`)
		if quoteIdx == -1 {
			quoteIdx = strings.LastIndex(match, `'`)
		}
		if quoteIdx == -1 {
			return match
		}
		prefix := match[:quoteIdx+1]
		path := match[quoteIdx+1:]
		return prefix + r.proxyPrefix + path
	})

	// CSS url() with root paths
	rootPathUrlPattern := regexp.MustCompile(`(url\s*\(\s*["']?)/([a-zA-Z0-9_-][^"')]*["']?\s*\))`)
	result = rootPathUrlPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		idx := strings.Index(match, "/")
		if idx == -1 {
			return match
		}
		return match[:idx] + r.proxyPrefix + match[idx:]
	})

	// <base href="..."> tag
	basePattern := regexp.MustCompile(`(<base[^>]*href\s*=\s*["'])([^"']*)(["'])`)
	result = basePattern.ReplaceAllStringFunc(result, func(match string) string {
		startQuote := strings.Index(match, `href`)
		if startQuote == -1 {
			return match
		}
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

		if r.targetPath != "" && strings.HasPrefix(href, r.targetPath) {
			href = r.proxyPrefix + strings.TrimPrefix(href, r.targetPath)
		} else if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "/proxy/") {
			href = r.proxyPrefix + href
		}

		return match[:quoteStart+1] + href + match[quoteEnd:]
	})

	return result
}

// rewriteURLBase rewrites JavaScript/JSON base path patterns for SPAs (e.g., Sonarr/Radarr).
// Handles empty base path strings like urlBase or basePath set to blank values.
func (r *contentRewriter) rewriteURLBase(result string) string {
	urlBaseEmptyPattern := regexp.MustCompile(`("?)(urlBase|basePath|baseUrl|baseHref)("?)\s*[:=]\s*(['"])(['"])`)
	return urlBaseEmptyPattern.ReplaceAllString(result, `${1}${2}${3}: "`+r.proxyPrefix+`"`)
}

// rewriteJSONPaths rewrites generic JSON paths: any "key": "/path" where path
// doesn't start with /proxy/. Handles apiRoot, basePath, redirectUrl, etc.
func (r *contentRewriter) rewriteJSONPaths(result string) string {
	jsonPathPattern := regexp.MustCompile(`("[\w]+"\s*:\s*")(/[^"/][^"]*)(")`)
	return jsonPathPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		firstQuote := strings.Index(match, `"/`)
		if firstQuote == -1 {
			return match
		}
		return match[:firstQuote+1] + r.proxyPrefix + match[firstQuote+1:]
	})
}

// rewriteImageSet rewrites CSS image-set() functions. Must run before JSON array
// handler because `, "/2x.png"` inside image-set() would otherwise match.
func (r *contentRewriter) rewriteImageSet(result string) string {
	imageSetPattern := regexp.MustCompile(`(image-set\s*\()([^)]+)(\))`)
	return imageSetPattern.ReplaceAllStringFunc(result, func(match string) string {
		pathInSet := regexp.MustCompile(`(["'])/([a-zA-Z0-9_-][^"']*)(["'])`)
		return pathInSet.ReplaceAllStringFunc(match, func(inner string) string {
			if strings.Contains(inner, "/proxy/") {
				return inner
			}
			q := string(inner[0])
			return q + r.proxyPrefix + "/" + inner[2:len(inner)-1] + q
		})
	})
}

// rewriteJSONArrayPaths rewrites JSON arrays of paths: ["/path1", "/path2"]
func (r *contentRewriter) rewriteJSONArrayPaths(result string) string {
	jsonArrayPathPattern := regexp.MustCompile(`(\[|,)\s*"(/[^"]+)"`)
	return jsonArrayPathPattern.ReplaceAllStringFunc(result, func(match string) string {
		if strings.Contains(match, "/proxy/") {
			return match
		}
		idx := strings.Index(match, `"/`)
		if idx == -1 {
			return match
		}
		return match[:idx+1] + r.proxyPrefix + match[idx+1:]
	})
}

// rewriteCSSImports rewrites CSS @import statements, both direct and url() forms.
func (r *contentRewriter) rewriteCSSImports(result string) string {
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

	return result
}

// rewriteSVGHrefs rewrites SVG use/image href and xlink:href attributes.
func (r *contentRewriter) rewriteSVGHrefs(result string) string {
	svgHrefPattern := regexp.MustCompile(`(<(?:use|image)[^>]*(?:href|xlink:href)\s*=\s*["'])(/[^"'#]+)(#[^"']*)?(['"])`)
	return svgHrefPattern.ReplaceAllStringFunc(result, func(match string) string {
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

				// Set standard proxy forwarding headers
				setProxyHeaders(req)

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

// setProxyHeaders adds standard proxy forwarding headers (X-Forwarded-For,
// X-Forwarded-Host, X-Forwarded-Proto, X-Real-IP) to the request.
func setProxyHeaders(r *http.Request) {
	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	// X-Forwarded-For: Client IP (append to existing if present)
	if prior := r.Header.Get("X-Forwarded-For"); prior != "" {
		r.Header.Set("X-Forwarded-For", prior+", "+clientIP)
	} else {
		r.Header.Set("X-Forwarded-For", clientIP)
	}

	// X-Forwarded-Host: Original host requested by client
	if r.Header.Get("X-Forwarded-Host") == "" {
		r.Header.Set("X-Forwarded-Host", r.Host)
	}

	// X-Forwarded-Proto: Original protocol
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	if r.Header.Get("X-Forwarded-Proto") == "" {
		r.Header.Set("X-Forwarded-Proto", proto)
	}

	// X-Real-IP: Original client IP (commonly used by nginx)
	if r.Header.Get("X-Real-IP") == "" {
		r.Header.Set("X-Real-IP", clientIP)
	}
}

// dialBackend establishes a TCP connection (plain or TLS) to the backend.
func (route *proxyRoute) dialBackend() (net.Conn, error) {
	targetHost := route.targetURL.Host
	scheme := route.targetURL.Scheme

	if scheme == "https" {
		host := targetHost
		if h, _, splitErr := net.SplitHostPort(targetHost); splitErr == nil {
			host = h
		}
		return tls.Dial("tcp", targetHost, &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: true, //nolint:gosec // internal network backends
		})
	}

	// If no port in host, default to 80
	dialHost := targetHost
	if _, _, splitErr := net.SplitHostPort(targetHost); splitErr != nil {
		dialHost = targetHost + ":80"
	}
	return net.Dial("tcp", dialHost)
}

// buildUpgradeRequest constructs the raw HTTP upgrade request to send to the backend.
func (route *proxyRoute) buildUpgradeRequest(r *http.Request, backendPath, targetHost string) []byte {
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
	return reqBuf.Bytes()
}

// forwardUpgradeResponse writes the 101 Switching Protocols response to the client.
func (route *proxyRoute) forwardUpgradeResponse(clientConn net.Conn, resp *http.Response) error {
	var respBuf bytes.Buffer
	fmt.Fprintf(&respBuf, "HTTP/1.1 101 Switching Protocols\r\n")
	for k, vs := range resp.Header {
		for _, v := range vs {
			fmt.Fprintf(&respBuf, "%s: %s\r\n", k, v)
		}
	}
	respBuf.WriteString("\r\n")

	_, err := clientConn.Write(respBuf.Bytes())
	return err
}

// bridgeConnections performs bidirectional copy between the client and backend,
// first flushing any data already buffered in either reader.
func (route *proxyRoute) bridgeConnections(clientConn net.Conn, clientBuf *bufio.ReadWriter, backendConn net.Conn, backendBuf *bufio.Reader) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Backend → Client
	go func() {
		defer wg.Done()
		// First drain any data already buffered in the reader
		if backendBuf.Buffered() > 0 {
			buffered, _ := backendBuf.Peek(backendBuf.Buffered())
			_, _ = clientConn.Write(buffered)
			_, _ = backendBuf.Discard(len(buffered))
		}
		_, _ = io.Copy(clientConn, backendConn)
	}()

	// Client → Backend
	go func() {
		defer wg.Done()
		// Drain any buffered client data
		if clientBuf.Reader.Buffered() > 0 {
			buffered, _ := clientBuf.Peek(clientBuf.Reader.Buffered())
			_, _ = backendConn.Write(buffered)
			_, _ = clientBuf.Discard(len(buffered))
		}
		_, _ = io.Copy(backendConn, clientConn)
	}()

	wg.Wait()
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
	backendConn, err := route.dialBackend()
	if err != nil {
		log.Printf("[proxy-ws] %s: failed to dial backend %s: %v", route.name, targetHost, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer backendConn.Close()

	// Add proxy headers to the original request before forwarding
	setProxyHeaders(r)

	// Send upgrade request to backend
	upgradeReq := route.buildUpgradeRequest(r, backendPath, targetHost)
	if _, err = backendConn.Write(upgradeReq); err != nil {
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
			_, _ = io.Copy(w, resp.Body)
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
	if err = route.forwardUpgradeResponse(clientConn, resp); err != nil {
		log.Printf("[proxy-ws] %s: failed to write upgrade response to client: %v", route.name, err)
		return
	}

	// Bidirectional copy: pipe frames between client and backend.
	// If the backend's bufio reader has buffered data (e.g. a frame sent
	// immediately after the handshake), flush it to the client first.
	route.bridgeConnections(clientConn, clientBuf, backendConn, backendBuf)
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

package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// Pre-compiled regex patterns for response content rewriting.
// These are compiled once at package init, not per-request.
var (
	integrityPattern    = regexp.MustCompile(`\s*(integrity|crossorigin)\s*=\s*["'][^"']*["']`)
	dynamicSriPattern   = regexp.MustCompile(`\w+\.integrity\s*=\s*\w+\.sriHashes\[[^\]]+\],?`)
	sriHashesPattern    = regexp.MustCompile(`(\w+\.sriHashes)\s*=\s*\{[^}]+\}`)
	srcsetPattern       = regexp.MustCompile(`(srcset\s*=\s*["'])([^"']+)(["'])`)
	rootPathAttrPattern = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*\s*=\s*["'])/([a-zA-Z0-9_][^"']*)`)
	rootPathUrlPattern  = regexp.MustCompile(`(url\s*\(\s*["']?)/([a-zA-Z0-9_-][^"')]*["']?\s*\))`)
	baseHrefPattern     = regexp.MustCompile(`(<base[^>]*href\s*=\s*["'])([^"']*)(["'])`)
	urlBaseEmptyPattern = regexp.MustCompile(`("?)(urlBase|basePath|baseUrl|baseHref)("?)\s*[:=]\s*(['"])(['"])`)
	jsonPathPattern     = regexp.MustCompile(`("[\w]+"\s*:\s*")(/[^"/][^"]*)(")`)
	imageSetPattern     = regexp.MustCompile(`(image-set\s*\()([^)]+)(\))`)
	imageSetPathPattern = regexp.MustCompile(`(["'])/([a-zA-Z0-9_-][^"']*)(["'])`)
	jsonArrayPathPat    = regexp.MustCompile(`(\[|,)\s*"(/[^"]+)"`)
	cssImportPattern    = regexp.MustCompile(`(@import\s+["'])(/[^"']+)(["'])`)
	cssImportUrlPattern = regexp.MustCompile(`(@import\s+url\s*\(\s*["']?)(/[^"')]+)(["']?\s*\))`)
	svgHrefPattern      = regexp.MustCompile(`(<(?:use|image)[^>]*(?:href|xlink:href)\s*=\s*["'])(/[^"'#]+)(#[^"']*)?(['"])`)
	linkPathPattern     = regexp.MustCompile(`<(/[^>]+)>`)

	// Byte version of the proxy path prefix for zero-copy comparisons
	proxyPathPrefixB = []byte(proxyPathPrefix)

	// Content types that should be rewritten (static list)
	rewriteTypes = []string{
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
)

// ReverseProxyHandler handles reverse proxy requests on the main server
type ReverseProxyHandler struct {
	mu      sync.RWMutex
	routes  map[string]*proxyRoute
	timeout time.Duration
}

type proxyRoute struct {
	name          string
	slug          string
	proxyPrefix   string
	targetURL     *url.URL
	targetPath    string
	skipTLSVerify bool
	proxy         *httputil.ReverseProxy
	rewriter      *contentRewriter
}

// contentRewriter handles URL rewriting in response content
type contentRewriter struct {
	proxyPrefix string
	targetPath  string // e.g., "/admin" - gets stripped and replaced with proxyPrefix
	targetHost  string

	// Pre-allocated byte slices for zero-copy rewriting
	proxyPrefixB []byte
	targetPathB  []byte
	targetHostB  []byte

	// Pre-built replacement templates for non-callback regex operations
	sriHashRepl []byte // "${1}={}"
	urlBaseRepl []byte // ${1}${2}${3}: "proxyPrefix"
	attrRepl    []byte // "${1}" + proxyPrefix + "${2}"
	urlRepl     []byte // "${1}" + proxyPrefix + "${2}"
	jsRepl      []byte // "${1}" + proxyPrefix + "${2}${3}"

	// Pre-compiled patterns that depend on targetHost/targetPath
	hostPattern *regexp.Regexp // nil when targetHost is empty
	attrPattern *regexp.Regexp // nil when targetPath is empty
	urlPattern  *regexp.Regexp // nil when targetPath is empty
	jsPattern   *regexp.Regexp // nil when targetPath is empty
}

func newContentRewriter(proxyPrefix, targetPath, targetHost string) *contentRewriter {
	rw := &contentRewriter{
		proxyPrefix:  proxyPrefix,
		targetPath:   strings.TrimSuffix(targetPath, "/"),
		targetHost:   targetHost,
		proxyPrefixB: []byte(proxyPrefix),
		targetHostB:  []byte(targetHost),
		sriHashRepl:  []byte("${1}={}"),
		urlBaseRepl:  []byte(`${1}${2}${3}: "` + proxyPrefix + `"`),
	}
	rw.targetPathB = []byte(rw.targetPath)

	// Pre-compile dynamic patterns at route creation, not per-request
	if targetHost != "" {
		rw.hostPattern = regexp.MustCompile(`https?://` + regexp.QuoteMeta(targetHost) + `(/[^"'\s>)]*)`)
	}
	tp := rw.targetPath
	if tp != "" {
		escaped := regexp.QuoteMeta(tp)
		rw.attrPattern = regexp.MustCompile(`((?:href|src|action|poster|srcset|content|data-[a-zA-Z0-9-]+)\s*=\s*["'])` + escaped + `([/"']|[^"']*["'])`)
		rw.urlPattern = regexp.MustCompile(`(url\s*\(\s*["']?)` + escaped + `([^"')]*["']?\s*\))`)
		rw.jsPattern = regexp.MustCompile(`(["'])` + escaped + `(/[^"']*)(["'])`)
		rw.attrRepl = []byte("${1}" + proxyPrefix + "${2}")
		rw.urlRepl = []byte("${1}" + proxyPrefix + "${2}")
		rw.jsRepl = []byte("${1}" + proxyPrefix + "${2}${3}")
	}

	return rw
}

// spliceAt creates a new []byte with insert spliced at position pos.
func spliceAt(src []byte, pos int, insert []byte) []byte {
	out := make([]byte, len(src)+len(insert))
	copy(out, src[:pos])
	copy(out[pos:], insert)
	copy(out[pos+len(insert):], src[pos:])
	return out
}

func (r *contentRewriter) rewrite(content []byte) []byte {
	content = r.stripIntegrity(content)
	content = r.rewriteAbsoluteURLs(content)
	content = r.rewriteTargetPaths(content)
	content = r.rewriteSrcset(content)
	content = r.rewriteRootPaths(content)
	content = r.rewriteURLBase(content)
	content = r.rewriteJSONPaths(content)
	content = r.rewriteImageSet(content)
	content = r.rewriteJSONArrayPaths(content)
	content = r.rewriteCSSImports(content)
	content = r.rewriteSVGHrefs(content)
	return content
}

// rewriteScript performs URL rewriting for JavaScript content.
// It only applies safe rewrites (absolute URLs, target paths, SRI stripping,
// and base path config values). Root-relative path rewriting is skipped because
// the injected runtime interceptor handles those, and statically rewriting paths
// in JS source corrupts URLs meant for third-party servers (e.g. plex.tv).
func (r *contentRewriter) rewriteScript(content []byte) []byte {
	content = r.stripIntegrity(content)
	content = r.rewriteAbsoluteURLs(content)
	content = r.rewriteTargetPaths(content)
	content = r.rewriteURLBase(content)
	return content
}

// stripIntegrity removes integrity and crossorigin attributes since we modify
// content (breaks SRI hashes), and neutralises dynamic SRI in webpack loaders.
func (r *contentRewriter) stripIntegrity(result []byte) []byte {
	result = integrityPattern.ReplaceAll(result, nil)
	result = dynamicSriPattern.ReplaceAll(result, nil)
	result = sriHashesPattern.ReplaceAll(result, r.sriHashRepl)
	return result
}

// rewriteAbsoluteURLs rewrites absolute URLs containing the target host.
// e.g., http://192.0.2.100/admin/foo -> /proxy/slug/foo
func (r *contentRewriter) rewriteAbsoluteURLs(result []byte) []byte {
	if r.hostPattern == nil {
		return result
	}
	return r.hostPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		idx := bytes.Index(match, r.targetHostB)
		path := match[idx+len(r.targetHostB):]
		if len(r.targetPathB) > 0 && bytes.HasPrefix(path, r.targetPathB) {
			path = bytes.TrimPrefix(path, r.targetPathB)
		}
		if len(path) == 0 {
			path = []byte("/")
		}
		out := make([]byte, 0, len(r.proxyPrefixB)+len(path))
		out = append(out, r.proxyPrefixB...)
		out = append(out, path...)
		return out
	})
}

// rewriteTargetPaths rewrites paths that start with the target path.
// e.g., /admin/foo -> /proxy/slug/foo
func (r *contentRewriter) rewriteTargetPaths(result []byte) []byte {
	if r.attrPattern == nil {
		return result
	}

	// HTML attributes: href, src, action, data-*, poster, srcset, content (for meta refresh)
	result = r.attrPattern.ReplaceAll(result, r.attrRepl)
	// CSS url()
	result = r.urlPattern.ReplaceAll(result, r.urlRepl)
	// JavaScript string literals
	result = r.jsPattern.ReplaceAll(result, r.jsRepl)

	return result
}

// rewriteSrcset handles srcset attributes which contain multiple comma-separated paths.
// srcset="/sm.jpg 1x, /lg.jpg 2x" -> srcset="/proxy/app/sm.jpg 1x, /proxy/app/lg.jpg 2x"
func (r *contentRewriter) rewriteSrcset(result []byte) []byte {
	return srcsetPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		quoteStart := bytes.IndexAny(match, `"'`)
		if quoteStart == -1 {
			return match
		}
		quoteChar := match[quoteStart]
		quoteEnd := bytes.LastIndex(match, []byte{quoteChar})
		if quoteEnd <= quoteStart {
			return match
		}

		srcsetValue := match[quoteStart+1 : quoteEnd]
		parts := bytes.Split(srcsetValue, []byte(","))
		for i, part := range parts {
			trimmed := bytes.TrimSpace(part)
			if len(trimmed) > 0 && trimmed[0] == '/' && !bytes.HasPrefix(trimmed, proxyPathPrefixB) {
				var leading []byte
				if len(part) > 0 && part[0] == ' ' {
					leading = []byte(" ")
				}
				out := make([]byte, 0, len(leading)+len(r.proxyPrefixB)+len(trimmed))
				out = append(out, leading...)
				out = append(out, r.proxyPrefixB...)
				out = append(out, trimmed...)
				parts[i] = out
			}
		}
		out := make([]byte, 0, len(match)+len(r.proxyPrefixB)*len(parts))
		out = append(out, match[:quoteStart+1]...)
		out = append(out, bytes.Join(parts, []byte(","))...)
		out = append(out, match[quoteEnd:]...)
		return out
	})
}

// rewriteRootPaths rewrites root-relative paths (/) that don't start with /proxy/,
// including attribute values, CSS url(), and <base href="..."> tags.
func (r *contentRewriter) rewriteRootPaths(result []byte) []byte {
	result = r.rewriteRootPathAttrs(result)
	result = r.rewriteRootPathURLFunc(result)
	result = r.rewriteBaseHref(result)
	return result
}

// rewriteRootPathAttrs rewrites attribute values starting with / but not /proxy/,
// skipping srcset (handled separately).
func (r *contentRewriter) rewriteRootPathAttrs(result []byte) []byte {
	return rootPathAttrPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		if bytes.HasPrefix(bytes.ToLower(match), []byte("srcset")) {
			return match
		}
		quoteIdx := bytes.LastIndex(match, []byte(`"`))
		if quoteIdx == -1 {
			quoteIdx = bytes.LastIndex(match, []byte(`'`))
		}
		if quoteIdx == -1 {
			return match
		}
		return spliceAt(match, quoteIdx+1, r.proxyPrefixB)
	})
}

// rewriteRootPathURLFunc rewrites CSS url() values with root-relative paths.
func (r *contentRewriter) rewriteRootPathURLFunc(result []byte) []byte {
	return rootPathUrlPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		idx := bytes.IndexByte(match, '/')
		if idx == -1 {
			return match
		}
		return spliceAt(match, idx, r.proxyPrefixB)
	})
}

// rewriteBaseHref rewrites <base href="..."> tags to use the proxy prefix.
func (r *contentRewriter) rewriteBaseHref(result []byte) []byte {
	return baseHrefPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		startQuote := bytes.Index(match, []byte(`href`))
		if startQuote == -1 {
			return match
		}
		quoteStart := bytes.IndexAny(match[startQuote:], `"'`)
		if quoteStart == -1 {
			return match
		}
		quoteStart += startQuote
		quoteChar := match[quoteStart]
		quoteEnd := bytes.IndexByte(match[quoteStart+1:], quoteChar)
		if quoteEnd == -1 {
			return match
		}
		quoteEnd += quoteStart + 1

		href := match[quoteStart+1 : quoteEnd]

		var newHref []byte
		switch {
		case len(r.targetPathB) > 0 && bytes.HasPrefix(href, r.targetPathB):
			remainder := bytes.TrimPrefix(href, r.targetPathB)
			newHref = make([]byte, 0, len(r.proxyPrefixB)+len(remainder))
			newHref = append(newHref, r.proxyPrefixB...)
			newHref = append(newHref, remainder...)
		case len(href) > 0 && href[0] == '/' && !bytes.HasPrefix(href, proxyPathPrefixB):
			newHref = make([]byte, 0, len(r.proxyPrefixB)+len(href))
			newHref = append(newHref, r.proxyPrefixB...)
			newHref = append(newHref, href...)
		default:
			return match
		}

		out := make([]byte, 0, quoteStart+1+len(newHref)+len(match)-quoteEnd)
		out = append(out, match[:quoteStart+1]...)
		out = append(out, newHref...)
		out = append(out, match[quoteEnd:]...)
		return out
	})
}

// rewriteURLBase rewrites JavaScript/JSON base path patterns for SPAs (e.g., Sonarr/Radarr).
// Handles empty base path strings like urlBase or basePath set to blank values.
func (r *contentRewriter) rewriteURLBase(result []byte) []byte {
	return urlBaseEmptyPattern.ReplaceAll(result, r.urlBaseRepl)
}

// rewriteJSONPaths rewrites generic JSON paths: any "key": "/path" where path
// doesn't start with /proxy/. Handles apiRoot, basePath, redirectUrl, etc.
func (r *contentRewriter) rewriteJSONPaths(result []byte) []byte {
	marker := []byte(`"/`)
	return jsonPathPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		idx := bytes.Index(match, marker)
		if idx == -1 {
			return match
		}
		return spliceAt(match, idx+1, r.proxyPrefixB)
	})
}

// rewriteImageSet rewrites CSS image-set() functions. Must run before JSON array
// handler because `, "/2x.png"` inside image-set() would otherwise match.
func (r *contentRewriter) rewriteImageSet(result []byte) []byte {
	return imageSetPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		return imageSetPathPattern.ReplaceAllFunc(match, func(inner []byte) []byte {
			if bytes.Contains(inner, proxyPathPrefixB) {
				return inner
			}
			q := inner[0]
			// q + proxyPrefix + "/" + inner[2:len-1] + q
			out := make([]byte, 0, 2+len(r.proxyPrefixB)+1+len(inner)-3)
			out = append(out, q)
			out = append(out, r.proxyPrefixB...)
			out = append(out, '/')
			out = append(out, inner[2:len(inner)-1]...)
			out = append(out, q)
			return out
		})
	})
}

// rewriteJSONArrayPaths rewrites JSON arrays of paths: ["/path1", "/path2"]
func (r *contentRewriter) rewriteJSONArrayPaths(result []byte) []byte {
	marker := []byte(`"/`)
	return jsonArrayPathPat.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		idx := bytes.Index(match, marker)
		if idx == -1 {
			return match
		}
		return spliceAt(match, idx+1, r.proxyPrefixB)
	})
}

// rewriteCSSImports rewrites CSS @import statements, both direct and url() forms.
func (r *contentRewriter) rewriteCSSImports(result []byte) []byte {
	dqSlash := []byte(`"/`)
	sqSlash := []byte(`'/`)
	parenSlash := []byte(`(/`)

	// @import "/styles.css" or @import '/styles.css'
	result = cssImportPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		idx := bytes.Index(match, dqSlash)
		if idx == -1 {
			idx = bytes.Index(match, sqSlash)
		}
		if idx == -1 {
			return match
		}
		return spliceAt(match, idx+1, r.proxyPrefixB)
	})

	// @import url("/styles.css") or @import url('/styles.css')
	result = cssImportUrlPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		idx := bytes.Index(match, parenSlash)
		if idx == -1 {
			return match
		}
		return spliceAt(match, idx+1, r.proxyPrefixB)
	})

	return result
}

// rewriteSVGHrefs rewrites SVG use/image href and xlink:href attributes.
func (r *contentRewriter) rewriteSVGHrefs(result []byte) []byte {
	dqSlash := []byte(`"/`)
	sqSlash := []byte(`'/`)
	return svgHrefPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		idx := bytes.Index(match, dqSlash)
		if idx == -1 {
			idx = bytes.Index(match, sqSlash)
		}
		if idx == -1 {
			return match
		}
		return spliceAt(match, idx+1, r.proxyPrefixB)
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

// NewReverseProxyHandler creates a new reverse proxy handler.
// proxyTimeout is the global timeout for proxied HTTP requests (e.g. "30s").
func NewReverseProxyHandler(apps []config.AppConfig, proxyTimeout string) *ReverseProxyHandler {
	timeout, err := time.ParseDuration(proxyTimeout)
	if err != nil || timeout <= 0 {
		timeout = 30 * time.Second
	}

	h := &ReverseProxyHandler{
		routes:  make(map[string]*proxyRoute),
		timeout: timeout,
	}
	h.RebuildRoutes(apps)
	return h
}

// RebuildRoutes rebuilds proxy routes from the current app config.
// This is called after config changes to pick up new/changed/removed proxy apps.
func (h *ReverseProxyHandler) RebuildRoutes(apps []config.AppConfig) {
	newRoutes := make(map[string]*proxyRoute)
	for i := range apps {
		if !apps[i].Proxy || !apps[i].Enabled {
			continue
		}
		route := buildSingleProxyRoute(&apps[i], h.timeout)
		if route != nil {
			newRoutes[route.slug] = route
		}
	}

	h.mu.Lock()
	h.routes = newRoutes
	h.mu.Unlock()

	slugs := make([]string, 0, len(newRoutes))
	for slug := range newRoutes {
		slugs = append(slugs, slug)
	}
	logging.Info("Proxy routes rebuilt", "source", "proxy", "routes", slugs)
}

// buildSingleProxyRoute creates a proxyRoute for a single app config.
// Returns nil if the app URL is invalid or already uses a proxy path.
func buildSingleProxyRoute(app *config.AppConfig, timeout time.Duration) *proxyRoute {
	targetURL, err := url.Parse(app.URL)
	if err != nil {
		return nil
	}

	// Skip apps that already use a proxy path (to avoid loops)
	if strings.HasPrefix(app.URL, proxyPathPrefix) {
		return nil
	}

	slug := Slugify(app.Name)
	proxyPrefix := proxyPathPrefix + slug
	targetPath := targetURL.Path
	if targetPath == "" {
		targetPath = "/"
	}

	// Per-app TLS verification: nil (default) = skip, explicit false = verify
	skipTLS := app.ProxySkipTLSVerify == nil || *app.ProxySkipTLSVerify

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLS, //nolint:gosec // configurable per-app
		},
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    true, // we handle content encoding in rewriteResponseBody
	}

	rewriter := newContentRewriter(proxyPrefix, targetPath, targetURL.Host)

	proxy := &httputil.ReverseProxy{
		Director:       buildDirector(proxyPrefix, targetPath, targetURL, app.ProxyHeaders),
		ModifyResponse: createModifyResponse(proxyPrefix, targetPath, rewriter), //nolint:bodyclose // response body managed by httputil.ReverseProxy
		Transport:      transport,
	}

	return &proxyRoute{
		name:          app.Name,
		slug:          slug,
		proxyPrefix:   proxyPrefix,
		targetURL:     targetURL,
		targetPath:    targetPath,
		skipTLSVerify: skipTLS,
		proxy:         proxy,
		rewriter:      rewriter,
	}
}

// buildDirector creates the Director function for a reverse proxy that rewrites
// incoming request paths from the proxy prefix to the backend target.
// customHeaders are injected into every proxied request.
func buildDirector(proxyPrefix, targetPath string, targetURL *url.URL, customHeaders map[string]string) func(*http.Request) {
	return func(req *http.Request) {
		// Strip the /proxy/{slug} prefix from the request path
		reqPath := strings.TrimPrefix(req.URL.Path, proxyPrefix)
		if reqPath == "" {
			reqPath = "/"
		}

		// Handle double-prefixing caused by SPAs that construct URLs with urlBase + endpoint
		if strings.Contains(reqPath, proxyPrefix) {
			reqPath = strings.ReplaceAll(reqPath, proxyPrefix, "")
		}

		req.URL.Path = resolveBackendRequestPath(reqPath, targetPath)
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host

		setProxyHeaders(req)

		// Inject per-app custom headers (e.g., Authorization, X-Api-Key)
		for k, v := range customHeaders {
			req.Header.Set(k, v)
		}

		req.Host = targetURL.Host
		req.Header.Set("Accept-Encoding", "gzip, identity")
	}
}

// resolveBackendRequestPath joins the request path with the target path,
// bypassing the target path prefix for /api paths.
func resolveBackendRequestPath(reqPath, targetPath string) string {
	trimmedTargetPath := strings.TrimSuffix(targetPath, "/")
	if trimmedTargetPath == "" {
		return reqPath
	}

	// API paths typically live at root, not under the target path
	if strings.HasPrefix(reqPath, "/api/") || reqPath == "/api" {
		return reqPath
	}
	if strings.HasPrefix(reqPath, "/") {
		return trimmedTargetPath + reqPath
	}
	return trimmedTargetPath + "/" + reqPath
}

func createModifyResponse(proxyPrefix, targetPath string, rewriter *contentRewriter) func(*http.Response) error {
	return func(resp *http.Response) error {
		// Remove headers that prevent iframe embedding
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")

		rewriteLocationHeaders(resp, proxyPrefix, targetPath, rewriter.targetHost)
		rewriteCookieHeaders(resp, rewriter)
		rewriteLinkHeaders(resp, proxyPrefix)

		return rewriteResponseBody(resp, rewriter)
	}
}

// rewriteLocationHeaders rewrites Location, Content-Location, and Refresh headers
// so that redirects point through the proxy path.
func rewriteLocationHeaders(resp *http.Response, proxyPrefix, targetPath, targetHost string) {
	if location := resp.Header.Get("Location"); location != "" {
		location = rewriteLocation(location, proxyPrefix, targetPath, targetHost)
		resp.Header.Set("Location", location)
	}

	if contentLoc := resp.Header.Get("Content-Location"); contentLoc != "" {
		contentLoc = rewriteLocation(contentLoc, proxyPrefix, targetPath, targetHost)
		resp.Header.Set("Content-Location", contentLoc)
	}

	if refresh := resp.Header.Get("Refresh"); refresh != "" {
		if idx := strings.Index(strings.ToLower(refresh), "url="); idx != -1 {
			urlPart := strings.TrimSpace(refresh[idx+4:])
			urlPart = rewriteLocation(urlPart, proxyPrefix, targetPath, targetHost)
			resp.Header.Set("Refresh", refresh[:idx+4]+urlPart)
		}
	}
}

// rewriteCookieHeaders rewrites Set-Cookie Path attributes to use the proxy prefix.
func rewriteCookieHeaders(resp *http.Response, rewriter *contentRewriter) {
	cookies := resp.Header.Values(headerSetCookie)
	if len(cookies) == 0 {
		return
	}
	resp.Header.Del(headerSetCookie)
	for _, cookie := range cookies {
		rewritten := rewriter.rewriteCookiePath(cookie)
		resp.Header.Add(headerSetCookie, rewritten)
	}
}

// rewriteLinkHeaders rewrites Link headers (preload, prefetch) to use the proxy prefix.
func rewriteLinkHeaders(resp *http.Response, proxyPrefix string) {
	linkHeaders := resp.Header.Values("Link")
	if len(linkHeaders) == 0 {
		return
	}
	resp.Header.Del("Link")
	for _, link := range linkHeaders {
		rewritten := linkPathPattern.ReplaceAllStringFunc(link, func(match string) string {
			if strings.Contains(match, proxyPathPrefix) {
				return match
			}
			path := match[1 : len(match)-1]
			return "<" + proxyPrefix + path + ">"
		})
		resp.Header.Add("Link", rewritten)
	}
}

// interceptorScript returns a <script> tag that patches fetch, XMLHttpRequest,
// WebSocket, EventSource, and DOM element property setters to rewrite root-relative
// URLs through the proxy prefix. Property setters (img.src, etc.) provide synchronous
// rewriting so the browser never sees the wrong URL, preserving normal event chains
// and animations. A MutationObserver serves as fallback for innerHTML/parser cases.
func (r *contentRewriter) interceptorScript() []byte {
	// The proxy prefix is derived from slugified app names (alphanumeric + hyphens),
	// so it's safe to embed directly in a JavaScript string literal.
	return []byte(`<script data-muximux-proxy>(function(){` +
		`var P="` + r.proxyPrefix + `";` +
		// R(u) rewrites root-relative and same-origin absolute URLs to go through the proxy
		`function R(u){` +
		`if(typeof u!=="string")return u;` +
		`if(u[0]==="/"&&!u.startsWith(P+"/")&&u!==P)return P+u;` +
		`try{var p=new URL(u);if(p.host===location.host&&!p.pathname.startsWith(P+"/")&&p.pathname!==P){p.pathname=P+p.pathname;return p.href}}catch(e){}` +
		`return u}` +
		// Patch fetch()
		`var _F=window.fetch;` +
		`window.fetch=function(i,o){` +
		`if(typeof i==="string")i=R(i);` +
		`else if(i instanceof Request){var n=R(i.url);if(n!==i.url)i=new Request(n,i)}` +
		`return _F.call(this,i,o)};` +
		// Patch XMLHttpRequest.open()
		`var _X=XMLHttpRequest.prototype.open;` +
		`XMLHttpRequest.prototype.open=function(){var a=[].slice.call(arguments);a[1]=R(a[1]);return _X.apply(this,a)};` +
		// Patch WebSocket constructor
		`var _W=window.WebSocket;` +
		`window.WebSocket=function(u,p){return p!==void 0?new _W(R(u),p):new _W(R(u))};` +
		`window.WebSocket.prototype=_W.prototype;` +
		`window.WebSocket.CONNECTING=_W.CONNECTING;window.WebSocket.OPEN=_W.OPEN;` +
		`window.WebSocket.CLOSING=_W.CLOSING;window.WebSocket.CLOSED=_W.CLOSED;` +
		// Patch EventSource constructor
		`var _E=window.EventSource;` +
		`if(_E){window.EventSource=function(u,c){return new _E(R(u),c)};` +
		`window.EventSource.prototype=_E.prototype;` +
		`window.EventSource.CONNECTING=_E.CONNECTING;window.EventSource.OPEN=_E.OPEN;window.EventSource.CLOSED=_E.CLOSED}` +
		// Property setter overrides for synchronous URL rewriting on DOM elements.
		// When an SPA sets img.src = "/photo/...", the setter intercepts it and
		// rewrites the URL BEFORE the browser starts loading, preserving the normal
		// load event chain and any animations (e.g. opacity fade-in).
		`function W(C,a){` +
		`var d=Object.getOwnPropertyDescriptor(C.prototype,a);` +
		`if(!d||!d.set)return;` +
		`Object.defineProperty(C.prototype,a,{get:d.get,set:function(v){d.set.call(this,R(v))},enumerable:d.enumerable,configurable:d.configurable})}` +
		`W(HTMLImageElement,"src");W(HTMLScriptElement,"src");W(HTMLSourceElement,"src");W(HTMLMediaElement,"src");W(HTMLVideoElement,"poster");` +
		// MutationObserver as fallback for elements created via innerHTML/parser
		// where property setters don't fire. Only rewrites if URL isn't already prefixed.
		`var urlAttrs={"src":1,"poster":1};` +
		`function fixAttr(el,a){var v=el.getAttribute(a);if(v){var n=R(v);if(n!==v)el.setAttribute(a,n)}}` +
		`function fixEl(el){` +
		`if(el.nodeType!==1)return;` +
		`for(var a in urlAttrs){if(el.hasAttribute&&el.hasAttribute(a))fixAttr(el,a)}` +
		`var ch=el.querySelectorAll("[src],[poster]");` +
		`for(var i=0;i<ch.length;i++){for(var a in urlAttrs){if(ch[i].hasAttribute(a))fixAttr(ch[i],a)}}}` +
		`new MutationObserver(function(muts){` +
		`for(var i=0;i<muts.length;i++){var m=muts[i];` +
		`if(m.type==="childList"){for(var j=0;j<m.addedNodes.length;j++)fixEl(m.addedNodes[j])}` +
		`else if(m.type==="attributes"&&urlAttrs[m.attributeName]){fixAttr(m.target,m.attributeName)}}` +
		`}).observe(document,{childList:true,subtree:true,attributes:true,attributeFilter:["src","poster"]});` +
		// Chrome may freeze document.timeline in iframes, leaving Web Animations
		// (like Plex's opacity fade-in) stuck indefinitely. Periodic scan detects
		// loaded images with opacity stuck at 0, cancels their frozen animations,
		// and forces them visible. Self-disables after 30s to avoid unnecessary work.
		`var fixT=0;` +
		`var fixI=setInterval(function(){` +
		`fixT+=200;` +
		`var imgs=document.querySelectorAll("img");` +
		`for(var i=0;i<imgs.length;i++){var t=imgs[i];` +
		`if(t.complete&&t.naturalWidth>0&&t.style.opacity==="0"){` +
		`if(t.getAnimations){var a=t.getAnimations();for(var j=0;j<a.length;j++)a[j].cancel()}` +
		`t.style.opacity="1"}}` +
		`if(fixT>=30000)clearInterval(fixI)` +
		`},200);` +
		`})()</script>`)
}

// injectInterceptor injects the runtime URL interceptor script into an HTML document,
// right after the opening <head> tag so it runs before any other scripts.
func (r *contentRewriter) injectInterceptor(content []byte) []byte {
	lower := bytes.ToLower(content)
	headIdx := bytes.Index(lower, []byte("<head"))
	if headIdx == -1 {
		return content
	}
	closeIdx := bytes.IndexByte(content[headIdx:], '>')
	if closeIdx == -1 {
		return content
	}
	insertPos := headIdx + closeIdx + 1

	script := r.interceptorScript()
	result := make([]byte, 0, len(content)+len(script))
	result = append(result, content[:insertPos]...)
	result = append(result, script...)
	result = append(result, content[insertPos:]...)
	return result
}

// maxRewriteSize is the maximum response body size (50 MB) that will be buffered
// for URL rewriting. Text responses larger than this stream through unmodified to
// avoid excessive memory use. In practice, HTML/CSS/JS are rarely this large.
const maxRewriteSize = 50 * 1024 * 1024

// rewriteResponseBody reads, decompresses (if gzipped), rewrites, and replaces the
// response body for content types that need URL rewriting (HTML, CSS, JS, JSON, XML).
// Binary content types and responses exceeding maxRewriteSize are streamed through
// without buffering.
func rewriteResponseBody(resp *http.Response, rewriter *contentRewriter) error {
	contentType := resp.Header.Get(headerContentType)
	if !shouldRewriteContent(contentType) {
		return nil
	}

	// Skip rewriting for responses that declare a size larger than the limit.
	if resp.ContentLength > maxRewriteSize {
		return nil
	}

	var reader io.Reader = resp.Body
	isGzipped := strings.Contains(resp.Header.Get(headerContentEncoding), "gzip")

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

	// Safety net for chunked responses without Content-Length: if the body
	// exceeds the rewrite limit, return it unmodified rather than rewriting.
	if int64(len(body)) > maxRewriteSize {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		resp.Header.Del(headerContentEncoding)
		return nil
	}

	lowerContentType := strings.ToLower(contentType)
	isHTML := strings.Contains(lowerContentType, "text/html")

	// Only HTML and CSS need full static path rewriting — they're rendered directly
	// by the browser. All other content types (JS, JSON, XML) are consumed
	// programmatically by the SPA; rewriting paths in API data causes double-prefixing
	// when the SPA embeds those paths in new URLs (e.g. Plex photo transcode).
	// The runtime interceptor handles outgoing requests for those content types.
	var rewritten []byte
	if isHTML || strings.Contains(lowerContentType, "text/css") {
		rewritten = rewriter.rewrite(body)
	} else {
		rewritten = rewriter.rewriteScript(body)
	}

	// Inject runtime URL interceptor for HTML responses so SPAs that construct
	// API URLs dynamically (fetch, XHR, WebSocket) route through the proxy.
	if isHTML {
		rewritten = rewriter.injectInterceptor(rewritten)
	}

	resp.Body = io.NopCloser(bytes.NewReader(rewritten))
	resp.ContentLength = int64(len(rewritten))
	resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	resp.Header.Del(headerContentEncoding)

	return nil
}

func rewriteLocation(location, proxyPrefix, targetPath, targetHost string) string {
	// Handle absolute URLs pointing to the target server
	// e.g., http://192.0.2.10:32400/web/index.html -> /proxy/plex/index.html
	location = resolveAbsoluteLocation(location, targetHost)

	// Skip if already rewritten or not a path
	if !strings.HasPrefix(location, "/") || strings.HasPrefix(location, proxyPathPrefix) {
		return location
	}

	return rewritePathWithTarget(location, proxyPrefix, targetPath)
}

// resolveAbsoluteLocation converts an absolute URL to a path if it points to the target host.
// Returns the original location unchanged if it's not an absolute URL or points to a different host.
func resolveAbsoluteLocation(location, targetHost string) string {
	if !strings.HasPrefix(location, "http://") && !strings.HasPrefix(location, "https://") {
		return location
	}
	parsed, err := url.Parse(location)
	if err != nil {
		return location
	}
	if parsed.Host != targetHost {
		return location
	}
	result := parsed.Path
	if parsed.RawQuery != "" {
		result += "?" + parsed.RawQuery
	}
	return result
}

// rewritePathWithTarget rewrites a root-relative path, stripping the target path prefix
// if present and prepending the proxy prefix.
func rewritePathWithTarget(location, proxyPrefix, targetPath string) string {
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

	if strings.Contains(path, route.proxyPrefix) {
		path = strings.ReplaceAll(path, route.proxyPrefix, "")
	}

	return resolveBackendRequestPath(path, route.targetPath)
}

// setProxyHeaders adds standard proxy forwarding headers (X-Forwarded-For,
// X-Forwarded-Host, X-Forwarded-Proto, X-Real-IP) to the request.
func setProxyHeaders(r *http.Request) {
	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	// X-Forwarded-For: Client IP (append to existing if present)
	if prior := r.Header.Get(headerXForwardedFor); prior != "" {
		r.Header.Set(headerXForwardedFor, prior+", "+clientIP)
	} else {
		r.Header.Set(headerXForwardedFor, clientIP)
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
			InsecureSkipVerify: route.skipTLSVerify, //nolint:gosec // configurable per-app
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
		logging.Error("Failed to dial backend", "source", "proxy", "app", route.name, "target", targetHost, "error", err)
		http.Error(w, errBadGateway, http.StatusBadGateway)
		return
	}
	defer backendConn.Close()

	// Add proxy headers to the original request before forwarding
	setProxyHeaders(r)

	// Send upgrade request to backend
	upgradeReq := route.buildUpgradeRequest(r, backendPath, targetHost)
	if _, err = backendConn.Write(upgradeReq); err != nil {
		logging.Error("Failed to write upgrade request", "source", "proxy", "app", route.name, "error", err)
		http.Error(w, errBadGateway, http.StatusBadGateway)
		return
	}

	// Read the backend's response
	backendBuf := bufio.NewReader(backendConn)
	resp, err := http.ReadResponse(backendBuf, r)
	if err != nil {
		logging.Error("Failed to read upgrade response", "source", "proxy", "app", route.name, "error", err)
		http.Error(w, errBadGateway, http.StatusBadGateway)
		return
	}

	// If backend didn't upgrade, forward the error response as-is
	if resp.StatusCode != http.StatusSwitchingProtocols {
		logging.Warn("Backend did not upgrade to WebSocket", "source", "proxy", "app", route.name, "status_code", resp.StatusCode)
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
		logging.Error("Response writer does not support hijacking", "source", "proxy", "app", route.name)
		http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		logging.Error("Failed to hijack client connection", "source", "proxy", "app", route.name, "error", err)
		return
	}
	defer clientConn.Close()

	// Forward the 101 response to the client
	if err = route.forwardUpgradeResponse(clientConn, resp); err != nil {
		logging.Error("Failed to write upgrade response to client", "source", "proxy", "app", route.name, "error", err)
		return
	}

	// Bidirectional copy: pipe frames between client and backend.
	// If the backend's bufio reader has buffered data (e.g. a frame sent
	// immediately after the handshake), flush it to the client first.
	route.bridgeConnections(clientConn, clientBuf, backendConn, backendBuf)
}

// ServeHTTP handles proxy requests
func (h *ReverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, proxyPathPrefix)
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		http.Error(w, "Invalid proxy path", http.StatusBadRequest)
		return
	}

	slug := parts[0]

	h.mu.RLock()
	route, exists := h.routes[slug]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, "App not found: "+slug, http.StatusNotFound)
		return
	}

	logging.Debug("Proxying request", "source", "proxy", "app", slug, "method", r.Method, "path", r.URL.Path)

	// WebSocket upgrade requests use hijack-based proxying
	if isWebSocketUpgrade(r) {
		logging.Debug("WebSocket upgrade detected", "source", "proxy", "app", slug)
		route.handleWebSocket(w, r)
		return
	}

	route.proxy.ServeHTTP(w, r)
}

// HasRoutes returns true if there are any proxy routes configured
func (h *ReverseProxyHandler) HasRoutes() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.routes) > 0
}

// GetRoutes returns a list of configured proxy slugs
func (h *ReverseProxyHandler) GetRoutes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	routes := make([]string, 0, len(h.routes))
	for slug := range h.routes {
		routes = append(routes, slug)
	}
	return routes
}

// Slugify is defined in api.go

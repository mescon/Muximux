package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"html"
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

	"github.com/mescon/muximux/v3/internal/auth"
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
	urlBaseEmptyPattern = regexp.MustCompile(`("?)(urlBase|basePath|baseUrl|baseHref)("?)\s*([:=])\s*(['"])(['"])`)
	imageSetPattern     = regexp.MustCompile(`(image-set\s*\()([^)]+)(\))`)
	imageSetPathPattern = regexp.MustCompile(`(["'])/([a-zA-Z0-9_-][^"']*)(["'])`)
	cssImportPattern    = regexp.MustCompile(`(@import\s+["'])(/[^"']+)(["'])`)
	cssImportUrlPattern = regexp.MustCompile(`(@import\s+url\s*\(\s*["']?)(/[^"')]+)(["']?\s*\))`)
	svgHrefPattern      = regexp.MustCompile(`(<(?:use|image)[^>]*(?:href|xlink:href)\s*=\s*["'])(/[^"'#]+)(#[^"']*)?(['"])`)
	// Meta refresh: <meta http-equiv="refresh" content="5;url=/path">
	metaRefreshPattern = regexp.MustCompile(`(?i)(content\s*=\s*["'][^"']*;\s*url\s*=\s*['"]?)(/[^"'\s>]+)`)
	// Meta CSP: <meta http-equiv="Content-Security-Policy" content="...">
	// Stripped so the injected interceptor script is not blocked by nonce requirements.
	metaCSPPattern = regexp.MustCompile(`(?i)<meta\s[^>]*http-equiv\s*=\s*["']Content-Security-Policy(?:-Report-Only)?["'][^>]*/?\s*>`)

	// ES module import/export patterns for rewriting module specifiers.
	// Dynamic import(): import('/path') — browser module loader, not interceptable by fetch patches.
	esDynImportPattern = regexp.MustCompile(`(import\s*\(\s*['"])(/[^"']+)(['"])`)
	// Static import/export with from: import { x } from '/path' or export * from '/path'
	esStaticImportPattern = regexp.MustCompile(`((?:import|export)\b[^;]*?\bfrom\s*['"])(/[^"']+)(['"])`)
	// Side-effect import: import '/polyfill.js'
	esSideEffectPattern = regexp.MustCompile(`(\bimport\s+['"])(/[^"']+)(['"])`)
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

// ReverseProxyOptions holds optional configuration for NewReverseProxyHandler.
// It exists as a struct so additional options can be added without breaking
// the handler's constructor signature.
type ReverseProxyOptions struct {
	// SessionCookieName, when set, is stripped from outgoing Cookie headers
	// on every proxied request (HTTP and WebSocket). Prevents leaking the
	// Muximux session identifier to backend operators.
	SessionCookieName string
}

// ReverseProxyHandler handles reverse proxy requests on the main server
type ReverseProxyHandler struct {
	mu                sync.RWMutex
	routes            map[string]*proxyRoute
	timeout           time.Duration
	sessionCookieName string
}

type proxyRoute struct {
	name              string
	slug              string
	proxyPrefix       string
	targetURL         *url.URL
	targetPath        string
	skipTLSVerify     bool
	timeout           time.Duration
	proxy             *httputil.ReverseProxy
	rewriter          *contentRewriter
	customHeaders     map[string]string
	sessionCookieName string

	// Access controls mirrored from the source AppConfig. ServeHTTP
	// enforces both before forwarding so a non-admin user who has
	// guessed (or learned) the slug of an admin-only app cannot
	// reach it via /proxy/{slug}/. The pre-Phase-6 build only
	// filtered apps in sanitizeApps, which gates the UI but not the
	// data path - the actual proxy was wide open. (codebase review C1)
	minRole       string
	allowedGroups []string
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
	urlBaseRepl []byte // ${1}${2}${3}${4} "proxyPrefix"
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
		urlBaseRepl:  []byte(`${1}${2}${3}${4} "` + proxyPrefix + `"`),
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
	content = r.stripMetaCSP(content)
	content = r.stripIntegrity(content)
	content = r.rewriteAbsoluteURLs(content)
	content = r.rewriteTargetPaths(content)
	content = r.rewriteSrcset(content)
	content = r.rewriteRootPaths(content)
	content = r.rewriteURLBase(content)
	// JSON path rewriting ("key": "/path") is intentionally NOT applied to HTML.
	// SSR frameworks (Nuxt 3, Next.js, SvelteKit) embed route/data payloads in
	// inline <script> tags. Rewriting paths in these payloads corrupts hydration
	// state and causes the client-side router to navigate to non-existent routes
	// (e.g. "/proxy/slug/recipe/123" instead of "/recipe/123") → 404.
	// The runtime interceptor handles outgoing API calls from these paths.
	content = r.rewriteImageSet(content)
	content = r.rewriteCSSImports(content)
	content = r.rewriteSVGHrefs(content)
	content = r.rewriteMetaRefresh(content)
	content = r.rewriteModuleImports(content)
	return content
}

// rewriteScript performs URL rewriting for JavaScript/JSON content.
// It only applies safe rewrites (absolute URLs, target paths, SRI stripping,
// base path config values, and ES module imports). General root-relative path
// rewriting is skipped because the injected runtime interceptor handles those,
// and statically rewriting paths in JS source corrupts URLs meant for
// third-party servers (e.g. plex.tv). ES module imports are an exception
// because the browser's module loader bypasses fetch/XHR interception.
func (r *contentRewriter) rewriteScript(content []byte) []byte {
	content = r.stripDynamicSRI(content)
	content = r.rewriteAbsoluteURLs(content)
	content = r.rewriteTargetPaths(content)
	content = r.rewriteURLBase(content)
	content = r.rewriteModuleImports(content)
	return content
}

// stripIntegrity removes HTML integrity/crossorigin attributes (we modify
// content, which breaks SRI hashes) and neutralises dynamic SRI in webpack
// loaders. HTML only -- the attribute pattern is unanchored and false-matches
// these words inside minified JS string literals, so the JS path uses
// stripDynamicSRI instead.
func (r *contentRewriter) stripIntegrity(result []byte) []byte {
	result = integrityPattern.ReplaceAll(result, nil)
	return r.stripDynamicSRI(result)
}

// stripDynamicSRI neutralises webpack's runtime SRI (the dynamic
// `x.integrity = y.sriHashes[z]` assignment and the `sriHashes` table) without
// touching HTML integrity/crossorigin attributes. The attribute form is an
// HTML construct; in JavaScript those words only appear inside string literals
// (e.g. Bazarr builds the CSS selector `'[crossorigin="' + a + '"]'`), where
// stripping them corrupts the script. Safe for JS and JSON. (#371)
func (r *contentRewriter) stripDynamicSRI(result []byte) []byte {
	result = dynamicSriPattern.ReplaceAll(result, nil)
	result = sriHashesPattern.ReplaceAll(result, r.sriHashRepl)
	return result
}

// stripMetaCSP removes <meta http-equiv="Content-Security-Policy" ...> tags.
// The proxy already strips the CSP response header (to allow iframe embedding),
// but some apps (Nuxt/Mealie) embed CSP in a meta tag with a nonce. This blocks
// the injected interceptor script, so we strip these meta tags too.
func (r *contentRewriter) stripMetaCSP(result []byte) []byte {
	return metaCSPPattern.ReplaceAll(result, nil)
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
// Skips matches that are part of a larger identifier (e.g. _baseHref,
// this._baseHref) by inspecting the byte just before each match.
//
// Uses FindAllSubmatchIndex so we get match positions for free,
// avoiding a per-match bytes.Index full-body scan that turned the
// rewrite O(n*m) on bundles with many matches.
func (r *contentRewriter) rewriteURLBase(result []byte) []byte {
	indices := urlBaseEmptyPattern.FindAllSubmatchIndex(result, -1)
	if len(indices) == 0 {
		return result
	}
	out := make([]byte, 0, len(result))
	lastEnd := 0
	for _, idx := range indices {
		start, end := idx[0], idx[1]
		// Identifier-boundary check: if the byte immediately before
		// the match is a word char or dot, the matched name is the
		// tail of a longer identifier and must NOT be rewritten.
		if start > 0 {
			prev := result[start-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
				(prev >= '0' && prev <= '9') || prev == '_' || prev == '.' {
				out = append(out, result[lastEnd:end]...)
				lastEnd = end
				continue
			}
		}
		out = append(out, result[lastEnd:start]...)
		// ReplaceAll on the match slice expands $1..$6 against the
		// regex's submatches in that slice. Cheaper than rebuilding
		// the replacement by hand since the pattern is fixed.
		out = append(out, urlBaseEmptyPattern.ReplaceAll(result[start:end], r.urlBaseRepl)...)
		lastEnd = end
	}
	out = append(out, result[lastEnd:]...)
	return out
}

// rewriteImageSet rewrites CSS image-set() functions.
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

// rewriteModuleImports rewrites ES module import/export specifiers so the
// browser's module loader fetches chunks through the proxy. Unlike fetch/XHR,
// module imports (static and dynamic) use the browser's internal loader which
// cannot be intercepted by the runtime script. Without this rewrite, code-split
// chunks (e.g. Nuxt lazy routes) would be fetched from the wrong origin.
func (r *contentRewriter) rewriteModuleImports(content []byte) []byte {
	dqSlash := []byte(`"/`)
	sqSlash := []byte(`'/`)

	rewriteImportPath := func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		// Find the quoted root-relative path ("/ or '/) and splice in the proxy prefix
		idx := bytes.LastIndex(match, dqSlash)
		if idx == -1 {
			idx = bytes.LastIndex(match, sqSlash)
		}
		if idx == -1 {
			return match
		}
		return spliceAt(match, idx+1, r.proxyPrefixB)
	}

	// Static: import { x } from '/path', export * from '/path'
	content = esStaticImportPattern.ReplaceAllFunc(content, rewriteImportPath)
	// Side-effect: import '/polyfill.js'
	content = esSideEffectPattern.ReplaceAllFunc(content, rewriteImportPath)
	// Dynamic: import('/chunk.js')
	content = esDynImportPattern.ReplaceAllFunc(content, rewriteImportPath)
	return content
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

// rewriteMetaRefresh rewrites URLs in <meta http-equiv="refresh" content="5;url=/path">.
// The Refresh response header is handled separately by rewriteLocationHeaders; this
// catches the equivalent <meta> tag in HTML where the URL is embedded in the content
// attribute value after ";url=".
func (r *contentRewriter) rewriteMetaRefresh(result []byte) []byte {
	return metaRefreshPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		if bytes.Contains(match, proxyPathPrefixB) {
			return match
		}
		lowerMatch := bytes.ToLower(match)
		urlIdx := bytes.Index(lowerMatch, []byte("url"))
		if urlIdx == -1 {
			return match
		}
		slashIdx := bytes.IndexByte(match[urlIdx:], '/')
		if slashIdx == -1 {
			return match
		}
		return spliceAt(match, urlIdx+slashIdx, r.proxyPrefixB)
	})
}

// rewriteCookie rewrites Set-Cookie attributes for proxy compatibility:
// - Path: rewritten to use the proxy prefix
// - Domain: stripped (cookie defaults to proxy host)
// - Secure: stripped when frontend is HTTP
// - SameSite: Strict downgraded to Lax (Strict is too restrictive through proxy)
func (r *contentRewriter) rewriteCookie(setCookie string, secure bool) string {
	parts := strings.Split(setCookie, ";")
	filtered := parts[:0] // reuse backing array
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lower := strings.ToLower(trimmed)

		// Strip Domain — cookie should default to the proxy host
		if strings.HasPrefix(lower, "domain=") {
			continue
		}

		// Strip Secure when frontend is HTTP
		if lower == "secure" && !secure {
			continue
		}

		// Downgrade SameSite=Strict to Lax
		if strings.HasPrefix(lower, "samesite=") {
			if strings.EqualFold(trimmed[9:], "Strict") {
				filtered = append(filtered, " SameSite=Lax")
				continue
			}
			filtered = append(filtered, part)
			continue
		}

		// Rewrite Path (existing logic)
		if strings.HasPrefix(lower, "path=") {
			path := trimmed[5:]
			if r.targetPath != "" && strings.HasPrefix(path, r.targetPath) {
				path = r.proxyPrefix + strings.TrimPrefix(path, r.targetPath)
				if path == r.proxyPrefix {
					path = r.proxyPrefix + "/"
				}
			} else if strings.HasPrefix(path, "/") && !strings.HasPrefix(path, r.proxyPrefix) {
				path = r.proxyPrefix + path
			}
			filtered = append(filtered, " Path="+path)
			continue
		}

		filtered = append(filtered, part)
	}
	return strings.Join(filtered, ";")
}

// NewReverseProxyHandler creates a new reverse proxy handler.
// proxyTimeout is the global timeout for proxied HTTP requests (e.g. "30s").
// Optional ReverseProxyOptions control cross-cutting behavior such as
// stripping the Muximux session cookie from outgoing requests.
func NewReverseProxyHandler(apps []config.AppConfig, proxyTimeout string, opts ...ReverseProxyOptions) *ReverseProxyHandler {
	timeout, err := time.ParseDuration(proxyTimeout)
	if err != nil || timeout <= 0 {
		timeout = 30 * time.Second
	}

	var opt ReverseProxyOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	h := &ReverseProxyHandler{
		routes:            make(map[string]*proxyRoute),
		timeout:           timeout,
		sessionCookieName: opt.SessionCookieName,
	}
	h.RebuildRoutes(apps)
	return h
}

// RebuildRoutes rebuilds proxy routes from the current app config.
// This is called after config changes to pick up new/changed/removed proxy apps.
func (h *ReverseProxyHandler) RebuildRoutes(apps []config.AppConfig) {
	h.mu.RLock()
	cookieName := h.sessionCookieName
	h.mu.RUnlock()

	newRoutes := make(map[string]*proxyRoute)
	for i := range apps {
		if !apps[i].Proxy || !apps[i].Enabled {
			continue
		}
		route := buildSingleProxyRoute(&apps[i], h.timeout, cookieName)
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
// sessionCookieName, when non-empty, is the Muximux session cookie name that
// must be stripped from outgoing Cookie headers before forwarding.
func buildSingleProxyRoute(app *config.AppConfig, timeout time.Duration, sessionCookieName string) *proxyRoute {
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

	appName := app.Name
	proxy := &httputil.ReverseProxy{
		Director:       buildDirector(proxyPrefix, targetPath, targetURL, app.ProxyHeaders, sessionCookieName),
		ModifyResponse: createModifyResponse(proxyPrefix, targetPath, rewriter), //nolint:bodyclose // response body managed by httputil.ReverseProxy
		Transport:      transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// httputil.ReverseProxy's default ErrorHandler is net/http's
			// "Bad Gateway" with no log line. Emit a structured audit
			// entry so an operator can correlate 502s with the backend
			// that produced them (findings.md H15).
			logging.From(r.Context()).Warn("Reverse proxy error",
				"source", "proxy",
				"app", appName,
				"target", targetURL.String(),
				"path", r.URL.Path,
				"error", err,
			)
			respondError(w, r, http.StatusBadGateway, errBadGateway)
		},
	}

	return &proxyRoute{
		name:              app.Name,
		slug:              slug,
		proxyPrefix:       proxyPrefix,
		targetURL:         targetURL,
		targetPath:        targetPath,
		skipTLSVerify:     skipTLS,
		timeout:           timeout,
		proxy:             proxy,
		rewriter:          rewriter,
		customHeaders:     app.ProxyHeaders,
		sessionCookieName: sessionCookieName,
		minRole:           app.MinRole,
		allowedGroups:     append([]string(nil), app.AllowedGroups...),
	}
}

// buildDirector creates the Director function for a reverse proxy that rewrites
// incoming request paths from the proxy prefix to the backend target.
// customHeaders are injected into every proxied request. sessionCookieName,
// when non-empty, is removed from the outgoing Cookie header so the Muximux
// session identifier is never leaked to the backend.
// expandHeaderValue substitutes the supported identity variables in a
// custom proxy-header value with the authenticated user's details, so an
// operator can forward identity to a backend (e.g. `X-Forwarded-User:
// ${user}`). Supported: ${user} (username), ${role}, ${email},
// ${display_name}, ${groups} (comma-joined). An unauthenticated request
// (auth.method=none) expands them to empty. Substituted values are
// stripped of CR/LF/NUL so a crafted IdP claim can't inject a header or
// smuggle a request. Unknown ${vars} are left untouched (a visible typo,
// not a silent empty). The backend must only trust these headers from
// Muximux.
func expandHeaderValue(value string, user *auth.User) string {
	if !strings.Contains(value, "${") {
		return value
	}
	var username, role, email, displayName, groups string
	if user != nil {
		username = sanitizeHeaderValue(user.Username)
		role = sanitizeHeaderValue(user.Role)
		email = sanitizeHeaderValue(user.Email)
		displayName = sanitizeHeaderValue(user.DisplayName)
		groups = sanitizeHeaderValue(strings.Join(user.Groups, ","))
	}
	return strings.NewReplacer(
		"${user}", username,
		"${role}", role,
		"${email}", email,
		"${display_name}", displayName,
		"${groups}", groups,
	).Replace(value)
}

// sanitizeHeaderValue strips characters that would let a value break out
// of its header (CR/LF) or terminate the field (NUL).
func sanitizeHeaderValue(v string) string {
	if !strings.ContainsAny(v, "\r\n\x00") {
		return v
	}
	return strings.NewReplacer("\r", "", "\n", "", "\x00", "").Replace(v)
}

func buildDirector(proxyPrefix, targetPath string, targetURL *url.URL, customHeaders map[string]string, sessionCookieName string) func(*http.Request) {
	// Precompute whether any custom-header value uses an identity
	// template, so the common (static-header) path never touches the
	// request context.
	customHeadersTemplated := false
	for _, v := range customHeaders {
		if strings.Contains(v, "${") {
			customHeadersTemplated = true
			break
		}
	}

	return func(req *http.Request) {
		// Strip the /proxy/{slug} prefix from the request path
		reqPath := strings.TrimPrefix(req.URL.Path, proxyPrefix)
		if reqPath == "" {
			reqPath = "/"
		}

		// Handle double-prefixing caused by SPAs that construct URLs with urlBase + endpoint.
		// Use TrimPrefix (not ReplaceAll) to avoid corrupting paths that happen to
		// contain the proxy prefix as a substring (e.g. /api/proxy/application/).
		reqPath = strings.TrimPrefix(reqPath, proxyPrefix)

		req.URL.Path = resolveBackendRequestPath(reqPath, targetPath)
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host

		setProxyHeaders(req)
		stripSessionCookie(req, sessionCookieName)

		// Inject per-app custom headers (e.g., Authorization, X-Api-Key).
		// Values may reference the authenticated user via ${user}/${role}/
		// ${email}/${display_name}/${groups} to forward identity to the
		// backend (proxy-auth pattern). The user is fetched once, only when
		// a template is actually used.
		var hdrUser *auth.User
		if customHeadersTemplated {
			hdrUser = auth.GetUserFromContext(req.Context())
		}
		for k, v := range customHeaders {
			req.Header.Set(k, expandHeaderValue(v, hdrUser))
		}

		req.Host = targetURL.Host
		req.Header.Set("Accept-Encoding", "gzip, identity")

		// Strip the Origin header entirely. The proxy makes a server-to-server
		// request where CORS is irrelevant, and backends with no CORS config
		// (e.g. Spring Security) reject ANY request bearing an Origin header
		// with 403 — regardless of method. CSRF protection for frameworks that
		// check Origin (Django, Rails, .NET) is covered by the Referer header,
		// which is rewritten below to match the backend host.
		req.Header.Del("Origin")
		if referer := req.Header.Get("Referer"); referer != "" {
			if u, err := url.Parse(referer); err == nil {
				u.Scheme = targetURL.Scheme
				u.Host = targetURL.Host
				u.Path = strings.TrimPrefix(u.Path, proxyPrefix)
				if u.Path == "" {
					u.Path = "/"
				}
				u.Path = resolveBackendRequestPath(u.Path, targetPath)
				req.Header.Set("Referer", u.String())
			}
		}
	}
}

// documentDir returns the directory portion of a document path, with a
// trailing slash, suitable for a <base href>. "/" and "/docs/" both yield
// "/docs/"-style results: "/docs/page.html" -> "/docs/", "/" -> "/".
func documentDir(docPath string) string {
	if docPath == "" {
		return "/"
	}
	return docPath[:strings.LastIndexByte(docPath, '/')+1]
}

// proxyFacingDocPath recovers the proxy-facing request path of a proxied
// document from its backend response. At ModifyResponse time resp.Request.URL
// holds the backend path (target-prefixed), so for a subpath-mounted app the
// target prefix is stripped back off to get the path the browser requested
// under /proxy/{slug}. Returns "/" when the path is empty or the request is
// absent.
func proxyFacingDocPath(resp *http.Response, targetPath string) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return "/"
	}
	p := resp.Request.URL.Path
	if trimmed := strings.TrimSuffix(targetPath, "/"); trimmed != "" {
		p = strings.TrimPrefix(p, trimmed)
	}
	if p == "" {
		return "/"
	}
	return p
}

// resolveBackendRequestPath joins the request path with the target path.
// When the app is mounted at a subpath (targetPath != "/"), its entire tree
// - including its own /api routes - lives under that prefix, so every path is
// prefixed. A root-mounted app (targetPath "" or "/") is returned unchanged.
func resolveBackendRequestPath(reqPath, targetPath string) string {
	trimmedTargetPath := strings.TrimSuffix(targetPath, "/")
	if trimmedTargetPath == "" {
		return reqPath
	}

	if strings.HasPrefix(reqPath, "/") {
		return trimmedTargetPath + reqPath
	}
	return trimmedTargetPath + "/" + reqPath
}

func createModifyResponse(proxyPrefix, targetPath string, rewriter *contentRewriter) func(*http.Response) error {
	return func(resp *http.Response) error {
		// Remove headers that prevent iframe embedding or restrict features inside it
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("Permissions-Policy")

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

// rewriteCookieHeaders rewrites Set-Cookie attributes for proxy compatibility.
func rewriteCookieHeaders(resp *http.Response, rewriter *contentRewriter) {
	cookies := resp.Header.Values(headerSetCookie)
	if len(cookies) == 0 {
		return
	}

	// Determine if the frontend connection is secure (HTTPS). The client
	// scheme is stamped into the request context by ResolveClientIP, which
	// is the only path that validates X-Forwarded-Proto against the set of
	// trusted proxies. Falling back to X-Forwarded-Proto here would let a
	// direct-HTTP client set Secure=true on proxied cookies just by sending
	// the header, locking the cookies out of their own browser.
	secure := false
	if resp.Request != nil {
		switch {
		case resp.Request.TLS != nil:
			secure = true
		case auth.ClientSchemeFromContext(resp.Request.Context()) == "https":
			secure = true
		}
	}

	resp.Header.Del(headerSetCookie)
	for _, cookie := range cookies {
		rewritten := rewriter.rewriteCookie(cookie, secure)
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
// It also overrides window.parent and window.top to point back to the iframe's own
// window, so proxied apps cannot detect they are embedded and their libraries
// (e.g. MooTools/MochaUI) do not call methods on the Muximux host window.
func (r *contentRewriter) interceptorScript() []byte {
	// The proxy prefix is derived from slugified app names (alphanumeric + hyphens),
	// so it's safe to embed directly in a JavaScript string literal.
	return []byte(`<script data-muximux-proxy>(function(){` +
		// P must be declared first — used by the window isolation check below.
		`var P="` + r.proxyPrefix + `";` +
		// Isolate window.parent/top so the proxied app thinks it is top-level,
		// preventing crashes when libraries (e.g. MochaUI) call methods on the
		// Muximux host window. However, if the parent is WITHIN the same proxy
		// app (e.g. qBittorrent's download dialog in a MochaUI sub-iframe),
		// keep window.parent intact so internal parent-child communication works.
		// window.top is always overridden to prevent any frame from reaching the
		// Muximux host.
		`try{if(window.parent!==window){` +
		`var _ip=false;` +
		`try{_ip=window.parent.location.pathname.indexOf(P)===0}catch(e){}` +
		`if(_ip){` +
		// Parent is within same proxy app — keep window.parent, override only window.top
		`try{if(window.top!==window.parent)Object.defineProperty(window,"top",{value:window.parent,configurable:true})}` +
		`catch(e){Object.defineProperty(window,"top",{value:window.parent,configurable:true})}` +
		`}else{` +
		// Parent is the Muximux host (or cross-origin) — isolate completely
		`Object.defineProperty(window,"parent",{value:window,configurable:true});` +
		`Object.defineProperty(window,"top",{value:window,configurable:true})}` +
		`}}catch(e){}` +
		// Save history API originals before any patching — used for initial strip,
		// pushState/replaceState overrides, and the popstate listener.
		`var _hps=history.pushState,_hrs=history.replaceState;` +
		// Try to override Location.prototype getters so SPA code sees clean
		// paths (without the proxy prefix) at all times. If the pathname getter
		// patch succeeds (_pG=true), the replaceState strip and popstate handler
		// become unnecessary: the actual URL keeps the proxy prefix (correct for
		// server reload) while getters transparently strip it.
		`var _pG=false;` +
		`try{var _pD=Object.getOwnPropertyDescriptor(Location.prototype,"pathname");` +
		`if(_pD&&_pD.get&&_pD.configurable){Object.defineProperty(Location.prototype,"pathname",` +
		`{get:function(){var v=_pD.get.call(this);if(v===P)return"/";` +
		`if(v.indexOf(P+"/")===0)return v.slice(P.length);return v},` +
		`set:_pD.set,enumerable:_pD.enumerable,configurable:true});_pG=true}}catch(e){}` +
		// Override href getter to return URL without proxy prefix, and setter
		// to add the prefix (best-effort — non-configurable in some browsers).
		`try{var _hD=Object.getOwnPropertyDescriptor(Location.prototype,"href");` +
		`if(_hD&&_hD.get&&_hD.configurable){Object.defineProperty(Location.prototype,"href",` +
		`{get:function(){var v=_hD.get.call(this);` +
		`var pn=_pD?_pD.get.call(this):"";` +
		`if(pn===P)return v.replace(P,"/");` +
		`if(pn.indexOf(P+"/")===0)return v.replace(P,"");` +
		`return v},` +
		`set:function(v){_hD.set.call(this,R(""+v))},` +
		`enumerable:_hD.enumerable,configurable:true})}}catch(e){}` +
		// Override toString to match the patched href getter.
		`Location.prototype.toString=function(){return this.href};` +
		// Override document.URL and document.documentURI to match patched href,
		// so frameworks reading these see the clean URL too.
		`try{var _dU=Object.getOwnPropertyDescriptor(Document.prototype,"URL");` +
		`if(_dU&&_dU.get&&_dU.configurable){Object.defineProperty(Document.prototype,"URL",` +
		`{get:function(){return location.href},enumerable:_dU.enumerable,configurable:true})}}catch(e){}` +
		`try{var _dI=Object.getOwnPropertyDescriptor(Document.prototype,"documentURI");` +
		`if(_dI&&_dI.get&&_dI.configurable){Object.defineProperty(Document.prototype,"documentURI",` +
		`{get:function(){return location.href},enumerable:_dI.enumerable,configurable:true})}}catch(e){}` +
		// Fallback: strip the proxy prefix from the initial URL via replaceState.
		// Skipped when getter patches succeeded (_pG=true) since those
		// transparently strip the prefix on every read.
		`if(!_pG){var _il=location.pathname;` +
		`if(_il===P||_il.indexOf(P+"/")===0){` +
		`_hrs.call(history,history.state,"",(_il.slice(P.length)||"/")+location.search+location.hash)}}` +
		// R(u) rewrites root-relative, relative, and same-origin absolute URLs
		// to go through the proxy. Protocol-relative URLs (//host) and scheme
		// URIs (data:, javascript:, mailto:, etc.) are left untouched.
		`function R(u){` +
		`if(u instanceof URL)u=u.href;` +
		`if(typeof u!=="string")return u;` +
		`if(u[0]==="/"&&u[1]!=="/"&&!u.startsWith(P+"/")&&u!==P)return P+u;` +
		`try{var p=new URL(u);if(p.host===location.host&&!p.pathname.startsWith(P+"/")&&p.pathname!==P){p.pathname=P+p.pathname;return p.href}}catch(e){}` +
		`if(u&&u[0]!=="#"&&u[0]!=="?"&&u[0]!=="/"&&u.indexOf(":")===-1)return P+"/"+u;` +
		`return u}` +
		// Patch history.pushState/replaceState to add the proxy prefix so that
		// "Reload frame" requests the correct /proxy/slug/... URL from the server.
		// When Location getters can't be patched (Chrome — non-configurable), the
		// proxy prefix added by R() makes location.pathname return the prefixed path.
		// Framework routers (Vue Router, React Router) that read location.pathname
		// during initialization would then fail to match routes ("Page not found").
		// Fix: immediately re-strip the prefix after each pushState/replaceState
		// so the URL stays clean throughout the entire initialization phase.
		// The guard stays active until the window 'load' event (all resources
		// loaded), which fires well after any framework init — covering sync,
		// microtask (Promise/await), and macrotask (setTimeout/fetch) init paths.
		`var _sR=!_pG,_skip=false;` +
		`if(_sR){if(document.readyState==="complete")_sR=false;` +
		`else window.addEventListener("load",function(){_sR=false},{once:true})}` +
		`function _S(){var p=location.pathname;` +
		`if(p===P||p.indexOf(P+"/")===0){` +
		`_skip=true;_hrs.call(history,history.state,"",(p.slice(P.length)||"/")+location.search+location.hash);_skip=false}}` +
		`history.pushState=function(s,t,u){if(u!=null)u=R(""+u);var r=_hps.call(this,s,t,u);if(_sR)_S();return r};` +
		`history.replaceState=function(s,t,u){if(u!=null)u=R(""+u);var r=_hrs.call(this,s,t,u);if(_sR)_S();return r};` +
		// On back/forward, strip the prefix before the SPA's popstate handler
		// reads location.pathname. Skipped when getter patches succeeded (_pG)
		// since the pathname getter already strips transparently.
		`if(!_pG){window.addEventListener("popstate",function(){` +
		`var p=location.pathname;` +
		`if(p===P||p.indexOf(P+"/")===0){` +
		`_skip=true;_hrs.call(history,history.state,"",(p.slice(P.length)||"/")+location.search+location.hash);_skip=false}` +
		`},true);` +
		// After init completes, restore the proxy prefix in the URL so that:
		// 1. Browser back/forward to this history entry navigates to /proxy/slug/...
		//    instead of "/" (which would load the Muximux SPA shell inside the iframe)
		// 2. "Reload frame" reloads the correct proxied URL from the server
		// The popstate handler above strips the prefix on each back/forward event,
		// so the app framework still sees clean paths.
		// NOTE: This load listener MUST be registered after _sR's load listener
		// (line above) so that _sR becomes false before _rP restores the prefix.
		// If reversed, _sR would still be true and a subsequent pushState/replaceState
		// would re-strip the restored prefix.
		`(function _rP(){` +
		`function _do(){var p=location.pathname;` +
		`if(p!==P&&p.indexOf(P+"/")!==0){` +
		`_hrs.call(history,history.state,"",P+(p==="/"?"/":p)+location.search+location.hash)}}` +
		`if(document.readyState==="complete")_do();` +
		`else window.addEventListener("load",function(){_do()},{once:true})})()}` +
		// Patch location.assign/replace so programmatic navigation goes through proxy
		`var _la=Location.prototype.assign;` +
		`Location.prototype.assign=function(u){return _la.call(this,R(u))};` +
		`var _lr=Location.prototype.replace;` +
		`Location.prototype.replace=function(u){return _lr.call(this,R(u))};` +
		// When the href setter can't be patched (Chrome — non-configurable),
		// location.href = "/path" navigates without the proxy prefix.
		// Use the Navigation API (Chrome 102+) to intercept and redirect these.
		// Skipped when getter patches succeeded (_pG) since the setter patch
		// also succeeded and handles this. Skips form submissions (e.formData)
		// to avoid turning POSTs into GETs. The _skip flag prevents this handler
		// from interfering with our own internal replaceState calls (_S and
		// popstate handler) which strip the proxy prefix — the Navigation API
		// fires synchronously during replaceState and would otherwise block the
		// URL change via preventDefault.
		`if(!_pG&&window.navigation){window.navigation.addEventListener("navigate",function(e){` +
		`if(_skip||!e.canIntercept||!e.cancelable||e.formData)return;` +
		`try{var u=new URL(e.destination.url);` +
		`if(u.host===location.host&&!u.pathname.startsWith(P+"/")&&u.pathname!==P){` +
		`e.preventDefault();_la.call(location,P+u.pathname+u.search+u.hash)}}catch(ex){}})}` +
		// Patch window.open so popups/new-tab navigations go through the proxy
		`var _wo=window.open;` +
		`window.open=function(u){var a=[].slice.call(arguments);if(typeof a[0]==="string")a[0]=R(a[0]);return _wo.apply(this,a)};` +
		// Patch navigator.sendBeacon so analytics/logging requests are proxied
		`var _sb=navigator.sendBeacon;` +
		`if(_sb){navigator.sendBeacon=function(u,d){return _sb.call(this,R(u),d)}}` +
		// Patch fetch() — handle string URLs, Request objects, and URL objects
		`var _F=window.fetch;` +
		`window.fetch=function(i,o){` +
		`if(typeof i==="string")i=R(i);` +
		`else if(i instanceof URL)i=R(i.href);` +
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
		// Block service worker registration from proxied apps — their SWs would
		// register under Muximux's origin and intercept unrelated requests.
		`if(navigator.serviceWorker){` +
		`navigator.serviceWorker.register=function(){return Promise.resolve()};` +
		`try{navigator.serviceWorker.getRegistrations().then(function(r){r.forEach(function(reg){` +
		`if(reg.scope.indexOf(P)!==-1)reg.unregister()})})}catch(e){}}` +
		// Patch Worker/SharedWorker constructors so worker scripts load through the
		// proxy. Without this, new Worker('/worker.js') loads from the Muximux origin.
		`var _Wk=window.Worker;` +
		`if(_Wk){window.Worker=function(u,o){` +
		`if(typeof u==="string")u=R(u);else if(u instanceof URL)u=R(u.href);` +
		`return o!==void 0?new _Wk(u,o):new _Wk(u)};` +
		`window.Worker.prototype=_Wk.prototype}` +
		`var _SW=window.SharedWorker;` +
		`if(_SW){window.SharedWorker=function(u,o){` +
		`if(typeof u==="string")u=R(u);else if(u instanceof URL)u=R(u.href);` +
		`return o!==void 0?new _SW(u,o):new _SW(u)};` +
		`window.SharedWorker.prototype=_SW.prototype}` +
		// Patch Audio constructor so new Audio('/sound.mp3') loads through the proxy.
		// The HTMLMediaElement.src setter (via W) catches audio.src = url, but the
		// constructor argument bypasses it.
		`var _Au=window.Audio;` +
		`if(_Au){window.Audio=function(u){return typeof u==="string"?new _Au(R(u)):new _Au};` +
		`window.Audio.prototype=_Au.prototype}` +
		// Shim the Notifications API so proxied apps can show notifications
		// despite the browser's cross-origin iframe restriction. Calls are
		// forwarded to Muximux via postMessage; Muximux's notification bridge
		// shows the notification under its top-level origin. Requires
		// allow_notifications: true on the app config. Apps that use the
		// standard new Notification(title, {body}) API work without code changes.
		//
		// Permission state is synced from the top-level Muximux window via a
		// postMessage handshake rather than hardcoded to "granted". The shim
		// starts at "default", queries the parent on load, and forwards any
		// requestPermission() call to the parent so the real browser prompt
		// shows up at Muximux's top-level origin.
		`try{` +
		`var _fakePerm="default";var _permResolvers=[];` +
		`var _fakeN=function(t,o){` +
		`o=o||{};` +
		`window.parent.postMessage({type:"muximux:notify",title:t,body:o.body,tag:o.tag},"*");` +
		`this.title=t;this.body=o.body||"";this.tag=o.tag||"";` +
		`this.close=function(){};this.addEventListener=function(){};this.removeEventListener=function(){};` +
		`};` +
		`Object.defineProperty(_fakeN,"permission",{get:function(){return _fakePerm},configurable:true});` +
		`_fakeN.requestPermission=function(cb){` +
		`var p=new Promise(function(r){_permResolvers.push(r)});` +
		`window.parent.postMessage({type:"muximux:notify-request-permission"},"*");` +
		`if(typeof cb==="function")p.then(cb);return p};` +
		`_fakeN.maxActions=0;` +
		`window.addEventListener("message",function(e){` +
		`if(!e.data||e.data.type!=="muximux:notify-permission")return;` +
		`if(e.source!==window.parent)return;` +
		`var p=e.data.permission;` +
		`if(p==="granted"||p==="denied"||p==="default"){` +
		`_fakePerm=p;` +
		`while(_permResolvers.length){var r=_permResolvers.shift();try{r(p)}catch(err){}}}});` +
		`Object.defineProperty(window,"Notification",{value:_fakeN,writable:true,configurable:true});` +
		`window.parent.postMessage({type:"muximux:notify-query-permission"},"*")` +
		`}catch(e){}` +
		// Namespace localStorage and sessionStorage so each proxied app gets
		// isolated storage, preventing key collisions across apps sharing the
		// same origin. Keys are prefixed with the proxy path (e.g.
		// "/proxy/mealie::token"). Uses ES6 Proxy when available so property
		// access syntax (localStorage["key"] = val) also works.
		`var _ls=window.localStorage,_ss=window.sessionStorage;` +
		`function NS(s){` +
		`var px=P+"::";` +
		`var m={` +
		`getItem:function(k){return s.getItem(px+k)},` +
		`setItem:function(k,v){s.setItem(px+k,""+v)},` +
		`removeItem:function(k){s.removeItem(px+k)},` +
		`clear:function(){var r=[];for(var i=0;i<s.length;i++){var k=s.key(i);` +
		`if(k&&k.indexOf(px)===0)r.push(k)}for(var j=0;j<r.length;j++)s.removeItem(r[j])},` +
		`key:function(n){var c=0;for(var i=0;i<s.length;i++){var k=s.key(i);` +
		`if(k&&k.indexOf(px)===0){if(c===n)return k.slice(px.length);c++}}return null}};` +
		`if(typeof Proxy==="undefined")return m;` +
		`return new Proxy(m,{` +
		`get:function(t,p){` +
		`if(p==="length"){var c=0;for(var i=0;i<s.length;i++)if((s.key(i)||"").indexOf(px)===0)c++;return c}` +
		`if(p in t)return t[p];` +
		`if(typeof p!=="string")return void 0;` +
		`return s.getItem(px+p)},` +
		`set:function(t,p,v){s.setItem(px+p,""+v);return true},` +
		`deleteProperty:function(t,p){s.removeItem(px+p);return true},` +
		`has:function(t,p){return p==="length"||p in t||s.getItem(px+p)!==null},` +
		`ownKeys:function(){var r=[];for(var i=0;i<s.length;i++){var k=s.key(i);` +
		`if(k&&k.indexOf(px)===0)r.push(k.slice(px.length))}return r},` +
		`getOwnPropertyDescriptor:function(t,p){` +
		`var v=s.getItem(px+p);if(v!==null)return{value:v,writable:true,enumerable:true,configurable:true};` +
		`return void 0}` +
		`})}` +
		`try{Object.defineProperty(window,"localStorage",{value:NS(_ls),configurable:true})}catch(e){}` +
		`try{Object.defineProperty(window,"sessionStorage",{value:NS(_ss),configurable:true})}catch(e){}` +
		// Property setter overrides for synchronous URL rewriting on DOM elements.
		// When an SPA sets img.src = "/photo/...", the setter intercepts it and
		// rewrites the URL BEFORE the browser starts loading, preserving the normal
		// load event chain and any animations (e.g. opacity fade-in).
		`function W(C,a){` +
		`var d=Object.getOwnPropertyDescriptor(C.prototype,a);` +
		`if(!d||!d.set)return;` +
		`Object.defineProperty(C.prototype,a,{get:d.get,set:function(v){d.set.call(this,R(v))},enumerable:d.enumerable,configurable:d.configurable})}` +
		`W(HTMLImageElement,"src");W(HTMLScriptElement,"src");W(HTMLSourceElement,"src");W(HTMLMediaElement,"src");W(HTMLVideoElement,"poster");` +
		`W(HTMLIFrameElement,"src");W(HTMLLinkElement,"href");W(HTMLAnchorElement,"href");W(HTMLBaseElement,"href");W(HTMLFormElement,"action");` +
		`W(HTMLObjectElement,"data");W(HTMLButtonElement,"formAction");W(HTMLInputElement,"formAction");` +
		// srcset property setter — parses comma-separated "url descriptor" pairs
		// and rewrites each URL via R(). W() can't handle this because srcset
		// contains multiple URLs, not a single value.
		`var _srs=Object.getOwnPropertyDescriptor(HTMLImageElement.prototype,"srcset");` +
		`if(_srs&&_srs.set){Object.defineProperty(HTMLImageElement.prototype,"srcset",` +
		`{get:_srs.get,set:function(v){if(typeof v==="string"){` +
		`var ps=v.split(","),c=false;` +
		`for(var i=0;i<ps.length;i++){var t=ps[i].trim();if(!t)continue;` +
		`var sp=t.indexOf(" "),u=sp>0?t.substring(0,sp):t,rest=sp>0?t.substring(sp):"";` +
		`var n=R(u);if(n!==u){ps[i]=(i?" ":"")+n+rest;c=true}}` +
		`if(c)v=ps.join(",")}` +
		`_srs.set.call(this,v)},enumerable:_srs.enumerable,configurable:true})}` +
		// MutationObserver as fallback for elements created via innerHTML/parser
		// where property setters don't fire. Only rewrites if URL isn't already prefixed.
		`var urlAttrs={"src":1,"poster":1,"href":1,"action":1,"data":1,"formaction":1};` +
		// Patch setAttribute so that libraries using el.setAttribute("src", url)
		// (instead of el.src = url) get synchronous URL rewriting. Without this,
		// only the MutationObserver catches setAttribute calls — but it fires
		// asynchronously, too late for <script> elements that start loading the
		// moment they're added to the DOM. This fixes MooTools/qBittorrent.
		`var _sA=Element.prototype.setAttribute;` +
		`Element.prototype.setAttribute=function(n,v){` +
		`if(urlAttrs[n.toLowerCase()]&&typeof v==="string")v=R(v);` +
		`return _sA.call(this,n,v)};` +
		// Patch CSSStyleSheet.insertRule to rewrite url() references in CSS rules.
		// CSS-in-JS libraries (styled-components, emotion) use insertRule to inject
		// styles with background-image, @font-face src, etc.
		`var _iR=CSSStyleSheet.prototype.insertRule;` +
		`CSSStyleSheet.prototype.insertRule=function(){` +
		`var a=[].slice.call(arguments);` +
		`if(typeof a[0]==="string")a[0]=a[0].replace(/url\(\s*(['"]?)([^)'"]+)\1\s*\)/g,` +
		`function(_,q,u){return"url("+q+R(u)+q+")"});` +
		`return _iR.apply(this,a)};` +
		// Patch insertAdjacentHTML for synchronous URL fixing. The original
		// inserts HTML into the DOM, then we immediately fixEl() the parent's
		// new children. This closes the same async gap as the setAttribute patch:
		// <script> elements start loading the moment they enter the DOM.
		`var _iAH=Element.prototype.insertAdjacentHTML;` +
		`Element.prototype.insertAdjacentHTML=function(pos,html){` +
		`_iAH.call(this,pos,html);` +
		`var lp=pos.toLowerCase(),t=lp==="beforebegin"||lp==="afterend"?this.parentElement:this;` +
		`if(t)fixEl(t)};` +
		`function fixAttr(el,a){var v=el.getAttribute(a);if(v){var n=R(v);if(n!==v)el.setAttribute(a,n)}}` +
		// fixSrcset rewrites each URL in a srcset attribute (comma-separated "url descriptor" pairs)
		`function fixSrcset(el){` +
		`var v=el.getAttribute("srcset");if(!v)return;` +
		`var changed=false,parts=v.split(",");` +
		`for(var i=0;i<parts.length;i++){var t=parts[i].trim();if(!t)continue;` +
		`var sp=t.indexOf(" "),url=sp>0?t.substring(0,sp):t,rest=sp>0?t.substring(sp):"";` +
		`var n=R(url);if(n!==url){parts[i]=(i?" ":"")+n+rest;changed=true}}` +
		`if(changed)el.setAttribute("srcset",parts.join(","))}` +
		`function fixEl(el){` +
		`if(el.nodeType!==1)return;` +
		`for(var a in urlAttrs){if(el.hasAttribute&&el.hasAttribute(a))fixAttr(el,a)}` +
		`if(el.hasAttribute&&el.hasAttribute("srcset"))fixSrcset(el);` +
		`var ch=el.querySelectorAll("[src],[poster],[href],[srcset],[action],[data],[formaction]");` +
		`for(var i=0;i<ch.length;i++){for(var a in urlAttrs){if(ch[i].hasAttribute(a))fixAttr(ch[i],a)}` +
		`if(ch[i].hasAttribute("srcset"))fixSrcset(ch[i])}}` +
		`new MutationObserver(function(muts){` +
		`for(var i=0;i<muts.length;i++){var m=muts[i];` +
		`if(m.type==="childList"){for(var j=0;j<m.addedNodes.length;j++)fixEl(m.addedNodes[j])}` +
		`else if(m.type==="attributes"){if(urlAttrs[m.attributeName])fixAttr(m.target,m.attributeName);` +
		`else if(m.attributeName==="srcset")fixSrcset(m.target)}}` +
		`}).observe(document,{childList:true,subtree:true,attributes:true,attributeFilter:["src","poster","href","srcset","action","data","formaction"]});` +
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
// It only injects into a document-level <head> — not one that appears inside a
// <script> block (e.g., an HTML string in a JS template literal). Injecting into
// script-embedded <head> corrupts the JavaScript: the interceptor's regex
// backreferences (\1) become illegal octal escapes inside template strings.
//
// When the document has no <base> tag, a <base href="/proxy/slug/"> is injected
// before the interceptor script. On Chrome, Location.prototype.pathname is
// non-configurable so the interceptor must strip the proxy prefix from the URL
// via replaceState (_S). Without <base>, relative resource URLs (css/style.css,
// scripts/app.js) resolve against the stripped "/" instead of the proxy prefix,
// causing 404s for apps like qBittorrent that use relative paths throughout.
// docPath is the proxy-facing request path of the document being served
// (after the /proxy/{slug} prefix is stripped, before the backend target path
// is applied), e.g. "/" or "/docs/page.html". It anchors the injected <base>
// at the document's own directory so relative assets on non-root pages resolve
// correctly even after the interceptor strips the proxy prefix from the URL.
func (r *contentRewriter) injectInterceptor(content []byte, docPath string) []byte {
	// Lowercasing the whole body just to find <head> is a body-sized
	// allocation per proxied HTML response. <head> always lives at
	// the top of valid HTML (and the spec requires <base> to live
	// inside <head> too), so a 4KB scan window covers every real
	// document. Pages that bury <head> past 4KB are malformed
	// enough that we'd rather fail-closed by returning the content
	// unmodified than pay the full ToLower cost.
	const headSearchBound = 4096
	bound := len(content)
	if bound > headSearchBound {
		bound = headSearchBound
	}
	lower := bytes.ToLower(content[:bound])
	searchFrom := 0
	for {
		idx := bytes.Index(lower[searchFrom:], []byte("<head"))
		if idx == -1 {
			return content
		}
		headIdx := searchFrom + idx

		// Ensure this <head is a tag (followed by > or whitespace), not e.g. <header
		after := headIdx + 5
		if after < len(lower) && lower[after] != '>' && lower[after] != ' ' &&
			lower[after] != '\t' && lower[after] != '\n' && lower[after] != '\r' &&
			lower[after] != '/' {
			searchFrom = after
			continue
		}

		// Check we're not inside a <script> block by counting unclosed script tags
		// before this position.
		prefix := lower[:headIdx]
		opens := bytes.Count(prefix, []byte("<script"))
		closes := bytes.Count(prefix, []byte("</script"))
		if opens > closes {
			// Inside a script block — skip this <head> and keep searching
			searchFrom = headIdx + 5
			continue
		}

		closeIdx := bytes.IndexByte(content[headIdx:], '>')
		if closeIdx == -1 {
			return content
		}
		insertPos := headIdx + closeIdx + 1

		// Inject <base> tag if the document doesn't already have one.
		// This anchors relative URL resolution so that apps using relative
		// paths (href="css/style.css") load correctly even after _S() strips
		// the proxy prefix from the document URL. The base is anchored at the
		// document's own directory (not the proxy root) so relative assets on
		// non-root pages resolve under the right directory rather than the
		// proxy root.
		// docPath is derived from the request URL path, so it is
		// attacker-influenced; HTML-escape the href value so a path containing
		// a quote or angle bracket cannot break out of the attribute and inject
		// markup. Proxied content shares the Muximux origin, so an unescaped
		// path here would be reflected XSS.
		var baseTag []byte
		if !bytes.Contains(lower, []byte("<base ")) && !bytes.Contains(lower, []byte("<base>")) {
			baseTag = []byte(`<base href="` + html.EscapeString(r.proxyPrefix+documentDir(docPath)) + `">`)
		}

		script := r.interceptorScript()
		result := make([]byte, 0, len(content)+len(baseTag)+len(script))
		result = append(result, content[:insertPos]...)
		result = append(result, baseTag...)
		result = append(result, script...)
		result = append(result, content[insertPos:]...)
		return result
	}
}

// maxRewriteSize is the maximum response body size (50 MB) that will be buffered
// for URL rewriting. Text responses larger than this stream through unmodified to
// avoid excessive memory use. In practice, HTML/CSS/JS are rarely this large.
// A var (not const) only so tests can lower it without allocating 50 MB.
var maxRewriteSize int64 = 50 * 1024 * 1024

// maxPooledBodyBufSize caps which read buffers go back into the
// pool. Without this, a single oversized response (e.g. a big
// Sonarr/Radarr bundle) would leave a multi-megabyte buffer
// permanently parked in the pool, hogging memory for every
// subsequent small request that picks it up. Standard pattern;
// net/http applies the same trick internally.
const maxPooledBodyBufSize = 1 << 20 // 1 MiB

// bodyReadBufPool reuses the read-into buffer in
// rewriteResponseBody. The buffer is only used to slurp the
// upstream body so the rewriter can scan it; the rewriter
// produces an independent []byte that escapes to the response, so
// returning the buffer to the pool after rewriting is safe and
// doesn't risk data races with the bytes.Reader wrapper that
// ships to the client.
var bodyReadBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

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

	orig := resp.Body
	var reader io.Reader = orig
	isGzipped := strings.Contains(resp.Header.Get(headerContentEncoding), "gzip")

	// When gzipped, tee the compressed bytes as we decompress. If the
	// decompressed body turns out to exceed the rewrite limit we forward
	// the ORIGINAL compressed stream verbatim: the compressed bytes the
	// gzip reader already consumed live in compressedSeen, the rest stays
	// unread in orig, and the two concatenate to the exact original stream.
	var compressedSeen *bytes.Buffer
	if isGzipped {
		compressedSeen = &bytes.Buffer{}
		gzReader, err := gzip.NewReader(io.TeeReader(orig, compressedSeen))
		if err != nil {
			// Surface decode failures so ReverseProxy's ErrorHandler can
			// return a clean 502 rather than forwarding a partially-
			// consumed body as "success" (findings.md H15).
			return fmt.Errorf("proxy: gzip open: %w", err)
		}
		reader = gzReader
		defer gzReader.Close()
	}

	// Cap the decompressed size so an upstream sending an enormous
	// Content-Encoding: gzip payload cannot expand into multi-GB of
	// allocated memory and OOM the process (findings.md H16). The +1 lets
	// us distinguish "exactly at the limit" from "definitely over".
	limited := io.LimitReader(reader, maxRewriteSize+1)

	// Read upstream body into a pooled buffer. The rewriter produces
	// an independent []byte (every rewrite path allocates fresh), so
	// once we've finished rewriting we can release the buffer back
	// to the pool without aliasing risk. orig is NOT closed yet: on the
	// over-limit path below we still need to forward its unread remainder.
	buf := bodyReadBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	if _, err := buf.ReadFrom(limited); err != nil {
		bodyReadBufPool.Put(buf)
		return fmt.Errorf("proxy: read body: %w", err)
	}
	body := buf.Bytes()

	// releaseBuf returns the read buffer to the pool when safe.
	// "Safe" means after any retained body slices have been copied
	// out. Capped to maxPooledBodyBufSize so a single oversized
	// response doesn't park megabytes in the pool indefinitely.
	releaseBuf := func() {
		if buf.Cap() <= maxPooledBodyBufSize {
			bodyReadBufPool.Put(buf)
		}
	}

	// Over the rewrite limit (chunked/gzipped responses whose full size
	// isn't known up front from Content-Length). Forward the WHOLE
	// original stream unmodified rather than truncating it to the first
	// maxRewriteSize bytes. The prefix already read is spliced back in
	// front of orig's unread remainder; headers (Content-Encoding,
	// Content-Length) are left untouched so the framing stays valid.
	if int64(len(body)) > maxRewriteSize {
		var prefix []byte
		if isGzipped {
			prefix = make([]byte, compressedSeen.Len())
			copy(prefix, compressedSeen.Bytes())
		} else {
			prefix = make([]byte, len(body))
			copy(prefix, body)
		}
		releaseBuf()
		resp.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(prefix), orig), orig}
		return nil
	}

	// Under the limit: the buffer holds the entire body, so the original
	// upstream reader can be closed now.
	orig.Close()

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
		rewritten = rewriter.injectInterceptor(rewritten, proxyFacingDocPath(resp, rewriter.targetPath))
	}

	// Rewriter allocates fresh slices, so `rewritten` doesn't alias
	// the pooled buffer - safe to release now.
	releaseBuf()

	resp.Body = io.NopCloser(bytes.NewReader(rewritten))
	resp.ContentLength = int64(len(rewritten))
	resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	resp.Header.Del(headerContentEncoding)

	// Strip caching validators — the body was modified by URL rewriting
	// and/or interceptor injection, so the original ETag and Last-Modified
	// no longer describe this response. Without this, browsers cache the
	// rewritten body keyed to the original ETag and get false 304s.
	resp.Header.Del("ETag")
	resp.Header.Del("Last-Modified")

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

	path = strings.TrimPrefix(path, route.proxyPrefix)

	return resolveBackendRequestPath(path, route.targetPath)
}

// stripSessionCookie removes the named cookie from the outgoing Cookie
// header. Backends must never see the Muximux session identifier: the
// cookie is valid on the Muximux origin with admin privileges and any
// operator of (or attacker between Muximux and) the backend would
// otherwise gain equivalent access.
func stripSessionCookie(r *http.Request, name string) {
	if name == "" {
		return
	}
	cookies := r.Cookies()
	if len(cookies) == 0 {
		return
	}
	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		if c.Name == name {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == len(cookies) {
		return
	}
	if len(filtered) == 0 {
		r.Header.Del("Cookie")
		return
	}
	parts := make([]string, 0, len(filtered))
	for _, c := range filtered {
		parts = append(parts, c.Name+"="+c.Value)
	}
	r.Header.Set("Cookie", strings.Join(parts, "; "))
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
// The WebSocket path used bare net.Dial / tls.Dial before, which hangs
// indefinitely on a dropped SYN-ACK and blocks the hijacked client
// connection right along with it (findings.md H17). Use the route's
// configured proxy timeout as both the connect and TLS handshake
// deadline.
func (route *proxyRoute) dialBackend() (net.Conn, error) {
	targetHost := route.targetURL.Host
	scheme := route.targetURL.Scheme

	timeout := route.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if scheme == "https" {
		host := targetHost
		if h, _, splitErr := net.SplitHostPort(targetHost); splitErr == nil {
			host = h
		}
		dialer := &net.Dialer{Timeout: timeout}
		tlsDialer := tls.Dialer{
			NetDialer: dialer,
			Config: &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: route.skipTLSVerify, //nolint:gosec // configurable per-app
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return tlsDialer.DialContext(ctx, "tcp", targetHost)
	}

	// If no port in host, default to 80
	dialHost := targetHost
	if _, _, splitErr := net.SplitHostPort(targetHost); splitErr != nil {
		dialHost = targetHost + ":80"
	}
	return net.DialTimeout("tcp", dialHost, timeout)
}

// buildUpgradeRequest constructs the raw HTTP upgrade request to send to the
// backend. It mirrors the HTTP Director's header policy: the Muximux session
// cookie is stripped from the outgoing Cookie header, and per-app
// ProxyHeaders are injected so backends that expect header-based
// authentication on WebSocket upgrades see the same credentials they see on
// ordinary HTTP requests.
func (route *proxyRoute) buildUpgradeRequest(r *http.Request, backendPath, targetHost string) []byte {
	stripSessionCookie(r, route.sessionCookieName)
	for k, v := range route.customHeaders {
		r.Header.Set(k, v)
	}

	var reqBuf bytes.Buffer
	fmt.Fprintf(&reqBuf, "%s %s HTTP/1.1\r\n", r.Method, backendPath)
	fmt.Fprintf(&reqBuf, "Host: %s\r\n", targetHost)

	// Forward all client headers except Host (already set above).
	// Reject any header value containing CR or LF: writing one verbatim
	// would smuggle extra headers (or a second request body) into the
	// upstream connection. Go's HTTP server normally rejects these at
	// parse time, but that's a brittle single line of defence for a
	// raw-bytes writer like this (findings.md H6).
	for key, values := range r.Header {
		if strings.EqualFold(key, "Host") {
			continue
		}
		if containsCRLF(key) {
			continue
		}
		for _, v := range values {
			if containsCRLF(v) {
				continue
			}
			fmt.Fprintf(&reqBuf, "%s: %s\r\n", key, v)
		}
	}
	reqBuf.WriteString("\r\n")
	return reqBuf.Bytes()
}

// containsCRLF reports whether s contains any CR, LF, or NUL byte. These
// are the request-smuggling primitives for anything that writes HTTP
// bytes directly to a socket.
func containsCRLF(s string) bool {
	return strings.ContainsAny(s, "\r\n\x00")
}

// forwardUpgradeResponse writes the 101 Switching Protocols response to the client.
//
// Set-Cookie headers from the backend are run through the same path /
// domain / Secure / SameSite rewriter that the HTTP path uses. Without
// this, a backend that sets a cookie on its 101 response (uncommon but
// legal) would land verbatim in the browser, scoped to the Muximux
// origin: it could shadow Muximux session cookies, set Domain values
// that escape the proxy mount, or set Secure=false on an HTTPS-wrapped
// origin (codebase review H3).
//
// Header values that contain CR / LF / NUL are dropped rather than
// emitted, matching the buildUpgradeRequest defence: this is the
// hijacked-conn path, so any unvalidated input becomes part of the
// HTTP byte stream.
func (route *proxyRoute) forwardUpgradeResponse(clientConn net.Conn, resp *http.Response, r *http.Request) error {
	secure := false
	if r != nil {
		switch {
		case r.TLS != nil:
			secure = true
		case auth.ClientSchemeFromContext(r.Context()) == "https":
			secure = true
		}
	}

	logging.From(r.Context()).Debug("Forwarding upgrade response",
		"source", "proxy",
		"app", route.name,
		"secure", secure,
		"set_cookie_count", len(resp.Header.Values(headerSetCookie)))

	var respBuf bytes.Buffer
	fmt.Fprintf(&respBuf, "HTTP/1.1 101 Switching Protocols\r\n")
	for k, vs := range resp.Header {
		isSetCookie := strings.EqualFold(k, headerSetCookie)
		for _, v := range vs {
			if containsCRLF(k) || containsCRLF(v) {
				continue
			}
			out := v
			if isSetCookie && route.rewriter != nil {
				out = route.rewriter.rewriteCookie(v, secure)
				// Re-validate after rewrite: rewriteCookie does
				// not introduce CR/LF today, but if a future
				// change ever produces them on output, we must
				// not write the result to the hijacked conn -
				// CR/LF in a header value is the request-
				// smuggling primitive (review fix L3).
				if containsCRLF(out) {
					continue
				}
			}
			fmt.Fprintf(&respBuf, "%s: %s\r\n", k, out)
		}
	}
	respBuf.WriteString("\r\n")

	_, err := clientConn.Write(respBuf.Bytes())
	return err
}

// bridgeConnections performs bidirectional copy between the client and
// backend, first flushing any data already buffered in either reader.
// When one direction completes (error or EOF) the opposite side is
// closed immediately so its io.Copy unblocks instead of leaking a
// goroutine until the peer eventually times out (findings.md M10).
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
		// Signal the other direction to stop by closing the read side
		// of the client connection. The write side stays open so our
		// own buffered writes drain before teardown.
		_ = clientConn.Close()
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
		_ = backendConn.Close()
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
		respondError(w, r, http.StatusBadGateway, errBadGateway, "source", "proxy", "app", route.name, "target", targetHost, "error", err)
		return
	}
	defer backendConn.Close()

	// Add proxy headers to the original request before forwarding
	setProxyHeaders(r)

	// Send upgrade request to backend
	upgradeReq := route.buildUpgradeRequest(r, backendPath, targetHost)
	if _, err = backendConn.Write(upgradeReq); err != nil {
		respondError(w, r, http.StatusBadGateway, errBadGateway, "source", "proxy", "app", route.name, "error", err)
		return
	}

	// Read the backend's response
	backendBuf := bufio.NewReader(backendConn)
	resp, err := http.ReadResponse(backendBuf, r)
	if err != nil {
		respondError(w, r, http.StatusBadGateway, errBadGateway, "source", "proxy", "app", route.name, "error", err)
		return
	}

	// If backend didn't upgrade, forward the error response as-is
	if resp.StatusCode != http.StatusSwitchingProtocols {
		logging.From(r.Context()).Warn("Backend did not upgrade to WebSocket", "source", "proxy", "app", route.name, "status_code", resp.StatusCode)
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
		respondError(w, r, http.StatusInternalServerError, "WebSocket not supported", "source", "proxy", "app", route.name)
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		logging.From(r.Context()).Error("Failed to hijack client connection", "source", "proxy", "app", route.name, "error", err)
		return
	}
	defer clientConn.Close()

	// Forward the 101 response to the client
	if err = route.forwardUpgradeResponse(clientConn, resp, r); err != nil {
		logging.From(r.Context()).Error("Failed to write upgrade response to client", "source", "proxy", "app", route.name, "error", err)
		return
	}

	// Bidirectional copy: pipe frames between client and backend.
	// If the backend's bufio reader has buffered data (e.g. a frame sent
	// immediately after the handshake), flush it to the client first.
	route.bridgeConnections(clientConn, clientBuf, backendConn, backendBuf)
}

// userMayAccess checks whether the request's authenticated user passes
// the per-app min_role and allowed_groups gates. Admins always pass.
// Unauthenticated requests (auth.method=none) are admitted because the
// route filter cannot enforce a policy without a user; that case is
// the operator's responsibility.
func (route *proxyRoute) userMayAccess(r *http.Request) bool {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		// auth.method=none, or middleware short-circuited. The
		// outer middleware decides whether to allow this; if it
		// reached us, we honor that.
		return true
	}
	if user.Role == auth.RoleAdmin {
		return true
	}
	if route.minRole != "" && !auth.HasMinRole(user.Role, route.minRole) {
		return false
	}
	if len(route.allowedGroups) > 0 {
		if !userInAnyGroup(user.Groups, route.allowedGroups) {
			return false
		}
	}
	return true
}

// userInAnyGroup reports whether any of the user's groups is in the
// allowed-groups list, case-insensitively (mirrors the OIDC and
// forward-auth admin-group matching style).
func userInAnyGroup(userGroups, allowed []string) bool {
	for _, ug := range userGroups {
		for _, ag := range allowed {
			if strings.EqualFold(ug, ag) {
				return true
			}
		}
	}
	return false
}

// ServeHTTP handles proxy requests
func (h *ReverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, proxyPathPrefix)
	parts := strings.SplitN(path, "/", 2)
	slug := parts[0]

	if slug == "" {
		respondError(w, r, http.StatusBadRequest, "Invalid proxy path")
		return
	}

	h.mu.RLock()
	route, exists := h.routes[slug]
	h.mu.RUnlock()

	if !exists {
		respondError(w, r, http.StatusNotFound, "App not found: "+slug)
		return
	}

	// Access control: the apps list returned to non-admin users is
	// already filtered by min_role and allowed_groups in the API
	// layer, but the proxy data path is what actually serves the
	// backend. Without this gate, a non-admin who learns the slug of
	// an admin-only app could reach it via /proxy/{slug}/, with
	// session cookies and any configured proxy_headers attached.
	// Admins bypass both gates by role; the API layer's sanitiser
	// and this proxy gate must agree. (codebase review C1)
	if !route.userMayAccess(r) {
		respondError(w, r, http.StatusForbidden, "Forbidden", "source", "proxy", "app", slug, "path", r.URL.Path)
		return
	}

	logging.From(r.Context()).Debug("Proxying request", "source", "proxy", "app", slug, "method", r.Method, "path", r.URL.Path)

	// WebSocket upgrade requests use hijack-based proxying
	if isWebSocketUpgrade(r) {
		logging.From(r.Context()).Debug("WebSocket upgrade detected", "source", "proxy", "app", slug)
		route.handleWebSocket(w, r)
		return
	}

	// Extend the server's WriteTimeout for proxied responses — proxied apps
	// may stream large files or have slow backends that exceed the default 15s.
	// The deadline is reset on every write/flush (see deadlineResettingWriter)
	// so it bounds idle time between writes rather than total response duration.
	// Without the reset, a long-lived stream (SSE, live logs, slow downloads)
	// is severed the moment h.timeout elapses even while data is still flowing.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(h.timeout))

	route.proxy.ServeHTTP(&deadlineResettingWriter{ResponseWriter: w, rc: rc, timeout: h.timeout}, r)
}

// deadlineResettingWriter wraps the proxy ResponseWriter and pushes the
// connection write deadline forward by timeout on every Write and Flush. This
// turns the absolute deadline set before ServeHTTP into a per-write idle
// deadline: a backend that keeps streaming within timeout never trips it,
// while a genuinely stalled backend is still cut after timeout of silence.
// Unwrap lets http.ResponseController reach the underlying writer for any
// capability this wrapper does not implement. WebSocket upgrades bypass this
// path entirely (handled via hijack before the wrapper is constructed).
type deadlineResettingWriter struct {
	http.ResponseWriter
	rc      *http.ResponseController
	timeout time.Duration
}

func (d *deadlineResettingWriter) Write(p []byte) (int, error) {
	_ = d.rc.SetWriteDeadline(time.Now().Add(d.timeout))
	return d.ResponseWriter.Write(p)
}

func (d *deadlineResettingWriter) Flush() {
	_ = d.rc.SetWriteDeadline(time.Now().Add(d.timeout))
	_ = d.rc.Flush()
}

func (d *deadlineResettingWriter) Unwrap() http.ResponseWriter {
	return d.ResponseWriter
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

# Reverse Proxy Path Rewriting Analysis

## Overview

When proxying web applications at a different path (e.g., `http://app:8080/` → `/proxy/app/`),
all internal references must be rewritten. This document tracks what is handled and what remains.

Implementation: `internal/handlers/reverse_proxy.go`
Tests: `internal/handlers/reverse_proxy_test.go`

---

## 1. HTML Content

| Element/Attribute | Example | Status |
|-------------------|---------|--------|
| `href` | `<a href="/page">` | ✅ |
| `src` | `<script src="/app.js">` | ✅ |
| `action` | `<form action="/submit">` | ✅ |
| `data-*` | `<div data-url="/api">` | ✅ |
| `poster` | `<video poster="/thumb.jpg">` | ✅ |
| `content` | `<meta content="/path">` | ✅ |
| `<base href>` | `<base href="/app/">` | ✅ |
| `srcset` | `<img srcset="/sm.jpg 1x, /lg.jpg 2x">` | ✅ |
| `<source src>` | `<source src="/video.mp4">` | ✅ via `src` handler |
| `<track src>` | `<track src="/captions.vtt">` | ✅ via `src` handler |
| `<embed src>` | `<embed src="/plugin">` | ✅ via `src` handler |
| `<object data>` | `<object data="/flash.swf">` | ✅ via root-relative handler |
| `<use href>` (SVG) | `<use href="/icons.svg#id">` | ✅ preserves fragment |
| `<image href>` (SVG) | `<image href="/img.png">` | ✅ |
| `<meta refresh>` | `<meta http-equiv="refresh" content="0;url=/page">` | ✅ via `content` attr + `Refresh` header |

---

## 2. CSS Content

| Pattern | Example | Status |
|---------|---------|--------|
| `url()` | `background: url("/img.png")` | ✅ |
| `@import` | `@import "/styles.css"` | ✅ |
| `@import url()` | `@import url("/styles.css")` | ✅ |
| `@font-face src` | `src: url("/fonts/font.woff")` | ✅ via `url()` handler |
| `image-set()` | `background: image-set("/1x.png" 1x)` | ❌ Not handled (rare) |

---

## 3. JavaScript Content

### Handled ✅
| Pattern | Example | Notes |
|---------|---------|-------|
| String literals | `fetch("/api/data")` | Via target-path + root-relative rewriting |
| Base path vars | `urlBase: ''`, `basePath`, `baseUrl`, `baseHref` | Empty-string pattern for SPAs |
| Generic JSON paths | `"apiRoot": "/api/v3"` | Any `"key": "/path"` pattern |
| SRI stripping | `integrity="sha256-..."` | Strips static + dynamic (`sriHashes`) |

### Unfixable ⚠️
These patterns are constructed at runtime and cannot be reliably rewritten via regex:

| Pattern | Example | Notes |
|---------|---------|-------|
| Template literals | `` `${base}/api` `` | Variable interpolation |
| String concat | `'/api' + endpoint` | Runtime construction |
| `history.pushState` | `pushState({}, '', '/page')` | Client-side routing |
| `history.replaceState` | `replaceState({}, '', '/page')` | Client-side routing |
| Dynamic imports | `import('/module.js')` | May work if literal string |
| `new URL()` | `new URL('/api', location)` | Runtime URL construction |
| `location.pathname` | Reading/comparing paths | Route matching |
| Worker imports | `new Worker('/worker.js')` | May work if literal string |
| Service Worker | `navigator.serviceWorker.register('/sw.js')` | Scope issues |

---

## 4. JSON/API Responses

| Pattern | Example | Status |
|---------|---------|--------|
| Simple paths | `"apiRoot": "/api/v3"` | ✅ |
| Empty bases | `"urlBase": ""` | ✅ |
| Path arrays | `["/a.jpg", "/b.jpg"]` | ✅ |
| Nested paths | `{"config": {"api": "/v1"}}` | ✅ via generic JSON rewriter |
| HTML in JSON | `"html": "<a href='/link'>"` | Partial — attribute patterns match inside strings |
| Absolute URLs | `"url": "http://host/path"` | ✅ when host matches target |

---

## 5. HTTP Headers

| Header | Example | Status |
|--------|---------|--------|
| `Location` | `Location: /login` | ✅ |
| `Content-Location` | `Content-Location: /resource` | ✅ |
| `Refresh` | `Refresh: 0;url=/page` | ✅ |
| `Set-Cookie Path` | `Set-Cookie: ...; Path=/` | ✅ |
| `Link` | `Link: </style.css>; rel=preload` | ✅ |
| `X-Frame-Options` | Stripped | ✅ removed to allow iframe |
| `Content-Security-Policy` | Stripped | ✅ removed to allow iframe |

---

## 6. WebSocket & Real-time

| Protocol | Example | Status |
|----------|---------|--------|
| WebSocket upgrade | `Upgrade: websocket` | ✅ Transparent hijack proxying |
| `ws://` / `wss://` URLs in JS | `new WebSocket('ws://host/socket')` | ✅ via absolute URL rewriter |
| Server-Sent Events | `new EventSource('/events')` | ✅ via root-relative handler |

WebSocket connections are detected by the `Upgrade: websocket` header and handled via
HTTP hijacking — the proxy dials the backend, copies the upgrade handshake, then
bidirectionally pipes raw TCP frames. Path rewriting happens in the initial HTTP upgrade
request (same Director logic as normal requests). No content rewriting is applied to
WebSocket frames — they pass through as raw binary.

---

## 7. Special Files

| File | Paths Inside | Status |
|------|--------------|--------|
| `manifest.json` | `start_url`, `scope`, `icons[].src` | ✅ via JSON rewriter |
| `browserconfig.xml` | `<square150x150logo src="">` | ✅ via XML/`src` handler |
| `sitemap.xml` | `<loc>` URLs | Partial — not a rewritten content type |
| `robots.txt` | Sitemap references | ❌ plain text, not rewritten |
| OpenAPI/Swagger | `servers[].url`, paths | ✅ via JSON rewriter |

---

## 8. Client-Side Routing

### The Core Problem
SPAs define routes like `/dashboard`, `/users`, etc. When proxied at `/proxy/app/`:
- User navigates to `/proxy/app/dashboard`
- App's router sees path as `/proxy/app/dashboard`
- Router doesn't match `/dashboard` route
- Results in 404 or blank page

### How We Handle It
1. **urlBase rewriting** — Empty base path variables (`urlBase: ''`) are rewritten to `/proxy/slug`
2. **Double-prefix stripping** — Director detects and removes double `/proxy/slug/proxy/slug/` paths caused by SPAs that prepend urlBase to API calls
3. **API path bypass** — `/api` paths skip the target subpath (handles apps like Pi-hole where UI is at `/admin` but API is at `/api`)

---

## Client Header Forwarding

The proxy preserves all original client headers and adds standard proxy headers:

| Header | Value |
|--------|-------|
| `X-Forwarded-For` | Client IP (appended to chain if existing) |
| `X-Forwarded-Host` | Original `Host` header from client |
| `X-Forwarded-Proto` | `http` or `https` based on client connection |
| `X-Real-IP` | Client IP (nginx convention) |

---

## Known Limitations

1. **Minified JS with hardcoded paths** — Cannot reliably rewrite without breaking code
2. **Runtime-constructed paths** — `'/api' + '/users'` happens at runtime, invisible to regex
3. **Binary protocols** — gRPC, MessagePack with embedded paths
4. **Service Workers** — Scope and caching issues are complex
5. **Full response buffering** — Entire response body is read into memory for rewriting; very large responses (>100MB) may cause memory pressure
6. **Set-Cookie Domain** — Only `Path` attribute is rewritten, not `Domain`

---

## Quick Reference: What We Rewrite

```
HTML Attributes:     href, src, action, data-*, poster, srcset, content, base href
SVG Attributes:      href, xlink:href (use, image)
CSS:                 url(), @import, @import url()
JavaScript:          String literals "/path", base path configs (urlBase, basePath, etc.)
JSON:                "key": "/path", "key": "", path arrays ["/a", "/b"]
Headers:             Location, Content-Location, Refresh, Set-Cookie Path, Link
Stripped Headers:    X-Frame-Options, Content-Security-Policy
SRI:                 integrity/crossorigin attrs, dynamic sriHashes
WebSocket:           Upgrade requests proxied via HTTP hijack
Client Forwarding:   X-Forwarded-For, X-Forwarded-Host, X-Forwarded-Proto, X-Real-IP
```

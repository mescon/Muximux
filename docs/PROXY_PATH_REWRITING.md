# Reverse Proxy Path Rewriting Analysis

## Overview

When proxying web applications at a different path (e.g., `http://app:8080/` → `/proxy/app/`),
all internal references must be rewritten. This document analyzes all potential path issues.

---

## 1. HTML Content

### Currently Handled ✅
| Element/Attribute | Example | Status |
|-------------------|---------|--------|
| `href` | `<a href="/page">` | ✅ |
| `src` | `<script src="/app.js">` | ✅ |
| `action` | `<form action="/submit">` | ✅ |
| `data-*` | `<div data-url="/api">` | ✅ |
| `poster` | `<video poster="/thumb.jpg">` | ✅ |
| `content` | `<meta content="/path">` | ✅ |
| `<base href>` | `<base href="/app/">` | ✅ |

### Needs Attention ⚠️
| Element/Attribute | Example | Risk |
|-------------------|---------|------|
| `srcset` | `<img srcset="/sm.jpg 1x, /lg.jpg 2x">` | Medium |
| `<source src>` | `<source src="/video.mp4">` | Low |
| `<track src>` | `<track src="/captions.vtt">` | Low |
| `<object data>` | `<object data="/flash.swf">` | Low |
| `<embed src>` | `<embed src="/plugin">` | Low |
| `<use href>` (SVG) | `<use href="/icons.svg#id">` | Medium |
| `<image href>` (SVG) | `<image href="/img.png">` | Medium |
| `<meta refresh>` | `<meta http-equiv="refresh" content="0;url=/page">` | Low |

---

## 2. CSS Content

### Currently Handled ✅
| Pattern | Example | Status |
|---------|---------|--------|
| `url()` | `background: url("/img.png")` | ✅ |

### Needs Attention ⚠️
| Pattern | Example | Risk |
|---------|---------|------|
| `@import` | `@import "/styles.css"` | High |
| `@import url()` | `@import url("/styles.css")` | High |
| `@font-face src` | `src: url("/fonts/font.woff")` | Medium |
| `image-set()` | `background: image-set("/1x.png" 1x)` | Low |

---

## 3. JavaScript Content

### Currently Handled ✅
| Pattern | Example | Status |
|---------|---------|--------|
| String literals | `fetch("/api/data")` | Partial ✅ |
| Base path vars | `urlBase: ''` | ✅ |

### Difficult to Handle ⚠️
| Pattern | Example | Risk | Notes |
|---------|---------|------|-------|
| Template literals | `` `${base}/api` `` | High | Variable interpolation |
| String concat | `'/api' + endpoint` | High | Runtime construction |
| `history.pushState` | `pushState({}, '', '/page')` | High | Client-side routing |
| `history.replaceState` | `replaceState({}, '', '/page')` | High | Client-side routing |
| Dynamic imports | `import('/module.js')` | Medium | Code splitting |
| `new URL()` | `new URL('/api', location)` | Medium | URL construction |
| `location.pathname` | Reading/comparing paths | High | Route matching |
| Worker imports | `new Worker('/worker.js')` | Medium | Web workers |
| Service Worker | `navigator.serviceWorker.register('/sw.js')` | High | Scope issues |

---

## 4. JSON/API Responses

### Currently Handled ✅
| Pattern | Example | Status |
|---------|---------|--------|
| Simple paths | `"apiRoot": "/api/v3"` | ✅ |
| Empty bases | `"urlBase": ""` | ✅ |

### Needs Attention ⚠️
| Pattern | Example | Risk |
|---------|---------|------|
| Path arrays | `"images": ["/a.jpg", "/b.jpg"]` | Medium |
| Nested paths | `{"config": {"api": "/v1"}}` | Medium |
| HTML in JSON | `"html": "<a href='/link'>"` | High |
| Absolute URLs | `"url": "http://host/path"` | Medium |

---

## 5. HTTP Headers

### Currently Handled ✅
| Header | Example | Status |
|--------|---------|--------|
| `Location` | `Location: /login` | ✅ |
| `Content-Location` | `Content-Location: /resource` | ✅ |
| `Refresh` | `Refresh: 0;url=/page` | ✅ |
| `Set-Cookie Path` | `Set-Cookie: ...; Path=/` | ✅ |

### Needs Attention ⚠️
| Header | Example | Risk |
|--------|---------|------|
| `Link` | `Link: </style.css>; rel=preload` | Medium |
| `Content-Security-Policy` | `script-src /scripts/` | Low |
| `X-Content-Type-Options` | Removed | ✅ |
| `X-Frame-Options` | Removed | ✅ |

---

## 6. WebSocket & Real-time

### Needs Attention ⚠️
| Protocol | Example | Risk |
|----------|---------|------|
| WebSocket | `ws://host/socket` | High |
| Secure WebSocket | `wss://host/socket` | High |
| Server-Sent Events | `new EventSource('/events')` | Medium |

**Solution:** WebSocket upgrade requests need path rewriting in the `Upgrade` request.

---

## 7. Special Files

### Needs Attention ⚠️
| File | Paths Inside | Risk |
|------|--------------|------|
| `manifest.json` | `start_url`, `scope`, `icons[].src` | High |
| `browserconfig.xml` | `<square150x150logo src="">` | Low |
| `sitemap.xml` | `<loc>` URLs | Low |
| `robots.txt` | Sitemap references | Low |
| OpenAPI/Swagger | `servers[].url`, paths | Medium |

---

## 8. Client-Side Routing

### The Core Problem
SPAs define routes like `/dashboard`, `/users`, etc. When proxied at `/proxy/app/`:
- User navigates to `/proxy/app/dashboard`
- App's router sees path as `/proxy/app/dashboard`
- Router doesn't match `/dashboard` route
- Results in 404 or blank page

### Solutions
1. **Base path configuration** - Most SPAs support `basename` or `base` config
2. **urlBase rewriting** - What we do (rewrite the config)
3. **Client-side path stripping** - App strips prefix before routing

---

## Implementation Plan

### Phase 1: Improve Existing (Low Risk)
1. Add `@import` handling for CSS
2. Add `srcset` parsing for images
3. Add `Link` header rewriting
4. Add path arrays in JSON: `["/a", "/b"]`

### Phase 2: WebSocket Support (Medium Risk)
1. Detect WebSocket upgrade requests
2. Rewrite `ws://` and `wss://` URLs in JS
3. Handle WebSocket path in upgrade

### Phase 3: Advanced Patterns (High Risk)
1. Detect common SPA routers and their config patterns
2. Add escape hatch for paths that shouldn't be rewritten
3. Consider AST-based JS rewriting (complex)

### Phase 4: Configuration Options
1. Per-app path exclusions
2. Per-app additional patterns
3. Debug mode to log unrewritten paths

---

## Testing Strategy

### Unit Tests Needed
```go
// CSS @import
{"@import '/styles.css'", "@import '/proxy/app/styles.css'"}
{"@import url('/styles.css')", "@import url('/proxy/app/styles.css')"}

// srcset
{`srcset="/sm.jpg 1x, /lg.jpg 2x"`, `srcset="/proxy/app/sm.jpg 1x, /proxy/app/lg.jpg 2x"`}

// JSON arrays
{`["images": ["/a.jpg", "/b.jpg"]]`, `["images": ["/proxy/app/a.jpg", "/proxy/app/b.jpg"]]`}

// SVG
{`<use href="/icons.svg#menu">`, `<use href="/proxy/app/icons.svg#menu">`}
```

### Integration Tests
- Sonarr: SPA with API calls ✅
- Pi-hole: Traditional app with separate API path ✅
- Radarr: Similar to Sonarr
- Apps with WebSocket (Portainer, etc.)
- Apps with Service Workers

---

## Known Limitations

1. **Minified JS with hardcoded paths** - Cannot reliably rewrite without breaking code
2. **Runtime-constructed paths** - `'/api' + '/users'` happens at runtime
3. **Binary protocols** - gRPC, MessagePack with embedded paths
4. **Encrypted content** - HTTPS between proxy and backend handles this
5. **Service Workers** - Scope and caching issues are complex

---

## Quick Reference: What We Rewrite

```
HTML Attributes:     href, src, action, data-*, poster, srcset, content
CSS:                 url(), @import (TODO)
JavaScript:          String literals "/path", base path configs
JSON:                "key": "/path", "key": ""
Headers:             Location, Content-Location, Refresh, Set-Cookie Path
Cookies:             Path attribute
```

# Built-in Reverse Proxy

## What It Does

A reverse proxy is a server that forwards requests to another server. When you set `proxy: true` on an app, Muximux proxies all requests to that app through `/proxy/{app-slug}/`. This path is on the same port that Muximux runs on. Muximux derives the slug (the URL-safe name) from the app name: lowercased, with spaces replaced by hyphens.

For example, Muximux proxies an app named "My Sonarr" at `/proxy/my-sonarr/`.

Muximux forwards all requests on that path to the app's configured `url`. It then rewrites the responses so the app works correctly at its new location.

> **Note:** This per-app reverse proxy is built into the Go server. It works in every deployment mode: behind Traefik, standalone, or as a full reverse proxy appliance with Caddy. It is completely independent of Caddy. It needs no extra configuration beyond `proxy: true`.

---

## Why It Exists

Most web applications set security headers that stop other pages from loading them inside iframes (frames that show one web page inside another page):

- `X-Frame-Options: DENY` or `SAMEORIGIN`
- `Content-Security-Policy: frame-ancestors 'none'`

Muximux's primary interface loads apps in iframes. Because of this, these headers make the app show a blank frame or a "refused to connect" error. The reverse proxy strips these headers, so Muximux can embed the app.

The proxy also rewrites paths throughout the response. This keeps the app's internal links, asset references, and API calls working from the new `/proxy/{slug}/` base path.

---

## What It Rewrites

The proxy performs several layers of rewriting to make apps work at their new path.

### HTTP Headers

**Response headers:**
- Strips `X-Frame-Options`, `Content-Security-Policy`, and `Permissions-Policy` (allows iframe embedding and unrestricted feature access)
- Rewrites `Location`, `Content-Location`, and `Refresh` redirect headers to the proxy path
- Rewrites `Set-Cookie` attributes: `Path` → proxy prefix, `Domain` stripped (defaults to proxy host), `Secure` stripped when frontend is HTTP, `SameSite=Strict` → `Lax` (too restrictive through proxy)
- Rewrites `Link` header URIs (e.g., preload hints)
- Strips `ETag` and `Last-Modified` on responses whose body was rewritten (prevents stale cache hits)

**Request headers:**
- Strips the `Origin` header entirely. A server-to-server request has no meaningful browser origin, and backends with no CORS config reject any request that carries an `Origin`
- Rewrites `Referer` to match the backend's scheme and host, but only when the request comes from the dashboard itself. Backends that check `Referer` for CSRF then accept requests made from the dashboard. A `Referer` from any other site is removed, so a cross-site request still fails that check. Do not rely on these headers alone: a proxied app should have its own CSRF protection
- Sets `Host` to the backend host
- Forwards `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto`, and `X-Real-IP`

### HTML Content

- Rewrites `href`, `src`, `action`, `poster`, `srcset`, `content`, and `data-*` attributes
- Rewrites `<base href>` tags to point to the proxy path. Injects a `<base>` tag when the document has none, so that relative resource URLs (e.g. qBittorrent's `css/style.css`) resolve against the proxy prefix
- Strips `<meta http-equiv="Content-Security-Policy">` tags, so that nonce requirements do not block the injected interceptor script

### CSS Content

- Rewrites `url()` references to point to the proxy path

### JavaScript and JSON/XML Content

- Strips SRI integrity checks (which break when content is modified)
- Rewrites absolute URLs pointing to the backend server
- Rewrites base path configuration variables (e.g., `urlBase: ""` becomes `urlBase: "/proxy/sonarr"`)
- Does **not** statically rewrite root-relative paths in JS/JSON/XML. The runtime interceptor (see below) handles these, to avoid corrupting URLs meant for third-party servers. One exception: ES module `import` and `export` specifiers (`from '/path'`) in JS are rewritten, because the browser loads modules without going through the interceptor

### Runtime URL Interceptor

Some single-page applications (SPAs, apps that load one page and then update it with JavaScript) build URLs dynamically in JavaScript. For these apps, static text rewriting is not enough. The proxy injects a small script into every HTML response. This script intercepts URL usage at runtime:

- **`fetch()`, `XMLHttpRequest`, `sendBeacon()`** - The interceptor rewrites API calls before they leave the browser
- **`WebSocket`, `EventSource`** - The interceptor routes real-time connections through the proxy
- **DOM property setters** - `img.src`, `script.src`, `source.src`, `media.src`, `video.poster`, `iframe.src`, `link.href`, `a.href`, `base.href`, `form.action`, `object.data`, `button.formAction`, `input.formAction` - the browser's internal setter only ever sees the rewritten URL
- **`img.srcset`** - Custom setter parses comma-separated `url descriptor` pairs and rewrites each URL
- **`setAttribute()`** - Wrapped so libraries using `el.setAttribute('src', url)` get synchronous URL rewriting
- **`CSSStyleSheet.insertRule()`** - Rewrites `url()` references in CSS-in-JS rules (styled-components, emotion)
- **`insertAdjacentHTML()`** - Synchronously fixes URLs in newly inserted HTML fragments
- **`history.pushState/replaceState`** - Wrapped so the app's router always reads back the path it navigated to, without the proxy prefix. Browsers expose `location` as unforgeable, so patching getters cannot hide the prefix. Instead, the interceptor keeps the address clean for the life of the document, including navigations that happen after the page has loaded (an auth check that finishes late, for example)
- **Full-document loads** - Because the address at rest has no prefix, a reload of the frame or a `location.href` assignment would leave the proxy. In browsers with the Navigation API (Chromium), the interceptor redirects the load to the prefixed path before the request goes out. In other browsers, the request reaches the Muximux shell. The shell recognises the app frame by its `window.name` and sends it back to `/proxy/<slug>/...`
- **`location.assign/replace`, `window.open`** - Wrapped to route through the proxy
- **`Worker`, `SharedWorker`, `Audio` constructors** - Script/source URLs routed through the proxy
- **Inside Web Workers** - Workers have their own global scope, so the page patches above do not reach them. A small worker prelude gives them the same `fetch`, `XMLHttpRequest`, `WebSocket`, `EventSource` and `importScripts` routing. The proxy prepends it to scripts that the browser loads as a worker. Workers started from a `blob:` URL (mpegts.js, hls.js and other bundled players) restart from a new blob with the prelude in front. This is what lets players that stream from a worker, such as Dispatcharr's, reach their app
- **`MutationObserver` fallback** - Catches elements created via `innerHTML` or HTML parsing where property setters don't fire
- **`window.parent` and `window.top` isolation** - The interceptor overrides them to point back to the iframe's own window, so the app behaves as if it is running in a standalone browser tab. Smart detection keeps `window.parent` intact for sub-iframes within the same proxied app (e.g., qBittorrent's MochaUI dialogs).
- **`localStorage` / `sessionStorage` isolation** - The interceptor namespaces storage per proxied app, so multiple apps sharing the Muximux origin don't collide on storage keys
- **Service worker blocking** - The interceptor turns `register()` into a no-op. This prevents proxied apps from registering service workers under the Muximux origin. It also unregisters existing proxy-scoped registrations

Some apps, like **Plex**, construct all their image and API URLs in JavaScript at runtime. These apps work through the proxy without any configuration in the app itself.

### SRI (Subresource Integrity)

- Strips `integrity` attributes from HTML tags, since hashes become invalid after the content has been rewritten
- Neutralizes dynamic SRI checks in JavaScript

### Gzip Handling

The proxy transparently decompresses compressed (gzipped) responses before rewriting. It sends the rewritten response uncompressed. If Muximux sits behind an external reverse proxy (nginx, Traefik, Caddy), that proxy can apply its own compression to the final response. You need no configuration for this.

---

## When to Use It

### Per-App Proxy Settings

When `proxy: true`, you can fine-tune the proxy behavior per app:

```yaml
apps:
  - name: Sonarr
    url: https://sonarr.internal:8989
    proxy: true
    proxy_skip_tls_verify: true      # Skip TLS cert verification (default: true)
    proxy_headers:                    # Custom headers sent to the backend
      X-Api-Key: "your-api-key"
      Authorization: "Bearer token"
```

| Setting | Default | Description |
|---------|---------|-------------|
| `proxy_skip_tls_verify` | `true` | When the backend uses HTTPS with a self-signed or internal CA certificate, this skips verification. Set to `false` if you want strict TLS validation. |
| `proxy_headers` | (none) | Key-value map of headers added to every request forwarded to the backend. Useful for API keys or auth tokens the app requires. |

#### Forwarding the signed-in user's identity

A `proxy_headers` value can reference the authenticated Muximux user. The backend can then do header-based ("proxy") auth without its own login. Use any of these variables in a value:

| Variable | Expands to |
|---|---|
| `${user}` | Username |
| `${role}` | `admin` / `power-user` / `user` |
| `${email}` | Email (if known) |
| `${display_name}` | Display name |
| `${groups}` | Group memberships, comma-separated |

```yaml
proxy_headers:
  X-Forwarded-User: "${user}"
  X-Forwarded-Groups: "${groups}"
```

Muximux resolves the values per request from the current session. It strips any CR/LF/NUL from them, so a crafted identity claim can't inject a header. On an unauthenticated instance (`auth.method: none`), the variables expand to empty. Muximux sends static values (API keys, bearer tokens) verbatim. It templates only values that contain `${...}`.

> **Security:** these headers assert who the user is. The backend must accept them **only** from Muximux. A shared Docker network or an internal address is not enough on its own. Anything else that can reach the backend can send its own `X-Forwarded-User`. Make Muximux the only thing that can connect to the backend. Use a network that holds just the two of them or a firewall rule, and publish no ports on the backend. Or have the backend also check a secret that only Muximux sends, such as a static value in `proxy_headers`. Send that secret over HTTPS or a private network, so it can't be read in transit. This is the same trust model as any reverse-proxy header auth.

The global `server.proxy_timeout` (default: `30s`) sets how long the proxy waits for a backend response before it times out. This applies to all proxied apps.

### When to Use It

Enable `proxy: true` when:

- The app refuses to load in an iframe. You see a blank frame, a "refused to connect" error, or a message saying the page cannot be displayed in a frame.
- The app loads in the iframe but assets (CSS, JS, images) fail because their paths do not resolve correctly.
- The app loads and looks correct, but navigation and links break. They point to the original path instead of the proxy path.

---

## When NOT to Use It

Leave `proxy: false` (or omit it) when:

- The app already works fine in an iframe without proxy. Some apps allow embedding by default and do not need any rewriting.
- You are using `open_mode: new_tab`, `new_window`, or `redirect`. The proxy is only useful for iframe mode, because the other modes open the app at its original URL.
- You want to reduce overhead. The rewriting step adds a small amount of latency, so skip the proxy if you don't need it.

---

## Why Some Apps May Not Work

Some applications may not work correctly in an iframe, even with the proxy and runtime interceptor enabled. The most common reasons are:

- **Binary protocols** -- The proxy cannot rewrite gRPC, MessagePack, and other non-text formats with embedded paths.
- **Runtime-constructed paths** -- Template literals (`` `${base}/api` ``) and string concatenation (`'/api' + endpoint`) are invisible to any rewriter.
- **SPA routing conflicts** -- Some SPAs hardcode routes instead of using a configurable base path. Their client-side router may not recognize the `/proxy/{slug}/` prefix. Browsers do not let scripts change what `location` returns. So the interceptor removes the prefix from the page address with `history.replaceState`, after each navigation and on back/forward. The router then reads the path without the prefix. This handles most cases.

> **Note:** The proxy handles these patterns automatically: `Origin`/`Referer` header validation, cookie scoping, service workers, `history.pushState/replaceState`, `location.pathname`, `Worker`/`SharedWorker`, and `localStorage`/`sessionStorage` isolation.

For detailed explanations of each limitation, symptoms, and workarounds, see the [Troubleshooting](troubleshooting.md#reverse-proxy-limitations) page.

### WebSocket Connections

The proxy fully supports WebSocket connections. It detects `Upgrade: websocket` requests. It proxies them transparently over a direct TCP connection to the backend. The proxy rewrites the path of the initial HTTP upgrade request. After that, data flows in both directions without modification. Apps that use WebSockets for live updates, logs, or chat should work through the proxy without additional configuration.

### Mixed Content

You may serve Muximux over HTTPS while the proxied app is HTTP-only on the internal network. The proxy handles this transparently: the browser talks HTTPS to Muximux, and Muximux talks HTTP to the app. However, the app's JavaScript may make direct HTTP requests to other internal services. The browser may block those requests as mixed content (HTTP content on an HTTPS page).

---

## Troubleshooting Proxy Issues

### Blank iframe

Open the browser's developer tools (F12). Check the Console tab for errors. Common causes:

- The app uses a Content-Security-Policy that the proxy did not fully strip. Look for "refused to frame" or "blocked by Content-Security-Policy" messages.
- The app's URL is unreachable from the Muximux server. Verify you can reach the URL from the machine running Muximux.

### Broken Styles or Images

The path rewriting may have missed some URLs. Try opening the app directly at `/proxy/{slug}/` in a new browser tab (not in the iframe). There, the developer tools are easier to use. You can see which requests fail and what paths they try to reach.

### Login Loops

The app's authentication system may conflict with the proxy. This often happens when the app redirects to a login page with an absolute URL that bypasses the proxy. If the app lets you configure a base URL or external URL, set it to match the proxy path (e.g., `/proxy/sonarr`).

### Intermittent Failures

If the app works only some of the time, the cause may be a timing issue with WebSocket connections or service workers. Check the Network tab in developer tools for failed requests.

### If Nothing Works

Set `open_mode: new_tab` as a fallback:

```yaml
apps:
  - name: Problematic App
    url: http://app:8080
    proxy: false
    open_mode: new_tab
```

The app opens in its own browser tab, without the proxy. You lose the integrated dashboard experience. But the app works exactly as it does when you open it directly.

---

## How It Differs from TLS/Gateway (Caddy)

The built-in reverse proxy and the Caddy-based gateway are completely separate systems that serve different purposes.

| | Built-in Reverse Proxy | Caddy Gateway |
|---|---|---|
| **Purpose** | Embed apps in iframes | Serve Muximux with TLS, or host additional sites alongside it |
| **Configured by** | `proxy: true` on individual apps | `server.gateway_sites:` declarative YAML (or Settings -> Gateway) |
| **Runs inside** | The Go server process | Embedded Caddy instance |
| **Works without TLS** | Yes | Yes. Gateway sites start Caddy on their own, with or without TLS for Muximux itself |
| **Rewrites content** | Yes (headers, HTML, CSS, JS, runtime) | No (standard reverse proxy behavior) |

The per-app `proxy: true` setting is for iframe embedding. The `gateway_sites:` entries serve additional sites alongside Muximux on their own subdomains, or handle TLS termination. You can use them independently or together.

---

## Dynamic Route Rebuilds

Muximux rebuilds proxy routes automatically whenever you save configuration changes (add, edit, or delete an app). You do not need to restart Muximux for proxy changes to take effect. New apps with `proxy: true` become available immediately. Removed apps stop being proxied right away.

---

## How It Works (Advanced)

This section describes the technical internals for users who want to understand why something works (or doesn't) and how to debug proxy issues.

### Three Layers of URL Rewriting

The proxy rewrites URLs in three layers:

**Layer 1: Static Rewriting (Server-Side)**

When a response passes through the proxy, the Go server rewrites URLs in the response body. The method depends on the content type:

| Content Type | Rewriting Strategy |
|---|---|
| HTML | Full rewriting - attribute paths (`href`, `src`, etc.), base tags, SRI stripping, CSP meta tag stripping, and interceptor script injection |
| CSS | Full rewriting - `url()` references |
| JS, JSON, XML | Safe-only - SRI stripping, absolute URL rewriting, base path config values. The proxy leaves root-relative paths untouched, to avoid corrupting API data. In JS, ES module `import`/`export` specifiers are the exception and are rewritten |

API responses (JSON, XML) contain data that the SPA reads programmatically. The proxy could rewrite paths inside API data (e.g., `"/library/metadata/123"` → `"/proxy/plex/library/metadata/123"`). The SPA may then embed those already-rewritten paths in new URLs. This causes double-prefixing (the prefix appears twice).

The proxy buffers (holds in memory) only text content types for rewriting. Binary responses (images, videos, archives, file downloads) stream directly from the backend to the browser without being read into memory. As a safety measure, text responses larger than 50 MB also stream through without rewriting.

**Layer 2: Runtime Interceptor (Client-Side, Synchronous)**

The proxy injects a small `<script>` tag into every HTML response. This script patches browser APIs before the app's own JavaScript runs:

| Category | What's Patched | How |
|----------|---|---|
| API calls | `fetch()`, `XMLHttpRequest.open()`, `sendBeacon()` | Wrapper rewrites URL argument |
| Real-time | `WebSocket`, `EventSource` constructors | Wrapper rewrites URL argument |
| DOM setters | `img.src`, `script.src`, `source.src`, `media.src`, `video.poster`, `iframe.src`, `link.href`, `a.href`, `base.href`, `form.action`, `object.data`, `button.formAction`, `input.formAction` | Property setter override on prototype |
| Special setters | `img.srcset`, `setAttribute()`, `CSSStyleSheet.insertRule()`, `insertAdjacentHTML()` | Custom wrappers for multi-URL and CSS-in-JS |
| Location | `pathname`, `href` getters, `assign()`, `replace()`, `toString()`, `document.URL`, `document.documentURI` | Getters strip the prefix where the browser lets the interceptor patch them. Otherwise, `history.replaceState` removes the prefix from the page address. Setters add the prefix. A Navigation API fallback intercepts `location.href` assignment when the setter is non-configurable (Chrome) |
| Navigation | `history.pushState/replaceState`, `window.open` | Wrapper adds the proxy prefix. When getters can't be patched (Chrome), a guard re-strips the prefix after each call, for the life of the page. A full reload of the frame then reaches the Muximux shell, which sends the frame back to `/proxy/<slug>/...` |
| Constructors | `Worker`, `SharedWorker`, `Audio` | Wrapper rewrites URL argument |
| Isolation | `window.parent`, `window.top` | `Object.defineProperty` override |
| Storage | `localStorage`, `sessionStorage` | Namespaced proxy with key prefix |
| Blocked | `navigator.serviceWorker.register()` | Returns resolved promise (no-op) |

Property setter overrides are **synchronous** (they run at once, in the same step). When the app sets `img.src = "/photo/..."`, the browser's internal setter only ever sees the rewritten URL. The image loads from the correct URL on the first try. This keeps the app's normal event chain (load events, animations, etc.) intact.

**Layer 3: MutationObserver (Client-Side, Fallback)**

A `MutationObserver` (a browser API that reports changes to the page) watches for new elements added to the DOM. It also watches for attribute changes on `src`/`poster`. This catches elements created via `innerHTML`, HTML template parsing, or other paths that bypass JavaScript property setters. If a `src` attribute contains an un-prefixed URL, the observer rewrites it.

### Chrome Iframe Timeline Workaround

Chrome may freeze `document.timeline` inside iframes. CSS and Web Animations API animations then stall. Some apps (notably Plex) use `element.animate()` for image fade-in effects with `fill: "auto"`. With this setting, the browser does not keep the animation's end state. When the timeline freezes, images stay stuck at opacity 0, even though they are fully loaded.

The interceptor includes a periodic scan (every 200ms for the first 30 seconds). The scan detects loaded images with `style.opacity === "0"`. It cancels any frozen animations and forces the images visible. The scan turns itself off after 30 seconds, to avoid unnecessary work on long-lived pages.

### Tested Apps

We tested the following apps with the built-in reverse proxy and runtime interceptor:

| App | Status | Notes |
|---|---|---|
| Plex | Works | Full support including posters, PIN auth, WebSocket, media playback |
| Sonarr/Radarr/Lidarr | Works | Set base URL to `/proxy/{slug}` for best results |
| Seerr | Works | |
| Tautulli | Works | Set URL base in Tautulli settings |

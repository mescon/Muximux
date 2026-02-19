# Built-in Reverse Proxy

## What It Does

When `proxy: true` is set on an app, Muximux proxies all requests to that app through `/proxy/{app-slug}/` on the same port Muximux is running on. The slug is derived from the app name: lowercased, with spaces replaced by hyphens.

For example, an app named "My Sonarr" would be proxied at `/proxy/my-sonarr/`.

All requests to that path are forwarded to the app's configured `url`, and the responses are rewritten so the app works correctly at its new location.

> **Note:** This per-app reverse proxy is built into the Go server and works in every deployment mode -- whether Muximux is behind Traefik, running standalone, or acting as a full reverse proxy appliance with Caddy. It is completely independent of Caddy and requires no extra configuration beyond `proxy: true`.

---

## Why It Exists

Most web applications set security headers that prevent them from being loaded inside iframes:

- `X-Frame-Options: DENY` or `SAMEORIGIN`
- `Content-Security-Policy: frame-ancestors 'none'`

Since Muximux's primary interface loads apps in iframes, these headers cause the app to show a blank frame or a "refused to connect" error. The reverse proxy strips these headers so the app can be embedded.

Beyond header stripping, the proxy also rewrites paths throughout the response so that the app's internal links, asset references, and API calls continue to work from the new `/proxy/{slug}/` base path.

---

## What It Rewrites

The proxy performs several layers of rewriting to make apps work at their new path.

### HTTP Headers

- Strips `X-Frame-Options` (allows iframe embedding)
- Strips `Content-Security-Policy` (allows loading in iframe context)
- Rewrites `Location` redirect headers to point to the proxy path
- Rewrites `Set-Cookie` path attributes so cookies are scoped correctly
- Rewrites `Content-Location` and `Refresh` headers

### HTML Content

- Rewrites `href`, `src`, `action`, `poster`, `srcset`, `content`, and `data-*` attributes
- Rewrites `<base href>` tags

### CSS Content

- Rewrites `url()` references to point to the proxy path

### JavaScript and JSON/XML Content

- Strips SRI integrity checks (which break when content is modified)
- Rewrites absolute URLs pointing to the backend server
- Rewrites base path configuration variables (e.g., `urlBase: ""` becomes `urlBase: "/proxy/sonarr"`)
- Root-relative paths in JS/JSON/XML are **not** statically rewritten — the runtime interceptor (see below) handles these to avoid corrupting URLs meant for third-party servers

### Runtime URL Interceptor

For single-page applications (SPAs) that build URLs dynamically in JavaScript, static text rewriting is not enough. The proxy injects a small script into HTML responses that intercepts URL usage at runtime:

- **`fetch()` and `XMLHttpRequest`** — API calls are rewritten before they leave the browser
- **`WebSocket` and `EventSource`** — Real-time connections are routed through the proxy
- **`img.src`, `script.src`, `video.poster`**, etc. — DOM property setters are overridden so the browser never requests the wrong URL
- **`MutationObserver` fallback** — Catches elements created via `innerHTML` or HTML parsing where property setters don't fire

This means apps like **Plex**, which construct all their image and API URLs in JavaScript at runtime, work through the proxy without needing any configuration in the app itself.

### SRI (Subresource Integrity)

- Strips `integrity` attributes from HTML tags, since hashes become invalid after the content has been rewritten
- Neutralizes dynamic SRI checks in JavaScript

### Gzip Handling

Compressed responses are transparently decompressed before rewriting, then re-compressed before being sent to the browser. No configuration is needed.

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

The global `server.proxy_timeout` (default: `30s`) controls how long the proxy waits for a backend response before timing out. This applies to all proxied apps.

### When to Use It

Enable `proxy: true` when:

- The app refuses to load in an iframe. You see a blank frame, a "refused to connect" error, or a message saying the page cannot be displayed in a frame.
- The app loads in the iframe but assets (CSS, JS, images) fail because their paths do not resolve correctly.
- The app loads and looks correct but navigation and links break because they point to the original path instead of the proxy path.

---

## When NOT to Use It

Leave `proxy: false` (or omit it) when:

- The app already works fine in an iframe without proxy. Some apps allow embedding by default and do not need any rewriting.
- You are using `open_mode: new_tab`, `new_window`, or `redirect`. The proxy is only useful for iframe mode, since the other modes open the app at its original URL.
- You want to reduce overhead. The proxy adds a small amount of latency due to the rewriting step, so skip it if it is not needed.

---

## Why Some Apps May Not Work

Even with the proxy and runtime interceptor enabled, some applications may not work correctly in an iframe. The most common reasons are:

- **Service workers** -- Can cache responses under wrong paths or intercept requests before they reach the proxy or the runtime interceptor.
- **Strict origin validation** -- Apps that validate `Origin` or `Referer` headers may reject proxied requests.
- **Binary protocols** -- gRPC, MessagePack, and other non-text formats cannot be rewritten.
- **SPA routing conflicts** -- Some SPAs may not recognize the `/proxy/{slug}/` prefix in their client-side router if they hardcode routes rather than using a configurable base path.

> **Note:** Runtime-constructed URLs (template literals, string concatenation, `fetch()`, `new URL()`, etc.) **are** handled by the runtime interceptor. If a previous version of this page said they weren't supported, that is no longer the case.

For detailed explanations of each limitation, symptoms, and workarounds, see the [Troubleshooting](troubleshooting.md#reverse-proxy-limitations) page.

### WebSocket Connections

WebSocket connections are fully supported. The proxy detects `Upgrade: websocket` requests and transparently proxies them by establishing a direct TCP connection to the backend. Path rewriting is applied to the initial HTTP upgrade request, then data flows bidirectionally without modification. Apps using WebSockets for live updates, logs, or chat should work through the proxy without additional configuration.

### Mixed Content

If Muximux is served over HTTPS but the proxied app is HTTP-only on the internal network, this is handled transparently by the proxy (the browser talks HTTPS to Muximux, and Muximux talks HTTP to the app). However, if the app's JavaScript makes direct HTTP requests to other internal services, the browser may block those as mixed content.

---

## Troubleshooting Proxy Issues

### Blank iframe

Open the browser's developer tools (F12) and check the Console tab for errors. Common causes:

- The app uses a Content-Security-Policy that was not fully stripped. Look for "refused to frame" or "blocked by Content-Security-Policy" messages.
- The app's URL is unreachable from the Muximux server. Verify you can reach the URL from the machine running Muximux.

### Broken Styles or Images

The path rewriting may have missed some URLs. Try accessing the app directly at `/proxy/{slug}/` in a new browser tab (not in the iframe). This lets you use developer tools more easily to see which requests are failing and what paths they are trying to reach.

### Login Loops

The app's authentication system may conflict with the proxy. This often happens when the app redirects to a login page using an absolute URL that bypasses the proxy. If the app supports configuring a base URL or external URL, set it to match the proxy path (e.g., `/proxy/sonarr`).

### Intermittent Failures

If the app works sometimes but not others, it may be a timing issue with WebSocket connections or service workers. Check the Network tab in developer tools for failed requests.

### If Nothing Works

Set `open_mode: new_tab` as a fallback:

```yaml
apps:
  - name: Problematic App
    url: http://app:8080
    proxy: false
    open_mode: new_tab
```

The app opens in its own browser tab with no proxy involvement. You lose the integrated dashboard experience, but the app will work exactly as it does when accessed directly.

---

## How It Differs from TLS/Gateway (Caddy)

The built-in reverse proxy and the Caddy-based gateway are completely separate systems that serve different purposes.

| | Built-in Reverse Proxy | Caddy Gateway |
|---|---|---|
| **Purpose** | Embed apps in iframes | Serve Muximux with TLS, or host additional sites alongside it |
| **Configured by** | `proxy: true` on individual apps | `server.gateway` Caddyfile in config |
| **Runs inside** | The Go server process | Embedded Caddy instance |
| **Works without TLS** | Yes | The gateway is only active when TLS/Caddy is enabled |
| **Rewrites content** | Yes (headers, HTML, CSS, JS, runtime) | No (standard reverse proxy behavior) |

The per-app `proxy: true` setting is for iframe embedding. The `server.gateway` Caddyfile is for serving additional sites alongside Muximux or handling TLS termination. They can be used independently or together.

---

## Dynamic Route Rebuilds

Proxy routes are rebuilt automatically whenever you save configuration changes (add, edit, or delete an app). You do not need to restart Muximux for proxy changes to take effect. New apps with `proxy: true` become available immediately, and removed apps stop being proxied right away.

---

## How It Works (Advanced)

This section describes the technical internals for users who want to understand why something works (or doesn't) and how to debug proxy issues.

### Three Layers of URL Rewriting

The proxy uses three complementary strategies to ensure URLs work correctly:

**Layer 1: Static Rewriting (Server-Side)**

When a response passes through the proxy, the Go server rewrites URLs in the response body based on content type:

| Content Type | Rewriting Strategy |
|---|---|
| HTML | Full rewriting — attribute paths (`href`, `src`, etc.), base tags, SRI stripping, and interceptor script injection |
| CSS | Full rewriting — `url()` references |
| JS, JSON, XML | Safe-only — SRI stripping, absolute URL rewriting, base path config values. Root-relative paths are left untouched to avoid corrupting API data |

The distinction matters: API responses (JSON, XML) contain data that the SPA reads programmatically. If the proxy rewrites paths inside API data (e.g., `"/library/metadata/123"` → `"/proxy/plex/library/metadata/123"`), the SPA may embed those already-rewritten paths in new URLs, causing double-prefixing.

**Layer 2: Runtime Interceptor (Client-Side, Synchronous)**

A small `<script>` tag injected into every HTML response patches browser APIs before the app's own JavaScript runs:

| What's Patched | How |
|---|---|
| `fetch()` | Wrapper rewrites the URL argument |
| `XMLHttpRequest.open()` | Wrapper rewrites the URL argument |
| `WebSocket` constructor | Wrapper rewrites the URL argument |
| `EventSource` constructor | Wrapper rewrites the URL argument |
| `HTMLImageElement.src` | Property setter override on the prototype |
| `HTMLScriptElement.src` | Property setter override on the prototype |
| `HTMLSourceElement.src` | Property setter override on the prototype |
| `HTMLMediaElement.src` | Property setter override on the prototype |
| `HTMLVideoElement.poster` | Property setter override on the prototype |

Property setter overrides are **synchronous** — when the app sets `img.src = "/photo/..."`, the browser's internal setter only ever sees the rewritten URL. This preserves the app's normal event chain (load events, animations, etc.) because the image loads from the correct URL on the first try.

**Layer 3: MutationObserver (Client-Side, Fallback)**

A `MutationObserver` watches for new elements added to the DOM and attribute changes on `src`/`poster`. This catches elements created via `innerHTML`, HTML template parsing, or other paths that bypass JavaScript property setters. If a `src` attribute contains an un-prefixed URL, it's rewritten.

### Chrome Iframe Timeline Workaround

Chrome may freeze `document.timeline` inside iframes, causing CSS and Web Animations API animations to stall. Some apps (notably Plex) use `element.animate()` for image fade-in effects with `fill: "auto"`, which means the animation's end state is not persisted. When the timeline freezes, images remain stuck at opacity 0 despite being fully loaded.

The interceptor includes a periodic scan (every 200ms for the first 30 seconds) that detects loaded images with `style.opacity === "0"`, cancels any frozen animations, and forces them visible. This self-disables after 30 seconds to avoid unnecessary work on long-lived pages.

### Tested Apps

The following apps have been tested with the built-in reverse proxy and runtime interceptor:

| App | Status | Notes |
|---|---|---|
| Plex | Works | Full support including posters, PIN auth, WebSocket, media playback |
| Sonarr/Radarr/Lidarr | Works | Set base URL to `/proxy/{slug}` for best results |
| Overseerr | Works | |
| Tautulli | Works | Set URL base in Tautulli settings |

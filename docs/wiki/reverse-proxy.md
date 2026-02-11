# Reverse Proxy

## What It Does

When `proxy: true` is set on an app, Muximux proxies all requests to that app through `/proxy/{app-slug}/` on the same port Muximux is running on. The slug is derived from the app name: lowercased, with spaces replaced by hyphens.

For example, an app named "My Sonarr" would be proxied at `/proxy/my-sonarr/`.

All requests to that path are forwarded to the app's configured `url`, and the responses are rewritten so the app works correctly at its new location.

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

### JavaScript Content

- Rewrites string literals containing the app's base path
- Rewrites base path configuration variables (e.g., `urlBase: ""` becomes `urlBase: "/proxy/sonarr"`)

### SRI (Subresource Integrity)

- Strips `integrity` attributes from HTML tags, since hashes become invalid after the content has been rewritten
- Neutralizes dynamic SRI checks in JavaScript

### Gzip Handling

Compressed responses are transparently decompressed before rewriting, then re-compressed before being sent to the browser. No configuration is needed.

---

## When to Use It

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

Even with the proxy enabled, some applications will not work correctly in an iframe. Here are the most common reasons:

### JavaScript-Constructed Paths

If an app builds URLs at runtime by concatenating strings or using template literals in JavaScript, the proxy cannot intercept these. The rewriting happens on the server side before the browser executes the JavaScript, but dynamically computed URLs are only known at execution time.

### Client-Side Routing

Single-page applications (SPAs) with client-side routers may not recognize the `/proxy/{slug}/` prefix. The proxy rewrites base path configuration where it can find it, but not all frameworks expose this in a way that can be rewritten.

### WebSocket Connections

Apps that use WebSockets for real-time features (live logs, notifications, chat) may fail if the WebSocket URL is hardcoded or constructed in JavaScript rather than derived from the page's current location.

### Service Workers

Apps using service workers for caching or offline support may conflict with the proxy's path rewriting. The service worker may cache responses under the wrong paths or intercept requests before they reach the proxy.

### Strict Authentication

Apps that validate the `Origin` or `Referer` header may reject proxied requests because the hostname in those headers belongs to Muximux, not the app itself.

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
| **Rewrites content** | Yes (headers, HTML, CSS, JS) | No (standard reverse proxy behavior) |

The per-app `proxy: true` setting is for iframe embedding. The `server.gateway` Caddyfile is for serving additional sites alongside Muximux or handling TLS termination. They can be used independently or together.

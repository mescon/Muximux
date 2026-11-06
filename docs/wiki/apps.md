# Apps & Groups

## Adding Apps

You define apps in `config.yaml` under the `apps` key. Each app is a web application that you want to open through Muximux.

Here is a complete example with all available fields:

```yaml
apps:
  - name: Sonarr                    # Display name
    url: http://sonarr:8989          # App URL (internal network)
    health_url: http://sonarr:8989/ping  # Optional custom health check URL
    icon:
      type: dashboard               # dashboard, lucide, custom, or url
      name: sonarr                   # Icon name from chosen source
      # variant: light              # light or dark (dashboard icons only)
      # file: healarr               # Filename in data/icons/ (type: custom only)
      # url: https://example.com/icon.png  # Remote image URL (type: url only)
      # color: "#ff9600"            # Icon tint color (Lucide only)
      # background: "#ffffff"       # Icon background override
      # invert: false               # Invert icon colors (dark ↔ light)
    color: "#3498db"                # App accent color (used in nav)
    group: Downloads                # Group name (must match a defined group)
    order: 1                        # Sort order within group
    enabled: true                   # Show/hide without deleting
    default: false                  # Load this app on startup
    open_mode: iframe               # How to open (see below)
    proxy: true                     # Route through built-in reverse proxy
    proxy_skip_tls_verify: true     # Skip TLS cert verification for proxy (default: true)
    proxy_headers:                  # Custom headers sent to the backend
      X-Api-Key: "your-key"
    forwarded_headers: true         # Send X-Forwarded-*/X-Real-IP (default true)
    scale: 1.0                      # Zoom level for iframe (0.5 - 2.0)
    health_check: true               # Enable health monitoring (opt-in, disabled by default)
    shortcut: 1                      # Assign keyboard shortcut 1-9
    min_role: ""                     # Minimum role to see this app (user, power-user, admin)
    force_icon_background: false     # Show icon background even when global setting is off
    permissions:                    # Browser features delegated to the iframe (see below)
      - camera
      - microphone
    allow_notifications: false       # Enable the postMessage notification bridge (see below)
    allow_navigation: false          # Let the app switch Muximux's apps via links or postMessage (see Navigation)
    allow_deep_links: false          # Let links with a path (#app?path=...) open a page inside this app (see Navigation)
    access:                         # Restrict access to specific roles/users
      roles: []
      users: []
```

Most fields are optional. A minimal app definition only needs `name` and `url`:

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
```

### Adding apps from Docker

If your apps run as Docker containers, use **Apps tab → Discover from Docker**. Muximux scans your Docker daemon and proposes entries that are ready to import. For known images, it fills in the name, icon and port. Muximux manages imported apps automatically: it updates their URL whenever the container's IP changes. If you prefer GitOps (managing configuration through files in Git), turn on `discovery.docker.auto_import`. Muximux then imports labeled containers automatically, without the modal. See [Docker Discovery](docker-discovery.md) for the full flow.

---

## Open Modes

The `open_mode` field controls how Muximux opens an app when you click it in the navigation.

- **iframe** (default) -- The app loads inside Muximux in an embedded frame. This is the best option for dashboard use. You stay in Muximux and can switch between apps without losing state. If the app refuses to load in an iframe, set `proxy: true`. Muximux then routes the app through the built-in reverse proxy (a server inside Muximux that forwards requests to the app). The proxy strips the headers that block embedding. See the [Reverse Proxy](reverse-proxy.md) page for details.

- **new_tab** -- Opens the app in a new browser tab. Use this for apps that cannot work in iframes at all. Examples are apps with complex authentication flows, or apps with heavy JavaScript that breaks under proxy rewriting.

- **new_window** -- Opens the app in a new browser window (popup-style). Behaves like `new_tab` but opens a separate window instead of a tab.

- **redirect** -- Navigates the current browser tab to the app URL. This leaves Muximux entirely. Use the browser's back button to return.

- **http_action** -- A click sends an HTTP request through the server-side relay, instead of opening a page. Use this for webhook triggers (n8n, Home Assistant, Sonarr commands, etc.). See [HTTP Actions](http-actions.md) for the full setup.

> **Security warning:** Muximux authentication only protects the Muximux dashboard itself. If you embed an app in an iframe **without** `proxy: true`, the browser loads it directly from the app's own URL. Muximux is not in the request path, so it cannot enforce authentication on those requests. Anyone who knows (or guesses) the app's URL can open it directly and bypass Muximux entirely.
>
> If you need Muximux to control access to an app, enable the reverse proxy (`proxy: true`). All requests then go through Muximux, which enforces authentication. Without the proxy, you must rely on the app's own authentication, or on a separate reverse proxy or VPN, to secure it.
>
> This applies to all open modes. `new_tab`, `new_window` and `redirect` all open the app's direct URL in the browser.

---

## Groups

Groups organize your apps in the navigation sidebar. You define them under the `groups` key:

```yaml
groups:
  - name: Media
    icon:
      type: lucide
      name: play
    color: "#e5a00d"
    order: 1
    expanded: true    # Start expanded in navigation
```

> **Note:** The `icon` field must be an object with `type` and `name`. It cannot be a plain string. `icon: play` causes a configuration error. Always use the full object format shown above.

Each app's `group` field must match the `name` of a defined group. If an app has no `group` set, or its group does not exist, the app appears in an "Ungrouped" section at the bottom of the navigation.

You can add apps from the gallery, in the onboarding wizard or in Settings. If the app's preset group does not exist in your configuration, Muximux creates the group automatically with default settings. You can then customize the group's icon, color, and order in Settings. Docker discovery does the same when it imports an app into a group that does not exist yet (see [Docker discovery](docker-discovery.md#groups-are-created-automatically)).

---

## App Ordering

Muximux sorts apps by their `order` value within their group. It sorts groups by their own `order` field. Lower numbers appear first.

If two apps or groups share the same `order` value, their relative order is not guaranteed. Assign unique order values to get a predictable layout.

---

## Default App

Set `default: true` on one app to have it load automatically when you open Muximux:

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
    default: true
```

If no app has `default: true`, Muximux shows a splash screen on startup.

Mark only one app as default. If more than one app has `default: true`, Muximux uses the first one it finds.

---

## Direct Links

You can link directly to any app using a hash URL:

```
https://muximux.example.com/#plex
https://muximux.example.com/#sonarr
```

The hash is the app name converted to a URL-friendly slug (lowercase, spaces replaced with hyphens, special characters removed). For example:

| App Name | Direct Link |
|---|---|
| Plex | `/#plex` |
| Home Assistant | `/#home-assistant` |
| Pi-hole | `/#pi-hole` |
| Sonarr + Radarr (split view) | `/#sonarr+radarr` |

Use a `+` between two app slugs to open them in split view. The first app loads in panel 1 (left/top). The second app loads in panel 2 (right/bottom).

This is useful for:
- Bookmarking a specific app or split view layout in your browser.
- Sharing a link that opens Muximux with a particular app (or pair) already loaded.
- Home screen shortcuts on mobile devices.

When Muximux loads with a hash in the URL, it skips the splash screen. It opens the matching app directly. If no app matches the hash, the normal startup behavior applies (default app or splash screen).

The URL hash updates automatically when you switch apps or toggle split view. You can copy the current URL at any time to get a direct link.

---

## Scale

The `scale` setting controls the zoom level of iframe content. It accepts values from `0.5` to `2.0`:

- Values **below 1.0** zoom out, showing more content at a smaller size.
- A value of **1.0** (the default) shows the app at its native size.
- Values **above 1.0** zoom in, showing less content at a larger size.

```yaml
apps:
  - name: Grafana
    url: http://grafana:3000
    scale: 0.8    # Zoom out slightly to fit more dashboard content
```

This is useful when an app is designed for a different screen size, or has a minimum width that does not fit your layout. It also helps when you want to see more of a dashboard at once.

---

## Forwarded Headers

When Muximux proxies an app, it adds the usual reverse-proxy headers to the upstream request (the request to the backend): `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto` and `X-Real-IP`. Most backends want these headers. They tell the backend the real client IP and the scheme that the browser used.

Some backends do not want them. Your backend can sit behind its own reverse proxy (Traefik, nginx, Caddy). If that proxy trusts forwarded headers only from specific sources, and Muximux is not in its trusted list, it **rejects the request outright**. Traefik answers `400 Bad Request`. In Muximux, this shows as a blank or failed pane, with a 400 in the logs.

Set `forwarded_headers: false` on the app to suppress them:

```yaml
apps:
  - name: Seerr
    url: https://request.example.com
    proxy: true
    forwarded_headers: false      # backend's reverse proxy rejects X-Forwarded-*
```

The field defaults to `true`, so omit it unless you hit this problem. It applies to both the HTTP and the WebSocket paths of the embedding proxy.

The same option exists on [gateway sites](gateway-examples.md) for the same reason, and as the `muximux.gateway.forwarded_headers` [Docker label](docker-discovery.md).

**Diagnosing it:** a proxied app can return 400 while the same URL loads fine in a browser tab. If so, compare the two directly:

```bash
curl -s -o /dev/null -w '%{http_code}\n' https://app.example.com/
curl -s -o /dev/null -w '%{http_code}\n' -H 'X-Forwarded-For: 127.0.0.1' \
     -H 'X-Forwarded-Proto: http' -H 'X-Forwarded-Host: muximux.example.com' \
     https://app.example.com/
```

A `200` followed by a `400` confirms the problem. Set `forwarded_headers: false` and reload.

---

## Enabling and Disabling Apps

Set `enabled: false` to hide an app from the navigation without removing its configuration:

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
    enabled: false    # Hidden from navigation
```

Muximux keeps the app's configuration. To enable the app again at any time, set `enabled: true`. Settings writes the field for every app it saves. In a hand-written `config.yaml`, an app without `enabled: true` stays disabled.

This is useful for temporarily hiding apps that are down for maintenance or that you are still configuring.

---

## Per-App Auth Bypass

When authentication is enabled globally, some paths of a proxied app may need to work without a login. The `auth_bypass` field lets you define these exceptions.

This is also the main reason Muximux has an API key. Combine `proxy: true` with `auth_bypass: [{require_api_key: true}]`. This exposes a backend app's API through Muximux without a Muximux login session. Integrations present the Muximux API key at the front door. Muximux then forwards the request to the backend, and adds the backend's own credentials through `proxy_headers`. See [Authentication > API Key Authentication](authentication.md#api-key-authentication) for the full picture and how to create the key.

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
    proxy: true
    auth_bypass:
      - path: /api/*           # Path pattern (supports * wildcard at end)
        methods: [GET, POST]    # Optional: restrict to certain HTTP methods
        require_api_key: true   # Optional: require X-Api-Key header instead
        allowed_ips:            # Optional: restrict to certain IPs/CIDRs
          - 192.168.0.0/16
      - path: /feed/*
        methods: [GET]          # RSS feeds accessible without login
```

Each bypass rule supports the following fields:

| Field | Required | Description |
|---|---|---|
| `path` | Yes | URL path pattern. Use `*` at the end to match any suffix. |
| `methods` | No | List of HTTP methods to allow. If omitted, all methods are allowed. |
| `require_api_key` | No | If `true`, the request must include a valid `X-Api-Key` header. This trades one form of auth for another rather than removing it entirely. |
| `allowed_ips` | No | List of IP addresses or CIDR ranges. If set, only requests from these sources are allowed through. |

Paths are relative to the app's proxy path. On Sonarr, `/api/*` covers `/proxy/sonarr/api/*`. The full path (`/proxy/sonarr/api/*`) works too. Rules only apply to apps that are enabled and have `proxy: true`, because only their traffic passes through Muximux. Muximux ignores a rule that could reach outside its app, and logs a warning. These rules are: an empty `path`, a path under another app's `/proxy/` prefix, or a path containing `..`. Changes apply as soon as you save the config.

A request that matches a rule needs no Muximux login. So the app's `access`, `min_role` and `allowed_groups` do not apply to it. Use `require_api_key` or `allowed_ips` to decide who gets through. Keep rules to the paths that integrations actually need.

Common use cases:

- Allowing RSS feed readers to fetch feeds without a login session
- Allowing API integrations (e.g., Seerr calling Sonarr) to communicate through the proxy
- Allowing webhook receivers (e.g., from GitHub or notification services) to reach an app endpoint

> **Tip:** Combine `require_api_key` or `allowed_ips` with auth bypass, so that the endpoint stays protected. These checks then protect it, not Muximux's session-based login.

---

## Per-App Access Control

You can restrict which users or roles are allowed to see specific apps using the `access` field:

```yaml
apps:
  - name: Admin Panel
    url: http://admin:9090
    access:
      roles: [admin]           # Only users with the "admin" role can see this app
      users: [alice, bob]      # Or allow specific usernames
```

If `access` is not set on an app, all authenticated users can see it.

You can use `roles`, `users`, or both. When you set both, a user who matches **either** condition gets access. The user does not need to match both.

Muximux checks the list everywhere the app can be reached: the menu and the apps API, the reverse proxy (`/proxy/<slug>/`), HTTP actions, health status and Docker controls. It adds to `min_role` and `allowed_groups`: a user must pass all three. Role names are case-insensitive. Usernames must match exactly, including case. Admins always pass.

---

## Iframe Permissions

Modern browsers deny sensitive features (camera, microphone, geolocation, etc.) to cross-origin iframes by default. A cross-origin iframe is a frame whose page comes from a different origin than Muximux. To let an embedded app use these features, you must delegate them explicitly in the `permissions` field.

```yaml
apps:
  - name: Video Meeting
    url: https://meet.local
    permissions:
      - camera
      - microphone
      - display-capture
      - fullscreen
```

Available permission names follow the [Permissions Policy spec](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy). The full list Muximux can delegate:

| Permission | What it unlocks |
|------------|-----------------|
| `camera` | `getUserMedia({ video: true })` |
| `microphone` | `getUserMedia({ audio: true })` |
| `geolocation` | `navigator.geolocation` |
| `display-capture` | Screen sharing via `getDisplayMedia()` |
| `fullscreen` | `element.requestFullscreen()` |
| `clipboard-read` / `clipboard-write` | Clipboard API |
| `autoplay` | Audio/video autoplay without a user gesture |
| `midi` | Web MIDI API |
| `payment` | Payment Request API for checkout flows |
| `publickey-credentials-get` | WebAuthn sign-in with existing passkeys / security keys |
| `publickey-credentials-create` | WebAuthn registration of new passkeys / security keys |
| `encrypted-media` | DRM-protected media playback (Plex, Jellyfin premium content) |
| `screen-wake-lock` | Keep the screen awake (Wake Lock API) |
| `picture-in-picture` | Floating Picture-in-Picture video window |
| `usb` | WebUSB for device programmers and firmware flashers |
| `serial` | Web Serial for serial consoles, ESPHome, 3D printer firmware |
| `hid` | WebHID for gamepads, security keys, HID devices |

As a shortcut, set `permissions: [all]` to delegate every supported feature. This also includes any new permissions that Muximux adds later. Set `permissions: [none]` to delegate nothing (the same as omitting the field).

When `proxy: true` is set, Muximux delegates each permission to `'self'` (the proxy's own origin). For apps that are not proxied, Muximux delegates the permission to the app's specific origin (e.g. `camera 'self' https://meet.local`).

If `permissions` is omitted or empty, Muximux delegates no features. For an app on a different origin (scheme, host and port) than Muximux, the browser then blocks them. There are two exceptions: fullscreen (every app frame allows it), and the few features that browsers allow everywhere, such as picture-in-picture. A proxied app, or any app served from Muximux's own origin, is same-origin with Muximux. So it still gets the features that the browser allows same-origin frames by default (camera, microphone, geolocation, passkeys and most others). See [Security > Permissions-Policy is permissive](security.md#permissions-policy-is-permissive).

> **Not listed**: `web-share` and `bluetooth` are valid Permissions-Policy directives in the spec. But Chrome currently logs "Unrecognized feature" warnings when they appear in HTTP Permissions-Policy headers. Muximux will add them again once browser support catches up.

---

## Notification Bridge

Browsers block the Web Notifications API in cross-origin iframes. This happens even when the embedded app has notification permission at the OS level. Muximux can route notifications from embedded apps through its own top-level origin. It uses a `postMessage` bridge (a message channel between the app's frame and the Muximux window).

**If your app's notifications do not appear when embedded, try setting `proxy: true` on the app. Proxied apps get a transparent Notifications API shim (replacement code that forwards the calls), so most existing apps work with no code changes.**

Enable the bridge per-app:

```yaml
apps:
  - name: My App
    url: https://app.local
    proxy: true                 # recommended: enables the transparent shim
    allow_notifications: true
```

### How it works

Muximux supports the bridge in two tiers, depending on whether the app is proxied:

**Tier 1: proxied apps (recommended, zero code changes needed).**
When `proxy: true` is set, Muximux injects a `Notification` API shim into the app's HTML. The shim transparently forwards to Muximux any call that the app makes to the standard Web Notifications API:

```javascript
// Inside the embedded app, works as if Muximux wasn't there:
new Notification('New message', { body: 'You have a new task' });

// Permission checks also "just work" (always returns granted):
if (Notification.permission === 'granted') { ... }
await Notification.requestPermission();
```

Most existing apps use exactly this pattern. So they work immediately once `allow_notifications` is enabled.

Muximux syncs the permission state from the top-level Muximux window with a `postMessage` handshake. The shim starts at `"default"` and asks the parent on load. It forwards any `Notification.requestPermission()` call to Muximux, so the real browser prompt appears at Muximux's origin. Reads of `Notification.permission` inside the embedded app show the parent's actual state.

**Tier 2: non-proxied apps (explicit bridge calls).**
When `proxy: false`, Muximux cannot inject code into the iframe, because browsers enforce cross-origin isolation. The app must post a message to the parent window explicitly:

```javascript
window.parent.postMessage({
  type: 'muximux:notify',
  title: 'New message',    // up to 120 chars
  body: 'You have a new task waiting.',  // up to 400 chars
  tag: 'task-123'          // optional: replaces earlier notifications with the same tag
}, '*');
```

### Validation and behaviour

Muximux validates every notification request:

- The `type` must be `'muximux:notify'` (ignored otherwise).
- The sending iframe must belong to an app with `allow_notifications: true`.
- Rate limit: at most one notification per app every 2 seconds.
- The notification always uses the app's configured icon. Muximux ignores any icon URL in the message so one embedded app cannot spoof another app's branding.
- Clicking the notification focuses the Muximux tab and switches to the sending app. Arbitrary click targets from the message are ignored.
- The first notification from any app triggers a browser permission prompt from Muximux's origin. Users grant or deny once for Muximux as a whole, not per embedded app.

### Limitations

- The shim only forwards `title`, `body`, and `tag`. Advanced Notification API features (`actions`, `data`, `onclick` handlers, service-worker-delivered notifications) are not supported.
- Browsers only allow notifications in **secure contexts**. So you must serve Muximux over HTTPS, or open it through `localhost`/`127.0.0.1`. On plain HTTP (non-localhost), the browser permanently denies notifications. The bridge can do nothing about it.

### Platform support

Notifications appear through the service worker registered at `/sw.js`. A service worker is a background script that the browser runs for a site. This one uses `ServiceWorkerRegistration.showNotification()`. This is the only path that works on mobile. The `Notification` constructor is unsupported or unreliable on Android Chrome, Samsung Browser and mobile Firefox. Some desktop browsers have no controlling service worker (e.g. the dev server before the SW activates). On those, Muximux falls back to the constructor.

| Platform | Support | Notes |
|----------|---------|-------|
| Desktop Chrome/Firefox/Edge/Safari on HTTPS | Works | Both proxied and non-proxied apps. |
| Desktop Chrome/Firefox/Edge on plain HTTP (non-localhost) | Blocked | Browsers deny the Notifications API on insecure origins. Use HTTPS or localhost. |
| Android Chrome / Samsung Browser / Firefox on HTTPS | Works | Via the service worker. |
| iOS Safari | Requires PWA install | iOS only allows notifications from sites installed to the home screen. Add Muximux to the home screen first; notifications then work for both proxied and non-proxied apps. |

> **Design note:** The permission belongs to Muximux's origin. So any app that you enable `allow_notifications` for can send notifications. Only enable this for apps that you trust to send appropriate content.

---

## Navigation from Embedded Apps

An app embedded in Muximux (for example a Homarr dashboard) can switch Muximux to another app. Optionally, it can also open a specific page inside that app. This is off by default. You enable it per app, on the app that sends the request:

```yaml
apps:
  - name: Homarr
    url: http://homarr:7575
    allow_navigation: true      # form label: "Allow navigating Muximux"
```

Switching apps needs no setting on the target app. One setting unlocks both mechanisms below. A link can also open a page inside the target, such as `#emby?path=...`. That kind of link needs `allow_deep_links` on the target as well (see [deep links in links](#deep-links-in-links)). In Docker discovery the label is `muximux.app.allow_navigation` (a strict boolean, like `muximux.app.allow_notifications`).

### What you can do with it

- **Use a dashboard as your start page.** Make Homarr (or another dashboard) the [default app](#default-app). Point its tiles at Muximux addresses such as `https://muximux.example.com/#sonarr`. A click then switches Muximux to Sonarr, with its menu entry, health status and keyboard shortcuts. It does not open a second Sonarr inside the dashboard. In Homarr, set the tile to open in the current tab.
- **One-click layouts.** A link to two apps opens them side by side in [split view](split-view.md). For example, use a "Downloads" tile linking to `#sonarr+sabnzbd`, a "Monitoring" tile linking to `#grafana+uptime-kuma` and a "Media" tile linking to `#jellyfin+jellyseerr`. These tiles turn the dashboard into a set of workspaces.
- **Go straight to the item.** A widget that lists things can open the exact item in the app that owns it. Examples are a "recently added" list that opens the movie in Emby or Jellyfin, a request list that opens the request in Jellyseerr, and an alert list that opens the matching Grafana dashboard. Put the path in the link itself ([deep links in links](#deep-links-in-links)), or use the [message API](#message-api) from a script.
- **Runbooks that link to the tool.** A notes or wiki app embedded in Muximux can link to `#portainer` or `#proxmox` right next to the instructions. Then following a runbook (a written procedure) does not mean hunting through the menu.

Turn `allow_navigation` on for the app that the click happens in (the dashboard, the notes app). The apps that it opens do not need it. If the links also carry a path, turn `allow_deep_links` on for the apps that open those pages.

### Plain links

With `allow_navigation` on, the app's iframe sandbox (the limits the browser puts on the frame) gets `allow-top-navigation-by-user-activation`. So a link with `target="_top"` can navigate the Muximux window:

```html
<a href="https://muximux.example.com/#emby" target="_top">Open Emby</a>
<a href="https://muximux.example.com/#sonarr+radarr" target="_top">Sonarr and Radarr</a>
```

- The part before `#` must be the same address that you have Muximux open on. If it differs, the result depends on the link. With `target="_top"`, the browser does a full page load of Muximux instead of just changing the hash. Without it, the frame stays blank.
- A link without `target` (or with `_self`) works too. An example is a Homarr app tile set to open in the current tab. The browser first loads Muximux inside the app's own frame. That copy does not start. It passes the hash to the Muximux window. The Muximux window puts the app back in its frame and then switches. The app reloads once, on the page that the link was on. In browsers without the Navigation API, it reloads on its start page instead. This needs:
  - `allow_navigation` on the app. Without it the frame stays blank, and the browser console says which setting to change.
  - The link to be on a page at the address of one of your apps, as shown by the browser's referrer (the address of the page the link was on). A link on a page that sends no referrer (`Referrer-Policy: no-referrer`, `rel="noreferrer"`) only puts the app back, without switching. The same applies to a link on a site that the app frame wandered off to. Use `target="_top"` there.
  - An app that is not behind Muximux's [reverse proxy](reverse-proxy.md). In a proxied app, the frame goes back to the app's own start page instead. Use `target="_top"`.
- Links in a frame inside the app (for example a Homarr iframe widget) need `target="_top"`. Without it the browser refuses to load Muximux there, because the app's page is still around it.
- To open a specific item, add its path to the link (see [Deep links in links](#deep-links-in-links) below) or use the message API.
- In split view, a single-app link (`#emby`) goes to the pane that already shows that app and makes it active. Otherwise the app replaces the pane the link was clicked in. The other pane is not changed or reloaded. Afterwards the address bar shows the normal split hash (for example `#homarr+emby`).
- Back and Forward work as they do for menu clicks. A link to the app already shown in the other pane still adds a history entry. That entry has the same split hash as before, so Back may first return to the same layout.

### Deep links in links

A link can also open a page inside the target app. To do this, add the path after `?` in the hash. This works in any link, bookmark or address bar, not only in embedded apps:

```
https://muximux.example.com/#emby?path=%2Fweb%2Findex.html%23!%2Fitem%3Fid%3D123
https://muximux.example.com/#radarr?path=%2Fmovie%2F384978
https://muximux.example.com/#emby+radarr?emby=%2Fweb%2Findex.html%23!%2Fitem%3Fid%3D123&radarr=%2Fmovie%2F384978
```

The target app has to allow it:

```yaml
apps:
  - name: Emby
    url: http://emby:8096
    allow_deep_links: true      # form label: "Allow deep links"
```

- A single-app link takes `path=`. A split link takes one key per app slug (`emby=`, `uptime-kuma=`), in any order.
- URL-encode the path (`encodeURIComponent` in JavaScript). An unencoded path works too, as long as it contains no `&` or `%`.
- The path follows the same [path rule](#path-rule) as the message API: it must stay inside the target app. Muximux may refuse the path, or the target may lack `allow_deep_links`. In both cases Muximux still switches apps, and the browser console says why it did not open the page.
- Muximux opens the page once. Then it shows the plain route (`#emby`) in the address bar, so a reload or Back does not reopen the item.
- Pane choice in split view is the same as for plain links. Only the frames that get a path reload.
- The sending app needs nothing extra beyond what plain links already need (`allow_navigation` for a link inside an embedded app). In Homarr, this covers tiles set to open in the current tab. It also covers plain links in Custom JSX widgets, which cannot run the message API.

> **Why the target opts in:** anyone who can send you a link can open any page of an app with `allow_deep_links`. The page opens in Muximux, where you are logged in. Leave the setting off for apps that change something when you merely open a page (a "delete" or "run now" URL).

In Docker discovery the label is `muximux.app.allow_deep_links`.

### Message API

An opted-in app can also ask Muximux to navigate with a `postMessage`:

```javascript
window.top.postMessage({
  type: 'muximux:navigate',
  app: 'emby',                              // slug or display name of the target app
  path: '/web/index.html#!/item?id=123',    // optional: a page inside the target app
  id: 'req-1'                               // optional: copied into the reply
}, '*');
```

Muximux replies to the sender (posted to the sender's origin):

```javascript
{ type: 'muximux:navigate-result', ok: true, id: 'req-1' }
{ type: 'muximux:navigate-result', ok: false, error: 'bad_path', id: 'req-1' }
```

A page directly inside Muximux can use `window.parent` or `window.top`. These are the same window. A page in a frame inside the app (a Homarr iframe widget, say) must use `window.top`, because its `parent` is the app. Such a nested frame counts as the app that it sits in, up to five frames deep. This applies only if the nested page is served from the address of one of your Muximux apps (the sending app's or another one's). Muximux refuses a widget page hosted anywhere else with `not_allowed`.

A page served through Muximux's [reverse proxy](reverse-proxy.md) cannot send the message. The proxy points its `window.parent` and `window.top` at the page itself. Use links there, or turn the proxy off for the sending app. A widget that the proxied app embeds from another address is not proxied. That widget can still use `window.top`.

Muximux echoes `id` only when the request has one. The `id` must be a string of up to 100 characters, or a number. Use it to match replies when more than one request can be in flight.

| `error` | Meaning |
|---------|---------|
| `not_allowed` | The sending app does not have `allow_navigation` enabled. |
| `not_visible` | The sending frame is not the app currently shown. Requests from hidden frames (an app in the other pane's background, or any app while the overview, logs, settings, command palette or shortcuts help cover it) are refused. |
| `bad_request` | Malformed message: `app` missing, not a string, empty or longer than 200 characters, or `path` not a string. |
| `unknown_app` | No enabled app you can see matches `app`. It is matched first against the slug used in hash routes, then against the display name. |
| `not_embeddable` | The target does not open in an iframe (`new_tab`, `new_window`, `redirect` and `http_action` apps are refused). An unset `open_mode` counts as `iframe`. |
| `bad_path` | `path` is not allowed (see the path rule below). |
| `rate_limited` | More than one accepted navigation from this app within 500 ms. Refused messages do not use up the slot. |

Every refusal also logs a `[muximux:navigate]` warning in the browser console. The warning explains why, and what to change. A message from a frame that is not a registered app gets no reply.

#### Path rule

Muximux resolves `path` the way it would resolve an `href`, against the target app's configured URL. The result must stay inside the target app:

- It must keep the app URL's origin and stay under its base directory. The base directory is the app URL's path up to and including the last `/`. When the last segment of the path has no `.` in it, the base directory is the whole path. `http://emby:8096/web/index.html` has base `/web/`. `http://host/sonarr/` and `http://host/sonarr` both have base `/sonarr/`. `http://homarr/?board=main` has base `/`.
- An empty `path` (`path: ''`) is the same as no `path`.
- A leading `/` means the origin root. Without a leading `/`, the path is relative to the app URL. Muximux normalises dot segments before the check, and rejects encoded `%2f` and `%5c`.
- Muximux rejects absolute URLs, `//host`, `javascript:` and other schemes, control characters and values over 2048 characters.

Examples with Emby at `http://emby:8096/`: `/web/index.html#!/item?id=123` is accepted. With Sonarr at `http://host/sonarr/`: `/series/x` is rejected (it leaves `/sonarr/`), while `series/x` and `/sonarr/series/x` are both accepted.

Proxied apps use the same rule against the app's configured URL. Muximux then maps the accepted address onto `/proxy/<slug>/`. So a widget sends the same `path` whether or not the target is proxied.

#### Behaviour

- Without `path`, nothing reloads: the target keeps the page it had, or loads its start page if it was never opened.
- With `path`, only the target's frame in the chosen pane navigates. Repeating the same `path` navigates again.
- Outside split view the target becomes the shown app, as if you had clicked it in the menu. In split view, if either pane already shows the target, that pane becomes active. Otherwise the target replaces the sender's pane. The other pane is never touched.
- Muximux's address records only the app switch. The page opened inside the target is not part of the URL. So Back returns to the previous app, not the previous item.

#### Example: dashboard widget

```html
<button id="open-movie">Open in Emby</button>
<script>
  // Resolves with the reply, or { ok: false, error: 'timeout' } when none
  // arrives (for example when the page is not embedded in Muximux).
  let nextId = 0;
  function muximuxNavigate(app, path) {
    const id = ++nextId;
    return new Promise((resolve) => {
      const done = (result) => {
        clearTimeout(timer);
        window.removeEventListener('message', onReply);
        resolve(result);
      };
      const onReply = (e) => {
        if (e.source !== window.top || !e.data || e.data.type !== 'muximux:navigate-result') return;
        if (e.data.id !== id) return;
        done(e.data);
      };
      const timer = setTimeout(() => done({ ok: false, error: 'timeout' }), 2000);
      window.addEventListener('message', onReply);
      window.top.postMessage({ type: 'muximux:navigate', app, path, id }, '*');
    });
  }

  document.getElementById('open-movie').addEventListener('click', async () => {
    const result = await muximuxNavigate('emby', '/web/index.html#!/item?id=123');
    if (!result.ok) console.warn('Muximux navigation failed:', result.error);
  });
</script>
```

### Trade-offs

> **Only enable this for apps you trust.**
>
> - Browsers cannot restrict where a `target="_top"` link goes. An app with `allow_navigation` can therefore send the whole browser tab to any website after a click. That is why the setting is per app and off by default.
> - Muximux cannot tell whether the user clicked before a message arrives. So an opted-in app that is visible can switch apps through the message API without a click.
> - A frame inside an opted-in app can send the message as that app if it is served from one of your apps' addresses.
> - The message API cannot be used to load a page outside the target app: the path rule keeps the result inside the target's origin and base directory.

## Appearance API

Apps embedded through the built-in reverse proxy can ask Muximux what language, theme and colors the user currently has active. They can then style themselves to match. Muximux exposes a single read-only endpoint:

```
GET /api/appearance
```

Example response:

```json
{
  "language": "en",
  "theme": {
    "family": "catppuccin",
    "variant": "dark",
    "id": "catppuccin",
    "is_dark": true
  },
  "colors": {
    "--bg-base": "#1e1e2e",
    "--bg-surface": "#313244",
    "--bg-elevated": "#45475a",
    "--bg-overlay": "#585b70",
    "--bg-hover": "#45475a",
    "--text-primary": "#cdd6f4",
    "--text-secondary": "#a6adc8",
    "--text-muted": "#9399b2",
    "--border-subtle": "rgba(205, 214, 244, 0.06)",
    "--border-default": "rgba(205, 214, 244, 0.1)",
    "--border-strong": "rgba(205, 214, 244, 0.16)",
    "--accent-primary": "#cba6f7",
    "--accent-secondary": "#b4befe",
    "--color-brand-500": "#cba6f7"
  },
  "theme_css_url": "/themes/catppuccin.css"
}
```

### Authentication

`/api/appearance` accepts two credential shapes.

**Proxied apps (`proxy: true`)** run at `/proxy/<slug>/` on Muximux's own origin. So a same-origin `fetch('/api/appearance')` automatically carries the Muximux session cookie. The app does not need to know about auth at all. This is the zero-config path, and the one most integrations should take.

**External apps** (cross-origin iframes, scripts, dashboards, scheduled jobs) can authenticate with an API key instead of a session cookie. Add the header:

```
X-Api-Key: your-api-key
```

to the request, and Muximux accepts it for `GET /api/appearance` specifically. This is a dedicated bypass rule for this endpoint. Other `/api/*` endpoints still require a session cookie. If the key does not match (or is not set), the request falls through to the normal auth path. It then gets a 401 / login redirect.

### Finding or creating the API key

The API key lives on disk as a **bcrypt hash** (a one-way scrambled form of the key) at `auth.api_key_hash` in `config.yaml`. Muximux never stores the plaintext value. Once you set it, only the hash remains. There are two ways to create one:

**In the UI:** **Settings → Security → API Key.** Paste or generate a key, then save. Muximux hashes the key before it saves it.

**On the command line:** use the built-in hash subcommand:

```bash
muximux hash 'your-chosen-api-key'
# prints: $2a$12$...
```

Then add the output to your config:

```yaml
auth:
  method: builtin
  api_key_hash: "$2a$12$..."
```

Restart Muximux (or hot-reload through the UI). The key is then live. Give the plaintext value to your integration as its `X-Api-Key` header value.

> **Keep the key out of browser code.** `X-Api-Key` is a bearer token: anyone who sees it can read the endpoint. Do NOT embed it in JavaScript that untrusted users load. Instead, put it on a server-side integration that fetches Muximux and passes the result to clients. Or use the proxied-app flow, where the session cookie does the work.

### What an app should do with this

Most apps will fetch this once, on page load, and either:

**Option 1: Apply the returned colors as CSS custom properties** (simplest, works in any app that already uses `var(--something)`):

```js
async function applyMuximuxTheme() {
  try {
    const r = await fetch('/api/appearance');
    if (!r.ok) return;
    const a = await r.json();
    for (const [name, value] of Object.entries(a.colors)) {
      document.documentElement.style.setProperty(name, value);
    }
    document.documentElement.setAttribute('data-theme', a.theme.id);
    document.documentElement.lang = a.language;
  } catch {
    // Muximux not reachable -- leave the app's own defaults alone.
  }
}

applyMuximuxTheme();
```

**Option 2: Load the full theme CSS into your own `<head>`** if you want every variable, not just the curated subset:

```js
async function importMuximuxTheme() {
  const r = await fetch('/api/appearance');
  if (!r.ok) return;
  const a = await r.json();
  if (!a.theme_css_url) return;
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = a.theme_css_url;
  document.head.appendChild(link);
}
```

Your app's own stylesheet can then reference the same `--bg-surface`, `--text-primary`, etc. that Muximux uses. The two then stay visually consistent.

### What's in `colors`

The fourteen names in `colors` are the curated, stable subset. They have documented meanings, and Muximux will not rename them between versions. If you need a variable that isn't in this list, load `theme_css_url` and parse it yourself.

| Variable | Purpose |
|---|---|
| `--bg-base` | Page background |
| `--bg-surface` | Card / panel background |
| `--bg-elevated` | Raised surface (dialog, dropdown) |
| `--bg-overlay` | Modal overlay |
| `--bg-hover` | Hover state background |
| `--text-primary` | Primary text |
| `--text-secondary` | Secondary text |
| `--text-muted` | Muted / placeholder text |
| `--border-subtle` | Faint borders |
| `--border-default` | Standard borders |
| `--border-strong` | Emphasized borders |
| `--accent-primary` | Brand accent |
| `--accent-secondary` | Secondary accent |
| `--color-brand-500` | Canonical brand color |

### Theme id conventions

The `theme.id` field is the exact value Muximux puts on `<html data-theme="...">` for itself. Every named theme follows the same pattern:

- Dark variant: `<family>` -- e.g. `catppuccin`, `nord`, `muximux` (the built-in default)
- Light variant: `<family>-light` -- e.g. `catppuccin-light`, `nord-light`, `muximux-light`

If `theme.variant === "system"`, the operator has asked Muximux to follow the client's OS preference. The server cannot know that preference, so it returns the dark palette (`muximux`) as the default. An app that cares about this can use `matchMedia('(prefers-color-scheme: light)')` in the browser and fetch again with the opposite variant. In practice, most apps just take whatever the endpoint returns.

### Re-fetching on changes

The endpoint does not push updates. If the operator changes the theme while your app is open, your app keeps the old colors until it fetches again. That is deliberate. Polling or a push channel would cost either battery or complexity, for a feature that most apps will not handle. If you do want to re-theme on every change, fetch again on `visibilitychange`. That is enough for the common case of "user switches theme, then tabs back to the app".

### Limitations

- Proxied apps authenticate automatically through the session cookie (same-origin). External integrations need an `X-Api-Key` header. Other `/api/*` endpoints are not reachable by API key.
- No push / event channel. Apps read once on boot.
- `colors` is a curated subset, not every theme variable. Use `theme_css_url` for the rest.
- `variant: "system"` resolves to the dark palette server-side; clients that care can check `prefers-color-scheme`.
- A cross-origin `fetch` with `X-Api-Key` may require a CORS preflight (a check request that the browser sends first). Muximux does not answer that preflight today. If that is a problem, run the fetch from a server-side integration rather than directly from the browser.

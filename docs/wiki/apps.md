# Apps & Groups

## Adding Apps

Apps are defined in `config.yaml` under the `apps` key. Each app represents a web application you want to access through Muximux.

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
    scale: 1.0                      # Zoom level for iframe (0.5 - 2.0)
    health_check: true               # Enable health monitoring (opt-in, disabled by default)
    shortcut: 1                      # Assign keyboard shortcut 1-9
    min_role: ""                     # Minimum role to see this app (user, power-user, admin)
    force_icon_background: false     # Show icon background even when global setting is off
    permissions:                    # Browser features delegated to the iframe (see below)
      - camera
      - microphone
    allow_notifications: false       # Enable the postMessage notification bridge (see below)
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

---

## Open Modes

The `open_mode` field controls how an app is opened when you click it in the navigation.

- **iframe** (default) -- The app loads inside Muximux in an embedded frame. This is the best option for dashboard use, as you stay within Muximux and can switch between apps without losing state. If the app refuses to load in an iframe, set `proxy: true` to route it through the built-in reverse proxy, which strips the headers that block embedding. See the [Reverse Proxy](reverse-proxy.md) page for details.

- **new_tab** -- Opens the app in a new browser tab. Use this for apps that cannot work in iframes at all, such as apps with complex authentication flows or heavy JavaScript that breaks under proxy rewriting.

- **new_window** -- Opens the app in a new browser window (popup-style). Behaves like `new_tab` but opens a separate window instead of a tab.

- **redirect** -- Navigates the current browser tab to the app URL. This leaves Muximux entirely. Use the browser's back button to return.

> **Security warning:** Muximux authentication only protects the Muximux dashboard itself. When an app is embedded in an iframe **without** `proxy: true`, the browser loads it directly from the app's own URL -- Muximux is not in the request path and cannot enforce authentication on those requests. This means anyone who knows (or guesses) the app's URL can access it directly, bypassing Muximux entirely.
>
> If you need Muximux to control access to an app, enable the reverse proxy (`proxy: true`). This routes all requests through Muximux, where authentication is enforced. Without the proxy, you must rely on the app's own authentication or a separate reverse proxy/VPN to secure it.
>
> This applies to all open modes -- `new_tab`, `new_window`, and `redirect` all open the app's direct URL in the browser.

---

## Groups

Groups organize your apps in the navigation sidebar. They are defined under the `groups` key:

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

> **Note:** The `icon` field must be an object with `type` and `name` -- it cannot be a plain string. Writing `icon: play` will cause a configuration error. Always use the full object format shown above.

Each app's `group` field must match the `name` of a defined group. If an app references a group that does not exist, or has no `group` set, it will appear in an "Ungrouped" section at the bottom of the navigation.

When adding apps from the gallery (in the onboarding wizard or Settings), if the app's preset group doesn't exist in your configuration, Muximux automatically creates the group with default settings. You can then customize the group's icon, color, and order in Settings.

---

## App Ordering

Apps are sorted by their `order` value within their group. Groups themselves are sorted by their own `order` field. Lower numbers appear first.

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

Only one app should be marked as default. If multiple apps have `default: true`, the first one found will be used.

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

Use a `+` between two app slugs to open them in split view. The first app loads in panel 1 (left/top) and the second in panel 2 (right/bottom).

This is useful for:
- **Bookmarking** a specific app or split view layout in your browser.
- **Sharing** a link that opens Muximux with a particular app (or pair) already loaded.
- **Home screen shortcuts** on mobile devices.

When Muximux loads with a hash in the URL, it skips the splash screen and opens the matching app directly. If no app matches the hash, the normal startup behavior applies (default app or splash screen).

The URL hash updates automatically as you switch between apps or toggle split view, so you can copy the current URL at any time to get a direct link.

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

This is useful when an app is designed for a different screen size, has a minimum width that does not fit your layout, or when you want to see more of a dashboard at once.

---

## Enabling and Disabling Apps

Set `enabled: false` to hide an app from the navigation without removing its configuration:

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
    enabled: false    # Hidden from navigation
```

The app's configuration is preserved and can be re-enabled at any time by setting `enabled: true` or removing the field entirely (apps are enabled by default).

This is useful for temporarily hiding apps that are down for maintenance or that you are still configuring.

---

## Per-App Auth Bypass

When authentication is enabled globally, you may need to allow certain paths of a proxied app to be accessed without logging in. The `auth_bypass` field lets you define exceptions.

This is also the **primary reason** Muximux has an API key. Combining `proxy: true` with `auth_bypass: [{require_api_key: true}]` exposes a backend app's API through Muximux without requiring a Muximux login session -- integrations present the Muximux API key at the front door, Muximux forwards to the backend with the backend's own credentials injected via `proxy_headers`. See [Authentication > API Key Authentication](authentication.md#api-key-authentication) for the full picture and how to create the key.

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
          - 10.0.0.0/8
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

**Common use cases:**

- Allowing RSS feed readers to fetch feeds without a login session
- Allowing API integrations (e.g., Overseerr calling Sonarr) to communicate through the proxy
- Allowing webhook receivers (e.g., from GitHub or notification services) to reach an app endpoint

> **Tip:** Combine `require_api_key` or `allowed_ips` with auth bypass to ensure the endpoint is still protected -- just not by Muximux's session-based login.

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

You can use `roles`, `users`, or both. When both are specified, a user who matches **either** condition gains access -- they do not need to satisfy both.

---

## Iframe Permissions

Modern browsers deny sensitive features (camera, microphone, geolocation, etc.) to cross-origin iframes by default. To let an embedded app use these features, you must explicitly delegate them via the `permissions` field.

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

As a shortcut, you can set `permissions: [all]` to delegate every supported feature (auto-includes any new permissions Muximux adds later) or `permissions: [none]` to explicitly deny everything (same as omitting the field).

When `proxy: true` is set, Muximux delegates each permission to `'self'` (the proxy's own origin). For non-proxied apps, the permission is delegated to the app's specific origin (e.g. `camera 'self' https://meet.local`).

If `permissions` is omitted or empty, no features are delegated -- the browser's default-deny behaviour stays in effect.

> **Not listed**: `web-share` and `bluetooth` are valid Permissions-Policy directives in the spec but Chrome currently logs "Unrecognized feature" warnings when they appear in HTTP Permissions-Policy headers. They'll be re-added once browser support catches up.

---

## Notification Bridge

Browsers block the Web Notifications API in cross-origin iframes, even when the embedded app has notification permission at the OS level. Muximux can route notifications from embedded apps through its own top-level origin via a `postMessage` bridge.

**If your app's notifications don't appear when embedded, try setting `proxy: true` on the app. Proxied apps get a transparent Notifications API shim so most existing apps work with no code changes.**

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
When `proxy: true` is set, Muximux injects a `Notification` API shim into the app's HTML. Any call the app makes to the standard Web Notifications API is transparently forwarded to Muximux:

```javascript
// Inside the embedded app, works as if Muximux wasn't there:
new Notification('New message', { body: 'You have a new task' });

// Permission checks also "just work" (always returns granted):
if (Notification.permission === 'granted') { ... }
await Notification.requestPermission();
```

Most existing apps use exactly this pattern, so they light up immediately once `allow_notifications` is enabled.

**Tier 2: non-proxied apps (explicit bridge calls).**
When `proxy: false`, Muximux cannot inject code into the iframe (browsers enforce cross-origin isolation). The app must explicitly post a message to the parent window:

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
- Browsers only allow notifications in **secure contexts**. Muximux must be served over HTTPS or accessed via `localhost`/`127.0.0.1`. On plain HTTP (non-localhost), the browser permanently denies notifications and the bridge can do nothing about it.
- If the user denies the Muximux-origin permission prompt, nothing shows, but the shim still returns `'granted'` to the embedded app. The app will believe its notification fired.

> **Design note:** Because the permission belongs to Muximux's origin, any app you enable `allow_notifications` for can send notifications. Only enable this for apps you trust to send appropriate content.

## Appearance API

Apps embedded through the built-in reverse proxy can ask Muximux what language, theme, and colors the user currently has active -- so they can style themselves to match. Muximux exposes a single read-only endpoint:

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

**Proxied apps (`proxy: true`)** run at `/proxy/<slug>/` on Muximux's own origin, so a same-origin `fetch('/api/appearance')` automatically carries the Muximux session cookie. The app doesn't need to know about auth at all -- this is the zero-config path and the one most integrations should take.

**External apps** (cross-origin iframes, scripts, dashboards, scheduled jobs) can authenticate with an API key instead of a session cookie. Add the header:

```
X-Api-Key: your-api-key
```

to the request and Muximux accepts it for `GET /api/appearance` specifically. This is a dedicated bypass rule for this endpoint -- other `/api/*` endpoints still require a session cookie. If the key doesn't match (or isn't set) the request falls through to the normal auth path and gets a 401 / login redirect.

### Finding or creating the API key

The API key lives on disk as a **bcrypt hash** at `auth.api_key_hash` in `config.yaml`. The plaintext value is never stored -- once you set it, only the hash remains. Two ways to create one:

**In the UI:** **Settings → Security → API Key.** Paste or generate a key, save. Muximux hashes it before persisting.

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

Restart Muximux (or hot-reload via the UI) and the key is live. Give the plaintext value to your integration as its `X-Api-Key` header value.

> **Keep the key out of browser code.** `X-Api-Key` is a bearer token: anyone who sees it can read the endpoint. Do NOT embed it in JavaScript loaded by untrusted users -- put it on a server-side integration that fetches Muximux and passes the result to clients, or use the proxied-app flow where the session cookie does the work.

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

Your app's own stylesheet can then reference the same `--bg-surface`, `--text-primary`, etc. that Muximux uses, and the two will stay visually consistent.

### What's in `colors`

The fourteen names in `colors` are the curated, stable subset -- they have documented semantics and will not be renamed between Muximux versions. If you need a variable that isn't in this list, load `theme_css_url` and parse it yourself.

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

If `theme.variant === "system"` the operator has asked Muximux to follow the client's OS preference. Because the server can't know that preference, it returns the dark palette (`muximux`) as the default. An app that cares about this can use `matchMedia('(prefers-color-scheme: light)')` client-side and re-fetch with the opposite variant -- but in practice most apps just take whatever the endpoint returns and move on.

### Re-fetching on changes

The endpoint doesn't push updates. If the operator changes the theme while your app is open, your app stays on the old colors until it next fetches. That's deliberate: polling or a push channel would cost either battery or complexity for a feature that most apps just won't care to handle. If you do want to re-theme on every change, fetch again on `visibilitychange` -- it's enough for the common case of "user switches theme, then tabs back to the app".

### Limitations

- Proxied apps authenticate automatically via the session cookie (same-origin). External integrations need an `X-Api-Key` header; other `/api/*` endpoints are not reachable by API key.
- No push / event channel. Apps read once on boot.
- `colors` is a curated subset, not every theme variable. Use `theme_css_url` for the rest.
- `variant: "system"` resolves to the dark palette server-side; clients that care can check `prefers-color-scheme`.
- A cross-origin `fetch` with `X-Api-Key` may require a CORS preflight that Muximux doesn't answer today. If that bites, run the fetch from a server-side integration rather than directly from the browser.

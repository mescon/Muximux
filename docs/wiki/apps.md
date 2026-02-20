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
      variant: light                 # light or dark (dashboard icons only)
      file: ""                      # Filename (for type: custom)
      url: ""                       # URL (for type: url)
      color: "#ff9600"              # Optional icon tint color (Lucide only)
      background: "#ffffff"          # Optional icon background override
      invert: false                  # Invert icon colors (dark ↔ light)
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
    health_check: true               # Set false to disable health monitoring
    shortcut: 1                      # Assign keyboard shortcut 1-9
    min_role: ""                     # Minimum role to see this app (user, power-user, admin)
    force_icon_background: false     # Show icon background even when global setting is off
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

This is useful for:
- **Bookmarking** a specific app in your browser.
- **Sharing** a link that opens Muximux with a particular app already loaded.
- **Home screen shortcuts** on mobile devices.

When Muximux loads with a hash in the URL, it skips the splash screen and opens the matching app directly. If no app matches the hash, the normal startup behavior applies (default app or splash screen).

The URL hash updates automatically as you switch between apps, so you can copy the current URL at any time to get a direct link.

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

When authentication is enabled globally, you may need to allow certain paths of a proxied app to be accessed without logging in. The `auth_bypass` field lets you define exceptions:

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

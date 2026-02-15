# Configuration

## Overview

Muximux is configured via a single YAML file. By default, this is `data/config.yaml` (inside the data directory). The data directory defaults to `data/` and can be changed with `--data` or `MUXIMUX_DATA`. You can also override the config path directly:

```bash
# Change the data directory (config will be at /opt/muximux/data/config.yaml)
muximux --data /opt/muximux/data

# Or override the config path explicitly
muximux --config /path/to/config.yaml
```

## Complete Configuration Reference

```yaml
# ─── Server ─────────────────────────────────────
server:
  listen: ":8080"              # Listen address (default: ":8080")
  title: "Muximux"             # Page title shown in browser tab
  log_level: info              # Log verbosity: debug, info, warn, error
  proxy_timeout: 30s           # Global timeout for proxied requests

  # Optional: TLS / HTTPS
  tls:
    domain: ""                 # Auto-HTTPS domain (requires email)
    email: ""                  # Let's Encrypt registration email
    cert: ""                   # Path to TLS certificate PEM
    key: ""                    # Path to TLS private key PEM

  # Optional: Serve additional sites via Caddy
  gateway: ""                  # Path to Caddyfile

# ─── Authentication ─────────────────────────────
auth:
  method: none                 # none, builtin, forward_auth, oidc

  session_max_age: 24h         # Session duration (default: 24h)
  secure_cookies: false        # Require HTTPS for session cookies
  api_key: ""                  # Optional API key for X-Api-Key auth

  # Builtin auth users
  users:
    - username: ""
      password_hash: ""        # bcrypt hash (see Authentication page)
      role: admin              # admin or user
      email: ""                # Optional
      display_name: ""         # Optional

  # Forward auth settings
  trusted_proxies: []          # CIDR ranges, e.g., ["10.0.0.0/8"]
  headers:
    user: Remote-User
    email: Remote-Email
    groups: Remote-Groups
    name: Remote-Name          # Note: key is "name" not "display_name"

  # OIDC settings
  oidc:
    enabled: false
    issuer_url: ""
    client_id: ""
    client_secret: ""          # Supports ${ENV_VAR} syntax
    redirect_url: ""           # Must match provider callback URL
    scopes: [openid, profile, email]
    username_claim: preferred_username
    email_claim: email
    groups_claim: groups
    display_name_claim: name
    admin_groups: []           # Groups that grant admin role

# ─── Theme ─────────────────────────────────────
theme:
  family: default              # Theme family: default, nord, dracula, etc.
  variant: system              # dark, light, system (follow OS preference)

# ─── Navigation ─────────────────────────────────
navigation:
  position: top                # top, left, right, bottom, floating
  width: 220px                 # Sidebar width
  auto_hide: false
  auto_hide_delay: 3s
  show_on_hover: true
  show_labels: true
  show_logo: true
  show_app_colors: true
  show_icon_background: true
  show_splash_on_startup: false
  show_shadow: true

# ─── Icons ──────────────────────────────────────
icons:
  dashboard_icons:
    enabled: true
    mode: on_demand            # on_demand, prefetch, offline
    cache_dir: icons/dashboard
    cache_ttl: 7d

# ─── Health Monitoring ──────────────────────────
health:
  enabled: true
  interval: 30s
  timeout: 5s

# ─── Keyboard Shortcuts ────────────────────────
keybindings:
  bindings: {}                 # Custom overrides only; defaults managed client-side

# ─── Groups ─────────────────────────────────────
groups:
  - name: Media
    icon:
      type: lucide
      name: play
    color: "#e5a00d"
    order: 1
    expanded: true

# ─── Apps ───────────────────────────────────────
apps:
  - name: Plex
    url: http://plex:32400
    health_url: ""             # Optional custom health endpoint
    icon:
      type: dashboard
      name: plex
      variant: light
      color: ""
      background: ""
    color: "#e5a00d"
    group: Media
    order: 1
    enabled: true
    default: true
    open_mode: iframe          # iframe, new_tab, new_window, redirect
    proxy: false
    proxy_skip_tls_verify: true  # Skip TLS certificate verification (default: true)
    proxy_headers:               # Custom headers sent to the backend
      X-Custom-Header: value
    scale: 1.0
    disable_keyboard_shortcuts: false
    auth_bypass: []
    access:
      roles: []
      users: []
```

## Command Line Options

```
muximux [flags]
  --data PATH      Base data directory (default: data, env: MUXIMUX_DATA)
  --config PATH    Override config file path (default: <data>/config.yaml, env: MUXIMUX_CONFIG)
  --listen ADDR    Override listen address (env: MUXIMUX_LISTEN)
  --version        Show version and exit
```

Precedence: CLI flag > environment variable > config file value > default.

## Environment Variable Expansion

Use `${VARIABLE_NAME}` in any string value to reference environment variables. This is useful for keeping secrets out of config files:

```yaml
auth:
  oidc:
    client_secret: ${OIDC_CLIENT_SECRET}
  api_key: ${MUXIMUX_API_KEY}

apps:
  - name: Plex
    url: ${PLEX_URL}
```

If the referenced environment variable is not set, the value will be an empty string.

### Direct Override Environment Variables

These override the corresponding config file values without needing `${VAR}` syntax in config.yaml:

| Variable | Description | Default |
|----------|-------------|---------|
| `MUXIMUX_DATA` | Base data directory | `data` |
| `MUXIMUX_CONFIG` | Override config file path | `<data>/config.yaml` |
| `MUXIMUX_LISTEN` | Listen address (e.g., `:9090`) | From config file |

## Validation Rules

Muximux validates the configuration on startup and will refuse to start if the configuration is invalid. The following rules are enforced:

- If `tls.domain` is set, `tls.email` is required (for Let's Encrypt registration).
- `tls.cert` and `tls.key` must both be set or both empty. You cannot provide only one.
- `tls.domain` and `tls.cert` are mutually exclusive. Use either auto-HTTPS or manual certificates, not both.
- If `gateway` is set, the referenced Caddyfile must exist on disk.

## Live Configuration

Most settings can be changed through the Settings panel while Muximux is running. Changes are saved to `data/config.yaml` immediately and take effect without restarting.

The following settings **require a restart** to take effect:
- `server.listen` (listen address/port)
- `server.tls.*` (all TLS settings)
- `server.gateway` (Caddyfile path)
- `auth.method` (authentication method)

Everything else -- navigation, themes, apps, groups, icons, keybindings, health monitoring, log level -- is applied immediately.

## Config Export and Import

You can export your configuration as a downloadable YAML file and import it on another instance:

- **Export** strips sensitive data (password hashes, OIDC secrets, API keys) before download.
- **Import** parses the uploaded YAML, validates it, and shows a preview so you can review before applying.

This is available in the Settings panel under the General tab, or via the API (see [API Reference](api.md)).

## Config API

The configuration can also be read and updated via the API:

- `GET /api/config` -- Retrieve the current configuration (any authenticated user).
- `PUT /api/config` -- Update the full configuration (admin role required).

See the [API Reference](api.md) for details.

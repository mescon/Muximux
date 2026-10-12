# Configuration

## Overview

Muximux is configured via a single YAML file. By default, this is `data/config.yaml` (inside the data directory). The data directory defaults to `data/` relative to the binary's location and can be changed with `--data` or `MUXIMUX_DATA`. You can also override the config path directly:

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
  language: en                 # Interface language (BCP 47 tag, e.g. "en", "sv")
  log_level: info              # Log verbosity: debug, info, warn, error
  log_format: text             # Log output format: text or json
  proxy_timeout: 30s           # Global timeout for proxied requests
  base_path: ""                # Subpath for reverse proxy (e.g. "/muximux")

  # Optional: TLS / HTTPS
  tls:
    domain: ""                 # Auto-HTTPS domain (requires email)
    email: ""                  # Let's Encrypt registration email
    cert: ""                   # Path to TLS certificate PEM
    key: ""                    # Path to TLS private key PEM

  # Replaced by `gateway_sites:` below. If you're upgrading from
  # 3.0.x and still have this set, Muximux converts it to
  # `gateway_sites:` on first boot and saves backups of both the
  # original `config.yaml` and your Caddyfile alongside them.
  gateway: ""                  # legacy

  # Optional: Override the bind address for the embedded Caddy that
  # serves gateway sites. Useful when (a) you don't want to grant
  # CAP_NET_BIND_SERVICE / run as root, or (b) an upstream proxy
  # terminates TLS and you want Caddy on a high port behind it.
  # When set, gateway sites are emitted with http://<domain>:<port>
  # and auto-https is disabled. Empty = legacy behaviour (:80/:443).
  gateway_listen: ""           # e.g. ":8443"

  # Optional: cookie scope for cross-subdomain session sharing.
  # Required when any entry in gateway_sites has require_auth=true,
  # otherwise the session cookie won't reach the gated subdomain.
  # Leading dot is optional (browsers normalise).
  session_cookie_domain: ""    # e.g. ".example.com"

  # Optional: declarative gateway sites served by the embedded Caddy.
  # See the Gateway Examples and Gateway Auth Gate wiki pages for the
  # full reference. Each entry can optionally gate access behind the
  # Muximux session via require_auth + min_role + allowed_groups.
  gateway_sites:
    - domain: sonarr.example.com         # required
      backend_url: http://192.168.1.5:8989  # required
      tls: auto                          # auto | none | custom
      tls_cert: ""                       # required when tls=custom
      tls_key: ""                        # required when tls=custom
      streaming: false                   # disable Caddy response buffering for live data
      strip_frame_blockers: true         # strip X-Frame-Options for iframe embedding
      forwarded_headers: true            # forward X-Forwarded-Proto/Host/Real-IP
      backend_skip_tls_verify: false     # skip verifying a self-signed HTTPS backend (Proxmox); https backends only
      proxy_headers: {}                  # extra headers injected on upstream requests
      # Optional: also surface this site in the dashboard menu
      app_name: ""                       # link to an existing app by name
      # Gateway auth gate (3.1.0+) -- gate this subdomain behind Muximux login
      require_auth: false                # 302 anonymous visitors to /login first
      min_role: ""                       # "user" / "power-user" / "admin"; empty = any signed-in user
      allowed_groups: []                 # case-insensitive; admins bypass
      # Auto-populated by Docker discovery on import; do not hand-edit.
      # Presence of docker_key locks backend_url to the refresh poller.
      docker_key: ""
      docker_endpoint: ""
      docker_strategy: ""

# ─── Authentication ─────────────────────────────
auth:
  method: none                 # none, builtin, forward_auth, oidc
  setup_complete: false        # Set automatically after onboarding wizard completes

  session_max_age: 24h         # Session duration (default: 24h)
  secure_cookies: false        # Require HTTPS for session cookies
  api_key_hash: ""             # Optional bcrypt hash of API key for X-Api-Key auth

  # Builtin auth users
  users:
    - username: ""
      password_hash: ""        # bcrypt hash (see Authentication page)
      role: admin              # admin, power-user, or user
      email: ""                # Optional
      display_name: ""         # Optional
      groups: []               # Optional group memberships (for allowed_groups filtering)

  # Forward auth settings
  trusted_proxies: []          # CIDR ranges, e.g., ["192.168.0.0/16"]
  headers:
    user: Remote-User
    email: Remote-Email
    groups: Remote-Groups
    name: Remote-Name          # Note: key is "name" not "display_name"
  logout_url: ""               # Auth provider logout URL (e.g., https://auth.example.com/logout)
  forward_auth_admin_groups: []  # Groups that grant admin (default: admin, admins, administrators)

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
    provider_logout: false     # End the session at the provider on logout
    post_logout_redirect_url: "" # Default: <origin of redirect_url>/login?logged_out=1; register it at the provider
    logout_url: ""             # End-session URL for providers without one in discovery
    auto_redirect: false       # Login page goes straight to SSO (except after logout, ?local=1 or an error)
    disable_local_login: false # Refuse password sign-in while OIDC is on (API keys still work)

# ─── Theme ─────────────────────────────────────
theme:
  family: default              # Theme family: default, nord, dracula, etc.
  variant: system              # dark, light, system (follow OS preference)

# ─── Navigation ─────────────────────────────────
navigation:
  position: top                # top, left, right, bottom, floating
  width: 220px                 # Sidebar width
  auto_hide: false
  auto_hide_delay: 0.5s
  show_on_hover: true
  show_labels: true
  show_logo: true
  show_home_button: true       # Home button in the navigation (required for the splash screen)
  show_app_colors: true
  show_icon_background: false
  show_splash_on_startup: false
  show_shadow: true
  dynamic_tab_branding: false  # Browser tab shows the active app's name and icon
  icon_scale: 1.0              # Icon zoom multiplier
  bar_style: grouped           # grouped or flat (top/bottom bars only)
  floating_position: bottom-right  # bottom-right, bottom-left, top-right, top-left
  hide_sidebar_footer: false   # Hide the footer section in sidebars
  show_docker_badge: true      # Docker whale on Docker-discovered apps in the nav and overview (admins only)
  max_open_tabs: 0             # Max app tabs kept alive in memory (0 = unlimited)

# ─── Icons ──────────────────────────────────────
icons:
  dashboard_icons:
    enabled: true
    mode: on_demand              # on_demand, prefetch, offline
    cache_dir: icons/dashboard   # Where fetched dashboard icons are cached
    cache_ttl: 7d                # How long a cached icon is reused before refetch

# ─── Health Monitoring ──────────────────────────
health:
  enabled: true
  interval: 30s
  timeout: 5s

# ─── Docker Discovery (optional, opt-in) ───────
# Connect to a Docker daemon to discover running
# containers and keep their URLs current. See the
# Docker Discovery wiki page for the full guide.
discovery:
  docker:
    enabled: false                               # opt-in; requires socket access
    endpoint: unix:///var/run/docker.sock        # or tcp://host:2376
    network_strategy: container_ip               # container_ip | container_dns | host_port | host_docker_internal
    network_filter: ""                           # scope scans to one docker network when set
    host_ip: ""                                  # required by host_port strategy
    refresh_interval: 60s                        # poller cadence
    auto_import: off                             # off (default) | add | update | sync; auto-import muximux.*-labeled containers
    require_explicit_enable: false               # auto-import only containers labelled muximux.app.enabled=true
    lifecycle_enabled: false                     # allow start/stop/restart of tracked containers (requires :rw socket mount)
    lifecycle_min_role: admin                    # min role for lifecycle controls
    lifecycle_allowed_groups: []                 # additionally require group membership
    health_badge_placement: overview             # overview | overview_and_nav | off
    tls:                                         # only needed for tcp:// with mTLS
      enabled: false
      ca_cert: ""                                # CA bundle that signed the daemon cert
      client_cert: ""                            # client certificate (PEM)
      client_key: ""                             # client key (PEM, chmod 600)

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
    # Server-owned; written by Docker discovery, ignored in API payloads:
    # docker_managed: true     # group created by discovery; muximux.group.* labels apply
    # docker_order: true       # the group's order comes from a muximux.group.order label

# ─── Apps ───────────────────────────────────────
apps:
  - name: Plex
    url: http://plex:32400
    # health_url: http://plex:32400/web  # Optional custom health endpoint
    icon:
      type: dashboard
      name: plex
      # variant: light         # Icon variant (dashboard icons only)
      # file: healarr          # Filename in data/icons/ (type: custom only)
      # url: https://example.com/icon.png  # Remote image URL (type: url only)
      # color: "#ff0000"       # Tint color (Lucide only)
      # background: "#1a1a2e"  # Icon background override
      # invert: false          # Invert icon colors (dark ↔ light)
    color: "#e5a00d"
    group: Media
    order: 1
    enabled: true
    default: true
    pinned: false              # Show as a button in the top/bottom bar, ahead of the group dropdowns (stays in its group)
    open_mode: iframe          # iframe, new_tab, new_window, redirect, http_action
    proxy: false
    proxy_skip_tls_verify: true  # Skip TLS certificate verification (default: true)
    proxy_headers:               # Custom headers sent to the backend
      X-Custom-Header: value
    forwarded_headers: true      # Send X-Forwarded-For/Host/Proto and X-Real-IP to the backend; false sends none (for backends that reject them)
    scale: 1.0
    health_check: true           # Opt-in: set true to enable health monitoring for this app
    shortcut: null               # Assign a number key (1-9) for quick switching
    min_role: ""                 # Minimum role to see this app (user, power-user, admin)
    force_icon_background: false # Show icon background even when global setting is off
    auth_bypass: []
    access:
      roles: []
      users: []
    # Docker tracking; written by discovery (Discover import, auto-import, Re-link).
    # Server-owned: ignored in API payloads. Editing url by hand detaches the app.
    # docker_key: "label:plex"             # tracking key (label:, swarm:, compose:, name:, id:)
    # docker_endpoint: unix:///var/run/docker.sock
    # docker_strategy: container_ip
    # docker_managed_url: http://172.18.0.5:32400   # URL the poller last wrote
    # docker_managed_health_check: true    # value of the muximux.app.health_check label
    # docker_auto: true                    # created by auto-import (only these are updated or removed by it)
```

## Command Line Options

```
muximux [flags]
  --data PATH      Base data directory (default: data/ beside binary, env: MUXIMUX_DATA)
  --config PATH    Override config file path (default: <data>/config.yaml, env: MUXIMUX_CONFIG)
  --listen ADDR    Override listen address (env: MUXIMUX_LISTEN)
  --base-path PATH Subpath for reverse proxy (e.g. /muximux, env: MUXIMUX_BASE_PATH)
  --version        Show version and exit
```

Precedence: CLI flag > environment variable > config file value > default.

## Environment Variable Expansion

Use `${VARIABLE_NAME}` in any string value to reference environment variables. This is useful for keeping secrets out of config files:

```yaml
auth:
  oidc:
    client_secret: ${OIDC_CLIENT_SECRET}
  api_key_hash: ${MUXIMUX_API_KEY_HASH}

apps:
  - name: Plex
    url: ${PLEX_URL}
```

If the referenced environment variable is not set, the `${VAR}` reference is left as-is (the literal string is preserved). This avoids silently deleting values when an env var is missing.

## Environment Variable Reference

This is the complete list of environment variables Muximux reads. Most of them override a `config.yaml` setting without needing `${VAR}` syntax in the file.

### Process and logging

| Variable | Overrides | Values | Default |
|----------|-----------|--------|---------|
| `MUXIMUX_DATA` | Data directory (flag `--data`) | Path | `data/` beside the binary |
| `MUXIMUX_CONFIG` | Config file path (flag `--config`) | Path | `<data>/config.yaml` |
| `MUXIMUX_LISTEN` | `server.listen` (flag `--listen`) | Address, e.g. `:9090` | From config file |
| `MUXIMUX_BASE_PATH` | `server.base_path` (flag `--base-path`) | Path, e.g. `/muximux` | From config file (empty = root) |
| `MUXIMUX_LOG_LEVEL` | `server.log_level` | `debug`, `info`, `warn`, `error` | From config file |
| `MUXIMUX_LOG_FORMAT` | `server.log_format` | `text` or `json` | From config file |

The Docker image starts Muximux with `--data /app/data`, and a flag wins over its variable, so `MUXIMUX_DATA` has no effect in the container unless you replace the command.

### Docker discovery

| Variable | Overrides | Values | Default |
|----------|-----------|--------|---------|
| `MUXIMUX_DISCOVERY_AUTO_IMPORT` | `discovery.docker.auto_import` | `off`, `add`, `update`, `sync` | From config file |
| `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE` | `discovery.docker.require_explicit_enable` | Boolean | From config file |

An unknown `MUXIMUX_DISCOVERY_AUTO_IMPORT` value is treated as `off` with a warning, and so is a variable that is set but empty. An invalid `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE` value (including an empty one) is ignored with a warning and the config file value stays. See [Docker Discovery > Automatic Import](docker-discovery.md#automatic-import).

### Authentication and OIDC

| Variable | Overrides | Values |
|----------|-----------|--------|
| `MUXIMUX_AUTH_METHOD` | `auth.method` | `none`, `builtin`, `forward_auth`, `oidc` |
| `MUXIMUX_OIDC_ENABLED` | `auth.oidc.enabled` | Boolean |
| `MUXIMUX_OIDC_ISSUER_URL` | `auth.oidc.issuer_url` | URL |
| `MUXIMUX_OIDC_CLIENT_ID` | `auth.oidc.client_id` | String |
| `MUXIMUX_OIDC_CLIENT_ID_FILE` | `auth.oidc.client_id` | Path of a file holding the value (Docker secrets) |
| `MUXIMUX_OIDC_CLIENT_SECRET` | `auth.oidc.client_secret` | String |
| `MUXIMUX_OIDC_CLIENT_SECRET_FILE` | `auth.oidc.client_secret` | Path of a file holding the value (Docker secrets) |
| `MUXIMUX_OIDC_REDIRECT_URL` | `auth.oidc.redirect_url` | Absolute `http(s)` URL |
| `MUXIMUX_OIDC_SCOPES` | `auth.oidc.scopes` | List, separated by commas and/or spaces |
| `MUXIMUX_OIDC_USERNAME_CLAIM` | `auth.oidc.username_claim` | String |
| `MUXIMUX_OIDC_EMAIL_CLAIM` | `auth.oidc.email_claim` | String |
| `MUXIMUX_OIDC_GROUPS_CLAIM` | `auth.oidc.groups_claim` | String |
| `MUXIMUX_OIDC_DISPLAY_NAME_CLAIM` | `auth.oidc.display_name_claim` | String |
| `MUXIMUX_OIDC_ADMIN_GROUPS` | `auth.oidc.admin_groups` | List, separated by commas |
| `MUXIMUX_OIDC_PROVIDER_LOGOUT` | `auth.oidc.provider_logout` | Boolean |
| `MUXIMUX_OIDC_POST_LOGOUT_REDIRECT_URL` | `auth.oidc.post_logout_redirect_url` | Absolute `http(s)` URL |
| `MUXIMUX_OIDC_LOGOUT_URL` | `auth.oidc.logout_url` | Absolute `http(s)` URL |
| `MUXIMUX_OIDC_AUTO_REDIRECT` | `auth.oidc.auto_redirect` | Boolean |
| `MUXIMUX_OIDC_DISABLE_LOCAL_LOGIN` | `auth.oidc.disable_local_login` | Boolean |

Each defaults to the config file value. An empty or blank auth variable counts as unset. Setting both `X` and `X_FILE` is an error. Invalid auth configuration stops startup instead of leaving the instance open, for example an unknown `MUXIMUX_AUTH_METHOD`, or OIDC variables without `MUXIMUX_AUTH_METHOD` on an instance whose `config.yaml` is not already on OIDC. An unknown `MUXIMUX_OIDC_*` name is logged as a warning. The full rules, with examples, are in [Configuring OIDC with environment variables](authentication.md#configuring-oidc-with-environment-variables); Compose and Swarm examples with Docker secrets are in [Deployment](deployment.md#oidc-from-environment-variables-and-docker-secrets).

### Container image

These are read by the Docker image's entrypoint, not by Muximux itself:

| Variable | Purpose | Default |
|----------|---------|---------|
| `PUID`, `PGID` | User and group ID the process runs as, to match your bind-mounted `data/` directory | `1000` |
| `DOCKER_SOCKET` | Path of the Docker socket whose group the entrypoint detects | `/var/run/docker.sock` |
| `DOCKER_GID` | Force the group ID used for Docker socket access instead of detecting it | Detected |
| `TZ` | Time zone | `UTC` |

### Booleans, precedence and the memory-only rule

- **Booleans** accept `true`, `1`, `yes`, `on` and `false`, `0`, `no`, `off` (case-insensitive).
- **Precedence:** command-line flag, then environment variable, then the `config.yaml` value (including a `${VAR}` reference there), then the built-in default.
- **Memory only.** An override changes the running configuration only. Saving from Settings (or through the API) writes the value `config.yaml` already had, never the override, so removing the variable brings the file's value back on the next start. The one exception is a setup wizard completed under `MUXIMUX_AUTH_METHOD=builtin`, which saves `method: builtin` with the new admin.
- **Locked in Settings.** A field whose value comes from a variable or flag is shown locked, labelled with the variable's name ("From MUXIMUX_LISTEN", "Set by MUXIMUX_OIDC_CLIENT_SECRET_FILE"). The admin `GET /api/config` response lists these fields in `env_overrides` (field to variable name, never the value); see the [API Reference](api.md#configuration).
- **First start.** The overrides also apply when there is no `config.yaml` yet, so a container configured only through the environment starts with those values.
- **Restart.** Variables are read at startup. Change one and restart Muximux.

## Validation Rules

Muximux validates the configuration on startup and will refuse to start if the configuration is invalid. The following rules are enforced:

- If `tls.domain` is set, `tls.email` is required (for Let's Encrypt registration).
- `tls.cert` and `tls.key` must both be set or both empty. You cannot provide only one.
- `tls.domain` and `tls.cert` are mutually exclusive. Use either auto-HTTPS or manual certificates, not both.
- Legacy `gateway:` (Caddyfile path) configs from 3.0.x are auto-migrated to `gateway_sites:` on first boot under 3.1.0 (clean conversions only; lossy ones surface a hard error pointing at `muximux migrate-gateway`).
- If any `gateway_sites[]` entry has `require_auth: true`, `server.session_cookie_domain` is required and every gated site must be a subdomain of it (real host-suffix check: `evilexample.com` is not a subdomain of `example.com`).
- `gateway_sites[].min_role` must be one of `user`, `power-user`, `admin` when set (empty means "any authenticated user").
- `server.gateway_listen` must be a valid bind address (e.g., `:8443` or `127.0.0.1:8443`).

## Live Configuration

Most settings can be changed through the Settings panel while Muximux is running. Changes are saved to `data/config.yaml` immediately and take effect without restarting.

The following settings **require a restart** to take effect:
- `server.listen` (listen address/port)
- `server.base_path` (subpath prefix; must be a plain path such as `/muximux`: no leading `//`, backslashes, dot segments, whitespace, `?` or `#`)
- `server.tls.*` (all TLS settings)
- `server.gateway_listen` (overrides the bind port for the embedded Caddy)
- `server.session_cookie_domain` (cookie scope; takes effect on next login)
- `server.gateway_sites:` schema changes (the auth gate; backend swaps and TLS-mode flips reload Caddy in place)

When Muximux saves the configuration it writes defaults for fields you omitted, so a saved `config.yaml` can contain more keys than you wrote by hand. Settings saves are a three-way merge: changes made on the server while Settings is open (a Docker URL refresh, auto-import, another tab) are kept, and fields you did not touch keep the server's value. A save that would create two apps or groups with the same name is refused (409) and Settings blocks Save until one is renamed. Other open browsers get a live update and reload the config.

A restored backup is read exactly like `config.yaml` at startup: `${VAR}` expansion, defaults and validation. Restore reloads users, auth, OIDC, discovery and proxy routes at once; listen address, TLS, base-path routing and session cookie settings still need a restart.

Everything else -- navigation, themes, apps, groups, icons, keybindings, health monitoring, log level, log format -- is applied immediately.

## Config Export and Import

You can export your configuration as a downloadable YAML file and import it on another instance:

- **Export** strips sensitive data (password hashes, OIDC secrets, API key hashes) before download.
- **Import** parses the uploaded YAML, validates it, and shows a preview so you can review before applying.

This is available in the Settings panel under the General tab, or via the API (see [API Reference](api.md)).

## Config API

The configuration can also be read and updated via the API:

- `GET /api/config` -- Retrieve the current configuration (any authenticated user).
- `PUT /api/config` -- Update the full configuration (admin role required).
- `GET /api/appearance` -- Read-only snapshot of language, theme id, and theme colors, for embedded apps that want to sync their own appearance (see [Apps > Appearance API](apps.md#appearance-api)).

See the [API Reference](api.md) for details.

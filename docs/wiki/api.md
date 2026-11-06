# API Reference

## Overview

Muximux provides a REST API for managing configuration, apps, groups, health, authentication, icons, and themes. All endpoints return JSON.

When authentication is enabled, most endpoints need a valid session cookie. An API key (`X-Api-Key` header) works only on a short list of allowed paths. See [API key authentication](#authentication) below. Most write operations (POST, PUT, DELETE) need the `admin` role. Some do not: for example, login needs no authentication, and an app action uses the role gate of its app. The "Auth Required" column of each endpoint table gives the requirement.

---

## Authentication

| Endpoint | Method | Auth Required | Description |
|----------|--------|---------------|-------------|
| `/api/auth/login` | POST | No | Login with username/password |
| `/api/auth/logout` | POST | No | Logout (clears session) |
| `/api/auth/status` | GET | No | Check auth status and current user |
| `/api/auth/me` | GET | Yes | Get current user details |
| `/api/auth/password` | POST | Yes | Change password (builtin auth only) |
| `/api/auth/users` | GET | Admin | List all users |
| `/api/auth/users` | POST | Admin | Create a new user |
| `/api/auth/users/{username}` | PUT | Admin | Update user role/email/display name |
| `/api/auth/users/{username}` | DELETE | Admin | Delete a user |
| `/api/auth/method` | PUT | Admin | Switch authentication method |
| `/api/auth/api-key` | GET | Admin | Report whether an instance API key is configured (never returns the key) |
| `/api/auth/api-key` | POST | Admin | Generate or rotate the instance API key |
| `/api/auth/api-key` | DELETE | Admin | Clear the instance API key |
| `/api/auth/setup` | POST | No (but requires `X-Setup-Token`) | Initial setup (onboarding wizard). See [Authentication > First-Run Setup](authentication.md#first-run-setup) for the token. |
| `/api/auth/oidc/login` | GET | No | Redirect to OIDC provider |
| `/api/auth/oidc/callback` | GET | No | OIDC callback handler |
| `/api/auth/oidc/backchannel-logout` | POST | No (signed logout token) | OIDC back-channel logout. See [Authentication > Back-channel logout](authentication.md#back-channel-logout) |
| `/api/auth/settings/oidc` | GET | Admin | Current OIDC settings (the client secret is reported only as `client_secret_set`), with `env_fields` for fields owned by the environment |
| `/api/auth/settings/oidc/test` | POST | Admin | Fetch an issuer's discovery document (`{"issuer_url": "..."}`) and report which endpoints it offers |

**Login request:**
```json
POST /api/auth/login
{
  "username": "admin",
  "password": "secretpassword"
}
```

**Login response:**
```json
{
  "success": true,
  "user": {
    "username": "admin",
    "role": "admin",
    "email": "admin@example.com",
    "display_name": "Admin User"
  }
}
```

**Auth status response:**
```json
GET /api/auth/status

{
  "authenticated": true,
  "auth_method": "builtin",
  "oidc_enabled": false,
  "setup_required": false,
  "user": {
    "username": "admin",
    "role": "admin",
    "email": "admin@example.com",
    "display_name": "Admin User"
  }
}
```

The `auth_method` field returns the active authentication method: `none`, `builtin`, `forward_auth`, or `oidc`. The `user` field is only present when the request is authenticated. The response includes a `logout_url` field when `auth_method` is `forward_auth` and a logout URL is configured. The client uses it to redirect on sign-out. The response also carries `oidc_auto_redirect` and `local_login` (whether the password form is offered). While the method comes from the environment, it also carries `auth_method_env` (the variable's name, for example `MUXIMUX_AUTH_METHOD`).

**Before setup completes.** On a new install, only these endpoints answer until the onboarding wizard finishes: `GET /api/auth/status`, `POST /api/auth/setup`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`, the `/api/auth/oidc/*` sign-in routes, `GET /api/health`, `POST /api/config/restore`, and the read-only theme list and icon routes the wizard uses. Every other endpoint returns `503` with `{"error": "setup_required"}`. This includes user and API key management, the auth method and OIDC settings, the log viewer, `/api/system/*`, the `/ws` live stream and the `/proxy/` app routes.

**API key authentication:**

Non-browser integrations can authenticate with an API key via the `X-Api-Key` header. This works **only on allowlisted paths**, not on every `/api/*` endpoint. Out of the box, the allowlist is:

- `GET /api/appearance` -- read Muximux's current theme + language (see [Apps > Appearance API](apps.md#appearance-api))
- Any proxied-app path the operator has opted into via an `auth_bypass` rule with a `path` and `require_api_key: true` (see [Apps > Per-App Auth Bypass](apps.md#per-app-auth-bypass))

Example, calling Sonarr through Muximux with a configured bypass:

```
GET /proxy/sonarr/api/v3/series
X-Api-Key: your-api-key-here
```

State-changing endpoints like `/api/config`, `/api/apps`, `/api/themes`, and `/api/auth/users` need a session cookie. The API key does not work against them. See [Authentication > API Key Authentication](authentication.md#api-key-authentication) for the full scope and setup instructions.

### User Management

**List users:**
```
GET /api/auth/users
```

Returns an array of users (without password hashes):
```json
[
  {"username": "admin", "role": "admin", "email": "admin@example.com", "display_name": "Admin User"},
  {"username": "viewer", "role": "user", "email": "", "display_name": ""}
]
```

**Create user:**
```json
POST /api/auth/users
{
  "username": "newuser",
  "password": "minimum8chars",
  "role": "user",
  "email": "user@example.com",
  "display_name": "New User"
}
```

Validation: the username is required. The password must be at least 8 characters. Valid roles: `admin`, `power-user`, `user`. If the role is omitted or invalid, it defaults to `user`.

**Update user:**
```json
PUT /api/auth/users/newuser
{
  "role": "admin",
  "email": "updated@example.com",
  "display_name": "Updated Name"
}
```

All fields are optional. Muximux updates only the fields you send.

**Delete user:**
```
DELETE /api/auth/users/newuser
```

Constraints: you cannot delete your own account. You also cannot delete the last admin user.

### Auth Method Switching

**Switch authentication method:**
```json
PUT /api/auth/method
{
  "method": "forward_auth",
  "trusted_proxies": ["192.168.0.0/16"],
  "headers": {
    "user": "Remote-User",
    "email": "Remote-Email",
    "groups": "Remote-Groups",
    "name": "Remote-Name"
  },
  "logout_url": "https://auth.example.com/logout"
}
```

Valid methods: `builtin`, `forward_auth`, `oidc`, `none`. Switching to `builtin` requires at least one user to exist. Switching to `forward_auth` requires `trusted_proxies`. Switching to `oidc` takes the provider settings in an `oidc` object. Muximux validates these settings and runs discovery (fetches the provider's endpoint list) before the switch. The optional `logout_url` sets where the browser redirects on sign-out, to clear the auth provider's session. The change takes effect immediately without a restart.

This endpoint does not change settings owned by the environment. While `MUXIMUX_AUTH_METHOD` is set, Muximux refuses a different method with `409 Conflict` and a message naming the variable. (Sending the current method to edit other settings is fine.) Muximux refuses two more cases the same way:

- choosing `oidc` while `MUXIMUX_OIDC_ENABLED=false`;
- leaving `oidc` while OIDC variables are set without `MUXIMUX_AUTH_METHOD`.

OIDC fields set by a `MUXIMUX_OIDC_*` variable or a `${VAR}` reference keep their value, whatever the request sends. `GET /api/auth/settings/oidc` lists them in `env_fields` (field to variable name, never the value). `enabled` appears there when `MUXIMUX_AUTH_METHOD=oidc` or `MUXIMUX_OIDC_ENABLED` sets it. While the method comes from the environment, `GET /api/auth/status` includes `auth_method_env` (the variable name), and `POST /api/auth/setup` refuses any other method with `409`. See [Configuring OIDC with environment variables](authentication.md#configuring-oidc-with-environment-variables).

---

## Configuration

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/config` | GET | Any | Get full configuration |
| `/api/config` | PUT | Admin | Update the configuration (three-way merge when `base` is sent) |
| `/api/config/export` | GET | Admin | Download config as YAML (sensitive data stripped) |
| `/api/config/import` | POST | Admin | Parse and validate uploaded YAML, returns preview |
| `/api/config/restore` | POST | Pre-setup only (requires `X-Setup-Token`) | Replace the live config with an uploaded YAML during first-run onboarding; parsed like `config.yaml` at startup |
| `/api/appearance` | GET | Any authenticated user, or any caller with `X-Api-Key` | Read-only snapshot of language + theme + colors for embedded apps |

The PUT endpoint accepts the full configuration object. Changes to most settings take effect immediately. Server-level settings (listen address, TLS, gateway) need a restart.

**Saving with `base` (three-way merge).** The request body may carry an optional `base`: the config the client loaded before editing. When `base` is present, Muximux merges the edit onto the current server config. Muximux applies only the fields you changed. Everything else keeps the server's value. Changes made while you were editing are therefore kept (a Docker URL refresh, auto-import, a gateway cookie scope, another tab's save). The merge may produce two apps or two groups with the same name. In that case, Muximux refuses the save with `409 Conflict` and a message naming the conflict. Without `base`, the payload replaces the config (the old two-way behaviour). Omitted fields are reset, and an omitted `proxy_timeout` clears it. Scripts that `PUT /api/config` without `base` keep working unchanged.

Apps and groups in the payload may carry `original_name`, the name they had when loaded. It tells the merge which server item matches an edited item. A rename therefore keeps the app's access rules, Docker tracking and forwarded-headers setting. `POST /api/groups` and `PUT /api/group/{name}` ignore `original_name`.

The admin `GET /api/config` response includes `env_overrides`. This is a map of field to environment variable name. It lists settings currently overridden from the environment or a command-line flag (for example `server.listen` from `MUXIMUX_LISTEN`). Overrides apply in memory only. Muximux never writes them to `config.yaml`. The Settings dialog shows these fields as locked. You can make auth method changes live via `PUT /api/auth/method`.

**Export config:**
```
GET /api/config/export
```
Returns a downloadable YAML file. The file has password hashes, OIDC client secrets, and API keys removed. The filename includes the current date (e.g., `muximux-config-2025-01-15.yaml`).

**Import config (preview):**
```
POST /api/config/import
Content-Type: application/x-yaml

(YAML body, max 1 MB)
```
Validates the YAML. Returns the parsed config as JSON for preview. The frontend can then apply it via `PUT /api/config`. Validation requires at least one app with a name and URL.

**Restore config (first-run only):**
```
POST /api/config/restore
Content-Type: application/x-yaml
X-Setup-Token: <token from server stdout or data/.setup-token>

(YAML body, max 1 MB)
```
Replaces the live config during the initial onboarding flow. The uploaded YAML goes through the same load path as `config.yaml` at startup. Muximux expands `${VAR}` references (and keeps them as references in the file). It applies defaults and validates the result. Muximux refuses a backup that still uses the legacy `server.gateway` setting with `400`. If Muximux cannot reach the OIDC provider, it clears the provider, so OIDC login stays off until you fix it. Muximux reloads users, auth, OIDC, discovery and proxy routes immediately. It drops all sessions. It refuses a login that is in progress during the restore. Listen address, TLS, base-path routing and session cookie settings still need a restart. Muximux accepts this call only while the instance is in the pre-setup state. Later calls return `409 Conflict`. A missing or invalid `X-Setup-Token` returns `401`. See [Authentication > First-Run Setup](authentication.md#first-run-setup) for how to find the token.

**Appearance (read-only):**
```
GET /api/appearance
```
Returns a JSON snapshot with `language`, `theme` (`family`, `variant`, `id`, `is_dark`), `colors` (a curated subset of CSS custom-property values), and `theme_css_url`. Embedded apps call this once on boot. They use it to sync their own styling with Muximux. See [Apps > Appearance API](apps.md#appearance-api) for the full contract and usage examples.

---

## Apps

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/apps` | GET | Any | List apps visible to the caller, projected for their role (admins see everything; others see enabled apps that pass `min_role` and `allowed_groups`, with `proxy_headers`, `http_action_headers`, the Docker tracking fields and URL credentials removed) |
| `/api/apps` | POST | Admin | Create a new app (Docker tracking fields in the payload are ignored; only discovery import attaches an app to a container) |
| `/api/app/{name}` | GET | Any | Get one app, with the same visibility rule and role projection as the list; answers 404 for an app the caller could not list |
| `/api/app/{name}` | PUT | Admin | Update app (full replace; omitted fields are reset) |
| `/api/app/{name}` | DELETE | Admin | Delete app |
| `/api/app-action/{name}` | POST | Per-app role gate | Fire the app's configured http_action HTTP request |
| `/api/app-docker/{name}/{start\|stop\|restart}` | POST | Role-gated | Control a Docker-tracked container (requires lifecycle_enabled and a daemon that accepts writes) |

**Create app request:**
```json
POST /api/apps
{
  "name": "Sonarr",
  "url": "http://sonarr:8989",
  "icon": {
    "type": "dashboard",
    "name": "sonarr"
  },
  "group": "Media",
  "enabled": true,
  "open_mode": "iframe"
}
```

---

## Groups

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/groups` | GET | Any | List all groups |
| `/api/groups` | POST | Admin | Create a new group |
| `/api/group/{name}` | GET | Any | Get group by name |
| `/api/group/{name}` | PUT | Admin | Update group (full replace; omitted fields are reset) |
| `/api/group/{name}` | DELETE | Admin | Delete group |

Admins also receive `docker_managed: true` on a group that Docker discovery created and still manages from `muximux.group.*` labels. This applies in these endpoints and in `GET /api/config`. The marker is server-owned: `POST /api/groups`, `PUT /api/group/{name}` and `PUT /api/config` ignore it in the payload. Admins also receive `docker_order: true` while the group's order comes from a `muximux.group.order` label. This marker is server-owned in the same way. An update that changes the group's `icon` or `color` clears `docker_managed`. After that, the labels no longer apply to that group. A change to `order` (a drag in Settings, for example) clears it only when `docker_order` is set.

---

## Discovery

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/discovery/docker/status` | GET | Admin | Docker daemon reachability + discovery config |
| `/api/discovery/docker/networks` | GET | Admin | List Docker networks for URL resolution |
| `/api/discovery/docker/config` | GET | Admin | Read the stored discovery config (incl. auto_import mode) |
| `/api/discovery/docker/config` | PUT | Admin | Update discovery config; merges onto the stored config (incl. auto_import mode, lifecycle_enabled) |
| `/api/discovery/docker/test` | POST | Admin | Test a discovery config without saving |
| `/api/discovery/docker/scan` | GET | Admin | Scan the daemon, return importable containers (with `labeled`, `label_enabled` and `auto_import_skip`) |
| `/api/discovery/docker/import` | POST | Admin | Import selected containers as apps |
| `/api/discovery/docker/tracked` | GET | Admin | List apps currently tracked from Docker, and quarantined entries |
| `/api/discovery/docker/track/{key}` | DELETE | Admin | Detach tracking for a key; also deletes quarantined entries for the key on any endpoint. `?scope=quarantined` deletes only the quarantined entries |
| `/api/discovery/docker/relink/probe` | POST | Admin | Probe a container to re-link a detached app |
| `/api/discovery/docker/relink/confirm` | POST | Admin | Confirm a re-link |
| `/api/discovery/docker-state` | GET | Any | Current container-state map for tracked apps |

`GET /api/discovery/docker/config` returns the stored values under `config`. This includes fields the Discovery tab does not edit. `env_overrides` is present only when an environment variable sets a field's live value. It maps the field to the variable's name (the tab shows such fields as locked):

```json
{
  "config": {"enabled": true, "endpoint": "unix:///var/run/docker.sock", "network_strategy": "container_ip", "host_ip": "", "refresh_interval": "60s", "auto_import": "off", "require_explicit_enable": false, "health_badge_placement": "overview"},
  "env_overrides": {"auto_import": "MUXIMUX_DISCOVERY_AUTO_IMPORT", "require_explicit_enable": "MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE"}
}
```

`PUT` takes the bare config object (without the `config` wrapper).

`PUT` merges onto the stored config. Fields you leave out (such as `auto_import` or the TLS paths) are therefore kept. A field set by an environment variable keeps its live value, whatever the request sends. Muximux accepts `npipe://` endpoints on Windows. An enabled config with an empty endpoint gets the platform default. The response is the new discovery status, as from `GET /api/discovery/docker/status`.

`GET /api/discovery/docker/scan` returns these fields per container:

- `labeled` (any `muximux.*` label);
- `label_enabled` (the value of `muximux.app.enabled`, absent when unset);
- `auto_import_skip` (`{code, detail}`, present when auto-import would not add the container). The codes are `unlabeled`, `disabled`, `not_enabled`, `no_port`, `no_url`, `invalid`.

The scan omits containers opted out with `muximux.app.enabled=false` from the rows. It counts them in `opted_out`. A container that the config already tracks (matched by key, on any endpoint) carries `tracked` (`{kind, name, auto_imported, endpoint?}`). In it, `kind` is `app`, `site` or `quarantined`, and `name` is the app name or site domain. `endpoint` is present only when the entry is tracked against a different Docker daemon than the one scanned. Untracked containers have no `tracked` field. `name_taken` is `true` on an untracked container whose name matches an existing app. An import under that name would collide.

`GET /api/discovery/docker/tracked` returns two fields:

- `entries`: each has an optional `missing_since` timestamp, set while the container cannot be found.
- `quarantined`: an array of `{kind, name, key, reason}` for invalid Docker-owned entries. These entries stay in `config.yaml` but are not loaded, until they are removed or a live entry supersedes them. See [Quarantined entries](docker-discovery.md#quarantined-entries).

Tracking keys are `label:<id>`, `swarm:<service>`, `compose:<project>:<service>`, `name:<name>` or `id:<id>`. `DELETE /api/discovery/docker/track/{key}` also removes quarantined entries for the key, whatever their endpoint. To remove only the quarantined entries, add `?scope=quarantined`. A live app or gateway site with the same key then stays tracked. The call returns 404 when no quarantined entry has the key. Any other `scope` value is a 400.

**Server-owned Docker fields.** Only discovery writes these fields (Discover import, auto-import, the refresh poller and **Re-link**). Muximux returns them to admins only. It ignores any value in a request payload (`POST /api/apps`, `PUT /api/app/{name}`, `PUT /api/config`, the group and gateway site endpoints):

| Object | Field | Meaning |
|---|---|---|
| App, gateway site | `docker_key`, `docker_endpoint`, `docker_strategy` | The container the entry tracks (key such as `label:sonarr` or `swarm:media_sonarr`), the daemon and the network strategy |
| App, gateway site | `docker_managed_url` | The URL (or `backend_url`) the poller last wrote; a different URL in a save detaches the entry |
| App | `docker_managed_health_check` | The value of `muximux.app.health_check` last applied; while set, the app form shows the health check locked |
| Group | `docker_managed`, `docker_order` | See [Groups](#groups) |

A save can detach an app or site by changing its URL. A save cannot attach, re-attach or move one. Muximux stores whether auto-import created an app as `docker_auto`, in `config.yaml` only. This field is not part of the API (the scan's `tracked.auto_imported` reports it).

Auto-import is opt-in via `discovery.docker.auto_import` (`off`, `add`, `update`, or `sync`). The discovery configuration is also part of the full configuration object. You can therefore set it via `PUT /api/config` as well as `PUT /api/discovery/docker/config`.

---

## Health

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Simple health probe (for load balancers) |
| `/api/apps/health` | GET | Get health status for all apps |
| `/api/apps/{name}/health` | GET | Get health status for one app |
| `/api/apps/{name}/health/check` | POST | Trigger immediate health check |

The `/api/health` endpoint always returns 200 OK when the server is running. It is meant for load balancers and uptime monitors.

**Health status response:**
```json
{
  "name": "Sonarr",
  "status": "healthy",
  "response_time_ms": 42,
  "last_check": "2024-01-15T10:30:00Z",
  "uptime_percent": 99.8,
  "check_count": 1440,
  "success_count": 1437
}
```

**Status values:**
- `healthy` -- Last check received an HTTP 2xx response
- `unhealthy` -- Last check failed (error, timeout, or non-2xx response)
- `unknown` -- App has not been checked yet

---

## Icons

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/icons/dashboard` | GET | Any | List dashboard icons |
| `/api/icons/dashboard/{name}` | GET | Any | Get dashboard icon metadata |
| `/api/icons/lucide` | GET | Any | List Lucide icons |
| `/api/icons/lucide/{name}` | GET | Any | Get Lucide icon SVG |
| `/api/icons/custom` | GET | Any | List custom icons |
| `/api/icons/custom` | POST | Admin | Upload custom icon (multipart, 2MB limit) |
| `/api/icons/custom/fetch` | POST | Admin | Download icon from URL and save locally |
| `/api/icons/custom/{name}` | DELETE | Admin | Delete custom icon |

**Upload custom icon:**

Send a multipart form request with the icon file:

```
POST /api/icons/custom
Content-Type: multipart/form-data

file: (binary data)
```

Accepted formats: PNG, SVG, JPG, WebP, GIF. Maximum file size: 2MB.

**Fetch icon from URL:**

Download an image from a remote URL and save it as a custom icon:

```
POST /api/icons/custom/fetch
Content-Type: application/json

{
  "url": "https://example.com/icon.png",
  "name": "my-icon"              // optional, derived from URL if omitted
}
```

The server downloads the image. It validates the content type and size (2MB limit), and stores the image locally. Returns `{"name": "my-icon", "status": "uploaded"}` on success.

---

## Themes

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/themes` | GET | Any | List all themes |
| `/api/themes` | POST | Admin | Save custom theme |
| `/api/themes/{name}` | DELETE | Admin | Delete custom theme |

---

## Logs

| Endpoint | Method | Auth Required | Description |
|----------|--------|---------------|-------------|
| `/api/logs/recent` | GET | Admin | Get recent log entries from the in-memory buffer (audit entries, client IPs, and panic stacks make this admin-only) |

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 200 | Maximum number of entries to return |
| `level` | string | | Filter by log level (`debug`, `info`, `warn`, `error`) |
| `source` | string | | Filter by source tag (`http`, `audit`, `server`, `auth`, `health`, `proxy`, `websocket`, `caddy`, `config`, `icons`, `themes`, `discovery`, `gateway`, `system`) |

**Example:**
```
GET /api/logs/recent?limit=50&level=error
```

**Response:**
```json
[
  {
    "timestamp": "2026-01-01T12:00:00Z",
    "level": "INFO",
    "message": "HTTP request",
    "source": "http",
    "attrs": {
      "method": "GET",
      "path": "/api/config",
      "status": "200",
      "latency_ms": "12",
      "request_id": "a1b2c3d4e5f6a7b8"
    }
  }
]
```

Muximux stores logs in a 1000-entry ring buffer (a fixed-size list that overwrites its oldest entries). When the buffer is full, Muximux drops the oldest entries. Muximux also broadcasts entries in real-time via WebSocket (see below).

---

## System

| Endpoint | Method | Auth Required | Description |
|----------|--------|---------------|-------------|
| `/api/system/info` | GET | Yes | Get system information (admin-only fields like data_dir, environment, and uptime are scrubbed for non-admin callers) |
| `/api/system/updates` | GET | Admin | Check for available updates |

**System info response:**
```json
{
  "version": "3.0.0",
  "commit": "abc1234",
  "build_date": "2025-01-15T00:00:00Z",
  "go_version": "go1.26.0",
  "os": "linux",
  "arch": "amd64",
  "uptime": "2d 5h 30m",
  "environment": "docker"
}
```

**Update check response:**
```json
{
  "current_version": "3.1.1",
  "latest_version": "3.2.0",
  "update_available": true,
  "release_url": "https://github.com/mescon/Muximux/releases/tag/v3.2.0",
  "changelog": "..."
}
```

---

## Proxy Status

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/proxy/status` | GET | Get Caddy proxy status |

**Response:**
```json
{
  "enabled": true,
  "running": true,
  "tls": true,
  "domain": "muximux.example.com"
}
```

---

## Gateway

The declarative `server.gateway_sites:` model is the current gateway path (the legacy Caddyfile was removed in 3.1.0).

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/gateway/sites` | GET | Admin | List configured gateway sites |
| `/api/gateway/sites` | POST | Admin | Create a gateway site (Docker tracking fields in the payload are ignored) |
| `/api/gateway/sites/{domain}` | PUT | Admin | Update a gateway site; validated like a startup load. Accepts an optional `base_backend_url` (the backend address the client loaded; if unchanged, the server's current value and Docker tracking are kept). Docker tracking fields (`docker_key`, `docker_endpoint`, `docker_strategy`, `docker_managed_url`) are server-owned and ignored in the payload |
| `/api/gateway/sites/{domain}` | DELETE | Admin | Delete a gateway site |
| `/api/gateway/validate` | POST | Admin | Validate a gateway site config without saving |

`/api/auth/forward` is the internal forward-auth verify endpoint. The gateway uses it to gate access. Do not call it directly from a browser (a direct hit returns `400`).

---

## WebSocket

| Endpoint | Protocol | Description |
|----------|----------|-------------|
| `/ws` | WebSocket | Real-time event stream |

Connect to `/ws` to receive real-time updates. Muximux sends events as JSON messages:

```json
{"type": "config_updated", "payload": {}}
{"type": "health_changed", "payload": [...]}
{"type": "app_health_changed", "payload": {"app": "Sonarr", "health": {"status": "healthy", ...}}}
```

**Event types:**

| Type | Payload | Description |
|------|---------|-------------|
| `config_updated` | Empty | Configuration was changed (Settings, API, import, auto-import). Sent to every connected client, which refetches `GET /api/config`. An open Settings dialog keeps unsaved edits and rebases them onto the new state |
| `health_changed` | Array of health statuses | Health status changed for one or more apps |
| `app_health_changed` | `{"app": "name", "health": {...}}` | Health status changed for a specific app |
| `docker_state_changed` | `{"app_name": "name", "state": {...}}` | Container state changed for a Docker-tracked app (status, health, restart count, etc.) |
| `log_entry` | `LogEntry` object | A new log entry was recorded (see Logs section above). Admin-only: only broadcast to admin clients |

If the connection drops, the WebSocket client automatically reconnects. It uses exponential backoff (a wait that grows after each failed try), up to 30 seconds between retries, max 10 attempts.

---

## Rate Limiting

The following endpoints allow only **5 attempts per IP address per minute**. This prevents brute-force attacks. If a client exceeds the limit, the endpoint returns HTTP 429 (Too Many Requests).

- `POST /api/auth/login` -- Login attempts
- `POST /api/auth/setup` -- Initial setup attempts
- `GET /api/auth/oidc/login` -- OIDC login redirects (shares the login limiter)

Other API endpoints are not rate-limited.

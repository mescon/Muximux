# API Reference

## Overview

Muximux provides a REST API for managing configuration, apps, groups, health, authentication, icons, and themes. All endpoints return JSON.

When authentication is enabled, most endpoints require a valid session cookie or API key (`X-Api-Key` header). Write operations (POST, PUT, DELETE) require the `admin` role.

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
| `/api/auth/setup` | POST | No | Initial setup (onboarding wizard) |
| `/api/auth/oidc/login` | GET | No | Redirect to OIDC provider |
| `/api/auth/oidc/callback` | GET | No | OIDC callback handler |

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

The `auth_method` field returns the active authentication method: `none`, `builtin`, `forward_auth`, or `oidc`. The `user` field is only present when the request is authenticated.

**API key authentication:**

Instead of a session cookie, you can authenticate with an API key by including the `X-Api-Key` header:

```
GET /api/apps
X-Api-Key: your-api-key-here
```

The API key is configured in `auth.api_key` in your config file.

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

Validation: username is required, password must be at least 8 characters. Valid roles: `admin`, `power-user`, `user`. If the role is omitted or invalid, it defaults to `user`.

**Update user:**
```json
PUT /api/auth/users/newuser
{
  "role": "admin",
  "email": "updated@example.com",
  "display_name": "Updated Name"
}
```

All fields are optional -- only provided fields are updated.

**Delete user:**
```
DELETE /api/auth/users/newuser
```

Constraints: you cannot delete your own account, and you cannot delete the last admin user.

### Auth Method Switching

**Switch authentication method:**
```json
PUT /api/auth/method
{
  "method": "forward_auth",
  "trusted_proxies": ["10.0.0.0/8"],
  "headers": {
    "user": "Remote-User",
    "email": "Remote-Email",
    "groups": "Remote-Groups",
    "name": "Remote-Name"
  }
}
```

Valid methods: `builtin`, `forward_auth`, `none`. Switching to `builtin` requires at least one user to exist. Switching to `forward_auth` requires `trusted_proxies`. The change takes effect immediately without a restart.

---

## Configuration

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/config` | GET | Any | Get full configuration |
| `/api/config` | PUT | Admin | Update full configuration |
| `/api/config/export` | GET | Admin | Download config as YAML (sensitive data stripped) |
| `/api/config/import` | POST | Admin | Parse and validate uploaded YAML, returns preview |

The PUT endpoint accepts the full configuration object. Changes to most settings take effect immediately. Server-level settings (listen address, TLS, gateway) require a restart. Auth method changes can be made live via `PUT /api/auth/method`.

**Export config:**
```
GET /api/config/export
```
Returns a downloadable YAML file with password hashes, OIDC client secrets, and API keys removed. The filename includes the current date (e.g., `muximux-config-2025-01-15.yaml`).

**Import config (preview):**
```
POST /api/config/import
Content-Type: application/x-yaml

(YAML body, max 1 MB)
```
Validates the YAML and returns the parsed config as JSON for preview. The frontend can then apply it via `PUT /api/config`. Validation requires at least one app with a name and URL.

---

## Apps

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/apps` | GET | Any | List all apps |
| `/api/apps` | POST | Admin | Create a new app |
| `/api/app/{name}` | GET | Any | Get app by name |
| `/api/app/{name}` | PUT | Admin | Update app |
| `/api/app/{name}` | DELETE | Admin | Delete app |

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
| `/api/group/{name}` | PUT | Admin | Update group |
| `/api/group/{name}` | DELETE | Admin | Delete group |

---

## Health

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Simple health probe (for load balancers) |
| `/api/apps/health` | GET | Get health status for all apps |
| `/api/apps/{name}/health` | GET | Get health status for one app |
| `/api/apps/{name}/health/check` | POST | Trigger immediate health check |

The `/api/health` endpoint always returns 200 OK when the server is running. It is intended for use with load balancers and uptime monitors.

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
| `/api/icons/custom` | POST | Admin | Upload custom icon (multipart, 5MB limit) |
| `/api/icons/custom/{name}` | DELETE | Admin | Delete custom icon |

**Upload custom icon:**

Send a multipart form request with the icon file:

```
POST /api/icons/custom
Content-Type: multipart/form-data

file: (binary data)
```

Accepted formats: PNG, SVG, JPG, WebP. Maximum file size: 5MB.

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
| `/api/logs/recent` | GET | Yes | Get recent log entries from the in-memory buffer |

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 200 | Maximum number of entries to return |
| `level` | string | | Filter by log level (`debug`, `info`, `warn`, `error`) |
| `source` | string | | Filter by source tag (`server`, `proxy`, `health`, `auth`, `websocket`, `caddy`, `config`, `icons`, `themes`) |

**Example:**
```
GET /api/logs/recent?limit=50&level=error
```

**Response:**
```json
[
  {
    "timestamp": "2025-01-15T14:23:01.234Z",
    "level": "error",
    "message": "Dial failed: connection refused",
    "source": "proxy"
  }
]
```

Logs are stored in a 1000-entry ring buffer. When the buffer is full, the oldest entries are dropped. Entries are also broadcast in real-time via WebSocket (see below).

---

## System

| Endpoint | Method | Auth Required | Description |
|----------|--------|---------------|-------------|
| `/api/system/info` | GET | No | Get system information |
| `/api/system/updates` | GET | No | Check for available updates |

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
  "current_version": "3.0.0",
  "latest_version": "3.1.0",
  "update_available": true,
  "release_url": "https://github.com/mescon/Muximux/releases/tag/v3.1.0",
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

## WebSocket

| Endpoint | Protocol | Description |
|----------|----------|-------------|
| `/ws` | WebSocket | Real-time event stream |

Connect to `/ws` to receive real-time updates. Events are sent as JSON messages:

```json
{"type": "config_updated", "payload": {...}}
{"type": "health_changed", "payload": [...]}
{"type": "app_health_changed", "payload": {"app": "Sonarr", "health": {"status": "healthy", ...}}}
```

**Event types:**

| Type | Payload | Description |
|------|---------|-------------|
| `config_updated` | Full config object | Configuration was changed (via Settings panel or API) |
| `health_changed` | Array of health statuses | Health status changed for one or more apps |
| `app_health_changed` | `{"app": "name", "health": {...}}` | Health status changed for a specific app |
| `log_entry` | `LogEntry` object | A new log entry was recorded (see Logs section above) |

The WebSocket client automatically reconnects if the connection drops, using exponential backoff (up to 30 seconds between retries, max 10 attempts).

---

## Rate Limiting

The following endpoints are rate-limited to **5 attempts per IP address per minute** to prevent brute-force attacks. If the limit is exceeded, the endpoint returns HTTP 429 (Too Many Requests).

- `POST /api/auth/login` -- Login attempts
- `POST /api/auth/setup` -- Initial setup attempts

Other API endpoints are not rate-limited.

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

**API key authentication:**

Instead of a session cookie, you can authenticate with an API key by including the `X-Api-Key` header:

```
GET /api/apps
X-Api-Key: your-api-key-here
```

The API key is configured in `auth.api_key` in your config file.

---

## Configuration

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/api/config` | GET | Any | Get full configuration |
| `/api/config` | PUT | Admin | Update full configuration |

The PUT endpoint accepts the full configuration object. Changes to most settings take effect immediately. Server-level settings (listen address, TLS, gateway) and auth method changes require a restart.

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
{"type": "config_updated", "data": {...}}
{"type": "health_changed", "data": [...]}
{"type": "app_health_changed", "data": {"name": "Sonarr", "status": "healthy", ...}}
```

**Event types:**

| Type | Description |
|------|-------------|
| `config_updated` | Configuration was changed (via Settings panel or API) |
| `health_changed` | Health status changed for one or more apps |
| `app_health_changed` | Health status changed for a specific app |

The WebSocket client automatically reconnects if the connection drops, using exponential backoff up to 10 retry attempts.

---

## Rate Limiting

The login endpoint (`POST /api/auth/login`) is rate-limited to **5 attempts per IP address per minute** to prevent brute-force attacks. If the limit is exceeded, the endpoint returns HTTP 429 (Too Many Requests).

Other API endpoints are not rate-limited.

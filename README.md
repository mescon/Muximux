# Muximux

A modern, self-hosted portal to your web applications. One binary, one port, one config file.

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

## Features

- **Single Binary** - No database required, configuration via YAML
- **Single Port** - One `listen` address for everything; TLS and gateway handled transparently
- **Integrated Reverse Proxy** - Proxies apps through `/proxy/{slug}/`, stripping headers that block iframe embedding
- **Real-time Health** - WebSocket-based health monitoring with live status indicators
- **Multiple Auth Methods** - Built-in users, forward auth (Authelia/Authentik), or OIDC
- **1,600+ Icons** - Lucide icons with category search, plus [dashboard-icons](https://github.com/homarr-labs/dashboard-icons)
- **Responsive UI** - Modern Svelte 5 frontend with Tailwind CSS
- **Easy Deployment** - Docker or native binary

## Quick Start

### Docker (Recommended)

```bash
mkdir -p data
cp config.example.yaml data/config.yaml
# Edit data/config.yaml with your apps

docker run -d \
  --name muximux \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  ghcr.io/mescon/muximux:latest
```

### Binary

```bash
./muximux --config config.yaml
```

### Development

```bash
git clone https://github.com/mescon/Muximux.git
cd muximux3

# Start development servers
docker compose -f docker-compose.dev.yml up

# Or run natively:
cd web && npm install && npm run dev &
go run ./cmd/muximux --config config.yaml
```

## Configuration

Muximux is configured via a YAML file. See [config.example.yaml](config.example.yaml) for all options.

### Server

```yaml
server:
  listen: ":8080"    # The port you access Muximux on
  title: "Muximux"
```

### Apps

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
    icon:
      type: dashboard
      name: sonarr
    color: "#3498db"
    group: Downloads
    order: 1
    enabled: true
    default: false
    open_mode: iframe       # iframe, new_tab, new_window, redirect
    proxy: true             # Proxy through /proxy/sonarr/ (see below)
    scale: 1                # Zoom level (0.5-2.0)
```

### Groups

```yaml
groups:
  - name: Media
    icon:
      type: lucide
      name: play
    color: "#e5a00d"
    order: 1
```

### Navigation

```yaml
navigation:
  position: top          # top, left, right, bottom, floating
  width: 220px
  auto_hide: false
  auto_hide_delay: 3s
  show_labels: true
  show_logo: true
```

---

## Integrated Reverse Proxy

When `proxy: true` is set on an app, Muximux proxies all requests through `/proxy/{app-slug}/` on the **same port**. This works automatically with zero extra configuration.

**What it does:**
- Strips `X-Frame-Options` and `Content-Security-Policy` headers that block iframe embedding
- Rewrites URLs in HTML, CSS, and JavaScript so the app works correctly under the proxy path
- Rewrites `Set-Cookie` paths, `Location` redirects, and `<base href>` tags
- Handles gzipped responses transparently

**Use this when:** an app refuses to load in an iframe (most apps set `X-Frame-Options: DENY`).

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
    proxy: true              # Now accessible at /proxy/sonarr/
    open_mode: iframe
```

The integrated proxy runs on the Go server itself and does **not** require TLS, Caddy, or any other configuration. It works in every deployment mode.

---

## TLS / HTTPS

Muximux serves on a single port. To add HTTPS, configure `tls` under `server`. An embedded [Caddy](https://caddyserver.com/) server starts automatically and handles the user-facing port, forwarding to Go internally.

### Auto-HTTPS (Let's Encrypt)

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"
    email: "admin@example.com"
```

Caddy manages certificate issuance and renewal automatically. It will also listen on ports 80 and 443 for the ACME challenge.

### Manual Certificates

```yaml
server:
  listen: ":8080"
  tls:
    cert: /path/to/cert.pem
    key: /path/to/key.pem
```

Muximux serves HTTPS on the configured `listen` port using your certificates. Use `domain` **or** `cert`/`key`, not both.

---

## Gateway

Serve additional sites alongside Muximux through the same Caddy instance, using standard [Caddyfile](https://caddyserver.com/docs/caddyfile) syntax:

```yaml
server:
  listen: ":8080"
  gateway: /path/to/sites.Caddyfile
```

The referenced Caddyfile is imported into Caddy's configuration. You can use this to reverse-proxy other services, serve static files, or add any Caddy-supported functionality.

---

## How It All Fits Together

| `tls` | `gateway` | What happens |
|-------|-----------|-------------|
| No | No | Go serves on `listen` directly. No Caddy. Simplest mode. |
| Yes | No | Caddy serves HTTPS on `listen`, Go on an internal port. |
| No | Yes | Caddy serves HTTP on `listen` + extra sites, Go on internal port. |
| Yes | Yes | Caddy serves HTTPS on `listen` + extra sites, Go on internal port. |

The internal port is computed automatically (`listen` port + 10000, e.g., `:8080` becomes `127.0.0.1:18080`). It is never user-configured.

The per-app `proxy: true` setting (integrated reverse proxy) works in **all four modes** above. It runs on the Go server and is independent of Caddy.

---

## Authentication

### No Auth (Default)
```yaml
auth:
  method: none
```

### Built-in Users
```yaml
auth:
  method: builtin
  session_max_age: 24h
  secure_cookies: true
  users:
    - username: admin
      password_hash: "$2a$10$..."  # bcrypt hash
      role: admin
    - username: user
      password_hash: "$2a$10$..."
      role: user
```

Generate password hashes:
```bash
./muximux hashpw
# Or in Docker:
docker exec muximux ./muximux hashpw
```

### Forward Auth (Authelia/Authentik)
```yaml
auth:
  method: forward_auth
  trusted_proxies:
    - 10.0.0.0/8
    - 172.16.0.0/12
  headers:
    user: Remote-User
    email: Remote-Email
    groups: Remote-Groups
```

### OIDC
```yaml
auth:
  method: oidc
  oidc:
    issuer: https://auth.example.com
    client_id: muximux
    client_secret: ${OIDC_CLIENT_SECRET}
    redirect_url: https://muximux.example.com/auth/callback
    scopes:
      - openid
      - profile
      - email
```

---

## Health Monitoring

```yaml
health:
  enabled: true
  interval: 30s
  timeout: 5s
```

Health status is displayed as colored indicators on app icons and broadcast via WebSocket for real-time updates.

## Icons

Muximux supports multiple icon sources:

```yaml
icon:
  type: dashboard     # From dashboard-icons project
  name: plex
  variant: light      # light, dark, or empty for default

icon:
  type: lucide        # Lucide icon library (~1,600 icons)
  name: server

icon:
  type: url           # Custom URL
  url: https://example.com/icon.png

icon:
  type: custom        # Uploaded file
  file: custom-icon.png
```

## Environment Variables

Configuration values can reference environment variables:

```yaml
auth:
  oidc:
    client_secret: ${OIDC_CLIENT_SECRET}
```

## API

### Apps
- `GET /api/apps` - List all apps
- `POST /api/apps` - Create app
- `GET /api/app/:name` - Get app
- `PUT /api/app/:name` - Update app
- `DELETE /api/app/:name` - Delete app

### Groups
- `GET /api/groups` - List all groups
- `POST /api/groups` - Create group
- `GET /api/group/:name` - Get group
- `PUT /api/group/:name` - Update group
- `DELETE /api/group/:name` - Delete group

### Config
- `GET /api/config` - Get configuration
- `PUT /api/config` - Update configuration

### Health
- `GET /api/apps/health` - Get all health statuses
- `GET /api/apps/:name/health` - Get app health

### Auth
- `POST /api/auth/login` - Login
- `POST /api/auth/logout` - Logout
- `GET /api/auth/status` - Auth status
- `GET /api/auth/me` - Current user

### WebSocket
- `GET /ws` - Real-time events

```json
{"type": "health_update", "data": {"app": "sonarr", "status": "healthy"}}
{"type": "config_change", "data": {"section": "apps"}}
```

## Architecture

```
muximux3/
├── cmd/muximux/      # Main entrypoint
├── internal/
│   ├── config/       # YAML configuration + validation
│   ├── server/       # HTTP server, routing, middleware
│   ├── handlers/     # API handlers + integrated reverse proxy
│   ├── health/       # Health monitoring
│   ├── websocket/    # WebSocket hub for real-time events
│   ├── auth/         # Authentication (builtin, forward_auth, OIDC)
│   ├── proxy/        # Caddy TLS/gateway management
│   ├── icons/        # Dashboard Icons + Lucide client
│   └── logging/      # Structured logging
└── web/              # Svelte 5 frontend
    ├── src/
    │   ├── components/
    │   ├── lib/      # Stores, types, API client
    │   └── App.svelte
    └── dist/         # Built assets (embedded in binary)
```

## Building

```bash
# Build frontend
cd web && npm run build && cd ..

# Build binary
go build -o muximux ./cmd/muximux

# Build with version info
go build -ldflags "-X main.version=1.0.0" -o muximux ./cmd/muximux
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Credits

- [Lucide Icons](https://lucide.dev/) for the icon library
- [Dashboard Icons](https://github.com/homarr-labs/dashboard-icons) by Homarr Labs
- [Caddy](https://caddyserver.com/) for TLS and gateway support

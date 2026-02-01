# Muximux 3

A modern rewrite of [Muximux](https://github.com/mescon/Muximux) - a lightweight portal to your web applications.

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

## Features

- **Single Binary** - No database required, configuration via YAML
- **Embedded Proxy** - Built-in Caddy-powered reverse proxy for iframe embedding
- **Real-time Health** - WebSocket-based health monitoring with live updates
- **Multiple Auth** - Built-in users, forward auth (Authelia/Authentik), or OIDC
- **Dashboard Icons** - Automatic icon fetching from [dashboard-icons](https://github.com/walkxcode/dashboard-icons)
- **Responsive UI** - Modern Svelte 5 frontend with Tailwind CSS
- **Easy Deployment** - Docker or native binary

## Quick Start

### Docker (Recommended)

```bash
# Create data directory and config
mkdir -p data
cp config.example.yaml data/config.yaml
# Edit data/config.yaml with your apps

# Run with Docker
docker run -d \
  --name muximux \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  ghcr.io/mescon/muximux3:latest
```

### Binary

```bash
# Download the latest release
# https://github.com/mescon/muximux3/releases

# Run
./muximux --config config.yaml
```

### Development

```bash
# Clone and install dependencies
git clone https://github.com/mescon/muximux3.git
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
  listen: ":8080"
  title: "Muximux"
```

### Apps

```yaml
apps:
  - name: Plex
    url: http://localhost:32400
    icon:
      type: dashboard    # Uses dashboard-icons
      name: plex
    color: "#e5a00d"
    group: Media
    order: 1
    enabled: true
    default: true        # Opens by default
    open_mode: iframe    # iframe, new_tab, new_window, redirect
    proxy: false         # Enable embedded proxy
    scale: 1             # Zoom level (0.5-2.0)
```

### Groups

```yaml
groups:
  - name: Media
    icon: play
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

### Embedded Proxy

The embedded proxy uses Caddy to serve applications that block iframe embedding:

```yaml
proxy:
  enabled: true
  listen: ":8443"
  auto_https: true
  acme_email: admin@example.com
  # Or use custom certs:
  # tls_cert: /path/to/cert.pem
  # tls_key: /path/to/key.pem
```

When `proxy: true` is set on an app, Muximux proxies requests through `/proxy/{app-slug}/` and strips X-Frame-Options/CSP headers that would prevent iframe embedding.

### Authentication

#### No Auth (Default)
```yaml
auth:
  method: none
```

#### Built-in Users
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

#### Forward Auth (Authelia/Authentik)
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

#### OIDC
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

### Health Monitoring

```yaml
health:
  enabled: true
  interval: 30s
  timeout: 5s
```

Health status is displayed as colored indicators on app icons and broadcast via WebSocket for real-time updates.

## API

### Apps
- `GET /api/apps` - List all apps
- `GET /api/apps/:name` - Get app by name
- `POST /api/apps` - Create app
- `PUT /api/apps/:name` - Update app
- `DELETE /api/apps/:name` - Delete app

### Groups
- `GET /api/groups` - List all groups
- `POST /api/groups` - Create group
- `PUT /api/groups/:name` - Update group
- `DELETE /api/groups/:name` - Delete group

### Config
- `GET /api/config` - Get configuration
- `PUT /api/config` - Update configuration

### Health
- `GET /api/health` - Get all health statuses
- `GET /api/health/:name` - Get app health status

### Auth
- `POST /api/auth/login` - Login
- `POST /api/auth/logout` - Logout
- `GET /api/auth/me` - Get current user

### WebSocket
- `GET /ws` - WebSocket connection for real-time events

Events:
```json
{"type": "health_update", "data": {"app": "plex", "status": "healthy"}}
{"type": "config_change", "data": {"section": "apps"}}
```

## Icons

Muximux supports multiple icon sources:

```yaml
icon:
  type: dashboard     # From dashboard-icons project
  name: plex
  variant: light      # light, dark, or empty for default

icon:
  type: builtin       # Built-in icons
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

## Architecture

```
muximux3/
├── cmd/muximux/      # Main entrypoint
├── internal/
│   ├── config/       # YAML configuration
│   ├── server/       # HTTP server & routing
│   ├── handlers/     # API handlers
│   ├── health/       # Health monitoring
│   ├── websocket/    # WebSocket hub
│   ├── auth/         # Authentication
│   ├── proxy/        # Embedded Caddy proxy
│   └── logging/      # Structured logging
└── web/              # Svelte frontend
    ├── src/
    │   ├── components/
    │   ├── lib/      # Stores & utilities
    │   └── App.svelte
    └── dist/         # Embedded in binary
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

- Original [Muximux](https://github.com/mescon/Muximux) by mescon
- [Dashboard Icons](https://github.com/walkxcode/dashboard-icons) by walkxcode
- [Caddy](https://caddyserver.com/) for the embedded proxy

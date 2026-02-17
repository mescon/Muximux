# Changelog

All notable changes to Muximux are documented in this file.

## [Unreleased]

### Added

- **Flat bar style** for top/bottom navigation — a streamlined layout that shows apps in a single row separated by group icon dividers, without group headers or collapsible sections.
- **Per-app keyboard shortcut assignment** — assign number keys (1–9) to specific apps instead of relying on position-based ordering. Configured via `shortcut` field on each app or in Settings > Keybindings.
- **Per-app health check toggle** — disable health monitoring for individual apps with `health_check: false`. Useful for apps that don't respond to HTTP checks or where you don't care about status. Includes bulk enable/disable in Settings.
- **Base path support** — serve Muximux at a subpath behind a reverse proxy (e.g., `https://example.com/muximux/`). Configure with `server.base_path` in config, `--base-path` CLI flag, or `MUXIMUX_BASE_PATH` environment variable.

---

## [3.0.0] - 2025-02-15

### Ground-Up Rewrite

Muximux v3 is a complete rewrite. The original PHP bookmark portal has been replaced with a Go backend and Svelte frontend, shipped as a single binary with no runtime dependencies.

### New Features

**Core**
- Single binary deployment with embedded frontend -- no PHP, no web server, no database
- YAML-based configuration with environment variable expansion (`${VAR}`)
- Guided onboarding wizard for first-run setup with live preview
- Data directory (`data/`) for config, themes, icons, and logs
- System info and update check API endpoints

**Built-in Reverse Proxy**
- Per-app proxy that strips iframe-blocking headers and rewrites HTML, CSS, JS paths
- WebSocket proxy support for live-updating apps
- Per-app TLS skip, custom headers, and configurable timeout
- Gzip-aware content rewriting and SRI neutralization
- Separate from TLS/gateway (Caddy) -- works in every deployment mode

**Authentication**
- Built-in username/password auth with bcrypt
- Forward auth support (Authelia, Authentik)
- OIDC provider integration
- User management with roles (admin, user, guest)
- Live auth method switching without restart
- API key authentication for programmatic access
- Rate-limited login and setup endpoints

**TLS & Gateway**
- Automatic HTTPS via Let's Encrypt (embedded Caddy)
- Manual certificate support
- Gateway mode to serve additional sites alongside Muximux via Caddyfile

**Real-Time Log Viewer**
- In-app log viewer with level and source filtering
- Real-time streaming via WebSocket
- Search, auto-scroll, pause/resume, and log download
- Hot-reloadable log level (debug, info, warn, error)
- Persistent log file with rotation

**Health Monitoring**
- Periodic health checks with configurable interval and timeout
- Real-time status updates via WebSocket
- Custom health check URLs per app
- Manual health check trigger via API

**Navigation & Layout**
- 5 navigation positions: top, left, right, bottom, floating
- Auto-hide with configurable delay
- Collapsible groups with drag-and-drop reordering
- Command palette with fuzzy search (`/` or `Ctrl+K`)

**Themes**
- 9 built-in theme families with dark/light variants: Default, Nord, Dracula, Catppuccin, Solarized, Tokyo Night, Gruvbox, Plex, Rose Pine
- System variant follows OS dark/light preference
- Custom theme editor with CSS custom properties
- Theme import/export

**Icons**
- 1,600+ Lucide icons
- Thousands of Dashboard Icons (on-demand, prefetch, or offline caching)
- Custom icon uploads (PNG, SVG, JPG, WebP)
- URL-based icons
- Per-icon color tinting and background

**Keyboard Shortcuts**
- Configurable keybindings for all actions
- Per-app shortcut disabling for apps with their own shortcuts
- Number keys (1-9) for quick app switching

**Config Export/Import**
- Export configuration as YAML with sensitive data stripped
- Import and preview before applying

**Developer Experience**
- REST API with full CRUD for apps, groups, config, health, auth, icons, and themes
- WebSocket event stream for real-time updates
- Cross-platform builds (Linux, macOS, Windows; amd64, arm64, arm)
- Docker multi-arch images
- CI with linting, testing, security scanning, and code coverage

### Migration from v2

Muximux v3 is not backwards-compatible with v2. The PHP application has been replaced entirely. Start fresh with the onboarding wizard or create a new `config.yaml` from `config.example.yaml`.

## [3.0.0-beta.1] - 2024-12-01

Beta release with reverse proxy improvements, onboarding redesign, WebSocket proxy support, and CI/CD hardening.

## [3.0.0-alpha.1] - 2024-11-15

Initial alpha release with core dashboard, authentication, health monitoring, icons, themes, keyboard shortcuts, and reverse proxy.

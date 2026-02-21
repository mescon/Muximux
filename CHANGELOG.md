# Changelog

All notable changes to Muximux are documented in this file.

## [3.0.0] - 2026-02-21

### Ground-Up Rewrite

Muximux v3 is a complete rewrite. The original PHP bookmark portal has been replaced with a Go backend and Svelte frontend, shipped as a single binary with no runtime dependencies.

### New Features

**Core**
- Single binary deployment with embedded frontend -- no PHP, no web server, no database
- YAML-based configuration with braced `${VAR}` environment variable expansion (literal `$` signs in values like bcrypt hashes are safe)
- Guided onboarding wizard for first-run setup with live preview
- Restore from backup on the onboarding welcome screen -- import an existing `config.yaml` to skip the wizard entirely
- Data directory (`data/`) for config, themes, icons, and logs -- resolves relative to the binary location regardless of working directory
- System info and update check API endpoints
- Docker PUID/PGID support for bind-mount permission compatibility (linuxserver.io convention)
- Docker security hardening: `init: true`, `no-new-privileges`, `cap_drop: ALL`
- Base path support for reverse proxy subpaths (e.g. `https://example.com/muximux/`) via config, CLI flag, or `MUXIMUX_BASE_PATH` env var

**Built-in Reverse Proxy**
- Per-app proxy that strips iframe-blocking headers and rewrites HTML, CSS, JS paths
- Runtime URL interceptor for JavaScript-constructed URLs -- patches `fetch()`, `XMLHttpRequest`, `WebSocket`, `EventSource`, and DOM property setters so proxied SPAs work correctly
- Content-type-aware rewriting: full path rewriting for HTML/CSS, safe-only rewriting (SRI stripping, absolute URLs) for JS/JSON/XML to avoid corrupting API data
- WebSocket proxy support for live-updating apps
- Per-app TLS skip, custom headers, and configurable timeout
- Gzip-aware content rewriting and SRI neutralization
- Dynamic proxy route rebuilds -- adding, editing, or removing a proxied app takes effect immediately without restart
- Separate from TLS/gateway (Caddy) -- works in every deployment mode

**Authentication**
- Built-in username/password auth with bcrypt
- Forward auth support (Authelia, Authentik) with dedicated external authentication login page
- OIDC provider integration
- User management with roles (admin, power-user, user)
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
- Opt-in per-app health checks with configurable interval and timeout
- TLS certificate verification skipped for health checks to support self-signed certs common in homelabs
- Real-time status updates via WebSocket
- Custom health check URLs per app
- Manual health check trigger via API
- Bulk enable/disable in Settings

**Navigation & Layout**
- 5 navigation positions: top, left, right, bottom, floating
- Mobile-responsive: automatically switches to floating navigation on small screens (< 640px)
- Flat bar style for top/bottom navigation -- apps in a single row with group icon dividers
- Auto-hide with configurable delay
- Collapsible groups with drag-and-drop reordering
- Command palette with fuzzy search (`/` or `Ctrl+K`)
- Iframe caching -- visited app iframes stay in the DOM for instant switching without reload, lost scroll position, or re-authentication
- Hash-based app routing -- URL hash links to specific apps (e.g. `#plex`) with browser back/forward navigation
- Dynamic themed favicons matching the current theme's accent color

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
- Per-app shortcut assignment with number keys (1-9)
- Per-app shortcut disabling for apps with their own shortcuts

**Config Export/Import**
- Export configuration as YAML with sensitive data stripped
- Import and preview before applying
- Restore from backup during onboarding

**Debug Tools**
- Browser debug logging via `?debug=true` URL parameter across all subsystems (config, websocket, auth, theme, health, icons, keybindings)
- Persists via localStorage; disable with `?debug=false`

**Developer Experience**
- REST API with full CRUD for apps, groups, config, health, auth, icons, and themes
- WebSocket event stream for real-time updates
- Cross-platform builds (Linux, macOS, Windows; amd64, arm64, arm)
- Docker multi-arch images
- CI with linting, testing, security scanning, and code coverage
- systemd service file for bare-metal deployments with security hardening
- CONTRIBUTING.md developer guide

### Migration from v2

Muximux v3 is not backwards-compatible with v2. The PHP application has been replaced entirely. Start fresh with the onboarding wizard or create a new `config.yaml` from `config.example.yaml`.

[3.0.0]: https://github.com/mescon/Muximux/releases/tag/v3.0.0

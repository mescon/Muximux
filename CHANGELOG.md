# Changelog

All notable changes to Muximux are documented in this file.

## [Unreleased]

### Added

- **Runtime URL interceptor for reverse proxy** — Proxied SPAs (like Plex) that construct URLs dynamically in JavaScript now work correctly. The proxy injects a script that patches `fetch()`, `XMLHttpRequest`, `WebSocket`, `EventSource`, and DOM property setters (`img.src`, etc.) so all requests route through the proxy automatically.
- **Content-type-aware rewriting** — HTML and CSS get full path rewriting; JS, JSON, and XML use safe-only rewriting (SRI stripping, absolute URLs) to avoid corrupting API data that apps read programmatically.
- **Dynamic proxy route rebuilds** — Adding, editing, or removing a proxied app in Settings takes effect immediately without restarting Muximux.
- **Hash-based app routing** — Clicking an app updates the URL hash (e.g., `#plex`), allowing direct links to specific apps and browser back/forward navigation between them.
- **Debug logging wiki page** — New [Debug Logging](docs/wiki/debug-logging.md) documentation covering frontend (`?debug=true`) and backend (log level) diagnostics.

### Fixed

- **Health tooltip showing nanoseconds** — Health check response times in tooltips now correctly display milliseconds instead of raw nanosecond values.
- **Proxy 404 on double-prefixed URLs** — XML/JSON API responses from proxied apps no longer have root-relative paths statically rewritten, preventing double-prefixing when the SPA embeds those paths in query parameters (e.g., Plex photo transcode URLs).
- **Proxied app images invisible due to frozen iframe timeline** — Chrome may freeze `document.timeline` inside iframes, stalling CSS/Web Animations. The interceptor detects loaded images stuck at opacity 0 and forces them visible.

---

## [3.0.0-rc.3] - 2026-02-18

### Changed

- **Build tag split** — Go builds no longer require a `dist/` placeholder directory. Dev builds compile without `embed_web` tag; production builds use `-tags embed_web` to embed frontend assets.
- **Docker PUID/PGID support** — Container entrypoint now creates a runtime user matching `PUID`/`PGID` environment variables for bind-mount permission compatibility (linuxserver.io convention).
- **Docker security hardening** — `docker-compose.yml` adds `init: true`, `no-new-privileges`, and `cap_drop: ALL`.
- **Settings modal refactored** — Extracted each tab (General, Apps, Theme, Security, About) into its own component, reducing Settings.svelte from ~3800 lines to ~1800.
- **Button styles standardized** — All buttons in Settings now use the design system classes (`.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`) instead of hand-rolled Tailwind.
- **Open mode labels** — Consistent display between Add and Edit flows; both now use shared `openModes` constant.

### Added

- **Debug logging** — Add `?debug=true` to the URL to enable browser console logging across all major subsystems (config, websocket, auth, theme, health, icons, keybindings). Persists via localStorage; disable with `?debug=false`.
- **Cancel button on Edit modals** — Edit App and Edit Group modals now have a Cancel button that reverts changes. Previously only "Done" was available, which applied changes immediately.
- **Validation on Edit modals** — Edit App and Edit Group modals now validate with Zod schemas before accepting, matching the Add flows.
- **Redirect open mode in UI** — The `redirect` open mode is now available in the Settings dropdown (previously only configurable via YAML).
- **`.btn-danger` design system class** — For destructive action buttons (delete confirmations).
- **`--accent-on-primary` theme variable** — Dedicated text color for accent-colored buttons, ensuring readable contrast in both dark and light themes.
- **Docstring coverage enforcement** — CI checks that 80%+ of exported Go identifiers have doc comments (`scripts/check-docstrings.sh`).
- **CHANGELOG-based release notes** — Release workflow extracts notes from CHANGELOG.md instead of auto-generating from PR titles. Falls back to auto-generation if no entry found.
- **CONTRIBUTING.md** — Developer guide covering prerequisites, dev mode, building, testing, and PR process.
- **systemd service file** — `muximux.service` for bare-metal deployments with security hardening.
- **CodeRabbit config** — `.coderabbit.yaml` with path-specific review instructions.
- **Codecov config** — `codecov.yml` with backend/frontend flags, patch target 70%, and carryforward support.
- **Dynamic themed favicons** — All favicons (browser tab, apple-touch-icon, Android manifest icon, theme-color meta) now update to match the current theme's accent color instead of using static green PNGs.
- **Snyk Node scan** — CI security workflow now scans frontend npm dependencies in addition to Go and Docker.

### Fixed

- **Config env var expansion corrupting bcrypt hashes** — Replaced `os.ExpandEnv` with braced-only `${VAR}` expansion so bare `$` signs in bcrypt hashes and other values are not treated as variable references.
- **Unset `${VAR}` silently replaced with empty string** — `${VAR}` references to undefined environment variables are now preserved literally instead of being silently deleted.
- **Config export zeroing live password hashes** — Exporting config (`GET /api/config/export`) no longer corrupts in-memory auth state. The shallow struct copy now deep-copies the users slice before stripping sensitive fields.
- **Config save race between API and auth handlers** — Both handlers now share a single `sync.RWMutex` for all config reads and writes, preventing concurrent saves from silently overwriting each other.
- **GetApps and GetGroups missing read lock** — These endpoints now acquire the config read lock, preventing data races with concurrent config writes.
- **Single-app update overwriting proxied app URL** — `PUT /api/app/{name}` now preserves the original backend URL for proxied apps instead of saving the frontend proxy path.
- **App rename via bulk save dropping auth rules** — Renaming an app in Settings no longer loses its AuthBypass and Access rules; a positional fallback matches renamed apps to their original config.
- **Theme delete failing when `@theme-id` differs from filename** — Theme ID is now always derived from the filename, ignoring `@theme-id` metadata comments.
- **Cannot clear user email or display name** — `PUT /api/auth/users/{name}` now accepts empty strings to clear these fields instead of silently ignoring them.
- **Button text contrast on accent backgrounds** — Primary buttons use `--accent-on-primary` (white) instead of `--bg-base` which was near-black in dark themes.
- **Theme family cards** — Now use semantic `<button>` elements instead of `<div role="button">` with manual keyboard handlers.
- **Separated setup and add-user state** — The "Create first user" form in Security no longer shares state with the "Add User" modal.
- **Icon browser pre-population** — Opening the icon browser for a new app/group now passes the current icon selection.
- **Static assets blocked by auth middleware** — Root-level static files (manifest.json, favicon.ico, apple-touch-icon.png, etc.) were incorrectly blocked by authentication, causing browser errors. Auth bypass rules now use explicit paths instead of non-functional glob patterns.

---

## [3.0.0-rc.2] - 2026-02-18

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

[3.0.0-rc.3]: https://github.com/mescon/Muximux/compare/v3.0.0-rc.2...v3.0.0-rc.3
[3.0.0-rc.2]: https://github.com/mescon/Muximux/compare/v3.0.0...v3.0.0-rc.2
[3.0.0]: https://github.com/mescon/Muximux/compare/v3.0.0-beta.1...v3.0.0
[3.0.0-beta.1]: https://github.com/mescon/Muximux/compare/v3.0.0-alpha.1...v3.0.0-beta.1
[3.0.0-alpha.1]: https://github.com/mescon/Muximux/releases/tag/v3.0.0-alpha.1

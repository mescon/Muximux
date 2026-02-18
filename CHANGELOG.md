# Changelog

All notable changes to Muximux are documented in this file.

## [Unreleased]

### Changed

- **Build tag split** — Go builds no longer require a `dist/` placeholder directory. Dev builds compile without `embed_web` tag; production builds use `-tags embed_web` to embed frontend assets.
- **Docker PUID/PGID support** — Container entrypoint now creates a runtime user matching `PUID`/`PGID` environment variables for bind-mount permission compatibility (linuxserver.io convention).
- **Docker security hardening** — `docker-compose.yml` adds `init: true`, `no-new-privileges`, and `cap_drop: ALL`.
- **Settings modal refactored** — Extracted each tab (General, Apps, Theme, Security, About) into its own component, reducing Settings.svelte from ~3800 lines to ~1800.
- **Button styles standardized** — All buttons in Settings now use the design system classes (`.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`) instead of hand-rolled Tailwind.
- **Open mode labels** — Consistent display between Add and Edit flows; both now use shared `openModes` constant.

### Added

- **Cancel button on Edit modals** — Edit App and Edit Group modals now have a Cancel button that reverts changes. Previously only "Done" was available, which applied changes immediately.
- **Validation on Edit modals** — Edit App and Edit Group modals now validate with Zod schemas before accepting, matching the Add flows.
- **Redirect open mode in UI** — The `redirect` open mode is now available in the Settings dropdown (previously only configurable via YAML).
- **`.btn-danger` design system class** — For destructive action buttons (delete confirmations).
- **Docstring coverage enforcement** — CI checks that 80%+ of exported Go identifiers have doc comments (`scripts/check-docstrings.sh`).
- **CHANGELOG-based release notes** — Release workflow extracts notes from CHANGELOG.md instead of auto-generating from PR titles. Falls back to auto-generation if no entry found.
- **CONTRIBUTING.md** — Developer guide covering prerequisites, dev mode, building, testing, and PR process.
- **systemd service file** — `muximux.service` for bare-metal deployments with security hardening.
- **CodeRabbit config** — `.coderabbit.yaml` with path-specific review instructions.
- **Codecov config** — `codecov.yml` with backend/frontend flags, patch target 70%, and carryforward support.

### Fixed

- **Theme family cards** — Now use semantic `<button>` elements instead of `<div role="button">` with manual keyboard handlers.
- **Separated setup and add-user state** — The "Create first user" form in Security no longer shares state with the "Add User" modal.
- **Icon browser pre-population** — Opening the icon browser for a new app/group now passes the current icon selection.

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

[Unreleased]: https://github.com/mescon/Muximux/compare/v3.0.0-rc.2...HEAD
[3.0.0-rc.2]: https://github.com/mescon/Muximux/compare/v3.0.0...v3.0.0-rc.2
[3.0.0]: https://github.com/mescon/Muximux/compare/v3.0.0-beta.1...v3.0.0
[3.0.0-beta.1]: https://github.com/mescon/Muximux/compare/v3.0.0-alpha.1...v3.0.0-beta.1
[3.0.0-alpha.1]: https://github.com/mescon/Muximux/releases/tag/v3.0.0-alpha.1

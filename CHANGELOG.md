# Changelog

All notable changes to Muximux are documented in this file.

## [Unreleased]

## [3.0.3] - 2026-02-26

### Added
- Configurable iframe cache limit (`navigation.max_open_tabs`) -- set the number of app tabs kept alive in memory, with LRU eviction for the oldest unused tabs (0 = unlimited, default)
- Iframe load error handling -- loading spinner, 30-second timeout, and error overlay with retry button when an embedded app fails to load
- WebSocket disconnect/reconnect toast notifications -- "Connection lost" warning when the server goes down, "Connection restored" when it comes back
- Collapsible groups on the splash page -- click a group header to collapse/expand, state persisted across reloads
- PWA service worker for static asset caching -- cache-first for hashed JS/CSS/fonts, network-first for HTML
- Accessibility labels on 70+ icon-only navigation buttons for screen reader support

### Changed
- Lazy-load Settings, CommandPalette, Logs, ShortcutsHelp, and OnboardingWizard -- initial bundle reduced from ~500kB to ~266kB
- Frontend performance improvements: rAF-gated scroll throttling, debounced resize listeners, cached getComputedStyle during drag, single-pass command palette grouping, fix healthStore subscription leak
- Global CSS transition rule replaced with targeted theme-switch-only class -- eliminates 150ms transition overhead on every hover
- "Shortcuts" hint on splash page changed to "All shortcuts" for clarity

### Fixed
- WebSocket reconnection no longer causes DOM flickering (state transitions suppressed during retry loop)

## [3.0.2] - 2026-02-26

### Added
- Logout action in the command palette -- visible when authenticated
- Reserved URL hashes: `#settings`, `#logs`, and `#overview` -- navigate directly to settings, logs, or the overview screen via URL
- URL hash now reflects the current view at all times (`#plex`, `#settings`, `#logs`, or `/` for overview)

### Changed
- Add App modal now exposes all settings (health check, proxy options, keyboard shortcut, min role, etc.) -- no longer requires a second trip to Edit

### Fixed
- Keyboard shortcut `?` (Show Shortcuts) now works on keyboards where `?` requires Shift
- URL hash not cleared when navigating home via logo click, command palette, or keyboard shortcut
- Collapsed sidebar cogwheel flyout now shows all footer actions including split view controls (orientation toggle, panel arrows, close)
- Open-mode icons (↗ ⧉) now have proper spacing from app names in all navigation layouts
- Search shortcut labels in navigation bars and splash screen now reflect custom keybindings instead of being hardcoded

## [3.0.1] - 2026-02-25

### Added
- Fetch icons from URL in the Custom tab -- paste an image URL and the server downloads, validates, and stores it locally as a custom icon, avoiding hotlinking issues
- `POST /api/icons/custom/fetch` API endpoint for downloading remote icons server-side
- Refresh button in the navigation bar (all 5 layout positions) -- visible when an app is active
- Auto-switch active split panel when clicking inside an iframe, so refresh and other actions target the correct panel

### Changed
- Number keys 1-9 now require explicit `shortcut` assignment -- positional fallback removed for clarity
- Onboarding wizard auto-assigns shortcuts 1-9 to the first 9 apps
- Splash tile badges only appear on apps with an explicit shortcut

### Security
- SSRF protection on `POST /api/icons/custom/fetch` -- rejects private, loopback, and link-local addresses

### Fixed
- Refresh action (command palette / `R` key) now targets the correct iframe in split view instead of always refreshing the first panel
- Gallery apps with preset group names that don't exist in the config now auto-create the group on add
- Collapsed sidebars (`show_labels: false`) now use the footer drawer pattern (cogwheel + hover-to-expand) so all footer actions remain accessible
- Fixed horizontal scrolling in collapsed left/right sidebars
- Logout URL not persisting in the settings security tab
- Forward-auth fields (`trusted_proxies`, `headers`, `logout_url`) now cleared from config when switching to a different auth method
- App rename/reorder no longer risks inheriting auth bypass rules from the wrong app due to positional matching
- CSP `manifest-src` directive added to allow dynamically generated blob manifests

## [3.0.0] - 2026-02-23

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
- API key authentication for programmatic access
- Rate-limited login and setup endpoints

**TLS & Gateway**
- Automatic HTTPS via Let's Encrypt (embedded Caddy)
- Manual certificate support
- Gateway mode -- reverse proxy other sites and services on your network that don't need to be in the Muximux menu, via Caddyfile

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
- Draggable floating FAB -- drag the button to any screen position; location persists per browser, double-click to reset to the configured corner
- Mobile-responsive: automatically switches to floating navigation on small screens (< 640px)
- Flat bar style for top/bottom navigation -- apps in a single row with group icon dividers
- Auto-hide with configurable delay
- Collapsible groups with drag-and-drop reordering
- Command palette with fuzzy search (`/` or `Ctrl+K`)
- Iframe caching -- visited app iframes stay in the DOM for instant switching without reload
- Split view -- display two apps side by side (horizontal) or stacked (vertical) with a draggable divider, inline panel selector to target each side, and URL hash routing with `#app1+app2` format
- Toggle buttons -- Logs and Overview buttons return to the previous app on a second press
- Hash-based app routing -- URL hash links to specific apps (e.g. `#plex`) with browser back/forward navigation
- Dynamic themed favicons matching the current theme's accent color

**Themes**
- 9 built-in theme families with dark/light variants: Default, Nord, Dracula, Catppuccin, Solarized, Tokyo Night, Gruvbox, Cineplex, Rose Pine
- System variant follows OS dark/light preference
- Custom theme editor with CSS custom properties
- Theme import/export

**Icons**
- 1,600+ Lucide icons
- Thousands of Dashboard Icons (on-demand, prefetch, or offline caching)
- Custom icon uploads (PNG, SVG, JPG, WebP) and fetch from URL
- URL-based icons for power users editing config.yaml directly
- Per-icon color tinting and background

**Keyboard Shortcuts**
- Configurable keybindings for all actions
- Per-app shortcut assignment with number keys (1-9)

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

[3.0.3]: https://github.com/mescon/Muximux/releases/tag/v3.0.3
[3.0.2]: https://github.com/mescon/Muximux/releases/tag/v3.0.2
[3.0.1]: https://github.com/mescon/Muximux/releases/tag/v3.0.1
[3.0.0]: https://github.com/mescon/Muximux/releases/tag/v3.0.0

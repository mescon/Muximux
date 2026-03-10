# Changelog

All notable changes to Muximux are documented in this file.

## [3.0.15] - 2026-03-09

### Fixed
- Proxied apps using relative URLs (e.g. `css/style.css`) now load correctly -- the interceptor rewrites relative paths through the proxy prefix, fixing CSS/JS/image loading in apps like qBittorrent
- `location.pathname`, `location.href`, `location.toString()`, `document.URL`, and `document.documentURI` in proxied apps now transparently strip the proxy prefix -- SPA code always sees clean paths without `replaceState` altering the real browser URL, so F5 refresh works correctly
- `location.href = "/path"` assignments in proxied apps now route through the proxy
- Inline SSR payloads (e.g. Nuxt 3 `fullPath` fields) are no longer rewritten -- prevents hydration route mismatches that caused 404s on page refresh in frameworks like Mealie
- Code-split chunks in proxied apps now load correctly -- the proxy rewrites ES module `import()` and `import`/`export...from` specifiers since the browser's module loader bypasses the runtime fetch interceptor
- `Worker` and `SharedWorker` constructors in proxied apps are now intercepted so worker scripts load through the proxy
- Protocol-relative URLs (`//cdn.example.com/lib.js`) in proxied apps are no longer incorrectly prefixed with the proxy path
- `<meta http-equiv="refresh">` URLs are now rewritten through the proxy -- previously only the `Refresh` response header was handled
- `<object data>`, `<button formaction>`, and `<input formaction>` attributes are now covered by the proxy URL rewriter
- `localStorage` and `sessionStorage` in proxied apps are now isolated per app -- keys are transparently namespaced so apps sharing the same origin no longer collide with each other or with Muximux
- Opening an app's settings no longer shows "unsaved changes" without making changes -- optional fields are now normalised with defaults on load
- Icon background colour picker now works on existing apps -- factory functions deep-merge icon fields so `background` is always present
- App names with trailing spaces or dashes (e.g. "qBittorrent - ") no longer produce malformed URL slugs with consecutive dashes -- the slugifier now collapses separators and trims edges
- `setAttribute('src', url)` in proxied apps is now intercepted synchronously -- fixes apps using MooTools or similar libraries that set URL attributes via setAttribute instead of property assignment
- `HTMLImageElement.srcset` property setter in proxied apps is now intercepted -- responsive image libraries setting srcset via JS no longer 404
- `<base href>` set via JS in proxied apps is now intercepted -- prevents relative URL resolution from breaking when the base element is modified programmatically
- `new Audio(url)` in proxied apps now routes through the proxy -- previously only subsequent `.src` assignments were caught
- `CSSStyleSheet.insertRule()` with `url()` references in proxied apps now rewrites paths through the proxy -- fixes CSS-in-JS libraries injecting background images and font sources
- `insertAdjacentHTML()` in proxied apps now synchronously rewrites URLs -- closes the same async MutationObserver timing gap that affected setAttribute
- `Origin` and `Referer` request headers are now rewritten to match the backend host when proxying -- fixes CSRF validation failures in apps like Django, Rails, and .NET that check these headers
- `Set-Cookie` `Domain` attribute is now stripped from proxied responses -- cookies default to the proxy host instead of being scoped to the unreachable backend host
- `Set-Cookie` `Secure` flag is now stripped when Muximux serves over HTTP -- prevents cookies from being silently dropped by browsers on non-HTTPS connections
- `Set-Cookie` `SameSite=Strict` is now downgraded to `Lax` in proxied responses -- `Strict` is too restrictive when the app is embedded through a proxy
- `ETag` and `Last-Modified` headers are now stripped from rewritten proxy responses -- prevents stale caching after the interceptor script is injected
- `Access-Control-Allow-Origin` headers from proxied backends are now rewritten to match the request origin -- fixes CORS failures when the backend returns its own host instead of the proxy host

## [3.0.14] - 2026-03-08

### Fixed
- Proxied SPAs (Mealie, Immich, etc.) no longer show a 404 on page refresh -- the proxy interceptor now strips the `/proxy/slug/` prefix from `location.pathname` before the SPA router reads it, and patches `history.pushState`/`replaceState` to maintain correct proxy URLs in the browser history
- Back/forward navigation within proxied SPAs now works correctly -- a `popstate` capture-phase listener strips the proxy prefix before the app's router handler fires
- `location.assign()`, `location.replace()`, and `window.open()` inside proxied apps are now rewritten to go through the proxy
- `navigator.sendBeacon()` inside proxied apps is now rewritten to go through the proxy
- Anchor `href` and form `action` property setters are now intercepted by the proxy rewriter -- previously only the MutationObserver fallback caught these, missing programmatic `a.href = "/page"` assignments
- Empty app color no longer produces invalid `color-mix()` CSS in icon backgrounds -- falls back to default grey when color is blank
- App color field now shows a reset button and placeholder, matching the icon background field pattern

## [3.0.13] - 2026-03-08

### Fixed
- Adding an app from the gallery that has a preset group no longer crashes with "missing 'id' property" -- the auto-created group now gets the DnD identifier that Svelte's keyed `{#each}` requires
- Adding the first app from the gallery no longer silently marks it as the default homepage app -- `default` is only set explicitly during onboarding
- Edit/delete buttons on app rows, keybinding combos, custom themes, and custom icons are now always visible -- previously hidden behind hover, making them inaccessible on touch devices

### Changed
- App and Group object construction consolidated into shared `makeApp()`/`makeGroup()` factory functions and `stampAppId()`/`stampGroupId()` helpers -- eliminates duplicated defaults across 4 creation paths and prevents field-omission bugs

## [3.0.12] - 2026-03-08

### Fixed
- Changing an app icon in Settings no longer silently breaks the Save button -- the icon property is now mutated in-place instead of replacing the object, preserving Svelte 5 reactivity references
- `manifest.json` no longer triggers a CORS error behind forward-auth proxies (Authelia, Authentik) -- the manifest link now sends credentials with the request
- Proxied apps that stream large responses or have slow backends no longer get killed after 15 seconds -- the reverse proxy now extends the server's write deadline per request
- RTL sidebar auto-hide positioning uses CSS logical properties (`inset-inline-start`/`inset-inline-end`) instead of physical `left`/`right`
- RTL sidebar resize drag and edge-swipe gesture direction are now correct in right-to-left layouts
- Rate limiter now extracts the real client IP via `X-Forwarded-For` / `X-Real-IP` from trusted proxies instead of using the raw upstream address
- Health check `CheckNow()` now uses a proper timeout context instead of an unbounded `context.Background()`
- Empty proxy path (`/proxy/`) now returns 400 Bad Request instead of silently falling through
- Closing the Edit App, Edit Group, or Import Config modal no longer crashes with "Cannot read properties of null (reading 'icon')" -- out-transitions were keeping the DOM alive while the backing variable was already null

### Changed
- Config saves use atomic write-to-temp + rename to prevent corruption on crash or power loss
- Session store cleanup goroutine can now be stopped cleanly on shutdown
- Paraglide generated files excluded from ESLint and SonarCloud analysis

### Security
- Bump svelte-check 4.4.4 → 4.4.5

## [3.0.11] - 2026-03-07

### Added
- Multi-language support -- 36 languages with native translations for every UI string, selectable in Settings > General > Language
- Language selector on the onboarding welcome screen -- new installations can choose their language before setup
- RTL layout support for Arabic
- Translation contributor guide in the wiki

## [3.0.10] - 2026-03-06

### Fixed
- Settings popup now always closes after saving -- if a render error occurred during the config update, the popup would remain stuck open requiring a page reload

## [3.0.9] - 2026-03-05

### Fixed
- WebSocket connections no longer fail with `response does not implement http.Hijacker` -- the request logging wrapper now delegates `Hijack()` and `Flush()` to the underlying `ResponseWriter`
- PWA manifest no longer triggers a CSP violation when behind a forward-auth proxy (Authelia, Authentik) -- the `<link rel="manifest">` is now injected after authentication instead of being present in the static HTML
- Request logs now show the real client IP instead of the upstream proxy IP when running behind a trusted reverse proxy (Traefik, nginx, etc.)
- Proxied apps can no longer register service workers under Muximux's origin -- the proxy interceptor now blocks `navigator.serviceWorker.register()` and cleans up any previously leaked registrations
- White flash eliminated when loading a new app on dark themes -- the iframe is kept invisible behind the loading overlay until the page has painted, then fades in smoothly
- PWA icon updates now take effect without clearing browser cache -- bumped service worker cache version and added revalidation headers for root-level static files (icons, manifest, browserconfig)

## [3.0.8] - 2026-03-05

### Fixed
- PWA "Add to Home Screen" icons now use the correct teal accent color (`#2dd4bf`) on a solid dark background (`#09090b`) instead of appearing with wrong colors on a transparent background
- PWA icons are no longer visually distorted -- the logo is properly centered within the maskable icon safe zone
- Added 512×512 icon for high-density Android home screen launchers
- Corrected `theme-color` and `background-color` meta tags and manifest to match the default dark theme

## [3.0.7] - 2026-03-05

### Added
- Log rotation -- `muximux.log` is automatically rotated at 10 MB with up to 3 archived copies; no external tooling needed
- Log persistence across restarts -- the `/logs` page now shows entries from before the last restart by replaying the log file on startup
- Icon cache cleanup -- expired dashboard and Lucide icon cache files are automatically pruned every 24 hours
- Centralized request logging -- API and page requests are logged at INFO with method, path, status, latency, bytes written, remote IP, and user agent (`source=http`); static asset requests are logged at DEBUG only to avoid noise
- Request ID correlation -- `X-Request-ID` header on every response; incoming IDs from upstream proxies are honored
- Context-aware logging -- all log entries within a request carry `request_id` and authenticated `user` for correlation
- Panic recovery middleware -- handler panics are caught, logged with stack trace, and return 500 with request ID
- `MUXIMUX_LOG_LEVEL` and `MUXIMUX_LOG_FORMAT` environment variables for runtime log configuration
- `server.log_format` config option (`text` or `json`; default: `text`)

### Changed
- All HTTP error responses now log at appropriate severity (5xx at ERROR, 401/403 at WARN, 4xx at DEBUG)
- Simplified frontend API layer with shared request helper

### Fixed
- Proxied apps with internal sub-iframes (e.g. qBittorrent search download dialog) no longer lose access to parent window state -- the proxy interceptor now distinguishes between internal frames and the Muximux host
- Proxy runtime interceptor now handles `URL` objects passed to `fetch()` and `XMLHttpRequest` -- previously only string URLs were rewritten
- Proxy runtime interceptor now rewrites URLs in dynamically created `<iframe>`, `<link>`, `<a>`, and `<img srcset>` elements -- previously only `<img>`, `<script>`, `<source>`, `<video>` were covered

### Removed
- Unused error helper functions (internal cleanup, no user-facing impact)

## [3.0.6] - 2026-03-04

### Fixed
- Proxied apps that access `window.parent` (e.g. qBittorrent's MochaUI/MooTools) no longer crash by calling methods on the Muximux host window -- the proxy interceptor now overrides `window.parent` and `window.top` so embedded apps behave as if running in a standalone tab
- Service worker no longer attempts to cache cross-origin requests
- Settings group editor no longer crashes after Svelte 5.53.7 effect scheduling changes

### Security
- Bump Svelte 5.53.5 → 5.53.7, svelte-check 4.4.2 → 4.4.4, svelte-sonner 1.0.5 → 1.0.8, globals 16.0.0 → 17.4.0
- Bump CI actions: actions/download-artifact v7 → v8, actions/setup-go 6.2.0 → 6.3.0, actions/setup-node 6.2.0 → 6.3.0, docker/setup-qemu-action 3.7.0 → 4.0.0, docker/login-action 3.7.0 → 4.0.0

## [3.0.5] - 2026-03-03

### Fixed
- White flash when switching between app iframes on dark themes -- iframe container now uses theme background instead of hardcoded white

## [3.0.4] - 2026-02-27

### Changed
- Service worker cache now updates automatically on each deployment -- previously, stale cached assets could persist until a hard refresh

### Fixed
- Proxied apps (like Pi-hole) no longer break due to Muximux's Content Security Policy blocking their inline scripts and styles
- Apps configured with `base_path` no longer trigger CSP violations on page load
- Rare proxy routing bug where an app named "app" could corrupt URLs containing `/proxy/application/` or similar substrings

### Security
- Bump `golang.org/x/net` to v0.51.0 -- fixes a denial-of-service vulnerability where malformed HTTP/2 frames could crash the server (CVE-2026-27141)
- Bump Svelte 5.53.3 → 5.53.5, Rollup 4.57.1 → 4.59.0

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

[3.0.15]: https://github.com/mescon/Muximux/releases/tag/v3.0.15
[3.0.14]: https://github.com/mescon/Muximux/releases/tag/v3.0.14
[3.0.13]: https://github.com/mescon/Muximux/releases/tag/v3.0.13
[3.0.12]: https://github.com/mescon/Muximux/releases/tag/v3.0.12
[3.0.11]: https://github.com/mescon/Muximux/releases/tag/v3.0.11
[3.0.10]: https://github.com/mescon/Muximux/releases/tag/v3.0.10
[3.0.9]: https://github.com/mescon/Muximux/releases/tag/v3.0.9
[3.0.8]: https://github.com/mescon/Muximux/releases/tag/v3.0.8
[3.0.7]: https://github.com/mescon/Muximux/releases/tag/v3.0.7
[3.0.6]: https://github.com/mescon/Muximux/releases/tag/v3.0.6
[3.0.5]: https://github.com/mescon/Muximux/releases/tag/v3.0.5
[3.0.4]: https://github.com/mescon/Muximux/releases/tag/v3.0.4
[3.0.3]: https://github.com/mescon/Muximux/releases/tag/v3.0.3
[3.0.2]: https://github.com/mescon/Muximux/releases/tag/v3.0.2
[3.0.1]: https://github.com/mescon/Muximux/releases/tag/v3.0.1
[3.0.0]: https://github.com/mescon/Muximux/releases/tag/v3.0.0

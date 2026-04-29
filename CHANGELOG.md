# Changelog

All notable changes to Muximux are documented in this file.

## [3.0.32] - 2026-04-30

Per-app group-based access control. Apps can now declare an `allowed_groups: [...]` allowlist; only users in at least one of those groups see and reach the app. Resolves #326.

### Added
- New `allowed_groups: []string` field on each app config. Empty or missing means no group gate (current behavior). When set, a non-admin user must belong to at least one matching group; matching is case-insensitive. Stacks with `min_role`: both gates must pass.
- Built-in user records gained an optional `groups: []string` field, editable in **Settings -> Security -> Users** next to the role selector. The change persists on blur.
- New **Allowed groups** input on the App edit form (comma-separated), so admins can wire up filtering without touching `config.yaml`.
- OIDC users now carry their `groups_claim` value through the session so per-app filtering can match against it. Forward-auth users similarly carry their `Remote-Groups` header value through. Admins still bypass the group gate the same way they bypass `min_role`.
- Authentication wiki gained a "Per-App Group Filtering" section explaining the rules, the source-of-truth for each auth method, and how a misconfigured IdP fails closed (invisible) rather than open.

API Key management UI plus per-provider OIDC and forward-auth setup guides covering Microsoft Entra ID, Keycloak, Authentik, Pocket ID, Zitadel, Google, Authelia, and Cloudflare Access.

### Added
- **Settings > Security > API Key** lets admins generate, rotate, and delete the instance-wide API key from the dashboard. The plaintext is shown exactly once after generation; afterwards only the bcrypt hash lives on disk. The previous flow (write `api_key_hash` to `config.yaml` by hand or via `muximux hash`) still works and the UI surfaces the result of either path. Resolves the comment from #321 about the missing UI.
- Authentication wiki gained a second worked example showing how to expose a proxied app's webhook endpoint (for example a CI tool's GitHub receiver) to an external service via `auth_bypass` + `require_api_key: true`, with notes on how the proxied app's own `X-Api-Key` header semantics interact with Muximux's.
- Step-by-step setup guides for eight identity providers, each with the IdP-side configuration, matching `config.yaml`, validation steps, and a troubleshooting table: Microsoft Entra ID, Keycloak, Authentik, Pocket ID, Zitadel, Google, Authelia (forward auth or OIDC), Cloudflare Access (forward auth). Linked from the wiki sidebar, the README, and the central authentication page.

## [3.0.30] - 2026-04-20

Notification bridge fixes for mobile and HTTP-origin Muximux installs (follow-up to #320 test results).

### Fixed
- Notifications from embedded apps now work on Android Chrome, Samsung Browser, and mobile Firefox. The bridge renders through `ServiceWorkerRegistration.showNotification()` instead of the `Notification` constructor, which mobile browsers don't implement.
- Notification clicks on mobile route back to the right app. The service worker posts the target app name to the active window client and the bridge calls `selectApp` on receipt.
- The proxy-injected `Notification` shim no longer lies about permission. It starts at `"default"`, queries the parent via `postMessage`, and forwards `requestPermission()` to the top-level window so the real browser prompt appears at Muximux's origin. Apps that read `Notification.permission` before sending see the actual state instead of a hardcoded `"granted"`.

### Changed
- Apps wiki documents the HTTPS requirement and the iOS Safari PWA-install caveat for notifications.
- `golangci-lint run` on the repo now reports zero issues -- eleven pre-existing false positives (test fixtures, ASCII-range byte conversions, trusted-content writes) got narrow `//nolint` comments with actual reasons so new contributions start from a clean baseline.

## [3.0.29] - 2026-04-19

A security-heavy release plus an appearance API for embedded apps.

### Appearance API for embedded apps
New `GET /api/appearance` returns active language, theme id, and a curated palette of CSS variables so embedded apps can style themselves to match Muximux. Proxied apps inherit the session cookie; external apps authenticate with `X-Api-Key`. Resolves #321.

### Built-in theme renamed to "Muximux"
`dark` / `light` theme ids are now `muximux` / `muximux-light` to match the `<family>` / `<family>-light` pattern used by every other theme. `config.theme.family` stays `default` -- existing configs load unchanged. If you have hand-written selectors targeting `[data-theme="dark"]`, update them.

### First-run setup is no longer a race
The onboarding wizard and "Restore from Backup" flow now require a one-time setup token generated on first boot. Find it in server logs (`Generated new setup token`) or at `<dataDir>/.setup-token`. Doesn't affect upgrades.

### Security audit
Full review between v3.0.28 and this release. Highlights: session cookies stripped from proxied backends, 7-day absolute session lifetime, admin-scoped WebSocket events no longer leak to regular users, login timing equalized + rate-limiter hardened against `X-Forwarded-For` spoofing, OIDC requires `id_token` + PKCE by default, SVG icons download instead of rendering, CR/LF blocked in proxy headers, gzip bombs capped, tighter CSP (`frame-ancestors`, `form-action`), HSTS on TLS. See the [security wiki](https://github.com/mescon/Muximux/wiki/security) for the full posture.

### Fixed
- Pull-to-refresh on mobile waits for the real iframe `load` event (capped at 10s) instead of a hard-coded 1s timeout.
- Frontend no longer loops on "Unexpected token '<'" when the backend returns HTML error pages.
- Toast messages no longer show raw reverse-proxy 502 HTML.
- Theme files are written atomically; a crash mid-write no longer leaves a truncated `.css`.
- Setup and restore are transactional: disk-write failures roll back in-memory config.
- Admin cannot demote the last remaining admin.
- Config imports reject unknown fields and validate durations / auth methods / `open_mode` / `min_role` up front.

### Changed
- API key docs rewritten -- it's only useful on allowlisted paths (`/api/appearance` + per-app `auth_bypass`). Mutating `/api/*` still requires a session cookie. See [authentication wiki](https://github.com/mescon/Muximux/wiki/authentication#api-key-authentication).
- WebSocket upgrades require a matching `Origin` header.
- Forward-auth admin-group matching is case-insensitive (matches OIDC).
- Bcrypt target cost 10 → 12. Accounts silently re-hash on next login.
- Forward-auth and OIDC providers shut down cleanly on restart.

## [3.0.28] - 2026-04-18

Your embedded apps can finally do the things they couldn't before.

### Let your apps actually use the camera, microphone, and friends

If you've ever tried embedding a video meeting app, a scanner, a passkey login, or anything that wants access to the camera, microphone, or geolocation, you've probably been greeted with "Permission denied". That's the browser being cautious: by default, sensitive features are off-limits to anything running inside an iframe, no matter what.

v3.0.28 adds a per-app `permissions` setting. Pick what the app is allowed to use (camera, microphone, geolocation, fullscreen, screen capture, clipboard, audio autoplay, MIDI, payments, passkeys, picture-in-picture, wake lock, USB, serial, HID) and Muximux delegates that permission to the iframe. There's a settings panel with a checkbox for every supported feature, each with a hover tooltip that explains what it does and a link to full MDN docs.

If you just want the embedded app to have everything, set `permissions: [all]` in your YAML (or click "Allow all permissions" in the settings). New permissions Muximux adds in future releases automatically get included.

### Notifications from embedded apps, finally

Browsers block the Web Notifications API in cross-origin iframes. Your self-hosted app might have notifications working perfectly when you open it directly, then go completely silent the moment it's embedded in Muximux.

There's now a notification bridge that fixes this. Enable `allow_notifications: true` on an app and it can trigger real browser notifications that appear under Muximux's own origin.

Two tiers, depending on whether the app is proxied:

- **Proxied apps (`proxy: true`):** transparent. No code changes needed. Muximux intercepts calls to the standard `new Notification(...)` API inside the iframe and routes them through the bridge. Most apps that already support notifications when opened directly will start working.
- **Non-proxied apps:** the app needs to explicitly post a message to Muximux with `window.parent.postMessage({ type: 'muximux:notify', title, body, tag }, '*')`. This is a small code change but unavoidable: browsers block cross-origin code injection, so Muximux can't reach into the iframe to install the shim.

Click a notification and Muximux switches to the app that sent it. There's a short rate limit (one notification per app every 2 seconds) and some anti-spoofing guardrails: the notification icon always comes from the app's configured icon, and clicks always go to the app in Muximux. An embedded app can't dress its notification up as another app, or use a notification click to redirect you somewhere unexpected.

Resolves #320.

### Changed
- Document-level `Permissions-Policy` header now permits delegatable features for iframe delegation (was `camera=(), microphone=(), geolocation=()`). Muximux's own JS does not call these APIs, so widening the policy does not broaden Muximux's attack surface. Per-app iframe `allow` attributes remain the effective gate.
- Bump `github.com/jackc/pgx/v5` from 5.8.0 to 5.9.0 to fix a memory-safety vulnerability (critical)
- Bump npm group (svelte 5.55.4, vite 8.0.8, vitest 4.1.4, @vitest/coverage-v8 4.1.4, @inlang/paraglide-js 2.16.0, globals 17.5.0, typescript-eslint 8.58.2)
- Bump `actions/upload-artifact` from 7.0.0 to 7.0.1
- Bump `softprops/action-gh-release` from 2.6.1 to 3.0.0
- Bump `github/codeql-action` from 4.35.1 to 4.35.2
- Bump `docker/build-push-action` from 7.0.0 to 7.1.0
- Bump go-dependencies group (2 updates)

## [3.0.27] - 2026-04-12

### Security
- Bump OpenTelemetry OTLP HTTP exporters to v1.43.0 -- fixes unbounded HTTP response body reads in `otlptracehttp` and `otlpmetrichttp`

### Changed
- Bump `github.com/coreos/go-oidc/v3` from 3.17.0 to 3.18.0
- Bump `go.opentelemetry.io/otel/sdk` from 1.40.0 to 1.43.0
- Bump `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp` from 0.16.0 to 0.19.0
- Bump `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp` from 1.40.0 to 1.43.0
- Bump `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` from 1.40.0 to 1.43.0
- Bump `docker/login-action` from 4.0.0 to 4.1.0
- Bump npm dependencies (svelte, vite, vitest, eslint, tailwindcss, and others)

## [3.0.26] - 2026-04-07

### Security
- Bump `vite` from 8.0.1 to 8.0.5 -- fixes path traversal in optimized deps `.map` handling, `server.fs.deny` bypass with queries, and arbitrary file read via dev server WebSocket
- Bump `github.com/go-jose/go-jose/v3` from 3.0.4 to 3.0.5 -- fixes panic in JWE decryption
- Bump `github.com/go-jose/go-jose/v4` from 4.1.3 to 4.1.4 -- fixes panic in JWE decryption
- Bump `picomatch` from 4.0.3 to 4.0.4 -- fixes method injection in POSIX character classes
- Fix `brace-expansion` and `yaml` moderate vulnerabilities via npm audit fix

### Changed
- Bump `typescript` from 5.9.3 to 6.0.2
- Bump `svelte` from 5.54.0 to 5.55.1
- Bump `eslint` from 10.0.3 to 10.1.0
- Bump `eslint-plugin-svelte` from 3.15.2 to 3.16.0
- Bump `jsdom` from 29.0.0 to 29.0.1
- Bump `svelte-check` from 4.4.5 to 4.4.6
- Bump `marked` from 17.0.4 to 17.0.5
- Bump `@inlang/paraglide-js` from 2.15.0 to 2.15.1
- Bump `@vitest/coverage-v8` from 4.1.0 to 4.1.2
- Bump `typescript-eslint` from 8.57.1 to 8.58.0
- Bump `actions/setup-go` from 6.3.0 to 6.4.0
- Bump `github/codeql-action` from 4.33.0 to 4.35.1
- Bump `codecov/codecov-action` from 5.5.2 to 6.0.0
- Bump `SonarSource/sonarqube-scan-action` from 7.0.0 to 7.1.0

## [3.0.25] - 2026-03-22

### Added
- Configurable overview button -- choose between hidden, Muximux logo, house icon, or any custom icon via a visual selector in Settings > Navigation
- Collapsible toolbar for top and bottom navigation -- utility buttons (logs, refresh, split view) collapse behind a cogwheel that reveals on hover, matching the existing sidebar footer drawer pattern; controlled by the "Collapsible Toolbar" toggle in settings

### Changed
- Bump `vite` from 8.0.0 to 8.0.1
- Bump `@tailwindcss/vite` from 4.2.1 to 4.2.2
- Bump `vitest` from 4.0.18 to 4.1.0
- Bump `@inlang/paraglide-js` from 2.14.0 to 2.15.0
- Bump `@sveltejs/vite-plugin-svelte` from 6.2.4 to 7.0.0
- Bump `jsdom` from 28.1.0 to 29.0.0
- Bump `svelte` from 5.53.10 to 5.54.0
- Bump `typescript-eslint` from 8.57.0 to 8.57.1
- Bump `@vitest/coverage-v8` from 4.0.18 to 4.1.0
- Bump `actions/upload-artifact` from 6.0.0 to 7.0.0
- Bump `softprops/action-gh-release` from 2.5.0 to 2.6.1
- Bump `github/codeql-action` from 4.32.6 to 4.33.0
- Bump `golang.org/x/crypto` from 0.48.0 to 0.49.0
- Bump `google.golang.org/grpc` from 1.79.1 to 1.79.3
- Bump `github.com/smallstep/certificates` from 0.30.0-rc3 to 0.30.0
- Bump `flatted` from 3.3.3 to 3.4.2
- Bump `kysely` and `@inlang/sdk`
- Bump `undici` from 7.21.0 to 7.24.1
- Bump `devalue` from 5.6.3 to 5.6.4

## [3.0.24] - 2026-03-17

### Fixed
- Proxied apps with Spring Security (CyberPower PowerPanel Business) no longer return 403 on POST/PUT/DELETE requests -- the `Origin` header is now stripped from all proxied requests, not just safe methods; Spring Security's CorsFilter rejects any request with an `Origin` header when no CORS config is defined, including login and API endpoints

## [3.0.23] - 2026-03-16

### Fixed
- Proxied Angular apps no longer get `SyntaxError: Unexpected token ':'` from corrupted JavaScript -- the base path rewriter was matching `_baseHref` as a substring of `baseHref` in minified code and replacing `=` with `:`, producing invalid syntax; the rewriter now requires the variable name to be standalone

## [3.0.22] - 2026-03-15

### Fixed
- OIDC authentication with Keycloak (and other providers) no longer returns 401 after a successful login -- the auth middleware was looking up OIDC users in the config-based user store, which only contains builtin users; the middleware now reconstructs the user from session data when the store lookup fails
- Proxied Angular apps (CyberPower PowerPanel Business) no longer return 403 Forbidden for module scripts -- the `Origin` header is now stripped from safe (GET/HEAD/OPTIONS) requests forwarded to backends, preventing Spring Security CORS rejection on apps with no CORS configuration; unsafe methods (POST/PUT/DELETE/PATCH) continue to send the rewritten `Origin` for CSRF compatibility

### Changed
- Bump `undici` from 7.21.0 to 7.24.1

## [3.0.21] - 2026-03-13

### Fixed
- Proxied Nuxt/Vue apps (Mealie) no longer show "Page not found" 404 errors on Chrome -- the Navigation API handler introduced in v3.0.20 was blocking internal `replaceState` calls that strip the proxy prefix during initialization; added a skip flag so the handler does not interfere with the interceptor's own prefix management
- Proxied apps using relative resource paths (e.g. qBittorrent's `css/style.css`) no longer fail to load stylesheets and scripts on Chrome -- a `<base>` tag is now injected into HTML responses that lack one, anchoring relative URL resolution to the proxy prefix before any resources load

### Changed
- Bump `devalue` from 5.6.3 to 5.6.4

## [3.0.20] - 2026-03-12

### Fixed
- Proxied apps using relative API paths (e.g. `api/v2/sync/maindata`) no longer 404 with double-prefixed URLs -- the interceptor's restore-prefix function now preserves the trailing slash in `document.baseURI`, so the browser resolves relative URLs against `/proxy/slug/` instead of `/proxy/`
- Interceptor script is no longer injected into `<head>` tags that appear inside `<script>` blocks -- apps like qBittorrent whose HTML fragments contain `<head>` in JavaScript template literals (e.g. `srcdoc`) no longer get a `SyntaxError: Octal escape sequences are not allowed in template strings` at load time
- Interceptor injection now correctly distinguishes `<head>` from `<header>` tags

## [3.0.19] - 2026-03-11

### Fixed
- Browser back button no longer exits Muximux entirely -- app navigation now creates proper history entries so back/forward moves between previously viewed apps
- Browser back from within a proxied app no longer loads Muximux inside its own iframe -- the interceptor now restores the proxy prefix in the URL after initialization completes, so back/forward navigates to the correct proxied URL instead of "/"
- Added safety net: Muximux detects when it is loaded inside an iframe and renders nothing, preventing nested dashboard UI if back/forward edge cases are missed
- Proxied apps that navigate via `location.href` assignment (e.g. Pi-hole after saving settings) no longer break out of the proxy on Chrome -- the interceptor now uses the Navigation API to catch these navigations when the native `href` setter can't be patched

## [3.0.18] - 2026-03-11

### Fixed
- Proxied SSR apps (Nuxt, Next.js) that read `location.pathname` during initialization in Chrome no longer get "Page not found" errors -- Chrome's `Location.prototype.pathname` is non-configurable so the getter patch fails, and framework routers calling `replaceState` during setup re-added the proxy prefix; the interceptor now re-strips the prefix after each `pushState`/`replaceState` until the page finishes loading

### Changed
- Bump `actions/download-artifact` from 8.0.0 to 8.0.1
- Bump `docker/setup-buildx-action` from 3.12.0 to 4.0.0
- Bump `docker/build-push-action` from 6.19.2 to 7.0.0
- Bump `docker/metadata-action` from 5.10.0 to 6.0.0
- Bump `github/codeql-action` from 4.32.4 to 4.32.6
- Bump `golang.org/x/term` from 0.40.0 to 0.41.0
- Bump npm dependencies (svelte, vite, vitest, @sveltejs/kit, @sveltejs/vite-plugin-svelte)

## [3.0.17] - 2026-03-10

### Fixed
- `<meta http-equiv="Content-Security-Policy">` tags are now stripped from proxied HTML responses -- apps like Mealie (Nuxt) that embed CSP with nonces in the HTML body no longer block the injected interceptor script
- `Permissions-Policy` response header is now stripped from proxied responses -- prevents apps from restricting features like clipboard, fullscreen, and autoplay inside the iframe
- Iframe sandbox now includes `allow-popups-to-escape-sandbox` -- OAuth and login popups opened by proxied apps can function without sandbox restrictions

## [3.0.16] - 2026-03-10

### Fixed
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

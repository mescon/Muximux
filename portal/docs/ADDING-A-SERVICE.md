# Adding a service (port) to the portal

Status: draft by the Architect (M-237), 2026-11-14. Commands marked (verify) were read from the code, not run end to end. Update this file with the engineer's M-323 findings.

## 0. What "works" means
A service is **portal-ready** only when it passes the gateway check in section 4 on the real URLs (tailnet host and `<portal-public-host>`), after login, with its WebSocket up. Until then it must not be in `gateway-apps.json`; it opens by its fallback (section 5), never as a broken tab.

## 1. Register the port (pick one)
| Case | How | Result |
| --- | --- | --- |
| Permanent service | Add an entry to `ports.yaml` (`id: <app>[-<lane>].<role>`, `app`, `lane`, `role`, `port`, `health`, `portal: {title, tab: true}`). Allocate with `bin/ports alloc <app> <lane> <role>` (range 45000-45999; it writes the `ports.yaml` entry) | Static registry; appears in `bin/ports list` and the portal within ~30 s |
| Self-registering process (engines) | `POST` `{id, app, lane, role, port, pid?, ttl?}` to `ports-api` `/v1/ports`, `PUT /v1/ports/<id>` as heartbeat (verify) | `ports.registered.json` (never hand-edit) |
| Ad hoc, one-off | Portal "Ports"/lease page (`/lease/`), `POST /ports/v1/open {port}` | Short-lived Muximux app |
Never hard-code a port: resolve with `bin/ports get <id> --port`.

## 2. The port must be allowed
`bin/portal-gw` allows `registry + registered + live leases - ports-deny.yaml` (`allowed_ports()`). A port missing from that union returns **403 "port denied"** (this was the Freebuff `…-45200` failure). Check: `curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3738/p/<port>/` must not be 403. Sensitive ports go into `ports-deny.yaml` with a reason.

## 3. Make the service reachable through the gateway
The gateway talks to `127.0.0.1:<port>` and rewrites Host/Origin/Referer to `localhost:<port>`. The service needs: listen on loopback or all interfaces; no hard dependency on its own public hostname.

## 4. Gateway compatibility check (do this before enabling)
Run in a real Chromium (headed or Playwright) against `/p/<port>/` on the tailnet host AND inside the portal iframe on `<portal-public-host>`:
1. First load: no blank page; no failed requests; JS returns `application/javascript` (not HTML).
2. Login completes and survives reload (cookies: upstream `SameSite=Strict`, `Domain`, `Path` are rewritten by the gateway; check the `multica_auth`-style cookie is sent on API and WS requests).
3. WebSocket/SSE connect (`ws upgrade <port> … cookie`, status 101, no `NO-COOKIE`); origin-checking servers (Multica, Cockpit) rely on the Host/Origin rewrite.
4. Deep link and reload at a sub-path keep working (apps that build URLs from `location.pathname` need the shim; absolute URLs built from `location.origin` are the usual breakers).
5. Cross-site iframe: cookies need `SameSite=None; Secure; Partitioned` (gateway adds it when `X-Forwarded-Proto: https`).
Known breakers: `<base href>`, CSP nonces vs the injected shim, hard-coded absolute URLs, services that set `X-Frame-Options`/`frame-ancestors` (stripped) or check `Origin` with their own allowlist.

## 5. Enable and fall back
- **Passed:** add the Muximux app name to `gateway-apps.json` `apps` (and, for deep links on public/tailnet hosts, a `claims` entry `{port, app, prefix, hosts}`); `portal-muximux-sync` then emits `url: /p/<port>/`.
- **Failed or unverified:** leave it out; set the port's `open:` profile in `port-profiles.yaml` to `public` or `newtab` (opens outside the iframe) and record the reason in the ADR open items.
- Rollback: remove the name from `gateway-apps.json`; the next sync (~30 s) restores the previous behaviour.

## 5b. Tab icons — the app's own favicon (M-357)
Every gateway tab shows the app's own web-page favicon, not a generic icon:
- `portal-muximux-sync` sets each `/p/<port>/` tab's Muximux icon to
  `{"type": "url", "url": "/p/<port>/__favicon"}`.
- `bin/portal-gw` serves `GET /p/<port>/__favicon`: it fetches the app
  page upstream, resolves `<link rel=icon>` (relative hrefs resolved
  against the page URL; the light `media=` variant wins over dark;
  `apple-touch-icon` then `mask-icon` as fallbacks), else `/favicon.ico`,
  and returns the bytes with the upstream (or extension-sniffed)
  `Content-Type`. One cache entry per port (TTL 3600 s); `?refresh=1`
  (called by the sync on every run) re-fetches at most every 300 s.
  Failures serve stale cache, else 404 — the tab then silently shows
  its letter, never a broken image.
- Override per port in `port-profiles.yaml` with `icon:` (missing key =
  default favicon URL; `""`/null/false = letter fallback; an
  `http(s)://` or `/` string = `url` icon; any other string =
  `dashboard` icon name; a `{type: ...}` dict is passed through as-is
  for `dashboard`/`lucide`/`custom`/`url`).
- Unit tests: `tests/test_favicon.py` (relative href, svg, missing,
  redirect, light preference) and `tests/test_gateway_apps.py`
  (sync icon + override shapes + change detection).

## 6. Guardrails to make this hold in most cases
- A registry-vs-allowlist test: every Muximux app with `url: /p/<port>/` must be allowed by the gateway (no 403).
- A per-service smoke test in `tests/e2e/` (first load, login if needed, WS 101) run by a new `bin/portal-compat <port>` (to build, M-323; `bin/ports check` already exists and only checks registry drift).
- Origin guard (ADR decision 11): state-changing methods and WS upgrades require a real `Origin` in {portal host, tailnet host, localhost, 127.0.0.1}; forged origins get 403. Override per port with `origin: none|<explicit>` in `port-profiles.yaml`.
- No service is added to `gateway-apps.json` without passing section 4 with evidence on the card.

## 7. Perf baseline (M-341, 2026-11-14, same machine `<server>`)
Chunk `/_next/static/chunks/02iy0-j94q3_r.js` (Multica :3701), 5-run medians:
| Path | Bytes on wire (gzip offered) | p50 latency |
| --- | --- | --- |
| Direct `:3701` | 8,553 B (`Content-Encoding: gzip`) | ~1.2 ms |
| Gateway `:3738` (identity — strips `Accept-Encoding` upstream) | 35,647 B | ~5.3 ms |
| Portal Caddy `:3720` after M-341 (`encode zstd gzip`, typed match) | **8,710 B** (`Content-Encoding: gzip`; `zstd` when offered) | ~5.4 ms (encode adds no measurable latency) |
What M-341 changed: `routes/pgw.inc` encodes `/p/*` (one value per
`header` line, `*` suffix required — upstream Content-Types carry `; charset=`
params and the matcher is exact; same-header lines are OR'd; no
`text/event-stream` line so SSE keeps streaming under `flush_interval -1`);
`lib/pgw/sw.js` skips cross-origin subresources, serves `/_next/static/*`
cache-first (content-hash URLs, no stale risk), streams non-GET/HEAD bodies
(`duplex: half`); `bin/portal-gw` streams request bodies upstream
(was `await request.read()`) with an explicit keep-alive pool (100/20, 30 s).
Scroll/render: headless rAF benchmark on the login document shows no SW
jank (181 frames/3 s, worst 17 ms, identical pre/post-control); ext.js
MutationObserver adds nothing measurable on 200 DOM inserts (0 longtasks
with and without). The Owner's lag matches DevTools-open profiling
(Network recording + 85 console errors on their screenshot), not the
gateway path — retest with DevTools closed before further render work.
Rollback per change is in the commit messages (`git log --oneline -4` from
`2ec577e`).

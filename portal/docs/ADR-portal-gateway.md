# ADR: Portal port gateway, tailnet entry and test environment (M-237)

Status: accepted by the Owner, 2026-11-13. Owner of record: Claude Architect (Leader).

## Context
One portal at `<portal-public-host>` (Muximux front end, behind Cloudflare Access) must open every registered local service (Multica :3701, Cockpit :9090, and all ports in the registry) as a tab, with deep links, and without wildcard DNS, per-app CNAMEs or extra tailscale ports. When the client device is on Tailscale, the portal renders the tailnet host `<tailnet-host>` directly in the iframe; otherwise it uses the Cloudflare path.

## Decisions
1. **Two entry paths, one router.** Caddy `:3720` is host-based. Cloudflare serves `<portal-public-host>`; `tailscale serve --bg --https=443 http://127.0.0.1:3720` serves the tailnet host. Both import `routes/pgw.inc`.
2. **Single-hostname path gateway** `/p/<port>/...` (`bin/portal-gw`, aiohttp :3738). Allowed ports = port registry + {3701, 9090} - `ports-deny.yaml`. No wildcard host needed.
3. **Combined service worker** served at `/sw.js` (ours + upstream Muximux; one SW per scope). It maps clientId -> port and rewrites app requests to `/p/<port>/...`.
4. **Header/cookie rewriting.** Host/Origin/Referer rewritten to `localhost:<port>` (Multica WS origin check). Frame/CORP/`frame-ancestors` stripped. Cookies: strip Domain, force `Path=/`, add `SameSite=None; Secure; Partitioned` when `X-Forwarded-Proto: https`. HTML gets an injected shim (WebSocket prefixing, `replaceState` stripping `/p/N`).
5. **Tailnet chooser** (`ext.js`): the portal probes the tailnet host (CORS `Access-Control-Allow-Origin: https://<portal-public-host>` + `Access-Control-Allow-Private-Network: true`); on success it frames the tailnet host with `allow="local-network-access"`, else falls back to Cloudflare. Chromium Local Network Access prompt is expected once per browser.
6. **Why not Muximux's built-in Caddy gateway:** it is one hostname per backend (subdomain/wildcard), which violates the single-hostname rule. Phase 2 may port the path gateway into the fork's Go code (`internal/proxy`) after Architect approval.
7. **Docker discovery / podman:** Muximux discovery uses the Docker Engine API on a configurable endpoint; podman's compatible socket answers it (verified directly with a container listing, not through Muximux). Discovery sees containers only; host processes come from the port registry.
8. **Tailscale support in Muximux: none.** The tailnet part is ours (`tailscale serve` + chooser).
9. **Code home:** `https://github.com/ddvnguyen/Muximux-Portal` (public fork). Never commit `.ports.token`, `routes/all.inc`, `muximux/data/`, `logs/`, `tunnel-config*.json`, `cf-backups/`, env files. Portal code goes under `portal/` on a branch, PR into the fork.
10. **Test environment (decided 2026-11-13):** a real-path test needs (a) a Cloudflare Access service token so a headless browser passes `<portal-public-host>`, and (b) a Playwright browser that is itself on the tailnet. (b) is a rootless podman container running `tailscaled` in userspace mode with a SOCKS5 proxy on 127.0.0.1:1055 (`spike/tsprobe/run.sh`); Chromium is launched with `--proxy-server=socks5://127.0.0.1:1055` so tailnet names resolve through MagicDNS. No TUN device, no root.

## Consequences
- Acceptance = real post-login evidence (not login pages) for each app, in both a same-site and a cross-site parent, on both paths.
- The probe node is a separate tailnet device (`portal-e2e-probe`); delete it from the admin console when testing ends.

## Open items
- Cockpit post-login: its `/cockpit/socket` WebSocket never reaches the gateway (M-323 item 1).
- Chooser, CORS/PNA headers, per-app matrix (opencode, Grafana, DSH-X, Freebuff Desktop, DSH-Z): M-323.
- Cloudflare Access service token for the test run (Owner-only; see M-237).

## Decision 11 (2026-11-14): origin handling
Default `origin: rewrite` for every port, guarded: the gateway validates the real `Origin`/`Sec-Fetch-Site` on state-changing methods and WebSocket upgrades against {portal host, tailnet host, localhost/127.0.0.1} before rewriting to `http://localhost:<port>`; others get 403. Reason: the gateway sets `SameSite=None` cookies, so an unguarded rewrite would allow cross-site CSRF into the upstream apps. Override per port with `origin: none|<explicit>` in `port-profiles.yaml`. Implemented by M-323.

## Decision 12 (2026-11-23, M-540): tailnet gateway iframe navigations
Symptom (Owner, Zen/Firefox + Tailscale): Cockpit tab showed "will not allow
... embedded"; Multica only worked via the public portal host.

Root cause: `portal-front` routed prefix-less requests to Muximux unless
(a) the `Sec-Fetch-Dest: document` claim redirect fired or (b) a
same-origin `/p/<port>/` Referer matched (navigations excluded). Iframe
navigations (`Sec-Fetch-Dest: iframe`, e.g. Multica `/moct/`) got the
Muximux SPA with `X-Frame-Options: SAMEORIGIN` + `frame-ancestors 'self'`,
which the cross-site tailnet embed refuses. Cockpit pushState URLs
(`/podman`, `/cockpit/login`) likewise had no hint: the gateway sends
`Referrer-Policy: no-referrer`, and iframe reloads carry no usable Referer.

Fix (cookie over Referer-policy change — no-referrer stays for privacy):
1. Claim redirect fires for `Sec-Fetch-Dest` in (`document`, `iframe`).
2. The gateway remembers the last `/p/<port>/` document per browser in a
   `pgw-port` cookie (`Path=/; Secure; SameSite=None; Partitioned`,
   document responses only). The front consults it for prefix-less
   iframe/document navigations when the Referer hint is absent.
3. The front forwards the original `Host` as `X-Forwarded-Host` so the
   gateway's `request_is_https` (hosts from `gateway-apps.json` claim
   entries, same file the front reads) recognizes the TLS portal hosts
   behind `tailscale serve` TLS termination.
4. Defence in depth: Muximux responses on the tailnet host drop
   `X-Frame-Options` and widen to `frame-ancestors 'self'
   https://<portal-public-host>` — a mis-route renders the SPA, never a
   blank browser error.

Verified: curl matrix (document/iframe x claim/cookie/Referer/no hint),
portal-compat 9090 7/7, Playwright Chromium+Firefox through the tailnet
host from a cross-site embedder (Cockpit login loads, no frame-block).

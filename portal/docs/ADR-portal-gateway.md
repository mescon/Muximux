# ADR: Portal port gateway, tailnet entry and test environment (M-237)

Status: accepted by the Owner, 2026-11-13. Owner of record: Claude Architect (Leader).

## Context
One portal at `https://portal.ddvnguyen.com` (Muximux front end, behind Cloudflare Access) must open every registered local service (Multica :3701, Cockpit :9090, and all ports in the registry) as a tab, with deep links, and without wildcard DNS, per-app CNAMEs or extra tailscale ports. When the client device is on Tailscale, the portal renders the tailnet host `https://ddv-server-00.zubron-denebola.ts.net` directly in the iframe; otherwise it uses the Cloudflare path.

## Decisions
1. **Two entry paths, one router.** Caddy `:3720` is host-based. Cloudflare serves `portal.ddvnguyen.com`; `tailscale serve --bg --https=443 http://127.0.0.1:3720` serves the tailnet host. Both import `routes/pgw.inc`.
2. **Single-hostname path gateway** `/p/<port>/...` (`bin/portal-gw`, aiohttp :3738). Allowed ports = port registry + {3701, 9090} - `ports-deny.yaml`. No wildcard host needed.
3. **Combined service worker** served at `/sw.js` (ours + upstream Muximux; one SW per scope). It maps clientId -> port and rewrites app requests to `/p/<port>/...`.
4. **Header/cookie rewriting.** Host/Origin/Referer rewritten to `localhost:<port>` (Multica WS origin check). Frame/CORP/`frame-ancestors` stripped. Cookies: strip Domain, force `Path=/`, add `SameSite=None; Secure; Partitioned` when `X-Forwarded-Proto: https`. HTML gets an injected shim (WebSocket prefixing, `replaceState` stripping `/p/N`).
5. **Tailnet chooser** (`ext.js`): the portal probes the tailnet host (CORS `Access-Control-Allow-Origin: https://portal.ddvnguyen.com` + `Access-Control-Allow-Private-Network: true`); on success it frames the tailnet host with `allow="local-network-access"`, else falls back to Cloudflare. Chromium Local Network Access prompt is expected once per browser.
6. **Why not Muximux's built-in Caddy gateway:** it is one hostname per backend (subdomain/wildcard), which violates the single-hostname rule. Phase 2 may port the path gateway into the fork's Go code (`internal/proxy`) after Architect approval.
7. **Docker discovery / podman:** Muximux discovery uses the Docker Engine API on a configurable endpoint; podman's compatible socket answers it (verified directly with a container listing, not through Muximux). Discovery sees containers only; host processes come from the port registry.
8. **Tailscale support in Muximux: none.** The tailnet part is ours (`tailscale serve` + chooser).
9. **Code home:** `https://github.com/ddvnguyen/Muximux-Portal` (public fork). Never commit `.ports.token`, `routes/all.inc`, `muximux/data/`, `logs/`, `tunnel-config*.json`, `cf-backups/`, env files. Portal code goes under `portal/` on a branch, PR into the fork.
10. **Test environment (decided 2026-11-13):** a real-path test needs (a) a Cloudflare Access service token so a headless browser passes `portal.ddvnguyen.com`, and (b) a Playwright browser that is itself on the tailnet. (b) is a rootless podman container running `tailscaled` in userspace mode with a SOCKS5 proxy on 127.0.0.1:1055 (`spike/tsprobe/run.sh`); Chromium is launched with `--proxy-server=socks5://127.0.0.1:1055` so tailnet names resolve through MagicDNS. No TUN device, no root.

## Consequences
- Acceptance = real post-login evidence (not login pages) for each app, in both a same-site and a cross-site parent, on both paths.
- The probe node is a separate tailnet device (`portal-e2e-probe`); delete it from the admin console when testing ends.

## Open items
- Cockpit post-login: its `/cockpit/socket` WebSocket never reaches the gateway (M-323 item 1).
- Chooser, CORS/PNA headers, per-app matrix (opencode, Grafana, DSH-X, Freebuff Desktop, DSH-Z): M-323.
- Cloudflare Access service token for the test run (Owner-only; see M-237).

## Candidate changes (M-323, not yet deployed)
- The front proxy routes `/p/<port>/` WebSockets to the gateway and uses a same-origin gateway Referer for first-load subresources until the worker controls the page. The gateway admits only its own script hash under an upstream CSP, preserves root cookie scope, and denies all ports on startup until the deny list is readable.
- Gateway app claims route Multica links into its Muximux tab. The chooser checks a CORS response and falls back when the probe fails. Isolated tests do not prove real Cloudflare Access, browser local-network permission, or authenticated Cockpit.
- Deploy as one coordinated cutover: save the existing `bin/portal-front`, `bin/portal-gw`, `lib/pgw/sw.js`, `routes/pgw.inc`, `gateway-apps.json`, and `ui/ports/` files plus the current Muximux app settings; install the matching versions together, validate Caddy, stop only the identified spike listener, start the `portal-gw` unit, restart `portal-front`, reload Caddy, and run the acceptance tests. Roll back by restoring the saved files and app settings, restarting the front and previous gateway, and reloading Caddy. Do not alter Tailscale Serve. This is a proposed rollback procedure, not evidence that the cutover has occurred.

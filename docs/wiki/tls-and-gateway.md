# TLS and Gateway

> **3.1.0 note:** The `server.gateway:` Caddyfile path is no longer the configured way to serve additional sites. Use `server.gateway_sites:` (declarative YAML) instead. Muximux rejects a non-empty `server.gateway:` value at startup. On first boot under 3.1.0, Muximux auto-migrates 3.0.x configs to `gateway_sites:`. This works only when the Caddyfile converts with no loss. If the conversion would lose settings, Muximux stops at startup with an error. Then run `muximux migrate-gateway <Caddyfile>` to convert the file by hand. The converter does not read TLS connection settings. In particular it does not carry over client-certificate authentication (`client_auth`), and it gives no warning about it. Review every migrated `gateway_sites:` entry, and keep any site that needs client certificates off the gateway until you have checked it. The Caddyfile recipes on [Gateway Examples](gateway-examples.md) are still useful as a reference for what each gateway-site option does. See the [Migration notes in the 3.1.0 CHANGELOG](https://github.com/mescon/Muximux/blob/main/CHANGELOG.md#breaking-changes) for details.

## Overview

Muximux always serves on one port (configured in `server.listen`). TLS and gateway features are optional -- when they are on, an embedded Caddy server (a web server built into Muximux) handles the user-facing port. Caddy forwards traffic to the Go server internally.

---

## When Caddy Starts

Caddy starts automatically when **either** `tls` or one or more `gateway_sites` is configured. If neither is set, Caddy does not start. Go serves directly, with no Caddy overhead.

| `tls` | `gateway_sites` | What happens |
|-------|-----------|-------------|
| No | No | Go serves on `listen` directly. No Caddy. |
| Yes | No | Caddy serves HTTPS on `listen`, Go on internal port. |
| No | Yes | Caddy serves HTTP on `listen` + extra sites, Go on internal port. |
| Yes | Yes | Caddy serves HTTPS on `listen` + extra sites, Go on internal port. |

Muximux computes the internal port automatically: listen port + 10000 (e.g., `:8080` becomes `127.0.0.1:18080`). You never configure it.

For deployments behind another reverse proxy (a server that forwards requests to another server), set `server.gateway_listen`. This binds gateway sites on a non-privileged port. See [Running Behind Another Reverse Proxy](#running-behind-another-reverse-proxy-gateway_listen) below.

---

## Auto-HTTPS (Let's Encrypt)

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"
    email: "admin@example.com"
```

- Caddy gets and renews certificates automatically from Let's Encrypt.
- `email` is required. Let's Encrypt uses it for registration and expiry notifications.
- Caddy also listens on ports 80 and 443 for the ACME challenge (the check Let's Encrypt uses to confirm that you control the domain) -- make sure these ports are accessible from the internet.
- After setup, access Muximux at `https://muximux.example.com`.

---

## Manual TLS Certificates

```yaml
server:
  listen: ":8443"
  tls:
    cert: /path/to/cert.pem
    key: /path/to/key.pem
```

- Both `cert` and `key` must be set (or both left empty).
- You cannot use `domain` and `cert`/`key` at the same time.
- Muximux serves HTTPS on the configured port using your certificates.

---

## Gateway (Additional Sites)

```yaml
server:
  listen: ":8080"
  gateway_sites:
    - domain: grafana.example.com
      backend_url: http://localhost:3000
    - domain: wiki.example.com
      backend_url: http://localhost:3001
```

Each entry declares a public hostname (`domain`) and the upstream server it forwards to (`backend_url`). The `tls` field per site defaults to `auto` (Let's Encrypt). You can also edit these visually under **Settings → Gateway**.

With this, you can reverse proxy other sites and services on your network that don't need to be in the Muximux menu. Examples are Grafana dashboards, wiki pages, or any other web app that just needs HTTPS or a public hostname. Everything runs through the same Caddy instance.

When a gateway site uses auto-HTTPS (the default), Caddy automatically gets TLS certificates for that domain. It also listens on ports 80 and 443 for that domain. Make sure those ports are accessible -- in Docker, add `-p 80:80 -p 443:443` to your port mappings.

> **Note:** Adding or removing `gateway_sites` reloads Caddy in place -- no full restart is required.

---

## Running Behind Another Reverse Proxy (`gateway_listen`)

By default, Caddy binds ports 80 and 443 directly, so that it can serve gateway sites with automatic HTTPS. For this, you must do one of these things:

- Run Muximux as root.
- Grant the binary `CAP_NET_BIND_SERVICE`.
- Let systemd hand it those sockets.

You need none of these when Muximux runs **behind** another reverse proxy that already terminates TLS (handles the HTTPS connection). Examples are Traefik, nginx, Cloudflare Tunnel, a router-level Caddy, etc.

For that setup, set `server.gateway_listen` to a non-privileged address. Then:

- Caddy binds that address instead of 80/443.
- Caddy serves gateway sites as plain HTTP. The exception is a site with `tls: custom`: Caddy serves it as HTTPS on that address, with your certificate. The upstream proxy must then connect to that site over HTTPS.
- The upstream proxy handles TLS.

For example:

```yaml
server:
  listen: ":8080"          # Muximux dashboard
  gateway_listen: ":8443"  # Gateway sites, served as plain HTTP

  gateway_sites:
    - domain: "sonarr.example.com"
      backend_url: "http://192.168.1.5:8989"
      tls: auto    # ignored when gateway_listen is set; site is HTTP-only
    - domain: "radarr.example.com"
      backend_url: "http://192.168.1.6:7878"
      tls: none    # explicit HTTP
```

Your upstream proxy then forwards both the dashboard and the gateway hosts to Muximux:

```
sonarr.example.com           -> https terminated upstream -> http://muximux-host:8443
radarr.example.com           -> https terminated upstream -> http://muximux-host:8443
muximux.example.com          -> https terminated upstream -> http://muximux-host:8080
```

**Behaviour rules with `gateway_listen` set:**

| Per-site `tls` | Gateway port serves | Notes |
|---|---|---|
| `auto` | Plain HTTP | Cert issuance via HTTP-01 needs port 80, which we don't bind here. The site still works but the operator is expected to terminate TLS upstream. |
| `none` | Plain HTTP | Same outcome as `auto` in this mode. |
| `custom` | HTTPS with operator-supplied cert | Useful for split-DNS where some clients hit Muximux directly. The cert must be valid for the gateway domain. |

Do not mix `tls: custom` sites with `tls: auto` or `tls: none` sites while `gateway_listen` is set. All gateway sites share one port, and Caddy cannot serve HTTP and HTTPS on the same port. Caddy then rejects the configuration, so the start or reload fails. Give all gateway sites the same protocol.

The dashboard (`server.listen`) is independent. It always binds the port you set there, with the TLS setup that `server.tls.*` describes.

**Format**: `gateway_listen` accepts anything `net.Listen` does: `":8443"` (all interfaces), `"127.0.0.1:8443"` (loopback only), `"[::]:8443"` (IPv6 all). Empty (the default) restores the auto-binding behaviour.

If Caddy can't bind the port you choose, Muximux exits at startup. It shows a one-line hint that lists the three fixes (run as root, `setcap`, or `gateway_listen`).

---

## TLS + Gateway Together

You can combine both:

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"
    email: "admin@example.com"
  gateway_sites:
    - domain: grafana.example.com
      backend_url: http://localhost:3000
      tls: auto
```

Caddy handles HTTPS for Muximux **and** serves the additional gateway sites. Each site can have its own TLS mode (`auto`, `custom`, or `none`).

---

## Using Muximux as Your Only Reverse Proxy

Muximux can be the only reverse proxy on your server. Then you can use its embedded Caddy to handle HTTPS for your dashboard and all your other services. This gives you automatic TLS certificates, HTTP→HTTPS redirects, and a single entry point for everything.

### 1. Configure Muximux with a domain and gateway sites

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"
    email: "admin@example.com"
  gateway_sites:
    - domain: grafana.example.com
      backend_url: http://localhost:3000
    - domain: sonarr.example.com
      backend_url: http://localhost:8989
    - domain: plex.example.com
      backend_url: http://localhost:32400
```

### 2. Each gateway site forwards to its backend

Each `gateway_sites` entry pairs a public `domain` with the `backend_url` that it forwards to. With the default `tls: auto`, every domain automatically gets a Let's Encrypt certificate. Caddy handles all renewals.

### 3. Expose ports 80 and 443

Caddy needs port 80 for ACME HTTP-01 challenges and HTTP→HTTPS redirects. It needs port 443 to serve HTTPS.

**Docker Compose:**

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./data:/app/data
```

You do not need to expose port 8080 -- Caddy handles all traffic on 80/443 and forwards it to the Go server internally.

**Binary / systemd:**

No extra configuration is needed -- Caddy binds to ports 80 and 443 directly. Make sure no other service (like nginx or Apache) uses those ports.

### 4. Point your DNS records

Create A/AAAA records for each domain pointing to your server:

```
muximux.example.com  → your-server-ip
grafana.example.com  → your-server-ip
sonarr.example.com   → your-server-ip
plex.example.com     → your-server-ip
```

### 5. Access your services

DNS must propagate, and Caddy must get certificates (usually within seconds). Then you can open:

- `https://muximux.example.com` -- your dashboard
- `https://grafana.example.com` -- served by Caddy directly to Grafana
- `https://sonarr.example.com` -- served by Caddy directly to Sonarr

Caddy automatically redirects all HTTP requests (port 80) to HTTPS (port 443).

> **Tip:** Caddy serves gateway sites directly -- they do not go through Muximux's built-in reverse proxy. You can still add these apps to Muximux's dashboard using their `https://` URLs and `open_mode: new_tab` or `open_mode: iframe`.

For more practical examples -- custom headers, Docker networking, security headers, and common homelab apps -- see [Gateway Examples](gateway-examples.md).

---

## Important Notes

- The built-in reverse proxy (`proxy: true` per app) works in **all** modes -- it is independent of Caddy.
- Caddy's admin API is disabled for security.
- When you use auto-HTTPS with a domain, Caddy handles the user-facing port entirely. The `listen` address becomes the internal forward target.
- When a gateway site uses auto-HTTPS, Caddy automatically listens on ports 80 and 443 for that domain even if `tls.domain` is not set for Muximux itself.

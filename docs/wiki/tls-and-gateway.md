# TLS and Gateway

## Overview

Muximux always serves on one port (configured via `server.listen`). TLS and gateway features are optional -- when enabled, an embedded Caddy server handles the user-facing port and forwards traffic to the Go server internally.

---

## When Caddy Starts

Caddy starts automatically when **either** `tls` or `gateway` is configured. If neither is set, Caddy does not start and Go serves directly -- zero overhead.

| `tls` | `gateway` | What happens |
|-------|-----------|-------------|
| No | No | Go serves on `listen` directly. No Caddy. |
| Yes | No | Caddy serves HTTPS on `listen`, Go on internal port. |
| No | Yes | Caddy serves HTTP on `listen` + extra sites, Go on internal port. |
| Yes | Yes | Caddy serves HTTPS on `listen` + extra sites, Go on internal port. |

The internal port is computed automatically: listen port + 10000 (e.g., `:8080` becomes `127.0.0.1:18080`). It is never user-configured.

---

## Auto-HTTPS (Let's Encrypt)

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"
    email: "admin@example.com"
```

- Caddy obtains and renews certificates automatically via Let's Encrypt.
- `email` is required (used for Let's Encrypt registration and expiry notifications).
- Caddy also listens on ports 80 and 443 for the ACME challenge -- make sure these ports are accessible from the internet.
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
  gateway: /path/to/sites.Caddyfile
```

The referenced file uses standard Caddyfile syntax. Example `sites.Caddyfile`:

```
grafana.example.com {
    reverse_proxy localhost:3000
}

wiki.example.com {
    reverse_proxy localhost:3001
}
```

This lets you reverse proxy other sites and services on your network that don't need to be in the Muximux menu -- things like Grafana dashboards, wiki pages, or any other web app that just needs HTTPS or a public hostname. Everything runs through the same Caddy instance.

When the gateway Caddyfile contains domain-based site blocks (like `grafana.example.com`), Caddy automatically provisions TLS certificates and listens on ports 80 and 443 for those domains. Make sure those ports are accessible -- in Docker, add `-p 80:80 -p 443:443` to your port mappings.

> **Note:** The gateway file must exist when Muximux starts, or it will fail with an error.

---

## TLS + Gateway Together

You can combine both:

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"
    email: "admin@example.com"
  gateway: /path/to/sites.Caddyfile
```

Caddy handles HTTPS for Muximux **and** serves the additional sites from the Caddyfile. Sites in the Caddyfile can have their own TLS settings.

---

## Using Muximux as Your Only Reverse Proxy

If Muximux is the only reverse proxy on your server, you can use its embedded Caddy to handle HTTPS for your dashboard and all your other services. This gives you automatic TLS certificates, HTTP→HTTPS redirects, and a single entry point for everything.

### 1. Configure Muximux with a domain

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"
    email: "admin@example.com"
  gateway: /app/data/sites.Caddyfile
```

### 2. Create a gateway Caddyfile for your other services

Create `sites.Caddyfile` with your other domains:

```
grafana.example.com {
    reverse_proxy localhost:3000
}

sonarr.example.com {
    reverse_proxy localhost:8989
}

plex.example.com {
    reverse_proxy localhost:32400
}
```

Each domain automatically gets a Let's Encrypt certificate. Caddy handles all renewals.

### 3. Expose ports 80 and 443

Caddy needs port 80 for ACME HTTP-01 challenges and HTTP→HTTPS redirects, and port 443 to serve HTTPS.

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
      - ./sites.Caddyfile:/app/data/sites.Caddyfile:ro
```

Port 8080 does not need to be exposed -- Caddy handles all traffic on 80/443 and forwards to the Go server internally.

**Binary / systemd:**

No extra configuration needed -- Caddy binds to ports 80 and 443 directly. Make sure no other service (like nginx or Apache) is using those ports.

### 4. Point your DNS records

Create A/AAAA records for each domain pointing to your server:

```
muximux.example.com  → your-server-ip
grafana.example.com  → your-server-ip
sonarr.example.com   → your-server-ip
plex.example.com     → your-server-ip
```

### 5. Access your services

Once DNS propagates and Caddy obtains certificates (usually within seconds):

- `https://muximux.example.com` -- your dashboard
- `https://grafana.example.com` -- served by Caddy directly to Grafana
- `https://sonarr.example.com` -- served by Caddy directly to Sonarr

All HTTP requests (port 80) are automatically redirected to HTTPS (port 443).

> **Tip:** Apps in the gateway Caddyfile are served directly by Caddy -- they do not go through Muximux's built-in reverse proxy. You can still add these apps to Muximux's dashboard using their `https://` URLs and `open_mode: new_tab` or `open_mode: iframe`.

For more practical examples -- custom headers, Docker networking, security headers, and common homelab apps -- see [Gateway Examples](gateway-examples.md).

---

## Important Notes

- The built-in reverse proxy (`proxy: true` per app) works in **all** modes -- it is independent of Caddy.
- Caddy's admin API is disabled for security.
- When using auto-HTTPS with a domain, Caddy handles the user-facing port entirely; the `listen` address becomes the internal forward target.
- When a gateway Caddyfile contains domain-based sites, Caddy automatically listens on ports 80 and 443 even if `tls.domain` is not set for Muximux itself.

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

This lets you serve additional sites alongside Muximux through the same Caddy instance. Useful when you want one entry point for multiple services.

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

## Important Notes

- The built-in reverse proxy (`proxy: true` per app) works in **all** modes -- it is independent of Caddy.
- Caddy's admin API is disabled for security.
- When using auto-HTTPS with a domain, Caddy handles the user-facing port entirely; the `listen` address becomes the internal forward target.

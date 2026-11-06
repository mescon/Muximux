# Gateway Examples

> **3.1.0 note:** The recipes below are written as gateway Caddyfile blocks for reference. In 3.1.0+ the supported configuration form is `server.gateway_sites:` declarative YAML. On first boot, Muximux auto-migrates a 3.0.x Caddyfile to the YAML form. Use the [Configuration Reference](configuration.md) for the YAML schema. Use **Settings -> Gateway** in the dashboard to edit visually. The Caddyfile snippets here still describe what each gateway-site option *does*. Map them to the YAML equivalents by hand. Or feed your existing Caddyfile through `muximux migrate-gateway` to see the conversion side-by-side.

Practical recipes for using Muximux's embedded Caddy as a reverse proxy (a server that forwards requests to another server) for your other services. See [TLS & HTTPS](tls-and-gateway.md) for initial setup.

---

## Prerequisites

Your `config.yaml` needs TLS configured plus one or more `gateway_sites:` entries:

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"    # or cert/key for manual TLS
    email: "admin@example.com"
  gateway_sites:
    - domain: grafana.example.com
      backend_url: http://grafana:3000
      tls: auto                      # auto | none | custom
```

Expose ports 80 and 443. Caddy needs both, for ACME (automatic certificate requests) and for HTTPS. When you add or remove `gateway_sites:` entries, Caddy reloads in place -- no full restart required. Schema changes to existing sites (TLS-mode flip, auth gate toggle) also reload in place.

The recipe sections below show the Caddyfile shape for each scenario. Translate each one into the matching `gateway_sites:` entry fields (see [Configuration Reference](configuration.md) for the full schema).

### What `gateway_sites` supports

Each gateway site proxies one `backend_url` per domain. The fields you can set are:

- `backend_url` -- the `http://` or `https://` backend (no path, query, or transport flags)
- `tls` -- certificate mode (`auto`, `none`, or `custom`)
- `proxy_headers` -- extra headers sent to the backend
- `forwarded_headers` -- emit `X-Forwarded-*` and `X-Real-IP` (default true)
- `streaming` -- disable response buffering for streaming backends
- `strip_frame_blockers` -- remove `X-Frame-Options`/CSP frame headers so the app can load in an iframe
- `backend_skip_tls_verify` -- skip verifying a self-signed / untrusted `https` backend certificate (e.g. Proxmox)
- `require_auth` / `min_role` / `allowed_groups` -- gate the site behind Muximux auth (see [Gateway Auth](gateway-auth.md))

Anything outside this list is not a gateway-site option. This includes basic auth, rate limiting, subpath routing, wildcard host matching, and arbitrary `transport` tuning like `read_buffer`. Those recipes are marked below. They need an external proxy in front of Muximux.

---

## Basic Reverse Proxy

The simplest case: one domain, one backend.

```
grafana.example.com {
    reverse_proxy localhost:3000
}
```

Caddy gets a Let's Encrypt certificate. It redirects HTTP to HTTPS. It proxies all traffic without any further configuration.

---

## Multiple Services

Add as many site blocks as you need:

```
grafana.example.com {
    reverse_proxy localhost:3000
}

sonarr.example.com {
    reverse_proxy localhost:8989
}

radarr.example.com {
    reverse_proxy localhost:7878
}

prowlarr.example.com {
    reverse_proxy localhost:9696
}

jellyfin.example.com {
    reverse_proxy localhost:8096
}
```

Each domain gets its own certificate. Caddy handles renewals automatically.

### Faster: Import from Docker

Those backends can run as Docker containers. Then **Apps tab → Discover from Docker** (or **Gateway tab → Discover from Docker**) builds the same site list in one click:

1. Pick "Gateway" routing for each row.
2. Enter the public domain. Muximux creates the gateway site and reloads Caddy in the same transaction.

URLs refresh automatically when container IPs change. See [Docker Discovery](docker-discovery.md).

Or enable `discovery.docker.auto_import`. Then labeled containers appear automatically, without opening the modal (see [Docker Discovery](docker-discovery.md)). Note: in update/sync mode, gateway-only label changes are applied only when an app field also changes.

Labels on your compose file pre-fill name, icon, port, and a stable tracking key:

```yaml
# docker-compose.yml
services:
  sonarr:
    image: linuxserver/sonarr
    labels:
      - muximux.discovery.id=sonarr-stable   # survives docker-compose --force-recreate
      - muximux.app.name=Sonarr
      - muximux.app.icon=sonarr
      - muximux.app.group=Media
      # port + scheme auto-detected from the catalog for known images
```

---

## WebSocket Support

Caddy proxies WebSocket connections automatically, so apps like Homeassistant, Code Server, Portainer, and Gotify work with no extra configuration.

```
homeassistant.example.com {
    reverse_proxy localhost:8123
}
```

---

## Custom Headers

Some apps need specific headers to work behind a reverse proxy.

On a gateway site, Muximux sends the forwarded headers for you. Put any other backend headers in `proxy_headers`:

```yaml
gateway_sites:
  - domain: app.example.com
    backend_url: http://app:8080
    proxy_headers:               # extra headers sent to the backend
      X-Api-Key: "secret"
    forwarded_headers: true      # X-Forwarded-* + X-Real-IP (default true; set false to suppress)
```

### Plex

Plex needs large header buffers and the real client IP:

```
plex.example.com {
    reverse_proxy localhost:32400 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        transport http {
            read_buffer 8192
        }
    }
}
```

> Not available via `gateway_sites`: this requires an external Caddy/nginx in front of Muximux. The embedded gateway only proxies one `backend_url` per domain. The gateway sends forwarded headers (`X-Real-IP`, `X-Forwarded-*`) automatically. You cannot configure the custom `transport`/`read_buffer` tuning on a gateway site.

### Proxmox

Proxmox serves its web UI on `:8006` with a self-signed certificate. Set `backend_skip_tls_verify: true`, so that the gateway does not reject that certificate:

```yaml
gateway_sites:
  - domain: proxmox.example.com
    backend_url: https://192.168.1.10:8006   # https + self-signed
    tls: auto
    backend_skip_tls_verify: true            # skip verifying the backend cert
```

`backend_skip_tls_verify` only applies to an `https://` `backend_url`. It affects only the hop from the gateway to the backend -- it does not affect the public-facing certificate that Caddy serves for `proxmox.example.com`. (Label form: `muximux.gateway.skip_tls_verify=true`.)

### Vaultwarden

Vaultwarden needs both the web vault and WebSocket notifications:

```
vault.example.com {
    reverse_proxy localhost:8082
}
```

Vaultwarden v1.29+ handles WebSocket on the same port, so a simple `reverse_proxy` is sufficient.

---

## Adding Security Headers

Harden responses with security headers:

```
sonarr.example.com {
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
        -Server
    }
    reverse_proxy localhost:8989
}
```

The `-Server` removes Caddy's `Server` header from responses.

---

## Basic Auth on a Service

Protect a service that has no built-in authentication:

```
filebrowser.example.com {
    basicauth {
        admin $2a$14$...  # bcrypt hash
    }
    reverse_proxy localhost:8085
}
```

Generate a bcrypt hash with Caddy's own tool (`caddy hash-password`) or any bcrypt utility -- for example `htpasswd -nbBC 12 "" 'your-password' | cut -d: -f2`. Muximux's embedded Caddy runs internally only. It does not have a command-line tool of its own.

> Not available via `gateway_sites`: this requires an external Caddy/nginx in front of Muximux. The embedded gateway only proxies one `backend_url` per domain. To gate a site behind Muximux's own login instead, set `require_auth` on the gateway site (see [Gateway Auth](gateway-auth.md)).

---

## Rate Limiting

Protect public-facing services from abuse:

```
api.example.com {
    rate_limit {
        zone dynamic_zone {
            key {remote_host}
            events 10
            window 1s
        }
    }
    reverse_proxy localhost:9000
}
```

> **Note:** Rate limiting requires the third-party `rate_limit` Caddy module. This module is **not** compiled into Muximux's embedded Caddy. Only the standard module set is included: reverse proxying, headers, TLS, and basic auth. So this directive will not work here. For rate limiting, put Muximux behind an external proxy that provides it.

---

## Subpath Routing

Route different paths on the same domain to different backends:

```
services.example.com {
    handle /grafana/* {
        reverse_proxy localhost:3000
    }
    handle /prometheus/* {
        reverse_proxy localhost:9090
    }
    handle {
        respond "Not found" 404
    }
}
```

Make sure each service is configured to serve from the correct subpath (e.g., Grafana's `GF_SERVER_ROOT_URL=https://services.example.com/grafana/`).

> Not available via `gateway_sites`: this requires an external Caddy/nginx in front of Muximux. The embedded gateway only proxies one `backend_url` per domain (no path or `handle` routing).

---

## Wildcard Domains

You can use a wildcard DNS record (`*.example.com → your-server-ip`). Then you add new services by just appending site blocks to the Caddyfile. You need no DNS changes per service.

> Not available via `gateway_sites`: this requires an external Caddy/nginx in front of Muximux. The embedded gateway only proxies one `backend_url` per domain, with no wildcard host matching. Add each subdomain as its own gateway site instead.

For wildcard TLS certificates, you need a DNS provider plugin for the ACME DNS-01 challenge (a check that proves domain control through a DNS record):

```
*.example.com {
    tls {
        dns cloudflare {env.CF_API_TOKEN}
    }

    @grafana host grafana.example.com
    handle @grafana {
        reverse_proxy localhost:3000
    }

    @sonarr host sonarr.example.com
    handle @sonarr {
        reverse_proxy localhost:8989
    }

    handle {
        abort
    }
}
```

> **Note:** DNS provider plugins (like `cloudflare`) must be compiled into the Caddy binary. Muximux's embedded Caddy includes the standard modules only. If you need DNS-01 challenges, you have two choices. Run Muximux behind an external Caddy instance that includes the DNS plugin. Or run certbot separately and use `tls.cert`/`tls.key` with manual certificates.

---

## Docker Networking

When you run Muximux in Docker, Muximux must be able to reach your gateway backends. There are two approaches:

### Same Docker network

Add your services to the same Docker network as Muximux and use container names:

```yaml
# docker-compose.yml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./data:/app/data
    networks:
      - proxy

  grafana:
    image: grafana/grafana
    networks:
      - proxy
    # No need to expose ports -- Caddy connects via the Docker network

networks:
  proxy:
    name: proxy
```

Then in your gateway site, use the container name as the backend host:

```yaml
gateway_sites:
  - domain: grafana.example.com
    backend_url: http://grafana:3000
```

### Host networking

Alternatively, use `network_mode: host` or reference `host.docker.internal`:

```yaml
gateway_sites:
  - domain: grafana.example.com
    backend_url: http://host.docker.internal:3000
```

> **Note:** `host.docker.internal` works on Docker Desktop and on Linux with Docker 20.10+ (add `extra_hosts: ["host.docker.internal:host-gateway"]` to your compose file).

---

## Putting It All Together

A complete example for a typical homelab:

**`config.yaml`:**

```yaml
server:
  listen: ":8080"
  tls:
    domain: "home.example.com"
    email: "admin@example.com"
  gateway_sites:
    - domain: grafana.example.com
      backend_url: http://grafana:3000
      tls: auto
    - domain: sonarr.example.com
      backend_url: http://sonarr:8989
      tls: auto

apps:
  - name: Grafana
    url: https://grafana.example.com
    icon: { type: dashboard, name: grafana }
    open_mode: iframe
    enabled: true

  - name: Sonarr
    url: https://sonarr.example.com
    icon: { type: dashboard, name: sonarr }
    open_mode: iframe
    enabled: true

  - name: Proxmox
    url: https://proxmox.example.com
    icon: { type: lucide, name: server }
    open_mode: new_tab     # Proxmox doesn't work in iframes
    health_check: false    # Proxmox health endpoint isn't standard
    enabled: true
```

Gateway sites only accept an `http://` or `https://` `backend_url` with no path, query, or subpath routing. This example shows Proxmox as a `new_tab` dashboard app, because its UI does not embed in an iframe. But it could also be a gateway site, because `backend_skip_tls_verify` handles its self-signed backend (see the Proxmox recipe above).

**`docker-compose.yml`:**

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./data:/app/data
    networks:
      - proxy
    restart: unless-stopped

networks:
  proxy:
    external: true
```

Each gateway site you add automatically gets HTTPS. Add a corresponding app entry in `config.yaml` to see it on your dashboard.

---

## Tips

- **In-place reload**: Adding, editing, or removing `gateway_sites` reloads Caddy in place -- no full restart required.
- **Validated before apply**: Muximux validates gateway sites when the config loads, or when you save in Settings → Gateway. It rejects an invalid entry (bad domain, backend with a path, unknown `tls` mode) with a specific error. It does not silently break Caddy.
- **Certificate storage**: Caddy stores certificates in its default data directory. In Docker, this is inside the container -- so Caddy gets new certificates each time you recreate the container. To keep them, mount a volume to `/data` (Caddy's default storage path inside the Muximux container).
- **Dashboard integration**: Caddy proxies gateway sites directly -- they don't go through Muximux's built-in reverse proxy. To see them on the dashboard, add them as apps in `config.yaml` with their `https://` domain URLs.
- **Debug**: If a gateway service isn't working, check Muximux's logs. Caddy logs through Muximux's logging system. It reports certificate errors and unreachable backends.

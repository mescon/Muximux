# Gateway Examples

> **3.1.0 note:** The recipes below are written as gateway Caddyfile blocks for reference. In 3.1.0+ the supported configuration form is `server.gateway_sites:` declarative YAML; a 3.0.x Caddyfile is auto-migrated to the YAML form on first boot. Use the [Configuration Reference](configuration.md) for the YAML schema, and **Settings -> Gateway** in the dashboard to edit visually. The Caddyfile snippets here still describe what each gateway-site option *does*; map them to the YAML equivalents by hand or feed your existing Caddyfile through `muximux migrate-gateway` to see the conversion side-by-side.

Practical recipes for using Muximux's embedded Caddy as a reverse proxy for your other services. See [TLS & HTTPS](tls-and-gateway.md) for initial setup.

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

Expose ports 80 and 443 (Caddy needs both for ACME and HTTPS). Adding or removing `gateway_sites:` entries reloads Caddy in place -- no full restart required. Schema changes to existing sites (TLS-mode flip, auth gate toggle) also reload in place.

The recipe sections below show the Caddyfile shape for each scenario; translate each into the corresponding `gateway_sites:` entry fields (see [Configuration Reference](configuration.md) for the full schema).

---

## Basic Reverse Proxy

The simplest case: one domain, one backend.

```
grafana.example.com {
    reverse_proxy localhost:3000
}
```

Caddy automatically obtains a Let's Encrypt certificate, redirects HTTP to HTTPS, and proxies all traffic. Nothing else needed.

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

If those backends run as Docker containers, **Apps tab → Discover from Docker** (or **Gateway tab → Discover from Docker**) builds the same site list in one click. Pick "Gateway" routing per row and supply the public domain; the gateway site is created and Caddy reloads in the same transaction. URLs auto-refresh as container IPs shift. See [Docker Discovery](docker-discovery.md).

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

Caddy proxies WebSocket connections automatically -- no extra configuration. This works out of the box for apps like Homeassistant, Code Server, Portainer, and Gotify.

```
homeassistant.example.com {
    reverse_proxy localhost:8123
}
```

---

## Custom Headers

Some apps need specific headers to work behind a reverse proxy.

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

### Proxmox

Proxmox uses a self-signed certificate on its backend:

```
proxmox.example.com {
    reverse_proxy https://localhost:8006 {
        transport http {
            tls_insecure_skip_verify
        }
    }
}
```

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

Generate a bcrypt hash with Caddy's own tool (`caddy hash-password`) or any bcrypt utility -- for example `htpasswd -nbBC 12 "" 'your-password' | cut -d: -f2`. Muximux's embedded Caddy is an internal runtime only and does not expose a CLI surface of its own.

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

> **Note:** Rate limiting requires the third-party `rate_limit` Caddy module, which is **not** compiled into Muximux's embedded Caddy (only the standard module set is included: reverse proxying, headers, TLS, and basic auth). This directive will not work here. For rate limiting, put Muximux behind an external proxy that provides it.

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

---

## Wildcard Domains

If you use a wildcard DNS record (`*.example.com → your-server-ip`), you can add new services by just appending site blocks to the Caddyfile. No DNS changes needed per service.

For wildcard TLS certificates, you need a DNS provider plugin for the ACME DNS-01 challenge:

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

> **Note:** DNS provider plugins (like `cloudflare`) must be compiled into the Caddy binary. Muximux's embedded Caddy includes the standard modules only. If you need DNS-01 challenges, consider using Muximux behind an external Caddy instance that includes the DNS plugin, or run certbot separately and use `tls.cert`/`tls.key` with manual certificates.

---

## Docker Networking

When running Muximux in Docker, your gateway backends need to be reachable. There are two approaches:

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

Gateway sites only accept an `http://` or `https://` `backend_url` with no path, query, or extra transport flags. A backend that serves a self-signed HTTPS certificate (like Proxmox) can't be verified through a gateway site, so reach it some other way -- here Proxmox is added as a dashboard app pointed at an external proxy or its direct URL rather than a gateway site.

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
- **Validated before apply**: Gateway sites are validated when the config loads or when you save in Settings → Gateway. An invalid entry (bad domain, backend with a path, unknown `tls` mode) is rejected with a specific error rather than silently breaking Caddy.
- **Certificate storage**: Caddy stores certificates in its default data directory. In Docker, this is inside the container -- certificates are re-obtained on container recreation. To persist them, mount a volume to `/data` (Caddy's default storage path inside the Muximux container).
- **Dashboard integration**: Gateway sites are proxied by Caddy directly -- they don't go through Muximux's built-in reverse proxy. Add them as apps in `config.yaml` using their `https://` domain URLs to see them on the dashboard.
- **Debug**: If a gateway service isn't working, check Muximux's logs. Caddy logs through Muximux's logging system and will report certificate errors and unreachable backends.

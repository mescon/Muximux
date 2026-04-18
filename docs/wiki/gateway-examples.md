# Gateway Examples

Practical recipes for using Muximux's embedded Caddy as a reverse proxy for your other services. Each example is a snippet for your gateway Caddyfile (`sites.Caddyfile` or whatever you named it). See [TLS & HTTPS](tls-and-gateway.md) for initial setup.

---

## Prerequisites

Your `config.yaml` needs two things:

```yaml
server:
  listen: ":8080"
  tls:
    domain: "muximux.example.com"    # or cert/key for manual TLS
    email: "admin@example.com"
  gateway: /path/to/sites.Caddyfile  # points to your gateway file
```

Expose ports 80 and 443 (Caddy needs both for ACME and HTTPS). Restart Muximux after changing `config.yaml`, but the gateway Caddyfile itself is read at startup -- any changes to it also require a restart.

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

Generate a bcrypt hash with: `caddy hash-password`

> **Note:** Muximux ships with Caddy embedded. You can use the Muximux binary itself: `muximux caddy hash-password` (if the Caddy subcommand is exposed), or generate the hash with any bcrypt tool (`htpasswd -nbBC 14 "" password | cut -d: -f2`).

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

> **Note:** Rate limiting requires the `rate_limit` Caddy module. If it's not compiled into Muximux's embedded Caddy, this directive won't work. The standard Caddy modules included cover reverse proxying, headers, TLS, and basic auth.

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
      - ./sites.Caddyfile:/app/data/sites.Caddyfile:ro
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

Then in your Caddyfile, use the container name:

```
grafana.example.com {
    reverse_proxy grafana:3000
}
```

### Host networking

Alternatively, use `network_mode: host` or reference `host.docker.internal`:

```
grafana.example.com {
    reverse_proxy host.docker.internal:3000
}
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
  gateway: /app/data/sites.Caddyfile

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

**`sites.Caddyfile`:**

```
grafana.example.com {
    reverse_proxy grafana:3000
}

sonarr.example.com {
    reverse_proxy sonarr:8989
}

proxmox.example.com {
    reverse_proxy https://proxmox:8006 {
        transport http {
            tls_insecure_skip_verify
        }
    }
}
```

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
      - ./sites.Caddyfile:/app/data/sites.Caddyfile:ro
    networks:
      - proxy
    restart: unless-stopped

networks:
  proxy:
    external: true
```

Each service you add to `sites.Caddyfile` automatically gets HTTPS. Add a corresponding app entry in `config.yaml` to see it on your dashboard.

---

## Tips

- **One restart per change**: Changes to `sites.Caddyfile` require restarting Muximux. Batch your changes.
- **Test syntax first**: The Caddyfile is validated at startup. A syntax error prevents Muximux from starting. Check Caddy's [documentation](https://caddyserver.com/docs/caddyfile) for syntax reference.
- **Certificate storage**: Caddy stores certificates in its default data directory. In Docker, this is inside the container -- certificates are re-obtained on container recreation. To persist them, mount a volume to `/data` (Caddy's default storage path inside the Muximux container).
- **Dashboard integration**: Services in the gateway Caddyfile are proxied by Caddy directly -- they don't go through Muximux's built-in reverse proxy. Add them as apps in `config.yaml` using their `https://` domain URLs to see them on the dashboard.
- **Debug**: If a gateway service isn't working, check Muximux's logs. Caddy logs through Muximux's logging system and will report certificate errors, unreachable backends, and Caddyfile parse failures.

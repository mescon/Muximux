# Deployment

Muximux supports three deployment styles. Pick the one that matches your setup.

| Style | You provide | Muximux does | Ports to expose |
|-------|------------|-------------|----------------|
| **Behind an external proxy** | Traefik / nginx / Caddy handles TLS and auth | Serves the dashboard on a single port | 8080 (internal only) |
| **Standalone with built-in proxy** | Direct access, optional built-in auth | Dashboard + per-app `/proxy/{slug}/` for iframe embedding | 8080 |
| **Full reverse proxy appliance** | DNS records | TLS certificates, HTTP→HTTPS redirects, gateway for other services | 80, 443 |

The built-in per-app reverse proxy (`proxy: true`) works in **all three styles** -- it runs inside the Go server and is independent of Caddy or any external proxy.

---

## Docker

The recommended way to run Muximux in production.

### Behind an external proxy (Traefik, nginx, etc.)

```yaml
# docker-compose.yml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

Set `auth.method: none` or `forward_auth` in config.yaml and let your external proxy handle TLS and authentication.

### As a full reverse proxy appliance

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
    restart: unless-stopped
```

Configure `tls.domain` and `gateway` in config.yaml. See [TLS & HTTPS](tls-and-gateway.md) for a full walkthrough. Port 8080 does not need to be exposed -- Caddy handles all traffic on 80/443.

### Volume

Mount a directory to `/app/data`. This stores:

- `config.yaml` -- Configuration
- `themes/` -- Custom themes
- `icons/` -- Icon caches and custom uploads

### Health Check

The Docker image includes a built-in health check (`GET /api/health` every 30s).

### Updating

```bash
docker compose pull
docker compose up -d
```

---

## Systemd Service

For running Muximux as a native binary on Linux.

Create `/etc/systemd/system/muximux.service`:

```ini
[Unit]
Description=Muximux - Web Application Portal
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=muximux
Group=muximux
WorkingDirectory=/opt/muximux
ExecStart=/opt/muximux/muximux --data /opt/muximux/data
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/muximux/data
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Setup

```bash
# Create user
sudo useradd -r -s /bin/false muximux

# Create directories
sudo mkdir -p /opt/muximux/data
sudo chown -R muximux:muximux /opt/muximux

# Copy binary
sudo cp muximux /opt/muximux/

# Create config
sudo cp config.example.yaml /opt/muximux/data/config.yaml
sudo chown muximux:muximux /opt/muximux/data/config.yaml

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable muximux
sudo systemctl start muximux

# Check status
sudo systemctl status muximux
sudo journalctl -u muximux -f
```

---

## Behind an External Reverse Proxy

If you run Muximux behind Nginx, Traefik, or Caddy (external), you do **not** need Muximux's built-in TLS or gateway. Your external proxy handles TLS termination and optionally authentication. Set `auth.method: none` (if your proxy handles auth) or `forward_auth` (for Authelia/Authentik) in config.yaml.

The per-app reverse proxy (`proxy: true`) still works in this setup -- it runs inside the Go server and is unrelated to the external proxy.

### Nginx

```nginx
server {
    listen 443 ssl;
    server_name muximux.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### Traefik (Docker Labels)

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.muximux.rule=Host(`muximux.example.com`)"
      - "traefik.http.routers.muximux.tls.certresolver=letsencrypt"
      - "traefik.http.services.muximux.loadbalancer.server.port=8080"
    volumes:
      - ./data:/app/data
```

---

## Network Considerations

- **Same network as apps**: Muximux needs to reach your apps (for health checks and the built-in reverse proxy). In Docker, use the same Docker network or host networking.
- **WebSocket support**: If running behind an external reverse proxy, make sure it supports WebSocket connections (needed for real-time health updates).
- **Ports**:
  - Behind an external proxy: only 8080 (or your configured listen port), and it doesn't need to be publicly accessible.
  - Standalone: 8080 (or your configured listen port).
  - Full reverse proxy appliance: 80 and 443. Port 8080 is not needed externally.

# Docker Discovery

Connect Muximux to a Docker daemon and it can enumerate running containers, propose them as apps, and keep their URLs current as IPs shift across restarts. **Off by default** because it requires read access to the Docker socket.

---

## What It Does

- **Scan**: lists running containers and proposes a name, icon, URL, port, and group for each, with confidence ratings (high / medium / low).
- **Import**: one-click adds the chosen containers as apps in the menu, gateway subdomains, or both. Each row picks a routing mode: Direct URL, internal Proxy, or Gateway domain.
- **Refresh**: a background poller (default 60s) re-resolves each tracked container against the daemon and rewrites the saved URL if the container's IP changes. Caddy reloads once per tick when gateway-site URLs change.
- **Auto-import** (optional): with `discovery.docker.auto_import` set to `add`/`update`/`sync`, Muximux imports `muximux.*`-labeled containers with no modal and no click, and keeps them in step with the labels. Off by default. See [Automatic Import](#automatic-import).
- **Detach / Re-link**: per-row controls in Settings → Discovery let you stop auto-managing an entry or re-point it at a different container when you migrate daemons.

---

## When to Use This

✅ You run apps as Docker containers on the same host as Muximux (or on hosts Muximux can reach over the Docker API)
✅ Container IPs change across restarts (default Docker bridge networking) and you don't want to chase them by hand
✅ You're comfortable granting Muximux read access to the Docker socket

❌ Your apps don't run in Docker (the scanner has nothing to enumerate)
❌ Your containers always have stable IPs / hostnames (manual app entries are simpler)
❌ You can't or don't want to expose the Docker socket to Muximux (this feature is opt-in; everything else works without it)

---

## Quickstart

1. **Settings → Discovery**: tick **Enable Docker discovery**, pick a **Network strategy**, click **Test connection**. The banner turns green when Muximux can reach the daemon and identify enough about itself to use the chosen strategy.

2. **Apps tab** (or Gateway tab) → **Discover from Docker**: the modal lists every running container the daemon returned (Muximux's own container is excluded so you can't accidentally import yourself as an app). Each row shows:
   - Suggested name, icon, group (catalog-recognised images get these auto-filled at "medium" confidence)
   - Stable tracking key (operator label > container name > container ID)
   - Resolved URL preview
   - Stability warning when the key is likely to break on a `docker-compose --force-recreate` or swarm task reschedule

   The catalog matcher is lenient about operator prefix conventions: a container named `homelab-sonarr`, `homelab_radarr`, or `prod.plex` still picks up the matching catalog entry just like a bare `sonarr` / `radarr` / `plex` would. Tokens are split on `-`, `_`, and `.`; comparison is exact-token (so `transmissionic` doesn't masquerade as `transmission`). Multi-word app names like `home-assistant` work via an adjacent-pair fallback.

3. **Pick routing per row** and click **Import**:
   - **Direct** - menu links to `http://<container-ip>:<port>` directly
   - **Proxy** - menu links via Muximux's `/proxy/<slug>` path-prefix reverse proxy
   - **Gateway domain** - menu links to `https://<your-subdomain>`; requires also creating the gateway site in the same row

4. **Settings → Discovery → Currently tracked**: every imported app or gateway site appears here. Per-row **Detach** stops auto-management; **Re-link** appears when the saved `DockerEndpoint` no longer matches the configured endpoint (typical after a daemon migration).

---

## Configuration

```yaml
discovery:
  docker:
    enabled: true
    endpoint: unix:///var/run/docker.sock   # or tcp://host:2376
    tls:                                    # only needed for tcp:// with mTLS
      enabled: false
      ca_cert: ""                           # path to CA bundle that signed the daemon cert
      client_cert: ""                       # path to client cert (PEM)
      client_key: ""                        # path to client key (PEM, chmod 600)
    network_strategy: container_ip          # see "Strategies" below
    network_filter: ""                      # optional: scope scans to one network
    host_ip: ""                             # required by host_port strategy
    refresh_interval: 60s                   # poller cadence, [10s, 1h]
    auto_import: off                        # off (default) | add | update | sync (3.2.0)
    lifecycle_enabled: false                # allow start/stop/restart of tracked containers (needs :rw socket)
    lifecycle_min_role: admin               # min role for lifecycle controls
    lifecycle_allowed_groups: []            # additionally require group membership
    health_badge_placement: overview        # overview | overview_and_nav | off
```

### Make the daemon socket reachable from Muximux

How you grant Muximux access to the Docker daemon depends on how you run Muximux. The `endpoint:` field in `config.yaml` is auto-populated to the platform default if you leave it blank, so most operators only need the OS-level access step below.

#### Linux, running Muximux in a container (most common)

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    # ... usual ports / volumes / env ...
    volumes:
      - ./data:/app/data
      # :ro = discovery only (read-only). Use :rw instead to also enable
      # start/stop/restart controls -- see "Container lifecycle controls" below.
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

That's the whole prerequisite. The entrypoint stats the bind-mounted socket at startup, reads its group ownership from the host, and adds the runtime user to a matching group inside the container - so the non-root muximux process can actually read the socket. No `DOCKER_GID` env var, no `group_add`, no hunting for the right number. The default `endpoint: unix:///var/run/docker.sock` already matches the mount.

The trailing `:ro` is the default and keeps Muximux read-only: with `:ro` it can see and list your containers but cannot start, stop, or change any of them. Marking the mount read-only means a compromised Muximux can't pivot to full daemon control. The Docker engine API still exposes enough surface to enumerate every container on the host, so treat this mount as "effective host root for read" and gate the dashboard accordingly with auth.

Container **lifecycle controls** (start / stop / restart, see below) are opt-in through two layers: the socket must be mounted `:rw` **and** `discovery.docker.lifecycle_enabled: true` must be set. Neither one alone is enough - the dashboard still won't start, stop, or restart anything until you turn on both. Even with both on, Muximux can only start, stop, and restart the containers it already tracks - it can't pull or build images, create or delete containers, run commands inside them, or touch networks and volumes (Muximux never calls those parts of the Docker API). Every action, and every blocked attempt, is written to the audit log.

For rootless Docker, docker-socket-proxy sidecars, or custom socket paths, two override env vars are available: `DOCKER_SOCKET` (the path the entrypoint stats) and `DOCKER_GID` (bypass auto-detect and force a specific GID). Almost no one needs these.

#### Linux, running Muximux as a native binary

There's no container in this path, so no entrypoint script. Linux's normal socket permissions apply: the OS user the binary runs as must be a member of the host's `docker` group.

```bash
# Add the user that runs Muximux to the docker group:
sudo usermod -aG docker $(whoami)

# Then re-login (or restart the muximux service) so the new
# group membership takes effect.
```

For systemd unit installs, you can also encode it directly in the unit:

```ini
[Service]
User=muximux
SupplementaryGroups=docker
ExecStart=/usr/local/bin/muximux --data /var/lib/muximux
```

The default `endpoint: unix:///var/run/docker.sock` works as is.

#### macOS, running Muximux as a native binary

Docker Desktop on macOS creates a host-side socket symlink at `/var/run/docker.sock`, so the default `endpoint: unix:///var/run/docker.sock` works out of the box for most setups.

If you've disabled **Docker Desktop → Settings → Advanced → "Allow the default Docker socket to be used"**, the symlink isn't created. Either re-enable that toggle, or point `endpoint:` at the per-user socket Docker Desktop still maintains:

```yaml
discovery:
  docker:
    endpoint: unix:///Users/<your-user>/.docker/run/docker.sock
```

#### Windows, running Muximux as a native binary

Docker on Windows exposes a **named pipe** rather than a unix socket. Muximux 3.1.0+ supports the `npipe://` scheme natively and uses it by default when running on Windows - no extra config needed:

```yaml
discovery:
  docker:
    enabled: true
    # endpoint is auto-filled to npipe:////./pipe/docker_engine on Windows
```

If the Windows account that runs Muximux is a member of the `docker-users` group, that's sufficient.

#### Remote daemon over TCP (any platform)

For a Docker daemon on a different host:

```yaml
discovery:
  docker:
    enabled: true
    endpoint: tcp://docker-host.lan:2376
    tls:
      enabled: true
      ca_cert: /etc/muximux/docker-ca.pem
      client_cert: /etc/muximux/docker-cert.pem
      client_key: /etc/muximux/docker-key.pem
```

The remote daemon must expose its API with TLS (`dockerd --tlsverify`), and the cert/key paths must be readable by the Muximux process.

### Network strategies

| Strategy | URL shape | Requires |
|---|---|---|
| `container_ip` | `http://<docker-network-ip>:<port>` | Muximux runs in a container on the same docker network, **or** `network_filter` is set |
| `container_dns` | `http://<container-name>:<port>` | Same prerequisites as `container_ip`; Docker's internal DNS resolves names within a network |
| `host_port` | `http://<host_ip>:<published-port>` | Container has `-p <host>:<container>` published; `host_ip` is set in config |
| `host_docker_internal` | `http://host.docker.internal:<published-port>` | Muximux runs in a Docker Desktop / WSL container where `host.docker.internal` resolves |

Strategy gating: when Muximux runs natively (not in a container), `container_ip` and `container_dns` need a `network_filter` to substitute for self-identification. The banner above the form tells you whether the chosen strategy is workable in your environment.

### Labels on your containers

Label any container with `muximux.*` keys and the Discover modal picks them up as high-confidence pre-fills. A fully-labelled container goes from `docker compose up` to a fully-configured Muximux entry with a single click in the Discover modal, no post-import editing -- or with no click at all if you turn on [automatic import](#automatic-import). This is the "GitOps your apps" pattern: declare your dashboard intent in `docker-compose.yml` alongside the service it describes.

Full example, showing one of each kind:

```yaml
services:
  sonarr:
    image: linuxserver/sonarr
    labels:
      # ── Tracking ─────────────────────────────────────────────
      - muximux.discovery.id=sonarr-stable
      # ── App fields (menu entry) ──────────────────────────────
      - muximux.app.enabled=true
      - muximux.app.name=Sonarr
      - muximux.app.icon=sonarr
      - muximux.app.group=Media
      - muximux.app.port=8989
      - muximux.app.scheme=https
      - muximux.app.path=/
      - muximux.app.health=/api/v3/health
      - muximux.app.color=#3498db
      - muximux.app.order=10
      - muximux.app.default=false
      - muximux.app.open_mode=iframe
      - muximux.app.proxy=true
      - muximux.app.proxy_skip_tls_verify=true
      - muximux.app.min_role=user
      - muximux.app.allowed_groups=family,admins
      - muximux.app.permissions=clipboard-read,clipboard-write
      - muximux.app.allow_notifications=true
      - muximux.app.shortcut=1
      # ── Gateway routing (subdomain + auth gate) ──────────────
      - muximux.app.gateway.domain=sonarr.example.com
      - muximux.gateway.tls=auto
      - muximux.gateway.streaming=false
      - muximux.gateway.strip_frame_blockers=true
      - muximux.gateway.forwarded_headers=true
      - muximux.gateway.require_auth=true
      - muximux.gateway.min_role=user
      - muximux.gateway.allowed_groups=family,admins
```

#### Common scenarios

The example above sets every label at once. In practice you set only the few an app needs. With automatic import (3.2.0+) these labels are the whole configuration, so here is what each common case looks like on its own. Every block is a complete, copy-pasteable `services:` entry.

**A catalog app, the short way.** If the image is one Muximux recognises, a single tracking label is enough -- name, icon, group, port, and health come from the catalog:

```yaml
services:
  sonarr:
    image: linuxserver/sonarr
    labels:
      - muximux.discovery.id=sonarr
```

**An app Muximux does not know.** For an image outside the catalog, opt in with `enabled` and supply the basics:

```yaml
services:
  myapp:
    image: ghcr.io/me/myapp
    labels:
      - muximux.discovery.id=myapp
      - muximux.app.enabled=true
      - muximux.app.name=My App
      - muximux.app.icon=myapp
      - muximux.app.group=Tools
      - muximux.app.port=8080
```

**An app that refuses to embed.** Route it through the built-in reverse proxy so it loads in an iframe:

```yaml
services:
  radarr:
    image: linuxserver/radarr
    labels:
      - muximux.discovery.id=radarr
      - muximux.app.port=7878
      - muximux.app.proxy=true
```

**Published on its own subdomain.** Add gateway labels and Muximux serves the app at a public HTTPS address (automatic TLS) behind the login:

```yaml
services:
  grafana:
    image: grafana/grafana
    labels:
      - muximux.discovery.id=grafana
      - muximux.app.name=Grafana
      - muximux.app.port=3000
      - muximux.app.gateway.domain=grafana.example.com
      - muximux.gateway.tls=auto
      - muximux.gateway.require_auth=true
```

**A button that fires a request, not a page.** Use `http_action` to turn a tile into a webhook trigger (for example an n8n flow), with a confirmation prompt. The app URL (built from `port` + `path`) is the request target:

```yaml
services:
  n8n:
    image: n8nio/n8n
    labels:
      - muximux.discovery.id=nightly-backup
      - muximux.app.name=Run Backup
      - muximux.app.icon=n8n
      - muximux.app.port=5678
      - muximux.app.path=/webhook/backup
      - muximux.app.open_mode=http_action
      - muximux.app.http_action_method=POST
      - muximux.app.http_action_confirm=true
```

**Restricted to certain people.** Gate an app by role and group:

```yaml
services:
  portainer:
    image: portainer/portainer-ce
    labels:
      - muximux.discovery.id=portainer
      - muximux.app.port=9000
      - muximux.app.proxy=true
      - muximux.app.min_role=admin
      - muximux.app.allowed_groups=ops
```

#### Finding an icon slug

The `muximux.app.icon` value is a [Dashboard Icons](https://dashboardicons.com) slug -- the icon filename without its extension (`sonarr.svg` becomes `sonarr`). Because the label flow has no GUI in front of you, here is where to look one up:

- Browse the gallery at [dashboardicons.com](https://dashboardicons.com) and search for your app.
- Or search visually in Muximux's own app editor (Add or Edit an app, then the icon picker); the name you land on is the slug to paste into the label.
- Or list them straight from your running instance: `GET /api/icons/dashboard` returns every available slug (it is the same data the picker uses).

Omit the label to fall back to the catalog icon. Only Dashboard Icons slugs work through this label; Lucide icons, custom uploads, and URL icons are set in the UI or `config.yaml`.

#### Full label reference

##### Tracking

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.discovery.id` | string | (container name) | Stable tracking key that survives `docker-compose --force-recreate` and swarm reschedules. **Most important label** - without it, Muximux falls back to the container name, which Compose appends a `_1`/`-1` suffix to that changes on recreate. |

##### App fields (the menu entry)

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.app.enabled` | bool | `true` if image matches catalog | Opt-out via `false`; opt-in for containers not in the catalog. |
| `muximux.app.name` | string | catalog name or container name | Display name in the menu. |
| `muximux.app.icon` | string | catalog icon or `""` | Any `dashboard-icons` slug (e.g. `sonarr`, `plex`, `qbittorrent`). |
| `muximux.app.group` | string | catalog group | Group the app lives in. Created if it doesn't exist. |
| `muximux.app.port` | int 1-65535 | catalog port or first exposed | Which container port the app listens on. |
| `muximux.app.scheme` | `http` \| `https` | `http` | Scheme for the constructed URL. |
| `muximux.app.path` | string | `/` | Sub-path appended to the URL (useful for apps behind a path prefix). |
| `muximux.app.health` | string | catalog default | Backend health-check endpoint. |
| `muximux.app.color` | `#rrggbb` | unset | Accent color in the dashboard. |
| `muximux.app.order` | int 0-9999 | unset | Sort order within the group. |
| `muximux.app.default` | bool | `false` | Load this app automatically when the dashboard opens. |
| `muximux.app.open_mode` | `iframe` \| `new_tab` \| `new_window` \| `redirect` \| `http_action` | `iframe` | How clicking the menu entry opens the app. `http_action` fires an HTTP request instead of opening a page. |
| `muximux.app.http_action_method` | `GET` \| `POST` \| `PUT` \| `DELETE` \| `PATCH` | `POST` | With `open_mode=http_action`, the HTTP method to send to the app URL. |
| `muximux.app.http_action_headers` | csv `Key=Value` | unset | Extra request headers, comma-separated (e.g. `Authorization=Bearer xyz`). |
| `muximux.app.http_action_confirm` | bool | `false` | Show a confirmation prompt before firing. |
| `muximux.app.http_action_show_toast` | bool | `true` | Show a result toast after firing. |
| `muximux.app.proxy` | bool | `false` | Route through Muximux's built-in reverse proxy (strips iframe-blocking headers, rewrites paths). Required for many apps that refuse to embed. |
| `muximux.app.proxy_skip_tls_verify` | bool | `true` | When `proxy=true`, skip backend TLS cert verification (useful for self-signed homelab apps). |
| `muximux.app.min_role` | `user` \| `power-user` \| `admin` | unset | Minimum role required to see this app. Admins always bypass. |
| `muximux.app.allowed_groups` | csv | unset | Comma-separated group allow-list; user must belong to at least one. Case-insensitive. Stacks with `min_role`. |
| `muximux.app.permissions` | csv | unset | Iframe feature delegations (`camera`, `microphone`, `geolocation`, `clipboard-read`, `clipboard-write`, `fullscreen`, ...). |
| `muximux.app.allow_notifications` | bool | `false` | Enable the cross-iframe Notifications API bridge for this app. |
| `muximux.app.shortcut` | int 1-9 | unset | Keyboard shortcut slot. |
| `muximux.app.gateway.domain` | string | unset | When set, the import modal also offers a gateway-site entry for this subdomain. Pairs with the `muximux.gateway.*` labels below. |

##### Gateway-site fields (only consulted when `muximux.app.gateway.domain` is set)

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.gateway.tls` | `auto` \| `none` \| `custom` | `auto` | TLS handling for the subdomain. `auto` runs Let's Encrypt; `none` is plain HTTP; `custom` expects `tls_cert` and `tls_key` paths (set those manually in the import modal). |
| `muximux.gateway.streaming` | bool | `false` | Disable Caddy response buffering for live-streaming backends. |
| `muximux.gateway.strip_frame_blockers` | bool | `true` | Drop `X-Frame-Options` / `Content-Security-Policy: frame-ancestors` on responses so the site can be iframed elsewhere. |
| `muximux.gateway.forwarded_headers` | bool | `true` | Forward `X-Forwarded-Proto` / `X-Forwarded-Host` / `X-Real-IP`. Turn off only when your backend has its own handling. |
| `muximux.gateway.require_auth` | bool | `false` | Gate the subdomain behind Muximux's login. Visitors land on `/login` first; admins bypass per-site role / group rules. Requires `server.session_cookie_domain` to be set. |
| `muximux.gateway.min_role` | `user` \| `power-user` \| `admin` | unset | When `require_auth=true`, minimum role to access the site. |
| `muximux.gateway.allowed_groups` | csv | unset | When `require_auth=true`, allow-list groups (comma-separated, case-insensitive). |

Unknown `muximux.*` labels are surfaced in the Discover modal's per-row notes so a typo is visible, not silently ignored.

---

## Automatic Import

Everything above is the **manual** path: labels become high-confidence pre-fills, and you click **Import** to commit them. Automatic import removes that review step. When enabled, Muximux imports every `muximux.*`-labeled container on the daemon by itself -- no modal, no click -- and keeps the imported apps in step with the labels over time.

> **Security -- read before enabling.** Auto-import is **off by default** and only the host operator (who controls `config.yaml` or the environment) can turn it on. Once on, **any container on the shared Docker socket can write itself into your config with no review step** -- including a `muximux.app.gateway.domain` label that publishes a **public HTTPS subdomain with an auto-issued ACME certificate**. Treat enabling this as trusting every label on every container the daemon can see. If you don't control all of those containers, leave it `off` and import by hand.

### Modes

Set the mode with the `discovery.docker.auto_import` config key, or override it with the `MUXIMUX_DISCOVERY_AUTO_IMPORT` environment variable (the env var wins). Four values:

| Mode | Adds new labeled containers | Re-syncs app fields when labels change | Removes the app when the container disappears |
|---|---|---|---|
| `off` (default) | no -- labels stay suggestions you import by hand | no | no |
| `add` | yes, once | no -- imported, then left alone forever | no |
| `update` | yes | yes | no |
| `sync` | yes | yes | yes -- full GitOps mirror |

- **`off`** -- today's behavior. Labeled containers are suggestions only; nothing is written until you click **Import**.
- **`add`** -- a newly labeled container is imported one time, then never touched again. Good for bootstrapping.
- **`update`** -- like `add`, plus Muximux re-syncs the app's fields from the labels whenever they change. Never removes anything.
- **`sync`** -- like `update`, plus when a tracked container disappears from the daemon the auto-imported app (and its gateway site) is removed. The config becomes an exact mirror of the labeled containers.

```yaml
discovery:
  docker:
    enabled: true
    auto_import: sync   # off (default) | add | update | sync
```

```bash
# Environment override (takes precedence over config.yaml):
MUXIMUX_DISCOVERY_AUTO_IMPORT=sync
```

### Opting a container out

A container with `muximux.app.enabled=false` is excluded from auto-import (and from the Discover modal). Use it to keep a labeled container off the dashboard without stripping its labels.

### Edit-wins (URL edits detach)

Auto-import never silently clobbers a URL you took manual control of. **Changing an auto-imported app's URL** -- in Settings, through the API, or in `config.yaml` directly -- or **removing the container's `muximux.*` labels** detaches that app from auto-management. From then on it is a normal manual entry: `update`/`sync` will not re-sync it from labels, and `sync` will not remove it. This is the same edit-lock / auto-detach mechanism described below for tracked URLs.

Other managed-field edits (name, icon, group, and similar) do **not** detach. Under `update`/`sync` they are re-synced from the labels on the next tick, so the labels remain the source of truth; under `add` they stick, because `add` never re-syncs an already-imported app.

### Known limitation -- gateway-only labels and `update`/`sync`

In `update` and `sync`, re-sync compares the **app** fields built from labels against the stored app. Gateway labels that change the app's URL -- `muximux.app.gateway.domain` and `muximux.gateway.tls` -- do change an app field, so edits to them **do** propagate on the next tick.

Gateway-**only** labels that do not map to any app field -- `muximux.gateway.require_auth`, `muximux.gateway.min_role`, `muximux.gateway.streaming`, `muximux.gateway.strip_frame_blockers`, and `muximux.gateway.forwarded_headers` -- are **not** picked up by re-sync. Changing one of these labels on a running, already-imported container has no effect until either an app field also changes, or you remove and re-add the container (which re-imports it from scratch). So if you rely on, say, toggling `muximux.gateway.require_auth` via a label, know that auto-import will not apply that toggle on its own.

---

## Routing Modes Explained

When you check "Add to menu" in the import modal, a radio selector decides how the menu link works:

### Direct

```
Browser → http://192.168.1.4:8989  (your dashboard machine reaches the container directly)
```

- **App.url** = the discovered container URL
- **App.proxy** = false
- The poller refreshes App.url every tick when the container's IP changes

Use when: your dashboard machine has direct network reachability to the container (same host, same subnet, VPN).

### Proxy

```
Browser → /proxy/sonarr → Muximux Go server → http://192.168.1.4:8989
```

- **App.url** = the container URL (kept as the upstream)
- **App.proxy** = true
- Muximux's built-in path-prefix reverse proxy strips iframe-blockers, rewrites HTML/CSS/JS paths, isolates `window.parent`, etc. - makes apps work in iframes that refuse them.
- Same tracking semantics as Direct (poller refreshes the upstream URL)

Use when: the app misbehaves in iframes, the dashboard machine cannot reach the container directly, or you want auth/CSP layering through Muximux.

### Gateway domain

```
Browser → https://sonarr.example.com → Muximux (embedded Caddy, :443, auto-HTTPS) → http://192.168.1.4:8989
```

Point the subdomain's DNS at the Muximux host: Muximux **is** the reverse proxy here. Its embedded Caddy binds 80/443, provisions the certificate via Let's Encrypt, and proxies the request to the container. You do not put nginx or another proxy in front of it for this to work.

- **App.url** = `https://<gateway-domain>` (static)
- **App.proxy** = false
- **App is NOT auto-managed** - the gateway site becomes the docker-managed entry instead
- The poller refreshes the gateway site's `backend_url` (not the App's URL, which doesn't depend on the container)

Use when: you want a public subdomain that survives container moves, and a clickable dashboard link that uses that domain.

> If you already run an edge proxy (Cloudflare Tunnel, an upstream Traefik/nginx) and would rather it terminate TLS, set `server.gateway_listen: ":8443"` and have that proxy forward to Muximux on that port instead. In that case the flow is `Browser → upstream proxy → muximux:8443 → http://192.168.1.4:8989`.

---

## Edit Lock + Auto-Detach

When an App or GatewaySite is tracked, the URL field is **read-only** in the editor with an amber lock badge. Muximux protects your edits across all three ways you might change the URL:

1. **In Settings**, the URL field is locked. To change it, click **Detach** first.
2. **Through the API**, if a SaveConfig request changes the URL of a tracked entry, Muximux treats that as a deliberate takeover, drops the tracking, and writes an audit log entry.
3. **In `config.yaml` by hand**, the same thing happens at next boot: Muximux notices your URL differs from the one the poller last wrote, drops the tracking, and notes it in the log. Your edit survives the next refresh tick.

The sanctioned forget path is the **Detach** button in Settings → Discovery (or `DELETE /api/discovery/docker/track/<key>` from a script).

### docker_managed_url (internal)

You'll see a `docker_managed_url` field appear next to `docker_key` for tracked entries. Muximux writes it from the import flow and updates it on every poller tick. You don't need to touch it. If you do hand-author a tracked entry from scratch, just set `docker_managed_url` to the same value as `url` (or `backend_url` for a gateway site) so the file-edit detach mechanism has a baseline to compare against.

---

## Divergence Banner

Caddy's reload is transactional only at the parse step. Post-parse failures (listener collision, async cert provisioning, module panic) can leave the running config in an indeterminate state. The refresh poller handles this with rollback:

- Candidate reload fails, rollback reload succeeds → audit log warning, refresh tick skipped
- Both fail → **divergence counter** increments, sticky red banner appears in Settings → Discovery
- First clean tick after a divergence → banner transitions to amber "recovered"

The banner gives you a one-glance signal that the running Caddy may not match disk. Recovery happens automatically on the next successful tick; the banner stays amber until you acknowledge it (currently by waiting - a future iteration may add a "clear" button).

---

## Running Behind Another Reverse Proxy

Caddy binds 80/443 with auto-HTTPS by default, which requires root or `CAP_NET_BIND_SERVICE`. For "behind another proxy" topologies (Traefik, Cloudflare Tunnel, nginx), set `server.gateway_listen` to a non-privileged port:

```yaml
server:
  listen: ":8080"
  gateway_listen: ":8443"   # gateway sites served as plain HTTP here
```

See [TLS and Gateway → Running Behind Another Reverse Proxy](tls-and-gateway.md#running-behind-another-reverse-proxy-gateway_listen) for the full topology guide.

---

## Troubleshooting

| Symptom | Diagnosis |
|---|---|
| Banner: "Connected to Docker but strategy `container_ip` cannot identify Muximux's container" | Muximux runs natively (not in a container). Set `network_filter` to scope the scan to a known docker network, or switch to `host_port`. |
| Banner: "Daemon unreachable: dial unix … no such file or directory" | The `endpoint` path is wrong, or the socket isn't bind-mounted into Muximux's container. |
| Banner: "Daemon unreachable: connect: permission denied" | The socket is mounted but the entrypoint's auto-detection didn't fire (e.g. unusual mount path, docker-socket-proxy sidecar). Override with `DOCKER_GID` set to the docker group GID the socket is owned by, or `DOCKER_SOCKET` to point the detection at a non-default path. See [Make the daemon socket reachable](#make-the-daemon-socket-reachable-from-muximux). |
| Discover modal shows containers but no auto-fill | The image isn't in Muximux's catalog. Add `muximux.app.*` labels to the container, or fill the fields manually before importing. |
| Imported app's URL doesn't update when container restarts | Check `refresh_interval` isn't set to 1h. Check the audit log for `Docker app URL refreshed`. Check the container hasn't been renamed (breaks `name:` tracking keys). |
| Gateway site doesn't serve after import | If you set `server.gateway_listen`, your upstream proxy needs to forward the host header to that port. Try `curl -H 'Host: site.example.com' http://muximux-host:8443/` to bypass the upstream. |
| Divergence banner is red and won't clear | Inspect the most recent `Docker refresh divergence` audit log line for the candidate + rollback errors. Most often a Caddyfile parse-OK but listener-collide situation. Restart Muximux to recover. |

---

## API Reference

All endpoints are admin-only.

| Method | Path | Body | Description |
|---|---|---|---|
| GET | `/api/discovery/docker/status` | - | Capability + cache status |
| PUT | `/api/discovery/docker/config` | `DiscoveryDockerConfig` | Persist new discovery settings + rebuild service |
| POST | `/api/discovery/docker/test` | `DiscoveryDockerConfig` | Probe a candidate config without saving |
| GET | `/api/discovery/docker/scan` | - | Enumerate running containers as `Suggestion` list |
| POST | `/api/discovery/docker/import` | `{items: ImportItem[]}` | Atomic batch import of selected containers |
| GET | `/api/discovery/docker/tracked` | - | Current tracked apps + sites with last-seen timestamps |
| DELETE | `/api/discovery/docker/track/{key}` | - | Detach tracking for everything matching `key` on the current endpoint |
| POST | `/api/discovery/docker/relink/probe` | `{key}` | "Does this key still resolve on the current daemon?" |
| POST | `/api/discovery/docker/relink/confirm` | `{old_key, new_key, strategy?}` | Move tracking from old key to new key |

## Container lifecycle controls

If you want the dashboard to start / stop / restart tracked containers from the overview page:

1. Edit `docker-compose.yml` and switch the Docker socket mount from `:ro` to `:rw` -- change the line to `- /var/run/docker.sock:/var/run/docker.sock:rw`. The `:ro` mount stays as the documented default; you opt in by changing this one character.
2. Set `discovery.docker.lifecycle_enabled: true` in your `config.yaml`, or toggle "Enable container lifecycle controls" under Settings -> Discovery (the checkbox is disabled until the socket is writable).
3. Optional: narrow who can use the controls. `discovery.docker.lifecycle_min_role` defaults to `admin`; set it to `power-user` or `user` to widen access. Set `discovery.docker.lifecycle_allowed_groups` to additionally require membership in specific groups.
4. Optional: set `discovery.docker.health_badge_placement` to `overview_and_nav` to show container state badges in the navigation sidebar as well as the overview (default is `overview`; `off` hides them).

Once enabled, Docker-tracked apps on the overview show the Docker logo plus a small status dot when the container needs a glance: red for stopped, amber for unhealthy or paused, blue for restarting (a healthy, running container shows no dot - quiet by default). Hovering the card reveals the action buttons in a footer below it - Start when stopped, Stop and Restart when running; on touch devices the buttons stay visible. The footer sits outside the card's open-app area, so a tap to open the app can't trigger a container action by accident. Stop and Restart prompt for confirmation; Start fires immediately.

Every action - success, failure, and denied attempt (role floor not met, group mismatch, socket read-only, lifecycle disabled) - is audit-logged with the caller's username, app name, container id, and outcome (`source=audit`).

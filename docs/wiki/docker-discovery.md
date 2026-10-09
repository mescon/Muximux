# Docker Discovery

Connect Muximux to a Docker daemon and it can enumerate running containers, propose them as apps, and keep their URLs current as IPs shift across restarts. **Off by default** because it requires read access to the Docker socket.

---

## What It Does

- **Scan**: lists running containers and proposes a name, icon, URL, port, and group for each, with confidence ratings (high / medium / low).
- **Import**: one-click adds the chosen containers as apps in the menu, gateway subdomains, or both. Each row picks a routing mode: Direct URL, internal Proxy, or Gateway domain.
- **Refresh**: a background poller (default 60s) re-resolves each tracked container against the daemon and rewrites the saved URL if the container's IP changes. Caddy reloads once per tick when gateway-site URLs change.
- **Auto-import** (optional): with `discovery.docker.auto_import` set to `add`/`update`/`sync`, Muximux imports every container that carries at least one `muximux.*` label (a `muximux.discovery.id` alone is enough) and does not carry `muximux.app.enabled=false`, with no modal and no click. Unlabelled containers are never auto-imported. `add` imports each new labeled container once; `update` also re-syncs an imported app when its labels change; `sync` additionally removes an imported app when its container disappears or opts out. Off by default. See [Automatic Import](#automatic-import).
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
   - Stable tracking key (operator label > Swarm service > Compose service > container name > container ID)
   - Resolved URL preview
   - Stability warning when the key is likely to break on a `docker-compose --force-recreate` or swarm task reschedule
   - A **Not importable** chip when auto-import would skip the container, with the reason (see [Why a container is skipped](#why-a-container-is-skipped))

   The catalog matcher is lenient about operator prefix conventions: a container named `homelab-sonarr`, `homelab_radarr`, or `prod.plex` still picks up the matching catalog entry just like a bare `sonarr` / `radarr` / `plex` would. Tokens are split on `-`, `_`, and `.`; comparison is exact-token (so `transmissionic` doesn't masquerade as `transmission`). Multi-word app names like `home-assistant` work via an adjacent-pair fallback.

3. **Pick routing per row** and click **Import**:
   - **Direct** - menu links to `http://<container-ip>:<port>` directly
   - **Proxy** - menu links via Muximux's `/proxy/<slug>` path-prefix reverse proxy
   - **Gateway domain** - menu links to `https://<your-subdomain>`; requires also creating the gateway site in the same row

   If a row's group does not exist yet, the import creates it in the same save (see [Groups are created automatically](#groups-are-created-automatically)).

4. **Settings → Discovery → Currently tracked**: every imported app or gateway site appears here. Per-row **Detach** stops auto-management; **Re-link** appears when the saved `DockerEndpoint` no longer matches the configured endpoint (typical after a daemon migration). A row shows "Container missing since <time>" while its container cannot be found on the daemon. Invalid Docker-owned entries that were not loaded are listed below the table; see [Quarantined entries](#quarantined-entries).

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
    require_explicit_enable: false          # only containers with muximux.app.enabled=true (3.6.0)
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

#### Running behind your own reverse proxy

Your apps already have public names behind Traefik, Authentik, NPM or another proxy, and Muximux should open those names rather than container IPs:

```yaml
services:
  sonarr:
    image: lscr.io/linuxserver/sonarr
    labels:
      - muximux.app.name=Sonarr
      - muximux.app.group=Media
      - muximux.app.icon=sonarr
      - muximux.app.port=8989
      - muximux.app.url=https://sonarr.example.com
      - muximux.app.health=/ping
```

Sonarr opens at `https://sonarr.example.com`. The health check calls `http://<container IP>:8989/ping`, and the poller keeps that address current when the container's IP changes (logging "Docker health address refreshed"); the app URL is never rewritten. A relative health label is resolved against the container; an absolute URL label is kept as written and never rewritten. Without a health label the check goes to the container's URL, including `muximux.app.path`. Enable monitoring with `muximux.app.health_check=true` or in the app's settings.

The poller owns the health address of such an app: it rewrites `health_url` on every refresh, so an edit made in Settings is overwritten. To pin it, set `muximux.app.health` to an absolute URL, which is kept as written.

Removing `muximux.app.url` later returns the app to its container URL on the next refresh tick. Editing the URL by hand in Settings detaches the app, as with any tracked app. Known limitation: after removing the `muximux.app.url` label, an app imported manually keeps the container address it had as its health address; set it in the app's settings if needed. Auto-imported apps get it reset by auto-import.

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
| `muximux.discovery.id` | string | (container name) | Stable tracking key that survives `docker-compose --force-recreate` and swarm reschedules. **Most important label** for plain containers - without it, Muximux falls back to the Swarm service, the Compose service, then the container name (see [Tracking keys](#tracking-keys)). Swarm tasks and Compose services stay stable without it. |

##### App fields (the menu entry)

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.app.enabled` | bool | `true` | Opt-out via `false`. With `require_explicit_enable` (or `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE`) only `true` opts in. See [Explicit opt-in](#explicit-opt-in). |
| `muximux.app.name` | string | catalog name or container name | Display name in the menu. Surrounding spaces are ignored. Re-synced while the app is tracked; see [Label re-sync for tracked apps](#label-re-sync-for-tracked-apps). |
| `muximux.app.icon` | string | catalog icon or `""` | Any `dashboard-icons` slug (e.g. `sonarr`, `plex`, `qbittorrent`). Surrounding spaces are ignored and the slug is lowercased. |
| `muximux.app.group` | string | catalog group | Group the app lives in. Created if it doesn't exist (from 3.6.0). |
| `muximux.app.port` | int 1-65535 | catalog port or first exposed | Which container port the app listens on. |
| `muximux.app.url` | absolute `http(s)` URL | unset | Open the app at this URL instead of the container address, e.g. its public name behind your own reverse proxy. Health checks still go to the container. Ignored (with a scan note) when `muximux.app.gateway.domain` is set, and invalid values are ignored with a note. See [Running behind your own reverse proxy](#running-behind-your-own-reverse-proxy). |
| `muximux.app.scheme` | `http` \| `https` | `http` | Scheme for the constructed URL. |
| `muximux.app.path` | string | `/` | Sub-path appended to the container URL (e.g. `/admin`). Not applied to `muximux.app.url` or to a gateway site's backend. |
| `muximux.app.health` | string | catalog default | Health-check address: a full URL, or a path such as `/api/v3/health` resolved against the container. |
| `muximux.app.health_check` | bool | unset | Enables (`true`) or disables (`false`) health monitoring for the app; the endpoint still comes from `muximux.app.health`. Unset keeps the setting in the app's form. Docker-managed: re-synced on update/sync and shown locked in Settings. |
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
| `muximux.app.gateway.domain` | string | unset | Required for auto-import to create a gateway site; the derived `<name>.<dashboard domain>` default only pre-fills the import modal. When set, the import modal also offers a gateway-site entry for this subdomain. Pairs with the `muximux.gateway.*` labels below. |

##### Group fields (the group the app is in)

These describe the group the container's app is in. They apply only to a group Docker discovery created; see [Group labels](#group-labels).

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.group.icon` | string | folder icon | Any `dashboard-icons` slug, the same as `muximux.app.icon`. Surrounding spaces are ignored and the slug is lowercased. |
| `muximux.group.color` | `#rrggbb` | unset | Group colour, a hex colour (`#rgb`, `#rgba`, `#rrggbb` or `#rrggbbaa`). Other values are ignored with a warning and a scan note. |
| `muximux.group.order` | int 0-9999 | after existing groups | Sort order of the group. Other values are ignored with a warning and a scan note. |

##### Gateway-site fields (only consulted when `muximux.app.gateway.domain` is set)

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.gateway.tls` | `auto` \| `none` \| `custom` | `auto` | TLS handling for the subdomain. `auto` runs Let's Encrypt; `none` is plain HTTP; `custom` expects `tls_cert` and `tls_key` paths (set those manually in the import modal). |
| `muximux.gateway.streaming` | bool | `false` | Disable Caddy response buffering for live-streaming backends. |
| `muximux.gateway.strip_frame_blockers` | bool | `true` | Drop `X-Frame-Options` / `Content-Security-Policy: frame-ancestors` on responses so the site can be iframed elsewhere. |
| `muximux.gateway.forwarded_headers` | bool | `true` | Forward `X-Forwarded-Proto` / `X-Forwarded-Host` / `X-Real-IP`. Turn off only when your backend has its own handling. |
| `muximux.gateway.skip_tls_verify` | bool | `false` | Skip verification of the backend's TLS certificate. Only takes effect when the backend URL is `https` -- use it for self-signed / untrusted backends like Proxmox on `:8006`. Leaves the public-facing certificate unaffected. |
| `muximux.gateway.require_auth` | bool | `false` | Gate the subdomain behind Muximux's login. Visitors land on `/login` first; admins bypass per-site role / group rules. Requires `server.session_cookie_domain` to be set. |
| `muximux.gateway.min_role` | `user` \| `power-user` \| `admin` | unset | When `require_auth=true`, minimum role to access the site. |
| `muximux.gateway.allowed_groups` | csv | unset | When `require_auth=true`, allow-list groups (comma-separated, case-insensitive). |

Unknown `muximux.*` labels are surfaced in the Discover modal's per-row notes so a typo is visible, not silently ignored.

---

## Automatic Import

Everything above is the **manual** path: labels become high-confidence pre-fills, and you click **Import** to commit them. Automatic import removes that review step. When enabled, Muximux imports every container that carries at least one `muximux.*` label (and is not opted out) by itself -- no modal, no click -- and keeps the imported apps in step with the labels over time.

> **Security -- read before enabling.** Auto-import is **off by default** and only the host operator (who controls `config.yaml` or the environment) can turn it on. Once on, **any container on the shared Docker socket can write itself into your config with no review step** -- including a `muximux.app.gateway.domain` label that publishes a **public HTTPS subdomain with an auto-issued ACME certificate**. Treat enabling this as trusting every label on every container the daemon can see. If you don't control all of those containers, leave it `off` and import by hand.

### Modes

Set the mode with the `discovery.docker.auto_import` config key, or override it with the `MUXIMUX_DISCOVERY_AUTO_IMPORT` environment variable (the env var wins). Four values:

| Mode | Adds new labelled containers (at least one `muximux.*` label, not opted out) | Re-syncs app fields when labels change | Removes the app when the container disappears or opts out |
|---|---|---|---|
| `off` (default) | no -- labels stay suggestions you import by hand | no | no |
| `add` | yes, once | no -- imported, then left alone forever | no |
| `update` | yes | yes | no |
| `sync` | yes | yes | yes -- full GitOps mirror |

- **`off`** -- today's behavior. Labeled containers are suggestions only; nothing is written until you click **Import**.
- **`add`** -- a newly labeled container is imported one time, then never touched again. Good for bootstrapping.
- **`update`** -- like `add`, plus Muximux re-syncs the app's fields from the labels whenever they change. Never removes anything.
- **`sync`** -- like `update`, plus when a tracked container disappears from the daemon, or opts out with `muximux.app.enabled=false`, the auto-imported app (and its gateway site) is removed. Removal waits until the container has been absent for three consecutive successful scans, so a restart does not remove and re-create the app. Only auto-imported apps are removed; apps you imported by hand or detached are never touched. The config becomes an exact mirror of the labelled containers.

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

The override applies in memory only and is never written to `config.yaml`. Settings -> Discovery shows the field as locked ("From MUXIMUX_DISCOVERY_AUTO_IMPORT"). The Discovery tab shows the stored values (host IP, network filter, refresh interval, auto-import mode, badge placement) and its save merges onto the stored config, so fields it does not show are kept. On Windows, `npipe://` endpoints can be entered in Settings.

### Opting a container out

A container with `muximux.app.enabled=false` is excluded from auto-import and hidden from the Discover modal. Use it to keep a labelled container off the dashboard without stripping its labels. Under `sync` an app already auto-imported from that container, and its gateway site, is removed after the grace period; under `update` it is left alone.

### Explicit opt-in

By default every labelled container that is not opted out is imported. To require a positive label per container, set `discovery.docker.require_explicit_enable: true` (or `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE=true`, which wins and applies in memory only; Settings shows the field as locked). Then only containers with `muximux.app.enabled=true` are imported; labelled containers without it are skipped as `not_enabled`. Invalid values of the environment variable are ignored with a warning. (#496)

### Why a container is skipped

A container with no usable URL is never imported. The Discover modal shows a **Not importable** chip with the reason, and `GET /api/discovery/docker/scan` returns it as `auto_import_skip`:

| Code | Meaning |
|---|---|
| `unlabeled` | No `muximux.*` label on the container. |
| `disabled` | `muximux.app.enabled=false`. |
| `not_enabled` | Explicit opt-in is on and `muximux.app.enabled=true` is missing. |
| `no_port` | No port could be determined; add `muximux.app.port`. |
| `no_url` | No URL could be built with the current network strategy. |
| `invalid` | The labels produce an invalid entry; the detail says why. |

### Containers that lose their labels

When a container loses all its `muximux.*` labels, its auto-imported app is detached from auto-import: it is kept, its URL is still refreshed, and `sync` never removes it. It is still tracked, so the [label re-sync](#label-re-sync-for-tracked-apps) applies to it if labels come back. The same happens on upgrade to an auto-imported app whose container has no labels, and when explicit opt-in is turned on for containers without `enabled=true`.

### Edit-wins (URL edits detach)

Auto-import never silently clobbers a URL you took manual control of. **Changing an auto-imported app's URL** -- in Settings, through the API, or in `config.yaml` directly -- detaches that app from auto-management. Removing the container's `muximux.*` labels also detaches it from auto-import (see above). From then on it is a normal manual entry: `update`/`sync` will not re-sync it from labels, and `sync` will not remove it. This is the same edit-lock / auto-detach mechanism described below for tracked URLs.

Other managed-field edits (name, icon, group, and similar) do **not** detach. The same applies to the health check: when `muximux.app.health_check` is set, Muximux records a server-owned `docker_managed_health_check` marker on the app, re-syncs the value under `update`/`sync`, and the app form shows the toggle locked. Remove the label to unlock it. Under `update`/`sync` they are re-synced from the labels on the next tick, so the labels remain the source of truth; under `add` they stick, because `add` never re-syncs an already-imported app.

### Label re-sync for tracked apps

**While an app is tracked, labels that are set win; detach to take control.**

This holds for every tracked app, including apps you imported by hand through the Discover dialog, and it works with `auto_import: off`. On each refresh tick, for every tracked app whose container is present, Muximux applies these labels when they are set on the container:

| Label | Applied as |
|---|---|
| `muximux.app.name` | The app name. A gateway site linked to the app follows the rename. |
| `muximux.app.icon` | A dashboard icon with that slug (as on import). Icon colour, background, variant and invert are kept. |
| `muximux.app.group` | The group, matched to an existing group by name, ignoring case and spacing (`infra` finds `Infra`). Created if no group matches. |
| `muximux.app.order` | The order within the group. |

The URL and health address keep following the container as before.

- A label that is **not set** never changes anything: your own name, icon, group or order stays.
- If you change one of these fields in Settings while the label is set, the next tick sets it back. To take control, remove the label or **Detach** the app (Settings -> Discovery -> Currently tracked). An app you detached (Detach in Settings), or one you added by hand, is never touched.
- The re-sync only updates apps that are already tracked. It never imports a container or removes an app, whatever the `auto_import` mode.
- Auto-imported apps follow their `auto_import` mode instead (re-synced under `update`/`sync`, left alone under `add`), so no field is written twice.
- A label is held back, and a warning is logged once, when it cannot be applied: the name is already used by another app (names are compared like proxy paths, so `TV` and `tv` collide), or the name is over 100 characters. The app keeps its current value until the conflict is resolved.
- If saving the config fails, the whole tick is rolled back and retried on the next tick.

### Groups are created automatically

When an app lands in a group that does not exist, Muximux creates the group in the same save. This covers a manual import from the Discover dialog, auto-import (new apps and re-synced ones) and the label re-sync above, and applies to a group from a `muximux.app.group` label as well as a catalog group.

- An existing group is matched by name first, then ignoring case and spacing, so `media`, `Media ` and `media-server` / `Media Server` never create a near-duplicate. The app is stored with the existing group's exact name.
- A new group gets the same defaults as one added in Settings: a folder icon, no colour, expanded. It is placed after your existing groups, in the order the apps were processed. Customise it in Settings like any other group.
- While a tracked app has `muximux.app.group` set, a group you delete is created again on the next tick. Remove the label or detach the app to stop that.
- If the save fails, the new groups are rolled back together with the apps.
- An app whose group is missing anyway (for example a group removed from `config.yaml` by hand) is never hidden: the navigation and **Settings -> Apps** list it under **Ungrouped** until the group exists again.

### Group labels

`muximux.group.icon`, `muximux.group.color` and `muximux.group.order` set the look and position of the group a container's app is in. Put them on any container whose app is in the group; they need no `muximux.app.group` on the same container, but without a group there is nothing to apply them to (the Discover dialog notes that).

```yaml
services:
  sonarr:
    labels:
      - muximux.app.group=Media
      - muximux.group.icon=plex
      - muximux.group.color=#e5a00d
      - muximux.group.order=1
```

- **Only groups Docker discovery created.** A group made by an import, auto-import or the label re-sync is marked as Docker-managed (`docker_managed: true` in `config.yaml`). The labels are applied when the group is created and re-synced on every tick while it stays managed. A group you created yourself is never touched.
- **Edit in Settings to take control.** Changing the icon or colour of a managed group in Settings clears the marker. Changing its order (including dragging groups into a new order) clears it only when the order comes from a `muximux.group.order` label; a managed group whose order no label sets can be reordered freely and stays managed. Once the marker is cleared the group is yours and later label changes are ignored. Renaming or expanding/collapsing the group does not count. Set `docker_managed: true` in `config.yaml` to hand a group back to the labels.
- **A label that is not set never changes anything:** the group keeps its own value for that field.
- **Conflicts.** When several containers in one group set different values, the container with the lowest tracking key wins, per field. The others get a scan note in the Discover dialog and a warning in the log (once until the conflict changes).
- **Invalid values** (a colour that is not a hex colour, an order outside 0-9999) are ignored, with one warning per change and a scan note.
- The labels are read from the containers of tracked apps and of tracked gateway sites (through the site's linked app), whatever the `auto_import` mode. A group created by a manual import gets its label values on the next refresh tick.
- Applying them is part of the tick's single save: if the save fails, the group changes are rolled back together with everything else.

### Gateway labels and `update`/`sync`

In `update` and `sync`, re-sync compares both the **app** fields and the **gateway site** built from labels against what is stored. Gateway labels that change the app's URL -- `muximux.app.gateway.domain` and `muximux.gateway.tls` -- and gateway-**only** labels that do not map to any app field -- `muximux.gateway.require_auth`, `muximux.gateway.min_role`, `muximux.gateway.allowed_groups`, `muximux.gateway.streaming`, `muximux.gateway.strip_frame_blockers`, `muximux.gateway.forwarded_headers`, and `muximux.gateway.skip_tls_verify` -- all propagate on the next tick. Toggling `muximux.gateway.require_auth` on a running container, for example, is re-synced without any app-field change.

Only the explicit `muximux.app.gateway.domain` label makes auto-import create a gateway site. With `server.tls.domain` set, the derived `<name>.<dashboard domain>` default is only a pre-fill in the import dialog. An update changes only label-managed fields: health check, auth bypass, access, scale, pinned, proxy headers and other per-app settings are kept.

**Upgrade note:** earlier versions created a gateway site `<name>.<your domain>` for every labelled container when `server.tls.domain` was set. If you relied on those derived subdomains, add `muximux.app.gateway.domain=<name>.<your domain>` to those containers before upgrading; otherwise the first refresh removes the sites and points the apps at their container URLs.

Removing the `muximux.app.gateway.domain` label from an already-imported container reverts its app to the direct container URL and drops the now-orphaned gateway site on the next tick.

---

## Docker Swarm and Compose

Swarm services and Compose projects are tracked by name, so redeploys and `--force-recreate` do not create duplicates.

### Tracking keys

The key is the first of these that applies:

| Order | Source | Key |
|---|---|---|
| 1 | `muximux.discovery.id` label | `label:<value>` |
| 2 | Swarm service | `swarm:<service>` |
| 3 | Compose project and service | `compose:<project>:<service>` |
| 4 | Container name | `name:<name>` (shown with a stability warning) |
| 5 | Container ID | `id:<id>` (last resort) |

Swarm tasks and Compose services therefore no longer need `muximux.discovery.id` for stability. Existing `name:` keys are migrated to the stable key automatically on the first refresh (a `Docker tracking key migrated` audit line is logged). Entries tracked on another endpoint are migrated after **Re-link**. Quarantined entries keep their old key; `sync` removes them once the task is gone from the endpoint it polls, or you can remove them in Settings. Duplicates left by earlier redeploys are cleaned by `sync`.

There is one app per key: Swarm replicas and the containers of a scaled Compose service (`--scale`) are collapsed into a single app. With `container_ip` and several replicas the address may alternate between them.

### Docker Swarm

- Labels under the service's `labels:` reach the task. `deploy.labels` are service labels and are read too; container labels win when both set the same key.
- Published ports come from the service, so the `host_port` and `host_docker_internal` strategies work with ingress-published ports.
- `container_dns` uses the service name.
- The endpoint must be a manager node, because only managers answer `/services`. On a worker the scan shows a note and the app needs `muximux.app.port`.
- Behind a socket proxy such as tecnativa/docker-socket-proxy, allow `/services` with `SERVICES=1`. Without it Muximux logs one warning ("Swarm services unavailable; using task data only") and uses task data only, so service labels and published ports are missing.

---

## Quarantined entries

An invalid Docker-owned entry in `config.yaml` (an auto-imported app or gateway site that fails validation, for example one written without a URL by an older version) no longer stops Muximux from starting and no longer rejects a save from Settings. Instead it is:

- kept in `config.yaml`, unchanged, until it is fixed, removed, or superseded (see below),
- not loaded, so it does not appear on the dashboard,
- logged once at startup as "Invalid Docker auto-imported entry quarantined", and
- listed in **Settings -> Discovery -> Tracked** with its reason.

Fix the container's labels and the next refresh replaces the entry, or remove it from the list (**Remove** or **Remove all**). Removing deletes the entry from `config.yaml`; a live app or gateway site that shares its tracking key stays tracked. A quarantined entry is also superseded, and dropped from `config.yaml` at the next save (logged as "Quarantined entry superseded by a live entry"), when a live app takes its name or slug or a live gateway site takes its domain; a quarantined site goes with its superseded app. Under `sync`, a quarantined app whose container is gone from the polled endpoint is removed after three consecutive scans, like an auto-imported app. A manual gateway site that was linked to a quarantined app loses its app link. Only Docker-owned entries are quarantined; other invalid config still fails validation as before.

Other housekeeping that applies to discovery: image names whose last segment is generic (`server`, `app`, `web`, `api` and similar) no longer match a catalog entry, apps whose names differ only by case or spacing are deduplicated, and a tracked container that cannot be found is logged once when it goes missing instead of on every refresh.

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

1. **In Settings**, the URL field is locked. To change it, click **Detach** under Settings → Discovery → Currently tracked. The app's URL unlocks right away in the same Settings session; saving afterwards keeps it detached.
2. **Through the API**, if a SaveConfig request changes the URL of a tracked entry, Muximux treats that as a deliberate takeover, drops the tracking, and writes an audit log entry.
3. **In `config.yaml` by hand**, the same thing happens at next boot: Muximux notices your URL differs from the one the poller last wrote, drops the tracking, and notes it in the log. Your edit survives the next refresh tick.

The sanctioned forget path is the **Detach** button in Settings → Discovery (or `DELETE /api/discovery/docker/track/<key>` from a script).

Tracking itself belongs to Muximux, not to the save payload: `docker_key`, `docker_endpoint`, `docker_strategy` and `docker_managed_url` in a SaveConfig or per-app PUT are ignored and the stored values kept. A save can detach an app by changing its URL, but it cannot attach one, re-attach a detached one, or switch it to another container. Only Discover, auto-import and **Re-link** create or change tracking.

Apps with a fixed `muximux.app.url` are not rewritten by the poller; their `docker_managed_url` equals the label URL. Their `health_url` is refreshed from the container on every tick, so a health address edited in Settings is overwritten unless `muximux.app.health` is an absolute URL.

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

To have the dashboard open your proxy's public names instead of container addresses, see [Running behind your own reverse proxy](#running-behind-your-own-reverse-proxy).

---

## Troubleshooting

| Symptom | Diagnosis |
|---|---|
| Banner: "Connected to Docker but strategy `container_ip` cannot identify Muximux's container" | Muximux runs natively (not in a container). Set `network_filter` to scope the scan to a known docker network, or switch to `host_port`. |
| Banner: "Daemon unreachable: dial unix … no such file or directory" | The `endpoint` path is wrong, or the socket isn't bind-mounted into Muximux's container. |
| Banner: "Daemon unreachable: connect: permission denied" | The socket is mounted but the entrypoint's auto-detection didn't fire (e.g. unusual mount path, docker-socket-proxy sidecar). Override with `DOCKER_GID` set to the docker group GID the socket is owned by, or `DOCKER_SOCKET` to point the detection at a non-default path. See [Make the daemon socket reachable](#make-the-daemon-socket-reachable-from-muximux). |
| Discover modal shows containers but no auto-fill | The image isn't in Muximux's catalog. Add `muximux.app.*` labels to the container, or fill the fields manually before importing. |
| Imported app's URL doesn't update when container restarts | Check `refresh_interval` isn't set to 1h. Check the audit log for `Docker app URL refreshed` (and `Docker health address refreshed` for health addresses). Check the container hasn't been renamed (breaks `name:` tracking keys; Swarm and Compose containers use stable keys). |
| Gateway site doesn't serve after import | If you set `server.gateway_listen`, your upstream proxy needs to forward the host header to that port. Try `curl -H 'Host: site.example.com' http://muximux-host:8443/` to bypass the upstream. |
| Discover modal shows "Not importable: No port" | The container exposes no port Muximux can pick. Add `muximux.app.port`. |
| Container is labelled but not auto-imported | Check the reason in the Discover modal. Common causes: `muximux.app.enabled=false`, or explicit opt-in is on and `muximux.app.enabled=true` is missing. Containers with no `muximux.*` label are never auto-imported. |
| Log says "Invalid Docker auto-imported entry quarantined" | The entry is kept in `config.yaml` but not loaded. Fix the container's labels (the next refresh replaces it) or remove it under Settings -> Discovery -> Tracked. A manual gateway site linked to it loses its app link. |
| Log says "Swarm services unavailable; using task data only" | The endpoint is not a manager node, or a socket proxy blocks `/services`. Point Muximux at a manager, or set `SERVICES=1` on the proxy. |
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
| DELETE | `/api/discovery/docker/track/{key}` | - | Detach tracking for everything matching `key` on the current endpoint, and delete quarantined entries with that key on any endpoint |
| POST | `/api/discovery/docker/relink/probe` | `{key}` | "Does this key still resolve on the current daemon?" |
| POST | `/api/discovery/docker/relink/confirm` | `{old_key, new_key, strategy?}` | Move tracking from old key to new key |

## Container lifecycle controls

If you want the dashboard to start / stop / restart tracked containers from the overview page:

1. Edit `docker-compose.yml` and switch the Docker socket mount from `:ro` to `:rw` -- change the line to `- /var/run/docker.sock:/var/run/docker.sock:rw`. The `:ro` mount stays as the documented default; you opt in by changing this one character.
2. Set `discovery.docker.lifecycle_enabled: true` in your `config.yaml`, or toggle "Enable container lifecycle controls" under Settings -> Discovery (the checkbox is disabled until the socket is writable).
3. Optional: narrow who can use the controls. `discovery.docker.lifecycle_min_role` defaults to `admin`; set it to `power-user` or `user` to widen access. Set `discovery.docker.lifecycle_allowed_groups` to additionally require membership in specific groups. These are **user and identity-provider group names** (built-in user groups, the OIDC groups claim, the forward-auth groups header), not dashboard groups. Renaming or deleting a dashboard group never touches this list.
4. Optional: set `discovery.docker.health_badge_placement` to `overview_and_nav` to show container state badges in the navigation sidebar as well as the overview (default is `overview`; `off` hides them).

Once enabled, Docker-tracked apps on the overview show the Docker logo plus a small status dot when the container needs a glance: red for stopped, amber for unhealthy or paused, blue for restarting (a healthy, running container shows no dot - quiet by default). Hovering the card reveals the action buttons in a footer below it - Start when stopped, Stop and Restart when running; on touch devices the buttons stay visible. The footer sits outside the card's open-app area, so a tap to open the app can't trigger a container action by accident. Stop and Restart prompt for confirmation; Start fires immediately.

Every action - success, failure, and denied attempt (role floor not met, group mismatch, socket read-only, lifecycle disabled) - is audit-logged with the caller's username, app name, container id, and outcome (`source=audit`).

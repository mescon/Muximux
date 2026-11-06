# Docker Discovery

Connect Muximux to a Docker daemon and it can enumerate running containers, propose them as apps, and keep their URLs current as IPs shift across restarts. It is off by default because it requires read access to the Docker socket.

---

## What It Does

- **Scan**: lists running containers. For each one, it proposes a name, icon, URL, port, and group, with confidence ratings (high / medium / low).
- **Import**: adds the chosen containers in one click, as apps in the menu, gateway subdomains, or both. Each row picks a routing mode: Direct URL, internal Proxy, or Gateway domain.
- **Refresh**: a background poller (a task that runs on a timer, default 60s) looks up each tracked container on the daemon again. If the container's IP changes, it rewrites the saved URL. When gateway-site URLs change, Caddy (the web server built into Muximux) reloads once per tick (one run of the poller).
- **Auto-import** (optional): set `discovery.docker.auto_import` to `add`/`update`/`sync`. Muximux then imports, with no modal and no click, every container that carries at least one `muximux.*` label (a `muximux.discovery.id` alone is enough) and does not carry `muximux.app.enabled=false`. With [explicit opt-in](#explicit-opt-in) on, the container also needs `muximux.app.enabled=true`. Muximux also skips a container that has no usable URL or whose labels give an invalid entry (see [Why a container is skipped](#why-a-container-is-skipped)). Unlabelled containers are never auto-imported. `add` imports each new labeled container once. `update` also re-syncs an imported app when its labels change. `sync` also removes an imported app when its container disappears or opts out. Off by default. See [Automatic Import](#automatic-import).
- **Detach / Re-link**: per-row controls in Settings → Discovery. Detach stops auto-managing an entry. Re-link points it at a different container when you migrate daemons.

Upgrading from 3.5 or earlier? Read [Upgrading to 3.6](#upgrading-to-36) first.

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

1. Open **Settings → Discovery**.
2. Tick **Enable Docker discovery**.
3. Pick a **Network strategy**.
4. Click **Test connection**.

   The banner turns green when Muximux can reach the daemon and can identify enough about itself to use the chosen strategy.

5. Click **Save**. The test does not save the settings.
6. Open the **Apps** tab (or the **Gateway** tab).
7. Click **Discover from Docker**.

   The modal lists every running container that the daemon returned. Muximux leaves out its own container, so you can't import yourself as an app by accident. Each row shows:
   - Suggested name, icon, group (catalog-recognised images get these auto-filled at "medium" confidence; the catalog is Muximux's built-in list of known apps)
   - Stable tracking key, the ID that Muximux uses to follow the container (operator label > Swarm service > Compose service > container name > container ID)
   - Resolved URL preview
   - Stability warning when the key is likely to break on a `docker-compose --force-recreate` or swarm task reschedule
   - A **Not importable** chip when auto-import would skip the container, with the reason (see [Why a container is skipped](#why-a-container-is-skipped))

   The catalog matcher accepts the prefixes that operators often add to names. A container named `homelab-sonarr`, `homelab_radarr`, or `prod.plex` still picks up the matching catalog entry, just like a bare `sonarr` / `radarr` / `plex` would. The matcher splits names into tokens at `-`, `_`, and `.`. It compares whole tokens only, so `transmissionic` doesn't masquerade as `transmission`. Multi-word app names like `home-assistant` match through a fallback that compares adjacent pairs of tokens.

   A container that is already tracked shows a **Tracked** chip in the Discover dialog. The chip names what tracks it: an app, an auto-imported app, a gateway site or a quarantined entry, plus the endpoint when it differs. You cannot select its row, because you cannot import it again. A row whose name another app already uses shows "Name already in use by another app".

8. Pick the routing mode for each row:
   - **Direct** - menu links to `http://<container-ip>:<port>` directly
   - **Proxy** - menu links via Muximux's `/proxy/<slug>` path-prefix reverse proxy (a server that forwards requests to another server, here under a path on the Muximux address)
   - **Gateway domain** - menu links to `https://<your-subdomain>`; you must also create the gateway site in the same row

9. Click **Import**.

   If a row's group does not exist yet, the import creates it in the same save (see [Groups are created automatically](#groups-are-created-automatically)).

10. Open **Settings → Discovery → Currently tracked**.

   Every imported app or gateway site appears here. **Detach** on a row stops auto-management. **Re-link** appears when the saved `DockerEndpoint` no longer matches the configured endpoint (typical after a daemon migration). A row shows "Container missing since <time>" while Muximux cannot find its container on the daemon. Invalid Docker-owned entries that Muximux did not load are listed below the table; see [Quarantined entries](#quarantined-entries).

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
    lifecycle_enabled: false                # allow start/stop/restart of tracked containers (needs a daemon that accepts writes)
    lifecycle_min_role: admin               # min role for lifecycle controls
    lifecycle_allowed_groups: []            # additionally require group membership
    health_badge_placement: overview        # overview | overview_and_nav | off
```

### Make the daemon socket reachable from Muximux

How you grant Muximux access to the Docker daemon depends on how you run Muximux. If you leave the `endpoint:` field in `config.yaml` blank, Muximux fills in the platform default. So most operators only need the OS-level access step below.

#### Linux, running Muximux in a container (most common)

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    # ... usual ports / volumes / env ...
    volumes:
      - ./data:/app/data
      # :ro does not limit Docker API calls; see "Container lifecycle
      # controls" below for what actually gates start/stop/restart.
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

That is all you need. At startup, the entrypoint (the script that starts Muximux in the container) checks the bind-mounted socket and reads its group ownership from the host. It then adds the runtime user to a matching group inside the container. This lets the non-root muximux process read the socket. You do not need a `DOCKER_GID` env var or `group_add`, or to look up the right group number. The default `endpoint: unix:///var/run/docker.sock` already matches the mount.

The trailing `:ro` only stops the container from replacing or deleting the socket file. It does not limit what can be sent through the socket. Anything that can connect to the socket can use the whole Docker API, including to create privileged containers. Treat socket access as root on the host, whatever the mount flag. Gate the dashboard with auth. To narrow what Muximux can do, put a socket proxy (a small service that filters Docker API calls) in front of the socket. Then point `endpoint` at the proxy. Filter by endpoint, not only by HTTP method. A `GET` can attach to a running container's console through a WebSocket. A broad `POST` allowance for containers also allows creating them. Expose only what Muximux needs: listing and inspecting containers, plus services and tasks on Swarm. Add the start, stop and restart endpoints only if you use lifecycle controls. Keep the proxy itself reachable only by Muximux.

Container **lifecycle controls** (start / stop / restart, see below) are opt-in through two layers. The daemon must accept write requests, **and** `discovery.docker.lifecycle_enabled: true` must be set. When Muximux connects to the daemon, it sends a harmless `POST` to check the first layer. A plain socket mount passes this check, `:ro` or not. A socket proxy that blocks `POST` fails it. Neither layer alone is enough: the dashboard still won't start, stop, or restart anything until both are in place. Even with both on, Muximux can only start, stop, and restart the containers it already tracks. It can't pull or build images, create or delete containers, run commands inside them, or touch networks and volumes (Muximux never calls those parts of the Docker API). Muximux writes every action, and every blocked attempt, to the audit log.

For rootless Docker, docker-socket-proxy sidecars, or custom socket paths, two override env vars are available: `DOCKER_SOCKET` (the path the entrypoint stats) and `DOCKER_GID` (bypass auto-detect and force a specific GID). Almost no one needs these.

#### Linux, running Muximux as a native binary

In this setup there is no container, so there is no entrypoint script. Linux's normal socket permissions apply. The OS user that runs the binary must be a member of the host's `docker` group.

```bash
# Add the user that runs Muximux to the docker group:
sudo usermod -aG docker $(whoami)

# Then re-login (or restart the muximux service) so the new
# group membership takes effect.
```

If you install Muximux as a systemd unit, you can also set the group directly in the unit:

```ini
[Service]
User=muximux
SupplementaryGroups=docker
ExecStart=/usr/local/bin/muximux --data /var/lib/muximux
```

The default `endpoint: unix:///var/run/docker.sock` works as is.

#### macOS, running Muximux as a native binary

Docker Desktop on macOS creates a host-side socket symlink at `/var/run/docker.sock`. So the default `endpoint: unix:///var/run/docker.sock` works out of the box for most setups.

If you've disabled **Docker Desktop → Settings → Advanced → "Allow the default Docker socket to be used"**, Docker Desktop does not create the symlink. Either re-enable that toggle, or point `endpoint:` at the per-user socket that Docker Desktop still maintains:

```yaml
discovery:
  docker:
    endpoint: unix:///Users/<your-user>/.docker/run/docker.sock
```

#### Windows, running Muximux as a native binary

Docker on Windows exposes a **named pipe** (a Windows channel between programs) rather than a unix socket. Muximux 3.1.0+ supports the `npipe://` scheme natively. It uses that scheme by default when it runs on Windows, so you need no extra config:

```yaml
discovery:
  docker:
    enabled: true
    # endpoint is auto-filled to npipe:////./pipe/docker_engine on Windows
```

The Windows account that runs Muximux only needs to be a member of the `docker-users` group.

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

The remote daemon must expose its API with TLS (`dockerd --tlsverify`). The Muximux process must be able to read the cert/key paths.

### Network strategies

| Strategy | URL shape | Requires |
|---|---|---|
| `container_ip` | `http://<docker-network-ip>:<port>` | Muximux runs in a container on the same docker network, **or** `network_filter` is set |
| `container_dns` | `http://<container-name>:<port>` | Same prerequisites as `container_ip`; Docker's internal DNS resolves names within a network |
| `host_port` | `http://<host_ip>:<published-port>` | Container has `-p <host>:<container>` published; `host_ip` is set in config |
| `host_docker_internal` | `http://host.docker.internal:<published-port>` | Muximux runs in a Docker Desktop / WSL container where `host.docker.internal` resolves |

Strategy gating: when Muximux runs natively (not in a container), it cannot identify its own container. So `container_ip` and `container_dns` need a `network_filter` instead. The banner above the form tells you whether the chosen strategy is workable in your environment.

`network_filter` applies everywhere Muximux reads containers. The scan and the background refresh list the same filtered set and see the same networks. A container attached to several networks gets its URL from the filtered network only, so a refresh never rewrites it to an address on another network. A tracked container that is not on the filtered network counts as missing. On Swarm, the `/services` endpoint is read once per refresh tick.

### Labels on your containers

Label any container with `muximux.*` keys, and the Discover modal picks them up as high-confidence pre-fills (values filled in for you). After `docker compose up`, a fully-labelled container becomes a fully-configured Muximux entry with a single click in the Discover modal. You need no editing after the import. If you turn on [automatic import](#automatic-import), you need no click at all. This is the "GitOps your apps" pattern: declare your dashboard intent in `docker-compose.yml` alongside the service it describes.

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
      - muximux.app.allow_navigation=true
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

The example above sets every label at once. In practice you set only the few an app needs. With automatic import (3.2.0+), these labels are the whole configuration. Here is what each common case looks like on its own. Every block is a complete, copy-pasteable `services:` entry.

**A catalog app, the short way.** If the image is one Muximux recognises, a single tracking label is enough. The name, icon, group, port, and health come from the catalog:

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

**A button that fires a request, not a page.** Use `http_action` to turn a tile into a webhook trigger (for example an n8n flow), with a confirmation prompt. Muximux sends the request to the app URL, which it builds from `port` + `path`:

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

Sonarr opens at `https://sonarr.example.com`. The health check calls `http://<container IP>:8989/ping`. When the container's IP changes, the poller updates that address and logs "Docker health address refreshed". The poller never rewrites the app URL. Muximux resolves a relative health label against the container. It keeps an absolute URL label as written and never rewrites it. Without a health label, the check goes to the container's URL, including `muximux.app.path`. Enable monitoring with `muximux.app.health_check=true` or in the app's settings.

The poller owns the health address of such an app. It rewrites `health_url` on every refresh, so it overwrites an edit made in Settings. To pin the address, set `muximux.app.health` to an absolute URL. Muximux keeps that URL as written.

If you remove `muximux.app.url` later, the app returns to its container URL on the next refresh tick. Editing the URL by hand in Settings detaches the app, as with any tracked app. Known limitation: after you remove the `muximux.app.url` label, an app imported manually keeps the container address it had as its health address. Set it in the app's settings if needed. For auto-imported apps, auto-import resets it.

#### Icon label formats

`muximux.app.icon` and `muximux.group.icon` take the same values:

| Value | Icon | Example |
|---|---|---|
| `<slug>` | Dashboard Icons slug (the original form) | `sonarr` |
| `dashboard:<slug>` | Dashboard Icons slug, explicit | `dashboard:sonarr` |
| `lucide:<name>` | Lucide icon | `lucide:shield-check` |
| `https://...` | Icon image at that URL (https only) | `https://example.com/icons/media.svg` |
| `url:<url>` | Icon image at that URL, explicit | `url:https://example.com/icons/media.svg` |

```yaml
labels:
  - muximux.app.icon=lucide:database
  - muximux.group.icon=https://example.com/icons/media.svg
```

- A bare value is always a Dashboard Icons slug. Muximux never guesses Lucide from a bare name, because names such as `server` exist in both sets. Write `lucide:server` instead. A bare value containing `:`, `/`, `\`, `?`, `#` or whitespace is not a slug. Since 3.6.1, Muximux ignores it. Use the `dashboard:`, `lucide:` or URL form.
- Prefixes are case-insensitive. Slugs and Lucide names are lowercased; URLs are kept exactly as written (paths and queries are case-sensitive).
- A Lucide name is lowercase letters, digits and dashes. Muximux checks that format only, so a name Lucide does not have shows as a missing icon, as in the UI.
- A URL must be an absolute `https` URL with a host, no credentials or spaces, and at most 2048 characters. Muximux's Content-Security-Policy (the rules that tell the browser what it may load) only allows images over https. So Muximux refuses `http` URLs and other schemes such as `javascript:`, `data:` and `file:`. The browser loads a URL icon straight from that address, and Muximux does not proxy or cache it. So the host must be reachable from the browser.
- Muximux ignores an invalid value. The scan notes it, and Muximux logs one warning each time the value changes. The stored icon is kept, and discovery carries on.
- A label cannot name a custom upload.

#### Finding an icon slug

A bare `muximux.app.icon` value is a [Dashboard Icons](https://dashboardicons.com) slug: the icon filename without its extension (`sonarr.svg` becomes `sonarr`). The label flow has no GUI in front of you, so here is where to look one up:

- Browse the gallery at [dashboardicons.com](https://dashboardicons.com) and search for your app.
- Or search visually in Muximux's own app editor (Add or Edit an app, then the icon picker). The name you land on is the slug to paste into the label.
- Or list them straight from your running instance: `GET /api/icons/dashboard` returns every available slug (it is the same data the picker uses).

Omit the label to fall back to the catalog icon. For Lucide icons, see [Lucide](https://lucide.dev/icons) or the Lucide tab of the icon picker.

#### Full label reference

How values are read:

- **Booleans** are true for `true`, `1`, `yes` or `on` and false for `false`, `0`, `no` or `off` (case-insensitive, surrounding spaces ignored). Muximux ignores any other value, including an empty one, and reports it in the Discover modal. This applies to every boolean label (`enabled`, `default`, `proxy`, `proxy_skip_tls_verify`, `allow_notifications`, `allow_navigation`, `allow_deep_links`, `health_check`, `http_action_confirm`, `http_action_show_toast` and the `muximux.gateway.*` ones). So a misspelt `muximux.app.enabled=tru` does not opt the container out. On update and sync, a misspelt or empty value keeps the app or gateway site's current setting. So a typo or an undefined template variable in `muximux.gateway.require_auth` never makes a protected site public. On first import, the default applies.
- **Invalid values** for other typed labels (a port outside 1-65535, an unknown `open_mode`, a colour that is not a hex colour, and so on) are ignored, as if the label were not set. Muximux lists each ignored value in the Discover modal's notes for the container. It logs a warning once for each value, until the value changes.
- **Labels that are set win while an app is tracked.** On apps you imported by hand, Muximux re-syncs `muximux.app.name`, `icon`, `group` and `order` on every refresh, whatever the `auto_import` mode. Under `update`/`sync`, Muximux re-syncs auto-imported apps from all their labels. Under `off` and `add`, it leaves them alone. The `muximux.group.*` labels apply to groups that Docker discovery created. A label that is not set never changes anything. To take a field back, remove the label or **Detach** the app. See [Label re-sync for tracked apps](#label-re-sync-for-tracked-apps) and [Group labels](#group-labels).
- Unknown `muximux.*` labels are listed in the Discover modal's notes for the container.

##### Tracking

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.discovery.id` | string | (container name) | Stable tracking key that survives `docker-compose --force-recreate` and swarm reschedules. **Most important label** for plain containers - without it, Muximux falls back to the Swarm service, the Compose service, then the container name (see [Tracking keys](#tracking-keys)). Swarm tasks and Compose services stay stable without it. |

##### App fields (the menu entry)

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.app.enabled` | bool | `true` | Opt-out via `false`. With `require_explicit_enable` (or `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE`) only `true` opts in. See [Explicit opt-in](#explicit-opt-in). |
| `muximux.app.name` | string | catalog name or container name | Display name in the menu. Surrounding spaces are ignored. Re-synced while the app is tracked; see [Label re-sync for tracked apps](#label-re-sync-for-tracked-apps). |
| `muximux.app.icon` | string | catalog icon or `""` | A Dashboard Icons slug (e.g. `sonarr`), `lucide:<name>`, or an `https` URL; see [Icon label formats](#icon-label-formats). Surrounding spaces are ignored. |
| `muximux.app.group` | string | catalog group | Group the app lives in. Created if it doesn't exist (from 3.6.0). |
| `muximux.app.port` | int 1-65535 | catalog port or first exposed | Which container port the app listens on. |
| `muximux.app.url` | absolute `http(s)` URL | unset | Open the app at this URL instead of the container address, e.g. its public name behind your own reverse proxy. Health checks still go to the container. Ignored (with a scan note) when `muximux.app.gateway.domain` is set, and invalid values are ignored with a note. See [Running behind your own reverse proxy](#running-behind-your-own-reverse-proxy). |
| `muximux.app.scheme` | `http` \| `https` | `http` | Scheme for the constructed URL. |
| `muximux.app.path` | string | `/` | Sub-path appended to the container URL (e.g. `/admin`). Not applied to `muximux.app.url` or to a gateway site's backend. |
| `muximux.app.health` | string | catalog default | Health-check address: a full URL, or a path such as `/api/v3/health` resolved against the container. A value starting with `//` or `/\` is not a path and is ignored (since 3.6.1). |
| `muximux.app.health_check` | bool | unset | Enables (`true`) or disables (`false`) health monitoring for the app; the endpoint still comes from `muximux.app.health`. Unset keeps the setting in the app's form. Docker-managed: re-synced on update/sync and shown locked in Settings. |
| `muximux.app.color` | hex colour | unset | Accent color in the dashboard: `#rgb`, `#rgba`, `#rrggbb` or `#rrggbbaa`. |
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
| `muximux.app.allow_navigation` | bool | `false` | Let this app switch Muximux to another app (and open a page inside it) through `target="_top"` links and a `postMessage` API. A `_top` link can also send the whole tab to any website after a click, and a visible app can switch apps without a click, so enable it only for apps you trust. See [Navigation from Embedded Apps](apps.md#navigation-from-embedded-apps). |
| `muximux.app.allow_deep_links` | bool | `false` | Let a Muximux link such as `#emby?path=...` open a page inside this app. Anyone who can send you a link can then open any page of the app, so leave it off for apps that change something when a page is opened. See [Deep links in links](apps.md#deep-links-in-links). |
| `muximux.app.shortcut` | int 1-9 | unset | Keyboard shortcut slot. |
| `muximux.app.gateway.domain` | string | unset | Required for auto-import to create a gateway site; the derived `<name>.<dashboard domain>` default only pre-fills the import modal. When set, the import modal also offers a gateway-site entry for this subdomain. Pairs with the `muximux.gateway.*` labels below. |

##### Group fields (the group the app is in)

These describe the group the container's app is in. They apply only to a group Docker discovery created; see [Group labels](#group-labels).

| Label | Type | Default | What it does |
|---|---|---|---|
| `muximux.group.icon` | string | folder icon | The same forms as `muximux.app.icon` (slug, `lucide:<name>`, `https` URL). Surrounding spaces are ignored. |
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

Unknown `muximux.*` labels are surfaced in the Discover modal's per-row notes, so a typo shows up instead of being silently ignored.

---

## Automatic Import

Everything above is the **manual** path. Labels become high-confidence pre-fills, and you click **Import** to commit them. Automatic import removes that review step. When it is on, Muximux imports by itself every container that carries at least one `muximux.*` label (and is not opted out). Muximux skips containers that fail the conditions in [Explicit opt-in](#explicit-opt-in) and [Why a container is skipped](#why-a-container-is-skipped). There is no modal and no click. Muximux also keeps the imported apps in step with the labels over time.

> **Security -- read before enabling.** Auto-import is **off by default**. Only the host operator (who controls `config.yaml` or the environment) can turn it on. Once it is on, **any container on the shared Docker socket can write itself into your config with no review step**. This includes a `muximux.app.gateway.domain` label, which publishes a **public HTTPS subdomain with an auto-issued ACME certificate** (a TLS certificate that is issued automatically). Treat enabling this as trusting every label on every container the daemon can see. If you don't control all of those containers, leave it `off` and import by hand.

### Modes

Set the mode with the `discovery.docker.auto_import` config key, or override it with the `MUXIMUX_DISCOVERY_AUTO_IMPORT` environment variable (the env var wins). Four values:

| Mode | Adds new labelled containers (at least one `muximux.*` label, not opted out) | Re-syncs app fields when labels change | Removes the app when the container disappears or opts out |
|---|---|---|---|
| `off` (default) | no -- labels stay suggestions you import by hand | no | no |
| `add` | yes, once | no -- imported, then left alone forever | no |
| `update` | yes | yes | no |
| `sync` | yes | yes | yes -- full GitOps mirror |

- **`off`** -- today's behavior. Labeled containers are suggestions only. Muximux writes nothing until you click **Import**.
- **`add`** -- a newly labeled container is imported one time, then never touched again. Good for bootstrapping.
- **`update`** -- like `add`, plus Muximux re-syncs the app's fields from the labels whenever they change. Never removes anything.
- **`sync`** -- like `update`, plus Muximux removes the auto-imported app (and its gateway site) when its tracked container disappears from the daemon or opts out with `muximux.app.enabled=false`. Removal waits until the container has been absent for three consecutive successful scans, so a restart does not remove and re-create the app. Only auto-imported apps are removed. Muximux never touches apps you imported by hand or detached. The config becomes an exact mirror of the labelled containers.

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

The override applies in memory only and is never written to `config.yaml`. Settings -> Discovery shows the field as locked ("From MUXIMUX_DISCOVERY_AUTO_IMPORT"). The Discovery tab shows the stored values (host IP, network filter, refresh interval, auto-import mode, badge placement). When you save the tab, Muximux merges it onto the stored config, so fields that the tab does not show are kept. On Windows, `npipe://` endpoints can be entered in Settings.

### Opting a container out

Auto-import skips a container with `muximux.app.enabled=false`, and the Discover modal hides it. Use it to keep a labelled container off the dashboard without stripping its labels. Under `sync`, Muximux removes an app already auto-imported from that container, and its gateway site, after the grace period. Under `update`, Muximux leaves the app alone.

### Explicit opt-in

By default, every labelled container that is not opted out is imported. To require a positive label per container, set `discovery.docker.require_explicit_enable: true` (or `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE=true`, which wins and applies in memory only; Settings shows the field as locked). Then Muximux imports only containers with `muximux.app.enabled=true`. It skips labelled containers without it as `not_enabled`. Muximux ignores invalid values of the environment variable and logs a warning. (#496)

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

When a container loses all its `muximux.*` labels, Muximux detaches its auto-imported app from auto-import. The app is kept, its URL is still refreshed, and `sync` never removes it. The app is still tracked, so the [label re-sync](#label-re-sync-for-tracked-apps) applies to it if labels come back. The same happens in two other cases. On upgrade, it happens to an auto-imported app whose container has no labels. When you turn on explicit opt-in, it happens for containers without `enabled=true`.

### Edit-wins (URL edits detach)

Auto-import never silently overwrites a URL that you took manual control of. **Changing an auto-imported app's URL** detaches that app from auto-management. This applies to a change in Settings, through the API, or in `config.yaml` directly. Removing the container's `muximux.*` labels also detaches it from auto-import (see above). From then on it is a normal manual entry: `update`/`sync` will not re-sync it from labels, and `sync` will not remove it. This is the same edit-lock / auto-detach mechanism described below for tracked URLs.

Other managed-field edits (name, icon, group, and similar) do **not** detach. The same applies to the health check. When `muximux.app.health_check` is set, Muximux records a server-owned `docker_managed_health_check` marker on the app. It re-syncs the value under `update`/`sync`, and the app form shows the toggle locked. Remove the label to unlock it. Under `update`/`sync`, Muximux re-syncs those edited fields from the labels on the next tick, so the labels remain the source of truth. Under `add`, the edits stick, because `add` never re-syncs an already-imported app.

### Label re-sync for tracked apps

**While an app is tracked, labels that are set win; detach to take control.**

This holds for every tracked app, including apps you imported by hand through the Discover dialog. It also works with `auto_import: off`. On each refresh tick, Muximux checks every tracked app whose container is present. It applies these labels when they are set on the container:

| Label | Applied as |
|---|---|
| `muximux.app.name` | The app name. A gateway site linked to the app follows the rename. |
| `muximux.app.icon` | The icon, as on import (dashboard, Lucide or URL; see [Icon label formats](#icon-label-formats)). Icon colour, background, variant and invert are kept. |
| `muximux.app.group` | The group, matched to an existing group by name, ignoring case and spacing (`infra` finds `Infra`). Created if no group matches. |
| `muximux.app.order` | The order within the group. |

The URL and health address keep following the container as before.

- A label that is **not set** never changes anything: your own name, icon, group or order stays.
- If you change one of these fields in Settings while the label is set, the next tick sets it back. To take control, remove the label or **Detach** the app (Settings -> Discovery -> Currently tracked). Muximux never touches an app you detached (Detach in Settings), or an untracked app you created yourself in Settings.
- The re-sync only updates apps that are already tracked. It never imports a container or removes an app, whatever the `auto_import` mode.
- Auto-imported apps follow their `auto_import` mode instead (re-synced under `update`/`sync`, left alone under `off` and `add`), so no field is written twice.
- If you rename a group in Settings while a container's `muximux.app.group` label still names the old group, the label re-creates the old group on the next tick. Change the label too.
- When Muximux cannot apply a label, it holds the label back and logs a warning once. This happens when the name is already used by another app, or when the name is over 100 characters. Muximux compares names like proxy paths, so `TV` and `tv` collide. The app keeps its current value until the conflict is resolved.
- If saving the config fails, Muximux rolls back the whole tick and retries on the next tick.

### Groups are created automatically

When an app lands in a group that does not exist, Muximux creates the group in the same save. This covers a manual import from the Discover dialog, auto-import (new apps and re-synced ones) and the label re-sync above. It applies to a group from a `muximux.app.group` label as well as a catalog group.

- Muximux matches an existing group by name first, then ignoring case and spacing. So `media`, `Media ` and `media-server` / `Media Server` never create a near-duplicate. Muximux stores the app with the existing group's exact name.
- A new group gets the same defaults as one added in Settings: a folder icon, no colour, expanded. Muximux places it after your existing groups, in the order it processed the apps. Customise it in Settings like any other group.
- If you delete a group while a tracked app has `muximux.app.group` set to it, Muximux creates the group again on the next tick. Remove the label or detach the app to stop that.
- If the save fails, Muximux rolls back the new groups together with the apps.
- Muximux never hides an app whose group is missing anyway (for example a group removed from `config.yaml` by hand). The navigation and **Settings -> Apps** list it under **Ungrouped** until the group exists again.

### Group labels

`muximux.group.icon`, `muximux.group.color` and `muximux.group.order` set the look and position of the group a container's app is in. Put them on any container whose app is in the group. They need no `muximux.app.group` on the same container. But without a group there is nothing to apply them to, and the Discover dialog notes that.

```yaml
services:
  sonarr:
    labels:
      - muximux.app.group=Media
      - muximux.group.icon=plex
      - muximux.group.color=#e5a00d
      - muximux.group.order=1
```

- **Only groups Docker discovery created.** Muximux marks a group made by an import, auto-import or the label re-sync as Docker-managed (`docker_managed: true` in `config.yaml`). It applies the labels when it creates the group, and re-syncs them on every tick while the group stays managed. Muximux never touches a group you created yourself.
- **Edit in Settings to take control.** Changing the icon or colour of a managed group in Settings clears the marker. Changing its order (including dragging groups into a new order) clears the marker only when the order comes from a `muximux.group.order` label. If no label sets the order of a managed group, you can reorder it freely and it stays managed. Once the marker is cleared, the group is yours, and Muximux ignores later label changes. Renaming or expanding/collapsing the group does not count. To hand a group back to the labels, set `docker_managed: true` in `config.yaml`.
- **A label that is not set never changes anything:** the group keeps its own value for that field.
- **Conflicts.** When several containers in one group set different values, the container with the lowest tracking key wins, per field. The others get a scan note in the Discover dialog and a warning in the log (once until the conflict changes).
- **Invalid values** (a colour that is not a hex colour, an order outside 0-9999) are ignored. Muximux logs one warning per change and adds a scan note.
- Muximux reads the labels from the containers of tracked apps and of tracked gateway sites (through the site's linked app), whatever the `auto_import` mode. A group created by a manual import gets its label values on the next refresh tick.
- Applying them is part of the tick's single save. If the save fails, Muximux rolls back the group changes together with everything else.

### Gateway labels and `update`/`sync`

In `update` and `sync`, re-sync compares both the **app** fields and the **gateway site** built from labels against what is stored. Two kinds of gateway labels propagate on the next tick. The first kind changes the app's URL: `muximux.app.gateway.domain` and `muximux.gateway.tls`. The second kind is gateway-**only** labels that do not map to any app field: `muximux.gateway.require_auth`, `muximux.gateway.min_role`, `muximux.gateway.allowed_groups`, `muximux.gateway.streaming`, `muximux.gateway.strip_frame_blockers`, `muximux.gateway.forwarded_headers`, and `muximux.gateway.skip_tls_verify`. For example, if you toggle `muximux.gateway.require_auth` on a running container, Muximux re-syncs it without any app-field change.

Only the explicit `muximux.app.gateway.domain` label makes auto-import create a gateway site. With `server.tls.domain` set, the derived `<name>.<dashboard domain>` default is only a pre-fill in the import dialog. An update changes only label-managed fields. It keeps the health check, auth bypass, access, scale, pinned, proxy headers and other per-app settings.

**Upgrade note:** when `server.tls.domain` was set, earlier versions created a gateway site `<name>.<your domain>` for every labelled container. If you relied on those derived subdomains, add `muximux.app.gateway.domain=<name>.<your domain>` to those containers before upgrading. Otherwise, the first refresh removes the sites and points the apps at their container URLs.

If you remove the `muximux.app.gateway.domain` label from an already-imported container, the next tick acts on it. Muximux returns the app to the direct container URL and drops the gateway site, which no label declares any more.

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

So Swarm tasks and Compose services no longer need `muximux.discovery.id` for stability. On the first refresh, Muximux migrates existing `name:` keys to the stable key automatically and logs a `Docker tracking key migrated` audit line. Entries tracked on another endpoint are migrated after **Re-link**. Quarantined entries keep their old key. `sync` removes them once the task is gone from the endpoint it polls, or you can remove them in Settings. `sync` also cleans up duplicates left by earlier redeploys.

There is one app per key. Muximux collapses Swarm replicas and the containers of a scaled Compose service (`--scale`) into a single app. With `container_ip` and several replicas, the address may alternate between them.

### Docker Swarm

- Labels under the service's `labels:` reach the task. `deploy.labels` are service labels, and Muximux reads them too. Container labels win when both set the same key.
- Published ports come from the service, so the `host_port` and `host_docker_internal` strategies work with ingress-published ports.
- `container_dns` uses the service name.
- The endpoint must be a manager node, because only managers answer `/services`. On a worker, the scan shows a note and the app needs `muximux.app.port`.
- Behind a socket proxy such as tecnativa/docker-socket-proxy, allow `/services` with `SERVICES=1`. Without it, Muximux logs one warning ("Swarm services unavailable; using task data only") and uses task data only. Service labels and published ports are then missing.

---

## Quarantined entries

A Docker-owned entry is an auto-imported app, or a gateway site that auto-import created for one. An invalid one fails validation, for example one that an older version wrote without a URL. Such an entry in `config.yaml` no longer stops Muximux from starting, and it no longer causes a save from Settings to be rejected. Instead, Muximux quarantines the entry. The entry is:

- kept in `config.yaml`, unchanged, until it is fixed, removed, or superseded (see below),
- not loaded, so it does not appear on the dashboard,
- logged once at startup as "Invalid Docker auto-imported entry quarantined", and
- listed in **Settings -> Discovery -> Tracked** with its reason.

If auto-import is `add`, `update` or `sync`, fix the container's labels, and the next refresh replaces a quarantined app. A quarantined gateway site of a live app is replaced only under `update` or `sync`. With auto-import `off`, a refresh does not replace the entry. Or remove it from the list (**Remove** or **Remove all**). Removing deletes the entry from `config.yaml`. A live app or gateway site that shares its tracking key stays tracked. A quarantined entry is also superseded when a live app takes its name or slug, or a live gateway site takes its domain. Muximux then drops the entry from `config.yaml` at the next save and logs "Quarantined entry superseded by a live entry". A quarantined site goes with its superseded app. Under `sync`, Muximux removes a quarantined app whose container is gone from the polled endpoint after three consecutive scans, like an auto-imported app. A manual gateway site that was linked to a quarantined app loses its app link. Only Docker-owned entries are quarantined. Other invalid config still fails validation as before.

Other housekeeping applies to discovery. Image names whose last segment is generic (`server`, `app`, `web`, `api` and similar) no longer match a catalog entry. Muximux deduplicates apps whose names differ only by case or spacing. When a tracked container goes missing, Muximux logs it once instead of on every refresh.

---

## Upgrading to 3.6

Discovery behaves more predictably from 3.6.0, and a few changes show up on the first refresh after an upgrade:

- **Auto-import needs a label.** Only containers with at least one `muximux.*` label are auto-imported. Under `update`/`sync`, the first refresh detaches an auto-imported app from auto-import if its container has no labels. The app stays and its URL keeps refreshing, but `sync` no longer removes it. A label that you add later does not put the app back under auto-import. The app stays tracked, so its `name`, `icon`, `group` and `order` labels re-sync. But `update` does not apply its other labels, and `sync` does not remove it. To make it auto-imported again, delete the app. The next refresh then imports the labelled container again. See [Containers that lose their labels](#containers-that-lose-their-labels).
- **New tracking keys for Swarm and Compose.** Keys become `swarm:<service>` and `compose:<project>:<service>`. On the first refresh, Muximux migrates existing `name:` keys in place and logs `Docker tracking key migrated`. Entries tracked on another endpoint are migrated after **Re-link**. See [Tracking keys](#tracking-keys).
- **Swarm app names.** Under `update`/`sync`, Muximux renames an auto-imported Swarm app without `muximux.app.name` from the task name to the service name. This can change its slug and its `/proxy/<slug>/` path. Behind a socket proxy, allow `/services` (`SERVICES=1`).
- **Labels re-sync onto apps you imported by hand.** A set `muximux.app.name`, `icon`, `group` or `order` label now wins on every tracked app, in every mode including `off`. So the next refresh resets an edit to those fields in Settings. Remove the label or **Detach** the app to keep your edit. See [Label re-sync for tracked apps](#label-re-sync-for-tracked-apps).
- **Invalid entries no longer stop startup.** An invalid auto-imported app or gateway site in `config.yaml` is quarantined (kept in the file, not loaded) with a warning. See [Quarantined entries](#quarantined-entries).
- **`sync` removes less.** Only auto-imported apps are removed, and only after the container has been absent for three consecutive scans.
- **Catalog matching.** Images whose last path segment is generic (`server`, `app`, `web`, `api` and similar) no longer match a catalog entry by that segment, so such a container may need `muximux.app.name`, `icon` and `port` labels.
- **`lifecycle_allowed_groups`** holds user and identity-provider group names; renaming or deleting a dashboard group no longer changes it.

From 3.6.1:

- The Docker whale in the navigation and on the overview is shown to admins only, and can be hidden with `navigation.show_docker_badge: false`.
- `MUXIMUX_DISCOVERY_AUTO_IMPORT` and `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE` now also apply on a first start without `config.yaml`. A container that relied on them being ignored there will see them take effect.
- A `muximux.app.health` value starting with `//` or `/\` is ignored.
- Muximux trims a bare `muximux.app.icon` (or `muximux.group.icon`) value. It then refuses the value as a Dashboard Icons slug if it still contains `:`, `/`, `\`, `?`, `#`, whitespace or control characters. Use the `dashboard:`, `lucide:` or URL forms (see [Icon label formats](#icon-label-formats)).

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
- Muximux's built-in path-prefix reverse proxy strips iframe-blockers (headers that stop a page from loading in an iframe), rewrites HTML/CSS/JS paths, isolates `window.parent`, etc. This lets apps that refuse to load in iframes work.
- Same tracking semantics as Direct (poller refreshes the upstream URL)

Use when: the app misbehaves in iframes, the dashboard machine cannot reach the container directly, or you want auth/CSP layering through Muximux.

### Gateway domain

```
Browser → https://sonarr.example.com → Muximux (embedded Caddy, :443, auto-HTTPS) → http://192.168.1.4:8989
```

Point the subdomain's DNS at the Muximux host. Muximux **is** the reverse proxy here. Its embedded Caddy listens on ports 80/443 and gets the certificate from Let's Encrypt. Then it proxies the request to the container. You do not put nginx or another proxy in front of it for this to work.

- **App.url** = `https://<gateway-domain>` (static)
- **App.proxy** = false
- **App is NOT auto-managed**: the gateway site becomes the docker-managed entry instead
- The poller refreshes the gateway site's `backend_url` (not the App's URL, which doesn't depend on the container)

Use when: you want a public subdomain that survives container moves, and a clickable dashboard link that uses that domain.

> You may already run an edge proxy (Cloudflare Tunnel, an upstream Traefik/nginx) and would rather it terminate TLS (decrypt the HTTPS traffic). If so, set `server.gateway_listen: ":8443"`. Then have that proxy forward to Muximux on that port instead. In that case, the flow is `Browser → upstream proxy → muximux:8443 → http://192.168.1.4:8989`.

---

## Edit Lock + Auto-Detach

When an App or GatewaySite is tracked, the URL field in the editor is **read-only** and shows an amber lock badge. Muximux protects your edits across all three ways you might change the URL:

1. **In Settings**, the URL field is locked. To change it, click **Detach** under Settings → Discovery → Currently tracked. The app's URL unlocks right away in the same Settings session. If you save afterwards, the app stays detached.
2. **Through the API**, if a SaveConfig request changes the URL of a tracked entry, Muximux treats that as a deliberate takeover. It drops the tracking and writes an audit log entry.
3. **In `config.yaml` by hand**, the same thing happens at next boot. Muximux notices that your URL differs from the one the poller last wrote. It drops the tracking and notes it in the log. Your edit survives the next refresh tick.

The sanctioned forget path is the **Detach** button in Settings → Discovery (or `DELETE /api/discovery/docker/track/<key>` from a script).

Tracking itself belongs to Muximux, not to the save payload. Muximux ignores `docker_key`, `docker_endpoint`, `docker_strategy` and `docker_managed_url` in a SaveConfig or per-app PUT, and keeps the stored values. A save can detach an app by changing its URL. But it cannot attach one, re-attach a detached one, or switch it to another container. Only Discover, auto-import and **Re-link** create or change tracking.

The poller does not rewrite apps with a fixed `muximux.app.url`. Their `docker_managed_url` equals the label URL. The poller refreshes their `health_url` from the container on every tick. So it overwrites a health address edited in Settings, unless `muximux.app.health` is an absolute URL.

### docker_managed_url (internal)

You'll see a `docker_managed_url` field appear next to `docker_key` for tracked entries. Muximux writes it from the import flow and updates it on every poller tick. You don't need to touch it. If you do write a tracked entry by hand from scratch, set `docker_managed_url` to the same value as `url` (or `backend_url` for a gateway site). The file-edit detach mechanism then has a baseline to compare against.

---

## Divergence Banner

Caddy's reload is transactional (all or nothing) only at the parse step. Failures after the parse step (listener collision, async cert provisioning, module panic) can leave the running config in an unknown state. The refresh poller handles this with rollback (a reload of the previous config):

- Candidate reload fails, rollback reload succeeds → audit log warning, refresh tick skipped
- Both fail → **divergence counter** increments, sticky red banner appears in Settings → Discovery
- First clean tick after a divergence → banner transitions to amber "recovered"

The banner shows at a glance that the running Caddy may not match disk. Recovery happens automatically on the next successful tick. The banner stays amber until you acknowledge it (currently by waiting; a future iteration may add a "clear" button).

---

## Running Behind Another Reverse Proxy

By default, Caddy listens on 80/443 with auto-HTTPS. This requires root or `CAP_NET_BIND_SERVICE`. For "behind another proxy" topologies (Traefik, Cloudflare Tunnel, nginx), set `server.gateway_listen` to a non-privileged port:

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

All endpoints are admin-only except `GET /api/discovery/docker-state`, which returns the container state of the apps the caller can see.

| Method | Path | Body | Description |
|---|---|---|---|
| GET | `/api/discovery/docker/status` | - | Capability + cache status |
| GET | `/api/discovery/docker/networks` | - | Docker networks, for the network filter |
| GET | `/api/discovery/docker/config` | - | The stored discovery settings under `config`, plus `env_overrides` for fields set by an environment variable |
| PUT | `/api/discovery/docker/config` | `DiscoveryDockerConfig` | Merge the fields sent onto the stored discovery settings (omitted fields are kept), save, and rebuild the service |
| POST | `/api/discovery/docker/test` | `DiscoveryDockerConfig` | Probe a candidate config without saving |
| GET | `/api/discovery/docker/scan` | - | Enumerate running containers as `Suggestion` list, with `labeled`, `label_enabled`, `auto_import_skip`, `tracked` and `name_taken` |
| POST | `/api/discovery/docker/import` | `{items: ImportItem[]}` | Atomic batch import of selected containers |
| GET | `/api/discovery/docker/tracked` | - | Current tracked apps + sites with last-seen and `missing_since` timestamps, and the `quarantined` entries |
| DELETE | `/api/discovery/docker/track/{key}` | - | Detach tracking for everything matching `key` on the current endpoint, and delete quarantined entries with that key on any endpoint. `?scope=quarantined` deletes only the quarantined entries |
| POST | `/api/discovery/docker/relink/probe` | `{key}` | "Does this key still resolve on the current daemon?" |
| POST | `/api/discovery/docker/relink/confirm` | `{old_key, new_key, strategy?}` | Move tracking from old key to new key |
| GET | `/api/discovery/docker-state` | - | Container state of tracked apps (any signed-in user; filtered to the apps they can see) |

Field-level details and examples are in the [API Reference](api.md#discovery).

## Container lifecycle controls

If you want the dashboard to start / stop / restart tracked containers from the overview page:

1. Make sure the daemon accepts write requests from Muximux. A plain socket mount already does (`:ro` makes no difference). If Muximux talks to a socket proxy, allow `POST` to the start, stop and restart endpoints there, and nothing broader.
2. Set `discovery.docker.lifecycle_enabled: true` in your `config.yaml`. Or toggle "Enable container lifecycle controls" under Settings -> Discovery. The checkbox is disabled until the socket is writable.
3. Optional: set the minimum role for the controls. `discovery.docker.lifecycle_min_role` defaults to `admin`. Set it to `power-user` or `user` to widen access.
4. Optional: narrow who can use the controls. Set `discovery.docker.lifecycle_allowed_groups` to additionally require membership in specific groups. These are **user and identity-provider group names** (built-in user groups, the OIDC groups claim, the forward-auth groups header), not dashboard groups. Renaming or deleting a dashboard group never touches this list.
5. Optional: set `discovery.docker.health_badge_placement` to `overview_and_nav` to show container state badges in the navigation sidebar as well as the overview (default is `overview`; `off` hides them).

Once the controls are enabled, Docker-tracked apps on the overview show the Docker logo (to admins only; hide it with `navigation.show_docker_badge: false`, see [Navigation > Docker Badge](navigation.md#docker-badge)). When the container needs a glance, the card also shows a small status dot: red for stopped, amber for unhealthy or paused, blue for restarting. A healthy, running container shows no dot, so cards stay quiet by default. When you hover over the card, the action buttons appear in a footer below it (Start when stopped, Stop and Restart when running). On touch devices, the buttons stay visible. The footer sits outside the card's open-app area, so a tap to open the app can't trigger a container action by accident. Stop and Restart prompt for confirmation. Start fires immediately.

Muximux writes every action to the audit log, with the caller's username, app name, container id, and outcome (`source=audit`). That covers successes, failures and denied attempts (role floor not met, group mismatch, socket read-only, lifecycle disabled).

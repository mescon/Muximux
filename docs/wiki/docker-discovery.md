# Docker Discovery

Connect Muximux to a Docker daemon and it can enumerate running containers, propose them as apps, and keep their URLs current as IPs shift across restarts. **Off by default** because it requires read access to the Docker socket.

---

## What It Does

- **Scan**: lists running containers and proposes a name, icon, URL, port, and group for each, with confidence ratings (high / medium / low).
- **Import**: one-click adds the chosen containers as apps in the menu, gateway subdomains, or both. Each row picks a routing mode: Direct URL, internal Proxy, or Gateway domain.
- **Refresh**: a background poller (default 60s) re-resolves each tracked container against the daemon and rewrites the saved URL if the container's IP changes. Caddy reloads once per tick when gateway-site URLs change.
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

2. **Apps tab** (or Gateway tab) → **Discover from Docker**: the modal lists every running container the daemon returned. Each row shows:
   - Suggested name, icon, group (catalog-recognised images get these auto-filled at "medium" confidence)
   - Stable tracking key (operator label > container name > container ID)
   - Resolved URL preview
   - Stability warning when the key is likely to break on a `docker-compose --force-recreate` or swarm task reschedule

3. **Pick routing per row** and click **Import**:
   - **Direct** — menu links to `http://<container-ip>:<port>` directly
   - **Proxy** — menu links via Muximux's `/proxy/<slug>` path-prefix reverse proxy
   - **Gateway domain** — menu links to `https://<your-subdomain>`; requires also creating the gateway site in the same row

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
```

### Make the daemon socket reachable from Muximux

Setting `endpoint: unix:///var/run/docker.sock` in `config.yaml` is only half the story. When Muximux itself runs in a container, the socket also has to be mounted into that container - otherwise the path resolves to nothing and the Discovery tab shows "daemon unreachable" with `connect: no such file or directory`.

Add this to your `docker-compose.yml` (or a `-v` flag if you run `docker run` directly):

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    # ... usual ports / volumes / env ...
    volumes:
      - ./data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

The trailing `:ro` is intentional. Muximux only **reads** container metadata - it never starts, stops, or builds anything. Marking the mount read-only means a compromised Muximux can't be used to pivot to full daemon control. The Docker engine API still exposes enough surface to enumerate every container on the host, so treat this mount as "effective host root for read" and gate the dashboard accordingly with auth.

For a **remote daemon over TCP**, you don't need a socket mount at all - point `endpoint:` at `tcp://your-daemon:2376` and ship the mTLS cert paths through the `tls:` block. The remote daemon must expose its API with TLS (Docker's `dockerd --tlsverify`), and your client cert/key paths must be readable by the Muximux process.

### Network strategies

| Strategy | URL shape | Requires |
|---|---|---|
| `container_ip` | `http://<docker-network-ip>:<port>` | Muximux runs in a container on the same docker network, **or** `network_filter` is set |
| `container_dns` | `http://<container-name>:<port>` | Same prerequisites as `container_ip`; Docker's internal DNS resolves names within a network |
| `host_port` | `http://<host_ip>:<published-port>` | Container has `-p <host>:<container>` published; `host_ip` is set in config |
| `host_docker_internal` | `http://host.docker.internal:<published-port>` | Muximux runs in a Docker Desktop / WSL container where `host.docker.internal` resolves |

Strategy gating: when Muximux runs natively (not in a container), `container_ip` and `container_dns` need a `network_filter` to substitute for self-identification. The banner above the form tells you whether the chosen strategy is workable in your environment.

### Labels on your containers

Add these labels to any container to override Muximux's defaults:

```yaml
labels:
  - muximux.discovery.id=sonarr-stable      # stable tracking key across recreates
  - muximux.app.enabled=true                # opt-out via "false" even if catalog matches
  - muximux.app.name=Sonarr
  - muximux.app.port=8989
  - muximux.app.scheme=https                # default: http
  - muximux.app.icon=sonarr                 # any dashboard-icons name
  - muximux.app.group=Media
  - muximux.app.path=/                      # appended to URL on import; useful for backends behind a sub-path
  - muximux.app.health=/api/v3/health       # backend health endpoint (overrides catalog default)
  - muximux.app.gateway.domain=sonarr.example.com  # seed the Discover modal's gateway-domain input
```

`muximux.discovery.id` is the most important — without it, Muximux falls back to the container *name* as the tracking key, which breaks on `docker-compose --force-recreate` (Compose appends a numeric suffix that bumps every recreate).

---

## Routing Modes Explained

When you check "Add to menu" in the import modal, a radio selector decides how the menu link works:

### Direct

```
Browser → http://10.0.0.4:8989  (your dashboard machine reaches the container directly)
```

- **App.url** = the discovered container URL
- **App.proxy** = false
- The poller refreshes App.url every tick when the container's IP changes

Use when: your dashboard machine has direct network reachability to the container (same host, same subnet, VPN).

### Proxy

```
Browser → /proxy/sonarr → Muximux Go server → http://10.0.0.4:8989
```

- **App.url** = the container URL (kept as the upstream)
- **App.proxy** = true
- Muximux's built-in path-prefix reverse proxy strips iframe-blockers, rewrites HTML/CSS/JS paths, isolates `window.parent`, etc. — makes apps work in iframes that refuse them.
- Same tracking semantics as Direct (poller refreshes the upstream URL)

Use when: the app misbehaves in iframes, the dashboard machine cannot reach the container directly, or you want auth/CSP layering through Muximux.

### Gateway domain

```
Browser → https://sonarr.example.com → upstream proxy → muximux:8443 → http://10.0.0.4:8989
```

- **App.url** = `https://<gateway-domain>` (static)
- **App.proxy** = false
- **App is NOT auto-managed** — the gateway site becomes the docker-managed entry instead
- The poller refreshes the gateway site's `backend_url` (not the App's URL, which doesn't depend on the container)

Use when: you want a public subdomain that survives container moves, and a clickable dashboard link that uses that domain.

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

The banner gives you a one-glance signal that the running Caddy may not match disk. Recovery happens automatically on the next successful tick; the banner stays amber until you acknowledge it (currently by waiting — a future iteration may add a "clear" button).

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
| Banner: "Daemon unreachable: dial unix … no such file or directory" | The `endpoint` path is wrong, or the user running Muximux doesn't have permission on the socket. |
| Discover modal shows containers but no auto-fill | The image isn't in Muximux's catalog. Add `muximux.app.*` labels to the container, or fill the fields manually before importing. |
| Imported app's URL doesn't update when container restarts | Check `refresh_interval` isn't set to 1h. Check the audit log for `Docker app URL refreshed`. Check the container hasn't been renamed (breaks `name:` tracking keys). |
| Gateway site doesn't serve after import | If you set `server.gateway_listen`, your upstream proxy needs to forward the host header to that port. Try `curl -H 'Host: site.example.com' http://muximux-host:8443/` to bypass the upstream. |
| Divergence banner is red and won't clear | Inspect the most recent `Docker refresh divergence` audit log line for the candidate + rollback errors. Most often a Caddyfile parse-OK but listener-collide situation. Restart Muximux to recover. |

---

## API Reference

All endpoints are admin-only.

| Method | Path | Body | Description |
|---|---|---|---|
| GET | `/api/discovery/docker/status` | — | Capability + cache status |
| PUT | `/api/discovery/docker/config` | `DiscoveryDockerConfig` | Persist new discovery settings + rebuild service |
| POST | `/api/discovery/docker/test` | `DiscoveryDockerConfig` | Probe a candidate config without saving |
| GET | `/api/discovery/docker/scan` | — | Enumerate running containers as `Suggestion` list |
| POST | `/api/discovery/docker/import` | `{items: ImportItem[]}` | Atomic batch import of selected containers |
| GET | `/api/discovery/docker/tracked` | — | Current tracked apps + sites with last-seen timestamps |
| DELETE | `/api/discovery/docker/track/{key}` | — | Detach tracking for everything matching `key` on the current endpoint |
| POST | `/api/discovery/docker/relink/probe` | `{key}` | "Does this key still resolve on the current daemon?" |
| POST | `/api/discovery/docker/relink/confirm` | `{old_key, new_key, strategy?}` | Move tracking from old key to new key |

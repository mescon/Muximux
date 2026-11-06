# Troubleshooting

> **Tip:** Enable [debug logging](debug-logging.md) to get detailed diagnostic information. Add `?debug=true` to your URL for frontend console logs. For backend request-level detail, set the log level to `debug` in Settings.

---

## App Won't Load in Iframe

**Symptom:** Blank iframe, "refused to connect" error, or security error in browser console.

**Cause:** Many web applications send HTTP headers (`X-Frame-Options`, `Content-Security-Policy`) that block embedding. These headers stop other pages from showing the app in an iframe (a frame that shows one web page inside another page).

**Solutions:**

1. **Enable the proxy** -- Set `proxy: true` on the app. Muximux's built-in reverse proxy (a server that forwards requests to another server) strips the headers that block iframe embedding.

2. **Debug the proxy** -- If proxy mode doesn't help, the app may use JavaScript to detect the iframe. To see what the app looks like through the proxy, open `/proxy/{slug}/` directly in a new browser tab.

3. **Use a different open mode** -- As a last resort, set `open_mode: new_tab`. The app then opens in a separate browser tab instead of an iframe.

---

## App Loads But Looks Broken (CSS/JS Missing)

**Symptom:** The app loads in the iframe but styles are missing, images don't appear, or there are JavaScript errors in the browser console.

**Cause:** When the reverse proxy serves the app, the app's internal URLs for CSS, JavaScript, and images may not resolve correctly.

**Solutions:**

1. Open browser DevTools (F12).

2. In the **Network** tab, look for 404 errors on asset files.

3. Look in the app's own configuration for a "base URL", "URL base", or "path prefix" setting. If the app has one, try setting it to `/proxy/{slug}`. `{slug}` is the lowercase, hyphenated version of your app name.

4. Some apps need specific configuration to work behind a reverse proxy. Check the app's own documentation for reverse proxy setup instructions.

---

## Reverse Proxy Limitations

The built-in reverse proxy rewrites paths in HTML, CSS, JavaScript, JSON, and HTTP headers. This lets apps work at `/proxy/{slug}/` instead of their original URL. However, the proxy cannot rewrite some patterns. This section explains what is **not supported**, why, and what you can do about it.

### JavaScript That Builds URLs at Runtime

Muximux includes a runtime URL interceptor (a script that rewrites URLs inside the browser). It handles most of these cases automatically. The interceptor patches `fetch()`, `XMLHttpRequest`, `WebSocket`, `EventSource`, and DOM property setters (`img.src`, etc.). As a result, it rewrites URLs that the page builds at runtime before the browser sends them. This section describes the remaining edge cases.

**Symptom:** The app loads, but some features, API calls, or navigation links point to wrong paths. The Network tab shows 404 errors for URLs that are missing the `/proxy/{slug}/` prefix.

**Patterns that may still cause issues:**

| Pattern | Example | Why |
|---------|---------|-----|
| `history.pushState` | `pushState({}, '', '/page')` | Client-side navigation bypasses the server |
| `history.replaceState` | `replaceState({}, '', '/page')` | Same as above |
| `location.pathname` checks | `if (location.pathname === '/login')` | Reads the current path which includes the proxy prefix |
| Inline event handlers | `<div onclick="go('/page')">` | URL in HTML attribute, not intercepted by runtime patches |

**Workaround:** Check if the app has a "base URL", "URL base", or "path prefix" setting in its own configuration. If it does, set it to `/proxy/{slug}`. The app then adds the correct prefix when it builds URLs. Many apps support this setting (Sonarr, Radarr, Prowlarr, Lidarr, Bazarr, Seerr, Tautulli, etc.).

### Single-Page App (SPA) Routing Issues

**Symptom:** The app loads initially but navigating within it causes a blank page, a 404, or routes back to the home page.

**What happens:** SPAs define client-side routes like `/dashboard` or `/users/settings`. When Muximux proxies the app at `/proxy/my-app/`, the browser's URL becomes `/proxy/my-app/dashboard`. The app's router sees the full path, including the `/proxy/my-app/` prefix. The router doesn't recognize this path, so the route doesn't match.

The proxy reduces this problem. It rewrites base path configuration variables (e.g., `urlBase: ""` becomes `urlBase: "/proxy/my-app"`), which helps many apps. However, some apps may still break: apps that hardcode routes in their JavaScript, and apps that use a routing framework that doesn't respect the base path.

**Workaround:** If the app has a base URL/path prefix setting, set it to `/proxy/{slug}`. This is the most reliable fix, because the app then knows its actual path.

### Service Workers

**Symptom:** The app works on first load but breaks after a page refresh. Cached content can also appear stale, or the app tries to serve offline content at wrong paths.

**What happens:** A service worker (a script that the browser runs in the background for a site) registers under the page's origin and intercepts network requests. When Muximux proxies an app, the app's service worker would register under Muximux's origin. It could then intercept unrelated requests.

**Mitigation:** Since v3.0.9, Muximux's proxy interceptor automatically blocks `navigator.serviceWorker.register()` calls from proxied apps. It also cleans up any registrations that leaked before. A service worker registered before you upgraded can still cause stale cache issues. In that case, clear it manually: open DevTools > Application > Service Workers > Unregister.

### Binary and Non-Text Protocols

**Symptom:** Features that use gRPC, Protocol Buffers, MessagePack, or other binary protocols fail when the app is proxied.

**What happens:** The proxy rewrites text-based content by matching patterns in the response body. Binary protocols encode data in non-text formats. In those formats, regex-based rewriting cannot find path strings or change them safely.

**Workaround:** No proxy-side fix is possible. For apps that rely on binary protocols, use `open_mode: new_tab`. Or, if the app has a non-binary API, configure the app to use it.

### Large Text Responses and Memory

**Symptom:** Muximux uses more memory than expected when a proxied app serves very large text responses. An example is a JSON API endpoint that returns hundreds of megabytes of data.

**What happens:** The proxy buffers (holds in memory) text-based responses (HTML, CSS, JavaScript, JSON, XML) so it can rewrite URLs in the content. Unusually large text responses can cause memory pressure.

The proxy does **not** buffer binary content (images, videos, archives, file downloads). This content streams directly from the backend to your browser without being read into memory. The proxy buffers only text content types for rewriting.

### Cookie Domain Attribute

**Symptom:** Login sessions in a proxied app don't persist, or you need to log in repeatedly.

**What happens:** The proxy rewrites the `Path` attribute of `Set-Cookie` headers, so cookies are scoped to the proxy path. However, the proxy does **not** rewrite the `Domain` attribute. The app may set a cookie with an explicit domain (e.g., `Domain=app.internal`). The browser may then reject the cookie, because the cookie domain doesn't match the Muximux domain.

**Workaround:** If possible, configure the app to not set an explicit cookie domain. Most apps that are designed for reverse proxy use work correctly without an explicit domain.

### Strict Origin Validation

**Symptom:** A proxied app rejects requests when you submit forms or make API calls. It returns CSRF errors, "invalid origin", or "forbidden" responses.

**What happens:** Some apps check the `Origin` or `Referer` HTTP header to prevent cross-site request forgery (CSRF, an attack where another site sends requests in your name). Through the proxy, these headers contain Muximux's hostname instead of the app's hostname. The app then rejects the request.

**Workaround:** Look in the app's settings for a "trusted origins" or "CORS allowed origins" option. Add Muximux's URL there. Some apps also have a "disable CSRF" or "allow reverse proxy" toggle.

---

## Health Checks Show Unhealthy But App Works

**Causes:**
- The app's main URL requires authentication, and Muximux's health checker does not send credentials.
- The URL returns a non-2xx status code (for example, a 302 redirect to a login page).
- The app takes longer than the configured timeout to respond.

**Solutions:**

1. **Set a custom health URL** -- Point `health_url` at an unauthenticated endpoint. Many apps offer endpoints like `/api/health`, `/ping`, `/status`, or `/identity` that don't require login.

   ```yaml
   apps:
     - name: Sonarr
       url: http://sonarr:8989
       health_url: http://sonarr:8989/ping
   ```

2. **Increase the timeout** -- If the app is slow to respond, increase `health.timeout` in your configuration:

   ```yaml
   health:
     timeout: 10s
   ```

3. **Check network connectivity** -- Make sure Muximux can reach the app's URL from its network. If Muximux runs in Docker, the app's hostname must be resolvable from inside the container.

---

## WebSocket Connection Fails

**Symptom:** Health status doesn't update in real-time. You see WebSocket errors in the browser console.

**Cause:** If Muximux is behind an external reverse proxy, that proxy may not be forwarding WebSocket connections correctly.

**Solution:** Configure your reverse proxy to support WebSocket upgrades:

- **Nginx:**
  ```nginx
  location / {
      proxy_pass http://muximux:8080;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";
  }
  ```

- **Traefik:** WebSocket support is automatic. No extra configuration needed.

- **Caddy:** WebSocket support is automatic. No extra configuration needed.

---

## Login Issues

### "Invalid state parameter" on OIDC login

- The OIDC state token has a 10-minute lifetime. If you waited too long on the login page, the token expired. Try logging in again.
- If this happens consistently, check that your server's system clock is accurate. OIDC relies on time-based token validation.

### Forward auth not working

- Verify that `trusted_proxies` in your configuration includes your reverse proxy's IP address or CIDR range.
- Check that your authentication proxy is sending the expected headers (`Remote-User`, `Remote-Email`, etc.).
- Muximux logs rejected forward auth attempts. Check the server logs to see why it rejects the requests.

### Session expires too quickly

- Increase `session_max_age` in your configuration. For example, set it to `7d` for 7-day sessions:

  ```yaml
  auth:
    session_max_age: 7d
  ```

- Muximux refreshes sessions on activity. Users who actively use Muximux should not be logged out unexpectedly.

---

## Port Conflicts

**Symptom:** Muximux fails to start with an "address already in use" error.

**Solutions:**

- Check what process is using the port:
  ```bash
  ss -tlnp | grep 8080
  ```

- Change `server.listen` to a different port in your configuration:
  ```yaml
  server:
    listen: ":9090"
  ```

- If you use auto-HTTPS (`tls.domain`), ports 80 and 443 must also be available. Caddy uses them for certificate issuance and HTTPS serving.

---

## Onboarding Wizard Doesn't Appear

The onboarding wizard opens only while the server reports setup as pending. Setup is pending on a new install where setup was never completed and no apps, users or auth method are configured. The wizard also opens only for an administrator. It never opens for a non-admin or on an established instance.

You may delete your `data/config.yaml` and still not see the wizard. In that case, verify that the server restarted and is serving the default (empty) config. Muximux resolves the `data/` directory relative to the binary's location, not the working directory. Make sure you're looking at the correct path. Check the server logs to confirm that the config loaded.

---

## Settings Shows Values I Did Not Set

A locked field shows "From VAR" or "Set by VAR". It gets its value from an environment variable or command-line flag, for example `MUXIMUX_LOG_LEVEL`, `MUXIMUX_LOG_FORMAT`, `MUXIMUX_LISTEN`, `MUXIMUX_BASE_PATH`, `MUXIMUX_DISCOVERY_AUTO_IMPORT`, `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE`, `MUXIMUX_AUTH_METHOD` or a `MUXIMUX_OIDC_*` variable. An OIDC field written in `config.yaml` as exactly `${VAR}` is locked the same way. The override applies in memory only. Muximux never writes it to `config.yaml`. To see the file's value, unset the variable and restart. The full list is in the [Environment Variable Reference](configuration.md#environment-variable-reference). Settings -> Discovery shows the stored values, including the auto-import mode. If Save is blocked with a name conflict, rename one of the two apps or groups with the same name.

---

## Muximux Refuses to Start After Setting Auth Variables

**Symptom:** The container exits at startup with `Failed to load configuration:` followed by a message naming a `MUXIMUX_AUTH_METHOD` or `MUXIMUX_OIDC_*` variable.

**Cause:** Authentication from the environment fails closed (it stops instead of running in an unsafe state). If a configuration could leave the instance open, or unable to sign anyone in, it stops startup instead. Common messages:

| Message contains | Fix |
|---|---|
| `is not a valid auth method` | Set `MUXIMUX_AUTH_METHOD` to `none`, `builtin`, `forward_auth` or `oidc`. |
| `set but MUXIMUX_AUTH_METHOD is not` | OIDC variables are set but the method is not OIDC. Add `MUXIMUX_AUTH_METHOD=oidc`, or remove the OIDC variables. |
| `MUXIMUX_OIDC_ENABLED=false contradicts auth method oidc` | Remove `MUXIMUX_OIDC_ENABLED` or choose another method. |
| `set either ... or ..._FILE, not both` | Keep only one of the two variables. |
| `_FILE: cannot read`, `is empty`, `is larger than` | The secret file is missing, unreadable by the container user (`PUID`), empty or too large. |
| `issuer_url is required`, `client_id is required`, `must be an absolute http(s) URL` | Set the named variable to a valid value. |

Some problems only log a warning at startup: invalid booleans, unknown `MUXIMUX_OIDC_*` names, and OIDC variables the chosen method does not use. See [Authentication > Configuring OIDC with environment variables](authentication.md#configuring-oidc-with-environment-variables).

---

## API Returns 503 "setup_required"

On a new install, most of the API (and `/ws` and `/proxy/`) waits until the onboarding wizard is finished. Complete setup with the [setup token](authentication.md#setup-token). Or, to skip the wizard, start with `MUXIMUX_AUTH_METHOD` set. See [API Reference > Authentication](api.md#authentication) for the endpoints that stay open.

---

## Docker App Missing: "Not loaded" in Settings -> Discovery

**Symptom:** An auto-imported app or gateway site is gone from the dashboard. The log says `Invalid Docker auto-imported entry quarantined`. Settings -> Discovery lists it under **Not loaded: invalid auto-imported entries**.

**Cause:** The entry in `config.yaml` fails validation. For example, an older version wrote it without a URL. Muximux does not refuse to start. Instead, it keeps the entry in the file but does not load it.

**Solution:** Fix the container's labels so the next refresh writes a valid entry. Or click **Remove** (or **Remove all**) to delete it from `config.yaml`. See [Docker Discovery > Quarantined entries](docker-discovery.md#quarantined-entries).

---

## Labelled Container Is Not Auto-Imported

Open the Discover dialog. Each container that auto-import skips shows a **Not importable** chip with the reason.

| Reason | Fix |
|---|---|
| No `muximux.*` labels | Add at least one label; `muximux.discovery.id` is enough. Unlabelled containers are never auto-imported. |
| `muximux.app.enabled=true is required` | Explicit opt-in is on (`require_explicit_enable` or `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE`); add the label. |
| No port | Add `muximux.app.port`. |
| No URL could be built | The network strategy cannot reach the container; see [Network strategies](docker-discovery.md#network-strategies). |
| Invalid app | The labels produce an invalid app; the detail says why. |

A container with `muximux.app.enabled=false` is opted out and hidden from the dialog. A container that is already tracked shows a **Tracked** chip instead. See [Why a container is skipped](docker-discovery.md#why-a-container-is-skipped).

---

## My Edit to a Docker App Keeps Reverting

While an app is tracked, a set `muximux.app.name`, `icon`, `group` or `order` label wins. If you edit that field in Settings, the next refresh puts the label's value back. This happens even for an app you imported by hand, and even with `auto_import: off`. To keep your edit, remove the label, or **Detach** the app in Settings -> Discovery -> Currently tracked. For groups, a different rule applies: if you edit the icon or colour of a Docker-created group in Settings, that hands the group back to you. See [Label re-sync for tracked apps](docker-discovery.md#label-re-sync-for-tracked-apps).

---

## Lucide Icon Categories Are Missing

**Symptom:** The Lucide tab of the icon picker lists icons but the category filter is empty, and the log shows `Lucide categories fetch failed; serving icon names without category metadata`.

**Cause:** Muximux fetches the category list from `https://lucide.dev/api/categories`. That request can fail (no outbound internet access, DNS, a firewall). Muximux then still serves the icon names, but without categories. It logs the warning once per process. Releases before 3.6.1 always failed here, because the upstream file they used was removed.

**Solution:**

1. Upgrade to 3.6.1 or later.
2. Allow Muximux outbound HTTPS to `lucide.dev`, `api.github.com` (icon list) and `cdn.jsdelivr.net` (icon files).
3. Restart Muximux to fetch the categories again. A restart is needed because Muximux keeps the icon list in memory for the icon cache lifetime (`icons.dashboard_icons.cache_ttl`, 7 days by default).

---

## Upgrading to 3.6

3.6.0 changed a few behaviours you may notice after upgrading:

- **Docker discovery:** auto-import only takes containers with a `muximux.*` label, Swarm and Compose tracking keys are migrated, set labels re-sync onto apps you imported by hand, and invalid auto-imported entries are quarantined. Details in [Docker Discovery > Upgrading to 3.6](docker-discovery.md#upgrading-to-36).
- **Saving from Settings** merges your edits onto the current server config, and Settings closes only after a successful save. API scripts that `PUT /api/config` without `base` keep the old replace behaviour; see [API Reference > Configuration](api.md#configuration).
- **Restore** goes through the startup load path and refuses a backup that still uses the legacy `server.gateway` setting, with a message explaining how to convert it. Listen address, TLS, base-path routing and session cookie settings still need a restart.
- **Themes:** a custom theme can no longer use a bundled theme's name, and colours were adjusted for WCAG AA contrast. Custom themes that omit the new tokens get derived values; see [Themes > Status and accent tokens](themes.md#status-and-accent-tokens).
- **`lifecycle_allowed_groups`** holds user and identity-provider group names, not dashboard groups.

3.6.1 shows the Docker whale to admins only, applies environment overrides on a first start without `config.yaml`, and ignores `muximux.app.health` values starting with `//` and bare `muximux.app.icon` values containing `:`, `/` or spaces.

---

## Config Changes Not Taking Effect

Different settings have different reload behaviors:

| Setting Category | Restart Required? |
|-----------------|-------------------|
| Navigation, themes, apps, groups, icons, keybindings | No -- changes via Settings panel are immediate |
| Health monitoring settings | No -- applied immediately |
| Server settings (listen, TLS domain, base_path) | Yes -- restart required |
| Gateway sites (`gateway_sites`) | No -- Caddy reloads in place |
| Auth method (via Settings or API) | No -- applied immediately |

If you edit `config.yaml` directly (not through the Settings panel), Muximux picks up the changes on the next restart.

---

## Docker: Permission Denied on Data Directory

**Symptom:** Muximux can't write to `/app/data` inside the Docker container, or you see "permission denied" errors in the logs.

**Cause:** The Muximux container runs as UID 1000 by default. If your mounted data directory is owned by a different user, the container cannot write to it.

**Solution:** Set the correct ownership on your data directory:

```bash
sudo chown -R 1000:1000 ./data
```

Or, to use a different UID, set `PUID` and `PGID` in your `docker-compose.yml`. Don't use `user:` for this. The entrypoint has to start as root to create the user and fix ownership of `/app/data`. Only then does it drop privileges. If you drop capabilities (`cap_drop: ALL`), add back `CHOWN`, `SETUID` and `SETGID`, as the example `docker-compose.yml` does. Without them, the container exits with `su-exec: setgroups: Operation not permitted`.

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    environment:
      - PUID=1000
      - PGID=1000
    volumes:
      - ./data:/app/data
```

---

## Docker Discovery Finds Nothing / Lifecycle Buttons Fail

**Symptom:** The Discover modal is empty, auto-import imports nothing, or the
start/stop/restart buttons return an error.

**Cause:** Muximux can't reach the Docker socket.

**Solutions:**
1. Mount the socket:
   `- /var/run/docker.sock:/var/run/docker.sock:ro`
   (`:ro` doesn't limit Docker API calls; see
   [Docker Discovery](docker-discovery.md#make-the-daemon-socket-reachable-from-muximux).)
   The container entrypoint auto-detects the socket's group. It then gives the
   runtime user access. You don't need `group_add`, root, or a custom entrypoint.
2. For lifecycle controls (start/stop/restart), set up two more things. You need
   both:
   - a daemon that accepts write requests (a plain socket mount does; a socket
     proxy must allow `POST`);
   - `discovery.docker.lifecycle_enabled: true`.

   Muximux limits these controls by role and writes each use to the audit log.
3. Confirm that discovery is enabled (`discovery.docker.enabled: true`).
4. Check the server logs for socket connection errors on startup.
5. For rootless Docker, a docker-socket-proxy sidecar, or a custom socket
   path, set the `DOCKER_SOCKET` and `DOCKER_GID` override env vars (see
   [Docker Discovery](docker-discovery.md#make-the-daemon-socket-reachable-from-muximux)).

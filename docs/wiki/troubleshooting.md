# Troubleshooting

> **Tip:** Enable [debug logging](debug-logging.md) to get detailed diagnostic information. Add `?debug=true` to your URL for frontend console logs, or set log level to `debug` in Settings for backend request-level detail.

---

## App Won't Load in Iframe

**Symptom:** Blank iframe, "refused to connect" error, or security error in browser console.

**Cause:** Many web applications send HTTP headers (`X-Frame-Options`, `Content-Security-Policy`) that prevent them from being embedded in iframes.

**Solutions:**

1. **Enable the proxy** -- Set `proxy: true` on the app. Muximux's built-in reverse proxy strips the headers that block iframe embedding.

2. **Debug the proxy** -- If proxy mode doesn't help, the app may use JavaScript-based iframe detection. Try accessing `/proxy/{slug}/` directly in a new browser tab to see what the app looks like through the proxy.

3. **Use a different open mode** -- As a last resort, set `open_mode: new_tab` so the app opens in a separate browser tab instead of an iframe.

---

## App Loads But Looks Broken (CSS/JS Missing)

**Symptom:** The app loads in the iframe but styles are missing, images don't appear, or there are JavaScript errors in the browser console.

**Cause:** The app's internal URLs for CSS, JavaScript, and images may not resolve correctly when served through the reverse proxy.

**Solutions:**

1. Open browser DevTools (F12) and check the **Network** tab for 404 errors on asset files.

2. If the app has a "base URL", "URL base", or "path prefix" setting in its own configuration, try setting it to `/proxy/{slug}` where `{slug}` is the lowercase, hyphenated version of your app name.

3. Some apps require specific configuration to work behind a reverse proxy. Check the app's own documentation for reverse proxy setup instructions.

---

## Reverse Proxy Limitations

The built-in reverse proxy rewrites paths in HTML, CSS, JavaScript, JSON, and HTTP headers so that apps work at `/proxy/{slug}/` instead of their original URL. However, some patterns cannot be rewritten. This section explains what is **not supported**, why, and what you can do about it.

### JavaScript That Builds URLs at Runtime

Muximux includes a runtime URL interceptor that handles most of these cases automatically. The interceptor patches `fetch()`, `XMLHttpRequest`, `WebSocket`, `EventSource`, and DOM property setters (`img.src`, etc.) so that dynamically constructed URLs are rewritten before the browser sends them. This section describes the remaining edge cases.

**Symptom:** The app loads but some features, API calls, or navigation links point to wrong paths. You see 404 errors in the Network tab for URLs that are missing the `/proxy/{slug}/` prefix.

**Patterns that may still cause issues:**

| Pattern | Example | Why |
|---------|---------|-----|
| `history.pushState` | `pushState({}, '', '/page')` | Client-side navigation bypasses the server |
| `history.replaceState` | `replaceState({}, '', '/page')` | Same as above |
| `location.pathname` checks | `if (location.pathname === '/login')` | Reads the current path which includes the proxy prefix |
| Inline event handlers | `<div onclick="go('/page')">` | URL in HTML attribute, not intercepted by runtime patches |

**Workaround:** If the app supports a "base URL", "URL base", or "path prefix" setting in its own configuration, set it to `/proxy/{slug}`. This tells the app to prepend the correct prefix when building URLs, solving the problem at the source. Many popular apps support this (Sonarr, Radarr, Prowlarr, Lidarr, Bazarr, Seerr, Tautulli, etc.).

### Single-Page App (SPA) Routing Issues

**Symptom:** The app loads initially but navigating within it causes a blank page, a 404, or routes back to the home page.

**What happens:** SPAs define client-side routes like `/dashboard` or `/users/settings`. When the app is proxied at `/proxy/my-app/`, the browser's URL becomes `/proxy/my-app/dashboard`. The app's router sees the full path including the `/proxy/my-app/` prefix, which it doesn't recognize, causing a route mismatch.

The proxy mitigates this by rewriting base path configuration variables (e.g., `urlBase: ""` becomes `urlBase: "/proxy/my-app"`), which helps many apps. However, apps that hardcode routes in their JavaScript or use a routing framework that doesn't respect the base path may still break.

**Workaround:** Configure the app's base URL/path prefix setting to `/proxy/{slug}` if available. This is the most reliable fix because it makes the app aware of its actual path.

### Service Workers

**Symptom:** The app works on first load but breaks after a page refresh, or cached content appears stale, or the app tries to serve offline content at wrong paths.

**What happens:** Service workers register under the page's origin and intercept network requests. When an app is proxied through Muximux, its service worker would register under Muximux's origin, potentially intercepting unrelated requests.

**Mitigation:** Since v3.0.9, Muximux's proxy interceptor automatically blocks `navigator.serviceWorker.register()` calls from proxied apps and cleans up any previously leaked registrations. If you still experience stale cache issues from a service worker registered before upgrading, clear it manually: open DevTools > Application > Service Workers > Unregister.

### Binary and Non-Text Protocols

**Symptom:** Features that use gRPC, Protocol Buffers, MessagePack, or other binary protocols fail when the app is proxied.

**What happens:** The proxy rewrites text-based content by matching patterns in the response body. Binary protocols encode data in non-text formats where path strings cannot be found or safely modified by regex-based rewriting.

**Workaround:** No proxy-side fix is possible. Use `open_mode: new_tab` for apps that rely on binary protocols, or configure the app to use its non-binary API if one exists.

### Large Text Responses and Memory

**Symptom:** Muximux uses more memory than expected when a proxied app serves very large text responses (e.g., a JSON API endpoint that returns hundreds of megabytes of data).

**What happens:** The proxy buffers text-based responses (HTML, CSS, JavaScript, JSON, XML) in memory so it can rewrite URLs in the content. For unusually large text responses this can cause memory pressure.

Binary content (images, videos, archives, file downloads) is **not** buffered - it streams directly from the backend to your browser without being read into memory. Only text content types are buffered for rewriting.

### Cookie Domain Attribute

**Symptom:** Login sessions in a proxied app don't persist, or you need to log in repeatedly.

**What happens:** The proxy rewrites the `Path` attribute of `Set-Cookie` headers so cookies are scoped to the proxy path. However, the `Domain` attribute is **not** rewritten. If the app sets a cookie with an explicit domain (e.g., `Domain=app.internal`), the browser may reject it because the cookie domain doesn't match the Muximux domain.

**Workaround:** Configure the app to not set an explicit cookie domain, if possible. Most apps that are designed for reverse proxy use will work correctly without an explicit domain.

### Strict Origin Validation

**Symptom:** Proxied app rejects requests with CSRF errors, "invalid origin", or "forbidden" responses when submitting forms or making API calls.

**What happens:** Some apps validate the `Origin` or `Referer` HTTP header to prevent cross-site request forgery (CSRF). When accessed through the proxy, these headers contain Muximux's hostname instead of the app's hostname, causing the app to reject the request.

**Workaround:** Check the app's settings for a "trusted origins" or "CORS allowed origins" option and add Muximux's URL. Some apps also have a "disable CSRF" or "allow reverse proxy" toggle.

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
- Muximux logs rejected forward auth attempts. Check server logs for details on why requests are being rejected.

### Session expires too quickly

- Increase `session_max_age` in your configuration. For example, set it to `7d` for 7-day sessions:

  ```yaml
  auth:
    session_max_age: 7d
  ```

- Sessions are refreshed on activity, so users who are actively using Muximux should not be logged out unexpectedly.

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

- If you are using auto-HTTPS (`tls.domain`), ports 80 and 443 must also be available. These are used by Caddy for certificate issuance and HTTPS serving.

---

## Onboarding Wizard Doesn't Appear

The onboarding wizard opens only while the server reports setup as pending (a new install where setup was never completed and no apps, users or auth method are configured), and only for an administrator. It never opens for a non-admin or on an established instance.

If you deleted your `data/config.yaml` but the wizard still doesn't appear, verify that the server restarted and is serving the default (empty) config. The `data/` directory is resolved relative to the binary's location (not the working directory), so make sure you're looking at the correct path. Check the server logs to confirm config loading.

---

## Settings Shows Values I Did Not Set

A field shown locked ("From VAR" or "Set by VAR") gets its value from an environment variable or command-line flag, for example `MUXIMUX_LOG_LEVEL`, `MUXIMUX_LOG_FORMAT`, `MUXIMUX_LISTEN`, `MUXIMUX_BASE_PATH`, `MUXIMUX_DISCOVERY_AUTO_IMPORT`, `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE`, `MUXIMUX_AUTH_METHOD` or a `MUXIMUX_OIDC_*` variable. An OIDC field written in `config.yaml` as exactly `${VAR}` is locked the same way. The override applies in memory only and is never written to `config.yaml`; unset the variable and restart to see the file's value. The full list is in the [Environment Variable Reference](configuration.md#environment-variable-reference). Settings -> Discovery shows the stored values, including the auto-import mode. If Save is blocked with a name conflict, rename one of the two apps or groups with the same name.

---

## Muximux Refuses to Start After Setting Auth Variables

**Symptom:** The container exits at startup with `Failed to load configuration:` followed by a message naming a `MUXIMUX_AUTH_METHOD` or `MUXIMUX_OIDC_*` variable.

**Cause:** Authentication from the environment fails closed: a configuration that could leave the instance open, or unable to sign anyone in, stops startup instead. Common messages:

| Message contains | Fix |
|---|---|
| `is not a valid auth method` | Set `MUXIMUX_AUTH_METHOD` to `none`, `builtin`, `forward_auth` or `oidc`. |
| `set but MUXIMUX_AUTH_METHOD is not` | OIDC variables are set but the method is not OIDC. Add `MUXIMUX_AUTH_METHOD=oidc`, or remove the OIDC variables. |
| `MUXIMUX_OIDC_ENABLED=false contradicts auth method oidc` | Remove `MUXIMUX_OIDC_ENABLED` or choose another method. |
| `set either ... or ..._FILE, not both` | Keep only one of the two variables. |
| `_FILE: cannot read`, `is empty`, `is larger than` | The secret file is missing, unreadable by the container user (`PUID`), empty or too large. |
| `issuer_url is required`, `client_id is required`, `must be an absolute http(s) URL` | Set the named variable to a valid value. |

Invalid booleans, unknown `MUXIMUX_OIDC_*` names and OIDC variables the chosen method does not use only log a warning at startup. See [Authentication > Configuring OIDC with environment variables](authentication.md#configuring-oidc-with-environment-variables).

---

## API Returns 503 "setup_required"

On a new install, most of the API (and `/ws` and `/proxy/`) waits until the onboarding wizard is finished. Complete setup with the [setup token](authentication.md#setup-token), or start with `MUXIMUX_AUTH_METHOD` set to skip the wizard. See [API Reference > Authentication](api.md#authentication) for the endpoints that stay open.

---

## Docker App Missing: "Not loaded" in Settings -> Discovery

**Symptom:** An auto-imported app or gateway site is gone from the dashboard, the log says `Invalid Docker auto-imported entry quarantined`, and Settings -> Discovery lists it under **Not loaded: invalid auto-imported entries**.

**Cause:** The entry in `config.yaml` fails validation (for example it was written without a URL by an older version). Instead of refusing to start, Muximux keeps it in the file but does not load it.

**Solution:** Fix the container's labels so the next refresh writes a valid entry, or click **Remove** (or **Remove all**) to delete it from `config.yaml`. See [Docker Discovery > Quarantined entries](docker-discovery.md#quarantined-entries).

---

## Labelled Container Is Not Auto-Imported

Open the Discover dialog: a container auto-import skips shows a **Not importable** chip with the reason.

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

While an app is tracked, a set `muximux.app.name`, `icon`, `group` or `order` label wins: an edit to that field in Settings is put back on the next refresh, even for an app you imported by hand and with `auto_import: off`. Remove the label, or **Detach** the app in Settings -> Discovery -> Currently tracked, to keep your edit. For groups, editing the icon or colour of a Docker-created group in Settings hands it back to you. See [Label re-sync for tracked apps](docker-discovery.md#label-re-sync-for-tracked-apps).

---

## Lucide Icon Categories Are Missing

**Symptom:** The Lucide tab of the icon picker lists icons but the category filter is empty, and the log shows `Lucide categories fetch failed; serving icon names without category metadata`.

**Cause:** Muximux fetches the category list from `https://lucide.dev/api/categories`. When that request fails (no outbound internet access, DNS, a firewall), icon names are still served without categories, and the warning is logged once per process. Releases before 3.6.1 always failed here because the upstream file they used was removed.

**Solution:** Upgrade to 3.6.1 or later and allow Muximux outbound HTTPS to `lucide.dev`, `api.github.com` (icon list) and `cdn.jsdelivr.net` (icon files). The icon list is kept in memory for the icon cache lifetime (`icons.dashboard_icons.cache_ttl`, 7 days by default), so restart Muximux after fixing network access to fetch the categories again.

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

If you edited `config.yaml` directly (not through the Settings panel), Muximux will pick up the changes on the next restart.

---

## Docker: Permission Denied on Data Directory

**Symptom:** Muximux can't write to `/app/data` inside the Docker container, or you see "permission denied" errors in the logs.

**Cause:** The Muximux container runs as UID 1000 by default. If your mounted data directory is owned by a different user, the container cannot write to it.

**Solution:** Set the correct ownership on your data directory:

```bash
sudo chown -R 1000:1000 ./data
```

Or, if you prefer to use a different UID, you can set it in your `docker-compose.yml`:

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    user: "1000:1000"
    volumes:
      - ./data:/app/data
```

---

## Docker Discovery Finds Nothing / Lifecycle Buttons Fail

**Symptom:** The Discover modal is empty, auto-import imports nothing, or the
start/stop/restart buttons return an error.

**Cause:** Muximux can't reach the Docker socket.

**Solutions:**
1. Mount the socket. Discovery and auto-import need it read-only:
   `- /var/run/docker.sock:/var/run/docker.sock:ro`
   The container entrypoint auto-detects the socket's group and grants the
   runtime user access -- no `group_add`, root, or custom entrypoint needed.
2. Lifecycle controls (start/stop/restart) additionally require write access
   (`:rw`) AND `discovery.docker.lifecycle_enabled: true`. They are role-gated
   and audit-logged.
3. Confirm discovery is enabled (`discovery.docker.enabled: true`) and check
   the server logs for socket connection errors on startup.
4. For rootless Docker, a docker-socket-proxy sidecar, or a custom socket
   path, set the `DOCKER_SOCKET` and `DOCKER_GID` override env vars (see
   [Docker Discovery](docker-discovery.md#make-the-daemon-socket-reachable-from-muximux)).

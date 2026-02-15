# Troubleshooting

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

**Symptom:** The app loads but some features, API calls, or navigation links point to wrong paths. You see 404 errors in the Network tab for URLs that are missing the `/proxy/{slug}/` prefix.

**What happens:** The proxy rewrites paths that appear as literal strings in the response body (e.g., `fetch("/api/data")`). However, when JavaScript constructs URLs at runtime by concatenating variables, using template literals, or calling `new URL()`, the final URL only exists in the browser's memory -- the proxy never sees it.

**Patterns that cannot be rewritten:**

| Pattern | Example | Why |
|---------|---------|-----|
| Template literals | `` `${base}/api/users` `` | The `base` variable is resolved by the browser |
| String concatenation | `'/api' + '/users'` | Two separate strings joined at runtime |
| `new URL()` | `new URL('/api', location.origin)` | URL constructed from parts at runtime |
| `history.pushState` | `pushState({}, '', '/page')` | Client-side navigation bypasses the server |
| `history.replaceState` | `replaceState({}, '', '/page')` | Same as above |
| `location.pathname` | `if (location.pathname === '/login')` | Reads the current path which includes the proxy prefix |

**Workaround:** If the app supports a "base URL", "URL base", or "path prefix" setting in its own configuration, set it to `/proxy/{slug}`. This tells the app to prepend the correct prefix when building URLs, solving the problem at the source. Many popular apps support this (Sonarr, Radarr, Prowlarr, Lidarr, Bazarr, Overseerr, Tautulli, etc.).

### Single-Page App (SPA) Routing Issues

**Symptom:** The app loads initially but navigating within it causes a blank page, a 404, or routes back to the home page.

**What happens:** SPAs define client-side routes like `/dashboard` or `/users/settings`. When the app is proxied at `/proxy/my-app/`, the browser's URL becomes `/proxy/my-app/dashboard`. The app's router sees the full path including the `/proxy/my-app/` prefix, which it doesn't recognize, causing a route mismatch.

The proxy mitigates this by rewriting base path configuration variables (e.g., `urlBase: ""` becomes `urlBase: "/proxy/my-app"`), which helps many apps. However, apps that hardcode routes in their JavaScript or use a routing framework that doesn't respect the base path may still break.

**Workaround:** Configure the app's base URL/path prefix setting to `/proxy/{slug}` if available. This is the most reliable fix because it makes the app aware of its actual path.

### Service Workers

**Symptom:** The app works on first load but breaks after a page refresh, or cached content appears stale, or the app tries to serve offline content at wrong paths.

**What happens:** Service workers intercept network requests and serve cached responses. When an app is proxied, the service worker may cache responses under paths that don't include the proxy prefix, or its scope may not cover the `/proxy/{slug}/` path correctly. The proxy cannot modify service worker behavior after it has been registered in the browser.

**Workaround:** If the app has a service worker toggle, try disabling it. Otherwise, clear the service worker from your browser: open DevTools > Application > Service Workers > Unregister. If the problem persists, use `open_mode: new_tab` instead.

### Binary and Non-Text Protocols

**Symptom:** Features that use gRPC, Protocol Buffers, MessagePack, or other binary protocols fail when the app is proxied.

**What happens:** The proxy rewrites text-based content by matching patterns in the response body. Binary protocols encode data in non-text formats where path strings cannot be found or safely modified by regex-based rewriting.

**Workaround:** No proxy-side fix is possible. Use `open_mode: new_tab` for apps that rely on binary protocols, or configure the app to use its non-binary API if one exists.

### Large Responses and Memory

**Symptom:** Muximux becomes slow or unresponsive when proxied apps serve very large responses (downloads, database exports, large media files).

**What happens:** The proxy buffers the entire response body in memory to perform rewriting. For very large responses (hundreds of megabytes or more), this can cause significant memory pressure on the server.

**Workaround:** Avoid proxying apps that serve large file downloads. Instead, access those apps directly via `open_mode: new_tab`, or use a custom `health_url` and access the download page directly. Binary content types like images, videos, and archives are not rewritten by the proxy, so they pass through with minimal overhead -- but the buffering still occurs.

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

The onboarding wizard appears automatically whenever no apps are configured (the `apps` list in config.yaml is empty or absent).

If you deleted your `data/config.yaml` but the wizard still doesn't appear, verify that the server restarted and is serving the default (empty) config. Check the server logs to confirm config loading.

---

## Config Changes Not Taking Effect

Different settings have different reload behaviors:

| Setting Category | Restart Required? |
|-----------------|-------------------|
| Navigation, themes, apps, groups, icons, keybindings | No -- changes via Settings panel are immediate |
| Health monitoring settings | No -- applied immediately |
| Server settings (listen, TLS, gateway) | Yes -- restart required |
| Auth method changes | Yes -- restart required |

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
    image: muximux
    user: "1000:1000"
    volumes:
      - ./data:/app/data
```

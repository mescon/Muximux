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

If you deleted your config.yaml but the wizard still doesn't appear, verify that the server restarted and is serving the default (empty) config. Check the server logs to confirm config loading.

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

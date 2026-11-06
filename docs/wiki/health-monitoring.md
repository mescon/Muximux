# Health Monitoring

## Overview

Muximux periodically checks whether your apps are reachable and displays their health status in the navigation, so you can see at a glance which services are up and which are down.

## Configuration

Health monitoring is configured in the top-level `health` section of `config.yaml`:

```yaml
health:
  enabled: true     # Enable/disable health monitoring (default: true)
  interval: 30s     # How often to check each app (default: 30s)
  timeout: 5s       # HTTP request timeout per check (default: 5s)
```

Set `enabled: false` to turn off health monitoring entirely. When it is off, all apps show a gray (unknown) status indicator.

## How It Works

- Muximux sends an HTTP GET request to each app that has `health_check: true`. It uses `health_url` if configured, otherwise the main `url`.
- A final response with HTTP status 2xx or 3xx counts as **healthy**. Muximux follows redirects automatically, up to 3 hops. If more redirects remain after the limit, Muximux uses the last 3xx response, and it still counts as healthy. Timeouts, connection errors, and 4xx/5xx responses count as **unhealthy**. Muximux ignores TLS certificate errors, so self-signed certs work fine.
- Muximux sends the results to all connected browsers in real time over WebSocket (a connection that stays open so the server can push updates).

## Custom Health URLs

Some apps have dedicated health or status endpoints that respond faster and don't require authentication. You can point Muximux at these instead of the main URL:

```yaml
apps:
  - name: Plex
    url: http://plex:32400/web
    health_url: http://plex:32400/identity   # Lightweight endpoint
```

If `health_url` is not set, the main `url` is used for health checks.

Docker-discovered apps with a `muximux.app.url` label get their health address from the container automatically. Muximux keeps that address current when the container's IP changes. See [Docker Discovery](docker-discovery.md#running-behind-your-own-reverse-proxy).

**Tip:** Many self-hosted apps expose endpoints like `/api/health`, `/ping`, `/status`, or `/identity` that are fast and don't require login. Check your app's documentation for available endpoints.

## Per-App Health Check Toggle

Health checks are **opt-in** per app. Enable them by setting `health_check: true`:

```yaml
apps:
  - name: Sonarr
    url: http://sonarr:8989
    health_check: true     # Enable health checks for this app

  - name: External Service
    url: https://external-service.example.com
    # health_check omitted → no health checks, no status indicator
```

When `health_check` is not set or is `false`:
- No health check requests are sent for that app.
- No health status indicator (dot) is shown in the navigation.
- The app always appears at full opacity regardless of other apps' health status.

By default, **no apps have health checks enabled**. You enable them one by one for the apps you care about. Or use the bulk toggle in **Settings > General > Health Checks** to turn all of them on or off at once.

This opt-in design keeps things quiet by default and lets you choose which apps to monitor. This helps when you have external services, VPN-only apps, or services behind firewalls that Muximux can't reach.

## Health Status Indicators

In the navigation, each app icon shows a small colored dot indicating its current health:

- **Green** -- Healthy (HTTP 2xx or 3xx response)
- **Red** -- Unhealthy (error, timeout, or 4xx/5xx response)
- **Gray** -- Unknown (not yet checked or health monitoring disabled)

## Health Data

Each app tracks the following health information:

- **Current status** -- healthy, unhealthy, or unknown
- **Response time** -- how long the health check took, in milliseconds
- **Last check** -- timestamp of the most recent health check
- **Uptime percentage** -- successful checks divided by total checks
- **Last error message** -- the reason for the most recent failure (if unhealthy)

This data is available through the API (see [API Reference](api.md)) and is displayed in the Settings panel.

## Manual Health Check

You can trigger an immediate health check for any app via the API:

```
POST /api/apps/{name}/health/check
```

This skips the regular interval and checks the app right away. Muximux sends the result to all connected browsers.

## Real-time Updates

Muximux pushes health status changes to your browser over WebSocket. Your dashboard updates instantly when an app goes up or down. You need no polling or page refresh.

If the WebSocket connection drops (for example, due to a network interruption), the client reconnects automatically. It uses exponential backoff: it waits longer after each failed attempt.

## Common Issues

If health checks show unexpected results, see the [Troubleshooting](troubleshooting.md) page. It has solutions to common problems, such as apps that show unhealthy when they work fine, or health status that does not update in real time.

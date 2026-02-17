# Health Monitoring

## Overview

Muximux periodically checks whether your apps are reachable and displays their health status in the navigation. This gives you an at-a-glance view of which services are up and which are down.

## Configuration

Health monitoring is configured in the top-level `health` section of `config.yaml`:

```yaml
health:
  enabled: true     # Enable/disable health monitoring (default: true)
  interval: 30s     # How often to check each app (default: 30s)
  timeout: 5s       # HTTP request timeout per check (default: 5s)
```

Set `enabled: false` to disable health monitoring entirely. When disabled, all apps show a gray (unknown) status indicator.

## How It Works

- Muximux sends an HTTP GET request to each enabled app's URL (or `health_url` if configured).
- A response with HTTP status 2xx is considered **healthy**. Anything else -- including timeouts, connection errors, and non-2xx status codes -- is considered **unhealthy**.
- Results are broadcast to all connected browsers via WebSocket in real-time.

## Custom Health URLs

Some apps have dedicated health or status endpoints that respond faster and don't require authentication. You can point Muximux at these instead of the main URL:

```yaml
apps:
  - name: Plex
    url: http://plex:32400/web
    health_url: http://plex:32400/identity   # Lightweight endpoint
```

If `health_url` is not set, the main `url` is used for health checks.

**Tip:** Many self-hosted apps expose endpoints like `/api/health`, `/ping`, `/status`, or `/identity` that are fast and don't require login. Check your app's documentation for available endpoints.

## Per-App Health Check Toggle

You can disable health monitoring for individual apps by setting `health_check: false`:

```yaml
apps:
  - name: External Service
    url: https://external-service.example.com
    health_check: false    # No health checks, no status indicator shown
```

When `health_check` is set to `false`:
- No health check requests are sent for that app.
- No health status indicator (dot) is shown in the navigation.
- The app always appears at full opacity regardless of other apps' health status.

This is useful for:
- External services you don't control and don't want to ping.
- Apps behind VPNs or firewalls that Muximux can't reach for health checks.
- Reducing noise when you only care about the health of certain critical services.

By default, all apps have health checks enabled (equivalent to `health_check: true`). You can bulk-enable or bulk-disable health checks for all apps in **Settings > Apps**.

## Health Status Indicators

In the navigation, each app icon shows a small colored dot indicating its current health:

- **Green** -- Healthy (HTTP 2xx response)
- **Red** -- Unhealthy (error, timeout, or non-2xx response)
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

This bypasses the regular interval and checks the app right away. The result is broadcast to all connected browsers.

## Real-time Updates

Health status changes are pushed to your browser via WebSocket. Your dashboard updates instantly when an app goes up or down -- no polling or page refresh needed.

If the WebSocket connection drops (for example, due to a network interruption), the client automatically reconnects with exponential backoff.

## Common Issues

If health checks are showing unexpected results, see the [Troubleshooting](troubleshooting.md) page for solutions to common problems like apps showing unhealthy when they work fine, or health status not updating in real-time.

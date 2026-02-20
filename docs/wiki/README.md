# Muximux Wiki

Muximux is a modern, self-hosted web application portal for your homelab. It runs as a single binary, serves on a single port, and stores all configuration in one YAML file. Add your self-hosted applications, organize them into groups, and access them from a unified dashboard with health monitoring, keyboard shortcuts, and a built-in reverse proxy.

![Muximux dashboard](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/09-dashboard-dark.png)

## How You Can Use Muximux

Muximux is designed to fit your setup, from a simple dashboard to a full reverse proxy appliance.

**Dashboard only** -- Run Muximux behind your existing reverse proxy (Traefik, nginx, Caddy, etc.) with `auth: none` and let your proxy handle TLS and authentication. Apps open via their direct URLs or in iframes. This is the simplest setup.

**Dashboard + built-in reverse proxy** -- Same as above, but enable `proxy: true` on apps that don't work in iframes. Muximux proxies those apps through `/proxy/{slug}/`, stripping iframe-blocking headers and rewriting paths. This works in all deployment modes -- no extra configuration needed.

**Full reverse proxy appliance** -- Use Muximux as your only reverse proxy. Configure `tls.domain` for automatic HTTPS and a `gateway` Caddyfile to serve your other services on their own domains. Caddy handles TLS certificates, HTTP-to-HTTPS redirects, and routing -- all from one binary. See [TLS & HTTPS](tls-and-gateway.md) for a full walkthrough.

## Table of Contents

- [Installation](installation.md) -- Docker, binary, and building from source
- [Getting Started](getting-started.md) -- First launch, onboarding wizard, and initial setup
- [Configuration Reference](configuration.md) -- Full config.yaml format and all available options
- [Apps](apps.md) -- Adding, configuring, and managing applications
- [Built-in Reverse Proxy](reverse-proxy.md) -- Proxying app traffic through Muximux
- [Authentication](authentication.md) -- Built-in auth, forward auth, and OIDC
- [TLS & HTTPS](tls-and-gateway.md) -- Automatic certificates, custom certificates, and gateway mode
- [Gateway Examples](gateway-examples.md) -- Recipes for proxying common homelab services
- [Navigation & Layout](navigation.md) -- Sidebar positions, auto-hide, labels, and display options
- [Themes](themes.md) -- Built-in themes, custom themes, and CSS custom properties
- [Health Monitoring](health-monitoring.md) -- Opt-in health checks and status indicators
- [Keyboard Shortcuts](keyboard-shortcuts.md) -- Default shortcuts and custom keybindings
- [Icons](icons.md) -- Dashboard Icons, Lucide icons, custom icons, and caching
- [Deployment Guide](deployment.md) -- Production deployment, reverse proxies, and networking
- [Troubleshooting](troubleshooting.md) -- Common issues and solutions
- [API Reference](api.md) -- REST API endpoints for programmatic access

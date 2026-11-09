<p align="center">
  <img src="web/public/muximux-logo.svg" width="200" alt="Muximux">
</p>

# Muximux

A self-hosted homelab dashboard with an optional built-in embedding proxy that makes stubborn apps work in iframes.

One binary, one port, one YAML config file.

[![CI](https://github.com/mescon/Muximux/actions/workflows/ci.yml/badge.svg)](https://github.com/mescon/Muximux/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mescon/Muximux/actions/workflows/codeql.yml/badge.svg)](https://github.com/mescon/Muximux/actions/workflows/codeql.yml)
[![Quality Gate](https://sonarcloud.io/api/project_badges/measure?project=mescon_Muximux&metric=alert_status)](https://sonarcloud.io/summary/overall?id=mescon_Muximux)
[![Maintainability](https://sonarcloud.io/api/project_badges/measure?project=mescon_Muximux&metric=sqale_rating)](https://sonarcloud.io/summary/overall?id=mescon_Muximux)
[![Security](https://sonarcloud.io/api/project_badges/measure?project=mescon_Muximux&metric=security_rating)](https://sonarcloud.io/summary/overall?id=mescon_Muximux)
[![Codecov](https://codecov.io/gh/mescon/Muximux/branch/main/graph/badge.svg)](https://codecov.io/gh/mescon/Muximux)
[![OWASP ASVS](https://img.shields.io/badge/OWASP_ASVS-Level_2-005571?logo=owasp)](docs/wiki/security.md)
[![Docker](https://img.shields.io/badge/ghcr.io-mescon%2Fmuximux-blue?logo=docker)](https://ghcr.io/mescon/muximux)
[![Release](https://img.shields.io/github/v/release/mescon/Muximux)](https://github.com/mescon/Muximux/releases/latest)
![License](https://img.shields.io/badge/License-GPL%20v2-blue)

> **AI Disclosure:** Muximux is developed with significant AI assistance (Claude Code). All code is reviewed, tested, and approved by the maintainer before shipping. See [Development](#development) for details.

![Muximux tour](docs/muximux-tour.gif)

*The dashboard, Plex running inside a pane (it refuses to be framed without the proxy), split view, and themes.*

![Muximux Dashboard](docs/screenshots/09-dashboard-dark.png)

<details>
<summary><strong>More screenshots</strong>: onboarding wizard, light mode, themes, split view, command palette</summary>
<br>
<table>
<tr>
<td align="center"><a href="docs/screenshots/10-dashboard-light.png"><img src="docs/screenshots/10-dashboard-light.png" width="280" alt="Dashboard (Light)"></a><br><sub>Dashboard (Light)</sub></td>
<td align="center"><a href="docs/screenshots/01-welcome.png"><img src="docs/screenshots/01-welcome.png" width="280" alt="Onboarding - Welcome"></a><br><sub>Onboarding - Welcome</sub></td>
<td align="center"><a href="docs/screenshots/02-security.png"><img src="docs/screenshots/02-security.png" width="280" alt="Onboarding - Security"></a><br><sub>Onboarding - Security</sub></td>
</tr>
<tr>
<td align="center"><a href="docs/screenshots/03-apps-catalog.png"><img src="docs/screenshots/03-apps-catalog.png" width="280" alt="Onboarding - App Catalog"></a><br><sub>Onboarding - App Catalog</sub></td>
<td align="center"><a href="docs/screenshots/04-apps-selected.png"><img src="docs/screenshots/04-apps-selected.png" width="280" alt="Onboarding - Apps Configured"></a><br><sub>Onboarding - Apps Configured</sub></td>
<td align="center"><a href="docs/screenshots/05-style.png"><img src="docs/screenshots/05-style.png" width="280" alt="Onboarding - Style"></a><br><sub>Onboarding - Style</sub></td>
</tr>
<tr>
<td align="center"><a href="docs/screenshots/06-theme-dark.png"><img src="docs/screenshots/06-theme-dark.png" width="280" alt="Onboarding - Themes (Dark)"></a><br><sub>Onboarding - Themes (Dark)</sub></td>
<td align="center"><a href="docs/screenshots/07-theme-light.png"><img src="docs/screenshots/07-theme-light.png" width="280" alt="Onboarding - Themes (Light)"></a><br><sub>Onboarding - Themes (Light)</sub></td>
<td align="center"><a href="docs/screenshots/08-done.png"><img src="docs/screenshots/08-done.png" width="280" alt="Onboarding - Done"></a><br><sub>Onboarding - Done</sub></td>
</tr>
<tr>
<td align="center"><a href="docs/screenshots/13-logs.png"><img src="docs/screenshots/13-logs.png" width="280" alt="Real-Time Log Viewer"></a><br><sub>Real-Time Log Viewer</sub></td>
<td align="center"><a href="docs/screenshots/11-themes.png"><img src="docs/screenshots/11-themes.png" width="280" alt="Theme Selector"></a><br><sub>Theme Selector</sub></td>
<td align="center"><a href="docs/screenshots/12-theme-customizer.png"><img src="docs/screenshots/12-theme-customizer.png" width="280" alt="Theme Customizer"></a><br><sub>Theme Customizer</sub></td>
</tr>
<tr>
<td align="center"><a href="docs/screenshots/14-split-horizontal.png"><img src="docs/screenshots/14-split-horizontal.png" width="280" alt="Split View - Horizontal"></a><br><sub>Split View - Horizontal</sub></td>
<td align="center"><a href="docs/screenshots/15-split-vertical.png"><img src="docs/screenshots/15-split-vertical.png" width="280" alt="Split View - Vertical"></a><br><sub>Split View - Vertical</sub></td>
<td align="center"><a href="docs/screenshots/16-command-palette.png"><img src="docs/screenshots/16-command-palette.png" width="280" alt="Command Palette"></a><br><sub>Command Palette</sub></td>
</tr>
<tr>
<td align="center"><a href="docs/screenshots/17-mobile.png"><img src="docs/screenshots/17-mobile.png" width="140" alt="Mobile - App Open"></a><br><sub>Mobile - App Open</sub></td>
<td align="center"><a href="docs/screenshots/18-mobile-menu.png"><img src="docs/screenshots/18-mobile-menu.png" width="140" alt="Mobile - Navigation"></a><br><sub>Mobile - Navigation</sub></td>
<td></td>
</tr>
</table>
</details>

---

## What Is Muximux?

There's a good chance you already have a homelab dashboard. [Homepage](https://gethomepage.dev), [Homarr](https://homarr.dev), [Organizr](https://github.com/causefx/Organizr), and [Dashy](https://dashy.to) are all good at what they do. If one of them already fits how you work, you probably don't need to read further.

Muximux is for people who spend the day working *inside* their self-hosted apps. Click Sonarr and it opens inside the dashboard; click Plex and so does Plex. You stay in Muximux the whole time instead of juggling browser tabs.

Most self-hosted apps set `X-Frame-Options: DENY` and refuse to load in an iframe, which makes embedding them hard. Muximux has a built-in **embedding proxy** that strips those headers, rewrites paths in HTML/CSS/JS, and patches `fetch()` and `XMLHttpRequest` at runtime. Even heavy single-page apps then work the way they do when you open them directly. No other dashboard does this, and it's the reason Muximux exists. Passkeys are the exception: a proxied app runs on Muximux's address, so a passkey registered for the app's own hostname works only if the app also accepts Muximux's address for it.

There are things Muximux deliberately doesn't do. It doesn't pull live widgets from Sonarr or qBittorrent (Homepage is lovely at that). It doesn't have a freeform grid editor where you arrange resizable widgets on a canvas (Homarr is built around that). It doesn't have Dashy's range of built-in monitoring widgets. What it does is run your apps side by side in a single tab, and it tries to be pleasant enough that you can leave that tab open all day.

All configuration is in one YAML file. Muximux needs no database and no external dependencies. The application ships as a single binary with the frontend embedded. You can change every setting in the built-in GUI, so you never have to edit the YAML directly.

The exception is a setting that comes from an environment variable. Settings shows that setting as locked. To change it, change the variable and restart Muximux (see [environment overrides](docs/wiki/configuration.md#booleans-precedence-and-the-memory-only-rule)).

To back up or move your setup, copy `config.yaml`. If you added your own icons or themes, also copy `data/icons/custom/` and `data/themes/`. Also bring the environment variables and secret files that your deployment sets. If the new host does not get an auth override that the old host had, it can start with a different auth method, or with no auth.

### How does it compare?

Each of these dashboards leans a different way:

| | Muximux | Homepage | Homarr | Organizr | Dashy |
|---|:---:|:---:|:---:|:---:|:---:|
| Apps open inside the dashboard | Iframe (default) | iFrame widget | Link-out + widgets | Iframe tabs | Workspace view |
| Rewrites iframe-blocking headers | ✅ | ❌ | ❌ | ❌ | ❌ |
| Runtime SPA interceptor (fetch / XHR / WebSocket) | ✅ | ❌ | ❌ | ❌ | ❌ |
| Iframe permission delegation (camera, mic, geo, passkeys) | ✅ | ❌ | ❌ | ❌ | ❌ |
| Notification bridge for embedded apps | ✅ | ❌ | ❌ | ❌ | ❌ |
| Split view (two apps side by side) | ✅ | ❌ | ❌ | ❌ | ❌ |
| Live service widgets (torrent counts, episode lists, etc.) | ❌ | ✅ (100+) | ✅ | Partial | ✅ (50+) |
| Drag-and-drop visual editor | ✅ apps + groups | ❌ | ✅ | Partial | ✅ |
| Real-time health checks | ✅ (WebSocket) | ✅ | ✅ | ✅ | ✅ |
| Built-in users and roles | ✅ | ❌ (external auth) | ✅ | ✅ | Simple |
| Forward-auth / OIDC / SSO | ✅ | Via external proxy | ✅ (OIDC + LDAP) | ✅ (Plex / Emby / LDAP) | ✅ (Keycloak) |
| Auto-HTTPS via Let's Encrypt | ✅ (embedded Caddy) | ❌ | ❌ | ❌ | ❌ |
| Can front other services on their own subdomains (embedded Caddy) | ✅ | ❌ | ❌ | ❌ | ❌ |
| Start / stop / restart Docker containers from the dashboard | ✅ | ❌ | ✅ | ❌ | ❌ |
| Keeps app URLs working as container IPs / names change | ✅ | ❌ | ❌ | ❌ | ❌ |
| Config | YAML + Docker labels (no DB) | YAML + Docker labels | Database | Database | YAML / UI |
| Deployment | Docker / single static binary | Docker / K8s / binary | Docker | Docker / PHP | Docker / Node.js |

If the rows near the top describe what you want, keep reading.

---

## Why v3?

Muximux v3 is a ground-up rewrite. The original [Muximux](https://github.com/mescon/Muximux/tree/v2.0) was a simple bookmark portal written in PHP. v3 has a new stack and does a lot more:

- A Go backend with the frontend embedded in the binary. No PHP, no web server, no runtime dependencies.
- A built-in embedding proxy that rewrites HTML, CSS, JS, and headers so apps work inside iframes. It is separate from the optional gateway below.
- Real-time health monitoring over WebSocket, so you see which apps are up or down without refreshing.
- Built-in authentication: username/password, forward auth (Authelia/Authentik), or OIDC, configurable in Settings.
- Optional TLS and gateway. An embedded Caddy instance can handle HTTPS certificates and serve other sites alongside Muximux.
- A setup wizard that walks you through security setup and adding your first apps.
- Split view, with two apps side by side or stacked and a draggable divider between them.
- 1,600+ icons from Lucide, plus thousands of service-specific icons from [Dashboard Icons](https://github.com/homarr-labs/dashboard-icons).
- Built-in light and dark modes, plus custom themes via CSS.

---

## How You Can Use It

Pick the setup that matches yours.

### Dashboard behind your existing proxy

You already have Traefik, nginx, or Caddy for TLS and auth. Run Muximux behind it with `auth: none`, and let your proxy handle TLS and auth. Muximux serves on a single port, and your proxy forwards to it. The proxy must forward WebSocket upgrades, because Muximux's live health updates and many embedded apps use them (see the [Deployment Guide](docs/wiki/deployment.md#behind-an-external-reverse-proxy)). This is the simplest setup.

### Dashboard with the embedding proxy

This is the same setup, but some of your apps refuse to load in iframes. Set `proxy: true` on those apps. Muximux's **embedding proxy** then forwards them through `/proxy/{slug}/`. It strips iframe-blocking headers and rewrites paths, so the apps render inside the dashboard. The embedding proxy runs inside the Go server. It works in every deployment mode with no extra configuration, and no other dashboard has one.

The embedding proxy and the gateway do different jobs. The **embedding proxy** makes an app render *inside the dashboard*, which a general-purpose proxy can't do. The **gateway** below is an ordinary proxy that routes a subdomain to a backend.

### Dashboard plus gateway (skip the separate proxy)

If you have no proxy and do not want to run one, Muximux's optional **gateway** (an embedded [Caddy](https://caddyserver.com)) can front your other services too.

- To get automatic HTTPS from Let's Encrypt, set `tls.domain` and `tls.email`.
- To serve other services on their own subdomains, declare them under `gateway_sites:` in `config.yaml` (or in **Settings -> Gateway**). You can add auth to each site if you want.

The same binary then handles TLS certificates, HTTP-to-HTTPS redirects, and routing.

The gateway is kept simple on purpose, for homelab-sized setups that would otherwise need a separate proxy. If you need wildcard certificates, complex routing rules, load balancing, or access-control lists, keep a dedicated proxy ([Nginx Proxy Manager](https://nginxproxymanager.com), [Traefik](https://traefik.io), or Caddy on its own) and run Muximux behind it, as in the first option above.

See the [Deployment Guide](docs/wiki/deployment.md) for Docker Compose examples for each setup.

---

## Quick Start

### Docker

```bash
docker run -d \
  --name muximux \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  ghcr.io/mescon/muximux:latest
```

Open `http://localhost:8080`. An onboarding wizard guides you through security and initial setup.

The wizard asks for a one-time **setup token**. Muximux generates this token on first boot, so nobody who reaches the port before you can claim the instance. The command above runs detached. Read the token from the container output or from the mounted data directory:

```bash
docker logs muximux | grep token
# or
cat data/.setup-token
```

### Docker Compose

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

### Binary

```bash
./muximux
```

By default, Muximux uses `data/` beside the binary as its data directory. This directory holds config, themes, and icons. To use a different directory, set `--data` or the `MUXIMUX_DATA` environment variable.

Release binaries and images carry signed build provenance and ship with an SBOM (software bill of materials, a list of what is in the build). With these, you can confirm that a download came from the workflow that claims to have built it. See [Verifying a Download](docs/wiki/installation.md#verifying-a-download).

See [docker-compose.yml](docker-compose.yml) for a full example with environment variables, health checks, and optional TLS/gateway ports.

---

## Documentation

Full documentation is available in the **[Wiki](docs/wiki/README.md)**:

- [Installation](docs/wiki/installation.md): Docker, binary, building from source, and verifying a download
- [Getting Started](docs/wiki/getting-started.md): First launch and onboarding
- [Configuration Reference](docs/wiki/configuration.md): All config.yaml options
- [Apps](docs/wiki/apps.md): Adding and configuring applications, and links that open a page inside an app
- [Built-in Reverse Proxy](docs/wiki/reverse-proxy.md): How the proxy works and when to use it
- [Docker Discovery](docs/wiki/docker-discovery.md): Import containers as apps (one-click, or automatically from labels) and control them from the dashboard
- [HTTP Actions](docs/wiki/http-actions.md): Turn an app into a one-click HTTP request
- [Security](docs/wiki/security.md): Security measures and OWASP ASVS compliance
- [Authentication](docs/wiki/authentication.md): Auth methods and setup
- [TLS & HTTPS](docs/wiki/tls-and-gateway.md): Certificates and gateway mode
- [Gateway Auth Gate](docs/wiki/gateway-auth.md): Single sign-on for gateway subdomains
- [Deployment Guide](docs/wiki/deployment.md): Production deployment examples
- [Split View](docs/wiki/split-view.md): Side-by-side or stacked app panels
- [Themes](docs/wiki/themes.md): Customizing the look
- [Keyboard Shortcuts](docs/wiki/keyboard-shortcuts.md): Shortcuts, keyboard navigation and accessibility
- [Troubleshooting](docs/wiki/troubleshooting.md): Common issues, and what changes when upgrading to 3.6
- [API Reference](docs/wiki/api.md): REST API endpoints

---

## Configuration at a Glance

All configuration is in one YAML file. This example shows the main settings:

```yaml
server:
  listen: ":8080"
  title: "My Homelab"

apps:
  - name: Sonarr
    url: http://sonarr:8989
    icon: { type: dashboard, name: sonarr }
    group: Media
    proxy: true            # Proxy through /proxy/sonarr/
    open_mode: iframe      # iframe, new_tab, new_window, redirect, http_action

  - name: Grafana
    url: http://grafana:3000
    icon: { type: dashboard, name: grafana }
    group: Monitoring

groups:
  - name: Media
    icon: { type: lucide, name: play }
    color: "#e5a00d"

  - name: Monitoring
    icon: { type: lucide, name: activity }
    color: "#27ae60"
```

Configuration values can reference environment variables, for example `${OIDC_CLIENT_SECRET}`. To override the listen address, use the `--listen` flag or the `MUXIMUX_LISTEN` environment variable. Logging, Docker auto-import, the auth method, and every OIDC setting also have `MUXIMUX_*` overrides. See the [environment variable reference](docs/wiki/configuration.md#environment-variable-reference). The data directory (where config, themes, and icons live) defaults to `data/` beside the binary. To change it, use `--data` or `MUXIMUX_DATA`.

### Or configure apps with Docker labels

With [Docker Discovery](docs/wiki/docker-discovery.md) enabled, a container can declare its own Muximux entry with labels. Without auto-import, the labels pre-fill the import modal. With `discovery.docker.auto_import`, the app appears with no clicks and stays in sync. Declare the app once in your compose file, and Muximux picks it up:

```yaml
services:
  sonarr:
    image: linuxserver/sonarr
    labels:
      - muximux.app.name=Sonarr
      - muximux.app.group=Media
      - muximux.app.icon=sonarr
      - muximux.app.port=8989
      - muximux.app.proxy=true
```

`muximux.app.*` labels cover the whole app entry (icon, group, open mode, health check, access control, and more). `muximux.gateway.*` labels can also publish the container on its own gateway subdomain. A background poller keeps imported URLs current when container IPs change across restarts. For the full label reference, see [Docker Discovery](docs/wiki/docker-discovery.md).

If Muximux is behind your own reverse proxy, add `muximux.app.url=https://sonarr.example.com`. The app then opens at its public name, and health checks still go to the container.

For the full configuration reference, authentication options, TLS setup, and more, see the **[Wiki](docs/wiki/README.md)**.

---

## Features

| | |
|---|---|
| **Reverse Proxy** | Strips iframe-blocking headers, rewrites HTML/CSS/JS paths, handles gzip, and isolates `window.parent`/`window.top`, so apps that normally refuse to be framed work in an iframe. Per-app TLS settings, custom headers, and a configurable timeout |
| **Health Monitoring** | Periodic health checks with real-time WebSocket updates and colored status indicators |
| **Real-Time Log Viewer** | In-app log viewer with level/source filtering, search, auto-scroll, pause/resume, and download. Debug level exposes detailed request tracing |
| **Authentication** | Built-in users (bcrypt), forward auth (Authelia/Authentik), or OIDC (set up in Settings), with user management and roles. The auth method and every OIDC setting can also come from `MUXIMUX_*` environment variables, with the client secret in a Docker secret. A fresh container can then start on single sign-on with no config file and no setup wizard |
| **TLS / HTTPS** | Automatic Let's Encrypt certificates or manual cert/key, powered by embedded Caddy |
| **Gateway** | Reverse proxies other sites and services on your network, each on its own subdomain. Sites are declared in YAML under `gateway_sites`, with per-site TLS modes (automatic Let's Encrypt, none, or custom certs), header forwarding, iframe-blocker stripping, and streaming-safe backends. Edit them visually in Settings -> Gateway or by hand. Each subdomain can optionally require the Muximux login (`require_auth`), so one sign-in covers your whole homelab |
| **Docker Discovery** | Connect Muximux to the Docker daemon and import running containers as apps in one click, with name, icon, and URL filled in. For each row you choose how the app is exposed: direct, through the proxy, or on its own gateway subdomain. A background poller keeps URLs current as container IPs shift across restarts. For GitOps, turn on automatic import (`discovery.docker.auto_import`, off by default) and Muximux picks up `muximux.*`-labeled containers by itself, so an app you declare in your compose file appears without any clicks. Labels you set (name, icon, group, order, health check, and the icon, colour and order of the group) stay applied while the app is tracked. Icon labels accept Dashboard Icons, Lucide icons or image URLs. Docker Swarm services and Compose projects keep one app across redeploys |
| **Container Controls** | Start, stop, and restart Docker-discovered containers from the dashboard, with a status dot on each app card that stays quiet until a container is stopped, unhealthy, or restarting. Opt-in: it needs a read-write socket mount and role/group permissions. Every action is audit-logged |
| **HTTP Actions** | Turn an app into a button that sends an HTTP request (any method, custom headers) through a server-side relay instead of opening a page. Handy for webhooks and automations, such as triggering a backup or waking a host. You can add a confirmation prompt, and every request is audit-logged |
| **Split View** | Display two apps side by side or stacked with a draggable divider, panel selector for targeting left/right or top/bottom, and URL hash routing (`#app1+app2`) for bookmarking layouts |
| **Links Into Apps** | A bookmark, a Homarr tile or a note can open Muximux on a given app, a split layout, or a page inside the app (`#emby?path=...`, `#sonarr+radarr?sonarr=...&radarr=...`). Apps you allow can also switch apps and open pages from their own links or a small `postMessage` API. Every path stays inside its target app, and both features are opt-in per app ([Navigation from Embedded Apps](docs/wiki/apps.md#navigation-from-embedded-apps)) |
| **Navigation** | 5 positions (top, left, right, bottom, floating), pinned apps as one-click buttons in the bar, draggable FAB, auto-hide, customizable width |
| **Icons** | 1,600+ Lucide icons, thousands of Dashboard Icons, URL icons, custom uploads |
| **Keyboard Shortcuts** | Keybindings for switching apps, toggling navigation, and search |
| **Accessibility** | Usable from the keyboard, with a visible focus outline, focus kept inside dialogs, named controls and screen reader announcements, and reduced motion when the operating system asks for it |
| **Themes** | 9 built-in theme families (Default, Nord, Dracula, Catppuccin, Solarized, Tokyo Night, Gruvbox, Cineplex, Rose Pine) with dark/light variants, all meeting WCAG 2.1 AA contrast, plus custom themes via CSS |
| **Config Export/Import** | Export the config as YAML with sensitive data stripped, or import a file, preview it, and apply it |
| **Environment Variables** | `${VAR}` expansion in config.yaml, plus `MUXIMUX_*` overrides for the data directory, listen address, base path, logging, Docker auto-import, the auth method and OIDC. Overrides stay in memory and never end up in `config.yaml` ([full list](docs/wiki/configuration.md#environment-variable-reference)) |
| **REST API** | Full CRUD for apps, groups, config, health, and auth |
| **Single Binary** | The frontend is embedded in the Go binary, so you deploy one file with no runtime dependencies |
| **Portable** | No database: one YAML config file stores everything. Every setting is editable in the GUI, saves merge onto the live config, and every open browser picks up a change at once. Copy one file to back up or migrate |

---

## Project Philosophy

Muximux is meant to stay a focused homelab dashboard: a single page where you open and organize your self-hosted apps. New features are judged by whether they serve that purpose, so it won't grow into a monitoring suite or a system management tool.

Suggestions are always welcome, but not all of them will make it in. That's on purpose: a smaller set of features that work well beats a long list.

## Development

Muximux is developed using Claude Code (Opus 4.6 through 4.8, currently Claude Fable 5) with MCP servers for browser automation (Playwright), GitHub integration, and other validation tools. It's a heavily AI-assisted workflow, but all code is reviewed, tested, and committed by the sole maintainer, [@mescon](https://github.com/mescon). AI output is treated the same way as a pull request from a contributor: nothing ships without understanding and approval.

Every push goes through CI with 85%+ test coverage on both backend (`./internal/...`) and frontend (the `src/lib` and `src/components` surface; the app shell is excluded and verified by browser testing), golangci-lint, ESLint, `svelte-check`, CodeQL analysis, Snyk vulnerability scanning (Go, Node, Docker), govulncheck, SonarCloud quality gates, and a pre-push hook that blocks pushes below coverage thresholds. The security model is designed against [OWASP ASVS Level 2](docs/wiki/security.md).

```bash
git clone https://github.com/mescon/Muximux.git
cd Muximux

# Enable git hooks (pre-push runs tests with coverage checks)
git config core.hooksPath .githooks

# Frontend dev server (hot reload)
cd web && npm install && npm run dev &

# Backend (uses data/ beside the binary by default for config, themes, icons)
go run ./cmd/muximux
```

### Building

```bash
# Build frontend (outputs to internal/server/dist/)
cd web && npm run build && cd ..

# Build binary (embeds frontend)
go build -tags embed_web -o muximux ./cmd/muximux

# With version info (uses latest git tag)
go build -tags embed_web -ldflags "-X main.version=$(git describe --tags --abbrev=0)" -o muximux ./cmd/muximux
```

---

## License

GPL v2. See [LICENSE](LICENSE) for details.

## Credits

- [Lucide Icons](https://lucide.dev/) for the icon library
- [Dashboard Icons](https://github.com/homarr-labs/dashboard-icons) by Homarr Labs
- [Caddy](https://caddyserver.com/) for embedded TLS and gateway support

## Star History

<a href="https://www.star-history.com/?repos=mescon%2FMuximux&type=date&legend=bottom-right">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=mescon/Muximux&type=date&theme=dark&legend=bottom-right&sealed_token=FULv35iBfdbVnac3f8vXM8CV4Sp45DJqgPc-clcNpVOA2mZ1sbRGkOiM4ASXWKj5l6s6i1VNq52fZVnmtXqksaQYFL_yP67455yuhaVGr1bJSHxkIWVDGg" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=mescon/Muximux&type=date&legend=bottom-right&sealed_token=FULv35iBfdbVnac3f8vXM8CV4Sp45DJqgPc-clcNpVOA2mZ1sbRGkOiM4ASXWKj5l6s6i1VNq52fZVnmtXqksaQYFL_yP67455yuhaVGr1bJSHxkIWVDGg" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=mescon/Muximux&type=date&legend=bottom-right&sealed_token=FULv35iBfdbVnac3f8vXM8CV4Sp45DJqgPc-clcNpVOA2mZ1sbRGkOiM4ASXWKj5l6s6i1VNq52fZVnmtXqksaQYFL_yP67455yuhaVGr1bJSHxkIWVDGg" />
 </picture>
</a>

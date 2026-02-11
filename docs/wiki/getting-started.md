# Getting Started

This page walks you through what happens the first time you launch Muximux and how to get your dashboard up and running.

---

## First Launch

When Muximux starts with no apps configured (either no `config.yaml` exists or the file has an empty `apps` list), it shows the **Onboarding Wizard** -- a guided setup that walks you through initial configuration.

Open your browser and navigate to Muximux (by default, `http://your-server:8080`).

---

## Onboarding Wizard

The wizard has five steps. You can move forward and backward between steps at any time.

### Step 1: Welcome

A brief introduction to Muximux and its core features: embedded app views via iframes, health monitoring, and keyboard-driven navigation. Click **Let's Get Started** to begin.

### Step 2: Apps

Browse a library of popular self-hosted application templates organized by category:

- **Media** -- Plex, Jellyfin, Emby, Tautulli, Overseerr, Navidrome
- **Downloads** -- Sonarr, Radarr, Lidarr, Prowlarr, qBittorrent, SABnzbd, NZBGet, Transmission, Deluge
- **System** -- Portainer, Proxmox, Unraid, TrueNAS, Home Assistant, Pi-hole, AdGuard Home, Nginx Proxy Manager, Traefik, Grafana, Prometheus, Uptime Kuma
- **Utilities** -- Vaultwarden, Nextcloud, Photoprism, Immich, Paperless-ngx, Gitea, Code Server, Syncthing, Mealie, Bookstack

Click on any app to select it. When selected, a URL field appears so you can enter the actual address of that service in your network (e.g., change `http://localhost:32400/web` to `http://192.168.1.50:32400/web`).

You can also add **custom applications** that are not in the template list by clicking "Add custom application" at the bottom. Provide a name, URL, color, and optionally a group.

### Step 3: Navigation Style

Choose how the navigation bar appears in your dashboard:

| Position | Description |
|----------|-------------|
| **Top Bar** | Horizontal navigation across the top of the page |
| **Left Sidebar** | Vertical sidebar on the left side |
| **Right Sidebar** | Vertical sidebar on the right side |
| **Bottom Dock** | macOS-style dock at the bottom of the page |
| **Floating** | Minimal floating buttons |

You can also toggle **Show App Labels** to control whether app names are displayed alongside their icons in the navigation.

### Step 4: Groups

Based on the apps you selected, Muximux automatically suggests groups to organize them (e.g., Media, Downloads, System, Utilities). Each group shows how many of your selected apps belong to it.

If you selected apps from only one category, or no apps at all, this step may show no groups -- all apps will simply appear in a flat list.

Groups can be fully customized later in Settings.

### Step 5: Complete

A summary of your choices is displayed: number of apps, navigation position, number of groups, and label visibility. Click **Launch Dashboard** to apply the configuration and open your new dashboard.

---

## After Onboarding

Once the wizard completes:

- Your configuration is saved to `config.yaml`.
- The first app in your list is automatically set as the **default app**, which loads when you open the dashboard.
- **Health monitoring** starts automatically, checking each app at the configured interval (default: every 30 seconds).
- The onboarding wizard will appear automatically whenever no apps are configured. Once you add apps, it will not appear again unless all apps are removed.

You can open **Settings** at any time from the navigation bar to modify apps, groups, navigation style, themes, and all other options.

---

## Resetting Muximux

To completely start fresh and see the onboarding wizard again:

1. **Stop Muximux** (or the container).
2. **Delete or rename** the `config.yaml` file.
3. **Start Muximux** again.

The onboarding wizard will appear automatically because no apps are configured. No browser state needs to be cleared.

---

## Quick Manual Setup

If you prefer to skip the onboarding wizard entirely, you can create a `config.yaml` by hand before starting Muximux. Here is a minimal example:

```yaml
server:
  listen: ":8080"
  title: "My Dashboard"

navigation:
  position: "left"
  show_labels: true

health:
  enabled: true
  interval: "30s"
  timeout: "5s"

apps:
  - name: "Portainer"
    url: "http://192.168.1.10:9000"
    icon:
      type: "dashboard"
      name: "portainer"
    color: "#13BEF9"
    group: "System"
    order: 0
    enabled: true
    default: true
    open_mode: "iframe"
    proxy: false
    scale: 1

  - name: "Plex"
    url: "http://192.168.1.10:32400/web"
    icon:
      type: "dashboard"
      name: "plex"
    color: "#E5A00D"
    group: "Media"
    order: 1
    enabled: true
    default: false
    open_mode: "iframe"
    proxy: false
    scale: 1

groups:
  - name: "System"
    icon:
      type: "lucide"
      name: "server"
    color: "#F46800"
    order: 0
    expanded: true

  - name: "Media"
    icon:
      type: "lucide"
      name: "play"
    color: "#E5A00D"
    order: 1
    expanded: true
```

See the [Configuration Reference](configuration.md) for a complete description of every available option.

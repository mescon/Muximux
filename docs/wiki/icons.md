# Icons

Muximux supports four types of icons for apps and groups: Dashboard Icons, Lucide Icons, Custom Icons, and URL Icons.

## Dashboard Icons

Over 600 icons for popular self-hosted applications, sourced from the [dashboard-icons](https://github.com/homarr-labs/dashboard-icons) project by Homarr Labs.

```yaml
icon:
  type: dashboard
  name: plex          # Icon name (matches the service name)
  variant: light      # light, dark, or empty for default
```

### Configuration

Dashboard icon behavior is configured in the `icons` section of `config.yaml`:

```yaml
icons:
  dashboard_icons:
    enabled: true
    mode: on_demand    # on_demand, prefetch, or offline
    cache_dir: icons/dashboard
    cache_ttl: 7d      # How long to cache icons
```

### Modes

- **on_demand** -- Icons are downloaded when first requested and cached locally. Best for most users. The first load of each icon has a slight delay; subsequent loads are instant from cache.
- **prefetch** -- All icons are downloaded at startup. Results in slower startup time, but all icons are available immediately once the server is running.
- **offline** -- Only use previously cached icons. No network requests are made. You must have run in `on_demand` or `prefetch` mode at least once before switching to `offline`.

## Lucide Icons

Over 1,600 open-source icons from the [Lucide](https://lucide.dev/) icon library. These are clean, consistent SVG icons organized by category.

```yaml
icon:
  type: lucide
  name: server        # Lucide icon name
```

Lucide icons are fetched from jsDelivr CDN and cached locally. They work well for generic icons (settings, home, play, download, folder, etc.) when dashboard-icons doesn't have a match for your app.

## Custom Icons

Upload your own icons (PNG, SVG, JPG, WebP) through the Settings panel or API.

```yaml
icon:
  type: custom
  file: my-custom-icon.png    # Filename in data/icons/custom/
```

**Upload limits:** 5MB per icon. Admin role required.

Custom icons are stored in the `data/icons/custom/` directory and served at `/icons/custom/{filename}`.

You can upload custom icons in two ways:
1. Through the Settings panel icon browser (click "Upload" in the Custom tab)
2. Via the API: `POST /api/icons/custom` with a multipart form upload

## URL Icons

Reference any external image URL as an icon.

```yaml
icon:
  type: url
  url: https://example.com/icon.png
```

Useful for apps that provide their own favicon or icon URL.

**Note:** The URL must be accessible from the user's browser, not from the Muximux server. The browser loads URL icons directly, so the image host must be reachable from wherever your users are browsing.

## Icon Appearance Options

All icon types support optional appearance customization:

```yaml
icon:
  type: dashboard
  name: sonarr
  variant: light
  color: "#ff9600"        # Tint the icon with this color
  background: "#1a1a2e"   # Background color behind the icon
```

The `color` and `background` options work together with the navigation settings `show_app_colors` and `show_icon_background`. If those navigation settings are disabled, the color and background values are ignored.

## Icon Browser

The Settings panel includes an icon browser that lets you search and preview icons from all available sources. You can:

- Search icons by name across all sources
- Filter by source (Dashboard, Lucide, Custom)
- Preview icons before selecting them
- See available variants for dashboard icons (light, dark, default)
- Upload new custom icons directly from the browser

To open the icon browser, edit any app or group in Settings and click the icon field.

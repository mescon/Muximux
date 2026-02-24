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

**Upload limits:** 2MB per icon. Admin role required.

Custom icons are stored in the `data/icons/custom/` directory and served at `/icons/custom/{filename}`.

You can add custom icons in three ways:
1. **File upload** -- Through the Settings panel icon browser (click "Upload" in the Custom tab)
2. **Fetch from URL** -- Paste an image URL in the Custom tab and click "Fetch". The server downloads the image, validates it, and saves it locally as a custom icon. This is the recommended way to use icons from external sources, since the image is stored locally and won't break if the remote server goes down.
3. **API** -- `POST /api/icons/custom` (multipart upload) or `POST /api/icons/custom/fetch` (download from URL)

## URL Icons

Reference any external image URL as an icon. This is a power-user option for manual `config.yaml` editing.

```yaml
icon:
  type: url
  url: https://example.com/icon.png
```

Useful for apps that provide their own favicon or icon URL.

**Note:** The URL must be accessible from the user's browser, not from the Muximux server. The browser loads URL icons directly, so the image host must be reachable from wherever your users are browsing.

> **Prefer fetching over hotlinking.** URL icons break if the remote server goes down or the image moves. The icon browser's **Fetch from URL** feature (in the Custom tab) downloads the image and stores it locally, avoiding this problem. Use `type: url` only when you need a live reference to an image that changes externally.

## Icon Appearance Options

All icon types support optional appearance customization:

```yaml
icon:
  type: dashboard
  name: sonarr
  variant: light
  color: "#ff9600"        # Tint color (Lucide icons only)
  background: "#1a1a2e"   # Background color behind the icon
  invert: true            # Invert icon colors (swap dark ↔ light)
```

### Color

The `color` field applies a color tint to Lucide icons. It has no effect on other icon types (dashboard, custom, or URL icons already have their own colors baked in).

### Background

The `background` field sets a custom background color behind the icon. By default, the app's accent `color` is used as the icon background. Setting `background` overrides that default for just the icon, without changing the app's accent color elsewhere (tab indicator, sidebar highlight).

The background is only visible when `show_icon_background` is enabled in the navigation settings, unless the app has `force_icon_background: true` set.

### Invert

The `invert` field flips the icon's colors -- turning light icons dark and dark icons bright. This is useful when a dashboard icon only comes in a variant that clashes with your theme. For example, if an app only provides a white-on-transparent icon and you are using a light theme, setting `invert: true` makes it visible.

## Icon Browser

The Settings panel includes an icon browser that lets you search and preview icons from all available sources. You can:

- Search icons by name across all sources
- Filter by source (Dashboard, Lucide, Custom)
- Preview icons before selecting them
- See available variants for dashboard icons (light, dark, default)
- Upload new custom icons directly from the browser
- Fetch icons from a URL (downloaded and stored locally)

To open the icon browser, edit any app or group in Settings and click the icon field.

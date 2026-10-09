# Themes

Muximux uses a CSS custom properties system for theming. You can choose from built-in themes, create your own through the Settings panel, or edit theme CSS files directly.

## Configuration

The active theme is stored in your `data/config.yaml`:

```yaml
theme:
  family: default              # Theme family: default, nord, dracula, etc.
  variant: system              # dark, light, system
```

This is the authoritative source of truth and syncs across all browsers and devices. The browser also caches the current theme in localStorage for instant application on page load (avoiding a flash of unstyled content), but the server config always takes precedence when loaded.

## Built-in Themes

Muximux ships with multiple built-in theme families, each with dark and light variants:

- **Default** -- Deep charcoal (dark) or clean and bright (light) with teal accents. This is the default theme.
- **Nord** -- Arctic, blue-grey palette inspired by the Nord color scheme.
- **Dracula** -- Dark purple tones from the popular Dracula theme.
- **Catppuccin** -- Warm, pastel tones from the Catppuccin palette.
- **Solarized** -- Ethan Schoonover's precision-crafted color scheme.
- **Tokyo Night** -- Inspired by Tokyo city lights at night.
- **Gruvbox** -- Retro groove with warm, earthy colors.
- **Cineplex** -- Warm amber and gold tones inspired by media player interfaces.
- **Rose Pine** -- Soft, muted tones with a natural feel.

Built-in themes cannot be deleted.

## Variant Modes

Each theme supports three variant modes:

- **Dark** -- Always use the dark variant of the selected theme.
- **Light** -- Always use the light variant of the selected theme.
- **System** -- Automatically follow your operating system's preference. This uses the `prefers-color-scheme` media query, so switching your OS between light and dark mode will update Muximux in real time.

## Changing Themes

1. Open **Settings** (click the gear icon or press `S`).
2. Go to the **Appearance** tab.
3. Select a theme family and a variant mode.

Changes apply instantly and are saved to `config.yaml` when you click Save. No restart is needed. Switching theme from the command palette is a per-browser preview; the next Settings save persists it.

[![Theme selector with 9 theme families](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/11-themes.png)](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/11-themes.png)

Here are a few examples of the available themes applied to the onboarding wizard:

| Muximux (Default) | Catppuccin | Dracula |
|:---:|:---:|:---:|
| [![Default dark](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/06-theme-dark.png)](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/06-theme-dark.png) | [![Catppuccin](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/06b-theme-catppuccin.png)](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/06b-theme-catppuccin.png) | [![Dracula](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/06c-theme-dracula.png)](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/06c-theme-dracula.png) |

You can also select a theme during the onboarding wizard when setting up Muximux for the first time.

## Custom Themes

You can create fully custom themes through the Settings panel:

1. Open **Settings > Appearance**.
2. Click **"New Theme"**.
3. Use the theme editor to customize colors, backgrounds, borders, and other visual properties.
4. Save the theme.

[![Theme editor with color pickers](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/12-theme-customizer.png)](https://raw.githubusercontent.com/mescon/Muximux/main/docs/screenshots/12-theme-customizer.png)

A custom theme name must not match a bundled theme's name; the save is refused.

Custom themes are stored as CSS files in the `data/themes/` directory. When you create a theme, Muximux generates a CSS file that overrides the default CSS custom properties (variables) with your chosen values.

## Theme CSS Variables

Themes control the visual appearance through CSS custom properties defined on `:root`. The key variable groups include:

- **Background colors** -- Page background, surface, elevated surface
- **Text colors** -- Primary, secondary, muted
- **Accent/brand colors** -- Used for highlights, active states, and interactive elements
- **Border colors and radii** -- Controls the look and roundness of UI elements
- **Shadow definitions** -- Drop shadows for depth and layering
- **Navigation-specific colors** -- Background, text, and active state colors for the navigation bar

You do not need to override every variable. Any variable you omit will fall back to the base theme's default.

### Status and accent tokens

Status boxes, pills, inline messages and the danger button read these variables. A theme may override any of them; omitted ones fall back to the defaults in `app.css`, which reproduce the colours used before the tokens existed.

| token | used for | fallback |
|---|---|---|
| `--success-text`, `--success-bg`, `--success-border` | success notices and pills (text, translucent background, border) | green text, a 10% green tint and a 40% green border |
| `--warning-text`, `--warning-bg`, `--warning-border` | warnings, "Unsaved changes", the discard prompt | amber text, a 10% amber tint and a 40% amber border |
| `--danger-text`, `--danger-bg`, `--danger-border` | errors, failed saves, destructive hints | red text, a 10% red tint and a 40% red border |
| `--info-text`, `--info-bg`, `--info-border` | informational pills | blue text, a 15% blue tint and a 30% blue border |
| `--danger-solid`, `--danger-solid-hover`, `--danger-on-solid` | the solid danger button fill, its hover fill and its text | red, a lighter red, `#ffffff` |
| `--accent-text` | the accent used as text: links, the active tab and the active navigation item | `--accent-primary` |
| `--accent-on-primary` | text on an accent-filled button | `#ffffff` |
| `--border-focus` | the keyboard focus outline on every control | `--accent-primary` |

For `--accent-on-primary`, use `#ffffff` or `#000000`, whichever has the higher contrast on your accent (one of the two always reaches at least 4.5:1). When you save a custom theme, the theme editor works this out from your accent colour (a translucent accent is judged over `--bg-base`) and writes it for you.

The `--color-brand-*` palette is no longer used by the interface and is kept only for older theme files.

### Focus, motion and screen readers

- **Focus.** Every focusable element shows the same outline in `--border-focus` while it has keyboard focus. A field with an error keeps its danger border while focused.
- **Reduced motion.** When your operating system asks for reduced motion, transitions collapse, spinners slow down and pulses stop.
- **Health indicator.** Each status has its own shape (circle, diamond or ring) so it does not rely on colour alone. The indicator has an accessible name and a tooltip that opens with the keyboard. The "Check now" action in that tooltip is mouse-only for now.
- **Screen readers.** Errors are announced as alerts and confirmations as status messages. The onboarding wizard announces each step change, and the log level filters report whether they are on or off. Form controls and icon-only buttons have names, and per-row actions name their item or position.

## Importing and Exporting Themes

Themes are plain CSS files, which makes sharing straightforward:

- **Import:** Copy a `.css` theme file into the `data/themes/` directory. It will appear in Settings automatically on the next page load.
- **Export/Share:** Copy a theme file from `data/themes/` and share it with others or transfer it to another Muximux installation.
- **Manual editing:** You can open any theme CSS file in a text editor for fine-grained control over individual variables.

## Deleting Custom Themes

To delete a custom theme:

1. Open **Settings > Appearance**.
2. Find the custom theme card.
3. Click the **delete** button on the theme card.

If the deleted theme was active, Muximux will revert to the default dark theme. Built-in themes cannot be deleted.

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
- **Plex** -- Inspired by the Plex media player interface.
- **Rose Pine** -- Soft, muted tones with a natural feel.

Built-in themes cannot be deleted.

## Variant Modes

Each theme supports three variant modes:

- **Dark** -- Always use the dark variant of the selected theme.
- **Light** -- Always use the light variant of the selected theme.
- **System** -- Automatically follow your operating system's preference. This uses the `prefers-color-scheme` media query, so switching your OS between light and dark mode will update Muximux in real time.

## Changing Themes

1. Open **Settings** (click the gear icon or press `Ctrl+,`).
2. Go to the **Appearance** tab.
3. Select a theme family and a variant mode.

Changes apply instantly and are saved to `config.yaml` when you click Save. No restart is needed.

You can also select a theme during the onboarding wizard when setting up Muximux for the first time.

## Custom Themes

You can create fully custom themes through the Settings panel:

1. Open **Settings > Appearance**.
2. Click **"New Theme"**.
3. Use the theme editor to customize colors, backgrounds, borders, and other visual properties.
4. Save the theme.

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

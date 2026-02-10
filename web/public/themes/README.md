# Custom Themes for Muximux

This directory contains custom theme CSS files that extend Muximux's appearance beyond the built-in dark and light themes.

## Using Custom Themes

1. Place your theme CSS file in this directory (`/themes/`)
2. Restart Muximux (or refresh the page)
3. Go to Settings → Theme and select your theme

## Creating a Custom Theme

The easiest way to create a custom theme is to copy an existing one:

```bash
cp nord.css my-theme.css
```

Then edit the CSS custom properties to match your desired color palette.

### Required CSS Structure

Your theme file must define a CSS rule that targets `[data-theme="your-theme-id"]`:

```css
[data-theme="my-theme"] {
  color-scheme: dark; /* or 'light' */

  /* Backgrounds */
  --bg-base: #...;
  --bg-surface: #...;
  --bg-elevated: #...;
  --bg-overlay: #...;
  --bg-hover: #...;
  --bg-active: #...;

  /* Glass surfaces */
  --glass-bg: rgba(...);
  --glass-border: rgba(...);
  --glass-highlight: rgba(...);

  /* Text */
  --text-primary: #...;
  --text-secondary: #...;
  --text-muted: #...;
  --text-disabled: #...;

  /* Borders */
  --border-subtle: rgba(...);
  --border-default: rgba(...);
  --border-strong: rgba(...);
  --border-focus: var(--accent-primary);

  /* Accent colors */
  --accent-primary: #...;
  --accent-secondary: #...;
  --accent-muted: rgba(...);
  --accent-subtle: rgba(...);

  /* Status colors */
  --status-success: #...;
  --status-warning: #...;
  --status-error: #...;
  --status-info: #...;

  /* Shadows */
  --shadow-sm: ...;
  --shadow-md: ...;
  --shadow-lg: ...;
  --shadow-glow: ...;
}
```

### Theme Metadata

Add metadata comments at the top of your file for the theme selector to display:

```css
/**
 * @theme-id: my-theme
 * @theme-name: My Custom Theme
 * @theme-description: A beautiful custom theme
 * @theme-is-dark: true
 * @theme-preview-bg: #1a1a1a
 * @theme-preview-surface: #2a2a2a
 * @theme-preview-accent: #ff6b6b
 * @theme-preview-text: #ffffff
 */
```

### Color Guidelines

**For dark themes (`color-scheme: dark`):**
- `--bg-base`: Darkest background, used for the main canvas
- `--bg-surface`: Slightly lighter, used for cards and panels
- `--bg-elevated`: Even lighter, used for elevated elements
- `--text-primary`: Should have high contrast against `--bg-base`

**For light themes (`color-scheme: light`):**
- `--bg-base`: Lightest background (near white)
- `--bg-surface`: Pure white or very light
- `--text-primary`: Should be very dark for readability

**General tips:**
- Use RGBA for borders to maintain transparency across backgrounds
- Accent colors should have good contrast in both light and dark contexts
- Status colors (success/warning/error/info) should remain distinguishable

## Included Themes

- **nord.css** - Arctic, north-bluish color palette based on [Nord](https://www.nordtheme.com/)

## Popular Color Palettes for Inspiration

- [Catppuccin](https://catppuccin.com/) - Soothing pastel themes
- [Dracula](https://draculatheme.com/) - Dark theme with vibrant colors
- [Gruvbox](https://github.com/morhetz/gruvbox) - Retro groove color scheme
- [Tokyo Night](https://github.com/enkia/tokyo-night-vscode-theme) - Clean dark theme inspired by Tokyo nights
- [Solarized](https://ethanschoonover.com/solarized/) - Precision colors for machines and people

## Contributing

If you create a theme you'd like to share, consider submitting a pull request!

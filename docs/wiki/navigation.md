# Navigation

Muximux supports five navigation layouts. You can switch between them at any time in the Settings panel or by editing `config.yaml` directly.

## Navigation Positions

### Top

Horizontal bar across the top of the screen. Apps are shown as icons with optional labels below. This layout works well on wide screens with many apps, as the full width is available for app icons.

### Left

Vertical sidebar on the left side of the screen. This is the classic dashboard layout. The sidebar width is configurable.

### Right

Vertical sidebar on the right side of the screen. A mirror of the left layout, useful if you prefer navigation on the right.

### Bottom

Horizontal bar across the bottom of the screen. Same layout as the top bar, just anchored to the bottom edge.

### Floating

A sidebar that overlays the content rather than pushing it aside. The sidebar appears on hover and disappears when you move away. This option maximizes the screen space available to your apps.

## Bar Style (Top/Bottom Only)

When using **top** or **bottom** navigation, you can choose between two bar styles:

### Grouped (default)

Apps are organized under collapsible group headers. Each group has its own section with the group name displayed above its apps. This is the standard layout and works well when you have many apps organized into distinct categories.

### Flat

A streamlined layout that displays all apps in a single continuous row. Groups are separated by small icon dividers (using the group's configured icon) rather than full headers. This creates a more compact, dock-like appearance and is useful when you want to maximize horizontal space or prefer a cleaner look.

```yaml
navigation:
  position: top
  bar_style: flat    # grouped (default) or flat
```

The flat style only applies to top and bottom bars. Left, right, and floating navigation always use the grouped layout with collapsible sections.

## Configuration

All navigation settings are available in `config.yaml` under the `navigation` key:

```yaml
navigation:
  position: top              # top, left, right, bottom, floating
  width: 220px               # Sidebar width (for left/right/floating)
  auto_hide: false            # Hide navigation after inactivity
  auto_hide_delay: 3s         # Delay before hiding (when auto_hide is true)
  show_on_hover: true         # Show hidden nav on mouse hover
  show_labels: true           # Display app names under/beside icons
  show_logo: true             # Show Muximux logo in navigation
  show_app_colors: true       # Tint app icons with their configured color
  show_icon_background: true  # Show circular background behind icons
  show_splash_on_startup: false # Show splash screen on initial load
  show_shadow: true           # Add drop shadow to navigation bar
  bar_style: grouped          # grouped or flat (top/bottom only)
```

## Auto-Hide Behavior

When `auto_hide` is set to `true`:

- The navigation bar hides itself after `auto_hide_delay` of no mouse activity near the navigation area.
- Move your mouse to the navigation edge of the screen to reveal it again (when `show_on_hover` is `true`).
- The **floating** position always overlays content regardless of visibility state. The other positions (top, left, right, bottom) reclaim the screen space when the navigation is hidden, giving your apps more room.

## Mobile

On small screens, the navigation automatically collapses into a hamburger menu in the top corner. On touch devices, you can also swipe from the left edge of the screen to open the navigation panel.

## All Settings Are Live

Every navigation setting can be changed in the Settings panel with an immediate preview. Changes take effect instantly -- no page reload or restart is needed. This makes it easy to experiment with different layouts and find the one that works best for your setup.

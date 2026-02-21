# Split View

Split view displays two apps side by side (horizontal) or stacked (vertical) in a single browser tab. Each panel is an independent iframe that stays loaded when you switch focus, so you can monitor two services at once without losing state.

## Enabling Split View

Click the split view icon in the navigation bar. This activates horizontal split by default and opens a second panel. The navigation controls expand to show orientation and panel selection options.

The split view controls appear inline in the navigation bar across all five positions (top, left, right, bottom, floating).

### Controls

When split view is active, the navigation bar shows:

| Control | Description |
|---------|-------------|
| **Horizontal** | Two side-by-side columns icon. Click to switch to horizontal orientation. Highlighted when active. |
| **Vertical** | Two stacked rows icon. Click to switch to vertical orientation. |
| **Panel arrows** | Chevron arrows indicating which panel receives the next app selection. The active panel's arrow is highlighted with the accent color. |
| **Close** | X icon. Closes split view and keeps the active panel's app. |

### Selecting a Panel

The panel arrows in the navigation bar control which panel is the **target** — when you click an app in the navigation, it loads into the target panel. The arrows adapt to the current orientation:

- **Horizontal split:** Left (◀) and right (▶) arrows
- **Vertical split:** Up (▲) and down (▼) arrows

You can also click directly inside a panel to set it as the active target.

### Active Panel Indicator

The draggable divider between panels shows a colored accent line on the active panel's edge. This provides a visual indicator of which panel is targeted without overlaying anything on the iframe content.

## Draggable Divider

The divider between the two panels can be dragged to resize them. Double-click the divider to reset it to a 50/50 split. The divider position is clamped between 20% and 80% to prevent either panel from becoming too small.

## URL Hash Routing

Split view state is reflected in the URL hash:

| URL | Result |
|-----|--------|
| `#sonarr` | Single view with Sonarr loaded |
| `#sonarr+radarr` | Split view with Sonarr in panel 1 and Radarr in panel 2 |

This means you can bookmark a split view configuration or share it as a link. When Muximux loads with a `+` in the hash, it automatically enables horizontal split and loads both apps.

## Switching Orientation

Click the horizontal or vertical icon to switch orientation while split view is active. Both panels and their loaded apps are preserved — only the layout direction changes.

## Closing Split View

Click the close (X) button in the split view controls. The app in the currently active panel is kept, and the other panel is discarded. The view returns to single-panel mode.

## Configuration

Split view is a client-side feature with no configuration in `config.yaml`. It is available on all navigation positions and works with any combination of apps.

Split view is not available on mobile viewports (below 640px) where the screen is too narrow to display two panels effectively.

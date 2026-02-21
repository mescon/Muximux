# Keyboard Shortcuts

Muximux includes a set of keyboard shortcuts for fast navigation and control. All shortcuts can be customized or disabled as needed.

## Default Keyboard Shortcuts

### Navigation

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` | Open Command Palette |
| `S` | Open Settings |
| `?` | Show Keyboard Shortcuts Help |
| `L` | Toggle Logs (press again to return to your app) |
| `H` | Toggle Overview (press again to return to your app) |

### Actions

| Shortcut | Action |
|----------|--------|
| `R` | Refresh Current App |
| `F` | Toggle Fullscreen (hide navigation) |
| `N` | Next App |
| `P` | Previous App |
| `1` - `9` | Jump to App (by assignment or position) |

**Note:** Keyboard shortcuts are only active when the Muximux UI itself is focused. When an iframe app has focus, keystrokes are sent to that app instead. Click outside the iframe or press `Escape` to return focus to Muximux.

## Command Palette

The command palette (opened with `Ctrl+K`) provides quick access to apps and actions without navigating through menus.

The palette supports the following:

- **App search** -- Type an app name to quickly switch to it. Recently used apps appear first.
- **Actions** -- Open Settings, Show Shortcuts, Toggle Fullscreen, Refresh, View Logs, Go Home.
- **Theme commands** -- Set Dark Theme, Set Light Theme, Use System Theme.

Use the **arrow keys** to navigate results, **Enter** to select, and **Escape** to close.

## Customizing Shortcuts

You can customize any keyboard shortcut through the Settings panel:

1. Open **Settings > Keybindings** tab.
2. Click on the shortcut you want to change.
3. Press the new key combination you want to assign.
4. Toggle modifier keys (Ctrl, Alt, Shift, Meta) as needed.
5. You can assign multiple key combinations to a single action.
6. Reset individual bindings or all bindings to their defaults.

Custom keybindings are saved in `config.yaml` under `keybindings.bindings`:

```yaml
keybindings:
  bindings:
    refresh:
      - key: "F5"
    settings:
      - key: ","
        ctrl: true
```

Only customized bindings are stored in the configuration file. Any action that is not listed will continue to use its default shortcut.

## Assigning Number Keys to Apps

By default, keys `1` through `9` switch to apps by their position in the navigation (first app = `1`, second = `2`, etc.). You can override this by assigning a specific number to an app using the `shortcut` field:

```yaml
apps:
  - name: Plex
    url: http://plex:32400
    shortcut: 1              # Always accessible via key "1"

  - name: Sonarr
    url: http://sonarr:8989
    shortcut: 5              # Always accessible via key "5"
```

When a number key is explicitly assigned to an app, it always points to that app regardless of navigation order. Unassigned number keys continue to work positionally for the remaining apps.

You can also assign shortcuts in **Settings > Keybindings** without editing the config file.

---

## Import and Export Keybindings

The Settings panel provides options to export your keybinding configuration as JSON and import it on another Muximux instance. This is useful for keeping a consistent shortcut layout across multiple installations.

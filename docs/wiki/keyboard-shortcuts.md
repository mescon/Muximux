# Keyboard Shortcuts

Muximux includes a set of keyboard shortcuts for fast navigation and control. All shortcuts can be customized or disabled as needed.

## Default Keyboard Shortcuts

### Navigation

| Shortcut | Action |
|----------|--------|
| `/` or `Ctrl+K` or `Ctrl+Shift+P` | Open Command Palette |
| `Ctrl+,` | Open Settings |
| `?` | Show Keyboard Shortcuts Help |
| `L` | View Logs |
| `Alt+H` | Go to Splash Screen |

### Actions

| Shortcut | Action |
|----------|--------|
| `R` | Refresh Current App |
| `F` | Toggle Fullscreen (hide navigation) |
| `Tab` | Next App |
| `Shift+Tab` | Previous App |
| `1` - `9` | Jump to App by Position |

**Note:** Keyboard shortcuts are only active when the Muximux UI itself is focused. When an iframe app has focus, keystrokes are sent to that app instead. Click outside the iframe or press `Escape` to return focus to Muximux.

## Command Palette

The command palette (opened with `/` or `Ctrl+K`) provides quick access to apps and actions without navigating through menus.

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
    search:
      - key: "/"
      - key: "k"
        ctrl: true
    refresh:
      - key: "F5"
```

Only customized bindings are stored in the configuration file. Any action that is not listed will continue to use its default shortcut.

## Disabling Shortcuts Per App

Some apps have their own keyboard shortcuts that may conflict with Muximux's. You can prevent Muximux from capturing keystrokes when a specific app is active by setting `disable_keyboard_shortcuts: true` on that app:

```yaml
apps:
  - name: VS Code
    url: http://localhost:8443
    disable_keyboard_shortcuts: true
```

When this option is enabled for an app, all Muximux keyboard shortcuts are suspended while that app's iframe is focused. Shortcuts resume when you switch to a different app or click outside the iframe.

## Import and Export Keybindings

The Settings panel provides options to export your keybinding configuration as JSON and import it on another Muximux instance. This is useful for keeping a consistent shortcut layout across multiple installations.

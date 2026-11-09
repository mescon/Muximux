# Keyboard Shortcuts

Muximux has keyboard shortcuts for navigation and control. You can customize or disable any shortcut.

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
| `1` - `9` | Jump to App (apps with an assigned shortcut) |

**Note:** Keyboard shortcuts work only when the Muximux UI itself has focus. When an iframe app has focus, the browser sends keystrokes to that app instead. Click outside the iframe, for example on the navigation bar, to return focus to Muximux. `Escape` does not return focus, because the app receives that key too.

## Command Palette

The command palette (opened with `Ctrl+K`) gives quick access to apps and actions, so you do not have to go through menus.

The palette offers:

- **App search** -- Type an app name to quickly switch to it. Recently used apps appear first.
- **Actions** -- Open Settings, Show Shortcuts, Toggle Fullscreen, Refresh, View Logs, Go Home.
- **Theme commands** -- Set Dark Theme, Set Light Theme, Use System Theme.

Use the **arrow keys** to navigate results, **Enter** to select, and **Escape** to close.

## Keyboard Navigation

You can use Muximux without a mouse:

- **Tab** and **Shift+Tab** move between controls. The control with keyboard focus always shows the same visible outline (in the theme's `--border-focus` colour).
- **Dialogs** (Settings, the Discover dialog, confirmations, the onboarding wizard and the others) keep focus inside while they are open. Tab wraps around within the dialog, and you cannot reach the page behind it. Closing a dialog returns focus to the control that opened it. **Escape** closes the topmost dialog. If an icon picker or other sub-dialog is open over a dialog, Escape closes only that sub-dialog.
- **Health status.** Focusing an app in the navigation, or a health dot on the overview, opens its health tooltip; **Escape** hides it. The tooltip's "Check now" action is mouse-only for now.
- **Logs.** You can focus the log list and scroll it with the arrow and Page keys.
- **Command palette.** `Ctrl+K`, then the arrow keys, **Enter** and **Escape** (see above).

Screen readers announce errors as alerts and confirmations as status messages. Icon-only buttons have names. Muximux also honours the operating system's reduced-motion setting. Every bundled theme meets WCAG 2.1 AA contrast. See [Themes > Focus, motion and screen readers](themes.md#focus-motion-and-screen-readers).

## Customizing Shortcuts

You can customize any keyboard shortcut through the Settings panel:

1. Open **Settings > Keybindings** tab.
2. Click on the shortcut you want to change.
3. Press the new key combination you want to assign.
4. Toggle modifier keys (Ctrl, Alt, Shift, Meta) as needed.

You can assign more than one key combination to a single action. You can also reset one binding, or all bindings, to the defaults.

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

Muximux stores only customized bindings in the configuration file. An action that is not in the list keeps its default shortcut.

## Assigning Number Keys to Apps

Keys `1` through `9` switch to apps that have an explicit `shortcut` assignment. During onboarding, Muximux assigns shortcuts to the first 9 apps automatically. After that, you manage them manually. When you assign a number to a new app, Muximux removes it from the app that held it before.

```yaml
apps:
  - name: Plex
    url: http://plex:32400
    shortcut: 1              # Accessible via key "1"

  - name: Sonarr
    url: http://sonarr:8989
    shortcut: 5              # Accessible via key "5"
```

Only apps with an explicit `shortcut` value respond to number keys. Apps without a shortcut have no number key binding. The splash screen badges match this: only apps with assigned shortcuts show a number badge.

You can also assign shortcuts in **Settings > Keybindings** without editing the config file.

---

## Import and Export Keybindings

In the Settings panel, you can export your keybinding configuration as JSON. You can then import it on another Muximux instance. This keeps the same shortcut layout across multiple installations.

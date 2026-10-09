import type { App, Config, HealthConfig, KeybindingsConfig, ThemeConfig } from './types';

/**
 * What the shell looks like when a config arrives. App.svelte passes an
 * object whose getters read its live state, so a fetch that resolves later
 * still sees the current panels and dialogs.
 */
export interface ShellState {
  config: Config | null;
  apps: App[];
  panels: (App | null)[];
  showLogs: boolean;
  showSettings: boolean;
  visited: Set<string>;
}

export interface ShellActions {
  /** Sets the shell's config and apps. */
  setConfig: (c: Config) => void;
  /** Empties one split panel. */
  clearPanel: (i: number) => void;
  /** Resets the split and shows the overview. */
  showSplash: () => void;
}

export interface ApplyDeps {
  syncTheme: (t: ThemeConfig) => void;
  initKeybindings: (k?: KeybindingsConfig) => void;
  restartHealth: (h?: HealthConfig) => void;
  /** Applies the server locale; true when it started a page reload. */
  applyLocale: (language?: string) => boolean;
}

/**
 * The one copy of the shell's apply step, used by the push path and by the
 * save path. Returns true when applyLocale started a reload.
 *
 * While Settings is open, the theme and keybinding stores hold the dialog's
 * unsaved edits and a reload would throw them away, so theme, keybindings
 * and locale are left to the dialog (its rebase applies untouched
 * keybindings; its save comes back through here with showSettings false).
 */
export function applyConfigToShell(next: Config, state: ShellState, actions: ShellActions, deps: ApplyDeps): boolean {
  const { showLogs, showSettings, visited } = state;
  actions.setConfig(next);
  const apps = next.apps ?? [];

  if (!showSettings) {
    if (next.theme) deps.syncTheme(next.theme);
    deps.initKeybindings(next.keybindings);
  }
  deps.restartHealth(next.health);

  // Clear a panel only when its app is gone.
  const names = new Set(apps.map(a => a.name));
  let allEmpty = true;
  state.panels.forEach((p, i) => {
    if (!p) return;
    if (names.has(p.name)) allEmpty = false;
    else actions.clearPanel(i);
  });
  if (allEmpty && !showLogs && !showSettings) actions.showSplash();

  // Drop cached iframes of apps that were removed or disabled.
  const enabled = new Set(apps.filter(a => a.enabled).map(a => a.name));
  for (const name of [...visited]) {
    if (!enabled.has(name)) visited.delete(name);
  }

  if (showSettings) return false;
  return deps.applyLocale(next.language);
}

// Bumped per fetch, so only the newest of overlapping fetches applies.
let fetchSeq = 0;

/**
 * Fetches the server config and applies it. Returns null and leaves the
 * shell alone when the fetch fails or a newer fetch started meanwhile.
 */
export async function applyServerConfig(
  fetchConfig: () => Promise<Config>,
  state: ShellState,
  actions: ShellActions,
  deps: ApplyDeps,
): Promise<Config | null> {
  const seq = ++fetchSeq;
  let next: Config;
  try {
    next = await fetchConfig();
  } catch {
    return null;
  }
  if (seq !== fetchSeq) return null;
  applyConfigToShell(next, state, actions, deps);
  return next;
}

/**
 * The cleared-hash route (e.g. the user navigated to /). Goes home only when
 * Settings actually closed; a discard prompt or an in-flight save keeps the
 * whole view as it is. Returns whether it went home.
 */
export function homeOnClearedHash(closeSettings: () => boolean, goHome: () => void): boolean {
  if (!closeSettings()) return false;
  goHome();
  return true;
}

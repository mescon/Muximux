import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { App, Config } from './types';
import { applyConfigToShell, applyServerConfig, homeOnClearedHash, reapplyDeferredPrefs, type ShellState, type ShellActions, type ApplyDeps } from './configSync';

function makeApp(name: string, enabled = true): App {
  return {
    name, url: `https://${name}.example`, icon: { type: 'dashboard', name: 'x' }, color: '#22c55e',
    group: '', order: 0, enabled, default: false, open_mode: 'iframe', proxy: false, scale: 1,
  } as App;
}

function makeConfig(apps: App[], extra: Partial<Config> = {}): Config {
  return {
    title: 'Muximux',
    navigation: { position: 'top' } as Config['navigation'],
    groups: [],
    apps,
    ...extra,
  };
}

let state: ShellState;
let actions: ShellActions & { setConfig: ReturnType<typeof vi.fn>; clearPanel: ReturnType<typeof vi.fn>; showSplash: ReturnType<typeof vi.fn> };
let deps: { [K in keyof ApplyDeps]: ReturnType<typeof vi.fn> };

beforeEach(() => {
  const a = makeApp('a');
  const b = makeApp('b');
  state = {
    config: makeConfig([a, b]),
    apps: [a, b],
    panels: [a, b],
    showLogs: false,
    showSettings: false,
    visited: new Set(['a', 'b']),
  };
  actions = {
    setConfig: vi.fn((c: Config) => { state.config = c; state.apps = c.apps; }),
    clearPanel: vi.fn((i: number) => { state.panels[i] = null; }),
    showSplash: vi.fn(),
  };
  deps = {
    syncTheme: vi.fn(),
    initKeybindings: vi.fn(),
    restartHealth: vi.fn(),
    applyLocale: vi.fn(() => false),
  };
});

describe('applyConfigToShell', () => {
  it('does not force the splash while Logs is open', () => {
    state.showLogs = true;
    applyConfigToShell(makeConfig([]), state, actions, deps);
    expect(actions.showSplash).not.toHaveBeenCalled();
  });

  it('does not force the splash while Settings is open', () => {
    state.showSettings = true;
    applyConfigToShell(makeConfig([]), state, actions, deps);
    expect(actions.showSplash).not.toHaveBeenCalled();
  });

  it('shows the splash when both panels end up empty and nothing else is open', () => {
    applyConfigToShell(makeConfig([]), state, actions, deps);
    expect(actions.clearPanel).toHaveBeenCalledWith(0);
    expect(actions.clearPanel).toHaveBeenCalledWith(1);
    expect(actions.showSplash).toHaveBeenCalledTimes(1);
  });

  it('does not show the splash while a panel still has an app', () => {
    applyConfigToShell(makeConfig([makeApp('a')]), state, actions, deps);
    expect(actions.showSplash).not.toHaveBeenCalled();
  });

  it('clears only the panel whose app vanished', () => {
    applyConfigToShell(makeConfig([makeApp('a')]), state, actions, deps);
    expect(actions.clearPanel).toHaveBeenCalledTimes(1);
    expect(actions.clearPanel).toHaveBeenCalledWith(1);
  });

  it('keeps a panel whose app was only disabled', () => {
    applyConfigToShell(makeConfig([makeApp('a'), makeApp('b', false)]), state, actions, deps);
    expect(actions.clearPanel).not.toHaveBeenCalled();
  });

  it('prunes visited names for removed and disabled apps', () => {
    state.visited.add('c');
    applyConfigToShell(makeConfig([makeApp('a'), makeApp('b', false)]), state, actions, deps);
    expect([...state.visited]).toEqual(['a']);
  });

  it('sets the config through setConfig', () => {
    const next = makeConfig([makeApp('a')]);
    applyConfigToShell(next, state, actions, deps);
    expect(actions.setConfig).toHaveBeenCalledWith(next);
  });

  it('re-applies theme, keybindings and health', () => {
    const theme = { family: 'nord', variant: 'dark' as const };
    const keybindings = { bindings: {} };
    const health = { enabled: true, interval: '10s', timeout: '5s' };
    applyConfigToShell(makeConfig([makeApp('a')], { theme, keybindings, health }), state, actions, deps);
    expect(deps.syncTheme).toHaveBeenCalledWith(theme);
    expect(deps.initKeybindings).toHaveBeenCalledWith(keybindings);
    expect(deps.restartHealth).toHaveBeenCalledWith(health);
  });

  it('restarts health polling with a saved interval or enabled flag (save path)', () => {
    const saveState = { ...state, showSettings: false };
    applyConfigToShell(makeConfig([makeApp('a')], { health: { enabled: true, interval: '5m', timeout: '5s' } }), saveState, actions, deps);
    expect(deps.restartHealth).toHaveBeenLastCalledWith({ enabled: true, interval: '5m', timeout: '5s' });
    applyConfigToShell(makeConfig([makeApp('a')], { health: { enabled: false, interval: '5m', timeout: '5s' } }), saveState, actions, deps);
    expect(deps.restartHealth).toHaveBeenLastCalledWith({ enabled: false, interval: '5m', timeout: '5s' });
  });

  it('skips the theme sync when the config has no theme', () => {
    applyConfigToShell(makeConfig([makeApp('a')]), state, actions, deps);
    expect(deps.syncTheme).not.toHaveBeenCalled();
  });

  it('passes the language to applyLocale and reports a reload', () => {
    deps.applyLocale.mockReturnValue(true);
    const reloading = applyConfigToShell(makeConfig([makeApp('a')], { language: 'sv' }), state, actions, deps);
    expect(deps.applyLocale).toHaveBeenCalledWith('sv');
    expect(reloading).toBe(true);
  });

  it('applies the locale last', () => {
    const order: string[] = [];
    actions.setConfig.mockImplementation(() => order.push('config'));
    actions.showSplash.mockImplementation(() => order.push('splash'));
    deps.restartHealth.mockImplementation(() => order.push('health'));
    deps.applyLocale.mockImplementation(() => { order.push('locale'); return false; });
    applyConfigToShell(makeConfig([]), state, actions, deps);
    expect(order[order.length - 1]).toBe('locale');
  });

  it('leaves theme, keybindings and locale to an open Settings dialog', () => {
    state.showSettings = true;
    deps.applyLocale.mockReturnValue(true);
    const reloading = applyConfigToShell(
      makeConfig([makeApp('a')], { theme: { family: 'nord', variant: 'dark' }, language: 'sv' }),
      state, actions, deps,
    );
    expect(deps.syncTheme).not.toHaveBeenCalled();
    expect(deps.initKeybindings).not.toHaveBeenCalled();
    expect(deps.applyLocale).not.toHaveBeenCalled();
    expect(deps.restartHealth).toHaveBeenCalled();
    expect(reloading).toBe(false);
  });

  it('treats a config without apps as empty', () => {
    const next = makeConfig([]);
    (next as { apps?: App[] }).apps = undefined;
    applyConfigToShell(next, state, actions, deps);
    expect(actions.showSplash).toHaveBeenCalled();
    expect(state.visited.size).toBe(0);
  });
});

describe('applyServerConfig', () => {
  it('fetches and applies the server config', async () => {
    const next = makeConfig([makeApp('a')]);
    const result = await applyServerConfig(() => Promise.resolve(next), state, actions, deps);
    expect(result).toBe(next);
    expect(actions.setConfig).toHaveBeenCalledWith(next);
  });

  it('returns null and leaves state alone when fetch fails', async () => {
    const before = state.config;
    const result = await applyServerConfig(() => Promise.reject(new Error('500')), state, actions, deps);
    expect(result).toBeNull();
    expect(state.config).toBe(before);
    expect(actions.setConfig).not.toHaveBeenCalled();
    expect(actions.clearPanel).not.toHaveBeenCalled();
    expect(deps.restartHealth).not.toHaveBeenCalled();
  });

  it('applies only the newest of overlapping fetches', async () => {
    let resolveOld: (c: Config) => void = () => {};
    const older = makeConfig([makeApp('old')]);
    const newer = makeConfig([makeApp('a')]);
    const first = applyServerConfig(() => new Promise<Config>(r => { resolveOld = r; }), state, actions, deps);
    const second = applyServerConfig(() => Promise.resolve(newer), state, actions, deps);
    expect(await second).toBe(newer);
    resolveOld(older);
    expect(await first).toBeNull();
    expect(actions.setConfig).toHaveBeenCalledTimes(1);
    expect(actions.setConfig).toHaveBeenCalledWith(newer);
  });
});

describe('applyServerConfig after a direct apply', () => {
  it('drops a fetch that started before a save was applied', async () => {
    let resolvePush: (c: Config) => void = () => {};
    const pending = applyServerConfig(() => new Promise<Config>(r => { resolvePush = r; }), state, actions, deps);
    const saved = makeConfig([makeApp('a')]);
    applyConfigToShell(saved, { ...state, showSettings: false }, actions, deps);
    resolvePush(makeConfig([makeApp('stale')]));
    expect(await pending).toBeNull();
    expect(actions.setConfig).toHaveBeenCalledTimes(1);
    expect(actions.setConfig).toHaveBeenCalledWith(saved);
  });
});

describe('reapplyDeferredPrefs', () => {
  it('applies theme, keybindings and locale and reports a reload', () => {
    deps.applyLocale.mockReturnValue(true);
    const theme = { family: 'nord', variant: 'dark' as const };
    const keybindings = { bindings: {} };
    const reloading = reapplyDeferredPrefs(makeConfig([], { theme, keybindings, language: 'sv' }), deps);
    expect(deps.syncTheme).toHaveBeenCalledWith(theme);
    expect(deps.initKeybindings).toHaveBeenCalledWith(keybindings);
    expect(deps.applyLocale).toHaveBeenCalledWith('sv');
    expect(reloading).toBe(true);
  });

  it('skips the theme when the config has none', () => {
    reapplyDeferredPrefs(makeConfig([]), deps);
    expect(deps.syncTheme).not.toHaveBeenCalled();
    expect(deps.initKeybindings).toHaveBeenCalled();
  });
});

describe('homeOnClearedHash', () => {
  it('goes home when Settings closed', () => {
    const goHome = vi.fn();
    expect(homeOnClearedHash(() => true, goHome)).toBe(true);
    expect(goHome).toHaveBeenCalled();
  });

  it('changes nothing when the close was blocked', () => {
    const goHome = vi.fn();
    expect(homeOnClearedHash(() => false, goHome)).toBe(false);
    expect(goHome).not.toHaveBeenCalled();
  });
});

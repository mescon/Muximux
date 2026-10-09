import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import type { App, Config } from './lib/types';

// App shell wiring around configSync: the login path, the save path, live
// pushes and the cleared-hash route. Heavy children are stubbed.

const api = vi.hoisted(() => ({
  fetchConfig: vi.fn(),
  saveConfig: vi.fn(),
  submitSetup: vi.fn(),
  fetchSystemInfo: vi.fn(() => Promise.resolve({ version: 'test' })),
  fireAppAction: vi.fn(),
}));
vi.mock('./lib/api', () => api);

const health = vi.hoisted(() => ({ restartHealthPolling: vi.fn(), stopHealthPolling: vi.fn() }));
vi.mock('./lib/healthStore', () => health);

const locale = vi.hoisted(() => ({ applyConfigLocale: vi.fn(() => false) }));
vi.mock('./lib/localeStore', async (orig) => ({ ...(await orig<object>()), ...locale }));

const ws = vi.hoisted(() => ({
  handlers: new Map<string, (p: unknown) => void>(),
  reconnect: [] as (() => void)[],
}));
vi.mock('./lib/websocketStore', async () => {
  const { writable: w } = await import('svelte/store');
  return {
    connect: vi.fn(),
    disconnect: vi.fn(),
    on: vi.fn((type: string, h: (p: unknown) => void) => { ws.handlers.set(type, h); return () => {}; }),
    onReconnect: vi.fn((h: () => void) => { ws.reconnect.push(h); return () => {}; }),
    connectionState: w('disconnected'),
  };
});

const docker = vi.hoisted(() => ({ refreshDockerState: vi.fn(() => Promise.resolve()) }));
vi.mock('./lib/dockerStateStore', async (orig) => ({ ...(await orig<object>()), ...docker }));

vi.mock('./lib/logStore', () => ({ initLogStore: vi.fn() }));

vi.mock('./lib/themeStore', async (orig) => ({
  ...(await orig<object>()),
  loadCustomThemesFromServer: vi.fn(),
  syncFromConfig: vi.fn(),
}));

const auth = vi.hoisted(() => ({ authenticated: null as null | { set: (v: boolean) => void } }));
vi.mock('./lib/authStore', async () => {
  const { writable: w } = await import('svelte/store');
  const isAuthenticated = w(false);
  auth.authenticated = isAuthenticated;
  return {
    checkAuthStatus: vi.fn(() => Promise.resolve()),
    isAuthenticated,
    isAdmin: w(true),
    setupRequired: w(false),
  };
});

vi.mock('./components/Login.svelte', async () => ({ default: (await import('./test/shell/LoginStub.svelte')).default }));
vi.mock('./components/Splash.svelte', async () => ({ default: (await import('./test/shell/SplashStub.svelte')).default }));
vi.mock('./components/Navigation.svelte', async () => ({ default: (await import('./test/shell/NavigationStub.svelte')).default }));
vi.mock('./components/AppFrame.svelte', async () => ({ default: (await import('./test/shell/AppFrameStub.svelte')).default }));
vi.mock('./components/Settings.svelte', async () => ({ default: (await import('./test/shell/SettingsStub.svelte')).default }));

import AppShell from './App.svelte';

function makeApp(name: string): App {
  return {
    name, url: `https://${name}.example`, icon: { type: 'dashboard', name: 'x' }, color: '#22c55e',
    group: '', order: 0, enabled: true, default: false, open_mode: 'iframe', proxy: false, scale: 1,
  } as App;
}

function makeConfig(extra: Partial<Config> = {}): Config {
  return {
    title: 'Muximux',
    navigation: { position: 'top', show_splash_on_startup: true } as Config['navigation'],
    auth: { method: 'builtin' } as Config['auth'],
    groups: [],
    apps: [makeApp('a'), makeApp('b')],
    ...extra,
  };
}

function setHash(hash: string) {
  history.replaceState(null, '', hash ? `/${hash}` : '/');
  globalThis.dispatchEvent(new HashChangeEvent('hashchange'));
}

// Mounts the shell behind the login screen and logs in.
async function mountAndLogin(config: Config) {
  api.fetchConfig.mockRejectedValueOnce(new Error('401 Unauthorized'));
  api.fetchConfig.mockResolvedValueOnce(config);
  render(AppShell);
  const button = await screen.findByTestId('login-stub');
  // The real Login marks the session authenticated before onsuccess.
  await fireEvent.click(button);
  auth.authenticated!.set(true);
}

async function loggedIn(config = makeConfig()) {
  await mountAndLogin(config);
  await screen.findByTestId('splash');
}

beforeEach(() => {
  vi.clearAllMocks();
  locale.applyConfigLocale.mockReturnValue(false);
  ws.handlers.clear();
  auth.authenticated?.set(false);
  ws.reconnect.length = 0;
  (globalThis as Record<string, unknown>).__settingsBlockClose = false;
  history.replaceState(null, '', '/');
});

// App adds window listeners it never removes; drop them after each test so
// an unmounted shell does not react to the next test's hash changes.
let windowListeners: [string, EventListenerOrEventListenerObject][] = [];
const addListener = globalThis.addEventListener.bind(globalThis);
beforeEach(() => {
  windowListeners = [];
  vi.spyOn(globalThis, 'addEventListener').mockImplementation(((type: string, l: EventListenerOrEventListenerObject, o?: AddEventListenerOptions) => {
    windowListeners.push([type, l]);
    addListener(type, l, o);
  }) as typeof globalThis.addEventListener);
});

afterEach(() => {
  cleanup();
  for (const [type, l] of windowListeners) globalThis.removeEventListener(type, l);
  vi.restoreAllMocks();
});

describe('App login path', () => {
  it('applies the config locale after login and stops when it reloads', async () => {
    locale.applyConfigLocale.mockReturnValue(true);
    await mountAndLogin(makeConfig({ language: 'sv' }));
    await waitFor(() => expect(locale.applyConfigLocale).toHaveBeenCalledWith('sv'));
    expect(health.restartHealthPolling).not.toHaveBeenCalled();
  });

  it('registers the live config listener after login', async () => {
    await loggedIn();
    expect(ws.handlers.has('config_updated')).toBe(true);
    expect(ws.reconnect).toHaveLength(1);
  });
});

describe('App save path', () => {
  it('restarts health polling with the saved interval and enabled flag', async () => {
    await loggedIn();
    setHash('#settings');
    await screen.findByTestId('settings', {}, { timeout: 5000 });
    const savedHealth = { enabled: false, interval: '5m', timeout: '5s' };
    api.saveConfig.mockResolvedValueOnce(makeConfig({ health: savedHealth }));
    await fireEvent.click(screen.getByTestId('settings-save'));
    await waitFor(() => expect(health.restartHealthPolling).toHaveBeenLastCalledWith(savedHealth));
  });
});

describe('App live push', () => {
  it('refetches on config_updated and shows the splash when the open app vanished', async () => {
    await loggedIn();
    setHash('#a');
    await screen.findByTestId('frame-a');
    await waitFor(() => expect(screen.queryByTestId('splash')).toBeNull());

    const pushed = makeConfig({ apps: [makeApp('b')], health: { enabled: true, interval: '1m', timeout: '5s' } });
    api.fetchConfig.mockResolvedValueOnce(pushed);
    ws.handlers.get('config_updated')!({});
    await screen.findByTestId('splash');
    expect(health.restartHealthPolling).toHaveBeenLastCalledWith(pushed.health);
    expect(screen.queryByTestId('frame-a')).toBeNull();
  });

  it('resyncs config and docker state on reconnect', async () => {
    await loggedIn();
    api.fetchConfig.mockResolvedValueOnce(makeConfig());
    const calls = api.fetchConfig.mock.calls.length;
    ws.reconnect[0]();
    await waitFor(() => expect(api.fetchConfig.mock.calls.length).toBe(calls + 1));
    expect(docker.refreshDockerState).toHaveBeenCalled();
  });
});

describe('App cleared hash', () => {
  it('keeps the view when Settings blocks the close, goes home once it closes', async () => {
    await loggedIn();
    setHash('#a');
    await screen.findByTestId('frame-a');
    setHash('#settings');
    await screen.findByTestId('settings', {}, { timeout: 5000 });

    (globalThis as Record<string, unknown>).__settingsBlockClose = true;
    setHash('');
    await Promise.resolve();
    expect(screen.getByTestId('settings')).toBeTruthy();
    expect(screen.queryByTestId('splash')).toBeNull();

    (globalThis as Record<string, unknown>).__settingsBlockClose = false;
    setHash('');
    await screen.findByTestId('splash');
    expect(screen.queryByTestId('settings')).toBeNull();
  });
});

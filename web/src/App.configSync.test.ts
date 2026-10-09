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
  // Every on() registration, duplicates included.
  registrations: [] as string[],
  reconnect: [] as (() => void)[],
}));
vi.mock('./lib/websocketStore', async () => {
  const { writable: w } = await import('svelte/store');
  return {
    connect: vi.fn(),
    disconnect: vi.fn(),
    on: vi.fn((type: string, h: (p: unknown) => void) => {
      ws.registrations.push(type);
      ws.handlers.set(type, h);
      return () => {};
    }),
    onReconnect: vi.fn((h: () => void) => { ws.reconnect.push(h); return () => {}; }),
    connectionState: w('disconnected'),
  };
});

const docker = vi.hoisted(() => ({ refreshDockerState: vi.fn(() => Promise.resolve()) }));
vi.mock('./lib/dockerStateStore', async (orig) => ({ ...(await orig<object>()), ...docker }));

vi.mock('./lib/logStore', () => ({ initLogStore: vi.fn() }));

const theme = vi.hoisted(() => ({ loadCustomThemesFromServer: vi.fn(), syncFromConfig: vi.fn() }));
vi.mock('./lib/themeStore', async (orig) => ({ ...(await orig<object>()), ...theme }));

type BoolStore = { set: (v: boolean) => void };
const auth = vi.hoisted(() => ({ authenticated: null as null | BoolStore, setupRequired: null as null | BoolStore }));
vi.mock('./lib/authStore', async () => {
  const { writable: w } = await import('svelte/store');
  const isAuthenticated = w(false);
  const setupRequired = w(false);
  auth.authenticated = isAuthenticated;
  auth.setupRequired = setupRequired;
  return {
    checkAuthStatus: vi.fn(() => Promise.resolve()),
    isAuthenticated,
    isAdmin: w(true),
    setupRequired,
  };
});

vi.mock('./components/Login.svelte', async () => ({ default: (await import('./test/shell/LoginStub.svelte')).default }));
vi.mock('./components/Splash.svelte', async () => ({ default: (await import('./test/shell/SplashStub.svelte')).default }));
vi.mock('./components/Navigation.svelte', async () => ({ default: (await import('./test/shell/NavigationStub.svelte')).default }));
vi.mock('./components/AppFrame.svelte', async () => ({ default: (await import('./test/shell/AppFrameStub.svelte')).default }));
vi.mock('./components/OnboardingWizard.svelte', async () => ({ default: (await import('./test/shell/OnboardingStub.svelte')).default }));
vi.mock('./components/Settings.svelte', async () => ({ default: (await import('./test/shell/SettingsStub.svelte')).default }));

import AppShell from './App.svelte';
import { toasts } from './lib/toastStore';

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
  ws.registrations.length = 0;
  (globalThis as Record<string, unknown>).__settingsConfigs = [];
  auth.authenticated?.set(false);
  auth.setupRequired?.set(false);
  api.fetchConfig.mockReset();
  api.saveConfig.mockReset();
  api.submitSetup.mockReset();
  delete (globalThis as Record<string, unknown>).__onboardingDetail;
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

const g = globalThis as Record<string, unknown>;

async function openSettings() {
  setHash('#settings');
  await screen.findByTestId('settings', {}, { timeout: 5000 });
}

function liveRegistrations() {
  return ws.registrations.filter(t => t === 'config_updated').length;
}

describe('App onboarding path', () => {
  // First run: setup is required, so the shell opens the wizard straight
  // away. submitSetup lowers the guard the way the real server does.
  async function firstRun(loaded: Config) {
    auth.setupRequired!.set(true);
    api.submitSetup.mockImplementation(async () => {
      auth.setupRequired!.set(false);
      return { success: true };
    });
    api.fetchConfig.mockResolvedValue(loaded);
    render(AppShell);
    return screen.findByTestId('onboarding-done', {}, { timeout: 5000 });
  }

  it('registers live sync after first-run onboarding', async () => {
    const empty = makeConfig({ apps: [], auth: { method: 'none' } as Config['auth'] });
    api.saveConfig.mockResolvedValueOnce(makeConfig({ auth: { method: 'none' } as Config['auth'] }));
    await fireEvent.click(await firstRun(empty));
    await waitFor(() => expect(liveRegistrations()).toBe(1));
    expect(ws.reconnect).toHaveLength(1);
    expect(api.submitSetup).toHaveBeenCalledTimes(1);
  });

  it('does not open the wizard for a configured instance with zero apps', async () => {
    await loggedIn(makeConfig({ apps: [] }));
    expect(screen.queryByTestId('onboarding-done')).toBeNull();
  });

  it('merges the wizard picks onto the loaded config and sends it as base', async () => {
    const loaded = makeConfig({
      language: 'en',
      groups: [{ name: 'Ops', icon: { type: 'dashboard', name: 'x' }, color: '#111111', order: 0, expanded: true }] as Config['groups'],
      apps: [{ ...makeApp('grafana'), group: 'Ops' }],
      discovery: { docker: { enabled: true, endpoint: 'tcp://d:2376', tls: { enabled: true }, network_strategy: 'container_dns', refresh_interval: '30s', auto_import: 'add' } },
    });
    g.__onboardingDetail = {
      apps: [{ ...makeApp('plex'), group: 'Media' }],
      groups: [{ name: 'Media', icon: { type: 'dashboard', name: 'x' }, color: '#222222', order: 0, expanded: true }],
      navigation: { position: 'left' },
      theme: { family: 'nord', variant: 'light' },
      language: 'sv',
      setup: { method: 'none' },
    };
    api.saveConfig.mockImplementationOnce(async (c: Config) => c);
    await fireEvent.click(await firstRun(loaded));
    await waitFor(() => expect(api.saveConfig).toHaveBeenCalledTimes(1));

    const [sent, base] = api.saveConfig.mock.calls[0] as [Config, Config];
    expect(base).toEqual(loaded);
    expect(sent.groups.map(gr => gr.name)).toEqual(['Ops', 'Media']);
    expect(sent.apps.map(a => a.name)).toEqual(['grafana', 'plex']);
    expect(sent.navigation).toEqual({ ...loaded.navigation, position: 'left' });
    expect(sent.theme).toEqual({ family: 'nord', variant: 'light' });
    expect(sent.language).toBe('sv');
    expect(sent.discovery).toEqual(loaded.discovery);
  });

  it('retries a failed config save without submitting setup again', async () => {
    api.saveConfig.mockRejectedValueOnce(new Error('API error: 500'));
    api.saveConfig.mockResolvedValueOnce(makeConfig());
    const done = await firstRun(makeConfig({ apps: [] }));
    await fireEvent.click(done);
    await waitFor(() => expect(api.saveConfig).toHaveBeenCalledTimes(1));
    expect(screen.getByTestId('onboarding-done')).toBeTruthy();

    await fireEvent.click(screen.getByTestId('onboarding-done'));
    await waitFor(() => expect(api.saveConfig).toHaveBeenCalledTimes(2));
    expect(api.submitSetup).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByTestId('onboarding-done')).toBeNull());
  });

  it('keeps the setup-time wizard restrictions while a failed save is retried', async () => {
    api.saveConfig.mockRejectedValueOnce(new Error('API error: 500'));
    const done = await firstRun(makeConfig({ apps: [] }));
    expect(done.dataset.needsSetup).toBe('true');
    await fireEvent.click(done);
    await waitFor(() => expect(api.saveConfig).toHaveBeenCalledTimes(1));
    // submitSetup lowered setupRequired, but the setup flow is still open.
    expect(screen.getByTestId('onboarding-done').dataset.needsSetup).toBe('true');
  });

  it('treats a 409 from setup as already done and saves the config', async () => {
    const error = vi.spyOn(toasts, 'error');
    const done = await firstRun(makeConfig({ apps: [] }));
    api.submitSetup.mockImplementationOnce(async () => {
      auth.setupRequired!.set(false);
      // Setup finished elsewhere without auth: this browser has a session.
      auth.authenticated!.set(true);
      throw new Error('API error: 409 {"error":"Setup already completed"}');
    });
    api.saveConfig.mockResolvedValueOnce(makeConfig());
    await fireEvent.click(done);
    await waitFor(() => expect(api.saveConfig).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.queryByTestId('onboarding-done')).toBeNull());
    await screen.findByTestId('splash');
    expect(error).not.toHaveBeenCalled();
  });

  it('sends the user to sign in when setup was completed elsewhere with auth', async () => {
    const error = vi.spyOn(toasts, 'error');
    const info = vi.spyOn(toasts, 'info');
    const done = await firstRun(makeConfig({ apps: [] }));
    api.submitSetup.mockImplementationOnce(async () => {
      auth.setupRequired!.set(false);
      throw new Error('API error: 409 {"error":"Setup already completed"}');
    });
    await fireEvent.click(done);
    await screen.findByTestId('login-stub');
    expect(screen.queryByTestId('onboarding-done')).toBeNull();
    expect(info).toHaveBeenCalledWith('Setup was completed elsewhere. Sign in to continue.');
    expect(error).not.toHaveBeenCalled();
    expect(api.fetchConfig).not.toHaveBeenCalled();
    expect(api.saveConfig).not.toHaveBeenCalled();
  });

  it('stops on any other setup error', async () => {
    const done = await firstRun(makeConfig({ apps: [] }));
    api.submitSetup.mockRejectedValueOnce(new Error('API error: 403 invalid setup token'));
    await fireEvent.click(done);
    await waitFor(() => expect(api.submitSetup).toHaveBeenCalledTimes(1));
    await new Promise(r => setTimeout(r, 0));
    expect(api.saveConfig).not.toHaveBeenCalled();
  });

  it('stops when setup reports failure', async () => {
    const done = await firstRun(makeConfig({ apps: [] }));
    api.submitSetup.mockResolvedValueOnce({ success: false, error: 'bad password' });
    await fireEvent.click(done);
    await waitFor(() => expect(api.submitSetup).toHaveBeenCalledTimes(1));
    await new Promise(r => setTimeout(r, 0));
    expect(api.saveConfig).not.toHaveBeenCalled();
  });
});

describe('App live push while Settings is open', () => {
  it('hands the pushed config to Settings and holds back theme and locale', async () => {
    await loggedIn();
    await openSettings();
    theme.syncFromConfig.mockClear();
    locale.applyConfigLocale.mockClear();

    const pushed = makeConfig({ language: 'sv', theme: { family: 'nord', variant: 'dark' } });
    api.fetchConfig.mockResolvedValueOnce(pushed);
    ws.handlers.get('config_updated')!({});
    await waitFor(() => expect(g.__settingsConfigs as unknown[]).toContainEqual(pushed));
    expect(theme.syncFromConfig).not.toHaveBeenCalled();
    expect(locale.applyConfigLocale).not.toHaveBeenCalled();
    expect(health.restartHealthPolling).toHaveBeenCalled();
  });

  it('applies the held-back language when Settings is discarded', async () => {
    await loggedIn();
    await openSettings();
    const pushed = makeConfig({ language: 'sv', theme: { family: 'nord', variant: 'dark' } });
    api.fetchConfig.mockResolvedValueOnce(pushed);
    ws.handlers.get('config_updated')!({});
    await waitFor(() => expect(g.__settingsConfigs as unknown[]).toContainEqual(pushed));
    expect(locale.applyConfigLocale).not.toHaveBeenCalledWith('sv');

    await fireEvent.click(screen.getByTestId('settings-discard'));
    expect(locale.applyConfigLocale).toHaveBeenCalledWith('sv');
    expect(theme.syncFromConfig).toHaveBeenLastCalledWith(pushed.theme);
  });

  it('does not re-apply prefs on a clean close without a push', async () => {
    await loggedIn();
    await openSettings();
    locale.applyConfigLocale.mockClear();
    theme.syncFromConfig.mockClear();
    await fireEvent.click(screen.getByTestId('settings-discard'));
    expect(locale.applyConfigLocale).not.toHaveBeenCalled();
    expect(theme.syncFromConfig).not.toHaveBeenCalled();
  });

  it('a push refetch that started before a save cannot overwrite the saved config', async () => {
    await loggedIn();
    await openSettings();
    let resolvePush: (c: Config) => void = () => {};
    api.fetchConfig.mockImplementationOnce(() => new Promise<Config>(r => { resolvePush = r; }));
    ws.handlers.get('config_updated')!({});

    const savedHealth = { enabled: true, interval: '2m', timeout: '5s' };
    api.saveConfig.mockResolvedValueOnce(makeConfig({ health: savedHealth }));
    await fireEvent.click(screen.getByTestId('settings-save'));
    await waitFor(() => expect(health.restartHealthPolling).toHaveBeenLastCalledWith(savedHealth));

    resolvePush(makeConfig({ health: { enabled: true, interval: '9m', timeout: '5s' } }));
    await new Promise(r => setTimeout(r, 0));
    expect(health.restartHealthPolling).toHaveBeenLastCalledWith(savedHealth);
  });
});

describe('App logout and login', () => {
  it('keeps exactly one live listener and does no extra refetch', async () => {
    await loggedIn();
    expect(liveRegistrations()).toBe(1);

    await fireEvent.click(screen.getByTestId('logout'));
    auth.authenticated!.set(false);
    const button = await screen.findByTestId('login-stub');
    const fetches = api.fetchConfig.mock.calls.length;
    api.fetchConfig.mockResolvedValueOnce(makeConfig());
    await fireEvent.click(button);
    auth.authenticated!.set(true);
    await screen.findByTestId('splash');

    expect(api.fetchConfig.mock.calls.length).toBe(fetches + 1);
    expect(liveRegistrations()).toBe(1);
    expect(ws.reconnect).toHaveLength(1);
    expect(docker.refreshDockerState).not.toHaveBeenCalled();
  });
});

describe('App cleared hash while Settings is blocked', () => {
  it('restores #settings in the URL without another hashchange', async () => {
    await loggedIn();
    await openSettings();
    g.__settingsBlockClose = true;
    const onHash = vi.fn();
    addListener('hashchange', onHash);
    setHash('');
    await Promise.resolve();
    expect(location.hash).toBe('#settings');
    expect(onHash).toHaveBeenCalledTimes(1); // only the test's own event
    globalThis.removeEventListener('hashchange', onHash);
    expect(screen.getByTestId('settings')).toBeTruthy();
  });
});

describe('App hash change to an app while Settings is open', () => {
  it('keeps Settings and #settings when the close is blocked, and does not select the app', async () => {
    await loggedIn();
    setHash('#a');
    await screen.findByTestId('frame-a');
    await openSettings();
    // Browser back from #settings to #a with unsaved edits.
    g.__settingsBlockClose = true;
    setHash('#b');
    await new Promise(r => setTimeout(r, 0));
    expect(location.hash).toBe('#settings');
    expect(screen.getByTestId('settings')).toBeTruthy();
    expect(screen.queryByTestId('frame-b')).toBeNull();
  });

  it('closes Settings and selects the app once nothing is unsaved', async () => {
    await loggedIn();
    await openSettings();
    setHash('#b');
    await screen.findByTestId('frame-b');
    expect(screen.queryByTestId('settings')).toBeNull();
    expect(location.hash).toBe('#b');
  });
});

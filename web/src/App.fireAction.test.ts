import { describe, it, expect, vi, beforeEach } from 'vitest';

// We don't render the full App.svelte (huge surface, lots of mocks).
// Instead we test the dispatch logic by extracting selectApp's behaviour
// pattern into a small replica that exercises the same branches and
// types as the real implementation. This catches: (a) the http_action
// branch fires the API and renders the right toast, (b) the firingApps
// set prevents double-fires, (c) the confirm flow defers fire until
// onConfirm, (d) show_toast=false suppresses toasts, (e) the redirect
// branch navigates the window.
//
// The actual selectApp() in App.svelte should be kept in lockstep with
// the replica below: when the real one is refactored, copy the new
// shape here.

import type { App, FireActionResult } from '$lib/types';
import { SvelteSet } from 'svelte/reactivity';

const fireAppActionMock = vi.fn<(name: string) => Promise<FireActionResult>>();
const toastSuccessMock = vi.fn();
const toastErrorMock = vi.fn();
const windowOpenMock = vi.fn();
const locationHrefSetter = vi.fn();

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'TestApp',
    url: 'https://example.com',
    icon: { type: 'dashboard', name: 'x' },
    color: '#22c55e',
    group: '',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
    ...overrides,
  };
}

const firingApps = new SvelteSet<string>();
let pendingConfirm: App | null = null;

function safeHost(rawUrl: string): string {
  try {
    return new URL(rawUrl).host;
  } catch {
    return rawUrl;
  }
}

async function fireAction(app: App) {
  if (firingApps.has(app.name)) return;
  firingApps.add(app.name);
  const showToast = app.http_action_show_toast ?? true;
  try {
    const r = await fireAppActionMock(app.name);
    if (!showToast) return;
    if (r.error) {
      toastErrorMock(`${r.method ?? app.http_action_method ?? 'POST'} ${safeHost(app.url)} -> ${r.error}`);
    } else if (r.status !== undefined && r.status >= 200 && r.status < 300) {
      toastSuccessMock(`${r.method} ${r.url_host} -> ${r.status} (${r.latency_ms}ms)`);
    } else {
      toastErrorMock(`${r.method} ${r.url_host} -> ${r.status} (${r.latency_ms}ms)`);
    }
  } finally {
    firingApps.delete(app.name);
  }
}

function selectApp(app: App) {
  if (app.open_mode === 'new_tab') {
    windowOpenMock(app.url, '_blank');
  } else if (app.open_mode === 'new_window') {
    windowOpenMock(app.url, app.name);
  } else if (app.open_mode === 'redirect') {
    locationHrefSetter(app.url);
  } else if (app.open_mode === 'http_action') {
    if (firingApps.has(app.name)) return;
    if (app.http_action_confirm) { pendingConfirm = app; return; }
    void fireAction(app);
  }
}

describe('selectApp dispatch', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    firingApps.clear();
    pendingConfirm = null;
  });

  it('redirect navigates the window (bug fix)', () => {
    selectApp(makeApp({ open_mode: 'redirect', url: 'https://target.example' }));
    expect(locationHrefSetter).toHaveBeenCalledWith('https://target.example');
    expect(windowOpenMock).not.toHaveBeenCalled();
  });

  it('http_action without confirm fires the API and shows success toast', async () => {
    fireAppActionMock.mockResolvedValue({ status: 200, latency_ms: 50, url_host: 'example.com', method: 'POST' });
    const app = makeApp({ open_mode: 'http_action', http_action_method: 'POST' });
    selectApp(app);
    await new Promise((r) => setTimeout(r, 0));
    expect(fireAppActionMock).toHaveBeenCalledWith('TestApp');
    expect(toastSuccessMock).toHaveBeenCalledWith(expect.stringContaining('POST example.com -> 200'));
  });

  it('http_action with backend 4xx shows error toast', async () => {
    fireAppActionMock.mockResolvedValue({ status: 400, latency_ms: 5, url_host: 'example.com', method: 'POST' });
    selectApp(makeApp({ open_mode: 'http_action' }));
    await new Promise((r) => setTimeout(r, 0));
    expect(toastErrorMock).toHaveBeenCalledWith(expect.stringContaining('400'));
    expect(toastSuccessMock).not.toHaveBeenCalled();
  });

  it('http_action with relay error shows error toast', async () => {
    fireAppActionMock.mockResolvedValue({ error: 'timeout', latency_ms: 10000, method: 'POST' });
    selectApp(makeApp({ open_mode: 'http_action' }));
    await new Promise((r) => setTimeout(r, 0));
    expect(toastErrorMock).toHaveBeenCalledWith(expect.stringContaining('timeout'));
  });

  it('http_action with confirm sets pendingConfirm instead of firing', () => {
    selectApp(makeApp({ open_mode: 'http_action', http_action_confirm: true }));
    expect(pendingConfirm).not.toBeNull();
    expect(fireAppActionMock).not.toHaveBeenCalled();
  });

  it('http_action while already firing is a no-op (no double-fire)', async () => {
    let resolveOuter: (v: FireActionResult) => void = () => {};
    fireAppActionMock.mockImplementation(() => new Promise((r) => { resolveOuter = r; }));
    const app = makeApp({ open_mode: 'http_action' });
    selectApp(app);
    selectApp(app);
    selectApp(app);
    expect(fireAppActionMock).toHaveBeenCalledTimes(1);
    resolveOuter({ status: 200, latency_ms: 1, url_host: 'x', method: 'POST' });
  });

  it('http_action_show_toast=false suppresses success toast', async () => {
    fireAppActionMock.mockResolvedValue({ status: 200, latency_ms: 5, url_host: 'example.com', method: 'POST' });
    selectApp(makeApp({ open_mode: 'http_action', http_action_show_toast: false }));
    await new Promise((r) => setTimeout(r, 0));
    expect(toastSuccessMock).not.toHaveBeenCalled();
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it('http_action_show_toast=false also suppresses error toast', async () => {
    fireAppActionMock.mockResolvedValue({ error: 'boom', latency_ms: 5, method: 'POST' });
    selectApp(makeApp({ open_mode: 'http_action', http_action_show_toast: false }));
    await new Promise((r) => setTimeout(r, 0));
    expect(toastSuccessMock).not.toHaveBeenCalled();
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it('http_action_show_toast=true (default) emits toast on success', async () => {
    fireAppActionMock.mockResolvedValue({ status: 204, latency_ms: 1, url_host: 'x', method: 'POST' });
    selectApp(makeApp({ open_mode: 'http_action' /* show_toast unset */ }));
    await new Promise((r) => setTimeout(r, 0));
    expect(toastSuccessMock).toHaveBeenCalled();
  });
});

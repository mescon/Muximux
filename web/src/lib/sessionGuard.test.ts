import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { authState } from './authStore';
import {
  clearResumeHash,
  ensureSession,
  installSessionGuard,
  markSessionOk,
  onSessionEnded,
  resetSessionGuardForTest,
  restoreResumeHash,
  runWithSession,
  saveResumeHash,
  sessionFresh,
  sessionSuspect,
  startProviderRecheck,
  FRESH_MS,
  RESUME_AFTER_MS,
} from './sessionGuard';

function setAuthenticated(authenticated: boolean) {
  authState.set({ authenticated, user: authenticated ? { username: 'u', role: 'user' } : null, loading: false, error: null, setupRequired: false, logoutUrl: null });
}

function mockStatus(body: unknown, ok = true) {
  const fetchMock = vi.fn().mockResolvedValue({ ok, json: () => Promise.resolve(body) });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('sessionGuard', () => {
  const realLocation = globalThis.location;
  let hrefSet: string | null;

  beforeEach(() => {
    resetSessionGuardForTest();
    sessionStorage.clear();
    history.replaceState(null, '', '/');
    setAuthenticated(true);
    hrefSet = null;
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-11-10T10:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    Object.defineProperty(globalThis, 'location', { value: realLocation, configurable: true, writable: true });
  });

  function trapNavigation() {
    const loc = { ...realLocation, pathname: '/', search: '', hash: realLocation.hash, origin: realLocation.origin } as Location;
    Object.defineProperty(loc, 'href', { get: () => hrefSet ?? '', set: (v: string) => { hrefSet = v; }, configurable: true });
    Object.defineProperty(globalThis, 'location', { value: loc, configurable: true, writable: true });
  }

  describe('freshness', () => {
    it('is fresh right after a successful check and stale after the window', () => {
      expect(sessionFresh()).toBe(false);
      markSessionOk();
      expect(sessionFresh()).toBe(true);
      vi.setSystemTime(Date.now() + FRESH_MS + 1);
      expect(sessionFresh()).toBe(false);
    });

    it('does not call the server while fresh', async () => {
      const fetchMock = mockStatus({ authenticated: true });
      markSessionOk();
      expect(await ensureSession()).toBe('ok');
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe('ensureSession', () => {
    it('reports ok and marks the session fresh', async () => {
      mockStatus({ authenticated: true });
      expect(await ensureSession()).toBe('ok');
      expect(sessionFresh()).toBe(true);
    });

    it('reports ended once and calls the handler once', async () => {
      mockStatus({ authenticated: false });
      const handler = vi.fn();
      onSessionEnded(handler);
      expect(await ensureSession({ force: true })).toBe('ended');
      expect(await ensureSession({ force: true })).toBe('ended');
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('does not treat an anonymous visitor as an ended session', async () => {
      setAuthenticated(false);
      mockStatus({ authenticated: false });
      const handler = vi.fn();
      onSessionEnded(handler);
      expect(await ensureSession()).toBe('unknown');
      expect(handler).not.toHaveBeenCalled();
    });

    it.each([
      ['a network error', () => vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')))],
      ['a 502', () => mockStatus(null, false)],
      ['a body without authenticated', () => mockStatus({})],
    ])('treats %s as unknown, never as a logout', async (_name, setup) => {
      setup();
      const handler = vi.fn();
      onSessionEnded(handler);
      expect(await ensureSession({ force: true })).toBe('unknown');
      expect(handler).not.toHaveBeenCalled();
    });

    it('shares one request between concurrent callers', async () => {
      const fetchMock = mockStatus({ authenticated: true });
      const [a, b] = await Promise.all([ensureSession(), ensureSession()]);
      expect(a).toBe('ok');
      expect(b).toBe('ok');
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('starts the provider recheck only when allowed', async () => {
      trapNavigation();
      mockStatus({ authenticated: true, revalidate: true });
      expect(await ensureSession({ force: true })).toBe('ok');
      expect(hrefSet).toBeNull();
      expect(await ensureSession({ force: true, allowRecheck: true })).toBe('rechecking');
      expect(hrefSet).toBe('/api/auth/oidc/login?prompt=none&redirect=%2F');
    });
  });

  describe('runWithSession', () => {
    it('runs at once while fresh', () => {
      markSessionOk();
      const fn = vi.fn();
      runWithSession(fn);
      expect(fn).toHaveBeenCalledTimes(1);
    });

    it('runs after a passing check, and on an unreachable server', async () => {
      mockStatus({ authenticated: true });
      const fn = vi.fn();
      runWithSession(fn);
      expect(fn).not.toHaveBeenCalled();
      await vi.waitFor(() => expect(fn).toHaveBeenCalledTimes(1));

      resetSessionGuardForTest();
      vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
      const fn2 = vi.fn();
      runWithSession(fn2);
      await vi.waitFor(() => expect(fn2).toHaveBeenCalledTimes(1));
    });

    it('does not run when the session has ended', async () => {
      mockStatus({ authenticated: false });
      onSessionEnded(() => {});
      const fn = vi.fn();
      runWithSession(fn);
      await ensureSession(); // let the shared check settle
      await Promise.resolve();
      expect(fn).not.toHaveBeenCalled();
    });
  });

  describe('sessionSuspect', () => {
    it('forces a check while signed in', async () => {
      markSessionOk();
      const fetchMock = mockStatus({ authenticated: true });
      sessionSuspect();
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    });

    it('does nothing when not signed in', () => {
      setAuthenticated(false);
      const fetchMock = mockStatus({ authenticated: false });
      sessionSuspect();
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe('resume hash', () => {
    it('round-trips the app route once', () => {
      history.replaceState(null, '', '/#emby');
      saveResumeHash();
      history.replaceState(null, '', '/');
      expect(restoreResumeHash()).toBe(true);
      expect(location.hash).toBe('#emby');
      history.replaceState(null, '', '/');
      expect(restoreResumeHash()).toBe(false);
    });

    it('does not overwrite a hash that is already there', () => {
      history.replaceState(null, '', '/#emby');
      saveResumeHash();
      history.replaceState(null, '', '/#radarr');
      expect(restoreResumeHash()).toBe(false);
      expect(location.hash).toBe('#radarr');
    });

    it('ignores a stale or malformed entry', () => {
      sessionStorage.setItem('muximux_resume_hash', JSON.stringify({ hash: '#emby', at: Date.now() - 11 * 60_000 }));
      expect(restoreResumeHash()).toBe(false);
      sessionStorage.setItem('muximux_resume_hash', JSON.stringify({ hash: 'javascript:x', at: Date.now() }));
      expect(restoreResumeHash()).toBe(false);
      sessionStorage.setItem('muximux_resume_hash', '{not json');
      expect(restoreResumeHash()).toBe(false);
      expect(location.hash).toBe('');
    });

    it('saves nothing without a route', () => {
      saveResumeHash();
      expect(sessionStorage.getItem('muximux_resume_hash')).toBeNull();
    });
  });

  describe('clearResumeHash', () => {
    it('forgets a saved route', () => {
      history.replaceState(null, '', '/#emby');
      saveResumeHash();
      clearResumeHash();
      history.replaceState(null, '', '/');
      expect(restoreResumeHash()).toBe(false);
    });
  });

  describe('startProviderRecheck', () => {
    it('does not recheck without sessionStorage (no loop guard)', () => {
      trapNavigation();
      const desc = Object.getOwnPropertyDescriptor(globalThis, 'sessionStorage')!;
      Object.defineProperty(globalThis, 'sessionStorage', { get() { throw new Error('blocked'); }, configurable: true });
      try {
        expect(startProviderRecheck()).toBe(false);
        expect(hrefSet).toBeNull();
      } finally {
        Object.defineProperty(globalThis, 'sessionStorage', desc);
      }
    });

    it('refuses a second recheck within a minute', () => {
      trapNavigation();
      expect(startProviderRecheck()).toBe(true);
      hrefSet = null;
      expect(startProviderRecheck()).toBe(false);
      expect(hrefSet).toBeNull();
      vi.setSystemTime(Date.now() + FRESH_MS + 1);
      expect(startProviderRecheck()).toBe(true);
    });
  });

  describe('installSessionGuard', () => {
    function setVisibility(state: 'visible' | 'hidden') {
      Object.defineProperty(document, 'visibilityState', { value: state, configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    }

    it('rechecks after a long hidden spell, not a short one', async () => {
      const fetchMock = mockStatus({ authenticated: true });
      const cleanup = installSessionGuard();
      setVisibility('hidden');
      vi.setSystemTime(Date.now() + 5_000);
      setVisibility('visible');
      expect(fetchMock).not.toHaveBeenCalled();

      setVisibility('hidden');
      vi.setSystemTime(Date.now() + RESUME_AFTER_MS);
      setVisibility('visible');
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
      cleanup();
    });

    it('rechecks on a restore from the back/forward cache', async () => {
      const fetchMock = mockStatus({ authenticated: true });
      const cleanup = installSessionGuard();
      const ev = new Event('pageshow') as PageTransitionEvent;
      Object.defineProperty(ev, 'persisted', { value: true });
      window.dispatchEvent(ev);
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
      cleanup();
    });

    it('rechecks when the network comes back', async () => {
      const fetchMock = mockStatus({ authenticated: true });
      const cleanup = installSessionGuard();
      window.dispatchEvent(new Event('offline'));
      vi.setSystemTime(Date.now() + RESUME_AFTER_MS);
      window.dispatchEvent(new Event('online'));
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
      await new Promise((r) => setTimeout(r, 0)); // let that check settle
      window.dispatchEvent(new Event('online')); // short blip: still verified
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
      cleanup();
    });

    it('verifies an app frame message from this origin only', async () => {
      const fetchMock = mockStatus({ authenticated: true });
      const cleanup = installSessionGuard();
      window.dispatchEvent(new MessageEvent('message', { origin: 'https://evil.example.net', data: { type: 'muximux:auth-required' } }));
      window.dispatchEvent(new MessageEvent('message', { origin: location.origin, data: { type: 'something-else' } }));
      window.dispatchEvent(new MessageEvent('message', { origin: location.origin, data: 'muximux:auth-required' }));
      expect(fetchMock).not.toHaveBeenCalled();
      window.dispatchEvent(new MessageEvent('message', { origin: location.origin, data: { type: 'muximux:auth-required' } }));
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
      cleanup();
      window.dispatchEvent(new MessageEvent('message', { origin: location.origin, data: { type: 'muximux:auth-required' } }));
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
  });
});

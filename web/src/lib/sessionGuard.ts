/**
 * Session guard (#554).
 *
 * The dashboard checks its session with the server before it opens an
 * app and when it comes back from sleep or a hidden tab. When the session
 * is gone it tears the app frames down and signs in again in the
 * top-level window, never inside an app's iframe. With
 * auth.oidc.revalidate_after set, it also sends the browser through a
 * silent (prompt=none) round trip to the identity provider when the
 * server says the provider has not vouched for the session in a while.
 *
 * Signals from elsewhere (an API 401, a WebSocket closed with 4401, an
 * app frame's "muximux:auth-required" message) never end the session on
 * their own: they only trigger a fresh status check. A proxied app is
 * same-origin with the dashboard and could post that message itself.
 */
import { get } from 'svelte/store';
import { isAuthenticated } from './authStore';
import { debug } from './debug';

/** A successful check is trusted for this long; app switching within it
 * costs no request. */
export const FRESH_MS = 60_000;
/** A hidden tab or an offline spell at least this long is re-checked on
 * return. */
export const RESUME_AFTER_MS = 60_000;

const RESUME_HASH_KEY = 'muximux_resume_hash';
const RECHECK_AT_KEY = 'muximux_recheck_at';
/** A saved hash older than this is not restored (it is from an abandoned
 * sign-in). Matches the server's pending sign-in lifetime. */
const RESUME_TTL_MS = 10 * 60_000;

export type SessionResult = 'ok' | 'ended' | 'rechecking' | 'unknown';

let lastOk = 0;
let inFlight: Promise<SessionResult> | null = null;
let endedHandler: (() => void) | null = null;
let ended = false;

function base(): string {
  return ((globalThis as unknown as Record<string, string>).__MUXIMUX_BASE__) || '';
}

function storage(): Storage | null {
  try {
    return globalThis.sessionStorage ?? null;
  } catch {
    return null;
  }
}

/** Register what the dashboard does when its session has ended. */
export function onSessionEnded(handler: () => void): void {
  endedHandler = handler;
  ended = false;
}

/** Record a successful sign-in or status check. */
export function markSessionOk(): void {
  lastOk = Date.now();
  ended = false;
}

/** Forget a saved app route (after a sign-in that did not need it, such
 * as a password sign-in that keeps the hash in the address). */
export function clearResumeHash(): void {
  try {
    storage()?.removeItem(RESUME_HASH_KEY);
  } catch {
    // nothing to clear
  }
}

export function sessionFresh(): boolean {
  return Date.now() - lastOk < FRESH_MS;
}

/** Save the current app route so it survives a sign-in round trip (the
 * provider redirects back to a path; the hash never reaches it). */
export function saveResumeHash(): void {
  const s = storage();
  if (!s || !location.hash || location.hash === '#') return;
  try {
    s.setItem(RESUME_HASH_KEY, JSON.stringify({ hash: location.hash, at: Date.now() }));
  } catch {
    // storage full or blocked: the user lands on the default app instead
  }
}

/** Put a route saved by saveResumeHash back into the address, once.
 * Returns true when it did. Call before the dashboard picks its app. */
export function restoreResumeHash(): boolean {
  const s = storage();
  if (!s) return false;
  let raw: string | null;
  try {
    raw = s.getItem(RESUME_HASH_KEY);
    s.removeItem(RESUME_HASH_KEY);
  } catch {
    return false;
  }
  if (!raw || location.hash) return false;
  try {
    const { hash, at } = JSON.parse(raw) as { hash?: unknown; at?: unknown };
    if (typeof hash !== 'string' || !hash.startsWith('#') || typeof at !== 'number') return false;
    if (Date.now() - at > RESUME_TTL_MS) return false;
    history.replaceState(history.state, '', location.pathname + location.search + hash);
    return true;
  } catch {
    return false;
  }
}

/** Start the silent provider recheck (auth.oidc.revalidate_after): a
 * top-level navigation, so the dashboard reloads afterwards. Refuses to
 * start twice within a minute, so a misbehaving server cannot loop. */
export function startProviderRecheck(): boolean {
  const s = storage();
  // Without storage there is no loop guard: do not recheck.
  if (!s) return false;
  try {
    const last = Number(s.getItem(RECHECK_AT_KEY) || 0);
    if (Date.now() - last < FRESH_MS) return false;
    s.setItem(RECHECK_AT_KEY, String(Date.now()));
  } catch {
    // without storage there is no loop guard: do not recheck
    return false;
  }
  saveResumeHash();
  debug('auth', 'provider recheck');
  const redirect = encodeURIComponent(location.pathname);
  globalThis.location.href = `${base()}/api/auth/oidc/login?prompt=none&redirect=${redirect}`;
  return true;
}

function sessionEnded(): void {
  if (ended) return;
  ended = true;
  lastOk = 0;
  debug('auth', 'session ended');
  saveResumeHash();
  endedHandler?.();
}

interface CheckOptions {
  /** Ignore the freshness window. */
  force?: boolean;
  /** Allow the provider recheck (page load and resume only, never an app
   * switch in active use). */
  allowRecheck?: boolean;
}

async function check(allowRecheck: boolean): Promise<SessionResult> {
  let data: { authenticated?: boolean; revalidate?: boolean } | null;
  try {
    const res = await fetch(`${base()}/api/auth/status`, { cache: 'no-store' });
    if (!res.ok) return 'unknown';
    data = await res.json();
  } catch {
    // Offline or the server is down: that is not a logout.
    return 'unknown';
  }
  if (!data || typeof data.authenticated !== 'boolean') return 'unknown';
  if (!data.authenticated) {
    if (get(isAuthenticated)) {
      sessionEnded();
      return 'ended';
    }
    return 'unknown';
  }
  if (data.revalidate && allowRecheck && startProviderRecheck()) return 'rechecking';
  markSessionOk();
  return 'ok';
}

/** Check the session with the server unless a recent check passed.
 * Concurrent callers share one request. */
export function ensureSession(opts: CheckOptions = {}): Promise<SessionResult> {
  if (!opts.force && sessionFresh()) return Promise.resolve('ok');
  if (!inFlight) {
    inFlight = check(opts.allowRecheck ?? false).finally(() => { inFlight = null; });
  }
  return inFlight;
}

/** Run fn now when the session is known to be good, otherwise after a
 * check. A check that cannot reach the server lets fn run: an outage is
 * not a logout. */
export function runWithSession(fn: () => void): void {
  if (sessionFresh()) {
    fn();
    return;
  }
  void ensureSession().then((r) => {
    if (r === 'ok' || r === 'unknown') fn();
  });
}

/** Something hinted that the session may be gone (API 401, WebSocket
 * close, an app frame's message). Verify with the server. */
export function sessionSuspect(): void {
  if (!get(isAuthenticated)) return;
  void ensureSession({ force: true });
}

/** Install the resume triggers and the app-frame listener. Returns a
 * cleanup function. */
export function installSessionGuard(win: Window = window): () => void {
  let hiddenAt = 0;
  let offlineAt = 0;

  const onVisibility = () => {
    if (win.document.visibilityState === 'hidden') {
      hiddenAt = Date.now();
      return;
    }
    if (hiddenAt && Date.now() - hiddenAt >= RESUME_AFTER_MS) {
      void ensureSession({ force: true, allowRecheck: true });
    }
    hiddenAt = 0;
  };
  const onPageShow = (e: PageTransitionEvent) => {
    if (e.persisted) void ensureSession({ force: true, allowRecheck: true });
  };
  const onOffline = () => { offlineAt = Date.now(); };
  const onOnline = () => {
    if (offlineAt && Date.now() - offlineAt >= RESUME_AFTER_MS) {
      void ensureSession({ force: true, allowRecheck: true });
    } else {
      sessionSuspect();
    }
    offlineAt = 0;
  };
  const onMessage = (e: MessageEvent) => {
    if (e.origin !== win.location.origin) return;
    const data = e.data as { type?: unknown } | null;
    if (data && typeof data === 'object' && data.type === 'muximux:auth-required') {
      sessionSuspect();
    }
  };

  win.document.addEventListener('visibilitychange', onVisibility);
  win.addEventListener('pageshow', onPageShow as EventListener);
  win.addEventListener('offline', onOffline);
  win.addEventListener('online', onOnline);
  win.addEventListener('message', onMessage);
  return () => {
    win.document.removeEventListener('visibilitychange', onVisibility);
    win.removeEventListener('pageshow', onPageShow as EventListener);
    win.removeEventListener('offline', onOffline);
    win.removeEventListener('online', onOnline);
    win.removeEventListener('message', onMessage);
  };
}

/** Test hook. */
export function resetSessionGuardForTest(): void {
  lastOk = 0;
  inFlight = null;
  endedHandler = null;
  ended = false;
}

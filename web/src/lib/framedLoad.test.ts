import { describe, it, expect, vi, afterEach } from 'vitest';
import { applyFrameLink, FRAME_RESTORE_TIMEOUT_MS, reportFramedLoad, restoreFrame } from './framedLoad';
import { makeApp } from './types';

describe('reportFramedLoad', () => {
  // A window as seen from the copy: its parent, top, navigation type and hash.
  function copyWindow(opts: {
    nested?: boolean; type?: string; hash?: string; selfParent?: boolean; activation?: string | null;
  } = {}) {
    const post = vi.fn();
    const shell = { postMessage: post } as unknown as Window;
    const win = {
      performance: {
        getEntriesByType: (t: string) => (t === 'navigation' && opts.type !== undefined ? [{ type: opts.type }] : []),
      },
      location: { hash: opts.hash ?? '#emby', origin: 'https://mux.example' },
      document: { referrer: 'http://homarr/board' },
      navigation: opts.activation === null ? undefined : { activation: { navigationType: opts.activation ?? 'push' } },
    } as unknown as Window & { parent: Window; top: Window };
    win.parent = opts.selfParent ? win : shell;
    win.top = opts.nested ? ({} as Window) : opts.selfParent ? win : shell;
    return { win, post };
  }

  it('sends the hash, referrer and pushed flag after a fresh navigation', () => {
    const { win, post } = copyWindow({ type: 'navigate' });
    reportFramedLoad(win);
    expect(post).toHaveBeenCalledWith(
      { type: 'muximux:frame-link', hash: '#emby', referrer: 'http://homarr/board', pushed: true },
      'https://mux.example',
    );
  });

  it.each([['replace', 'replace'], ['no Navigation API', null]])('reports pushed false for %s', (_l, activation) => {
    const { win, post } = copyWindow({ type: 'navigate', activation });
    reportFramedLoad(win);
    expect(post).toHaveBeenCalledWith(expect.objectContaining({ pushed: false }), 'https://mux.example');
  });

  it('sends an empty hash too', () => {
    const { win, post } = copyWindow({ type: 'navigate', hash: '' });
    reportFramedLoad(win);
    expect(post).toHaveBeenCalledWith(expect.objectContaining({ hash: '' }), 'https://mux.example');
  });

  it.each(['back_forward', 'reload', undefined])('stays quiet for navigation type %s', (type) => {
    const { win, post } = copyWindow({ type });
    reportFramedLoad(win);
    expect(post).not.toHaveBeenCalled();
  });

  it('stays quiet when not directly inside the shell', () => {
    const { win, post } = copyWindow({ type: 'navigate', nested: true });
    reportFramedLoad(win);
    expect(post).not.toHaveBeenCalled();
  });

  it('stays quiet in a top-level window', () => {
    const { win, post } = copyWindow({ type: 'navigate', selfParent: true });
    reportFramedLoad(win);
    expect(post).not.toHaveBeenCalled();
  });

  it('defaults to the real window, which is top-level in tests', () => {
    expect(() => reportFramedLoad()).not.toThrow();
  });
});

describe('restoreFrame', () => {
  const app = makeApp({ name: 'Homarr', url: 'http://homarr/' });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  // A frame whose window reports `pathname` (the copy shows the shell's own
  // path) or throws like a cross-origin page when pathname is null.
  function stubFrame(initial: string | null = window.location.pathname) {
    const frame = document.createElement('iframe');
    let pathname = initial;
    const replace = vi.fn();
    const win = {
      location: {
        get pathname() { if (pathname === null) throw new Error('cross-origin'); return pathname; },
        replace,
      },
    };
    Object.defineProperty(frame, 'contentWindow', { value: win });
    return { frame, replace, show: (p: string | null) => { pathname = p; } };
  }

  function track(p: Promise<void>) {
    const state = { resolved: false };
    void p.then(() => { state.resolved = true; });
    return state;
  }

  it('goes back when the link pushed an entry, ignoring the copy\'s own load', async () => {
    const back = vi.spyOn(history, 'back').mockImplementation(() => {});
    const { frame, replace, show } = stubFrame();
    const state = track(restoreFrame(frame, app, true));
    expect(back).toHaveBeenCalledTimes(1);
    expect(replace).not.toHaveBeenCalled();
    frame.dispatchEvent(new Event('load'));
    await Promise.resolve();
    expect(state.resolved).toBe(false);
    show(null);
    frame.dispatchEvent(new Event('load'));
    await Promise.resolve();
    expect(state.resolved).toBe(true);
  });

  it('replaces the copy with the app URL when nothing was pushed', async () => {
    const back = vi.spyOn(history, 'back').mockImplementation(() => {});
    const { frame, replace, show } = stubFrame();
    const state = track(restoreFrame(frame, app, false));
    expect(back).not.toHaveBeenCalled();
    expect(replace).toHaveBeenCalledWith('http://homarr/');
    show('/somewhere');
    frame.dispatchEvent(new Event('load'));
    await Promise.resolve();
    expect(state.resolved).toBe(true);
  });

  it('replaces with a proxied app\'s proxy path and blanks an unsafe URL', () => {
    const a = stubFrame();
    void restoreFrame(a.frame, makeApp({ name: 'P', url: 'http://p/', proxyUrl: '/proxy/p/' }), false);
    expect(a.replace).toHaveBeenCalledWith('/proxy/p/');
    const b = stubFrame();
    void restoreFrame(b.frame, makeApp({ name: 'X', url: 'javascript:alert(1)' }), false);
    expect(b.replace).toHaveBeenCalledWith('about:blank');
  });

  it('falls back to src when replace throws', () => {
    const { frame, replace } = stubFrame();
    replace.mockImplementation(() => { throw new Error('gone'); });
    void restoreFrame(frame, app, false);
    expect(frame.getAttribute('src')).toBe('http://homarr/');
  });

  it('replaces the copy if it is still there at the timeout', async () => {
    vi.useFakeTimers();
    vi.spyOn(history, 'back').mockImplementation(() => {});
    const { frame, replace } = stubFrame();
    const state = track(restoreFrame(frame, app, true));
    await vi.advanceTimersByTimeAsync(FRAME_RESTORE_TIMEOUT_MS - 1);
    expect(state.resolved).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(replace).toHaveBeenCalledWith('http://homarr/');
    expect(state.resolved).toBe(true);
  });

  it('only resolves at the timeout once the app is back', async () => {
    vi.useFakeTimers();
    vi.spyOn(history, 'back').mockImplementation(() => {});
    const { frame, replace, show } = stubFrame();
    const state = track(restoreFrame(frame, app, true));
    show(null);
    await vi.advanceTimersByTimeAsync(FRAME_RESTORE_TIMEOUT_MS);
    expect(replace).not.toHaveBeenCalled();
    expect(state.resolved).toBe(true);
  });
});

describe('applyFrameLink', () => {
  const sender = makeApp({ name: 'Homarr', url: 'http://homarr/', allow_navigation: true });
  const frame = document.createElement('iframe');
  const req = (hash: string) => ({ sender, senderFrame: frame, hash, pushed: true });

  function run(hash: string, apps = [sender, makeApp({ name: 'Emby', url: 'http://emby/' })]) {
    const navigate = vi.fn();
    const setHash = vi.fn();
    applyFrameLink(req(hash), apps, { navigate, setHash });
    return { navigate, setHash };
  }

  it('navigates to a single iframe app by slug', () => {
    const { navigate, setHash } = run('#emby');
    expect(navigate).toHaveBeenCalledWith({ senderFrame: frame, target: expect.objectContaining({ name: 'Emby' }) });
    expect(setHash).not.toHaveBeenCalled();
  });

  it('sets other hashes on the shell', () => {
    for (const hash of ['#homarr+emby', '#settings', '#nope']) {
      const { navigate, setHash } = run(hash);
      expect(navigate).not.toHaveBeenCalled();
      expect(setHash).toHaveBeenCalledWith(hash);
    }
  });

  it('leaves non-iframe apps to the shell hash', () => {
    const apps = [makeApp({ name: 'Tab', url: 'http://tab/', open_mode: 'new_tab' })];
    expect(run('#tab', apps).setHash).toHaveBeenCalledWith('#tab');
  });

  it('does nothing for a disabled app', () => {
    const { navigate, setHash } = run('#off', [makeApp({ name: 'Off', url: 'http://off/', enabled: false })]);
    expect(navigate).not.toHaveBeenCalled();
    expect(setHash).not.toHaveBeenCalled();
  });

  it('sets reserved hashes on the shell even when an app has that slug', () => {
    const { navigate, setHash } = run('#settings', [makeApp({ name: 'Settings', url: 'http://s/' })]);
    expect(navigate).not.toHaveBeenCalled();
    expect(setHash).toHaveBeenCalledWith('#settings');
  });

  it('does nothing for an empty hash', () => {
    const { navigate, setHash } = run('');
    expect(navigate).not.toHaveBeenCalled();
    expect(setHash).not.toHaveBeenCalled();
  });
});

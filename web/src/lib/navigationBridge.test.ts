import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { installNavigationBridge, MAX_FRAME_LINK_HASH, MAX_REQUEST_ID, NAV_RATE_LIMIT_MS } from './navigationBridge';
import { makeApp, type App } from './types';

describe('navigationBridge', () => {
  let cleanup: (() => void) | null = null;
  let apps: App[];
  let visible: boolean;
  let clock: number;
  let onNavigate: ReturnType<typeof vi.fn>;
  let onFrameLink: ReturnType<typeof vi.fn>;
  let warn: ReturnType<typeof vi.spyOn>;
  const frames: HTMLIFrameElement[] = [];
  const posts = new Map<HTMLIFrameElement, ReturnType<typeof vi.fn>>();

  const sender = () => makeApp({ name: 'Sender', url: 'https://s.local', allow_navigation: true });
  const target = () => makeApp({ name: 'My Target', url: 'https://t.local/app/' });

  beforeEach(() => {
    apps = [];
    visible = true;
    clock = 10_000;
    onNavigate = vi.fn();
    onFrameLink = vi.fn();
    warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
  });

  afterEach(() => {
    cleanup?.();
    cleanup = null;
    for (const f of frames) f.remove();
    frames.length = 0;
    posts.clear();
    warn.mockRestore();
  });

  function addFrame(app: App): HTMLIFrameElement {
    const iframe = document.createElement('iframe');
    iframe.dataset.app = app.name;
    document.body.appendChild(iframe);
    frames.push(iframe);
    const post = vi.fn();
    iframe.contentWindow!.postMessage = post as unknown as Window['postMessage'];
    posts.set(iframe, post);
    return iframe;
  }

  function install() {
    cleanup = installNavigationBridge({
      getApps: () => apps,
      isFrameVisible: () => visible,
      onNavigate,
      onFrameLink,
      now: () => clock,
    });
  }

  function send(frame: HTMLIFrameElement, data: unknown, origin = 'https://s.local') {
    window.dispatchEvent(new MessageEvent('message', { data, origin, source: frame.contentWindow! }));
  }

  function expectReply(frame: HTMLIFrameElement, error: string, origin = 'https://s.local') {
    expect(posts.get(frame)).toHaveBeenCalledWith(
      { type: 'muximux:navigate-result', ok: false, error },
      origin,
    );
  }

  function setup(s = sender(), t = target()) {
    apps = [s, t];
    const frame = addFrame(s);
    install();
    return { s, t, frame };
  }

  it('ignores other message types silently', () => {
    const { frame } = setup();
    send(frame, { type: 'other' });
    send(frame, null);
    send(frame, 'muximux:navigate');
    expect(onNavigate).not.toHaveBeenCalled();
    expect(warn).not.toHaveBeenCalled();
    expect(posts.get(frame)).not.toHaveBeenCalled();
  });

  it('1: warns and does not reply when no sender frame matches', () => {
    setup();
    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'muximux:navigate', app: 'my-target' },
      origin: 'https://evil.example',
      source: window,
    }));
    expect(warn).toHaveBeenCalledWith('[muximux:navigate] no matching app for message', expect.anything());
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('1: sender removed from getApps while its frame stays gets a warning only', () => {
    const { frame } = setup();
    apps = [];
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(warn).toHaveBeenCalledTimes(1);
    expect(onNavigate).not.toHaveBeenCalled();
    expect(posts.get(frame)).not.toHaveBeenCalled();
  });

  it('2: not_allowed when sender lacks allow_navigation', () => {
    const { frame } = setup(makeApp({ name: 'Sender', url: 'https://s.local' }));
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expectReply(frame, 'not_allowed');
    expect(warn).toHaveBeenCalledWith('[muximux:navigate] navigation not enabled for app',
      expect.objectContaining({ app: 'Sender', hint: expect.stringContaining('Allow navigating Muximux') }));
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('3: not_visible when the frame is not visible', () => {
    const { frame } = setup();
    visible = false;
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expectReply(frame, 'not_visible');
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it.each([
    ['missing app', { type: 'muximux:navigate' }],
    ['non-string app', { type: 'muximux:navigate', app: 5 }],
    ['empty app', { type: 'muximux:navigate', app: '' }],
    ['oversized app', { type: 'muximux:navigate', app: 'x'.repeat(201) }],
    ['non-string path', { type: 'muximux:navigate', app: 'my-target', path: 5 }],
  ])('4: bad_request for %s', (_label, msg) => {
    const { frame } = setup();
    send(frame, msg);
    expectReply(frame, 'bad_request');
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('5: unknown_app when nothing matches', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'nope' });
    expectReply(frame, 'unknown_app');
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('5: unknown_app for a disabled target, by slug or name', () => {
    const { frame } = setup(sender(), makeApp({ name: 'My Target', url: 'https://t.local', enabled: false }));
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    clock += NAV_RATE_LIMIT_MS;
    send(frame, { type: 'muximux:navigate', app: 'My Target' });
    expect(posts.get(frame)).toHaveBeenCalledTimes(2);
    expectReply(frame, 'unknown_app');
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('5: matches the target by slug', () => {
    const { s, t, frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(onNavigate).toHaveBeenCalledWith({ sender: s, senderFrame: frame, target: t, url: undefined });
  });

  it('5: matches the target by display name', () => {
    const { t, frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'My Target' });
    expect(onNavigate).toHaveBeenCalledWith(expect.objectContaining({ target: t }));
  });

  it('6: not_embeddable for non-iframe targets', () => {
    const { frame } = setup(sender(), makeApp({ name: 'My Target', url: 'https://t.local', open_mode: 'new_tab' }));
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expectReply(frame, 'not_embeddable');
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('6: a target with undefined open_mode is embeddable', () => {
    const t = makeApp({ name: 'My Target', url: 'https://t.local' });
    (t as { open_mode?: App['open_mode'] }).open_mode = undefined;
    const { frame } = setup(sender(), t);
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(onNavigate).toHaveBeenCalledTimes(1);
  });

  it('6: a target with an empty open_mode (unset in YAML) is embeddable', () => {
    const t = makeApp({ name: 'My Target', url: 'https://t.local' });
    (t as unknown as { open_mode: string }).open_mode = '';
    const { frame } = setup(sender(), t);
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(onNavigate).toHaveBeenCalledTimes(1);
  });

  it('7: bad_path when the path escapes the app', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'my-target', path: '../escape' });
    expectReply(frame, 'bad_path');
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('7: a valid path is resolved into url', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'my-target', path: 'series/foo' });
    expect(onNavigate).toHaveBeenCalledWith(expect.objectContaining({ url: 'https://t.local/app/series/foo' }));
    expect(posts.get(frame)).toHaveBeenCalledWith({ type: 'muximux:navigate-result', ok: true }, 'https://s.local');
  });

  it('echoes a request id in the reply', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'my-target', id: 'req-1' });
    expect(posts.get(frame)).toHaveBeenLastCalledWith(
      { type: 'muximux:navigate-result', ok: true, id: 'req-1' }, 'https://s.local');
    clock += 1000;
    send(frame, { type: 'muximux:navigate', app: 'my-target', id: 7 });
    expect(posts.get(frame)).toHaveBeenLastCalledWith(
      { type: 'muximux:navigate-result', ok: true, id: 7 }, 'https://s.local');
  });

  it('echoes a request id on refusals too', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'nope', id: 'r' });
    expect(posts.get(frame)).toHaveBeenLastCalledWith(
      { type: 'muximux:navigate-result', ok: false, error: 'unknown_app', id: 'r' }, 'https://s.local');
  });

  it.each([['x'.repeat(MAX_REQUEST_ID + 1)], [Number.NaN], [Infinity], [{ a: 1 }], [null]])(
    'drops an unusable request id %#', (id) => {
      const { frame } = setup();
      send(frame, { type: 'muximux:navigate', app: 'my-target', id });
      expect(posts.get(frame)).toHaveBeenLastCalledWith(
        { type: 'muximux:navigate-result', ok: true }, 'https://s.local');
    });

  it('7: empty path means no path', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'my-target', path: '' });
    expect(onNavigate).toHaveBeenCalledWith(expect.objectContaining({ url: undefined }));
  });

  it('accepts self-navigation with a path', () => {
    const s = makeApp({ name: 'Sender', url: 'https://s.local/', allow_navigation: true });
    apps = [s];
    const frame = addFrame(s);
    install();
    send(frame, { type: 'muximux:navigate', app: 'sender', path: '/page' });
    expect(onNavigate).toHaveBeenCalledWith(expect.objectContaining({ target: s, url: 'https://s.local/page' }));
  });

  it('8: rate limits at 499 ms and accepts at 500 ms', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    clock += NAV_RATE_LIMIT_MS - 1;
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expectReply(frame, 'rate_limited');
    expect(onNavigate).toHaveBeenCalledTimes(1);
    clock += 1;
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(onNavigate).toHaveBeenCalledTimes(1 + 1);
  });

  it('8: different senders have independent slots', () => {
    const s1 = sender();
    const s2 = makeApp({ name: 'Other', url: 'https://o.local', allow_navigation: true });
    apps = [s1, s2, target()];
    const f1 = addFrame(s1);
    const f2 = addFrame(s2);
    install();
    send(f1, { type: 'muximux:navigate', app: 'my-target' });
    send(f2, { type: 'muximux:navigate', app: 'my-target' }, 'https://o.local');
    expect(onNavigate).toHaveBeenCalledTimes(2);
  });

  it('an invalid message does not consume the rate-limit slot', () => {
    const { frame } = setup();
    send(frame, { type: 'muximux:navigate', app: 'nope' });
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(onNavigate).toHaveBeenCalledTimes(1);
  });

  it('survives a throwing reply postMessage', () => {
    const { frame } = setup();
    posts.get(frame)!.mockImplementation(() => { throw new Error('gone'); });
    expect(() => send(frame, { type: 'muximux:navigate', app: 'my-target' })).not.toThrow();
    expect(onNavigate).toHaveBeenCalledTimes(1);
  });

  it('1: a message without a source matches no sender and gets no reply', () => {
    const { frame } = setup();
    window.dispatchEvent(new MessageEvent('message', { data: { type: 'muximux:navigate', app: 'my-target' }, origin: 'https://s.local' }));
    expect(warn).toHaveBeenCalledWith('[muximux:navigate] no matching app for message', expect.anything());
    expect(onNavigate).not.toHaveBeenCalled();
    expect(posts.get(frame)).not.toHaveBeenCalled();
  });

  it('cleanup removes the listener', () => {
    const { frame } = setup();
    cleanup!();
    cleanup = null;
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(onNavigate).not.toHaveBeenCalled();
  });

  describe('nested senders', () => {
    // A widget iframe inside the sender app's document.
    function widgetIn(frame: HTMLIFrameElement): { win: Window; post: ReturnType<typeof vi.fn> } {
      const doc = frame.contentDocument!;
      const child = doc.createElement('iframe');
      doc.body.appendChild(child);
      const post = vi.fn();
      child.contentWindow!.postMessage = post as unknown as Window['postMessage'];
      return { win: child.contentWindow!, post };
    }

    function sendFrom(win: Window, data: unknown, origin: string) {
      window.dispatchEvent(new MessageEvent('message', { data, origin, source: win }));
    }

    it('counts a widget inside an opted-in app as that app', () => {
      const { s, t, frame } = setup();
      const widget = widgetIn(frame);
      sendFrom(widget.win, { type: 'muximux:navigate', app: 'my-target', id: 3 }, 'https://s.local');
      expect(onNavigate).toHaveBeenCalledWith({ sender: s, senderFrame: frame, target: t, url: undefined });
      expect(widget.post).toHaveBeenCalledWith(
        { type: 'muximux:navigate-result', ok: true, id: 3 },
        'https://s.local',
      );
    });

    it('accepts a widget served from another configured app', () => {
      const { frame } = setup();
      const widget = widgetIn(frame);
      sendFrom(widget.win, { type: 'muximux:navigate', app: 'my-target' }, 'https://t.local');
      expect(onNavigate).toHaveBeenCalledTimes(1);
    });

    it('refuses a widget from an address that is not an app', () => {
      const { frame } = setup();
      const widget = widgetIn(frame);
      sendFrom(widget.win, { type: 'muximux:navigate', app: 'my-target' }, 'https://widget.example');
      expect(onNavigate).not.toHaveBeenCalled();
      expect(warn).toHaveBeenCalledWith(expect.stringContaining('not one of your apps'), expect.anything());
    });

    it('drops a widget with an opaque origin without a reply', () => {
      const { frame } = setup();
      const widget = widgetIn(frame);
      sendFrom(widget.win, { type: 'muximux:navigate', app: 'my-target' }, 'null');
      expect(onNavigate).not.toHaveBeenCalled();
      expect(widget.post).not.toHaveBeenCalled();
    });

    it('does not let a direct frame with the wrong origin pass as nested', () => {
      const { frame } = setup();
      send(frame, { type: 'muximux:navigate', app: 'my-target' }, 'https://t.local');
      expect(onNavigate).not.toHaveBeenCalled();
    });

    it('refuses a widget whose app has not opted in', () => {
      const { frame } = setup(makeApp({ name: 'Sender', url: 'https://s.local' }));
      const widget = widgetIn(frame);
      sendFrom(widget.win, { type: 'muximux:navigate', app: 'my-target' }, 'https://s.local');
      expect(onNavigate).not.toHaveBeenCalled();
      expect(widget.post).toHaveBeenCalledWith(
        { type: 'muximux:navigate-result', ok: false, error: 'not_allowed' },
        'https://s.local',
      );
    });

    it('refuses a widget in a hidden app', () => {
      const { frame } = setup();
      visible = false;
      const widget = widgetIn(frame);
      sendFrom(widget.win, { type: 'muximux:navigate', app: 'my-target' }, 'https://s.local');
      expect(onNavigate).not.toHaveBeenCalled();
    });
  });

  describe('frame links', () => {
    // The frame now holds a Muximux copy: same origin, the shell's path.
    function copyFrame(app: App, pathname = window.location.pathname) {
      const frame = document.createElement('iframe');
      frame.dataset.app = app.name;
      document.body.appendChild(frame);
      frames.push(frame);
      const win = { location: { pathname } } as unknown as Window;
      Object.defineProperty(frame, 'contentWindow', { value: win });
      return { frame, win };
    }

    function link(win: Window, hash: unknown, origin = window.location.origin,
      extra: Record<string, unknown> = { referrer: 'https://s.local/board', pushed: true }) {
      window.dispatchEvent(new MessageEvent('message', {
        data: { type: 'muximux:frame-link', hash, ...extra }, origin, source: win,
      }));
    }

    function setupCopy(s = sender()) {
      apps = [s, target()];
      const copy = copyFrame(s);
      install();
      return { s, ...copy };
    }

    it('hands an accepted link to onFrameLink', () => {
      const { s, frame, win } = setupCopy();
      link(win, '#my-target');
      expect(onFrameLink).toHaveBeenCalledWith({ sender: s, senderFrame: frame, hash: '#my-target', pushed: true });
      expect(onNavigate).not.toHaveBeenCalled();
    });

    it('passes pushed only when it is exactly true', () => {
      const { win } = setupCopy();
      link(win, '#my-target', undefined, { referrer: 'https://s.local/', pushed: 'yes' });
      expect(onFrameLink).toHaveBeenCalledWith(expect.objectContaining({ pushed: false }));
    });

    it('accepts a link on another configured app\'s page', () => {
      const { win } = setupCopy();
      link(win, '#my-target', undefined, { referrer: 'https://t.local/app/x' });
      expect(onFrameLink).toHaveBeenCalledTimes(1);
    });

    it.each([
      ['a foreign page', 'https://evil.example/'],
      ['no referrer', ''],
      ['a malformed referrer', 'not a url'],
      ['a missing referrer', undefined],
    ])('only restores the app for a link on %s', (_label, referrer) => {
      const { s, frame, win } = setupCopy();
      link(win, '#my-target', undefined, { referrer, pushed: true });
      expect(onFrameLink).toHaveBeenCalledWith({ sender: s, senderFrame: frame, hash: '', pushed: true });
      expect(warn).toHaveBeenCalledWith(
        expect.stringContaining('not on the page of one of your apps'),
        expect.objectContaining({ hint: 'Give the link target="_top"' }),
      );
    });

    it('accepts an empty hash (a link to Muximux itself)', () => {
      const { win } = setupCopy();
      link(win, '');
      expect(onFrameLink).toHaveBeenCalledWith(expect.objectContaining({ hash: '' }));
    });

    it('ignores a sender that is not a Muximux copy', () => {
      const { win } = setupCopy();
      link(win, '#my-target', 'https://s.local');
      const other = copyFrame(apps[0], '/proxy/sender/');
      link(other.win, '#my-target');
      expect(onFrameLink).not.toHaveBeenCalled();
      expect(warn).not.toHaveBeenCalled();
    });

    it('ignores a copy whose location cannot be read', () => {
      apps = [sender()];
      const frame = document.createElement('iframe');
      frame.dataset.app = 'Sender';
      document.body.appendChild(frame);
      frames.push(frame);
      const win = { get location(): Location { throw new Error('cross-origin'); } } as unknown as Window;
      Object.defineProperty(frame, 'contentWindow', { value: win });
      install();
      link(win, '#my-target');
      expect(onFrameLink).not.toHaveBeenCalled();
    });

    it('ignores a window that is not a registered frame', () => {
      setupCopy();
      link(window, '#my-target');
      expect(onFrameLink).not.toHaveBeenCalled();
    });

    it('warns when the app has not opted in', () => {
      const { win } = setupCopy(makeApp({ name: 'Sender', url: 'https://s.local' }));
      link(win, '#my-target');
      expect(onFrameLink).not.toHaveBeenCalled();
      expect(warn).toHaveBeenCalledWith(
        expect.stringContaining('navigation is not enabled'),
        expect.objectContaining({ app: 'Sender' }),
      );
    });

    it('warns when the app is not visible', () => {
      const { win } = setupCopy();
      visible = false;
      link(win, '#my-target');
      expect(onFrameLink).not.toHaveBeenCalled();
      expect(warn).toHaveBeenCalled();
    });

    it.each([42, 'my-target', '#' + 'x'.repeat(MAX_FRAME_LINK_HASH)])('warns on malformed hash %j', (hash) => {
      const { win } = setupCopy();
      link(win, hash);
      expect(onFrameLink).not.toHaveBeenCalled();
      expect(warn).toHaveBeenCalledWith('[muximux:navigate] malformed frame link', expect.anything());
    });

    it('accepts a hash of exactly MAX_FRAME_LINK_HASH characters', () => {
      const { win } = setupCopy();
      link(win, '#' + 'x'.repeat(MAX_FRAME_LINK_HASH - 1));
      expect(onFrameLink).toHaveBeenCalledTimes(1);
    });

    it('shares the rate-limit slot with navigate messages, restoring a refused link', () => {
      const { win } = setupCopy();
      link(win, '#my-target');
      clock += NAV_RATE_LIMIT_MS - 1;
      link(win, '#my-target');
      expect(onFrameLink).toHaveBeenCalledTimes(2);
      expect(onFrameLink).toHaveBeenLastCalledWith(expect.objectContaining({ hash: '' }));
      expect(warn).toHaveBeenCalledWith('[muximux:navigate] navigating too fast', expect.anything());
      clock += 1;
      link(win, '#my-target');
      expect(onFrameLink).toHaveBeenLastCalledWith(expect.objectContaining({ hash: '#my-target' }));
    });

    it('ignores messages from an opaque origin before any lookup', () => {
      const { win } = setupCopy();
      link(win, '#my-target', 'null');
      expect(onFrameLink).not.toHaveBeenCalled();
      expect(warn).not.toHaveBeenCalled();
    });
  });
});

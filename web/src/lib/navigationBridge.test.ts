import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { installNavigationBridge, NAV_RATE_LIMIT_MS } from './navigationBridge';
import { makeApp, type App } from './types';

describe('navigationBridge', () => {
  let cleanup: (() => void) | null = null;
  let apps: App[];
  let visible: boolean;
  let clock: number;
  let onNavigate: ReturnType<typeof vi.fn>;
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

  it('does not reply when the event has no source', () => {
    setup();
    // Refusal path with no source: reply() must bail out quietly.
    window.dispatchEvent(new MessageEvent('message', { data: { type: 'muximux:navigate', app: 'x' }, origin: 'https://s.local' }));
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it('cleanup removes the listener', () => {
    const { frame } = setup();
    cleanup!();
    cleanup = null;
    send(frame, { type: 'muximux:navigate', app: 'my-target' });
    expect(onNavigate).not.toHaveBeenCalled();
  });
});

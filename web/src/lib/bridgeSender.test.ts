import { describe, it, expect, afterEach } from 'vitest';
import { findSenderFrame } from './bridgeSender';
import { makeApp, type App } from './types';

describe('findSenderFrame', () => {
  const frames: HTMLIFrameElement[] = [];

  afterEach(() => {
    for (const f of frames) f.remove();
    frames.length = 0;
  });

  function addFrame(app: App): HTMLIFrameElement {
    const iframe = document.createElement('iframe');
    iframe.dataset.app = app.name;
    document.body.appendChild(iframe);
    frames.push(iframe);
    return iframe;
  }

  it('matches by window handle and returns app and frame', () => {
    const app = makeApp({ name: 'A', url: 'https://a.local' });
    const frame = addFrame(app);
    const res = findSenderFrame([app], 'https://a.local', frame.contentWindow);
    expect(res?.app).toBe(app);
    expect(res?.frame).toBe(frame);
  });

  it('rejects a non-matching window handle', () => {
    const app = makeApp({ name: 'A', url: 'https://a.local' });
    addFrame(app);
    expect(findSenderFrame([app], 'https://a.local', window)).toBeUndefined();
  });

  it('rejects a null source', () => {
    const app = makeApp({ name: 'A', url: 'https://a.local' });
    addFrame(app);
    expect(findSenderFrame([app], 'https://a.local', null)).toBeUndefined();
  });

  it('rejects a cross-origin app reporting the wrong origin', () => {
    const app = makeApp({ name: 'A', url: 'https://a.local' });
    const frame = addFrame(app);
    expect(findSenderFrame([app], 'https://evil.example', frame.contentWindow)).toBeUndefined();
  });

  it('rejects an app whose configured URL is unparseable', () => {
    const app = makeApp({ name: 'A', url: 'not a url' });
    const frame = addFrame(app);
    expect(findSenderFrame([app], 'https://a.local', frame.contentWindow)).toBeUndefined();
  });

  it('accepts a proxied app reporting Muximux\'s own origin', () => {
    const app = makeApp({ name: 'P', url: 'https://p.local', proxyUrl: '/proxy/p/' });
    const frame = addFrame(app);
    const res = findSenderFrame([app], window.location.origin, frame.contentWindow);
    expect(res?.app).toBe(app);
  });

  it('rejects a proxied app reporting a foreign origin', () => {
    const app = makeApp({ name: 'P', url: 'https://p.local', proxyUrl: '/proxy/p/' });
    const frame = addFrame(app);
    expect(findSenderFrame([app], 'https://p.local', frame.contentWindow)).toBeUndefined();
  });

  it('returns undefined when the app is missing from the list', () => {
    const app = makeApp({ name: 'A', url: 'https://a.local' });
    const frame = addFrame(app);
    expect(findSenderFrame([], 'https://a.local', frame.contentWindow)).toBeUndefined();
  });
});

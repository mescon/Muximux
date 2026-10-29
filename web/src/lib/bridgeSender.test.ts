import { describe, it, expect, afterEach } from 'vitest';
import { findNestedSenderFrame, findSenderFrame, frameOfWindow, isAppOrigin, MAX_NEST_DEPTH } from './bridgeSender';
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
    const app = makeApp({ name: 'A', url: 'http://' });
    const frame = addFrame(app);
    expect(findSenderFrame([app], 'https://a.local', frame.contentWindow)).toBeUndefined();
  });

  it('matches a single-slash app URL against Muximux\'s own origin', () => {
    const app = makeApp({ name: 'L', url: '/local/' });
    const frame = addFrame(app);
    expect(findSenderFrame([app], window.location.origin, frame.contentWindow)?.app).toBe(app);
    expect(findSenderFrame([app], 'https://evil.test', frame.contentWindow)).toBeUndefined();
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

describe('frameOfWindow', () => {
  afterEach(() => {
    document.querySelectorAll('iframe').forEach(f => f.remove());
  });

  it('matches by window handle without any origin check', () => {
    const app = makeApp({ name: 'A', url: 'https://a.local' });
    const frame = document.createElement('iframe');
    frame.dataset.app = 'A';
    document.body.appendChild(frame);
    expect(frameOfWindow([app], frame.contentWindow)).toEqual({ app, frame });
    expect(frameOfWindow([app], null)).toBeUndefined();
    expect(frameOfWindow([], frame.contentWindow)).toBeUndefined();
  });
});

describe('findNestedSenderFrame', () => {
  afterEach(() => {
    document.querySelectorAll('iframe').forEach(f => f.remove());
  });

  function appFrame(name: string): HTMLIFrameElement {
    const frame = document.createElement('iframe');
    frame.dataset.app = name;
    document.body.appendChild(frame);
    return frame;
  }

  // An iframe inside `parent`'s document, as a dashboard widget would be.
  function nest(parent: HTMLIFrameElement): HTMLIFrameElement {
    const doc = parent.contentDocument!;
    const child = doc.createElement('iframe');
    doc.body.appendChild(child);
    return child;
  }

  it('finds the app frame a widget sits in', () => {
    const app = makeApp({ name: 'Homarr', url: 'https://h.local' });
    const frame = appFrame('Homarr');
    const widget = nest(frame);
    expect(findNestedSenderFrame([app], widget.contentWindow)).toEqual({ app, frame });
  });

  it('walks several levels up', () => {
    const app = makeApp({ name: 'Homarr', url: 'https://h.local' });
    const frame = appFrame('Homarr');
    const deep = nest(nest(frame));
    expect(findNestedSenderFrame([app], deep.contentWindow)?.frame).toBe(frame);
  });

  it('accepts MAX_NEST_DEPTH levels below the app frame and no more', () => {
    const app = makeApp({ name: 'Homarr', url: 'https://h.local' });
    let f = appFrame('Homarr');
    for (let i = 0; i < MAX_NEST_DEPTH; i++) f = nest(f);
    expect(findNestedSenderFrame([app], f.contentWindow)?.app).toBe(app);
    expect(findNestedSenderFrame([app], nest(f).contentWindow)).toBeUndefined();
  });

  it('does not match the app frame itself or the shell', () => {
    const app = makeApp({ name: 'Homarr', url: 'https://h.local' });
    const frame = appFrame('Homarr');
    expect(findNestedSenderFrame([app], frame.contentWindow)).toBeUndefined();
    expect(findNestedSenderFrame([app], window)).toBeUndefined();
  });

  it('rejects frames outside any app and non-window sources', () => {
    const app = makeApp({ name: 'Homarr', url: 'https://h.local' });
    const stray = document.createElement('iframe');
    document.body.appendChild(stray);
    const widget = nest(stray);
    expect(findNestedSenderFrame([app], widget.contentWindow)).toBeUndefined();
    expect(findNestedSenderFrame([app], null)).toBeUndefined();
    expect(findNestedSenderFrame([app], {} as MessagePort)).toBeUndefined();
  });

  it('treats a window that throws on parent as unknown', () => {
    const app = makeApp({ name: 'Homarr', url: 'https://h.local' });
    const source = { get parent(): Window { throw new Error('gone'); } } as unknown as Window;
    expect(findNestedSenderFrame([app], source)).toBeUndefined();
  });
});

describe('isAppOrigin', () => {
  const apps = [
    makeApp({ name: 'A', url: 'https://a.local/x/' }),
    makeApp({ name: 'L', url: '/local/' }),
    makeApp({ name: 'Bad', url: 'http://' }),
  ];

  it('matches configured app origins, including Muximux\'s own for local URLs', () => {
    expect(isAppOrigin(apps, 'https://a.local')).toBe(true);
    expect(isAppOrigin(apps, window.location.origin)).toBe(true);
  });

  it('rejects other origins and the opaque origin', () => {
    expect(isAppOrigin(apps, 'https://evil.example')).toBe(false);
    expect(isAppOrigin(apps, 'null')).toBe(false);
    expect(isAppOrigin([], 'https://a.local')).toBe(false);
  });
});

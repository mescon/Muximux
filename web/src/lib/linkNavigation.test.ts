import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  applyLinkHash,
  applyNavigate,
  consumeDeepLink,
  deepLinkKey,
  isFrameVisible,
  layoutHash,
  linkSenderPane,
  paneOf,
  setDeepLink,
  type DeepLinks,
  type Overlays,
} from './linkNavigation';
import { splitState, enableSplit, resetSplit, setPanelApp } from './splitStore.svelte';
import { NAV_STATE, isMuximuxEntry } from './navHistory';
import { makeApp, type App } from './types';

const A = makeApp({ name: 'App A', url: 'https://a.local' });
const B = makeApp({ name: 'App B', url: 'https://b.local' });
const C = makeApp({ name: 'App C', url: 'https://c.local' });

const NONE: Overlays = { splash: false, logs: false, settings: false, commandPalette: false, shortcuts: false };

// Mirrors App.svelte: split view puts each pane's frames in [data-pane]
// containers; single view has no pane containers.
let root: HTMLDivElement;

function frameIn(container: Element, app: App): HTMLIFrameElement {
  const f = document.createElement('iframe');
  f.dataset.app = app.name;
  container.appendChild(f);
  return f;
}

function pane(n: 0 | 1): HTMLDivElement {
  const d = document.createElement('div');
  d.dataset.pane = String(n);
  root.appendChild(d);
  return d;
}

/** Split [left | right], active pane `active`. */
function split(left: App | null, right: App | null, active: 0 | 1 = 0): void {
  enableSplit('horizontal');
  splitState.panels[0] = left;
  splitState.panels[1] = right;
  splitState.activePanel = active;
}

beforeEach(() => {
  resetSplit();
  root = document.createElement('div');
  document.body.appendChild(root);
});

afterEach(() => {
  root.remove();
  history.replaceState(null, '', '/');
});

describe('deep link store', () => {
  it('keys by pane and app name', () => {
    expect(deepLinkKey(1, 'App A')).toBe('1:App A');
  });

  it('gives every set a new seq, also after the key was consumed', () => {
    const links: DeepLinks = {};
    setDeepLink(links, '0:A', 'https://a.local/x');
    const first = links['0:A'].seq;
    consumeDeepLink(links, '0:A', first);
    expect(links['0:A']).toBeUndefined();
    setDeepLink(links, '0:A', 'https://a.local/x');
    expect(links['0:A'].seq).toBeGreaterThan(first);
  });

  it('keeps a newer deep link when an older seq is consumed', () => {
    const links: DeepLinks = {};
    setDeepLink(links, '0:A', 'https://a.local/1');
    const old = links['0:A'].seq;
    setDeepLink(links, '0:A', 'https://a.local/2');
    consumeDeepLink(links, '0:A', old);
    expect(links['0:A'].url).toBe('https://a.local/2');
  });

  it('consuming a missing key is a no-op', () => {
    const links: DeepLinks = {};
    consumeDeepLink(links, '1:A', 3);
    expect(links).toEqual({});
  });
});

describe('paneOf', () => {
  it('reads the closest data-pane', () => {
    expect(paneOf(frameIn(pane(0), A))).toBe(0);
    expect(paneOf(frameIn(pane(1), A))).toBe(1);
  });

  it('is null outside a pane, for a bad value, and for null', () => {
    expect(paneOf(frameIn(root, A))).toBeNull();
    const odd = document.createElement('div');
    odd.dataset.pane = '2';
    root.appendChild(odd);
    expect(paneOf(frameIn(odd, A))).toBeNull();
    expect(paneOf(null)).toBeNull();
  });
});

describe('isFrameVisible', () => {
  it('single view: only the frame of the shown app', () => {
    setPanelApp(A);
    expect(isFrameVisible(frameIn(root, A), NONE)).toBe(true);
    expect(isFrameVisible(frameIn(root, B), NONE)).toBe(false);
  });

  it('split view: maps each frame to its own pane', () => {
    split(A, B);
    const p0 = pane(0);
    const p1 = pane(1);
    expect(isFrameVisible(frameIn(p0, A), NONE)).toBe(true);
    expect(isFrameVisible(frameIn(p1, B), NONE)).toBe(true);
    // Hidden frames kept alive in each pane for other visited apps.
    expect(isFrameVisible(frameIn(p0, B), NONE)).toBe(false);
    expect(isFrameVisible(frameIn(p1, A), NONE)).toBe(false);
    expect(isFrameVisible(frameIn(p1, C), NONE)).toBe(false);
  });

  it('split view: a frame outside any pane is not visible', () => {
    split(A, B);
    expect(isFrameVisible(frameIn(root, A), NONE)).toBe(false);
  });

  it('split view with an empty pane', () => {
    split(A, null);
    expect(isFrameVisible(frameIn(pane(1), A), NONE)).toBe(false);
  });

  it.each(Object.keys(NONE))('refuses while %s covers the frames', (key) => {
    setPanelApp(A);
    expect(isFrameVisible(frameIn(root, A), { ...NONE, [key]: true })).toBe(false);
  });
});

describe('applyNavigate', () => {
  let links: DeepLinks;
  let trackVisit: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    links = {};
    trackVisit = vi.fn();
  });

  it('single view: shows the target, deep link keyed on pane 0', () => {
    setPanelApp(A);
    const sender = frameIn(root, A);
    expect(applyNavigate({ senderFrame: sender, target: B, url: 'https://b.local/x' }, links, trackVisit)).toBe(0);
    expect(splitState.panels[0]).toEqual(B);
    expect(links['0:App B'].url).toBe('https://b.local/x');
    expect(trackVisit).toHaveBeenCalledWith('App B');
  });

  it('without a url stores no deep link', () => {
    setPanelApp(A);
    applyNavigate({ senderFrame: frameIn(root, A), target: B }, links, trackVisit);
    expect(links).toEqual({});
    expect(splitState.panels[0]).toEqual(B);
  });

  it('split: target not shown replaces the sender pane (pane 1), other pane untouched', () => {
    split(A, C, 0);
    const sender = frameIn(pane(1), C);
    expect(applyNavigate({ senderFrame: sender, target: B, url: 'https://b.local/x' }, links, trackVisit)).toBe(1);
    expect(splitState.panels).toEqual([A, B]);
    expect(splitState.activePanel).toBe(1);
    expect(Object.keys(links)).toEqual(['1:App B']);
  });

  it('split: target not shown replaces the sender pane (pane 0)', () => {
    split(A, C, 1);
    const sender = frameIn(pane(0), A);
    expect(applyNavigate({ senderFrame: sender, target: B }, links, trackVisit)).toBe(0);
    expect(splitState.panels).toEqual([B, C]);
    expect(splitState.activePanel).toBe(0);
  });

  it('split: target in the other pane only activates it, deep link keyed there', () => {
    split(A, B, 0);
    const sender = frameIn(pane(0), A);
    expect(applyNavigate({ senderFrame: sender, target: B, url: 'https://b.local/x' }, links, trackVisit)).toBe(1);
    expect(splitState.panels).toEqual([A, B]);
    expect(splitState.activePanel).toBe(1);
    expect(Object.keys(links)).toEqual(['1:App B']);
  });

  it('split: target in the sender pane (self) stays there', () => {
    split(A, B, 1);
    const sender = frameIn(pane(0), A);
    expect(applyNavigate({ senderFrame: sender, target: A, url: 'https://a.local/y' }, links, trackVisit)).toBe(0);
    expect(splitState.panels).toEqual([A, B]);
    expect(Object.keys(links)).toEqual(['0:App A']);
  });

  it('split: a sender outside any pane falls back to the active pane', () => {
    split(A, C, 1);
    applyNavigate({ senderFrame: frameIn(root, C), target: B }, links, trackVisit);
    expect(splitState.panels).toEqual([A, B]);
  });

  it('a repeated deep link gets a new seq', () => {
    setPanelApp(B);
    const sender = frameIn(root, B);
    applyNavigate({ senderFrame: sender, target: B, url: 'https://b.local/x' }, links, trackVisit);
    const first = links['0:App B'].seq;
    applyNavigate({ senderFrame: sender, target: B, url: 'https://b.local/x' }, links, trackVisit);
    expect(links['0:App B'].seq).toBeGreaterThan(first);
  });
});

describe('linkSenderPane', () => {
  it('uses the focused iframe\'s pane', () => {
    split(A, C, 1);
    const f = frameIn(pane(0), A);
    const spy = vi.spyOn(document, 'activeElement', 'get').mockReturnValue(f);
    expect(linkSenderPane()).toBe(0);
    spy.mockRestore();
  });

  it('falls back to activePanel when the focused element is not an iframe', () => {
    split(A, C, 1);
    expect(linkSenderPane()).toBe(1);
  });

  it('falls back to activePanel when the focused iframe is outside any pane', () => {
    split(A, C, 0);
    const f = frameIn(root, A);
    const spy = vi.spyOn(document, 'activeElement', 'get').mockReturnValue(f);
    expect(linkSenderPane()).toBe(0);
    splitState.activePanel = 1;
    expect(linkSenderPane()).toBe(1);
    spy.mockRestore();
  });
});

describe('layoutHash', () => {
  it('is empty with nothing shown', () => {
    expect(layoutHash()).toBe('');
  });

  it('single app', () => {
    setPanelApp(A);
    expect(layoutHash()).toBe('#app-a');
  });

  it('split with both panes and with one', () => {
    split(A, B);
    expect(layoutHash()).toBe('#app-a+app-b');
    split(null, B);
    expect(layoutHash()).toBe('#app-b');
  });
});

describe('applyLinkHash', () => {
  let trackVisit: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    trackVisit = vi.fn();
    history.replaceState(NAV_STATE, '', '/#app-a+app-c');
  });

  // What a target="_top" link click does: a new, untagged entry.
  function clickLink(hash: string): void {
    history.pushState(null, '', hash);
  }

  it('split link from pane 0: replaces the sender pane and the link entry', () => {
    split(A, C, 1);
    const sender = frameIn(pane(0), A);
    const focus = vi.spyOn(document, 'activeElement', 'get').mockReturnValue(sender);
    const before = history.length;
    clickLink('#app-b');
    const afterClick = history.length;
    expect(afterClick).toBe(before + 1);
    const push = vi.spyOn(history, 'pushState');

    const hash = applyLinkHash(B, { entryState: history.state, isMobile: false, trackVisit });

    expect(hash).toBe('#app-b+app-c');
    expect(splitState.panels).toEqual([B, C]);
    expect(splitState.activePanel).toBe(0);
    expect(trackVisit).toHaveBeenCalledWith('App B');
    // Normalising is a replace: the click leaves exactly one new entry.
    expect(push).not.toHaveBeenCalled();
    expect(history.length).toBe(afterClick);
    expect(location.hash).toBe('#app-b+app-c');
    expect(isMuximuxEntry(history.state)).toBe(true);
    push.mockRestore();
    focus.mockRestore();
  });

  it('split link to the app in the other pane only activates that pane', () => {
    split(A, B, 0);
    clickLink('#app-b');
    const len = history.length;
    expect(applyLinkHash(B, { entryState: history.state, isMobile: false, trackVisit })).toBe('#app-a+app-b');
    expect(splitState.panels).toEqual([A, B]);
    expect(splitState.activePanel).toBe(1);
    expect(history.length).toBe(len);
  });

  it('split link with an empty other pane', () => {
    split(A, null, 0);
    clickLink('#app-b');
    expect(applyLinkHash(B, { entryState: history.state, isMobile: false, trackVisit })).toBe('#app-b');
    expect(splitState.panels).toEqual([B, null]);
  });

  it('Back/Forward onto a tagged entry takes today\'s path', () => {
    split(A, C, 0);
    history.replaceState(NAV_STATE, '', '#app-b');
    expect(applyLinkHash(B, { entryState: history.state, isMobile: false, trackVisit })).toBeNull();
    expect(splitState.panels).toEqual([A, C]);
    expect(trackVisit).not.toHaveBeenCalled();
  });

  it('outside split view takes today\'s path', () => {
    setPanelApp(A);
    clickLink('#app-b');
    expect(applyLinkHash(B, { entryState: history.state, isMobile: false, trackVisit })).toBeNull();
    expect(splitState.panels[0]).toEqual(A);
  });

  it('on mobile takes today\'s path', () => {
    split(A, C, 0);
    clickLink('#app-b');
    expect(applyLinkHash(B, { entryState: history.state, isMobile: true, trackVisit })).toBeNull();
  });

  it('a target that does not open in an iframe takes today\'s path', () => {
    split(A, C, 0);
    clickLink('#app-b');
    const tab = makeApp({ name: 'App B', url: 'https://b.local', open_mode: 'new_tab' });
    expect(applyLinkHash(tab, { entryState: history.state, isMobile: false, trackVisit })).toBeNull();
  });

  it('an unset (empty) open_mode counts as iframe', () => {
    split(A, C, 0);
    clickLink('#app-b');
    const unset = { ...B, open_mode: '' } as unknown as App;
    expect(applyLinkHash(unset, { entryState: history.state, isMobile: false, trackVisit })).toBe('#app-b+app-c');
  });
});

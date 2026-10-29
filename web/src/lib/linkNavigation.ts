/**
 * App navigation decisions for the shell (issue #529).
 *
 * App.svelte only wires these into its state; the rules live here so they
 * can be tested:
 * - which frames count as visible for the navigation bridge,
 * - where a `muximux:navigate` request lands and how its deep link is
 *   handed to the pane's AppFrame,
 * - how a plain `target="_top"` link to `#app` is applied while split.
 */

import type { App } from './types';
import { slugify } from './slug';
import { splitState, paneShowing, focusOrPlace } from './splitStore.svelte';
import { NAV_STATE, isMuximuxEntry } from './navHistory';

export type Pane = 0 | 1;

export interface DeepLink {
  url: string;
  seq: number;
}

/** Pending deep links, keyed by deepLinkKey(pane, app name). */
export type DeepLinks = Record<string, DeepLink>;

export function deepLinkKey(pane: Pane, appName: string): string {
  return `${pane}:${appName}`;
}

// One counter for all keys: a key that was consumed and is set again must
// get a seq its (still mounted) frame has not seen yet.
let seqCounter = 0;

export function setDeepLink(links: DeepLinks, key: string, url: string): void {
  seqCounter += 1;
  links[key] = { url, seq: seqCounter };
}

/**
 * Drop a deep link once its frame has used it, so a later remount of that
 * frame (LRU eviction, split toggle) loads the app's normal URL again. A
 * newer deep link set under the same key in the meantime is kept.
 */
export function consumeDeepLink(links: DeepLinks, key: string, seq: number): void {
  if (links[key]?.seq === seq) delete links[key];
}

/** The split pane an element sits in, from the closest [data-pane]. */
export function paneOf(el: Element | null): Pane | null {
  const attr = el?.closest('[data-pane]')?.getAttribute('data-pane');
  if (attr === '0') return 0;
  if (attr === '1') return 1;
  return null;
}

/** Shell layers that cover the app frames. */
export interface Overlays {
  splash: boolean;
  logs: boolean;
  settings: boolean;
  commandPalette: boolean;
  shortcuts: boolean;
}

/**
 * A frame is visible when no overlay covers the frames and its app is the
 * one shown in its pane (pane 0 outside split view). Every pane keeps a
 * hidden frame per visited app; those must not count.
 */
export function isFrameVisible(frame: HTMLIFrameElement, overlays: Overlays): boolean {
  if (overlays.splash || overlays.logs || overlays.settings || overlays.commandPalette || overlays.shortcuts) {
    return false;
  }
  const pane = splitState.enabled ? paneOf(frame) : 0;
  if (pane === null) return false;
  return splitState.panels[pane]?.name === frame.dataset.app;
}

/**
 * Apply an accepted navigate request: the pane already showing the target
 * becomes active, otherwise the target replaces the sender's pane. A deep
 * link is stored for the target's frame in that pane before trackVisit, so
 * a never-visited frame mounts straight at it. Returns the pane used.
 */
export function applyNavigate(
  req: { senderFrame: HTMLIFrameElement; target: App; url?: string },
  links: DeepLinks,
  trackVisit: (appName: string) => void,
): Pane {
  const senderPane: Pane = splitState.enabled ? (paneOf(req.senderFrame) ?? splitState.activePanel) : 0;
  if (req.url) {
    const pane = paneShowing(req.target.name) ?? senderPane;
    setDeepLink(links, deepLinkKey(pane, req.target.name), req.url);
  }
  trackVisit(req.target.name);
  return focusOrPlace(req.target, senderPane);
}

export const FRAME_POLL_MS = 200;

/**
 * Report the split pane of the app frame the user clicks into. Clicks inside
 * an iframe never reach the shell, and Chromium fires no focus event on the
 * iframe element either. What every browser does is blur the shell window on
 * the first click and point document.activeElement at the clicked frame;
 * moving between two frames after that changes activeElement silently. So
 * the frame is checked on window blur and then polled until the shell has
 * focus again. The capture focus listener covers browsers that do fire it.
 */
export function installFramePaneTracker(onPane: (pane: Pane) => void): () => void {
  let last: Element | null = null;
  let timer: ReturnType<typeof setInterval> | undefined;

  const check = () => {
    const el = document.activeElement;
    if (el === last) return;
    last = el;
    const pane = el instanceof HTMLIFrameElement ? paneOf(el) : null;
    if (pane !== null) onPane(pane);
  };
  const stop = () => {
    clearInterval(timer);
    timer = undefined;
    last = null;
  };
  const onBlur = () => {
    check();
    if (timer === undefined) timer = setInterval(check, FRAME_POLL_MS);
  };
  const onFocus = (e: FocusEvent) => {
    if (e.target instanceof HTMLIFrameElement) check();
  };
  // A hidden tab needs no polling. Coming back with a frame still focused
  // fires no window focus or blur, so resume from here.
  const onVisibility = () => {
    if (document.hidden) {
      clearInterval(timer);
      timer = undefined;
    } else if (document.activeElement instanceof HTMLIFrameElement) {
      onBlur();
    }
  };

  window.addEventListener('blur', onBlur);
  window.addEventListener('focus', stop);
  document.addEventListener('focus', onFocus, true);
  document.addEventListener('visibilitychange', onVisibility);
  return () => {
    stop();
    window.removeEventListener('blur', onBlur);
    window.removeEventListener('focus', stop);
    document.removeEventListener('focus', onFocus, true);
    document.removeEventListener('visibilitychange', onVisibility);
  };
}

/**
 * The pane a plain link came from. The clicked frame is the focused element;
 * activePanel is only the fallback for a browser that reports no focus.
 */
export function linkSenderPane(): Pane {
  const focused = document.activeElement;
  const pane = focused instanceof HTMLIFrameElement ? paneOf(focused) : null;
  return pane ?? splitState.activePanel;
}

/** The Muximux hash for the current layout ('' when nothing is shown). */
export function layoutHash(): string {
  const [a, b] = splitState.panels;
  if (splitState.enabled && a && b) return '#' + slugify(a.name) + '+' + slugify(b.name);
  const app = a || b;
  return app ? '#' + slugify(app.name) : '';
}

/**
 * Handle a single-app hash that arrived through hashchange. Only a link
 * click (an entry Muximux did not tag) while split, on desktop, to an app
 * that opens inside Muximux takes the new path: focus the pane already
 * showing the app or place it in the sender's pane, never emptying the
 * other pane, then replace the link's entry with the layout hash so the
 * click leaves exactly one history entry. Returns that hash, or null when
 * the caller should take today's path (Back/Forward onto a tagged entry,
 * no split, mobile, non-iframe app).
 */
export function applyLinkHash(
  app: App,
  ctx: { entryState: unknown; isMobile: boolean; trackVisit: (appName: string) => void },
): string | null {
  if (isMuximuxEntry(ctx.entryState) || !splitState.enabled || ctx.isMobile) return null;
  if (app.open_mode && app.open_mode !== 'iframe') return null;
  const pane = linkSenderPane();
  ctx.trackVisit(app.name);
  focusOrPlace(app, pane);
  const hash = layoutHash();
  history.replaceState(NAV_STATE, '', hash);
  return hash;
}

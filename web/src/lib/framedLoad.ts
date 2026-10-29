/**
 * Muximux loaded inside one of its own app frames (issue #529).
 *
 * A link without target="_top" in an embedded app, such as a Homarr tile set
 * to open in the current tab, navigates the app's own frame. A link to
 * Muximux therefore loads Muximux inside Muximux; the browser allows it
 * because every frame in the chain is Muximux's origin once the app page is
 * gone. Instead of starting, the copy reports its hash to the shell, which
 * puts the app back in the frame and then applies the hash as if the link
 * had used target="_top".
 *
 * The shell checks the sending frame, the app's allow_navigation and the
 * page the link was on in navigationBridge.ts before onFrameLink runs.
 * Proxied apps never get here: frameRescue.ts sends such a frame back to
 * its proxy path first.
 */

import type { App } from './types';
import type { FrameLinkRequest } from './navigationBridge';
import { getEffectiveUrl } from './types';
import { isSafeAppUrl } from './appUrl';
import { slugify } from './slug';

export const FRAME_RESTORE_TIMEOUT_MS = 3000;

interface NavigationActivation {
  navigationType?: string;
}

/**
 * Copy side: send the hash to the shell, with the page the link was on
 * (document.referrer) and whether this load added a history entry. Only a
 * fresh navigation directly inside the shell counts. A copy reached by
 * Back/Forward or a reload (an app's history leading back to Muximux's own
 * page) is left blank as before.
 */
export function reportFramedLoad(win: Window = window): void {
  if (win.parent === win || win.parent !== win.top) return;
  const entry = win.performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined;
  if (entry?.type !== 'navigate') return;
  // The Navigation API says whether this load pushed an entry; a replace or
  // the frame's first load did not, and going back would undo something
  // else. Without the API the shell takes the safe path.
  const activation = (win as Window & { navigation?: { activation?: NavigationActivation } })
    .navigation?.activation;
  win.parent.postMessage({
    type: 'muximux:frame-link',
    hash: win.location.hash,
    referrer: win.document.referrer,
    pushed: activation?.navigationType === 'push',
  }, win.location.origin);
}

/** The URL an app's frame normally loads, or about:blank if it is unsafe. */
function appFrameUrl(app: App): string {
  const raw = getEffectiveUrl(app);
  return isSafeAppUrl(raw) ? raw : 'about:blank';
}

/**
 * Shell side: put the app back in its frame and resolve once it has loaded
 * (or after FRAME_RESTORE_TIMEOUT_MS). When the link's load pushed a
 * history entry, going back one entry returns the exact page the link was
 * on. Otherwise the app's URL replaces the copy, which leaves no entry
 * behind. Waiting matters: a hash change pushed before a pending traversal
 * lands would be undone by it. Load events while the frame still holds the
 * copy do not count, and a timeout with the copy still there falls back to
 * the replace.
 */
export function restoreFrame(
  frame: HTMLIFrameElement,
  app: App,
  pushed: boolean,
  timeoutMs = FRAME_RESTORE_TIMEOUT_MS,
): Promise<void> {
  const holdsCopy = () => {
    try {
      return frame.contentWindow?.location.pathname === window.location.pathname;
    } catch {
      return false; // The app's page is cross-origin, so it is back.
    }
  };
  const replace = () => {
    const url = appFrameUrl(app);
    try {
      frame.contentWindow!.location.replace(url);
    } catch {
      frame.src = url;
    }
  };
  return new Promise((resolve) => {
    const done = () => {
      clearTimeout(timer);
      frame.removeEventListener('load', onLoad);
      resolve();
    };
    const onLoad = () => {
      if (!holdsCopy()) done();
    };
    const timer = setTimeout(() => {
      if (holdsCopy()) replace();
      done();
    }, timeoutMs);
    frame.addEventListener('load', onLoad);
    if (pushed) history.back();
    else replace();
  });
}

/**
 * Apply the copy's hash. A single app that opens in an iframe goes through
 * the navigate rules (reuse the pane showing it, else the sender's pane).
 * Anything else (a split '#a+b', a reserved hash) is set on the shell, the
 * same as a target="_top" link. An empty hash only restored the app.
 */
export function applyFrameLink(
  req: FrameLinkRequest,
  apps: App[],
  actions: {
    navigate: (req: { senderFrame: HTMLIFrameElement; target: App }) => void;
    setHash: (hash: string) => void;
  },
): void {
  const slug = req.hash.slice(1);
  if (!slug) return;
  const target = apps.find(a => a.enabled !== false && slugify(a.name) === slug);
  if (target && (!target.open_mode || target.open_mode === 'iframe')) {
    actions.navigate({ senderFrame: req.senderFrame, target });
  } else {
    actions.setHash(req.hash);
  }
}

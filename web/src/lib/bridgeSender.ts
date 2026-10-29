import type { App } from './types';

/**
 * Identify which registered app iframe a window message came from.
 *
 * The unforgeable identity is `source === frame.contentWindow`: a postMessage
 * can spoof origin (same-origin proxied apps all share window.location.origin)
 * but it cannot pretend to be a different window handle. Iterate iframes and
 * match by source only.
 *
 * As a second line of defence we also check the reported `origin`: a
 * non-proxied (cross-origin) app must report the origin of its configured
 * URL, and a proxied app is served from Muximux itself, so it must report
 * Muximux's own origin.
 *
 * No window-handle match means the sender is not one of our registered
 * iframes, so nothing is returned. This closes the cross-app spoofing path:
 * any same-origin proxied app could otherwise act on behalf of a different
 * registered app merely by matching its origin.
 */
export function findSenderFrame(
  apps: App[],
  origin: string,
  source: MessageEventSource | null,
): { app: App; frame: HTMLIFrameElement } | undefined {
  const found = frameOfWindow(apps, source);
  if (!found) return undefined;
  const { app } = found;
  if (app.proxyUrl) {
    if (origin !== window.location.origin) return undefined;
  } else {
    try {
      // A single-slash URL (/local/) is served from Muximux's own origin.
      if (new URL(app.url, window.location.origin).origin !== origin) return undefined;
    } catch {
      return undefined;
    }
  }
  return found;
}

/**
 * The registered app frame whose window is `source`, by window handle only.
 * Callers add whatever origin check fits their message.
 */
export function frameOfWindow(
  apps: App[],
  source: MessageEventSource | null,
): { app: App; frame: HTMLIFrameElement } | undefined {
  if (!source) return undefined;
  const frames = document.querySelectorAll<HTMLIFrameElement>('iframe[data-app]');
  for (const frame of frames) {
    if (frame.contentWindow !== source) continue;
    const app = apps.find(a => a.name === frame.dataset.app);
    return app ? { app, frame } : undefined;
  }
  return undefined;
}

export const MAX_NEST_DEPTH = 5;

/**
 * The registered app frame that contains `source` further down, for a
 * message from a frame nested inside an app (a Homarr iframe widget, say).
 * Walks up `parent`, which is readable across origins, at most
 * MAX_NEST_DEPTH levels. No origin is checked here: whatever page the app
 * frame shows can nest a frame, so the caller must check the sender's
 * origin itself (see isAppOrigin).
 */
export function findNestedSenderFrame(
  apps: App[],
  source: MessageEventSource | null,
): { app: App; frame: HTMLIFrameElement } | undefined {
  if (!source || !('parent' in source)) return undefined;
  let win: Window = source;
  try {
    for (let depth = 0; depth < MAX_NEST_DEPTH; depth++) {
      const up = win.parent;
      if (up === win || up === window) return undefined;
      const found = frameOfWindow(apps, up);
      if (found) return found;
      win = up;
    }
  } catch {
    // A window that went away mid-walk; treat as unknown.
  }
  return undefined;
}

/**
 * Whether `origin` is the origin of a configured app's URL. A single-slash
 * URL (/local/) counts as Muximux's own origin. Unparseable URLs and the
 * opaque origin "null" never match.
 */
export function isAppOrigin(apps: App[], origin: string): boolean {
  if (origin === 'null') return false;
  return apps.some((a) => {
    try {
      return new URL(a.url, window.location.origin).origin === origin;
    } catch {
      return false;
    }
  });
}

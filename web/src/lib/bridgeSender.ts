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
  const frames = document.querySelectorAll<HTMLIFrameElement>('iframe[data-app]');
  for (const frame of frames) {
    if (!source || frame.contentWindow !== source) continue;
    const name = frame.dataset.app;
    const app = apps.find(a => a.name === name);
    if (!app) return undefined;
    if (app.proxyUrl) {
      if (origin !== window.location.origin) return undefined;
    } else {
      try {
        if (new URL(app.url).origin !== origin) return undefined;
      } catch {
        return undefined;
      }
    }
    return { app, frame };
  }
  return undefined;
}

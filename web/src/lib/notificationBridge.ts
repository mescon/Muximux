/**
 * Notification bridge for embedded apps.
 *
 * Modern browsers deny the Notifications API to cross-origin iframes, even
 * when the iframe's backend has VAPID keys and permission at the OS level.
 * This bridge lets opted-in apps postMessage Muximux's top-level window to
 * request a notification, which Muximux then shows under its own origin.
 *
 * Protocol:
 *   window.parent.postMessage(
 *     { type: 'muximux:notify', title, body, tag, url },
 *     '*'
 *   );
 *
 * Rendering:
 *   Android Chrome, Samsung Browser, and mobile Firefox do not implement
 *   the Notification() constructor at all - it throws TypeError. The
 *   ServiceWorkerRegistration.showNotification() path is the only one
 *   that works on mobile, so we prefer it when a service worker is
 *   registered and fall back to the constructor only on platforms where
 *   no SW is available (e.g. dev servers or no-SW browsers). The SW
 *   (web/public/sw.js) has a notificationclick handler that posts back
 *   to the active client, which is routed to onActivate via a
 *   navigator.serviceWorker message listener.
 *
 * Security:
 * - The app must have allow_notifications: true in its Muximux config
 * - The message sender's origin must match the app's configured URL origin
 *   (for proxied apps, must be the top-level Muximux origin)
 * - Input fields are length-capped; icon is always the app's configured icon
 *   (bridge callers cannot supply arbitrary image URLs)
 * - Clicking the notification switches to the app in Muximux, never to an
 *   arbitrary URL
 *
 * Rate limit: at most one notification per app every 2 seconds.
 */

import type { App } from './types';
import { debug } from './debug';

const RATE_LIMIT_MS = 2000;
const MAX_TITLE = 120;
const MAX_BODY = 400;
const MAX_TAG = 80;

interface NotifyMessage {
  type: 'muximux:notify';
  title?: unknown;
  body?: unknown;
  tag?: unknown;
  url?: unknown;
}

type PermissionQuery = { type: 'muximux:notify-query-permission' };
type PermissionRequest = { type: 'muximux:notify-request-permission' };
type BridgeMessage = NotifyMessage | PermissionQuery | PermissionRequest;

const BRIDGE_TYPES = new Set<string>([
  'muximux:notify',
  'muximux:notify-query-permission',
  'muximux:notify-request-permission',
]);

function isBridgeMessage(data: unknown): data is BridgeMessage {
  if (!data || typeof data !== 'object') return false;
  const type = (data as { type?: unknown }).type;
  return typeof type === 'string' && BRIDGE_TYPES.has(type);
}

function truncate(v: unknown, max: number): string {
  if (typeof v !== 'string') return '';
  return v.length > max ? v.slice(0, max) : v;
}

export interface NotificationBridgeOptions {
  /** Getter for the current list of apps so the bridge sees live updates. */
  getApps: () => App[];
  /** Called when the user clicks the notification. */
  onActivate: (app: App) => void;
  /** Base URL for resolving relative icon paths. */
  baseUrl?: string;
}

export function installNotificationBridge(opts: NotificationBridgeOptions): () => void {
  const lastNotified = new Map<string, number>();

  async function ensurePermission(): Promise<NotificationPermission> {
    if (!('Notification' in window)) return 'denied';
    if (Notification.permission === 'granted' || Notification.permission === 'denied') {
      return Notification.permission;
    }
    try {
      return await Notification.requestPermission();
    } catch {
      return 'denied';
    }
  }

  // Prefer the service worker's showNotification() because mobile browsers
  // (Android Chrome / Samsung / Firefox) do not implement the Notification
  // constructor. The SW is registered once in App.svelte; here we wait on
  // navigator.serviceWorker.ready so the first call after page load doesn't
  // race the registration. The fallback constructor is kept for desktop
  // browsers without a controlling SW (dev servers, WebView-based hosts).
  async function showNotificationForApp(app: App, title: string, options: NotificationOptions): Promise<void> {
    const dataCarrier = { ...(options.data as Record<string, unknown> | undefined), muximuxApp: app.name };
    if ('serviceWorker' in navigator) {
      try {
        const reg = await navigator.serviceWorker.ready;
        await reg.showNotification(title, { ...options, data: dataCarrier });
        return;
      } catch (err) {
        // Fall through to the constructor path. The SW may not be
        // controlling this page yet on the very first visit, or the
        // platform may have rejected showNotification for a reason
        // unrelated to the Notification constructor itself.
        debug('notify', 'SW showNotification failed, falling back to constructor', err);
      }
    }
    const notification = new Notification(title, { ...options, data: dataCarrier });
    notification.onclick = () => {
      window.focus();
      opts.onActivate(app);
      notification.close();
    };
  }

  function findAppForOrigin(origin: string, source: MessageEventSource | null): App | undefined {
    const apps = opts.getApps();
    // The unforgeable identity is `source === frame.contentWindow`: a
    // postMessage can spoof origin (same-origin proxied apps all share
    // window.location.origin) but it cannot pretend to be a different
    // window handle. Iterate iframes and match by source only.
    //
    // For non-proxied (cross-origin) apps we additionally check that
    // the reported `origin` matches the app's configured URL, as a
    // second line of defence if the data-app attribute is ever reused.
    const frames = document.querySelectorAll<HTMLIFrameElement>('iframe[data-app]');
    for (const frame of frames) {
      if (!source || frame.contentWindow !== source) continue;
      const name = frame.dataset.app;
      const app = apps.find(a => a.name === name);
      if (!app) return undefined;
      if (!app.proxyUrl) {
        try {
          if (new URL(app.url).origin !== origin) return undefined;
        } catch {
          return undefined;
        }
      }
      return app;
    }
    // No window-handle match: the sender is not one of our registered
    // iframes. Refusing here closes the cross-app spoofing path
    // (findings.md H22): any same-origin proxied app could otherwise
    // forge a notification on behalf of a different registered app
    // merely by matching its origin.
    return undefined;
  }

  function replyPermission(target: MessageEventSource | null, targetOrigin: string, permission: NotificationPermission): void {
    if (!target) return;
    try {
      // Narrow to Window.postMessage (ports/SW have a different signature).
      (target as Window).postMessage(
        { type: 'muximux:notify-permission', permission },
        targetOrigin,
      );
    } catch {
      // Source may have gone away between dispatch and reply; nothing to do.
    }
  }

  async function handleMessage(event: MessageEvent) {
    if (!isBridgeMessage(event.data)) return;

    // Warnings for the bridge are always-on (not gated by ?debug=true) so
    // app developers can immediately see why their messages aren't producing
    // notifications. These are per-message, not spam.
    const app = findAppForOrigin(event.origin, event.source);
    if (!app) {
      console.warn('[muximux:notify] no matching app for message', {
        origin: event.origin,
        hint: 'The sending iframe must be a registered app in Muximux',
      });
      return;
    }
    if (!app.allow_notifications) {
      console.warn('[muximux:notify] notifications not enabled for app', {
        app: app.name,
        hint: 'Toggle "Allow notifications" in the app\'s settings',
      });
      return;
    }

    if (event.data.type === 'muximux:notify-query-permission') {
      const perm = 'Notification' in window ? Notification.permission : 'denied';
      replyPermission(event.source, event.origin, perm);
      return;
    }

    if (event.data.type === 'muximux:notify-request-permission') {
      const perm = await ensurePermission();
      replyPermission(event.source, event.origin, perm);
      return;
    }

    const now = Date.now();
    const last = lastNotified.get(app.name) ?? 0;
    if (now - last < RATE_LIMIT_MS) {
      debug('notify', 'bridge: rate limited', { app: app.name });
      return;
    }
    // Reserve the rate-limit slot before awaiting. Otherwise a burst of
    // messages all see the same (stale) last-notified time and race past.
    lastNotified.set(app.name, now);

    const perm = await ensurePermission();
    if (perm !== 'granted') {
      console.warn('[muximux:notify] browser notification permission is not granted', {
        permission: perm,
        hint: 'Grant notification permission for this page and retry',
      });
      return;
    }

    const title = truncate(event.data.title, MAX_TITLE) || app.name;
    const body = truncate(event.data.body, MAX_BODY);
    const tag = truncate(event.data.tag, MAX_TAG) || `muximux-${app.name}`;

    const icon = iconUrlFor(app, opts.baseUrl);

    try {
      await showNotificationForApp(app, title, { body, tag, icon });
    } catch (err) {
      console.warn('[muximux:notify] failed to create notification', err);
    }
  }

  // Handle clicks routed back from the service worker. The SW cannot call
  // onActivate directly because it runs in a different realm; it posts
  // a message to the active window client and we re-dispatch it here.
  function handleServiceWorkerMessage(event: MessageEvent): void {
    const data = event.data;
    if (!data || typeof data !== 'object') return;
    if ((data as { type?: unknown }).type !== 'muximux:notification-click') return;
    const appName = (data as { appName?: unknown }).appName;
    if (typeof appName !== 'string') return;
    const app = opts.getApps().find(a => a.name === appName);
    if (app) opts.onActivate(app);
  }

  function iconUrlFor(app: App, baseUrl: string | undefined): string | undefined {
    const base = baseUrl ?? '';
    const icon = app.icon;
    if (!icon) return undefined;
    if (icon.type === 'url' && icon.url) return icon.url;
    if (icon.type === 'custom' && icon.file) return `${base}/icons/custom/${icon.file}`;
    if (icon.type === 'dashboard' && icon.name) {
      const variant = icon.variant ? `-${icon.variant}` : '';
      return `${base}/icons/dashboard/${icon.name}${variant}.svg`;
    }
    return undefined;
  }

  window.addEventListener('message', handleMessage);
  const sw = 'serviceWorker' in navigator ? navigator.serviceWorker : null;
  sw?.addEventListener('message', handleServiceWorkerMessage);
  return () => {
    window.removeEventListener('message', handleMessage);
    sw?.removeEventListener('message', handleServiceWorkerMessage);
  };
}

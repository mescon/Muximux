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

interface BridgeMessage {
  type: 'muximux:notify';
  title?: unknown;
  body?: unknown;
  tag?: unknown;
  url?: unknown;
}

function isBridgeMessage(data: unknown): data is BridgeMessage {
  return !!data && typeof data === 'object'
    && (data as { type?: unknown }).type === 'muximux:notify';
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

  function findAppForOrigin(origin: string, source: MessageEventSource | null): App | undefined {
    const apps = opts.getApps();
    // Iterate iframes so we can match source === iframe.contentWindow
    const frames = document.querySelectorAll<HTMLIFrameElement>('iframe[data-app]');
    for (const frame of frames) {
      if (source && frame.contentWindow === source) {
        const name = frame.dataset.app;
        return apps.find(a => a.name === name);
      }
    }
    // Fallback: origin match (useful if data-app missing for some reason)
    return apps.find(app => {
      if (!app.allow_notifications) return false;
      try {
        const expected = app.proxyUrl
          ? window.location.origin
          : new URL(app.url).origin;
        return expected === origin;
      } catch {
        return false;
      }
    });
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
      const notification = new Notification(title, {
        body,
        tag,
        icon,
      });
      notification.onclick = () => {
        window.focus();
        opts.onActivate(app);
        notification.close();
      };
    } catch (err) {
      console.warn('[muximux:notify] failed to create notification', err);
    }
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
  return () => window.removeEventListener('message', handleMessage);
}

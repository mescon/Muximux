const CACHE_NAME = 'muximux-v2';
const STATIC_ASSETS = [
  './',
  './index.html',
  './manifest.json',
];

// Install: cache shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
  );
  self.skipWaiting();
});

// Activate: clean old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

// Notification click: the bridge renders notifications via
// registration.showNotification() because mobile browsers don't support the
// Notification constructor. Clicks fire here in the SW realm, not in the
// page. Close the notification, focus an existing client (opening one if
// none exists), and post the app name to the client so it can call
// onActivate to switch tabs.
self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const data = event.notification.data || {};
  const appName = typeof data.muximuxApp === 'string' ? data.muximuxApp : null;
  event.waitUntil((async () => {
    const clients = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    let client = clients.find((c) => c.focused) || clients[0];
    if (!client) {
      client = await self.clients.openWindow('./');
    } else {
      try { await client.focus(); } catch { /* best-effort */ }
    }
    if (client && appName) {
      try {
        client.postMessage({ type: 'muximux:notification-click', appName });
      } catch {
        // Client went away between matchAll and postMessage; nothing to do.
      }
    }
  })());
});

// Fetch: network-first for API/WS, cache-first for assets
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // Skip non-GET, cross-origin, API calls, WebSocket upgrades, and proxy routes
  if (event.request.method !== 'GET') return;
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/ws') || url.pathname.startsWith('/proxy/')) return;

  // Cache-first for static assets (hashed filenames)
  if (url.pathname.match(/\/assets\/.*\.[a-f0-9]+\.(js|css|woff2?|png|svg)$/)) {
    event.respondWith(
      caches.match(event.request).then((cached) =>
        cached || fetch(event.request).then((response) => {
          if (response.ok) {
            const clone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
          }
          return response;
        })
      )
    );
    return;
  }

  // Network-first for HTML and other requests
  event.respondWith(
    fetch(event.request).catch(() => caches.match(event.request))
  );
});

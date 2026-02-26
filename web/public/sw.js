const CACHE_NAME = 'muximux-v1';
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

// Fetch: network-first for API/WS, cache-first for assets
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // Skip non-GET, API calls, WebSocket upgrades, and proxy routes
  if (event.request.method !== 'GET') return;
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

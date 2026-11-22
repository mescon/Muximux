// pgw service worker (M-323): maps controlled app clients to /p/<port>/ and rewrites root-absolute requests.
const map = new Map();
let loaded = null;
function load() { return loaded || (loaded = caches.open('pgw').then(c => c.match('/pgw-map')).then(r => r ? r.json() : {}).then(o => { for (const k in o) if (!map.has(k)) map.set(k, o[k]); }).catch(() => {})); }
function save() { return caches.open('pgw').then(c => c.put('/pgw-map', new Response(JSON.stringify(Object.fromEntries(map))))); }
function setp(id, p) { if (id && map.get(id) !== p) { map.set(id, p); save(); } }
const PRE = /^\/p\/(\d+)(\/|$)/;
// Hashed build output only: /_next/static/* is content-hashed per build,
// so keying by URL has no stale risk. Everything else revalidates upstream.
const IMMUTABLE_CACHE = 'pgw-immutable-v1';
const IMMUTABLE_RE = /\/_next\/static\//;
async function portFor(e) {
  await load();
  let p = map.get(e.clientId);
  if (p) return p;
  // Parser-inserted subresources of a navigating document can arrive with
  // an empty clientId before the new client exists (M-323): fall back to
  // the navigation's own port (request Referer is /p/<port>/... then).
  // NOTE: FetchEvent has no .referrer — it lives on e.request (a Request).
  if (!e.clientId && e.request.referrer) {
    try {
      const rm = new URL(e.request.referrer).pathname.match(PRE);
      if (rm) return +rm[1];
    } catch { /* malformed referrer: fall through */ }
  }
  if (e.clientId) {
    const c = await self.clients.get(e.clientId);
    if (c) { const m = new URL(c.url).pathname.match(PRE); if (m) { setp(e.clientId, +m[1]); return +m[1]; } }
  }
  return null;
}
self.addEventListener('fetch', e => {
  const u = new URL(e.request.url);
  if (u.origin !== location.origin) return; // M-341: cross-origin (fonts, CDNs) skips the SW round-trip entirely
  const m = u.pathname.match(PRE);
  if (m) {
    if (e.request.mode === 'navigate') {
      // A claimed top-level deep link may redirect from /p/N/... to the
      // portal shell. Do not label the resulting shell client as an app.
      e.stopImmediatePropagation();
      e.respondWith((async () => {
        try {
          const response = await fetch(e.request);
          const final = new URL(response.url).pathname.match(PRE);
          if (final) setp(e.resultingClientId, +final[1]);
          return response;
        } catch (err) { return new Response('pgw navigate fetch failed: ' + err, { status: 598 }); }
      })());
    }
    // M-341: /_next/static/* is content-hashed per build; serve cache-first
    // so repeat views and back/forward never re-hit the gateway. URL-keyed,
    // a new deploy has a new hash, so no stale risk.
    if (e.request.method === 'GET' && IMMUTABLE_RE.test(u.pathname)) {
      e.respondWith((async () => {
        try {
          const cache = await caches.open(IMMUTABLE_CACHE);
          const hit = await cache.match(e.request.url);
          if (hit) return hit;
          const res = await fetch(e.request);
          if (res.ok) cache.put(e.request.url, res.clone());
          return res;
        } catch (err) { return new Response('pgw immutable fetch failed: ' + err, { status: 598 }); }
      })());
    }
    return;
  }
  e.respondWith((async () => {
    try {
      const port = await portFor(e);
      if (!port) { try { return await fetch(e.request); } catch (err) { return new Response('pgw passthrough fetch failed: ' + err, { status: 598 }); } }
      const nu = '/p/' + port + u.pathname + u.search;
      if (e.request.mode === 'navigate') {
        setp(e.resultingClientId, port);
        return Response.redirect(new URL(nu, location.origin).href, 302);
      }
      // M-341: immutable hashed assets rewritten above go cache-first;
      // stream the body through instead of buffering it whole.
      if (e.request.method === 'GET' && IMMUTABLE_RE.test(u.pathname)) {
        const cache = await caches.open(IMMUTABLE_CACHE);
        const key = new URL(nu, location.origin).href;
        const hit = await cache.match(key);
        if (hit) return hit;
        const res = await fetch(nu);
        if (res.ok) cache.put(key, res.clone());
        return res;
      }
      const r = e.request, init = { method: r.method, headers: r.headers, credentials: r.credentials, redirect: r.redirect };
      if (r.method !== 'GET' && r.method !== 'HEAD') init.body = r.body;
      init.duplex = 'half';
      return await fetch(nu, init);
    } catch (err) { return new Response('sw error: ' + err, { status: 598 }); }
  })());
});

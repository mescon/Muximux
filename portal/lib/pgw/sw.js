// pgw service worker (M-323): maps controlled app clients to /p/<port>/ and rewrites root-absolute requests.
const map = new Map();
let loaded = null;
function load() { return loaded || (loaded = caches.open('pgw').then(c => c.match('/pgw-map')).then(r => r ? r.json() : {}).then(o => { for (const k in o) if (!map.has(k)) map.set(k, o[k]); }).catch(() => {})); }
function save() { return caches.open('pgw').then(c => c.put('/pgw-map', new Response(JSON.stringify(Object.fromEntries(map))))); }
function setp(id, p) { if (id && map.get(id) !== p) { map.set(id, p); save(); } }
const PRE = /^\/p\/(\d+)(\/|$)/;
self.addEventListener('message', e => { if (e.data && e.data.pgw === 'ping' && e.ports[0]) e.ports[0].postMessage('pong'); });
async function portFor(e) {
  await load();
  let p = map.get(e.clientId);
  if (p) return p;
  // Parser-inserted subresources of a navigating document can arrive with
  // an empty clientId before the new client exists (M-323): fall back to
  // the navigation's own port (Referer is /p/<port>/... at that point).
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
  const m = u.pathname.match(PRE);
  if (m) {
    if (e.request.mode === 'navigate') {
      // A claimed top-level deep link may redirect from /p/N/... to the
      // portal shell. Do not label the resulting shell client as an app.
      e.stopImmediatePropagation();
      e.respondWith((async () => {
        const response = await fetch(e.request);
        const final = new URL(response.url).pathname.match(PRE);
        if (final) setp(e.resultingClientId, +final[1]);
        return response;
      })());
    }
    return;
  }
  e.stopImmediatePropagation();
  e.respondWith((async () => {
    try {
      const port = await portFor(e);
      if (!port) { return fetch(e.request); }
      const nu = '/p/' + port + u.pathname + u.search;
      if (e.request.mode === 'navigate') {
        setp(e.resultingClientId, port);
        return Response.redirect(new URL(nu, location.origin).href, 302);
      }
      const r = e.request, init = { method: r.method, headers: r.headers, credentials: r.credentials, redirect: r.redirect };
      if (r.method !== 'GET' && r.method !== 'HEAD') init.body = await r.arrayBuffer();
      return await fetch(nu, init);
    } catch (err) { return new Response('sw error: ' + err, { status: 598 }); }
  })());
});

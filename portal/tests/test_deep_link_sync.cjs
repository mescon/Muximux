const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../ui/ports/ext.js'), 'utf8');
const claim = { port: 3701, app: 'multica', prefix: '/moct/', hosts: ['portal.ddvnguyen.com', 'ddv-server-00.zubron-denebola.ts.net'] };

function boot({ hash = '', hostname = 'portal.ddvnguyen.com', cockpit = false } = {}) {
  const iframe = {
    src: `https://${hostname}/p/3701/moct/`, contentWindow: {},
    attrs: { src: `https://${hostname}/p/3701/moct/`, 'data-app': 'Multica' },
    getAttribute(name) { return this.attrs[name] || null; },
    setAttribute(name, val) { this.attrs[name] = val; },
  };
  const cockpitFrame = {
    src: `https://${hostname}/p/9090/`, contentWindow: {},
    attrs: { src: `https://${hostname}/p/9090/`, 'data-app': 'Cockpit' },
    getAttribute(name) { return this.attrs[name] || null; },
    setAttribute(name, val) { this.attrs[name] = val; },
  };
  const allFrames = cockpit ? [cockpitFrame] : [iframe];
  const rootStyle = new Map();
  const fab = { style: {}, querySelector: () => ({ style: {} }) };
  const handlers = {};
  const storage = new Map();
  const animationFrames = [];
  const mutationCallbacks = [];
  let lastUrl = null;
  const location = { protocol: 'https:', host: hostname, hostname, pathname: '/', search: '', hash };
  const context = vm.createContext({
    AbortController, URL, URLSearchParams, Date, Math, setTimeout, clearTimeout, queueMicrotask,
    location,
    history: { state: null, replaceState(_state, _title, url) { lastUrl = url; }, pushState() {} },
    window: { addEventListener(name, callback) { handlers[name] = callback; } },
    document: {
      documentElement: { style: { setProperty(name, value) { rootStyle.set(name, value); } } },
      hidden: false,
      addEventListener() {},
      querySelector(selector) { return selector === '[role="navigation"].fixed.z-40 > button' ? fab : null; },
      querySelectorAll(selector) { return selector === 'iframe[src]' ? allFrames : []; },
    },
    requestAnimationFrame(callback) { animationFrames.push(callback); },
    MutationObserver: class {
      constructor(callback) { mutationCallbacks.push(callback); }
      observe() {}
    },
    sessionStorage: { getItem() { return null; }, setItem() {} },
    localStorage: { getItem(key) { return storage.get(key) ?? null; }, setItem(key, value) { storage.set(key, value); } },
    fetch: async (url) => String(url).includes('gateway-apps.json')
      ? { ok: true, json: async () => ({ claims: [claim] }) }
      : { ok: false },
    crypto: { randomUUID: () => 'test-session' },
  });
  vm.runInContext(source, context);
  return {
    handlers, iframe, cockpitFrame, fab, rootStyle, storage, animationFrames,
    triggerMutation() { mutationCallbacks[0](); },
    flushAnimationFrame() { animationFrames.shift()?.(); },
    get lastUrl() { return lastUrl; },
  };
}

const wait = () => new Promise((resolve) => setTimeout(resolve, 0));

(async () => {
  const app = boot();
  await wait();
  app.handlers.message({
    source: app.iframe.contentWindow,
    origin: 'https://portal.ddvnguyen.com',
    data: { type: 'portal-gateway-path', port: 3701, path: '/moct/login' },
  });
  assert.equal(app.lastUrl, '/#multica/login');
  app.handlers.message({
    source: app.iframe.contentWindow,
    origin: 'https://evil.example',
    data: { type: 'portal-gateway-path', port: 3701, path: '/moct/forged' },
  });
  assert.equal(app.lastUrl, '/#multica/login', 'untrusted message origins are ignored');

  const restored = boot({ hash: '#multica/login' });
  await wait();
  assert.equal(restored.lastUrl, '/#multica?path=moct%2Flogin');
  const restoredQuery = boot({ hash: '#multica/login?view=compact#details' });
  await wait();
  assert.equal(restoredQuery.lastUrl, '/#multica?path=moct%2Flogin%3Fview%3Dcompact%23details');

  // M-502: the gateway shim posts gateway-prefixed pathnames; the stored
  // hash must strip /p/<port> before the claim prefix check.
  app.handlers.message({
    source: app.iframe.contentWindow,
    origin: 'https://portal.ddvnguyen.com',
    data: { type: 'portal-gateway-path', port: 3701, path: '/p/3701/moct/login' },
  });
  assert.equal(app.lastUrl, '/#multica/login', 'gateway prefix is stripped');

  // M-502: bookmarks saved by the doubled-prefix bug restore cleanly.
  const restoredDoubled = boot({ hash: '#multica/p/3701/moct/login' });
  await wait();
  assert.equal(restoredDoubled.lastUrl, '/#multica?path=moct%2Flogin');

  // M-502: unclaimed apps (Cockpit has no claim) restore plain paths and
  // tolerate legacy doubled bookmarks.
  const rCockpit = boot({ hash: '#cockpit/system' });
  await wait();
  assert.equal(rCockpit.lastUrl, '/#cockpit?path=system');

  const rCockpitDoubled = boot({ hash: '#cockpit/p/9090/podman' });
  await wait();
  assert.equal(rCockpitDoubled.lastUrl, '/#cockpit?path=podman');

  // M-502: gateway-prefixed pathname from an unclaimed iframe strips to
  // the app path instead of doubling (#cockpit/p/9090/podman).

  const app2 = boot({ cockpit: true });
  await wait();
  app2.handlers.message({
    source: app2.cockpitFrame.contentWindow,
    origin: 'https://portal.ddvnguyen.com',
    data: { type: 'portal-gateway-path', port: 9090, path: '/p/9090/podman' },
  });
  assert.equal(app2.lastUrl, '/#cockpit/podman');

  app2.handlers.message({
    source: app2.cockpitFrame.contentWindow,
    origin: 'https://portal.ddvnguyen.com',
    data: { type: 'portal-gateway-path', port: 9090, path: '/system' },
  });
  assert.equal(app2.lastUrl, '/#cockpit/system');

  assert.equal(app.rootStyle.get('--portal-fab-size'), '56px');
  app.storage.set('portal.fab.size', 'large');
  app.handlers.storage({ key: 'portal.fab.size' });
  assert.equal(app.rootStyle.get('--portal-fab-size'), '68px');
  assert.equal(app.fab.style.width, '68px');

  app.storage.set('portal.fab.size', 'small');
  app.triggerMutation();
  app.triggerMutation();
  assert.equal(app.animationFrames.length, 1, 'DOM mutations coalesce into one animation frame');
  app.flushAnimationFrame();
  assert.equal(app.fab.style.width, '44px', 'the coalesced update applies the latest saved size');
  console.log('deep-link updates, restore, origin guard, and live FAB sizing passed');
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

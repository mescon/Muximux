// Exercise chooser transitions with a controlled probe and a real iframe-like state.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const ext = fs.readFileSync(path.join(__dirname, '../ui/ports/ext.js'), 'utf8');
const chooser = ext.slice(ext.indexOf('const TAILNET_HOST ='));
const attrs = { src: '/p/3701/moct/', allow: "camera 'none'" };
const frame = {
  getAttribute: key => attrs[key] || null,
  setAttribute: (key, value) => { attrs[key] = value; },
  set src(value) { attrs.src = value; },
};
let ok = true;
const context = vm.createContext({
  AbortController, setTimeout, clearTimeout, Date,
  fetch: async () => ({ ok }),
  sessionStorage: { setItem() {} },
  document: { hidden: false, addEventListener() {}, querySelectorAll: () => [frame] },
});
vm.runInContext(chooser, context);
(async () => {
  await vm.runInContext('probeTailnet().then(rewritePortIframes)', context);
  assert.equal(attrs.src, 'https://ddv-server-00.zubron-denebola.ts.net/p/3701/moct/');
  assert.equal(attrs.allow, "camera 'none'; local-network-access");
  ok = false;
  await vm.runInContext('probeTailnet(true).then(rewritePortIframes)', context);
  assert.equal(attrs.src, '/p/3701/moct/');
  console.log('chooser switches to tailnet and back to cloudflare');
})().catch(e => { console.error(e); process.exitCode = 1; });

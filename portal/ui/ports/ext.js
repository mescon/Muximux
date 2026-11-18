/* Portal palette extension (M-312, M-323 chooser). Injected into the Muximux
 * page by portal-front. Adds `port:N` / `:N` / `N` rows to the Cmd/Ctrl-K
 * palette and opens the port as a Muximux tab (lease API + hash select).
 * Denied ports show the reason inline. M-323: client-side tailnet chooser —
 * probe the tailnet gateway, rewrite /p/<n>/ iframes to it, badge tabs.
 * No Muximux code is modified. */

const PORT_RE = /^(?:port:|:)?(\d{1,5})$/i;
let registry = null;
let claimsPromise = null;

function loadClaims() {
  if (!claimsPromise) claimsPromise = loadClaimsRaw()
    .then((data) => (data && data.claims) || [])
    .catch(() => { claimsPromise = null; return []; });
  return claimsPromise;
}

function claimedUrl(text, claims) {
  try {
    const url = new URL(text.trim());
    if (url.protocol !== "https:" || url.username || url.password || (url.port && url.port !== "443")) return null;
    const claim = claims.find((c) => Array.isArray(c.hosts) && c.hosts.includes(url.hostname) &&
      typeof c.prefix === "string" && url.pathname.startsWith(c.prefix));
    return claim ? { claim, path: url.pathname + url.search + url.hash } : null;
  } catch { return null; }
}

function openClaim(claim, path) {
  // Muximux resolves paths relative to the app's /p/<port>/ base.
  location.hash = `#${claim.app}?path=${encodeURIComponent(path.replace(/^\//, ""))}`;
}

async function restoreClaimedNavigation() {
  const params = new URLSearchParams(location.search);
  const port = Number(params.get("gateway_port"));
  const path = params.get("gateway_path");
  if (!port || !path) return;
  const claim = (await loadClaims()).find((c) => c.port === port && path.startsWith(c.prefix));
  if (!claim) return;
  const fragment = location.hash;
  history.replaceState(history.state, "", "/");
  openClaim(claim, path + fragment);
}
restoreClaimedNavigation();

function restoreInAppPath() {
  const raw = location.hash.slice(1);
  if (!raw) return;
  const slash = raw.indexOf("/");
  if (slash < 1) return;
  const slug = raw.slice(0, slash);
  loadClaims().then((claims) => {
    const claim = claims.find((item) => item.app === slug &&
      Array.isArray(item.hosts) && item.hosts.includes(location.hostname));
    if (!claim) return;
    const requested = `/${raw.slice(slash + 1)}`;
    const path = `${claim.prefix.replace(/\/$/, "")}${requested}`;
    history.replaceState(history.state, "", `${location.pathname}${location.search}#${slug}?path=${encodeURIComponent(path.replace(/^\//, ""))}`);
  }).catch(() => {});
}
restoreInAppPath();

async function watchInAppPaths() {
  const claims = await loadClaims();
  window.addEventListener("message", (event) => {
    const data = event.data;
    if (!data || data.type !== "portal-gateway-path" || typeof data.path !== "string" || !data.path.startsWith("/")) return;
    const frame = [...document.querySelectorAll("iframe[src]")].find((item) => item.contentWindow === event.source);
    if (!frame) return;
    let frameUrl;
    try { frameUrl = new URL(frame.src, location.href); } catch { return; }
    const claim = claims.find((item) => item.port === data.port &&
      Array.isArray(item.hosts) && item.hosts.includes(frameUrl.hostname) &&
      item.hosts.includes(location.hostname));
    if (!claim || event.origin !== frameUrl.origin) return;
    const prefix = claim.prefix.replace(/\/$/, "");
    if (data.path !== prefix && !data.path.startsWith(`${prefix}/`)) return;
    const appPath = data.path.slice(prefix.length) || "/";
    const route = appPath === "/" && !data.path.includes("?") && !data.path.includes("#")
      ? `#${claim.app}`
      : `#${claim.app}${appPath}`;
    history.replaceState(history.state, "", `${location.pathname}${location.search}${route}`);
  });
}
watchInAppPaths();

const FAB_SIZE_KEY = "portal.fab.size";
const FAB_SIZES = { small: 44, medium: 56, large: 68 };
function applyFabSize(value) {
  const size = FAB_SIZES[value] || FAB_SIZES.medium;
  document.documentElement.style.setProperty("--portal-fab-size", `${size}px`);
  const fab = document.querySelector('[role="navigation"].fixed.z-40 > button');
  if (fab) {
    fab.style.width = `${size}px`;
    fab.style.height = `${size}px`;
    fab.style.padding = "0";
    const icon = fab.querySelector("svg");
    if (icon) { icon.style.width = `${Math.round(size * 0.43)}px`; icon.style.height = `${Math.round(size * 0.43)}px`; }
  }
}
function syncFabSize() {
  let value = "medium";
  try { value = localStorage.getItem(FAB_SIZE_KEY) || "medium"; } catch { /* private mode */ }
  applyFabSize(value);
}
syncFabSize();
window.addEventListener("storage", (event) => { if (event.key === FAB_SIZE_KEY) syncFabSize(); });
let fabSyncScheduled = false;
new MutationObserver(() => {
  if (fabSyncScheduled) return;
  fabSyncScheduled = true;
  requestAnimationFrame(() => {
    fabSyncScheduled = false;
    syncFabSize();
  });
}).observe(document.documentElement, { childList: true, subtree: true });

let sid = null;
try { sid = sessionStorage.getItem("portal.ext.session"); } catch { sid = null; }
if (!sid) {
  try {
    sid = (typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()) + Math.random()).replace(/-/g, "").slice(0, 16);
  } catch { sid = (Date.now().toString(36) + Math.random().toString(36).slice(2, 14)).slice(0, 16); }
  try { sessionStorage.setItem("portal.ext.session", sid); } catch { /* private mode */ }
}

function parsePort(text) {
  const m = PORT_RE.exec((text || "").trim());
  if (!m) return null;
  const n = Number(m[1]);
  return Number.isInteger(n) && n >= 1 && n <= 65535 ? n : null;
}

async function loadRegistry() {
  if (registry) return registry;
  try {
    const res = await fetch("/ports/v1/registry", { cache: "no-store" });
    if (res.ok) registry = await res.json();
  } catch { /* search still works for bare ports */ }
  return registry || { entries: [] };
}

function palette() {
  const dlg = document.querySelector('[role="dialog"]');
  if (!dlg) return null;
  const input = dlg.querySelector(".command-palette-input");
  if (!input) return null;
  const modal = dlg.querySelector(".command-palette-modal") || dlg;
  let list = modal.querySelector("ul, ol");
  return { dlg, modal, input, list };
}
function resultItem(port, title, sub, denied, action) {
  const li = document.createElement("li");
  li.setAttribute("data-portal-ext", "1");
  li.style.cssText = "max-width:100%;overflow:hidden;";
  const btn = document.createElement("button");
  btn.className = "command-palette-item w-full px-4 min-h-[52px] flex items-center space-x-3 text-start py-3";
  btn.setAttribute("type", "button");
  btn.dataset.selected = "false";
  btn.onmouseenter = () => {
    palette()?.dlg.querySelectorAll(".command-palette-item").forEach((b) => { b.dataset.selected = "false"; });
    btn.dataset.selected = "true";
  };
  btn.innerHTML =
    '<div style="flex:none;width:32px;height:32px;border-radius:8px;background:#1f2937;color:#fff;display:flex;align-items:center;justify-content:center;font-weight:700;font-size:14px;">:</div>' +
    '<div style="flex:1;min-width:0;text-align:left;"><div style="font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;"></div>' +
    '<div style="font-size:13px;opacity:.6;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;"></div></div>';
  btn.children[1].children[0].textContent = title;
  btn.children[1].children[1].textContent = sub;
  if (!denied) btn.addEventListener("click", action || (() => openPort(port)));
  else btn.setAttribute("aria-disabled", "true");
  li.append(btn);
  return li;
}
function clearOurs(root) {
  (root || document).querySelectorAll("[data-portal-ext]").forEach((e) => e.remove());
  // unhide the empty-state Muximux owns (we hide it while our rows show)
  (root || document).querySelectorAll("[data-portal-ext-hidden]").forEach((e) => {
    e.hidden = false;
    e.removeAttribute("data-portal-ext-hidden");
  });
}

function hideEmptyState(modal) {
  for (const d of modal.querySelectorAll("div")) {
    if ((d.textContent || "").includes("No results found") && !d.hidden) {
      d.hidden = true;
      d.setAttribute("data-portal-ext-hidden", "1");
    }
  }
}

function manageRow() {
  const li = document.createElement("li");
  li.setAttribute("data-portal-ext", "1");
  const btn = document.createElement("button");
  btn.className = "command-palette-item w-full px-4 min-h-[52px] flex items-center space-x-3 text-start py-3";
  btn.setAttribute("type", "button");
  btn.dataset.selected = "false";
  btn.innerHTML =
    '<div style="flex:none;width:32px;height:32px;border-radius:8px;background:#1f2937;color:#fff;display:flex;align-items:center;justify-content:center;font-weight:700;font-size:14px;">#</div>' +
    '<div style="flex:1;min-width:0;text-align:left;"><div style="font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;"></div>' +
    '<div style="font-size:13px;opacity:.6;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;"></div></div>';
  btn.children[1].children[0].textContent = "Manage local ports";
  btn.children[1].children[1].textContent = "table, live status, saved profiles";
  btn.addEventListener("click", () => {
    document.querySelector('[role="dialog"]')?.remove();
    location.hash = "#/ports";
  });
  li.append(btn);
  return li;
}

let refreshing = false;
let refreshPending = false;
let lastQuery = null;

async function refresh(force) {
  if (refreshing) { refreshPending = true; return; }
  refreshing = true;
  try {
    const p = palette();
    if (!p) return;
    const q = p.input.value;
    if (!force && q === lastQuery) return; // our own DOM edits retrigger us
    lastQuery = q;
    clearOurs(p.dlg);
    const port = parsePort(q);
    const reg = await loadRegistry();
    const deep = claimedUrl(q, await loadClaims());
    const ql = (q || "").trim().toLowerCase();
    const rows = [];
    if (deep) rows.push(resultItem(deep.claim.port, "Open Multica link", "opens this page in the portal", false, () => {
      p.dlg.remove();
      openClaim(deep.claim, deep.path);
    }));
    if (port != null) {
      let reason = null;
      try {
        const res = await fetch(`/ports/v1/resolve?port=${port}`, { cache: "no-store" });
        const body = await res.json();
        if (body.action === "refuse" || res.status === 403) reason = body.reason || "denied";
      } catch { reason = "ports service unreachable"; }
      if (reason) rows.push(resultItem(port, `Port ${port} — denied`, reason, true));
      else rows.push(resultItem(port, `Open local port ${port}`, "opens as an app in the portal", false));
    }
    if (ql && ql !== "port:" && port == null && !deep) {
      for (const e of reg.entries || []) {
        const hay = `${e.title} ${e.id} :${e.port}`.toLowerCase();
        if (hay.includes(ql) && rows.length < 6) {
          rows.push(resultItem(e.port, `Open ${e.title} :${e.port}`, "opens as an app in the portal", false));
        }
      }
    }
    if (!rows.length) return;
    hideEmptyState(p.modal);
    rows.push(manageRow());
    let list = p.list;
    if (!list || !p.modal.contains(list)) {
      // empty state hidden above; own list inside the modal
      list = document.createElement("ul");
      list.className = "pb-2";
      const empty = [...p.modal.querySelectorAll("div")].find((d) =>
        (d.textContent || "").includes("No results found"));
      if (empty && empty.parentElement) empty.parentElement.insertBefore(list, empty);
      else p.modal.append(list);
    }
    for (let i = rows.length - 1; i >= 0; i--) list.prepend(rows[i]);
    // our row becomes the selection so Enter picks it
    const items = p.dlg.querySelectorAll(".command-palette-item");
    items.forEach((b) => { b.dataset.selected = "false"; });
    const first = rows[0].querySelector("button:not([aria-disabled])");
    if (first) first.dataset.selected = "true";
  } finally {
    refreshing = false;
    if (refreshPending) {
      refreshPending = false;
      queueMicrotask(() => refresh());
    }
  }
}
async function openPort(port) {
  const dlg = document.querySelector('[role="dialog"]');
  try {
    const res = await fetch("/ports/v1/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ port, session: sid }),
    });
    const body = await res.json();
    if (body.action === "refuse" || res.status === 403) {
      alert(`Port ${port} denied: ${body.reason || "deny list"}`);
      return;
    }
    if (body.error) { alert(`Could not open :${port}: ${body.error}`); return; }
    if (body && body.lease) {
      if (dlg) dlg.remove();
      try {
        const proto = location.protocol === "https:" ? "wss:" : "ws:";
        const ws = new WebSocket(`${proto}//${location.host}/lease/v1/ws?session=${encodeURIComponent(sid)}`);
        ws.onopen = () => ws.send(JSON.stringify({ adopt: [body.lease.id] }));
      } catch { /* lease still lives out its grace window */ }
      location.hash = `#lease-${port}`;
    }
  } catch {
    alert(`Ports service unreachable — is portal-lease running?`);
  }
}

function hookInput(input) {
  if (input.dataset.portalExtHooked) return;
  input.dataset.portalExtHooked = "1";
  input.addEventListener("input", () => { lastQuery = null; refresh(); });
  input.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") {
      const ours = document.querySelector('[data-portal-ext] button[data-selected="true"]:not([aria-disabled])');
      if (ours) {
        ev.stopImmediatePropagation();
        ev.preventDefault();
        ours.click();
      }
    }
  }, true);
}

new MutationObserver(() => {
  const p = palette();
  if (p) {
    hookInput(p.input);
    refresh();
  }
  rewritePortIframes();
}).observe(document.documentElement, { childList: true, subtree: true });

/* -- tailnet chooser (M-323) ----------------------------------------------
 * <portal-public-host> is the only URL. When this browser can reach the
 * tailnet gateway directly, /p/<n>/ iframes are rewritten to it (faster,
 * no Cloudflare hop); otherwise they stay on the Cloudflare path.
 * Chromium's Local Network Access may show a one-time "local network
 * access" prompt for <portal-public-host>; without the grant the probe
 * fails and the portal silently stays on the Cloudflare path. */

let TAILNET_HOST = "";  // set from /portal-ext/gateway-apps.json (tailnet_host); "" disables the chooser rewrite
const PROBE_TTL_MS = 60 * 1000;
let tailnetOk = null;   // null = unknown, true/false = probed
let tailnetAt = 0;
let tailnetProbe = null;

async function loadTailnetHost() {
  if (TAILNET_HOST) return TAILNET_HOST;
  try {
    const data = await loadClaimsRaw();
    if (data && typeof data.tailnet_host === "string" && data.tailnet_host) {
      TAILNET_HOST = data.tailnet_host.replace(/\/+$/, "");
    }
  } catch { /* chooser stays disabled */ }
  return TAILNET_HOST;
}

function loadClaimsRaw() {
  return fetch("/portal-ext/gateway-apps.json", { cache: "no-store" })
    .then((r) => r.ok ? r.json() : null)
    .catch(() => null);
}

async function probeTailnet(force) {
  await loadTailnetHost();
  if (!TAILNET_HOST) return false;
  const now = Date.now();
  if (!force && tailnetOk !== null && now - tailnetAt < PROBE_TTL_MS) return tailnetOk;
  if (tailnetProbe) return tailnetProbe;
  tailnetProbe = (async () => {
    try {
      const ctl = new AbortController();
      const t = setTimeout(() => ctl.abort(), 2000);
      let res;
      try {
        res = await fetch(`${TAILNET_HOST}/pgw-ping`, { mode: "cors", cache: "no-store", signal: ctl.signal });
      } finally { clearTimeout(t); }
      tailnetOk = res.ok;
    } catch { tailnetOk = false; }
    tailnetAt = Date.now();
    tailnetProbe = null;
    try { sessionStorage.setItem("portal.ext.tailnet", tailnetOk ? "1" : "0"); } catch { /* private mode */ }
    return tailnetOk;
  })();
  return tailnetProbe;
}

function rewritePortIframes() {
  for (const f of document.querySelectorAll("iframe[src]")) {
    const src = f.getAttribute("src");
    if (tailnetOk === true && /^\/p\/\d+(?:\/|$)/.test(src)) {
      const allow = f.getAttribute("allow") || "";
      if (!allow.split(";").some((item) => item.trim().split(/\s+/)[0] === "local-network-access")) {
        f.setAttribute("allow", `${allow}${allow ? "; " : ""}local-network-access`);
      }
      f.src = TAILNET_HOST + src;
    } else if (tailnetOk === false && src.startsWith(`${TAILNET_HOST}/p/`)) {
      f.src = src.slice(TAILNET_HOST.length);
    }
  }
}

// refresh the probe when the tab becomes visible again (laptop roams networks)
document.addEventListener("visibilitychange", () => {
  if (!document.hidden) probeTailnet().then(rewritePortIframes);
});
// early probe so iframes created on load are rewritten on arrival
probeTailnet().then(rewritePortIframes);

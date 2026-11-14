/* Port manager page (M-315). Themed like Muximux; Open does what the
 * palette does: lease the port and select it as the app tab in the portal
 * (#lease-<port>). Profiles and the detail pane stay on this page.
 * No JS timers: one WebSocket per session keeps leases alive server-side. */
"use strict";

const $ = (id) => document.getElementById(id);
const sidKey = "portal.ports.session";
const probeKey = "portal.ports.tailnet";

function newSid() {
  try {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
      return crypto.randomUUID().replace(/-/g, "").slice(0, 16);
    }
  } catch { /* insecure context: fall through */ }
  return (Date.now().toString(36) + Math.random().toString(36).slice(2, 14)).slice(0, 16);
}

let sid = sessionStorage.getItem(sidKey);
if (!sid) {
  sid = newSid();
  sessionStorage.setItem(sidKey, sid);
}

let probeOk = sessionStorage.getItem(probeKey) === "1";
let probeDone = false;
let current = null;
let ws = null;

function showMsg(text, isErr) {
  $("msg").textContent = text || "";
  $("msg").classList.toggle("err", !!isErr);
}

/* -- tailnet probe (M-323 chooser: 2 s abort, 60 s cache, visibility refresh) - */

async function tailnetProbeUrl() {
  try {
    const r = await fetch("/portal-ext/gateway-apps.json", { cache: "no-store" });
    const data = r.ok ? await r.json() : null;
    if (data && typeof data.tailnet_host === "string" && data.tailnet_host) {
      return data.tailnet_host.replace(/\/+$/, "") + "/pgw-ping";
    }
  } catch { /* chooser stays on the Cloudflare path */ }
  return "";
}

async function probeTailnet(force) {
  const now = Date.now();
  if (!force && probeDone && now - (probeTailnet._at || 0) < 60 * 1000) return probeOk;
  probeDone = true;
  const probe = await tailnetProbeUrl();
  if (!probe) return false;
  try {
    const ctl = new AbortController();
    const t = setTimeout(() => ctl.abort(), 2000);
    let res;
    try {
      res = await fetch(probe, { mode: "cors", cache: "no-store", signal: ctl.signal });
    } finally { clearTimeout(t); }
    probeOk = res.ok;
  } catch {
    probeOk = false;
  }
  probeTailnet._at = Date.now();
  try { sessionStorage.setItem(probeKey, probeOk ? "1" : "0"); } catch { /* private mode */ }
  const badge = document.getElementById("gwbadge");
  if (badge) badge.textContent = probeOk ? "tailnet" : "cloudflare";
  return probeOk;
}

document.addEventListener("visibilitychange", () => {
  if (!document.hidden) probeTailnet(true).then(() => loadTable());
});

/* -- manager table -------------------------------------------------------- */

async function loadTable() {
  let entries = [];
  try {
    const res = await fetch("/ports/v1/registry", { cache: "no-store" });
    if (res.ok) entries = (await res.json()).entries || [];
  } catch {
    showMsg("Ports service unreachable — is portal-lease running?", true);
    return;
  }
  const tb = $("rows");
  tb.textContent = "";
  for (const e of entries) {
    const tr = document.createElement("tr");
    tr.innerHTML =
      "<td></td><td class='num'></td><td class='st'></td>" +
      "<td class='via'></td><td class='prof'></td><td class='lease'></td><td class='act'></td>";
    const tds = tr.children;
    tds[0].textContent = e.title || e.id;
    tds[1].textContent = ":" + e.port;
    const dot = document.createElement("span");
    dot.textContent = e.live ? "● live" : "○ down";
    dot.className = e.live ? "st-live" : "st-down";
    tds[2].append(dot);
    const via = document.createElement("span");
    via.className = "pill";
    via.textContent = viaLabel(e);
    tds[3].append(via);
    const prof = document.createElement("span");
    prof.className = "pill";
    prof.textContent = `${e.open || "auto"}${e.spa ? " · SPA" : ""}`;
    tds[4].append(prof);
    const lease = e.lease;
    if (lease) {
      const l = document.createElement("span");
      l.className = "lease-active";
      l.textContent = `leased (${lease.sessions} session${lease.sessions === 1 ? "" : "s"})`;
      tds[5].append(l);
    } else {
      tds[5].textContent = "—";
    }
    const openBtn = document.createElement("button");
    openBtn.textContent = "Open";
    openBtn.className = "primary";
    openBtn.title = "Open as an app tab in the portal (same as the palette)";
    openBtn.onclick = () => openAsApp(e.port);
    const profBtn = document.createElement("button");
    profBtn.textContent = "Profile";
    profBtn.onclick = () => { doOpen(e.port, {}, true); };
    const closeBtn = document.createElement("button");
    closeBtn.textContent = "Close";
    closeBtn.disabled = !lease;
    closeBtn.onclick = async () => {
      await fetch(`/lease/v1/lease/${lease.id}`, { method: "DELETE" });
      loadTable();
      if (current && current.port === e.port) hideViewer();
    };
    tds[6].append(openBtn, profBtn, closeBtn);
    tb.append(tr);
  }
  if (!entries.length) {
    const tr = document.createElement("tr");
    tr.innerHTML = "<td colspan='7'>No registry ports found.</td>";
    tb.append(tr);
  }
}

function viaLabel(e) {
  // effective path for `auto` (M-315 gap 4): the public host renders only
  // on an explicit `public` profile choice, never by default
  if ((e.open || "auto") === "public" && e.public) return "public";
  if ((e.open || "auto") === "tailnet") return e.tailnet_url ? "tailnet" : "tailnet?";
  if (e.tailnet_url && probeOk) return "tailnet";
  if (e.tailnet_url) return "tailnet?";
  return "proxy";
}

/* -- open in pane ---------------------------------------------------------- */

function parseHash() {
  const raw = location.hash.startsWith("#") ? location.hash.slice(1) : location.hash;
  const [head, qs] = raw.split("?", 2);
  const m = /^\/?(?:ports?\/)?(\d{1,5})\/?$/i.exec((head || "").trim());
  if (!m) return null;
  const port = Number(m[1]);
  if (!Number.isInteger(port) || port < 1 || port > 65535) return null;
  const overrides = {};
  for (const [k, v] of new URLSearchParams(qs || "")) {
    if (["open", "path", "scheme", "tailnet_url"].includes(k) && v) overrides[k] = v;
  }
  return { port, overrides };
}

function canonHash(port, overrides) {
  let h = `#/${port}`;
  const q = new URLSearchParams();
  for (const k of ["open", "path", "scheme", "tailnet_url"]) {
    if (overrides && overrides[k]) q.set(k, overrides[k]);
  }
  const s = q.toString();
  return s ? `${h}?${s}` : h;
}

function openPort(port, overrides) {
  if (location.hash !== canonHash(port, overrides)) {
    location.hash = canonHash(port, overrides);
    return; // hashchange handler does the open
  }
  doOpen(port, overrides, false);
}

// M-315 gap 1: Open does exactly what the palette does — create/select the
// port's lease, then select it as the app tab in the portal (#lease-<port>).
// Same request body as the palette (openPort in ext.js), same destination.
async function openAsApp(port) {
  showMsg(`Opening :${port} as an app…`);
  let body = null;
  try {
    const res = await fetch("/ports/v1/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ port, session: sid, probe_ok: probeOk }),
    });
    body = await res.json();
    if ((body && body.action === "refuse") || res.status === 403) {
      refuse(body.reason || body.error || "Port denied.");
      return;
    }
    if (!body || body.error || !body.lease) {
      refuse((body && (body.error + (body.detail ? `: ${body.detail}` : ""))) || `HTTP ${res.status}`);
      return;
    }
  } catch {
    refuse("ports service unreachable.");
    return;
  }
  showMsg("");
  // leave the manager for the portal root: the lease tab lives there
  location.href = `/#lease-${port}`;
}

async function doOpen(port, overrides, showForm) {
  current = { port, overrides: overrides || {} };
  $("viewer").hidden = false;
  $("vtitle").textContent = `:${port}`;
  $("vrefuse").hidden = true;
  $("vframe").hidden = true;
  $("vwarn").hidden = true;
  $("vvia").textContent = "opening…";
  $("vpopout").hidden = true;
  showMsg("");
  await probeTailnet();
  let body = null;
  try {
    const res = await fetch("/ports/v1/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ port, session: sid, probe_ok: probeOk, ...current.overrides }),
    });
    body = await res.json();
    if ((body && body.action === "refuse") || res.status === 403) {
      refuse(body.reason || body.error || "Port denied.");
      return;
    }
    if (!body || body.error) {
      refuse((body && (body.error + (body.detail ? `: ${body.detail}` : ""))) || `HTTP ${res.status}`);
      return;
    }
  } catch {
    refuse("ports service unreachable.");
    return;
  }
  // frame source: public hosts render through the prefix shim (whole origin
  // proxies, X-Frame-Options/CSP stripped); the path proxy loads directly.
  let src = body.url;
  if (body.gateway === "proxy") src = body.url;
  else if (!body.url.startsWith("/")) {
    src = `/ports/v1/shim/${port}${profilePrefixPath(body)}`;
  }
  $("vvia").textContent = `via ${body.gateway}${body.gateway === "proxy" ? " (path proxy)" : ""}`;
  if (body.warning) {
    $("vwarn").textContent = body.warning;
    $("vwarn").hidden = false;
  }
  if (body.popout_url) {
    $("vpopout").hidden = false;
    $("vpopout").onclick = () => window.open(body.popout_url, "_blank", "noopener");
  }
  connectLeaseSocket(body.lease ? body.lease.id : null);
  $("vframe").src = src;
  $("vframe").hidden = false;
  $("prof").hidden = !showForm;
  $("vprofile").setAttribute("aria-expanded", String(!!showForm));
  if (showForm || !$("pport").textContent || $("pport").textContent === String(port)) {
    loadProfile(port);
  }
}

function profilePrefixPath(body) {
  try {
    const u = new URL(body.url, location.origin);
    const p = (u.pathname || "/") + (u.search || "");
    return p.startsWith("/") ? p : "/" + p;
  } catch { return "/"; }
}

function refuse(reason) {
  $("viewer").hidden = false;
  $("vtitle").textContent = current && current.port ? `:${current.port}` : ":—";
  $("vvia").textContent = "";
  $("vrefuse").textContent = `Refused: ${reason}`;
  $("vrefuse").hidden = false;
  $("vframe").hidden = true;
}

function hideViewer() {
  $("viewer").hidden = true;
  $("vframe").removeAttribute("src");
  current = null;
}

$("vclose").onclick = hideViewer;

window.addEventListener("hashchange", () => {
  const parsed = parseHash();
  if (parsed) doOpen(parsed.port, parsed.overrides, false);
});

/* -- lease socket ---------------------------------------------------------- */

function wsUrl() {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/lease/v1/ws?session=${encodeURIComponent(sid)}`;
}

function connectLeaseSocket(leaseId) {
  if (!leaseId) return;
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    try { ws.send(JSON.stringify({ adopt: [leaseId] })); } catch { /* reconnect adopts */ }
    return;
  }
  ws = new WebSocket(wsUrl());
  ws.onopen = () => ws.send(JSON.stringify({ adopt: [leaseId] }));
  ws.onclose = () => { ws = null; };
}

/* -- profile form ------------------------------------------------------------ */

$("vprofile").onclick = () => {
  const open = $("prof").hidden;
  $("prof").hidden = !open;
  $("vprofile").setAttribute("aria-expanded", String(open));
  if (open && current) loadProfile(current.port);
};

async function loadProfile(port) {
  try {
    const res = await fetch(`/ports/v1/profile/${port}`);
    if (!res.ok) return;
    const { profile } = await res.json();
    $("pport").textContent = port;
    $("plabel").value = profile.label || "";
    $("pscheme").value = profile.scheme || "http";
    $("ppath").value = profile.path || "/";
    $("ptailnet").value = profile.tailnet_url || "";
    $("pspa").checked = !!profile.spa;
    $("popen").value = profile.open || "auto";
  } catch { /* form keeps defaults */ }
}

$("psave").onclick = async () => {
  const port = current ? current.port : Number($("pport").textContent);
  if (!port) return;
  const body = {
    label: $("plabel").value.trim(),
    scheme: $("pscheme").value,
    path: $("ppath").value.trim() || "/",
    tailnet_url: $("ptailnet").value.trim(),
    spa: $("pspa").checked,
    open: $("popen").value,
    newtab: false,
    pinned: false,
  };
  try {
    const res = await fetch(`/ports/v1/profile/${port}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    $("pmsg").textContent = res.ok ? "Saved." : `Error: ${data.error}`;
    if (res.ok) loadTable();
  } catch {
    $("pmsg").textContent = "Save failed: service unreachable.";
  }
};

$("pdelete").onclick = async () => {
  const port = current ? current.port : Number($("pport").textContent);
  if (!port) return;
  try {
    await fetch(`/ports/v1/profile/${port}`, { method: "DELETE" });
    $("pmsg").textContent = "Reset to defaults.";
    loadProfile(port);
    loadTable();
  } catch {
    $("pmsg").textContent = "Reset failed.";
  }
};

/* -- boot ------------------------------------------------------------------ */

(async function boot() {
  await probeTailnet();
  await loadTable();
  const parsed = parseHash();
  if (parsed) {
    const canon = canonHash(parsed.port, parsed.overrides);
    if (location.hash !== canon) location.hash = canon;
    else doOpen(parsed.port, parsed.overrides, false);
  }
})();

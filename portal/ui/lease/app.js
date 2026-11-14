/* Port-lease UI (M-249). One WebSocket per portal session (session id in
   sessionStorage, so a reload re-adopts its leases; a NEW tab gets its own
   session). The server pings us at protocol level — no JS timers here, on
   purpose: background tabs throttle timers but still answer WS pings. */
"use strict";

const $ = (id) => document.getElementById(id);
const sidKey = "portal.lease.session";
const leaseKey = "portal.lease.ids";

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

function heldLeases() {
  try { return JSON.parse(sessionStorage.getItem(leaseKey) || "[]"); }
  catch { return []; }
}

function rememberHeld(leases) {
  sessionStorage.setItem(leaseKey, JSON.stringify(leases.map((l) => l.id)));
}

function wsUrl() {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/lease/v1/ws?session=${encodeURIComponent(sid)}`;
}

let ws;

function connect() {
  ws = new WebSocket(wsUrl());
  ws.onopen = () => {
    $("state").textContent = "connected";
    const held = heldLeases();
    if (held.length) ws.send(JSON.stringify({ adopt: held }));
  };
  ws.onclose = () => {
    $("state").textContent = "disconnected — reconnecting…";
    setTimeout(connect, 2000);
  };
  ws.onmessage = (ev) => {
    const data = JSON.parse(ev.data);
    if (data.type === "hello" || data.type === "leases") render(data.leases);
  };
}

function render(leases) {
  rememberHeld(leases);
  const ul = $("leases");
  ul.textContent = "";
  if (!leases.length) {
    const li = document.createElement("li");
    li.textContent = "No active leases.";
    ul.append(li);
    return;
  }
  for (const l of leases) {
    const li = document.createElement("li");
    const a = document.createElement("a");
    a.href = l.url;
    a.textContent = `:${l.port} — open tab`;
    li.append(a);
    const meta = document.createElement("span");
    meta.className = "state";
    meta.textContent = l.close_at ? "reconnecting… (grace)" : `${l.sessions} session(s)`;
    li.append(meta);
    const btn = document.createElement("button");
    btn.textContent = "Close";
    btn.onclick = () => fetch(`/lease/v1/lease/${l.id}`, { method: "DELETE" });
    li.append(btn);
    ul.append(li);
  }
}

$("open").onsubmit = async (ev) => {
  ev.preventDefault();
  $("error").textContent = "";
  const port = Number($("port").value);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    $("error").textContent = "Enter a port from 1 to 65535.";
    return;
  }
  const res = await fetch("/lease/v1/lease", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ port, session: sid }),
  });
  const body = await res.json();
  if (!res.ok) {
    $("error").textContent = body.error === "port denied"
      ? `Port ${port} is denied: ${body.reason || "deny list"}`
      : `Error: ${body.error}`;
    return;
  }
  $("port").value = "";
};

connect();

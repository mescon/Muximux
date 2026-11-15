#!/usr/bin/python3
"""leaselib — lease lifecycle for the open-any-port proxy (M-249).

A lease maps one loopback port to one Muximux proxied tab ("Lease :<port>",
served at /p/<port>/). The lease exists only while at least one
portal session WebSocket holds it: liveness is the SERVER's protocol-level
ping (browser answers even in a background tab), never a browser JS timer.

Lifecycle:
  create(port, session)  -> validated against ports-deny.yaml, one lease per
                            port (a second request returns the existing lease)
  adopt(id, session)     -> reload/tab switch: a returning session re-attaches
                            and cancels the pending grace close
  session_closed(s)      -> schedules close at now + grace for leases no other
                            session holds
  sweep()                -> enforces the grace deadline and the 8h hard cap
  close(id, reason)      -> immediate close (user request, restart purge)

State is memory-only: a restart of the service (or the portal) drops every
lease; the caller purges leftover "Lease :" apps from Muximux on startup.

The blocking calls (Muximux REST, audit file) are hooks supplied by the
caller, so everything here is testable without a network.
"""
import os
import secrets
import threading
import time

LEASE_PREFIX = "Lease :"
LEASE_GROUP = "Leases"
APP_NAME_RE = None  # names are f"{LEASE_PREFIX}<port>"; slug stays lease-<port>


class LeaseError(ValueError):
    """Bad lease request (malformed port, unknown lease id)."""


class PortDenied(LeaseError):
    """Port is in ports-deny.yaml."""


def load_config(path):
    """lease.yaml -> dict with int/str fields; owner-approved defaults if absent."""
    import yaml
    cfg = {
        "grace_seconds": 30,
        "max_lifetime_seconds": 28800,
        "sweep_interval_seconds": 2,
        "heartbeat_seconds": 15,
        "listen": "127.0.0.1:3726",
        "muximux_base": "http://127.0.0.1:45001",
        "lease_group": LEASE_GROUP,
        "audit_log": "logs/lease-audit.jsonl",
    }
    if os.path.exists(path):
        with open(path) as f:
            loaded = yaml.safe_load(f) or {}
        cfg.update(loaded)
    for k in ("grace_seconds", "max_lifetime_seconds", "sweep_interval_seconds", "heartbeat_seconds"):
        cfg[k] = int(cfg[k])
    return cfg


def load_deny(path):
    """ports-deny.yaml -> {port: reason}. Absent file = empty deny list."""
    import yaml
    if not os.path.exists(path):
        return {}
    with open(path) as f:
        entries = (yaml.safe_load(f) or {}).get("deny") or []
    out = {}
    for e in entries:
        out[int(e["port"])] = str(e.get("reason", "denied"))
    return out


def check_port(port, deny):
    """1..65535 int, not bool, not denied. Returns None or raises."""
    if isinstance(port, bool) or not isinstance(port, int):
        raise LeaseError(f"port must be an integer, got {port!r}")
    if not 0 < port < 65536:
        raise LeaseError(f"port out of range 1..65535: {port}")
    if port in deny:
        raise PortDenied(deny[port])
    return None


class Lease:
    __slots__ = ("id", "port", "name", "created_at", "cap_at", "sessions", "close_at")

    def __init__(self, lid, port, now, max_lifetime):
        self.id = lid
        self.port = port
        self.name = f"{LEASE_PREFIX}{port}"
        self.created_at = now()
        self.cap_at = self.created_at + max_lifetime
        self.sessions = set()
        self.close_at = None  # pending grace deadline, or None

    def to_json(self):
        return {
            "id": self.id,
            "port": self.port,
            "name": self.name,
            "url": f"/p/{self.port}/",
            "created_at": self.created_at,
            "cap_at": self.cap_at,
            "close_at": self.close_at,
            "sessions": len(self.sessions),
        }


class LeaseRegistry:
    """Thread-safe in-memory lease table. `mux_add`/`mux_remove`/`audit` are
    caller hooks: mux_add(name, port) -> None, mux_remove(name) -> None,
    audit(event, **fields) -> None. They run while the registry lock is NOT held.
    create() rolls back (mux_remove + table drop) when mux_add fails, so a
    half-open lease never survives."""
    def __init__(self, cfg, deny, now=time.time, mux_add=None, mux_remove=None, audit=None):
        self.cfg = cfg
        self.deny = deny
        self._now = now
        self._mux_add = mux_add if mux_add is not None else (lambda name, port: None)
        self._mux_remove = mux_remove if mux_remove is not None else (lambda name: None)
        self._audit = audit or (lambda event, **f: None)
        self._lock = threading.Lock()
        self._by_id = {}
        self._by_port = {}

    # -- creation / adoption ------------------------------------------------

    def create(self, port, session):
        """Create (or return) the lease for `port` and attach `session`.
        Returns (lease_dict, created)."""
        check_port(port, self.deny)
        with self._lock:
            lease = self._by_port.get(port)
            created = lease is None
            if created:
                lease = Lease(secrets.token_hex(4), port, self._now, self.cfg["max_lifetime_seconds"])
                self._by_id[lease.id] = lease
                self._by_port[port] = lease
            lease.sessions.add(session)
            lease.close_at = None  # a live attach always cancels a pending close
            snapshot = lease.to_json()
        if not created:
            self._audit("adopt", port=port, lease_id=snapshot["id"], session=session)
            return snapshot, created
        try:
            self._mux_add(snapshot["name"], port)
        except Exception:
            with self._lock:
                self._by_id.pop(snapshot["id"], None)
                if self._by_port.get(port, None) is not None and self._by_port[port].id == snapshot["id"]:
                    self._by_port.pop(port, None)
            try:
                self._mux_remove(snapshot["name"])
            except Exception:
                pass
            raise
        self._audit("open", port=port, lease_id=snapshot["id"], session=session)
        return snapshot, created

    def adopt(self, lease_id, session):
        """Re-attach a session to an existing lease (reload / tab switch).
        Cancels the pending grace close. Returns True on success."""
        with self._lock:
            lease = self._by_id.get(lease_id)
            if lease is None:
                return False
            lease.sessions.add(session)
            lease.close_at = None
            snapshot = lease.to_json()
        self._audit("adopt", port=snapshot["port"], lease_id=lease_id, session=session)
        return True

    # -- closing ------------------------------------------------------------

    def session_closed(self, session):
        """The session WS dropped: schedule grace closes for leases it held."""
        grace = self.cfg["grace_seconds"]
        with self._lock:
            now = self._now()
            for lease in list(self._by_id.values()):
                lease.sessions.discard(session)
                if not lease.sessions and lease.close_at is None:
                    lease.close_at = now + grace

    def close(self, lease_id, reason):
        """Immediate close. Returns True if a lease was removed."""
        with self._lock:
            lease = self._by_id.pop(lease_id, None)
            if lease is None:
                return False
            self._by_port.pop(lease.port, None)
            snapshot = lease.to_json()
        self._mux_remove(snapshot["name"])
        self._audit("close", port=snapshot["port"], lease_id=lease_id, reason=reason)
        return True

    def sweep(self):
        """Enforce pending grace deadlines and the 8h hard cap. Returns the
        list of closed (id, port, reason)."""
        out = []
        now = self._now()
        with self._lock:
            due = []
            for lease in list(self._by_id.values()):
                if now >= lease.cap_at:
                    due.append((lease.id, "hard cap 8h expired"))
                elif lease.close_at is not None and now >= lease.close_at:
                    due.append((lease.id, "session closed, grace expired"))
            for lid, reason in due:
                lease = self._by_id.pop(lid, None)
                if lease is None:
                    continue
                self._by_port.pop(lease.port, None)
                out.append((lid, lease.port, reason, lease.to_json()))
        for lid, port, reason, snapshot in out:
            self._mux_remove(snapshot["name"])
            self._audit("close", port=port, lease_id=lid, reason=reason)
        return [(lid, port, reason) for lid, port, reason, _ in out]

    def purge_all(self, reason):
        """Close every lease (service restart). Returns closed names."""
        with self._lock:
            leases = list(self._by_id.values())
            self._by_id.clear()
            self._by_port.clear()
            snapshots = [(l.id, l.port, l.name) for l in leases]
        for lid, port, name in snapshots:
            self._mux_remove(name)
            self._audit("close", port=port, lease_id=lid, reason=reason)
        return [name for _, _, name in snapshots]

    # -- views ----------------------------------------------------------------

    def active(self):
        with self._lock:
            return [l.to_json() for l in self._by_id.values()]

    def get(self, lease_id):
        with self._lock:
            lease = self._by_id.get(lease_id)
            return lease.to_json() if lease else None


class AuditLog:
    """JSON-lines audit log: {ts, event, port, lease_id, session, reason}."""

    def __init__(self, path):
        self.path = path
        os.makedirs(os.path.dirname(path), exist_ok=True)
        self._lock = threading.Lock()

    def __call__(self, event, port=None, lease_id=None, session=None, reason=None):
        import json
        line = json.dumps({
            "ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "event": event,
            "port": port,
            "lease_id": lease_id,
            "session": session,
            "reason": reason,
        })
        with self._lock:
            with open(self.path, "a") as f:
                f.write(line + "\n")

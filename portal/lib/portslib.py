#!/usr/bin/python3 -I
"""portslib — shared helpers for the app-level port registry (M-245).

Three layers, merged into one view by bin/ports and bin/ports-api:
  static     ports.yaml            hand-edited / `bin/ports alloc`
  dynamic    ports.dynamic.json    written by bin/portal-scan every 30s
                                  (rules in ports.yaml, never hand-edited)
  registered ports.registered.json written by bin/ports-api (POST/PUT/DELETE)

`last_seen` is an epoch int in ports.registered.json (reap math) and an
ISO-8601 UTC string in ports.dynamic.json (display only).
"""
import json
import os
import re
import subprocess
import time

ENV_HOME = "PORTS_HOME"
DEFAULT_HOME = "/mnt/workspace/portal"
APPS = {"dsh", "mmx", "moct", "multica", "hydra", "agents", "freebuff"}
LANES = {"x", "z", "none"}
TABS = {True, False, "auto"}
ID_RE = re.compile(r"^([a-z][a-z0-9]*)(?:-([a-z0-9]+))?\.([a-z][a-z0-9]*)(?:\[[^\]]+\])?$")
DYNAMIC_FILE = "ports.dynamic.json"
REGISTERED_FILE = "ports.registered.json"
YAML_FILE = "ports.yaml"


class RegistryError(ValueError):
    """Invalid registry content (schema, parse, conflict)."""


def home():
    return os.environ.get(ENV_HOME) or DEFAULT_HOME


def ts(epoch=None):
    t = time.gmtime(time.time() if epoch is None else epoch)
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", t)


def parse_id(idm):
    """`dsh-x.web` -> ('dsh', 'x', 'web'); `freebuff.engine` -> ('freebuff', None, 'engine')."""
    m = ID_RE.match(idm or "")
    if not m:
        raise RegistryError(f"bad id {idm!r}: want <app>-[<lane>].<role>")
    return m.group(1), m.group(2), m.group(3)


def _check_id_consistency(e):
    app, lane_seg, role = parse_id(e["id"])
    if app != e["app"]:
        raise RegistryError(f"{e['id']}: id app {app!r} != app {e['app']!r}")
    if role != e["role"]:
        raise RegistryError(f"{e['id']}: id role {role!r} != role {e['role']!r}")
    if lane_seg is None:
        if e["lane"] != "none":
            raise RegistryError(f"{e['id']}: id omits lane but lane={e['lane']!r}")
    elif lane_seg != e["lane"]:
        raise RegistryError(f"{e['id']}: id lane {lane_seg!r} != lane {e['lane']!r}")


def _check_static_entry(e, where, ids, ports):
    if not isinstance(e, dict):
        raise RegistryError(f"{where}: entry must be a mapping")
    for k in ("id", "app", "lane", "role", "port", "bind", "unit", "health", "portal"):
        if k not in e:
            raise RegistryError(f"{where} {e.get('id', '?')}: missing field {k!r}")
    _check_id_consistency(e)
    if e["app"] not in APPS:
        raise RegistryError(f"{e['id']}: app {e['app']!r} not in {sorted(APPS)}")
    if e["lane"] not in LANES:
        raise RegistryError(f"{e['id']}: lane {e['lane']!r} not in {sorted(LANES)}")
    port = e["port"]
    if not isinstance(port, int) or isinstance(port, bool) or not 0 < port < 65536:
        raise RegistryError(f"{e['id']}: port must be an int in 1..65535, got {port!r}")
    if port in ports:
        raise RegistryError(f"{e['id']}: port {port} already used by {ports[port]}")
    ports[port] = e["id"]
    if e["id"] in ids:
        raise RegistryError(f"{e['id']}: duplicate id")
    ids.add(e["id"])
    for k in ("bind", "unit", "health"):
        if not isinstance(e[k], str):
            raise RegistryError(f"{e['id']}: {k} must be a string")
    _check_portal(e["id"], e["portal"])


def _check_portal(idm, portal):
    if not isinstance(portal, dict):
        raise RegistryError(f"{idm}: portal must be a mapping")
    if not isinstance(portal.get("title"), str) or not portal["title"]:
        raise RegistryError(f"{idm}: portal.title must be a non-empty string")
    if portal.get("tab") not in TABS:
        raise RegistryError(f"{idm}: portal.tab must be true/false/'auto', got {portal.get('tab')!r}")


def validate(data):
    """Raise RegistryError unless `data` is a valid ports.yaml document."""
    if not isinstance(data, dict):
        raise RegistryError("top level must be a mapping")
    if data.get("version") != 1:
        raise RegistryError(f"version must be 1, got {data.get('version')!r}")
    static = data.get("static")
    if not isinstance(static, list) or not static:
        raise RegistryError("static must be a non-empty list")
    ids, ports = set(), {}
    for e in static:
        _check_static_entry(e, "static", ids, ports)
    dynamic = data.get("dynamic", [])
    if not isinstance(dynamic, list):
        raise RegistryError("dynamic must be a list")
    dyn_ids = set()
    for r in dynamic:
        if not isinstance(r, dict):
            raise RegistryError("dynamic: rule must be a mapping")
        for k in ("id", "app", "lane", "role", "match", "portal"):
            if k not in r:
                raise RegistryError(f"dynamic {r.get('id', '?')}: missing field {k!r}")
        if "port" in r:
            raise RegistryError(f"dynamic {r['id']}: rules carry match, not port")
        _check_id_consistency(r)
        if r["app"] not in APPS or r["lane"] not in LANES:
            raise RegistryError(f"dynamic {r['id']}: bad app/lane")
        if r["id"] in dyn_ids:
            raise RegistryError(f"dynamic {r['id']}: duplicate rule id")
        dyn_ids.add(r["id"])
        try:
            re.compile(r["match"])
        except re.error as exc:
            raise RegistryError(f"dynamic {r['id']}: bad match regex: {exc}")
        _check_portal(r["id"], r["portal"])
        ik = r.get("instance_key")
        if ik is not None:
            if not isinstance(ik, dict) or not isinstance(ik.get("env"), str) or not ik["env"]:
                raise RegistryError(f"dynamic {r['id']}: instance_key needs env: <name>")
            if "extract" in ik:
                try:
                    re.compile(ik["extract"])
                except re.error as exc:
                    raise RegistryError(f"dynamic {r['id']}: bad instance_key extract: {exc}")
    ignore = data.get("ignore", [])
    if not isinstance(ignore, list) or len(set(ignore)) != len(ignore):
        raise RegistryError("ignore must be a list of unique ports")
    for p in ignore:
        if not isinstance(p, int) or isinstance(p, bool) or not 0 < p < 65536:
            raise RegistryError(f"ignore: bad port {p!r}")
    alloc = data.get("alloc", {})
    if not isinstance(alloc, dict) or not isinstance(alloc.get("range"), list) or len(alloc["range"]) != 2:
        raise RegistryError("alloc.range must be [lo, hi]")
    lo, hi = alloc["range"]
    if not (isinstance(lo, int) and isinstance(hi, int) and 0 < lo <= hi < 65536):
        raise RegistryError(f"alloc.range bad: {alloc['range']!r}")
    return data


def _load_yaml(path):
    import yaml  # PyYAML (system python3)
    try:
        with open(path) as f:
            data = yaml.safe_load(f)
    except OSError as exc:
        raise RegistryError(f"{path}: {exc}")
    except Exception as exc:  # yaml errors
        raise RegistryError(f"{path}: YAML parse error: {exc}")
    if data is None:
        raise RegistryError(f"{path}: empty file")
    return data


def load_registry(root=None):
    root = root or home()
    data = _load_yaml(os.path.join(root, YAML_FILE))
    return validate(data)


# ---------------------------------------------------------------- listeners

def ss_listen(ss_text=None):
    """Parse `ss -ltnpH` output -> {port: {'bind': str, 'pids': [int, ...]}}."""
    if ss_text is None:
        ss_text = subprocess.run(["ss", "-ltnpH"], capture_output=True, text=True).stdout
    out = {}
    for line in ss_text.splitlines():
        cols = line.split()
        if len(cols) < 4 or cols[0] != "LISTEN":
            continue
        host, _, port = cols[3].rpartition(":")
        if not port.isdigit():
            continue
        pids = [int(p) for p in re.findall(r"pid=(\d+)", line)]
        rec = out.setdefault(int(port), {"bind": host or "*", "pids": []})
        for p in pids:
            if p not in rec["pids"]:
                rec["pids"].append(p)
    return out


def is_alive(pid):
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True


def _cmdline(pid):
    try:
        with open(f"/proc/{pid}/cmdline", "rb") as f:
            return f.read().replace(b"\0", b" ").decode(errors="replace").strip()
    except OSError:
        return None


def _environ(pid, key):
    try:
        with open(f"/proc/{pid}/environ", "rb") as f:
            raw = f.read()
    except OSError:
        return None
    prefix = key.encode() + b"="
    for item in raw.split(b"\0"):
        if item.startswith(prefix):
            return item[len(prefix):].decode(errors="replace")
    return None


# ---------------------------------------------------------------- dynamic

def scan_dynamic(reg, listen, now_iso=None):
    """Build ports.dynamic.json entries from ports.yaml rules against listeners.

    A port already claimed by a static entry or the ignore list is skipped;
    first matching rule wins; instance key = env extract, else the port.
    """
    taken = {e["port"] for e in reg["static"]} | set(reg.get("ignore", []))
    rules = [(re.compile(r["match"]), r) for r in reg.get("dynamic", [])]
    now_iso = now_iso or ts()
    entries = []
    for port in sorted(listen):
        if port in taken:
            continue
        info = listen[port]
        for pid in info["pids"]:
            cmd = _cmdline(pid)
            if not cmd:
                continue
            for rx, rule in rules:
                if not rx.search(cmd):
                    continue
                key = str(port)
                ik = rule.get("instance_key")
                if ik:
                    val = _environ(pid, ik["env"])
                    if val:
                        m = re.search(ik.get("extract", "(.*)"), val)
                        if m:
                            key = m.group(1) if m.lastindex else m.group(0)
                entries.append({
                    "id": f"{rule['id']}[{key}]",
                    "family": rule["id"],
                    "app": rule["app"],
                    "lane": rule["lane"],
                    "role": rule["role"],
                    "port": port,
                    "bind": info["bind"],
                    "pid": pid,
                    "last_seen": now_iso,
                    "portal": rule["portal"],
                })
                break
            else:
                continue
            break  # port claimed by one instance
    return entries


def _load_json(path):
    try:
        with open(path) as f:
            return json.load(f)
    except FileNotFoundError:
        return None
    except (OSError, ValueError) as exc:
        raise RegistryError(f"{path}: {exc}")


def load_dynamic(root=None):
    root = root or home()
    data = _load_json(os.path.join(root, DYNAMIC_FILE))
    return (data or {}).get("entries", [])


def load_registered(root=None):
    root = root or home()
    data = _load_json(os.path.join(root, REGISTERED_FILE))
    return (data or {}).get("entries", [])


def write_json_atomic(path, obj):
    tmp = f"{path}.tmp"
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o644)
    with os.fdopen(fd, "w") as f:
        json.dump(obj, f, indent=1)
        f.write("\n")
    os.replace(tmp, path)


def reap_registered(entries, listening, now, alive=is_alive):
    """Drop entries whose pid is gone, whose port no longer listens, or whose
    ttl expired (now > last_seen + ttl). Pure: returns the kept list."""
    kept = []
    for e in entries:
        if e.get("pid") and not alive(e["pid"]):
            continue
        if e.get("port") not in listening:
            continue
        if now > e.get("last_seen", 0) + e.get("ttl", 60):
            continue
        kept.append(e)
    return kept


# ---------------------------------------------------------------- merge

def _row(e, source, up, last_seen=None):
    row = {
        "id": e.get("id"),
        "app": e.get("app"),
        "lane": e.get("lane"),
        "role": e.get("role"),
        "port": e.get("port"),
        "bind": e.get("bind", ""),
        "unit": e.get("unit", ""),
        "health": e.get("health", ""),
        "portal": e.get("portal", {"title": e.get("id", ""), "tab": False}),
        "source": source,
        "up": up,
    }
    for k in ("pid", "ttl"):
        if k in e:
            row[k] = e[k]
    if last_seen is not None:
        row["last_seen"] = last_seen
    return row


def merge(reg, dynamic_entries, registered_entries, listening):
    """Merged view: static + live dynamic + live registered; static wins on id."""
    rows = []
    seen = set()
    for e in reg["static"]:
        rows.append(_row(e, "static", e["port"] in listening))
        seen.add(e["id"])
    for e in registered_entries:
        if e["id"] in seen or e["port"] not in listening:
            continue
        rows.append(_row(e, "registered", True, last_seen=ts(e.get("last_seen", 0))))
        seen.add(e["id"])
    for e in dynamic_entries:
        if e["id"] in seen or e["port"] not in listening:
            continue  # expired since the last scan
        rows.append(_row(e, "dynamic", True, last_seen=e.get("last_seen")))
    rows.sort(key=lambda r: (r["app"], r["id"]))
    return rows


# ---------------------------------------------------------------- alloc

def taken_ports(reg, root=None, listen=None):
    root = root or home()
    if listen is None:
        listen = ss_listen()
    ports = set(listen) | set(reg.get("ignore", []))
    ports |= {e["port"] for e in reg["static"]}
    ports |= {e["port"] for e in load_dynamic(root)}
    ports |= {e["port"] for e in load_registered(root)}
    return ports


def choose_port(reg, root=None, listen=None, rng=None):
    lo, hi = (rng or reg["alloc"]["range"])
    taken = taken_ports(reg, root=root, listen=listen)
    for port in range(lo, hi + 1):
        if port not in taken:
            return port
    raise RegistryError(f"no free port in {lo}-{hi}")


def render_static_entry(e):
    """YAML text for one static entry (2-space list indent, appended at EOF)."""
    title = e["portal"]["title"]
    return (
        f"\n  - id: {e['id']}\n"
        f"    app: {e['app']}\n"
        f"    lane: {e['lane']}\n"
        f"    role: {e['role']}\n"
        f"    port: {e['port']}\n"
        f"    bind: {json.dumps(e['bind'])}\n"
        f"    unit: {json.dumps(e['unit'])}\n"
        f"    health: {json.dumps(e['health'])}\n"
        f"    portal: {{title: {json.dumps(title)}, tab: {str(e['portal']['tab']).lower()}}}\n"
    )


def append_static(path, entry):
    """Append one entry to ports.yaml's static list; roll back on any failure.

    ports.yaml must keep `static` as its last top-level key (see file header).
    """
    with open(path, "rb") as f:
        original = f.read()
    block = render_static_entry(entry).encode()
    if not original.endswith(b"\n"):
        block = b"\n" + block
    with open(path, "ab") as f:
        f.write(block)
    try:
        data = validate(_load_yaml(path))
        if not any(e["id"] == entry["id"] for e in data["static"]):
            raise RegistryError(f"{entry['id']}: not present after append")
    except Exception:
        with open(path, "wb") as f:
            f.write(original)
        raise

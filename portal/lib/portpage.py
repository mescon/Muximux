#!/usr/bin/python3 -I
"""portpage — shared helpers for the ports UI (M-312, respec 2026-11-11).

Covers: port query/hash parsing, per-port profiles (port-profiles.yaml), and
open resolution. Resolution order (16:19, 16:38, 16:55 corrections):
  tailnet service hostname (when the client probes reachable) >
  public hostname for known SPAs > portal path proxy (embed).
There is no wildcard host and no per-port `tailscale serve`: `tailnet_url`
is a per-port field (Services address once the Owner finishes tagging), and
the path proxy is a Muximux proxied tab. `spa` marks apps that cannot render
in the path proxy (needs tailnet or public host). No framework imports;
everything is testable without a network.
"""
import json
import os
import re
import urllib.parse
import urllib.request

TS_HOST = "ddv-server-00.zubron-denebola.ts.net"
EMBED_WARNING = "may not render (SPA)"
MODES = ("auto", "tailnet", "public", "embed")

PORT_QUERY_RE = re.compile(r"^(?:port:|:)?(\d{1,5})$", re.IGNORECASE)

def parse_port_query(text):
    """`port:3701` / `:3701` / `3701` -> int, else None. 1..65535 only."""
    if text is None:
        return None
    m = PORT_QUERY_RE.match(str(text).strip())
    if not m:
        return None
    try:
        port = int(m.group(1), 10)
    except ValueError:
        return None
    return port if 0 < port < 65536 else None


def parse_hash(frag):
    """`#/3701`, `#port/3701`, `#ports/3701` (+ `?open=&path=`) -> (port, overrides).

    Returns None when the fragment is not a port link. Overrides are the raw
    query pairs (`open`, `path`, `scheme`); validation happens in resolve().
    """
    if not frag:
        return None
    frag = frag.strip()
    if frag.startswith("#"):
        frag = frag[1:]
    if "?" in frag:
        head, qs = frag.split("?", 1)
        overrides = {k: v[0] for k, v in urllib.parse.parse_qs(qs, keep_blank_values=True).items()}
    else:
        head, overrides = frag, {}
    m = re.match(r"^/?(?:ports?/)?(\d{1,5})/?$", head.strip(), re.IGNORECASE)
    if not m:
        return None
    try:
        port = int(m.group(1), 10)
    except ValueError:
        return None
    if not 0 < port < 65536:
        return None
    return port, overrides


def normalize_hash(port, overrides=None):
    """Canonical bookmark form: `#/3701` (+ `?open=&path=` when set)."""
    check_port_int(port)
    out = f"#/{port}"
    q = {}
    for k in ("open", "path", "scheme"):
        if overrides and overrides.get(k):
            q[k] = overrides[k]
    if q:
        out += "?" + urllib.parse.urlencode(q)
    return out


def check_port_int(port):
    """int 1..65535 (never bool/str/float). Raises ValueError."""
    if isinstance(port, bool) or not isinstance(port, int):
        raise ValueError(f"port must be an integer, got {port!r}")
    if not 0 < port < 65536:
        raise ValueError(f"port out of range 1..65535: {port}")
    return port


# ---------------------------------------------------------------- profiles

PROFILE_DEFAULTS = {
    "label": "",
    "scheme": "http",
    "path": "/",
    "open": "auto",
    "newtab": True,
    "pinned": False,
    "tailnet_url": "",
    "spa": False,
}


def default_profile(port, label=""):
    check_port_int(port)
    p = {"port": port}
    p.update(PROFILE_DEFAULTS)
    if label:
        p["label"] = label
    return p


def validate_profile(data):
    """Raises ValueError unless `data` is a storable profile dict."""
    if not isinstance(data, dict):
        raise ValueError(f"profile must be an object, got {data!r}")
    check_port_int(data.get("port"))
    scheme = data.get("scheme", "http")
    if scheme not in ("http", "https"):
        raise ValueError(f"scheme must be http/https, got {scheme!r}")
    mode = data.get("open", "auto")
    if mode not in MODES:
        raise ValueError(f"open must be one of {MODES}, got {mode!r}")
    for key in ("newtab", "pinned", "spa"):
        if key in data and not isinstance(data[key], bool):
            raise ValueError(f"{key} must be true/false, got {data[key]!r}")
    for key in ("label", "path", "tailnet_url"):
        if key in data and not isinstance(data[key], str):
            raise ValueError(f"{key} must be a string, got {data[key]!r}")
    path = data.get("path", "/")
    if not path.startswith("/"):
        raise ValueError(f"path must start with /, got {path!r}")
    return True


def load_profiles(path):
    """port-profiles.yaml -> {port: profile}. Absent file = {}."""
    if not os.path.exists(path):
        return {}
    import yaml
    with open(path) as f:
        data = yaml.safe_load(f) or {}
    if not isinstance(data, dict):
        raise ValueError(f"{path}: top level must be a mapping")
    out = {}
    for key, prof in (data.get("profiles") or {}).items():
        if not isinstance(prof, dict):
            raise ValueError(f"{path}: profile {key!r} must be a mapping")
        prof = dict(prof)
        try:
            prof["port"] = int(key)
        except (TypeError, ValueError):
            raise ValueError(f"{path}: profile key {key!r} is not a port")
        validate_profile(prof)
        merged = default_profile(prof["port"])
        merged.update({k: v for k, v in prof.items() if v is not None})
        out[prof["port"]] = merged
    return out


def save_profile(path, profile):
    """Upsert one profile (validated) into port-profiles.yaml, atomic."""
    import yaml
    validate_profile(profile)
    current = load_profiles(path)
    merged = default_profile(profile["port"])
    merged.update({k: v for k, v in profile.items() if v is not None})
    current[profile["port"]] = merged
    doc = {"profiles": {p["port"]: {k: v for k, v in p.items() if k != "port"} for p in current.values()}}
    tmp = f"{path}.tmp"
    with open(tmp, "w") as f:
        yaml.safe_dump(doc, f, default_flow_style=False, sort_keys=True)
    os.replace(tmp, path)
    return merged


def delete_profile(path, port):
    """Remove one profile. Returns True if one was stored."""
    import yaml
    check_port_int(port)
    current = load_profiles(path)
    if port not in current:
        return False
    del current[port]
    doc = {"profiles": {p["port"]: {k: v for k, v in p.items() if k != "port"} for p in current.values()}}
    tmp = f"{path}.tmp"
    with open(tmp, "w") as f:
        yaml.safe_dump(doc, f, default_flow_style=False, sort_keys=True)
    os.replace(tmp, path)
    return True


# ---------------------------------------------------------------- resolve

def tailnet_frame_url(port, profile, overrides=None):
    """Tailnet service hostname for `port` (e.g. https://multica.…ts.net/),
    or '' when no address is stored yet. The address is provisioned by the
    Owner's Tailscale Services step; until then the pane uses the proxy."""
    check_port_int(port)
    url = (overrides or {}).get("tailnet_url", profile.get("tailnet_url") or "")
    return url if isinstance(url, str) else ""


def embed_url(port):
    check_port_int(port)
    return f"/p/{port}/"


def with_path(url, profile, overrides=None):
    """Apply the profile path to an in-portal URL."""
    path = (overrides or {}).get("path", profile.get("path") or "/")
    if not path.startswith("/"):
        path = "/" + path
    if path != "/":
        return url.rstrip("/") + path
    return url


def resolve(port, mode="auto", public_url=None, tailnet_reachable=False,
            profile=None, overrides=None):
    """Decide how a port opens INSIDE the portal page.

    Returns {ok, action, url, warning, reason, gateway, mode, spa} where
    action is `frame` (render in the /ports/ iframe pane) or `refuse`.
    Order for `auto` (M-315): tailnet service hostname when the client
    probes reachable and one is stored, else the portal path proxy
    (lease tab). The public hostname renders only for an explicit `public`
    saved-profile choice; it is never the auto default. `popout_url` is
    the public hostname for the "pop out" button ('' if none). The port
    int is validated; hosts come from the profile or the registry.
    """
    check_port_int(port)
    profile = profile or {}
    mode = ((overrides or {}).get("open") or profile.get("open") or mode or "auto")
    if mode not in MODES:
        mode = "auto"
    spa = bool(profile.get("spa"))

    def frame(url, gateway, warning=""):
        return {"ok": True, "action": "frame",
                "url": with_path(url, profile, overrides),
                "warning": warning or (EMBED_WARNING if gateway == "proxy" and spa else ""),
                "reason": "", "gateway": gateway, "mode": mode, "spa": spa,
                "popout_url": public_url or ""}

    def refuse(reason):
        return {"ok": False, "action": "refuse", "url": "",
                "warning": "", "reason": reason,
                "gateway": "none", "mode": mode, "spa": spa,
                "popout_url": public_url or ""}

    tailnet = tailnet_frame_url(port, profile, overrides)
    if mode == "tailnet":
        if tailnet_reachable and tailnet:
            return frame(tailnet, "tailnet")
        return refuse("no tailnet address is stored for "
                      f":{port} yet" if tailnet_reachable else
                      "tailnet unreachable from this browser")
    if mode == "public":
        # explicit saved-profile choice only: render the public host in
        # the pane via the shim; never the auto default (M-315 gap 4)
        if public_url:
            return frame(public_url, "public")
        return refuse(f"no public hostname is registered for :{port}")
    if mode == "auto" and tailnet_reachable and tailnet:
        return frame(tailnet, "tailnet")
    if mode == "embed":
        return frame(embed_url(port), "proxy")
    return frame(embed_url(port), "proxy")


# ---------------------------------------------------------------- lookup

def public_url_for(port, public_hosts, registry_ports, extra_ports=None):
    """public-hosts.json value for `port`, via registry id or extra app name.
    (Cockpit lives in extra-apps.json, not the registry.)"""
    by_key = {}
    for entry in registry_ports or []:
        by_key.setdefault(entry.get("id"), entry)
    for entry in extra_ports or []:
        by_key.setdefault(entry.get("name"), {"port": entry.get("port")})
    for key, url in (public_hosts or {}).items():
        entry = by_key.get(key)
        if entry and entry.get("port") == port:
            return url
    return None

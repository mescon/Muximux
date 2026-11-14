#!/usr/bin/env python3
"""Capture the gateway app matrix at an explicitly supplied local test origin.

Usage: python3 tests/e2e/test_gateway_matrix.py http://127.0.0.1:PORT
This does not authenticate or claim that post-login flows work.
"""
import sys
import importlib.machinery
import importlib.util
import json
import re
from itertools import product
from pathlib import Path
from urllib.parse import urlsplit
from urllib.request import urlopen

if len(sys.argv) == 2 and sys.argv[1] in ("--check-allowlist", "--test-allowlist"):
    mode = sys.argv[1]
    root = Path(__file__).resolve().parents[2]
    loader = importlib.machinery.SourceFileLoader("portal_gw", str(root / "bin/portal-gw"))
    gateway = importlib.util.module_from_spec(importlib.util.spec_from_loader(loader.name, loader))
    sys.argv = [sys.argv[0]]  # portal-gw accepts an optional listen port
    loader.exec_module(gateway)
    if mode == "--test-allowlist":
        import io
        from unittest.mock import mock_open, patch
        registrations = json.dumps({"entries": [
            {"port": 45200, "last_seen": 990, "ttl": 60},
            {"port": 45201, "last_seen": 800, "ttl": 60},
            {"port": 45202, "last_seen": 990, "ttl": 60},
        ]})
        def response(url, **kwargs):
            body = {"entries": [{"port": 3000}]} if url == gateway.REGISTRY_URL else {"leases": [{"port": 45000}]}
            return io.BytesIO(json.dumps(body).encode())
        gateway._cache.update(t=0, allow=set())
        with patch.object(gateway.time, "time", return_value=1000), \
             patch.object(gateway.urllib.request, "urlopen", side_effect=response), \
             patch("builtins.open", mock_open(read_data=registrations)), \
             patch.object(gateway, "_load_deny", return_value={45202, 9090}):
            assert gateway.allowed_ports() == {3000, 3701, 45000, 45200}
        print("Registration expiry, leases and deny list: passed")
        raise SystemExit(0)
    with urlopen("http://127.0.0.1:45001/api/apps", timeout=5) as response:
        apps = json.load(response)
    allowed = gateway.allowed_ports()
    denied = [(app.get("name"), int(match.group(1))) for app in apps
              if (match := re.match(r"^/p/(\d+)(?:/|$)", app.get("url", "")))
              and int(match.group(1)) not in allowed]
    if denied:
        raise SystemExit(f"Muximux gateway apps missing from allowlist: {denied}")
    print("All Muximux /p/<port>/ apps are allowed")
    raise SystemExit(0)

from playwright.sync_api import sync_playwright

BASE = sys.argv[1].rstrip("/")
OUT = Path(__file__).resolve().parent / "shots"
OUT.mkdir(exist_ok=True)

with sync_playwright() as p:
    browser = p.chromium.launch(args=["--no-sandbox"])
    for scheme, port in product(("light", "dark"), (4096, 3000, 3181, 4320, 3180, 9091)):
        context = browser.new_context(viewport={"width": 1500, "height": 1300},
                                      color_scheme=scheme)
        page = context.new_page()
        failures = []
        def response(r):
            if r.status >= 400:
                u = urlsplit(r.url)
                failures.append(f"{r.status} {u.path}")
        page.on("response", response)
        page.on("pageerror", lambda e: failures.append("pageerror " + str(e)[:160]))
        try:
            r = page.goto(f"{BASE}/p/{port}/", wait_until="domcontentloaded", timeout=15000)
            page.wait_for_timeout(4000)
            page.screenshot(path=str(OUT / f"m323-matrix-{port}-{scheme}.png"))
            print(f"{port} {scheme}: initial={r.status if r else 'none'}, title={page.title()!r}, failures={failures[:12]}", flush=True)
        except Exception as exc:
            print(f"{port}: error={type(exc).__name__}: {str(exc)[:160]}", flush=True)
        finally:
            context.close()
    browser.close()

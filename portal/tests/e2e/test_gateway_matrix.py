#!/usr/bin/env python3
"""Capture the gateway app matrix at an explicitly supplied local test origin.

Usage: python3 tests/e2e/test_gateway_matrix.py http://127.0.0.1:PORT
This does not authenticate or claim that post-login flows work.
"""
import sys
from itertools import product
from pathlib import Path
from urllib.parse import urlsplit
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

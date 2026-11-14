#!/usr/bin/env python3
"""Exercise claimed links on an isolated portal front, without credentials.

Usage: python3 tests/e2e/test_claimed_links.py http://127.0.0.1:PORT
The app config is intercepted to enable deep links; no live app is modified.
"""
import json
import sys
from urllib.request import urlopen
from playwright.sync_api import sync_playwright

base = sys.argv[1].rstrip("/")
with urlopen(base + "/api/config", timeout=5) as response:
    config = json.load(response)
for app in config["apps"]:
    if app.get("name") == "Multica":
        app["allow_deep_links"] = True

with sync_playwright() as p:
    browser = p.chromium.launch(args=["--no-sandbox"])
    context = browser.new_context(viewport={"width": 1500, "height": 1300})
    context.route("**/api/config", lambda route: route.fulfill(status=200, json=config))
    context.route("**/pgw-ping", lambda route: route.abort())
    page = context.new_page()
    for entry in ("/moct/runtimes/local%3A123?foo=bar#frag",
                  "/p/3701/moct/runtimes/local%3A123?foo=bar#frag"):
        page.goto(base + entry, wait_until="domcontentloaded")
        frame = page.locator('iframe[data-app="Multica"]')
        frame.wait_for(timeout=15000)
        page.wait_for_function("""() => {
          const f = document.querySelector('iframe[data-app="Multica"]');
          return f && f.src.includes('/p/3701/moct/runtimes/local%3A123?foo=bar#frag');
        }""", timeout=15000)
        print("passed:", entry, flush=True)
    page.goto(base + "/", wait_until="domcontentloaded")
    page.wait_for_timeout(2000)
    page.keyboard.press("ControlOrMeta+k")
    field = page.locator(".command-palette-input")
    field.wait_for(timeout=10000)
    page.wait_for_function("() => document.querySelector('.command-palette-input')?.dataset.portalExtHooked === '1'", timeout=10000)
    field.fill("https://public-host.example/moct/runtimes/local%3A456?foo=bar#frag")
    page.get_by_role("button", name="Open Multica link").click(timeout=10000)
    page.wait_for_function("""() => {
      const f = document.querySelector('iframe[data-app="Multica"]');
      return f && f.src.includes('/p/3701/moct/runtimes/local%3A456?foo=bar#frag');
    }""", timeout=15000)
    print("passed: palette public Multica URL", flush=True)
    context.close()
    browser.close()

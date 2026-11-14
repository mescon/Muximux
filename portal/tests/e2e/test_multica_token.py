#!/usr/bin/env python3
"""Read-only, non-interactive Multica smoke test; not a login acceptance test."""
import json
import sys
from pathlib import Path
from urllib.request import urlopen
from playwright.sync_api import sync_playwright

base = sys.argv[1].rstrip("/")
with (Path.home() / ".multica" / "config.json").open() as f:
    token = json.load(f)["token"]
with urlopen(base + "/api/config", timeout=5) as response:
    config = json.load(response)
for app in config["apps"]:
    if app.get("name") == "Multica":
        app["allow_deep_links"] = True

with sync_playwright() as p:
    browser = p.chromium.launch(args=["--no-sandbox"])
    context = browser.new_context(viewport={"width": 1500, "height": 1300})
    context.add_cookies([{"name": "multica_auth", "value": token, "url": base}])
    context.route("**/api/config", lambda route: route.fulfill(status=200, json=config))
    context.route("**/pgw-ping", lambda route: route.abort())
    page = context.new_page()
    sockets = []
    page.on("websocket", lambda ws: sockets.append(ws.url.split("?", 1)[0]))
    page.goto(base + "/moct/issues", wait_until="domcontentloaded", timeout=15000)
    iframe = page.frame_locator('iframe[data-app="Multica"]')
    iframe.locator("body").wait_for(timeout=15000)
    page.wait_for_timeout(8000)
    iframe.locator('a[href*="/issues/"]').first.wait_for(timeout=20000)
    text = iframe.locator("body").inner_text(timeout=5000)
    frame = next((f for f in page.frames if "/moct/" in f.url), None)
    print("iframe present:", bool(frame), "body characters:", len(text),
          "login form visible:", "sign in" in text.lower() or "log in" in text.lower(),
          "websocket count:", len(sockets),
          "issue links:", iframe.locator('a[href*="/issues/"]').count(), flush=True)
    assert frame and len(text) > 20
    issue = iframe.locator('a[href*="/issues/"]').first
    assert issue.count() > 0
    target = issue.get_attribute("href")
    issue.click()
    page.wait_for_function("""target => [...document.querySelectorAll('iframe[data-app="Multica"]')]
      .some(f => { try { return f.contentWindow.location.pathname === new URL(target, location.origin).pathname; } catch { return false; } })""",
      arg=target, timeout=10000)
    print("opened an issue:", True, flush=True)
    context.close()
    browser.close()

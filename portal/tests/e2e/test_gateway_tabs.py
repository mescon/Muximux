#!/usr/bin/env python3
"""Verify candidate app paths in Muximux without changing its live config."""
import json
import sys
from urllib.request import urlopen
from playwright.sync_api import sync_playwright

base = sys.argv[1].rstrip("/")
with urlopen(base + "/api/config", timeout=5) as response:
    config = json.load(response)
targets = {"opencode": (4096, "Projects"), "Grafana": (3000, "Grafana"),
           "Freebuff Desktop": (4320, "")}
for app in config["apps"]:
    if app.get("name") in targets:
        port, _ = targets[app["name"]]
        app.update({"url": f"/p/{port}/", "proxy": False, "open_mode": "iframe"})

with sync_playwright() as p:
    browser = p.chromium.launch(args=["--no-sandbox"])
    for name, (port, text) in targets.items():
        context = browser.new_context(viewport={"width": 1500, "height": 1300})
        context.route("**/api/config", lambda route: route.fulfill(status=200, json=config))
        context.route("**/pgw-ping", lambda route: route.abort())
        page = context.new_page()
        failures = []
        requests = []
        responses = []
        page.on("request", lambda r: requests.append((r.resource_type, r.url.split("?", 1)[0])))
        page.on("response", lambda r: responses.append((r.status, r.url.split("?", 1)[0], r.headers.get("content-type", ""))) if r.request.resource_type in ("fetch", "script") else None)
        page.on("requestfailed", lambda r: failures.append(("requestfailed", r.url.split("?", 1)[0])))
        page.on("console", lambda m: failures.append(("console", m.text[:200])) if m.type == "error" else None)
        page.on("pageerror", lambda e: failures.append(("pageerror", str(e)[:200])))
        page.on("response", lambda r: failures.append((r.status, r.url.split("?", 1)[0])) if r.status >= 400 else None)
        page.on("response", lambda r: failures.append(("html-for-js", r.url.split("?", 1)[0])) if r.request.resource_type == "script" and "text/html" in r.headers.get("content-type", "") else None)
        page.goto(base + "/#" + name.lower().replace(" ", "-"), wait_until="domcontentloaded")
        frame = page.frame_locator(f'iframe[data-app="{name}"]')
        try:
            if text:
                frame.get_by_text(text, exact=False).first.wait_for(timeout=12000)
            else:
                frame.locator("body").wait_for(timeout=20000)
                page.wait_for_timeout(4000)
                assert len(frame.locator("body").inner_text()) > 20
        except Exception:
            print("frames:", page.locator("iframe").evaluate_all("es => es.map(e => ({app:e.dataset.app,src:e.getAttribute('src')}))"), flush=True)
            print("frame urls:", [f.url for f in page.frames], flush=True)
            for f in page.frames[1:]:
                print("frame body:", f.locator("body").inner_text(timeout=2000)[:200], flush=True)
                print("frame state:", f.evaluate("""() => ({url:location.href, ready:document.readyState,
                  root:document.querySelector('#root')?.children.length,
                  worker:!!navigator.serviceWorker.controller,
                  ok:localStorage.getItem('__pgw_ok'), reload:sessionStorage.getItem('__pgw_r')})"""), flush=True)
            print("failures:", failures[:15], flush=True)
            print("requests:", requests[-30:], flush=True)
            print("responses:", responses[-20:], flush=True)
            raise
        page.wait_for_timeout(2000)
        app_failures = [(code, url) for code, url in failures if f"/p/{port}/" in url]
        assert not app_failures, (name, app_failures)
        print(f"passed: {name} inside Muximux, no gateway response failures", flush=True)
        context.close()
    browser.close()

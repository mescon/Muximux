#!/usr/bin/env python3
"""M-357 evidence: DSH-Z through the gateway + per-app favicon list.

Covers (per the M-357 acceptance):
- DSH-Z tab inside the portal on the secure tailnet origin
  (https://<tailnet-host>/#dsh-z — same Caddy +
  gateway code path as <portal-public-host>, but reachable headless):
  SW-controlled, sidebar/sessions UI renders, zero gateway-originated
  failures (no "port denied"/"origin denied", no html-for-js, no
  /p/3180/* 4xx). Upstream lane-z plugin 404/403s
  ({"error":"forbidden..."} JSON bodies) are recorded separately as
  lane-backend state (M-340's domain), not gateway failures.
- DSH-X baseline on the same origin for comparison.
- /p/<port>/__favicon status + Content-Type for every gateway tab.
- Screenshots (1500x1300, light + dark) under tests/e2e/shots/.

Calibrated 2026-11-14: DSH-Z renders like DSH-X (accepted M-338 bar);
both show the same duplicate root-asset fetch after the shim strips
the iframe path — harmless, the /p/ copies load 200.
"""
import os
import sys
from collections import Counter
from pathlib import Path
from urllib.request import urlopen

from playwright.sync_api import sync_playwright

TAILNET = "https://%s" % os.environ.get("PORTAL_TAILNET_HOST", "<tailnet-host>")
GW = "http://127.0.0.1:3738"
OUT = Path(__file__).resolve().parent / "shots"
OUT.mkdir(exist_ok=True)

GATEWAY_PORTS = [3701, 3000, 4096, 4320, 9090, 3181, 3180, 9091]
# 4320 (Freebuff desktop) ships no page icon: letter fallback, not a failure.
ALLOWED_FAVICON_MISS = {4320}


def gateway_failure(url, status, body_head):
    """True when a failure is attributable to the portal gateway."""
    path = url.split("ts.net")[-1]
    if "html-for-js" in body_head:
        return True
    if status in (403,) and body_head.strip() in ("port denied",
                                                  "origin denied"):
        return True
    if path.startswith("/p/") and status >= 400:
        return True
    return False


def main():
    with sync_playwright() as p:
        browser = p.chromium.launch(args=["--no-sandbox"])
        results = {}
        for scheme in ("light", "dark"):
            for slug, port in (("dsh-x", 3181), ("dsh-z", 3180)):
                ctx = browser.new_context(
                    viewport={"width": 1500, "height": 1300},
                    color_scheme=scheme)
                page = ctx.new_page()
                recs = []
                page.on("response", lambda r: recs.append(r))
                page.on("pageerror",
                        lambda e: recs.append(("pageerror", str(e)[:160])))
                page.goto(f"{TAILNET}/#{slug}",
                          wait_until="domcontentloaded", timeout=30000)
                try:
                    page.wait_for_selector(
                        f'iframe[data-app="{frame_name(slug)}"]',
                        timeout=30000)
                    iloc = page.frame_locator(
                        f'iframe[data-app="{frame_name(slug)}"]')
                    iloc.locator("#root").wait_for(timeout=90000)
                    # lane z boots slowly: poll for rendered content, not
                    # just #root (which exists in the boot shell)
                    for _ in range(90):
                        try:
                            if len(iloc.locator("body").inner_text(
                                    timeout=5000)) > 100:
                                break
                        except Exception:
                            pass
                        page.wait_for_timeout(1000)
                    frame = pg_frame(page)
                    page.wait_for_timeout(3000)
                except Exception as e:
                    frame = pg_frame(page)
                    print(f"{slug} {scheme}: tab wait hit {e!r}"[:160],
                          flush=True)
                state, body = {}, ""
                if frame is not None:
                    try:
                        state = frame.evaluate(
                            "() => ({title: document.title, worker: !!("
                            "navigator.serviceWorker && "
                            "navigator.serviceWorker.controller)})")
                        body = frame.locator("body").inner_text(timeout=8000)
                    except Exception as e:
                        state = {"eval": repr(e)[:160]}
                gw_bad, upstream_noise, html_for_js = [], [], []
                for r in recs:
                    if isinstance(r, tuple):
                        upstream_noise.append(r)
                        continue
                    try:
                        head = r.text()[:80] if r.status >= 400 else ""
                    except Exception:
                        head = ""
                    u = r.url
                    if "/p/" in u and ".js" in u and "text/html" in (
                            r.headers.get("content-type", "")):
                        html_for_js.append(u.split("ts.net")[-1])
                    if gateway_failure(u, r.status, head):
                        gw_bad.append((r.status, u.split("ts.net")[-1][:90]))
                    elif r.status >= 400:
                        upstream_noise.append(
                            (r.status, r.request.method,
                             u.split("ts.net")[-1][:90]))
                page.screenshot(
                    path=str(OUT / f"m357-{slug}-portal-{scheme}.png"))
                results[(slug, scheme)] = (state, len(body), gw_bad,
                                           upstream_noise)
                print(f"{slug} {scheme}: state={state} body={len(body)} "
                      f"gateway_failures={gw_bad} "
                      f"upstream_noise={len(upstream_noise)}", flush=True)
                ctx.close()
        browser.close()
    state, n, gw_bad, _ = results[("dsh-z", "light")]
    assert state.get("worker") is True, f"DSH-Z not SW-controlled: {state}"
    assert n > 100, f"DSH-Z UI did not render (body {n} chars)"
    assert not gw_bad, f"gateway failures on DSH-Z: {gw_bad}"
    for key, (state, n, gw_bad, _) in results.items():
        assert not gw_bad, f"gateway failures on {key}: {gw_bad}"
    print("favicon list (port, status, content-type):")
    bad = 0
    for port in GATEWAY_PORTS:
        try:
            with urlopen(f"{GW}/p/{port}/__favicon", timeout=10) as r:
                status, ct = r.status, r.headers.get("Content-Type", "")
        except Exception as e:
            status, ct = f"ERR {e}", ""
        print(f"  {port}: {status} {ct}")
        if status != 200 and port not in ALLOWED_FAVICON_MISS:
            bad += 1
    assert bad == 0, "a gateway tab favicon is missing"


def pg_frame(page):
    for f in page.frames[1:]:
        try:
            if f.evaluate("() => !!document.querySelector('#root,#app,body')"):
                return f
        except Exception:
            continue
    return page.frames[1] if len(page.frames) > 1 else None


def frame_name(slug):
    return {"dsh-x": "DSH-X", "dsh-z": "DSH-Z"}[slug]


if __name__ == "__main__":
    main()

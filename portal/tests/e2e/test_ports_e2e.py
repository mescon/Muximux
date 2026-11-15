#!/usr/bin/python3
"""M-315 evidence: palette + manager flows against the live portal.

Run: python3 tests/e2e/test_ports_e2e.py [--base http://127.0.0.1:3736]
Screenshots land in tests/e2e/shots/ (1500x1300, light + dark).
Covers: palette row + hidden empty-state, Enter opens Multica in-portal,
:9090 Cockpit login in-portal, themed manager w/ live status, denied port,
plain-http load.
"""
import os
import sys

BASE = sys.argv[sys.argv.index("--base") + 1] if "--base" in sys.argv else "http://127.0.0.1:3736"
OUT = os.path.join(os.path.dirname(os.path.realpath(__file__)), "shots")
os.makedirs(OUT, exist_ok=True)

from playwright.sync_api import sync_playwright

CHROME_ARGS = ["--no-sandbox"]


def shot(pg, name):
    pg.screenshot(path=os.path.join(OUT, name))
    print("shot:", name)


def open_palette(pg):
    pg.keyboard.press("ControlOrMeta+k")
    pg.wait_for_selector('.command-palette-input', timeout=10000)
    pg.wait_for_timeout(800)


def palette_state(pg):
    return pg.evaluate("""() => {
      const modal = document.querySelector('[role=dialog] .command-palette-modal');
      if (!modal) return {open: false};
      const rows = [...modal.querySelectorAll('[data-portal-ext] button')]
        .map(b => b.textContent);
      const noresults = [...modal.querySelectorAll('div')]
        .filter(d => (d.textContent || '').includes('No results found'));
      return {open: true, rows,
              noresults_visible: noresults.some(d => d.offsetParent !== null)};
    }""")


def run_scheme(p, scheme, tag):
    b = p.chromium.launch(args=CHROME_ARGS)
    ctx = b.new_context(viewport={"width": 1500, "height": 1300},
                        color_scheme=scheme)
    pg = ctx.new_page()
    pg.goto(BASE + "/", wait_until="networkidle", timeout=30000)
    pg.wait_for_timeout(3500)

    # (a) palette row for port:3701, empty-state hidden
    open_palette(pg)
    pg.query_selector(".command-palette-input").fill("port:3701")
    pg.wait_for_timeout(2500)
    st = palette_state(pg)
    assert st["open"], "palette did not open"
    assert any("Open local port 3701" in r for r in st["rows"]), st["rows"]
    assert not st["noresults_visible"], "empty-state still visible"
    shot(pg, f"a-palette-3701-{tag}.png")

    # (b) Enter opens Multica inside the portal as #lease-3701
    pg.keyboard.press("Enter")
    pg.wait_for_timeout(6000)
    assert pg.evaluate("() => location.hash") == "#lease-3701", pg.evaluate("() => location.hash")
    shot(pg, f"b-multica-inportal-{tag}.png")
    print("multica lease tab:", pg.evaluate("() => location.hash"),
          "panes:", pg.evaluate("() => document.querySelectorAll('iframe').length"))

    # (c) :9090 opens the Cockpit login inside the portal (fresh tab:
    # focus stays in the lease iframe after (b), so keyboard stays there)
    pg = ctx.new_page()
    pg.goto(BASE + '/', wait_until='networkidle', timeout=30000)
    pg.wait_for_timeout(3500)
    open_palette(pg)
    pg.query_selector(".command-palette-input").fill(":9090")
    pg.wait_for_timeout(2500)
    st = palette_state(pg)
    assert any("9090" in r for r in st["rows"]), st["rows"]
    shot(pg, f"c-palette-9090-{tag}.png")
    pg.keyboard.press("Enter")
    pg.wait_for_timeout(6000)
    assert pg.evaluate("() => location.hash") == "#lease-9090", pg.evaluate("() => location.hash")
    shot(pg, f"c-cockpit-inportal-{tag}.png")

    # (e) denied port shows the reason
    pg = ctx.new_page()
    pg.goto(BASE + '/', wait_until='networkidle', timeout=30000)
    pg.wait_for_timeout(3500)
    open_palette(pg)
    pg.query_selector(".command-palette-input").fill("port:5432")
    pg.wait_for_timeout(2500)
    st = palette_state(pg)
    assert any("denied" in r for r in st["rows"]), st["rows"]
    shot(pg, f"e-denied-5432-{tag}.png")
    pg.keyboard.press("Escape")

    # (d) manager page, themed, live status
    pg.goto(BASE + "/ports/", wait_until="networkidle", timeout=30000)
    pg.wait_for_timeout(4000)
    theme = pg.evaluate("() => document.documentElement.getAttribute('data-theme')")
    body = pg.evaluate("() => document.body.textContent.slice(0, 400)")
    assert "loading" not in body or "live" in body or "down" in body, body[:200]
    shot(pg, f"d-manager-{tag}.png")
    print("manager theme:", theme)

    # manager Open for :3701 selects the app tab
    pg.evaluate("""() => [...document.querySelectorAll('button')]
      .find(b => b.textContent === 'Open' && b.closest('tr').textContent.includes(':3701')).click()""")
    pg.wait_for_timeout(6000)
    print("after manager Open:", pg.evaluate("() => location.href"))
    shot(pg, f"d-manager-open-{tag}.png")
    b.close()


with sync_playwright() as p:
    run_scheme(p, "light", "light")
    run_scheme(p, "dark", "dark")

# (f) plain http (insecure context): page must load past "loading..."
with sync_playwright() as p:
    b = p.chromium.launch(args=CHROME_ARGS)
    pg = b.new_page(viewport={"width": 1500, "height": 1300})
    pg.goto(BASE + "/ports/", wait_until="networkidle", timeout=30000)
    pg.wait_for_timeout(4000)
    body = pg.evaluate("() => document.body.textContent")
    assert "No registry ports" in body or "live" in body or "down" in body, body[:200]
    shot(pg, "f-manager-http.png")
    print("plain-http manager ok, secure context:", pg.evaluate("() => window.isSecureContext"))
    b.close()

print("M-315 e2e evidence complete in", OUT)

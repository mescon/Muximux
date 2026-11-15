"""Regression checks for Cockpit's secure cookie and WebSocket auth path."""
import importlib.machinery
import importlib.util
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
loader = importlib.machinery.SourceFileLoader(
    "portal_gw_cockpit_test", str(ROOT / "bin/portal-gw"))
gateway = importlib.util.module_from_spec(
    importlib.util.spec_from_loader(loader.name, loader))
argv = sys.argv
try:
    sys.argv = ["portal-gw"]
    loader.exec_module(gateway)
finally:
    sys.argv = argv


class Request:
    def __init__(self, host, forwarded=""):
        self.headers = {"Host": host}
        if forwarded:
            self.headers["X-Forwarded-Proto"] = forwarded


class CockpitGatewayCookie(unittest.TestCase):
    def test_tailnet_https_is_recognized_after_tls_termination(self):
        req = Request("tailnet-host.example.ts.net", "http")
        self.assertTrue(gateway.request_is_https(req))
        self.assertEqual(
            gateway.rewrite_cookie("cockpit=value; SameSite=Strict",
                                   gateway.request_is_https(req)),
            "cockpit=value; Path=/; Secure; SameSite=None; Partitioned")

    def test_canonical_portal_and_forwarded_https_are_secure(self):
        self.assertTrue(gateway.request_is_https(Request("portal.example.com", "http")))
        self.assertTrue(gateway.request_is_https(Request("localhost:3738", "https")))

    def test_https_external_host_survives_local_proxy_host_rewrite(self):
        req = Request("127.0.0.1:3738", "http")
        req.headers["X-Forwarded-Host"] = "tailnet-host.example.ts.net"
        self.assertTrue(gateway.request_is_https(req))

    def test_plain_local_http_does_not_receive_cross_site_cookie_attributes(self):
        req = Request("127.0.0.1:3738", "http")
        self.assertFalse(gateway.request_is_https(req))
        cookie = gateway.rewrite_cookie("cockpit=value; SameSite=Strict", gateway.request_is_https(req))
        self.assertIn("SameSite=Strict", cookie)
        self.assertNotIn("Partitioned", cookie)

    def test_cockpit_auth_cookie_is_forwarded_on_websocket_upgrade(self):
        headers = {
            "Cookie": "cockpit=authenticated-session",
            "Origin": "http://localhost:9090",
            "Host": "localhost:9090",
            "Sec-WebSocket-Key": "filtered",
        }
        forwarded = gateway.websocket_upstream_headers(headers)
        self.assertEqual(forwarded["Cookie"], "cockpit=authenticated-session")
        self.assertEqual(forwarded["Origin"], "http://localhost:9090")
        self.assertNotIn("Sec-WebSocket-Key", forwarded)

    def test_cockpit_base_and_absolute_urls_stay_inside_gateway(self):
        html = ('<base href="/">'
                '<script src="cockpit/static/login.js"></script>'
                '<script src="/cockpit/static/other.js"></script>'
                '<form action="/cockpit/login"></form>')
        self.assertEqual(
            gateway.rewrite_cockpit_html_paths(html),
            '<base href="/p/9090/">'
            '<script src="cockpit/static/login.js"></script>'
            '<script src="/p/9090/cockpit/static/other.js"></script>'
            '<form action="/p/9090/cockpit/login"></form>')

    def test_cockpit_html_rewriter_leaves_unrelated_paths_unchanged(self):
        html = '<script src="/api/app.js"></script><a href="/">home</a>'
        self.assertEqual(gateway.rewrite_cockpit_html_paths(html), html)

    def test_cockpit_shell_base_stays_inside_gateway(self):
        # Post-login shell: absolute-path base (nested @localhost iframe root)
        html = ('<base href="/cockpit/@localhost/shell/index.html">'
                '<link href="shell.css" rel="stylesheet" />')
        self.assertEqual(
            gateway.rewrite_cockpit_html_paths(html),
            '<base href="/p/9090/cockpit/@localhost/shell/index.html">'
            '<link href="shell.css" rel="stylesheet" />')

    def test_cockpit_rewriter_takes_throwaway_port(self):
        html = ('<base href="/cockpit/@localhost/shell/index.html">'
                '<script src="/cockpit/static/login.js"></script>')
        self.assertEqual(
            gateway.rewrite_cockpit_html_paths(html, 45991),
            '<base href="/p/45991/cockpit/@localhost/shell/index.html">'
            '<script src="/p/45991/cockpit/static/login.js"></script>')

    def test_cockpit_login_base_still_rewritten(self):
        self.assertEqual(
            gateway.rewrite_cockpit_html_paths('<base href="/">'),
            '<base href="/p/9090/">')

    def test_cockpit_rewriter_ignores_absolute_and_empty_base(self):
        html = '<base href="https://example.com/">'
        self.assertEqual(gateway.rewrite_cockpit_html_paths(html), html)
        html = '<base href="">'
        self.assertEqual(gateway.rewrite_cockpit_html_paths(html), html)


MUX_SAMPLE = """const CACHE_NAME = 'muximux-v2';
// Install: cache shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(['./', './x']))
  );
  self.skipWaiting();
});

// Activate: clean old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(caches.keys().then((keys) =>
    Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
  ));
  self.clients.claim();
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
});

self.addEventListener('fetch', (event) => {
  event.respondWith(fetch(event.request));
});
"""


class CombinedWorkerSanitizer(unittest.TestCase):
    def test_install_and_activate_are_cut(self):
        out = gateway.sw_without_lifecycle(MUX_SAMPLE)
        self.assertNotIn("addEventListener('install'", out)
        self.assertNotIn('addEventListener("install"', out)
        self.assertNotIn("addEventListener('activate'", out)
        self.assertNotIn("addAll(", out)
        self.assertNotIn("caches.delete", out)
        self.assertNotIn("clients.claim", out)

    def test_fetch_and_notification_handlers_survive(self):
        out = gateway.sw_without_lifecycle(MUX_SAMPLE)
        self.assertIn("addEventListener('notificationclick'", out)
        self.assertIn("addEventListener('fetch'", out)
        self.assertIn("event.notification.close();", out)
        self.assertIn("event.respondWith(fetch(event.request));", out)

    def test_braces_in_strings_do_not_break_cut(self):
        mux = ("self.addEventListener('install', (event) => {\n"
               "  event.waitUntil(foo('{ not a brace }'));\n"
               "});\n"
               "self.addEventListener('fetch', (e) => { e.respondWith(f(e)); });\n")
        out = gateway.sw_without_lifecycle(mux)
        self.assertNotIn("install", out)
        self.assertIn("addEventListener('fetch'", out)

    def test_no_markers_is_noop(self):
        mux = "self.addEventListener('fetch', (e) => {});\n"
        self.assertEqual(gateway.sw_without_lifecycle(mux), mux)

    def test_double_quoted_listener_names_cut(self):
        mux = ('self.addEventListener("install", (e) => {\n'
               '  e.waitUntil(caches.open("x").then((c) => c.addAll(["./"])));\n'
               '});\nkeep();\n')
        out = gateway.sw_without_lifecycle(mux)
        self.assertNotIn("install", out)
        self.assertIn("keep();", out)


if __name__ == "__main__":
    unittest.main()

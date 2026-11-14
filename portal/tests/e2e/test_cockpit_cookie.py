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


if __name__ == "__main__":
    unittest.main()

"""Unit tests for the M-357 favicon resolver (lib/pgw/favicon.py)."""
import asyncio
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "lib"))
from pgw.favicon import (  # noqa: E402
    content_type_for,
    favicon_candidates,
    pick_icon_href,
)


class PickHref(unittest.TestCase):
    def test_relative_href(self):
        html = '<html><head><link rel="icon" href="./favicon.svg"></head></html>'
        self.assertEqual(pick_icon_href(html, "http://127.0.0.1:3180/"),
                         "http://127.0.0.1:3180/favicon.svg")

    def test_bare_relative_href(self):
        html = '<link rel="icon" type="image/svg+xml" href="favicon.svg">'
        self.assertEqual(pick_icon_href(html, "http://h:1/some/page"),
                         "http://h:1/some/favicon.svg")

    def test_absolute_and_protocol_relative(self):
        html = '<link rel="icon" href="/icons/f.png">'
        self.assertEqual(pick_icon_href(html, "http://h:1/a/b"),
                         "http://h:1/icons/f.png")
        html = '<link rel="icon" href="//cdn.h/f.png">'
        self.assertEqual(pick_icon_href(html, "http://h:1/"),
                         "http://cdn.h/f.png")

    def test_svg_link(self):
        html = ('<link rel="icon" type="image/svg+xml" href="/favicon-dark.svg" '
                'media="(prefers-color-scheme: dark)">'
                '<link rel="icon" type="image/svg+xml" href="/favicon.svg">')
        self.assertEqual(pick_icon_href(html, "http://h:1/"),
                         "http://h:1/favicon.svg")

    def test_prefers_light_variant(self):
        # dark first in document order, light must still win (M-357)
        html = ('<link rel="icon" type="image/svg+xml" href="./favicon-dark.svg" '
                'media="(prefers-color-scheme: dark)">'
                '<link rel="icon" type="image/svg+xml" href="./favicon.svg" '
                'media="(prefers-color-scheme: light)">')
        self.assertEqual(pick_icon_href(html, "http://h:1:3180/"),
                         "http://h:1:3180/favicon.svg")

    def test_shortcut_icon(self):
        html = '<link rel="shortcut icon" href="/f.ico">'
        self.assertEqual(pick_icon_href(html, "http://h:1/"),
                         "http://h:1/f.ico")

    def test_apple_touch_fallback(self):
        html = '<link rel="apple-touch-icon" href="/apple-touch-icon.png">'
        self.assertEqual(pick_icon_href(html, "http://h:1/"),
                         "http://h:1/apple-touch-icon.png")

    def test_missing_returns_none(self):
        self.assertIsNone(pick_icon_href("<html><head></head></html>",
                                         "http://h:1/"))


class Candidates(unittest.TestCase):
    def test_chain_ends_with_favicon_ico(self):
        c = favicon_candidates("http://h:1/", None)
        self.assertEqual(c, ["http://h:1/favicon.ico"])

    def test_picked_first_then_ico(self):
        c = favicon_candidates("http://h:1:3180/",
                               '<link rel="icon" href="./favicon.svg">')
        self.assertEqual(c, ["http://h:1:3180/favicon.svg",
                             "http://h:1:3180/favicon.ico"])

    def test_no_duplicate_when_link_is_ico(self):
        c = favicon_candidates("http://h:1/", '<link rel="icon" href="/favicon.ico">')
        self.assertEqual(c, ["http://h:1/favicon.ico"])


class ContentType(unittest.TestCase):
    def test_passthrough_image(self):
        self.assertEqual(content_type_for("http://h/f.svg", "image/svg+xml"),
                         "image/svg+xml")
        self.assertEqual(content_type_for("http://h/f.png", "image/png; charset=x"),
                         "image/png")

    def test_sniff_svg_served_as_octet_stream(self):
        self.assertEqual(content_type_for("http://h/f.svg", "application/octet-stream"),
                         "image/svg+xml")

    def test_sniff_ico(self):
        self.assertEqual(content_type_for("http://h/f.ico", "text/plain"),
                         "image/x-icon")


class Redirect(unittest.TestCase):
    """The fetch chain follows redirects (M-357: missing, redirect)."""

    def test_redirect_chain(self):
        seen = []

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def do_GET(self):
                seen.append(self.path)
                if self.path == "/":
                    body = ('<html><head><link rel="icon" href="/jump">'
                            "</head></html>").encode()
                    self.send_response(200)
                    self.send_header("Content-Type", "text/html")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                elif self.path == "/jump":
                    self.send_response(302)
                    self.send_header("Location", "/final.svg")
                    self.end_headers()
                elif self.path == "/final.svg":
                    body = b"<svg></svg>"
                    self.send_response(200)
                    self.send_header("Content-Type", "image/svg+xml")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                else:
                    self.send_response(404)
                    self.end_headers()

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        try:
            import urllib.request
            base = f"http://127.0.0.1:{server.server_port}"
            with urllib.request.urlopen(base + "/", timeout=5) as r:
                html = r.read().decode()
            cands = favicon_candidates(base + "/", html)
            self.assertEqual(cands[0], base + "/jump")
            # urllib follows the 302 like the gateway's aiohttp client does
            with urllib.request.urlopen(cands[0], timeout=5) as r:
                self.assertEqual(r.status, 200)
                self.assertIn("svg", r.headers.get("Content-Type", ""))
            self.assertIn("/final.svg", seen)
        finally:
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    unittest.main()

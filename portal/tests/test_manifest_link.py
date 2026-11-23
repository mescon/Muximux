"""M-514: gateway manifest-link rewrite + unload policy (no upstream Access bypass)."""
import importlib.machinery
import importlib.util
import sys
import unittest
from pathlib import Path

loader = importlib.machinery.SourceFileLoader(
    "portal_gw", str(Path(__file__).resolve().parents[1] / "bin/portal-gw"))
gateway = importlib.util.module_from_spec(importlib.util.spec_from_loader(loader.name, loader))
args = sys.argv
try:
    sys.argv = ["portal-gw"]
    loader.exec_module(gateway)
finally:
    sys.argv = args


class ManifestLinkRewrite(unittest.TestCase):
    """The gateway HTML keeps the app manifest fetch same-origin and
    credentialed so Cloudflare Access lets it through."""

    def test_prefixes_root_href_and_adds_credentials(self):
        out = gateway.rewrite_manifest_link(
            '<link rel="manifest" href="/manifest.webmanifest"/>', 3701)
        self.assertIn('href="/p/3701/manifest.webmanifest"', out)
        self.assertIn('crossorigin="use-credentials"', out)

    def test_leaves_prefixed_href_alone(self):
        src = ('<link rel="manifest" href="/p/3701/manifest.webmanifest"'
               ' crossorigin="use-credentials">')
        self.assertEqual(gateway.rewrite_manifest_link(src, 3701), src)

    def test_keeps_existing_crossorigin_and_prefixes(self):
        out = gateway.rewrite_manifest_link(
            '<link rel="manifest" crossorigin="use-credentials"'
            ' href="/m.webmanifest">', 3701)
        self.assertIn('href="/p/3701/m.webmanifest"', out)
        self.assertEqual(out.count("crossorigin"), 1)

    def test_ignores_protocol_relative_and_pages_without_link(self):
        src = '<link rel="manifest" href="//cdn.example/m.webmanifest">'
        self.assertIn("//cdn.example/m.webmanifest",
                      gateway.rewrite_manifest_link(src, 3701))
        page = "<html><head><title>t</title></head></html>"
        self.assertEqual(gateway.rewrite_manifest_link(page, 3701), page)

    def test_leaves_relative_per_app_manifest_alone(self):
        # M-536: per-app links are relative (./manifest/<slug>.webmanifest)
        # so they resolve under the gateway prefix already; only absolute
        # root hrefs need prefixing (M-514). Credentials still enforced.
        src = ('<link rel="manifest" href="./manifest/plex.webmanifest"'
               ' crossorigin="use-credentials">')
        self.assertEqual(gateway.rewrite_manifest_link(src, 3701), src)
        self.assertEqual(
            gateway.rewrite_manifest_link(
                '<link rel="manifest" href="./manifest/plex.webmanifest">', 3701).count(
                "crossorigin"), 1)


class UnloadPolicy(unittest.TestCase):
    """Gateway HTML opts back into unload handlers for our origin
    (Chrome 155+ default-off) without dropping upstream directives."""

    def test_absent_header_gets_self(self):
        self.assertEqual(gateway.unload_policy({}), "unload=(self)")

    def test_merges_with_upstream_directives(self):
        self.assertEqual(
            gateway.unload_policy({"Permissions-Policy": "camera=*, microphone=*"}),
            "camera=*, microphone=*, unload=(self)")

    def test_respects_upstream_unload(self):
        self.assertEqual(
            gateway.unload_policy({"permissions-policy": "unload=()"}),
            "unload=()")


if __name__ == "__main__":
    unittest.main()

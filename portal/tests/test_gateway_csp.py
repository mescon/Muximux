"""The gateway adds only its injected script to an upstream CSP."""
import base64
import asyncio
import hashlib
import importlib.machinery
import importlib.util
import io
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

loader = importlib.machinery.SourceFileLoader(
    "portal_gw", str(Path(__file__).resolve().parents[1] / "bin/portal-gw"))
gateway = importlib.util.module_from_spec(importlib.util.spec_from_loader(loader.name, loader))
args = sys.argv
try:
    sys.argv = ["portal-gw"]
    loader.exec_module(gateway)
finally:
    sys.argv = args


class GatewayCsp(unittest.TestCase):
    def test_shim_hash_is_added_without_allowing_other_inline_scripts(self):
        csp = "default-src 'self'; script-src 'self' 'sha256-original'; object-src 'none'"
        result = gateway.allow_shim(csp, 4096)
        script = gateway.shim_source(4096)[len("<script>"):-len("</script>")]
        digest = base64.b64encode(hashlib.sha256(script.encode()).digest()).decode()
        self.assertIn("'sha256-" + digest + "'", result)
        self.assertIn("'sha256-original'", result)
        self.assertNotIn("'unsafe-inline'", result)
        self.assertIn("object-src 'none'", result)

    def test_default_policy_preserves_existing_script_sources(self):
        result = gateway.allow_shim("default-src 'self' https://assets.example", 9090)
        self.assertIn("script-src 'self' https://assets.example 'sha256-", result)

    def test_preserves_upstream_unsafe_inline_semantics(self):
        csp = "default-src 'self' 'unsafe-inline'; object-src 'none'"
        self.assertEqual(gateway.allow_shim(csp, 9090), csp)

    def test_private_network_probe_only_authorizes_portal_origin(self):
        class Request:
            def __init__(self, origin):
                self.headers = {"Origin": origin}
        allowed = asyncio.run(gateway.ping(Request("https://portal.example.com")))
        other = asyncio.run(gateway.ping(Request("https://example.org")))
        self.assertEqual(allowed.headers.get("Access-Control-Allow-Origin"),
                         "https://portal.example.com")
        self.assertEqual(allowed.headers.get("Access-Control-Allow-Private-Network"), "true")
        self.assertNotIn("Access-Control-Allow-Origin", other.headers)

    def test_cookie_scope_survives_prefixed_websocket_and_cross_site_iframe(self):
        result = gateway.rewrite_cookie(
            "cockpit=value; DOMAIN=localhost; path=/cockpit; SameSite=Strict; HttpOnly", True)
        self.assertEqual(result, "cockpit=value; Path=/; HttpOnly; Secure; SameSite=None; Partitioned")
        self.assertEqual(gateway.rewrite_cookie("x=value; HttpOnly", False),
                         "x=value; HttpOnly; Path=/")
        self.assertEqual(gateway.rewrite_cookie("x=value; Secure; SameSite=Lax", True).count("Secure"), 1)

    def test_unreadable_deny_list_does_not_allow_fallback_ports_on_start(self):
        gateway._cache.update(t=0, allow=set())
        with patch.object(gateway.urllib.request, "urlopen", side_effect=lambda *a, **k: io.BytesIO(b'{"entries": [], "leases": []}')):
            with patch.object(gateway, "_load_deny", return_value=None):
                self.assertEqual(gateway.allowed_ports(), set())


if __name__ == "__main__":
    unittest.main()

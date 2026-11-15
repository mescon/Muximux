"""M-372: loopback-port mapping (billion-context) in the gateway shim.

The DSH-Z page embeds `globalThis.__BILI__ = {origin:
"http://127.0.0.1:18791"}`; plugin JS fetches that loopback origin
directly. On localhost browser == server so it works; inside the portal
the browser is remote and the URL points at the viewer's own machine.
The shim maps opted-in loopback ports to same-origin `/p/<port>/...`.
"""
import importlib.machinery
import importlib.util
import json
import re
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


class LoopbackMap(unittest.TestCase):
    def test_3180_maps_bili_port(self):
        self.assertEqual(gateway.port_loopback_map(3180), {18791})

    def test_unlisted_port_maps_nothing(self):
        self.assertEqual(gateway.port_loopback_map(3701), set())
        self.assertEqual(gateway.port_loopback_map(9999), set())

    def test_bad_profile_value_is_empty(self):
        real = gateway.port_profile
        gateway.port_profile = lambda port: {"loopback_map": "18791"}
        try:
            self.assertEqual(gateway.port_loopback_map(3180), set())
        finally:
            gateway.port_profile = real

    def test_shim_embeds_targets(self):
        src = gateway.shim_source(3180)
        self.assertIn(json.dumps([18791]), src)
        self.assertIn("window.fetch=", src)
        self.assertIn("XMLHttpRequest", src)
        self.assertIn("window.WebSocket=", src)

    def test_shim_without_targets_maps_nothing(self):
        src = gateway.shim_source(3701)
        self.assertIn("var L=[]", src)

    def test_csp_hash_matches_generated_source(self):
        import base64
        import hashlib
        inner = gateway.shim_source(3180)[len("<script>"):-len("</script>")]
        digest = base64.b64encode(hashlib.sha256(inner.encode()).digest()).decode()
        self.assertIn("'sha256-" + digest + "'",
                      gateway.allow_shim("default-src 'self'", 3180))
    def test_js_rewrite_logic(self):
        """Exercise the shipped JS mapping function in isolation (node)."""
        import subprocess
        inner = gateway.shim_source(3180).removeprefix(
            "<script>").removesuffix("</script>")
        import tempfile, os
        with tempfile.NamedTemporaryFile(
                "w", suffix=".js", delete=False) as tmp:
            tmp.write(inner)
            tmp_path = tmp.name
        try:
            node_check = subprocess.run(
                ["node", "--check", tmp_path],
                capture_output=True, text=True, timeout=15)
            self.assertEqual(node_check.returncode, 0, node_check.stderr[:300])
        finally:
            os.unlink(tmp_path)
        # Test the pure mapping functions (P/L/f/fw): extract the function
        # definitions from the IIFE body and eval them at top level.
        end = inner.find("var _fetch=window.fetch")
        self.assertGreater(end, 0, "shim patch tail not found")
        head = "var P='/p/3180';var L=[18791];\n" + inner[inner.find("function isLoop"):end]
        driver = ("var window={fetch:function(){},WebSocket:function(){},"
                  "XMLHttpRequest:{prototype:{open:function(){}}}};"
                  "var location={host:'x',protocol:'https:',href:'https://x/p/3180/'};var navigator={};\n"
                  + head + "\n"
                  "console.log(JSON.stringify(["
                  "fw('http://127.0.0.1:18791/__bili/upstreams'),"
                  "fw('http://localhost:18791/__bili/?embed=1'),"
                  "fw('http://127.0.0.1:9999/x'),"
                  "f('/api/session/list'),"
                  "fw('wss://127.0.0.1:18791/sock')"
                  "]));")
        out = subprocess.run(["node", "-e", driver], capture_output=True,
                             text=True, timeout=15)
        self.assertEqual(out.returncode, 0, out.stderr[:300])
        got = json.loads(out.stdout)
        self.assertEqual(got[0], "/p/18791/__bili/upstreams")
        self.assertEqual(got[1], "/p/18791/__bili/?embed=1")
        self.assertEqual(got[2], "http://127.0.0.1:9999/x")
        self.assertEqual(got[3], "/p/3180/api/session/list")
        self.assertTrue(got[4].startswith("wss://x/p/18791/sock"), got[4])


if __name__ == "__main__":
    unittest.main()

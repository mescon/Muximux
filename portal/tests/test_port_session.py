"""M-379: profile `source: loopback` selects the loopback-sourced client
session (dsh-claude trustedRequest fences on a loopback peer address)."""
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

APP = {"sess": "default", "sess_lo": "loopback"}


class PortSession(unittest.TestCase):
    def test_3180_uses_loopback_source(self):
        self.assertEqual(gateway.port_session(APP, 3180), "loopback")

    def test_other_ports_use_default(self):
        self.assertEqual(gateway.port_session(APP, 3701), "default")
        self.assertEqual(gateway.port_session(APP, 9999), "default")

    def test_value_is_case_and_space_tolerant(self):
        real = gateway.port_profile
        gateway.port_profile = lambda port: {"source": " Loopback "}
        try:
            self.assertEqual(gateway.port_session(APP, 1), "loopback")
            gateway.port_profile = lambda port: {"source": "other"}
            self.assertEqual(gateway.port_session(APP, 1), "default")
        finally:
            gateway.port_profile = real

    def test_proxy_identity_headers_listed(self):
        for h in ("x-forwarded-for", "cf-connecting-ip", "x-real-ip"):
            self.assertIn(h, gateway.PROXY_IDENTITY)
        self.assertTrue(gateway.port_is_loopback_source(3180))
        self.assertFalse(gateway.port_is_loopback_source(3701))


if __name__ == "__main__":
    unittest.main()

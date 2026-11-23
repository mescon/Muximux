"""M-237: a stale unpartitioned `cockpit` cookie must not shadow the fresh one."""
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


class CockpitCookie(unittest.TestCase):
    def test_keeps_last_duplicate(self):
        self.assertEqual(
            gateway.dedupe_cookie("a=1; cockpit=stale; b=2; cockpit=fresh", "cockpit"),
            "a=1; b=2; cockpit=fresh")

    def test_single_cookie_untouched(self):
        h = "a=1; cockpit=x; b=2"
        self.assertEqual(gateway.dedupe_cookie(h, "cockpit"), h)

    def test_other_names_untouched(self):
        h = "x=1; x=2"
        self.assertEqual(gateway.dedupe_cookie(h, "cockpit"), h)

    def test_expiry_only_for_cockpit_and_https(self):
        sc = ["cockpit=abc; Path=/; HttpOnly", "other=1; Path=/"]
        self.assertEqual(gateway.stale_cookie_expiry(sc, True),
                         ["cockpit=; Max-Age=0; Path=/; Secure; SameSite=None"])
        self.assertEqual(gateway.stale_cookie_expiry(sc, False), [])
        self.assertEqual(gateway.stale_cookie_expiry(["other=1"], True), [])

    def test_gateway_port_cookie_is_partitioned_and_https_only(self):
        # M-540: the front reads pgw-port same-origin; Partitioned keeps
        # the cross-site tailnet embed working.
        self.assertEqual(
            gateway.gateway_port_cookie(9090, True),
            "pgw-port=9090; Path=/; Secure; SameSite=None; Partitioned")
        self.assertIsNone(gateway.gateway_port_cookie(9090, False))


if __name__ == "__main__":
    unittest.main()

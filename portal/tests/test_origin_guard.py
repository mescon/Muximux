import importlib.machinery
import importlib.util
from pathlib import Path
import sys
import unittest

loader = importlib.machinery.SourceFileLoader(
    "portal_gw", str(Path(__file__).resolve().parents[1] / "bin/portal-gw"))
gateway = importlib.util.module_from_spec(importlib.util.spec_from_loader(loader.name, loader))
args = sys.argv
try:
    sys.argv = ["portal-gw"]
    loader.exec_module(gateway)
finally:
    sys.argv = args


class OriginGuardNone(unittest.TestCase):
    def test_forged_origin_on_none_port_is_rejected(self):
        gateway.port_origin_mode = lambda port: "none"
        class Request:
            def __init__(self, origin):
                self.headers = {"Origin": origin}
                self.method = "POST"
        self.assertFalse(gateway.origin_allowed(Request("https://evil.example"), 3701))
        self.assertTrue(gateway.origin_allowed(Request("https://portal.ddvnguyen.com"), 3701))


if __name__ == "__main__":
    unittest.main()

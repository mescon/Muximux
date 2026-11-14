import importlib.machinery
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

loader = importlib.machinery.SourceFileLoader(
    "portal_sync", str(Path(__file__).resolve().parents[1] / "bin/portal-muximux-sync"))
sync = importlib.util.module_from_spec(importlib.util.spec_from_loader(loader.name, loader))
loader.exec_module(sync)


class GatewayApps(unittest.TestCase):
    def test_claimed_app_enables_muximux_deep_links(self):
        ports = b'[{"id":"multica.web","app":"multica","port":3701,"source":"static","portal":{"tab":true,"title":"Multica"}}]'
        with patch.object(sync.subprocess, "check_output", return_value=ports):
            app = next(x for x in sync.desired("proxy") if x["name"] == "Multica")
        self.assertEqual(app["url"], "/p/3701/")
        self.assertTrue(app["allow_deep_links"])
        self.assertFalse(app["proxy"])


if __name__ == "__main__":
    unittest.main()

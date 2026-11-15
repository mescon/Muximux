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

    def test_gateway_app_gets_favicon_icon(self):
        ports = b'[{"id":"dsh-z.web","app":"dsh","port":3180,"source":"static","portal":{"tab":true,"title":"DSH-Z"}}]'
        with patch.object(sync.subprocess, "check_output", return_value=ports), \
             patch.object(sync, "_load_profiles", return_value={}):
            app = next(x for x in sync.desired("proxy") if x["name"] == "DSH-Z")
        self.assertEqual(app["url"], "/p/3180/")
        self.assertEqual(app["icon"],
                         {"type": "url", "url": "/p/3180/__favicon"})

    def test_icon_override_shapes(self):
        cases = [
            ({}, {"type": "url", "url": "/p/3180/__favicon"}),
            ({"icon": ""}, {"type": ""}),
            ({"icon": None}, {"type": ""}),
            ({"icon": "https://h/i.png"}, {"type": "url", "url": "https://h/i.png"}),
            ({"icon": "/i.png"}, {"type": "url", "url": "/i.png"}),
            ({"icon": "grafana"}, {"type": "dashboard", "name": "grafana"}),
            ({"icon": {"type": "lucide", "name": "home"}},
             {"type": "lucide", "name": "home"}),
        ]
        for profile, want in cases:
            with patch.object(sync, "_load_profiles",
                              return_value={3180: profile}):
                self.assertEqual(sync.app_icon(3180), want)

    def test_icon_change_detection_ignores_muximux_extra_keys(self):
        have = {"type": "url", "url": "/p/3180/__favicon",
                "color": "", "background": ""}
        want = {"type": "url", "url": "/p/3180/__favicon"}
        self.assertTrue(sync._same("icon", have, want))
        self.assertFalse(sync._same(
            "icon", have, {"type": "url", "url": "/p/3181/__favicon"}))
        self.assertTrue(sync._same("url", "/p/1/", "/p/1/"))


if __name__ == "__main__":
    unittest.main()

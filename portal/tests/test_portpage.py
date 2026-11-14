#!/usr/bin/python3 -I
"""Tests for portpage (M-312): query/hash parsing, profiles, resolution,
gateway helpers (all against fakes — never a live tailscale or Caddy)."""
import json
import os
import sys
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
sys.path.insert(0, os.path.join(ROOT, "lib"))
import portpage  # noqa: E402

TMP_BASE = os.environ.get("PORTS_TEST_TMP", "/mnt/workspace/tmp")


class TestParseQuery(unittest.TestCase):
    def test_forms(self):
        self.assertEqual(portpage.parse_port_query("port:3701"), 3701)
        self.assertEqual(portpage.parse_port_query("PORT:3701"), 3701)
        self.assertEqual(portpage.parse_port_query(":3701"), 3701)
        self.assertEqual(portpage.parse_port_query("3701"), 3701)
        self.assertEqual(portpage.parse_port_query("  3701  "), 3701)

    def test_junk(self):
        for bad in ("", "abc", "port:", "port:abc", "37a1", "37.1",
                    "3701x", "x3701", "port:3701x", "http://x:3701",
                    "127.0.0.1:3701", "-5", "+5", "3 701", None):
            self.assertIsNone(portpage.parse_port_query(bad), bad)

    def test_range(self):
        self.assertEqual(portpage.parse_port_query("1"), 1)
        self.assertEqual(portpage.parse_port_query("65535"), 65535)
        for bad in ("0", "65536", "99999", "123456"):
            self.assertIsNone(portpage.parse_port_query(bad), bad)


class TestParseHash(unittest.TestCase):
    def test_forms(self):
        self.assertEqual(portpage.parse_hash("#/3701"), (3701, {}))
        self.assertEqual(portpage.parse_hash("#port/3701"), (3701, {}))
        self.assertEqual(portpage.parse_hash("#ports/3701"), (3701, {}))
        self.assertEqual(portpage.parse_hash("#/3701?open=embed&path=/login"),
                         (3701, {"open": "embed", "path": "/login"}))
        self.assertEqual(portpage.parse_hash("/3701"), (3701, {}))

    def test_junk(self):
        for bad in ("", "#", "#/", "#/abc", "#/port/abc", "#/3701/extra",
                    "#/0", "#/99999", "#/login", "#/3701?open="):
            if bad == "#/3701?open=":
                self.assertEqual(portpage.parse_hash(bad), (3701, {"open": ""}))
            else:
                self.assertIsNone(portpage.parse_hash(bad), bad)

    def test_roundtrip(self):
        port, ov = portpage.parse_hash(portpage.normalize_hash(3701))
        self.assertEqual((port, ov), (3701, {}))
        port, ov = portpage.parse_hash(
            portpage.normalize_hash(9090, {"open": "public"}))
        self.assertEqual((port, ov), (9090, {"open": "public"}))
        with self.assertRaises(ValueError):
            portpage.normalize_hash(99999)


class TestResolve(unittest.TestCase):
    def test_tailnet_stored_and_reachable(self):
        r = portpage.resolve(3701, public_url="https://public-host.example",
                             tailnet_reachable=True,
                             profile={"tailnet_url": "https://multica.x.ts.net/",
                                      "open": "auto", "path": "/"})
        self.assertEqual((r["action"], r["gateway"]), ("frame", "tailnet"))
        self.assertEqual(r["url"], "https://multica.x.ts.net/")

    def test_auto_prefers_proxy_without_tailnet(self):
        # M-315 gap 4: auto never renders the public host; only an explicit
        # `public` profile choice does. The public URL stays as popout.
        r = portpage.resolve(3701, public_url="https://public-host.example",
                             profile={"spa": True, "open": "auto", "path": "/"})
        self.assertEqual((r["action"], r["gateway"]), ("frame", "proxy"))
        self.assertEqual(r["url"], "/p/3701/")
        self.assertIn("may not render", r["warning"])
        self.assertEqual(r["popout_url"], "https://public-host.example")
    def test_plain_port_embeds(self):
        r = portpage.resolve(4321, profile=portpage.default_profile(4321))
        self.assertEqual((r["action"], r["gateway"]), ("frame", "proxy"))
        self.assertEqual(r["url"], "/p/4321/")
        self.assertEqual(r["warning"], "")

    def test_spa_warns_in_proxy(self):
        r = portpage.resolve(4321, profile={"open": "embed", "spa": True,
                                            "path": "/"})
        self.assertEqual(r["gateway"], "proxy")
        self.assertIn("may not render", r["warning"])

    def test_pinned_modes(self):
        r = portpage.resolve(3701, profile={"open": "public"},
                             public_url="https://public-host.example",
                             tailnet_reachable=True)
        self.assertEqual(r["gateway"], "public")
        r = portpage.resolve(3701, profile={"open": "tailnet"},
                             public_url="https://public-host.example",
                             tailnet_reachable=False)
        self.assertEqual(r["action"], "refuse")
        r = portpage.resolve(3701, overrides={"open": "embed"},
                             public_url="https://public-host.example",
                             tailnet_reachable=True)
        self.assertEqual(r["gateway"], "proxy")

    def test_bad_port(self):
        with self.assertRaises(ValueError):
            portpage.resolve(99999)
        with self.assertRaises(ValueError):
            portpage.resolve("3701")


class TestProfiles(unittest.TestCase):
    def test_crud(self):
        path = os.path.join(TMP_BASE, "m312-profiles.yaml")
        if os.path.exists(path):
            os.unlink(path)
        try:
            self.assertEqual(portpage.load_profiles(path), {})
            saved = portpage.save_profile(path, {"port": 3701, "open": "public",
                                                 "label": "Multica",
                                                 "tailnet_url": "",
                                                 "spa": True})
            self.assertEqual(saved["open"], "public")
            self.assertTrue(saved["spa"])
            got = portpage.load_profiles(path)
            self.assertEqual(got[3701]["label"], "Multica")
            self.assertEqual(got[3701]["path"], "/")
            self.assertTrue(portpage.delete_profile(path, 3701))
            self.assertFalse(portpage.delete_profile(path, 3701))
        finally:
            if os.path.exists(path):
                os.unlink(path)

    def test_rejects(self):
        for bad in ({"port": 0}, {"port": 99999}, {"port": "3701"},
                    {"port": 3701, "open": "sideways"},
                    {"port": 3701, "open": "wildcard"},
                    {"port": 3701, "scheme": "ftp"},
                    {"port": 3701, "path": "login"},
                    {"port": 3701, "spa": "yes"}):
            with self.assertRaises(ValueError, msg=str(bad)):
                portpage.validate_profile(bad)


class TestLookup(unittest.TestCase):
    def test_public_url_for(self):
        reg = [{"id": "multica.web", "port": 3701}]
        self.assertEqual(portpage.public_url_for(
            3701, {"multica.web": "https://public-host.example"}, reg),
            "https://public-host.example")
        self.assertIsNone(portpage.public_url_for(
            4321, {"multica.web": "https://public-host.example"}, reg))
        self.assertEqual(portpage.public_url_for(
            9090, {"Cockpit": "https://cockpit-host.example"}, reg,
            [{"name": "Cockpit", "port": 9090}]),
            "https://cockpit-host.example")

    def test_pinned_public_without_hostname_refuses(self):
        r = portpage.resolve(4321, profile={"open": "public"})
        self.assertEqual(r["action"], "refuse")


if __name__ == "__main__":
    unittest.main()

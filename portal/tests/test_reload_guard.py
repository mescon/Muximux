"""The one-shot post-SW-registration reload must not wipe login forms (M-532)."""
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


class OneShotReload(unittest.TestCase):
    def setUp(self):
        self.js = gateway.shim_source(9090).removeprefix(
            "<script>").removesuffix("</script>")
    def test_no_unconditional_reload(self):
        # Exactly one reload call remains and it sits inside tryReload,
        # behind the controller/form-busy/inflight guards.
        self.assertEqual(self.js.count("location.reload()"), 1)
        flat = " ".join(self.js.split())
        i = flat.find("function tryReload")
        self.assertGreater(i, 0)
        tail = flat[i:]
        self.assertIn("formBusy()", tail)
        self.assertIn("inflight", tail)
        self.assertIn("location.reload()", tail)
    def test_skips_controlled_documents(self):
        # Already SW-controlled documents never reload.
        self.assertIn("navigator.serviceWorker.controller", self.js)
        self.assertIn("if(sw.controller)return", self.js.replace(" ", ""))

    def test_defers_while_form_busy_or_inflight(self):
        self.assertIn("formBusy", self.js)
        self.assertIn("input[type=password]", self.js)
        self.assertIn("inflight", self.js)
        self.assertIn("setTimeout", self.js)  # retry, not reload-now

    def test_bounds_sw_waits(self):
        # reg.update() and sw.ready no longer wait unbounded (~9 s cause).
        self.assertIn("Promise.race", self.js)
        self.assertIn("5000", self.js)


if __name__ == "__main__":
    unittest.main()

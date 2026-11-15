#!/usr/bin/python3 -I
"""Unit tests for leaselib (M-249): deny list, idempotent create, grace after
session close, re-adopt cancels grace, 8h hard cap, purge on restart."""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.realpath(__file__))), "lib"))
import leaselib  # noqa: E402


class FakeClock:
    def __init__(self, t=1000.0):
        self.t = t

    def __call__(self):
        return self.t

    def advance(self, s):
        self.t += s


def make_registry(deny=None, cfg_over=None, mux_log=None, audit_log=None):
    cfg = leaselib.load_config("/nonexistent")  # defaults
    cfg.update(cfg_over or {})
    mux_log = mux_log if mux_log is not None else []
    audit_log = audit_log if audit_log is not None else []
    reg = leaselib.LeaseRegistry(
        cfg, deny if deny is not None else {5432: "PostgreSQL"},
        now=FakeClock(),  # replaced below
        mux_add=lambda name, port: mux_log.append(("add", name, port)),
        mux_remove=lambda name: mux_log.append(("remove", name)),
        audit=lambda event, **f: audit_log.append((event, f.get("port"), f.get("reason"))),
    )
    return reg, mux_log, audit_log


class TestPortCheck(unittest.TestCase):
    def test_deny(self):
        with self.assertRaises(leaselib.PortDenied):
            leaselib.check_port(5432, {5432: "PostgreSQL"})

    def test_bad_ports(self):
        deny = {}
        for bad in (0, -1, 65536, "4096", None, 4.5, True):
            with self.assertRaises(leaselib.LeaseError):
                leaselib.check_port(bad, deny)

    def test_ok(self):
        leaselib.check_port(4096, {5432: "x"})


class TestLeaseLifecycle(unittest.TestCase):
    def setUp(self):
        self.clock = FakeClock()
        self.mux, self.audit = [], []
        self.cfg = leaselib.load_config("/nonexistent")
        self.reg = leaselib.LeaseRegistry(
            self.cfg, {5432: "PostgreSQL"}, now=self.clock,
            mux_add=lambda n, p: self.mux.append(("add", n, p)),
            mux_remove=lambda n: self.mux.append(("remove", n)),
            audit=lambda e, **f: self.audit.append((e, f.get("port"), f.get("reason"))),
        )

    def test_create_adds_muximux_app_and_audits(self):
        lease, created = self.reg.create(4096, "s1")
        self.assertTrue(created)
        self.assertEqual(lease["name"], "Lease :4096")
        self.assertEqual(lease["url"], "/p/4096/")
        self.assertIn(("add", "Lease :4096", 4096), self.mux)
        self.assertEqual(self.audit[0], ("open", 4096, None))

    def test_second_create_same_port_is_idempotent(self):
        l1, c1 = self.reg.create(4096, "s1")
        l2, c2 = self.reg.create(4096, "s2")
        self.assertTrue(c1 and not c2)
        self.assertEqual(l1["id"], l2["id"])
        self.assertEqual(len(self.mux), 1)  # only one muximux app
        self.assertEqual(l2["sessions"], 2)

    def test_denied_port(self):
        with self.assertRaises(leaselib.PortDenied):
            self.reg.create(5432, "s1")
        self.assertEqual(self.mux, [])

    def test_grace_close_after_session_drop(self):
        lease, _ = self.reg.create(4096, "s1")
        self.reg.session_closed("s1")
        snap = self.reg.get(lease["id"])
        self.assertIsNotNone(snap)
        self.assertAlmostEqual(snap["close_at"], self.clock.t + 30)
        self.clock.advance(29)
        self.assertEqual(self.reg.sweep(), [])
        self.clock.advance(2)  # past grace
        self.assertEqual(self.reg.sweep(), [(lease["id"], 4096, "session closed, grace expired")])
        self.assertIsNone(self.reg.get(lease["id"]))
        self.assertIn(("remove", "Lease :4096"), self.mux)
        self.assertEqual(self.audit[-1], ("close", 4096, "session closed, grace expired"))

    def test_readopt_cancels_grace(self):
        lease, _ = self.reg.create(4096, "s1")
        self.reg.session_closed("s1")
        self.clock.advance(10)
        self.assertTrue(self.reg.adopt(lease["id"], "s1"))
        self.clock.advance(60)
        self.assertEqual(self.reg.sweep(), [])
        self.assertIsNotNone(self.reg.get(lease["id"]))

    def test_tab_switch_within_grace_keeps_lease(self):
        lease, _ = self.reg.create(4096, "s1")
        self.reg.session_closed("s1")          # ws drops on switch
        self.clock.advance(5)
        self.reg.create(4096, "s1")            # reconnect same session, re-attach
        self.clock.advance(60)
        self.assertEqual(self.reg.sweep(), [])
        self.assertIsNotNone(self.reg.get(lease["id"]))

    def test_other_session_holds_lease_past_first_drop(self):
        lease, _ = self.reg.create(4096, "s1")
        self.reg.create(4096, "s2")
        self.reg.session_closed("s1")
        self.clock.advance(120)
        self.assertEqual(self.reg.sweep(), [])
        self.assertIsNotNone(self.reg.get(lease["id"]))

    def test_hard_cap_8h_even_while_connected(self):
        lease, _ = self.reg.create(4096, "s1")
        self.clock.advance(self.cfg["max_lifetime_seconds"] + 1)
        self.assertEqual(self.reg.sweep(), [(lease["id"], 4096, "hard cap 8h expired")])
        self.assertIn(("remove", "Lease :4096"), self.mux)

    def test_user_close_is_immediate(self):
        lease, _ = self.reg.create(4096, "s1")
        self.assertTrue(self.reg.close(lease["id"], "closed by user"))
        self.assertFalse(self.reg.close(lease["id"], "again"))
        self.assertEqual(self.reg.active(), [])

    def test_purge_all_on_restart(self):
        self.reg.create(4096, "s1")
        self.reg.create(3730, "s1")
        names = self.reg.purge_all("lease-api restart purge")
        self.assertEqual(names, ["Lease :4096", "Lease :3730"])
        self.assertEqual(self.reg.active(), [])
        self.assertEqual(self.mux.count(("remove", "Lease :4096")), 1)

    def test_audit_log_written(self):
        with tempfile.TemporaryDirectory() as d:
            audit = leaselib.AuditLog(os.path.join(d, "log", "a.jsonl"))
            audit("open", port=4096, lease_id="abc", session="s1")
            lines = open(os.path.join(d, "log", "a.jsonl")).read().strip().split("\n")
            self.assertEqual(len(lines), 1)
            self.assertIn('"port": 4096', lines[0])

    def test_deny_yaml_load(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "ports-deny.yaml")
            with open(p, "w") as f:
                f.write("deny:\n  - {port: 22, reason: sshd}\n")
            self.assertEqual(leaselib.load_deny(p), {22: "sshd"})
        self.assertEqual(leaselib.load_deny("/nonexistent"), {})


class TestMuxRollback(unittest.TestCase):
    def test_mux_fail_rolls_back(self):
        reg, mux, audit = make_registry()
        reg._mux_add = lambda name, port: (_ for _ in ()).throw(ConnectionError("boom"))
        with self.assertRaises(ConnectionError):
            reg.create(4096, "s1")
        self.assertEqual(reg.active(), [])
        self.assertIn(("remove", "Lease :4096"), mux)
if __name__ == "__main__":
    unittest.main()

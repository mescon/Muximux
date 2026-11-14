#!/usr/bin/python3
"""Tests for the app-level port registry (M-245).

Run: cd /mnt/workspace/portal && python3 -m unittest discover -s tests -v

Covers: schema validation (positive + rejections), `ports alloc` conflict
behaviour, the registration reaper, and the ports-api endpoints incl. token
auth. Everything runs against temp dirs under /mnt/workspace/tmp — never /tmp,
never the live registry files.
"""
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
sys.path.insert(0, os.path.join(ROOT, "lib"))
import portslib  # noqa: E402

import yaml  # noqa: E402

TMP_BASE = os.environ.get("PORTS_TEST_TMP", "/mnt/workspace/tmp")
ALLOC_LO, ALLOC_HI = 45000, 45999

MINI_YAML = """\
version: 1
alloc:
  range: [45000, 45999]
dynamic: []
ignore: [22]
static:
  - id: dsh-x.web
    app: dsh
    lane: x
    role: web
    port: 3181
    bind: 127.0.0.1
    unit: ""
    health: ""
    portal: {title: DSH-X, tab: true}
"""


def tmp_home():
    os.makedirs(TMP_BASE, exist_ok=True)
    home = tempfile.mkdtemp(prefix="ports-test-", dir=TMP_BASE)
    with open(os.path.join(home, "ports.yaml"), "w") as f:
        f.write(MINI_YAML)
    return home


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class SchemaTest(unittest.TestCase):
    def test_repo_registry_is_valid(self):
        reg = portslib.load_registry(ROOT)
        self.assertEqual(reg["version"], 1)
        self.assertTrue(reg["static"])

    def _mutated(self, mutate):
        data = yaml.safe_load(MINI_YAML)
        mutate(data)
        return data

    def _reject(self, data):
        with self.assertRaises(portslib.RegistryError):
            portslib.validate(data)

    def test_rejects_bad_app(self):
        self._reject(self._mutated(lambda d: d["static"][0].update(app="nope")))

    def test_rejects_id_field_mismatch(self):
        self._reject(self._mutated(lambda d: d["static"][0].update(id="dsh-z.web")))

    def test_rejects_lane_omitted_from_id(self):
        # id has no lane segment but lane is x
        self._reject(self._mutated(lambda d: d["static"][0].update(id="dsh.web")))

    def test_rejects_duplicate_port(self):
        def dup(d):
            d["static"].append(dict(d["static"][0], id="dsh-x.dup"))
        self._reject(self._mutated(dup))

    def test_rejects_bad_portal_tab(self):
        self._reject(self._mutated(lambda d: d["static"][0]["portal"].update(tab="yes")))

    def test_rejects_dynamic_rule_with_port(self):
        def rule(d):
            d["dynamic"] = [{"id": "freebuff.engine", "app": "freebuff", "lane": "none",
                             "role": "engine", "match": "x", "portal": {"title": "t", "tab": "auto"},
                             "port": 1}]
        self._reject(self._mutated(rule))

    def test_rejects_bad_match_regex(self):
        def rule(d):
            d["dynamic"] = [{"id": "freebuff.engine", "app": "freebuff", "lane": "none",
                             "role": "engine", "match": "(", "portal": {"title": "t", "tab": True}}]
        self._reject(self._mutated(rule))


class AllocTest(unittest.TestCase):
    def setUp(self):
        self.home = tmp_home()
        self.addCleanup(shutil.rmtree, self.home, ignore_errors=True)
        self.yaml_path = os.path.join(self.home, "ports.yaml")
        self.env = dict(os.environ, PORTS_HOME=self.home)

    def _alloc(self, *args):
        return subprocess.run(
            [sys.executable, os.path.join(ROOT, "bin", "ports"), "alloc", *args],
            env=self.env, capture_output=True, text=True, timeout=30)

    def test_refuses_taken_port(self):
        with socket.socket() as s:
            s.bind(("127.0.0.1", 0))
            s.listen(1)
            taken = s.getsockname()[1]
            before = open(self.yaml_path, "rb").read()
            r = self._alloc("dsh", "x", "web", "--port", str(taken))
            self.assertEqual(r.returncode, 1, r.stderr)
            self.assertIn("taken", r.stderr)
            self.assertEqual(open(self.yaml_path, "rb").read(), before)

    def test_refuses_duplicate_id(self):
        before = open(self.yaml_path, "rb").read()
        r = self._alloc("dsh", "x", "web", "--port", str(free_port()))
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn("already in ports.yaml", r.stderr)
        self.assertEqual(open(self.yaml_path, "rb").read(), before)

    def test_picks_free_port_and_appends_valid_entry(self):
        r = self._alloc("dsh", "x", "web2")
        self.assertEqual(r.returncode, 0, r.stderr)
        m = re.search(r"port (\d+)", r.stdout)
        self.assertIsNotNone(m, r.stdout)
        port = int(m.group(1))
        self.assertTrue(ALLOC_LO <= port <= ALLOC_HI, port)
        reg = portslib.load_registry(self.home)  # re-validates the file
        entry = [e for e in reg["static"] if e["id"] == "dsh-x.web2"]
        self.assertEqual(len(entry), 1)
        self.assertEqual(entry[0]["port"], port)
        self.assertEqual(entry[0]["portal"]["tab"], False)


class ReaperTest(unittest.TestCase):
    def setUp(self):
        self.now = 1_000_000
        self.listening = {45000, 45001}
        self.base = {"id": "agents-none.web", "app": "agents", "lane": "none",
                     "role": "web", "port": 45000, "ttl": 60, "last_seen": self.now}

    def test_keeps_healthy_entry(self):
        out = portslib.reap_registered([dict(self.base, pid=1)], self.listening,
                                       self.now, alive=lambda p: True)
        self.assertEqual(len(out), 1)

    def test_drops_pid_gone(self):
        out = portslib.reap_registered([dict(self.base, pid=1)], self.listening,
                                       self.now, alive=lambda p: False)
        self.assertEqual(out, [])

    def test_drops_port_not_listening(self):
        out = portslib.reap_registered([dict(self.base, port=45999, pid=1)],
                                       self.listening, self.now, alive=lambda p: True)
        self.assertEqual(out, [])

    def test_drops_ttl_expired(self):
        out = portslib.reap_registered([dict(self.base, pid=1)], self.listening,
                                       self.now + 61, alive=lambda p: True)
        self.assertEqual(out, [])


class ApiTest(unittest.TestCase):
    def setUp(self):
        self.home = tmp_home()
        self.addCleanup(shutil.rmtree, self.home, ignore_errors=True)
        self.port = free_port()
        env = dict(os.environ, PORTS_HOME=self.home, PORTS_API_PORT=str(self.port))
        self.proc = subprocess.Popen(
            [sys.executable, os.path.join(ROOT, "bin", "ports-api")],
            env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        self.addCleanup(self._stop)
        deadline = time.time() + 10
        while True:
            try:
                socket.create_connection(("127.0.0.1", self.port), 0.2).close()
                break
            except OSError:
                if time.time() > deadline or self.proc.poll() is not None:
                    out = self.proc.stdout.read() if self.proc.poll() is not None else ""
                    self.fail(f"ports-api did not start: {out}")
                time.sleep(0.05)
        with open(os.path.join(self.home, ".ports.token")) as f:
            self.token = f.read().strip()

    def _stop(self):
        if self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.proc.kill()

    def _req(self, method, path, body=None, token=None):
        import urllib.error
        import urllib.request
        data = json.dumps(body).encode() if body is not None else None
        headers = {}
        if token is not None:
            headers["X-Ports-Token"] = token
        req = urllib.request.Request(f"http://127.0.0.1:{self.port}{path}",
                                     data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=5) as resp:
                return resp.status, json.loads(resp.read().decode() or "{}")
        except urllib.error.HTTPError as exc:
            return exc.code, json.loads(exc.read().decode() or "{}")

    def test_registration_lifecycle_and_auth(self):
        # auth: no token / wrong token refused on every method
        self.assertEqual(self._req("GET", "/v1/ports")[0], 401)
        self.assertEqual(self._req("GET", "/v1/ports", token="wrong")[0], 401)
        self.assertEqual(self._req("POST", "/v1/ports", body={}, token="wrong")[0], 401)

        # a real listening process registers and shows up
        srv_port = free_port()
        srv = subprocess.Popen([sys.executable, "-m", "http.server", str(srv_port),
                                "--bind", "127.0.0.1"],
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.addCleanup(lambda: srv.poll() is None and srv.terminate())
        deadline = time.time() + 10
        while True:
            try:
                socket.create_connection(("127.0.0.1", srv_port), 0.2).close()
                break
            except OSError:
                if time.time() > deadline:
                    self.fail("test http.server did not start")
                time.sleep(0.05)

        code, resp = self._req("POST", "/v1/ports", token=self.token, body={
            "id": "agents-none.web", "app": "agents", "lane": "none", "role": "web",
            "port": srv_port, "pid": srv.pid, "ttl": 60})
        self.assertEqual(code, 201, resp)
        code, data = self._req("GET", "/v1/ports?app=agents", token=self.token)
        self.assertEqual(code, 200)
        ids = [e["id"] for e in data["entries"]]
        self.assertIn("agents-none.web", ids)

        # heartbeat and delete
        self.assertEqual(self._req("PUT", "/v1/ports/agents-none.web", token=self.token)[0], 200)
        self.assertEqual(self._req("PUT", "/v1/ports/nope", token=self.token)[0], 404)
        code, resp = self._req("DELETE", "/v1/ports/agents-none.web", token=self.token)
        self.assertEqual((code, resp["removed"]), (200, 1))
        self.assertEqual(self._req("DELETE", "/v1/ports/agents-none.web",
                                   token=self.token)[1]["removed"], 0)

        # registering a port that is not listening is refused
        code, resp = self._req("POST", "/v1/ports", token=self.token, body={
            "id": "agents-none.dead", "app": "agents", "lane": "none", "role": "web",
            "port": free_port()})
        self.assertEqual(code, 400, resp)

        # schema violations are refused
        code, resp = self._req("POST", "/v1/ports", token=self.token, body={
            "id": "agents-none.bad", "app": "nope", "lane": "none", "role": "web",
            "port": srv_port})
        self.assertEqual(code, 400, resp)

        # re-register, then kill the process: reaped from the view on next read
        code, resp = self._req("POST", "/v1/ports", token=self.token, body={
            "id": "agents-none.web", "app": "agents", "lane": "none", "role": "web",
            "port": srv_port, "pid": srv.pid, "ttl": 60})
        self.assertEqual(code, 201, resp)
        srv.terminate()
        srv.wait(timeout=10)
        code, data = self._req("GET", "/v1/ports", token=self.token)
        self.assertEqual(code, 200)
        self.assertNotIn("agents-none.web", [e["id"] for e in data["entries"]])


if __name__ == "__main__":
    unittest.main()

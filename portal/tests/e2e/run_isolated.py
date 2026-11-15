#!/usr/bin/env python3
"""Run a portal browser test with temporary front and gateway listeners."""
import importlib.machinery
import importlib.util
import os
import socket
import subprocess
import sys
import threading
import time
from http.server import ThreadingHTTPServer
from pathlib import Path

root = Path(__file__).resolve().parents[2]
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
env = os.environ.copy()
env["PORTAL_GW_DENY_FILE"] = str(root / "tests/e2e/deny-fixture.yaml")
gateway = subprocess.Popen([sys.executable, str(root / "bin/portal-gw"), str(port)],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, env=env)
loader = importlib.machinery.SourceFileLoader("portal_front", str(root / "bin/portal-front"))
front = importlib.util.module_from_spec(importlib.util.spec_from_loader(loader.name, loader))
loader.exec_module(front)
front.GW_PORT = port
front.EXT_JS = str(root / "ui/ports/ext.js")
front.Handler.log_message = lambda *args: None
server = ThreadingHTTPServer(("127.0.0.1", 0), front.Handler)
server.handle_error = lambda *args: None
threading.Thread(target=server.serve_forever, daemon=True).start()
try:
    for _ in range(50):
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=.2):
                break
        except OSError:
            time.sleep(.1)
    else:
        raise RuntimeError("temporary gateway did not start")
    script = Path(sys.argv[1]).resolve()
    subprocess.run([sys.executable, str(script), f"http://127.0.0.1:{server.server_port}"],
                   check=True, timeout=150)
finally:
    server.shutdown()
    server.server_close()
    gateway.terminate()
    try:
        gateway.wait(timeout=8)
    except subprocess.TimeoutExpired:
        gateway.kill()
        gateway.wait()

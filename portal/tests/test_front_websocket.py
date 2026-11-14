"""Exercise WebSocket routing through a real portal-front handler."""
import importlib.machinery
import importlib.util
import socket
import threading
import unittest
import urllib.request
import urllib.error
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

loader = importlib.machinery.SourceFileLoader(
    "portal_front", str(Path(__file__).resolve().parents[1] / "bin/portal-front"))
front = importlib.util.module_from_spec(importlib.util.spec_from_loader(loader.name, loader))
loader.exec_module(front)


class FrontWebsocketRouting(unittest.TestCase):
    def test_event_stream_reaches_client_before_upstream_closes(self):
        release = threading.Event()
        class Gateway(BaseHTTPRequestHandler):
            def do_GET(self):
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                self.wfile.write(b"data: ready\n\n")
                self.wfile.flush()
                release.wait(3)
            def log_message(self, *args):
                pass
        gateway = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
        front.GW_PORT = gateway.server_port
        server = ThreadingHTTPServer(("127.0.0.1", 0), front.Handler)
        for srv in (gateway, server):
            threading.Thread(target=srv.serve_forever, daemon=True).start()
        try:
            with socket.create_connection(server.server_address, timeout=2) as client:
                client.settimeout(2)
                client.sendall(b"GET /p/4096/global/event HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
                with client.makefile("rb") as response:
                    self.assertIn(b"200 OK", response.readline())
                    while response.readline() not in (b"\r\n", b""):
                        pass
                    self.assertEqual(response.readline(), b"data: ready\n")
        finally:
            release.set()
            server.shutdown()
            server.server_close()
            gateway.shutdown()
            gateway.server_close()

    def test_tailnet_probe_origin_is_added_without_relaxing_other_sources(self):
        result = front.allow_tailnet_probe("default-src 'self'; connect-src 'self'; object-src 'none'")
        self.assertIn("connect-src 'self' https://tailnet-host.example.ts.net", result)
        self.assertIn("object-src 'none'", result)
        self.assertNotIn("connect-src *", result)

    def test_claimed_top_level_paths_redirect_to_the_portal(self):
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, *args, **kwargs):
                return None
        server = ThreadingHTTPServer(("127.0.0.1", 0), front.Handler)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        opener = urllib.request.build_opener(NoRedirect)
        try:
            base = f"http://127.0.0.1:{server.server_port}"
            for path, location in (
                ("/moct/runtimes/local%3A123?x=1", "/p/3701/moct/runtimes/local%3A123?x=1"),
                ("/p/3701/moct/runtimes/local%3A123?x=1", "/?gateway_port=3701&gateway_path=%2Fmoct%2Fruntimes%2Flocal%253A123%3Fx%3D1"),
            ):
                request = urllib.request.Request(base + path, headers={"Sec-Fetch-Dest": "document"})
                with self.assertRaises(urllib.error.HTTPError) as caught:
                    opener.open(request, timeout=3)
                self.assertEqual(caught.exception.code, 302)
                self.assertEqual(caught.exception.headers["Location"], location)
        finally:
            server.shutdown()
            server.server_close()

    def test_prefixed_upgrade_reaches_gateway_and_regular_upgrade_reaches_mux(self):
        listeners = []
        seen = []
        threads = []
        for label in ("gateway", "mux"):
            sock = socket.socket()
            sock.bind(("127.0.0.1", 0))
            sock.listen(1)
            sock.settimeout(3)
            listeners.append(sock)

            def upstream(listener=sock, target=label):
                conn, _ = listener.accept()
                with conn:
                    data = conn.recv(4096)
                    seen.append((target, data.split(b"\r\n", 1)[0]))
                    conn.sendall(b"HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")

            thread = threading.Thread(target=upstream, daemon=True)
            thread.start()
            threads.append(thread)
        front.GW_PORT = listeners[0].getsockname()[1]
        front.MUX_PORT = listeners[1].getsockname()[1]
        server = ThreadingHTTPServer(("127.0.0.1", 0), front.Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            for path in ("/p/9090/cockpit/socket", "/ws"):
                with socket.create_connection(server.server_address, timeout=3) as client:
                    client.settimeout(3)
                    client.sendall((f"GET {path} HTTP/1.1\r\nHost: localhost\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n").encode())
                    self.assertIn(b"101 Switching Protocols", client.recv(4096))
            for t in threads:
                t.join(3)
            self.assertEqual(seen, [("gateway", b"GET /p/9090/cockpit/socket HTTP/1.1"),
                                    ("mux", b"GET /ws HTTP/1.1")])
        finally:
            server.shutdown()
            server.server_close()
            for listener in listeners:
                listener.close()

    def test_first_load_asset_uses_same_origin_gateway_referrer(self):
        paths = []
        class Gateway(BaseHTTPRequestHandler):
            def do_GET(self):
                paths.append(self.path)
                self.send_response(200)
                self.send_header("Content-Length", "0")
                self.end_headers()
            def log_message(self, *args):
                pass
        gateway = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
        front.GW_PORT = gateway.server_port
        server = ThreadingHTTPServer(("127.0.0.1", 0), front.Handler)
        for srv in (gateway, server):
            threading.Thread(target=srv.serve_forever, daemon=True).start()
        try:
            host = f"127.0.0.1:{server.server_port}"
            with socket.create_connection(server.server_address, timeout=3) as client:
                client.sendall((f"GET /assets/app.js?v=1 HTTP/1.1\r\nHost: {host}\r\n"
                                f"Referer: http://{host}/p/4096/\r\n"
                                "Sec-Fetch-Mode: no-cors\r\nSec-Fetch-Dest: script\r\nConnection: close\r\n\r\n").encode())
                self.assertIn(b"200 OK", client.recv(4096))
            self.assertEqual(paths, ["/p/4096/assets/app.js?v=1"])
        finally:
            server.shutdown()
            server.server_close()
            gateway.shutdown()
            gateway.server_close()


if __name__ == "__main__":
    unittest.main()

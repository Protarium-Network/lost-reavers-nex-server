"""
Minimal logging stub for Lost Reavers' own Bandai Namco backend
(cos.xml's GAMESERVER_ADDRESS/GAMESERVER_PORT), completely separate from
Nintendo NEX/NNAS. Nothing about this protocol is publicly documented, so
this just logs every request in full (method, path, headers, body) and
replies with a minimal generic "success" so we can see what the game
actually asks for and iterate.
"""

import http.server
import ssl

class Handler(http.server.BaseHTTPRequestHandler):
    def _handle(self):
        length = int(self.headers.get("Content-Length", 0))
        payload = self.rfile.read(length) if length else b""

        print("=" * 70)
        print(f"[bnei] {self.command} {self.path} from {self.client_address}")
        print("[bnei] headers:")
        for k, v in self.headers.items():
            print(f"[bnei]   {k}: {v}")
        print(f"[bnei] body ({len(payload)} bytes):")
        print(payload[:2000])
        print("=" * 70)

        body = b"{}"
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self._handle()

    def do_POST(self):
        self._handle()

    def do_PUT(self):
        self._handle()

    def log_message(self, format, *args):
        pass


def main():
    server = http.server.ThreadingHTTPServer(("0.0.0.0", 8444), Handler)
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.load_cert_chain(certfile="cert.pem", keyfile="key.pem")
    ctx.minimum_version = ssl.TLSVersion.TLSv1
    try:
        ctx.set_ciphers("DEFAULT@SECLEVEL=1")
    except ssl.SSLError:
        pass
    server.socket = ctx.wrap_socket(server.socket, server_side=True)
    print("[bnei] listening on https://0.0.0.0:8444")
    server.serve_forever()


if __name__ == "__main__":
    main()

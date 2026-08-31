"""
Local stand-in for Cemu's <act> account-server endpoint, just for capturing
Lost Reavers' first PRUDP packet.

Forwarding to the real account.protarium.lol turned out not to work: it
proxies to Pretendo's real production account.pretendo.cc behind Cloudflare,
which 403s any request that doesn't look like it came from a real console
(TLS/JA3 fingerprint, and possibly the Wii U's shared client certificate for
mTLS -- see https://github.com/kinnay/NintendoClients/wiki/Account-Server).
There is no way to make a plain Python HTTPS client pass that check.

So instead of proxying, this answers the two calls Cemu needs to reach
the point of asking for a NEX token, entirely locally:

  POST /v1/api/oauth20/access_token/generate  -> canned OAuth20 token
  GET  /v1/api/provider/nex_token/@me         -> canned NEX token pointing
                                                  at the local
                                                  lost-reavers-nex-server

Nothing else is answered meaningfully; PN_LR_LOCAL_AUTH mode on the NEX
server doesn't validate any of this content, it only checks the NEX
password in settings.json once the console reaches LoginEx.
"""

import base64
import hashlib
import hmac
import http.server
import secrets
import ssl
import struct
import time
from urllib.parse import urlparse, parse_qs

GAME_SERVER_ID = "1018D900"

# Must match settings.json in the lost-reavers-nex-server repo.
LOCAL_NEX_HOST = "192.168.1.110"
LOCAL_NEX_PORT = "60000"
LOCAL_NEX_PID = "1799999999"
LOCAL_NEX_PASSWORD = "aBcDeFgHiJkLmNoP"

OAUTH_XML = f"""<?xml version="1.0" encoding="utf-8"?>
<OAuth20>
  <access_token>
    <token>{secrets.token_hex(16)}</token>
    <refresh_token>{secrets.token_hex(20)}</refresh_token>
    <expires_in>3600</expires_in>
  </access_token>
</OAuth20>"""

NEX_TOKEN_XML = f"""<?xml version="1.0" encoding="utf-8"?>
<nex_token>
  <host>{LOCAL_NEX_HOST}</host>
  <nex_password>{LOCAL_NEX_PASSWORD}</nex_password>
  <pid>{LOCAL_NEX_PID}</pid>
  <port>{LOCAL_NEX_PORT}</port>
  <token>capture_nex_token</token>
</nex_token>"""

PROFILE_XML = f"""<?xml version="1.0" encoding="utf-8"?>
<person>
  <pid>{LOCAL_NEX_PID}</pid>
  <user_id>{LOCAL_NEX_PID}</user_id>
  <mii><name>Capture</name></mii>
</person>"""

TITLE_ID_EUR = 0x00050000101B9900  # Lost Reavers EUR, the build being tested


def build_service_token(pid: int, title_id: int) -> str:
    """
    Mirrors PretendoNetwork/account's createServiceToken (src/util.ts):
    a 28-byte struct (pid, title_id, issued, expires -- all big-endian)
    followed by a 32-byte HMAC-SHA256, base64-encoded. The console can't
    verify the HMAC (it has no copy of the per-game secret -- only the real
    game backend would), so a made-up key is fine; what matters is that the
    struct is the right shape so client-side parsing doesn't choke on it.
    """
    now_ms = int(time.time() * 1000)
    expires_ms = now_ms + 3600 * 1000

    data = struct.pack(">I", pid) + struct.pack(">QQQ", title_id, now_ms, expires_ms)
    signature = hmac.new(b"capture-fake-server-key", data, hashlib.sha256).digest()

    return base64.b64encode(data + signature).decode("ascii")


def service_token_xml() -> str:
    token = build_service_token(int(LOCAL_NEX_PID), TITLE_ID_EUR)
    return f"""<?xml version="1.0" encoding="utf-8"?>
<service_token>
  <token>{token}</token>
</service_token>"""


class Handler(http.server.BaseHTTPRequestHandler):
    def _reply(self, status, body_str, content_type="text/xml"):
        body = body_str.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Server", "Nintendo 3DS (http)")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self._handle()

    def do_POST(self):
        self._handle()

    def do_PUT(self):
        self._handle()

    def _handle(self):
        length = int(self.headers.get("Content-Length", 0))
        payload = self.rfile.read(length) if length else b""

        parsed = urlparse(self.path)
        path = parsed.path
        qs = parse_qs(parsed.query)

        print(f"[proxy] {self.command} {self.path} from {self.client_address} "
              f"({len(payload)} bytes body)")

        if path == "/v1/api/oauth20/access_token/generate":
            print("[proxy] -> canned OAuth20 token")
            self._reply(200, OAUTH_XML)
            return

        if path == "/v1/api/provider/nex_token/@me":
            requested = qs.get("game_server_id", [None])[0]
            print(f"[proxy] -> canned NEX token for game_server_id={requested} "
                  f"({LOCAL_NEX_HOST}:{LOCAL_NEX_PORT})")
            self._reply(200, NEX_TOKEN_XML)
            return

        if path == "/v1/api/provider/service_token/@me":
            print("[proxy] -> canned service token (proper 60-byte structure)")
            self._reply(200, service_token_xml())
            return

        if path == "/v1/api/people/@me/profile":
            print("[proxy] -> canned profile")
            self._reply(200, PROFILE_XML)
            return

        # Anything else: empty-ish success rather than an error, so an
        # unexpected call doesn't hard-fail the boot sequence. Logged so it's
        # obvious what else Cemu asked for if the capture still doesn't reach
        # PRUDP.
        print(f"[proxy] -> unhandled path, replying empty 200")
        self._reply(200, "<result/>")

    def log_message(self, format, *args):
        pass  # quiet; we print our own lines above


def main():
    server = http.server.ThreadingHTTPServer(("0.0.0.0", 8443), Handler)
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.load_cert_chain(certfile="cert.pem", keyfile="key.pem")
    ctx.minimum_version = ssl.TLSVersion.TLSv1
    try:
        ctx.set_ciphers("DEFAULT@SECLEVEL=1")
    except ssl.SSLError:
        pass
    server.socket = ctx.wrap_socket(server.socket, server_side=True)
    print("[proxy] listening on https://0.0.0.0:8443 (fully local, no forwarding)")
    print(f"[proxy] game_server_id={GAME_SERVER_ID} -> {LOCAL_NEX_HOST}:{LOCAL_NEX_PORT}, "
          f"pid={LOCAL_NEX_PID}")
    server.serve_forever()


if __name__ == "__main__":
    main()

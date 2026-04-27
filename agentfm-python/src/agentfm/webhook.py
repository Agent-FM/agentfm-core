"""Stdlib-only webhook receiver for async-task callbacks.

Use it when your code submits an async task and wants to be notified when it
completes:

    def on_done(payload):
        print(f"task {payload['task_id']} finished")

    WebhookReceiver(port=8000, callback=on_done).serve_forever()

The receiver intentionally avoids FastAPI / Flask so the SDK doesn't drag a
web framework into every install.
"""

from __future__ import annotations

import hmac
import json
import logging
import threading
from collections.abc import Callable
from hashlib import sha256
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from pydantic import BaseModel, ConfigDict, ValidationError

from .models import PeerID

_log = logging.getLogger(__name__)

# Per-request body cap. Real AgentFM callbacks are ~200 bytes; 64 KiB leaves
# headroom for any future field additions while making a Content-Length DoS
# (`Content-Length: 4000000000`) bounded at first read.
DEFAULT_MAX_BODY_BYTES = 64 * 1024

# Header the Go boss writes when AGENTFM_WEBHOOK_SECRET is set on its side.
# Receivers configured with the matching secret verify in constant time.
SIGNATURE_HEADER = "X-AgentFM-Signature"


class WebhookPayload(BaseModel):
    """Body the gateway POSTs to a webhook URL when an async task completes."""

    model_config = ConfigDict(extra="allow")

    task_id: str
    worker_id: PeerID
    status: str


WebhookCallback = Callable[[WebhookPayload], None]


class WebhookReceiver:
    """Tiny HTTP server that POSTs land on a single callback."""

    def __init__(
        self,
        *,
        port: int,
        callback: WebhookCallback,
        host: str = "127.0.0.1",
        path: str = "/cb",
        secret: str | None = None,
        max_body_bytes: int = DEFAULT_MAX_BODY_BYTES,
    ) -> None:
        """Bind to ``host:port`` and POST decoded payloads to ``callback``.

        ``host`` defaults to ``127.0.0.1`` (loopback only). Pass ``"0.0.0.0"``
        explicitly to accept callbacks from off-host gateways.

        ``secret`` enables HMAC-SHA256 signature verification of the request
        body. Set the same secret on the boss side via the environment
        variable ``AGENTFM_WEBHOOK_SECRET``. When ``secret`` is ``None``,
        unsigned requests are accepted (compatible with un-configured gateways
        and trusted-network deployments).

        ``max_body_bytes`` caps the request body. Anything bigger gets a 413
        and the rest of the body is discarded. Defends against a malicious
        ``Content-Length: 4000000000`` blowing up RAM at first read.
        """
        self.host = host
        self.port = port
        self.path = path
        self.callback = callback
        self.secret = secret
        self.max_body_bytes = max_body_bytes
        self._server: ThreadingHTTPServer | None = None
        self._thread: threading.Thread | None = None

    # -- context manager (background) ---------------------------------------

    def __enter__(self) -> WebhookReceiver:
        self.start()
        return self

    def __exit__(self, *exc: Any) -> None:
        self.stop()

    # -- explicit lifecycle -------------------------------------------------

    def start(self) -> None:
        """Run the server in a background daemon thread."""
        if self._server is not None:
            return
        handler = self._make_handler()
        self._server = ThreadingHTTPServer((self.host, self.port), handler)
        self._thread = threading.Thread(
            target=self._server.serve_forever,
            name="agentfm-webhook",
            daemon=True,
        )
        self._thread.start()
        _log.info("webhook receiver listening on http://%s:%s%s", self.host, self.port, self.path)

    def stop(self) -> None:
        if self._server is not None:
            self._server.shutdown()
            self._server.server_close()
            self._server = None
        if self._thread is not None:
            self._thread.join(timeout=2.0)
            self._thread = None

    def serve_forever(self) -> None:
        """Block on the foreground thread until interrupted."""
        handler = self._make_handler()
        with ThreadingHTTPServer((self.host, self.port), handler) as srv:
            self._server = srv
            _log.info("webhook receiver listening on http://%s:%s%s", self.host, self.port, self.path)
            try:
                srv.serve_forever()
            except KeyboardInterrupt:
                _log.info("webhook receiver shutting down on Ctrl-C")
            finally:
                # Clear so a subsequent start() does not silently no-op
                # (start() returns early when self._server is non-None).
                self._server = None

    # -- internals ----------------------------------------------------------

    def _make_handler(self) -> type[BaseHTTPRequestHandler]:
        path = self.path
        callback = self.callback
        secret = self.secret
        max_body_bytes = self.max_body_bytes

        class _Handler(BaseHTTPRequestHandler):
            def do_POST(self) -> None:
                if self.path != path:
                    self.send_response(404)
                    self.end_headers()
                    return

                # Reject early on body size. Cap is enforced inside
                # _parse_content_length so a hostile Content-Length:
                # 4000000000 cannot make rfile.read() allocate billions
                # of bytes — the parser fails closed before we touch the
                # body.
                content_length = self._parse_content_length(max_body_bytes)
                if content_length is None:
                    return  # response already sent

                content_type = self.headers.get("Content-Type", "").split(";")[0].strip().lower()
                if content_type and content_type != "application/json":
                    self.send_response(415)
                    self.end_headers()
                    return

                raw = self.rfile.read(content_length) if content_length > 0 else b""

                if secret is not None and not _verify_signature(
                    secret, raw, self.headers.get(SIGNATURE_HEADER, "")
                ):
                    self.send_response(401)
                    self.end_headers()
                    return

                try:
                    body = json.loads(raw.decode("utf-8")) if raw else {}
                    payload = WebhookPayload.model_validate(body)
                except (json.JSONDecodeError, UnicodeDecodeError, ValidationError) as exc:
                    _log.warning("rejected malformed webhook body: %s", exc)
                    self.send_response(400)
                    self.end_headers()
                    return

                try:
                    callback(payload)
                except Exception:
                    # User callbacks can raise anything; log and 500 rather
                    # than crash the server thread.
                    _log.exception("webhook callback raised")
                    self.send_response(500)
                    self.end_headers()
                    return

                self.send_response(200)
                self.end_headers()

            def _parse_content_length(self, cap: int) -> int | None:
                """Return Content-Length as int, or send 411 / 400 / 413 on failure.

                ``cap`` is the upper bound (max_body_bytes). Sending 413 here
                so the cap is enforced in one place and a future refactor that
                accepted chunked transfer-encoding can't accidentally bypass
                the size guard.
                """
                raw = self.headers.get("Content-Length")
                if raw is None:
                    self.send_response(411)
                    self.end_headers()
                    return None
                try:
                    n = int(raw)
                except ValueError:
                    self.send_response(400)
                    self.end_headers()
                    return None
                if n < 0:
                    self.send_response(400)
                    self.end_headers()
                    return None
                if n > cap:
                    self.send_response(413)
                    self.end_headers()
                    return None
                return n

            def log_message(self, format: str, *args: Any) -> None:
                # Route stdlib's noisy logging through the SDK logger.
                _log.debug(format, *args)

        return _Handler


def _verify_signature(secret: str, body: bytes, presented: str) -> bool:
    """Constant-time HMAC-SHA256 verification.

    Returns True iff `presented` matches the hex digest of HMAC-SHA256(body)
    keyed by `secret`. False on empty / wrong-length presentations.
    """
    if not presented:
        return False
    expected = hmac.new(secret.encode("utf-8"), body, sha256).hexdigest()
    return hmac.compare_digest(expected, presented)


__all__ = ["WebhookCallback", "WebhookPayload", "WebhookReceiver"]

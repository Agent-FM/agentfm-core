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

import json
import logging
import threading
from collections.abc import Callable
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from pydantic import BaseModel, ConfigDict

from .models import PeerID

_log = logging.getLogger(__name__)


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
        host: str = "0.0.0.0",
        path: str = "/cb",
    ) -> None:
        self.host = host
        self.port = port
        self.path = path
        self.callback = callback
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

    # -- internals ----------------------------------------------------------

    def _make_handler(self) -> type[BaseHTTPRequestHandler]:
        path = self.path
        callback = self.callback

        class _Handler(BaseHTTPRequestHandler):
            def do_POST(self_inner) -> None:
                if self_inner.path != path:
                    self_inner.send_response(404)
                    self_inner.end_headers()
                    return
                length = int(self_inner.headers.get("Content-Length", 0))
                raw = self_inner.rfile.read(length) if length > 0 else b""
                try:
                    body = json.loads(raw.decode("utf-8")) if raw else {}
                    payload = WebhookPayload.model_validate(body)
                except Exception as exc:  # pragma: no cover - safety net
                    _log.warning("rejected malformed webhook body: %s", exc)
                    self_inner.send_response(400)
                    self_inner.end_headers()
                    return
                try:
                    callback(payload)
                except Exception:  # pragma: no cover - user code
                    _log.exception("webhook callback raised")
                    self_inner.send_response(500)
                    self_inner.end_headers()
                    return
                self_inner.send_response(200)
                self_inner.end_headers()

            def log_message(self_inner, format: str, *args: Any) -> None:
                # Route stdlib's noisy logging through the SDK logger.
                _log.debug(format, *args)

        return _Handler


__all__ = ["WebhookCallback", "WebhookPayload", "WebhookReceiver"]

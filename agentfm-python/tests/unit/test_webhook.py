from __future__ import annotations

import hashlib
import hmac
import json
import time
import urllib.error
import urllib.request
from threading import Event

from agentfm import WebhookPayload, WebhookReceiver


def _post(url: str, body: dict) -> int:
    req = urllib.request.Request(
        url,
        data=json.dumps(body).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=2.0) as resp:
        return resp.status


def test_webhook_receiver_callback_fires(unused_tcp_port: int):
    received: list[WebhookPayload] = []
    fired = Event()

    def cb(payload: WebhookPayload) -> None:
        received.append(payload)
        fired.set()

    with WebhookReceiver(port=unused_tcp_port, callback=cb) as _:
        body = {"task_id": "t-1", "worker_id": "12D3KooWX", "status": "completed"}
        status = _post(f"http://127.0.0.1:{unused_tcp_port}/cb", body)
        assert status == 200
        assert fired.wait(2.0)

    assert len(received) == 1
    assert received[0].task_id == "t-1"
    assert received[0].status == "completed"


def test_webhook_receiver_rejects_wrong_path(unused_tcp_port: int):
    def cb(_p: WebhookPayload) -> None:
        raise AssertionError("should not be called")

    with WebhookReceiver(port=unused_tcp_port, callback=cb):
        req = urllib.request.Request(
            f"http://127.0.0.1:{unused_tcp_port}/wrong",
            data=b"{}",
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            urllib.request.urlopen(req, timeout=2.0)
        except urllib.error.HTTPError as e:
            assert e.code == 404
        else:
            raise AssertionError("expected 404")


def test_webhook_receiver_rejects_malformed_json(unused_tcp_port: int):
    def cb(_p: WebhookPayload) -> None:
        raise AssertionError("should not be called")

    with WebhookReceiver(port=unused_tcp_port, callback=cb):
        req = urllib.request.Request(
            f"http://127.0.0.1:{unused_tcp_port}/cb",
            data=b"not json",
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            urllib.request.urlopen(req, timeout=2.0)
        except urllib.error.HTTPError as e:
            assert e.code == 400
        else:
            raise AssertionError("expected 400")
        time.sleep(0.05)  # let server log


def _expect_status(url: str, *, body: bytes, headers: dict[str, str], want: int) -> None:
    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=2.0) as resp:
            got = resp.status
    except urllib.error.HTTPError as e:
        got = e.code
    assert got == want, f"status={got}, want {want}"


def test_webhook_receiver_rejects_oversize_body(unused_tcp_port: int):
    """A Content-Length above max_body_bytes must 413 before reading."""

    def cb(_p: WebhookPayload) -> None:
        raise AssertionError("oversized callback must not fire")

    with WebhookReceiver(port=unused_tcp_port, callback=cb, max_body_bytes=128):
        # Don't actually send 4 GB; just send headers + a body that exceeds
        # the cap. urllib.request will set Content-Length from len(data).
        big = b"x" * 256
        _expect_status(
            f"http://127.0.0.1:{unused_tcp_port}/cb",
            body=big,
            headers={"Content-Type": "application/json"},
            want=413,
        )


def test_webhook_receiver_rejects_wrong_content_type(unused_tcp_port: int):
    def cb(_p: WebhookPayload) -> None:
        raise AssertionError("wrong content type must not fire")

    with WebhookReceiver(port=unused_tcp_port, callback=cb):
        _expect_status(
            f"http://127.0.0.1:{unused_tcp_port}/cb",
            body=b'{"task_id":"t","worker_id":"12D3","status":"ok"}',
            headers={"Content-Type": "text/plain"},
            want=415,
        )


def test_webhook_receiver_requires_signature_when_secret_configured(unused_tcp_port: int):
    def cb(_p: WebhookPayload) -> None:
        raise AssertionError("unsigned request must not fire")

    with WebhookReceiver(port=unused_tcp_port, callback=cb, secret="topsecret"):
        _expect_status(
            f"http://127.0.0.1:{unused_tcp_port}/cb",
            body=b'{"task_id":"t","worker_id":"12D3","status":"ok"}',
            headers={"Content-Type": "application/json"},
            want=401,
        )


def test_webhook_receiver_accepts_valid_signature(unused_tcp_port: int):
    received: list[WebhookPayload] = []
    fired = Event()

    def cb(payload: WebhookPayload) -> None:
        received.append(payload)
        fired.set()

    secret = "topsecret"
    body = b'{"task_id":"t","worker_id":"12D3","status":"ok"}'
    sig = hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()

    with WebhookReceiver(port=unused_tcp_port, callback=cb, secret=secret):
        _expect_status(
            f"http://127.0.0.1:{unused_tcp_port}/cb",
            body=body,
            headers={"Content-Type": "application/json", "X-AgentFM-Signature": sig},
            want=200,
        )
        assert fired.wait(2.0)
    assert received[0].task_id == "t"


def test_serve_forever_resets_server_so_restart_works(unused_tcp_port: int):
    """Pre-fix, serve_forever's `with` block left self._server populated after
    KeyboardInterrupt; the next start() saw _server is not None and returned
    silently, so the receiver looked alive but accepted no requests."""
    import threading

    received: list[WebhookPayload] = []

    def cb(payload: WebhookPayload) -> None:
        received.append(payload)

    rx = WebhookReceiver(port=unused_tcp_port, callback=cb)

    # Run serve_forever in a thread, then shut it down to simulate the
    # KeyboardInterrupt-then-restart scenario.
    server_thread = threading.Thread(target=rx.serve_forever, daemon=True)
    server_thread.start()

    # Wait for the server to be alive
    deadline = time.monotonic() + 2.0
    while time.monotonic() < deadline and rx._server is None:
        time.sleep(0.02)
    assert rx._server is not None, "serve_forever didn't bind in time"

    # Shut it down by reaching into the server (KeyboardInterrupt would do this)
    rx._server.shutdown()
    server_thread.join(timeout=2.0)
    assert rx._server is None, (
        "serve_forever must clear self._server on exit so restart works"
    )

    # Now start() should actually start, not silently no-op.
    try:
        rx.start()
        assert rx._server is not None, "start() after serve_forever must rebind"
        body = b'{"task_id":"r2","worker_id":"12D3K","status":"ok"}'
        _expect_status(
            f"http://127.0.0.1:{unused_tcp_port}/cb",
            body=body,
            headers={"Content-Type": "application/json"},
            want=200,
        )
        time.sleep(0.1)
        assert received and received[0].task_id == "r2"
    finally:
        rx.stop()


def test_webhook_receiver_rejects_wrong_signature(unused_tcp_port: int):
    def cb(_p: WebhookPayload) -> None:
        raise AssertionError("wrong-sig request must not fire")

    body = b'{"task_id":"t","worker_id":"12D3","status":"ok"}'
    bad_sig = "0" * 64

    with WebhookReceiver(port=unused_tcp_port, callback=cb, secret="topsecret"):
        _expect_status(
            f"http://127.0.0.1:{unused_tcp_port}/cb",
            body=body,
            headers={"Content-Type": "application/json", "X-AgentFM-Signature": bad_sig},
            want=401,
        )

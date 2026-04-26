from __future__ import annotations

import json
import time
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
            method="POST",
        )
        try:
            urllib.request.urlopen(req, timeout=2.0)
        except urllib.error.HTTPError as e:
            assert e.code == 400
        else:
            raise AssertionError("expected 400")
        time.sleep(0.05)  # let server log

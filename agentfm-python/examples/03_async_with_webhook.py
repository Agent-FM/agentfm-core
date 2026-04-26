"""Submit a long-running job and receive a webhook callback when it finishes.

Uses only stdlib for the receiver — no FastAPI or Flask required.
"""

from __future__ import annotations

import threading

from agentfm import AgentFMClient, WebhookPayload, WebhookReceiver

PORT = 8765


def main() -> None:
    done = threading.Event()
    received: list[WebhookPayload] = []

    def on_done(payload: WebhookPayload) -> None:
        received.append(payload)
        done.set()

    with WebhookReceiver(port=PORT, callback=on_done), AgentFMClient() as client:
        worker = client.workers.list(available_only=True)[0]
        ack = client.tasks.submit_async(
            worker_id=worker.peer_id,
            prompt="A long-running task...",
            webhook_url=f"http://127.0.0.1:{PORT}/cb",
        )
        print(f"submitted task_id={ack.task_id}; waiting for callback...")
        done.wait(timeout=300)

    if received:
        print(f"got webhook: {received[0]}")
    else:
        print("timed out waiting for webhook")


if __name__ == "__main__":
    main()

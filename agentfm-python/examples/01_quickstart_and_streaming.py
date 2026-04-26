"""Hello-world: discover, pick by peer_id, dispatch.

Shows both the blocking ``tasks.run`` (returns a TaskResult) and the
generator-based ``tasks.stream`` (yields TaskChunks for live progress).
"""

from __future__ import annotations

import sys

from agentfm import AgentFMClient


def main() -> None:
    with AgentFMClient(gateway_url="http://127.0.0.1:8080") as client:
        # 1. Discover
        workers = client.workers.list(model="llama3.2", available_only=True)
        if not workers:
            print("No llama3.2 workers on the mesh right now.")
            return

        for w in workers:
            print(
                f"  {w.peer_id[:12]}...  {w.author!r}  "
                f"load={w.current_tasks}/{w.max_tasks}  status={w.status}"
            )

        # 2. Pick (peer_id is the cryptographically verifiable identifier)
        worker = workers[0]

        # 3a. Blocking dispatch
        result = client.tasks.run(
            worker_id=worker.peer_id,
            prompt="Draft a 200-word leave policy for a startup.",
        )
        print(result.text)
        print(f"\n{len(result.artifacts)} artifact(s) on disk:", result.artifacts)

        # 3b. Streaming dispatch (token-by-token UX)
        print("\n--- streaming round 2 ---")
        for chunk in client.tasks.stream(
            worker_id=worker.peer_id,
            prompt="Write a haiku about distributed systems.",
        ):
            sys.stdout.write(chunk.text)
            sys.stdout.flush()
        sys.stdout.write("\n")


if __name__ == "__main__":
    main()

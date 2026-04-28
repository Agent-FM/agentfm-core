"""Browse the mesh, pick a peer by ID, dispatch.

Why not filter by ``model="llama3.2"`` like 01_quickstart? Because operator-
supplied strings (``model``, ``agent_name``, ``author``) are exact-match —
a typo (``llmama3.2``) silently returns ``[]``. PeerIDs are
cryptographically verifiable Ed25519 base58 strings; they don't have a
typo failure mode.

Three patterns shown:

1. ``client.workers.list()`` with no kwargs — see everything online.
2. ``client.workers.get(peer_id)`` — fetch one when you already know the ID.
3. ``client.tasks.run(worker_id=peer_id, ...)`` — pin without a list call
   at all, when the peer_id is hard-coded (e.g. an internal mesh).
"""

from __future__ import annotations

import os
import sys

from agentfm import AgentFMClient, WorkerNotFoundError


def pattern_1_browse_and_pick() -> None:
    """List everything; print a numbered table; pick by index."""
    with AgentFMClient(gateway_url="http://127.0.0.1:8080") as client:
        workers = client.workers.list()
        if not workers:
            print("No workers on the mesh.")
            return

        print(f"{len(workers)} worker(s) online:\n")
        print(f"  {'#':>3}  {'peer_id':<14}  {'agent':<22}  {'model':<14}  {'load':>7}  status")
        print(f"  {'-'*3}  {'-'*14}  {'-'*22}  {'-'*14}  {'-'*7}  ------")
        for i, w in enumerate(workers):
            print(
                f"  {i:>3}  {w.peer_id[:12]+'..':<14}  "
                f"{(w.name or '')[:22]:<22}  {(w.model or '')[:14]:<14}  "
                f"{w.current_tasks:>2}/{w.max_tasks:<4}  {w.status}"
            )

        worker = workers[0]
        print(f"\nDispatching to {worker.peer_id[:12]}... ({worker.name!r})")
        result = client.tasks.run(
            worker_id=worker.peer_id,
            prompt="Write a haiku about distributed systems.",
        )
        print(result.text)


def pattern_2_get_one_by_id() -> None:
    """Fetch a known peer's profile directly. Raises if not in telemetry."""
    KNOWN_PEER = os.environ.get(
        "AGENTFM_PIN_PEER",
        "12D3KooWAxjuHoaQAoxEep2niP8Nao6kmdavxRSekoVg88wKy94v",
    )
    with AgentFMClient(gateway_url="http://127.0.0.1:8080") as client:
        try:
            worker = client.workers.get(KNOWN_PEER)
        except WorkerNotFoundError:
            print(f"peer {KNOWN_PEER[:12]}... not in current telemetry")
            return
        print(
            f"  agent={worker.name!r}  model={worker.model!r}  "
            f"status={worker.status}  load_ratio={worker.load_ratio:.2f}"
        )


def pattern_3_pin_without_list_call() -> None:
    """Skip discovery entirely — dispatch directly to a known peer_id.

    Lowest-overhead path. The gateway dials the peer via ``dialOmni``
    (peerstore → DHT → circuit-relay) without any workers.list() round-trip.
    Errors surface as typed AgentFMError subclasses:

      WorkerUnreachableError — gateway couldn't dial the peer
      WorkerStreamError      — libp2p stream failed mid-task
      MeshOverloadedError    — peer is at MaxTasks
    """
    KNOWN_PEER = os.environ.get(
        "AGENTFM_PIN_PEER",
        "12D3KooWAxjuHoaQAoxEep2niP8Nao6kmdavxRSekoVg88wKy94v",
    )
    with AgentFMClient(gateway_url="http://127.0.0.1:8080") as client:
        for chunk in client.tasks.stream(
            worker_id=KNOWN_PEER,
            prompt="Stream a 50-word welcome message.",
        ):
            if chunk.kind == "text":
                sys.stdout.write(chunk.text)
                sys.stdout.flush()
        sys.stdout.write("\n")


def main() -> None:
    print("=" * 70)
    print("Pattern 1 — browse all workers, pick by index")
    print("=" * 70)
    pattern_1_browse_and_pick()

    print("\n" + "=" * 70)
    print("Pattern 2 — fetch a known peer by ID (workers.get)")
    print("=" * 70)
    pattern_2_get_one_by_id()

    print("\n" + "=" * 70)
    print("Pattern 3 — pin to a peer_id without a list call (zero round-trips)")
    print("=" * 70)
    pattern_3_pin_without_list_call()


if __name__ == "__main__":
    main()

"""Spin up an ephemeral ``agentfm -mode api`` gateway for the lifetime of a context."""

from __future__ import annotations

from agentfm import AgentFMClient, LocalMeshGateway


def main() -> None:
    with LocalMeshGateway(port=8080, debug=False) as gw, AgentFMClient(gateway_url=gw.url) as client:
        workers = client.workers.list(wait_for_workers=1, poll_timeout=30.0)
        print(f"found {len(workers)} worker(s) on the mesh")
        for w in workers:
            print(f"  - {w.peer_id[:12]}... {w.name!r}")


if __name__ == "__main__":
    main()

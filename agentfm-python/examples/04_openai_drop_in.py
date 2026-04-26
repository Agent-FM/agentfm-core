"""Drop-in replacement for the OpenAI Python SDK against an AgentFM mesh.

Show both the typed AgentFM client (preferred — peer_id pinning, agentfm_*
fields) and the official ``openai`` SDK pointed at the same gateway.
"""

from __future__ import annotations

from agentfm import AgentFMClient


def with_agentfm_sdk() -> None:
    print("--- via AgentFM SDK ---")
    with AgentFMClient() as client:
        listing = client.openai.models.list()
        if not listing.data:
            print("  no models on mesh")
            return
        first = listing.data[0]
        print(
            f"  picked id={first.id[:12]}... "
            f"({first.agentfm_name!r} on {first.agentfm_engine!r})"
        )
        resp = client.openai.chat.completions.create(
            model=first.id,  # peer_id pin-routes
            messages=[{"role": "user", "content": "Say hi in 3 words."}],
        )
        print("  reply:", resp.choices[0].message.content)  # type: ignore[union-attr]


def with_openai_sdk() -> None:
    """Alternative: use the official ``openai`` library directly.

    Run ``pip install openai`` first; this isn't an AgentFM dependency.
    """
    try:
        from openai import OpenAI
    except ImportError:
        print("openai package not installed; skipping (pip install openai)")
        return
    print("--- via official openai SDK ---")
    client = OpenAI(base_url="http://127.0.0.1:8080/v1", api_key="anything")
    listing = client.models.list()
    if not listing.data:
        print("  no models on mesh")
        return
    first = listing.data[0]
    resp = client.chat.completions.create(
        model=first.id,
        messages=[{"role": "user", "content": "Say hi in 3 words."}],
    )
    print("  reply:", resp.choices[0].message.content)


if __name__ == "__main__":
    with_agentfm_sdk()
    with_openai_sdk()

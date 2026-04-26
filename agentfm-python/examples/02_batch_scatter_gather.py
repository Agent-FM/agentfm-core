"""Distribute many prompts across workers running the same model."""

from __future__ import annotations

from agentfm import AgentFMClient


def main() -> None:
    prompts = [
        f"Summarize the value of factor {i} in 2 sentences." for i in range(20)
    ]
    with AgentFMClient() as client:
        results = client.tasks.scatter_by_model(
            prompts,
            model="llama3.2",
            max_concurrency=4,
        )
    succeeded = [r for r in results if r.status == "success"]
    failed = [r for r in results if r.status == "failed"]
    print(f"{len(succeeded)} succeeded, {len(failed)} failed")
    for r in failed:
        print(f"  failed prompt: {r.prompt[:50]!r}  reason={r.error!r}")


if __name__ == "__main__":
    main()

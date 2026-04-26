"""``agentfm-py`` CLI — quick interactive testing of the SDK against a gateway.

Stdlib argparse only. This is a demo / debugging tool, not a replacement for
the Go ``agentfm`` binary.

Subcommands:

    agentfm-py ping                           Liveness check
    agentfm-py workers                        Listing of /api/workers
    agentfm-py models                         Listing of /v1/models
    agentfm-py chat --peer <id> --prompt ...  One-shot chat completion
"""

from __future__ import annotations

import argparse
import json
import sys
from typing import Any

from . import __version__
from .client import DEFAULT_GATEWAY, AgentFMClient
from .exceptions import AgentFMError
from .openai.models import ChatCompletion, ChatCompletionChunk


def _emit_json(obj: Any) -> None:
    json.dump(obj, sys.stdout, indent=2, default=str)
    sys.stdout.write("\n")


def _cmd_ping(args: argparse.Namespace) -> int:
    with AgentFMClient(gateway_url=args.gateway, timeout=args.timeout) as client:
        ok = client.ping()
    print("ok" if ok else "unreachable", file=sys.stdout if ok else sys.stderr)
    return 0 if ok else 1


def _cmd_workers(args: argparse.Namespace) -> int:
    with AgentFMClient(gateway_url=args.gateway, timeout=args.timeout) as client:
        workers = client.workers.list(model=args.model, available_only=args.available)
    _emit_json([w.model_dump(mode="json") for w in workers])
    return 0


def _cmd_models(args: argparse.Namespace) -> int:
    with AgentFMClient(gateway_url=args.gateway, timeout=args.timeout) as client:
        listing = client.openai.models.list()
    _emit_json(listing.model_dump(mode="json"))
    return 0


def _cmd_chat(args: argparse.Namespace) -> int:
    with AgentFMClient(gateway_url=args.gateway, timeout=args.timeout) as client:
        if args.stream:
            stream = client.openai.chat.completions.create(
                model=args.peer,
                messages=[{"role": "user", "content": args.prompt}],
                stream=True,
            )
            for chunk in stream:
                assert isinstance(chunk, ChatCompletionChunk)
                if chunk.choices and chunk.choices[0].delta.content:
                    sys.stdout.write(chunk.choices[0].delta.content)
                    sys.stdout.flush()
            sys.stdout.write("\n")
        else:
            resp = client.openai.chat.completions.create(
                model=args.peer,
                messages=[{"role": "user", "content": args.prompt}],
            )
            assert isinstance(resp, ChatCompletion)
            print(resp.choices[0].message.content if resp.choices else "")
    return 0


def _build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="agentfm-py",
        description="Interactive CLI for the AgentFM Python SDK.",
    )
    p.add_argument("--version", action="version", version=f"agentfm-py {__version__}")
    # Global flags live on the top parser so they can appear BEFORE the subcommand:
    #   agentfm-py --gateway http://... ping
    p.add_argument(
        "--gateway",
        default=DEFAULT_GATEWAY,
        help=f"Gateway URL (default {DEFAULT_GATEWAY})",
    )
    p.add_argument("--timeout", type=float, default=30.0, help="HTTP timeout in seconds")
    sub = p.add_subparsers(dest="command", required=True)

    ping = sub.add_parser("ping", help="Check whether the gateway is reachable")
    ping.set_defaults(func=_cmd_ping)

    workers = sub.add_parser("workers", help="List all workers on the mesh")
    workers.add_argument("--model", help="Filter by model engine")
    workers.add_argument("--available", action="store_true", help="Only show AVAILABLE workers")
    workers.set_defaults(func=_cmd_workers)

    models = sub.add_parser("models", help="GET /v1/models (peer-id-keyed listing)")
    models.set_defaults(func=_cmd_models)

    chat = sub.add_parser("chat", help="One-shot /v1/chat/completions")
    chat.add_argument("--peer", required=True, help="Peer ID (preferred) or model name")
    chat.add_argument("--prompt", required=True, help="User prompt")
    chat.add_argument("--stream", action="store_true", help="Stream as SSE deltas")
    chat.set_defaults(func=_cmd_chat)

    return p


def main(argv: list[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    try:
        return int(args.func(args))
    except AgentFMError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

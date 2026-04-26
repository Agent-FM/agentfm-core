"""v0.1 -> v0.2 deprecation shims.

Defined as a mixin (:class:`LegacyAPIMixin`) inherited by
:class:`agentfm.AgentFMClient` so the deprecated method names are visible to
type checkers and IDE autocomplete. Slated for removal in v0.4.
"""

from __future__ import annotations

import warnings
from pathlib import Path
from typing import TYPE_CHECKING, Any

from .exceptions import WorkerNotFoundError
from .models import WorkerProfile

if TYPE_CHECKING:
    from .client import _TasksNamespace, _WorkersNamespace


class LegacyAPIMixin:
    """Inherited by :class:`AgentFMClient` to expose v0.1 method names.

    Each method emits :class:`DeprecationWarning` and forwards to the v0.2
    canonical surface. Type-checked via the ``TYPE_CHECKING`` shadow attrs
    below, which describe the structural contract this mixin needs from the
    concrete client.
    """

    if TYPE_CHECKING:
        @property
        def workers(self) -> _WorkersNamespace: ...
        @property
        def tasks(self) -> _TasksNamespace: ...

    def discover_workers(
        self,
        models: list[str] | None = None,
        wait_for_workers: int = 0,
        poll_timeout: int = 15,
    ) -> list[WorkerProfile]:
        """Deprecated: use :meth:`AgentFMClient.workers.list`."""
        warnings.warn(
            "discover_workers() is deprecated; use client.workers.list(model=...) instead",
            DeprecationWarning,
            stacklevel=2,
        )
        if models and len(models) > 1:
            seen: dict[str, WorkerProfile] = {}
            for m in models:
                for w in self.workers.list(
                    model=m,
                    wait_for_workers=wait_for_workers,
                    poll_timeout=float(poll_timeout),
                ):
                    seen[w.peer_id] = w
            return list(seen.values())
        model = models[0] if models else None
        return self.workers.list(
            model=model,
            wait_for_workers=wait_for_workers,
            poll_timeout=float(poll_timeout),
        )

    def execute_task(
        self, worker_id: str, prompt: str, silent: bool = False
    ) -> list[Path]:
        """Deprecated: use :meth:`AgentFMClient.tasks.run` or :meth:`tasks.stream`."""
        warnings.warn(
            "execute_task() is deprecated; use client.tasks.run(...) for a TaskResult "
            "or client.tasks.stream(...) for an iterator",
            DeprecationWarning,
            stacklevel=2,
        )
        result = self.tasks.run(worker_id=worker_id, prompt=prompt)
        if not silent:
            print(result.text)
        return result.artifacts

    def batch_execute(
        self, prompts: list[str], models: list[str] | None = None
    ) -> list[dict[str, Any]]:
        """Deprecated: use :meth:`AgentFMClient.tasks.scatter` / :meth:`scatter_by_model`."""
        warnings.warn(
            "batch_execute() is deprecated; use client.tasks.scatter(...) "
            "or client.tasks.scatter_by_model(...)",
            DeprecationWarning,
            stacklevel=2,
        )
        if models:
            results = self.tasks.scatter_by_model(prompts, model=models[0])
        else:
            workers = self.workers.list(available_only=True)
            if not workers:
                raise WorkerNotFoundError("no workers available")
            results = self.tasks.scatter(prompts, peer_ids=[w.peer_id for w in workers])
        return [r.model_dump(mode="json") for r in results]

    def submit_async_task(
        self, worker_id: str, prompt: str, webhook_url: str
    ) -> str:
        """Deprecated: use :meth:`AgentFMClient.tasks.submit_async`."""
        warnings.warn(
            "submit_async_task() is deprecated; use client.tasks.submit_async(...) "
            "(returns an AsyncTaskAck object)",
            DeprecationWarning,
            stacklevel=2,
        )
        return self.tasks.submit_async(
            worker_id=worker_id, prompt=prompt, webhook_url=webhook_url
        ).task_id


__all__ = ["LegacyAPIMixin"]

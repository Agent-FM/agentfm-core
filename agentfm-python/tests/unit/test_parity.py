"""Sync ↔ async parity introspection.

The two clients must stay symmetric: every public method on
:class:`_TasksNamespace` must exist on :class:`_AsyncTasksNamespace` with the
same kwarg names (modulo `async`/`await` semantics). Same for
:class:`_WorkersNamespace`. Catches drift like the historical
``scatter_by_model.pick`` kwarg only being on sync.
"""

from __future__ import annotations

import inspect

from agentfm.async_client import _AsyncTasksNamespace, _AsyncWorkersNamespace
from agentfm.client import _TasksNamespace, _WorkersNamespace


def _public_methods(cls: type) -> dict[str, inspect.Signature]:
    return {
        name: inspect.signature(getattr(cls, name))
        for name, member in inspect.getmembers(cls, predicate=inspect.isfunction)
        if not name.startswith("_")
    }


def _kwargs_only(sig: inspect.Signature) -> set[str]:
    return {
        name
        for name, p in sig.parameters.items()
        if p.kind in (p.KEYWORD_ONLY, p.POSITIONAL_OR_KEYWORD) and name != "self"
    }


def test_tasks_namespace_method_set_matches():
    sync_methods = _public_methods(_TasksNamespace)
    async_methods = _public_methods(_AsyncTasksNamespace)
    assert sync_methods.keys() == async_methods.keys(), (
        f"sync-only: {sync_methods.keys() - async_methods.keys()}; "
        f"async-only: {async_methods.keys() - sync_methods.keys()}"
    )


def test_workers_namespace_method_set_matches():
    sync_methods = _public_methods(_WorkersNamespace)
    async_methods = _public_methods(_AsyncWorkersNamespace)
    assert sync_methods.keys() == async_methods.keys()


def test_tasks_namespace_kwargs_match_per_method():
    sync_methods = _public_methods(_TasksNamespace)
    async_methods = _public_methods(_AsyncTasksNamespace)
    for name in sync_methods:
        sync_kw = _kwargs_only(sync_methods[name])
        async_kw = _kwargs_only(async_methods[name])
        assert sync_kw == async_kw, (
            f"method={name!r} kwargs drift: sync={sync_kw} async={async_kw}"
        )


def test_workers_namespace_kwargs_match_per_method():
    sync_methods = _public_methods(_WorkersNamespace)
    async_methods = _public_methods(_AsyncWorkersNamespace)
    for name in sync_methods:
        sync_kw = _kwargs_only(sync_methods[name])
        async_kw = _kwargs_only(async_methods[name])
        assert sync_kw == async_kw, (
            f"method={name!r} kwargs drift: sync={sync_kw} async={async_kw}"
        )

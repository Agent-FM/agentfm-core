"""Resource base classes that absorb the try/except + retry + parse boilerplate.

Every namespace class (workers, tasks, openai.chat.completions, etc.) inherits
from :class:`SyncResource` or :class:`AsyncResource`. Each endpoint method then
becomes a one-line dispatch instead of the previous 5-line dance:

    # Before
    try:
        r = retry_sync(self._client._http.post, "/v1/chat/completions", json=body, retries=...)
    except (httpx.ConnectError, httpx.ReadError) as exc:
        raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc
    raise_for_response(r)
    return ChatCompletion.model_validate(r.json())

    # After
    return self._post("/v1/chat/completions", body=body, parse=ChatCompletion)
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, TypeVar

import httpx
from pydantic import BaseModel

from .._transport import (
    raise_for_response,
    retry_async,
    retry_sync,
    wrap_connection_error,
)

if TYPE_CHECKING:

    from ..async_client import AsyncAgentFMClient
    from ..client import AgentFMClient

T = TypeVar("T", bound=BaseModel)


class SyncResource:
    """Base for namespaces attached to ``AgentFMClient``."""

    def __init__(self, client: AgentFMClient) -> None:
        self._client = client

    # -- request helpers ----------------------------------------------------

    def _request(
        self,
        method: str,
        path: str,
        *,
        json: Any = None,
        params: dict[str, Any] | None = None,
    ) -> httpx.Response:
        try:
            response = retry_sync(
                self._client._http.request,
                method,
                path,
                json=json,
                params=params,
                retries=self._client.retries,
            )
        except (httpx.ConnectError, httpx.ReadError) as exc:
            raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc
        raise_for_response(response)
        return response

    def _get(self, path: str, *, parse: type[T], params: dict[str, Any] | None = None) -> T:
        return parse.model_validate(self._request("GET", path, params=params).json())

    def _post(self, path: str, *, body: Any, parse: type[T]) -> T:
        return parse.model_validate(self._request("POST", path, json=body).json())

    def _post_text(self, path: str, *, body: Any) -> httpx.Response:
        """POST and return the raw response (for streaming endpoints)."""
        return self._request("POST", path, json=body)


class AsyncResource:
    """Base for namespaces attached to ``AsyncAgentFMClient``."""

    def __init__(self, client: AsyncAgentFMClient) -> None:
        self._client = client

    async def _request(
        self,
        method: str,
        path: str,
        *,
        json: Any = None,
        params: dict[str, Any] | None = None,
    ) -> httpx.Response:
        try:
            response = await retry_async(
                self._client._http.request,
                method,
                path,
                json=json,
                params=params,
                retries=self._client.retries,
            )
        except (httpx.ConnectError, httpx.ReadError) as exc:
            raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc
        raise_for_response(response)
        return response

    async def _get(
        self, path: str, *, parse: type[T], params: dict[str, Any] | None = None
    ) -> T:
        r = await self._request("GET", path, params=params)
        return parse.model_validate(r.json())

    async def _post(self, path: str, *, body: Any, parse: type[T]) -> T:
        r = await self._request("POST", path, json=body)
        return parse.model_validate(r.json())


__all__ = ["AsyncResource", "SyncResource"]

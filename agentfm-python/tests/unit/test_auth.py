"""Bearer-token auth: api_key constructor + AGENTFM_API_KEY env fallback.

Mirrors the Go-side auth contract on the gateway. Tests cover:
* explicit api_key wins over AGENTFM_API_KEY env var
* AGENTFM_API_KEY fallback when the explicit arg is omitted
* no header at all when both are unset (solo-dev mode)
* AuthenticationError envelope mapping (`unauthorized`, `invalid_api_key`)
* with_options inherits api_key
"""

from __future__ import annotations

import pytest
import respx
from httpx import Response

from agentfm import AgentFMClient, AsyncAgentFMClient, AuthenticationError
from agentfm.exceptions import AgentFMError, from_envelope

GATEWAY = "http://test-gateway"


# ---------------------------------------------------------------------------
# Constructor + env-fallback precedence
# ---------------------------------------------------------------------------


def test_explicit_api_key_wins_over_env(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("AGENTFM_API_KEY", "from-env")
    c = AgentFMClient(gateway_url=GATEWAY, api_key="from-arg")
    try:
        assert c.api_key == "from-arg"
        assert c._http.headers["Authorization"] == "Bearer from-arg"
    finally:
        c.close()


def test_env_fallback_when_arg_omitted(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("AGENTFM_API_KEY", "from-env")
    c = AgentFMClient(gateway_url=GATEWAY)
    try:
        assert c.api_key == "from-env"
        assert c._http.headers["Authorization"] == "Bearer from-env"
    finally:
        c.close()


def test_no_header_when_both_unset(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    c = AgentFMClient(gateway_url=GATEWAY)
    try:
        assert c.api_key is None
        assert "authorization" not in {k.lower() for k in c._http.headers}
    finally:
        c.close()


def test_empty_string_env_treated_as_unset(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("AGENTFM_API_KEY", "")
    c = AgentFMClient(gateway_url=GATEWAY)
    try:
        assert c.api_key is None
    finally:
        c.close()


def test_async_client_mirrors_sync_precedence(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("AGENTFM_API_KEY", "from-env-async")
    c = AsyncAgentFMClient(gateway_url=GATEWAY, api_key="from-arg-async")
    assert c.api_key == "from-arg-async"
    assert c._http.headers["Authorization"] == "Bearer from-arg-async"


def test_async_client_env_fallback(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("AGENTFM_API_KEY", "from-env-async")
    c = AsyncAgentFMClient(gateway_url=GATEWAY)
    assert c.api_key == "from-env-async"


def test_async_client_no_header_when_unset(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    c = AsyncAgentFMClient(gateway_url=GATEWAY)
    assert c.api_key is None
    assert "authorization" not in {k.lower() for k in c._http.headers}


# ---------------------------------------------------------------------------
# with_options inherits api_key (PR 5 will add an explicit override kwarg)
# ---------------------------------------------------------------------------


def test_with_options_inherits_api_key(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    base = AgentFMClient(gateway_url=GATEWAY, api_key="parent-key")
    derived = base.with_options(retries=5)
    try:
        assert derived.api_key == "parent-key"
        assert derived._http.headers["Authorization"] == "Bearer parent-key"
    finally:
        base.close()
        derived.close()


def test_with_options_async_inherits_api_key(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    base = AsyncAgentFMClient(gateway_url=GATEWAY, api_key="parent-key")
    derived = base.with_options(retries=7)
    assert derived.api_key == "parent-key"


# ---------------------------------------------------------------------------
# Envelope -> AuthenticationError mapping
# ---------------------------------------------------------------------------


@pytest.mark.parametrize("code", ["unauthorized", "invalid_api_key"])
def test_auth_codes_map_to_authentication_error(code: str):
    exc = from_envelope(
        {"error": {"code": code, "message": "boom", "type": "invalid_request_error"}},
        status=401,
    )
    assert isinstance(exc, AuthenticationError)
    assert isinstance(exc, AgentFMError)
    assert exc.code == code
    assert exc.status == 401


# ---------------------------------------------------------------------------
# End-to-end: respx-mocked round-trip
# ---------------------------------------------------------------------------


def test_authorization_header_sent_on_request(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        route = router.get("/api/workers").respond(
            json={"success": True, "agents": []}
        )
        with AgentFMClient(gateway_url=GATEWAY, api_key="round-trip-key") as c:
            c.workers.list()
        sent = route.calls.last.request
        assert sent.headers["Authorization"] == "Bearer round-trip-key"


def test_no_authorization_header_when_api_key_unset(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        route = router.get("/api/workers").respond(
            json={"success": True, "agents": []}
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            c.workers.list()
        sent = route.calls.last.request
        assert "authorization" not in {k.lower() for k in sent.headers}


def test_401_envelope_raises_authentication_error(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/api/workers").mock(
            return_value=Response(
                401,
                json={
                    "error": {
                        "code": "invalid_api_key",
                        "message": "API key is invalid",
                        "type": "invalid_request_error",
                    }
                },
            )
        )
        with AgentFMClient(gateway_url=GATEWAY, api_key="wrong-key") as c:
            with pytest.raises(AuthenticationError) as ei:
                c.workers.list()
            assert ei.value.code == "invalid_api_key"
            assert ei.value.status == 401


async def test_async_401_envelope_raises_authentication_error(
    monkeypatch: pytest.MonkeyPatch,
):
    monkeypatch.delenv("AGENTFM_API_KEY", raising=False)
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/api/workers").mock(
            return_value=Response(
                401,
                json={
                    "error": {
                        "code": "unauthorized",
                        "message": "missing bearer token",
                        "type": "invalid_request_error",
                    }
                },
            )
        )
        async with AsyncAgentFMClient(gateway_url=GATEWAY) as c:
            with pytest.raises(AuthenticationError) as ei:
                await c.workers.list()
            assert ei.value.code == "unauthorized"

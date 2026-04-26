"""Verify ``with_options`` returns a fresh client with overrides applied."""

from __future__ import annotations

import httpx

from agentfm import AgentFMClient, AsyncAgentFMClient


def test_with_options_overrides_retries():
    base = AgentFMClient(gateway_url="http://x:8080", retries=2)
    derived = base.with_options(retries=5)
    try:
        assert derived is not base
        assert derived.retries == 5
        assert base.retries == 2
        assert derived.gateway_url == base.gateway_url
    finally:
        base.close()
        derived.close()


def test_with_options_overrides_gateway_url():
    base = AgentFMClient(gateway_url="http://x:8080", retries=2)
    derived = base.with_options(gateway_url="http://y:9090")
    try:
        assert derived.gateway_url == "http://y:9090"
        assert base.gateway_url == "http://x:8080"
        assert derived.retries == base.retries
    finally:
        base.close()
        derived.close()


def test_with_options_overrides_timeout():
    base = AgentFMClient(gateway_url="http://x:8080", timeout=30.0)
    derived = base.with_options(timeout=httpx.Timeout(5.0))
    try:
        assert derived._http.timeout != base._http.timeout
    finally:
        base.close()
        derived.close()


def test_with_options_no_args_is_identity_clone():
    base = AgentFMClient(gateway_url="http://x:8080", retries=3)
    derived = base.with_options()
    try:
        assert derived is not base
        assert derived.gateway_url == base.gateway_url
        assert derived.retries == base.retries
        assert derived._http is not base._http
    finally:
        base.close()
        derived.close()


def test_with_options_inherits_unset_artifacts_dir(tmp_path):
    base = AgentFMClient(gateway_url="http://x:8080", artifacts_dir=tmp_path)
    derived = base.with_options(retries=10)
    try:
        assert derived.artifacts is not None
        assert derived.artifacts.watch_dir == tmp_path
    finally:
        base.close()
        derived.close()


def test_with_options_can_clear_artifacts_dir(tmp_path):
    base = AgentFMClient(gateway_url="http://x:8080", artifacts_dir=tmp_path)
    derived = base.with_options(artifacts_dir=None)
    try:
        assert derived.artifacts is None
        assert base.artifacts is not None
    finally:
        base.close()
        derived.close()


def test_with_options_async_client_overrides_retries():
    base = AsyncAgentFMClient(gateway_url="http://x:8080", retries=2)
    derived = base.with_options(retries=7)
    assert derived is not base
    assert derived.retries == 7
    assert base.retries == 2

from __future__ import annotations

import pytest

from agentfm.exceptions import (
    AgentFMError,
    GatewayInternalError,
    GatewayProtocolError,
    InvalidRequestError,
    MeshOverloadedError,
    ModelNotFoundError,
    WorkerStreamError,
    WorkerUnreachableError,
    from_envelope,
)


@pytest.mark.parametrize(
    "code,expected_cls",
    [
        ("model_not_found", ModelNotFoundError),
        ("mesh_overloaded", MeshOverloadedError),
        ("worker_unreachable", WorkerUnreachableError),
        ("worker_stream_failed", WorkerStreamError),
        ("model_required", InvalidRequestError),
        ("prompt_required", InvalidRequestError),
        ("unsupported_prompt_type", InvalidRequestError),
        ("invalid_request_error", InvalidRequestError),
        ("method_not_allowed", InvalidRequestError),
        ("internal_error", GatewayInternalError),
    ],
)
def test_envelope_codes_map_to_exception_classes(code: str, expected_cls: type):
    exc = from_envelope({"error": {"code": code, "message": "boom", "type": "x"}}, status=400)
    assert isinstance(exc, expected_cls)
    assert exc.code == code
    assert exc.message == "boom"
    assert exc.status == 400


def test_unknown_code_falls_back_to_base_class():
    exc = from_envelope({"error": {"code": "weird_new_code", "message": "?"}}, status=418)
    assert type(exc) is AgentFMError
    assert exc.code == "weird_new_code"


def test_missing_envelope_returns_protocol_error():
    exc = from_envelope({"not": "an error envelope"}, status=500)
    assert isinstance(exc, GatewayProtocolError)
    assert exc.status == 500


def test_non_dict_envelope_returns_protocol_error():
    exc = from_envelope({}, status=500)  # type: ignore[arg-type]
    assert isinstance(exc, GatewayProtocolError)


def test_internal_error_is_not_protocol_error():
    """Pre-fix, internal_error mapped to GatewayProtocolError, conflating
    well-formed server-bug envelopes with malformed responses. They route
    to different exception classes so callers can handle them separately."""
    exc = from_envelope(
        {"error": {"code": "internal_error", "message": "boom"}}, status=500
    )
    assert isinstance(exc, GatewayInternalError)
    assert not isinstance(exc, GatewayProtocolError), (
        "internal_error must NOT be GatewayProtocolError (different semantic)"
    )


def test_repr_is_informative():
    exc = ModelNotFoundError("nope", code="model_not_found", status=404)
    r = repr(exc)
    assert "ModelNotFoundError" in r
    assert "nope" in r
    assert "404" in r

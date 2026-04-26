from __future__ import annotations

import os
from pathlib import Path

import pytest

from agentfm.crypto import HEADER, KEY_HEX_LEN, SwarmKey


def test_generate_creates_valid_key():
    k = SwarmKey.generate()
    assert len(k.key_hex) == KEY_HEX_LEN
    assert all(c in "0123456789abcdef" for c in k.key_hex)


def test_init_rejects_short_key():
    with pytest.raises(ValueError, match="64"):
        SwarmKey("abc")


def test_init_rejects_non_hex_key():
    with pytest.raises(ValueError, match="non-hex"):
        SwarmKey("z" * 64)


def test_init_rejects_non_string():
    with pytest.raises(TypeError):
        SwarmKey(b"\x00" * 32)  # type: ignore[arg-type]


def test_init_lowercases_input():
    k = SwarmKey("A" * 64)
    assert k.key_hex == "a" * 64


def test_to_text_uses_libp2p_format():
    k = SwarmKey("0" * 64)
    text = k.to_text()
    assert text.startswith(HEADER)
    assert text.endswith("0" * 64 + "\n")


def test_save_writes_file_with_correct_format(tmp_path: Path):
    k = SwarmKey("a" * 64)
    out = k.save(tmp_path / "swarm.key")
    assert out.exists()
    text = out.read_text(encoding="utf-8")
    assert text == k.to_text()


@pytest.mark.skipif(os.name != "posix", reason="POSIX-only chmod check")
def test_save_sets_strict_permissions(tmp_path: Path):
    k = SwarmKey("a" * 64)
    p = k.save(tmp_path / "swarm.key")
    mode = p.stat().st_mode & 0o777
    assert mode == 0o600


def test_load_round_trip(tmp_path: Path):
    k = SwarmKey.generate()
    p = k.save(tmp_path / "k")
    loaded = SwarmKey.load(p)
    assert loaded == k
    assert hash(loaded) == hash(k)


def test_load_missing_raises():
    with pytest.raises(FileNotFoundError):
        SwarmKey.load("/no/such/file/please")


def test_from_string_rejects_bad_format():
    with pytest.raises(ValueError, match="invalid libp2p"):
        SwarmKey.from_string("not a key file at all")


def test_repr_truncates_key():
    k = SwarmKey("0123456789abcdef" * 4)
    assert "..." in repr(k)
    assert "0123456789abcdef" * 4 not in repr(k)

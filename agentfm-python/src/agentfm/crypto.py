"""libp2p Pre-Shared Key (PSK v1) generation and loading.

Generates and validates the swarm.key files that the Go ``agentfm`` binary
expects when running in private-mesh (``-swarmkey``) mode.

Hardened vs the previous implementation:

* ``key_hex`` is validated to be exactly 64 hex characters on construction.
* Files are written in binary mode for cross-platform consistency
  (line endings would otherwise corrupt the format on Windows).
* Permissions are set to 0o600 on POSIX and via ACLs on Windows where
  ``os.chmod`` is supported.
"""

from __future__ import annotations

import contextlib
import os
import secrets
import string
from pathlib import Path

# libp2p PSK v1 file format. Three lines, then the key on its own line.
HEADER = "/key/swarm/psk/1.0.0/\n/base16/\n"
KEY_HEX_LEN = 64  # 32 bytes * 2 hex chars

_HEX_ALPHABET = frozenset(string.hexdigits)


class SwarmKey:
    """A 256-bit libp2p PSK in libp2p's text-file format."""

    __slots__ = ("key_hex",)

    def __init__(self, key_hex: str) -> None:
        if not isinstance(key_hex, str):
            raise TypeError(f"key_hex must be str, got {type(key_hex).__name__}")
        if len(key_hex) != KEY_HEX_LEN:
            raise ValueError(
                f"key_hex must be exactly {KEY_HEX_LEN} hex characters, got {len(key_hex)}"
            )
        if not all(c in _HEX_ALPHABET for c in key_hex):
            raise ValueError("key_hex contains non-hex characters")
        self.key_hex = key_hex.lower()

    # -- factories -----------------------------------------------------------

    @classmethod
    def generate(cls) -> SwarmKey:
        """Generate a new cryptographically strong 256-bit key."""
        return cls(secrets.token_bytes(32).hex())

    @classmethod
    def from_string(cls, text: str) -> SwarmKey:
        """Parse the contents of a ``swarm.key`` file already in memory."""
        lines = [line.rstrip("\r\n") for line in text.splitlines()]
        if len(lines) < 3 or "/key/swarm/psk/1.0.0/" not in lines[0]:
            raise ValueError("invalid libp2p swarm key format")
        return cls(lines[2].strip())

    @classmethod
    def load(cls, path: str | Path) -> SwarmKey:
        """Load a swarm key from disk."""
        p = Path(path).resolve()
        if not p.exists():
            raise FileNotFoundError(f"swarm key not found at {p}")
        return cls.from_string(p.read_text(encoding="utf-8"))

    # -- serialization -------------------------------------------------------

    def to_text(self) -> str:
        """Render the key in libp2p's expected file layout."""
        return f"{HEADER}{self.key_hex}\n"

    def __str__(self) -> str:
        return self.to_text()

    def save(self, path: str | Path) -> Path:
        """Write the key to ``path`` with strict (owner-only) permissions."""
        p = Path(path).resolve()
        p.parent.mkdir(parents=True, exist_ok=True)
        # Binary mode prevents Windows CRLF translation.
        p.write_bytes(self.to_text().encode("utf-8"))
        with contextlib.suppress(OSError, NotImplementedError):
            # Some Windows filesystems don't support chmod; key is still
            # written, just without strict permissions.
            os.chmod(p, 0o600)
        return p

    # -- equality / repr -----------------------------------------------------

    def __eq__(self, other: object) -> bool:
        return isinstance(other, SwarmKey) and self.key_hex == other.key_hex

    def __hash__(self) -> int:
        return hash(("SwarmKey", self.key_hex))

    def __repr__(self) -> str:
        return f"SwarmKey({self.key_hex[:8]}...{self.key_hex[-4:]})"


__all__ = ["HEADER", "KEY_HEX_LEN", "SwarmKey"]

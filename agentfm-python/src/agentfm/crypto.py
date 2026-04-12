import os
import secrets
from pathlib import Path

class SwarmKey:
    """
    Generates and manages libp2p Private Swarm Keys (PSK v1).
    Used to create isolated, encrypted P2P mesh networks on the fly.
    """
    HEADER = "/key/swarm/psk/1.0.0/\n/bin/\n"

    def __init__(self, key_hex: str):
        """Initialize with an existing 64-character hex key."""
        self.key_hex = key_hex

    @classmethod
    def generate(cls) -> "SwarmKey":
        """Generates a cryptographically secure 256-bit (32-byte) swarm key."""
        # 32 bytes = 64 hex characters
        key_bytes = secrets.token_bytes(32)
        return cls(key_bytes.hex())

    def __str__(self) -> str:
        """Formats the key exactly how the Go libp2p node expects it."""
        return f"{self.HEADER}{self.key_hex}\n"

    def save(self, filepath: str) -> Path:
        """Saves the swarm key to disk with strict Unix permissions."""
        path = Path(filepath).resolve()
        
        # Ensure the parent directory exists
        path.parent.mkdir(parents=True, exist_ok=True)
        
        with open(path, "w") as f:
            f.write(str(self))
            
        # Security best practice: lock down file permissions (read/write for owner only)
        if os.name == "posix":
            os.chmod(path, 0o600)
            
        return path

    @classmethod
    def load(cls, filepath: str) -> "SwarmKey":
        """Loads and validates an existing swarm key from disk."""
        path = Path(filepath).resolve()
        if not path.exists():
            raise FileNotFoundError(f"Swarm key not found at {path}")
        
        with open(path, "r") as f:
            lines = f.readlines()
        
        if len(lines) < 3 or "/key/swarm/psk/1.0.0/" not in lines[0]:
            raise ValueError(f"Invalid libp2p swarm key format in {filepath}")
            
        key_hex = lines[2].strip()
        return cls(key_hex)
package network

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"

	"github.com/libp2p/go-libp2p/core/pnet"
)

// GenerateSwarmKey creates a cryptographically secure 256-bit PSK for libp2p.
func GenerateSwarmKey(outputPath string) error {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return fmt.Errorf("failed to generate random bytes: %w", err)
	}

	encodedKey := hex.EncodeToString(key)
	// This specific header is strictly required by the libp2p pnet specification
	keyContent := fmt.Sprintf("/key/swarm/psk/1.0.0/\n/base16/\n%s\n", encodedKey)

	err = os.WriteFile(outputPath, []byte(keyContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write swarm key file: %w", err)
	}

	return nil
}

// LoadSwarmKey reads a PSK file and decodes it for libp2p consumption.
// Warns (does not refuse) when the file is group/world-readable; the spec
// is 0600 and a swarm.key checked into git or chmod 644'd is a real-world
// foot-shooting we should call out without bricking the node.
func LoadSwarmKey(path string) (pnet.PSK, error) {
	if info, err := os.Stat(path); err == nil {
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			slog.Warn("swarm key file is group/world readable; expected 0600",
				slog.String("path", path),
				slog.String("mode", fmt.Sprintf("%#o", mode)),
			)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open swarm key file: %w", err)
	}
	defer file.Close()

	psk, err := pnet.DecodeV1PSK(file)
	if err != nil {
		return nil, fmt.Errorf("could not decode swarm key: %w", err)
	}

	return psk, nil
}

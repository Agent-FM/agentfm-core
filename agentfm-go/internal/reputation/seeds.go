package reputation

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

//go:embed default_seeds.json
var defaultSeedsBytes []byte

// SeedsManifest is the on-disk shape for genesis-seeds.json (P5-2).
type SeedsManifest struct {
	Version   int          `json:"version"`
	UpdatedAt string       `json:"updated_at,omitempty"`
	Seeds     []SeedEntry  `json:"seeds"`
}

// SeedEntry is one curated entry in the manifest.
type SeedEntry struct {
	PeerID   string  `json:"peer_id"`
	Score    float64 `json:"score"`
	Operator string  `json:"operator,omitempty"`
	Note     string  `json:"note,omitempty"`
}

// ErrSeedsVersion is returned when the manifest version isn't
// understood by this build.
var ErrSeedsVersion = errors.New("reputation: unsupported seeds manifest version")

// LoadDefaultSeeds returns the bundled genesis-seeds.json as a Seed
// slice. Used when the operator hasn't supplied --genesis-seeds.
func LoadDefaultSeeds() ([]Seed, error) {
	return parseSeeds(defaultSeedsBytes)
}

// LoadSeedsFile parses the manifest at path. Empty path falls back
// to the bundled default. Errors surface to the caller — a missing
// or malformed seeds file is fatal at boot because EigenTrust
// without seeds converges to zero everywhere (silent loss of
// scoring is worse than failing loudly).
func LoadSeedsFile(path string) ([]Seed, error) {
	if path == "" {
		return LoadDefaultSeeds()
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reputation: read seeds: %w", err)
	}
	return parseSeeds(bs)
}

func parseSeeds(bs []byte) ([]Seed, error) {
	var m SeedsManifest
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil, fmt.Errorf("reputation: parse seeds: %w", err)
	}
	if m.Version != 1 {
		return nil, fmt.Errorf("%w: %d", ErrSeedsVersion, m.Version)
	}
	out := make([]Seed, 0, len(m.Seeds))
	for _, e := range m.Seeds {
		if e.PeerID == "" {
			continue
		}
		if e.Score < -1 || e.Score > 1 {
			continue
		}
		out = append(out, Seed{PeerID: e.PeerID, Score: e.Score})
	}
	return out, nil
}

package reputation_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agentfm/internal/reputation"
)

func osWriteFile(path string, body []byte) error {
	return os.WriteFile(path, body, 0o600)
}

func TestLoadDefaultSeeds_ParsesBundled(t *testing.T) {
	seeds, err := reputation.LoadDefaultSeeds()
	if err != nil {
		t.Fatalf("LoadDefaultSeeds: %v", err)
	}
	if len(seeds) == 0 {
		t.Fatal("bundled seeds parsed but list is empty")
	}
	// First seed should be the maintainers' lighthouse (score 1.0).
	if seeds[0].Score != 1.0 {
		t.Errorf("expected first seed score 1.0, got %v", seeds[0].Score)
	}
}

func TestLoadSeedsFile_EmptyFallsBackToDefault(t *testing.T) {
	seeds, err := reputation.LoadSeedsFile("")
	if err != nil {
		t.Fatalf("LoadSeedsFile(\"\"): %v", err)
	}
	if len(seeds) == 0 {
		t.Fatal("fallback should produce bundled seeds")
	}
}

func TestLoadSeedsFile_BadJSON(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.json")
	if err := writeFile(tmp, []byte("{not json")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := reputation.LoadSeedsFile(tmp)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadSeedsFile_BadVersion(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.json")
	if err := writeFile(tmp, []byte(`{"version": 42, "seeds": []}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := reputation.LoadSeedsFile(tmp)
	if !errors.Is(err, reputation.ErrSeedsVersion) {
		t.Fatalf("want ErrSeedsVersion, got %v", err)
	}
}

func TestLoadSeedsFile_FiltersInvalidEntries(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "mixed.json")
	if err := writeFile(tmp, []byte(`{
		"version": 1,
		"seeds": [
			{"peer_id": "good", "score": 0.5},
			{"peer_id": "", "score": 0.5},
			{"peer_id": "outOfRange", "score": 2.0}
		]
	}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	seeds, err := reputation.LoadSeedsFile(tmp)
	if err != nil {
		t.Fatalf("LoadSeedsFile: %v", err)
	}
	if len(seeds) != 1 || seeds[0].PeerID != "good" {
		t.Fatalf("filter failed; got %+v", seeds)
	}
}

func writeFile(path string, body []byte) error {
	return osWriteFile(path, body)
}

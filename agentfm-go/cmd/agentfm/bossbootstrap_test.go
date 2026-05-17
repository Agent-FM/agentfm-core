package main

import (
	"context"
	"testing"

	"agentfm/internal/reputation"
	"agentfm/test/testutil"
)

// TestBossSelfSeed_HasNonZeroWeight verifies that the boss's self-seed
// uses a base58-encoded peer ID so EigenTrust recognises it as a rater
// with non-zero voting weight. Before the fix, string(peer.ID) returned
// raw bytes that never matched the inbox's base58 rater IDs, so the
// boss's own ratings carried zero weight.
func TestBossSelfSeed_HasNonZeroWeight(t *testing.T) {
	host := testutil.NewHost(t)
	store := testutil.OpenTestStore(t)
	defer store.Close()

	seeds := []reputation.Seed{{PeerID: host.ID().String(), Score: 1.0}}
	engine := reputation.New(seeds, reputation.Config{})

	subject := testutil.NewHost(t).ID()
	testutil.AppendOwnRating(t, store, host, subject, -0.3, "test")

	if _, err := engine.Recompute(context.Background(), store); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if got := engine.Score(subject.String()); got >= 0 {
		t.Fatalf("expected negative score; got %v", got)
	}
}

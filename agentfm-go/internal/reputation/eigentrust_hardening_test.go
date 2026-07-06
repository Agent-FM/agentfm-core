package reputation_test

import (
	"context"
	"math"
	"testing"

	"agentfm/internal/reputation"
)

// M6: a peer that rates ITSELF must not amplify its own score. Without a
// rater!=subject guard, an already-trusted peer self-reinforces toward 1.0.
func TestEigenTrust_SelfRating_DoesNotAmplify(t *testing.T) {
	r := newRig(t)
	p := newPID(t)
	r.insertRating(p, p, 1.0, 1.0)
	eng := reputation.New([]reputation.Seed{{PeerID: p.String(), Score: 0.5}}, reputation.Config{})
	if _, err := eng.Recompute(context.Background(), r.store); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	got := eng.Score(p.String())
	if got > 0.5 {
		t.Fatalf("self-rating amplified score above seed: got %v, want <= 0.5 (self-edge must be ignored)", got)
	}
}

// M5: a rating with a non-finite (NaN/Inf) score must not propagate into
// the trust computation, or NaN comparisons silently bypass the trust gate.
func TestEigenTrust_NaNRating_DoesNotPoisonScore(t *testing.T) {
	r := newRig(t)
	seed := newPID(t)
	target := newPID(t)
	r.insertRating(seed, target, math.NaN(), 1.0)
	eng := reputation.New([]reputation.Seed{{PeerID: seed.String(), Score: 1.0}}, reputation.Config{})
	if _, err := eng.Recompute(context.Background(), r.store); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	got := eng.Score(target.String())
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("non-finite rating poisoned the score: got %v", got)
	}
}

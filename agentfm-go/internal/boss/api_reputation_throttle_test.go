package boss

import (
	"context"
	"testing"

	"agentfm/internal/ledger/store"
	"agentfm/test/testutil"
)

type countingEngine struct{ calls int }

func (c *countingEngine) Score(string) float64 { return 0 }
func (c *countingEngine) Recompute(context.Context, *store.Store) (float64, error) {
	c.calls++
	return 0, nil
}

// M4: a burst of fresh-on-read reputation requests must coalesce into a
// single recompute within the throttle window, not one O(ledger) pass
// per request (CPU DoS amplifier).
func TestReadRecompute_IsRateLimited(t *testing.T) {
	fake := &countingEngine{}
	b := &Boss{
		reputationEngine: fake,
		readStore:        testutil.OpenTestStore(t),
	}

	for i := 0; i < 50; i++ {
		b.recomputeThrottled(context.Background())
	}

	if fake.calls != 1 {
		t.Fatalf("expected 1 recompute within the throttle window, got %d", fake.calls)
	}
}

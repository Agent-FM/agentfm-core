package boss

import (
	"context"
	"testing"

	"agentfm/internal/types"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// alwaysEquivocatorLedger returns true from IsEquivocator for every peer.
type alwaysEquivocatorLedger struct{ stubLedger }

func (alwaysEquivocatorLedger) IsEquivocator(_ context.Context, _ []byte) (bool, error) {
	return true, nil
}

// newTestPeerID generates a fresh libp2p peer.ID for test use.
func newTestPeerID(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("peer id from key: %v", err)
	}
	return id
}

func TestCheckTrust_AllowsByDefault(t *testing.T) {
	b := &Boss{}
	// No ledger, no reputation engine → must allow.
	out := b.checkTrust(context.Background(), types.WorkerProfile{PeerID: "nopeer"})
	if !out.Allowed {
		t.Errorf("want allowed; got Reason=%q", out.Reason)
	}
}

func TestCheckTrust_NilLedgerSafe(t *testing.T) {
	// Nil ledger must not panic even when reputation engine is present.
	b := &Boss{reputationFloor: -0.5}
	b.SetReputationScoreForTest("somepeer", 0.0)
	out := b.checkTrust(context.Background(), types.WorkerProfile{PeerID: "somepeer"})
	if !out.Allowed {
		t.Errorf("want allowed; got Reason=%q", out.Reason)
	}
}

func TestCheckTrust_RefusesBelowFloor(t *testing.T) {
	b := &Boss{reputationFloor: -0.5}
	b.SetReputationScoreForTest("badpeer", -0.7)
	out := b.checkTrust(context.Background(), types.WorkerProfile{PeerID: "badpeer"})
	if out.Allowed {
		t.Error("want refused (score -0.7 < floor -0.5)")
	}
	if out.Reason != ErrReputationBelowFloor.Error() {
		t.Errorf("reason = %q, want %q", out.Reason, ErrReputationBelowFloor.Error())
	}
}

func TestCheckTrust_AllowsExactlyAtFloor(t *testing.T) {
	// Score == floor: strict less-than means allowed at exactly floor.
	b := &Boss{reputationFloor: -0.5}
	b.SetReputationScoreForTest("exactpeer", -0.5)
	out := b.checkTrust(context.Background(), types.WorkerProfile{PeerID: "exactpeer"})
	if !out.Allowed {
		t.Errorf("score == floor should be allowed (strict <); got Reason=%q", out.Reason)
	}
}

// TestCheckTrust_ExplicitZeroFloorRefusesNegativeScores pins the regression
// for the zero-sentinel collision. Previously, --reputation-floor=0 was
// silently converted to -1.0 (allow all) by an `if floor == 0` sentinel,
// so an operator setting `0` got the opposite of the intended behavior
// ("refuse anyone with negative reputation"). After the fix, the floor
// field is resolved once at construction time from a *float64 in Options,
// so 0 is treated as a literal value.
func TestCheckTrust_ExplicitZeroFloorRefusesNegativeScores(t *testing.T) {
	zero := 0.0
	b := NewWithOptions(nil, Options{ReputationFloor: &zero})
	b.SetReputationScoreForTest("negpeer", -0.1)

	out := b.checkTrust(context.Background(), types.WorkerProfile{PeerID: "negpeer"})
	if out.Allowed {
		t.Fatal("want refused: score -0.1 must be below explicit floor 0.0")
	}
	if out.Reason != ErrReputationBelowFloor.Error() {
		t.Errorf("reason = %q, want %q", out.Reason, ErrReputationBelowFloor.Error())
	}
}

// TestCheckTrust_NilFloorOptionDefaultsToAllowAll verifies that omitting
// ReputationFloor (the zero-value of *float64 is nil) defaults to -1.0
// (allow everything), preserving backwards-compatible Boss{} construction.
func TestCheckTrust_NilFloorOptionDefaultsToAllowAll(t *testing.T) {
	b := NewWithOptions(nil, Options{})
	b.SetReputationScoreForTest("negpeer", -0.99)

	out := b.checkTrust(context.Background(), types.WorkerProfile{PeerID: "negpeer"})
	if !out.Allowed {
		t.Fatalf("want allowed (nil floor → -1.0); got Reason=%q", out.Reason)
	}
}

func TestCheckTrust_RefusesEquivocator(t *testing.T) {
	b := &Boss{ledger: alwaysEquivocatorLedger{}}
	// Generate a real peer.ID so peer.Decode succeeds and the equivocator
	// path is exercised.
	pid := newTestPeerID(t)
	out := b.checkTrust(context.Background(), types.WorkerProfile{PeerID: pid.String()})
	if out.Allowed {
		t.Error("want refused for equivocator")
	}
	if out.Reason != ErrPeerIsEquivocator.Error() {
		t.Errorf("reason = %q, want %q", out.Reason, ErrPeerIsEquivocator.Error())
	}
}

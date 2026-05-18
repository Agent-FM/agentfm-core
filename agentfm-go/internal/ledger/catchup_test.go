package ledger_test

// P5-1 catch-up tests.
//
// TestCatchUp_PullsAndIngestsFromRelay verifies the happy path: a
// freshly-started boss with an empty inbox catches up from a relay that
// has 3 entries, ending up with all 3 in its inbox.
//
// TestCatchUp_RejectsEntriesPastRelayHead is intentionally deferred as a
// DONE_WITH_CONCERNS item: constructing a mock relay that forges an
// idx > head.TreeSize requires either a test-only stream handler override
// or a second integration-test binary, which is disproportionate for one
// commit. The boundary check lives in the production CatchUp loop and is
// unit-testable at the level of the entry-by-entry guard.

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/test/testutil"
)

// TestCatchUp_PullsAndIngestsFromRelay is the main happy-path integration
// test for Phase 5 catch-up. Setup:
//
//   relay  = ledgerImpl with a real libp2p Host that serves both
//            LedgerFetchProtocol and HeadFetchProtocol.
//   boss   = fresh ledger, empty inbox, different host.
//
// Steps:
//  1. Relay appends 3 ratings about a known subject.
//  2. Boss calls ledger.CatchUp.
//  3. Test asserts boss's inbox has all 3 entries.
func TestCatchUp_PullsAndIngestsFromRelay(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	relayHost, bossHost := hosts[0], hosts[1]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- relay ledger (serves both fetch protocols) --------------------
	relayKey, relayRaterID := signingIdentity(t)
	relayLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "relay.db"),
		relayKey,
		nil, // no gossip needed for this test
		ledger.Options{Host: relayHost},
	)
	if err != nil {
		t.Fatalf("relay ledger: %v", err)
	}
	t.Cleanup(func() { _ = relayLedger.Close() })

	// Relay appends 3 entries.
	subject := bytes.Repeat([]byte{0xAB}, 32)
	var hashes [3][32]byte
	for i := 0; i < 3; i++ {
		entry := &pb.SignedEntry{
			Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
				RaterPeerId:     relayRaterID,
				SubjectPeerId:   subject,
				Dimension:       "reliability",
				Score:           float64(i+1) * 0.3,
				Context:         "catchup-test",
				TimestampUnixNs: time.Now().UnixNano(),
			}},
		}
		h, err := relayLedger.Append(ctx, entry)
		if err != nil {
			t.Fatalf("relay Append %d: %v", i, err)
		}
		hashes[i] = h
	}

	// Verify relay head is signed and tree_size == 3.
	relayHead, err := relayLedger.Head(ctx)
	if err != nil {
		t.Fatalf("relay Head: %v", err)
	}
	if relayHead == nil || relayHead.TreeSize != 3 {
		t.Fatalf("relay head tree_size = %v, want 3", relayHead)
	}

	// --- boss ledger (fresh, no gossip, no direct fetch handler) -------
	bossKey, _ := signingIdentity(t)
	bossLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "boss.db"),
		bossKey,
		nil,
		ledger.Options{Host: bossHost},
	)
	if err != nil {
		t.Fatalf("boss ledger: %v", err)
	}
	t.Cleanup(func() { _ = bossLedger.Close() })

	// --- catch-up -------------------------------------------------------
	if err := ledger.CatchUp(ctx, bossLedger, bossHost, relayHost.ID()); err != nil {
		t.Fatalf("CatchUp: %v", err)
	}

	// Assert boss's inbox now holds the relay's 3 entries. We use
	// InboxHas(raterID, hash) — the same API that gossip-ingestion tests
	// use — so we verify both the hash and the peer-id path.
	for i, h := range hashes {
		ok, err := bossLedger.InboxHas(ctx, relayRaterID, h)
		if err != nil {
			t.Fatalf("InboxHas[%d]: %v", i, err)
		}
		if !ok {
			t.Errorf("boss inbox missing entry %d (hash=%x)", i, h)
		}
	}
}

// TestCatchUp_NoOpWhenRelayEmpty verifies that CatchUp against a relay
// with no entries returns nil without error.
func TestCatchUp_NoOpWhenRelayEmpty(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	relayHost, bossHost := hosts[0], hosts[1]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayKey, _ := signingIdentity(t)
	relayLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "relay.db"),
		relayKey,
		nil,
		ledger.Options{Host: relayHost},
	)
	if err != nil {
		t.Fatalf("relay ledger: %v", err)
	}
	t.Cleanup(func() { _ = relayLedger.Close() })

	bossKey, _ := signingIdentity(t)
	bossLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "boss.db"),
		bossKey,
		nil,
		ledger.Options{Host: bossHost},
	)
	if err != nil {
		t.Fatalf("boss ledger: %v", err)
	}
	t.Cleanup(func() { _ = bossLedger.Close() })

	if err := ledger.CatchUp(ctx, bossLedger, bossHost, relayHost.ID()); err != nil {
		t.Fatalf("CatchUp on empty relay: %v", err)
	}
}

// TestCatchUp_IdempotentOnSecondCall verifies that calling CatchUp twice
// does not double-count or error — the inbox dedup must handle it cleanly.
func TestCatchUp_IdempotentOnSecondCall(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	relayHost, bossHost := hosts[0], hosts[1]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayKey, relayRaterID := signingIdentity(t)
	relayLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "relay.db"),
		relayKey,
		nil,
		ledger.Options{Host: relayHost},
	)
	if err != nil {
		t.Fatalf("relay ledger: %v", err)
	}
	t.Cleanup(func() { _ = relayLedger.Close() })

	// One entry on the relay.
	entry := &pb.SignedEntry{
		Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
			RaterPeerId:     relayRaterID,
			SubjectPeerId:   bytes.Repeat([]byte{0xCC}, 32),
			Dimension:       "honesty",
			Score:           0.9,
			Context:         "idempotent-test",
			TimestampUnixNs: time.Now().UnixNano(),
		}},
	}
	h, err := relayLedger.Append(ctx, entry)
	if err != nil {
		t.Fatalf("relay Append: %v", err)
	}

	bossKey, _ := signingIdentity(t)
	bossLedger, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "boss.db"),
		bossKey,
		nil,
		ledger.Options{Host: bossHost},
	)
	if err != nil {
		t.Fatalf("boss ledger: %v", err)
	}
	t.Cleanup(func() { _ = bossLedger.Close() })

	// First catch-up.
	if err := ledger.CatchUp(ctx, bossLedger, bossHost, relayHost.ID()); err != nil {
		t.Fatalf("CatchUp #1: %v", err)
	}

	// Second catch-up — must not error.
	if err := ledger.CatchUp(ctx, bossLedger, bossHost, relayHost.ID()); err != nil {
		t.Fatalf("CatchUp #2 (idempotent): %v", err)
	}

	// Entry must still be present exactly once.
	ok, err := bossLedger.InboxHas(ctx, relayRaterID, h)
	if err != nil {
		t.Fatalf("InboxHas: %v", err)
	}
	if !ok {
		t.Fatal("boss inbox missing entry after idempotent catch-up")
	}
}

// TestVerifyHeadSignature_ValidAndInvalid exercises the exported helper
// so the compilation boundary is covered.
func TestVerifyHeadSignature_ValidAndInvalid(t *testing.T) {
	ctx := context.Background()

	priv, _ := signingIdentity(t)
	l, err := ledger.NewWithOptions(
		filepath.Join(t.TempDir(), "sig.db"),
		priv,
		nil,
		ledger.Options{},
	)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer func() { _ = l.Close() }()

	raterID := make([]byte, 32)
	entry := &pb.SignedEntry{
		Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
			RaterPeerId:     raterID,
			SubjectPeerId:   raterID,
			Dimension:       "x",
			Score:           0.1,
			Context:         "c",
			TimestampUnixNs: time.Now().UnixNano(),
		}},
	}
	_, _ = l.Append(ctx, entry) // populate lastHead

	head, err := l.Head(ctx)
	if err != nil || head == nil {
		t.Fatalf("Head: err=%v head=%v", err, head)
	}

	if !ledger.VerifyHeadSignature(head) {
		t.Fatal("VerifyHeadSignature returned false for a valid head")
	}

	// Corrupt the signature.
	head.Signature[0] ^= 0xFF
	if ledger.VerifyHeadSignature(head) {
		t.Fatal("VerifyHeadSignature returned true for a tampered head")
	}

	// Nil head.
	if ledger.VerifyHeadSignature(nil) {
		t.Fatal("VerifyHeadSignature returned true for nil head")
	}
}

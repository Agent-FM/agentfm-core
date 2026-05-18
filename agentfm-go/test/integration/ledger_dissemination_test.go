package integration

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	"agentfm/internal/network"
	pb "agentfm/internal/ledger/pb"
	"agentfm/test/testutil"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestLedger_TwoPeerDisseminationAndRestart is the P1-7 acceptance
// test: A and B run real libp2p hosts on 127.0.0.1, each with its own
// GossipSub and Ledger. A appends ten signed Ratings; B's inbox
// auto-ingests all ten via the FeedbackTopic. Both ledgers are then
// closed and reopened against the same SQLite files. A appends an
// eleventh entry — its prev_hash MUST chain to entry #10's hash,
// proving A's Merkle tree was rebuilt from the store; B receives the
// eleventh entry over gossip AND accepts it directly into the inbox
// (not orphaned), proving B's known-chain-head survived the restart.
//
// This is the integration counterpart to:
//   - internal/ledger/impl_test.go::TestAppend_SurvivesRestart_ChainContinues
//   - internal/ledger/impl_test.go::TestTwoPeer_BInboxIngestsAEntry
//
// Either alone covers half the story; this test stitches them together
// end-to-end so a regression in either the persistence path or the
// receive path is surfaced in one place.
func TestLedger_TwoPeerDisseminationAndRestart(t *testing.T) {
	const entryCount = 10

	dirA := t.TempDir()
	dirB := t.TempDir()
	dbPathA := filepath.Join(dirA, "ledger.db")
	dbPathB := filepath.Join(dirB, "ledger.db")

	// Stable identities across the restart — the production worker
	// loads the same Ed25519 key from worker_identity.key, and the
	// ledger's signatures + PeerID derivation must round-trip across
	// process restarts in real deployments.
	keyA := mintKey(t)
	keyB := mintKey(t)
	pidA, err := peer.IDFromPrivateKey(keyA)
	if err != nil {
		t.Fatalf("derive A peer id: %v", err)
	}

	// -------------------------------------------------------------------
	// Session 1: A appends ten entries; B's inbox ingests all ten.
	// -------------------------------------------------------------------
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hosts := testutil.NewConnectedMesh(t, 2)
	hostA, hostB := hosts[0], hosts[1]
	psA, psB := newPubSubPair(t, ctx, hostA, hostB)

	// B opens FIRST so its subscription is in place when A starts
	// publishing. The 800ms grace lets GossipSub's mesh-formation
	// heartbeat propagate B's subscription into A's view.
	ledgerB1, err := ledger.New(dbPathB, keyB, psB)
	if err != nil {
		t.Fatalf("session 1: ledger B: %v", err)
	}
	time.Sleep(800 * time.Millisecond)

	ledgerA1, err := ledger.New(dbPathA, keyA, psA)
	if err != nil {
		_ = ledgerB1.Close()
		t.Fatalf("session 1: ledger A: %v", err)
	}

	// Append ten entries to A's log. Capture the last hash so we can
	// assert prev-hash chain continuity across the restart in session 2.
	hashes := make([][32]byte, 0, entryCount)
	for i := 0; i < entryCount; i++ {
		e := freshRating(pidA, "honesty", float64(i)/10)
		h, err := ledgerA1.Append(ctx, e)
		if err != nil {
			_ = ledgerA1.Close()
			_ = ledgerB1.Close()
			t.Fatalf("session 1: Append #%d: %v", i, err)
		}
		hashes = append(hashes, h)
	}

	// Wait for B's auto-subscribe goroutine to ingest all ten into the
	// inbox. We poll the LAST hash — if it's present, dedup semantics
	// guarantee the earlier entries were already accepted (they had to
	// be, otherwise #10 would be sitting in the orphan queue).
	if err := waitForInbox(ctx, ledgerB1, []byte(pidA), hashes[entryCount-1], 5*time.Second); err != nil {
		_ = ledgerA1.Close()
		_ = ledgerB1.Close()
		t.Fatalf("session 1: B did not ingest entry #%d in time: %v", entryCount, err)
	}

	// Pre-close sanity: every entry from A is in B's inbox.
	for i, h := range hashes {
		ok, err := ledgerB1.InboxHas(ctx, []byte(pidA), h)
		if err != nil {
			t.Fatalf("session 1: InboxHas: %v", err)
		}
		if !ok {
			t.Fatalf("session 1: entry #%d missing from B's inbox", i)
		}
	}

	if err := ledgerA1.Close(); err != nil {
		t.Fatalf("session 1: close A: %v", err)
	}
	if err := ledgerB1.Close(); err != nil {
		t.Fatalf("session 1: close B: %v", err)
	}

	// -------------------------------------------------------------------
	// Session 2: Reopen both ledgers; A appends #11; B receives it.
	// -------------------------------------------------------------------
	// New hosts + pubsubs (existing ones owned by the cancelled session)
	// would be cleaner, but testutil's NewConnectedMesh is t.Cleanup-
	// scoped to the test, so the OLD hosts/pubsubs are still alive AND
	// the subscriptions on them died with the closed ledgers. Build a
	// fresh mesh for session 2 so the subscribe lifecycle starts clean.
	hostsB2 := testutil.NewConnectedMesh(t, 2)
	hostA2, hostB2 := hostsB2[0], hostsB2[1]
	psA2, psB2 := newPubSubPair(t, ctx, hostA2, hostB2)

	ledgerB2, err := ledger.New(dbPathB, keyB, psB2)
	if err != nil {
		t.Fatalf("session 2: ledger B: %v", err)
	}
	t.Cleanup(func() { _ = ledgerB2.Close() })

	time.Sleep(800 * time.Millisecond)

	ledgerA2, err := ledger.New(dbPathA, keyA, psA2)
	if err != nil {
		t.Fatalf("session 2: ledger A: %v", err)
	}
	t.Cleanup(func() { _ = ledgerA2.Close() })

	// Append entry #11. The prev_hash MUST equal hashes[entryCount-1]
	// (i.e. hash of entry #10) — that's the proof that A's Merkle tree
	// was correctly rebuilt from the on-disk store.
	e11 := freshRating(pidA, "honesty", 0.99)
	h11, err := ledgerA2.Append(ctx, e11)
	if err != nil {
		t.Fatalf("session 2: Append #11: %v", err)
	}
	gotPrev := e11.GetRating().GetPrevHash()
	if !bytes.Equal(gotPrev, hashes[entryCount-1][:]) {
		t.Fatalf("session 2: entry #11 prev_hash != hash of #10:\n got  %x\n want %x",
			gotPrev, hashes[entryCount-1])
	}

	// Wait for B's inbox to receive #11. This is the crucial assertion:
	// if B's chain head had been LOST across the restart, #11's
	// prev_hash would point to a hash B doesn't know — the inbox would
	// orphan it, not accept it. By asserting InboxHas (not just
	// IsOrphan), we prove B's chain-head persistence works.
	if err := waitForInbox(ctx, ledgerB2, []byte(pidA), h11, 5*time.Second); err != nil {
		t.Fatalf("session 2: B did not ingest entry #11 in time: %v", err)
	}

	// Final sanity: B's inbox still contains every single entry from
	// session 1 — closure + reopen MUST be transparent.
	for i, h := range hashes {
		ok, err := ledgerB2.InboxHas(ctx, []byte(pidA), h)
		if err != nil {
			t.Fatalf("session 2: InboxHas idx=%d: %v", i, err)
		}
		if !ok {
			t.Fatalf("session 2: entry #%d disappeared from B's inbox after restart", i)
		}
	}
}

// -----------------------------------------------------------------------------
// helpers — copied verbatim from internal/ledger/impl_test.go style so this
// integration test can stand alone without poking into the unit-test
// helpers (different package).
// -----------------------------------------------------------------------------

func mintKey(t *testing.T) crypto.PrivKey {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	return priv
}

func newPubSubPair(t *testing.T, ctx context.Context, hA, hB host.Host) (*pubsub.PubSub, *pubsub.PubSub) {
	t.Helper()
	psA, err := pubsub.NewGossipSub(ctx, hA, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub A: %v", err)
	}
	psB, err := pubsub.NewGossipSub(ctx, hB, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub B: %v", err)
	}
	return psA, psB
}

// freshRating builds an unsigned Rating envelope from raterID about a
// fabricated subject. The ledger fills in PrevHash + Signature.
func freshRating(raterID peer.ID, dim string, score float64) *pb.SignedEntry {
	return &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(raterID),
		SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
		Dimension:       dim,
		Score:           score,
		Context:         "integration",
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
}

// waitForInbox polls InboxHas until the entry shows up or the deadline
// passes. Polling cadence is tight enough that the test feels
// instantaneous in the happy case (~100ms) but the deadline absorbs
// CI scheduler jitter.
func waitForInbox(ctx context.Context, l ledger.Ledger, raterID []byte, hash [32]byte, budget time.Duration) error {
	deadline := time.Now().Add(budget)
	for {
		ok, err := l.InboxHas(ctx, raterID, hash)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return context.DeadlineExceeded
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// Sanity guard: the integration test imports the FeedbackTopic constant
// to ensure a rename of that constant breaks this test loudly.
var _ = network.FeedbackTopic

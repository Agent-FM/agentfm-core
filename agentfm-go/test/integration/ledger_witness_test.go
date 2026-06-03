package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	"agentfm/test/testutil"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestWitness_FillsCatchUpGapAfterBossesOffline is the witness-mode
// acceptance test: a boss writes a signed entry; a witness ingests
// it via live gossip; the boss disappears; a NEW boss joins later
// and recovers the entry from the witness via CatchUpInbox.
//
// This is what `agentfm -mode witness` ultimately enables — a
// long-lived ledger replica that other bosses can catch up against
// even when the original author is gone.
func TestWitness_FillsCatchUpGapAfterBossesOffline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Phase 1: boss + witness online together.
	hosts := testutil.NewConnectedMesh(t, 2)
	bossHost, witHost := hosts[0], hosts[1]
	psBoss, psWit := newPubSubPair(t, ctx, bossHost, witHost)

	bossKey := mintKey(t)
	bossPid, err := peer.IDFromPrivateKey(bossKey)
	if err != nil {
		t.Fatalf("derive boss peer id: %v", err)
	}
	witKey := mintKey(t)

	// Witness opens FIRST so its FeedbackTopic subscription is in
	// place when the boss publishes. ~800ms grace lets gossipsub
	// mesh-formation propagate the witness's subscription.
	witDB := filepath.Join(t.TempDir(), "witness.db")
	witLedger, err := ledger.NewWithOptions(witDB, witKey, psWit, ledger.Options{Host: witHost})
	if err != nil {
		t.Fatalf("open witness ledger: %v", err)
	}
	t.Cleanup(func() { _ = witLedger.Close() })

	time.Sleep(800 * time.Millisecond)

	bossDB := filepath.Join(t.TempDir(), "boss.db")
	bossLedger, err := ledger.NewWithOptions(bossDB, bossKey, psBoss, ledger.Options{Host: bossHost})
	if err != nil {
		t.Fatalf("open boss ledger: %v", err)
	}

	// Boss writes one Rating.
	entry := freshRating(bossPid, "honesty", 0.8)
	entryHash, err := bossLedger.Append(ctx, entry)
	if err != nil {
		_ = bossLedger.Close()
		t.Fatalf("append: %v", err)
	}

	// Wait for witness's inbox to receive the gossiped entry.
	if err := waitForInbox(ctx, witLedger, []byte(bossPid), entryHash, 5*time.Second); err != nil {
		_ = bossLedger.Close()
		t.Fatalf("witness did not ingest gossiped entry: %v", err)
	}

	// Phase 2: boss disappears.
	if err := bossLedger.Close(); err != nil {
		t.Fatalf("close boss ledger: %v", err)
	}
	if err := bossHost.Close(); err != nil {
		t.Fatalf("close boss host: %v", err)
	}

	// Phase 3: late boss joins, connects only to the witness, recovers entry.
	lateHost := testutil.NewHost(t)
	testutil.ConnectHosts(t, lateHost, witHost)

	// PubSub on the late host (mostly for completeness; CatchUpInbox
	// doesn't actually need it because it pulls via stream protocol).
	latePS, err := pubsub.NewGossipSub(ctx, lateHost)
	if err != nil {
		t.Fatalf("late pubsub: %v", err)
	}

	lateKey := mintKey(t)
	lateDB := filepath.Join(t.TempDir(), "late.db")
	lateLedger, err := ledger.NewWithOptions(lateDB, lateKey, latePS, ledger.Options{Host: lateHost})
	if err != nil {
		t.Fatalf("open late ledger: %v", err)
	}
	t.Cleanup(func() { _ = lateLedger.Close() })

	if err := ledger.CatchUpInbox(ctx, lateLedger, lateHost, witHost.ID()); err != nil {
		t.Fatalf("late boss catch-up against witness: %v", err)
	}

	ok, err := lateLedger.InboxHas(ctx, []byte(bossPid), entryHash)
	if err != nil {
		t.Fatalf("InboxHas: %v", err)
	}
	if !ok {
		t.Fatalf("late boss never recovered the gossiped entry from the witness — durability gap not filled")
	}
}

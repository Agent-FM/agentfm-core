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

// TestWitness_RecoversFullMultiEntryHistoryWithNoBossOnline proves the
// durability guarantee a witness provides: a brand-new boss with an
// EMPTY ledger, joining when NO original boss is online, recovers the
// COMPLETE multi-entry history from the witness alone.
//
// It strengthens TestWitness_FillsCatchUpGapAfterBossesOffline (which
// syncs a single entry) by authoring a batch of distinct ratings and
// asserting every one of them is recovered — not just that catch-up
// returns without error.
func TestWitness_RecoversFullMultiEntryHistoryWithNoBossOnline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const numEntries = 6

	hosts := testutil.NewConnectedMesh(t, 2)
	bossHost, witHost := hosts[0], hosts[1]
	psBoss, psWit := newPubSubPair(t, ctx, bossHost, witHost)

	bossKey := mintKey(t)
	bossPid, err := peer.IDFromPrivateKey(bossKey)
	if err != nil {
		t.Fatalf("derive boss peer id: %v", err)
	}
	witKey := mintKey(t)

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

	hashes := make([][32]byte, 0, numEntries)
	for i := 0; i < numEntries; i++ {
		score := -0.5 + float64(i)*0.2
		h, err := bossLedger.Append(ctx, freshRating(bossPid, "honesty", score))
		if err != nil {
			_ = bossLedger.Close()
			t.Fatalf("append entry %d: %v", i, err)
		}
		hashes = append(hashes, h)
	}

	for i, h := range hashes {
		if err := waitForInbox(ctx, witLedger, []byte(bossPid), h, 10*time.Second); err != nil {
			_ = bossLedger.Close()
			t.Fatalf("witness did not ingest entry %d/%d via gossip: %v", i+1, numEntries, err)
		}
	}

	if err := bossLedger.Close(); err != nil {
		t.Fatalf("close boss ledger: %v", err)
	}
	if err := bossHost.Close(); err != nil {
		t.Fatalf("close boss host: %v", err)
	}

	lateHost := testutil.NewHost(t)
	testutil.ConnectHosts(t, lateHost, witHost)

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

	for i, h := range hashes {
		ok, err := lateLedger.InboxHas(ctx, []byte(bossPid), h)
		if err != nil {
			t.Fatalf("pre-catch-up InboxHas %d: %v", i, err)
		}
		if ok {
			t.Fatalf("fresh ledger already had entry %d before catch-up; test cannot prove recovery", i)
		}
	}

	if err := ledger.CatchUpInbox(ctx, lateLedger, lateHost, witHost.ID()); err != nil {
		t.Fatalf("late boss catch-up against witness: %v", err)
	}

	recovered := 0
	for i, h := range hashes {
		ok, err := lateLedger.InboxHas(ctx, []byte(bossPid), h)
		if err != nil {
			t.Fatalf("post-catch-up InboxHas %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("entry %d/%d NOT recovered from witness — full history not synced", i+1, numEntries)
		}
		recovered++
	}
	if recovered != numEntries {
		t.Fatalf("recovered %d/%d entries", recovered, numEntries)
	}
}

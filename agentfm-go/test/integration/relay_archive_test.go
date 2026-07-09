package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestRelay_ArchivesGossippedEntries spawns a tiny in-process relay (host +
// pubsub + ledger archive) and a separate gossiper-with-ledger that emits one
// signed Rating. The relay's SQLite inbox_entries table must contain the entry
// within 8s, proving the auto-subscribed FeedbackTopic goroutine is wired.
func TestRelay_ArchivesGossippedEntries(t *testing.T) {
	tmp := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Both hosts use known Ed25519 keys so the ledger's peerID matches the
	// host's peerID — the inbox verifier derives the rater's pubkey from
	// RaterPeerId and checks it against the entry signature. Using a
	// mismatched key causes every entry to be rejected.
	relayKey := mintEdKey(t)
	gossipKey := mintEdKey(t)

	relayHost := testutil.NewHostWithKey(t, relayKey)
	gossipHost := testutil.NewHostWithKey(t, gossipKey)
	testutil.ConnectHosts(t, relayHost, gossipHost)

	gossipPeerID, err := peer.IDFromPrivateKey(gossipKey)
	if err != nil {
		t.Fatalf("gossip peer id: %v", err)
	}

	// Create BOTH pubsubs up-front — same pattern as newPubSubPair in
	// ledger_dissemination_test.go.
	relayPS, gossipPS := newPubSubPair(t, ctx, relayHost, gossipHost)

	// Relay archive ledger opened FIRST so its FeedbackTopic subscription
	// is active before the GossipSub heartbeat tick.
	relayPath := filepath.Join(tmp, "relay_ledger.db")
	if err := os.MkdirAll(filepath.Dir(relayPath), 0o700); err != nil {
		t.Fatalf("mkdir relay: %v", err)
	}
	arch, err := ledger.NewWithOptions(relayPath, relayKey, relayPS, ledger.Options{Host: relayHost})
	if err != nil {
		t.Fatalf("open archive ledger: %v", err)
	}
	defer func() { _ = arch.Close() }()

	// 800ms heartbeat grace — same as ledger_dissemination_test.go.
	time.Sleep(800 * time.Millisecond)

	// Gossiper ledger opened after the sleep so the relay subscription is
	// visible in GossipSub's local peer registry.
	gossipPath := filepath.Join(tmp, "gossip_ledger.db")
	if err := os.MkdirAll(filepath.Dir(gossipPath), 0o700); err != nil {
		t.Fatalf("mkdir gossip: %v", err)
	}
	g, err := ledger.NewWithOptions(gossipPath, gossipKey, gossipPS, ledger.Options{Host: gossipHost})
	if err != nil {
		t.Fatalf("open gossip ledger: %v", err)
	}
	defer func() { _ = g.Close() }()

	// Append one Rating. The ledger signs it with gossipKey whose public
	// half is embedded in gossipPeerID, so inbox verification succeeds.
	rating := &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(gossipPeerID),
		SubjectPeerId:   bytes.Repeat([]byte{0xab}, 32),
		Dimension:       "honesty",
		Score:           -0.3,
		Context:         "relay_archive_test",
		TimestampUnixNs: time.Now().UnixNano(),
	}}}
	entryHash, err := g.Append(ctx, rating)
	if err != nil {
		t.Fatalf("gossip Append: %v", err)
	}

	// Poll the relay's Ledger.InboxHas — the cleanest way to wait for the
	// auto-subscribed goroutine to accept the entry without opening a
	// second SQLite handle against a live WAL-mode database.
	testutil.Eventually(t, 8*time.Second, func() bool {
		ok, err := arch.InboxHas(ctx, []byte(gossipPeerID), entryHash)
		if err != nil {
			return false
		}
		return ok
	}, "relay archive should ingest the gossipped rating into its inbox")

	// Belt-and-suspenders: confirm the row is queryable via a raw store
	// handle. This exercises the relay's SQLite persistence end-to-end.
	s, err := store.Open(relayPath)
	if err != nil {
		t.Fatalf("open relay store directly: %v", err)
	}
	defer s.Close()

	n := 0
	if err := s.IterateAllInboxEntries(ctx, func(*store.InboxEntry) error {
		n++
		return nil
	}); err != nil {
		t.Fatalf("iterate inbox: %v", err)
	}
	if n < 1 {
		t.Fatalf("relay SQLite has 0 inbox entries; want >= 1")
	}
}

// mintEdKey generates a fresh Ed25519 private key for a test identity.
// Separate from mintKey so callers can pass it to both NewHostWithKey and
// ledger.NewWithOptions, ensuring the peerID embedded in the host matches the
// peerID the ledger derives from the same key.
func mintEdKey(t *testing.T) crypto.PrivKey {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("gen ed25519 key: %v", err)
	}
	return priv
}

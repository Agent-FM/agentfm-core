package ledger_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/network"
	"agentfm/test/testutil"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// fixtures
// -----------------------------------------------------------------------------

func ctxImpl() context.Context { return context.Background() }

// signingIdentity returns (priv, peerID bytes) — a freshly-generated
// Ed25519 keypair plus the PeerID bytes derived from its public half.
func signingIdentity(t *testing.T) (crypto.PrivKey, []byte) {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatalf("derive peer id: %v", err)
	}
	return priv, []byte(pid)
}

// freshRating builds an unsigned Rating envelope owned by `raterID` —
// SignEntry inside Append fills in PrevHash + Signature.
func freshRating(raterID []byte, dim string, score float64) *pb.SignedEntry {
	return &pb.SignedEntry{
		Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
			RaterPeerId:     raterID,
			SubjectPeerId:   bytes.Repeat([]byte{0xee}, 32),
			Dimension:       dim,
			Score:           score,
			Context:         "test",
			TimestampUnixNs: time.Now().UnixNano(),
		}},
	}
}

// -----------------------------------------------------------------------------
// Append + Head: local-only mode (no gossip)
// -----------------------------------------------------------------------------

func TestAppend_LocalOnly_IncrementsHead(t *testing.T) {
	priv, rid := signingIdentity(t)
	path := filepath.Join(t.TempDir(), "ledger.db")
	l, err := ledger.New(path, priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	for i := 0; i < 5; i++ {
		_, err := l.Append(ctxImpl(), freshRating(rid, "honesty", float64(i)/10))
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	head, err := l.Head(ctxImpl())
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head == nil {
		t.Fatal("Head returned nil after 5 Appends")
	}
	if head.TreeSize != 5 {
		t.Fatalf("TreeSize = %d, want 5", head.TreeSize)
	}
	if len(head.RootHash) != 32 {
		t.Fatalf("RootHash len = %d, want 32", len(head.RootHash))
	}
	if len(head.Signature) == 0 {
		t.Fatal("head Signature empty — head was not self-signed")
	}
}

func TestAppend_ReturnsEntryHash(t *testing.T) {
	priv, rid := signingIdentity(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "h.db"), priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	entry := freshRating(rid, "latency", 0.42)
	h, err := l.Append(ctxImpl(), entry)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	var zero [32]byte
	if h == zero {
		t.Fatal("Append returned zero hash")
	}
	// The returned hash MUST equal what EntryHash now sees on the (signed) entry.
	if got := ledger.EntryHash(entry); got != h {
		t.Fatalf("returned hash %x != EntryHash %x", h, got)
	}
}

// -----------------------------------------------------------------------------
// Restart: rebuild Merkle tree from store; prev_hash chain continues.
// -----------------------------------------------------------------------------

func TestAppend_SurvivesRestart_ChainContinues(t *testing.T) {
	priv, rid := signingIdentity(t)
	path := filepath.Join(t.TempDir(), "restart.db")

	// Session 1: write 3 entries.
	l1, err := ledger.New(path, priv, nil)
	if err != nil {
		t.Fatalf("New #1: %v", err)
	}
	var lastHashBeforeRestart [32]byte
	for i := 0; i < 3; i++ {
		h, err := l1.Append(ctxImpl(), freshRating(rid, "honesty", float64(i)))
		if err != nil {
			t.Fatalf("Append #%d: %v", i, err)
		}
		lastHashBeforeRestart = h
	}
	headBefore, err := l1.Head(ctxImpl())
	if err != nil {
		t.Fatalf("Head before close: %v", err)
	}
	if err := l1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Session 2: reopen. Head should reflect the pre-close state; the
	// next Append's prev_hash should equal the last leaf from before.
	l2, err := ledger.New(path, priv, nil)
	if err != nil {
		t.Fatalf("New #2: %v", err)
	}
	t.Cleanup(func() { _ = l2.Close() })

	headAfter, err := l2.Head(ctxImpl())
	if err != nil {
		t.Fatalf("Head after reopen: %v", err)
	}
	if headAfter == nil {
		t.Fatal("Head after reopen is nil; persisted head was lost")
	}
	if headAfter.TreeSize != headBefore.TreeSize {
		t.Fatalf("TreeSize after reopen = %d, want %d", headAfter.TreeSize, headBefore.TreeSize)
	}
	if !bytes.Equal(headAfter.RootHash, headBefore.RootHash) {
		t.Fatalf("RootHash changed across restart")
	}

	// Append one more entry; its prev_hash MUST be the lastHash from
	// session 1, proving the tree was correctly rebuilt.
	nextEntry := freshRating(rid, "honesty", 9.9)
	if _, err := l2.Append(ctxImpl(), nextEntry); err != nil {
		t.Fatalf("post-restart Append: %v", err)
	}
	gotPrev := nextEntry.GetRating().GetPrevHash()
	if !bytes.Equal(gotPrev, lastHashBeforeRestart[:]) {
		t.Fatalf("post-restart prev_hash != last hash before restart:\n got  %x\n want %x",
			gotPrev, lastHashBeforeRestart[:])
	}
}

// -----------------------------------------------------------------------------
// Close idempotency
// -----------------------------------------------------------------------------

func TestClose_Idempotent(t *testing.T) {
	priv, _ := signingIdentity(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "close.db"), priv, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("second close should be a no-op, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Publish failure isolation: a missing topic / unsubscribed graph must
// NOT prevent local persistence. We exercise this by starting a single-
// node pubsub with no peers — Publish becomes a no-op success in
// libp2p, but more importantly, Append's contract is unchanged.
// -----------------------------------------------------------------------------

func TestAppend_NoSubscribers_PersistsLocally(t *testing.T) {
	priv, rid := signingIdentity(t)
	h := testutil.NewHost(t)
	ps, err := pubsub.NewGossipSub(ctxImpl(), h)
	if err != nil {
		t.Fatalf("NewGossipSub: %v", err)
	}

	l, err := ledger.New(filepath.Join(t.TempDir(), "lonely.db"), priv, ps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	for i := 0; i < 4; i++ {
		if _, err := l.Append(ctxImpl(), freshRating(rid, "honesty", float64(i))); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	head, err := l.Head(ctxImpl())
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head.TreeSize != 4 {
		t.Fatalf("TreeSize = %d, want 4", head.TreeSize)
	}
}

// -----------------------------------------------------------------------------
// 2-peer dissemination: A appends, B receives the raw payload on the
// feedback topic. This is the P1-4 acceptance test; P1-5 will hook the
// receiver into the ledger's verify path.
// -----------------------------------------------------------------------------

func TestAppend_TwoPeerGossipDissemination(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	hostA, hostB := hosts[0], hosts[1]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	psA, err := pubsub.NewGossipSub(ctx, hostA, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub A: %v", err)
	}
	psB, err := pubsub.NewGossipSub(ctx, hostB, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub B: %v", err)
	}

	// B subscribes first so that A's first Publish can reach it. (libp2p
	// GossipSub gossips heartbeats every second; without an existing
	// subscription the message may be dropped before B is ready.)
	topicB, err := psB.Join(network.FeedbackTopic)
	if err != nil {
		t.Fatalf("B Join: %v", err)
	}
	t.Cleanup(func() { _ = topicB.Close() })
	subB, err := topicB.Subscribe()
	if err != nil {
		t.Fatalf("B Subscribe: %v", err)
	}
	t.Cleanup(subB.Cancel)

	// Tiny grace period for B's subscription to propagate into A's mesh
	// view before A starts publishing. Empirically 200ms is enough; we
	// give a generous 800ms to keep CI machines happy under load.
	time.Sleep(800 * time.Millisecond)

	// A appends an entry via the real ledger pipeline.
	privA, ridA := signingIdentity(t)
	l, err := ledger.New(filepath.Join(t.TempDir(), "a.db"), privA, psA)
	if err != nil {
		t.Fatalf("ledger A: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	sent := freshRating(ridA, "honesty", 0.77)
	if _, err := l.Append(ctx, sent); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// B reads next message with a 5s budget — generous so flake risk is
	// low on slow CI. Normal latency is sub-100ms locally.
	rxCtx, rxCancel := context.WithTimeout(ctx, 5*time.Second)
	defer rxCancel()
	msg, err := subB.Next(rxCtx)
	if err != nil {
		t.Fatalf("B did not receive within 5s: %v", err)
	}

	// The gossiped payload must unmarshal to a SignedEntry equivalent
	// to what A sent (post-signing). proto.Equal handles cross-pointer
	// comparison; bytes equality would also work since the wire format
	// is byte-stable.
	var got pb.SignedEntry
	if err := proto.Unmarshal(msg.Data, &got); err != nil {
		t.Fatalf("unmarshal received: %v", err)
	}
	if !proto.Equal(sent, &got) {
		t.Fatalf("received payload != sent\n sent: %+v\n got:  %+v", sent, &got)
	}
}

// -----------------------------------------------------------------------------
// P1-5 acceptance: 2-peer end-to-end ingestion via the inbox.
//
// A appends an entry. B's ledger auto-subscribes to the feedback topic
// at construction; its inbox should pick up A's entry within a few
// seconds. Unlike the P1-4 raw-wire test, this exercises the full
// receive pipeline: pubsub → handler → verify → dedup → chain-check →
// inbox table.
// -----------------------------------------------------------------------------

func TestTwoPeer_BInboxIngestsAEntry(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	hostA, hostB := hosts[0], hosts[1]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	psA, err := pubsub.NewGossipSub(ctx, hostA, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub A: %v", err)
	}
	psB, err := pubsub.NewGossipSub(ctx, hostB, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("pubsub B: %v", err)
	}

	// B's ledger must be constructed FIRST so its subscription is in
	// place when A starts publishing. We also need a brief grace for
	// the GossipSub mesh to settle so A's mesh view includes B.
	keyB, _ := signingIdentity(t)
	ledgerB, err := ledger.New(filepath.Join(t.TempDir(), "b.db"), keyB, psB)
	if err != nil {
		t.Fatalf("ledger B: %v", err)
	}
	t.Cleanup(func() { _ = ledgerB.Close() })

	time.Sleep(800 * time.Millisecond)

	keyA, ridA := signingIdentity(t)
	ledgerA, err := ledger.New(filepath.Join(t.TempDir(), "a.db"), keyA, psA)
	if err != nil {
		t.Fatalf("ledger A: %v", err)
	}
	t.Cleanup(func() { _ = ledgerA.Close() })

	sent := freshRating(ridA, "honesty", 0.33)
	hash, err := ledgerA.Append(ctx, sent)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Wait up to 5s for B's auto-subscribe goroutine to drain the
	// gossip message and write the entry into the inbox. We poll
	// InboxHas, which is a pure read of B's local SQLite state — no
	// side-effects, so it cleanly distinguishes "gossip arrived and
	// ingested" from "we manually retried."
	deadline := time.Now().Add(5 * time.Second)
	for {
		ok, err := ledgerB.InboxHas(ctx, ridA, hash)
		if err != nil {
			t.Fatalf("InboxHas: %v", err)
		}
		if ok {
			return // success
		}
		if time.Now().After(deadline) {
			t.Fatalf("B's inbox did not ingest A's entry within 5s; hash=%x", hash)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

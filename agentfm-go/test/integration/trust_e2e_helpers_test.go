//go:build trust_e2e

package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
	"agentfm/test/testutil"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// openTestLedger opens a real SQLite-backed Ledger in the test's temp dir,
// using the host's own identity key for signing. A GossipSub instance is
// started on the host. The ledger is closed via t.Cleanup.
//
// The function includes a 1.2s sleep so GossipSub's heartbeat has time to
// propagate subscriptions to connected peers before the caller appends entries.
func openTestLedger(t testing.TB, h host.Host, name string) ledger.Ledger {
	t.Helper()
	priv := h.Peerstore().PrivKey(h.ID())
	if priv == nil {
		t.Fatalf("openTestLedger: host %s has no private key in peerstore", h.ID())
	}
	dbPath := filepath.Join(t.TempDir(), name+".db")

	ctx := context.Background()
	ps, err := pubsub.NewGossipSub(ctx, h, pubsub.WithFloodPublish(true))
	if err != nil {
		t.Fatalf("openTestLedger: pubsub: %v", err)
	}

	l, err := ledger.NewWithOptions(dbPath, priv, ps, ledger.Options{Host: h})
	if err != nil {
		t.Fatalf("openTestLedger: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	// Allow GossipSub's heartbeat (1s interval) to propagate subscriptions
	// to connected peers before the caller starts appending.
	time.Sleep(1200 * time.Millisecond)

	return l
}

// newSignedRating builds a pb.SignedEntry carrying a Rating authored by
// raterHost about subject.
func newSignedRating(t testing.TB, raterHost host.Host, subject peer.ID, score float64, ctx string) *pb.SignedEntry {
	t.Helper()
	return &pb.SignedEntry{Body: &pb.SignedEntry_Rating{Rating: &pb.Rating{
		RaterPeerId:     []byte(raterHost.ID()),
		SubjectPeerId:   []byte(subject),
		Dimension:       "honesty",
		Score:           score,
		Context:         ctx,
		TimestampUnixNs: time.Now().UnixNano(),
		PrevHash:        make([]byte, 32),
	}}}
}

// compile-time type check
var _ = testutil.Eventually

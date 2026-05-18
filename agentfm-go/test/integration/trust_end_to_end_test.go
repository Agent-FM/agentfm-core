//go:build trust_e2e

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"agentfm/internal/boss"
	"agentfm/internal/network"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TestTrustEndToEnd is the Phase 8 end-to-end acceptance test.
//
// Scenario:
//  1. Boss A appends a -0.30 rating about a subject peer.
//  2. Boss B, connected to boss A, receives the entry via GossipSub
//     (the standard mesh dissemination path).
//  3. Boss B can find the subject in ListKnownPeers (offline section).
//  4. Boss B's peer-view renders the -0.30 score.
func TestTrustEndToEnd(t *testing.T) {
	// Two connected hosts: A (rater) and B (observer).
	hosts := testutil.NewConnectedMesh(t, 2)
	bossAHost, bossBHost := hosts[0], hosts[1]

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Open B's ledger first so it has an active FeedbackTopic subscription
	// before A publishes. OpenTestLedger includes a GossipSub heartbeat
	// grace period so by the time it returns the subscription is live.
	bossBLedger := openTestLedger(t, bossBHost, "bossB")
	defer bossBLedger.Close()

	// Open A's ledger after B is ready to receive.
	bossALedger := openTestLedger(t, bossAHost, "bossA")
	defer bossALedger.Close()

	// Generate a subject peer (offline — just an ID, not a running host).
	subject := testutil.NewHost(t).ID()

	// Boss A rates the subject -0.30.
	rating := newSignedRating(t, bossAHost, subject, -0.30, "trust_e2e")
	entryHash, err := bossALedger.Append(ctx, rating)
	if err != nil {
		t.Fatalf("boss A append: %v", err)
	}

	// Boss B should receive the entry via GossipSub into its inbox.
	testutil.Eventually(t, 10*time.Second, func() bool {
		ok, _ := bossBLedger.InboxHas(ctx, []byte(bossAHost.ID()), entryHash)
		return ok
	}, "boss B should receive boss A's rating via gossip")

	// Boss B now knows about the subject. Wire the boss struct with the
	// ledger and its store so ListKnownPeers / RenderPeerView work.
	b := boss.NewForTest(&network.MeshNode{Host: bossBHost})
	b.SetLedger(bossBLedger)
	b.SetReadStoreForTest(bossBLedger.Store())

	known, _ := b.ListKnownPeers(ctx)
	found := false
	for _, kp := range known {
		if kp.PeerID == subject && !kp.IsOnline {
			found = true
		}
	}
	if !found {
		t.Fatal("expected subject to appear in offline section of ListKnownPeers")
	}

	rendered := b.RenderPeerView(ctx, subject.String())
	if !strings.Contains(rendered, "-0.30") {
		t.Fatalf("TUI peer-view did not render the propagated rating:\n%s", rendered)
	}
}

// Compile-time anchor: keep peer.ID import alive.
var _ peer.ID

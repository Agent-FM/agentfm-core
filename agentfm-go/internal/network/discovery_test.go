package network

import (
	"context"
	"testing"
	"time"

	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestMaintainLighthouse_ReconnectsAfterDrop proves the relay-connection
// keepalive re-dials after the direct connection drops (idle prune / blip).
// Without the maintenance loop the node loses its relay for good, which is
// what left the desktop toolbar stuck on "Connecting to relay…".
func TestMaintainLighthouse_ReconnectsAfterDrop(t *testing.T) {
	a := testutil.NewHost(t)
	relay := testutil.NewHost(t)
	testutil.ConnectHosts(t, a, relay)

	relayInfo := &peer.AddrInfo{ID: relay.ID(), Addrs: relay.Addrs()}
	if a.Network().Connectedness(relay.ID()) != netcore.Connected {
		t.Fatal("precondition: A should be connected to the relay")
	}

	// Drop the connection the way an idle prune or network blip would.
	if err := a.Network().ClosePeer(relay.ID()); err != nil {
		t.Fatalf("close peer: %v", err)
	}
	testutil.Eventually(t, 3*time.Second, func() bool {
		return a.Network().Connectedness(relay.ID()) != netcore.Connected
	}, "A should be disconnected after ClosePeer")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go maintainLighthouseConnection(ctx, a, relayInfo, 200*time.Millisecond)

	testutil.Eventually(t, 8*time.Second, func() bool {
		return a.Network().Connectedness(relay.ID()) == netcore.Connected
	}, "maintenance loop should re-dial the dropped relay connection")
}

// TestMaintainLighthouse_ExitsOnContextCancel guards the goroutine-lifecycle
// rule: the loop must return promptly when its context is cancelled.
func TestMaintainLighthouse_ExitsOnContextCancel(t *testing.T) {
	a := testutil.NewHost(t)
	relay := testutil.NewHost(t)
	relayInfo := &peer.AddrInfo{ID: relay.ID(), Addrs: relay.Addrs()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		maintainLighthouseConnection(ctx, a, relayInfo, 200*time.Millisecond)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance loop did not exit within 3s of context cancel")
	}
}

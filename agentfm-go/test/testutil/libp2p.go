package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// NewHost returns a real libp2p host bound to 127.0.0.1 on a random port.
// We use real hosts (not mocknet) because AgentFM's stream-deadline code
// paths depend on SetReadDeadline / SetWriteDeadline, which mocknet's
// in-memory pipes do not implement. The host is closed on test cleanup
// so no goroutines leak under `go test -race`.
func NewHost(t testing.TB) host.Host {
	t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// NewConnectedMesh returns n libp2p hosts fully connected to each other.
// Suitable for any test that needs peers which can open streams immediately.
func NewConnectedMesh(t testing.TB, n int) []host.Host {
	t.Helper()
	hosts := make([]host.Host, 0, n)
	for i := 0; i < n; i++ {
		hosts = append(hosts, NewHost(t))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if err := hosts[i].Connect(ctx, peer.AddrInfo{
				ID:    hosts[j].ID(),
				Addrs: hosts[j].Addrs(),
			}); err != nil {
				t.Fatalf("connect[%d→%d]: %v", i, j, err)
			}
		}
	}
	return hosts
}

// NewLinkedMesh returns n hosts that *can* dial each other but are not yet
// connected. Used for tests that need to verify a connection is actively
// established (e.g. mDNS notifee behaviour).
func NewLinkedMesh(t testing.TB, n int) []host.Host {
	t.Helper()
	hosts := make([]host.Host, 0, n)
	for i := 0; i < n; i++ {
		hosts = append(hosts, NewHost(t))
	}
	return hosts
}

// ConnectHosts dials b from a under a bounded context. Fails the test on
// timeout, keeping networking-related flakes surfaced quickly.
func ConnectHosts(t testing.TB, a, b host.Host) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Connect(ctx, peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}); err != nil {
		t.Fatalf("connect: %v", err)
	}
}

// PeerIDs returns the peer IDs of the given hosts in slice order. Useful
// in assertions that need to reference peer identities.
func PeerIDs(hosts []host.Host) []peer.ID {
	ids := make([]peer.ID, len(hosts))
	for i, h := range hosts {
		ids[i] = h.ID()
	}
	return ids
}

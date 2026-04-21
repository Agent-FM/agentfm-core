package network

import (
	"context"
	"fmt"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
)

// connectToLighthouse dials the bootstrap relay and, on success, reserves
// a circuit-relay slot on it so this node can be reached via p2p-circuit
// when direct NAT traversal fails. Both steps are bounded by
// StreamDialTimeout so a dead lighthouse never blocks startup.
func connectToLighthouse(ctx context.Context, h host.Host, relayInfo *peer.AddrInfo) {
	fmt.Println("🌍 Dialing Bootstrap Node / Relay...")

	dialCtx, dialCancel := context.WithTimeout(ctx, StreamDialTimeout)
	defer dialCancel()
	if err := h.Connect(dialCtx, *relayInfo); err != nil {
		fmt.Printf("⚠️  Failed to connect to Bootstrap Node: %v\n", err)
		return
	}
	fmt.Println("✅ Successfully connected to Bootstrap Node!")

	reserveCtx, reserveCancel := context.WithTimeout(ctx, StreamDialTimeout)
	defer reserveCancel()
	if _, err := client.Reserve(reserveCtx, h, *relayInfo); err != nil {
		fmt.Printf("⚠️  Failed to secure relay reservation: %v\n", err)
	} else {
		fmt.Println("✅ Relay reservation secured! Ready for NAT traversal fallback.")
	}
}

// startDiscovery wires up both discovery mechanisms: mDNS for same-LAN
// peers and DHT rendezvous for the wider internet. Workers advertise
// themselves under the rendezvous string; Boss nodes actively search for
// matching peers on a timer.
func startDiscovery(ctx context.Context, h host.Host, kademliaDHT *dht.IpfsDHT, isWorker bool) {
	mdnsService := mdns.NewMdnsService(h, MDNSServiceTag, &mdnsNotifee{h: h})
	mdnsService.Start()
	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)

	if isWorker {
		util.Advertise(ctx, routingDiscovery, RendezvousString)
		fmt.Println("🌍 [DHT] Worker advertised to decentralized mesh.")
	} else {
		go discoverPeers(ctx, h, routingDiscovery)
	}
}

// discoverPeers runs on Boss nodes only. Every 10 seconds it asks the
// DHT for peers under the rendezvous string and dials any it is not
// already connected to. The loop exits cleanly when ctx is cancelled.
func discoverPeers(ctx context.Context, h host.Host, routingDiscovery *routing.RoutingDiscovery) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Each DHT sweep is individually bounded so one unresponsive
			// peer can't stall the tick beyond the next interval.
			lookupCtx, lookupCancel := context.WithTimeout(ctx, StreamDialTimeout)
			peerChan, err := routingDiscovery.FindPeers(lookupCtx, RendezvousString)
			if err != nil {
				lookupCancel()
				continue
			}
			for p := range peerChan {
				if p.ID == h.ID() || len(p.Addrs) == 0 {
					continue
				}
				if h.Network().Connectedness(p.ID) == netcore.Connected {
					continue
				}
				dialCtx, dialCancel := context.WithTimeout(ctx, StreamDialTimeout)
				if err := h.Connect(dialCtx, p); err == nil {
					fmt.Printf("\n🌍 [DHT Fallback] Successfully connected to peer: %s\n", p.ID.String()[:8])
				}
				dialCancel()
			}
			lookupCancel()
		}
	}
}

// mdnsNotifee implements the libp2p mDNS callback interface. Each time
// a peer is discovered on the local network we attempt a bounded dial.
type mdnsNotifee struct{ h host.Host }

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("\n⚡ [mDNS] Discovered local node on Wi-Fi: %s\n", pi.ID.String()[:8])
	// mdns invokes this callback from its own goroutine with no parent
	// context, so a fresh bounded ctx is the right escape hatch here.
	// It caps the dial instead of blocking the mdns service indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), StreamDialTimeout)
	defer cancel()
	if err := n.h.Connect(ctx, pi); err != nil {
		fmt.Printf("⚠️  [mDNS] Failed to dial %s: %v\n", pi.ID.String()[:8], err)
	}
}

package network

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"agentfm/internal/obs"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
)

// lighthouseConnTag protects the relay connection from connection-manager
// trimming: without it, the connmgr prunes the (usually idle) lighthouse
// connection under pressure and the node silently loses its relay.
const lighthouseConnTag = "lighthouse"

// LighthouseReconnectInterval is how often maintainLighthouseConnection
// re-checks and, if dropped, re-dials the relay. Lives here (not in the
// gitignored constants.go) so it stays version-controlled.
const LighthouseReconnectInterval = 30 * time.Second

// connectToLighthouse dials the bootstrap relay and, on success, reserves
// a circuit-relay slot on it so this node can be reached via p2p-circuit
// when direct NAT traversal fails. Both steps are bounded by
// StreamDialTimeout so a dead lighthouse never blocks startup.
func connectToLighthouse(ctx context.Context, h host.Host, relayInfo *peer.AddrInfo) {
	fmt.Println("🌍 Dialing Bootstrap Node / Relay...")

	dialCtx, dialCancel := context.WithTimeout(ctx, StreamDialTimeout)
	defer dialCancel()
	if err := h.Connect(dialCtx, *relayInfo); err != nil {
		slog.Warn("connect to bootstrap node",
			slog.Any(obs.FieldErr, err),
			slog.String(obs.FieldPeerID, relayInfo.ID.String()),
		)
		return
	}
	// Protect the relay from connmgr trimming so an idle lighthouse
	// connection survives connection-count pressure. Idempotent.
	h.ConnManager().Protect(relayInfo.ID, lighthouseConnTag)
	fmt.Println("✅ Successfully connected to Bootstrap Node!")

	reserveCtx, reserveCancel := context.WithTimeout(ctx, StreamDialTimeout)
	defer reserveCancel()
	if _, err := client.Reserve(reserveCtx, h, *relayInfo); err != nil {
		slog.Warn("secure relay reservation",
			slog.Any(obs.FieldErr, err),
			slog.String(obs.FieldPeerID, relayInfo.ID.String()),
		)
	} else {
		fmt.Println("✅ Relay reservation secured! Ready for NAT traversal fallback.")
	}
}

// maintainLighthouseConnection re-dials (and re-reserves) the relay whenever
// the direct connection drops — an idle prune, a reservation TTL expiry, or a
// transient network blip. Without it the node loses its relay for good after
// the first disconnect, and boss /v1/about reports the relay as unreachable
// ("Connecting to relay…") permanently. Exits on ctx cancellation.
func maintainLighthouseConnection(ctx context.Context, h host.Host, relayInfo *peer.AddrInfo, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if h.Network().Connectedness(relayInfo.ID) == netcore.Connected {
				continue
			}
			slog.Warn("lighthouse connection lost; re-dialing",
				slog.String(obs.FieldPeerID, relayInfo.ID.String()))
			dialCtx, dialCancel := context.WithTimeout(ctx, StreamDialTimeout)
			err := h.Connect(dialCtx, *relayInfo)
			dialCancel()
			if err != nil {
				slog.Warn("lighthouse re-dial failed",
					slog.Any(obs.FieldErr, err),
					slog.String(obs.FieldPeerID, relayInfo.ID.String()))
				continue
			}
			h.ConnManager().Protect(relayInfo.ID, lighthouseConnTag)
			reserveCtx, reserveCancel := context.WithTimeout(ctx, StreamDialTimeout)
			if _, err := client.Reserve(reserveCtx, h, *relayInfo); err != nil {
				slog.Warn("lighthouse re-reservation failed",
					slog.Any(obs.FieldErr, err))
			}
			reserveCancel()
		}
	}
}

// startDiscovery wires up both discovery mechanisms: mDNS for same-LAN
// peers and DHT rendezvous for the wider internet. Workers advertise
// themselves under the rendezvous string; Boss nodes actively search for
// matching peers on a timer.
func startDiscovery(ctx context.Context, h host.Host, kademliaDHT *dht.IpfsDHT, isWorker bool) {
	mdnsService := mdns.NewMdnsService(h, MDNSServiceTag, &mdnsNotifee{h: h})
	if err := mdnsService.Start(); err != nil {
		slog.Warn("mdns discovery unavailable; same-LAN peer discovery disabled",
			slog.Any(obs.FieldErr, err),
		)
	}
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
				slog.Warn("dht find peers",
					slog.Any(obs.FieldErr, err),
					slog.String("rendezvous", RendezvousString),
				)
				lookupCancel()
				continue
			}
			for p := range peerChan {
				if ctx.Err() != nil {
					lookupCancel()
					return
				}
				if p.ID == h.ID() || len(p.Addrs) == 0 {
					continue
				}
				if h.Network().Connectedness(p.ID) == netcore.Connected {
					continue
				}
				dialCtx, dialCancel := context.WithTimeout(ctx, StreamDialTimeout)
				if err := h.Connect(dialCtx, p); err == nil {
					fmt.Printf("\n🌍 [DHT Fallback] Successfully connected to peer: %s\n", p.ID.String()[:8])
				} else {
					slog.Debug("dht fallback dial failed",
						slog.String(obs.FieldPeerID, p.ID.String()),
						slog.Any(obs.FieldErr, err),
					)
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
		slog.Warn("mdns dial",
			slog.Any(obs.FieldErr, err),
			slog.String(obs.FieldPeerID, pi.ID.String()),
		)
	}
}

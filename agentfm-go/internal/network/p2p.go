package network

import (
	"context"
	"fmt"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Config is the operator-facing knob set. Mode selects the node role
// (boss / worker / relay / api), SwarmKeyPath enables private-mesh PSK
// mode, and BootstrapURL overrides the hard-coded lighthouse for dev
// swarms. ListenPort is 0 for "pick any free port".
type Config struct {
	Mode         string
	SwarmKeyPath string
	ListenPort   int
	BootstrapURL string
}

// MeshNode is the assembled libp2p stack returned by Setup. The Boss and
// Worker packages only interact with the mesh through this handle, which
// keeps their code out of the libp2p build options entirely.
type MeshNode struct {
	Host        host.Host
	DHT         *dht.IpfsDHT
	PubSub      *pubsub.PubSub
	RelayAddr   string
	RelayPeerID peer.ID
}

// Setup builds the full mesh stack: libp2p host, Kademlia DHT, GossipSub,
// optional lighthouse dial, and the per-role discovery loop. Callers can
// immediately use the returned MeshNode to attach stream handlers and
// subscribe to telemetry.
func Setup(ctx context.Context, cfg Config) (*MeshNode, error) {
	bootstrapAddr := cfg.BootstrapURL
	if bootstrapAddr == "" && cfg.SwarmKeyPath == "" {
		bootstrapAddr = PublicLighthouse
	}

	h, err := createHost(cfg, bootstrapAddr)
	if err != nil {
		return nil, err
	}

	isWorker := cfg.Mode == "worker"
	ps, kademliaDHT, err := initRoutingAndPubSub(ctx, h, isWorker)
	if err != nil {
		return nil, err
	}

	var relayPeerID peer.ID
	if cfg.Mode != "relay" && bootstrapAddr != "" {
		if relayInfo, err := parseRelayInfo(bootstrapAddr); err == nil {
			relayPeerID = relayInfo.ID
			connectToLighthouse(ctx, h, relayInfo)
		}
	}

	startDiscovery(ctx, h, kademliaDHT, isWorker)

	return &MeshNode{
		Host:        h,
		DHT:         kademliaDHT,
		PubSub:      ps,
		RelayAddr:   bootstrapAddr,
		RelayPeerID: relayPeerID,
	}, nil
}

// parseRelayInfo turns an operator-supplied multiaddr string into a
// peer.AddrInfo the libp2p host can dial directly.
func parseRelayInfo(addr string) (*peer.AddrInfo, error) {
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid relay multiaddr: %w", err)
	}
	return peer.AddrInfoFromP2pAddr(maddr)
}

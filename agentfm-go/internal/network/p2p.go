package network

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/host/autonat"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/multiformats/go-multiaddr"
)

type Config struct {
	Mode         string
	SwarmKeyPath string
	ListenPort   int
	BootstrapURL string
}

type MeshNode struct {
	Host      host.Host
	DHT       *dht.IpfsDHT
	PubSub    *pubsub.PubSub
	RelayAddr string
}

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

	if cfg.Mode != "relay" && bootstrapAddr != "" {
		if relayInfo, err := parseRelayInfo(bootstrapAddr); err == nil {
			connectToLighthouse(ctx, h, relayInfo)
		}
	}

	startDiscovery(ctx, h, kademliaDHT, isWorker)

	return &MeshNode{
		Host:      h,
		DHT:       kademliaDHT,
		PubSub:    ps,
		RelayAddr: bootstrapAddr,
	}, nil
}

func parseRelayInfo(addr string) (*peer.AddrInfo, error) {
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid relay multiaddr: %w", err)
	}
	return peer.AddrInfoFromP2pAddr(maddr)
}

func loadOrGenerateIdentity(mode string) (crypto.PrivKey, error) {
	keyPath := fmt.Sprintf(".agentfm_%s_identity.key", mode)

	if keyBytes, err := os.ReadFile(keyPath); err == nil {
		if priv, err := crypto.UnmarshalPrivateKey(keyBytes); err == nil {
			return priv, nil
		}
	}

	fmt.Println("🔑 Generating new permanent node identity...")
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, err
	}

	keyBytes, _ := crypto.MarshalPrivateKey(priv)
	os.WriteFile(keyPath, keyBytes, 0600)

	return priv, nil
}

func createHost(cfg Config, bootstrapAddr string) (host.Host, error) {
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.ListenPort)

	privKey, err := loadOrGenerateIdentity(cfg.Mode)
	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}

	options := []libp2p.Option{
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.NATPortMap(),
		libp2p.Identity(privKey), // Attach permanent identity to libp2p
	}

	if cfg.SwarmKeyPath != "" {
		psk, err := LoadSwarmKey(cfg.SwarmKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load swarm key: %w", err)
		}
		options = append(options, libp2p.PrivateNetwork(psk))
		fmt.Printf("🔒 Joining Private Swarm (Port: %d)\n", cfg.ListenPort)
	} else {
		fmt.Println("🌐 Joining Public P2P Mesh...")
	}

	if cfg.Mode == "relay" {
		fmt.Println("📡 Enabling Circuit Relay v2 Service (Anchor Node)")
		options = append(options, libp2p.EnableRelayService())
	} else {
		options = append(options, libp2p.EnableRelay(), libp2p.EnableHolePunching())

		if bootstrapAddr != "" {
			if relayInfo, err := parseRelayInfo(bootstrapAddr); err == nil {
				options = append(options, libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}))
			}
		}
	}

	h, err := libp2p.New(options...)
	if err != nil {
		return nil, err
	}

	if cfg.Mode != "relay" {
		if _, err = autonat.New(h); err != nil {
			fmt.Printf("⚠️  [NAT] Failed to start AutoNAT service: %v\n", err)
		} else {
			fmt.Println("🌐 [NAT] AutoNAT service started. Testing public reachability...")
		}
	}

	return h, nil
}

func initRoutingAndPubSub(ctx context.Context, h host.Host, isWorker bool) (*pubsub.PubSub, *dht.IpfsDHT, error) {
	ps, err := pubsub.NewGossipSub(ctx, h, pubsub.WithFloodPublish(true))
	if err != nil {
		return nil, nil, err
	}
	var dhtOptions []dht.Option
	if isWorker {
		dhtOptions = append(dhtOptions, dht.Mode(dht.ModeClient))
	} else {
		dhtOptions = append(dhtOptions, dht.Mode(dht.ModeServer))
	}
	kademliaDHT, err := dht.New(ctx, h, dhtOptions...)
	if err != nil {
		return nil, nil, err
	}
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return nil, nil, err
	}
	return ps, kademliaDHT, nil
}

func connectToLighthouse(ctx context.Context, h host.Host, relayInfo *peer.AddrInfo) {
	fmt.Println("🌍 Dialing Bootstrap Node / Relay...")
	if err := h.Connect(ctx, *relayInfo); err != nil {
		fmt.Printf("⚠️  Failed to connect to Bootstrap Node: %v\n", err)
		return
	}
	fmt.Println("✅ Successfully connected to Bootstrap Node!")
	if _, err := client.Reserve(ctx, h, *relayInfo); err != nil {
		fmt.Printf("⚠️  Failed to secure relay reservation: %v\n", err)
	} else {
		fmt.Println("✅ Relay reservation secured! Ready for NAT traversal fallback.")
	}
}

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

func discoverPeers(ctx context.Context, h host.Host, routingDiscovery *routing.RoutingDiscovery) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			peerChan, err := routingDiscovery.FindPeers(ctx, RendezvousString)
			if err != nil {
				continue
			}
			for p := range peerChan {
				if p.ID == h.ID() || len(p.Addrs) == 0 {
					continue
				}
				if h.Network().Connectedness(p.ID) != netcore.Connected {
					if err := h.Connect(ctx, p); err == nil {
						fmt.Printf("\n🌍 [DHT Fallback] Successfully connected to peer: %s\n", p.ID.String()[:8])
					}
				}
			}
		}
	}
}

type mdnsNotifee struct{ h host.Host }

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("\n⚡ [mDNS] Discovered local node on Wi-Fi: %s\n", pi.ID.String()[:8])
	n.h.Connect(context.Background(), pi)
}
